package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/handler"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// fakeSourceService implements service.SourceData for handler tests. Every
// method defaults to failing the test; set only the ones under test.
type fakeSourceService struct {
	t *testing.T

	listCustomers func(context.Context, service.CustomerListParams) (service.PageResult[models.Customer], error)
	getCustomer   func(context.Context, uuid.UUID) (models.Customer, error)
	listProducts  func(context.Context, service.ProductListParams) (service.PageResult[models.Product], error)
	getProduct    func(context.Context, uuid.UUID) (models.Product, error)
	listOrders    func(context.Context, service.OrderListParams) (service.PageResult[models.Order], error)
	getOrder      func(context.Context, uuid.UUID) (models.Order, error)
	listItems     func(context.Context, service.OrderItemListParams) (service.PageResult[models.OrderItem], error)
	getItem       func(context.Context, int64) (models.OrderItem, error)
}

func newFakeService(t *testing.T) *fakeSourceService {
	t.Helper()
	unexpected := func(method string) {
		t.Errorf("unexpected call to SourceData.%s", method)
	}
	return &fakeSourceService{
		t: t,
		listCustomers: func(context.Context, service.CustomerListParams) (service.PageResult[models.Customer], error) {
			unexpected("ListCustomers")
			return service.PageResult[models.Customer]{}, nil
		},
		getCustomer: func(context.Context, uuid.UUID) (models.Customer, error) {
			unexpected("GetCustomer")
			return models.Customer{}, nil
		},
		listProducts: func(context.Context, service.ProductListParams) (service.PageResult[models.Product], error) {
			unexpected("ListProducts")
			return service.PageResult[models.Product]{}, nil
		},
		getProduct: func(context.Context, uuid.UUID) (models.Product, error) {
			unexpected("GetProduct")
			return models.Product{}, nil
		},
		listOrders: func(context.Context, service.OrderListParams) (service.PageResult[models.Order], error) {
			unexpected("ListOrders")
			return service.PageResult[models.Order]{}, nil
		},
		getOrder: func(context.Context, uuid.UUID) (models.Order, error) {
			unexpected("GetOrder")
			return models.Order{}, nil
		},
		listItems: func(context.Context, service.OrderItemListParams) (service.PageResult[models.OrderItem], error) {
			unexpected("ListOrderItems")
			return service.PageResult[models.OrderItem]{}, nil
		},
		getItem: func(context.Context, int64) (models.OrderItem, error) {
			unexpected("GetOrderItem")
			return models.OrderItem{}, nil
		},
	}
}

func (f *fakeSourceService) ListCustomers(ctx context.Context, p service.CustomerListParams) (service.PageResult[models.Customer], error) {
	return f.listCustomers(ctx, p)
}
func (f *fakeSourceService) GetCustomer(ctx context.Context, id uuid.UUID) (models.Customer, error) {
	return f.getCustomer(ctx, id)
}
func (f *fakeSourceService) ListProducts(ctx context.Context, p service.ProductListParams) (service.PageResult[models.Product], error) {
	return f.listProducts(ctx, p)
}
func (f *fakeSourceService) GetProduct(ctx context.Context, id uuid.UUID) (models.Product, error) {
	return f.getProduct(ctx, id)
}
func (f *fakeSourceService) ListOrders(ctx context.Context, p service.OrderListParams) (service.PageResult[models.Order], error) {
	return f.listOrders(ctx, p)
}
func (f *fakeSourceService) GetOrder(ctx context.Context, id uuid.UUID) (models.Order, error) {
	return f.getOrder(ctx, id)
}
func (f *fakeSourceService) ListOrderItems(ctx context.Context, p service.OrderItemListParams) (service.PageResult[models.OrderItem], error) {
	return f.listItems(ctx, p)
}
func (f *fakeSourceService) GetOrderItem(ctx context.Context, id int64) (models.OrderItem, error) {
	return f.getItem(ctx, id)
}

var _ service.SourceData = (*fakeSourceService)(nil)

