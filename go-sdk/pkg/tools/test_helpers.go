package tools

import (
	"testing"
	"time"
	"context"
)

// TestContext creates a context with an appropriate timeout for the test environment
func TestContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	
	timeout := OptimizedTestTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	
	// Set a deadline on the test itself
	t.Cleanup(func() {
		cancel()
	})
	
	return ctx, cancel
}

// PerformanceTestContext creates a context for performance tests with appropriate timeout
func PerformanceTestContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	
	var timeout time.Duration
	if isCI() {
		timeout = 10 * time.Second // Shorter for CI
	} else {
		timeout = 30 * time.Second // Longer for local testing
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	
	t.Cleanup(func() {
		cancel()
	})
	
	return ctx, cancel
}

// SkipIfCI skips a test if running in CI environment
func SkipIfCI(t *testing.T, reason string) {
	t.Helper()
	
	if isCI() {
		t.Skipf("Skipping in CI: %s", reason)
	}
}

// SkipSlowTestInCI skips slow tests in CI environment
func SkipSlowTestInCI(t *testing.T) {
	t.Helper()
	
	if isCI() {
		t.Skip("Skipping slow test in CI environment")
	}
}

// AdjustIterationsForEnvironment adjusts iteration count based on environment
func AdjustIterationsForEnvironment(baseIterations int) int {
	if isCI() {
		// Reduce iterations by 80% in CI
		return max(1, baseIterations/5)
	}
	return baseIterations
}

// AdjustDurationForEnvironment adjusts duration based on environment
func AdjustDurationForEnvironment(baseDuration time.Duration) time.Duration {
	if isCI() {
		// Reduce duration by 70% in CI
		return baseDuration * 3 / 10
	}
	return baseDuration
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// TestWithTimeout runs a test function with an appropriate timeout
func TestWithTimeout(t *testing.T, name string, timeout time.Duration, fn func(t *testing.T)) {
	t.Helper()
	
	adjustedTimeout := AdjustDurationForEnvironment(timeout)
	
	t.Run(name, func(t *testing.T) {
		done := make(chan struct{})
		
		go func() {
			defer close(done)
			fn(t)
		}()
		
		select {
		case <-done:
			// Test completed successfully
		case <-time.After(adjustedTimeout):
			t.Fatalf("Test timed out after %v", adjustedTimeout)
		}
	})
}

// BenchmarkWithOptimizedConfig runs a benchmark with optimized configuration
func BenchmarkWithOptimizedConfig(b *testing.B, fn func(b *testing.B, config *PerformanceConfig)) {
	b.Helper()
	
	config := OptimizedPerformanceConfig()
	
	// Further reduce for benchmarks
	if isCI() {
		config.BaselineIterations = 5
		config.LoadTestDuration = 2 * time.Second
	}
	
	fn(b, config)
}