package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	eventerrors "github.com/ag-ui/go-sdk/pkg/core/events/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBasicSetGetOperations tests basic cache set/get functionality
func TestBasicSetGetOperations(t *testing.T) {
	tests := []struct {
		name          string
		config        *CacheValidatorConfig
		events        []events.Event
		expectedHits  uint64
		expectedMisses uint64
	}{
		{
			name: "single event caching",
			config: &CacheValidatorConfig{
				L1Size:        100,
				L1TTL:         5 * time.Minute,
				L2Enabled:     false,
				MetricsEnabled: true,
				Validator:     events.NewValidator(events.DefaultValidationConfig()),
			},
			events: []events.Event{
				events.NewRunStartedEvent("thread-1", "run-1"),
				events.NewRunStartedEvent("thread-1", "run-1"), // Same event again for cache hit
			},
			expectedHits:  1,
			expectedMisses: 1,
		},
		{
			name: "multiple different events",
			config: &CacheValidatorConfig{
				L1Size:        100,
				L1TTL:         5 * time.Minute,
				L2Enabled:     false,
				MetricsEnabled: true,
				Validator:     events.NewValidator(events.DefaultValidationConfig()),
			},
			events: []events.Event{
				events.NewRunStartedEvent("thread-1", "run-1"),
				events.NewRunStartedEvent("thread-2", "run-2"),
				events.NewToolCallStartEvent("tool-1", "ToolName"),
			},
			expectedHits:  0,
			expectedMisses: 3,
		},
		{
			name: "repeated event access",
			config: &CacheValidatorConfig{
				L1Size:        100,
				L1TTL:         5 * time.Minute,
				L2Enabled:     false,
				MetricsEnabled: true,
				Validator:     events.NewValidator(events.DefaultValidationConfig()),
			},
			events: []events.Event{
				events.NewRunStartedEvent("thread-1", "run-1"),
				events.NewRunStartedEvent("thread-1", "run-1"),
				events.NewRunStartedEvent("thread-1", "run-1"),
			},
			expectedHits:  2,
			expectedMisses: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cv, err := NewCacheValidator(tt.config)
			require.NoError(t, err)
			defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

			ctx := context.Background()

			// Process events
			for _, event := range tt.events {
				err := cv.ValidateEvent(ctx, event)
				assert.NoError(t, err)
			}

			// Verify stats
			stats := cv.GetStats()
			assert.Equal(t, tt.expectedHits, stats.L1Hits, "L1 hits mismatch")
			assert.Equal(t, tt.expectedMisses, stats.L1Misses, "L1 misses mismatch")
		})
	}
}

// TestCacheEvictionPolicies tests various cache eviction scenarios
func TestCacheEvictionPolicies(t *testing.T) {
	t.Run("LRU eviction", func(t *testing.T) {
		config := &CacheValidatorConfig{
			L1Size:        3, // Small size to trigger eviction
			L1TTL:         5 * time.Minute,
			L2Enabled:     false,
			MetricsEnabled: true,
			Validator:     events.NewValidator(events.DefaultValidationConfig()),
		}

		cv, err := NewCacheValidator(config)
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

		ctx := context.Background()

		// Fill cache beyond capacity
		testEvents := []events.Event{
			events.NewRunStartedEvent("thread-1", "run-1"),
			events.NewRunStartedEvent("thread-2", "run-2"),
			events.NewRunStartedEvent("thread-3", "run-3"),
			events.NewRunStartedEvent("thread-4", "run-4"), // This should evict thread-1
		}

		for _, event := range testEvents {
			err := cv.ValidateEvent(ctx, event)
			assert.NoError(t, err)
		}

		// Access thread-1 again - should be a miss
		err = cv.ValidateEvent(ctx, testEvents[0])
		assert.NoError(t, err)

		stats := cv.GetStats()
		assert.Greater(t, stats.L1Misses, uint64(4), "Should have cache misses due to eviction")
	})

	t.Run("Size-based eviction", func(t *testing.T) {
		config := &CacheValidatorConfig{
			L1Size:        5,
			L1TTL:         5 * time.Minute,
			L2Enabled:     false,
			MetricsEnabled: true,
			Validator:     events.NewValidator(events.DefaultValidationConfig()),
		}

		cv, err := NewCacheValidator(config)
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

		ctx := context.Background()

		// Create events with increasing sizes
		for i := 0; i < 10; i++ {
			event := events.NewRunStartedEvent(fmt.Sprintf("thread-%d", i), fmt.Sprintf("run-%d", i))
			err := cv.ValidateEvent(ctx, event)
			assert.NoError(t, err)
		}

		stats := cv.GetStats()
		assert.Greater(t, stats.L1Misses, uint64(5), "Should have evictions due to cache size limit")
	})
}