// do executes a GET request against the source handler routes.
func do(t *testing.T, svc service.SourceData, target string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	h := handler.NewSourceHandler(svc)
	engine := gin.New()
	engine.GET("/api/v1/source/customers", h.ListCustomers)
	engine.GET("/api/v1/source/customers/:id", h.GetCustomer)
	engine.GET("/api/v1/source/products", h.ListProducts)
	engine.GET("/api/v1/source/products/:id", h.GetProduct)
	engine.GET("/api/v1/source/orders", h.ListOrders)
	engine.GET("/api/v1/source/orders/:id", h.GetOrder)
	engine.GET("/api/v1/source/order-items", h.ListOrderItems)

	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func decodePaginated(t *testing.T, rec *httptest.ResponseRecorder) handler.PaginatedResponse {
	t.Helper()
	var body handler.PaginatedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode paginated body: %v (body: %s)", err, rec.Body.String())
	}
	return body
}

// --- customers ------------------------------------------------------------

func TestSourceListCustomersDefaults(t *testing.T) {
	var got service.CustomerListParams
	svc := newFakeService(t)
	svc.listCustomers = func(_ context.Context, p service.CustomerListParams) (service.PageResult[models.Customer], error) {
		got = p
		return service.PageResult[models.Customer]{
			Items:   []models.Customer{{ID: uuid.New(), Email: "a@b.c"}},
			Total:   21,
			Page:    1,
			PerPage: 20,
		}, nil
	}

	rec := do(t, svc, "/api/v1/source/customers")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if got.Page != 1 || got.PerPage != 20 {
		t.Errorf("service params = %+v, want page 1 per_page 20", got)
	}
	if got.Search != "" || got.Country != "" {
		t.Errorf("expected empty filters, got %+v", got)
	}

	body := decodePaginated(t, rec)
	if body.Meta.Page != 1 || body.Meta.PerPage != 20 || body.Meta.Total != 21 || body.Meta.TotalPages != 2 {
		t.Errorf("unexpected meta: %+v", body.Meta)
	}
	items, ok := body.Data.([]any)
	if !ok || len(items) != 1 {
		t.Errorf("expected 1 item, got %+v", body.Data)
	}
}

func TestSourceListCustomersFiltersAndPaging(t *testing.T) {
	var got service.CustomerListParams
	svc := newFakeService(t)
	svc.listCustomers = func(_ context.Context, p service.CustomerListParams) (service.PageResult[models.Customer], error) {
		got = p
		return service.PageResult[models.Customer]{Items: []models.Customer{}, Total: 0, Page: p.Page, PerPage: p.PerPage}, nil
	}

	rec := do(t, svc, "/api/v1/source/customers?page=3&per_page=15&search=%20alice%20&country=us")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if got.Page != 3 || got.PerPage != 15 || got.Search != "alice" || got.Country != "us" {
		t.Errorf("service params = %+v", got)
	}
}

func TestSourceListCustomersInvalidPaging(t *testing.T) {
	svc := newFakeService(t)

	rec := do(t, svc, "/api/v1/source/customers?per_page=101")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
	resp := decodeError(t, rec)
	if resp.Error.Code != handler.CodeValidation {
		t.Errorf("expected code %s, got %q", handler.CodeValidation, resp.Error.Code)
	}
	if _, ok := resp.Error.Details["fields"].(map[string]any); !ok {
		t.Errorf("expected field details, got %+v", resp.Error.Details)
	}
}

func TestSourceListCustomersInvalidParamType(t *testing.T) {
	svc := newFakeService(t)

	rec := do(t, svc, "/api/v1/source/customers?page=abc")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if decodeError(t, rec).Error.Code != handler.CodeValidation {
		t.Errorf("expected validation code")
	}
}

func TestSourceListCustomersServiceError(t *testing.T) {
	svc := newFakeService(t)
	svc.listCustomers = func(context.Context, service.CustomerListParams) (service.PageResult[models.Customer], error) {
		return service.PageResult[models.Customer]{}, errors.New("db down")
	}

	rec := do(t, svc, "/api/v1/source/customers")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	resp := decodeError(t, rec)
	if resp.Error.Code != handler.CodeInternal {
		t.Errorf("expected code %s, got %q", handler.CodeInternal, resp.Error.Code)
	}
	if rec.Body.String() == "" || json.Valid(rec.Body.Bytes()) == false {
		t.Errorf("expected valid JSON error body, got %s", rec.Body.String())
	}
}

