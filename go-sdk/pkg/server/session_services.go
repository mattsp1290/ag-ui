package server

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Background service constants
const (
	CleanupTimeoutSeconds     = 30
	SessionLockTimeoutSeconds = 30
	MinTimingAttackDelay      = 10 * time.Millisecond
	MinValidationTime         = 50 * time.Millisecond
	MetricsUpdateInterval     = 30 * time.Second
)

// startBackgroundServices starts the background cleanup and maintenance services
func (sm *SessionManager) startBackgroundServices() {
	// Start cleanup service
	sm.startCleanupService()

	// Start metrics update service
	sm.startMetricsService()

	sm.logger.Debug("Background services started",
		zap.Duration("cleanup_interval", sm.config.CleanupInterval),
		zap.Duration("metrics_interval", MetricsUpdateInterval))
}

// startCleanupService starts the periodic cleanup of expired sessions
func (sm *SessionManager) startCleanupService() {
	if sm.config.CleanupInterval <= 0 {
		sm.logger.Warn("Session cleanup disabled: cleanup interval is zero or negative")
		return
	}

	sm.cleanupTicker = time.NewTicker(sm.config.CleanupInterval)

	sm.wg.Add(1)
	go func() {
		defer sm.wg.Done()
		defer sm.cleanupTicker.Stop()

		for {
			select {
			case <-sm.cleanupTicker.C:
				sm.performScheduledCleanup()
			case <-sm.stopCleanup:
				sm.logger.Debug("Cleanup service stopping")
				return
			}
		}
	}()
}

// startMetricsService starts the periodic metrics update service
func (sm *SessionManager) startMetricsService() {
	sm.wg.Add(1)
	go func() {
		defer sm.wg.Done()

		ticker := time.NewTicker(MetricsUpdateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				sm.updateSessionMetrics()
			case <-sm.stopCleanup:
				sm.logger.Debug("Metrics service stopping")
				return
			}
		}
	}()
}

// performScheduledCleanup performs a scheduled cleanup of expired sessions
func (sm *SessionManager) performScheduledCleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	startTime := time.Now()
	cleaned, err := sm.CleanupExpiredSessions(ctx)

	if err != nil {
		sm.logger.Error("Scheduled cleanup failed", zap.Error(err))
		return
	}

	duration := time.Since(startTime)

	if cleaned > 0 {
		sm.logger.Info("Scheduled cleanup completed",
			zap.Int64("cleaned_sessions", cleaned),
			zap.Duration("duration", duration))
	} else {
		sm.logger.Debug("Scheduled cleanup completed with no expired sessions",
			zap.Duration("duration", duration))
	}
}

// CleanupExpiredSessions removes expired sessions and updates metrics
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
	sm.updateSessionMetrics()

	duration := time.Since(startTime)
	if cleaned > 0 {
		sm.logger.Info("Session cleanup completed",
			zap.Int64("cleaned", cleaned),
			zap.Duration("duration", duration))
	}

	return cleaned, nil
}

// GetMetrics returns a copy of current session management metrics
func (sm *SessionManager) GetMetrics() *SessionMetrics {
	sm.metrics.mu.RLock()
	defer sm.metrics.mu.RUnlock()

	// Create a copy of metrics to prevent external modification
	metrics := &SessionMetrics{
		TotalSessions:      sm.metrics.TotalSessions,
		ActiveSessions:     atomic.LoadInt64(&sm.activeSessions),
		ExpiredSessions:    sm.metrics.ExpiredSessions,
		CleanupRuns:        sm.metrics.CleanupRuns,
		LastCleanup:        sm.metrics.LastCleanup,
		MemoryUsageBytes:   sm.metrics.MemoryUsageBytes,
		AverageSessionSize: sm.metrics.AverageSessionSize,
		SessionsPerSecond:  sm.metrics.SessionsPerSecond,
		LastMetricsUpdate:  sm.metrics.LastMetricsUpdate,
		StorageErrors:      sm.metrics.StorageErrors,
		ValidationErrors:   sm.metrics.ValidationErrors,
	}

	return metrics
}

