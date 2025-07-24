package testing

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// TestTimeout provides timeout protection to prevent tests from hanging
const (
	// ShortTestTimeout for quick unit tests
	ShortTestTimeout = 10 * time.Second
	// MediumTestTimeout for integration tests  
	MediumTestTimeout = 30 * time.Second
	// LongTestTimeout for load/stress tests
	LongTestTimeout = 60 * time.Second
)

// WithTimeout wraps a test function with a timeout to prevent hanging
func WithTimeout(t testing.TB, timeout time.Duration, testFunc func()) {
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Test panic recovered: %v", r)
			}
			close(done)
		}()
		testFunc()
	}()
	
	select {
	case <-done:
		// Test completed successfully
		return
	case <-time.After(timeout):
		// Print goroutine stack traces for debugging
		buf := make([]byte, 1<<20)
		stackLen := runtime.Stack(buf, true)
		t.Logf("Goroutine stack traces at timeout:\n%s", buf[:stackLen])
		t.Fatalf("Test timed out after %v - possible hang detected", timeout)
	}
}

// WithContextTimeout creates a context with timeout for test operations
func WithContextTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

// FastFail provides early termination for tests that should fail quickly
func FastFail(ctx context.Context, t testing.TB, operation func() error) error {
	done := make(chan error, 1)
	go func() {
		done <- operation()
	}()
	
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		t.Logf("Operation cancelled due to context timeout")
		return ctx.Err()
	}
}

// QuickCleanup runs cleanup with a timeout to prevent hangs during teardown
func QuickCleanup(t testing.TB, timeout time.Duration, cleanup func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		cleanup()
	}()
	
	select {
	case <-done:
		// Cleanup completed successfully
	case <-time.After(timeout):
		t.Logf("Warning: Cleanup timed out after %v", timeout)
	}
}