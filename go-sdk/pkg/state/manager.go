package state

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// ManagerOptions configures the StateManager
type ManagerOptions struct {
	// Storage configuration
	MaxHistorySize int
	EnableCaching  bool

	// Conflict resolution configuration
	ConflictStrategy ConflictResolutionStrategy
	MaxRetries       int
	RetryDelay       time.Duration

	// Validation configuration
	ValidationRules []ValidationRule
	StrictMode      bool

	// Rollback configuration
	MaxCheckpoints       int
	CheckpointInterval   time.Duration
	AutoCheckpoint       bool
	CompressCheckpoints  bool

	// Event handling configuration
	EventBufferSize      int
	ProcessingWorkers    int
	EventRetryBackoff    time.Duration

	// Performance configuration
	CacheSize          int
	CacheTTL           time.Duration
	EnableCompression  bool
	EnableBatching     bool
	BatchSize          int
	BatchTimeout       time.Duration

	// Monitoring configuration
	EnableMetrics      bool
	MetricsInterval    time.Duration
	EnableTracing      bool
}

// DefaultManagerOptions returns sensible defaults
func DefaultManagerOptions() ManagerOptions {
	return ManagerOptions{
		MaxHistorySize:       100,
		ConflictStrategy:     LastWriteWins,
		MaxRetries:           3,
		RetryDelay:           100 * time.Millisecond,
		StrictMode:           true,
		MaxCheckpoints:       10,
		CheckpointInterval:   5 * time.Minute,
		AutoCheckpoint:       true,
		CompressCheckpoints:  true,
		EventBufferSize:      1000,
		ProcessingWorkers:    4,
		EventRetryBackoff:    time.Second,
		CacheSize:            1000,
		CacheTTL:             5 * time.Minute,
		EnableCompression:    true,
		EnableBatching:       true,
		BatchSize:            100,
		BatchTimeout:         100 * time.Millisecond,
		EnableMetrics:        true,
		MetricsInterval:      30 * time.Second,
		EnableTracing:        false,
	}
}

// StateManager is the main entry point for state management
type StateManager struct {
	// Core components
	store            *StateStore
	deltaComputer    *DeltaComputer
	conflictResolver *ConflictResolverImpl
	validator        StateValidator
	rollbackManager  *StateRollback
	eventHandler     *StateEventHandler

	// Configuration
	options ManagerOptions

	// Runtime state
	mu              sync.RWMutex
	activeContexts  map[string]*StateContext
	updateQueue     chan *updateRequest
	eventQueue      chan *stateEvent
	metricsCollector *metricsCollector

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// StateContext represents an active state context
type StateContext struct {
	ID           string
	StateID      string
	Created      time.Time
	LastAccessed time.Time
	Metadata     map[string]interface{}
}

// updateRequest represents a state update request
type updateRequest struct {
	contextID string
	stateID   string
	updates   map[string]interface{}
	options   UpdateOptions
	result    chan updateResult
}

// updateResult represents the result of an update
type updateResult struct {
	newVersion string
	delta      JSONPatch
	err        error
}

// stateEvent represents a state-related event
type stateEvent struct {
	Type      string
	StateID   string
	Version   string
	Timestamp time.Time
	Data      map[string]interface{}
}

// UpdateOptions configures update behavior
type UpdateOptions struct {
	// Validation options
	SkipValidation bool
	CustomRules    []ValidationRule

	// Conflict resolution
	ConflictStrategy ConflictResolutionStrategy
	ForceUpdate      bool

	// Checkpoint options
	CreateCheckpoint bool
	CheckpointName   string

	// Event options
	SuppressEvents bool
	EventMetadata  map[string]interface{}

	// Performance options
	Priority int
	Timeout  time.Duration
}

// NewStateManager creates a new state manager with the given options
func NewStateManager(opts ManagerOptions) (*StateManager, error) {
	// Create core components
	store := NewStateStore()

	deltaComputer := NewDeltaComputer(DefaultDeltaOptions())
	
	conflictResolver := NewConflictResolver(opts.ConflictStrategy)

	validator := NewStateValidator(nil) // No schema for now
	for _, rule := range opts.ValidationRules {
		if err := validator.AddRule(rule); err != nil {
			return nil, fmt.Errorf("failed to add validation rule: %w", err)
		}
	}

	rollbackManager := NewStateRollback(store)

	eventHandler := NewStateEventHandler(store,
		WithBatchSize(opts.BatchSize),
		WithBatchTimeout(opts.BatchTimeout),
	)

	ctx, cancel := context.WithCancel(context.Background())

	sm := &StateManager{
		store:            store,
		deltaComputer:    deltaComputer,
		conflictResolver: conflictResolver,
		validator:        validator,
		rollbackManager:  rollbackManager,
		eventHandler:     eventHandler,
		options:          opts,
		activeContexts:   make(map[string]*StateContext),
		updateQueue:      make(chan *updateRequest, opts.BatchSize*2),
		eventQueue:       make(chan *stateEvent, opts.EventBufferSize),
		ctx:              ctx,
		cancel:           cancel,
	}

	if opts.EnableMetrics {
		sm.metricsCollector = newMetricsCollector(opts.MetricsInterval)
		sm.wg.Add(1)
		go sm.collectMetrics()
	}

	// Start background workers
	sm.wg.Add(1)
	go sm.processUpdates()

	sm.wg.Add(1)
	go sm.processEvents()

	if opts.AutoCheckpoint {
		sm.wg.Add(1)
		go sm.autoCheckpoint()
	}

	return sm, nil
}

// CreateContext creates a new state context
func (sm *StateManager) CreateContext(stateID string, metadata map[string]interface{}) (string, error) {
	contextID := uuid.New().String()
	
	sm.mu.Lock()
	sm.activeContexts[contextID] = &StateContext{
		ID:           contextID,
		StateID:      stateID,
		Created:      time.Now(),
		LastAccessed: time.Now(),
		Metadata:     metadata,
	}
	sm.mu.Unlock()

	// Emit context created event
	sm.emitEvent(&stateEvent{
		Type:      "context.created",
		StateID:   stateID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"contextID": contextID,
			"metadata":  metadata,
		},
	})

	return contextID, nil
}

