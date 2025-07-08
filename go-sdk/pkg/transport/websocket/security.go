package websocket

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
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
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		RequireAuth:       true,
		AuthTimeout:       30 * time.Second,
		StrictOriginCheck: true,
		GlobalRateLimit:   1000.0,
		ClientRateLimit:   100.0,
		ClientBurstSize:   200,
		MaxConnections:    10000,
		RequireTLS:        true,
		MinTLSVersion:     tls.VersionTLS12,
		MaxMessageSize:    1024 * 1024, // 1MB
		MaxFrameSize:      64 * 1024,   // 64KB
		ReadDeadline:      60 * time.Second,
		WriteDeadline:     10 * time.Second,
		PingInterval:      30 * time.Second,
		PongTimeout:       10 * time.Second,
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
	clientLimiters  map[string]*rate.Limiter
	connectionCount int
	connections     map[string]*SecureConnection
	mu              sync.RWMutex
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
	mu           sync.RWMutex
}

// NewSecurityManager creates a new WebSocket security manager
func NewSecurityManager(config *SecurityConfig) *SecurityManager {
	if config == nil {
		config = DefaultSecurityConfig()
	}

	sm := &SecurityManager{
		config:         config,
		globalLimiter:  rate.NewLimiter(rate.Limit(config.GlobalRateLimit), int(config.GlobalRateLimit)),
		clientLimiters: make(map[string]*rate.Limiter),
		connections:    make(map[string]*SecureConnection),
		shutdownCh:     make(chan struct{}),
	}

	// Start cleanup routine
	sm.cleanupTicker = time.NewTicker(5 * time.Minute)
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
	sm.mu.RLock()
	connCount := sm.connectionCount
	sm.mu.RUnlock()

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
	}

	// Configure connection limits
	conn.SetReadLimit(sm.config.MaxMessageSize)

	// Set up ping/pong handling
	conn.SetPongHandler(func(string) error {
		secureConn.updateActivity()
		return conn.SetReadDeadline(time.Now().Add(sm.config.ReadDeadline))
	})

	// Register connection
	sm.mu.Lock()
	connectionID := fmt.Sprintf("%s_%d", clientIP, time.Now().UnixNano())
	sm.connections[connectionID] = secureConn
	sm.connectionCount++
	sm.mu.Unlock()

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
	sm.mu.Lock()
	defer sm.mu.Unlock()

	limiter, exists := sm.clientLimiters[clientIP]
	if !exists {
		limiter = rate.NewLimiter(rate.Limit(sm.config.ClientRateLimit), sm.config.ClientBurstSize)
		sm.clientLimiters[clientIP] = limiter
	}

	return limiter
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
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Remove limiters that haven't been used recently
	cutoff := time.Now().Add(-30 * time.Minute)
	for clientIP, limiter := range sm.clientLimiters {
		// Check if the limiter has been used recently
		if limiter.Tokens() == float64(sm.config.ClientBurstSize) {
			// If limiter is full, it hasn't been used recently
			delete(sm.clientLimiters, clientIP)
		}
	}

	// Clean up disconnected connections
	for id, conn := range sm.connections {
		if conn.lastActivity.Before(cutoff) {
			delete(sm.connections, id)
			sm.connectionCount--
		}
	}
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

	// Check message size (using a default limit)
	maxMessageSize := int64(1024 * 1024) // 1MB default
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
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, conn := range sm.connections {
		conn.conn.Close()
	}
}

// GetStats returns security statistics
func (sm *SecurityManager) GetStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return map[string]interface{}{
		"active_connections": sm.connectionCount,
		"client_limiters":    len(sm.clientLimiters),
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
	secretKey []byte
	issuer    string
}

// NewJWTTokenValidator creates a new JWT token validator
func NewJWTTokenValidator(secretKey []byte, issuer string) *JWTTokenValidator {
	return &JWTTokenValidator{
		secretKey: secretKey,
		issuer:    issuer,
	}
}

// ValidateToken validates a JWT token
func (v *JWTTokenValidator) ValidateToken(ctx context.Context, token string) (*AuthContext, error) {
	// This is a simplified JWT validation
	// In production, use a proper JWT library like github.com/golang-jwt/jwt/v5

	// Parse the token (simplified - just check if it's not empty)
	if token == "" {
		return nil, fmt.Errorf("empty token")
	}

	// In a real implementation, you would:
	// 1. Parse the JWT token
	// 2. Verify the signature
	// 3. Check expiration
	// 4. Validate issuer/audience
	// 5. Extract claims

	// For now, return a mock auth context
	return &AuthContext{
		UserID:      "user123",
		Username:    "testuser",
		Roles:       []string{"user"},
		Permissions: []string{"read", "write"},
		ExpiresAt:   time.Now().Add(1 * time.Hour),
		Claims:      map[string]interface{}{"sub": "user123"},
	}, nil
}

// NoOpWSAuditLogger provides a no-op audit logger for testing
type NoOpWSAuditLogger struct{}

// LogSecurityEvent logs a security event (no-op)
func (l *NoOpWSAuditLogger) LogSecurityEvent(ctx context.Context, event *SecurityEvent) error {
	return nil
}