func TestSourceGetCustomer(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	t.Run("ok", func(t *testing.T) {
		var gotID uuid.UUID
		svc := newFakeService(t)
		svc.getCustomer = func(_ context.Context, got uuid.UUID) (models.Customer, error) {
			gotID = got
			return models.Customer{ID: got, Email: "alice@example.com", FirstName: "Alice"}, nil
		}

		rec := do(t, svc, "/api/v1/source/customers/"+id.String())

		if rec.Code != http.StatusOK {
			t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
		}
		if gotID != id {
			t.Errorf("service id = %s, want %s", gotID, id)
		}
		var body models.Customer
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode customer: %v", err)
		}
		if body.Email != "alice@example.com" {
			t.Errorf("unexpected body: %+v", body)
		}
	})

	t.Run("invalid uuid", func(t *testing.T) {
		svc := newFakeService(t)

		rec := do(t, svc, "/api/v1/source/customers/not-a-uuid")

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
		}
		resp := decodeError(t, rec)
		if resp.Error.Code != handler.CodeValidation {
			t.Errorf("expected %s, got %q", handler.CodeValidation, resp.Error.Code)
		}
		fields, _ := resp.Error.Details["fields"].(map[string]any)
		if fields["id"] == nil {
			t.Errorf("expected id field detail, got %+v", resp.Error.Details)
		}
	})

	t.Run("not found", func(t *testing.T) {
		svc := newFakeService(t)
		svc.getCustomer = func(context.Context, uuid.UUID) (models.Customer, error) {
			return models.Customer{}, service.ErrNotFound
		}

		rec := do(t, svc, "/api/v1/source/customers/"+id.String())

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
		}
		resp := decodeError(t, rec)
		if resp.Error.Code != handler.CodeNotFound {
			t.Errorf("expected %s, got %q", handler.CodeNotFound, resp.Error.Code)
		}
		if resp.Error.Message != "customer not found" {
			t.Errorf("unexpected message: %q", resp.Error.Message)
		}
	})

	t.Run("service error", func(t *testing.T) {
		svc := newFakeService(t)
		svc.getCustomer = func(context.Context, uuid.UUID) (models.Customer, error) {
			return models.Customer{}, errors.New("boom")
		}

		rec := do(t, svc, "/api/v1/source/customers/"+id.String())

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})
}

// --- products -------------------------------------------------------------

func TestSourceListProducts(t *testing.T) {
	var got service.ProductListParams
	svc := newFakeService(t)
	svc.listProducts = func(_ context.Context, p service.ProductListParams) (service.PageResult[models.Product], error) {
		got = p
		return service.PageResult[models.Product]{
			Items:   []models.Product{{ID: uuid.New(), SKU: "SKU-1"}},
			Total:   1,
			Page:    p.Page,
			PerPage: p.PerPage,
		}, nil
	}

	rec := do(t, svc, "/api/v1/source/products?category=Electronics&search=widget&page=2&per_page=5")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if got.Category != "Electronics" || got.Search != "widget" || got.Page != 2 || got.PerPage != 5 {
		t.Errorf("service params = %+v", got)
	}
	body := decodePaginated(t, rec)
	if body.Meta.Total != 1 || body.Meta.Page != 2 {
		t.Errorf("unexpected meta: %+v", body.Meta)
	}
}

