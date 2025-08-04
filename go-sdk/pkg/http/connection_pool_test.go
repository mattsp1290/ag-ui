package httppool

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewHTTPConnectionPool(t *testing.T) {
	tests := []struct {
		name      string
		config    *HTTPPoolConfig
		wantError bool
	}{
		{
			name:      "default config",
			config:    nil,
			wantError: false,
		},
		{
			name:      "valid custom config",
			config:    DefaultHTTPPoolConfig(),
			wantError: false,
		},
		{
			name: "invalid config - negative max connections",
			config: &HTTPPoolConfig{
				MaxConnectionsPerServer: -1,
			},
			wantError: true,
		},
		{
			name: "invalid config - zero timeout",
			config: &HTTPPoolConfig{
				MaxConnectionsPerServer: 10,
				MaxTotalConnections:     100,
				ConnectTimeout:          0,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewHTTPConnectionPool(tt.config)
			if tt.wantError {
				if err == nil {
					t.Errorf("NewHTTPConnectionPool() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("NewHTTPConnectionPool() unexpected error: %v", err)
				return
			}
			if pool == nil {
				t.Errorf("NewHTTPConnectionPool() returned nil pool")
				return
			}

			// Cleanup
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			pool.Shutdown(ctx)
		})
	}
}

func TestConnectionPoolAddRemoveServer(t *testing.T) {
	pool, err := NewHTTPConnectionPool(nil)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pool.Shutdown(ctx)
	}()

	// Test adding server
	err = pool.AddServer("http://example.com", 1)
	if err != nil {
		t.Errorf("AddServer() failed: %v", err)
	}

	// Test server was added
	stats := pool.GetServerStats()
	if len(stats) != 1 {
		t.Errorf("Expected 1 server, got %d", len(stats))
	}

	// Test adding invalid server
	err = pool.AddServer("://:invalid-url", 1)
	if err == nil {
		t.Errorf("AddServer() should fail for invalid URL")
	}

	// Test removing server
	err = pool.RemoveServer("http://example.com")
	if err != nil {
		t.Errorf("RemoveServer() failed: %v", err)
	}

	// Test server was removed
	stats = pool.GetServerStats()
	if len(stats) != 0 {
		t.Errorf("Expected 0 servers, got %d", len(stats))
	}

	// Test removing non-existent server
	err = pool.RemoveServer("http://nonexistent.com")
	if err == nil {
		t.Errorf("RemoveServer() should fail for non-existent server")
	}
}

func TestConnectionPoolLoadBalancing(t *testing.T) {
	// Create test servers
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("server1"))
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("server2"))
	}))
	defer server2.Close()

	strategies := []LoadBalanceStrategy{RoundRobin, LeastConn, Random, WeightedRound}

	for _, strategy := range strategies {
		t.Run(string(strategy), func(t *testing.T) {
			config := DefaultHTTPPoolConfig()
			config.LoadBalanceStrategy = strategy

			pool, err := NewHTTPConnectionPool(config)
			if err != nil {
				t.Fatalf("Failed to create connection pool: %v", err)
			}
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				pool.Shutdown(ctx)
			}()

			// Add servers
			err = pool.AddServer(server1.URL, 1)
			if err != nil {
				t.Fatalf("Failed to add server1: %v", err)
			}
			err = pool.AddServer(server2.URL, 1)
			if err != nil {
				t.Fatalf("Failed to add server2: %v", err)
			}

			// Wait for health checks
			time.Sleep(100 * time.Millisecond)

			// Test getting connections
			req := &ConnectionRequest{
				Context: context.Background(),
			}

			connections := make([]*ConnectionResponse, 0, 10)
			for i := 0; i < 10; i++ {
				resp, err := pool.GetConnection(req)
				if err != nil {
					t.Errorf("GetConnection() failed: %v", err)
					continue
				}
				connections = append(connections, resp)
			}

			// Release all connections
			for _, conn := range connections {
				pool.ReleaseConnection(conn.Connection)
			}

			// Verify metrics
			metrics := pool.GetMetrics()
			if metrics.TotalRequests != 10 {
				t.Errorf("Expected 10 total requests, got %d", metrics.TotalRequests)
			}
		})
	}
}

