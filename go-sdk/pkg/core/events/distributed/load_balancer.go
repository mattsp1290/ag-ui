package distributed

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// LoadBalancingAlgorithm represents the load balancing algorithm
type LoadBalancingAlgorithm string

const (
	// LoadBalancingRoundRobin uses round-robin selection
	LoadBalancingRoundRobin LoadBalancingAlgorithm = "round_robin"
	// LoadBalancingLeastConnections selects node with least connections
	LoadBalancingLeastConnections LoadBalancingAlgorithm = "least_connections"
	// LoadBalancingWeightedRoundRobin uses weighted round-robin
	LoadBalancingWeightedRoundRobin LoadBalancingAlgorithm = "weighted_round_robin"
	// LoadBalancingConsistentHash uses consistent hashing
	LoadBalancingConsistentHash LoadBalancingAlgorithm = "consistent_hash"
	// LoadBalancingLeastResponseTime selects node with lowest response time
	LoadBalancingLeastResponseTime LoadBalancingAlgorithm = "least_response_time"
	// LoadBalancingRandom uses random selection
	LoadBalancingRandom LoadBalancingAlgorithm = "random"
)

// LoadBalancerConfig contains configuration for load balancing
type LoadBalancerConfig struct {
	// Algorithm specifies which load balancing algorithm to use
	Algorithm LoadBalancingAlgorithm

	// HealthCheckInterval is the interval between health checks
	HealthCheckInterval time.Duration

	// UnhealthyThreshold is the number of failures before marking unhealthy
	UnhealthyThreshold int

	// HealthyThreshold is the number of successes before marking healthy
	HealthyThreshold int

	// LoadUpdateInterval is the interval for updating load metrics
	LoadUpdateInterval time.Duration

	// MaxLoadPerNode is the maximum load a node should handle (0.0-1.0)
	MaxLoadPerNode float64

	// EnableCircuitBreaker enables circuit breaker functionality
	EnableCircuitBreaker bool

	// CircuitBreakerThreshold is the error rate to trip the circuit
	CircuitBreakerThreshold float64

	// CircuitBreakerTimeout is the timeout before retrying
	CircuitBreakerTimeout time.Duration
}

// DefaultLoadBalancerConfig returns default load balancer configuration
func DefaultLoadBalancerConfig() *LoadBalancerConfig {
	return &LoadBalancerConfig{
		Algorithm:               LoadBalancingLeastResponseTime,
		HealthCheckInterval:     10 * time.Second,
		UnhealthyThreshold:      3,
		HealthyThreshold:        2,
		LoadUpdateInterval:      5 * time.Second,
		MaxLoadPerNode:          0.8,
		EnableCircuitBreaker:    true,
		CircuitBreakerThreshold: 0.5,
		CircuitBreakerTimeout:   30 * time.Second,
	}
}

// NodeLoadInfo tracks load information for a node
type NodeLoadInfo struct {
	NodeID             NodeID    `json:"node_id"`
	CurrentLoad        float64   `json:"current_load"`
	ResponseTimeMs     float64   `json:"response_time_ms"`
	ActiveConnections  int       `json:"active_connections"`
	ProcessedRequests  uint64    `json:"processed_requests"`
	FailedRequests     uint64    `json:"failed_requests"`
	LastUpdated        time.Time `json:"last_updated"`
	IsHealthy          bool      `json:"is_healthy"`
	ConsecutiveFails   int       `json:"consecutive_fails"`
	ConsecutiveSuccess int       `json:"consecutive_success"`
	Weight             int       `json:"weight"`

	// Circuit breaker state
	CircuitState    CircuitState `json:"circuit_state"`
	CircuitOpenedAt *time.Time   `json:"circuit_opened_at,omitempty"`
	ErrorRate       float64      `json:"error_rate"`
}

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	// CircuitClosed allows requests through
	CircuitClosed CircuitState = iota
	// CircuitOpen blocks all requests
	CircuitOpen
	// CircuitHalfOpen allows limited requests for testing
	CircuitHalfOpen
)

