package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// GormProductRepository reads products from lab_source via GORM.
type GormProductRepository struct {
	db *gorm.DB
}

// NewProductRepository builds a ProductRepository on db (lab_source).
func NewProductRepository(db *gorm.DB) ProductRepository {
	return &GormProductRepository{db: db}
}

func productQuery(db *gorm.DB, f ProductFilter) *gorm.DB {
	q := db.Model(&models.Product{})
	if category := strings.TrimSpace(f.Category); category != "" {
		q = q.Where("category = ?", category)
	}
	if search := strings.TrimSpace(f.Search); search != "" {
		p := ilikePattern(search)
		q = q.Where("name ILIKE ? OR sku ILIKE ?", p, p)
	}
	return q
}

// List returns a filtered page of products and the matching total count.
func (r *GormProductRepository) List(ctx context.Context, opts ListOptions, f ProductFilter) ([]models.Product, int64, error) {
	var total int64
	if err := productQuery(r.db.WithContext(ctx), f).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("repository: count products: %w", err)
	}

	rows := make([]models.Product, 0)
	q := applyPaging(productQuery(r.db.WithContext(ctx), f), opts).
		Order("created_at DESC, id DESC")
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("repository: list products: %w", err)
	}
	return rows, total, nil
}

// GetByID returns the product with the given id or ErrNotFound.
func (r *GormProductRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Product, error) {
	var row models.Product
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if err != nil {
		return nil, wrapNotFound(fmt.Sprintf("get product %s", id), err)
	}
	return &row, nil
}

// Create inserts a new product.
func (r *GormProductRepository) Create(ctx context.Context, m *models.Product) error {
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return mapPgError(fmt.Sprintf("create product %s", m.ID), err)
	}
	return nil
}

// Update replaces an existing product. Returns ErrNotFound if no rows affected.
// Select forces every mutable column to be written, so zero values (e.g. stock
// or price set to 0) are persisted rather than skipped by GORM.
func (r *GormProductRepository) Update(ctx context.Context, m *models.Product) error {
	res := r.db.WithContext(ctx).Model(&models.Product{}).Where("id = ?", m.ID).
		Select("sku", "name", "category", "price", "stock", "status").
		Updates(m)
	if res.Error != nil {
		return mapPgError(fmt.Sprintf("update product %s", m.ID), res.Error)
	}
	if res.RowsAffected == 0 {
		return wrapNotFound(fmt.Sprintf("update product %s", m.ID), gorm.ErrRecordNotFound)
	}
	return nil
}

// Delete removes a product by id. Returns ErrNotFound if no rows affected.
func (r *GormProductRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&models.Product{}, "id = ?", id)
	if res.Error != nil {
		return mapPgError(fmt.Sprintf("delete product %s", id), res.Error)
	}
	if res.RowsAffected == 0 {
		return wrapNotFound(fmt.Sprintf("delete product %s", id), gorm.ErrRecordNotFound)
	}
	return nil
}
