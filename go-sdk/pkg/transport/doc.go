// Package transport provides a pluggable transport abstraction layer for the AG-UI protocol.
//
// This package implements a comprehensive transport abstraction that allows easy addition
// of new transport mechanisms, including transport factory, capability negotiation,
// automatic transport selection, and transport-specific configuration options.
//
// Key Features:
//   - Pluggable transport interface supporting HTTP/SSE, WebSocket, HTTP, and gRPC
//   - Automatic transport selection based on capabilities and performance
//   - Capability negotiation between client and server
//   - Transport factory and registry system for dynamic transport creation
//   - Middleware system for authentication, logging, metrics, retry, and compression
//   - Central transport manager with failover and load balancing
//   - Unified configuration management with environment variable support
//   - Health monitoring and performance metrics collection
//
// Architecture:
//
// The transport layer consists of several key components:
//
//   1. Transport Interface: Core abstraction for all transport implementations
//   2. Factory System: Creates and manages transport instances
//   3. Registry: Discovers and selects appropriate transports based on requirements
//   4. Capability System: Negotiates and matches transport capabilities
//   5. Configuration: Unified configuration management across all transports
//   6. Manager: Orchestrates transport operations, failover, and load balancing
//   7. Middleware: Pluggable middleware for cross-cutting concerns
//
// Supported Transport Types:
//   - WebSocket: Full-duplex, streaming, multiplexing, low-latency
//   - HTTP/SSE: Server-Sent Events for real-time streaming
//   - HTTP: Traditional request-response for simple interactions
//   - gRPC: High-performance RPC with streaming support
//
// Transport Selection:
//
// The transport manager automatically selects the best transport based on:
//   - Capability requirements (streaming, bidirectional, compression, etc.)
//   - Performance thresholds (latency, throughput)
//   - Transport priorities and availability
//   - Configuration preferences and fallback options
//
// Example usage:
//
//	import (
//		"github.com/ag-ui/go-sdk/pkg/transport"
//		"github.com/ag-ui/go-sdk/pkg/transport/config"
//		"github.com/ag-ui/go-sdk/pkg/transport/factory"
//		"github.com/ag-ui/go-sdk/pkg/transport/middleware"
//	)
//
//	// Create configuration
//	cfg := &config.Config{
//		Primary:  "websocket",
//		Fallback: []string{"sse", "http"},
//		Selection: config.SelectionConfig{
//			Strategy: "performance",
//			HealthCheckInterval: 30 * time.Second,
//			FailoverThreshold: 3,
//		},
//		Capabilities: config.CapabilityConfig{
//			Required: []string{"streaming", "bidirectional"},
//		},
//	}
//
//	// Create factory and registry
//	transportFactory := factory.New()
//	registry := factory.NewRegistry(transportFactory)
//
//	// Register transport implementations
//	transportFactory.Register(NewWebSocketFactory())
//	transportFactory.Register(NewHTTPSSEFactory())
//	transportFactory.Register(NewHTTPFactory())
//
//	// Create transport manager
//	manager := transport.NewManager(cfg, registry, transportFactory)
//
//	// Add middleware
//	manager.AddMiddleware(
//		middleware.NewLoggingMiddleware(logger),
//		middleware.NewMetricsMiddleware(),
//		middleware.NewRetryMiddleware(3, time.Second, 2.0),
//	)
//
//	// Start the manager
//	ctx := context.Background()
//	err := manager.Start(ctx)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer manager.Stop(ctx)
//
//	// Send events through the best available transport
//	err = manager.Send(ctx, event)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Receive events
//	go func() {
//		for event := range manager.Receive() {
//			log.Printf("Received: %s", event.Event.ID())
//		}
//	}()
//
// Configuration can also be loaded from files or environment variables:
//
//	// Load from YAML file
//	configManager := config.NewConfigManager()
//	err := configManager.LoadFromFile("transport.yaml")
//
//	// Or from environment variables
//	err := configManager.LoadFromEnvironment()
//
// Transport capabilities can be discovered automatically:
//
//	discovery := capabilities.NewHTTPDiscoveryService(httpClient)
//	caps, err := discovery.DiscoverCapabilities(ctx, "https://api.example.com")
//
// For more advanced usage, see the examples directory and integration tests.
package transport