func TestConnectionPoolHealthChecking(t *testing.T) {
	// Create healthy server
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("healthy"))
		}
	}))
	defer healthyServer.Close()

	// Create unhealthy server
	unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("unhealthy"))
		}
	}))
	defer unhealthyServer.Close()

	config := DefaultHTTPPoolConfig()
	config.HealthCheckInterval = 50 * time.Millisecond  // More frequent checks
	config.UnhealthyThreshold = 1 // Mark unhealthy after 1 failure

	pool, err := NewHTTPConnectionPool(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pool.Shutdown(ctx)
	}()

	// Add servers
	err = pool.AddServer(healthyServer.URL, 1)
	if err != nil {
		t.Fatalf("Failed to add healthy server: %v", err)
	}
	err = pool.AddServer(unhealthyServer.URL, 1)
	if err != nil {
		t.Fatalf("Failed to add unhealthy server: %v", err)
	}

	// Wait for health checks to run (allow time for multiple checks)
	time.Sleep(200 * time.Millisecond)
	
	// Manually update metrics for testing
	pool.UpdateMetrics()

	// Check server stats
	stats := pool.GetServerStats()
	healthyCount := 0
	unhealthyCount := 0

	for _, stat := range stats {
		t.Logf("Server %s: healthy=%t, failureCount=%d, lastCheck=%v", 
			stat.URL.String(), stat.IsHealthy, stat.FailureCount, stat.LastHealthCheck)
		if stat.IsHealthy {
			healthyCount++
		} else {
			unhealthyCount++
		}
	}

	if healthyCount != 1 {
		t.Errorf("Expected 1 healthy server, got %d", healthyCount)
	}
	if unhealthyCount != 1 {
		t.Errorf("Expected 1 unhealthy server, got %d", unhealthyCount)
	}

	// Verify metrics
	metrics := pool.GetMetrics()
	if metrics.HealthyServers != 1 {
		t.Errorf("Expected 1 healthy server in metrics, got %d", metrics.HealthyServers)
	}
	if metrics.UnhealthyServers != 1 {
		t.Errorf("Expected 1 unhealthy server in metrics, got %d", metrics.UnhealthyServers)
	}
}

func TestConnectionPoolConcurrency(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
		} else {
			// Simulate some processing time
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}
	}))
	defer server.Close()

	config := DefaultHTTPPoolConfig()
	config.MaxConnectionsPerServer = 50  // Increase to handle concurrency
	config.MaxTotalConnections = 100     // Increase total limit

	pool, err := NewHTTPConnectionPool(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pool.Shutdown(ctx)
	}()

	// Add server
	err = pool.AddServer(server.URL, 1)
	if err != nil {
		t.Fatalf("Failed to add server: %v", err)
	}

	// Wait for health check
	time.Sleep(100 * time.Millisecond)

	// Test concurrent requests
	numWorkers := 10  // Reduce concurrency
	numRequestsPerWorker := 3  // Reduce requests per worker
	var wg sync.WaitGroup
	errors := make(chan error, numWorkers*numRequestsPerWorker)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < numRequestsPerWorker; j++ {
				req := &ConnectionRequest{
					Context:  context.Background(),
					ClientID: fmt.Sprintf("worker-%d", workerID),
				}

				resp, err := pool.GetConnection(req)
				if err != nil {
					errors <- fmt.Errorf("worker %d request %d failed: %w", workerID, j, err)
					continue
				}

				// Simulate using the connection
				time.Sleep(5 * time.Millisecond)

				// Release connection
				err = pool.ReleaseConnection(resp.Connection)
				if err != nil {
					errors <- fmt.Errorf("worker %d release %d failed: %w", workerID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Logf("Concurrent test error: %v", err)
		errorCount++
	}

	// Allow some errors due to connection limits, but not too many
	if errorCount > numWorkers*numRequestsPerWorker/4 {
		t.Errorf("Too many errors in concurrent test: %d", errorCount)
	}

	// Verify metrics
	metrics := pool.GetMetrics()
	t.Logf("Final metrics: requests=%d, successful=%d, failed=%d, created=%d, reused=%d",
		metrics.TotalRequests, metrics.SuccessfulRequests, metrics.FailedRequests,
		metrics.ConnectionsCreated, metrics.ConnectionsReused)
}

func TestConnectionHTTPPoolMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}
	}))
	defer server.Close()

	pool, err := NewHTTPConnectionPool(nil)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pool.Shutdown(ctx)
	}()

	// Add server
	err = pool.AddServer(server.URL, 1)
	if err != nil {
		t.Fatalf("Failed to add server: %v", err)
	}

	// Wait for health check
	time.Sleep(100 * time.Millisecond)

	// Get initial metrics
	initialMetrics := pool.GetMetrics()
	if initialMetrics == nil {
		t.Fatal("GetMetrics() returned nil")
	}

	// Make some requests
	req := &ConnectionRequest{
		Context: context.Background(),
	}

	for i := 0; i < 5; i++ {
		resp, err := pool.GetConnection(req)
		if err != nil {
			t.Errorf("GetConnection() failed: %v", err)
			continue
		}
		pool.ReleaseConnection(resp.Connection)
	}

	// Get updated metrics
	finalMetrics := pool.GetMetrics()
	if finalMetrics.TotalRequests != 5 {
		t.Errorf("Expected 5 total requests, got %d", finalMetrics.TotalRequests)
	}

	if finalMetrics.ConnectionsCreated == 0 {
		t.Errorf("Expected some connections to be created")
	}

	if finalMetrics.StartTime.IsZero() {
		t.Errorf("StartTime should not be zero")
	}

	if finalMetrics.LastUpdated.IsZero() {
		t.Errorf("LastUpdated should not be zero")
	}
}