// GetState retrieves the current state
func (sm *StateManager) GetState(contextID, stateID string) (interface{}, error) {
	// Update context access time
	sm.updateContextAccess(contextID)

	// Get from store with caching
	state, err := sm.store.Get("/")
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	// Validate if strict mode is enabled
	if sm.options.StrictMode {
		if stateMap, ok := state.(map[string]interface{}); ok {
			result, err := sm.validator.Validate(stateMap)
			if err != nil {
				return nil, fmt.Errorf("state validation error: %w", err)
			}
			if !result.Valid {
				return nil, fmt.Errorf("state validation failed: %v", result.Errors)
			}
		}
	}

	return state, nil
}

// UpdateState updates the state with conflict resolution and validation
func (sm *StateManager) UpdateState(contextID, stateID string, updates map[string]interface{}, opts UpdateOptions) (JSONPatch, error) {
	// Create update request
	req := &updateRequest{
		contextID: contextID,
		stateID:   stateID,
		updates:   updates,
		options:   opts,
		result:    make(chan updateResult, 1),
	}

	// Submit to update queue
	select {
	case sm.updateQueue <- req:
	case <-time.After(opts.Timeout):
		return nil, fmt.Errorf("update queue timeout")
	}

	// Wait for result
	select {
	case result := <-req.result:
		if result.err != nil {
			return nil, result.err
		}
		return result.delta, nil
	case <-time.After(opts.Timeout):
		return nil, fmt.Errorf("update processing timeout")
	}
}

// Subscribe subscribes to state change events
func (sm *StateManager) Subscribe(path string, handler func(StateChange)) func() {
	return sm.store.Subscribe(path, handler)
}

// Unsubscribe removes an event subscription
func (sm *StateManager) Unsubscribe(unsubscribe func()) {
	if unsubscribe != nil {
		unsubscribe()
	}
}

// CreateCheckpoint creates a manual checkpoint
func (sm *StateManager) CreateCheckpoint(stateID, name string) (string, error) {
	// Get state to ensure it exists
	_, err := sm.store.Get("/")
	if err != nil {
		return "", fmt.Errorf("failed to get state for checkpoint: %w", err)
	}

	err = sm.rollbackManager.CreateMarker(name)
	if err != nil {
		return "", fmt.Errorf("failed to create checkpoint: %w", err)
	}
	
	checkpointID := uuid.New().String()

	sm.emitEvent(&stateEvent{
		Type:      "checkpoint.created",
		StateID:   stateID,
		Version:   "",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"checkpointID": checkpointID,
			"name":         name,
		},
	})

	return checkpointID, nil
}

// Rollback rolls back to a checkpoint
func (sm *StateManager) Rollback(stateID, checkpointID string) error {
	err := sm.rollbackManager.RollbackToMarker(checkpointID)
	if err != nil {
		return fmt.Errorf("failed to rollback: %w", err)
	}

	sm.emitEvent(&stateEvent{
		Type:      "state.rolledback",
		StateID:   stateID,
		Version:   "",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"checkpointID": checkpointID,
		},
	})

	return nil
}

