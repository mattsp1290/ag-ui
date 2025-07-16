package sse

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// ============================================================================
// Security Manager
// ============================================================================

// SecurityManager provides comprehensive security features for SSE transport
type SecurityManager struct {
	config          SecurityConfig
	authenticators  map[AuthType]Authenticator
	rateLimiter     *RateLimiter
	validator       *RequestValidator
	auditLogger     *AuditLogger
	securityHeaders *SecurityHeaders
	mutex           sync.RWMutex
	logger          *zap.Logger
	metrics         *SecurityMetrics
}

// NewSecurityManager creates a new security manager
func NewSecurityManager(config SecurityConfig, logger *zap.Logger) (*SecurityManager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	sm := &SecurityManager{
		config:          config,
		authenticators:  make(map[AuthType]Authenticator),
		logger:          logger,
		metrics:         NewSecurityMetrics(),
		securityHeaders: NewSecurityHeaders(config.CORS),
	}

	// Initialize components
	if err := sm.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize security manager: %w", err)
	}

	return sm, nil
}

// initialize sets up all security components
func (sm *SecurityManager) initialize() error {
	// Initialize authenticators based on config
	if err := sm.initializeAuthenticators(); err != nil {
		return fmt.Errorf("failed to initialize authenticators: %w", err)
	}

	// Initialize rate limiter
	if sm.config.RateLimit.Enabled {
		sm.rateLimiter = NewRateLimiter(sm.config.RateLimit, sm.logger)
	}

	// Initialize request validator
	sm.validator = NewRequestValidator(sm.config.Validation, sm.logger)

	// Initialize audit logger
	sm.auditLogger = NewAuditLogger(sm.logger, sm.config.Auth.Type)

	return nil
}

// initializeAuthenticators sets up authentication providers
func (sm *SecurityManager) initializeAuthenticators() error {
	authConfig := sm.config.Auth

	switch authConfig.Type {
	case AuthTypeNone:
		// No authentication required
		return nil

	case AuthTypeBearer:
		if authConfig.BearerToken == "" {
			return errors.New("bearer token is required but not configured")
		}
		sm.authenticators[AuthTypeBearer] = NewBearerAuthenticator(authConfig.BearerToken)

	case AuthTypeAPIKey:
		if authConfig.APIKey == "" {
			return errors.New("API key is required but not configured")
		}
		sm.authenticators[AuthTypeAPIKey] = NewAPIKeyAuthenticator(
			authConfig.APIKey,
			authConfig.APIKeyHeader,
		)

	case AuthTypeBasic:
		if authConfig.BasicAuth.Username == "" || authConfig.BasicAuth.Password == "" {
			return errors.New("username and password are required for basic auth")
		}
		sm.authenticators[AuthTypeBasic] = NewBasicAuthenticator(
			authConfig.BasicAuth.Username,
			authConfig.BasicAuth.Password,
		)

	case AuthTypeJWT:
		jwtAuth, err := NewJWTAuthenticator(authConfig.JWT)
		if err != nil {
			return fmt.Errorf("failed to create JWT authenticator: %w", err)
		}
		sm.authenticators[AuthTypeJWT] = jwtAuth

	case AuthTypeOAuth2:
		oauth2Auth, err := NewOAuth2Authenticator(authConfig.OAuth2)
		if err != nil {
			return fmt.Errorf("failed to create OAuth2 authenticator: %w", err)
		}
		sm.authenticators[AuthTypeOAuth2] = oauth2Auth

	default:
		return fmt.Errorf("unsupported authentication type: %s", authConfig.Type)
	}

	return nil
}

// Authenticate validates the request authentication
func (sm *SecurityManager) Authenticate(r *http.Request) (*AuthContext, error) {
	sm.metrics.IncrementAuthAttempts()

	// No authentication required
	if sm.config.Auth.Type == AuthTypeNone {
		return &AuthContext{Authenticated: true}, nil
	}

	// Get authenticator
	sm.mutex.RLock()
	authenticator, exists := sm.authenticators[sm.config.Auth.Type]
	sm.mutex.RUnlock()

	if !exists {
		sm.metrics.IncrementAuthFailures()
		return nil, fmt.Errorf("no authenticator configured for type: %s", sm.config.Auth.Type)
	}

	// Perform authentication
	ctx, err := authenticator.Authenticate(r)
	if err != nil {
		sm.metrics.IncrementAuthFailures()
		sm.auditLogger.LogAuthFailure(r, err)
		return nil, err
	}

	sm.metrics.IncrementAuthSuccesses()
	sm.auditLogger.LogAuthSuccess(r, ctx)
	return ctx, nil
}

// CheckRateLimit checks if the request is within rate limits
func (sm *SecurityManager) CheckRateLimit(r *http.Request) error {
	if sm.rateLimiter == nil {
		return nil
	}

	clientID := sm.getClientIdentifier(r)
	allowed := sm.rateLimiter.Allow(clientID, r.URL.Path)

	if !allowed {
		sm.metrics.IncrementRateLimitHits()
		sm.auditLogger.LogRateLimitExceeded(r, clientID)
		return errors.New("rate limit exceeded")
	}

	return nil
}

