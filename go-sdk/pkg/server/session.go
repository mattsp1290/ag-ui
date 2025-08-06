package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/memory"
	"go.uber.org/zap"
)

// Session represents a user session
type Session struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"user_id"`
	CreatedAt    time.Time             `json:"created_at"`
	LastAccessed time.Time             `json:"last_accessed"`
	ExpiresAt    time.Time             `json:"expires_at"`
	IPAddress    string                 `json:"ip_address"`
	UserAgent    string                 `json:"user_agent"`
	IsActive     bool                   `json:"is_active"`
	Data         map[string]interface{} `json:"data"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// SessionManager manages user sessions with pluggable storage backends
type SessionManager struct {
	config      *SessionConfig
	logger      *zap.Logger
	storage     SessionStorage
	memManager  *memory.MemoryManager
	
	// Runtime state
	activeSessions int64
	totalSessions  int64
	cleanupTicker  *time.Ticker
	stopCleanup    chan struct{}
	wg             sync.WaitGroup
	
	// Synchronization
	mu sync.RWMutex
	
	// Metrics
	metrics *SessionMetrics
}

// SessionConfig configures the session manager
type SessionConfig struct {
	// Backend configuration
	Backend                string                 `json:"backend" yaml:"backend"`
	TTL                    time.Duration         `json:"ttl" yaml:"ttl"`
	CleanupInterval        time.Duration         `json:"cleanup_interval" yaml:"cleanup_interval"`
	
	// Security settings
	SecureCookies          bool                  `json:"secure_cookies" yaml:"secure_cookies"`
	HTTPOnlyCookies        bool                  `json:"http_only_cookies" yaml:"http_only_cookies"`
	SameSiteCookies        string                `json:"same_site_cookies" yaml:"same_site_cookies"`
	CookieName             string                `json:"cookie_name" yaml:"cookie_name"`
	CookiePath             string                `json:"cookie_path" yaml:"cookie_path"`
	CookieDomain           string                `json:"cookie_domain" yaml:"cookie_domain"`
	
	// Session validation
	ValidateIP             bool                  `json:"validate_ip" yaml:"validate_ip"`
	ValidateUserAgent      bool                  `json:"validate_user_agent" yaml:"validate_user_agent"`
	MaxSessionsPerUser     int                   `json:"max_sessions_per_user" yaml:"max_sessions_per_user"`
	
	// Performance settings
	MaxConcurrentSessions  int                   `json:"max_concurrent_sessions" yaml:"max_concurrent_sessions"`
	SessionPoolSize        int                   `json:"session_pool_size" yaml:"session_pool_size"`
	EnableCompression      bool                  `json:"enable_compression" yaml:"enable_compression"`
	
	// Backend-specific configurations
	Redis                  *RedisSessionConfig   `json:"redis,omitempty" yaml:"redis,omitempty"`
	Database               *DatabaseSessionConfig `json:"database,omitempty" yaml:"database,omitempty"`
	Memory                 *MemorySessionConfig  `json:"memory,omitempty" yaml:"memory,omitempty"`
}

// RedisSessionConfig configures Redis session storage
type RedisSessionConfig struct {
	Address     string `json:"address" yaml:"address"`
	Password    string `json:"password" yaml:"password"`
	DB          int    `json:"db" yaml:"db"`
	KeyPrefix   string `json:"key_prefix" yaml:"key_prefix"`
	PoolSize    int    `json:"pool_size" yaml:"pool_size"`
	MaxRetries  int    `json:"max_retries" yaml:"max_retries"`
	EnableTLS   bool   `json:"enable_tls" yaml:"enable_tls"`
}

// DatabaseSessionConfig configures database session storage
type DatabaseSessionConfig struct {
	Driver           string `json:"driver" yaml:"driver"`
	ConnectionString string `json:"connection_string" yaml:"connection_string"`
	TableName        string `json:"table_name" yaml:"table_name"`
	MaxConnections   int    `json:"max_connections" yaml:"max_connections"`
	EnableSSL        bool   `json:"enable_ssl" yaml:"enable_ssl"`
}

// MemorySessionConfig configures in-memory session storage
type MemorySessionConfig struct {
	MaxSessions    int  `json:"max_sessions" yaml:"max_sessions"`
	EnableSharding bool `json:"enable_sharding" yaml:"enable_sharding"`
	ShardCount     int  `json:"shard_count" yaml:"shard_count"`
}

// SessionMetrics tracks session management metrics
type SessionMetrics struct {
	mu                    sync.RWMutex
	TotalSessions         int64     `json:"total_sessions"`
	ActiveSessions        int64     `json:"active_sessions"`
	ExpiredSessions       int64     `json:"expired_sessions"`
	CleanupRuns           int64     `json:"cleanup_runs"`
	LastCleanup           time.Time `json:"last_cleanup"`
	MemoryUsageBytes      int64     `json:"memory_usage_bytes"`
	AverageSessionSize    int64     `json:"average_session_size"`
	SessionsPerSecond     float64   `json:"sessions_per_second"`
	LastMetricsUpdate     time.Time `json:"last_metrics_update"`
	StorageErrors         int64     `json:"storage_errors"`
	ValidationErrors      int64     `json:"validation_errors"`
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
		config:     config,
		logger:     logger,
		storage:    storage,
		memManager: memManager,
		stopCleanup: make(chan struct{}),
		metrics: &SessionMetrics{
			LastMetricsUpdate: time.Now(),
		},
	}
	
	// Start background services
	sm.startBackgroundServices()
	
	logger.Info("Session manager initialized",
		zap.String("backend", config.Backend),
		zap.Duration("ttl", config.TTL),
		zap.Duration("cleanup_interval", config.CleanupInterval))
	
	return sm, nil
}

// DefaultSessionConfig returns default session configuration
func DefaultSessionConfig() *SessionConfig {
	return &SessionConfig{
		Backend:                "memory",
		TTL:                    24 * time.Hour,
		CleanupInterval:        time.Hour,
		SecureCookies:          true,
		HTTPOnlyCookies:        true,
		SameSiteCookies:        "Strict",
		CookieName:             "session_id",
		CookiePath:             "/",
		ValidateIP:             false,
		ValidateUserAgent:      false,
		MaxSessionsPerUser:     10,
		MaxConcurrentSessions:  10000,
		SessionPoolSize:        1000,
		EnableCompression:      true,
		Memory: &MemorySessionConfig{
			MaxSessions:    10000,
			EnableSharding: true,
			ShardCount:     16,
		},
	}
}

