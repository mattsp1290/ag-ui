// Package main demonstrates distributed state synchronization across multiple nodes
// using the AG-UI state management system.
//
// This example shows:
// - State synchronization across distributed nodes
// - Consensus mechanisms for distributed state
// - Network partition handling
// - Eventual consistency patterns
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/state"
)

// NodeState represents the state of a distributed node
type NodeState struct {
	NodeID       string                 `json:"nodeId"`
	ClusterState ClusterState           `json:"clusterState"`
	LocalData    map[string]interface{} `json:"localData"`
	Metadata     NodeMetadata           `json:"metadata"`
	LastSync     time.Time              `json:"lastSync"`
}

// ClusterState represents the shared cluster state
type ClusterState struct {
	Version      int64                  `json:"version"`
	LeaderID     string                 `json:"leaderId"`
	Members      map[string]NodeInfo    `json:"members"`
	SharedData   map[string]interface{} `json:"sharedData"`
	Consensus    ConsensusInfo          `json:"consensus"`
	LastModified time.Time              `json:"lastModified"`
}

// NodeInfo contains information about a cluster node
type NodeInfo struct {
	ID         string    `json:"id"`
	Address    string    `json:"address"`
	Status     string    `json:"status"` // active, inactive, partitioned
	LastSeen   time.Time `json:"lastSeen"`
	Role       string    `json:"role"` // leader, follower
	DataCenter string    `json:"dataCenter"`
	Zone       string    `json:"zone"`
}

// NodeMetadata contains node-specific metadata
type NodeMetadata struct {
	StartTime    time.Time         `json:"startTime"`
	Version      string            `json:"version"`
	Capabilities []string          `json:"capabilities"`
	Resources    ResourceInfo      `json:"resources"`
	Stats        NodeStats         `json:"stats"`
}

// ResourceInfo contains node resource information
type ResourceInfo struct {
	CPUCores    int     `json:"cpuCores"`
	MemoryGB    float64 `json:"memoryGB"`
	StorageGB   float64 `json:"storageGB"`
	NetworkMbps float64 `json:"networkMbps"`
}

// NodeStats contains node statistics
type NodeStats struct {
	MessagesReceived int64     `json:"messagesReceived"`
	MessagesSent     int64     `json:"messagesSent"`
	SyncOperations   int64     `json:"syncOperations"`
	Conflicts        int64     `json:"conflicts"`
	LastError        string    `json:"lastError"`
	LastErrorTime    time.Time `json:"lastErrorTime"`
}

// ConsensusInfo contains consensus protocol information
type ConsensusInfo struct {
	Algorithm    string    `json:"algorithm"` // raft, paxos, gossip
	Term         int64     `json:"term"`
	CommitIndex  int64     `json:"commitIndex"`
	LastApplied  int64     `json:"lastApplied"`
	VotedFor     string    `json:"votedFor"`
	ElectionTime time.Time `json:"electionTime"`
}

// DistributedNode represents a node in the distributed system
type DistributedNode struct {
	ID              string
	store           *state.StateStore
	eventGen        *state.StateEventGenerator
	eventHandler    *state.StateEventHandler
	resolver        *state.DefaultConflictResolver
	networkSim      *NetworkSimulator
	peers           map[string]*DistributedNode
	isLeader        bool
	isPartitioned   bool
	ctx             context.Context
	cancel          context.CancelFunc
	mu              sync.RWMutex
	messageQueue    chan NetworkMessage
	stats           NodeStats
}

// NetworkMessage represents a message between nodes
type NetworkMessage struct {
	ID        string
	From      string
	To        string
	Type      string // sync, heartbeat, election, data
	Payload   interface{}
	Timestamp time.Time
}

// NetworkSimulator simulates network conditions
type NetworkSimulator struct {
	latencyMs      int
	packetLossRate float64
	partitions     map[string]bool
	mu             sync.RWMutex
}

