package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Redis storage constants
const (
	DefaultRedisPoolSize    = 10
	DefaultRedisMaxRetries  = 3
	DefaultRedisKeyPrefix   = "session:"
	RedisUserIndexPrefix    = "user_sessions:"
	RedisActiveSetKey       = "active_sessions"
	RedisCleanupLockKey     = "cleanup_lock"
	RedisCleanupLockTTL     = 30 * time.Second
)

// RedisClient interface to abstract Redis operations
// This allows for different Redis client implementations
type RedisClient interface {
	// Basic key-value operations
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error

	// Set operations for user session indexes
	SAdd(ctx context.Context, key string, members ...interface{}) error
	SRem(ctx context.Context, key string, members ...interface{}) error
	SMembers(ctx context.Context, key string) ([]string, error)
	SCard(ctx context.Context, key string) (int64, error)

	// Scan operations for cleanup
	Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error)

	// Lock operations for distributed cleanup
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)

	// Connection management
	Close() error
	Ping(ctx context.Context) error
}

// RedisSessionStorage implements Redis-based session storage with plain credentials
type RedisSessionStorage struct {
	config *RedisSessionConfig
	logger *zap.Logger
	client RedisClient // Injected Redis client
}

// SecureRedisSessionStorage implements Redis-based session storage with secure credential handling
type SecureRedisSessionStorage struct {
	config *SecureRedisSessionConfig
	logger *zap.Logger
	client RedisClient // Injected Redis client
}

// NewRedisSessionStorage creates a new Redis session storage
func NewRedisSessionStorage(config *RedisSessionConfig, logger *zap.Logger) (*RedisSessionStorage, error) {
	if config == nil {
		return nil, fmt.Errorf("redis config cannot be nil")
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// Validate configuration
	if err := validateRedisConfig(config); err != nil {
		return nil, fmt.Errorf("invalid redis config: %w", err)
	}

	storage := &RedisSessionStorage{
		config: config,
		logger: logger,
		// client will be injected or created by the caller
	}

	logger.Info("Redis session storage initialized",
		zap.String("address", config.Address),
		zap.String("key_prefix", config.KeyPrefix),
		zap.Int("pool_size", config.PoolSize))

	return storage, nil
}

// NewSecureRedisSessionStorage creates a new secure Redis session storage
func NewSecureRedisSessionStorage(config *SecureRedisSessionConfig, logger *zap.Logger) (*SecureRedisSessionStorage, error) {
	if config == nil {
		return nil, fmt.Errorf("redis config cannot be nil")
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// Load credentials before creating storage
	if err := config.LoadCredentials(logger); err != nil {
		return nil, fmt.Errorf("failed to load Redis credentials: %w", err)
	}

	// Validate configuration
	if err := validateSecureRedisConfig(config); err != nil {
		return nil, fmt.Errorf("invalid secure redis config: %w", err)
	}

	storage := &SecureRedisSessionStorage{
		config: config,
		logger: logger,
		// client will be injected or created by the caller
	}

	logger.Info("Secure Redis session storage initialized",
		zap.String("address", config.Address),
		zap.String("key_prefix", config.KeyPrefix),
		zap.Bool("password_loaded", config.GetPassword() != nil))

	return storage, nil
}

// SetClient injects a Redis client for the regular Redis storage
func (r *RedisSessionStorage) SetClient(client RedisClient) {
	r.client = client
}

// SetClient injects a Redis client for the secure Redis storage
func (r *SecureRedisSessionStorage) SetClient(client RedisClient) {
	r.client = client
}

// Validation functions

func validateRedisConfig(config *RedisSessionConfig) error {
	if config.Address == "" {
		return fmt.Errorf("redis address is required")
	}

	if config.PoolSize <= 0 {
		config.PoolSize = DefaultRedisPoolSize
	}

	if config.MaxRetries < 0 {
		config.MaxRetries = DefaultRedisMaxRetries
	}

	if config.KeyPrefix == "" {
		config.KeyPrefix = DefaultRedisKeyPrefix
	}

	return nil
}

func validateSecureRedisConfig(config *SecureRedisSessionConfig) error {
	if config.Address == "" {
		return fmt.Errorf("redis address is required")
	}

	if config.PoolSize <= 0 {
		config.PoolSize = DefaultRedisPoolSize
	}

	if config.MaxRetries < 0 {
		config.MaxRetries = DefaultRedisMaxRetries
	}

	if config.KeyPrefix == "" {
		config.KeyPrefix = DefaultRedisKeyPrefix
	}

	if config.PasswordEnv == "" {
		return fmt.Errorf("password environment variable is required for secure configuration")
	}

	return nil
}

// RedisSessionStorage implementation

// CreateSession creates a new session in Redis
func (r *RedisSessionStorage) CreateSession(ctx context.Context, session *Session) error {
	if r.client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	// Serialize session to JSON
	sessionData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to serialize session: %w", err)
	}

	// Create session key
	sessionKey := r.config.KeyPrefix + session.ID
	
	// Calculate TTL based on expiration time
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("session is already expired")
	}

	// Store session in Redis
	if err := r.client.Set(ctx, sessionKey, sessionData, ttl); err != nil {
		return fmt.Errorf("failed to store session in redis: %w", err)
	}

	// Add session to user index
	userKey := RedisUserIndexPrefix + session.UserID
	if err := r.client.SAdd(ctx, userKey, session.ID); err != nil {
		r.logger.Warn("Failed to update user session index",
			zap.String("user_id", session.UserID),
			zap.Error(err))
		// Don't fail the operation for index update failures
	}

	// Add to active sessions set
	if err := r.client.SAdd(ctx, RedisActiveSetKey, session.ID); err != nil {
		r.logger.Warn("Failed to update active sessions set", zap.Error(err))
	}

	return nil
}

