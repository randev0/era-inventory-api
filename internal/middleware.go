package internal

import (
	"context"
	"net/http"
	"strconv"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// OrgIDKey is the context key for organization ID
	OrgIDKey contextKey = "orgID"
)

// OrgIDFromContext extracts the organization ID from the request context.
// Returns 1 as default if not found or invalid.
func OrgIDFromContext(ctx context.Context) int64 {
	if v := ctx.Value(OrgIDKey); v != nil {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 1
}

// RBACMiddleware reads the X-Org-ID header and stores the organization ID in the request context.
// If the header is missing or invalid, it defaults to orgID=1.
// This is a scaffold for future JWT validation and role-based access control.
func RBACMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read X-Org-ID header, default to "1" if missing
		orgIDStr := r.Header.Get("X-Org-ID")
		if orgIDStr == "" {
			orgIDStr = "1"
		}

		// Parse the organization ID
		orgID, err := strconv.ParseInt(orgIDStr, 10, 64)
		if err != nil {
			// If parsing fails, default to 1
			orgID = 1
		}

		// Store orgID in request context using custom key type
		ctx := context.WithValue(r.Context(), OrgIDKey, orgID)

		// TODO: Add JWT validation and role checking here
		// For now, just pass the request with orgID in context

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
