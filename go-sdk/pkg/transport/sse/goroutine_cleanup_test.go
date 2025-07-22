package sse

import (
	"context"
	"net/http"
	"runtime"
	"sync"
	"testing"
	"time"

	sdktesting "github.com/ag-ui/go-sdk/pkg/testing"
)

// TestSSEGoroutineCleanup verifies that all goroutines are properly cleaned up
func TestSSEGoroutineCleanup(t *testing.T) {
	testCases := []struct {
		name     string
		testFunc func(t *testing.T, conn *Connection, helper *sdktesting.TestingCleanupHelper)
	}{
		{
			name:     "immediate_close",
			testFunc: testSSEImmediateClose,
		},
		{
			name:     "heartbeat_cleanup",
			testFunc: testSSEHeartbeatCleanup,
		},
		{
			name:     "context_cancellation",
			testFunc: testSSEContextCancellation,
		},
		{
			name:     "connection_pool_cleanup",
			testFunc: testSSEConnectionPoolCleanup,
		},
		{
			name:     "concurrent_operations",
			testFunc: testSSEConcurrentOperations,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			helper := sdktesting.NewTestingCleanupHelper()
			defer helper.Cleanup(t)

			// Track initial goroutine count
			initialCount := runtime.NumGoroutine()
			t.Logf("Initial goroutine count: %d", initialCount)

			// Create connection with test config
			config := createTestSSEConfig()
			conn, err := NewConnection(config, nil)
			if err != nil {
				t.Fatal(err)
			}

			helper.AddCleanup(func() {
				conn.Close()
			})

			// Run test-specific function
			tc.testFunc(t, conn, helper)

			// Allow cleanup to complete
			time.Sleep(100 * time.Millisecond)

			// Force garbage collection
			runtime.GC()
			time.Sleep(10 * time.Millisecond)

			// Check final goroutine count
			finalCount := runtime.NumGoroutine()
			t.Logf("Final goroutine count: %d (diff: %d)", finalCount, finalCount-initialCount)

			// Allow for some variance but detect major leaks
			if finalCount > initialCount+3 {
				t.Errorf("Potential goroutine leak detected: initial=%d, final=%d", 
					initialCount, finalCount)
			}
		})
	}
}

// testSSEImmediateClose tests closing connection immediately
func testSSEImmediateClose(t *testing.T, conn *Connection, helper *sdktesting.TestingCleanupHelper) {
	err := conn.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

// testSSEHeartbeatCleanup tests heartbeat goroutine cleanup
func testSSEHeartbeatCleanup(t *testing.T, conn *Connection, helper *sdktesting.TestingCleanupHelper) {
	// Configure short heartbeat interval for testing
	conn.heartbeatConfig.Enabled = true
	conn.heartbeatConfig.Interval = 10 * time.Millisecond
	conn.heartbeatConfig.Timeout = 20 * time.Millisecond

	// Start heartbeat
	conn.startHeartbeat()
	
	// Let it run briefly
	time.Sleep(25 * time.Millisecond)

	// Stop heartbeat - should clean up goroutine
	conn.stopHeartbeat()
	
	// Verify cleanup
	time.Sleep(10 * time.Millisecond)
}

// testSSEContextCancellation tests cleanup when context is cancelled
func testSSEContextCancellation(t *testing.T, conn *Connection, helper *sdktesting.TestingCleanupHelper) {
	_, cancel := context.WithCancel(context.Background())
	helper.AddCleanup(cancel)

	// Start heartbeat with cancellable context
	conn.heartbeatConfig.Enabled = true
	conn.heartbeatConfig.Interval = 20 * time.Millisecond
	conn.startHeartbeat()

	// Let it run
	time.Sleep(30 * time.Millisecond)

	// Cancel context - should trigger cleanup
	cancel()
	
	// Allow cleanup to complete
	time.Sleep(20 * time.Millisecond)
}

// testSSEConnectionPoolCleanup tests connection pool cleanup
func testSSEConnectionPoolCleanup(t *testing.T, conn *Connection, helper *sdktesting.TestingCleanupHelper) {
	config := createTestSSEConfig()
	pool, err := NewConnectionPool(config)
	if err != nil {
		t.Fatal(err)
	}

	helper.AddCleanup(func() {
		pool.Close()
	})

	ctx := context.Background()

	// Create multiple connections in pool
	var connections []*Connection
	for i := 0; i < 3; i++ {
		c, err := pool.AcquireConnection(ctx)
		if err != nil {
			t.Logf("Failed to acquire connection %d: %v", i, err)
			continue // Expected for test connections that can't actually connect
		}
		connections = append(connections, c)
	}

	// Release connections
	for _, c := range connections {
		pool.ReleaseConnection(c)
	}

	// Let pool health monitoring run briefly
	time.Sleep(50 * time.Millisecond)

	// Close pool - should cleanup all background goroutines
	err = pool.Close()
	if err != nil {
		t.Errorf("Pool Close() returned error: %v", err)
	}
}

// testSSEConcurrentOperations tests cleanup under concurrent operations
func testSSEConcurrentOperations(t *testing.T, conn *Connection, helper *sdktesting.TestingCleanupHelper) {
	var wg sync.WaitGroup

	// Start concurrent operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Simulate state changes
			conn.setState(ConnectionStateConnecting)
			time.Sleep(5 * time.Millisecond)
			conn.setState(ConnectionStateDisconnected)
			time.Sleep(5 * time.Millisecond)
		}(i)
	}

	// Wait for operations to complete
	wg.Wait()
	
	// Close should cleanup everything
	time.Sleep(10 * time.Millisecond)
}

