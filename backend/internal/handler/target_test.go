package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/handler"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// fakeTargetService implements service.TargetData for handler tests. Entity
// reads delegate to the shared fakeSourceService; analytics methods are
// function fields defaulting to failing the test.
type fakeTargetService struct {
	*fakeSourceService

	summary      func(context.Context) (models.AnalyticsSummary, error)
	topProducts  func(context.Context, int) ([]models.TopProduct, error)
	topCustomers func(context.Context, int) ([]models.TopCustomer, error)
}

func newFakeTargetService(t *testing.T) *fakeTargetService {
	t.Helper()
	unexpected := func(method string) {
		t.Errorf("unexpected call to TargetData.%s", method)
	}
	return &fakeTargetService{
		fakeSourceService: newFakeService(t),
		summary: func(context.Context) (models.AnalyticsSummary, error) {
			unexpected("Summary")
			return models.AnalyticsSummary{}, nil
		},
		topProducts: func(context.Context, int) ([]models.TopProduct, error) {
			unexpected("TopProducts")
			return nil, nil
		},
		topCustomers: func(context.Context, int) ([]models.TopCustomer, error) {
			unexpected("TopCustomers")
			return nil, nil
		},
	}
}

func (f *fakeTargetService) Summary(ctx context.Context) (models.AnalyticsSummary, error) {
	return f.summary(ctx)
}
func (f *fakeTargetService) TopProducts(ctx context.Context, limit int) ([]models.TopProduct, error) {
	return f.topProducts(ctx, limit)
}
func (f *fakeTargetService) TopCustomers(ctx context.Context, limit int) ([]models.TopCustomer, error) {
	return f.topCustomers(ctx, limit)
}

var _ service.TargetData = (*fakeTargetService)(nil)

