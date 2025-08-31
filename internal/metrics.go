package internal

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics provides Prometheus metrics collection for HTTP requests
type Metrics struct {
	reqTotal   *prometheus.CounterVec
	reqLatency *prometheus.HistogramVec
	registry   *prometheus.Registry
}

// NewMetrics creates a new Metrics instance with a private Prometheus registry
func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()

	reqTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	reqLatency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	registry.MustRegister(reqTotal, reqLatency)

	return &Metrics{
		reqTotal:   reqTotal,
		reqLatency: reqLatency,
		registry:   registry,
	}
}

// Middleware returns a Chi middleware that collects metrics
func (m *Metrics) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a response writer that captures the status code
			rw := &statusRecorder{ResponseWriter: w, code: http.StatusOK}

			// Process the request
			next.ServeHTTP(rw, r)

			// Get the path (use Chi's route pattern if available)
			path := r.URL.Path
			if chiCtx := chi.RouteContext(r.Context()); chiCtx != nil && len(chiCtx.RoutePatterns) > 0 {
				path = chiCtx.RoutePatterns[len(chiCtx.RoutePatterns)-1]
			}

			// Record metrics
			status := http.StatusText(rw.code)
			m.reqTotal.WithLabelValues(r.Method, path, status).Inc()
			m.reqLatency.WithLabelValues(r.Method, path, status).Observe(time.Since(start).Seconds())
		})
	}
}

// Handler returns an http.Handler that serves Prometheus metrics
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// statusRecorder captures the HTTP status code for metrics
type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.code = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	return sr.ResponseWriter.Write(b)
}
