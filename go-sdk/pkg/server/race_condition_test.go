package server

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestBoundedMapRaceConditions tests the GetOrSet method under high concurrency
func TestBoundedMapRaceConditions(t *testing.T) {
	config := BoundedMapConfig{
		MaxSize:        100,
		EnableTimeouts: true,
		TTL:            5 * time.Second,
	}
	
	bm := NewBoundedMap[string, int](config)
	
	// Test parameters
	const (
		numGoroutines = 100
		numOperations = 1000
		testKey       = "test-key"
	)
	
	var (
		creationCount int64
		successCount  int64
		wg            sync.WaitGroup
	)
	
	// Factory function that tracks creations
	factory := func() int {
		atomic.AddInt64(&creationCount, 1)
		return 42
	}
	
	// Launch goroutines that concurrently call GetOrSet
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				value := bm.GetOrSet(testKey, factory)
				if value == 42 {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify results
	totalOperations := int64(numGoroutines * numOperations)
	
	t.Logf("Total operations: %d", totalOperations)
	t.Logf("Successful operations: %d", successCount)
	t.Logf("Factory invocations: %d", creationCount)
	t.Logf("Retry count: %d", bm.Stats().Retries)
	
	// All operations should succeed
	if successCount != totalOperations {
		t.Errorf("Expected %d successful operations, got %d", totalOperations, successCount)
	}
	
	// Factory should be called very few times (ideally just once, but allowing for retries)
	if creationCount > 10 {
		t.Errorf("Factory was called too many times: %d (indicates race condition)", creationCount)
	}
	
	// The key should exist in the map
	if value, exists := bm.Get(testKey); !exists || value != 42 {
		t.Errorf("Expected key to exist with value 42, got exists=%v, value=%v", exists, value)
	}
}

// TestRateLimiterConcurrency tests rate limiter operations under concurrent access
func TestRateLimiterConcurrency(t *testing.T) {
	config := DefaultStreamingServerConfig()
	config.Security.EnableRateLimit = true
	config.Security.RateLimit = 100
	config.Security.RateLimitWindow = time.Minute
	
	cm := NewConnectionManager(config)
	
	// Test parameters
	const (
		numGoroutines = 50
		numRequests   = 200
		testIP        = "192.168.1.100"
	)
	
	var (
		allowedCount int64
		blockedCount int64
		wg           sync.WaitGroup
	)
	
	// Launch goroutines that make concurrent rate limit requests
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for j := 0; j < numRequests; j++ {
				if cm.AllowRequest(testIP) {
					atomic.AddInt64(&allowedCount, 1)
				} else {
					atomic.AddInt64(&blockedCount, 1)
				}
			}
		}()
	}
	
	wg.Wait()
	
	totalRequests := int64(numGoroutines * numRequests)
	
	t.Logf("Total requests: %d", totalRequests)
	t.Logf("Allowed requests: %d", allowedCount)
	t.Logf("Blocked requests: %d", blockedCount)
	
	stats := cm.GetRateLimiterStats()
	t.Logf("Rate limiter stats: %+v", stats)
	
	// Verify that some requests were allowed (rate limiter is working)
	if allowedCount == 0 {
		t.Error("No requests were allowed - rate limiter may be too restrictive")
	}
	
	// Verify that some requests were blocked (rate limiting is working)
	if blockedCount == 0 {
		t.Error("No requests were blocked - rate limiting may not be working")
	}
	
	// Total should match
	if allowedCount+blockedCount != totalRequests {
		t.Errorf("Total count mismatch: %d + %d != %d", allowedCount, blockedCount, totalRequests)
	}
	
	// Verify rate limiter health
	if limiter, exists := cm.rateLimiters.Get(testIP); !exists || !limiter.IsHealthy() {
		t.Error("Rate limiter is not healthy after concurrent access")
	}
}