// ValidateRequest validates the incoming request
func (sm *SecurityManager) ValidateRequest(r *http.Request) error {
	if sm.validator == nil {
		return nil
	}

	if err := sm.validator.Validate(r); err != nil {
		sm.metrics.IncrementValidationFailures()
		sm.auditLogger.LogValidationFailure(r, err)
		return err
	}

	return nil
}

// ApplySecurityHeaders applies security headers to the response
func (sm *SecurityManager) ApplySecurityHeaders(w http.ResponseWriter, r *http.Request) {
	sm.securityHeaders.Apply(w, r)
}

// getClientIdentifier extracts a client identifier from the request
func (sm *SecurityManager) getClientIdentifier(r *http.Request) string {
	// Try to get from configured method
	switch sm.config.RateLimit.PerClient.IdentificationMethod {
	case "ip":
		return getClientIP(r)
	case "user":
		if authCtx := r.Context().Value("auth_context"); authCtx != nil {
			if ctx, ok := authCtx.(*AuthContext); ok && ctx.UserID != "" {
				return ctx.UserID
			}
		}
	case "api_key":
		if key := r.Header.Get(sm.config.Auth.APIKeyHeader); key != "" {
			return hashString(key)
		}
	}

	// Fallback to IP
	return getClientIP(r)
}

// Metrics returns security metrics
func (sm *SecurityManager) Metrics() *SecurityMetrics {
	return sm.metrics
}

// ============================================================================
// Authentication Interfaces and Implementations
// ============================================================================

// Authenticator defines the interface for authentication providers
type Authenticator interface {
	Authenticate(r *http.Request) (*AuthContext, error)
	Type() AuthType
}

// AuthContext contains authentication information
type AuthContext struct {
	Authenticated bool
	UserID        string
	Username      string
	Roles         []string
	Permissions   []string
	TokenID       string
	ExpiresAt     time.Time
	Metadata      map[string]interface{}
}

// ============================================================================
// Bearer Token Authentication
// ============================================================================

// BearerAuthenticator implements bearer token authentication
type BearerAuthenticator struct {
	token string
}

// NewBearerAuthenticator creates a new bearer token authenticator
func NewBearerAuthenticator(token string) *BearerAuthenticator {
	return &BearerAuthenticator{token: token}
}

// Authenticate validates bearer token
func (ba *BearerAuthenticator) Authenticate(r *http.Request) (*AuthContext, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, errors.New("authorization header missing")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return nil, errors.New("invalid authorization header format")
	}

	if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(ba.token)) != 1 {
		return nil, errors.New("invalid bearer token")
	}

	return &AuthContext{
		Authenticated: true,
		TokenID:       hashString(parts[1]),
	}, nil
}

// Type returns the authenticator type
func (ba *BearerAuthenticator) Type() AuthType {
	return AuthTypeBearer
}

// ============================================================================
// API Key Authentication
// ============================================================================

// APIKeyAuthenticator implements API key authentication
type APIKeyAuthenticator struct {
	apiKeys map[string]*APIKeyInfo
	header  string
	mutex   sync.RWMutex
}

// APIKeyInfo contains API key metadata
type APIKeyInfo struct {
	Key         string
	UserID      string
	Permissions []string
	ExpiresAt   time.Time
}

// NewAPIKeyAuthenticator creates a new API key authenticator
func NewAPIKeyAuthenticator(apiKey, header string) *APIKeyAuthenticator {
	aka := &APIKeyAuthenticator{
		apiKeys: make(map[string]*APIKeyInfo),
		header:  header,
	}

	// Add the configured API key
	aka.AddKey(&APIKeyInfo{
		Key:    apiKey,
		UserID: "default",
	})

	return aka
}

// AddKey adds an API key
func (aka *APIKeyAuthenticator) AddKey(info *APIKeyInfo) {
	aka.mutex.Lock()
	defer aka.mutex.Unlock()
	aka.apiKeys[info.Key] = info
}

// RemoveKey removes an API key
func (aka *APIKeyAuthenticator) RemoveKey(key string) {
	aka.mutex.Lock()
	defer aka.mutex.Unlock()
	delete(aka.apiKeys, key)
}

// Authenticate validates API key
func (aka *APIKeyAuthenticator) Authenticate(r *http.Request) (*AuthContext, error) {
	key := r.Header.Get(aka.header)
	if key == "" {
		// Also check query parameter
		key = r.URL.Query().Get("api_key")
		if key == "" {
			return nil, fmt.Errorf("API key missing in header %s or query parameter", aka.header)
		}
	}

	aka.mutex.RLock()
	keyInfo, exists := aka.apiKeys[key]
	aka.mutex.RUnlock()

	if !exists {
		return nil, errors.New("invalid API key")
	}

	// Check expiration
	if !keyInfo.ExpiresAt.IsZero() && time.Now().After(keyInfo.ExpiresAt) {
		return nil, errors.New("API key expired")
	}

	return &AuthContext{
		Authenticated: true,
		UserID:        keyInfo.UserID,
		Permissions:   keyInfo.Permissions,
		TokenID:       hashString(key),
	}, nil
}

// Type returns the authenticator type
func (aka *APIKeyAuthenticator) Type() AuthType {
	return AuthTypeAPIKey
}

// ============================================================================
// Basic Authentication
// ============================================================================

// BasicAuthenticator implements HTTP basic authentication
type BasicAuthenticator struct {
	users map[string]*BasicAuthUser
	mutex sync.RWMutex
}

