package distributed

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
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

// TestingGoroutineRestartPolicy returns a restart policy suitable for testing
// This policy disables restarts to prevent goroutine leak issues in tests
func TestingGoroutineRestartPolicy() *GoroutineRestartPolicy {
	return &GoroutineRestartPolicy{
		MaxRestarts:       0, // No restarts during testing
		RestartWindow:     1 * time.Second,
		BaseBackoff:       10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 1.0,
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

// Stop stops the managed goroutine with timeout
func (gm *GoroutineManager) Stop() {
	gm.StopWithTimeout(10 * time.Second)
}

// StopWithTimeout stops the managed goroutine with a specified timeout
func (gm *GoroutineManager) StopWithTimeout(timeout time.Duration) {
	// First, signal that we shouldn't restart and cancel context
	gm.mu.Lock()
	gm.shouldRestart = false
	if gm.cancel != nil {
		gm.cancel()
	}
	// Set not running immediately after cancelling context
	// This helps other goroutines see the state change quickly
	wasRunning := gm.isRunning
	gm.isRunning = false
	gm.mu.Unlock()
	
	// Only wait if we were actually running
	if !wasRunning {
		return
	}
	
	// Wait for goroutine to finish without holding the lock, with timeout
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic while waiting for goroutine %s to stop: %v", gm.name, r)
			}
			close(done)
		}()
		gm.wg.Wait()
	}()
	
	select {
	case <-done:
		// Goroutine stopped gracefully
		log.Printf("Goroutine %s stopped gracefully", gm.name)
	case <-time.After(timeout):
		// Timeout occurred
		log.Printf("Warning: Goroutine %s did not stop within timeout %v", gm.name, timeout)
		// Force mark as not running even on timeout to prevent further waits
		gm.mu.Lock()
		gm.isRunning = false
		gm.mu.Unlock()
	}
}

// runWithRestart runs the function with restart capability
func (gm *GoroutineManager) runWithRestart(fn func(context.Context)) {
	defer gm.wg.Done()
	
	for {
		// Check context cancellation first
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
		
		// Run the function with panic recovery in the same goroutine
		// to avoid creating nested goroutines that are hard to track
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Panic in goroutine %s: %v", gm.name, r)
				}
			}()
			
			fn(gm.ctx)
		}()
		
		// Immediately check if context was cancelled after function completion
		// This prevents race conditions between context cancellation and shouldRestart checks
		if gm.ctx.Err() != nil {
			return
		}
		
		// Check if context was cancelled during function execution
		select {
		case <-gm.ctx.Done():
			// Context cancelled, stop restarting
			return
		default:
			// Function completed, check if we should restart
		}
		
		// Check if we should restart after function exit
		gm.mu.RLock()
		shouldRestart = gm.shouldRestart
		gm.mu.RUnlock()
		
		if !shouldRestart {
			return
		}
		
		// Check context again before attempting restart
		select {
		case <-gm.ctx.Done():
			return
		default:
		}
		
		// Function completed, but check if we're being stopped before restart
		gm.mu.RLock()
		if !gm.shouldRestart {
			gm.mu.RUnlock()
			return
		}
		gm.mu.RUnlock()
		
		// Function completed, handle restart with delay
		gm.handleRestart()
	}
}

