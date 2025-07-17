package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// BasicAuthProvider provides a simple in-memory authentication implementation
// This can be used for testing or as a foundation for more complex providers
type BasicAuthProvider struct {
	config        *AuthConfig
	users         map[string]*User
	sessions      map[string]*AuthContext
	revokedTokens map[string]time.Time
	auditLogger   AuditLogger
	rotationInfo  map[string]*TokenRotationInfo // token -> rotation info
	stopRotation  chan struct{}
	mutex         sync.RWMutex
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
	
	provider := &BasicAuthProvider{
		config:        config,
		users:         make(map[string]*User),
		sessions:      make(map[string]*AuthContext),
		revokedTokens: make(map[string]time.Time),
		rotationInfo:  make(map[string]*TokenRotationInfo),
		stopRotation:  make(chan struct{}),
	}
	
	// Initialize audit logger if enabled
	if config.EnableAuditLogging {
		provider.auditLogger = NewMemoryAuditLogger()
	}
	
	// Start token rotation if enabled
	if config.AutoRotateTokens {
		go provider.startTokenRotation()
	}
	
	return provider
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
		p.logAuditEvent(&AuditEvent{
			ID:        generateID("audit"),
			Timestamp: time.Now(),
			EventType: AuditEventAuthFailure,
			Username:  creds.Username,
			Result:    "FAILURE",
			Error:     "user not found",
			Metadata: map[string]interface{}{
				"credential_type": "basic",
			},
		})
		return nil, ErrInvalidCredentials
	}
	
	if !user.Active {
		p.logAuditEvent(&AuditEvent{
			ID:        generateID("audit"),
			Timestamp: time.Now(),
			EventType: AuditEventAuthFailure,
			UserID:    user.ID,
			Username:  user.Username,
			Result:    "FAILURE",
			Error:     "user inactive",
			Metadata: map[string]interface{}{
				"credential_type": "basic",
			},
		})
		return nil, ErrUnauthorized
	}
	
	// Verify password
	if !verifyPassword(creds.Password, user.PasswordHash) {
		p.logAuditEvent(&AuditEvent{
			ID:        generateID("audit"),
			Timestamp: time.Now(),
			EventType: AuditEventAuthFailure,
			UserID:    user.ID,
			Username:  user.Username,
			Result:    "FAILURE",
			Error:     "invalid password",
			Metadata: map[string]interface{}{
				"credential_type": "basic",
			},
		})
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
	
	// Log successful authentication
	p.logAuditEvent(&AuditEvent{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		EventType: AuditEventLogin,
		UserID:    user.ID,
		Username:  user.Username,
		Result:    "SUCCESS",
		TokenID:   token,
		Metadata: map[string]interface{}{
			"credential_type": "basic",
		},
	})
	
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
	
	// Log token refresh
	p.logAuditEvent(&AuditEvent{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		EventType: AuditEventTokenRefresh,
		UserID:    oldSession.UserID,
		Username:  oldSession.Username,
		Result:    "SUCCESS",
		TokenID:   newToken,
		Metadata: map[string]interface{}{
			"old_token_id": authCtx.Token,
			"new_token_id": newToken,
		},
	})
	
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
	
	// Log logout/revocation
	p.logAuditEvent(&AuditEvent{
		ID:        generateID("audit"),
		Timestamp: time.Now(),
		EventType: AuditEventLogout,
		UserID:    authCtx.UserID,
		Username:  authCtx.Username,
		Result:    "SUCCESS",
		TokenID:   authCtx.Token,
	})
	
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
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		// Fallback to a secure default if bcrypt fails
		// This should not happen in normal circumstances
		panic(fmt.Sprintf("failed to hash password: %v", err))
	}
	return string(hash)
}

// verifyPassword verifies a password against a hash
func verifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func generateToken() string {
	return generateID("token")
}

func generateID(prefix string) string {
	// Generate 16 random bytes
	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback to timestamp-based generation if crypto/rand fails
		// This should not happen in normal circumstances
		timestamp := time.Now().UnixNano()
		hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", prefix, timestamp)))
		return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(hash[:])[:16])
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(randomBytes))
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

// Helper methods for audit logging and token rotation

// logAuditEvent logs an audit event if audit logging is enabled
func (p *BasicAuthProvider) logAuditEvent(event *AuditEvent) {
	if p.config.EnableAuditLogging && p.auditLogger != nil {
		p.auditLogger.LogEvent(event)
	}
}

// startTokenRotation starts the automatic token rotation process
func (p *BasicAuthProvider) startTokenRotation() {
	ticker := time.NewTicker(p.config.TokenRotationInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.rotateAllTokens()
		case <-p.stopRotation:
			return
		}
	}
}