// DistributedCluster manages the distributed cluster
type DistributedCluster struct {
	nodes       map[string]*DistributedNode
	network     *NetworkSimulator
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
}

func main() {
	// Initialize distributed cluster
	fmt.Println("=== Distributed State Synchronization Demo ===")
	fmt.Println("Creating distributed cluster with multiple nodes...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create network simulator
	network := &NetworkSimulator{
		latencyMs:      50,
		packetLossRate: 0.01, // 1% packet loss
		partitions:     make(map[string]bool),
	}

	// Create distributed cluster
	cluster := &DistributedCluster{
		nodes:   make(map[string]*DistributedNode),
		network: network,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Create nodes in different data centers
	nodeConfigs := []struct {
		ID         string
		DataCenter string
		Zone       string
	}{
		{"node-1", "us-east", "zone-a"},
		{"node-2", "us-east", "zone-b"},
		{"node-3", "us-west", "zone-a"},
		{"node-4", "us-west", "zone-b"},
		{"node-5", "eu-west", "zone-a"},
	}

	fmt.Println("\n=== Creating Cluster Nodes ===")
	for _, config := range nodeConfigs {
		node := cluster.CreateNode(config.ID, config.DataCenter, config.Zone)
		fmt.Printf("Created node %s in %s/%s\n", config.ID, config.DataCenter, config.Zone)
		cluster.nodes[config.ID] = node
	}

	// Establish peer connections
	fmt.Println("\n=== Establishing Peer Connections ===")
	cluster.EstablishPeerConnections()

	// Elect initial leader
	fmt.Println("\n=== Leader Election ===")
	cluster.ElectLeader()

	// Start all nodes
	fmt.Println("\n=== Starting Nodes ===")
	var wg sync.WaitGroup
	for _, node := range cluster.nodes {
		wg.Add(1)
		go func(n *DistributedNode) {
			defer wg.Done()
			n.Start()
		}(node)
	}

	// Wait for nodes to initialize
	time.Sleep(2 * time.Second)

	// Demonstrate various distributed scenarios
	fmt.Println("\n=== Distributed State Operations ===")

	// Scenario 1: Leader writes to shared state
	fmt.Println("\n1. Leader writes to shared state:")
	leader := cluster.GetLeader()
	if leader != nil {
		err := leader.WriteSharedData("config", map[string]interface{}{
			"maxConnections": 1000,
			"timeout":        30,
			"retryCount":     3,
		})
		if err != nil {
			log.Printf("Failed to write shared data: %v", err)
		} else {
			fmt.Println("Leader successfully wrote configuration to shared state")
		}
	}
	time.Sleep(1 * time.Second)

	// Scenario 2: Multiple nodes write to different keys
	fmt.Println("\n2. Concurrent writes to different keys:")
	var writeWg sync.WaitGroup
	for i, node := range cluster.nodes {
		if i >= 3 {
			break // Only first 3 nodes
		}
		writeWg.Add(1)
		go func(n *DistributedNode, idx int) {
			defer writeWg.Done()
			key := fmt.Sprintf("service-%d-status", idx)
			value := map[string]interface{}{
				"healthy":      true,
				"lastCheck":    time.Now(),
				"responseTime": rand.Float64() * 100,
			}
			n.WriteSharedData(key, value)
			fmt.Printf("Node %s wrote %s\n", n.ID, key)
		}(node, i)
	}
	writeWg.Wait()
	time.Sleep(1 * time.Second)

	// Scenario 3: Network partition
	fmt.Println("\n3. Simulating network partition:")
	fmt.Println("Partitioning eu-west from other regions...")
	cluster.SimulatePartition([]string{"node-5"}, []string{"node-1", "node-2", "node-3", "node-4"})
	time.Sleep(2 * time.Second)

	// Write during partition
	fmt.Println("\nWriting data during partition:")
	node5 := cluster.nodes["node-5"]
	node5.WriteLocalData("eu-config", map[string]interface{}{
		"region": "eu-west",
		"gdpr":   true,
	})
	fmt.Println("Node-5 (partitioned) wrote local data")

	leader = cluster.GetLeader()
	if leader != nil {
		leader.WriteSharedData("global-config", map[string]interface{}{
			"version": "2.0",
			"updated": time.Now(),
		})
		fmt.Println("Leader wrote global config during partition")
	}
	time.Sleep(2 * time.Second)

	// Heal partition
	fmt.Println("\n4. Healing network partition:")
	cluster.HealPartition()
	fmt.Println("Network partition healed, nodes reconciling...")
	time.Sleep(3 * time.Second)

	// Scenario 4: Node failure and recovery
	fmt.Println("\n5. Simulating node failure:")
	failedNode := cluster.nodes["node-2"]
	fmt.Printf("Stopping %s...\n", failedNode.ID)
	failedNode.Stop()
	time.Sleep(2 * time.Second)

	// New leader election if necessary
	if failedNode.isLeader {
		fmt.Println("Leader failed, triggering new election...")
		cluster.ElectLeader()
		time.Sleep(1 * time.Second)
	}

	// Restart failed node
	fmt.Printf("\nRestarting %s...\n", failedNode.ID)
	failedNode = cluster.CreateNode("node-2", "us-east", "zone-b")
	cluster.nodes["node-2"] = failedNode
	go failedNode.Start()
	time.Sleep(2 * time.Second)

	// Scenario 5: Conflict resolution
	fmt.Println("\n6. Testing conflict resolution:")
	// Create conflicting writes
	var conflictWg sync.WaitGroup
	conflictKey := "shared-counter"
	
	for i := 0; i < 3; i++ {
		conflictWg.Add(1)
		go func(idx int) {
			defer conflictWg.Done()
			node := cluster.nodes[fmt.Sprintf("node-%d", idx+1)]
			value := map[string]interface{}{
				"count":     idx * 10,
				"updatedBy": node.ID,
				"timestamp": time.Now(),
			}
			node.WriteSharedData(conflictKey, value)
			fmt.Printf("Node %s wrote counter value: %d\n", node.ID, idx*10)
		}(i)
	}
	conflictWg.Wait()
	time.Sleep(2 * time.Second)

	// Show conflict resolution results
	fmt.Println("\nConflict resolution results:")
	for _, node := range cluster.nodes {
		value, err := node.ReadSharedData(conflictKey)
		if err == nil {
			fmt.Printf("Node %s sees value: %v\n", node.ID, value)
		}
	}

	// Scenario 6: Rolling updates
	fmt.Println("\n7. Performing rolling update:")
	cluster.PerformRollingUpdate()

	// Show cluster statistics
	fmt.Println("\n=== Cluster Statistics ===")
	cluster.ShowStatistics()

	// Show state consistency
	fmt.Println("\n=== State Consistency Check ===")
	cluster.CheckStateConsistency()

	// Demonstrate data locality
	fmt.Println("\n=== Data Locality Demo ===")
	cluster.DemonstrateDataLocality()

	// Show network metrics
	fmt.Println("\n=== Network Metrics ===")
	cluster.ShowNetworkMetrics()

	// Graceful shutdown
	fmt.Println("\n=== Graceful Shutdown ===")
	cancel()
	
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("All nodes shut down gracefully")
	case <-time.After(5 * time.Second):
		fmt.Println("Shutdown timeout")
	}
}

// DistributedNode methods

func (n *DistributedNode) Start() {
	fmt.Printf("Node %s starting...\n", n.ID)
	
	// Start message processing
	go n.processMessages()
	
	// Start heartbeat
	go n.sendHeartbeats()
	
	// Start state synchronization
	go n.syncState()
	
	// Start monitoring
	go n.monitor()
}

func (n *DistributedNode) Stop() {
	n.cancel()
	close(n.messageQueue)
}

func (n *DistributedNode) processMessages() {
	for {
		select {
		case <-n.ctx.Done():
			return
		case msg := <-n.messageQueue:
			n.handleMessage(msg)
		}
	}
}

func (n *DistributedNode) handleMessage(msg NetworkMessage) {
	atomic.AddInt64(&n.stats.MessagesReceived, 1)
	
	switch msg.Type {
	case "sync":
		n.handleSyncMessage(msg)
	case "heartbeat":
		n.handleHeartbeat(msg)
	case "election":
		n.handleElectionMessage(msg)
	case "data":
		n.handleDataMessage(msg)
	}
}

func (n *DistributedNode) sendHeartbeats() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.broadcastHeartbeat()
		}
	}
}

