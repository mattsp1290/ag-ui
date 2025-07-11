package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// BasicAuthProvider provides a simple in-memory authentication implementation
// This can be used for testing or as a foundation for more complex providers
type BasicAuthProvider struct {
	config    *AuthConfig
	users     map[string]*User
	sessions  map[string]*AuthContext
	revokedTokens map[string]time.Time
	mutex     sync.RWMutex
}

// User represents a user in the basic auth provider
type User struct {
	ID          string
	Username    string
	PasswordHash string
	Roles       []string
	Permissions []string
	Metadata    map[string]interface{}
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewBasicAuthProvider creates a new basic authentication provider
func NewBasicAuthProvider(config *AuthConfig) *BasicAuthProvider {
	if config == nil {
		config = DefaultAuthConfig()
	}
	
	return &BasicAuthProvider{
		config:        config,
		users:         make(map[string]*User),
		sessions:      make(map[string]*AuthContext),
		revokedTokens: make(map[string]time.Time),
	}
}

// AddUser adds a user to the provider
func (p *BasicAuthProvider) AddUser(user *User) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	if user == nil {
		return errors.New("user cannot be nil")
	}
	
	if user.Username == "" {
		return errors.New("username is required")
	}
	
	if _, exists := p.users[user.Username]; exists {
		return fmt.Errorf("user %s already exists", user.Username)
	}
	
	if user.ID == "" {
		user.ID = generateID("user")
	}
	
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	user.UpdatedAt = time.Now()
	
	p.users[user.Username] = user
	return nil
}

// RemoveUser removes a user from the provider
func (p *BasicAuthProvider) RemoveUser(username string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	if _, exists := p.users[username]; !exists {
		return fmt.Errorf("user %s not found", username)
	}
	
	delete(p.users, username)
	
	// Revoke all sessions for this user
	for token, session := range p.sessions {
		if session.Username == username {
			p.revokedTokens[token] = time.Now()
			delete(p.sessions, token)
		}
	}
	
	return nil
}

// SetUserPassword sets a user's password (stores hash)
func (p *BasicAuthProvider) SetUserPassword(username, password string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	user, exists := p.users[username]
	if !exists {
		return fmt.Errorf("user %s not found", username)
	}
	
	user.PasswordHash = hashPassword(password)
	user.UpdatedAt = time.Now()
	
	return nil
}

// Authenticate validates credentials and returns an authentication context
func (p *BasicAuthProvider) Authenticate(ctx context.Context, credentials Credentials) (*AuthContext, error) {
	if !p.config.Enabled {
		return nil, ErrNoAuthProvider
	}
	
	if err := credentials.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}
	
	switch creds := credentials.(type) {
	case *BasicCredentials:
		return p.authenticateBasic(ctx, creds)
	case *TokenCredentials:
		return p.authenticateToken(ctx, creds)
	case *APIKeyCredentials:
		return p.authenticateAPIKey(ctx, creds)
	default:
		return nil, fmt.Errorf("unsupported credential type: %s", credentials.GetType())
	}
}

// authenticateBasic handles username/password authentication
func (p *BasicAuthProvider) authenticateBasic(ctx context.Context, creds *BasicCredentials) (*AuthContext, error) {
	p.mutex.RLock()
	user, exists := p.users[creds.Username]
	p.mutex.RUnlock()
	
	if !exists {
		return nil, ErrInvalidCredentials
	}
	
	if !user.Active {
		return nil, ErrUnauthorized
	}
	
	// Verify password
	if user.PasswordHash != hashPassword(creds.Password) {
		return nil, ErrInvalidCredentials
	}
	
	// Create authentication context
	token := generateToken()
	expiresAt := time.Now().Add(p.config.TokenExpiration)
	
	authCtx := &AuthContext{
		UserID:       user.ID,
		Username:     user.Username,
		Roles:        user.Roles,
		Permissions:  user.Permissions,
		Token:        token,
		ExpiresAt:    &expiresAt,
		IssuedAt:     time.Now(),
		Metadata:     user.Metadata,
		ProviderType: p.GetProviderType(),
	}
	
	// Store session
	p.mutex.Lock()
	p.sessions[token] = authCtx
	p.mutex.Unlock()
	
	return authCtx, nil
}

