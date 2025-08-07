package core

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// EventData is the base interface that all typed event data must implement.
// This provides a common contract for event data validation and serialization.
type EventData interface {
	// Validate ensures the event data is valid
	Validate() error

	// ToMap converts the event data to a map[string]interface{} for backward compatibility
	ToMap() map[string]interface{}

	// FromMap populates the event data from a map[string]interface{} for backward compatibility
	FromMap(data map[string]interface{}) error
}

// TypedEvent represents a protocol event in the AG-UI system with type-safe data.
// Events flow bidirectionally between agents and front-end applications.
// T must implement EventData for validation and serialization support.
type TypedEvent[T EventData] interface {
	// ID returns the unique identifier for this event
	ID() string

	// Type returns the event type (e.g., "message", "state_update", "tool_call")
	Type() string

	// Timestamp returns when the event was created
	Timestamp() time.Time

	// TypedData returns the strongly-typed event data
	TypedData() T

	// Data returns the event data as a map for backward compatibility
	// Deprecated: Use TypedData() for type-safe access
	Data() map[string]interface{}

	// Validate validates the event and its data
	Validate() error

	// ToJSON serializes the event to JSON
	ToJSON() ([]byte, error)
}

// Event represents the legacy event interface for backward compatibility.
// New code should use TypedEvent[T] instead.
type Event[T any] interface {
	// ID returns the unique identifier for this event
	ID() string

	// Type returns the event type (e.g., "message", "state_update", "tool_call")
	Type() string

	// Timestamp returns when the event was created
	Timestamp() time.Time

	// Data returns the typed event payload
	Data() T
}

// TypedBaseEvent provides a common implementation for typed events
type TypedBaseEvent[T EventData] struct {
	id        string
	eventType string
	timestamp time.Time
	data      T
}

// NewTypedEvent creates a new typed event with the given parameters
func NewTypedEvent[T EventData](id, eventType string, data T) TypedEvent[T] {
	return &TypedBaseEvent[T]{
		id:        id,
		eventType: eventType,
		timestamp: time.Now(),
		data:      data,
	}
}

// ID returns the unique identifier for this event
func (e *TypedBaseEvent[T]) ID() string {
	return e.id
}

// Type returns the event type
func (e *TypedBaseEvent[T]) Type() string {
	return e.eventType
}

// Timestamp returns when the event was created
func (e *TypedBaseEvent[T]) Timestamp() time.Time {
	return e.timestamp
}

// TypedData returns the strongly-typed event data
func (e *TypedBaseEvent[T]) TypedData() T {
	return e.data
}

// Data returns the event data as a map for backward compatibility
func (e *TypedBaseEvent[T]) Data() map[string]interface{} {
	return e.data.ToMap()
}

// Validate validates the event and its data
func (e *TypedBaseEvent[T]) Validate() error {
	if e.id == "" {
		return fmt.Errorf("event ID is required")
	}
	if e.eventType == "" {
		return fmt.Errorf("event type is required")
	}
	if e.timestamp.IsZero() {
		return fmt.Errorf("event timestamp is required")
	}
	return e.data.Validate()
}

// ToJSON serializes the event to JSON
func (e *TypedBaseEvent[T]) ToJSON() ([]byte, error) {
	eventMap := map[string]interface{}{
		"id":        e.id,
		"type":      e.eventType,
		"timestamp": e.timestamp.UnixMilli(),
		"data":      e.data.ToMap(),
	}
	return json.Marshal(eventMap)
}

// BaseEvent provides a common implementation for legacy events
type BaseEvent[T any] struct {
	id        string
	eventType string
	timestamp time.Time
	data      T
}

func (e BaseEvent[T]) ID() string           { return e.id }
func (e BaseEvent[T]) Type() string         { return e.eventType }
func (e BaseEvent[T]) Timestamp() time.Time { return e.timestamp }
func (e BaseEvent[T]) Data() T              { return e.data }

// NewEvent creates a new legacy event with the given parameters
// Deprecated: Use NewTypedEvent for type-safe events
func NewEvent[T any](id, eventType string, data T) Event[T] {
	return BaseEvent[T]{
		id:        id,
		eventType: eventType,
		timestamp: time.Now(),
		data:      data,
	}
}

// Concrete event data types with type safety

