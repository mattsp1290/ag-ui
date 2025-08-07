package distributed

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// PartitionDetectionMethod represents how network partitions are detected
type PartitionDetectionMethod string

const (
	// PartitionDetectionHeartbeat uses heartbeat timeouts to detect partitions
	PartitionDetectionHeartbeat PartitionDetectionMethod = "heartbeat"
	// PartitionDetectionQuorum uses quorum loss to detect partitions
	PartitionDetectionQuorum PartitionDetectionMethod = "quorum"
	// PartitionDetectionGossip uses gossip protocol failures
	PartitionDetectionGossip PartitionDetectionMethod = "gossip"
	// PartitionDetectionHybrid uses multiple methods
	PartitionDetectionHybrid PartitionDetectionMethod = "hybrid"
)

// PartitionRecoveryStrategy defines how to recover from partitions
type PartitionRecoveryStrategy string

const (
	// PartitionRecoveryWait waits for partition to heal
	PartitionRecoveryWait PartitionRecoveryStrategy = "wait"
	// PartitionRecoveryMerge attempts to merge diverged states
	PartitionRecoveryMerge PartitionRecoveryStrategy = "merge"
	// PartitionRecoveryReset resets to a known good state
	PartitionRecoveryReset PartitionRecoveryStrategy = "reset"
	// PartitionRecoveryManual requires manual intervention
	PartitionRecoveryManual PartitionRecoveryStrategy = "manual"
)

// PartitionHandlerConfig contains configuration for partition handling
type PartitionHandlerConfig struct {
	// DetectionMethod specifies how to detect partitions
	DetectionMethod PartitionDetectionMethod

	// RecoveryStrategy specifies how to recover from partitions
	RecoveryStrategy PartitionRecoveryStrategy

	// HeartbeatTimeout is the timeout for heartbeat-based detection
	HeartbeatTimeout time.Duration

	// QuorumSize is the minimum nodes required for quorum
	QuorumSize int

	// MaxPartitionDuration is the maximum time to tolerate a partition
	MaxPartitionDuration time.Duration

	// AllowLocalValidation allows local validation during partition
	AllowLocalValidation bool

	// AutoRecovery enables automatic partition recovery
	AutoRecovery bool

	// RecoveryTimeout is the timeout for recovery operations
	RecoveryTimeout time.Duration

	// MinNodesForOperation is the minimum nodes required for operation
	MinNodesForOperation int
}

// DefaultPartitionHandlerConfig returns default partition handler configuration
func DefaultPartitionHandlerConfig() *PartitionHandlerConfig {
	return &PartitionHandlerConfig{
		DetectionMethod:      PartitionDetectionHybrid,
		RecoveryStrategy:     PartitionRecoveryMerge,
		HeartbeatTimeout:     10 * time.Second,
		QuorumSize:           2,
		MaxPartitionDuration: 5 * time.Minute,
		AllowLocalValidation: true,
		AutoRecovery:         true,
		RecoveryTimeout:      30 * time.Second,
		MinNodesForOperation: 1,
	}
}

// PartitionInfo represents information about a network partition
type PartitionInfo struct {
	ID               string                    `json:"id"`
	DetectedAt       time.Time                 `json:"detected_at"`
	AffectedNodes    []NodeID                  `json:"affected_nodes"`
	ReachableNodes   []NodeID                  `json:"reachable_nodes"`
	UnreachableNodes []NodeID                  `json:"unreachable_nodes"`
	Type             PartitionType             `json:"type"`
	Severity         PartitionSeverity         `json:"severity"`
	RecoveryStrategy PartitionRecoveryStrategy `json:"recovery_strategy"`
	RecoveredAt      *time.Time                `json:"recovered_at,omitempty"`
	IsActive         bool                      `json:"is_active"`
}

// PartitionType represents the type of partition
type PartitionType string

const (
	// PartitionTypeMinority indicates this node is in the minority partition
	PartitionTypeMinority PartitionType = "minority"
	// PartitionTypeMajority indicates this node is in the majority partition
	PartitionTypeMajority PartitionType = "majority"
	// PartitionTypeSplit indicates an even split
	PartitionTypeSplit PartitionType = "split"
	// PartitionTypeIsolated indicates this node is isolated
	PartitionTypeIsolated PartitionType = "isolated"
)

// PartitionSeverity represents the severity of a partition
type PartitionSeverity string

