package errors

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCircuitBreaker_BasicOperation(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test")
	config.MaxFailures = 3
	config.ResetTimeout = 100 * time.Millisecond

	cb := NewCircuitBreaker(config)

	// Initially closed
	if cb.State() != StateClosed {
		t.Errorf("Expected state to be CLOSED, got %v", cb.State())
	}

	// Successful operation
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	counts := cb.Counts()
	if counts.Requests != 1 || counts.TotalSuccesses != 1 {
		t.Errorf("Expected 1 request and 1 success, got %+v", counts)
	}
}

func TestCircuitBreaker_FailureThreshold(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test")
	config.MaxFailures = 3
	config.ResetTimeout = 100 * time.Millisecond

	cb := NewCircuitBreaker(config)

	// Generate failures
	for i := 0; i < 3; i++ {
		err := cb.Execute(context.Background(), func() error {
			return fmt.Errorf("failure %d", i)
		})
		if err == nil {
			t.Errorf("Expected error for failure %d", i)
		}

		if i < 2 && cb.State() != StateClosed {
			t.Errorf("Expected state to be CLOSED after %d failures, got %v", i+1, cb.State())
		}
	}

	// Should be open after 3 failures
	if cb.State() != StateOpen {
		t.Errorf("Expected state to be OPEN after 3 failures, got %v", cb.State())
	}

	// Should reject new requests
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if err == nil {
		t.Error("Expected circuit breaker to reject request when open")
	}
}

