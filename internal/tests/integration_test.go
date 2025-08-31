//go:build integration

package tests

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"era-inventory-api/internal"
	"era-inventory-api/internal/auth"
	"era-inventory-api/internal/config"
	"era-inventory-api/internal/testutil"
)

var testServer *internal.Server
var testDB *sql.DB

func TestMain(m *testing.M) {
	// Skip if not running integration tests
	if os.Getenv("INTEGRATION") != "1" {
		os.Exit(0)
	}

	// Setup test database
	testDB = testutil.NewTestDB(&testing.T{})
	
	// Reset schema for clean state
	testutil.ResetSchema(&testing.T{}, testDB)

	// Create test config
	cfg := &config.Config{
		JWTSecret:   "supersecretkeyforintegrationtestingonly",
		JWTIssuer:   "era-inventory-api",
		JWTAudience: "era-inventory-api",
		JWTExpiry:   24 * time.Hour,
	}

	// Create test server
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://era:era@localhost:5432/era_test?sslmode=disable"
	}
	
	testServer = internal.NewServer(dsn, cfg)

	// Run tests
	code := m.Run()

	// Cleanup
	if testServer != nil {
		testServer.Close(context.Background())
	}
	if testDB != nil {
		testDB.Close()
	}

	os.Exit(code)
}

func TestHealthEndpoint(t *testing.T) {
	testutil.RequireIntegration(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	testServer.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "ok" {
		t.Errorf("Expected body 'ok', got '%s'", w.Body.String())
	}
}

func TestUnauthorizedAccess(t *testing.T) {
	testutil.RequireIntegration(t)

	req := httptest.NewRequest("GET", "/items", nil)
	w := httptest.NewRecorder()

	testServer.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestInvalidToken(t *testing.T) {
	testutil.RequireIntegration(t)

	req := httptest.NewRequest("GET", "/items", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	testServer.Router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestValidToken(t *testing.T) {
	testutil.RequireIntegration(t)

	// Create a valid JWT token for testing
	jwtManager := auth.NewJWTManager(
		"supersecretkeyforintegrationtestingonly",
		"era-inventory-api",
		"era-inventory-api",
		24*time.Hour,
	)

	userID := int64(1)
	orgID := int64(1)
	roles := []string{"org_admin"}

	token, err := jwtManager.GenerateToken(userID, orgID, roles)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	req := httptest.NewRequest("GET", "/items", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	w := httptest.NewRecorder()

	testServer.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestCreateItem(t *testing.T) {
	testutil.RequireIntegration(t)

	// Create a valid JWT token for testing
	jwtManager := auth.NewJWTManager(
		"supersecretkeyforintegrationtestingonly",
		"era-inventory-api",
		"era-inventory-api",
		24*time.Hour,
	)

	userID := int64(1)
	orgID := int64(1)
	roles := []string{"org_admin"}

	token, err := jwtManager.GenerateToken(userID, orgID, roles)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Test creating an item
	req := httptest.NewRequest("POST", "/items", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	testServer.Router.ServeHTTP(w, req)

	// Should get a 400 since we're not sending a body, but the auth should work
	if w.Code != http.StatusBadRequest && w.Code != http.StatusOK {
		t.Errorf("Expected status 400 or 200, got %d", w.Code)
	}
}

func TestInsufficientPermissions(t *testing.T) {
	testutil.RequireIntegration(t)

	// Create a JWT token with insufficient permissions
	jwtManager := auth.NewJWTManager(
		"supersecretkeyforintegrationtestingonly",
		"era-inventory-api",
		"era-inventory-api",
		24*time.Hour,
	)

	userID := int64(1)
	orgID := int64(1)
	roles := []string{"viewer"} // Only viewer role

	token, err := jwtManager.GenerateToken(userID, orgID, roles)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}

	// Try to create an item (requires org_admin role)
	req := httptest.NewRequest("POST", "/items", nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	testServer.Router.ServeHTTP(w, req)

	// Should get 403 Forbidden
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}
