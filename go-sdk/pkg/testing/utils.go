package testing

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestTimeout wraps a test function with a timeout to prevent hanging tests
func WithTestTimeout(t *testing.T, timeout time.Duration, testFunc func()) {
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Test panicked: %v", r)
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
		t.Fatalf("Test timed out after %v - possible hang detected", timeout)
	}
}

// RetryConfig configures retry behavior for flaky operations
type RetryConfig struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
	ShouldRetry   func(error) bool
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		ShouldRetry: func(err error) bool {
			return err != nil
		},
	}
}

// RetryUntilSuccess retries a function until it succeeds or max attempts are reached
func RetryUntilSuccess(ctx context.Context, config RetryConfig, operation func() error) error {
	var lastErr error
	delay := config.InitialDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled during retry attempt %d: %w", attempt, err)
		}

		lastErr = operation()
		if lastErr == nil {
			return nil // Success
		}

		if !config.ShouldRetry(lastErr) {
			return fmt.Errorf("non-retryable error on attempt %d: %w", attempt, lastErr)
		}

		if attempt < config.MaxAttempts {
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled while waiting to retry: %w", ctx.Err())
			case <-time.After(delay):
				// Continue to next attempt
			}

			// Calculate next delay with backoff
			delay = time.Duration(float64(delay) * config.BackoffFactor)
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
		}
	}

	return fmt.Errorf("operation failed after %d attempts, last error: %w", config.MaxAttempts, lastErr)
}

// EventuallyWithTimeout waits for a condition to become true within a timeout
func EventuallyWithTimeout(t *testing.T, condition func() bool, timeout time.Duration, checkInterval time.Duration, msgAndArgs ...interface{}) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		if condition() {
			return // Success
		}

		select {
		case <-ctx.Done():
			if len(msgAndArgs) > 0 {
				t.Fatalf("Condition not met within %v: %v", timeout, fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...))
			} else {
				t.Fatalf("Condition not met within %v", timeout)
			}
		case <-ticker.C:
			// Continue checking
		}
	}
}

// ConcurrentTester helps test concurrent operations safely
type ConcurrentTester struct {
	t          *testing.T
	wg         sync.WaitGroup
	errors     chan error
	panicCount int64
	mu         sync.Mutex
}

// NewConcurrentTester creates a new concurrent tester
func NewConcurrentTester(t *testing.T, bufferSize int) *ConcurrentTester {
	return &ConcurrentTester{
		t:      t,
		errors: make(chan error, bufferSize),
	}
}

// Go runs a function in a goroutine with panic recovery
func (c *ConcurrentTester) Go(fn func() error) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				atomic.AddInt64(&c.panicCount, 1)
				c.errors <- fmt.Errorf("goroutine panicked: %v", r)
			}
		}()

		if err := fn(); err != nil {
			c.errors <- err
		}
	}()
}

// Wait waits for all goroutines to complete and reports errors
func (c *ConcurrentTester) Wait() {
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines completed
	case <-time.After(30 * time.Second):
		c.t.Fatal("Concurrent test timed out waiting for goroutines")
	}

	close(c.errors)

	var allErrors []error
	for err := range c.errors {
		allErrors = append(allErrors, err)
	}

	panicCount := atomic.LoadInt64(&c.panicCount)
	if panicCount > 0 {
		c.t.Errorf("Detected %d panics during concurrent execution", panicCount)
	}

	if len(allErrors) > 0 {
		c.t.Errorf("Detected %d errors during concurrent execution:", len(allErrors))
		for i, err := range allErrors {
			c.t.Errorf("  Error %d: %v", i+1, err)
		}
	}
}

// ResourceMonitor monitors system resources during tests
type ResourceMonitor struct {
	initialGoroutines int
	initialMemory     uint64
	peakGoroutines    int
	peakMemory        uint64
	mu                sync.RWMutex
	stopCh            chan struct{}
	stopped           bool
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor() *ResourceMonitor {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return &ResourceMonitor{
		initialGoroutines: runtime.NumGoroutine(),
		initialMemory:     memStats.Alloc,
		peakGoroutines:    runtime.NumGoroutine(),
		peakMemory:        memStats.Alloc,
		stopCh:            make(chan struct{}),
	}
}

// Start begins monitoring resources
func (r *ResourceMonitor) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-r.stopCh:
				return
			case <-ticker.C:
				r.update()
			}
		}
	}()
}

func (r *ResourceMonitor) update() {
	goroutines := runtime.NumGoroutine()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	r.mu.Lock()
	if goroutines > r.peakGoroutines {
		r.peakGoroutines = goroutines
	}
	if memStats.Alloc > r.peakMemory {
		r.peakMemory = memStats.Alloc
	}
	r.mu.Unlock()
}

