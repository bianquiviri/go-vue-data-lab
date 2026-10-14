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

	"github.com/josebianco/go-vue-data-lab/internal/config"
	"github.com/josebianco/go-vue-data-lab/internal/infrastructure/database"
	"github.com/josebianco/go-vue-data-lab/internal/infrastructure/logger"
	"github.com/josebianco/go-vue-data-lab/internal/router"
)

// version is overridable at build time:
//
//	go build -ldflags "-X main.version=1.2.3" ./cmd/api
var version = "dev"

// shutdownTimeout bounds the graceful shutdown window.
const shutdownTimeout = 15 * time.Second

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

	targetDB, err := database.OpenTarget(cfg)
	if err != nil {
		return fmt.Errorf("main: open target database: %w", err)
	}
	defer func() { _ = database.Close(targetDB) }()

	engine := router.New(router.Dependencies{
		Logger:  log,
		Version: version,
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
