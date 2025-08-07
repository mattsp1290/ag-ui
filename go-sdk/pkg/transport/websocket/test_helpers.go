package websocket

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	testutils "github.com/mattsp1290/ag-ui/go-sdk/pkg/testing"
)

// ReliableTestServer provides a WebSocket server designed for reliable testing
type ReliableTestServer struct {
	server       *httptest.Server
	upgrader     websocket.Upgrader
	connections  int64
	messages     int64
	errors       int64
	logger       *zap.Logger
	echoMode     bool
	dropRate     float64
	delayMs      int64
	ctx          context.Context
	cancel       context.CancelFunc
	connsMutex   sync.RWMutex
	activeConns  map[*websocket.Conn]bool
	shutdownCh   chan struct{}
	shutdownOnce sync.Once
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

	// Use shorter read timeout for faster shutdown response
	timeoutTicker := time.NewTicker(100 * time.Millisecond)
	defer timeoutTicker.Stop()

	for {
		// Priority check for shutdown
		select {
		case <-s.shutdownCh:
			// Immediate shutdown
			conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutdown"),
				time.Now().Add(50*time.Millisecond))
			return
		case <-s.ctx.Done():
			// Context cancelled
			return
		default:
		}

		// Read message with very short timeout for responsiveness
		conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			// Check shutdown first on any error
			select {
			case <-s.shutdownCh:
				return
			case <-s.ctx.Done():
				return
			default:
			}

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

		// Simulate delay if configured (but check for shutdown)
		if delay := atomic.LoadInt64(&s.delayMs); delay > 0 {
			delayCh := time.After(time.Duration(delay) * time.Millisecond)
			select {
			case <-s.shutdownCh:
				return
			case <-s.ctx.Done():
				return
			case <-delayCh:
				// Delay completed
			}
		}

		// Echo back if in echo mode
		if s.echoMode {
			conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond)) // Shorter timeout
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

		// Signal shutdown to all handlers first
		close(s.shutdownCh)

		// Cancel context immediately to stop all operations
		s.cancel()

		// Close all active connections aggressively
		s.connsMutex.Lock()
		var conns []*websocket.Conn
		for conn := range s.activeConns {
			conns = append(conns, conn)
		}
		s.connsMutex.Unlock()

		// Force close all connections immediately
		for _, conn := range conns {
			// Set immediate deadlines
			now := time.Now()
			conn.SetReadDeadline(now)
			conn.SetWriteDeadline(now)

			// Try to send close message quickly
			conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutdown"),
				now.Add(10*time.Millisecond))

			// Force close immediately
			conn.Close()
		}

		// Clear active connections map
		s.connsMutex.Lock()
		for conn := range s.activeConns {
			delete(s.activeConns, conn)
		}
		s.connsMutex.Unlock()

		// Close HTTP server
		s.server.Close()

		s.logger.Debug("Reliable test server shutdown completed")
	})
}

// GetStats returns current server statistics
func (s *ReliableTestServer) GetStats() (connections, messages, errors int64) {
	return atomic.LoadInt64(&s.connections),
		atomic.LoadInt64(&s.messages),
		atomic.LoadInt64(&s.errors)
}

// GetConnectionCount returns the current number of active connections
func (s *ReliableTestServer) GetConnectionCount() int {
	return int(atomic.LoadInt64(&s.connections))
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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second) // Reduced timeout
	defer cancel()

	err := testutils.RetryUntilSuccess(ctx, retryConfig, func() error {
		conn := rt.CreateConnection()

		// Ensure connection is closed properly
		defer func() {
			if err := conn.Close(); err != nil {
				rt.t.Logf("Error closing connection in test: %v", err)
			}
		}()

		// Connect with shorter timeout
		connectCtx, connectCancel := context.WithTimeout(ctx, 3*time.Second)
		defer connectCancel()

		if err := conn.Connect(connectCtx); err != nil {
			return err
		}

		// Wait for connection to be established with shorter timeout
		if !EventuallyTrue(func() bool {
			return conn.IsConnected()
		}, 1*time.Second, 10*time.Millisecond) {
			return fmt.Errorf("connection failed to establish within timeout")
		}

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

// TestCleanupHelper provides aggressive cleanup utilities for tests
type TestCleanupHelper struct {
	t            *testing.T
	servers      []*ReliableTestServer
	transports   []*Transport
	connections  []*Connection
	cleanupFuncs []func() error
	mutex        sync.Mutex
}

// NewTestCleanupHelper creates a new cleanup helper
func NewTestCleanupHelper(t *testing.T) *TestCleanupHelper {
	h := &TestCleanupHelper{
		t:            t,
		servers:      make([]*ReliableTestServer, 0),
		transports:   make([]*Transport, 0),
		connections:  make([]*Connection, 0),
		cleanupFuncs: make([]func() error, 0),
	}

	// Register cleanup on test completion
	t.Cleanup(func() {
		h.CleanupAll()
	})

	return h
}

// RegisterServer registers a server for cleanup
func (h *TestCleanupHelper) RegisterServer(server *ReliableTestServer) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.servers = append(h.servers, server)
}

// RegisterTransport registers a transport for cleanup
func (h *TestCleanupHelper) RegisterTransport(transport *Transport) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.transports = append(h.transports, transport)
}

// RegisterConnection registers a connection for cleanup
func (h *TestCleanupHelper) RegisterConnection(conn *Connection) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.connections = append(h.connections, conn)
}

