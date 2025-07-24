package distributed

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// DecoupledDistributedValidator implements DistributedValidatorProvider with proper dependency injection
type DecoupledDistributedValidator struct {
	// Core dependencies injected through constructor
	configProvider        ConfigProvider
	validationProvider    ValidationProvider
	consensusProvider     ConsensusProvider
	stateSyncProvider     StateSyncProvider
	loadBalancerProvider  LoadBalancerProvider
	partitionHandler      PartitionHandlerProvider
	metricsProvider       MetricsProvider
	networkProvider       NetworkProvider
	serviceRegistry       ServiceRegistry
	healthChecker         HealthChecker
	
	// Internal state
	nodeID              NodeID
	nodes               map[NodeID]*NodeInfo
	pendingValidations  map[string]*PendingValidation
	
	// Goroutine managers
	goroutineManagers   map[string]GoroutineManagerProvider
	
	// Lifecycle management
	state              ComponentState
	running            bool
	stopChan           chan struct{}
	stopOnce           sync.Once
	
	// Synchronization
	nodesMutex         sync.RWMutex
	validationMutex    sync.RWMutex
	stateMutex         sync.RWMutex
	
	// Tracing
	tracer             trace.Tracer
	
	// Configuration
	config             interface{}
	
	// Cleanup functions
	cleanupFuncs       []func() error
	cleanupMutex       sync.Mutex
}

// NewDecoupledDistributedValidator creates a new decoupled distributed validator
func NewDecoupledDistributedValidator(options ...ValidatorOption) (*DecoupledDistributedValidator, error) {
	validator := &DecoupledDistributedValidator{
		nodes:              make(map[NodeID]*NodeInfo),
		pendingValidations: make(map[string]*PendingValidation),
		goroutineManagers:  make(map[string]GoroutineManagerProvider),
		stopChan:           make(chan struct{}),
		cleanupFuncs:       make([]func() error, 0),
		tracer:             otel.Tracer("ag-ui/distributed-validation-decoupled"),
		state:              ComponentStateInitialized,
	}
	
	// Apply options
	for _, option := range options {
		if err := option.Apply(validator); err != nil {
			return nil, fmt.Errorf("failed to apply validator option: %w", err)
		}
	}
	
	// Validate dependencies
	if err := validator.validateDependencies(); err != nil {
		return nil, fmt.Errorf("dependency validation failed: %w", err)
	}
	
	// Initialize from configuration
	if err := validator.initializeFromConfig(); err != nil {
		return nil, fmt.Errorf("failed to initialize from config: %w", err)
	}
	
	return validator, nil
}

// ValidatorOption defines options for the validator
type ValidatorOption interface {
	Apply(validator *DecoupledDistributedValidator) error
}

// ConfigProviderOption sets the configuration provider
type ConfigProviderOption struct {
	Provider ConfigProvider
}

func (o *ConfigProviderOption) Apply(validator *DecoupledDistributedValidator) error {
	validator.configProvider = o.Provider
	return nil
}

func WithConfigProvider(provider ConfigProvider) ValidatorOption {
	return &ConfigProviderOption{Provider: provider}
}

// ValidationProviderOption sets the validation provider
type ValidationProviderOption struct {
	Provider ValidationProvider
}

func (o *ValidationProviderOption) Apply(validator *DecoupledDistributedValidator) error {
	validator.validationProvider = o.Provider
	return nil
}

func WithValidationProvider(provider ValidationProvider) ValidatorOption {
	return &ValidationProviderOption{Provider: provider}
}

// ConsensusProviderOption sets the consensus provider
type ConsensusProviderOption struct {
	Provider ConsensusProvider
}

func (o *ConsensusProviderOption) Apply(validator *DecoupledDistributedValidator) error {
	validator.consensusProvider = o.Provider
	return nil
}

func WithConsensusProvider(provider ConsensusProvider) ValidatorOption {
	return &ConsensusProviderOption{Provider: provider}
}

