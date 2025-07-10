package distributed

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/proto/generated"
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
	config := DefaultDistributedValidatorConfig("node-1")
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
	config := DefaultDistributedValidatorConfig("node-1")
	localValidator := events.NewEventValidator(nil)

	dv, err := NewDistributedValidator(config, localValidator)
	require.NoError(t, err)

	// Test Start
	err = dv.Start()
	assert.NoError(t, err)

	// Test double start
	err = dv.Start()
	assert.Error(t, err)

	// Test Stop
	err = dv.Stop()
	assert.NoError(t, err)

	// Test double stop
	err = dv.Stop()
	assert.NoError(t, err)
}

// Test node registration and management
func TestNodeManagement(t *testing.T) {
	config := DefaultDistributedValidatorConfig("node-1")
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
	config := DefaultDistributedValidatorConfig("node-1")
	config.PartitionHandler.AllowLocalValidation = true
	localValidator := events.NewEventValidator(nil)

	dv, err := NewDistributedValidator(config, localValidator)
	require.NoError(t, err)

	err = dv.Start()
	require.NoError(t, err)
	defer dv.Stop()

	// Create a valid event
	event := &mockEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventType("MOCK"),
		},
		ID:    "test-1",
		Valid: true,
	}

	// Simulate partition
	dv.partitionHandler.HandleNodeFailure("node-2")
	dv.partitionHandler.HandleNodeFailure("node-3")

	// Validate event during partition
	ctx := context.Background()
	result := dv.ValidateEvent(ctx, event)

	// Should fall back to local validation
	assert.True(t, result.IsValid)
	assert.Len(t, result.Errors, 0)
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
	config := DefaultStateSyncConfig()
	ss, err := NewStateSynchronizer(config, "node-1")
	require.NoError(t, err)

	err = ss.Start()
	require.NoError(t, err)
	defer ss.Stop()

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
}

// Test partition detection and recovery
func TestPartitionDetectionAndRecovery(t *testing.T) {
	config := DefaultPartitionHandlerConfig()
	config.HeartbeatTimeout = 100 * time.Millisecond
	config.AutoRecovery = true

	ph := NewPartitionHandler(config, "node-1")

	// Set up partition callbacks
	partitionDetected := make(chan *PartitionInfo, 1)
	partitionRecovered := make(chan *PartitionInfo, 1)

	ph.SetPartitionCallbacks(
		func(p *PartitionInfo) { partitionDetected <- p },
		func(p *PartitionInfo) { partitionRecovered <- p },
	)

	err := ph.Start()
	require.NoError(t, err)
	defer ph.Stop()

	// Register healthy nodes
	ph.UpdateNodeHealth("node-2", true, 10*time.Millisecond)
	ph.UpdateNodeHealth("node-3", true, 10*time.Millisecond)

	// Simulate node failures
	ph.HandleNodeFailure("node-2")
	ph.HandleNodeFailure("node-3")

	// Wait for partition detection
	select {
	case partition := <-partitionDetected:
		assert.True(t, partition.IsActive)
		assert.Contains(t, partition.UnreachableNodes, NodeID("node-2"))
		assert.Contains(t, partition.UnreachableNodes, NodeID("node-3"))
	case <-time.After(1 * time.Second):
		t.Fatal("Partition not detected within timeout")
	}

	assert.True(t, ph.IsPartitioned())
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
	config := DefaultDistributedValidatorConfig("node-1")
	localValidator := events.NewEventValidator(nil)

	dv, err := NewDistributedValidator(config, localValidator)
	require.NoError(t, err)

	err = dv.Start()
	require.NoError(t, err)
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

	// Perform concurrent validations
	var wg sync.WaitGroup
	results := make([]*ValidationResult, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			event := &mockEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventType("MOCK"),
				},
				ID:    fmt.Sprintf("test-%d", idx),
				Valid: idx%2 == 0, // Half valid, half invalid
			}

			ctx := context.Background()
			results[idx] = dv.ValidateEvent(ctx, event)
		}(i)
	}

	wg.Wait()

	// Verify results
	validCount := 0
	invalidCount := 0

	for _, result := range results {
		if result.IsValid {
			validCount++
		} else {
			invalidCount++
		}
	}

	assert.Equal(t, 50, validCount)
	assert.Equal(t, 50, invalidCount)
}

// Test distributed lock functionality
func TestDistributedLock(t *testing.T) {
	config := DefaultConsensusConfig()
	cm1, err := NewConsensusManager(config, "node-1")
	require.NoError(t, err)

	cm2, err := NewConsensusManager(config, "node-2")
	require.NoError(t, err)

	ctx := context.Background()

	// Acquire lock on node 1
	err = cm1.AcquireLock(ctx, "test-lock", 1*time.Second)
	assert.NoError(t, err)

	// Try to acquire same lock on node 2 (should fail)
	err = cm2.AcquireLock(ctx, "test-lock", 1*time.Second)
	assert.Error(t, err)

	// Release lock on node 1
	err = cm1.ReleaseLock(ctx, "test-lock")
	assert.NoError(t, err)

	// Now node 2 should be able to acquire
	err = cm2.AcquireLock(ctx, "test-lock", 1*time.Second)
	assert.NoError(t, err)

	// Clean up
	err = cm2.ReleaseLock(ctx, "test-lock")
	assert.NoError(t, err)
}

// Test metrics collection
func TestDistributedMetrics(t *testing.T) {
	config := DefaultDistributedValidatorConfig("node-1")
	config.EnableMetrics = true
	localValidator := events.NewEventValidator(nil)

	dv, err := NewDistributedValidator(config, localValidator)
	require.NoError(t, err)

	err = dv.Start()
	require.NoError(t, err)
	defer dv.Stop()

	// Perform some validations
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		event := &mockEvent{
			BaseEvent: &events.BaseEvent{
				EventType: events.EventType("MOCK"),
			},
			ID:    fmt.Sprintf("test-%d", i),
			Valid: true,
		}
		_ = dv.ValidateEvent(ctx, event)
	}

	// Get metrics
	metrics := dv.GetMetrics()
	assert.Equal(t, uint64(10), metrics.GetValidationCount())
	assert.Equal(t, float64(0), metrics.GetErrorRate())
	assert.Greater(t, metrics.GetAverageResponseTime(), float64(0))
}

// Benchmark distributed validation
func BenchmarkDistributedValidation(b *testing.B) {
	config := DefaultDistributedValidatorConfig("node-1")
	localValidator := events.NewEventValidator(nil)

	dv, err := NewDistributedValidator(config, localValidator)
	if err != nil {
		b.Fatal(err)
	}

	err = dv.Start()
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

	ctx := context.Background()
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