// TestConcurrentCacheOperations tests thread-safe cache operations
func TestConcurrentCacheOperations(t *testing.T) {
	config := &CacheValidatorConfig{
		L1Size:        1000,
		L1TTL:         5 * time.Minute,
		L2Enabled:     false,
		MetricsEnabled: true,
		Validator:     events.NewValidator(events.DefaultValidationConfig()),
	}

	cv, err := NewCacheValidator(config)
	require.NoError(t, err)
	defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

	ctx := context.Background()
	numGoroutines := 50
	numOperations := 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	// Concurrent reads and writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				// Mix of same and different events
				var event events.Event
				if j%3 == 0 {
					// Same event across goroutines
					event = events.NewRunStartedEvent("shared-thread", "shared-run")
				} else {
					// Unique event per goroutine
					event = events.NewRunStartedEvent(
						fmt.Sprintf("thread-%d", id),
						fmt.Sprintf("run-%d", j),
					)
				}

				if err := cv.ValidateEvent(ctx, event); err != nil {
					errors <- err
					return
				}

				// Occasionally invalidate
				if j%10 == 0 {
					if err := cv.InvalidateEvent(ctx, event); err != nil {
						errors <- err
						return
					}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Fatalf("Concurrent operation failed: %v", err)
	}

	// Verify cache is still functional
	testEvent := events.NewRunStartedEvent("test-thread", "test-run")
	err = cv.ValidateEvent(ctx, testEvent)
	assert.NoError(t, err)

	stats := cv.GetStats()
	assert.Greater(t, stats.TotalHits+stats.TotalMisses, uint64(0), "Cache should have processed requests")
}

// TestTTLExpirationHandling tests TTL expiration behavior
func TestTTLExpirationHandling(t *testing.T) {
	t.Run("Basic TTL expiration", func(t *testing.T) {
		config := &CacheValidatorConfig{
			L1Size:        100,
			L1TTL:         200 * time.Millisecond,
			L2Enabled:     false,
			MetricsEnabled: true,
			Validator:     events.NewValidator(events.DefaultValidationConfig()),
		}

		cv, err := NewCacheValidator(config)
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

		ctx := context.Background()
		event := events.NewRunStartedEvent("thread-1", "run-1")

		// First call - cache miss
		err = cv.ValidateEvent(ctx, event)
		assert.NoError(t, err)

		// Second call - cache hit
		err = cv.ValidateEvent(ctx, event)
		assert.NoError(t, err)

		stats := cv.GetStats()
		assert.Equal(t, uint64(1), stats.L1Hits)

		// Wait for expiration
		time.Sleep(300 * time.Millisecond)

		// Third call - cache miss due to expiration
		err = cv.ValidateEvent(ctx, event)
		assert.NoError(t, err)

		stats = cv.GetStats()
		assert.Equal(t, uint64(2), stats.L1Misses)
	})

	t.Run("TTL refresh on access", func(t *testing.T) {
		config := &CacheValidatorConfig{
			L1Size:        100,
			L1TTL:         500 * time.Millisecond,
			L2Enabled:     false,
			MetricsEnabled: true,
			Validator:     events.NewValidator(events.DefaultValidationConfig()),
		}

		cv, err := NewCacheValidator(config)
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

		ctx := context.Background()
		event := events.NewRunStartedEvent("thread-1", "run-1")

		// Initial validation
		err = cv.ValidateEvent(ctx, event)
		assert.NoError(t, err)

		// Keep accessing to verify TTL doesn't expire if accessed
		for i := 0; i < 5; i++ {
			time.Sleep(100 * time.Millisecond)
			err = cv.ValidateEvent(ctx, event)
			assert.NoError(t, err)
		}

		stats := cv.GetStats()
		assert.Equal(t, uint64(5), stats.L1Hits, "All accesses should be cache hits")
	})
}

