package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/memory"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/server/middleware"
	"go.uber.org/zap"
)

// Session constants for better maintainability
const (
	DefaultSessionTTL           = 24 * time.Hour
	DefaultCleanupInterval      = time.Hour
	DefaultMaxConcurrentSessions = 10000
	DefaultSessionPoolSize      = 1000
	DefaultMaxSessionsPerUser   = 10
	
	// Cookie configuration
	DefaultCookieName       = "session_id"
	DefaultCookiePath       = "/"
	DefaultSameSiteCookies  = "Strict"
	
	// Session validation
	MinSessionTTL           = time.Minute
	MinCleanupInterval      = time.Minute
	MaxSessionIDLength      = 128
	SessionIDLength         = 64
)

// Session represents a user session
type Session struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"user_id"`
	CreatedAt    time.Time              `json:"created_at"`
	LastAccessed time.Time              `json:"last_accessed"`
	ExpiresAt    time.Time              `json:"expires_at"`
	IPAddress    string                 `json:"ip_address"`
	UserAgent    string                 `json:"user_agent"`
	IsActive     bool                   `json:"is_active"`
	Data         map[string]interface{} `json:"data"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// SessionManager manages user sessions with pluggable storage backends
type SessionManager struct {
	config     *SessionConfig
	logger     *zap.Logger
	storage    SessionStorage
	memManager *memory.MemoryManager

	// Runtime state
	activeSessions int64
	totalSessions  int64
	cleanupTicker  *time.Ticker
	stopCleanup    chan struct{}
	wg             sync.WaitGroup

	// Synchronization
	mu sync.RWMutex

	// Cleanup state synchronization
	cleanupMu         sync.Mutex
	cleanupInProgress int64            // atomic flag for cleanup operations
	pendingCleanups   map[string]int64 // track pending cleanup operations with timestamps

	// Session operation synchronization
	sessionOps   map[string]*sync.Mutex // per-session operation locks
	sessionOpsMu sync.RWMutex           // protects sessionOps map

	// Shutdown synchronization
	closeOnce sync.Once
	closed    atomic.Bool

	// Metrics
	metrics *SessionMetrics
	
	// Secure credential management
	credentialManager *middleware.CredentialManager
	auditor          *middleware.CredentialAuditor
}

// SessionMetrics tracks session management metrics
type SessionMetrics struct {
	mu                 sync.RWMutex
	TotalSessions      int64     `json:"total_sessions"`
	ActiveSessions     int64     `json:"active_sessions"`
	ExpiredSessions    int64     `json:"expired_sessions"`
	CleanupRuns        int64     `json:"cleanup_runs"`
	LastCleanup        time.Time `json:"last_cleanup"`
	MemoryUsageBytes   int64     `json:"memory_usage_bytes"`
	AverageSessionSize int64     `json:"average_session_size"`
	SessionsPerSecond  float64   `json:"sessions_per_second"`
	LastMetricsUpdate  time.Time `json:"last_metrics_update"`
	StorageErrors      int64     `json:"storage_errors"`
	ValidationErrors   int64     `json:"validation_errors"`
}

// SessionStorage defines the interface for session storage backends
type SessionStorage interface {
	// Session operations
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	UpdateSession(ctx context.Context, session *Session) error
	DeleteSession(ctx context.Context, sessionID string) error

	// Bulk operations
	GetUserSessions(ctx context.Context, userID string) ([]*Session, error)
	DeleteUserSessions(ctx context.Context, userID string) error
	CleanupExpiredSessions(ctx context.Context) (int64, error)

	// Session queries
	GetActiveSessions(ctx context.Context, limit int) ([]*Session, error)
	CountSessions(ctx context.Context) (int64, error)

	// Lifecycle
	Close() error
	Ping(ctx context.Context) error
	Stats() map[string]interface{}
}

// NewSessionManager creates a new session manager
func NewSessionManager(config *SessionConfig, logger *zap.Logger) (*SessionManager, error) {
	if config == nil {
		config = DefaultSessionConfig()
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid session config: %w", err)
	}

	// Create storage backend
	storage, err := createSessionStorage(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create session storage: %w", err)
	}

	// Initialize memory manager
	memConfig := memory.DefaultMemoryManagerConfig()
	memConfig.Logger = logger
	memManager := memory.NewMemoryManager(memConfig)

	sm := &SessionManager{
		config:            config,
		logger:            logger,
		storage:           storage,
		memManager:        memManager,
		stopCleanup:       make(chan struct{}),
		pendingCleanups:   make(map[string]int64),
		sessionOps:        make(map[string]*sync.Mutex),
		credentialManager: middleware.NewCredentialManager(logger),
		auditor:          middleware.NewCredentialAuditor(logger),
		metrics: &SessionMetrics{
			LastMetricsUpdate: time.Now(),
		},
	}

	// Load secure credentials from environment variables
	if err := sm.loadSecureCredentials(); err != nil {
		return nil, fmt.Errorf("failed to load secure credentials: %w", err)
	}

	// Start background services
	sm.startBackgroundServices()

	logger.Info("Session manager initialized",
		zap.String("backend", config.Backend),
		zap.Duration("ttl", config.TTL),
		zap.Duration("cleanup_interval", config.CleanupInterval))

	return sm, nil
}

