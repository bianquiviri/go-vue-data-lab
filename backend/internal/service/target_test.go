package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// fakeAnalyticsRepo implements repository.AnalyticsRepository with
// function fields for service tests.
type fakeAnalyticsRepo struct {
	summary      func(context.Context) (models.AnalyticsSummary, error)
	topProducts  func(context.Context, int) ([]models.TopProduct, error)
	topCustomers func(context.Context, int) ([]models.TopCustomer, error)
}

func (f *fakeAnalyticsRepo) Summary(ctx context.Context) (models.AnalyticsSummary, error) {
	return f.summary(ctx)
}
func (f *fakeAnalyticsRepo) TopProducts(ctx context.Context, limit int) ([]models.TopProduct, error) {
	return f.topProducts(ctx, limit)
}
func (f *fakeAnalyticsRepo) TopCustomers(ctx context.Context, limit int) ([]models.TopCustomer, error) {
	return f.topCustomers(ctx, limit)
}

var _ repository.AnalyticsRepository = (*fakeAnalyticsRepo)(nil)

// nopAnalytics fails the test if any analytics method is called.
func nopAnalytics(t *testing.T) *fakeAnalyticsRepo {
	t.Helper()
	return &fakeAnalyticsRepo{
		summary: func(context.Context) (models.AnalyticsSummary, error) {
			t.Fatal("analytics.Summary must not be called")
			return models.AnalyticsSummary{}, nil
		},
		topProducts: func(context.Context, int) ([]models.TopProduct, error) {
			t.Fatal("analytics.TopProducts must not be called")
			return nil, nil
		},
		topCustomers: func(context.Context, int) ([]models.TopCustomer, error) {
			t.Fatal("analytics.TopCustomers must not be called")
			return nil, nil
		},
	}
}

// nopSourceData fails the test if any entity method is called.
func nopSourceData(t *testing.T) service.SourceData {
	t.Helper()
	return &nopEntitySource{t: t}
}

type nopEntitySource struct{ t *testing.T }

func (s *nopEntitySource) ListCustomers(context.Context, service.CustomerListParams) (service.PageResult[models.Customer], error) {
	s.t.Fatal("ListCustomers must not be called")
	return service.PageResult[models.Customer]{}, nil
}
func (s *nopEntitySource) GetCustomer(context.Context, uuid.UUID) (models.Customer, error) {
	s.t.Fatal("GetCustomer must not be called")
	return models.Customer{}, nil
}
func (s *nopEntitySource) ListProducts(context.Context, service.ProductListParams) (service.PageResult[models.Product], error) {
	s.t.Fatal("ListProducts must not be called")
	return service.PageResult[models.Product]{}, nil
}
func (s *nopEntitySource) GetProduct(context.Context, uuid.UUID) (models.Product, error) {
	s.t.Fatal("GetProduct must not be called")
	return models.Product{}, nil
}
func (s *nopEntitySource) ListOrders(context.Context, service.OrderListParams) (service.PageResult[models.Order], error) {
	s.t.Fatal("ListOrders must not be called")
	return service.PageResult[models.Order]{}, nil
}
func (s *nopEntitySource) GetOrder(context.Context, uuid.UUID) (models.Order, error) {
	s.t.Fatal("GetOrder must not be called")
	return models.Order{}, nil
}
func (s *nopEntitySource) ListOrderItems(context.Context, service.OrderItemListParams) (service.PageResult[models.OrderItem], error) {
	s.t.Fatal("ListOrderItems must not be called")
	return service.PageResult[models.OrderItem]{}, nil
}
func (s *nopEntitySource) GetOrderItem(context.Context, int64) (models.OrderItem, error) {
	s.t.Fatal("GetOrderItem must not be called")
	return models.OrderItem{}, nil
}

var _ service.SourceData = (*nopEntitySource)(nil)

