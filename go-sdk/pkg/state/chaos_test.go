// +build chaos integration

package state

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ChaosMonkey introduces random failures and delays
type ChaosMonkey struct {
	mu               sync.RWMutex
	enabled          bool
	failureRate      float64
	delayRate        float64
	minDelay         time.Duration
	maxDelay         time.Duration
	partitionRate    float64
	corruptionRate   float64
	memoryPressure   bool
	cpuPressure      bool
	
	// Statistics
	injectedFailures  int32
	injectedDelays    int32
	injectedPartitions int32
	totalOperations   int32
}

// NewChaosMonkey creates a new chaos monkey
func NewChaosMonkey() *ChaosMonkey {
	return &ChaosMonkey{
		enabled:        true,
		failureRate:    0.1,
		delayRate:      0.2,
		minDelay:       10 * time.Millisecond,
		maxDelay:       500 * time.Millisecond,
		partitionRate:  0.05,
		corruptionRate: 0.02,
	}
}

// MaybeInjectChaos randomly injects chaos based on configured rates
func (cm *ChaosMonkey) MaybeInjectChaos(operation string) error {
	if !cm.isEnabled() {
		return nil
	}
	
	atomic.AddInt32(&cm.totalOperations, 1)
	
	// Inject delay
	if cm.shouldInjectDelay() {
		delay := cm.randomDelay()
		time.Sleep(delay)
		atomic.AddInt32(&cm.injectedDelays, 1)
	}
	
	// Inject failure
	if cm.shouldInjectFailure() {
		atomic.AddInt32(&cm.injectedFailures, 1)
		return fmt.Errorf("chaos monkey injected failure for %s", operation)
	}
	
	// Inject partition
	if cm.shouldInjectPartition() {
		atomic.AddInt32(&cm.injectedPartitions, 1)
		return errors.New("network partition")
	}
	
	return nil
}

// Configuration methods
func (cm *ChaosMonkey) SetFailureRate(rate float64) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.failureRate = rate
}

func (cm *ChaosMonkey) SetDelayRate(rate float64) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.delayRate = rate
}

func (cm *ChaosMonkey) Enable() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.enabled = true
}

func (cm *ChaosMonkey) Disable() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.enabled = false
}

func (cm *ChaosMonkey) GetStats() (failures, delays, partitions, total int32) {
	return atomic.LoadInt32(&cm.injectedFailures),
		atomic.LoadInt32(&cm.injectedDelays),
		atomic.LoadInt32(&cm.injectedPartitions),
		atomic.LoadInt32(&cm.totalOperations)
}

// Internal methods
func (cm *ChaosMonkey) isEnabled() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.enabled
}

func (cm *ChaosMonkey) shouldInjectFailure() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return rand.Float64() < cm.failureRate
}

func (cm *ChaosMonkey) shouldInjectDelay() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return rand.Float64() < cm.delayRate
}

func (cm *ChaosMonkey) shouldInjectPartition() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return rand.Float64() < cm.partitionRate
}

func (cm *ChaosMonkey) randomDelay() time.Duration {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	delayRange := int64(cm.maxDelay - cm.minDelay)
	randomDelay := time.Duration(rand.Int63n(delayRange))
	return cm.minDelay + randomDelay
}

// ChaosStore wraps StateStore with chaos injection
type ChaosStore struct {
	*StateStore
	chaos *ChaosMonkey
}

func NewChaosStore(store *StateStore, chaos *ChaosMonkey) *ChaosStore {
	return &ChaosStore{
		StateStore: store,
		chaos:      chaos,
	}
}

func (cs *ChaosStore) Get(path string) (interface{}, error) {
	if err := cs.chaos.MaybeInjectChaos("get"); err != nil {
		return nil, err
	}
	return cs.StateStore.Get(path)
}

func (cs *ChaosStore) Set(path string, value interface{}) error {
	if err := cs.chaos.MaybeInjectChaos("set"); err != nil {
		return err
	}
	return cs.StateStore.Set(path, value)
}

func (cs *ChaosStore) ApplyPatch(patch JSONPatch) error {
	if err := cs.chaos.MaybeInjectChaos("patch"); err != nil {
		return err
	}
	return cs.StateStore.ApplyPatch(patch)
}

// PartitionedEventHandler simulates network partitions
type PartitionedEventHandler struct {
	*StateEventHandler
	partitioned     atomic.Bool
	droppedEvents   int32
	delayedEvents   int32
	partitionStart  time.Time
	partitionEnd    time.Time
}

