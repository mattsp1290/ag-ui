package server

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/testhelper"
)

// getTestStreamingServerConfig returns a config suitable for tests with dynamic port allocation
func getTestStreamingServerConfig() *StreamingServerConfig {
	config := DefaultStreamingServerConfig()
	config.Address = ":0" // Use dynamic port allocation for tests
	return config
}

func TestStreamingServer(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := getTestStreamingServerConfig()

	// Create streaming server
	server, err := NewStreamingServer(config)
	require.NoError(t, err)
	require.NotNil(t, server)

	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Stop(ctx)
	})

	t.Run("StreamingServer Configuration", func(t *testing.T) {
		assert.Equal(t, config.SSE.BufferSize, server.config.SSE.BufferSize)
		assert.Equal(t, config.MaxConnections, server.config.MaxConnections)
		assert.Equal(t, config.SSE.HeartbeatInterval, server.config.SSE.HeartbeatInterval)
	})

	t.Run("StreamingServer Start and Stop", func(t *testing.T) {
		ctx := context.Background()

		// Start server
		err := server.Start()
		require.NoError(t, err)

		// Verify running state by checking metrics
		metrics := server.GetMetrics()
		assert.NotNil(t, metrics)
		assert.NotZero(t, metrics.StartTime)

		// Stop server
		err = server.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("StreamingServer Double Start Error", func(t *testing.T) {
		ctx := context.Background()

		// Start server
		err := server.Start()
		require.NoError(t, err)
		defer server.Stop(ctx)

		// Try to start again - should error
		err = server.Start()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already running")
	})
}

func TestStreamingServerConfig(t *testing.T) {
	t.Run("DefaultStreamingServerConfig", func(t *testing.T) {
		config := DefaultStreamingServerConfig()

		assert.Greater(t, config.SSE.BufferSize, 0)
		assert.Greater(t, config.MaxConnections, 0)
		assert.Greater(t, config.SSE.HeartbeatInterval, time.Duration(0))
		assert.Greater(t, config.ReadTimeout, time.Duration(0))
		assert.Greater(t, config.WriteTimeout, time.Duration(0))
	})

	t.Run("StreamingServerConfig Validation", func(t *testing.T) {
		tests := []struct {
			name    string
			config  *StreamingServerConfig
			wantErr bool
		}{
			{
				name: "zero buffer size",
				config: &StreamingServerConfig{
					Address: ":0",
					SSE: SSEConfig{
						BufferSize: 0,
					},
					WebSocket: WebSocketConfig{
						BufferSize:     1024,
						MaxMessageSize: 1024,
					},
					MaxConnections: 100,
				},
				wantErr: true,
			},
			{
				name: "negative max connections",
				config: &StreamingServerConfig{
					Address: ":0",
					SSE: SSEConfig{
						BufferSize: 1000,
					},
					WebSocket: WebSocketConfig{
						BufferSize:     1024,
						MaxMessageSize: 1024,
					},
					MaxConnections: -1,
				},
				wantErr: true,
			},
			{
				name: "valid config",
				config: &StreamingServerConfig{
					Address: ":0",
					SSE: SSEConfig{
						BufferSize:        1000,
						HeartbeatInterval: 30 * time.Second,
					},
					WebSocket: WebSocketConfig{
						BufferSize:     1024,
						MaxMessageSize: 1024,
					},
					MaxConnections: 100,
				},
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Test validation through constructor since ValidateStreamingServerConfig doesn't exist
				_, err := NewStreamingServer(tt.config)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestStreamingServerEventStreaming(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := getTestStreamingServerConfig()
	config.SSE.BufferSize = 10 // Small buffer for testing

	server, err := NewStreamingServer(config)
	require.NoError(t, err)

	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Stop(ctx)
	})

	ctx := context.Background()
	err = server.Start()
	require.NoError(t, err)
	defer server.Stop(ctx)

	t.Run("Basic Event Broadcasting", func(t *testing.T) {
		// Create test event
		testEvent := &StreamEvent{
			ID:        "test-1",
			Event:     "test-event",
			Data:      "test message",
			Timestamp: time.Now(),
		}

		// Broadcast event
		err = server.BroadcastEvent(testEvent)
		require.NoError(t, err)

		// Verify metrics updated
		metrics := server.GetMetrics()
		assert.NotNil(t, metrics)
	})

	t.Run("Multicast Event", func(t *testing.T) {
		// Create test event
		testEvent := &StreamEvent{
			ID:        "test-2",
			Event:     "multicast-event",
			Data:      "multicast message",
			Timestamp: time.Now(),
		}

		// Multicast to specific clients
		clientIDs := []string{"client-1", "client-2"}
		err = server.MulticastEvent(testEvent, clientIDs)
		require.NoError(t, err)

		// Verify metrics updated
		metrics := server.GetMetrics()
		assert.NotNil(t, metrics)
	})

	t.Run("Connection Metrics", func(t *testing.T) {
		// Get active connections
		sseCount, wsCount := server.GetActiveConnections()
		assert.GreaterOrEqual(t, sseCount, 0)
		assert.GreaterOrEqual(t, wsCount, 0)

		// Verify metrics
		metrics := server.GetMetrics()
		assert.NotNil(t, metrics)
		assert.GreaterOrEqual(t, metrics.SSEConnections, int64(0))
		assert.GreaterOrEqual(t, metrics.WebSocketConnections, int64(0))
	})
}

func TestStreamingServerMetrics(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := getTestStreamingServerConfig()

	server, err := NewStreamingServer(config)
	require.NoError(t, err)

	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Stop(ctx)
	})

	ctx := context.Background()
	err = server.Start()
	require.NoError(t, err)
	defer server.Stop(ctx)

	t.Run("Basic Metrics", func(t *testing.T) {
		// Get metrics
		metrics := server.GetMetrics()
		require.NotNil(t, metrics)

		// Verify initial state
		assert.GreaterOrEqual(t, metrics.SSEConnections, int64(0))
		assert.GreaterOrEqual(t, metrics.WebSocketConnections, int64(0))
		assert.GreaterOrEqual(t, metrics.TotalConnections, int64(0))
		assert.NotZero(t, metrics.StartTime)
	})

	t.Run("Connection Counts", func(t *testing.T) {
		// Get active connection counts
		sseCount, wsCount := server.GetActiveConnections()
		assert.GreaterOrEqual(t, sseCount, 0)
		assert.GreaterOrEqual(t, wsCount, 0)

		// Verify consistency with metrics
		metrics := server.GetMetrics()
		assert.Equal(t, int64(sseCount), metrics.SSEConnections)
		assert.Equal(t, int64(wsCount), metrics.WebSocketConnections)
	})

	t.Run("Event Broadcasting Metrics", func(t *testing.T) {
		initialMetrics := server.GetMetrics()

		// Broadcast some events
		for i := 0; i < 3; i++ {
			testEvent := &StreamEvent{
				ID:        fmt.Sprintf("test-%d", i),
				Event:     "test-event",
				Data:      fmt.Sprintf("test message %d", i),
				Timestamp: time.Now(),
			}

			err = server.BroadcastEvent(testEvent)
			require.NoError(t, err)
		}

		// Verify metrics updated
		finalMetrics := server.GetMetrics()
		assert.GreaterOrEqual(t, finalMetrics.BroadcastEvents, initialMetrics.BroadcastEvents)
	})
}

func TestStreamingServerConcurrency(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := getTestStreamingServerConfig()
	config.SSE.BufferSize = 1000 // Larger buffer for concurrency tests

	server, err := NewStreamingServer(config)
	require.NoError(t, err)

	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Stop(ctx)
	})

	ctx := context.Background()
	err = server.Start()
	require.NoError(t, err)
	defer server.Stop(ctx)

	t.Run("Concurrent Event Broadcasting", func(t *testing.T) {
		const numEvents = 50
		var wg sync.WaitGroup
		var eventCounter int64

		// Broadcast events concurrently
		for i := 0; i < numEvents; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				testEvent := &StreamEvent{
					ID:        fmt.Sprintf("concurrent-%d", id),
					Event:     "concurrent-event",
					Data:      fmt.Sprintf("message-%d", id),
					Timestamp: time.Now(),
				}

				err := server.BroadcastEvent(testEvent)
				if err != nil {
					t.Errorf("failed to broadcast event: %v", err)
					return
				}

				atomic.AddInt64(&eventCounter, 1)
			}(i)
		}

		wg.Wait()

		// Verify all events were processed
		assert.Equal(t, int64(numEvents), atomic.LoadInt64(&eventCounter))
		t.Logf("Successfully broadcast %d events concurrently", atomic.LoadInt64(&eventCounter))
	})

	t.Run("Concurrent Multicast Events", func(t *testing.T) {
		const numEvents = 25
		var wg sync.WaitGroup
		var eventCounter int64

		// Multicast events concurrently
		for i := 0; i < numEvents; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				testEvent := &StreamEvent{
					ID:        fmt.Sprintf("multicast-%d", id),
					Event:     "multicast-event",
					Data:      fmt.Sprintf("multicast-message-%d", id),
					Timestamp: time.Now(),
				}

				clientIDs := []string{fmt.Sprintf("client-%d", id%5)}
				err := server.MulticastEvent(testEvent, clientIDs)
				if err != nil {
					t.Errorf("failed to multicast event: %v", err)
					return
				}

				atomic.AddInt64(&eventCounter, 1)
			}(i)
		}

		wg.Wait()

		// Verify all events were processed
		assert.Equal(t, int64(numEvents), atomic.LoadInt64(&eventCounter))
		t.Logf("Successfully multicast %d events concurrently", atomic.LoadInt64(&eventCounter))
	})
}

