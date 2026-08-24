package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// migrationsFS embeds the SQL migrations into the binary so that the runtime
// image does not need to ship a migrations directory, and so that a binary can
// never be paired with a mismatched migration set.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// PoolConfig describes a connection pool. It deliberately duplicates a few
// fields from internal/config so that this package does not depend on how the
// process happens to be configured, which keeps it usable from tests.
type PoolConfig struct {
	DSN               string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	ConnectTimeout    time.Duration
	ApplicationName   string
}

func NewConnectionPool(ctx context.Context, cfg PoolConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	// Recycling every connection at exactly the same age produces a reconnection stampede; stagger it.
	poolCfg.MaxConnLifetimeJitter = cfg.MaxConnLifetime / 10

	if cfg.ApplicationName != "" {
		poolCfg.ConnConfig.RuntimeParams["application_name"] = cfg.ApplicationName
	}

	connectCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pingWithBackoff(connectCtx, pool); err != nil {
		pool.Close()
		return nil, err
	}

	slog.Info("database pool ready",
		"max_conns", poolCfg.MaxConns,
		"min_conns", poolCfg.MinConns,
		"max_conn_lifetime", poolCfg.MaxConnLifetime,
	)

	return pool, nil
}

// pingWithBackoff tolerates a database that is still starting up. Without it,
// a Postgres pod that is thirty seconds behind itero puts the deployment into
// CrashLoopBackOff instead of simply waiting.
func pingWithBackoff(ctx context.Context, pool *pgxpool.Pool) error {
	const (
		initialDelay = 250 * time.Millisecond
		maxDelay     = 5 * time.Second
	)

	delay := initialDelay

	for attempt := 1; ; attempt++ {
		err := pool.Ping(ctx)
		if err == nil {
			return nil
		}

		slog.Warn("database not ready, retrying",
			"attempt", attempt,
			"retry_in", delay,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return fmt.Errorf("database unreachable after %d attempts: %w", attempt, errors.Join(err, ctx.Err()))
		case <-time.After(delay):
		}

		delay = min(delay*2, maxDelay)
	}
}

// RunMigrations applies all outstanding migrations. golang-migrate takes a
// Postgres advisory lock, so concurrent replicas serialise rather than
// conflict — but they do serialise, which is why this is gated by
// RUN_MIGRATIONS and should move to a Helm pre-upgrade Job in Epic 5.
func RunMigrations(dsn string) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}
	defer source.Close()

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open sql connection: %w", err)
	}
	defer sqlDB.Close()

	driver, err := pgxmigrate.WithInstance(sqlDB, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("create migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("initialise migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("read migration version: %w", err)
	}

	slog.Info("database migrations applied", "version", version, "dirty", dirty)
	return nil
}
