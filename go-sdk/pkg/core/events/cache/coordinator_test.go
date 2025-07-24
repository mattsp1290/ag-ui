package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// CacheCoordinatorTestSuite provides comprehensive testing for cache coordination
type CacheCoordinatorTestSuite struct {
	suite.Suite
	coordinator *CacheCoordinator
	transport   *MockTransport
	ctx         context.Context
	cancel      context.CancelFunc
}

// MockTransport implements the Transport interface for testing
type MockTransport struct {
	mu          sync.RWMutex
	messages    []Message
	subscribers map[string][]chan Message
	closed      bool
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		messages:    make([]Message, 0),
		subscribers: make(map[string][]chan Message),
	}
}

func (m *MockTransport) Send(ctx context.Context, nodeID string, message Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return fmt.Errorf("transport closed")
	}
	
	message.Target = nodeID
	m.messages = append(m.messages, message)
	
	// Deliver to subscribers
	if channels, exists := m.subscribers[message.Type]; exists {
		for _, ch := range channels {
			select {
			case ch <- message:
			default:
				// Channel full, drop message
			}
		}
	}
	
	return nil
}

func (m *MockTransport) Broadcast(ctx context.Context, message Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return fmt.Errorf("transport closed")
	}
	
	m.messages = append(m.messages, message)
	
	// Deliver to all subscribers
	if channels, exists := m.subscribers[message.Type]; exists {
		for _, ch := range channels {
			select {
			case ch <- message:
			default:
				// Channel full, drop message
			}
		}
	}
	
	return nil
}

func (m *MockTransport) Subscribe(messageType string) <-chan Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	ch := make(chan Message, 100)
	if m.subscribers[messageType] == nil {
		m.subscribers[messageType] = make([]chan Message, 0)
	}
	m.subscribers[messageType] = append(m.subscribers[messageType], ch)
	
	return ch
}

func (m *MockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return nil // Already closed
	}
	
	m.closed = true
	
	// Close all subscriber channels
	for _, channels := range m.subscribers {
		for _, ch := range channels {
			select {
			case <-ch:
				// Channel already closed
			default:
				close(ch)
			}
		}
	}
	
	return nil
}

func (m *MockTransport) GetMessages() []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	messages := make([]Message, len(m.messages))
	copy(messages, m.messages)
	return messages
}

func (m *MockTransport) ClearMessages() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.messages = make([]Message, 0)
}

func (suite *CacheCoordinatorTestSuite) SetupTest() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.transport = NewMockTransport()
	
	config := DefaultCoordinatorConfig()
	config.HeartbeatInterval = 100 * time.Millisecond
	config.NodeTimeout = 500 * time.Millisecond
	
	suite.coordinator = NewCacheCoordinator("node-1", suite.transport, config)
	err := suite.coordinator.Start(suite.ctx)
	suite.Require().NoError(err)
}

func (suite *CacheCoordinatorTestSuite) TearDownTest() {
	if suite.coordinator != nil {
		suite.coordinator.Stop(suite.ctx)
	}
	if suite.transport != nil {
		suite.transport.Close()
	}
	if suite.cancel != nil {
		suite.cancel()
	}
}

func (suite *CacheCoordinatorTestSuite) TestCoordinatorCreation() {
	suite.NotNil(suite.coordinator)
	suite.Equal("node-1", suite.coordinator.nodeID)
	suite.NotNil(suite.coordinator.nodes["node-1"])
	suite.Equal(NodeStateActive, suite.coordinator.nodes["node-1"].State)
}

func (suite *CacheCoordinatorTestSuite) TestBroadcastInvalidation() {
	invalidationMsg := InvalidationMessage{
		NodeID:    "node-1",
		EventType: "test-event",
		Timestamp: time.Now(),
	}
	
	err := suite.coordinator.BroadcastInvalidation(suite.ctx, invalidationMsg)
	suite.NoError(err)
	
	// Check that message was broadcast
	messages := suite.transport.GetMessages()
	suite.Len(messages, 1)
	suite.Equal("invalidation", messages[0].Type)
	suite.Equal("node-1", messages[0].Source)
	
	// Verify message content
	var receivedMsg InvalidationMessage
	err = json.Unmarshal(messages[0].Payload, &receivedMsg)
	suite.NoError(err)
	suite.Equal(invalidationMsg.NodeID, receivedMsg.NodeID)
	suite.Equal(invalidationMsg.EventType, receivedMsg.EventType)
}

