// Package server provides agent registry and discovery functionality for the AG-UI Go SDK.
// The registry manages agent lifecycle, health monitoring, capability advertising, and
// load balancing across agent instances with dynamic registration and performance-aware routing.
package server

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/client"
	pkgerrors "github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/transport"
)

// ==============================================================================
// CORE REGISTRY INTERFACES
// ==============================================================================

// ExtendedAgentRegistry defines the extended interface for agent registration and discovery.
// It provides comprehensive agent management capabilities including registration,
// health monitoring, capability discovery, and load balancing.
// This extends the base AgentRegistry interface from the framework package.
type ExtendedAgentRegistry interface {
	// Agent lifecycle management
	RegisterAgent(ctx context.Context, agent client.Agent, metadata *AgentRegistrationMetadata) error
	UnregisterAgent(ctx context.Context, agentID string) error
	GetAgent(ctx context.Context, agentID string) (*AgentRegistration, error)
	ListAgents(ctx context.Context, filter *AgentFilter) ([]*AgentRegistration, error)

	// Health monitoring
	UpdateAgentHealth(ctx context.Context, agentID string, health *AgentHealthStatus) error
	GetAgentHealth(ctx context.Context, agentID string) (*AgentHealthStatus, error)
	GetHealthyAgents(ctx context.Context, capabilities []string) ([]*AgentRegistration, error)

	// Capability management
	UpdateAgentCapabilities(ctx context.Context, agentID string, capabilities *client.AgentCapabilities) error
	FindAgentsByCapability(ctx context.Context, capability string) ([]*AgentRegistration, error)
	GetCapabilityMatrix(ctx context.Context) (map[string][]string, error)

	// Load balancing and routing
	SelectAgent(ctx context.Context, request *AgentSelectionRequest) (*AgentRegistration, error)
	GetLoadBalancingStats(ctx context.Context) (*LoadBalancingStats, error)
	UpdateAgentMetrics(ctx context.Context, agentID string, metrics *AgentPerformanceMetrics) error

	// Service discovery
	DiscoverAgents(ctx context.Context, query *DiscoveryQuery) (*DiscoveryResult, error)
	WatchAgentChanges(ctx context.Context) (<-chan *AgentChangeEvent, error)

	// Registry management
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	GetRegistryStats(ctx context.Context) (*RegistryStats, error)
}

// AgentHealthChecker defines the interface for performing health checks on agents.
type AgentHealthChecker interface {
	// CheckHealth performs a health check on the specified agent
	CheckHealth(ctx context.Context, agent client.Agent) (*AgentHealthStatus, error)
	
	// GetHealthCheckInterval returns the interval for health checks
	GetHealthCheckInterval() time.Duration
	
	// SetHealthCheckInterval sets the interval for health checks
	SetHealthCheckInterval(interval time.Duration)
}

// LoadBalancer defines the interface for load balancing across agent instances.
type LoadBalancer interface {
	// SelectAgent selects an agent based on the provided criteria and current load
	SelectAgent(ctx context.Context, agents []*AgentRegistration, request *AgentSelectionRequest) (*AgentRegistration, error)
	
	// UpdateAgentLoad updates the load information for an agent
	UpdateAgentLoad(ctx context.Context, agentID string, load *AgentLoadInfo) error
	
	// GetAlgorithm returns the load balancing algorithm name
	GetAlgorithm() string
}

// AgentWatcher defines the interface for watching agent changes.
type AgentWatcher interface {
	// Watch returns a channel that receives agent change events
	Watch(ctx context.Context) (<-chan *AgentChangeEvent, error)
	
	// Close stops watching and closes the event channel
	Close() error
}

// ==============================================================================
// DATA STRUCTURES
// ==============================================================================

// AgentRegistration represents a registered agent with all its metadata.
type AgentRegistration struct {
	// Core agent information
	AgentID     string        `json:"agent_id"`
	Agent       client.Agent  `json:"-"` // Not serialized
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Version     string        `json:"version"`
	
	// Registration metadata
	RegisteredAt time.Time                  `json:"registered_at"`
	LastSeen     time.Time                  `json:"last_seen"`
	Metadata     *AgentRegistrationMetadata `json:"metadata"`
	
	// Capabilities and health
	Capabilities *client.AgentCapabilities `json:"capabilities"`
	Health       *AgentHealthStatus        `json:"health"`
	
	// Performance and load balancing
	Metrics    *AgentPerformanceMetrics `json:"metrics"`
	LoadInfo   *AgentLoadInfo           `json:"load_info"`
	
	// Transport and connectivity
	Transport transport.Transport `json:"-"` // Not serialized
	Endpoints []string            `json:"endpoints"`
	
	// Internal state
	Status       AgentRegistrationStatus `json:"status"`
	FailureCount int32                   `json:"failure_count"`
	mu           sync.RWMutex            `json:"-"`
}

// AgentRegistrationMetadata contains additional metadata for agent registration.
type AgentRegistrationMetadata struct {
	Tags        []string               `json:"tags"`
	Environment string                 `json:"environment"`
	Region      string                 `json:"region"`
	Priority    int                    `json:"priority"`
	Weight      int                    `json:"weight"`
	Custom      map[string]interface{} `json:"custom,omitempty"`
}

