package testhelper

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// MockNetworkConnection provides a mock network connection for testing
type MockNetworkConnection struct {
	t              *testing.T
	mu             sync.RWMutex
	localAddr      net.Addr
	remoteAddr     net.Addr
	readBuffer     []byte
	writeBuffer    []byte
	closed         bool
	readDeadline   time.Time
	writeDeadline  time.Time
	onRead         func([]byte) (int, error)
	onWrite        func([]byte) (int, error)
	onClose        func() error
	readBlockTime  time.Duration
	writeBlockTime time.Duration
	simulateError  error
	bytesRead      int64
	bytesWritten   int64
}

// NewMockNetworkConnection creates a new mock network connection
func NewMockNetworkConnection(t *testing.T, localAddr, remoteAddr net.Addr) *MockNetworkConnection {
	return &MockNetworkConnection{
		t:           t,
		localAddr:   localAddr,
		remoteAddr:  remoteAddr,
		readBuffer:  make([]byte, 0),
		writeBuffer: make([]byte, 0),
	}
}

// Read implements the net.Conn interface
func (m *MockNetworkConnection) Read(b []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return 0, io.EOF
	}

	if m.simulateError != nil {
		return 0, m.simulateError
	}

	// Simulate read blocking
	if m.readBlockTime > 0 {
		time.Sleep(m.readBlockTime)
	}

	if m.onRead != nil {
		return m.onRead(b)
	}

	if len(m.readBuffer) == 0 {
		// No data available
		return 0, fmt.Errorf("no data available")
	}

	n := copy(b, m.readBuffer)
	m.readBuffer = m.readBuffer[n:]
	m.bytesRead += int64(n)

	m.t.Logf("MockNetwork: Read %d bytes", n)
	return n, nil
}

// Write implements the net.Conn interface
func (m *MockNetworkConnection) Write(b []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return 0, fmt.Errorf("connection closed")
	}

	if m.simulateError != nil {
		return 0, m.simulateError
	}

	// Simulate write blocking
	if m.writeBlockTime > 0 {
		time.Sleep(m.writeBlockTime)
	}

	if m.onWrite != nil {
		return m.onWrite(b)
	}

	m.writeBuffer = append(m.writeBuffer, b...)
	m.bytesWritten += int64(len(b))

	m.t.Logf("MockNetwork: Wrote %d bytes", len(b))
	return len(b), nil
}

// Close implements the net.Conn interface
func (m *MockNetworkConnection) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true
	m.t.Log("MockNetwork: Connection closed")

	if m.onClose != nil {
		return m.onClose()
	}

	return nil
}

// LocalAddr implements the net.Conn interface
func (m *MockNetworkConnection) LocalAddr() net.Addr {
	return m.localAddr
}

// RemoteAddr implements the net.Conn interface
func (m *MockNetworkConnection) RemoteAddr() net.Addr {
	return m.remoteAddr
}

// SetDeadline implements the net.Conn interface
func (m *MockNetworkConnection) SetDeadline(t time.Time) error {
	m.SetReadDeadline(t)
	m.SetWriteDeadline(t)
	return nil
}

// SetReadDeadline implements the net.Conn interface
func (m *MockNetworkConnection) SetReadDeadline(t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readDeadline = t
	return nil
}

// SetWriteDeadline implements the net.Conn interface
func (m *MockNetworkConnection) SetWriteDeadline(t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeDeadline = t
	return nil
}

// Mock-specific methods

// AddReadData adds data to be returned by Read calls
func (m *MockNetworkConnection) AddReadData(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readBuffer = append(m.readBuffer, data...)
}

// GetWrittenData returns all data written to the connection
func (m *MockNetworkConnection) GetWrittenData() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]byte, len(m.writeBuffer))
	copy(result, m.writeBuffer)
	return result
}

// SetReadHandler sets a custom read handler
func (m *MockNetworkConnection) SetReadHandler(handler func([]byte) (int, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onRead = handler
}

// SetWriteHandler sets a custom write handler
func (m *MockNetworkConnection) SetWriteHandler(handler func([]byte) (int, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onWrite = handler
}

// SetCloseHandler sets a custom close handler
func (m *MockNetworkConnection) SetCloseHandler(handler func() error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onClose = handler
}

// SimulateError sets an error to be returned by Read/Write operations
func (m *MockNetworkConnection) SimulateError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.simulateError = err
}

// SetBlockingBehavior sets how long Read/Write operations should block
func (m *MockNetworkConnection) SetBlockingBehavior(readBlock, writeBlock time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readBlockTime = readBlock
	m.writeBlockTime = writeBlock
}

// GetStats returns connection statistics
func (m *MockNetworkConnection) GetStats() (bytesRead, bytesWritten int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bytesRead, m.bytesWritten
}

// IsClosed returns whether the connection is closed
func (m *MockNetworkConnection) IsClosed() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.closed
}

