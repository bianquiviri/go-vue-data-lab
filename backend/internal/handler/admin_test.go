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

	"github.com/josebianco/go-vue-data-lab/internal/handler"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// fakeErrorLogs is a controllable service.ErrorLogs double.
type fakeErrorLogs struct {
	entries []service.LogEntry
	total   int
	err     error
	gotLim  int
	gotOff  int
}

func (f *fakeErrorLogs) RecentErrors(_ context.Context, limit, offset int) ([]service.LogEntry, int, error) {
	f.gotLim, f.gotOff = limit, offset
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.entries, f.total, nil
}

var _ service.ErrorLogs = (*fakeErrorLogs)(nil)

func adminErrors(t *testing.T, logs service.ErrorLogs, target string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.GET("/api/v1/admin/logs/errors", handler.NewAdminHandler(logs).Errors)

	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestAdminErrorsReturnsPaginatedEnvelope(t *testing.T) {
	logs := &fakeErrorLogs{
		entries: []service.LogEntry{{
			Time:    time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC),
			Level:   "ERROR",
			Message: "boom",
			Caller:  "internal/api/main.go:42",
		}},
		total: 41,
	}

	rec := adminErrors(t, logs, "/api/v1/admin/logs/errors?page=2&per_page=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if logs.gotLim != 10 || logs.gotOff != 10 {
		t.Errorf("service got limit=%d offset=%d, want 10/10", logs.gotLim, logs.gotOff)
	}

	var body handler.PaginatedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (body: %s)", err, rec.Body.String())
	}
	if body.Meta.Page != 2 || body.Meta.PerPage != 10 || body.Meta.Total != 41 || body.Meta.TotalPages != 5 {
		t.Errorf("unexpected meta: %+v", body.Meta)
	}

	entries, ok := body.Data.([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("expected 1 entry in data, got %#v", body.Data)
	}
	entry, ok := entries[0].(map[string]any)
	if !ok {
		t.Fatalf("expected object entry, got %#v", entries[0])
	}
	if entry["message"] != "boom" || entry["level"] != "ERROR" {
		t.Errorf("unexpected entry: %+v", entry)
	}
}

func TestAdminErrorsEmptyListIsNotAnError(t *testing.T) {
	logs := &fakeErrorLogs{entries: nil, total: 0}

	rec := adminErrors(t, logs, "/api/v1/admin/logs/errors")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	var body struct {
		Data []any `json:"data"`
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data == nil {
		t.Error("expected data to be an empty array, not null")
	}
	if body.Meta.Total != 0 {
		t.Errorf("total = %d, want 0", body.Meta.Total)
	}
}

func TestAdminErrorsRejectsInvalidPagination(t *testing.T) {
	logs := &fakeErrorLogs{}

	rec := adminErrors(t, logs, "/api/v1/admin/logs/errors?per_page=500")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != handler.CodeValidation {
		t.Errorf("code = %q, want %q", body.Error.Code, handler.CodeValidation)
	}
	if logs.gotLim != 0 {
		t.Error("service must not be called on validation failure")
	}
}

func TestAdminErrorsMapsServiceErrorTo500(t *testing.T) {
	logs := &fakeErrorLogs{err: errors.New("disk on fire")}

	rec := adminErrors(t, logs, "/api/v1/admin/logs/errors")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != handler.CodeInternal {
		t.Errorf("code = %q, want %q", body.Error.Code, handler.CodeInternal)
	}
	if body.Error.Message == "disk on fire" {
		t.Error("internal error details must not leak to the client")
	}
}
