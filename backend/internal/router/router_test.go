package router_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/josebianco/go-vue-data-lab/internal/auth"
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
func (stubJobControl) Stop() error { return nil }
func (stubJobControl) Status() (jobs.JobStatus, error) {
	return jobs.JobStatus{Status: jobs.StatusIdle}, nil
}
func (stubJobControl) Progress() (jobs.Progress, error) {
	return jobs.Progress{Status: jobs.StatusIdle}, nil
}
func (stubJobControl) Subscribe(context.Context) <-chan jobs.Progress {
	ch := make(chan jobs.Progress)
	close(ch)
	return ch
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

func TestRouterSSEMiddlewareAppliedToJobsEventsOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sseHook := func(c *gin.Context) {
		c.Header("X-SSE-Hook", "applied")
		c.Next()
	}
	baseHook := func(c *gin.Context) {
		c.Header("X-Base-Hook", "applied")
		c.Next()
	}

	engine := router.New(router.Dependencies{
		Logger:        zap.NewNop(),
		Jobs:          stubJobControl{},
		Middleware:    []gin.HandlerFunc{baseHook},
		SSEMiddleware: []gin.HandlerFunc{sseHook},
	})

	eventsRec := get(t, engine, "/api/v1/jobs/events", nil)
	if eventsRec.Code != http.StatusOK {
		t.Fatalf("GET /jobs/events: expected %d, got %d (body: %s)", http.StatusOK, eventsRec.Code, eventsRec.Body.String())
	}
	if got := eventsRec.Header().Get("X-SSE-Hook"); got != "applied" {
		t.Errorf("expected the SSE middleware on /jobs/events, got %q", got)
	}
	// The SSE route is mounted outside the /jobs group chain: the group guard
	// must not run on it, otherwise a browser EventSource request (which cannot
	// send an Authorization header) would be rejected before the SSE guard.
	if got := eventsRec.Header().Get("X-Base-Hook"); got != "" {
		t.Errorf("the base middleware must not run on /jobs/events, got %q", got)
	}

	statusRec := get(t, engine, "/api/v1/jobs/status", nil)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("GET /jobs/status: expected %d, got %d", http.StatusOK, statusRec.Code)
	}
	if got := statusRec.Header().Get("X-SSE-Hook"); got != "" {
		t.Errorf("SSE middleware must not run on /jobs/status, got %q", got)
	}
	if got := statusRec.Header().Get("X-Base-Hook"); got != "applied" {
		t.Errorf("expected the base middleware on /jobs/status, got %q", got)
	}
}

// TestRouterSSEEventsAcceptsEventSourceQueryToken reproduces the production
// wiring of cmd/api/main.go: the jobs group carries the bearer guard
// (middleware.AuthRequired) and the SSE route carries the query-token guard
// (middleware.AuthRequiredSSE). A browser EventSource cannot send the
// Authorization header, so the stream must be accepted with ?token= alone.
func TestRouterSSEEventsAcceptsEventSourceQueryToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tokens := auth.NewJWTManager("router-sse-secret", 15*time.Minute, time.Hour)
	access, refresh, err := tokens.GenerateTokens(uuid.New(), "root@gmail.com", models.RoleAdmin)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	engine := router.New(router.Dependencies{
		Logger:        zap.NewNop(),
		Jobs:          stubJobControl{},
		Middleware:    []gin.HandlerFunc{middleware.AuthRequired(tokens)},
		SSEMiddleware: []gin.HandlerFunc{middleware.AuthRequiredSSE(tokens)},
	})

	rec := get(t, engine, "/api/v1/jobs/events?token="+access, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /jobs/events?token=<access> = %d, want 200 (EventSource cannot send headers); body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", got)
	}

	// A refresh token is never accepted on the stream.
	refreshRec := get(t, engine, "/api/v1/jobs/events?token="+refresh, nil)
	if refreshRec.Code != http.StatusUnauthorized {
		t.Errorf("GET /jobs/events?token=<refresh> = %d, want 401 (body: %s)", refreshRec.Code, refreshRec.Body.String())
	}

	// A missing or garbage token is rejected too.
	for _, path := range []string{"/api/v1/jobs/events", "/api/v1/jobs/events?token=not-a-jwt"} {
		badRec := get(t, engine, path, nil)
		if badRec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s = %d, want 401", path, badRec.Code)
		}
	}
}