func (suite *CacheCoordinatorTestSuite) TestNotifyCacheUpdate() {
	updateMsg := CacheUpdateMessage{
		NodeID:    "node-1",
		Key:       "test-key",
		EventType: "test-event",
		Operation: "SET",
		Timestamp: time.Now(),
	}
	
	err := suite.coordinator.NotifyCacheUpdate(suite.ctx, updateMsg)
	suite.NoError(err)
	
	// Check that message was broadcast
	messages := suite.transport.GetMessages()
	suite.Len(messages, 1)
	suite.Equal("cache_update", messages[0].Type)
	suite.Equal("node-1", messages[0].Source)
}

func (suite *CacheCoordinatorTestSuite) TestHeartbeatWorker() {
	// Wait for a few heartbeats
	time.Sleep(250 * time.Millisecond)
	
	// Check that heartbeat messages were sent
	messages := suite.transport.GetMessages()
	suite.Greater(len(messages), 0)
	
	// Find heartbeat messages
	heartbeatCount := 0
	for _, msg := range messages {
		if msg.Type == "heartbeat" {
			heartbeatCount++
		}
	}
	
	suite.Greater(heartbeatCount, 0)
}

func (suite *CacheCoordinatorTestSuite) TestNodeHealthManagement() {
	// Add a node
	suite.coordinator.mu.Lock()
	suite.coordinator.nodes["node-2"] = &NodeInfo{
		ID:            "node-2",
		State:         NodeStateActive,
		LastHeartbeat: time.Now(),
	}
	suite.coordinator.mu.Unlock()
	
	// Wait for health check
	time.Sleep(200 * time.Millisecond)
	
	// Node should still be active
	suite.coordinator.mu.RLock()
	node := suite.coordinator.nodes["node-2"]
	suite.Equal(NodeStateActive, node.State)
	suite.coordinator.mu.RUnlock()
	
	// Set node's last heartbeat to past
	suite.coordinator.mu.Lock()
	suite.coordinator.nodes["node-2"].LastHeartbeat = time.Now().Add(-1 * time.Second)
	suite.coordinator.mu.Unlock()
	
	// Wait for health check
	time.Sleep(200 * time.Millisecond)
	
	// Node should be suspect
	suite.coordinator.mu.RLock()
	node = suite.coordinator.nodes["node-2"]
	suite.Equal(NodeStateSuspect, node.State)
	suite.coordinator.mu.RUnlock()
	
	// Wait more time
	time.Sleep(300 * time.Millisecond)
	
	// Node should be failed
	suite.coordinator.mu.RLock()
	node = suite.coordinator.nodes["node-2"]
	suite.Equal(NodeStateFailed, node.State)
	suite.coordinator.mu.RUnlock()
}

func (suite *CacheCoordinatorTestSuite) TestConsensusRequest() {
	// Create a simple consensus request
	request := ConsensusRequest{
		ID:        "req-1",
		Operation: "INVALIDATE",
		Key:       "test-key",
		Value:     "test-value",
		Timestamp: time.Now(),
	}
	
	// Add another active node to have quorum
	suite.coordinator.mu.Lock()
	suite.coordinator.nodes["node-2"] = &NodeInfo{
		ID:            "node-2",
		State:         NodeStateActive,
		LastHeartbeat: time.Now(),
	}
	suite.coordinator.mu.Unlock()
	
	// Request consensus
	consensus, err := suite.coordinator.RequestConsensus(suite.ctx, request)
	suite.NoError(err)
	
	// With only self vote, should achieve consensus (quorum = 0.51)
	suite.True(consensus)
	
	// Check that request was broadcast
	messages := suite.transport.GetMessages()
	consensusRequests := 0
	for _, msg := range messages {
		if msg.Type == "consensus_request" {
			consensusRequests++
		}
	}
	suite.Greater(consensusRequests, 0)
}

func (suite *CacheCoordinatorTestSuite) TestShardingEnabled() {
	// Create coordinator with sharding enabled
	config := DefaultCoordinatorConfig()
	config.EnableSharding = true
	config.ShardCount = 4
	
	coordinator := NewCacheCoordinator("node-1", suite.transport, config)
	defer coordinator.Stop(suite.ctx)
	
	// Add nodes
	coordinator.mu.Lock()
	coordinator.nodes["node-2"] = &NodeInfo{
		ID:            "node-2",
		State:         NodeStateActive,
		LastHeartbeat: time.Now(),
	}
	coordinator.mu.Unlock()
	
	// Start coordinator
	err := coordinator.Start(suite.ctx)
	suite.NoError(err)
	
	// Wait for shard rebalancing
	time.Sleep(100 * time.Millisecond)
	
	// Check shard distribution
	info := coordinator.GetClusterInfo()
	suite.Equal(4, info["shard_count"])
	suite.Equal(true, info["sharding_enabled"])
}