// handleRestart handles the restart logic with exponential backoff
func (gm *GoroutineManager) handleRestart() {
	// Check context first
	select {
	case <-gm.ctx.Done():
		return
	default:
	}
	
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
	
	// Check if we should still restart
	gm.mu.RLock()
	shouldRestart := gm.shouldRestart
	gm.mu.RUnlock()
	
	if !shouldRestart {
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
	
	// Wait for backoff period with context cancellation check using timer to prevent goroutine leak
	timer := time.NewTimer(backoffDuration)
	defer func() {
		if !timer.Stop() {
			// Drain the timer channel if it wasn't stopped
			select {
			case <-timer.C:
			default:
			}
		}
	}()
	
	select {
	case <-gm.ctx.Done():
		// Context cancelled, stop restarting
		return
	case <-timer.C:
		// Check if we should still restart after backoff
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

	// NodeCleanupInterval is the interval for cleaning up stale nodes
	NodeCleanupInterval time.Duration

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
		NodeCleanupInterval:    5 * time.Minute,
		EnableMetrics:          true,
		ConsensusCircuitBreakerConfig:   errors.DefaultCircuitBreakerConfig("consensus"),
		StateSyncCircuitBreakerConfig:   errors.DefaultCircuitBreakerConfig("state-sync"),
		HeartbeatCircuitBreakerConfig:   errors.DefaultCircuitBreakerConfig("heartbeat"),
		GoroutineRestartPolicy: DefaultGoroutineRestartPolicy(),
	}
}

// TestingDistributedValidatorConfig returns configuration optimized for testing
// This prevents goroutine leaks by disabling restart policies and using shorter timeouts
func TestingDistributedValidatorConfig(nodeID NodeID) *DistributedValidatorConfig {
	testingStateSync := DefaultStateSyncConfig()
	testingStateSync.SyncInterval = 50 * time.Millisecond // Faster for tests
	
	testingPartitionHandler := DefaultPartitionHandlerConfig()
	testingPartitionHandler.HeartbeatTimeout = 100 * time.Millisecond
	testingPartitionHandler.AllowLocalValidation = true
	testingPartitionHandler.MinNodesForOperation = 1
	
	return &DistributedValidatorConfig{
		NodeID:                 nodeID,
		ConsensusConfig:        DefaultConsensusConfig(),
		StateSync:              testingStateSync,
		LoadBalancer:           DefaultLoadBalancerConfig(),
		PartitionHandler:       testingPartitionHandler,
		MaxNodeFailures:        2,
		ValidationTimeout:      2 * time.Second,
		HeartbeatInterval:      100 * time.Millisecond,
		NodeCleanupInterval:    1 * time.Second, // Faster cleanup for tests
		EnableMetrics:          true,
		ConsensusCircuitBreakerConfig:   errors.DefaultCircuitBreakerConfig("consensus"),
		StateSyncCircuitBreakerConfig:   errors.DefaultCircuitBreakerConfig("state-sync"),
		HeartbeatCircuitBreakerConfig:   errors.DefaultCircuitBreakerConfig("heartbeat"),
		GoroutineRestartPolicy: TestingGoroutineRestartPolicy(), // No restarts for tests
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
	
	// Pre-computed active nodes list for performance optimization
	activeNodes      []NodeID
	activeNodesMutex sync.RWMutex
	
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
		activeNodes:        make([]NodeID, 0),
		pendingValidations: make(map[string]*PendingValidation),
		metrics:            NewDistributedMetrics(),
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

	// Reset stopOnce for restart capability
	dv.stopOnce = sync.Once{}

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

	var errs []error
	
	// Use a single execution pattern to ensure Stop is called only once
	dv.stopOnce.Do(func() {
		// Stop managed goroutines with timeout - these should stop cleanly
		// The goroutine managers handle context cancellation internally
		stopTimeout := 2 * time.Second // Reasonable timeout for tests and production
		
		// Create a waitgroup for parallel shutdown of goroutines
		var goroutineWG sync.WaitGroup
		
		// Stop all goroutines in parallel for faster shutdown
		managers := []*GoroutineManager{
			dv.heartbeatManager, 
			dv.cleanupManager, 
			dv.metricsManager, 
			dv.consensusManager,
		}
		
		for i, manager := range managers {
			if manager != nil {
				goroutineWG.Add(1)
				go func(mgr *GoroutineManager, index int) {
					defer func() {
						// Ensure WaitGroup is always decremented even if panic occurs
						if r := recover(); r != nil {
							fmt.Printf("Panic stopping manager %d: %v\n", index, r)
						}
						goroutineWG.Done()
					}()
					mgr.StopWithTimeout(stopTimeout)
				}(manager, i)
			}
		}
		
		// Wait for all goroutines to stop or timeout with proper cleanup
		goroutineStopDone := make(chan struct{})
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Panic in goroutine stop waiter: %v\n", r)
				}
				close(goroutineStopDone)
			}()
			goroutineWG.Wait()
		}()
		
		select {
		case <-goroutineStopDone:
			// All goroutines stopped successfully
		case <-time.After(stopTimeout + 500*time.Millisecond):
			// Overall timeout - some goroutines may not have stopped
			errs = append(errs, fmt.Errorf("timeout waiting for goroutines to stop"))
		}

		// Stop components in reverse dependency order
		// Stop partition handler first as it might have dependencies on others
		if dv.partitionHandler != nil {
			if err := dv.partitionHandler.Stop(); err != nil {
				errs = append(errs, fmt.Errorf("failed to stop partition handler: %w", err))
			}
		}

		// Stop state sync (has worker manager)
		if dv.stateSync != nil {
			if err := dv.stateSync.Stop(); err != nil {
				errs = append(errs, fmt.Errorf("failed to stop state sync: %w", err))
			}
		}

		// Stop consensus last
		if dv.consensus != nil {
			if err := dv.consensus.Stop(); err != nil {
				errs = append(errs, fmt.Errorf("failed to stop consensus: %w", err))
			}
		}
		
		// Execute resource cleanup functions
		dv.cleanupMutex.Lock()
		for i, cleanup := range dv.resourceCleanup {
			if cleanup != nil {
				if err := cleanup(); err != nil {
					errs = append(errs, fmt.Errorf("resource cleanup error %d: %w", i, err))
				}
			}
		}
		dv.cleanupMutex.Unlock()

		// Mark as stopped
		dv.running = false
	})

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
	if nodeInfo == nil {
		return fmt.Errorf("nodeInfo cannot be nil")
	}

	dv.nodesMutex.Lock()
	dv.activeNodesMutex.Lock()
	defer dv.activeNodesMutex.Unlock()
	defer dv.nodesMutex.Unlock()

	dv.nodes[nodeInfo.ID] = nodeInfo
	dv.loadBalancer.UpdateNodeMetrics(nodeInfo.ID, nodeInfo.Load, nodeInfo.ResponseTimeMs)
	
	// Update active nodes cache if this is an active node
	if nodeInfo.State == NodeStateActive {
		dv.updateActiveNodesList()
	}

	return nil
}

