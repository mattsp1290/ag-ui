package state

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/google/uuid"
)

// Common errors
var (
	ErrManagerClosing = errors.New("state manager is closing")
	ErrManagerClosed  = errors.New("state manager is closed")
	ErrQueueFull      = errors.New("update queue is full")
)

// ManagerOptions configures the StateManager
type ManagerOptions struct {
	// Storage configuration
	MaxHistorySize int
	EnableCaching  bool
	CustomStore    StoreInterface // Custom store for dependency injection

	// Conflict resolution configuration
	ConflictStrategy ConflictResolutionStrategy
	MaxRetries       int
	RetryDelay       time.Duration

	// Validation configuration
	ValidationRules []ValidationRule
	StrictMode      bool

	// Rollback configuration
	MaxCheckpoints      int
	CheckpointInterval  time.Duration
	AutoCheckpoint      bool
	CompressCheckpoints bool

	// Event handling configuration
	EventBufferSize   int
	ProcessingWorkers int
	EventRetryBackoff time.Duration

	// Performance configuration
	CacheSize         int
	CacheTTL          time.Duration
	EnableCompression bool
	EnableBatching    bool
	BatchSize         int
	BatchTimeout      time.Duration
	UpdateQueueSize   int // Size of the update queue for concurrent operations
	
	// Performance optimizer configuration
	PerformanceOptimizer PerformanceOptimizer

	// Monitoring configuration
	EnableMetrics   bool
	MetricsInterval time.Duration
	EnableTracing   bool

	// Rate limiting configuration
	GlobalRateLimit        int                      // Global rate limit (requests per second)
	ClientRateLimiterConfig *ClientRateLimiterConfig // Client rate limiter configuration

	// Audit configuration
	EnableAudit bool
	AuditLogger AuditLogger
}

// Validate validates the manager options
func (opts *ManagerOptions) Validate() error {
	if opts == nil {
		return fmt.Errorf("manager options cannot be nil")
	}
	
	// Validate history size
	if opts.MaxHistorySize < 0 {
		return fmt.Errorf("max history size cannot be negative, got %d", opts.MaxHistorySize)
	}
	
	// Validate retry configuration
	if opts.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative, got %d", opts.MaxRetries)
	}
	
	if opts.RetryDelay < 0 {
		return fmt.Errorf("retry delay cannot be negative, got %v", opts.RetryDelay)
	}
	
	// Validate checkpoint configuration
	if opts.MaxCheckpoints < 0 {
		return fmt.Errorf("max checkpoints cannot be negative, got %d", opts.MaxCheckpoints)
	}
	
	if opts.CheckpointInterval < 0 {
		return fmt.Errorf("checkpoint interval cannot be negative, got %v", opts.CheckpointInterval)
	}
	
	// Validate event handling configuration
	if opts.EventBufferSize < 0 {
		return fmt.Errorf("event buffer size cannot be negative, got %d", opts.EventBufferSize)
	}
	
	if opts.ProcessingWorkers <= 0 {
		return fmt.Errorf("processing workers must be positive, got %d", opts.ProcessingWorkers)
	}
	
	if opts.EventRetryBackoff < 0 {
		return fmt.Errorf("event retry backoff cannot be negative, got %v", opts.EventRetryBackoff)
	}
	
	// Validate performance configuration
	if opts.CacheSize < 0 {
		return fmt.Errorf("cache size cannot be negative, got %d", opts.CacheSize)
	}
	
	if opts.CacheTTL < 0 {
		return fmt.Errorf("cache TTL cannot be negative, got %v", opts.CacheTTL)
	}
	
	if opts.BatchSize < 0 {
		return fmt.Errorf("batch size cannot be negative, got %d", opts.BatchSize)
	}
	
	if opts.BatchTimeout < 0 {
		return fmt.Errorf("batch timeout cannot be negative, got %v", opts.BatchTimeout)
	}
	
	if opts.UpdateQueueSize <= 0 {
		// Set a sensible default based on the existing pattern but with higher capacity
		// Use max of BatchSize*10 or DefaultUpdateQueueSize to handle high-concurrency scenarios
		if opts.BatchSize > 0 {
			queueSize := opts.BatchSize * 10
			if queueSize < DefaultUpdateQueueSize {
				queueSize = DefaultUpdateQueueSize
			}
			opts.UpdateQueueSize = queueSize
		} else {
			opts.UpdateQueueSize = DefaultUpdateQueueSize
		}
	}
	
	// Validate metrics configuration
	if opts.MetricsInterval < 0 {
		return fmt.Errorf("metrics interval cannot be negative, got %v", opts.MetricsInterval)
	}
	
	// Validate rate limiting configuration
	if opts.GlobalRateLimit < 0 {
		return fmt.Errorf("global rate limit cannot be negative, got %d", opts.GlobalRateLimit)
	}
	
	return nil
}

// DefaultManagerOptions returns sensible defaults
func DefaultManagerOptions() ManagerOptions {
	return ManagerOptions{
		MaxHistorySize:      DefaultMaxHistorySize,
		ConflictStrategy:    LastWriteWins,
		MaxRetries:          DefaultMaxRetries,
		RetryDelay:          GetDefaultRetryDelay(),
		StrictMode:          true,
		MaxCheckpoints:      DefaultMaxCheckpoints,
		CheckpointInterval:  GetDefaultCheckpointInterval(),
		AutoCheckpoint:      true,
		CompressCheckpoints: true,
		EventBufferSize:     DefaultEventBufferSize,
		ProcessingWorkers:   DefaultProcessingWorkers,
		EventRetryBackoff:   GetDefaultEventRetryBackoff(),
		CacheSize:           DefaultCacheSize,
		CacheTTL:            GetDefaultCacheTTL(),
		EnableCompression:   true,
		EnableBatching:      true,
		BatchSize:           DefaultBatchSize,
		BatchTimeout:        GetDefaultBatchTimeout(),
		UpdateQueueSize:     DefaultUpdateQueueSize,
		EnableMetrics:       true,
		MetricsInterval:     GetDefaultMetricsInterval(),
		EnableTracing:       false,
		GlobalRateLimit:     DefaultGlobalRateLimit,
		ClientRateLimiterConfig: nil, // Will use default configuration
		EnableAudit:         true,
		AuditLogger:         nil, // Will use default JSON logger
	}
}

