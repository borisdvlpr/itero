package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"slices"
	"strconv"
	"time"
)

// Config is the root of itero's configuration. Everything below it is grouped
// by the subsystem that consumes it, so a component can be handed the section
// it needs rather than the whole struct.
type Config struct {
	LogLevel slog.Level
	Server   ServerConfig
	DB       DBConfig
}

// ServerConfig holds the HTTP listener and request-handling settings.
type ServerConfig struct {
	Address         string
	Port            string
	ShutdownTimeout time.Duration
	RequestTimeout  time.Duration
	MaxRequestBytes int64
}

// Addr renders the listen address in the form http.Server expects.
func (s ServerConfig) Addr() string {
	return net.JoinHostPort(s.Address, s.Port)
}

// DBConfig holds the Postgres connection settings and the migration switch.
type DBConfig struct {
	User              string
	Password          string
	Host              string
	Port              string
	Database          string
	SSLMode           string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	ConnectTimeout    time.Duration
	RunMigrations     bool
}

func (d DBConfig) Dsn() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(d.User, d.Password),
		Host:   net.JoinHostPort(d.Host, d.Port),
		Path:   d.Database,
	}

	q := u.Query()
	q.Set("sslmode", d.SSLMode)
	u.RawQuery = q.Encode()

	return u.String()
}

// LogValue implements slog.LogValuer so that logging a DBConfig — directly or
// as part of a Config — can never leak the password.
func (d DBConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("user", d.User),
		slog.String("password", "REDACTED"),
		slog.String("host", d.Host),
		slog.String("port", d.Port),
		slog.String("database", d.Database),
		slog.String("sslmode", d.SSLMode),
		slog.Int("max_conns", int(d.MaxConns)),
		slog.Bool("run_migrations", d.RunMigrations),
	)
}

// LoadConfig reads configuration from the environment. Every problem found is
// collected and returned together, so a misconfigured deployment reports all
// of its mistakes on the first boot rather than one per restart.
func LoadConfig() (*Config, error) {
	var l loader

	cfg := &Config{
		LogLevel: l.logLevel("LOG_LEVEL", slog.LevelInfo),
		Server: ServerConfig{
			Address:         l.optional("ADDRESS", "0.0.0.0"),
			Port:            l.optional("PORT", "8000"),
			ShutdownTimeout: l.duration("SHUTDOWN_TIMEOUT", "30s"),
			RequestTimeout:  l.duration("REQUEST_TIMEOUT", "10s"),
			MaxRequestBytes: l.integer("MAX_REQUEST_BYTES", 1<<20),
		},
		DB: DBConfig{
			User:              l.optional("PG_USER", "postgres"),
			Password:          l.required("PG_PASSWORD"),
			Host:              l.optional("PG_HOST", "localhost"),
			Port:              l.optional("PG_PORT", "5432"),
			Database:          l.optional("PG_DATABASE", "itero"),
			SSLMode:           l.enum("PG_SSLMODE", "disable", "disable", "allow", "prefer", "require", "verify-ca", "verify-full"),
			MaxConns:          int32(l.integer("PG_MAX_CONNS", 10)),
			MinConns:          int32(l.integer("PG_MIN_CONNS", 2)),
			MaxConnLifetime:   l.duration("PG_MAX_CONN_LIFETIME", "1h"),
			MaxConnIdleTime:   l.duration("PG_MAX_CONN_IDLE_TIME", "30m"),
			HealthCheckPeriod: l.duration("PG_HEALTH_CHECK_PERIOD", "1m"),
			ConnectTimeout:    l.duration("PG_CONNECT_TIMEOUT", "30s"),
			RunMigrations:     l.boolean("RUN_MIGRATIONS", true),
		},
	}

	if err := l.err(); err != nil {
		return nil, err
	}

	if cfg.DB.MinConns > cfg.DB.MaxConns {
		return nil, fmt.Errorf("PG_MIN_CONNS (%d) must not exceed PG_MAX_CONNS (%d)", cfg.DB.MinConns, cfg.DB.MaxConns)
	}

	return cfg, nil
}

// loader accumulates configuration errors instead of returning on the first one.
type loader struct {
	errs []error
}

func (l *loader) fail(format string, args ...any) {
	l.errs = append(l.errs, fmt.Errorf(format, args...))
}

func (l *loader) err() error {
	return errors.Join(l.errs...)
}

func (l *loader) optional(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return defaultValue
}

func (l *loader) required(key string) string {
	v := os.Getenv(key)
	if v == "" {
		l.fail("%s is required but is not set", key)
	}

	return v
}

func (l *loader) enum(key, defaultValue string, allowed ...string) string {
	v := l.optional(key, defaultValue)
	if slices.Contains(allowed, v) {
		return v
	}

	l.fail("%s: %q is not one of %v", key, v, allowed)
	return defaultValue
}

func (l *loader) duration(key, defaultValue string) time.Duration {
	raw := l.optional(key, defaultValue)

	d, err := time.ParseDuration(raw)
	if err != nil {
		l.fail("%s: %q is not a valid duration (want e.g. 30s, 5m, 1h)", key, raw)
		return 0
	}

	if d <= 0 {
		l.fail("%s must be positive, got %s", key, d)
	}

	return d
}

func (l *loader) integer(key string, defaultValue int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}

	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		l.fail("%s: %q is not a valid integer", key, raw)
		return defaultValue
	}

	if n <= 0 {
		l.fail("%s must be positive, got %d", key, n)
	}

	return n
}

func (l *loader) boolean(key string, defaultValue bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}

	b, err := strconv.ParseBool(raw)
	if err != nil {
		l.fail("%s: %q is not a valid boolean", key, raw)
		return defaultValue
	}

	return b
}

// logLevel rejects unknown values rather than silently falling back to the default value
func (l *loader) logLevel(key string, defaultValue slog.Level) slog.Level {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}

	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(raw)); err != nil {
		l.fail("%s: %q is not a valid log level (want debug, info, warn or error)", key, raw)
		return defaultValue
	}

	return lvl
}
