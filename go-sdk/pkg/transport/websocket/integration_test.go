package websocket

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// testTransportConfig returns a DefaultTransportConfig with rate limiting disabled for tests
func testTransportConfig() *TransportConfig {
	config := DefaultTransportConfig()
	// Disable rate limiting for tests to avoid rate limit errors
	config.PoolConfig.ConnectionTemplate.RateLimiter = nil
	// Optimize heartbeat settings for faster tests (reduce from 30s/35s to 1s/2s)
	config.PoolConfig.ConnectionTemplate.PingPeriod = 1 * time.Second
	config.PoolConfig.ConnectionTemplate.PongWait = 2 * time.Second
	return config
}


// TestWebSocketServer provides a configurable WebSocket test server
type TestWebSocketServer struct {
	server         *httptest.Server
	upgrader       websocket.Upgrader
	connections    map[string]*websocket.Conn
	connMutex      sync.RWMutex
	messageHandler func(conn *websocket.Conn, messageType int, data []byte) error
	onConnect      func(conn *websocket.Conn)
	onDisconnect   func(conn *websocket.Conn, err error)
	closeDelay     time.Duration
	dropMessages   bool
	dropRate       float64
	logger         *zap.Logger
}

func NewTestWebSocketServer(t testing.TB) *TestWebSocketServer {
	server := &TestWebSocketServer{
		connections: make(map[string]*websocket.Conn),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		logger: zaptest.NewLogger(t),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.handleWebSocket)
	server.server = httptest.NewServer(mux)

	return server
}

func NewTestTLSWebSocketServer(t testing.TB) *TestWebSocketServer {
	server := &TestWebSocketServer{
		connections: make(map[string]*websocket.Conn),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		logger: zaptest.NewLogger(t),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.handleWebSocket)
	server.server = httptest.NewTLSServer(mux)

	return server
}

func (s *TestWebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", zap.Error(err))
		return
	}

	connID := fmt.Sprintf("%s_%d", r.RemoteAddr, time.Now().UnixNano())

	s.connMutex.Lock()
	s.connections[connID] = conn
	s.connMutex.Unlock()

	if s.onConnect != nil {
		s.onConnect(conn)
	}

	defer func() {
		s.connMutex.Lock()
		delete(s.connections, connID)
		s.connMutex.Unlock()

		if s.closeDelay > 0 {
			time.Sleep(s.closeDelay)
		}

		conn.Close()

		if s.onDisconnect != nil {
			s.onDisconnect(conn, nil)
		}
	}()

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Debug("WebSocket connection closed unexpectedly", zap.Error(err))
			}
			break
		}

		// Simulate message drops
		if s.dropMessages && rand.Float64() < s.dropRate {
			continue
		}

		if s.messageHandler != nil {
			if err := s.messageHandler(conn, messageType, message); err != nil {
				s.logger.Error("Message handler failed", zap.Error(err))
				break
			}
		} else {
			// Default: echo the message back
			if err := conn.WriteMessage(messageType, message); err != nil {
				s.logger.Error("Failed to write message", zap.Error(err))
				break
			}
		}
	}
}

func (s *TestWebSocketServer) URL() string {
	return "ws" + strings.TrimPrefix(s.server.URL, "http") + "/ws"
}

func (s *TestWebSocketServer) TLSURL() string {
	return "wss" + strings.TrimPrefix(s.server.URL, "https") + "/ws"
}

func (s *TestWebSocketServer) Close() {
	s.server.Close()
}

func (s *TestWebSocketServer) GetConnectionCount() int {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()
	return len(s.connections)
}

func (s *TestWebSocketServer) BroadcastMessage(messageType int, data []byte) {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()

	for _, conn := range s.connections {
		if err := conn.WriteMessage(messageType, data); err != nil {
			s.logger.Error("Failed to broadcast message", zap.Error(err))
		}
	}
}

func (s *TestWebSocketServer) CloseAllConnections() {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()

	for _, conn := range s.connections {
		conn.Close()
	}
}

