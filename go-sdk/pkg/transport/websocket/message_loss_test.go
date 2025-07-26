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

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// TestMessageLossPrevention specifically tests the 1000 message scenario that was failing
func TestMessageLossPrevention(t *testing.T) {
	server := NewLoadTestServer(t)
	defer func() {
		t.Log("Closing message loss test server")
		server.Close()
		time.Sleep(200 * time.Millisecond)
	}()

	config := FastTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zaptest.NewLogger(t)
	config.EnableEventValidation = false
	config.PoolConfig.MinConnections = 5
	config.PoolConfig.MaxConnections = 10

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer func() {
		t.Log("Stopping message loss test transport")
		if err := transport.Stop(); err != nil {
			t.Logf("Warning: Transport stop error: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}()

	// Wait for connections to be established
	time.Sleep(1 * time.Second)

	t.Run("Exactly1000MessagesTest", func(t *testing.T) {
		const totalMessages = 1000
		const numWorkers = 20
		const messagesPerWorker = totalMessages / numWorkers

		var wg sync.WaitGroup
		var messagesSent int64
		var sendErrors int64

		startTime := time.Now()

		// Send exactly 1000 messages across workers
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < messagesPerWorker; j++ {
					event := &MockEvent{
						EventType: events.EventTypeTextMessageContent,
						Data:      "message loss test",
					}

					sendStart := time.Now()
					err := transport.SendEvent(ctx, event)
					if err != nil {
						atomic.AddInt64(&sendErrors, 1)
						t.Logf("Send error from worker %d, message %d: %v", workerID, j, err)
						
						// Check if we should continue on error
						if ctx.Err() != nil {
							return
						}
					} else {
						sent := atomic.AddInt64(&messagesSent, 1)
						
						// Log progress every 100 messages
						if sent%100 == 0 {
							t.Logf("Progress: %d/1000 messages sent (%.1f%%) - Send time: %v", 
								sent, float64(sent)/10.0, time.Since(sendStart))
						}
					}
				}
			}(i)
		}

		// Wait for all sending to complete
		wg.Wait()
		sendDuration := time.Since(startTime)

		// Verify we attempted to send the right number of messages
		finalSent := atomic.LoadInt64(&messagesSent)
		finalErrors := atomic.LoadInt64(&sendErrors)
		
		t.Logf("Sending completed in %v", sendDuration)
		t.Logf("Messages sent successfully: %d", finalSent)
		t.Logf("Send errors: %d", finalErrors)
		t.Logf("Total send attempts: %d", finalSent+finalErrors)

		// We should have sent 1000 messages with minimal errors
		assert.Equal(t, int64(totalMessages), finalSent+finalErrors, 
			"Should have attempted to send exactly 1000 messages")
		assert.LessOrEqual(t, finalErrors, int64(5), 
			"Should have minimal send errors (≤5)")
		assert.GreaterOrEqual(t, finalSent, int64(995), 
			"Should successfully send at least 995/1000 messages")

		// Wait for transport to process all sent messages
		t.Log("Waiting for transport to process all messages...")
		maxWaitTime := 5 * time.Second
		waitStart := time.Now()
		
		for time.Since(waitStart) < maxWaitTime {
			stats := transport.Stats()
			if stats.EventsSent >= finalSent {
				t.Logf("Transport processed all %d messages in %v", 
					stats.EventsSent, time.Since(waitStart))
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		// Final verification
		finalStats := transport.Stats()
		t.Logf("Final transport stats: EventsSent=%d, BytesTransferred=%d", 
			finalStats.EventsSent, finalStats.BytesTransferred)

		// The key assertion: transport should have processed all sent messages
		assert.GreaterOrEqual(t, finalStats.EventsSent, finalSent,
			"Transport should have processed all %d sent messages, but only processed %d (lost %d)",
			finalSent, finalStats.EventsSent, finalSent-finalStats.EventsSent)
		
		// Also verify we're getting close to our expected 1000
		if finalStats.EventsSent < 999 {
			t.Errorf("REGRESSION: Expected ~1000 messages, but only got %d (missing %d)", 
				finalStats.EventsSent, 1000-finalStats.EventsSent)
		} else {
			t.Logf("SUCCESS: Processed %d/1000 messages (within acceptable range)", 
				finalStats.EventsSent)
		}
	})

	t.Run("MessageCountingConsistency", func(t *testing.T) {
		// Additional test to verify counting doesn't drift over time
		const batchSize = 100
		const numBatches = 5
		
		initialStats := transport.Stats()
		var totalSent int64
		
		for batch := 0; batch < numBatches; batch++ {
			t.Logf("Sending batch %d/%d", batch+1, numBatches)
			
			var batchWg sync.WaitGroup
			var batchSent int64
			
			for i := 0; i < batchSize; i++ {
				batchWg.Add(1)
				go func() {
					defer batchWg.Done()
					
					event := &MockEvent{
						EventType: events.EventTypeTextMessageContent,
						Data:      "consistency test",
					}
					
					if err := transport.SendEvent(ctx, event); err == nil {
						atomic.AddInt64(&batchSent, 1)
						atomic.AddInt64(&totalSent, 1)
					}
				}()
			}
			
			batchWg.Wait()
			t.Logf("Batch %d: sent %d messages", batch+1, batchSent)
			
			// Brief pause between batches
			time.Sleep(100 * time.Millisecond)
		}

		// Wait for processing
		time.Sleep(500 * time.Millisecond)
		
		finalStats := transport.Stats()
		expectedTotal := initialStats.EventsSent + totalSent
		
		t.Logf("Consistency test: sent %d messages, transport processed %d total", 
			totalSent, finalStats.EventsSent)
		
		assert.GreaterOrEqual(t, finalStats.EventsSent, expectedTotal,
			"Message counting should be consistent across batches")
	})
}