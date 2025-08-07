package transport

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// TestManagerHelper provides isolated test manager instances with proper cleanup
type TestManagerHelper struct {
	t               *testing.T
	managers        []*SimpleManager
	transports      []Transport
	initialRoutines int
}

// NewTestManagerHelper creates a new test manager helper for isolated testing
func NewTestManagerHelper(t *testing.T) *TestManagerHelper {
	return &TestManagerHelper{
		t:               t,
		managers:        make([]*SimpleManager, 0),
		transports:      make([]Transport, 0),
		initialRoutines: runtime.NumGoroutine(),
	}
}

// CreateManager creates a new SimpleManager with proper tracking for cleanup
func (h *TestManagerHelper) CreateManager() *SimpleManager {
	manager := NewSimpleManager()
	h.managers = append(h.managers, manager)
	return manager
}

// CreateManagerWithBackpressure creates a manager with backpressure config
func (h *TestManagerHelper) CreateManagerWithBackpressure(config BackpressureConfig) *SimpleManager {
	manager := NewSimpleManagerWithBackpressure(config)
	h.managers = append(h.managers, manager)
	return manager
}

// CreateTransport creates a new test transport with proper tracking for cleanup
func (h *TestManagerHelper) CreateTransport() Transport {
	transport := NewMockTransport()
	h.transports = append(h.transports, transport)
	return transport
}

// CreateAdvancedTransport creates a new advanced mock transport with proper tracking
func (h *TestManagerHelper) CreateAdvancedTransport() *AdvancedMockTransport {
	transport := NewAdvancedMockTransport()
	h.transports = append(h.transports, transport)
	return transport
}

// Cleanup performs comprehensive cleanup of all test resources
func (h *TestManagerHelper) Cleanup() {
	// Use a context with timeout for cleanup to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop all managers first
	for _, manager := range h.managers {
		if err := manager.Stop(ctx); err != nil {
			h.t.Logf("Warning: error stopping manager during cleanup: %v", err)
		}
	}

	// Close all transports
	for _, transport := range h.transports {
		if err := transport.Close(ctx); err != nil {
			h.t.Logf("Warning: error closing transport during cleanup: %v", err)
		}
	}

	// Give goroutines time to finish
	time.Sleep(100 * time.Millisecond)

	// Check for goroutine leaks (with tolerance)
	finalRoutines := runtime.NumGoroutine()
	leaked := finalRoutines - h.initialRoutines

	if leaked > 5 { // Allow some tolerance for test framework goroutines
		h.t.Logf("Warning: potential goroutine leak detected: %d goroutines leaked", leaked)
		// Don't fail the test for this, just log it
	}

	// Force garbage collection to clean up any remaining resources
	runtime.GC()
	runtime.GC() // Run twice to be thorough
}

// WaitForCompletion waits for operations to complete with a timeout
func (h *TestManagerHelper) WaitForCompletion(timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Wait for all managers to settle
	for _, manager := range h.managers {
		if !manager.waitForReceiveGoroutines(timeout) {
			return false
		}
	}

	select {
	case <-ctx.Done():
		return false
	default:
		return true
	}
}

// RunWithTimeout runs a test function with a timeout to prevent hanging
func (h *TestManagerHelper) RunWithTimeout(timeout time.Duration, testFunc func()) bool {
	done := make(chan struct{})
	var panicked bool

	go func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
				h.t.Logf("Test function panicked: %v", r)
			}
			close(done)
		}()
		testFunc()
	}()

	select {
	case <-done:
		return !panicked
	case <-time.After(timeout):
		h.t.Log("Test function timed out")
		return false
	}
}
