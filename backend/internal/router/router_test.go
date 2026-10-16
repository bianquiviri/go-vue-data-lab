package router_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/jobs"
	"github.com/josebianco/go-vue-data-lab/internal/middleware"
	"github.com/josebianco/go-vue-data-lab/internal/router"
	"github.com/josebianco/go-vue-data-lab/internal/scheduler"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// stubSourceData is a permissive service.SourceData fake: lists return empty
// pages and lookups return empty records.
type stubSourceData struct{}

func (stubSourceData) ListCustomers(_ context.Context, p service.CustomerListParams) (service.PageResult[models.Customer], error) {
	return service.PageResult[models.Customer]{Items: []models.Customer{}, Page: p.Page, PerPage: p.PerPage}, nil
}
func (stubSourceData) GetCustomer(context.Context, uuid.UUID) (models.Customer, error) {
	return models.Customer{}, nil
}
func (stubSourceData) ListProducts(_ context.Context, p service.ProductListParams) (service.PageResult[models.Product], error) {
	return service.PageResult[models.Product]{Items: []models.Product{}, Page: p.Page, PerPage: p.PerPage}, nil
}
func (stubSourceData) GetProduct(context.Context, uuid.UUID) (models.Product, error) {
	return models.Product{}, nil
}
func (stubSourceData) ListOrders(_ context.Context, p service.OrderListParams) (service.PageResult[models.Order], error) {
	return service.PageResult[models.Order]{Items: []models.Order{}, Page: p.Page, PerPage: p.PerPage}, nil
}
func (stubSourceData) GetOrder(context.Context, uuid.UUID) (models.Order, error) {
	return models.Order{}, nil
}
func (stubSourceData) ListOrderItems(_ context.Context, p service.OrderItemListParams) (service.PageResult[models.OrderItem], error) {
	return service.PageResult[models.OrderItem]{Items: []models.OrderItem{}, Page: p.Page, PerPage: p.PerPage}, nil
}
func (stubSourceData) GetOrderItem(context.Context, int64) (models.OrderItem, error) {
	return models.OrderItem{}, nil
}

var _ service.SourceData = stubSourceData{}

// stubTargetData extends stubSourceData with permissive analytics fakes.
type stubTargetData struct {
	stubSourceData
}

func (stubTargetData) Summary(context.Context) (models.AnalyticsSummary, error) {
	return models.AnalyticsSummary{OrdersByStatus: []models.OrderStatusCount{}}, nil
}
func (stubTargetData) TopProducts(context.Context, int) ([]models.TopProduct, error) {
	return []models.TopProduct{}, nil
}
func (stubTargetData) TopCustomers(context.Context, int) ([]models.TopCustomer, error) {
	return []models.TopCustomer{}, nil
}

var _ service.TargetData = stubTargetData{}

// stubJobControl is a permissive service.JobControl fake.
type stubJobControl struct{}

func (stubJobControl) Start(context.Context, string, any) (uuid.UUID, error) {
	return uuid.New(), nil
}
func (stubJobControl) Stop() error                     { return nil }
func (stubJobControl) Status() (jobs.JobStatus, error) { return jobs.JobStatus{Status: jobs.StatusIdle}, nil }
func (stubJobControl) Progress() (jobs.Progress, error) {
	return jobs.Progress{Status: jobs.StatusIdle}, nil
}
func (stubJobControl) History(context.Context, int, int) ([]models.JobExecution, int64, error) {
	return []models.JobExecution{}, 0, nil
}

var _ service.JobControl = stubJobControl{}

// stubSchedulerControl is a permissive service.SchedulerControl fake.
type stubSchedulerControl struct{}