// TestRouterSSEEventsRouteIsFailClosed asserts the SSE route is never
// registered open: without any guard the endpoint must not exist at all, and
// with only the bearer guard it must still reject anonymous requests.
func TestRouterSSEEventsRouteIsFailClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	unguarded := router.New(router.Dependencies{
		Logger: zap.NewNop(),
		Jobs:   stubJobControl{},
	})

	rec := get(t, unguarded, "/api/v1/jobs/events", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected the SSE route to be unregistered without a guard (%d), got %d", http.StatusNotFound, rec.Code)
	}

	// Bearer-only fallback: the route exists for non-browser clients but still
	// requires a token, and it does not accept the ?token= query parameter.
	tokens := auth.NewJWTManager("router-sse-secret", 15*time.Minute, time.Hour)
	access, _, err := tokens.GenerateTokens(uuid.New(), "root@gmail.com", models.RoleAdmin)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	engine := router.New(router.Dependencies{
		Logger:     zap.NewNop(),
		Jobs:       stubJobControl{},
		Middleware: []gin.HandlerFunc{middleware.AuthRequired(tokens)},
	})

	anonRec := get(t, engine, "/api/v1/jobs/events", nil)
	if anonRec.Code != http.StatusUnauthorized {
		t.Errorf("GET /jobs/events without a token = %d, want 401", anonRec.Code)
	}
	queryRec := get(t, engine, "/api/v1/jobs/events?token="+access, nil)
	if queryRec.Code != http.StatusUnauthorized {
		t.Errorf("GET /jobs/events?token=<access> with a bearer-only guard = %d, want 401", queryRec.Code)
	}
	headerRec := get(t, engine, "/api/v1/jobs/events", map[string]string{"Authorization": "Bearer " + access})
	if headerRec.Code != http.StatusOK {
		t.Errorf("GET /jobs/events with a bearer token = %d, want 200 (body: %s)", headerRec.Code, headerRec.Body.String())
	}
}

func TestRouterSSEMiddlewareCanAbortEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	unauthorized := func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	}

	engine := router.New(router.Dependencies{
		Logger:        zap.NewNop(),
		Jobs:          stubJobControl{},
		SSEMiddleware: []gin.HandlerFunc{unauthorized},
	})

	rec := get(t, engine, "/api/v1/jobs/events", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected the SSE guard to reject (%d), got %d", http.StatusUnauthorized, rec.Code)
	}
	// The other job routes stay untouched by the SSE guard.
	if rec := get(t, engine, "/api/v1/jobs/status", nil); rec.Code != http.StatusOK {
		t.Errorf("job status must stay open to the base middleware, got %d", rec.Code)
	}
}

func TestRouterJobRoutesAbsentWithoutJobs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{Logger: zap.NewNop()})

	rec := request(t, engine, http.MethodGet, "/api/v1/jobs/status", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected jobs group to be unregistered (%d), got %d", http.StatusNotFound, rec.Code)
	}

	eventsRec := get(t, engine, "/api/v1/jobs/events", nil)
	if eventsRec.Code != http.StatusNotFound {
		t.Fatalf("expected SSE route to be unregistered (%d), got %d", http.StatusNotFound, eventsRec.Code)
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

func TestRouterSourceWriteRoutesRegisteredWithData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{Logger: zap.NewNop(), Data: stubSourceData{}})
	id := mustParseUUID(t)
	customerBody := `{"first_name":"A","last_name":"B","email":"a@b.com","country_code":"US","city":"NY"}`
	productBody := `{"sku":"S","name":"N","category":"C","price":1,"stock":1}`
	orderBody := `{"customer_id":"` + id + `"}`

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"create customer", http.MethodPost, "/api/v1/source/customers", customerBody, http.StatusCreated},
		{"update customer", http.MethodPut, "/api/v1/source/customers/" + id, customerBody, http.StatusOK},
		{"delete customer", http.MethodDelete, "/api/v1/source/customers/" + id, "", http.StatusNoContent},
		{"create product", http.MethodPost, "/api/v1/source/products", productBody, http.StatusCreated},
		{"update product", http.MethodPut, "/api/v1/source/products/" + id, productBody, http.StatusOK},
		{"delete product", http.MethodDelete, "/api/v1/source/products/" + id, "", http.StatusNoContent},
		{"create order", http.MethodPost, "/api/v1/source/orders", orderBody, http.StatusCreated},
		{"update order", http.MethodPut, "/api/v1/source/orders/" + id, `{"status":"shipped"}`, http.StatusOK},
		{"delete order", http.MethodDelete, "/api/v1/source/orders/" + id, "", http.StatusNoContent},
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

