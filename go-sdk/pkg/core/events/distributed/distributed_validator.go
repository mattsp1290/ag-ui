package distributed

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// NodeID represents a unique identifier for a validation node
type NodeID string

// NodeState represents the state of a validation node
type NodeState int

const (
	// NodeStateActive indicates the node is active and participating in validation
	NodeStateActive NodeState = iota
	// NodeStatePartitioned indicates the node is partitioned from the cluster
	NodeStatePartitioned
	// NodeStateFailed indicates the node has failed
	NodeStateFailed
	// NodeStateSyncing indicates the node is synchronizing state
	NodeStateSyncing
)

// ValidationDecision represents a validation decision from a node
type ValidationDecision struct {
	NodeID    NodeID                       `json:"node_id"`
	EventID   string                       `json:"event_id"`
	EventType events.EventType             `json:"event_type"`
	IsValid   bool                         `json:"is_valid"`
	Errors    []*ValidationError    `json:"errors,omitempty"`
	Warnings  []*ValidationError    `json:"warnings,omitempty"`
	Timestamp time.Time                    `json:"timestamp"`
	Sequence  uint64                       `json:"sequence"`
}

// NodeInfo represents information about a validation node
type NodeInfo struct {
	ID              NodeID        `json:"id"`
	Address         string        `json:"address"`
	State           NodeState     `json:"state"`
	LastHeartbeat   time.Time     `json:"last_heartbeat"`
	ValidationCount uint64        `json:"validation_count"`
	ErrorRate       float64       `json:"error_rate"`
	ResponseTimeMs  float64       `json:"response_time_ms"`
	Load            float64       `json:"load"`
}

// GoroutineRestartPolicy defines the restart policy for goroutines
type GoroutineRestartPolicy struct {
	// MaxRestarts is the maximum number of restarts allowed
	MaxRestarts int
	
	// RestartWindow is the time window for restart counting
	RestartWindow time.Duration
	
	// BaseBackoff is the base backoff duration
	BaseBackoff time.Duration
	
	// MaxBackoff is the maximum backoff duration
	MaxBackoff time.Duration
	
	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier float64
}

// DefaultGoroutineRestartPolicy returns a default restart policy
func DefaultGoroutineRestartPolicy() *GoroutineRestartPolicy {
	return &GoroutineRestartPolicy{
		MaxRestarts:       10,
		RestartWindow:     5 * time.Minute,
		BaseBackoff:       100 * time.Millisecond,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// GoroutineManager manages goroutine lifecycle with restart capabilities
type GoroutineManager struct {
	name               string
	restartPolicy      *GoroutineRestartPolicy
	restartCount       int64
	lastRestartTime    time.Time
	isRunning          bool
	shouldRestart      bool
	ctx                context.Context
	cancel             context.CancelFunc
	mu                 sync.RWMutex
	wg                 sync.WaitGroup
}

// NewGoroutineManager creates a new goroutine manager
func NewGoroutineManager(name string, policy *GoroutineRestartPolicy) *GoroutineManager {
	if policy == nil {
		policy = DefaultGoroutineRestartPolicy()
	}
	
	return &GoroutineManager{
		name:          name,
		restartPolicy: policy,
		shouldRestart: true,
	}
}

// Start starts the managed goroutine
func (gm *GoroutineManager) Start(parentCtx context.Context, fn func(context.Context)) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	
	if gm.isRunning {
		return
	}
	
	gm.ctx, gm.cancel = context.WithCancel(parentCtx)
	gm.isRunning = true
	gm.shouldRestart = true
	
	gm.wg.Add(1)
	go gm.runWithRestart(fn)
}

// Stop stops the managed goroutine
func (gm *GoroutineManager) Stop() {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	
	gm.shouldRestart = false
	if gm.cancel != nil {
		gm.cancel()
	}
	
	gm.wg.Wait()
	gm.isRunning = false
}

// runWithRestart runs the function with restart capability
func (gm *GoroutineManager) runWithRestart(fn func(context.Context)) {
	defer gm.wg.Done()
	
	for {
		select {
		case <-gm.ctx.Done():
			return
		default:
		}
		
		gm.mu.RLock()
		shouldRestart := gm.shouldRestart
		gm.mu.RUnlock()
		
		if !shouldRestart {
			return
		}
		
		// Run the function with panic recovery
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Panic in goroutine %s: %v", gm.name, r)
					gm.handleRestart()
				}
			}()
			
			fn(gm.ctx)
		}()
		
		// If we reach here, the function exited normally
		gm.mu.RLock()
		shouldRestart = gm.shouldRestart
		gm.mu.RUnlock()
		
		if !shouldRestart {
			return
		}
		
		// Function exited, attempt restart
		gm.handleRestart()
	}
}