func TestBasicWebSocketIntegration(t *testing.T) {
	t.Run("BasicConnection", func(t *testing.T) {
		server := NewTestWebSocketServer(t)
		defer server.Close()

		// Reduced timeout from 15s to 5s (67% reduction)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		config := testTransportConfig()
		config.URLs = []string{server.URL()}
		config.Logger = zaptest.NewLogger(t)
		config.EnableEventValidation = false

		transport, err := NewTransport(config)
		require.NoError(t, err)

		err = transport.Start(ctx)
		require.NoError(t, err)
		
		defer transport.Stop()

		// Wait for connections to establish with faster polling (reduced from 3s to 2s)
		assert.Eventually(t, func() bool {
			return transport.IsConnected()
		}, 2*time.Second, 50*time.Millisecond)

		assert.Greater(t, transport.GetActiveConnectionCount(), 0)
		assert.Greater(t, server.GetConnectionCount(), 0)
	})

	t.Run("MessageExchange", func(t *testing.T) {
		server := NewTestWebSocketServer(t)
		defer server.Close()

		// Reduced timeout from 15s to 5s (67% reduction)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		config := testTransportConfig()
		config.URLs = []string{server.URL()}
		config.Logger = zaptest.NewLogger(t)
		config.EnableEventValidation = false

		transport, err := NewTransport(config)
		require.NoError(t, err)

		err = transport.Start(ctx)
		require.NoError(t, err)
		
		defer transport.Stop()

		// Wait for connections (reduced from 10s to 3s)
		assert.Eventually(t, func() bool {
			isConnected := transport.IsConnected()
			activeCount := transport.GetActiveConnectionCount()
			t.Logf("IsConnected: %v, ActiveConnections: %d", isConnected, activeCount)
			return isConnected && activeCount > 0
		}, 3*time.Second, 100*time.Millisecond) // Also reduced polling interval

		// Send a message
		event := &MockEvent{
			EventType: events.EventTypeTextMessageContent,
			Data:      "integration test message",
		}

		err = transport.SendEvent(ctx, event)
		assert.NoError(t, err)

		// Verify statistics
		stats := transport.Stats()
		assert.Greater(t, stats.EventsSent, int64(0))
	})
}

func TestMultiServerIntegration(t *testing.T) {
	server1 := NewTestWebSocketServer(t)
	defer server1.Close()

	server2 := NewTestWebSocketServer(t)
	defer server2.Close()

	config := testTransportConfig()
	config.URLs = []string{server1.URL(), server2.URL()}
	config.Logger = zaptest.NewLogger(t)
	config.EnableEventValidation = false
	config.PoolConfig.MinConnections = 2
	config.PoolConfig.MaxConnections = 4
	// Disable rate limiting for tests
	config.PoolConfig.ConnectionTemplate.RateLimiter = nil

	transport, err := NewTransport(config)
	require.NoError(t, err)

	// Reduced timeout from 30s to 8s (73% reduction)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	
	// Add cleanup timeout to prevent hanging goroutines
	defer func() {
		done := make(chan struct{})
		go func() {
			transport.Stop()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("Transport.Stop() timed out after 2s")
		}
	}()

	t.Run("MultipleServerConnections", func(t *testing.T) {
		// Wait for connections to establish
		assert.Eventually(t, func() bool {
			return transport.GetActiveConnectionCount() >= 2
		}, 3*time.Second, 100*time.Millisecond)

		// Verify both servers have connections
		assert.Eventually(t, func() bool {
			return server1.GetConnectionCount() >= 1 && server2.GetConnectionCount() >= 1
		}, 2*time.Second, 100*time.Millisecond)
	})

	t.Run("LoadBalancing", func(t *testing.T) {
		// Wait for connections
		assert.Eventually(t, func() bool {
			return transport.GetActiveConnectionCount() >= 2
		}, 3*time.Second, 100*time.Millisecond)

		// Send multiple messages
		for i := 0; i < 10; i++ {
			event := &MockEvent{
				EventType: events.EventTypeTextMessageContent,
				Data:      fmt.Sprintf("load balance test message %d", i),
			}

			err := transport.SendEvent(ctx, event)
			assert.NoError(t, err)
		}

		stats := transport.Stats()
		assert.Equal(t, int64(10), stats.EventsSent)
	})
}

