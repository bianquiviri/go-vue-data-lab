// Package router wires the HTTP layer: it builds the Gin engine, attaches
// middleware and registers the API routes.
package router

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/josebianco/go-vue-data-lab/internal/auth"
	"github.com/josebianco/go-vue-data-lab/internal/handler"
	"github.com/josebianco/go-vue-data-lab/internal/middleware"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// Dependencies holds the collaborators required to build the router.
type Dependencies struct {
	Logger  *zap.Logger
	Version string

	// AuthService and TokenManager are optional. When both are provided the
	// /api/v1/auth routes are registered; otherwise they are omitted (useful
	// for tests and for bootstrapping the server without a database).
	AuthService  service.AuthService
	TokenManager *auth.JWTManager
}

// New builds the Gin engine with middleware and the v1 API routes registered.
func New(deps Dependencies) *gin.Engine {
	if deps.Logger == nil {
		deps.Logger = zap.NewNop()
	}
	if deps.Version == "" {
		deps.Version = "dev"
	}

	engine := gin.New()
	engine.Use(middleware.Recovery(deps.Logger))
	engine.Use(middleware.RequestLogger(deps.Logger))

	health := handler.NewHealthHandler(deps.Version)

	v1 := engine.Group("/api/v1")
	{
		v1.GET("/health", health.Health)

		if deps.AuthService != nil && deps.TokenManager != nil {
			registerAuthRoutes(v1, deps.AuthService, deps.TokenManager)
		}
		// Future route groups (jobs, data, admin) are registered here.
	}

	return engine
}

// registerAuthRoutes mounts the authentication endpoints under /auth.
func registerAuthRoutes(v1 *gin.RouterGroup, svc service.AuthService, tokens *auth.JWTManager) {
	authHandler := handler.NewAuthHandler(svc)
	authRoutes := v1.Group("/auth")
	{
		// Public endpoints.
		authRoutes.POST("/login", authHandler.Login)
		authRoutes.POST("/refresh", authHandler.Refresh)

		// Protected endpoints.
		protected := authRoutes.Group("")
		protected.Use(middleware.AuthRequired(tokens))
		{
			protected.GET("/me", authHandler.Me)
			protected.POST("/change-password", authHandler.ChangePassword)
			protected.POST("/logout", authHandler.Logout)
		}
	}
}