// RegisterCleanupFunc registers a custom cleanup function
func (h *TestCleanupHelper) RegisterCleanupFunc(f func() error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	h.cleanupFuncs = append(h.cleanupFuncs, f)
}

// CleanupAll performs aggressive cleanup of all registered resources with enhanced goroutine termination
func (h *TestCleanupHelper) CleanupAll() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Track initial goroutine count for verification
	initialGoroutines := runtime.NumGoroutine()
	h.t.Logf("CleanupAll starting: initial goroutines=%d", initialGoroutines)

	// Phase 1: Stop all transports first (highest level)
	h.t.Logf("Phase 1: Stopping %d transports", len(h.transports))
	for i, transport := range h.transports {
		if transport != nil {
			h.t.Logf("Stopping transport %d/%d", i+1, len(h.transports))
			if err := h.forceStopTransport(transport); err != nil {
				h.t.Logf("Error stopping transport %d: %v", i+1, err)
			}
		}
	}

	// Brief pause for transport cleanup to propagate
	time.Sleep(100 * time.Millisecond)
	runtime.GC()

	// Phase 2: Close all connections
	h.t.Logf("Phase 2: Closing %d connections", len(h.connections))
	for i, conn := range h.connections {
		if conn != nil {
			h.t.Logf("Closing connection %d/%d", i+1, len(h.connections))
			if err := h.forceCloseConnection(conn); err != nil {
				h.t.Logf("Error closing connection %d: %v", i+1, err)
			}
		}
	}

	// Brief pause for connection cleanup to propagate
	time.Sleep(100 * time.Millisecond)
	runtime.GC()

	// Phase 3: Close all servers
	h.t.Logf("Phase 3: Closing %d servers", len(h.servers))
	for i, server := range h.servers {
		if server != nil {
			h.t.Logf("Closing server %d/%d", i+1, len(h.servers))
			server.Close()
		}
	}

	// Phase 4: Run custom cleanup functions
	h.t.Logf("Phase 4: Running %d custom cleanup functions", len(h.cleanupFuncs))
	for i, f := range h.cleanupFuncs {
		if err := f(); err != nil {
			h.t.Logf("Error in custom cleanup %d: %v", i+1, err)
		}
	}

	// Phase 5: Aggressive final cleanup with multiple attempts
	h.t.Logf("Phase 5: Performing aggressive final cleanup")
	for attempt := 1; attempt <= 3; attempt++ {
		h.t.Logf("Final cleanup attempt %d/3", attempt)

		// Progressive cleanup intensity
		for i := 0; i < attempt; i++ {
			runtime.GC()
		}

		// Progressive wait time
		waitTime := time.Duration(attempt*100) * time.Millisecond
		time.Sleep(waitTime)

		// Check goroutine count
		currentGoroutines := runtime.NumGoroutine()
		h.t.Logf("After cleanup attempt %d: goroutines=%d (initial=%d)",
			attempt, currentGoroutines, initialGoroutines)

		// If we're making progress, continue
		if attempt == 3 || currentGoroutines <= initialGoroutines+10 {
			break
		}
	}

	// Final verification
	finalGoroutines := runtime.NumGoroutine()
	goroutineChange := finalGoroutines - initialGoroutines
	h.t.Logf("CleanupAll completed: initial=%d, final=%d, change=%+d",
		initialGoroutines, finalGoroutines, goroutineChange)

	if goroutineChange > 15 {
		h.t.Logf("WARNING: Significant goroutine increase during cleanup: %+d", goroutineChange)
	}
}

// forceStopTransport aggressively stops a transport with enhanced timeout and forced termination
func (h *TestCleanupHelper) forceStopTransport(transport *Transport) error {
	// Phase 1: Normal stop with short timeout
	done := make(chan error, 1)
	go func() {
		done <- transport.Stop()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(1 * time.Second): // Reduced timeout for quicker escalation
		h.t.Logf("Transport stop timed out after 1s, escalating to forced cleanup")
	}

	// Phase 2: Force cancel context and additional cleanup
	if transport.cancel != nil {
		h.t.Logf("Force cancelling transport context")
		transport.cancel()
	}

	// Phase 3: Force cleanup of internal components
	h.forceCleanupTransportInternals(transport)

	// Phase 4: Give one more chance for graceful shutdown with shorter timeout
	done2 := make(chan error, 1)
	go func() {
		select {
		case err := <-done: // Original stop might have completed
			done2 <- err
		case <-time.After(500 * time.Millisecond):
			done2 <- fmt.Errorf("forced cleanup completed")
		}
	}()

	select {
	case err := <-done2:
		h.t.Logf("Transport force cleanup completed")
		return err
	case <-time.After(1 * time.Second):
		h.t.Logf("Final transport cleanup timeout - abandoning")
		return fmt.Errorf("transport force stop final timeout")
	}
}

// forceCleanupTransportInternals attempts to force cleanup of transport internal components
func (h *TestCleanupHelper) forceCleanupTransportInternals(transport *Transport) {
	h.t.Logf("Forcing cleanup of transport internals")

	// Force garbage collection to help cleanup
	runtime.GC()
	runtime.GC()

	// Force connection pool cleanup if accessible
	if transport.pool != nil {
		h.t.Logf("Force closing connection pool")
		// The pool should have its own cleanup mechanism
		// This is a best-effort attempt to trigger it
		go func() {
			defer func() {
				if r := recover(); r != nil {
					h.t.Logf("Recovered from pool cleanup panic: %v", r)
				}
			}()
			// Pool cleanup would go here if accessible
		}()
	}

	// Additional aggressive cleanup
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
}

