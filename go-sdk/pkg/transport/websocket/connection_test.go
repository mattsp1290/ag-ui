package websocket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestConnectionBasicOperations(t *testing.T) {
	// Setup test WebSocket server
	server := createTestWebSocketServer(t)
	defer server.Close()

	// Create connection
	config := DefaultConnectionConfig()
	config.URL = "ws" + strings.TrimPrefix(server.URL, "http")
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)
	require.NotNil(t, conn)

	// Test initial state
	assert.Equal(t, StateDisconnected, conn.State())
	assert.False(t, conn.IsConnected())

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = conn.Connect(ctx)
	require.NoError(t, err)
	assert.Equal(t, StateConnected, conn.State())
	assert.True(t, conn.IsConnected())

	// Test sending message
	err = conn.SendMessage(ctx, []byte("test message"))
	require.NoError(t, err)

	// Test disconnection
	err = conn.Disconnect()
	require.NoError(t, err)
	assert.Equal(t, StateDisconnected, conn.State())
	assert.False(t, conn.IsConnected())

	// Test close
	err = conn.Close()
	require.NoError(t, err)
	assert.Equal(t, StateClosed, conn.State())
}

func TestConnectionStateTransitions(t *testing.T) {
	config := DefaultConnectionConfig()
	config.URL = "ws://localhost:8080"
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)

	// Test valid state transitions
	assert.True(t, conn.setState(StateConnecting))
	assert.Equal(t, StateConnecting, conn.State())

	assert.True(t, conn.setState(StateConnected))
	assert.Equal(t, StateConnected, conn.State())

	assert.True(t, conn.setState(StateReconnecting))
	assert.Equal(t, StateReconnecting, conn.State())

	assert.True(t, conn.setState(StateDisconnected))
	assert.Equal(t, StateDisconnected, conn.State())

	assert.True(t, conn.setState(StateClosed))
	assert.Equal(t, StateClosed, conn.State())

	// Test invalid state transitions
	assert.False(t, conn.setState(StateConnecting))
	assert.Equal(t, StateClosed, conn.State())
}

func TestConnectionReconnection(t *testing.T) {
	// Setup test WebSocket server that can be stopped and started
	server := createTestWebSocketServer(t)

	config := DefaultConnectionConfig()
	config.URL = "ws" + strings.TrimPrefix(server.URL, "http")
	config.Logger = zaptest.NewLogger(t)
	config.MaxReconnectAttempts = 3
	config.InitialReconnectDelay = 100 * time.Millisecond
	config.ReadTimeout = 200 * time.Millisecond // Short timeout to detect disconnection quickly

	conn, err := NewConnection(config)
	require.NoError(t, err)

	// Connect initially
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = conn.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, conn.IsConnected())

	// Start auto-reconnect
	conn.StartAutoReconnect(ctx)

	// Send a message to ensure read pump is active
	err = conn.SendMessage(ctx, []byte("test"))
	require.NoError(t, err)

	// Close server to trigger reconnection
	server.Close()

	// Wait for the connection to detect the closed server and attempt reconnection
	// The read timeout is typically around 100ms, so wait a bit longer
	time.Sleep(1 * time.Second)

	// Check that reconnection was attempted
	attempts := conn.GetReconnectAttempts()
	t.Logf("Reconnect attempts: %d", attempts)
	assert.Greater(t, attempts, int32(0))

	// Close connection
	err = conn.Close()
	require.NoError(t, err)
}

func TestConnectionMetrics(t *testing.T) {
	server := createTestWebSocketServer(t)
	defer server.Close()

	config := DefaultConnectionConfig()
	config.URL = "ws" + strings.TrimPrefix(server.URL, "http")
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect
	err = conn.Connect(ctx)
	require.NoError(t, err)

	// Send messages
	for i := 0; i < 5; i++ {
		err = conn.SendMessage(ctx, []byte(fmt.Sprintf("message %d", i)))
		require.NoError(t, err)
	}

	// Wait for messages to be processed by write pump
	time.Sleep(100 * time.Millisecond)

	// Check metrics
	metrics := conn.GetMetrics()
	assert.Equal(t, int64(1), metrics.ConnectAttempts)
	assert.Equal(t, int64(1), metrics.SuccessfulConnects)
	assert.Equal(t, int64(5), metrics.MessagesSent)
	assert.Greater(t, metrics.BytesSent, int64(0))

	// Close
	err = conn.Close()
	require.NoError(t, err)
}