// BasicAuthUser contains basic auth user information
type BasicAuthUser struct {
	Username     string
	Password     string
	PasswordHash string
	Roles        []string
}

// NewBasicAuthenticator creates a new basic authenticator
func NewBasicAuthenticator(username, password string) *BasicAuthenticator {
	ba := &BasicAuthenticator{
		users: make(map[string]*BasicAuthUser),
	}

	// Add the configured user
	ba.AddUser(&BasicAuthUser{
		Username:     username,
		Password:     password,
		PasswordHash: hashPassword(password),
	})

	return ba
}

// AddUser adds a user for basic auth
func (ba *BasicAuthenticator) AddUser(user *BasicAuthUser) {
	ba.mutex.Lock()
	defer ba.mutex.Unlock()

	if user.PasswordHash == "" && user.Password != "" {
		user.PasswordHash = hashPassword(user.Password)
		user.Password = "" // Clear plaintext password
	}

	ba.users[user.Username] = user
}

// Authenticate validates basic auth credentials
func (ba *BasicAuthenticator) Authenticate(r *http.Request) (*AuthContext, error) {
	username, password, ok := r.BasicAuth()
	if !ok {
		return nil, errors.New("basic auth credentials missing")
	}

	ba.mutex.RLock()
	user, exists := ba.users[username]
	ba.mutex.RUnlock()

	if !exists {
		return nil, errors.New("invalid username")
	}

	// Verify password
	if !verifyPassword(password, user.PasswordHash) {
		return nil, errors.New("invalid password")
	}

	return &AuthContext{
		Authenticated: true,
		UserID:        username,
		Username:      username,
		Roles:         user.Roles,
	}, nil
}

// Type returns the authenticator type
func (ba *BasicAuthenticator) Type() AuthType {
	return AuthTypeBasic
}

// ============================================================================
// JWT Authentication
// ============================================================================

// JWTAuthenticator implements JWT authentication
type JWTAuthenticator struct {
	config    JWTConfig
	parser    *jwt.Parser
	validator jwt.Keyfunc
}

// NewJWTAuthenticator creates a new JWT authenticator
func NewJWTAuthenticator(config JWTConfig) (*JWTAuthenticator, error) {
	ja := &JWTAuthenticator{
		config: config,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{config.Algorithm}),
		),
	}

	// Set up key validation function
	ja.validator = func(token *jwt.Token) (interface{}, error) {
		// Verify algorithm
		if token.Method.Alg() != config.Algorithm {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Method.Alg())
		}

		// Return the key for validation
		switch config.Algorithm {
		case "HS256", "HS384", "HS512":
			return []byte(config.SigningKey), nil
		case "RS256", "RS384", "RS512":
			// For RSA, parse the public key
			return jwt.ParseRSAPublicKeyFromPEM([]byte(config.SigningKey))
		default:
			return nil, fmt.Errorf("unsupported algorithm: %s", config.Algorithm)
		}
	}

	return ja, nil
}

// Authenticate validates JWT token
func (ja *JWTAuthenticator) Authenticate(r *http.Request) (*AuthContext, error) {
	// Extract token from header
	tokenString := extractJWTFromRequest(r)
	if tokenString == "" {
		return nil, errors.New("JWT token missing")
	}

	// Parse and validate token
	token, err := ja.parser.Parse(tokenString, ja.validator)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("JWT token is invalid")
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid JWT claims")
	}

	// JWT v5 validates claims during Parse, so no separate validation needed

	// Build auth context from claims
	ctx := &AuthContext{
		Authenticated: true,
		Metadata:      make(map[string]interface{}),
	}

	// Extract standard claims
	if sub, ok := claims["sub"].(string); ok {
		ctx.UserID = sub
	}

	if username, ok := claims["username"].(string); ok {
		ctx.Username = username
	}

	if roles, ok := claims["roles"].([]interface{}); ok {
		for _, role := range roles {
			if r, ok := role.(string); ok {
				ctx.Roles = append(ctx.Roles, r)
			}
		}
	}

	if perms, ok := claims["permissions"].([]interface{}); ok {
		for _, perm := range perms {
			if p, ok := perm.(string); ok {
				ctx.Permissions = append(ctx.Permissions, p)
			}
		}
	}

	if exp, ok := claims["exp"].(float64); ok {
		ctx.ExpiresAt = time.Unix(int64(exp), 0)
	}

	if jti, ok := claims["jti"].(string); ok {
		ctx.TokenID = jti
	}

	// Store additional claims
	for key, value := range claims {
		if key != "sub" && key != "username" && key != "roles" && key != "permissions" {
			ctx.Metadata[key] = value
		}
	}

	return ctx, nil
}

// Type returns the authenticator type
func (ja *JWTAuthenticator) Type() AuthType {
	return AuthTypeJWT
}

// extractJWTFromRequest extracts JWT token from request
func extractJWTFromRequest(r *http.Request) string {
	// Try Authorization header first
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	// Try query parameter
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}

	// Try cookie
	if cookie, err := r.Cookie("jwt"); err == nil {
		return cookie.Value
	}

	return ""
}

// ============================================================================
// OAuth2 Authentication
// ============================================================================

// OAuth2Authenticator implements OAuth2 authentication
type OAuth2Authenticator struct {
	config     OAuth2Config
	tokenCache *TokenCache
	httpClient *http.Client
	mutex      sync.RWMutex
}

