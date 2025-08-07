package transport

import (
	"time"
)

// SimpleTransportEvent is a basic implementation of TransportEvent
type SimpleTransportEvent struct {
	EventID        string
	EventType      string
	EventTimestamp time.Time
	EventData      map[string]interface{}
}

// ID returns the event ID
func (e *SimpleTransportEvent) ID() string {
	return e.EventID
}

// Type returns the event type
func (e *SimpleTransportEvent) Type() string {
	return e.EventType
}

// Timestamp returns the event timestamp
func (e *SimpleTransportEvent) Timestamp() time.Time {
	return e.EventTimestamp
}

// Data returns the event data
func (e *SimpleTransportEvent) Data() map[string]interface{} {
	if e.EventData == nil {
		return make(map[string]interface{})
	}
	return e.EventData
}