// UnregisterNode removes a validation node
func (dv *DistributedValidator) UnregisterNode(nodeID NodeID) error {
	dv.nodesMutex.Lock()
	dv.activeNodesMutex.Lock()
	defer dv.activeNodesMutex.Unlock()
	defer dv.nodesMutex.Unlock()

	// Check if this was an active node before removing
	wasActive := false
	if info, exists := dv.nodes[nodeID]; exists && info.State == NodeStateActive {
		wasActive = true
	}

	delete(dv.nodes, nodeID)
	dv.loadBalancer.RemoveNode(nodeID)
	
	// Update active nodes cache if we removed an active node
	if wasActive {
		dv.updateActiveNodesList()
	}

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

// updateActiveNodesList rebuilds the cached active nodes list
// Must be called with both nodesMutex and activeNodesMutex write locks
func (dv *DistributedValidator) updateActiveNodesList() {
	activeNodes := make([]NodeID, 0, len(dv.nodes))
	for nodeID, info := range dv.nodes {
		if info.State == NodeStateActive {
			activeNodes = append(activeNodes, nodeID)
		}
	}
	dv.activeNodes = activeNodes
}

// updateNodeState updates a node's state and maintains the active nodes cache
func (dv *DistributedValidator) updateNodeState(nodeID NodeID, newState NodeState) {
	dv.nodesMutex.Lock()
	dv.activeNodesMutex.Lock()
	defer dv.activeNodesMutex.Unlock()
	defer dv.nodesMutex.Unlock()

	if info, exists := dv.nodes[nodeID]; exists {
		oldState := info.State
		info.State = newState
		
		// Only update cache if state changed between active/inactive
		if (oldState == NodeStateActive && newState != NodeStateActive) ||
		   (oldState != NodeStateActive && newState == NodeStateActive) {
			dv.updateActiveNodesList()
		}
	}
}

// getActiveNodesCopy returns a copy of the active nodes list for thread-safe access
func (dv *DistributedValidator) getActiveNodesCopy() []NodeID {
	dv.activeNodesMutex.RLock()
	defer dv.activeNodesMutex.RUnlock()
	
	if len(dv.activeNodes) == 0 {
		return nil
	}
	
	// Return a copy to prevent external modification
	activeNodesCopy := make([]NodeID, len(dv.activeNodes))
	copy(activeNodesCopy, dv.activeNodes)
	return activeNodesCopy
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

// WaitForGoroutineTermination waits for all managed goroutines to terminate
// This is useful for testing scenarios to ensure clean shutdown
func (dv *DistributedValidator) WaitForGoroutineTermination(timeout time.Duration) error {
	start := time.Now()
	deadline := start.Add(timeout)
	
	// Check every 10ms for goroutine termination
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if dv.AreAllGoroutinesTerminated() {
				return nil
			}
			
			// Check timeout
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for goroutines to terminate after %v", timeout)
			}
			
		case <-time.After(timeout):
			return fmt.Errorf("timeout waiting for goroutines to terminate")
		}
	}
}

// AreAllGoroutinesTerminated checks if all managed goroutines have terminated
func (dv *DistributedValidator) AreAllGoroutinesTerminated() bool {
	status := dv.GetGoroutineStatus()
	
	for _, goroutineStatus := range status {
		if goroutineStatus.IsRunning {
			return false
		}
	}
	
	return true
}

// GetRunningGoroutines returns a list of goroutines that are still running
func (dv *DistributedValidator) GetRunningGoroutines() []string {
	status := dv.GetGoroutineStatus()
	running := make([]string, 0)
	
	for name, goroutineStatus := range status {
		if goroutineStatus.IsRunning {
			running = append(running, name)
		}
	}
	
	return running
}

// GoroutineStatus represents the status of a managed goroutine
type GoroutineStatus struct {
	Name         string `json:"name"`
	IsRunning    bool   `json:"is_running"`
	RestartCount int64  `json:"restart_count"`
}

// selectValidationNodes selects nodes for validation based on load balancing
// Uses pre-computed active nodes list for optimal performance
func (dv *DistributedValidator) selectValidationNodes(event events.Event) []NodeID {
	// Get cached active nodes list (thread-safe copy)
	activeNodes := dv.getActiveNodesCopy()
	
	// If no active nodes cached, fall back to slow path
	if len(activeNodes) == 0 {
		return dv.selectValidationNodesSlow(event)
	}

	// Use load balancer to select nodes from pre-computed list
	requiredNodes := dv.consensus.GetRequiredNodes()
	return dv.loadBalancer.SelectNodes(activeNodes, requiredNodes)
}

// selectValidationNodesSlow is the fallback method when active nodes cache is empty
// This method filters nodes on-demand and rebuilds the cache
func (dv *DistributedValidator) selectValidationNodesSlow(event events.Event) []NodeID {
	dv.nodesMutex.RLock()
	dv.activeNodesMutex.Lock()
	
	// Rebuild active nodes cache while we have the locks
	activeNodes := make([]NodeID, 0, len(dv.nodes))
	for nodeID, info := range dv.nodes {
		if info.State == NodeStateActive {
			activeNodes = append(activeNodes, nodeID)
		}
	}
	
	// Update cache
	dv.activeNodes = make([]NodeID, len(activeNodes))
	copy(dv.activeNodes, activeNodes)
	
	dv.activeNodesMutex.Unlock()
	dv.nodesMutex.RUnlock()

	// Use load balancer to select nodes
	requiredNodes := dv.consensus.GetRequiredNodes()
	return dv.loadBalancer.SelectNodes(activeNodes, requiredNodes)
}

