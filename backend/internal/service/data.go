// Package service holds the application's business logic layer.
//
// Services orchestrate repositories and domain models, and are consumed by the
// HTTP handlers. Concrete services (auth, data generation, concurrent
// processing, scheduler) are added in later issues.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
)

// Paging defaults/limits applied defensively by the service. They mirror the
// HTTP-level pagination contract (handler.DefaultPage/DefaultPerPage/MaxPerPage).
const (
	defaultPage    = 1
	defaultPerPage = 20
	maxPerPage     = 100
)

// ErrNotFound is returned by service lookups when the record does not exist.
// Handlers translate it into a 404.
var ErrNotFound = errors.New("record not found")

// PageResult is the outcome of a paged service query: the items, the total
// number of matching rows and the paging actually applied.
type PageResult[T any] struct {
	Items   []T
	Total   int64
	Page    int
	PerPage int
}

// TotalPages returns the number of pages for the result's total.
func (r PageResult[T]) TotalPages() int {
	if r.Total <= 0 || r.PerPage <= 0 {
		return 0
	}
	return int((r.Total + int64(r.PerPage) - 1) / int64(r.PerPage))
}

// CustomerListParams is the service-level query for customers.
type CustomerListParams struct {
	Page    int
	PerPage int
	Search  string
	Country string
}

// ProductListParams is the service-level query for products.
type ProductListParams struct {
	Page     int
	PerPage  int
	Search   string
	Category string
}

// OrderListParams is the service-level query for orders.
type OrderListParams struct {
	Page       int
	PerPage    int
	CustomerID *uuid.UUID
	Status     string
	DateFrom   *time.Time
	DateTo     *time.Time
}

// OrderItemListParams is the service-level query for order items.
type OrderItemListParams struct {
	Page      int
	PerPage   int
	OrderID   *uuid.UUID
	ProductID *uuid.UUID
}

// SourceData is the read port over lab_source consumed by the HTTP layer.
type SourceData interface {
	ListCustomers(ctx context.Context, p CustomerListParams) (PageResult[models.Customer], error)
	GetCustomer(ctx context.Context, id uuid.UUID) (models.Customer, error)

	ListProducts(ctx context.Context, p ProductListParams) (PageResult[models.Product], error)
	GetProduct(ctx context.Context, id uuid.UUID) (models.Product, error)

	ListOrders(ctx context.Context, p OrderListParams) (PageResult[models.Order], error)
	GetOrder(ctx context.Context, id uuid.UUID) (models.Order, error)

	ListOrderItems(ctx context.Context, p OrderItemListParams) (PageResult[models.OrderItem], error)
	GetOrderItem(ctx context.Context, id int64) (models.OrderItem, error)
}

// DataService implements SourceData over the lab_source repositories.
type DataService struct {
	customers  repository.CustomerRepository
	products   repository.ProductRepository
	orders     repository.OrderRepository
	orderItems repository.OrderItemRepository
}

// NewDataService builds a DataService from the lab_source repositories.
func NewDataService(
	customers repository.CustomerRepository,
	products repository.ProductRepository,
	orders repository.OrderRepository,
	orderItems repository.OrderItemRepository,
) *DataService {
	return &DataService{
		customers:  customers,
		products:   products,
		orders:     orders,
		orderItems: orderItems,
	}
}

// normalizePaging clamps the requested paging to the sane contract.
func normalizePaging(page, perPage int) (int, int) {
	if page < 1 {
		page = defaultPage
	}
	switch {
	case perPage < 1:
		perPage = defaultPerPage
	case perPage > maxPerPage:
		perPage = maxPerPage
	}
	return page, perPage
}

// listOptions resolves page/per_page into SQL limit/offset.
func listOptions(page, perPage int) (int, int, repository.ListOptions) {
	page, perPage = normalizePaging(page, perPage)
	return page, perPage, repository.ListOptions{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}
}

