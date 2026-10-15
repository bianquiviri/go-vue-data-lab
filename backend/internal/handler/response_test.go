package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/josebianco/go-vue-data-lab/internal/handler"
)

func newTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	return c, rec
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) handler.ErrorResponse {
	t.Helper()
	var resp handler.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error envelope: %v (body: %s)", err, rec.Body.String())
	}
	return resp
}

func TestRespondJSON(t *testing.T) {
	c, rec := newTestContext(http.MethodGet, "/", "")

	handler.RespondJSON(c, http.StatusCreated, map[string]string{"id": "1"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("expected JSON content type, got %q", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["id"] != "1" {
		t.Errorf("expected id 1, got %q", body["id"])
	}
}

func TestRespondErrorEnvelopeShape(t *testing.T) {
	c, rec := newTestContext(http.MethodGet, "/", "")

	handler.RespondError(c, http.StatusNotFound, handler.CodeNotFound, "customer not found")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	resp := decodeError(t, rec)
	if resp.Error.Code != "not_found" {
		t.Errorf("expected code not_found, got %q", resp.Error.Code)
	}
	if resp.Error.Message != "customer not found" {
		t.Errorf("expected message %q, got %q", "customer not found", resp.Error.Message)
	}
	if resp.Error.Details != nil {
		t.Errorf("expected no details, got %+v", resp.Error.Details)
	}
}

func TestRespondErrorDetailsEnvelope(t *testing.T) {
	c, rec := newTestContext(http.MethodGet, "/", "")

	handler.RespondErrorDetails(c, http.StatusBadRequest, handler.CodeValidation,
		"validation failed", map[string]any{"fields": map[string]string{"Page": "failed on 'min' validation"}})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	resp := decodeError(t, rec)
	if resp.Error.Code != handler.CodeValidation {
		t.Errorf("expected code %s, got %q", handler.CodeValidation, resp.Error.Code)
	}
	fields, ok := resp.Error.Details["fields"].(map[string]any)
	if !ok {
		t.Fatalf("expected details.fields map, got %+v", resp.Error.Details)
	}
	if fields["Page"] == "" {
		t.Error("expected field-level detail for Page")
	}
}

func TestRespondValidationErrorWithFieldDetails(t *testing.T) {
	c, rec := newTestContext(http.MethodPost, "/", `{}`)

	// Build a real binding failure the same way handlers do.
	var q struct {
		Page    int `form:"page" binding:"omitempty,min=1"`
		PerPage int `form:"per_page" binding:"omitempty,min=1,max=10"`
	}
	c.Request.URL.RawQuery = "page=-1&per_page=500"
	err := c.ShouldBindQuery(&q)
	if err == nil {
		t.Fatal("expected binding to fail")
	}

	handler.RespondValidationError(c, err)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	resp := decodeError(t, rec)
	if resp.Error.Code != handler.CodeValidation {
		t.Errorf("expected code %s, got %q", handler.CodeValidation, resp.Error.Code)
	}
	fields, ok := resp.Error.Details["fields"].(map[string]any)
	if !ok {
		t.Fatalf("expected details.fields, got %+v", resp.Error.Details)
	}
	for _, name := range []string{"Page", "PerPage"} {
		if fields[name] == nil {
			t.Errorf("expected field detail for %s, got %+v", name, fields)
		}
	}
}

func TestRespondValidationErrorWithNonFieldError(t *testing.T) {
	c, rec := newTestContext(http.MethodPost, "/", ``)

	handler.RespondValidationError(c, errors.New("invalid request body"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	resp := decodeError(t, rec)
	if resp.Error.Code != handler.CodeValidation {
		t.Errorf("expected code %s, got %q", handler.CodeValidation, resp.Error.Code)
	}
	if resp.Error.Details["reason"] != "invalid request body" {
		t.Errorf("expected reason detail, got %+v", resp.Error.Details)
	}
}

func TestRespondNotFound(t *testing.T) {
	c, rec := newTestContext(http.MethodGet, "/", "")

	handler.RespondNotFound(c, "product not found")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
	if resp := decodeError(t, rec); resp.Error.Code != handler.CodeNotFound {
		t.Errorf("expected code %s, got %q", handler.CodeNotFound, resp.Error.Code)
	}
}

func TestRespondInternalErrorHidesDetailsAndKeepsError(t *testing.T) {
	c, rec := newTestContext(http.MethodGet, "/", "")

	handler.RespondInternalError(c, errors.New("boom: secret internals"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "secret internals") {
		t.Errorf("internal error leaked to response: %s", body)
	}
	resp := decodeError(t, rec)
	if resp.Error.Code != handler.CodeInternal {
		t.Errorf("expected code %s, got %q", handler.CodeInternal, resp.Error.Code)
	}
	if resp.Error.Message != "internal server error" {
		t.Errorf("expected generic message, got %q", resp.Error.Message)
	}
	if len(c.Errors) != 1 {
		t.Fatalf("expected the error to be attached to the context, got %d errors", len(c.Errors))
	}
}

func TestRespondInternalErrorWithNilError(t *testing.T) {
	c, rec := newTestContext(http.MethodGet, "/", "")

	handler.RespondInternalError(c, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
	if len(c.Errors) != 0 {
		t.Errorf("expected no context errors for nil err, got %d", len(c.Errors))
	}
}

func TestRespondBadRequest(t *testing.T) {
	c, rec := newTestContext(http.MethodGet, "/", "")

	handler.RespondBadRequest(c, "bad uuid")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	resp := decodeError(t, rec)
	if resp.Error.Code != handler.CodeBadRequest {
		t.Errorf("expected code %s, got %q", handler.CodeBadRequest, resp.Error.Code)
	}
	if resp.Error.Message != "bad uuid" {
		t.Errorf("expected message %q, got %q", "bad uuid", resp.Error.Message)
	}
}

func TestRespondConflict(t *testing.T) {
	c, rec := newTestContext(http.MethodPost, "/", "")

	handler.RespondConflict(c, "a job is already running")

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec.Code)
	}
	resp := decodeError(t, rec)
	if resp.Error.Code != handler.CodeConflict {
		t.Errorf("expected code %s, got %q", handler.CodeConflict, resp.Error.Code)
	}
	if resp.Error.Message != "a job is already running" {
		t.Errorf("unexpected message: %q", resp.Error.Message)
	}
}

func TestRespondDataEnvelope(t *testing.T) {
	c, rec := newTestContext(http.MethodGet, "/", "")

	handler.RespondData(c, http.StatusOK, map[string]string{"status": "running"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	var body struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data["status"] != "running" {
		t.Errorf("unexpected data envelope: %+v", body.Data)
	}
}
