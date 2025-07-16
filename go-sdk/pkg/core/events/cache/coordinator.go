package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CacheValidatorInterface defines the interface for cache invalidation coordination
type CacheValidatorInterface interface {
	InvalidateByKeys(ctx context.Context, keys []string) error
	InvalidateEventType(ctx context.Context, eventType string) error
}

// CacheCoordinator coordinates distributed cache operations
type CacheCoordinator struct {
	nodeID        string
	nodes         map[string]*NodeInfo
	transport     Transport
	config        *CoordinatorConfig
	
	// Message channels
	invalidationCh chan InvalidationMessage
	updateCh       chan CacheUpdateMessage
	metricsCh      chan MetricsReport
	
	// State
	clusterState  *ClusterState
	
	// Cache validators for coordination (simple approach for testing)
	cacheValidators map[string]CacheValidatorInterface
	
	// Synchronization
	mu            sync.RWMutex
	shutdownCh    chan struct{}
	wg            sync.WaitGroup
}

// CoordinatorConfig contains coordinator configuration
type CoordinatorConfig struct {
	HeartbeatInterval   time.Duration
	NodeTimeout         time.Duration
	MaxRetries          int
	RetryInterval       time.Duration
	EnableConsensus     bool
	ConsensusQuorum     float64
	EnableSharding      bool
	ShardCount          int
}

// DefaultCoordinatorConfig returns default configuration
func DefaultCoordinatorConfig() *CoordinatorConfig {
	return &CoordinatorConfig{
		HeartbeatInterval: 10 * time.Second,
		NodeTimeout:       30 * time.Second,
		MaxRetries:        3,
		RetryInterval:     1 * time.Second,
		EnableConsensus:   true,
		ConsensusQuorum:   0.51,
		EnableSharding:    false,
		ShardCount:        16,
	}
}

// NodeInfo represents information about a cache node
type NodeInfo struct {
	ID            string
	Address       string
	State         NodeState
	LastHeartbeat time.Time
	Metrics       CacheStats
	Shards        []int
}

// NodeState represents the state of a node
type NodeState int

const (
	NodeStateActive NodeState = iota
	NodeStateInactive
	NodeStateSuspect
	NodeStateFailed
)

// ClusterState maintains cluster-wide state
type ClusterState struct {
	Version       uint64
	Leader        string
	ActiveNodes   []string
	ShardMap      map[int][]string // shard -> nodes
	LastUpdated   time.Time
	mu            sync.RWMutex
}

// Transport interface for node communication
type Transport interface {
	Send(ctx context.Context, nodeID string, message Message) error
	Broadcast(ctx context.Context, message Message) error
	Subscribe(messageType string) <-chan Message
	Close() error
}

// Message represents a coordination message
type Message struct {
	Type      string
	Source    string
	Target    string
	Payload   json.RawMessage
	Timestamp time.Time
}

// InvalidationMessage represents a cache invalidation
type InvalidationMessage struct {
	NodeID    string
	EventType string
	Keys      []string
	Timestamp time.Time
}

// CacheUpdateMessage represents a cache update notification
type CacheUpdateMessage struct {
	NodeID    string
	Key       string
	EventType string
	Operation string
	Timestamp time.Time
}

// MetricsReport represents node metrics
type MetricsReport struct {
	NodeID    string
	Stats     CacheStats
	Timestamp time.Time
}

// ConsensusRequest represents a consensus request
type ConsensusRequest struct {
	ID        string
	Operation string
	Key       string
	Value     interface{}
	Timestamp time.Time
}

// ConsensusResponse represents a consensus response
type ConsensusResponse struct {
	RequestID string
	NodeID    string
	Vote      bool
	Timestamp time.Time
}

