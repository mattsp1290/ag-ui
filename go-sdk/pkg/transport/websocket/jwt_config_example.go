package websocket

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"
)

// JWTConfigExample shows how to configure JWT authentication for WebSocket transport
type JWTConfigExample struct{}

// ExampleHMACConfig demonstrates configuring JWT validation with HMAC (symmetric key)
func ExampleHMACConfig() (*SecurityConfig, error) {
	// Get secret key from environment variable
	secretKey := os.Getenv("JWT_SECRET_KEY")
	if secretKey == "" {
		return nil, fmt.Errorf("JWT_SECRET_KEY environment variable not set")
	}

	// Create HMAC JWT validator
	validator := NewJWTTokenValidator([]byte(secretKey), "your-issuer")

	// Create security config
	securityConfig := DefaultSecurityConfig()
	securityConfig.TokenValidator = validator
	securityConfig.RequireAuth = true

	return securityConfig, nil
}

// ExampleRSAConfig demonstrates configuring JWT validation with RSA (asymmetric key)
func ExampleRSAConfig() (*SecurityConfig, error) {
	// Read public key from file or environment
	publicKeyPEM := os.Getenv("JWT_PUBLIC_KEY")
	if publicKeyPEM == "" {
		// Try reading from file
		data, err := os.ReadFile("jwt_public_key.pem")
		if err != nil {
			return nil, fmt.Errorf("failed to read public key: %w", err)
		}
		publicKeyPEM = string(data)
	}

	// Parse RSA public key
	publicKey, err := parseRSAPublicKey(publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA public key: %w", err)
	}

	// Create RSA JWT validator
	validator := NewJWTTokenValidatorRSA(publicKey, "your-issuer", "your-audience")

	// Create security config
	securityConfig := DefaultSecurityConfig()
	securityConfig.TokenValidator = validator
	securityConfig.RequireAuth = true

	return securityConfig, nil
}

// ExampleFullConfig demonstrates a complete WebSocket transport configuration with JWT
func ExampleFullConfig() (*TransportConfig, error) {
	// Get JWT configuration
	securityConfig, err := ExampleHMACConfig()
	if err != nil {
		return nil, err
	}

	// Configure additional security settings
	securityConfig.AllowedOrigins = []string{
		"https://yourdomain.com",
		"https://*.yourdomain.com",
	}
	securityConfig.RequireTLS = true
	securityConfig.MaxMessageSize = 1024 * 1024 // 1MB
	securityConfig.ClientRateLimit = 100        // 100 requests per second per client

	// Create transport config
	config := &TransportConfig{
		URLs:           []string{"wss://api.yourdomain.com/ws"},
		SecurityConfig: securityConfig,
		EventTimeout:   30 * time.Second,
		MaxEventSize:   1024 * 1024, // 1MB
	}

	return config, nil
}

// parseRSAPublicKey parses a PEM-encoded RSA public key
func parseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// Try parsing as PKCS1
		pub, err = x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPub, nil
}

// ExampleCustomValidation demonstrates custom JWT validation with additional claims
func ExampleCustomValidation() TokenValidator {
	return &customJWTValidator{
		base: NewJWTTokenValidator([]byte("your-secret"), "your-issuer"),
	}
}

type customJWTValidator struct {
	base *JWTTokenValidator
}

func (v *customJWTValidator) ValidateToken(ctx context.Context, token string) (*AuthContext, error) {
	// First do standard validation
	authCtx, err := v.base.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// Add custom validation logic
	// For example, check if user has specific scope
	if scopes, ok := authCtx.Claims["scope"].(string); ok {
		// Parse scopes and validate
		if !containsScope(scopes, "websocket:connect") {
			return nil, fmt.Errorf("missing required scope: websocket:connect")
		}
	}

	// Check custom claims
	if org, ok := authCtx.Claims["org_id"].(string); ok {
		// Validate organization access
		if !isValidOrganization(org) {
			return nil, fmt.Errorf("invalid organization")
		}
	}

	return authCtx, nil
}

func containsScope(scopes string, required string) bool {
	// Simple scope check - in production, parse properly
	return strings.Contains(scopes, required)
}

func isValidOrganization(orgID string) bool {
	// Implement organization validation logic
	return orgID != ""
}

// Environment variables for JWT configuration:
//
// JWT_SECRET_KEY     - HMAC secret key for JWT signing
// JWT_PUBLIC_KEY     - RSA public key for JWT verification (PEM format)
// JWT_ISSUER         - Expected JWT issuer
// JWT_AUDIENCE       - Expected JWT audience
// JWT_ALGORITHM      - JWT signing algorithm (HS256, RS256, etc.)
//
// Example environment setup:
//   export JWT_SECRET_KEY="your-256-bit-secret"
//   export JWT_ISSUER="https://auth.yourdomain.com"
//   export JWT_AUDIENCE="websocket-api"
//   export JWT_ALGORITHM="HS256"
