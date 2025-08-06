package server

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestSessionRaceConditions(t *testing.T) {
	// Create a session manager with memory backend
	config := DefaultSessionConfig()
	config.Backend = "memory"
	config.TTL = time.Minute // Minimum valid TTL
	config.CleanupInterval = time.Minute

	logger := zap.NewNop()
	sm, err := NewSessionManager(config, logger)
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}
	defer sm.Close()

	// Test concurrent session cleanup operations
	t.Run("ConcurrentCleanup", func(t *testing.T) {
		ctx := context.Background()

		// Create a session that will expire quickly
		req := httptest.NewRequest("GET", "/", nil)
		session, err := sm.CreateSession(ctx, "user1", req)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Delete the session to create a missing session scenario
		err = sm.DeleteSession(ctx, session.ID)
		if err != nil {
			t.Fatalf("Failed to delete session: %v", err)
		}

		// Try to trigger multiple concurrent operations on missing session
		var wg sync.WaitGroup
		var successfulDetections int64

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				
				// This should detect missing session
				_, err := sm.GetSession(cleanupCtx, session.ID)
				if err != nil && strings.Contains(err.Error(), "not found") {
					atomic.AddInt64(&successfulDetections, 1)
				}
			}()
		}

		wg.Wait()

		// All requests should have detected the missing session
		if successfulDetections != 10 {
			t.Errorf("Expected 10 successful missing session detections, got %d", successfulDetections)
		}

		// Verify cleanup state is consistent
		cleanupInProgress, pendingCleanups := sm.getCleanupStats()
		if cleanupInProgress != 0 {
			t.Errorf("Expected no cleanup operations in progress, got %d", cleanupInProgress)
		}
		if pendingCleanups != 0 {
			t.Errorf("Expected no pending cleanups, got %d", pendingCleanups)
		}
	})

	// Test concurrent session updates
	t.Run("ConcurrentUpdates", func(t *testing.T) {
		ctx := context.Background()

		// Create a session
		req := httptest.NewRequest("GET", "/", nil)
		session, err := sm.CreateSession(ctx, "user2", req)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Perform concurrent validation operations that trigger updates
		var wg sync.WaitGroup
		var successfulValidations int64

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				// This should trigger an async update
				validateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				
				_, err := sm.ValidateSession(validateCtx, session.ID, req)
				if err == nil {
					atomic.AddInt64(&successfulValidations, 1)
				}
			}()
		}

		wg.Wait()

		// All validations should succeed
		if successfulValidations != 10 {
			t.Errorf("Expected 10 successful validations, got %d", successfulValidations)
		}

		// Give async updates and timing protection time to complete (50ms * 10 operations + buffer)
		time.Sleep(600 * time.Millisecond)

		// Verify session locks eventually get cleaned up (allow for timing variations)
		var sessionLockCount int
		for i := 0; i < 10; i++ {
			sm.sessionOpsMu.RLock()
			sessionLockCount = len(sm.sessionOps)
			sm.sessionOpsMu.RUnlock()
			
			if sessionLockCount <= 1 { // Allow for at most one lock (the session we're still working with)
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		if sessionLockCount > 2 { // Allow for a reasonable number of locks (2 sessions in this test)
			t.Errorf("Expected at most 2 session locks after wait period, got %d", sessionLockCount)
		}
	})

	// Test timing attack protection
	t.Run("TimingAttackProtection", func(t *testing.T) {
		ctx := context.Background()
		req := httptest.NewRequest("GET", "/", nil)

		// Create a session
		session, err := sm.CreateSession(ctx, "user3", req)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Test that operations take at least the minimum time
		startTime := time.Now()
		_, err = sm.ValidateSession(ctx, session.ID, req)
		elapsed := time.Since(startTime)

		if err != nil {
			t.Fatalf("Session validation failed: %v", err)
		}

		// Should take at least the minimum validation time (50ms)
		minTime := 50 * time.Millisecond
		if elapsed < minTime {
			t.Errorf("Validation completed too quickly: %v, expected at least %v", elapsed, minTime)
		}

		// Test with invalid session ID (should still take minimum time)
		startTime = time.Now()
		_, err = sm.ValidateSession(ctx, "invalid-session-id", req)
		elapsed = time.Since(startTime)

		if err == nil {
			t.Error("Expected validation to fail for invalid session ID")
		}

		// Should still take at least the minimum validation time
		if elapsed < minTime {
			t.Errorf("Invalid session validation completed too quickly: %v, expected at least %v", elapsed, minTime)
		}
	})
}

func TestConstantTimeComparison(t *testing.T) {
	config := DefaultSessionConfig()
	logger := zap.NewNop()
	sm, err := NewSessionManager(config, logger)
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}
	defer sm.Close()

	// Test equal strings
	if !sm.constantTimeCompare("hello", "hello") {
		t.Error("Expected equal strings to compare as true")
	}

	// Test unequal strings
	if sm.constantTimeCompare("hello", "world") {
		t.Error("Expected unequal strings to compare as false")
	}

	// Test different lengths
	if sm.constantTimeCompare("short", "much longer string") {
		t.Error("Expected strings of different lengths to compare as false")
	}

	// Test timing consistency by running many comparisons
	// This is a basic test - in practice you'd need more sophisticated timing analysis
	iterations := 1000
	
	// Time comparisons of equal-length strings
	start := time.Now()
	for i := 0; i < iterations; i++ {
		sm.constantTimeCompare("hello", "world")
	}
	equalLengthTime := time.Since(start)

	// Time comparisons of different-length strings
	start = time.Now()
	for i := 0; i < iterations; i++ {
		sm.constantTimeCompare("hello", "much longer string")
	}
	differentLengthTime := time.Since(start)

	// The timing should be relatively consistent
	// We allow for some variation but they shouldn't be dramatically different
	ratio := float64(differentLengthTime) / float64(equalLengthTime)
	if ratio < 0.5 || ratio > 2.0 {
		t.Logf("Warning: Timing ratio between different-length and equal-length comparisons: %.2f", ratio)
		t.Logf("Equal-length time: %v, Different-length time: %v", equalLengthTime, differentLengthTime)
		// This is just a warning, not a failure, as timing can vary on different systems
	}
}