package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"era-inventory-api/internal/auth"
	"era-inventory-api/internal/config"
)

func main() {
	var (
		userID     = flag.Int64("user", 1, "User ID")
		orgID      = flag.Int64("org", 1, "Organization ID")
		roles      = flag.String("roles", "org_admin", "Comma-separated list of roles")
		expiryMins = flag.Int("expiry", 1440, "Token expiry in minutes (default: 24 hours)")
		secret     = flag.String("secret", "", "JWT secret (overrides JWT_SECRET env var)")
		issuer     = flag.String("issuer", "", "JWT issuer (overrides JWT_ISS env var)")
		audience   = flag.String("audience", "", "JWT audience (overrides JWT_AUD env var)")
	)
	flag.Parse()

	// Load config
	cfg := config.Load()

	// Override with command line flags if provided
	if *secret != "" {
		cfg.JWTSecret = *secret
	}
	if *issuer != "" {
		cfg.JWTIssuer = *issuer
	}
	if *audience != "" {
		cfg.JWTAudience = *audience
	}

	// Parse roles
	roleList := strings.Split(*roles, ",")
	for i, role := range roleList {
		roleList[i] = strings.TrimSpace(role)
	}

	// Create JWT manager
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, time.Duration(*expiryMins)*time.Minute)

	// Generate token
	token, err := jwtManager.GenerateToken(*userID, *orgID, roleList)
	if err != nil {
		log.Fatalf("Failed to generate token: %v", err)
	}

	// Print token info
	fmt.Printf("JWT Token generated successfully!\n\n")
	fmt.Printf("User ID: %d\n", *userID)
	fmt.Printf("Org ID: %d\n", *orgID)
	fmt.Printf("Roles: %s\n", strings.Join(roleList, ", "))
	fmt.Printf("Expiry: %d minutes\n", *expiryMins)
	fmt.Printf("Issuer: %s\n", cfg.JWTIssuer)
	fmt.Printf("Audience: %s\n", cfg.JWTAudience)
	fmt.Printf("\nToken:\n%s\n\n", token)

	// Print usage example
	fmt.Printf("Usage example:\n")
	fmt.Printf("curl -H \"Authorization: Bearer %s\" http://localhost:8080/items\n", token)
}