// MockListener provides a mock network listener for testing
type MockListener struct {
	t               *testing.T
	mu              sync.Mutex
	addr            net.Addr
	closed          bool
	acceptQueue     chan net.Conn
	acceptDelay     time.Duration
	acceptError     error
	connectionCount int64
}

// NewMockListener creates a new mock listener
func NewMockListener(t *testing.T, addr net.Addr) *MockListener {
	return &MockListener{
		t:           t,
		addr:        addr,
		acceptQueue: make(chan net.Conn, 10),
	}
}

// Accept implements the net.Listener interface
func (m *MockListener) Accept() (net.Conn, error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, fmt.Errorf("listener closed")
	}

	if m.acceptError != nil {
		err := m.acceptError
		m.mu.Unlock()
		return nil, err
	}

	delay := m.acceptDelay
	m.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	select {
	case conn := <-m.acceptQueue:
		m.mu.Lock()
		m.connectionCount++
		m.mu.Unlock()
		m.t.Logf("MockListener: Accepted connection %d", m.connectionCount)
		return conn, nil
	case <-time.After(GlobalTimeouts.Network):
		return nil, fmt.Errorf("accept timeout")
	}
}

// Close implements the net.Listener interface
func (m *MockListener) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true
	close(m.acceptQueue)
	m.t.Log("MockListener: Listener closed")
	return nil
}

// Addr implements the net.Listener interface
func (m *MockListener) Addr() net.Addr {
	return m.addr
}

// Mock-specific methods

// AddConnection adds a connection to the accept queue
func (m *MockListener) AddConnection(conn net.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.closed {
		select {
		case m.acceptQueue <- conn:
		default:
			m.t.Log("MockListener: Accept queue full, dropping connection")
		}
	}
}

// SetAcceptDelay sets a delay for Accept operations
func (m *MockListener) SetAcceptDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acceptDelay = delay
}

// SetAcceptError sets an error to be returned by Accept
func (m *MockListener) SetAcceptError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acceptError = err
}

// GetConnectionCount returns the number of accepted connections
func (m *MockListener) GetConnectionCount() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connectionCount
}

// MockDialer provides a mock dialer for testing
type MockDialer struct {
	t           *testing.T
	mu          sync.Mutex
	connections map[string]net.Conn
	dialDelay   time.Duration
	dialError   error
	onDial      func(network, address string) (net.Conn, error)
}

// NewMockDialer creates a new mock dialer
func NewMockDialer(t *testing.T) *MockDialer {
	return &MockDialer{
		t:           t,
		connections: make(map[string]net.Conn),
	}
}

// Dial dials a mock connection
func (m *MockDialer) Dial(network, address string) (net.Conn, error) {
	return m.DialContext(context.Background(), network, address)
}

// DialContext dials a mock connection with context
func (m *MockDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.dialDelay > 0 {
		select {
		case <-time.After(m.dialDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.dialError != nil {
		return nil, m.dialError
	}

	if m.onDial != nil {
		return m.onDial(network, address)
	}

	key := network + ":" + address
	if conn, exists := m.connections[key]; exists {
		return conn, nil
	}

	// Create a new mock connection
	localAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}

	conn := NewMockNetworkConnection(m.t, localAddr, remoteAddr)
	m.connections[key] = conn

	m.t.Logf("MockDialer: Dialed %s %s", network, address)
	return conn, nil
}

// SetDialDelay sets a delay for dial operations
func (m *MockDialer) SetDialDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dialDelay = delay
}

// SetDialError sets an error to be returned by dial operations
func (m *MockDialer) SetDialError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dialError = err
}

