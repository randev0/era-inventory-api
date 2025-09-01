package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// ClaimsKey is the context key for JWT claims
	ClaimsKey contextKey = "claims"
	// UserIDKey is the context key for user ID
	UserIDKey contextKey = "userID"
	// OrgIDKey is the context key for organization ID
	OrgIDKey contextKey = "orgID"
	// RolesKey is the context key for user roles
	RolesKey contextKey = "roles"
)

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// TokenExpirationWarning represents a token expiration warning
type TokenExpirationWarning struct {
	Warning     string    `json:"warning"`
	ExpiresAt   time.Time `json:"expires_at"`
	ExpiresIn   string    `json:"expires_in"`
	Code        string    `json:"code"`
}

// ClaimsFromContext extracts the JWT claims from the request context
func ClaimsFromContext(ctx context.Context) *Claims {
	if claims, ok := ctx.Value(ClaimsKey).(*Claims); ok {
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

// sendErrorResponse sends a standardized error response
func sendErrorResponse(w http.ResponseWriter, message, code string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := ErrorResponse{
		Error: message,
		Code:  code,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// sendTokenExpirationWarning adds a warning header when token expires soon
func sendTokenExpirationWarning(w http.ResponseWriter, expiresAt time.Time) {
	timeUntilExpiry := time.Until(expiresAt)
	if timeUntilExpiry <= time.Hour && timeUntilExpiry > 0 {
		w.Header().Set("X-Token-Expires-At", expiresAt.Format(time.RFC3339))
		w.Header().Set("X-Token-Expires-In", timeUntilExpiry.String())
	}
}

// validateTokenFormat performs basic token format validation
func validateTokenFormat(tokenString string) error {
	if len(tokenString) == 0 {
		return errors.New("token cannot be empty")
	}
	if len(tokenString) > 8192 { // 8KB limit
		return errors.New("token size exceeds maximum allowed")
	}
	// Basic JWT format validation (3 parts separated by dots)
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return errors.New("invalid JWT token format")
	}
	return nil
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
				sendErrorResponse(w, "Authorization header required", "MISSING_AUTH_HEADER", http.StatusUnauthorized)
				return
			}

			// Check Bearer token format
			if !strings.HasPrefix(authHeader, "Bearer ") {
				sendErrorResponse(w, "Invalid authorization header format. Expected: Bearer <token>", "INVALID_AUTH_FORMAT", http.StatusUnauthorized)
				return
			}

			// Extract token
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == "" {
				sendErrorResponse(w, "Token is required", "MISSING_TOKEN", http.StatusUnauthorized)
				return
			}

			// Validate token format
			if err := validateTokenFormat(tokenString); err != nil {
				sendErrorResponse(w, "Invalid token format: "+err.Error(), "INVALID_TOKEN_FORMAT", http.StatusUnauthorized)
				return
			}

			// Validate token
			claims, err := jwtManager.ValidateToken(tokenString)
			if err != nil {
				// Determine specific error type
				var errorCode string
				var errorMessage string
				
				if strings.Contains(err.Error(), "expired") {
					errorCode = "TOKEN_EXPIRED"
					errorMessage = "Token has expired"
				} else if strings.Contains(err.Error(), "signing method") {
					errorCode = "INVALID_SIGNING_METHOD"
					errorMessage = "Invalid token signing method"
				} else if strings.Contains(err.Error(), "malformed") {
					errorCode = "MALFORMED_TOKEN"
					errorMessage = "Token is malformed"
				} else {
					errorCode = "INVALID_TOKEN"
					errorMessage = "Invalid or expired token"
				}
				
				sendErrorResponse(w, errorMessage, errorCode, http.StatusUnauthorized)
				return
			}

			// Validate claims
			if claims.UserID <= 0 {
				sendErrorResponse(w, "Invalid user ID in token", "INVALID_USER_ID", http.StatusUnauthorized)
				return
			}
			if claims.OrgID <= 0 {
				sendErrorResponse(w, "Invalid organization ID in token", "INVALID_ORG_ID", http.StatusUnauthorized)
				return
			}
			if len(claims.Roles) == 0 {
				sendErrorResponse(w, "No roles assigned to user", "NO_ROLES", http.StatusUnauthorized)
				return
			}

			// Set user context
			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			ctx = context.WithValue(ctx, UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, OrgIDKey, claims.OrgID)
			ctx = context.WithValue(ctx, RolesKey, claims.Roles)

			// Add token expiration warning header if needed
			if claims.ExpiresAt != nil {
				sendTokenExpirationWarning(w, claims.ExpiresAt.Time)
			}

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
				sendErrorResponse(w, "Authentication required", "AUTHENTICATION_REQUIRED", http.StatusUnauthorized)
				return
			}

			// Validate required roles
			if len(requiredRoles) == 0 {
				sendErrorResponse(w, "No roles specified for this endpoint", "NO_ROLES_SPECIFIED", http.StatusInternalServerError)
				return
			}

			// Sanitize role names
			sanitizedRoles := make([]string, 0, len(requiredRoles))
			for _, role := range requiredRoles {
				if role != "" && len(role) <= 50 { // Reasonable role name length
					sanitizedRoles = append(sanitizedRoles, strings.TrimSpace(role))
				}
			}

			if !claims.HasRole(sanitizedRoles...) {
				sendErrorResponse(w, "Insufficient permissions", "INSUFFICIENT_PERMISSIONS", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