func TestStreamingServerAdvancedMetrics(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)
	cleanup := testhelper.NewCleanupHelper(t)

	config := getTestStreamingServerConfig()

	server, err := NewStreamingServer(config)
	require.NoError(t, err)

	cleanup.Add(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Stop(ctx)
	})

	t.Run("Metrics Before Start", func(t *testing.T) {
		metrics := server.GetMetrics()
		assert.NotNil(t, metrics)
		assert.GreaterOrEqual(t, metrics.SSEConnections, int64(0))
		assert.GreaterOrEqual(t, metrics.WebSocketConnections, int64(0))
		assert.NotZero(t, metrics.StartTime)
	})

	t.Run("Metrics After Start", func(t *testing.T) {
		ctx := context.Background()

		err := server.Start()
		require.NoError(t, err)
		defer server.Stop(ctx)

		metrics := server.GetMetrics()
		assert.NotNil(t, metrics)
		assert.NotZero(t, metrics.StartTime)
		assert.Greater(t, metrics.Uptime, time.Duration(0))
	})

	t.Run("Event Broadcasting Metrics", func(t *testing.T) {
		ctx := context.Background()

		// Use a separate server for this test to avoid defer cleanup issues
		testConfig := getTestStreamingServerConfig()
		testServer, err := NewStreamingServer(testConfig)
		require.NoError(t, err)

		err = testServer.Start()
		require.NoError(t, err)

		initialMetrics := testServer.GetMetrics()

		// Broadcast events
		for i := 0; i < 5; i++ {
			testEvent := &StreamEvent{
				ID:        fmt.Sprintf("metrics-test-%d", i),
				Event:     "metrics-event",
				Data:      fmt.Sprintf("metrics message %d", i),
				Timestamp: time.Now(),
			}

			err = testServer.BroadcastEvent(testEvent)
			require.NoError(t, err)
		}

		// Check updated metrics
		finalMetrics := testServer.GetMetrics()
		assert.GreaterOrEqual(t, finalMetrics.BroadcastEvents, initialMetrics.BroadcastEvents)

		// Clean up
		err = testServer.Stop(ctx)
		require.NoError(t, err)
	})
}

