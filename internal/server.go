package internal

import (
	"context"
	"database/sql"
	"embed"
	"log"
	"net/http"
	"os"
	"time"

	"era-inventory-api/internal/auth"
	"era-inventory-api/internal/config"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed openapi/openapi.yaml
var openapiFS embed.FS

type Server struct {
	DB         *sql.DB
	Router     *chi.Mux
	JWTManager *auth.JWTManager
	Metrics    *Metrics
}

func NewServer(dsn string, cfg *config.Config) *Server {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal("Failed to open database connection:", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal("Database ping failed:", err)
	}

	// Initialize JWT manager
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, cfg.JWTExpiry)

	// Validate JWT configuration
	if err := jwtManager.ValidateConfig(); err != nil {
		log.Fatal("JWT configuration validation failed:", err)
	}

	// Initialize metrics
	metrics := NewMetrics()

	s := &Server{
		DB:         db,
		Router:     chi.NewRouter(),
		JWTManager: jwtManager,
		Metrics:    metrics,
	}
	// Mount public routes FIRST (no middleware)
	s.Router.Get("/health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	s.Router.Get("/dbping", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("db: ok")) })
	s.mountDocs(s.Router)

	// Mount metrics if enabled
	if os.Getenv("ENABLE_METRICS") == "true" {
		s.Router.Use(s.Metrics.Middleware())
		s.Router.Get("/metrics", s.Metrics.Handler().ServeHTTP)
	}

	// Create a protected route group with middleware
	s.Router.Group(func(r chi.Router) {
		// Apply middleware to this group only
		r.Use(auth.AuthMiddleware(s.JWTManager))
		r.Use(s.withRLSSession)

		// Mount protected routes
		s.mountProtectedRoutes(r)
	})

	return s
}

// Close properly shuts down the server and cleans up resources
func (s *Server) Close(ctx context.Context) error {
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

// withRLSSession middleware for org isolation
func (s *Server) withRLSSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := auth.OrgIDFromContext(r.Context()) // from your JWT middleware
		conn, ctx2, err := withDBConn(r.Context(), s.DB, orgID)
		if err != nil {
			http.Error(w, "db acquire: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if conn != nil {
			defer conn.Close()
		}
		next.ServeHTTP(w, r.WithContext(ctx2))
	})
}

// mountDocs serves the OpenAPI spec and Swagger UI
func (s *Server) mountDocs(mux *chi.Mux) {
	// Check if Swagger is enabled
	if os.Getenv("ENABLE_SWAGGER") != "true" {
		return
	}

	// Serve the raw YAML
	mux.HandleFunc("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		data, err := openapiFS.ReadFile("openapi/openapi.yaml")
		if err != nil {
			http.Error(w, "Failed to read OpenAPI spec", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Write(data)
	})

	// Serve a minimal Swagger UI page from CDN
	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte(`<!doctype html>
<html>
<head><meta charset="utf-8"><title>Era Inventory API — Docs</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist/swagger-ui.css"></link>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"></script>
<script>
window.ui = SwaggerUIBundle({ url: '/openapi.yaml', dom_id: '#swagger-ui' });
</script>
</body>
</html>`))
	})
}

// mountPublicRoutes mounts public routes that bypass auth middleware
func (s *Server) mountPublicRoutes() {
	// Create a new router for public routes
	publicRouter := chi.NewRouter()

	// Public routes (no auth required)
	publicRouter.Get("/health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	publicRouter.Get("/dbping", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("db: ok")) })

	// Mount docs (public)
	s.mountDocs(publicRouter)

	// Mount metrics if enabled
	if os.Getenv("ENABLE_METRICS") == "true" {
		publicRouter.Use(s.Metrics.Middleware())
		publicRouter.Get("/metrics", s.Metrics.Handler().ServeHTTP)
	}

	// Mount public router to main router
	s.Router.Mount("/", publicRouter)
}

// mountProtectedRoutes mounts all protected routes that require authentication
func (s *Server) mountProtectedRoutes(r chi.Router) {
	// CRUD - require org_admin role for write operations
	r.Get("/items", s.listItems)
	r.Get("/items/{id}", s.getItem)
	r.Post("/items", auth.MustRole("org_admin", "project_admin")(http.HandlerFunc(s.createItem)).(http.HandlerFunc))
	r.Put("/items/{id}", auth.MustRole("org_admin", "project_admin")(http.HandlerFunc(s.updateItem)).(http.HandlerFunc))
	r.Delete("/items/{id}", auth.MustRole("org_admin")(http.HandlerFunc(s.deleteItem)).(http.HandlerFunc))

	// Sites - require org_admin role for write operations
	r.Get("/sites", s.listSites)
	r.Get("/sites/{id}", s.getSite)
	r.Post("/sites", auth.MustRole("org_admin")(http.HandlerFunc(s.createSite)).(http.HandlerFunc))
	r.Put("/sites/{id}", auth.MustRole("org_admin")(http.HandlerFunc(s.updateSite)).(http.HandlerFunc))
	r.Delete("/sites/{id}", auth.MustRole("org_admin")(http.HandlerFunc(s.deleteSite)).(http.HandlerFunc))

	// Vendors - require org_admin role for write operations
	r.Get("/vendors", s.listVendors)
	r.Get("/vendors/{id}", s.getVendor)
	r.Post("/vendors", auth.MustRole("org_admin")(http.HandlerFunc(s.createVendor)).(http.HandlerFunc))
	r.Put("/vendors/{id}", auth.MustRole("org_admin")(http.HandlerFunc(s.updateVendor)).(http.HandlerFunc))
	r.Delete("/vendors/{id}", auth.MustRole("org_admin")(http.HandlerFunc(s.deleteVendor)).(http.HandlerFunc))

	// Projects - require org_admin role for write operations
	r.Get("/projects", s.listProjects)
	r.Get("/projects/{id}", s.getProject)
	r.Post("/projects", auth.MustRole("org_admin")(http.HandlerFunc(s.createProject)).(http.HandlerFunc))
	r.Put("/projects/{id}", auth.MustRole("org_admin")(http.HandlerFunc(s.updateProject)).(http.HandlerFunc))
	r.Delete("/projects/{id}", auth.MustRole("org_admin")(http.HandlerFunc(s.deleteProject)).(http.HandlerFunc))
}