func (stubSchedulerControl) List(context.Context) ([]models.CronSchedule, error) {
	return []models.CronSchedule{}, nil
}
func (stubSchedulerControl) Create(_ context.Context, p scheduler.CreateParams) (models.CronSchedule, error) {
	return models.CronSchedule{ID: uuid.New(), Name: p.Name, CronExpr: p.CronExpr, Command: p.Command}, nil
}
func (stubSchedulerControl) Update(_ context.Context, id uuid.UUID, p scheduler.UpdateParams) (models.CronSchedule, error) {
	return models.CronSchedule{ID: id, Name: p.Name, CronExpr: p.CronExpr, Command: p.Command}, nil
}
func (stubSchedulerControl) Delete(context.Context, uuid.UUID) error { return nil }
func (stubSchedulerControl) Enable(_ context.Context, id uuid.UUID) (models.CronSchedule, error) {
	return models.CronSchedule{ID: id, Enabled: true}, nil
}
func (stubSchedulerControl) Disable(_ context.Context, id uuid.UUID) (models.CronSchedule, error) {
	return models.CronSchedule{ID: id, Enabled: false}, nil
}
func (stubSchedulerControl) RunNow(context.Context, uuid.UUID) (uuid.UUID, error) {
	return uuid.New(), nil
}

var _ service.SchedulerControl = stubSchedulerControl{}

func mustParseUUID(t *testing.T) string {
	t.Helper()
	return uuid.NewString()
}

func get(t *testing.T, engine *gin.Engine, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestRouterHealthRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{
		Logger:  zap.NewNop(),
		Version: "test",
	})

	rec := get(t, engine, "/api/v1/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status %q, got %v", "ok", body["status"])
	}
	if body["version"] != "test" {
		t.Errorf("expected version %q, got %v", "test", body["version"])
	}
}

func TestRouterUnknownRouteReturnsErrorEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{Logger: zap.NewNop()})

	rec := get(t, engine, "/does-not-exist", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error envelope: %v (body: %s)", err, rec.Body.String())
	}
	if body.Error.Code != "not_found" {
		t.Errorf("expected code not_found, got %q", body.Error.Code)
	}
	if body.Error.Message != "route not found" {
		t.Errorf("unexpected message: %q", body.Error.Message)
	}
}

