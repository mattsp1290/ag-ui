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
		MinNodes:          1, // Allow single-node scenarios for testing
		QuorumSize:        1, // Single node is sufficient for quorum in testing
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

	// Signal stop to all background goroutines
	cm.stopOnce.Do(func() {
		close(cm.stopChan)
	})
	
	// Give background goroutines time to finish
	// The runRaft and runPBFT methods check stopChan and should exit cleanly
	time.Sleep(50 * time.Millisecond)
	
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
	result := cm.initializeValidationResult()

	// Check if we have enough decisions for a quorum
	if !cm.hasQuorum(decisions, result) {
		return result
	}

	// Count and categorize decisions
	validCount, invalidCount, errorMap, warningMap := cm.categorizeDecisions(decisions)

	// Determine consensus result
	cm.determineMajorityResult(result, validCount, invalidCount, errorMap, warningMap)

	return result
}

// initializeValidationResult creates a new ValidationResult with default values
func (cm *ConsensusManager) initializeValidationResult() *ValidationResult {
	return &ValidationResult{
		IsValid:     true,
		Errors:      make([]*ValidationError, 0),
		Warnings:    make([]*ValidationError, 0),
		Information: make([]*ValidationError, 0),
		EventCount:  1,
		Timestamp:   time.Now(),
	}
}

// hasQuorum checks if we have enough decisions for a quorum
func (cm *ConsensusManager) hasQuorum(decisions []*ValidationDecision, result *ValidationResult) bool {
	if len(decisions) < cm.config.QuorumSize {
		result.IsValid = false
		result.Errors = append(result.Errors, &ValidationError{
			RuleID:    "NO_QUORUM",
			Message:   fmt.Sprintf("Only %d of %d required nodes for quorum", len(decisions), cm.config.QuorumSize),
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		})
		return false
	}
	return true
}

// categorizeDecisions counts valid/invalid decisions and collects errors/warnings
func (cm *ConsensusManager) categorizeDecisions(decisions []*ValidationDecision) (int, int, map[string]*ValidationError, map[string]*ValidationError) {
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

	return validCount, invalidCount, errorMap, warningMap
}

