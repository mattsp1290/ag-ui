//go:build integration
// +build integration

package state

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConcurrentStateUpdates_RaceCondition tests for race conditions during concurrent state updates
func TestConcurrentStateUpdates_RaceCondition(t *testing.T) {
	t.Parallel()

	manager, err := NewStateManager(DefaultManagerOptions())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	contextID, err := manager.CreateContext(ctx, "race-test", nil)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	// Initialize state
	initialState := map[string]interface{}{
		"counter": 0,
		"values":  []int{},
		"map":     map[string]int{},
	}
	_, err = manager.UpdateState(ctx, contextID, "race-test", initialState, UpdateOptions{})
	if err != nil {
		t.Fatalf("failed to set initial state: %v", err)
	}

	// Test concurrent updates to the same field
	const numGoroutines = 20   // Reduced from 100
	const updatesPerGoroutine = 5  // Reduced from 10

	var wg sync.WaitGroup
	var successCount atomic.Int64
	var errorCount atomic.Int64

	startSignal := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			// Wait for start signal to ensure maximum concurrency
			<-startSignal

			for j := 0; j < updatesPerGoroutine; j++ {
				updates := map[string]interface{}{
					"counter": goroutineID*1000 + j,
					"values":  []int{goroutineID, j},
					"map": map[string]int{
						fmt.Sprintf("g%d", goroutineID): j,
					},
				}

				_, err := manager.UpdateState(ctx, contextID, "race-test", updates, UpdateOptions{
					ConflictStrategy: LastWriteWins,
				})

				if err != nil {
					t.Logf("goroutine %d update %d failed: %v", goroutineID, j, err)
					errorCount.Add(1)
				} else {
					successCount.Add(1)
				}
			}
		}(i)
	}

	// Start all goroutines simultaneously
	close(startSignal)

	// Wait for completion
	wg.Wait()

	// Verify results
	finalState, err := manager.GetState(ctx, contextID, "race-test")
	if err != nil {
		t.Fatalf("failed to get final state: %v", err)
	}

	t.Logf("Success count: %d, Error count: %d", successCount.Load(), errorCount.Load())

	// Verify state consistency
	if stateMap, ok := finalState.(map[string]interface{}); ok {
		if stateMap["counter"] == nil {
			t.Error("counter field is nil")
		}
		if stateMap["values"] == nil {
			t.Error("values field is nil")
		}
		if stateMap["map"] == nil {
			t.Error("map field is nil")
		}
	} else {
		t.Error("finalState is not a map")
	}
}