// Validate validates session configuration
func (c *SessionConfig) Validate() error {
	if c.TTL <= 0 {
		return fmt.Errorf("session TTL must be positive")
	}
	
	if c.CleanupInterval <= 0 {
		return fmt.Errorf("cleanup interval must be positive")
	}
	
	if c.MaxConcurrentSessions <= 0 {
		return fmt.Errorf("max concurrent sessions must be positive")
	}
	
	switch c.Backend {
	case "memory":
		if c.Memory == nil {
			return fmt.Errorf("memory config required for memory backend")
		}
		if c.Memory.MaxSessions <= 0 {
			return fmt.Errorf("max sessions must be positive")
		}
		
	case "redis":
		if c.Redis == nil {
			return fmt.Errorf("redis config required for redis backend")
		}
		if c.Redis.Address == "" {
			return fmt.Errorf("redis address is required")
		}
		
	case "database":
		if c.Database == nil {
			return fmt.Errorf("database config required for database backend")
		}
		if c.Database.ConnectionString == "" {
			return fmt.Errorf("database connection string is required")
		}
		
	default:
		return fmt.Errorf("unsupported session backend: %s", c.Backend)
	}
	
	return nil
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(ctx context.Context, userID string, r *http.Request) (*Session, error) {
	// Check session limits
	if err := sm.checkSessionLimits(ctx, userID); err != nil {
		return nil, err
	}
	
	// Generate session ID
	sessionID, err := sm.generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}
	
	// Create session
	now := time.Now()
	session := &Session{
		ID:           sessionID,
		UserID:       userID,
		CreatedAt:    now,
		LastAccessed: now,
		ExpiresAt:    now.Add(sm.config.TTL),
		IPAddress:    sm.getClientIP(r),
		UserAgent:    r.UserAgent(),
		IsActive:     true,
		Data:         make(map[string]interface{}),
		Metadata:     make(map[string]interface{}),
	}
	
	// Store session
	if err := sm.storage.CreateSession(ctx, session); err != nil {
		sm.recordStorageError()
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	
	// Update metrics
	atomic.AddInt64(&sm.activeSessions, 1)
	atomic.AddInt64(&sm.totalSessions, 1)
	sm.updateMetrics()
	
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
	
	// Get session from storage
	session, err := sm.storage.GetSession(ctx, sessionID)
	if err != nil {
		sm.recordStorageError()
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	
	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		// Clean up expired session
		go func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := sm.storage.DeleteSession(cleanupCtx, sessionID); err != nil {
				sm.logger.Warn("Failed to cleanup expired session", zap.Error(err))
			}
		}()
		return nil, fmt.Errorf("session expired")
	}
	
	// Check if session is active
	if !session.IsActive {
		return nil, fmt.Errorf("session is inactive")
	}
	
	return session, nil
}

// ValidateSession validates a session with security checks
func (sm *SessionManager) ValidateSession(ctx context.Context, sessionID string, r *http.Request) (*Session, error) {
	// Get session
	session, err := sm.GetSession(ctx, sessionID)
	if err != nil {
		sm.recordValidationError()
		return nil, err
	}
	
	// IP validation
	if sm.config.ValidateIP {
		currentIP := sm.getClientIP(r)
		if currentIP != session.IPAddress {
			sm.recordValidationError()
			return nil, fmt.Errorf("IP address mismatch")
		}
	}
	
	// User agent validation
	if sm.config.ValidateUserAgent {
		currentUA := r.UserAgent()
		if currentUA != session.UserAgent {
			sm.recordValidationError()
			return nil, fmt.Errorf("user agent mismatch")
		}
	}
	
	// Update last accessed time
	session.LastAccessed = time.Now()
	session.ExpiresAt = session.LastAccessed.Add(sm.config.TTL)
	
	// Update session in storage (async)
	go func() {
		updateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := sm.storage.UpdateSession(updateCtx, session); err != nil {
			sm.logger.Warn("Failed to update session last accessed time", zap.Error(err))
		}
	}()
	
	return session, nil
}

// UpdateSession updates an existing session
func (sm *SessionManager) UpdateSession(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}
	
	// Update timestamp
	session.LastAccessed = time.Now()
	
	// Store updated session
	if err := sm.storage.UpdateSession(ctx, session); err != nil {
		sm.recordStorageError()
		return fmt.Errorf("failed to update session: %w", err)
	}
	
	return nil
}

// DeleteSession deletes a session
func (sm *SessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}
	
	// Delete from storage
	if err := sm.storage.DeleteSession(ctx, sessionID); err != nil {
		sm.recordStorageError()
		return fmt.Errorf("failed to delete session: %w", err)
	}
	
	// Update metrics
	atomic.AddInt64(&sm.activeSessions, -1)
	sm.updateMetrics()
	
	sm.logger.Debug("Session deleted", zap.String("session_id", sessionID))
	
	return nil
}

// GetUserSessions retrieves all sessions for a user
func (sm *SessionManager) GetUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}
	
	sessions, err := sm.storage.GetUserSessions(ctx, userID)
	if err != nil {
		sm.recordStorageError()
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}
	
	// Filter expired sessions
	activeSessions := make([]*Session, 0, len(sessions))
	now := time.Now()
	
	for _, session := range sessions {
		if session.IsActive && now.Before(session.ExpiresAt) {
			activeSessions = append(activeSessions, session)
		}
	}
	
	return activeSessions, nil
}

// DeleteUserSessions deletes all sessions for a user
func (sm *SessionManager) DeleteUserSessions(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}
	
	// Get current session count for metrics
	sessions, err := sm.storage.GetUserSessions(ctx, userID)
	if err != nil {
		sm.recordStorageError()
		return fmt.Errorf("failed to get user sessions for deletion: %w", err)
	}
	
	// Delete sessions
	if err := sm.storage.DeleteUserSessions(ctx, userID); err != nil {
		sm.recordStorageError()
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}
	
	// Update metrics
	atomic.AddInt64(&sm.activeSessions, -int64(len(sessions)))
	sm.updateMetrics()
	
	sm.logger.Info("User sessions deleted",
		zap.String("user_id", userID),
		zap.Int("count", len(sessions)))
	
	return nil
}

