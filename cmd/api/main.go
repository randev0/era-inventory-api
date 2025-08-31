package main

import (
	"log"
	"net/http"
	"os"

	"era-inventory-api/internal"
	"era-inventory-api/internal/config"
)

func main() {
	// Load and validate configuration
	cfg, err := config.LoadAndValidate()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Validate database connection string
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		log.Fatal("DB_DSN environment variable is required")
	}

	// Create and start server
	srv := internal.NewServer(dsn, cfg)

	log.Println("Starting Era Inventory API server...")
	log.Printf("JWT Issuer: %s", cfg.JWTIssuer)
	log.Printf("JWT Audience: %s", cfg.JWTAudience)
	log.Printf("JWT Expiry: %v", cfg.JWTExpiry)
	log.Println("Listening on :8080")

	log.Fatal(http.ListenAndServe(":8080", srv.Router))
}
