package distributed

import (
	"context"
	"time"
)

// ValidationProvider defines the interface for validation providers
type ValidationProvider interface {
	// ValidateEvent validates a single event
	ValidateEvent(ctx context.Context, event interface{}) (*ValidationResult, error)
	
	// ValidateSequence validates a sequence of events
	ValidateSequence(ctx context.Context, events []interface{}) (*ValidationResult, error)
	
	// GetName returns the provider name
	GetName() string
	
	// GetVersion returns the provider version
	GetVersion() string
	
	// IsHealthy returns the health status
	IsHealthy() bool
	
	// GetMetrics returns provider metrics
	GetMetrics() interface{}
}

// ConsensusCore handles core consensus operations
type ConsensusCore interface {
	// AggregateDecisions aggregates validation decisions
	AggregateDecisions(decisions []*ValidationDecision) *ValidationResult
	
	// GetRequiredNodes returns the number of nodes required for consensus
	GetRequiredNodes() int
	
	// GetAlgorithm returns the consensus algorithm name
	GetAlgorithm() string
}

// DistributedLockManager manages distributed locks
type DistributedLockManager interface {
	// AcquireLock acquires a distributed lock
	AcquireLock(ctx context.Context, lockID string, timeout time.Duration) error
	
	// ReleaseLock releases a distributed lock
	ReleaseLock(ctx context.Context, lockID string) error
}

// ConsensusLifecycle manages consensus provider lifecycle
type ConsensusLifecycle interface {
	// Start starts the consensus provider
	Start(ctx context.Context) error
	
	// Stop stops the consensus provider
	Stop() error
	
	// IsHealthy returns the health status
	IsHealthy() bool
}

// ConsensusProvider defines the interface for consensus algorithms
// Composed of focused interfaces following Interface Segregation Principle
type ConsensusProvider interface {
	ConsensusCore
	DistributedLockManager
	ConsensusLifecycle
}

// StateSynchronizerInterface handles state synchronization operations
type StateSynchronizerInterface interface {
	// SyncState synchronizes state across nodes
	SyncState(ctx context.Context) error
	
	// GetSyncStatus returns the synchronization status
	GetSyncStatus() SyncStatus
	
	// GetProtocol returns the synchronization protocol name
	GetProtocol() string
}

// StateChangeNotifier handles state change notifications
type StateChangeNotifier interface {
	// RegisterStateChangeCallback registers a callback for state changes
	RegisterStateChangeCallback(callback func(state interface{})) error
}

// StateSyncLifecycle manages state sync provider lifecycle
type StateSyncLifecycle interface {
	// Start starts the state synchronization provider
	Start(ctx context.Context) error
	
	// Stop stops the state synchronization provider
	Stop() error
	
	// IsHealthy returns the health status
	IsHealthy() bool
}

// StateSyncProvider defines the interface for state synchronization
// Composed of focused interfaces following Interface Segregation Principle
type StateSyncProvider interface {
	StateSynchronizerInterface
	StateChangeNotifier
	StateSyncLifecycle
}

// NodeSelector handles node selection for load balancing
type NodeSelector interface {
	// SelectNodes selects nodes for validation based on load balancing strategy
	SelectNodes(availableNodes []NodeID, requiredCount int) []NodeID
	
	// GetStrategy returns the load balancing strategy
	GetStrategy() string
}

// NodeMetricsManager manages node metrics for load balancing
type NodeMetricsManager interface {
	// UpdateNodeMetrics updates metrics for a node
	UpdateNodeMetrics(nodeID NodeID, load float64, responseTime float64)
	
	// RemoveNode removes a node from load balancing
	RemoveNode(nodeID NodeID)
	
	// GetNodeMetrics returns metrics for all nodes
	GetNodeMetrics() map[NodeID]NodeMetrics
}

// LoadBalancerHealth provides health status for load balancer
type LoadBalancerHealth interface {
	// IsHealthy returns the health status
	IsHealthy() bool
}

// LoadBalancerProvider defines the interface for load balancing
// Composed of focused interfaces following Interface Segregation Principle
type LoadBalancerProvider interface {
	NodeSelector
	NodeMetricsManager
	LoadBalancerHealth
}

