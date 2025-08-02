package distributed

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoroutineLeakDetection tests for goroutine leaks in distributed systems
func TestGoroutineLeakDetection(t *testing.T) {
	t.Parallel()
	defer testhelper.VerifyNoGoroutineLeaks(t)

	// Use test context with automatic cleanup
	ctx := testhelper.NewTestContextWithTimeout(t, 30*time.Second)

	// Set up cleanup manager
	cleanup := testhelper.NewCleanupManager(t)

	// Record initial goroutine count
	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutines: %d", initialGoroutines)

	// Test multiple components that might leak goroutines
	t.Run("ConsensusManager", func(t *testing.T) {
		testConsensusManagerLeaks(t, ctx, cleanup)
	})

	t.Run("StateSynchronizer", func(t *testing.T) {
		testStateSynchronizerLeaks(t, ctx, cleanup)
	})

	t.Run("DistributedValidator", func(t *testing.T) {
		testDistributedValidatorLeaks(t, ctx, cleanup)
	})

	t.Run("PartitionHandler", func(t *testing.T) {
		testPartitionHandlerLeaks(t, ctx, cleanup)
	})

	// Force garbage collection to ensure cleanup
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	// Check final goroutine count
	finalGoroutines := runtime.NumGoroutine()
	t.Logf("Final goroutines: %d", finalGoroutines)

	// Allow some tolerance for test framework goroutines
	goroutineDiff := finalGoroutines - initialGoroutines
	if goroutineDiff > 5 {
		t.Errorf("Potential goroutine leak detected: started with %d, ended with %d (diff: %d)",
			initialGoroutines, finalGoroutines, goroutineDiff)
	}
}

func testConsensusManagerLeaks(t *testing.T, ctx context.Context, cleanup *testhelper.CleanupManager) {
	config := DefaultConsensusConfig()
	cm, err := NewConsensusManager(config, "test-node")
	require.NoError(t, err)

	// Start and stop multiple times to test for leaks
	for i := 0; i < 3; i++ {
		err = cm.Start(ctx)
		require.NoError(t, err)

		// Perform some operations
		decisions := []*ValidationDecision{
			{NodeID: "node-1", IsValid: true},
			{NodeID: "node-2", IsValid: true},
		}
		result := cm.AggregateDecisions(decisions)
		assert.True(t, result.IsValid)

		// Test lock operations
		lockCtx, lockCancel := context.WithTimeout(ctx, 1*time.Second)
		err = cm.AcquireLock(lockCtx, "test-lock", 500*time.Millisecond)
		assert.NoError(t, err)

		err = cm.ReleaseLock(lockCtx, "test-lock")
		assert.NoError(t, err)
		lockCancel()

		err = cm.Stop()
		require.NoError(t, err)

		// Small delay to allow cleanup
		time.Sleep(50 * time.Millisecond)
	}
}

func testStateSynchronizerLeaks(t *testing.T, ctx context.Context, cleanup *testhelper.CleanupManager) {
	config := DefaultStateSyncConfig()
	config.SyncInterval = 50 * time.Millisecond // Fast for testing
	
	ss, err := NewStateSynchronizer(config, "test-node")
	require.NoError(t, err)

	// Start and stop multiple times to test for leaks
	for i := 0; i < 3; i++ {
		err = ss.Start(ctx)
		require.NoError(t, err)

		// Perform some operations
		err = ss.SetState("key1", "value1")
		assert.NoError(t, err)

		state, exists := ss.GetState("key1")
		assert.True(t, exists)
		assert.Equal(t, "value1", state.Value)

		// Test sync operations
		syncCtx, syncCancel := context.WithTimeout(ctx, 500*time.Millisecond)
		err = ss.SyncState(syncCtx)
		// Don't assert on error as sync might fail due to no nodes
		syncCancel()

		err = ss.Stop()
		require.NoError(t, err)

		// Small delay to allow cleanup
		time.Sleep(50 * time.Millisecond)
	}
}

