package testhelper

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// SafeWaitGroup provides a safer WaitGroup with leak detection and debugging
type SafeWaitGroup struct {
	wg         sync.WaitGroup
	t          *testing.T
	name       string
	counter    int64
	mu         sync.RWMutex
	goroutines map[string]time.Time
	timeout    time.Duration
}

// NewSafeWaitGroup creates a new safe wait group
func NewSafeWaitGroup(t *testing.T) *SafeWaitGroup {
	return &SafeWaitGroup{
		t:          t,
		name:       "default",
		goroutines: make(map[string]time.Time),
		timeout:    GlobalTimeouts.Context,
	}
}

// NewNamedSafeWaitGroup creates a new named safe wait group
func NewNamedSafeWaitGroup(t *testing.T, name string) *SafeWaitGroup {
	return &SafeWaitGroup{
		t:          t,
		name:       name,
		goroutines: make(map[string]time.Time),
		timeout:    GlobalTimeouts.Context,
	}
}

// Add increments the WaitGroup counter and tracks the goroutine
func (swg *SafeWaitGroup) Add(delta int) {
	swg.wg.Add(delta)

	newCount := atomic.AddInt64(&swg.counter, int64(delta))
	swg.t.Logf("SafeWaitGroup[%s]: Add(%d) -> count=%d", swg.name, delta, newCount)

	if delta > 0 {
		// Track the goroutine that added work
		swg.mu.Lock()
		goroutineID := getGoroutineID()
		swg.goroutines[goroutineID] = time.Now()
		swg.mu.Unlock()
	}
}

// Done decrements the WaitGroup counter
func (swg *SafeWaitGroup) Done() {
	swg.wg.Done()

	newCount := atomic.AddInt64(&swg.counter, -1)
	swg.t.Logf("SafeWaitGroup[%s]: Done() -> count=%d", swg.name, newCount)

	// Remove the goroutine from tracking
	swg.mu.Lock()
	goroutineID := getGoroutineID()
	delete(swg.goroutines, goroutineID)
	swg.mu.Unlock()
}

// Wait waits for all goroutines to complete
func (swg *SafeWaitGroup) Wait() {
	swg.t.Logf("SafeWaitGroup[%s]: Waiting for %d goroutines", swg.name, atomic.LoadInt64(&swg.counter))

	done := make(chan struct{})
	go func() {
		swg.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		swg.t.Logf("SafeWaitGroup[%s]: All goroutines completed", swg.name)
	case <-time.After(swg.timeout):
		swg.reportStuckGoroutines()
		swg.t.Errorf("SafeWaitGroup[%s]: Timeout after %v waiting for goroutines", swg.name, swg.timeout)
	}
}

// WaitWithTimeout waits with a custom timeout
func (swg *SafeWaitGroup) WaitWithTimeout(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		swg.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		swg.t.Logf("SafeWaitGroup[%s]: All goroutines completed within %v", swg.name, timeout)
		return true
	case <-time.After(timeout):
		swg.reportStuckGoroutines()
		swg.t.Logf("SafeWaitGroup[%s]: Timeout after %v waiting for goroutines", swg.name, timeout)
		return false
	}
}

// GetCount returns the current counter value
func (swg *SafeWaitGroup) GetCount() int64 {
	return atomic.LoadInt64(&swg.counter)
}

// SetTimeout sets the default timeout for Wait operations
func (swg *SafeWaitGroup) SetTimeout(timeout time.Duration) {
	swg.timeout = timeout
}

// reportStuckGoroutines reports goroutines that may be stuck
func (swg *SafeWaitGroup) reportStuckGoroutines() {
	swg.mu.RLock()
	defer swg.mu.RUnlock()

	now := time.Now()
	for goroutineID, startTime := range swg.goroutines {
		duration := now.Sub(startTime)
		swg.t.Logf("SafeWaitGroup[%s]: Goroutine %s has been running for %v", swg.name, goroutineID, duration)
	}
}

// GetWaitGroup returns the underlying sync.WaitGroup (for compatibility)
func (swg *SafeWaitGroup) GetWaitGroup() *sync.WaitGroup {
	return &swg.wg
}

// WaitGroup property for accessing the underlying WaitGroup
var WaitGroup *sync.WaitGroup

// getGoroutineID returns a simple goroutine identifier
func getGoroutineID() string {
	// Simple implementation - in real scenarios you might want a more sophisticated approach
	return "goroutine-" + time.Now().Format("15:04:05.000000")
}

// WaitGroupPool manages a pool of SafeWaitGroups
type WaitGroupPool struct {
	t          *testing.T
	mu         sync.Mutex
	waitGroups map[string]*SafeWaitGroup
	cleanup    *CleanupHelper
}

// NewWaitGroupPool creates a new wait group pool
func NewWaitGroupPool(t *testing.T) *WaitGroupPool {
	pool := &WaitGroupPool{
		t:          t,
		waitGroups: make(map[string]*SafeWaitGroup),
		cleanup:    NewCleanupHelper(t),
	}

	pool.cleanup.Add(func() {
		pool.WaitAll()
	})

	return pool
}

