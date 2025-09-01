package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewJWTManager(t *testing.T) {
	secret := "test-secret-key-that-is-long-enough-for-testing"
	issuer := "test-issuer"
	audience := "test-audience"
	expiry := time.Hour

	manager := NewJWTManager(secret, issuer, audience, expiry)

	if manager.secret != secret {
		t.Errorf("Expected secret %s, got %s", secret, manager.secret)
	}
	if manager.issuer != issuer {
		t.Errorf("Expected issuer %s, got %s", issuer, manager.issuer)
	}
	if manager.audience != audience {
		t.Errorf("Expected audience %s, got %s", audience, manager.audience)
	}
	if manager.expiry != expiry {
		t.Errorf("Expected expiry %v, got %v", expiry, manager.expiry)
	}
}

func TestJWTManager_ValidateConfig(t *testing.T) {
	tests := []struct {
		name     string
		secret   string
		issuer   string
		audience string
		expiry   time.Duration
		wantErr  bool
	}{
		{
			name:     "valid config",
			secret:   "valid-secret-that-is-long-enough-for-testing",
			issuer:   "test-issuer",
			audience: "test-audience",
			expiry:   time.Hour,
			wantErr:  false,
		},
		{
			name:     "empty secret",
			secret:   "",
			issuer:   "test-issuer",
			audience: "test-audience",
			expiry:   time.Hour,
			wantErr:  true,
		},
		{
			name:     "secret too short",
			secret:   "short",
			issuer:   "test-issuer",
			audience: "test-audience",
			expiry:   time.Hour,
			wantErr:  true,
		},
		{
			name:     "empty issuer",
			secret:   "valid-secret-that-is-long-enough-for-testing",
			issuer:   "",
			audience: "test-audience",
			expiry:   time.Hour,
			wantErr:  true,
		},
		{
			name:     "empty audience",
			secret:   "valid-secret-that-is-long-enough-for-testing",
			issuer:   "test-issuer",
			audience: "",
			expiry:   time.Hour,
			wantErr:  true,
		},
		{
			name:     "negative expiry",
			secret:   "valid-secret-that-is-long-enough-for-testing",
			issuer:   "test-issuer",
			audience: "test-audience",
			expiry:   -time.Hour,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewJWTManager(tt.secret, tt.issuer, tt.audience, tt.expiry)
			err := manager.ValidateConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJWTManager_GenerateToken(t *testing.T) {
	secret := "test-secret-key-that-is-long-enough-for-testing"
	issuer := "test-issuer"
	audience := "test-audience"
	expiry := time.Hour

	manager := NewJWTManager(secret, issuer, audience, expiry)

	tests := []struct {
		name    string
		userID  int64
		orgID   int64
		roles   []string
		wantErr bool
	}{
		{
			name:    "valid token",
			userID:  1,
			orgID:   1,
			roles:   []string{"admin"},
			wantErr: false,
		},
		{
			name:    "invalid user ID",
			userID:  0,
			orgID:   1,
			roles:   []string{"admin"},
			wantErr: true,
		},
		{
			name:    "invalid org ID",
			userID:  1,
			orgID:   0,
			roles:   []string{"admin"},
			wantErr: true,
		},
		{
			name:    "empty roles",
			userID:  1,
			orgID:   1,
			roles:   []string{},
			wantErr: true,
		},
		{
			name:    "nil roles",
			userID:  1,
			orgID:   1,
			roles:   nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := manager.GenerateToken(tt.userID, tt.orgID, tt.roles)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && token == "" {
				t.Error("GenerateToken() returned empty token")
			}
		})
	}
}

func TestJWTManager_ValidateToken(t *testing.T) {
	secret := "test-secret-key-that-is-long-enough-for-testing"
	issuer := "test-issuer"
	audience := "test-audience"
	expiry := time.Hour

	manager := NewJWTManager(secret, issuer, audience, expiry)

	// Generate a valid token
	validToken, err := manager.GenerateToken(1, 1, []string{"admin"})
	if err != nil {
		t.Fatalf("Failed to generate valid token: %v", err)
	}

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "valid token",
			token:   validToken,
			wantErr: false,
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
		{
			name:    "malformed token",
			token:   "invalid.token",
			wantErr: true,
		},
		{
			name:    "token with wrong secret",
			token:   "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := manager.ValidateToken(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && claims == nil {
				t.Error("ValidateToken() returned nil claims for valid token")
			}
		})
	}
}

