package auth

import (
	"context"
	"net/http"
	"strings"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// UserIDKey is the context key for user ID
	UserIDKey contextKey = "userID"
	// OrgIDKey is the context key for organization ID
	OrgIDKey contextKey = "orgID"
	// RolesKey is the context key for user roles
	RolesKey contextKey = "roles"
)

// ClaimsFromContext extracts the JWT claims from the request context
func ClaimsFromContext(ctx context.Context) *Claims {
	if claims, ok := ctx.Value("claims").(*Claims); ok {
		return claims
	}
	return nil
}

// UserIDFromContext extracts the user ID from the request context
func UserIDFromContext(ctx context.Context) int64 {
	if v := ctx.Value(UserIDKey); v != nil {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}

// OrgIDFromContext extracts the organization ID from the request context
func OrgIDFromContext(ctx context.Context) int64 {
	if v := ctx.Value(OrgIDKey); v != nil {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}

// RolesFromContext extracts the user roles from the request context
func RolesFromContext(ctx context.Context) []string {
	if v := ctx.Value(RolesKey); v != nil {
		if roles, ok := v.([]string); ok {
			return roles
		}
	}
	return nil
}

// Public paths that don't require authentication
var publicPaths = map[string]bool{
	"/health": true,
	"/dbping": true,
}

// isPublicPath checks if the given path is public (no auth required)
func isPublicPath(path string) bool {
	return publicPaths[path]
}

// AuthMiddleware validates JWT tokens and sets user context
func AuthMiddleware(jwtManager *JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if this is a public path
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Extract Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error": "Authorization header required"}`, http.StatusUnauthorized)
				return
			}

			// Check Bearer token format
			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"error": "Invalid authorization header format. Expected: Bearer <token>"}`, http.StatusUnauthorized)
				return
			}

			// Extract token
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == "" {
				http.Error(w, `{"error": "Token is required"}`, http.StatusUnauthorized)
				return
			}

			// Validate token
			claims, err := jwtManager.ValidateToken(tokenString)
			if err != nil {
				http.Error(w, `{"error": "Invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			// Set user context
			ctx := context.WithValue(r.Context(), "claims", claims)
			ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, OrgIDKey, claims.OrgID)
			ctx = context.WithValue(ctx, RolesKey, claims.Roles)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MustRole creates middleware that requires specific roles
func MustRole(requiredRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromContext(r.Context())
			if claims == nil {
				http.Error(w, `{"error": "Authentication required"}`, http.StatusUnauthorized)
				return
			}

			if !claims.HasRole(requiredRoles...) {
				http.Error(w, `{"error": "Insufficient permissions"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
