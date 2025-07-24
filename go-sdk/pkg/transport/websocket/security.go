package websocket

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
	"github.com/ag-ui/go-sdk/pkg/internal/timeconfig"
)

// Common JWT validation errors
var (
	ErrTokenExpired     = errors.New("token is expired")
	ErrTokenNotYetValid = errors.New("token is not valid yet")
	ErrInvalidToken     = errors.New("invalid token")
	ErrEmptyToken       = errors.New("empty token")
	ErrInvalidIssuer    = errors.New("invalid issuer")
	ErrInvalidAudience  = errors.New("invalid audience")
	ErrMissingClaims    = errors.New("missing required claims")
)

// SecurityConfig defines WebSocket security configuration
type SecurityConfig struct {
	// Authentication
	RequireAuth    bool           `json:"require_auth"`
	AuthTimeout    time.Duration  `json:"auth_timeout"`
	TokenValidator TokenValidator `json:"-"`

	// Origin validation
	AllowedOrigins    []string `json:"allowed_origins"`
	StrictOriginCheck bool     `json:"strict_origin_check"`

	// Subprotocol negotiation
	SupportedProtocols []string `json:"supported_protocols"`
	RequireSubprotocol bool     `json:"require_subprotocol"`

	// Rate limiting
	GlobalRateLimit float64 `json:"global_rate_limit"` // requests per second
	ClientRateLimit float64 `json:"client_rate_limit"` // requests per second per client
	ClientBurstSize int     `json:"client_burst_size"` // burst size for client rate limiting
	MaxConnections  int     `json:"max_connections"`   // maximum concurrent connections

	// TLS/SSL settings
	TLSConfig     *tls.Config `json:"-"`
	RequireTLS    bool        `json:"require_tls"`
	MinTLSVersion uint16      `json:"min_tls_version"`
	CertFile      string      `json:"cert_file"`
	KeyFile       string      `json:"key_file"`

	// Attack protection
	MaxMessageSize int64         `json:"max_message_size"` // maximum message size in bytes
	MaxFrameSize   int64         `json:"max_frame_size"`   // maximum frame size in bytes
	ReadDeadline   time.Duration `json:"read_deadline"`    // read timeout
	WriteDeadline  time.Duration `json:"write_deadline"`   // write timeout
	PingInterval   time.Duration `json:"ping_interval"`    // ping interval for keepalive
	PongTimeout    time.Duration `json:"pong_timeout"`     // pong timeout

	// Audit logging
	AuditLogger       WSAuditLogger `json:"-"`
	LogConnections    bool          `json:"log_connections"`
	LogMessages       bool          `json:"log_messages"`
	LogSecurityEvents bool          `json:"log_security_events"`
}

// DefaultSecurityConfig returns secure default configuration
// Uses configurable timeouts that adapt to test/production environments
func DefaultSecurityConfig() *SecurityConfig {
	config := timeconfig.GetConfig()
	return &SecurityConfig{
		RequireAuth:       true,
		AuthTimeout:       config.DefaultAuthTimeout,
		StrictOriginCheck: true,
		GlobalRateLimit:   1000.0,
		ClientRateLimit:   100.0,
		ClientBurstSize:   200,
		MaxConnections:    10000,
		RequireTLS:        true,
		MinTLSVersion:     tls.VersionTLS12,
		MaxMessageSize:    1024 * 1024, // 1MB
		MaxFrameSize:      64 * 1024,   // 64KB
		ReadDeadline:      config.DefaultReadTimeout,
		WriteDeadline:     config.DefaultWriteTimeout,
		PingInterval:      config.DefaultPingPeriod,
		PongTimeout:       config.DefaultPongTimeout,
		LogConnections:    true,
		LogMessages:       false,
		LogSecurityEvents: true,
	}
}

// TokenValidator interface for authentication token validation
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (*AuthContext, error)
}

// AuthContext contains authentication information
type AuthContext struct {
	UserID      string                 `json:"user_id"`
	Username    string                 `json:"username"`
	Roles       []string               `json:"roles"`
	Permissions []string               `json:"permissions"`
	ExpiresAt   time.Time              `json:"expires_at"`
	Claims      map[string]interface{} `json:"claims"`
}