// GetHistory retrieves state history
func (sm *StateManager) GetHistory(stateID string, limit int) ([]*StateVersion, error) {
	return sm.store.GetHistory()
}

// GetMetrics returns current metrics
func (sm *StateManager) GetMetrics() map[string]interface{} {
	if sm.metricsCollector == nil {
		return nil
	}
	return sm.metricsCollector.GetMetrics()
}

// Close shuts down the state manager
func (sm *StateManager) Close() error {
	// Cancel context to signal shutdown
	sm.cancel()

	// Close channels
	close(sm.updateQueue)
	close(sm.eventQueue)

	// Wait for workers to finish
	sm.wg.Wait()

	// Close components
	// Store and EventHandler don't need explicit closing

	return nil
}

// processUpdates processes update requests with batching
func (sm *StateManager) processUpdates() {
	defer sm.wg.Done()

	batch := make([]*updateRequest, 0, sm.options.BatchSize)
	timer := time.NewTimer(sm.options.BatchTimeout)
	defer timer.Stop()

	for {
		select {
		case req, ok := <-sm.updateQueue:
			if !ok {
				// Process remaining batch
				if len(batch) > 0 {
					sm.processBatch(batch)
				}
				return
			}

			batch = append(batch, req)

			if len(batch) >= sm.options.BatchSize {
				sm.processBatch(batch)
				batch = batch[:0]
				timer.Reset(sm.options.BatchTimeout)
			}

		case <-timer.C:
			if len(batch) > 0 {
				sm.processBatch(batch)
				batch = batch[:0]
			}
			timer.Reset(sm.options.BatchTimeout)

		case <-sm.ctx.Done():
			// Process remaining batch
			if len(batch) > 0 {
				sm.processBatch(batch)
			}
			return
		}
	}
}

// processBatch processes a batch of updates
func (sm *StateManager) processBatch(batch []*updateRequest) {
	// Group by state ID for efficiency
	groups := make(map[string][]*updateRequest)
	for _, req := range batch {
		groups[req.stateID] = append(groups[req.stateID], req)
	}

	// Process each group
	for stateID, requests := range groups {
		sm.processStateUpdates(stateID, requests)
	}
}

// processStateUpdates processes updates for a single state
func (sm *StateManager) processStateUpdates(stateID string, requests []*updateRequest) {
	// Get current state
	currentState, err := sm.store.Get(stateID)
	if err != nil {
		// Send error to all requests
		for _, req := range requests {
			req.result <- updateResult{err: fmt.Errorf("failed to get state: %w", err)}
		}
		return
	}

	// Process each request sequentially
	for _, req := range requests {
		result := sm.processSingleUpdate(currentState, req)
		req.result <- result

		if result.err == nil {
			// Apply the delta to update current state for next request
			newState, _ := result.delta.Apply(currentState)
			currentState = newState
		}
	}
}

// processSingleUpdate processes a single update request
func (sm *StateManager) processSingleUpdate(state interface{}, req *updateRequest) updateResult {
	// Update context access
	sm.updateContextAccess(req.contextID)

	// Compute delta between current state and updates
	delta, err := sm.deltaComputer.ComputeDelta(state, req.updates)
	if err != nil {
		return updateResult{err: fmt.Errorf("delta computation failed: %w", err)}
	}

	// Apply the delta to get the new state
	newState, err := delta.Apply(state)
	if err != nil {
		return updateResult{err: fmt.Errorf("delta application failed: %w", err)}
	}

	// Validate unless skipped
	if !req.options.SkipValidation {
		if stateMap, ok := newState.(map[string]interface{}); ok {
			result, err := sm.validator.Validate(stateMap)
			if err != nil {
				return updateResult{err: fmt.Errorf("validation error: %w", err)}
			}
			if !result.Valid {
				return updateResult{err: fmt.Errorf("validation failed: %v", result.Errors)}
			}
		}
	}

	// Apply the patch to the store
	if err := sm.store.ApplyPatch(delta); err != nil {
		return updateResult{err: fmt.Errorf("store update failed: %w", err)}
	}

	// Create checkpoint if requested
	if req.options.CreateCheckpoint {
		if err := sm.rollbackManager.CreateMarker(req.options.CheckpointName); err != nil {
			// Log error but don't fail the update
			sm.logError("checkpoint creation failed", err)
		}
	}

	// Emit events unless suppressed
	if !req.options.SuppressEvents {
		sm.emitEvent(&stateEvent{
			Type:      "state.updated",
			StateID:   req.stateID,
			Version:   "",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"contextID": req.contextID,
				"delta":     delta,
				"metadata":  req.options.EventMetadata,
			},
		})
	}

	return updateResult{
		newVersion: uuid.New().String(),
		delta:      delta,
		err:        nil,
	}
}

