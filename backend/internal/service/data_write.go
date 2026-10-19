package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
)

var (
	// ErrConflict is returned when a unique constraint is violated.
	ErrConflict = errors.New("record conflict")
	// ErrInvalidReference is returned when a foreign key constraint is violated.
	ErrInvalidReference = errors.New("invalid reference")
)

// DTOs for write operations.

type CustomerInput struct {
	FirstName   string  `json:"first_name" binding:"required"`
	LastName    string  `json:"last_name" binding:"required"`
	Email       string  `json:"email" binding:"required,email"`
	CountryCode string  `json:"country_code" binding:"required,len=2"`
	City        string  `json:"city" binding:"required"`
	State       *string `json:"state"`
}

type ProductInput struct {
	SKU      string  `json:"sku" binding:"required"`
	Name     string  `json:"name" binding:"required"`
	Category string  `json:"category" binding:"required"`
	Price    float64 `json:"price" binding:"gte=0"`
	Stock    int     `json:"stock" binding:"gte=0"`
	Status   string  `json:"status"`
}

type OrderItemInput struct {
	ProductID uuid.UUID `json:"product_id" binding:"required"`
	Quantity  int       `json:"quantity" binding:"gt=0"`
	UnitPrice float64   `json:"unit_price" binding:"gte=0"`
}

type OrderCreateInput struct {
	OrderNumber     string           `json:"order_number"`
	CustomerID      uuid.UUID        `json:"customer_id" binding:"required"`
	Status          string           `json:"status"`
	Currency        string           `json:"currency"`
	ShippingAddress json.RawMessage  `json:"shipping_address" swaggertype:"object"`
	OrderDate       *time.Time       `json:"order_date"`
	Items           []OrderItemInput `json:"items"`
}

type OrderUpdateInput struct {
	Status          *string          `json:"status"`
	Currency        *string          `json:"currency"`
	ShippingAddress *json.RawMessage `json:"shipping_address" swaggertype:"object"`
	OrderDate       *time.Time       `json:"order_date"`
}

func round2(v float64) float64 {
	return math.Round(v*100.0) / 100.0
}

func wrapWriteErr(op string, err error) error {
	if err == nil {
		return nil
	}
	msg := "service: " + op
	if errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("%s: %w", msg, ErrNotFound)
	}
	if errors.Is(err, repository.ErrConflict) {
		return fmt.Errorf("%s: %w", msg, ErrConflict)
	}
	if errors.Is(err, repository.ErrInvalidReference) {
		return fmt.Errorf("%s: %w", msg, ErrInvalidReference)
	}
	return fmt.Errorf("%s: %w", msg, err)
}

func normalizeCustomer(in CustomerInput) models.Customer {
	return models.Customer{
		FirstName:   strings.TrimSpace(in.FirstName),
		LastName:    strings.TrimSpace(in.LastName),
		Email:       strings.ToLower(strings.TrimSpace(in.Email)),
		CountryCode: strings.ToUpper(strings.TrimSpace(in.CountryCode)),
		City:        strings.TrimSpace(in.City),
		State:       in.State,
	}
}

func normalizeProduct(in ProductInput) models.Product {
	status := strings.ToLower(strings.TrimSpace(in.Status))
	if status == "" {
		status = "active"
	}
	return models.Product{
		SKU:      strings.TrimSpace(in.SKU),
		Name:     strings.TrimSpace(in.Name),
		Category: strings.TrimSpace(in.Category),
		Price:    round2(in.Price),
		Stock:    in.Stock,
		Status:   status,
	}
}

// Customer write methods
func (s *DataService) CreateCustomer(ctx context.Context, in CustomerInput) (models.Customer, error) {
	c := normalizeCustomer(in)
	c.ID = uuid.New()
	if err := s.customers.Create(ctx, &c); err != nil {
		return models.Customer{}, wrapWriteErr("create customer", err)
	}
	return c, nil
}

