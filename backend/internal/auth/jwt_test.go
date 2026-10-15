package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

const testSecret = "test-secret-do-not-use-in-production"

func newTestManager() *JWTManager {
	return NewJWTManager(testSecret, 15*time.Minute, 7*24*time.Hour)
}

func TestGenerateTokensProducesDistinctTokens(t *testing.T) {
	m := newTestManager()
	userID := uuid.New()

	access, refresh, err := m.GenerateTokens(userID, "user@example.com", models.RoleAdmin)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}
	if access == "" || refresh == "" {
		t.Fatal("GenerateTokens() returned an empty token")
	}
	if access == refresh {
		t.Error("access and refresh tokens are identical")
	}
}

func TestValidateTokenClaims(t *testing.T) {
	m := newTestManager()
	userID := uuid.New()

	access, _, err := m.GenerateTokens(userID, "user@example.com", models.RoleOperator)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	claims, err := m.ValidateToken(access)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("UserID = %v, want %v", claims.UserID, userID)
	}
	if claims.Email != "user@example.com" {
		t.Errorf("Email = %q, want %q", claims.Email, "user@example.com")
	}
	if claims.Role != models.RoleOperator {
		t.Errorf("Role = %q, want %q", claims.Role, models.RoleOperator)
	}
	if claims.TokenType != TokenTypeAccess {
		t.Errorf("TokenType = %q, want %q", claims.TokenType, TokenTypeAccess)
	}
	if claims.ID == "" {
		t.Error("jti (ID) is empty")
	}
	if claims.IssuedAt == nil || claims.ExpiresAt == nil {
		t.Fatal("iat/exp claims are not set")
	}
	if !claims.ExpiresAt.After(claims.IssuedAt.Time) {
		t.Error("exp is not after iat")
	}
	if claims.Subject != userID.String() {
		t.Errorf("Subject = %q, want %q", claims.Subject, userID.String())
	}
}

func TestValidateRefreshTokenType(t *testing.T) {
	m := newTestManager()

	_, refresh, err := m.GenerateTokens(uuid.New(), "user@example.com", models.RoleViewer)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	claims, err := m.ValidateToken(refresh)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if claims.TokenType != TokenTypeRefresh {
		t.Errorf("TokenType = %q, want %q", claims.TokenType, TokenTypeRefresh)
	}
}

func TestValidateExpiredToken(t *testing.T) {
	m := newTestManager()
	base := time.Now()
	m.now = func() time.Time { return base }

	access, _, err := m.GenerateTokens(uuid.New(), "user@example.com", models.RoleViewer)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	// Move the clock past the access-token TTL.
	m.now = func() time.Time { return base.Add(16 * time.Minute) }

	_, err = m.ValidateToken(access)
	if err == nil {
		t.Fatal("ValidateToken() error = nil, want expired error")
	}
	if !errors.Is(err, ErrExpiredToken) {
		t.Errorf("error = %v, want ErrExpiredToken", err)
	}
	if !errors.Is(err, jwt.ErrTokenExpired) {
		t.Errorf("error = %v, want to wrap jwt.ErrTokenExpired", err)
	}
}

func TestValidateTokenWrongSecret(t *testing.T) {
	issuer := NewJWTManager("secret-one", time.Minute, time.Hour)
	verifier := NewJWTManager("secret-two", time.Minute, time.Hour)

	access, _, err := issuer.GenerateTokens(uuid.New(), "user@example.com", models.RoleViewer)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	if _, err := verifier.ValidateToken(access); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("error = %v, want ErrInvalidToken", err)
	}
}

func TestValidateTokenTampered(t *testing.T) {
	m := newTestManager()

	access, _, err := m.GenerateTokens(uuid.New(), "user@example.com", models.RoleViewer)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	tampered := access[:len(access)-2] + "xx"
	if _, err := m.ValidateToken(tampered); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("error = %v, want ErrInvalidToken", err)
	}
}

func TestValidateTokenGarbage(t *testing.T) {
	m := newTestManager()

	if _, err := m.ValidateToken("not-a-jwt"); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("error = %v, want ErrInvalidToken", err)
	}
}

func TestValidateTokenRejectsUnexpectedAlgorithm(t *testing.T) {
	m := newTestManager()

	claims := Claims{
		UserID:    uuid.New(),
		Email:     "user@example.com",
		Role:      models.RoleViewer,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}

	if _, err := m.ValidateToken(signed); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("error = %v, want ErrInvalidToken for HS512 token", err)
	}
}

func TestNewJWTManagerDefaults(t *testing.T) {
	m := NewJWTManager("secret", 0, -time.Second)

	if m.AccessTTL() != 15*time.Minute {
		t.Errorf("AccessTTL() = %v, want 15m", m.AccessTTL())
	}
	if m.RefreshTTL() != 7*24*time.Hour {
		t.Errorf("RefreshTTL() = %v, want 168h", m.RefreshTTL())
	}
}

func TestNewJWTManagerCustomTTLs(t *testing.T) {
	m := NewJWTManager("secret", 5*time.Minute, 48*time.Hour)

	if m.AccessTTL() != 5*time.Minute {
		t.Errorf("AccessTTL() = %v, want 5m", m.AccessTTL())
	}
	if m.RefreshTTL() != 48*time.Hour {
		t.Errorf("RefreshTTL() = %v, want 48h", m.RefreshTTL())
	}
}
