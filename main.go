package main

import (
	"log"

	server "github.com/borisdvlpr/itero/cmd"
	"github.com/borisdvlpr/itero/internal/config"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("unable to load .env file: %v. Using default values.", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if err := server.Run(cfg); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}
}