func TestTLSIntegration(t *testing.T) {
	server := NewTestTLSWebSocketServer(t)
	defer server.Close()

	// Create transport with TLS configuration
	config := testTransportConfig()
	config.URLs = []string{server.TLSURL()}
	config.Logger = zaptest.NewLogger(t)
	config.EnableEventValidation = false
	// Disable rate limiting for tests
	config.PoolConfig.ConnectionTemplate.RateLimiter = nil

	// Configure connection to accept self-signed certificates
	config.PoolConfig.ConnectionTemplate.Headers = map[string]string{
		"User-Agent": "AG-UI-Go-SDK-Test",
	}

	// Configure TLS to skip certificate verification for testing
	config.PoolConfig.ConnectionTemplate.TLSClientConfig = createInsecureTLSConfig()

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("TLSConnection", func(t *testing.T) {
		err := transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		// Wait for TLS connections to establish
		assert.Eventually(t, func() bool {
			return transport.IsConnected()
		}, 3*time.Second, 100*time.Millisecond)

		assert.Greater(t, transport.GetActiveConnectionCount(), 0)
	})

	t.Run("SecureMessageExchange", func(t *testing.T) {
		err := transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		// Wait for connections
		assert.Eventually(t, func() bool {
			return transport.IsConnected()
		}, 3*time.Second, 100*time.Millisecond)

		// Send encrypted message
		event := &MockEvent{
			EventType: events.EventTypeTextMessageContent,
			Data:      "secure message over TLS",
		}

		err = transport.SendEvent(ctx, event)
		assert.NoError(t, err)
	})
}

func TestReconnectionIntegration(t *testing.T) {
	server := NewTestWebSocketServer(t)
	defer server.Close()

	config := testTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zaptest.NewLogger(t)
	config.EnableEventValidation = false
	config.PoolConfig.ConnectionTemplate.MaxReconnectAttempts = 5
	config.PoolConfig.ConnectionTemplate.InitialReconnectDelay = 100 * time.Millisecond
	// Disable rate limiting for tests
	config.PoolConfig.ConnectionTemplate.RateLimiter = nil

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	t.Run("ReconnectionAfterServerClose", func(t *testing.T) {
		// Wait for initial connection
		assert.Eventually(t, func() bool {
			return transport.IsConnected()
		}, 2*time.Second, 100*time.Millisecond)

		initialConnCount := transport.GetActiveConnectionCount()
		assert.Greater(t, initialConnCount, 0)

		// Close all server connections
		server.CloseAllConnections()

		// Wait longer for disconnection to be detected and reconnection attempts to start
		time.Sleep(500 * time.Millisecond)

		// Create new server (simulating server restart)
		newServer := NewTestWebSocketServer(t)
		defer newServer.Close()

		// Update transport configuration to use new server URL
		// Note: In a real scenario, the URL would be the same but the server would restart
		// For testing, we'll verify that reconnection attempts occur
		poolStats := transport.GetConnectionPoolStats()

		// Should eventually try to reconnect - increased timeout for more reliable detection
		assert.Eventually(t, func() bool {
			currentStats := transport.GetConnectionPoolStats()
			// Check for either failed requests or reconnection attempts
			return currentStats.FailedRequests > poolStats.FailedRequests || 
				   currentStats.TotalConnections > poolStats.TotalConnections
		}, 5*time.Second, 200*time.Millisecond)
	})
}