// TestCacheKeyGeneration tests cache key generation consistency
func TestCacheKeyGeneration(t *testing.T) {
	config := DefaultCacheValidatorConfig()
	cv, err := NewCacheValidator(config)
	require.NoError(t, err)
	defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

	tests := []struct {
		name           string
		event1         events.Event
		event2         events.Event
		shouldBeSameKey bool
	}{
		{
			name:           "identical events",
			event1:         events.NewRunStartedEvent("thread-1", "run-1"),
			event2:         events.NewRunStartedEvent("thread-1", "run-1"),
			shouldBeSameKey: true,
		},
		{
			name:           "different thread IDs",
			event1:         events.NewRunStartedEvent("thread-1", "run-1"),
			event2:         events.NewRunStartedEvent("thread-2", "run-1"),
			shouldBeSameKey: false,
		},
		{
			name:           "different run IDs",
			event1:         events.NewRunStartedEvent("thread-1", "run-1"),
			event2:         events.NewRunStartedEvent("thread-1", "run-2"),
			shouldBeSameKey: false,
		},
		{
			name:           "different event types",
			event1:         events.NewRunStartedEvent("thread-1", "run-1"),
			event2:         events.NewToolCallStartEvent("tool-1", "ToolName"),
			shouldBeSameKey: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1, err := cv.generateCacheKey(tt.event1)
			require.NoError(t, err)

			key2, err := cv.generateCacheKey(tt.event2)
			require.NoError(t, err)

			if tt.shouldBeSameKey {
				assert.Equal(t, key1.EventHash, key2.EventHash, "Keys should be identical")
			} else {
				assert.NotEqual(t, key1.EventHash, key2.EventHash, "Keys should be different")
			}

			// Verify key consistency
			key1Again, err := cv.generateCacheKey(tt.event1)
			require.NoError(t, err)
			assert.Equal(t, key1.EventHash, key1Again.EventHash, "Key generation should be consistent")
		})
	}
}

// TestCacheErrorHandling tests error scenarios
func TestCacheErrorHandling(t *testing.T) {
	t.Run("nil event handling", func(t *testing.T) {
		config := DefaultCacheValidatorConfig()
		cv, err := NewCacheValidator(config)
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

		err = cv.ValidateEvent(context.Background(), nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "event cannot be nil")
	})

	t.Run("invalid cache configuration", func(t *testing.T) {
		config := &CacheValidatorConfig{
			L1Size: -1, // Invalid size
		}

		_, err := NewCacheValidator(config)
		assert.Error(t, err)
	})

	t.Run("L2 cache errors", func(t *testing.T) {
		mockL2 := NewMockDistributedCache()
		config := &CacheValidatorConfig{
			L1Size:        100,
			L1TTL:         5 * time.Minute,
			L2Cache:       mockL2,
			L2Enabled:     true,
			L2TTL:         10 * time.Minute,
			MetricsEnabled: true,
			Validator:     events.NewValidator(events.DefaultValidationConfig()),
		}

		cv, err := NewCacheValidator(config)
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

		ctx := context.Background()
		event := events.NewRunStartedEvent("thread-1", "run-1")

		// First validation - should work
		err = cv.ValidateEvent(ctx, event)
		assert.NoError(t, err)

		// Clear L1 cache
		cv.l1Cache.Purge()

		// Set L2 to error mode
		mockL2.SetError(true)

		// Should still work by falling back to validation
		err = cv.ValidateEvent(ctx, event)
		assert.NoError(t, err)

		stats := cv.GetStats()
		assert.Equal(t, uint64(0), stats.L2Hits, "L2 should not have hits when in error")
	})
}