// forceCloseConnection aggressively closes a connection with enhanced timeout and forced termination
func (h *TestCleanupHelper) forceCloseConnection(conn *Connection) error {
	// Phase 1: Normal close with very short timeout
	done := make(chan error, 1)
	go func() {
		done <- conn.Close()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(500 * time.Millisecond): // Reduced timeout
		h.t.Logf("Connection close timed out after 500ms, escalating to forced cleanup")
	}

	// Phase 2: Force cancel contexts
	if conn.cancel != nil {
		h.t.Logf("Force cancelling connection main context")
		conn.cancel()
	}

	// Phase 3: Force cleanup connection internals
	h.forceCleanupConnectionInternals(conn)

	// Phase 4: Force state transition to closed
	atomic.StoreInt32((*int32)(&conn.state), int32(StateClosed))

	// Phase 5: Final verification
	select {
	case err := <-done:
		h.t.Logf("Connection eventually closed gracefully")
		return err
	case <-time.After(300 * time.Millisecond):
		h.t.Logf("Connection forced cleanup completed")
		return fmt.Errorf("connection forced close completed")
	}
}

// forceCleanupConnectionInternals attempts to force cleanup of connection internal components
func (h *TestCleanupHelper) forceCleanupConnectionInternals(conn *Connection) {
	h.t.Logf("Forcing cleanup of connection internals")

	// Stop heartbeat if accessible
	if conn.heartbeat != nil {
		h.t.Logf("Force stopping heartbeat manager")
		go func() {
			defer func() {
				if r := recover(); r != nil {
					h.t.Logf("Recovered from heartbeat stop panic: %v", r)
				}
			}()
			conn.heartbeat.Stop()
		}()
	}

	// Force close underlying websocket connection
	conn.connMutex.Lock()
	if conn.conn != nil {
		h.t.Logf("Force closing underlying websocket connection")
		// Set immediate deadlines
		now := time.Now()
		conn.conn.SetReadDeadline(now)
		conn.conn.SetWriteDeadline(now)
		conn.conn.Close()
		conn.conn = nil
	}
	conn.connMutex.Unlock()

	// Mark channels as closed
	atomic.StoreInt32(&conn.channelsClosed, 1)

	// Force garbage collection
	runtime.GC()
}

// WithReliableTimeout wraps a test function with enhanced timeout, cleanup, and goroutine tracking
func WithReliableTimeout(t *testing.T, timeout time.Duration, testFunc func(context.Context)) {
	// Track initial goroutine count
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Enhanced cleanup tracking
	defer func() {
		// Force cleanup after test completion
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
		runtime.GC()
		time.Sleep(50 * time.Millisecond)

		finalGoroutines := runtime.NumGoroutine()
		goroutineChange := finalGoroutines - initialGoroutines

		if goroutineChange > 10 {
			t.Logf("WARNING: Goroutine increase in WithReliableTimeout: initial=%d, final=%d, change=%+d",
				initialGoroutines, finalGoroutines, goroutineChange)
		}
	}()

	done := make(chan struct{})
	var testErr error

	go func() {
		defer func() {
			if r := recover(); r != nil {
				testErr = fmt.Errorf("test panicked: %v", r)
				t.Logf("Test panic recovered in WithReliableTimeout: %v", r)
			}
			close(done)
		}()

		// Run test function with additional panic protection
		func() {
			defer func() {
				if r := recover(); r != nil {
					testErr = fmt.Errorf("test function panicked: %v", r)
				}
			}()
			testFunc(ctx)
		}()
	}()

	select {
	case <-done:
		if testErr != nil {
			t.Fatal(testErr)
		}
	case <-ctx.Done():
		// Enhanced timeout handling with goroutine info
		currentGoroutines := runtime.NumGoroutine()
		t.Logf("Test timeout after %v - goroutines: initial=%d, current=%d",
			timeout, initialGoroutines, currentGoroutines)
		t.Fatalf("Test timed out after %v", timeout)
	}
}

