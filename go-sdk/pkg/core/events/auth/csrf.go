package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CSRFConfig configures CSRF protection
type CSRFConfig struct {
	// Secret key for HMAC signing
	SecretKey []byte

	// TokenHeader is the header name for CSRF tokens
	TokenHeader string

	// TokenField is the form field name for CSRF tokens
	TokenField string

	// CookieName is the cookie name for CSRF tokens
	CookieName string

	// TokenExpiration is how long CSRF tokens are valid
	TokenExpiration time.Duration

	// SecureOnly sets secure flag on cookies
	SecureOnly bool

	// SameSite sets SameSite attribute on cookies
	SameSite http.SameSite

	// SkipMethods lists HTTP methods to skip CSRF protection
	SkipMethods []string

	// TrustedOrigins lists trusted origins for CSRF protection
	TrustedOrigins []string
}

// DefaultCSRFConfig returns default CSRF configuration
func DefaultCSRFConfig() *CSRFConfig {
	return &CSRFConfig{
		TokenHeader:     "X-CSRF-Token",
		TokenField:      "csrf_token",
		CookieName:      "csrf_token",
		TokenExpiration: 24 * time.Hour,
		SecureOnly:      true,
		SameSite:        http.SameSiteStrictMode,
		SkipMethods:     []string{"GET", "HEAD", "OPTIONS", "TRACE"},
		TrustedOrigins:  []string{},
	}
}

// CSRFManager handles CSRF token generation and validation
type CSRFManager struct {
	config *CSRFConfig
	tokens map[string]*CSRFToken
	mutex  sync.RWMutex
}

// CSRFToken represents a CSRF token with metadata
type CSRFToken struct {
	Token     string
	UserID    string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// NewCSRFManager creates a new CSRF manager
func NewCSRFManager(config *CSRFConfig) (*CSRFManager, error) {
	if config == nil {
		config = DefaultCSRFConfig()
	}

	if len(config.SecretKey) == 0 {
		// Generate a random secret key if none provided
		config.SecretKey = make([]byte, 32)
		if _, err := rand.Read(config.SecretKey); err != nil {
			return nil, fmt.Errorf("failed to generate CSRF secret key: %w", err)
		}
	}

	return &CSRFManager{
		config: config,
		tokens: make(map[string]*CSRFToken),
	}, nil
}

// GenerateToken generates a new CSRF token for a user
func (c *CSRFManager) GenerateToken(userID string) (string, error) {
	// Generate random token data
	tokenData := make([]byte, 32)
	if _, err := rand.Read(tokenData); err != nil {
		return "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}

	// Create token with timestamp
	now := time.Now()
	expiresAt := now.Add(c.config.TokenExpiration)

	// Create token payload
	payload := fmt.Sprintf("%s|%s|%d", userID, base64.RawURLEncoding.EncodeToString(tokenData), now.Unix())

	// Sign the payload
	mac := hmac.New(sha256.New, c.config.SecretKey)
	mac.Write([]byte(payload))
	signature := mac.Sum(nil)

	// Create final token
	token := fmt.Sprintf("%s.%s", payload, base64.RawURLEncoding.EncodeToString(signature))

	// Store token for validation
	c.mutex.Lock()
	c.tokens[token] = &CSRFToken{
		Token:     token,
		UserID:    userID,
		IssuedAt:  now,
		ExpiresAt: expiresAt,
	}
	c.mutex.Unlock()

	return token, nil
}

// ValidateToken validates a CSRF token
func (c *CSRFManager) ValidateToken(token, userID string) error {
	if token == "" {
		return errors.New("CSRF token is required")
	}

	// Parse token
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return errors.New("invalid CSRF token format")
	}

	payload := parts[0]
	signature := parts[1]

	// Verify signature
	mac := hmac.New(sha256.New, c.config.SecretKey)
	mac.Write([]byte(payload))
	expectedSignature := mac.Sum(nil)

	decodedSignature, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return errors.New("invalid CSRF token signature")
	}

	if subtle.ConstantTimeCompare(decodedSignature, expectedSignature) != 1 {
		return errors.New("invalid CSRF token signature")
	}

	// Parse payload
	payloadParts := strings.Split(payload, "|")
	if len(payloadParts) != 3 {
		return errors.New("invalid CSRF token payload")
	}

	tokenUserID := payloadParts[0]
	tokenData := payloadParts[1]
	issuedAtStr := payloadParts[2]

	// Verify user ID
	if tokenUserID != userID {
		return errors.New("CSRF token user ID mismatch")
	}

	// Verify token hasn't expired
	c.mutex.RLock()
	storedToken, exists := c.tokens[token]
	c.mutex.RUnlock()

	if !exists {
		return errors.New("CSRF token not found")
	}

	if time.Now().After(storedToken.ExpiresAt) {
		c.mutex.Lock()
		delete(c.tokens, token)
		c.mutex.Unlock()
		return errors.New("CSRF token has expired")
	}

	// Additional validation can be added here
	_ = tokenData
	_ = issuedAtStr

	return nil
}