// doTarget executes a GET request against the target handler routes.
func doTarget(t *testing.T, svc service.TargetData, target string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	h := handler.NewTargetHandler(svc)
	engine := gin.New()
	engine.GET("/api/v1/target/customers", h.ListCustomers)
	engine.GET("/api/v1/target/customers/:id", h.GetCustomer)
	engine.GET("/api/v1/target/products", h.ListProducts)
	engine.GET("/api/v1/target/products/:id", h.GetProduct)
	engine.GET("/api/v1/target/orders", h.ListOrders)
	engine.GET("/api/v1/target/orders/:id", h.GetOrder)
	engine.GET("/api/v1/target/analytics/summary", h.AnalyticsSummary)
	engine.GET("/api/v1/target/analytics/top-products", h.TopProducts)
	engine.GET("/api/v1/target/analytics/top-customers", h.TopCustomers)

	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// --- entity reads ----------------------------------------------------------

func TestTargetListCustomersDefaults(t *testing.T) {
	var got service.CustomerListParams
	svc := newFakeTargetService(t)
	svc.listCustomers = func(_ context.Context, p service.CustomerListParams) (service.PageResult[models.Customer], error) {
		got = p
		return service.PageResult[models.Customer]{
			Items:   []models.Customer{{ID: uuid.New(), Email: "a@b.c"}},
			Total:   21,
			Page:    1,
			PerPage: 20,
		}, nil
	}

	rec := doTarget(t, svc, "/api/v1/target/customers")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if got.Page != 1 || got.PerPage != 20 {
		t.Errorf("service params = %+v, want page 1 per_page 20", got)
	}
	body := decodePaginated(t, rec)
	if body.Meta.Total != 21 || body.Meta.TotalPages != 2 {
		t.Errorf("unexpected meta: %+v", body.Meta)
	}
}

func TestTargetListCustomersFiltersAndPaging(t *testing.T) {
	var got service.CustomerListParams
	svc := newFakeTargetService(t)
	svc.listCustomers = func(_ context.Context, p service.CustomerListParams) (service.PageResult[models.Customer], error) {
		got = p
		return service.PageResult[models.Customer]{Items: []models.Customer{}, Page: p.Page, PerPage: p.PerPage}, nil
	}

	rec := doTarget(t, svc, "/api/v1/target/customers?page=3&per_page=15&search=%20alice%20&country=us")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if got.Page != 3 || got.PerPage != 15 || got.Search != "alice" || got.Country != "us" {
		t.Errorf("service params = %+v", got)
	}
}

func TestTargetListCustomersValidation(t *testing.T) {
	svc := newFakeTargetService(t)

	rec := doTarget(t, svc, "/api/v1/target/customers?per_page=101")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if decodeError(t, rec).Error.Code != handler.CodeValidation {
		t.Errorf("expected validation code")
	}
}

func TestTargetGetCustomer(t *testing.T) {
	id := uuid.New()

	t.Run("ok", func(t *testing.T) {
		var gotID uuid.UUID
		svc := newFakeTargetService(t)
		svc.getCustomer = func(_ context.Context, got uuid.UUID) (models.Customer, error) {
			gotID = got
			return models.Customer{ID: got, Email: "alice@example.com"}, nil
		}

		rec := doTarget(t, svc, "/api/v1/target/customers/"+id.String())

		if rec.Code != http.StatusOK {
			t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
		}
		if gotID != id {
			t.Errorf("service id = %s, want %s", gotID, id)
		}
	})

	t.Run("invalid uuid", func(t *testing.T) {
		svc := newFakeTargetService(t)
		rec := doTarget(t, svc, "/api/v1/target/customers/not-a-uuid")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("not found", func(t *testing.T) {
		svc := newFakeTargetService(t)
		svc.getCustomer = func(context.Context, uuid.UUID) (models.Customer, error) {
			return models.Customer{}, service.ErrNotFound
		}
		rec := doTarget(t, svc, "/api/v1/target/customers/"+id.String())
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
		}
		if decodeError(t, rec).Error.Message != "customer not found" {
			t.Errorf("unexpected message: %s", rec.Body.String())
		}
	})

	t.Run("service error", func(t *testing.T) {
		svc := newFakeTargetService(t)
		svc.getCustomer = func(context.Context, uuid.UUID) (models.Customer, error) {
			return models.Customer{}, errors.New("boom")
		}
		rec := doTarget(t, svc, "/api/v1/target/customers/"+id.String())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})
}

func TestTargetListProducts(t *testing.T) {
	var got service.ProductListParams
	svc := newFakeTargetService(t)
	svc.listProducts = func(_ context.Context, p service.ProductListParams) (service.PageResult[models.Product], error) {
		got = p
		return service.PageResult[models.Product]{Items: []models.Product{}, Page: p.Page, PerPage: p.PerPage}, nil
	}

	rec := doTarget(t, svc, "/api/v1/target/products?category=Electronics&search=widget&page=2&per_page=5")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if got.Category != "Electronics" || got.Search != "widget" || got.Page != 2 || got.PerPage != 5 {
		t.Errorf("service params = %+v", got)
	}
}

func TestTargetGetProduct(t *testing.T) {
	id := uuid.New()

	t.Run("ok", func(t *testing.T) {
		svc := newFakeTargetService(t)
		svc.getProduct = func(_ context.Context, got uuid.UUID) (models.Product, error) {
			return models.Product{ID: got, SKU: "SKU-9"}, nil
		}
		rec := doTarget(t, svc, "/api/v1/target/products/"+id.String())
		if rec.Code != http.StatusOK {
			t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
		}
	})

	t.Run("not found", func(t *testing.T) {
		svc := newFakeTargetService(t)
		svc.getProduct = func(context.Context, uuid.UUID) (models.Product, error) {
			return models.Product{}, service.ErrNotFound
		}
		rec := doTarget(t, svc, "/api/v1/target/products/"+id.String())
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
		}
	})
}

func TestTargetListOrdersParsesFilters(t *testing.T) {
	customerID := uuid.New()
	var got service.OrderListParams
	svc := newFakeTargetService(t)
	svc.listOrders = func(_ context.Context, p service.OrderListParams) (service.PageResult[models.Order], error) {
		got = p
		return service.PageResult[models.Order]{Items: []models.Order{}, Page: p.Page, PerPage: p.PerPage}, nil
	}

	rec := doTarget(t, svc, "/api/v1/target/orders?customer_id="+customerID.String()+"&status=paid&date_from=2024-01-01&date_to=2024-01-31")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if got.CustomerID == nil || *got.CustomerID != customerID || got.Status != "paid" {
		t.Errorf("service params = %+v", got)
	}
	if got.DateFrom == nil || got.DateTo == nil {
		t.Errorf("expected date bounds, got %+v", got)
	}
}

func TestTargetListOrdersValidationErrors(t *testing.T) {
	cases := []struct {
		name   string
		target string
	}{
		{"invalid customer uuid", "/api/v1/target/orders?customer_id=nope"},
		{"invalid date_from", "/api/v1/target/orders?date_from=yesterday"},
		{"reversed range", "/api/v1/target/orders?date_from=2024-02-01&date_to=2024-01-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newFakeTargetService(t)
			rec := doTarget(t, svc, tc.target)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected %d, got %d (body: %s)", http.StatusBadRequest, rec.Code, rec.Body.String())
			}
			if decodeError(t, rec).Error.Code != handler.CodeValidation {
				t.Errorf("expected validation code, got %s", rec.Body.String())
			}
		})
	}
}