// SessionMiddleware provides HTTP middleware for session management
func (sm *SessionManager) SessionMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract session ID from cookie
			cookie, err := r.Cookie(sm.config.CookieName)
			if err != nil {
				// No session cookie, continue without session
				next.ServeHTTP(w, r)
				return
			}
			
			// Validate session
			session, err := sm.ValidateSession(r.Context(), cookie.Value, r)
			if err != nil {
				// Invalid session, clear cookie and continue
				sm.clearSessionCookie(w)
				next.ServeHTTP(w, r)
				return
			}
			
			// Add session to request context
			ctx := context.WithValue(r.Context(), "session", session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SetSessionCookie sets a session cookie in the response
func (sm *SessionManager) SetSessionCookie(w http.ResponseWriter, sessionID string) {
	cookie := &http.Cookie{
		Name:     sm.config.CookieName,
		Value:    sessionID,
		Path:     sm.config.CookiePath,
		Domain:   sm.config.CookieDomain,
		Expires:  time.Now().Add(sm.config.TTL),
		Secure:   sm.config.SecureCookies,
		HttpOnly: sm.config.HTTPOnlyCookies,
	}
	
	// Set SameSite attribute
	switch strings.ToLower(sm.config.SameSiteCookies) {
	case "strict":
		cookie.SameSite = http.SameSiteStrictMode
	case "lax":
		cookie.SameSite = http.SameSiteLaxMode
	case "none":
		cookie.SameSite = http.SameSiteNoneMode
	default:
		cookie.SameSite = http.SameSiteDefaultMode
	}
	
	http.SetCookie(w, cookie)
}

// ClearSessionCookie clears the session cookie
func (sm *SessionManager) ClearSessionCookie(w http.ResponseWriter) {
	sm.clearSessionCookie(w)
}

// GetSessionFromRequest extracts session from HTTP request
func (sm *SessionManager) GetSessionFromRequest(r *http.Request) (*Session, bool) {
	session, ok := r.Context().Value("session").(*Session)
	return session, ok
}

// CleanupExpiredSessions removes expired sessions
func (sm *SessionManager) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	startTime := time.Now()
	
	cleaned, err := sm.storage.CleanupExpiredSessions(ctx)
	if err != nil {
		sm.recordStorageError()
		return 0, fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}
	
	// Update metrics
	sm.metrics.mu.Lock()
	sm.metrics.ExpiredSessions += cleaned
	sm.metrics.CleanupRuns++
	sm.metrics.LastCleanup = time.Now()
	sm.metrics.mu.Unlock()
	
	atomic.AddInt64(&sm.activeSessions, -cleaned)
	sm.updateMetrics()
	
	duration := time.Since(startTime)
	sm.logger.Info("Session cleanup completed",
		zap.Int64("cleaned", cleaned),
		zap.Duration("duration", duration))
	
	return cleaned, nil
}

// GetMetrics returns session management metrics
func (sm *SessionManager) GetMetrics() *SessionMetrics {
	sm.metrics.mu.RLock()
	defer sm.metrics.mu.RUnlock()
	
	// Create a copy of metrics
	metrics := &SessionMetrics{
		TotalSessions:         sm.metrics.TotalSessions,
		ActiveSessions:        atomic.LoadInt64(&sm.activeSessions),
		ExpiredSessions:       sm.metrics.ExpiredSessions,
		CleanupRuns:           sm.metrics.CleanupRuns,
		LastCleanup:           sm.metrics.LastCleanup,
		MemoryUsageBytes:      sm.metrics.MemoryUsageBytes,
		AverageSessionSize:    sm.metrics.AverageSessionSize,
		SessionsPerSecond:     sm.metrics.SessionsPerSecond,
		LastMetricsUpdate:     sm.metrics.LastMetricsUpdate,
		StorageErrors:         sm.metrics.StorageErrors,
		ValidationErrors:      sm.metrics.ValidationErrors,
	}
	
	return metrics
}

// Stats returns session manager statistics
func (sm *SessionManager) Stats() map[string]interface{} {
	metrics := sm.GetMetrics()
	storageStats := sm.storage.Stats()
	
	stats := map[string]interface{}{
		"total_sessions":         metrics.TotalSessions,
		"active_sessions":        metrics.ActiveSessions,
		"expired_sessions":       metrics.ExpiredSessions,
		"cleanup_runs":           metrics.CleanupRuns,
		"last_cleanup":           metrics.LastCleanup,
		"memory_usage_bytes":     metrics.MemoryUsageBytes,
		"average_session_size":   metrics.AverageSessionSize,
		"sessions_per_second":    metrics.SessionsPerSecond,
		"storage_errors":         metrics.StorageErrors,
		"validation_errors":      metrics.ValidationErrors,
		"backend":                sm.config.Backend,
		"ttl_seconds":            sm.config.TTL.Seconds(),
		"cleanup_interval_seconds": sm.config.CleanupInterval.Seconds(),
	}
	
	// Add storage-specific stats
	for k, v := range storageStats {
		stats["storage_"+k] = v
	}
	
	return stats
}

// Close shuts down the session manager
func (sm *SessionManager) Close() error {
	sm.logger.Info("Shutting down session manager")
	
	// Stop background services
	close(sm.stopCleanup)
	if sm.cleanupTicker != nil {
		sm.cleanupTicker.Stop()
	}
	
	// Wait for cleanup operations to complete
	sm.wg.Wait()
	
	// Stop memory manager
	if sm.memManager != nil {
		sm.memManager.Stop()
	}
	
	// Close storage
	if err := sm.storage.Close(); err != nil {
		return fmt.Errorf("failed to close session storage: %w", err)
	}
	
	sm.logger.Info("Session manager shutdown complete")
	return nil
}

// Private methods

func (sm *SessionManager) startBackgroundServices() {
	// Start memory manager
	sm.memManager.Start()
	
	// Register memory pressure callback
	sm.memManager.OnMemoryPressure(func(level memory.MemoryPressureLevel) {
		if level >= memory.MemoryPressureHigh {
			sm.logger.Warn("High memory pressure detected, running session cleanup",
				zap.String("pressure_level", level.String()))
			
			// Trigger cleanup
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if _, err := sm.CleanupExpiredSessions(ctx); err != nil {
					sm.logger.Error("Emergency session cleanup failed", zap.Error(err))
				}
			}()
		}
	})
	
	// Start cleanup timer
	sm.cleanupTicker = time.NewTicker(sm.config.CleanupInterval)
	sm.wg.Add(1)
	
	go func() {
		defer sm.wg.Done()
		
		for {
			select {
			case <-sm.cleanupTicker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				if _, err := sm.CleanupExpiredSessions(ctx); err != nil {
					sm.logger.Error("Scheduled session cleanup failed", zap.Error(err))
				}
				cancel()
				
			case <-sm.stopCleanup:
				return
			}
		}
	}()
}