// NewCacheCoordinator creates a new cache coordinator
func NewCacheCoordinator(nodeID string, transport Transport, config *CoordinatorConfig) *CacheCoordinator {
	if config == nil {
		config = DefaultCoordinatorConfig()
	}
	
	cc := &CacheCoordinator{
		nodeID:         nodeID,
		nodes:          make(map[string]*NodeInfo),
		transport:      transport,
		config:         config,
		invalidationCh: make(chan InvalidationMessage, 100),
		updateCh:       make(chan CacheUpdateMessage, 100),
		metricsCh:      make(chan MetricsReport, 100),
		clusterState:   &ClusterState{
			ShardMap: make(map[int][]string),
		},
		cacheValidators: make(map[string]CacheValidatorInterface),
		shutdownCh:     make(chan struct{}),
	}
	
	// Add self to nodes
	cc.nodes[nodeID] = &NodeInfo{
		ID:            nodeID,
		State:         NodeStateActive,
		LastHeartbeat: time.Now(),
	}
	
	return cc
}

// Start starts the coordinator
func (cc *CacheCoordinator) Start(ctx context.Context) error {
	// Subscribe to messages
	cc.subscribeToMessages()
	
	// Start workers
	cc.wg.Add(4)
	go cc.heartbeatWorker(ctx)
	go cc.messageProcessor(ctx)
	go cc.healthChecker(ctx)
	go cc.shardManager(ctx)
	
	return nil
}