// authenticateToken handles token-based authentication
func (p *BasicAuthProvider) authenticateToken(ctx context.Context, creds *TokenCredentials) (*AuthContext, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	// Check if token is revoked
	if _, revoked := p.revokedTokens[creds.Token]; revoked {
		return nil, ErrUnauthorized
	}
	
	// Look up session
	authCtx, exists := p.sessions[creds.Token]
	if !exists {
		return nil, ErrInvalidCredentials
	}
	
	// Check expiration
	if authCtx.IsExpired() {
		return nil, ErrTokenExpired
	}
	
	// Return a copy to prevent external modification
	authCtxCopy := *authCtx
	return &authCtxCopy, nil
}

// authenticateAPIKey handles API key authentication
func (p *BasicAuthProvider) authenticateAPIKey(ctx context.Context, creds *APIKeyCredentials) (*AuthContext, error) {
	// For this basic implementation, we'll treat API keys as a special type of user
	// In a real implementation, you would have a separate API key management system
	
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	// Look for a user with matching API key in metadata
	for _, user := range p.users {
		if user.Metadata != nil {
			if apiKey, ok := user.Metadata["api_key"].(string); ok && apiKey == creds.APIKey {
				if !user.Active {
					return nil, ErrUnauthorized
				}
				
				// For API keys, we might not want expiration
				authCtx := &AuthContext{
					UserID:       user.ID,
					Username:     user.Username,
					Roles:        user.Roles,
					Permissions:  user.Permissions,
					Token:        creds.APIKey,
					IssuedAt:     time.Now(),
					Metadata:     user.Metadata,
					ProviderType: p.GetProviderType(),
				}
				
				return authCtx, nil
			}
		}
	}
	
	return nil, ErrInvalidCredentials
}

// Authorize checks if the authenticated context has permission for a specific action
func (p *BasicAuthProvider) Authorize(ctx context.Context, authCtx *AuthContext, resource string, action string) error {
	if !p.config.Enabled {
		return nil // If auth is disabled, allow all actions
	}
	
	if authCtx == nil {
		if p.config.AllowAnonymous {
			// Check if anonymous access is allowed for this resource/action
			if isAnonymousAllowed(resource, action) {
				return nil
			}
		}
		return ErrUnauthorized
	}
	
	// Validate the context is still valid
	if err := p.ValidateContext(ctx, authCtx); err != nil {
		return err
	}
	
	// Build permission string
	permission := fmt.Sprintf("%s:%s", resource, action)
	
	// Check specific permission
	if authCtx.HasPermission(permission) {
		return nil
	}
	
	// Check wildcard permissions
	if authCtx.HasPermission(fmt.Sprintf("%s:*", resource)) {
		return nil
	}
	if authCtx.HasPermission("*:*") {
		return nil
	}
	
	// Check role-based permissions
	if p.checkRolePermissions(authCtx, resource, action) {
		return nil
	}
	
	return ErrInsufficientPermissions
}

// checkRolePermissions checks if any of the user's roles grant the required permission
func (p *BasicAuthProvider) checkRolePermissions(authCtx *AuthContext, resource string, action string) bool {
	// Define some basic role permissions
	rolePermissions := map[string][]string{
		"admin": {"*:*"}, // Admin can do anything
		"validator": {
			"event:validate",
			"event:read",
			"validation:*",
		},
		"reader": {
			"event:read",
			"validation:read",
		},
	}
	
	for _, role := range authCtx.Roles {
		if perms, ok := rolePermissions[role]; ok {
			for _, perm := range perms {
				if matchPermission(perm, resource, action) {
					return true
				}
			}
		}
	}
	
	return false
}

