// Command api is the HTTP entrypoint for the Go Concurrent Data Processing Lab.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	_ "github.com/josebianco/go-vue-data-lab/docs/swagger" // generated OpenAPI docs
	"github.com/josebianco/go-vue-data-lab/internal/config"
	"github.com/josebianco/go-vue-data-lab/internal/etl"
	etlpostgres "github.com/josebianco/go-vue-data-lab/internal/etl/postgres"
	"github.com/josebianco/go-vue-data-lab/internal/generator"
	generatorpostgres "github.com/josebianco/go-vue-data-lab/internal/generator/postgres"
	"github.com/josebianco/go-vue-data-lab/internal/infrastructure/database"
	"github.com/josebianco/go-vue-data-lab/internal/infrastructure/logger"
	"github.com/josebianco/go-vue-data-lab/internal/jobs"
	"github.com/josebianco/go-vue-data-lab/internal/middleware"
	"github.com/josebianco/go-vue-data-lab/internal/repository"
	"github.com/josebianco/go-vue-data-lab/internal/router"
	"github.com/josebianco/go-vue-data-lab/internal/scheduler"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// version is overridable at build time:
//
//	go build -ldflags "-X main.version=1.2.3" ./cmd/api
var version = "dev"

// shutdownTimeout bounds the graceful shutdown window.
const shutdownTimeout = 15 * time.Second

// @title			Go Concurrent Data Processing Lab API
// @version		1.0
// @description	REST API for the Go Concurrent Data Processing Lab, a concurrent data processing and synthetic e-commerce analytics platform.
// @BasePath		/api/v1
// @schemes		http https
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("main: load config: %w", err)
	}

	log, err := logger.New(cfg.LogLevel, cfg.LogPath)
	if err != nil {
		return fmt.Errorf("main: init logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	log.Info("starting api",
		zap.String("env", cfg.AppEnv),
		zap.String("version", version),
	)

	sourceDB, err := database.OpenSource(cfg)
	if err != nil {
		return fmt.Errorf("main: open source database: %w", err)
	}
	defer func() { _ = database.Close(sourceDB) }()

	sourceRepos := repository.NewSourceRepositories(sourceDB)

	targetDB, err := database.OpenTarget(cfg)
	if err != nil {
		return fmt.Errorf("main: open target database: %w", err)
	}
	defer func() { _ = database.Close(targetDB) }()

	targetRepos := repository.NewTargetRepositories(targetDB)

	jobManager, err := buildJobManager(log, sourceDB, targetDB)
	if err != nil {
		return fmt.Errorf("main: build job manager: %w", err)
	}
	defer func() { _ = jobManager.Stop() }()

	sched, err := buildScheduler(log, targetDB, jobManager.Start)
	if err != nil {
		return fmt.Errorf("main: build scheduler: %w", err)
	}
	if err := sched.Start(context.Background()); err != nil {
		return fmt.Errorf("main: start scheduler: %w", err)
	}
	defer sched.Stop()

	engine := router.New(router.Dependencies{
		Logger:  log,
		Version: version,
		CORS:    middleware.CORSConfig{AllowedOrigins: cfg.CORSAllowedOrigins},
		Data:    service.NewDataService(sourceRepos.Customers, sourceRepos.Products, sourceRepos.Orders, sourceRepos.OrderItems),
		Target: service.NewTargetDataService(
			service.NewDataService(targetRepos.Customers, targetRepos.Products, targetRepos.Orders, targetRepos.OrderItems),
			repository.NewAnalyticsRepository(targetDB),
		),
		Jobs:      jobManager,
		Scheduler: sched,
	})

	addr := fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort)
	srv := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("main: http server: %w", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("main: graceful shutdown: %w", err)
	}

	log.Info("server stopped cleanly")
	return nil
}

// buildJobManager wires the job manager with the generate and migrate runners.
func buildJobManager(log *zap.Logger, sourceDB, targetDB *gorm.DB) (*jobs.Manager, error) {
	manager, err := jobs.New(repository.NewJobExecutionRepository(targetDB), jobs.WithLogger(log))
	if err != nil {
		return nil, err
	}

	store, err := generatorpostgres.NewPostgresStore(sourceDB, generator.DefaultBatchSize)
	if err != nil {
		return nil, fmt.Errorf("build generator store: %w", err)
	}
	if err := manager.Register(jobs.TypeGenerate, jobs.NewGenerateRunner(store)); err != nil {
		return nil, fmt.Errorf("register generate runner: %w", err)
	}

	src, err := etlpostgres.NewSource(sourceDB)
	if err != nil {
		return nil, fmt.Errorf("build etl source: %w", err)
	}
	loader, err := etlpostgres.NewLoader(targetDB)
	if err != nil {
		return nil, fmt.Errorf("build etl loader: %w", err)
	}
	if err := manager.Register(jobs.TypeMigrate, jobs.NewMigrateRunner(src, loader, etl.WithLogger(log))); err != nil {
		return nil, fmt.Errorf("register migrate runner: %w", err)
	}

	return manager, nil
}

// buildScheduler wires the cron scheduler over the lab_target schedules table.
func buildScheduler(log *zap.Logger, targetDB *gorm.DB, starter scheduler.JobStarter) (*scheduler.Scheduler, error) {
	return scheduler.New(
		repository.NewCronScheduleRepository(targetDB),
		starter,
		scheduler.WithLogger(log),
		scheduler.WithValidCommands(jobs.TypeGenerate, jobs.TypeMigrate),
	)
}
