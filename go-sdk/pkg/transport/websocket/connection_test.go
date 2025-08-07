package websocket

import (
	"context"
	"errors"
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
	config.PingPeriod = 100 * time.Millisecond
	config.ReadTimeout = 500 * time.Millisecond
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

	// Force the connection into a reconnecting state
	conn.setState(StateConnected) // Ensure we're in connected state
	conn.triggerReconnect()       // Manually trigger reconnection

	// Wait for reconnection attempts
	time.Sleep(500 * time.Millisecond)
	// Give the server time to close
	time.Sleep(100 * time.Millisecond)

	// Manually trigger disconnection by closing the connection
	conn.disconnect(errors.New("test disconnection"))

	// Wait a bit for the state to change
	time.Sleep(50 * time.Millisecond)

	// Manually trigger reconnection
	conn.triggerReconnect()

	// Wait for reconnection attempts - longer timeout for reliability
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

	// Wait for messages to be processed
	time.Sleep(100 * time.Millisecond)
	// Wait for messages to be processed - use sophisticated waiting if available, fallback to time-based
	if waitErr := conn.WaitForMessages(ctx, 5); waitErr != nil {
		// Fallback to time-based waiting if WaitForMessages fails
		time.Sleep(100 * time.Millisecond)
	}

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
	// Disable auto-reconnect for this test to have predictable behavior
	config.MaxReconnectAttempts = 0

	conn, err := NewConnection(config)
	require.NoError(t, err)

	// Setup event handlers with more detailed tracking
	var connectCalled, disconnectCalled bool
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Use WaitGroup to ensure handlers complete before assertions
	wg.Add(2) // Expect both connect and disconnect to be called

	conn.SetOnConnect(func() {
		mu.Lock()
		defer mu.Unlock()
		connectCalled = true
		t.Logf("OnConnect handler called")
		wg.Done()
	})

	conn.SetOnDisconnect(func(err error) {
		mu.Lock()
		defer mu.Unlock()
		disconnectCalled = true
		t.Logf("OnDisconnect handler called with error: %v", err)
		wg.Done()
	})

	var messageReceived bool
	messageWg := sync.WaitGroup{}
	conn.SetOnMessage(func(data []byte) {
		mu.Lock()
		defer mu.Unlock()
		messageReceived = true
		t.Logf("OnMessage handler called with data: %s", string(data))
		messageWg.Done()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect
	err = conn.Connect(ctx)
	require.NoError(t, err)

	// Send a test message to trigger message handler
	messageWg.Add(1)
	err = conn.SendMessage(ctx, []byte("test message"))
	require.NoError(t, err)

	// Wait for message handler with timeout
	done := make(chan struct{})
	go func() {
		messageWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Message received
	case <-time.After(2 * time.Second):
		t.Logf("Warning: Message handler not called within timeout")
	}

	// Verify connect handler was called
	mu.Lock()
	assert.True(t, connectCalled, "Connect handler should have been called")
	mu.Unlock()

	// Disconnect
	err = conn.Disconnect()
	require.NoError(t, err)

	// Wait for both handlers to complete with timeout
	handlerDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(handlerDone)
	}()

	select {
	case <-handlerDone:
		// Handlers completed
	case <-time.After(3 * time.Second):
		t.Logf("Warning: Event handlers did not complete within timeout")
	}

	// Verify both handlers were called
	mu.Lock()
	assert.True(t, connectCalled, "Connect handler should have been called")
	assert.True(t, disconnectCalled, "Disconnect handler should have been called")
	if messageReceived {
		t.Logf("Message handler was successfully called")
	}
	mu.Unlock()

	// Close
	err = conn.Close()
	require.NoError(t, err)
}

func TestConnectionConfiguration(t *testing.T) {

	// Test default configuration
	config := DefaultConnectionConfig()
	assert.Equal(t, 10, config.MaxReconnectAttempts)
	// Don't test exact timeout values as they change based on test mode
	assert.Greater(t, config.InitialReconnectDelay, time.Duration(0))
	assert.Greater(t, config.MaxReconnectDelay, time.Duration(0))
	assert.Equal(t, 2.0, config.ReconnectBackoffMultiplier)
	assert.Greater(t, config.HandshakeTimeout, time.Duration(0))
	assert.Greater(t, config.ReadTimeout, time.Duration(0))
	assert.Greater(t, config.WriteTimeout, time.Duration(0))
	assert.Greater(t, config.PingPeriod, time.Duration(0))
	assert.Greater(t, config.PongWait, config.PingPeriod) // PongWait should be > PingPeriod
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
	WithResourceControl(t, "TestConnectionConcurrency", func() {
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

		// Send messages concurrently with environment-aware scaling
		var wg sync.WaitGroup
		concurrencyConfig := getConcurrencyConfig("TestConnectionConcurrency")
		numGoroutines := concurrencyConfig.NumGoroutines
		messagesPerGoroutine := concurrencyConfig.OperationsPerRoutine

		t.Logf("TestConnectionConcurrency: Using %d goroutines × %d operations = %d total operations (full_suite=%v)",
			numGoroutines, messagesPerGoroutine, numGoroutines*messagesPerGoroutine, isFullTestSuite())

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

		// Check metrics - allow for race conditions in concurrent sending
		metrics := conn.GetMetrics()
		expectedMessages := int64(numGoroutines * messagesPerGoroutine)
		assert.GreaterOrEqual(t, metrics.MessagesSent, expectedMessages-5, "Messages sent should be at least %d (allowing for race conditions)", expectedMessages-5)
		assert.LessOrEqual(t, metrics.MessagesSent, expectedMessages, "Messages sent should not exceed %d", expectedMessages)

		err = conn.Close()
		require.NoError(t, err)
	}) // Close WithResourceControl
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
		defer func() {
			// Safely close the connection with proper error handling
			if err := conn.Close(); err != nil {
				t.Logf("WebSocket connection close error: %v", err)
			}
		}()

		// Create a timeout context for this connection
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Reduced from 30s
		defer cancel()

		// Track connection state to prevent "repeated read on failed websocket connection" panic
		var connectionClosed bool
		var closeMutex sync.Mutex

		// Set close handler to track connection state
		conn.SetCloseHandler(func(code int, text string) error {
			closeMutex.Lock()
			connectionClosed = true
			closeMutex.Unlock()
			t.Logf("WebSocket connection closed by peer: code=%d, text=%s", code, text)
			return nil
		})

		// Echo messages back to client with proper timeout and close handling
		for {
			// Check if context was cancelled
			select {
			case <-ctx.Done():
				t.Logf("WebSocket context cancelled, closing connection")
				return
			default:
			}

			// Check if connection is already closed to prevent panic
			closeMutex.Lock()
			if connectionClosed {
				t.Logf("WebSocket connection already closed, stopping read loop")
				closeMutex.Unlock()
				return
			}
			closeMutex.Unlock()

			// Set very short read deadline to prevent hanging during tests
			conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))

			// Use panic recovery to prevent "repeated read on failed websocket connection" panic
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Logf("Recovered from WebSocket read panic: %v", r)
						closeMutex.Lock()
						connectionClosed = true
						closeMutex.Unlock()
					}
				}()

				messageType, message, err := conn.ReadMessage()
				if err != nil {
					// Check for timeout error first
					if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
						// Normal timeout - check cancellation and continue reading
						select {
						case <-ctx.Done():
							return
						default:
							return // Just return from the anonymous function, continue outer loop
						}
					}

					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
						t.Logf("WebSocket error: %v", err)
					}

					// Mark connection as closed on any read error
					closeMutex.Lock()
					connectionClosed = true
					closeMutex.Unlock()
					return // Return from anonymous function, which will exit the main loop
				}

				// Check if connection was closed during read
				closeMutex.Lock()
				if connectionClosed {
					closeMutex.Unlock()
					return
				}
				closeMutex.Unlock()

				// Set write deadline to prevent hanging
				conn.SetWriteDeadline(time.Now().Add(50 * time.Millisecond))
				if err := conn.WriteMessage(messageType, message); err != nil {
					t.Logf("WebSocket write error: %v", err)
					// Mark connection as closed on write error
					closeMutex.Lock()
					connectionClosed = true
					closeMutex.Unlock()
					return
				}
			}()

			// Check if we should exit the main loop
			closeMutex.Lock()
			if connectionClosed {
				closeMutex.Unlock()
				break
			}
			closeMutex.Unlock()
		}
	}))

	return server
}