func TestRouterSourceRoutesRegisteredWithData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{
		Logger: zap.NewNop(),
		Data:   stubSourceData{},
	})

	id := mustParseUUID(t)
	cases := []struct {
		name string
		path string
	}{
		{"customers", "/api/v1/source/customers"},
		{"customer detail", "/api/v1/source/customers/" + id},
		{"products", "/api/v1/source/products"},
		{"product detail", "/api/v1/source/products/" + id},
		{"orders", "/api/v1/source/orders"},
		{"order detail", "/api/v1/source/orders/" + id},
		{"order items", "/api/v1/source/order-items"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, engine, tc.path, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s: expected %d, got %d (body: %s)", tc.path, http.StatusOK, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRouterSourceRoutesListEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{Logger: zap.NewNop(), Data: stubSourceData{}})

	rec := get(t, engine, "/api/v1/source/customers?page=2&per_page=5", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	var body struct {
		Data []any `json:"data"`
		Meta struct {
			Page    int `json:"page"`
			PerPage int `json:"per_page"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Meta.Page != 2 || body.Meta.PerPage != 5 {
		t.Errorf("unexpected meta: %+v", body.Meta)
	}
	if body.Data == nil {
		t.Error("expected data array")
	}
}

func TestRouterSourceRoutesAbsentWithoutData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{Logger: zap.NewNop()})

	rec := get(t, engine, "/api/v1/source/customers", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected source group to be unregistered (%d), got %d", http.StatusNotFound, rec.Code)
	}
}

func TestRouterOptionalMiddlewareAppliedToSourceGroupOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hook := func(c *gin.Context) {
		c.Header("X-Auth-Hook", "applied")
		// The hook must not run for health.
		c.Next()
	}

	engine := router.New(router.Dependencies{
		Logger:     zap.NewNop(),
		Data:       stubSourceData{},
		Middleware: []gin.HandlerFunc{hook},
	})

	sourceRec := get(t, engine, "/api/v1/source/customers", nil)
	if sourceRec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, sourceRec.Code)
	}
	if got := sourceRec.Header().Get("X-Auth-Hook"); got != "applied" {
		t.Errorf("expected middleware hook to run on the data group, got %q", got)
	}

	healthRec := get(t, engine, "/api/v1/health", nil)
	if got := healthRec.Header().Get("X-Auth-Hook"); got != "" {
		t.Errorf("middleware hook must not run on health, got %q", got)
	}
}

func TestRouterOptionalMiddlewareCanAbort(t *testing.T) {
	gin.SetMode(gin.TestMode)

	unauthorized := func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	}

	engine := router.New(router.Dependencies{
		Logger:     zap.NewNop(),
		Data:       stubSourceData{},
		Middleware: []gin.HandlerFunc{unauthorized},
	})

	if rec := get(t, engine, "/api/v1/source/products", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("expected data group to be guarded (%d), got %d", http.StatusUnauthorized, rec.Code)
	}
	if rec := get(t, engine, "/api/v1/health", nil); rec.Code != http.StatusOK {
		t.Errorf("health must stay open, got %d", rec.Code)
	}
}

func TestRouterCORSApplied(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{
		Logger: zap.NewNop(),
		CORS:   middleware.CORSConfig{AllowedOrigins: []string{"https://app.example"}},
	})

	rec := get(t, engine, "/api/v1/health", map[string]string{"Origin": "https://app.example"})
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example" {
		t.Errorf("Allow-Origin = %q, want configured origin", got)
	}

	denied := get(t, engine, "/api/v1/health", map[string]string{"Origin": "https://evil.example"})
	if got := denied.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Allow-Origin for disallowed origin, got %q", got)
	}
}

func TestRouterSwaggerUIRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{Logger: zap.NewNop()})

	rec := get(t, engine, "/swagger/index.html", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected Swagger UI at /swagger/index.html (%d), got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("expected Swagger UI HTML body")
	}
}

func TestRouterTargetRoutesRegisteredWithTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{
		Logger: zap.NewNop(),
		Target: stubTargetData{},
	})

	id := mustParseUUID(t)
	cases := []struct {
		name string
		path string
	}{
		{"customers", "/api/v1/target/customers"},
		{"customer detail", "/api/v1/target/customers/" + id},
		{"products", "/api/v1/target/products"},
		{"product detail", "/api/v1/target/products/" + id},
		{"orders", "/api/v1/target/orders"},
		{"order detail", "/api/v1/target/orders/" + id},
		{"analytics summary", "/api/v1/target/analytics/summary"},
		{"analytics top products", "/api/v1/target/analytics/top-products"},
		{"analytics top customers", "/api/v1/target/analytics/top-customers"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, engine, tc.path, nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s: expected %d, got %d (body: %s)", tc.path, http.StatusOK, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRouterTargetRoutesAbsentWithoutTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{Logger: zap.NewNop(), Data: stubSourceData{}})

	rec := get(t, engine, "/api/v1/target/customers", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected target group to be unregistered (%d), got %d", http.StatusNotFound, rec.Code)
	}
}

func TestRouterOptionalMiddlewareAppliedToTargetGroupOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hook := func(c *gin.Context) {
		c.Header("X-Auth-Hook", "applied")
		c.Next()
	}

	engine := router.New(router.Dependencies{
		Logger:     zap.NewNop(),
		Target:     stubTargetData{},
		Middleware: []gin.HandlerFunc{hook},
	})

	targetRec := get(t, engine, "/api/v1/target/analytics/summary", nil)
	if targetRec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, targetRec.Code)
	}
	if got := targetRec.Header().Get("X-Auth-Hook"); got != "applied" {
		t.Errorf("expected middleware hook to run on the target group, got %q", got)
	}

	healthRec := get(t, engine, "/api/v1/health", nil)
	if got := healthRec.Header().Get("X-Auth-Hook"); got != "" {
		t.Errorf("middleware hook must not run on health, got %q", got)
	}
}

// request executes an arbitrary method/body request against the engine.
func request(t *testing.T, engine *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestRouterJobRoutesRegisteredWithJobs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{
		Logger: zap.NewNop(),
		Jobs:   stubJobControl{},
	})

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"generate start", http.MethodPost, "/api/v1/jobs/generate/start", `{}`, http.StatusAccepted},
		{"migrate start", http.MethodPost, "/api/v1/jobs/migrate/start", `{}`, http.StatusAccepted},
		{"stop", http.MethodPost, "/api/v1/jobs/stop", "", http.StatusOK},
		{"status", http.MethodGet, "/api/v1/jobs/status", "", http.StatusOK},
		{"history", http.MethodGet, "/api/v1/jobs/history", "", http.StatusOK},
		{"progress", http.MethodGet, "/api/v1/jobs/progress", "", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := request(t, engine, tc.method, tc.path, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("%s %s: expected %d, got %d (body: %s)", tc.method, tc.path, tc.want, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRouterJobRoutesAbsentWithoutJobs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{Logger: zap.NewNop()})

	rec := request(t, engine, http.MethodGet, "/api/v1/jobs/status", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected jobs group to be unregistered (%d), got %d", http.StatusNotFound, rec.Code)
	}
}

func TestRouterSchedulerRoutesRegisteredWithScheduler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{
		Logger:    zap.NewNop(),
		Scheduler: stubSchedulerControl{},
	})

	id := mustParseUUID(t)
	cases := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"list", http.MethodGet, "/api/v1/scheduler/jobs", "", http.StatusOK},
		{"create", http.MethodPost, "/api/v1/scheduler/jobs", `{"name":"n","cron_expr":"0 3 * * *","command":"generate"}`, http.StatusCreated},
		{"update", http.MethodPut, "/api/v1/scheduler/jobs/" + id, `{"name":"n","cron_expr":"0 3 * * *","command":"generate"}`, http.StatusOK},
		{"delete", http.MethodDelete, "/api/v1/scheduler/jobs/" + id, "", http.StatusNoContent},
		{"enable", http.MethodPost, "/api/v1/scheduler/jobs/" + id + "/enable", "", http.StatusOK},
		{"disable", http.MethodPost, "/api/v1/scheduler/jobs/" + id + "/disable", "", http.StatusOK},
		{"run", http.MethodPost, "/api/v1/scheduler/jobs/" + id + "/run", "", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := request(t, engine, tc.method, tc.path, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("%s %s: expected %d, got %d (body: %s)", tc.method, tc.path, tc.want, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRouterSchedulerRoutesAbsentWithoutScheduler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{Logger: zap.NewNop()})

	rec := request(t, engine, http.MethodGet, "/api/v1/scheduler/jobs", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected scheduler group to be unregistered (%d), got %d", http.StatusNotFound, rec.Code)
	}
}

func TestRouterOptionalMiddlewareAppliedToJobsAndSchedulerGroups(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hook := func(c *gin.Context) {
		c.Header("X-Auth-Hook", "applied")
		c.Next()
	}

	engine := router.New(router.Dependencies{
		Logger:     zap.NewNop(),
		Jobs:       stubJobControl{},
		Scheduler:  stubSchedulerControl{},
		Middleware: []gin.HandlerFunc{hook},
	})

	for _, path := range []string{"/api/v1/jobs/status", "/api/v1/scheduler/jobs"} {
		rec := get(t, engine, path, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: expected %d, got %d", path, http.StatusOK, rec.Code)
		}
		if got := rec.Header().Get("X-Auth-Hook"); got != "applied" {
			t.Errorf("expected middleware hook on %s, got %q", path, got)
		}
	}

	healthRec := get(t, engine, "/api/v1/health", nil)
	if got := healthRec.Header().Get("X-Auth-Hook"); got != "" {
		t.Errorf("middleware hook must not run on health, got %q", got)
	}
}