// handleRestart handles the restart logic with exponential backoff
func (gm *GoroutineManager) handleRestart() {
	now := time.Now()
	
	// Check if we're within the restart window
	if now.Sub(gm.lastRestartTime) > gm.restartPolicy.RestartWindow {
		// Reset restart count if outside window
		atomic.StoreInt64(&gm.restartCount, 0)
	}
	
	restarts := atomic.AddInt64(&gm.restartCount, 1)
	gm.lastRestartTime = now
	
	// Check if we've exceeded max restarts
	if int(restarts) > gm.restartPolicy.MaxRestarts {
		log.Printf("Goroutine %s exceeded max restarts (%d), stopping", gm.name, gm.restartPolicy.MaxRestarts)
		gm.mu.Lock()
		gm.shouldRestart = false
		gm.mu.Unlock()
		return
	}
	
	// Calculate backoff duration
	backoffDuration := time.Duration(float64(gm.restartPolicy.BaseBackoff) * 
		math.Pow(gm.restartPolicy.BackoffMultiplier, float64(restarts-1)))
	
	if backoffDuration > gm.restartPolicy.MaxBackoff {
		backoffDuration = gm.restartPolicy.MaxBackoff
	}
	
	log.Printf("Restarting goroutine %s in %v (attempt %d/%d)", 
		gm.name, backoffDuration, restarts, gm.restartPolicy.MaxRestarts)
	
	// Wait for backoff period
	select {
	case <-gm.ctx.Done():
		return
	case <-time.After(backoffDuration):
		// Continue with restart
	}
}

// GetRestartCount returns the current restart count
func (gm *GoroutineManager) GetRestartCount() int64 {
	return atomic.LoadInt64(&gm.restartCount)
}

// IsRunning returns whether the goroutine is currently running
func (gm *GoroutineManager) IsRunning() bool {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.isRunning
}

// DistributedValidatorConfig contains configuration for the distributed validator
type DistributedValidatorConfig struct {
	// NodeID is the unique identifier for this validator node
	NodeID NodeID

	// ConsensusConfig contains consensus algorithm configuration
	ConsensusConfig *ConsensusConfig

	// StateSync contains state synchronization configuration
	StateSync *StateSyncConfig

	// LoadBalancer contains load balancing configuration
	LoadBalancer *LoadBalancerConfig

	// PartitionHandler contains partition handling configuration
	PartitionHandler *PartitionHandlerConfig

	// MaxNodeFailures is the maximum number of node failures to tolerate
	MaxNodeFailures int

	// ValidationTimeout is the timeout for validation operations
	ValidationTimeout time.Duration

	// HeartbeatInterval is the interval between heartbeats
	HeartbeatInterval time.Duration

	// EnableMetrics enables distributed metrics collection
	EnableMetrics bool
	
	// Circuit Breaker settings
	ConsensusCircuitBreakerConfig   *errors.CircuitBreakerConfig
	StateSyncCircuitBreakerConfig   *errors.CircuitBreakerConfig
	HeartbeatCircuitBreakerConfig   *errors.CircuitBreakerConfig
	
	// GoroutineRestartPolicy defines restart behavior for background goroutines
	GoroutineRestartPolicy *GoroutineRestartPolicy
}

// DefaultDistributedValidatorConfig returns default configuration
func DefaultDistributedValidatorConfig(nodeID NodeID) *DistributedValidatorConfig {
	return &DistributedValidatorConfig{
		NodeID:                 nodeID,
		ConsensusConfig:        DefaultConsensusConfig(),
		StateSync:              DefaultStateSyncConfig(),
		LoadBalancer:           DefaultLoadBalancerConfig(),
		PartitionHandler:       DefaultPartitionHandlerConfig(),
		MaxNodeFailures:        2,
		ValidationTimeout:      5 * time.Second,
		HeartbeatInterval:      1 * time.Second,
		EnableMetrics:          true,
		ConsensusCircuitBreakerConfig:   errors.DefaultCircuitBreakerConfig("consensus"),
		StateSyncCircuitBreakerConfig:   errors.DefaultCircuitBreakerConfig("state-sync"),
		HeartbeatCircuitBreakerConfig:   errors.DefaultCircuitBreakerConfig("heartbeat"),
		GoroutineRestartPolicy: DefaultGoroutineRestartPolicy(),
	}
}

// DistributedValidator implements distributed validation across multiple nodes
type DistributedValidator struct {
	config           *DistributedValidatorConfig
	localValidator   *events.EventValidator
	consensus        *ConsensusManager
	stateSync        *StateSynchronizer
	partitionHandler *PartitionHandler
	loadBalancer     *LoadBalancer
	
	// Node management
	nodes            map[NodeID]*NodeInfo
	nodesMutex       sync.RWMutex
	
	// Validation state
	pendingValidations map[string]*PendingValidation
	validationMutex    sync.RWMutex
	
	// Circuit Breakers
	consensusCircuitBreaker   errors.CircuitBreaker
	stateSyncCircuitBreaker   errors.CircuitBreaker
	heartbeatCircuitBreaker   errors.CircuitBreaker
	
	// Metrics
	metrics          *DistributedMetrics
	
	// Lifecycle
	running          bool
	runningMutex     sync.RWMutex
	stopChan         chan struct{}
	stopOnce         sync.Once
	
	// Goroutine managers for background routines
	heartbeatManager   *GoroutineManager
	cleanupManager     *GoroutineManager
	metricsManager     *GoroutineManager
	consensusManager   *GoroutineManager
	
	// Resource cleanup
	resourceCleanup    []func() error
	cleanupMutex       sync.Mutex
	
	// Tracing
	tracer           trace.Tracer
}