// Stop stops the coordinator
func (cc *CacheCoordinator) Stop(ctx context.Context) error {
	close(cc.shutdownCh)
	
	// Wait for workers
	done := make(chan struct{})
	go func() {
		cc.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		if cc.transport != nil {
			return cc.transport.Close()
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// BroadcastInvalidation broadcasts a cache invalidation
func (cc *CacheCoordinator) BroadcastInvalidation(ctx context.Context, msg InvalidationMessage) error {
	if cc.transport == nil {
		return fmt.Errorf("transport not initialized")
	}
	
	message := Message{
		Type:      "invalidation",
		Source:    cc.nodeID,
		Timestamp: time.Now(),
	}
	
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal invalidation: %w", err)
	}
	message.Payload = payload
	
	return cc.transport.Broadcast(ctx, message)
}

// NotifyCacheUpdate notifies about a cache update
func (cc *CacheCoordinator) NotifyCacheUpdate(ctx context.Context, msg CacheUpdateMessage) error {
	// For sharded caches, only notify relevant nodes
	if cc.config.EnableSharding {
		shard := cc.getShardForKey(msg.Key)
		nodes := cc.getNodesForShard(shard)
		
		message := Message{
			Type:      "cache_update",
			Source:    cc.nodeID,
			Timestamp: time.Now(),
		}
		
		payload, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal update: %w", err)
		}
		message.Payload = payload
		
		// Send to relevant nodes only
		for _, nodeID := range nodes {
			if nodeID != cc.nodeID && cc.transport != nil {
				if err := cc.transport.Send(ctx, nodeID, message); err != nil {
					// Log error but continue
					continue
				}
			}
		}
		
		return nil
	}
	
	// Broadcast to all nodes
	return cc.broadcastUpdate(ctx, msg)
}

// ReportMetrics reports node metrics
func (cc *CacheCoordinator) ReportMetrics(ctx context.Context, report MetricsReport) error {
	select {
	case cc.metricsCh <- report:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RequestConsensus requests consensus for an operation
func (cc *CacheCoordinator) RequestConsensus(ctx context.Context, request ConsensusRequest) (bool, error) {
	if !cc.config.EnableConsensus {
		return true, nil
	}
	
	if cc.transport == nil {
		return false, fmt.Errorf("transport not initialized")
	}
	
	// Get active nodes
	activeNodes := cc.getActiveNodes()
	requiredVotes := int(float64(len(activeNodes)) * cc.config.ConsensusQuorum)
	
	// Create consensus message
	message := Message{
		Type:      "consensus_request",
		Source:    cc.nodeID,
		Timestamp: time.Now(),
	}
	
	payload, err := json.Marshal(request)
	if err != nil {
		return false, fmt.Errorf("failed to marshal consensus request: %w", err)
	}
	message.Payload = payload
	
	// Broadcast request
	if err := cc.transport.Broadcast(ctx, message); err != nil {
		return false, fmt.Errorf("failed to broadcast consensus request: %w", err)
	}
	
	// Collect votes
	votes := 1 // Self vote
	voteCh := make(chan bool, len(activeNodes))
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	
	for {
		select {
		case vote := <-voteCh:
			if vote {
				votes++
				if votes >= requiredVotes {
					return true, nil
				}
			}
		case <-timeout.C:
			return votes >= requiredVotes, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
}

// GetClusterInfo returns cluster information
func (cc *CacheCoordinator) GetClusterInfo() map[string]interface{} {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	
	activeNodes := 0
	totalShards := 0
	
	for _, node := range cc.nodes {
		if node.State == NodeStateActive {
			activeNodes++
			totalShards += len(node.Shards)
		}
	}
	
	return map[string]interface{}{
		"node_id":      cc.nodeID,
		"total_nodes":  len(cc.nodes),
		"active_nodes": activeNodes,
		"leader":       cc.clusterState.Leader,
		"shard_count":  cc.config.ShardCount,
		"total_shards": totalShards,
		"consensus_enabled": cc.config.EnableConsensus,
		"sharding_enabled":  cc.config.EnableSharding,
	}
}

// RegisterCacheValidator registers a cache validator for coordination
func (cc *CacheCoordinator) RegisterCacheValidator(nodeID string, validator CacheValidatorInterface) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.cacheValidators[nodeID] = validator
}

// UnregisterCacheValidator unregisters a cache validator
func (cc *CacheCoordinator) UnregisterCacheValidator(nodeID string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	delete(cc.cacheValidators, nodeID)
}

// Private methods

func (cc *CacheCoordinator) subscribeToMessages() {
	if cc.transport == nil {
		return
	}
	
	// Subscribe to various message types
	go cc.handleMessages("invalidation", cc.handleInvalidation)
	go cc.handleMessages("cache_update", cc.handleCacheUpdate)
	go cc.handleMessages("heartbeat", cc.handleHeartbeat)
	go cc.handleMessages("consensus_request", cc.handleConsensusRequest)
	go cc.handleMessages("consensus_response", cc.handleConsensusResponse)
	go cc.handleMessages("metrics", cc.handleMetrics)
}

func (cc *CacheCoordinator) handleMessages(messageType string, handler func(Message)) {
	if cc.transport == nil {
		return
	}
	
	ch := cc.transport.Subscribe(messageType)
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			handler(msg)
		case <-cc.shutdownCh:
			return
		}
	}
}

func (cc *CacheCoordinator) handleInvalidation(msg Message) {
	var inv InvalidationMessage
	if err := json.Unmarshal(msg.Payload, &inv); err != nil {
		return
	}
	
	select {
	case cc.invalidationCh <- inv:
	default:
		// Channel full, drop message
	}
}

func (cc *CacheCoordinator) handleCacheUpdate(msg Message) {
	var update CacheUpdateMessage
	if err := json.Unmarshal(msg.Payload, &update); err != nil {
		return
	}
	
	select {
	case cc.updateCh <- update:
	default:
		// Channel full, drop message
	}
}

func (cc *CacheCoordinator) handleHeartbeat(msg Message) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	
	node, exists := cc.nodes[msg.Source]
	if !exists {
		node = &NodeInfo{
			ID:    msg.Source,
			State: NodeStateActive,
		}
		cc.nodes[msg.Source] = node
	}
	
	node.LastHeartbeat = time.Now()
	node.State = NodeStateActive
}

func (cc *CacheCoordinator) handleConsensusRequest(msg Message) {
	var request ConsensusRequest
	if err := json.Unmarshal(msg.Payload, &request); err != nil {
		return
	}
	
	// Process consensus request and send response
	vote := cc.processConsensusRequest(request)
	
	response := ConsensusResponse{
		RequestID: request.ID,
		NodeID:    cc.nodeID,
		Vote:      vote,
		Timestamp: time.Now(),
	}
	
	responseMsg := Message{
		Type:      "consensus_response",
		Source:    cc.nodeID,
		Target:    msg.Source,
		Timestamp: time.Now(),
	}
	
	if payload, err := json.Marshal(response); err == nil {
		responseMsg.Payload = payload
		if cc.transport != nil {
			cc.transport.Send(context.Background(), msg.Source, responseMsg)
		}
	}
}

func (cc *CacheCoordinator) handleConsensusResponse(msg Message) {
	// TODO: Implement consensus response handling
}

func (cc *CacheCoordinator) handleMetrics(msg Message) {
	var report MetricsReport
	if err := json.Unmarshal(msg.Payload, &report); err != nil {
		return
	}
	
	cc.mu.Lock()
	defer cc.mu.Unlock()
	
	if node, exists := cc.nodes[report.NodeID]; exists {
		node.Metrics = report.Stats
	}
}

func (cc *CacheCoordinator) heartbeatWorker(ctx context.Context) {
	defer cc.wg.Done()
	
	ticker := time.NewTicker(cc.config.HeartbeatInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-cc.shutdownCh:
			return
		case <-ticker.C:
			cc.sendHeartbeat(ctx)
		}
	}
}

