package main

import (
	"fmt"
	"log/slog"
	"os"

	server "github.com/borisdvlpr/itero/cmd"
	"github.com/borisdvlpr/itero/internal/config"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Info(fmt.Sprintf("unable to load .env file: %v. Using default values.", err))
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error(fmt.Sprintf("failed to load config: %v", err))
		os.Exit(1)
	}

	if err := server.Run(cfg); err != nil {
		slog.Error(fmt.Sprintf("server exited with error: %v", err))
		os.Exit(1)
	}
}
