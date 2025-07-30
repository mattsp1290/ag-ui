package distributed

import (
	"context"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSimpleDistributedValidatorCreation tests basic distributed validator creation
func TestSimpleDistributedValidatorCreation(t *testing.T) {
	config := DefaultDistributedValidatorConfig("node-1")
	localValidator := events.NewEventValidator(nil)

	dv, err := NewDistributedValidator(config, localValidator)
	require.NoError(t, err)
	require.NotNil(t, dv)

	// Verify basic components are initialized
	assert.Equal(t, NodeID("node-1"), dv.config.NodeID)
	assert.NotNil(t, dv.consensus)
	assert.NotNil(t, dv.stateSync)
	assert.NotNil(t, dv.partitionHandler)
	assert.NotNil(t, dv.loadBalancer)
	assert.NotNil(t, dv.localValidator)
}

// TestSimpleConsensus tests basic consensus functionality without timing
func TestSimpleConsensus(t *testing.T) {
	tests := []struct {
		name      string
		algorithm ConsensusAlgorithm
		decisions []*ValidationDecision
		expected  bool
	}{
		{
			name:      "Majority - 2 valid, 1 invalid",
			algorithm: ConsensusMajority,
			decisions: []*ValidationDecision{
				{NodeID: "node-1", IsValid: true},
				{NodeID: "node-2", IsValid: true},
				{NodeID: "node-3", IsValid: false},
			},
			expected: true,
		},
		{
			name:      "Unanimous - all valid",
			algorithm: ConsensusUnanimous,
			decisions: []*ValidationDecision{
				{NodeID: "node-1", IsValid: true},
				{NodeID: "node-2", IsValid: true},
			},
			expected: true,
		},
		{
			name:      "Unanimous - one invalid",
			algorithm: ConsensusUnanimous,
			decisions: []*ValidationDecision{
				{NodeID: "node-1", IsValid: true},
				{NodeID: "node-2", IsValid: false},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ConsensusConfig{
				Algorithm:  tt.algorithm,
				MinNodes:   1,
				QuorumSize: len(tt.decisions),
			}

			cm, err := NewConsensusManager(config, "node-1")
			require.NoError(t, err)

			result := cm.AggregateDecisions(tt.decisions)
			assert.Equal(t, tt.expected, result.IsValid)
		})
	}
}

// TestSimpleLoadBalancer tests basic load balancer functionality
func TestSimpleLoadBalancer(t *testing.T) {
	config := &LoadBalancerConfig{
		Algorithm:            LoadBalancingRoundRobin,
		EnableCircuitBreaker: false, // Disable circuit breaker for simplicity
	}

	lb := NewLoadBalancer(config)

	// Set up node metrics
	nodes := []NodeID{"node-1", "node-2", "node-3"}
	lb.UpdateNodeMetrics("node-1", 0.1, 10.0)
	lb.UpdateNodeMetrics("node-2", 0.2, 20.0)
	lb.UpdateNodeMetrics("node-3", 0.3, 30.0)

	// Test round-robin selection
	selected := lb.SelectNodes(nodes, 2)
	assert.Len(t, selected, 2)

	// Verify selected nodes are from available nodes
	for _, node := range selected {
		assert.Contains(t, nodes, node)
	}

	// Test selecting more nodes than available
	selected = lb.SelectNodes(nodes, 5)
	assert.Len(t, selected, 3) // Should return all available nodes
}

// TestSimplePartitionDetection tests basic partition detection
func TestSimplePartitionDetection(t *testing.T) {
	config := &PartitionHandlerConfig{
		HeartbeatTimeout:      1 * time.Second,
		QuorumSize:            3, // Need 3 nodes for quorum
		AllowLocalValidation:  true,
		AutoRecovery:          false, // Disable auto-recovery for simplicity
		MinNodesForOperation:  3,
	}

	ph := NewPartitionHandler(config, "node-1")

	// Set up partition detection callback with buffered channel
	partitionDetected := make(chan *PartitionInfo, 1)
	ph.SetPartitionCallbacks(
		func(p *PartitionInfo) {
			select {
			case partitionDetected <- p:
			default:
				// Channel full, ignore
			}
		},
		nil,
	)

	// Start the partition handler to enable detection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := ph.Start(ctx)
	require.NoError(t, err)
	defer ph.Stop()

	// Register nodes as healthy
	ph.UpdateNodeHealth("node-2", true, 100*time.Millisecond)
	ph.UpdateNodeHealth("node-3", true, 100*time.Millisecond)

	// Initially should not be partitioned (3 healthy nodes)
	assert.False(t, ph.IsPartitioned())

	// Simulate node failures multiple times to trigger detection
	// Each node needs 3 consecutive failures to be marked as unreachable
	for i := 0; i < 3; i++ {
		ph.HandleNodeFailure("node-2")
	}
	for i := 0; i < 3; i++ {
		ph.HandleNodeFailure("node-3")
	}

	// Wait for partition detection with timeout
	select {
	case partition := <-partitionDetected:
		assert.True(t, partition.IsActive)
		// At least one node should be unreachable
		assert.Greater(t, len(partition.UnreachableNodes), 0)
		// The partition should be detected because we have fewer than MinNodesForOperation reachable nodes
		// Since MinNodesForOperation is 3 and we have 1 (self) + 0 unreachable = 1 reachable node
		assert.GreaterOrEqual(t, len(partition.UnreachableNodes), 1, "At least one node should be unreachable")
	case <-ctx.Done():
		t.Fatal("Test context timeout - partition not detected")
	case <-time.After(2 * time.Second):
		t.Fatal("Partition not detected within timeout")
	}

	// Now should be partitioned (only 1 healthy node, need 3 for quorum)
	assert.True(t, ph.IsPartitioned())
}

// TestSimpleNodeRegistration tests basic node registration
func TestSimpleNodeRegistration(t *testing.T) {
	config := DefaultDistributedValidatorConfig("node-1")
	localValidator := events.NewEventValidator(nil)

	dv, err := NewDistributedValidator(config, localValidator)
	require.NoError(t, err)

	// Register a node
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

	// Verify node is registered
	info, exists := dv.GetNodeInfo("node-2")
	assert.True(t, exists)
	assert.Equal(t, NodeID("node-2"), info.ID)
	assert.Equal(t, "node2:8080", info.Address)

	// Get all nodes
	allNodes := dv.GetAllNodes()
	assert.Len(t, allNodes, 1)
	assert.NotNil(t, allNodes[NodeID("node-2")])

	// Unregister node
	err = dv.UnregisterNode("node-2")
	assert.NoError(t, err)

	// Verify node is unregistered
	_, exists = dv.GetNodeInfo("node-2")
	assert.False(t, exists)
}

// TestSimpleLocalValidationFallback tests fallback to local validation
func TestSimpleLocalValidationFallback(t *testing.T) {
	config := DefaultDistributedValidatorConfig("node-1")
	config.PartitionHandler.AllowLocalValidation = true
	config.ValidationTimeout = 1 * time.Second // Set a short timeout
	config.PartitionHandler.MinNodesForOperation = 1 // Allow operation with 1 node
	// Configure consensus to only require 1 node for quorum
	config.ConsensusConfig.MinNodes = 1
	config.ConsensusConfig.QuorumSize = 1
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

	// Start the distributed validator
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	err = dv.Start(ctx)
	require.NoError(t, err)
	defer dv.Stop()

	// Create a valid event that should pass validation - using a RUN_STARTED event
	event := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeRunStarted,
			TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
		},
		RunIDValue:    "test-run-1",
		ThreadIDValue: "test-thread-1",
	}

	// Don't register any other nodes - this will force local validation

	// Validate event - should fall back to local validation since no other nodes exist
	validateCtx, validateCancel := context.WithTimeout(ctx, 3*time.Second)
	defer validateCancel()
	
	result := dv.ValidateEvent(validateCtx, event)

	if !result.IsValid && len(result.Errors) > 0 {
		t.Logf("Validation failed with error: %s", result.Errors[0].Message)
	}

	assert.True(t, result.IsValid)
	assert.Len(t, result.Errors, 0)
}