// wrapErr adds operation context and maps ErrNotFound to service.ErrNotFound
// so callers can match a single sentinel.
func wrapErr(op string, err error) error {
	if err == nil {
		return nil
	}
	msg := "service: " + op
	if errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("%s: %w", msg, ErrNotFound)
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// ListCustomers returns a filtered page of customers.
func (s *DataService) ListCustomers(ctx context.Context, p CustomerListParams) (PageResult[models.Customer], error) {
	page, perPage, opts := listOptions(p.Page, p.PerPage)
	rows, total, err := s.customers.List(ctx, opts, repository.CustomerFilter{
		Search:  p.Search,
		Country: p.Country,
	})
	if err != nil {
		return PageResult[models.Customer]{}, wrapErr("list customers", err)
	}
	return PageResult[models.Customer]{Items: rows, Total: total, Page: page, PerPage: perPage}, nil
}

// GetCustomer returns a single customer or ErrNotFound.
func (s *DataService) GetCustomer(ctx context.Context, id uuid.UUID) (models.Customer, error) {
	row, err := s.customers.GetByID(ctx, id)
	if err != nil {
		return models.Customer{}, wrapErr("get customer", err)
	}
	return *row, nil
}

// ListProducts returns a filtered page of products.
func (s *DataService) ListProducts(ctx context.Context, p ProductListParams) (PageResult[models.Product], error) {
	page, perPage, opts := listOptions(p.Page, p.PerPage)
	rows, total, err := s.products.List(ctx, opts, repository.ProductFilter{
		Search:   p.Search,
		Category: p.Category,
	})
	if err != nil {
		return PageResult[models.Product]{}, wrapErr("list products", err)
	}
	return PageResult[models.Product]{Items: rows, Total: total, Page: page, PerPage: perPage}, nil
}

// GetProduct returns a single product or ErrNotFound.
func (s *DataService) GetProduct(ctx context.Context, id uuid.UUID) (models.Product, error) {
	row, err := s.products.GetByID(ctx, id)
	if err != nil {
		return models.Product{}, wrapErr("get product", err)
	}
	return *row, nil
}

// ListOrders returns a filtered page of orders (without items).
func (s *DataService) ListOrders(ctx context.Context, p OrderListParams) (PageResult[models.Order], error) {
	page, perPage, opts := listOptions(p.Page, p.PerPage)
	rows, total, err := s.orders.List(ctx, opts, repository.OrderFilter{
		CustomerID: p.CustomerID,
		Status:     p.Status,
		DateFrom:   p.DateFrom,
		DateTo:     p.DateTo,
	})
	if err != nil {
		return PageResult[models.Order]{}, wrapErr("list orders", err)
	}
	return PageResult[models.Order]{Items: rows, Total: total, Page: page, PerPage: perPage}, nil
}

// GetOrder returns a single order including its items, or ErrNotFound.
func (s *DataService) GetOrder(ctx context.Context, id uuid.UUID) (models.Order, error) {
	row, err := s.orders.GetByID(ctx, id)
	if err != nil {
		return models.Order{}, wrapErr("get order", err)
	}
	return *row, nil
}

// ListOrderItems returns a filtered page of order items.
func (s *DataService) ListOrderItems(ctx context.Context, p OrderItemListParams) (PageResult[models.OrderItem], error) {
	page, perPage, opts := listOptions(p.Page, p.PerPage)
	rows, total, err := s.orderItems.List(ctx, opts, repository.OrderItemFilter{
		OrderID:   p.OrderID,
		ProductID: p.ProductID,
	})
	if err != nil {
		return PageResult[models.OrderItem]{}, wrapErr("list order items", err)
	}
	return PageResult[models.OrderItem]{Items: rows, Total: total, Page: page, PerPage: perPage}, nil
}

// GetOrderItem returns a single order item or ErrNotFound.
func (s *DataService) GetOrderItem(ctx context.Context, id int64) (models.OrderItem, error) {
	row, err := s.orderItems.GetByID(ctx, id)
	if err != nil {
		return models.OrderItem{}, wrapErr("get order item", err)
	}
	return *row, nil
}