func (n *DistributedNode) broadcastHeartbeat() {
	n.mu.RLock()
	defer n.mu.RUnlock()
	
	for peerID, peer := range n.peers {
		if !n.networkSim.IsPartitioned(n.ID, peerID) {
			msg := NetworkMessage{
				ID:        fmt.Sprintf("hb-%d", time.Now().UnixNano()),
				From:      n.ID,
				To:        peerID,
				Type:      "heartbeat",
				Timestamp: time.Now(),
			}
			
			if n.networkSim.ShouldDeliver() {
				go func(p *DistributedNode, m NetworkMessage) {
					time.Sleep(time.Duration(n.networkSim.latencyMs) * time.Millisecond)
					select {
					case p.messageQueue <- m:
					case <-time.After(100 * time.Millisecond):
						// Drop message if queue is full
					}
				}(peer, msg)
			}
		}
	}
}

func (n *DistributedNode) syncState() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.performStateSync()
		}
	}
}

func (n *DistributedNode) performStateSync() {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	atomic.AddInt64(&n.stats.SyncOperations, 1)
	
	// Generate delta since last sync
	if n.eventGen != nil {
		delta, err := n.eventGen.GenerateDeltaFromCurrent()
		if err == nil && len(delta.Delta) > 0 {
			// Broadcast delta to peers
			for peerID, peer := range n.peers {
				if !n.networkSim.IsPartitioned(n.ID, peerID) {
					msg := NetworkMessage{
						ID:        fmt.Sprintf("sync-%d", time.Now().UnixNano()),
						From:      n.ID,
						To:        peerID,
						Type:      "sync",
						Payload:   delta,
						Timestamp: time.Now(),
					}
					
					select {
					case peer.messageQueue <- msg:
						atomic.AddInt64(&n.stats.MessagesSent, 1)
					default:
						// Queue full, skip
					}
				}
			}
		}
	}
	
	// Update last sync time
	n.store.Set("/metadata/lastSync", time.Now())
}

