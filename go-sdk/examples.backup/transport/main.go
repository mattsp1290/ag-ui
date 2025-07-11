package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core"
	"github.com/ag-ui/go-sdk/pkg/transport"
	"github.com/ag-ui/go-sdk/pkg/transport/capabilities"
	"github.com/ag-ui/go-sdk/pkg/transport/config"
	"github.com/ag-ui/go-sdk/pkg/transport/factory"
	"github.com/ag-ui/go-sdk/pkg/transport/middleware"
)

// ExampleEvent implements a simple event for demonstration
type ExampleEvent struct {
	id        string
	eventType string
	data      map[string]interface{}
	timestamp time.Time
}

func NewExampleEvent(id, eventType string, data map[string]interface{}) *ExampleEvent {
	return &ExampleEvent{
		id:        id,
		eventType: eventType,
		data:      data,
		timestamp: time.Now(),
	}
}

func (e *ExampleEvent) ID() string                        { return e.id }
func (e *ExampleEvent) Type() string                      { return e.eventType }
func (e *ExampleEvent) Data() map[string]interface{}     { return e.data }
func (e *ExampleEvent) Timestamp() time.Time             { return e.timestamp }

// DemoTransport demonstrates a simple transport implementation
type DemoTransport struct {
	mu            sync.RWMutex
	name          string
	connected     bool
	capabilities  transport.Capabilities
	eventChan     chan transport.Event
	errorChan     chan error
	sentCount     uint64
	receivedCount uint64
	middleware    []transport.Middleware
}

func NewDemoTransport(name string, caps transport.Capabilities) *DemoTransport {
	return &DemoTransport{
		name:         name,
		capabilities: caps,
		eventChan:    make(chan transport.Event, 100),
		errorChan:    make(chan error, 100),
	}
}

func (t *DemoTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.connected {
		return transport.ErrAlreadyConnected
	}
	
	fmt.Printf("🔗 %s transport connecting...\n", t.name)
	
	// Simulate connection time
	time.Sleep(100 * time.Millisecond)
	
	t.connected = true
	fmt.Printf("✅ %s transport connected successfully\n", t.name)
	return nil
}

func (t *DemoTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if !t.connected {
		return transport.ErrNotConnected
	}
	
	fmt.Printf("🔌 %s transport closing...\n", t.name)
	t.connected = false
	close(t.eventChan)
	close(t.errorChan)
	fmt.Printf("✅ %s transport closed\n", t.name)
	return nil
}

func (t *DemoTransport) Send(ctx context.Context, event core.Event) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if !t.connected {
		return transport.ErrNotConnected
	}
	
	t.sentCount++
	fmt.Printf("📤 %s transport sent event: %s (type: %s)\n", t.name, event.ID(), event.Type())
	
	// Simulate sending event back as received (echo)
	go func() {
		time.Sleep(50 * time.Millisecond)
		t.eventChan <- transport.Event{
			Event: event,
			Metadata: transport.EventMetadata{
				TransportID: t.name,
				Headers:     map[string]string{"echo": "true"},
				Size:        1024,
				Latency:     50 * time.Millisecond,
			},
			Timestamp: time.Now(),
		}
	}()
	
	return nil
}

func (t *DemoTransport) Receive() <-chan transport.Event {
	return t.eventChan
}

func (t *DemoTransport) Errors() <-chan error {
	return t.errorChan
}