const (
	// PartitionSeverityLow - can continue with degraded service
	PartitionSeverityLow PartitionSeverity = "low"
	// PartitionSeverityMedium - limited functionality available
	PartitionSeverityMedium PartitionSeverity = "medium"
	// PartitionSeverityHigh - critical functionality affected
	PartitionSeverityHigh PartitionSeverity = "high"
	// PartitionSeverityCritical - system cannot function
	PartitionSeverityCritical PartitionSeverity = "critical"
)

// PartitionHandler manages network partition detection and recovery
type PartitionHandler struct {
	config *PartitionHandlerConfig
	nodeID NodeID

	// Node tracking
	nodeHealth      map[NodeID]*NodeHealthInfo
	nodeHealthMutex sync.RWMutex

	// Partition tracking
	currentPartition *PartitionInfo
	partitionHistory []*PartitionInfo
	partitionMutex   sync.RWMutex

	// Recovery state
	recoveryInProgress bool
	recoveryMutex      sync.RWMutex

	// Callbacks
	onPartitionDetected  func(*PartitionInfo)
	onPartitionRecovered func(*PartitionInfo)

	// Metrics
	partitionCount   uint64
	recoveryCount    uint64
	failedRecoveries uint64

	// Lifecycle
	running      int32 // Use atomic operations for thread-safe access
	runningMutex sync.RWMutex
	stopChan     chan struct{}
	stopOnce     sync.Once
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
}

// NodeHealthInfo tracks health information for a node
type NodeHealthInfo struct {
	NodeID           NodeID    `json:"node_id"`
	LastHeartbeat    time.Time `json:"last_heartbeat"`
	LastResponse     time.Time `json:"last_response"`
	FailureCount     int       `json:"failure_count"`
	IsReachable      bool      `json:"is_reachable"`
	ResponseTimeMs   float64   `json:"response_time_ms"`
	ConsecutiveFails int       `json:"consecutive_fails"`
}

// NewPartitionHandler creates a new partition handler
func NewPartitionHandler(config *PartitionHandlerConfig, nodeID NodeID) *PartitionHandler {
	if config == nil {
		config = DefaultPartitionHandlerConfig()
	}

	ph := &PartitionHandler{
		config:           config,
		nodeID:           nodeID,
		nodeHealth:       make(map[NodeID]*NodeHealthInfo),
		partitionHistory: make([]*PartitionInfo, 0),
		stopChan:         make(chan struct{}),
	}

	return ph
}

// Start starts the partition handler
func (ph *PartitionHandler) Start(ctx context.Context) error {
	ph.runningMutex.Lock()
	defer ph.runningMutex.Unlock()

	if atomic.LoadInt32(&ph.running) == 1 {
		return fmt.Errorf("partition handler already running")
	}

	// Create a cancellable context for all goroutines
	ph.ctx, ph.cancel = context.WithCancel(ctx)

	// Start detection routines based on method
	ph.startDetectionRoutines(ph.ctx)

	// Start recovery routine if enabled
	ph.startRecoveryRoutine(ph.ctx)

	// Start cleanup routine
	ph.startCleanupRoutine(ph.ctx)

	atomic.StoreInt32(&ph.running, 1)
	return nil
}

// startDetectionRoutines starts the appropriate detection routines
func (ph *PartitionHandler) startDetectionRoutines(ctx context.Context) {
	switch ph.config.DetectionMethod {
	case PartitionDetectionHeartbeat:
		ph.startHeartbeatDetection(ctx)
	case PartitionDetectionQuorum:
		ph.startQuorumDetection(ctx)
	case PartitionDetectionGossip:
		ph.startGossipDetection(ctx)
	case PartitionDetectionHybrid:
		ph.startHeartbeatDetection(ctx)
		ph.startQuorumDetection(ctx)
		ph.startGossipDetection(ctx)
	}
}

// startHeartbeatDetection starts heartbeat detection routine
func (ph *PartitionHandler) startHeartbeatDetection(ctx context.Context) {
	ph.wg.Add(1)
	go func() {
		defer ph.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Panic in partition handler heartbeat detection: %v\n", r)
			}
		}()
		ph.runHeartbeatDetection(ctx)
	}()
}

