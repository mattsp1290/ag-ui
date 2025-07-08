package sse

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/messages"
)

// Example demonstrates how to use the connection management system
func ExampleConnectionUsage() {
	// Create configuration
	config := DefaultConfig()
	config.BaseURL = "http://localhost:8080"
	config.ReconnectDelay = 1 * time.Second
	config.MaxReconnects = 5

	// Add authentication headers
	config.Headers["Authorization"] = "Bearer your-token-here"
	config.Headers["X-API-Key"] = "your-api-key"

	// Create a single connection
	conn, err := NewConnection(config, nil)
	if err != nil {
		log.Fatalf("Failed to create connection: %v", err)
	}
	defer conn.Close()

	// Set up event handlers
	go handleConnectionEvents(conn)
	go handleConnectionErrors(conn)
	go handleStateChanges(conn)

	// Attempt to connect
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := conn.Connect(ctx); err != nil {
		log.Printf("Failed to connect: %v", err)
		// Connection will attempt to reconnect automatically if enabled
	}

	// Monitor connection for a while
	time.Sleep(30 * time.Second)

	// Print connection metrics
	printConnectionMetrics(conn)
}

// ExampleConnectionPoolUsage demonstrates connection pooling
func ExampleConnectionPoolUsage() {
	// Create configuration
	config := DefaultConfig()
	config.BaseURL = "http://localhost:8080"
	config.Headers["Authorization"] = "Bearer your-token-here"

	// Create connection pool
	pool, err := NewConnectionPool(config)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	// Acquire connections from the pool
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		conn, err := pool.AcquireConnection(ctx)
		if err != nil {
			log.Printf("Failed to acquire connection %d: %v", i, err)
			continue
		}

		// Use the connection
		go func(c *Connection, index int) {
			defer pool.ReleaseConnection(c)

			log.Printf("Using connection %d: %s", index, c.ID())

			// Set up event handling
			go handleConnectionEvents(c)

			// Simulate work
			time.Sleep(10 * time.Second)

			log.Printf("Finished using connection %d", index)
		}(conn, i)
	}

	// Monitor pool for a while
	go monitorPool(pool)
	time.Sleep(30 * time.Second)

	// Print pool statistics
	printPoolStats(pool)
}

// ExampleReconnectionHandling demonstrates reconnection behavior
func ExampleReconnectionHandling() {
	config := DefaultConfig()
	config.BaseURL = "http://localhost:8080"

	conn, err := NewConnection(config, nil)
	if err != nil {
		log.Fatalf("Failed to create connection: %v", err)
	}
	defer conn.Close()

	// Customize reconnection policy
	conn.reconnectPolicy.MaxAttempts = 3
	conn.reconnectPolicy.InitialDelay = 500 * time.Millisecond
	conn.reconnectPolicy.MaxDelay = 10 * time.Second
	conn.reconnectPolicy.BackoffMultiplier = 2.0
	conn.reconnectPolicy.JitterFactor = 0.1

	// Monitor state changes
	go func() {
		for state := range conn.ReadStateChanges() {
			log.Printf("Connection state changed to: %s", state.String())

			switch state {
			case ConnectionStateConnected:
				log.Println("✅ Connection established successfully")
			case ConnectionStateReconnecting:
				log.Println("🔄 Attempting to reconnect...")
			case ConnectionStateError:
				log.Println("❌ Connection error occurred")
			case ConnectionStateClosed:
				log.Println("🔒 Connection closed")
				return
			}
		}
	}()

	// Attempt initial connection
	ctx := context.Background()
	if err := conn.Connect(ctx); err != nil {
		log.Printf("Initial connection failed: %v", err)
	}

	// Simulate network interruption and recovery
	time.Sleep(5 * time.Second)
	conn.Disconnect() // Simulate network interruption

	// Connection will attempt to reconnect automatically
	time.Sleep(15 * time.Second)

	printConnectionMetrics(conn)
}