// ExtractTokenFromRequest extracts CSRF token from HTTP request
func (c *CSRFManager) ExtractTokenFromRequest(r *http.Request) string {
	// Try header first
	if token := r.Header.Get(c.config.TokenHeader); token != "" {
		return token
	}

	// Try form field
	if token := r.FormValue(c.config.TokenField); token != "" {
		return token
	}

	// Try cookie
	if cookie, err := r.Cookie(c.config.CookieName); err == nil {
		return cookie.Value
	}

	return ""
}

// SetTokenCookie sets a CSRF token as a cookie
func (c *CSRFManager) SetTokenCookie(w http.ResponseWriter, token string) {
	cookie := &http.Cookie{
		Name:     c.config.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.config.SecureOnly,
		SameSite: c.config.SameSite,
		Expires:  time.Now().Add(c.config.TokenExpiration),
	}

	http.SetCookie(w, cookie)
}

// Middleware returns a CSRF protection middleware
func (c *CSRFManager) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip CSRF protection for certain methods
			if c.shouldSkipMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			// Extract user ID from context (set by authentication middleware)
			userID := c.getUserIDFromContext(r)
			if userID == "" {
				c.writeCSRFError(w, "authentication required for CSRF protection")
				return
			}

			// Validate origin
			if !c.isOriginTrusted(r) {
				c.writeCSRFError(w, "untrusted origin")
				return
			}

			// Extract and validate CSRF token
			token := c.ExtractTokenFromRequest(r)
			if err := c.ValidateToken(token, userID); err != nil {
				c.writeCSRFError(w, fmt.Sprintf("CSRF validation failed: %v", err))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// shouldSkipMethod checks if CSRF protection should be skipped for this method
func (c *CSRFManager) shouldSkipMethod(method string) bool {
	for _, skipMethod := range c.config.SkipMethods {
		if strings.EqualFold(method, skipMethod) {
			return true
		}
	}
	return false
}

// isOriginTrusted checks if the request origin is trusted
func (c *CSRFManager) isOriginTrusted(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// No origin header, check referer
		referer := r.Header.Get("Referer")
		if referer == "" {
			return false
		}
		origin = referer
	}

	// If no trusted origins configured, allow all
	if len(c.config.TrustedOrigins) == 0 {
		return true
	}

	// Check if origin is in trusted list
	for _, trusted := range c.config.TrustedOrigins {
		if strings.HasPrefix(origin, trusted) {
			return true
		}
	}

	return false
}

// getUserIDFromContext extracts user ID from request context
func (c *CSRFManager) getUserIDFromContext(r *http.Request) string {
	// This would typically extract the user ID from the authentication context
	// For now, we'll look for a header or context value
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID
	}

	// Check context for auth information
	if authCtx := r.Context().Value("auth_context"); authCtx != nil {
		if ctx, ok := authCtx.(*AuthContext); ok {
			return ctx.UserID
		}
	}

	return ""
}

// writeCSRFError writes a CSRF error response
func (c *CSRFManager) writeCSRFError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)

	response := map[string]interface{}{
		"error":     "CSRF protection failed",
		"message":   message,
		"timestamp": time.Now().Unix(),
	}

	// Write JSON response (simplified)
	fmt.Fprintf(w, `{"error": "%s", "message": "%s", "timestamp": %d}`,
		response["error"], response["message"], response["timestamp"])
}

// CleanupExpiredTokens removes expired CSRF tokens
func (c *CSRFManager) CleanupExpiredTokens() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	for token, tokenData := range c.tokens {
		if now.After(tokenData.ExpiresAt) {
			delete(c.tokens, token)
		}
	}
}

// GetTokenCount returns the number of active CSRF tokens
func (c *CSRFManager) GetTokenCount() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.tokens)
}

// RevokeToken revokes a specific CSRF token
func (c *CSRFManager) RevokeToken(token string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.tokens, token)
}

// RevokeUserTokens revokes all CSRF tokens for a specific user
func (c *CSRFManager) RevokeUserTokens(userID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for token, tokenData := range c.tokens {
		if tokenData.UserID == userID {
			delete(c.tokens, token)
		}
	}
}
