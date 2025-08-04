package httppool

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"time"
)

// ExampleBasicUsage demonstrates basic connection pool usage.
func ExampleBasicUsage() {
	// Create a connection pool with default configuration
	pool, err := NewHTTPConnectionPool(nil)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pool.Shutdown(ctx)
	}()

	// Add backend servers
	if err := pool.AddServer("https://api1.example.com", 1); err != nil {
		log.Fatalf("Failed to add server: %v", err)
	}
	if err := pool.AddServer("https://api2.example.com", 1); err != nil {
		log.Fatalf("Failed to add server: %v", err)
	}

	// Get a connection
	req := &ConnectionRequest{
		Context: context.Background(),
	}

	resp, err := pool.GetConnection(req)
	if err != nil {
		log.Fatalf("Failed to get connection: %v", err)
	}

	fmt.Printf("Connected to: %s\n", resp.Server.URL.String())

	// Use the connection for HTTP requests
	httpReq, _ := http.NewRequest("GET", resp.Server.URL.String()+"/api/data", nil)
	httpResp, err := resp.Connection.client.Do(httpReq)
	if err != nil {
		log.Printf("HTTP request failed: %v", err)
	} else {
		httpResp.Body.Close()
		fmt.Printf("HTTP response status: %s\n", httpResp.Status)
	}

	// Release the connection back to the pool
	pool.ReleaseConnection(resp.Connection)

	// Get and display metrics
	metrics := pool.GetMetrics()
	fmt.Printf("Total requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Pool utilization: %.2f%%\n", metrics.PoolUtilization*100)
}

// ExampleAdvancedConfiguration demonstrates advanced configuration options.
func ExampleAdvancedConfiguration() {
	// Create custom TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
	}

	// Create advanced pool configuration
	config := &HTTPPoolConfig{
		// Connection limits
		MaxConnectionsPerServer: 100,
		MaxTotalConnections:     1000,
		MaxIdleConnections:      50,
		MaxIdleTime:             10 * time.Minute,

		// Timeouts
		ConnectTimeout:    15 * time.Second,
		RequestTimeout:    60 * time.Second,
		KeepAliveTimeout:  30 * time.Second,
		IdleConnTimeout:   90 * time.Second,

		// Health checking
		HealthCheckInterval: 30 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
		HealthCheckPath:     "/health",
		UnhealthyThreshold:  3,
		HealthyThreshold:    2,

		// Load balancing
		LoadBalanceStrategy: LeastConn,

		// Monitoring
		CleanupInterval: 1 * time.Minute,
		MetricsInterval: 10 * time.Second,

		// TLS and transport options
		TLSConfig:             tlsConfig,
		DisableKeepAlives:     false,
		DisableCompression:    false,
		MaxResponseHeaderSize: 1 << 20, // 1MB
		WriteBufferSize:       8192,
		ReadBufferSize:        8192,
	}

	pool, err := NewHTTPConnectionPool(config)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pool.Shutdown(ctx)
	}()

	// Add weighted servers for load balancing
	pool.AddServer("https://primary.example.com", 3)   // Higher weight
	pool.AddServer("https://secondary.example.com", 2) // Medium weight
	pool.AddServer("https://backup.example.com", 1)    // Lower weight

	fmt.Println("Advanced connection pool configured successfully")
}

// ExampleLoadBalancingStrategies demonstrates different load balancing strategies.
func ExampleLoadBalancingStrategies() {
	strategies := []LoadBalanceStrategy{
		RoundRobin,    // Distribute requests evenly across servers
		LeastConn,     // Route to server with fewest active connections
		WeightedRound, // Distribute based on server weights
		Random,        // Random server selection
		IPHash,        // Consistent routing based on client ID
	}

	for _, strategy := range strategies {
		fmt.Printf("\n=== Testing %s strategy ===\n", strategy)

		config := DefaultHTTPPoolConfig()
		config.LoadBalanceStrategy = strategy

		pool, err := NewHTTPConnectionPool(config)
		if err != nil {
			log.Printf("Failed to create pool for %s: %v", strategy, err)
			continue
		}

		// Add test servers
		pool.AddServer("https://server1.example.com", 1)
		pool.AddServer("https://server2.example.com", 2) // Higher weight for weighted strategy
		pool.AddServer("https://server3.example.com", 1)

		// Make test requests
		serverCounts := make(map[string]int)
		for i := 0; i < 10; i++ {
			req := &ConnectionRequest{
				Context:  context.Background(),
				ClientID: fmt.Sprintf("client-%d", i), // For IP hash strategy
			}

			resp, err := pool.GetConnection(req)
			if err != nil {
				log.Printf("Failed to get connection: %v", err)
				continue
			}

			serverURL := resp.Server.URL.String()
			serverCounts[serverURL]++
			
			fmt.Printf("Request %d -> %s\n", i+1, serverURL)
			
			pool.ReleaseConnection(resp.Connection)
		}

		fmt.Printf("Distribution: %+v\n", serverCounts)

		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pool.Shutdown(ctx)
		cancel()
	}
}