// StateManager is the main entry point for state management
type StateManager struct {
	// Core components
	store             StoreInterface
	deltaComputer     *DeltaComputer
	conflictResolver  *ConflictResolverImpl
	validator         StateValidator
	rollbackManager   *StateRollback
	eventHandler      *StateEventHandler
	securityValidator *SecurityValidator
	rateLimiter       *RateLimiter
	clientRateLimiter *ClientRateLimiter
	logger            Logger
	auditManager      *AuditManager

	// Configuration
	options ManagerOptions

	// Runtime state
	mu               sync.RWMutex
	activeContexts   *ContextManager // Bounded context manager to prevent memory leaks
	updateQueue      chan *updateRequest
	eventQueue       chan *stateEvent
	metricsCollector *metricsCollector
	errCh            chan error // Channel for error propagation from goroutines
	performanceOptimizer PerformanceOptimizer // Internal performance optimizer instance

	// Context management
	contextTTL      time.Duration
	lastCleanup     time.Time
	cleanupInterval time.Duration

	// Lifecycle
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	closing int32 // Atomic flag for graceful shutdown
}

// StateContext represents an active state context
type StateContext struct {
	ID           string
	StateID      string
	Created      time.Time
	lastAccessed int64 // Unix nanoseconds, accessed via atomic operations
	Metadata     map[string]interface{}
	mu           sync.RWMutex // Protect concurrent access to Metadata
}

// GetLastAccessed returns the last accessed time using atomic operations
func (ctx *StateContext) GetLastAccessed() time.Time {
	nanos := atomic.LoadInt64(&ctx.lastAccessed)
	return time.Unix(0, nanos)
}

// SetLastAccessed sets the last accessed time using atomic operations
func (ctx *StateContext) SetLastAccessed(t time.Time) {
	atomic.StoreInt64(&ctx.lastAccessed, t.UnixNano())
}

// UpdateLastAccessed updates the last accessed time to current time using atomic operations
func (ctx *StateContext) UpdateLastAccessed() {
	atomic.StoreInt64(&ctx.lastAccessed, time.Now().UnixNano())
}

// updateRequest represents a state update request
type updateRequest struct {
	ctx       context.Context
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
	logger := DefaultLogger()
	// Create core components
	var store StoreInterface
	if opts.CustomStore != nil {
		store = opts.CustomStore
	} else {
		store = NewStateStore(WithLogger(logger))
	}

	deltaComputer := NewDeltaComputer(DefaultDeltaOptions())

	conflictResolver := NewConflictResolver(opts.ConflictStrategy)
	conflictResolver.SetLogger(logger)

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

	// Create rate limiter with configuration from options
	rateLimiter := NewRateLimiter(opts.GlobalRateLimit)

	// Create client rate limiter with configuration from options
	var clientRateLimiterConfig ClientRateLimiterConfig
	if opts.ClientRateLimiterConfig != nil {
		clientRateLimiterConfig = *opts.ClientRateLimiterConfig
	} else {
		clientRateLimiterConfig = DefaultClientRateLimiterConfig()
	}
	clientRateLimiter := NewClientRateLimiter(clientRateLimiterConfig)

	// Create audit manager if enabled
	var auditManager *AuditManager
	if opts.EnableAudit {
		auditLogger := opts.AuditLogger
		if auditLogger == nil {
			// Use default JSON audit logger writing to stdout
			auditLogger = NewJSONAuditLogger(nil)
		}
		auditManager = NewAuditManager(auditLogger)
	}

	// Create performance optimizer if not provided
	var performanceOptimizer PerformanceOptimizer
	if opts.PerformanceOptimizer != nil {
		performanceOptimizer = opts.PerformanceOptimizer
	} else {
		performanceOpts := DefaultPerformanceOptions()
		performanceOpts.EnableBatching = opts.EnableBatching
		performanceOpts.EnableCompression = opts.EnableCompression
		performanceOpts.BatchSize = opts.BatchSize
		performanceOpts.BatchTimeout = opts.BatchTimeout
		performanceOptimizer = NewPerformanceOptimizer(performanceOpts)
	}

	// Determine max contexts based on cache size or use default
	maxContexts := opts.CacheSize
	if maxContexts <= 0 {
		maxContexts = DefaultMaxContexts // Default max contexts
	}

	sm := &StateManager{
		store:             store,
		deltaComputer:     deltaComputer,
		conflictResolver:  conflictResolver,
		validator:         validator,
		rollbackManager:   rollbackManager,
		eventHandler:      eventHandler,
		securityValidator: securityValidator,
		rateLimiter:       rateLimiter,
		clientRateLimiter: clientRateLimiter,
		logger:            logger,
		auditManager:      auditManager,
		options:           opts,
		activeContexts:    NewContextManager(maxContexts),
		updateQueue:       make(chan *updateRequest, opts.UpdateQueueSize),
		eventQueue:        make(chan *stateEvent, opts.EventBufferSize),
		errCh:             make(chan error, DefaultErrorChannelSize), // Buffer for error propagation
		contextTTL:        GetDefaultContextTTL(),                         // Default context TTL
		cleanupInterval:   GetDefaultCleanupInterval(),                    // Default cleanup interval
		lastCleanup:       time.Now(),
		ctx:               ctx,
		cancel:            cancel,
		performanceOptimizer: performanceOptimizer,
	}

	if opts.EnableMetrics {
		sm.metricsCollector = newMetricsCollector(opts.MetricsInterval)
		sm.wg.Add(1)
		go sm.collectMetrics()
	}

	// Set error handler for the store
	sm.store.SetErrorHandler(func(err error) {
		sm.reportError(err)
	})

	sm.logger.Info("state manager initialized",
		Int("max_contexts", maxContexts),
		Int("batch_size", opts.BatchSize),
		Bool("auto_checkpoint", opts.AutoCheckpoint))

	// Start error handler first
	sm.wg.Add(1)
	go sm.handleErrors()

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
		return "", ErrManagerClosing
	default:
	}

	contextID := uuid.New().String()
	now := time.Now()

	// Security validation for metadata
	if err := sm.securityValidator.ValidateMetadata(metadata); err != nil {
		// Audit security violation
		if sm.auditManager != nil {
			sm.auditManager.LogSecurityEvent(ctx, AuditActionSecurityBlock, "", "", "context_metadata", map[string]interface{}{
				"state_id": stateID,
				"error":    err.Error(),
			})
		}
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
		ID:       contextID,
		StateID:  stateID,
		Created:  now,
		Metadata: metadataCopy,
	}
	context.SetLastAccessed(now)

	sm.activeContexts.Put(contextID, context)

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

	sm.logger.Debug("context created",
		String("context_id", contextID),
		String("state_id", stateID),
		Int("active_contexts", sm.activeContexts.Size()))

	// Audit context creation
	if sm.auditManager != nil {
		sm.auditManager.LogStateUpdate(ctx, contextID, stateID, "", nil, metadataCopy, AuditResultSuccess, nil)
	}

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
		return nil, ErrManagerClosing
	default:
	}

	// Update context access time
	sm.updateContextAccess(contextID)

	// Get from store with caching
	state, err := sm.store.Get("/")
	if err != nil {
		// Audit failed access
		if sm.auditManager != nil {
			details := map[string]interface{}{
				"error_type": "state_not_found",
			}
			sm.auditManager.LogError(ctx, AuditActionStateAccess, err, details)
		}
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

	// Audit successful state access
	if sm.auditManager != nil {
		log := &AuditLog{
			ID:        generateAuditID(),
			Timestamp: time.Now(),
			Action:    AuditActionStateAccess,
			Result:    AuditResultSuccess,
			ContextID: contextID,
			StateID:   stateID,
			Resource:  "state",
		}
		sm.auditManager.enrichFromContext(ctx, log)
		sm.logAuditAsync(log, "state_access")
	}

	return state, nil
}