// StateSyncProviderOption sets the state synchronization provider
type StateSyncProviderOption struct {
	Provider StateSyncProvider
}

func (o *StateSyncProviderOption) Apply(validator *DecoupledDistributedValidator) error {
	validator.stateSyncProvider = o.Provider
	return nil
}

func WithStateSyncProvider(provider StateSyncProvider) ValidatorOption {
	return &StateSyncProviderOption{Provider: provider}
}

// LoadBalancerProviderOption sets the load balancer provider
type LoadBalancerProviderOption struct {
	Provider LoadBalancerProvider
}

func (o *LoadBalancerProviderOption) Apply(validator *DecoupledDistributedValidator) error {
	validator.loadBalancerProvider = o.Provider
	return nil
}

func WithLoadBalancerProvider(provider LoadBalancerProvider) ValidatorOption {
	return &LoadBalancerProviderOption{Provider: provider}
}

// PartitionHandlerOption sets the partition handler
type PartitionHandlerOption struct {
	Handler PartitionHandlerProvider
}

func (o *PartitionHandlerOption) Apply(validator *DecoupledDistributedValidator) error {
	validator.partitionHandler = o.Handler
	return nil
}

func WithPartitionHandler(handler PartitionHandlerProvider) ValidatorOption {
	return &PartitionHandlerOption{Handler: handler}
}

// MetricsProviderOption sets the metrics provider
type MetricsProviderOption struct {
	Provider MetricsProvider
}

func (o *MetricsProviderOption) Apply(validator *DecoupledDistributedValidator) error {
	validator.metricsProvider = o.Provider
	return nil
}

func WithMetricsProvider(provider MetricsProvider) ValidatorOption {
	return &MetricsProviderOption{Provider: provider}
}

// NetworkProviderOption sets the network provider
type NetworkProviderOption struct {
	Provider NetworkProvider
}

func (o *NetworkProviderOption) Apply(validator *DecoupledDistributedValidator) error {
	validator.networkProvider = o.Provider
	return nil
}

func WithNetworkProvider(provider NetworkProvider) ValidatorOption {
	return &NetworkProviderOption{Provider: provider}
}

// ServiceRegistryOption sets the service registry
type ServiceRegistryOption struct {
	Registry ServiceRegistry
}

func (o *ServiceRegistryOption) Apply(validator *DecoupledDistributedValidator) error {
	validator.serviceRegistry = o.Registry
	return nil
}

func WithServiceRegistry(registry ServiceRegistry) ValidatorOption {
	return &ServiceRegistryOption{Registry: registry}
}

// HealthCheckerOption sets the health checker
type HealthCheckerOption struct {
	Checker HealthChecker
}

func (o *HealthCheckerOption) Apply(validator *DecoupledDistributedValidator) error {
	validator.healthChecker = o.Checker
	return nil
}

func WithHealthChecker(checker HealthChecker) ValidatorOption {
	return &HealthCheckerOption{Checker: checker}
}

// Implementation of DistributedValidatorProvider interface