// TokenCache caches OAuth2 tokens
type TokenCache struct {
	tokens map[string]*CachedToken
	mutex  sync.RWMutex
}

// CachedToken represents a cached OAuth2 token
type CachedToken struct {
	Token     string
	ExpiresAt time.Time
	UserInfo  map[string]interface{}
}

// NewOAuth2Authenticator creates a new OAuth2 authenticator
func NewOAuth2Authenticator(config OAuth2Config) (*OAuth2Authenticator, error) {
	return &OAuth2Authenticator{
		config:     config,
		tokenCache: &TokenCache{tokens: make(map[string]*CachedToken)},
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
					CipherSuites: []uint16{
						tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
						tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
						tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
						tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
						tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					},
				},
			},
		},
	}, nil
}

// Authenticate validates OAuth2 token
func (oa *OAuth2Authenticator) Authenticate(r *http.Request) (*AuthContext, error) {
	// Extract token
	token := extractOAuth2Token(r)
	if token == "" {
		return nil, errors.New("OAuth2 token missing")
	}

	// Check cache
	if cached := oa.tokenCache.Get(token); cached != nil {
		if time.Now().Before(cached.ExpiresAt) {
			return oa.buildAuthContext(cached), nil
		}
		oa.tokenCache.Remove(token)
	}

	// Validate token with OAuth2 provider
	userInfo, expiresAt, err := oa.validateToken(token)
	if err != nil {
		return nil, fmt.Errorf("OAuth2 token validation failed: %w", err)
	}

	// Cache the result
	oa.tokenCache.Set(token, &CachedToken{
		Token:     token,
		ExpiresAt: expiresAt,
		UserInfo:  userInfo,
	})

	return oa.buildAuthContext(&CachedToken{
		Token:     token,
		ExpiresAt: expiresAt,
		UserInfo:  userInfo,
	}), nil
}

// validateToken validates the token with the OAuth2 provider
func (oa *OAuth2Authenticator) validateToken(token string) (map[string]interface{}, time.Time, error) {
	// This is a simplified implementation
	// In production, you would validate against the OAuth2 provider's introspection endpoint

	// For now, return a placeholder
	userInfo := map[string]interface{}{
		"sub":      "oauth2_user",
		"username": "oauth2_user",
	}

	expiresAt := time.Now().Add(time.Hour)

	return userInfo, expiresAt, nil
}

// buildAuthContext builds auth context from cached token
func (oa *OAuth2Authenticator) buildAuthContext(cached *CachedToken) *AuthContext {
	ctx := &AuthContext{
		Authenticated: true,
		TokenID:       hashString(cached.Token),
		ExpiresAt:     cached.ExpiresAt,
		Metadata:      cached.UserInfo,
	}

	// Extract common fields
	if sub, ok := cached.UserInfo["sub"].(string); ok {
		ctx.UserID = sub
	}

	if username, ok := cached.UserInfo["username"].(string); ok {
		ctx.Username = username
	}

	return ctx
}

// Type returns the authenticator type
func (oa *OAuth2Authenticator) Type() AuthType {
	return AuthTypeOAuth2
}

// TokenCache methods

// Get retrieves a token from cache
func (tc *TokenCache) Get(token string) *CachedToken {
	tc.mutex.RLock()
	defer tc.mutex.RUnlock()
	return tc.tokens[token]
}

// Set adds a token to cache
func (tc *TokenCache) Set(token string, cached *CachedToken) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	tc.tokens[token] = cached
}

// Remove removes a token from cache
func (tc *TokenCache) Remove(token string) {
	tc.mutex.Lock()
	defer tc.mutex.Unlock()
	delete(tc.tokens, token)
}

// extractOAuth2Token extracts OAuth2 token from request
func extractOAuth2Token(r *http.Request) string {
	// Try Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	// Try access_token query parameter
	return r.URL.Query().Get("access_token")
}

// ============================================================================
// Rate Limiting
// ============================================================================

// RateLimiter implements rate limiting functionality
type RateLimiter struct {
	config           RateLimitConfig
	globalLimiter    *rate.Limiter
	clientLimiters   map[string]*rate.Limiter
	endpointLimiters map[string]*rate.Limiter
	mutex            sync.Mutex
	logger           *zap.Logger
	cleanupTicker    *time.Ticker
	stopCh           chan struct{}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimitConfig, logger *zap.Logger) *RateLimiter {
	rl := &RateLimiter{
		config:           config,
		globalLimiter:    rate.NewLimiter(rate.Limit(config.RequestsPerSecond), config.BurstSize),
		clientLimiters:   make(map[string]*rate.Limiter),
		endpointLimiters: make(map[string]*rate.Limiter),
		logger:           logger,
		cleanupTicker:    time.NewTicker(5 * time.Minute),
		stopCh:           make(chan struct{}),
	}

	// Initialize endpoint limiters
	for endpoint, endpointConfig := range config.PerEndpoint {
		rl.endpointLimiters[endpoint] = rate.NewLimiter(
			rate.Limit(endpointConfig.RequestsPerSecond),
			endpointConfig.BurstSize,
		)
	}

	// Start cleanup routine
	go rl.cleanupRoutine()

	return rl
}

