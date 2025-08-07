package transport

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/proto/generated"
)

// BackpressureMockEvent implements TransportEvent for testing
// Deprecated: Use typed events with CreateDataEvent, CreateConnectionEvent, etc.
type BackpressureMockEvent struct {
	id   string
	typ  string
	data map[string]interface{}
}

func (e *BackpressureMockEvent) ID() string                   { return e.id }
func (e *BackpressureMockEvent) Type() string                 { return e.typ }
func (e *BackpressureMockEvent) Timestamp() time.Time         { return time.Now() }
func (e *BackpressureMockEvent) Data() map[string]interface{} { return e.data }

// BackpressureMockCoreEvent implements events.Event for testing
type BackpressureMockCoreEvent struct {
	*events.BaseEvent
	id string // for test tracking
}

func (e *BackpressureMockCoreEvent) Validate() error { return nil }
func (e *BackpressureMockCoreEvent) ToJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"type":      e.Type(),
		"timestamp": e.Timestamp(),
		"id":        e.id,
	})
}
func (e *BackpressureMockCoreEvent) ToProtobuf() (*generated.Event, error) { return nil, nil }
func (e *BackpressureMockCoreEvent) GetBaseEvent() *events.BaseEvent       { return e.BaseEvent }

// createCoreTestEvent creates a core event for backpressure testing
func createCoreTestEvent(id string) *BackpressureMockCoreEvent {
	timestamp := time.Now().UnixMilli()
	return &BackpressureMockCoreEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeCustom,
			TimestampMs: &timestamp,
		},
		id: id,
	}
}

