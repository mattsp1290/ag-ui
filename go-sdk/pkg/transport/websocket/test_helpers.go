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
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	testutils "github.com/ag-ui/go-sdk/pkg/testing"
)

// ReliableTestServer provides a WebSocket server designed for reliable testing
type ReliableTestServer struct {
	server         *httptest.Server
	upgrader       websocket.Upgrader
	connections    int64
	messages       int64
	errors         int64
	logger         *zap.Logger
	echoMode       bool
	dropRate       float64
	delayMs        int64
	ctx            context.Context
	cancel         context.CancelFunc
	connsMutex     sync.RWMutex
	activeConns    map[*websocket.Conn]bool
	shutdownCh     chan struct{}
	shutdownOnce   sync.Once
}

// NewReliableTestServer creates a new reliable test server
func NewReliableTestServer(t *testing.T) *ReliableTestServer {
	ctx, cancel := context.WithCancel(context.Background())
	
	server := &ReliableTestServer{
		upgrader: websocket.Upgrader{
			CheckOrigin:     func(r *http.Request) bool { return true },
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
		},
		logger:      zaptest.NewLogger(t),
		echoMode:    true,
		ctx:         ctx,
		cancel:      cancel,
		activeConns: make(map[*websocket.Conn]bool),
		shutdownCh:  make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.handleWebSocket)
	server.server = httptest.NewServer(mux)

	return server
}

func (s *ReliableTestServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check if server is shutting down
	select {
	case <-s.shutdownCh:
		http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
		return
	default:
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		atomic.AddInt64(&s.errors, 1)
		s.logger.Error("Failed to upgrade connection", zap.Error(err))
		return
	}

	// Track connection
	s.connsMutex.Lock()
	s.activeConns[conn] = true
	s.connsMutex.Unlock()

	atomic.AddInt64(&s.connections, 1)
	defer func() {
		s.connsMutex.Lock()
		delete(s.activeConns, conn)
		s.connsMutex.Unlock()
		
		atomic.AddInt64(&s.connections, -1)
		conn.Close()
	}()

	// Set reasonable timeouts
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// Handle ping/pong
	conn.SetPingHandler(func(message string) error {
		conn.SetWriteDeadline(time.Now().Add(time.Second))
		return conn.WriteMessage(websocket.PongMessage, []byte(message))
	})

	for {
		select {
		case <-s.shutdownCh:
			// Graceful shutdown
			conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutdown"),
				time.Now().Add(time.Second))
			return
		default:
		}

		// Read message with timeout
		conn.SetReadDeadline(time.Now().Add(time.Second))
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			// Check if it's just a timeout for shutdown check
			if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
				continue
			}
			
			// Handle close errors gracefully
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				return
			}
			
			atomic.AddInt64(&s.errors, 1)
			return
		}

		atomic.AddInt64(&s.messages, 1)

		// Simulate delay if configured
		if delay := atomic.LoadInt64(&s.delayMs); delay > 0 {
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}

		// Echo back if in echo mode
		if s.echoMode {
			conn.SetWriteDeadline(time.Now().Add(time.Second))
			if err := conn.WriteMessage(messageType, message); err != nil {
				atomic.AddInt64(&s.errors, 1)
				return
			}
		}
	}
}

// URL returns the WebSocket URL
func (s *ReliableTestServer) URL() string {
	return "ws" + s.server.URL[4:] + "/ws"
}

// Close gracefully shuts down the server
func (s *ReliableTestServer) Close() {
	s.shutdownOnce.Do(func() {
		s.logger.Debug("Shutting down reliable test server")
		
		// Signal shutdown to all handlers
		close(s.shutdownCh)
		
		// Close all active connections gracefully
		s.connsMutex.RLock()
		var conns []*websocket.Conn
		for conn := range s.activeConns {
			conns = append(conns, conn)
		}
		s.connsMutex.RUnlock()
		
		// Send close messages to all connections
		for _, conn := range conns {
			conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
			conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutdown"),
				time.Now().Add(100*time.Millisecond))
		}
		
		// Give connections time to close gracefully
		time.Sleep(100 * time.Millisecond)
		
		// Force close any remaining connections
		s.connsMutex.Lock()
		for conn := range s.activeConns {
			conn.Close()
			delete(s.activeConns, conn)
		}
		s.connsMutex.Unlock()
		
		// Close HTTP server
		s.server.Close()
		s.cancel()
	})
}

// GetStats returns current server statistics
func (s *ReliableTestServer) GetStats() (connections, messages, errors int64) {
	return atomic.LoadInt64(&s.connections), 
		   atomic.LoadInt64(&s.messages), 
		   atomic.LoadInt64(&s.errors)
}

// SetEchoMode enables or disables echo mode
func (s *ReliableTestServer) SetEchoMode(enabled bool) {
	s.echoMode = enabled
}

// SetDelay sets artificial delay for message processing
func (s *ReliableTestServer) SetDelay(delay time.Duration) {
	atomic.StoreInt64(&s.delayMs, delay.Milliseconds())
}

// ReliableConnectionTester provides utilities for testing WebSocket connections reliably
type ReliableConnectionTester struct {
	t      *testing.T
	server *ReliableTestServer
	config *ConnectionConfig
}

