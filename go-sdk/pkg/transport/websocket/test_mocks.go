package websocket

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/proto/generated"
)

// MockEvent implements events.Event for websocket testing
type MockEvent struct {
	EventType     events.EventType
	Data          interface{}
	timestampMs   *int64
}

func (e *MockEvent) ID() string {
	return fmt.Sprintf("mock-%d", time.Now().UnixNano())
}

func (e *MockEvent) Type() events.EventType {
	return e.EventType
}

func (e *MockEvent) Timestamp() *int64 {
	if e.timestampMs == nil {
		timestamp := time.Now().UnixMilli()
		e.timestampMs = &timestamp
	}
	return e.timestampMs
}

func (e *MockEvent) SetTimestamp(timestamp int64) {
	e.timestampMs = &timestamp
}

func (e *MockEvent) Validate() error {
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
	if e.timestampMs == nil {
		timestamp := time.Now().UnixMilli()
		e.timestampMs = &timestamp
	}
	return &events.BaseEvent{
		EventType:   e.EventType,
		TimestampMs: e.timestampMs,
	}
}