package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// --- function-field fakes -------------------------------------------------

type fakeCustomerRepo struct {
	list   func(context.Context, repository.ListOptions, repository.CustomerFilter) ([]models.Customer, int64, error)
	get    func(context.Context, uuid.UUID) (*models.Customer, error)
	create func(context.Context, *models.Customer) error
	update func(context.Context, *models.Customer) error
	delete func(context.Context, uuid.UUID) error
}

func (f *fakeCustomerRepo) List(ctx context.Context, o repository.ListOptions, flt repository.CustomerFilter) ([]models.Customer, int64, error) {
	return f.list(ctx, o, flt)
}
func (f *fakeCustomerRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Customer, error) {
	return f.get(ctx, id)
}
func (f *fakeCustomerRepo) Create(ctx context.Context, m *models.Customer) error {
	if f.create != nil {
		return f.create(ctx, m)
	}
	return nil
}
func (f *fakeCustomerRepo) Update(ctx context.Context, m *models.Customer) error {
	if f.update != nil {
		return f.update(ctx, m)
	}
	return nil
}
func (f *fakeCustomerRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if f.delete != nil {
		return f.delete(ctx, id)
	}
	return nil
}

type fakeProductRepo struct {
	list   func(context.Context, repository.ListOptions, repository.ProductFilter) ([]models.Product, int64, error)
	get    func(context.Context, uuid.UUID) (*models.Product, error)
	create func(context.Context, *models.Product) error
	update func(context.Context, *models.Product) error
	delete func(context.Context, uuid.UUID) error
}

func (f *fakeProductRepo) List(ctx context.Context, o repository.ListOptions, flt repository.ProductFilter) ([]models.Product, int64, error) {
	return f.list(ctx, o, flt)
}
func (f *fakeProductRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Product, error) {
	return f.get(ctx, id)
}
func (f *fakeProductRepo) Create(ctx context.Context, m *models.Product) error {
	if f.create != nil {
		return f.create(ctx, m)
	}
	return nil
}
func (f *fakeProductRepo) Update(ctx context.Context, m *models.Product) error {
	if f.update != nil {
		return f.update(ctx, m)
	}
	return nil
}
func (f *fakeProductRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if f.delete != nil {
		return f.delete(ctx, id)
	}
	return nil
}

type fakeOrderRepo struct {
	list   func(context.Context, repository.ListOptions, repository.OrderFilter) ([]models.Order, int64, error)
	get    func(context.Context, uuid.UUID) (*models.Order, error)
	create func(context.Context, *models.Order) error
	update func(context.Context, *models.Order) error
	delete func(context.Context, uuid.UUID) error
}

func (f *fakeOrderRepo) List(ctx context.Context, o repository.ListOptions, flt repository.OrderFilter) ([]models.Order, int64, error) {
	return f.list(ctx, o, flt)
}
func (f *fakeOrderRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Order, error) {
	return f.get(ctx, id)
}
func (f *fakeOrderRepo) Create(ctx context.Context, m *models.Order) error {
	if f.create != nil {
		return f.create(ctx, m)
	}
	return nil
}
func (f *fakeOrderRepo) Update(ctx context.Context, m *models.Order) error {
	if f.update != nil {
		return f.update(ctx, m)
	}
	return nil
}
func (f *fakeOrderRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if f.delete != nil {
		return f.delete(ctx, id)
	}
	return nil
}

type fakeOrderItemRepo struct {
	list func(context.Context, repository.ListOptions, repository.OrderItemFilter) ([]models.OrderItem, int64, error)
	get  func(context.Context, int64) (*models.OrderItem, error)
}

func (f *fakeOrderItemRepo) List(ctx context.Context, o repository.ListOptions, flt repository.OrderItemFilter) ([]models.OrderItem, int64, error) {
	return f.list(ctx, o, flt)
}
func (f *fakeOrderItemRepo) GetByID(ctx context.Context, id int64) (*models.OrderItem, error) {
	return f.get(ctx, id)
}