func (sm *SessionManager) generateSessionID() (string, error) {
	// Generate 32 random bytes
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	
	// Hash the bytes for additional security
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:]), nil
}

func (sm *SessionManager) getClientIP(r *http.Request) string {
	// Check for forwarded IP headers
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}
	
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	
	// Extract IP from remote address
	parts := strings.Split(r.RemoteAddr, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	
	return r.RemoteAddr
}

func (sm *SessionManager) checkSessionLimits(ctx context.Context, userID string) error {
	// Check concurrent session limit
	currentSessions := atomic.LoadInt64(&sm.activeSessions)
	if currentSessions >= int64(sm.config.MaxConcurrentSessions) {
		// Force cleanup to make room
		if _, err := sm.CleanupExpiredSessions(ctx); err != nil {
			sm.logger.Warn("Failed to cleanup sessions for limit check", zap.Error(err))
		}
		
		// Check again after cleanup
		currentSessions = atomic.LoadInt64(&sm.activeSessions)
		if currentSessions >= int64(sm.config.MaxConcurrentSessions) {
			return fmt.Errorf("maximum concurrent sessions reached")
		}
	}
	
	// Check user session limit
	if sm.config.MaxSessionsPerUser > 0 {
		userSessions, err := sm.storage.GetUserSessions(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to check user session limit: %w", err)
		}
		
		// Count active sessions
		activeCount := 0
		now := time.Now()
		for _, session := range userSessions {
			if session.IsActive && now.Before(session.ExpiresAt) {
				activeCount++
			}
		}
		
		if activeCount >= sm.config.MaxSessionsPerUser {
			return fmt.Errorf("maximum sessions per user reached")
		}
	}
	
	return nil
}

func (sm *SessionManager) clearSessionCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     sm.config.CookieName,
		Value:    "",
		Path:     sm.config.CookiePath,
		Domain:   sm.config.CookieDomain,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		Secure:   sm.config.SecureCookies,
		HttpOnly: sm.config.HTTPOnlyCookies,
	}
	
	http.SetCookie(w, cookie)
}

func (sm *SessionManager) updateMetrics() {
	sm.metrics.mu.Lock()
	defer sm.metrics.mu.Unlock()
	
	sm.metrics.TotalSessions = atomic.LoadInt64(&sm.totalSessions)
	sm.metrics.ActiveSessions = atomic.LoadInt64(&sm.activeSessions)
	
	// Calculate sessions per second
	now := time.Now()
	if !sm.metrics.LastMetricsUpdate.IsZero() {
		duration := now.Sub(sm.metrics.LastMetricsUpdate).Seconds()
		if duration > 0 {
			sessionsDiff := sm.metrics.TotalSessions - atomic.LoadInt64(&sm.totalSessions)
			sm.metrics.SessionsPerSecond = float64(sessionsDiff) / duration
		}
	}
	
	sm.metrics.LastMetricsUpdate = now
}

func (sm *SessionManager) recordStorageError() {
	atomic.AddInt64(&sm.metrics.StorageErrors, 1)
}

func (sm *SessionManager) recordValidationError() {
	atomic.AddInt64(&sm.metrics.ValidationErrors, 1)
}

// Storage backend implementations

func createSessionStorage(config *SessionConfig, logger *zap.Logger) (SessionStorage, error) {
	switch config.Backend {
	case "memory":
		return NewMemorySessionStorage(config.Memory, logger)
	case "redis":
		return NewRedisSessionStorage(config.Redis, logger)
	case "database":
		return NewDatabaseSessionStorage(config.Database, logger)
	default:
		return nil, fmt.Errorf("unsupported session backend: %s", config.Backend)
	}
}

// MemorySessionStorage implements in-memory session storage
type MemorySessionStorage struct {
	config   *MemorySessionConfig
	logger   *zap.Logger
	sessions map[string]*Session
	userSessions map[string][]string
	shards   []*SessionShard
	mu       sync.RWMutex
}

// SessionShard represents a shard for memory storage
type SessionShard struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewMemorySessionStorage creates a new memory session storage
func NewMemorySessionStorage(config *MemorySessionConfig, logger *zap.Logger) (*MemorySessionStorage, error) {
	if config == nil {
		config = &MemorySessionConfig{
			MaxSessions:    10000,
			EnableSharding: true,
			ShardCount:     16,
		}
	}
	
	storage := &MemorySessionStorage{
		config:       config,
		logger:       logger,
		sessions:     make(map[string]*Session),
		userSessions: make(map[string][]string),
	}
	
	// Initialize shards if sharding is enabled
	if config.EnableSharding {
		storage.shards = make([]*SessionShard, config.ShardCount)
		for i := 0; i < config.ShardCount; i++ {
			storage.shards[i] = &SessionShard{
				sessions: make(map[string]*Session),
			}
		}
	}
	
	logger.Info("Memory session storage initialized",
		zap.Int("max_sessions", config.MaxSessions),
		zap.Bool("sharding_enabled", config.EnableSharding),
		zap.Int("shard_count", config.ShardCount))
	
	return storage, nil
}

func (m *MemorySessionStorage) getShard(sessionID string) *SessionShard {
	if !m.config.EnableSharding || len(m.shards) == 0 {
		return nil
	}
	
	// Use FNV hash for consistent sharding
	h := fnv.New32a()
	h.Write([]byte(sessionID))
	shardIndex := int(h.Sum32()) % len(m.shards)
	return m.shards[shardIndex]
}

func (m *MemorySessionStorage) CreateSession(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}
	
	if shard := m.getShard(session.ID); shard != nil {
		shard.mu.Lock()
		defer shard.mu.Unlock()
		
		if len(shard.sessions) >= m.config.MaxSessions/len(m.shards) {
			return fmt.Errorf("shard session limit reached")
		}
		
		shard.sessions[session.ID] = session
	} else {
		m.mu.Lock()
		defer m.mu.Unlock()
		
		if len(m.sessions) >= m.config.MaxSessions {
			return fmt.Errorf("session limit reached")
		}
		
		m.sessions[session.ID] = session
		
		// Update user sessions index
		if m.userSessions[session.UserID] == nil {
			m.userSessions[session.UserID] = make([]string, 0)
		}
		m.userSessions[session.UserID] = append(m.userSessions[session.UserID], session.ID)
	}
	
	return nil
}

