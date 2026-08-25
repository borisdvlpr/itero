package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/borisdvlpr/itero/internal/config"
	"github.com/borisdvlpr/itero/internal/db"
	"github.com/borisdvlpr/itero/internal/handler"
	mw "github.com/borisdvlpr/itero/internal/middleware"
	"github.com/borisdvlpr/itero/internal/response"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// ofrepPrefix is the namespace reserved by the OpenFeature Remote Evaluation
// Protocol. Routes are mounted under it and errors below it are rendered in
// OFREP's own shapes, so both must be derived from this one constant.
const ofrepPrefix = "/ofrep"

// readinessTimeout bounds the database check behind /readyz. It is kept short
// and independent of RequestTimeout: a probe that takes longer than the
// kubelet's own timeout is indistinguishable from a failure.
const readinessTimeout = 2 * time.Second

type Dependencies struct {
	Db handler.Pingable
}

func Run(cfg *config.Config) error {
	logger := slog.Default()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// The pool is created before migrations so that its startup backoff
	// absorbs a database that is not accepting connections yet.
	pool, err := db.NewConnectionPool(context.Background(), db.PoolConfig{
		DSN:               cfg.DB.Dsn(),
		MaxConns:          cfg.DB.MaxConns,
		MinConns:          cfg.DB.MinConns,
		MaxConnLifetime:   cfg.DB.MaxConnLifetime,
		MaxConnIdleTime:   cfg.DB.MaxConnIdleTime,
		HealthCheckPeriod: cfg.DB.HealthCheckPeriod,
		ConnectTimeout:    cfg.DB.ConnectTimeout,
		ApplicationName:   "itero",
	})
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
		Addr:              cfg.Server.Addr(),
		Handler:           service(cfg.Server, logger, Dependencies{Db: pool}),
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
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

func service(cfg config.ServerConfig, logger *slog.Logger, deps Dependencies) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)

	if cfg.TrustProxyHeaders {
		r.Use(chimw.RealIP)
	}

	// Ahead of every middleware below that can write an error body: chi runs this
	// chain before it matches a route, so a sub-router could not mark the dialect in time.
	r.Use(response.Dialects(ofrepPrefix))

	r.Use(mw.RequestLogger(logger, "/healthz", "/readyz"))
	r.Use(mw.Recoverer(logger))
	r.Use(mw.MaxBodyBytes(cfg.MaxRequestBytes))

	if cfg.RequestTimeout > 0 {
		r.Use(chimw.Timeout(cfg.RequestTimeout))
	}

	health := handler.NewHealthHandler(deps.Db, readinessTimeout)
	health.Routes(r)

	return r
}