// MessageData represents typed message event data
type MessageData struct {
	Content string `json:"content"`
	Sender  string `json:"sender"`
	Role    string `json:"role,omitempty"`
}

// Validate ensures the message data is valid
func (m MessageData) Validate() error {
	if m.Content == "" {
		return fmt.Errorf("message content is required")
	}
	if m.Sender == "" {
		return fmt.Errorf("message sender is required")
	}
	return nil
}

// ToMap converts the message data to a map for backward compatibility
func (m MessageData) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"content": m.Content,
		"sender":  m.Sender,
	}
	if m.Role != "" {
		result["role"] = m.Role
	}
	return result
}

// FromMap populates the message data from a map for backward compatibility
func (m *MessageData) FromMap(data map[string]interface{}) error {
	if content, ok := data["content"].(string); ok {
		m.Content = content
	} else {
		return fmt.Errorf("content field is required and must be a string")
	}

	if sender, ok := data["sender"].(string); ok {
		m.Sender = sender
	} else {
		return fmt.Errorf("sender field is required and must be a string")
	}

	if role, ok := data["role"].(string); ok {
		m.Role = role
	}

	return nil
}

// StateData represents typed state event data
type StateData struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// Validate ensures the state data is valid
func (s StateData) Validate() error {
	if s.Key == "" {
		return fmt.Errorf("state key is required")
	}
	if s.Value == nil {
		return fmt.Errorf("state value is required")
	}
	return nil
}

// ToMap converts the state data to a map for backward compatibility
func (s StateData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"key":   s.Key,
		"value": s.Value,
	}
}

// FromMap populates the state data from a map for backward compatibility
func (s *StateData) FromMap(data map[string]interface{}) error {
	if key, ok := data["key"].(string); ok {
		s.Key = key
	} else {
		return fmt.Errorf("key field is required and must be a string")
	}

	s.Value = data["value"]
	if s.Value == nil {
		return fmt.Errorf("value field is required")
	}

	return nil
}

// ToolData represents typed tool event data
type ToolData struct {
	ToolName string                 `json:"tool_name"`
	Args     map[string]interface{} `json:"args"`
	Result   interface{}            `json:"result,omitempty"`
}

// Validate ensures the tool data is valid
func (t ToolData) Validate() error {
	if t.ToolName == "" {
		return fmt.Errorf("tool name is required")
	}
	if t.Args == nil {
		return fmt.Errorf("tool args are required")
	}
	return nil
}

// ToMap converts the tool data to a map for backward compatibility
func (t ToolData) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"tool_name": t.ToolName,
		"args":      t.Args,
	}
	if t.Result != nil {
		result["result"] = t.Result
	}
	return result
}

// FromMap populates the tool data from a map for backward compatibility
func (t *ToolData) FromMap(data map[string]interface{}) error {
	if toolName, ok := data["tool_name"].(string); ok {
		t.ToolName = toolName
	} else {
		return fmt.Errorf("tool_name field is required and must be a string")
	}

	if args, ok := data["args"].(map[string]interface{}); ok {
		t.Args = args
	} else {
		return fmt.Errorf("args field is required and must be a map")
	}

	t.Result = data["result"]

	return nil
}

// Concrete typed event types
type TypedMessageEvent = TypedEvent[*MessageData]
type TypedStateEvent = TypedEvent[*StateData]
type TypedToolEvent = TypedEvent[*ToolData]

// Legacy concrete event types for backward compatibility
type MessageEvent = Event[MessageData]
type StateEvent = Event[StateData]
type ToolEvent = Event[ToolData]

// ==============================================================================
// DECOMPOSED AGENT INTERFACES
// ==============================================================================

// AgentIdentity provides basic agent identification.
type AgentIdentity interface {
	// Name returns the agent's identifier
	Name() string

	// Description returns a human-readable description of the agent's capabilities
	Description() string
}

// AgentEventHandler processes events and generates responses.
type AgentEventHandler interface {
	// HandleEvent processes an incoming event and optionally returns response events
	// Note: Returns generic events - agents must handle type assertions as needed
	HandleEvent(ctx context.Context, event any) ([]any, error)
}

// Agent represents an AI agent that can process events and generate responses.
// Agents are the core abstraction in the AG-UI protocol.
// Composed of focused interfaces following the Interface Segregation Principle.
type Agent interface {
	AgentIdentity
	AgentEventHandler
}