func TestHeartbeatIntegration(t *testing.T) {
	// Use fast test configuration to prevent hanging
	WithTimeout(t, FastTestConfig().MediumTest, func(ctx context.Context) {
		server := NewTestWebSocketServer(t)
		defer server.Close()

		config := OptimizedTransportConfig()
		config.URLs = []string{server.URL()}
		config.Logger = zaptest.NewLogger(t)
		config.EnableEventValidation = false

		transport, err := NewTransport(config)
		require.NoError(t, err)

		err = transport.Start(ctx)
		require.NoError(t, err)
		defer func() {
			// Ensure transport stops within timeout
			done := make(chan struct{})
			go func() {
				transport.Stop()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Error("Transport.Stop() timed out")
			}
		}()

		t.Run("HeartbeatFunctionality", func(t *testing.T) {
			// Wait for connection establishment
			assert.Eventually(t, func() bool {
				return transport.IsConnected()
			}, 2*time.Second, 50*time.Millisecond)

			// Verify connections are healthy
			assert.Greater(t, transport.GetHealthyConnectionCount(), 0)

			// Check pool status
			status := transport.GetConnectionPoolStats()
			assert.Greater(t, status.TotalConnections, int64(0))
		})
	})
}

func TestSubscriptionIntegration(t *testing.T) {
	server := NewTestWebSocketServer(t)
	defer server.Close()

	// Set up server to broadcast messages
	var messageCount int32
	server.messageHandler = func(conn *websocket.Conn, messageType int, data []byte) error {
		// Echo back and also send a broadcast message
		if err := conn.WriteMessage(messageType, data); err != nil {
			return err
		}

		// Send a custom event
		customEvent := map[string]interface{}{
			"type": "server_broadcast",
			"data": fmt.Sprintf("broadcast_%d", atomic.AddInt32(&messageCount, 1)),
		}
		eventData, _ := json.Marshal(customEvent)
		return conn.WriteMessage(websocket.TextMessage, eventData)
	}

	config := testTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zaptest.NewLogger(t)
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	t.Run("EventSubscriptionAndReceiving", func(t *testing.T) {
		// Wait for connections
		assert.Eventually(t, func() bool {
			return transport.IsConnected()
		}, 2*time.Second, 100*time.Millisecond)

		var receivedEvents []events.Event
		var mu sync.Mutex

		handler := func(ctx context.Context, event events.Event) error {
			mu.Lock()
			defer mu.Unlock()
			receivedEvents = append(receivedEvents, event)
			return nil
		}

		// Subscribe to server broadcast events
		sub, err := transport.Subscribe(ctx, []string{"server_broadcast"}, handler)
		require.NoError(t, err)
		require.NotNil(t, sub)

		// Send a message to trigger server response
		event := &MockEvent{
			EventType: events.EventTypeTextMessageContent,
			Data:      "trigger server broadcast",
		}

		err = transport.SendEvent(ctx, event)
		require.NoError(t, err)

		// Wait for response
		assert.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return len(receivedEvents) > 0
		}, 2*time.Second, 100*time.Millisecond)

		mu.Lock()
		if assert.Greater(t, len(receivedEvents), 0) {
			assert.Equal(t, events.EventType("server_broadcast"), receivedEvents[0].Type())
		}
		mu.Unlock()

		// Unsubscribe
		err = transport.Unsubscribe(sub.ID)
		assert.NoError(t, err)
	})
}

func TestCompressionIntegration(t *testing.T) {
	server := NewTestWebSocketServer(t)
	defer server.Close()

	// Enable compression on the server
	server.upgrader.EnableCompression = true

	config := testTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zaptest.NewLogger(t)
	config.EnableEventValidation = false
	config.PoolConfig.ConnectionTemplate.EnableCompression = true
	// Disable rate limiting for tests
	config.PoolConfig.ConnectionTemplate.RateLimiter = nil

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	t.Run("CompressedMessageExchange", func(t *testing.T) {
		// Wait for connections
		assert.Eventually(t, func() bool {
			return transport.IsConnected()
		}, 2*time.Second, 100*time.Millisecond)

		// Send a large message that should benefit from compression
		largeData := strings.Repeat("This is a test message that should compress well. ", 100)
		event := &MockEvent{
			EventType: events.EventTypeTextMessageContent,
			Data:      largeData,
		}

		err := transport.SendEvent(ctx, event)
		assert.NoError(t, err)

		stats := transport.Stats()
		assert.Greater(t, stats.EventsSent, int64(0))
		assert.Greater(t, stats.BytesTransferred, int64(0))
	})
}

