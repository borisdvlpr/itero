package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Address string
	Port    string
	Timeout time.Duration
}

func LoadConfig() (*Config, error) {
	timeout, err := strconv.Atoi(envOrDefault("TIMEOUT", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid TIMEOUT value: %w", err)
	}

	return &Config{
		Address: envOrDefault("ADDRESS", "0.0.0.0"),
		Port:    envOrDefault("PORT", "3000"),
		Timeout: time.Duration(timeout),
	}, nil
}

func envOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}
