package testing

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// GoroutineLifecycleManager provides utilities for managing goroutine lifecycles
// to prevent leaks and ensure proper cleanup.
type GoroutineLifecycleManager struct {
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	activeCount  int64
	shutdownOnce sync.Once
	shutdownCh   chan struct{}
	name         string
	timeout      time.Duration
	mu           sync.RWMutex
	goroutines   map[string]*ManagedGoroutine
}

// ManagedGoroutine represents a single managed goroutine
type ManagedGoroutine struct {
	id        string
	startTime time.Time
	stack     string
	cancel    context.CancelFunc
	done      chan struct{}
}

// NewGoroutineLifecycleManager creates a new lifecycle manager
func NewGoroutineLifecycleManager(name string) *GoroutineLifecycleManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &GoroutineLifecycleManager{
		ctx:        ctx,
		cancel:     cancel,
		shutdownCh: make(chan struct{}),
		name:       name,
		timeout:    30 * time.Second,
		goroutines: make(map[string]*ManagedGoroutine),
	}
}

// WithTimeout sets the shutdown timeout
func (glm *GoroutineLifecycleManager) WithTimeout(timeout time.Duration) *GoroutineLifecycleManager {
	glm.timeout = timeout
	return glm
}

// Go starts a new managed goroutine with proper lifecycle management
func (glm *GoroutineLifecycleManager) Go(id string, fn func(context.Context)) error {
	select {
	case <-glm.shutdownCh:
		return fmt.Errorf("manager is shutting down")
	default:
	}

	// Create context for this specific goroutine
	ctx, cancel := context.WithCancel(glm.ctx)

	// Capture stack trace for debugging
	buf := make([]byte, 4096)
	stack := string(buf[:runtime.Stack(buf, false)])

	mg := &ManagedGoroutine{
		id:        id,
		startTime: time.Now(),
		stack:     stack,
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	glm.mu.Lock()
	glm.goroutines[id] = mg
	glm.mu.Unlock()

	glm.wg.Add(1)
	atomic.AddInt64(&glm.activeCount, 1)

	go func() {
		defer func() {
			// Always clean up
			glm.wg.Done()
			atomic.AddInt64(&glm.activeCount, -1)
			cancel()
			close(mg.done)

			glm.mu.Lock()
			delete(glm.goroutines, id)
			glm.mu.Unlock()

			// Recover from panics
			if r := recover(); r != nil {
				fmt.Printf("Goroutine %s/%s panicked: %v\n", glm.name, id, r)
				fmt.Printf("Stack trace:\n%s\n", mg.stack)
			}
		}()

		// Run the actual function
		fn(ctx)
	}()

	return nil
}

// GoWithRecovery starts a goroutine with custom panic recovery
func (glm *GoroutineLifecycleManager) GoWithRecovery(id string, fn func(context.Context), onPanic func(interface{})) error {
	return glm.Go(id, func(ctx context.Context) {
		defer func() {
			if r := recover(); r != nil {
				if onPanic != nil {
					onPanic(r)
				}
			}
		}()
		fn(ctx)
	})
}

// GoTicker starts a managed ticker goroutine
func (glm *GoroutineLifecycleManager) GoTicker(id string, interval time.Duration, fn func(context.Context)) error {
	return glm.Go(id, func(ctx context.Context) {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fn(ctx)
			}
		}
	})
}

// GoWorker starts a managed worker goroutine that processes from a channel
func (glm *GoroutineLifecycleManager) GoWorker(id string, workCh <-chan interface{}, fn func(context.Context, interface{})) error {
	return glm.Go(id, func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case work, ok := <-workCh:
				if !ok {
					return // Channel closed
				}
				fn(ctx, work)
			}
		}
	})
}

// GoBatch starts a batch processing goroutine
func (glm *GoroutineLifecycleManager) GoBatch(id string, batchSize int, flushInterval time.Duration,
	inputCh <-chan interface{}, fn func(context.Context, []interface{})) error {

	return glm.Go(id, func(ctx context.Context) {
		batch := make([]interface{}, 0, batchSize)
		flushTicker := time.NewTicker(flushInterval)
		defer flushTicker.Stop()

		flush := func() {
			if len(batch) > 0 {
				fn(ctx, batch)
				batch = batch[:0] // Reset slice
			}
		}

		for {
			select {
			case <-ctx.Done():
				flush() // Process remaining items
				return
			case item, ok := <-inputCh:
				if !ok {
					flush() // Channel closed, process remaining
					return
				}
				batch = append(batch, item)
				if len(batch) >= batchSize {
					flush()
				}
			case <-flushTicker.C:
				flush()
			}
		}
	})
}

// Cancel cancels a specific managed goroutine
func (glm *GoroutineLifecycleManager) Cancel(id string) error {
	glm.mu.RLock()
	mg, exists := glm.goroutines[id]
	glm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("goroutine %s not found", id)
	}

	mg.cancel()
	return nil
}

// Wait waits for a specific goroutine to complete
func (glm *GoroutineLifecycleManager) Wait(id string, timeout time.Duration) error {
	glm.mu.RLock()
	mg, exists := glm.goroutines[id]
	glm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("goroutine %s not found", id)
	}

	select {
	case <-mg.done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for goroutine %s", id)
	}
}

