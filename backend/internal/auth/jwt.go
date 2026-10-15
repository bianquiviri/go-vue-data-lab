package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// Token types carried in the token_type claim so an access token cannot be
// replayed as a refresh token (and vice versa).
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// defaultIssuer identifies tokens minted by this service.
const defaultIssuer = "go-vue-data-lab"

var (
	// ErrInvalidToken is returned when a token is malformed, tampered with or
	// signed with an unexpected algorithm/secret.
	ErrInvalidToken = errors.New("auth: invalid token")
	// ErrExpiredToken is returned when a token is well-formed but expired.
	ErrExpiredToken = errors.New("auth: token expired")
)

// Claims is the set of registered and custom claims embedded in every token.
type Claims struct {
	UserID    uuid.UUID       `json:"user_id"`
	Email     string          `json:"email"`
	Role      models.UserRole `json:"role"`
	TokenType string          `json:"token_type"`
	jwt.RegisteredClaims
}

// JWTManager mints and validates HMAC-SHA256 (HS256) JWTs. It is safe for
// concurrent use.
type JWTManager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
	now        func() time.Time
	newID      func() string
}

// NewJWTManager builds a JWTManager. Non-positive TTLs fall back to sensible
// defaults (15 minutes for access, 7 days for refresh).
func NewJWTManager(secret string, accessTTL, refreshTTL time.Duration) *JWTManager {
	if accessTTL <= 0 {
		accessTTL = 15 * time.Minute
	}
	if refreshTTL <= 0 {
		refreshTTL = 7 * 24 * time.Hour
	}
	return &JWTManager{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		issuer:     defaultIssuer,
		now:        time.Now,
		newID:      func() string { return uuid.NewString() },
	}
}

// AccessTTL returns the configured access-token lifetime.
func (m *JWTManager) AccessTTL() time.Duration { return m.accessTTL }

// RefreshTTL returns the configured refresh-token lifetime.
func (m *JWTManager) RefreshTTL() time.Duration { return m.refreshTTL }

// GenerateTokens mints a fresh access/refresh token pair for the given user.
func (m *JWTManager) GenerateTokens(userID uuid.UUID, email string, role models.UserRole) (accessToken, refreshToken string, err error) {
	accessToken, err = m.generate(TokenTypeAccess, userID, email, role, m.accessTTL)
	if err != nil {
		return "", "", fmt.Errorf("auth: generate access token: %w", err)
	}

	refreshToken, err = m.generate(TokenTypeRefresh, userID, email, role, m.refreshTTL)
	if err != nil {
		return "", "", fmt.Errorf("auth: generate refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// ValidateToken parses and validates token, returning its claims. Expired
// tokens wrap ErrExpiredToken (and jwt.ErrTokenExpired); any other failure
// wraps ErrInvalidToken.
func (m *JWTManager) ValidateToken(token string) (*Claims, error) {
	claims := &Claims{}

	parsed, err := jwt.ParseWithClaims(
		token,
		claims,
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("%w: unexpected signing method %q", ErrInvalidToken, t.Header["alg"])
			}
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: %w", ErrExpiredToken, err)
		}
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("%w: token marked invalid", ErrInvalidToken)
	}

	return claims, nil
}

// generate signs a single token of the given type and lifetime.
func (m *JWTManager) generate(tokenType string, userID uuid.UUID, email string, role models.UserRole, ttl time.Duration) (string, error) {
	now := m.now()
	claims := Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        m.newID(),
			Subject:   userID.String(),
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("auth: sign %s token: %w", tokenType, err)
	}
	return signed, nil
}
