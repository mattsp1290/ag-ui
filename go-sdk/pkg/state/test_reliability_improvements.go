package state

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	testutils "github.com/mattsp1290/ag-ui/go-sdk/pkg/testing"
)

// ReliableStateManagerTester provides utilities for testing state managers reliably
type ReliableStateManagerTester struct {
	t           *testing.T
	manager     *StateManager
	contextIDs  []string
	cleanupFunc func()
	mu          sync.Mutex
}

// NewReliableStateManagerTester creates a new reliable state manager tester
func NewReliableStateManagerTester(t *testing.T) *ReliableStateManagerTester {
	opts := DefaultManagerOptions()
	opts.EnableAudit = false // Disable for faster tests
	opts.EnableMetrics = false // Disable for cleaner tests
	
	// Use optimized performance settings for tests
	perfOpts := DefaultPerformanceOptions()
	perfOpts.EnableBatching = false // Disable batching to prevent hangs
	perfOpts.MaxConcurrency = 4     // Reasonable limit for tests
	perfOpts.MaxPoolSize = 50       // Smaller pool size
	opts.PerformanceOptimizer = NewPerformanceOptimizer(perfOpts)
	
	manager, err := NewStateManager(opts)
	if err != nil {
		t.Fatalf("Failed to create state manager: %v", err)
	}
	
	tester := &ReliableStateManagerTester{
		t:       t,
		manager: manager,
	}
	
	// Set up cleanup
	t.Cleanup(func() {
		tester.Cleanup()
	})
	
	return tester
}

// CreateContext creates a test context with cleanup tracking
func (rst *ReliableStateManagerTester) CreateContext(ctx context.Context, name string, initialState interface{}) string {
	// Convert initialState to map[string]interface{} if needed
	var metadata map[string]interface{}
	if initialState != nil {
		if stateMap, ok := initialState.(map[string]interface{}); ok {
			metadata = stateMap
		} else {
			// Create a wrapper map for non-map types
			metadata = map[string]interface{}{
				"initialState": initialState,
			}
		}
	}
	contextID, err := rst.manager.CreateContext(ctx, name, metadata)
	if err != nil {
		rst.t.Fatalf("Failed to create context %s: %v", name, err)
	}
	
	rst.mu.Lock()
	rst.contextIDs = append(rst.contextIDs, contextID)
	rst.mu.Unlock()
	
	return contextID
}

// UpdateState updates state with retry logic for reliability
func (rst *ReliableStateManagerTester) UpdateState(ctx context.Context, contextID, stateName string, updates map[string]interface{}, opts UpdateOptions) (JSONPatch, error) {
	retryConfig := testutils.DefaultRetryConfig()
	retryConfig.MaxAttempts = 3
	retryConfig.ShouldRetry = func(err error) bool {
		// Retry on temporary errors but not permanent ones
		return err != nil && err != ErrManagerClosed && err != ErrManagerClosing
	}
	
	var result JSONPatch
	err := testutils.RetryUntilSuccess(ctx, retryConfig, func() error {
		var err error
		result, err = rst.manager.UpdateState(ctx, contextID, stateName, updates, opts)
		return err
	})
	
	return result, err
}

// GetState gets state with retry logic
func (rst *ReliableStateManagerTester) GetState(ctx context.Context, contextID, stateName string) (interface{}, error) {
	retryConfig := testutils.DefaultRetryConfig()
	retryConfig.MaxAttempts = 3
	
	var result interface{}
	err := testutils.RetryUntilSuccess(ctx, retryConfig, func() error {
		var err error
		result, err = rst.manager.GetState(ctx, contextID, stateName)
		return err
	})
	
	return result, err
}

// TestConcurrentOperations tests concurrent operations with proper synchronization
func (rst *ReliableStateManagerTester) TestConcurrentOperations(numGoroutines, operationsPerGoroutine int, operation func(int, int) error) {
	tester := testutils.NewConcurrentTester(rst.t, numGoroutines*operationsPerGoroutine)
	barrier := testutils.NewTestBarrier(numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		goroutineID := i
		tester.Go(func() error {
			// Wait for all goroutines to be ready
			if err := barrier.WaitWithTimeout(5 * time.Second); err != nil {
				return err
			}
			
			// Execute operations
			for j := 0; j < operationsPerGoroutine; j++ {
				if err := operation(goroutineID, j); err != nil {
					return err
				}
			}
			return nil
		})
	}
	
	tester.Wait()
}

// WaitForCondition waits for a condition to become true with timeout
func (rst *ReliableStateManagerTester) WaitForCondition(condition func() bool, timeout time.Duration, message string) {
	testutils.EventuallyWithTimeout(rst.t, condition, timeout, 10*time.Millisecond, message)
}

// Cleanup performs cleanup operations
func (rst *ReliableStateManagerTester) Cleanup() {
	if rst.cleanupFunc != nil {
		rst.cleanupFunc()
	}
	
	if rst.manager != nil {
		// Close with timeout to prevent hanging
		done := make(chan struct{})
		go func() {
			rst.manager.Close()
			close(done)
		}()
		
		select {
		case <-done:
			// Clean shutdown
		case <-time.After(2 * time.Second):
			rst.t.Log("Warning: State manager close timed out")
		}
	}
}

