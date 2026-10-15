package generator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

type memStore struct {
	customers []models.Customer
	products  []models.Product
	orders    []models.Order
	items     []models.OrderItem
	markers   map[string]DatasetMarker
	reset     int
	// batch sizes observed per entity, used to assert batching.
	batchSizes map[string][]int
}

func newMemStore() *memStore {
	return &memStore{
		markers:    make(map[string]DatasetMarker),
		batchSizes: make(map[string][]int),
	}
}

func (m *memStore) recordBatch(entity string, n int) {
	m.batchSizes[entity] = append(m.batchSizes[entity], n)
}

func (m *memStore) EnsureSchema(ctx context.Context) error { return nil }
func (m *memStore) Reset(ctx context.Context) error {
	m.reset++
	m.customers = nil
	m.products = nil
	m.orders = nil
	m.items = nil
	return nil
}
func (m *memStore) SaveCustomers(ctx context.Context, rows []models.Customer) error {
	m.recordBatch(EntityCustomers, len(rows))
	m.customers = append(m.customers, rows...)
	return nil
}
func (m *memStore) SaveProducts(ctx context.Context, rows []models.Product) error {
	m.recordBatch(EntityProducts, len(rows))
	m.products = append(m.products, rows...)
	return nil
}
func (m *memStore) SaveOrders(ctx context.Context, rows []models.Order) ([]uuid.UUID, error) {
	m.recordBatch(EntityOrders, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		m.orders = append(m.orders, r)
		ids = append(ids, r.ID)
	}
	return ids, nil
}
func (m *memStore) SaveOrderItems(ctx context.Context, rows []models.OrderItem) error {
	m.recordBatch(EntityOrderItems, len(rows))
	m.items = append(m.items, rows...)
	return nil
}
func (m *memStore) DatasetExists(ctx context.Context, datasetKey string) (bool, error) {
	_, ok := m.markers[datasetKey]
	return ok, nil
}
func (m *memStore) SaveDatasetMarker(ctx context.Context, marker DatasetMarker) error {
	m.markers[marker.DatasetKey] = marker
	return nil
}

func TestNew_Validation(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{"customers negative", Config{Customers: -1, MinItems: 1, MaxItems: 1}, "customers must be >= 0"},
		{"products negative", Config{Products: -1, MinItems: 1, MaxItems: 1}, "products must be >= 0"},
		{"orders negative", Config{Orders: -1, MinItems: 1, MaxItems: 1}, "orders must be >= 0"},
		{"min items < 1", Config{MinItems: -1, MaxItems: 1}, "min items per order must be >= 1"},
		{"max < min", Config{MinItems: 2, MaxItems: 1}, "max items per order"},
		{"batch < 1", Config{MinItems: 1, MaxItems: 1, BatchSize: -1}, "batch size must be >= 1"},
		{"batch too large", Config{MinItems: 1, MaxItems: 1, BatchSize: MaxBatchSize + 1}, "batch size must be <= 10000"},
		{"orders need customers", Config{Orders: 5, Customers: 0, Products: 1, MinItems: 1, MaxItems: 1}, "at least 1 customer"},
		{"orders need products", Config{Orders: 5, Customers: 1, Products: 0, MinItems: 1, MaxItems: 1}, "at least 1 product"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.cfg)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q missing %q", err.Error(), tc.want)
			}
		})
	}
}

