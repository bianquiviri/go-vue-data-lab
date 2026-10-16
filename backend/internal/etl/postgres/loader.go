package postgres

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/etl"
)

// DefaultMaxRowsPerStmt bounds how many rows a single INSERT statement
// carries, keeping the parameter count far below PostgreSQL's 65535 limit even
// for wide tables.
const DefaultMaxRowsPerStmt = 1000

// upsertFlag is the scan target of `RETURNING (xmax = 0) AS inserted`.
type upsertFlag struct {
	Inserted bool `gorm:"column:inserted"`
}

// The canonical upsert statements. %s receives the multi-row VALUES clause.
//
// Conflict handling: every table is upserted on its primary key. The source IDs
// are copied verbatim to the target, so a re-run with the same data is a no-op
// and a re-run after edits performs an update. `(xmax = 0)` distinguishes newly
// inserted rows from updated ones so the runner can report both counts. Rows
// that collide on a secondary unique key (email/SKU/order_number) while holding
// a different primary key abort the batch — a data integrity problem that must
// be surfaced, not silently merged.
const upsertCustomersSQL = `INSERT INTO customers (id, first_name, last_name, email, country_code, city, state, created_at)
VALUES %s
ON CONFLICT (id) DO UPDATE SET
  first_name = EXCLUDED.first_name,
  last_name = EXCLUDED.last_name,
  email = EXCLUDED.email,
  country_code = EXCLUDED.country_code,
  city = EXCLUDED.city,
  state = EXCLUDED.state,
  created_at = EXCLUDED.created_at
RETURNING (xmax = 0) AS inserted`

const upsertProductsSQL = `INSERT INTO products (id, sku, name, category, price, stock, status, created_at)
VALUES %s
ON CONFLICT (id) DO UPDATE SET
  sku = EXCLUDED.sku,
  name = EXCLUDED.name,
  category = EXCLUDED.category,
  price = EXCLUDED.price,
  stock = EXCLUDED.stock,
  status = EXCLUDED.status,
  created_at = EXCLUDED.created_at
RETURNING (xmax = 0) AS inserted`

const upsertOrdersSQL = `INSERT INTO orders (id, order_number, customer_id, status, total_amount, currency, shipping_address, order_date)
VALUES %s
ON CONFLICT (id) DO UPDATE SET
  order_number = EXCLUDED.order_number,
  customer_id = EXCLUDED.customer_id,
  status = EXCLUDED.status,
  total_amount = EXCLUDED.total_amount,
  currency = EXCLUDED.currency,
  shipping_address = EXCLUDED.shipping_address,
  order_date = EXCLUDED.order_date
RETURNING (xmax = 0) AS inserted`

const upsertOrderItemsSQL = `INSERT INTO order_items (id, order_id, product_id, quantity, unit_price, subtotal)
VALUES %s
ON CONFLICT (id) DO UPDATE SET
  order_id = EXCLUDED.order_id,
  product_id = EXCLUDED.product_id,
  quantity = EXCLUDED.quantity,
  unit_price = EXCLUDED.unit_price,
  subtotal = EXCLUDED.subtotal
RETURNING (xmax = 0) AS inserted`

// Loader writes to the lab_target schema using SQL Raw bulk upserts.
type Loader struct {
	db             *gorm.DB
	maxRowsPerStmt int
}

var _ etl.Loader = (*Loader)(nil)

// NewLoader builds a Loader over db (the lab_target connection). A non-positive
// maxRowsPerStmt falls back to DefaultMaxRowsPerStmt.
func NewLoader(db *gorm.DB) (*Loader, error) {
	if db == nil {
		return nil, fmt.Errorf("etl/postgres: el loader requiere un *gorm.DB no nulo")
	}
	return &Loader{db: db, maxRowsPerStmt: DefaultMaxRowsPerStmt}, nil
}