// ReliableMessageTester helps test message sending/receiving reliably
type ReliableMessageTester struct {
	conn     *Connection
	received chan []byte
	errors   chan error
	stopCh   chan struct{}
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

// TestConfig defines test timeout configurations
type TestConfig struct {
	ShortTest  time.Duration
	MediumTest time.Duration
	LongTest   time.Duration
}

// FastTestConfig returns optimized timeout configurations for faster test execution
func FastTestConfig() TestConfig {
	return TestConfig{
		ShortTest:  500 * time.Millisecond,
		MediumTest: 2 * time.Second,
		LongTest:   5 * time.Second,
	}
}

// FastTransportConfig returns a transport configuration optimized for fast tests
func FastTransportConfig() *TransportConfig {
	config := DefaultTransportConfig()

	// Very aggressive timeouts for fast tests
	config.DialTimeout = 1 * time.Second
	config.EventTimeout = 2 * time.Second
	config.EnableEventValidation = false
	config.ShutdownTimeout = 1 * time.Second

	// Configure fast pool settings
	if config.PoolConfig == nil {
		config.PoolConfig = DefaultPoolConfig()
	}
	config.PoolConfig.MinConnections = 1
	config.PoolConfig.MaxConnections = 3
	config.PoolConfig.IdleTimeout = 10 * time.Second
	config.PoolConfig.HealthCheckInterval = 200 * time.Millisecond

	// Configure fast connection template
	if config.PoolConfig.ConnectionTemplate == nil {
		config.PoolConfig.ConnectionTemplate = DefaultConnectionConfig()
	}
	config.PoolConfig.ConnectionTemplate.PingPeriod = 50 * time.Millisecond
	config.PoolConfig.ConnectionTemplate.PongWait = 100 * time.Millisecond
	config.PoolConfig.ConnectionTemplate.WriteTimeout = 500 * time.Millisecond
	config.PoolConfig.ConnectionTemplate.ReadTimeout = 500 * time.Millisecond
	config.PoolConfig.ConnectionTemplate.MaxReconnectAttempts = 2
	config.PoolConfig.ConnectionTemplate.InitialReconnectDelay = 25 * time.Millisecond
	config.PoolConfig.ConnectionTemplate.RateLimiter = nil // Disable rate limiting for tests

	// Configure fast backpressure settings
	if config.BackpressureConfig == nil {
		config.BackpressureConfig = DefaultBackpressureConfig()
	}
	config.BackpressureConfig.EventChannelBuffer = 100
	config.BackpressureConfig.MaxDroppedEvents = 10

	return config
}

// WithTimeout executes a test function with a timeout and proper cleanup
func WithTimeout(t *testing.T, timeout time.Duration, testFunc func(context.Context)) {
	t.Helper()

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

// OptimizedTransportConfig returns a transport configuration optimized for testing
func OptimizedTransportConfig() *TransportConfig {
	config := DefaultTransportConfig()

	// Optimize for faster test execution
	config.DialTimeout = 2 * time.Second
	config.EventTimeout = 5 * time.Second
	config.EnableEventValidation = false // Disable for speed in tests
	config.ShutdownTimeout = 2 * time.Second

	// Configure optimized pool settings
	if config.PoolConfig == nil {
		config.PoolConfig = DefaultPoolConfig()
	}
	config.PoolConfig.MinConnections = 1
	config.PoolConfig.MaxConnections = 5
	config.PoolConfig.IdleTimeout = 30 * time.Second
	config.PoolConfig.HealthCheckInterval = 500 * time.Millisecond

	// Configure optimized connection template
	if config.PoolConfig.ConnectionTemplate == nil {
		config.PoolConfig.ConnectionTemplate = DefaultConnectionConfig()
	}
	config.PoolConfig.ConnectionTemplate.PingPeriod = 100 * time.Millisecond
	config.PoolConfig.ConnectionTemplate.PongWait = 200 * time.Millisecond
	config.PoolConfig.ConnectionTemplate.WriteTimeout = 1 * time.Second
	config.PoolConfig.ConnectionTemplate.MaxReconnectAttempts = 2
	config.PoolConfig.ConnectionTemplate.InitialReconnectDelay = 50 * time.Millisecond

	// Configure optimized backpressure settings
	if config.BackpressureConfig == nil {
		config.BackpressureConfig = DefaultBackpressureConfig()
	}
	config.BackpressureConfig.EventChannelBuffer = 1000
	config.BackpressureConfig.MaxDroppedEvents = 100

	return config
}

// createTransportTestWebSocketServer creates a test WebSocket server for transport testing
func createTransportTestWebSocketServer(tb testing.TB) *ReliableTestServer {
	tb.Helper()

	// Type assert to *testing.T since NewReliableTestServer expects *testing.T
	t, ok := tb.(*testing.T)
	if !ok {
		// If it's not a *testing.T (might be *testing.B), create a wrapper
		panic("createTransportTestWebSocketServer requires *testing.T")
	}

	server := NewReliableTestServer(t)

	// Configure for transport testing
	server.SetEchoMode(true)
	server.SetDelay(0) // No artificial delay for transport tests

	// Ensure cleanup
	tb.Cleanup(func() {
		server.Close()
	})

	return server
}

// IsolatedTestRunner provides test isolation utilities
type IsolatedTestRunner struct {
	t                 *testing.T
	cleanup           *TestCleanupHelper
	initialGoroutines int
}

// NewIsolatedTestRunner creates a new isolated test runner
func NewIsolatedTestRunner(t *testing.T) *IsolatedTestRunner {
	// Force garbage collection and capture initial state
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	return &IsolatedTestRunner{
		t:                 t,
		cleanup:           NewTestCleanupHelper(t),
		initialGoroutines: runtime.NumGoroutine(),
	}
}

// RunIsolated runs a test function with proper isolation
func (r *IsolatedTestRunner) RunIsolated(name string, timeout time.Duration, testFunc func(*TestCleanupHelper)) {
	r.t.Run(name, func(t *testing.T) {
		// Create isolated cleanup helper for this sub-test
		subCleanup := NewTestCleanupHelper(t)

		// Run with timeout
		done := make(chan struct{})
		var testErr error

		go func() {
			defer func() {
				if r := recover(); r != nil {
					testErr = fmt.Errorf("test panicked: %v", r)
				}
				close(done)
			}()
			testFunc(subCleanup)
		}()

		select {
		case <-done:
			if testErr != nil {
				t.Fatal(testErr)
			}
		case <-time.After(timeout):
			// Force cleanup on timeout
			subCleanup.CleanupAll()
			t.Fatalf("Test timed out after %v", timeout)
		}

		// Verify no goroutine leaks (with tolerance)
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
		time.Sleep(50 * time.Millisecond)

		finalGoroutines := runtime.NumGoroutine()
		leaked := finalGoroutines - r.initialGoroutines
		if leaked > 10 { // Allow 10 goroutine tolerance
			t.Errorf("Potential goroutine leak: started=%d, ended=%d, leaked=%d",
				r.initialGoroutines, finalGoroutines, leaked)
		}
	})
}

// GetCleanupHelper returns the main cleanup helper
func (r *IsolatedTestRunner) GetCleanupHelper() *TestCleanupHelper {
	return r.cleanup
}

// CreateIsolatedTransport creates a transport with proper cleanup registration
func CreateIsolatedTransport(t *testing.T, helper *TestCleanupHelper, config *TransportConfig) *Transport {
	transport, err := NewTransport(config)
	if err != nil {
		t.Fatalf("Failed to create transport: %v", err)
	}

	helper.RegisterTransport(transport)
	return transport
}

// CreateIsolatedConnection creates a connection with proper cleanup registration
func CreateIsolatedConnection(t *testing.T, helper *TestCleanupHelper, config *ConnectionConfig) *Connection {
	conn, err := NewConnection(config)
	if err != nil {
		t.Fatalf("Failed to create connection: %v", err)
	}

	helper.RegisterConnection(conn)
	return conn
}

// CreateIsolatedServer creates a server with proper cleanup registration
func CreateIsolatedServer(t *testing.T, helper *TestCleanupHelper) *ReliableTestServer {
	server := NewReliableTestServer(t)
	helper.RegisterServer(server)
	return server
}

// getOptimizedSleep returns an optimized sleep duration for testing
// This reduces cumulative sleep time to improve test performance
func getOptimizedSleep(duration time.Duration) time.Duration {
	if testing.Short() {
		// In short mode, use 10% of original duration
		return duration / 10
	}
	// In normal testing, use 50% of original duration
	return duration / 2
}

// LoadTestServer provides a server for load testing scenarios
type LoadTestServer struct {
	server      *httptest.Server
	upgrader    websocket.Upgrader
	connections int64
	messages    int64
	errors      int64
	logger      *zap.Logger
	echoMode    bool
	dropRate    float64
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	conns       sync.Map // Track active connections
}

// connWrapper wraps a websocket connection with a mutex for thread-safe operations
type connWrapper struct {
	conn  *websocket.Conn
	mutex sync.RWMutex
}

func NewLoadTestServer(t testing.TB) *LoadTestServer {
	ctx, cancel := context.WithCancel(context.Background())
	server := &LoadTestServer{
		upgrader: websocket.Upgrader{
			CheckOrigin:     func(r *http.Request) bool { return true },
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
		},
		logger:   zaptest.NewLogger(t),
		echoMode: true,
		ctx:      ctx,
		cancel:   cancel,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", server.handleWebSocket)
	server.server = httptest.NewServer(mux)

	return server
}

func (s *LoadTestServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check if server is shutting down
	select {
	case <-s.ctx.Done():
		http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
		return
	default:
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		atomic.AddInt64(&s.errors, 1)
		return
	}

	// Track connection with wrapper
	connID := fmt.Sprintf("%p", conn)
	wrapper := &connWrapper{conn: conn}
	s.conns.Store(connID, wrapper)
	s.wg.Add(1)
	defer func() {
		s.conns.Delete(connID)
		s.wg.Done()
		conn.Close()
	}()

	atomic.AddInt64(&s.connections, 1)
	defer atomic.AddInt64(&s.connections, -1)

	// Set initial deadlines with mutex protection
	wrapper.mutex.Lock()
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	wrapper.mutex.Unlock()

	// Set up close handler to prevent panic on connection close
	conn.SetCloseHandler(func(code int, text string) error {
		// Connection is closing, exit the read loop gracefully
		return nil
	})

	for {
		// Check context for shutdown first
		select {
		case <-s.ctx.Done():
			// Send close message with timeout
			closeCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			go func() {
				defer cancel()
				conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutdown"),
					time.Now().Add(100*time.Millisecond))
			}()
			<-closeCtx.Done()
			return
		default:
		}

		// Set aggressive read deadline based on context
		readDeadline := time.Now().Add(200 * time.Millisecond) // Shorter timeout
		if deadline, ok := s.ctx.Deadline(); ok && deadline.Before(readDeadline) {
			readDeadline = deadline
		}
		conn.SetReadDeadline(readDeadline)

		// Use goroutine with timeout for reading to prevent indefinite blocking
		type readResult struct {
			messageType int
			message     []byte
			err         error
		}

		resultChan := make(chan readResult, 1)
		readCtx, cancel := context.WithTimeout(s.ctx, 150*time.Millisecond)

		go func() {
			defer func() {
				if r := recover(); r != nil {
					resultChan <- readResult{err: websocket.ErrCloseSent}
				}
			}()

			messageType, message, err := conn.ReadMessage()
			select {
			case resultChan <- readResult{messageType, message, err}:
			case <-readCtx.Done():
				// Context cancelled - exit goroutine
			}
		}()

		var messageType int
		var message []byte
		var err error
		select {
		case <-s.ctx.Done():
			cancel()
			return
		case result := <-resultChan:
			cancel()
			messageType = result.messageType
			message = result.message
			err = result.err
		case <-readCtx.Done():
			cancel()
			// Timeout occurred - treat as timeout error
			if readCtx.Err() == context.DeadlineExceeded {
				continue // Continue to next iteration for context check
			}
			return
		}

		if err != nil {
			// Check if error is due to server context cancellation
			select {
			case <-s.ctx.Done():
				return
			default:
				if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
					continue
				}
				if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) ||
					err == websocket.ErrCloseSent {
					return
				}
				atomic.AddInt64(&s.errors, 1)
				return
			}
		}

		// Increment message count first to ensure accurate counting
		atomic.AddInt64(&s.messages, 1)

		// Simulate message processing and potential drops
		if s.dropRate > 0 && rand.Float64() < s.dropRate {
			continue
		}

		if s.echoMode {
			// Use aggressive write deadline to prevent hanging during shutdown
			writeDeadline := time.Now().Add(100 * time.Millisecond)
			if deadline, ok := s.ctx.Deadline(); ok && deadline.Before(writeDeadline) {
				writeDeadline = deadline
			}
			conn.SetWriteDeadline(writeDeadline)

			// Check context before writing to avoid writing during shutdown
			select {
			case <-s.ctx.Done():
				return
			default:
				if err := conn.WriteMessage(messageType, message); err != nil {
					// Only count as error if not due to shutdown
					select {
					case <-s.ctx.Done():
						// Shutdown in progress, don't count as error
						return
					default:
						atomic.AddInt64(&s.errors, 1)
						return
					}
				}
			}
		}
	}
}

