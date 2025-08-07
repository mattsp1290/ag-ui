package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"
)

// BasicAuthProvider implements basic HTTP authentication
type BasicAuthProvider struct {
	config      *BasicAuthConfig
	cache       TokenCache
	auditor     AuditLogger
	userStore   UserStore
	mu          sync.RWMutex
	users       map[string]*UserInfo
}

// UserInfo represents user information for basic authentication
type UserInfo struct {
	Username     string            `json:"username"`
	PasswordHash string            `json:"password_hash"`
	Salt         string            `json:"salt,omitempty"`
	UserID       string            `json:"user_id"`
	Email        string            `json:"email,omitempty"`
	FullName     string            `json:"full_name,omitempty"`
	Roles        []string          `json:"roles"`
	Permissions  []string          `json:"permissions"`
	Metadata     map[string]string `json:"metadata"`
	Active       bool              `json:"active"`
	CreatedAt    time.Time         `json:"created_at"`
	LastLoginAt  time.Time         `json:"last_login_at"`
	FailedLogins int               `json:"failed_logins"`
	LockedUntil  time.Time         `json:"locked_until,omitempty"`
}

// UserStore provides user storage interface for basic auth
type UserStore interface {
	// GetUser retrieves user by username
	GetUser(ctx context.Context, username string) (*UserInfo, error)
	
	// UpdateUser updates user information
	UpdateUser(ctx context.Context, user *UserInfo) error
	
	// CreateUser creates a new user
	CreateUser(ctx context.Context, user *UserInfo) error
	
	// DeleteUser deletes a user
	DeleteUser(ctx context.Context, username string) error
	
	// ListUsers lists all users
	ListUsers(ctx context.Context) ([]*UserInfo, error)
}

// MemoryUserStore provides in-memory user storage
type MemoryUserStore struct {
	users map[string]*UserInfo
	mu    sync.RWMutex
}

// NewMemoryUserStore creates a new memory user store
func NewMemoryUserStore() *MemoryUserStore {
	return &MemoryUserStore{
		users: make(map[string]*UserInfo),
	}
}

// GetUser retrieves user by username
func (m *MemoryUserStore) GetUser(ctx context.Context, username string) (*UserInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if user, exists := m.users[username]; exists {
		return user, nil
	}
	
	return nil, fmt.Errorf("user not found: %s", username)
}

// UpdateUser updates user information
func (m *MemoryUserStore) UpdateUser(ctx context.Context, user *UserInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.users[user.Username] = user
	return nil
}

// CreateUser creates a new user
func (m *MemoryUserStore) CreateUser(ctx context.Context, user *UserInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.users[user.Username]; exists {
		return fmt.Errorf("user already exists: %s", user.Username)
	}
	
	user.CreatedAt = time.Now()
	user.Active = true
	m.users[user.Username] = user
	return nil
}

// DeleteUser deletes a user
func (m *MemoryUserStore) DeleteUser(ctx context.Context, username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.users, username)
	return nil
}

// ListUsers lists all users
func (m *MemoryUserStore) ListUsers(ctx context.Context) ([]*UserInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	users := make([]*UserInfo, 0, len(m.users))
	for _, user := range m.users {
		users = append(users, user)
	}
	
	return users, nil
}

// NewBasicAuthProvider creates a new basic authentication provider
func NewBasicAuthProvider(config *BasicAuthConfig, cache TokenCache, auditor AuditLogger, userStore UserStore) *BasicAuthProvider {
	if config == nil {
		config = &BasicAuthConfig{
			Realm:              "Restricted Area",
			UserProvider:       "memory",
			PasswordValidation: "bcrypt",
			HashAlgorithm:      "bcrypt",
		}
	}

	if userStore == nil {
		userStore = NewMemoryUserStore()
	}

	return &BasicAuthProvider{
		config:    config,
		cache:     cache,
		auditor:   auditor,
		userStore: userStore,
		users:     make(map[string]*UserInfo),
	}
}

// Name returns the provider name
func (b *BasicAuthProvider) Name() string {
	return "basic_auth"
}

