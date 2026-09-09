package events

import (
	"encoding/json"
	"fmt"

	coretypes "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
)

// validJSONPatchOps contains the valid JSON Patch operations for efficient lookup
var validJSONPatchOps = map[string]bool{
	"add":     true,
	"remove":  true,
	"replace": true,
	"move":    true,
	"copy":    true,
	"test":    true,
}

// RoleActivity is the role for activity messages
const RoleActivity = "activity"

type Message = coretypes.Message

type ToolCall = coretypes.ToolCall

type Function = coretypes.FunctionCall

// StateSnapshotEvent contains a complete snapshot of the state
type StateSnapshotEvent struct {
	*BaseEvent
	Snapshot any `json:"snapshot"`
	// SubagentRunID attributes this event to a subagent invocation.
	// Empty when the event comes from the root agent.
	SubagentRunID string `json:"subagentRunId,omitempty"`
}

// NewStateSnapshotEvent creates a new state snapshot event
func NewStateSnapshotEvent(snapshot any) *StateSnapshotEvent {
	return &StateSnapshotEvent{
		BaseEvent: NewBaseEvent(EventTypeStateSnapshot),
		Snapshot:  snapshot,
	}
}

// Validate validates the state snapshot event
func (e *StateSnapshotEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}
	return nil
}

// ToJSON serializes the event to JSON
func (e *StateSnapshotEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// JSONPatchOperation represents a JSON Patch operation (RFC 6902)
type JSONPatchOperation struct {
	Op    string `json:"op"`              // "add", "remove", "replace", "move", "copy", "test"
	Path  string `json:"path"`            // JSON Pointer path
	Value any    `json:"value,omitempty"` // Value for add, replace, test operations
	From  string `json:"from,omitempty"`  // Source path for move, copy operations
}

// MarshalJSON preserves the required members whose valid values overlap with
// Go zero values. In particular, an empty JSON Pointer addresses the document
// root and nil is the explicit JSON null value for value operations.
func (op JSONPatchOperation) MarshalJSON() ([]byte, error) {
	payload := map[string]any{
		"op":   op.Op,
		"path": op.Path,
	}
	if op.Value != nil || op.Op == "add" || op.Op == "replace" || op.Op == "test" {
		payload["value"] = op.Value
	}
	if op.From != "" || op.Op == "move" || op.Op == "copy" {
		payload["from"] = op.From
	}
	return json.Marshal(payload)
}

// UnmarshalJSON validates required member presence before decoding into the
// public representation, which intentionally has no presence bookkeeping.
// Missing members always fail here because their absence cannot be retained in
// the public fields. Operation-name validation remains in event Validate methods.
func (op *JSONPatchOperation) UnmarshalJSON(data []byte) error {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	// RFC 6902 operation extensions are ignored; required members are decoded below.

	decodeString := func(name string) (string, error) {
		raw, ok := payload[name]
		if !ok {
			return "", fmt.Errorf("%s field is required", name)
		}
		if string(raw) == "null" {
			return "", fmt.Errorf("%s field must be a string", name)
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", fmt.Errorf("%s field must be a string", name)
		}
		return value, nil
	}

	operation, err := decodeString("op")
	if err != nil {
		return err
	}
	path, err := decodeString("path")
	if err != nil {
		return err
	}

	decoded := JSONPatchOperation{Op: operation, Path: path}
	if raw, ok := payload["value"]; ok {
		if err := json.Unmarshal(raw, &decoded.Value); err != nil {
			return fmt.Errorf("invalid value field: %w", err)
		}
	} else if operation == "add" || operation == "replace" || operation == "test" {
		return fmt.Errorf("value field is required for %s operation", operation)
	}
	if _, ok := payload["from"]; ok {
		decoded.From, err = decodeString("from")
		if err != nil {
			return err
		}
	} else if operation == "move" || operation == "copy" {
		return fmt.Errorf("from field is required for %s operation", operation)
	}
	*op = decoded
	return nil
}

// StateDeltaEvent contains incremental state changes using JSON Patch
type StateDeltaEvent struct {
	*BaseEvent
	Delta []JSONPatchOperation `json:"delta"`
	// SubagentRunID attributes this event to a subagent invocation.
	// Empty when the event comes from the root agent.
	SubagentRunID string `json:"subagentRunId,omitempty"`
}

// NewStateDeltaEvent creates a new state delta event
func NewStateDeltaEvent(delta []JSONPatchOperation) *StateDeltaEvent {
	if delta == nil {
		delta = []JSONPatchOperation{}
	}
	return &StateDeltaEvent{
		BaseEvent: NewBaseEvent(EventTypeStateDelta),
		Delta:     delta,
	}
}

// Validate validates the state delta event
func (e *StateDeltaEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}

	// Validate each JSON patch operation
	for i, op := range e.Delta {
		if err := validateJSONPatchOperation(op); err != nil {
			return fmt.Errorf("StateDeltaEvent validation failed: invalid operation at index %d: %w", i, err)
		}
	}

	return nil
}

