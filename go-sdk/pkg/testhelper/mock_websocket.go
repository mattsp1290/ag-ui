package testhelper

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// MockWebSocketConn provides a mock implementation of a WebSocket connection
type MockWebSocketConn struct {
	t              *testing.T
	mu             sync.RWMutex
	closed         bool
	messages       []MockMessage
	closeCode      int
	closeText      string
	readDeadline   time.Time
	writeDeadline  time.Time
	onWriteMessage func(messageType int, data []byte) error
	onClose        func(code int, text string) error
	localAddr      net.Addr
	remoteAddr     net.Addr
}

// MockMessage represents a message in the mock WebSocket
type MockMessage struct {
	Type int
	Data []byte
	Time time.Time
}

// NewMockWebSocketConn creates a new mock WebSocket connection
func NewMockWebSocketConn(t *testing.T) *MockWebSocketConn {
	return &MockWebSocketConn{
		t:          t,
		messages:   make([]MockMessage, 0),
		localAddr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0},
	}
}

// WriteMessage implements the websocket.Conn interface
func (m *MockWebSocketConn) WriteMessage(messageType int, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return websocket.ErrCloseSent
	}
	
	m.t.Logf("MockWebSocket: Writing message type %d, data: %s", messageType, string(data))
	
	if m.onWriteMessage != nil {
		return m.onWriteMessage(messageType, data)
	}
	
	return nil
}

// ReadMessage implements the websocket.Conn interface
func (m *MockWebSocketConn) ReadMessage() (messageType int, p []byte, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return 0, nil, websocket.ErrCloseSent
	}
	
	if len(m.messages) == 0 {
		// Simulate waiting for a message
		return 0, nil, errors.New("no messages available")
	}
	
	message := m.messages[0]
	m.messages = m.messages[1:]
	
	m.t.Logf("MockWebSocket: Reading message type %d, data: %s", message.Type, string(message.Data))
	
	return message.Type, message.Data, nil
}

// Close implements the websocket.Conn interface
func (m *MockWebSocketConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.closed {
		return nil
	}
	
	m.closed = true
	m.t.Log("MockWebSocket: Connection closed")
	
	if m.onClose != nil {
		return m.onClose(m.closeCode, m.closeText)
	}
	
	return nil
}

// SetReadDeadline implements the websocket.Conn interface
func (m *MockWebSocketConn) SetReadDeadline(t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readDeadline = t
	return nil
}

// SetWriteDeadline implements the websocket.Conn interface
func (m *MockWebSocketConn) SetWriteDeadline(t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeDeadline = t
	return nil
}

// SetPongHandler implements the websocket.Conn interface
func (m *MockWebSocketConn) SetPongHandler(h func(appData string) error) {
	// Mock implementation - store handler if needed
}

// SetPingHandler implements the websocket.Conn interface
func (m *MockWebSocketConn) SetPingHandler(h func(appData string) error) {
	// Mock implementation - store handler if needed
}

// LocalAddr implements the websocket.Conn interface
func (m *MockWebSocketConn) LocalAddr() net.Addr {
	return m.localAddr
}

// RemoteAddr implements the websocket.Conn interface
func (m *MockWebSocketConn) RemoteAddr() net.Addr {
	return m.remoteAddr
}

// Mock-specific methods for test control

// AddMessage adds a message to be returned by ReadMessage
func (m *MockWebSocketConn) AddMessage(messageType int, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.messages = append(m.messages, MockMessage{
		Type: messageType,
		Data: data,
		Time: time.Now(),
	})
}

// SetWriteHandler sets a handler for WriteMessage calls
func (m *MockWebSocketConn) SetWriteHandler(handler func(messageType int, data []byte) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onWriteMessage = handler
}

// SetCloseHandler sets a handler for Close calls
func (m *MockWebSocketConn) SetCloseHandler(handler func(code int, text string) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onClose = handler
}

// IsClosed returns whether the connection is closed
func (m *MockWebSocketConn) IsClosed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.closed
}

// GetMessageCount returns the number of pending messages
func (m *MockWebSocketConn) GetMessageCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.messages)
}

// MockWebSocketDialer provides a mock dialer for WebSocket connections
type MockWebSocketDialer struct {
	t           *testing.T
	mu          sync.Mutex
	connections map[string]*MockWebSocketConn
	dialError   error
	dialDelay   time.Duration
}

// NewMockWebSocketDialer creates a new mock WebSocket dialer
func NewMockWebSocketDialer(t *testing.T) *MockWebSocketDialer {
	return &MockWebSocketDialer{
		t:           t,
		connections: make(map[string]*MockWebSocketConn),
	}
}

// Dial implements a mock WebSocket dialer
func (d *MockWebSocketDialer) Dial(urlStr string, requestHeader http.Header) (*MockWebSocketConn, *http.Response, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if d.dialDelay > 0 {
		time.Sleep(d.dialDelay)
	}
	
	if d.dialError != nil {
		return nil, nil, d.dialError
	}
	
	d.t.Logf("MockWebSocket: Dialing %s", urlStr)
	
	conn := NewMockWebSocketConn(d.t)
	d.connections[urlStr] = conn
	
	// Create a mock HTTP response
	resp := &http.Response{
		Status:     "101 Switching Protocols",
		StatusCode: 101,
		Header:     make(http.Header),
	}
	resp.Header.Set("Upgrade", "websocket")
	resp.Header.Set("Connection", "Upgrade")
	
	return conn, resp, nil
}

// DialContext implements a mock WebSocket dialer with context
func (d *MockWebSocketDialer) DialContext(ctx context.Context, urlStr string, requestHeader http.Header) (*MockWebSocketConn, *http.Response, error) {
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	default:
		return d.Dial(urlStr, requestHeader)
	}
}

