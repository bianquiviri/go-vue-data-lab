package etl

import (
	"context"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// LoadResult reports how many rows a bulk upsert inserted versus updated.
type LoadResult struct {
	Inserted int64
	Updated  int64
}

// Source is the read port over lab_source. Implementations must honour keyset
// pagination: FetchX(after, limit) returns up to limit rows whose primary key
// sorts strictly after after, in ascending key order.
type Source interface {
	// Count returns the total number of rows for entity.
	Count(ctx context.Context, entity Entity) (int64, error)

	FetchCustomers(ctx context.Context, after uuid.UUID, limit int) ([]models.Customer, error)
	FetchProducts(ctx context.Context, after uuid.UUID, limit int) ([]models.Product, error)
	FetchOrders(ctx context.Context, after uuid.UUID, limit int) ([]models.Order, error)
	FetchOrderItems(ctx context.Context, after int64, limit int) ([]models.OrderItem, error)

	// OrderItemSums returns, per order id, the sum of its item subtotals. Orders
	// without items are absent from the map.
	OrderItemSums(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID]float64, error)
}

// Loader is the write port over lab_target. Every Upsert call is executed as a
// single atomic transaction: on error nothing from the batch is persisted.
type Loader interface {
	UpsertCustomers(ctx context.Context, rows []models.Customer) (LoadResult, error)
	UpsertProducts(ctx context.Context, rows []models.Product) (LoadResult, error)
	UpsertOrders(ctx context.Context, rows []models.Order) (LoadResult, error)
	UpsertOrderItems(ctx context.Context, rows []models.OrderItem) (LoadResult, error)
}