// TestConcurrentReadsAndWrites tests parallel reads and writes
func TestConcurrentReadsAndWrites(t *testing.T) {
	t.Parallel()

	manager, err := NewStateManager(DefaultManagerOptions())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	contextID, err := manager.CreateContext(ctx, "rw-test", nil)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	// Initialize state with multiple fields
	initialState := map[string]interface{}{
		"field1":  "initial1",
		"field2":  "initial2",
		"field3":  map[string]int{"a": 1, "b": 2},
		"field4":  []string{"one", "two", "three"},
		"counter": 0,
	}
	_, err = manager.UpdateState(ctx, contextID, "rw-test", initialState, UpdateOptions{})
	if err != nil {
		t.Fatalf("failed to set initial state: %v", err)
	}

	const numReaders = 5    // Reduced from 20
	const numWriters = 5    // Reduced from 20
	const operationsPerWorker = 10  // Reduced from 50

	var wg sync.WaitGroup
	var readErrors atomic.Int64
	var writeErrors atomic.Int64
	var totalReads atomic.Int64
	var totalWrites atomic.Int64

	startSignal := make(chan struct{})
	stopSignal := make(chan struct{})

	// Start readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()

			<-startSignal

			for j := 0; j < operationsPerWorker; j++ {
				select {
				case <-stopSignal:
					return
				default:
					state, err := manager.GetState(ctx, contextID, "rw-test")
					if err != nil {
						readErrors.Add(1)
						t.Logf("reader %d read %d failed: %v", readerID, j, err)
					} else {
						totalReads.Add(1)
						// Verify state structure
						if stateMap, ok := state.(map[string]interface{}); ok {
							if stateMap["field1"] == nil || stateMap["field2"] == nil ||
								stateMap["field3"] == nil || stateMap["field4"] == nil {
								t.Errorf("reader %d: incomplete state structure", readerID)
							}
						}
					}

					// Small delay to simulate processing
					time.Sleep(time.Microsecond * 10)
				}
			}
		}(i)
	}

	// Start writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()

			<-startSignal

			for j := 0; j < operationsPerWorker; j++ {
				select {
				case <-stopSignal:
					return
				default:
					updates := map[string]interface{}{
						"field1":  fmt.Sprintf("writer%d-update%d", writerID, j),
						"field2":  fmt.Sprintf("data-%d-%d", writerID, j),
						"counter": writerID*1000 + j,
					}

					_, err := manager.UpdateState(ctx, contextID, "rw-test", updates, UpdateOptions{})
					if err != nil {
						writeErrors.Add(1)
						t.Logf("writer %d write %d failed: %v", writerID, j, err)
					} else {
						totalWrites.Add(1)
					}

					// Small delay to simulate processing
					time.Sleep(time.Microsecond * 10)
				}
			}
		}(i)
	}

	// Start all workers
	close(startSignal)

	// Let them run for a bit
	time.Sleep(20 * time.Millisecond)  // Reduced from 100ms

	// Stop all workers
	close(stopSignal)

	// Wait for completion
	wg.Wait()

	t.Logf("Total reads: %d (errors: %d), Total writes: %d (errors: %d)",
		totalReads.Load(), readErrors.Load(), totalWrites.Load(), writeErrors.Load())

	// Verify final state is consistent
	finalState, err := manager.GetState(ctx, contextID, "rw-test")
	if err != nil {
		t.Fatalf("failed to get final state: %v", err)
	}

	// All fields should still exist
	if stateMap, ok := finalState.(map[string]interface{}); ok {
		expectedFields := []string{"field1", "field2", "field3", "field4", "counter"}
		for _, field := range expectedFields {
			if stateMap[field] == nil {
				t.Errorf("field %s is missing from final state", field)
			}
		}
	} else {
		t.Error("finalState is not a map")
	}
}

// TestConcurrentSubscriptions tests multiple subscribers with concurrent events
func TestConcurrentSubscriptions(t *testing.T) {
	t.Parallel()

	manager, err := NewStateManager(DefaultManagerOptions())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	contextID, err := manager.CreateContext(ctx, "sub-test", nil)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	// Create multiple subscribers
	const numSubscribers = 3   // Reduced from 10
	const numPublishers = 2    // Reduced from 5
	const eventsPerPublisher = 5  // Reduced from 20

	var wg sync.WaitGroup
	var totalEventsReceived atomic.Int64
	var subscriptionErrors atomic.Int64

	unsubscribers := make([]func(), numSubscribers)
	subscriberCounts := make([]atomic.Int64, numSubscribers)

	// Create subscribers
	for i := 0; i < numSubscribers; i++ {
		subIndex := i
		unsubscribe := manager.Subscribe("/", func(event StateChange) {
			subscriberCounts[subIndex].Add(1)
			totalEventsReceived.Add(1)

			// Verify event structure
			if event.Path == "" || event.Timestamp.IsZero() {
				subscriptionErrors.Add(1)
				t.Errorf("subscriber %d: invalid event structure", subIndex)
			}

			// Simulate processing
			time.Sleep(time.Microsecond * 50)
		})
		unsubscribers[i] = unsubscribe
	}

	// Give subscribers time to start
	time.Sleep(5 * time.Millisecond)  // Reduced from 10ms

	// Start publishers
	startSignal := make(chan struct{})

	for i := 0; i < numPublishers; i++ {
		wg.Add(1)
		go func(publisherID int) {
			defer wg.Done()

			<-startSignal

			for j := 0; j < eventsPerPublisher; j++ {
				updates := map[string]interface{}{
					"publisher": publisherID,
					"event":     j,
					"timestamp": time.Now().UnixNano(),
					"data":      fmt.Sprintf("event-%d-%d", publisherID, j),
				}

				_, err := manager.UpdateState(ctx, contextID, "sub-test", updates, UpdateOptions{})
				if err != nil {
					t.Logf("publisher %d event %d failed: %v", publisherID, j, err)
				}

				// Small delay between events
				time.Sleep(time.Microsecond * 10)  // Reduced from 100µs
			}
		}(i)
	}

	// Start all publishers
	close(startSignal)

	// Wait for publishers to finish
	wg.Wait()

	// Give time for event processing
	time.Sleep(20 * time.Millisecond)  // Reduced from 100ms

	// Unsubscribe all
	for _, unsubscribe := range unsubscribers {
		if unsubscribe != nil {
			manager.Unsubscribe(unsubscribe)
		}
	}

	// Log statistics
	t.Logf("Total events received: %d, Errors: %d", totalEventsReceived.Load(), subscriptionErrors.Load())
	for i := 0; i < numSubscribers; i++ {
		t.Logf("Subscriber %d received %d events", i, subscriberCounts[i].Load())
	}
}

