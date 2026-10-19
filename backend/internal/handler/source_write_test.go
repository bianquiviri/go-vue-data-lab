package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/handler"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// doSourceWrite executes a request with a JSON body against an engine mounting the
// source write routes.
func doSourceWrite(t *testing.T, svc service.SourceData, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	h := handler.NewSourceHandler(svc)
	engine := gin.New()
	engine.POST("/api/v1/source/customers", h.CreateCustomer)
	engine.PUT("/api/v1/source/customers/:id", h.UpdateCustomer)
	engine.DELETE("/api/v1/source/customers/:id", h.DeleteCustomer)
	engine.POST("/api/v1/source/products", h.CreateProduct)
	engine.PUT("/api/v1/source/products/:id", h.UpdateProduct)
	engine.DELETE("/api/v1/source/products/:id", h.DeleteProduct)
	engine.POST("/api/v1/source/orders", h.CreateOrder)
	engine.PUT("/api/v1/source/orders/:id", h.UpdateOrder)
	engine.DELETE("/api/v1/source/orders/:id", h.DeleteOrder)

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func decodeData(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode data envelope: %v (body: %s)", err, rec.Body.String())
	}
	return body.Data
}

// --- customers ------------------------------------------------------------

func TestSourceCreateCustomer(t *testing.T) {
	svc := newFakeService(t)
	svc.createCustomer = func(_ context.Context, in service.CustomerInput) (models.Customer, error) {
		return models.Customer{ID: uuid.New(), FirstName: in.FirstName, Email: in.Email}, nil
	}

	rec := doSourceWrite(t, svc, http.MethodPost, "/api/v1/source/customers",
		`{"first_name":"Ada","last_name":"Lovelace","email":"ada@example.com","country_code":"GB","city":"London"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusCreated, rec.Code, rec.Body.String())
	}
	if got := decodeData(t, rec)["email"]; got != "ada@example.com" {
		t.Errorf("email = %v, want ada@example.com", got)
	}
}

func TestSourceCreateCustomerValidation(t *testing.T) {
	svc := newFakeService(t)
	rec := doSourceWrite(t, svc, http.MethodPost, "/api/v1/source/customers", `{"first_name":"Ada"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSourceCreateCustomerConflict(t *testing.T) {
	svc := newFakeService(t)
	svc.createCustomer = func(context.Context, service.CustomerInput) (models.Customer, error) {
		return models.Customer{}, service.ErrConflict
	}
	rec := doSourceWrite(t, svc, http.MethodPost, "/api/v1/source/customers",
		`{"first_name":"A","last_name":"B","email":"a@b.com","country_code":"US","city":"NY"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected %d, got %d", http.StatusConflict, rec.Code)
	}
}

func TestSourceUpdateCustomer(t *testing.T) {
	id := uuid.New()
	var gotID uuid.UUID
	svc := newFakeService(t)
	svc.updateCustomer = func(_ context.Context, inID uuid.UUID, in service.CustomerInput) (models.Customer, error) {
		gotID = inID
		return models.Customer{ID: inID, FirstName: in.FirstName}, nil
	}

	rec := doSourceWrite(t, svc, http.MethodPut, "/api/v1/source/customers/"+id.String(),
		`{"first_name":"Grace","last_name":"Hopper","email":"grace@example.com","country_code":"US","city":"Arlington"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if gotID != id {
		t.Errorf("service got id %s, want %s", gotID, id)
	}
}

func TestSourceUpdateCustomerInvalidID(t *testing.T) {
	svc := newFakeService(t)
	rec := doSourceWrite(t, svc, http.MethodPut, "/api/v1/source/customers/not-a-uuid", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSourceUpdateCustomerNotFound(t *testing.T) {
	svc := newFakeService(t)
	svc.updateCustomer = func(context.Context, uuid.UUID, service.CustomerInput) (models.Customer, error) {
		return models.Customer{}, service.ErrNotFound
	}
	rec := doSourceWrite(t, svc, http.MethodPut, "/api/v1/source/customers/"+uuid.NewString(),
		`{"first_name":"A","last_name":"B","email":"a@b.com","country_code":"US","city":"NY"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestSourceDeleteCustomer(t *testing.T) {
	svc := newFakeService(t)
	svc.deleteCustomer = func(context.Context, uuid.UUID) error { return nil }
	rec := doSourceWrite(t, svc, http.MethodDelete, "/api/v1/source/customers/"+uuid.NewString(), "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("expected empty body, got %q", rec.Body.String())
	}
}

func TestSourceDeleteCustomerNotFound(t *testing.T) {
	svc := newFakeService(t)
	svc.deleteCustomer = func(context.Context, uuid.UUID) error { return service.ErrNotFound }
	rec := doSourceWrite(t, svc, http.MethodDelete, "/api/v1/source/customers/"+uuid.NewString(), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

// --- products -------------------------------------------------------------

func TestSourceCreateProduct(t *testing.T) {
	svc := newFakeService(t)
	svc.createProduct = func(_ context.Context, in service.ProductInput) (models.Product, error) {
		return models.Product{ID: uuid.New(), SKU: in.SKU}, nil
	}
	rec := doSourceWrite(t, svc, http.MethodPost, "/api/v1/source/products",
		`{"sku":"SKU-1","name":"Widget","category":"tools","price":9.99,"stock":10}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusCreated, rec.Code, rec.Body.String())
	}
	if got := decodeData(t, rec)["sku"]; got != "SKU-1" {
		t.Errorf("sku = %v, want SKU-1", got)
	}
}

func TestSourceCreateProductValidation(t *testing.T) {
	svc := newFakeService(t)
	rec := doSourceWrite(t, svc, http.MethodPost, "/api/v1/source/products", `{"name":"Widget"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSourceCreateProductConflict(t *testing.T) {
	svc := newFakeService(t)
	svc.createProduct = func(context.Context, service.ProductInput) (models.Product, error) {
		return models.Product{}, service.ErrConflict
	}
	rec := doSourceWrite(t, svc, http.MethodPost, "/api/v1/source/products",
		`{"sku":"SKU-1","name":"W","category":"c","price":1,"stock":1}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected %d, got %d", http.StatusConflict, rec.Code)
	}
}

func TestSourceUpdateProductNotFound(t *testing.T) {
	svc := newFakeService(t)
	svc.updateProduct = func(context.Context, uuid.UUID, service.ProductInput) (models.Product, error) {
		return models.Product{}, service.ErrNotFound
	}
	rec := doSourceWrite(t, svc, http.MethodPut, "/api/v1/source/products/"+uuid.NewString(),
		`{"sku":"SKU-1","name":"W","category":"c","price":1,"stock":0}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestSourceDeleteProduct(t *testing.T) {
	svc := newFakeService(t)
	svc.deleteProduct = func(context.Context, uuid.UUID) error { return nil }
	rec := doSourceWrite(t, svc, http.MethodDelete, "/api/v1/source/products/"+uuid.NewString(), "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
}

// --- orders ---------------------------------------------------------------

func TestSourceCreateOrder(t *testing.T) {
	svc := newFakeService(t)
	svc.createOrder = func(_ context.Context, in service.OrderCreateInput) (models.Order, error) {
		return models.Order{ID: uuid.New(), OrderNumber: in.OrderNumber, TotalAmount: 19.98}, nil
	}
	rec := doSourceWrite(t, svc, http.MethodPost, "/api/v1/source/orders",
		`{"customer_id":"`+uuid.NewString()+`","items":[{"product_id":"`+uuid.NewString()+`","quantity":2,"unit_price":9.99}]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusCreated, rec.Code, rec.Body.String())
	}
	if got := decodeData(t, rec)["total_amount"]; got != 19.98 {
		t.Errorf("total_amount = %v, want 19.98", got)
	}
}

func TestSourceCreateOrderValidation(t *testing.T) {
	svc := newFakeService(t)
	rec := doSourceWrite(t, svc, http.MethodPost, "/api/v1/source/orders", `{"status":"pending"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestSourceCreateOrderInvalidReference(t *testing.T) {
	svc := newFakeService(t)
	svc.createOrder = func(context.Context, service.OrderCreateInput) (models.Order, error) {
		return models.Order{}, service.ErrInvalidReference
	}
	rec := doSourceWrite(t, svc, http.MethodPost, "/api/v1/source/orders", `{"customer_id":"`+uuid.NewString()+`"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestSourceUpdateOrder(t *testing.T) {
	id := uuid.New()
	var gotID uuid.UUID
	svc := newFakeService(t)
	svc.updateOrder = func(_ context.Context, inID uuid.UUID, in service.OrderUpdateInput) (models.Order, error) {
		gotID = inID
		return models.Order{ID: inID}, nil
	}
	rec := doSourceWrite(t, svc, http.MethodPut, "/api/v1/source/orders/"+id.String(), `{"status":"shipped"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if gotID != id {
		t.Errorf("service got id %s, want %s", gotID, id)
	}
}

func TestSourceDeleteOrderNotFound(t *testing.T) {
	svc := newFakeService(t)
	svc.deleteOrder = func(context.Context, uuid.UUID) error { return service.ErrNotFound }
	rec := doSourceWrite(t, svc, http.MethodDelete, "/api/v1/source/orders/"+uuid.NewString(), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestSourceDeleteOrder(t *testing.T) {
	svc := newFakeService(t)
	svc.deleteOrder = func(context.Context, uuid.UUID) error { return nil }
	rec := doSourceWrite(t, svc, http.MethodDelete, "/api/v1/source/orders/"+uuid.NewString(), "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
}
