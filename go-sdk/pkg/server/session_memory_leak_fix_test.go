package server

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TestMemoryLeakPrevention verifies that the map recreation logic works correctly
func TestMemoryLeakPrevention(t *testing.T) {
	// Create a config with aggressive recreation thresholds for testing
	config := &MemorySessionConfig{
		MaxSessions:                 1000,
		EnableSharding:              true,
		ShardCount:                  2,
		EnableMapRecreation:         true,
		RecreationDeletionThreshold: 5,   // Recreate after 5 deletions
		RecreationTimeThreshold:     1 * time.Second, // Recreate after 1 second
		MaxMapCapacityRatio:         2.0, // Recreate when wasted space > 50%
	}

	logger := zap.NewNop()
	storage, err := NewMemorySessionStorage(config, logger)
	if err != nil {
		t.Fatalf("Failed to create memory session storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create multiple sessions
	sessions := make([]*Session, 10)
	for i := 0; i < 10; i++ {
		sessions[i] = &Session{
			ID:           uuid.New().String(),
			UserID:       "test_user",
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			ExpiresAt:    time.Now().Add(1 * time.Hour),
			IsActive:     true,
			Data:         make(map[string]interface{}),
			Metadata:     make(map[string]interface{}),
		}
		
		err := storage.CreateSession(ctx, sessions[i])
		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
	}

	// Verify initial session count
	count, err := storage.CountSessions(ctx)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	if count != 10 {
		t.Errorf("Expected 10 sessions, got %d", count)
	}

	// Delete sessions to trigger recreation threshold
	for i := 0; i < 6; i++ {
		err := storage.DeleteSession(ctx, sessions[i].ID)
		if err != nil {
			t.Fatalf("Failed to delete session %d: %v", i, err)
		}
	}

	// Verify remaining sessions
	count, err = storage.CountSessions(ctx)
	if err != nil {
		t.Fatalf("Failed to count sessions after deletion: %v", err)
	}
	if count != 4 {
		t.Errorf("Expected 4 sessions after deletion, got %d", count)
	}

	// Check memory stats to see if recreation occurred
	stats := storage.getMemoryStats()
	t.Logf("Memory stats after deletions: %+v", stats)

	// Verify that recreation was enabled
	if !stats["enabled"].(bool) {
		t.Error("Memory recreation should be enabled")
	}

	// Verify that some deletions were tracked
	totalDeletions := stats["total_deletions"].(int64)
	if totalDeletions != 6 {
		t.Errorf("Expected 6 total deletions, got %d", totalDeletions)
	}
}

// TestMemoryLeakPreventionGlobalMaps tests map recreation for global (non-sharded) storage
func TestMemoryLeakPreventionGlobalMaps(t *testing.T) {
	// Create config with global storage and aggressive thresholds
	config := &MemorySessionConfig{
		MaxSessions:                 1000,
		EnableSharding:              false, // Use global storage
		EnableMapRecreation:         true,
		RecreationDeletionThreshold: 3,    // Recreate after 3 deletions
		RecreationTimeThreshold:     500 * time.Millisecond,
		MaxMapCapacityRatio:         1.5,  // Recreate when wasted space > 33%
	}

	logger := zap.NewNop()
	storage, err := NewMemorySessionStorage(config, logger)
	if err != nil {
		t.Fatalf("Failed to create memory session storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create sessions
	sessions := make([]*Session, 8)
	for i := 0; i < 8; i++ {
		sessions[i] = &Session{
			ID:           uuid.New().String(),
			UserID:       "test_user",
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			ExpiresAt:    time.Now().Add(1 * time.Hour),
			IsActive:     true,
			Data:         make(map[string]interface{}),
			Metadata:     make(map[string]interface{}),
		}
		
		err := storage.CreateSession(ctx, sessions[i])
		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
	}

	// Delete sessions to trigger recreation
	for i := 0; i < 4; i++ {
		err := storage.DeleteSession(ctx, sessions[i].ID)
		if err != nil {
			t.Fatalf("Failed to delete session %d: %v", i, err)
		}
	}

	// Check stats
	stats := storage.getMemoryStats()
	t.Logf("Global storage stats after deletions: %+v", stats)

	// Verify tracking
	totalDeletions := stats["total_deletions"].(int64)
	globalDeletions := stats["global_deletion_count"].(int64)
	
	if totalDeletions != 4 {
		t.Errorf("Expected 4 total deletions, got %d", totalDeletions)
	}

	// Since we deleted 4 and have a threshold of 3, recreation should have occurred
	// After recreation, global_deletion_count should be reset
	if globalDeletions > 3 {
		t.Errorf("Expected global deletion count to be reset after recreation, got %d", globalDeletions)
	}

	// Verify remaining sessions are still accessible
	count, err := storage.CountSessions(ctx)
	if err != nil {
		t.Fatalf("Failed to count sessions: %v", err)
	}
	if count != 4 {
		t.Errorf("Expected 4 sessions remaining, got %d", count)
	}
}

// TestMapRecreationWithExpiredSessions tests recreation during cleanup
func TestMapRecreationWithExpiredSessions(t *testing.T) {
	config := &MemorySessionConfig{
		MaxSessions:                 100,
		EnableSharding:              false,
		EnableMapRecreation:         true,
		RecreationDeletionThreshold: 2,
		RecreationTimeThreshold:     10 * time.Minute, // Long time threshold
		MaxMapCapacityRatio:         2.0,
	}

	logger := zap.NewNop()
	storage, err := NewMemorySessionStorage(config, logger)
	if err != nil {
		t.Fatalf("Failed to create memory session storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create expired sessions
	for i := 0; i < 5; i++ {
		session := &Session{
			ID:           uuid.New().String(),
			UserID:       "test_user",
			CreatedAt:    time.Now().Add(-2 * time.Hour),
			LastAccessed: time.Now().Add(-2 * time.Hour),
			ExpiresAt:    time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
			IsActive:     true,
			Data:         make(map[string]interface{}),
			Metadata:     make(map[string]interface{}),
		}
		
		// Bypass validation by creating session directly
		storage.mu.Lock()
		storage.sessions[session.ID] = session
		storage.userSessions[session.UserID] = append(storage.userSessions[session.UserID], session.ID)
		storage.mu.Unlock()
	}

	// Add one active session
	activeSession := &Session{
		ID:           uuid.New().String(),
		UserID:       "test_user",
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		IsActive:     true,
		Data:         make(map[string]interface{}),
		Metadata:     make(map[string]interface{}),
	}
	err = storage.CreateSession(ctx, activeSession)
	if err != nil {
		t.Fatalf("Failed to create active session: %v", err)
	}

	// Run cleanup - should delete expired sessions and potentially trigger recreation
	cleaned, err := storage.CleanupExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("Failed to cleanup expired sessions: %v", err)
	}
	
	t.Logf("Cleaned up %d expired sessions", cleaned)

	if cleaned != 5 {
		t.Errorf("Expected to clean 5 expired sessions, got %d", cleaned)
	}

	// Verify only active session remains
	count, err := storage.CountSessions(ctx)
	if err != nil {
		t.Fatalf("Failed to count sessions after cleanup: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 active session remaining, got %d", count)
	}

	// Check that the active session is still accessible
	retrieved, err := storage.GetSession(ctx, activeSession.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve active session: %v", err)
	}
	if retrieved == nil {
		t.Error("Active session should still be accessible after cleanup")
	}
}