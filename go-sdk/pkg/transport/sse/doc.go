// Package sse provides Server-Sent Events (SSE) transport implementation for the AG-UI protocol.
//
// This package implements RFC-compliant SSE transport for real-time streaming of AG-UI events
// between clients and servers. It supports bidirectional communication with events sent via
// HTTP POST requests and received via SSE streams.
//
// # Features
//
// - RFC 6455 compliant SSE implementation
// - Automatic reconnection with exponential backoff
// - Event validation and type-safe parsing
// - CORS and security header support
// - Connection lifecycle management
// - Batch event sending for efficiency
// - Comprehensive error handling
// - Context-based cancellation and timeouts
// - Thread-safe concurrent operations
// - Built-in health checking and monitoring
//
// # Basic Usage
//
//	// Create transport with default configuration
//	transport, err := sse.NewSSETransport(nil)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer transport.Close(context.Background())
//
//	// Send an event
//	ctx := context.Background()
//	event := events.NewRunStartedEvent("thread-123", "run-456")
//	err = transport.Send(ctx, event)
//	if err != nil {
//		log.Printf("Send failed: %v", err)
//	}
//
//	// Receive events
//	eventChan, err := transport.Receive(ctx)
//	if err != nil {
//		log.Printf("Receive failed: %v", err)
//	}
//
//	for event := range eventChan {
//		fmt.Printf("Received: %s\n", event.Type())
//	}
//
// # Advanced Configuration
//
//	config := &sse.Config{
//		BaseURL:        "https://api.example.com",
//		Headers:        map[string]string{
//			"Authorization": "Bearer token",
//		},
//		BufferSize:     1000,
//		ReadTimeout:    30 * time.Second,
//		WriteTimeout:   10 * time.Second,
//		ReconnectDelay: 5 * time.Second,
//		MaxReconnects:  3,
//	}
//
//	transport, err := sse.NewSSETransport(config)
//	if err != nil {
//		log.Fatal(err)
//	}
//
// # Event Types
//
// The transport supports all AG-UI protocol event types:
//
// - Run events: RUN_STARTED, RUN_FINISHED, RUN_ERROR
// - Step events: STEP_STARTED, STEP_FINISHED
// - Message events: TEXT_MESSAGE_START, TEXT_MESSAGE_CONTENT, TEXT_MESSAGE_END
// - Tool events: TOOL_CALL_START, TOOL_CALL_ARGS, TOOL_CALL_END
// - State events: STATE_SNAPSHOT, STATE_DELTA, MESSAGES_SNAPSHOT
// - Custom events: RAW, CUSTOM
//
// # Error Handling
//
// The transport provides detailed error information using the core error types:
//
//	err := transport.Send(ctx, event)
//	if err != nil {
//		if messages.IsValidationError(err) {
//			log.Printf("Event validation failed: %v", err)
//		} else if messages.IsStreamingError(err) {
//			log.Printf("Transport error: %v", err)
//		}
//	}
//
// # Connection Management
//
// The transport handles connection lifecycle automatically:
//
//	// Check connection status
//	status := transport.GetConnectionStatus()
//	fmt.Printf("Status: %s\n", status)
//
//	// Test connectivity
//	err := transport.Ping(ctx)
//	if err != nil {
//		log.Printf("Server unreachable: %v", err)
//	}
//
//	// Get statistics
//	stats := transport.GetStats()
//	fmt.Printf("Reconnects: %d\n", stats.ReconnectCount)
//
// # Batch Operations
//
// For efficiency, multiple events can be sent in a single request:
//
//	events := []events.Event{
//		events.NewRunStartedEvent("thread-123", "run-456"),
//		events.NewStepStartedEvent("step-1"),
//		events.NewStepFinishedEvent("step-1"),
//		events.NewRunFinishedEvent("thread-123", "run-456"),
//	}
//
//	err := transport.SendBatch(ctx, events)
//	if err != nil {
//		log.Printf("Batch send failed: %v", err)
//	}
//
// # Server Implementation
//
// The package also provides utilities for server-side SSE implementation:
//
//	// Format event as SSE
//	sseData, err := sse.FormatSSEEvent(event)
//	if err != nil {
//		log.Printf("Format failed: %v", err)
//	}
//
//	// Write to HTTP response
//	w.Header().Set("Content-Type", "text/event-stream")
//	w.Header().Set("Cache-Control", "no-cache")
//	w.Header().Set("Connection", "keep-alive")
//
//	err = sse.WriteSSEEvent(w, event)
//	if err != nil {
//		log.Printf("Write failed: %v", err)
//	}
//
// # Security
//
// The transport supports standard HTTP security mechanisms:
//
//	transport.SetHeader("Authorization", "Bearer token")
//	transport.SetHeader("X-API-Key", "your-api-key")
//
// # Monitoring
//
// Monitor transport errors and connection issues:
//
//	go func() {
//		for err := range transport.GetErrorChannel() {
//			log.Printf("Transport error: %v", err)
//
//			// Implement custom error handling
//			if shouldRestart(err) {
//				transport.Reset()
//			}
//		}
//	}()
//
// # Thread Safety
//
// All transport methods are thread-safe and can be called concurrently:
//
//	// Safe to call from multiple goroutines
//	go transport.Send(ctx, event1)
//	go transport.Send(ctx, event2)
//
// # Performance
//
// The transport is optimized for high-throughput scenarios:
//
// - Efficient JSON parsing with minimal allocations
// - Configurable buffer sizes for memory management
// - Connection pooling and reuse
// - Batch operations for bulk data transfer
// - Streaming parser for low-latency event processing
//
// # Limitations
//
// - SSE is unidirectional (server to client) for event streaming
// - Events are sent to server via separate HTTP POST requests
// - Requires HTTP/1.1 or HTTP/2 for optimal performance
// - Browser EventSource API limitations apply for web clients
//
// # Compliance
//
// This implementation follows these standards:
//
// - RFC 6455: Server-Sent Events specification
// - RFC 7234: HTTP Caching (for cache-control headers)
// - AG-UI Protocol: Event format and validation rules
// - JSON RFC 7159: Event payload serialization
//
// # Testing
//
// The package includes comprehensive tests and examples:
//
//	go test ./pkg/transport/sse/...
//	go test -bench=. ./pkg/transport/sse/...
//
// See the test files for usage examples and performance benchmarks.
package sse