func (n *DistributedNode) handleSyncMessage(msg NetworkMessage) {
	if delta, ok := msg.Payload.(*events.StateDeltaEvent); ok {
		if err := n.eventHandler.HandleStateDelta(delta); err != nil {
			n.mu.Lock()
			n.stats.LastError = fmt.Sprintf("sync error: %v", err)
			n.stats.LastErrorTime = time.Now()
			atomic.AddInt64(&n.stats.Conflicts, 1)
			n.mu.Unlock()
		}
	}
}

func (n *DistributedNode) handleHeartbeat(msg NetworkMessage) {
	// Update peer's last seen time
	n.store.Set(fmt.Sprintf("/clusterState/members/%s/lastSeen", msg.From), time.Now())
}

func (n *DistributedNode) handleElectionMessage(msg NetworkMessage) {
	// Simple leader election - highest ID wins
	n.mu.Lock()
	defer n.mu.Unlock()
	
	if msg.From > n.ID {
		n.isLeader = false
		n.store.Set("/clusterState/leaderId", msg.From)
	}
}

func (n *DistributedNode) handleDataMessage(msg NetworkMessage) {
	if data, ok := msg.Payload.(map[string]interface{}); ok {
		for key, value := range data {
			n.store.Set(fmt.Sprintf("/clusterState/sharedData/%s", key), value)
		}
	}
}

