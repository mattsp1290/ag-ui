package client

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// BaseAgent provides a common implementation of the Agent interface that can be
// embedded by specific agent implementations.
type BaseAgent struct {
	// Configuration and identity
	config *AgentConfig
	name   string
	desc   string
	
	// Lifecycle management
	status     atomic.Value // AgentStatus
	mu         sync.RWMutex
	startTime  time.Time
	
	// Event processing
	eventStream chan events.Event
	streamMu    sync.RWMutex
	
	// Metrics and monitoring
	metrics      AgentMetrics
	metricsMu    sync.RWMutex
	healthStatus HealthStatus
	healthMu     sync.RWMutex
}

// AgentMetrics contains performance and operational metrics for an agent.
type AgentMetrics struct {
	EventsProcessed       int64         `json:"events_processed"`
	EventsPerSecond       float64       `json:"events_per_second"`
	AverageProcessingTime time.Duration `json:"average_processing_time"`
	ToolsExecuted         int64         `json:"tools_executed"`
	StateUpdates          int64         `json:"state_updates"`
	ErrorCount            int64         `json:"error_count"`
	MemoryUsage           int64         `json:"memory_usage"`
	StartTime             time.Time     `json:"start_time"`
	LastActivity          time.Time     `json:"last_activity"`
}

// NewBaseAgent creates a new base agent with default configuration.
func NewBaseAgent(name, description string) *BaseAgent {
	agent := &BaseAgent{
		name: name,
		desc: description,
		metrics: AgentMetrics{
			StartTime: time.Now(),
		},
		healthStatus: HealthStatus{
			Status:    "uninitialized",
			LastCheck: time.Now(),
			Details:   make(map[string]interface{}),
			Errors:    make([]string, 0),
		},
	}
	
	agent.status.Store(AgentStatusUninitialized)
	return agent
}

// Initialize prepares the agent with the given configuration.
func (a *BaseAgent) Initialize(ctx context.Context, config *AgentConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if a.getStatus() != AgentStatusUninitialized {
		return &errors.AgentError{
			Type:    errors.ErrorTypeInvalidState,
			Message: fmt.Sprintf("agent %s is already initialized", a.name),
			Agent:   a.name,
		}
	}
	
	// Validate configuration
	if err := a.validateConfig(config); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	
	// Set configuration with defaults
	a.config = a.mergeWithDefaults(config)
	
	// Initialize event stream
	a.eventStream = make(chan events.Event, a.config.EventProcessing.BufferSize)
	
	// Update status and health
	a.setStatus(AgentStatusInitialized)
	a.updateHealth("initialized", nil)
	
	return nil
}

// Start begins the agent's operation.
func (a *BaseAgent) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	status := a.getStatus()
	if status != AgentStatusInitialized && status != AgentStatusStopped {
		return &errors.AgentError{
			Type:    errors.ErrorTypeInvalidState,
			Message: fmt.Sprintf("agent %s cannot be started from status %s", a.name, status),
			Agent:   a.name,
		}
	}
	
	a.setStatus(AgentStatusStarting)
	
	// Update status and metrics
	a.setStatus(AgentStatusRunning)
	a.startTime = time.Now()
	a.updateHealth("healthy", nil)
	
	return nil
}

// Stop gracefully shuts down the agent.
func (a *BaseAgent) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if a.getStatus() != AgentStatusRunning {
		return &errors.AgentError{
			Type:    errors.ErrorTypeInvalidState,
			Message: fmt.Sprintf("agent %s is not running", a.name),
			Agent:   a.name,
		}
	}
	
	a.setStatus(AgentStatusStopping)
	
	// Close event streams
	if a.eventStream != nil {
		close(a.eventStream)
		a.eventStream = nil
	}
	
	a.setStatus(AgentStatusStopped)
	a.updateHealth("stopped", nil)
	
	return nil
}

// Cleanup releases all resources held by the agent.
func (a *BaseAgent) Cleanup() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Reset state
	a.config = nil
	
	return nil
}

// ProcessEvent handles a single incoming event and returns response events.
func (a *BaseAgent) ProcessEvent(ctx context.Context, event events.Event) ([]events.Event, error) {
	if a.getStatus() != AgentStatusRunning {
		return nil, &errors.AgentError{
			Type:    errors.ErrorTypeInvalidState,
			Message: fmt.Sprintf("agent %s is not running", a.name),
			Agent:   a.name,
		}
	}
	
	// Update metrics
	a.incrementEventsProcessed()
	startTime := time.Now()
	
	defer func() {
		processingTime := time.Since(startTime)
		a.updateAverageProcessingTime(processingTime)
		a.updateLastActivity()
	}()
	
	// Basic event processing implementation
	// In a real implementation, this would delegate to specialized processors
	return []events.Event{}, nil
}

// StreamEvents returns a channel for receiving events from the agent.
func (a *BaseAgent) StreamEvents(ctx context.Context) (<-chan events.Event, error) {
	if a.getStatus() != AgentStatusRunning {
		return nil, &errors.AgentError{
			Type:    errors.ErrorTypeInvalidState,
			Message: fmt.Sprintf("agent %s is not running", a.name),
			Agent:   a.name,
		}
	}
	
	if !a.config.Capabilities.Streaming {
		return nil, &errors.AgentError{
			Type:    errors.ErrorTypeUnsupported,
			Message: fmt.Sprintf("agent %s does not support streaming", a.name),
			Agent:   a.name,
		}
	}
	
	return a.eventStream, nil
}

// GetState returns the current state of the agent.
func (a *BaseAgent) GetState(ctx context.Context) (interface{}, error) {
	// Simplified implementation
	return map[string]interface{}{
		"status": a.getStatus(),
		"name":   a.name,
	}, nil
}