// Stats returns comprehensive session manager statistics
func (sm *SessionManager) Stats() map[string]interface{} {
	metrics := sm.GetMetrics()
	storageStats := sm.storage.Stats()

	// Get cleanup statistics
	cleanupInProgress, pendingCleanups := sm.getCleanupStats()

	sm.sessionOpsMu.RLock()
	sessionLockCount := len(sm.sessionOps)
	sm.sessionOpsMu.RUnlock()

	stats := map[string]interface{}{
		"total_sessions":           metrics.TotalSessions,
		"active_sessions":          metrics.ActiveSessions,
		"expired_sessions":         metrics.ExpiredSessions,
		"cleanup_runs":             metrics.CleanupRuns,
		"last_cleanup":             metrics.LastCleanup,
		"memory_usage_bytes":       metrics.MemoryUsageBytes,
		"average_session_size":     metrics.AverageSessionSize,
		"sessions_per_second":      metrics.SessionsPerSecond,
		"storage_errors":           metrics.StorageErrors,
		"validation_errors":        metrics.ValidationErrors,
		"backend":                  sm.config.Backend,
		"ttl_seconds":              sm.config.TTL.Seconds(),
		"cleanup_interval_seconds": sm.config.CleanupInterval.Seconds(),

		// Cleanup and synchronization stats
		"cleanup_in_progress":  cleanupInProgress,
		"pending_cleanups":     pendingCleanups,
		"session_locks_active": sessionLockCount,
	}

	// Add storage-specific stats
	for k, v := range storageStats {
		stats["storage_"+k] = v
	}

	return stats
}

// shutdownBackgroundServices gracefully shuts down all background services
func (sm *SessionManager) shutdownBackgroundServices(ctx context.Context) error {
	// Check if already closed
	if sm.closed.Load() {
		return nil
	}

	sm.logger.Info("Shutting down session manager")

	// Mark as closed to prevent double-close of channels
	sm.closed.Store(true)

	// Stop background services
	close(sm.stopCleanup)
	if sm.cleanupTicker != nil {
		sm.cleanupTicker.Stop()
	}

	// Wait for background goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		sm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		sm.logger.Info("All background services stopped")
	case <-ctx.Done():
		sm.logger.Warn("Shutdown timeout reached, forcing shutdown")
	}

	// Wait for any in-progress cleanup operations to finish
	maxWait := time.Now().Add(CleanupTimeoutSeconds * time.Second)
	for time.Now().Before(maxWait) {
		cleanupInProgress, pendingCleanups := sm.getCleanupStats()
		if cleanupInProgress == 0 && pendingCleanups == 0 {
			break
		}
		sm.logger.Debug("Waiting for cleanup operations to complete",
			zap.Int64("cleanup_in_progress", cleanupInProgress),
			zap.Int("pending_cleanups", pendingCleanups))
		time.Sleep(100 * time.Millisecond)
	}

	// Clean up synchronization structures
	sm.cleanupMu.Lock()
	sm.pendingCleanups = make(map[string]int64)
	sm.cleanupMu.Unlock()

	sm.sessionOpsMu.Lock()
	sm.sessionOps = make(map[string]*sync.Mutex)
	sm.sessionOpsMu.Unlock()

	// Stop memory manager
	if sm.memManager != nil {
		sm.memManager.Stop()
	}

	// Close storage
	if err := sm.storage.Close(); err != nil {
		sm.logger.Error("Failed to close session storage", zap.Error(err))
		return fmt.Errorf("failed to close session storage: %w", err)
	}

	// Securely cleanup credentials
	if sm.credentialManager != nil {
		sm.credentialManager.Cleanup()
	}

	// Cleanup backend-specific credentials
	if sm.config != nil {
		if sm.config.Redis != nil {
			sm.config.Redis.Cleanup()
		}
		if sm.config.Database != nil {
			sm.config.Database.Cleanup()
		}
	}

	sm.logger.Info("Session manager shutdown complete with secure credential cleanup")
	return nil
}

// Session operation synchronization methods

