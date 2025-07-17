package transport

import (
	"time"
)

// TransportEventType represents different types of transport events.
type TransportEventType string

const (
	// EventTypeConnected indicates a successful connection.
	EventTypeConnected TransportEventType = "connected"
	// EventTypeDisconnected indicates a disconnection.
	EventTypeDisconnected TransportEventType = "disconnected"
	// EventTypeReconnecting indicates a reconnection attempt.
	EventTypeReconnecting TransportEventType = "reconnecting"
	// EventTypeError indicates an error occurred.
	EventTypeError TransportEventType = "error"
	// EventTypeEventSent indicates an event was sent.
	EventTypeEventSent TransportEventType = "event_sent"
	// EventTypeEventReceived indicates an event was received.
	EventTypeEventReceived TransportEventType = "event_received"
	// EventTypeStatsUpdated indicates transport statistics were updated.
	EventTypeStatsUpdated TransportEventType = "stats_updated"
)

// TransportEventImpl represents a transport-related event.
type TransportEventImpl struct {
	Type      TransportEventType `json:"type"`
	Timestamp time.Time          `json:"timestamp"`
	Transport string             `json:"transport"`
	Data      any                `json:"data,omitempty"`
	Error     error              `json:"error,omitempty"`
}

// NewTransportEvent creates a new transport event.
func NewTransportEvent(eventType TransportEventType, transport string, data any) *TransportEventImpl {
	return &TransportEventImpl{
		Type:      eventType,
		Timestamp: time.Now(),
		Transport: transport,
		Data:      data,
	}
}

// NewTransportErrorEvent creates a new transport error event.
func NewTransportErrorEvent(transport string, err error) *TransportEventImpl {
	return &TransportEventImpl{
		Type:      EventTypeError,
		Timestamp: time.Now(),
		Transport: transport,
		Error:     err,
	}
}