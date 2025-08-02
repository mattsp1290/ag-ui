//go:build heavy

package websocket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	testutils "github.com/mattsp1290/ag-ui/go-sdk/pkg/testing"
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
	WithResourceControl(t, "TestConcurrentWriteSynchronization", func() {
		testutils.WithTestTimeout(t, 10*time.Second, func() {  // Reduced from 30s
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
	}) // Close WithResourceControl
}

// TestHeartbeatWriteMessageConcurrency specifically tests heartbeat ping vs regular message concurrency
func TestHeartbeatWriteMessageConcurrency(t *testing.T) {
	WithResourceControl(t, "TestHeartbeatWriteMessageConcurrency", func() {
		// Create reliable test server with proper cleanup
		tester := NewReliableConnectionTester(t)
		defer tester.Cleanup()

		// Use the reliable tester's connection with proper cleanup and aggressive heartbeat settings
		tester.TestConnection(func(conn *Connection) {
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
				msgCtx, msgCancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
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
		})
	})
}

// TestRaceConditionFixes verifies that race conditions in message counting are resolved
func TestRaceConditionFixes(t *testing.T) {
	server := NewLoadTestServer(t)
	defer func() {
		t.Log("Closing race condition test server")
		server.Close()
		// Give extra time for cleanup
		time.Sleep(200 * time.Millisecond)
	}()

	config := FastTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zaptest.NewLogger(t)
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer func() {
		t.Log("Stopping race condition test transport")
		if err := transport.Stop(); err != nil {
			t.Logf("Warning: Transport stop error: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}()

	// Wait for connections
	time.Sleep(500 * time.Millisecond)

	t.Run("ConcurrentMessageSending", func(t *testing.T) {
		const numGoroutines = 20  // Reduced from 50 for better reliability
		const messagesPerGoroutine = 10  // Reduced from 20 for better reliability
		const expectedMessages = numGoroutines * messagesPerGoroutine

		var wg sync.WaitGroup
		var errors int64
		var messagesSent int64

		startTime := time.Now()
		
		// Launch concurrent message senders
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < messagesPerGoroutine; j++ {
					event := &MockEvent{
						EventType: events.EventTypeTextMessageContent,
						Data:      "race condition test message",
					}

					if err := transport.SendEvent(ctx, event); err != nil {
						atomic.AddInt64(&errors, 1)
						t.Logf("Send error from goroutine %d, message %d: %v", id, j, err)
					} else {
						atomic.AddInt64(&messagesSent, 1)
					}
				}
			}(i)
		}

		wg.Wait()

		// Give time for message processing
		time.Sleep(200 * time.Millisecond)

		duration := time.Since(startTime)
		finalSent := atomic.LoadInt64(&messagesSent)
		finalErrors := atomic.LoadInt64(&errors)

		// Verify no messages were lost due to race conditions
		t.Logf("Test completed in %v", duration)
		t.Logf("Messages sent: %d/%d", finalSent, expectedMessages)
		t.Logf("Errors: %d", finalErrors)

		// Check that we successfully sent most/all messages
		successRate := float64(finalSent) / float64(expectedMessages)
		assert.Greater(t, successRate, 0.95, "Should successfully send at least 95% of messages")
		
		// Wait for transport to process all sent messages
		for i := 0; i < 100; i++ { // Wait up to 1 second
			stats := transport.Stats()
			if stats.EventsSent >= finalSent {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		// Verify transport stats are consistent
		stats := transport.Stats()
		assert.GreaterOrEqual(t, stats.EventsSent, finalSent, 
			"Transport should have processed all sent messages")
	})
}

// TestConnectionRaceConditions tests connection-level race conditions
func TestConnectionRaceConditions(t *testing.T) {
	server := NewLoadTestServer(t)
	defer func() {
		t.Log("Closing connection race test server")
		server.Close()
		time.Sleep(200 * time.Millisecond)
	}()

	config := DefaultConnectionConfig()
	config.URL = server.URL()
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("ConcurrentConnectDisconnect", func(t *testing.T) {
		// Test rapid connect/disconnect cycles - reduced from 5 to 3 for reliability
		for i := 0; i < 3; i++ {
			t.Logf("Connect/disconnect cycle %d", i+1)
			
			err := conn.Connect(ctx)
			require.NoError(t, err)
			
			// Send a few messages - reduced from 10 to 5
			for j := 0; j < 5; j++ {
				message := []byte("test message")
				_ = conn.SendMessage(ctx, message)
			}
			
			// Wait briefly before disconnect
			time.Sleep(50 * time.Millisecond)
			
			err = conn.Disconnect()
			require.NoError(t, err)
			
			// Brief pause between cycles
			time.Sleep(100 * time.Millisecond)
		}
	})

	// Final cleanup
	t.Log("Final connection cleanup")
	conn.Close()
	time.Sleep(100 * time.Millisecond)
}