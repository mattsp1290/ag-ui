package transport

import (
	"context"
	"testing"
	"time"
)

// MockEvent implements TransportEvent for testing
// Deprecated: Use typed events with CreateDataEvent, CreateConnectionEvent, etc.
type MockEvent struct {
	id   string
	typ  string
	data map[string]interface{}
}

func (e *MockEvent) ID() string                      { return e.id }
func (e *MockEvent) Type() string                    { return e.typ }
func (e *MockEvent) Timestamp() time.Time            { return time.Now() }
func (e *MockEvent) Data() map[string]interface{}    { return e.data }

// createTypedTestEvent creates a type-safe test event for backpressure testing
func createTypedTestEvent(id string) Event {
	// Use type-safe data event creation
	dataEvent := CreateDataEvent(id, []byte("backpressure test data"),
		func(data *DataEventData) {
			data.ContentType = "text/plain"
			data.Encoding = "utf-8"
		},
	)
	
	return Event{Event: NewTransportEventAdapter(dataEvent)}
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
	event1 := createTypedTestEvent("1")
	event2 := createTypedTestEvent("2")
	event3 := createTypedTestEvent("3")
	
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
		if receivedEvent.Event.ID() != "2" {
			t.Errorf("Expected event ID '2', got '%s'", receivedEvent.Event.ID())
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}
	
	select {
	case receivedEvent := <-handler.EventChan():
		if receivedEvent.Event.ID() != "3" {
			t.Errorf("Expected event ID '3', got '%s'", receivedEvent.Event.ID())
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
	event1 := createTypedTestEvent("1")
	event2 := createTypedTestEvent("2")
	event3 := createTypedTestEvent("3")
	
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
		if receivedEvent.Event.ID() != "1" {
			t.Errorf("Expected event ID '1', got '%s'", receivedEvent.Event.ID())
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}
	
	select {
	case receivedEvent := <-handler.EventChan():
		if receivedEvent.Event.ID() != "2" {
			t.Errorf("Expected event ID '2', got '%s'", receivedEvent.Event.ID())
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
	event1 := createTypedTestEvent("1")
	event2 := createTypedTestEvent("2")
	
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
	event1 := createTypedTestEvent("1")
	event2 := createTypedTestEvent("2")
	
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
	config := &Config{
		Primary:     "websocket",
		Fallback:    []string{"sse", "http"},
		BufferSize:  1024,
		LogLevel:    "info",
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
		event := Event{Event: &MockEvent{id: string(rune('1' + i)), typ: "test", data: nil}}
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