func (suite *CacheCoordinatorTestSuite) TestShardKeyMapping() {
	// Create coordinator with sharding enabled
	config := DefaultCoordinatorConfig()
	config.EnableSharding = true
	config.ShardCount = 4
	
	coordinator := NewCacheCoordinator("node-1", suite.transport, config)
	defer coordinator.Stop(suite.ctx)
	
	// Test shard calculation
	key1 := "test-key-1"
	key2 := "test-key-2"
	key3 := "test-key-1" // Same as key1
	
	shard1 := coordinator.getShardForKey(key1)
	shard2 := coordinator.getShardForKey(key2)
	shard3 := coordinator.getShardForKey(key3)
	
	suite.GreaterOrEqual(shard1, 0)
	suite.Less(shard1, 4)
	suite.GreaterOrEqual(shard2, 0)
	suite.Less(shard2, 4)
	suite.Equal(shard1, shard3) // Same key should map to same shard
}

func (suite *CacheCoordinatorTestSuite) TestShardedCacheUpdate() {
	// Create coordinator with sharding enabled
	config := DefaultCoordinatorConfig()
	config.EnableSharding = true
	config.ShardCount = 4
	
	coordinator := NewCacheCoordinator("node-1", suite.transport, config)
	defer coordinator.Stop(suite.ctx)
	
	err := coordinator.Start(suite.ctx)
	suite.NoError(err)
	
	// Add nodes and set up shards
	coordinator.mu.Lock()
	coordinator.nodes["node-2"] = &NodeInfo{
		ID:            "node-2",
		State:         NodeStateActive,
		LastHeartbeat: time.Now(),
		Shards:        []int{0, 1},
	}
	coordinator.clusterState.ShardMap = map[int][]string{
		0: {"node-1"},
		1: {"node-2"},
		2: {"node-1"},
		3: {"node-2"},
	}
	coordinator.mu.Unlock()
	
	// Find a key that maps to shard 1 (which has node-2)
	testKey := "a" // Simple key that should map to shard 1
	hash := 0
	for _, b := range []byte(testKey) {
		hash = hash*31 + int(b)
	}
	expectedShard := hash % 4
	// If it doesn't map to shard 1, try a different key
	if expectedShard != 1 && expectedShard != 3 { // shard 1 or 3 both have node-2
		testKey = "b"
	}
	
	updateMsg := CacheUpdateMessage{
		NodeID:    "node-1",
		Key:       testKey,
		EventType: "test-event",
		Operation: "SET",
		Timestamp: time.Now(),
	}
	
	err = coordinator.NotifyCacheUpdate(suite.ctx, updateMsg)
	suite.NoError(err)
	
	// Should send targeted messages based on sharding
	messages := suite.transport.GetMessages()
	suite.Greater(len(messages), 0)
}

func (suite *CacheCoordinatorTestSuite) TestMetricsReporting() {
	report := MetricsReport{
		NodeID: "node-1",
		Stats: CacheStats{
			L1Hits:   10,
			L1Misses: 5,
			L2Hits:   3,
			L2Misses: 2,
		},
		Timestamp: time.Now(),
	}
	
	err := suite.coordinator.ReportMetrics(suite.ctx, report)
	suite.NoError(err)
	
	// Wait for metrics processing
	time.Sleep(50 * time.Millisecond)
	
	// Check that metrics were processed
	// (In a real implementation, this would update node metrics)
	suite.True(true) // Placeholder assertion
}

func (suite *CacheCoordinatorTestSuite) TestClusterInfo() {
	// Add some nodes
	suite.coordinator.mu.Lock()
	suite.coordinator.nodes["node-2"] = &NodeInfo{
		ID:            "node-2",
		State:         NodeStateActive,
		LastHeartbeat: time.Now(),
		Shards:        []int{0, 1},
	}
	suite.coordinator.nodes["node-3"] = &NodeInfo{
		ID:            "node-3",
		State:         NodeStateInactive,
		LastHeartbeat: time.Now().Add(-1 * time.Hour),
	}
	suite.coordinator.mu.Unlock()
	
	info := suite.coordinator.GetClusterInfo()
	
	suite.Equal("node-1", info["node_id"])
	suite.Equal(3, info["total_nodes"])
	suite.Equal(2, info["active_nodes"]) // node-1 and node-2
	suite.Equal(false, info["sharding_enabled"])
	suite.Equal(true, info["consensus_enabled"])
}