// SetDialError sets an error to be returned by Dial
func (d *MockWebSocketDialer) SetDialError(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dialError = err
}

// SetDialDelay sets a delay for Dial operations
func (d *MockWebSocketDialer) SetDialDelay(delay time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.dialDelay = delay
}

// GetConnection returns a connection for a given URL
func (d *MockWebSocketDialer) GetConnection(urlStr string) *MockWebSocketConn {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.connections[urlStr]
}

// GetAllConnections returns all active connections
func (d *MockWebSocketDialer) GetAllConnections() map[string]*MockWebSocketConn {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	result := make(map[string]*MockWebSocketConn)
	for k, v := range d.connections {
		result[k] = v
	}
	return result
}

// CloseAll closes all connections
func (d *MockWebSocketDialer) CloseAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	for _, conn := range d.connections {
		conn.Close()
	}
	d.connections = make(map[string]*MockWebSocketConn)
}

// MockWebSocketServer provides a mock WebSocket server for testing
type MockWebSocketServer struct {
	t           *testing.T
	server      *http.Server
	listener    net.Listener
	upgrader    websocket.Upgrader
	connections map[string]*websocket.Conn
	mu          sync.Mutex
	onConnect   func(*websocket.Conn)
	onMessage   func(*websocket.Conn, int, []byte)
	onClose     func(*websocket.Conn, error)
}

// NewMockWebSocketServer creates a new mock WebSocket server
func NewMockWebSocketServer(t *testing.T) *MockWebSocketServer {
	return &MockWebSocketServer{
		t:           t,
		connections: make(map[string]*websocket.Conn),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// Start starts the mock WebSocket server
func (s *MockWebSocketServer) Start() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	
	s.listener = listener
	
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	
	s.server = &http.Server{Handler: mux}
	
	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.t.Logf("Server error: %v", err)
		}
	}()
	
	s.t.Logf("MockWebSocket server started on %s", listener.Addr().String())
	return nil
}

// Stop stops the mock WebSocket server
func (s *MockWebSocketServer) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), GlobalTimeouts.Cleanup)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// GetURL returns the WebSocket URL for the server
func (s *MockWebSocketServer) GetURL() string {
	if s.listener == nil {
		return ""
	}
	return fmt.Sprintf("ws://%s/ws", s.listener.Addr().String())
}

// GetHTTPURL returns the HTTP URL for the server  
func (s *MockWebSocketServer) GetHTTPURL() string {
	if s.listener == nil {
		return ""
	}
	return fmt.Sprintf("http://%s", s.listener.Addr().String())
}

// SetHandlers sets event handlers for the server
func (s *MockWebSocketServer) SetHandlers(
	onConnect func(*websocket.Conn),
	onMessage func(*websocket.Conn, int, []byte),
	onClose func(*websocket.Conn, error),
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.onConnect = onConnect
	s.onMessage = onMessage
	s.onClose = onClose
}

// BroadcastMessage sends a message to all connected clients
func (s *MockWebSocketServer) BroadcastMessage(messageType int, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for _, conn := range s.connections {
		if err := conn.WriteMessage(messageType, data); err != nil {
			s.t.Logf("Error broadcasting message: %v", err)
		}
	}
}

// GetConnectionCount returns the number of active connections
func (s *MockWebSocketServer) GetConnectionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.connections)
}

// handleWebSocket handles WebSocket connections
func (s *MockWebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.t.Logf("WebSocket upgrade error: %v", err)
		return
	}
	
	connID := fmt.Sprintf("%s_%d", conn.RemoteAddr().String(), time.Now().UnixNano())
	
	s.mu.Lock()
	s.connections[connID] = conn
	s.mu.Unlock()
	
	defer func() {
		s.mu.Lock()
		delete(s.connections, connID)
		s.mu.Unlock()
		conn.Close()
	}()
	
	if s.onConnect != nil {
		s.onConnect(conn)
	}
	
	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			if s.onClose != nil {
				s.onClose(conn, err)
			}
			break
		}
		
		if s.onMessage != nil {
			s.onMessage(conn, messageType, data)
		}
	}
}

// WebSocketTestSuite provides a complete test setup for WebSocket testing
type WebSocketTestSuite struct {
	t           *testing.T
	server      *MockWebSocketServer
	dialer      *MockWebSocketDialer
	cleanup     *CleanupHelper
	timeouts    *TimeoutConfig
}

// NewWebSocketTestSuite creates a new WebSocket test suite
func NewWebSocketTestSuite(t *testing.T) *WebSocketTestSuite {
	return &WebSocketTestSuite{
		t:        t,
		server:   NewMockWebSocketServer(t),
		dialer:   NewMockWebSocketDialer(t),
		cleanup:  NewCleanupHelper(t),
		timeouts: GlobalTimeouts,
	}
}

// Setup initializes the test suite
func (suite *WebSocketTestSuite) Setup() error {
	if err := suite.server.Start(); err != nil {
		return err
	}
	
	suite.cleanup.Add(func() {
		suite.server.Stop()
		suite.dialer.CloseAll()
	})
	
	return nil
}

// GetServerURL returns the WebSocket server URL
func (suite *WebSocketTestSuite) GetServerURL() string {
	return suite.server.GetURL()
}

// GetDialer returns the mock dialer
func (suite *WebSocketTestSuite) GetDialer() *MockWebSocketDialer {
	return suite.dialer
}

// GetServer returns the mock server
func (suite *WebSocketTestSuite) GetServer() *MockWebSocketServer {
	return suite.server
}

// WithTimeouts sets custom timeouts for the test suite
func (suite *WebSocketTestSuite) WithTimeouts(timeouts *TimeoutConfig) *WebSocketTestSuite {
	suite.timeouts = timeouts
	return suite
}