package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address  string
	Port     string
	LogLevel slog.Level
	Timeout  time.Duration
}

func LoadConfig() (*Config, error) {
	timeout, err := strconv.Atoi(envOrDefault("TIMEOUT", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid TIMEOUT value: %w", err)
	}

	return &Config{
		Address:  envOrDefault("ADDRESS", "0.0.0.0"),
		Port:     envOrDefault("PORT", "3000"),
		LogLevel: envLogLevel(),
		Timeout:  time.Duration(timeout),
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
