package websocket

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	sdktesting "github.com/ag-ui/go-sdk/pkg/testing"
)

// TestWebSocketGoroutineCleanup verifies that all goroutines are properly cleaned up
func TestWebSocketGoroutineCleanup(t *testing.T) {
	testCases := []struct {
		name        string
		connectTime time.Duration
		testFunc    func(t *testing.T, conn *Connection, helper *sdktesting.TestingCleanupHelper)
	}{
		{
			name:        "immediate_close",
			connectTime: 0,
			testFunc:    testImmediateClose,
		},
		{
			name:        "delayed_close",
			connectTime: 100 * time.Millisecond,
			testFunc:    testDelayedClose,
		},
		{
			name:        "context_cancellation",
			connectTime: 50 * time.Millisecond,
			testFunc:    testContextCancellation,
		},
		{
			name:        "multiple_connections",
			connectTime: 50 * time.Millisecond,
			testFunc:    testMultipleConnections,
		},
		{
			name:        "concurrent_operations",
			connectTime: 100 * time.Millisecond,
			testFunc:    testConcurrentOperations,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			helper := sdktesting.NewTestingCleanupHelper()
			defer helper.Cleanup(t)

			// Track initial goroutine count
			initialCount := runtime.NumGoroutine()
			t.Logf("Initial goroutine count: %d", initialCount)

			// Create connection with test-friendly config
			config := createTestConfig()
			conn, err := NewConnection(config)
			if err != nil {
				t.Fatal(err)
			}

			helper.AddCleanup(func() {
				conn.Close()
			})

			// Run test-specific function
			tc.testFunc(t, conn, helper)

			// Allow some time for cleanup
			time.Sleep(tc.connectTime + 50*time.Millisecond)

			// Force garbage collection to ensure cleanup
			runtime.GC()
			time.Sleep(10 * time.Millisecond)

			// Check final goroutine count
			finalCount := runtime.NumGoroutine()
			t.Logf("Final goroutine count: %d (diff: %d)", finalCount, finalCount-initialCount)

			// Allow for some variance but no major leaks
			if finalCount > initialCount+3 {
				t.Errorf("Potential goroutine leak detected: initial=%d, final=%d", 
					initialCount, finalCount)
			}
		})
	}
}

// testImmediateClose tests closing connection immediately after creation
func testImmediateClose(t *testing.T, conn *Connection, helper *sdktesting.TestingCleanupHelper) {
	// Close immediately without connecting
	err := conn.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

// testDelayedClose tests closing connection after a delay
func testDelayedClose(t *testing.T, conn *Connection, helper *sdktesting.TestingCleanupHelper) {
	time.Sleep(50 * time.Millisecond)
	
	err := conn.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

// testContextCancellation tests cleanup when context is cancelled
func testContextCancellation(t *testing.T, conn *Connection, helper *sdktesting.TestingCleanupHelper) {
	ctx, cancel := context.WithCancel(context.Background())
	helper.AddCleanup(cancel)

	// Start auto-reconnect which creates goroutines
	conn.StartAutoReconnect(ctx)

	// Cancel context to trigger cleanup
	time.Sleep(25 * time.Millisecond)
	cancel()
	
	// Allow cleanup to complete
	time.Sleep(25 * time.Millisecond)
}

// testMultipleConnections tests cleanup of multiple connections
func testMultipleConnections(t *testing.T, conn *Connection, helper *sdktesting.TestingCleanupHelper) {
	const numConnections = 5
	var connections []*Connection
	
	// Create multiple connections
	for i := 0; i < numConnections; i++ {
		config := createTestConfig()
		c, err := NewConnection(config)
		if err != nil {
			t.Fatal(err)
		}
		connections = append(connections, c)
		
		helper.AddCleanup(func() {
			c.Close()
		})
	}

	// Let them exist for a short time
	time.Sleep(25 * time.Millisecond)

	// Close all connections concurrently
	var wg sync.WaitGroup
	for _, c := range connections {
		wg.Add(1)
		go func(conn *Connection) {
			defer wg.Done()
			conn.Close()
		}(c)
	}
	
	wg.Wait()
}

// testConcurrentOperations tests cleanup under concurrent operations
func testConcurrentOperations(t *testing.T, conn *Connection, helper *sdktesting.TestingCleanupHelper) {
	ctx, cancel := context.WithCancel(context.Background())
	helper.AddCleanup(cancel)

	// Start auto-reconnect
	conn.StartAutoReconnect(ctx)

	var wg sync.WaitGroup
	
	// Simulate concurrent operations
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			// Try various operations that might create goroutines
			conn.triggerReconnect()
			time.Sleep(10 * time.Millisecond)
			
			conn.ForceConnectionCheck()
			time.Sleep(10 * time.Millisecond)
		}()
	}

	// Let operations run briefly
	time.Sleep(50 * time.Millisecond)

	// Cancel context and wait for operations to complete
	cancel()
	wg.Wait()
}

