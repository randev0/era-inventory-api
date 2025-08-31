package internal

import (
	"context"
	"database/sql"
	"os"
	"strconv"
)

type ctxKey string
const dbConnKey ctxKey = "dbconn"

func rlsEnabled() bool {
	return os.Getenv("RLS_ENABLED") == "true"
}

func withDBConn(ctx context.Context, db *sql.DB, orgID int64) (*sql.Conn, context.Context, error) {
	if !rlsEnabled() {
		return nil, ctx, nil
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, ctx, err
	}
	// Set session GUC for RLS
	_, err = conn.ExecContext(ctx, "SET app.current_org_id = $1", orgID)
	if err != nil {
		conn.Close()
		return nil, ctx, err
	}
	ctx2 := context.WithValue(ctx, dbConnKey, conn)
	return conn, ctx2, nil
}

// Prefer DB from context when RLS on; else use pool directly.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func dbFrom(ctx context.Context, db *sql.DB) querier {
	if !rlsEnabled() {
		return db
	}
	if v := ctx.Value(dbConnKey); v != nil {
		if c, ok := v.(*sql.Conn); ok {
			return c
		}
	}
	return db // fallback
}

func parseOrgID(s string) int64 {
	id, _ := strconv.ParseInt(s, 10, 64)
	if id <= 0 { id = 1 }
	return id
}