func nopService(t *testing.T) *service.DataService {
	t.Helper()
	unused := func(t *testing.T) {}
	_ = unused
	return service.NewDataService(
		&fakeCustomerRepo{
			list: func(context.Context, repository.ListOptions, repository.CustomerFilter) ([]models.Customer, int64, error) {
				t.Fatal("customers.List must not be called")
				return nil, 0, nil
			},
			get: func(context.Context, uuid.UUID) (*models.Customer, error) {
				t.Fatal("customers.GetByID must not be called")
				return nil, nil
			},
		},
		&fakeProductRepo{
			list: func(context.Context, repository.ListOptions, repository.ProductFilter) ([]models.Product, int64, error) {
				t.Fatal("products.List must not be called")
				return nil, 0, nil
			},
			get: func(context.Context, uuid.UUID) (*models.Product, error) {
				t.Fatal("products.GetByID must not be called")
				return nil, nil
			},
		},
		&fakeOrderRepo{
			list: func(context.Context, repository.ListOptions, repository.OrderFilter) ([]models.Order, int64, error) {
				t.Fatal("orders.List must not be called")
				return nil, 0, nil
			},
			get: func(context.Context, uuid.UUID) (*models.Order, error) {
				t.Fatal("orders.GetByID must not be called")
				return nil, nil
			},
		},
		&fakeOrderItemRepo{
			list: func(context.Context, repository.ListOptions, repository.OrderItemFilter) ([]models.OrderItem, int64, error) {
				t.Fatal("orderItems.List must not be called")
				return nil, 0, nil
			},
			get: func(context.Context, int64) (*models.OrderItem, error) {
				t.Fatal("orderItems.GetByID must not be called")
				return nil, nil
			},
		},
	)
}

// --- tests ----------------------------------------------------------------

func TestDataServiceListCustomersPageMathAndFilters(t *testing.T) {
	var gotOpts repository.ListOptions
	var gotFilter repository.CustomerFilter
	svc := service.NewDataService(
		&fakeCustomerRepo{
			list: func(_ context.Context, o repository.ListOptions, f repository.CustomerFilter) ([]models.Customer, int64, error) {
				gotOpts, gotFilter = o, f
				return []models.Customer{{FirstName: "Ada"}}, 42, nil
			},
			get: func(context.Context, uuid.UUID) (*models.Customer, error) { return nil, nil },
		},
		&fakeProductRepo{}, &fakeOrderRepo{}, &fakeOrderItemRepo{},
	)
	// The unused repos must not be exercised, so give them panicking bodies.

	res, err := svc.ListCustomers(context.Background(), service.CustomerListParams{
		Page: 3, PerPage: 25, Search: "ada", Country: "us",
	})
	if err != nil {
		t.Fatalf("ListCustomers: %v", err)
	}
	if gotOpts.Limit != 25 || gotOpts.Offset != 50 {
		t.Errorf("ListOptions = %+v, want limit 25 offset 50", gotOpts)
	}
	if gotFilter.Search != "ada" || gotFilter.Country != "us" {
		t.Errorf("filter = %+v, want search=ada country=us", gotFilter)
	}
	if res.Page != 3 || res.PerPage != 25 {
		t.Errorf("result paging = %d/%d, want 3/25", res.Page, res.PerPage)
	}
	if res.Total != 42 || len(res.Items) != 1 {
		t.Errorf("result = %+v, want total 42 with 1 item", res)
	}
	if res.TotalPages() != 2 {
		t.Errorf("TotalPages = %d, want 2", res.TotalPages())
	}
}

func TestDataServicePagingDefaultsAndClamp(t *testing.T) {
	cases := []struct {
		name              string
		page, perPage     int
		wantPage, wantPer int
		wantOffset        int
	}{
		{"zero defaults", 0, 0, 1, 20, 0},
		{"negative defaults", -4, -9, 1, 20, 0},
		{"per_page clamped", 1, 5000, 1, 100, 0},
		{"page 2", 2, 10, 2, 10, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotOpts repository.ListOptions
			svc := service.NewDataService(
				&fakeCustomerRepo{
					list: func(_ context.Context, o repository.ListOptions, _ repository.CustomerFilter) ([]models.Customer, int64, error) {
						gotOpts = o
						return []models.Customer{}, 0, nil
					},
					get: func(context.Context, uuid.UUID) (*models.Customer, error) { return nil, nil },
				},
				&fakeProductRepo{}, &fakeOrderRepo{}, &fakeOrderItemRepo{},
			)

			res, err := svc.ListCustomers(context.Background(), service.CustomerListParams{Page: tc.page, PerPage: tc.perPage})
			if err != nil {
				t.Fatalf("ListCustomers: %v", err)
			}
			if res.Page != tc.wantPage || res.PerPage != tc.wantPer {
				t.Errorf("paging = %d/%d, want %d/%d", res.Page, res.PerPage, tc.wantPage, tc.wantPer)
			}
			if gotOpts.Limit != tc.wantPer || gotOpts.Offset != tc.wantOffset {
				t.Errorf("ListOptions = %+v, want limit %d offset %d", gotOpts, tc.wantPer, tc.wantOffset)
			}
		})
	}
}

