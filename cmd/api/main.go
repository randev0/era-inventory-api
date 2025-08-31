package main

import (
	"log"
	"net/http"
	"os"

	"era-inventory-api/internal"
	"era-inventory-api/internal/config"
)

func main() {
	// Load configuration
	cfg := config.Load()

	dsn := os.Getenv("DB_DSN") // postgres://postgres:postgres@db:5432/era?sslmode=disable
	srv := internal.NewServer(dsn, cfg)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", srv.Router))
}