func (s *DataService) UpdateCustomer(ctx context.Context, id uuid.UUID, in CustomerInput) (models.Customer, error) {
	c := normalizeCustomer(in)
	c.ID = id
	if err := s.customers.Update(ctx, &c); err != nil {
		return models.Customer{}, wrapWriteErr("update customer", err)
	}
	return c, nil
}

func (s *DataService) DeleteCustomer(ctx context.Context, id uuid.UUID) error {
	if err := s.customers.Delete(ctx, id); err != nil {
		return wrapWriteErr("delete customer", err)
	}
	return nil
}

// Product write methods
func (s *DataService) CreateProduct(ctx context.Context, in ProductInput) (models.Product, error) {
	p := normalizeProduct(in)
	p.ID = uuid.New()
	if err := s.products.Create(ctx, &p); err != nil {
		return models.Product{}, wrapWriteErr("create product", err)
	}
	return p, nil
}

func (s *DataService) UpdateProduct(ctx context.Context, id uuid.UUID, in ProductInput) (models.Product, error) {
	p := normalizeProduct(in)
	p.ID = id
	if err := s.products.Update(ctx, &p); err != nil {
		return models.Product{}, wrapWriteErr("update product", err)
	}
	return p, nil
}

func (s *DataService) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	if err := s.products.Delete(ctx, id); err != nil {
		return wrapWriteErr("delete product", err)
	}
	return nil
}

// Order write methods
func (s *DataService) CreateOrder(ctx context.Context, in OrderCreateInput) (models.Order, error) {
	status := strings.ToLower(strings.TrimSpace(in.Status))
	if status == "" {
		status = "pending"
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = "USD"
	}

	orderNumber := strings.TrimSpace(in.OrderNumber)
	if orderNumber == "" {
		orderNumber = fmt.Sprintf("ORD-%d", time.Now().UnixNano())
	}

	orderDate := time.Now().UTC()
	if in.OrderDate != nil {
		orderDate = in.OrderDate.UTC()
	}

	items := make([]models.OrderItem, 0, len(in.Items))
	total := 0.0
	for _, it := range in.Items {
		sub := float64(it.Quantity) * it.UnitPrice
		items = append(items, models.OrderItem{
			ProductID: it.ProductID,
			Quantity:  it.Quantity,
			UnitPrice: round2(it.UnitPrice),
			Subtotal:  round2(sub),
		})
		total += sub
	}

	o := models.Order{
		ID:              uuid.New(),
		OrderNumber:     orderNumber,
		CustomerID:      in.CustomerID,
		Status:          status,
		TotalAmount:     round2(total),
		Currency:        currency,
		ShippingAddress: in.ShippingAddress,
		OrderDate:       orderDate,
		OrderItems:      items,
	}

	if err := s.orders.Create(ctx, &o); err != nil {
		return models.Order{}, wrapWriteErr("create order", err)
	}
	return o, nil
}

func (s *DataService) UpdateOrder(ctx context.Context, id uuid.UUID, in OrderUpdateInput) (models.Order, error) {
	o := models.Order{ID: id}
	changed := false

	if in.Status != nil {
		o.Status = strings.ToLower(strings.TrimSpace(*in.Status))
		changed = true
	}
	if in.Currency != nil {
		o.Currency = strings.ToUpper(strings.TrimSpace(*in.Currency))
		changed = true
	}
	if in.ShippingAddress != nil {
		o.ShippingAddress = *in.ShippingAddress
		changed = true
	}
	if in.OrderDate != nil {
		o.OrderDate = in.OrderDate.UTC()
		changed = true
	}

	if changed {
		if err := s.orders.Update(ctx, &o); err != nil {
			return models.Order{}, wrapWriteErr("update order", err)
		}
	}

	row, err := s.orders.GetByID(ctx, id)
	if err != nil {
		return models.Order{}, wrapWriteErr("get order", err)
	}
	return *row, nil
}

func (s *DataService) DeleteOrder(ctx context.Context, id uuid.UUID) error {
	if err := s.orders.Delete(ctx, id); err != nil {
		return wrapWriteErr("delete order", err)
	}
	return nil
}
