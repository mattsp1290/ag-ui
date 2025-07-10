package transport

import (
	"context"
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
	t.connected = true
	return nil
}

func (t *DemoTransport) Close() error {
	t.connected = false
	close(t.eventChan)
	close(t.errorChan)
	return nil
}

func (t *DemoTransport) Send(ctx context.Context, event TransportEvent) error {
	if !t.connected {
		return ErrNotConnected
	}
	// Echo the event back
	t.eventChan <- Event{
		Event:     event,
		Metadata:  EventMetadata{TransportID: "demo"},
		Timestamp: time.Now(),
	}
	return nil
}

func (t *DemoTransport) Receive() <-chan Event {
	return t.eventChan
}

func (t *DemoTransport) Errors() <-chan error {
	return t.errorChan
}

func (t *DemoTransport) IsConnected() bool {
	return t.connected
}

func (t *DemoTransport) Capabilities() Capabilities {
	return t.capabilities
}

func (t *DemoTransport) Health(ctx context.Context) error {
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
	err = transport.Close()
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
	defer manager.Stop()
	
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