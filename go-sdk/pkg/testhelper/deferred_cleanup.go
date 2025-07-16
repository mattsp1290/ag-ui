package testhelper

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// DeferredCleanup provides patterns for automatic resource cleanup using defer
type DeferredCleanup struct {
	t       *testing.T
	mu      sync.Mutex
	stack   []deferredItem
	active  bool
	timeout time.Duration
}

type deferredItem struct {
	name     string
	fn       func() error
	location string
}

// NewDeferredCleanup creates a new deferred cleanup manager
func NewDeferredCleanup(t *testing.T) *DeferredCleanup {
	dc := &DeferredCleanup{
		t:       t,
		stack:   make([]deferredItem, 0),
		active:  true,
		timeout: GlobalTimeouts.Cleanup,
	}
	
	// Automatically register with test cleanup
	t.Cleanup(func() {
		dc.ExecuteAll()
	})
	
	return dc
}

// Defer adds a cleanup function to be executed in LIFO order
func (dc *DeferredCleanup) Defer(name string, fn func() error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	
	if !dc.active {
		dc.t.Logf("DeferredCleanup: Cannot defer %s - cleanup already executed", name)
		return
	}
	
	// Capture caller location for debugging
	_, file, line, _ := runtime.Caller(1)
	location := fmt.Sprintf("%s:%d", file, line)
	
	dc.stack = append(dc.stack, deferredItem{
		name:     name,
		fn:       fn,
		location: location,
	})
	
	dc.t.Logf("DeferredCleanup: Deferred %s at %s", name, location)
}

// DeferSimple adds a simple cleanup function without error handling
func (dc *DeferredCleanup) DeferSimple(name string, fn func()) {
	dc.Defer(name, func() error {
		fn()
		return nil
	})
}

// DeferClose adds a closer to be closed during cleanup
func (dc *DeferredCleanup) DeferClose(name string, closer interface{}) {
	switch c := closer.(type) {
	case interface{ Close() error }:
		dc.Defer(name, c.Close)
	case interface{ Close() }:
		dc.DeferSimple(name, c.Close)
	default:
		dc.t.Errorf("DeferredCleanup: %s does not implement Close()", name)
	}
}

// DeferCancel adds a context cancel function to cleanup
func (dc *DeferredCleanup) DeferCancel(name string, cancel context.CancelFunc) {
	dc.DeferSimple(name, func() {
		cancel()
		dc.t.Logf("DeferredCleanup: Cancelled context %s", name)
	})
}

// ExecuteAll executes all deferred cleanup functions in LIFO order
func (dc *DeferredCleanup) ExecuteAll() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	
	if !dc.active {
		return
	}
	
	dc.active = false
	
	if len(dc.stack) == 0 {
		return
	}
	
	dc.t.Logf("DeferredCleanup: Executing %d deferred items", len(dc.stack))
	
	// Execute in reverse order (LIFO)
	for i := len(dc.stack) - 1; i >= 0; i-- {
		item := dc.stack[i]
		dc.executeItem(item)
	}
	
	dc.stack = nil
}

// executeItem executes a single deferred item with timeout and error handling
func (dc *DeferredCleanup) executeItem(item deferredItem) {
	defer func() {
		if r := recover(); r != nil {
			dc.t.Logf("DeferredCleanup: Panic in %s (from %s): %v", item.name, item.location, r)
		}
	}()
	
	start := time.Now()
	
	done := make(chan error, 1)
	go func() {
		done <- item.fn()
	}()
	
	select {
	case err := <-done:
		duration := time.Since(start)
		if err != nil {
			dc.t.Logf("DeferredCleanup: %s failed after %v: %v", item.name, duration, err)
		} else {
			dc.t.Logf("DeferredCleanup: %s completed in %v", item.name, duration)
		}
	case <-time.After(dc.timeout):
		dc.t.Logf("DeferredCleanup: %s timed out after %v", item.name, dc.timeout)
	}
}