func TestDataServiceListCustomersError(t *testing.T) {
	boom := errors.New("db down")
	svc := service.NewDataService(
		&fakeCustomerRepo{
			list: func(context.Context, repository.ListOptions, repository.CustomerFilter) ([]models.Customer, int64, error) {
				return nil, 0, boom
			},
			get: func(context.Context, uuid.UUID) (*models.Customer, error) { return nil, nil },
		},
		&fakeProductRepo{}, &fakeOrderRepo{}, &fakeOrderItemRepo{},
	)

	_, err := svc.ListCustomers(context.Background(), service.CustomerListParams{})
	if !errors.Is(err, boom) {
		t.Fatalf("expected wrapped cause, got %v", err)
	}
	if !strings.Contains(err.Error(), "list customers") {
		t.Errorf("expected operation context, got %v", err)
	}
}

func TestDataServiceGetCustomer(t *testing.T) {
	id := uuid.New()

	t.Run("ok", func(t *testing.T) {
		want := models.Customer{ID: id, Email: "a@b.c"}
		svc := service.NewDataService(
			&fakeCustomerRepo{
				list: func(context.Context, repository.ListOptions, repository.CustomerFilter) ([]models.Customer, int64, error) {
					return nil, 0, nil
				},
				get: func(_ context.Context, got uuid.UUID) (*models.Customer, error) {
					if got != id {
						t.Errorf("id = %s, want %s", got, id)
					}
					return &want, nil
				},
			},
			&fakeProductRepo{}, &fakeOrderRepo{}, &fakeOrderItemRepo{},
		)

		row, err := svc.GetCustomer(context.Background(), id)
		if err != nil {
			t.Fatalf("GetCustomer: %v", err)
		}
		if row.ID != id || row.Email != "a@b.c" {
			t.Errorf("unexpected row: %+v", row)
		}
	})

	t.Run("not found translated", func(t *testing.T) {
		svc := service.NewDataService(
			&fakeCustomerRepo{
				list: func(context.Context, repository.ListOptions, repository.CustomerFilter) ([]models.Customer, int64, error) {
					return nil, 0, nil
				},
				get: func(context.Context, uuid.UUID) (*models.Customer, error) {
					return nil, repository.ErrNotFound
				},
			},
			&fakeProductRepo{}, &fakeOrderRepo{}, &fakeOrderItemRepo{},
		)

		_, err := svc.GetCustomer(context.Background(), id)
		if !errors.Is(err, service.ErrNotFound) {
			t.Fatalf("expected service.ErrNotFound, got %v", err)
		}
		if !strings.Contains(err.Error(), "get customer") {
			t.Errorf("expected operation context, got %v", err)
		}
	})

	t.Run("generic error preserved", func(t *testing.T) {
		boom := errors.New("boom")
		svc := service.NewDataService(
			&fakeCustomerRepo{
				list: func(context.Context, repository.ListOptions, repository.CustomerFilter) ([]models.Customer, int64, error) {
					return nil, 0, nil
				},
				get: func(context.Context, uuid.UUID) (*models.Customer, error) { return nil, boom },
			},
			&fakeProductRepo{}, &fakeOrderRepo{}, &fakeOrderItemRepo{},
		)

		_, err := svc.GetCustomer(context.Background(), id)
		if !errors.Is(err, boom) {
			t.Fatalf("expected wrapped cause, got %v", err)
		}
		if errors.Is(err, service.ErrNotFound) {
			t.Errorf("unexpected ErrNotFound: %v", err)
		}
	})
}