// loadSecureCredentials loads credentials from environment variables based on backend
func (sm *SessionManager) loadSecureCredentials() error {
	switch sm.config.Backend {
	case "redis":
		if sm.config.Redis != nil {
			if err := sm.config.Redis.LoadCredentials(sm.logger); err != nil {
				return fmt.Errorf("failed to load Redis credentials: %w", err)
			}
			sm.auditor.AuditCredentialValidation("redis_session_config", true, nil)
		}
	case "database":
		if sm.config.Database != nil {
			if err := sm.config.Database.LoadCredentials(sm.logger); err != nil {
				return fmt.Errorf("failed to load database credentials: %w", err)
			}
			sm.auditor.AuditCredentialValidation("database_session_config", true, nil)
		}
	case "memory":
		// Memory backend doesn't require credentials
	default:
		return fmt.Errorf("unsupported session backend: %s", sm.config.Backend)
	}

	return nil
}

// CreateSession creates a new session for the given user
func (sm *SessionManager) CreateSession(ctx context.Context, userID string, req *http.Request) (*Session, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	// Validate request
	if err := sm.validateCreateSessionRequest(req); err != nil {
		atomic.AddInt64(&sm.metrics.ValidationErrors, 1)
		return nil, fmt.Errorf("invalid session creation request: %w", err)
	}

	// Check concurrent session limits
	if err := sm.enforceSessionLimits(ctx, userID); err != nil {
		return nil, fmt.Errorf("session limit exceeded: %w", err)
	}

	// Generate secure session ID
	sessionID, err := sm.generateSecureSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	// Create session object
	now := time.Now()
	session := &Session{
		ID:           sessionID,
		UserID:       userID,
		CreatedAt:    now,
		LastAccessed: now,
		ExpiresAt:    now.Add(sm.config.TTL),
		IPAddress:    sm.extractIPAddress(req),
		UserAgent:    req.UserAgent(),
		IsActive:     true,
		Data:         make(map[string]interface{}),
		Metadata:     make(map[string]interface{}),
	}

	// Store session
	if err := sm.storage.CreateSession(ctx, session); err != nil {
		atomic.AddInt64(&sm.metrics.StorageErrors, 1)
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	// Update metrics
	atomic.AddInt64(&sm.activeSessions, 1)
	atomic.AddInt64(&sm.totalSessions, 1)
	sm.updateSessionMetrics()

	sm.logger.Debug("Session created",
		zap.String("session_id", sessionID),
		zap.String("user_id", userID),
		zap.String("ip_address", session.IPAddress))

	return session, nil
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	if err := sm.validateSessionID(sessionID); err != nil {
		atomic.AddInt64(&sm.metrics.ValidationErrors, 1)
		return nil, fmt.Errorf("invalid session ID: %w", err)
	}

	// Get session lock for this operation
	opLock := sm.getSessionOperationLock(sessionID)
	opLock.Lock()
	defer opLock.Unlock()

	// Retrieve session from storage
	session, err := sm.storage.GetSession(ctx, sessionID)
	if err != nil {
		atomic.AddInt64(&sm.metrics.StorageErrors, 1)
		return nil, fmt.Errorf("failed to retrieve session: %w", err)
	}

	if session == nil {
		return nil, nil // Session not found
	}

	// Check if session is expired
	if session.ExpiresAt.Before(time.Now()) {
		// Clean up expired session
		if err := sm.storage.DeleteSession(ctx, sessionID); err != nil {
			sm.logger.Warn("Failed to delete expired session",
				zap.String("session_id", sessionID),
				zap.Error(err))
		}
		atomic.AddInt64(&sm.metrics.ExpiredSessions, 1)
		return nil, nil
	}

	// Update last accessed time
	session.LastAccessed = time.Now()
	if err := sm.storage.UpdateSession(ctx, session); err != nil {
		sm.logger.Warn("Failed to update session last accessed time",
			zap.String("session_id", sessionID),
			zap.Error(err))
	}

	return session, nil
}

// ValidateSession validates a session and optionally validates IP and User Agent
func (sm *SessionManager) ValidateSession(ctx context.Context, sessionID string, req *http.Request) (*Session, error) {
	session, err := sm.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if session == nil {
		return nil, fmt.Errorf("session not found")
	}

	// Validate IP address if configured
	if sm.config.ValidateIP {
		currentIP := sm.extractIPAddress(req)
		if session.IPAddress != currentIP {
			sm.logger.Warn("Session IP validation failed",
				zap.String("session_id", sessionID),
				zap.String("expected_ip", session.IPAddress),
				zap.String("actual_ip", currentIP))
			atomic.AddInt64(&sm.metrics.ValidationErrors, 1)
			return nil, fmt.Errorf("session IP validation failed")
		}
	}

	// Validate User Agent if configured
	if sm.config.ValidateUserAgent {
		currentUA := req.UserAgent()
		if session.UserAgent != currentUA {
			sm.logger.Warn("Session User-Agent validation failed",
				zap.String("session_id", sessionID),
				zap.String("expected_ua", session.UserAgent),
				zap.String("actual_ua", currentUA))
			atomic.AddInt64(&sm.metrics.ValidationErrors, 1)
			return nil, fmt.Errorf("session User-Agent validation failed")
		}
	}

	return session, nil
}

// UpdateSession updates an existing session
func (sm *SessionManager) UpdateSession(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	if err := sm.validateSessionID(session.ID); err != nil {
		atomic.AddInt64(&sm.metrics.ValidationErrors, 1)
		return fmt.Errorf("invalid session ID: %w", err)
	}

	// Get session lock for this operation
	opLock := sm.getSessionOperationLock(session.ID)
	opLock.Lock()
	defer opLock.Unlock()

	// Update last accessed time
	session.LastAccessed = time.Now()

	// Update session in storage
	if err := sm.storage.UpdateSession(ctx, session); err != nil {
		atomic.AddInt64(&sm.metrics.StorageErrors, 1)
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// DeleteSession deletes a session
func (sm *SessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	if err := sm.validateSessionID(sessionID); err != nil {
		atomic.AddInt64(&sm.metrics.ValidationErrors, 1)
		return fmt.Errorf("invalid session ID: %w", err)
	}

	// Get session lock for this operation
	opLock := sm.getSessionOperationLock(sessionID)
	opLock.Lock()
	defer opLock.Unlock()

	// Delete session from storage
	if err := sm.storage.DeleteSession(ctx, sessionID); err != nil {
		atomic.AddInt64(&sm.metrics.StorageErrors, 1)
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// Update metrics
	atomic.AddInt64(&sm.activeSessions, -1)

	// Clean up session operation lock
	sm.cleanupSessionOperationLock(sessionID)

	sm.logger.Debug("Session deleted", zap.String("session_id", sessionID))
	return nil
}

// GetUserSessions returns all sessions for a given user
func (sm *SessionManager) GetUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	sessions, err := sm.storage.GetUserSessions(ctx, userID)
	if err != nil {
		atomic.AddInt64(&sm.metrics.StorageErrors, 1)
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}

	// Filter out expired sessions
	var activeSessions []*Session
	now := time.Now()
	for _, session := range sessions {
		if session.ExpiresAt.After(now) {
			activeSessions = append(activeSessions, session)
		}
	}

	return activeSessions, nil
}

// DeleteUserSessions deletes all sessions for a given user
func (sm *SessionManager) DeleteUserSessions(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}

	// Get user sessions first to clean up locks
	sessions, err := sm.GetUserSessions(ctx, userID)
	if err != nil {
		return err
	}

	// Delete sessions from storage
	if err := sm.storage.DeleteUserSessions(ctx, userID); err != nil {
		atomic.AddInt64(&sm.metrics.StorageErrors, 1)
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	// Clean up session operation locks and update metrics
	for _, session := range sessions {
		sm.cleanupSessionOperationLock(session.ID)
		atomic.AddInt64(&sm.activeSessions, -1)
	}

	sm.logger.Debug("User sessions deleted",
		zap.String("user_id", userID),
		zap.Int("session_count", len(sessions)))

	return nil
}

// Cleanup and shutdown methods

// Close gracefully shuts down the session manager
// This method is idempotent and safe to call multiple times
func (sm *SessionManager) Close() error {
	var shutdownErr error
	sm.closeOnce.Do(func() {
		// Use a reasonable timeout for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		shutdownErr = sm.Shutdown(ctx)
	})
	return shutdownErr
}

// Shutdown gracefully shuts down the session manager
func (sm *SessionManager) Shutdown(ctx context.Context) error {
	return sm.shutdownBackgroundServices(ctx)
}

// Helper methods for session management

// validateCreateSessionRequest validates a session creation request
func (sm *SessionManager) validateCreateSessionRequest(req *http.Request) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}

	// Additional request validation can be added here
	return nil
}

// enforceSessionLimits checks and enforces session limits
func (sm *SessionManager) enforceSessionLimits(ctx context.Context, userID string) error {
	// Check global session limit
	if sm.activeSessions >= int64(sm.config.MaxConcurrentSessions) {
		return fmt.Errorf("maximum concurrent sessions reached (%d)", sm.config.MaxConcurrentSessions)
	}

	// Check per-user session limit
	if sm.config.MaxSessionsPerUser > 0 {
		userSessions, err := sm.GetUserSessions(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to check user session count: %w", err)
		}

		if len(userSessions) >= sm.config.MaxSessionsPerUser {
			return fmt.Errorf("maximum sessions per user reached (%d)", sm.config.MaxSessionsPerUser)
		}
	}

	return nil
}

// generateSecureSessionID generates a cryptographically secure session ID
func (sm *SessionManager) generateSecureSessionID() (string, error) {
	bytes := make([]byte, SessionIDLength/2) // 32 bytes = 64 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Create a hash of the random bytes + timestamp for additional entropy
	hash := sha256.New()
	hash.Write(bytes)
	hash.Write([]byte(strconv.FormatInt(time.Now().UnixNano(), 10)))

	return hex.EncodeToString(hash.Sum(nil))[:SessionIDLength], nil
}

// validateSessionID validates a session ID
func (sm *SessionManager) validateSessionID(sessionID string) error {
	if len(sessionID) == 0 {
		return fmt.Errorf("session ID cannot be empty")
	}

	if len(sessionID) > MaxSessionIDLength {
		return fmt.Errorf("session ID too long (max %d characters)", MaxSessionIDLength)
	}

	// Check for valid hex characters
	for _, char := range sessionID {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return fmt.Errorf("session ID contains invalid characters")
		}
	}

	return nil
}