// Allow checks if a request is allowed
func (rl *RateLimiter) Allow(clientID, endpoint string) bool {
	// Check global limit
	if !rl.globalLimiter.Allow() {
		return false
	}

	// Check endpoint-specific limit
	if endpointLimiter, exists := rl.endpointLimiters[endpoint]; exists {
		if !endpointLimiter.Allow() {
			return false
		}
	}

	// Check per-client limit
	if rl.config.PerClient.Enabled {
		limiter := rl.getClientLimiter(clientID)
		if !limiter.Allow() {
			return false
		}
	}

	return true
}

// getClientLimiter gets or creates a limiter for a client
func (rl *RateLimiter) getClientLimiter(clientID string) *rate.Limiter {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	if limiter, exists := rl.clientLimiters[clientID]; exists {
		return limiter
	}

	limiter := rate.NewLimiter(
		rate.Limit(rl.config.PerClient.RequestsPerSecond),
		rl.config.PerClient.BurstSize,
	)
	rl.clientLimiters[clientID] = limiter

	return limiter
}

// cleanupRoutine periodically cleans up old client limiters
func (rl *RateLimiter) cleanupRoutine() {
	for {
		select {
		case <-rl.cleanupTicker.C:
			rl.cleanup()
		case <-rl.stopCh:
			rl.cleanupTicker.Stop()
			return
		}
	}
}

// cleanup removes inactive client limiters
func (rl *RateLimiter) cleanup() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	// In a production system, you would track last access time
	// and remove limiters that haven't been used recently
	// For now, we'll limit the map size
	maxClients := 10000
	if len(rl.clientLimiters) > maxClients {
		// Remove oldest entries (simplified implementation)
		count := 0
		for clientID := range rl.clientLimiters {
			delete(rl.clientLimiters, clientID)
			count++
			if len(rl.clientLimiters) <= maxClients/2 {
				break
			}
		}
		rl.logger.Info("cleaned up client limiters", zap.Int("removed", count))
	}
}

// Stop stops the rate limiter
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// ============================================================================
// Request Validation
// ============================================================================

// RequestValidator validates incoming requests
type RequestValidator struct {
	config  ValidationConfig
	logger  *zap.Logger
	schemas map[string]interface{} // JSON schemas for validation
}

// NewRequestValidator creates a new request validator
func NewRequestValidator(config ValidationConfig, logger *zap.Logger) *RequestValidator {
	return &RequestValidator{
		config:  config,
		logger:  logger,
		schemas: make(map[string]interface{}),
	}
}

// Validate validates a request
func (rv *RequestValidator) Validate(r *http.Request) error {
	// Check request size
	if r.ContentLength > rv.config.MaxRequestSize {
		return fmt.Errorf("request size %d exceeds maximum %d", r.ContentLength, rv.config.MaxRequestSize)
	}

	// Check header size
	headerSize := rv.calculateHeaderSize(r)
	if headerSize > rv.config.MaxHeaderSize {
		return fmt.Errorf("header size %d exceeds maximum %d", headerSize, rv.config.MaxHeaderSize)
	}

	// Validate content type
	if len(rv.config.AllowedContentTypes) > 0 && r.Method != "GET" {
		contentType := r.Header.Get("Content-Type")
		if !rv.isAllowedContentType(contentType) {
			return fmt.Errorf("content type %s not allowed", contentType)
		}
	}

	// Validate URL
	if err := rv.validateURL(r.URL); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Validate headers
	if err := rv.validateHeaders(r.Header); err != nil {
		return fmt.Errorf("invalid headers: %w", err)
	}

	return nil
}

// calculateHeaderSize calculates the total size of headers
func (rv *RequestValidator) calculateHeaderSize(r *http.Request) int64 {
	size := int64(0)
	for key, values := range r.Header {
		size += int64(len(key))
		for _, value := range values {
			size += int64(len(value))
		}
	}
	return size
}

// isAllowedContentType checks if content type is allowed
func (rv *RequestValidator) isAllowedContentType(contentType string) bool {
	// Extract base content type (without parameters)
	parts := strings.Split(contentType, ";")
	baseType := strings.TrimSpace(parts[0])

	for _, allowed := range rv.config.AllowedContentTypes {
		if strings.EqualFold(allowed, baseType) {
			return true
		}
	}
	return false
}

// validateURL validates the request URL
func (rv *RequestValidator) validateURL(u *url.URL) error {
	// Check for suspicious patterns
	if strings.Contains(u.Path, "..") {
		return errors.New("path traversal detected")
	}

	// Validate query parameters
	for key, values := range u.Query() {
		// Check key length
		if len(key) > 256 {
			return fmt.Errorf("query parameter key too long: %s", key)
		}

		// Check value lengths
		for _, value := range values {
			if len(value) > 4096 {
				return fmt.Errorf("query parameter value too long for key: %s", key)
			}

			// Check for SQL injection patterns
			if rv.containsSQLInjectionPattern(value) {
				return fmt.Errorf("potential SQL injection in query parameter: %s", key)
			}

			// Check for XSS patterns
			if rv.containsXSSPattern(value) {
				return fmt.Errorf("potential XSS in query parameter: %s", key)
			}
		}
	}

	return nil
}