// PendingValidation represents a validation in progress
type PendingValidation struct {
	Event            events.Event
	Context          *events.ValidationContext
	Decisions        map[NodeID]*ValidationDecision
	DecisionsMutex   sync.RWMutex
	StartTime        time.Time
	CompleteChan     chan *ValidationResult
}

// NewDistributedValidator creates a new distributed validator
func NewDistributedValidator(config *DistributedValidatorConfig, localValidator *events.EventValidator) (*DistributedValidator, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if localValidator == nil {
		return nil, fmt.Errorf("localValidator cannot be nil")
	}

	dv := &DistributedValidator{
		config:             config,
		localValidator:     localValidator,
		nodes:              make(map[NodeID]*NodeInfo),
		pendingValidations: make(map[string]*PendingValidation),
		metrics:            NewDistributedMetrics(),
		stopChan:           make(chan struct{}),
		resourceCleanup:    make([]func() error, 0),
		tracer:             otel.Tracer("ag-ui/distributed-validation"),
		consensusCircuitBreaker: errors.GetCircuitBreaker("consensus", config.ConsensusCircuitBreakerConfig),
		stateSyncCircuitBreaker: errors.GetCircuitBreaker("state-sync", config.StateSyncCircuitBreakerConfig),
		heartbeatCircuitBreaker: errors.GetCircuitBreaker("heartbeat", config.HeartbeatCircuitBreakerConfig),
	}
	
	// Initialize goroutine managers
	dv.heartbeatManager = NewGoroutineManager("heartbeat", config.GoroutineRestartPolicy)
	dv.cleanupManager = NewGoroutineManager("cleanup", config.GoroutineRestartPolicy)
	dv.metricsManager = NewGoroutineManager("metrics", config.GoroutineRestartPolicy)
	dv.consensusManager = NewGoroutineManager("consensus", config.GoroutineRestartPolicy)

	// Initialize components
	var err error
	
	// Initialize consensus manager
	dv.consensus, err = NewConsensusManager(config.ConsensusConfig, config.NodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to create consensus manager: %w", err)
	}

	// Initialize state synchronizer
	dv.stateSync, err = NewStateSynchronizer(config.StateSync, config.NodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to create state synchronizer: %w", err)
	}

	// Initialize partition handler
	dv.partitionHandler = NewPartitionHandler(config.PartitionHandler, config.NodeID)

	// Initialize load balancer
	dv.loadBalancer = NewLoadBalancer(config.LoadBalancer)

	return dv, nil
}

// Start starts the distributed validator
func (dv *DistributedValidator) Start(ctx context.Context) error {
	dv.runningMutex.Lock()
	defer dv.runningMutex.Unlock()

	if dv.running {
		return fmt.Errorf("distributed validator already running")
	}

	// Start components
	if err := dv.consensus.Start(ctx); err != nil {
		return fmt.Errorf("failed to start consensus: %w", err)
	}

	if err := dv.stateSync.Start(ctx); err != nil {
		dv.consensus.Stop()
		return fmt.Errorf("failed to start state sync: %w", err)
	}

	if err := dv.partitionHandler.Start(ctx); err != nil {
		dv.consensus.Stop()
		dv.stateSync.Stop()
		return fmt.Errorf("failed to start partition handler: %w", err)
	}

	// Start background routines with managed goroutines
	dv.heartbeatManager.Start(ctx, dv.heartbeatRoutine)
	dv.cleanupManager.Start(ctx, dv.cleanupRoutine)
	dv.metricsManager.Start(ctx, dv.metricsRoutine)
	dv.consensusManager.Start(ctx, dv.consensusRoutine)

	dv.running = true
	return nil
}

// Stop stops the distributed validator
func (dv *DistributedValidator) Stop() error {
	dv.runningMutex.Lock()
	defer dv.runningMutex.Unlock()

	if !dv.running {
		return nil
	}

	// Signal stop to background routines
	dv.stopOnce.Do(func() {
		close(dv.stopChan)
	})

	// Stop managed goroutines
	if dv.heartbeatManager != nil {
		dv.heartbeatManager.Stop()
	}
	if dv.cleanupManager != nil {
		dv.cleanupManager.Stop()
	}
	if dv.metricsManager != nil {
		dv.metricsManager.Stop()
	}
	if dv.consensusManager != nil {
		dv.consensusManager.Stop()
	}

	// Stop components
	var errs []error
	
	if err := dv.partitionHandler.Stop(); err != nil {
		errs = append(errs, fmt.Errorf("failed to stop partition handler: %w", err))
	}

	if err := dv.stateSync.Stop(); err != nil {
		errs = append(errs, fmt.Errorf("failed to stop state sync: %w", err))
	}

	if err := dv.consensus.Stop(); err != nil {
		errs = append(errs, fmt.Errorf("failed to stop consensus: %w", err))
	}
	
	// Execute resource cleanup functions
	dv.cleanupMutex.Lock()
	for _, cleanup := range dv.resourceCleanup {
		if err := cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("resource cleanup error: %w", err))
		}
	}
	dv.cleanupMutex.Unlock()

	dv.running = false

	if len(errs) > 0 {
		return fmt.Errorf("errors stopping distributed validator: %v", errs)
	}

	return nil
}

