package client

import (
	"context"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
)

// Agent defines the fundamental interface that all AG-UI agents must implement.
// This interface provides lifecycle management, event processing, state management,
// tool integration, and context-aware operations with cancellation support.
type Agent interface {
	// Core lifecycle management
	Initialize(ctx context.Context, config *AgentConfig) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Cleanup() error
	
	// Event processing
	ProcessEvent(ctx context.Context, event events.Event) ([]events.Event, error)
	StreamEvents(ctx context.Context) (<-chan events.Event, error)
	
	// State management
	GetState(ctx context.Context) (interface{}, error)
	UpdateState(ctx context.Context, delta interface{}) error
	
	// Tool integration
	ExecuteTool(ctx context.Context, name string, params interface{}) (interface{}, error)
	ListTools() []ToolDefinition
	
	// Metadata and capabilities
	Name() string
	Description() string
	Capabilities() AgentCapabilities
	Health() HealthStatus
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

// StateConfig contains configuration for state management.
type StateConfig struct {
	SyncInterval       time.Duration `json:"sync_interval" yaml:"sync_interval"`
	CacheSize          string        `json:"cache_size" yaml:"cache_size"`
	EnablePersistence  bool          `json:"enable_persistence" yaml:"enable_persistence"`
	ConflictResolution string        `json:"conflict_resolution" yaml:"conflict_resolution"`
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
type HealthStatus struct {
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
			ConflictResolution: "last-writer-wins",
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