// startQuorumDetection starts quorum detection routine
func (ph *PartitionHandler) startQuorumDetection(ctx context.Context) {
	ph.wg.Add(1)
	go func() {
		defer ph.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Panic in partition handler quorum detection: %v\n", r)
			}
		}()
		ph.runQuorumDetection(ctx)
	}()
}

// startGossipDetection starts gossip detection routine
func (ph *PartitionHandler) startGossipDetection(ctx context.Context) {
	ph.wg.Add(1)
	go func() {
		defer ph.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Panic in partition handler gossip detection: %v\n", r)
			}
		}()
		ph.runGossipDetection(ctx)
	}()
}

// startRecoveryRoutine starts recovery routine if auto-recovery is enabled
func (ph *PartitionHandler) startRecoveryRoutine(ctx context.Context) {
	if ph.config.AutoRecovery {
		ph.wg.Add(1)
		go func() {
			defer ph.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Panic in partition handler recovery routine: %v\n", r)
				}
			}()
			ph.runRecoveryRoutine(ctx)
		}()
	}
}

// startCleanupRoutine starts cleanup routine
func (ph *PartitionHandler) startCleanupRoutine(ctx context.Context) {
	ph.wg.Add(1)
	go func() {
		defer ph.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Panic in partition handler cleanup routine: %v\n", r)
			}
		}()
		ph.runCleanupRoutine(ctx)
	}()
}

// Stop stops the partition handler
func (ph *PartitionHandler) Stop() error {
	ph.runningMutex.Lock()
	defer ph.runningMutex.Unlock()

	if atomic.LoadInt32(&ph.running) == 0 {
		return nil
	}

	// Cancel the context to signal all goroutines to stop
	if ph.cancel != nil {
		ph.cancel()
	}

	// Close the stop channel as well for backward compatibility
	ph.stopOnce.Do(func() {
		close(ph.stopChan)
	})

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		ph.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines finished
	case <-time.After(500 * time.Millisecond):
		// Shorter timeout, more aggressive
		fmt.Printf("Warning: Partition handler goroutines did not stop within timeout\n")
	}

	atomic.StoreInt32(&ph.running, 0)
	ph.cancel = nil
	ph.ctx = nil
	return nil
}

// IsPartitioned returns whether the node is currently partitioned
func (ph *PartitionHandler) IsPartitioned() bool {
	ph.partitionMutex.RLock()
	defer ph.partitionMutex.RUnlock()

	return ph.currentPartition != nil && ph.currentPartition.IsActive
}

// GetCurrentPartition returns the current partition info
func (ph *PartitionHandler) GetCurrentPartition() *PartitionInfo {
	ph.partitionMutex.RLock()
	defer ph.partitionMutex.RUnlock()

	if ph.currentPartition == nil {
		return nil
	}

	// Return a copy
	partitionCopy := *ph.currentPartition
	return &partitionCopy
}

// GetPartitionHistory returns the partition history
func (ph *PartitionHandler) GetPartitionHistory() []*PartitionInfo {
	ph.partitionMutex.RLock()
	defer ph.partitionMutex.RUnlock()

	// Return a copy
	history := make([]*PartitionInfo, len(ph.partitionHistory))
	for i, p := range ph.partitionHistory {
		partitionCopy := *p
		history[i] = &partitionCopy
	}

	return history
}

// UpdateNodeHealth updates health information for a node
func (ph *PartitionHandler) UpdateNodeHealth(nodeID NodeID, isReachable bool, responseTime time.Duration) {
	// Update health information with lock
	func() {
		ph.nodeHealthMutex.Lock()
		defer ph.nodeHealthMutex.Unlock()

		health, exists := ph.nodeHealth[nodeID]
		if !exists {
			health = &NodeHealthInfo{
				NodeID:      nodeID,
				IsReachable: true,
			}
			ph.nodeHealth[nodeID] = health
		}

		health.LastResponse = time.Now()
		health.ResponseTimeMs = float64(responseTime.Milliseconds())

		if isReachable {
			health.LastHeartbeat = time.Now()
			health.IsReachable = true
			health.ConsecutiveFails = 0
		} else {
			health.FailureCount++
			health.ConsecutiveFails++
			if health.ConsecutiveFails >= 3 {
				health.IsReachable = false
			}
		}
	}()

	// Check for partition after health update (without holding the lock)
	ph.checkForPartition()
}

