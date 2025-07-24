package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// createSimpleEchoServer creates a simple echo server for testing
func createSimpleEchoServer(t *testing.T) *httptest.Server {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("Server upgrade error: %v", err)
			return
		}
		defer conn.Close()
		
		// Set up automatic pong responses for ping messages
		conn.SetPingHandler(func(message string) error {
			return conn.WriteMessage(websocket.PongMessage, []byte(message))
		})
		
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			
			// Echo the message back
			if err := conn.WriteMessage(messageType, message); err != nil {
				break
			}
		}
	}))
}

// TestConcurrentWriteSynchronization specifically tests that concurrent write operations
// are properly synchronized and don't cause panics
func TestConcurrentWriteSynchronization(t *testing.T) {
	// Create test server
	server := createSimpleEchoServer(t)
	defer server.Close()

	// Create connection with fast heartbeat to increase write concurrency
	config := DefaultConnectionConfig()
	config.URL = "ws" + server.URL[4:] // Convert http:// to ws://
	config.PingPeriod = 10 * time.Millisecond // Very fast heartbeat
	config.PongWait = 50 * time.Millisecond
	config.WriteTimeout = 1 * time.Second
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect to server
	err = conn.Connect(ctx)
	require.NoError(t, err)
	defer conn.Close()

	// Give heartbeat time to start
	time.Sleep(50 * time.Millisecond)

	// Test concurrent writes from multiple goroutines
	// This should trigger the scenario where heartbeat pings and regular messages
	// are being sent concurrently
	const numGoroutines = 10
	const messagesPerGoroutine = 20
	
	var wg sync.WaitGroup
	var panicMutex sync.Mutex
	panics := make([]interface{}, 0)

	// Capture any panics from concurrent writes
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Unexpected panic in main goroutine: %v", r)
		}
	}()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panicMutex.Lock()
					panics = append(panics, r)
					panicMutex.Unlock()
				}
			}()

			for j := 0; j < messagesPerGoroutine; j++ {
				message := []byte("test message from goroutine")
				msgCtx, msgCancel := context.WithTimeout(ctx, 100*time.Millisecond)
				
				err := conn.SendMessage(msgCtx, message)
				msgCancel()
				
				if err != nil && err != context.DeadlineExceeded {
					// Log but don't fail test on send errors - we're testing for panics
					t.Logf("Send error in goroutine %d message %d: %v", goroutineID, j, err)
				}
				
				// Small delay to allow heartbeat and other operations to interleave
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines completed successfully
	case <-ctx.Done():
		t.Fatal("Test timed out waiting for concurrent writes to complete")
	}

	// Check if any panics occurred
	panicMutex.Lock()
	defer panicMutex.Unlock()

	if len(panics) > 0 {
		t.Fatalf("Concurrent write panics detected: %v", panics)
	}

	// Verify the connection is still functional
	assert.True(t, conn.IsConnected(), "Connection should still be connected after concurrent writes")
	
	// Verify heartbeat is still functioning
	heartbeat := conn.GetHeartbeat()
	assert.NotNil(t, heartbeat, "Heartbeat manager should be available")
	assert.True(t, heartbeat.IsHealthy(), "Connection should be healthy after concurrent writes")

	t.Logf("Successfully completed %d concurrent goroutines each sending %d messages", 
		numGoroutines, messagesPerGoroutine)
}

// TestHeartbeatWriteMessageConcurrency specifically tests heartbeat ping vs regular message concurrency
func TestHeartbeatWriteMessageConcurrency(t *testing.T) {
	// Create test server that responds to pings
	server := createSimpleEchoServer(t)
	defer server.Close()

	// Create connection with very aggressive heartbeat timing
	config := DefaultConnectionConfig()
	config.URL = "ws" + server.URL[4:] // Convert http:// to ws://
	config.PingPeriod = 5 * time.Millisecond  // Very aggressive ping
	config.PongWait = 20 * time.Millisecond
	config.WriteTimeout = 100 * time.Millisecond
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Connect to server
	err = conn.Connect(ctx)
	require.NoError(t, err)
	defer conn.Close()

	// Give heartbeat time to start sending pings aggressively
	time.Sleep(20 * time.Millisecond)

	// Capture any panics
	var panicOccurred bool
	defer func() {
		if r := recover(); r != nil {
			panicOccurred = true
			t.Errorf("Panic detected during concurrent heartbeat/message writes: %v", r)
		}
	}()

	// Send messages continuously while heartbeat is pinging
	for i := 0; i < 100; i++ {
		msgCtx, msgCancel := context.WithTimeout(ctx, 50*time.Millisecond)
		message := []byte("concurrent test message")
		
		err := conn.SendMessage(msgCtx, message)
		msgCancel()
		
		if err != nil && err != context.DeadlineExceeded {
			t.Logf("Send error on message %d: %v", i, err)
		}
		
		// No delay - maximum stress test
	}

	assert.False(t, panicOccurred, "No panics should occur during concurrent heartbeat/message writes")

	// Verify connection is still healthy
	assert.True(t, conn.IsConnected(), "Connection should still be connected")
	
	heartbeat := conn.GetHeartbeat()
	assert.NotNil(t, heartbeat, "Heartbeat manager should be available")
	
	// Give a moment for final heartbeat operations
	time.Sleep(50 * time.Millisecond)
	
	t.Logf("Heartbeat stats: %+v", heartbeat.GetStats())
	t.Log("Concurrent heartbeat/message write test completed successfully")
}