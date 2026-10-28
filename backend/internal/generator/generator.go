package generator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// Generator produces synthetic e-commerce data and persists it via Store.
//
// The generator is stateless with respect to randomness: every record is a pure
// function of the seed and its index, so data can be generated in batches
// without holding the whole dataset in memory, and runs are reproducible.
type Generator struct {
	cfg Config
	ns  uuid.UUID
}

// New creates a Generator with the provided configuration. The config is
// normalized and validated.
func New(cfg Config) (*Generator, error) {
	cfg = cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	ns := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("lab-generator-seed-%d", cfg.Seed)))
	return &Generator{cfg: cfg, ns: ns}, nil
}

// Config returns the normalized configuration.
func (g *Generator) Config() Config {
	return g.cfg
}

// DatasetKey returns the fingerprint of the dataset (seed + counts).
func (g *Generator) DatasetKey() string {
	return g.cfg.DatasetKey()
}

// Generate runs the full pipeline: optionally reset, generate, bulk-insert and
// record a dataset marker. Progress is reported per batch through onProgress.
func (g *Generator) Generate(ctx context.Context, store Store, onProgress ProgressFunc) (Summary, error) {
	if store == nil {
		return Summary{}, fmt.Errorf("generator: store is nil")
	}
	if onProgress == nil {
		onProgress = func(string, int64, int64) {}
	}

	if err := store.EnsureSchema(ctx); err != nil {
		return Summary{}, fmt.Errorf("generator: ensure schema: %w", err)
	}

	key := g.cfg.DatasetKey()
	if g.cfg.SkipIfExists {
		exists, err := store.DatasetExists(ctx, key)
		if err != nil {
			return Summary{}, fmt.Errorf("generator: check dataset exists: %w", err)
		}
		if exists {
			return Summary{
				DatasetKey: key,
				Seed:       g.cfg.Seed,
				Customers:  int64(g.cfg.Customers),
				Products:   int64(g.cfg.Products),
				Orders:     int64(g.cfg.Orders),
				Skipped:    true,
			}, nil
		}
	}

	if g.cfg.Reset {
		if err := store.Reset(ctx); err != nil {
			return Summary{}, fmt.Errorf("generator: reset: %w", err)
		}
	}

	// Speed up bulk inserts by dropping non-PK indexes during generation.
	type indexManager interface {
		DisableIndexes(context.Context) error
		RecreateIndexes(context.Context) error
	}
	if im, ok := store.(indexManager); ok {
		if err := im.DisableIndexes(ctx); err != nil {
			return Summary{}, err
		}
		defer func() {
			_ = im.RecreateIndexes(context.Background())
		}()
	}

	// Customers: streamed in batches, IDs are reconstructible by index so no
	// in-memory list is required.
	if err := g.persistCustomers(ctx, store, onProgress); err != nil {
		return Summary{}, err
	}

	// Products: kept in memory because order items reuse their IDs and prices.
	products := g.generateProducts()
	if err := saveInBatches(ctx, g.cfg.BatchSize, products, store.SaveProducts, onProgress, EntityProducts, int64(len(products))); err != nil {
		return Summary{}, err
	}

	// Orders and their items: generated and persisted one batch at a time. The
	// planned item total is computed once so progress reporting is exact.
	totalItems := int64(g.totalItems())
	if err := g.persistOrders(ctx, store, products, totalItems, onProgress); err != nil {
		return Summary{}, err
	}

	marker := DatasetMarker{
		DatasetKey: key,
		Seed:       g.cfg.Seed,
		Customers:  g.cfg.Customers,
		Products:   g.cfg.Products,
		Orders:     g.cfg.Orders,
		MinItems:   g.cfg.MinItems,
		MaxItems:   g.cfg.MaxItems,
	}
	if err := store.SaveDatasetMarker(ctx, marker); err != nil {
		return Summary{}, fmt.Errorf("generator: save dataset marker: %w", err)
	}

	return Summary{
		DatasetKey: key,
		Seed:       g.cfg.Seed,
		Customers:  int64(g.cfg.Customers),
		Products:   int64(g.cfg.Products),
		Orders:     int64(g.cfg.Orders),
		OrderItems: totalItems,
	}, nil
}

