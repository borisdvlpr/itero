package main

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"

	"github.com/borisdvlpr/itero/internal/config"
	"github.com/borisdvlpr/itero/internal/server"
	"github.com/joho/godotenv"
)

var (
	version = "dev"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	loadDotEnv()

	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("starting itero",
		"version", version,
	)

	if err := server.Run(cfg); err != nil {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

// loadDotEnv reads .env when one is present. It is logged explicitly so that a
// stray .env on a production host can never silently override the environment.
func loadDotEnv() {
	err := godotenv.Load()

	switch {
	case err == nil:
		slog.Info("loaded configuration from .env")

	case errors.Is(err, fs.ErrNotExist):

	default:
		slog.Warn("found .env but could not load it", "error", err)
	}
}