// ExampleHeartbeatMonitoring demonstrates heartbeat functionality
func ExampleHeartbeatMonitoring() {
	config := DefaultConfig()
	config.BaseURL = "http://localhost:8080"

	conn, err := NewConnection(config, nil)
	if err != nil {
		log.Fatalf("Failed to create connection: %v", err)
	}
	defer conn.Close()

	// Customize heartbeat configuration
	conn.heartbeatConfig.Enabled = true
	conn.heartbeatConfig.Interval = 10 * time.Second
	conn.heartbeatConfig.Timeout = 5 * time.Second
	conn.heartbeatConfig.MaxMissed = 2
	conn.heartbeatConfig.PingEndpoint = "/ping"

	// Monitor heartbeat status
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				metrics := conn.GetMetrics()
				lastHeartbeat := metrics.GetLastHeartbeatTime()
				successRate := metrics.GetHeartbeatSuccessRate()

				if !lastHeartbeat.IsZero() {
					log.Printf("Last heartbeat: %s ago, Success rate: %.1f%%",
						time.Since(lastHeartbeat), successRate)
				}
			case <-conn.ctx.Done():
				return
			}
		}
	}()

	// Connect and monitor
	ctx := context.Background()
	if err := conn.Connect(ctx); err != nil {
		log.Printf("Connection failed: %v", err)
	}

	time.Sleep(30 * time.Second)
}

// ExampleAdvancedConfiguration demonstrates advanced configuration options
func ExampleAdvancedConfiguration() {
	// Create a production-ready configuration
	config := &Config{
		BaseURL:        "https://api.example.com",
		Headers:        make(map[string]string),
		BufferSize:     5000,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   30 * time.Second,
		ReconnectDelay: 1 * time.Second,
		MaxReconnects:  10,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Add comprehensive headers
	config.Headers["Authorization"] = "Bearer production-token"
	config.Headers["X-API-Version"] = "v1"
	config.Headers["X-Client-Version"] = "go-sdk-1.0"
	config.Headers["User-Agent"] = "MyApp/1.0 Go-SDK/1.0"

	// Create connection pool with custom configuration
	pool, err := NewConnectionPool(config)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	// Simulate high-load scenario
	ctx := context.Background()
	numWorkers := 5

	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			for j := 0; j < 10; j++ {
				conn, err := pool.AcquireConnection(ctx)
				if err != nil {
					log.Printf("Worker %d: Failed to acquire connection: %v", workerID, err)
					time.Sleep(1 * time.Second)
					continue
				}

				// Simulate work
				time.Sleep(time.Duration(j+1) * time.Second)

				// Handle events
				select {
				case event := <-conn.ReadEvents():
					log.Printf("Worker %d: Received event: %s", workerID, event.Type())
				case err := <-conn.ReadErrors():
					log.Printf("Worker %d: Connection error: %v", workerID, err)
				case <-time.After(5 * time.Second):
					// Timeout, release connection
				}

				pool.ReleaseConnection(conn)
			}
		}(i)
	}

	// Monitor pool health
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats := pool.GetPoolStats()
				log.Printf("Pool stats: Total=%d, Active=%d, Utilization=%.1f%%",
					stats["total_connections"].(int64),
					stats["active_connections"].(int64),
					stats["pool_utilization"].(float64))
			case <-pool.ctx.Done():
				return
			}
		}
	}()

	time.Sleep(60 * time.Second)
	printPoolStats(pool)
}

// Helper functions for event handling

func handleConnectionEvents(conn *Connection) {
	for event := range conn.ReadEvents() {
		log.Printf("Connection %s received event: %s", conn.ID(), event.Type())

		// Update metrics
		conn.metrics.EventsReceived.Inc()

		// Handle specific event types
		switch event.Type() {
		case events.EventTypeTextMessageContent:
			log.Printf("Received text content event")
		case events.EventTypeStateSnapshot:
			log.Printf("Received state snapshot event")
		default:
			log.Printf("Received unknown event type: %s", event.Type())
		}
	}
}