func TestTargetGetOrderIncludesItems(t *testing.T) {
	id := uuid.New()
	svc := newFakeTargetService(t)
	svc.getOrder = func(_ context.Context, got uuid.UUID) (models.Order, error) {
		return models.Order{
			ID:          got,
			OrderNumber: "ORD-1",
			OrderItems:  []models.OrderItem{{ID: 1, OrderID: got, Subtotal: 10}},
		}, nil
	}

	rec := doTarget(t, svc, "/api/v1/target/orders/"+id.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	var body models.Order
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.OrderItems) != 1 || body.OrderItems[0].Subtotal != 10 {
		t.Errorf("expected items in payload, got %+v", body.OrderItems)
	}
}

func TestTargetGetOrderNotFound(t *testing.T) {
	svc := newFakeTargetService(t)
	svc.getOrder = func(context.Context, uuid.UUID) (models.Order, error) {
		return models.Order{}, service.ErrNotFound
	}
	rec := doTarget(t, svc, "/api/v1/target/orders/"+uuid.NewString())
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

// --- analytics -------------------------------------------------------------

func TestTargetAnalyticsSummary(t *testing.T) {
	want := models.AnalyticsSummary{
		TotalCustomers:    10,
		TotalProducts:     20,
		TotalOrders:       30,
		TotalRevenue:      1234.5,
		AverageOrderValue: 41.15,
		OrdersByStatus: []models.OrderStatusCount{
			{Status: "paid", Count: 25},
			{Status: "pending", Count: 5},
		},
	}
	svc := newFakeTargetService(t)
	svc.summary = func(context.Context) (models.AnalyticsSummary, error) {
		return want, nil
	}

	rec := doTarget(t, svc, "/api/v1/target/analytics/summary")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	var body models.AnalyticsSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.TotalOrders != 30 || body.TotalRevenue != 1234.5 || len(body.OrdersByStatus) != 2 {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestTargetAnalyticsSummaryServiceError(t *testing.T) {
	svc := newFakeTargetService(t)
	svc.summary = func(context.Context) (models.AnalyticsSummary, error) {
		return models.AnalyticsSummary{}, errors.New("boom")
	}

	rec := doTarget(t, svc, "/api/v1/target/analytics/summary")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if decodeError(t, rec).Error.Code != handler.CodeInternal {
		t.Errorf("expected internal code")
	}
}

func TestTargetTopProductsDefaultLimit(t *testing.T) {
	var gotLimit int
	svc := newFakeTargetService(t)
	svc.topProducts = func(_ context.Context, limit int) ([]models.TopProduct, error) {
		gotLimit = limit
		return []models.TopProduct{{ID: uuid.New(), SKU: "SKU-1", TotalQuantity: 9, TotalRevenue: 99.9}}, nil
	}

	rec := doTarget(t, svc, "/api/v1/target/analytics/top-products")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if gotLimit != service.DefaultAnalyticsLimit {
		t.Errorf("limit = %d, want default %d", gotLimit, service.DefaultAnalyticsLimit)
	}

	var body handler.AnalyticsListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Meta.Limit != service.DefaultAnalyticsLimit {
		t.Errorf("meta.limit = %d, want %d", body.Meta.Limit, service.DefaultAnalyticsLimit)
	}
	items, ok := body.Data.([]any)
	if !ok || len(items) != 1 {
		t.Errorf("expected 1 item, got %+v", body.Data)
	}
}

func TestTopLimitQueryParams(t *testing.T) {
	t.Run("custom limit", func(t *testing.T) {
		var gotLimit int
		svc := newFakeTargetService(t)
		svc.topProducts = func(_ context.Context, limit int) ([]models.TopProduct, error) {
			gotLimit = limit
			return []models.TopProduct{}, nil
		}

		rec := doTarget(t, svc, "/api/v1/target/analytics/top-products?limit=3")
		if rec.Code != http.StatusOK {
			t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
		}
		if gotLimit != 3 {
			t.Errorf("limit = %d, want 3", gotLimit)
		}
	})

	t.Run("limit above max rejected", func(t *testing.T) {
		svc := newFakeTargetService(t)
		rec := doTarget(t, svc, "/api/v1/target/analytics/top-products?limit=101")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
		}
		fields, _ := decodeError(t, rec).Error.Details["fields"].(map[string]any)
		if fields["Limit"] == nil {
			t.Errorf("expected Limit field detail, got %+v", decodeError(t, rec).Error.Details)
		}
	})

	t.Run("zero limit falls back to default", func(t *testing.T) {
		var gotLimit int
		svc := newFakeTargetService(t)
		svc.topCustomers = func(_ context.Context, limit int) ([]models.TopCustomer, error) {
			gotLimit = limit
			return []models.TopCustomer{}, nil
		}

		rec := doTarget(t, svc, "/api/v1/target/analytics/top-customers?limit=0")
		if rec.Code != http.StatusOK {
			t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
		}
		if gotLimit != service.DefaultAnalyticsLimit {
			t.Errorf("limit = %d, want default", gotLimit)
		}
	})

	t.Run("invalid limit type rejected", func(t *testing.T) {
		svc := newFakeTargetService(t)
		rec := doTarget(t, svc, "/api/v1/target/analytics/top-customers?limit=abc")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})
}

func TestTargetTopCustomers(t *testing.T) {
	want := []models.TopCustomer{{ID: uuid.New(), Email: "a@b.c", TotalOrders: 7, TotalSpent: 500}}
	svc := newFakeTargetService(t)
	svc.topCustomers = func(_ context.Context, limit int) ([]models.TopCustomer, error) {
		if limit != 25 {
			t.Errorf("limit = %d, want 25", limit)
		}
		return want, nil
	}

	rec := doTarget(t, svc, "/api/v1/target/analytics/top-customers?limit=25")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	var body handler.AnalyticsListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Meta.Limit != 25 {
		t.Errorf("meta.limit = %d, want 25", body.Meta.Limit)
	}
	items, ok := body.Data.([]any)
	if !ok || len(items) != 1 {
		t.Errorf("expected 1 item, got %+v", body.Data)
	}
}

func TestTargetTopCustomersServiceError(t *testing.T) {
	svc := newFakeTargetService(t)
	svc.topCustomers = func(context.Context, int) ([]models.TopCustomer, error) {
		return nil, errors.New("boom")
	}

	rec := doTarget(t, svc, "/api/v1/target/analytics/top-customers")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}
