package config

import (
	"os"
	"time"
)

type Config struct {
	JWTSecret string
	JWTIssuer string
	JWTAudience string
	JWTExpiry time.Duration
}

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

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