// NewReliableConnectionTester creates a new connection tester
func NewReliableConnectionTester(t *testing.T) *ReliableConnectionTester {
	server := NewReliableTestServer(t)
	
	config := DefaultConnectionConfig()
	config.URL = server.URL()
	config.Logger = zaptest.NewLogger(t)
	config.PingPeriod = 100 * time.Millisecond
	config.PongWait = 200 * time.Millisecond
	config.WriteTimeout = 1 * time.Second
	config.MaxReconnectAttempts = 3
	config.InitialReconnectDelay = 50 * time.Millisecond
	
	tester := &ReliableConnectionTester{
		t:      t,
		server: server,
		config: config,
	}
	
	t.Cleanup(func() {
		tester.Cleanup()
	})
	
	return tester
}

// CreateConnection creates a new connection with the test configuration
func (rt *ReliableConnectionTester) CreateConnection() *Connection {
	conn, err := NewConnection(rt.config)
	require.NoError(rt.t, err)
	return conn
}

// TestConnection tests a connection with retry logic
func (rt *ReliableConnectionTester) TestConnection(testFunc func(*Connection)) {
	retryConfig := testutils.DefaultRetryConfig()
	retryConfig.MaxAttempts = 3
	retryConfig.ShouldRetry = func(err error) bool {
		// Retry on connection errors but not assertion failures
		return err != nil && !isTestFailure(err)
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	err := testutils.RetryUntilSuccess(ctx, retryConfig, func() error {
		conn := rt.CreateConnection()
		defer conn.Close()
		
		// Connect with timeout
		connectCtx, connectCancel := context.WithTimeout(ctx, 5*time.Second)
		defer connectCancel()
		
		if err := conn.Connect(connectCtx); err != nil {
			return err
		}
		
		// Wait for connection to be established
		testutils.EventuallyWithTimeout(rt.t, func() bool {
			return conn.IsConnected()
		}, 2*time.Second, 10*time.Millisecond, "Connection should be established")
		
		// Run the test
		testFunc(conn)
		return nil
	})
	
	require.NoError(rt.t, err)
}

// TestConcurrentConnections tests multiple connections concurrently
func (rt *ReliableConnectionTester) TestConcurrentConnections(numConnections int, testFunc func(int, *Connection)) {
	tester := testutils.NewConcurrentTester(rt.t, numConnections*2)
	
	for i := 0; i < numConnections; i++ {
		connIndex := i
		tester.Go(func() error {
			conn := rt.CreateConnection()
			defer conn.Close()
			
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			
			if err := conn.Connect(ctx); err != nil {
				return err
			}
			
			// Wait for connection
			if !EventuallyTrue(func() bool {
				return conn.IsConnected()
			}, 2*time.Second, 10*time.Millisecond) {
				return fmt.Errorf("connection %d failed to connect", connIndex)
			}
			
			testFunc(connIndex, conn)
			return nil
		})
	}
	
	tester.Wait()
}

// Cleanup cleans up test resources
func (rt *ReliableConnectionTester) Cleanup() {
	if rt.server != nil {
		rt.server.Close()
	}
}

// Helper function to check if an error is a test failure
func isTestFailure(err error) bool {
	// This is a simple heuristic - could be improved
	return false
}

// EventuallyTrue is a helper that checks a condition repeatedly
func EventuallyTrue(condition func() bool, timeout time.Duration, interval time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(interval)
	}
	return false
}

// WithReliableTimeout wraps a test function with a timeout and cleanup
func WithReliableTimeout(t *testing.T, timeout time.Duration, testFunc func(context.Context)) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	done := make(chan struct{})
	var testErr error
	
	go func() {
		defer func() {
			if r := recover(); r != nil {
				testErr = fmt.Errorf("test panicked: %v", r)
			}
			close(done)
		}()
		testFunc(ctx)
	}()
	
	select {
	case <-done:
		if testErr != nil {
			t.Fatal(testErr)
		}
	case <-ctx.Done():
		t.Fatalf("Test timed out after %v", timeout)
	}
}

// ReliableMessageTester helps test message sending/receiving reliably
type ReliableMessageTester struct {
	conn       *Connection
	received   chan []byte
	errors     chan error
	stopCh     chan struct{}
}

// NewReliableMessageTester creates a new message tester
func NewReliableMessageTester(conn *Connection) *ReliableMessageTester {
	tester := &ReliableMessageTester{
		conn:     conn,
		received: make(chan []byte, 100),
		errors:   make(chan error, 10),
		stopCh:   make(chan struct{}),
	}
	
	// Set up message handler
	conn.SetOnMessage(tester.handleMessage)
	
	return tester
}

func (mt *ReliableMessageTester) handleMessage(message []byte) {
	select {
	case mt.received <- message:
	case <-mt.stopCh:
		// Test is stopping, ignore message
	default:
		// Channel full, drop message to prevent blocking
	}
}

// SendAndVerify sends a message and verifies it's echoed back
func (mt *ReliableMessageTester) SendAndVerify(ctx context.Context, message []byte, timeout time.Duration) error {
	// Send message
	if err := mt.conn.SendMessage(ctx, message); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	
	// Wait for echo
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	
	select {
	case received := <-mt.received:
		if string(received) != string(message) {
			return fmt.Errorf("received message %q doesn't match sent %q", string(received), string(message))
		}
		return nil
	case err := <-mt.errors:
		return fmt.Errorf("error while waiting for message: %w", err)
	case <-timer.C:
		return fmt.Errorf("timeout waiting for message echo")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop stops the message tester
func (mt *ReliableMessageTester) Stop() {
	close(mt.stopCh)
}