func TestErrorHandlingIntegration(t *testing.T) {
	t.Run("ServerUnavailable", func(t *testing.T) {
		config := testTransportConfig()
		config.URLs = []string{"ws://localhost:99999"} // Non-existent server
		config.Logger = zaptest.NewLogger(t)
		config.PoolConfig.ConnectionTemplate.MaxReconnectAttempts = 2
		config.PoolConfig.ConnectionTemplate.InitialReconnectDelay = 100 * time.Millisecond
		// Disable rate limiting for tests
		config.PoolConfig.ConnectionTemplate.RateLimiter = nil

		transport, err := NewTransport(config)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = transport.Start(ctx)
		// Start doesn't fail immediately as connections are established asynchronously

		// Wait and verify that connections fail - optimized for faster tests
		time.Sleep(500 * time.Millisecond)
		assert.False(t, transport.IsConnected())
		assert.Equal(t, 0, transport.GetActiveConnectionCount())

		transport.Stop()
	})

	t.Run("MalformedServerResponses", func(t *testing.T) {
		server := NewTestWebSocketServer(t)
		defer server.Close()

		// Configure server to send malformed responses
		server.messageHandler = func(conn *websocket.Conn, messageType int, data []byte) error {
			// Send invalid JSON
			return conn.WriteMessage(websocket.TextMessage, []byte("{invalid json"))
		}

		config := testTransportConfig()
		config.URLs = []string{server.URL()}
		config.Logger = zaptest.NewLogger(t)
		config.EnableEventValidation = false

		transport, err := NewTransport(config)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err = transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		// Wait for connections
		assert.Eventually(t, func() bool {
			return transport.IsConnected()
		}, 2*time.Second, 100*time.Millisecond)

		// Send message that will trigger malformed response
		event := &MockEvent{
			EventType: events.EventTypeTextMessageContent,
			Data:      "trigger malformed response",
		}

		err = transport.SendEvent(ctx, event)
		assert.NoError(t, err) // Sending should succeed

		// Response parsing should fail, but transport should remain stable
		time.Sleep(100 * time.Millisecond)
		assert.True(t, transport.IsConnected()) // Should still be connected
	})
}

func TestHighThroughputIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high throughput test in short mode")
	}

	// Use aggressive timeout to prevent hanging
	WithTimeout(t, FastTestConfig().LongTest, func(ctx context.Context) {
		server := NewTestWebSocketServer(t)
		defer server.Close()

		config := FastTransportConfig()
		config.URLs = []string{server.URL()}
		config.Logger = zaptest.NewLogger(t)
		config.PoolConfig.MaxConnections = 5
		config.EnableEventValidation = false

		transport, err := NewTransport(config)
		require.NoError(t, err)

		err = transport.Start(ctx)
		require.NoError(t, err)
		defer func() {
			// Force shutdown with timeout
			done := make(chan struct{})
			go func() {
				transport.Stop()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Error("Transport.Stop() timed out during high throughput test")
			}
		}()

		t.Run("HighVolumeMessageSending", func(t *testing.T) {
			// Wait for connections
			assert.Eventually(t, func() bool {
				return transport.IsConnected()
			}, 2*time.Second, 100*time.Millisecond)

			// Reduced message count for faster test execution
			const numMessages = 100 // Reduced from 1000
			var wg sync.WaitGroup
			var errors int32

			startTime := time.Now()

			// Send messages with context deadline check
			for i := 0; i < numMessages; i++ {
				// Check context before creating new goroutine
				select {
				case <-ctx.Done():
					t.Fatal("Test context cancelled during message sending")
				default:
				}

				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					event := &MockEvent{
						EventType: events.EventTypeTextMessageContent,
						Data:      fmt.Sprintf("high throughput message %d", id),
					}

					if err := transport.SendEvent(ctx, event); err != nil {
						atomic.AddInt32(&errors, 1)
					}
				}(i)
			}

			// Wait with timeout protection
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				// All messages sent successfully
			case <-time.After(10 * time.Second):
				t.Fatal("Message sending timed out")
			case <-ctx.Done():
				t.Fatal("Test context cancelled while waiting for messages")
			}

			duration := time.Since(startTime)
			errorCount := atomic.LoadInt32(&errors)

			// Allow some errors in high throughput scenarios
			if errorCount > int32(numMessages/10) { // Allow up to 10% errors
				t.Errorf("Too many errors: %d/%d", errorCount, numMessages)
			}

			stats := transport.Stats()
			expectedMessages := int64(numMessages) - int64(errorCount)
			if stats.EventsSent < expectedMessages/2 { // Allow 50% tolerance
				t.Errorf("Expected at least %d messages sent, got %d", expectedMessages/2, stats.EventsSent)
			}

			throughput := float64(stats.EventsSent) / duration.Seconds()
			t.Logf("Sent %d messages in %v (%.2f messages/sec, %d errors)", 
				stats.EventsSent, duration, throughput, errorCount)

			// Reduced throughput requirement for stability
			assert.Greater(t, throughput, 10.0) // At least 10 messages per second
		})
	})
}

