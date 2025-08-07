package client

import (
	"context"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
)

// LifecycleManager handles agent lifecycle operations including initialization,
// startup, shutdown, and cleanup. This interface follows the single responsibility
// principle by focusing only on the agent's lifecycle state transitions.
type LifecycleManager interface {
	// Initialize prepares the agent with the given configuration.
	// The agent must be in an uninitialized state for this to succeed.
	Initialize(ctx context.Context, config *AgentConfig) error

	// Start begins the agent's operation.
	// The agent must be initialized or stopped for this to succeed.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the agent.
	// The agent must be running for this to succeed.
	Stop(ctx context.Context) error

	// Cleanup releases all resources held by the agent.
	// This can be called from any state and should be idempotent.
	Cleanup() error
}

// AgentEventProcessor handles event processing and streaming operations.
// This interface is responsible for all event-related functionality
// including single event processing and continuous event streaming.
type AgentEventProcessor interface {
	// ProcessEvent handles a single incoming event and returns response events.
	// The agent must be running for this to succeed.
	ProcessEvent(ctx context.Context, event events.Event) ([]events.Event, error)

	// StreamEvents returns a channel for receiving events from the agent.
	// The agent must be running and support streaming for this to succeed.
	StreamEvents(ctx context.Context) (<-chan events.Event, error)
}

// StateManager handles agent state operations including retrieval and updates.
// This interface manages the agent's internal state with support for
// atomic updates and conflict resolution.
type StateManager interface {
	// GetState returns the current state of the agent.
	// This can be called from any state where the agent is initialized.
	GetState(ctx context.Context) (*AgentState, error)

	// UpdateState applies a state change delta to the agent's state.
	// This uses optimistic concurrency control with version numbers.
	UpdateState(ctx context.Context, delta *StateDelta) error
}

// ToolRunner handles tool execution and discovery operations.
// This interface provides the agent's ability to execute tools and
// expose available tool definitions to clients.
type ToolRunner interface {
	// ExecuteTool executes a tool with the given name and parameters.
	// The agent must be running for this to succeed.
	ExecuteTool(ctx context.Context, name string, params interface{}) (interface{}, error)

	// ListTools returns a list of tools available to this agent.
	// This can be called from any state where the agent is initialized.
	ListTools() []ToolDefinition
}

// AgentMetadata provides access to agent metadata and status information.
// This interface handles read-only agent properties and health status.
type AgentMetadata interface {
	// Name returns the unique identifier for this agent instance.
	Name() string

	// Description returns a human-readable description of the agent's purpose.
	Description() string

	// Capabilities returns information about what this agent can do.
	Capabilities() AgentCapabilities

	// Health returns the current health status of the agent.
	Health() AgentHealthStatus
}

// Agent defines the complete interface that all AG-UI agents must implement.
// This interface composes smaller, focused interfaces following the Interface
// Segregation Principle. Each component interface has a single responsibility:
//
//   - LifecycleManager: handles agent lifecycle (init, start, stop, cleanup)
//   - AgentEventProcessor: handles event processing and streaming
//   - StateManager: handles agent state operations
//   - ToolRunner: handles tool execution and discovery
//   - AgentMetadata: provides agent metadata and health status
//
// This design allows for better testing, easier mocking, and more flexible
// implementations while maintaining backward compatibility.
type Agent interface {
	LifecycleManager
	AgentEventProcessor
	StateManager
	ToolRunner
	AgentMetadata
}

// AgentConfig contains configuration options for agent initialization.
type AgentConfig struct {
	Name            string                 `json:"name" yaml:"name"`
	Description     string                 `json:"description" yaml:"description"`
	Capabilities    AgentCapabilities      `json:"capabilities" yaml:"capabilities"`
	EventProcessing EventProcessingConfig  `json:"event_processing" yaml:"event_processing"`
	State           StateConfig            `json:"state" yaml:"state"`
	Tools           ToolsConfig            `json:"tools" yaml:"tools"`
	History         HistoryConfig          `json:"history" yaml:"history"`
	Custom          map[string]interface{} `json:"custom,omitempty" yaml:"custom,omitempty"`
}

// AgentCapabilities describes what features an agent supports.
type AgentCapabilities struct {
	Tools              []string               `json:"tools"`
	Streaming          bool                   `json:"streaming"`
	StateSync          bool                   `json:"state_sync"`
	MessageHistory     bool                   `json:"message_history"`
	CustomCapabilities map[string]interface{} `json:"custom_capabilities,omitempty"`
}

// EventProcessingConfig contains configuration for event processing.
type EventProcessingConfig struct {
	BufferSize       int           `json:"buffer_size" yaml:"buffer_size"`
	BatchSize        int           `json:"batch_size" yaml:"batch_size"`
	Timeout          time.Duration `json:"timeout" yaml:"timeout"`
	EnableValidation bool          `json:"enable_validation" yaml:"enable_validation"`
	EnableMetrics    bool          `json:"enable_metrics" yaml:"enable_metrics"`
}

// Note: ConflictResolutionStrategy is defined in agent_state.go with more comprehensive constants

