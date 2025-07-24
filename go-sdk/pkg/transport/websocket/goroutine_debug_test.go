package websocket

import (
	"context"
	"runtime"
	"testing"
	"time"
	
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// TestTransportGoroutineLeaks specifically tests for goroutine leaks
func TestTransportGoroutineLeaks(t *testing.T) {
	t.Run("BasicStartStop", func(t *testing.T) {
		VerifyNoLeaks(t, func() {
			server := NewLoadTestServer(t)
			defer server.Close()
			
			config := DefaultTransportConfig()
			config.URLs = []string{server.URL()}
			config.Logger = zaptest.NewLogger(t)
			config.EnableEventValidation = false
			
			transport, err := NewTransport(config)
			require.NoError(t, err)
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			err = transport.Start(ctx)
			require.NoError(t, err)
			
			// Give it time to establish connections
			time.Sleep(100 * time.Millisecond)
			
			// Stop the transport
			err = transport.Stop()
			require.NoError(t, err)
			
			// Give goroutines time to clean up
			time.Sleep(200 * time.Millisecond)
		})
	})
	
	t.Run("ConnectionPoolStartStop", func(t *testing.T) {
		VerifyNoLeaks(t, func() {
			server := NewLoadTestServer(t)
			defer server.Close()
			
			config := &PoolConfig{
				MinConnections:      2,
				MaxConnections:      5,
				ConnectionTimeout:   5 * time.Second,
				HealthCheckInterval: 30 * time.Second,
				URLs:                []string{server.URL()},
				Logger:              zaptest.NewLogger(t),
				ConnectionTemplate: func() *ConnectionConfig {
					config := DefaultConnectionConfig()
					config.Logger = zaptest.NewLogger(t)
					return config
				}(),
			}
			
			pool, err := NewConnectionPool(config)
			require.NoError(t, err)
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			err = pool.Start(ctx)
			require.NoError(t, err)
			
			// Give it time to establish connections
			time.Sleep(100 * time.Millisecond)
			
			// Stop the pool
			err = pool.Stop()
			require.NoError(t, err)
			
			// Give goroutines time to clean up
			time.Sleep(200 * time.Millisecond)
		})
	})
	
	t.Run("SingleConnectionStartStop", func(t *testing.T) {
		VerifyNoLeaks(t, func() {
			server := NewLoadTestServer(t)
			defer server.Close()
			
			config := DefaultConnectionConfig()
			config.URL = server.URL()
			config.Logger = zaptest.NewLogger(t)
			
			conn, err := NewConnection(config)
			require.NoError(t, err)
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			err = conn.Connect(ctx)
			require.NoError(t, err)
			
			// Start auto-reconnect
			conn.StartAutoReconnect(ctx)
			
			// Give it time to stabilize
			time.Sleep(100 * time.Millisecond)
			
			// Close the connection
			err = conn.Close()
			require.NoError(t, err)
			
			// Give goroutines time to clean up
			time.Sleep(200 * time.Millisecond)
		})
	})
}

// TestDebugGoroutineGrowth helps identify where goroutines are growing
func TestDebugGoroutineGrowth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping debug test in short mode")
	}
	
	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutines: %d", initialGoroutines)
	
	server := NewLoadTestServer(t)
	defer server.Close()
	
	serverGoroutines := runtime.NumGoroutine()
	t.Logf("After server creation: %d (diff: %d)", serverGoroutines, serverGoroutines-initialGoroutines)
	
	config := DefaultTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zap.NewNop()
	config.EnableEventValidation = false
	config.PoolConfig.MinConnections = 10
	config.PoolConfig.MaxConnections = 20
	
	transport, err := NewTransport(config)
	require.NoError(t, err)
	
	transportGoroutines := runtime.NumGoroutine()
	t.Logf("After transport creation: %d (diff: %d)", transportGoroutines, transportGoroutines-serverGoroutines)
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	err = transport.Start(ctx)
	require.NoError(t, err)
	
	startGoroutines := runtime.NumGoroutine()
	t.Logf("After transport start: %d (diff: %d)", startGoroutines, startGoroutines-transportGoroutines)
	
	// Wait for connections to establish
	time.Sleep(2 * time.Second)
	
	connectedGoroutines := runtime.NumGoroutine()
	t.Logf("After connections established: %d (diff: %d)", connectedGoroutines, connectedGoroutines-startGoroutines)
	
	// Send a few messages
	for i := 0; i < 10; i++ {
		event := &MockEvent{
			EventType: "test",
			Data:      "test message",
		}
		err := transport.SendEvent(ctx, event)
		require.NoError(t, err)
	}
	
	afterSendGoroutines := runtime.NumGoroutine()
	t.Logf("After sending messages: %d (diff: %d)", afterSendGoroutines, afterSendGoroutines-connectedGoroutines)
	
	// Stop the transport
	err = transport.Stop()
	require.NoError(t, err)
	
	stopGoroutines := runtime.NumGoroutine()
	t.Logf("After transport stop: %d (diff from start: %d)", stopGoroutines, stopGoroutines-startGoroutines)
	
	// Wait for cleanup
	time.Sleep(1 * time.Second)
	
	cleanupGoroutines := runtime.NumGoroutine()
	t.Logf("After cleanup wait: %d (diff from initial: %d)", cleanupGoroutines, cleanupGoroutines-initialGoroutines)
	
	// Print stack trace if goroutines leaked
	if cleanupGoroutines > initialGoroutines+5 {
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		t.Logf("Goroutine stack trace:\n%s", string(buf[:n]))
	}
}