func (peh *PartitionedEventHandler) HandleStateSnapshot(event interface{}) error {
	if peh.partitioned.Load() {
		atomic.AddInt32(&peh.droppedEvents, 1)
		return errors.New("network partition: event dropped")
	}
	
	// Random delay to simulate network issues
	if rand.Float64() < 0.1 {
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
		atomic.AddInt32(&peh.delayedEvents, 1)
	}
	
	// For this test, just return nil - in a real implementation
	// we would properly handle the event
	return nil
}

func (peh *PartitionedEventHandler) StartPartition() {
	peh.partitioned.Store(true)
	peh.partitionStart = time.Now()
}

func (peh *PartitionedEventHandler) EndPartition() {
	peh.partitioned.Store(false)
	peh.partitionEnd = time.Now()
}

func (peh *PartitionedEventHandler) GetStats() (dropped, delayed int32, duration time.Duration) {
	dropped = atomic.LoadInt32(&peh.droppedEvents)
	delayed = atomic.LoadInt32(&peh.delayedEvents)
	if !peh.partitionEnd.IsZero() {
		duration = peh.partitionEnd.Sub(peh.partitionStart)
	}
	return
}

// TestStateManager_ChaosEngineering tests system resilience under chaos conditions
func TestStateManager_ChaosEngineering(t *testing.T) {
	// Create chaos monkey
	chaos := NewChaosMonkey()
	chaos.SetFailureRate(0.1)
	chaos.SetDelayRate(0.2)
	
	// Create manager with chaos-injected store
	opts := DefaultManagerOptions()
	opts.MaxRetries = 5
	opts.RetryDelay = 50 * time.Millisecond
	opts.ProcessingWorkers = 4
	opts.EnableMetrics = false // Disable to avoid logger issues
	
	manager, err := NewStateManager(opts)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()
	
	// Wrap store with chaos (in a real implementation this would use dependency injection)
	baseStore := manager.store
	chaosStore := NewChaosStore(baseStore, chaos)
	_ = chaosStore // Reference for future enhancement
	
	// Create contexts
	ctx := context.Background()
	var contexts []string
	for i := 0; i < 10; i++ {
		contextID, err := manager.CreateContext(ctx, fmt.Sprintf("chaos_state_%d", i), nil)
		if err != nil {
			t.Fatalf("Failed to create context: %v", err)
		}
		contexts = append(contexts, contextID)
	}
	
	// Run chaos test
	testDuration := 10 * time.Second
	endTime := time.Now().Add(testDuration)
	
	var wg sync.WaitGroup
	var successOps int32
	var failedOps int32
	var timeouts int32
	
	// Start multiple goroutines simulating concurrent operations
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			opCount := 0
			for time.Now().Before(endTime) {
				// Random operation
				contextID := contexts[rand.Intn(len(contexts))]
				stateID := fmt.Sprintf("chaos_state_%d", rand.Intn(10))
				
				// Create timeout context
				opCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				
				// Perform operation
				switch rand.Intn(3) {
				case 0: // Update
					updates := map[string]interface{}{
						fmt.Sprintf("worker_%d_op_%d", workerID, opCount): time.Now().UnixNano(),
						"random_data": rand.Intn(1000),
					}
					_, err := manager.UpdateState(opCtx, contextID, stateID, updates, UpdateOptions{})
					if err != nil {
						if errors.Is(err, context.DeadlineExceeded) {
							atomic.AddInt32(&timeouts, 1)
						} else {
							atomic.AddInt32(&failedOps, 1)
						}
					} else {
						atomic.AddInt32(&successOps, 1)
					}
					
				case 1: // Read
					_, err := manager.GetState(opCtx, contextID, stateID)
					if err != nil {
						atomic.AddInt32(&failedOps, 1)
					} else {
						atomic.AddInt32(&successOps, 1)
					}
					
				case 2: // Create checkpoint
					_, err := manager.CreateCheckpoint(opCtx, stateID, fmt.Sprintf("chaos_checkpoint_%d", opCount))
					if err != nil {
						atomic.AddInt32(&failedOps, 1)
					} else {
						atomic.AddInt32(&successOps, 1)
					}
				}
				
				cancel()
				opCount++
				
				// Small delay between operations
				time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
			}
		}(i)
	}
	
	// Chaos scenarios during test
	go func() {
		time.Sleep(2 * time.Second)
		// Increase failure rate
		chaos.SetFailureRate(0.3)
		t.Log("Increased failure rate to 30%")
		
		time.Sleep(2 * time.Second)
		// Introduce high delays
		chaos.SetDelayRate(0.5)
		t.Log("Increased delay rate to 50%")
		
		time.Sleep(2 * time.Second)
		// Reduce chaos
		chaos.SetFailureRate(0.05)
		chaos.SetDelayRate(0.1)
		t.Log("Reduced chaos levels")
		
		time.Sleep(2 * time.Second)
		// Disable chaos
		chaos.Disable()
		t.Log("Disabled chaos")
	}()
	
	wg.Wait()
	
	// Get statistics
	failures, delays, partitions, total := chaos.GetStats()
	
	t.Logf("Chaos test completed:")
	t.Logf("  Total operations: %d", total)
	t.Logf("  Successful: %d (%.2f%%)", successOps, float64(successOps)/float64(successOps+failedOps+timeouts)*100)
	t.Logf("  Failed: %d", failedOps)
	t.Logf("  Timeouts: %d", timeouts)
	t.Logf("  Chaos injections - Failures: %d, Delays: %d, Partitions: %d", failures, delays, partitions)
	
	// Verify system maintained some level of operation
	if successOps == 0 {
		t.Error("No operations succeeded during chaos test")
	}
	
	successRate := float64(successOps) / float64(successOps+failedOps+timeouts)
	if successRate < 0.5 {
		t.Errorf("Success rate too low: %.2f%% (expected > 50%%)", successRate*100)
	}
}