// WithTimeout sets a custom timeout for cleanup operations
func (dc *DeferredCleanup) WithTimeout(timeout time.Duration) *DeferredCleanup {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.timeout = timeout
	return dc
}

// DeferredCloser wraps a resource with automatic cleanup
type DeferredCloser[T any] struct {
	resource T
	closer   func(T) error
	dc       *DeferredCleanup
	closed   bool
	mu       sync.Mutex
}

// NewDeferredCloser creates a new deferred closer for a resource
func NewDeferredCloser[T any](dc *DeferredCleanup, name string, resource T, closer func(T) error) *DeferredCloser[T] {
	defer_closer := &DeferredCloser[T]{
		resource: resource,
		closer:   closer,
		dc:       dc,
	}
	
	dc.Defer(name, func() error {
		return defer_closer.Close()
	})
	
	return defer_closer
}

// Get returns the wrapped resource
func (dc_res *DeferredCloser[T]) Get() T {
	return dc_res.resource
}

// Close manually closes the resource
func (dc_res *DeferredCloser[T]) Close() error {
	dc_res.mu.Lock()
	defer dc_res.mu.Unlock()
	
	if dc_res.closed {
		return nil
	}
	
	dc_res.closed = true
	return dc_res.closer(dc_res.resource)
}

// IsClosed returns whether the resource has been closed
func (dc_res *DeferredCloser[T]) IsClosed() bool {
	dc_res.mu.Lock()
	defer dc_res.mu.Unlock()
	return dc_res.closed
}

// AutoCleanup provides automatic cleanup for resources using defer patterns
type AutoCleanup struct {
	t        *testing.T
	deferred *DeferredCleanup
	mu       sync.Mutex
	active   bool
}

// NewAutoCleanup creates a new auto cleanup manager
func NewAutoCleanup(t *testing.T) *AutoCleanup {
	return &AutoCleanup{
		t:        t,
		deferred: NewDeferredCleanup(t),
		active:   true,
	}
}

// Use wraps a resource creation function with automatic cleanup
func (ac *AutoCleanup) Use(name string, create func() (interface{}, func() error, error)) (interface{}, error) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	
	if !ac.active {
		return nil, fmt.Errorf("AutoCleanup is no longer active")
	}
	
	resource, cleanup, err := create()
	if err != nil {
		return nil, err
	}
	
	ac.deferred.Defer(name, cleanup)
	return resource, nil
}

// MustUse is like Use but panics on error
func (ac *AutoCleanup) MustUse(name string, create func() (interface{}, func() error, error)) interface{} {
	resource, err := ac.Use(name, create)
	if err != nil {
		ac.t.Fatalf("AutoCleanup.MustUse failed for %s: %v", name, err)
	}
	return resource
}

// CreateTempDir creates a temporary directory with automatic cleanup
func (ac *AutoCleanup) CreateTempDir(prefix string) (string, error) {
	return ac.Use(fmt.Sprintf("temp-dir-%s", prefix), func() (interface{}, func() error, error) {
		tempDir, err := NewAdvancedCleanupManager(ac.t).CreateTempDir(prefix)
		if err != nil {
			return nil, nil, err
		}
		
		return tempDir, func() error {
			return nil // Already handled by AdvancedCleanupManager
		}, nil
	})
}

// CreateMockServer creates a mock server with automatic cleanup
func (ac *AutoCleanup) CreateMockServer() (*MockHTTPServer, error) {
	server, err := ac.Use("mock-http-server", func() (interface{}, func() error, error) {
		server := NewMockHTTPServer(ac.t)
		return server, func() error {
			server.Close()
			return nil
		}, nil
	})
	
	if err != nil {
		return nil, err
	}
	
	return server.(*MockHTTPServer), nil
}

