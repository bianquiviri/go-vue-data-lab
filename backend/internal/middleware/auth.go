package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/auth"
	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

// Gin context keys populated by AuthRequired and consumed by RBAC/handlers.
const (
	ContextUserID = "user_id"
	ContextEmail  = "email"
	ContextRole   = "role"
)

// bearerPrefix is the required Authorization header scheme.
const bearerPrefix = "Bearer "

// AuthRequired validates the bearer access token in the Authorization header.
// On success it stores user_id, email and role in the Gin context; otherwise it
// aborts with 401 Unauthorized.
func AuthRequired(tokens *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authenticate(c, tokens, extractBearer(c.GetHeader("Authorization")))
	}
}

// AuthRequiredSSE is the variant of AuthRequired used by Server-Sent Events
// endpoints. Browsers' native EventSource cannot set request headers, so the
// JWT is accepted from the "token" query parameter, with the standard
// Authorization bearer header kept as a fallback for non-browser clients.
//
// Security trade-off: putting a token in the URL means it can leak into access
// logs, proxy logs, browser history and Referer headers. That is acceptable
// here only because (a) the endpoint is read-only progress streaming and (b)
// only short-lived access tokens are accepted — a refresh token is rejected with
// 401, exactly like AuthRequired. Prefer the header whenever the client can set
// it; use ?token= only where EventSource forces it. When both are present the
// query parameter wins, so an invalid explicit token is never masked by a
// valid header.
func AuthRequiredSSE(tokens *auth.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.Query("token")
		if raw == "" {
			raw = extractBearer(c.GetHeader("Authorization"))
		} else {
			raw = extractBearer(raw)
		}
		authenticate(c, tokens, raw)
	}
}

// authenticate is the shared token-validation core behind AuthRequired and
// AuthRequiredSSE. It validates the access token and populates the Gin context,
// or aborts the request with 401 Unauthorized. A nil manager (auth not wired)
// always denies.
func authenticate(c *gin.Context, tokens *auth.JWTManager, raw string) {
	if tokens == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if raw == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization token"})
		return
	}

	claims, err := tokens.ValidateToken(raw)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}
	if claims.TokenType != auth.TokenTypeAccess {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token type"})
		return
	}

	c.Set(ContextUserID, claims.UserID.String())
	c.Set(ContextEmail, claims.Email)
	c.Set(ContextRole, string(claims.Role))
	c.Next()
}

// RBAC restricts access to the given roles. It must be attached after
// AuthRequired (it reads the role from the Gin context). Denied requests are
// rejected with 403 Forbidden; a missing role is treated as 401 Unauthorized.
func RBAC(roles ...models.UserRole) gin.HandlerFunc {
	allowed := make(map[models.UserRole]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}

	return func(c *gin.Context) {
		value, ok := c.Get(ContextRole)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		role, ok := value.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		if _, ok := allowed[models.UserRole(role)]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		c.Next()
	}
}

// CurrentUserID extracts the authenticated user's id from the Gin context.
func CurrentUserID(c *gin.Context) (uuid.UUID, bool) {
	value, ok := c.Get(ContextUserID)
	if !ok {
		return uuid.Nil, false
	}

	switch v := value.(type) {
	case uuid.UUID:
		return v, true
	case string:
		id, err := uuid.Parse(v)
		if err != nil {
			return uuid.Nil, false
		}
		return id, true
	default:
		return uuid.Nil, false
	}
}

// extractBearer returns the token portion of an Authorization header value,
// accepting both "Bearer <token>" and a bare token for convenience.
func extractBearer(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	if strings.HasPrefix(header, bearerPrefix) {
		return strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))
	}
	// Be lenient with scheme casing (e.g. "bearer").
	if len(header) > len(bearerPrefix) && strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
		return strings.TrimSpace(header[len(bearerPrefix):])
	}
	return header
}