// PartitionHandlerProvider defines the interface for partition handling
type PartitionHandlerProvider interface {
	// Start starts the partition handler
	Start(ctx context.Context) error
	
	// Stop stops the partition handler
	Stop() error
	
	// IsPartitioned returns whether the node is partitioned
	IsPartitioned() bool
	
	// HandleNodeFailure handles node failure
	HandleNodeFailure(nodeID NodeID)
	
	// GetPartitionStatus returns the partition status
	GetPartitionStatus() PartitionStatus
	
	// RegisterPartitionCallback registers a callback for partition events
	RegisterPartitionCallback(callback func(event PartitionEvent)) error
	
	// IsHealthy returns the health status
	IsHealthy() bool
}

// MetricsProvider defines the interface for metrics collection
type MetricsProvider interface {
	// RecordValidation records a validation operation
	RecordValidation(duration time.Duration, success bool)
	
	// RecordTimeout records a validation timeout
	RecordTimeout()
	
	// RecordBroadcastSuccess records a successful broadcast
	RecordBroadcastSuccess(nodeID NodeID)
	
	// RecordBroadcastFailure records a failed broadcast
	RecordBroadcastFailure(nodeID NodeID)
	
	// RecordHeartbeat records a heartbeat operation
	RecordHeartbeat(nodeID NodeID, success bool)
	
	// GetMetrics returns collected metrics
	GetMetrics() interface{}
	
	// GetName returns the provider name
	GetName() string
	
	// IsHealthy returns the health status
	IsHealthy() bool
}

// CircuitBreakerProvider defines the interface for circuit breakers
type CircuitBreakerProvider interface {
	// Execute executes a function with circuit breaker protection
	Execute(fn func() error) error
	
	// GetState returns the current circuit breaker state
	GetState() CircuitBreakerState
	
	// GetName returns the circuit breaker name
	GetName() string
	
	// GetMetrics returns circuit breaker metrics
	GetMetrics() CircuitBreakerMetrics
	
	// IsHealthy returns the health status
	IsHealthy() bool
}

// GoroutineManagerProvider defines the interface for goroutine management
type GoroutineManagerProvider interface {
	// Start starts the managed goroutine
	Start(ctx context.Context, fn func(context.Context))
	
	// Stop stops the managed goroutine
	Stop()
	
	// GetRestartCount returns the current restart count
	GetRestartCount() int64
	
	// IsRunning returns whether the goroutine is running
	IsRunning() bool
	
	// GetName returns the manager name
	GetName() string
	
	// GetStatus returns the manager status
	GetStatus() GoroutineStatus
}

// ValidationRequestSender handles validation request sending
type ValidationRequestSender interface {
	// SendValidationRequest sends a validation request to a node
	SendValidationRequest(ctx context.Context, nodeID NodeID, event interface{}) error
}

// HeartbeatSender handles heartbeat operations
type HeartbeatSender interface {
	// SendHeartbeat sends a heartbeat to a node
	SendHeartbeat(ctx context.Context, nodeID NodeID) error
}

// MessageBroadcaster handles message broadcasting
type MessageBroadcaster interface {
	// BroadcastMessage broadcasts a message to multiple nodes
	BroadcastMessage(ctx context.Context, message interface{}, nodes []NodeID) error
}

// ConnectionStatusProvider provides connection status information
type ConnectionStatusProvider interface {
	// GetConnectionStatus returns the connection status for a node
	GetConnectionStatus(nodeID NodeID) ConnectionStatus
}

// NetworkProviderMetadata provides metadata about the network provider
type NetworkProviderMetadata interface {
	// GetName returns the provider name
	GetName() string
	
	// IsHealthy returns the health status
	IsHealthy() bool
}

// NetworkProvider defines the interface for network operations
// Composed of focused interfaces following Interface Segregation Principle
type NetworkProvider interface {
	ValidationRequestSender
	HeartbeatSender
	MessageBroadcaster
	ConnectionStatusProvider
	NetworkProviderMetadata
}

// ConfigProvider defines the interface for configuration access
type ConfigProvider interface {
	// GetString retrieves a string configuration value
	GetString(key string) (string, error)
	
	// GetInt retrieves an integer configuration value
	GetInt(key string) (int, error)
	
	// GetDuration retrieves a duration configuration value
	GetDuration(key string) (time.Duration, error)
	
	// GetBool retrieves a boolean configuration value
	GetBool(key string) (bool, error)
	
	// IsEnabled checks if a feature is enabled
	IsEnabled(feature string) bool
	
	// Watch watches for configuration changes
	Watch(callback func(key string, value interface{})) error
	
	// Validate validates the configuration
	Validate() error
}

// Supporting types and enums

// SyncStatus represents the state synchronization status
type SyncStatus int

