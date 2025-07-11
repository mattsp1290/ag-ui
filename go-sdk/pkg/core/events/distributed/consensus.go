package distributed

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ConsensusAlgorithm represents the type of consensus algorithm
type ConsensusAlgorithm string

const (
	// ConsensusRaft uses the Raft consensus algorithm
	ConsensusRaft ConsensusAlgorithm = "raft"
	// ConsensusPBFT uses Practical Byzantine Fault Tolerance
	ConsensusPBFT ConsensusAlgorithm = "pbft"
	// ConsensusMajority uses simple majority voting
	ConsensusMajority ConsensusAlgorithm = "majority"
	// ConsensusUnanimous requires unanimous agreement
	ConsensusUnanimous ConsensusAlgorithm = "unanimous"
)

// ConsensusConfig contains configuration for consensus algorithms
type ConsensusConfig struct {
	// Algorithm specifies which consensus algorithm to use
	Algorithm ConsensusAlgorithm

	// MinNodes is the minimum number of nodes required for consensus
	MinNodes int

	// QuorumSize is the number of nodes required for a quorum
	QuorumSize int

	// RequireUnanimous indicates if unanimous agreement is required
	RequireUnanimous bool

	// ElectionTimeout is the timeout for leader election (Raft)
	ElectionTimeout time.Duration

	// HeartbeatInterval is the interval between heartbeats (Raft)
	HeartbeatInterval time.Duration

	// MaxLogEntries is the maximum number of log entries to keep (Raft)
	MaxLogEntries int

	// ByzantineFaultTolerance is the number of Byzantine faults to tolerate (PBFT)
	ByzantineFaultTolerance int
}

// DefaultConsensusConfig returns default consensus configuration
func DefaultConsensusConfig() *ConsensusConfig {
	return &ConsensusConfig{
		Algorithm:         ConsensusMajority,
		MinNodes:          3,
		QuorumSize:        2,
		RequireUnanimous:  false,
		ElectionTimeout:   5 * time.Second,
		HeartbeatInterval: 1 * time.Second,
		MaxLogEntries:     10000,
		ByzantineFaultTolerance: 1,
	}
}

// ConsensusState represents the state of the consensus protocol
type ConsensusState int

const (
	// ConsensusStateFollower indicates the node is a follower
	ConsensusStateFollower ConsensusState = iota
	// ConsensusStateCandidate indicates the node is a candidate for leadership
	ConsensusStateCandidate
	// ConsensusStateLeader indicates the node is the leader
	ConsensusStateLeader
)

// ConsensusManager manages distributed consensus for validation decisions
type ConsensusManager struct {
	config      *ConsensusConfig
	nodeID      NodeID
	state       ConsensusState
	currentTerm uint64
	votedFor    NodeID
	leader      NodeID
	
	// Log entries for Raft
	log         []LogEntry
	commitIndex uint64
	lastApplied uint64
	
	// State for each node (Raft)
	nextIndex   map[NodeID]uint64
	matchIndex  map[NodeID]uint64
	
	// Voting state
	votes       map[NodeID]bool
	
	// Distributed locks
	locks       map[string]*DistributedLock
	locksMutex  sync.RWMutex
	
	// Lifecycle
	running     bool
	mutex       sync.RWMutex
	stopChan    chan struct{}
	stopOnce    sync.Once
}