func (m *MemorySessionStorage) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if shard := m.getShard(sessionID); shard != nil {
		shard.mu.RLock()
		defer shard.mu.RUnlock()
		
		session, exists := shard.sessions[sessionID]
		if !exists {
			return nil, fmt.Errorf("session not found")
		}
		
		// Return a copy to prevent external modification
		sessionCopy := *session
		return &sessionCopy, nil
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}
	
	// Return a copy to prevent external modification
	sessionCopy := *session
	return &sessionCopy, nil
}

func (m *MemorySessionStorage) UpdateSession(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}
	
	if shard := m.getShard(session.ID); shard != nil {
		shard.mu.Lock()
		defer shard.mu.Unlock()
		
		if _, exists := shard.sessions[session.ID]; !exists {
			return fmt.Errorf("session not found")
		}
		
		shard.sessions[session.ID] = session
		return nil
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, exists := m.sessions[session.ID]; !exists {
		return fmt.Errorf("session not found")
	}
	
	m.sessions[session.ID] = session
	return nil
}

func (m *MemorySessionStorage) DeleteSession(ctx context.Context, sessionID string) error {
	if shard := m.getShard(sessionID); shard != nil {
		shard.mu.Lock()
		defer shard.mu.Unlock()
		
		delete(shard.sessions, sessionID)
		return nil
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	session, exists := m.sessions[sessionID]
	if exists {
		delete(m.sessions, sessionID)
		
		// Update user sessions index
		if userSessions, ok := m.userSessions[session.UserID]; ok {
			for i, id := range userSessions {
				if id == sessionID {
					m.userSessions[session.UserID] = append(userSessions[:i], userSessions[i+1:]...)
					break
				}
			}
			
			// Clean up empty user session lists
			if len(m.userSessions[session.UserID]) == 0 {
				delete(m.userSessions, session.UserID)
			}
		}
	}
	
	return nil
}

func (m *MemorySessionStorage) GetUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	if m.config.EnableSharding {
		// Search all shards for user sessions
		var sessions []*Session
		for _, shard := range m.shards {
			shard.mu.RLock()
			for _, session := range shard.sessions {
				if session.UserID == userID {
					sessionCopy := *session
					sessions = append(sessions, &sessionCopy)
				}
			}
			shard.mu.RUnlock()
		}
		return sessions, nil
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	sessionIDs, exists := m.userSessions[userID]
	if !exists {
		return []*Session{}, nil
	}
	
	sessions := make([]*Session, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		if session, exists := m.sessions[sessionID]; exists {
			sessionCopy := *session
			sessions = append(sessions, &sessionCopy)
		}
	}
	
	return sessions, nil
}

func (m *MemorySessionStorage) DeleteUserSessions(ctx context.Context, userID string) error {
	if m.config.EnableSharding {
		// Delete from all shards
		for _, shard := range m.shards {
			shard.mu.Lock()
			for sessionID, session := range shard.sessions {
				if session.UserID == userID {
					delete(shard.sessions, sessionID)
				}
			}
			shard.mu.Unlock()
		}
		return nil
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	sessionIDs, exists := m.userSessions[userID]
	if !exists {
		return nil
	}
	
	// Delete all user sessions
	for _, sessionID := range sessionIDs {
		delete(m.sessions, sessionID)
	}
	
	// Remove user from index
	delete(m.userSessions, userID)
	
	return nil
}

func (m *MemorySessionStorage) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	now := time.Now()
	var cleaned int64
	
	if m.config.EnableSharding {
		// Cleanup all shards
		for _, shard := range m.shards {
			shard.mu.Lock()
			var toDelete []string
			for sessionID, session := range shard.sessions {
				if now.After(session.ExpiresAt) || !session.IsActive {
					toDelete = append(toDelete, sessionID)
				}
			}
			
			for _, sessionID := range toDelete {
				delete(shard.sessions, sessionID)
				cleaned++
			}
			shard.mu.Unlock()
		}
		return cleaned, nil
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Find expired sessions
	var expiredSessions []string
	for sessionID, session := range m.sessions {
		if now.After(session.ExpiresAt) || !session.IsActive {
			expiredSessions = append(expiredSessions, sessionID)
		}
	}
	
	// Remove expired sessions
	for _, sessionID := range expiredSessions {
		if session, exists := m.sessions[sessionID]; exists {
			delete(m.sessions, sessionID)
			cleaned++
			
			// Update user sessions index
			if userSessions, ok := m.userSessions[session.UserID]; ok {
				for i, id := range userSessions {
					if id == sessionID {
						m.userSessions[session.UserID] = append(userSessions[:i], userSessions[i+1:]...)
						break
					}
				}
				
				// Clean up empty user session lists
				if len(m.userSessions[session.UserID]) == 0 {
					delete(m.userSessions, session.UserID)
				}
			}
		}
	}
	
	return cleaned, nil
}

func (m *MemorySessionStorage) GetActiveSessions(ctx context.Context, limit int) ([]*Session, error) {
	var sessions []*Session
	now := time.Now()
	
	if m.config.EnableSharding {
		// Collect from all shards
		for _, shard := range m.shards {
			shard.mu.RLock()
			for _, session := range shard.sessions {
				if session.IsActive && now.Before(session.ExpiresAt) {
					sessionCopy := *session
					sessions = append(sessions, &sessionCopy)
					if len(sessions) >= limit {
						shard.mu.RUnlock()
						return sessions, nil
					}
				}
			}
			shard.mu.RUnlock()
		}
		return sessions, nil
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for _, session := range m.sessions {
		if session.IsActive && now.Before(session.ExpiresAt) {
			sessionCopy := *session
			sessions = append(sessions, &sessionCopy)
			if len(sessions) >= limit {
				break
			}
		}
	}
	
	return sessions, nil
}

func (m *MemorySessionStorage) CountSessions(ctx context.Context) (int64, error) {
	if m.config.EnableSharding {
		var count int64
		for _, shard := range m.shards {
			shard.mu.RLock()
			count += int64(len(shard.sessions))
			shard.mu.RUnlock()
		}
		return count, nil
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return int64(len(m.sessions)), nil
}

func (m *MemorySessionStorage) Close() error {
	m.logger.Info("Closing memory session storage")
	
	if m.config.EnableSharding {
		for _, shard := range m.shards {
			shard.mu.Lock()
			shard.sessions = make(map[string]*Session)
			shard.mu.Unlock()
		}
	} else {
		m.mu.Lock()
		m.sessions = make(map[string]*Session)
		m.userSessions = make(map[string][]string)
		m.mu.Unlock()
	}
	
	return nil
}

func (m *MemorySessionStorage) Ping(ctx context.Context) error {
	return nil // Memory storage is always available
}

func (m *MemorySessionStorage) Stats() map[string]interface{} {
	count, _ := m.CountSessions(context.Background())
	
	stats := map[string]interface{}{
		"type":               "memory",
		"session_count":      count,
		"max_sessions":       m.config.MaxSessions,
		"sharding_enabled":   m.config.EnableSharding,
		"shard_count":        m.config.ShardCount,
	}
	
	if m.config.EnableSharding {
		shardCounts := make([]int, len(m.shards))
		for i, shard := range m.shards {
			shard.mu.RLock()
			shardCounts[i] = len(shard.sessions)
			shard.mu.RUnlock()
		}
		stats["shard_counts"] = shardCounts
	}
	
	return stats
}

// RedisSessionStorage implements Redis-based session storage
// Note: This is a stub implementation - full Redis implementation would require redis client
type RedisSessionStorage struct {
	config *RedisSessionConfig
	logger *zap.Logger
}

// NewRedisSessionStorage creates a new Redis session storage
func NewRedisSessionStorage(config *RedisSessionConfig, logger *zap.Logger) (*RedisSessionStorage, error) {
	if config == nil {
		return nil, fmt.Errorf("redis config cannot be nil")
	}
	
	storage := &RedisSessionStorage{
		config: config,
		logger: logger,
	}
	
	// TODO: Initialize Redis client when redis package is available
	logger.Info("Redis session storage initialized (stub)",
		zap.String("address", config.Address),
		zap.String("key_prefix", config.KeyPrefix))
	
	return storage, nil
}

func (r *RedisSessionStorage) CreateSession(ctx context.Context, session *Session) error {
	// TODO: Implement Redis session creation
	return fmt.Errorf("redis session storage not fully implemented")
}

func (r *RedisSessionStorage) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	// TODO: Implement Redis session retrieval
	return nil, fmt.Errorf("redis session storage not fully implemented")
}

func (r *RedisSessionStorage) UpdateSession(ctx context.Context, session *Session) error {
	// TODO: Implement Redis session update
	return fmt.Errorf("redis session storage not fully implemented")
}

func (r *RedisSessionStorage) DeleteSession(ctx context.Context, sessionID string) error {
	// TODO: Implement Redis session deletion
	return fmt.Errorf("redis session storage not fully implemented")
}

func (r *RedisSessionStorage) GetUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	// TODO: Implement Redis user sessions retrieval
	return nil, fmt.Errorf("redis session storage not fully implemented")
}

func (r *RedisSessionStorage) DeleteUserSessions(ctx context.Context, userID string) error {
	// TODO: Implement Redis user sessions deletion
	return fmt.Errorf("redis session storage not fully implemented")
}

func (r *RedisSessionStorage) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	// TODO: Implement Redis session cleanup
	return 0, fmt.Errorf("redis session storage not fully implemented")
}

func (r *RedisSessionStorage) GetActiveSessions(ctx context.Context, limit int) ([]*Session, error) {
	// TODO: Implement Redis active sessions retrieval
	return nil, fmt.Errorf("redis session storage not fully implemented")
}

func (r *RedisSessionStorage) CountSessions(ctx context.Context) (int64, error) {
	// TODO: Implement Redis session count
	return 0, fmt.Errorf("redis session storage not fully implemented")
}

func (r *RedisSessionStorage) Close() error {
	r.logger.Info("Closing Redis session storage")
	// TODO: Close Redis client
	return nil
}

func (r *RedisSessionStorage) Ping(ctx context.Context) error {
	// TODO: Ping Redis server
	return fmt.Errorf("redis session storage not fully implemented")
}

func (r *RedisSessionStorage) Stats() map[string]interface{} {
	return map[string]interface{}{
		"type":    "redis",
		"address": r.config.Address,
		"status":  "stub_implementation",
	}
}

// DatabaseSessionStorage implements database-based session storage
type DatabaseSessionStorage struct {
	config *DatabaseSessionConfig
	logger *zap.Logger
	db     *sql.DB
}

// NewDatabaseSessionStorage creates a new database session storage
func NewDatabaseSessionStorage(config *DatabaseSessionConfig, logger *zap.Logger) (*DatabaseSessionStorage, error) {
	if config == nil {
		return nil, fmt.Errorf("database config cannot be nil")
	}
	
	// Set defaults
	if config.TableName == "" {
		config.TableName = "sessions"
	}
	if config.MaxConnections == 0 {
		config.MaxConnections = 10
	}
	
	// Open database connection
	db, err := sql.Open(config.Driver, config.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	
	// Configure connection pool
	db.SetMaxOpenConns(config.MaxConnections)
	db.SetMaxIdleConns(config.MaxConnections / 2)
	db.SetConnMaxLifetime(5 * time.Minute)
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	
	storage := &DatabaseSessionStorage{
		config: config,
		logger: logger,
		db:     db,
	}
	
	// Initialize schema
	if err := storage.initSchema(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}
	
	logger.Info("Database session storage initialized",
		zap.String("driver", config.Driver),
		zap.String("table_name", config.TableName),
		zap.Int("max_connections", config.MaxConnections))
	
	return storage, nil
}

func (d *DatabaseSessionStorage) initSchema(ctx context.Context) error {
	var createTableSQL string
	
	switch d.config.Driver {
	case "postgres", "postgresql":
		createTableSQL = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id VARCHAR(64) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				created_at TIMESTAMP WITH TIME ZONE NOT NULL,
				last_accessed TIMESTAMP WITH TIME ZONE NOT NULL,
				expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
				ip_address INET,
				user_agent TEXT,
				is_active BOOLEAN NOT NULL DEFAULT true,
				data JSONB,
				metadata JSONB,
				INDEX (user_id),
				INDEX (expires_at),
				INDEX (last_accessed)
			)`, d.config.TableName)
		
	case "mysql":
		createTableSQL = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id VARCHAR(64) PRIMARY KEY,
				user_id VARCHAR(255) NOT NULL,
				created_at TIMESTAMP NOT NULL,
				last_accessed TIMESTAMP NOT NULL,
				expires_at TIMESTAMP NOT NULL,
				ip_address VARCHAR(45),
				user_agent TEXT,
				is_active BOOLEAN NOT NULL DEFAULT true,
				data JSON,
				metadata JSON,
				INDEX idx_user_id (user_id),
				INDEX idx_expires_at (expires_at),
				INDEX idx_last_accessed (last_accessed)
			)`, d.config.TableName)
		
	case "sqlite3", "sqlite":
		createTableSQL = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id TEXT PRIMARY KEY,
				user_id TEXT NOT NULL,
				created_at DATETIME NOT NULL,
				last_accessed DATETIME NOT NULL,
				expires_at DATETIME NOT NULL,
				ip_address TEXT,
				user_agent TEXT,
				is_active BOOLEAN NOT NULL DEFAULT 1,
				data TEXT,
				metadata TEXT
			)`, d.config.TableName)
		
		// Create indexes separately for SQLite
		indexQueries := []string{
			fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_user_id ON %s (user_id)", d.config.TableName, d.config.TableName),
			fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_expires_at ON %s (expires_at)", d.config.TableName, d.config.TableName),
			fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_last_accessed ON %s (last_accessed)", d.config.TableName, d.config.TableName),
		}
		
		if _, err := d.db.ExecContext(ctx, createTableSQL); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
		
		for _, indexSQL := range indexQueries {
			if _, err := d.db.ExecContext(ctx, indexSQL); err != nil {
				return fmt.Errorf("failed to create index: %w", err)
			}
		}
		return nil
		
	default:
		return fmt.Errorf("unsupported database driver: %s", d.config.Driver)
	}
	
	if _, err := d.db.ExecContext(ctx, createTableSQL); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	
	return nil
}

func (d *DatabaseSessionStorage) CreateSession(ctx context.Context, session *Session) error {
	dataJSON, err := json.Marshal(session.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}
	
	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal session metadata: %w", err)
	}
	
	query := fmt.Sprintf(`
		INSERT INTO %s (id, user_id, created_at, last_accessed, expires_at, ip_address, user_agent, is_active, data, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, d.config.TableName)
	
	// Adjust placeholders for MySQL
	if d.config.Driver == "mysql" {
		query = strings.ReplaceAll(query, "$", "?")
	}
	
	_, err = d.db.ExecContext(ctx, query,
		session.ID, session.UserID, session.CreatedAt, session.LastAccessed,
		session.ExpiresAt, session.IPAddress, session.UserAgent, session.IsActive,
		string(dataJSON), string(metadataJSON))
	
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	
	return nil
}