// AgentHealthStatus represents the health status of an agent.
type AgentHealthStatus struct {
	Status       AgentHealthStatusType  `json:"status"`
	LastCheck    time.Time              `json:"last_check"`
	ResponseTime time.Duration          `json:"response_time"`
	ErrorCount   int32                  `json:"error_count"`
	Uptime       time.Duration          `json:"uptime"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Errors       []string               `json:"errors,omitempty"`
}

// AgentPerformanceMetrics contains performance metrics for an agent.
type AgentPerformanceMetrics struct {
	// Request metrics
	RequestCount     int64         `json:"request_count"`
	SuccessCount     int64         `json:"success_count"`
	ErrorCount       int32         `json:"error_count"`
	AverageLatency   time.Duration `json:"average_latency"`
	P95Latency       time.Duration `json:"p95_latency"`
	P99Latency       time.Duration `json:"p99_latency"`
	
	// Throughput metrics
	RequestsPerSecond float64 `json:"requests_per_second"`
	BytesPerSecond    float64 `json:"bytes_per_second"`
	
	// Resource utilization
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	
	// Connection metrics
	ActiveConnections int32 `json:"active_connections"`
	MaxConnections    int32 `json:"max_connections"`
	
	// Updated timestamp
	LastUpdated time.Time `json:"last_updated"`
}

// AgentLoadInfo represents current load information for an agent.
type AgentLoadInfo struct {
	CurrentLoad    float64   `json:"current_load"`    // 0.0 to 1.0
	RequestQueue   int32     `json:"request_queue"`   // Number of queued requests
	ProcessingTime time.Duration `json:"processing_time"` // Average processing time
	Capacity       int32     `json:"capacity"`        // Maximum concurrent requests
	LastUpdated    time.Time `json:"last_updated"`
}

// AgentSelectionRequest represents a request for selecting an agent.
type AgentSelectionRequest struct {
	RequiredCapabilities []string               `json:"required_capabilities"`
	PreferredTags        []string               `json:"preferred_tags"`
	ExcludeAgents        []string               `json:"exclude_agents"`
	LoadBalancingHint    LoadBalancingAlgorithm `json:"load_balancing_hint"`
	MaxLatency           time.Duration          `json:"max_latency"`
	MinVersion           string                 `json:"min_version"`
	Context              map[string]interface{} `json:"context,omitempty"`
}

// AgentFilter represents filtering criteria for listing agents.
type AgentFilter struct {
	Status       []AgentRegistrationStatus `json:"status,omitempty"`
	Capabilities []string                  `json:"capabilities,omitempty"`
	Tags         []string                  `json:"tags,omitempty"`
	Environment  string                    `json:"environment,omitempty"`
	Region       string                    `json:"region,omitempty"`
	HealthStatus []AgentHealthStatusType   `json:"health_status,omitempty"`
	MinVersion   string                    `json:"min_version,omitempty"`
}

// DiscoveryQuery represents a service discovery query.
type DiscoveryQuery struct {
	ServiceName      string                 `json:"service_name,omitempty"`
	Capabilities     []string               `json:"capabilities,omitempty"`
	Tags             []string               `json:"tags,omitempty"`
	Environment      string                 `json:"environment,omitempty"`
	Region           string                 `json:"region,omitempty"`
	HealthRequired   bool                   `json:"health_required"`
	MaxResults       int                    `json:"max_results"`
	IncludeMetrics   bool                   `json:"include_metrics"`
	CustomFilters    map[string]interface{} `json:"custom_filters,omitempty"`
}

// DiscoveryResult represents the result of a service discovery query.
type DiscoveryResult struct {
	Agents      []*AgentRegistration `json:"agents"`
	TotalCount  int                  `json:"total_count"`
	QueryTime   time.Duration        `json:"query_time"`
	Timestamp   time.Time            `json:"timestamp"`
}

// AgentChangeEvent represents a change in agent registration.
type AgentChangeEvent struct {
	Type      AgentChangeType      `json:"type"`
	AgentID   string               `json:"agent_id"`
	Agent     *AgentRegistration   `json:"agent,omitempty"`
	OldAgent  *AgentRegistration   `json:"old_agent,omitempty"`
	Timestamp time.Time            `json:"timestamp"`
}

// LoadBalancingStats represents load balancing statistics.
type LoadBalancingStats struct {
	TotalRequests      int64                    `json:"total_requests"`
	RequestsPerAgent   map[string]int64         `json:"requests_per_agent"`
	AverageLatency     time.Duration            `json:"average_latency"`
	LatencyPerAgent    map[string]time.Duration `json:"latency_per_agent"`
	ErrorRate          float64                  `json:"error_rate"`
	ErrorsPerAgent     map[string]int32         `json:"errors_per_agent"`
	Algorithm          string                   `json:"algorithm"`
	LastUpdated        time.Time                `json:"last_updated"`
}

// RegistryStats represents overall registry statistics.
type RegistryStats struct {
	TotalAgents       int32                        `json:"total_agents"`
	HealthyAgents     int32                        `json:"healthy_agents"`
	UnhealthyAgents   int32                        `json:"unhealthy_agents"`
	AgentsByStatus    map[AgentRegistrationStatus]int32 `json:"agents_by_status"`
	AgentsByHealth    map[AgentHealthStatusType]int32   `json:"agents_by_health"`
	CapabilityMatrix  map[string]int32             `json:"capability_matrix"`
	AverageLatency    time.Duration                `json:"average_latency"`
	TotalRequests     int64                        `json:"total_requests"`
	StartTime         time.Time                    `json:"start_time"`
	Uptime            time.Duration                `json:"uptime"`
}

// RegistryConfig contains configuration for the agent registry.
type RegistryConfig struct {
	// Health checking configuration
	HealthCheckInterval      time.Duration `json:"health_check_interval"`
	HealthCheckTimeout       time.Duration `json:"health_check_timeout"`
	UnhealthyThreshold       int32         `json:"unhealthy_threshold"`
	HealthyThreshold         int32         `json:"healthy_threshold"`
	
	// Load balancing configuration
	DefaultLoadBalancingAlgorithm LoadBalancingAlgorithm `json:"default_load_balancing_algorithm"`
	EnableMetricsCollection       bool                   `json:"enable_metrics_collection"`
	MetricsCollectionInterval     time.Duration          `json:"metrics_collection_interval"`
	
	// Registration configuration  
	RegistrationTimeout       time.Duration `json:"registration_timeout"`
	DeregistrationTimeout     time.Duration `json:"deregistration_timeout"`
	MaxAgents                 int32         `json:"max_agents"`
	EnableVersionCompatibility bool         `json:"enable_version_compatibility"`
	
	// Discovery configuration
	MaxDiscoveryResults       int           `json:"max_discovery_results"`
	DiscoveryQueryTimeout     time.Duration `json:"discovery_query_timeout"`
	EnableChangeNotifications bool          `json:"enable_change_notifications"`
	
	// Performance tuning
	EnablePerformanceAwareRouting bool          `json:"enable_performance_aware_routing"`
	PerformanceWindowSize         time.Duration `json:"performance_window_size"`
	LoadBalancingWeight           float64       `json:"load_balancing_weight"`
}

// ==============================================================================
// ENUMS AND CONSTANTS
// ==============================================================================

// AgentRegistrationStatus represents the registration status of an agent.
type AgentRegistrationStatus string

const (
	AgentStatusRegistering    AgentRegistrationStatus = "registering"
	AgentStatusActive         AgentRegistrationStatus = "active"
	AgentStatusDraining       AgentRegistrationStatus = "draining"
	AgentStatusUnhealthy      AgentRegistrationStatus = "unhealthy"
	AgentStatusDeregistering  AgentRegistrationStatus = "deregistering"
	AgentStatusDeregistered   AgentRegistrationStatus = "deregistered"
)

// AgentHealthStatusType represents the health status of an agent.
type AgentHealthStatusType string

const (
	HealthStatusHealthy     AgentHealthStatusType = "healthy"
	HealthStatusUnhealthy   AgentHealthStatusType = "unhealthy"
	HealthStatusUnknown     AgentHealthStatusType = "unknown"
	HealthStatusDegraded    AgentHealthStatusType = "degraded"
	HealthStatusMaintenance AgentHealthStatusType = "maintenance"
)

// AgentChangeType represents the type of agent change event.
type AgentChangeType string

const (
	ChangeTypeRegistered    AgentChangeType = "registered"
	ChangeTypeDeregistered  AgentChangeType = "deregistered"
	ChangeTypeHealthChanged AgentChangeType = "health_changed"
	ChangeTypeMetricsUpdated AgentChangeType = "metrics_updated"
	ChangeTypeCapabilitiesUpdated AgentChangeType = "capabilities_updated"
	ChangeTypeStatusChanged AgentChangeType = "status_changed"
)

// LoadBalancingAlgorithm represents different load balancing algorithms.
type LoadBalancingAlgorithm string

const (
	LoadBalancingRoundRobin     LoadBalancingAlgorithm = "round_robin"
	LoadBalancingWeightedRoundRobin LoadBalancingAlgorithm = "weighted_round_robin"
	LoadBalancingLeastConnections LoadBalancingAlgorithm = "least_connections"
	LoadBalancingWeightedLeastConnections LoadBalancingAlgorithm = "weighted_least_connections"
	LoadBalancingLatencyBased   LoadBalancingAlgorithm = "latency_based"
	LoadBalancingRandom         LoadBalancingAlgorithm = "random"
	LoadBalancingWeightedRandom LoadBalancingAlgorithm = "weighted_random"
	LoadBalancingPerformanceBased LoadBalancingAlgorithm = "performance_based"
)

// ==============================================================================
// IMPLEMENTATION
// ==============================================================================

// DefaultAgentRegistry provides the default implementation of the ExtendedAgentRegistry interface.
type DefaultAgentRegistry struct {
	// Configuration
	config *RegistryConfig
	
	// Agent storage and indexing
	agents           map[string]*AgentRegistration
	agentsByCapability map[string][]*AgentRegistration
	agentsByTag        map[string][]*AgentRegistration
	agentsByStatus     map[AgentRegistrationStatus][]*AgentRegistration
	agentsByHealth     map[AgentHealthStatusType][]*AgentRegistration
	
	// Synchronization
	mu sync.RWMutex
	
	// Health checking
	healthChecker    AgentHealthChecker
	healthCheckStop  chan struct{}
	healthCheckDone  chan struct{}
	
	// Load balancing
	loadBalancer LoadBalancer
	
	// Change notification
	watchers     map[string]AgentWatcher
	watchersMu   sync.RWMutex
	changeEvents chan *AgentChangeEvent
	changeEventsMu sync.RWMutex  // Protects access to changeEvents channel
	changeEventsClosed atomic.Bool
	
	// Statistics and metrics
	stats           *RegistryStats
	loadStats       *LoadBalancingStats
	metricsStop     chan struct{}
	metricsDone     chan struct{}
	
	// Lifecycle management
	startTime            time.Time
	running              atomic.Bool
	shutdownOnce         sync.Once
	healthCheckDoneOnce  sync.Once
	metricsDoneOnce      sync.Once
}

// NewAgentRegistry creates a new agent registry with the given configuration.
func NewAgentRegistry(config *RegistryConfig) ExtendedAgentRegistry {
	if config == nil {
		config = DefaultRegistryConfig()
	}
	
	registry := &DefaultAgentRegistry{
		config:             config,
		agents:             make(map[string]*AgentRegistration),
		agentsByCapability: make(map[string][]*AgentRegistration),
		agentsByTag:        make(map[string][]*AgentRegistration),
		agentsByStatus:     make(map[AgentRegistrationStatus][]*AgentRegistration),
		agentsByHealth:     make(map[AgentHealthStatusType][]*AgentRegistration),
		watchers:           make(map[string]AgentWatcher),
		changeEvents:       make(chan *AgentChangeEvent, 1000),
		healthCheckStop:    make(chan struct{}),
		healthCheckDone:    make(chan struct{}),
		metricsStop:        make(chan struct{}),
		metricsDone:        make(chan struct{}),
		stats:              &RegistryStats{},
		loadStats:          &LoadBalancingStats{
			Algorithm:          string(config.DefaultLoadBalancingAlgorithm),
			RequestsPerAgent:   make(map[string]int64),
			LatencyPerAgent:    make(map[string]time.Duration),
			ErrorsPerAgent:     make(map[string]int32),
			LastUpdated:        time.Now(),
		},
	}
	
	// Initialize components
	registry.healthChecker = NewDefaultHealthChecker(config.HealthCheckInterval)
	registry.loadBalancer = NewLoadBalancer(config.DefaultLoadBalancingAlgorithm)
	
	return registry
}

// Start starts the agent registry and its background processes.
func (r *DefaultAgentRegistry) Start(ctx context.Context) error {
	if r.running.Load() {
		return pkgerrors.NewBaseError("CONFIGURATION_ERROR", "registry already running")
	}
	
	r.startTime = time.Now()
	r.running.Store(true)
	
	// Start health checking
	if r.config.HealthCheckInterval > 0 {
		go r.runHealthChecks()
	}
	
	// Start metrics collection
	if r.config.EnableMetricsCollection && r.config.MetricsCollectionInterval > 0 {
		go r.runMetricsCollection()
	}
	
	// Start change event processing
	go r.processChangeEvents()
	
	return nil
}

// Stop stops the agent registry and cleans up resources.
func (r *DefaultAgentRegistry) Stop(ctx context.Context) error {
	r.shutdownOnce.Do(func() {
		r.running.Store(false)
		
		// Stop health checking
		close(r.healthCheckStop)
		select {
		case <-r.healthCheckDone:
		case <-ctx.Done():
		}
		
		// Stop metrics collection
		close(r.metricsStop)
		select {
		case <-r.metricsDone:
		case <-ctx.Done():
		}
		
		// Close change events channel with proper synchronization
		r.changeEventsMu.Lock()
		if !r.changeEventsClosed.Load() {
			r.changeEventsClosed.Store(true)
			close(r.changeEvents)
		}
		r.changeEventsMu.Unlock()
		
		// Close all watchers
		r.watchersMu.Lock()
		for _, watcher := range r.watchers {
			watcher.Close()
		}
		r.watchers = make(map[string]AgentWatcher)
		r.watchersMu.Unlock()
	})
	
	return nil
}

// RegisterAgent registers a new agent with the registry.
func (r *DefaultAgentRegistry) RegisterAgent(ctx context.Context, agent client.Agent, metadata *AgentRegistrationMetadata) error {
	if !r.running.Load() {
		return pkgerrors.NewBaseError("CONFIGURATION_ERROR", "registry not running")
	}
	
	agentID := agent.Name()
	if agentID == "" {
		return pkgerrors.NewValidationError("VALIDATION_FAILED", "agent name cannot be empty")
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Check if agent already exists
	if _, exists := r.agents[agentID]; exists {
		return pkgerrors.NewConflictError("RESOURCE_CONFLICT", fmt.Sprintf("agent %s already registered", agentID))
	}
	
	// Check agent limit
	if r.config.MaxAgents > 0 && int32(len(r.agents)) >= r.config.MaxAgents {
		return pkgerrors.NewResourceLimitError("maximum number of agents reached", nil)
	}
	
	// Create registration
	registration := &AgentRegistration{
		AgentID:      agentID,
		Agent:        agent,
		Name:         agent.Name(),
		Description:  agent.Description(),
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
		Metadata:     metadata,
		Capabilities: func() *client.AgentCapabilities { c := agent.Capabilities(); return &c }(),
		Health: &AgentHealthStatus{
			Status:    HealthStatusUnknown,
			LastCheck: time.Now(),
		},
		Metrics: &AgentPerformanceMetrics{
			LastUpdated: time.Now(),
		},
		LoadInfo: &AgentLoadInfo{
			Capacity:    100, // Default capacity
			LastUpdated: time.Now(),
		},
		Status: AgentStatusRegistering,
	}
	
	// Update registration status to active before indexing
	registration.Status = AgentStatusActive
	
	// Store the registration
	r.agents[agentID] = registration
	
	// Update indices
	r.updateIndices(registration, nil)
	
	// Send change event
	r.sendChangeEvent(&AgentChangeEvent{
		Type:      ChangeTypeRegistered,
		AgentID:   agentID,
		Agent:     registration,
		Timestamp: time.Now(),
	})
	
	// Trigger initial health check
	go func() {
		// Check if registry is still running before performing health check
		if !r.running.Load() {
			return
		}
		
		if health, err := r.healthChecker.CheckHealth(ctx, agent); err == nil {
			// Double-check registry is still running before updating health
			if r.running.Load() {
				r.UpdateAgentHealth(context.Background(), agentID, health)
			}
		}
	}()
	
	return nil
}

// UnregisterAgent removes an agent from the registry.
func (r *DefaultAgentRegistry) UnregisterAgent(ctx context.Context, agentID string) error {
	if !r.running.Load() {
		return pkgerrors.NewConfigurationError("registry not running", nil)
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	registration, exists := r.agents[agentID]
	if !exists {
		return pkgerrors.NewNotFoundError(fmt.Sprintf("agent %s not found", agentID), nil)
	}
	
	// Update status to deregistering
	oldRegistration := *registration
	registration.Status = AgentStatusDeregistering
	
	// Remove from indices
	r.updateIndices(nil, registration)
	
	// Remove from storage
	delete(r.agents, agentID)
	
	// Send change event
	r.sendChangeEvent(&AgentChangeEvent{
		Type:      ChangeTypeDeregistered,
		AgentID:   agentID,
		OldAgent:  &oldRegistration,
		Timestamp: time.Now(),
	})
	
	return nil
}

// GetAgent retrieves a specific agent registration.
func (r *DefaultAgentRegistry) GetAgent(ctx context.Context, agentID string) (*AgentRegistration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	registration, exists := r.agents[agentID]
	if !exists {
		return nil, pkgerrors.NewNotFoundError(fmt.Sprintf("agent %s not found", agentID), nil)
	}
	
	// Create a copy to avoid data races
	regCopy := *registration
	return &regCopy, nil
}

// ListAgents lists agents based on the provided filter.
func (r *DefaultAgentRegistry) ListAgents(ctx context.Context, filter *AgentFilter) ([]*AgentRegistration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var result []*AgentRegistration
	
	for _, registration := range r.agents {
		if r.matchesFilter(registration, filter) {
			regCopy := *registration
			result = append(result, &regCopy)
		}
	}
	
	// Sort by registration time (newest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].RegisteredAt.After(result[j].RegisteredAt)
	})
	
	return result, nil
}

// UpdateAgentHealth updates the health status of an agent.
func (r *DefaultAgentRegistry) UpdateAgentHealth(ctx context.Context, agentID string, health *AgentHealthStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	registration, exists := r.agents[agentID]
	if !exists {
		return pkgerrors.NewNotFoundError(fmt.Sprintf("agent %s not found", agentID), nil)
	}
	
	oldHealth := registration.Health.Status
	registration.Health = health
	registration.LastSeen = time.Now()
	
	// Update health index
	if oldHealth != health.Status {
		r.removeFromHealthIndex(registration, oldHealth)
		r.addToHealthIndex(registration, health.Status)
		
		// Update registration status based on health
		oldStatus := registration.Status
		if health.Status == HealthStatusHealthy && registration.Status == AgentStatusUnhealthy {
			registration.Status = AgentStatusActive
		} else if health.Status == HealthStatusUnhealthy && registration.Status == AgentStatusActive {
			registration.Status = AgentStatusUnhealthy
		}
		
		// Send change events
		if oldHealth != health.Status {
			r.sendChangeEvent(&AgentChangeEvent{
				Type:      ChangeTypeHealthChanged,
				AgentID:   agentID,
				Agent:     registration,
				Timestamp: time.Now(),
			})
		}
		
		if oldStatus != registration.Status {
			r.sendChangeEvent(&AgentChangeEvent{
				Type:      ChangeTypeStatusChanged,
				AgentID:   agentID,
				Agent:     registration,
				Timestamp: time.Now(),
			})
		}
	}
	
	return nil
}

// GetAgentHealth retrieves the health status of an agent.
func (r *DefaultAgentRegistry) GetAgentHealth(ctx context.Context, agentID string) (*AgentHealthStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	registration, exists := r.agents[agentID]
	if !exists {
		return nil, pkgerrors.NewNotFoundError(fmt.Sprintf("agent %s not found", agentID), nil)
	}
	
	// Create a copy
	healthCopy := *registration.Health
	return &healthCopy, nil
}

// GetHealthyAgents returns all healthy agents that have the specified capabilities.
func (r *DefaultAgentRegistry) GetHealthyAgents(ctx context.Context, capabilities []string) ([]*AgentRegistration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var result []*AgentRegistration
	
	healthyAgents := r.agentsByHealth[HealthStatusHealthy]
	for _, registration := range healthyAgents {
		if r.hasCapabilities(registration, capabilities) {
			regCopy := *registration
			result = append(result, &regCopy)
		}
	}
	
	return result, nil
}

// UpdateAgentCapabilities updates the capabilities of an agent.
func (r *DefaultAgentRegistry) UpdateAgentCapabilities(ctx context.Context, agentID string, capabilities *client.AgentCapabilities) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	registration, exists := r.agents[agentID]
	if !exists {
		return pkgerrors.NewNotFoundError(fmt.Sprintf("agent %s not found", agentID), nil)
	}
	
	// Update capabilities
	oldCapabilities := registration.Capabilities
	registration.Capabilities = capabilities
	
	// Update capability indices
	r.removeFromCapabilityIndex(registration, oldCapabilities.Tools)
	r.addToCapabilityIndex(registration, capabilities.Tools)
	
	// Send change event
	r.sendChangeEvent(&AgentChangeEvent{
		Type:      ChangeTypeCapabilitiesUpdated,
		AgentID:   agentID,
		Agent:     registration,
		Timestamp: time.Now(),
	})
	
	return nil
}

// FindAgentsByCapability finds agents that have a specific capability.
func (r *DefaultAgentRegistry) FindAgentsByCapability(ctx context.Context, capability string) ([]*AgentRegistration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	agents := r.agentsByCapability[capability]
	result := make([]*AgentRegistration, len(agents))
	
	for i, registration := range agents {
		regCopy := *registration
		result[i] = &regCopy
	}
	
	return result, nil
}

// GetCapabilityMatrix returns a matrix of capabilities and the agents that provide them.
func (r *DefaultAgentRegistry) GetCapabilityMatrix(ctx context.Context) (map[string][]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	matrix := make(map[string][]string)
	
	for capability, agents := range r.agentsByCapability {
		agentIDs := make([]string, len(agents))
		for i, agent := range agents {
			agentIDs[i] = agent.AgentID
		}
		matrix[capability] = agentIDs
	}
	
	return matrix, nil
}

// SelectAgent selects an agent based on the selection request using load balancing.
func (r *DefaultAgentRegistry) SelectAgent(ctx context.Context, request *AgentSelectionRequest) (*AgentRegistration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Filter agents based on requirements
	candidates := r.findCandidateAgents(request)
	if len(candidates) == 0 {
		return nil, pkgerrors.NewNotFoundError("no suitable agents found", nil)
	}
	
	// Use load balancer to select agent
	selected, err := r.loadBalancer.SelectAgent(ctx, candidates, request)
	if err != nil {
		return nil, fmt.Errorf("load balancer selection failed: %w", err)
	}
	
	// Update load statistics
	r.updateLoadStats(selected.AgentID)
	
	return selected, nil
}

// GetLoadBalancingStats returns current load balancing statistics.
func (r *DefaultAgentRegistry) GetLoadBalancingStats(ctx context.Context) (*LoadBalancingStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Create a copy
	statsCopy := *r.loadStats
	return &statsCopy, nil
}

// UpdateAgentMetrics updates performance metrics for an agent.
func (r *DefaultAgentRegistry) UpdateAgentMetrics(ctx context.Context, agentID string, metrics *AgentPerformanceMetrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	registration, exists := r.agents[agentID]
	if !exists {
		return pkgerrors.NewNotFoundError(fmt.Sprintf("agent %s not found", agentID), nil)
	}
	
	registration.Metrics = metrics
	
	// Update load balancer with new metrics
	loadInfo := &AgentLoadInfo{
		CurrentLoad:    float64(metrics.ActiveConnections) / float64(registration.LoadInfo.Capacity),
		RequestQueue:   0, // Would be provided by the agent
		ProcessingTime: metrics.AverageLatency,
		Capacity:       registration.LoadInfo.Capacity,
		LastUpdated:    time.Now(),
	}
	
	registration.LoadInfo = loadInfo
	r.loadBalancer.UpdateAgentLoad(ctx, agentID, loadInfo)
	
	// Send change event
	r.sendChangeEvent(&AgentChangeEvent{
		Type:      ChangeTypeMetricsUpdated,
		AgentID:   agentID,
		Agent:     registration,
		Timestamp: time.Now(),
	})
	
	return nil
}

// DiscoverAgents performs service discovery based on the query.
func (r *DefaultAgentRegistry) DiscoverAgents(ctx context.Context, query *DiscoveryQuery) (*DiscoveryResult, error) {
	startTime := time.Now()
	
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var candidates []*AgentRegistration
	
	// Start with all agents if no specific filters
	if len(query.Capabilities) == 0 && len(query.Tags) == 0 {
		for _, registration := range r.agents {
			candidates = append(candidates, registration)
		}
	} else {
		// Filter by capabilities
		if len(query.Capabilities) > 0 {
			candidateMap := make(map[string]*AgentRegistration)
			for _, capability := range query.Capabilities {
				agents := r.agentsByCapability[capability]
				for _, agent := range agents {
					candidateMap[agent.AgentID] = agent
				}
			}
			for _, agent := range candidateMap {
				candidates = append(candidates, agent)
			}
		}
		
		// Filter by tags
		if len(query.Tags) > 0 {
			var tagFiltered []*AgentRegistration
			for _, candidate := range candidates {
				if r.hasAnyTag(candidate, query.Tags) {
					tagFiltered = append(tagFiltered, candidate)
				}
			}
			candidates = tagFiltered
		}
	}
	
	// Apply additional filters
	var filtered []*AgentRegistration
	for _, candidate := range candidates {
		if r.matchesDiscoveryQuery(candidate, query) {
			regCopy := *candidate
			filtered = append(filtered, &regCopy)
		}
	}
	
	// Apply max results limit
	if query.MaxResults > 0 && len(filtered) > query.MaxResults {
		filtered = filtered[:query.MaxResults]
	}
	
	result := &DiscoveryResult{
		Agents:     filtered,
		TotalCount: len(filtered),
		QueryTime:  time.Since(startTime),
		Timestamp:  time.Now(),
	}
	
	return result, nil
}

// WatchAgentChanges returns a channel for receiving agent change events.
func (r *DefaultAgentRegistry) WatchAgentChanges(ctx context.Context) (<-chan *AgentChangeEvent, error) {
	if !r.config.EnableChangeNotifications {
		return nil, pkgerrors.NewConfigurationError("change notifications not enabled", nil)
	}
	
	// Create a buffered channel for the watcher
	watcherChan := make(chan *AgentChangeEvent, 100)
	
	// Generate watcher ID
	watcherID := fmt.Sprintf("watcher-%d", time.Now().UnixNano())
	
	// Create and register watcher
	watcher := &defaultAgentWatcher{
		id:      watcherID,
		channel: watcherChan,
		ctx:     ctx,
	}
	
	r.watchersMu.Lock()
	r.watchers[watcherID] = watcher
	r.watchersMu.Unlock()
	
	// Clean up watcher when context is done
	go func() {
		<-ctx.Done()
		r.watchersMu.Lock()
		delete(r.watchers, watcherID)
		r.watchersMu.Unlock()
		close(watcherChan)
	}()
	
	return watcherChan, nil
}

// GetRegistryStats returns overall registry statistics.
func (r *DefaultAgentRegistry) GetRegistryStats(ctx context.Context) (*RegistryStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	stats := &RegistryStats{
		TotalAgents:      int32(len(r.agents)),
		AgentsByStatus:   make(map[AgentRegistrationStatus]int32),
		AgentsByHealth:   make(map[AgentHealthStatusType]int32),
		CapabilityMatrix: make(map[string]int32),
		StartTime:        r.startTime,
		Uptime:           time.Since(r.startTime),
	}
	
	// Count agents by status and health
	for _, registration := range r.agents {
		stats.AgentsByStatus[registration.Status]++
		stats.AgentsByHealth[registration.Health.Status]++
		
		if registration.Health.Status == HealthStatusHealthy {
			stats.HealthyAgents++
		} else {
			stats.UnhealthyAgents++
		}
	}
	
	// Count capabilities
	for capability, agents := range r.agentsByCapability {
		stats.CapabilityMatrix[capability] = int32(len(agents))
	}
	
	// Copy load balancing stats
	if r.loadStats != nil {
		stats.AverageLatency = r.loadStats.AverageLatency
		stats.TotalRequests = r.loadStats.TotalRequests
	}
	
	return stats, nil
}

// ==============================================================================
// HELPER METHODS
// ==============================================================================

// updateIndices updates the internal indices when an agent is added or removed.
func (r *DefaultAgentRegistry) updateIndices(newAgent, oldAgent *AgentRegistration) {
	if oldAgent != nil {
		// Remove from indices
		if oldAgent.Capabilities != nil {
			r.removeFromCapabilityIndex(oldAgent, oldAgent.Capabilities.Tools)
		}
		if oldAgent.Metadata != nil {
			r.removeFromTagIndex(oldAgent, oldAgent.Metadata.Tags)
		}
		r.removeFromStatusIndex(oldAgent, oldAgent.Status)
		r.removeFromHealthIndex(oldAgent, oldAgent.Health.Status)
	}
	
	if newAgent != nil {
		// Add to indices
		if newAgent.Capabilities != nil {
			r.addToCapabilityIndex(newAgent, newAgent.Capabilities.Tools)
		}
		if newAgent.Metadata != nil {
			r.addToTagIndex(newAgent, newAgent.Metadata.Tags)
		}
		r.addToStatusIndex(newAgent, newAgent.Status)
		r.addToHealthIndex(newAgent, newAgent.Health.Status)
	}
}

// addToCapabilityIndex adds an agent to the capability index.
func (r *DefaultAgentRegistry) addToCapabilityIndex(agent *AgentRegistration, capabilities []string) {
	for _, capability := range capabilities {
		if r.agentsByCapability[capability] == nil {
			r.agentsByCapability[capability] = make([]*AgentRegistration, 0)
		}
		r.agentsByCapability[capability] = append(r.agentsByCapability[capability], agent)
	}
}

// removeFromCapabilityIndex removes an agent from the capability index.
func (r *DefaultAgentRegistry) removeFromCapabilityIndex(agent *AgentRegistration, capabilities []string) {
	for _, capability := range capabilities {
		agents := r.agentsByCapability[capability]
		for i, a := range agents {
			if a.AgentID == agent.AgentID {
				r.agentsByCapability[capability] = append(agents[:i], agents[i+1:]...)
				break
			}
		}
		if len(r.agentsByCapability[capability]) == 0 {
			delete(r.agentsByCapability, capability)
		}
	}
}

// addToTagIndex adds an agent to the tag index.
func (r *DefaultAgentRegistry) addToTagIndex(agent *AgentRegistration, tags []string) {
	for _, tag := range tags {
		if r.agentsByTag[tag] == nil {
			r.agentsByTag[tag] = make([]*AgentRegistration, 0)
		}
		r.agentsByTag[tag] = append(r.agentsByTag[tag], agent)
	}
}

// removeFromTagIndex removes an agent from the tag index.
func (r *DefaultAgentRegistry) removeFromTagIndex(agent *AgentRegistration, tags []string) {
	for _, tag := range tags {
		agents := r.agentsByTag[tag]
		for i, a := range agents {
			if a.AgentID == agent.AgentID {
				r.agentsByTag[tag] = append(agents[:i], agents[i+1:]...)
				break
			}
		}
		if len(r.agentsByTag[tag]) == 0 {
			delete(r.agentsByTag, tag)
		}
	}
}

// addToStatusIndex adds an agent to the status index.
func (r *DefaultAgentRegistry) addToStatusIndex(agent *AgentRegistration, status AgentRegistrationStatus) {
	if r.agentsByStatus[status] == nil {
		r.agentsByStatus[status] = make([]*AgentRegistration, 0)
	}
	r.agentsByStatus[status] = append(r.agentsByStatus[status], agent)
}

// removeFromStatusIndex removes an agent from the status index.
func (r *DefaultAgentRegistry) removeFromStatusIndex(agent *AgentRegistration, status AgentRegistrationStatus) {
	agents := r.agentsByStatus[status]
	for i, a := range agents {
		if a.AgentID == agent.AgentID {
			r.agentsByStatus[status] = append(agents[:i], agents[i+1:]...)
			break
		}
	}
	if len(r.agentsByStatus[status]) == 0 {
		delete(r.agentsByStatus, status)
	}
}

// addToHealthIndex adds an agent to the health index.
func (r *DefaultAgentRegistry) addToHealthIndex(agent *AgentRegistration, health AgentHealthStatusType) {
	if r.agentsByHealth[health] == nil {
		r.agentsByHealth[health] = make([]*AgentRegistration, 0)
	}
	r.agentsByHealth[health] = append(r.agentsByHealth[health], agent)
}

// removeFromHealthIndex removes an agent from the health index.
func (r *DefaultAgentRegistry) removeFromHealthIndex(agent *AgentRegistration, health AgentHealthStatusType) {
	agents := r.agentsByHealth[health]
	for i, a := range agents {
		if a.AgentID == agent.AgentID {
			r.agentsByHealth[health] = append(agents[:i], agents[i+1:]...)
			break
		}
	}
	if len(r.agentsByHealth[health]) == 0 {
		delete(r.agentsByHealth, health)
	}
}

// matchesFilter checks if an agent registration matches the given filter.
func (r *DefaultAgentRegistry) matchesFilter(registration *AgentRegistration, filter *AgentFilter) bool {
	if filter == nil {
		return true
	}
	
	// Check status filter
	if len(filter.Status) > 0 {
		statusMatch := false
		for _, status := range filter.Status {
			if registration.Status == status {
				statusMatch = true
				break
			}
		}
		if !statusMatch {
			return false
		}
	}
	
	// Check capabilities filter
	if len(filter.Capabilities) > 0 && !r.hasCapabilities(registration, filter.Capabilities) {
		return false
	}
	
	// Check tags filter
	if len(filter.Tags) > 0 && !r.hasAnyTag(registration, filter.Tags) {
		return false
	}
	
	// Check environment filter
	if filter.Environment != "" && (registration.Metadata == nil || registration.Metadata.Environment != filter.Environment) {
		return false
	}
	
	// Check region filter
	if filter.Region != "" && (registration.Metadata == nil || registration.Metadata.Region != filter.Region) {
		return false
	}
	
	// Check health status filter
	if len(filter.HealthStatus) > 0 {
		healthMatch := false
		for _, health := range filter.HealthStatus {
			if registration.Health.Status == health {
				healthMatch = true
				break
			}
		}
		if !healthMatch {
			return false
		}
	}
	
	return true
}

// hasCapabilities checks if an agent has all the required capabilities.
func (r *DefaultAgentRegistry) hasCapabilities(registration *AgentRegistration, requiredCapabilities []string) bool {
	if registration.Capabilities == nil {
		return len(requiredCapabilities) == 0
	}
	
	agentCapabilities := make(map[string]bool)
	for _, capability := range registration.Capabilities.Tools {
		agentCapabilities[capability] = true
	}
	
	for _, required := range requiredCapabilities {
		if !agentCapabilities[required] {
			return false
		}
	}
	
	return true
}

// hasAnyTag checks if an agent has any of the specified tags.
func (r *DefaultAgentRegistry) hasAnyTag(registration *AgentRegistration, tags []string) bool {
	if registration.Metadata == nil || len(registration.Metadata.Tags) == 0 {
		return false
	}
	
	agentTags := make(map[string]bool)
	for _, tag := range registration.Metadata.Tags {
		agentTags[tag] = true
	}
	
	for _, tag := range tags {
		if agentTags[tag] {
			return true
		}
	}
	
	return false
}

// findCandidateAgents finds agents that match the selection request.
func (r *DefaultAgentRegistry) findCandidateAgents(request *AgentSelectionRequest) []*AgentRegistration {
	var candidates []*AgentRegistration
	
	// Collect all eligible agents from different health states, prioritizing healthy ones
	candidateAgents := make([]*AgentRegistration, 0)
	
	// First, add healthy agents (highest priority)
	if healthyAgents := r.agentsByHealth[HealthStatusHealthy]; healthyAgents != nil {
		candidateAgents = append(candidateAgents, healthyAgents...)
	}
	
	// Then add unknown health agents (newly registered agents that haven't been health-checked yet)
	if unknownHealthAgents := r.agentsByHealth[HealthStatusUnknown]; unknownHealthAgents != nil {
		for _, agent := range unknownHealthAgents {
			// Only include active agents
			if agent.Status == AgentStatusActive {
				candidateAgents = append(candidateAgents, agent)
			}
		}
	}
	
	// If still no candidates, also consider degraded agents (as fallback)
	if len(candidateAgents) == 0 {
		if degradedAgents := r.agentsByHealth[HealthStatusDegraded]; degradedAgents != nil {
			for _, agent := range degradedAgents {
				// Only include active agents
				if agent.Status == AgentStatusActive {
					candidateAgents = append(candidateAgents, agent)
				}
			}
		}
	}
	
	for _, agent := range candidateAgents {
		// Check required capabilities
		if len(request.RequiredCapabilities) > 0 && !r.hasCapabilities(agent, request.RequiredCapabilities) {
			continue
		}
		
		// Check excluded agents
		excluded := false
		for _, excludeID := range request.ExcludeAgents {
			if agent.AgentID == excludeID {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		
		// Check preferred tags (optional)
		if len(request.PreferredTags) > 0 && !r.hasAnyTag(agent, request.PreferredTags) {
			// Don't exclude, just deprioritize
		}
		
		// Check max latency
		if request.MaxLatency > 0 && agent.Metrics != nil && agent.Metrics.AverageLatency > request.MaxLatency {
			continue
		}
		
		candidates = append(candidates, agent)
	}
	
	return candidates
}

// matchesDiscoveryQuery checks if an agent matches the discovery query.
func (r *DefaultAgentRegistry) matchesDiscoveryQuery(registration *AgentRegistration, query *DiscoveryQuery) bool {
	// Check health requirement
	if query.HealthRequired && registration.Health.Status != HealthStatusHealthy {
		return false
	}
	
	// Check environment
	if query.Environment != "" && (registration.Metadata == nil || registration.Metadata.Environment != query.Environment) {
		return false
	}
	
	// Check region
	if query.Region != "" && (registration.Metadata == nil || registration.Metadata.Region != query.Region) {
		return false
	}
	
	return true
}

// sendChangeEvent sends a change event to all registered watchers.
func (r *DefaultAgentRegistry) sendChangeEvent(event *AgentChangeEvent) {
	// Check if registry is running before attempting to send
	if !r.running.Load() || r.changeEventsClosed.Load() {
		return
	}
	
	// Use read lock to protect channel access
	r.changeEventsMu.RLock()
	defer r.changeEventsMu.RUnlock()
	
	// Double-check if channel is closed after acquiring lock
	if r.changeEventsClosed.Load() {
		return
	}
	
	// Use defer to recover from potential "send on closed channel" panic
	defer func() {
		if r := recover(); r != nil {
			// Channel was closed between the checks and send attempt
			// This is expected during shutdown, so we ignore the panic
		}
	}()
	
	select {
	case r.changeEvents <- event:
		// Successfully sent
	default:
		// Channel is full, skip event
	}
}

// processChangeEvents processes change events and distributes them to watchers.
func (r *DefaultAgentRegistry) processChangeEvents() {
	for event := range r.changeEvents {
		r.watchersMu.RLock()
		for _, watcher := range r.watchers {
			select {
			case watcher.(*defaultAgentWatcher).channel <- event:
			default:
				// Watcher channel is full, skip
			}
		}
		r.watchersMu.RUnlock()
	}
}

// updateLoadStats updates load balancing statistics.
func (r *DefaultAgentRegistry) updateLoadStats(agentID string) {
	if r.loadStats.RequestsPerAgent == nil {
		r.loadStats.RequestsPerAgent = make(map[string]int64)
	}
	
	r.loadStats.TotalRequests++
	r.loadStats.RequestsPerAgent[agentID]++
	r.loadStats.Algorithm = r.loadBalancer.GetAlgorithm()
	r.loadStats.LastUpdated = time.Now()
}

// runHealthChecks runs periodic health checks on all registered agents.
func (r *DefaultAgentRegistry) runHealthChecks() {
	defer r.healthCheckDoneOnce.Do(func() {
		close(r.healthCheckDone)
	})
	
	ticker := time.NewTicker(r.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-r.healthCheckStop:
			return
		case <-ticker.C:
			r.performHealthChecks()
		}
	}
}

// performHealthChecks performs health checks on all agents.
func (r *DefaultAgentRegistry) performHealthChecks() {
	r.mu.RLock()
	agents := make([]*AgentRegistration, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}
	r.mu.RUnlock()
	
	for _, registration := range agents {
		go func(reg *AgentRegistration) {
			ctx, cancel := context.WithTimeout(context.Background(), r.config.HealthCheckTimeout)
			defer cancel()
			
			// Check if registry is still running before health check
			if !r.running.Load() {
				return
			}
			
			health, err := r.healthChecker.CheckHealth(ctx, reg.Agent)
			if err != nil {
				// Health check failed
				health = &AgentHealthStatus{
					Status:       HealthStatusUnhealthy,
					LastCheck:    time.Now(),
					ResponseTime: r.config.HealthCheckTimeout,
					ErrorCount:   atomic.AddInt32(&reg.FailureCount, 1),
					Errors:       []string{err.Error()},
				}
			} else {
				// Reset failure count on successful health check
				atomic.StoreInt32(&reg.FailureCount, 0)
			}
			
			// Double-check registry is still running before updating health
			if r.running.Load() {
				r.UpdateAgentHealth(context.Background(), reg.AgentID, health)
			}
		}(registration)
	}
}

// runMetricsCollection runs periodic metrics collection.
func (r *DefaultAgentRegistry) runMetricsCollection() {
	defer r.metricsDoneOnce.Do(func() {
		close(r.metricsDone)
	})
	
	ticker := time.NewTicker(r.config.MetricsCollectionInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-r.metricsStop:
			return
		case <-ticker.C:
			r.collectMetrics()
		}
	}
}

// collectMetrics collects performance metrics from all agents.
func (r *DefaultAgentRegistry) collectMetrics() {
	// Implementation would collect metrics from agents
	// This is a placeholder for the actual metrics collection logic
}

// ==============================================================================
// DEFAULT IMPLEMENTATIONS
// ==============================================================================

// DefaultHealthChecker provides a basic health checker implementation.
type DefaultHealthChecker struct {
	interval time.Duration
}

// NewDefaultHealthChecker creates a new default health checker.
func NewDefaultHealthChecker(interval time.Duration) AgentHealthChecker {
	return &DefaultHealthChecker{
		interval: interval,
	}
}

// CheckHealth performs a basic health check on an agent.
func (hc *DefaultHealthChecker) CheckHealth(ctx context.Context, agent client.Agent) (*AgentHealthStatus, error) {
	startTime := time.Now()
	
	// Get agent health from the agent itself
	health := agent.Health()
	
	// Convert to our health status format
	status := &AgentHealthStatus{
		Status:       HealthStatusHealthy, // Default to healthy
		LastCheck:    time.Now(),
		ResponseTime: time.Since(startTime),
		ErrorCount:   0,
		Details:      health.Details,
		Errors:       health.Errors,
	}
	
	// Map agent health status to registry health status
	switch health.Status {
	case "healthy":
		status.Status = HealthStatusHealthy
	case "unhealthy":
		status.Status = HealthStatusUnhealthy
	case "degraded":
		status.Status = HealthStatusDegraded
	default:
		status.Status = HealthStatusUnknown
	}
	
	if len(health.Errors) > 0 {
		status.Status = HealthStatusUnhealthy
		return status, fmt.Errorf("agent health check failed: %v", health.Errors)
	}
	
	return status, nil
}

// GetHealthCheckInterval returns the health check interval.
func (hc *DefaultHealthChecker) GetHealthCheckInterval() time.Duration {
	return hc.interval
}

// SetHealthCheckInterval sets the health check interval.
func (hc *DefaultHealthChecker) SetHealthCheckInterval(interval time.Duration) {
	hc.interval = interval
}

// DefaultLoadBalancer provides basic load balancing implementations.
type DefaultLoadBalancer struct {
	algorithm     LoadBalancingAlgorithm
	roundRobinIdx atomic.Uint64
	agentLoads    sync.Map // map[string]*AgentLoadInfo
}

// NewLoadBalancer creates a new load balancer with the specified algorithm.
func NewLoadBalancer(algorithm LoadBalancingAlgorithm) LoadBalancer {
	return &DefaultLoadBalancer{
		algorithm: algorithm,
	}
}

// SelectAgent selects an agent using the configured load balancing algorithm.
func (lb *DefaultLoadBalancer) SelectAgent(ctx context.Context, agents []*AgentRegistration, request *AgentSelectionRequest) (*AgentRegistration, error) {
	if len(agents) == 0 {
		return nil, fmt.Errorf("no agents available")
	}
	
	if len(agents) == 1 {
		return agents[0], nil
	}
	
	algorithm := lb.algorithm
	if request.LoadBalancingHint != "" {
		algorithm = request.LoadBalancingHint
	}
	
	switch algorithm {
	case LoadBalancingRoundRobin:
		idx := lb.roundRobinIdx.Add(1) % uint64(len(agents))
		return agents[idx], nil
		
	case LoadBalancingRandom:
		idx := rand.Intn(len(agents))
		return agents[idx], nil
		
	case LoadBalancingLeastConnections:
		return lb.selectLeastConnections(agents), nil
		
	case LoadBalancingLatencyBased:
		return lb.selectLowestLatency(agents), nil
		
	case LoadBalancingPerformanceBased:
		return lb.selectBestPerformance(agents), nil
		
	default:
		// Default to round robin
		idx := lb.roundRobinIdx.Add(1) % uint64(len(agents))
		return agents[idx], nil
	}
}

// selectLeastConnections selects the agent with the least connections.
func (lb *DefaultLoadBalancer) selectLeastConnections(agents []*AgentRegistration) *AgentRegistration {
	var best *AgentRegistration
	minConnections := int32(-1)
	
	for _, agent := range agents {
		connections := agent.Metrics.ActiveConnections
		if minConnections == -1 || connections < minConnections {
			minConnections = connections
			best = agent
		}
	}
	
	return best
}

// selectLowestLatency selects the agent with the lowest latency.
func (lb *DefaultLoadBalancer) selectLowestLatency(agents []*AgentRegistration) *AgentRegistration {
	var best *AgentRegistration
	var minLatency time.Duration = -1
	
	for _, agent := range agents {
		latency := agent.Metrics.AverageLatency
		if minLatency == -1 || latency < minLatency {
			minLatency = latency
			best = agent
		}
	}
	
	return best
}

// selectBestPerformance selects the agent with the best overall performance.
func (lb *DefaultLoadBalancer) selectBestPerformance(agents []*AgentRegistration) *AgentRegistration {
	var best *AgentRegistration
	bestScore := float64(-1)
	
	for _, agent := range agents {
		// Calculate performance score based on multiple metrics
		score := lb.calculatePerformanceScore(agent)
		if bestScore == -1 || score > bestScore {
			bestScore = score
			best = agent
		}
	}
	
	return best
}

// calculatePerformanceScore calculates a performance score for an agent.
func (lb *DefaultLoadBalancer) calculatePerformanceScore(agent *AgentRegistration) float64 {
	metrics := agent.Metrics
	loadInfo := agent.LoadInfo
	
	// Base score starts at 100
	score := 100.0
	
	// Reduce score based on current load (0-50 point reduction)
	score -= loadInfo.CurrentLoad * 50
	
	// Reduce score based on error rate (0-30 point reduction)
	if metrics.RequestCount > 0 {
		errorRate := float64(metrics.ErrorCount) / float64(metrics.RequestCount)
		score -= errorRate * 30
	}
	
	// Reduce score based on latency (0-20 point reduction)
	if metrics.AverageLatency > 0 {
		latencyMs := float64(metrics.AverageLatency.Milliseconds())
		if latencyMs > 1000 { // > 1 second
			score -= 20
		} else if latencyMs > 500 { // > 500ms
			score -= 10
		} else if latencyMs > 100 { // > 100ms
			score -= 5
		}
	}
	
	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}
	
	return score
}

// UpdateAgentLoad updates load information for an agent.
func (lb *DefaultLoadBalancer) UpdateAgentLoad(ctx context.Context, agentID string, load *AgentLoadInfo) error {
	lb.agentLoads.Store(agentID, load)
	return nil
}

// GetAlgorithm returns the load balancing algorithm name.
func (lb *DefaultLoadBalancer) GetAlgorithm() string {
	return string(lb.algorithm)
}

// defaultAgentWatcher provides a basic agent watcher implementation.
type defaultAgentWatcher struct {
	id      string
	channel chan *AgentChangeEvent
	ctx     context.Context
	closed  atomic.Bool
}

// Watch returns the event channel.
func (w *defaultAgentWatcher) Watch(ctx context.Context) (<-chan *AgentChangeEvent, error) {
	return w.channel, nil
}

// Close closes the watcher.
func (w *defaultAgentWatcher) Close() error {
	if w.closed.CompareAndSwap(false, true) {
		close(w.channel)
	}
	return nil
}

// DefaultRegistryConfig returns a default registry configuration.
func DefaultRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		HealthCheckInterval:               30 * time.Second,
		HealthCheckTimeout:                5 * time.Second,
		UnhealthyThreshold:                3,
		HealthyThreshold:                  2,
		DefaultLoadBalancingAlgorithm:     LoadBalancingRoundRobin,
		EnableMetricsCollection:           true,
		MetricsCollectionInterval:         60 * time.Second,
		RegistrationTimeout:               30 * time.Second,
		DeregistrationTimeout:             10 * time.Second,
		MaxAgents:                         1000,
		EnableVersionCompatibility:        true,
		MaxDiscoveryResults:               100,
		DiscoveryQueryTimeout:             10 * time.Second,
		EnableChangeNotifications:         true,
		EnablePerformanceAwareRouting:     true,
		PerformanceWindowSize:             5 * time.Minute,
		LoadBalancingWeight:               1.0,
	}
}