// LoadBalancer manages load distribution across validation nodes
type LoadBalancer struct {
	config     *LoadBalancerConfig
	nodes      map[NodeID]*NodeLoadInfo
	nodesMutex sync.RWMutex

	// Round-robin state
	currentIndex int
	indexMutex   sync.Mutex

	// Consistent hash ring
	hashRing *ConsistentHashRing

	// Metrics
	totalRequests uint64
	metrics       *LoadBalancerMetrics

	// Random generator
	rand *rand.Rand
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer(config *LoadBalancerConfig) *LoadBalancer {
	if config == nil {
		config = DefaultLoadBalancerConfig()
	}

	lb := &LoadBalancer{
		config:  config,
		nodes:   make(map[NodeID]*NodeLoadInfo),
		metrics: NewLoadBalancerMetrics(),
		rand:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	// Initialize consistent hash ring if needed
	if config.Algorithm == LoadBalancingConsistentHash {
		lb.hashRing = NewConsistentHashRing()
	}

	return lb
}

// SelectNodes selects nodes for load distribution
func (lb *LoadBalancer) SelectNodes(availableNodes []NodeID, count int) []NodeID {
	if len(availableNodes) == 0 || count <= 0 {
		return []NodeID{}
	}

	// Filter healthy nodes
	healthyNodes := lb.filterHealthyNodes(availableNodes)
	if len(healthyNodes) == 0 {
		// If no healthy nodes, fall back to all available
		healthyNodes = availableNodes
	}

	// Limit count to available nodes
	if count > len(healthyNodes) {
		count = len(healthyNodes)
	}

	// Select nodes based on algorithm
	switch lb.config.Algorithm {
	case LoadBalancingRoundRobin:
		return lb.selectRoundRobin(healthyNodes, count)
	case LoadBalancingLeastConnections:
		return lb.selectLeastConnections(healthyNodes, count)
	case LoadBalancingWeightedRoundRobin:
		return lb.selectWeightedRoundRobin(healthyNodes, count)
	case LoadBalancingConsistentHash:
		return lb.selectConsistentHash(healthyNodes, count)
	case LoadBalancingLeastResponseTime:
		return lb.selectLeastResponseTime(healthyNodes, count)
	case LoadBalancingRandom:
		return lb.selectRandom(healthyNodes, count)
	default:
		return lb.selectRoundRobin(healthyNodes, count)
	}
}

// UpdateNodeMetrics updates metrics for a node
func (lb *LoadBalancer) UpdateNodeMetrics(nodeID NodeID, load float64, responseTimeMs float64) {
	lb.nodesMutex.Lock()
	defer lb.nodesMutex.Unlock()

	info, exists := lb.nodes[nodeID]
	if !exists {
		info = &NodeLoadInfo{
			NodeID:       nodeID,
			IsHealthy:    true,
			Weight:       1,
			CircuitState: CircuitClosed,
		}
		lb.nodes[nodeID] = info
	}

	info.CurrentLoad = load
	info.ResponseTimeMs = responseTimeMs
	info.LastUpdated = time.Now()

	// Update error rate
	if info.ProcessedRequests > 0 {
		info.ErrorRate = float64(info.FailedRequests) / float64(info.ProcessedRequests)
	}

	// Check circuit breaker
	if lb.config.EnableCircuitBreaker {
		lb.updateCircuitBreaker(info)
	}
}

// RecordRequest records a request to a node
func (lb *LoadBalancer) RecordRequest(nodeID NodeID, success bool, responseTime time.Duration) {
	lb.nodesMutex.Lock()
	defer lb.nodesMutex.Unlock()

	info, exists := lb.nodes[nodeID]
	if !exists {
		info = &NodeLoadInfo{
			NodeID:       nodeID,
			IsHealthy:    true,
			Weight:       1,
			CircuitState: CircuitClosed,
		}
		lb.nodes[nodeID] = info
	}

	info.ProcessedRequests++
	info.ResponseTimeMs = float64(responseTime.Milliseconds())

	if success {
		info.ConsecutiveSuccess++
		info.ConsecutiveFails = 0

		// Mark healthy if threshold reached
		if info.ConsecutiveSuccess >= lb.config.HealthyThreshold {
			info.IsHealthy = true
		}
	} else {
		info.FailedRequests++
		info.ConsecutiveFails++
		info.ConsecutiveSuccess = 0

		// Mark unhealthy if threshold reached
		if info.ConsecutiveFails >= lb.config.UnhealthyThreshold {
			info.IsHealthy = false
		}
	}

	// Update error rate
	info.ErrorRate = float64(info.FailedRequests) / float64(info.ProcessedRequests)

	// Update circuit breaker
	if lb.config.EnableCircuitBreaker {
		lb.updateCircuitBreaker(info)
	}

	// Update metrics
	lb.metrics.RecordRequest(nodeID, success, responseTime)
}

// RemoveNode removes a node from the load balancer
func (lb *LoadBalancer) RemoveNode(nodeID NodeID) {
	lb.nodesMutex.Lock()
	defer lb.nodesMutex.Unlock()

	delete(lb.nodes, nodeID)

	// Remove from consistent hash ring if applicable
	if lb.hashRing != nil {
		lb.hashRing.RemoveNode(nodeID)
	}
}

// GetNodeInfo returns information about a specific node
func (lb *LoadBalancer) GetNodeInfo(nodeID NodeID) (*NodeLoadInfo, bool) {
	lb.nodesMutex.RLock()
	defer lb.nodesMutex.RUnlock()

	info, exists := lb.nodes[nodeID]
	if !exists {
		return nil, false
	}

	// Return a copy
	infoCopy := *info
	return &infoCopy, true
}

// GetAllNodesInfo returns information about all nodes
func (lb *LoadBalancer) GetAllNodesInfo() map[NodeID]*NodeLoadInfo {
	lb.nodesMutex.RLock()
	defer lb.nodesMutex.RUnlock()

	// Return a copy
	nodesCopy := make(map[NodeID]*NodeLoadInfo)
	for k, v := range lb.nodes {
		vCopy := *v
		nodesCopy[k] = &vCopy
	}

	return nodesCopy
}

// filterHealthyNodes filters out unhealthy nodes
func (lb *LoadBalancer) filterHealthyNodes(nodes []NodeID) []NodeID {
	lb.nodesMutex.RLock()
	defer lb.nodesMutex.RUnlock()

	healthy := make([]NodeID, 0, len(nodes))

	for _, nodeID := range nodes {
		info, exists := lb.nodes[nodeID]
		if !exists || (info.IsHealthy && info.CircuitState != CircuitOpen) {
			healthy = append(healthy, nodeID)
		}
	}

	return healthy
}

// selectRoundRobin selects nodes using round-robin
func (lb *LoadBalancer) selectRoundRobin(nodes []NodeID, count int) []NodeID {
	if len(nodes) == 0 {
		return []NodeID{}
	}

	selected := make([]NodeID, 0, count)

	lb.indexMutex.Lock()
	defer lb.indexMutex.Unlock()

	for i := 0; i < count; i++ {
		selected = append(selected, nodes[lb.currentIndex%len(nodes)])
		lb.currentIndex++
	}

	return selected
}

// selectLeastConnections selects nodes with least active connections
func (lb *LoadBalancer) selectLeastConnections(nodes []NodeID, count int) []NodeID {
	lb.nodesMutex.RLock()
	defer lb.nodesMutex.RUnlock()

	// Sort nodes by active connections
	type nodeConnection struct {
		nodeID      NodeID
		connections int
	}

	nodeConns := make([]nodeConnection, 0, len(nodes))
	for _, nodeID := range nodes {
		conns := 0
		if info, exists := lb.nodes[nodeID]; exists {
			conns = info.ActiveConnections
		}
		nodeConns = append(nodeConns, nodeConnection{nodeID, conns})
	}

	sort.Slice(nodeConns, func(i, j int) bool {
		return nodeConns[i].connections < nodeConns[j].connections
	})

	// Select nodes with least connections
	selected := make([]NodeID, 0, count)
	for i := 0; i < count && i < len(nodeConns); i++ {
		selected = append(selected, nodeConns[i].nodeID)
	}

	return selected
}

// selectWeightedRoundRobin selects nodes using weighted round-robin
func (lb *LoadBalancer) selectWeightedRoundRobin(nodes []NodeID, count int) []NodeID {
	lb.nodesMutex.RLock()
	defer lb.nodesMutex.RUnlock()

	// Build weighted list
	weightedNodes := make([]NodeID, 0)
	for _, nodeID := range nodes {
		weight := 1
		if info, exists := lb.nodes[nodeID]; exists {
			weight = info.Weight
		}

		// Add node multiple times based on weight
		for i := 0; i < weight; i++ {
			weightedNodes = append(weightedNodes, nodeID)
		}
	}

	if len(weightedNodes) == 0 {
		return []NodeID{}
	}

	// Select from weighted list
	selected := make([]NodeID, 0, count)
	selectedMap := make(map[NodeID]bool)

	lb.indexMutex.Lock()
	defer lb.indexMutex.Unlock()

	for len(selected) < count {
		nodeID := weightedNodes[lb.currentIndex%len(weightedNodes)]
		lb.currentIndex++

		if !selectedMap[nodeID] {
			selected = append(selected, nodeID)
			selectedMap[nodeID] = true
		}
	}

	return selected
}

// selectConsistentHash selects nodes using consistent hashing
func (lb *LoadBalancer) selectConsistentHash(nodes []NodeID, count int) []NodeID {
	if lb.hashRing == nil {
		return lb.selectRandom(nodes, count)
	}

	// Update hash ring with current nodes
	lb.hashRing.UpdateNodes(nodes)

	// Generate a request key (could be based on actual request data)
	requestKey := fmt.Sprintf("request-%d", time.Now().UnixNano())

	// Get nodes from hash ring
	return lb.hashRing.GetNodes(requestKey, count)
}

// selectLeastResponseTime selects nodes with lowest response times
func (lb *LoadBalancer) selectLeastResponseTime(nodes []NodeID, count int) []NodeID {
	lb.nodesMutex.RLock()
	defer lb.nodesMutex.RUnlock()

	// Calculate weighted score for each node
	type nodeScore struct {
		nodeID NodeID
		score  float64
	}

	nodeScores := make([]nodeScore, 0, len(nodes))
	for _, nodeID := range nodes {
		score := math.MaxFloat64

		if info, exists := lb.nodes[nodeID]; exists {
			// Score based on response time and load
			// Lower is better
			score = info.ResponseTimeMs * (1 + info.CurrentLoad)

			// Penalize nodes with high error rates
			score *= (1 + info.ErrorRate)

			// Favor nodes with circuit in closed state
			if info.CircuitState == CircuitOpen {
				score *= 10 // Heavy penalty
			} else if info.CircuitState == CircuitHalfOpen {
				score *= 2 // Moderate penalty
			}
		}

		nodeScores = append(nodeScores, nodeScore{nodeID, score})
	}

	// Sort by score (ascending)
	sort.Slice(nodeScores, func(i, j int) bool {
		return nodeScores[i].score < nodeScores[j].score
	})

	// Select nodes with best scores
	selected := make([]NodeID, 0, count)
	for i := 0; i < count && i < len(nodeScores); i++ {
		selected = append(selected, nodeScores[i].nodeID)
	}

	return selected
}

// selectRandom selects random nodes
func (lb *LoadBalancer) selectRandom(nodes []NodeID, count int) []NodeID {
	if count >= len(nodes) {
		return nodes
	}

	// Shuffle and take first count elements
	shuffled := make([]NodeID, len(nodes))
	copy(shuffled, nodes)

	lb.rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	return shuffled[:count]
}

// updateCircuitBreaker updates circuit breaker state for a node
func (lb *LoadBalancer) updateCircuitBreaker(info *NodeLoadInfo) {
	switch info.CircuitState {
	case CircuitClosed:
		// Check if we should open the circuit
		if info.ErrorRate > lb.config.CircuitBreakerThreshold {
			info.CircuitState = CircuitOpen
			now := time.Now()
			info.CircuitOpenedAt = &now
		}

	case CircuitOpen:
		// Check if we should transition to half-open
		if info.CircuitOpenedAt != nil &&
			time.Since(*info.CircuitOpenedAt) > lb.config.CircuitBreakerTimeout {
			info.CircuitState = CircuitHalfOpen
		}

	case CircuitHalfOpen:
		// Transition based on recent performance
		if info.ConsecutiveSuccess >= lb.config.HealthyThreshold {
			info.CircuitState = CircuitClosed
			info.CircuitOpenedAt = nil
		} else if info.ConsecutiveFails > 0 {
			info.CircuitState = CircuitOpen
			now := time.Now()
			info.CircuitOpenedAt = &now
		}
	}
}

// ConsistentHashRing implements consistent hashing for load distribution
type ConsistentHashRing struct {
	nodes        map[uint32]NodeID
	sortedHashes []uint32
	replicas     int
	mutex        sync.RWMutex
}

// NewConsistentHashRing creates a new consistent hash ring
func NewConsistentHashRing() *ConsistentHashRing {
	return &ConsistentHashRing{
		nodes:    make(map[uint32]NodeID),
		replicas: 150, // Virtual nodes per physical node
	}
}

// UpdateNodes updates the nodes in the hash ring
func (chr *ConsistentHashRing) UpdateNodes(nodes []NodeID) {
	chr.mutex.Lock()
	defer chr.mutex.Unlock()

	// Clear existing nodes
	chr.nodes = make(map[uint32]NodeID)
	chr.sortedHashes = nil

	// Add new nodes
	for _, node := range nodes {
		for i := 0; i < chr.replicas; i++ {
			hash := hashKey(fmt.Sprintf("%s:%d", node, i))
			chr.nodes[hash] = node
			chr.sortedHashes = append(chr.sortedHashes, hash)
		}
	}

	// Sort hashes
	sort.Slice(chr.sortedHashes, func(i, j int) bool {
		return chr.sortedHashes[i] < chr.sortedHashes[j]
	})
}

// RemoveNode removes a node from the hash ring
func (chr *ConsistentHashRing) RemoveNode(nodeID NodeID) {
	chr.mutex.Lock()
	defer chr.mutex.Unlock()

	// Remove all virtual nodes for this node
	newNodes := make(map[uint32]NodeID)
	newHashes := make([]uint32, 0)

	for hash, node := range chr.nodes {
		if node != nodeID {
			newNodes[hash] = node
			newHashes = append(newHashes, hash)
		}
	}

	chr.nodes = newNodes
	chr.sortedHashes = newHashes

	// Re-sort hashes
	sort.Slice(chr.sortedHashes, func(i, j int) bool {
		return chr.sortedHashes[i] < chr.sortedHashes[j]
	})
}

// GetNodes returns nodes for a given key
func (chr *ConsistentHashRing) GetNodes(key string, count int) []NodeID {
	chr.mutex.RLock()
	defer chr.mutex.RUnlock()

	if len(chr.sortedHashes) == 0 {
		return []NodeID{}
	}

	hash := hashKey(key)

	// Binary search for the first hash >= key hash
	idx := sort.Search(len(chr.sortedHashes), func(i int) bool {
		return chr.sortedHashes[i] >= hash
	})

	// Wrap around if necessary
	if idx == len(chr.sortedHashes) {
		idx = 0
	}

	// Collect unique nodes
	selected := make([]NodeID, 0, count)
	selectedMap := make(map[NodeID]bool)

	for i := 0; len(selected) < count && i < len(chr.sortedHashes); i++ {
		hashIdx := (idx + i) % len(chr.sortedHashes)
		nodeID := chr.nodes[chr.sortedHashes[hashIdx]]

		if !selectedMap[nodeID] {
			selected = append(selected, nodeID)
			selectedMap[nodeID] = true
		}
	}

	return selected
}

// hashKey generates a hash for a key
func hashKey(key string) uint32 {
	// Simple FNV-1a hash
	hash := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= 16777619
	}
	return hash
}

// LoadBalancerMetrics tracks load balancer metrics
type LoadBalancerMetrics struct {
	requestsPerNode map[NodeID]uint64
	errorsPerNode   map[NodeID]uint64
	totalRequests   uint64
	totalErrors     uint64
	mutex           sync.RWMutex
}

// NewLoadBalancerMetrics creates new load balancer metrics
func NewLoadBalancerMetrics() *LoadBalancerMetrics {
	return &LoadBalancerMetrics{
		requestsPerNode: make(map[NodeID]uint64),
		errorsPerNode:   make(map[NodeID]uint64),
	}
}

// RecordRequest records a request metric
func (m *LoadBalancerMetrics) RecordRequest(nodeID NodeID, success bool, duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.totalRequests++
	m.requestsPerNode[nodeID]++

	if !success {
		m.totalErrors++
		m.errorsPerNode[nodeID]++
	}
}

// GetMetrics returns current metrics
func (m *LoadBalancerMetrics) GetMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	errorRate := float64(0)
	if m.totalRequests > 0 {
		errorRate = float64(m.totalErrors) / float64(m.totalRequests)
	}

	return map[string]interface{}{
		"total_requests":    m.totalRequests,
		"total_errors":      m.totalErrors,
		"error_rate":        errorRate,
		"requests_per_node": m.requestsPerNode,
		"errors_per_node":   m.errorsPerNode,
	}
}
