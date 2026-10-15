// Package postgres provides the GORM/SQL-Raw adapters for the etl.Source and
// etl.Loader ports. Reads use keyset pagination; writes use multi-row
// INSERT ... ON CONFLICT upserts executed atomically per batch. No AutoMigrate
// is ever used here.
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/etl"
)

// entityTables maps an etl.Entity to its (fixed, non-injectable) table name.
var entityTables = map[etl.Entity]string{
	etl.EntityCustomers:  "customers",
	etl.EntityProducts:   "products",
	etl.EntityOrders:     "orders",
	etl.EntityOrderItems: "order_items",
}

// Source reads from the lab_source schema using keyset pagination.
type Source struct {
	db *gorm.DB
}

var _ etl.Source = (*Source)(nil)

// NewSource builds a Source over db (the lab_source connection).
func NewSource(db *gorm.DB) (*Source, error) {
	if db == nil {
		return nil, fmt.Errorf("etl/postgres: el source requiere un *gorm.DB no nulo")
	}
	return &Source{db: db}, nil
}

// Count returns the number of rows of entity.
func (s *Source) Count(ctx context.Context, entity etl.Entity) (int64, error) {
	table, ok := entityTables[entity]
	if !ok {
		return 0, fmt.Errorf("etl/postgres: entidad desconocida %q", entity)
	}
	var n int64
	if err := s.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM " + table).Scan(&n).Error; err != nil {
		return 0, fmt.Errorf("etl/postgres: contar %s: %w", entity, err)
	}
	return n, nil
}

// FetchCustomers returns up to limit customers with id > after, ordered.
func (s *Source) FetchCustomers(ctx context.Context, after uuid.UUID, limit int) ([]models.Customer, error) {
	if limit <= 0 {
		return nil, nil
	}
	var out []models.Customer
	err := s.db.WithContext(ctx).Raw(`
		SELECT id, first_name, last_name, email, country_code, city, state, created_at
		FROM customers
		WHERE id > ?
		ORDER BY id ASC
		LIMIT ?`, after, limit).Scan(&out).Error
	if err != nil {
		return nil, fmt.Errorf("etl/postgres: extraer customers: %w", err)
	}
	return out, nil
}

// FetchProducts returns up to limit products with id > after, ordered.
func (s *Source) FetchProducts(ctx context.Context, after uuid.UUID, limit int) ([]models.Product, error) {
	if limit <= 0 {
		return nil, nil
	}
	var out []models.Product
	err := s.db.WithContext(ctx).Raw(`
		SELECT id, sku, name, category, price, stock, status, created_at
		FROM products
		WHERE id > ?
		ORDER BY id ASC
		LIMIT ?`, after, limit).Scan(&out).Error
	if err != nil {
		return nil, fmt.Errorf("etl/postgres: extraer products: %w", err)
	}
	return out, nil
}

// FetchOrders returns up to limit orders with id > after, ordered.
func (s *Source) FetchOrders(ctx context.Context, after uuid.UUID, limit int) ([]models.Order, error) {
	if limit <= 0 {
		return nil, nil
	}
	var out []models.Order
	err := s.db.WithContext(ctx).Raw(`
		SELECT id, order_number, customer_id, status, total_amount, currency, shipping_address, order_date
		FROM orders
		WHERE id > ?
		ORDER BY id ASC
		LIMIT ?`, after, limit).Scan(&out).Error
	if err != nil {
		return nil, fmt.Errorf("etl/postgres: extraer orders: %w", err)
	}
	return out, nil
}

// FetchOrderItems returns up to limit order items with id > after, ordered.
func (s *Source) FetchOrderItems(ctx context.Context, after int64, limit int) ([]models.OrderItem, error) {
	if limit <= 0 {
		return nil, nil
	}
	var out []models.OrderItem
	err := s.db.WithContext(ctx).Raw(`
		SELECT id, order_id, product_id, quantity, unit_price, subtotal
		FROM order_items
		WHERE id > ?
		ORDER BY id ASC
		LIMIT ?`, after, limit).Scan(&out).Error
	if err != nil {
		return nil, fmt.Errorf("etl/postgres: extraer order_items: %w", err)
	}
	return out, nil
}

// orderSum is the scan target of OrderItemSums.
type orderSum struct {
	OrderID uuid.UUID `gorm:"column:order_id"`
	Total   float64   `gorm:"column:total"`
}

// OrderItemSums returns the sum of item subtotals per order id.
func (s *Source) OrderItemSums(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID]float64, error) {
	sums := make(map[uuid.UUID]float64, len(orderIDs))
	if len(orderIDs) == 0 {
		return sums, nil
	}
	var rows []orderSum
	err := s.db.WithContext(ctx).Raw(`
		SELECT order_id, COALESCE(SUM(subtotal), 0)::float8 AS total
		FROM order_items
		WHERE order_id IN ?
		GROUP BY order_id`, orderIDs).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("etl/postgres: sumar subtotales de order_items: %w", err)
	}
	for _, r := range rows {
		sums[r.OrderID] = r.Total
	}
	return sums, nil
}
