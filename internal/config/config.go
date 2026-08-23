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

type Config struct {
	Address         string
	Port            string
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
	RequestTimeout  time.Duration
	MaxRequestBytes int64
	PgUser          string
	PgPassword      string
	PgHost          string
	PgPort          string
	PgDatabase      string
	PgSslMode       string
	RunMigrations   bool
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
		Address:         envOrDefault("ADDRESS", "0.0.0.0"),
		Port:            envOrDefault("PORT", "8000"),
		LogLevel:        envLogLevel(),
		ShutdownTimeout: shutdownTimeout,
		RequestTimeout:  requestTimeout,
		MaxRequestBytes: maxRequestBytes,
		PgUser:          envOrDefault("PG_USER", "postgres"),
		PgPassword:      envOrDefault("PG_PASSWORD", "password"),
		PgHost:          envOrDefault("PG_HOST", "localhost"),
		PgPort:          envOrDefault("PG_PORT", "5432"),
		PgDatabase:      envOrDefault("PG_DATABASE", "itero"),
		PgSslMode:       envOrDefault("PG_SSLMODE", "disable"),
		RunMigrations:   runMigrations,
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

func (cfg Config) DSN() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.PgUser, cfg.PgPassword),
		Host:   fmt.Sprintf("%s:%s", cfg.PgHost, cfg.PgPort),
		Path:   cfg.PgDatabase,
	}

	q := u.Query()
	q.Set("sslmode", cfg.PgSslMode)
	u.RawQuery = q.Encode()

	return u.String()
}