// LogEntry represents an entry in the consensus log
type LogEntry struct {
	Term      uint64                 `json:"term"`
	Index     uint64                 `json:"index"`
	Type      LogEntryType           `json:"type"`
	Data      interface{}            `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// LogEntryType represents the type of log entry
type LogEntryType string

const (
	// LogEntryValidation represents a validation decision
	LogEntryValidation LogEntryType = "validation"
	// LogEntryConfiguration represents a configuration change
	LogEntryConfiguration LogEntryType = "configuration"
	// LogEntryHeartbeat represents a heartbeat entry
	LogEntryHeartbeat LogEntryType = "heartbeat"
)

// DistributedLock represents a distributed lock
type DistributedLock struct {
	ID        string    `json:"id"`
	Owner     NodeID    `json:"owner"`
	ExpiresAt time.Time `json:"expires_at"`
	Data      string    `json:"data,omitempty"`
}

// NewConsensusManager creates a new consensus manager
func NewConsensusManager(config *ConsensusConfig, nodeID NodeID) (*ConsensusManager, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	cm := &ConsensusManager{
		config:      config,
		nodeID:      nodeID,
		state:       ConsensusStateFollower,
		currentTerm: 0,
		log:         make([]LogEntry, 0),
		nextIndex:   make(map[NodeID]uint64),
		matchIndex:  make(map[NodeID]uint64),
		votes:       make(map[NodeID]bool),
		locks:       make(map[string]*DistributedLock),
		stopChan:    make(chan struct{}),
	}

	return cm, nil
}

// Start starts the consensus manager
func (cm *ConsensusManager) Start(ctx context.Context) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.running {
		return fmt.Errorf("consensus manager already running")
	}

	// Start consensus protocol based on algorithm
	switch cm.config.Algorithm {
	case ConsensusRaft:
		go cm.runRaft(ctx)
	case ConsensusPBFT:
		go cm.runPBFT(ctx)
	case ConsensusMajority, ConsensusUnanimous:
		// These don't require background routines
	default:
		return fmt.Errorf("unknown consensus algorithm: %s", cm.config.Algorithm)
	}

	cm.running = true
	return nil
}

// Stop stops the consensus manager
func (cm *ConsensusManager) Stop() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if !cm.running {
		return nil
	}

	cm.stopOnce.Do(func() {
		close(cm.stopChan)
	})
	cm.running = false
	return nil
}

// GetRequiredNodes returns the number of nodes required for consensus
func (cm *ConsensusManager) GetRequiredNodes() int {
	switch cm.config.Algorithm {
	case ConsensusUnanimous:
		return cm.config.MinNodes
	case ConsensusPBFT:
		// PBFT requires 3f+1 nodes to tolerate f Byzantine faults
		return 3*cm.config.ByzantineFaultTolerance + 1
	default:
		return cm.config.QuorumSize
	}
}

// AggregateDecisions aggregates validation decisions using the configured consensus algorithm
func (cm *ConsensusManager) AggregateDecisions(decisions []*ValidationDecision) *ValidationResult {
	if len(decisions) == 0 {
		return &ValidationResult{
			IsValid: false,
			Errors: []*ValidationError{{
				RuleID:    "NO_DECISIONS",
				Message:   "No validation decisions available",
				Severity:  ValidationSeverityError,
				Timestamp: time.Now(),
			}},
			EventCount: 1,
			Timestamp:  time.Now(),
		}
	}

	switch cm.config.Algorithm {
	case ConsensusUnanimous:
		return cm.aggregateUnanimous(decisions)
	case ConsensusMajority:
		return cm.aggregateMajority(decisions)
	case ConsensusRaft:
		return cm.aggregateRaft(decisions)
	case ConsensusPBFT:
		return cm.aggregatePBFT(decisions)
	default:
		return cm.aggregateMajority(decisions)
	}
}

// aggregateUnanimous requires all nodes to agree
func (cm *ConsensusManager) aggregateUnanimous(decisions []*ValidationDecision) *ValidationResult {
	result := &ValidationResult{
		IsValid:     true,
		Errors:      make([]*ValidationError, 0),
		Warnings:    make([]*ValidationError, 0),
		Information: make([]*ValidationError, 0),
		EventCount:  1,
		Timestamp:   time.Now(),
	}

	// Check if we have enough decisions
	if len(decisions) < cm.config.MinNodes {
		result.IsValid = false
		result.Errors = append(result.Errors, &ValidationError{
			RuleID:    "INSUFFICIENT_NODES",
			Message:   fmt.Sprintf("Only %d of %d required nodes responded", len(decisions), cm.config.MinNodes),
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		})
		return result
	}

	// All nodes must agree for unanimous consensus
	for _, decision := range decisions {
		if !decision.IsValid {
			result.IsValid = false
		}

		// Collect all errors and warnings
		result.Errors = append(result.Errors, decision.Errors...)
		result.Warnings = append(result.Warnings, decision.Warnings...)
	}

	// Deduplicate errors and warnings
	result.Errors = deduplicateErrors(result.Errors)
	result.Warnings = deduplicateErrors(result.Warnings)

	return result
}

// aggregateMajority uses simple majority voting
func (cm *ConsensusManager) aggregateMajority(decisions []*ValidationDecision) *ValidationResult {
	result := &ValidationResult{
		IsValid:     true,
		Errors:      make([]*ValidationError, 0),
		Warnings:    make([]*ValidationError, 0),
		Information: make([]*ValidationError, 0),
		EventCount:  1,
		Timestamp:   time.Now(),
	}

	// Check if we have enough decisions for a quorum
	if len(decisions) < cm.config.QuorumSize {
		result.IsValid = false
		result.Errors = append(result.Errors, &ValidationError{
			RuleID:    "NO_QUORUM",
			Message:   fmt.Sprintf("Only %d of %d required nodes for quorum", len(decisions), cm.config.QuorumSize),
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		})
		return result
	}

	// Count valid and invalid decisions
	validCount := 0
	invalidCount := 0
	errorMap := make(map[string]*ValidationError)
	warningMap := make(map[string]*ValidationError)

	for _, decision := range decisions {
		if decision.IsValid {
			validCount++
		} else {
			invalidCount++
		}

		// Collect unique errors and warnings
		for _, err := range decision.Errors {
			errorMap[err.RuleID] = err
		}
		for _, warn := range decision.Warnings {
			warningMap[warn.RuleID] = warn
		}
	}

	// Determine consensus result
	if validCount > invalidCount {
		result.IsValid = true
		// Include warnings even if validation passed
		for _, warn := range warningMap {
			result.Warnings = append(result.Warnings, warn)
		}
	} else {
		result.IsValid = false
		// Include all errors
		for _, err := range errorMap {
			result.Errors = append(result.Errors, err)
		}
	}

	return result
}

// aggregateRaft uses Raft consensus for decision aggregation
func (cm *ConsensusManager) aggregateRaft(decisions []*ValidationDecision) *ValidationResult {
	// In Raft, only the leader can make decisions
	cm.mutex.RLock()
	isLeader := cm.state == ConsensusStateLeader
	cm.mutex.RUnlock()

	if !isLeader {
		// Forward to leader or wait for leader decision
		return &ValidationResult{
			IsValid: false,
			Errors: []*ValidationError{{
				RuleID:    "NOT_LEADER",
				Message:   "This node is not the Raft leader",
				Severity:  ValidationSeverityError,
				Timestamp: time.Now(),
			}},
			EventCount: 1,
			Timestamp:  time.Now(),
		}
	}

	// Leader aggregates decisions using majority vote
	return cm.aggregateMajority(decisions)
}

// aggregatePBFT uses PBFT consensus for decision aggregation
func (cm *ConsensusManager) aggregatePBFT(decisions []*ValidationDecision) *ValidationResult {
	// PBFT requires 2f+1 matching decisions to tolerate f Byzantine faults
	requiredMatches := 2*cm.config.ByzantineFaultTolerance + 1

	// Group decisions by result
	decisionGroups := make(map[string][]*ValidationDecision)
	for _, decision := range decisions {
		key := fmt.Sprintf("%v", decision.IsValid)
		decisionGroups[key] = append(decisionGroups[key], decision)
	}

	// Find the group with enough matching decisions
	for _, group := range decisionGroups {
		if len(group) >= requiredMatches {
			// Use the first decision from the matching group as the result
			decision := group[0]
			return &ValidationResult{
				IsValid:     decision.IsValid,
				Errors:      decision.Errors,
				Warnings:    decision.Warnings,
				Information: make([]*ValidationError, 0),
				EventCount:  1,
				Timestamp:   time.Now(),
			}
		}
	}

	// No group has enough matching decisions
	return &ValidationResult{
		IsValid: false,
		Errors: []*ValidationError{{
			RuleID:    "PBFT_NO_CONSENSUS",
			Message:   fmt.Sprintf("Could not achieve Byzantine consensus with %d nodes", len(decisions)),
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		}},
		EventCount: 1,
		Timestamp:  time.Now(),
	}
}

// AcquireLock acquires a distributed lock
func (cm *ConsensusManager) AcquireLock(ctx context.Context, lockID string, duration time.Duration) error {
	cm.locksMutex.Lock()
	defer cm.locksMutex.Unlock()

	// Check if lock already exists
	if lock, exists := cm.locks[lockID]; exists {
		if lock.Owner == cm.nodeID && time.Now().Before(lock.ExpiresAt) {
			// We already own this lock, extend it
			lock.ExpiresAt = time.Now().Add(duration)
			return nil
		}
		if time.Now().Before(lock.ExpiresAt) {
			// Lock is held by another node
			return fmt.Errorf("lock %s is held by node %s", lockID, lock.Owner)
		}
		// Lock has expired, we can take it
	}

	// Create new lock
	cm.locks[lockID] = &DistributedLock{
		ID:        lockID,
		Owner:     cm.nodeID,
		ExpiresAt: time.Now().Add(duration),
	}

	// TODO: Replicate lock to other nodes using consensus protocol

	return nil
}

// ReleaseLock releases a distributed lock
func (cm *ConsensusManager) ReleaseLock(ctx context.Context, lockID string) error {
	cm.locksMutex.Lock()
	defer cm.locksMutex.Unlock()

	lock, exists := cm.locks[lockID]
	if !exists {
		return nil // Lock doesn't exist, nothing to release
	}

	if lock.Owner != cm.nodeID {
		return fmt.Errorf("cannot release lock %s owned by node %s", lockID, lock.Owner)
	}

	delete(cm.locks, lockID)

	// TODO: Replicate lock release to other nodes

	return nil
}

// runRaft implements the Raft consensus protocol
func (cm *ConsensusManager) runRaft(ctx context.Context) {
	electionTimer := time.NewTimer(cm.randomElectionTimeout())
	heartbeatTimer := time.NewTicker(cm.config.HeartbeatInterval)
	defer electionTimer.Stop()
	defer heartbeatTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-cm.stopChan:
			return

		case <-electionTimer.C:
			cm.mutex.Lock()
			if cm.state != ConsensusStateLeader {
				// Start election
				cm.startElection()
			}
			cm.mutex.Unlock()
			electionTimer.Reset(cm.randomElectionTimeout())

		case <-heartbeatTimer.C:
			cm.mutex.RLock()
			isLeader := cm.state == ConsensusStateLeader
			cm.mutex.RUnlock()

			if isLeader {
				cm.sendHeartbeats()
			}
		}
	}
}

// runPBFT implements the PBFT consensus protocol
func (cm *ConsensusManager) runPBFT(ctx context.Context) {
	// TODO: Implement PBFT protocol
	// For now, this is a placeholder
	select {
	case <-ctx.Done():
		return
	case <-cm.stopChan:
		return
	}
}

// startElection starts a new leader election
func (cm *ConsensusManager) startElection() {
	cm.currentTerm++
	cm.state = ConsensusStateCandidate
	cm.votedFor = cm.nodeID
	cm.votes = make(map[NodeID]bool)
	cm.votes[cm.nodeID] = true

	// TODO: Request votes from other nodes
}

// sendHeartbeats sends heartbeats to all followers
func (cm *ConsensusManager) sendHeartbeats() {
	// TODO: Send heartbeat messages to maintain leadership
}

// randomElectionTimeout returns a random election timeout
func (cm *ConsensusManager) randomElectionTimeout() time.Duration {
	// Add some randomness to prevent split votes
	base := cm.config.ElectionTimeout
	jitter := time.Duration(time.Now().UnixNano()%int64(base/2))
	return base + jitter
}

// deduplicateErrors removes duplicate errors based on RuleID
func deduplicateErrors(errors []*ValidationError) []*ValidationError {
	seen := make(map[string]bool)
	result := make([]*ValidationError, 0)

	for _, err := range errors {
		if !seen[err.RuleID] {
			seen[err.RuleID] = true
			result = append(result, err)
		}
	}

	// Sort by severity and timestamp for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		if result[i].Severity != result[j].Severity {
			return result[i].Severity < result[j].Severity
		}
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	return result
}