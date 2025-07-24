package auth

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
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

// Validate validates the basic credentials with security checks
func (c *BasicCredentials) Validate() error {
	if c.Username == "" {
		return errors.New("username is required")
	}
	if c.Password == "" {
		return errors.New("password is required")
	}
	
	// Validate username length and characters
	if len(c.Username) < 3 || len(c.Username) > 64 {
		return errors.New("username must be between 3 and 64 characters")
	}
	
	// Check for valid username characters (alphanumeric, underscore, hyphen, dot)
	if !isValidUsername(c.Username) {
		return errors.New("username contains invalid characters")
	}
	
	// Validate password strength
	if len(c.Password) < 8 || len(c.Password) > 128 {
		return errors.New("password must be between 8 and 128 characters")
	}
	
	// Check for null bytes and control characters
	if strings.ContainsRune(c.Username, '\x00') || strings.ContainsRune(c.Password, '\x00') {
		return errors.New("credentials cannot contain null bytes")
	}
	
	// Check for basic password complexity
	if !isPasswordComplex(c.Password) {
		return errors.New("password must contain at least one uppercase letter, one lowercase letter, one digit, and one special character")
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

// Validate validates the token credentials with security checks
func (c *TokenCredentials) Validate() error {
	if c.Token == "" {
		return errors.New("token is required")
	}
	if c.TokenType == "" {
		c.TokenType = "Bearer" // Default to Bearer
	}
	
	// Validate token length
	if len(c.Token) < 32 || len(c.Token) > 2048 {
		return errors.New("token must be between 32 and 2048 characters")
	}
	
	// Check for null bytes and control characters
	if strings.ContainsRune(c.Token, '\x00') {
		return errors.New("token cannot contain null bytes")
	}
	
	// Check for valid token format (base64-like or hex)
	if !isValidToken(c.Token) {
		return errors.New("token contains invalid characters")
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

// Validate validates the API key credentials with security checks
func (c *APIKeyCredentials) Validate() error {
	if c.APIKey == "" {
		return errors.New("API key is required")
	}
	
	// Validate API key length
	if len(c.APIKey) < 32 || len(c.APIKey) > 256 {
		return errors.New("API key must be between 32 and 256 characters")
	}
	
	// Check for null bytes and control characters
	if strings.ContainsRune(c.APIKey, '\x00') {
		return errors.New("API key cannot contain null bytes")
	}
	
	// Check for valid API key format
	if !isValidAPIKey(c.APIKey) {
		return errors.New("API key contains invalid characters")
	}
	
	// Validate secret if present
	if c.Secret != "" {
		if len(c.Secret) < 16 || len(c.Secret) > 128 {
			return errors.New("API key secret must be between 16 and 128 characters")
		}
		
		if strings.ContainsRune(c.Secret, '\x00') {
			return errors.New("API key secret cannot contain null bytes")
		}
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
	
	// AutoRotateTokens enables automatic token rotation
	AutoRotateTokens bool `json:"auto_rotate_tokens"`
	
	// TokenRotationInterval specifies how often tokens should be rotated
	TokenRotationInterval time.Duration `json:"token_rotation_interval"`
	
	// EnableAuditLogging enables audit logging for authentication events
	EnableAuditLogging bool `json:"enable_audit_logging"`
	
	// AuditLogRetention specifies how long audit logs are retained
	AuditLogRetention time.Duration `json:"audit_log_retention"`
	
	// ProviderConfig contains provider-specific configuration
	ProviderConfig map[string]interface{} `json:"provider_config,omitempty"`
}

// AuditLogger interface for audit logging
type AuditLogger interface {
	LogEvent(event *AuditEvent) error
	GetEvents(filter *AuditEventFilter) ([]*AuditEvent, error)
	CleanupOldEvents(before time.Time) error
}

// AuditEvent represents an authentication audit event
type AuditEvent struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	EventType   AuditEventType         `json:"event_type"`
	UserID      string                 `json:"user_id,omitempty"`
	Username    string                 `json:"username,omitempty"`
	Result      string                 `json:"result"` // SUCCESS, FAILURE
	Error       string                 `json:"error,omitempty"`
	TokenID     string                 `json:"token_id,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AuditEventType represents the type of audit event
type AuditEventType string

const (
	AuditEventLogin            AuditEventType = "login"
	AuditEventLogout           AuditEventType = "logout"
	AuditEventAuthFailure      AuditEventType = "auth_failure"
	AuditEventTokenRefresh     AuditEventType = "token_refresh"
	AuditEventTokenRotation    AuditEventType = "token_rotation"
	AuditEventPermissionDenied AuditEventType = "permission_denied"
)

// AuditEventFilter filters audit events
type AuditEventFilter struct {
	EventTypes []AuditEventType `json:"event_types,omitempty"`
	UserID     string           `json:"user_id,omitempty"`
	Username   string           `json:"username,omitempty"`
	Result     string           `json:"result,omitempty"`
	StartTime  *time.Time       `json:"start_time,omitempty"`
	EndTime    *time.Time       `json:"end_time,omitempty"`
	Limit      int              `json:"limit,omitempty"`
	Offset     int              `json:"offset,omitempty"`
}

// TokenRotationInfo tracks token rotation history
type TokenRotationInfo struct {
	OldTokenID    string    `json:"old_token_id"`
	NewTokenID    string    `json:"new_token_id"`
	RotatedAt     time.Time `json:"rotated_at"`
	NextRotation  time.Time `json:"next_rotation"`
	RotationCount int       `json:"rotation_count"`
}

// DefaultAuthConfig returns the default authentication configuration
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		Enabled:               true,
		RequireAuth:           false,
		AllowAnonymous:        true,
		TokenExpiration:       24 * time.Hour,
		RefreshEnabled:        true,
		RefreshExpiration:     7 * 24 * time.Hour,
		AutoRotateTokens:      true,
		TokenRotationInterval: 12 * time.Hour, // Rotate tokens every 12 hours
		EnableAuditLogging:    true,
		AuditLogRetention:     30 * 24 * time.Hour, // 30 days
		ProviderConfig:        make(map[string]interface{}),
	}
}

// Input validation helper functions

// isValidUsername checks if a username contains only allowed characters
func isValidUsername(username string) bool {
	// Allow alphanumeric, underscore, hyphen, and dot
	validUsernameRegex := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	return validUsernameRegex.MatchString(username)
}

// isPasswordComplex checks if a password meets complexity requirements
func isPasswordComplex(password string) bool {
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}
	
	return hasUpper && hasLower && hasDigit && hasSpecial
}

// isValidToken checks if a token contains only valid characters
func isValidToken(token string) bool {
	// Allow base64-like characters and hex characters
	validTokenRegex := regexp.MustCompile(`^[A-Za-z0-9+/=._-]+$`)
	return validTokenRegex.MatchString(token)
}

// isValidAPIKey checks if an API key contains only valid characters
func isValidAPIKey(apiKey string) bool {
	// Allow alphanumeric and common API key characters
	validAPIKeyRegex := regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	return validAPIKeyRegex.MatchString(apiKey)
}