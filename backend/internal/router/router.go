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
	"github.com/josebianco/go-vue-data-lab/internal/secureid"
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

	// Admin is the error-log read port. It is only wired when AdminMiddleware
	// is also provided: the /api/v1/admin group is registered exclusively
	// guarded, so error logs can never be exposed by default.
	Admin service.ErrorLogs

	// AdminUsers is the user management port of the admin panel. When nil the
	// /api/v1/admin/users routes are not registered (same fail-closed rule as
	// Admin: the group also requires AdminMiddleware).
	AdminUsers service.AdminUsers

	// AdminStats is the dashboard counters port of the admin panel. When nil
	// the /api/v1/admin/stats route is not registered.
	AdminStats service.AdminStats

	// Middleware is an optional hook applied to the /api/v1/source,
	// /api/v1/target, /api/v1/jobs and /api/v1/scheduler groups only. It
	// exists so auth/RBAC can be attached later without importing the auth
	// package here; pass nil to leave the groups open.
	Middleware []gin.HandlerFunc

	// WriteMiddleware is an optional hook applied on top of Middleware to the
	// mutating routes (POST/PUT/DELETE) of the source, jobs and scheduler
	// groups. It is where write-level RBAC (e.g. admin/operator) is attached.
	WriteMiddleware []gin.HandlerFunc

	// AdminMiddleware guards the /api/v1/admin group (auth + admin-only RBAC).
	// When empty the group is not registered at all.
	AdminMiddleware []gin.HandlerFunc

	// SSEMiddleware is an optional hook applied only to the Server-Sent Events
	// route GET /api/v1/jobs/events. It exists because EventSource cannot set
	// the Authorization header, so the SSE endpoint needs a token-aware guard
	// that reads the token from the query string (see middleware.AuthRequiredSSE)
	// instead of the group's bearer guard. The SSE route is registered outside
	// the /jobs group chain, so this hook replaces — and never stacks on top
	// of — Middleware for that route.
	SSEMiddleware []gin.HandlerFunc

	// Codec is the opaque id codec. When provided, every response seals its
	// id / *_id values into opaque tokens and every inbound id (path, query
	// or body) must be an opaque token; raw UUIDs are rejected with a 400.
	// When nil the API keeps the legacy raw-UUID contract (useful for tests
	// and for bootstrapping the server without a key).
	Codec *secureid.Codec
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
	// Opaque id codec: installed before any route so responders seal outbound
	// ids and the id parsers accept only opaque tokens.
	engine.Use(handler.WithCodec(deps.Codec))

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
		registerAdminRoutes(v1, deps)
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

	// Mutating routes live in their own subgroup so an optional write-level
	// guard (e.g. RBAC) can be attached without affecting the read routes.
	writes := source.Group("")
	if len(deps.WriteMiddleware) > 0 {
		writes.Use(deps.WriteMiddleware...)
	}
	writes.POST("/customers", h.CreateCustomer)
	writes.PUT("/customers/:id", h.UpdateCustomer)
	writes.DELETE("/customers/:id", h.DeleteCustomer)
	writes.POST("/products", h.CreateProduct)
	writes.PUT("/products/:id", h.UpdateProduct)
	writes.DELETE("/products/:id", h.DeleteProduct)
	writes.POST("/orders", h.CreateOrder)
	writes.PUT("/orders/:id", h.UpdateOrder)
	writes.DELETE("/orders/:id", h.DeleteOrder)
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
	control := jobsGroup.Group("")
	if len(deps.WriteMiddleware) > 0 {
		control.Use(deps.WriteMiddleware...)
	}
	control.POST("/generate/start", h.StartGenerate)
	control.POST("/migrate/start", h.StartMigrate)
	control.POST("/stop", h.Stop)

	jobsGroup.GET("/status", h.Status)
	jobsGroup.GET("/history", h.History)
	jobsGroup.GET("/progress", h.Progress)

	registerJobEventsRoute(v1, deps, h)
}

// registerJobEventsRoute mounts GET /api/v1/jobs/events on a standalone group
// under /api/v1, outside the /jobs group middleware chain.
//
// The route must not inherit the group's bearer guard: a browser EventSource
// cannot send an Authorization header, so the stream is authenticated with a
// dedicated query-parameter guard (middleware.AuthRequiredSSE). Inheriting the
// group middleware would abort every EventSource request with 401 before the
// SSE guard ever runs.
//
// The route is fail-closed: it is registered only when a guard is available.
// SSEMiddleware is preferred (the query-token guard used in production);
// Middleware is the bearer-only fallback for non-browser clients. Without any
// guard the endpoint is not registered at all, exactly like the admin group.
func registerJobEventsRoute(v1 *gin.RouterGroup, deps Dependencies, h *handler.JobsHandler) {
	guard := deps.SSEMiddleware
	if len(guard) == 0 {
		guard = deps.Middleware
	}
	if len(guard) == 0 {
		return
	}

	events := v1.Group("/jobs/events")
	events.Use(guard...)
	events.GET("", h.Events)
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

	control := sched.Group("")
	if len(deps.WriteMiddleware) > 0 {
		control.Use(deps.WriteMiddleware...)
	}
	control.POST("/jobs", h.Create)
	control.PUT("/jobs/:id", h.Update)
	control.DELETE("/jobs/:id", h.Delete)
	control.POST("/jobs/:id/enable", h.Enable)
	control.POST("/jobs/:id/disable", h.Disable)
	control.POST("/jobs/:id/run", h.Run)
}

// registerAdminRoutes mounts the admin-only operational endpoints. It is a
// no-op unless an admin guard is configured, and every port is registered
// individually so a missing collaborator can never expose an open route: the
// error logs, user management and dashboard endpoints are all fail-closed.
func registerAdminRoutes(v1 *gin.RouterGroup, deps Dependencies) {
	if len(deps.AdminMiddleware) == 0 {
		return
	}

	admin := v1.Group("/admin")
	admin.Use(deps.AdminMiddleware...)

	if deps.Admin != nil {
		h := handler.NewAdminHandler(deps.Admin)
		admin.GET("/logs/errors", h.Errors)
	}

	if deps.AdminUsers != nil {
		h := handler.NewAdminUsersHandler(deps.AdminUsers)
		admin.GET("/users", h.ListUsers)
		admin.GET("/users/:id", h.GetUser)
		admin.POST("/users", h.CreateUser)
		admin.PUT("/users/:id", h.UpdateUser)
		admin.DELETE("/users/:id", h.DeleteUser)
	}

	if deps.AdminStats != nil {
		h := handler.NewAdminStatsHandler(deps.AdminStats)
		admin.GET("/stats", h.Stats)
	}
}