const (
	SyncStatusUnknown SyncStatus = iota
	SyncStatusSyncing
	SyncStatusSynced
	SyncStatusFailed
)

// PartitionStatus represents the partition status
type PartitionStatus int

const (
	PartitionStatusUnknown PartitionStatus = iota
	PartitionStatusHealthy
	PartitionStatusPartitioned
	PartitionStatusRecovering
)

// PartitionEvent represents a partition event
type PartitionEvent struct {
	Type      PartitionEventType
	NodeID    NodeID
	Timestamp time.Time
	Details   map[string]interface{}
}

// PartitionEventType represents the type of partition event
type PartitionEventType int

const (
	PartitionEventTypeUnknown PartitionEventType = iota
	PartitionEventTypeNodeDown
	PartitionEventTypeNodeUp
	PartitionEventTypeNetworkSplit
	PartitionEventTypeNetworkRecovered
)

// NodeMetrics represents metrics for a node
type NodeMetrics struct {
	NodeID       NodeID
	Load         float64
	ResponseTime float64
	ErrorRate    float64
	Timestamp    time.Time
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerStateClosed CircuitBreakerState = iota
	CircuitBreakerStateOpen
	CircuitBreakerStateHalfOpen
)

// CircuitBreakerMetrics represents metrics for a circuit breaker
type CircuitBreakerMetrics struct {
	State               CircuitBreakerState
	RequestsTotal       int64
	RequestsSuccessful  int64
	RequestsFailed      int64
	ConsecutiveFailures int64
	LastFailureTime     time.Time
	LastSuccessTime     time.Time
}

// ConnectionStatus represents the connection status to a node
type ConnectionStatus int

const (
	ConnectionStatusUnknown ConnectionStatus = iota
	ConnectionStatusConnected
	ConnectionStatusDisconnected
	ConnectionStatusConnecting
	ConnectionStatusError
)

// ServiceRegistrar handles service registration operations
type ServiceRegistrar interface {
	// RegisterService registers a service
	RegisterService(name string, service interface{}) error
	
	// UnregisterService removes a service
	UnregisterService(name string) error
}

// ServiceLocator handles service discovery operations
type ServiceLocator interface {
	// GetService retrieves a service by name
	GetService(name string) (interface{}, error)
	
	// GetTypedService retrieves a service with type assertion
	GetTypedService(name string, target interface{}) error
}

// ServiceListing provides service listing capabilities
type ServiceListing interface {
	// ListServices returns all registered services
	ListServices() []string
	
	// IsServiceRegistered checks if a service is registered
	IsServiceRegistered(name string) bool
}

// ServiceHealthMonitor provides service health monitoring
type ServiceHealthMonitor interface {
	// GetServiceHealth returns the health status of a service
	GetServiceHealth(name string) (bool, error)
}

// ServiceRegistry defines the interface for service registration and discovery
// Composed of focused interfaces following Interface Segregation Principle
type ServiceRegistry interface {
	ServiceRegistrar
	ServiceLocator
	ServiceListing
	ServiceHealthMonitor
}

// ValidatorFactory creates distributed validator instances
type ValidatorFactory interface {
	// CreateDistributedValidator creates a new distributed validator
	CreateDistributedValidator(config interface{}) (DistributedValidatorProvider, error)
}

// ConsensusFactory creates consensus provider instances
type ConsensusFactory interface {
	// CreateConsensusProvider creates a consensus provider
	CreateConsensusProvider(algorithm string, config interface{}) (ConsensusProvider, error)
}

// StateSyncFactory creates state sync provider instances
type StateSyncFactory interface {
	// CreateStateSyncProvider creates a state synchronization provider
	CreateStateSyncProvider(protocol string, config interface{}) (StateSyncProvider, error)
}

// LoadBalancerFactory creates load balancer provider instances
type LoadBalancerFactory interface {
	// CreateLoadBalancerProvider creates a load balancer provider
	CreateLoadBalancerProvider(strategy string, config interface{}) (LoadBalancerProvider, error)
}

// PartitionHandlerFactory creates partition handler provider instances
type PartitionHandlerFactory interface {
	// CreatePartitionHandlerProvider creates a partition handler provider
	CreatePartitionHandlerProvider(config interface{}) (PartitionHandlerProvider, error)
}

// NetworkProviderFactory creates network provider instances
type NetworkProviderFactory interface {
	// CreateNetworkProvider creates a network provider
	CreateNetworkProvider(config interface{}) (NetworkProvider, error)
}

