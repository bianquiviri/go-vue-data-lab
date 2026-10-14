package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/josebianco/go-vue-data-lab/internal/handler"
)

func TestHealthHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.GET("/api/v1/health", handler.NewHealthHandler("test-version").Health)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body handler.HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf("expected status %q, got %q", "ok", body.Status)
	}
	if body.Version != "test-version" {
		t.Errorf("expected version %q, got %q", "test-version", body.Version)
	}
	if _, err := time.Parse(time.RFC3339, body.Time); err != nil {
		t.Errorf("expected RFC3339 time, got %q: %v", body.Time, err)
	}
}