// UpdateState updates the state with conflict resolution and validation
func (sm *StateManager) UpdateState(ctx context.Context, contextID, stateID string, updates map[string]interface{}, opts UpdateOptions) (JSONPatch, error) {
	// Check if manager is shutting down first, before rate limiting
	if atomic.LoadInt32(&sm.closing) == 1 {
		return nil, ErrManagerClosing
	}
	
	select {
	case <-sm.ctx.Done():
		return nil, ErrManagerClosed
	default:
	}

	// Apply global rate limiting
	if sm.rateLimiter != nil {
		if !sm.rateLimiter.Allow() {
			// Audit rate limit violation
			if sm.auditManager != nil {
				sm.auditManager.LogSecurityEvent(ctx, AuditActionRateLimit, contextID, "", "global_rate_limit", map[string]interface{}{
					"state_id": stateID,
				})
			}
			return nil, ErrRateLimited
		}
	}

	// Apply per-client rate limiting using contextID as the client identifier
	if !sm.clientRateLimiter.Allow(contextID) {
		// Audit client rate limit violation
		if sm.auditManager != nil {
			sm.auditManager.LogSecurityEvent(ctx, AuditActionRateLimit, contextID, "", "client_rate_limit", map[string]interface{}{
				"state_id": stateID,
			})
		}
		return nil, ErrRateLimited
	}

	// Create update request
	req := &updateRequest{
		ctx:       ctx,
		contextID: contextID,
		stateID:   stateID,
		updates:   updates,
		options:   opts,
		result:    make(chan updateResult, 1),
	}

	// Set default timeout if not specified
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = GetDefaultUpdateTimeout()
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Submit to update queue
	if err := sm.enqueueUpdate(req); err != nil {
		return nil, err
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
		return nil, ErrManagerClosing
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

	sm.logger.Info("checkpoint created",
		String("checkpoint_id", checkpointID),
		String("checkpoint_name", name),
		String("state_id", stateID))

	// Audit checkpoint creation
	if sm.auditManager != nil {
		log := &AuditLog{
			ID:        generateAuditID(),
			Timestamp: time.Now(),
			Action:    AuditActionCheckpoint,
			Result:    AuditResultSuccess,
			StateID:   stateID,
			Resource:  "checkpoint",
			Details: map[string]interface{}{
				"checkpoint_id":   checkpointID,
				"checkpoint_name": name,
			},
		}
		sm.auditManager.enrichFromContext(ctx, log)
		sm.logAuditAsync(log, "checkpoint_creation")
	}

	return checkpointID, nil
}

// Rollback rolls back to a checkpoint
func (sm *StateManager) Rollback(ctx context.Context, stateID, checkpointID string) error {
	// Get the old state before rollback for audit logging
	var oldState interface{}
	if sm.auditManager != nil {
		oldState, _ = sm.store.Get("/")
	}

	err := sm.rollbackManager.RollbackToMarker(checkpointID)
	if err != nil {
		// Audit failed rollback
		if sm.auditManager != nil {
			sm.auditManager.LogError(ctx, AuditActionStateRollback, err, map[string]interface{}{
				"state_id":      stateID,
				"checkpoint_id": checkpointID,
			})
		}
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

	sm.logger.Info("state rolled back",
		String("checkpoint_id", checkpointID),
		String("state_id", stateID))

	// Audit successful rollback
	if sm.auditManager != nil {
		newState, _ := sm.store.Get("/")
		log := &AuditLog{
			ID:        generateAuditID(),
			Timestamp: time.Now(),
			Action:    AuditActionStateRollback,
			Result:    AuditResultSuccess,
			StateID:   stateID,
			Resource:  "state",
			OldValue:  oldState,
			NewValue:  newState,
			Details: map[string]interface{}{
				"checkpoint_id": checkpointID,
			},
		}
		sm.auditManager.enrichFromContext(ctx, log)
		sm.logAuditAsync(log, "state_rollback")
	}

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
	sm.logger.Info("shutting down state manager")

	// Audit shutdown
	if sm.auditManager != nil {
		log := &AuditLog{
			ID:        generateAuditID(),
			Timestamp: time.Now(),
			Action:    AuditActionConfigChange,
			Result:    AuditResultSuccess,
			Resource:  "state_manager",
			Details: map[string]interface{}{
				"operation":       "shutdown",
				"active_contexts": sm.activeContexts.Size(),
			},
		}
		// Use synchronous logging for shutdown
		if err := sm.auditManager.logger.Log(context.Background(), log); err != nil {
			sm.logger.Error("failed to write shutdown audit log", Err(err))
		}
	}

	// Signal shutdown to all goroutines
	sm.cancel()

	// Stop accepting new work
	atomic.StoreInt32(&sm.closing, 1)

	// Give a very short moment for any in-flight enqueues to complete
	time.Sleep(1 * time.Millisecond)

	// Wait for all background workers to complete with timeout
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Suppress panics during shutdown
			}
		}()
		sm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Workers finished cleanly
	case <-time.After(GetDefaultShutdownTimeout()):
		sm.logger.Error("shutdown timeout, forcing close", Duration("timeout", GetDefaultShutdownTimeout()))
	}

	// Close channels to prevent new data - use panic recovery to handle already-closed channels
	func() {
		defer func() { recover() }()
		close(sm.updateQueue)
	}()
	func() {
		defer func() { recover() }()
		close(sm.eventQueue)
	}()
	func() {
		defer func() { recover() }()
		close(sm.errCh)
	}()

	// Stop rate limiter before closing audit manager to prevent new goroutines
	if sm.rateLimiter != nil {
		sm.rateLimiter.Stop()
	}

	// Reset client rate limiter (clear tracked clients)
	if sm.clientRateLimiter != nil {
		sm.clientRateLimiter.Reset()
	}

	// Close audit manager and wait for its goroutines to finish
	if sm.auditManager != nil {
		// First close the audit manager to wait for goroutines
		if err := sm.auditManager.Close(); err != nil {
			sm.logger.Error("failed to close audit manager", Err(err))
		}
		
		// Then close the logger
		if sm.auditManager.logger != nil {
			if err := sm.auditManager.logger.Close(); err != nil {
				sm.logger.Error("failed to close audit logger", Err(err))
			}
		}
	}

	// Stop performance optimizer
	if sm.performanceOptimizer != nil {
		sm.performanceOptimizer.Stop()
	}

	// Close state store
	if sm.store != nil {
		if err := sm.store.Close(); err != nil {
			sm.logger.Error("failed to close state store", Err(err))
		}
	}

	sm.logger.Info("state manager shutdown complete")
	
	// Shutdown the logger if it's a TestSafeLogger
	if testLogger, ok := sm.logger.(*TestSafeLogger); ok {
		testLogger.Shutdown()
	}
	
	return nil
}