// SetDialHandler sets a custom dial handler
func (m *MockDialer) SetDialHandler(handler func(network, address string) (net.Conn, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onDial = handler
}

// GetConnection returns a connection for a specific address
func (m *MockDialer) GetConnection(network, address string) net.Conn {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := network + ":" + address
	return m.connections[key]
}

// NetworkTestSuite provides comprehensive network testing utilities
type NetworkTestSuite struct {
	t         *testing.T
	listeners map[string]*MockListener
	dialers   map[string]*MockDialer
	conns     map[string]*MockNetworkConnection
	cleanup   *CleanupHelper
}

// NewNetworkTestSuite creates a new network test suite
func NewNetworkTestSuite(t *testing.T) *NetworkTestSuite {
	return &NetworkTestSuite{
		t:         t,
		listeners: make(map[string]*MockListener),
		dialers:   make(map[string]*MockDialer),
		conns:     make(map[string]*MockNetworkConnection),
		cleanup:   NewCleanupHelper(t),
	}
}

// CreateListener creates a named mock listener
func (nts *NetworkTestSuite) CreateListener(name string, addr net.Addr) *MockListener {
	listener := NewMockListener(nts.t, addr)
	nts.listeners[name] = listener

	nts.cleanup.Add(func() {
		listener.Close()
	})

	return listener
}

// CreateDialer creates a named mock dialer
func (nts *NetworkTestSuite) CreateDialer(name string) *MockDialer {
	dialer := NewMockDialer(nts.t)
	nts.dialers[name] = dialer
	return dialer
}

// CreateConnection creates a named mock connection
func (nts *NetworkTestSuite) CreateConnection(name string, localAddr, remoteAddr net.Addr) *MockNetworkConnection {
	conn := NewMockNetworkConnection(nts.t, localAddr, remoteAddr)
	nts.conns[name] = conn

	nts.cleanup.Add(func() {
		conn.Close()
	})

	return conn
}

// GetListener returns a named listener
func (nts *NetworkTestSuite) GetListener(name string) *MockListener {
	return nts.listeners[name]
}

// GetDialer returns a named dialer
func (nts *NetworkTestSuite) GetDialer(name string) *MockDialer {
	return nts.dialers[name]
}

// GetConnection returns a named connection
func (nts *NetworkTestSuite) GetConnection(name string) *MockNetworkConnection {
	return nts.conns[name]
}

// SimulateNetworkPartition simulates a network partition between connections
func (nts *NetworkTestSuite) SimulateNetworkPartition(connNames ...string) {
	for _, name := range connNames {
		if conn := nts.conns[name]; conn != nil {
			conn.SimulateError(fmt.Errorf("network partition"))
		}
	}
}

// RestoreNetworkConnectivity restores network connectivity for connections
func (nts *NetworkTestSuite) RestoreNetworkConnectivity(connNames ...string) {
	for _, name := range connNames {
		if conn := nts.conns[name]; conn != nil {
			conn.SimulateError(nil)
		}
	}
}

// MockPacketConn provides a mock packet connection for UDP testing
type MockPacketConn struct {
	t             *testing.T
	mu            sync.RWMutex
	localAddr     net.Addr
	packets       []mockPacket
	closed        bool
	readDeadline  time.Time
	writeDeadline time.Time
	onReadFrom    func([]byte) (int, net.Addr, error)
	onWriteTo     func([]byte, net.Addr) (int, error)
}

type mockPacket struct {
	data []byte
	addr net.Addr
	time time.Time
}

// NewMockPacketConn creates a new mock packet connection
func NewMockPacketConn(t *testing.T, localAddr net.Addr) *MockPacketConn {
	return &MockPacketConn{
		t:         t,
		localAddr: localAddr,
		packets:   make([]mockPacket, 0),
	}
}

// ReadFrom implements the net.PacketConn interface
func (m *MockPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return 0, nil, io.EOF
	}

	if m.onReadFrom != nil {
		return m.onReadFrom(p)
	}

	if len(m.packets) == 0 {
		return 0, nil, fmt.Errorf("no packets available")
	}

	packet := m.packets[0]
	m.packets = m.packets[1:]

	n := copy(p, packet.data)
	m.t.Logf("MockPacketConn: Read %d bytes from %s", n, packet.addr)

	return n, packet.addr, nil
}

// WriteTo implements the net.PacketConn interface
func (m *MockPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return 0, fmt.Errorf("connection closed")
	}

	if m.onWriteTo != nil {
		return m.onWriteTo(p, addr)
	}

	m.t.Logf("MockPacketConn: Wrote %d bytes to %s", len(p), addr)
	return len(p), nil
}

// Close implements the net.PacketConn interface
func (m *MockPacketConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true
	m.t.Log("MockPacketConn: Connection closed")
	return nil
}

// LocalAddr implements the net.PacketConn interface
func (m *MockPacketConn) LocalAddr() net.Addr {
	return m.localAddr
}

// SetDeadline implements the net.PacketConn interface
func (m *MockPacketConn) SetDeadline(t time.Time) error {
	m.SetReadDeadline(t)
	m.SetWriteDeadline(t)
	return nil
}

// SetReadDeadline implements the net.PacketConn interface
func (m *MockPacketConn) SetReadDeadline(t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readDeadline = t
	return nil
}

// SetWriteDeadline implements the net.PacketConn interface
func (m *MockPacketConn) SetWriteDeadline(t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeDeadline = t
	return nil
}

// AddPacket adds a packet to be returned by ReadFrom
func (m *MockPacketConn) AddPacket(data []byte, addr net.Addr) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.packets = append(m.packets, mockPacket{
		data: data,
		addr: addr,
		time: time.Now(),
	})
}
