package websocket

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestGoroutineLeakFix verifies that connections clean up their goroutines properly
func TestGoroutineLeakFix(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping goroutine leak test in short mode")
	}
	
	// Get initial goroutine count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	server := NewTestWebSocketServer(t)
	defer server.Close()

	// Create and use multiple connections
	for i := 0; i < 5; i++ {
		config := DefaultConnectionConfig()
		config.URL = server.URL()
		config.Logger = zaptest.NewLogger(t)
		config.ReadTimeout = 1 * time.Second
		config.WriteTimeout = 1 * time.Second

		conn, err := NewConnection(config)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		// Connect
		err = conn.Connect(ctx)
		require.NoError(t, err)

		// Send some messages
		for j := 0; j < 3; j++ {
			err = conn.SendMessage(ctx, []byte("test message"))
			require.NoError(t, err)
		}

		// Close the connection
		err = conn.Close()
		require.NoError(t, err)

		cancel()
	}

	// Give some time for cleanup
	time.Sleep(2 * time.Second)
	runtime.GC()
	time.Sleep(500 * time.Millisecond)

	// Check final goroutine count
	finalGoroutines := runtime.NumGoroutine()
	
	t.Logf("Initial goroutines: %d, Final goroutines: %d", initialGoroutines, finalGoroutines)
	
	// Allow for some tolerance (test framework, logger, etc. might create some goroutines)
	// The key is that we shouldn't have a massive increase indicating leaks
	assert.InDelta(t, initialGoroutines, finalGoroutines, 10, 
		"Goroutine count increased by more than expected, possible leak detected")
}

// TestConnectionCloseTimeout verifies that connections close within reasonable time
func TestConnectionCloseTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping connection close timeout test in short mode")
	}
	
	server := NewTestWebSocketServer(t)
	defer server.Close()

	config := DefaultConnectionConfig()
	config.URL = server.URL()
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect
	err = conn.Connect(ctx)
	require.NoError(t, err)

	// Send a message to ensure goroutines are active
	err = conn.SendMessage(ctx, []byte("test message"))
	require.NoError(t, err)

	// Close with timeout measurement
	start := time.Now()
	err = conn.Close()
	closeTime := time.Since(start)

	require.NoError(t, err)
	
	t.Logf("Connection close took: %v", closeTime)
	
	// Connection should close within reasonable time (allowing for our 10s timeout + some buffer)
	assert.Less(t, closeTime, 15*time.Second, "Connection took too long to close")
}

// TestStopConnectionGoroutinesTimeout verifies stopConnectionGoroutines doesn't hang
func TestStopConnectionGoroutinesTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stopConnectionGoroutines timeout test in short mode")
	}
	
	server := NewTestWebSocketServer(t)
	defer server.Close()

	config := DefaultConnectionConfig()
	config.URL = server.URL()
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to start goroutines
	err = conn.Connect(ctx)
	require.NoError(t, err)

	// Send a message to ensure goroutines are active
	err = conn.SendMessage(ctx, []byte("test message"))
	require.NoError(t, err)

	// Test stopConnectionGoroutines with timeout
	start := time.Now()
	conn.stopConnectionGoroutines()
	stopTime := time.Since(start)

	t.Logf("stopConnectionGoroutines took: %v", stopTime)
	
	// Should stop within reasonable time (our implementation has improved 2.5s timeout total)
	assert.Less(t, stopTime, 5*time.Second, "stopConnectionGoroutines took too long")

	// Clean up
	err = conn.Close()
	require.NoError(t, err)
}

// TestRapidConnectDisconnectCycles verifies connections handle rapid cycles properly
func TestRapidConnectDisconnectCycles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rapid connect/disconnect cycles test in short mode")
	}
	
	server := NewTestWebSocketServer(t)
	defer server.Close()

	config := DefaultConnectionConfig()
	config.URL = server.URL()
	config.Logger = zaptest.NewLogger(t)
	
	// Use shorter timeouts for faster cycling
	config.ReadTimeout = 500 * time.Millisecond
	config.WriteTimeout = 500 * time.Millisecond

	conn, err := NewConnection(config)
	require.NoError(t, err)
	defer conn.Close()

	// Perform rapid connect/disconnect cycles
	cycles := 10
	for i := 0; i < cycles; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		
		// Connect
		start := time.Now()
		err = conn.Connect(ctx)
		connectTime := time.Since(start)
		require.NoError(t, err, "Cycle %d: Failed to connect", i+1)
		assert.Equal(t, StateConnected, conn.State(), "Cycle %d: Not in connected state", i+1)
		
		// Send a quick message to ensure the connection is working
		err = conn.SendMessage(ctx, []byte(fmt.Sprintf("test message %d", i+1)))
		require.NoError(t, err, "Cycle %d: Failed to send message", i+1)
		
		// Disconnect
		start = time.Now()
		err = conn.Disconnect()
		disconnectTime := time.Since(start)
		require.NoError(t, err, "Cycle %d: Failed to disconnect", i+1)
		assert.Equal(t, StateDisconnected, conn.State(), "Cycle %d: Not in disconnected state", i+1)
		
		t.Logf("Cycle %d: Connect=%v, Disconnect=%v", i+1, connectTime, disconnectTime)
		
		// Ensure operations complete quickly
		assert.Less(t, connectTime, 2*time.Second, "Cycle %d: Connect took too long", i+1)
		assert.Less(t, disconnectTime, 3*time.Second, "Cycle %d: Disconnect took too long", i+1)
		
		cancel()
		
		// Small delay between cycles
		time.Sleep(50 * time.Millisecond)
	}
}

// TestImprovedConnectionCleanup verifies that the improved cleanup works properly
func TestImprovedConnectionCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping improved connection cleanup test in short mode")
	}
	
	// Get initial goroutine count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	server := NewTestWebSocketServer(t)
	defer server.Close()

	// Test with a single connection that sends multiple messages
	config := DefaultConnectionConfig()
	config.URL = server.URL()
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect
	err = conn.Connect(ctx)
	require.NoError(t, err)

	// Send several messages to ensure goroutines are actively working
	for i := 0; i < 20; i++ {
		err = conn.SendMessage(ctx, []byte(fmt.Sprintf("test message %d", i)))
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
	}

	// Close the connection and measure cleanup time
	start := time.Now()
	err = conn.Close()
	closeTime := time.Since(start)
	
	require.NoError(t, err)
	t.Logf("Improved connection close took: %v", closeTime)
	
	// Should close much faster now with immediate WebSocket close
	assert.Less(t, closeTime, 5*time.Second, "Improved connection close took too long")

	// Give some time for cleanup
	time.Sleep(1 * time.Second)
	runtime.GC()
	time.Sleep(500 * time.Millisecond)

	// Check final goroutine count
	finalGoroutines := runtime.NumGoroutine()
	
	t.Logf("Improved cleanup: Initial goroutines: %d, Final goroutines: %d", 
		initialGoroutines, finalGoroutines)
	
	// Should have much better cleanup now
	assert.InDelta(t, initialGoroutines, finalGoroutines, 5, 
		"Improved cleanup should result in better goroutine management")
}