func testDistributedValidatorLeaks(t *testing.T, ctx context.Context, cleanup *testhelper.CleanupManager) {
	config := TestingDistributedValidatorConfig("test-node")
	config.ValidationTimeout = 500 * time.Millisecond
	config.HeartbeatInterval = 50 * time.Millisecond
	config.PartitionHandler.AllowLocalValidation = true
	config.PartitionHandler.MinNodesForOperation = 1

	localValidator := events.NewEventValidator(&events.ValidationConfig{
		Level:                   events.ValidationPermissive,
		SkipTimestampValidation: true,
		SkipSequenceValidation:  true,
		AllowEmptyIDs:           true,
		AllowUnknownEventTypes:  true,
	})

	dv, err := NewDistributedValidator(config, localValidator)
	require.NoError(t, err)

	// Start and stop multiple times to test for leaks
	for i := 0; i < 3; i++ {
		err = dv.Start(ctx)
		require.NoError(t, err)

		// Perform validation operations
		event := &events.RunStartedEvent{
			BaseEvent: &events.BaseEvent{
				EventType:   events.EventTypeRunStarted,
				TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
			},
			RunIDValue:    "test-run",
			ThreadIDValue: "test-thread",
		}

		validationCtx, validationCancel := context.WithTimeout(ctx, 1*time.Second)
		result := dv.ValidateEvent(validationCtx, event)
		assert.NotNil(t, result)
		validationCancel()

		err = dv.Stop()
		require.NoError(t, err)

		// Small delay to allow cleanup
		time.Sleep(100 * time.Millisecond)
	}
}

func testPartitionHandlerLeaks(t *testing.T, ctx context.Context, cleanup *testhelper.CleanupManager) {
	config := DefaultPartitionHandlerConfig()
	config.HeartbeatTimeout = 100 * time.Millisecond
	config.AutoRecovery = false // Disable auto-recovery for simpler testing

	ph := NewPartitionHandler(config, "test-node")

	// Start and stop multiple times to test for leaks
	for i := 0; i < 3; i++ {
		err := ph.Start(ctx)
		require.NoError(t, err)

		// Perform some operations
		ph.UpdateNodeHealth("other-node", true, 10*time.Millisecond)
		ph.HandleNodeFailure("other-node")

		assert.NotNil(t, ph.GetPartitionHistory())

		err = ph.Stop()
		require.NoError(t, err)

		// Small delay to allow cleanup
		time.Sleep(50 * time.Millisecond)
	}
}

// TestConcurrentOperationsRaceDetection tests for race conditions in concurrent operations
func TestConcurrentOperationsRaceDetection(t *testing.T) {
	t.Parallel()
	defer testhelper.VerifyNoGoroutineLeaks(t)

	// Use test context with automatic cleanup
	ctx := testhelper.NewTestContextWithTimeout(t, 20*time.Second)

	// Set up cleanup manager
	cleanup := testhelper.NewCleanupManager(t)

	config := TestingDistributedValidatorConfig("race-test-node")
	config.ValidationTimeout = 1 * time.Second
	config.HeartbeatInterval = 50 * time.Millisecond
	config.PartitionHandler.AllowLocalValidation = true
	config.PartitionHandler.MinNodesForOperation = 1

	localValidator := events.NewEventValidator(&events.ValidationConfig{
		Level:                   events.ValidationPermissive,
		SkipTimestampValidation: true,
		SkipSequenceValidation:  true,
		AllowEmptyIDs:           true,
		AllowUnknownEventTypes:  true,
	})

	dv, err := NewDistributedValidator(config, localValidator)
	require.NoError(t, err)

	err = dv.Start(ctx)
	require.NoError(t, err)

	// Register cleanup for the distributed validator
	cleanup.Register("distributed-validator", func() {
		if err := dv.Stop(); err != nil {
			t.Logf("Error stopping distributed validator: %v", err)
		}
	})

	// Perform concurrent operations to test for race conditions
	var wg sync.WaitGroup
	concurrency := 10

	// Concurrent validations
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			event := &events.RunStartedEvent{
				BaseEvent: &events.BaseEvent{
					EventType:   events.EventTypeRunStarted,
					TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
				},
				RunIDValue:    "concurrent-run",
				ThreadIDValue: "concurrent-thread",
			}

			validationCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()

			result := dv.ValidateEvent(validationCtx, event)
			assert.NotNil(t, result)
		}(i)
	}

	// Concurrent node operations
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			nodeInfo := &NodeInfo{
				ID:              NodeID("concurrent-node"),
				Address:         "localhost:8080",
				State:           NodeStateActive,
				LastHeartbeat:   time.Now(),
				ValidationCount: 100,
				ErrorRate:       0.01,
				ResponseTimeMs:  50,
				Load:            0.5,
			}

			err := dv.RegisterNode(nodeInfo)
			assert.NoError(t, err)

			_, exists := dv.GetNodeInfo("concurrent-node")
			assert.True(t, exists)

			err = dv.UnregisterNode("concurrent-node")
			assert.NoError(t, err)
		}(i)
	}

	// Wait for all concurrent operations to complete
	if !testhelper.WaitGroupTimeout(t, &wg, 10*time.Second) {
		t.Fatal("Concurrent operations timed out")
	}

	// Verify system is still functional
	event := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeRunStarted,
			TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
		},
		RunIDValue:    "final-test-run",
		ThreadIDValue: "final-test-thread",
	}

	validationCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	result := dv.ValidateEvent(validationCtx, event)
	assert.NotNil(t, result)
	assert.True(t, result.IsValid)
}