func TestDataServiceProducts(t *testing.T) {
	var gotOpts repository.ListOptions
	var gotFilter repository.ProductFilter
	productID := uuid.New()
	svc := service.NewDataService(
		&fakeCustomerRepo{},
		&fakeProductRepo{
			list: func(_ context.Context, o repository.ListOptions, f repository.ProductFilter) ([]models.Product, int64, error) {
				gotOpts, gotFilter = o, f
				return []models.Product{{ID: productID, SKU: "SKU-1"}}, 7, nil
			},
			get: func(_ context.Context, id uuid.UUID) (*models.Product, error) {
				return &models.Product{ID: id, SKU: "SKU-1"}, nil
			},
		},
		&fakeOrderRepo{}, &fakeOrderItemRepo{},
	)

	res, err := svc.ListProducts(context.Background(), service.ProductListParams{
		Page: 2, PerPage: 10, Search: "widget", Category: "Electronics",
	})
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if gotOpts.Limit != 10 || gotOpts.Offset != 10 {
		t.Errorf("ListOptions = %+v, want limit 10 offset 10", gotOpts)
	}
	if gotFilter.Search != "widget" || gotFilter.Category != "Electronics" {
		t.Errorf("filter = %+v", gotFilter)
	}
	if res.Total != 7 || res.TotalPages() != 1 {
		t.Errorf("result = %+v", res)
	}

	row, err := svc.GetProduct(context.Background(), productID)
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	if row.ID != productID {
		t.Errorf("unexpected product: %+v", row)
	}
}

func TestDataServiceListProductsError(t *testing.T) {
	boom := errors.New("boom")
	svc := service.NewDataService(
		&fakeCustomerRepo{},
		&fakeProductRepo{
			list: func(context.Context, repository.ListOptions, repository.ProductFilter) ([]models.Product, int64, error) {
				return nil, 0, boom
			},
		},
		&fakeOrderRepo{}, &fakeOrderItemRepo{},
	)

	_, err := svc.ListProducts(context.Background(), service.ProductListParams{})
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "list products") {
		t.Fatalf("expected wrapped list products error, got %v", err)
	}
}