// DistributedValidatorFactory defines the interface for creating distributed validators
// Composed of focused interfaces following Interface Segregation Principle
type DistributedValidatorFactory interface {
	ValidatorFactory
	ConsensusFactory
	StateSyncFactory
	LoadBalancerFactory
	PartitionHandlerFactory
	NetworkProviderFactory
	
	// CreateMetricsProvider creates a metrics provider
	CreateMetricsProvider(config interface{}) (MetricsProvider, error)
}

// ValidatorLifecycleManager manages validator lifecycle
type ValidatorLifecycleManager interface {
	// Start starts the distributed validator
	Start(ctx context.Context) error
	
	// Stop stops the distributed validator
	Stop() error
	
	// RegisterCleanupFunc registers a cleanup function
	RegisterCleanupFunc(cleanup func() error)
}

// NodeRegistryManager manages node registration and information
type NodeRegistryManager interface {
	// RegisterNode registers a validation node
	RegisterNode(nodeInfo *NodeInfo) error
	
	// UnregisterNode removes a validation node
	UnregisterNode(nodeID NodeID) error
	
	// GetNodeInfo returns information about a node
	GetNodeInfo(nodeID NodeID) (*NodeInfo, bool)
	
	// GetAllNodes returns information about all nodes
	GetAllNodes() map[NodeID]*NodeInfo
}

// DistributedMetricsProvider provides metrics and status information
type DistributedMetricsProvider interface {
	// GetDistributedMetrics returns distributed validation metrics
	GetDistributedMetrics() *DistributedMetrics
	
	// GetGoroutineStatus returns the status of managed goroutines
	GetGoroutineStatus() map[string]GoroutineStatus
}

// ConfigurationManager handles configuration management
type ConfigurationManager interface {
	// GetConfiguration returns the validator configuration
	GetConfiguration() interface{}
	
	// UpdateConfiguration updates the validator configuration
	UpdateConfiguration(config interface{}) error
}

// ServiceRegistryProvider provides access to service registry
type ServiceRegistryProvider interface {
	// GetServiceRegistry returns the service registry
	GetServiceRegistry() ServiceRegistry
}

// DistributedValidatorProvider defines the main interface for distributed validators
// Composed of focused interfaces following Interface Segregation Principle
type DistributedValidatorProvider interface {
	ValidationProvider
	ValidatorLifecycleManager
	NodeRegistryManager
	DistributedMetricsProvider
	ConfigurationManager
	ServiceRegistryProvider
}

// HealthChecker defines the interface for health checking
type HealthChecker interface {
	// CheckHealth checks the health of the component
	CheckHealth() HealthStatus
	
	// GetHealthDetails returns detailed health information
	GetHealthDetails() map[string]interface{}
	
	// RegisterHealthCheck registers a health check function
	RegisterHealthCheck(name string, check func() error) error
	
	// UnregisterHealthCheck removes a health check
	UnregisterHealthCheck(name string) error
}

// HealthStatus represents the health status of a component
type HealthStatus int

const (
	HealthStatusUnknown HealthStatus = iota
	HealthStatusHealthy
	HealthStatusDegraded
	HealthStatusUnhealthy
)

// String returns the string representation of the health status
func (s HealthStatus) String() string {
	switch s {
	case HealthStatusHealthy:
		return "healthy"
	case HealthStatusDegraded:
		return "degraded"
	case HealthStatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// ComponentLifecycle defines the interface for component lifecycle management
type ComponentLifecycle interface {
	// Initialize initializes the component
	Initialize(config interface{}) error
	
	// Start starts the component
	Start(ctx context.Context) error
	
	// Stop stops the component
	Stop() error
	
	// Restart restarts the component
	Restart(ctx context.Context) error
	
	// GetState returns the current state of the component
	GetState() ComponentState
	
	// GetName returns the component name
	GetName() string
	
	// GetVersion returns the component version
	GetVersion() string
}

// ComponentState represents the state of a component
type ComponentState int

const (
	ComponentStateUnknown ComponentState = iota
	ComponentStateInitialized
	ComponentStateStarted
	ComponentStateStopped
	ComponentStateError
)

// String returns the string representation of the component state
func (s ComponentState) String() string {
	switch s {
	case ComponentStateInitialized:
		return "initialized"
	case ComponentStateStarted:
		return "started"
	case ComponentStateStopped:
		return "stopped"
	case ComponentStateError:
		return "error"
	default:
		return "unknown"
	}
}