// getSessionLock returns a mutex for the given session ID, creating one if needed
func (sm *SessionManager) getSessionLock(sessionID string) *sync.Mutex {
	sm.sessionOpsMu.RLock()
	if mu, exists := sm.sessionOps[sessionID]; exists {
		sm.sessionOpsMu.RUnlock()
		return mu
	}
	sm.sessionOpsMu.RUnlock()

	// Need to create the mutex
	sm.sessionOpsMu.Lock()
	defer sm.sessionOpsMu.Unlock()

	// Double-check after acquiring write lock
	if mu, exists := sm.sessionOps[sessionID]; exists {
		return mu
	}

	mu := &sync.Mutex{}
	sm.sessionOps[sessionID] = mu
	return mu
}

// releaseSessionLock removes the mutex for a session if it's no longer needed
func (sm *SessionManager) releaseSessionLock(sessionID string) {
	sm.sessionOpsMu.Lock()
	defer sm.sessionOpsMu.Unlock()
	delete(sm.sessionOps, sessionID)
}

// atomicCleanupSession performs atomic session cleanup with race condition protection
func (sm *SessionManager) atomicCleanupSession(ctx context.Context, sessionID string) error {
	// Check if cleanup is already pending/in progress for this session
	sm.cleanupMu.Lock()
	if timestamp, exists := sm.pendingCleanups[sessionID]; exists {
		// Check if cleanup is stale (older than 30 seconds)
		if time.Now().Unix()-timestamp > SessionLockTimeoutSeconds {
			delete(sm.pendingCleanups, sessionID)
		} else {
			sm.cleanupMu.Unlock()
			return nil // Cleanup already in progress
		}
	}

	// Mark cleanup as in progress
	sm.pendingCleanups[sessionID] = time.Now().Unix()
	sm.cleanupMu.Unlock()

	// Ensure cleanup is marked complete when done
	defer func() {
		sm.cleanupMu.Lock()
		delete(sm.pendingCleanups, sessionID)
		sm.cleanupMu.Unlock()
		sm.cleanupSessionOperationLock(sessionID)
	}()

	// Get session-specific lock for atomic operation
	sessionMu := sm.getSessionOperationLock(sessionID)
	sessionMu.Lock()
	defer sessionMu.Unlock()

	// Increment cleanup counter
	atomic.AddInt64(&sm.cleanupInProgress, 1)
	defer atomic.AddInt64(&sm.cleanupInProgress, -1)

	// Perform the actual cleanup
	if err := sm.storage.DeleteSession(ctx, sessionID); err != nil {
		sm.logger.Warn("Failed to cleanup session",
			zap.String("session_id", sessionID),
			zap.Error(err))
		sm.recordStorageError()
		return err
	}

	// Update metrics atomically
	atomic.AddInt64(&sm.activeSessions, -1)
	sm.updateSessionMetrics()

	sm.logger.Debug("Session cleanup completed",
		zap.String("session_id", sessionID))

	return nil
}

// atomicUpdateSession performs atomic session update with synchronization
func (sm *SessionManager) atomicUpdateSession(ctx context.Context, session *Session) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	// Get session-specific lock for atomic operation
	sessionMu := sm.getSessionOperationLock(session.ID)
	sessionMu.Lock()
	defer sessionMu.Unlock()

	// Perform the update
	if err := sm.storage.UpdateSession(ctx, session); err != nil {
		sm.recordStorageError()
		return fmt.Errorf("failed to update session: %w", err)
	}

	return nil
}

// getCleanupStats returns current cleanup operation statistics
func (sm *SessionManager) getCleanupStats() (int64, int) {
	cleanupInProgress := atomic.LoadInt64(&sm.cleanupInProgress)

	sm.cleanupMu.Lock()
	pendingCount := len(sm.pendingCleanups)
	sm.cleanupMu.Unlock()

	return cleanupInProgress, pendingCount
}

// Metrics and error tracking methods

// performMetricsUpdate updates session metrics periodically
func (sm *SessionManager) performMetricsUpdate() {
	sm.updateSessionMetrics()
}

// recordStorageError increments the storage error counter
func (sm *SessionManager) recordStorageError() {
	atomic.AddInt64(&sm.metrics.StorageErrors, 1)
}

