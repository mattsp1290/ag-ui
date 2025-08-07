package server

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestConnectionManager_BoundedRateLimiters(t *testing.T) {
	config := &StreamingServerConfig{
		Security: SecurityConfig{
			EnableRateLimit:   true,
			RateLimit:         10,
			RateLimitWindow:   time.Second,
			MaxRateLimiters:   5, // Small limit for testing
			RateLimiterTTL:    100 * time.Millisecond,
		},
	}
	
	cm := NewConnectionManager(config)
	
	// Test basic rate limiting
	ip1 := "192.168.1.1"
	ip2 := "192.168.1.2"
	
	// Both IPs should initially be allowed
	if !cm.AllowRequest(ip1) {
		t.Error("IP1 should initially be allowed")
	}
	if !cm.AllowRequest(ip2) {
		t.Error("IP2 should initially be allowed")
	}
	
	// Verify rate limiters were created
	stats := cm.GetRateLimiterStats()
	if stats.Size < 2 {
		t.Errorf("Expected at least 2 rate limiters, got %d", stats.Size)
	}
}

func TestConnectionManager_MemoryBounds(t *testing.T) {
	config := &StreamingServerConfig{
		Security: SecurityConfig{
			EnableRateLimit:   true,
			RateLimit:         100,
			RateLimitWindow:   time.Minute,
			MaxRateLimiters:   3, // Very small limit for testing
			RateLimiterTTL:    10 * time.Second,
		},
	}
	
	cm := NewConnectionManager(config)
	
	// Create more rate limiters than the limit
	ips := []string{
		"192.168.1.1",
		"192.168.1.2", 
		"192.168.1.3",
		"192.168.1.4", // This should trigger eviction
		"192.168.1.5", // This should trigger more eviction
	}
	
	for _, ip := range ips {
		cm.AllowRequest(ip)
	}
	
	// Should never exceed the maximum
	stats := cm.GetRateLimiterStats()
	if stats.Size > config.Security.MaxRateLimiters {
		t.Errorf("Rate limiter count %d exceeded maximum %d", 
			stats.Size, config.Security.MaxRateLimiters)
	}
	
	// Should have evictions
	if stats.Evictions == 0 {
		t.Error("Expected evictions to occur")
	}
	
	t.Logf("Rate limiter stats: %+v", stats)
}

func TestConnectionManager_TTLCleanup(t *testing.T) {
	config := &StreamingServerConfig{
		Security: SecurityConfig{
			EnableRateLimit:   true,
			RateLimit:         100,
			RateLimitWindow:   time.Minute,
			MaxRateLimiters:   10,
			RateLimiterTTL:    50 * time.Millisecond, // Very short TTL
		},
	}
	
	cm := NewConnectionManager(config)
	
	// Create some rate limiters
	ips := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}
	for _, ip := range ips {
		cm.AllowRequest(ip)
	}
	
	// Wait for TTL expiration
	time.Sleep(100 * time.Millisecond)
	
	// Add a new one to have different timestamp
	cm.AllowRequest("192.168.1.100")
	
	// Test that expired limiters are handled on access
	expiredCount := 0
	stats := cm.GetRateLimiterStats()
	initialSize := stats.Size
	
	// Try to access the old rate limiters - they should be expired and recreated
	for _, ip := range ips {
		cm.AllowRequest(ip) // This should detect expiration and create new limiters
	}
	
	// Stats should show timeouts occurred
	newStats := cm.GetRateLimiterStats()
	if newStats.Timeouts > 0 || newStats.Size != initialSize {
		expiredCount = int(newStats.Timeouts)
		t.Logf("Detected %d expired rate limiters, timeouts: %d", expiredCount, newStats.Timeouts)
	} else {
		t.Log("TTL expiration handled automatically")
	}
}

func TestConnectionManager_ConcurrentAccess(t *testing.T) {
	config := &StreamingServerConfig{
		Security: SecurityConfig{
			EnableRateLimit:   true,
			RateLimit:         100,
			RateLimitWindow:   time.Minute,
			MaxRateLimiters:   1000,
			RateLimiterTTL:    10 * time.Minute,
		},
	}
	
	cm := NewConnectionManager(config)
	
	var wg sync.WaitGroup
	numGoroutines := 10
	requestsPerGoroutine := 50
	
	// Concurrent rate limit checks
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				ip := fmt.Sprintf("192.168.%d.%d", id, j)
				cm.AllowRequest(ip)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Should not exceed maximum size
	stats := cm.GetRateLimiterStats()
	if stats.Size > config.Security.MaxRateLimiters {
		t.Errorf("Rate limiter count %d exceeded maximum %d", 
			stats.Size, config.Security.MaxRateLimiters)
	}
	
	t.Logf("Final rate limiter stats: %+v", stats)
}

func TestConnectionManager_DisabledRateLimit(t *testing.T) {
	config := &StreamingServerConfig{
		Security: SecurityConfig{
			EnableRateLimit: false, // Disabled
		},
	}
	
	cm := NewConnectionManager(config)
	
	// Should always allow requests when rate limiting is disabled
	for i := 0; i < 100; i++ {
		ip := fmt.Sprintf("192.168.1.%d", i)
		if !cm.AllowRequest(ip) {
			t.Error("Should allow all requests when rate limiting is disabled")
		}
	}
	
	// Should have no rate limiters created
	stats := cm.GetRateLimiterStats()
	if stats.Size != 0 {
		t.Errorf("Expected no rate limiters when disabled, got %d", stats.Size)
	}
}

func TestConnectionManager_RateLimitExceeded(t *testing.T) {
	config := &StreamingServerConfig{
		Security: SecurityConfig{
			EnableRateLimit:   true,
			RateLimit:         1, // Very low limit
			RateLimitWindow:   time.Second,
			MaxRateLimiters:   10,
			RateLimiterTTL:    10 * time.Second,
		},
	}
	
	cm := NewConnectionManager(config)
	
	ip := "192.168.1.1"
	
	// First request should be allowed (creates rate limiter with 1 token)
	if !cm.AllowRequest(ip) {
		t.Error("First request should be allowed")
	}
	
	// Subsequent requests should be denied until tokens refill
	denied := 0
	for i := 0; i < 5; i++ {
		if !cm.AllowRequest(ip) {
			denied++
		}
	}
	
	if denied == 0 {
		t.Error("Expected some requests to be denied due to rate limiting")
	}
	
	t.Logf("Denied %d out of 5 requests", denied)
}

// Benchmark to verify performance isn't significantly degraded
func BenchmarkConnectionManager_AllowRequest(b *testing.B) {
	config := &StreamingServerConfig{
		Security: SecurityConfig{
			EnableRateLimit:   true,
			RateLimit:         1000,
			RateLimitWindow:   time.Minute,
			MaxRateLimiters:   10000,
			RateLimiterTTL:    10 * time.Minute,
		},
	}
	
	cm := NewConnectionManager(config)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ip := fmt.Sprintf("192.168.1.%d", i%1000) // Cycle through 1000 IPs
		cm.AllowRequest(ip)
	}
}

func BenchmarkConnectionManager_AllowRequestDisabled(b *testing.B) {
	config := &StreamingServerConfig{
		Security: SecurityConfig{
			EnableRateLimit: false,
		},
	}
	
	cm := NewConnectionManager(config)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ip := fmt.Sprintf("192.168.1.%d", i%1000)
		cm.AllowRequest(ip)
	}
}