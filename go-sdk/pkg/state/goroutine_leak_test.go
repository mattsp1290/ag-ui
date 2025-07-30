//go:build integration
// +build integration

package state

import (
	"context"
	"fmt"  
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	
	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/stretchr/testify/require"
)

// TestNoGoroutineLeaks verifies that all goroutines are properly cleaned up
func TestNoGoroutineLeaks(t *testing.T) {
	// Get baseline goroutine count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	// Run a series of operations that previously leaked goroutines
	t.Run("MonitoringSystem", func(t *testing.T) {
		config := MonitoringConfig{
			MetricsInterval:     100 * time.Millisecond,
			HealthCheckInterval: 100 * time.Millisecond,
			AlertNotifiers:      []AlertNotifier{}, // No alert notifiers to avoid nil pointer
			LogFormat:           "console",
		}
		
		ms, err := NewMonitoringSystem(config)
		if err != nil {
			t.Fatalf("Failed to create monitoring system: %v", err)
		}
		
		// Add a health check
		ms.RegisterHealthCheck(&testHealthCheck{})
		
		// Trigger some alerts by recording high latency
		ms.RecordStateOperation("test-op", 10*time.Second, nil)
		
		// Let it run briefly
		time.Sleep(200 * time.Millisecond)
		
		// Shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := ms.Shutdown(ctx); err != nil {
			t.Fatalf("Failed to shutdown monitoring system: %v", err)
		}
	})

	t.Run("StateStore", func(t *testing.T) {
		store := NewStateStore()
		
		// Subscribe to changes
		unsubscribe := store.Subscribe("/test", func(change StateChange) {
			// Handler
		})
		defer unsubscribe()
		
		// Make some changes to trigger notifications
		store.Set("/test/value", "data")
		store.Set("/test/value2", "data2")
		
		// Let notifications run
		time.Sleep(100 * time.Millisecond)
		
		// Close the store
		if err := store.Close(); err != nil {
			t.Fatalf("Failed to close store: %v", err)
		}
	})

	t.Run("StateEventStream", func(t *testing.T) {
		store := NewStateStore()
		defer store.Close()
		
		generator := NewStateEventGenerator(store)
		stream := NewStateEventStream(store, generator, WithStreamInterval(50*time.Millisecond))
		
		// Add handler
		stream.Subscribe(func(event events.Event) error {
			// Process event
			return nil
		})
		
		// Start stream
		stream.Start()
		
		// Make some changes
		store.Set("/test", "value")
		
		// Let it run
		time.Sleep(100 * time.Millisecond)
		
		// Stop stream
		stream.Stop()
	})

	t.Run("AuditManager", func(t *testing.T) {
		logger := &NoOpAuditLogger{}
		am := NewAuditManager(logger)
		
		// Log some events asynchronously
		ctx := context.Background()
		am.LogStateUpdate(ctx, "ctx1", "state1", "user1", "old", "new", AuditResultSuccess, nil)
		am.LogError(ctx, AuditActionStateUpdate, fmt.Errorf("test error"), map[string]interface{}{"test": "data"})
		
		// Let async operations run
		time.Sleep(100 * time.Millisecond)
		
		// Close audit manager
		if err := am.Close(); err != nil {
			t.Fatalf("Failed to close audit manager: %v", err)
		}
	})

	// Wait for goroutines to clean up
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	
	// Check final goroutine count
	finalCount := runtime.NumGoroutine()
	leaked := finalCount - baseline
	
	// Allow for some variance (test framework may create goroutines)
	if leaked > 5 {
		t.Errorf("Goroutine leak detected: baseline=%d, final=%d, leaked=%d", baseline, finalCount, leaked)
		
		// Print stack traces for debugging
		buf := make([]byte, 1<<20)
		stackLen := runtime.Stack(buf, true)
		t.Logf("Goroutine stack traces:\n%s", buf[:stackLen])
	}
}

// testHealthCheck is a simple health check for testing
type testHealthCheck struct{}

func (t *testHealthCheck) Name() string {
	return "test"
}

func (t *testHealthCheck) Check(ctx context.Context) error {
	return nil
}

