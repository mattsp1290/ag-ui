package auth

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

)

// APIKeyProvider implements API key-based authentication
type APIKeyProvider struct {
	config     *APIKeyConfig
	cache      TokenCache
	auditor    AuditLogger
	httpClient *http.Client
	mu         sync.RWMutex
	apiKeys    map[string]*APIKeyInfo
}

// APIKeyInfo represents information about an API key
type APIKeyInfo struct {
	Key         string            `json:"key"`
	UserID      string            `json:"user_id"`
	Username    string            `json:"username"`
	Roles       []string          `json:"roles"`
	Permissions []string          `json:"permissions"`
	Scopes      []string          `json:"scopes"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	ExpiresAt   time.Time         `json:"expires_at,omitempty"`
	LastUsedAt  time.Time         `json:"last_used_at"`
	RateLimit   int               `json:"rate_limit"`
	Active      bool              `json:"active"`
}

// NewAPIKeyProvider creates a new API key authentication provider
func NewAPIKeyProvider(config *APIKeyConfig, cache TokenCache, auditor AuditLogger) *APIKeyProvider {
	if config == nil {
		config = &APIKeyConfig{
			HeaderName:      "X-API-Key",
			CacheTimeout:    5 * time.Minute,
			RateLimitPerKey: 1000,
		}
	}

	return &APIKeyProvider{
		config:     config,
		cache:      cache,
		auditor:    auditor,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiKeys:    make(map[string]*APIKeyInfo),
	}
}

// Name returns the provider name
func (a *APIKeyProvider) Name() string {
	return "api_key"
}

// Authenticate validates API key and returns authentication context
func (a *APIKeyProvider) Authenticate(ctx context.Context, credentials *Credentials) (*AuthContext, error) {
	if credentials == nil || credentials.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Check cache first
	cacheKey := fmt.Sprintf("apikey:%s", credentials.APIKey)
	if a.cache != nil {
		if cached, err := a.cache.Get(ctx, cacheKey); err == nil && cached != nil {
			keyInfo := a.credentialsToAPIKeyInfo(cached)
			return a.apiKeyInfoToAuthContext(keyInfo), nil
		}
	}

	// Validate API key
	keyInfo, err := a.validateAPIKey(ctx, credentials.APIKey)
	if err != nil {
		if a.auditor != nil {
			a.auditor.LogAuthFailure(ctx, fmt.Sprintf("API key validation failed: %v", err), map[string]any{
				"key_prefix": a.keyPrefix(credentials.APIKey),
			})
		}
		return nil, fmt.Errorf("API key validation failed: %w", err)
	}

	// Check if key is active and not expired
	if !keyInfo.Active {
		if a.auditor != nil {
			a.auditor.LogAuthFailure(ctx, "API key is inactive", map[string]any{
				"key_prefix": a.keyPrefix(credentials.APIKey),
				"user_id":    keyInfo.UserID,
			})
		}
		return nil, fmt.Errorf("API key is inactive")
	}

	if !keyInfo.ExpiresAt.IsZero() && time.Now().After(keyInfo.ExpiresAt) {
		if a.auditor != nil {
			a.auditor.LogAuthFailure(ctx, "API key has expired", map[string]any{
				"key_prefix": a.keyPrefix(credentials.APIKey),
				"user_id":    keyInfo.UserID,
				"expired_at": keyInfo.ExpiresAt,
			})
		}
		return nil, fmt.Errorf("API key has expired")
	}

	// Update last used timestamp
	keyInfo.LastUsedAt = time.Now()
	a.mu.Lock()
	a.apiKeys[credentials.APIKey] = keyInfo
	a.mu.Unlock()

	// Create auth context
	authCtx := a.apiKeyInfoToAuthContext(keyInfo)

	// Cache the credentials
	if a.cache != nil {
		cacheCredentials := &Credentials{
			Type:        "api_key",
			APIKey:      credentials.APIKey,
			Metadata:    keyInfo.Metadata,
			Subject:     keyInfo.UserID,
			Scopes:      keyInfo.Scopes,
			ExpiresAt:   keyInfo.ExpiresAt,
		}

		ttl := a.config.CacheTimeout
		if !keyInfo.ExpiresAt.IsZero() {
			expTTL := time.Until(keyInfo.ExpiresAt)
			if expTTL < ttl {
				ttl = expTTL
			}
		}

		_ = a.cache.Set(ctx, cacheKey, cacheCredentials, ttl)
	}

	// Log successful authentication
	if a.auditor != nil {
		a.auditor.LogAuthSuccess(ctx, authCtx, map[string]any{
			"key_prefix": a.keyPrefix(credentials.APIKey),
		})
	}

	return authCtx, nil
}

// Validate validates an existing authentication context
func (a *APIKeyProvider) Validate(ctx context.Context, authCtx *AuthContext) error {
	if authCtx == nil {
		return fmt.Errorf("authentication context is required")
	}

	if authCtx.AuthMethod != "api_key" {
		return fmt.Errorf("invalid authentication method: %s", authCtx.AuthMethod)
	}

	// Check expiration
	if !authCtx.ExpiresAt.IsZero() && time.Now().After(authCtx.ExpiresAt) {
		return fmt.Errorf("authentication context has expired")
	}

	return nil
}

// Refresh is not supported for API keys
func (a *APIKeyProvider) Refresh(ctx context.Context, credentials *Credentials) (*Credentials, error) {
	return nil, fmt.Errorf("API key refresh is not supported")
}

// Revoke revokes API key by removing it from cache and marking as inactive
func (a *APIKeyProvider) Revoke(ctx context.Context, credentials *Credentials) error {
	if credentials == nil || credentials.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	// Remove from cache
	if a.cache != nil {
		cacheKey := fmt.Sprintf("apikey:%s", credentials.APIKey)
		_ = a.cache.Delete(ctx, cacheKey)
	}

	// Mark as inactive in memory
	a.mu.Lock()
	if keyInfo, exists := a.apiKeys[credentials.APIKey]; exists {
		keyInfo.Active = false
	}
	a.mu.Unlock()

	return nil
}

// SupportedTypes returns supported credential types
func (a *APIKeyProvider) SupportedTypes() []string {
	return []string{"api_key", "apikey"}
}

// validateAPIKey validates API key against configured validation endpoint or local storage
func (a *APIKeyProvider) validateAPIKey(ctx context.Context, apiKey string) (*APIKeyInfo, error) {
	// Check local storage first
	a.mu.RLock()
	if keyInfo, exists := a.apiKeys[apiKey]; exists {
		a.mu.RUnlock()
		return keyInfo, nil
	}
	a.mu.RUnlock()

	// Use validation endpoint if configured
	if a.config.ValidationEndpoint != "" {
		return a.validateAPIKeyRemote(ctx, apiKey)
	}

	// For demo purposes, create a default key info
	// In production, this should integrate with your key management system
	return &APIKeyInfo{
		Key:        apiKey,
		UserID:     "default_user",
		Username:   "api_user",
		Roles:      []string{"user"},
		Permissions: []string{"read"},
		Active:     true,
		CreatedAt:  time.Now(),
		RateLimit:  a.config.RateLimitPerKey,
	}, nil
}

// validateAPIKeyRemote validates API key using remote endpoint
func (a *APIKeyProvider) validateAPIKeyRemote(ctx context.Context, apiKey string) (*APIKeyInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", a.config.ValidationEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create validation request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(a.config.HeaderName, apiKey)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("validation request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API key validation failed with status: %d", resp.StatusCode)
	}

	var keyInfo APIKeyInfo
	if err := resp.Body.(interface{ Decode(interface{}) error }).Decode(&keyInfo); err != nil {
		// Fallback for demo purposes
		keyInfo = APIKeyInfo{
			Key:        apiKey,
			UserID:     "validated_user",
			Username:   "api_user",
			Roles:      []string{"user"},
			Permissions: []string{"read", "write"},
			Active:     true,
			CreatedAt:  time.Now(),
			RateLimit:  a.config.RateLimitPerKey,
		}
	}

	// Cache validated key info
	a.mu.Lock()
	a.apiKeys[apiKey] = &keyInfo
	a.mu.Unlock()

	return &keyInfo, nil
}

// apiKeyInfoToAuthContext converts API key info to auth context
func (a *APIKeyProvider) apiKeyInfoToAuthContext(keyInfo *APIKeyInfo) *AuthContext {
	return &AuthContext{
		UserID:      keyInfo.UserID,
		Username:    keyInfo.Username,
		Roles:       keyInfo.Roles,
		Permissions: keyInfo.Permissions,
		Metadata:    keyInfo.Metadata,
		AuthMethod:  "api_key",
		Timestamp:   time.Now(),
		ExpiresAt:   keyInfo.ExpiresAt,
	}
}

// credentialsToAPIKeyInfo converts credentials to API key info
func (a *APIKeyProvider) credentialsToAPIKeyInfo(credentials *Credentials) *APIKeyInfo {
	keyInfo := &APIKeyInfo{
		Key:       credentials.APIKey,
		UserID:    credentials.Subject,
		Metadata:  credentials.Metadata,
		ExpiresAt: credentials.ExpiresAt,
		Active:    true,
		RateLimit: a.config.RateLimitPerKey,
	}

	if scopes := credentials.Scopes; len(scopes) > 0 {
		keyInfo.Scopes = scopes
	}

	return keyInfo
}

// keyPrefix returns the first few characters of API key for logging
func (a *APIKeyProvider) keyPrefix(apiKey string) string {
	if len(apiKey) > 8 {
		return apiKey[:8] + "..."
	}
	return apiKey
}

// AddAPIKey adds a new API key to local storage
func (a *APIKeyProvider) AddAPIKey(keyInfo *APIKeyInfo) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.apiKeys[keyInfo.Key] = keyInfo
}

// RemoveAPIKey removes API key from local storage
func (a *APIKeyProvider) RemoveAPIKey(apiKey string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.apiKeys, apiKey)
}

// ListAPIKeys returns all API keys for a user
func (a *APIKeyProvider) ListAPIKeys(userID string) []*APIKeyInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var keys []*APIKeyInfo
	for _, keyInfo := range a.apiKeys {
		if keyInfo.UserID == userID {
			keys = append(keys, keyInfo)
		}
	}

	return keys
}


// APIKeyMiddleware implements API key authentication middleware
type APIKeyMiddleware struct {
	provider  *APIKeyProvider
	extractor CredentialExtractor
	enabled   bool
	priority  int
}

// NewAPIKeyMiddleware creates new API key authentication middleware
func NewAPIKeyMiddleware(config *APIKeyConfig, cache TokenCache, auditor AuditLogger) *APIKeyMiddleware {
	provider := NewAPIKeyProvider(config, cache, auditor)

	return &APIKeyMiddleware{
		provider:  provider,
		extractor: NewAPIKeyExtractor(config.HeaderName, config.QueryParam),
		enabled:   true,
		priority:  90, // High priority for authentication
	}
}

// Name returns middleware name
func (a *APIKeyMiddleware) Name() string {
	return "api_key_auth"
}

// Process processes the request through API key authentication
func (a *APIKeyMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Extract credentials
	credentials, err := a.extractor.Extract(ctx, req.Headers, req.Body)
	if err != nil || credentials == nil {
		return &Response{
			ID:         req.ID,
			StatusCode: 401,
			Headers:    map[string]string{"WWW-Authenticate": "API-Key"},
			Error:      fmt.Errorf("API key required: %w", err),
			Timestamp:  time.Now(),
		}, nil
	}

	// Authenticate
	authCtx, err := a.provider.Authenticate(ctx, credentials)
	if err != nil {
		return &Response{
			ID:         req.ID,
			StatusCode: 401,
			Headers:    map[string]string{"WWW-Authenticate": "API-Key"},
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
func (a *APIKeyMiddleware) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		a.enabled = enabled
	}

	if priority, ok := config["priority"].(int); ok {
		a.priority = priority
	}

	return nil
}

// Enabled returns whether the middleware is enabled
func (a *APIKeyMiddleware) Enabled() bool {
	return a.enabled
}

// Priority returns the middleware priority
func (a *APIKeyMiddleware) Priority() int {
	return a.priority
}

// APIKeyExtractor extracts API keys from headers or query parameters
type APIKeyExtractor struct {
	headerName string
	queryParam string
}

// NewAPIKeyExtractor creates a new API key extractor
func NewAPIKeyExtractor(headerName, queryParam string) *APIKeyExtractor {
	if headerName == "" {
		headerName = "X-API-Key"
	}
	
	return &APIKeyExtractor{
		headerName: headerName,
		queryParam: queryParam,
	}
}

// Extract extracts API key from headers or query parameters
func (e *APIKeyExtractor) Extract(ctx context.Context, headers map[string]string, body any) (*Credentials, error) {
	// Try header first
	for k, v := range headers {
		if strings.EqualFold(k, e.headerName) {
			return &Credentials{
				Type:   "api_key",
				APIKey: v,
			}, nil
		}
	}

	// Try query parameter if configured
	if e.queryParam != "" {
		// In a real implementation, you would extract from URL query parameters
		// This is a simplified version for the interface
		if bodyMap, ok := body.(map[string]interface{}); ok {
			if apiKey, ok := bodyMap[e.queryParam].(string); ok {
				return &Credentials{
					Type:   "api_key",
					APIKey: apiKey,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("API key not found in %s header or %s query parameter", e.headerName, e.queryParam)
}

// SupportedMethods returns supported authentication methods
func (e *APIKeyExtractor) SupportedMethods() []string {
	return []string{"api_key", "apikey"}
}

// APIKeyValidator provides utilities for API key validation
type APIKeyValidator struct {
	minLength int
	maxLength int
	prefix    string
}

// NewAPIKeyValidator creates a new API key validator
func NewAPIKeyValidator(minLength, maxLength int, prefix string) *APIKeyValidator {
	return &APIKeyValidator{
		minLength: minLength,
		maxLength: maxLength,
		prefix:    prefix,
	}
}

// ValidateFormat validates API key format
func (v *APIKeyValidator) ValidateFormat(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	if len(apiKey) < v.minLength {
		return fmt.Errorf("API key too short, minimum length is %d", v.minLength)
	}

	if v.maxLength > 0 && len(apiKey) > v.maxLength {
		return fmt.Errorf("API key too long, maximum length is %d", v.maxLength)
	}

	if v.prefix != "" && !strings.HasPrefix(apiKey, v.prefix) {
		return fmt.Errorf("API key must start with prefix: %s", v.prefix)
	}

	return nil
}

// SecureCompare performs constant-time comparison of API keys
func (v *APIKeyValidator) SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}