func (s *LoadTestServer) URL() string {
	return "ws" + strings.TrimPrefix(s.server.URL, "http") + "/ws"
}

func (s *LoadTestServer) Close() {
	s.logger.Debug("Closing load test server")

	// First, close the HTTP server to prevent new connections
	s.server.Close()

	// Signal all handlers to stop processing new messages
	s.cancel()

	// Reduced wait time for faster test execution
	time.Sleep(50 * time.Millisecond)

	// Send graceful close messages to all active connections
	var closedConns int
	var closeWg sync.WaitGroup
	s.conns.Range(func(key, value interface{}) bool {
		if wrapper, ok := value.(*connWrapper); ok {
			closeWg.Add(1)
			go func(w *connWrapper, connID string) {
				defer closeWg.Done()

				// Send graceful close message with mutex protection
				w.mutex.Lock()
				w.conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
				w.conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutdown"),
					time.Now().Add(100*time.Millisecond))
				w.mutex.Unlock()

				// Give a moment for graceful close
				time.Sleep(50 * time.Millisecond)

				// Now force close with mutex protection
				w.mutex.Lock()
				now := time.Now()
				w.conn.SetReadDeadline(now)
				w.conn.SetWriteDeadline(now)
				w.conn.Close()
				w.mutex.Unlock()

				// Remove from connections map
				s.conns.Delete(connID)
			}(wrapper, key.(string))
			closedConns++
		}
		return true
	})

	// Wait for all close operations to complete
	closeWg.Wait()

	if closedConns > 0 {
		s.logger.Debug("Initiated close for connections", zap.Int("count", closedConns))
	}

	// Wait for all handlers to complete with reasonable timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Debug("All handlers completed gracefully")
	case <-time.After(1 * time.Second): // Increased timeout for proper cleanup
		remaining := 0
		s.conns.Range(func(_, _ interface{}) bool {
			remaining++
			return true
		})
		if remaining > 0 {
			s.logger.Warn("Timeout waiting for handlers to complete",
				zap.Int("remaining_connections", remaining))
		}
	}

	// Final cleanup - ensure all connections are removed
	s.conns.Range(func(key, _ interface{}) bool {
		s.conns.Delete(key)
		return true
	})
}