// determineMajorityResult determines the final result based on majority voting
func (cm *ConsensusManager) determineMajorityResult(result *ValidationResult, validCount, invalidCount int, errorMap, warningMap map[string]*ValidationError) {
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

// AcquireLock acquires a distributed lock asynchronously
func (cm *ConsensusManager) AcquireLock(ctx context.Context, lockID string, duration time.Duration) error {
	// Create timeout context for lock acquisition - use reasonable timeout based on context deadline
	lockTimeout := 5 * time.Second // Default timeout
	if deadline, ok := ctx.Deadline(); ok {
		// Use remaining time minus buffer
		remaining := time.Until(deadline)
		if remaining > 1*time.Second {
			lockTimeout = remaining - 500*time.Millisecond
		}
	}
	
	lockCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	// Use a channel to signal lock acquisition result
	resultChan := make(chan error, 1)
	
	go func() {
		defer func() {
			if r := recover(); r != nil {
				select {
				case resultChan <- fmt.Errorf("panic in lock acquisition: %v", r):
				default:
				}
			}
		}()
		
		err := cm.acquireLockInternal(lockCtx, lockID, duration)
		select {
		case resultChan <- err:
		case <-lockCtx.Done():
		}
	}()

	select {
	case <-lockCtx.Done():
		return fmt.Errorf("lock acquisition timeout for %s", lockID)
	case err := <-resultChan:
		return err
	}
}

// acquireLockInternal performs the actual lock acquisition logic
func (cm *ConsensusManager) acquireLockInternal(ctx context.Context, lockID string, duration time.Duration) error {
	// First, check if we need to wait for an existing lock without holding the write lock
	cm.locksMutex.RLock()
	lock, exists := cm.locks[lockID]
	cm.locksMutex.RUnlock()
	
	if exists {
		// Check if we already own this lock
		if lock.Owner == cm.nodeID && time.Now().Before(lock.ExpiresAt) {
			// We already own this lock, extend it atomically
			cm.locksMutex.Lock()
			// Double-check under write lock to prevent race condition
			if existingLock, stillExists := cm.locks[lockID]; stillExists && existingLock.Owner == cm.nodeID {
				existingLock.ExpiresAt = time.Now().Add(duration)
				cm.locksMutex.Unlock()
				return nil
			}
			cm.locksMutex.Unlock()
			// Lock state changed, fall through to recheck
		}
		
		// Check if lock is still held by another node
		if time.Now().Before(lock.ExpiresAt) {
			// Lock is held by another node, wait for it to expire or be released
			// Use context timeout for waiting
			waitTimeout := duration
			if deadline, ok := ctx.Deadline(); ok {
				remaining := time.Until(deadline)
				if remaining < waitTimeout {
					waitTimeout = remaining
				}
			}
			if err := cm.waitForLockRelease(ctx, lockID, waitTimeout); err != nil {
				return err
			}
			// After waiting, we need to try to acquire the lock again
		}
		// Lock has expired, we can try to take it
	}

	// Try to acquire the lock atomically
	cm.locksMutex.Lock()
	defer cm.locksMutex.Unlock()
	
	// Double-check lock state under write lock
	if existingLock, exists := cm.locks[lockID]; exists {
		if existingLock.Owner == cm.nodeID && time.Now().Before(existingLock.ExpiresAt) {
			// We already own this lock, extend it
			existingLock.ExpiresAt = time.Now().Add(duration)
			return nil
		}
		if time.Now().Before(existingLock.ExpiresAt) {
			// Lock is still held by another node
			return fmt.Errorf("lock %s is held by node %s until %v", lockID, existingLock.Owner, existingLock.ExpiresAt)
		}
		// Lock has expired, we can take it
	}

	// Create new lock
	newLock := &DistributedLock{
		ID:        lockID,
		Owner:     cm.nodeID,
		ExpiresAt: time.Now().Add(duration),
	}

	// Replicate lock to other nodes asynchronously
	if err := cm.replicateLockAsync(ctx, newLock); err != nil {
		return fmt.Errorf("failed to replicate lock: %w", err)
	}

	cm.locks[lockID] = newLock
	return nil
}

// ReleaseLock releases a distributed lock asynchronously
func (cm *ConsensusManager) ReleaseLock(ctx context.Context, lockID string) error {
	// Create timeout context for lock release
	releaseCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Use a channel to signal lock release result
	resultChan := make(chan error, 1)
	
	go func() {
		defer func() {
			if r := recover(); r != nil {
				select {
				case resultChan <- fmt.Errorf("panic in lock release: %v", r):
				default:
				}
			}
		}()
		
		err := cm.releaseLockInternal(releaseCtx, lockID)
		select {
		case resultChan <- err:
		case <-releaseCtx.Done():
		}
	}()

	select {
	case <-releaseCtx.Done():
		return fmt.Errorf("lock release timeout for %s", lockID)
	case err := <-resultChan:
		return err
	}
}

// releaseLockInternal performs the actual lock release logic
func (cm *ConsensusManager) releaseLockInternal(ctx context.Context, lockID string) error {
	// Check if we own the lock before acquiring write lock
	cm.locksMutex.RLock()
	lock, exists := cm.locks[lockID]
	cm.locksMutex.RUnlock()
	
	if !exists {
		return nil // Lock doesn't exist, nothing to release
	}

	if lock.Owner != cm.nodeID {
		return fmt.Errorf("cannot release lock %s owned by node %s", lockID, lock.Owner)
	}

	// Acquire write lock to perform the release
	cm.locksMutex.Lock()
	defer cm.locksMutex.Unlock()
	
	// Double-check under write lock
	lock, exists = cm.locks[lockID]
	if !exists {
		return nil // Lock was already released
	}

	if lock.Owner != cm.nodeID {
		return fmt.Errorf("cannot release lock %s owned by node %s", lockID, lock.Owner)
	}

	// Replicate lock release to other nodes asynchronously
	if err := cm.replicateLockReleaseAsync(ctx, lockID); err != nil {
		return fmt.Errorf("failed to replicate lock release: %w", err)
	}

	delete(cm.locks, lockID)
	return nil
}

// waitForLockRelease waits for a lock to be released or expired
func (cm *ConsensusManager) waitForLockRelease(ctx context.Context, lockID string, duration time.Duration) error {
	ticker := time.NewTicker(50 * time.Millisecond) // Poll more frequently
	defer ticker.Stop()

	// Use context deadline if available, otherwise use duration
	deadline := time.Now().Add(duration)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			cm.locksMutex.RLock()
			lock, exists := cm.locks[lockID]
			cm.locksMutex.RUnlock()
			
			if !exists || time.Now().After(lock.ExpiresAt) {
				// Lock is now available, try to acquire it
				return nil
			}
			
			if time.Now().After(deadline) {
				return fmt.Errorf("lock wait timeout for %s", lockID)
			}
		}
	}
}