// ValidateEvent validates a single event using the injected validation provider
func (v *DecoupledDistributedValidator) ValidateEvent(ctx context.Context, event interface{}) (*ValidationResult, error) {
	ctx, span := v.tracer.Start(ctx, "decoupled_distributed_validation",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	start := time.Now()
	
	// Check if validator is running
	if !v.isRunning() {
		span.SetStatus(codes.Error, "validator not running")
		return &ValidationResult{
			IsValid: false,
			Errors: []*ValidationError{{
				RuleID:    "VALIDATOR_NOT_RUNNING",
				Message:   "Distributed validator is not running",
				Severity:  ValidationSeverityError,
				Timestamp: time.Now(),
			}},
			EventCount: 1,
			Duration:   time.Since(start),
			Timestamp:  time.Now(),
		}, nil
	}
	
	// Setup tracing
	span.SetAttributes(
		attribute.String("validator.type", "decoupled_distributed"),
		attribute.String("node.id", string(v.nodeID)),
	)
	
	// Check for partition
	if v.partitionHandler != nil && v.partitionHandler.IsPartitioned() {
		span.SetAttributes(attribute.Bool("node.partitioned", true))
		
		// Check if local validation is allowed during partition
		if allowLocal, err := v.configProvider.GetBool("partition.allow_local_validation"); err == nil && allowLocal {
			span.AddEvent("validating_locally_due_to_partition")
			return v.validationProvider.ValidateEvent(ctx, event)
		}
		
		result := &ValidationResult{
			IsValid: false,
			Errors: []*ValidationError{{
				RuleID:    "DISTRIBUTED_PARTITION",
				Message:   "Node is partitioned from cluster",
				Severity:  ValidationSeverityError,
				Timestamp: time.Now(),
			}},
			EventCount: 1,
			Duration:   time.Since(start),
			Timestamp:  time.Now(),
		}
		
		span.SetStatus(codes.Error, "node partitioned")
		return result, nil
	}
	
	// Perform distributed validation
	result, err := v.performDistributedValidation(ctx, event)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "distributed validation failed")
		return nil, err
	}
	
	// Record metrics
	if v.metricsProvider != nil {
		v.metricsProvider.RecordValidation(result.Duration, result.IsValid)
	}
	
	span.SetAttributes(
		attribute.Bool("validation.valid", result.IsValid),
		attribute.Int("validation.errors", len(result.Errors)),
		attribute.Int64("validation.duration_ms", result.Duration.Milliseconds()),
	)
	
	if result.IsValid {
		span.SetStatus(codes.Ok, "validation completed")
	} else {
		span.SetStatus(codes.Error, "validation failed")
	}
	
	return result, nil
}

// ValidateSequence validates a sequence of events
func (v *DecoupledDistributedValidator) ValidateSequence(ctx context.Context, events []interface{}) (*ValidationResult, error) {
	ctx, span := v.tracer.Start(ctx, "decoupled_distributed_sequence_validation",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()
	
	span.SetAttributes(
		attribute.Int("sequence.length", len(events)),
		attribute.String("validator.type", "decoupled_distributed"),
		attribute.String("node.id", string(v.nodeID)),
	)
	
	// Check if validator is running
	if !v.isRunning() {
		span.SetStatus(codes.Error, "validator not running")
		return &ValidationResult{
			IsValid: false,
			Errors: []*ValidationError{{
				RuleID:    "VALIDATOR_NOT_RUNNING",
				Message:   "Distributed validator is not running",
				Severity:  ValidationSeverityError,
				Timestamp: time.Now(),
			}},
			EventCount: len(events),
			Timestamp:  time.Now(),
		}, nil
	}
	
	// For sequence validation, we need distributed coordination
	if v.consensusProvider != nil {
		// Acquire distributed lock for sequence validation
		lockID := fmt.Sprintf("sequence-%d", time.Now().UnixNano())
		timeout, _ := v.configProvider.GetDuration("validation.timeout")
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		
		if err := v.consensusProvider.AcquireLock(ctx, lockID, timeout); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to acquire distributed lock")
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
			}, nil
		}
		defer v.consensusProvider.ReleaseLock(ctx, lockID)
	}
	
	// Synchronize state before sequence validation
	if v.stateSyncProvider != nil {
		if err := v.stateSyncProvider.SyncState(ctx); err != nil {
			span.RecordError(err)
			span.AddEvent("state_sync_failed")
			// Continue with validation but note the sync failure
		}
	}
	
	// Validate sequence using the injected validation provider
	result, err := v.validationProvider.ValidateSequence(ctx, events)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "sequence validation failed")
		return nil, err
	}
	
	// Record metrics
	if v.metricsProvider != nil {
		v.metricsProvider.RecordValidation(result.Duration, result.IsValid)
	}
	
	span.SetAttributes(
		attribute.Bool("validation.valid", result.IsValid),
		attribute.Int("validation.errors", len(result.Errors)),
		attribute.Int64("validation.duration_ms", result.Duration.Milliseconds()),
	)
	
	if result.IsValid {
		span.SetStatus(codes.Ok, "sequence validation completed")
	} else {
		span.SetStatus(codes.Error, "sequence validation failed")
	}
	
	return result, nil
}