func TestNormalizeAnalyticsLimit(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, service.DefaultAnalyticsLimit},
		{-5, service.DefaultAnalyticsLimit},
		{1, 1},
		{10, 10},
		{service.MaxAnalyticsLimit, service.MaxAnalyticsLimit},
		{service.MaxAnalyticsLimit + 1, service.MaxAnalyticsLimit},
		{1000, service.MaxAnalyticsLimit},
	}
	for _, tc := range cases {
		if got := service.NormalizeAnalyticsLimit(tc.in); got != tc.want {
			t.Errorf("NormalizeAnalyticsLimit(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestTargetDataServiceSummary(t *testing.T) {
	want := models.AnalyticsSummary{
		TotalCustomers:    3,
		TotalProducts:     4,
		TotalOrders:       5,
		TotalRevenue:      100.5,
		AverageOrderValue: 20.1,
		OrdersByStatus:    []models.OrderStatusCount{{Status: "paid", Count: 5}},
	}

	t.Run("ok", func(t *testing.T) {
		repo := nopAnalytics(t)
		repo.summary = func(context.Context) (models.AnalyticsSummary, error) {
			return want, nil
		}
		svc := service.NewTargetDataService(nopSourceData(t), repo)

		got, err := svc.Summary(context.Background())
		if err != nil {
			t.Fatalf("summary: %v", err)
		}
		if got.TotalOrders != 5 || got.TotalRevenue != 100.5 || len(got.OrdersByStatus) != 1 {
			t.Errorf("unexpected summary: %+v", got)
		}
	})

	t.Run("error wrapped", func(t *testing.T) {
		repo := nopAnalytics(t)
		boom := errors.New("boom")
		repo.summary = func(context.Context) (models.AnalyticsSummary, error) {
			return models.AnalyticsSummary{}, boom
		}
		svc := service.NewTargetDataService(nopSourceData(t), repo)

		_, err := svc.Summary(context.Background())
		if !errors.Is(err, boom) {
			t.Errorf("expected wrapped boom, got %v", err)
		}
		if !strings.Contains(err.Error(), "analytics summary") {
			t.Errorf("expected operation context, got %v", err)
		}
	})
}

func TestTargetDataServiceTopProductsNormalizesLimit(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"zero falls back to default", 0, service.DefaultAnalyticsLimit},
		{"negative falls back to default", -1, service.DefaultAnalyticsLimit},
		{"in range passes through", 25, 25},
		{"above max is clamped", 500, service.MaxAnalyticsLimit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotLimit int
			repo := nopAnalytics(t)
			repo.topProducts = func(_ context.Context, limit int) ([]models.TopProduct, error) {
				gotLimit = limit
				return []models.TopProduct{{ID: uuid.New(), SKU: "SKU-1"}}, nil
			}
			svc := service.NewTargetDataService(nopSourceData(t), repo)

			rows, err := svc.TopProducts(context.Background(), tc.in)
			if err != nil {
				t.Fatalf("top products: %v", err)
			}
			if gotLimit != tc.want {
				t.Errorf("limit passed to repo = %d, want %d", gotLimit, tc.want)
			}
			if len(rows) != 1 {
				t.Errorf("expected 1 row, got %d", len(rows))
			}
		})
	}
}

func TestTargetDataServiceTopCustomers(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		want := []models.TopCustomer{{ID: uuid.New(), Email: "a@b.c", TotalOrders: 2, TotalSpent: 42}}
		repo := nopAnalytics(t)
		repo.topCustomers = func(_ context.Context, limit int) ([]models.TopCustomer, error) {
			if limit != 3 {
				t.Errorf("limit = %d, want 3", limit)
			}
			return want, nil
		}
		svc := service.NewTargetDataService(nopSourceData(t), repo)

		rows, err := svc.TopCustomers(context.Background(), 3)
		if err != nil {
			t.Fatalf("top customers: %v", err)
		}
		if len(rows) != 1 || rows[0].TotalSpent != 42 {
			t.Errorf("unexpected rows: %+v", rows)
		}
	})

	t.Run("error wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		repo := nopAnalytics(t)
		repo.topCustomers = func(context.Context, int) ([]models.TopCustomer, error) {
			return nil, boom
		}
		svc := service.NewTargetDataService(nopSourceData(t), repo)

		_, err := svc.TopCustomers(context.Background(), 10)
		if !errors.Is(err, boom) {
			t.Errorf("expected wrapped boom, got %v", err)
		}
		if !strings.Contains(err.Error(), "analytics top customers") {
			t.Errorf("expected operation context, got %v", err)
		}
	})
}

