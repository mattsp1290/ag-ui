// Package transport provides a comprehensive transport abstraction layer for the AG-UI Go SDK.
//
// This package implements a robust, type-safe transport system that enables reliable,
// bidirectional communication between agents and front-end applications. The transport
// layer supports multiple protocols, connection management, error handling, and advanced
// features like streaming, compression, and security.
//
// Key Features:
//   - Type-safe transport interfaces with comprehensive validation
//   - Support for multiple transport protocols (WebSocket, HTTP, gRPC)
//   - Streaming and reliable transport capabilities
//   - Simplified capabilities system for basic transport characteristics
//   - Comprehensive error handling and circuit breaker patterns
//   - Middleware and interceptor support for cross-cutting concerns
//   - Transport manager with load balancing and failover
//   - Health checking and performance monitoring
//   - Security features including TLS, JWT, API keys, and OAuth2
//
// Core Interfaces:
//
// The transport layer is built around several key interfaces:
//
//   1. Transport: Basic transport operations (connect, send, receive, close)
//   2. StreamingTransport: Real-time bidirectional streaming capabilities
//   3. ReliableTransport: Guaranteed delivery with acknowledgments and retries
//   4. TransportManager: Manages multiple transports with load balancing
//   5. Config: Type-safe configuration with validation
//   6. Middleware: Interceptors for cross-cutting concerns
//
// Basic Capabilities:
//
// The simplified capabilities system provides basic transport characteristics:
//   - Streaming: Whether the transport supports streaming
//   - Bidirectional: Whether the transport supports bidirectional communication
//   - MaxMessageSize: Maximum message size supported
//   - ProtocolVersion: Version of the transport protocol
//
// Transport Protocols:
//   - WebSocket: Full-duplex, real-time communication
//   - HTTP: Request-response with SSE support for streaming
//   - gRPC: High-performance RPC with bidirectional streaming
//   - Mock: Testing and development transport
//
// Advanced Features:
//   - Circuit breakers for fault tolerance
//   - Automatic reconnection with exponential backoff
//   - Event filtering and middleware chains
//   - Comprehensive metrics and health checking
//   - Load balancing strategies (round-robin, failover, performance-based)
//
// Basic Transport Usage:
//
//	import (
//		"context"
//		"time"
//		"github.com/ag-ui/go-sdk/pkg/transport"
//		"github.com/ag-ui/go-sdk/pkg/core/events"
//	)
//
//	// Create type-safe configuration
//	config := &transport.BasicConfig{
//		Type:     "websocket",
//		Endpoint: "ws://localhost:8080/ws",
//		Timeout:  30 * time.Second,
//		Headers: map[string]string{
//			"Authorization": "Bearer token123",
//		},
//		Secure: true,
//	}
//
//	// Validate configuration
//	if err := config.Validate(); err != nil {
//		log.Fatalf("Invalid config: %v", err)
//	}
//
//	// Create transport
//	transport := transport.NewWebSocketTransport(config)
//
//	// Connect
//	ctx := context.Background()
//	if err := transport.Connect(ctx); err != nil {
//		log.Fatalf("Connection failed: %v", err)
//	}
//	defer transport.Close(ctx)
//
//	// Send type-safe events
//	event := events.NewRunStartedEvent("thread-123", "run-456")
//	if err := transport.Send(ctx, event); err != nil {
//		log.Printf("Send failed: %v", err)
//	}
//
//	// Receive events
//	go func() {
//		for event := range transport.Receive() {
//			log.Printf("Received: %s", event.GetEventType())
//		}
//	}()
//
//	// Handle errors
//	go func() {
//		for err := range transport.Errors() {
//			log.Printf("Transport error: %v", err)
//		}
//	}()
//
// Basic Capabilities:
//
//	// Create simple capabilities
//	capabilities := transport.Capabilities{
//		Streaming:        true,
//		Bidirectional:    true,
//		MaxMessageSize:   1024 * 1024,
//		ProtocolVersion:  "1.0",
//	}
//
//	// Capabilities are used for basic transport characteristics
//	// and can be extended via the Extensions field for custom features
//
// Streaming Transport:
//
//	streamingTransport := transport.NewGRPCStreamingTransport(config)
//	
//	// Start bidirectional streaming
//	send, receive, errors, err := streamingTransport.StartStreaming(ctx)
//	if err != nil {
//		log.Fatalf("Streaming failed: %v", err)
//	}
//
//	// Send events via channel
//	go func() {
//		event := events.NewTextMessageContentEvent("msg-123", "Hello")
//		send <- event
//	}()
//
//	// Batch sending for performance
//	events := []transport.TransportEvent{
//		events.NewStepStartedEvent("step-1"),
//		events.NewStepFinishedEvent("step-1"),
//	}
//	err = streamingTransport.SendBatch(ctx, events)
//
// Transport Manager with Load Balancing:
//
//	manager := transport.NewTransportManager()
//	
//	// Add multiple transports
//	manager.AddTransport("primary", primaryTransport)
//	manager.AddTransport("backup", backupTransport)
//	
//	// Configure load balancer
//	manager.SetLoadBalancer(transport.NewFailoverLoadBalancer())
//	
//	// Send using best available transport
//	err = manager.SendEvent(ctx, event)
//
// For comprehensive examples, see the examples/ directory and API documentation.
package transport
