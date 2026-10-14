package router_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/josebianco/go-vue-data-lab/internal/router"
)

func TestRouterHealthRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{
		Logger:  zap.NewNop(),
		Version: "test",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

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

func TestRouterUnknownRouteReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{Logger: zap.NewNop()})

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}