// HandleNodeFailure handles a node failure
func (ph *PartitionHandler) HandleNodeFailure(nodeID NodeID) {
	ph.UpdateNodeHealth(nodeID, false, 0)
}

// SetPartitionCallbacks sets callbacks for partition events
func (ph *PartitionHandler) SetPartitionCallbacks(onDetected, onRecovered func(*PartitionInfo)) {
	ph.onPartitionDetected = onDetected
	ph.onPartitionRecovered = onRecovered
}

// runHeartbeatDetection runs heartbeat-based partition detection
func (ph *PartitionHandler) runHeartbeatDetection(ctx context.Context) {
	ticker := time.NewTicker(ph.config.HeartbeatTimeout / 3)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ph.stopChan:
			return
		case <-ticker.C:
			ph.checkHeartbeats()
		}
	}
}

// runQuorumDetection runs quorum-based partition detection
func (ph *PartitionHandler) runQuorumDetection(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ph.stopChan:
			return
		case <-ticker.C:
			ph.checkQuorum()
		}
	}
}

// runGossipDetection runs gossip-based partition detection
func (ph *PartitionHandler) runGossipDetection(ctx context.Context) {
	// TODO: Implement gossip-based detection
	// For now, just wait for cancellation
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ph.stopChan:
			return
		case <-ticker.C:
			// Would implement gossip checks here
		}
	}
}

// runRecoveryRoutine runs automatic partition recovery
func (ph *PartitionHandler) runRecoveryRoutine(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ph.stopChan:
			return
		case <-ticker.C:
			ph.attemptRecovery()
		}
	}
}

// runCleanupRoutine cleans up old partition history
func (ph *PartitionHandler) runCleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ph.stopChan:
			return
		case <-ticker.C:
			ph.cleanupHistory()
		}
	}
}

// checkHeartbeats checks node heartbeats for failures
func (ph *PartitionHandler) checkHeartbeats() {
	// Count failed nodes while holding the lock
	failedNodes := 0
	totalNodes := 0

	func() {
		ph.nodeHealthMutex.RLock()
		defer ph.nodeHealthMutex.RUnlock()

		now := time.Now()
		totalNodes = len(ph.nodeHealth)

		for _, health := range ph.nodeHealth {
			if now.Sub(health.LastHeartbeat) > ph.config.HeartbeatTimeout {
				failedNodes++
			}
		}
	}()

	// If too many nodes have failed, we might be partitioned
	// Check outside the lock to avoid potential deadlock
	if totalNodes > 0 && failedNodes > totalNodes/2 {
		ph.detectPartition(PartitionDetectionHeartbeat)
	}
}

// checkQuorum checks if we have quorum
func (ph *PartitionHandler) checkQuorum() {
	// Count reachable nodes while holding the lock
	reachableNodes := 0

	func() {
		ph.nodeHealthMutex.RLock()
		defer ph.nodeHealthMutex.RUnlock()

		for _, health := range ph.nodeHealth {
			if health.IsReachable {
				reachableNodes++
			}
		}
	}()

	// Add self to reachable count
	reachableNodes++

	// Check quorum outside the lock to avoid potential deadlock
	if reachableNodes < ph.config.QuorumSize {
		ph.detectPartition(PartitionDetectionQuorum)
	}
}

// checkForPartition checks if a partition has occurred
func (ph *PartitionHandler) checkForPartition() {
	ph.nodeHealthMutex.RLock()

	reachableNodes := []NodeID{ph.nodeID}
	unreachableNodes := []NodeID{}

	for nodeID, health := range ph.nodeHealth {
		if health.IsReachable {
			reachableNodes = append(reachableNodes, nodeID)
		} else {
			unreachableNodes = append(unreachableNodes, nodeID)
		}
	}

	ph.nodeHealthMutex.RUnlock()

	// Determine if we're partitioned
	if len(unreachableNodes) > 0 && len(reachableNodes) < ph.config.MinNodesForOperation {
		ph.detectPartition(PartitionDetectionHybrid)
	}
}

// detectPartition detects and records a partition
func (ph *PartitionHandler) detectPartition(method PartitionDetectionMethod) {
	ph.partitionMutex.Lock()
	defer ph.partitionMutex.Unlock()

	// Check if we already have an active partition
	if ph.currentPartition != nil && ph.currentPartition.IsActive {
		return
	}

	// Analyze current node health
	reachableNodes, unreachableNodes, totalNodes := ph.analyzeNodeHealth()

	// Create partition info
	partition := ph.createPartitionInfo(reachableNodes, unreachableNodes, totalNodes)

	// Record the partition
	ph.recordPartition(partition)

	// Notify callback
	ph.notifyPartitionDetected(partition)
}