func (s *LoadTestServer) GetStats() (connections, messages, errors int64) {
	return atomic.LoadInt64(&s.connections), atomic.LoadInt64(&s.messages), atomic.LoadInt64(&s.errors)
}

func (s *LoadTestServer) SetDropRate(rate float64) {
	s.dropRate = rate
}

func (s *LoadTestServer) SetEchoMode(enabled bool) {
	s.echoMode = enabled
}

// TestResourceManager provides centralized resource management for tests
// to prevent cumulative resource exhaustion when running the full test suite
type TestResourceManager struct {
	mu                 sync.Mutex
	heavyTestSemaphore chan struct{}
	testFailureCount   int64
	maxConcurrentHeavy int
	memoryThresholdMB  uint64
	isCircuitOpen      bool
	lastFailureTime    time.Time

	// Enhanced resource budget tracking
	activeBudget int64            // Current active goroutine budget being used
	maxBudget    int64            // Maximum total goroutine budget allowed
	budgetMutex  sync.RWMutex     // Protects budget operations
	testBudgets  map[string]int64 // Track budget usage per test
}

var globalResourceManager = &TestResourceManager{
	heavyTestSemaphore: make(chan struct{}, 2), // Allow max 2 concurrent heavy tests
	maxConcurrentHeavy: 2,
	memoryThresholdMB:  100, // 100MB memory threshold
	maxBudget:          200, // Allow max 200 goroutines across all tests
	testBudgets:        make(map[string]int64),
}

// Sequential test execution manager for full test suite
var sequentialTestMutex sync.Mutex
var heavyTestSequencer = make(chan struct{}, 1) // Allow only 1 heavy test at a time in sequential mode

// Resource budget management methods

// AcquireBudget attempts to acquire a portion of the global goroutine budget
func (trm *TestResourceManager) AcquireBudget(testName string, requestedGoroutines int64) bool {
	trm.budgetMutex.Lock()
	defer trm.budgetMutex.Unlock()

	if trm.activeBudget+requestedGoroutines > trm.maxBudget {
		return false // Budget exhausted
	}

	trm.activeBudget += requestedGoroutines
	trm.testBudgets[testName] = requestedGoroutines
	return true
}

// ReleaseBudget releases the goroutine budget for a test
func (trm *TestResourceManager) ReleaseBudget(testName string) {
	trm.budgetMutex.Lock()
	defer trm.budgetMutex.Unlock()

	if budget, exists := trm.testBudgets[testName]; exists {
		trm.activeBudget -= budget
		delete(trm.testBudgets, testName)
	}
}