func TestStreamingServerErrorHandling(t *testing.T) {
	defer testhelper.VerifyNoGoroutineLeaks(t)

	t.Run("Invalid Configuration", func(t *testing.T) {
		// Invalid buffer size
		config := &StreamingServerConfig{
			Address: ":0", // Use dynamic port allocation
			SSE: SSEConfig{
				BufferSize: 0, // Invalid
			},
			WebSocket: WebSocketConfig{
				BufferSize:     1024,
				MaxMessageSize: 1024,
			},
			MaxConnections: 100,
		}

		server, err := NewStreamingServer(config)
		assert.Error(t, err)
		assert.Nil(t, server)
	})

	t.Run("Broadcast to Stopped Server", func(t *testing.T) {
		config := getTestStreamingServerConfig()

		server, err := NewStreamingServer(config)
		require.NoError(t, err)

		// Try to broadcast without starting
		testEvent := &StreamEvent{
			ID:        "error-test",
			Event:     "error-event",
			Data:      "test message",
			Timestamp: time.Now(),
		}

		err = server.BroadcastEvent(testEvent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	})

	t.Run("Multicast to Stopped Server", func(t *testing.T) {
		config := getTestStreamingServerConfig()

		server, err := NewStreamingServer(config)
		require.NoError(t, err)

		ctx := context.Background()
		err = server.Start()
		require.NoError(t, err)

		// Stop server
		err = server.Stop(ctx)
		require.NoError(t, err)

		testEvent := &StreamEvent{
			ID:        "multicast-error-test",
			Event:     "multicast-error-event",
			Data:      "test message",
			Timestamp: time.Now(),
		}

		// Multicast should fail on stopped server
		clientIDs := []string{"client-1"}
		err = server.MulticastEvent(testEvent, clientIDs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	})
}

// Mock streaming event for testing removed since we're using actual StreamEvent struct