// StateConfig contains configuration for state management.
type StateConfig struct {
	SyncInterval       time.Duration              `json:"sync_interval" yaml:"sync_interval"`
	CacheSize          string                     `json:"cache_size" yaml:"cache_size"`
	EnablePersistence  bool                       `json:"enable_persistence" yaml:"enable_persistence"`
	ConflictResolution ConflictResolutionStrategy `json:"conflict_resolution" yaml:"conflict_resolution"`
}

// ToolsConfig contains configuration for tool execution.
type ToolsConfig struct {
	Timeout          time.Duration `json:"timeout" yaml:"timeout"`
	MaxConcurrent    int           `json:"max_concurrent" yaml:"max_concurrent"`
	EnableSandboxing bool          `json:"enable_sandboxing" yaml:"enable_sandboxing"`
	EnableCaching    bool          `json:"enable_caching" yaml:"enable_caching"`
}

// HistoryConfig contains configuration for message history management.
type HistoryConfig struct {
	MaxMessages       int           `json:"max_messages" yaml:"max_messages"`
	Retention         time.Duration `json:"retention" yaml:"retention"`
	EnablePersistence bool          `json:"enable_persistence" yaml:"enable_persistence"`
	EnableCompression bool          `json:"enable_compression" yaml:"enable_compression"`
}

// ToolDefinition describes a tool available to the agent.
type ToolDefinition struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Schema       *tools.ToolSchema      `json:"schema"`
	Capabilities map[string]interface{} `json:"capabilities,omitempty"`
}

// HealthStatus represents the current health of an agent.
type AgentHealthStatus struct {
	Status    string                 `json:"status"`
	LastCheck time.Time              `json:"last_check"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Errors    []string               `json:"errors,omitempty"`
}

// AgentStatus represents the lifecycle status of an agent.
type AgentStatus string

const (
	AgentStatusUninitialized AgentStatus = "uninitialized"
	AgentStatusInitialized   AgentStatus = "initialized"
	AgentStatusStarting      AgentStatus = "starting"
	AgentStatusRunning       AgentStatus = "running"
	AgentStatusStopping      AgentStatus = "stopping"
	AgentStatusStopped       AgentStatus = "stopped"
	AgentStatusError         AgentStatus = "error"
)

// DefaultAgentConfig returns a default configuration for agents.
func DefaultAgentConfig() *AgentConfig {
	return &AgentConfig{
		Name:        "",
		Description: "",
		Capabilities: AgentCapabilities{
			Tools:          []string{},
			Streaming:      true,
			StateSync:      true,
			MessageHistory: true,
		},
		EventProcessing: EventProcessingConfig{
			BufferSize:       1000,
			BatchSize:        100,
			Timeout:          30 * time.Second,
			EnableValidation: true,
			EnableMetrics:    true,
		},
		State: StateConfig{
			SyncInterval:       5 * time.Second,
			CacheSize:          "100MB",
			EnablePersistence:  true,
			ConflictResolution: ConflictResolutionLastWriterWins,
		},
		Tools: ToolsConfig{
			Timeout:          30 * time.Second,
			MaxConcurrent:    10,
			EnableSandboxing: true,
			EnableCaching:    true,
		},
		History: HistoryConfig{
			MaxMessages:       10000,
			Retention:         30 * 24 * time.Hour,
			EnablePersistence: true,
			EnableCompression: true,
		},
		Custom: make(map[string]interface{}),
	}
}

// AgentState represents the complete state of an agent
type AgentState struct {
	// Status is the current lifecycle status
	Status AgentStatus `json:"status"`

	// Name is the agent identifier
	Name string `json:"name"`

	// Version is the state version for conflict resolution
	Version int64 `json:"version"`

	// Data contains the agent's custom state data
	Data map[string]interface{} `json:"data"`

	// Metadata contains additional state information
	Metadata map[string]interface{} `json:"metadata"`

	// LastModified is when the state was last updated
	LastModified time.Time `json:"last_modified"`

	// Checksum for state integrity verification
	Checksum string `json:"checksum,omitempty"`
}

// StateDelta represents an incremental state change
type StateDelta struct {
	// Version is the expected current version
	Version int64 `json:"version"`

	// Operations are the changes to apply
	Operations []StateOperation `json:"operations"`

	// Metadata for the change
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Timestamp when the delta was created
	Timestamp time.Time `json:"timestamp"`
}

// StateOperation represents a single state change operation
type StateOperation struct {
	// Operation type (set, delete, merge)
	Op StateOperationType `json:"op"`

	// Path to the field to modify (JSON pointer style)
	Path string `json:"path"`

	// Value for set/merge operations
	Value interface{} `json:"value,omitempty"`

	// Condition for conditional operations
	Condition *StateCondition `json:"condition,omitempty"`
}

// StateOperationType defines the type of state operation
type StateOperationType string

const (
	StateOpSet    StateOperationType = "set"
	StateOpDelete StateOperationType = "delete"
	StateOpMerge  StateOperationType = "merge"
	StateOpTest   StateOperationType = "test"
)

// StateCondition represents a condition for state operations
type StateCondition struct {
	// Field path to test
	Path string `json:"path"`

	// Expected value
	Expected interface{} `json:"expected"`

	// Comparison operation
	Op string `json:"op"` // "eq", "ne", "exists", "not_exists"
}