// rotateAllTokens rotates all active tokens that are eligible for rotation
func (p *BasicAuthProvider) rotateAllTokens() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	now := time.Now()
	tokensToRotate := make([]string, 0)
	
	// Find tokens that need rotation
	for token, session := range p.sessions {
		if session.IssuedAt.Add(p.config.TokenRotationInterval).Before(now) {
			tokensToRotate = append(tokensToRotate, token)
		}
	}
	
	// Rotate eligible tokens
	for _, oldToken := range tokensToRotate {
		if session, exists := p.sessions[oldToken]; exists {
			newToken := generateToken()
			expiresAt := now.Add(p.config.TokenExpiration)
			
			// Create new session
			newSession := &AuthContext{
				UserID:       session.UserID,
				Username:     session.Username,
				Roles:        session.Roles,
				Permissions:  session.Permissions,
				Token:        newToken,
				ExpiresAt:    &expiresAt,
				IssuedAt:     now,
				Metadata:     session.Metadata,
				ProviderType: p.GetProviderType(),
			}
			
			// Update rotation info
			rotationCount := 0
			if info, exists := p.rotationInfo[oldToken]; exists {
				rotationCount = info.RotationCount + 1
			}
			
			p.rotationInfo[newToken] = &TokenRotationInfo{
				OldTokenID:    oldToken,
				NewTokenID:    newToken,
				RotatedAt:     now,
				NextRotation:  now.Add(p.config.TokenRotationInterval),
				RotationCount: rotationCount,
			}
			
			// Store new session and revoke old one
			p.sessions[newToken] = newSession
			p.revokedTokens[oldToken] = now
			delete(p.sessions, oldToken)
			delete(p.rotationInfo, oldToken)
			
			// Log token rotation
			p.logAuditEvent(&AuditEvent{
				ID:        generateID("audit"),
				Timestamp: now,
				EventType: AuditEventTokenRotation,
				UserID:    session.UserID,
				Username:  session.Username,
				Result:    "SUCCESS",
				TokenID:   newToken,
				Metadata: map[string]interface{}{
					"old_token_id":    oldToken,
					"new_token_id":    newToken,
					"rotation_count":  rotationCount,
					"auto_rotation":   true,
				},
			})
		}
	}
}

// StopTokenRotation stops the automatic token rotation process
func (p *BasicAuthProvider) StopTokenRotation() {
	close(p.stopRotation)
}

// GetAuditEvents returns audit events based on the provided filter
func (p *BasicAuthProvider) GetAuditEvents(filter *AuditEventFilter) ([]*AuditEvent, error) {
	if p.auditLogger == nil {
		return nil, errors.New("audit logging is not enabled")
	}
	return p.auditLogger.GetEvents(filter)
}

// CleanupOldAuditEvents removes audit events older than the retention period
func (p *BasicAuthProvider) CleanupOldAuditEvents() error {
	if p.auditLogger == nil {
		return nil
	}
	
	cutoff := time.Now().Add(-p.config.AuditLogRetention)
	return p.auditLogger.CleanupOldEvents(cutoff)
}

// MemoryAuditLogger is a simple in-memory implementation of AuditLogger
type MemoryAuditLogger struct {
	events []AuditEvent
	mutex  sync.RWMutex
}

// NewMemoryAuditLogger creates a new in-memory audit logger
func NewMemoryAuditLogger() *MemoryAuditLogger {
	return &MemoryAuditLogger{
		events: make([]AuditEvent, 0),
	}
}

// LogEvent logs an audit event
func (mal *MemoryAuditLogger) LogEvent(event *AuditEvent) error {
	mal.mutex.Lock()
	defer mal.mutex.Unlock()
	
	mal.events = append(mal.events, *event)
	return nil
}

// GetEvents returns audit events based on the provided filter
func (mal *MemoryAuditLogger) GetEvents(filter *AuditEventFilter) ([]*AuditEvent, error) {
	mal.mutex.RLock()
	defer mal.mutex.RUnlock()
	
	var filtered []*AuditEvent
	
	for i := range mal.events {
		event := &mal.events[i]
		
		// Apply filters
		if filter != nil {
			// Filter by event types
			if len(filter.EventTypes) > 0 {
				found := false
				for _, eventType := range filter.EventTypes {
					if event.EventType == eventType {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			
			// Filter by user ID
			if filter.UserID != "" && event.UserID != filter.UserID {
				continue
			}
			
			// Filter by username
			if filter.Username != "" && event.Username != filter.Username {
				continue
			}
			
			// Filter by result
			if filter.Result != "" && event.Result != filter.Result {
				continue
			}
			
			// Filter by time range
			if filter.StartTime != nil && event.Timestamp.Before(*filter.StartTime) {
				continue
			}
			if filter.EndTime != nil && event.Timestamp.After(*filter.EndTime) {
				continue
			}
		}
		
		filtered = append(filtered, event)
	}
	
	// Apply limit and offset
	if filter != nil {
		if filter.Offset > 0 && filter.Offset < len(filtered) {
			filtered = filtered[filter.Offset:]
		}
		if filter.Limit > 0 && filter.Limit < len(filtered) {
			filtered = filtered[:filter.Limit]
		}
	}
	
	return filtered, nil
}

// CleanupOldEvents removes audit events older than the specified time
func (mal *MemoryAuditLogger) CleanupOldEvents(before time.Time) error {
	mal.mutex.Lock()
	defer mal.mutex.Unlock()
	
	var kept []AuditEvent
	for _, event := range mal.events {
		if event.Timestamp.After(before) {
			kept = append(kept, event)
		}
	}
	
	mal.events = kept
	return nil
}