// buildCustomer derives customer i from the seed.
func (g *Generator) buildCustomer(i int) models.Customer {
	r := g.deterministicRand("customer", i)
	fn := pick(r, firstNames)
	ln := pick(r, lastNames)
	loc := pick(r, locations)
	return models.Customer{
		ID:          g.customerID(i),
		FirstName:   fn,
		LastName:    ln,
		Email:       g.deterministicEmail(i, fn, ln),
		CountryCode: loc.CountryCode,
		City:        loc.City,
		State:       ptrString(loc.State),
		CreatedAt:   g.deterministicTime("customer-created", i),
	}
}

func (g *Generator) customerID(i int) uuid.UUID {
	return g.deterministicUUID("customer", i)
}

// productAttrs are the fields needed both to persist a product and to price the
// order items that reference it.
type productAttrs struct {
	Name     string
	Category string
	Price    float64
	Stock    int
	Status   string
}

func (g *Generator) buildProductAttrs(i int) productAttrs {
	r := g.deterministicRand("product", i)
	cat := pick(r, productCategories)
	noun := pick(r, categoryNouns[cat])
	return productAttrs{
		Name:     pick(r, productAdjectives) + " " + noun,
		Category: cat,
		Price:    round2(float64Range(r, 5.00, 500.00)),
		Stock:    intRange(r, 0, 500),
		Status:   pick(r, productStatuses),
	}
}

func (g *Generator) buildProduct(i int) models.Product {
	attrs := g.buildProductAttrs(i)
	return models.Product{
		ID:        g.deterministicUUID("product", i),
		SKU:       g.deterministicSKU(i, attrs.Name),
		Name:      attrs.Name,
		Category:  attrs.Category,
		Price:     attrs.Price,
		Stock:     attrs.Stock,
		Status:    attrs.Status,
		CreatedAt: g.deterministicTime("product-created", i),
	}
}

func (g *Generator) generateProducts() []models.Product {
	rows := make([]models.Product, 0, g.cfg.Products)
	for i := 0; i < g.cfg.Products; i++ {
		rows = append(rows, g.buildProduct(i))
	}
	return rows
}

// itemCountFor returns the deterministic number of items for order i.
func (g *Generator) itemCountFor(i int) int {
	r := g.deterministicRand("item-count", i)
	return intRange(r, g.cfg.MinItems, g.cfg.MaxItems)
}

// totalItems returns the planned number of order items across all orders.
func (g *Generator) totalItems() int {
	total := 0
	for i := 0; i < g.cfg.Orders; i++ {
		total += g.itemCountFor(i)
	}
	return total
}

// buildOrder derives order i and its items. products must be non-empty; item
// unit prices are taken from the referenced product so totals stay consistent.
func (g *Generator) buildOrder(i int, products []models.Product) (models.Order, []models.OrderItem) {
	r := g.deterministicRand("order", i)
	custIdx := r.Intn(g.cfg.Customers)
	loc := pick(r, locations)

	addr := shippingAddress{
		Street:      fmt.Sprintf("%d %s", intRange(r, 1, 999), pick(r, streetNames)),
		City:        loc.City,
		State:       loc.State,
		PostalCode:  fmt.Sprintf("%05d", intRange(r, 10000, 99999)),
		CountryCode: loc.CountryCode,
	}
	addrBytes, _ := json.Marshal(addr)

	orderID := g.deterministicUUID("order", i)
	itemCount := g.itemCountFor(i)
	items := make([]models.OrderItem, 0, itemCount)
	var total float64
	for j := 0; j < itemCount; j++ {
		ir := g.deterministicRand(fmt.Sprintf("order-item-%d", i), j)
		pIdx := ir.Intn(len(products))
		qty := intRange(ir, 1, 5)
		unit := products[pIdx].Price
		sub := round2(float64(qty) * unit)
		total += sub
		items = append(items, models.OrderItem{
			OrderID:   orderID,
			ProductID: products[pIdx].ID,
			Quantity:  qty,
			UnitPrice: unit,
			Subtotal:  sub,
		})
	}
	total = round2(total)

	order := models.Order{
		ID:              orderID,
		OrderNumber:     g.deterministicOrderNumber(i),
		CustomerID:      g.customerID(custIdx),
		Status:          pick(r, orderStatuses),
		TotalAmount:     total,
		Currency:        pick(r, currencies),
		ShippingAddress: addrBytes,
		OrderDate:       g.deterministicTime("order-date", i),
	}
	return order, items
}