// Start starts the distributed validator
func (v *DecoupledDistributedValidator) Start(ctx context.Context) error {
	v.stateMutex.Lock()
	defer v.stateMutex.Unlock()
	
	if v.running {
		return fmt.Errorf("validator is already running")
	}
	
	// Start all providers
	if err := v.startProviders(ctx); err != nil {
		return fmt.Errorf("failed to start providers: %w", err)
	}
	
	// Start managed goroutines
	if err := v.startGoroutines(ctx); err != nil {
		v.stopProviders()
		return fmt.Errorf("failed to start goroutines: %w", err)
	}
	
	v.running = true
	v.state = ComponentStateStarted
	
	return nil
}

// Stop stops the distributed validator
func (v *DecoupledDistributedValidator) Stop() error {
	v.stateMutex.Lock()
	defer v.stateMutex.Unlock()
	
	if !v.running {
		return nil
	}
	
	// Signal stop to all components
	v.stopOnce.Do(func() {
		close(v.stopChan)
	})
	
	// Stop managed goroutines
	v.stopGoroutines()
	
	// Stop providers
	v.stopProviders()
	
	// Execute cleanup functions
	v.executeCleanupFunctions()
	
	v.running = false
	v.state = ComponentStateStopped
	
	return nil
}

// GetName returns the validator name
func (v *DecoupledDistributedValidator) GetName() string {
	return "decoupled_distributed_validator"
}

// GetVersion returns the validator version
func (v *DecoupledDistributedValidator) GetVersion() string {
	return "1.0.0"
}

// IsHealthy returns the health status
func (v *DecoupledDistributedValidator) IsHealthy() bool {
	if v.healthChecker != nil {
		return v.healthChecker.CheckHealth() == HealthStatusHealthy
	}
	
	// Basic health check
	return v.isRunning() && v.validateDependencies() == nil
}

// GetMetrics returns validation metrics
func (v *DecoupledDistributedValidator) GetMetrics() interface{} {
	if v.metricsProvider != nil {
		return v.metricsProvider.GetMetrics()
	}
	return nil
}

// RegisterNode registers a validation node
func (v *DecoupledDistributedValidator) RegisterNode(nodeInfo *NodeInfo) error {
	v.nodesMutex.Lock()
	defer v.nodesMutex.Unlock()
	
	if nodeInfo == nil {
		return fmt.Errorf("nodeInfo cannot be nil")
	}
	
	v.nodes[nodeInfo.ID] = nodeInfo
	
	// Update load balancer
	if v.loadBalancerProvider != nil {
		v.loadBalancerProvider.UpdateNodeMetrics(nodeInfo.ID, nodeInfo.Load, nodeInfo.ResponseTimeMs)
	}
	
	return nil
}

// UnregisterNode removes a validation node
func (v *DecoupledDistributedValidator) UnregisterNode(nodeID NodeID) error {
	v.nodesMutex.Lock()
	defer v.nodesMutex.Unlock()
	
	delete(v.nodes, nodeID)
	
	// Remove from load balancer
	if v.loadBalancerProvider != nil {
		v.loadBalancerProvider.RemoveNode(nodeID)
	}
	
	return nil
}

// GetNodeInfo returns information about a node
func (v *DecoupledDistributedValidator) GetNodeInfo(nodeID NodeID) (*NodeInfo, bool) {
	v.nodesMutex.RLock()
	defer v.nodesMutex.RUnlock()
	
	info, exists := v.nodes[nodeID]
	return info, exists
}

// GetAllNodes returns information about all nodes
func (v *DecoupledDistributedValidator) GetAllNodes() map[NodeID]*NodeInfo {
	v.nodesMutex.RLock()
	defer v.nodesMutex.RUnlock()
	
	nodesCopy := make(map[NodeID]*NodeInfo)
	for k, v := range v.nodes {
		nodeCopy := *v
		nodesCopy[k] = &nodeCopy
	}
	
	return nodesCopy
}

