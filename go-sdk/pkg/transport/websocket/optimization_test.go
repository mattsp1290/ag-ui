package websocket

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// TestOptimizationsIntegration validates all our performance optimizations
func TestOptimizationsIntegration(t *testing.T) {
	t.Run("ZeroCopyStringOptimization", func(t *testing.T) {
		// Test zero-copy string conversion
		data := []byte("test zero-copy optimization")
		zcb := NewZeroCopyBuffer(data)
		
		// String conversion should be extremely fast
		start := time.Now()
		for i := 0; i < 1000000; i++ {
			_ = zcb.String()
		}
		duration := time.Since(start)
		
		// Should complete 1M operations in under 100ms
		assert.Less(t, duration, 100*time.Millisecond)
		t.Logf("Zero-copy string: 1M operations in %v", duration)
	})

	t.Run("DynamicMemoryMonitoring", func(t *testing.T) {
		mm := NewMemoryManager(100 * 1024 * 1024) // 100MB
		
		// Test interval calculation
		intervals := map[float64]time.Duration{
			10.0: 60 * time.Second,   // Low pressure
			60.0: 15 * time.Second,   // Medium pressure
			85.0: 2 * time.Second,    // High pressure
			95.0: 500 * time.Millisecond, // Critical pressure
		}
		
		for pressure, expected := range intervals {
			actual := mm.getMonitoringInterval(pressure)
			assert.Equal(t, expected, actual, "For pressure %.0f%%", pressure)
		}
		
		t.Log("Dynamic memory monitoring intervals validated")
	})

	t.Run("LockFreeRateLimiting", func(t *testing.T) {
		config := DefaultSecurityConfig()
		sm := NewSecurityManager(config)
		
		// Test concurrent access without lock contention
		var wg sync.WaitGroup
		concurrent := 1000
		iterations := 1000
		
		start := time.Now()
		
		for i := 0; i < concurrent; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				clientIP := fmt.Sprintf("192.168.1.%d", id%256)
				
				for j := 0; j < iterations; j++ {
					limiter := sm.getClientLimiter(clientIP)
					require.NotNil(t, limiter)
					assert.IsType(t, &rate.Limiter{}, limiter)
				}
			}(i)
		}
		
		wg.Wait()
		duration := time.Since(start)
		
		totalOps := concurrent * iterations
		opsPerSecond := float64(totalOps) / duration.Seconds()
		
		t.Logf("Lock-free rate limiting: %d operations in %v (%.0f ops/sec)", 
			totalOps, duration, opsPerSecond)
		
		// Should achieve high throughput with sync.Map
		assert.Greater(t, opsPerSecond, 1000000.0, "Should achieve >1M ops/sec")
	})

	t.Run("CombinedOptimizations", func(t *testing.T) {
		// Test all optimizations working together
		config := DefaultPerformanceConfig()
		pm, err := NewPerformanceManager(config)
		require.NoError(t, err)
		
		// Verify zero-copy is enabled
		assert.True(t, config.EnableZeroCopy)
		
		// Verify memory pooling is enabled
		assert.True(t, config.EnableMemoryPooling)
		
		// Test buffer operations
		buf := pm.GetBuffer()
		assert.NotNil(t, buf)
		
		// Use zero-copy buffer
		data := []byte("combined optimization test")
		zcb := NewZeroCopyBuffer(data)
		str := zcb.String()
		assert.Equal(t, "combined optimization test", str)
		
		// Return buffer to pool
		pm.PutBuffer(buf)
		
		t.Log("All optimizations working together successfully")
	})
}

// TestSecurityManagerConcurrency validates sync.Map implementation
func TestSecurityManagerConcurrency(t *testing.T) {
	sm := NewSecurityManager(DefaultSecurityConfig())
	
	// Launch many goroutines to test concurrent access
	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 10000
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				// Simulate different IPs
				ip := fmt.Sprintf("10.0.%d.%d", id%256, j%256)
				
				// Get limiter (this should be lock-free with sync.Map)
				limiter := sm.getClientLimiter(ip)
				require.NotNil(t, limiter)
				
				// Use the limiter
				limiter.Allow()
			}
		}(i)
	}
	
	// Measure time
	start := time.Now()
	wg.Wait()
	duration := time.Since(start)
	
	totalOps := numGoroutines * numOperations
	opsPerSec := float64(totalOps) / duration.Seconds()
	
	t.Logf("Processed %d rate limiter operations in %v (%.0f ops/sec)",
		totalOps, duration, opsPerSec)
	
	// With sync.Map, we should see significant throughput
	assert.Greater(t, opsPerSec, 500000.0, "Should process >500k ops/sec")
}

// BenchmarkOptimizationComparison compares optimized vs unoptimized
func BenchmarkOptimizationComparison(b *testing.B) {
	b.Run("OptimizedZeroCopy", func(b *testing.B) {
		data := make([]byte, 1024)
		zcb := NewZeroCopyBuffer(data)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = zcb.String()
		}
	})
	
	b.Run("UnoptimizedStringCopy", func(b *testing.B) {
		data := make([]byte, 1024)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = string(data) // Standard conversion (creates copy)
		}
	})
}