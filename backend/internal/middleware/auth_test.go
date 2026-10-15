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