// GetSession retrieves a session from Redis
func (r *RedisSessionStorage) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if r.client == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}

	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	// Get session data from Redis
	sessionKey := r.config.KeyPrefix + sessionID
	sessionData, err := r.client.Get(ctx, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get session from redis: %w", err)
	}

	if sessionData == "" {
		return nil, nil // Session not found
	}

	// Deserialize session from JSON
	var session Session
	if err := json.Unmarshal([]byte(sessionData), &session); err != nil {
		return nil, fmt.Errorf("failed to deserialize session: %w", err)
	}

	// Check if session is expired (additional safety check)
	if time.Now().After(session.ExpiresAt) {
		// Clean up expired session
		go func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = r.DeleteSession(cleanupCtx, sessionID)
		}()
		return nil, nil
	}

	return &session, nil
}

// UpdateSession updates an existing session in Redis
func (r *RedisSessionStorage) UpdateSession(ctx context.Context, session *Session) error {
	if r.client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	// Check if session exists
	sessionKey := r.config.KeyPrefix + session.ID
	exists, err := r.client.Exists(ctx, sessionKey)
	if err != nil {
		return fmt.Errorf("failed to check session existence: %w", err)
	}

	if exists == 0 {
		return fmt.Errorf("session not found")
	}

	// Serialize updated session
	sessionData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to serialize session: %w", err)
	}

	// Calculate new TTL
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		// Session is expired, delete it
		return r.DeleteSession(ctx, session.ID)
	}

	// Update session in Redis
	if err := r.client.Set(ctx, sessionKey, sessionData, ttl); err != nil {
		return fmt.Errorf("failed to update session in redis: %w", err)
	}

	return nil
}

// DeleteSession deletes a session from Redis
func (r *RedisSessionStorage) DeleteSession(ctx context.Context, sessionID string) error {
	if r.client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	// Get session first to get user ID for index cleanup
	session, err := r.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session for deletion: %w", err)
	}

	// Delete session key
	sessionKey := r.config.KeyPrefix + sessionID
	if err := r.client.Del(ctx, sessionKey); err != nil {
		return fmt.Errorf("failed to delete session from redis: %w", err)
	}

	// Clean up user index if we have the session data
	if session != nil {
		userKey := RedisUserIndexPrefix + session.UserID
		if err := r.client.SRem(ctx, userKey, sessionID); err != nil {
			r.logger.Warn("Failed to update user session index",
				zap.String("user_id", session.UserID),
				zap.Error(err))
		}
	}

	// Remove from active sessions set
	if err := r.client.SRem(ctx, RedisActiveSetKey, sessionID); err != nil {
		r.logger.Warn("Failed to update active sessions set", zap.Error(err))
	}

	return nil
}