// Authenticate validates basic auth credentials and returns authentication context
func (b *BasicAuthProvider) Authenticate(ctx context.Context, credentials *Credentials) (*AuthContext, error) {
	if credentials == nil || credentials.Username == "" || credentials.Password == "" {
		return nil, fmt.Errorf("username and password are required")
	}

	// Check cache first for recent authentication
	cacheKey := fmt.Sprintf("basic:%s", credentials.Username)
	if b.cache != nil {
		if cached, err := b.cache.Get(ctx, cacheKey); err == nil && cached != nil {
			// Verify password matches cached credentials
			if subtle.ConstantTimeCompare([]byte(credentials.Password), []byte(cached.Password)) == 1 {
				return b.credentialsToAuthContext(cached), nil
			}
		}
	}

	// Get user from store
	user, err := b.userStore.GetUser(ctx, credentials.Username)
	if err != nil {
		if b.auditor != nil {
			b.auditor.LogAuthFailure(ctx, fmt.Sprintf("User not found: %v", err), map[string]any{
				"username": credentials.Username,
			})
		}
		return nil, fmt.Errorf("authentication failed")
	}

	// Check if user is active
	if !user.Active {
		if b.auditor != nil {
			b.auditor.LogAuthFailure(ctx, "User account is inactive", map[string]any{
				"username": credentials.Username,
				"user_id":  user.UserID,
			})
		}
		return nil, fmt.Errorf("account is inactive")
	}

	// Check if user is locked
	if !user.LockedUntil.IsZero() && time.Now().Before(user.LockedUntil) {
		if b.auditor != nil {
			b.auditor.LogAuthFailure(ctx, "User account is locked", map[string]any{
				"username":     credentials.Username,
				"user_id":      user.UserID,
				"locked_until": user.LockedUntil,
			})
		}
		return nil, fmt.Errorf("account is locked")
	}

	// Validate password
	if err := b.validatePassword(credentials.Password, user.PasswordHash); err != nil {
		// Increment failed login counter
		user.FailedLogins++
		
		// Lock account if too many failed attempts
		if user.FailedLogins >= 5 {
			user.LockedUntil = time.Now().Add(30 * time.Minute)
		}
		
		_ = b.userStore.UpdateUser(ctx, user)

		if b.auditor != nil {
			b.auditor.LogAuthFailure(ctx, fmt.Sprintf("Password validation failed: %v", err), map[string]any{
				"username":      credentials.Username,
				"user_id":       user.UserID,
				"failed_logins": user.FailedLogins,
			})
		}
		return nil, fmt.Errorf("authentication failed")
	}

	// Reset failed login counter and update last login
	user.FailedLogins = 0
	user.LockedUntil = time.Time{}
	user.LastLoginAt = time.Now()
	_ = b.userStore.UpdateUser(ctx, user)

	// Create auth context
	authCtx := &AuthContext{
		UserID:      user.UserID,
		Username:    user.Username,
		Roles:       user.Roles,
		Permissions: user.Permissions,
		Metadata:    user.Metadata,
		AuthMethod:  "basic_auth",
		Timestamp:   time.Now(),
	}

	// Cache credentials for short period to avoid repeated DB lookups
	if b.cache != nil {
		cacheCredentials := &Credentials{
			Type:     "basic_auth",
			Username: credentials.Username,
			Password: credentials.Password,
			Metadata: user.Metadata,
			Subject:  user.UserID,
		}

		_ = b.cache.Set(ctx, cacheKey, cacheCredentials, 5*time.Minute)
	}

	// Log successful authentication
	if b.auditor != nil {
		b.auditor.LogAuthSuccess(ctx, authCtx, map[string]any{
			"username": credentials.Username,
		})
	}

	return authCtx, nil
}

// Validate validates an existing authentication context
func (b *BasicAuthProvider) Validate(ctx context.Context, authCtx *AuthContext) error {
	if authCtx == nil {
		return fmt.Errorf("authentication context is required")
	}

	if authCtx.AuthMethod != "basic_auth" {
		return fmt.Errorf("invalid authentication method: %s", authCtx.AuthMethod)
	}

	// Basic auth sessions typically don't expire, but check if configured
	if !authCtx.ExpiresAt.IsZero() && time.Now().After(authCtx.ExpiresAt) {
		return fmt.Errorf("authentication context has expired")
	}

	return nil
}

// Refresh creates new credentials (basic auth doesn't support refresh)
func (b *BasicAuthProvider) Refresh(ctx context.Context, credentials *Credentials) (*Credentials, error) {
	return nil, fmt.Errorf("basic auth refresh is not supported")
}

// Revoke revokes credentials by removing from cache
func (b *BasicAuthProvider) Revoke(ctx context.Context, credentials *Credentials) error {
	if credentials == nil || credentials.Username == "" {
		return fmt.Errorf("username is required")
	}

	if b.cache != nil {
		cacheKey := fmt.Sprintf("basic:%s", credentials.Username)
		return b.cache.Delete(ctx, cacheKey)
	}

	return nil
}

// SupportedTypes returns supported credential types
func (b *BasicAuthProvider) SupportedTypes() []string {
	return []string{"basic", "basic_auth"}
}

// validatePassword validates password against stored hash
func (b *BasicAuthProvider) validatePassword(password, hash string) error {
	switch b.config.HashAlgorithm {
	case "sha256":
		expectedHash := sha256.Sum256([]byte(password))
		storedHash, err := base64.StdEncoding.DecodeString(hash)
		if err != nil {
			return err
		}
		if subtle.ConstantTimeCompare(expectedHash[:], storedHash) != 1 {
			return fmt.Errorf("password mismatch")
		}
		return nil
	default:
		// Plain text comparison (not recommended for production)
		if subtle.ConstantTimeCompare([]byte(password), []byte(hash)) != 1 {
			return fmt.Errorf("password mismatch")
		}
		return nil
	}
}