// GetDistributedMetrics returns distributed validation metrics
func (v *DecoupledDistributedValidator) GetDistributedMetrics() *DistributedMetrics {
	// Create metrics from the metrics provider
	if v.metricsProvider != nil {
		if metrics, ok := v.metricsProvider.GetMetrics().(*DistributedMetrics); ok {
			return metrics
		}
	}
	
	// Return empty metrics if provider is not available
	return &DistributedMetrics{}
}

// GetGoroutineStatus returns the status of managed goroutines
func (v *DecoupledDistributedValidator) GetGoroutineStatus() map[string]GoroutineStatus {
	status := make(map[string]GoroutineStatus)
	
	for name, manager := range v.goroutineManagers {
		status[name] = manager.GetStatus()
	}
	
	return status
}

// RegisterCleanupFunc registers a cleanup function
func (v *DecoupledDistributedValidator) RegisterCleanupFunc(cleanup func() error) {
	v.cleanupMutex.Lock()
	defer v.cleanupMutex.Unlock()
	
	v.cleanupFuncs = append(v.cleanupFuncs, cleanup)
}

// GetConfiguration returns the validator configuration
func (v *DecoupledDistributedValidator) GetConfiguration() interface{} {
	return v.config
}

// UpdateConfiguration updates the validator configuration
func (v *DecoupledDistributedValidator) UpdateConfiguration(config interface{}) error {
	v.stateMutex.Lock()
	defer v.stateMutex.Unlock()
	
	v.config = config
	
	// Re-initialize from new configuration
	return v.initializeFromConfig()
}

// GetServiceRegistry returns the service registry
func (v *DecoupledDistributedValidator) GetServiceRegistry() ServiceRegistry {
	return v.serviceRegistry
}

// Helper methods

// validateDependencies validates that all required dependencies are injected
func (v *DecoupledDistributedValidator) validateDependencies() error {
	if v.configProvider == nil {
		return fmt.Errorf("config provider is required")
	}
	
	if v.validationProvider == nil {
		return fmt.Errorf("validation provider is required")
	}
	
	// Other providers are optional and can be nil
	return nil
}

// initializeFromConfig initializes the validator from configuration
func (v *DecoupledDistributedValidator) initializeFromConfig() error {
	if v.configProvider == nil {
		return fmt.Errorf("config provider not available")
	}
	
	// Get node ID from configuration
	nodeID, err := v.configProvider.GetString("node.id")
	if err != nil {
		return fmt.Errorf("failed to get node ID: %w", err)
	}
	
	v.nodeID = NodeID(nodeID)
	
	return nil
}

// startProviders starts all the injected providers
func (v *DecoupledDistributedValidator) startProviders(ctx context.Context) error {
	// Start consensus provider
	if v.consensusProvider != nil {
		if err := v.consensusProvider.Start(ctx); err != nil {
			return fmt.Errorf("failed to start consensus provider: %w", err)
		}
	}
	
	// Start state sync provider
	if v.stateSyncProvider != nil {
		if err := v.stateSyncProvider.Start(ctx); err != nil {
			return fmt.Errorf("failed to start state sync provider: %w", err)
		}
	}
	
	// Start partition handler
	if v.partitionHandler != nil {
		if err := v.partitionHandler.Start(ctx); err != nil {
			return fmt.Errorf("failed to start partition handler: %w", err)
		}
	}
	
	return nil
}

// stopProviders stops all the injected providers
func (v *DecoupledDistributedValidator) stopProviders() {
	// Stop partition handler
	if v.partitionHandler != nil {
		v.partitionHandler.Stop()
	}
	
	// Stop state sync provider
	if v.stateSyncProvider != nil {
		v.stateSyncProvider.Stop()
	}
	
	// Stop consensus provider
	if v.consensusProvider != nil {
		v.consensusProvider.Stop()
	}
}

