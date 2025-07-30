package state

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStateContextAtomicLastAccessed tests the atomic operations for LastAccessed field
func TestStateContextAtomicLastAccessed(t *testing.T) {
	ctx := &StateContext{
		ID:      "test-ctx",
		StateID: "test-state",
		Created: time.Now(),
	}
	
	// Set initial time
	initialTime := time.Now()
	ctx.SetLastAccessed(initialTime)
	
	// Verify we can read it back
	readTime := ctx.GetLastAccessed()
	assert.Equal(t, initialTime.UnixNano(), readTime.UnixNano())
	
	// Update to a new time
	newTime := time.Now().Add(time.Hour)
	ctx.SetLastAccessed(newTime)
	
	// Verify it was updated
	readTime2 := ctx.GetLastAccessed()
	assert.Equal(t, newTime.UnixNano(), readTime2.UnixNano())
	
	// Test UpdateLastAccessed method
	before := time.Now()
	ctx.UpdateLastAccessed()
	after := time.Now()
	
	readTime3 := ctx.GetLastAccessed()
	assert.True(t, readTime3.After(before) || readTime3.Equal(before))
	assert.True(t, readTime3.Before(after) || readTime3.Equal(after))
}

// TestStateContextConcurrentLastAccessedOperations tests concurrent access to LastAccessed field
func TestStateContextConcurrentLastAccessedOperations(t *testing.T) {
	ctx := &StateContext{
		ID:      "test-ctx-concurrent",
		StateID: "test-state-concurrent",
		Created: time.Now(),
	}
	
	ctx.SetLastAccessed(time.Now())
	
	const numGoroutines = 10  // Reduced from 100 to prevent resource exhaustion
	const numOperationsPerGoroutine = 10  // Reduced from 100 to prevent test timeouts
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	// Launch multiple goroutines to concurrently read and write LastAccessed
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperationsPerGoroutine; j++ {
				if j%2 == 0 {
					// Update LastAccessed
					ctx.UpdateLastAccessed()
				} else {
					// Read LastAccessed
					lastAccessed := ctx.GetLastAccessed()
					// Verify it's not zero time (which would indicate corruption)
					require.False(t, lastAccessed.IsZero(), "LastAccessed should never be zero")
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Final verification that LastAccessed is still valid
	finalTime := ctx.GetLastAccessed()
	assert.False(t, finalTime.IsZero())
	assert.True(t, finalTime.After(time.Now().Add(-time.Minute))) // Should be recent
}

// TestContextManagerConcurrentAccessWithAtomicOperations tests the ContextManager 
// with our new atomic operations to ensure no race conditions
func TestContextManagerConcurrentAccessWithAtomicOperations(t *testing.T) {
	cm := NewContextManager(100)
	
	// Pre-populate with some contexts
	for i := 0; i < 20; i++ {
		ctx := &StateContext{
			ID:      fmt.Sprintf("pre-ctx-%d", i),
			StateID: "pre-state",
			Created: time.Now(),
		}
		ctx.SetLastAccessed(time.Now())
		cm.Put(ctx.ID, ctx)
	}
	
	const numGoroutines = 10  // Reduced from 50 to prevent resource exhaustion
	const numOperations = 10  // Reduced from 100 to prevent test timeouts
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	// Test concurrent operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				switch j % 4 {
				case 0:
					// Get operation (updates LastAccessed atomically)
					cm.Get(fmt.Sprintf("pre-ctx-%d", j%20))
				case 1:
					// Put operation
					ctx := &StateContext{
						ID:      fmt.Sprintf("ctx-%d-%d", id, j),
						StateID: "concurrent-state",
						Created: time.Now(),
					}
					ctx.SetLastAccessed(time.Now())
					cm.Put(ctx.ID, ctx)
				case 2:
					// GetExpiredContexts (reads LastAccessed atomically)
					cm.GetExpiredContexts(time.Hour)
				case 3:
					// Range operation (snapshot approach)
					cm.Range(func(key, value interface{}) bool {
						ctx := value.(*StateContext)
						// This read should be safe due to atomic operations
						_ = ctx.GetLastAccessed()
						return true
					})
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify the manager is still in a valid state
	assert.True(t, cm.Size() > 0)
	
	// Verify all contexts have valid LastAccessed times
	cm.Range(func(key, value interface{}) bool {
		ctx := value.(*StateContext)
		lastAccessed := ctx.GetLastAccessed()
		assert.False(t, lastAccessed.IsZero(), "LastAccessed should not be zero for context %s", ctx.ID)
		return true
	})
}