// TestTargetDataServiceDelegatesEntityReads verifies the embedded SourceData
// port keeps working through the target service (lab_target reads).
func TestTargetDataServiceDelegatesEntityReads(t *testing.T) {
	id := uuid.New()
	fake := &fakeCustomerRepo{
		list: func(_ context.Context, o repository.ListOptions, f repository.CustomerFilter) ([]models.Customer, int64, error) {
			if o.Limit != 20 || o.Offset != 0 {
				t.Errorf("unexpected options: %+v", o)
			}
			if f.Country != "US" {
				t.Errorf("unexpected filter: %+v", f)
			}
			return []models.Customer{{ID: id, Email: "a@b.c"}}, 1, nil
		},
		get: func(context.Context, uuid.UUID) (*models.Customer, error) {
			return &models.Customer{ID: id, Email: "a@b.c"}, nil
		},
	}

	base := service.NewDataService(fake, nopProductRepo(t), nopOrderRepo(t), nopOrderItemRepo(t))
	svc := service.NewTargetDataService(base, nopAnalytics(t))

	var _ service.TargetData = svc

	page, err := svc.ListCustomers(context.Background(), service.CustomerListParams{Country: "US"})
	if err != nil {
		t.Fatalf("list customers: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Email != "a@b.c" {
		t.Errorf("unexpected page: %+v", page)
	}

	cust, err := svc.GetCustomer(context.Background(), id)
	if err != nil {
		t.Fatalf("get customer: %v", err)
	}
	if cust.ID != id {
		t.Errorf("unexpected customer: %+v", cust)
	}
}

// --- no-op entity repo helpers --------------------------------------------

func nopProductRepo(t *testing.T) *fakeProductRepo {
	t.Helper()
	return &fakeProductRepo{
		list: func(context.Context, repository.ListOptions, repository.ProductFilter) ([]models.Product, int64, error) {
			t.Fatal("products.List must not be called")
			return nil, 0, nil
		},
		get: func(context.Context, uuid.UUID) (*models.Product, error) {
			t.Fatal("products.GetByID must not be called")
			return nil, nil
		},
	}
}

func nopOrderRepo(t *testing.T) *fakeOrderRepo {
	t.Helper()
	return &fakeOrderRepo{
		list: func(context.Context, repository.ListOptions, repository.OrderFilter) ([]models.Order, int64, error) {
			t.Fatal("orders.List must not be called")
			return nil, 0, nil
		},
		get: func(context.Context, uuid.UUID) (*models.Order, error) {
			t.Fatal("orders.GetByID must not be called")
			return nil, nil
		},
	}
}

func nopOrderItemRepo(t *testing.T) *fakeOrderItemRepo {
	t.Helper()
	return &fakeOrderItemRepo{
		list: func(context.Context, repository.ListOptions, repository.OrderItemFilter) ([]models.OrderItem, int64, error) {
			t.Fatal("orderItems.List must not be called")
			return nil, 0, nil
		},
		get: func(context.Context, int64) (*models.OrderItem, error) {
			t.Fatal("orderItems.GetByID must not be called")
			return nil, nil
		},
	}
}

func (s *nopEntitySource) CreateCustomer(context.Context, service.CustomerInput) (models.Customer, error) {
	s.t.Fatal("CreateCustomer must not be called")
	return models.Customer{}, nil
}
func (s *nopEntitySource) UpdateCustomer(context.Context, uuid.UUID, service.CustomerInput) (models.Customer, error) {
	s.t.Fatal("UpdateCustomer must not be called")
	return models.Customer{}, nil
}
func (s *nopEntitySource) DeleteCustomer(context.Context, uuid.UUID) error {
	s.t.Fatal("DeleteCustomer must not be called")
	return nil
}
func (s *nopEntitySource) CreateProduct(context.Context, service.ProductInput) (models.Product, error) {
	s.t.Fatal("CreateProduct must not be called")
	return models.Product{}, nil
}
func (s *nopEntitySource) UpdateProduct(context.Context, uuid.UUID, service.ProductInput) (models.Product, error) {
	s.t.Fatal("UpdateProduct must not be called")
	return models.Product{}, nil
}
func (s *nopEntitySource) DeleteProduct(context.Context, uuid.UUID) error {
	s.t.Fatal("DeleteProduct must not be called")
	return nil
}
func (s *nopEntitySource) CreateOrder(context.Context, service.OrderCreateInput) (models.Order, error) {
	s.t.Fatal("CreateOrder must not be called")
	return models.Order{}, nil
}
func (s *nopEntitySource) UpdateOrder(context.Context, uuid.UUID, service.OrderUpdateInput) (models.Order, error) {
	s.t.Fatal("UpdateOrder must not be called")
	return models.Order{}, nil
}
func (s *nopEntitySource) DeleteOrder(context.Context, uuid.UUID) error {
	s.t.Fatal("DeleteOrder must not be called")
	return nil
}