// WSAuditLogger interface for security event logging
type WSAuditLogger interface {
	LogSecurityEvent(ctx context.Context, event *SecurityEvent) error
}

// SecurityEvent represents a security-related event
type SecurityEvent struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	ClientIP  string                 `json:"client_ip"`
	UserAgent string                 `json:"user_agent"`
	UserID    string                 `json:"user_id,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Severity  string                 `json:"severity"`
	Message   string                 `json:"message"`
}

// SecurityManager handles WebSocket security enforcement
type SecurityManager struct {
	config          *SecurityConfig
	globalLimiter   *rate.Limiter
	clientLimiters  sync.Map // map[string]*rate.Limiter
	connectionCount atomic.Int64
	connections     sync.Map     // map[string]*SecureConnection
	mu              sync.RWMutex // still needed for some operations
	shutdownCh      chan struct{}
	cleanupTicker   *time.Ticker
}

// SecureConnection represents a secured WebSocket connection
type SecureConnection struct {
	conn         *websocket.Conn
	limiter      *rate.Limiter
	authContext  *AuthContext
	clientIP     string
	userAgent    string
	connectedAt  time.Time
	lastActivity time.Time
	subprotocol  string
	manager      *SecurityManager
	mu           sync.RWMutex
}

// NewSecurityManager creates a new WebSocket security manager
func NewSecurityManager(config *SecurityConfig) *SecurityManager {
	if config == nil {
		config = DefaultSecurityConfig()
	}

	sm := &SecurityManager{
		config:        config,
		globalLimiter: rate.NewLimiter(rate.Limit(config.GlobalRateLimit), int(config.GlobalRateLimit)),
		shutdownCh:    make(chan struct{}),
	}

	// Start cleanup routine
	sm.cleanupTicker = time.NewTicker(timeconfig.GetConfig().DefaultCleanupInterval)
	go sm.cleanupRoutine()

	return sm
}

// ValidateUpgrade validates a WebSocket upgrade request
func (sm *SecurityManager) ValidateUpgrade(w http.ResponseWriter, r *http.Request) (*AuthContext, error) {
	ctx := r.Context()
	clientIP := getClientIP(r)
	userAgent := r.UserAgent()

	// Check global rate limit
	if !sm.globalLimiter.Allow() {
		sm.logSecurityEvent(ctx, &SecurityEvent{
			Type:      "rate_limit_exceeded",
			Timestamp: time.Now(),
			ClientIP:  clientIP,
			UserAgent: userAgent,
			Severity:  "warning",
			Message:   "Global rate limit exceeded",
		})
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// Check client rate limit
	clientLimiter := sm.getClientLimiter(clientIP)
	if !clientLimiter.Allow() {
		sm.logSecurityEvent(ctx, &SecurityEvent{
			Type:      "client_rate_limit_exceeded",
			Timestamp: time.Now(),
			ClientIP:  clientIP,
			UserAgent: userAgent,
			Severity:  "warning",
			Message:   "Client rate limit exceeded",
		})
		return nil, fmt.Errorf("client rate limit exceeded")
	}

	// Check connection count
	connCount := int(sm.connectionCount.Load())
	if connCount >= sm.config.MaxConnections {
		sm.logSecurityEvent(ctx, &SecurityEvent{
			Type:      "connection_limit_exceeded",
			Timestamp: time.Now(),
			ClientIP:  clientIP,
			UserAgent: userAgent,
			Severity:  "warning",
			Message:   "Maximum connections exceeded",
		})
		return nil, fmt.Errorf("maximum connections exceeded")
	}

	// Validate origin
	if err := sm.validateOrigin(r); err != nil {
		sm.logSecurityEvent(ctx, &SecurityEvent{
			Type:      "origin_validation_failed",
			Timestamp: time.Now(),
			ClientIP:  clientIP,
			UserAgent: userAgent,
			Severity:  "high",
			Message:   fmt.Sprintf("Origin validation failed: %v", err),
			Details:   map[string]interface{}{"origin": r.Header.Get("Origin")},
		})
		return nil, fmt.Errorf("origin validation failed: %w", err)
	}

	// Validate TLS if required
	if sm.config.RequireTLS && r.TLS == nil {
		sm.logSecurityEvent(ctx, &SecurityEvent{
			Type:      "tls_required",
			Timestamp: time.Now(),
			ClientIP:  clientIP,
			UserAgent: userAgent,
			Severity:  "high",
			Message:   "TLS required but not present",
		})
		return nil, fmt.Errorf("TLS required")
	}

	// Validate TLS version
	if r.TLS != nil && r.TLS.Version < sm.config.MinTLSVersion {
		sm.logSecurityEvent(ctx, &SecurityEvent{
			Type:      "tls_version_too_low",
			Timestamp: time.Now(),
			ClientIP:  clientIP,
			UserAgent: userAgent,
			Severity:  "high",
			Message:   fmt.Sprintf("TLS version %d below minimum %d", r.TLS.Version, sm.config.MinTLSVersion),
		})
		return nil, fmt.Errorf("TLS version too low")
	}

	// Authenticate if required
	var authContext *AuthContext
	if sm.config.RequireAuth {
		token := extractToken(r)
		if token == "" {
			sm.logSecurityEvent(ctx, &SecurityEvent{
				Type:      "authentication_required",
				Timestamp: time.Now(),
				ClientIP:  clientIP,
				UserAgent: userAgent,
				Severity:  "medium",
				Message:   "Authentication token required",
			})
			return nil, fmt.Errorf("authentication required")
		}

		authCtx, err := sm.config.TokenValidator.ValidateToken(ctx, token)
		if err != nil {
			sm.logSecurityEvent(ctx, &SecurityEvent{
				Type:      "authentication_failed",
				Timestamp: time.Now(),
				ClientIP:  clientIP,
				UserAgent: userAgent,
				Severity:  "high",
				Message:   fmt.Sprintf("Authentication failed: %v", err),
			})
			return nil, fmt.Errorf("authentication failed: %w", err)
		}

		authContext = authCtx
	}

	// Log successful validation
	if sm.config.LogConnections {
		sm.logSecurityEvent(ctx, &SecurityEvent{
			Type:      "connection_validated",
			Timestamp: time.Now(),
			ClientIP:  clientIP,
			UserAgent: userAgent,
			UserID:    getStringValue(authContext, "UserID"),
			Severity:  "info",
			Message:   "WebSocket upgrade validated",
		})
	}

	return authContext, nil
}

// CreateUpgrader creates a configured WebSocket upgrader
func (sm *SecurityManager) CreateUpgrader() *websocket.Upgrader {
	return &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     sm.checkOrigin,
		Subprotocols:    sm.config.SupportedProtocols,
	}
}

// SecureConnection wraps a WebSocket connection with security features
func (sm *SecurityManager) SecureConnection(conn *websocket.Conn, authContext *AuthContext, r *http.Request) *SecureConnection {
	clientIP := getClientIP(r)
	userAgent := r.UserAgent()

	secureConn := &SecureConnection{
		conn:         conn,
		limiter:      sm.getClientLimiter(clientIP),
		authContext:  authContext,
		clientIP:     clientIP,
		userAgent:    userAgent,
		connectedAt:  time.Now(),
		lastActivity: time.Now(),
		subprotocol:  conn.Subprotocol(),
		manager:      sm,
	}

	// Configure connection limits
	conn.SetReadLimit(sm.config.MaxMessageSize)

	// Set up ping/pong handling
	conn.SetPongHandler(func(string) error {
		secureConn.updateActivity()
		return conn.SetReadDeadline(time.Now().Add(sm.config.ReadDeadline))
	})

	// Register connection
	connectionID := fmt.Sprintf("%s_%d", clientIP, time.Now().UnixNano())
	sm.connections.Store(connectionID, secureConn)
	sm.connectionCount.Add(1)

	// Start ping routine
	go sm.pingRoutine(secureConn)

	return secureConn
}

// validateOrigin validates the request origin
func (sm *SecurityManager) validateOrigin(r *http.Request) error {
	origin := r.Header.Get("Origin")

	// Allow requests with no origin (same-origin)
	if origin == "" && !sm.config.StrictOriginCheck {
		return nil
	}

	if origin == "" && sm.config.StrictOriginCheck {
		return fmt.Errorf("origin required")
	}

	// Parse origin URL
	originURL, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("invalid origin URL: %w", err)
	}

	// Check against allowed origins
	if len(sm.config.AllowedOrigins) == 0 {
		return nil // Allow all origins if none specified
	}

	for _, allowedOrigin := range sm.config.AllowedOrigins {
		if allowedOrigin == "*" {
			return nil
		}

		if strings.EqualFold(originURL.Host, allowedOrigin) {
			return nil
		}

		// Check for wildcard subdomains
		if strings.HasPrefix(allowedOrigin, "*.") {
			domain := strings.TrimPrefix(allowedOrigin, "*.")
			if strings.HasSuffix(strings.ToLower(originURL.Host), "."+strings.ToLower(domain)) {
				return nil
			}
		}
	}

	return fmt.Errorf("origin %s not allowed", origin)
}

// checkOrigin is used by the upgrader to validate origins
func (sm *SecurityManager) checkOrigin(r *http.Request) bool {
	return sm.validateOrigin(r) == nil
}

// getClientLimiter returns a rate limiter for the client
func (sm *SecurityManager) getClientLimiter(clientIP string) *rate.Limiter {
	if limiter, ok := sm.clientLimiters.Load(clientIP); ok {
		return limiter.(*rate.Limiter)
	}

	newLimiter := rate.NewLimiter(rate.Limit(sm.config.ClientRateLimit), sm.config.ClientBurstSize)

	if actual, loaded := sm.clientLimiters.LoadOrStore(clientIP, newLimiter); loaded {
		return actual.(*rate.Limiter)
	}

	return newLimiter
}

// pingRoutine handles ping/pong keepalive
func (sm *SecurityManager) pingRoutine(conn *SecureConnection) {
	ticker := time.NewTicker(sm.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			conn.mu.Lock()
			if err := conn.conn.SetWriteDeadline(time.Now().Add(sm.config.WriteDeadline)); err != nil {
				conn.mu.Unlock()
				return
			}
			if err := conn.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				conn.mu.Unlock()
				return
			}
			conn.mu.Unlock()
		case <-sm.shutdownCh:
			return
		}
	}
}

// cleanupRoutine periodically cleans up expired client limiters
func (sm *SecurityManager) cleanupRoutine() {
	defer sm.cleanupTicker.Stop()

	for {
		select {
		case <-sm.cleanupTicker.C:
			sm.cleanupExpiredLimiters()
		case <-sm.shutdownCh:
			return
		}
	}
}

// cleanupExpiredLimiters removes unused client limiters
func (sm *SecurityManager) cleanupExpiredLimiters() {
	// Remove limiters that haven't been used recently
	cutoff := time.Now().Add(-30 * time.Minute)
	sm.clientLimiters.Range(func(key, value interface{}) bool {
		clientIP := key.(string)
		limiter := value.(*rate.Limiter)
		// Check if the limiter has been used recently
		if limiter.Tokens() == float64(sm.config.ClientBurstSize) {
			// If limiter is full, it hasn't been used recently
			sm.clientLimiters.Delete(clientIP)
		}
		return true
	})

	// Clean up disconnected connections
	sm.connections.Range(func(key, value interface{}) bool {
		id := key.(string)
		conn := value.(*SecureConnection)
		if conn.lastActivity.Before(cutoff) {
			sm.connections.Delete(id)
			sm.connectionCount.Add(-1)
		}
		return true
	})
}

// updateActivity updates the last activity time
func (sc *SecureConnection) updateActivity() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.lastActivity = time.Now()
}

// ValidateMessage validates an incoming message
func (sc *SecureConnection) ValidateMessage(messageType int, data []byte) error {
	sc.updateActivity()

	// Check message size against configured limit
	maxMessageSize := sc.manager.config.MaxMessageSize
	if maxMessageSize == 0 {
		maxMessageSize = int64(1024 * 1024) // 1MB default if not configured
	}
	if int64(len(data)) > maxMessageSize {
		return fmt.Errorf("message size %d exceeds limit", len(data))
	}

	// Apply rate limiting
	if !sc.limiter.Allow() {
		return fmt.Errorf("rate limit exceeded")
	}

	return nil
}

// logSecurityEvent logs a security event
func (sm *SecurityManager) logSecurityEvent(ctx context.Context, event *SecurityEvent) {
	if sm.config.AuditLogger == nil || !sm.config.LogSecurityEvents {
		return
	}

	go func() {
		if err := sm.config.AuditLogger.LogSecurityEvent(ctx, event); err != nil {
			// Log error to stderr if audit logging fails
			fmt.Printf("Failed to log security event: %v\n", err)
		}
	}()
}

// Shutdown gracefully shuts down the security manager
func (sm *SecurityManager) Shutdown() {
	close(sm.shutdownCh)

	// Close all connections
	sm.connections.Range(func(key, value interface{}) bool {
		conn := value.(*SecureConnection)
		conn.conn.Close()
		return true
	})
}

// GetStats returns security statistics
func (sm *SecurityManager) GetStats() map[string]interface{} {
	// Count client limiters
	limiterCount := 0
	sm.clientLimiters.Range(func(key, value interface{}) bool {
		limiterCount++
		return true
	})

	return map[string]interface{}{
		"active_connections": sm.connectionCount.Load(),
		"client_limiters":    limiterCount,
		"max_connections":    sm.config.MaxConnections,
		"global_rate_limit":  sm.config.GlobalRateLimit,
		"client_rate_limit":  sm.config.ClientRateLimit,
	}
}

// Utility functions

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
		return r.RemoteAddr[:idx]
	}

	return r.RemoteAddr
}

// extractToken extracts the authentication token from the request
func extractToken(r *http.Request) string {
	// Check Authorization header
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
	}

	// Check query parameter
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}

	// Check Sec-WebSocket-Protocol header for token
	if protocols := r.Header.Get("Sec-WebSocket-Protocol"); protocols != "" {
		parts := strings.Split(protocols, ", ")
		for _, part := range parts {
			if strings.HasPrefix(part, "auth.") {
				return strings.TrimPrefix(part, "auth.")
			}
		}
	}

	return ""
}

// getStringValue safely gets a string value from an interface
func getStringValue(obj interface{}, field string) string {
	if obj == nil {
		return ""
	}

	switch v := obj.(type) {
	case *AuthContext:
		switch field {
		case "UserID":
			return v.UserID
		case "Username":
			return v.Username
		}
	}

	return ""
}

// JWTTokenValidator provides JWT token validation
type JWTTokenValidator struct {
	secretKey     []byte         // For HMAC signing
	publicKey     *rsa.PublicKey // For RSA signing
	issuer        string
	audience      string
	signingMethod jwt.SigningMethod
}

// NewJWTTokenValidator creates a new JWT token validator
func NewJWTTokenValidator(secretKey []byte, issuer string) *JWTTokenValidator {
	return &JWTTokenValidator{
		secretKey:     secretKey,
		issuer:        issuer,
		audience:      "",
		signingMethod: jwt.SigningMethodHS256,
	}
}

// NewJWTTokenValidatorWithOptions creates a new JWT token validator with full options
func NewJWTTokenValidatorWithOptions(secretKey []byte, issuer, audience string, signingMethod jwt.SigningMethod) *JWTTokenValidator {
	if signingMethod == nil {
		signingMethod = jwt.SigningMethodHS256
	}
	return &JWTTokenValidator{
		secretKey:     secretKey,
		issuer:        issuer,
		audience:      audience,
		signingMethod: signingMethod,
	}
}

// NewJWTTokenValidatorRSA creates a new JWT token validator for RSA signatures
func NewJWTTokenValidatorRSA(publicKey *rsa.PublicKey, issuer, audience string) *JWTTokenValidator {
	return &JWTTokenValidator{
		publicKey:     publicKey,
		issuer:        issuer,
		audience:      audience,
		signingMethod: jwt.SigningMethodRS256,
	}
}

// ValidateToken validates a JWT token
func (v *JWTTokenValidator) ValidateToken(ctx context.Context, tokenString string) (*AuthContext, error) {
	if tokenString == "" {
		return nil, ErrEmptyToken
	}

	// Parse and validate the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method
		if token.Method != v.signingMethod {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Return the appropriate key based on signing method
		switch token.Method.(type) {
		case *jwt.SigningMethodHMAC:
			if v.secretKey == nil {
				return nil, fmt.Errorf("HMAC key not configured")
			}
			return v.secretKey, nil
		case *jwt.SigningMethodRSA, *jwt.SigningMethodRSAPSS:
			if v.publicKey == nil {
				return nil, fmt.Errorf("RSA public key not configured")
			}
			return v.publicKey, nil
		default:
			return nil, fmt.Errorf("unsupported signing method: %v", token.Header["alg"])
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Check if token is valid
	if !token.Valid {
		return nil, ErrInvalidToken
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrMissingClaims
	}

	// Validate standard claims
	// Check issuer
	if v.issuer != "" {
		issuer, ok := claims["iss"].(string)
		if !ok || issuer != v.issuer {
			return nil, ErrInvalidIssuer
		}
	}

	// Check audience
	if v.audience != "" {
		switch aud := claims["aud"].(type) {
		case string:
			if aud != v.audience {
				return nil, ErrInvalidAudience
			}
		case []interface{}:
			found := false
			for _, a := range aud {
				if audStr, ok := a.(string); ok && audStr == v.audience {
					found = true
					break
				}
			}
			if !found {
				return nil, ErrInvalidAudience
			}
		default:
			return nil, ErrInvalidAudience
		}
	}

	// Check expiration
	exp, ok := claims["exp"].(float64)
	if !ok {
		return nil, ErrMissingClaims
	}
	expiresAt := time.Unix(int64(exp), 0)
	if time.Now().After(expiresAt) {
		return nil, ErrTokenExpired
	}

	// Check not before
	if nbf, ok := claims["nbf"].(float64); ok {
		notBefore := time.Unix(int64(nbf), 0)
		if time.Now().Before(notBefore) {
			return nil, ErrTokenNotYetValid
		}
	}

	// Extract user information
	authContext := &AuthContext{
		ExpiresAt: expiresAt,
		Claims:    make(map[string]interface{}),
	}

	// Extract user ID from subject or custom claim
	if sub, ok := claims["sub"].(string); ok {
		authContext.UserID = sub
	} else if userID, ok := claims["user_id"].(string); ok {
		authContext.UserID = userID
	}

	// Extract username
	if username, ok := claims["username"].(string); ok {
		authContext.Username = username
	} else if email, ok := claims["email"].(string); ok {
		authContext.Username = email
	}

	// Extract roles
	if rolesInterface, ok := claims["roles"]; ok {
		switch roles := rolesInterface.(type) {
		case []interface{}:
			for _, role := range roles {
				if roleStr, ok := role.(string); ok {
					authContext.Roles = append(authContext.Roles, roleStr)
				}
			}
		case []string:
			authContext.Roles = roles
		}
	}

	// Extract permissions
	if permsInterface, ok := claims["permissions"]; ok {
		switch perms := permsInterface.(type) {
		case []interface{}:
			for _, perm := range perms {
				if permStr, ok := perm.(string); ok {
					authContext.Permissions = append(authContext.Permissions, permStr)
				}
			}
		case []string:
			authContext.Permissions = perms
		}
	}

	// Store all claims for reference
	for k, v := range claims {
		authContext.Claims[k] = v
	}

	return authContext, nil
}

// NoOpWSAuditLogger provides a no-op audit logger for testing
type NoOpWSAuditLogger struct{}

// LogSecurityEvent logs a security event (no-op)
func (l *NoOpWSAuditLogger) LogSecurityEvent(ctx context.Context, event *SecurityEvent) error {
	return nil
}