// validateHeaders validates request headers
func (rv *RequestValidator) validateHeaders(headers http.Header) error {
	// List of potentially dangerous headers
	dangerousHeaders := []string{
		"X-Forwarded-Host",
		"X-Original-URL",
		"X-Rewrite-URL",
	}

	for _, header := range dangerousHeaders {
		if value := headers.Get(header); value != "" {
			// Log suspicious header
			rv.logger.Warn("potentially dangerous header detected",
				zap.String("header", header),
				zap.String("value", value),
			)
		}
	}

	// Check for header injection
	for key, values := range headers {
		if strings.ContainsAny(key, "\r\n") {
			return fmt.Errorf("header injection detected in key: %s", key)
		}

		for _, value := range values {
			if strings.ContainsAny(value, "\r\n") {
				return fmt.Errorf("header injection detected in value for key: %s", key)
			}
		}
	}

	return nil
}

// containsSQLInjectionPattern checks for common SQL injection patterns
func (rv *RequestValidator) containsSQLInjectionPattern(value string) bool {
	// Common SQL injection patterns (comprehensive)
	patterns := []string{
		"(?i)(union.*select)",
		"(?i)(select.*from)",
		"(?i)(insert.*into)",
		"(?i)(delete.*from)",
		"(?i)(drop.*table)",
		"(?i)(script.*>)",
		"(?i)(<.*iframe)",
		"(?i)(.*'.*or.*'.*)",      // Single quote OR attacks
		"(?i)(.*\".*or.*\".*)",    // Double quote OR attacks
		"(?i)(.*'.*union.*)",      // Single quote UNION attacks
		"(?i)(.*\".*union.*)",     // Double quote UNION attacks
		"(?i)(.*--.*)",            // SQL comment attacks
		"(?i)(/\\*.*\\*/)",        // Multi-line comment attacks
		"(?i)(.*'.*and.*'.*)",     // Single quote AND attacks
		"(?i)(.*\".*and.*\".*)",   // Double quote AND attacks
		"(?i)(.*'.*=.*'.*)",       // Single quote equality attacks
		"(?i)(.*\".*=.*\".*)",     // Double quote equality attacks
		"(?i)(.*\\+.*or.*\\+.*)",  // URL encoded OR attacks
		"(?i)(.*%27.*or.*%27.*)",  // URL encoded single quote OR
		"(?i)(.*%22.*or.*%22.*)",  // URL encoded double quote OR
		"(?i)(.*1.*=.*1.*)",       // Common tautology
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, value); matched {
			return true
		}
	}

	return false
}

// containsXSSPattern checks for common XSS patterns
func (rv *RequestValidator) containsXSSPattern(value string) bool {
	// Common XSS patterns (simplified)
	patterns := []string{
		"<script",
		"javascript:",
		"onerror=",
		"onload=",
		"onclick=",
		"<iframe",
		"<object",
		"<embed",
		"<link",
		"eval\\(",
	}

	lowerValue := strings.ToLower(value)
	for _, pattern := range patterns {
		if strings.Contains(lowerValue, pattern) {
			return true
		}
	}

	return false
}

// ============================================================================
// Security Headers
// ============================================================================

// SecurityHeaders manages security-related HTTP headers
type SecurityHeaders struct {
	corsConfig CORSConfig
	cspPolicy  string
}

// NewSecurityHeaders creates a new security headers manager
func NewSecurityHeaders(corsConfig CORSConfig) *SecurityHeaders {
	return &SecurityHeaders{
		corsConfig: corsConfig,
		cspPolicy:  buildDefaultCSP(),
	}
}

// Apply applies security headers to the response
func (sh *SecurityHeaders) Apply(w http.ResponseWriter, r *http.Request) {
	// Apply CORS headers if enabled
	if sh.corsConfig.Enabled {
		sh.applyCORS(w, r)
	}

	// Apply security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Content-Security-Policy", sh.cspPolicy)

	// HSTS for HTTPS connections
	if r.TLS != nil {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
}

// applyCORS applies CORS headers
func (sh *SecurityHeaders) applyCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")

	// Check if origin is allowed
	if sh.isOriginAllowed(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)

		if sh.corsConfig.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(sh.corsConfig.AllowedMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(sh.corsConfig.AllowedHeaders, ", "))
			w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", int(sh.corsConfig.MaxAge.Seconds())))
		}

		if len(sh.corsConfig.ExposedHeaders) > 0 {
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(sh.corsConfig.ExposedHeaders, ", "))
		}
	}
}

// isOriginAllowed checks if an origin is allowed
func (sh *SecurityHeaders) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range sh.corsConfig.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}

		// Support wildcard subdomains
		if strings.HasPrefix(allowed, "*.") {
			domain := strings.TrimPrefix(allowed, "*.")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}

	return false
}

// buildDefaultCSP builds a default Content Security Policy
func buildDefaultCSP() string {
	policies := []string{
		"default-src 'self'",
		"script-src 'self' 'unsafe-inline' 'unsafe-eval'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: https:",
		"font-src 'self'",
		"connect-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
	}

	return strings.Join(policies, "; ")
}

// ============================================================================
// Audit Logging
// ============================================================================

// AuditLogger logs security-related events
type AuditLogger struct {
	logger   *zap.Logger
	authType AuthType
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger *zap.Logger, authType AuthType) *AuditLogger {
	return &AuditLogger{
		logger:   logger,
		authType: authType,
	}
}

