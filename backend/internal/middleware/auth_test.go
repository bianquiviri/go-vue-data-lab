package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/auth"
	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/middleware"
)

func newTestManager() *auth.JWTManager {
	return auth.NewJWTManager("middleware-test-secret", 15*time.Minute, time.Hour)
}

func newEngine(tokens *auth.JWTManager, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/protected", middleware.AuthRequired(tokens), handler)
	return engine
}

func doRequest(engine *gin.Engine, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestAuthRequiredMissingHeader(t *testing.T) {
	engine := newEngine(newTestManager(), func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := doRequest(engine, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthRequiredInvalidToken(t *testing.T) {
	engine := newEngine(newTestManager(), func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := doRequest(engine, "Bearer not-a-token")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthRequiredRejectsRefreshToken(t *testing.T) {
	m := newTestManager()
	_, refresh, err := m.GenerateTokens(uuid.New(), "user@example.com", models.RoleViewer)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	engine := newEngine(m, func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := doRequest(engine, "Bearer "+refresh)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthRequiredSetsContext(t *testing.T) {
	m := newTestManager()
	userID := uuid.New()
	access, _, err := m.GenerateTokens(userID, "user@example.com", models.RoleAdmin)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	var (
		gotID    uuid.UUID
		gotEmail string
		gotRole  string
		ok       bool
	)
	engine := newEngine(m, func(c *gin.Context) {
		gotID, ok = middleware.CurrentUserID(c)
		gotEmail = c.GetString(middleware.ContextEmail)
		gotRole = c.GetString(middleware.ContextRole)
		c.Status(http.StatusOK)
	})

	rec := doRequest(engine, "Bearer "+access)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !ok || gotID != userID {
		t.Errorf("CurrentUserID() = (%v, %v), want (%v, true)", gotID, ok, userID)
	}
	if gotEmail != "user@example.com" {
		t.Errorf("email = %q, want %q", gotEmail, "user@example.com")
	}
	if gotRole != string(models.RoleAdmin) {
		t.Errorf("role = %q, want %q", gotRole, models.RoleAdmin)
	}
}

func TestAuthRequiredAcceptsBareToken(t *testing.T) {
	m := newTestManager()
	access, _, err := m.GenerateTokens(uuid.New(), "user@example.com", models.RoleViewer)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	engine := newEngine(m, func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := doRequest(engine, access)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthRequiredNilManager(t *testing.T) {
	engine := newEngine(nil, func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := doRequest(engine, "Bearer whatever")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRBACAllowsPermittedRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/admin",
		func(c *gin.Context) { c.Set(middleware.ContextRole, string(models.RoleAdmin)); c.Next() },
		middleware.RBAC(models.RoleAdmin),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRBACDeniesUnpermittedRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/admin",
		func(c *gin.Context) { c.Set(middleware.ContextRole, string(models.RoleViewer)); c.Next() },
		middleware.RBAC(models.RoleAdmin),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRBACWithoutRoleIsUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/admin",
		middleware.RBAC(models.RoleAdmin),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRBACAllowsAnyOfMultipleRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/ops",
		func(c *gin.Context) { c.Set(middleware.ContextRole, string(models.RoleOperator)); c.Next() },
		middleware.RBAC(models.RoleAdmin, models.RoleOperator),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ops", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCurrentUserIDMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	if _, ok := middleware.CurrentUserID(c); ok {
		t.Error("CurrentUserID() ok = true, want false when unset")
	}
}

func TestCurrentUserIDInvalidString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(middleware.ContextUserID, "not-a-uuid")

	if _, ok := middleware.CurrentUserID(c); ok {
		t.Error("CurrentUserID() ok = true, want false for invalid uuid")
	}
}

// sseEngine mounts a route guarded by AuthRequiredSSE, which reads the token
// from the ?token= query parameter (with the Authorization header as fallback).
func sseEngine(tokens *auth.JWTManager, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/events", middleware.AuthRequiredSSE(tokens), handler)
	return engine
}

// doSSE issues a GET /events request with the given query token and header.
func doSSE(engine *gin.Engine, token, authorization string) *httptest.ResponseRecorder {
	path := "/events"
	if token != "" {
		path += "?token=" + token
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestAuthRequiredSSEAcceptsQueryTokenAndSetsContext(t *testing.T) {
	m := newTestManager()
	userID := uuid.New()
	access, _, err := m.GenerateTokens(userID, "sse@example.com", models.RoleOperator)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	var (
		gotID    uuid.UUID
		gotEmail string
		gotRole  string
		ok       bool
	)
	engine := sseEngine(m, func(c *gin.Context) {
		gotID, ok = middleware.CurrentUserID(c)
		gotEmail = c.GetString(middleware.ContextEmail)
		gotRole = c.GetString(middleware.ContextRole)
		c.Status(http.StatusOK)
	})

	rec := doSSE(engine, access, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !ok || gotID != userID {
		t.Errorf("CurrentUserID() = (%v, %v), want (%v, true)", gotID, ok, userID)
	}
	if gotEmail != "sse@example.com" {
		t.Errorf("email = %q, want %q", gotEmail, "sse@example.com")
	}
	if gotRole != string(models.RoleOperator) {
		t.Errorf("role = %q, want %q", gotRole, models.RoleOperator)
	}
}

func TestAuthRequiredSSERejectsRefreshToken(t *testing.T) {
	m := newTestManager()
	_, refresh, err := m.GenerateTokens(uuid.New(), "sse@example.com", models.RoleViewer)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	engine := sseEngine(m, func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := doSSE(engine, refresh, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthRequiredSSERejectsInvalidToken(t *testing.T) {
	engine := sseEngine(newTestManager(), func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := doSSE(engine, "not-a-token", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthRequiredSSEMissingToken(t *testing.T) {
	engine := sseEngine(newTestManager(), func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := doSSE(engine, "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthRequiredSSEFallsBackToAuthorizationHeader(t *testing.T) {
	m := newTestManager()
	access, _, err := m.GenerateTokens(uuid.New(), "sse@example.com", models.RoleViewer)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	engine := sseEngine(m, func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := doSSE(engine, "", "Bearer "+access)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthRequiredSSEPrefersQueryTokenOverHeader(t *testing.T) {
	m := newTestManager()
	valid, _, err := m.GenerateTokens(uuid.New(), "sse@example.com", models.RoleViewer)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	engine := sseEngine(m, func(c *gin.Context) { c.Status(http.StatusOK) })

	// A valid query token wins even when the header carries garbage.
	rec := doSSE(engine, valid, "Bearer not-a-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthRequiredSSERejectsInvalidQueryTokenEvenWithValidHeader(t *testing.T) {
	m := newTestManager()
	valid, _, err := m.GenerateTokens(uuid.New(), "sse@example.com", models.RoleViewer)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	engine := sseEngine(m, func(c *gin.Context) { c.Status(http.StatusOK) })

	// An explicitly supplied but invalid query token is not silently overridden
	// by the header: it is rejected.
	rec := doSSE(engine, "not-a-token", "Bearer "+valid)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthRequiredSSENilManager(t *testing.T) {
	engine := sseEngine(nil, func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := doSSE(engine, "whatever", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
