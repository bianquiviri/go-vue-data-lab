package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// GormOrderRepository reads orders (and their items) from lab_source via GORM.
type GormOrderRepository struct {
	db *gorm.DB
}

// NewOrderRepository builds an OrderRepository on db (lab_source).
func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &GormOrderRepository{db: db}
}

func orderQuery(db *gorm.DB, f OrderFilter) *gorm.DB {
	q := db.Model(&models.Order{})
	if f.CustomerID != nil {
		q = q.Where("customer_id = ?", *f.CustomerID)
	}
	if status := strings.TrimSpace(f.Status); status != "" {
		q = q.Where("status = ?", strings.ToLower(status))
	}
	if f.DateFrom != nil {
		q = q.Where("order_date >= ?", *f.DateFrom)
	}
	if f.DateTo != nil {
		q = q.Where("order_date <= ?", *f.DateTo)
	}
	return q
}

// List returns a filtered page of orders and the matching total count. Items
// are not preloaded here (use GetByID for a single order with its items).
func (r *GormOrderRepository) List(ctx context.Context, opts ListOptions, f OrderFilter) ([]models.Order, int64, error) {
	var total int64
	if err := orderQuery(r.db.WithContext(ctx), f).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("repository: count orders: %w", err)
	}

	rows := make([]models.Order, 0)
	q := applyPaging(orderQuery(r.db.WithContext(ctx), f), opts).
		Order("order_date DESC, id DESC")
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("repository: list orders: %w", err)
	}
	return rows, total, nil
}

// GetByID returns the order with the given id, including its order items.
func (r *GormOrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Order, error) {
	var row models.Order
	err := r.db.WithContext(ctx).
		Preload("OrderItems").
		Where("id = ?", id).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(fmt.Sprintf("get order %s", id), err)
	}
	return &row, nil
}
