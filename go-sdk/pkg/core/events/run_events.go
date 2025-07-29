package events

import (
	"encoding/json"
	"fmt"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/proto/generated"
)

// RunStartedEvent indicates that an agent run has started
type RunStartedEvent struct {
	*BaseEvent
	ThreadID string `json:"threadId"`
	RunID    string `json:"runId"`
}

// NewRunStartedEvent creates a new run started event
func NewRunStartedEvent(threadID, runID string) *RunStartedEvent {
	return &RunStartedEvent{
		BaseEvent: NewBaseEvent(EventTypeRunStarted),
		ThreadID:  threadID,
		RunID:     runID,
	}
}

// NewRunStartedEventWithOptions creates a new run started event with options
func NewRunStartedEventWithOptions(threadID, runID string, options ...RunStartedOption) *RunStartedEvent {
	event := &RunStartedEvent{
		BaseEvent: NewBaseEvent(EventTypeRunStarted),
		ThreadID:  threadID,
		RunID:     runID,
	}

	for _, opt := range options {
		opt(event)
	}

	return event
}

// RunStartedOption defines options for creating run started events
type RunStartedOption func(*RunStartedEvent)

// WithAutoRunID automatically generates a unique run ID if the provided runID is empty
func WithAutoRunID() RunStartedOption {
	return func(e *RunStartedEvent) {
		if e.RunID == "" {
			e.RunID = GenerateRunID()
		}
	}
}

// WithAutoThreadID automatically generates a unique thread ID if the provided threadID is empty
func WithAutoThreadID() RunStartedOption {
	return func(e *RunStartedEvent) {
		if e.ThreadID == "" {
			e.ThreadID = GenerateThreadID()
		}
	}
}

// Validate validates the run started event
func (e *RunStartedEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}

	if e.ThreadID == "" {
		return fmt.Errorf("RunStartedEvent validation failed: threadId field is required")
	}

	if e.RunID == "" {
		return fmt.Errorf("RunStartedEvent validation failed: runId field is required")
	}

	return nil
}

// ToJSON serializes the event to JSON
func (e *RunStartedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToProtobuf converts the event to its protobuf representation
func (e *RunStartedEvent) ToProtobuf() (*generated.Event, error) {
	pbEvent := &generated.RunStartedEvent{
		BaseEvent: e.BaseEvent.ToProtobufBase(),
		ThreadId:  e.ThreadID,
		RunId:     e.RunID,
	}

	return &generated.Event{
		Event: &generated.Event_RunStarted{
			RunStarted: pbEvent,
		},
	}, nil
}

// RunFinishedEvent indicates that an agent run has finished successfully
type RunFinishedEvent struct {
	*BaseEvent
	ThreadID string `json:"threadId"`
	RunID    string `json:"runId"`
}

// NewRunFinishedEvent creates a new run finished event
func NewRunFinishedEvent(threadID, runID string) *RunFinishedEvent {
	return &RunFinishedEvent{
		BaseEvent: NewBaseEvent(EventTypeRunFinished),
		ThreadID:  threadID,
		RunID:     runID,
	}
}

// NewRunFinishedEventWithOptions creates a new run finished event with options
func NewRunFinishedEventWithOptions(threadID, runID string, options ...RunFinishedOption) *RunFinishedEvent {
	event := &RunFinishedEvent{
		BaseEvent: NewBaseEvent(EventTypeRunFinished),
		ThreadID:  threadID,
		RunID:     runID,
	}

	for _, opt := range options {
		opt(event)
	}

	return event
}

// RunFinishedOption defines options for creating run finished events
type RunFinishedOption func(*RunFinishedEvent)

// WithAutoRunIDFinished automatically generates a unique run ID if the provided runID is empty
func WithAutoRunIDFinished() RunFinishedOption {
	return func(e *RunFinishedEvent) {
		if e.RunID == "" {
			e.RunID = GenerateRunID()
		}
	}
}

// WithAutoThreadIDFinished automatically generates a unique thread ID if the provided threadID is empty
func WithAutoThreadIDFinished() RunFinishedOption {
	return func(e *RunFinishedEvent) {
		if e.ThreadID == "" {
			e.ThreadID = GenerateThreadID()
		}
	}
}

// Validate validates the run finished event
func (e *RunFinishedEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}

	if e.ThreadID == "" {
		return fmt.Errorf("RunFinishedEvent validation failed: threadId field is required")
	}

	if e.RunID == "" {
		return fmt.Errorf("RunFinishedEvent validation failed: runId field is required")
	}

	return nil
}