func handleConnectionErrors(conn *Connection) {
	for err := range conn.ReadErrors() {
		log.Printf("Connection %s error: %v", conn.ID(), err)

		// Update metrics
		conn.metrics.NetworkErrors.Inc()

		// Handle specific error types
		if messages.IsConnectionError(err) {
			log.Printf("Network connection error detected")
		} else if messages.IsStreamingError(err) {
			log.Printf("Streaming error detected")
		}
	}
}

func handleStateChanges(conn *Connection) {
	for state := range conn.ReadStateChanges() {
		log.Printf("Connection %s state changed to: %s", conn.ID(), state.String())

		switch state {
		case ConnectionStateConnected:
			log.Printf("Connection %s is now healthy", conn.ID())
		case ConnectionStateError:
			log.Printf("Connection %s encountered an error", conn.ID())
		case ConnectionStateClosed:
			log.Printf("Connection %s has been closed", conn.ID())
			return
		}
	}
}

func printConnectionMetrics(conn *Connection) {
	metrics := conn.GetMetrics()
	info := conn.GetConnectionInfo()

	fmt.Println("\n=== Connection Metrics ===")
	fmt.Printf("Connection ID: %s\n", conn.ID())
	fmt.Printf("Current State: %s\n", conn.State().String())
	fmt.Printf("Uptime: %v\n", conn.GetUptime())
	fmt.Printf("Connect Attempts: %d\n", metrics.ConnectAttempts.Load())
	fmt.Printf("Connect Success Rate: %.1f%%\n", metrics.GetConnectSuccessRate())
	fmt.Printf("Reconnect Attempts: %d\n", metrics.ReconnectAttempts.Load())
	fmt.Printf("Heartbeats Sent: %d\n", metrics.HeartbeatsSent.Load())
	fmt.Printf("Heartbeat Success Rate: %.1f%%\n", metrics.GetHeartbeatSuccessRate())
	fmt.Printf("Events Received: %d\n", metrics.EventsReceived.Load())
	fmt.Printf("Events Sent: %d\n", metrics.EventsSent.Load())
	fmt.Printf("Network Errors: %d\n", metrics.NetworkErrors.Load())
	fmt.Printf("Last Connect Time: %v\n", info["last_connect_time"])
	fmt.Printf("Last Heartbeat Time: %v\n", info["last_heartbeat_time"])
	fmt.Println("========================")
}

func printPoolStats(pool *ConnectionPool) {
	stats := pool.GetPoolStats()

	fmt.Println("\n=== Pool Statistics ===")
	fmt.Printf("Total Connections: %d\n", stats["total_connections"].(int64))
	fmt.Printf("Active Connections: %d\n", stats["active_connections"].(int64))
	fmt.Printf("Idle Connections: %d\n", stats["idle_connections"].(int64))
	fmt.Printf("Failed Connections: %d\n", stats["failed_connections"].(int64))
	fmt.Printf("Pool Utilization: %.1f%%\n", stats["pool_utilization"].(float64))
	fmt.Printf("Acquire Requests: %d\n", stats["acquire_requests"].(int64))
	fmt.Printf("Acquire Successes: %d\n", stats["acquire_successes"].(int64))
	fmt.Printf("Acquire Timeouts: %d\n", stats["acquire_timeouts"].(int64))
	fmt.Printf("Max Connections: %d\n", stats["max_connections"].(int))
	fmt.Printf("Healthy Connections: %d\n", pool.GetHealthyConnectionCount())
	fmt.Println("=====================")
}

func monitorPool(pool *ConnectionPool) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats := pool.GetPoolStats()
			log.Printf("Pool: %d total, %d active, %.1f%% utilization",
				stats["total_connections"].(int64),
				stats["active_connections"].(int64),
				stats["pool_utilization"].(float64))
		case <-pool.ctx.Done():
			return
		}
	}
}