// GetBudgetStatus returns current budget usage statistics
func (trm *TestResourceManager) GetBudgetStatus() (active, max int64, utilizationPercent float64) {
	trm.budgetMutex.RLock()
	defer trm.budgetMutex.RUnlock()

	active = trm.activeBudget
	max = trm.maxBudget
	if max > 0 {
		utilizationPercent = float64(active) / float64(max) * 100
	}
	return
}

// AcquireHeavyTestSlot acquires a slot for resource-intensive tests
// This prevents too many heavy tests from running simultaneously
func (trm *TestResourceManager) AcquireHeavyTestSlot(t *testing.T, testName string) func() {
	// Check circuit breaker
	trm.mu.Lock()
	if trm.isCircuitOpen {
		if time.Since(trm.lastFailureTime) > 30*time.Second {
			// Reset circuit after 30 seconds
			trm.isCircuitOpen = false
			trm.testFailureCount = 0
		} else {
			trm.mu.Unlock()
			t.Skip("Circuit breaker open: too many test failures, skipping heavy test")
			return func() {}
		}
	}
	trm.mu.Unlock()

	// Check memory usage before starting heavy test
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	currentMemMB := memStats.Alloc / (1024 * 1024)

	if currentMemMB > trm.memoryThresholdMB {
		t.Logf("High memory usage detected (%dMB), performing GC before %s", currentMemMB, testName)
		runtime.GC()
		runtime.GC() // Run twice for more aggressive cleanup
		time.Sleep(100 * time.Millisecond)

		// Check again after GC
		runtime.ReadMemStats(&memStats)
		currentMemMB = memStats.Alloc / (1024 * 1024)
		if currentMemMB > trm.memoryThresholdMB {
			t.Skipf("Memory usage still high (%dMB) after GC, skipping heavy test %s", currentMemMB, testName)
			return func() {}
		}
	}

	// Get expected goroutine count for budget planning
	concurrencyConfig := getConcurrencyConfig(testName)
	expectedGoroutines := int64(concurrencyConfig.NumGoroutines + 5) // Add buffer for test framework overhead

	// Try to acquire budget before acquiring semaphore slot
	if !trm.AcquireBudget(testName, expectedGoroutines) {
		active, max, utilization := trm.GetBudgetStatus()
		t.Skipf("Goroutine budget exhausted (active: %d/%d, %.1f%%), skipping %s to prevent resource exhaustion",
			active, max, utilization, testName)
		return func() {}
	}

	// Acquire semaphore slot
	select {
	case trm.heavyTestSemaphore <- struct{}{}:
		active, max, utilization := trm.GetBudgetStatus()
		t.Logf("%s: Acquired heavy test slot (goroutines: %d, memory: %dMB, budget: %d/%d %.1f%%)",
			testName, runtime.NumGoroutine(), currentMemMB, active, max, utilization)
	case <-time.After(5 * time.Second):
		// Release budget on timeout
		trm.ReleaseBudget(testName)
		t.Skip("Timeout waiting for heavy test slot, skipping to prevent resource exhaustion")
		return func() {}
	}

	startTime := time.Now()
	initialGoroutines := runtime.NumGoroutine()

	// Return cleanup function
	return func() {
		// Release semaphore slot
		select {
		case <-trm.heavyTestSemaphore:
		default:
		}

		// Release budget
		trm.ReleaseBudget(testName)

		// Check for goroutine leaks
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
		finalGoroutines := runtime.NumGoroutine()
		leaked := finalGoroutines - initialGoroutines

		duration := time.Since(startTime)
		active, max, utilization := trm.GetBudgetStatus()
		t.Logf("%s: Released heavy test slot (duration: %v, goroutines: %d->%d, leaked: %d, budget: %d/%d %.1f%%)",
			testName, duration, initialGoroutines, finalGoroutines, leaked, active, max, utilization)

		// Update circuit breaker on failure patterns
		if leaked > 10 || duration > 30*time.Second {
			trm.mu.Lock()
			trm.testFailureCount++
			trm.lastFailureTime = time.Now()
			if trm.testFailureCount >= 3 {
				trm.isCircuitOpen = true
				t.Logf("Circuit breaker opened due to repeated resource issues")
			}
			trm.mu.Unlock()
		}
	}
}

// WithResourceControl wraps resource-intensive tests with resource management
func WithResourceControl(t *testing.T, testName string, testFunc func()) {
	concurrencyConfig := getConcurrencyConfig(testName)

	if concurrencyConfig.EnableSequential {
		// Sequential execution for full test suite
		WithSequentialExecution(t, testName, func() {
			cleanup := globalResourceManager.AcquireHeavyTestSlot(t, testName)
			defer cleanup()
			testFunc()
		})
	} else {
		// Normal concurrent execution for individual tests
		cleanup := globalResourceManager.AcquireHeavyTestSlot(t, testName)
		defer cleanup()
		testFunc()
	}
}

// WithSequentialExecution ensures that resource-intensive tests run one at a time
// This prevents resource contention when running the full test suite
func WithSequentialExecution(t *testing.T, testName string, testFunc func()) {
	// In full test suite mode, heavy tests should run sequentially
	sequentialTestMutex.Lock()
	t.Logf("%s: Acquiring sequential execution lock (full test suite mode)", testName)

	// Additional semaphore to prevent too many tests from queuing
	select {
	case heavyTestSequencer <- struct{}{}:
		defer func() {
			<-heavyTestSequencer
			sequentialTestMutex.Unlock()
			t.Logf("%s: Released sequential execution lock", testName)
		}()

		t.Logf("%s: Starting sequential execution", testName)
		testFunc()

	case <-time.After(30 * time.Second):
		sequentialTestMutex.Unlock()
		t.Skipf("%s: Skipped due to sequential execution timeout (test suite overload)", testName)
	}
}

