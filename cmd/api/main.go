// Command api is the entry point and composition root for the Saydalah
// pharmacy-management API. It wires configuration, the database pool,
// migrations, feature modules, and the HTTP server together — the only place
// where concrete dependencies are constructed and injected.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/config"
	"github.com/ebnsina/saydalah-api/internal/database"
	"github.com/ebnsina/saydalah-api/internal/migrations"
	"github.com/ebnsina/saydalah-api/internal/server"
	"github.com/ebnsina/saydalah-api/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg)
	slog.SetDefault(logger)

	// Root context cancelled on SIGINT/SIGTERM for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := migrations.Up(ctx, pool); err != nil {
		return err
	}
	logger.Info("migrations applied")

	st := store.NewStore(pool)
	tm := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTTTL)

	if err := bootstrapAdmin(ctx, st, cfg, logger); err != nil {
		return err
	}

	srv := server.New(cfg, logger, pool)
	registerModules(srv, st, tm, cfg)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       time.Minute,
	}

	return serve(ctx, httpServer, logger, cfg.ShutdownTimeout)
}

// serve runs the HTTP server and blocks until the context is cancelled, then
// drains in-flight requests within the shutdown timeout.
func serve(ctx context.Context, srv *http.Server, logger *slog.Logger, timeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received, draining connections")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
