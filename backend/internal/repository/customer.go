package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// GormCustomerRepository reads customers from lab_source via GORM.
type GormCustomerRepository struct {
	db *gorm.DB
}

// NewCustomerRepository builds a CustomerRepository on db (lab_source).
func NewCustomerRepository(db *gorm.DB) CustomerRepository {
	return &GormCustomerRepository{db: db}
}

func customerQuery(db *gorm.DB, f CustomerFilter) *gorm.DB {
	q := db.Model(&models.Customer{})
	if country := strings.TrimSpace(f.Country); country != "" {
		q = q.Where("country_code = ?", strings.ToUpper(country))
	}
	if search := strings.TrimSpace(f.Search); search != "" {
		p := ilikePattern(search)
		q = q.Where("first_name ILIKE ? OR last_name ILIKE ? OR email ILIKE ?", p, p, p)
	}
	return q
}

// List returns a filtered page of customers and the matching total count.
func (r *GormCustomerRepository) List(ctx context.Context, opts ListOptions, f CustomerFilter) ([]models.Customer, int64, error) {
	var total int64
	if err := customerQuery(r.db.WithContext(ctx), f).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("repository: count customers: %w", err)
	}

	rows := make([]models.Customer, 0)
	q := applyPaging(customerQuery(r.db.WithContext(ctx), f), opts).
		Order("created_at DESC, id DESC")
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("repository: list customers: %w", err)
	}
	return rows, total, nil
}

// GetByID returns the customer with the given id or ErrNotFound.
func (r *GormCustomerRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Customer, error) {
	var row models.Customer
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if err != nil {
		return nil, wrapNotFound(fmt.Sprintf("get customer %s", id), err)
	}
	return &row, nil
}

// Create inserts a new customer.
func (r *GormCustomerRepository) Create(ctx context.Context, m *models.Customer) error {
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return mapPgError(fmt.Sprintf("create customer %s", m.ID), err)
	}
	return nil
}

// Update replaces an existing customer. Returns ErrNotFound if no rows affected.
// Select forces every mutable column to be written, so zero values (e.g. a nil
// state) are persisted rather than skipped by GORM's zero-value omission.
func (r *GormCustomerRepository) Update(ctx context.Context, m *models.Customer) error {
	res := r.db.WithContext(ctx).Model(&models.Customer{}).Where("id = ?", m.ID).
		Select("first_name", "last_name", "email", "country_code", "city", "state").
		Updates(m)
	if res.Error != nil {
		return mapPgError(fmt.Sprintf("update customer %s", m.ID), res.Error)
	}
	if res.RowsAffected == 0 {
		return wrapNotFound(fmt.Sprintf("update customer %s", m.ID), gorm.ErrRecordNotFound)
	}
	return nil
}

// Delete removes a customer by id. Returns ErrNotFound if no rows affected.
func (r *GormCustomerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Delete(&models.Customer{}, "id = ?", id)
	if res.Error != nil {
		return mapPgError(fmt.Sprintf("delete customer %s", id), res.Error)
	}
	if res.RowsAffected == 0 {
		return wrapNotFound(fmt.Sprintf("delete customer %s", id), gorm.ErrRecordNotFound)
	}
	return nil
}