// ValidateEvent validates an event using distributed consensus
func (dv *DistributedValidator) ValidateEvent(ctx context.Context, event events.Event) *ValidationResult {
	ctx, span := dv.tracer.Start(ctx, "distributed_validation",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	start := time.Now()

	// Setup tracing
	dv.setupEventTracing(span, event)

	// Check for partition and handle if necessary
	if partitionResult := dv.handlePartitionValidation(ctx, span, event, start); partitionResult != nil {
		return partitionResult
	}

	// Prepare for distributed validation
	nodes, pending, eventID := dv.prepareDistributedValidation(ctx, span, event, start)

	// Perform local validation and create decision
	dv.performLocalValidation(ctx, event, eventID, pending)

	// Coordinate distributed validation
	return dv.coordinateDistributedValidation(ctx, span, eventID, event, nodes, pending, start)
}

// setupEventTracing sets up tracing attributes for the event
func (dv *DistributedValidator) setupEventTracing(span trace.Span, event events.Event) {
	if event != nil {
		span.SetAttributes(
			attribute.String("event.type", string(event.Type())),
			attribute.String("node.id", string(dv.config.NodeID)),
		)
	}
}

// handlePartitionValidation handles validation during network partition
func (dv *DistributedValidator) handlePartitionValidation(ctx context.Context, span trace.Span, event events.Event, startTime time.Time) *ValidationResult {
	if !dv.partitionHandler.IsPartitioned() {
		return nil
	}

	span.SetAttributes(attribute.Bool("node.partitioned", true))

	// Handle partition based on configuration
	if dv.config.PartitionHandler.AllowLocalValidation {
		span.AddEvent("validating_locally_due_to_partition")
		localResult := dv.localValidator.ValidateEvent(ctx, event)
		return dv.convertValidationResult(localResult)
	}

	result := &ValidationResult{
		IsValid:   false,
		Errors:    []*ValidationError{{
			RuleID:    "DISTRIBUTED_PARTITION",
			Message:   "Node is partitioned from cluster",
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		}},
		EventCount: 1,
		Duration:   time.Since(startTime),
		Timestamp:  time.Now(),
	}

	span.SetStatus(codes.Error, "node partitioned")
	return result
}

// prepareDistributedValidation prepares the distributed validation setup
func (dv *DistributedValidator) prepareDistributedValidation(ctx context.Context, span trace.Span, event events.Event, startTime time.Time) ([]NodeID, *PendingValidation, string) {
	// Select validation nodes based on load balancing
	nodes := dv.selectValidationNodes(event)
	span.SetAttributes(attribute.Int("validation.nodes", len(nodes)))

	// Create pending validation
	eventID := dv.generateEventID(event)
	pending := &PendingValidation{
		Event:        event,
		Decisions:    make(map[NodeID]*ValidationDecision),
		StartTime:    startTime,
		CompleteChan: make(chan *ValidationResult, 1),
	}

	dv.validationMutex.Lock()
	dv.pendingValidations[eventID] = pending
	dv.validationMutex.Unlock()

	return nodes, pending, eventID
}

// performLocalValidation performs local validation and creates decision
func (dv *DistributedValidator) performLocalValidation(ctx context.Context, event events.Event, eventID string, pending *PendingValidation) *ValidationDecision {
	// Perform local validation
	localResult := dv.localValidator.ValidateEvent(ctx, event)
	localDecision := &ValidationDecision{
		NodeID:    dv.config.NodeID,
		EventID:   eventID,
		EventType: event.Type(),
		IsValid:   localResult.IsValid,
		Errors:    dv.convertValidationErrors(localResult.Errors),
		Warnings:  dv.convertValidationErrors(localResult.Warnings),
		Timestamp: time.Now(),
		Sequence:  dv.getNextSequence(),
	}

	pending.DecisionsMutex.Lock()
	pending.Decisions[dv.config.NodeID] = localDecision
	decisionCount := len(pending.Decisions)
	pending.DecisionsMutex.Unlock()

	// Check if we can reach consensus with current decisions
	dv.checkAndTriggerConsensus(pending, decisionCount)

	return localDecision
}

// checkAndTriggerConsensus checks if consensus can be reached and triggers completion
func (dv *DistributedValidator) checkAndTriggerConsensus(pending *PendingValidation, decisionCount int) {
	// For single-node scenarios, complete immediately
	if dv.config.ConsensusConfig.MinNodes <= 1 {
		result := dv.aggregateDecisions(pending)
		select {
		case pending.CompleteChan <- result:
		default:
			// Channel might already be closed or have a result
		}
		return
	}

	// Check if we have enough decisions for consensus
	requiredNodes := dv.consensus.GetRequiredNodes()
	if decisionCount >= requiredNodes {
		// We have enough decisions, aggregate and complete
		result := dv.aggregateDecisions(pending)
		select {
		case pending.CompleteChan <- result:
		default:
			// Channel might already be closed or have a result
		}
	}
}

// coordinateDistributedValidation coordinates the distributed validation process
func (dv *DistributedValidator) coordinateDistributedValidation(ctx context.Context, span trace.Span, eventID string, event events.Event, nodes []NodeID, pending *PendingValidation, startTime time.Time) *ValidationResult {
	defer func() {
		dv.validationMutex.Lock()
		delete(dv.pendingValidations, eventID)
		dv.validationMutex.Unlock()
	}()

	// Broadcast validation request to other nodes
	dv.broadcastValidationRequest(ctx, eventID, event, nodes)

	// Wait for consensus or timeout
	consensusCtx, consensusCancel := context.WithTimeout(ctx, dv.config.ValidationTimeout)
	defer consensusCancel()

	select {
	case result := <-pending.CompleteChan:
		return dv.handleValidationSuccess(span, result, startTime)
		
	case <-consensusCtx.Done():
		return dv.handleValidationTimeout(span, pending, startTime)
	}
}

// handleValidationSuccess handles successful validation completion
func (dv *DistributedValidator) handleValidationSuccess(span trace.Span, result *ValidationResult, startTime time.Time) *ValidationResult {
	duration := time.Since(startTime)
	result.Duration = duration

	span.SetAttributes(
		attribute.Bool("validation.valid", result.IsValid),
		attribute.Int("validation.errors", len(result.Errors)),
		attribute.Int("validation.warnings", len(result.Warnings)),
		attribute.Int64("validation.duration_ms", duration.Milliseconds()),
	)

	dv.metrics.RecordValidation(duration, result.IsValid)
	return result
}

// handleValidationTimeout handles validation timeout
func (dv *DistributedValidator) handleValidationTimeout(span trace.Span, pending *PendingValidation, startTime time.Time) *ValidationResult {
	// Timeout - use available decisions
	result := dv.aggregateDecisions(pending)
	result.Duration = time.Since(startTime)

	span.SetAttributes(
		attribute.Bool("validation.timeout", true),
		attribute.Bool("validation.valid", result.IsValid),
		attribute.Int("validation.errors", len(result.Errors)),
	)

	dv.metrics.RecordTimeout()
	return result
}

// ValidateSequence validates a sequence of events using distributed consensus
func (dv *DistributedValidator) ValidateSequence(ctx context.Context, events []events.Event) *ValidationResult {
	ctx, span := dv.tracer.Start(ctx, "distributed_sequence_validation",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	span.SetAttributes(
		attribute.Int("sequence.length", len(events)),
		attribute.String("node.id", string(dv.config.NodeID)),
	)

	// For sequence validation, we need to ensure all nodes process events in the same order
	// This requires stronger coordination through the consensus mechanism

	// Acquire distributed lock for sequence validation
	lockID := fmt.Sprintf("sequence-%d", time.Now().UnixNano())
	if err := dv.consensus.AcquireLock(ctx, lockID, dv.config.ValidationTimeout); err != nil {
		span.RecordError(err)
		return &ValidationResult{
			IsValid: false,
			Errors: []*ValidationError{{
				RuleID:    "DISTRIBUTED_LOCK_FAILED",
				Message:   fmt.Sprintf("Failed to acquire distributed lock: %v", err),
				Severity:  ValidationSeverityError,
				Timestamp: time.Now(),
			}},
			EventCount: len(events),
			Timestamp:  time.Now(),
		}
	}
	defer dv.consensus.ReleaseLock(ctx, lockID)

	// Synchronize state across nodes before validation
	if err := dv.stateSync.SyncState(ctx); err != nil {
		span.RecordError(err)
		span.AddEvent("state_sync_failed")
		// Continue with validation but note the sync failure
	}

	// Validate sequence locally first
	localResult := dv.localValidator.ValidateSequence(ctx, events)

	// If local validation fails and we're configured to fail fast, return immediately
	if !localResult.IsValid && dv.consensus.config.RequireUnanimous {
		return dv.convertValidationResult(localResult)
	}

	// TODO: Implement distributed sequence validation protocol
	// For now, return local result
	return dv.convertValidationResult(localResult)
}

// convertValidationResult converts events.ValidationResult to distributed.ValidationResult
func (dv *DistributedValidator) convertValidationResult(eventsResult *events.ValidationResult) *ValidationResult {
	if eventsResult == nil {
		return nil
	}

	result := &ValidationResult{
		IsValid:     eventsResult.IsValid,
		Errors:      make([]*ValidationError, 0, len(eventsResult.Errors)),
		Warnings:    make([]*ValidationError, 0, len(eventsResult.Warnings)),
		Information: make([]*ValidationError, 0, len(eventsResult.Information)),
		EventCount:  eventsResult.EventCount,
		Duration:    eventsResult.Duration,
		Timestamp:   eventsResult.Timestamp,
	}

	// Convert errors
	for _, err := range eventsResult.Errors {
		result.Errors = append(result.Errors, &ValidationError{
			RuleID:      err.RuleID,
			EventID:     err.EventID,
			EventType:   string(err.EventType),
			Message:     err.Message,
			Severity:    ValidationSeverity(err.Severity),
			Context:     err.Context,
			Suggestions: err.Suggestions,
			Timestamp:   err.Timestamp,
		})
	}

	// Convert warnings
	for _, warn := range eventsResult.Warnings {
		result.Warnings = append(result.Warnings, &ValidationError{
			RuleID:      warn.RuleID,
			EventID:     warn.EventID,
			EventType:   string(warn.EventType),
			Message:     warn.Message,
			Severity:    ValidationSeverity(warn.Severity),
			Context:     warn.Context,
			Suggestions: warn.Suggestions,
			Timestamp:   warn.Timestamp,
		})
	}

	// Convert information
	for _, info := range eventsResult.Information {
		result.Information = append(result.Information, &ValidationError{
			RuleID:      info.RuleID,
			EventID:     info.EventID,
			EventType:   string(info.EventType),
			Message:     info.Message,
			Severity:    ValidationSeverity(info.Severity),
			Context:     info.Context,
			Suggestions: info.Suggestions,
			Timestamp:   info.Timestamp,
		})
	}

	return result
}

// convertValidationErrors converts events.ValidationError slice to distributed.ValidationError slice
func (dv *DistributedValidator) convertValidationErrors(eventsErrors []*events.ValidationError) []*ValidationError {
	result := make([]*ValidationError, 0, len(eventsErrors))
	for _, err := range eventsErrors {
		result = append(result, &ValidationError{
			RuleID:      err.RuleID,
			EventID:     err.EventID,
			EventType:   string(err.EventType),
			Message:     err.Message,
			Severity:    ValidationSeverity(err.Severity),
			Context:     err.Context,
			Suggestions: err.Suggestions,
			Timestamp:   err.Timestamp,
		})
	}
	return result
}

// RegisterNode registers a new validation node
func (dv *DistributedValidator) RegisterNode(nodeInfo *NodeInfo) error {
	dv.nodesMutex.Lock()
	defer dv.nodesMutex.Unlock()

	if nodeInfo == nil {
		return fmt.Errorf("nodeInfo cannot be nil")
	}

	dv.nodes[nodeInfo.ID] = nodeInfo
	dv.loadBalancer.UpdateNodeMetrics(nodeInfo.ID, nodeInfo.Load, nodeInfo.ResponseTimeMs)

	return nil
}

// UnregisterNode removes a validation node
func (dv *DistributedValidator) UnregisterNode(nodeID NodeID) error {
	dv.nodesMutex.Lock()
	defer dv.nodesMutex.Unlock()

	delete(dv.nodes, nodeID)
	dv.loadBalancer.RemoveNode(nodeID)

	return nil
}

// GetNodeInfo returns information about a specific node
func (dv *DistributedValidator) GetNodeInfo(nodeID NodeID) (*NodeInfo, bool) {
	dv.nodesMutex.RLock()
	defer dv.nodesMutex.RUnlock()

	info, exists := dv.nodes[nodeID]
	return info, exists
}

// GetAllNodes returns information about all registered nodes
func (dv *DistributedValidator) GetAllNodes() map[NodeID]*NodeInfo {
	dv.nodesMutex.RLock()
	defer dv.nodesMutex.RUnlock()

	// Return a copy to prevent external modification
	nodesCopy := make(map[NodeID]*NodeInfo)
	for k, v := range dv.nodes {
		nodeCopy := *v
		nodesCopy[k] = &nodeCopy
	}

	return nodesCopy
}

// GetMetrics returns distributed validation metrics
func (dv *DistributedValidator) GetMetrics() *DistributedMetrics {
	return dv.metrics
}

// RegisterCleanupFunc registers a function to be called during shutdown
func (dv *DistributedValidator) RegisterCleanupFunc(cleanup func() error) {
	dv.cleanupMutex.Lock()
	defer dv.cleanupMutex.Unlock()
	dv.resourceCleanup = append(dv.resourceCleanup, cleanup)
}

// GetGoroutineStatus returns status of all managed goroutines
func (dv *DistributedValidator) GetGoroutineStatus() map[string]GoroutineStatus {
	status := make(map[string]GoroutineStatus)
	
	if dv.heartbeatManager != nil {
		status["heartbeat"] = GoroutineStatus{
			Name:         "heartbeat",
			IsRunning:    dv.heartbeatManager.IsRunning(),
			RestartCount: dv.heartbeatManager.GetRestartCount(),
		}
	}
	
	if dv.cleanupManager != nil {
		status["cleanup"] = GoroutineStatus{
			Name:         "cleanup",
			IsRunning:    dv.cleanupManager.IsRunning(),
			RestartCount: dv.cleanupManager.GetRestartCount(),
		}
	}
	
	if dv.metricsManager != nil {
		status["metrics"] = GoroutineStatus{
			Name:         "metrics",
			IsRunning:    dv.metricsManager.IsRunning(),
			RestartCount: dv.metricsManager.GetRestartCount(),
		}
	}
	
	if dv.consensusManager != nil {
		status["consensus"] = GoroutineStatus{
			Name:         "consensus",
			IsRunning:    dv.consensusManager.IsRunning(),
			RestartCount: dv.consensusManager.GetRestartCount(),
		}
	}
	
	return status
}

// GoroutineStatus represents the status of a managed goroutine
type GoroutineStatus struct {
	Name         string `json:"name"`
	IsRunning    bool   `json:"is_running"`
	RestartCount int64  `json:"restart_count"`
}

// selectValidationNodes selects nodes for validation based on load balancing
func (dv *DistributedValidator) selectValidationNodes(event events.Event) []NodeID {
	dv.nodesMutex.RLock()
	defer dv.nodesMutex.RUnlock()

	activeNodes := make([]NodeID, 0)
	for nodeID, info := range dv.nodes {
		if info.State == NodeStateActive {
			activeNodes = append(activeNodes, nodeID)
		}
	}

	// Use load balancer to select nodes
	requiredNodes := dv.consensus.GetRequiredNodes()
	return dv.loadBalancer.SelectNodes(activeNodes, requiredNodes)
}

// broadcastValidationRequest broadcasts a validation request to selected nodes
func (dv *DistributedValidator) broadcastValidationRequest(ctx context.Context, eventID string, event events.Event, nodes []NodeID) {
	for _, nodeID := range nodes {
		if nodeID == dv.config.NodeID {
			continue // Skip self
		}

		go func(nID NodeID) {
			defer func() {
				if r := recover(); r != nil {
					// Log panic but continue
					fmt.Printf("Panic in distributed validator broadcast: %v\n", r)
				}
			}()
			
			// Check if context is already cancelled
			select {
			case <-ctx.Done():
				return
			default:
			}
			
			// TODO: Implement actual network communication
			// For now, this is a placeholder
			_ = nID
		}(nodeID)
	}
}

// aggregateDecisions aggregates validation decisions from multiple nodes
func (dv *DistributedValidator) aggregateDecisions(pending *PendingValidation) *ValidationResult {
	pending.DecisionsMutex.RLock()
	defer pending.DecisionsMutex.RUnlock()

	// Use consensus algorithm to determine final result
	decisions := make([]*ValidationDecision, 0, len(pending.Decisions))
	for _, decision := range pending.Decisions {
		decisions = append(decisions, decision)
	}

	return dv.consensus.AggregateDecisions(decisions)
}

// generateEventID generates a unique ID for an event
func (dv *DistributedValidator) generateEventID(event events.Event) string {
	// Try to get event ID if available
	if eventWithID, ok := event.(interface{ GetEventID() string }); ok {
		if id := eventWithID.GetEventID(); id != "" {
			return id
		}
	}

	// Generate based on event type and timestamp
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s-%s-%d", dv.config.NodeID, event.Type(), timestamp)
}

// getNextSequence returns the next sequence number for validation decisions
func (dv *DistributedValidator) getNextSequence() uint64 {
	return dv.metrics.GetNextSequence()
}

// heartbeatRoutine sends periodic heartbeats
func (dv *DistributedValidator) heartbeatRoutine(ctx context.Context) {
	ticker := time.NewTicker(dv.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-dv.stopChan:
			return
		case <-ticker.C:
			dv.sendHeartbeat()
		}
	}
}

// sendHeartbeat sends a heartbeat to other nodes
func (dv *DistributedValidator) sendHeartbeat() {
	// Update local node info
	dv.nodesMutex.Lock()
	if info, exists := dv.nodes[dv.config.NodeID]; exists {
		info.LastHeartbeat = time.Now()
		info.ValidationCount = dv.metrics.GetValidationCount()
		info.ErrorRate = dv.metrics.GetErrorRate()
		info.ResponseTimeMs = dv.metrics.GetAverageResponseTime()
		info.Load = dv.metrics.GetCurrentLoad()
	}
	dv.nodesMutex.Unlock()

	// TODO: Broadcast heartbeat to other nodes
}

// cleanupRoutine performs periodic cleanup tasks
func (dv *DistributedValidator) cleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-dv.stopChan:
			return
		case <-ticker.C:
			dv.cleanup()
		}
	}
}