func TestRouterSourceWriteRoutesAbsentWithoutData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{Logger: zap.NewNop()})

	rec := request(t, engine, http.MethodPost, "/api/v1/source/customers", `{}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected source group to be unregistered (%d), got %d", http.StatusNotFound, rec.Code)
	}
}

func TestRouterWriteMiddlewareAppliedToSourceWritesOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	base := func(c *gin.Context) {
		c.Header("X-Base-Hook", "applied")
		c.Next()
	}
	write := func(c *gin.Context) {
		c.Header("X-Write-Hook", "applied")
		c.Next()
	}

	engine := router.New(router.Dependencies{
		Logger:          zap.NewNop(),
		Data:            stubSourceData{},
		Middleware:      []gin.HandlerFunc{base},
		WriteMiddleware: []gin.HandlerFunc{write},
	})

	readRec := request(t, engine, http.MethodGet, "/api/v1/source/customers", "")
	if readRec.Code != http.StatusOK {
		t.Fatalf("GET: expected %d, got %d", http.StatusOK, readRec.Code)
	}
	if got := readRec.Header().Get("X-Base-Hook"); got != "applied" {
		t.Errorf("base middleware should run on reads, got %q", got)
	}
	if got := readRec.Header().Get("X-Write-Hook"); got != "" {
		t.Errorf("write middleware must not run on reads, got %q", got)
	}

	writeRec := request(t, engine, http.MethodPost, "/api/v1/source/customers",
		`{"first_name":"A","last_name":"B","email":"a@b.com","country_code":"US","city":"NY"}`)
	if writeRec.Code != http.StatusCreated {
		t.Fatalf("POST: expected %d, got %d (body: %s)", http.StatusCreated, writeRec.Code, writeRec.Body.String())
	}
	if got := writeRec.Header().Get("X-Base-Hook"); got != "applied" {
		t.Errorf("base middleware should also run on writes, got %q", got)
	}
	if got := writeRec.Header().Get("X-Write-Hook"); got != "applied" {
		t.Errorf("write middleware should run on writes, got %q", got)
	}
}

func TestRouterWriteMiddlewareCanAbortWritesOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	forbidden := func(c *gin.Context) {
		c.AbortWithStatus(http.StatusForbidden)
	}

	engine := router.New(router.Dependencies{
		Logger:          zap.NewNop(),
		Data:            stubSourceData{},
		WriteMiddleware: []gin.HandlerFunc{forbidden},
	})

	if rec := request(t, engine, http.MethodPost, "/api/v1/source/products", `{"sku":"S","name":"N","category":"C","price":1,"stock":1}`); rec.Code != http.StatusForbidden {
		t.Errorf("expected writes guarded (%d), got %d", http.StatusForbidden, rec.Code)
	}
	if rec := request(t, engine, http.MethodGet, "/api/v1/source/products", ""); rec.Code != http.StatusOK {
		t.Errorf("reads must stay open, got %d", rec.Code)
	}
}

// stubErrorLogs is a permissive service.ErrorLogs fake.
type stubErrorLogs struct{}

func (stubErrorLogs) RecentErrors(context.Context, int, int) ([]service.LogEntry, int, error) {
	return []service.LogEntry{}, 0, nil
}

var _ service.ErrorLogs = stubErrorLogs{}

func TestRouterAdminRouteRegisteredWhenGuarded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{
		Logger:          zap.NewNop(),
		Admin:           stubErrorLogs{},
		AdminMiddleware: []gin.HandlerFunc{func(c *gin.Context) { c.Next() }},
	})

	if rec := get(t, engine, "/api/v1/admin/logs/errors", nil); rec.Code != http.StatusOK {
		t.Fatalf("expected admin route (%d), got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestRouterAdminRouteAbsentWithoutGuardOrService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name string
		deps router.Dependencies
	}{
		{"service without middleware", router.Dependencies{Admin: stubErrorLogs{}}},
		{"middleware without service", router.Dependencies{
			AdminMiddleware: []gin.HandlerFunc{func(c *gin.Context) { c.Next() }},
		}},
		{"neither", router.Dependencies{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.deps.Logger = zap.NewNop()
			engine := router.New(tc.deps)
			if rec := get(t, engine, "/api/v1/admin/logs/errors", nil); rec.Code != http.StatusNotFound {
				t.Fatalf("expected admin route to be unregistered (%d), got %d", http.StatusNotFound, rec.Code)
			}
		})
	}
}

func TestRouterAdminMiddlewareRunsInOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authHook := func(c *gin.Context) {
		c.Header("X-Auth-Hook", "applied")
		c.Next()
	}
	rbacHook := func(c *gin.Context) {
		c.AbortWithStatus(http.StatusForbidden)
	}

	engine := router.New(router.Dependencies{
		Logger:          zap.NewNop(),
		Admin:           stubErrorLogs{},
		AdminMiddleware: []gin.HandlerFunc{authHook, rbacHook},
	})

	rec := get(t, engine, "/api/v1/admin/logs/errors", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected the admin guard to reject (%d), got %d", http.StatusForbidden, rec.Code)
	}
	if got := rec.Header().Get("X-Auth-Hook"); got != "applied" {
		t.Errorf("expected the auth hook to run first, got %q", got)
	}
	if rec := get(t, engine, "/api/v1/health", nil); rec.Code != http.StatusOK {
		t.Errorf("health must stay open, got %d", rec.Code)
	}
}

func TestRouterWriteMiddlewareAppliedToJobsAndSchedulerWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)

	write := func(c *gin.Context) {
		c.Header("X-Write-Hook", "applied")
		c.Next()
	}

	engine := router.New(router.Dependencies{
		Logger:          zap.NewNop(),
		Jobs:            stubJobControl{},
		Scheduler:       stubSchedulerControl{},
		WriteMiddleware: []gin.HandlerFunc{write},
	})

	id := mustParseUUID(t)
	cases := []struct {
		name    string
		method  string
		path    string
		body    string
		guarded bool
	}{
		{"job start write", http.MethodPost, "/api/v1/jobs/generate/start", `{}`, true},
		{"job stop write", http.MethodPost, "/api/v1/jobs/stop", "", true},
		{"job status read", http.MethodGet, "/api/v1/jobs/status", "", false},
		{"job history read", http.MethodGet, "/api/v1/jobs/history", "", false},
		{"schedule create write", http.MethodPost, "/api/v1/scheduler/jobs", `{"name":"n","cron_expr":"0 3 * * *","command":"generate"}`, true},
		{"schedule run write", http.MethodPost, "/api/v1/scheduler/jobs/" + id + "/run", "", true},
		{"schedule list read", http.MethodGet, "/api/v1/scheduler/jobs", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := request(t, engine, tc.method, tc.path, tc.body)
			if rec.Code >= 400 {
				t.Fatalf("%s %s: expected success, got %d (body: %s)", tc.method, tc.path, rec.Code, rec.Body.String())
			}
			got := rec.Header().Get("X-Write-Hook") == "applied"
			if got != tc.guarded {
				t.Errorf("%s %s: write hook applied = %v, want %v", tc.method, tc.path, got, tc.guarded)
			}
		})
	}
}

// TestRouterSourceRBACMap pins the approved role map end-to-end: reads are
// open to admin/operator/viewer, while POST/PUT/DELETE need admin/operator.
func TestRouterSourceRBACMap(t *testing.T) {
	gin.SetMode(gin.TestMode)

	asRole := func(r models.UserRole) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Set(middleware.ContextRole, string(r))
			c.Next()
		}
	}

	cases := []struct {
		role       models.UserRole
		writeClaim int
	}{
		{models.RoleAdmin, http.StatusCreated},
		{models.RoleOperator, http.StatusCreated},
		{models.RoleViewer, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(string(tc.role), func(t *testing.T) {
			engine := router.New(router.Dependencies{
				Logger:          zap.NewNop(),
				Data:            stubSourceData{},
				Middleware:      []gin.HandlerFunc{asRole(tc.role)},
				WriteMiddleware: []gin.HandlerFunc{middleware.RBAC(models.RoleAdmin, models.RoleOperator)},
			})

			if rec := get(t, engine, "/api/v1/source/customers", nil); rec.Code != http.StatusOK {
				t.Errorf("read for %s: expected %d, got %d", tc.role, http.StatusOK, rec.Code)
			}

			rec := request(t, engine, http.MethodPost, "/api/v1/source/customers",
				`{"first_name":"A","last_name":"B","email":"a@b.com","country_code":"US","city":"NY"}`)
			if rec.Code != tc.writeClaim {
				t.Errorf("write for %s: expected %d, got %d (body: %s)", tc.role, tc.writeClaim, rec.Code, rec.Body.String())
			}
		})
	}
}

func (stubSourceData) CreateCustomer(context.Context, service.CustomerInput) (models.Customer, error) {
	return models.Customer{}, nil
}
func (stubSourceData) UpdateCustomer(context.Context, uuid.UUID, service.CustomerInput) (models.Customer, error) {
	return models.Customer{}, nil
}
func (stubSourceData) DeleteCustomer(context.Context, uuid.UUID) error { return nil }
func (stubSourceData) CreateProduct(context.Context, service.ProductInput) (models.Product, error) {
	return models.Product{}, nil
}
func (stubSourceData) UpdateProduct(context.Context, uuid.UUID, service.ProductInput) (models.Product, error) {
	return models.Product{}, nil
}
func (stubSourceData) DeleteProduct(context.Context, uuid.UUID) error { return nil }
func (stubSourceData) CreateOrder(context.Context, service.OrderCreateInput) (models.Order, error) {
	return models.Order{}, nil
}
func (stubSourceData) UpdateOrder(context.Context, uuid.UUID, service.OrderUpdateInput) (models.Order, error) {
	return models.Order{}, nil
}
func (stubSourceData) DeleteOrder(context.Context, uuid.UUID) error { return nil }