func (d *DatabaseSessionStorage) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	query := fmt.Sprintf(`
		SELECT id, user_id, created_at, last_accessed, expires_at, ip_address, user_agent, is_active, data, metadata
		FROM %s WHERE id = $1
	`, d.config.TableName)
	
	// Adjust placeholders for MySQL
	if d.config.Driver == "mysql" {
		query = strings.ReplaceAll(query, "$1", "?")
	}
	
	row := d.db.QueryRowContext(ctx, query, sessionID)
	
	var session Session
	var dataJSON, metadataJSON string
	
	err := row.Scan(&session.ID, &session.UserID, &session.CreatedAt,
		&session.LastAccessed, &session.ExpiresAt, &session.IPAddress,
		&session.UserAgent, &session.IsActive, &dataJSON, &metadataJSON)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	
	// Unmarshal JSON data
	if err := json.Unmarshal([]byte(dataJSON), &session.Data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}
	
	if err := json.Unmarshal([]byte(metadataJSON), &session.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session metadata: %w", err)
	}
	
	return &session, nil
}

func (d *DatabaseSessionStorage) UpdateSession(ctx context.Context, session *Session) error {
	dataJSON, err := json.Marshal(session.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}
	
	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal session metadata: %w", err)
	}
	
	query := fmt.Sprintf(`
		UPDATE %s SET last_accessed = $1, expires_at = $2, is_active = $3, data = $4, metadata = $5
		WHERE id = $6
	`, d.config.TableName)
	
	// Adjust placeholders for MySQL
	if d.config.Driver == "mysql" {
		query = strings.ReplaceAll(query, "$", "?")
	}
	
	_, err = d.db.ExecContext(ctx, query,
		session.LastAccessed, session.ExpiresAt, session.IsActive,
		string(dataJSON), string(metadataJSON), session.ID)
	
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}
	
	return nil
}