// TestWorkerManagerGoroutineCleanup specifically tests worker manager cleanup
func TestWorkerManagerGoroutineCleanup(t *testing.T) {
	t.Parallel()
	defer testhelper.VerifyNoGoroutineLeaks(t)

	// Use test context with automatic cleanup
	ctx := testhelper.NewTestContextWithTimeout(t, 10*time.Second)

	// Create worker manager
	config := DefaultStateSyncConfig()
	ss, err := NewStateSynchronizer(config, "worker-test-node")
	require.NoError(t, err)

	// Start with workers
	err = ss.Start(ctx)
	require.NoError(t, err)

	// Let workers run for a short time
	time.Sleep(200 * time.Millisecond)

	// Stop and verify clean shutdown
	err = ss.Stop()
	require.NoError(t, err)

	// Allow time for cleanup
	time.Sleep(100 * time.Millisecond)

	// Verify the worker manager is properly cleaned up
	assert.False(t, ss.running)
}

// TestAsyncOperationCleanup tests cleanup of async operations
func TestAsyncOperationCleanup(t *testing.T) {
	t.Parallel()
	defer testhelper.VerifyNoGoroutineLeaks(t)

	// Use test context with automatic cleanup
	ctx := testhelper.NewTestContextWithTimeout(t, 15*time.Second)

	// Set up cleanup manager
	cleanup := testhelper.NewCleanupManager(t)

	config := TestingDistributedValidatorConfig("async-test-node")
	config.ValidationTimeout = 1 * time.Second
	config.HeartbeatInterval = 100 * time.Millisecond

	localValidator := events.NewEventValidator(&events.ValidationConfig{
		Level:                   events.ValidationPermissive,
		SkipTimestampValidation: true,
		SkipSequenceValidation:  true,
		AllowEmptyIDs:           true,
		AllowUnknownEventTypes:  true,
	})

	dv, err := NewDistributedValidator(config, localValidator)
	require.NoError(t, err)

	err = dv.Start(ctx)
	require.NoError(t, err)

	// Register cleanup
	cleanup.Register("distributed-validator", func() {
		if err := dv.Stop(); err != nil {
			t.Logf("Error stopping distributed validator: %v", err)
		}
	})

	// Register multiple nodes to trigger async operations
	for i := 1; i <= 5; i++ {
		nodeInfo := &NodeInfo{
			ID:              NodeID(fmt.Sprintf("async-node-%d", i)),
			Address:         fmt.Sprintf("localhost:808%d", i),
			State:           NodeStateActive,
			LastHeartbeat:   time.Now(),
			ValidationCount: 100,
			ErrorRate:       0.01,
			ResponseTimeMs:  50,
			Load:            0.5,
		}
		err := dv.RegisterNode(nodeInfo)
		require.NoError(t, err)
	}

	// Perform validation to trigger async broadcast operations
	event := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeRunStarted,
			TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
		},
		RunIDValue:    "async-test-run",
		ThreadIDValue: "async-test-thread",
	}

	validationCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	result := dv.ValidateEvent(validationCtx, event)
	assert.NotNil(t, result)

	// Allow async operations to process
	time.Sleep(500 * time.Millisecond)

	// Stop should clean up all async operations
	err = dv.Stop()
	require.NoError(t, err)

	// Additional cleanup time
	time.Sleep(200 * time.Millisecond)
}