// broadcastValidationRequest broadcasts a validation request to selected nodes with async processing
func (dv *DistributedValidator) broadcastValidationRequest(ctx context.Context, eventID string, event events.Event, nodes []NodeID) {
	// Filter out self node
	targetNodes := make([]NodeID, 0, len(nodes))
	for _, nodeID := range nodes {
		if nodeID != dv.config.NodeID {
			targetNodes = append(targetNodes, nodeID)
		}
	}

	if len(targetNodes) == 0 {
		return
	}

	// Create timeout context for broadcast operations to prevent indefinite hanging
	broadcastCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Create buffered channel for batch processing
	batchSize := 5 // Process in batches of 5
	if len(targetNodes) < batchSize {
		batchSize = len(targetNodes)
	}

	// Create worker pool for async processing with proper cleanup
	workerChan := make(chan broadcastTask, len(targetNodes))
	resultChan := make(chan broadcastResult, len(targetNodes))
	var workerWG sync.WaitGroup

	// Start worker goroutines with wait group for proper cleanup
	for i := 0; i < batchSize; i++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			dv.broadcastWorker(broadcastCtx, eventID, event, workerChan, resultChan)
		}()
	}

	// Send tasks to workers
	for _, nodeID := range targetNodes {
		task := broadcastTask{
			nodeID:  nodeID,
			eventID: eventID,
			event:   event,
		}
		
		select {
		case workerChan <- task:
		case <-broadcastCtx.Done():
			close(workerChan)
			// Wait for workers to finish
			workerWG.Wait()
			return
		}
	}

	close(workerChan)

	// Collect results asynchronously with proper cleanup
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Panic in broadcast result collection: %v\n", r)
			}
			// Ensure workers are properly cleaned up
			workerWG.Wait()
			// Drain any remaining results to prevent goroutine leaks
			for {
				select {
				case <-resultChan:
					// Drain remaining results
				default:
					return
				}
			}
		}()

		successCount := 0
		failureCount := 0
		collectedResults := 0
		
		for collectedResults < len(targetNodes) {
			select {
			case result, ok := <-resultChan:
				if !ok {
					return // Channel closed
				}
				collectedResults++
				if result.err != nil {
					failureCount++
					dv.metrics.RecordBroadcastFailure(result.nodeID)
				} else {
					successCount++
					dv.metrics.RecordBroadcastSuccess(result.nodeID)
				}
			case <-broadcastCtx.Done():
				// Context cancelled, wait for workers to finish and return
				workerWG.Wait()
				return
			case <-time.After(8 * time.Second): // Timeout for result collection
				// Timeout occurred, wait for workers to finish and return
				workerWG.Wait()
				return
			}
		}

		// Update metrics
		dv.metrics.RecordBroadcastCompletion(successCount, failureCount)
	}()
}

// broadcastTask represents a broadcast task for a worker
type broadcastTask struct {
	nodeID  NodeID
	eventID string
	event   events.Event
}

// broadcastResult represents the result of a broadcast operation
type broadcastResult struct {
	nodeID NodeID
	err    error
}

// broadcastWorker processes broadcast tasks asynchronously
func (dv *DistributedValidator) broadcastWorker(ctx context.Context, eventID string, event events.Event, tasks <-chan broadcastTask, results chan<- broadcastResult) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in broadcast worker: %v\n", r)
			// Send panic result to prevent deadlock
			select {
			case results <- broadcastResult{nodeID: "unknown", err: fmt.Errorf("worker panic: %v", r)}:
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

			// Create timeout context for individual broadcast
			broadcastCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			
			err := dv.sendValidationRequestAsync(broadcastCtx, task.nodeID, task.eventID, task.event)
			cancel()

			// Send result with timeout protection
			select {
			case results <- broadcastResult{nodeID: task.nodeID, err: err}:
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
				// Timeout sending result, continue to prevent deadlock
				continue
			}
		}
	}
}

// sendValidationRequestAsync sends a validation request to a specific node asynchronously
func (dv *DistributedValidator) sendValidationRequestAsync(ctx context.Context, nodeID NodeID, eventID string, event events.Event) error {
	// Use circuit breaker to prevent cascade failures
	return dv.consensusCircuitBreaker.Execute(ctx, func() error {
		return dv.performNetworkCall(ctx, nodeID, eventID, event)
	})
}

// performNetworkCall performs the actual network call with retry logic
func (dv *DistributedValidator) performNetworkCall(ctx context.Context, nodeID NodeID, eventID string, event events.Event) error {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := dv.executeNetworkCall(ctx, nodeID, eventID, event)
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on the last attempt
		if attempt == maxRetries-1 {
			break
		}

		// Exponential backoff with jitter
		backoffDuration := time.Duration(100*(1<<attempt)) * time.Millisecond
		jitter := time.Duration(time.Now().UnixNano() % int64(backoffDuration/2))
		backoffDuration += jitter

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoffDuration):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("max retries exceeded for node %s: %w", nodeID, lastErr)
}