// credentialsToAuthContext converts credentials to auth context
func (b *BasicAuthProvider) credentialsToAuthContext(credentials *Credentials) *AuthContext {
	return &AuthContext{
		UserID:     credentials.Subject,
		Username:   credentials.Username,
		Metadata:   credentials.Metadata,
		AuthMethod: "basic_auth",
		Timestamp:  time.Now(),
	}
}

// CreateUser creates a new user with hashed password
func (b *BasicAuthProvider) CreateUser(ctx context.Context, username, password string, roles []string) error {
	hashedPassword, err := b.hashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user := &UserInfo{
		Username:     username,
		PasswordHash: hashedPassword,
		UserID:       username, // Use username as ID for simplicity
		Roles:        roles,
		Active:       true,
		CreatedAt:    time.Now(),
	}

	return b.userStore.CreateUser(ctx, user)
}

// hashPassword creates hash of password
func (b *BasicAuthProvider) hashPassword(password string) (string, error) {
	switch b.config.HashAlgorithm {
	case "sha256":
		hash := sha256.Sum256([]byte(password))
		return base64.StdEncoding.EncodeToString(hash[:]), nil
	default:
		// Plain text (not recommended for production)
		return password, nil
	}
}


// BasicAuthMiddleware implements basic authentication middleware
type BasicAuthMiddleware struct {
	provider  *BasicAuthProvider
	extractor CredentialExtractor
	enabled   bool
	priority  int
}

// NewBasicAuthMiddleware creates new basic authentication middleware
func NewBasicAuthMiddleware(config *BasicAuthConfig, cache TokenCache, auditor AuditLogger, userStore UserStore) *BasicAuthMiddleware {
	provider := NewBasicAuthProvider(config, cache, auditor, userStore)

	return &BasicAuthMiddleware{
		provider:  provider,
		extractor: NewBasicAuthExtractor(),
		enabled:   true,
		priority:  80, // High priority for authentication
	}
}

// Name returns middleware name
func (b *BasicAuthMiddleware) Name() string {
	return "basic_auth"
}

// Process processes the request through basic authentication
func (b *BasicAuthMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Extract credentials
	credentials, err := b.extractor.Extract(ctx, req.Headers, req.Body)
	if err != nil || credentials == nil {
		realm := b.provider.config.Realm
		return &Response{
			ID:         req.ID,
			StatusCode: 401,
			Headers:    map[string]string{"WWW-Authenticate": fmt.Sprintf("Basic realm=\"%s\"", realm)},
			Error:      fmt.Errorf("basic authentication required: %w", err),
			Timestamp:  time.Now(),
		}, nil
	}

	// Authenticate
	authCtx, err := b.provider.Authenticate(ctx, credentials)
	if err != nil {
		realm := b.provider.config.Realm
		return &Response{
			ID:         req.ID,
			StatusCode: 401,
			Headers:    map[string]string{"WWW-Authenticate": fmt.Sprintf("Basic realm=\"%s\"", realm)},
			Error:      fmt.Errorf("authentication failed: %w", err),
			Timestamp:  time.Now(),
		}, nil
	}

	// Add auth context to request metadata
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}
	req.Metadata["auth_context"] = authCtx

	// Continue to next middleware
	return next(ctx, req)
}

// Configure configures the middleware
func (b *BasicAuthMiddleware) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		b.enabled = enabled
	}

	if priority, ok := config["priority"].(int); ok {
		b.priority = priority
	}

	return nil
}

// Enabled returns whether the middleware is enabled
func (b *BasicAuthMiddleware) Enabled() bool {
	return b.enabled
}

// Priority returns the middleware priority
func (b *BasicAuthMiddleware) Priority() int {
	return b.priority
}

// BasicAuthExtractor extracts basic auth credentials from Authorization header
type BasicAuthExtractor struct{}

// NewBasicAuthExtractor creates a new basic auth extractor
func NewBasicAuthExtractor() *BasicAuthExtractor {
	return &BasicAuthExtractor{}
}

// Extract extracts basic auth credentials from Authorization header
func (e *BasicAuthExtractor) Extract(ctx context.Context, headers map[string]string, body any) (*Credentials, error) {
	authHeader := ""

	// Look for Authorization header (case-insensitive)
	for k, v := range headers {
		if strings.ToLower(k) == "authorization" {
			authHeader = v
			break
		}
	}

	if authHeader == "" {
		return nil, fmt.Errorf("authorization header not found")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "basic" {
		return nil, fmt.Errorf("invalid authorization header format")
	}

	// Decode base64 credentials
	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode basic auth credentials: %w", err)
	}

	// Split username and password
	credentialParts := strings.SplitN(string(decoded), ":", 2)
	if len(credentialParts) != 2 {
		return nil, fmt.Errorf("invalid basic auth credential format")
	}

	return &Credentials{
		Type:     "basic_auth",
		Username: credentialParts[0],
		Password: credentialParts[1],
	}, nil
}

// SupportedMethods returns supported authentication methods
func (e *BasicAuthExtractor) SupportedMethods() []string {
	return []string{"basic", "basic_auth"}
}