func (d *DatabaseSessionStorage) DeleteSession(ctx context.Context, sessionID string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", d.config.TableName)
	
	// Adjust placeholders for MySQL
	if d.config.Driver == "mysql" {
		query = strings.ReplaceAll(query, "$1", "?")
	}
	
	_, err := d.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	
	return nil
}

func (d *DatabaseSessionStorage) GetUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	query := fmt.Sprintf(`
		SELECT id, user_id, created_at, last_accessed, expires_at, ip_address, user_agent, is_active, data, metadata
		FROM %s WHERE user_id = $1 ORDER BY last_accessed DESC
	`, d.config.TableName)
	
	// Adjust placeholders for MySQL
	if d.config.Driver == "mysql" {
		query = strings.ReplaceAll(query, "$1", "?")
	}
	
	rows, err := d.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user sessions: %w", err)
	}
	defer rows.Close()
	
	var sessions []*Session
	for rows.Next() {
		var session Session
		var dataJSON, metadataJSON string
		
		err := rows.Scan(&session.ID, &session.UserID, &session.CreatedAt,
			&session.LastAccessed, &session.ExpiresAt, &session.IPAddress,
			&session.UserAgent, &session.IsActive, &dataJSON, &metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		
		// Unmarshal JSON data
		if err := json.Unmarshal([]byte(dataJSON), &session.Data); err != nil {
			d.logger.Warn("Failed to unmarshal session data", zap.Error(err))
			session.Data = make(map[string]interface{})
		}
		
		if err := json.Unmarshal([]byte(metadataJSON), &session.Metadata); err != nil {
			d.logger.Warn("Failed to unmarshal session metadata", zap.Error(err))
			session.Metadata = make(map[string]interface{})
		}
		
		sessions = append(sessions, &session)
	}
	
	return sessions, nil
}