// Manager returns the underlying state manager
func (rst *ReliableStateManagerTester) Manager() *StateManager {
	return rst.manager
}

// ReliableConcurrencyTester provides utilities for testing concurrent state operations
type ReliableConcurrencyTester struct {
	t               *testing.T
	manager         *StateManager
	successCount    *testutils.SynchronizedCounter
	errorCount      *testutils.SynchronizedCounter
	resourceMonitor *testutils.ResourceMonitor
}

// NewReliableConcurrencyTester creates a new concurrency tester
func NewReliableConcurrencyTester(t *testing.T) *ReliableConcurrencyTester {
	tester := NewReliableStateManagerTester(t)
	
	return &ReliableConcurrencyTester{
		t:               t,
		manager:         tester.Manager(),
		successCount:    testutils.NewSynchronizedCounter(),
		errorCount:      testutils.NewSynchronizedCounter(),
		resourceMonitor: testutils.NewResourceMonitor(),
	}
}

// TestRaceCondition tests for race conditions in state updates
func (rct *ReliableConcurrencyTester) TestRaceCondition(numGoroutines, updatesPerGoroutine int) {
	ctx := context.Background()
	contextID, err := rct.manager.CreateContext(ctx, "race-test", map[string]interface{}{
		"counter": 0,
		"values":  []int{},
	})
	if err != nil {
		rct.t.Fatalf("Failed to create context: %v", err)
	}
	
	// Start resource monitoring
	rct.resourceMonitor.Start(100 * time.Millisecond)
	defer func() {
		stats := rct.resourceMonitor.Stop()
		rct.t.Logf("Resource usage: %s", stats.String())
	}()
	
	barrier := testutils.NewTestBarrier(numGoroutines)
	tester := testutils.NewConcurrentTester(rct.t, numGoroutines*2)
	
	for i := 0; i < numGoroutines; i++ {
		goroutineID := i
		tester.Go(func() error {
			// Wait for all goroutines to start simultaneously
			if err := barrier.WaitWithTimeout(5 * time.Second); err != nil {
				return err
			}
			
			for j := 0; j < updatesPerGoroutine; j++ {
				updates := map[string]interface{}{
					"counter": goroutineID*1000 + j,
					"values":  []int{goroutineID, j},
				}
				
				ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Second)
				_, err := rct.manager.UpdateState(ctxWithTimeout, contextID, "race-test", updates, UpdateOptions{
					ConflictStrategy: LastWriteWins,
				})
				cancel()
				
				if err != nil {
					rct.errorCount.Increment()
					return err
				} else {
					rct.successCount.Increment()
				}
			}
			return nil
		})
	}
	
	tester.Wait()
	
	// Verify final state consistency
	finalState, err := rct.manager.GetState(ctx, contextID, "race-test")
	if err != nil {
		rct.t.Fatalf("Failed to get final state: %v", err)
	}
	
	if stateMap, ok := finalState.(map[string]interface{}); ok {
		if stateMap["counter"] == nil || stateMap["values"] == nil {
			rct.t.Error("Final state is missing expected fields")
		}
	} else {
		rct.t.Error("Final state is not a map")
	}
	
	rct.t.Logf("Race condition test completed - Successes: %d, Errors: %d", 
		rct.successCount.Get(), rct.errorCount.Get())
}

// TestReadWriteConcurrency tests concurrent reads and writes
func (rct *ReliableConcurrencyTester) TestReadWriteConcurrency(numReaders, numWriters, operationsPerWorker int) {
	ctx := context.Background()
	contextID, err := rct.manager.CreateContext(ctx, "rw-test", map[string]interface{}{
		"data": "initial",
		"counter": 0,
	})
	if err != nil {
		rct.t.Fatalf("Failed to create context: %v", err)
	}
	
	tester := testutils.NewConcurrentTester(rct.t, numReaders+numWriters)
	barrier := testutils.NewTestBarrier(numReaders + numWriters)
	
	var readCount, writeCount atomic.Int64
	
	// Start readers
	for i := 0; i < numReaders; i++ {
		_ = i // Use the loop variable to avoid unused variable warning
		tester.Go(func() error {
			if err := barrier.WaitWithTimeout(5 * time.Second); err != nil {
				return err
			}
			
			for j := 0; j < operationsPerWorker; j++ {
				ctxWithTimeout, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
				_, err := rct.manager.GetState(ctxWithTimeout, contextID, "rw-test")
				cancel()
				
				if err != nil {
					rct.errorCount.Increment()
					return err
				}
				readCount.Add(1)
			}
			return nil
		})
	}
	
	// Start writers
	for i := 0; i < numWriters; i++ {
		writerID := i
		tester.Go(func() error {
			if err := barrier.WaitWithTimeout(5 * time.Second); err != nil {
				return err
			}
			
			for j := 0; j < operationsPerWorker; j++ {
				updates := map[string]interface{}{
					"data": fmt.Sprintf("writer%d-update%d", writerID, j),
					"counter": writerID*1000 + j,
				}
				
				ctxWithTimeout, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
				_, err := rct.manager.UpdateState(ctxWithTimeout, contextID, "rw-test", updates, UpdateOptions{})
				cancel()
				
				if err != nil {
					rct.errorCount.Increment()
					return err
				}
				writeCount.Add(1)
			}
			return nil
		})
	}
	
	tester.Wait()
	
	rct.t.Logf("Read/Write test completed - Reads: %d, Writes: %d, Errors: %d", 
		readCount.Load(), writeCount.Load(), rct.errorCount.Get())
}