// executeNetworkCall executes the actual network call
func (dv *DistributedValidator) executeNetworkCall(ctx context.Context, nodeID NodeID, eventID string, event events.Event) error {
	// TODO: Implement actual network communication
	// For now, simulate async network call
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(50 * time.Millisecond): // Simulate network delay
		// Simulate occasional failures for testing
		if time.Now().UnixNano()%10 == 0 {
			return fmt.Errorf("simulated network failure for node %s", nodeID)
		}
		return nil
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

	// Generate based on event type and timestamp using strings.Builder for performance
	timestamp := time.Now().UnixNano()
	var builder strings.Builder
	builder.Grow(len(string(dv.config.NodeID)) + len(string(event.Type())) + 30) // Node + Type + timestamp + separators
	builder.WriteString(string(dv.config.NodeID))
	builder.WriteByte('-')
	builder.WriteString(string(event.Type()))
	builder.WriteByte('-')
	builder.WriteString(strconv.FormatInt(timestamp, 10))
	return builder.String()
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
			// Context cancelled, terminate immediately
			return
		case <-ticker.C:
			// Check context before processing to ensure quick termination
			select {
			case <-ctx.Done():
				return
			default:
				// Send heartbeat with context cancellation support
				// Use a shorter timeout context to prevent blocking on shutdown
				heartbeatCtx, cancel := context.WithTimeout(ctx, dv.config.HeartbeatInterval/2)
				dv.sendHeartbeatWithContext(heartbeatCtx)
				cancel()
			}
		}
	}
}

// sendHeartbeat sends a heartbeat to other nodes asynchronously
func (dv *DistributedValidator) sendHeartbeat() {
	dv.sendHeartbeatWithContext(context.Background())
}

// sendHeartbeatWithContext sends a heartbeat to other nodes asynchronously with context support
func (dv *DistributedValidator) sendHeartbeatWithContext(ctx context.Context) {
	// Check context before starting any work
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Update local node info
	dv.updateLocalNodeInfo()

	// Get active nodes to send heartbeat to
	targetNodes := dv.getActiveTargetNodes()
	if len(targetNodes) == 0 {
		return
	}

	// Check context again before async operation
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Send heartbeats asynchronously with context support
	go dv.broadcastHeartbeatAsyncWithContext(ctx, targetNodes)
}

// updateLocalNodeInfo updates the local node information
func (dv *DistributedValidator) updateLocalNodeInfo() {
	dv.nodesMutex.Lock()
	defer dv.nodesMutex.Unlock()
	
	if info, exists := dv.nodes[dv.config.NodeID]; exists {
		info.LastHeartbeat = time.Now()
		info.ValidationCount = dv.metrics.GetValidationCount()
		info.ErrorRate = dv.metrics.GetErrorRate()
		info.ResponseTimeMs = dv.metrics.GetAverageResponseTime()
		info.Load = dv.metrics.GetCurrentLoad()
	}
}

// getActiveTargetNodes returns a list of active nodes to send heartbeats to
func (dv *DistributedValidator) getActiveTargetNodes() []NodeID {
	dv.nodesMutex.RLock()
	defer dv.nodesMutex.RUnlock()
	
	targetNodes := make([]NodeID, 0)
	for nodeID, info := range dv.nodes {
		if nodeID != dv.config.NodeID && info.State == NodeStateActive {
			targetNodes = append(targetNodes, nodeID)
		}
	}
	
	return targetNodes
}

// broadcastHeartbeatAsync broadcasts heartbeat to nodes asynchronously
func (dv *DistributedValidator) broadcastHeartbeatAsync(nodes []NodeID) {
	dv.broadcastHeartbeatAsyncWithContext(context.Background(), nodes)
}

// broadcastHeartbeatAsyncWithContext broadcasts heartbeat to nodes asynchronously with context support
func (dv *DistributedValidator) broadcastHeartbeatAsyncWithContext(parentCtx context.Context, nodes []NodeID) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in heartbeat broadcast: %v\n", r)
		}
	}()

	// Check parent context before starting
	select {
	case <-parentCtx.Done():
		return
	default:
	}

	// Create timeout context derived from parent context for heartbeat operations
	ctx, cancel := context.WithTimeout(parentCtx, 8*time.Second)
	defer cancel()

	// Create buffered channel for heartbeat tasks
	heartbeatTasks := make(chan heartbeatTask, len(nodes))
	results := make(chan heartbeatResult, len(nodes))
	
	// Start worker pool for heartbeat processing with wait group
	workerCount := 3 // Limited workers for heartbeat
	if len(nodes) < workerCount {
		workerCount = len(nodes)
	}
	
	var workerWG sync.WaitGroup
	
	// Start workers with wait group tracking
	for i := 0; i < workerCount; i++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			dv.heartbeatWorker(ctx, heartbeatTasks, results)
		}()
	}
	
	// Send tasks to workers
	for _, nodeID := range nodes {
		task := heartbeatTask{
			nodeID:    nodeID,
			timestamp: time.Now(),
		}
		
		select {
		case heartbeatTasks <- task:
		case <-ctx.Done():
			close(heartbeatTasks)
			// Wait for workers to finish
			workerWG.Wait()
			return
		}
	}
	
	close(heartbeatTasks)
	
	// Collect results without blocking but with proper cleanup
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Panic in heartbeat result collection: %v\n", r)
			}
			// Ensure workers are cleaned up
			workerWG.Wait()
			// Drain any remaining results
			for {
				select {
				case <-results:
					// Drain remaining results
				default:
					return
				}
			}
		}()
		
		successCount := 0
		failureCount := 0
		collectedResults := 0
		
		for collectedResults < len(nodes) {
			select {
			case result, ok := <-results:
				if !ok {
					return // Channel closed
				}
				collectedResults++
				if result.err != nil {
					failureCount++
					dv.handleHeartbeatFailure(result.nodeID, result.err)
				} else {
					successCount++
					dv.handleHeartbeatSuccess(result.nodeID)
				}
			case <-ctx.Done():
				// Context cancelled, wait for workers and return
				workerWG.Wait()
				return
			case <-time.After(5 * time.Second):
				// Timeout, wait for workers and return
				workerWG.Wait()
				return
			}
		}
		
		dv.metrics.RecordHeartbeatCompletion(successCount, failureCount)
	}()
}