// TestSimpleStateSynchronization tests basic state sync functionality
func TestSimpleStateSynchronization(t *testing.T) {
	config := DefaultStateSyncConfig()
	ss, err := NewStateSynchronizer(config, "node-1")
	require.NoError(t, err)

	// Set state
	err = ss.SetState("key1", "value1")
	assert.NoError(t, err)

	err = ss.SetState("key2", "value2")
	assert.NoError(t, err)

	// Get state
	state, exists := ss.GetState("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", state.Value)

	state, exists = ss.GetState("key2")
	assert.True(t, exists)
	assert.Equal(t, "value2", state.Value)

	// Get snapshot
	snapshot := ss.GetSnapshot()
	assert.NotNil(t, snapshot)
	assert.Len(t, snapshot.StateItems, 2)

	// Apply snapshot to verify it works
	err = ss.ApplySnapshot(snapshot)
	assert.NoError(t, err)

	// Verify state still exists
	state, exists = ss.GetState("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", state.Value)
}

// TestSimpleMetricsCollection tests basic metrics collection
func TestSimpleMetricsCollection(t *testing.T) {
	config := DefaultDistributedValidatorConfig("node-1")
	config.EnableMetrics = true
	localValidator := events.NewEventValidator(nil)

	dv, err := NewDistributedValidator(config, localValidator)
	require.NoError(t, err)

	// Test that metrics collection is working by checking initial state
	metrics := dv.GetMetrics()
	assert.Equal(t, uint64(0), metrics.GetValidationCount())
	assert.Equal(t, float64(0), metrics.GetErrorRate())
	assert.GreaterOrEqual(t, metrics.GetAverageResponseTime(), float64(0))
	
	// Test that we can start and stop the validator
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err = dv.Start(ctx)
	require.NoError(t, err)
	defer dv.Stop()
	
	// Metrics should still be available after starting
	metrics = dv.GetMetrics()
	assert.Equal(t, uint64(0), metrics.GetValidationCount())
	assert.Equal(t, float64(0), metrics.GetErrorRate())
}

