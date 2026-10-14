// Package router wires the HTTP layer: it builds the Gin engine, attaches
// middleware and registers the API routes.
package router

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/josebianco/go-vue-data-lab/internal/handler"
	"github.com/josebianco/go-vue-data-lab/internal/middleware"
)

// Dependencies holds the collaborators required to build the router.
type Dependencies struct {
	Logger  *zap.Logger
	Version string
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
		// Future route groups (auth, jobs, data, admin) are registered here.
	}

	return engine
}
