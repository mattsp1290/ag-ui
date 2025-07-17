package testhelper

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestContext wraps a context with automatic cancellation
type TestContext struct {
	context.Context
	cancel context.CancelFunc
	t      *testing.T
	mu     sync.Mutex
	done   bool
}

// NewTestContext creates a context that is automatically cancelled at test end
func NewTestContext(t *testing.T) *TestContext {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	tc := &TestContext{
		Context: ctx,
		cancel:  cancel,
		t:       t,
	}

	t.Cleanup(func() {
		tc.Cancel()
	})

	return tc
}

// NewTestContextWithTimeout creates a context with timeout that is automatically cancelled
func NewTestContextWithTimeout(t *testing.T, timeout time.Duration) *TestContext {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	tc := &TestContext{
		Context: ctx,
		cancel:  cancel,
		t:       t,
	}

	t.Cleanup(func() {
		tc.Cancel()
	})

	return tc
}

// Cancel cancels the context (safe to call multiple times)
func (tc *TestContext) Cancel() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if !tc.done {
		tc.cancel()
		tc.done = true
		tc.t.Log("Test context cancelled")
	}
}

// Deadline returns the context deadline if set
func (tc *TestContext) Deadline() (time.Time, bool) {
	return tc.Context.Deadline()
}

// Done returns the context's done channel
func (tc *TestContext) Done() <-chan struct{} {
	return tc.Context.Done()
}

// Err returns the context's error
func (tc *TestContext) Err() error {
	return tc.Context.Err()
}

// Value returns the value associated with key
func (tc *TestContext) Value(key interface{}) interface{} {
	return tc.Context.Value(key)
}

// ContextManager manages multiple contexts in tests
type ContextManager struct {
	t        *testing.T
	mu       sync.Mutex
	contexts map[string]*TestContext
}

// NewContextManager creates a context manager
func NewContextManager(t *testing.T) *ContextManager {
	cm := &ContextManager{
		t:        t,
		contexts: make(map[string]*TestContext),
	}

	t.Cleanup(func() {
		cm.CancelAll()
	})

	return cm
}

// Create creates a named context
func (cm *ContextManager) Create(name string) *TestContext {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if ctx, exists := cm.contexts[name]; exists {
		ctx.Cancel()
	}

	ctx := NewTestContext(cm.t)
	cm.contexts[name] = ctx
	return ctx
}

// CreateWithTimeout creates a named context with timeout
func (cm *ContextManager) CreateWithTimeout(name string, timeout time.Duration) *TestContext {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if ctx, exists := cm.contexts[name]; exists {
		ctx.Cancel()
	}

	ctx := NewTestContextWithTimeout(cm.t, timeout)
	cm.contexts[name] = ctx
	return ctx
}

// Get retrieves a named context
func (cm *ContextManager) Get(name string) *TestContext {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	return cm.contexts[name]
}

// Cancel cancels a specific context
func (cm *ContextManager) Cancel(name string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if ctx, exists := cm.contexts[name]; exists {
		ctx.Cancel()
		delete(cm.contexts, name)
	}
}

// CancelAll cancels all managed contexts
func (cm *ContextManager) CancelAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for name, ctx := range cm.contexts {
		ctx.Cancel()
		cm.t.Logf("Cancelled context: %s", name)
	}

	cm.contexts = make(map[string]*TestContext)
}

// WaitForContext waits for a context to be done or timeout
func WaitForContext(t *testing.T, ctx context.Context, name string, timeout time.Duration) bool {
	t.Helper()

	select {
	case <-ctx.Done():
		t.Logf("Context %s done: %v", name, ctx.Err())
		return true
	case <-time.After(timeout):
		t.Logf("Timeout waiting for context %s", name)
		return false
	}
}

// RunWithContext runs a function with automatic context cleanup
func RunWithContext(t *testing.T, name string, fn func(context.Context) error) error {
	t.Helper()

	ctx := NewTestContext(t)
	defer ctx.Cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- fn(ctx)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		t.Logf("Context %s cancelled before completion", name)
		return ctx.Err()
	}
}

// ParallelContexts helps manage contexts in parallel tests
type ParallelContexts struct {
	t        *testing.T
	mu       sync.RWMutex
	contexts map[string]context.CancelFunc
	parent   context.Context
}

// NewParallelContexts creates a parallel context manager
func NewParallelContexts(t *testing.T) *ParallelContexts {
	parent, cancel := context.WithCancel(context.Background())

	pc := &ParallelContexts{
		t:        t,
		contexts: make(map[string]context.CancelFunc),
		parent:   parent,
	}

	t.Cleanup(func() {
		pc.CancelAll()
		cancel()
	})

	return pc
}

// Create creates a child context for a parallel test
func (pc *ParallelContexts) Create(name string) context.Context {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	ctx, cancel := context.WithCancel(pc.parent)
	pc.contexts[name] = cancel

	return ctx
}

// Cancel cancels a specific parallel context
func (pc *ParallelContexts) Cancel(name string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if cancel, exists := pc.contexts[name]; exists {
		cancel()
		delete(pc.contexts, name)
		pc.t.Logf("Cancelled parallel context: %s", name)
	}
}

// CancelAll cancels all parallel contexts
func (pc *ParallelContexts) CancelAll() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	for name, cancel := range pc.contexts {
		cancel()
		pc.t.Logf("Cancelled parallel context: %s", name)
	}

	pc.contexts = make(map[string]context.CancelFunc)
}

// TimeoutGuard provides timeout protection for operations
type TimeoutGuard struct {
	t       *testing.T
	timeout time.Duration
}

// NewTimeoutGuard creates a timeout guard
func NewTimeoutGuard(t *testing.T, timeout time.Duration) *TimeoutGuard {
	return &TimeoutGuard{
		t:       t,
		timeout: timeout,
	}
}

// Run executes a function with timeout protection
func (tg *TimeoutGuard) Run(name string, fn func() error) error {
	tg.t.Helper()

	done := make(chan error, 1)

	go func() {
		done <- fn()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(tg.timeout):
		tg.t.Errorf("Operation %s timed out after %v", name, tg.timeout)
		return context.DeadlineExceeded
	}
}

// ContextWithValues creates a context with common test values
func ContextWithValues(parent context.Context, values map[string]interface{}) context.Context {
	ctx := parent
	for k, v := range values {
		ctx = context.WithValue(ctx, k, v)
	}
	return ctx
}

// AssertContextCancelled verifies a context was cancelled within timeout
func AssertContextCancelled(t *testing.T, ctx context.Context, timeout time.Duration) {
	t.Helper()

	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled {
			t.Errorf("Context error = %v, want %v", ctx.Err(), context.Canceled)
		}
	case <-time.After(timeout):
		t.Errorf("Context was not cancelled within %v", timeout)
	}
}

// AssertContextTimeout verifies a context timed out
func AssertContextTimeout(t *testing.T, ctx context.Context, timeout time.Duration) {
	t.Helper()

	select {
	case <-ctx.Done():
		if ctx.Err() != context.DeadlineExceeded {
			t.Errorf("Context error = %v, want %v", ctx.Err(), context.DeadlineExceeded)
		}
	case <-time.After(timeout):
		t.Errorf("Context did not timeout within %v", timeout)
	}
}
