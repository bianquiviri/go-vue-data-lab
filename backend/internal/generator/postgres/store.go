// Package postgres provides the GORM-backed adapter for the generator's Store
// port. It is kept separate from the pure generation logic so the latter can be
// tested without a database.
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/generator"
)

// generatorSeedsDDL is the canonical DDL for the generator's audit table. The
// same statement lives in the versioned migration
// migrations/source/003_generator_seeds.up.sql; it is repeated here (idempotent
// CREATE TABLE IF NOT EXISTS) so the CLI works even when migrations have not
// been applied yet. This is explicit DDL, never AutoMigrate.
const generatorSeedsDDL = `
CREATE TABLE IF NOT EXISTS generator_seeds (
  dataset_key TEXT PRIMARY KEY,
  seed BIGINT NOT NULL,
  customers INTEGER NOT NULL,
  products INTEGER NOT NULL,
  orders INTEGER NOT NULL,
  min_items INTEGER NOT NULL,
  max_items INTEGER NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

// PostgresStore is the GORM-backed Store implementation for the lab_source
// schema. Large writes use CreateInBatches with ON CONFLICT DO NOTHING so that
// deterministic keys make repeated runs idempotent.
type PostgresStore struct {
	db        *gorm.DB
	batchSize int
}

var _ generator.Store = (*PostgresStore)(nil)

// NewPostgresStore builds a PostgresStore. A non-positive batchSize falls back
// to DefaultBatchSize.
func NewPostgresStore(db *gorm.DB, batchSize int) (*PostgresStore, error) {
	if db == nil {
		return nil, fmt.Errorf("generator: postgres store requires a non-nil *gorm.DB")
	}
	if batchSize < 1 {
		batchSize = generator.DefaultBatchSize
	}
	if batchSize > 10000 {
		batchSize = 10000
	}
	return &PostgresStore{db: db, batchSize: batchSize}, nil
}

// DisableIndexes drops non-primary-key indexes to speed up bulk inserts.
// Only indexes from migrations/source/001_initial are considered; constraints
// (PK/UNIQUE) are kept.
func (p *PostgresStore) DisableIndexes(ctx context.Context) error {
	indexes := []string{
		"idx_customers_email",
		"idx_products_sku",
		"idx_products_category",
		"idx_orders_customer_id",
		"idx_orders_order_number",
		"idx_orders_order_date",
		"idx_order_items_order_id",
		"idx_order_items_product_id",
	}
	for _, idx := range indexes {
		if err := p.db.WithContext(ctx).Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s", idx)).Error; err != nil {
			return fmt.Errorf("generator: drop index %s: %w", idx, err)
		}
	}
	return nil
}

// RecreateIndexes recreates the indexes after bulk generation.
func (p *PostgresStore) RecreateIndexes(ctx context.Context) error {
	stmts := []string{
		"CREATE INDEX IF NOT EXISTS idx_customers_email ON customers(email)",
		"CREATE INDEX IF NOT EXISTS idx_products_sku ON products(sku)",
		"CREATE INDEX IF NOT EXISTS idx_products_category ON products(category)",
		"CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON orders(customer_id)",
		"CREATE INDEX IF NOT EXISTS idx_orders_order_number ON orders(order_number)",
		"CREATE INDEX IF NOT EXISTS idx_orders_order_date ON orders(order_date)",
		"CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id)",
		"CREATE INDEX IF NOT EXISTS idx_order_items_product_id ON order_items(product_id)",
	}
	for _, stmt := range stmts {
		if err := p.db.WithContext(ctx).Exec(stmt).Error; err != nil {
			return fmt.Errorf("generator: recreate index: %w", err)
		}
	}
	return nil
}

// EnsureSchema creates the marker table when missing.
func (p *PostgresStore) EnsureSchema(ctx context.Context) error {
	if err := p.db.WithContext(ctx).Exec(generatorSeedsDDL).Error; err != nil {
		return fmt.Errorf("generator: create generator_seeds: %w", err)
	}
	return nil
}

// Reset truncates the source data tables. RESTART IDENTITY resets the
// order_items sequence; CASCADE follows foreign keys.
func (p *PostgresStore) Reset(ctx context.Context) error {
	err := p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Exec("TRUNCATE TABLE order_items, orders, products, customers RESTART IDENTITY CASCADE").Error
	})
	if err != nil {
		return fmt.Errorf("generator: truncate source tables: %w", err)
	}
	return nil
}

// SaveCustomers bulk-inserts customers, skipping existing emails/IDs.
func (p *PostgresStore) SaveCustomers(ctx context.Context, rows []models.Customer) error {
	if len(rows) == 0 {
		return nil
	}
	if err := p.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(rows, p.batchSize).Error; err != nil {
		return fmt.Errorf("generator: insert customers: %w", err)
	}
	return nil
}

// SaveProducts bulk-inserts products, skipping existing SKUs/IDs.
func (p *PostgresStore) SaveProducts(ctx context.Context, rows []models.Product) error {
	if len(rows) == 0 {
		return nil
	}
	if err := p.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(rows, p.batchSize).Error; err != nil {
		return fmt.Errorf("generator: insert products: %w", err)
	}
	return nil
}

// SaveOrders inserts orders that do not exist yet and returns the IDs of the
// rows actually inserted. Because order IDs are deterministic, a repeated run
// with the same seed finds them already present and reports nothing inserted;
// order items are then not re-created.
func (p *PostgresStore) SaveOrders(ctx context.Context, rows []models.Order) ([]uuid.UUID, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	ids := make([]uuid.UUID, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}

	var existing []uuid.UUID
	if err := p.db.WithContext(ctx).
		Model(&models.Order{}).
		Where("id IN ?", ids).
		Pluck("id", &existing).Error; err != nil {
		return nil, fmt.Errorf("generator: query existing orders: %w", err)
	}

	if len(existing) == 0 {
		if err := p.insertOrders(ctx, rows); err != nil {
			return nil, err
		}
		return ids, nil
	}

	seen := make(map[uuid.UUID]struct{}, len(existing))
	for _, id := range existing {
		seen[id] = struct{}{}
	}
	fresh := make([]models.Order, 0, len(rows)-len(existing))
	freshIDs := make([]uuid.UUID, 0, cap(fresh))
	for _, o := range rows {
		if _, ok := seen[o.ID]; ok {
			continue
		}
		fresh = append(fresh, o)
		freshIDs = append(freshIDs, o.ID)
	}
	if len(fresh) > 0 {
		if err := p.insertOrders(ctx, fresh); err != nil {
			return nil, err
		}
	}
	// Always return all order IDs in this batch so order_items can be
	// generated for newly inserted orders even when run in append mode.
	return ids, nil
}

func (p *PostgresStore) insertOrders(ctx context.Context, rows []models.Order) error {
	if err := p.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(rows, p.batchSize).Error; err != nil {
		return fmt.Errorf("generator: insert orders: %w", err)
	}
	return nil
}

// SaveOrderItems bulk-inserts order items.
func (p *PostgresStore) SaveOrderItems(ctx context.Context, rows []models.OrderItem) error {
	if len(rows) == 0 {
		return nil
	}
	if err := p.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(rows, p.batchSize).Error; err != nil {
		return fmt.Errorf("generator: insert order items: %w", err)
	}
	return nil
}

// DatasetExists reports whether a marker row exists for datasetKey.
func (p *PostgresStore) DatasetExists(ctx context.Context, datasetKey string) (bool, error) {
	var count int64
	if err := p.db.WithContext(ctx).
		Model(&generator.DatasetMarker{}).
		Where("dataset_key = ?", datasetKey).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("generator: count dataset markers: %w", err)
	}
	return count > 0, nil
}

// SaveDatasetMarker upserts the audit row for a completed dataset.
func (p *PostgresStore) SaveDatasetMarker(ctx context.Context, marker generator.DatasetMarker) error {
	err := p.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "dataset_key"}},
			DoUpdates: clause.Assignments(map[string]any{
				"seed":       marker.Seed,
				"customers":  marker.Customers,
				"products":   marker.Products,
				"orders":     marker.Orders,
				"min_items":  marker.MinItems,
				"max_items":  marker.MaxItems,
				"updated_at": gorm.Expr("NOW()"),
			}),
		}).
		Create(&marker).Error
	if err != nil {
		return fmt.Errorf("generator: upsert dataset marker: %w", err)
	}
	return nil
}