func (cc *CacheCoordinator) sendHeartbeat(ctx context.Context) {
	if cc.transport == nil {
		return
	}
	
	message := Message{
		Type:      "heartbeat",
		Source:    cc.nodeID,
		Timestamp: time.Now(),
	}
	
	// Include node info in heartbeat
	nodeInfo := map[string]interface{}{
		"id":     cc.nodeID,
		"state":  NodeStateActive,
		"shards": cc.getNodeShards(),
	}
	
	if payload, err := json.Marshal(nodeInfo); err == nil {
		message.Payload = payload
		cc.transport.Broadcast(ctx, message)
	}
}

func (cc *CacheCoordinator) messageProcessor(ctx context.Context) {
	defer cc.wg.Done()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-cc.shutdownCh:
			return
		case inv := <-cc.invalidationCh:
			// Process invalidation
			cc.processInvalidation(inv)
		case update := <-cc.updateCh:
			// Process update
			cc.processCacheUpdate(update)
		case report := <-cc.metricsCh:
			// Process metrics
			cc.processMetrics(report)
		}
	}
}

func (cc *CacheCoordinator) healthChecker(ctx context.Context) {
	defer cc.wg.Done()
	
	ticker := time.NewTicker(cc.config.NodeTimeout / 3)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-cc.shutdownCh:
			return
		case <-ticker.C:
			cc.checkNodeHealth()
		}
	}
}

func (cc *CacheCoordinator) checkNodeHealth() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	
	now := time.Now()
	for nodeID, node := range cc.nodes {
		if nodeID == cc.nodeID {
			continue
		}
		
		timeSinceHeartbeat := now.Sub(node.LastHeartbeat)
		
		switch node.State {
		case NodeStateActive:
			if timeSinceHeartbeat > cc.config.NodeTimeout {
				node.State = NodeStateSuspect
			}
		case NodeStateSuspect:
			if timeSinceHeartbeat > cc.config.NodeTimeout*2 {
				node.State = NodeStateFailed
				cc.handleNodeFailure(nodeID)
			} else if timeSinceHeartbeat < cc.config.HeartbeatInterval*2 {
				node.State = NodeStateActive
			}
		case NodeStateFailed:
			if timeSinceHeartbeat < cc.config.HeartbeatInterval*2 {
				node.State = NodeStateActive
				cc.handleNodeRecovery(nodeID)
			}
		}
	}
}

func (cc *CacheCoordinator) shardManager(ctx context.Context) {
	defer cc.wg.Done()
	
	if !cc.config.EnableSharding {
		return
	}
	
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-cc.shutdownCh:
			return
		case <-ticker.C:
			cc.rebalanceShards()
		}
	}
}

