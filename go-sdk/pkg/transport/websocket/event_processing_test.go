package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// TestEventProcessingPipeline tests the event processing pipeline
func TestEventProcessingPipeline(t *testing.T) {
	// Create transport
	config := DefaultTransportConfig()
	config.Logger = zap.NewNop()
	config.URLs = []string{"ws://localhost:8080"} // Dummy URL

	transport, err := NewTransport(config)
	require.NoError(t, err)

	// Track received events
	receivedEvents := make([]events.Event, 0)
	var mu sync.Mutex

	// Subscribe to test events
	sub, err := transport.Subscribe(context.Background(), []string{"test.event"}, func(ctx context.Context, event events.Event) error {
		mu.Lock()
		receivedEvents = append(receivedEvents, event)
		mu.Unlock()
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, sub)

	// Simulate receiving an event through the event channel
	testEventData := map[string]interface{}{
		"type": "test.event",
		"id":   "test-123",
		"data": "test data",
	}

	eventJSON, err := json.Marshal(testEventData)
	require.NoError(t, err)

	// Send event to the channel
	select {
	case transport.eventCh <- eventJSON:
		// Event sent successfully
	case <-time.After(5 * time.Second):
		t.Fatal("Failed to send event to channel")
	}

	// Start event processing in background using standard method
	transport.startGoroutine("event-processing", transport.eventProcessingLoop)

	// Wait for event to be processed
	time.Sleep(500 * time.Millisecond)

	// Verify event was received
	mu.Lock()
	assert.Len(t, receivedEvents, 1)
	if len(receivedEvents) > 0 {
		assert.Equal(t, events.EventType("test.event"), receivedEvents[0].Type())
	}
	mu.Unlock()

	// Verify stats
	stats := transport.GetStats()
	assert.Equal(t, int64(1), stats.EventsReceived)
	assert.Equal(t, int64(1), stats.EventsProcessed)
	assert.Equal(t, int64(0), stats.EventsFailed)
}

// TestEventChannelCapacity tests that the event channel can handle multiple events
func TestEventChannelCapacity(t *testing.T) {
	config := DefaultTransportConfig()
	config.Logger = zap.NewNop()
	config.URLs = []string{"ws://localhost:8080"}

	transport, err := NewTransport(config)
	require.NoError(t, err)

	// Send multiple events without processing
	for i := 0; i < 100; i++ {
		eventData := map[string]interface{}{
			"type": "test.event",
			"id":   i,
		}
		eventJSON, _ := json.Marshal(eventData)

		select {
		case transport.eventCh <- eventJSON:
			// Event sent successfully
		default:
			t.Logf("Channel full at event %d", i)
		}
	}

	// Verify channel has events
	assert.Greater(t, len(transport.eventCh), 0)
	assert.LessOrEqual(t, len(transport.eventCh), cap(transport.eventCh))
}

// TestEventProcessingShutdown tests graceful shutdown of event processing
func TestEventProcessingShutdown(t *testing.T) {
	config := DefaultTransportConfig()
	config.Logger = zap.NewNop()
	config.URLs = []string{"ws://localhost:8080"}

	// Mock the connection pool to avoid actual connections
	config.PoolConfig = DefaultPoolConfig()
	config.PoolConfig.MinConnections = 1 // Minimum required

	transport, err := NewTransport(config)
	require.NoError(t, err)

	// Start the event processing loop using the standard method
	transport.startGoroutine("event-processing", transport.eventProcessingLoop)

	// Send an event
	eventData := map[string]interface{}{
		"type": "test.event",
	}
	eventJSON, _ := json.Marshal(eventData)

	select {
	case transport.eventCh <- eventJSON:
		// Event sent
	case <-time.After(5 * time.Second):
		t.Fatal("Failed to send event")
	}

	// Give some time for processing
	time.Sleep(200 * time.Millisecond)

	// Cancel the transport context to trigger shutdown
	transport.cancel()

	// Wait for the event processing loop to finish
	done := make(chan struct{})
	go func() {
		transport.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Successfully shut down
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown timeout")
	}
}