// startGoroutines starts managed goroutines
func (v *DecoupledDistributedValidator) startGoroutines(ctx context.Context) error {
	// Start heartbeat goroutine
	if heartbeatManager, exists := v.goroutineManagers["heartbeat"]; exists {
		heartbeatManager.Start(ctx, v.heartbeatRoutine)
	}
	
	// Start cleanup goroutine
	if cleanupManager, exists := v.goroutineManagers["cleanup"]; exists {
		cleanupManager.Start(ctx, v.cleanupRoutine)
	}
	
	// Start metrics goroutine
	if metricsManager, exists := v.goroutineManagers["metrics"]; exists {
		metricsManager.Start(ctx, v.metricsRoutine)
	}
	
	return nil
}

// stopGoroutines stops managed goroutines
func (v *DecoupledDistributedValidator) stopGoroutines() {
	for _, manager := range v.goroutineManagers {
		manager.Stop()
	}
}

// executeCleanupFunctions executes all registered cleanup functions
func (v *DecoupledDistributedValidator) executeCleanupFunctions() {
	v.cleanupMutex.Lock()
	defer v.cleanupMutex.Unlock()
	
	for _, cleanup := range v.cleanupFuncs {
		if err := cleanup(); err != nil {
			// Log error but continue with other cleanup functions
			fmt.Printf("Error during cleanup: %v\n", err)
		}
	}
}

// isRunning returns whether the validator is running
func (v *DecoupledDistributedValidator) isRunning() bool {
	v.stateMutex.RLock()
	defer v.stateMutex.RUnlock()
	return v.running
}

// performDistributedValidation performs distributed validation
func (v *DecoupledDistributedValidator) performDistributedValidation(ctx context.Context, event interface{}) (*ValidationResult, error) {
	// Select nodes for validation
	var selectedNodes []NodeID
	if v.loadBalancerProvider != nil {
		v.nodesMutex.RLock()
		activeNodes := make([]NodeID, 0, len(v.nodes))
		for nodeID, info := range v.nodes {
			if info.State == NodeStateActive {
				activeNodes = append(activeNodes, nodeID)
			}
		}
		v.nodesMutex.RUnlock()
		
		requiredNodes := 1
		if v.consensusProvider != nil {
			requiredNodes = v.consensusProvider.GetRequiredNodes()
		}
		
		selectedNodes = v.loadBalancerProvider.SelectNodes(activeNodes, requiredNodes)
	}
	
	// Perform local validation
	result, err := v.validationProvider.ValidateEvent(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("local validation failed: %w", err)
	}
	
	// If no other nodes are selected, return local result
	if len(selectedNodes) <= 1 {
		return result, nil
	}
	
	// Broadcast validation request to other nodes
	if v.networkProvider != nil {
		eventID := fmt.Sprintf("event-%d", time.Now().UnixNano())
		
		// Type assert event to events.Event interface
		eventTyped, ok := event.(events.Event)
		if !ok {
			return nil, fmt.Errorf("event does not implement events.Event interface")
		}
		
		// Create pending validation
		pending := &PendingValidation{
			Event:        eventTyped,
			Decisions:    make(map[NodeID]*ValidationDecision),
			StartTime:    time.Now(),
			CompleteChan: make(chan *ValidationResult, 1),
		}
		
		v.validationMutex.Lock()
		v.pendingValidations[eventID] = pending
		v.validationMutex.Unlock()
		
		// Broadcast to selected nodes
		otherNodes := make([]NodeID, 0, len(selectedNodes)-1)
		for _, nodeID := range selectedNodes {
			if nodeID != v.nodeID {
				otherNodes = append(otherNodes, nodeID)
			}
		}
		
		if len(otherNodes) > 0 {
			go func() {
				if err := v.networkProvider.BroadcastMessage(ctx, event, otherNodes); err != nil {
					// Log error but don't fail validation
					fmt.Printf("Broadcast failed: %v\n", err)
				}
			}()
		}
		
		// Wait for consensus or timeout
		timeout, _ := v.configProvider.GetDuration("validation.timeout")
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		
		select {
		case consensusResult := <-pending.CompleteChan:
			v.validationMutex.Lock()
			delete(v.pendingValidations, eventID)
			v.validationMutex.Unlock()
			return consensusResult, nil
			
		case <-time.After(timeout):
			v.validationMutex.Lock()
			delete(v.pendingValidations, eventID)
			v.validationMutex.Unlock()
			
			if v.metricsProvider != nil {
				v.metricsProvider.RecordTimeout()
			}
			
			// Return local result on timeout
			return result, nil
		}
	}
	
	return result, nil
}

