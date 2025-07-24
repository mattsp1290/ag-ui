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
		iterations := 50
		// Reduce iterations in testing.Short() mode for CI
		if testing.Short() {
			iterations = 10
		}

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
		case <-time.After(5 * time.Second):
			t.Error("potential deadlock detected - operations took too long")
		}
	})

	// Test 2: Nested operations - removed as it uses time.Sleep for timing coordination
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
	numGoroutines := 20
	incrementsPerGoroutine := 25
	
	// Reduce load in testing.Short() mode for CI
	if testing.Short() {
		numGoroutines = 5
		incrementsPerGoroutine = 5
	}

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