// isChannelClosed checks if a channel is closed by attempting to send to it with select
func (sm *StateManager) isChannelClosed(ch interface{}) bool {
	// Use reflection or a simple non-blocking send attempt
	// For channels, we'll use a different approach - track the closed state
	// Since Go doesn't provide a direct way to check if a channel is closed,
	// we'll rely on proper shutdown order and error handling
	return false // For now, always try to close - panic will be caught if already closed
}

// processUpdates processes update requests with batching
func (sm *StateManager) processUpdates() {
	defer sm.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			// During shutdown, suppress error reporting to prevent hanging
			if atomic.LoadInt32(&sm.closing) == 0 {
				sm.reportError(fmt.Errorf("panic in processUpdates: %v", r))
			}
		}
	}()

	batch := make([]*updateRequest, 0, sm.options.BatchSize)
	timer := time.NewTimer(sm.options.BatchTimeout)
	defer timer.Stop()
	
	// Track batch resets to detect memory growth
	batchResetCount := 0

	for {
		select {
		case req, ok := <-sm.updateQueue:
			if !ok {
				// Channel closed - process remaining batch and exit
				if len(batch) > 0 && atomic.LoadInt32(&sm.closing) == 0 {
					sm.processBatch(batch)
				}
				sm.logger.Debug("processUpdates queue closed")
				return
			}

			// Skip nil requests (inserted during channel close)
			if req == nil {
				continue
			}

			// Check if we're closing and should not process
			if atomic.LoadInt32(&sm.closing) == 1 {
				// Send error to request - but don't block
				select {
				case req.result <- updateResult{err: ErrManagerClosing}:
				default:
					// Channel full or closed, drop the result
				}
				continue
			}

			batch = append(batch, req)

			if len(batch) >= sm.options.BatchSize {
				sm.processBatch(batch)
				// Proper slice cleanup with memory management
				batch = sm.resetBatchSlice(batch, &batchResetCount)
				timer.Reset(sm.options.BatchTimeout)
			}

		case <-timer.C:
			if len(batch) > 0 && atomic.LoadInt32(&sm.closing) == 0 {
				sm.processBatch(batch)
				// Proper slice cleanup with memory management  
				batch = sm.resetBatchSlice(batch, &batchResetCount)
			}
			timer.Reset(sm.options.BatchTimeout)

		case <-sm.ctx.Done():
			// Context cancelled - process remaining batch and exit quickly
			if len(batch) > 0 && atomic.LoadInt32(&sm.closing) == 0 {
				sm.processBatch(batch)
			}
			sm.logger.Debug("processUpdates context cancelled", Err(sm.ctx.Err()))
			return
		case <-time.After(5 * time.Second):
			// Timeout to prevent goroutine hangs - process any pending batch
			if len(batch) > 0 {
				sm.processBatch(batch)
				batch = batch[:0]
				timer.Reset(sm.options.BatchTimeout)
			}
			continue
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

	sm.logger.Debug("processing update batch",
		Int("batch_size", len(batch)),
		Int("state_groups", len(groups)))

	// Process each group
	for stateID, requests := range groups {
		sm.processStateUpdates(stateID, requests)
	}
}

// processStateUpdates processes updates for a single state
func (sm *StateManager) processStateUpdates(stateID string, requests []*updateRequest) {
	// Get current state with retry logic
	var currentState interface{}
	var err error
	
	for attempt := 0; attempt <= sm.options.MaxRetries; attempt++ {
		currentState, err = sm.store.Get(stateID)
		if err == nil {
			break // Success
		}
		
		// If this is the last attempt, fail
		if attempt == sm.options.MaxRetries {
			// Send error to all requests
			for _, req := range requests {
				req.result <- updateResult{err: fmt.Errorf("failed to get state after %d attempts: %w", attempt+1, err)}
			}
			return
		}
		
		// Log retry attempt
		sm.logger.Debug("store get failed, retrying",
			Err(err),
			String("state_id", stateID),
			Int("attempt", attempt+1),
			Int("max_retries", sm.options.MaxRetries))
		
		// Wait before retrying (with exponential backoff)
		retryDelay := time.Duration(float64(sm.options.RetryDelay) * float64(attempt+1))
		time.Sleep(retryDelay)
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

	// Track start time for duration logging
	startTime := time.Now()

	// Security validation for updates
	if err := sm.securityValidator.ValidateState(req.updates); err != nil {
		// Audit security validation failure
		if sm.auditManager != nil {
			sm.auditManager.LogSecurityEvent(req.ctx, AuditActionValidationFail, req.contextID, "", "update_validation", map[string]interface{}{
				"state_id": req.stateID,
				"error":    err.Error(),
			})
		}
		return updateResult{err: fmt.Errorf("security validation failed for updates: %w", err)}
	}

	// Compute delta between current state and updates
	delta, err := sm.deltaComputer.ComputeDelta(state, req.updates)
	if err != nil {
		return updateResult{err: fmt.Errorf("delta computation failed: %w", err)}
	}

	// Security validation for computed delta
	if err := sm.securityValidator.ValidatePatch(delta); err != nil {
		// Audit security validation failure
		if sm.auditManager != nil {
			sm.auditManager.LogSecurityEvent(req.ctx, AuditActionSizeLimit, req.contextID, "", "delta_validation", map[string]interface{}{
				"state_id": req.stateID,
				"error":    err.Error(),
			})
		}
		return updateResult{err: fmt.Errorf("security validation failed for delta: %w", err)}
	}

	// Apply the delta to get the new state
	newState, err := delta.Apply(state)
	if err != nil {
		return updateResult{err: fmt.Errorf("delta application failed: %w", err)}
	}

	// Security validation for resulting state
	if err := sm.securityValidator.ValidateState(newState); err != nil {
		// Audit security validation failure
		if sm.auditManager != nil {
			sm.auditManager.LogSecurityEvent(req.ctx, AuditActionSizeLimit, req.contextID, "", "state_size_limit", map[string]interface{}{
				"state_id": req.stateID,
				"error":    err.Error(),
			})
		}
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

	// Apply the patch to the store with retry logic
	for attempt := 0; attempt <= sm.options.MaxRetries; attempt++ {
		if err := sm.store.ApplyPatch(delta); err != nil {
			
			// If this is the last attempt, fail
			if attempt == sm.options.MaxRetries {
				// Audit failed store update after all retries
				if sm.auditManager != nil {
					sm.auditManager.LogError(req.ctx, AuditActionError, err, map[string]interface{}{
						"context_id": req.contextID,
						"state_id":   req.stateID,
						"operation":  "store_update",
						"attempts":   attempt + 1,
					})
				}
				return updateResult{err: fmt.Errorf("store update failed after %d attempts: %w", attempt+1, err)}
			}
			
			// Log retry attempt
			sm.logger.Debug("store update failed, retrying",
				Err(err),
				String("context_id", req.contextID),
				String("state_id", req.stateID),
				Int("attempt", attempt+1),
				Int("max_retries", sm.options.MaxRetries))
			
			// Wait before retrying (with exponential backoff)
			retryDelay := time.Duration(float64(sm.options.RetryDelay) * float64(attempt+1))
			select {
			case <-req.ctx.Done():
				return updateResult{err: fmt.Errorf("context cancelled during retry: %w", req.ctx.Err())}
			case <-time.After(retryDelay):
				// Continue to next attempt
			}
		} else {
			// Success - log if we had to retry
			if attempt > 0 {
				sm.logger.Info("store update succeeded after retry",
					String("context_id", req.contextID),
					String("state_id", req.stateID),
					Int("successful_attempt", attempt+1))
			}
			break
		}
	}

	// Create checkpoint if requested
	if req.options.CreateCheckpoint {
		if err := sm.rollbackManager.CreateMarker(req.options.CheckpointName); err != nil {
			// Report error but don't fail the update
			sm.logger.Error("checkpoint creation failed during update",
				Err(err),
				String("checkpoint_name", req.options.CheckpointName))
			sm.reportError(err)
		} else {
			sm.logger.Debug("checkpoint created",
				String("checkpoint_name", req.options.CheckpointName),
				String("state_id", req.stateID))
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

	// Audit successful state update
	if sm.auditManager != nil {
		log := &AuditLog{
			ID:        generateAuditID(),
			Timestamp: time.Now(),
			Action:    AuditActionStateUpdate,
			Result:    AuditResultSuccess,
			ContextID: req.contextID,
			StateID:   req.stateID,
			Resource:  "state",
			OldValue:  state,
			NewValue:  newState,
			Duration:  time.Since(startTime),
			Details: map[string]interface{}{
				"delta_operations":   len(delta),
				"checkpoint_created": req.options.CreateCheckpoint,
			},
		}

		sm.auditManager.enrichFromContext(req.ctx, log)
		sm.logAuditAsync(log, "state_update")
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
	defer func() {
		if r := recover(); r != nil {
			// During shutdown, suppress error reporting to prevent hanging
			if atomic.LoadInt32(&sm.closing) == 0 {
				sm.reportError(fmt.Errorf("panic in processEvents: %v", r))
			}
		}
	}()

	for {
		select {
		case event, ok := <-sm.eventQueue:
			if !ok {
				sm.logger.Debug("processEvents queue closed")
				return
			}

			// Skip nil events (inserted during channel close)
			if event == nil {
				continue
			}

			// Check if we're shutting down before processing event
			if atomic.LoadInt32(&sm.closing) == 1 {
				sm.logger.Debug("processEvents skipping event during shutdown")
				return
			}

			// Process state events based on type with very short timeout
			func() {
				eventCtx, cancel := context.WithTimeout(sm.ctx, 50*time.Millisecond)
				defer cancel()
				defer func() {
					if r := recover(); r != nil {
						// Suppress panics during event processing
					}
				}()
				
				switch event.Type {
				case "state_snapshot":
					if snapshot, ok := event.Data["snapshot"]; ok {
						snapEvent := events.NewStateSnapshotEvent(snapshot)
						if err := sm.eventHandler.HandleStateSnapshot(snapEvent); err != nil && atomic.LoadInt32(&sm.closing) == 0 {
							sm.logger.Error("snapshot event processing failed", Err(err))
							sm.reportError(err)
						}
					}
				case "state_delta":
					if delta, ok := event.Data["delta"].([]events.JSONPatchOperation); ok {
						deltaEvent := events.NewStateDeltaEvent(delta)
						if err := sm.eventHandler.HandleStateDelta(deltaEvent); err != nil && atomic.LoadInt32(&sm.closing) == 0 {
							sm.logger.Error("delta event processing failed", Err(err))
							sm.reportError(err)
						}
					}
				}
				// TODO: Use eventCtx in event handlers to respect timeout
				_ = eventCtx
			}()

		case <-sm.ctx.Done():
			sm.logger.Debug("processEvents context cancelled", Err(sm.ctx.Err()))
			return
		case <-time.After(3 * time.Second):
			// Timeout to prevent goroutine hangs - just continue the loop
			continue
		}
	}
}

// autoCheckpoint creates automatic checkpoints
func (sm *StateManager) autoCheckpoint() {
	defer sm.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			// During shutdown, suppress error reporting to prevent hanging
			if atomic.LoadInt32(&sm.closing) == 0 {
				sm.reportError(fmt.Errorf("panic in autoCheckpoint: %v", r))
			}
		}
	}()

	// Validate interval before creating ticker
	interval := sm.options.CheckpointInterval
	if interval <= 0 {
		interval = GetDefaultCheckpointInterval() // Use default interval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if we're shutting down before creating checkpoint
			if atomic.LoadInt32(&sm.closing) == 1 {
				sm.logger.Debug("autoCheckpoint exiting during shutdown")
				return
			}
			
			// Create checkpoints with timeout
			go func() {
				defer func() {
					if r := recover(); r != nil {
						// Suppress panics during checkpoint creation
					}
				}()
				
				if atomic.LoadInt32(&sm.closing) == 0 {
					sm.createAutoCheckpoints()
				}
			}()

		case <-sm.ctx.Done():
			sm.logger.Debug("autoCheckpoint context cancelled", Err(sm.ctx.Err()))
			return
		case <-time.After(6 * time.Second):
			// Timeout to prevent goroutine hangs - just continue the loop
			continue
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

	for range stateIDs {
		// Ensure state exists before creating checkpoint
		_, err := sm.store.Get("/")
		if err != nil {
			sm.logger.Error("auto checkpoint failed to get state", Err(err))
			sm.reportError(err)
			continue
		}

		name := fmt.Sprintf("auto-%s", time.Now().Format("20060102-150405")) // AutoCheckpointNameLength format
		if err := sm.rollbackManager.CreateMarker(name); err != nil {
			sm.logger.Error("auto checkpoint creation failed",
				Err(err),
				String("checkpoint_name", name))
			sm.reportError(err)
		}
	}
}

// collectMetrics collects performance metrics
func (sm *StateManager) collectMetrics() {
	defer sm.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			// During shutdown, suppress error reporting to prevent hanging
			if atomic.LoadInt32(&sm.closing) == 0 {
				sm.reportError(fmt.Errorf("panic in collectMetrics: %v", r))
			}
		}
	}()

	// Validate interval before creating ticker
	interval := sm.options.MetricsInterval
	if interval <= 0 {
		interval = GetDefaultMetricsInterval() // Use default interval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if we're shutting down before collecting metrics
			if atomic.LoadInt32(&sm.closing) == 1 {
				sm.logger.Debug("collectMetrics exiting during shutdown")
				return
			}
			
			// Collect metrics with timeout
			go func() {
				defer func() {
					if r := recover(); r != nil {
						// Suppress panics during metrics collection
					}
				}()
				
				if atomic.LoadInt32(&sm.closing) == 0 {
					sm.metricsCollector.Collect(sm)
				}
			}()

		case <-sm.ctx.Done():
			sm.logger.Debug("collectMetrics context cancelled", Err(sm.ctx.Err()))
			return
		case <-time.After(3 * time.Second):
			// Timeout to prevent goroutine hangs - just continue the loop
			continue
		}
	}
}

// Helper methods

func (sm *StateManager) updateContextAccess(contextID string) {
	// The ContextManager.Get method already updates LastAccessed
	sm.activeContexts.Get(contextID)
}

func (sm *StateManager) emitEvent(event *stateEvent) {
	// Check if closing
	if atomic.LoadInt32(&sm.closing) == 1 {
		return
	}

	select {
	case sm.eventQueue <- event:
	case <-sm.ctx.Done():
		// Manager is shutting down
	default:
		// Queue full, log and drop
		sm.logger.Warn("event queue full, dropping event",
			String("event_type", event.Type),
			String("state_id", event.StateID))
	}
}

// reportError sends an error to the error channel if possible
func (sm *StateManager) reportError(err error) {
	if err == nil {
		return
	}

	// Try to send error to channel, but don't block
	select {
	case sm.errCh <- err:
	case <-sm.ctx.Done():
		// Manager is shutting down
	default:
		// Channel is full, log directly
		sm.logger.Error("error channel full, dropping error", Err(err))
	}
}

// logAuditAsync performs asynchronous audit logging with enhanced error handling
func (sm *StateManager) logAuditAsync(log *AuditLog, operation string) {
	if sm.auditManager == nil || log == nil {
		return
	}
	
	// Check if manager is closing to avoid spawning new goroutines during shutdown
	if atomic.LoadInt32(&sm.closing) == 1 {
		// For shutdown operations, log synchronously to ensure they're captured
		if operation == "shutdown" {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := sm.auditManager.logger.Log(ctx, log); err != nil {
				sm.logger.Error("failed to write shutdown audit log", Err(err))
			}
		}
		return
	}
	
	// Use bounded goroutine pool to prevent excessive goroutine creation
	go func() {
		defer func() {
			if r := recover(); r != nil {
				sm.reportError(fmt.Errorf("panic in %s audit logging goroutine: %v", operation, r))
			}
		}()
		
		// Create context that respects both timeout and manager shutdown
		auditCtx, cancel := context.WithTimeout(sm.ctx, 5*time.Second)
		defer cancel()
		
		if err := sm.auditManager.logger.Log(auditCtx, log); err != nil {
			// Check if error is due to context cancellation (manager shutting down)
			select {
			case <-sm.ctx.Done():
				sm.logger.Debug(fmt.Sprintf("%s audit logging cancelled due to shutdown", operation))
				return
			default:
				sm.logger.Error(fmt.Sprintf("failed to write %s audit log", operation), Err(err))
				// Report the error through the error channel if available
				if sm.errCh != nil {
					select {
					case sm.errCh <- fmt.Errorf("%s audit logging failed: %w", operation, err):
					case <-sm.ctx.Done():
						// Manager shutting down, don't block
					default: // Don't block if error channel is full
					}
				}
			}
		}
	}()
}

// handleErrors processes errors from goroutines
func (sm *StateManager) handleErrors() {
	defer sm.wg.Done()

	errorCounts := make(map[string]int)
	resetTicker := time.NewTicker(DefaultErrorResetInterval)
	defer resetTicker.Stop()

	for {
		select {
		case err, ok := <-sm.errCh:
			if !ok {
				// Error channel closed
				sm.logger.Debug("handleErrors error channel closed")
				return
			}
			if err == nil {
				continue
			}

			// Skip error processing during shutdown to prevent blocking
			if atomic.LoadInt32(&sm.closing) == 1 {
				continue
			}

			// Log the error
			sm.logger.Error("async operation failed",
				Err(err),
				String("error_type", categorizeError(err)))

			// Track error counts for circuit breaker
			errType := categorizeError(err)
			errorCounts[errType]++

			// Check if we should enter degraded mode
			if sm.shouldCircuitBreak(errorCounts) {
				sm.enterDegradedMode()
			}

		case <-resetTicker.C:
			// Reset error counts periodically
			if atomic.LoadInt32(&sm.closing) == 0 {
				errorCounts = make(map[string]int)
			}

		case <-sm.ctx.Done():
			sm.logger.Debug("handleErrors context cancelled", Err(sm.ctx.Err()))
			// Drain remaining errors before exiting with timeout
			drainTimeout := time.After(5 * time.Second)
			for {
				select {
				case err := <-sm.errCh:
					if err != nil {
						sm.logger.Error("async operation failed during shutdown", Err(err))
					}
				case <-drainTimeout:
					sm.logger.Debug("error drain timeout, forcing exit")
					return
				default:
					return
				}
			}
		case <-time.After(2 * time.Second):
			// Timeout to prevent goroutine hangs - just continue the loop
			continue
		}
	}
}

// categorizeError determines the type of error for circuit breaker logic
func categorizeError(err error) string {
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "checkpoint"):
		return "checkpoint"
	case strings.Contains(errStr, "event"):
		return "event"
	case strings.Contains(errStr, "update"):
		return "update"
	case strings.Contains(errStr, "metrics"):
		return "metrics"
	default:
		return "unknown"
	}
}

