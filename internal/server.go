package internal

import (
	"context"
	"database/sql"
	"log"
	"time"

	"era-inventory-api/internal/auth"
	"era-inventory-api/internal/config"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Server struct {
	DB         *sql.DB
	Router     *chi.Mux
	JWTManager *auth.JWTManager
}

func NewServer(dsn string, cfg *config.Config) *Server {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal("DB ping failed:", err)
	}

	// Initialize JWT manager
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, cfg.JWTExpiry)

	s := &Server{
		DB:         db,
		Router:     chi.NewRouter(),
		JWTManager: jwtManager,
	}
	s.routes() // defined in items.go
	return s
}