// TestGoroutineLeakFixes tests that goroutine leaks are fixed
func TestGoroutineLeakFixes(t *testing.T) {
	// Record initial goroutine count
	initialGoroutines := runtime.NumGoroutine()
	
	// Test LazyCache
	t.Run("LazyCache", func(t *testing.T) {
		cache := NewLazyCache(10, time.Minute)
		cache.Set("key", "value")
		
		// Give some time for cleanup goroutine to start
		time.Sleep(100 * time.Millisecond)
		
		// Close the cache
		cache.Close()
		
		// Give time for cleanup goroutine to exit
		time.Sleep(100 * time.Millisecond)
	})
	
	// Test RateLimiter
	t.Run("RateLimiter", func(t *testing.T) {
		limiter := NewRateLimiter(10)
		
		// Give some time for generate goroutine to start
		time.Sleep(100 * time.Millisecond)
		
		// Stop the limiter
		limiter.Stop()
		
		// Give time for generate goroutine to exit
		time.Sleep(100 * time.Millisecond)
	})
	
	// Test StateStore
	t.Run("StateStore", func(t *testing.T) {
		store := NewStateStore()
		
		// Subscribe to trigger cleanup goroutines
		unsubscribe := store.Subscribe("/test", func(change StateChange) {})
		defer unsubscribe()
		
		// Set some data and trigger cleanup
		store.Set("/test", "value")
		
		// Give some time for cleanup goroutines to be created
		time.Sleep(100 * time.Millisecond)
		
		// Close the store
		store.Close()
		
		// Give time for cleanup goroutines to exit
		time.Sleep(100 * time.Millisecond)
	})
	
	// Test StateManager
	t.Run("StateManager", func(t *testing.T) {
		opts := DefaultManagerOptions()
		opts.EnableMetrics = true
		opts.AutoCheckpoint = true
		
		sm, err := NewStateManager(opts)
		if err != nil {
			t.Fatalf("Failed to create StateManager: %v", err)
		}
		
		// Give some time for background goroutines to start
		time.Sleep(200 * time.Millisecond)
		
		// Close the state manager
		err = sm.Close()
		if err != nil {
			t.Fatalf("Failed to close StateManager: %v", err)
		}
		
		// Give time for background goroutines to exit
		time.Sleep(200 * time.Millisecond)
	})
	
	// Test PerformanceOptimizer
	t.Run("PerformanceOptimizer", func(t *testing.T) {
		opts := DefaultPerformanceOptions()
		opts.EnableBatching = true
		opts.EnableLazyLoading = true
		
		po := NewPerformanceOptimizerImpl(opts)
		
		// Give some time for background goroutines to start
		time.Sleep(200 * time.Millisecond)
		
		// Stop the optimizer
		po.Stop()
		
		// Give time for background goroutines to exit
		time.Sleep(200 * time.Millisecond)
	})
	
	// Allow some time for all goroutines to finish
	time.Sleep(500 * time.Millisecond)
	
	// Force garbage collection to clean up any lingering goroutines
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	// Check final goroutine count
	finalGoroutines := runtime.NumGoroutine()
	
	// Allow some leeway for test goroutines and framework goroutines
	leeway := 5
	if finalGoroutines > initialGoroutines+leeway {
		t.Logf("Initial goroutines: %d", initialGoroutines)
		t.Logf("Final goroutines: %d", finalGoroutines)
		t.Errorf("Potential goroutine leak detected: %d goroutines leaked", finalGoroutines-initialGoroutines)
	} else {
		t.Logf("No goroutine leaks detected. Initial: %d, Final: %d", initialGoroutines, finalGoroutines)
	}
}

