// Package auth provides comprehensive authentication middleware for the AG-UI system
package auth

import (
	"context"
	"time"
)

// Credentials represents authentication credentials
type Credentials struct {
	Type       string            `json:"type"`
	Token      string            `json:"token,omitempty"`
	Username   string            `json:"username,omitempty"`
	Password   string            `json:"password,omitempty"`
	APIKey     string            `json:"api_key,omitempty"`
	Claims     map[string]any    `json:"claims,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	ExpiresAt  time.Time         `json:"expires_at,omitempty"`
	IssuedAt   time.Time         `json:"issued_at,omitempty"`
	Subject    string            `json:"subject,omitempty"`
	Issuer     string            `json:"issuer,omitempty"`
	Audience   []string          `json:"audience,omitempty"`
	Scopes     []string          `json:"scopes,omitempty"`
}

// AuthContext represents the authentication context
type AuthContext struct {
	UserID      string            `json:"user_id"`
	Username    string            `json:"username"`
	Roles       []string          `json:"roles"`
	Permissions []string          `json:"permissions"`
	Claims      map[string]any    `json:"claims"`
	Metadata    map[string]string `json:"metadata"`
	AuthMethod  string            `json:"auth_method"`
	Timestamp   time.Time         `json:"timestamp"`
	ExpiresAt   time.Time         `json:"expires_at,omitempty"`
}

// AuthProvider defines the interface for authentication providers
type AuthProvider interface {
	// Name returns the provider name
	Name() string

	// Authenticate validates credentials and returns authentication context
	Authenticate(ctx context.Context, credentials *Credentials) (*AuthContext, error)

	// Validate validates an existing authentication context
	Validate(ctx context.Context, authCtx *AuthContext) error

	// Refresh attempts to refresh authentication credentials
	Refresh(ctx context.Context, credentials *Credentials) (*Credentials, error)

	// Revoke revokes authentication credentials
	Revoke(ctx context.Context, credentials *Credentials) error

	// SupportedTypes returns the credential types this provider supports
	SupportedTypes() []string
}

// TokenValidator validates authentication tokens
type TokenValidator interface {
	// ValidateToken validates a token and returns claims
	ValidateToken(ctx context.Context, token string) (*TokenClaims, error)

	// RefreshToken refreshes an existing token
	RefreshToken(ctx context.Context, token string) (string, error)

	// RevokeToken revokes a token
	RevokeToken(ctx context.Context, token string) error
}

// TokenClaims represents token claims
type TokenClaims struct {
	Subject   string         `json:"sub,omitempty"`
	Issuer    string         `json:"iss,omitempty"`
	Audience  []string       `json:"aud,omitempty"`
	ExpiresAt time.Time      `json:"exp,omitempty"`
	IssuedAt  time.Time      `json:"iat,omitempty"`
	NotBefore time.Time      `json:"nbf,omitempty"`
	JTI       string         `json:"jti,omitempty"`
	Custom    map[string]any `json:"custom,omitempty"`
	Scopes    []string       `json:"scopes,omitempty"`
	Roles     []string       `json:"roles,omitempty"`
}

// CredentialExtractor extracts credentials from requests
type CredentialExtractor interface {
	// Extract extracts credentials from request headers, body, or other sources
	Extract(ctx context.Context, headers map[string]string, body any) (*Credentials, error)

	// SupportedMethods returns the authentication methods this extractor supports
	SupportedMethods() []string
}

// PermissionChecker checks user permissions
type PermissionChecker interface {
	// HasPermission checks if the user has a specific permission
	HasPermission(ctx context.Context, authCtx *AuthContext, permission string) bool

	// HasRole checks if the user has a specific role
	HasRole(ctx context.Context, authCtx *AuthContext, role string) bool

	// CheckPermissions checks multiple permissions (AND logic)
	CheckPermissions(ctx context.Context, authCtx *AuthContext, permissions []string) bool

	// CheckRoles checks multiple roles (OR logic)
	CheckRoles(ctx context.Context, authCtx *AuthContext, roles []string) bool
}

// RoleManager manages user roles and permissions
type RoleManager interface {
	// GetRoles returns roles for a user
	GetRoles(ctx context.Context, userID string) ([]string, error)

	// GetPermissions returns permissions for a user
	GetPermissions(ctx context.Context, userID string) ([]string, error)

	// AssignRole assigns a role to a user
	AssignRole(ctx context.Context, userID, role string) error

	// RevokeRole revokes a role from a user
	RevokeRole(ctx context.Context, userID, role string) error
}

// AuditLogger logs authentication events for security auditing
type AuditLogger interface {
	// LogAuthSuccess logs successful authentication
	LogAuthSuccess(ctx context.Context, authCtx *AuthContext, metadata map[string]any)

	// LogAuthFailure logs failed authentication
	LogAuthFailure(ctx context.Context, reason string, metadata map[string]any)

	// LogPermissionDenied logs permission denied events
	LogPermissionDenied(ctx context.Context, authCtx *AuthContext, resource string, action string)

	// LogTokenRefresh logs token refresh events
	LogTokenRefresh(ctx context.Context, authCtx *AuthContext)

	// LogLogout logs logout events
	LogLogout(ctx context.Context, authCtx *AuthContext)
}

// TokenCache provides token caching functionality
type TokenCache interface {
	// Set caches a token
	Set(ctx context.Context, key string, token *Credentials, ttl time.Duration) error

	// Get retrieves a cached token
	Get(ctx context.Context, key string) (*Credentials, error)

	// Delete removes a cached token
	Delete(ctx context.Context, key string) error

	// Exists checks if a token exists in cache
	Exists(ctx context.Context, key string) bool

	// Clear clears all cached tokens
	Clear(ctx context.Context) error
}

// SessionManager manages user sessions
type SessionManager interface {
	// CreateSession creates a new session
	CreateSession(ctx context.Context, authCtx *AuthContext) (string, error)

	// GetSession retrieves session information
	GetSession(ctx context.Context, sessionID string) (*AuthContext, error)

	// UpdateSession updates session information
	UpdateSession(ctx context.Context, sessionID string, authCtx *AuthContext) error

	// DeleteSession deletes a session
	DeleteSession(ctx context.Context, sessionID string) error

	// IsValidSession checks if a session is valid
	IsValidSession(ctx context.Context, sessionID string) bool

	// ExtendSession extends session expiration
	ExtendSession(ctx context.Context, sessionID string, duration time.Duration) error
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	// Provider specific configuration
	JWTConfig      *JWTConfig      `json:"jwt,omitempty" yaml:"jwt,omitempty"`
	APIKeyConfig   *APIKeyConfig   `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	BasicAuthConfig *BasicAuthConfig `json:"basic_auth,omitempty" yaml:"basic_auth,omitempty"`
	OAuth2Config   *OAuth2Config   `json:"oauth2,omitempty" yaml:"oauth2,omitempty"`

	// General settings
	EnableAuditLogging bool          `json:"enable_audit_logging" yaml:"enable_audit_logging"`
	TokenCaching      bool          `json:"token_caching" yaml:"token_caching"`
	SessionTimeout    time.Duration `json:"session_timeout" yaml:"session_timeout"`
	RefreshThreshold  time.Duration `json:"refresh_threshold" yaml:"refresh_threshold"`

	// Security settings
	RequireHTTPS      bool     `json:"require_https" yaml:"require_https"`
	AllowedOrigins    []string `json:"allowed_origins" yaml:"allowed_origins"`
	RequiredScopes    []string `json:"required_scopes" yaml:"required_scopes"`
	RequiredRoles     []string `json:"required_roles" yaml:"required_roles"`
}