// ToJSON serializes the event to JSON
func (e *RunFinishedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToProtobuf converts the event to its protobuf representation
func (e *RunFinishedEvent) ToProtobuf() (*generated.Event, error) {
	pbEvent := &generated.RunFinishedEvent{
		BaseEvent: e.BaseEvent.ToProtobufBase(),
		ThreadId:  e.ThreadID,
		RunId:     e.RunID,
	}

	return &generated.Event{
		Event: &generated.Event_RunFinished{
			RunFinished: pbEvent,
		},
	}, nil
}

// RunErrorEvent indicates that an agent run has encountered an error
type RunErrorEvent struct {
	*BaseEvent
	Code    *string `json:"code,omitempty"`
	Message string  `json:"message"`
	RunID   string  `json:"runId,omitempty"`
}

// NewRunErrorEvent creates a new run error event
func NewRunErrorEvent(message string, options ...RunErrorOption) *RunErrorEvent {
	event := &RunErrorEvent{
		BaseEvent: NewBaseEvent(EventTypeRunError),
		Message:   message,
	}

	for _, opt := range options {
		opt(event)
	}

	return event
}

// RunErrorOption defines options for creating run error events
type RunErrorOption func(*RunErrorEvent)

// WithErrorCode sets the error code
func WithErrorCode(code string) RunErrorOption {
	return func(e *RunErrorEvent) {
		e.Code = &code
	}
}

// WithRunID sets the run ID for the error
func WithRunID(runID string) RunErrorOption {
	return func(e *RunErrorEvent) {
		e.RunID = runID
	}
}

// WithAutoRunIDError automatically generates a unique run ID if the provided runID is empty
func WithAutoRunIDError() RunErrorOption {
	return func(e *RunErrorEvent) {
		if e.RunID == "" {
			e.RunID = GenerateRunID()
		}
	}
}

// Validate validates the run error event
func (e *RunErrorEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}

	if e.Message == "" {
		return fmt.Errorf("RunErrorEvent validation failed: message field is required")
	}

	return nil
}

// ToJSON serializes the event to JSON
func (e *RunErrorEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToProtobuf converts the event to its protobuf representation
func (e *RunErrorEvent) ToProtobuf() (*generated.Event, error) {
	pbEvent := &generated.RunErrorEvent{
		BaseEvent: e.BaseEvent.ToProtobufBase(),
		Message:   e.Message,
	}

	if e.Code != nil {
		pbEvent.Code = e.Code
	}

	return &generated.Event{
		Event: &generated.Event_RunError{
			RunError: pbEvent,
		},
	}, nil
}

// StepStartedEvent indicates that an agent step has started
type StepStartedEvent struct {
	*BaseEvent
	StepName string `json:"stepName"`
}

// NewStepStartedEvent creates a new step started event
func NewStepStartedEvent(stepName string) *StepStartedEvent {
	return &StepStartedEvent{
		BaseEvent: NewBaseEvent(EventTypeStepStarted),
		StepName:  stepName,
	}
}

// NewStepStartedEventWithOptions creates a new step started event with options
func NewStepStartedEventWithOptions(stepName string, options ...StepStartedOption) *StepStartedEvent {
	event := &StepStartedEvent{
		BaseEvent: NewBaseEvent(EventTypeStepStarted),
		StepName:  stepName,
	}

	for _, opt := range options {
		opt(event)
	}

	return event
}

// StepStartedOption defines options for creating step started events
type StepStartedOption func(*StepStartedEvent)

// WithAutoStepName automatically generates a unique step name if the provided stepName is empty
func WithAutoStepName() StepStartedOption {
	return func(e *StepStartedEvent) {
		if e.StepName == "" {
			e.StepName = GenerateStepID()
		}
	}
}

// Validate validates the step started event
func (e *StepStartedEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}

	if e.StepName == "" {
		return fmt.Errorf("StepStartedEvent validation failed: stepName field is required")
	}

	return nil
}

// ToJSON serializes the event to JSON
func (e *StepStartedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToProtobuf converts the event to its protobuf representation
func (e *StepStartedEvent) ToProtobuf() (*generated.Event, error) {
	pbEvent := &generated.StepStartedEvent{
		BaseEvent: e.BaseEvent.ToProtobufBase(),
		StepName:  e.StepName,
	}

	return &generated.Event{
		Event: &generated.Event_StepStarted{
			StepStarted: pbEvent,
		},
	}, nil
}

// StepFinishedEvent indicates that an agent step has finished
type StepFinishedEvent struct {
	*BaseEvent
	StepName string `json:"stepName"`
}

// NewStepFinishedEvent creates a new step finished event
func NewStepFinishedEvent(stepName string) *StepFinishedEvent {
	return &StepFinishedEvent{
		BaseEvent: NewBaseEvent(EventTypeStepFinished),
		StepName:  stepName,
	}
}

// NewStepFinishedEventWithOptions creates a new step finished event with options
func NewStepFinishedEventWithOptions(stepName string, options ...StepFinishedOption) *StepFinishedEvent {
	event := &StepFinishedEvent{
		BaseEvent: NewBaseEvent(EventTypeStepFinished),
		StepName:  stepName,
	}

	for _, opt := range options {
		opt(event)
	}

	return event
}

// StepFinishedOption defines options for creating step finished events
type StepFinishedOption func(*StepFinishedEvent)

// WithAutoStepNameFinished automatically generates a unique step name if the provided stepName is empty
func WithAutoStepNameFinished() StepFinishedOption {
	return func(e *StepFinishedEvent) {
		if e.StepName == "" {
			e.StepName = GenerateStepID()
		}
	}
}

// Validate validates the step finished event
func (e *StepFinishedEvent) Validate() error {
	if err := e.BaseEvent.Validate(); err != nil {
		return err
	}

	if e.StepName == "" {
		return fmt.Errorf("StepFinishedEvent validation failed: stepName field is required")
	}

	return nil
}

// ToJSON serializes the event to JSON
func (e *StepFinishedEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToProtobuf converts the event to its protobuf representation
func (e *StepFinishedEvent) ToProtobuf() (*generated.Event, error) {
	pbEvent := &generated.StepFinishedEvent{
		BaseEvent: e.BaseEvent.ToProtobufBase(),
		StepName:  e.StepName,
	}

	return &generated.Event{
		Event: &generated.Event_StepFinished{
			StepFinished: pbEvent,
		},
	}, nil
}
