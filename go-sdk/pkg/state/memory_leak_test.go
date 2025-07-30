package state

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContextManagerBoundedSize verifies that ContextManager respects size limits
func TestContextManagerBoundedSize(t *testing.T) {
	maxSize := 10
	cm := NewContextManager(maxSize)

	// Add more contexts than the max size
	for i := 0; i < maxSize*2; i++ {
		ctx := &StateContext{
			ID:       fmt.Sprintf("context-%d", i),
			StateID:  fmt.Sprintf("state-%d", i),
			Created:  time.Now(),
			Metadata: map[string]interface{}{"index": i},
		}
		ctx.SetLastAccessed(time.Now())
		cm.Put(ctx.ID, ctx)
	}

	// Verify size is bounded
	assert.Equal(t, maxSize, cm.Size(), "ContextManager should not exceed max size")

	// Verify oldest contexts were evicted (LRU)
	for i := 0; i < maxSize; i++ {
		_, exists := cm.Get(fmt.Sprintf("context-%d", i))
		assert.False(t, exists, "Oldest contexts should have been evicted")
	}

	// Verify newest contexts are still present
	for i := maxSize; i < maxSize*2; i++ {
		_, exists := cm.Get(fmt.Sprintf("context-%d", i))
		assert.True(t, exists, "Newest contexts should still be present")
	}
}

// TestContextManagerConcurrentAccess verifies thread safety
func TestContextManagerConcurrentAccess(t *testing.T) {
	cm := NewContextManager(100)
	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	// Concurrent puts
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				ctx := &StateContext{
					ID:      fmt.Sprintf("ctx-%d-%d", id, j),
					StateID: "state",
					Created: time.Now(),
				}
				ctx.SetLastAccessed(time.Now())
				cm.Put(ctx.ID, ctx)
			}
		}(i)
	}

	// Concurrent gets
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				cm.Get(fmt.Sprintf("ctx-%d-%d", id, j))
			}
		}(i)
	}

	// Concurrent ranges
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				count := 0
				cm.Range(func(key, value interface{}) bool {
					count++
					return count < 50 // Limit iteration
				})
			}
		}()
	}

	wg.Wait()

	// Verify no data corruption
	assert.LessOrEqual(t, cm.Size(), 100, "Size should not exceed max")
}

// TestContextManagerExpiredCleanup verifies expired context cleanup
func TestContextManagerExpiredCleanup(t *testing.T) {
	cm := NewContextManager(100)

	// Add contexts with different access times
	now := time.Now()
	for i := 0; i < 10; i++ {
		ctx := &StateContext{
			ID:      fmt.Sprintf("ctx-%d", i),
			StateID: "state",
			Created: now.Add(-time.Hour),
		}
		ctx.SetLastAccessed(now.Add(-time.Duration(i) * time.Minute))
		cm.Put(ctx.ID, ctx)
	}

	// Get expired contexts (older than 5 minutes)
	expired := cm.GetExpiredContexts(5 * time.Minute)
	assert.GreaterOrEqual(t, len(expired), 5, "Should have at least 5 expired contexts")

	// Cleanup expired
	cleaned := cm.CleanupExpired(5 * time.Minute)
	assert.GreaterOrEqual(t, cleaned, 5, "Should have cleaned at least 5 contexts")

	// Verify they're gone
	for i := 5; i < 10; i++ {
		_, exists := cm.Get(fmt.Sprintf("ctx-%d", i))
		assert.False(t, exists, "Expired contexts should be removed")
	}
}

// TestBoundedResolverRegistrySize verifies resolver registry size limits
func TestBoundedResolverRegistrySize(t *testing.T) {
	maxSize := 10
	br := NewBoundedResolverRegistry(maxSize)

	// Add more resolvers than max size
	for i := 0; i < maxSize*2; i++ {
		name := fmt.Sprintf("resolver-%d", i)
		resolver := func(conflict *StateConflict) (*ConflictResolution, error) {
			return nil, nil
		}
		err := br.Register(name, resolver)
		require.NoError(t, err)
	}

	// Verify size is bounded
	assert.Equal(t, maxSize, br.Size(), "Registry should not exceed max size")

	// Verify oldest resolvers were evicted
	for i := 0; i < maxSize; i++ {
		_, exists := br.Get(fmt.Sprintf("resolver-%d", i))
		assert.False(t, exists, "Oldest resolvers should have been evicted")
	}

	// Verify newest resolvers are still present
	for i := maxSize; i < maxSize*2; i++ {
		_, exists := br.Get(fmt.Sprintf("resolver-%d", i))
		assert.True(t, exists, "Newest resolvers should still be present")
	}
}

