package distributed

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events/internal/worker"
	"go.uber.org/zap"
)

// SyncProtocol represents the state synchronization protocol
type SyncProtocol string

const (
	// SyncProtocolGossip uses gossip protocol for eventual consistency
	SyncProtocolGossip SyncProtocol = "gossip"
	// SyncProtocolMerkle uses Merkle trees for efficient sync
	SyncProtocolMerkle SyncProtocol = "merkle"
	// SyncProtocolCRDT uses Conflict-free Replicated Data Types
	SyncProtocolCRDT SyncProtocol = "crdt"
	// SyncProtocolSnapshot uses periodic snapshots
	SyncProtocolSnapshot SyncProtocol = "snapshot"
)

// StateSyncConfig contains configuration for state synchronization
type StateSyncConfig struct {
	// Protocol specifies which sync protocol to use
	Protocol SyncProtocol

	// SyncInterval is the interval between sync operations
	SyncInterval time.Duration

	// BatchSize is the maximum number of items to sync at once
	BatchSize int

	// MaxRetries is the maximum number of sync retries
	MaxRetries int

	// ConflictResolution specifies how to resolve conflicts
	ConflictResolution ConflictResolutionStrategy

	// EnableCompression enables compression for sync data
	EnableCompression bool

	// GossipFanout is the number of nodes to gossip to (for gossip protocol)
	GossipFanout int

	// SnapshotThreshold is the number of changes before creating a snapshot
	SnapshotThreshold int
}

// ConflictResolutionStrategy defines how to resolve state conflicts
type ConflictResolutionStrategy string

const (
	// ConflictResolutionLastWrite uses last-write-wins strategy
	ConflictResolutionLastWrite ConflictResolutionStrategy = "last_write"
	// ConflictResolutionHighestVersion uses highest version number
	ConflictResolutionHighestVersion ConflictResolutionStrategy = "highest_version"
	// ConflictResolutionMerge attempts to merge conflicting states
	ConflictResolutionMerge ConflictResolutionStrategy = "merge"
	// ConflictResolutionCustom uses custom resolution logic
	ConflictResolutionCustom ConflictResolutionStrategy = "custom"
)

// DefaultStateSyncConfig returns default state sync configuration
func DefaultStateSyncConfig() *StateSyncConfig {
	return &StateSyncConfig{
		Protocol:           SyncProtocolGossip,
		SyncInterval:       5 * time.Second,
		BatchSize:          100,
		MaxRetries:         3,
		ConflictResolution: ConflictResolutionLastWrite,
		EnableCompression:  true,
		GossipFanout:       3,
		SnapshotThreshold:  1000,
	}
}

// TestingStateSyncConfig returns a state sync configuration optimized for testing
// This uses faster intervals and timeouts to prevent test interference
func TestingStateSyncConfig() *StateSyncConfig {
	return &StateSyncConfig{
		Protocol:           SyncProtocolSnapshot, // Use simpler protocol for tests
		SyncInterval:       10 * time.Millisecond, // Very fast for tests
		BatchSize:          5,                     // Small batches for faster processing
		MaxRetries:         1,                     // Minimal retries for faster tests
		ConflictResolution: ConflictResolutionLastWrite,
		EnableCompression:  false,                 // Disable compression for simpler tests
		GossipFanout:       1,                     // Minimal fanout for tests
		SnapshotThreshold:  10,                    // Low threshold for tests
	}
}

// StateVersion represents a versioned state item
type StateVersion struct {
	Key       string      `json:"key"`
	Value     interface{} `json:"value"`
	Version   uint64      `json:"version"`
	Timestamp time.Time   `json:"timestamp"`
	NodeID    NodeID      `json:"node_id"`
	Checksum  string      `json:"checksum"`
}

// StateSnapshot represents a complete state snapshot
type StateSnapshot struct {
	NodeID         NodeID                   `json:"node_id"`
	Timestamp      time.Time                `json:"timestamp"`
	Version        uint64                   `json:"version"`
	ValidationState *events.ValidationState `json:"validation_state"`
	StateItems     map[string]*StateVersion `json:"state_items"`
	Checksum       string                   `json:"checksum"`
}

// SyncRequest represents a state sync request
type SyncRequest struct {
	RequestID  string    `json:"request_id"`
	FromNode   NodeID    `json:"from_node"`
	ToNode     NodeID    `json:"to_node"`
	Since      time.Time `json:"since"`
	Keys       []string  `json:"keys,omitempty"`
	MaxItems   int       `json:"max_items"`
}