// validateJSONPatchOperation validates a single JSON patch operation
func validateJSONPatchOperation(op JSONPatchOperation) error {
	// Validate operation type using map lookup for better performance
	if !validJSONPatchOps[op.Op] {
		return fmt.Errorf("op field must be one of: add, remove, replace, move, copy, test, got: %s", op.Op)
	}

	return nil
}

// MarshalJSON normalizes a nil delta to the protocol's empty patch array.
func (e StateDeltaEvent) MarshalJSON() ([]byte, error) {
	type wire StateDeltaEvent
	if e.Delta == nil {
		e.Delta = []JSONPatchOperation{}
	}
	return json.Marshal(wire(e))
}

// ToJSON serializes the event to JSON
func (e *StateDeltaEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// MessagesSnapshotEvent contains a snapshot of all messages
type MessagesSnapshotEvent struct {
	*BaseEvent
	Messages []Message `json:"messages"`
}

// NewMessagesSnapshotEvent creates a new messages snapshot event
func NewMessagesSnapshotEvent(messages []Message) *MessagesSnapshotEvent {
	return &MessagesSnapshotEvent{
		BaseEvent: NewBaseEvent(EventTypeMessagesSnapshot),
		Messages:  messages,
	}
}

// Validate validates the messages snapshot event
func (e *MessagesSnapshotEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}

	// Validate each message
	for i, msg := range e.Messages {
		if err := validateMessage(msg); err != nil {
			return fmt.Errorf("invalid message at index %d: %w", i, err)
		}
	}

	return nil
}

// MarshalJSON normalizes nil messages to an empty array without mutating the event.
func (e MessagesSnapshotEvent) MarshalJSON() ([]byte, error) {
	type wire MessagesSnapshotEvent
	if e.Messages == nil {
		e.Messages = []Message{}
	}
	return json.Marshal(wire(e))
}

// validateMessage validates a single message
func validateMessage(msg Message) error {
	if msg.ID == "" {
		return fmt.Errorf("message id field is required")
	}

	if msg.Role == "" {
		return fmt.Errorf("message role field is required")
	}

	if msg.ActivityType != "" && msg.Role != coretypes.RoleActivity {
		return fmt.Errorf("activityType is only valid for activity messages")
	}

	switch msg.Role {
	case coretypes.RoleDeveloper, coretypes.RoleSystem:
		if _, ok := msg.ContentString(); !ok {
			return fmt.Errorf("content field must be a string for %s messages", msg.Role)
		}
	case coretypes.RoleAssistant:
		if msg.Content != nil {
			if _, ok := msg.ContentString(); !ok {
				return fmt.Errorf("content field must be a string for assistant messages")
			}
		}
	case coretypes.RoleReasoning:
		if _, ok := msg.ContentString(); !ok {
			return fmt.Errorf("content field must be a string for reasoning messages")
		}
	case coretypes.RoleUser:
		if _, ok := msg.ContentString(); ok {
			break
		}
		if _, ok := msg.ContentInputContents(); ok {
			break
		}
		return fmt.Errorf("content field must be a string or input content array for user messages")
	case coretypes.RoleTool:
		if _, ok := msg.ContentString(); !ok {
			return fmt.Errorf("content field must be a string for tool messages")
		}
		if msg.ToolCallID == "" {
			return fmt.Errorf("toolCallId field is required for tool messages")
		}
	case coretypes.RoleActivity:
		if msg.ActivityType == "" {
			return fmt.Errorf("activityType field is required for activity messages")
		}
		if _, ok := msg.ContentActivity(); !ok {
			return fmt.Errorf("content field must be a map for activity messages")
		}
	default:
		return fmt.Errorf("unsupported message role: %s", msg.Role)
	}

	if msg.Role != coretypes.RoleAssistant && len(msg.ToolCalls) > 0 {
		return fmt.Errorf("toolCalls are only valid for assistant messages")
	}

	if msg.Role != coretypes.RoleTool {
		if msg.ToolCallID != "" {
			return fmt.Errorf("toolCallId is only valid for tool messages")
		}
		if msg.Error != "" {
			return fmt.Errorf("error is only valid for tool messages")
		}
	}

	// Validate tool calls if present
	for i, toolCall := range msg.ToolCalls {
		if err := validateToolCall(toolCall); err != nil {
			return fmt.Errorf("invalid tool call at index %d: %w", i, err)
		}
	}

	return nil
}

// validateToolCall validates a single tool call
func validateToolCall(toolCall ToolCall) error {
	if toolCall.ID == "" {
		return fmt.Errorf("tool call id field is required")
	}

	if toolCall.Type == "" {
		return fmt.Errorf("tool call type field is required")
	}

	if toolCall.Function.Name == "" {
		return fmt.Errorf("function name field is required")
	}

	return nil
}

// ToJSON serializes the event to JSON
func (e *MessagesSnapshotEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}