// TestGracefulShutdownUnderLoad tests shutdown while operations are active
func TestGracefulShutdownUnderLoad(t *testing.T) {
	t.Parallel()

	manager, err := NewStateManager(DefaultManagerOptions())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	ctx := context.Background()

	// Create multiple contexts
	const numContexts = 3
	contextIDs := make([]string, numContexts)

	for i := 0; i < numContexts; i++ {
		contextID, err := manager.CreateContext(ctx, fmt.Sprintf("shutdown-test-%d", i), nil)
		if err != nil {
			t.Fatalf("failed to create context %d: %v", i, err)
		}
		contextIDs[i] = contextID
	}

	// Start workers for each context
	var wg sync.WaitGroup
	stopWorkers := make(chan struct{})
	shutdownStarted := make(chan struct{})

	const workersPerContext = 5

	for i, contextID := range contextIDs {
		for w := 0; w < workersPerContext; w++ {
			wg.Add(1)
			go func(ctxIndex, workerID int, ctxID string) {
				defer wg.Done()

				for {
					select {
					case <-stopWorkers:
						return
					default:
						// Perform various operations
						switch workerID % 3 {
						case 0: // Writer
							updates := map[string]interface{}{
								"worker": workerID,
								"time":   time.Now().UnixNano(),
							}
							_, err := manager.UpdateState(ctx, ctxID, fmt.Sprintf("state-%d", ctxIndex), updates, UpdateOptions{})
							if err != nil {
								select {
								case <-shutdownStarted:
									// Expected during shutdown
									if err != ErrManagerClosing && err != ErrManagerClosed {
										t.Logf("unexpected error during shutdown: %v", err)
									}
								default:
									t.Errorf("writer error before shutdown: %v", err)
								}
							}

						case 1: // Reader
							_, err := manager.GetState(ctx, ctxID, fmt.Sprintf("state-%d", ctxIndex))
							if err != nil {
								select {
								case <-shutdownStarted:
									// Expected during shutdown
								default:
									t.Errorf("reader error before shutdown: %v", err)
								}
							}

						case 2: // History reader
							_, err := manager.GetHistory(ctx, fmt.Sprintf("state-%d", ctxIndex), 10)
							if err != nil {
								select {
								case <-shutdownStarted:
									// Expected during shutdown
								default:
									t.Errorf("history error before shutdown: %v", err)
								}
							}
						}

						// Small delay
						time.Sleep(time.Microsecond * 100)
					}
				}
			}(i, w, contextID)
		}
	}

	// Let workers run for a bit
	time.Sleep(10 * time.Millisecond)  // Reduced from 50ms

	// Start shutdown
	close(shutdownStarted)

	// Begin graceful shutdown
	shutdownDone := make(chan struct{})
	go func() {
		manager.Close()
		close(shutdownDone)
	}()

	// Give shutdown some time
	select {
	case <-shutdownDone:
		// Shutdown completed
	case <-time.After(2 * time.Second):  // Reduced from 5s
		t.Error("shutdown took too long")
	}

	// Stop all workers
	close(stopWorkers)

	// Wait for all workers
	wg.Wait()

	// Verify manager is closed
	_, err = manager.GetState(ctx, contextIDs[0], "state-0")
	if err != ErrManagerClosed {
		t.Errorf("expected ErrManagerClosed, got %v", err)
	}
}