// cleanup performs cleanup tasks
func (dv *DistributedValidator) cleanup() {
	// Clean up stale nodes
	dv.nodesMutex.Lock()
	now := time.Now()
	staleTimeout := 5 * dv.config.HeartbeatInterval

	for nodeID, info := range dv.nodes {
		if nodeID != dv.config.NodeID && now.Sub(info.LastHeartbeat) > staleTimeout {
			info.State = NodeStateFailed
			dv.partitionHandler.HandleNodeFailure(nodeID)
		}
	}
	dv.nodesMutex.Unlock()

	// Clean up old pending validations
	dv.validationMutex.Lock()
	for eventID, pending := range dv.pendingValidations {
		if now.Sub(pending.StartTime) > dv.config.ValidationTimeout*2 {
			delete(dv.pendingValidations, eventID)
		}
	}
	dv.validationMutex.Unlock()
}

// metricsRoutine collects and reports metrics
func (dv *DistributedValidator) metricsRoutine(ctx context.Context) {
	if !dv.config.EnableMetrics {
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-dv.stopChan:
			return
		case <-ticker.C:
			dv.collectMetrics()
		}
	}
}

// collectMetrics collects current metrics
func (dv *DistributedValidator) collectMetrics() {
	dv.nodesMutex.RLock()
	activeNodes := 0
	totalLoad := 0.0

	for _, info := range dv.nodes {
		if info.State == NodeStateActive {
			activeNodes++
			totalLoad += info.Load
		}
	}
	dv.nodesMutex.RUnlock()

	dv.metrics.UpdateClusterMetrics(activeNodes, totalLoad)
}

