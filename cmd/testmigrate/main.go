package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://era:era@localhost:5433/era_test?sslmode=disable"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal("Failed to open database connection:", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	fmt.Println("Connected to test database")

	// Create schema_migrations table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id BIGSERIAL PRIMARY KEY,
			filename TEXT NOT NULL UNIQUE,
			checksum TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
	if err != nil {
		log.Fatal("Failed to create schema_migrations table:", err)
	}

	// Get list of migration files
	migrationsDir := "db/migrations"
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		log.Fatal("Failed to read migrations directory:", err)
	}

	var migrationFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") {
			migrationFiles = append(migrationFiles, file.Name())
		}
	}

	// Sort files lexicographically
	sort.Strings(migrationFiles)

	fmt.Printf("Found %d migration files\n", len(migrationFiles))

	// Apply each migration
	for _, filename := range migrationFiles {
		filepath := filepath.Join(migrationsDir, filename)
		
		// Check if already applied
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE filename = $1", filename).Scan(&count)
		if err != nil {
			log.Fatal("Failed to check migration status:", err)
		}

		if count > 0 {
			fmt.Printf("Skipping %s (already applied)\n", filename)
			continue
		}

		// Read migration file
		content, err := os.ReadFile(filepath)
		if err != nil {
			log.Fatal("Failed to read migration file:", err)
		}

		// Apply migration
		fmt.Printf("Applying %s...\n", filename)
		_, err = db.Exec(string(content))
		if err != nil {
			log.Fatal("Failed to apply migration:", err)
		}

		// Record migration
		checksum := fmt.Sprintf("%x", len(content)) // Simple checksum for now
		_, err = db.Exec("INSERT INTO schema_migrations (filename, checksum) VALUES ($1, $2)", filename, checksum)
		if err != nil {
			log.Fatal("Failed to record migration:", err)
		}

		fmt.Printf("Applied %s successfully\n", filename)
	}

	fmt.Println("All migrations applied successfully")
}