// TestStateManagerGoroutineCleanup specifically tests StateManager cleanup
func TestStateManagerGoroutineCleanup(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()
	
	for i := 0; i < 5; i++ {
		func() {
			opts := DefaultManagerOptions()
			opts.EnableMetrics = true
			opts.AutoCheckpoint = true
			
			sm, err := NewStateManager(opts)
			if err != nil {
				t.Fatalf("Failed to create StateManager: %v", err)
			}
			
			// Create some state
			ctx := context.Background()
			contextID, err := sm.CreateContext(ctx, "test-state", nil)
			if err != nil {
				t.Fatalf("Failed to create context: %v", err)
			}
			
			// Update some state
			updates := map[string]interface{}{
				"test": "value",
			}
			_, err = sm.UpdateState(ctx, contextID, "test-state", updates, UpdateOptions{})
			if err != nil {
				t.Fatalf("Failed to update state: %v", err)
			}
			
			// Give some time for operations to complete
			time.Sleep(50 * time.Millisecond)
			
			// Close the state manager
			err = sm.Close()
			if err != nil {
				t.Fatalf("Failed to close StateManager: %v", err)
			}
		}()
	}
	
	// Allow time for cleanup
	time.Sleep(300 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	finalGoroutines := runtime.NumGoroutine()
	
	// Be more strict here as we're creating and destroying multiple managers
	leeway := 3
	if finalGoroutines > initialGoroutines+leeway {
		t.Logf("Initial goroutines: %d", initialGoroutines)
		t.Logf("Final goroutines: %d", finalGoroutines)
		t.Errorf("StateManager goroutine leak detected: %d goroutines leaked", finalGoroutines-initialGoroutines)
	} else {
		t.Logf("StateManager cleanup successful. Initial: %d, Final: %d", initialGoroutines, finalGoroutines)
	}
}

// StateManagerGoroutineLeakDetector helps detect goroutine leaks in state manager tests
type StateManagerGoroutineLeakDetector struct {
	t                testing.TB
	startGoroutines  int
	startStack       string
	tolerance        int
	excludePatterns  []string
	maxWaitTime      time.Duration
	checkInterval    time.Duration
}

// NewStateManagerGoroutineLeakDetector creates a new leak detector for state manager
func NewStateManagerGoroutineLeakDetector(t testing.TB) *StateManagerGoroutineLeakDetector {
	detector := &StateManagerGoroutineLeakDetector{
		t:               t,
		tolerance:       10, // Allow more tolerance for state manager due to internal goroutines
		maxWaitTime:     10 * time.Second, // Longer wait for state manager cleanup
		checkInterval:   200 * time.Millisecond,
		excludePatterns: []string{
			"testing.(*T)",
			"runtime.goexit",
			"created by runtime",
			"created by net/http",
			"database/sql",
			"go.uber.org/zap",
			"context.WithCancel",
			"time.NewTicker",
			"sync.(*Pool)",
			"finalizer goroutine",
		},
	}
	detector.snapshot()
	return detector
}

// snapshot captures current goroutine state
func (d *StateManagerGoroutineLeakDetector) snapshot() {
	d.startGoroutines = runtime.NumGoroutine()
	d.startStack = d.getGoroutineStack()
}

// Check verifies no goroutines leaked with enhanced retry logic
func (d *StateManagerGoroutineLeakDetector) Check() {
	// Wait for goroutines to clean up with periodic checks
	timeout := time.After(d.maxWaitTime)
	ticker := time.NewTicker(d.checkInterval)
	defer ticker.Stop()
	
	var endGoroutines int
	var leaked int
	
	for {
		select {
		case <-timeout:
			// Timeout reached, perform final check
			endGoroutines = runtime.NumGoroutine()
			leaked = endGoroutines - d.startGoroutines
			if leaked > d.tolerance {
				d.reportLeak(endGoroutines, leaked)
			}
			return
		case <-ticker.C:
			// Force garbage collection to help clean up
			runtime.GC()
			runtime.GC() // Run GC twice to ensure finalization
			
			endGoroutines = runtime.NumGoroutine()
			leaked = endGoroutines - d.startGoroutines
			
			// If we're within tolerance, we're good
			if leaked <= d.tolerance {
				d.t.Logf("State manager goroutine cleanup successful: started=%d, ended=%d, leaked=%d (within tolerance %d)",
					d.startGoroutines, endGoroutines, leaked, d.tolerance)
				return
			}
		}
	}
}

// reportLeak reports the goroutine leak with detailed information
func (d *StateManagerGoroutineLeakDetector) reportLeak(endGoroutines, leaked int) {
	endStack := d.getGoroutineStack()
	d.t.Errorf("State manager goroutine leak detected: %d goroutines leaked (started with %d, ended with %d)",
		leaked, d.startGoroutines, endGoroutines)
	
	d.t.Logf("Start stack:\n%s", d.startStack)
	d.t.Logf("End stack:\n%s", endStack)
	
	// Try to identify the leaked goroutines
	d.identifyLeakedGoroutines(d.startStack, endStack)
	
	d.t.FailNow()
}

// identifyLeakedGoroutines tries to identify which goroutines are leaked
func (d *StateManagerGoroutineLeakDetector) identifyLeakedGoroutines(startStack, endStack string) {
	startGoroutines := d.parseGoroutineStacks(startStack)
	endGoroutines := d.parseGoroutineStacks(endStack)
	
	d.t.Log("Potentially leaked goroutines:")
	for id, stack := range endGoroutines {
		if _, existed := startGoroutines[id]; !existed {
			excluded := false
			for _, pattern := range d.excludePatterns {
				if strings.Contains(stack, pattern) {
					excluded = true
					break
				}
			}
			if !excluded {
				d.t.Logf("New goroutine %s:\n%s", id, stack)
			}
		}
	}
}

// getGoroutineStack returns current goroutine stack traces
func (d *StateManagerGoroutineLeakDetector) getGoroutineStack() string {
	buf := make([]byte, 1<<20) // 1MB buffer
	n := runtime.Stack(buf, true)
	return string(buf[:n])
}

// parseGoroutineStacks parses stack trace into individual goroutines
func (d *StateManagerGoroutineLeakDetector) parseGoroutineStacks(stack string) map[string]string {
	goroutines := make(map[string]string)
	lines := strings.Split(stack, "\n")
	
	var currentID string
	var currentStack strings.Builder
	
	for _, line := range lines {
		if strings.HasPrefix(line, "goroutine ") {
			if currentID != "" {
				goroutines[currentID] = currentStack.String()
			}
			currentID = strings.TrimSpace(strings.Split(line, "[")[0])
			currentStack.Reset()
			currentStack.WriteString(line + "\n")
		} else if currentID != "" {
			currentStack.WriteString(line + "\n")
		}
	}
	
	if currentID != "" {
		goroutines[currentID] = currentStack.String()
	}
	
	return goroutines
}

// TestStateManagerBasicGoroutineCleanup tests basic state manager goroutine cleanup
func TestStateManagerBasicGoroutineCleanup(t *testing.T) {
	detector := NewStateManagerGoroutineLeakDetector(t)
	defer detector.Check()

	opts := DefaultManagerOptions()
	opts.EnableAudit = false // Disable audit to reduce goroutine complexity
	opts.EnableMetrics = false
	opts.AutoCheckpoint = false

	manager, err := NewStateManager(opts)
	require.NoError(t, err)

	ctx := context.Background()
	
	// Create a context and perform some operations
	contextID, err := manager.CreateContext(ctx, "test-state", nil)
	require.NoError(t, err)

	// Perform some updates
	for i := 0; i < 5; i++ {
		updates := map[string]interface{}{
			"value": i,
			"timestamp": time.Now().UnixNano(),
		}
		_, err = manager.UpdateState(ctx, contextID, "test-state", updates, UpdateOptions{})
		require.NoError(t, err)
	}

	// Get state to verify it works
	_, err = manager.GetState(ctx, contextID, "test-state")
	require.NoError(t, err)

	// Close the manager and verify cleanup
	err = manager.Close()
	require.NoError(t, err)
}

// TestStateManagerMultipleInstancesCleanup tests cleanup of multiple state manager instances
func TestStateManagerMultipleInstancesCleanup(t *testing.T) {
	detector := NewStateManagerGoroutineLeakDetector(t)
	defer detector.Check()

	const numManagers = 3
	managers := make([]*StateManager, numManagers)

	// Create multiple managers
	for i := 0; i < numManagers; i++ {
		opts := DefaultManagerOptions()
		opts.EnableAudit = false
		opts.EnableMetrics = false
		opts.AutoCheckpoint = false
		opts.UpdateQueueSize = 10 // Smaller queue for tests

		manager, err := NewStateManager(opts)
		require.NoError(t, err)
		managers[i] = manager

		ctx := context.Background()
		
		// Create contexts and perform operations on each manager
		for j := 0; j < 3; j++ {
			contextID, err := manager.CreateContext(ctx, fmt.Sprintf("test-state-%d-%d", i, j), nil)
			require.NoError(t, err)

			updates := map[string]interface{}{
				"manager": i,
				"context": j,
				"value":   i*10 + j,
			}
			_, err = manager.UpdateState(ctx, contextID, fmt.Sprintf("test-state-%d-%d", i, j), updates, UpdateOptions{})
			require.NoError(t, err)
		}
	}

	// Close all managers
	for _, manager := range managers {
		err := manager.Close()
		require.NoError(t, err)
	}
}

// TestStateManagerConcurrentOperationsCleanup tests cleanup with concurrent operations
func TestStateManagerConcurrentOperationsCleanup(t *testing.T) {
	detector := NewStateManagerGoroutineLeakDetector(t)
	defer detector.Check()

	opts := DefaultManagerOptions()
	opts.EnableAudit = false
	opts.EnableMetrics = false
	opts.AutoCheckpoint = false

	manager, err := NewStateManager(opts)
	require.NoError(t, err)

	ctx := context.Background()
	var wg sync.WaitGroup

	// Start concurrent operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			contextID, err := manager.CreateContext(ctx, fmt.Sprintf("worker-state-%d", workerID), nil)
			if err != nil {
				return // Manager might be closing
			}

			// Perform operations
			for j := 0; j < 10; j++ {
				updates := map[string]interface{}{
					"worker": workerID,
					"iteration": j,
					"timestamp": time.Now().UnixNano(),
				}
				_, err = manager.UpdateState(ctx, contextID, fmt.Sprintf("worker-state-%d", workerID), updates, UpdateOptions{})
				if err != nil {
					return // Manager might be closing
				}
				
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	// Let operations run for a bit
	time.Sleep(200 * time.Millisecond)

	// Close manager while operations are running
	err = manager.Close()
	require.NoError(t, err)

	// Wait for workers to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good, workers finished
	case <-time.After(5 * time.Second):
		t.Log("Warning: Some workers did not complete in time")
	}
}

// TestStateManagerAuditGoroutineCleanup tests cleanup with audit logging enabled
func TestStateManagerAuditGoroutineCleanup(t *testing.T) {
	detector := NewStateManagerGoroutineLeakDetector(t)
	defer detector.Check()

	opts := DefaultManagerOptions()
	opts.EnableAudit = true
	opts.EnableMetrics = false
	opts.AutoCheckpoint = false

	manager, err := NewStateManager(opts)
	require.NoError(t, err)

	ctx := context.Background()
	
	// Create context and perform operations that will generate audit logs
	contextID, err := manager.CreateContext(ctx, "audit-test-state", map[string]interface{}{
		"test": "audit",
	})
	require.NoError(t, err)

	// Perform multiple updates to generate audit logs
	for i := 0; i < 10; i++ {
		updates := map[string]interface{}{
			"value": i,
			"audit_test": true,
		}
		_, err = manager.UpdateState(ctx, contextID, "audit-test-state", updates, UpdateOptions{})
		require.NoError(t, err)
	}

	// Create checkpoint to generate more audit logs
	_, err = manager.CreateCheckpoint(ctx, "audit-test-state", "test-checkpoint")
	require.NoError(t, err)

	// Give audit logging goroutines time to process
	time.Sleep(500 * time.Millisecond)

	// Close manager and verify cleanup
	err = manager.Close()
	require.NoError(t, err)
}

// VerifyStateManagerNoLeaks runs a test function and verifies no goroutines leak
func VerifyStateManagerNoLeaks(t testing.TB, testFunc func()) {
	detector := NewStateManagerGoroutineLeakDetector(t)
	defer detector.Check()
	
	testFunc()
}