func TestDeterminism(t *testing.T) {
	cfg := Config{
		Seed:      12345,
		Customers: 10,
		Products:  5,
		Orders:    3,
		MinItems:  1,
		MaxItems:  2,
		BatchSize: 1000,
		Reset:     true,
	}
	g1, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	g2, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s1 := newMemStore()
	s2 := newMemStore()
	sum1, err := g1.Generate(context.Background(), s1, nil)
	if err != nil {
		t.Fatal(err)
	}
	sum2, err := g2.Generate(context.Background(), s2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum1.Skipped || sum2.Skipped {
		t.Fatal("should not be skipped")
	}
	if len(s1.customers) != len(s2.customers) || len(s1.products) != len(s2.products) || len(s1.orders) != len(s2.orders) || len(s1.items) != len(s2.items) {
		t.Fatal("lengths differ")
	}
	if !reflect.DeepEqual(s1.customers, s2.customers) {
		t.Fatal("customers differ between runs with the same seed")
	}
	if !reflect.DeepEqual(s1.products, s2.products) {
		t.Fatal("products differ between runs with the same seed")
	}
	if !reflect.DeepEqual(s1.orders, s2.orders) {
		t.Fatal("orders differ between runs with the same seed")
	}
	if !reflect.DeepEqual(s1.items, s2.items) {
		t.Fatal("order items differ between runs with the same seed")
	}
}

func TestDifferentSeedsProduceDifferentData(t *testing.T) {
	base := Config{Customers: 10, Products: 5, Orders: 5, MinItems: 1, MaxItems: 3, BatchSize: 1000}
	cfgA := base
	cfgA.Seed = 1
	cfgB := base
	cfgB.Seed = 2

	gA, err := New(cfgA)
	if err != nil {
		t.Fatal(err)
	}
	gB, err := New(cfgB)
	if err != nil {
		t.Fatal(err)
	}
	sA, sB := newMemStore(), newMemStore()
	if _, err := gA.Generate(context.Background(), sA, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := gB.Generate(context.Background(), sB, nil); err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(sA.customers, sB.customers) {
		t.Fatal("different seeds produced identical customers")
	}
	if gA.DatasetKey() == gB.DatasetKey() {
		t.Fatal("different seeds produced the same dataset key")
	}
}

func TestUniqueness(t *testing.T) {
	cfg := Config{
		Seed:      42,
		Customers: 50,
		Products:  30,
		Orders:    20,
		MinItems:  1,
		MaxItems:  3,
		BatchSize: 500,
	}
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := newMemStore()
	if _, err := g.Generate(context.Background(), s, nil); err != nil {
		t.Fatal(err)
	}
	emails := make(map[string]struct{}, len(s.customers))
	for _, c := range s.customers {
		if c.Email == "" {
			t.Fatal("empty email")
		}
		if _, ok := emails[c.Email]; ok {
			t.Fatalf("duplicate email %s", c.Email)
		}
		emails[c.Email] = struct{}{}
	}
	skus := make(map[string]struct{}, len(s.products))
	for _, p := range s.products {
		if p.SKU == "" {
			t.Fatal("empty sku")
		}
		if _, ok := skus[p.SKU]; ok {
			t.Fatalf("duplicate sku %s", p.SKU)
		}
		skus[p.SKU] = struct{}{}
	}
	ordNums := make(map[string]struct{}, len(s.orders))
	for _, o := range s.orders {
		if o.OrderNumber == "" {
			t.Fatal("empty order number")
		}
		if _, ok := ordNums[o.OrderNumber]; ok {
			t.Fatalf("duplicate order number %s", o.OrderNumber)
		}
		ordNums[o.OrderNumber] = struct{}{}
	}
}

func TestTotalsMatch(t *testing.T) {
	cfg := Config{
		Seed:      7,
		Customers: 5,
		Products:  10,
		Orders:    15,
		MinItems:  1,
		MaxItems:  5,
		BatchSize: 1000,
	}
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := newMemStore()
	if _, err := g.Generate(context.Background(), s, nil); err != nil {
		t.Fatal(err)
	}
	itemsByOrder := make(map[uuid.UUID][]models.OrderItem, len(s.orders))
	for _, it := range s.items {
		itemsByOrder[it.OrderID] = append(itemsByOrder[it.OrderID], it)
	}
	for _, o := range s.orders {
		var sum float64
		for _, it := range itemsByOrder[o.ID] {
			sum += it.Subtotal
			if it.Quantity <= 0 {
				t.Fatalf("quantity <= 0 for order %s", o.ID)
			}
			if it.UnitPrice < 0 {
				t.Fatalf("unit price < 0 for order %s", o.ID)
			}
			if it.Subtotal < 0 {
				t.Fatalf("subtotal < 0 for order %s", o.ID)
			}
			if round2(float64(it.Quantity)*it.UnitPrice) != round2(it.Subtotal) {
				t.Fatalf("subtotal mismatch for item of order %s", o.ID)
			}
		}
		if round2(sum) != round2(o.TotalAmount) {
			t.Fatalf("total mismatch for order %s: sum=%f total=%f", o.ID, sum, o.TotalAmount)
		}
	}
}

func TestProgressCallback(t *testing.T) {
	cfg := Config{
		Seed:      1,
		Customers: 5,
		Products:  3,
		Orders:    4,
		MinItems:  1,
		MaxItems:  2,
		BatchSize: 2, // force multiple batches
	}
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := newMemStore()

	last := map[string]int64{}
	totals := map[string]int64{}
	_, genErr := g.Generate(context.Background(), s, func(entity string, done, total int64) {
		if prev, ok := last[entity]; ok && done < prev {
			t.Errorf("progress for %s went backwards: %d -> %d", entity, prev, done)
		}
		if total < done {
			t.Errorf("progress for %s has done(%d) > total(%d)", entity, done, total)
		}
		last[entity] = done
		totals[entity] = total
	})
	if genErr != nil {
		t.Fatal(genErr)
	}

	wantTotals := map[string]int64{
		EntityCustomers: int64(cfg.Customers),
		EntityProducts:  int64(cfg.Products),
		EntityOrders:    int64(cfg.Orders),
	}
	for entity, want := range wantTotals {
		if got := last[entity]; got != want {
			t.Errorf("%s final done = %d, want %d", entity, got, want)
		}
		if got := totals[entity]; got != want {
			t.Errorf("%s total = %d, want %d", entity, got, want)
		}
	}
	if last[EntityOrderItems] == 0 || last[EntityOrderItems] != totals[EntityOrderItems] {
		t.Errorf("order_items final done = %d, total = %d", last[EntityOrderItems], totals[EntityOrderItems])
	}
}

func TestBatchingRespectsBatchSize(t *testing.T) {
	const batch = 7
	cfg := Config{
		Seed:      3,
		Customers: 20,
		Products:  15,
		Orders:    25,
		MinItems:  1,
		MaxItems:  2,
		BatchSize: batch,
	}
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := newMemStore()
	if _, err := g.Generate(context.Background(), s, nil); err != nil {
		t.Fatal(err)
	}
	for entity, sizes := range s.batchSizes {
		if entity == EntityOrderItems {
			continue // filtered to inserted orders; sizes still <= batch
		}
		if len(sizes) < 2 {
			t.Errorf("%s: expected multiple batches, got %d", entity, len(sizes))
		}
		for i, size := range sizes {
			if size > batch {
				t.Errorf("%s batch %d size = %d, want <= %d", entity, i, size, batch)
			}
		}
	}
}

func TestReferentialIntegrity(t *testing.T) {
	cfg := Config{
		Seed:      11,
		Customers: 30,
		Products:  20,
		Orders:    40,
		MinItems:  1,
		MaxItems:  4,
		BatchSize: 6,
	}
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := newMemStore()
	if _, err := g.Generate(context.Background(), s, nil); err != nil {
		t.Fatal(err)
	}

	customers := make(map[uuid.UUID]struct{}, len(s.customers))
	for _, c := range s.customers {
		customers[c.ID] = struct{}{}
	}
	products := make(map[uuid.UUID]struct{}, len(s.products))
	for _, p := range s.products {
		products[p.ID] = struct{}{}
	}
	orders := make(map[uuid.UUID]struct{}, len(s.orders))
	for _, o := range s.orders {
		orders[o.ID] = struct{}{}
		if _, ok := customers[o.CustomerID]; !ok {
			t.Fatalf("order %s references unknown customer %s", o.ID, o.CustomerID)
		}
	}
	if len(s.items) == 0 {
		t.Fatal("expected order items")
	}
	for _, it := range s.items {
		if _, ok := orders[it.OrderID]; !ok {
			t.Fatalf("item references unknown order %s", it.OrderID)
		}
		if _, ok := products[it.ProductID]; !ok {
			t.Fatalf("item references unknown product %s", it.ProductID)
		}
	}
}

func TestDistributionAndFormats(t *testing.T) {
	cfg := Config{
		Seed:      2024,
		Customers: 100,
		Products:  80,
		Orders:    60,
		MinItems:  1,
		MaxItems:  5,
		BatchSize: 1000,
	}
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := newMemStore()
	if _, err := g.Generate(context.Background(), s, nil); err != nil {
		t.Fatal(err)
	}

	for _, c := range s.customers {
		if len(c.CountryCode) != 2 {
			t.Errorf("country code %q is not 2 chars", c.CountryCode)
		}
		if !strings.Contains(c.Email, "@") || !strings.Contains(c.Email, ".") {
			t.Errorf("malformed email %q", c.Email)
		}
		if c.FirstName == "" || c.LastName == "" || c.City == "" {
			t.Errorf("empty customer field: %+v", c)
		}
	}

	for _, p := range s.products {
		if p.Price < 5.00 || p.Price > 500.00 {
			t.Errorf("price %v out of range for %s", p.Price, p.SKU)
		}
		if p.Stock < 0 {
			t.Errorf("negative stock for %s", p.SKU)
		}
		if p.Category == "" || p.Name == "" {
			t.Errorf("empty product field: %+v", p)
		}
		if round2(p.Price) != p.Price {
			t.Errorf("price %v is not rounded to cents", p.Price)
		}
	}

	for _, o := range s.orders {
		if o.TotalAmount < 0 {
			t.Errorf("negative total for order %s", o.OrderNumber)
		}
		if !strings.HasPrefix(o.OrderNumber, "ORD-") {
			t.Errorf("bad order number %q", o.OrderNumber)
		}
		if len(o.Currency) != 3 {
			t.Errorf("bad currency %q", o.Currency)
		}
		var addr shippingAddress
		if err := json.Unmarshal(o.ShippingAddress, &addr); err != nil {
			t.Errorf("bad shipping address for order %s: %v", o.OrderNumber, err)
		}
		if addr.City == "" || addr.CountryCode == "" {
			t.Errorf("incomplete shipping address for order %s: %+v", o.OrderNumber, addr)
		}
	}
}

func TestTotalAmountEqualsItemSum(t *testing.T) {
	// Independent recomputation of total_amount to guard against regressions.
	cfg := Config{
		Seed:      77,
		Customers: 12,
		Products:  9,
		Orders:    30,
		MinItems:  1,
		MaxItems:  3,
		BatchSize: 4,
	}
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := newMemStore()
	if _, err := g.Generate(context.Background(), s, nil); err != nil {
		t.Fatal(err)
	}
	sums := make(map[uuid.UUID]float64, len(s.orders))
	for _, it := range s.items {
		sums[it.OrderID] += it.Subtotal
	}
	for _, o := range s.orders {
		if got, want := round2(o.TotalAmount), round2(sums[o.ID]); got != want {
			t.Fatalf("order %s: total_amount=%v, sum(items)=%v", o.OrderNumber, got, want)
		}
	}
}

func TestSummaryCounts(t *testing.T) {
	cfg := Config{
		Seed:      5,
		Customers: 4,
		Products:  3,
		Orders:    6,
		MinItems:  2,
		MaxItems:  3,
		BatchSize: 2,
	}
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := newMemStore()
	sum, err := g.Generate(context.Background(), s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Customers != 4 || sum.Products != 3 || sum.Orders != 6 {
		t.Errorf("unexpected summary: %+v", sum)
	}
	if int(sum.OrderItems) != len(s.items) {
		t.Errorf("summary items = %d, store has %d", sum.OrderItems, len(s.items))
	}
	if sum.OrderItems < 12 || sum.OrderItems > 18 {
		t.Errorf("items out of bounds: %d", sum.OrderItems)
	}
	if sum.DatasetKey == "" || sum.Skipped {
		t.Errorf("unexpected summary: %+v", sum)
	}
}

func TestSkipIfExists(t *testing.T) {
	cfg := Config{
		Seed:         99,
		Customers:    2,
		Products:     1,
		Orders:       1,
		MinItems:     1,
		MaxItems:     1,
		BatchSize:    1000,
		SkipIfExists: true,
	}
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := newMemStore()
	sum1, err := g.Generate(context.Background(), s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum1.Skipped {
		t.Fatal("first run should not skip")
	}
	sum2, err := g.Generate(context.Background(), s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !sum2.Skipped {
		t.Fatal("second run should skip")
	}
	if len(s.customers) != 2 {
		t.Fatal("data should not be regenerated on skip")
	}
}

func TestResetCalled(t *testing.T) {
	cfg := Config{
		Seed:      5,
		Customers: 1,
		Products:  1,
		Orders:    1,
		MinItems:  1,
		MaxItems:  1,
		BatchSize: 1000,
		Reset:     true,
	}
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := newMemStore()
	if _, err := g.Generate(context.Background(), s, nil); err != nil {
		t.Fatal(err)
	}
	if s.reset != 1 {
		t.Fatalf("reset called %d times, want 1", s.reset)
	}
}

func TestNilStore(t *testing.T) {
	cfg := DefaultConfig()
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Generate(context.Background(), nil, nil); err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestStoreErrorPropagates(t *testing.T) {
	cfg := Config{
		Seed:      1,
		Customers: 5,
		Products:  1,
		Orders:    0,
		MinItems:  1,
		MaxItems:  1,
		BatchSize: 2,
	}
	g, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := &failingStore{memStore: *newMemStore(), failAt: 2}
	_, genErr := g.Generate(context.Background(), s, nil)
	if genErr == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(genErr, errFail) {
		t.Errorf("expected wrapped fail error, got %v", genErr)
	}
}

var errFail = errors.New("fail")

type failingStore struct {
	memStore
	failAt int
	calls  int
}

func (f *failingStore) SaveCustomers(ctx context.Context, rows []models.Customer) error {
	f.calls++
	if f.calls >= f.failAt {
		return fmt.Errorf("save customers: %w", errFail)
	}
	return f.memStore.SaveCustomers(ctx, rows)
}
