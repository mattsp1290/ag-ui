package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events/auth"
)

// AuthContextKey is a type for context keys to avoid collisions
type AuthContextKey string

const (
	// AuthContextUserKey is the key for user context
	AuthContextUserKey AuthContextKey = "auth_user"
	// AuthContextRolesKey is the key for roles context
	AuthContextRolesKey AuthContextKey = "auth_roles"
	// AuthContextPermissionsKey is the key for permissions context
	AuthContextPermissionsKey AuthContextKey = "auth_permissions"
)

// AuthMiddleware provides authentication and authorization functionality
type AuthMiddleware struct {
	provider       auth.AuthProvider
	config         *AuthMiddlewareConfig
	logger         Logger
	securityLogger SecurityLogger
}

// AuthMiddlewareConfig configures the authentication middleware
type AuthMiddlewareConfig struct {
	// TokenHeader is the header name for authentication tokens (default: "Authorization")
	TokenHeader string
	
	// TokenPrefix is the prefix for tokens (default: "Bearer ")
	TokenPrefix string
	
	// AllowAnonymous allows anonymous access when no auth header is present
	AllowAnonymous bool
	
	// RequiredRoles lists roles required for access (empty means any authenticated user)
	RequiredRoles []string
	
	// RequiredPermissions lists permissions required for access
	RequiredPermissions []string
	
	// SecureErrorMode prevents leaking sensitive information in error responses
	SecureErrorMode bool
	
	// RateLimiting enables rate limiting per user
	RateLimiting bool
	
	// RateLimit defines requests per minute per user
	RateLimit int
	
	// CORSEnabled enables CORS handling
	CORSEnabled bool
	
	// AllowedOrigins lists allowed CORS origins
	AllowedOrigins []string
}

// DefaultAuthMiddlewareConfig returns a secure default configuration
func DefaultAuthMiddlewareConfig() *AuthMiddlewareConfig {
	return &AuthMiddlewareConfig{
		TokenHeader:         "Authorization",
		TokenPrefix:         "Bearer ",
		AllowAnonymous:      false,
		RequiredRoles:       []string{},
		RequiredPermissions: []string{},
		SecureErrorMode:     true,
		RateLimiting:        true,
		RateLimit:           60, // 60 requests per minute
		CORSEnabled:         false,
		AllowedOrigins:      []string{},
	}
}

// Logger interface for structured logging
type Logger interface {
	Info(msg string, fields ...LogField)
	Warn(msg string, fields ...LogField)
	Error(msg string, fields ...LogField)
	Debug(msg string, fields ...LogField)
}

// SecurityLogger interface for security event logging
type SecurityLogger interface {
	LogAuthAttempt(userID, action string, success bool, metadata map[string]interface{})
	LogUnauthorizedAccess(userID, resource, action string, metadata map[string]interface{})
	LogSuspiciousActivity(userID, activity string, metadata map[string]interface{})
}

// LogField represents a structured log field
type LogField struct {
	Key   string
	Value interface{}
}

// SimpleLogger provides a basic logger implementation
type SimpleLogger struct{}

func (l *SimpleLogger) Info(msg string, fields ...LogField) {
	log.Printf("[INFO] %s %v", msg, fields)
}

func (l *SimpleLogger) Warn(msg string, fields ...LogField) {
	log.Printf("[WARN] %s %v", msg, fields)
}

func (l *SimpleLogger) Error(msg string, fields ...LogField) {
	log.Printf("[ERROR] %s %v", msg, fields)
}

func (l *SimpleLogger) Debug(msg string, fields ...LogField) {
	log.Printf("[DEBUG] %s %v", msg, fields)
}

// SimpleSecurityLogger provides a basic security logger implementation
type SimpleSecurityLogger struct{}

func (l *SimpleSecurityLogger) LogAuthAttempt(userID, action string, success bool, metadata map[string]interface{}) {
	status := "FAILED"
	if success {
		status = "SUCCESS"
	}
	log.Printf("[SECURITY] Auth attempt: user=%s, action=%s, status=%s, metadata=%v", 
		userID, action, status, metadata)
}

func (l *SimpleSecurityLogger) LogUnauthorizedAccess(userID, resource, action string, metadata map[string]interface{}) {
	log.Printf("[SECURITY] Unauthorized access: user=%s, resource=%s, action=%s, metadata=%v", 
		userID, resource, action, metadata)
}