// heartbeatTask represents a heartbeat task
type heartbeatTask struct {
	nodeID    NodeID
	timestamp time.Time
}

// heartbeatResult represents the result of a heartbeat operation
type heartbeatResult struct {
	nodeID NodeID
	err    error
}

// heartbeatWorker processes heartbeat tasks
func (dv *DistributedValidator) heartbeatWorker(ctx context.Context, tasks <-chan heartbeatTask, results chan<- heartbeatResult) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in heartbeat worker: %v\n", r)
			// Send panic result to prevent deadlock
			select {
			case results <- heartbeatResult{nodeID: "unknown", err: fmt.Errorf("heartbeat worker panic: %v", r)}:
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

			// Send heartbeat with timeout
			heartbeatCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
			err := dv.sendHeartbeatToNode(heartbeatCtx, task.nodeID)
			cancel()

			// Send result with timeout protection
			select {
			case results <- heartbeatResult{nodeID: task.nodeID, err: err}:
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
				// Timeout sending result, continue to prevent deadlock
				continue
			}
		}
	}
}

// sendHeartbeatToNode sends a heartbeat to a specific node
func (dv *DistributedValidator) sendHeartbeatToNode(ctx context.Context, nodeID NodeID) error {
	// Use circuit breaker to prevent cascade failures
	return dv.heartbeatCircuitBreaker.Execute(ctx, func() error {
		return dv.executeHeartbeatCall(ctx, nodeID)
	})
}

// executeHeartbeatCall executes the actual heartbeat network call
func (dv *DistributedValidator) executeHeartbeatCall(ctx context.Context, nodeID NodeID) error {
	// TODO: Implement actual heartbeat network communication
	// For now, simulate async heartbeat call
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(50 * time.Millisecond): // Simulate network delay
		// Simulate occasional failures for testing
		if time.Now().UnixNano()%15 == 0 {
			return fmt.Errorf("heartbeat network failure for node %s", nodeID)
		}
		return nil
	}
}

// handleHeartbeatSuccess handles successful heartbeat
func (dv *DistributedValidator) handleHeartbeatSuccess(nodeID NodeID) {
	// Update node state if needed using optimized method
	dv.nodesMutex.RLock()
	needsUpdate := false
	if info, exists := dv.nodes[nodeID]; exists && info.State == NodeStateFailed {
		needsUpdate = true
	}
	dv.nodesMutex.RUnlock()
	
	if needsUpdate {
		dv.updateNodeState(nodeID, NodeStateActive)
	}
}

// handleHeartbeatFailure handles failed heartbeat
func (dv *DistributedValidator) handleHeartbeatFailure(nodeID NodeID, err error) {
	// Update node state using optimized method and handle potential failure
	dv.updateNodeState(nodeID, NodeStateFailed)
	dv.partitionHandler.HandleNodeFailure(nodeID)
}

// cleanupRoutine performs periodic cleanup tasks
func (dv *DistributedValidator) cleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context cancelled, terminate immediately
			return
		case <-ticker.C:
			// Check context before processing to ensure quick termination
			select {
			case <-ctx.Done():
				return
			default:
				// Perform cleanup with context awareness and timeout
				// Use a timeout to prevent blocking on shutdown
				cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				dv.cleanupWithContext(cleanupCtx)
				cancel()
			}
		}
	}
}

// cleanup performs cleanup tasks
func (dv *DistributedValidator) cleanup() {
	dv.cleanupWithContext(context.Background())
}

// cleanupWithContext performs cleanup tasks with context support
func (dv *DistributedValidator) cleanupWithContext(ctx context.Context) {
	// Check context before starting cleanup
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Clean up nodes first
	dv.cleanupNodesWithContext(ctx)
	
	// Check context before proceeding to next cleanup task
	select {
	case <-ctx.Done():
		return
	default:
	}
	
	// Clean up pending validations
	dv.cleanupPendingValidationsWithContext(ctx)
}

// cleanupNodes removes nodes not seen for > 5 minutes (heartbeat-based eviction)
func (dv *DistributedValidator) cleanupNodes() {
	dv.cleanupNodesWithContext(context.Background())
}