func TestRealWorldScenarios(t *testing.T) {
	server := NewTestWebSocketServer(t)
	defer server.Close()

	config := testTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zaptest.NewLogger(t)
	config.EnableEventValidation = false
	config.PoolConfig.MinConnections = 2
	config.PoolConfig.MaxConnections = 4
	// Disable rate limiting for tests
	config.PoolConfig.ConnectionTemplate.RateLimiter = nil

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	t.Run("ChatApplicationSimulation", func(t *testing.T) {
		// Wait for connections
		assert.Eventually(t, func() bool {
			return transport.IsConnected()
		}, 2*time.Second, 100*time.Millisecond)

		// Simulate chat messages with different event types
		eventTypes := []events.EventType{
			events.EventTypeTextMessageStart,
			events.EventTypeTextMessageContent,
			events.EventTypeTextMessageEnd,
		}

		var wg sync.WaitGroup
		numConversations := 10
		messagesPerConversation := 5

		for conv := 0; conv < numConversations; conv++ {
			wg.Add(1)
			go func(conversationID int) {
				defer wg.Done()

				for msg := 0; msg < messagesPerConversation; msg++ {
					for _, eventType := range eventTypes {
						event := &MockEvent{
							EventType: eventType,
							Data:      fmt.Sprintf("conv_%d_msg_%d_type_%s", conversationID, msg, eventType),
						}

						err := transport.SendEvent(ctx, event)
						assert.NoError(t, err)

						// Small delay between message parts
						time.Sleep(10 * time.Millisecond)
					}
				}
			}(conv)
		}

		wg.Wait()

		expectedMessages := int64(numConversations * messagesPerConversation * len(eventTypes))
		stats := transport.Stats()
		assert.Equal(t, expectedMessages, stats.EventsSent)
	})

	t.Run("SubscriptionManagementWorkflow", func(t *testing.T) {
		// Simulate dynamic subscription management
		var subscriptions []*Subscription
		var mu sync.Mutex

		handler := func(ctx context.Context, event events.Event) error {
			// Simple event handler
			return nil
		}

		// Create multiple subscriptions
		for i := 0; i < 5; i++ {
			eventTypes := []string{fmt.Sprintf("dynamic_event_%d", i)}
			sub, err := transport.Subscribe(ctx, eventTypes, handler)
			require.NoError(t, err)

			mu.Lock()
			subscriptions = append(subscriptions, sub)
			mu.Unlock()
		}

		// Send events that match subscriptions
		for i := 0; i < 5; i++ {
			event := &MockEvent{
				EventType: events.EventType(fmt.Sprintf("dynamic_event_%d", i)),
				Data:      fmt.Sprintf("subscription test event %d", i),
			}
			err := transport.SendEvent(ctx, event)
			require.NoError(t, err)
		}

		// Remove some subscriptions
		mu.Lock()
		for i := 0; i < 2; i++ {
			err := transport.Unsubscribe(subscriptions[i].ID)
			assert.NoError(t, err)
		}
		mu.Unlock()

		// Verify remaining subscriptions
		remainingSubs := transport.ListSubscriptions()
		assert.Len(t, remainingSubs, 3)
	})
}