// Goroutine routines

// heartbeatRoutine sends periodic heartbeats
func (v *DecoupledDistributedValidator) heartbeatRoutine(ctx context.Context) {
	if v.networkProvider == nil {
		return
	}
	
	interval, _ := v.configProvider.GetDuration("heartbeat.interval")
	if interval <= 0 {
		interval = 30 * time.Second
	}
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-v.stopChan:
			return
		case <-ticker.C:
			v.sendHeartbeats(ctx)
		}
	}
}

// cleanupRoutine performs periodic cleanup
func (v *DecoupledDistributedValidator) cleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-v.stopChan:
			return
		case <-ticker.C:
			v.performCleanup()
		}
	}
}

// metricsRoutine collects and reports metrics
func (v *DecoupledDistributedValidator) metricsRoutine(ctx context.Context) {
	if v.metricsProvider == nil {
		return
	}
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-v.stopChan:
			return
		case <-ticker.C:
			v.collectMetrics()
		}
	}
}

// sendHeartbeats sends heartbeats to all active nodes
func (v *DecoupledDistributedValidator) sendHeartbeats(ctx context.Context) {
	v.nodesMutex.RLock()
	nodes := make([]NodeID, 0, len(v.nodes))
	for nodeID := range v.nodes {
		if nodeID != v.nodeID {
			nodes = append(nodes, nodeID)
		}
	}
	v.nodesMutex.RUnlock()
	
	for _, nodeID := range nodes {
		go func(id NodeID) {
			if err := v.networkProvider.SendHeartbeat(ctx, id); err != nil {
				if v.metricsProvider != nil {
					v.metricsProvider.RecordBroadcastFailure(id)
				}
			} else {
				if v.metricsProvider != nil {
					v.metricsProvider.RecordBroadcastSuccess(id)
				}
			}
		}(nodeID)
	}
}

// performCleanup performs cleanup operations
func (v *DecoupledDistributedValidator) performCleanup() {
	// Clean up old pending validations
	v.validationMutex.Lock()
	now := time.Now()
	timeout, _ := v.configProvider.GetDuration("validation.timeout")
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	
	for eventID, pending := range v.pendingValidations {
		if now.Sub(pending.StartTime) > timeout*2 {
			delete(v.pendingValidations, eventID)
		}
	}
	v.validationMutex.Unlock()
	
	// Clean up stale nodes
	v.nodesMutex.Lock()
	heartbeatInterval, _ := v.configProvider.GetDuration("heartbeat.interval")
	if heartbeatInterval <= 0 {
		heartbeatInterval = 30 * time.Second
	}
	staleTimeout := 5 * heartbeatInterval
	
	for nodeID, info := range v.nodes {
		if nodeID != v.nodeID && now.Sub(info.LastHeartbeat) > staleTimeout {
			info.State = NodeStateFailed
			if v.partitionHandler != nil {
				v.partitionHandler.HandleNodeFailure(nodeID)
			}
		}
	}
	v.nodesMutex.Unlock()
}

// collectMetrics collects current metrics
func (v *DecoupledDistributedValidator) collectMetrics() {
	// This would collect and update metrics from various providers
	// Implementation depends on the specific metrics provider
}

// GetState returns the current state of the validator
func (v *DecoupledDistributedValidator) GetState() ComponentState {
	v.stateMutex.RLock()
	defer v.stateMutex.RUnlock()
	return v.state
}