// Stop stops monitoring and returns resource usage stats
func (r *ResourceMonitor) Stop() ResourceStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.stopped {
		close(r.stopCh)
		r.stopped = true
	}

	// Final update
	r.mu.Unlock()
	r.update()
	r.mu.Lock()

	return ResourceStats{
		InitialGoroutines: r.initialGoroutines,
		PeakGoroutines:    r.peakGoroutines,
		InitialMemoryMB:   float64(r.initialMemory) / (1024 * 1024),
		PeakMemoryMB:      float64(r.peakMemory) / (1024 * 1024),
	}
}

// ResourceStats contains resource usage statistics
type ResourceStats struct {
	InitialGoroutines int
	PeakGoroutines    int
	InitialMemoryMB   float64
	PeakMemoryMB      float64
}

// String returns a string representation of the stats
func (r ResourceStats) String() string {
	return fmt.Sprintf("Goroutines: %d->%d, Memory: %.1fMB->%.1fMB",
		r.InitialGoroutines, r.PeakGoroutines, r.InitialMemoryMB, r.PeakMemoryMB)
}

// SynchronizedCounter provides a thread-safe counter for testing
type SynchronizedCounter struct {
	value int64
}

// NewSynchronizedCounter creates a new synchronized counter
func NewSynchronizedCounter() *SynchronizedCounter {
	return &SynchronizedCounter{}
}

// Increment atomically increments the counter
func (c *SynchronizedCounter) Increment() {
	atomic.AddInt64(&c.value, 1)
}

// Add atomically adds a value to the counter
func (c *SynchronizedCounter) Add(delta int64) {
	atomic.AddInt64(&c.value, delta)
}

// Get atomically gets the current value
func (c *SynchronizedCounter) Get() int64 {
	return atomic.LoadInt64(&c.value)
}

// Reset atomically resets the counter to zero
func (c *SynchronizedCounter) Reset() {
	atomic.StoreInt64(&c.value, 0)
}

// TestBarrier helps synchronize multiple goroutines in tests
type TestBarrier struct {
	n       int
	count   int64
	entered chan struct{}
	exit    chan struct{}
}

// NewTestBarrier creates a new test barrier for n goroutines
func NewTestBarrier(n int) *TestBarrier {
	return &TestBarrier{
		n:       n,
		entered: make(chan struct{}),
		exit:    make(chan struct{}),
	}
}

// Wait waits for all goroutines to reach the barrier
func (b *TestBarrier) Wait() {
	if atomic.AddInt64(&b.count, 1) == int64(b.n) {
		close(b.entered)
		close(b.exit)
	} else {
		<-b.entered
	}
	<-b.exit
}

// WaitWithTimeout waits for all goroutines with a timeout
func (b *TestBarrier) WaitWithTimeout(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		b.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("barrier timeout after %v", timeout)
	}
}

// CleanupFunc represents a cleanup function
type CleanupFunc func()

// TestContext provides enhanced testing context with cleanup and timeout management
type TestContext struct {
	t        *testing.T
	ctx      context.Context
	cancel   context.CancelFunc
	cleanups []CleanupFunc
	mu       sync.Mutex
}

// NewTestContext creates a new test context with timeout
func NewTestContext(t *testing.T, timeout time.Duration) *TestContext {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	tc := &TestContext{
		t:      t,
		ctx:    ctx,
		cancel: cancel,
	}

	// Auto-cleanup on test completion
	t.Cleanup(func() {
		tc.Cleanup()
	})

	return tc
}

// Context returns the underlying context
func (tc *TestContext) Context() context.Context {
	return tc.ctx
}

// AddCleanup adds a cleanup function to be called when the test completes
func (tc *TestContext) AddCleanup(cleanup CleanupFunc) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cleanups = append(tc.cleanups, cleanup)
}

// Cleanup runs all registered cleanup functions
func (tc *TestContext) Cleanup() {
	tc.cancel()

	tc.mu.Lock()
	cleanups := make([]CleanupFunc, len(tc.cleanups))
	copy(cleanups, tc.cleanups)
	tc.mu.Unlock()

	// Run cleanups in reverse order
	for i := len(cleanups) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					tc.t.Logf("Cleanup function panicked: %v", r)
				}
			}()
			cleanups[i]()
		}()
	}
}

// AssertEventually asserts that a condition becomes true within a timeout
func AssertEventually(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	EventuallyWithTimeout(t, condition, timeout, 10*time.Millisecond, message)
}

// AssertNoGoroutineLeaks checks for goroutine leaks after test completion
func AssertNoGoroutineLeaks(t *testing.T, testFunc func()) {
	detector := NewGoroutineLeakDetector()
	defer detector.CheckForLeaks(t)
	testFunc()
}
