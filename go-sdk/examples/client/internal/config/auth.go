package config

import (
	"fmt"
	"strings"
)

// AuthConfig represents the authentication configuration
type AuthConfig struct {
	APIKey     string
	AuthHeader string
	AuthScheme string
}

// CredentialResolver resolves authentication credentials with proper precedence
type CredentialResolver struct {
	manager *Manager
}

// NewCredentialResolver creates a new credential resolver
func NewCredentialResolver(manager *Manager) *CredentialResolver {
	return &CredentialResolver{
		manager: manager,
	}
}

// Resolve returns the resolved authentication configuration
// Precedence: flags > env > config file > defaults
func (cr *CredentialResolver) Resolve() (*AuthConfig, error) {
	cfg := cr.manager.GetConfig()
	
	// Validate auth header value if custom
	if cfg.AuthHeader != "" && cfg.AuthHeader != "Authorization" && cfg.AuthHeader != "X-API-Key" {
		// Allow any header name but warn about non-standard ones
		if !strings.HasPrefix(cfg.AuthHeader, "X-") && cfg.AuthHeader != "Authorization" {
			// This is just a warning, not an error
			fmt.Printf("Warning: Using non-standard auth header: %s\n", cfg.AuthHeader)
		}
	}
	
	return &AuthConfig{
		APIKey:     cfg.APIKey,
		AuthHeader: cfg.AuthHeader,
		AuthScheme: cfg.AuthScheme,
	}, nil
}

// GetAuthorizationHeader returns the formatted authorization header value
func (ac *AuthConfig) GetAuthorizationHeader() (string, string) {
	if ac.APIKey == "" {
		return "", ""
	}
	
	header := ac.AuthHeader
	if header == "" {
		header = "Authorization"
	}
	
	value := ac.APIKey
	if header == "Authorization" && ac.AuthScheme != "" {
		value = ac.AuthScheme + " " + ac.APIKey
	}
	
	return header, value
}

// RedactAPIKey returns a redacted version of the API key for logging
func RedactAPIKey(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	if len(apiKey) <= 8 {
		return "***"
	}
	// Show last 4 chars as per requirement
	return "***" + apiKey[len(apiKey)-4:]
}

// ValidateAuthConfig validates the authentication configuration
func ValidateAuthConfig(config *AuthConfig) error {
	// Validate AuthHeader if specified
	if config.AuthHeader != "" {
		// Check for common invalid header names
		if strings.ContainsAny(config.AuthHeader, " \t\n\r") {
			return fmt.Errorf("invalid auth header name: contains whitespace")
		}
		if strings.Contains(config.AuthHeader, ":") {
			return fmt.Errorf("invalid auth header name: contains colon")
		}
	}
	
	// Validate AuthScheme when using Authorization header
	if config.AuthHeader == "Authorization" || config.AuthHeader == "" {
		if config.AuthScheme == "" && config.APIKey != "" {
			// Default to Bearer if not specified
			config.AuthScheme = "Bearer"
		}
	}
	
	return nil
}