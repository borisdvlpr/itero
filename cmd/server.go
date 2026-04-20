package cmd

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
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Run(cfg *config.Config) error {
	dsn := cfg.DSN()
	if err := db.RunMigrations(dsn, cfg.MigrationsPath); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	pool, err := db.NewConnectionPool(context.Background(), dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", cfg.Address, cfg.Port),
		Handler: service(pool),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)

	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	slog.Info(fmt.Sprintf("server listening on %s", srv.Addr))

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Timeout*time.Second)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}

func service(pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(requestLogMiddleware())

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
