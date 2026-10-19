// Package repository holds the persistence layer.
//
// Repositories encapsulate GORM and raw SQL access to the lab_source and
// lab_target schemas. Concrete repositories are added in later issues.
package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// ErrNotFound is returned when a requested record does not exist. Consumers
// check it with errors.Is to translate it into a 404.
var ErrNotFound = errors.New("record not found")

// ListOptions carries SQL-level paging (already resolved from the API page
// contract by the service layer).
type ListOptions struct {
	Limit  int
	Offset int
}

// CustomerFilter narrows customer listings.
type CustomerFilter struct {
	// Search matches (case-insensitively) first name, last name or email.
	Search string
	// Country matches country_code exactly (ISO 3166-1 alpha-2).
	Country string
}

// ProductFilter narrows product listings.
type ProductFilter struct {
	// Search matches (case-insensitively) name or SKU.
	Search string
	// Category matches category exactly.
	Category string
}

// OrderFilter narrows order listings.
type OrderFilter struct {
	CustomerID *uuid.UUID
	Status     string
	// DateFrom/DateTo bound order_date inclusively (UTC).
	DateFrom *time.Time
	DateTo   *time.Time
}

// OrderItemFilter narrows order-item listings.
type OrderItemFilter struct {
	OrderID   *uuid.UUID
	ProductID *uuid.UUID
}

// CustomerRepository reads customers from lab_source.
type CustomerRepository interface {
	// List returns a page of customers plus the total row count for the
	// same filter.
	List(ctx context.Context, opts ListOptions, f CustomerFilter) ([]models.Customer, int64, error)
	// GetByID returns the customer with the given id, or ErrNotFound.
	GetByID(ctx context.Context, id uuid.UUID) (*models.Customer, error)
	Create(ctx context.Context, m *models.Customer) error
	Update(ctx context.Context, m *models.Customer) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// ProductRepository reads products from lab_source.
type ProductRepository interface {
	List(ctx context.Context, opts ListOptions, f ProductFilter) ([]models.Product, int64, error)
	// GetByID returns the product with the given id, or ErrNotFound.
	GetByID(ctx context.Context, id uuid.UUID) (*models.Product, error)
	Create(ctx context.Context, m *models.Product) error
	Update(ctx context.Context, m *models.Product) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// OrderRepository reads orders from lab_source.
type OrderRepository interface {
	List(ctx context.Context, opts ListOptions, f OrderFilter) ([]models.Order, int64, error)
	// GetByID returns the order with the given id including its order
	// items, or ErrNotFound.
	GetByID(ctx context.Context, id uuid.UUID) (*models.Order, error)
	Create(ctx context.Context, m *models.Order) error
	Update(ctx context.Context, m *models.Order) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// OrderItemRepository reads order items from lab_source.
type OrderItemRepository interface {
	List(ctx context.Context, opts ListOptions, f OrderItemFilter) ([]models.OrderItem, int64, error)
	// GetByID returns the order item with the given id, or ErrNotFound.
	GetByID(ctx context.Context, id int64) (*models.OrderItem, error)
}

// SourceRepositories bundles every lab_source repository so they can be
// constructed and injected together.
type SourceRepositories struct {
	Customers  CustomerRepository
	Products   ProductRepository
	Orders     OrderRepository
	OrderItems OrderItemRepository
}

// NewSourceRepositories builds the GORM-backed lab_source repositories on db.
func NewSourceRepositories(db *gorm.DB) *SourceRepositories {
	return &SourceRepositories{
		Customers:  NewCustomerRepository(db),
		Products:   NewProductRepository(db),
		Orders:     NewOrderRepository(db),
		OrderItems: NewOrderItemRepository(db),
	}
}

// TargetRepositories bundles every lab_target repository. lab_target mirrors
// the lab_source schema, so the same GORM implementations are reused against
// the target DB handle.
type TargetRepositories = SourceRepositories

// NewTargetRepositories builds the GORM-backed lab_target repositories on db.
// It reuses the lab_source implementations because both schemas are identical.
func NewTargetRepositories(db *gorm.DB) *TargetRepositories {
	return NewSourceRepositories(db)
}