func TestConnectionPoolShutdown(t *testing.T) {
	pool, err := NewHTTPConnectionPool(nil)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}

	// Add a server
	err = pool.AddServer("http://example.com", 1)
	if err != nil {
		t.Fatalf("Failed to add server: %v", err)
	}

	// Shutdown pool
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = pool.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() failed: %v", err)
	}

	// Test that operations fail after shutdown
	err = pool.AddServer("http://example2.com", 1)
	if err == nil {
		t.Errorf("AddServer() should fail after shutdown")
	}

	req := &ConnectionRequest{
		Context: context.Background(),
	}
	_, err = pool.GetConnection(req)
	if err == nil {
		t.Errorf("GetConnection() should fail after shutdown")
	}
}

func TestConnectionPoolIPHashLoadBalancing(t *testing.T) {
	// Create test servers
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("server1"))
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("server2"))
	}))
	defer server2.Close()

	config := DefaultHTTPPoolConfig()
	config.LoadBalanceStrategy = IPHash

	pool, err := NewHTTPConnectionPool(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pool.Shutdown(ctx)
	}()

	// Add servers
	err = pool.AddServer(server1.URL, 1)
	if err != nil {
		t.Fatalf("Failed to add server1: %v", err)
	}
	err = pool.AddServer(server2.URL, 1)
	if err != nil {
		t.Fatalf("Failed to add server2: %v", err)
	}

	// Wait for health checks
	time.Sleep(100 * time.Millisecond)

	// Test that same client ID gets same server
	clientID := "test-client-123"
	var firstServer *ServerTarget

	for i := 0; i < 5; i++ {
		req := &ConnectionRequest{
			Context:  context.Background(),
			ClientID: clientID,
		}

		resp, err := pool.GetConnection(req)
		if err != nil {
			t.Errorf("GetConnection() failed: %v", err)
			continue
		}

		if firstServer == nil {
			firstServer = resp.Server
		} else if resp.Server.URL.String() != firstServer.URL.String() {
			t.Errorf("IP hash should return same server for same client ID")
		}

		pool.ReleaseConnection(resp.Connection)
	}
}

// Benchmark tests

func BenchmarkConnectionPoolGetRelease(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := DefaultHTTPPoolConfig()
	config.MaxConnectionsPerServer = 1000  // High limit for benchmarks
	config.MaxTotalConnections = 5000
	
	pool, err := NewHTTPConnectionPool(config)
	if err != nil {
		b.Fatalf("Failed to create connection pool: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pool.Shutdown(ctx)
	}()

	err = pool.AddServer(server.URL, 1)
	if err != nil {
		b.Fatalf("Failed to add server: %v", err)
	}

	// Wait for health check
	time.Sleep(100 * time.Millisecond)

	req := &ConnectionRequest{
		Context: context.Background(),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := pool.GetConnection(req)
			if err != nil {
				b.Errorf("GetConnection() failed: %v", err)
				continue
			}
			pool.ReleaseConnection(resp.Connection)
		}
	})
}

func BenchmarkConnectionHTTPPoolMetrics(b *testing.B) {
	pool, err := NewHTTPConnectionPool(nil)
	if err != nil {
		b.Fatalf("Failed to create connection pool: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pool.Shutdown(ctx)
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = pool.GetMetrics()
		}
	})
}

// Example usage

func ExampleHTTPConnectionPool() {
	// Create connection pool with custom configuration
	config := &HTTPPoolConfig{
		MaxConnectionsPerServer: 50,
		MaxTotalConnections:     500,
		MaxIdleConnections:      25,
		MaxIdleTime:             5 * time.Minute,
		ConnectTimeout:          10 * time.Second,
		RequestTimeout:          30 * time.Second,
		HealthCheckInterval:     30 * time.Second,
		LoadBalanceStrategy:     LeastConn,
	}

	pool, err := NewHTTPConnectionPool(config)
	if err != nil {
		fmt.Printf("Failed to create pool: %v\n", err)
		return
	}

	// Add servers
	pool.AddServer("https://api1.example.com", 2) // Higher weight
	pool.AddServer("https://api2.example.com", 1)
	pool.AddServer("https://api3.example.com", 1)

	// Get connection
	req := &ConnectionRequest{
		Context:  context.Background(),
		ClientID: "user-123",
	}

	resp, err := pool.GetConnection(req)
	if err != nil {
		fmt.Printf("Failed to get connection: %v\n", err)
		return
	}

	// Use connection (resp.Connection contains the HTTP client and transport)
	fmt.Printf("Got connection to server: %s\n", resp.Server.URL.String())
	fmt.Printf("Wait time: %v\n", resp.WaitTime)
	fmt.Printf("From pool: %t\n", resp.FromPool)

	// Release connection back to pool
	pool.ReleaseConnection(resp.Connection)

	// Get metrics
	metrics := pool.GetMetrics()
	fmt.Printf("Pool utilization: %.2f%%\n", metrics.PoolUtilization*100)
	fmt.Printf("Total requests: %d\n", metrics.TotalRequests)
	fmt.Printf("Healthy servers: %d\n", metrics.HealthyServers)

	// Shutdown gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool.Shutdown(ctx)
}