// analyzeNodeHealth analyzes the health of all nodes
func (ph *PartitionHandler) analyzeNodeHealth() ([]NodeID, []NodeID, int) {
	ph.nodeHealthMutex.RLock()
	defer ph.nodeHealthMutex.RUnlock()

	reachableNodes := []NodeID{ph.nodeID}
	unreachableNodes := []NodeID{}
	totalNodes := len(ph.nodeHealth) + 1 // Include self

	for nodeID, health := range ph.nodeHealth {
		if health.IsReachable {
			reachableNodes = append(reachableNodes, nodeID)
		} else {
			unreachableNodes = append(unreachableNodes, nodeID)
		}
	}

	return reachableNodes, unreachableNodes, totalNodes
}

// createPartitionInfo creates partition information from node analysis
func (ph *PartitionHandler) createPartitionInfo(reachableNodes, unreachableNodes []NodeID, totalNodes int) *PartitionInfo {
	// Determine partition type and severity
	partitionType := ph.determinePartitionType(len(reachableNodes), totalNodes)
	severity := ph.determinePartitionSeverity(partitionType, len(reachableNodes))

	return &PartitionInfo{
		ID:               fmt.Sprintf("partition-%d", time.Now().UnixNano()),
		DetectedAt:       time.Now(),
		AffectedNodes:    append(reachableNodes, unreachableNodes...),
		ReachableNodes:   reachableNodes,
		UnreachableNodes: unreachableNodes,
		Type:             partitionType,
		Severity:         severity,
		RecoveryStrategy: ph.config.RecoveryStrategy,
		IsActive:         true,
	}
}

// recordPartition records the partition in history and state
func (ph *PartitionHandler) recordPartition(partition *PartitionInfo) {
	ph.currentPartition = partition
	ph.partitionHistory = append(ph.partitionHistory, partition)
	ph.partitionCount++
}

// notifyPartitionDetected notifies the callback about partition detection
func (ph *PartitionHandler) notifyPartitionDetected(partition *PartitionInfo) {
	if ph.onPartitionDetected != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Panic in partition detected callback: %v\n", r)
				}
			}()
			ph.onPartitionDetected(partition)
		}()
	}
}

// determinePartitionType determines the type of partition
func (ph *PartitionHandler) determinePartitionType(reachableCount, totalCount int) PartitionType {
	if reachableCount == 1 {
		return PartitionTypeIsolated
	}

	majoritySize := (totalCount / 2) + 1

	if reachableCount >= majoritySize {
		return PartitionTypeMajority
	} else if reachableCount == totalCount/2 {
		return PartitionTypeSplit
	} else {
		return PartitionTypeMinority
	}
}

// determinePartitionSeverity determines the severity of a partition
func (ph *PartitionHandler) determinePartitionSeverity(partitionType PartitionType, reachableCount int) PartitionSeverity {
	switch partitionType {
	case PartitionTypeIsolated:
		return PartitionSeverityCritical
	case PartitionTypeMinority:
		if reachableCount >= ph.config.MinNodesForOperation {
			return PartitionSeverityMedium
		}
		return PartitionSeverityHigh
	case PartitionTypeSplit:
		return PartitionSeverityHigh
	case PartitionTypeMajority:
		return PartitionSeverityLow
	default:
		return PartitionSeverityMedium
	}
}

// attemptRecovery attempts to recover from a partition
func (ph *PartitionHandler) attemptRecovery() {
	ph.partitionMutex.RLock()
	partition := ph.currentPartition
	ph.partitionMutex.RUnlock()

	if partition == nil || !partition.IsActive {
		return
	}

	// Check if partition has exceeded maximum duration
	if time.Since(partition.DetectedAt) > ph.config.MaxPartitionDuration {
		// Force recovery or escalate
		ph.forceRecovery(partition)
		return
	}

	ph.recoveryMutex.Lock()
	if ph.recoveryInProgress {
		ph.recoveryMutex.Unlock()
		return
	}
	ph.recoveryInProgress = true
	ph.recoveryMutex.Unlock()

	defer func() {
		ph.recoveryMutex.Lock()
		ph.recoveryInProgress = false
		ph.recoveryMutex.Unlock()
	}()

	// Attempt recovery based on strategy
	var recovered bool
	switch partition.RecoveryStrategy {
	case PartitionRecoveryWait:
		recovered = ph.recoveryWait(partition)
	case PartitionRecoveryMerge:
		recovered = ph.recoveryMerge(partition)
	case PartitionRecoveryReset:
		recovered = ph.recoveryReset(partition)
	case PartitionRecoveryManual:
		// Manual recovery requires external intervention
		recovered = false
	}

	if recovered {
		ph.completeRecovery(partition)
	} else {
		ph.failedRecoveries++
	}
}

