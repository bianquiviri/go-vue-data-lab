// Package middleware contains reusable Gin middleware (request logging,
// panic recovery, CORS, and future auth/RBAC guards).
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// DefaultCORSAllowedOrigins is used when no origins are configured: the local
// HTTPS app URL plus the Vite dev-server origins.
var DefaultCORSAllowedOrigins = []string{
	"https://frontgodata.josebianco.local",
	"https://apidatago.josebianco.local",
	"http://localhost:5173",
	"http://127.0.0.1:5173",
}

// corsMaxAge is the default preflight cache duration (12 hours, seconds).
const corsMaxAge = "43200"

var (
	defaultCORSMethods = []string{
		http.MethodGet, http.MethodHead, http.MethodPost,
		http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions,
	}
	defaultCORSHeaders = []string{"Origin", "Accept", "Content-Type", "Authorization"}
)

// CORSConfig configures the CORS middleware. The zero value uses safe local
// development defaults.
type CORSConfig struct {
	// AllowedOrigins is the exact list of origins allowed to cross the
	// origin boundary. The special value "*" allows any origin. When empty,
	// DefaultCORSAllowedOrigins is used.
	AllowedOrigins []string
	// AllowedMethods defaults to the common REST methods.
	AllowedMethods []string
	// AllowedHeaders defaults to Origin, Accept, Content-Type, Authorization.
	AllowedHeaders []string
	// AllowCredentials enables Access-Control-Allow-Credentials (never
	// combined with the "*" origin).
	AllowCredentials bool
	// MaxAge is the preflight cache duration as a seconds string; empty
	// falls back to 12 hours.
	MaxAge string
}

// CORS returns a hand-rolled CORS middleware (no third-party dependency).
//
// It sets the CORS response headers on every request whose Origin is allowed
// and answers preflight OPTIONS requests (flagged by an
// Access-Control-Request-Method header) with 204. Preflights from disallowed
// origins get 204 without CORS headers, so the browser blocks them.
func CORS(cfg CORSConfig) gin.HandlerFunc {
	origins := cfg.AllowedOrigins
	if len(origins) == 0 {
		origins = DefaultCORSAllowedOrigins
	}
	methods := cfg.AllowedMethods
	if len(methods) == 0 {
		methods = defaultCORSMethods
	}
	headers := cfg.AllowedHeaders
	if len(headers) == 0 {
		headers = defaultCORSHeaders
	}
	maxAge := cfg.MaxAge
	if maxAge == "" {
		maxAge = corsMaxAge
	}

	allowed := make(map[string]bool, len(origins))
	allowAny := false
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o == "*" {
			allowAny = true
		}
		if o != "" {
			allowed[o] = true
		}
	}
	methodsHeader := strings.Join(methods, ", ")
	headersHeader := strings.Join(headers, ", ")

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		resolved := ""
		switch {
		case origin == "":
			// Non-CORS (same-origin or curl) request: nothing to do.
		case allowAny:
			resolved = "*"
		case allowed[origin]:
			resolved = origin
		}

		if resolved != "" {
			c.Header("Access-Control-Allow-Origin", resolved)
			c.Writer.Header().Add("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", methodsHeader)
			c.Header("Access-Control-Allow-Headers", headersHeader)
			c.Header("Access-Control-Max-Age", maxAge)
			if cfg.AllowCredentials && resolved != "*" {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
		}

		if c.Request.Method == http.MethodOptions &&
			c.GetHeader("Access-Control-Request-Method") != "" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
