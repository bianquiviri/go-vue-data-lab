// Package router wires the HTTP layer: it builds the Gin engine, attaches
// middleware and registers the API routes.
package router

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"

	"github.com/josebianco/go-vue-data-lab/internal/auth"
	"github.com/josebianco/go-vue-data-lab/internal/handler"
	"github.com/josebianco/go-vue-data-lab/internal/middleware"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// Dependencies holds the collaborators required to build the router. The
// only mandatory fields are Logger and Version; everything else is optional so
// the shape stays backward compatible.
type Dependencies struct {
	Logger  *zap.Logger
	Version string

	// AuthService and TokenManager are optional. When both are provided the
	// /api/v1/auth routes are registered; otherwise they are omitted (useful
	// for tests and for bootstrapping the server without a database).
	AuthService  service.AuthService
	TokenManager *auth.JWTManager

	// CORS configures the cross-origin middleware. The zero value uses safe
	// local development defaults.
	CORS middleware.CORSConfig

	// Data is the lab_source read port. When nil, the /api/v1/source group
	// is not registered (useful for tests and for deployments without a
	// data service).
	Data service.SourceData

	// Target is the lab_target read port. When nil, the /api/v1/target
	// group is not registered.
	Target service.TargetData

	// Jobs is the job control port. When nil, the /api/v1/jobs group is not
	// registered.
	Jobs service.JobControl

	// Scheduler is the cron schedule control port. When nil, the
	// /api/v1/scheduler group is not registered.
	Scheduler service.SchedulerControl

	// Middleware is an optional hook applied to the /api/v1/source,
	// /api/v1/target, /api/v1/jobs and /api/v1/scheduler groups only. It
	// exists so auth/RBAC can be attached later without importing the auth
	// package here; pass nil to leave the groups open.
	Middleware []gin.HandlerFunc
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
	engine.Use(middleware.CORS(deps.CORS))

	// Consistent 404 envelope for unknown routes.
	engine.NoRoute(func(c *gin.Context) {
		handler.RespondNotFound(c, "route not found")
	})

	health := handler.NewHealthHandler(deps.Version)

	v1 := engine.Group("/api/v1")
	{
		v1.GET("/health", health.Health)

		if deps.AuthService != nil && deps.TokenManager != nil {
			registerAuthRoutes(v1, deps.AuthService, deps.TokenManager)
		}
		registerSourceRoutes(v1, deps)
		registerTargetRoutes(v1, deps)
		registerJobRoutes(v1, deps)
		registerSchedulerRoutes(v1, deps)
	}

	// Swagger UI at /swagger/index.html (OpenAPI 3.x spec served from the
	// generated docs package imported by the entrypoint).
	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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

// registerSourceRoutes mounts the lab_source read endpoints when a data
// service is provided, applying the optional middleware hook (e.g. auth) to
// the group.
func registerSourceRoutes(v1 *gin.RouterGroup, deps Dependencies) {
	if deps.Data == nil {
		return
	}

	source := v1.Group("/source")
	if len(deps.Middleware) > 0 {
		source.Use(deps.Middleware...)
	}

	h := handler.NewSourceHandler(deps.Data)
	source.GET("/customers", h.ListCustomers)
	source.GET("/customers/:id", h.GetCustomer)
	source.GET("/products", h.ListProducts)
	source.GET("/products/:id", h.GetProduct)
	source.GET("/orders", h.ListOrders)
	source.GET("/orders/:id", h.GetOrder)
	source.GET("/order-items", h.ListOrderItems)
}

// registerTargetRoutes mounts the lab_target read endpoints (entity reads plus
// analytics) when a target data service is provided, applying the optional
// middleware hook (e.g. auth) to the group.
func registerTargetRoutes(v1 *gin.RouterGroup, deps Dependencies) {
	if deps.Target == nil {
		return
	}

	target := v1.Group("/target")
	if len(deps.Middleware) > 0 {
		target.Use(deps.Middleware...)
	}

	h := handler.NewTargetHandler(deps.Target)
	target.GET("/customers", h.ListCustomers)
	target.GET("/customers/:id", h.GetCustomer)
	target.GET("/products", h.ListProducts)
	target.GET("/products/:id", h.GetProduct)
	target.GET("/orders", h.ListOrders)
	target.GET("/orders/:id", h.GetOrder)

	analytics := target.Group("/analytics")
	analytics.GET("/summary", h.AnalyticsSummary)
	analytics.GET("/top-products", h.TopProducts)
	analytics.GET("/top-customers", h.TopCustomers)
}

// registerJobRoutes mounts the job control endpoints when a job control
// service is provided, applying the optional middleware hook (e.g. auth) to
// the group.
func registerJobRoutes(v1 *gin.RouterGroup, deps Dependencies) {
	if deps.Jobs == nil {
		return
	}

	jobsGroup := v1.Group("/jobs")
	if len(deps.Middleware) > 0 {
		jobsGroup.Use(deps.Middleware...)
	}

	h := handler.NewJobsHandler(deps.Jobs)
	jobsGroup.POST("/generate/start", h.StartGenerate)
	jobsGroup.POST("/migrate/start", h.StartMigrate)
	jobsGroup.POST("/stop", h.Stop)
	jobsGroup.GET("/status", h.Status)
	jobsGroup.GET("/history", h.History)
	jobsGroup.GET("/progress", h.Progress)
}

// registerSchedulerRoutes mounts the cron schedule endpoints when a scheduler
// service is provided, applying the optional middleware hook (e.g. auth) to
// the group.
func registerSchedulerRoutes(v1 *gin.RouterGroup, deps Dependencies) {
	if deps.Scheduler == nil {
		return
	}

	sched := v1.Group("/scheduler")
	if len(deps.Middleware) > 0 {
		sched.Use(deps.Middleware...)
	}

	h := handler.NewSchedulerHandler(deps.Scheduler)
	sched.GET("/jobs", h.List)
	sched.POST("/jobs", h.Create)
	sched.PUT("/jobs/:id", h.Update)
	sched.DELETE("/jobs/:id", h.Delete)
	sched.POST("/jobs/:id/enable", h.Enable)
	sched.POST("/jobs/:id/disable", h.Disable)
	sched.POST("/jobs/:id/run", h.Run)
}
