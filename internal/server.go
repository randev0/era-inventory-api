package internal

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Server struct {
	DB     *sql.DB
	Router *chi.Mux
}

func NewServer(dsn string) *Server {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal("DB ping failed:", err)
	}

	s := &Server{DB: db, Router: chi.NewRouter()}
	s.routes() // defined in items.go
	return s
}