// UpsertCustomers inserts or updates a batch of customers atomically.
func (l *Loader) UpsertCustomers(ctx context.Context, rows []models.Customer) (etl.LoadResult, error) {
	return l.upsert(ctx, etl.EntityCustomers, upsertCustomersSQL, 8, len(rows),
		func(lo, hi int) []any {
			args := make([]any, 0, (hi-lo)*8)
			for _, r := range rows[lo:hi] {
				args = append(args, r.ID, r.FirstName, r.LastName, r.Email,
					r.CountryCode, r.City, r.State, r.CreatedAt)
			}
			return args
		})
}

// UpsertProducts inserts or updates a batch of products atomically.
func (l *Loader) UpsertProducts(ctx context.Context, rows []models.Product) (etl.LoadResult, error) {
	return l.upsert(ctx, etl.EntityProducts, upsertProductsSQL, 8, len(rows),
		func(lo, hi int) []any {
			args := make([]any, 0, (hi-lo)*8)
			for _, r := range rows[lo:hi] {
				args = append(args, r.ID, r.SKU, r.Name, r.Category,
					r.Price, r.Stock, r.Status, r.CreatedAt)
			}
			return args
		})
}

// UpsertOrders inserts or updates a batch of orders atomically.
func (l *Loader) UpsertOrders(ctx context.Context, rows []models.Order) (etl.LoadResult, error) {
	return l.upsert(ctx, etl.EntityOrders, upsertOrdersSQL, 8, len(rows),
		func(lo, hi int) []any {
			args := make([]any, 0, (hi-lo)*8)
			for _, r := range rows[lo:hi] {
				var addr any
				if len(r.ShippingAddress) > 0 {
					addr = string(r.ShippingAddress)
				}
				args = append(args, r.ID, r.OrderNumber, r.CustomerID, r.Status,
					r.TotalAmount, r.Currency, addr, r.OrderDate)
			}
			return args
		})
}

// UpsertOrderItems inserts or updates a batch of order items atomically.
func (l *Loader) UpsertOrderItems(ctx context.Context, rows []models.OrderItem) (etl.LoadResult, error) {
	return l.upsert(ctx, etl.EntityOrderItems, upsertOrderItemsSQL, 6, len(rows),
		func(lo, hi int) []any {
			args := make([]any, 0, (hi-lo)*6)
			for _, r := range rows[lo:hi] {
				args = append(args, r.ID, r.OrderID, r.ProductID,
					r.Quantity, r.UnitPrice, r.Subtotal)
			}
			return args
		})
}

// upsert runs the whole batch inside a single transaction: on any statement
// failure the transaction rolls back and nothing from the batch persists.
func (l *Loader) upsert(
	ctx context.Context,
	entity etl.Entity,
	queryTemplate string,
	cols, n int,
	buildArgs func(lo, hi int) []any,
) (etl.LoadResult, error) {
	if n == 0 {
		return etl.LoadResult{}, nil
	}

	var total etl.LoadResult
	err := l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for lo := 0; lo < n; lo += l.maxRowsPerStmt {
			hi := min(lo+l.maxRowsPerStmt, n)
			query := fmt.Sprintf(queryTemplate, placeholders(hi-lo, cols))

			var flags []upsertFlag
			if err := tx.Raw(query, buildArgs(lo, hi)...).Scan(&flags).Error; err != nil {
				return fmt.Errorf("etl/postgres: upsert de %s (filas %d..%d): %w", entity, lo, hi-1, err)
			}
			for _, f := range flags {
				if f.Inserted {
					total.Inserted++
				} else {
					total.Updated++
				}
			}
		}
		return nil
	})
	if err != nil {
		return etl.LoadResult{}, fmt.Errorf("etl/postgres: cargar %s: %w", entity, err)
	}
	return total, nil
}

// placeholders builds a multi-row VALUES clause: "(?,?),(?,?)".
func placeholders(rows, cols int) string {
	var b strings.Builder
	b.Grow(rows * (cols*2 + 3))
	for i := 0; i < rows; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('(')
		for j := 0; j < cols; j++ {
			if j > 0 {
				b.WriteByte(',')
			}
			b.WriteByte('?')
		}
		b.WriteByte(')')
	}
	return b.String()
}