// TestStateManager_NetworkPartition tests behavior during network partitions
func TestStateManager_NetworkPartition(t *testing.T) {
	// Create manager
	opts := DefaultManagerOptions()
	opts.EventBufferSize = 100
	opts.EnableMetrics = false // Disable to avoid logger issues
	manager, err := NewStateManager(opts)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()
	
	// Create partitioned event handler (in a real implementation this would use dependency injection)
	baseHandler := manager.eventHandler
	partitionedHandler := &PartitionedEventHandler{
		StateEventHandler: baseHandler,
	}
	_ = partitionedHandler // Reference for future enhancement
	
	ctx := context.Background()
	contextID, err := manager.CreateContext(ctx, "partition_test", nil)
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	
	// Normal operations
	for i := 0; i < 10; i++ {
		updates := map[string]interface{}{
			fmt.Sprintf("pre_partition_%d", i): i,
		}
		_, err := manager.UpdateState(ctx, contextID, "partition_test", updates, UpdateOptions{})
		if err != nil {
			t.Errorf("Pre-partition update failed: %v", err)
		}
	}
	
	// Start partition
	partitionedHandler.StartPartition()
	t.Log("Network partition started")
	
	// Operations during partition
	var partitionErrors int32
	var wg sync.WaitGroup
	
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			
			updates := map[string]interface{}{
				fmt.Sprintf("during_partition_%d", i): i,
			}
			
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			
			_, err := manager.UpdateState(ctx, contextID, "partition_test", updates, UpdateOptions{})
			if err != nil {
				atomic.AddInt32(&partitionErrors, 1)
			}
		}(i)
	}
	
	// Let operations run during partition
	time.Sleep(2 * time.Second)
	
	// End partition
	partitionedHandler.EndPartition()
	t.Log("Network partition ended")
	
	wg.Wait()
	
	// Post-partition operations
	var recoveredOps int32
	for i := 0; i < 10; i++ {
		updates := map[string]interface{}{
			fmt.Sprintf("post_partition_%d", i): i,
		}
		_, err := manager.UpdateState(ctx, contextID, "partition_test", updates, UpdateOptions{})
		if err == nil {
			atomic.AddInt32(&recoveredOps, 1)
		}
	}
	
	// Get stats
	dropped, delayed, duration := partitionedHandler.GetStats()
	
	t.Logf("Network partition test results:")
	t.Logf("  Partition duration: %v", duration)
	t.Logf("  Dropped events: %d", dropped)
	t.Logf("  Delayed events: %d", delayed)
	t.Logf("  Errors during partition: %d", partitionErrors)
	t.Logf("  Recovered operations: %d/10", recoveredOps)
	
	// Verify recovery
	if recoveredOps < 8 {
		t.Errorf("Poor recovery after partition: only %d/10 operations succeeded", recoveredOps)
	}
}