// GetUserSessions retrieves all sessions for a user
func (r *RedisSessionStorage) GetUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	if r.client == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}

	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	// Get session IDs for user
	userKey := RedisUserIndexPrefix + userID
	sessionIDs, err := r.client.SMembers(ctx, userKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions from redis: %w", err)
	}

	// Retrieve each session
	var sessions []*Session
	for _, sessionID := range sessionIDs {
		session, err := r.GetSession(ctx, sessionID)
		if err != nil {
			r.logger.Warn("Failed to get user session",
				zap.String("session_id", sessionID),
				zap.String("user_id", userID),
				zap.Error(err))
			continue
		}
		if session != nil {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// DeleteUserSessions deletes all sessions for a user
func (r *RedisSessionStorage) DeleteUserSessions(ctx context.Context, userID string) error {
	if r.client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}

	// Get user sessions
	sessions, err := r.GetUserSessions(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user sessions: %w", err)
	}

	// Delete each session
	for _, session := range sessions {
		if err := r.DeleteSession(ctx, session.ID); err != nil {
			r.logger.Error("Failed to delete user session",
				zap.String("session_id", session.ID),
				zap.String("user_id", userID),
				zap.Error(err))
		}
	}

	// Clean up user index
	userKey := RedisUserIndexPrefix + userID
	if err := r.client.Del(ctx, userKey); err != nil {
		r.logger.Warn("Failed to delete user session index", zap.Error(err))
	}

	return nil
}

// CleanupExpiredSessions removes expired sessions from Redis
func (r *RedisSessionStorage) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	if r.client == nil {
		return 0, fmt.Errorf("redis client not initialized")
	}

	// Use distributed lock to prevent multiple cleanup operations
	lockAcquired, err := r.client.SetNX(ctx, RedisCleanupLockKey, "cleanup", RedisCleanupLockTTL)
	if err != nil {
		return 0, fmt.Errorf("failed to acquire cleanup lock: %w", err)
	}

	if !lockAcquired {
		r.logger.Debug("Cleanup already in progress by another instance")
		return 0, nil
	}

	defer func() {
		// Release lock
		_ = r.client.Del(ctx, RedisCleanupLockKey)
	}()

	var cleaned int64
	var cursor uint64
	
	// Scan for session keys
	pattern := r.config.KeyPrefix + "*"
	
	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, pattern, 100)
		if err != nil {
			return cleaned, fmt.Errorf("failed to scan redis keys: %w", err)
		}

		// Check each key
		for _, key := range keys {
			sessionID := key[len(r.config.KeyPrefix):]
			session, err := r.GetSession(ctx, sessionID)
			if err != nil {
				continue
			}

			// GetSession already handles expired sessions by deleting them
			if session == nil {
				cleaned++
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return cleaned, nil
}

// GetActiveSessions retrieves active sessions up to the limit
func (r *RedisSessionStorage) GetActiveSessions(ctx context.Context, limit int) ([]*Session, error) {
	if r.client == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}

	if limit <= 0 {
		return []*Session{}, nil
	}

	// Get session IDs from active set
	sessionIDs, err := r.client.SMembers(ctx, RedisActiveSetKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}

	var sessions []*Session
	for _, sessionID := range sessionIDs {
		if len(sessions) >= limit {
			break
		}

		session, err := r.GetSession(ctx, sessionID)
		if err != nil {
			continue
		}

		if session != nil && session.IsActive && time.Now().Before(session.ExpiresAt) {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// CountSessions returns the total number of sessions
func (r *RedisSessionStorage) CountSessions(ctx context.Context) (int64, error) {
	if r.client == nil {
		return 0, fmt.Errorf("redis client not initialized")
	}

	return r.client.SCard(ctx, RedisActiveSetKey)
}

// Close closes the Redis connection
func (r *RedisSessionStorage) Close() error {
	r.logger.Info("Closing Redis session storage")
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// Ping checks Redis connectivity
func (r *RedisSessionStorage) Ping(ctx context.Context) error {
	if r.client == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return r.client.Ping(ctx)
}

// Stats returns Redis storage statistics
func (r *RedisSessionStorage) Stats() map[string]interface{} {
	stats := map[string]interface{}{
		"type":        "redis",
		"address":     r.config.Address,
		"key_prefix":  r.config.KeyPrefix,
		"pool_size":   r.config.PoolSize,
		"max_retries": r.config.MaxRetries,
		"tls_enabled": r.config.EnableTLS,
	}

	if r.client != nil {
		count, err := r.CountSessions(context.Background())
		if err == nil {
			stats["session_count"] = count
		}
		
		if err := r.client.Ping(context.Background()); err != nil {
			stats["status"] = "disconnected"
			stats["error"] = err.Error()
		} else {
			stats["status"] = "connected"
		}
	} else {
		stats["status"] = "client_not_initialized"
	}

	return stats
}

// SecureRedisSessionStorage implementation (delegates to regular methods with secure client)

func (r *SecureRedisSessionStorage) CreateSession(ctx context.Context, session *Session) error {
	if r.client == nil {
		return fmt.Errorf("redis client not initialized")
	}
	// Implementation is the same as regular Redis storage
	return r.createSession(ctx, session)
}

func (r *SecureRedisSessionStorage) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if r.client == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}
	return r.getSession(ctx, sessionID)
}

func (r *SecureRedisSessionStorage) UpdateSession(ctx context.Context, session *Session) error {
	if r.client == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return r.updateSession(ctx, session)
}

func (r *SecureRedisSessionStorage) DeleteSession(ctx context.Context, sessionID string) error {
	if r.client == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return r.deleteSession(ctx, sessionID)
}

func (r *SecureRedisSessionStorage) GetUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	if r.client == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}
	return r.getUserSessions(ctx, userID)
}

