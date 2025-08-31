package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Test default configuration
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("JWT_ISS")
	os.Unsetenv("JWT_AUD")
	os.Unsetenv("JWT_EXPIRY")

	cfg := Load()

	// Check defaults
	if cfg.JWTSecret != "your-secret-key-change-in-production" {
		t.Errorf("Expected default JWT_SECRET, got %s", cfg.JWTSecret)
	}
	if cfg.JWTIssuer != "era-inventory-api" {
		t.Errorf("Expected default JWT_ISS, got %s", cfg.JWTIssuer)
	}
	if cfg.JWTAudience != "era-inventory-api" {
		t.Errorf("Expected default JWT_AUD, got %s", cfg.JWTAudience)
	}
	if cfg.JWTExpiry != 24*time.Hour {
		t.Errorf("Expected default JWT_EXPIRY, got %v", cfg.JWTExpiry)
	}
}

func TestLoadWithEnvironment(t *testing.T) {
	// Test with environment variables
	os.Setenv("JWT_SECRET", "test-secret-key")
	os.Setenv("JWT_ISS", "test-issuer")
	os.Setenv("JWT_AUD", "test-audience")
	os.Setenv("JWT_EXPIRY", "2h")

	cfg := Load()

	// Check environment values
	if cfg.JWTSecret != "test-secret-key" {
		t.Errorf("Expected JWT_SECRET from env, got %s", cfg.JWTSecret)
	}
	if cfg.JWTIssuer != "test-issuer" {
		t.Errorf("Expected JWT_ISS from env, got %s", cfg.JWTIssuer)
	}
	if cfg.JWTAudience != "test-audience" {
		t.Errorf("Expected JWT_AUD from env, got %s", cfg.JWTAudience)
	}
	if cfg.JWTExpiry != 2*time.Hour {
		t.Errorf("Expected JWT_EXPIRY from env, got %v", cfg.JWTExpiry)
	}

	// Cleanup
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("JWT_ISS")
	os.Unsetenv("JWT_AUD")
	os.Unsetenv("JWT_EXPIRY")
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				JWTSecret:   "valid-secret-that-is-long-enough-for-testing",
				JWTIssuer:   "test-issuer",
				JWTAudience: "test-audience",
				JWTExpiry:   time.Hour,
			},
			expectError: false,
		},
		{
			name: "empty secret",
			config: &Config{
				JWTSecret:   "",
				JWTIssuer:   "test-issuer",
				JWTAudience: "test-audience",
				JWTExpiry:   time.Hour,
			},
			expectError: true,
		},
		{
			name: "secret too short",
			config: &Config{
				JWTSecret:   "short",
				JWTIssuer:   "test-issuer",
				JWTAudience: "test-audience",
				JWTExpiry:   time.Hour,
			},
			expectError: true,
		},
		{
			name: "empty issuer",
			config: &Config{
				JWTSecret:   "valid-secret-that-is-long-enough-for-testing",
				JWTIssuer:   "",
				JWTAudience: "test-audience",
				JWTExpiry:   time.Hour,
			},
			expectError: true,
		},
		{
			name: "empty audience",
			config: &Config{
				JWTSecret:   "valid-secret-that-is-long-enough-for-testing",
				JWTIssuer:   "test-issuer",
				JWTAudience: "",
				JWTExpiry:   time.Hour,
			},
			expectError: true,
		},
		{
			name: "negative expiry",
			config: &Config{
				JWTSecret:   "valid-secret-that-is-long-enough-for-testing",
				JWTIssuer:   "test-issuer",
				JWTAudience: "test-audience",
				JWTExpiry:   -time.Hour,
			},
			expectError: true,
		},
		{
			name: "zero expiry",
			config: &Config{
				JWTSecret:   "valid-secret-that-is-long-enough-for-testing",
				JWTIssuer:   "test-issuer",
				JWTAudience: "test-audience",
				JWTExpiry:   0,
			},
			expectError: true,
		},
		{
			name: "expiry too short",
			config: &Config{
				JWTSecret:   "valid-secret-that-is-long-enough-for-testing",
				JWTIssuer:   "test-issuer",
				JWTAudience: "test-audience",
				JWTExpiry:   30 * time.Second,
			},
			expectError: true,
		},
		{
			name: "expiry too long",
			config: &Config{
				JWTSecret:   "valid-secret-that-is-long-enough-for-testing",
				JWTIssuer:   "test-issuer",
				JWTAudience: "test-audience",
				JWTExpiry:   31 * 24 * time.Hour,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.expectError {
				t.Errorf("Validate() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestLoadAndValidate(t *testing.T) {
	// Test with valid configuration
	os.Setenv("JWT_SECRET", "test-secret-key-that-is-long-enough-for-testing")
	os.Setenv("JWT_ISS", "test-issuer")
	os.Setenv("JWT_AUD", "test-audience")
	os.Setenv("JWT_EXPIRY", "1h")

	cfg, err := LoadAndValidate()
	if err != nil {
		t.Errorf("LoadAndValidate() failed with valid config: %v", err)
	}
	if cfg == nil {
		t.Error("LoadAndValidate() returned nil config with valid config")
	}

	// Test with invalid configuration
	os.Setenv("JWT_SECRET", "short")
	
	_, err = LoadAndValidate()
	if err == nil {
		t.Error("LoadAndValidate() should fail with invalid config")
	}

	// Cleanup
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("JWT_ISS")
	os.Unsetenv("JWT_AUD")
	os.Unsetenv("JWT_EXPIRY")
}

func TestProductionSecretValidation(t *testing.T) {
	// Test production environment validation
	os.Setenv("ENVIRONMENT", "production")
	os.Setenv("JWT_SECRET", "your-secret-key-change-in-production")

	cfg := Load()
	err := cfg.Validate()
	if err == nil {
		t.Error("Production validation should fail with default secret")
	}

	// Test with proper production secret
	os.Setenv("JWT_SECRET", "proper-production-secret-that-is-long-enough")
	
	cfg = Load()
	err = cfg.Validate()
	if err != nil {
		t.Errorf("Production validation should pass with proper secret: %v", err)
	}

	// Cleanup
	os.Unsetenv("ENVIRONMENT")
	os.Unsetenv("JWT_SECRET")
}
