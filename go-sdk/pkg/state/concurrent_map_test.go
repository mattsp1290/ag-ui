package state

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestStateManager_ConcurrentOperations tests for concurrent map writes panic
func TestStateManager_ConcurrentOperations(t *testing.T) {
	t.Parallel()

	// Create multiple state managers concurrently to trigger potential
	// global map concurrent access issues
	const numManagers = 50
	const numOperations = 10

	var wg sync.WaitGroup
	var managerCreationErrors []error
	var operationErrors []error
	var mu sync.Mutex

	startSignal := make(chan struct{})

	for i := 0; i < numManagers; i++ {
		wg.Add(1)
		go func(managerID int) {
			defer wg.Done()

			<-startSignal // Wait for all goroutines to be ready

			// Create state manager with audit disabled for faster testing
			opts := DefaultManagerOptions()
			opts.EnableAudit = false
			manager, err := NewStateManager(opts)
			if err != nil {
				mu.Lock()
				managerCreationErrors = append(managerCreationErrors, err)
				mu.Unlock()
				return
			}
			defer manager.Close()

			ctx := context.Background()

			// Perform operations to stress test the manager
			for j := 0; j < numOperations; j++ {
				// Create context
				contextID, err := manager.CreateContext(ctx, "test-state", map[string]interface{}{
					"manager":   managerID,
					"operation": j,
				})
				if err != nil {
					mu.Lock()
					operationErrors = append(operationErrors, err)
					mu.Unlock()
					continue
				}

				// Update state
				updates := map[string]interface{}{
					"data": map[string]interface{}{
						"value":   managerID*1000 + j,
						"time":    time.Now().UnixNano(),
						"manager": managerID,
					},
				}
				_, err = manager.UpdateState(ctx, contextID, "test-state", updates, UpdateOptions{})
				if err != nil {
					mu.Lock()
					operationErrors = append(operationErrors, err)
					mu.Unlock()
				}

				// Get state
				_, err = manager.GetState(ctx, contextID, "test-state")
				if err != nil {
					mu.Lock()
					operationErrors = append(operationErrors, err)
					mu.Unlock()
				}

				// Get history
				_, err = manager.GetHistory(ctx, "test-state", 5)
				if err != nil {
					mu.Lock()
					operationErrors = append(operationErrors, err)
					mu.Unlock()
				}
			}
		}(i)
	}

	// Start all goroutines simultaneously to maximize concurrent access
	close(startSignal)

	// Wait for completion with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(60 * time.Second):
		t.Fatal("test timed out - possible deadlock or slow operations")
	}

	// Check for errors
	if len(managerCreationErrors) > 0 {
		t.Logf("Manager creation errors: %d", len(managerCreationErrors))
		for i, err := range managerCreationErrors {
			if i < 5 { // Log first 5 errors
				t.Logf("Manager creation error %d: %v", i, err)
			}
		}
	}

	if len(operationErrors) > 0 {
		t.Logf("Operation errors: %d", len(operationErrors))
		for i, err := range operationErrors {
			if i < 5 { // Log first 5 errors
				t.Logf("Operation error %d: %v", i, err)
			}
		}
	}

	t.Logf("Test completed with %d managers, %d operations each. Manager errors: %d, Operation errors: %d",
		numManagers, numOperations, len(managerCreationErrors), len(operationErrors))
}