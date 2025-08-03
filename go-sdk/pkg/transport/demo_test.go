package transport

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// DemoEvent implements TransportEvent for testing
// Deprecated: Use typed events with CreateConnectionEvent, CreateDataEvent, etc.
type DemoEvent struct {
	id        string
	eventType string
	timestamp time.Time
	data      map[string]interface{}
}

func (e *DemoEvent) ID() string                        { return e.id }
func (e *DemoEvent) Type() string                      { return e.eventType }
func (e *DemoEvent) Timestamp() time.Time             { return e.timestamp }
func (e *DemoEvent) Data() map[string]interface{}     { return e.data }

// DemoTransport implements Transport for testing
type DemoTransport struct {
	connected   bool
	capabilities Capabilities
	eventChan   chan events.Event
	errorChan   chan error
	closed      bool
	mu          sync.Mutex
	eventsSent  int64
}

func NewDemoTransport() *DemoTransport {
	// Use simplified capabilities
	caps := Capabilities{
		Streaming:        true,
		Bidirectional:    true,
		MaxMessageSize:   1024 * 1024,
		ProtocolVersion:  "1.0",
	}
	
	return &DemoTransport{
		capabilities: caps,
		eventChan: make(chan events.Event, 10),
		errorChan: make(chan error, 10),
	}
}

func (t *DemoTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// Check context before proceeding
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	t.connected = true
	return nil
}

func (t *DemoTransport) Close(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// Check context before proceeding
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	t.connected = false
	
	// Only close channels if not already closed
	if !t.closed {
		close(t.eventChan)
		close(t.errorChan)
		t.closed = true
	}
	return nil
}

func (t *DemoTransport) Send(ctx context.Context, event TransportEvent) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if !t.connected {
		return ErrNotConnected
	}
	
	if t.closed {
		return ErrConnectionClosed
	}
	
	// Check context before proceeding
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	// Echo the event back with context support
	// Convert TransportEvent to events.Event
	baseEvent := &events.BaseEvent{
		EventType: events.EventType(event.Type()),
	}
	baseEvent.SetTimestamp(event.Timestamp().UnixMilli())
	
	select {
	case t.eventChan <- baseEvent:
		t.eventsSent++
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errors.New("event channel full")
	}
}

func (t *DemoTransport) Receive() <-chan events.Event {
	return t.eventChan
}

func (t *DemoTransport) Errors() <-chan error {
	return t.errorChan
}

func (t *DemoTransport) Channels() (<-chan events.Event, <-chan error) {
	return t.eventChan, t.errorChan
}

func (t *DemoTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}

func (t *DemoTransport) Config() Config {
	return &BaseConfig{
		Type:           "demo",
		Endpoint:       "demo://localhost:8080",
		Timeout:        30 * time.Second,
		MaxMessageSize: 64 * 1024 * 1024,
	}
}

func (t *DemoTransport) Stats() TransportStats {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// Return realistic stats based on actual transport state
	if !t.connected {
		return TransportStats{
			EventsSent:       0,
			EventsReceived:   0,
			AverageLatency:   0,
		}
	}
	
	return TransportStats{
		ConnectedAt:      time.Now().Add(-time.Hour),
		EventsSent:       t.eventsSent,
		EventsReceived:   0,
		AverageLatency:   50 * time.Millisecond,
	}
}

// Health and Metrics functionality removed - not part of Transport interface

// SetMiddleware removed - not part of Transport interface

// TestTransportInterface tests that our interfaces work correctly
func TestTransportInterface(t *testing.T) {
	// Create demo transport
	transport := NewDemoTransport()
	
	// Test connection
	ctx := context.Background()
	err := transport.Connect(ctx)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	
	if !transport.IsConnected() {
		t.Error("Transport should be connected")
	}
	
	// Test config
	config := transport.Config()
	if config.GetType() != "demo" {
		t.Error("Expected demo transport type")
	}
	
	if config.GetEndpoint() != "demo://localhost:8080" {
		t.Error("Expected demo endpoint")
	}
	
	// Test sending events using type-safe API
	event := CreateConnectionEvent("test-1", "connected", 
		func(data *ConnectionEventData) {
			data.RemoteAddress = "demo://localhost:8080"
			data.Protocol = "demo"
			data.Version = "1.0"
		},
	)
	
	// Convert to legacy event for transport interface compatibility
	legacyEvent := NewTransportEventAdapter(event)
	
	err = transport.Send(ctx, legacyEvent)
	if err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}
	
	// Test receiving events
	select {
	case receivedEvent := <-transport.Receive():
		// Compare event types since BaseEvent doesn't have ID
		if receivedEvent.Type() != events.EventType(event.Type()) {
			t.Errorf("Expected event type %s, got %s", event.Type(), receivedEvent.Type())
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for event")
	}
	
	// Test stats
	stats := transport.Stats()
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	
	// Test stats
	stats = transport.Stats()
	if stats.EventsSent == 0 {
		t.Error("Expected non-zero events sent")
	}
	
	// Test close
	err = transport.Close(ctx)
	if err != nil {
		t.Fatalf("Failed to close transport: %v", err)
	}
	
	if transport.IsConnected() {
		t.Error("Transport should not be connected after close")
	}
}

