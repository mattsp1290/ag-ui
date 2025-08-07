package server

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Memory storage constants
const (
	DefaultMaxSessions = 10000
	DefaultShardCount  = 16
)

// MemorySessionStorage implements in-memory session storage
type MemorySessionStorage struct {
	config       *MemorySessionConfig
	logger       *zap.Logger
	sessions     map[string]*Session
	userSessions map[string][]string
	shards       []*SessionShard
	mu           sync.RWMutex
}

// SessionShard represents a shard for memory storage to improve concurrent access
type SessionShard struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewMemorySessionStorage creates a new memory session storage
func NewMemorySessionStorage(config *MemorySessionConfig, logger *zap.Logger) (*MemorySessionStorage, error) {
	if config == nil {
		config = &MemorySessionConfig{
			MaxSessions:    DefaultMaxSessions,
			EnableSharding: true,
			ShardCount:     DefaultShardCount,
		}
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// Validate configuration
	if err := validateMemoryConfig(config); err != nil {
		return nil, fmt.Errorf("invalid memory config: %w", err)
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

// validateMemoryConfig validates memory storage configuration
func validateMemoryConfig(config *MemorySessionConfig) error {
	if config.MaxSessions <= 0 {
		return fmt.Errorf("max sessions must be positive")
	}

	if config.EnableSharding {
		if config.ShardCount <= 0 {
			return fmt.Errorf("shard count must be positive when sharding is enabled")
		}

		if config.ShardCount > config.MaxSessions {
			return fmt.Errorf("shard count cannot exceed max sessions")
		}
	}

	return nil
}

// getShard returns the appropriate shard for a session ID
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

// getShardCapacity returns the maximum capacity for a single shard
func (m *MemorySessionStorage) getShardCapacity() int {
	if !m.config.EnableSharding || len(m.shards) == 0 {
		return m.config.MaxSessions
	}
	return m.config.MaxSessions / len(m.shards)
}

// CreateSession creates a new session in memory storage
func (m *MemorySessionStorage) CreateSession(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	if err := m.validateSession(session); err != nil {
		return fmt.Errorf("invalid session: %w", err)
	}

	if shard := m.getShard(session.ID); shard != nil {
		return m.createSessionInShard(shard, session)
	}

	return m.createSessionGlobal(session)
}

// createSessionInShard creates a session in a specific shard
func (m *MemorySessionStorage) createSessionInShard(shard *SessionShard, session *Session) error {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Check shard capacity
	if len(shard.sessions) >= m.getShardCapacity() {
		return fmt.Errorf("shard session limit reached (%d)", m.getShardCapacity())
	}

	// Check if session already exists
	if _, exists := shard.sessions[session.ID]; exists {
		return fmt.Errorf("session already exists")
	}

	// Create a copy to avoid external modifications
	sessionCopy := m.copySession(session)
	shard.sessions[session.ID] = sessionCopy

	return nil
}

// createSessionGlobal creates a session in global storage (no sharding)
func (m *MemorySessionStorage) createSessionGlobal(session *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check global capacity
	if len(m.sessions) >= m.config.MaxSessions {
		return fmt.Errorf("session limit reached (%d)", m.config.MaxSessions)
	}

	// Check if session already exists
	if _, exists := m.sessions[session.ID]; exists {
		return fmt.Errorf("session already exists")
	}

	// Create a copy to avoid external modifications
	sessionCopy := m.copySession(session)
	m.sessions[session.ID] = sessionCopy

	// Update user sessions index
	m.userSessions[session.UserID] = append(m.userSessions[session.UserID], session.ID)

	return nil
}

// GetSession retrieves a session by ID
func (m *MemorySessionStorage) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	if shard := m.getShard(sessionID); shard != nil {
		return m.getSessionFromShard(shard, sessionID)
	}

	return m.getSessionGlobal(sessionID)
}

// getSessionFromShard retrieves a session from a specific shard
func (m *MemorySessionStorage) getSessionFromShard(shard *SessionShard, sessionID string) (*Session, error) {
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	session, exists := shard.sessions[sessionID]
	if !exists {
		return nil, nil // Session not found
	}

	// Return a copy to prevent external modification
	sessionCopy := m.copySession(session)
	return sessionCopy, nil
}

// getSessionGlobal retrieves a session from global storage
func (m *MemorySessionStorage) getSessionGlobal(sessionID string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, nil // Session not found
	}

	// Return a copy to prevent external modification
	sessionCopy := m.copySession(session)
	return sessionCopy, nil
}

// UpdateSession updates an existing session
func (m *MemorySessionStorage) UpdateSession(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	if err := m.validateSession(session); err != nil {
		return fmt.Errorf("invalid session: %w", err)
	}

	if shard := m.getShard(session.ID); shard != nil {
		return m.updateSessionInShard(shard, session)
	}

	return m.updateSessionGlobal(session)
}

// updateSessionInShard updates a session in a specific shard
func (m *MemorySessionStorage) updateSessionInShard(shard *SessionShard, session *Session) error {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if _, exists := shard.sessions[session.ID]; !exists {
		return fmt.Errorf("session not found")
	}

	// Update with a copy to avoid external modifications
	sessionCopy := m.copySession(session)
	shard.sessions[session.ID] = sessionCopy

	return nil
}

// updateSessionGlobal updates a session in global storage
func (m *MemorySessionStorage) updateSessionGlobal(session *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[session.ID]; !exists {
		return fmt.Errorf("session not found")
	}

	// Update with a copy to avoid external modifications
	sessionCopy := m.copySession(session)
	m.sessions[session.ID] = sessionCopy

	return nil
}

// DeleteSession deletes a session
func (m *MemorySessionStorage) DeleteSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	if shard := m.getShard(sessionID); shard != nil {
		return m.deleteSessionFromShard(shard, sessionID)
	}

	return m.deleteSessionGlobal(sessionID)
}