func TestDataServiceGetProductNotFound(t *testing.T) {
	svc := service.NewDataService(
		&fakeCustomerRepo{},
		&fakeProductRepo{
			get: func(context.Context, uuid.UUID) (*models.Product, error) { return nil, repository.ErrNotFound },
		},
		&fakeOrderRepo{}, &fakeOrderItemRepo{},
	)

	_, err := svc.GetProduct(context.Background(), uuid.New())
	if !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDataServiceOrders(t *testing.T) {
	var gotOpts repository.ListOptions
	var gotFilter repository.OrderFilter
	customerID := uuid.New()
	orderID := uuid.New()
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	svc := service.NewDataService(
		&fakeCustomerRepo{}, &fakeProductRepo{},
		&fakeOrderRepo{
			list: func(_ context.Context, o repository.ListOptions, f repository.OrderFilter) ([]models.Order, int64, error) {
				gotOpts, gotFilter = o, f
				return []models.Order{{ID: orderID, OrderNumber: "ORD-1"}}, 3, nil
			},
			get: func(_ context.Context, id uuid.UUID) (*models.Order, error) {
				return &models.Order{
					ID:         id,
					OrderItems: []models.OrderItem{{ID: 1, OrderID: id}},
				}, nil
			},
		},
		&fakeOrderItemRepo{},
	)

	res, err := svc.ListOrders(context.Background(), service.OrderListParams{
		Page: 1, PerPage: 50, CustomerID: &customerID, Status: "pending", DateFrom: &from, DateTo: &to,
	})
	if err != nil {
		t.Fatalf("ListOrders: %v", err)
	}
	if gotOpts.Limit != 50 || gotOpts.Offset != 0 {
		t.Errorf("ListOptions = %+v", gotOpts)
	}
	if gotFilter.CustomerID == nil || *gotFilter.CustomerID != customerID {
		t.Errorf("customer filter = %+v", gotFilter.CustomerID)
	}
	if gotFilter.Status != "pending" || gotFilter.DateFrom == nil || gotFilter.DateTo == nil {
		t.Errorf("filter = %+v", gotFilter)
	}
	if res.Total != 3 {
		t.Errorf("total = %d, want 3", res.Total)
	}

	order, err := svc.GetOrder(context.Background(), orderID)
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if len(order.OrderItems) != 1 {
		t.Errorf("expected preloaded items, got %+v", order.OrderItems)
	}
}

func TestDataServiceListOrdersErrorAndNotFound(t *testing.T) {
	boom := errors.New("boom")
	svc := service.NewDataService(
		&fakeCustomerRepo{}, &fakeProductRepo{},
		&fakeOrderRepo{
			list: func(context.Context, repository.ListOptions, repository.OrderFilter) ([]models.Order, int64, error) {
				return nil, 0, boom
			},
			get: func(context.Context, uuid.UUID) (*models.Order, error) { return nil, repository.ErrNotFound },
		},
		&fakeOrderItemRepo{},
	)

	if _, err := svc.ListOrders(context.Background(), service.OrderListParams{}); !errors.Is(err, boom) {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
	if _, err := svc.GetOrder(context.Background(), uuid.New()); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDataServiceOrderItems(t *testing.T) {
	var gotFilter repository.OrderItemFilter
	orderID := uuid.New()
	productID := uuid.New()

	svc := service.NewDataService(
		&fakeCustomerRepo{}, &fakeProductRepo{}, &fakeOrderRepo{},
		&fakeOrderItemRepo{
			list: func(_ context.Context, _ repository.ListOptions, f repository.OrderItemFilter) ([]models.OrderItem, int64, error) {
				gotFilter = f
				return []models.OrderItem{{ID: 9}}, 1, nil
			},
			get: func(_ context.Context, id int64) (*models.OrderItem, error) {
				return &models.OrderItem{ID: id}, nil
			},
		},
	)

	res, err := svc.ListOrderItems(context.Background(), service.OrderItemListParams{
		Page: 1, PerPage: 5, OrderID: &orderID, ProductID: &productID,
	})
	if err != nil {
		t.Fatalf("ListOrderItems: %v", err)
	}
	if gotFilter.OrderID == nil || *gotFilter.OrderID != orderID {
		t.Errorf("order filter = %+v", gotFilter.OrderID)
	}
	if gotFilter.ProductID == nil || *gotFilter.ProductID != productID {
		t.Errorf("product filter = %+v", gotFilter.ProductID)
	}
	if res.Total != 1 || res.TotalPages() != 1 {
		t.Errorf("result = %+v", res)
	}

	item, err := svc.GetOrderItem(context.Background(), 9)
	if err != nil {
		t.Fatalf("GetOrderItem: %v", err)
	}
	if item.ID != 9 {
		t.Errorf("unexpected item: %+v", item)
	}
}

func TestDataServiceOrderItemsErrors(t *testing.T) {
	boom := errors.New("boom")
	svc := service.NewDataService(
		&fakeCustomerRepo{}, &fakeProductRepo{}, &fakeOrderRepo{},
		&fakeOrderItemRepo{
			list: func(context.Context, repository.ListOptions, repository.OrderItemFilter) ([]models.OrderItem, int64, error) {
				return nil, 0, boom
			},
			get: func(context.Context, int64) (*models.OrderItem, error) { return nil, repository.ErrNotFound },
		},
	)

	if _, err := svc.ListOrderItems(context.Background(), service.OrderItemListParams{}); !errors.Is(err, boom) {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
	if _, err := svc.GetOrderItem(context.Background(), 1); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPageResultTotalPages(t *testing.T) {
	cases := []struct {
		total, perPage, want int
	}{
		{0, 20, 0},
		{-1, 20, 0},
		{20, 20, 1},
		{21, 20, 2},
		{100, 25, 4},
	}
	for _, tc := range cases {
		r := service.PageResult[int]{Total: int64(tc.total), PerPage: tc.perPage}
		if got := r.TotalPages(); got != tc.want {
			t.Errorf("TotalPages(total=%d, perPage=%d) = %d, want %d", tc.total, tc.perPage, got, tc.want)
		}
	}
}

// nopService ensures the fully-stubbed constructor wiring compiles and panics
// only when an unexpected repository method is invoked.
func TestNopServiceWiring(t *testing.T) {
	svc := nopService(t)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

var _ service.SourceData = (*service.DataService)(nil)