// TypedAgentEventHandler processes typed events with type safety.
type TypedAgentEventHandler[T EventData, R EventData] interface {
	// HandleTypedEvent processes a typed event and returns typed response events
	HandleTypedEvent(ctx context.Context, event TypedEvent[T]) ([]TypedEvent[R], error)
}

// AgentTypeRegistry provides information about supported event types.
type AgentTypeRegistry interface {
	// SupportedInputTypes returns the event data types this agent can handle
	SupportedInputTypes() []string

	// SupportedOutputTypes returns the event data types this agent can produce
	SupportedOutputTypes() []string
}

// TypedAgent represents a type-safe AI agent that can process typed events.
// T represents the input event data type, R represents the response event data type.
// Composed of focused interfaces following the Interface Segregation Principle.
type TypedAgent[T EventData, R EventData] interface {
	AgentIdentity
	TypedAgentEventHandler[T, R]
	AgentTypeRegistry
}

// TypedEventHandler is a function type for handling specific typed event types.
type TypedEventHandler[T EventData, R EventData] func(ctx context.Context, event TypedEvent[T]) ([]TypedEvent[R], error)

// EventHandler is a function type for handling specific event types (legacy).
// Deprecated: Use TypedEventHandler for type-safe event handling
type EventHandler[T any] func(ctx context.Context, event Event[T]) ([]any, error)

// StreamConfig contains configuration for event streaming.
type StreamConfig struct {
	// BufferSize is the size of the event buffer
	BufferSize int

	// Timeout is the maximum time to wait for events
	Timeout time.Duration

	// EnableCompression enables event compression during transport
	EnableCompression bool

	// ValidationEnabled enables event validation during streaming
	ValidationEnabled bool

	// TypeSafetyEnabled enables type-safe event processing
	TypeSafetyEnabled bool
}

// EventConstraint is a type constraint for events that can be processed
type EventConstraint interface {
	ID() string
	Type() string
	Timestamp() time.Time
}

// TypedEventConstraint is a type constraint for typed events
type TypedEventConstraint[T EventData] interface {
	EventConstraint
	TypedData() T
	Validate() error
}

// ValidationResult contains the result of event validation
type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

// EventProcessor provides type-safe event processing capabilities
type EventProcessor[T EventData, R EventData] struct {
	handler TypedEventHandler[T, R]
	config  *StreamConfig
}

// NewEventProcessor creates a new type-safe event processor
func NewEventProcessor[T EventData, R EventData](
	handler TypedEventHandler[T, R],
	config *StreamConfig,
) *EventProcessor[T, R] {
	if config == nil {
		config = &StreamConfig{
			BufferSize:        100,
			Timeout:           30 * time.Second,
			EnableCompression: false,
			ValidationEnabled: true,
			TypeSafetyEnabled: true,
		}
	}
	return &EventProcessor[T, R]{
		handler: handler,
		config:  config,
	}
}

// Process processes a typed event through the handler
func (p *EventProcessor[T, R]) Process(ctx context.Context, event TypedEvent[T]) ([]TypedEvent[R], error) {
	if p.config.ValidationEnabled {
		if err := event.Validate(); err != nil {
			return nil, fmt.Errorf("event validation failed: %w", err)
		}
	}

	return p.handler(ctx, event)
}

// EventAdapter provides conversion between legacy and typed events
type EventAdapter struct{}

// ToTypedEvent converts a legacy event to a typed event
func ToTypedEvent[T EventData](
	event Event[map[string]interface{}],
	constructor func() T,
) (TypedEvent[T], error) {
	data := constructor()
	if err := data.FromMap(event.Data()); err != nil {
		return nil, fmt.Errorf("failed to convert event data: %w", err)
	}

	return NewTypedEvent[T](event.ID(), event.Type(), data), nil
}

// ToLegacyEvent converts a typed event to a legacy event
func ToLegacyEvent[T EventData](
	event TypedEvent[T],
) Event[map[string]interface{}] {
	return NewEvent[map[string]interface{}](
		event.ID(),
		event.Type(),
		event.Data(),
	)
}

// Additional core types will be defined here as the protocol specification
// is implemented in subsequent development phases.