type shippingAddress struct {
	Street      string `json:"street"`
	City        string `json:"city"`
	State       string `json:"state"`
	PostalCode  string `json:"postal_code"`
	CountryCode string `json:"country_code"`
}

func (g *Generator) persistCustomers(ctx context.Context, store Store, onProgress ProgressFunc) error {
	total := int64(g.cfg.Customers)
	for start := 0; start < g.cfg.Customers; start += g.cfg.BatchSize {
		end := min(start+g.cfg.BatchSize, g.cfg.Customers)
		rows := make([]models.Customer, 0, end-start)
		for i := start; i < end; i++ {
			rows = append(rows, g.buildCustomer(i))
		}
		if err := store.SaveCustomers(ctx, rows); err != nil {
			return fmt.Errorf("generator: save customers batch %d-%d: %w", start, end-1, err)
		}
		onProgress(EntityCustomers, int64(end), total)
	}
	return nil
}

func (g *Generator) persistOrders(ctx context.Context, store Store, products []models.Product, totalItems int64, onProgress ProgressFunc) error {
	if g.cfg.Orders == 0 {
		onProgress(EntityOrders, 0, 0)
		onProgress(EntityOrderItems, 0, 0)
		return nil
	}
	totalOrders := int64(g.cfg.Orders)

	var processedOrders, processedItems int64
	for start := 0; start < g.cfg.Orders; start += g.cfg.BatchSize {
		end := min(start+g.cfg.BatchSize, g.cfg.Orders)

		orders := make([]models.Order, 0, end-start)
		items := make([]models.OrderItem, 0, (end-start)*4)
		for i := start; i < end; i++ {
			order, orderItems := g.buildOrder(i, products)
			orders = append(orders, order)
			items = append(items, orderItems...)
		}

		inserted, err := store.SaveOrders(ctx, orders)
		if err != nil {
			return fmt.Errorf("generator: save orders batch %d-%d: %w", start, end-1, err)
		}
		processedOrders += int64(len(orders))
		onProgress(EntityOrders, processedOrders, totalOrders)

		if len(inserted) > 0 {
			insertedSet := make(map[uuid.UUID]struct{}, len(inserted))
			for _, id := range inserted {
				insertedSet[id] = struct{}{}
			}
			toInsert := make([]models.OrderItem, 0, len(items))
			for _, it := range items {
				if _, ok := insertedSet[it.OrderID]; ok {
					toInsert = append(toInsert, it)
				}
			}
			if len(toInsert) > 0 {
				base := processedItems
				if err := saveInBatches(ctx, g.cfg.BatchSize, toInsert, store.SaveOrderItems, func(_ string, done, total int64) {
					cur := base + done
					if cur > total {
						cur = total
					}
					onProgress(EntityOrderItems, cur, total)
				}, EntityOrderItems, totalItems); err != nil {
					return err
				}
				processedItems = base + int64(len(toInsert))
			} else {
				processedItems += int64(len(items))
			}
		} else {
			processedItems += int64(len(items))
		}
		if processedItems > totalItems {
			processedItems = totalItems
		}
		onProgress(EntityOrderItems, processedItems, totalItems)
	}
	return nil
}

// saveInBatches writes rows in chunks of BatchSize, invoking onProgress after
// each chunk. It is generic over the row type; onProgress may be nil.
func saveInBatches[T any](ctx context.Context, batchSize int, rows []T, save func(context.Context, []T) error, onProgress ProgressFunc, entity string, total int64) error {
	if len(rows) == 0 {
		if onProgress != nil {
			onProgress(entity, 0, total)
		}
		return nil
	}
	for start := 0; start < len(rows); start += batchSize {
		end := min(start+batchSize, len(rows))
		if err := save(ctx, rows[start:end]); err != nil {
			return fmt.Errorf("generator: save %s batch %d-%d: %w", entity, start, end-1, err)
		}
		if onProgress != nil {
			done := int64(end)
			if done > total {
				done = total
			}
			onProgress(entity, done, total)
		}
	}
	return nil
}