// TestRateLimiterEvictionRecovery tests recovery from rate limiter evictions
func TestRateLimiterEvictionRecovery(t *testing.T) {
	config := DefaultStreamingServerConfig()
	config.Security.EnableRateLimit = true
	config.Security.MaxRateLimiters = 5 // Small size to force evictions
	config.Security.RateLimiterTTL = 100 * time.Millisecond // Short TTL
	
	cm := NewConnectionManager(config)
	
	// Create many IPs to force evictions
	ips := make([]string, 20)
	for i := range ips {
		ips[i] = fmt.Sprintf("192.168.1.%d", i+1)
	}
	
	var wg sync.WaitGroup
	var totalRequests int64
	var allowedRequests int64
	
	// Make requests from many IPs concurrently
	for _, ip := range ips {
		wg.Add(1)
		go func(clientIP string) {
			defer wg.Done()
			
			for i := 0; i < 100; i++ {
				atomic.AddInt64(&totalRequests, 1)
				if cm.AllowRequest(clientIP) {
					atomic.AddInt64(&allowedRequests, 1)
				}
				
				// Small delay to allow for evictions
				time.Sleep(time.Microsecond * 100)
			}
		}(ip)
	}
	
	wg.Wait()
	
	stats := cm.GetRateLimiterStats()
	
	t.Logf("Total requests: %d", totalRequests)
	t.Logf("Allowed requests: %d", allowedRequests)
	t.Logf("Rate limiter stats: %+v", stats)
	
	// Should have had evictions due to small map size
	if stats.Evictions == 0 {
		t.Error("Expected evictions due to small map size, but none occurred")
	}
	
	// System should still function despite evictions
	if allowedRequests == 0 {
		t.Error("No requests allowed despite evictions - system may be broken")
	}
	
	// Verify health score
	health := cm.GetRateLimitingHealth()
	t.Logf("Rate limiting health: %+v", health)
	
	healthScore := health["health_score"].(float64)
	if healthScore < 0.5 {
		t.Errorf("Health score too low: %f", healthScore)
	}
}

// TestStreamingServerRateLimitIntegration tests rate limiting in full server context
func TestStreamingServerRateLimitIntegration(t *testing.T) {
	config := DefaultStreamingServerConfig()
	config.Address = ":0" // Use dynamic port allocation for tests
	config.Security.EnableRateLimit = true
	config.Security.RateLimit = 10 // Low limit for testing
	config.Security.RateLimitWindow = time.Second
	
	server, err := NewStreamingServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop(ctx)
	
	// Test concurrent rate limit checks
	const numGoroutines = 20
	var wg sync.WaitGroup
	var allowedCount int64
	var blockedCount int64
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			ip := fmt.Sprintf("10.0.0.%d", goroutineID%5) // Use 5 different IPs
			
			for j := 0; j < 50; j++ {
				if server.connectionManager.AllowRequest(ip) {
					atomic.AddInt64(&allowedCount, 1)
				} else {
					atomic.AddInt64(&blockedCount, 1)
				}
				
				// Small delay to spread requests over time
				time.Sleep(time.Millisecond * 10)
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Allowed: %d, Blocked: %d", allowedCount, blockedCount)
	
	// Get final metrics
	metrics := server.GetMetrics()
	health := server.connectionManager.GetRateLimitingHealth()
	
	t.Logf("Server metrics - Rate limit hits: %d", metrics.RateLimitHits)
	t.Logf("Rate limiting health: %+v", health)
	
	// Verify that rate limiting worked
	if blockedCount == 0 {
		t.Error("Expected some blocked requests due to rate limiting")
	}
	
	if allowedCount == 0 {
		t.Error("Expected some allowed requests")
	}
	
	// Verify no errors in rate limiting system
	if metrics.RateLimitErrors > 0 {
		t.Errorf("Rate limit errors detected: %d", metrics.RateLimitErrors)
	}
}

// BenchmarkBoundedMapGetOrSet benchmarks the GetOrSet operation under concurrency
func BenchmarkBoundedMapGetOrSet(b *testing.B) {
	config := BoundedMapConfig{
		MaxSize:        1000,
		EnableTimeouts: false,
	}
	
	bm := NewBoundedMap[string, int](config)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", counter%100) // Use 100 different keys
			bm.GetOrSet(key, func() int { return counter })
			counter++
		}
	})
	
	stats := bm.Stats()
	b.Logf("Final stats: %+v", stats)
}

// BenchmarkRateLimiterAllow benchmarks rate limiter allow operations
func BenchmarkRateLimiterAllow(b *testing.B) {
	limiter := NewRateLimiter(1000, 1000, 100) // High limits for benchmarking
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}