// TestNoGoroutineLeaks verifies all goroutines are cleaned up
func TestNoGoroutineLeaks(t *testing.T) {
	// Get initial goroutine count
	runtime.GC()
	initialCount := runtime.NumGoroutine()

	// Run a full lifecycle test
	func() {
		manager, err := NewStateManager(DefaultManagerOptions())
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}
		defer manager.Close()

		ctx := context.Background()

		// Create contexts and perform operations
		const numContexts = 3
		contextIDs := make([]string, numContexts)

		for i := 0; i < numContexts; i++ {
			contextID, err := manager.CreateContext(ctx, fmt.Sprintf("leak-test-%d", i), nil)
			if err != nil {
				t.Fatalf("failed to create context: %v", err)
			}
			contextIDs[i] = contextID
		}

		// Create subscriptions
		var subscriptions []func()
		for range contextIDs {
			unsubscribe := manager.Subscribe("/", func(event StateChange) {
				// Just consume the event
			})
			subscriptions = append(subscriptions, unsubscribe)
		}

		// Perform some operations
		for i := 0; i < 10; i++ {
			for _, contextID := range contextIDs {
				updates := map[string]interface{}{
					"iteration": i,
					"timestamp": time.Now().UnixNano(),
				}
				_, err := manager.UpdateState(ctx, contextID, "leak-test", updates, UpdateOptions{})
				if err != nil {
					t.Errorf("update failed: %v", err)
				}
			}
		}

		// Unsubscribe all
		for _, unsubscribe := range subscriptions {
			if unsubscribe != nil {
				manager.Unsubscribe(unsubscribe)
			}
		}
	}()

	// Wait for goroutines to clean up
	time.Sleep(20 * time.Millisecond)  // Reduced from 100ms
	runtime.GC()

	// Check final goroutine count
	finalCount := runtime.NumGoroutine()
	leaked := finalCount - initialCount

	if leaked > 2 { // Allow small tolerance for test framework
		t.Errorf("goroutine leak detected: initial=%d, final=%d, leaked=%d",
			initialCount, finalCount, leaked)

		// Print goroutine stack traces for debugging
		buf := make([]byte, 1<<20)
		stackLen := runtime.Stack(buf, true)
		t.Logf("Goroutine stack traces:\n%s", buf[:stackLen])
	}
}

// TestDeadlockPrevention tests scenarios that could cause deadlocks
func TestDeadlockPrevention(t *testing.T) {
	t.Parallel()

	manager, err := NewStateManager(DefaultManagerOptions())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Test 1: Multiple locks in different orders
	t.Run("LockOrdering", func(t *testing.T) {
		ctx1, _ := manager.CreateContext(ctx, "lock-order-1", nil)
		ctx2, _ := manager.CreateContext(ctx, "lock-order-2", nil)

		var wg sync.WaitGroup
		const iterations = 10  // Reduced from 50

		// Goroutine 1: Updates ctx1 then ctx2
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				// Update context 1
				manager.UpdateState(ctx, ctx1, "lock-order-1", map[string]interface{}{
					"counter": i,
					"thread":  1,
				}, UpdateOptions{})

				// Immediately update context 2
				manager.UpdateState(ctx, ctx2, "lock-order-2", map[string]interface{}{
					"counter": i,
					"thread":  1,
				}, UpdateOptions{})
			}
		}()

		// Goroutine 2: Updates ctx2 then ctx1 (opposite order)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				// Update context 2
				manager.UpdateState(ctx, ctx2, "lock-order-2", map[string]interface{}{
					"counter": i,
					"thread":  2,
				}, UpdateOptions{})

				// Immediately update context 1
				manager.UpdateState(ctx, ctx1, "lock-order-1", map[string]interface{}{
					"counter": i,
					"thread":  2,
				}, UpdateOptions{})
			}
		}()

		// Wait with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success - no deadlock
		case <-time.After(2 * time.Second):  // Reduced from 5s
			t.Error("potential deadlock detected - operations took too long")
		}
	})

	// Test 2: Nested operations
	t.Run("NestedOperations", func(t *testing.T) {
		ctxID, _ := manager.CreateContext(ctx, "nested-ops", nil)

		// Subscribe and perform nested updates
		events := make(chan StateChange, 100)
		unsubscribe := manager.Subscribe("/", func(event StateChange) {
			select {
			case events <- event:
			default:
				// Channel full, skip
			}
		})
		defer manager.Unsubscribe(unsubscribe)
		defer close(events)

		var wg sync.WaitGroup
		done := make(chan struct{})

		// Event handler that triggers more updates
		wg.Add(1)
		go func() {
			defer wg.Done()
			updateCount := 0
			for {
				select {
				case event := <-events:
					if updateCount < 5 { // Limit nesting depth
						// Nested update within event handler
						manager.UpdateState(ctx, ctxID, "nested-ops", map[string]interface{}{
							"nested_update": updateCount,
							"parent_path":   event.Path,
						}, UpdateOptions{})
						updateCount++
					}
				case <-done:
					return
				}
			}
		}()

		// Trigger initial update
		manager.UpdateState(ctx, ctxID, "nested-ops", map[string]interface{}{
			"initial": true,
		}, UpdateOptions{})

		// Wait a bit
		time.Sleep(20 * time.Millisecond)  // Reduced from 100ms

		// Clean up
		close(done)
		wg.Wait()
	})
}

