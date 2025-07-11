package auth

import (
	"context"
	"errors"
	"time"
)

// Common authentication errors
var (
	ErrNoAuthProvider       = errors.New("no authentication provider configured")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrTokenExpired         = errors.New("authentication token expired")
	ErrInsufficientPermissions = errors.New("insufficient permissions")
)

// AuthProvider defines the interface for authentication providers
// This interface can be implemented by various authentication backends like JWT, OAuth, RBAC, etc.
type AuthProvider interface {
	// Authenticate validates credentials and returns an authentication context
	Authenticate(ctx context.Context, credentials Credentials) (*AuthContext, error)
	
	// Authorize checks if the authenticated context has permission for a specific action
	Authorize(ctx context.Context, authCtx *AuthContext, resource string, action string) error
	
	// Refresh refreshes the authentication context (e.g., refresh tokens)
	Refresh(ctx context.Context, authCtx *AuthContext) (*AuthContext, error)
	
	// Revoke revokes the authentication context
	Revoke(ctx context.Context, authCtx *AuthContext) error
	
	// ValidateContext validates if an authentication context is still valid
	ValidateContext(ctx context.Context, authCtx *AuthContext) error
	
	// GetProviderType returns the type of authentication provider
	GetProviderType() string
}

// Credentials represents authentication credentials
type Credentials interface {
	// GetType returns the type of credentials (e.g., "basic", "token", "api_key")
	GetType() string
	
	// Validate performs basic validation of the credentials
	Validate() error
}

// BasicCredentials represents username/password credentials
type BasicCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// GetType returns the credential type
func (c *BasicCredentials) GetType() string {
	return "basic"
}

// Validate validates the basic credentials
func (c *BasicCredentials) Validate() error {
	if c.Username == "" {
		return errors.New("username is required")
	}
	if c.Password == "" {
		return errors.New("password is required")
	}
	return nil
}

// TokenCredentials represents token-based credentials
type TokenCredentials struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"` // e.g., "Bearer", "API"
}

// GetType returns the credential type
func (c *TokenCredentials) GetType() string {
	return "token"
}

// Validate validates the token credentials
func (c *TokenCredentials) Validate() error {
	if c.Token == "" {
		return errors.New("token is required")
	}
	if c.TokenType == "" {
		c.TokenType = "Bearer" // Default to Bearer
	}
	return nil
}

// APIKeyCredentials represents API key credentials
type APIKeyCredentials struct {
	APIKey string `json:"api_key"`
	Secret string `json:"secret,omitempty"`
}

// GetType returns the credential type
func (c *APIKeyCredentials) GetType() string {
	return "api_key"
}

// Validate validates the API key credentials
func (c *APIKeyCredentials) Validate() error {
	if c.APIKey == "" {
		return errors.New("API key is required")
	}
	return nil
}

// AuthContext represents an authenticated session context
type AuthContext struct {
	// UserID is the unique identifier of the authenticated user
	UserID string `json:"user_id"`
	
	// Username is the human-readable username
	Username string `json:"username"`
	
	// Roles contains the roles assigned to the user
	Roles []string `json:"roles"`
	
	// Permissions contains the specific permissions granted
	Permissions []string `json:"permissions"`
	
	// Token is the authentication token (if applicable)
	Token string `json:"token,omitempty"`
	
	// ExpiresAt is when the authentication expires
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	
	// IssuedAt is when the authentication was issued
	IssuedAt time.Time `json:"issued_at"`
	
	// Metadata contains additional provider-specific data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	
	// ProviderType indicates which provider authenticated this context
	ProviderType string `json:"provider_type"`
}

// IsExpired checks if the authentication context has expired
func (ac *AuthContext) IsExpired() bool {
	if ac.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*ac.ExpiresAt)
}

// HasRole checks if the user has a specific role
func (ac *AuthContext) HasRole(role string) bool {
	for _, r := range ac.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasPermission checks if the user has a specific permission
func (ac *AuthContext) HasPermission(permission string) bool {
	for _, p := range ac.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the user has any of the specified roles
func (ac *AuthContext) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if ac.HasRole(role) {
			return true
		}
	}
	return false
}

// HasAllRoles checks if the user has all of the specified roles
func (ac *AuthContext) HasAllRoles(roles ...string) bool {
	for _, role := range roles {
		if !ac.HasRole(role) {
			return false
		}
	}
	return true
}

// AuthConfig represents configuration for authentication
type AuthConfig struct {
	// Enabled determines if authentication is enabled
	Enabled bool `json:"enabled"`
	
	// RequireAuth determines if authentication is required for all operations
	RequireAuth bool `json:"require_auth"`
	
	// AllowAnonymous allows anonymous access for certain operations
	AllowAnonymous bool `json:"allow_anonymous"`
	
	// TokenExpiration is the default token expiration duration
	TokenExpiration time.Duration `json:"token_expiration"`
	
	// RefreshEnabled allows token refresh
	RefreshEnabled bool `json:"refresh_enabled"`
	
	// RefreshExpiration is the refresh token expiration duration
	RefreshExpiration time.Duration `json:"refresh_expiration"`
	
	// ProviderConfig contains provider-specific configuration
	ProviderConfig map[string]interface{} `json:"provider_config,omitempty"`
}

// DefaultAuthConfig returns the default authentication configuration
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		Enabled:           true,
		RequireAuth:       false,
		AllowAnonymous:    true,
		TokenExpiration:   24 * time.Hour,
		RefreshEnabled:    true,
		RefreshExpiration: 7 * 24 * time.Hour,
		ProviderConfig:    make(map[string]interface{}),
	}
}