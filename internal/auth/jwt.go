package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims structure
type Claims struct {
	UserID int64    `json:"sub"`
	OrgID  int64    `json:"org_id"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

// JWTManager handles JWT operations
type JWTManager struct {
	secret   string
	issuer   string
	audience string
	expiry   time.Duration
}

// JWT validation errors
var (
	ErrInvalidSigningMethod = errors.New("invalid signing method")
	ErrTokenExpired         = errors.New("token expired")
	ErrTokenNotValidYet     = errors.New("token not valid yet")
	ErrTokenMalformed       = errors.New("token malformed")
	ErrInvalidClaims        = errors.New("invalid claims")
	ErrEmptySecret          = errors.New("JWT secret cannot be empty")
	ErrSecretTooShort       = errors.New("JWT secret must be at least 32 characters")
)

// NewJWTManager creates a new JWT manager
func NewJWTManager(secret, issuer, audience string, expiry time.Duration) *JWTManager {
	return &JWTManager{
		secret:   secret,
		issuer:   issuer,
		audience: audience,
		expiry:   expiry,
	}
}

// ValidateConfig validates the JWT configuration
func (j *JWTManager) ValidateConfig() error {
	if j.secret == "" {
		return ErrEmptySecret
	}
	if len(j.secret) < 32 {
		return ErrSecretTooShort
	}
	if j.issuer == "" {
		return errors.New("JWT issuer cannot be empty")
	}
	if j.audience == "" {
		return errors.New("JWT audience cannot be empty")
	}
	if j.expiry <= 0 {
		return errors.New("JWT expiry must be positive")
	}
	return nil
}

// GenerateToken creates a new JWT token
func (j *JWTManager) GenerateToken(userID, orgID int64, roles []string) (string, error) {
	// Validate configuration
	if err := j.ValidateConfig(); err != nil {
		return "", fmt.Errorf("invalid JWT configuration: %w", err)
	}

	// Validate input parameters
	if userID <= 0 {
		return "", errors.New("user ID must be positive")
	}
	if orgID <= 0 {
		return "", errors.New("organization ID must be positive")
	}
	if len(roles) == 0 {
		return "", errors.New("at least one role is required")
	}

	// Sanitize roles
	sanitizedRoles := make([]string, 0, len(roles))
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role != "" && len(role) <= 50 {
			sanitizedRoles = append(sanitizedRoles, role)
		}
	}
	if len(sanitizedRoles) == 0 {
		return "", errors.New("no valid roles provided")
	}

	now := time.Now()
	claims := &Claims{
		UserID: userID,
		OrgID:  orgID,
		Roles:  sanitizedRoles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(j.expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    j.issuer,
			Audience:  []string{j.audience},
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.secret))
}

// ValidateToken validates and parses a JWT token
func (j *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	// Validate configuration
	if err := j.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid JWT configuration: %w", err)
	}

	// Parse token with custom validation
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: %v", ErrInvalidSigningMethod, token.Header["alg"])
		}

		// Validate algorithm specifically
		if alg, ok := token.Header["alg"].(string); ok && alg != "HS256" {
			return nil, fmt.Errorf("%w: only HS256 is supported, got %s", ErrInvalidSigningMethod, alg)
		}

		return []byte(j.secret), nil
	})

	if err != nil {
		// Map JWT errors to our custom errors based on error message
		errStr := err.Error()
		if strings.Contains(errStr, "expired") {
			return nil, ErrTokenExpired
		}
		if strings.Contains(errStr, "not valid yet") {
			return nil, ErrTokenNotValidYet
		}
		if strings.Contains(errStr, "malformed") {
			return nil, ErrTokenMalformed
		}
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	// Extract and validate claims
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}

	// Additional claims validation
	if err := j.validateClaims(claims); err != nil {
		return nil, fmt.Errorf("claims validation failed: %w", err)
	}

	return claims, nil
}

// validateClaims performs additional validation on JWT claims
func (j *JWTManager) validateClaims(claims *Claims) error {
	if claims.UserID <= 0 {
		return errors.New("invalid user ID in claims")
	}
	if claims.OrgID <= 0 {
		return errors.New("invalid organization ID in claims")
	}
	if len(claims.Roles) == 0 {
		return errors.New("no roles in claims")
	}
	if claims.Issuer != j.issuer {
		return fmt.Errorf("invalid issuer: expected %s, got %s", j.issuer, claims.Issuer)
	}
	if len(claims.Audience) == 0 || claims.Audience[0] != j.audience {
		return fmt.Errorf("invalid audience: expected %s, got %v", j.audience, claims.Audience)
	}
	return nil
}

// HasRole checks if the user has any of the required roles
func (c *Claims) HasRole(requiredRoles ...string) bool {
	for _, required := range requiredRoles {
		required = strings.TrimSpace(required)
		if required == "" {
			continue
		}
		for _, userRole := range c.Roles {
			if userRole == required {
				return true
			}
		}
	}
	return false
}

// IsExpiringSoon checks if the token expires within the given duration
func (c *Claims) IsExpiringSoon(duration time.Duration) bool {
	if c.ExpiresAt == nil {
		return false
	}
	return time.Until(c.ExpiresAt.Time) <= duration
}