func (t *DemoTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

func (t *DemoTransport) Capabilities() transport.Capabilities {
	return t.capabilities
}

func (t *DemoTransport) Health(ctx context.Context) error {
	if !t.IsConnected() {
		return transport.ErrNotConnected
	}
	return nil
}

func (t *DemoTransport) Metrics() transport.Metrics {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	return transport.Metrics{
		ConnectionUptime:   time.Hour,
		MessagesSent:       t.sentCount,
		MessagesReceived:   t.receivedCount,
		BytesSent:          t.sentCount * 1024,
		BytesReceived:      t.receivedCount * 1024,
		ErrorCount:         0,
		AverageLatency:     50 * time.Millisecond,
		CurrentThroughput:  100.0,
		ReconnectCount:     0,
	}
}

func (t *DemoTransport) SetMiddleware(middleware ...transport.Middleware) {
	t.middleware = append(t.middleware, middleware...)
}

// DemoTransportFactory creates demo transports
type DemoTransportFactory struct {
	name string
	caps transport.Capabilities
}

func NewDemoTransportFactory(name string, caps transport.Capabilities) *DemoTransportFactory {
	return &DemoTransportFactory{name: name, caps: caps}
}

func (f *DemoTransportFactory) Name() string {
	return f.name
}

func (f *DemoTransportFactory) Create(ctx context.Context, config interface{}) (transport.Transport, error) {
	return NewDemoTransport(f.name, f.caps), nil
}

func (f *DemoTransportFactory) ValidateConfig(config interface{}) error {
	return nil
}

// ConsoleLogger implements a simple console logger
type ConsoleLogger struct{}

func (l *ConsoleLogger) Info(msg string, fields ...middleware.Field) {
	fmt.Printf("ℹ️  INFO: %s %s\n", msg, formatFields(fields))
}

func (l *ConsoleLogger) Warn(msg string, fields ...middleware.Field) {
	fmt.Printf("⚠️  WARN: %s %s\n", msg, formatFields(fields))
}

func (l *ConsoleLogger) Error(msg string, fields ...middleware.Field) {
	fmt.Printf("❌ ERROR: %s %s\n", msg, formatFields(fields))
}

func (l *ConsoleLogger) Debug(msg string, fields ...middleware.Field) {
	fmt.Printf("🐛 DEBUG: %s %s\n", msg, formatFields(fields))
}

func formatFields(fields []middleware.Field) string {
	if len(fields) == 0 {
		return ""
	}
	
	result := "["
	for i, field := range fields {
		if i > 0 {
			result += ", "
		}
		result += fmt.Sprintf("%s=%v", field.Key, field.Value)
	}
	result += "]"
	return result
}

func main() {
	fmt.Println("🚀 AG-UI Transport Abstraction Demo")
	fmt.Println("===================================")
	
	// Create configuration
	cfg := &config.Config{
		Primary:  "websocket",
		Fallback: []string{"http", "sse"},
		Selection: config.SelectionConfig{
			Strategy:            "performance",
			HealthCheckInterval: 10 * time.Second,
			FailoverThreshold:   3,
		},
		Capabilities: config.CapabilityConfig{
			Required:  []string{"streaming", "bidirectional"},
			Preferred: []string{"compression", "multiplexing"},
		},
		Performance: config.PerformanceConfig{
			LatencyThreshold:    100,
			ThroughputThreshold: 1000,
		},
		Global: config.GlobalConfig{
			BufferSize:    1000,
			EnableMetrics: true,
			EnableTracing: false,
		},
	}
	
	// Create factory and registry
	transportFactory := factory.New()
	registry := factory.NewRegistry(transportFactory)
	
	// Register demo transports with different capabilities
	websocketCaps := transport.Capabilities{
		Streaming:     true,
		Bidirectional: true,
		Multiplexing:  true,
		Reconnection:  true,
		Compression:   []transport.CompressionType{transport.CompressionGzip},
		Security:      []transport.SecurityFeature{transport.SecurityTLS},
		MaxMessageSize: 1024 * 1024, // 1MB
		ProtocolVersion: "1.0",
		Features: map[string]interface{}{
			"websocket_extensions": []string{"permessage-deflate"},
		},
	}
	
	httpCaps := transport.Capabilities{
		Streaming:     false,
		Bidirectional: false,
		Multiplexing:  false,
		Reconnection:  false,
		Compression:   []transport.CompressionType{transport.CompressionGzip},
		Security:      []transport.SecurityFeature{transport.SecurityTLS},
		MaxMessageSize: 512 * 1024, // 512KB
		ProtocolVersion: "1.0",
	}
	
	sseCaps := transport.Capabilities{
		Streaming:     true,
		Bidirectional: false,
		Multiplexing:  false,
		Reconnection:  true,
		Compression:   []transport.CompressionType{transport.CompressionGzip},
		Security:      []transport.SecurityFeature{transport.SecurityTLS},
		MaxMessageSize: 256 * 1024, // 256KB
		ProtocolVersion: "1.0",
	}
	
	// Register transport factories
	transportFactory.Register(NewDemoTransportFactory("websocket", websocketCaps))
	transportFactory.Register(NewDemoTransportFactory("http", httpCaps))
	transportFactory.Register(NewDemoTransportFactory("sse", sseCaps))
	
	// Register capabilities with the registry
	registry.RegisterCapabilities("websocket", websocketCaps)
	registry.RegisterCapabilities("http", httpCaps)
	registry.RegisterCapabilities("sse", sseCaps)
	
	// Set transport priorities
	registry.SetPriority("websocket", 10) // Highest priority
	registry.SetPriority("sse", 8)
	registry.SetPriority("http", 5)       // Lowest priority
	
	// Create transport manager
	manager := transport.NewManager(cfg, registry, transportFactory)
	
	// Add middleware
	logger := &ConsoleLogger{}
	loggingMiddleware := middleware.NewLoggingMiddleware(logger)
	metricsMiddleware := middleware.NewMetricsMiddleware()
	retryMiddleware := middleware.NewRetryMiddleware(3, 500*time.Millisecond, 2.0)
	
	manager.AddMiddleware(loggingMiddleware, metricsMiddleware, retryMiddleware)
	
	fmt.Println("\n📋 Configuration:")
	fmt.Printf("   Primary: %s\n", cfg.Primary)
	fmt.Printf("   Fallback: %v\n", cfg.Fallback)
	fmt.Printf("   Strategy: %s\n", cfg.Selection.Strategy)
	
	// Start the transport manager
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	fmt.Println("\n🎯 Starting transport manager...")
	err := manager.Start(ctx)
	if err != nil {
		log.Fatalf("Failed to start transport manager: %v", err)
	}
	defer manager.Stop()
	
	// Start event receiver
	go func() {
		fmt.Println("\n👂 Starting event receiver...")
		for {
			select {
			case event := <-manager.Receive():
				fmt.Printf("📥 Received event: %s (from %s, latency: %v)\n", 
					event.Event.ID(), 
					event.Metadata.TransportID, 
					event.Metadata.Latency)
			case err := <-manager.Errors():
				fmt.Printf("❌ Transport error: %v\n", err)
			case <-ctx.Done():
				return
			}
		}
	}()
	
	// Demonstrate capability negotiation
	fmt.Println("\n🤝 Demonstrating capability negotiation...")
	negotiator := capabilities.NewDefaultNegotiator(websocketCaps)
	
	result, err := negotiator.NegotiateCapabilities(ctx, websocketCaps, httpCaps)
	if err != nil {
		fmt.Printf("❌ Negotiation failed: %v\n", err)
	} else {
		fmt.Printf("✅ Negotiation successful:\n")
		fmt.Printf("   Agreed streaming: %v\n", result.Agreed.Streaming)
		fmt.Printf("   Agreed bidirectional: %v\n", result.Agreed.Bidirectional)
		fmt.Printf("   Agreed compression: %v\n", result.Agreed.Compression)
		fmt.Printf("   Conflicts: %d\n", len(result.Conflicts))
	}
	
	// Send some example events
	fmt.Println("\n📤 Sending example events...")
	
	events := []*ExampleEvent{
		NewExampleEvent("event-1", "user_action", map[string]interface{}{
			"action": "click",
			"button": "submit",
			"user_id": "user123",
		}),
		NewExampleEvent("event-2", "system_notification", map[string]interface{}{
			"type": "info",
			"message": "Process completed successfully",
		}),
		NewExampleEvent("event-3", "data_update", map[string]interface{}{
			"table": "users",
			"operation": "insert",
			"count": 5,
		}),
	}
	
	for i, event := range events {
		err := manager.Send(ctx, event)
		if err != nil {
			fmt.Printf("❌ Failed to send event %d: %v\n", i+1, err)
		}
		time.Sleep(500 * time.Millisecond) // Space out events
	}
	
	// Demonstrate transport switching
	fmt.Println("\n🔄 Demonstrating manual transport switching...")
	
	time.Sleep(1 * time.Second)
	
	fmt.Println("Switching to HTTP transport...")
	err = manager.SwitchTransport(ctx, "http")
	if err != nil {
		fmt.Printf("❌ Failed to switch to HTTP: %v\n", err)
	} else {
		fmt.Println("✅ Successfully switched to HTTP")
		
		// Send an event with the new transport
		event := NewExampleEvent("event-4", "transport_test", map[string]interface{}{
			"transport": "http",
			"message": "Testing HTTP transport",
		})
		
		err = manager.Send(ctx, event)
		if err != nil {
			fmt.Printf("❌ Failed to send via HTTP: %v\n", err)
		}
	}
	
	time.Sleep(1 * time.Second)
	
	fmt.Println("Switching back to WebSocket transport...")
	err = manager.SwitchTransport(ctx, "websocket")
	if err != nil {
		fmt.Printf("❌ Failed to switch to WebSocket: %v\n", err)
	} else {
		fmt.Println("✅ Successfully switched back to WebSocket")
	}
	
	// Display metrics
	fmt.Println("\n📊 Transport Metrics:")
	managerMetrics := manager.GetMetrics()
	fmt.Printf("   Total messages sent: %d\n", managerMetrics.TotalMessagesSent)
	fmt.Printf("   Transport switches: %d\n", managerMetrics.TransportSwitches)
	fmt.Printf("   Active connections: %d\n", managerMetrics.ActiveConnections)
	
	middlewareMetrics := metricsMiddleware.GetMetrics()
	fmt.Printf("   Connection attempts: %d\n", middlewareMetrics.ConnectionAttempts)
	fmt.Printf("   Send attempts: %d\n", middlewareMetrics.SendAttempts)
	fmt.Printf("   Average send duration: %v\n", middlewareMetrics.SendDuration/time.Duration(middlewareMetrics.SendAttempts))
	
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	fmt.Println("\n⏳ Demo running... Press Ctrl+C to stop")
	
	// Keep the demo running until interrupted
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	eventCounter := 5
	
	for {
		select {
		case <-sigChan:
			fmt.Println("\n👋 Shutting down gracefully...")
			return
		case <-ticker.C:
			// Send periodic events to keep the demo active
			eventCounter++
			event := NewExampleEvent(
				fmt.Sprintf("event-%d", eventCounter),
				"periodic",
				map[string]interface{}{
					"counter": eventCounter,
					"timestamp": time.Now().Unix(),
				},
			)
			
			err := manager.Send(ctx, event)
			if err != nil {
				fmt.Printf("❌ Failed to send periodic event: %v\n", err)
			}
		}
	}
}