// recordValidationError increments the validation error counter
func (sm *SessionManager) recordValidationError() {
	atomic.AddInt64(&sm.metrics.ValidationErrors, 1)
}

// Security helper methods for timing attack protection

// Storage factory function

// createSessionStorage creates the appropriate session storage backend
func createSessionStorage(config *SessionConfig, logger *zap.Logger) (SessionStorage, error) {
	switch config.Backend {
	case "memory":
		return NewMemorySessionStorage(config.Memory, logger)
	case "redis":
		return NewSecureRedisSessionStorage(config.Redis, logger)
	case "database":
		return NewSecureDatabaseSessionStorage(config.Database, logger)
	default:
		return nil, fmt.Errorf("unsupported session backend: %s", config.Backend)
	}
}

// Health check methods

// Ping checks if the session manager and its storage are healthy
func (sm *SessionManager) Ping(ctx context.Context) error {
	if sm.storage == nil {
		return fmt.Errorf("session storage not initialized")
	}

	return sm.storage.Ping(ctx)
}

// IsHealthy returns whether the session manager is in a healthy state
func (sm *SessionManager) IsHealthy(ctx context.Context) bool {
	// Check basic functionality
	if sm.storage == nil || sm.config == nil {
		return false
	}

	// Check storage health
	if err := sm.Ping(ctx); err != nil {
		sm.logger.Warn("Storage health check failed", zap.Error(err))
		return false
	}

	// Check for excessive error rates
	metrics := sm.GetMetrics()
	if metrics.StorageErrors > 100 || metrics.ValidationErrors > 100 {
		sm.logger.Warn("High error rates detected",
			zap.Int64("storage_errors", metrics.StorageErrors),
			zap.Int64("validation_errors", metrics.ValidationErrors))
		return false
	}

	return true
}

// GetStorageStats returns storage-specific statistics
func (sm *SessionManager) GetStorageStats() map[string]interface{} {
	if sm.storage == nil {
		return map[string]interface{}{"error": "storage not initialized"}
	}

	return sm.storage.Stats()
}

// Session count methods

// GetActiveSessionCount returns the current number of active sessions
func (sm *SessionManager) GetActiveSessionCount() int64 {
	return atomic.LoadInt64(&sm.activeSessions)
}

// GetTotalSessionCount returns the total number of sessions created
func (sm *SessionManager) GetTotalSessionCount() int64 {
	return atomic.LoadInt64(&sm.totalSessions)
}

// Configuration methods

// GetConfig returns a copy of the current configuration
func (sm *SessionManager) GetConfig() *SessionConfig {
	// Return a copy to prevent external modification
	configCopy := *sm.config
	return &configCopy
}

// UpdateConfig updates the session manager configuration (limited fields)
func (sm *SessionManager) UpdateConfig(updates *SessionConfigUpdate) error {
	if updates == nil {
		return fmt.Errorf("config updates cannot be nil")
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Only allow updating certain safe fields
	if updates.CleanupInterval != nil && *updates.CleanupInterval > MinCleanupInterval {
		sm.config.CleanupInterval = *updates.CleanupInterval

		// Restart cleanup ticker with new interval
		if sm.cleanupTicker != nil {
			sm.cleanupTicker.Stop()
			sm.cleanupTicker = time.NewTicker(sm.config.CleanupInterval)
		}
	}

	if updates.MaxSessionsPerUser != nil && *updates.MaxSessionsPerUser >= 0 {
		sm.config.MaxSessionsPerUser = *updates.MaxSessionsPerUser
	}

	sm.logger.Info("Session manager configuration updated",
		zap.Duration("cleanup_interval", sm.config.CleanupInterval),
		zap.Int("max_sessions_per_user", sm.config.MaxSessionsPerUser))

	return nil
}

// SessionConfigUpdate represents allowed configuration updates
type SessionConfigUpdate struct {
	CleanupInterval    *time.Duration `json:"cleanup_interval,omitempty"`
	MaxSessionsPerUser *int           `json:"max_sessions_per_user,omitempty"`
}