// replicateLockAsync replicates a lock to other nodes asynchronously
func (cm *ConsensusManager) replicateLockAsync(ctx context.Context, lock *DistributedLock) error {
	// Create timeout context for replication to prevent indefinite blocking
	replicationCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond) // Shorter timeout for simulation
	defer cancel()
	
	// TODO: Implement actual network replication
	// For now, simulate synchronous replication with proper cancellation
	select {
	case <-replicationCtx.Done():
		return fmt.Errorf("lock replication timeout: %w", replicationCtx.Err())
	case <-time.After(10 * time.Millisecond): // Simulate network delay
		// Check context one more time
		select {
		case <-replicationCtx.Done():
			return replicationCtx.Err()
		default:
			return nil // Success
		}
	}
}

// replicateLockReleaseAsync replicates a lock release to other nodes asynchronously
func (cm *ConsensusManager) replicateLockReleaseAsync(ctx context.Context, lockID string) error {
	// Create timeout context for replication to prevent indefinite blocking
	releaseCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond) // Shorter timeout for simulation
	defer cancel()
	
	// TODO: Implement actual network replication
	// For now, simulate synchronous replication with proper cancellation
	select {
	case <-releaseCtx.Done():
		return fmt.Errorf("lock release replication timeout: %w", releaseCtx.Err())
	case <-time.After(10 * time.Millisecond): // Simulate network delay
		// Check context one more time
		select {
		case <-releaseCtx.Done():
			return releaseCtx.Err()
		default:
			return nil // Success
		}
	}
}

// runRaft implements the Raft consensus protocol
func (cm *ConsensusManager) runRaft(ctx context.Context) {
	electionTimer := time.NewTimer(cm.randomElectionTimeout())
	heartbeatTimer := time.NewTicker(cm.config.HeartbeatInterval)
	
	// Ensure timers are properly stopped to prevent goroutine leaks
	defer func() {
		if !electionTimer.Stop() {
			// Drain the timer channel if it wasn't stopped
			select {
			case <-electionTimer.C:
			default:
			}
		}
		heartbeatTimer.Stop()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-cm.stopChan:
			return

		case <-electionTimer.C:
			// Check if we should still be running before acquiring lock
			select {
			case <-ctx.Done():
				return
			case <-cm.stopChan:
				return
			default:
			}
			
			cm.mutex.Lock()
			if cm.state != ConsensusStateLeader {
				// Start election
				cm.startElection()
			}
			cm.mutex.Unlock()
			
			// Reset timer only if still running
			select {
			case <-ctx.Done():
				return
			case <-cm.stopChan:
				return
			default:
				electionTimer.Reset(cm.randomElectionTimeout())
			}

		case <-heartbeatTimer.C:
			// Check if we should still be running before processing heartbeat
			select {
			case <-ctx.Done():
				return
			case <-cm.stopChan:
				return
			default:
			}
			
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
	// For now, this is a placeholder that properly handles cancellation
	
	// Create a ticker for PBFT operations to prevent goroutine hanging
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-cm.stopChan:
			return
		case <-ticker.C:
			// TODO: Implement PBFT consensus operations
			// For now, just continue the loop to allow proper shutdown
		}
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

// sendHeartbeats sends heartbeats to all followers with circuit breaker protection
func (cm *ConsensusManager) sendHeartbeats() {
	// TODO: Send heartbeat messages to maintain leadership with circuit breaker
	// This would use a circuit breaker to prevent cascade failures
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