// extractIPAddress extracts IP address from HTTP request
func (sm *SessionManager) extractIPAddress(req *http.Request) string {
	// Check for forwarded IP (behind proxy/load balancer)
	if forwarded := req.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Take the first IP in the comma-separated list
		if idx := strings.Index(forwarded, ","); idx != -1 {
			return strings.TrimSpace(forwarded[:idx])
		}
		return strings.TrimSpace(forwarded)
	}

	// Check for real IP header
	if realIP := req.Header.Get("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}

	// Fall back to remote address
	if idx := strings.LastIndex(req.RemoteAddr, ":"); idx != -1 {
		return req.RemoteAddr[:idx]
	}
	return req.RemoteAddr
}

// getSessionOperationLock gets or creates a per-session operation lock
func (sm *SessionManager) getSessionOperationLock(sessionID string) *sync.Mutex {
	sm.sessionOpsMu.RLock()
	if lock, exists := sm.sessionOps[sessionID]; exists {
		sm.sessionOpsMu.RUnlock()
		return lock
	}
	sm.sessionOpsMu.RUnlock()

	// Need to create a new lock
	sm.sessionOpsMu.Lock()
	defer sm.sessionOpsMu.Unlock()

	// Double-check in case another goroutine created it
	if lock, exists := sm.sessionOps[sessionID]; exists {
		return lock
	}

	lock := &sync.Mutex{}
	sm.sessionOps[sessionID] = lock
	return lock
}

