package internal

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMetricsEndpoint(t *testing.T) {
	// Test with metrics enabled
	os.Setenv("ENABLE_METRICS", "true")
	defer os.Unsetenv("ENABLE_METRICS")

	// Create a new metrics instance
	metrics := NewMetrics()

	// Create a Chi router with test mode
	router := chi.NewRouter()

	// Add metrics middleware
	router.Use(metrics.Middleware())

	// Add a test endpoint
	router.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	// Mount metrics endpoint
	router.Get("/metrics", metrics.Handler().ServeHTTP)

	// Make a request to generate some metrics
	testReq := httptest.NewRequest("GET", "/ping", nil)
	testW := httptest.NewRecorder()
	router.ServeHTTP(testW, testReq)

	// Verify the test request worked
	if testW.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", testW.Code)
	}
	if testW.Body.String() != "pong" {
		t.Errorf("Expected body 'pong', got '%s'", testW.Body.String())
	}

	// Now test metrics endpoint
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check that metrics are returned
	body := w.Body.String()
	if body == "" {
		t.Error("Expected non-empty metrics response")
	}

	// Check for expected metric names
	expectedMetrics := []string{"http_requests_total", "http_request_duration_seconds"}
	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("Expected metric '%s' not found in response", metric)
		}
	}

	// Check that we have metrics for the /ping endpoint
	if !strings.Contains(body, `path="/ping"`) {
		t.Error("Expected metrics to contain path label for /ping endpoint")
	}
}

func TestMetricsEndpointDisabled(t *testing.T) {
	// Test with metrics disabled
	os.Setenv("ENABLE_METRICS", "false")
	defer os.Unsetenv("ENABLE_METRICS")

	// Create a new metrics instance
	metrics := NewMetrics()

	// Create a Chi router
	router := chi.NewRouter()

	// Mount metrics endpoint
	router.Get("/metrics", metrics.Handler().ServeHTTP)

	// Test metrics endpoint
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should still work even when disabled (just no metrics collected)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestMetricsMiddleware(t *testing.T) {
	metrics := NewMetrics()

	// Create a Chi router
	router := chi.NewRouter()

	// Add metrics middleware
	router.Use(metrics.Middleware())

	// Create a test handler
	router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Test the middleware
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "test response" {
		t.Errorf("Expected body 'test response', got '%s'", w.Body.String())
	}
}

func TestMetricsWithChiRoutePatterns(t *testing.T) {
	// Test with metrics enabled
	os.Setenv("ENABLE_METRICS", "true")
	defer os.Unsetenv("ENABLE_METRICS")

	metrics := NewMetrics()
	router := chi.NewRouter()

	// Add metrics middleware
	router.Use(metrics.Middleware())

	// Add a parameterized route
	router.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("user"))
	})

	// Mount metrics endpoint
	router.Get("/metrics", metrics.Handler().ServeHTTP)

	// Make a request to generate metrics
	testReq := httptest.NewRequest("GET", "/users/123", nil)
	testW := httptest.NewRecorder()
	router.ServeHTTP(testW, testReq)

	// Now check metrics
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Should contain the route pattern, not the actual path
	if !strings.Contains(body, `path="/users/{id}"`) {
		t.Error("Expected metrics to contain Chi route pattern, not actual path")
	}
}