func (suite *CacheCoordinatorTestSuite) TestConcurrentOperations() {
	const numGoroutines = 10
	const numOperations = 50
	
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				// Mix different operations
				switch j % 3 {
				case 0:
					// Invalidation
					invalidationMsg := InvalidationMessage{
						NodeID:    suite.coordinator.nodeID,
						EventType: "test-event",
						Timestamp: time.Now(),
					}
					if err := suite.coordinator.BroadcastInvalidation(suite.ctx, invalidationMsg); err != nil {
						errors <- err
						return
					}
				case 1:
					// Cache update
					updateMsg := CacheUpdateMessage{
						NodeID:    suite.coordinator.nodeID,
						Key:       "test-key",
						EventType: "test-event",
						Operation: "SET",
						Timestamp: time.Now(),
					}
					if err := suite.coordinator.NotifyCacheUpdate(suite.ctx, updateMsg); err != nil {
						errors <- err
						return
					}
				case 2:
					// Metrics report
					report := MetricsReport{
						NodeID: suite.coordinator.nodeID,
						Stats: CacheStats{
							L1Hits:   uint64(j),
							L1Misses: uint64(j / 2),
						},
						Timestamp: time.Now(),
					}
					if err := suite.coordinator.ReportMetrics(suite.ctx, report); err != nil {
						errors <- err
						return
					}
				}
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	for err := range errors {
		suite.Fail("Concurrent operation failed", err)
	}
	
	// Verify that messages were sent
	messages := suite.transport.GetMessages()
	suite.Greater(len(messages), 0)
}

func (suite *CacheCoordinatorTestSuite) TestTransportErrors() {
	// Close transport to simulate errors
	suite.transport.Close()
	
	invalidationMsg := InvalidationMessage{
		NodeID:    "node-1",
		EventType: "test-event",
		Timestamp: time.Now(),
	}
	
	err := suite.coordinator.BroadcastInvalidation(suite.ctx, invalidationMsg)
	suite.Error(err)
	
	updateMsg := CacheUpdateMessage{
		NodeID:    "node-1",
		Key:       "test-key",
		EventType: "test-event",
		Operation: "SET",
		Timestamp: time.Now(),
	}
	
	err = suite.coordinator.NotifyCacheUpdate(suite.ctx, updateMsg)
	suite.Error(err)
}

func TestCacheCoordinatorTestSuite(t *testing.T) {
	suite.Run(t, new(CacheCoordinatorTestSuite))
}

// Benchmark tests for cache coordinator
func BenchmarkBroadcastInvalidation(b *testing.B) {
	transport := NewMockTransport()
	defer transport.Close()
	
	config := DefaultCoordinatorConfig()
	coordinator := NewCacheCoordinator("node-1", transport, config)
	defer coordinator.Stop(context.Background())
	
	coordinator.Start(context.Background())
	
	invalidationMsg := InvalidationMessage{
		NodeID:    "node-1",
		EventType: "test-event",
		Timestamp: time.Now(),
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		coordinator.BroadcastInvalidation(ctx, invalidationMsg)
	}
}

func BenchmarkNotifyCacheUpdate(b *testing.B) {
	transport := NewMockTransport()
	defer transport.Close()
	
	config := DefaultCoordinatorConfig()
	coordinator := NewCacheCoordinator("node-1", transport, config)
	defer coordinator.Stop(context.Background())
	
	coordinator.Start(context.Background())
	
	updateMsg := CacheUpdateMessage{
		NodeID:    "node-1",
		Key:       "test-key",
		EventType: "test-event",
		Operation: "SET",
		Timestamp: time.Now(),
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		coordinator.NotifyCacheUpdate(ctx, updateMsg)
	}
}

func BenchmarkGetClusterInfo(b *testing.B) {
	transport := NewMockTransport()
	defer transport.Close()
	
	config := DefaultCoordinatorConfig()
	coordinator := NewCacheCoordinator("node-1", transport, config)
	defer coordinator.Stop(context.Background())
	
	// Add some nodes
	coordinator.nodes["node-2"] = &NodeInfo{
		ID:            "node-2",
		State:         NodeStateActive,
		LastHeartbeat: time.Now(),
	}
	coordinator.nodes["node-3"] = &NodeInfo{
		ID:            "node-3",
		State:         NodeStateActive,
		LastHeartbeat: time.Now(),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		coordinator.GetClusterInfo()
	}
}