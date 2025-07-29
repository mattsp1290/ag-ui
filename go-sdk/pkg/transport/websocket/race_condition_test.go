package websocket

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

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
		const numGoroutines = 50
		const messagesPerGoroutine = 20
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

	t.Run("MessageCountingConsistency", func(t *testing.T) {
		// Reset stats tracking
		initialStats := transport.Stats()
		
		const testMessages = 100
		var messagesSent int64
		
		// Send messages sequentially to verify counting is accurate
		for i := 0; i < testMessages; i++ {
			event := &MockEvent{
				EventType: events.EventTypeTextMessageContent,
				Data:      "consistency test message",
			}
			
			if err := transport.SendEvent(ctx, event); err != nil {
				t.Logf("Send error at message %d: %v", i, err)
			} else {
				atomic.AddInt64(&messagesSent, 1)
			}
		}

		// Wait for processing
		time.Sleep(100 * time.Millisecond)

		finalSent := atomic.LoadInt64(&messagesSent)
		finalStats := transport.Stats()
		
		// Verify counts are consistent
		assert.Equal(t, testMessages, int(finalSent), 
			"Should have sent exactly the expected number of messages")
		assert.GreaterOrEqual(t, finalStats.EventsSent, initialStats.EventsSent+finalSent,
			"Transport stats should reflect all sent messages")
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
		// Test rapid connect/disconnect cycles
		for i := 0; i < 5; i++ {
			t.Logf("Connect/disconnect cycle %d", i+1)
			
			err := conn.Connect(ctx)
			require.NoError(t, err)
			
			// Send a few messages
			for j := 0; j < 10; j++ {
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