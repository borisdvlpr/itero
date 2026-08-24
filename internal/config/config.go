package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
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
	return fmt.Sprintf("%s:%s", s.Address, s.Port)
}

// DBConfig holds the Postgres connection settings and the migration switch.
type DBConfig struct {
	User          string
	Password      string
	Host          string
	Port          string
	Database      string
	SSLMode       string
	RunMigrations bool
}

// DSN renders the connection string shared by the pgx pool and golang-migrate.
func (d DBConfig) DSN() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(d.User, d.Password),
		Host:   fmt.Sprintf("%s:%s", d.Host, d.Port),
		Path:   d.Database,
	}

	q := u.Query()
	q.Set("sslmode", d.SSLMode)
	u.RawQuery = q.Encode()

	return u.String()
}

func LoadConfig() (*Config, error) {
	shutdownTimeout, err := time.ParseDuration(envOrDefault("SHUTDOWN_TIMEOUT", "30s"))
	if err != nil {
		return nil, fmt.Errorf("invalid SHUTDOWN_TIMEOUT value: %w", err)
	}

	if shutdownTimeout <= 0 {
		return nil, fmt.Errorf("SHUTDOWN_TIMEOUT must be positive, got %s", shutdownTimeout)
	}

	requestTimeout, err := time.ParseDuration(envOrDefault("REQUEST_TIMEOUT", "10s"))
	if err != nil {
		return nil, fmt.Errorf("invalid REQUEST_TIMEOUT value: %w", err)
	}

	if requestTimeout <= 0 {
		return nil, fmt.Errorf("REQUEST_TIMEOUT must be positive, got %s", requestTimeout)
	}

	maxRequestBytes, err := envInt64("MAX_REQUEST_BYTES", 1<<20)
	if err != nil {
		return nil, err
	}

	if maxRequestBytes <= 0 {
		return nil, fmt.Errorf("MAX_REQUEST_BYTES must be positive, got %d", maxRequestBytes)
	}

	runMigrations, err := envBoolean("RUN_MIGRATIONS", true)
	if err != nil {
		return nil, err
	}

	return &Config{
		LogLevel: envLogLevel(),
		Server: ServerConfig{
			Address:         envOrDefault("ADDRESS", "0.0.0.0"),
			Port:            envOrDefault("PORT", "8000"),
			ShutdownTimeout: shutdownTimeout,
			RequestTimeout:  requestTimeout,
			MaxRequestBytes: maxRequestBytes,
		},
		DB: DBConfig{
			User:          envOrDefault("PG_USER", "postgres"),
			Password:      envOrDefault("PG_PASSWORD", "password"),
			Host:          envOrDefault("PG_HOST", "localhost"),
			Port:          envOrDefault("PG_PORT", "5432"),
			Database:      envOrDefault("PG_DATABASE", "itero"),
			SSLMode:       envOrDefault("PG_SSLMODE", "disable"),
			RunMigrations: runMigrations,
		},
	}, nil
}

func envOrDefault(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}

func envInt64(key string, defaultValue int64) (int64, error) {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue, nil
	}

	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value: %w", key, err)
	}

	return n, nil
}

func envBoolean(key string, defaultValue bool) (bool, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue, nil
	}

	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: %q is not a valid boolean", key, raw)
	}

	return b, nil
}

func envLogLevel() slog.Level {
	levelStr := os.Getenv("LOG_LEVEL")

	switch strings.ToLower(levelStr) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
