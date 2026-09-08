package events

import (
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
)

// EventDecoder handles decoding of SSE events to Go SDK event types.
type EventDecoder struct {
	logger *logrus.Logger
}

// NewEventDecoder creates a new event decoder.
func NewEventDecoder(logger *logrus.Logger) *EventDecoder {
	if logger == nil {
		logger = logrus.New()
	}
	return &EventDecoder{logger: logger}
}

// DecodeEvent decodes a raw SSE event into the concrete type selected by
// eventName. The payload's type field remains authoritative for Event.Type.
// Payloads without base fields retain their legacy nil BaseEvent.
func (ed *EventDecoder) DecodeEvent(eventName string, data []byte) (Event, error) {
	eventType := EventType(eventName)
	constructor, ok := eventConstructors[eventType]
	if !ok {
		ed.logger.WithField("event", eventName).Warn("Unknown event type")
		return nil, fmt.Errorf("unknown event type: %s", eventName)
	}
	// Use the shared factory mapping without preinitializing the base. Some
	// legacy payloads omit all base fields and historically keep a nil base.
	event := constructor(nil)

	if err := json.Unmarshal(data, event); err != nil {
		return nil, fmt.Errorf("failed to decode %s: %w", eventType, err)
	}
	return event, nil
}
