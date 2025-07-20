// This is a stub file for testing purposes
package http

import (
	"time"
)

// Stub implementations for missing dependencies

// EventType represents the type of an event
type EventType string

const (
	EventTypeTextMessageContent EventType = "text_message_content"
	EventTypeToolCallStart      EventType = "tool_call_start"
	EventTypeToolCallEnd        EventType = "tool_call_end"
)

// Event interface for testing
type Event interface {
	Type() EventType
	Timestamp() *int64
	SetTimestamp(timestamp int64)
	ToJSON() ([]byte, error)
	ToProtobuf() (*GeneratedEvent, error)
	GetBaseEvent() *BaseEvent
	Validate() error
}

// BaseEvent stub
type BaseEvent struct{}

// GeneratedEvent stub  
type GeneratedEvent struct{}

// ValidationResult stub
type ValidationResult struct {
	IsValid   bool
	Errors    []*ValidationError
	Timestamp time.Time
}

func (r *ValidationResult) AddError(err *ValidationError) {
	r.Errors = append(r.Errors, err)
	r.IsValid = false
}

// ValidationError stub
type ValidationError struct {
	RuleID    string
	EventType EventType
	Message   string
	Severity  ValidationSeverity
	Timestamp time.Time
}

// ValidationSeverity stub
type ValidationSeverity int

const (
	ValidationSeverityError ValidationSeverity = iota
	ValidationSeverityWarning
	ValidationSeverityInfo
)

// PublicTransportStats is the public interface for transport stats
type PublicTransportStats struct {
	EventsSent       int64
	EventsReceived   int64
	EventsFailed     int64
	BytesTransferred int64
	AverageLatency   time.Duration
}