// JWTConfig represents JWT authentication configuration
type JWTConfig struct {
	Secret         string        `json:"secret,omitempty" yaml:"secret,omitempty"`
	PublicKey      string        `json:"public_key,omitempty" yaml:"public_key,omitempty"`
	PrivateKey     string        `json:"private_key,omitempty" yaml:"private_key,omitempty"`
	Algorithm      string        `json:"algorithm" yaml:"algorithm"`
	Issuer         string        `json:"issuer" yaml:"issuer"`
	Audience       []string      `json:"audience" yaml:"audience"`
	Expiration     time.Duration `json:"expiration" yaml:"expiration"`
	RefreshWindow  time.Duration `json:"refresh_window" yaml:"refresh_window"`
	ClaimsValidation bool        `json:"claims_validation" yaml:"claims_validation"`
}

// APIKeyConfig represents API key authentication configuration
type APIKeyConfig struct {
	HeaderName         string        `json:"header_name" yaml:"header_name"`
	QueryParam         string        `json:"query_param,omitempty" yaml:"query_param,omitempty"`
	ValidationEndpoint string        `json:"validation_endpoint" yaml:"validation_endpoint"`
	CacheTimeout       time.Duration `json:"cache_timeout" yaml:"cache_timeout"`
	RateLimitPerKey    int           `json:"rate_limit_per_key" yaml:"rate_limit_per_key"`
}

// BasicAuthConfig represents basic authentication configuration
type BasicAuthConfig struct {
	Realm              string `json:"realm" yaml:"realm"`
	UserProvider       string `json:"user_provider" yaml:"user_provider"`
	PasswordValidation string `json:"password_validation" yaml:"password_validation"`
	HashAlgorithm      string `json:"hash_algorithm" yaml:"hash_algorithm"`
}

// OAuth2Config represents OAuth 2.0 authentication configuration
type OAuth2Config struct {
	ClientID         string   `json:"client_id" yaml:"client_id"`
	ClientSecret     string   `json:"client_secret,omitempty" yaml:"client_secret,omitempty"`
	AuthURL          string   `json:"auth_url" yaml:"auth_url"`
	TokenURL         string   `json:"token_url" yaml:"token_url"`
	RedirectURL      string   `json:"redirect_url" yaml:"redirect_url"`
	Scopes           []string `json:"scopes" yaml:"scopes"`
	UserInfoURL      string   `json:"user_info_url,omitempty" yaml:"user_info_url,omitempty"`
	RefreshTokens    bool     `json:"refresh_tokens" yaml:"refresh_tokens"`
}