// cleanupNodesWithContext removes nodes not seen for > 5 minutes (heartbeat-based eviction) with context support
func (dv *DistributedValidator) cleanupNodesWithContext(ctx context.Context) {
	// Check context before starting work
	select {
	case <-ctx.Done():
		return
	default:
	}
	now := time.Now()
	cutoff := now.Add(-dv.config.NodeCleanupInterval)
	removedCount := 0
	failedNodes := make([]NodeID, 0)
	
	// First pass: collect nodes that need state changes
	dv.nodesMutex.RLock()
	for nodeID, info := range dv.nodes {
		// Check context periodically during iteration
		select {
		case <-ctx.Done():
			dv.nodesMutex.RUnlock()
			return
		default:
		}
		
		// Don't process self node
		if nodeID == dv.config.NodeID {
			continue
		}
		
		// Check if node should be marked as failed
		if !info.LastHeartbeat.Before(cutoff) && 
		   now.Sub(info.LastHeartbeat) > 5*dv.config.HeartbeatInterval &&
		   info.State != NodeStateFailed {
			failedNodes = append(failedNodes, nodeID)
		}
	}
	dv.nodesMutex.RUnlock()
	
	// Update failed nodes using optimized method
	for _, nodeID := range failedNodes {
		// Check context before each update
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		dv.updateNodeState(nodeID, NodeStateFailed)
		dv.partitionHandler.HandleNodeFailure(nodeID)
	}
	
	// Check context before second pass
	select {
	case <-ctx.Done():
		return
	default:
	}
	
	// Second pass: remove completely stale nodes
	dv.nodesMutex.Lock()
	dv.activeNodesMutex.Lock()
	needsCacheUpdate := false
	
	for nodeID, info := range dv.nodes {
		// Check context periodically during iteration
		select {
		case <-ctx.Done():
			dv.activeNodesMutex.Unlock()
			dv.nodesMutex.Unlock()
			return
		default:
		}
		
		// Don't remove self node
		if nodeID == dv.config.NodeID {
			continue
		}
		
		// Remove nodes not seen for > NodeCleanupInterval
		if info.LastHeartbeat.Before(cutoff) {
			wasActive := info.State == NodeStateActive
			delete(dv.nodes, nodeID)
			dv.partitionHandler.HandleNodeFailure(nodeID)
			removedCount++
			if wasActive {
				needsCacheUpdate = true
			}
		}
	}
	
	// Update cache if we removed active nodes
	if needsCacheUpdate {
		dv.updateActiveNodesList()
	}
	
	dv.activeNodesMutex.Unlock()
	dv.nodesMutex.Unlock()
	
	// Final context check before recording metrics
	select {
	case <-ctx.Done():
		return
	default:
	}
	
	if removedCount > 0 {
		dv.metrics.RecordNodesRemoved(removedCount)
	}
}

// cleanupPendingValidations removes old pending validations
func (dv *DistributedValidator) cleanupPendingValidations() {
	dv.cleanupPendingValidationsWithContext(context.Background())
}

// cleanupPendingValidationsWithContext removes old pending validations with context support
func (dv *DistributedValidator) cleanupPendingValidationsWithContext(ctx context.Context) {
	// Check context before starting work
	select {
	case <-ctx.Done():
		return
	default:
	}

	dv.validationMutex.Lock()
	// Check context again after acquiring lock
	select {
	case <-ctx.Done():
		dv.validationMutex.Unlock()
		return
	default:
	}

	now := time.Now()
	timeoutThreshold := dv.config.ValidationTimeout * 2
	removedCount := 0
	
	for eventID, pending := range dv.pendingValidations {
		// Check context periodically during iteration
		select {
		case <-ctx.Done():
			dv.validationMutex.Unlock()
			return
		default:
		}
		
		if now.Sub(pending.StartTime) > timeoutThreshold {
			delete(dv.pendingValidations, eventID)
			removedCount++
		}
	}
	
	dv.validationMutex.Unlock()
	
	// Final context check before recording metrics
	select {
	case <-ctx.Done():
		return
	default:
	}
	
	if removedCount > 0 {
		dv.metrics.RecordValidationsCleanup(removedCount)
	}
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
			// Context cancelled, terminate immediately
			return
		case <-ticker.C:
			// Check context before processing to ensure quick termination
			select {
			case <-ctx.Done():
				return
			default:
				// Collect metrics with context awareness and timeout
				// Use a short timeout to prevent blocking on shutdown
				metricsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				dv.collectMetricsWithContext(metricsCtx)
				cancel()
			}
		}
	}
}

// collectMetrics collects current metrics
func (dv *DistributedValidator) collectMetrics() {
	dv.collectMetricsWithContext(context.Background())
}

// collectMetricsWithContext collects current metrics with context support
func (dv *DistributedValidator) collectMetricsWithContext(ctx context.Context) {
	// Check context before starting work
	select {
	case <-ctx.Done():
		return
	default:
	}

	dv.nodesMutex.RLock()
	// Check context again after acquiring lock
	select {
	case <-ctx.Done():
		dv.nodesMutex.RUnlock()
		return
	default:
	}

	activeNodes := 0
	totalLoad := 0.0

	for _, info := range dv.nodes {
		// Check context periodically during iteration for responsive cancellation
		select {
		case <-ctx.Done():
			dv.nodesMutex.RUnlock()
			return
		default:
		}

		if info.State == NodeStateActive {
			activeNodes++
			totalLoad += info.Load
		}
	}
	dv.nodesMutex.RUnlock()

	// Final context check before updating metrics
	select {
	case <-ctx.Done():
		return
	default:
	}

	dv.metrics.UpdateClusterMetrics(activeNodes, totalLoad)
}

// consensusRoutine periodically checks for consensus completion
func (dv *DistributedValidator) consensusRoutine(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond) // Check every 100ms
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context cancelled, terminate immediately
			return
		case <-ticker.C:
			// Check context before processing to ensure quick termination
			select {
			case <-ctx.Done():
				return
			default:
				// Check pending consensus with context awareness and timeout
				// Use a short timeout to prevent blocking on shutdown
				consensusCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
				dv.checkPendingConsensusWithContext(consensusCtx)
				cancel()
			}
		}
	}
}

