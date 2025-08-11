package events

import (
	"encoding/json"
	"fmt"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/proto/generated"
)

// ThinkingStartEvent indicates the start of a thinking/reasoning phase
type ThinkingStartEvent struct {
	*BaseEvent
	Title *string `json:"title,omitempty"`
}

// NewThinkingStartEvent creates a new thinking start event
func NewThinkingStartEvent() *ThinkingStartEvent {
	return &ThinkingStartEvent{
		BaseEvent: NewBaseEvent(EventTypeThinkingStart),
	}
}

// WithTitle sets the title for the thinking phase
func (e *ThinkingStartEvent) WithTitle(title string) *ThinkingStartEvent {
	e.Title = &title
	return e
}

// Validate validates the thinking start event
func (e *ThinkingStartEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}
	return nil
}

// ToJSON serializes the event to JSON
func (e *ThinkingStartEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToProtobuf converts the event to its protobuf representation
func (e *ThinkingStartEvent) ToProtobuf() (*generated.Event, error) {
	// Note: This requires adding ThinkingStartEvent to the protobuf definition
	// For now, we'll return a basic event
	return &generated.Event{}, nil
}

// ThinkingEndEvent indicates the end of a thinking/reasoning phase
type ThinkingEndEvent struct {
	*BaseEvent
}

// NewThinkingEndEvent creates a new thinking end event
func NewThinkingEndEvent() *ThinkingEndEvent {
	return &ThinkingEndEvent{
		BaseEvent: NewBaseEvent(EventTypeThinkingEnd),
	}
}

// Validate validates the thinking end event
func (e *ThinkingEndEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}
	return nil
}

// ToJSON serializes the event to JSON
func (e *ThinkingEndEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToProtobuf converts the event to its protobuf representation
func (e *ThinkingEndEvent) ToProtobuf() (*generated.Event, error) {
	// Note: This requires adding ThinkingEndEvent to the protobuf definition
	// For now, we'll return a basic event
	return &generated.Event{}, nil
}

// ThinkingTextMessageStartEvent indicates the start of a thinking text message
type ThinkingTextMessageStartEvent struct {
	*BaseEvent
}

// NewThinkingTextMessageStartEvent creates a new thinking text message start event
func NewThinkingTextMessageStartEvent() *ThinkingTextMessageStartEvent {
	return &ThinkingTextMessageStartEvent{
		BaseEvent: NewBaseEvent(EventTypeThinkingTextMessageStart),
	}
}

// Validate validates the thinking text message start event
func (e *ThinkingTextMessageStartEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}
	return nil
}

// ToJSON serializes the event to JSON
func (e *ThinkingTextMessageStartEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToProtobuf converts the event to its protobuf representation
func (e *ThinkingTextMessageStartEvent) ToProtobuf() (*generated.Event, error) {
	// Note: This requires adding ThinkingTextMessageStartEvent to the protobuf definition
	// For now, we'll return a basic event
	return &generated.Event{}, nil
}

// ThinkingTextMessageContentEvent contains streaming thinking text content
type ThinkingTextMessageContentEvent struct {
	*BaseEvent
	Delta string `json:"delta"`
}

// NewThinkingTextMessageContentEvent creates a new thinking text message content event
func NewThinkingTextMessageContentEvent(delta string) *ThinkingTextMessageContentEvent {
	return &ThinkingTextMessageContentEvent{
		BaseEvent: NewBaseEvent(EventTypeThinkingTextMessageContent),
		Delta:     delta,
	}
}

// Validate validates the thinking text message content event
func (e *ThinkingTextMessageContentEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}

	if e.Delta == "" {
		return fmt.Errorf("ThinkingTextMessageContentEvent validation failed: delta field is required")
	}

	return nil
}

// ToJSON serializes the event to JSON
func (e *ThinkingTextMessageContentEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToProtobuf converts the event to its protobuf representation
func (e *ThinkingTextMessageContentEvent) ToProtobuf() (*generated.Event, error) {
	// Note: This requires adding ThinkingTextMessageContentEvent to the protobuf definition
	// For now, we'll return a basic event
	return &generated.Event{}, nil
}

// ThinkingTextMessageEndEvent indicates the end of a thinking text message
type ThinkingTextMessageEndEvent struct {
	*BaseEvent
}

// NewThinkingTextMessageEndEvent creates a new thinking text message end event
func NewThinkingTextMessageEndEvent() *ThinkingTextMessageEndEvent {
	return &ThinkingTextMessageEndEvent{
		BaseEvent: NewBaseEvent(EventTypeThinkingTextMessageEnd),
	}
}

// Validate validates the thinking text message end event
func (e *ThinkingTextMessageEndEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}
	return nil
}

// ToJSON serializes the event to JSON
func (e *ThinkingTextMessageEndEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToProtobuf converts the event to its protobuf representation
func (e *ThinkingTextMessageEndEvent) ToProtobuf() (*generated.Event, error) {
	// Note: This requires adding ThinkingTextMessageEndEvent to the protobuf definition
	// For now, we'll return a basic event
	return &generated.Event{}, nil
}