func (r *SecureRedisSessionStorage) DeleteUserSessions(ctx context.Context, userID string) error {
	if r.client == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return r.deleteUserSessions(ctx, userID)
}

func (r *SecureRedisSessionStorage) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	if r.client == nil {
		return 0, fmt.Errorf("redis client not initialized")
	}
	return r.cleanupExpiredSessions(ctx)
}

func (r *SecureRedisSessionStorage) GetActiveSessions(ctx context.Context, limit int) ([]*Session, error) {
	if r.client == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}
	return r.getActiveSessions(ctx, limit)
}

func (r *SecureRedisSessionStorage) CountSessions(ctx context.Context) (int64, error) {
	if r.client == nil {
		return 0, fmt.Errorf("redis client not initialized")
	}
	return r.client.SCard(ctx, RedisActiveSetKey)
}

func (r *SecureRedisSessionStorage) Close() error {
	r.logger.Info("Closing secure Redis session storage")
	if r.client != nil {
		if err := r.client.Close(); err != nil {
			return err
		}
	}
	
	// Cleanup credentials
	if r.config != nil {
		r.config.Cleanup()
	}
	
	return nil
}

func (r *SecureRedisSessionStorage) Ping(ctx context.Context) error {
	if r.client == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return r.client.Ping(ctx)
}

func (r *SecureRedisSessionStorage) Stats() map[string]interface{} {
	stats := map[string]interface{}{
		"type":             "secure_redis",
		"address":          r.config.Address,
		"key_prefix":       r.config.KeyPrefix,
		"pool_size":        r.config.PoolSize,
		"max_retries":      r.config.MaxRetries,
		"tls_enabled":      r.config.EnableTLS,
		"password_loaded":  r.config.GetPassword() != nil,
	}

	if r.client != nil {
		count, err := r.CountSessions(context.Background())
		if err == nil {
			stats["session_count"] = count
		}
		
		if err := r.client.Ping(context.Background()); err != nil {
			stats["status"] = "disconnected"
			stats["error"] = err.Error()
		} else {
			stats["status"] = "connected"
		}
	} else {
		stats["status"] = "client_not_initialized"
	}

	return stats
}

// Helper methods for SecureRedisSessionStorage (same implementation as regular Redis)

func (r *SecureRedisSessionStorage) createSession(ctx context.Context, session *Session) error {
	// Implementation identical to RedisSessionStorage.CreateSession
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	sessionData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to serialize session: %w", err)
	}

	sessionKey := r.config.KeyPrefix + session.ID
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("session is already expired")
	}

	if err := r.client.Set(ctx, sessionKey, sessionData, ttl); err != nil {
		return fmt.Errorf("failed to store session in redis: %w", err)
	}

	userKey := RedisUserIndexPrefix + session.UserID
	if err := r.client.SAdd(ctx, userKey, session.ID); err != nil {
		r.logger.Warn("Failed to update user session index", zap.Error(err))
	}

	if err := r.client.SAdd(ctx, RedisActiveSetKey, session.ID); err != nil {
		r.logger.Warn("Failed to update active sessions set", zap.Error(err))
	}

	return nil
}

