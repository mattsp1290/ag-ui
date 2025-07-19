package testing

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestCleanupManager manages cleanup operations for tests to prevent resource leaks
type TestCleanupManager struct {
	cleanups []func() error
	mu       sync.Mutex
}

// NewTestCleanupManager creates a new test cleanup manager
func NewTestCleanupManager() *TestCleanupManager {
	return &TestCleanupManager{}
}

// AddCleanup adds a cleanup function that will be called during test teardown
func (tcm *TestCleanupManager) AddCleanup(cleanup func() error) {
	tcm.mu.Lock()
	defer tcm.mu.Unlock()
	tcm.cleanups = append(tcm.cleanups, cleanup)
}

// AddSimpleCleanup adds a simple cleanup function (no error return)
func (tcm *TestCleanupManager) AddSimpleCleanup(cleanup func()) {
	tcm.AddCleanup(func() error {
		cleanup()
		return nil
	})
}

// AddContextCleanup adds a cleanup function that respects context cancellation
func (tcm *TestCleanupManager) AddContextCleanup(cleanup func(context.Context) error) {
	tcm.AddCleanup(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return cleanup(ctx)
	})
}

// Cleanup executes all registered cleanup functions in reverse order (LIFO)
func (tcm *TestCleanupManager) Cleanup(t *testing.T) {
	tcm.mu.Lock()
	defer tcm.mu.Unlock()

	// Execute cleanups in reverse order
	for i := len(tcm.cleanups) - 1; i >= 0; i-- {
		func(cleanup func() error) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Cleanup function panicked: %v", r)
				}
			}()
			
			if err := cleanup(); err != nil {
				t.Errorf("Cleanup function failed: %v", err)
			}
		}(tcm.cleanups[i])
	}
}

// TransportTestHelper provides helper functions for transport-related tests
type TransportTestHelper struct {
	*TestCleanupManager
	contexts []context.CancelFunc
	mu       sync.Mutex
}

// NewTransportTestHelper creates a new transport test helper
func NewTransportTestHelper() *TransportTestHelper {
	return &TransportTestHelper{
		TestCleanupManager: NewTestCleanupManager(),
	}
}

// CreateContextWithTimeout creates a context with timeout and adds its cleanup
func (tth *TransportTestHelper) CreateContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	
	tth.mu.Lock()
	tth.contexts = append(tth.contexts, cancel)
	tth.mu.Unlock()
	
	tth.AddSimpleCleanup(cancel)
	return ctx, cancel
}

// CreateCancellableContext creates a cancellable context and adds its cleanup
func (tth *TransportTestHelper) CreateCancellableContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	
	tth.mu.Lock()
	tth.contexts = append(tth.contexts, cancel)
	tth.mu.Unlock()
	
	tth.AddSimpleCleanup(cancel)
	return ctx, cancel
}

// Cleanup performs all cleanup operations including context cancellation
func (tth *TransportTestHelper) Cleanup(t *testing.T) {
	// First cancel all contexts
	tth.mu.Lock()
	for _, cancel := range tth.contexts {
		cancel()
	}
	tth.mu.Unlock()
	
	// Then perform other cleanup operations
	tth.TestCleanupManager.Cleanup(t)
}

// PerformanceTestHelper provides helper functions for performance tests
type PerformanceTestHelper struct {
	*TransportTestHelper
	leakDetector *GoroutineLeakDetector
}

// NewPerformanceTestHelper creates a new performance test helper
func NewPerformanceTestHelper() *PerformanceTestHelper {
	return &PerformanceTestHelper{
		TransportTestHelper: NewTransportTestHelper(),
		leakDetector:        NewGoroutineLeakDetector(),
	}
}

// CheckForLeaks checks for goroutine leaks
func (pth *PerformanceTestHelper) CheckForLeaks(t *testing.T) {
	pth.leakDetector.CheckForLeaks(t)
}

// Cleanup performs all cleanup operations and checks for leaks
func (pth *PerformanceTestHelper) Cleanup(t *testing.T) {
	// Perform regular cleanup first
	pth.TransportTestHelper.Cleanup(t)
	
	// Then check for goroutine leaks
	pth.CheckForLeaks(t)
}

// LoadTestHelper provides helper functions for load tests
type LoadTestHelper struct {
	*PerformanceTestHelper
	monitors []func() // monitoring goroutines
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewLoadTestHelper creates a new load test helper
func NewLoadTestHelper() *LoadTestHelper {
	ctx, cancel := context.WithCancel(context.Background())
	
	lth := &LoadTestHelper{
		PerformanceTestHelper: NewPerformanceTestHelper(),
		ctx:                   ctx,
		cancel:                cancel,
	}
	
	lth.AddSimpleCleanup(cancel)
	lth.AddCleanup(func() error {
		// Wait for all monitoring goroutines to finish
		done := make(chan struct{})
		go func() {
			lth.wg.Wait()
			close(done)
		}()
		
		select {
		case <-done:
			return nil
		case <-time.After(5 * time.Second):
			return fmt.Errorf("monitoring goroutines did not finish within 5 seconds")
		}
	})
	
	return lth
}

// StartMonitoring starts a monitoring goroutine with proper lifecycle management
func (lth *LoadTestHelper) StartMonitoring(monitor func(context.Context)) {
	lth.wg.Add(1)
	go func() {
		defer lth.wg.Done()
		monitor(lth.ctx)
	}()
}

// Example usage patterns:

// BasicTestPattern demonstrates basic test cleanup pattern
func BasicTestPattern(t *testing.T) {
	helper := NewTestCleanupManager()
	defer helper.Cleanup(t)
	
	// Example: Create a transport
	// transport := NewTransport(config)
	// helper.AddContextCleanup(transport.Close)
	
	// Example: Create a cancellable context
	// ctx, cancel := context.WithCancel(context.Background())
	// helper.AddSimpleCleanup(cancel)
}

// TransportTestPattern demonstrates transport test cleanup pattern
func TransportTestPattern(t *testing.T) {
	helper := NewTransportTestHelper()
	defer helper.Cleanup(t)
	
	// Create contexts with automatic cleanup
	ctx, _ := helper.CreateContextWithTimeout(30 * time.Second)
	
	// Use ctx for your transport operations
	_ = ctx
}

// PerformanceTestPattern demonstrates performance test cleanup pattern
func PerformanceTestPattern(t *testing.T) {
	helper := NewPerformanceTestHelper()
	defer helper.Cleanup(t) // This includes goroutine leak detection
	
	ctx, _ := helper.CreateContextWithTimeout(60 * time.Second)
	
	// Your performance test code here
	_ = ctx
}

// LoadTestPattern demonstrates load test cleanup pattern
func LoadTestPattern(t *testing.T) {
	helper := NewLoadTestHelper()
	defer helper.Cleanup(t)
	
	// Start monitoring with automatic cleanup
	helper.StartMonitoring(func(ctx context.Context) {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Do monitoring work
			}
		}
	})
	
	// Your load test code here
}