package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// writeService builds a DataService whose repos are the given fakes (nil fakes
// are replaced by empty ones; the order-item repo is never used on writes).
func writeService(customers *fakeCustomerRepo, products *fakeProductRepo, orders *fakeOrderRepo) *service.DataService {
	if customers == nil {
		customers = &fakeCustomerRepo{}
	}
	if products == nil {
		products = &fakeProductRepo{}
	}
	if orders == nil {
		orders = &fakeOrderRepo{}
	}
	return service.NewDataService(customers, products, orders, &fakeOrderItemRepo{})
}

func TestCreateCustomerNormalizes(t *testing.T) {
	var got models.Customer
	svc := writeService(&fakeCustomerRepo{
		create: func(_ context.Context, m *models.Customer) error {
			got = *m
			return nil
		},
	}, nil, nil)

	out, err := svc.CreateCustomer(context.Background(), service.CustomerInput{
		FirstName:   "  Ada  ",
		LastName:    "Lovelace",
		Email:       "ADA@Example.COM",
		CountryCode: "gb",
		City:        "London",
	})
	if err != nil {
		t.Fatalf("CreateCustomer: %v", err)
	}
	if got.Email != "ada@example.com" {
		t.Errorf("email not lowercased: %q", got.Email)
	}
	if got.CountryCode != "GB" {
		t.Errorf("country not uppercased: %q", got.CountryCode)
	}
	if got.FirstName != "Ada" {
		t.Errorf("first name not trimmed: %q", got.FirstName)
	}
	if out.ID == uuid.Nil {
		t.Error("expected generated id")
	}
}

func TestCreateCustomerConflict(t *testing.T) {
	svc := writeService(&fakeCustomerRepo{
		create: func(context.Context, *models.Customer) error { return repository.ErrConflict },
	}, nil, nil)

	_, err := svc.CreateCustomer(context.Background(), service.CustomerInput{Email: "a@b.com"})
	if !errors.Is(err, service.ErrConflict) {
		t.Fatalf("expected service.ErrConflict, got %v", err)
	}
}

func TestDeleteCustomerNotFound(t *testing.T) {
	svc := writeService(&fakeCustomerRepo{
		delete: func(context.Context, uuid.UUID) error { return repository.ErrNotFound },
	}, nil, nil)

	if err := svc.DeleteCustomer(context.Background(), uuid.New()); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected service.ErrNotFound, got %v", err)
	}
}

func TestCreateProductDefaultsAndRounds(t *testing.T) {
	var got models.Product
	svc := writeService(nil, &fakeProductRepo{
		create: func(_ context.Context, m *models.Product) error {
			got = *m
			return nil
		},
	}, nil)

	out, err := svc.CreateProduct(context.Background(), service.ProductInput{
		SKU: "SKU-1", Name: "Widget", Category: "tools", Price: 9.999, Stock: 0,
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	if got.Status != "active" {
		t.Errorf("status = %q, want active", got.Status)
	}
	if got.Price != 10.0 {
		t.Errorf("price = %v, want 10.0 (rounded)", got.Price)
	}
	if got.Stock != 0 {
		t.Errorf("stock = %d, want 0", got.Stock)
	}
	if out.ID == uuid.Nil {
		t.Error("expected generated id")
	}
}

func TestCreateOrderComputesTotals(t *testing.T) {
	var got models.Order
	customerID := uuid.New()
	svc := writeService(nil, nil, &fakeOrderRepo{
		create: func(_ context.Context, m *models.Order) error {
			got = *m
			return nil
		},
	})

	out, err := svc.CreateOrder(context.Background(), service.OrderCreateInput{
		CustomerID: customerID,
		Items: []service.OrderItemInput{
			{ProductID: uuid.New(), Quantity: 2, UnitPrice: 9.99},
			{ProductID: uuid.New(), Quantity: 1, UnitPrice: 5.01},
		},
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if got.OrderNumber == "" {
		t.Error("expected generated order_number")
	}
	if got.TotalAmount != 24.99 {
		t.Errorf("total = %v, want 24.99", got.TotalAmount)
	}
	if len(got.OrderItems) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got.OrderItems))
	}
	if got.OrderItems[0].Subtotal != 19.98 {
		t.Errorf("item[0].subtotal = %v, want 19.98", got.OrderItems[0].Subtotal)
	}
	if got.OrderItems[1].Subtotal != 5.01 {
		t.Errorf("item[1].subtotal = %v, want 5.01", got.OrderItems[1].Subtotal)
	}
	if got.Status != "pending" || got.Currency != "USD" {
		t.Errorf("unexpected defaults: status=%q currency=%q", got.Status, got.Currency)
	}
	if got.CustomerID != customerID {
		t.Errorf("customer id = %s, want %s", got.CustomerID, customerID)
	}
	_ = out
}

func TestCreateOrderInvalidReference(t *testing.T) {
	svc := writeService(nil, nil, &fakeOrderRepo{
		create: func(context.Context, *models.Order) error { return repository.ErrInvalidReference },
	})
	_, err := svc.CreateOrder(context.Background(), service.OrderCreateInput{CustomerID: uuid.New()})
	if !errors.Is(err, service.ErrInvalidReference) {
		t.Fatalf("expected service.ErrInvalidReference, got %v", err)
	}
}

func TestUpdateOrderPartialAndReturnsPersisted(t *testing.T) {
	id := uuid.New()
	status := "SHIPPED"
	var got models.Order

	svc := writeService(nil, nil, &fakeOrderRepo{
		update: func(_ context.Context, m *models.Order) error {
			got = *m
			return nil
		},
		get: func(_ context.Context, inID uuid.UUID) (*models.Order, error) {
			return &models.Order{ID: inID, Status: "shipped", OrderItems: []models.OrderItem{{ID: 1}}}, nil
		},
	})

	out, err := svc.UpdateOrder(context.Background(), id, service.OrderUpdateInput{Status: &status})
	if err != nil {
		t.Fatalf("UpdateOrder: %v", err)
	}
	if got.ID != id {
		t.Errorf("update id = %s, want %s", got.ID, id)
	}
	if got.Status != "shipped" {
		t.Errorf("status not normalized: %q", got.Status)
	}
	if got.Currency != "" {
		t.Errorf("currency should be untouched when not provided, got %q", got.Currency)
	}
	if len(out.OrderItems) != 1 {
		t.Errorf("expected persisted order with items, got %+v", out)
	}
}

func TestUpdateOrderNoChangesFetchesExisting(t *testing.T) {
	id := uuid.New()
	updateCalled := false

	svc := writeService(nil, nil, &fakeOrderRepo{
		update: func(context.Context, *models.Order) error {
			updateCalled = true
			return nil
		},
		get: func(_ context.Context, inID uuid.UUID) (*models.Order, error) {
			return &models.Order{ID: inID, Status: "pending"}, nil
		},
	})

	out, err := svc.UpdateOrder(context.Background(), id, service.OrderUpdateInput{})
	if err != nil {
		t.Fatalf("UpdateOrder: %v", err)
	}
	if updateCalled {
		t.Error("expected repo.Update not to be called for an empty update")
	}
	if out.Status != "pending" {
		t.Errorf("expected existing order, got %+v", out)
	}
}

func TestDeleteOrderNotFound(t *testing.T) {
	svc := writeService(nil, nil, &fakeOrderRepo{
		delete: func(context.Context, uuid.UUID) error { return repository.ErrNotFound },
	})
	if err := svc.DeleteOrder(context.Background(), uuid.New()); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected service.ErrNotFound, got %v", err)
	}
}
