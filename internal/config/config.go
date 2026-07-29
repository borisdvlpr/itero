package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	Address    string
	Port       string
	LogLevel   slog.Level
	Timeout    time.Duration
	PgUser     string
	PgPassword string
	PgHost     string
	PgPort     string
	PgDatabase string
	PgSslMode  string
}

func LoadConfig() (*Config, error) {
	timeout, err := time.ParseDuration(envOrDefault("TIMEOUT", "30s"))
	if err != nil {
		return nil, fmt.Errorf("invalid TIMEOUT value: %w", err)
	}

	if timeout <= 0 {
		return nil, fmt.Errorf("TIMEOUT must be positive, got %s", timeout)
	}

	return &Config{
		Address:    envOrDefault("ADDRESS", "0.0.0.0"),
		Port:       envOrDefault("PORT", "3000"),
		LogLevel:   envLogLevel(),
		Timeout:    timeout,
		PgUser:     envOrDefault("PG_USER", "postgres"),
		PgPassword: envOrDefault("PG_PASSWORD", "password"),
		PgHost:     envOrDefault("PG_HOST", "localhost"),
		PgPort:     envOrDefault("PG_PORT", "5432"),
		PgDatabase: envOrDefault("PG_DATABASE", "itero"),
		PgSslMode:  envOrDefault("PG_SSLMODE", "disable"),
	}, nil
}

func envOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
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
