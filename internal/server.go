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
	"era-inventory-api/internal/handlers"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed openapi
var openapiFS embed.FS

type Server struct {
	DB         *sql.DB
	Pool       *pgxpool.Pool
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

	// Also create a pgxpool for the importer
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal("Failed to create pgxpool:", err)
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
		Pool:       pool,
		Router:     chi.NewRouter(),
		JWTManager: jwtManager,
		Metrics:    metrics,
	}
	// Mount public routes FIRST (no middleware)
	s.Router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("ok")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	s.Router.Get("/dbping", func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write([]byte("db: ok")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Public auth routes (no JWT required)
	s.Router.Post("/auth/login", s.loginUser)
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
		if _, err := w.Write(data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Serve enhanced Swagger UI page
	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte(`<!doctype html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Era Inventory API - Documentation</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui.css">
    <style>
        body { margin: 0; background: #f7f7f7; }
        .swagger-ui .topbar { background: #1f2937; border-bottom: 3px solid #3b82f6; }
        .swagger-ui .topbar .download-url-wrapper { display: none; }
        .swagger-ui .info { margin: 20px 0; }
        .swagger-ui .info .title { color: #1f2937; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-bundle.js"></script>
    <script>
        window.onload = function() {
            window.ui = SwaggerUIBundle({
                url: '/openapi.yaml',
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIBundle.presets.standalone
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout",
                tryItOutEnabled: true,
                requestInterceptor: function(req) {
                    // Add custom headers or modify requests here if needed
                    return req;
                },
                responseInterceptor: function(res) {
                    // Handle responses here if needed
                    return res;
                }
            });
        };
    </script>
</body>
</html>`))
	})
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

	// Assets - require project_admin/org_admin for write operations
	r.Get("/assets", s.listAssets)
	r.Get("/assets/{id}", s.getAsset)
	r.Post("/assets", auth.MustRole("org_admin", "project_admin")(http.HandlerFunc(s.createAsset)).(http.HandlerFunc))
	r.Put("/assets/{id}", auth.MustRole("org_admin", "project_admin")(http.HandlerFunc(s.updateAsset)).(http.HandlerFunc))
	r.Delete("/assets/{id}", auth.MustRole("org_admin")(http.HandlerFunc(s.deleteAsset)).(http.HandlerFunc))

	// Asset subtypes
	r.Get("/switches", s.listSwitches)
	r.Get("/vlans", s.listVLANs)

	// Site asset categories
	r.Get("/sites/{id}/asset-categories", s.getSiteAssetCategories)

	// Excel import - require project_admin/org_admin
	importsHandler := handlers.NewImportsHandler(s.Pool)
	r.Post("/imports/excel", auth.MustRole("org_admin", "project_admin")(http.HandlerFunc(importsHandler.UploadExcel)).(http.HandlerFunc))

	// User management - org_admin only, with multi-tenant logic
	r.Post("/users", auth.MustRole("org_admin")(http.HandlerFunc(s.createUser)).(http.HandlerFunc))
	r.Get("/users", auth.MustRole("org_admin")(http.HandlerFunc(s.listUsers)).(http.HandlerFunc))
	r.Get("/users/{id}", auth.MustRole("org_admin")(http.HandlerFunc(s.getUser)).(http.HandlerFunc))
	r.Put("/users/{id}", auth.MustRole("org_admin")(http.HandlerFunc(s.updateUser)).(http.HandlerFunc))
	r.Delete("/users/{id}", auth.MustRole("org_admin")(http.HandlerFunc(s.deleteUser)).(http.HandlerFunc))

	// Organization management - main tenant only
	r.Get("/organizations", auth.MustRole("org_admin")(http.HandlerFunc(s.listOrganizations)).(http.HandlerFunc))
	r.Get("/organizations/{id}", auth.MustRole("org_admin")(http.HandlerFunc(s.getOrganization)).(http.HandlerFunc))
	r.Get("/organizations/{id}/stats", auth.MustRole("org_admin")(http.HandlerFunc(s.getOrganizationStats)).(http.HandlerFunc))
	r.Post("/organizations", auth.MustRole("org_admin")(http.HandlerFunc(s.createOrganization)).(http.HandlerFunc))
	r.Put("/organizations/{id}", auth.MustRole("org_admin")(http.HandlerFunc(s.updateOrganization)).(http.HandlerFunc))
	r.Delete("/organizations/{id}", auth.MustRole("org_admin")(http.HandlerFunc(s.deleteOrganization)).(http.HandlerFunc))

	// Self-service routes
	r.Get("/auth/profile", s.getUserProfile)
	r.Put("/auth/profile", s.updateUserProfile)
	r.Put("/auth/change-password", s.changePassword)
}