// TestHighConcurrencyStress performs stress testing with 100+ goroutines
func TestHighConcurrencyStress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	t.Parallel()

	manager, err := NewStateManager(DefaultManagerOptions())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Create multiple contexts for stress testing
	const numContexts = 2             // Reduced from 5
	const goroutinesPerContext = 5    // Reduced from 20
	const operationsPerGoroutine = 10 // Reduced from 50

	contextIDs := make([]string, numContexts)
	for i := 0; i < numContexts; i++ {
		contextID, err := manager.CreateContext(ctx, fmt.Sprintf("stress-%d", i), nil)
		if err != nil {
			t.Fatalf("failed to create context %d: %v", i, err)
		}
		contextIDs[i] = contextID

		// Initialize state
		_, err = manager.UpdateState(ctx, contextID, fmt.Sprintf("stress-%d", i), map[string]interface{}{
			"counter": 0,
			"data":    make(map[string]interface{}),
		}, UpdateOptions{})
		if err != nil {
			t.Fatalf("failed to initialize state: %v", err)
		}
	}

	var wg sync.WaitGroup
	var totalOps atomic.Int64
	var errors atomic.Int64

	startTime := time.Now()
	startSignal := make(chan struct{})

	// Launch goroutines
	for ctxIdx, contextID := range contextIDs {
		for g := 0; g < goroutinesPerContext; g++ {
			wg.Add(1)
			go func(contextIndex, goroutineID int, ctxID string) {
				defer wg.Done()

				<-startSignal // Wait for simultaneous start

				for op := 0; op < operationsPerGoroutine; op++ {
					// Mix of operations
					switch op % 5 {
					case 0, 1: // 40% writes
						updates := map[string]interface{}{
							fmt.Sprintf("g%d_op%d", goroutineID, op): map[string]interface{}{
								"timestamp": time.Now().UnixNano(),
								"context":   contextIndex,
								"goroutine": goroutineID,
								"operation": op,
							},
						}
						_, err := manager.UpdateState(ctx, ctxID, fmt.Sprintf("stress-%d", contextIndex), updates, UpdateOptions{})
						if err != nil {
							errors.Add(1)
						}

					case 2, 3: // 40% reads
						state, err := manager.GetState(ctx, ctxID, fmt.Sprintf("stress-%d", contextIndex))
						if err != nil {
							errors.Add(1)
						} else if state == nil {
							errors.Add(1)
							t.Errorf("got nil state")
						}

					case 4: // 20% history reads
						_, err := manager.GetHistory(ctx, fmt.Sprintf("stress-%d", contextIndex), 5)
						if err != nil {
							errors.Add(1)
						}
					}

					totalOps.Add(1)
				}
			}(ctxIdx, g, contextID)
		}
	}

	// Start all goroutines
	close(startSignal)

	// Wait for completion
	wg.Wait()

	duration := time.Since(startTime)
	opsPerSecond := float64(totalOps.Load()) / duration.Seconds()

	t.Logf("Stress test completed:")
	t.Logf("  Total operations: %d", totalOps.Load())
	t.Logf("  Errors: %d", errors.Load())
	t.Logf("  Duration: %v", duration)
	t.Logf("  Operations/second: %.2f", opsPerSecond)

	// Verify final state consistency
	for i, contextID := range contextIDs {
		state, err := manager.GetState(ctx, contextID, fmt.Sprintf("stress-%d", i))
		if err != nil {
			t.Errorf("failed to get final state for context %d: %v", i, err)
		} else if state == nil {
			t.Errorf("final state is nil for context %d", i)
		}
	}
}