func TestClaims_HasRole(t *testing.T) {
	claims := &Claims{
		UserID: 1,
		OrgID:  1,
		Roles:  []string{"admin", "user"},
	}

	tests := []struct {
		name          string
		requiredRoles []string
		want          bool
	}{
		{
			name:          "has admin role",
			requiredRoles: []string{"admin"},
			want:          true,
		},
		{
			name:          "has user role",
			requiredRoles: []string{"user"},
			want:          true,
		},
		{
			name:          "has any of multiple roles",
			requiredRoles: []string{"admin", "moderator"},
			want:          true,
		},
		{
			name:          "does not have role",
			requiredRoles: []string{"moderator"},
			want:          false,
		},
		{
			name:          "empty required roles",
			requiredRoles: []string{},
			want:          false,
		},
		{
			name:          "nil required roles",
			requiredRoles: nil,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := claims.HasRole(tt.requiredRoles...); got != tt.want {
				t.Errorf("HasRole() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaims_IsExpiringSoon(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		expiresAt *jwt.NumericDate
		duration  time.Duration
		want      bool
	}{
		{
			name:      "expires soon",
			expiresAt: jwt.NewNumericDate(now.Add(30 * time.Minute)),
			duration:  time.Hour,
			want:      true,
		},
		{
			name:      "expires later",
			expiresAt: jwt.NewNumericDate(now.Add(2 * time.Hour)),
			duration:  time.Hour,
			want:      false,
		},
		{
			name:      "already expired",
			expiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
			duration:  time.Hour,
			want:      true, // Already expired tokens are considered "expiring soon"
		},
		{
			name:      "nil expires at",
			expiresAt: nil,
			duration:  time.Hour,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &Claims{
				UserID: 1,
				OrgID:  1,
				Roles:  []string{"admin"},
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: tt.expiresAt,
				},
			}
			if got := claims.IsExpiringSoon(tt.duration); got != tt.want {
				t.Errorf("IsExpiringSoon() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContextFunctions(t *testing.T) {
	ctx := context.Background()

	// Test with no values
	if UserIDFromContext(ctx) != 0 {
		t.Error("Expected UserIDFromContext to return 0 for empty context")
	}
	if OrgIDFromContext(ctx) != 0 {
		t.Error("Expected OrgIDFromContext to return 0 for empty context")
	}
	if RolesFromContext(ctx) != nil {
		t.Error("Expected RolesFromContext to return nil for empty context")
	}
	if ClaimsFromContext(ctx) != nil {
		t.Error("Expected ClaimsFromContext to return nil for empty context")
	}

	// Test with values
	claims := &Claims{
		UserID: 123,
		OrgID:  456,
		Roles:  []string{"admin"},
	}

	ctx = context.WithValue(ctx, UserIDKey, int64(123))
	ctx = context.WithValue(ctx, OrgIDKey, int64(456))
	ctx = context.WithValue(ctx, RolesKey, []string{"admin"})
	ctx = context.WithValue(ctx, ClaimsKey, claims)

	if UserIDFromContext(ctx) != 123 {
		t.Errorf("Expected UserIDFromContext to return 123, got %d", UserIDFromContext(ctx))
	}
	if OrgIDFromContext(ctx) != 456 {
		t.Errorf("Expected OrgIDFromContext to return 456, got %d", OrgIDFromContext(ctx))
	}

	roles := RolesFromContext(ctx)
	if len(roles) != 1 || roles[0] != "admin" {
		t.Errorf("Expected RolesFromContext to return [admin], got %v", roles)
	}

	ctxClaims := ClaimsFromContext(ctx)
	if ctxClaims != claims {
		t.Error("Expected ClaimsFromContext to return the same claims")
	}
}

func TestPublicPaths(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/health", true},
		{"/dbping", true},
		{"/items", false},
		{"/sites", false},
		{"/vendors", false},
		{"/projects", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isPublicPath(tt.path); got != tt.want {
				t.Errorf("isPublicPath(%s) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestValidateTokenFormat(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "valid JWT format",
			token:   "header.payload.signature",
			wantErr: false,
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
		{
			name:    "too many parts",
			token:   "header.payload.signature.extra",
			wantErr: true,
		},
		{
			name:    "too few parts",
			token:   "header.payload",
			wantErr: true,
		},
		{
			name:    "token too long",
			token:   strings.Repeat("a", 9000),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTokenFormat(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTokenFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuthMiddleware_InvalidTokenFormat(t *testing.T) {
	secret := "test-secret-key-that-is-long-enough-for-testing"
	issuer := "test-issuer"
	audience := "test-audience"
	expiry := time.Hour

	manager := NewJWTManager(secret, issuer, audience, expiry)
	middleware := AuthMiddleware(manager)

	req := httptest.NewRequest("GET", "/items", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.format")
	w := httptest.NewRecorder()
	
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called when auth fails")
	}))
	
	handler.ServeHTTP(w, req)
	
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status Unauthorized, got %d", w.Code)
	}
	
	var errorResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errorResp); err != nil {
		t.Errorf("Failed to decode error response: %v", err)
	}
	
	// The actual error code depends on the JWT validation, so we'll check for any error
	if errorResp.Code == "" {
		t.Error("Expected error code to be set")
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	secret := "test-secret-key-that-is-long-enough-for-testing"
	issuer := "test-issuer"
	audience := "test-audience"
	expiry := time.Hour

	manager := NewJWTManager(secret, issuer, audience, expiry)
	middleware := AuthMiddleware(manager)

	// Generate a valid token
	token, err := manager.GenerateToken(1, 1, []string{"admin"})
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req := httptest.NewRequest("GET", "/items", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	
	handlerCalled := false
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		
		// Check that context values are set
		userID := UserIDFromContext(r.Context())
		if userID != 1 {
			t.Errorf("Expected UserID 1, got %d", userID)
		}
		
		orgID := OrgIDFromContext(r.Context())
		if orgID != 1 {
			t.Errorf("Expected OrgID 1, got %d", orgID)
		}
		
		roles := RolesFromContext(r.Context())
		if len(roles) != 1 || roles[0] != "admin" {
			t.Errorf("Expected roles [admin], got %v", roles)
		}
		
		w.WriteHeader(http.StatusOK)
	}))
	
	handler.ServeHTTP(w, req)
	
	if !handlerCalled {
		t.Error("Handler should be called with valid token")
	}
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", w.Code)
	}
}

func TestMustRole_SufficientPermissions(t *testing.T) {
	middleware := MustRole("admin")
	
	req := httptest.NewRequest("GET", "/items", nil)
	// Set up context with claims that have required role
	ctx := context.WithValue(req.Context(), ClaimsKey, &Claims{
		UserID: 1,
		OrgID:  1,
		Roles:  []string{"admin", "user"},
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	
	handlerCalled := false
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	
	handler.ServeHTTP(w, req)
	
	if !handlerCalled {
		t.Error("Handler should be called when user has required role")
	}
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %d", w.Code)
	}
}

func TestSendErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()
	
	sendErrorResponse(w, "Test error", "TEST_ERROR", http.StatusBadRequest)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status BadRequest, got %d", w.Code)
	}
	
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}
	
	var errorResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errorResp); err != nil {
		t.Errorf("Failed to decode error response: %v", err)
	}
	
	if errorResp.Error != "Test error" {
		t.Errorf("Expected error message 'Test error', got %s", errorResp.Error)
	}
	
	if errorResp.Code != "TEST_ERROR" {
		t.Errorf("Expected error code 'TEST_ERROR', got %s", errorResp.Code)
	}
}
