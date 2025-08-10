package config

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestValidationCache_BasicOperations(t *testing.T) {
	cache := NewValidationCache(DefaultValidationCacheConfig())
	defer cache.Stop()

	// Test cache miss
	config := map[string]interface{}{"key": "value"}
	result, found := cache.Get("test-validator", config)
	if found {
		t.Error("Expected cache miss for new entry")
	}
	if result != nil {
		t.Error("Expected nil result for cache miss")
	}

	// Test cache put and hit
	expectedErr := fmt.Errorf("validation error")
	cache.Put("test-validator", config, expectedErr)

	result, found = cache.Get("test-validator", config)
	if !found {
		t.Error("Expected cache hit after putting entry")
	}
	if result == nil {
		t.Error("Expected non-nil result for cache hit")
	}
	if result.Error() != expectedErr.Error() {
		t.Errorf("Expected error %v, got %v", expectedErr, result)
	}

	// Test successful validation (nil error)
	cache.Put("test-validator", config, nil)
	result, found = cache.Get("test-validator", config)
	if !found {
		t.Error("Expected cache hit for nil validation result")
	}
	if result != nil {
		t.Error("Expected nil result for successful validation")
	}
}

func TestValidationCache_TTLExpiration(t *testing.T) {
	config := &ValidationCacheConfig{
		MaxSize:       100,
		TTL:           50 * time.Millisecond,
		CleanupPeriod: 10 * time.Millisecond,
		Enabled:       true,
	}
	cache := NewValidationCache(config)
	defer cache.Stop()

	// Put an entry
	testConfig := map[string]interface{}{"key": "value"}
	testErr := fmt.Errorf("test error")
	cache.Put("test-validator", testConfig, testErr)

	// Verify it's there
	_, found := cache.Get("test-validator", testConfig)
	if !found {
		t.Error("Expected cache hit immediately after putting entry")
	}

	// Wait for expiration
	time.Sleep(60 * time.Millisecond)

	// Verify it's expired
	_, found = cache.Get("test-validator", testConfig)
	if found {
		t.Error("Expected cache miss after TTL expiration")
	}
}

func TestValidationCache_MaxSizeEviction(t *testing.T) {
	config := &ValidationCacheConfig{
		MaxSize:       3,
		TTL:           time.Minute,
		CleanupPeriod: time.Second,
		Enabled:       true,
	}
	cache := NewValidationCache(config)
	defer cache.Stop()

	// Fill cache to max capacity
	for i := 0; i < 3; i++ {
		testConfig := map[string]interface{}{"key": i}
		cache.Put("test-validator", testConfig, fmt.Errorf("error %d", i))
	}

	// Verify all entries are there
	for i := 0; i < 3; i++ {
		testConfig := map[string]interface{}{"key": i}
		_, found := cache.Get("test-validator", testConfig)
		if !found {
			t.Errorf("Expected cache hit for entry %d", i)
		}
	}

	// Add one more entry to trigger eviction
	testConfig := map[string]interface{}{"key": 99}
	cache.Put("test-validator", testConfig, fmt.Errorf("error 99"))

	// Verify the new entry is there
	_, found := cache.Get("test-validator", testConfig)
	if !found {
		t.Error("Expected cache hit for newly added entry")
	}

	// Verify cache size doesn't exceed max size
	if cache.Size() > 3 {
		t.Errorf("Cache size %d exceeds max size 3", cache.Size())
	}
}

func TestValidationCache_ConcurrentAccess(t *testing.T) {
	cache := NewValidationCache(DefaultValidationCacheConfig())
	defer cache.Stop()

	const numGoroutines = 100
	const numOperations = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch multiple goroutines performing cache operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				testConfig := map[string]interface{}{
					"id":    id,
					"value": j,
				}
				
				// Put operation
				cache.Put(fmt.Sprintf("validator-%d", id), testConfig, fmt.Errorf("error %d-%d", id, j))
				
				// Get operation
				_, _ = cache.Get(fmt.Sprintf("validator-%d", id), testConfig)
			}
		}(i)
	}

	wg.Wait()

	// Verify cache is still functional
	testConfig := map[string]interface{}{"final": "test"}
	cache.Put("final-validator", testConfig, fmt.Errorf("final error"))
	
	result, found := cache.Get("final-validator", testConfig)
	if !found {
		t.Error("Cache should be functional after concurrent operations")
	}
	if result == nil {
		t.Error("Expected non-nil result after concurrent operations")
	}
}