func (cc *CacheCoordinator) rebalanceShards() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	
	activeNodes := cc.getActiveNodesLocked()
	if len(activeNodes) == 0 {
		return
	}
	
	// Simple round-robin shard assignment
	shardsPerNode := cc.config.ShardCount / len(activeNodes)
	remainder := cc.config.ShardCount % len(activeNodes)
	
	cc.clusterState.mu.Lock()
	defer cc.clusterState.mu.Unlock()
	
	cc.clusterState.ShardMap = make(map[int][]string)
	shardIndex := 0
	
	for i, nodeID := range activeNodes {
		shards := shardsPerNode
		if i < remainder {
			shards++
		}
		
		node := cc.nodes[nodeID]
		node.Shards = make([]int, 0, shards)
		
		for j := 0; j < shards; j++ {
			node.Shards = append(node.Shards, shardIndex)
			cc.clusterState.ShardMap[shardIndex] = append(cc.clusterState.ShardMap[shardIndex], nodeID)
			shardIndex++
		}
	}
	
	cc.clusterState.Version++
	cc.clusterState.LastUpdated = time.Now()
}

func (cc *CacheCoordinator) getShardForKey(key string) int {
	if !cc.config.EnableSharding {
		return 0
	}
	
	// Simple hash-based sharding
	hash := 0
	for _, b := range []byte(key) {
		hash = hash*31 + int(b)
	}
	
	return hash % cc.config.ShardCount
}

func (cc *CacheCoordinator) getNodesForShard(shard int) []string {
	cc.clusterState.mu.RLock()
	defer cc.clusterState.mu.RUnlock()
	
	return cc.clusterState.ShardMap[shard]
}

func (cc *CacheCoordinator) getActiveNodes() []string {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	
	return cc.getActiveNodesLocked()
}

func (cc *CacheCoordinator) getActiveNodesLocked() []string {
	active := make([]string, 0)
	for nodeID, node := range cc.nodes {
		if node.State == NodeStateActive {
			active = append(active, nodeID)
		}
	}
	return active
}

func (cc *CacheCoordinator) getNodeShards() []int {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	
	if node, exists := cc.nodes[cc.nodeID]; exists {
		return node.Shards
	}
	return nil
}

func (cc *CacheCoordinator) broadcastUpdate(ctx context.Context, msg CacheUpdateMessage) error {
	if cc.transport == nil {
		return fmt.Errorf("transport not initialized")
	}
	
	message := Message{
		Type:      "cache_update",
		Source:    cc.nodeID,
		Timestamp: time.Now(),
	}
	
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal update: %w", err)
	}
	message.Payload = payload
	
	return cc.transport.Broadcast(ctx, message)
}

func (cc *CacheCoordinator) processConsensusRequest(request ConsensusRequest) bool {
	// Simple voting logic - can be extended
	return true
}

func (cc *CacheCoordinator) processInvalidation(inv InvalidationMessage) {
	// Invalidate caches on all registered validators except the originating node
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	
	for nodeID, validator := range cc.cacheValidators {
		if nodeID != inv.NodeID { // Don't invalidate on the originating node
			// If EventType is specified, invalidate by event type
			if inv.EventType != "" {
				validator.InvalidateEventType(context.Background(), inv.EventType)
			} else if len(inv.Keys) > 0 {
				// Otherwise, invalidate by keys
				validator.InvalidateByKeys(context.Background(), inv.Keys)
			}
		}
	}
}

func (cc *CacheCoordinator) processCacheUpdate(update CacheUpdateMessage) {
	// TODO: Implement update processing
}

func (cc *CacheCoordinator) processMetrics(report MetricsReport) {
	// TODO: Implement metrics processing
}

func (cc *CacheCoordinator) handleNodeFailure(nodeID string) {
	// TODO: Implement node failure handling
	// - Reassign shards
	// - Notify other nodes
}

func (cc *CacheCoordinator) handleNodeRecovery(nodeID string) {
	// TODO: Implement node recovery handling
	// - Rebalance shards
	// - Sync state
}