// Refresh refreshes the authentication context
func (p *BasicAuthProvider) Refresh(ctx context.Context, authCtx *AuthContext) (*AuthContext, error) {
	if !p.config.RefreshEnabled {
		return nil, errors.New("refresh is not enabled")
	}
	
	if authCtx == nil {
		return nil, ErrUnauthorized
	}
	
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	// Check if the old token exists and is valid
	oldSession, exists := p.sessions[authCtx.Token]
	if !exists {
		return nil, ErrUnauthorized
	}
	
	// Create new token
	newToken := generateToken()
	expiresAt := time.Now().Add(p.config.TokenExpiration)
	
	// Create new auth context
	newAuthCtx := &AuthContext{
		UserID:       oldSession.UserID,
		Username:     oldSession.Username,
		Roles:        oldSession.Roles,
		Permissions:  oldSession.Permissions,
		Token:        newToken,
		ExpiresAt:    &expiresAt,
		IssuedAt:     time.Now(),
		Metadata:     oldSession.Metadata,
		ProviderType: p.GetProviderType(),
	}
	
	// Store new session
	p.sessions[newToken] = newAuthCtx
	
	// Revoke old token
	p.revokedTokens[authCtx.Token] = time.Now()
	delete(p.sessions, authCtx.Token)
	
	return newAuthCtx, nil
}

// Revoke revokes the authentication context
func (p *BasicAuthProvider) Revoke(ctx context.Context, authCtx *AuthContext) error {
	if authCtx == nil {
		return ErrUnauthorized
	}
	
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	p.revokedTokens[authCtx.Token] = time.Now()
	delete(p.sessions, authCtx.Token)
	
	return nil
}

// ValidateContext validates if an authentication context is still valid
func (p *BasicAuthProvider) ValidateContext(ctx context.Context, authCtx *AuthContext) error {
	if authCtx == nil {
		return ErrUnauthorized
	}
	
	// Check expiration
	if authCtx.IsExpired() {
		return ErrTokenExpired
	}
	
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	// Check if token is revoked
	if _, revoked := p.revokedTokens[authCtx.Token]; revoked {
		return ErrUnauthorized
	}
	
	// Check if session still exists
	if authCtx.Token != "" {
		if _, exists := p.sessions[authCtx.Token]; !exists {
			return ErrUnauthorized
		}
	}
	
	return nil
}

// GetProviderType returns the type of authentication provider
func (p *BasicAuthProvider) GetProviderType() string {
	return "basic"
}

// CleanupExpiredSessions removes expired sessions and old revoked tokens
func (p *BasicAuthProvider) CleanupExpiredSessions() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	now := time.Now()
	
	// Clean up expired sessions
	for token, session := range p.sessions {
		if session.IsExpired() {
			delete(p.sessions, token)
		}
	}
	
	// Clean up old revoked tokens (keep for 24 hours)
	cutoff := now.Add(-24 * time.Hour)
	for token, revokedAt := range p.revokedTokens {
		if revokedAt.Before(cutoff) {
			delete(p.revokedTokens, token)
		}
	}
}

// Helper functions

func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

func generateToken() string {
	return generateID("token")
}

func generateID(prefix string) string {
	timestamp := time.Now().UnixNano()
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", prefix, timestamp)))
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(hash[:])[:16])
}

func isAnonymousAllowed(resource, action string) bool {
	// Define anonymous permissions
	anonymousPermissions := []string{
		"event:read",
		"validation:read",
	}
	
	permission := fmt.Sprintf("%s:%s", resource, action)
	for _, allowed := range anonymousPermissions {
		if allowed == permission {
			return true
		}
	}
	
	return false
}

func matchPermission(pattern, resource, action string) bool {
	parts := strings.Split(pattern, ":")
	if len(parts) != 2 {
		return false
	}
	
	resourcePattern := parts[0]
	actionPattern := parts[1]
	
	// Check resource match
	if resourcePattern != "*" && resourcePattern != resource {
		return false
	}
	
	// Check action match
	if actionPattern != "*" && actionPattern != action {
		return false
	}
	
	return true
}