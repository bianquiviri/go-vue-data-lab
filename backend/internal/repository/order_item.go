package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// GormOrderItemRepository reads order items from lab_source via GORM.
type GormOrderItemRepository struct {
	db *gorm.DB
}

// NewOrderItemRepository builds an OrderItemRepository on db (lab_source).
func NewOrderItemRepository(db *gorm.DB) OrderItemRepository {
	return &GormOrderItemRepository{db: db}
}

func orderItemQuery(db *gorm.DB, f OrderItemFilter) *gorm.DB {
	q := db.Model(&models.OrderItem{})
	if f.OrderID != nil {
		q = q.Where("order_id = ?", *f.OrderID)
	}
	if f.ProductID != nil {
		q = q.Where("product_id = ?", *f.ProductID)
	}
	return q
}

// List returns a filtered page of order items and the matching total count.
func (r *GormOrderItemRepository) List(ctx context.Context, opts ListOptions, f OrderItemFilter) ([]models.OrderItem, int64, error) {
	var total int64
	if err := orderItemQuery(r.db.WithContext(ctx), f).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("repository: count order items: %w", err)
	}

	rows := make([]models.OrderItem, 0)
	q := applyPaging(orderItemQuery(r.db.WithContext(ctx), f), opts).Order("id ASC")
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("repository: list order items: %w", err)
	}
	return rows, total, nil
}

// GetByID returns the order item with the given id or ErrNotFound.
func (r *GormOrderItemRepository) GetByID(ctx context.Context, id int64) (*models.OrderItem, error) {
	var row models.OrderItem
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if err != nil {
		return nil, wrapNotFound(fmt.Sprintf("get order item %d", id), err)
	}
	return &row, nil
}