// TestHeartbeatGoroutineCleanup specifically tests heartbeat manager cleanup
func TestHeartbeatGoroutineCleanup(t *testing.T) {
	helper := sdktesting.NewTestingCleanupHelper()
	defer helper.Cleanup(t)

	config := createTestConfig()
	config.PingPeriod = 10 * time.Millisecond
	config.PongWait = 20 * time.Millisecond
	
	conn, err := NewConnection(config)
	if err != nil {
		t.Fatal(err)
	}

	helper.AddCleanup(func() {
		conn.Close()
	})

	// Start and stop heartbeat multiple times
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		
		conn.heartbeat.Start(ctx)
		time.Sleep(15 * time.Millisecond)
		
		conn.heartbeat.Stop()
		cancel()
		time.Sleep(5 * time.Millisecond)
	}
}

// TestConnectionPoolGoroutineCleanup tests cleanup of connection state changes
func TestConnectionStateGoroutineCleanup(t *testing.T) {
	helper := sdktesting.NewTestingCleanupHelper()
	defer helper.Cleanup(t)

	config := createTestConfig()
	conn, err := NewConnection(config)
	if err != nil {
		t.Fatal(err)
	}

	helper.AddCleanup(func() {
		conn.Close()
	})

	// Rapidly change states to test state management goroutines
	states := []ConnectionState{
		StateConnecting,
		StateDisconnected,
		StateReconnecting,
		StateDisconnected,
		StateClosing,
	}

	for _, state := range states {
		conn.setState(state)
		time.Sleep(5 * time.Millisecond)
	}
}

// TestWaitGroupCleanup verifies WaitGroup usage prevents goroutine leaks
func TestWaitGroupCleanup(t *testing.T) {
	helper := sdktesting.NewTestingCleanupHelper()
	defer helper.Cleanup(t)

	config := createTestConfig()
	conn, err := NewConnection(config)
	if err != nil {
		t.Fatal(err)
	}

	helper.AddCleanup(func() {
		conn.Close()
	})

	// Simulate what happens during Connect() - add goroutines to WaitGroup
	conn.wg.Add(2)
	
	// Start mock goroutines that use the WaitGroup properly
	go func() {
		defer conn.wg.Done()
		select {
		case <-conn.ctx.Done():
			return
		case <-time.After(50 * time.Millisecond):
			return
		}
	}()
	
	go func() {
		defer conn.wg.Done()
		select {
		case <-conn.ctx.Done():
			return
		case <-time.After(75 * time.Millisecond):
			return
		}
	}()

	// Close should wait for these goroutines
	time.Sleep(25 * time.Millisecond)
	start := time.Now()
	
	err = conn.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
	
	duration := time.Since(start)
	t.Logf("Close() took %v", duration)
	
	// Should not take too long (goroutines should exit quickly due to context)
	if duration > 200*time.Millisecond {
		t.Errorf("Close() took too long: %v", duration)
	}
}

// createTestConfig creates a configuration suitable for testing
func createTestConfig() *ConnectionConfig {
	config := DefaultConnectionConfig()
	config.URL = "ws://test.example.com/ws"
	config.Logger = zap.NewNop() // No logging during tests
	config.PingPeriod = 50 * time.Millisecond
	config.PongWait = 100 * time.Millisecond
	config.ReadTimeout = 100 * time.Millisecond
	config.WriteTimeout = 100 * time.Millisecond
	config.DialTimeout = 100 * time.Millisecond
	config.HandshakeTimeout = 100 * time.Millisecond
	return config
}

// BenchmarkConnectionCleanup benchmarks the cleanup performance
func BenchmarkConnectionCleanup(b *testing.B) {
	config := createTestConfig()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		conn, err := NewConnection(config)
		if err != nil {
			b.Fatal(err)
		}
		
		// Simulate some activity
		ctx, cancel := context.WithCancel(context.Background())
		conn.StartAutoReconnect(ctx)
		
		// Cleanup
		cancel()
		conn.Close()
	}
}