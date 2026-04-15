// Package main is the entry point of the GoApi server.
package main

import (
	"log"
	"path/filepath"

	"github.com/joho/godotenv"

	"cmd/go-api/internal/config"
	appHTTP "cmd/go-api/internal/interfaces/http"
	"cmd/go-api/internal/infrastructure/stats"
)

func main() {
	// Load .env if present. In production variables come from the container environment.
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found — using environment variables directly.")
	}

	cfg := config.Load()

	counter, err := stats.New(filepath.Join(cfg.DataDir, "stats.json"))
	if err != nil {
		log.Fatalf("Failed to initialise stats counter: %v", err)
	}

	log.Printf("Starting GoApi on port %s", cfg.Port)
	log.Printf("Swagger UI → https://%s/docs", cfg.URLHostDomain)
	log.Printf("OpenAPI spec → https://%s/openapi.json", cfg.URLHostDomain)

	router := appHTTP.NewRouter(cfg, counter)

	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Server stopped with error: %v", err)
	}
}