// WithResource executes a function with a resource that gets automatically cleaned up
func WithResource[T any](t *testing.T, name string, create func() (T, func() error, error), use func(T) error) error {
	dc := NewDeferredCleanup(t)
	
	resource, cleanup, err := create()
	if err != nil {
		return err
	}
	
	dc.Defer(name, cleanup)
	
	return use(resource)
}

// WithTempDir executes a function with a temporary directory
func WithTempDir(t *testing.T, prefix string, fn func(string) error) error {
	return WithResource(t, "temp-dir", func() (string, func() error, error) {
		acm := NewAdvancedCleanupManager(t)
		tempDir, err := acm.CreateTempDir(prefix)
		if err != nil {
			return "", nil, err
		}
		
		return tempDir, func() error {
			return nil // Handled by AdvancedCleanupManager
		}, nil
	}, fn)
}

// WithMockServer executes a function with a mock HTTP server
func WithMockServer(t *testing.T, fn func(*MockHTTPServer) error) error {
	return WithResource(t, "mock-server", func() (*MockHTTPServer, func() error, error) {
		server := NewMockHTTPServer(t)
		return server, func() error {
			server.Close()
			return nil
		}, nil
	}, fn)
}

// WithTimeout executes a function with a timeout and automatic cleanup
func WithTimeout[T any](t *testing.T, timeout time.Duration, fn func() (T, error)) (T, error) {
	var result T
	var resultErr error
	
	done := make(chan struct{})
	
	go func() {
		defer close(done)
		result, resultErr = fn()
	}()
	
	select {
	case <-done:
		return result, resultErr
	case <-time.After(timeout):
		return result, fmt.Errorf("operation timed out after %v", timeout)
	}
}

// CleanupOnExit ensures cleanup happens even if test exits unexpectedly
type CleanupOnExit struct {
	t       *testing.T
	cleanup []func()
	mu      sync.Mutex
	active  bool
}

// NewCleanupOnExit creates a cleanup-on-exit manager
func NewCleanupOnExit(t *testing.T) *CleanupOnExit {
	coe := &CleanupOnExit{
		t:       t,
		cleanup: make([]func(), 0),
		active:  true,
	}
	
	t.Cleanup(func() {
		coe.ExecuteAll()
	})
	
	return coe
}

// Register adds a cleanup function to be executed on exit
func (coe *CleanupOnExit) Register(fn func()) {
	coe.mu.Lock()
	defer coe.mu.Unlock()
	
	if coe.active {
		coe.cleanup = append(coe.cleanup, fn)
	}
}

// ExecuteAll executes all registered cleanup functions
func (coe *CleanupOnExit) ExecuteAll() {
	coe.mu.Lock()
	defer coe.mu.Unlock()
	
	if !coe.active {
		return
	}
	
	coe.active = false
	
	for i := len(coe.cleanup) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					coe.t.Logf("Panic in cleanup-on-exit: %v", r)
				}
			}()
			coe.cleanup[i]()
		}()
	}
}

// DeferStack provides a simple defer-like stack for cleanup
type DeferStack struct {
	stack []func()
	mu    sync.Mutex
}

// NewDeferStack creates a new defer stack
func NewDeferStack() *DeferStack {
	return &DeferStack{
		stack: make([]func(), 0),
	}
}

// Push adds a function to the defer stack
func (ds *DeferStack) Push(fn func()) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.stack = append(ds.stack, fn)
}

// Pop and execute the most recent function
func (ds *DeferStack) Pop() {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	
	if len(ds.stack) == 0 {
		return
	}
	
	fn := ds.stack[len(ds.stack)-1]
	ds.stack = ds.stack[:len(ds.stack)-1]
	
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Log panic but don't propagate
			}
		}()
		fn()
	}()
}

// PopAll executes all functions in LIFO order
func (ds *DeferStack) PopAll() {
	for ds.Len() > 0 {
		ds.Pop()
	}
}

// Len returns the number of functions in the stack
func (ds *DeferStack) Len() int {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return len(ds.stack)
}