// SyncResponse represents a state sync response
type SyncResponse struct {
	RequestID  string          `json:"request_id"`
	FromNode   NodeID          `json:"from_node"`
	ToNode     NodeID          `json:"to_node"`
	Items      []*StateVersion `json:"items"`
	HasMore    bool            `json:"has_more"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

// StateSynchronizer manages distributed state synchronization
type StateSynchronizer struct {
	config       *StateSyncConfig
	nodeID       NodeID
	
	// Local state storage
	state        map[string]*StateVersion
	stateMutex   sync.RWMutex
	
	// Version tracking
	localVersion uint64
	nodeVersions map[NodeID]uint64
	versionMutex sync.RWMutex
	
	// Sync tracking
	syncQueue    []*SyncRequest
	syncMutex    sync.Mutex
	pendingSync  map[string]time.Time
	
	// Merkle tree for efficient sync (if using Merkle protocol)
	merkleTree   *MerkleTree
	
	// CRDT state (if using CRDT protocol)
	crdtState    *CRDTState
	
	// Distributed cache layer
	distributedCache *DistributedCache
	
	// Circuit breakers for network operations
	syncCircuitBreaker    *NetworkCircuitBreaker
	gossipCircuitBreaker  *NetworkCircuitBreaker
	
	// Metrics
	syncCount    uint64
	conflictCount uint64
	
	// Lifecycle
	running      int32 // Use atomic operations for thread-safe access
	runningMutex sync.RWMutex
	stopChan     chan struct{}
	stopOnce     sync.Once
	
	// Direct goroutine tracking for fallback workers
	directGoroutines sync.WaitGroup
	
	// Worker management
	workerManager *worker.WorkerManager
	logger        *zap.Logger
}

// NewStateSynchronizer creates a new state synchronizer
func NewStateSynchronizer(config *StateSyncConfig, nodeID NodeID) (*StateSynchronizer, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Create logger
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Create worker manager with sufficient worker count for concurrent operations
	// Use unique configuration per instance to avoid interference
	workerConfig := &worker.WorkerConfig{
		MaxWorkers:      20, // Sufficient workers for multiple concurrent components
		ShutdownTimeout: 2 * time.Second, // Shorter timeout for faster test completion
		Logger:          logger,
		PanicHandler:    worker.NewDefaultPanicHandler(logger),
	}
	workerManager := worker.NewWorkerManager(workerConfig)

	// Determine appropriate distributed cache configuration
	var cacheConfig *DistributedCacheConfig
	if config.SyncInterval <= 100*time.Millisecond {
		// This appears to be a test configuration based on very short sync interval
		cacheConfig = &DistributedCacheConfig{
			BatchSize:       config.BatchSize,
			FlushInterval:   config.SyncInterval / 2,
			MaxRetries:      config.MaxRetries,
			ShutdownTimeout: 4 * time.Second, // Extra time for concurrent test scenarios
		}
	} else {
		// Production configuration
		cacheConfig = &DistributedCacheConfig{
			BatchSize:       config.BatchSize,
			FlushInterval:   config.SyncInterval / 2,
			MaxRetries:      config.MaxRetries,
			ShutdownTimeout: 2 * time.Second, // Standard production timeout
		}
	}

	ss := &StateSynchronizer{
		config:           config,
		nodeID:           nodeID,
		state:            make(map[string]*StateVersion),
		nodeVersions:     make(map[NodeID]uint64),
		syncQueue:        make([]*SyncRequest, 0),
		pendingSync:      make(map[string]time.Time),
		stopChan:         make(chan struct{}),
		workerManager:    workerManager,
		logger:           logger,
		distributedCache: NewDistributedCache(cacheConfig),
		syncCircuitBreaker: NewNetworkCircuitBreaker(&NetworkCircuitBreakerConfig{
			MaxFailures:  5,
			ResetTimeout: 30 * time.Second,
			RetryTimeout: 5 * time.Second,
		}),
		gossipCircuitBreaker: NewNetworkCircuitBreaker(&NetworkCircuitBreakerConfig{
			MaxFailures:  3,
			ResetTimeout: 15 * time.Second,
			RetryTimeout: 3 * time.Second,
		}),
	}

	// Initialize protocol-specific components
	switch config.Protocol {
	case SyncProtocolMerkle:
		ss.merkleTree = NewMerkleTree()
	case SyncProtocolCRDT:
		ss.crdtState = NewCRDTState(nodeID)
	}

	return ss, nil
}

// Start starts the state synchronizer
func (ss *StateSynchronizer) Start(ctx context.Context) error {
	ss.runningMutex.Lock()
	defer ss.runningMutex.Unlock()

	if atomic.LoadInt32(&ss.running) == 1 {
		return fmt.Errorf("state synchronizer already running")
	}

	ss.logger.Info("Starting StateSynchronizer", 
		zap.String("node_id", string(ss.nodeID)),
		zap.String("protocol", string(ss.config.Protocol)))

	// Start sync routine based on protocol
	if err := ss.startSyncProtocol(ctx); err != nil {
		ss.logger.Error("Failed to start sync protocol", zap.Error(err))
		return err
	}

	// Start common background routines
	ss.startBackgroundRoutines(ctx)

	// Start distributed cache with context
	ss.distributedCache.Start(ctx)

	atomic.StoreInt32(&ss.running, 1)
	ss.logger.Info("StateSynchronizer started successfully")
	return nil
}

// startSyncProtocol starts the appropriate sync protocol routine
func (ss *StateSynchronizer) startSyncProtocol(ctx context.Context) error {
	switch ss.config.Protocol {
	case SyncProtocolGossip:
		ss.startGossipSync(ctx)
	case SyncProtocolMerkle:
		ss.startMerkleSync(ctx)
	case SyncProtocolCRDT:
		ss.startCRDTSync(ctx)
	case SyncProtocolSnapshot:
		ss.startSnapshotSync(ctx)
	default:
		return fmt.Errorf("unknown sync protocol: %s", ss.config.Protocol)
	}
	return nil
}

// startGossipSync starts gossip sync routine
func (ss *StateSynchronizer) startGossipSync(ctx context.Context) {
	// Use StartOneOffWorker instead of BackgroundWorker to reduce worker pool pressure
	_, err := ss.workerManager.StartOneOffWorker("gossip-sync", func(ctx context.Context) error {
		ss.runGossipSync(ctx)
		return nil
	})
	if err != nil {
		ss.logger.Warn("Failed to start gossip sync worker, will try direct execution", zap.Error(err))
		// Fall back to direct execution if worker pool is exhausted
		ss.directGoroutines.Add(1)
		go func() {
			defer func() {
				ss.directGoroutines.Done()
				if r := recover(); r != nil {
					ss.logger.Error("Panic in direct gossip sync", zap.Any("panic", r))
				}
			}()
			ss.runGossipSync(ctx)
		}()
	}
}

// startMerkleSync starts merkle sync routine
func (ss *StateSynchronizer) startMerkleSync(ctx context.Context) {
	// Use StartOneOffWorker instead of BackgroundWorker to reduce worker pool pressure
	_, err := ss.workerManager.StartOneOffWorker("merkle-sync", func(ctx context.Context) error {
		ss.runMerkleSync(ctx)
		return nil
	})
	if err != nil {
		ss.logger.Warn("Failed to start merkle sync worker, will try direct execution", zap.Error(err))
		// Fall back to direct execution if worker pool is exhausted
		ss.directGoroutines.Add(1)
		go func() {
			defer func() {
				ss.directGoroutines.Done()
				if r := recover(); r != nil {
					ss.logger.Error("Panic in direct merkle sync", zap.Any("panic", r))
				}
			}()
			ss.runMerkleSync(ctx)
		}()
	}
}

// startCRDTSync starts CRDT sync routine
func (ss *StateSynchronizer) startCRDTSync(ctx context.Context) {
	// Use StartOneOffWorker instead of BackgroundWorker to reduce worker pool pressure
	_, err := ss.workerManager.StartOneOffWorker("crdt-sync", func(ctx context.Context) error {
		ss.runCRDTSync(ctx)
		return nil
	})
	if err != nil {
		ss.logger.Warn("Failed to start CRDT sync worker, will try direct execution", zap.Error(err))
		// Fall back to direct execution if worker pool is exhausted
		ss.directGoroutines.Add(1)
		go func() {
			defer func() {
				ss.directGoroutines.Done()
				if r := recover(); r != nil {
					ss.logger.Error("Panic in direct CRDT sync", zap.Any("panic", r))
				}
			}()
			ss.runCRDTSync(ctx)
		}()
	}
}

// startSnapshotSync starts snapshot sync routine
func (ss *StateSynchronizer) startSnapshotSync(ctx context.Context) {
	// Use StartOneOffWorker instead of BackgroundWorker to reduce worker pool pressure
	_, err := ss.workerManager.StartOneOffWorker("snapshot-sync", func(ctx context.Context) error {
		ss.runSnapshotSync(ctx)
		return nil
	})
	if err != nil {
		ss.logger.Warn("Failed to start snapshot sync worker, will try direct execution", zap.Error(err))
		// Fall back to direct execution if worker pool is exhausted
		ss.directGoroutines.Add(1)
		go func() {
			defer func() {
				ss.directGoroutines.Done()
				if r := recover(); r != nil {
					ss.logger.Error("Panic in direct snapshot sync", zap.Any("panic", r))
				}
			}()
			ss.runSnapshotSync(ctx)
		}()
	}
}

// startBackgroundRoutines starts common background routines
func (ss *StateSynchronizer) startBackgroundRoutines(ctx context.Context) {
	// Use StartOneOffWorker instead of BackgroundWorker to reduce worker pool pressure
	_, err := ss.workerManager.StartOneOffWorker("queue-processor", func(ctx context.Context) error {
		ss.processSyncQueue(ctx)
		return nil
	})
	if err != nil {
		ss.logger.Warn("Failed to start queue processor worker, will try direct execution", zap.Error(err))
		// Fall back to direct execution if worker pool is exhausted
		ss.directGoroutines.Add(1)
		go func() {
			defer func() {
				ss.directGoroutines.Done()
				if r := recover(); r != nil {
					ss.logger.Error("Panic in direct queue processor", zap.Any("panic", r))
				}
			}()
			ss.processSyncQueue(ctx)
		}()
	}
	
	// Use StartOneOffWorker instead of BackgroundWorker to reduce worker pool pressure
	_, err = ss.workerManager.StartOneOffWorker("cleanup-routine", func(ctx context.Context) error {
		ss.cleanupRoutine(ctx)
		return nil
	})
	if err != nil {
		ss.logger.Warn("Failed to start cleanup routine worker, will try direct execution", zap.Error(err))
		// Fall back to direct execution if worker pool is exhausted
		ss.directGoroutines.Add(1)
		go func() {
			defer func() {
				ss.directGoroutines.Done()
				if r := recover(); r != nil {
					ss.logger.Error("Panic in direct cleanup routine", zap.Any("panic", r))
				}
			}()
			ss.cleanupRoutine(ctx)
		}()
	}
}

// Stop stops the state synchronizer
func (ss *StateSynchronizer) Stop() error {
	ss.runningMutex.Lock()
	defer ss.runningMutex.Unlock()

	if atomic.LoadInt32(&ss.running) == 0 {
		return nil
	}

	ss.logger.Info("Stopping StateSynchronizer", 
		zap.String("node_id", string(ss.nodeID)))

	// Mark not running immediately to prevent new operations
	atomic.StoreInt32(&ss.running, 0)

	// Signal stop to all goroutines first to prevent new operations
	ss.stopOnce.Do(func() {
		close(ss.stopChan)
	})
	
	// Give a brief moment for all goroutines to see the stop signal
	// This is critical for allowing goroutines to exit gracefully
	time.Sleep(20 * time.Millisecond)
	
	// Stop distributed cache first to prevent new writes - with timeout
	if ss.distributedCache != nil {
		ss.logger.Debug("Stopping distributed cache")
		// Run cache stop in a goroutine to avoid being blocked
		cacheDone := make(chan struct{})
		go func() {
			defer close(cacheDone)
			ss.distributedCache.Stop()
		}()
		
		select {
		case <-cacheDone:
			ss.logger.Debug("Distributed cache stopped successfully")
		case <-time.After(1 * time.Second):
			ss.logger.Warn("Distributed cache stop timed out, continuing shutdown")
		}
	}
	
	// Stop worker manager with timeout
	ss.logger.Debug("Stopping worker manager")
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- ss.workerManager.Stop()
	}()
	
	var workerStopErr error
	select {
	case workerStopErr = <-workerDone:
		ss.logger.Debug("Worker manager stopped")
	case <-time.After(1 * time.Second):
		ss.logger.Warn("Worker manager stop timed out")
		workerStopErr = fmt.Errorf("worker manager stop timeout")
	}
	
	// Wait for direct goroutines to finish with shorter timeout
	ss.logger.Debug("Waiting for direct goroutines to finish")
	directGoroutineDone := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ss.logger.Error("Panic while waiting for direct goroutines", zap.Any("panic", r))
			}
			close(directGoroutineDone)
		}()
		ss.directGoroutines.Wait()
	}()
	
	select {
	case <-directGoroutineDone:
		ss.logger.Debug("All direct goroutines finished")
	case <-time.After(1 * time.Second): // Increased timeout for resource contention scenarios
		ss.logger.Warn("Some direct goroutines may still be running after timeout")
	}
	
	// Final verification
	finalMetrics := ss.workerManager.GetMetrics()
	if finalMetrics.WorkersActive > 0 {
		ss.logger.Warn("Workers still active after shutdown", 
			zap.Int64("active_workers", finalMetrics.WorkersActive))
	} else {
		ss.logger.Debug("All workers stopped successfully")
	}
	
	ss.logger.Info("StateSynchronizer stopped", 
		zap.Int64("workers_created", finalMetrics.WorkersCreated),
		zap.Int64("workers_completed", finalMetrics.WorkersCompleted),
		zap.Int64("workers_failed", finalMetrics.WorkersFailed),
		zap.Int64("final_active_workers", finalMetrics.WorkersActive))
		
	// Return worker stop error if there was one
	return workerStopErr
}

// SyncState synchronizes state with other nodes asynchronously
func (ss *StateSynchronizer) SyncState(ctx context.Context) error {
	// Get list of active nodes to sync with
	nodes := ss.getActiveNodes()
	if len(nodes) == 0 {
		return nil // No nodes to sync with
	}

	// Create timeout context for sync operations
	syncCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Create buffered channel for sync results
	results := make(chan syncResult, len(nodes))
	var syncWG sync.WaitGroup
	
	// Launch async sync operations for each node with wait group tracking
	for _, nodeID := range nodes {
		syncWG.Add(1)
		go func(nID NodeID) {
			defer func() {
				syncWG.Done()
				if r := recover(); r != nil {
					ss.logger.Error("Panic in async sync operation", 
						zap.String("node_id", string(nID)), 
						zap.Any("panic", r))
					// Send panic result to prevent deadlock
					select {
					case results <- syncResult{nodeID: nID, err: fmt.Errorf("sync panic: %v", r)}:
					case <-syncCtx.Done():
						// Context cancelled, don't block
					}
				}
			}()
			
			err := ss.syncWithNode(syncCtx, nID)
			
			// Send result with timeout protection
			select {
			case results <- syncResult{nodeID: nID, err: err}:
			case <-syncCtx.Done():
				// Context cancelled, stop
				return
			case <-time.After(2 * time.Second):
				// Timeout sending result, log and continue
				ss.logger.Warn("Timeout sending sync result", zap.String("node_id", string(nID)))
				return
			}
		}(nodeID)
	}

	// Ensure goroutines are cleaned up properly
	defer func() {
		// Wait for all sync operations to complete or timeout
		done := make(chan struct{})
		go func() {
			syncWG.Wait()
			close(done)
		}()
		
		select {
		case <-done:
			// All goroutines finished
		case <-time.After(5 * time.Second):
			// Timeout waiting for goroutines, they will be abandoned
			ss.logger.Warn("Timeout waiting for sync goroutines to finish")
		}
	}()

	// Collect results with timeout
	var syncErrors []error
	completedCount := 0
	
	for completedCount < len(nodes) {
		select {
		case <-syncCtx.Done():
			return fmt.Errorf("state sync timeout after %d/%d nodes completed", completedCount, len(nodes))
		case result, ok := <-results:
			if !ok {
				return fmt.Errorf("results channel closed unexpectedly")
			}
			completedCount++
			if result.err != nil {
				syncErrors = append(syncErrors, fmt.Errorf("node %s: %w", result.nodeID, result.err))
			}
		}
	}

	// Return error if majority of syncs failed
	if len(syncErrors) > len(nodes)/2 {
		return fmt.Errorf("state sync failed for majority of nodes: %v", syncErrors)
	}

	return nil
}

// syncResult represents the result of a sync operation with a node
type syncResult struct {
	nodeID NodeID
	err    error
}

// syncWithNode performs async synchronization with a specific node
func (ss *StateSynchronizer) syncWithNode(ctx context.Context, nodeID NodeID) error {
	request := &SyncRequest{
		RequestID: generateRequestID(),
		FromNode:  ss.nodeID,
		ToNode:    nodeID,
		Since:     time.Now().Add(-ss.config.SyncInterval * 2),
		MaxItems:  ss.config.BatchSize,
	}

	// Use circuit breaker pattern for resilience
	return ss.executeWithRetry(ctx, func() error {
		return ss.processSyncRequestAsync(ctx, request)
	})
}

// GetState retrieves a state value by key
func (ss *StateSynchronizer) GetState(key string) (*StateVersion, bool) {
	ss.stateMutex.RLock()
	defer ss.stateMutex.RUnlock()

	state, exists := ss.state[key]
	return state, exists
}

// SetState sets a state value
func (ss *StateSynchronizer) SetState(key string, value interface{}) error {
	ss.stateMutex.Lock()
	defer ss.stateMutex.Unlock()

	ss.localVersion++
	
	stateVersion := &StateVersion{
		Key:       key,
		Value:     value,
		Version:   ss.localVersion,
		Timestamp: time.Now(),
		NodeID:    ss.nodeID,
		Checksum:  ss.calculateChecksum(value),
	}

	ss.state[key] = stateVersion

	// Update protocol-specific structures
	switch ss.config.Protocol {
	case SyncProtocolMerkle:
		ss.merkleTree.Update(key, stateVersion)
	case SyncProtocolCRDT:
		ss.crdtState.Update(key, value)
	}

	// Trigger sync if using gossip protocol
	if ss.config.Protocol == SyncProtocolGossip {
		// Use worker manager to handle gossip updates to prevent goroutine leaks
		_, err := ss.workerManager.StartOneOffWorker("gossip-update", func(ctx context.Context) error {
			ss.gossipUpdate(stateVersion)
			return nil
		})
		if err != nil {
			ss.logger.Warn("Failed to start gossip update worker, will skip gossip", zap.Error(err))
			// Don't fall back to direct execution for gossip updates to avoid overwhelming the system
		}
	}

	return nil
}

// GetSnapshot returns a complete state snapshot
func (ss *StateSynchronizer) GetSnapshot() *StateSnapshot {
	ss.stateMutex.RLock()
	defer ss.stateMutex.RUnlock()

	// Create a copy of the state
	stateCopy := make(map[string]*StateVersion)
	for k, v := range ss.state {
		vCopy := *v
		stateCopy[k] = &vCopy
	}

	snapshot := &StateSnapshot{
		NodeID:     ss.nodeID,
		Timestamp:  time.Now(),
		Version:    ss.localVersion,
		StateItems: stateCopy,
	}

	snapshot.Checksum = ss.calculateSnapshotChecksum(snapshot)
	return snapshot
}

// ApplySnapshot applies a state snapshot
func (ss *StateSynchronizer) ApplySnapshot(snapshot *StateSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot cannot be nil")
	}

	// Verify snapshot checksum
	expectedChecksum := ss.calculateSnapshotChecksum(snapshot)
	if snapshot.Checksum != expectedChecksum {
		return fmt.Errorf("snapshot checksum mismatch")
	}

	ss.stateMutex.Lock()
	defer ss.stateMutex.Unlock()

	// Apply snapshot based on conflict resolution strategy
	for key, remoteState := range snapshot.StateItems {
		localState, exists := ss.state[key]
		
		if !exists {
			// New state item
			ss.state[key] = remoteState
			continue
		}

		// Resolve conflict
		if ss.shouldApplyRemoteState(localState, remoteState) {
			ss.state[key] = remoteState
		}
	}

	// Update version tracking
	ss.versionMutex.Lock()
	ss.nodeVersions[snapshot.NodeID] = snapshot.Version
	ss.versionMutex.Unlock()

	return nil
}

// runGossipSync implements gossip protocol for state synchronization
func (ss *StateSynchronizer) runGossipSync(ctx context.Context) {
	ticker := time.NewTicker(ss.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ss.stopChan:
			return
		case <-ticker.C:
			// Check context again before performing gossip round
			select {
			case <-ctx.Done():
				return
			case <-ss.stopChan:
				return
			default:
				// Use a timeout to prevent blocking on shutdown
				gossipCtx, cancel := context.WithTimeout(ctx, ss.config.SyncInterval/2)
				ss.performGossipRoundWithContext(gossipCtx)
				cancel()
			}
		}
	}
}

// runMerkleSync implements Merkle tree-based synchronization
func (ss *StateSynchronizer) runMerkleSync(ctx context.Context) {
	ticker := time.NewTicker(ss.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ss.stopChan:
			return
		case <-ticker.C:
			ss.performMerkleSync()
		}
	}
}

// runCRDTSync implements CRDT-based synchronization
func (ss *StateSynchronizer) runCRDTSync(ctx context.Context) {
	ticker := time.NewTicker(ss.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ss.stopChan:
			return
		case <-ticker.C:
			ss.performCRDTSync()
		}
	}
}

// runSnapshotSync implements snapshot-based synchronization
func (ss *StateSynchronizer) runSnapshotSync(ctx context.Context) {
	ticker := time.NewTicker(ss.config.SyncInterval)
	defer ticker.Stop()

	changeCount := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ss.stopChan:
			return
		case <-ticker.C:
			changeCount++
			if changeCount >= ss.config.SnapshotThreshold {
				ss.createAndDistributeSnapshot()
				changeCount = 0
			}
		}
	}
}

// performGossipRound performs one round of gossip with async cache operations
func (ss *StateSynchronizer) performGossipRound() {
	ss.performGossipRoundWithContext(context.Background())
}

// performGossipRoundWithContext performs one round of gossip with context support
func (ss *StateSynchronizer) performGossipRoundWithContext(ctx context.Context) {
	// Check context before starting
	select {
	case <-ctx.Done():
		return
	default:
	}
	
	// Select random nodes to gossip with
	nodes := ss.selectGossipNodes()
	
	// Get recent updates with batching
	updates := ss.getRecentUpdatesWithBatching()
	
	// Send updates to selected nodes asynchronously with buffering and context
	ss.sendGossipUpdatesAsyncWithContext(ctx, nodes, updates)
}

// getRecentUpdatesWithBatching gets recent updates with intelligent batching
func (ss *StateSynchronizer) getRecentUpdatesWithBatching() []*StateVersion {
	ss.stateMutex.RLock()
	defer ss.stateMutex.RUnlock()

	updates := make([]*StateVersion, 0)
	cutoff := time.Now().Add(-ss.config.SyncInterval * 2)

	// Collect updates with batching limit
	count := 0
	maxBatchSize := ss.config.BatchSize
	
	for _, state := range ss.state {
		if state.Timestamp.After(cutoff) && count < maxBatchSize {
			updates = append(updates, state)
			count++
		}
	}

	return updates
}

// sendGossipUpdatesAsync sends gossip updates asynchronously with buffering
func (ss *StateSynchronizer) sendGossipUpdatesAsync(nodes []NodeID, updates []*StateVersion) {
	ss.sendGossipUpdatesAsyncWithContext(context.Background(), nodes, updates)
}

// sendGossipUpdatesAsyncWithContext sends gossip updates asynchronously with buffering and context
func (ss *StateSynchronizer) sendGossipUpdatesAsyncWithContext(parentCtx context.Context, nodes []NodeID, updates []*StateVersion) {
	if len(nodes) == 0 || len(updates) == 0 {
		return
	}

	// Check parent context before starting
	select {
	case <-parentCtx.Done():
		return
	default:
	}

	// Create buffered channel for async operations
	taskChan := make(chan gossipTask, len(nodes))
	resultChan := make(chan gossipResult, len(nodes))
	
	// Create context with timeout derived from parent context
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()
	
	// Start worker pool with wait group for proper cleanup
	workerCount := 3
	if len(nodes) < workerCount {
		workerCount = len(nodes)
	}
	
	var workerWG sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			ss.gossipWorker(ctx, taskChan, resultChan)
		}()
	}
	
	// Send tasks to workers
	for _, nodeID := range nodes {
		task := gossipTask{
			nodeID:  nodeID,
			updates: updates,
		}
		
		select {
		case taskChan <- task:
		case <-ctx.Done():
			close(taskChan)
			// Wait for workers to finish
			workerWG.Wait()
			return
		}
	}
	
	close(taskChan)
	
	// Collect results asynchronously with proper cleanup
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ss.logger.Error("Panic in gossip result collection", zap.Any("panic", r))
			}
			// Ensure workers are cleaned up
			workerWG.Wait()
			// Drain any remaining results with timeout to prevent deadlock
			drainTimeout := time.After(100 * time.Millisecond)
			for {
				select {
				case <-resultChan:
					// Drain remaining results
				case <-drainTimeout:
					return
				default:
					return
				}
			}
		}()
		
		collectedResults := 0
		for collectedResults < len(nodes) {
			select {
			case result, ok := <-resultChan:
				if !ok {
					ss.logger.Debug("Gossip result channel closed")
					return // Channel closed
				}
				collectedResults++
				if result.err != nil {
					ss.logger.Error("Gossip operation failed", 
						zap.String("node_id", string(result.nodeID)),
						zap.Error(result.err))
				} else {
					ss.logger.Debug("Gossip operation successful", 
						zap.String("node_id", string(result.nodeID)))
				}
			case <-ctx.Done():
				// Context cancelled, wait for workers and return
				ss.logger.Debug("Gossip result collection cancelled by context")
				workerWG.Wait()
				return
			case <-time.After(5 * time.Second): // Reduced timeout to be more responsive
				// Timeout, wait for workers and return
				ss.logger.Warn("Gossip result collection timed out")
				workerWG.Wait()
				return
			}
		}
	}()
}

// gossipTask represents a gossip task
type gossipTask struct {
	nodeID  NodeID
	updates []*StateVersion
}

// gossipResult represents the result of a gossip operation
type gossipResult struct {
	nodeID NodeID
	err    error
}

// gossipWorker processes gossip tasks asynchronously
func (ss *StateSynchronizer) gossipWorker(ctx context.Context, tasks <-chan gossipTask, results chan<- gossipResult) {
	defer func() {
		if r := recover(); r != nil {
			ss.logger.Error("Panic in gossip worker", zap.Any("panic", r))
			// Send panic result to prevent deadlock
			select {
			case results <- gossipResult{nodeID: "unknown", err: fmt.Errorf("gossip worker panic: %v", r)}:
			default:
				// Results channel might be full or closed
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-tasks:
			if !ok {
				return // Channel closed
			}
			
			// Check context again before processing
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Create timeout context for individual gossip operation
			gossipCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err := ss.sendGossipUpdateWithCache(gossipCtx, task.nodeID, task.updates)
			cancel()

			// Send result with timeout protection
			select {
			case results <- gossipResult{nodeID: task.nodeID, err: err}:
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
				// Timeout sending result, continue to prevent deadlock
				continue
			}
		}
	}
}

// sendGossipUpdateWithCache sends gossip update with distributed cache optimization
func (ss *StateSynchronizer) sendGossipUpdateWithCache(ctx context.Context, nodeID NodeID, updates []*StateVersion) error {
	// Implement distributed cache batching
	batchedUpdates := ss.batchUpdatesForNode(nodeID, updates)
	
	// Execute with circuit breaker protection
	return ss.gossipCircuitBreaker.Execute(func() error {
		return ss.executeWithRetry(ctx, func() error {
			return ss.sendGossipUpdateBatch(ctx, nodeID, batchedUpdates)
		})
	})
}

// batchUpdatesForNode batches updates optimally for a specific node
func (ss *StateSynchronizer) batchUpdatesForNode(nodeID NodeID, updates []*StateVersion) []*StateVersion {
	// Get node's last known version to optimize batching
	ss.versionMutex.RLock()
	lastVersion := ss.nodeVersions[nodeID]
	ss.versionMutex.RUnlock()
	
	// Filter updates that are newer than node's last version
	filteredUpdates := make([]*StateVersion, 0)
	for _, update := range updates {
		if update.Version > lastVersion {
			filteredUpdates = append(filteredUpdates, update)
		}
	}
	
	return filteredUpdates
}

// sendGossipUpdateBatch sends a batch of gossip updates
func (ss *StateSynchronizer) sendGossipUpdateBatch(ctx context.Context, nodeID NodeID, updates []*StateVersion) error {
	// TODO: Implement actual network communication with batching
	// For now, simulate async batch operation
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond): // Simulate network delay
		// Simulate occasional failures for testing
		if time.Now().UnixNano()%12 == 0 {
			return fmt.Errorf("batch gossip network failure for node %s", nodeID)
		}
		
		// Update node version on successful batch
		if len(updates) > 0 {
			maxVersion := uint64(0)
			for _, update := range updates {
				if update.Version > maxVersion {
					maxVersion = update.Version
				}
			}
			
			ss.versionMutex.Lock()
			if maxVersion > ss.nodeVersions[nodeID] {
				ss.nodeVersions[nodeID] = maxVersion
			}
			ss.versionMutex.Unlock()
		}
		
		return nil
	}
}

// performMerkleSync performs Merkle tree synchronization
func (ss *StateSynchronizer) performMerkleSync() {
	// TODO: Implement Merkle tree sync
}

// performCRDTSync performs CRDT synchronization
func (ss *StateSynchronizer) performCRDTSync() {
	// TODO: Implement CRDT sync
}

// createAndDistributeSnapshot creates and distributes a state snapshot
func (ss *StateSynchronizer) createAndDistributeSnapshot() {
	snapshot := ss.GetSnapshot()
	
	// TODO: Distribute snapshot to other nodes
	_ = snapshot
}

// processSyncQueue processes pending sync requests
func (ss *StateSynchronizer) processSyncQueue(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			ss.logger.Debug("Sync queue processor cancelled by context")
			return
		case <-ss.stopChan:
			ss.logger.Debug("Sync queue processor stopped by stop signal")
			return
		case <-ticker.C:
			// Check context before processing
			select {
			case <-ctx.Done():
				ss.logger.Debug("Sync queue processor cancelled during tick")
				return
			case <-ss.stopChan:
				ss.logger.Debug("Sync queue processor stopped during tick")
				return
			default:
				request := ss.dequeueSyncRequest()
				if request != nil {
					ss.processSyncRequest(request)
				}
			}
		}
	}
}

// processSyncRequest processes a single sync request (legacy method)
func (ss *StateSynchronizer) processSyncRequest(request *SyncRequest) {
	// TODO: Implement actual network communication
	// For now, this is a placeholder
}

// processSyncRequestAsync processes a single sync request asynchronously
func (ss *StateSynchronizer) processSyncRequestAsync(ctx context.Context, request *SyncRequest) error {
	// Create timeout context for this specific request
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Track pending sync
	ss.markSyncPending(request.RequestID)
	defer ss.markSyncComplete(request.RequestID)

	// Execute with circuit breaker protection
	return ss.syncCircuitBreaker.Execute(func() error {
		// TODO: Implement actual async network communication
		// For now, simulate async operation
		select {
		case <-reqCtx.Done():
			return fmt.Errorf("sync request timeout for node %s", request.ToNode)
		case <-time.After(100 * time.Millisecond): // Simulate network delay
			// Simulate successful sync
			return nil
		}
	})
}

// executeWithRetry executes a function with retry logic and exponential backoff
func (ss *StateSynchronizer) executeWithRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	
	for attempt := 0; attempt < ss.config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		err := fn()
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Don't retry on the last attempt
		if attempt == ss.config.MaxRetries-1 {
			break
		}
		
		// Exponential backoff with jitter
		backoffDuration := time.Duration(100*(1<<attempt)) * time.Millisecond
		jitter := time.Duration(time.Now().UnixNano()%int64(backoffDuration/2))
		backoffDuration += jitter
		
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoffDuration):
			// Continue to next attempt
		}
	}
	
	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// markSyncPending marks a sync operation as pending
func (ss *StateSynchronizer) markSyncPending(requestID string) {
	ss.syncMutex.Lock()
	defer ss.syncMutex.Unlock()
	ss.pendingSync[requestID] = time.Now()
}

// markSyncComplete marks a sync operation as complete
func (ss *StateSynchronizer) markSyncComplete(requestID string) {
	ss.syncMutex.Lock()
	defer ss.syncMutex.Unlock()
	delete(ss.pendingSync, requestID)
}

// cleanupRoutine performs periodic cleanup
func (ss *StateSynchronizer) cleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			ss.logger.Debug("Cleanup routine cancelled by context")
			return
		case <-ss.stopChan:
			ss.logger.Debug("Cleanup routine stopped by stop signal")
			return
		case <-ticker.C:
			// Check context before performing cleanup
			select {
			case <-ctx.Done():
				ss.logger.Debug("Cleanup routine cancelled during tick")
				return
			case <-ss.stopChan:
				ss.logger.Debug("Cleanup routine stopped during tick")
				return
			default:
				ss.cleanupPendingSync()
			}
		}
	}
}

// cleanupPendingSync cleans up stale pending sync operations
func (ss *StateSynchronizer) cleanupPendingSync() {
	ss.syncMutex.Lock()
	defer ss.syncMutex.Unlock()

	now := time.Now()
	for key, timestamp := range ss.pendingSync {
		if now.Sub(timestamp) > 5*time.Minute {
			delete(ss.pendingSync, key)
		}
	}
}

// Helper methods

func (ss *StateSynchronizer) getActiveNodes() []NodeID {
	// TODO: Get list of active nodes from cluster
	return []NodeID{}
}

func (ss *StateSynchronizer) selectGossipNodes() []NodeID {
	// TODO: Select random nodes for gossip
	return []NodeID{}
}

func (ss *StateSynchronizer) getRecentUpdates() []*StateVersion {
	ss.stateMutex.RLock()
	defer ss.stateMutex.RUnlock()

	updates := make([]*StateVersion, 0)
	cutoff := time.Now().Add(-ss.config.SyncInterval * 2)

	for _, state := range ss.state {
		if state.Timestamp.After(cutoff) {
			updates = append(updates, state)
		}
	}

	return updates
}

func (ss *StateSynchronizer) gossipUpdate(update *StateVersion) {
	nodes := ss.selectGossipNodes()
	if len(nodes) == 0 {
		return
	}
	
	// Send updates asynchronously with proper context and worker management
	// Use the existing async mechanism with proper cleanup
	ss.sendGossipUpdatesAsync(nodes, []*StateVersion{update})
}

func (ss *StateSynchronizer) sendGossipUpdate(nodeID NodeID, updates []*StateVersion) {
	// TODO: Send gossip update to node
}

func (ss *StateSynchronizer) shouldApplyRemoteState(local, remote *StateVersion) bool {
	switch ss.config.ConflictResolution {
	case ConflictResolutionLastWrite:
		return remote.Timestamp.After(local.Timestamp)
	case ConflictResolutionHighestVersion:
		return remote.Version > local.Version
	case ConflictResolutionMerge:
		// TODO: Implement merge logic
		return false
	default:
		return false
	}
}

func (ss *StateSynchronizer) calculateChecksum(value interface{}) string {
	// TODO: Implement proper checksum calculation
	data, _ := json.Marshal(value)
	return fmt.Sprintf("%x", data)
}

func (ss *StateSynchronizer) calculateSnapshotChecksum(snapshot *StateSnapshot) string {
	// TODO: Implement proper checksum calculation
	data, _ := json.Marshal(snapshot.StateItems)
	return fmt.Sprintf("%x", data)
}

func (ss *StateSynchronizer) enqueueSyncRequest(request *SyncRequest) {
	ss.syncMutex.Lock()
	defer ss.syncMutex.Unlock()
	
	ss.syncQueue = append(ss.syncQueue, request)
}

func (ss *StateSynchronizer) dequeueSyncRequest() *SyncRequest {
	ss.syncMutex.Lock()
	defer ss.syncMutex.Unlock()
	
	if len(ss.syncQueue) == 0 {
		return nil
	}
	
	request := ss.syncQueue[0]
	ss.syncQueue = ss.syncQueue[1:]
	return request
}

func (ss *StateSynchronizer) waitForSyncCompletion() <-chan struct{} {
	done := make(chan struct{})
	
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Log panic but ensure done channel is closed
				fmt.Printf("Panic in state sync completion goroutine: %v\n", r)
				// Ensure channel is closed even if panic occurs
				select {
				case <-done:
					// Already closed
				default:
					close(done)
				}
			}
		}()
		// TODO: Implement proper sync completion detection
		time.Sleep(1 * time.Second)
		close(done)
	}()
	
	return done
}

func generateRequestID() string {
	return fmt.Sprintf("sync-%d", time.Now().UnixNano())
}

// MerkleTree represents a Merkle tree for efficient state comparison
type MerkleTree struct {
	// TODO: Implement Merkle tree
}

func NewMerkleTree() *MerkleTree {
	return &MerkleTree{}
}

func (mt *MerkleTree) Update(key string, value *StateVersion) {
	// TODO: Implement Merkle tree update
}

// CRDTState represents CRDT-based state
type CRDTState struct {
	nodeID NodeID
	// TODO: Implement CRDT state
}

func NewCRDTState(nodeID NodeID) *CRDTState {
	return &CRDTState{
		nodeID: nodeID,
	}
}

func (cs *CRDTState) Update(key string, value interface{}) {
	// TODO: Implement CRDT update
}

// DistributedCache implements asynchronous distributed caching with buffering and batching
type DistributedCache struct {
	config *DistributedCacheConfig
	
	// Write buffer for batching
	writeBuffer map[string]*CacheItem
	bufferMutex sync.RWMutex
	
	// Async write queue
	writeQueue chan *CacheItem
	
	// Read cache for performance
	readCache map[string]*CacheItem
	cacheMutex sync.RWMutex
	
	// Lifecycle
	running int32 // Use atomic operations for thread-safe access
	stopChan chan struct{}
	stopOnce sync.Once
	wg sync.WaitGroup
}

// DistributedCacheConfig contains configuration for distributed cache
type DistributedCacheConfig struct {
	BatchSize     int
	FlushInterval time.Duration
	MaxRetries    int
	ShutdownTimeout time.Duration // Timeout for graceful shutdown
}

// CacheItem represents an item in the distributed cache
type CacheItem struct {
	Key       string
	Value     interface{}
	Version   uint64
	Timestamp time.Time
	NodeID    NodeID
}

// NewDistributedCache creates a new distributed cache
func NewDistributedCache(config *DistributedCacheConfig) *DistributedCache {
	// Set default shutdown timeout if not specified
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 2 * time.Second // More reasonable default for production
	}
	
	return &DistributedCache{
		config:      config,
		writeBuffer: make(map[string]*CacheItem),
		writeQueue:  make(chan *CacheItem, config.BatchSize*2),
		readCache:   make(map[string]*CacheItem),
		stopChan:    make(chan struct{}),
	}
}

// NewTestingDistributedCache creates a distributed cache optimized for testing
// This uses very fast flush intervals and small buffers to prevent test interference
func NewTestingDistributedCache() *DistributedCache {
	config := &DistributedCacheConfig{
		BatchSize:       2,                      // Very small batches for fast processing
		FlushInterval:   5 * time.Millisecond,  // Very fast flush for tests
		MaxRetries:      1,                      // Minimal retries
		ShutdownTimeout: 3 * time.Second,       // Longer timeout for concurrent testing scenarios
	}
	
	return &DistributedCache{
		config:      config,
		writeBuffer: make(map[string]*CacheItem),
		writeQueue:  make(chan *CacheItem, config.BatchSize*2), // Small queue
		readCache:   make(map[string]*CacheItem),
		stopChan:    make(chan struct{}),
	}
}

// Start starts the distributed cache background routines
func (dc *DistributedCache) Start(ctx context.Context) {
	atomic.StoreInt32(&dc.running, 1)
	
	// Start batch flush routine
	dc.wg.Add(1)
	go func() {
		defer dc.wg.Done()
		dc.flushRoutine(ctx)
	}()
	
	// Start write queue processor
	dc.wg.Add(1)
	go func() {
		defer dc.wg.Done()
		dc.writeQueueProcessor(ctx)
	}()
}

// Stop stops the distributed cache
func (dc *DistributedCache) Stop() {
	dc.stopOnce.Do(func() {
		fmt.Printf("DistributedCache stopping...\n")
		atomic.StoreInt32(&dc.running, 0)
		// Signal stop first - this will be seen immediately by goroutines
		close(dc.stopChan)
		
		// Give processors adequate time to see the stop signal and drain queues
		// This prevents goroutine leaks by ensuring processors can exit cleanly
		time.Sleep(100 * time.Millisecond)
		
		// Then close write queue channel to signal processors to finish
		// Use a safer approach to close the channel to avoid double-close panics
		defer func() {
			if r := recover(); r != nil {
				// Channel was already closed, which is fine
				fmt.Printf("DistributedCache: writeQueue already closed\n")
			}
		}()
		close(dc.writeQueue)
	})
	
	// Wait for all goroutines to finish with more aggressive timeout and monitoring
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Panic waiting for DistributedCache goroutines: %v\n", r)
			}
			close(done)
		}()
		dc.wg.Wait()
	}()
	
	// Wait for shutdown with configurable timeout and better monitoring
	timeoutDuration := dc.config.ShutdownTimeout
	select {
	case <-done:
		// All goroutines finished gracefully
		fmt.Printf("DistributedCache stopped successfully\n")
	case <-time.After(timeoutDuration):
		// Timeout occurred - this indicates resource contention or hanging goroutines
		fmt.Printf("Warning: DistributedCache goroutines did not stop within timeout (%v), forcing shutdown\n", timeoutDuration)
		// Cancel any remaining work by marking as not running
		atomic.StoreInt32(&dc.running, 0)
		// Give a final brief moment for goroutines to detect the running=false flag
		time.Sleep(50 * time.Millisecond)
	}
}

// GetAsync retrieves a value from the distributed cache asynchronously
func (dc *DistributedCache) GetAsync(ctx context.Context, key string) (*CacheItem, error) {
	// Check read cache first
	dc.cacheMutex.RLock()
	if item, exists := dc.readCache[key]; exists {
		dc.cacheMutex.RUnlock()
		return item, nil
	}
	dc.cacheMutex.RUnlock()
	
	// Check write buffer
	dc.bufferMutex.RLock()
	if item, exists := dc.writeBuffer[key]; exists {
		dc.bufferMutex.RUnlock()
		return item, nil
	}
	dc.bufferMutex.RUnlock()
	
	// TODO: Implement async network fetch from other nodes
	// For now, return not found
	return nil, fmt.Errorf("key %s not found in distributed cache", key)
}

// SetAsync sets a value in the distributed cache asynchronously
func (dc *DistributedCache) SetAsync(ctx context.Context, key string, value interface{}, nodeID NodeID) error {
	item := &CacheItem{
		Key:       key,
		Value:     value,
		Version:   uint64(time.Now().UnixNano()),
		Timestamp: time.Now(),
		NodeID:    nodeID,
	}
	
	// Add to write buffer
	dc.bufferMutex.Lock()
	dc.writeBuffer[key] = item
	dc.bufferMutex.Unlock()
	
	// Add to read cache for immediate reads
	dc.cacheMutex.Lock()
	dc.readCache[key] = item
	dc.cacheMutex.Unlock()
	
	// Queue for async write
	select {
	case dc.writeQueue <- item:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Queue is full, force immediate flush
		go dc.flushBuffer()
		return nil
	}
}

// flushRoutine periodically flushes the write buffer
func (dc *DistributedCache) flushRoutine(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in distributed cache flush routine: %v\n", r)
		}
		// Perform final flush on exit to prevent data loss
		if atomic.LoadInt32(&dc.running) == 1 {
			dc.flushBuffer()
		}
		fmt.Printf("DistributedCache flushRoutine: exiting\n")
	}()
	
	ticker := time.NewTicker(dc.config.FlushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			// Context cancelled, perform final flush and exit
			fmt.Printf("DistributedCache flushRoutine: context cancelled\n")
			return
		case <-dc.stopChan:
			// Stop requested, perform final flush and exit immediately
			fmt.Printf("DistributedCache flushRoutine: stop signal received\n")
			return
		case <-ticker.C:
			// Check if we're still running before processing
			if atomic.LoadInt32(&dc.running) == 0 {
				return
			}
			// Quick check for stop conditions before flushing
			select {
			case <-ctx.Done():
				fmt.Printf("DistributedCache flushRoutine: context cancelled during tick\n")
				return
			case <-dc.stopChan:
				fmt.Printf("DistributedCache flushRoutine: stop signal during tick\n")
				return
			default:
				// Only flush if we're definitely still running
				if atomic.LoadInt32(&dc.running) == 1 {
					dc.flushBuffer()
				}
			}
		}
	}
}

// flushBuffer flushes the write buffer to distributed nodes
func (dc *DistributedCache) flushBuffer() {
	dc.bufferMutex.Lock()
	if len(dc.writeBuffer) == 0 {
		dc.bufferMutex.Unlock()
		return
	}
	
	// Create batch from buffer
	batch := make([]*CacheItem, 0, len(dc.writeBuffer))
	for _, item := range dc.writeBuffer {
		batch = append(batch, item)
	}
	
	// Clear buffer
	dc.writeBuffer = make(map[string]*CacheItem)
	dc.bufferMutex.Unlock()
	
	// Check if still running before creating goroutine
	if atomic.LoadInt32(&dc.running) == 0 {
		// If not running, process batch synchronously to avoid goroutine leak
		dc.processBatch(batch)
		return
	}
	
	// Process batch asynchronously only if still running
	dc.wg.Add(1)
	go func() {
		defer func() {
			dc.wg.Done()
			if r := recover(); r != nil {
				fmt.Printf("Panic in cache flush: %v\n", r)
			}
		}()
		
		dc.processBatch(batch)
	}()
}

// writeQueueProcessor processes the write queue
func (dc *DistributedCache) writeQueueProcessor(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in distributed cache write queue processor: %v\n", r)
		}
		// Ensure write queue is drained on any exit
		dc.drainWriteQueue()
		fmt.Printf("DistributedCache writeQueueProcessor: exiting\n")
	}()
	
	for {
		// Check stop conditions first
		if atomic.LoadInt32(&dc.running) == 0 {
			fmt.Printf("DistributedCache writeQueueProcessor: cache not running, exiting\n")
			return
		}
		
		select {
		case <-ctx.Done():
			// Context cancelled, drain remaining items and exit
			fmt.Printf("DistributedCache writeQueueProcessor: context cancelled\n")
			return
		case <-dc.stopChan:
			// Stop requested, drain remaining items and exit
			fmt.Printf("DistributedCache writeQueueProcessor: stop signal received\n")
			return
		case item, ok := <-dc.writeQueue:
			if !ok {
				// Channel closed, stop processing
				fmt.Printf("DistributedCache writeQueueProcessor: write queue channel closed\n")
				return
			}
			
			// Double-check we're still running before processing
			if atomic.LoadInt32(&dc.running) == 0 {
				return
			}
			
			// Always check shutdown conditions first before any processing
			if atomic.LoadInt32(&dc.running) == 0 {
				return
			}
			
			// Check context and stop signal before processing with non-blocking select
			select {
			case <-ctx.Done():
				// Context cancelled while receiving item
				fmt.Printf("DistributedCache writeQueueProcessor: context cancelled during item processing\n")
				return
			case <-dc.stopChan:
				// Stop requested while receiving item
				fmt.Printf("DistributedCache writeQueueProcessor: stop signal during item processing\n")
				return
			default:
				// Process individual write only if we're still running
				// Simplified approach to avoid nested goroutines which can cause leaks
				if atomic.LoadInt32(&dc.running) == 1 {
					dc.processWrite(item)
				}
			}
		}
	}
}

// drainWriteQueue drains any remaining items in the write queue
func (dc *DistributedCache) drainWriteQueue() {
	drainCount := 0
	for {
		select {
		case <-dc.writeQueue:
			// Drain remaining items
			drainCount++
			if drainCount > 1000 { // Prevent infinite loop
				fmt.Printf("DistributedCache: drained %d items, stopping to prevent infinite loop\n", drainCount)
				return
			}
		default:
			if drainCount > 0 {
				fmt.Printf("DistributedCache: drained %d items from write queue\n", drainCount)
			}
			return
		}
	}
}

// processBatch processes a batch of cache items
func (dc *DistributedCache) processBatch(batch []*CacheItem) {
	// TODO: Implement actual distributed batch write
	// For now, simulate batch processing with shutdown-aware sleep
	if atomic.LoadInt32(&dc.running) == 1 {
		select {
		case <-dc.stopChan:
			// Stop requested, exit immediately
			return
		case <-time.After(50 * time.Millisecond):
			// Completed processing simulation
		}
	}
}

// processWrite processes a single cache write
func (dc *DistributedCache) processWrite(item *CacheItem) {
	// TODO: Implement actual distributed write
	// For now, simulate write processing
	time.Sleep(10 * time.Millisecond)
}

// InvalidateAsync invalidates a key in the distributed cache asynchronously
func (dc *DistributedCache) InvalidateAsync(ctx context.Context, key string) error {
	// Remove from read cache
	dc.cacheMutex.Lock()
	delete(dc.readCache, key)
	dc.cacheMutex.Unlock()
	
	// Remove from write buffer
	dc.bufferMutex.Lock()
	delete(dc.writeBuffer, key)
	dc.bufferMutex.Unlock()
	
	// TODO: Implement distributed invalidation
	return nil
}

// NetworkCircuitBreaker implements a circuit breaker specifically for network operations
type NetworkCircuitBreaker struct {
	config       *NetworkCircuitBreakerConfig
	state        CircuitBreakerState
	failures     int
	lastFailure  time.Time
	lastSuccess  time.Time
	mutex        sync.RWMutex
}

// NetworkCircuitBreakerConfig contains configuration for network circuit breaker
type NetworkCircuitBreakerConfig struct {
	MaxFailures  int
	ResetTimeout time.Duration
	RetryTimeout time.Duration
}


// NewNetworkCircuitBreaker creates a new network circuit breaker
func NewNetworkCircuitBreaker(config *NetworkCircuitBreakerConfig) *NetworkCircuitBreaker {
	return &NetworkCircuitBreaker{
		config:      config,
		state:       CircuitBreakerStateClosed,
		failures:    0,
		lastSuccess: time.Now(),
	}
}

// Execute executes a function with circuit breaker protection
func (cb *NetworkCircuitBreaker) Execute(fn func() error) error {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	// Check if circuit breaker allows execution
	if !cb.canExecute() {
		return fmt.Errorf("circuit breaker is open")
	}
	
	// Execute the function
	err := fn()
	
	// Update circuit breaker state based on result
	if err != nil {
		cb.onFailure()
		return err
	}
	
	cb.onSuccess()
	return nil
}

// canExecute checks if the circuit breaker allows execution
func (cb *NetworkCircuitBreaker) canExecute() bool {
	switch cb.state {
	case CircuitBreakerStateClosed:
		return true
	case CircuitBreakerStateOpen:
		// Check if enough time has passed to try again
		if time.Since(cb.lastFailure) > cb.config.ResetTimeout {
			cb.state = CircuitBreakerStateHalfOpen
			return true
		}
		return false
	case CircuitBreakerStateHalfOpen:
		return true
	}
	return false
}

// onSuccess handles successful execution
func (cb *NetworkCircuitBreaker) onSuccess() {
	cb.failures = 0
	cb.lastSuccess = time.Now()
	cb.state = CircuitBreakerStateClosed
}

// onFailure handles failed execution
func (cb *NetworkCircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailure = time.Now()
	
	if cb.failures >= cb.config.MaxFailures {
		cb.state = CircuitBreakerStateOpen
	}
}

// GetState returns the current state of the circuit breaker
func (cb *NetworkCircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetFailures returns the current failure count
func (cb *NetworkCircuitBreaker) GetFailures() int {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.failures
}

// Reset resets the circuit breaker to closed state
func (cb *NetworkCircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.state = CircuitBreakerStateClosed
	cb.failures = 0
	cb.lastSuccess = time.Now()
}