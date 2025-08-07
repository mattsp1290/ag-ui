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

// MemoryStats tracks memory usage statistics for leak prevention
type MemoryStats struct {
	mu                     sync.RWMutex
	totalDeletions         int64     // Total deletions since startup
	globalDeletionCount    int64     // Global deletions since last recreation
	lastGlobalRecreation   time.Time // Last recreation of global maps
	recreationCount        int64     // Total number of recreations
}

// MemorySessionStorage implements in-memory session storage
type MemorySessionStorage struct {
	config       *MemorySessionConfig
	logger       *zap.Logger
	sessions     map[string]*Session
	userSessions map[string][]string
	shards       []*SessionShard
	mu           sync.RWMutex
	
	// Memory management tracking
	memoryStats   *MemoryStats
	lastRecreation time.Time
}

// SessionShard represents a shard for memory storage to improve concurrent access
type SessionShard struct {
	sessions       map[string]*Session
	mu             sync.RWMutex
	deletionCount  int64     // Track number of deletions since last recreation
	lastRecreation time.Time // Track when the map was last recreated
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
		config:         config,
		logger:         logger,
		sessions:       make(map[string]*Session),
		userSessions:   make(map[string][]string),
		memoryStats:    &MemoryStats{lastGlobalRecreation: time.Now()},
		lastRecreation: time.Now(),
	}

	// Initialize shards if sharding is enabled
	if config.EnableSharding {
		storage.shards = make([]*SessionShard, config.ShardCount)
		for i := 0; i < config.ShardCount; i++ {
			storage.shards[i] = &SessionShard{
				sessions:       make(map[string]*Session),
				lastRecreation: time.Now(),
			}
		}
	}

	logger.Info("Memory session storage initialized",
		zap.Int("max_sessions", config.MaxSessions),
		zap.Bool("sharding_enabled", config.EnableSharding),
		zap.Int("shard_count", config.ShardCount),
		zap.Bool("map_recreation_enabled", config.EnableMapRecreation),
		zap.Int("recreation_deletion_threshold", config.RecreationDeletionThreshold),
		zap.Duration("recreation_time_threshold", config.RecreationTimeThreshold),
		zap.Float64("max_map_capacity_ratio", config.MaxMapCapacityRatio))

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
	
	// Check if session exists before deletion
	_, existed := shard.sessions[sessionID]
	delete(shard.sessions, sessionID)
	
	// Track deletion for memory management
	if existed {
		shard.deletionCount++
		m.trackDeletion()
		
		// Check if we need to recreate the map while holding the lock
		shouldRecreate := m.shouldRecreateShardMap(shard)
		if shouldRecreate {
			m.recreateShardMap(shard)
		}
	}
	
	shard.mu.Unlock()
	return nil
}

// deleteSessionGlobal deletes a session from global storage
func (m *MemorySessionStorage) deleteSessionGlobal(sessionID string) error {
	m.mu.Lock()
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

		// Track deletion for memory management
		m.memoryStats.mu.Lock()
		m.memoryStats.totalDeletions++
		m.memoryStats.globalDeletionCount++
		m.memoryStats.mu.Unlock()
	}
	
	// Check if we need to recreate global maps (release lock first)
	shouldRecreate := false
	if exists {
		m.mu.Unlock() // Release the main lock before checking recreation
		shouldRecreate = m.shouldRecreateGlobalMaps()
		if shouldRecreate {
			m.mu.Lock() // Acquire lock again for recreation
			m.recreateGlobalMaps()
			m.mu.Unlock()
			return nil
		}
	} else {
		m.mu.Unlock()
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
	var totalDeleted int64
	
	for _, shard := range m.shards {
		shard.mu.Lock()
		var toDelete []string
		for sessionID, session := range shard.sessions {
			if session.UserID == userID {
				toDelete = append(toDelete, sessionID)
			}
		}
		
		deletedCount := len(toDelete)
		for _, sessionID := range toDelete {
			delete(shard.sessions, sessionID)
		}
		
		// Track deletions for memory management
		if deletedCount > 0 {
			shard.deletionCount += int64(deletedCount)
			totalDeleted += int64(deletedCount)
			
			// Check if we need to recreate the shard map
			shouldRecreate := m.shouldRecreateShardMap(shard)
			if shouldRecreate {
				m.recreateShardMap(shard)
			}
		}
		
		shard.mu.Unlock()
	}

	// Update global deletion stats
	if totalDeleted > 0 {
		m.memoryStats.mu.Lock()
		m.memoryStats.totalDeletions += totalDeleted
		m.memoryStats.mu.Unlock()
	}

	return nil
}