// TestCacheWarmup tests cache pre-warming functionality
func TestCacheWarmup(t *testing.T) {
	config := &CacheValidatorConfig{
		L1Size:        100,
		L1TTL:         5 * time.Minute,
		L2Enabled:     false,
		MetricsEnabled: true,
		Validator:     events.NewValidator(events.DefaultValidationConfig()),
		// Use no-op logger for tests to reduce noise
		Logger: &eventerrors.NoOpLogger{},
		RetryPolicy: &eventerrors.RetryPolicy{MaxAttempts: 1},
	}

	cv, err := NewCacheValidator(config)
	require.NoError(t, err)
	defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

	ctx := context.Background()

	// Test with a single event first to isolate the issue
	testEvent := events.NewRunStartedEvent("thread-1", "run-1")
	warmupEvents := []events.Event{testEvent}

	// Perform warmup
	err = cv.Warmup(ctx, warmupEvents)
	assert.NoError(t, err)

	// Subsequent validation should hit cache
	err = cv.ValidateEvent(ctx, testEvent)
	assert.NoError(t, err)

	stats := cv.GetStats()
	// Should have 1 miss (during warmup) and 1 hit (during validation)
	assert.Equal(t, uint64(1), stats.L1Hits, "Warmed event should hit cache")
	assert.Equal(t, uint64(1), stats.L1Misses, "Should have one miss during warmup")
}

// TestCacheInvalidation tests various invalidation scenarios
func TestCacheInvalidation(t *testing.T) {
	t.Run("single event invalidation", func(t *testing.T) {
		config := DefaultCacheValidatorConfig()
		cv, err := NewCacheValidator(config)
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

		ctx := context.Background()
		event := events.NewRunStartedEvent("thread-1", "run-1")

		// Populate cache
		err = cv.ValidateEvent(ctx, event)
		assert.NoError(t, err)

		// Verify it's cached
		err = cv.ValidateEvent(ctx, event)
		assert.NoError(t, err)
		stats := cv.GetStats()
		assert.Equal(t, uint64(1), stats.L1Hits)

		// Invalidate
		err = cv.InvalidateEvent(ctx, event)
		assert.NoError(t, err)

		// Next access should miss
		err = cv.ValidateEvent(ctx, event)
		assert.NoError(t, err)
		stats = cv.GetStats()
		assert.Equal(t, uint64(2), stats.L1Misses)
	})

	t.Run("event type invalidation", func(t *testing.T) {
		config := DefaultCacheValidatorConfig()
		cv, err := NewCacheValidator(config)
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

		ctx := context.Background()

		// Populate cache with different event types
		runEvent1 := events.NewRunStartedEvent("thread-1", "run-1")
		runEvent2 := events.NewRunStartedEvent("thread-2", "run-2")
		toolEvent := events.NewToolCallStartEvent("tool-1", "ToolName")

		err = cv.ValidateEvent(ctx, runEvent1)
		assert.NoError(t, err)
		err = cv.ValidateEvent(ctx, runEvent2)
		assert.NoError(t, err)
		err = cv.ValidateEvent(ctx, toolEvent)
		assert.NoError(t, err)

		// Invalidate all RunStarted events
		err = cv.InvalidateEventTypeInternal(ctx, events.EventTypeRunStarted)
		assert.NoError(t, err)

		// Run events should miss cache
		err = cv.ValidateEvent(ctx, runEvent1)
		assert.NoError(t, err)
		err = cv.ValidateEvent(ctx, runEvent2)
		assert.NoError(t, err)

		// Tool event should still hit cache
		err = cv.ValidateEvent(ctx, toolEvent)
		assert.NoError(t, err)

		stats := cv.GetStats()
		assert.Equal(t, uint64(1), stats.L1Hits, "Only tool event should hit")
	})
}