// checkPendingConsensus checks all pending validations for consensus opportunities
func (dv *DistributedValidator) checkPendingConsensus() {
	dv.checkPendingConsensusWithContext(context.Background())
}

// checkPendingConsensusWithContext checks all pending validations for consensus opportunities with context support
func (dv *DistributedValidator) checkPendingConsensusWithContext(ctx context.Context) {
	// Check context before starting work
	select {
	case <-ctx.Done():
		return
	default:
	}

	dv.validationMutex.RLock()
	// Check context again after acquiring lock
	select {
	case <-ctx.Done():
		dv.validationMutex.RUnlock()
		return
	default:
	}

	pendingCopy := make(map[string]*PendingValidation)
	for k, v := range dv.pendingValidations {
		// Check context periodically during iteration
		select {
		case <-ctx.Done():
			dv.validationMutex.RUnlock()
			return
		default:
		}
		pendingCopy[k] = v
	}
	dv.validationMutex.RUnlock()

	for _, pending := range pendingCopy {
		// Check context before processing each pending validation
		select {
		case <-ctx.Done():
			return
		default:
		}

		pending.DecisionsMutex.RLock()
		decisionCount := len(pending.Decisions)
		pending.DecisionsMutex.RUnlock()

		// Check context again before consensus logic
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Check if timeout is approaching
		timeRemaining := dv.config.ValidationTimeout - time.Since(pending.StartTime)
		if timeRemaining <= 500*time.Millisecond { // If less than 500ms remaining
			// Force consensus with available decisions
			result := dv.aggregateDecisions(pending)
			select {
			case pending.CompleteChan <- result:
				// Successfully sent result
			case <-ctx.Done():
				// Context cancelled while trying to send result
				return
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
// Fields are padded to prevent false sharing between atomic operations
type DistributedMetrics struct {
	validationCount     uint64
	_padding1          [7]uint64  // Prevent false sharing
	errorCount          uint64
	_padding2          [7]uint64  // Prevent false sharing
	timeoutCount        uint64
	_padding3          [7]uint64  // Prevent false sharing
	sequenceCounter     uint64
	_padding4          [7]uint64  // Prevent false sharing
	
	// Non-atomic fields (protected by mutex)
	totalDuration       time.Duration
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
	// Use atomic operations for counters to avoid false sharing
	atomic.AddUint64(&m.validationCount, 1)
	if !success {
		atomic.AddUint64(&m.errorCount, 1)
	}
	
	// Use mutex only for non-atomic fields
	m.mutex.Lock()
	m.totalDuration += duration
	m.mutex.Unlock()
}

// RecordTimeout records a validation timeout
func (m *DistributedMetrics) RecordTimeout() {
	atomic.AddUint64(&m.timeoutCount, 1)
}

// GetNextSequence returns the next sequence number
func (m *DistributedMetrics) GetNextSequence() uint64 {
	return atomic.AddUint64(&m.sequenceCounter, 1)
}

// GetValidationCount returns the total validation count
func (m *DistributedMetrics) GetValidationCount() uint64 {
	return atomic.LoadUint64(&m.validationCount)
}

// GetErrorRate returns the error rate
func (m *DistributedMetrics) GetErrorRate() float64 {
	validationCount := atomic.LoadUint64(&m.validationCount)
	if validationCount == 0 {
		return 0
	}
	
	errorCount := atomic.LoadUint64(&m.errorCount)
	return float64(errorCount) / float64(validationCount)
}

// GetAverageResponseTime returns the average response time in milliseconds
func (m *DistributedMetrics) GetAverageResponseTime() float64 {
	validationCount := atomic.LoadUint64(&m.validationCount)
	if validationCount == 0 {
		return 0
	}
	
	m.mutex.RLock()
	totalDuration := m.totalDuration
	m.mutex.RUnlock()
	
	avgDuration := totalDuration / time.Duration(validationCount)
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

// RecordBroadcastSuccess records a successful broadcast operation
func (m *DistributedMetrics) RecordBroadcastSuccess(nodeID NodeID) {
	// TODO: Implement broadcast success metrics
	// For now, this is a placeholder
}

// RecordBroadcastFailure records a failed broadcast operation
func (m *DistributedMetrics) RecordBroadcastFailure(nodeID NodeID) {
	// TODO: Implement broadcast failure metrics
	// For now, this is a placeholder
}

// RecordBroadcastCompletion records the completion of a broadcast round
func (m *DistributedMetrics) RecordBroadcastCompletion(successCount, failureCount int) {
	// TODO: Implement broadcast completion metrics
	// For now, this is a placeholder
}

// RecordHeartbeatCompletion records the completion of a heartbeat round
func (m *DistributedMetrics) RecordHeartbeatCompletion(successCount, failureCount int) {
	// TODO: Implement heartbeat completion metrics
	// For now, this is a placeholder
}

// RecordNodesRemoved records the number of nodes removed during cleanup
func (m *DistributedMetrics) RecordNodesRemoved(count int) {
	// TODO: Implement nodes removed metrics tracking
	// For now, this is a placeholder
}

// RecordValidationsCleanup records the number of validations cleaned up
func (m *DistributedMetrics) RecordValidationsCleanup(count int) {
	// TODO: Implement validations cleanup metrics tracking
	// For now, this is a placeholder
}