// LogAuthSuccess logs successful authentication
func (al *AuditLogger) LogAuthSuccess(r *http.Request, ctx *AuthContext) {
	al.logger.Info("authentication successful",
		zap.String("auth_type", string(al.authType)),
		zap.String("user_id", ctx.UserID),
		zap.String("client_ip", getClientIP(r)),
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
	)
}

// LogAuthFailure logs failed authentication
func (al *AuditLogger) LogAuthFailure(r *http.Request, err error) {
	al.logger.Warn("authentication failed",
		zap.String("auth_type", string(al.authType)),
		zap.Error(err),
		zap.String("client_ip", getClientIP(r)),
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
	)
}

// LogRateLimitExceeded logs rate limit exceeded events
func (al *AuditLogger) LogRateLimitExceeded(r *http.Request, clientID string) {
	al.logger.Warn("rate limit exceeded",
		zap.String("client_id", clientID),
		zap.String("client_ip", getClientIP(r)),
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
	)
}

// LogValidationFailure logs request validation failures
func (al *AuditLogger) LogValidationFailure(r *http.Request, err error) {
	al.logger.Warn("request validation failed",
		zap.Error(err),
		zap.String("client_ip", getClientIP(r)),
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
		zap.Int64("content_length", r.ContentLength),
	)
}

// LogSecurityEvent logs generic security events
func (al *AuditLogger) LogSecurityEvent(event string, details map[string]interface{}) {
	fields := []zap.Field{
		zap.String("event", event),
		zap.Time("timestamp", time.Now()),
	}

	for key, value := range details {
		fields = append(fields, zap.Any(key, value))
	}

	al.logger.Info("security event", fields...)
}

// ============================================================================
// Security Metrics
// ============================================================================

// SecurityMetrics tracks security-related metrics
type SecurityMetrics struct {
	authAttempts       atomic.Int64
	authSuccesses      atomic.Int64
	authFailures       atomic.Int64
	rateLimitHits      atomic.Int64
	validationFailures atomic.Int64
}

// NewSecurityMetrics creates new security metrics
func NewSecurityMetrics() *SecurityMetrics {
	return &SecurityMetrics{}
}

// IncrementAuthAttempts increments authentication attempts
func (sm *SecurityMetrics) IncrementAuthAttempts() {
	sm.authAttempts.Add(1)
}

// IncrementAuthSuccesses increments successful authentications
func (sm *SecurityMetrics) IncrementAuthSuccesses() {
	sm.authSuccesses.Add(1)
}

// IncrementAuthFailures increments failed authentications
func (sm *SecurityMetrics) IncrementAuthFailures() {
	sm.authFailures.Add(1)
}

// IncrementRateLimitHits increments rate limit hits
func (sm *SecurityMetrics) IncrementRateLimitHits() {
	sm.rateLimitHits.Add(1)
}

// IncrementValidationFailures increments validation failures
func (sm *SecurityMetrics) IncrementValidationFailures() {
	sm.validationFailures.Add(1)
}

// GetMetrics returns current metrics
func (sm *SecurityMetrics) GetMetrics() map[string]int64 {
	return map[string]int64{
		"auth_attempts":       sm.authAttempts.Load(),
		"auth_successes":      sm.authSuccesses.Load(),
		"auth_failures":       sm.authFailures.Load(),
		"rate_limit_hits":     sm.rateLimitHits.Load(),
		"validation_failures": sm.validationFailures.Load(),
	}
}

// ============================================================================
// Utility Functions
// ============================================================================

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to remote address
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

