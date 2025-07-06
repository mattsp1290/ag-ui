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
	securityValidator *SecurityValidator

	// Configuration
	options ManagerOptions

	// Runtime state
	mu              sync.RWMutex
	activeContexts  sync.Map // Use sync.Map for better concurrency
	updateQueue     chan *updateRequest
	eventQueue      chan *stateEvent
	metricsCollector *metricsCollector

	// Context management
	contextTTL      time.Duration
	lastCleanup     time.Time
	cleanupInterval time.Duration

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
	mu           sync.RWMutex // Protect concurrent access to LastAccessed
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

	// Create security validator with safe defaults
	securityValidator := NewSecurityValidator(DefaultSecurityConfig())
	
	sm := &StateManager{
		store:             store,
		deltaComputer:     deltaComputer,
		conflictResolver:  conflictResolver,
		validator:         validator,
		rollbackManager:   rollbackManager,
		eventHandler:      eventHandler,
		securityValidator: securityValidator,
		options:           opts,
		updateQueue:       make(chan *updateRequest, opts.BatchSize*2),
		eventQueue:        make(chan *stateEvent, opts.EventBufferSize),
		contextTTL:        1 * time.Hour,   // Default context TTL
		cleanupInterval:   15 * time.Minute, // Default cleanup interval
		lastCleanup:       time.Now(),
		ctx:               ctx,
		cancel:            cancel,
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

	// Start context cleanup worker
	sm.wg.Add(1)
	go sm.contextCleanup()

	return sm, nil
}

// CreateContext creates a new state context
func (sm *StateManager) CreateContext(ctx context.Context, stateID string, metadata map[string]interface{}) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("context cannot be nil")
	}
	if stateID == "" {
		return "", fmt.Errorf("stateID cannot be empty")
	}
	
	// Check if manager is shutting down
	select {
	case <-sm.ctx.Done():
		return "", fmt.Errorf("manager is shutting down: %w", sm.ctx.Err())
	default:
	}
	
	contextID := uuid.New().String()
	now := time.Now()
	
	// Security validation for metadata
	if err := sm.securityValidator.ValidateMetadata(metadata); err != nil {
		return "", fmt.Errorf("security validation failed for metadata: %w", err)
	}
	
	// Create metadata copy to avoid external modifications
	metadataCopy := make(map[string]interface{})
	if metadata != nil {
		for k, v := range metadata {
			metadataCopy[k] = v
		}
	}
	
	context := &StateContext{
		ID:           contextID,
		StateID:      stateID,
		Created:      now,
		LastAccessed: now,
		Metadata:     metadataCopy,
	}
	
	sm.activeContexts.Store(contextID, context)

	// Trigger cleanup if needed
	sm.maybeCleanupContexts()

	// Emit context created event
	sm.emitEvent(&stateEvent{
		Type:      "context.created",
		StateID:   stateID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"contextID": contextID,
			"metadata":  metadataCopy,
		},
	})

	return contextID, nil
}

// GetState retrieves the current state
func (sm *StateManager) GetState(ctx context.Context, contextID, stateID string) (interface{}, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if contextID == "" {
		return nil, fmt.Errorf("contextID cannot be empty")
	}
	if stateID == "" {
		return nil, fmt.Errorf("stateID cannot be empty")
	}
	
	// Check if manager is shutting down
	select {
	case <-sm.ctx.Done():
		return nil, fmt.Errorf("manager is shutting down: %w", sm.ctx.Err())
	default:
	}

	// Update context access time
	sm.updateContextAccess(contextID)

	// Get from store with caching
	state, err := sm.store.Get("/")
	if err != nil {
		return nil, fmt.Errorf("failed to get state for stateID %s: %w", stateID, err)
	}

	// Validate if strict mode is enabled
	if sm.options.StrictMode {
		if stateMap, ok := state.(map[string]interface{}); ok {
			if sm.validator == nil {
				return nil, fmt.Errorf("validator is nil but strict mode is enabled")
			}
			result, err := sm.validator.Validate(stateMap)
			if err != nil {
				return nil, fmt.Errorf("state validation error for stateID %s: %w", stateID, err)
			}
			if !result.Valid {
				return nil, fmt.Errorf("state validation failed for stateID %s: %v", stateID, result.Errors)
			}
		}
	}

	return state, nil
}