func (d *DatabaseSessionStorage) DeleteUserSessions(ctx context.Context, userID string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE user_id = $1", d.config.TableName)
	
	// Adjust placeholders for MySQL
	if d.config.Driver == "mysql" {
		query = strings.ReplaceAll(query, "$1", "?")
	}
	
	_, err := d.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}
	
	return nil
}

func (d *DatabaseSessionStorage) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	query := fmt.Sprintf("DELETE FROM %s WHERE expires_at < $1 OR is_active = false", d.config.TableName)
	
	// Adjust placeholders for MySQL
	if d.config.Driver == "mysql" {
		query = strings.ReplaceAll(query, "$1", "?")
	}
	
	result, err := d.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired sessions: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	return rowsAffected, nil
}

func (d *DatabaseSessionStorage) GetActiveSessions(ctx context.Context, limit int) ([]*Session, error) {
	query := fmt.Sprintf(`
		SELECT id, user_id, created_at, last_accessed, expires_at, ip_address, user_agent, is_active, data, metadata
		FROM %s WHERE is_active = true AND expires_at > $1 
		ORDER BY last_accessed DESC LIMIT $2
	`, d.config.TableName)
	
	// Adjust placeholders for MySQL
	if d.config.Driver == "mysql" {
		query = strings.ReplaceAll(query, "$", "?")
	}
	
	// SQLite uses LIMIT differently
	if d.config.Driver == "sqlite3" || d.config.Driver == "sqlite" {
		query = strings.ReplaceAll(query, "$1", "?")
		query = strings.ReplaceAll(query, "$2", "?")
	}
	
	rows, err := d.db.QueryContext(ctx, query, time.Now(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query active sessions: %w", err)
	}
	defer rows.Close()
	
	var sessions []*Session
	for rows.Next() {
		var session Session
		var dataJSON, metadataJSON string
		
		err := rows.Scan(&session.ID, &session.UserID, &session.CreatedAt,
			&session.LastAccessed, &session.ExpiresAt, &session.IPAddress,
			&session.UserAgent, &session.IsActive, &dataJSON, &metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		
		// Unmarshal JSON data
		if err := json.Unmarshal([]byte(dataJSON), &session.Data); err != nil {
			d.logger.Warn("Failed to unmarshal session data", zap.Error(err))
			session.Data = make(map[string]interface{})
		}
		
		if err := json.Unmarshal([]byte(metadataJSON), &session.Metadata); err != nil {
			d.logger.Warn("Failed to unmarshal session metadata", zap.Error(err))
			session.Metadata = make(map[string]interface{})
		}
		
		sessions = append(sessions, &session)
	}
	
	return sessions, nil
}

func (d *DatabaseSessionStorage) CountSessions(ctx context.Context) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE is_active = true AND expires_at > $1", d.config.TableName)
	
	// Adjust placeholders for MySQL
	if d.config.Driver == "mysql" {
		query = strings.ReplaceAll(query, "$1", "?")
	}
	
	var count int64
	err := d.db.QueryRowContext(ctx, query, time.Now()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sessions: %w", err)
	}
	
	return count, nil
}

func (d *DatabaseSessionStorage) Close() error {
	d.logger.Info("Closing database session storage")
	return d.db.Close()
}

func (d *DatabaseSessionStorage) Ping(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

func (d *DatabaseSessionStorage) Stats() map[string]interface{} {
	stats := map[string]interface{}{
		"type":         "database",
		"driver":       d.config.Driver,
		"table_name":   d.config.TableName,
	}
	
	// Add database connection stats if available
	if d.db != nil {
		dbStats := d.db.Stats()
		stats["max_open_connections"] = dbStats.MaxOpenConnections
		stats["open_connections"] = dbStats.OpenConnections
		stats["in_use"] = dbStats.InUse
		stats["idle"] = dbStats.Idle
	}
	
	return stats
}

// Utility functions for session helpers

// GetSessionFromContext extracts session from request context
func GetSessionFromContext(ctx context.Context) (*Session, bool) {
	session, ok := ctx.Value("session").(*Session)
	return session, ok
}

// RequireSession is a middleware that requires a valid session
func RequireSessionMiddleware(sm *SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if session exists in context
			if _, ok := GetSessionFromContext(r.Context()); !ok {
				http.Error(w, "Session required", http.StatusUnauthorized)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// SessionData provides typed access to session data
type SessionData struct {
	session *Session
}

// NewSessionData creates a new SessionData wrapper
func NewSessionData(session *Session) *SessionData {
	if session.Data == nil {
		session.Data = make(map[string]interface{})
	}
	return &SessionData{session: session}
}

// Set stores a value in session data
func (sd *SessionData) Set(key string, value interface{}) {
	sd.session.Data[key] = value
}

// Get retrieves a value from session data
func (sd *SessionData) Get(key string) (interface{}, bool) {
	value, exists := sd.session.Data[key]
	return value, exists
}

// GetString retrieves a string value from session data
func (sd *SessionData) GetString(key string) (string, bool) {
	if value, exists := sd.session.Data[key]; exists {
		if str, ok := value.(string); ok {
			return str, true
		}
	}
	return "", false
}

// GetInt retrieves an int value from session data
func (sd *SessionData) GetInt(key string) (int, bool) {
	if value, exists := sd.session.Data[key]; exists {
		switch v := value.(type) {
		case int:
			return v, true
		case float64:
			return int(v), true
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i, true
			}
		}
	}
	return 0, false
}

// GetBool retrieves a bool value from session data
func (sd *SessionData) GetBool(key string) (bool, bool) {
	if value, exists := sd.session.Data[key]; exists {
		if b, ok := value.(bool); ok {
			return b, true
		}
	}
	return false, false
}

// Delete removes a value from session data
func (sd *SessionData) Delete(key string) {
	delete(sd.session.Data, key)
}

// Clear removes all data from the session
func (sd *SessionData) Clear() {
	sd.session.Data = make(map[string]interface{})
}

// Keys returns all keys in session data
func (sd *SessionData) Keys() []string {
	keys := make([]string, 0, len(sd.session.Data))
	for key := range sd.session.Data {
		keys = append(keys, key)
	}
	return keys
}