// TestBoundedResolverRegistryConcurrentAccess verifies thread safety
func TestBoundedResolverRegistryConcurrentAccess(t *testing.T) {
	br := NewBoundedResolverRegistry(50)
	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				name := fmt.Sprintf("resolver-%d-%d", id, j)
				resolver := func(conflict *StateConflict) (*ConflictResolution, error) {
					return nil, nil
				}
				br.Register(name, resolver)
			}
		}(i)
	}

	// Concurrent gets
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				br.Get(fmt.Sprintf("resolver-%d-%d", id, j))
			}
		}(i)
	}

	wg.Wait()

	// Verify no data corruption
	assert.LessOrEqual(t, br.Size(), 50, "Size should not exceed max")

	// Verify statistics are consistent
	stats := br.GetStatistics()
	assert.Equal(t, br.Size(), stats["current_size"])
}

// TestStateManagerMemoryLeakPrevention tests that StateManager doesn't leak memory
func TestStateManagerMemoryLeakPrevention(t *testing.T) {
	// Skip in short mode as this test can be slow
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	opts := DefaultManagerOptions()
	opts.CacheSize = 100 // Small cache to test eviction
	opts.EnableMetrics = false // Disable metrics to reduce noise

	sm, err := NewStateManager(opts)
	require.NoError(t, err)
	defer sm.Close()

	ctx := context.Background()

	// Track initial memory
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Create many contexts to test memory bounds
	for i := 0; i < 1000; i++ {
		contextID, err := sm.CreateContext(ctx, fmt.Sprintf("state-%d", i), map[string]interface{}{
			"index": i,
			"data":  fmt.Sprintf("some data for context %d", i),
		})
		require.NoError(t, err)

		// Simulate some work
		_, err = sm.GetState(ctx, contextID, fmt.Sprintf("state-%d", i))
		assert.NoError(t, err)
	}

	// Force cleanup
	time.Sleep(100 * time.Millisecond)
	runtime.GC()

	// Check final memory
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	// Memory growth should be bounded despite creating 1000 contexts
	memGrowth := m2.Alloc - m1.Alloc
	maxExpectedGrowth := uint64(10 * 1024 * 1024) // 10MB max growth

	assert.Less(t, memGrowth, maxExpectedGrowth,
		"Memory growth (%d bytes) exceeded expected maximum (%d bytes)",
		memGrowth, maxExpectedGrowth)

	// Verify context count is bounded
	contextCount := 0
	sm.activeContexts.Range(func(key, value interface{}) bool {
		contextCount++
		return true
	})

	assert.LessOrEqual(t, contextCount, opts.CacheSize,
		"Active contexts (%d) should not exceed cache size (%d)",
		contextCount, opts.CacheSize)
}

// TestConflictResolverMemoryLeakPrevention tests resolver registry bounds
func TestConflictResolverMemoryLeakPrevention(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}
	
	cr := NewConflictResolver(LastWriteWins)

	// Register many custom resolvers
	for i := 0; i < 1000; i++ {
		name := fmt.Sprintf("custom-resolver-%d", i)
		resolver := func(conflict *StateConflict) (*ConflictResolution, error) {
			return &ConflictResolution{
				ID:       fmt.Sprintf("resolution-%d", i),
				Strategy: CustomStrategy,
			}, nil
		}
		err := cr.RegisterCustomResolver(name, resolver)
		assert.NoError(t, err, "Failed to register custom resolver %s", name)
	}

	// Verify resolver count is bounded
	resolverCount := cr.customResolvers.Size()
	assert.LessOrEqual(t, resolverCount, 100,
		"Custom resolvers (%d) should not exceed max size (100)",
		resolverCount)

	// Verify we can still register new resolvers (old ones evicted)
	err := cr.RegisterCustomResolver("final-resolver", func(conflict *StateConflict) (*ConflictResolution, error) {
		return nil, nil
	})
	assert.NoError(t, err, "Failed to register final resolver")

	_, exists := cr.customResolvers.Get("final-resolver")
	assert.True(t, exists, "Should be able to register new resolver after eviction")
}

// BenchmarkContextManagerOperations benchmarks context manager performance
func BenchmarkContextManagerOperations(b *testing.B) {
	cm := NewContextManager(1000)

	// Pre-populate with some contexts
	for i := 0; i < 500; i++ {
		ctx := &StateContext{
			ID:      fmt.Sprintf("ctx-%d", i),
			StateID: "state",
			Created: time.Now(),
		}
		ctx.SetLastAccessed(time.Now())
		cm.Put(ctx.ID, ctx)
	}

	b.Run("Put", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ctx := &StateContext{
				ID:      fmt.Sprintf("bench-ctx-%d", i),
				StateID: "state",
				Created: time.Now(),
			}
			ctx.SetLastAccessed(time.Now())
			cm.Put(ctx.ID, ctx)
		}
	})

	b.Run("Get", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cm.Get(fmt.Sprintf("ctx-%d", i%500))
		}
	})

	b.Run("Range", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			count := 0
			cm.Range(func(key, value interface{}) bool {
				count++
				return count < 100
			})
		}
	})
}