// ExampleHealthMonitoring demonstrates health checking and monitoring capabilities.
func ExampleHealthMonitoring() {
	// Configure aggressive health checking for demonstration
	config := &HTTPPoolConfig{
		MaxConnectionsPerServer: 10,
		MaxTotalConnections:     100,
		HealthCheckInterval:     5 * time.Second,  // Check every 5 seconds
		HealthCheckTimeout:      2 * time.Second,  // 2 second timeout
		HealthCheckPath:         "/health",
		UnhealthyThreshold:      2, // Mark unhealthy after 2 failures
		HealthyThreshold:        1, // Mark healthy after 1 success
		LoadBalanceStrategy:     LeastConn,
	}

	pool, err := NewHTTPConnectionPool(config)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pool.Shutdown(ctx)
	}()

	// Add servers (these would be real servers in production)
	pool.AddServer("https://healthy-api.example.com", 1)
	pool.AddServer("https://slow-api.example.com", 1)
	pool.AddServer("https://unreliable-api.example.com", 1)

	// Monitor health status
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Println("\n=== Health Status ===")
				
				// Get server statistics
				stats := pool.GetServerStats()
				for _, stat := range stats {
					status := "HEALTHY"
					if !stat.IsHealthy {
						status = "UNHEALTHY"
					}
					
					fmt.Printf("Server: %s\n", stat.URL.String())
					fmt.Printf("  Status: %s\n", status)
					fmt.Printf("  Failure Count: %d\n", stat.FailureCount)
					fmt.Printf("  Response Time: %v\n", stat.ResponseTime)
					fmt.Printf("  Total Requests: %d\n", stat.TotalRequests)
					fmt.Printf("  Current Connections: %d\n", stat.CurrentConnections)
					fmt.Printf("  Last Health Check: %v\n", stat.LastHealthCheck)
					fmt.Println()
				}

				// Get overall metrics
				metrics := pool.GetMetrics()
				fmt.Printf("Pool Metrics:\n")
				fmt.Printf("  Healthy Servers: %d\n", metrics.HealthyServers)
				fmt.Printf("  Unhealthy Servers: %d\n", metrics.UnhealthyServers)
				fmt.Printf("  Health Checks Success: %d\n", metrics.HealthChecksSuccess)
				fmt.Printf("  Health Checks Failed: %d\n", metrics.HealthChecksFailed)
				fmt.Printf("  Pool Utilization: %.2f%%\n", metrics.PoolUtilization*100)
				fmt.Println()
			}
		}
	}()

	// Simulate application traffic
	fmt.Println("Starting traffic simulation...")
	for i := 0; i < 50; i++ {
		req := &ConnectionRequest{
			Context: context.Background(),
		}

		resp, err := pool.GetConnection(req)
		if err != nil {
			fmt.Printf("Request %d failed: %v\n", i+1, err)
			time.Sleep(1 * time.Second)
			continue
		}

		fmt.Printf("Request %d -> %s (wait: %v)\n", 
			i+1, resp.Server.URL.String(), resp.WaitTime)

		// Simulate work
		time.Sleep(100 * time.Millisecond)
		
		pool.ReleaseConnection(resp.Connection)
		time.Sleep(500 * time.Millisecond)
	}
}