// deleteUserSessionsGlobal deletes user sessions from global storage
func (m *MemorySessionStorage) deleteUserSessionsGlobal(userID string) error {
	m.mu.Lock()

	sessionIDs, exists := m.userSessions[userID]
	if !exists {
		m.mu.Unlock()
		return nil
	}

	deletedCount := len(sessionIDs)
	for _, sessionID := range sessionIDs {
		delete(m.sessions, sessionID)
	}

	delete(m.userSessions, userID)

	// Track deletions for memory management
	if deletedCount > 0 {
		m.memoryStats.mu.Lock()
		m.memoryStats.totalDeletions += int64(deletedCount)
		m.memoryStats.globalDeletionCount += int64(deletedCount)
		m.memoryStats.mu.Unlock()
	}

	// Release the main lock before checking recreation
	m.mu.Unlock()
	
	// Check if we need to recreate global maps
	if deletedCount > 0 {
		shouldRecreate := m.shouldRecreateGlobalMaps()
		if shouldRecreate {
			m.mu.Lock()
			m.recreateGlobalMaps()
			m.mu.Unlock()
		}
	}

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

		deletedCount := len(toDelete)
		for _, sessionID := range toDelete {
			delete(shard.sessions, sessionID)
		}
		
		// Track deletions for memory management
		if deletedCount > 0 {
			shard.deletionCount += int64(deletedCount)
			cleaned += int64(deletedCount)
			
			// Check if we need to recreate the shard map
			shouldRecreate := m.shouldRecreateShardMap(shard)
			if shouldRecreate {
				reclaimedEntries := m.recreateShardMap(shard)
				m.logger.Debug("Map recreation during cleanup",
					zap.Int("shard_deleted_sessions", deletedCount),
					zap.Int64("reclaimed_entries", reclaimedEntries))
			}
		}
		
		shard.mu.Unlock()
	}

	// Update global deletion stats
	if cleaned > 0 {
		m.memoryStats.mu.Lock()
		m.memoryStats.totalDeletions += cleaned
		m.memoryStats.mu.Unlock()
	}

	return cleaned
}

// cleanupExpiredSessionsGlobal removes expired sessions from global storage
func (m *MemorySessionStorage) cleanupExpiredSessionsGlobal(now time.Time) int64 {
	m.mu.Lock()

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

	// Track deletions for memory management
	if cleaned > 0 {
		m.memoryStats.mu.Lock()
		m.memoryStats.totalDeletions += cleaned
		m.memoryStats.globalDeletionCount += cleaned
		m.memoryStats.mu.Unlock()
	}

	// Release the main lock before checking recreation
	m.mu.Unlock()
	
	// Check if we need to recreate global maps
	if cleaned > 0 {
		shouldRecreate := m.shouldRecreateGlobalMaps()
		if shouldRecreate {
			m.mu.Lock()
			reclaimedEntries := m.recreateGlobalMaps()
			m.mu.Unlock()
			
			m.logger.Debug("Map recreation during global cleanup",
				zap.Int64("deleted_sessions", cleaned),
				zap.Int64("reclaimed_entries", reclaimedEntries))
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
			shard.deletionCount = 0
			shard.lastRecreation = time.Now()
			shard.mu.Unlock()
		}
	} else {
		m.mu.Lock()
		m.sessions = make(map[string]*Session)
		m.userSessions = make(map[string][]string)
		m.mu.Unlock()
	}

	// Reset memory stats
	m.memoryStats.mu.Lock()
	m.memoryStats.totalDeletions = 0
	m.memoryStats.globalDeletionCount = 0
	m.memoryStats.lastGlobalRecreation = time.Now()
	m.memoryStats.recreationCount = 0
	m.memoryStats.mu.Unlock()

	return nil
}

