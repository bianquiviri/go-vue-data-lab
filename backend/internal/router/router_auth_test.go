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
	"github.com/josebianco/go-vue-data-lab/internal/router"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

type stubAuthService struct {
	user *models.User
}

func (s *stubAuthService) Login(_ context.Context, _, _ string) (*service.LoginResult, error) {
	return &service.LoginResult{
		User: s.user,
		Tokens: service.TokenPair{
			AccessToken:      "access",
			RefreshToken:     "refresh",
			AccessExpiresIn:  15 * time.Minute,
			RefreshExpiresIn: time.Hour,
		},
	}, nil
}

func (s *stubAuthService) Refresh(_ context.Context, _ string) (*service.LoginResult, error) {
	return s.Login(context.Background(), "", "")
}

func (s *stubAuthService) Me(_ context.Context, _ uuid.UUID) (*models.User, error) {
	return s.user, nil
}

func (s *stubAuthService) ChangePassword(_ context.Context, _ uuid.UUID, _, _ string) error {
	return nil
}

func (s *stubAuthService) Logout(_ context.Context, _ uuid.UUID) error { return nil }

func TestRouterAuthRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)

	user := &models.User{ID: uuid.New(), Email: "root@gmail.com", Role: models.RoleAdmin, Active: true}
	tokens := auth.NewJWTManager("router-test-secret", 15*time.Minute, time.Hour)

	engine := router.New(router.Dependencies{
		Logger:       zap.NewNop(),
		Version:      "test",
		AuthService:  &stubAuthService{user: user},
		TokenManager: tokens,
	})

	body := strings.NewReader(`{"email":"root@gmail.com","password":"secret"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Tokens struct {
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Tokens.AccessToken != "access" {
		t.Errorf("access token = %q, want %q", resp.Tokens.AccessToken, "access")
	}
}

func TestRouterAuthMeRequiresToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tokens := auth.NewJWTManager("router-test-secret", 15*time.Minute, time.Hour)
	engine := router.New(router.Dependencies{
		Logger:       zap.NewNop(),
		AuthService:  &stubAuthService{user: &models.User{ID: uuid.New(), Email: "root@gmail.com", Role: models.RoleAdmin, Active: true}},
		TokenManager: tokens,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRouterAuthMeWithValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	tokens := auth.NewJWTManager("router-test-secret", 15*time.Minute, time.Hour)
	access, _, err := tokens.GenerateTokens(userID, "root@gmail.com", models.RoleAdmin)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	engine := router.New(router.Dependencies{
		Logger:       zap.NewNop(),
		AuthService:  &stubAuthService{user: &models.User{ID: userID, Email: "root@gmail.com", Role: models.RoleAdmin, Active: true}},
		TokenManager: tokens,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestRouterAuthRoutesAbsentWithoutDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := router.New(router.Dependencies{Logger: zap.NewNop()})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d when auth deps are absent", rec.Code, http.StatusNotFound)
	}
}