// processEvents processes state events
func (sm *StateManager) processEvents() {
	defer sm.wg.Done()

	for {
		select {
		case event, ok := <-sm.eventQueue:
			if !ok {
				return
			}

			// Process state events based on type
			switch event.Type {
			case "state_snapshot":
				if snapshot, ok := event.Data["snapshot"]; ok {
					snapEvent := events.NewStateSnapshotEvent(snapshot)
					if err := sm.eventHandler.HandleStateSnapshot(snapEvent); err != nil {
						sm.logError("snapshot event processing failed", err)
					}
				}
			case "state_delta":
				if delta, ok := event.Data["delta"].([]events.JSONPatchOperation); ok {
					deltaEvent := events.NewStateDeltaEvent(delta)
					if err := sm.eventHandler.HandleStateDelta(deltaEvent); err != nil {
						sm.logError("delta event processing failed", err)
					}
				}
			}

		case <-sm.ctx.Done():
			return
		}
	}
}

// autoCheckpoint creates automatic checkpoints
func (sm *StateManager) autoCheckpoint() {
	defer sm.wg.Done()

	ticker := time.NewTicker(sm.options.CheckpointInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.createAutoCheckpoints()

		case <-sm.ctx.Done():
			return
		}
	}
}

// createAutoCheckpoints creates checkpoints for all active states
func (sm *StateManager) createAutoCheckpoints() {
	sm.mu.RLock()
	stateIDs := make(map[string]bool)
	for _, ctx := range sm.activeContexts {
		stateIDs[ctx.StateID] = true
	}
	sm.mu.RUnlock()

	for _ = range stateIDs {
		// Ensure state exists before creating checkpoint
		_, err := sm.store.Get("/")
		if err != nil {
			sm.logError("auto checkpoint failed to get state", err)
			continue
		}

		name := fmt.Sprintf("auto-%s", time.Now().Format("20060102-150405"))
		if err := sm.rollbackManager.CreateMarker(name); err != nil {
			sm.logError("auto checkpoint creation failed", err)
		}
	}
}

// collectMetrics collects performance metrics
func (sm *StateManager) collectMetrics() {
	defer sm.wg.Done()

	ticker := time.NewTicker(sm.options.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.metricsCollector.Collect(sm)

		case <-sm.ctx.Done():
			return
		}
	}
}

// Helper methods

func (sm *StateManager) updateContextAccess(contextID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if ctx, exists := sm.activeContexts[contextID]; exists {
		ctx.LastAccessed = time.Now()
	}
}

func (sm *StateManager) emitEvent(event *stateEvent) {
	select {
	case sm.eventQueue <- event:
	default:
		// Queue full, log and drop
		sm.logError("event queue full, dropping event", nil)
	}
}

func (sm *StateManager) logError(msg string, err error) {
	// In production, this would use a proper logging framework
	if err != nil {
		fmt.Printf("[StateManager] ERROR: %s: %v\n", msg, err)
	} else {
		fmt.Printf("[StateManager] ERROR: %s\n", msg)
	}
}

// Utility functions

func applyUpdates(data, updates map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Copy existing data
	for k, v := range data {
		result[k] = v
	}
	
	// Apply updates
	for k, v := range updates {
		if v == nil {
			delete(result, k)
		} else {
			result[k] = v
		}
	}
	
	return result
}

func generateVersion() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), uuid.New().String()[:8])
}

// metricsCollector collects and stores metrics
type metricsCollector struct {
	mu       sync.RWMutex
	metrics  map[string]interface{}
	interval time.Duration
}

func newMetricsCollector(interval time.Duration) *metricsCollector {
	return &metricsCollector{
		metrics:  make(map[string]interface{}),
		interval: interval,
	}
}

func (mc *metricsCollector) Collect(sm *StateManager) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	sm.mu.RLock()
	activeContexts := len(sm.activeContexts)
	sm.mu.RUnlock()

	mc.metrics = map[string]interface{}{
		"active_contexts":    activeContexts,
		"update_queue_size": len(sm.updateQueue),
		"event_queue_size":  len(sm.eventQueue),
		"timestamp":         time.Now(),
	}

	// Collect component metrics
	// Store and EventHandler metrics could be added here if needed
}

func (mc *metricsCollector) GetMetrics() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Return a copy
	result := make(map[string]interface{})
	for k, v := range mc.metrics {
		result[k] = v
	}
	return result
}