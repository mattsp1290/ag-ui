package transport

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// DemoEvent implements TransportEvent for testing
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
	eventChan   chan Event
	errorChan   chan error
	closed      bool
	mu          sync.Mutex
}

func NewDemoTransport() *DemoTransport {
	return &DemoTransport{
		capabilities: Capabilities{
			Streaming:     true,
			Bidirectional: true,
			Compression:   []CompressionType{CompressionGzip},
			Security:      []SecurityFeature{SecurityTLS},
		},
		eventChan: make(chan Event, 10),
		errorChan: make(chan error, 10),
	}
}

func (t *DemoTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connected = true
	return nil
}

func (t *DemoTransport) Close(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
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
	
	// Echo the event back
	select {
	case t.eventChan <- Event{
		Event:     event,
		Metadata:  EventMetadata{TransportID: "demo"},
		Timestamp: time.Now(),
	}:
		return nil
	default:
		return errors.New("event channel full")
	}
}

func (t *DemoTransport) Receive() <-chan Event {
	return t.eventChan
}

func (t *DemoTransport) Errors() <-chan error {
	return t.errorChan
}

func (t *DemoTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}

func (t *DemoTransport) Capabilities() Capabilities {
	return t.capabilities
}

func (t *DemoTransport) Health(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if !t.connected {
		return ErrNotConnected
	}
	return nil
}

func (t *DemoTransport) Metrics() Metrics {
	return Metrics{
		ConnectionUptime:  time.Hour,
		MessagesSent:      10,
		MessagesReceived:  5,
		AverageLatency:    50 * time.Millisecond,
		CurrentThroughput: 100.0,
	}
}

func (t *DemoTransport) SetMiddleware(middleware ...Middleware) {
	// No-op for demo
}

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
	
	// Test capabilities
	caps := transport.Capabilities()
	if !caps.Streaming {
		t.Error("Expected streaming capability")
	}
	
	if !caps.Bidirectional {
		t.Error("Expected bidirectional capability")
	}
	
	// Test sending events
	event := &DemoEvent{
		id:        "test-1",
		eventType: "demo",
		timestamp: time.Now(),
		data:      map[string]interface{}{"message": "hello"},
	}
	
	err = transport.Send(ctx, event)
	if err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}
	
	// Test receiving events
	select {
	case receivedEvent := <-transport.Receive():
		if receivedEvent.Event.ID() != event.ID() {
			t.Errorf("Expected event ID %s, got %s", event.ID(), receivedEvent.Event.ID())
		}
		if receivedEvent.Metadata.TransportID != "demo" {
			t.Errorf("Expected transport ID 'demo', got %s", receivedEvent.Metadata.TransportID)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for event")
	}
	
	// Test health check
	err = transport.Health(ctx)
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	
	// Test metrics
	metrics := transport.Metrics()
	if metrics.MessagesSent == 0 {
		t.Error("Expected non-zero messages sent")
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
	
	// Test sending through manager
	event := &DemoEvent{
		id:        "manager-test-1",
		eventType: "demo",
		timestamp: time.Now(),
		data:      map[string]interface{}{"message": "hello from manager"},
	}
	
	err = manager.Send(ctx, event)
	if err != nil {
		t.Fatalf("Failed to send event through manager: %v", err)
	}
	
	// Test receiving through manager
	select {
	case receivedEvent := <-manager.Receive():
		if receivedEvent.Event.ID() != event.ID() {
			t.Errorf("Expected event ID %s, got %s", event.ID(), receivedEvent.Event.ID())
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for event through manager")
	}
}

// TestDemoTransportErrorPaths tests error scenarios with DemoTransport
func TestDemoTransportErrorPaths(t *testing.T) {
	t.Run("send_when_not_connected", func(t *testing.T) {
		transport := NewDemoTransport()
		event := &DemoEvent{
			id:        "error-test-1",
			eventType: "demo",
			timestamp: time.Now(),
		}
		
		ctx := context.Background()
		err := transport.Send(ctx, event)
		if err != ErrNotConnected {
			t.Errorf("Expected ErrNotConnected, got %v", err)
		}
	})
	
	t.Run("health_check_when_not_connected", func(t *testing.T) {
		transport := NewDemoTransport()
		ctx := context.Background()
		
		err := transport.Health(ctx)
		if err != ErrNotConnected {
			t.Errorf("Expected ErrNotConnected, got %v", err)
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
		
		// Send without transport
		event := &DemoEvent{id: "test", eventType: "demo"}
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
				event := &DemoEvent{
					id:        fmt.Sprintf("concurrent-%d", id),
					eventType: "demo",
					timestamp: time.Now(),
				}
				if err := manager.Send(ctx, event); err != nil {
					t.Errorf("Concurrent send %d failed: %v", id, err)
				}
			}(i)
		}
		
		wg.Wait()
	})
}