// TestSimpleManager tests the simple manager
func TestSimpleManager(t *testing.T) {
	manager := NewSimpleManager()
	transport := NewDemoTransport()
	
	manager.SetTransport(transport)
	
	ctx := context.Background()
	err := manager.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop(ctx)
	
	// Test sending through manager using type-safe API
	dataEvent := CreateDataEvent("manager-test-1", []byte("hello from manager"),
		func(data *DataEventData) {
			data.ContentType = "text/plain"
			data.Encoding = "utf-8"
		},
	)
	
	// Convert to legacy event for manager interface compatibility
	event := NewTransportEventAdapter(dataEvent)
	
	err = manager.Send(ctx, event)
	if err != nil {
		t.Fatalf("Failed to send event through manager: %v", err)
	}
	
	// Test receiving through manager
	select {
	case receivedEvent := <-manager.Receive():
		// Compare event types since BaseEvent doesn't have ID
		if receivedEvent.Type() != events.EventType(event.Type()) {
			t.Errorf("Expected event type %s, got %s", event.Type(), receivedEvent.Type())
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for event through manager")
	}
}

// TestDemoTransportErrorPaths tests error scenarios with DemoTransport
func TestDemoTransportErrorPaths(t *testing.T) {
	t.Run("send_when_not_connected", func(t *testing.T) {
		transport := NewDemoTransport()
		// Create a typed error event for testing
		errorEvent := CreateErrorEvent("error-test-1", "test error",
			func(data *ErrorEventData) {
				data.Code = "TEST_ERROR"
				data.Severity = "error"
				data.Category = "transport"
				data.Retryable = false
			},
		)
		event := NewTransportEventAdapter(errorEvent)
		
		ctx := context.Background()
		err := transport.Send(ctx, event)
		if err != ErrNotConnected {
			t.Errorf("Expected ErrNotConnected, got %v", err)
		}
	})
	
	t.Run("stats_when_not_connected", func(t *testing.T) {
		transport := NewDemoTransport()
		
		// Test stats on disconnected transport
		stats := transport.Stats()
		if stats.EventsSent != 0 {
			t.Errorf("Expected 0 events sent for disconnected transport, got %d", stats.EventsSent)
		}
	})
	
	t.Run("double_close", func(t *testing.T) {
		transport := NewDemoTransport()
		ctx := context.Background()
		
		// Connect first
		if err := transport.Connect(ctx); err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		
		// Close once
		if err := transport.Close(ctx); err != nil {
			t.Fatalf("First close failed: %v", err)
		}
		
		// Close again - should not panic
		if err := transport.Close(ctx); err != nil {
			t.Fatalf("Second close failed: %v", err)
		}
	})
	
	t.Run("manager_lifecycle_errors", func(t *testing.T) {
		manager := NewSimpleManager()
		ctx := context.Background()
		
		// Stop without start
		err := manager.Stop(ctx)
		if err != nil {
			t.Errorf("Stop without start should not error, got %v", err)
		}
		
		// Send without transport using type-safe API
		errorEvent := CreateErrorEvent("test", "manager error test",
			func(data *ErrorEventData) {
				data.Code = "MANAGER_ERROR"
				data.Severity = "warning"
				data.Retryable = true
			},
		)
		event := NewTransportEventAdapter(errorEvent)
		err = manager.Send(ctx, event)
		if err != ErrNotConnected {
			t.Errorf("Expected ErrNotConnected, got %v", err)
		}
	})
	
	t.Run("concurrent_manager_operations", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewDemoTransport()
		manager.SetTransport(transport)
		
		ctx := context.Background()
		if err := manager.Start(ctx); err != nil {
			t.Fatalf("Failed to start: %v", err)
		}
		defer manager.Stop(ctx)
		
		// Launch concurrent operations
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				// Create typed data event for concurrent testing
				dataEvent := CreateDataEvent(fmt.Sprintf("concurrent-%d", id), 
					[]byte(fmt.Sprintf("concurrent message %d", id)),
					func(data *DataEventData) {
						data.ContentType = "text/plain"
						data.SequenceNumber = uint64(id)
					},
				)
				event := NewTransportEventAdapter(dataEvent)
				if err := manager.Send(ctx, event); err != nil {
					t.Errorf("Concurrent send %d failed: %v", id, err)
				}
			}(i)
		}
		
		wg.Wait()
	})
}