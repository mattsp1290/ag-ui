package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// TestEventHandlerReliableRemoval tests that event handlers can be reliably added and removed
func TestEventHandlerReliableRemoval(t *testing.T) {
	// Create a transport with minimal config
	config := &TransportConfig{
		URLs:                  []string{"ws://localhost:8080"},
		PoolConfig:            DefaultPoolConfig(),
		PerformanceConfig:     DefaultPerformanceConfig(),
		EventTimeout:          30 * time.Second,
		MaxEventSize:          1024 * 1024,
		EnableEventValidation: false,
		Logger:                zap.NewNop(),
	}

	transport, err := NewTransport(config)
	require.NoError(t, err)
	require.NotNil(t, transport)

	// Test 1: Add and remove a single handler
	t.Run("SingleHandlerAddRemove", func(t *testing.T) {
		eventType := "test.event"

		handler := func(ctx context.Context, event events.Event) error {
			return nil
		}

		// Add handler
		handlerID := transport.AddEventHandler(eventType, handler)
		assert.NotEmpty(t, handlerID, "Handler ID should not be empty")

		// Verify handler was added
		transport.handlersMutex.RLock()
		handlers, exists := transport.eventHandlers[eventType]
		transport.handlersMutex.RUnlock()
		assert.True(t, exists, "Event type should exist in handlers map")
		assert.Len(t, handlers, 1, "Should have exactly one handler")

		// Remove handler
		err := transport.RemoveEventHandler(eventType, handlerID)
		assert.NoError(t, err, "RemoveEventHandler should not return error")

		// Verify handler was removed
		transport.handlersMutex.RLock()
		handlers, exists = transport.eventHandlers[eventType]
		transport.handlersMutex.RUnlock()
		assert.False(t, exists, "Event type should not exist after removal")
	})

	// Test 2: Add multiple handlers and remove specific ones
	t.Run("MultipleHandlersSelectiveRemoval", func(t *testing.T) {
		eventType := "test.multi.event"

		handler1 := func(ctx context.Context, event events.Event) error {
			return nil
		}
		handler2 := func(ctx context.Context, event events.Event) error {
			return nil
		}
		handler3 := func(ctx context.Context, event events.Event) error {
			return nil
		}

		// Add handlers
		id1 := transport.AddEventHandler(eventType, handler1)
		id2 := transport.AddEventHandler(eventType, handler2)
		id3 := transport.AddEventHandler(eventType, handler3)

		// Verify all handlers were added
		assert.Equal(t, 3, getHandlerCount(transport, eventType))

		// Remove handler2
		err := transport.RemoveEventHandler(eventType, id2)
		assert.NoError(t, err)
		assert.Equal(t, 2, getHandlerCount(transport, eventType))

		// Verify the correct handlers remain
		transport.handlersMutex.RLock()
		handlers := transport.eventHandlers[eventType]
		transport.handlersMutex.RUnlock()

		remainingIDs := make([]string, 0)
		for _, h := range handlers {
			remainingIDs = append(remainingIDs, h.ID)
		}
		assert.Contains(t, remainingIDs, id1)
		assert.Contains(t, remainingIDs, id3)
		assert.NotContains(t, remainingIDs, id2)
	})

	// Test 3: Test subscription with handler tracking
	t.Run("SubscriptionWithHandlerTracking", func(t *testing.T) {
		ctx := context.Background()
		eventTypes := []string{"event1", "event2", "event3"}

		handler := func(ctx context.Context, event events.Event) error {
			return nil
		}

		// Create subscription
		sub, err := transport.Subscribe(ctx, eventTypes, handler)
		require.NoError(t, err)
		require.NotNil(t, sub)

		// Verify handler IDs were tracked
		assert.Len(t, sub.HandlerIDs, len(eventTypes))
		for _, id := range sub.HandlerIDs {
			assert.NotEmpty(t, id)
		}

		// Verify handlers were added for each event type
		for _, eventType := range eventTypes {
			assert.Equal(t, 1, getHandlerCount(transport, eventType))
		}

		// Unsubscribe
		err = transport.Unsubscribe(sub.ID)
		assert.NoError(t, err)

		// Verify all handlers were removed
		for _, eventType := range eventTypes {
			assert.Equal(t, 0, getHandlerCount(transport, eventType))
		}
	})

	// Test 4: Test error cases
	t.Run("ErrorCases", func(t *testing.T) {
		// Try to remove non-existent handler
		err := transport.RemoveEventHandler("non.existent.event", "fake-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no handlers found")

		// Add a handler then try to remove with wrong ID
		eventType := "test.error.event"
		handlerID := transport.AddEventHandler(eventType, func(ctx context.Context, event events.Event) error {
			return nil
		})

		err = transport.RemoveEventHandler(eventType, "wrong-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		// Clean up
		_ = transport.RemoveEventHandler(eventType, handlerID)

		// Try to remove with empty handler ID
		err = transport.RemoveEventHandler("any.event", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "handler ID cannot be empty")
	})

	// Test 5: Verify handler IDs are unique
	t.Run("UniqueHandlerIDs", func(t *testing.T) {
		eventType := "test.unique.event"
		idSet := make(map[string]bool)

		// Add 100 handlers and verify all IDs are unique
		for i := 0; i < 100; i++ {
			id := transport.AddEventHandler(eventType, func(ctx context.Context, event events.Event) error {
				return nil
			})
			assert.NotEmpty(t, id)
			assert.False(t, idSet[id], "Handler ID should be unique")
			idSet[id] = true
		}

		// Clean up
		transport.handlersMutex.Lock()
		delete(transport.eventHandlers, eventType)
		transport.handlersMutex.Unlock()
	})
}

// Helper function to get handler count for an event type
func getHandlerCount(t *Transport, eventType string) int {
	t.handlersMutex.RLock()
	defer t.handlersMutex.RUnlock()

	if handlers, exists := t.eventHandlers[eventType]; exists {
		return len(handlers)
	}
	return 0
}
