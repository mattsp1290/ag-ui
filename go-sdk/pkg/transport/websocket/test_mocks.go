package websocket

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/proto/generated"
)

// MockEvent implements events.Event for websocket testing
type MockEvent struct {
	EventType      events.EventType
	Data           interface{}
	TimestampMs    *int64  // Exported field for test initialization
	timestampMs    *int64  // Internal field for compatibility
	ValidationFunc func() error // Optional validation function for testing
}

func (e *MockEvent) ID() string {
	return fmt.Sprintf("mock-%d", time.Now().UnixNano())
}

func (e *MockEvent) Type() events.EventType {
	return e.EventType
}

func (e *MockEvent) Timestamp() *int64 {
	// Prioritize the exported field if set
	if e.TimestampMs != nil {
		e.timestampMs = e.TimestampMs
		return e.TimestampMs
	}
	
	// Return nil if no timestamp was explicitly set
	// This allows testing of validation logic that requires timestamps
	return e.timestampMs
}

func (e *MockEvent) SetTimestamp(timestamp int64) {
	e.timestampMs = &timestamp
}

func (e *MockEvent) Validate() error {
	if e.ValidationFunc != nil {
		return e.ValidationFunc()
	}
	return nil
}

func (e *MockEvent) ToJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":        e.ID(),
		"type":      e.Type(),
		"timestamp": e.Timestamp(),
		"data":      e.Data,
	})
}

func (e *MockEvent) ToProtobuf() (*generated.Event, error) {
	return nil, nil
}

func (e *MockEvent) GetBaseEvent() *events.BaseEvent {
	// Use the Timestamp() method which handles both fields correctly
	timestamp := e.Timestamp()
	return &events.BaseEvent{
		EventType:   e.EventType,
		TimestampMs: timestamp,
	}
}

func (e *MockEvent) ThreadID() string {
	return "mock-thread-id"
}

func (e *MockEvent) RunID() string {
	return "mock-run-id"
}