// GetActiveCount returns the number of currently active goroutines
func (glm *GoroutineLifecycleManager) GetActiveCount() int64 {
	return atomic.LoadInt64(&glm.activeCount)
}

// GetActiveGoroutines returns information about currently active goroutines
func (glm *GoroutineLifecycleManager) GetActiveGoroutines() map[string]GoroutineInfo {
	glm.mu.RLock()
	defer glm.mu.RUnlock()

	info := make(map[string]GoroutineInfo)
	for id, mg := range glm.goroutines {
		info[id] = GoroutineInfo{
			ID:        id,
			StartTime: mg.startTime,
			Running:   time.Since(mg.startTime),
			Stack:     mg.stack,
		}
	}

	return info
}

// GoroutineInfo contains information about a managed goroutine
type GoroutineInfo struct {
	ID        string
	StartTime time.Time
	Running   time.Duration
	Stack     string
}

// Shutdown gracefully shuts down all managed goroutines
func (glm *GoroutineLifecycleManager) Shutdown(ctx context.Context) error {
	var shutdownErr error

	glm.shutdownOnce.Do(func() {
		// Signal shutdown to prevent new goroutines
		close(glm.shutdownCh)

		// Cancel all goroutines
		glm.cancel()

		// Wait for goroutines with timeout
		done := make(chan struct{})
		go func() {
			glm.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Clean shutdown
		case <-ctx.Done():
			shutdownErr = fmt.Errorf("shutdown timeout: %d goroutines may have leaked", glm.GetActiveCount())

			// Log active goroutines for debugging
			active := glm.GetActiveGoroutines()
			if len(active) > 0 {
				fmt.Printf("Active goroutines during shutdown timeout:\n")
				for id, info := range active {
					fmt.Printf("- %s (running for %v)\n", id, info.Running)
				}
			}
		}
	})

	return shutdownErr
}

// ShutdownWithTimeout is a convenience method that uses the manager's configured timeout
func (glm *GoroutineLifecycleManager) ShutdownWithTimeout() error {
	ctx, cancel := context.WithTimeout(context.Background(), glm.timeout)
	defer cancel()
	return glm.Shutdown(ctx)
}

// MustShutdown shuts down and panics on timeout (useful for tests)
func (glm *GoroutineLifecycleManager) MustShutdown() {
	if err := glm.ShutdownWithTimeout(); err != nil {
		panic(fmt.Sprintf("Failed to shutdown %s: %v", glm.name, err))
	}
}

// Context returns the manager's context (cancelled during shutdown)
func (glm *GoroutineLifecycleManager) Context() context.Context {
	return glm.ctx
}

// SafeGoroutineManager provides additional safety features for goroutine management
type SafeGoroutineManager struct {
	*GoroutineLifecycleManager
	maxGoroutines int
	panicHandler  func(string, interface{})
}

// NewSafeGoroutineManager creates a manager with additional safety features
func NewSafeGoroutineManager(name string, maxGoroutines int) *SafeGoroutineManager {
	return &SafeGoroutineManager{
		GoroutineLifecycleManager: NewGoroutineLifecycleManager(name),
		maxGoroutines:             maxGoroutines,
	}
}

// WithPanicHandler sets a custom panic handler
func (sgm *SafeGoroutineManager) WithPanicHandler(handler func(string, interface{})) *SafeGoroutineManager {
	sgm.panicHandler = handler
	return sgm
}

// Go starts a goroutine with additional safety checks
func (sgm *SafeGoroutineManager) Go(id string, fn func(context.Context)) error {
	// Check goroutine limit
	if sgm.maxGoroutines > 0 && sgm.GetActiveCount() >= int64(sgm.maxGoroutines) {
		return fmt.Errorf("maximum goroutines reached (%d)", sgm.maxGoroutines)
	}

	return sgm.GoWithRecovery(id, fn, func(r interface{}) {
		if sgm.panicHandler != nil {
			sgm.panicHandler(id, r)
		}
	})
}

// Example usage patterns:

// Basic usage:
//   manager := NewGoroutineLifecycleManager("my-service")
//   defer manager.MustShutdown()
//
//   manager.Go("worker-1", func(ctx context.Context) {
//       for {
//           select {
//           case <-ctx.Done():
//               return
//           default:
//               // Do work
//           }
//       }
//   })

// Ticker example:
//   manager.GoTicker("metrics-collector", 30*time.Second, func(ctx context.Context) {
//       // Collect metrics
//   })

// Worker pool example:
//   workCh := make(chan interface{}, 100)
//   for i := 0; i < 5; i++ {
//       manager.GoWorker(fmt.Sprintf("worker-%d", i), workCh, func(ctx context.Context, work interface{}) {
//           // Process work
//       })
//   }

// Safe manager with limits:
//   safeManager := NewSafeGoroutineManager("limited-service", 10).
//       WithPanicHandler(func(id string, panic interface{}) {
//           log.Errorf("Goroutine %s panicked: %v", id, panic)
//       })
//   defer safeManager.MustShutdown()