// consensusRoutine periodically checks for consensus completion
func (dv *DistributedValidator) consensusRoutine(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond) // Check every 100ms
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-dv.stopChan:
			return
		case <-ticker.C:
			dv.checkPendingConsensus()
		}
	}
}

// checkPendingConsensus checks all pending validations for consensus opportunities
func (dv *DistributedValidator) checkPendingConsensus() {
	dv.validationMutex.RLock()
	pendingCopy := make(map[string]*PendingValidation)
	for k, v := range dv.pendingValidations {
		pendingCopy[k] = v
	}
	dv.validationMutex.RUnlock()

	for _, pending := range pendingCopy {
		pending.DecisionsMutex.RLock()
		decisionCount := len(pending.Decisions)
		pending.DecisionsMutex.RUnlock()

		// Check if timeout is approaching
		timeRemaining := dv.config.ValidationTimeout - time.Since(pending.StartTime)
		if timeRemaining <= 500*time.Millisecond { // If less than 500ms remaining
			// Force consensus with available decisions
			result := dv.aggregateDecisions(pending)
			select {
			case pending.CompleteChan <- result:
				// Successfully sent result
			default:
				// Channel might already be closed or have a result
			}
		} else {
			// Check if we can reach consensus early
			dv.checkAndTriggerConsensus(pending, decisionCount)
		}
	}
}