func TestSourceListProductsInvalidPaging(t *testing.T) {
	svc := newFakeService(t)

	rec := do(t, svc, "/api/v1/source/products?page=-2")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSourceGetProduct(t *testing.T) {
	id := uuid.New()

	t.Run("ok", func(t *testing.T) {
		svc := newFakeService(t)
		svc.getProduct = func(_ context.Context, got uuid.UUID) (models.Product, error) {
			return models.Product{ID: got, SKU: "SKU-9"}, nil
		}

		rec := do(t, svc, "/api/v1/source/products/"+id.String())

		if rec.Code != http.StatusOK {
			t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
		}
		var body models.Product
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.SKU != "SKU-9" {
			t.Errorf("unexpected body: %+v", body)
		}
	})

	t.Run("not found", func(t *testing.T) {
		svc := newFakeService(t)
		svc.getProduct = func(context.Context, uuid.UUID) (models.Product, error) {
			return models.Product{}, service.ErrNotFound
		}

		rec := do(t, svc, "/api/v1/source/products/"+id.String())
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("invalid uuid", func(t *testing.T) {
		svc := newFakeService(t)
		rec := do(t, svc, "/api/v1/source/products/nope")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})
}

// --- orders ---------------------------------------------------------------

func TestSourceListOrdersParsesFilters(t *testing.T) {
	customerID := uuid.New()
	var got service.OrderListParams
	svc := newFakeService(t)
	svc.listOrders = func(_ context.Context, p service.OrderListParams) (service.PageResult[models.Order], error) {
		got = p
		return service.PageResult[models.Order]{Items: []models.Order{}, Total: 0, Page: p.Page, PerPage: p.PerPage}, nil
	}

	rec := do(t, svc, "/api/v1/source/orders?customer_id="+customerID.String()+"&status=paid&date_from=2024-01-01&date_to=2024-01-31")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if got.CustomerID == nil || *got.CustomerID != customerID {
		t.Errorf("customer id = %+v", got.CustomerID)
	}
	if got.Status != "paid" {
		t.Errorf("status = %q", got.Status)
	}
	if got.DateFrom == nil || !got.DateFrom.Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("date_from = %v", got.DateFrom)
	}
	// date-only end bound is inclusive: extended to the last instant of the day.
	wantTo := time.Date(2024, 1, 31, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
	if got.DateTo == nil || !got.DateTo.Equal(wantTo) {
		t.Errorf("date_to = %v, want %v", got.DateTo, wantTo)
	}
}

func TestSourceListOrdersRFC3339Range(t *testing.T) {
	var got service.OrderListParams
	svc := newFakeService(t)
	svc.listOrders = func(_ context.Context, p service.OrderListParams) (service.PageResult[models.Order], error) {
		got = p
		return service.PageResult[models.Order]{Items: []models.Order{}, Page: p.Page, PerPage: p.PerPage}, nil
	}

	rec := do(t, svc, "/api/v1/source/orders?date_from=2024-01-01T10:00:00Z&date_to=2024-01-02T10:00:00Z")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if got.DateFrom == nil || !got.DateFrom.Equal(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("date_from = %v", got.DateFrom)
	}
	// RFC3339 end bound is exact (not extended).
	if got.DateTo == nil || !got.DateTo.Equal(time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("date_to = %v", got.DateTo)
	}
}

func TestSourceListOrdersValidationErrors(t *testing.T) {
	cases := []struct {
		name   string
		target string
		field  string
	}{
		{"invalid customer uuid", "/api/v1/source/orders?customer_id=nope", "customer_id"},
		{"invalid date_from", "/api/v1/source/orders?date_from=yesterday", "date_from"},
		{"invalid date_to", "/api/v1/source/orders?date_to=31-01-2024", "date_to"},
		{"reversed range", "/api/v1/source/orders?date_from=2024-02-01&date_to=2024-01-01", "date_from"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newFakeService(t)

			rec := do(t, svc, tc.target)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected %d, got %d (body: %s)", http.StatusBadRequest, rec.Code, rec.Body.String())
			}
			resp := decodeError(t, rec)
			if resp.Error.Code != handler.CodeValidation {
				t.Errorf("expected code %s, got %q", handler.CodeValidation, resp.Error.Code)
			}
			fields, _ := resp.Error.Details["fields"].(map[string]any)
			if fields[tc.field] == nil {
				t.Errorf("expected field %q in details, got %+v", tc.field, resp.Error.Details)
			}
		})
	}
}

func TestSourceListOrdersServiceError(t *testing.T) {
	svc := newFakeService(t)
	svc.listOrders = func(context.Context, service.OrderListParams) (service.PageResult[models.Order], error) {
		return service.PageResult[models.Order]{}, errors.New("boom")
	}

	rec := do(t, svc, "/api/v1/source/orders")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestSourceGetOrderIncludesItems(t *testing.T) {
	id := uuid.New()
	svc := newFakeService(t)
	svc.getOrder = func(_ context.Context, got uuid.UUID) (models.Order, error) {
		return models.Order{
			ID:          got,
			OrderNumber: "ORD-1",
			OrderItems: []models.OrderItem{
				{ID: 1, OrderID: got, Quantity: 2, UnitPrice: 5, Subtotal: 10},
			},
		}, nil
	}

	rec := do(t, svc, "/api/v1/source/orders/"+id.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	var body models.Order
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode order: %v", err)
	}
	if len(body.OrderItems) != 1 || body.OrderItems[0].Subtotal != 10 {
		t.Errorf("expected items in payload, got %+v", body.OrderItems)
	}
}

func TestSourceGetOrderNotFoundAndInvalid(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		svc := newFakeService(t)
		svc.getOrder = func(context.Context, uuid.UUID) (models.Order, error) {
			return models.Order{}, service.ErrNotFound
		}

		rec := do(t, svc, "/api/v1/source/orders/"+uuid.NewString())
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
		}
		if decodeError(t, rec).Error.Message != "order not found" {
			t.Errorf("unexpected message: %s", rec.Body.String())
		}
	})

	t.Run("invalid uuid", func(t *testing.T) {
		svc := newFakeService(t)
		rec := do(t, svc, "/api/v1/source/orders/nope")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("service error", func(t *testing.T) {
		svc := newFakeService(t)
		svc.getOrder = func(context.Context, uuid.UUID) (models.Order, error) {
			return models.Order{}, errors.New("boom")
		}
		rec := do(t, svc, "/api/v1/source/orders/"+uuid.NewString())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})
}

// --- order items ----------------------------------------------------------

func TestSourceListOrderItems(t *testing.T) {
	orderID := uuid.New()
	productID := uuid.New()
	var got service.OrderItemListParams
	svc := newFakeService(t)
	svc.listItems = func(_ context.Context, p service.OrderItemListParams) (service.PageResult[models.OrderItem], error) {
		got = p
		return service.PageResult[models.OrderItem]{
			Items:   []models.OrderItem{{ID: 1, OrderID: orderID, ProductID: productID}},
			Total:   1,
			Page:    p.Page,
			PerPage: p.PerPage,
		}, nil
	}

	rec := do(t, svc, "/api/v1/source/order-items?order_id="+orderID.String()+"&product_id="+productID.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if got.OrderID == nil || *got.OrderID != orderID {
		t.Errorf("order id = %+v", got.OrderID)
	}
	if got.ProductID == nil || *got.ProductID != productID {
		t.Errorf("product id = %+v", got.ProductID)
	}
	if body := decodePaginated(t, rec); body.Meta.Total != 1 {
		t.Errorf("unexpected meta: %+v", body.Meta)
	}
}

func TestSourceListOrderItemsValidation(t *testing.T) {
	t.Run("invalid order id", func(t *testing.T) {
		svc := newFakeService(t)
		rec := do(t, svc, "/api/v1/source/order-items?order_id=nope")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
		}
		fields, _ := decodeError(t, rec).Error.Details["fields"].(map[string]any)
		if fields["order_id"] == nil {
			t.Errorf("expected order_id field detail, got %s", rec.Body.String())
		}
	})

	t.Run("service error", func(t *testing.T) {
		svc := newFakeService(t)
		svc.listItems = func(context.Context, service.OrderItemListParams) (service.PageResult[models.OrderItem], error) {
			return service.PageResult[models.OrderItem]{}, errors.New("boom")
		}
		rec := do(t, svc, "/api/v1/source/order-items")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
		}
	})
}