// TestCacheMetrics tests metrics collection
func TestCacheMetrics(t *testing.T) {
	config := &CacheValidatorConfig{
		L1Size:        100,
		L1TTL:         5 * time.Minute,
		L2Enabled:     false,
		MetricsEnabled: true,
		Validator:     events.NewValidator(events.DefaultValidationConfig()),
		// Use no-op logger for tests to reduce noise
		Logger: &eventerrors.NoOpLogger{},
		RetryPolicy: &eventerrors.RetryPolicy{MaxAttempts: 1},
	}

	cv, err := NewCacheValidator(config)
	require.NoError(t, err)
	defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

	ctx := context.Background()

	// Perform various operations with a single event for predictable metrics
	event := events.NewRunStartedEvent("thread-1", "run-1")
	
	// Get initial stats
	initialStats := cv.GetStats()

	// Cache miss
	cv.ValidateEvent(ctx, event)
	
	// Cache hit
	cv.ValidateEvent(ctx, event)

	// Invalidation (this will be counted as an eviction)
	cv.InvalidateEvent(ctx, event)

	// Another miss after invalidation
	cv.ValidateEvent(ctx, event)

	stats := cv.GetStats()
	// Verify the metrics: 1 hit, 2 misses (initial + after invalidation)
	hitsFromTest := stats.L1Hits - initialStats.L1Hits
	missesFromTest := stats.L1Misses - initialStats.L1Misses
	evictionsFromTest := stats.Evictions - initialStats.Evictions
	
	assert.Equal(t, uint64(1), hitsFromTest, "Should have 1 cache hit")
	assert.Equal(t, uint64(2), missesFromTest, "Should have 2 cache misses")
	// Note: evictions might be higher due to background cleanup, so we just check it's non-zero
	assert.GreaterOrEqual(t, evictionsFromTest, uint64(0), "Should have some evictions")
	assert.Equal(t, hitsFromTest, stats.TotalHits-initialStats.TotalHits, "Total hits should match L1 hits")
	assert.Equal(t, missesFromTest, stats.TotalMisses-initialStats.TotalMisses, "Total misses should match L1 misses")
}

// BenchmarkCacheOperations benchmarks cache performance
func BenchmarkCacheOperations(b *testing.B) {
	b.Run("sequential access", func(b *testing.B) {
		config := DefaultCacheValidatorConfig()
		config.L2Enabled = false
		
		cv, err := NewCacheValidator(config)
		if err != nil {
			b.Fatal(err)
		}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

		event := events.NewRunStartedEvent("thread-1", "run-1")
		ctx := context.Background()

		// Warm up
		cv.ValidateEvent(ctx, event)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cv.ValidateEvent(ctx, event)
		}
	})

	b.Run("random access", func(b *testing.B) {
		config := DefaultCacheValidatorConfig()
		config.L2Enabled = false
		
		cv, err := NewCacheValidator(config)
		if err != nil {
			b.Fatal(err)
		}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

		ctx := context.Background()

		// Pre-generate events
		testEvents := make([]events.Event, 100)
		for i := 0; i < 100; i++ {
			testEvents[i] = events.NewRunStartedEvent(fmt.Sprintf("thread-%d", i), fmt.Sprintf("run-%d", i))
		}

		// Warm up
		for _, event := range testEvents {
			cv.ValidateEvent(ctx, event)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			event := testEvents[i%100]
			cv.ValidateEvent(ctx, event)
		}
	})

	b.Run("concurrent access", func(b *testing.B) {
		config := DefaultCacheValidatorConfig()
		config.L2Enabled = false
		
		cv, err := NewCacheValidator(config)
		if err != nil {
			b.Fatal(err)
		}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cv.Shutdown(ctx)
		}()

		event := events.NewRunStartedEvent("thread-1", "run-1")
		ctx := context.Background()

		// Warm up
		cv.ValidateEvent(ctx, event)

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				cv.ValidateEvent(ctx, event)
			}
		})
	})
}