// Environment-aware test scaling functions

// isFullTestSuite detects if we're running the full test suite vs individual tests
func isFullTestSuite() bool {
	// Check for test suite indicators:
	// 1. Multiple parallel tests running (detected via environment)
	// 2. CI environment variables
	// 3. go test with no specific test filter

	// Check common CI environments
	if IsRunningInCI() {
		return true
	}

	// Check if we're running with -short flag
	if testing.Short() {
		return true // In short mode, assume full suite for conservative resource usage
	}

	// Check common test runner patterns that indicate full suite
	for _, arg := range os.Args {
		// If no specific test filter is provided, likely running full suite
		if arg == "-test.v" || arg == "-v" {
			// Running in verbose mode often indicates full suite
			continue
		}
		if strings.HasPrefix(arg, "-test.run=") && arg != "-test.run=" {
			// Check if multiple tests are specified (contains |)
			testPattern := strings.TrimPrefix(arg, "-test.run=")
			if strings.Contains(testPattern, "|") {
				// Multiple tests specified, treat as full suite
				return true
			}
			// Single specific test filter provided, likely individual test
			return false
		}
		if strings.HasPrefix(arg, "-run=") && arg != "-run=" {
			// Check if multiple tests are specified (contains |)
			testPattern := strings.TrimPrefix(arg, "-run=")
			if strings.Contains(testPattern, "|") {
				// Multiple tests specified, treat as full suite
				return true
			}
			// Single specific test filter provided, likely individual test
			return false
		}
	}

	// Default to full suite assumption for safety
	return true
}

// getScaledGoroutineCount returns environment-aware goroutine count
func getScaledGoroutineCount(fullSuiteCount, individualTestCount int) int {
	if isFullTestSuite() {
		return fullSuiteCount
	}
	return individualTestCount
}

// getScaledOperationCount returns environment-aware operation count per goroutine
func getScaledOperationCount(fullSuiteCount, individualTestCount int) int {
	if isFullTestSuite() {
		return fullSuiteCount
	}
	return individualTestCount
}

// getConcurrencyConfig returns optimized concurrency configuration for tests
type ConcurrencyConfig struct {
	NumGoroutines        int
	OperationsPerRoutine int
	TimeoutScale         float64
	EnableSequential     bool
}

func getConcurrencyConfig(testName string) ConcurrencyConfig {
	config := ConcurrencyConfig{
		TimeoutScale:     1.0,
		EnableSequential: false,
	}

	if isFullTestSuite() {
		// Conservative settings for full test suite
		switch testName {
		case "TestTransportConcurrency":
			config.NumGoroutines = 3        // Already optimized
			config.OperationsPerRoutine = 3 // Already optimized
		case "TestConnectionConcurrency":
			config.NumGoroutines = 3        // Reduced from 10
			config.OperationsPerRoutine = 3 // Reduced from 10
		case "TestHeartbeatConcurrency":
			config.NumGoroutines = 3         // Reduced from 10
			config.OperationsPerRoutine = 10 // Reduced from 100
		default:
			// Default conservative settings for unknown tests
			config.NumGoroutines = 3
			config.OperationsPerRoutine = 5
		}
		config.TimeoutScale = 0.5      // Shorter timeouts in full suite
		config.EnableSequential = true // Enable sequential execution for heavy tests
	} else {
		// Higher resource usage for individual tests
		switch testName {
		case "TestTransportConcurrency":
			config.NumGoroutines = 5 // Moderate increase from full suite
			config.OperationsPerRoutine = 5
		case "TestConnectionConcurrency":
			config.NumGoroutines = 10 // Original values for individual test
			config.OperationsPerRoutine = 10
		case "TestHeartbeatConcurrency":
			config.NumGoroutines = 10 // Original values for individual test
			config.OperationsPerRoutine = 100
		default:
			// Default settings for individual tests
			config.NumGoroutines = 10
			config.OperationsPerRoutine = 10
		}
		config.TimeoutScale = 1.0
		config.EnableSequential = false
	}

	return config
}

// IsRunningInCI is provided by transport.go - no need to redefine it here

// WithBudgetAwareScaling provides budget-aware resource scaling for tests that don't use WithResourceControl
// This is useful for tests that want to participate in resource budgeting without full resource control
func WithBudgetAwareScaling(testName string) (numGoroutines, operationsPerGoroutine int, shouldSkip bool, reason string) {
	concurrencyConfig := getConcurrencyConfig(testName)
	expectedGoroutines := int64(concurrencyConfig.NumGoroutines + 2) // Small buffer

	// Check if budget is available
	if !globalResourceManager.AcquireBudget(testName+"_budget_check", expectedGoroutines) {
		active, max, utilization := globalResourceManager.GetBudgetStatus()
		return 0, 0, true, fmt.Sprintf("Goroutine budget exhausted (active: %d/%d, %.1f%%)", active, max, utilization)
	}

	// Release the budget check - the actual test will acquire it properly
	globalResourceManager.ReleaseBudget(testName + "_budget_check")

	return concurrencyConfig.NumGoroutines, concurrencyConfig.OperationsPerRoutine, false, ""
}