// Ping checks if the memory storage is available (always true for memory)
func (m *MemorySessionStorage) Ping(ctx context.Context) error {
	return nil // Memory storage is always available
}

// Stats returns statistics about the memory storage
func (m *MemorySessionStorage) Stats() map[string]interface{} {
	count, _ := m.CountSessions(context.Background())
	memoryStats := m.getMemoryStats()

	stats := map[string]interface{}{
		"type":             "memory",
		"session_count":    count,
		"max_sessions":     m.config.MaxSessions,
		"sharding_enabled": m.config.EnableSharding,
		"shard_count":      m.config.ShardCount,
		"memory_management": memoryStats,
	}

	if m.config.EnableSharding {
		shardCounts := make([]int, len(m.shards))
		shardDeletions := make([]int64, len(m.shards))
		shardLastRecreations := make([]string, len(m.shards))
		
		for i, shard := range m.shards {
			shard.mu.RLock()
			shardCounts[i] = len(shard.sessions)
			shardDeletions[i] = shard.deletionCount
			shardLastRecreations[i] = shard.lastRecreation.Format(time.RFC3339)
			shard.mu.RUnlock()
		}
		
		stats["shard_counts"] = shardCounts
		stats["shard_deletions"] = shardDeletions
		stats["shard_last_recreations"] = shardLastRecreations
	}

	return stats
}

// getMemoryStats returns current memory management statistics
func (m *MemorySessionStorage) getMemoryStats() map[string]interface{} {
	m.memoryStats.mu.RLock()
	defer m.memoryStats.mu.RUnlock()

	stats := map[string]interface{}{
		"enabled":                    m.config.EnableMapRecreation,
		"total_deletions":           m.memoryStats.totalDeletions,
		"global_deletion_count":     m.memoryStats.globalDeletionCount,
		"recreation_count":          m.memoryStats.recreationCount,
		"last_global_recreation":    m.memoryStats.lastGlobalRecreation.Format(time.RFC3339),
		"deletion_threshold":        m.config.RecreationDeletionThreshold,
		"time_threshold_minutes":    int(m.config.RecreationTimeThreshold.Minutes()),
		"max_capacity_ratio":        m.config.MaxMapCapacityRatio,
	}

	if m.config.EnableSharding {
		var totalShardDeletions int64
		for _, shard := range m.shards {
			shard.mu.RLock()
			totalShardDeletions += shard.deletionCount
			shard.mu.RUnlock()
		}
		stats["total_shard_deletions"] = totalShardDeletions
	}

	return stats
}

// Map recreation methods for memory leak prevention

// shouldRecreateShardMap checks if a shard map should be recreated to prevent memory leaks
func (m *MemorySessionStorage) shouldRecreateShardMap(shard *SessionShard) bool {
	if !m.config.EnableMapRecreation {
		return false
	}

	// Check deletion threshold
	if shard.deletionCount >= int64(m.config.RecreationDeletionThreshold) {
		return true
	}

	// Check time threshold
	if time.Since(shard.lastRecreation) >= m.config.RecreationTimeThreshold {
		return true
	}

	// Check capacity ratio threshold
	currentCapacity := len(shard.sessions)
	if currentCapacity > 0 {
		estimatedWastedSpace := float64(shard.deletionCount) / float64(currentCapacity+int(shard.deletionCount))
		if estimatedWastedSpace >= (m.config.MaxMapCapacityRatio - 1.0) / m.config.MaxMapCapacityRatio {
			return true
		}
	}

	return false
}