func TestCircuitBreaker_HalfOpenTransition(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test")
	config.MaxFailures = 2
	config.ResetTimeout = 50 * time.Millisecond
	config.HalfOpenMaxCalls = 2
	config.SuccessThreshold = 2

	cb := NewCircuitBreaker(config)

	// Trip the circuit breaker
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func() error {
			return fmt.Errorf("failure")
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("Expected state to be OPEN, got %v", cb.State())
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Next call should transition to half-open
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error in half-open state, got %v", err)
	}

	if cb.State() != StateHalfOpen {
		t.Errorf("Expected state to be HALF_OPEN, got %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenToClosedTransition(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test")
	config.MaxFailures = 2
	config.ResetTimeout = 50 * time.Millisecond
	config.HalfOpenMaxCalls = 3
	config.SuccessThreshold = 2

	cb := NewCircuitBreaker(config)

	// Trip the circuit breaker
	for i := 0; i < 2; i++ {
		cb.Execute(context.Background(), func() error {
			return fmt.Errorf("failure")
		})
	}

	// Wait for reset timeout
	time.Sleep(60 * time.Millisecond)

	// Make successful calls in half-open state
	for i := 0; i < 2; i++ {
		err := cb.Execute(context.Background(), func() error {
			return nil
		})
		if err != nil {
			t.Errorf("Expected no error in half-open state, got %v", err)
		}
	}

	// Should be closed now
	if cb.State() != StateClosed {
		t.Errorf("Expected state to be CLOSED after successful threshold, got %v", cb.State())
	}
}

func TestCircuitBreaker_Timeout(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test")
	config.Timeout = 50 * time.Millisecond

	cb := NewCircuitBreaker(config)

	start := time.Now()
	err := cb.Execute(context.Background(), func() error {
		time.Sleep(100 * time.Millisecond) // Longer than timeout
		return nil
	})

	duration := time.Since(start)

	if err == nil {
		t.Error("Expected timeout error")
	}

	if duration > 80*time.Millisecond {
		t.Errorf("Expected operation to timeout quickly, took %v", duration)
	}

	// Should record as failure
	counts := cb.Counts()
	if counts.TotalFailures != 1 {
		t.Errorf("Expected 1 failure due to timeout, got %d", counts.TotalFailures)
	}
}

func TestCircuitBreaker_PanicRecovery(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test")
	cb := NewCircuitBreaker(config)

	err := cb.Execute(context.Background(), func() error {
		panic("test panic")
	})

	if err == nil {
		t.Error("Expected error from panic recovery")
	}

	// Should record as failure
	counts := cb.Counts()
	if counts.TotalFailures != 1 {
		t.Errorf("Expected 1 failure due to panic, got %d", counts.TotalFailures)
	}
}

func TestCircuitBreaker_CallWithResult(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test")
	cb := NewCircuitBreaker(config)

	result, err := cb.Call(context.Background(), func() (interface{}, error) {
		return "success", nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result != "success" {
		t.Errorf("Expected 'success', got %v", result)
	}
}

func TestCircuitBreaker_ManualReset(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test")
	config.MaxFailures = 1

	cb := NewCircuitBreaker(config)

	// Trip the breaker
	cb.Execute(context.Background(), func() error {
		return fmt.Errorf("failure")
	})

	if cb.State() != StateOpen {
		t.Errorf("Expected state to be OPEN, got %v", cb.State())
	}

	// Manual reset
	cb.Reset()

	if cb.State() != StateClosed {
		t.Errorf("Expected state to be CLOSED after reset, got %v", cb.State())
	}

	// Should allow requests again
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error after reset, got %v", err)
	}
}

func TestCircuitBreaker_ManualTrip(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test")
	cb := NewCircuitBreaker(config)

	// Manual trip
	cb.Trip()

	if cb.State() != StateOpen {
		t.Errorf("Expected state to be OPEN after trip, got %v", cb.State())
	}

	// Should reject requests
	err := cb.Execute(context.Background(), func() error {
		return nil
	})
	if err == nil {
		t.Error("Expected circuit breaker to reject request after manual trip")
	}
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test")
	config.MaxFailures = 10

	cb := NewCircuitBreaker(config)

	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64

	// Run concurrent operations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			err := cb.Execute(context.Background(), func() error {
				if id%5 == 0 {
					return fmt.Errorf("failure %d", id)
				}
				return nil
			})

			if err != nil {
				atomic.AddInt64(&errorCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	counts := cb.Counts()

	if counts.Requests != 100 {
		t.Errorf("Expected 100 requests, got %d", counts.Requests)
	}

	// Should have some successes and failures
	if successCount == 0 || errorCount == 0 {
		t.Errorf("Expected both successes (%d) and errors (%d)", successCount, errorCount)
	}
}

func TestCircuitBreakerManager(t *testing.T) {
	manager := NewCircuitBreakerManager()

	// Get or create circuit breaker
	config := DefaultCircuitBreakerConfig("test1")
	cb1 := manager.GetOrCreate("test1", config)

	if cb1 == nil {
		t.Error("Expected circuit breaker to be created")
	}

	// Get existing circuit breaker
	cb2 := manager.GetOrCreate("test1", nil)

	if cb1 != cb2 {
		t.Error("Expected same circuit breaker instance")
	}

	// Get different circuit breaker
	cb3 := manager.GetOrCreate("test2", config)

	if cb1 == cb3 {
		t.Error("Expected different circuit breaker instance")
	}

	// List circuit breakers
	names := manager.List()
	if len(names) != 2 {
		t.Errorf("Expected 2 circuit breakers, got %d", len(names))
	}

	// Remove circuit breaker
	removed := manager.Remove("test1")
	if !removed {
		t.Error("Expected circuit breaker to be removed")
	}

	// Verify removal
	_, exists := manager.Get("test1")
	if exists {
		t.Error("Expected circuit breaker to not exist after removal")
	}
}

func TestCircuitBreakerManager_Stats(t *testing.T) {
	manager := NewCircuitBreakerManager()

	config := DefaultCircuitBreakerConfig("test")
	cb := manager.GetOrCreate("test", config)

	// Execute some operations
	cb.Execute(context.Background(), func() error { return nil })
	cb.Execute(context.Background(), func() error { return fmt.Errorf("error") })

	// Get stats
	stats := manager.GetStats()

	testStats, exists := stats["test"]
	if !exists {
		t.Error("Expected stats for 'test' circuit breaker")
	}

	if testStats.Counts.Requests != 2 {
		t.Errorf("Expected 2 requests, got %d", testStats.Counts.Requests)
	}

	if testStats.Counts.TotalSuccesses != 1 {
		t.Errorf("Expected 1 success, got %d", testStats.Counts.TotalSuccesses)
	}

	if testStats.Counts.TotalFailures != 1 {
		t.Errorf("Expected 1 failure, got %d", testStats.Counts.TotalFailures)
	}
}

func TestCircuitBreaker_CustomShouldTrip(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test")
	config.ShouldTrip = func(counts Counts) bool {
		// Custom logic: trip if failure rate > 50%
		if counts.Requests >= 4 {
			failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
			return failureRate > 0.5
		}
		return false
	}

	cb := NewCircuitBreaker(config)

	// Execute operations with 50% failure rate (2 success, 2 failures)
	cb.Execute(context.Background(), func() error { return nil })
	cb.Execute(context.Background(), func() error { return fmt.Errorf("error") })
	cb.Execute(context.Background(), func() error { return nil })
	cb.Execute(context.Background(), func() error { return fmt.Errorf("error") })

	counts := cb.Counts()
	t.Logf("After 4 operations: Requests=%d, Successes=%d, Failures=%d",
		counts.Requests, counts.TotalSuccesses, counts.TotalFailures)

	// Should still be closed (exactly 50%)
	if cb.State() != StateClosed {
		t.Errorf("Expected state to be CLOSED, got %v", cb.State())
	}

	// Add one more failure to exceed 50% (3 failures out of 5 = 60%)
	cb.Execute(context.Background(), func() error { return fmt.Errorf("error") })

	counts = cb.Counts()
	t.Logf("After 5 operations: Requests=%d, Successes=%d, Failures=%d",
		counts.Requests, counts.TotalSuccesses, counts.TotalFailures)

	// Should trip now
	if cb.State() != StateOpen {
		t.Errorf("Expected state to be OPEN after exceeding failure rate, got %v", cb.State())
	}
}

func TestGlobalCircuitBreaker(t *testing.T) {
	config := DefaultCircuitBreakerConfig("global-test")

	// Get global circuit breaker
	cb1 := GetCircuitBreaker("global-test", config)
	cb2 := GetCircuitBreaker("global-test", nil)

	if cb1 != cb2 {
		t.Error("Expected same global circuit breaker instance")
	}

	// Get global stats
	stats := GetCircuitBreakerStats()
	if len(stats) == 0 {
		t.Error("Expected global circuit breaker stats")
	}
}

func TestCircuitBreaker_ErrorDetails(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test")
	config.MaxFailures = 1

	cb := NewCircuitBreaker(config)

	// Trip the breaker
	cb.Execute(context.Background(), func() error {
		return fmt.Errorf("test failure")
	})

	// Try operation when open
	err := cb.Execute(context.Background(), func() error {
		return nil
	})

	if err == nil {
		t.Error("Expected error when circuit breaker is open")
	}

	// Check error details
	if baseErr, ok := err.(*BaseError); ok {
		if baseErr.Code != "CIRCUIT_BREAKER_OPEN" {
			t.Errorf("Expected error code 'CIRCUIT_BREAKER_OPEN', got '%s'", baseErr.Code)
		}

		if baseErr.Details["circuit_breaker"] != "test" {
			t.Errorf("Expected circuit breaker name 'test', got '%v'", baseErr.Details["circuit_breaker"])
		}

		if baseErr.Details["suggestion"] == nil {
			t.Error("Expected actionable guidance in error details")
		}
	} else {
		t.Errorf("Expected BaseError, got %T", err)
	}
}

func TestCircuitBreaker_ContextCancellation(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test")
	config.Timeout = 1 * time.Second

	cb := NewCircuitBreaker(config)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := cb.Execute(ctx, func() error {
		time.Sleep(100 * time.Millisecond) // Longer than context timeout
		return nil
	})

	duration := time.Since(start)

	if err == nil {
		t.Error("Expected context cancellation error")
	}

	if duration > 100*time.Millisecond {
		t.Errorf("Expected operation to be cancelled quickly, took %v", duration)
	}
}

func BenchmarkCircuitBreaker_SuccessfulOperations(b *testing.B) {
	config := DefaultCircuitBreakerConfig("bench")
	cb := NewCircuitBreaker(config)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cb.Execute(context.Background(), func() error {
				return nil
			})
		}
	})
}

func BenchmarkCircuitBreaker_FailedOperations(b *testing.B) {
	config := DefaultCircuitBreakerConfig("bench")
	config.MaxFailures = uint64(b.N) // Don't trip during benchmark
	cb := NewCircuitBreaker(config)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cb.Execute(context.Background(), func() error {
				return fmt.Errorf("benchmark error")
			})
		}
	})
}

func BenchmarkCircuitBreaker_OpenState(b *testing.B) {
	config := DefaultCircuitBreakerConfig("bench")
	config.MaxFailures = 1
	cb := NewCircuitBreaker(config)

	// Trip the circuit breaker
	cb.Execute(context.Background(), func() error {
		return fmt.Errorf("trip error")
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cb.Execute(context.Background(), func() error {
				return nil
			})
		}
	})
}