func TestValidationCache_Metrics(t *testing.T) {
	cache := NewValidationCache(DefaultValidationCacheConfig())
	defer cache.Stop()

	// Test initial metrics
	metrics := cache.GetMetrics()
	if metrics.Hits != 0 {
		t.Errorf("Expected 0 initial hits, got %d", metrics.Hits)
	}
	if metrics.Misses != 0 {
		t.Errorf("Expected 0 initial misses, got %d", metrics.Misses)
	}

	// Test cache miss
	testConfig := map[string]interface{}{"key": "value"}
	cache.Get("test-validator", testConfig)

	metrics = cache.GetMetrics()
	if metrics.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", metrics.Misses)
	}
	if metrics.TotalChecks != 1 {
		t.Errorf("Expected 1 total check, got %d", metrics.TotalChecks)
	}

	// Test cache hit
	cache.Put("test-validator", testConfig, fmt.Errorf("test error"))
	cache.Get("test-validator", testConfig)

	metrics = cache.GetMetrics()
	if metrics.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", metrics.Hits)
	}
	if metrics.TotalChecks != 2 {
		t.Errorf("Expected 2 total checks, got %d", metrics.TotalChecks)
	}

	// Test hit ratio
	hitRatio := cache.GetHitRatio()
	expectedRatio := 50.0 // 1 hit out of 2 total checks
	if hitRatio != expectedRatio {
		t.Errorf("Expected hit ratio %.1f%%, got %.1f%%", expectedRatio, hitRatio)
	}
}

func TestValidationCache_Invalidation(t *testing.T) {
	cache := NewValidationCache(DefaultValidationCacheConfig())
	defer cache.Stop()

	// Put some entries
	for i := 0; i < 5; i++ {
		testConfig := map[string]interface{}{"key": i}
		cache.Put(fmt.Sprintf("validator-%d", i), testConfig, fmt.Errorf("error %d", i))
	}

	// Verify entries are there
	if cache.Size() != 5 {
		t.Errorf("Expected cache size 5, got %d", cache.Size())
	}

	// Invalidate all
	cache.Invalidate()

	// Verify cache is empty
	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after invalidation, got %d", cache.Size())
	}

	// Verify entries are actually gone
	for i := 0; i < 5; i++ {
		testConfig := map[string]interface{}{"key": i}
		_, found := cache.Get(fmt.Sprintf("validator-%d", i), testConfig)
		if found {
			t.Errorf("Expected cache miss after invalidation for entry %d", i)
		}
	}

	// Verify metrics show invalidations
	metrics := cache.GetMetrics()
	if metrics.Invalidations != 5 {
		t.Errorf("Expected 5 invalidations, got %d", metrics.Invalidations)
	}
}

func TestValidationCache_DisabledCache(t *testing.T) {
	config := &ValidationCacheConfig{
		MaxSize:       100,
		TTL:           time.Minute,
		CleanupPeriod: time.Second,
		Enabled:       false, // Disabled
	}
	cache := NewValidationCache(config)
	defer cache.Stop()

	// Try to put an entry
	testConfig := map[string]interface{}{"key": "value"}
	cache.Put("test-validator", testConfig, fmt.Errorf("test error"))

	// Verify it's not cached
	_, found := cache.Get("test-validator", testConfig)
	if found {
		t.Error("Expected cache miss when caching is disabled")
	}

	// Verify cache size is 0
	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 when disabled, got %d", cache.Size())
	}
}

func TestValidationCache_KeyGeneration(t *testing.T) {
	cache := NewValidationCache(DefaultValidationCacheConfig())
	defer cache.Stop()

	// Test that different configurations generate different keys
	config1 := map[string]interface{}{"key1": "value1"}
	config2 := map[string]interface{}{"key1": "value2"}
	config3 := map[string]interface{}{"key2": "value1"}

	key1 := cache.generateCacheKey("validator", config1)
	key2 := cache.generateCacheKey("validator", config2)
	key3 := cache.generateCacheKey("validator", config3)

	if key1 == key2 {
		t.Error("Different config values should generate different keys")
	}
	if key1 == key3 {
		t.Error("Different config keys should generate different keys")
	}
	if key2 == key3 {
		t.Error("Different configurations should generate different keys")
	}

	// Test that same configuration generates same key
	key1Again := cache.generateCacheKey("validator", config1)
	if key1 != key1Again {
		t.Error("Same configuration should generate same key")
	}

	// Test that different validators generate different keys
	key1DifferentValidator := cache.generateCacheKey("different-validator", config1)
	if key1 == key1DifferentValidator {
		t.Error("Different validators should generate different keys")
	}
}

func TestValidationCache_Cleanup(t *testing.T) {
	config := &ValidationCacheConfig{
		MaxSize:       100,
		TTL:           50 * time.Millisecond,
		CleanupPeriod: 25 * time.Millisecond,
		Enabled:       true,
	}
	cache := NewValidationCache(config)
	defer cache.Stop()

	// Add some entries
	for i := 0; i < 10; i++ {
		testConfig := map[string]interface{}{"key": i}
		cache.Put(fmt.Sprintf("validator-%d", i), testConfig, fmt.Errorf("error %d", i))
	}

	// Verify entries are there
	if cache.Size() != 10 {
		t.Errorf("Expected cache size 10, got %d", cache.Size())
	}

	// Wait for cleanup to run (should happen after TTL + cleanup period)
	time.Sleep(100 * time.Millisecond)

	// Verify entries are cleaned up
	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after cleanup, got %d", cache.Size())
	}

	// Verify cleanup time is updated in metrics
	metrics := cache.GetMetrics()
	if metrics.LastCleanup.IsZero() {
		t.Error("Expected LastCleanup time to be set")
	}
}