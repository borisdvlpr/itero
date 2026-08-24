package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/borisdvlpr/itero/internal/config"
	"github.com/borisdvlpr/itero/internal/db"
	"github.com/borisdvlpr/itero/internal/handler"
	mw "github.com/borisdvlpr/itero/internal/middleware"
	"github.com/borisdvlpr/itero/internal/response"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ofrepPrefix is the namespace reserved by the OpenFeature Remote Evaluation
// Protocol. Routes are mounted under it and errors below it are rendered in
// OFREP's own shapes, so both must be derived from this one constant.
const ofrepPrefix = "/ofrep"

func Run(cfg *config.Config) error {
	logger := slog.Default()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dsn := cfg.DB.Dsn()

	pool, err := db.NewConnectionPool(ctx, dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	if cfg.DB.RunMigrations {
		if err := db.RunMigrations(cfg.DB.Dsn()); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

	} else {
		logger.Info("migrations skipped", "reason", "RUN_MIGRATIONS is false")
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", cfg.Server.Address, cfg.Server.Port),
		Handler: service(cfg, logger, pool),
	}

	serverErr := make(chan error, 1)

	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	slog.Info("server listening", "addr", srv.Addr)

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():

	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	slog.Info("server stopped")
	return nil
}

func service(cfg *config.Config, logger *slog.Logger, pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)

	// Ahead of every middleware below that can write an error body: chi runs this
	// chain before it matches a route, so a sub-router could not mark the dialect in time.
	r.Use(response.Dialects(ofrepPrefix))

	r.Use(mw.RequestLogger(logger, "/healthz", "/readyz"))
	r.Use(mw.Recoverer(logger))
	r.Use(mw.MaxBodyBytes(cfg.Server.MaxRequestBytes))
	r.Use(chimw.Timeout(cfg.Server.RequestTimeout))

	health := handler.NewHealthHandler(pool)
	health.Routes(r)

	return r
}