func (r *SecureRedisSessionStorage) getSession(ctx context.Context, sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	sessionKey := r.config.KeyPrefix + sessionID
	sessionData, err := r.client.Get(ctx, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get session from redis: %w", err)
	}

	if sessionData == "" {
		return nil, nil
	}

	var session Session
	if err := json.Unmarshal([]byte(sessionData), &session); err != nil {
		return nil, fmt.Errorf("failed to deserialize session: %w", err)
	}

	if time.Now().After(session.ExpiresAt) {
		go func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = r.DeleteSession(cleanupCtx, sessionID)
		}()
		return nil, nil
	}

	return &session, nil
}

func (r *SecureRedisSessionStorage) updateSession(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	sessionKey := r.config.KeyPrefix + session.ID
	exists, err := r.client.Exists(ctx, sessionKey)
	if err != nil {
		return fmt.Errorf("failed to check session existence: %w", err)
	}

	if exists == 0 {
		return fmt.Errorf("session not found")
	}

	sessionData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to serialize session: %w", err)
	}

	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return r.DeleteSession(ctx, session.ID)
	}

	return r.client.Set(ctx, sessionKey, sessionData, ttl)
}

func (r *SecureRedisSessionStorage) deleteSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	session, err := r.getSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session for deletion: %w", err)
	}

	sessionKey := r.config.KeyPrefix + sessionID
	if err := r.client.Del(ctx, sessionKey); err != nil {
		return fmt.Errorf("failed to delete session from redis: %w", err)
	}

	if session != nil {
		userKey := RedisUserIndexPrefix + session.UserID
		if err := r.client.SRem(ctx, userKey, sessionID); err != nil {
			r.logger.Warn("Failed to update user session index", zap.Error(err))
		}
	}

	if err := r.client.SRem(ctx, RedisActiveSetKey, sessionID); err != nil {
		r.logger.Warn("Failed to update active sessions set", zap.Error(err))
	}

	return nil
}

func (r *SecureRedisSessionStorage) getUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	userKey := RedisUserIndexPrefix + userID
	sessionIDs, err := r.client.SMembers(ctx, userKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions from redis: %w", err)
	}

	var sessions []*Session
	for _, sessionID := range sessionIDs {
		session, err := r.getSession(ctx, sessionID)
		if err != nil {
			r.logger.Warn("Failed to get user session", zap.Error(err))
			continue
		}
		if session != nil {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

func (r *SecureRedisSessionStorage) deleteUserSessions(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}

	sessions, err := r.getUserSessions(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user sessions: %w", err)
	}

	for _, session := range sessions {
		if err := r.deleteSession(ctx, session.ID); err != nil {
			r.logger.Error("Failed to delete user session", zap.Error(err))
		}
	}

	userKey := RedisUserIndexPrefix + userID
	if err := r.client.Del(ctx, userKey); err != nil {
		r.logger.Warn("Failed to delete user session index", zap.Error(err))
	}

	return nil
}

func (r *SecureRedisSessionStorage) cleanupExpiredSessions(ctx context.Context) (int64, error) {
	lockAcquired, err := r.client.SetNX(ctx, RedisCleanupLockKey, "cleanup", RedisCleanupLockTTL)
	if err != nil {
		return 0, fmt.Errorf("failed to acquire cleanup lock: %w", err)
	}

	if !lockAcquired {
		r.logger.Debug("Cleanup already in progress by another instance")
		return 0, nil
	}

	defer func() {
		_ = r.client.Del(ctx, RedisCleanupLockKey)
	}()

	var cleaned int64
	var cursor uint64
	pattern := r.config.KeyPrefix + "*"

	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, pattern, 100)
		if err != nil {
			return cleaned, fmt.Errorf("failed to scan redis keys: %w", err)
		}

		for _, key := range keys {
			sessionID := key[len(r.config.KeyPrefix):]
			session, err := r.getSession(ctx, sessionID)
			if err != nil {
				continue
			}

			if session == nil {
				cleaned++
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return cleaned, nil
}

func (r *SecureRedisSessionStorage) getActiveSessions(ctx context.Context, limit int) ([]*Session, error) {
	if limit <= 0 {
		return []*Session{}, nil
	}

	sessionIDs, err := r.client.SMembers(ctx, RedisActiveSetKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}

	var sessions []*Session
	for _, sessionID := range sessionIDs {
		if len(sessions) >= limit {
			break
		}

		session, err := r.getSession(ctx, sessionID)
		if err != nil {
			continue
		}

		if session != nil && session.IsActive && time.Now().Before(session.ExpiresAt) {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}