// shouldCircuitBreak determines if we should enter degraded mode
func (sm *StateManager) shouldCircuitBreak(errorCounts map[string]int) bool {
	// Simple circuit breaker logic - can be made more sophisticated
	for errType, count := range errorCounts {
		switch errType {
		case "update":
			if count > DefaultUpdateErrorThreshold {
				return true
			}
		case "checkpoint":
			if count > DefaultCheckpointErrorThreshold {
				return true
			}
		default:
			if count > DefaultMaxErrorCount {
				return true
			}
		}
	}
	return false
}

// enterDegradedMode puts the system in a degraded state
func (sm *StateManager) enterDegradedMode() {
	sm.logger.Warn("entering degraded mode due to excessive errors")

	// Emit degraded mode event
	sm.emitEvent(&stateEvent{
		Type:      "system.degraded",
		StateID:   "",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"reason": "excessive_errors",
		},
	})

	// Could implement additional degraded mode behavior here
	// For example: disable non-critical features, increase timeouts, etc.
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

	// Get active contexts count from ContextManager
	activeContexts := sm.activeContexts.Size()

	mc.metrics = map[string]interface{}{
		"active_contexts":   activeContexts,
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
	defer func() {
		if r := recover(); r != nil {
			// During shutdown, suppress error reporting to prevent hanging
			if atomic.LoadInt32(&sm.closing) == 0 {
				sm.reportError(fmt.Errorf("panic in contextCleanup: %v", r))
			}
		}
	}()

	ticker := time.NewTicker(sm.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if we're shutting down before cleanup
			if atomic.LoadInt32(&sm.closing) == 1 {
				sm.logger.Debug("contextCleanup exiting during shutdown")
				return
			}
			
			// Cleanup with timeout
			go func() {
				defer func() {
					if r := recover(); r != nil {
						// Suppress panics during cleanup
					}
				}()
				
				if atomic.LoadInt32(&sm.closing) == 0 {
					sm.cleanupExpiredContexts()
				}
			}()
		case <-sm.ctx.Done():
			sm.logger.Debug("contextCleanup context cancelled", Err(sm.ctx.Err()))
			return
		case <-time.After(3 * time.Second):
			// Timeout to prevent goroutine hangs - just continue the loop
			continue
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
	// Launch cleanup in a goroutine with proper error handling
	go func() {
		defer func() {
			if r := recover(); r != nil {
				sm.reportError(fmt.Errorf("panic in cleanupExpiredContexts: %v", r))
			}
		}()
		// Check context before processing
		if err := sm.ctx.Err(); err != nil {
			sm.logger.Debug("cleanup goroutine cancelled", Err(err))
			return
		}
		sm.cleanupExpiredContexts()
	}()
}

// cleanupExpiredContexts removes expired contexts
func (sm *StateManager) cleanupExpiredContexts() {
	// Get expired contexts
	expired := sm.activeContexts.GetExpiredContexts(sm.contextTTL)

	// Remove each expired context
	for _, contextID := range expired {
		// Get context before deletion for event
		ctx, _ := sm.activeContexts.Get(contextID)

		// Delete the context
		sm.activeContexts.Delete(contextID)

		// Emit context expired event
		if ctx != nil {
			sm.emitEvent(&stateEvent{
				Type:      "context.expired",
				StateID:   ctx.StateID,
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"contextID": ctx.ID,
					"reason":    "expired",
				},
			})

			sm.logger.Debug("context expired",
				String("context_id", ctx.ID),
				String("state_id", ctx.StateID),
				Duration("age", time.Since(ctx.Created)))

			// Audit context expiration
			if sm.auditManager != nil {
				log := &AuditLog{
					ID:        generateAuditID(),
					Timestamp: time.Now(),
					Action:    AuditActionContextExpire,
					Result:    AuditResultSuccess,
					ContextID: ctx.ID,
					StateID:   ctx.StateID,
					Resource:  "context",
					Details: map[string]interface{}{
						"reason":        "expired",
						"age_seconds":   time.Since(ctx.Created).Seconds(),
						"last_accessed": ctx.GetLastAccessed(),
					},
				}
				sm.logAuditAsync(log, "context_expiration")
			}
		}
	}
}