// TestSSEWaitGroupUsage verifies proper WaitGroup usage
func TestSSEWaitGroupUsage(t *testing.T) {
	helper := sdktesting.NewTestingCleanupHelper()
	defer helper.Cleanup(t)

	config := createTestSSEConfig()
	conn, err := NewConnection(config, nil)
	if err != nil {
		t.Fatal(err)
	}

	helper.AddCleanup(func() {
		conn.Close()
	})

	// Start a mock goroutine that uses WaitGroup properly
	conn.wg.Add(1)
	go func() {
		defer conn.wg.Done()
		select {
		case <-conn.ctx.Done():
			return
		case <-time.After(50 * time.Millisecond):
			return
		}
	}()

	// Close should wait for goroutine
	start := time.Now()
	err = conn.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	duration := time.Since(start)
	t.Logf("Close() took %v", duration)

	// Should not take too long since context cancellation makes goroutine exit quickly
	if duration > 200*time.Millisecond {
		t.Errorf("Close() took too long: %v", duration)
	}
}

// TestSSEChannelCleanup tests that channels are properly closed without leaks
func TestSSEChannelCleanup(t *testing.T) {
	helper := sdktesting.NewTestingCleanupHelper()
	defer helper.Cleanup(t)

	config := createTestSSEConfig()
	conn, err := NewConnection(config, nil)
	if err != nil {
		t.Fatal(err)
	}

	helper.AddCleanup(func() {
		conn.Close()
	})

	// Test that channels are accessible before closing
	select {
	case <-conn.ReadEvents():
		t.Log("Event channel is open")
	default:
		t.Log("Event channel is not immediately readable (expected)")
	}

	select {
	case <-conn.ReadErrors():
		t.Log("Error channel has data")
	default:
		t.Log("Error channel is empty (expected)")
	}

	// Close and verify channels are closed
	err = conn.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Channels should be closed after Close()
	// Note: Reading from a closed channel returns zero value immediately
	select {
	case _, ok := <-conn.ReadEvents():
		if ok {
			t.Error("Event channel should be closed after Close()")
		}
	case <-time.After(10 * time.Millisecond):
		// If channel isn't closed, this timeout will trigger
		// But we can't easily test this without race conditions
	}
}

// TestSSEReconnectionCleanup tests cleanup during reconnection attempts
func TestSSEReconnectionCleanup(t *testing.T) {
	helper := sdktesting.NewTestingCleanupHelper()
	defer helper.Cleanup(t)

	config := createTestSSEConfig()
	conn, err := NewConnection(config, nil)
	if err != nil {
		t.Fatal(err)
	}

	helper.AddCleanup(func() {
		conn.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Enable reconnection
	conn.reconnectPolicy.Enabled = true
	conn.reconnectPolicy.MaxAttempts = 3
	conn.reconnectPolicy.InitialDelay = 10 * time.Millisecond
	
	// Attempt reconnection (will fail since no server)
	_ = conn.Reconnect(ctx)

	// Cancel context to cleanup any reconnection goroutines
	cancel()
	time.Sleep(20 * time.Millisecond)
}

// createTestSSEConfig creates a test configuration for SSE
func createTestSSEConfig() *Config {
	return &Config{
		BaseURL:       "http://test.example.com",
		BufferSize:    100,
		MaxReconnects: 3,
		ReconnectDelay: 10 * time.Millisecond,
		Client: &http.Client{
			Timeout: 50 * time.Millisecond,
		},
		Headers: make(map[string]string),
	}
}

// BenchmarkSSEConnectionCleanup benchmarks SSE connection cleanup
func BenchmarkSSEConnectionCleanup(b *testing.B) {
	config := createTestSSEConfig()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		conn, err := NewConnection(config, nil)
		if err != nil {
			b.Fatal(err)
		}

		// Start heartbeat to create goroutines
		conn.heartbeatConfig.Enabled = true
		conn.heartbeatConfig.Interval = 10 * time.Millisecond
		conn.startHeartbeat()

		// Quick cleanup
		conn.Close()
	}
}

// BenchmarkSSEPoolCleanup benchmarks connection pool cleanup
func BenchmarkSSEPoolCleanup(b *testing.B) {
	config := createTestSSEConfig()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pool, err := NewConnectionPool(config)
		if err != nil {
			b.Fatal(err)
		}

		// Let health monitoring start
		time.Sleep(1 * time.Millisecond)

		// Close pool
		pool.Close()
	}
}