func (n *DistributedNode) monitor() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.updateNodeStats()
		}
	}
}

func (n *DistributedNode) updateNodeStats() {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	statsData := map[string]interface{}{
		"messagesReceived": atomic.LoadInt64(&n.stats.MessagesReceived),
		"messagesSent":     atomic.LoadInt64(&n.stats.MessagesSent),
		"syncOperations":   atomic.LoadInt64(&n.stats.SyncOperations),
		"conflicts":        atomic.LoadInt64(&n.stats.Conflicts),
		"lastError":        n.stats.LastError,
		"lastErrorTime":    n.stats.LastErrorTime,
	}
	
	n.store.Set("/metadata/stats", statsData)
}

func (n *DistributedNode) WriteSharedData(key string, value interface{}) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	
	if n.isPartitioned {
		return fmt.Errorf("node is partitioned")
	}
	
	// Write to local store
	err := n.store.Set(fmt.Sprintf("/clusterState/sharedData/%s", key), value)
	if err != nil {
		return err
	}
	
	// Broadcast to peers
	msg := NetworkMessage{
		ID:        fmt.Sprintf("data-%d", time.Now().UnixNano()),
		From:      n.ID,
		To:        "broadcast",
		Type:      "data",
		Payload:   map[string]interface{}{key: value},
		Timestamp: time.Now(),
	}
	
	for _, peer := range n.peers {
		select {
		case peer.messageQueue <- msg:
		default:
		}
	}
	
	return nil
}

func (n *DistributedNode) WriteLocalData(key string, value interface{}) error {
	return n.store.Set(fmt.Sprintf("/localData/%s", key), value)
}

func (n *DistributedNode) ReadSharedData(key string) (interface{}, error) {
	return n.store.Get(fmt.Sprintf("/clusterState/sharedData/%s", key))
}

// NetworkSimulator methods

func (ns *NetworkSimulator) IsPartitioned(node1, node2 string) bool {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	
	return ns.partitions[node1] || ns.partitions[node2]
}

func (ns *NetworkSimulator) ShouldDeliver() bool {
	return rand.Float64() > ns.packetLossRate
}

func (ns *NetworkSimulator) SetPartition(nodeID string, partitioned bool) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	
	ns.partitions[nodeID] = partitioned
}

// DistributedCluster methods

func (c *DistributedCluster) CreateNode(id, dataCenter, zone string) *DistributedNode {
	nodeCtx, nodeCancel := context.WithCancel(c.ctx)
	
	node := &DistributedNode{
		ID:            id,
		store:         state.NewStateStore(state.WithMaxHistory(100)),
		networkSim:    c.network,
		peers:         make(map[string]*DistributedNode),
		ctx:           nodeCtx,
		cancel:        nodeCancel,
		messageQueue:  make(chan NetworkMessage, 1000),
	}
	
	// Initialize node state
	initialState := NodeState{
		NodeID: id,
		ClusterState: ClusterState{
			Version:  1,
			Members:  make(map[string]NodeInfo),
			SharedData: make(map[string]interface{}),
			Consensus: ConsensusInfo{
				Algorithm: "raft",
				Term:      1,
			},
			LastModified: time.Now(),
		},
		LocalData: make(map[string]interface{}),
		Metadata: NodeMetadata{
			StartTime:    time.Now(),
			Version:      "1.0.0",
			Capabilities: []string{"storage", "compute"},
			Resources: ResourceInfo{
				CPUCores:    4,
				MemoryGB:    16,
				StorageGB:   100,
				NetworkMbps: 1000,
			},
		},
		LastSync: time.Now(),
	}
	
	// Add self to members
	initialState.ClusterState.Members[id] = NodeInfo{
		ID:         id,
		Address:    fmt.Sprintf("%s.cluster.local:8080", id),
		Status:     "active",
		LastSeen:   time.Now(),
		Role:       "follower",
		DataCenter: dataCenter,
		Zone:       zone,
	}
	
	// Set initial state
	stateData, _ := json.Marshal(initialState)
	var stateMap map[string]interface{}
	json.Unmarshal(stateData, &stateMap)
	
	for key, value := range stateMap {
		node.store.Set("/"+key, value)
	}
	
	// Create event generator and handler
	node.eventGen = state.NewStateEventGenerator(node.store)
	node.eventHandler = state.NewStateEventHandler(
		node.store,
		state.WithBatchSize(10),
		state.WithBatchTimeout(100*time.Millisecond),
	)
	
	// Create conflict resolver
	node.resolver = state.NewDefaultConflictResolver(
		state.WithResolutionStrategy(state.LastWriteWins),
	)
	
	return node
}