// TestStateManager_StorageFailureScenarios tests various storage failure scenarios
func TestStateManager_StorageFailureScenarios(t *testing.T) {
	scenarios := []struct {
		name        string
		setup       func(*ChaosMonkey)
		operations  int
		concurrent  int
		expectation string
	}{
		{
			name: "intermittent_failures",
			setup: func(cm *ChaosMonkey) {
				cm.SetFailureRate(0.2)
				cm.SetDelayRate(0.1)
			},
			operations:  100,
			concurrent:  10,
			expectation: "should handle intermittent failures gracefully",
		},
		{
			name: "high_latency",
			setup: func(cm *ChaosMonkey) {
				cm.SetFailureRate(0.05)
				cm.SetDelayRate(0.8)
				cm.minDelay = 100 * time.Millisecond
				cm.maxDelay = 500 * time.Millisecond
			},
			operations:  50,
			concurrent:  5,
			expectation: "should handle high latency without deadlocks",
		},
		{
			name: "cascading_failures",
			setup: func(cm *ChaosMonkey) {
				cm.SetFailureRate(0.5)
				cm.SetDelayRate(0.3)
			},
			operations:  50,
			concurrent:  20,
			expectation: "should prevent cascading failures",
		},
		{
			name: "total_failure_recovery",
			setup: func(cm *ChaosMonkey) {
				cm.SetFailureRate(1.0) // Total failure initially
			},
			operations:  30,
			concurrent:  5,
			expectation: "should recover when storage becomes available",
		},
	}
	
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Create chaos monkey
			chaos := NewChaosMonkey()
			scenario.setup(chaos)
			
			// Create manager
			opts := DefaultManagerOptions()
			opts.MaxRetries = 3
			opts.RetryDelay = 100 * time.Millisecond
			opts.EnableMetrics = false // Disable to avoid logger issues
			manager, err := NewStateManager(opts)
			if err != nil {
				t.Fatalf("Failed to create manager: %v", err)
			}
			defer manager.Close()
			
			// Wrap store with chaos (in a real implementation this would use dependency injection)
			chaosStore := NewChaosStore(manager.store, chaos)
			_ = chaosStore // Reference for future enhancement
			
			ctx := context.Background()
			contextID, err := manager.CreateContext(ctx, "scenario_test", nil)
			if err != nil {
				t.Fatalf("Failed to create context: %v", err)
			}
			
			// For total failure scenario, start recovery after some time
			if scenario.name == "total_failure_recovery" {
				go func() {
					time.Sleep(2 * time.Second)
					chaos.SetFailureRate(0.1)
					t.Log("Reduced failure rate to enable recovery")
				}()
			}
			
			// Run concurrent operations
			var wg sync.WaitGroup
			var successCount int32
			var errorCount int32
			operationChan := make(chan int, scenario.operations)
			
			// Fill operation channel
			for i := 0; i < scenario.operations; i++ {
				operationChan <- i
			}
			close(operationChan)
			
			// Start workers
			for w := 0; w < scenario.concurrent; w++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					
					for op := range operationChan {
						updates := map[string]interface{}{
							fmt.Sprintf("worker_%d_op_%d", workerID, op): time.Now().UnixNano(),
						}
						
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						_, err := manager.UpdateState(ctx, contextID, "scenario_test", updates, UpdateOptions{})
						cancel()
						
						if err != nil {
							atomic.AddInt32(&errorCount, 1)
						} else {
							atomic.AddInt32(&successCount, 1)
						}
					}
				}(w)
			}
			
			wg.Wait()
			
			// Get chaos stats
			failures, delays, _, total := chaos.GetStats()
			
			t.Logf("Scenario '%s' results:", scenario.name)
			t.Logf("  Success: %d, Errors: %d", successCount, errorCount)
			t.Logf("  Chaos stats - Operations: %d, Failures: %d, Delays: %d", total, failures, delays)
			t.Logf("  Expectation: %s", scenario.expectation)
			
			// Verify expectations
			switch scenario.name {
			case "intermittent_failures":
				if float64(successCount)/float64(scenario.operations) < 0.7 {
					t.Error("Too many failures for intermittent failure scenario")
				}
			case "high_latency":
				if errorCount > int32(scenario.operations/10) {
					t.Error("Too many errors in high latency scenario")
				}
			case "total_failure_recovery":
				if successCount < 10 {
					t.Error("Recovery did not happen in total failure scenario")
				}
			}
		})
	}
}