// UpdateState applies a state change delta to the agent's state.
func (a *BaseAgent) UpdateState(ctx context.Context, delta interface{}) error {
	a.incrementStateUpdates()
	return nil
}

// ExecuteTool executes a tool with the given name and parameters.
func (a *BaseAgent) ExecuteTool(ctx context.Context, name string, params interface{}) (interface{}, error) {
	a.incrementToolsExecuted()
	return nil, fmt.Errorf("tool execution not implemented in base agent")
}

// ListTools returns a list of tools available to this agent.
func (a *BaseAgent) ListTools() []ToolDefinition {
	return []ToolDefinition{}
}

// Name returns the unique identifier for this agent instance.
func (a *BaseAgent) Name() string {
	return a.name
}

// Description returns a human-readable description of the agent's purpose.
func (a *BaseAgent) Description() string {
	return a.desc
}

// Capabilities returns information about what this agent can do.
func (a *BaseAgent) Capabilities() AgentCapabilities {
	if a.config == nil {
		return AgentCapabilities{}
	}
	return a.config.Capabilities
}

// Health returns the current health status of the agent.
func (a *BaseAgent) Health() HealthStatus {
	a.healthMu.RLock()
	defer a.healthMu.RUnlock()
	
	// Update health details with current metrics
	health := a.healthStatus
	health.Details["status"] = a.getStatus()
	health.Details["uptime"] = time.Since(a.startTime).String()
	health.Details["events_processed"] = a.getEventsProcessed()
	health.Details["error_count"] = a.getErrorCount()
	
	return health
}

// Helper methods

func (a *BaseAgent) getStatus() AgentStatus {
	return a.status.Load().(AgentStatus)
}

func (a *BaseAgent) setStatus(status AgentStatus) {
	a.status.Store(status)
}

func (a *BaseAgent) updateHealth(status string, errors []string) {
	a.healthMu.Lock()
	defer a.healthMu.Unlock()
	
	a.healthStatus.Status = status
	a.healthStatus.LastCheck = time.Now()
	if errors != nil {
		a.healthStatus.Errors = errors
	}
}

func (a *BaseAgent) incrementEventsProcessed() {
	atomic.AddInt64(&a.metrics.EventsProcessed, 1)
}

func (a *BaseAgent) incrementErrorCount() {
	atomic.AddInt64(&a.metrics.ErrorCount, 1)
}

func (a *BaseAgent) incrementStateUpdates() {
	atomic.AddInt64(&a.metrics.StateUpdates, 1)
}

func (a *BaseAgent) incrementToolsExecuted() {
	atomic.AddInt64(&a.metrics.ToolsExecuted, 1)
}

func (a *BaseAgent) getEventsProcessed() int64 {
	return atomic.LoadInt64(&a.metrics.EventsProcessed)
}

func (a *BaseAgent) getErrorCount() int64 {
	return atomic.LoadInt64(&a.metrics.ErrorCount)
}

func (a *BaseAgent) updateAverageProcessingTime(duration time.Duration) {
	a.metricsMu.Lock()
	defer a.metricsMu.Unlock()
	
	if a.metrics.AverageProcessingTime == 0 {
		a.metrics.AverageProcessingTime = duration
	} else {
		a.metrics.AverageProcessingTime = (a.metrics.AverageProcessingTime + duration) / 2
	}
}

func (a *BaseAgent) updateLastActivity() {
	a.metricsMu.Lock()
	defer a.metricsMu.Unlock()
	a.metrics.LastActivity = time.Now()
}

func (a *BaseAgent) validateConfig(config *AgentConfig) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}
	
	if config.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	
	if config.EventProcessing.BufferSize <= 0 {
		return fmt.Errorf("event processing buffer size must be positive")
	}
	
	if config.EventProcessing.BatchSize <= 0 {
		return fmt.Errorf("event processing batch size must be positive")
	}
	
	if config.Tools.MaxConcurrent <= 0 {
		return fmt.Errorf("tool max concurrent must be positive")
	}
	
	if config.History.MaxMessages <= 0 {
		return fmt.Errorf("history max messages must be positive")
	}
	
	return nil
}

func (a *BaseAgent) mergeWithDefaults(config *AgentConfig) *AgentConfig {
	defaults := DefaultAgentConfig()
	
	// Merge configuration with defaults
	merged := *config
	
	if merged.EventProcessing.BufferSize == 0 {
		merged.EventProcessing.BufferSize = defaults.EventProcessing.BufferSize
	}
	
	if merged.EventProcessing.BatchSize == 0 {
		merged.EventProcessing.BatchSize = defaults.EventProcessing.BatchSize
	}
	
	if merged.EventProcessing.Timeout == 0 {
		merged.EventProcessing.Timeout = defaults.EventProcessing.Timeout
	}
	
	if merged.State.SyncInterval == 0 {
		merged.State.SyncInterval = defaults.State.SyncInterval
	}
	
	if merged.State.CacheSize == "" {
		merged.State.CacheSize = defaults.State.CacheSize
	}
	
	if merged.Tools.Timeout == 0 {
		merged.Tools.Timeout = defaults.Tools.Timeout
	}
	
	if merged.Tools.MaxConcurrent == 0 {
		merged.Tools.MaxConcurrent = defaults.Tools.MaxConcurrent
	}
	
	if merged.History.MaxMessages == 0 {
		merged.History.MaxMessages = defaults.History.MaxMessages
	}
	
	if merged.History.Retention == 0 {
		merged.History.Retention = defaults.History.Retention
	}
	
	if merged.Custom == nil {
		merged.Custom = make(map[string]interface{})
	}
	
	return &merged
}