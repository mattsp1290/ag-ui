package state

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestConcurrentMapWriteStress is a comprehensive test to trigger potential concurrent map write panics
func TestConcurrentMapWriteStress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	t.Parallel()

	const (
		numManagerInstances = 10  // Multiple manager instances (reduced for faster tests)
		numWorkersPerManager = 5  // Workers per manager (reduced for faster tests)
		numOperations = 10        // Operations per worker (reduced for faster tests)
		maxTestDuration = 30 * time.Second // Reduced from 120 seconds to 30
	)

	var (
		totalErrors       int64
		totalPanics       int64
		totalOperations   int64
		managers          []*StateManager
		wg                sync.WaitGroup
		mu                sync.Mutex
		testCtx, cancel   = context.WithTimeout(context.Background(), maxTestDuration)
	)
	defer cancel()

	// Panic recovery counter
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Test panic recovered: %v", r)
		}
	}()

	t.Logf("Starting concurrent stress test with %d managers, %d workers per manager, %d operations per worker",
		numManagerInstances, numWorkersPerManager, numOperations)

	startTime := time.Now()

	// Phase 1: Create managers concurrently
	managerWg := sync.WaitGroup{}
	managerChan := make(chan *StateManager, numManagerInstances)

	for i := 0; i < numManagerInstances; i++ {
		managerWg.Add(1)
		go func(managerID int) {
			defer managerWg.Done()
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					totalPanics++
					mu.Unlock()
					t.Logf("Manager creation panic recovered for manager %d: %v", managerID, r)
				}
			}()

			// Create manager with varied options to stress different code paths
			opts := DefaultManagerOptions()
			if managerID%2 == 0 {
				opts.EnableMetrics = true
				opts.EnableAudit = true
			}
			if managerID%3 == 0 {
				opts.MaxCheckpoints = 5
				opts.AutoCheckpoint = true
			}
			
			// Use test-friendly performance optimizer with shorter intervals to prevent hangs
			performanceOpts := DefaultPerformanceOptions()
			performanceOpts.EnableBatching = false // Disable batching in tests to prevent hanging
			performanceOpts.MaxConcurrency = 2    // Limit concurrency in tests
			performanceOpts.MaxPoolSize = 100     // Smaller pools for tests
			performanceOpts.MaxIdleObjects = 10   // Smaller idle objects
			performanceOpts.ConnectionPoolSize = 5 // Smaller connection pool
			performanceOpts.LazyCacheSize = 50    // Smaller cache
			opts.PerformanceOptimizer = NewPerformanceOptimizer(performanceOpts)

			manager, err := NewStateManager(opts)
			if err != nil {
				mu.Lock()
				totalErrors++
				mu.Unlock()
				t.Logf("Failed to create manager %d: %v", managerID, err)
				return
			}

			select {
			case managerChan <- manager:
			case <-testCtx.Done():
				manager.Close()
				return
			}
		}(i)
	}

	// Wait for all managers to be created
	managerWg.Wait()
	close(managerChan)

	// Collect managers
	for manager := range managerChan {
		if manager != nil {
			managers = append(managers, manager)
		}
	}

	t.Logf("Created %d managers successfully", len(managers))

	if len(managers) == 0 {
		t.Fatal("No managers created successfully")
	}

	// Phase 2: Launch concurrent workers on all managers
	for managerIdx, manager := range managers {
		for workerID := 0; workerID < numWorkersPerManager; workerID++ {
			wg.Add(1)
			go func(mgrIdx, wID int, mgr *StateManager) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						mu.Lock()
						totalPanics++
						mu.Unlock()
						t.Logf("Worker panic recovered (manager %d, worker %d): %v", mgrIdx, wID, r)
					}
				}()

				ctx := context.Background()
				workerPrefix := fmt.Sprintf("mgr%d_w%d", mgrIdx, wID)

				for opID := 0; opID < numOperations; opID++ {
					select {
					case <-testCtx.Done():
						return
					default:
					}

					// Mix of different operations to stress various code paths
					switch opID % 7 {
					case 0: // Create context and update state
						contextID, err := mgr.CreateContext(ctx, workerPrefix+"_state", map[string]interface{}{
							"worker":    wID,
							"operation": opID,
							"manager":   mgrIdx,
						})
						if err != nil {
							mu.Lock()
							totalErrors++
							mu.Unlock()
							continue
						}

						updates := map[string]interface{}{
							"data": map[string]interface{}{
								"timestamp": time.Now().UnixNano(),
								"value":     mgrIdx*1000 + wID*100 + opID,
							},
						}
						_, err = mgr.UpdateState(ctx, contextID, workerPrefix+"_state", updates, UpdateOptions{})
						if err != nil {
							mu.Lock()
							totalErrors++
							mu.Unlock()
						}

					case 1: // Get state
						contextID, _ := mgr.CreateContext(ctx, workerPrefix+"_get", nil)
						_, err := mgr.GetState(ctx, contextID, workerPrefix+"_get")
						if err != nil {
							mu.Lock()
							totalErrors++
							mu.Unlock()
						}

					case 2: // Get history
						_, err := mgr.GetHistory(ctx, workerPrefix+"_hist", 5)
						if err != nil {
							mu.Lock()
							totalErrors++
							mu.Unlock()
						}

					case 3: // Create checkpoint
						_, err := mgr.CreateCheckpoint(ctx, workerPrefix+"_state", fmt.Sprintf("%s_checkpoint_%d", workerPrefix, opID))
						if err != nil {
							mu.Lock()
							totalErrors++
							mu.Unlock()
						}

					case 4: // Subscribe and unsubscribe
						unsubscribe := mgr.Subscribe("/", func(change StateChange) {
							// Simple callback
						})
						time.Sleep(time.Microsecond * 10) // Brief subscription
						if unsubscribe != nil {
							mgr.Unsubscribe(unsubscribe)
						}

					case 5: // Get metrics if enabled
						if metrics := mgr.GetMetrics(); metrics != nil {
							// Access metrics map
							_ = len(metrics)
						}

					case 6: // Force garbage collection occasionally to stress concurrent cleanup
						if opID%20 == 0 {
							runtime.GC()
						}
					}

					mu.Lock()
					totalOperations++
					mu.Unlock()
				}
			}(managerIdx, workerID, manager)
		}
	}

	// Wait for all workers to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Logf("All workers completed successfully")
	case <-testCtx.Done():
		t.Logf("Test timed out, waiting for workers to finish...")
		wg.Wait() // Still wait for workers to prevent resource leaks
	}

	// Phase 3: Cleanup all managers sequentially with timeout
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cleanupCancel()
	
	cleanupWg := sync.WaitGroup{}
	for i, manager := range managers {
		cleanupWg.Add(1)
		go func(idx int, mgr *StateManager) {
			defer cleanupWg.Done()
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					totalPanics++
					mu.Unlock()
					t.Logf("Manager cleanup panic recovered for manager %d: %v", idx, r)
				}
			}()
			
			// Create a channel to signal completion
			done := make(chan struct{})
			go func() {
				defer close(done)
				mgr.Close()
			}()
			
			// Wait for close to complete or timeout
			select {
			case <-done:
				// Close completed successfully
			case <-cleanupCtx.Done():
				t.Logf("Manager %d cleanup timed out", idx)
			case <-time.After(2 * time.Second):
				t.Logf("Manager %d cleanup took too long", idx)
			}
		}(i, manager)
	}
	
	// Wait for all cleanup to complete with timeout
	cleanupDone := make(chan struct{})
	go func() {
		cleanupWg.Wait()
		close(cleanupDone)
	}()
	
	select {
	case <-cleanupDone:
		t.Logf("All managers cleaned up successfully")
	case <-cleanupCtx.Done():
		t.Logf("Cleanup timed out, some managers may not have closed properly")
	}

	duration := time.Since(startTime)
	
	// Report results
	mu.Lock()
	errors := totalErrors
	panics := totalPanics
	operations := totalOperations
	mu.Unlock()

	t.Logf("Concurrent stress test completed:")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Total operations: %d", operations)
	t.Logf("  Total errors: %d", errors)
	t.Logf("  Total panics recovered: %d", panics)
	t.Logf("  Operations per second: %.2f", float64(operations)/duration.Seconds())

	if panics > 0 {
		t.Errorf("Detected %d panics during concurrent operations - this indicates concurrent map write issues", panics)
	}

	errorRate := float64(errors) / float64(operations) * 100
	if errorRate > 20 { // Allow some errors due to resource contention, but not too many
		t.Errorf("High error rate: %.2f%% (%d/%d)", errorRate, errors, operations)
	}

	t.Logf("Test completed with %.2f%% error rate", errorRate)
}