func (l *SimpleSecurityLogger) LogSuspiciousActivity(userID, activity string, metadata map[string]interface{}) {
	log.Printf("[SECURITY] Suspicious activity: user=%s, activity=%s, metadata=%v", 
		userID, activity, metadata)
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(provider auth.AuthProvider, config *AuthMiddlewareConfig) *AuthMiddleware {
	if config == nil {
		config = DefaultAuthMiddlewareConfig()
	}
	
	return &AuthMiddleware{
		provider:       provider,
		config:         config,
		logger:         &SimpleLogger{},
		securityLogger: &SimpleSecurityLogger{},
	}
}

// SetLogger sets a custom logger
func (m *AuthMiddleware) SetLogger(logger Logger) {
	m.logger = logger
}

// SetSecurityLogger sets a custom security logger
func (m *AuthMiddleware) SetSecurityLogger(logger SecurityLogger) {
	m.securityLogger = logger
}

// Middleware returns an HTTP middleware function
func (m *AuthMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Handle CORS if enabled
			if m.config.CORSEnabled {
				m.handleCORS(w, r)
				if r.Method == http.MethodOptions {
					return
				}
			}
			
			// Extract and validate authentication
			authCtx, err := m.authenticateRequest(r)
			if err != nil {
				m.handleAuthError(w, r, err)
				return
			}
			
			// Check authorization
			if err := m.authorizeRequest(r, authCtx); err != nil {
				m.handleAuthzError(w, r, authCtx, err)
				return
			}
			
			// Rate limiting
			if m.config.RateLimiting && authCtx != nil {
				if !m.checkRateLimit(authCtx.UserID) {
					m.handleRateLimitError(w, r, authCtx)
					return
				}
			}
			
			// Add user context to request
			ctx := m.addUserContext(r.Context(), authCtx)
			r = r.WithContext(ctx)
			
			// Log successful authentication
			if authCtx != nil {
				m.securityLogger.LogAuthAttempt(authCtx.UserID, r.Method+" "+r.URL.Path, true, map[string]interface{}{
					"user_agent": r.UserAgent(),
					"ip":         getClientIP(r),
				})
			}
			
			// Continue to next handler
			next.ServeHTTP(w, r)
		})
	}
}

// authenticateRequest extracts and validates authentication from the request
func (m *AuthMiddleware) authenticateRequest(r *http.Request) (*auth.AuthContext, error) {
	// Extract token from header
	authHeader := r.Header.Get(m.config.TokenHeader)
	if authHeader == "" {
		if m.config.AllowAnonymous {
			return nil, nil // Allow anonymous access
		}
		return nil, auth.ErrUnauthorized
	}
	
	// Remove token prefix
	token := strings.TrimPrefix(authHeader, m.config.TokenPrefix)
	if token == authHeader && m.config.TokenPrefix != "" {
		return nil, fmt.Errorf("invalid token format: expected prefix '%s'", m.config.TokenPrefix)
	}
	
	// Create token credentials
	tokenCreds := &auth.TokenCredentials{
		Token:     token,
		TokenType: strings.TrimSpace(m.config.TokenPrefix),
	}
	
	// Authenticate with provider
	authCtx, err := m.provider.Authenticate(r.Context(), tokenCreds)
	if err != nil {
		m.securityLogger.LogAuthAttempt("unknown", r.Method+" "+r.URL.Path, false, map[string]interface{}{
			"error":      err.Error(),
			"user_agent": r.UserAgent(),
			"ip":         getClientIP(r),
		})
		return nil, err
	}
	
	return authCtx, nil
}

// authorizeRequest checks if the user has required permissions
func (m *AuthMiddleware) authorizeRequest(r *http.Request, authCtx *auth.AuthContext) error {
	// If no auth context and anonymous is allowed, skip authorization
	if authCtx == nil && m.config.AllowAnonymous {
		return nil
	}
	
	// If no auth context but authentication is required
	if authCtx == nil {
		return auth.ErrUnauthorized
	}
	
	// Check required roles
	if len(m.config.RequiredRoles) > 0 {
		if !authCtx.HasAnyRole(m.config.RequiredRoles...) {
			return auth.ErrInsufficientPermissions
		}
	}
	
	// Check required permissions
	for _, perm := range m.config.RequiredPermissions {
		if !authCtx.HasPermission(perm) {
			return auth.ErrInsufficientPermissions
		}
	}
	
	// Check provider-level authorization
	resource := r.URL.Path
	action := strings.ToLower(r.Method)
	
	if err := m.provider.Authorize(r.Context(), authCtx, resource, action); err != nil {
		return err
	}
	
	return nil
}

// addUserContext adds user information to the request context
func (m *AuthMiddleware) addUserContext(ctx context.Context, authCtx *auth.AuthContext) context.Context {
	if authCtx == nil {
		return ctx
	}
	
	ctx = context.WithValue(ctx, AuthContextUserKey, authCtx.UserID)
	ctx = context.WithValue(ctx, AuthContextRolesKey, authCtx.Roles)
	ctx = context.WithValue(ctx, AuthContextPermissionsKey, authCtx.Permissions)
	
	return ctx
}