func TestBackpressureHandler_DropOldest(t *testing.T) {
	config := BackpressureConfig{
		Strategy:      BackpressureDropOldest,
		BufferSize:    2,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  time.Second,
		EnableMetrics: true,
	}

	handler := NewBackpressureHandler(config)
	defer handler.Stop()

	// Fill the buffer using type-safe events
	event1 := createCoreTestEvent("1")
	event2 := createCoreTestEvent("2")
	event3 := createCoreTestEvent("3")

	// Send first two events - should succeed
	err := handler.SendEvent(event1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = handler.SendEvent(event2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Send third event - should drop oldest
	err = handler.SendEvent(event3)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify we get event2 and event3 (event1 should be dropped)
	select {
	case receivedEvent := <-handler.EventChan():
		mockEvent := receivedEvent.(*BackpressureMockCoreEvent)
		if mockEvent.id != "2" {
			t.Errorf("Expected event ID '2', got '%s'", mockEvent.id)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}

	select {
	case receivedEvent := <-handler.EventChan():
		mockEvent := receivedEvent.(*BackpressureMockCoreEvent)
		if mockEvent.id != "3" {
			t.Errorf("Expected event ID '3', got '%s'", mockEvent.id)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}

	// Verify metrics
	metrics := handler.GetMetrics()
	if metrics.EventsDropped != 1 {
		t.Errorf("Expected 1 dropped event, got %d", metrics.EventsDropped)
	}
}

func TestBackpressureHandler_DropNewest(t *testing.T) {
	config := BackpressureConfig{
		Strategy:      BackpressureDropNewest,
		BufferSize:    2,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  time.Second,
		EnableMetrics: true,
	}

	handler := NewBackpressureHandler(config)
	defer handler.Stop()

	// Fill the buffer using type-safe events
	event1 := createCoreTestEvent("1")
	event2 := createCoreTestEvent("2")
	event3 := createCoreTestEvent("3")

	// Send first two events - should succeed
	err := handler.SendEvent(event1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	err = handler.SendEvent(event2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Send third event - should drop newest (event3)
	err = handler.SendEvent(event3)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify we get event1 and event2 (event3 should be dropped)
	select {
	case receivedEvent := <-handler.EventChan():
		mockEvent := receivedEvent.(*BackpressureMockCoreEvent)
		if mockEvent.id != "1" {
			t.Errorf("Expected event ID '1', got '%s'", mockEvent.id)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}

	select {
	case receivedEvent := <-handler.EventChan():
		mockEvent := receivedEvent.(*BackpressureMockCoreEvent)
		if mockEvent.id != "2" {
			t.Errorf("Expected event ID '2', got '%s'", mockEvent.id)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}

	// Verify metrics
	metrics := handler.GetMetrics()
	if metrics.EventsDropped != 1 {
		t.Errorf("Expected 1 dropped event, got %d", metrics.EventsDropped)
	}
}

func TestBackpressureHandler_BlockTimeout(t *testing.T) {
	config := BackpressureConfig{
		Strategy:      BackpressureBlockWithTimeout,
		BufferSize:    1,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  100 * time.Millisecond,
		EnableMetrics: true,
	}

	handler := NewBackpressureHandler(config)
	defer handler.Stop()

	// Fill the buffer using type-safe events
	event1 := createCoreTestEvent("1")
	event2 := createCoreTestEvent("2")

	// Send first event - should succeed
	err := handler.SendEvent(event1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Send second event - should timeout
	start := time.Now()
	err = handler.SendEvent(event2)
	elapsed := time.Since(start)

	if err != ErrBackpressureTimeout {
		t.Errorf("Expected timeout error, got %v", err)
	}

	// Should have taken at least the timeout duration
	if elapsed < 100*time.Millisecond {
		t.Errorf("Expected timeout after 100ms, but took %v", elapsed)
	}

	// Verify metrics
	metrics := handler.GetMetrics()
	if metrics.EventsBlocked != 1 {
		t.Errorf("Expected 1 blocked event, got %d", metrics.EventsBlocked)
	}
}

func TestBackpressureHandler_None(t *testing.T) {
	config := BackpressureConfig{
		Strategy:      BackpressureNone,
		BufferSize:    1,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  time.Second,
		EnableMetrics: true,
	}

	handler := NewBackpressureHandler(config)
	defer handler.Stop()

	// Fill the buffer using type-safe events
	event1 := createCoreTestEvent("1")
	event2 := createCoreTestEvent("2")

	// Send first event - should succeed
	err := handler.SendEvent(event1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Send second event - should fail immediately
	err = handler.SendEvent(event2)
	if err != ErrBackpressureActive {
		t.Errorf("Expected backpressure active error, got %v", err)
	}
}

func TestSimpleManagerWithBackpressure(t *testing.T) {
	config := BackpressureConfig{
		Strategy:      BackpressureDropOldest,
		BufferSize:    2,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  time.Second,
		EnableMetrics: true,
	}

	manager := NewSimpleManagerWithBackpressure(config)
	defer manager.Stop(context.Background())

	// Test that manager returns the correct channels
	eventChan := manager.Receive()
	errorChan := manager.Errors()

	if eventChan == nil {
		t.Fatal("Expected non-nil event channel")
	}

	if errorChan == nil {
		t.Fatal("Expected non-nil error channel")
	}

	// Test backpressure metrics
	metrics := manager.GetBackpressureMetrics()
	if metrics.MaxBufferSize != 2 {
		t.Errorf("Expected max buffer size 2, got %d", metrics.MaxBufferSize)
	}
}

func TestFullManagerWithBackpressure(t *testing.T) {
	config := &ManagerConfig{
		Primary:       "websocket",
		Fallback:      []string{"sse", "http"},
		BufferSize:    1024,
		LogLevel:      "info",
		EnableMetrics: true,
		Backpressure: BackpressureConfig{
			Strategy:      BackpressureDropOldest,
			BufferSize:    2,
			HighWaterMark: 0.8,
			LowWaterMark:  0.2,
			BlockTimeout:  time.Second,
			EnableMetrics: true,
		},
	}

	manager := NewManager(config)
	defer manager.Stop(context.Background())

	// Test that manager returns the correct channels
	eventChan := manager.Receive()
	errorChan := manager.Errors()

	if eventChan == nil {
		t.Fatal("Expected non-nil event channel")
	}

	if errorChan == nil {
		t.Fatal("Expected non-nil error channel")
	}

	// Test backpressure metrics
	metrics := manager.GetBackpressureMetrics()
	if metrics.MaxBufferSize != 2 {
		t.Errorf("Expected max buffer size 2, got %d", metrics.MaxBufferSize)
	}
}

func TestBackpressureMetrics(t *testing.T) {
	config := BackpressureConfig{
		Strategy:      BackpressureDropOldest,
		BufferSize:    2,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  time.Second,
		EnableMetrics: true,
	}

	handler := NewBackpressureHandler(config)
	defer handler.Stop()

	// Get initial metrics
	metrics := handler.GetMetrics()
	if metrics.MaxBufferSize != 2 {
		t.Errorf("Expected max buffer size 2, got %d", metrics.MaxBufferSize)
	}

	if metrics.EventsDropped != 0 {
		t.Errorf("Expected 0 dropped events initially, got %d", metrics.EventsDropped)
	}

	// Send events to trigger drops
	for i := 0; i < 5; i++ {
		event := createCoreTestEvent(string(rune('1' + i)))
		handler.SendEvent(event)
	}

	// Check metrics after dropping events
	metrics = handler.GetMetrics()
	if metrics.EventsDropped == 0 {
		t.Error("Expected some dropped events, got 0")
	}

	if metrics.LastDropTime.IsZero() {
		t.Error("Expected non-zero last drop time")
	}
}
