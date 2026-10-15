package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/handler"
	"github.com/josebianco/go-vue-data-lab/internal/middleware"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// --- Fake service -----------------------------------------------------------

type fakeAuthService struct {
	loginResult   *service.LoginResult
	loginErr      error
	refreshResult *service.LoginResult
	refreshErr    error
	meUser        *models.User
	meErr         error
	changeErr     error
	logoutErr     error

	changeUserID uuid.UUID
	changeOld    string
	changeNew    string
}

func (f *fakeAuthService) Login(_ context.Context, _, _ string) (*service.LoginResult, error) {
	return f.loginResult, f.loginErr
}

func (f *fakeAuthService) Refresh(_ context.Context, _ string) (*service.LoginResult, error) {
	return f.refreshResult, f.refreshErr
}

func (f *fakeAuthService) Me(_ context.Context, _ uuid.UUID) (*models.User, error) {
	return f.meUser, f.meErr
}

func (f *fakeAuthService) ChangePassword(_ context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	f.changeUserID = userID
	f.changeOld = oldPassword
	f.changeNew = newPassword
	return f.changeErr
}

func (f *fakeAuthService) Logout(_ context.Context, _ uuid.UUID) error {
	return f.logoutErr
}

// --- Helpers ----------------------------------------------------------------

func sampleResult() *service.LoginResult {
	return &service.LoginResult{
		User: &models.User{
			ID:        uuid.New(),
			Email:     "user@example.com",
			Role:      models.RoleAdmin,
			Active:    true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Tokens: service.TokenPair{
			AccessToken:      "access-token",
			RefreshToken:     "refresh-token",
			AccessExpiresIn:  15 * time.Minute,
			RefreshExpiresIn: time.Hour,
		},
	}
}

func newAuthEngine(svc service.AuthService, userID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	h := handler.NewAuthHandler(svc)

	engine.POST("/api/v1/auth/login", h.Login)
	engine.POST("/api/v1/auth/refresh", h.Refresh)

	protected := engine.Group("/api/v1/auth")
	protected.Use(func(c *gin.Context) {
		if userID != uuid.Nil {
			c.Set(middleware.ContextUserID, userID.String())
		}
		c.Next()
	})
	protected.GET("/me", h.Me)
	protected.POST("/change-password", h.ChangePassword)
	protected.POST("/logout", h.Logout)

	return engine
}

func performJSON(engine *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// --- Login ------------------------------------------------------------------

func TestLoginHandlerSuccess(t *testing.T) {
	svc := &fakeAuthService{loginResult: sampleResult()}
	engine := newAuthEngine(svc, uuid.Nil)

	rec := performJSON(engine, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    "user@example.com",
		"password": "secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body handler.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Tokens.AccessToken != "access-token" {
		t.Errorf("access token = %q, want %q", body.Tokens.AccessToken, "access-token")
	}
	if body.Tokens.ExpiresIn != 900 {
		t.Errorf("expires_in = %d, want 900", body.Tokens.ExpiresIn)
	}
	if body.User.Email != "user@example.com" {
		t.Errorf("email = %q, want %q", body.User.Email, "user@example.com")
	}
}

func TestLoginHandlerBadRequest(t *testing.T) {
	engine := newAuthEngine(&fakeAuthService{}, uuid.Nil)

	rec := performJSON(engine, http.MethodPost, "/api/v1/auth/login", map[string]string{"email": "not-an-email"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLoginHandlerInvalidCredentials(t *testing.T) {
	svc := &fakeAuthService{loginErr: service.ErrInvalidCredentials}
	engine := newAuthEngine(svc, uuid.Nil)

	rec := performJSON(engine, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    "user@example.com",
		"password": "wrong",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLoginHandlerInactiveUser(t *testing.T) {
	svc := &fakeAuthService{loginErr: service.ErrUserInactive}
	engine := newAuthEngine(svc, uuid.Nil)

	rec := performJSON(engine, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    "user@example.com",
		"password": "secret",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLoginHandlerInternalError(t *testing.T) {
	svc := &fakeAuthService{loginErr: errors.New("boom")}
	engine := newAuthEngine(svc, uuid.Nil)

	rec := performJSON(engine, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    "user@example.com",
		"password": "secret",
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- Refresh ----------------------------------------------------------------

func TestRefreshHandlerSuccess(t *testing.T) {
	svc := &fakeAuthService{refreshResult: sampleResult()}
	engine := newAuthEngine(svc, uuid.Nil)

	rec := performJSON(engine, http.MethodPost, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": "refresh-token",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestRefreshHandlerInvalidToken(t *testing.T) {
	svc := &fakeAuthService{refreshErr: service.ErrInvalidToken}
	engine := newAuthEngine(svc, uuid.Nil)

	rec := performJSON(engine, http.MethodPost, "/api/v1/auth/refresh", map[string]string{
		"refresh_token": "bad",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRefreshHandlerBadRequest(t *testing.T) {
	engine := newAuthEngine(&fakeAuthService{}, uuid.Nil)

	rec := performJSON(engine, http.MethodPost, "/api/v1/auth/refresh", map[string]string{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// --- Me ---------------------------------------------------------------------

func TestMeHandlerSuccess(t *testing.T) {
	userID := uuid.New()
	svc := &fakeAuthService{meUser: &models.User{ID: userID, Email: "user@example.com", Role: models.RoleViewer, Active: true}}
	engine := newAuthEngine(svc, userID)

	rec := performJSON(engine, http.MethodGet, "/api/v1/auth/me", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		User handler.UserResponse `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.User.ID != userID {
		t.Errorf("user id = %v, want %v", body.User.ID, userID)
	}
}

func TestMeHandlerUnauthorizedWithoutContext(t *testing.T) {
	engine := newAuthEngine(&fakeAuthService{}, uuid.Nil)

	rec := performJSON(engine, http.MethodGet, "/api/v1/auth/me", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMeHandlerNotFound(t *testing.T) {
	svc := &fakeAuthService{meErr: service.ErrUserNotFound}
	engine := newAuthEngine(svc, uuid.New())

	rec := performJSON(engine, http.MethodGet, "/api/v1/auth/me", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// --- ChangePassword ---------------------------------------------------------

func TestChangePasswordHandlerSuccess(t *testing.T) {
	userID := uuid.New()
	svc := &fakeAuthService{}
	engine := newAuthEngine(svc, userID)

	rec := performJSON(engine, http.MethodPost, "/api/v1/auth/change-password", map[string]string{
		"old_password": "old-password",
		"new_password": "new-password",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
	if svc.changeUserID != userID {
		t.Errorf("service user id = %v, want %v", svc.changeUserID, userID)
	}
	if svc.changeOld != "old-password" || svc.changeNew != "new-password" {
		t.Errorf("service received old=%q new=%q", svc.changeOld, svc.changeNew)
	}
}

func TestChangePasswordHandlerWeakPassword(t *testing.T) {
	svc := &fakeAuthService{changeErr: service.ErrWeakPassword}
	engine := newAuthEngine(svc, uuid.New())

	rec := performJSON(engine, http.MethodPost, "/api/v1/auth/change-password", map[string]string{
		"old_password": "old-password",
		"new_password": "new-password",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestChangePasswordHandlerWrongOldPassword(t *testing.T) {
	svc := &fakeAuthService{changeErr: service.ErrInvalidCredentials}
	engine := newAuthEngine(svc, uuid.New())

	rec := performJSON(engine, http.MethodPost, "/api/v1/auth/change-password", map[string]string{
		"old_password": "wrong",
		"new_password": "new-password",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestChangePasswordHandlerBadRequest(t *testing.T) {
	engine := newAuthEngine(&fakeAuthService{}, uuid.New())

	rec := performJSON(engine, http.MethodPost, "/api/v1/auth/change-password", map[string]string{
		"old_password": "old-password",
		"new_password": "short",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestChangePasswordHandlerUnauthorizedWithoutContext(t *testing.T) {
	engine := newAuthEngine(&fakeAuthService{}, uuid.Nil)

	rec := performJSON(engine, http.MethodPost, "/api/v1/auth/change-password", map[string]string{
		"old_password": "old-password",
		"new_password": "new-password",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// --- Logout -----------------------------------------------------------------

func TestLogoutHandlerSuccess(t *testing.T) {
	engine := newAuthEngine(&fakeAuthService{}, uuid.New())

	rec := performJSON(engine, http.MethodPost, "/api/v1/auth/logout", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestLogoutHandlerUnauthorizedWithoutContext(t *testing.T) {
	engine := newAuthEngine(&fakeAuthService{}, uuid.Nil)

	rec := performJSON(engine, http.MethodPost, "/api/v1/auth/logout", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