// DistributedMetrics tracks metrics for distributed validation
type DistributedMetrics struct {
	validationCount     uint64
	errorCount          uint64
	timeoutCount        uint64
	totalDuration       time.Duration
	sequenceCounter     uint64
	activeNodes         int
	averageLoad         float64
	mutex               sync.RWMutex
}

// NewDistributedMetrics creates new distributed metrics
func NewDistributedMetrics() *DistributedMetrics {
	return &DistributedMetrics{}
}

// RecordValidation records a validation operation
func (m *DistributedMetrics) RecordValidation(duration time.Duration, success bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.validationCount++
	m.totalDuration += duration
	if !success {
		m.errorCount++
	}
}

// RecordTimeout records a validation timeout
func (m *DistributedMetrics) RecordTimeout() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.timeoutCount++
}

// GetNextSequence returns the next sequence number
func (m *DistributedMetrics) GetNextSequence() uint64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.sequenceCounter++
	return m.sequenceCounter
}

// GetValidationCount returns the total validation count
func (m *DistributedMetrics) GetValidationCount() uint64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	return m.validationCount
}

// GetErrorRate returns the error rate
func (m *DistributedMetrics) GetErrorRate() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if m.validationCount == 0 {
		return 0
	}
	
	return float64(m.errorCount) / float64(m.validationCount)
}

// GetAverageResponseTime returns the average response time in milliseconds
func (m *DistributedMetrics) GetAverageResponseTime() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if m.validationCount == 0 {
		return 0
	}
	
	avgDuration := m.totalDuration / time.Duration(m.validationCount)
	return float64(avgDuration.Milliseconds())
}

// GetCurrentLoad returns the current load (0.0 to 1.0)
func (m *DistributedMetrics) GetCurrentLoad() float64 {
	// TODO: Implement actual load calculation
	return 0.5
}

// UpdateClusterMetrics updates cluster-wide metrics
func (m *DistributedMetrics) UpdateClusterMetrics(activeNodes int, averageLoad float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.activeNodes = activeNodes
	m.averageLoad = averageLoad
}