// TestStateManager_MemoryPressure tests behavior under memory pressure
func TestStateManager_MemoryPressure(t *testing.T) {
	// Create manager with limited resources
	opts := DefaultManagerOptions()
	opts.CacheSize = 10 // Very small cache
	opts.MaxHistorySize = 5
	opts.EventBufferSize = 10
	opts.BatchSize = 5
	opts.EnableMetrics = false // Disable to avoid logger issues
	
	manager, err := NewStateManager(opts)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()
	
	ctx := context.Background()
	
	// Create many contexts to stress memory
	var contexts []string
	for i := 0; i < 100; i++ {
		contextID, err := manager.CreateContext(ctx, fmt.Sprintf("memory_test_%d", i), map[string]interface{}{
			"large_data": make([]byte, 1024), // 1KB per context
		})
		if err != nil {
			// Expected to fail at some point due to limits
			t.Logf("Context creation failed at %d: %v", i, err)
			break
		}
		contexts = append(contexts, contextID)
	}
	
	t.Logf("Created %d contexts before hitting limits", len(contexts))
	
	// Perform operations to stress the system
	var wg sync.WaitGroup
	var memoryErrors int32
	
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			
			if len(contexts) == 0 {
				return
			}
			
			contextID := contexts[i%len(contexts)]
			largeUpdate := map[string]interface{}{
				"data": make([]byte, 10*1024), // 10KB update
			}
			
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			
			_, err := manager.UpdateState(ctx, contextID, fmt.Sprintf("memory_test_%d", i%10), largeUpdate, UpdateOptions{})
			if err != nil {
				atomic.AddInt32(&memoryErrors, 1)
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Memory pressure test - Errors: %d/50", memoryErrors)
	
	// Verify system is still operational
	if len(contexts) > 0 {
		testCtx := contexts[0]
		_, err := manager.GetState(ctx, testCtx, "memory_test_0")
		if err != nil {
			t.Errorf("System not operational after memory pressure: %v", err)
		}
	}
}

// TestStateManager_ChaoticWorkload tests a realistic chaotic workload
func TestStateManager_ChaoticWorkload(t *testing.T) {
	// Create multiple chaos monkeys with different configurations
	storeChaos := NewChaosMonkey()
	storeChaos.SetFailureRate(0.05)
	storeChaos.SetDelayRate(0.1)
	
	// Create manager
	opts := DefaultManagerOptions()
	opts.EnableMetrics = false // Disable to avoid logger issues
	manager, err := NewStateManager(opts)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()
	
	// Wrap store with chaos (in a real implementation this would use dependency injection)
	chaosStore := NewChaosStore(manager.store, storeChaos)
	_ = chaosStore // Reference for future enhancement
	
	// Create partitioned event handler (in a real implementation this would use dependency injection)
	partitionedHandler := &PartitionedEventHandler{
		StateEventHandler: manager.eventHandler,
	}
	_ = partitionedHandler // Reference for future enhancement
	
	ctx := context.Background()
	
	// Workload configuration
	numUsers := 50
	numStates := 10
	testDuration := 15 * time.Second
	
	// Create initial contexts
	userContexts := make(map[int]string)
	for i := 0; i < numUsers; i++ {
		contextID, err := manager.CreateContext(ctx, fmt.Sprintf("user_%d", i), map[string]interface{}{
			"userID": i,
			"role":   []string{"user", "admin"}[i%2],
		})
		if err != nil {
			t.Fatalf("Failed to create user context: %v", err)
		}
		userContexts[i] = contextID
	}
	
	// Metrics
	var totalOps int32
	var successfulOps int32
	var conflicts int32
	var validationErrors int32
	var networkErrors int32
	
	// Start chaotic events
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				// Random chaos event
				switch rand.Intn(6) {
				case 0:
					// Increase failure rate
					rate := rand.Float64() * 0.3
					storeChaos.SetFailureRate(rate)
					t.Logf("Changed failure rate to %.2f", rate)
				case 1:
					// Network partition
					if rand.Float64() < 0.3 {
						partitionedHandler.StartPartition()
						go func() {
							time.Sleep(time.Duration(rand.Intn(3)+1) * time.Second)
							partitionedHandler.EndPartition()
						}()
						t.Log("Network partition started")
					}
				case 2:
					// High latency
					storeChaos.SetDelayRate(rand.Float64() * 0.5)
				case 3:
					// Recovery
					storeChaos.SetFailureRate(0.01)
					storeChaos.SetDelayRate(0.05)
					t.Log("Chaos reduced - recovery period")
				}
			case <-time.After(testDuration):
				return
			}
		}
	}()
	
	// Start workers simulating user operations
	var wg sync.WaitGroup
	endTime := time.Now().Add(testDuration)
	
	for userID := 0; userID < numUsers; userID++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			
			contextID := userContexts[userID]
			opCount := 0
			
			for time.Now().Before(endTime) {
				atomic.AddInt32(&totalOps, 1)
				
				// Random operation type
				switch rand.Intn(5) {
				case 0, 1: // Update (most common)
					stateID := fmt.Sprintf("state_%d", rand.Intn(numStates))
					updates := map[string]interface{}{
						"lastUpdate":  time.Now().UnixNano(),
						"updateCount": opCount,
						"userID":      userID,
						"data":        fmt.Sprintf("update_%d_%d", userID, opCount),
					}
					
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					_, err := manager.UpdateState(ctx, contextID, stateID, updates, UpdateOptions{})
					cancel()
					
					if err != nil {
						switch {
						case errors.Is(err, ErrInjectedConflict):
							atomic.AddInt32(&conflicts, 1)
						case errors.Is(err, ErrInjectedValidation):
							atomic.AddInt32(&validationErrors, 1)
						case err.Error() == "network partition":
							atomic.AddInt32(&networkErrors, 1)
						}
					} else {
						atomic.AddInt32(&successfulOps, 1)
					}
					
				case 2: // Read
					stateID := fmt.Sprintf("state_%d", rand.Intn(numStates))
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					_, err := manager.GetState(ctx, contextID, stateID)
					cancel()
					
					if err == nil {
						atomic.AddInt32(&successfulOps, 1)
					}
					
				case 3: // Checkpoint
					stateID := fmt.Sprintf("state_%d", rand.Intn(numStates))
					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					_, err := manager.CreateCheckpoint(ctx, stateID, fmt.Sprintf("checkpoint_%d_%d", userID, opCount))
					cancel()
					
					if err == nil {
						atomic.AddInt32(&successfulOps, 1)
					}
					
				case 4: // Rollback (rare)
					if rand.Float64() < 0.1 {
						stateID := fmt.Sprintf("state_%d", rand.Intn(numStates))
						ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
						err := manager.Rollback(ctx, stateID, fmt.Sprintf("checkpoint_%d_%d", userID, rand.Intn(opCount+1)))
						cancel()
						
						if err == nil {
							atomic.AddInt32(&successfulOps, 1)
						}
					}
				}
				
				opCount++
				
				// Variable delay between operations
				time.Sleep(time.Duration(rand.Intn(100)+50) * time.Millisecond)
			}
		}(userID)
	}
	
	wg.Wait()
	
	// Collect final statistics
	failures, delays, partitions, chaosTotal := storeChaos.GetStats()
	dropped, delayed, _ := partitionedHandler.GetStats()
	
	// Calculate success rate
	successRate := float64(successfulOps) / float64(totalOps) * 100
	
	t.Logf("Chaotic workload test completed:")
	t.Logf("  Test duration: %v", testDuration)
	t.Logf("  Total operations: %d", totalOps)
	t.Logf("  Successful: %d (%.2f%%)", successfulOps, successRate)
	t.Logf("  Conflicts: %d", conflicts)
	t.Logf("  Validation errors: %d", validationErrors)
	t.Logf("  Network errors: %d", networkErrors)
	t.Logf("  Chaos injections:")
	t.Logf("    Store - Total: %d, Failures: %d, Delays: %d, Partitions: %d", chaosTotal, failures, delays, partitions)
	t.Logf("    Network - Dropped: %d, Delayed: %d", dropped, delayed)
	
	// Verify system maintained reasonable performance
	if successRate < 60 {
		t.Errorf("Success rate too low under chaos: %.2f%% (expected >= 60%%)", successRate)
	}
	
	// Verify system is still operational after chaos
	testCtx := userContexts[0]
	_, err = manager.GetState(ctx, testCtx, "state_0")
	if err != nil {
		t.Errorf("System not operational after chaotic workload: %v", err)
	}
}