// UpdateState updates the state with conflict resolution and validation
func (sm *StateManager) UpdateState(ctx context.Context, contextID, stateID string, updates map[string]interface{}, opts UpdateOptions) (JSONPatch, error) {
	// Create update request
	req := &updateRequest{
		contextID: contextID,
		stateID:   stateID,
		updates:   updates,
		options:   opts,
		result:    make(chan updateResult, 1),
	}

	// Set default timeout if not specified
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Submit to update queue
	select {
	case sm.updateQueue <- req:
	case <-timeoutCtx.Done():
		return nil, fmt.Errorf("update queue timeout: %w", timeoutCtx.Err())
	case <-sm.ctx.Done():
		return nil, fmt.Errorf("manager shutting down: %w", sm.ctx.Err())
	}

	// Wait for result
	select {
	case result := <-req.result:
		if result.err != nil {
			return nil, result.err
		}
		return result.delta, nil
	case <-timeoutCtx.Done():
		return nil, fmt.Errorf("update processing timeout: %w", timeoutCtx.Err())
	case <-sm.ctx.Done():
		return nil, fmt.Errorf("manager shutting down: %w", sm.ctx.Err())
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
func (sm *StateManager) CreateCheckpoint(ctx context.Context, stateID, name string) (string, error) {
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
func (sm *StateManager) Rollback(ctx context.Context, stateID, checkpointID string) error {
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
func (sm *StateManager) GetHistory(ctx context.Context, stateID string, limit int) ([]*StateVersion, error) {
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

	// Security validation for updates
	if err := sm.securityValidator.ValidateState(req.updates); err != nil {
		return updateResult{err: fmt.Errorf("security validation failed for updates: %w", err)}
	}

	// Compute delta between current state and updates
	delta, err := sm.deltaComputer.ComputeDelta(state, req.updates)
	if err != nil {
		return updateResult{err: fmt.Errorf("delta computation failed: %w", err)}
	}
	
	// Security validation for computed delta
	if err := sm.securityValidator.ValidatePatch(delta); err != nil {
		return updateResult{err: fmt.Errorf("security validation failed for delta: %w", err)}
	}

	// Apply the delta to get the new state
	newState, err := delta.Apply(state)
	if err != nil {
		return updateResult{err: fmt.Errorf("delta application failed: %w", err)}
	}
	
	// Security validation for resulting state
	if err := sm.securityValidator.ValidateState(newState); err != nil {
		return updateResult{err: fmt.Errorf("security validation failed for new state: %w", err)}
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
	stateIDs := make(map[string]bool)
	sm.activeContexts.Range(func(key, value interface{}) bool {
		ctx := value.(*StateContext)
		stateIDs[ctx.StateID] = true
		return true
	})

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
	if value, exists := sm.activeContexts.Load(contextID); exists {
		ctx := value.(*StateContext)
		ctx.mu.Lock()
		ctx.LastAccessed = time.Now()
		ctx.mu.Unlock()
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

	// Count active contexts using sync.Map
	activeContexts := 0
	sm.activeContexts.Range(func(key, value interface{}) bool {
		activeContexts++
		return true
	})

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

// contextCleanup runs background cleanup for expired contexts
func (sm *StateManager) contextCleanup() {
	defer sm.wg.Done()

	ticker := time.NewTicker(sm.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.cleanupExpiredContexts()
		case <-sm.ctx.Done():
			return
		}
	}
}

// maybeCleanupContexts triggers cleanup if enough time has passed
func (sm *StateManager) maybeCleanupContexts() {
	now := time.Now()
	if now.Sub(sm.lastCleanup) < sm.cleanupInterval {
		return
	}
	
	sm.lastCleanup = now
	go sm.cleanupExpiredContexts()
}

// cleanupExpiredContexts removes expired contexts
func (sm *StateManager) cleanupExpiredContexts() {
	cutoff := time.Now().Add(-sm.contextTTL)
	
	sm.activeContexts.Range(func(key, value interface{}) bool {
		ctx := value.(*StateContext)
		ctx.mu.RLock()
		lastAccessed := ctx.LastAccessed
		ctx.mu.RUnlock()
		
		if lastAccessed.Before(cutoff) {
			sm.activeContexts.Delete(key)
			
			// Emit context expired event
			sm.emitEvent(&stateEvent{
				Type:      "context.expired",
				StateID:   ctx.StateID,
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"contextID": ctx.ID,
					"reason":    "expired",
				},
			})
		}
		return true
	})
}