// deleteSessionFromShard deletes a session from a specific shard
func (m *MemorySessionStorage) deleteSessionFromShard(shard *SessionShard, sessionID string) error {
	shard.mu.Lock()
	defer shard.mu.Unlock()

	delete(shard.sessions, sessionID)
	return nil
}

// deleteSessionGlobal deletes a session from global storage
func (m *MemorySessionStorage) deleteSessionGlobal(sessionID string) error {
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

			// Clean up empty user session entries
			if len(m.userSessions[session.UserID]) == 0 {
				delete(m.userSessions, session.UserID)
			}
		}
	}

	return nil
}

// GetUserSessions retrieves all sessions for a user
func (m *MemorySessionStorage) GetUserSessions(ctx context.Context, userID string) ([]*Session, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	if m.config.EnableSharding {
		return m.getUserSessionsFromShards(userID)
	}

	return m.getUserSessionsGlobal(userID)
}

// getUserSessionsFromShards retrieves user sessions from all shards
func (m *MemorySessionStorage) getUserSessionsFromShards(userID string) ([]*Session, error) {
	var sessions []*Session

	for _, shard := range m.shards {
		shard.mu.RLock()
		for _, session := range shard.sessions {
			if session.UserID == userID {
				sessionCopy := m.copySession(session)
				sessions = append(sessions, sessionCopy)
			}
		}
		shard.mu.RUnlock()
	}

	return sessions, nil
}

// getUserSessionsGlobal retrieves user sessions from global storage
func (m *MemorySessionStorage) getUserSessionsGlobal(userID string) ([]*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessionIDs, exists := m.userSessions[userID]
	if !exists {
		return []*Session{}, nil
	}

	sessions := make([]*Session, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		if session, exists := m.sessions[sessionID]; exists {
			sessionCopy := m.copySession(session)
			sessions = append(sessions, sessionCopy)
		}
	}

	return sessions, nil
}

// DeleteUserSessions deletes all sessions for a user
func (m *MemorySessionStorage) DeleteUserSessions(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}

	if m.config.EnableSharding {
		return m.deleteUserSessionsFromShards(userID)
	}

	return m.deleteUserSessionsGlobal(userID)
}

// deleteUserSessionsFromShards deletes user sessions from all shards
func (m *MemorySessionStorage) deleteUserSessionsFromShards(userID string) error {
	for _, shard := range m.shards {
		shard.mu.Lock()
		var toDelete []string
		for sessionID, session := range shard.sessions {
			if session.UserID == userID {
				toDelete = append(toDelete, sessionID)
			}
		}
		for _, sessionID := range toDelete {
			delete(shard.sessions, sessionID)
		}
		shard.mu.Unlock()
	}

	return nil
}

// deleteUserSessionsGlobal deletes user sessions from global storage
func (m *MemorySessionStorage) deleteUserSessionsGlobal(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionIDs, exists := m.userSessions[userID]
	if !exists {
		return nil
	}

	for _, sessionID := range sessionIDs {
		delete(m.sessions, sessionID)
	}

	delete(m.userSessions, userID)
	return nil
}

// CleanupExpiredSessions removes expired and inactive sessions
func (m *MemorySessionStorage) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	now := time.Now()
	var cleaned int64

	if m.config.EnableSharding {
		cleaned = m.cleanupExpiredSessionsFromShards(now)
	} else {
		cleaned = m.cleanupExpiredSessionsGlobal(now)
	}

	if cleaned > 0 {
		m.logger.Debug("Cleaned up expired sessions",
			zap.Int64("count", cleaned))
	}

	return cleaned, nil
}

// cleanupExpiredSessionsFromShards removes expired sessions from all shards
func (m *MemorySessionStorage) cleanupExpiredSessionsFromShards(now time.Time) int64 {
	var cleaned int64

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

	return cleaned
}