// Get gets or creates a named wait group
func (wgp *WaitGroupPool) Get(name string) *SafeWaitGroup {
	wgp.mu.Lock()
	defer wgp.mu.Unlock()

	if wg, exists := wgp.waitGroups[name]; exists {
		return wg
	}

	wg := NewNamedSafeWaitGroup(wgp.t, name)
	wgp.waitGroups[name] = wg
	return wg
}

// WaitAll waits for all wait groups in the pool
func (wgp *WaitGroupPool) WaitAll() {
	wgp.mu.Lock()
	waitGroups := make([]*SafeWaitGroup, 0, len(wgp.waitGroups))
	for _, wg := range wgp.waitGroups {
		waitGroups = append(waitGroups, wg)
	}
	wgp.mu.Unlock()

	wgp.t.Logf("WaitGroupPool: Waiting for %d wait groups", len(waitGroups))

	for _, wg := range waitGroups {
		wg.Wait()
	}

	wgp.t.Log("WaitGroupPool: All wait groups completed")
}

// WaitAllWithTimeout waits for all wait groups with a timeout
func (wgp *WaitGroupPool) WaitAllWithTimeout(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	wgp.mu.Lock()
	waitGroups := make([]*SafeWaitGroup, 0, len(wgp.waitGroups))
	for _, wg := range wgp.waitGroups {
		waitGroups = append(waitGroups, wg)
	}
	wgp.mu.Unlock()

	for _, wg := range waitGroups {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			wgp.t.Log("WaitGroupPool: Timeout reached")
			return false
		}

		if !wg.WaitWithTimeout(remaining) {
			return false
		}
	}

	return true
}

// GetActiveCount returns the number of active wait groups
func (wgp *WaitGroupPool) GetActiveCount() int {
	wgp.mu.Lock()
	defer wgp.mu.Unlock()

	activeCount := 0
	for _, wg := range wgp.waitGroups {
		if wg.GetCount() > 0 {
			activeCount++
		}
	}

	return activeCount
}

// Remove removes a wait group from the pool
func (wgp *WaitGroupPool) Remove(name string) {
	wgp.mu.Lock()
	defer wgp.mu.Unlock()

	if wg, exists := wgp.waitGroups[name]; exists {
		wg.Wait() // Wait for completion before removing
		delete(wgp.waitGroups, name)
		wgp.t.Logf("WaitGroupPool: Removed wait group %s", name)
	}
}

// Reset clears all wait groups from the pool
func (wgp *WaitGroupPool) Reset() {
	wgp.WaitAll()

	wgp.mu.Lock()
	defer wgp.mu.Unlock()

	wgp.waitGroups = make(map[string]*SafeWaitGroup)
	wgp.t.Log("WaitGroupPool: Reset completed")
}

// ConcurrentRunner helps run multiple goroutines safely with wait groups
type ConcurrentRunner struct {
	t       *testing.T
	wg      *SafeWaitGroup
	errChan chan error
	cleanup *DeferredCleanup
}

// NewConcurrentRunner creates a new concurrent runner
func NewConcurrentRunner(t *testing.T) *ConcurrentRunner {
	return &ConcurrentRunner{
		t:       t,
		wg:      NewSafeWaitGroup(t),
		errChan: make(chan error, 100), // Buffer for errors
		cleanup: NewDeferredCleanup(t),
	}
}

// Run executes a function in a goroutine
func (cr *ConcurrentRunner) Run(name string, fn func() error) {
	cr.wg.Add(1)

	go func() {
		defer cr.wg.Done()

		cr.t.Logf("ConcurrentRunner: Starting %s", name)

		defer func() {
			if r := recover(); r != nil {
				cr.t.Logf("ConcurrentRunner: Panic in %s: %v", name, r)
				cr.errChan <- fmt.Errorf("panic in %s: %v", name, r)
			}
		}()

		if err := fn(); err != nil {
			cr.t.Logf("ConcurrentRunner: Error in %s: %v", name, err)
			cr.errChan <- fmt.Errorf("%s: %w", name, err)
		} else {
			cr.t.Logf("ConcurrentRunner: Completed %s", name)
		}
	}()
}

// RunSimple executes a simple function in a goroutine
func (cr *ConcurrentRunner) RunSimple(name string, fn func()) {
	cr.Run(name, func() error {
		fn()
		return nil
	})
}

// Wait waits for all goroutines to complete and returns any errors
func (cr *ConcurrentRunner) Wait() []error {
	cr.wg.Wait()
	close(cr.errChan)

	var errors []error
	for err := range cr.errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		cr.t.Logf("ConcurrentRunner: Completed with %d errors", len(errors))
	} else {
		cr.t.Log("ConcurrentRunner: All goroutines completed successfully")
	}

	return errors
}

// WaitWithTimeout waits with a timeout and returns any errors
func (cr *ConcurrentRunner) WaitWithTimeout(timeout time.Duration) ([]error, bool) {
	success := cr.wg.WaitWithTimeout(timeout)

	// Don't close the channel if we timed out, as goroutines might still be running
	if success {
		close(cr.errChan)
	}

	var errors []error
	for {
		select {
		case err, ok := <-cr.errChan:
			if !ok {
				return errors, success
			}
			errors = append(errors, err)
		default:
			return errors, success
		}
	}
}

// GetRunningCount returns the number of currently running goroutines
func (cr *ConcurrentRunner) GetRunningCount() int64 {
	return cr.wg.GetCount()
}