// recoveryWait waits for the partition to heal naturally
func (ph *PartitionHandler) recoveryWait(partition *PartitionInfo) bool {
	// Re-check node health
	ph.nodeHealthMutex.RLock()
	unreachableCount := 0

	for _, nodeID := range partition.UnreachableNodes {
		if health, exists := ph.nodeHealth[nodeID]; exists && !health.IsReachable {
			unreachableCount++
		}
	}
	ph.nodeHealthMutex.RUnlock()

	// If all nodes are reachable again, partition has healed
	return unreachableCount == 0
}

// recoveryMerge attempts to merge diverged states
func (ph *PartitionHandler) recoveryMerge(partition *PartitionInfo) bool {
	// TODO: Implement state merge recovery
	// This would involve:
	// 1. Collecting state from all reachable nodes
	// 2. Determining conflicts
	// 3. Resolving conflicts based on strategy
	// 4. Distributing merged state
	return false
}

// recoveryReset resets to a known good state
func (ph *PartitionHandler) recoveryReset(partition *PartitionInfo) bool {
	// TODO: Implement reset recovery
	// This would involve:
	// 1. Identifying a known good state (e.g., last checkpoint)
	// 2. Rolling back to that state
	// 3. Re-synchronizing with other nodes
	return false
}

// forceRecovery forces recovery when partition duration exceeds limits
func (ph *PartitionHandler) forceRecovery(partition *PartitionInfo) {
	// TODO: Implement forced recovery
	// This might involve more aggressive measures like:
	// 1. Forcing a new leader election
	// 2. Resetting connections
	// 3. Escalating to operators
}

// completeRecovery marks a partition as recovered
func (ph *PartitionHandler) completeRecovery(partition *PartitionInfo) {
	ph.partitionMutex.Lock()
	defer ph.partitionMutex.Unlock()

	now := time.Now()
	partition.RecoveredAt = &now
	partition.IsActive = false

	if ph.currentPartition == partition {
		ph.currentPartition = nil
	}

	ph.recoveryCount++

	// Notify callback
	if ph.onPartitionRecovered != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Panic in partition recovered callback: %v\n", r)
				}
			}()
			ph.onPartitionRecovered(partition)
		}()
	}
}

// cleanupHistory removes old partition history entries
func (ph *PartitionHandler) cleanupHistory() {
	ph.partitionMutex.Lock()
	defer ph.partitionMutex.Unlock()

	// Keep only last 100 partitions or partitions from last 24 hours
	cutoff := time.Now().Add(-24 * time.Hour)
	newHistory := make([]*PartitionInfo, 0)

	for _, partition := range ph.partitionHistory {
		if partition.DetectedAt.After(cutoff) || len(newHistory) < 100 {
			newHistory = append(newHistory, partition)
		}
	}

	ph.partitionHistory = newHistory
}

// GetMetrics returns partition handler metrics
func (ph *PartitionHandler) GetMetrics() map[string]interface{} {
	ph.partitionMutex.RLock()
	isPartitioned := ph.currentPartition != nil && ph.currentPartition.IsActive
	partitionDuration := time.Duration(0)
	if isPartitioned && ph.currentPartition != nil {
		partitionDuration = time.Since(ph.currentPartition.DetectedAt)
	}
	ph.partitionMutex.RUnlock()

	return map[string]interface{}{
		"is_partitioned":     isPartitioned,
		"partition_count":    ph.partitionCount,
		"recovery_count":     ph.recoveryCount,
		"failed_recoveries":  ph.failedRecoveries,
		"partition_duration": partitionDuration,
		"history_size":       len(ph.partitionHistory),
	}
}