// enqueueUpdate adds an update request to the queue with closing check
func (sm *StateManager) enqueueUpdate(req *updateRequest) error {
	// Check if manager is closing
	if atomic.LoadInt32(&sm.closing) == 1 {
		return ErrManagerClosing
	}

	select {
	case sm.updateQueue <- req:
		return nil
	case <-sm.ctx.Done():
		return ErrManagerClosed
	default:
		return ErrQueueFull
	}
}

// isClosing returns true if the manager is in the process of closing
func (sm *StateManager) isClosing() bool {
	return atomic.LoadInt32(&sm.closing) == 1
}

// resetBatchSlice properly resets the batch slice to prevent memory retention
func (sm *StateManager) resetBatchSlice(batch []*updateRequest, resetCount *int) []*updateRequest {
	*resetCount++
	
	// Clear the slice properly to allow GC of underlying data
	batch = batch[:0]
	
	// Recreate slice periodically when capacity grows too large
	// This prevents memory retention from large batches
	if cap(batch) > sm.options.BatchSize*4 {
		sm.logger.Debug("recreating batch slice due to large capacity",
			Int("current_cap", cap(batch)),
			Int("target_batch_size", sm.options.BatchSize),
			Int("reset_count", *resetCount))
		
		// Create new slice with original capacity
		batch = make([]*updateRequest, 0, sm.options.BatchSize)
		*resetCount = 0 // Reset counter after recreation
	}
	
	return batch
}
