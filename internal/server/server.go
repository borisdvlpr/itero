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
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(cfg *config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dsn := cfg.DSN()

	pool, err := db.NewConnectionPool(ctx, dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	if err := db.RunMigrations(dsn); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", cfg.Address, cfg.Port),
		Handler: service(pool),
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	slog.Info("server stopped")
	return nil
}

func service(pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogMiddleware())
	r.Use(middleware.Recoverer)

	health := handler.NewHealthHandler(pool)
	health.Routes(r)

	return r
}

func requestLogMiddleware() func(http.Handler) http.Handler {
	return middleware.RequestLogger(&middleware.DefaultLogFormatter{
		Logger:  slog.NewLogLogger(slog.Default().Handler(), slog.LevelInfo),
		NoColor: true,
	})
}