func (c *DistributedCluster) EstablishPeerConnections() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Connect all nodes to each other
	for id1, node1 := range c.nodes {
		for id2, node2 := range c.nodes {
			if id1 != id2 {
				node1.peers[id2] = node2
			}
		}
	}
}

func (c *DistributedCluster) ElectLeader() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Simple election - node with highest ID becomes leader
	var leaderID string
	for id := range c.nodes {
		if id > leaderID {
			leaderID = id
		}
	}
	
	// Update leader status
	for id, node := range c.nodes {
		if id == leaderID {
			node.isLeader = true
			node.store.Set("/clusterState/leaderId", leaderID)
			node.store.Set(fmt.Sprintf("/clusterState/members/%s/role", id), "leader")
			fmt.Printf("Node %s elected as leader\n", id)
		} else {
			node.isLeader = false
			node.store.Set(fmt.Sprintf("/clusterState/members/%s/role", id), "follower")
		}
	}
}

func (c *DistributedCluster) GetLeader() *DistributedNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	for _, node := range c.nodes {
		if node.isLeader {
			return node
		}
	}
	return nil
}

func (c *DistributedCluster) SimulatePartition(group1, group2 []string) {
	for _, nodeID := range group1 {
		c.network.SetPartition(nodeID, true)
		if node, exists := c.nodes[nodeID]; exists {
			node.isPartitioned = true
		}
	}
}

func (c *DistributedCluster) HealPartition() {
	c.network.mu.Lock()
	for nodeID := range c.network.partitions {
		c.network.partitions[nodeID] = false
	}
	c.network.mu.Unlock()
	
	c.mu.Lock()
	for _, node := range c.nodes {
		node.isPartitioned = false
	}
	c.mu.Unlock()
}

func (c *DistributedCluster) PerformRollingUpdate() {
	fmt.Println("Starting rolling update...")
	
	updateVersion := "2.0.0"
	for _, node := range c.nodes {
		fmt.Printf("Updating node %s to version %s\n", node.ID, updateVersion)
		
		// Simulate update
		node.store.Set("/metadata/version", updateVersion)
		node.store.Set("/metadata/capabilities", []string{"storage", "compute", "analytics"})
		
		time.Sleep(500 * time.Millisecond) // Simulate update time
	}
	
	fmt.Println("Rolling update completed")
}

func (c *DistributedCluster) ShowStatistics() {
	for _, node := range c.nodes {
		stats, _ := node.store.Get("/metadata/stats")
		fmt.Printf("\nNode %s statistics:\n", node.ID)
		if statsMap, ok := stats.(map[string]interface{}); ok {
			fmt.Printf("  Messages received: %v\n", statsMap["messagesReceived"])
			fmt.Printf("  Messages sent: %v\n", statsMap["messagesSent"])
			fmt.Printf("  Sync operations: %v\n", statsMap["syncOperations"])
			fmt.Printf("  Conflicts: %v\n", statsMap["conflicts"])
		}
		fmt.Printf("  Is Leader: %v\n", node.isLeader)
		fmt.Printf("  Is Partitioned: %v\n", node.isPartitioned)
	}
}