// hashString creates a SHA256 hash of a string
func hashString(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// hashPassword creates a secure hash of a password
func hashPassword(password string) string {
	// In production, use bcrypt or argon2
	// This is a simplified implementation
	salt := "static_salt_for_demo" // In production, use a random salt
	h := hmac.New(sha256.New, []byte(salt))
	h.Write([]byte(password))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// verifyPassword verifies a password against a hash
func verifyPassword(password, hash string) bool {
	expectedHash := hashPassword(password)
	return subtle.ConstantTimeCompare([]byte(hash), []byte(expectedHash)) == 1
}

// ============================================================================
// Security Policy Levels
// ============================================================================

// SecurityLevel defines security policy levels
type SecurityLevel int

const (
	SecurityLevelMinimal SecurityLevel = iota
	SecurityLevelBasic
	SecurityLevelStandard
	SecurityLevelHigh
	SecurityLevelMaximum
)

// GetSecurityConfig returns a security configuration for the given level
func GetSecurityConfig(level SecurityLevel) SecurityConfig {
	switch level {
	case SecurityLevelMinimal:
		return minimalSecurityConfig()
	case SecurityLevelBasic:
		return basicSecurityConfig()
	case SecurityLevelStandard:
		return standardSecurityConfig()
	case SecurityLevelHigh:
		return highSecurityConfig()
	case SecurityLevelMaximum:
		return maximumSecurityConfig()
	default:
		return standardSecurityConfig()
	}
}

// minimalSecurityConfig returns minimal security settings
func minimalSecurityConfig() SecurityConfig {
	return SecurityConfig{
		Auth: AuthConfig{
			Type: AuthTypeNone,
		},
		RateLimit: RateLimitConfig{
			Enabled: false,
		},
		Validation: ValidationConfig{
			Enabled:             true,
			MaxRequestSize:      10 * 1024 * 1024, // 10MB
			MaxHeaderSize:       1024 * 1024,      // 1MB
			AllowedContentTypes: []string{"application/json", "text/plain"},
		},
	}
}

// basicSecurityConfig returns basic security settings
func basicSecurityConfig() SecurityConfig {
	config := minimalSecurityConfig()
	config.Auth.Type = AuthTypeAPIKey
	config.RateLimit = RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 100,
		BurstSize:         200,
	}
	return config
}

// standardSecurityConfig returns standard security settings
func standardSecurityConfig() SecurityConfig {
	config := basicSecurityConfig()
	config.RateLimit.RequestsPerSecond = 50
	config.RateLimit.BurstSize = 100
	config.RateLimit.PerClient = RateLimitPerClientConfig{
		Enabled:              true,
		RequestsPerSecond:    10,
		BurstSize:            20,
		IdentificationMethod: "ip",
	}
	config.CORS = CORSConfig{
		Enabled:          true,
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
		MaxAge:           24 * time.Hour,
	}
	return config
}

// highSecurityConfig returns high security settings
func highSecurityConfig() SecurityConfig {
	config := standardSecurityConfig()
	config.Auth.Type = AuthTypeJWT
	config.RateLimit.RequestsPerSecond = 20
	config.RateLimit.BurstSize = 40
	config.RateLimit.PerClient.RequestsPerSecond = 5
	config.RateLimit.PerClient.BurstSize = 10
	config.Validation.MaxRequestSize = 1 * 1024 * 1024 // 1MB
	config.Validation.MaxHeaderSize = 100 * 1024       // 100KB
	config.CORS.AllowedOrigins = []string{"https://trusted-domain.com"}
	config.CORS.AllowCredentials = true
	return config
}

// maximumSecurityConfig returns maximum security settings
func maximumSecurityConfig() SecurityConfig {
	config := highSecurityConfig()
	config.RateLimit.RequestsPerSecond = 10
	config.RateLimit.BurstSize = 20
	config.RateLimit.PerClient.RequestsPerSecond = 2
	config.RateLimit.PerClient.BurstSize = 5
	config.Validation.MaxRequestSize = 100 * 1024 // 100KB
	config.Validation.MaxHeaderSize = 10 * 1024   // 10KB
	config.Validation.AllowedContentTypes = []string{"application/json"}
	config.RequestSigning = RequestSigningConfig{
		Enabled:          true,
		Algorithm:        "HMAC-SHA256",
		SignedHeaders:    []string{"host", "date", "content-type"},
		SignatureHeader:  "X-Signature",
		TimestampHeader:  "X-Timestamp",
		MaxTimestampSkew: 5 * time.Minute,
	}
	return config
}

// ============================================================================
// Request Signing
// ============================================================================

// RequestSigner signs and verifies HTTP requests
type RequestSigner struct {
	config RequestSigningConfig
	key    []byte
}

// NewRequestSigner creates a new request signer
func NewRequestSigner(config RequestSigningConfig) *RequestSigner {
	return &RequestSigner{
		config: config,
		key:    []byte(config.SigningKey),
	}
}

// SignRequest signs an HTTP request
func (rs *RequestSigner) SignRequest(r *http.Request) error {
	// Add timestamp
	timestamp := time.Now().Unix()
	r.Header.Set(rs.config.TimestampHeader, fmt.Sprintf("%d", timestamp))

	// Build string to sign
	stringToSign := rs.buildStringToSign(r)

	// Generate signature
	h := hmac.New(sha256.New, rs.key)
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Add signature header
	r.Header.Set(rs.config.SignatureHeader, signature)

	return nil
}

// VerifyRequest verifies a signed request
func (rs *RequestSigner) VerifyRequest(r *http.Request) error {
	// Check timestamp
	timestampStr := r.Header.Get(rs.config.TimestampHeader)
	if timestampStr == "" {
		return errors.New("missing timestamp header")
	}

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}

	// Check timestamp skew
	now := time.Now().Unix()
	skew := now - timestamp
	if skew < 0 {
		skew = -skew
	}

	if skew > int64(rs.config.MaxTimestampSkew.Seconds()) {
		return fmt.Errorf("timestamp skew too large: %d seconds", skew)
	}

	// Get signature
	signature := r.Header.Get(rs.config.SignatureHeader)
	if signature == "" {
		return errors.New("missing signature header")
	}

	// Build string to sign
	stringToSign := rs.buildStringToSign(r)

	// Verify signature
	h := hmac.New(sha256.New, rs.key)
	h.Write([]byte(stringToSign))
	expectedSignature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(signature), []byte(expectedSignature)) != 1 {
		return errors.New("invalid signature")
	}

	return nil
}

// buildStringToSign builds the string to sign from request
func (rs *RequestSigner) buildStringToSign(r *http.Request) string {
	parts := []string{
		r.Method,
		r.URL.Path,
	}

	// Add query string if present
	if r.URL.RawQuery != "" {
		parts = append(parts, r.URL.RawQuery)
	}

	// Add signed headers
	for _, header := range rs.config.SignedHeaders {
		value := r.Header.Get(header)
		parts = append(parts, fmt.Sprintf("%s:%s", strings.ToLower(header), value))
	}

	return strings.Join(parts, "\n")
}

