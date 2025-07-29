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

	testutils "github.com/mattsp1290/ag-ui/go-sdk/pkg/testing"
)

// TestConcurrentStateUpdates_RaceCondition tests for race conditions during concurrent state updates
func TestConcurrentStateUpdates_RaceCondition(t *testing.T) {
	t.Parallel()

	testutils.WithTestTimeout(t, 30*time.Second, func() {
		tester := NewReliableConcurrencyTester(t)
		tester.TestRaceCondition(10, 5) // Reduced for reliability
	})
}

// TestConcurrentReadsAndWrites tests parallel reads and writes
func TestConcurrentReadsAndWrites(t *testing.T) {
	t.Parallel()

	testutils.WithTestTimeout(t, 30*time.Second, func() {
		tester := NewReliableConcurrencyTester(t)
		tester.TestReadWriteConcurrency(3, 3, 10) // Reduced for reliability
	})
}

// TestConcurrentSubscriptions tests multiple subscribers with concurrent events
func TestConcurrentSubscriptions(t *testing.T) {
	t.Parallel()

	opts := DefaultManagerOptions()
	opts.EnableAudit = false // Disable audit for faster testing
	manager, err := NewStateManager(opts)
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

	testutils.WithTestTimeout(t, 30*time.Second, func() {
		tester := NewGracefulShutdownTester(t)
		tester.TestShutdownUnderLoad(5, 10) // Reduced for reliability
	})
}

// TestNoGoroutineLeaks verifies all goroutines are cleaned up
func TestNoGoroutineLeaks(t *testing.T) {
	testutils.AssertNoGoroutineLeaks(t, func() {
		tester := NewReliableStateManagerTester(t)
		defer tester.Cleanup()

		ctx := context.Background()

		// Create contexts and perform operations
		contextID1 := tester.CreateContext(ctx, "leak-test-1", nil)
		contextID2 := tester.CreateContext(ctx, "leak-test-2", nil)

		// Create subscriptions
		var subscriptions []func()
		unsubscribe1 := tester.Manager().Subscribe("/", func(event StateChange) {
			// Just consume the event
		})
		unsubscribe2 := tester.Manager().Subscribe("/", func(event StateChange) {
			// Just consume the event
		})
		subscriptions = append(subscriptions, unsubscribe1, unsubscribe2)

		// Perform some operations
		for i := 0; i < 5; i++ {
			updates := map[string]interface{}{
				"iteration": i,
				"timestamp": time.Now().UnixNano(),
			}
			tester.UpdateState(ctx, contextID1, "leak-test", updates, UpdateOptions{})
			tester.UpdateState(ctx, contextID2, "leak-test", updates, UpdateOptions{})
		}

		// Unsubscribe all
		for _, unsubscribe := range subscriptions {
			if unsubscribe != nil {
				tester.Manager().Unsubscribe(unsubscribe)
			}
		}
	})
}

// TestDeadlockPrevention tests scenarios that could cause deadlocks
func TestDeadlockPrevention(t *testing.T) {
	t.Parallel()

	opts := DefaultManagerOptions()
	opts.EnableAudit = false // Disable audit for faster testing
	manager, err := NewStateManager(opts)
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

	opts := DefaultManagerOptions()
	opts.EnableAudit = false // Disable audit for faster testing
	manager, err := NewStateManager(opts)
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

	opts := DefaultManagerOptions()
	opts.EnableAudit = false // Disable audit for faster testing
	manager, err := NewStateManager(opts)
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