func (c *DistributedCluster) CheckStateConsistency() {
	fmt.Println("Checking state consistency across nodes...")
	
	// Compare shared data across all nodes
	sharedKeys := make(map[string]map[string]interface{})
	
	for _, node := range c.nodes {
		sharedData, err := node.store.Get("/clusterState/sharedData")
		if err == nil {
			if data, ok := sharedData.(map[string]interface{}); ok {
				sharedKeys[node.ID] = data
			}
		}
	}
	
	// Check for inconsistencies
	inconsistencies := 0
	for key := range sharedKeys[c.nodes["node-1"].ID] {
		values := make(map[string]interface{})
		for nodeID, data := range sharedKeys {
			if val, exists := data[key]; exists {
				values[nodeID] = val
			}
		}
		
		// Check if all values are the same
		var firstVal interface{}
		consistent := true
		for _, val := range values {
			if firstVal == nil {
				firstVal = val
			} else if !reflect.DeepEqual(firstVal, val) {
				consistent = false
				inconsistencies++
				break
			}
		}
		
		if !consistent {
			fmt.Printf("  Inconsistency found for key '%s'\n", key)
			for nodeID, val := range values {
				fmt.Printf("    %s: %v\n", nodeID, val)
			}
		}
	}
	
	if inconsistencies == 0 {
		fmt.Println("  All nodes have consistent shared state")
	} else {
		fmt.Printf("  Found %d inconsistencies\n", inconsistencies)
	}
}

func (c *DistributedCluster) DemonstrateDataLocality() {
	fmt.Println("Demonstrating data locality optimization...")
	
	// Each region stores region-specific data
	regions := map[string][]string{
		"us-east": {"node-1", "node-2"},
		"us-west": {"node-3", "node-4"},
		"eu-west": {"node-5"},
	}
	
	for region, nodes := range regions {
		for _, nodeID := range nodes {
			if node, exists := c.nodes[nodeID]; exists {
				// Store region-specific data
				regionData := map[string]interface{}{
					"region":     region,
					"regulation": getRegionRegulation(region),
					"currency":   getRegionCurrency(region),
					"timezone":   getRegionTimezone(region),
				}
				
				node.WriteLocalData(fmt.Sprintf("%s-config", region), regionData)
				fmt.Printf("  Node %s storing %s region data locally\n", nodeID, region)
			}
		}
	}
}

func (c *DistributedCluster) ShowNetworkMetrics() {
	fmt.Printf("Network latency: %dms\n", c.network.latencyMs)
	fmt.Printf("Packet loss rate: %.2f%%\n", c.network.packetLossRate*100)
	
	// Calculate total message volume
	var totalReceived, totalSent int64
	for _, node := range c.nodes {
		totalReceived += atomic.LoadInt64(&node.stats.MessagesReceived)
		totalSent += atomic.LoadInt64(&node.stats.MessagesSent)
	}
	
	fmt.Printf("Total messages sent: %d\n", totalSent)
	fmt.Printf("Total messages received: %d\n", totalReceived)
	
	if totalSent > 0 {
		deliveryRate := float64(totalReceived) / float64(totalSent) * 100
		fmt.Printf("Message delivery rate: %.2f%%\n", deliveryRate)
	}
}

// Helper functions

func getRegionRegulation(region string) string {
	regulations := map[string]string{
		"us-east": "SOC2",
		"us-west": "CCPA",
		"eu-west": "GDPR",
	}
	return regulations[region]
}

func getRegionCurrency(region string) string {
	currencies := map[string]string{
		"us-east": "USD",
		"us-west": "USD",
		"eu-west": "EUR",
	}
	return currencies[region]
}

func getRegionTimezone(region string) string {
	timezones := map[string]string{
		"us-east": "America/New_York",
		"us-west": "America/Los_Angeles",
		"eu-west": "Europe/London",
	}
	return timezones[region]
}