package websocket

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// TestSimpleTransportLeakDetection creates fewer transports to isolate the leak
func TestSimpleTransportLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping transport leak detection test in short mode")
	}
	
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutines: %d", initialGoroutines)

	server := NewLoadTestServer(t)
	defer server.Close()

	// Create just 10 transports to isolate the leak
	const numTransports = 10
	const messagesPerTransport = 2

	var wg sync.WaitGroup

	for i := 0; i < numTransports; i++ {
		wg.Add(1)
		go func(transportID int) {
			defer wg.Done()

			// Create transport config with minimal connections
			config := DefaultTransportConfig()
			config.URLs = []string{server.URL()}
			config.Logger = zap.NewNop()
			config.PoolConfig.MinConnections = 1
			config.PoolConfig.MaxConnections = 1

			transport, err := NewTransport(config)
			if err != nil {
				t.Logf("Transport %d: failed to create: %v", transportID, err)
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Start the transport
			if err := transport.Start(ctx); err != nil {
				t.Logf("Transport %d: failed to start: %v", transportID, err)
				return
			}

			// Send minimal messages
			for j := 0; j < messagesPerTransport; j++ {
				event := &MockEvent{
					EventType: events.EventTypeTextMessageContent,
					Data:      fmt.Sprintf("simple_transport_%d_message_%d", transportID, j),
				}

				msgCtx, msgCancel := context.WithTimeout(ctx, 500*time.Millisecond)
				err := transport.SendEvent(msgCtx, event)
				msgCancel()
				
				if err != nil {
					t.Logf("Transport %d: failed to send message %d: %v", transportID, j, err)
				}

				time.Sleep(10 * time.Millisecond)
			}

			// Stop the transport
			if err := transport.Stop(); err != nil {
				t.Logf("Transport %d: failed to stop: %v", transportID, err)
			}

			t.Logf("Transport %d completed", transportID)
		}(i)
	}

	// Wait for all transports to complete
	wg.Wait()

	t.Logf("All transports completed, running cleanup...")

	// Force garbage collection multiple times
	for i := 0; i < 5; i++ {
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
	}

	// Check final goroutine count
	finalGoroutines := runtime.NumGoroutine()
	t.Logf("Final goroutines: %d (diff: %d)", finalGoroutines, finalGoroutines-initialGoroutines)

	// With just 10 transports, we should have very few leaked goroutines
	maxAllowedIncrease := 5 
	if finalGoroutines-initialGoroutines > maxAllowedIncrease {
		t.Errorf("Simple goroutine leak detected: started with %d, ended with %d (increase: %d)",
			initialGoroutines, finalGoroutines, finalGoroutines-initialGoroutines)
	}
}

// TestSingleTransportLifecycle tests a single transport lifecycle
func TestSingleTransportLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping transport lifecycle test in short mode")
	}
	
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutines: %d", initialGoroutines)

	server := NewLoadTestServer(t)
	defer server.Close()

	// Create minimal transport config
	config := DefaultTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zap.NewNop()
	config.PoolConfig.MinConnections = 1
	config.PoolConfig.MaxConnections = 1

	transport, err := NewTransport(config)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start the transport
	err = transport.Start(ctx)
	assert.NoError(t, err)

	// Send one message
	event := &MockEvent{
		EventType: events.EventTypeTextMessageContent,
		Data:      "single_transport_test",
	}

	msgCtx, msgCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	err = transport.SendEvent(msgCtx, event)
	msgCancel()
	
	if err != nil {
		t.Logf("Failed to send message: %v", err)
	}

	// Stop the transport
	err = transport.Stop()
	assert.NoError(t, err)

	// Force cleanup
	for i := 0; i < 5; i++ {
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
	}

	finalGoroutines := runtime.NumGoroutine()
	t.Logf("Single transport: Initial %d, Final %d (diff: %d)",
		initialGoroutines, finalGoroutines, finalGoroutines-initialGoroutines)

	// Single transport should have minimal leak
	assert.InDelta(t, initialGoroutines, finalGoroutines, 3,
		"Single transport should not leak significant goroutines")
}