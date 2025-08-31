package auth

import (
	"errors"
	"fmt"
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

// NewJWTManager creates a new JWT manager
func NewJWTManager(secret, issuer, audience string, expiry time.Duration) *JWTManager {
	return &JWTManager{
		secret:   secret,
		issuer:   issuer,
		audience: audience,
		expiry:   expiry,
	}
}

// GenerateToken creates a new JWT token
func (j *JWTManager) GenerateToken(userID, orgID int64, roles []string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID: userID,
		OrgID:  orgID,
		Roles:  roles,
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
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// HasRole checks if the user has any of the required roles
func (c *Claims) HasRole(requiredRoles ...string) bool {
	for _, required := range requiredRoles {
		for _, userRole := range c.Roles {
			if userRole == required {
				return true
			}
		}
	}
	return false
}
