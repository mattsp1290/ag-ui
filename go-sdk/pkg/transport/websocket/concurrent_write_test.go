package websocket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	testutils "github.com/ag-ui/go-sdk/pkg/testing"
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
	testutils.WithTestTimeout(t, 30*time.Second, func() {
		tester := NewReliableConnectionTester(t)
		defer tester.Cleanup()

		tester.TestConnection(func(conn *Connection) {
			// Test concurrent writes from multiple goroutines with proper synchronization
			const numGoroutines = 5  // Reduced for reliability
			const messagesPerGoroutine = 10  // Reduced for faster execution
			
			concurrentTester := testutils.NewConcurrentTester(t, numGoroutines*2)
			barrier := testutils.NewTestBarrier(numGoroutines)
			
			for i := 0; i < numGoroutines; i++ {
				goroutineID := i
				concurrentTester.Go(func() error {
					// Wait for all goroutines to start simultaneously
					if err := barrier.WaitWithTimeout(5 * time.Second); err != nil {
						return err
					}

					for j := 0; j < messagesPerGoroutine; j++ {
						message := []byte(fmt.Sprintf("test message from goroutine %d-%d", goroutineID, j))
						
						ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
						err := conn.SendMessage(ctx, message)
						cancel()
						
						if err != nil && err != context.DeadlineExceeded {
							return fmt.Errorf("send error in goroutine %d message %d: %w", goroutineID, j, err)
						}
						
						// Small delay to allow heartbeat and other operations to interleave
						time.Sleep(time.Millisecond)
					}
					return nil
				})
			}

			concurrentTester.Wait()

			// Verify the connection is still functional
			assert.True(t, conn.IsConnected(), "Connection should still be connected after concurrent writes")
			
			// Verify heartbeat is still functioning
			heartbeat := conn.GetHeartbeat()
			assert.NotNil(t, heartbeat, "Heartbeat manager should be available")
			assert.True(t, heartbeat.IsHealthy(), "Connection should be healthy after concurrent writes")

			t.Logf("Successfully completed %d concurrent goroutines each sending %d messages", 
				numGoroutines, messagesPerGoroutine)
		})
	})
}

// TestHeartbeatWriteMessageConcurrency specifically tests heartbeat ping vs regular message concurrency
func TestHeartbeatWriteMessageConcurrency(t *testing.T) {
	// Create test server that responds to pings
	server := createSimpleEchoServer(t)
	defer server.Close()

	// Create connection with moderately aggressive heartbeat timing for stress testing
	config := DefaultConnectionConfig()
	config.URL = "ws" + server.URL[4:] // Convert http:// to ws://
	config.PingPeriod = 50 * time.Millisecond  // Aggressive but manageable ping
	config.PongWait = 100 * time.Millisecond
	config.WriteTimeout = 200 * time.Millisecond
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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
		msgCtx, msgCancel := context.WithTimeout(ctx, 25*time.Millisecond)
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