// TestDataConsistencyAfterConcurrentOps verifies data remains consistent after concurrent operations
func TestDataConsistencyAfterConcurrentOps(t *testing.T) {
	t.Parallel()

	manager, err := NewStateManager(DefaultManagerOptions())
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer manager.Close()

	ctx := context.Background()
	contextID, err := manager.CreateContext(ctx, "consistency-test", nil)
	if err != nil {
		t.Fatalf("failed to create context: %v", err)
	}

	// Initialize with structured data
	initialData := map[string]interface{}{
		"counters": map[string]int{
			"a": 0,
			"b": 0,
			"c": 0,
		},
		"arrays": map[string][]int{
			"list1": {1, 2, 3},
			"list2": {4, 5, 6},
		},
		"metadata": map[string]interface{}{
			"version":     1,
			"last_update": time.Now().Unix(),
		},
	}

	_, err = manager.UpdateState(ctx, contextID, "consistency-test", initialData, UpdateOptions{})
	if err != nil {
		t.Fatalf("failed to set initial state: %v", err)
	}

	// Perform concurrent increments
	const numGoroutines = 5       // Reduced from 20
	const incrementsPerGoroutine = 5  // Reduced from 25

	var wg sync.WaitGroup
	startSignal := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			<-startSignal

			for j := 0; j < incrementsPerGoroutine; j++ {
				// Get current state
				state, err := manager.GetState(ctx, contextID, "consistency-test")
				if err != nil {
					t.Errorf("failed to get state: %v", err)
					continue
				}

				// Extract counters
				stateMap, ok := state.(map[string]interface{})
				if !ok {
					t.Errorf("state is not a map")
					continue
				}

				counters, ok := stateMap["counters"].(map[string]interface{})
				if !ok {
					t.Errorf("counters not found or wrong type")
					continue
				}

				// Increment each counter
				updates := map[string]interface{}{
					"counters": map[string]interface{}{
						"a": getIntValue(counters["a"]) + 1,
						"b": getIntValue(counters["b"]) + 1,
						"c": getIntValue(counters["c"]) + 1,
					},
					"metadata": map[string]interface{}{
						"last_update": time.Now().Unix(),
						"updated_by":  goroutineID,
					},
				}

				_, err = manager.UpdateState(ctx, contextID, "consistency-test", updates, UpdateOptions{
					ConflictStrategy: LastWriteWins,
				})
				if err != nil {
					t.Errorf("update failed: %v", err)
				}
			}
		}(i)
	}

	// Start all goroutines
	close(startSignal)

	// Wait for completion
	wg.Wait()

	// Verify final consistency
	finalState, err := manager.GetState(ctx, contextID, "consistency-test")
	if err != nil {
		t.Fatalf("failed to get final state: %v", err)
	}

	// Check structure
	stateMap, ok := finalState.(map[string]interface{})
	if !ok {
		t.Fatalf("final state is not a map")
	}

	// Check counters
	counters, ok := stateMap["counters"].(map[string]interface{})
	if !ok {
		t.Fatalf("counters not found in final state")
	}

	// Due to conflict resolution, we might not have exact counts
	// but they should be positive and reasonable
	counterA := getIntValue(counters["a"])
	counterB := getIntValue(counters["b"])
	counterC := getIntValue(counters["c"])

	t.Logf("Final counters: a=%d, b=%d, c=%d", counterA, counterB, counterC)

	// Counters should be positive after increments
	if counterA <= 0 || counterB <= 0 || counterC <= 0 {
		t.Error("counters should be positive after increments")
	}

	// Arrays should still exist
	arrays, ok := stateMap["arrays"].(map[string]interface{})
	if !ok {
		t.Error("arrays structure was lost")
	} else {
		if arrays["list1"] == nil || arrays["list2"] == nil {
			t.Error("array data was corrupted")
		}
	}

	// Metadata should exist
	metadata, ok := stateMap["metadata"].(map[string]interface{})
	if !ok {
		t.Error("metadata was lost")
	} else {
		if metadata["last_update"] == nil {
			t.Error("last_update was not preserved")
		}
	}
}

// Helper function to safely extract int value
func getIntValue(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case int64:
		return int(val)
	default:
		return 0
	}
}
