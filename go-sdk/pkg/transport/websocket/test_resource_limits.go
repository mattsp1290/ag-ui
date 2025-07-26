package websocket

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"
)

// ResourceLimitedTest manages test resource consumption to prevent system exhaustion
type ResourceLimitedTest struct {
	maxGoroutines int
	maxMemoryMB   int64
	semaphore     chan struct{}
	mu            sync.Mutex
	activeTests   int
}

// NewResourceLimitedTest creates a resource-limited test manager
func NewResourceLimitedTest(maxGoroutines int, maxMemoryMB int64) *ResourceLimitedTest {
	return &ResourceLimitedTest{
		maxGoroutines: maxGoroutines,
		maxMemoryMB:   maxMemoryMB,
		semaphore:     make(chan struct{}, maxGoroutines),
	}
}

// DefaultResourceLimits returns reasonable limits for websocket tests
func DefaultResourceLimits() *ResourceLimitedTest {
	return NewResourceLimitedTest(
		50,  // Max 50 concurrent goroutines
		100, // Max 100MB memory usage
	)
}

// RunWithLimits executes a test function with resource limits
func (r *ResourceLimitedTest) RunWithLimits(t *testing.T, name string, testFunc func(t *testing.T)) {
	t.Run(name, func(t *testing.T) {
		// Acquire semaphore slot
		select {
		case r.semaphore <- struct{}{}:
			defer func() { <-r.semaphore }()
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for test resource slot")
		}

		r.mu.Lock()
		r.activeTests++
		currentActive := r.activeTests
		r.mu.Unlock()

		defer func() {
			r.mu.Lock()
			r.activeTests--
			r.mu.Unlock()
		}()

		// Check system resources before starting
		initialGoroutines := runtime.NumGoroutine()
		var initialMemStats runtime.MemStats
		runtime.ReadMemStats(&initialMemStats)

		t.Logf("Starting test with %d active tests, %d goroutines, %d MB memory", 
			currentActive, initialGoroutines, initialMemStats.Alloc/(1024*1024))

		// Run test with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		done := make(chan struct{})
		go func() {
			defer close(done)
			testFunc(t)
		}()

		select {
		case <-done:
			// Test completed successfully
		case <-ctx.Done():
			t.Fatalf("Test %s timed out after 30s", name)
		}

		// Check for resource leaks
		runtime.GC()
		time.Sleep(100 * time.Millisecond)

		finalGoroutines := runtime.NumGoroutine()
		var finalMemStats runtime.MemStats
		runtime.ReadMemStats(&finalMemStats)

		goroutineLeak := finalGoroutines - initialGoroutines
		memoryLeakMB := int64(finalMemStats.Alloc-initialMemStats.Alloc) / (1024 * 1024)

		if goroutineLeak > 10 {
			t.Logf("WARNING: Potential goroutine leak detected: %d goroutines", goroutineLeak)
		}

		if memoryLeakMB > 10 {
			t.Logf("WARNING: Potential memory leak detected: %d MB", memoryLeakMB)
		}

		t.Logf("Test completed: %d goroutines (%+d), %d MB memory (%+d MB)", 
			finalGoroutines, goroutineLeak, 
			finalMemStats.Alloc/(1024*1024), memoryLeakMB)
	})
}

// ConnectionBudget manages connection creation to prevent resource exhaustion
type ConnectionBudget struct {
	maxConnections int
	activeConns    int32
	connCh         chan struct{}
}

// NewConnectionBudget creates a connection budget manager
func NewConnectionBudget(maxConnections int) *ConnectionBudget {
	return &ConnectionBudget{
		maxConnections: maxConnections,
		connCh:         make(chan struct{}, maxConnections),
	}
}

// AcquireConnection attempts to acquire a connection slot
func (c *ConnectionBudget) AcquireConnection(ctx context.Context) error {
	select {
	case c.connCh <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
		return context.DeadlineExceeded
	}
}

// ReleaseConnection releases a connection slot
func (c *ConnectionBudget) ReleaseConnection() {
	select {
	case <-c.connCh:
	default:
		// Channel already empty, ignore
	}
}

// GetActiveCount returns the number of active connections
func (c *ConnectionBudget) GetActiveCount() int {
	return len(c.connCh)
}

// TestConnectionManager helps manage connections in tests to prevent leaks
type TestConnectionManager struct {
	connections []*Connection
	transports  []*Transport
	servers     []*ReliableTestServer
	mu          sync.Mutex
}

// NewTestConnectionManager creates a new connection manager for tests
func NewTestConnectionManager() *TestConnectionManager {
	return &TestConnectionManager{}
}

// AddConnection adds a connection to be managed
func (m *TestConnectionManager) AddConnection(conn *Connection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connections = append(m.connections, conn)
}

// AddTransport adds a transport to be managed
func (m *TestConnectionManager) AddTransport(transport *Transport) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transports = append(m.transports, transport)
}

// AddServer adds a test server to be managed
func (m *TestConnectionManager) AddServer(server *ReliableTestServer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers = append(m.servers, server)
}

// CleanupAll closes all managed resources with timeout
func (m *TestConnectionManager) CleanupAll(t *testing.T) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close transports first (they manage connections)
	for _, transport := range m.transports {
		done := make(chan struct{})
		go func(tr *Transport) {
			defer close(done)
			if err := tr.Stop(); err != nil {
				t.Logf("Error stopping transport: %v", err)
			}
		}(transport)

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Log("WARNING: Transport stop timed out")
		}
	}

	// Close individual connections
	for _, conn := range m.connections {
		done := make(chan struct{})
		go func(c *Connection) {
			defer close(done)
			if err := c.Close(); err != nil {
				t.Logf("Error closing connection: %v", err)
			}
		}(conn)

		select {
		case <-done:
		case <-time.After(1 * time.Second):
			t.Log("WARNING: Connection close timed out")
		}
	}

	// Close test servers last
	for _, server := range m.servers {
		server.Close()
	}

	// Clear all references
	m.connections = nil
	m.transports = nil
	m.servers = nil

	// Force cleanup
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
}