package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/josebianco/go-vue-data-lab/internal/middleware"
)

// runCORS executes a request through the CORS middleware plus an inner
// handler, returning the recorder.
func runCORS(t *testing.T, cfg middleware.CORSConfig, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(middleware.CORS(cfg))
	engine.Any("/*path", func(c *gin.Context) {
		c.String(http.StatusOK, "handler")
	})

	req := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestCORSAllowedOriginSimpleRequest(t *testing.T) {
	rec := runCORS(t, middleware.CORSConfig{}, http.MethodGet, "/api/v1/health",
		map[string]string{"Origin": "http://localhost:5173"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Allow-Origin = %q, want the request origin", got)
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Methods"), "GET") {
		t.Errorf("expected GET in Allow-Methods, got %q", rec.Header().Get("Access-Control-Allow-Methods"))
	}
	if rec.Header().Get("Access-Control-Max-Age") == "" {
		t.Error("expected Access-Control-Max-Age header")
	}
	if v := rec.Header().Values("Vary"); len(v) == 0 {
		t.Error("expected Vary: Origin header")
	}
	// The downstream handler must still run.
	if rec.Body.String() != "handler" {
		t.Errorf("expected handler body, got %q", rec.Body.String())
	}
}

func TestCORSDefaultOriginFromDefaultList(t *testing.T) {
	rec := runCORS(t, middleware.CORSConfig{}, http.MethodGet, "/x",
		map[string]string{"Origin": "https://frontgodata.josebianco.local"})

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://frontgodata.josebianco.local" {
		t.Errorf("Allow-Origin = %q, want app URL origin", got)
	}
}

func TestCORSDisallowedOriginGetsNoHeaders(t *testing.T) {
	rec := runCORS(t, middleware.CORSConfig{AllowedOrigins: []string{"https://allowed.example"}},
		http.MethodGet, "/x", map[string]string{"Origin": "https://evil.example"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected handler to run with %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Allow-Origin for disallowed origin, got %q", got)
	}
}

func TestCORSConfiguredOrigins(t *testing.T) {
	rec := runCORS(t, middleware.CORSConfig{AllowedOrigins: []string{"https://custom.example", "http://localhost:4173"}},
		http.MethodGet, "/x", map[string]string{"Origin": "http://localhost:4173"})

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:4173" {
		t.Errorf("Allow-Origin = %q, want configured origin", got)
	}
}

func TestCORSWildcardOrigin(t *testing.T) {
	rec := runCORS(t, middleware.CORSConfig{AllowedOrigins: []string{"*"}},
		http.MethodGet, "/x", map[string]string{"Origin": "https://anything.example"})

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Allow-Origin = %q, want *", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("credentials must not accompany wildcard origin, got %q", got)
	}
}

func TestCORSPreflightAllowedOrigin(t *testing.T) {
	rec := runCORS(t, middleware.CORSConfig{}, http.MethodOptions, "/api/v1/source/customers",
		map[string]string{
			"Origin":                         "http://localhost:5173",
			"Access-Control-Request-Method":  http.MethodGet,
			"Access-Control-Request-Headers": "Authorization",
		})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d for preflight, got %d", http.StatusNoContent, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Allow-Origin = %q", got)
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Errorf("expected Authorization in Allow-Headers, got %q", rec.Header().Get("Access-Control-Allow-Headers"))
	}
	if rec.Body.Len() != 0 {
		t.Errorf("expected empty preflight body, got %q", rec.Body.String())
	}
}

func TestCORSPreflightDisallowedOrigin(t *testing.T) {
	rec := runCORS(t, middleware.CORSConfig{AllowedOrigins: []string{"https://allowed.example"}},
		http.MethodOptions, "/x",
		map[string]string{
			"Origin":                        "https://evil.example",
			"Access-Control-Request-Method": http.MethodGet,
		})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Allow-Origin on rejected preflight, got %q", got)
	}
}

func TestCORSOptionsWithoutPreflightFlagPassesThrough(t *testing.T) {
	rec := runCORS(t, middleware.CORSConfig{}, http.MethodOptions, "/x",
		map[string]string{"Origin": "http://localhost:5173"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected plain OPTIONS to reach handler (%d), got %d", http.StatusOK, rec.Code)
	}
	if rec.Body.String() != "handler" {
		t.Errorf("expected handler body, got %q", rec.Body.String())
	}
}

func TestCORSNoOriginRequest(t *testing.T) {
	rec := runCORS(t, middleware.CORSConfig{}, http.MethodGet, "/x", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS headers without Origin, got %q", got)
	}
}

func TestCORSAllowCredentials(t *testing.T) {
	rec := runCORS(t, middleware.CORSConfig{
		AllowedOrigins:   []string{"https://app.example"},
		AllowCredentials: true,
	}, http.MethodGet, "/x", map[string]string{"Origin": "https://app.example"})

	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q, want true", got)
	}
}

func TestCORSCustomMethodsHeadersMaxAge(t *testing.T) {
	rec := runCORS(t, middleware.CORSConfig{
		AllowedOrigins: []string{"https://app.example"},
		AllowedMethods: []string{http.MethodGet},
		AllowedHeaders: []string{"X-Custom"},
		MaxAge:         "60",
	}, http.MethodGet, "/x", map[string]string{"Origin": "https://app.example"})

	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET" {
		t.Errorf("Allow-Methods = %q, want GET", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "X-Custom" {
		t.Errorf("Allow-Headers = %q, want X-Custom", got)
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "60" {
		t.Errorf("Max-Age = %q, want 60", got)
	}
}
