package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// NewTestDB creates a new test database connection
func NewTestDB(t *testing.T) *sql.DB {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://era:era@localhost:5432/era_test?sslmode=disable"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("Failed to ping test database: %v", err)
	}

	// Cleanup on test completion
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Warning: failed to close test database: %v", err)
		}
	})

	return db
}

// ResetSchema resets the database schema and reapplies migrations + seeds
func ResetSchema(t *testing.T, db *sql.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Drop and recreate public schema
	_, err := db.ExecContext(ctx, "DROP SCHEMA public CASCADE")
	if err != nil {
		t.Fatalf("Failed to drop schema: %v", err)
	}

	_, err = db.ExecContext(ctx, "CREATE SCHEMA public")
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Reapply migrations
	if err := runMigrations(ctx, db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Reapply seeds
	if err := runSeeds(ctx, db); err != nil {
		t.Fatalf("Failed to run seeds: %v", err)
	}
}

// runMigrations applies all migration files
func runMigrations(ctx context.Context, db *sql.DB) error {
	// Create schema_migrations table
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id BIGSERIAL PRIMARY KEY,
			filename TEXT NOT NULL UNIQUE,
			checksum TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Get list of migration files
	migrationsDir := "db/migrations"
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrationFiles []string
	for _, file := range files {
		if !file.IsDir() && len(file.Name()) > 4 && file.Name()[len(file.Name())-4:] == ".sql" {
			migrationFiles = append(migrationFiles, file.Name())
		}
	}

	// Sort files lexicographically
	for i := 0; i < len(migrationFiles)-1; i++ {
		for j := i + 1; j < len(migrationFiles); j++ {
			if migrationFiles[i] > migrationFiles[j] {
				migrationFiles[i], migrationFiles[j] = migrationFiles[j], migrationFiles[i]
			}
		}
	}

	// Apply each migration
	for _, filename := range migrationFiles {
		filepath := fmt.Sprintf("%s/%s", migrationsDir, filename)
		
		// Read migration file
		content, err := os.ReadFile(filepath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", filename, err)
		}

		// Apply migration
		_, err = db.ExecContext(ctx, string(content))
		if err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", filename, err)
		}

		// Record migration
		checksum := fmt.Sprintf("%x", len(content)) // Simple checksum
		_, err = db.ExecContext(ctx, 
			"INSERT INTO schema_migrations (filename, checksum) VALUES ($1, $2)", 
			filename, checksum)
		if err != nil {
			return fmt.Errorf("failed to record migration %s: %w", filename, err)
		}
	}

	return nil
}

// runSeeds applies seed files
func runSeeds(ctx context.Context, db *sql.DB) error {
	seedsDir := "db/seeds"
	files, err := os.ReadDir(seedsDir)
	if err != nil {
		// Seeds directory might not exist, that's OK
		return nil
	}

	var seedFiles []string
	for _, file := range files {
		if !file.IsDir() && len(file.Name()) > 4 && file.Name()[len(file.Name())-4:] == ".sql" {
			seedFiles = append(seedFiles, file.Name())
		}
	}

	// Sort files lexicographically
	for i := 0; i < len(seedFiles)-1; i++ {
		for j := i + 1; j < len(seedFiles); j++ {
			if seedFiles[i] > seedFiles[j] {
				seedFiles[i], seedFiles[j] = seedFiles[j], seedFiles[i]
			}
		}
	}

	// Apply each seed file
	for _, filename := range seedFiles {
		filepath := fmt.Sprintf("%s/%s", seedsDir, filename)
		
		// Read seed file
		content, err := os.ReadFile(filepath)
		if err != nil {
			return fmt.Errorf("failed to read seed file %s: %w", filename, err)
		}

		// Apply seed
		_, err = db.ExecContext(ctx, string(content))
		if err != nil {
			return fmt.Errorf("failed to apply seed %s: %w", filename, err)
		}
	}

	return nil
}

// RequireIntegration skips the test unless INTEGRATION=1
func RequireIntegration(t *testing.T) {
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("Skipping integration test. Set INTEGRATION=1 to run.")
	}
}
