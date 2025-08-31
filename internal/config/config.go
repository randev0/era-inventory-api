package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	JWTSecret   string
	JWTIssuer   string
	JWTAudience string
	JWTExpiry   time.Duration
}

// Load loads configuration from environment variables
func Load() *Config {
	config := &Config{
		JWTSecret:   getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		JWTIssuer:   getEnv("JWT_ISS", "era-inventory-api"),
		JWTAudience: getEnv("JWT_AUD", "era-inventory-api"),
		JWTExpiry:   24 * time.Hour, // Default to 24 hours
	}

	// Parse JWT expiry from environment if provided
	if expiryStr := os.Getenv("JWT_EXPIRY"); expiryStr != "" {
		if expiry, err := time.ParseDuration(expiryStr); err == nil {
			config.JWTExpiry = expiry
		}
	}

	return config
}

// Validate performs comprehensive configuration validation
func (c *Config) Validate() error {
	// Validate JWT configuration
	if c.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET environment variable is required")
	}
	
	// Check if using default secret in production
	if c.JWTSecret == "your-secret-key-change-in-production" {
		if os.Getenv("ENVIRONMENT") == "production" {
			return fmt.Errorf("JWT_SECRET must be changed from default value in production")
		}
	}
	
	// Validate JWT secret length
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters long (current: %d)", len(c.JWTSecret))
	}
	
	if c.JWTIssuer == "" {
		return fmt.Errorf("JWT_ISS environment variable is required")
	}
	
	if c.JWTAudience == "" {
		return fmt.Errorf("JWT_AUD environment variable is required")
	}
	
	if c.JWTExpiry <= 0 {
		return fmt.Errorf("JWT_EXPIRY must be positive (current: %v)", c.JWTExpiry)
	}
	
	// Validate reasonable expiry limits
	if c.JWTExpiry < time.Minute {
		return fmt.Errorf("JWT_EXPIRY too short: %v (minimum: 1m)", c.JWTExpiry)
	}
	if c.JWTExpiry > 30*24*time.Hour {
		return fmt.Errorf("JWT_EXPIRY too long: %v (maximum: 30d)", c.JWTExpiry)
	}
	
	return nil
}

// LoadAndValidate loads and validates configuration
func LoadAndValidate() (*Config, error) {
	config := Load()
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	return config, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