// ExampleMetricsAndMonitoring demonstrates comprehensive metrics collection.
func ExampleMetricsAndMonitoring() {
	pool, err := NewHTTPConnectionPool(nil)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pool.Shutdown(ctx)
	}()

	// Add servers
	pool.AddServer("https://api1.example.com", 1)
	pool.AddServer("https://api2.example.com", 1)

	// Generate some traffic to collect metrics
	for i := 0; i < 20; i++ {
		req := &ConnectionRequest{
			Context: context.Background(),
		}

		resp, err := pool.GetConnection(req)
		if err != nil {
			log.Printf("Request failed: %v", err)
			continue
		}

		// Simulate variable response times
		time.Sleep(time.Duration(i*10) * time.Millisecond)
		
		pool.ReleaseConnection(resp.Connection)
	}

	// Display comprehensive metrics
	metrics := pool.GetMetrics()
	
	fmt.Println("=== Connection Pool Metrics ===")
	fmt.Printf("Total Connections: %d\n", metrics.TotalConnections)
	fmt.Printf("Active Connections: %d\n", metrics.ActiveConnections)
	fmt.Printf("Idle Connections: %d\n", metrics.IdleConnections)
	fmt.Printf("Connections Created: %d\n", metrics.ConnectionsCreated)
	fmt.Printf("Connections Destroyed: %d\n", metrics.ConnectionsDestroyed)
	fmt.Printf("Connections Reused: %d\n", metrics.ConnectionsReused)
	fmt.Println()

	fmt.Println("=== Request Metrics ===")
	fmt.Printf("Total Requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Successful Requests: %d\n", metrics.SuccessfulRequests)
	fmt.Printf("Failed Requests: %d\n", metrics.FailedRequests)
	fmt.Printf("Average Response Time: %v\n", metrics.AverageResponseTime)
	fmt.Printf("Average Wait Time: %v\n", metrics.AverageWaitTime)
	fmt.Printf("Max Wait Time: %v\n", metrics.MaxWaitTime)
	fmt.Println()

	fmt.Println("=== Server Health ===")
	fmt.Printf("Healthy Servers: %d\n", metrics.HealthyServers)
	fmt.Printf("Unhealthy Servers: %d\n", metrics.UnhealthyServers)
	fmt.Printf("Health Checks Success: %d\n", metrics.HealthChecksSuccess)
	fmt.Printf("Health Checks Failed: %d\n", metrics.HealthChecksFailed)
	fmt.Println()

	fmt.Println("=== Performance ===")
	fmt.Printf("Pool Utilization: %.2f%%\n", metrics.PoolUtilization*100)
	fmt.Printf("Memory Usage: %d bytes\n", metrics.MemoryUsage)
	fmt.Printf("Connection Errors: %d\n", metrics.ConnectionErrors)
	fmt.Printf("Timeout Errors: %d\n", metrics.TimeoutErrors)
	fmt.Println()

	fmt.Println("=== Requests Per Server ===")
	for server, count := range metrics.RequestsPerServer {
		fmt.Printf("  %s: %d requests\n", server, count)
	}
	fmt.Println()

	fmt.Printf("Pool started: %v\n", metrics.StartTime)
	fmt.Printf("Last updated: %v\n", metrics.LastUpdated)
	fmt.Printf("Uptime: %v\n", time.Since(metrics.StartTime))
}

// ExampleGracefulShutdown demonstrates proper pool shutdown procedures.
func ExampleGracefulShutdown() {
	pool, err := NewHTTPConnectionPool(nil)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}

	// Add servers
	pool.AddServer("https://api1.example.com", 1)
	pool.AddServer("https://api2.example.com", 1)

	// Simulate some activity
	for i := 0; i < 5; i++ {
		req := &ConnectionRequest{
			Context: context.Background(),
		}

		resp, err := pool.GetConnection(req)
		if err != nil {
			log.Printf("Request failed: %v", err)
			continue
		}
		
		// Don't immediately release to test graceful shutdown
		go func(conn *pooledConnection) {
			time.Sleep(2 * time.Second)
			pool.ReleaseConnection(conn)
		}(resp.Connection)
	}

	fmt.Println("Starting graceful shutdown...")
	
	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Perform graceful shutdown
	start := time.Now()
	err = pool.Shutdown(ctx)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("Shutdown completed with error: %v (took %v)\n", err, duration)
	} else {
		fmt.Printf("Graceful shutdown completed successfully (took %v)\n", duration)
	}

	// Verify that operations fail after shutdown
	req := &ConnectionRequest{
		Context: context.Background(),
	}
	_, err = pool.GetConnection(req)
	if err != nil {
		fmt.Printf("✓ Operations correctly fail after shutdown: %v\n", err)
	}
}

// ExampleCustomTransport demonstrates how to integrate with existing HTTP clients.
func ExampleCustomTransport() {
	pool, err := NewHTTPConnectionPool(nil)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pool.Shutdown(ctx)
	}()

	// Add server
	pool.AddServer("https://api.example.com", 1)

	// Helper function to make HTTP requests using the pool
	makeRequest := func(path string) error {
		req := &ConnectionRequest{
			Context: context.Background(),
		}

		resp, err := pool.GetConnection(req)
		if err != nil {
			return fmt.Errorf("failed to get connection: %w", err)
		}
		defer pool.ReleaseConnection(resp.Connection)

		// Use the pooled connection's HTTP client
		httpReq, err := http.NewRequest("GET", resp.Server.URL.String()+path, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		httpResp, err := resp.Connection.client.Do(httpReq)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		defer httpResp.Body.Close()

		fmt.Printf("GET %s -> %s (server: %s)\n", 
			path, httpResp.Status, resp.Server.URL.String())
		
		return nil
	}

	// Make several requests
	paths := []string{"/users", "/orders", "/products", "/stats"}
	for _, path := range paths {
		if err := makeRequest(path); err != nil {
			log.Printf("Request error: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Display final metrics
	metrics := pool.GetMetrics()
	fmt.Printf("\nFinal pool state:\n")
	fmt.Printf("  Requests: %d\n", metrics.TotalRequests)
	fmt.Printf("  Reused connections: %d\n", metrics.ConnectionsReused)
	fmt.Printf("  Pool utilization: %.2f%%\n", metrics.PoolUtilization*100)
}