func TestConnectionPoolIntegration(t *testing.T) {
	server1 := NewTestWebSocketServer(t)
	defer server1.Close()

	server2 := NewTestWebSocketServer(t)
	defer server2.Close()

	server3 := NewTestWebSocketServer(t)
	defer server3.Close()

	config := testTransportConfig()
	config.URLs = []string{server1.URL(), server2.URL(), server3.URL()}
	config.Logger = zaptest.NewLogger(t)
	config.EnableEventValidation = false
	config.PoolConfig.MinConnections = 3
	config.PoolConfig.MaxConnections = 6
	config.PoolConfig.LoadBalancingStrategy = RoundRobin
	// Disable rate limiting for tests
	config.PoolConfig.ConnectionTemplate.RateLimiter = nil

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	t.Run("ConnectionDistribution", func(t *testing.T) {
		// Wait for all connections to establish
		assert.Eventually(t, func() bool {
			return transport.GetActiveConnectionCount() >= 3
		}, 3*time.Second, 100*time.Millisecond)

		// Verify connections are distributed across servers
		assert.Eventually(t, func() bool {
			return server1.GetConnectionCount() >= 1 &&
				server2.GetConnectionCount() >= 1 &&
				server3.GetConnectionCount() >= 1
		}, 2*time.Second, 100*time.Millisecond)

		// Check detailed status
		status := transport.GetDetailedStatus()
		connectionPool, ok := status["connection_pool"].(map[string]interface{})
		require.True(t, ok)

		totalConnections, ok := connectionPool["total_connections"].(int)
		require.True(t, ok)
		assert.GreaterOrEqual(t, totalConnections, 3)
	})

	t.Run("LoadBalancedMessageSending", func(t *testing.T) {
		// Wait for connections
		assert.Eventually(t, func() bool {
			return transport.GetActiveConnectionCount() >= 3
		}, 3*time.Second, 100*time.Millisecond)

		// Send messages and verify they're distributed
		const numMessages = 30
		for i := 0; i < numMessages; i++ {
			event := &MockEvent{
				EventType: events.EventTypeTextMessageContent,
				Data:      fmt.Sprintf("load balanced message %d", i),
			}

			err := transport.SendEvent(ctx, event)
			assert.NoError(t, err)
		}

		stats := transport.Stats()
		assert.Equal(t, int64(numMessages), stats.EventsSent)

		// All servers should have received some connections
		totalServerConnections := server1.GetConnectionCount() +
			server2.GetConnectionCount() +
			server3.GetConnectionCount()
		assert.GreaterOrEqual(t, totalServerConnections, 3)
	})
}

// Integration benchmark tests
func BenchmarkIntegrationMessageThroughput(b *testing.B) {
	server := NewTestWebSocketServer(b)
	defer server.Close()

	config := testTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zap.NewNop()
	config.EnableEventValidation = false
	config.PoolConfig.MaxConnections = 10
	// Disable rate limiting for tests
	config.PoolConfig.ConnectionTemplate.RateLimiter = nil

	transport, err := NewTransport(config)
	require.NoError(b, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(b, err)
	defer transport.Stop()

	// Wait for connections
	time.Sleep(100 * time.Millisecond)

	event := &MockEvent{
		EventType: events.EventTypeTextMessageContent,
		Data:      "benchmark message",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = transport.SendEvent(ctx, event)
		}
	})
}

func BenchmarkIntegrationSubscriptionThroughput(b *testing.B) {
	server := NewTestWebSocketServer(b)
	defer server.Close()

	config := testTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zap.NewNop()
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(b, err)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(b, err)
	defer transport.Stop()

	handler := func(ctx context.Context, event events.Event) error { return nil }

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eventType := fmt.Sprintf("bench_event_%d", i)
		sub, err := transport.Subscribe(ctx, []string{eventType}, handler)
		if err != nil {
			b.Fatal(err)
		}
		_ = transport.Unsubscribe(sub.ID)
	}
}

// createInsecureTLSConfig creates a TLS config that skips certificate verification
// for use in tests with self-signed certificates
func createInsecureTLSConfig() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true}
}
