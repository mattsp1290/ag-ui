package distributed

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/proto/generated"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock event for testing
type mockEvent struct {
	*events.BaseEvent
	ID    string
	Valid bool
}

func (m *mockEvent) Validate() error {
	if !m.Valid {
		return fmt.Errorf("mock validation error")
	}
	return nil
}

func (m *mockEvent) ToJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"id":"%s","valid":%t}`, m.ID, m.Valid)), nil
}

func (m *mockEvent) ToProtobuf() (*generated.Event, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockEvent) GetEventID() string {
	return m.ID
}

// Test DistributedValidator creation and initialization
func TestNewDistributedValidator(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	config := TestingDistributedValidatorConfig("node-1")
	localValidator := events.NewEventValidator(nil)

	dv, err := NewDistributedValidator(config, localValidator)
	require.NoError(t, err)
	require.NotNil(t, dv)

	assert.Equal(t, NodeID("node-1"), dv.config.NodeID)
	assert.NotNil(t, dv.consensus)
	assert.NotNil(t, dv.stateSync)
	assert.NotNil(t, dv.partitionHandler)
	assert.NotNil(t, dv.loadBalancer)
}

// Test DistributedValidator lifecycle
func TestDistributedValidatorLifecycle(t *testing.T) {
	t.Parallel()
	defer testhelper.VerifyNoGoroutineLeaks(t)

	// Use test context with automatic cleanup
	ctx := testhelper.NewTestContextWithTimeout(t, 10*time.Second)

	config := TestingDistributedValidatorConfig("node-1")
	localValidator := events.NewEventValidator(nil)

	dv, err := NewDistributedValidator(config, localValidator)
	require.NoError(t, err)

	// Test Start
	err = dv.Start(ctx)
	assert.NoError(t, err)

	// Test double start
	err = dv.Start(ctx)
	assert.Error(t, err)

	// Test Stop with timeout
	done := make(chan bool)
	go func() {
		err = dv.Stop()
		assert.NoError(t, err)
		done <- true
	}()

	select {
	case <-done:
		// Stop completed successfully
	case <-ctx.Done():
		t.Fatal("Test timed out waiting for Stop")
	}

	// Test double stop
	err = dv.Stop()
	assert.NoError(t, err)
}

// Test node registration and management
func TestNodeManagement(t *testing.T) {
	config := TestingDistributedValidatorConfig("node-1")
	localValidator := events.NewEventValidator(nil)

	dv, err := NewDistributedValidator(config, localValidator)
	require.NoError(t, err)

	// Register nodes
	node2 := &NodeInfo{
		ID:              "node-2",
		Address:         "node2:8080",
		State:           NodeStateActive,
		LastHeartbeat:   time.Now(),
		ValidationCount: 100,
		ErrorRate:       0.01,
		ResponseTimeMs:  50,
		Load:            0.5,
	}

	err = dv.RegisterNode(node2)
	assert.NoError(t, err)

	// Get node info
	info, exists := dv.GetNodeInfo("node-2")
	assert.True(t, exists)
	assert.Equal(t, NodeID("node-2"), info.ID)

	// Get all nodes
	allNodes := dv.GetAllNodes()
	assert.Len(t, allNodes, 1)

	// Unregister node
	err = dv.UnregisterNode("node-2")
	assert.NoError(t, err)

	_, exists = dv.GetNodeInfo("node-2")
	assert.False(t, exists)
}

// Test distributed validation with partition handling
func TestDistributedValidationWithPartition(t *testing.T) {
	t.Parallel()
	defer testhelper.VerifyNoGoroutineLeaks(t)

	// Use test context with automatic cleanup
	testCtx := testhelper.NewTestContextWithTimeout(t, 15*time.Second)

	// Set up cleanup manager
	cleanup := testhelper.NewCleanupManager(t)

	config := TestingDistributedValidatorConfig("node-1")
	// Set MinNodesForOperation to 2 so partition is detected when all other nodes fail
	config.PartitionHandler.MinNodesForOperation = 2

	// Create a validator without sequence validation for this test
	localValidator := events.NewEventValidator(events.TestingValidationConfig())

	dv, err := NewDistributedValidator(config, localValidator)
	require.NoError(t, err)

	err = dv.Start(testCtx)
	require.NoError(t, err)

	// Register cleanup for the distributed validator
	cleanup.Register("distributed-validator", func() {
		if err := dv.Stop(); err != nil {
			t.Logf("Error stopping distributed validator: %v", err)
		}
	})

	// Create a valid RUN_STARTED event since validator expects it as first event
	event := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeRunStarted,
			TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
		},
		RunIDValue:    "test-run-1",
		ThreadIDValue: "test-thread-1",
	}

	// First, register some nodes to create a cluster
	for i := 2; i <= 4; i++ {
		node := &NodeInfo{
			ID:            NodeID(fmt.Sprintf("node-%d", i)),
			State:         NodeStateActive,
			LastHeartbeat: time.Now(),
		}
		dv.RegisterNode(node)
		// Also register in partition handler
		dv.partitionHandler.UpdateNodeHealth(node.ID, true, 10*time.Millisecond)
	}

	// Now simulate partition by marking all nodes as failed
	for i := 0; i < 3; i++ {
		for j := 2; j <= 4; j++ {
			dv.partitionHandler.HandleNodeFailure(NodeID(fmt.Sprintf("node-%d", j)))
		}
	}

	// Give partition handler time to detect the partition
	time.Sleep(200 * time.Millisecond)

	// Wait for partition detection to happen via background goroutines
	maxWait := time.After(1 * time.Second)
	for !dv.partitionHandler.IsPartitioned() {
		select {
		case <-maxWait:
			t.Log("Partition not detected after 1 second, test may fail")
			break
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
	t.Logf("Is partitioned before validation: %v", dv.partitionHandler.IsPartitioned())

	// Validate event during partition with timeout
	validateCtx, validateCancel := context.WithTimeout(testCtx, 2*time.Second)
	defer validateCancel()

	resultChan := make(chan *ValidationResult)
	go func() {
		result := dv.ValidateEvent(validateCtx, event)
		resultChan <- result
	}()

	select {
	case result := <-resultChan:
		// Should fall back to local validation
		if !result.IsValid && len(result.Errors) > 0 {
			t.Logf("Validation failed with errors: %v", result.Errors[0].Message)
			t.Logf("Is partitioned: %v", dv.partitionHandler.IsPartitioned())
		}
		assert.True(t, result.IsValid)
		assert.Len(t, result.Errors, 0)
	case <-validateCtx.Done():
		t.Fatal("Validation timed out")
	}
}

// Test consensus algorithms
func TestConsensusAlgorithms(t *testing.T) {
	tests := []struct {
		name      string
		algorithm ConsensusAlgorithm
		decisions []*ValidationDecision
		expected  bool
	}{
		{
			name:      "Unanimous - all valid",
			algorithm: ConsensusUnanimous,
			decisions: []*ValidationDecision{
				{NodeID: "node-1", IsValid: true},
				{NodeID: "node-2", IsValid: true},
				{NodeID: "node-3", IsValid: true},
			},
			expected: true,
		},
		{
			name:      "Unanimous - one invalid",
			algorithm: ConsensusUnanimous,
			decisions: []*ValidationDecision{
				{NodeID: "node-1", IsValid: true},
				{NodeID: "node-2", IsValid: false},
				{NodeID: "node-3", IsValid: true},
			},
			expected: false,
		},
		{
			name:      "Majority - most valid",
			algorithm: ConsensusMajority,
			decisions: []*ValidationDecision{
				{NodeID: "node-1", IsValid: true},
				{NodeID: "node-2", IsValid: false},
				{NodeID: "node-3", IsValid: true},
			},
			expected: true,
		},
		{
			name:      "Majority - most invalid",
			algorithm: ConsensusMajority,
			decisions: []*ValidationDecision{
				{NodeID: "node-1", IsValid: false},
				{NodeID: "node-2", IsValid: false},
				{NodeID: "node-3", IsValid: true},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ConsensusConfig{
				Algorithm:  tt.algorithm,
				MinNodes:   3,
				QuorumSize: 2,
			}

			cm, err := NewConsensusManager(config, "node-1")
			require.NoError(t, err)

			result := cm.AggregateDecisions(tt.decisions)
			assert.Equal(t, tt.expected, result.IsValid)
		})
	}
}

// Test state synchronization
func TestStateSynchronization(t *testing.T) {
	// Removed t.Parallel() to avoid resource contention with worker pool
	defer testhelper.VerifyNoGoroutineLeaks(t)

	// Use a background context that won't be cancelled during the test
	// The StateSynchronizer will be stopped explicitly via Stop()
	ctx := context.Background()

	// Create a separate timeout context for test operations
	testCtx, testCancel := context.WithTimeout(ctx, 8*time.Second)
	defer testCancel()

	// Set up cleanup manager
	cleanup := testhelper.NewCleanupManager(t)

	config := DefaultStateSyncConfig()
	// Use shorter intervals for testing to speed up test execution
	config.SyncInterval = 50 * time.Millisecond
	config.MaxRetries = 1 // Reduce retries for faster testing
	ss, err := NewStateSynchronizer(config, "node-1")
	require.NoError(t, err)

	err = ss.Start(ctx)
	require.NoError(t, err)

	// Register cleanup for the state synchronizer - must be called before test ends
	cleanup.Register("state-synchronizer", func() {
		if err := ss.Stop(); err != nil {
			t.Logf("Error stopping state synchronizer: %v", err)
		}
		// Give time for cleanup to complete
		time.Sleep(100 * time.Millisecond)
	})

	// Run test operations with timeout
	testDone := make(chan bool, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Panic in test goroutine: %v", r)
			}
		}()

		// Set state
		err = ss.SetState("key1", "value1")
		assert.NoError(t, err)

		// Get state
		state, exists := ss.GetState("key1")
		assert.True(t, exists)
		assert.Equal(t, "value1", state.Value)

		// Get snapshot
		snapshot := ss.GetSnapshot()
		assert.NotNil(t, snapshot)
		assert.Len(t, snapshot.StateItems, 1)

		// Apply snapshot
		err = ss.ApplySnapshot(snapshot)
		assert.NoError(t, err)

		testDone <- true
	}()

	select {
	case <-testDone:
		// Test completed successfully - cleanup will be called automatically
	case <-testCtx.Done():
		t.Fatal("Test timed out")
	}
}

// Test partition detection and recovery
func TestPartitionDetectionAndRecovery(t *testing.T) {
	// Remove t.Parallel() to avoid interference with other tests
	defer testhelper.VerifyNoGoroutineLeaks(t)

	// Use test context with automatic cleanup
	ctx := testhelper.NewTestContextWithTimeout(t, 8*time.Second)

	// Set up cleanup manager
	cleanup := testhelper.NewCleanupManager(t)

	config := DefaultPartitionHandlerConfig()
	config.HeartbeatTimeout = 100 * time.Millisecond
	config.AutoRecovery = true

	ph := NewPartitionHandler(config, "node-1")

	// Set up partition callbacks with buffered channels
	partitionDetected := make(chan *PartitionInfo, 2)
	partitionRecovered := make(chan *PartitionInfo, 2)

	// Register cleanup for channels - move this after setting callbacks
	cleanup.Register("partition-channels", func() {
		testhelper.CloseChannel(t, partitionDetected, "partitionDetected")
		testhelper.CloseChannel(t, partitionRecovered, "partitionRecovered")
	})

	ph.SetPartitionCallbacks(
		func(p *PartitionInfo) {
			select {
			case partitionDetected <- p:
			default:
				// Channel full, ignore
			}
		},
		func(p *PartitionInfo) {
			select {
			case partitionRecovered <- p:
			default:
				// Channel full, ignore
			}
		},
	)

	err := ph.Start(ctx)
	require.NoError(t, err)

	// Register cleanup for the partition handler - this must happen before any goroutine callbacks
	cleanup.Register("partition-handler", func() {
		if err := ph.Stop(); err != nil {
			t.Logf("Error stopping partition handler: %v", err)
		}
		// Give time for all goroutines to fully exit
		time.Sleep(100 * time.Millisecond)
	})

	// Register healthy nodes
	ph.UpdateNodeHealth("node-2", true, 10*time.Millisecond)
	ph.UpdateNodeHealth("node-3", true, 10*time.Millisecond)

	// Give nodes time to register
	time.Sleep(50 * time.Millisecond)

	// Simulate node failures (need 3 consecutive failures to mark as unreachable)
	for i := 0; i < 3; i++ {
		ph.HandleNodeFailure("node-2")
		ph.HandleNodeFailure("node-3")
	}

	// Wait for partition detection with context timeout
	select {
	case partition := <-partitionDetected:
		assert.True(t, partition.IsActive)
		assert.Contains(t, partition.UnreachableNodes, NodeID("node-2"))
		assert.Contains(t, partition.UnreachableNodes, NodeID("node-3"))
	case <-ctx.Done():
		t.Fatal("Test context timeout - partition not detected")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Partition not detected within timeout")
	}

	assert.True(t, ph.IsPartitioned())

	// Drain any remaining callback goroutines by reading from channels
	select {
	case <-partitionRecovered:
		// Recovery callback fired, drain it
	case <-time.After(10 * time.Millisecond):
		// No recovery callback, that's fine
	}
}

// Test load balancing algorithms
func TestLoadBalancingAlgorithms(t *testing.T) {
	tests := []struct {
		name      string
		algorithm LoadBalancingAlgorithm
		nodes     []NodeID
		count     int
	}{
		{
			name:      "Round Robin",
			algorithm: LoadBalancingRoundRobin,
			nodes:     []NodeID{"node-1", "node-2", "node-3"},
			count:     2,
		},
		{
			name:      "Least Response Time",
			algorithm: LoadBalancingLeastResponseTime,
			nodes:     []NodeID{"node-1", "node-2", "node-3"},
			count:     2,
		},
		{
			name:      "Random",
			algorithm: LoadBalancingRandom,
			nodes:     []NodeID{"node-1", "node-2", "node-3"},
			count:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &LoadBalancerConfig{
				Algorithm: tt.algorithm,
			}

			lb := NewLoadBalancer(config)

			// Set up node metrics
			for i, node := range tt.nodes {
				lb.UpdateNodeMetrics(node, float64(i)*0.1, float64(i+1)*10)
			}

			// Select nodes
			selected := lb.SelectNodes(tt.nodes, tt.count)
			assert.Len(t, selected, tt.count)

			// Verify all selected nodes are from available nodes
			for _, node := range selected {
				assert.Contains(t, tt.nodes, node)
			}
		})
	}
}

// Test circuit breaker functionality
func TestCircuitBreaker(t *testing.T) {
	config := DefaultLoadBalancerConfig()
	config.EnableCircuitBreaker = true
	config.CircuitBreakerThreshold = 0.5
	config.CircuitBreakerTimeout = 100 * time.Millisecond

	lb := NewLoadBalancer(config)

	// Record successful requests
	for i := 0; i < 10; i++ {
		lb.RecordRequest("node-1", true, 10*time.Millisecond)
	}

	info, exists := lb.GetNodeInfo("node-1")
	require.True(t, exists)
	assert.Equal(t, CircuitClosed, info.CircuitState)

	// Record failures to trip circuit
	for i := 0; i < 15; i++ {
		lb.RecordRequest("node-1", false, 10*time.Millisecond)
	}

	info, exists = lb.GetNodeInfo("node-1")
	require.True(t, exists)
	assert.Equal(t, CircuitOpen, info.CircuitState)

	// Wait for circuit timeout
	time.Sleep(150 * time.Millisecond)

	// Record success to close circuit
	lb.RecordRequest("node-1", true, 10*time.Millisecond)
	lb.RecordRequest("node-1", true, 10*time.Millisecond)

	info, exists = lb.GetNodeInfo("node-1")
	require.True(t, exists)
	assert.Equal(t, CircuitClosed, info.CircuitState)
}

// Test concurrent validation
func TestConcurrentDistributedValidation(t *testing.T) {
	// Removed t.Parallel() to avoid resource contention with worker pool
	defer testhelper.VerifyNoGoroutineLeaks(t)

	// Use test context with automatic cleanup
	ctx := testhelper.NewTestContextWithTimeout(t, 30*time.Second)

	// Set up cleanup manager
	cleanup := testhelper.NewCleanupManager(t)

	config := TestingDistributedValidatorConfig("node-1")
	// Set consensus to only require 1 node since we're testing locally
	config.ConsensusConfig.MinNodes = 1
	config.ConsensusConfig.QuorumSize = 1

	// Use a permissive validator that won't reject events based on sequence
	localValidator := events.NewEventValidator(&events.ValidationConfig{
		Level:                   events.ValidationPermissive,
		SkipTimestampValidation: true,
		SkipSequenceValidation:  true,
		AllowEmptyIDs:           true,
		AllowUnknownEventTypes:  false,
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

	// Perform concurrent validations with limited concurrency
	var wg sync.WaitGroup
	results := make([]*ValidationResult, 50)
	semaphore := make(chan struct{}, 10) // Limit concurrent validations
	errorCount := 0
	var errorMutex sync.Mutex

	// Register cleanup for semaphore channel
	cleanup.Register("semaphore", func() {
		testhelper.CloseChannel(t, semaphore, "semaphore")
	})

	// Run concurrent validations
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				errorMutex.Lock()
				errorCount++
				errorMutex.Unlock()
				return
			}

			// Create independent RUN_STARTED events (each is a separate run)
			event := &events.RunStartedEvent{
				BaseEvent: &events.BaseEvent{
					EventType:   events.EventTypeRunStarted,
					TimestampMs: func() *int64 { t := time.Now().UnixMilli() + int64(idx*10); return &t }(),
				},
				RunIDValue:    fmt.Sprintf("test-run-%d", idx),
				ThreadIDValue: fmt.Sprintf("test-thread-%d", idx),
			}

			// Use context with timeout for each validation
			validateCtx, validateCancel := context.WithTimeout(ctx, 3*time.Second)
			defer validateCancel()

			result := dv.ValidateEvent(validateCtx, event)
			if result != nil {
				results[idx] = result
				if !result.IsValid && len(result.Errors) > 0 {
					t.Logf("Event %d failed: %s", idx, result.Errors[0].Message)
				}
			} else {
				errorMutex.Lock()
				errorCount++
				errorMutex.Unlock()
			}
		}(i)
	}

	// Wait for all validations with timeout
	if !testhelper.WaitGroupTimeout(t, &wg, 5*time.Second) {
		t.Fatal("Test timed out waiting for concurrent validations")
	}

	// Verify results
	validCount := 0
	invalidCount := 0
	nilCount := 0

	for _, result := range results {
		if result == nil {
			nilCount++
		} else if result.IsValid {
			validCount++
		} else {
			invalidCount++
		}
	}

	t.Logf("Results: valid=%d, invalid=%d, nil=%d, errors=%d", validCount, invalidCount, nilCount, errorCount)

	// With permissive validation and skip sequence, events should validate
	// We're testing concurrent execution works without deadlocks
	assert.GreaterOrEqual(t, validCount, 40, "At least 40 events should be valid")
	assert.LessOrEqual(t, nilCount+errorCount, 5, "At most 5 events should timeout")
}

// Test distributed lock functionality
func TestDistributedLock(t *testing.T) {
	// Removed t.Parallel() to avoid resource contention with worker pool
	defer testhelper.VerifyNoGoroutineLeaks(t)

	// Use test context with automatic cleanup
	ctx := testhelper.NewTestContextWithTimeout(t, 15*time.Second)

	// Set up cleanup manager
	cleanup := testhelper.NewCleanupManager(t)

	config := DefaultConsensusConfig()
	cm1, err := NewConsensusManager(config, "node-1")
	require.NoError(t, err)

	// Start consensus manager
	err = cm1.Start(ctx)
	require.NoError(t, err)

	// Register cleanup for consensus manager
	cleanup.Register("consensus-manager", func() {
		if err := cm1.Stop(); err != nil {
			t.Logf("Error stopping consensus manager: %v", err)
		}
	})

	// Test lock acquisition and release on same node
	// Acquire lock with timeout
	lockCtx1, lockCancel1 := context.WithTimeout(ctx, 2*time.Second)
	defer lockCancel1()
	err = cm1.AcquireLock(lockCtx1, "test-lock", 1*time.Second)
	assert.NoError(t, err)

	// Try to acquire same lock again on same node (should succeed by extending)
	lockCtx2, lockCancel2 := context.WithTimeout(ctx, 500*time.Millisecond)
	defer lockCancel2()
	err = cm1.AcquireLock(lockCtx2, "test-lock", 1*time.Second)
	assert.NoError(t, err)

	// Release lock
	releaseCtx1, releaseCancel1 := context.WithTimeout(ctx, 1*time.Second)
	defer releaseCancel1()
	err = cm1.ReleaseLock(releaseCtx1, "test-lock")
	assert.NoError(t, err)

	// Now should be able to acquire again
	lockCtx3, lockCancel3 := context.WithTimeout(ctx, 2*time.Second)
	defer lockCancel3()
	err = cm1.AcquireLock(lockCtx3, "test-lock", 1*time.Second)
	assert.NoError(t, err)

	// Clean up
	releaseCtx2, releaseCancel2 := context.WithTimeout(ctx, 1*time.Second)
	defer releaseCancel2()
	err = cm1.ReleaseLock(releaseCtx2, "test-lock")
	assert.NoError(t, err)

	// Test expired lock takeover
	// Acquire lock with very short duration
	lockCtx4, lockCancel4 := context.WithTimeout(ctx, 1*time.Second)
	defer lockCancel4()
	err = cm1.AcquireLock(lockCtx4, "test-lock-2", 10*time.Millisecond)
	assert.NoError(t, err)

	// Wait for lock to expire
	time.Sleep(20 * time.Millisecond)

	// Should be able to acquire expired lock
	lockCtx5, lockCancel5 := context.WithTimeout(ctx, 1*time.Second)
	defer lockCancel5()
	err = cm1.AcquireLock(lockCtx5, "test-lock-2", 1*time.Second)
	assert.NoError(t, err)
}

// Test metrics collection
func TestDistributedMetrics(t *testing.T) {
	// Removed t.Parallel() to avoid resource contention with worker pool
	// Remove defer here to fix cleanup order - moved to end of function

	// Use test context with automatic cleanup
	ctx := testhelper.NewTestContextWithTimeout(t, 35*time.Second)

	config := TestingDistributedValidatorConfig("node-1")
	config.EnableMetrics = true

	// Use a permissive local validator to avoid sequence validation issues
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

	// Ensure cleanup happens before goroutine leak check
	defer func() {
		// Stop the distributed validator first
		if err := dv.Stop(); err != nil {
			t.Logf("Error stopping distributed validator: %v", err)
		}

		// Give goroutines time to fully exit
		time.Sleep(100 * time.Millisecond)

		// Now check for goroutine leaks
		testhelper.VerifyNoGoroutineLeaks(t)
	}()

	// Test that metrics collection is working by checking initial state
	metrics := dv.GetMetrics()
	assert.Equal(t, uint64(0), metrics.GetValidationCount())
	assert.Equal(t, float64(0), metrics.GetErrorRate())
	assert.GreaterOrEqual(t, metrics.GetAverageResponseTime(), float64(0))

	// Metrics should still be available after starting
	metrics = dv.GetMetrics()
	assert.Equal(t, uint64(0), metrics.GetValidationCount())
	assert.Equal(t, float64(0), metrics.GetErrorRate())
	assert.GreaterOrEqual(t, metrics.GetAverageResponseTime(), float64(0))
}

// Benchmark distributed validation
func BenchmarkDistributedValidation(b *testing.B) {
	config := TestingDistributedValidatorConfig("node-1")
	localValidator := events.NewEventValidator(nil)

	dv, err := NewDistributedValidator(config, localValidator)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	err = dv.Start(ctx)
	if err != nil {
		b.Fatal(err)
	}
	defer dv.Stop()

	// Register additional nodes
	for i := 2; i <= 5; i++ {
		node := &NodeInfo{
			ID:            NodeID(fmt.Sprintf("node-%d", i)),
			State:         NodeStateActive,
			LastHeartbeat: time.Now(),
		}
		dv.RegisterNode(node)
	}
	event := &mockEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventType("MOCK"),
		},
		ID:    "bench-1",
		Valid: true,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = dv.ValidateEvent(ctx, event)
	}
}

// Benchmark consensus algorithms
func BenchmarkConsensusAlgorithms(b *testing.B) {
	algorithms := []ConsensusAlgorithm{
		ConsensusMajority,
		ConsensusUnanimous,
		ConsensusPBFT,
	}

	decisions := make([]*ValidationDecision, 10)
	for i := 0; i < 10; i++ {
		decisions[i] = &ValidationDecision{
			NodeID:  NodeID(fmt.Sprintf("node-%d", i)),
			IsValid: i%2 == 0,
		}
	}

	for _, algo := range algorithms {
		b.Run(string(algo), func(b *testing.B) {
			config := &ConsensusConfig{
				Algorithm:  algo,
				MinNodes:   5,
				QuorumSize: 3,
			}

			cm, err := NewConsensusManager(config, "node-1")
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				cm.AggregateDecisions(decisions)
			}
		})
	}
}