// cleanupExpiredSessionsGlobal removes expired sessions from global storage
func (m *MemorySessionStorage) cleanupExpiredSessionsGlobal(now time.Time) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	var toDelete []string
	for sessionID, session := range m.sessions {
		if now.After(session.ExpiresAt) || !session.IsActive {
			toDelete = append(toDelete, sessionID)
		}
	}

	cleaned := int64(len(toDelete))

	// Delete sessions and update user sessions index
	for _, sessionID := range toDelete {
		if session := m.sessions[sessionID]; session != nil {
			delete(m.sessions, sessionID)

			// Update user sessions index
			if userSessions, ok := m.userSessions[session.UserID]; ok {
				for i, id := range userSessions {
					if id == sessionID {
						m.userSessions[session.UserID] = append(userSessions[:i], userSessions[i+1:]...)
						break
					}
				}

				// Clean up empty user session entries
				if len(m.userSessions[session.UserID]) == 0 {
					delete(m.userSessions, session.UserID)
				}
			}
		}
	}

	return cleaned
}

// GetActiveSessions retrieves active sessions up to the limit
func (m *MemorySessionStorage) GetActiveSessions(ctx context.Context, limit int) ([]*Session, error) {
	if limit <= 0 {
		return []*Session{}, nil
	}

	var sessions []*Session
	now := time.Now()

	if m.config.EnableSharding {
		sessions = m.getActiveSessionsFromShards(now, limit)
	} else {
		sessions = m.getActiveSessionsGlobal(now, limit)
	}

	return sessions, nil
}

// getActiveSessionsFromShards retrieves active sessions from all shards
func (m *MemorySessionStorage) getActiveSessionsFromShards(now time.Time, limit int) []*Session {
	var sessions []*Session

	for _, shard := range m.shards {
		shard.mu.RLock()
		for _, session := range shard.sessions {
			if session.IsActive && now.Before(session.ExpiresAt) {
				sessionCopy := m.copySession(session)
				sessions = append(sessions, sessionCopy)
				if len(sessions) >= limit {
					shard.mu.RUnlock()
					return sessions
				}
			}
		}
		shard.mu.RUnlock()

		if len(sessions) >= limit {
			break
		}
	}

	return sessions
}

// getActiveSessionsGlobal retrieves active sessions from global storage
func (m *MemorySessionStorage) getActiveSessionsGlobal(now time.Time, limit int) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []*Session
	for _, session := range m.sessions {
		if session.IsActive && now.Before(session.ExpiresAt) {
			sessionCopy := m.copySession(session)
			sessions = append(sessions, sessionCopy)
			if len(sessions) >= limit {
				break
			}
		}
	}

	return sessions
}

// CountSessions returns the total number of sessions
func (m *MemorySessionStorage) CountSessions(ctx context.Context) (int64, error) {
	if m.config.EnableSharding {
		return m.countSessionsFromShards(), nil
	}

	return m.countSessionsGlobal(), nil
}

// countSessionsFromShards counts sessions from all shards
func (m *MemorySessionStorage) countSessionsFromShards() int64 {
	var count int64
	for _, shard := range m.shards {
		shard.mu.RLock()
		count += int64(len(shard.sessions))
		shard.mu.RUnlock()
	}
	return count
}

// countSessionsGlobal counts sessions from global storage
func (m *MemorySessionStorage) countSessionsGlobal() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.sessions))
}

// Close closes the memory session storage and clears all data
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

// Ping checks if the memory storage is available (always true for memory)
func (m *MemorySessionStorage) Ping(ctx context.Context) error {
	return nil // Memory storage is always available
}

// Stats returns statistics about the memory storage
func (m *MemorySessionStorage) Stats() map[string]interface{} {
	count, _ := m.CountSessions(context.Background())

	stats := map[string]interface{}{
		"type":             "memory",
		"session_count":    count,
		"max_sessions":     m.config.MaxSessions,
		"sharding_enabled": m.config.EnableSharding,
		"shard_count":      m.config.ShardCount,
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

// Helper methods

// validateSession validates session data
func (m *MemorySessionStorage) validateSession(session *Session) error {
	if session.ID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	if session.UserID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}

	if session.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("session is expired")
	}

	return nil
}

// copySession creates a deep copy of a session to prevent external modifications
func (m *MemorySessionStorage) copySession(session *Session) *Session {
	if session == nil {
		return nil
	}

	// Create new maps for data and metadata
	data := make(map[string]interface{})
	for k, v := range session.Data {
		data[k] = v
	}

	metadata := make(map[string]interface{})
	for k, v := range session.Metadata {
		metadata[k] = v
	}

	return &Session{
		ID:           session.ID,
		UserID:       session.UserID,
		CreatedAt:    session.CreatedAt,
		LastAccessed: session.LastAccessed,
		ExpiresAt:    session.ExpiresAt,
		IPAddress:    session.IPAddress,
		UserAgent:    session.UserAgent,
		IsActive:     session.IsActive,
		Data:         data,
		Metadata:     metadata,
	}
}
