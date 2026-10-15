package generator

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// Store persists generated records. The interface is intentionally small so the
// generator can be unit tested with an in-memory implementation and so the
// transport (GORM, raw SQL, COPY…) can change without touching the generation
// logic.
type Store interface {
	// EnsureSchema creates any auxiliary tables required by the generator
	// (currently only the dataset marker table). It must be idempotent.
	EnsureSchema(ctx context.Context) error
	// Reset removes previously generated source data. It is called before
	// generation when Config.Reset is true.
	Reset(ctx context.Context) error
	// SaveCustomers bulk-inserts customers, ignoring rows that already exist.
	SaveCustomers(ctx context.Context, rows []models.Customer) error
	// SaveProducts bulk-inserts products, ignoring rows that already exist.
	SaveProducts(ctx context.Context, rows []models.Product) error
	// SaveOrders bulk-inserts orders and returns the IDs of the rows that were
	// actually inserted on this call (rows that already existed are skipped).
	SaveOrders(ctx context.Context, rows []models.Order) (inserted []uuid.UUID, err error)
	// SaveOrderItems bulk-inserts order items.
	SaveOrderItems(ctx context.Context, rows []models.OrderItem) error
	// DatasetExists reports whether a dataset marker for datasetKey is present.
	DatasetExists(ctx context.Context, datasetKey string) (bool, error)
	// SaveDatasetMarker records (upserts) the completion of a dataset.
	SaveDatasetMarker(ctx context.Context, marker DatasetMarker) error
}

// DatasetMarker is the audit row written to the generator_seeds table once a
// dataset has been fully generated. Its primary key is the dataset fingerprint
// (Config.DatasetKey), which makes the write idempotent.
type DatasetMarker struct {
	DatasetKey string    `json:"dataset_key" gorm:"column:dataset_key;primaryKey;type:text"`
	Seed       int64     `json:"seed" gorm:"column:seed;not null"`
	Customers  int       `json:"customers" gorm:"column:customers;not null"`
	Products   int       `json:"products" gorm:"column:products;not null"`
	Orders     int       `json:"orders" gorm:"column:orders;not null"`
	MinItems   int       `json:"min_items" gorm:"column:min_items;not null"`
	MaxItems   int       `json:"max_items" gorm:"column:max_items;not null"`
	CreatedAt  time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// TableName pins the GORM model to the marker table created by the migration.
func (DatasetMarker) TableName() string {
	return "generator_seeds"
}