// GracefulShutdownTester helps test graceful shutdown scenarios
type GracefulShutdownTester struct {
	t       *testing.T
	manager *StateManager
}

// NewGracefulShutdownTester creates a new shutdown tester
func NewGracefulShutdownTester(t *testing.T) *GracefulShutdownTester {
	tester := NewReliableStateManagerTester(t)
	return &GracefulShutdownTester{
		t:       t,
		manager: tester.Manager(),
	}
}

// TestShutdownUnderLoad tests shutdown while operations are active
func (gst *GracefulShutdownTester) TestShutdownUnderLoad(numWorkers, operationsPerWorker int) {
	ctx := context.Background()
	
	// Create test contexts
	contextIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		contextID, err := gst.manager.CreateContext(ctx, fmt.Sprintf("shutdown-test-%d", i), nil)
		if err != nil {
			gst.t.Fatalf("Failed to create context %d: %v", i, err)
		}
		contextIDs[i] = contextID
	}
	
	// Start workers
	tester := testutils.NewConcurrentTester(gst.t, numWorkers)
	stopWorkers := make(chan struct{})
	shutdownStarted := make(chan struct{})
	
	for i := 0; i < numWorkers; i++ {
		workerID := i
		tester.Go(func() error {
			for j := 0; j < operationsPerWorker; j++ {
				select {
				case <-stopWorkers:
					return nil
				default:
				}
				
				contextID := contextIDs[j%len(contextIDs)]
				
				// Perform various operations
				switch j % 3 {
				case 0: // Write
					updates := map[string]interface{}{
						"worker": workerID,
						"time":   time.Now().UnixNano(),
					}
					
					opCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
					_, err := gst.manager.UpdateState(opCtx, contextID, fmt.Sprintf("state-%d", workerID%3), updates, UpdateOptions{})
					cancel()
					
					if err != nil {
						select {
						case <-shutdownStarted:
							// Expected during shutdown
							if err != ErrManagerClosing && err != ErrManagerClosed {
								return fmt.Errorf("unexpected error during shutdown: %w", err)
							}
						default:
							return fmt.Errorf("unexpected error before shutdown: %w", err)
						}
					}
					
				case 1: // Read
					opCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
					_, err := gst.manager.GetState(opCtx, contextID, fmt.Sprintf("state-%d", workerID%3))
					cancel()
					
					if err != nil {
						select {
						case <-shutdownStarted:
							// Expected during shutdown
						default:
							return fmt.Errorf("read error before shutdown: %w", err)
						}
					}
					
				case 2: // History
					_, err := gst.manager.GetHistory(ctx, fmt.Sprintf("state-%d", workerID%3), 5)
					if err != nil {
						select {
						case <-shutdownStarted:
							// Expected during shutdown
						default:
							return fmt.Errorf("history error before shutdown: %w", err)
						}
					}
				}
				
				// Small delay
				time.Sleep(time.Microsecond * 100)
			}
			return nil
		})
	}
	
	// Let workers run briefly
	time.Sleep(50 * time.Millisecond)
	
	// Signal shutdown started
	close(shutdownStarted)
	
	// Begin graceful shutdown with timeout
	shutdownDone := make(chan struct{})
	go func() {
		gst.manager.Close()
		close(shutdownDone)
	}()
	
	// Wait for shutdown with timeout
	select {
	case <-shutdownDone:
		gst.t.Log("Shutdown completed successfully")
	case <-time.After(3 * time.Second):
		gst.t.Error("Shutdown took too long")
	}
	
	// Stop all workers
	close(stopWorkers)
	tester.Wait()
	
	// Verify manager is closed
	_, err := gst.manager.GetState(ctx, contextIDs[0], "state-0")
	if err != ErrManagerClosed {
		gst.t.Errorf("Expected ErrManagerClosed, got %v", err)
	}
}

// NoGoroutineLeakTester helps test for goroutine leaks
func NoGoroutineLeakTester(t *testing.T, testFunc func()) {
	testutils.AssertNoGoroutineLeaks(t, testFunc)
}

// WithReliableTestContext creates a test context with timeout and cleanup
func WithReliableTestContext(t *testing.T, timeout time.Duration, testFunc func(context.Context)) {
	testCtx := testutils.NewTestContext(t, timeout)
	testFunc(testCtx.Context())
}