func TestConnectionEventHandlers(t *testing.T) {
	server := createTestWebSocketServer(t)
	defer server.Close()

	config := DefaultConnectionConfig()
	config.URL = "ws" + strings.TrimPrefix(server.URL, "http")
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)

	// Setup event handlers
	var connectCalled, disconnectCalled bool
	var mu sync.Mutex

	conn.SetOnConnect(func() {
		mu.Lock()
		defer mu.Unlock()
		connectCalled = true
	})

	conn.SetOnDisconnect(func(err error) {
		mu.Lock()
		defer mu.Unlock()
		disconnectCalled = true
	})

	conn.SetOnMessage(func(data []byte) {
		// Message handler - just log for testing
		_ = data
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect
	err = conn.Connect(ctx)
	require.NoError(t, err)

	// Wait for handlers to be called
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.True(t, connectCalled)
	mu.Unlock()

	// Disconnect
	err = conn.Disconnect()
	require.NoError(t, err)

	// Wait for handlers to be called
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.True(t, disconnectCalled)
	mu.Unlock()

	// Close
	err = conn.Close()
	require.NoError(t, err)
}

func TestConnectionConfiguration(t *testing.T) {
	// Test default configuration
	config := DefaultConnectionConfig()
	assert.Equal(t, 10, config.MaxReconnectAttempts)
	assert.Equal(t, 1*time.Second, config.InitialReconnectDelay)
	assert.Equal(t, 30*time.Second, config.MaxReconnectDelay)
	assert.Equal(t, 2.0, config.ReconnectBackoffMultiplier)
	assert.Equal(t, 10*time.Second, config.HandshakeTimeout)
	assert.Equal(t, 60*time.Second, config.ReadTimeout)
	assert.Equal(t, 10*time.Second, config.WriteTimeout)
	assert.Equal(t, 30*time.Second, config.PingPeriod)
	assert.Equal(t, 35*time.Second, config.PongWait)
	assert.Equal(t, int64(1024*1024), config.MaxMessageSize)
	assert.Equal(t, 4096, config.WriteBufferSize)
	assert.Equal(t, 4096, config.ReadBufferSize)
	assert.True(t, config.EnableCompression)
	assert.NotNil(t, config.Headers)
	assert.NotNil(t, config.RateLimiter)
	assert.NotNil(t, config.Logger)

	// Test custom configuration
	customConfig := &ConnectionConfig{
		URL:                        "ws://example.com",
		MaxReconnectAttempts:       5,
		InitialReconnectDelay:      500 * time.Millisecond,
		MaxReconnectDelay:          10 * time.Second,
		ReconnectBackoffMultiplier: 1.5,
		HandshakeTimeout:           5 * time.Second,
		ReadTimeout:                30 * time.Second,
		WriteTimeout:               5 * time.Second,
		PingPeriod:                 15 * time.Second,
		PongWait:                   20 * time.Second,
		MaxMessageSize:             512 * 1024,
		WriteBufferSize:            2048,
		ReadBufferSize:             2048,
		EnableCompression:          false,
		Headers:                    map[string]string{"Custom": "Header"},
		Logger:                     zaptest.NewLogger(t),
	}

	conn, err := NewConnection(customConfig)
	require.NoError(t, err)
	assert.Equal(t, customConfig.URL, conn.config.URL)
	assert.Equal(t, customConfig.MaxReconnectAttempts, conn.config.MaxReconnectAttempts)
	assert.Equal(t, customConfig.InitialReconnectDelay, conn.config.InitialReconnectDelay)
}

func TestConnectionErrors(t *testing.T) {
	// Test invalid URL
	config := DefaultConnectionConfig()
	config.URL = ""
	_, err := NewConnection(config)
	assert.Error(t, err)

	// Test invalid scheme
	config.URL = "http://example.com"
	_, err = NewConnection(config)
	assert.Error(t, err)

	// Test malformed URL
	config.URL = "ws://[invalid-url"
	_, err = NewConnection(config)
	assert.Error(t, err)

	// Test connection to non-existent server
	config.URL = "ws://localhost:9999"
	conn, err := NewConnection(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = conn.Connect(ctx)
	assert.Error(t, err)
	assert.Equal(t, StateDisconnected, conn.State())
}

func TestConnectionConcurrency(t *testing.T) {
	server := createTestWebSocketServer(t)
	defer server.Close()

	config := DefaultConnectionConfig()
	config.URL = "ws" + strings.TrimPrefix(server.URL, "http")
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = conn.Connect(ctx)
	require.NoError(t, err)

	// Send messages concurrently
	var wg sync.WaitGroup
	numGoroutines := 10
	messagesPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				message := fmt.Sprintf("message from goroutine %d, iteration %d", id, j)
				err := conn.SendMessage(ctx, []byte(message))
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// Wait for all messages to be processed by the write pump
	time.Sleep(200 * time.Millisecond)

	// Check metrics
	metrics := conn.GetMetrics()
	assert.Equal(t, int64(numGoroutines*messagesPerGoroutine), metrics.MessagesSent)

	err = conn.Close()
	require.NoError(t, err)
}

func TestConnectionBackoffCalculation(t *testing.T) {
	config := DefaultConnectionConfig()
	config.URL = "ws://localhost:8080"
	config.InitialReconnectDelay = 1 * time.Second
	config.MaxReconnectDelay = 30 * time.Second
	config.ReconnectBackoffMultiplier = 2.0
	config.Logger = zaptest.NewLogger(t)

	conn, err := NewConnection(config)
	require.NoError(t, err)

	// Test backoff calculation
	assert.Equal(t, 1*time.Second, conn.calculateBackoffDelay(0))
	assert.Equal(t, 2*time.Second, conn.calculateBackoffDelay(1))
	assert.Equal(t, 4*time.Second, conn.calculateBackoffDelay(2))
	assert.Equal(t, 8*time.Second, conn.calculateBackoffDelay(3))
	assert.Equal(t, 16*time.Second, conn.calculateBackoffDelay(4))
	assert.Equal(t, 30*time.Second, conn.calculateBackoffDelay(5))  // Capped at max
	assert.Equal(t, 30*time.Second, conn.calculateBackoffDelay(10)) // Still capped
}

func TestConnectionDialTimeout(t *testing.T) {
	t.Run("DialTimeoutConfiguration", func(t *testing.T) {
		config := DefaultConnectionConfig()
		config.URL = "ws://localhost:8080" // Invalid URL to test timeout
		config.DialTimeout = 1 * time.Second
		config.Logger = zaptest.NewLogger(t)

		conn, err := NewConnection(config)
		require.NoError(t, err)
		assert.Equal(t, 1*time.Second, conn.config.DialTimeout)
	})

	t.Run("DialTimeoutEnforced", func(t *testing.T) {
		config := DefaultConnectionConfig()
		config.URL = "ws://192.0.2.1:8080"          // RFC 5737 TEST-NET-1 address that should timeout
		config.DialTimeout = 100 * time.Millisecond // Very short timeout
		config.Logger = zaptest.NewLogger(t)

		conn, err := NewConnection(config)
		require.NoError(t, err)

		// Test that connection times out quickly
		start := time.Now()
		ctx := context.Background()
		err = conn.Connect(ctx)
		elapsed := time.Since(start)

		// Should fail due to timeout
		assert.Error(t, err)
		// Should timeout roughly within the dial timeout (allowing some margin)
		assert.Less(t, elapsed, 5*time.Second) // Much less than default timeout
	})
}

// Helper function to create a test WebSocket server
func createTestWebSocketServer(t *testing.T) *httptest.Server {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("WebSocket upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Echo messages back to client
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					t.Logf("WebSocket error: %v", err)
				}
				break
			}

			if err := conn.WriteMessage(messageType, message); err != nil {
				t.Logf("WebSocket write error: %v", err)
				break
			}
		}
	}))

	return server
}