// recreateShardMap creates a new map for the shard and copies active sessions
func (m *MemorySessionStorage) recreateShardMap(shard *SessionShard) int64 {
	now := time.Now()
	newMap := make(map[string]*Session)
	var copiedSessions int64

	// Copy only active, non-expired sessions to the new map
	for sessionID, session := range shard.sessions {
		if session.IsActive && now.Before(session.ExpiresAt) {
			newMap[sessionID] = session
			copiedSessions++
		}
	}

	// Replace the old map
	oldSessionCount := len(shard.sessions)
	shard.sessions = newMap
	shard.deletionCount = 0
	shard.lastRecreation = now

	m.logger.Info("Recreated shard map to prevent memory leak",
		zap.Int("old_sessions", oldSessionCount),
		zap.Int64("copied_sessions", copiedSessions),
		zap.Int64("reclaimed_entries", int64(oldSessionCount)-copiedSessions))

	return int64(oldSessionCount) - copiedSessions
}

// shouldRecreateGlobalMaps checks if global maps should be recreated
func (m *MemorySessionStorage) shouldRecreateGlobalMaps() bool {
	if !m.config.EnableMapRecreation {
		return false
	}

	m.memoryStats.mu.RLock()
	defer m.memoryStats.mu.RUnlock()

	// Check deletion threshold
	if m.memoryStats.globalDeletionCount >= int64(m.config.RecreationDeletionThreshold) {
		return true
	}

	// Check time threshold
	if time.Since(m.memoryStats.lastGlobalRecreation) >= m.config.RecreationTimeThreshold {
		return true
	}

	// Check capacity ratio threshold
	currentCapacity := len(m.sessions)
	if currentCapacity > 0 {
		estimatedWastedSpace := float64(m.memoryStats.globalDeletionCount) / float64(currentCapacity+int(m.memoryStats.globalDeletionCount))
		if estimatedWastedSpace >= (m.config.MaxMapCapacityRatio - 1.0) / m.config.MaxMapCapacityRatio {
			return true
		}
	}

	return false
}

// recreateGlobalMaps creates new global maps and copies active sessions
func (m *MemorySessionStorage) recreateGlobalMaps() int64 {
	now := time.Now()
	newSessionMap := make(map[string]*Session)
	newUserSessionMap := make(map[string][]string)
	var copiedSessions int64

	// Copy only active, non-expired sessions to the new maps
	for sessionID, session := range m.sessions {
		if session.IsActive && now.Before(session.ExpiresAt) {
			newSessionMap[sessionID] = session
			newUserSessionMap[session.UserID] = append(newUserSessionMap[session.UserID], sessionID)
			copiedSessions++
		}
	}

	// Replace the old maps
	oldSessionCount := len(m.sessions)
	m.sessions = newSessionMap
	m.userSessions = newUserSessionMap

	// Update memory stats
	m.memoryStats.mu.Lock()
	m.memoryStats.globalDeletionCount = 0
	m.memoryStats.lastGlobalRecreation = now
	m.memoryStats.recreationCount++
	m.memoryStats.mu.Unlock()

	m.logger.Info("Recreated global maps to prevent memory leak",
		zap.Int("old_sessions", oldSessionCount),
		zap.Int64("copied_sessions", copiedSessions),
		zap.Int64("reclaimed_entries", int64(oldSessionCount)-copiedSessions),
		zap.Int64("total_recreations", m.memoryStats.recreationCount))

	return int64(oldSessionCount) - copiedSessions
}

// trackDeletion increments deletion counters for memory management
func (m *MemorySessionStorage) trackDeletion() {
	m.memoryStats.mu.Lock()
	m.memoryStats.totalDeletions++
	m.memoryStats.mu.Unlock()
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