// handleCORS handles CORS headers with security-first approach
func (m *AuthMiddleware) handleCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	
	// Check if origin is allowed (no wildcard allowed for security)
	allowed := false
	if len(m.config.AllowedOrigins) == 0 {
		// Only allow same origin if no origins specified
		allowed = false
	} else {
		for _, allowedOrigin := range m.config.AllowedOrigins {
			// Remove wildcard support for security
			if allowedOrigin == origin {
				allowed = true
				break
			}
			// Support wildcard subdomains only if explicitly configured
			if strings.HasPrefix(allowedOrigin, "*.") {
				domain := strings.TrimPrefix(allowedOrigin, "*.")
				if strings.HasSuffix(origin, domain) {
					allowed = true
					break
				}
			}
		}
	}
	
	if allowed {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
	}
}

// Error handling methods with secure error responses
func (m *AuthMiddleware) handleAuthError(w http.ResponseWriter, r *http.Request, err error) {
	statusCode := http.StatusUnauthorized
	message := "Authentication required"
	
	if !m.config.SecureErrorMode {
		message = err.Error()
	}
	
	m.logger.Warn("Authentication failed", 
		LogField{Key: "error", Value: err.Error()},
		LogField{Key: "path", Value: r.URL.Path},
		LogField{Key: "ip", Value: getClientIP(r)})
	
	m.writeErrorResponse(w, statusCode, message)
}

func (m *AuthMiddleware) handleAuthzError(w http.ResponseWriter, r *http.Request, authCtx *auth.AuthContext, err error) {
	statusCode := http.StatusForbidden
	message := "Access denied"
	
	if !m.config.SecureErrorMode {
		message = err.Error()
	}
	
	userID := "anonymous"
	if authCtx != nil {
		userID = authCtx.UserID
	}
	
	m.securityLogger.LogUnauthorizedAccess(userID, r.URL.Path, r.Method, map[string]interface{}{
		"error":      err.Error(),
		"user_agent": r.UserAgent(),
		"ip":         getClientIP(r),
	})
	
	m.writeErrorResponse(w, statusCode, message)
}

func (m *AuthMiddleware) handleRateLimitError(w http.ResponseWriter, r *http.Request, authCtx *auth.AuthContext) {
	statusCode := http.StatusTooManyRequests
	message := "Rate limit exceeded"
	
	m.securityLogger.LogSuspiciousActivity(authCtx.UserID, "rate_limit_exceeded", map[string]interface{}{
		"path":       r.URL.Path,
		"user_agent": r.UserAgent(),
		"ip":         getClientIP(r),
	})
	
	m.writeErrorResponse(w, statusCode, message)
}

func (m *AuthMiddleware) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := map[string]interface{}{
		"error":   message,
		"timestamp": time.Now().Unix(),
	}
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		m.logger.Error("Failed to encode error response", LogField{Key: "error", Value: err.Error()})
	}
}

// Rate limiting (simplified implementation)
var userRequestCounts = make(map[string][]time.Time)

func (m *AuthMiddleware) checkRateLimit(userID string) bool {
	now := time.Now()
	minute := 60 * time.Second
	
	// Clean old requests
	if requests, exists := userRequestCounts[userID]; exists {
		var validRequests []time.Time
		for _, reqTime := range requests {
			if now.Sub(reqTime) < minute {
				validRequests = append(validRequests, reqTime)
			}
		}
		userRequestCounts[userID] = validRequests
	}
	
	// Check rate limit
	if len(userRequestCounts[userID]) >= m.config.RateLimit {
		return false
	}
	
	// Add current request
	userRequestCounts[userID] = append(userRequestCounts[userID], now)
	return true
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// Take the first IP in the list
		if idx := strings.Index(forwarded, ","); idx != -1 {
			return strings.TrimSpace(forwarded[:idx])
		}
		return strings.TrimSpace(forwarded)
	}
	
	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return strings.TrimSpace(realIP)
	}
	
	// Fall back to RemoteAddr
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}

// Helper functions to extract user information from context

// GetUserID extracts the user ID from the request context
func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(AuthContextUserKey).(string)
	return userID, ok
}

// GetUserRoles extracts the user roles from the request context
func GetUserRoles(ctx context.Context) ([]string, bool) {
	roles, ok := ctx.Value(AuthContextRolesKey).([]string)
	return roles, ok
}

// GetUserPermissions extracts the user permissions from the request context
func GetUserPermissions(ctx context.Context) ([]string, bool) {
	permissions, ok := ctx.Value(AuthContextPermissionsKey).([]string)
	return permissions, ok
}

// HasRole checks if the current user has a specific role
func HasRole(ctx context.Context, role string) bool {
	roles, ok := GetUserRoles(ctx)
	if !ok {
		return false
	}
	
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasPermission checks if the current user has a specific permission
func HasPermission(ctx context.Context, permission string) bool {
	permissions, ok := GetUserPermissions(ctx)
	if !ok {
		return false
	}
	
	for _, p := range permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the current user has any of the specified roles
func HasAnyRole(ctx context.Context, roles ...string) bool {
	for _, role := range roles {
		if HasRole(ctx, role) {
			return true
		}
	}
	return false
}