// cleanupSessionOperationLock removes a per-session operation lock
func (sm *SessionManager) cleanupSessionOperationLock(sessionID string) {
	sm.sessionOpsMu.Lock()
	delete(sm.sessionOps, sessionID)
	sm.sessionOpsMu.Unlock()
}

// updateSessionMetrics updates session metrics
func (sm *SessionManager) updateSessionMetrics() {
	sm.metrics.mu.Lock()
	defer sm.metrics.mu.Unlock()

	now := time.Now()
	sm.metrics.TotalSessions = atomic.LoadInt64(&sm.totalSessions)
	sm.metrics.ActiveSessions = atomic.LoadInt64(&sm.activeSessions)
	sm.metrics.LastMetricsUpdate = now
}

// GetSessionFromRequest extracts and validates a session from an HTTP request
func (sm *SessionManager) GetSessionFromRequest(r *http.Request) (*Session, error) {
	// Extract session ID from cookie
	cookie, err := r.Cookie(DefaultCookieName)
	if err != nil {
		return nil, fmt.Errorf("no session cookie found")
	}
	
	sessionID := cookie.Value
	if sessionID == "" {
		return nil, fmt.Errorf("empty session ID")
	}
	
	// Get and validate session
	return sm.ValidateSession(context.Background(), sessionID, r)
}