// TestSimpleCircuitBreaker tests basic circuit breaker functionality
func TestSimpleCircuitBreaker(t *testing.T) {
	config := &LoadBalancerConfig{
		Algorithm:               LoadBalancingRoundRobin,
		EnableCircuitBreaker:    true,
		CircuitBreakerThreshold: 0.5,
		CircuitBreakerTimeout:   100 * time.Millisecond,
	}

	lb := NewLoadBalancer(config)

	// Record successful requests
	for i := 0; i < 10; i++ {
		lb.RecordRequest("node-1", true, 10*time.Millisecond)
	}

	// Check circuit is closed
	info, exists := lb.GetNodeInfo("node-1")
	require.True(t, exists)
	assert.Equal(t, CircuitClosed, info.CircuitState)

	// Record failures to trip circuit
	for i := 0; i < 15; i++ {
		lb.RecordRequest("node-1", false, 10*time.Millisecond)
	}

	// Check circuit is open
	info, exists = lb.GetNodeInfo("node-1")
	require.True(t, exists)
	assert.Equal(t, CircuitOpen, info.CircuitState)

	// Verify we can retrieve node info and the circuit breaker is functioning
	assert.NotNil(t, info)
	assert.Equal(t, NodeID("node-1"), info.NodeID)
	assert.NotEqual(t, CircuitClosed, info.CircuitState) // Should not be closed anymore
}