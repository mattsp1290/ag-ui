package websocket

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// Global test sequencer to prevent resource conflicts when running all tests together
var (
	testSequenceMutex sync.Mutex
	heavyTestSemaphore = make(chan struct{}, 2) // Allow max 2 heavy tests concurrently
	lastTestTime      = time.Now()
	testGapDuration   = 50 * time.Millisecond // Minimum gap between tests
)

// TestCategory defines the resource intensity of a test
type TestCategory int

const (
	LightTest TestCategory = iota // Basic unit tests
	MediumTest                    // Integration tests  
	HeavyTest                     // Load/performance tests
)

// SequencedTestRunner manages test execution to prevent resource conflicts
type SequencedTestRunner struct {
	t        *testing.T
	category TestCategory
	timeout  time.Duration
	helper   *MinimalTestHelper
	logger   *zap.Logger
}

// NewSequencedTestRunner creates a test runner with proper sequencing
func NewSequencedTestRunner(t *testing.T, category TestCategory, timeout time.Duration) *SequencedTestRunner {
	return &SequencedTestRunner{
		t:        t,
		category: category,
		timeout:  timeout,
		helper:   NewMinimalTestHelper(t),
		logger:   zaptest.NewLogger(t),
	}
}

// Run executes the test with proper sequencing and resource management
func (r *SequencedTestRunner) Run(testFunc func(*MinimalTestHelper)) {
	r.acquireTestSlot()
	defer r.releaseTestSlot()
	
	// Enforce minimum gap between tests to prevent resource conflicts
	r.enforceTestGap()
	
	// Track initial resource state
	initialGoroutines := runtime.NumGoroutine()
	
	// Run test with timeout
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	
	done := make(chan struct{})
	var testErr error
	
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				r.logger.Error("Test panicked", zap.Any("panic", rec))
				testErr = fmt.Errorf("test panicked: %v", rec)
			}
			close(done)
		}()
		
		testFunc(r.helper)
	}()
	
	select {
	case <-done:
		if testErr != nil {
			r.t.Fatal(testErr)
		}
	case <-ctx.Done():
		r.logger.Error("Test timed out", 
			zap.Duration("timeout", r.timeout),
			zap.String("category", r.categoryString()))
		r.t.Fatalf("Test timed out after %v", r.timeout)
	}
	
	// Verify no significant goroutine leaks
	r.verifyResourceCleanup(initialGoroutines)
}

func (r *SequencedTestRunner) acquireTestSlot() {
	switch r.category {
	case HeavyTest:
		// Heavy tests are limited and sequential
		heavyTestSemaphore <- struct{}{}
		r.logger.Debug("Acquired heavy test slot")
	case MediumTest:
		// Medium tests get a brief delay to avoid conflicts
		time.Sleep(25 * time.Millisecond)
	case LightTest:
		// Light tests can run more freely
		// No special slot management needed
	}
}

func (r *SequencedTestRunner) releaseTestSlot() {
	switch r.category {
	case HeavyTest:
		<-heavyTestSemaphore
		r.logger.Debug("Released heavy test slot")
		
		// Force cleanup after heavy tests
		runtime.GC()
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
	case MediumTest:
		// Brief cleanup after medium tests
		runtime.GC()
		time.Sleep(25 * time.Millisecond)
	}
}

func (r *SequencedTestRunner) enforceTestGap() {
	testSequenceMutex.Lock()
	defer testSequenceMutex.Unlock()
	
	elapsed := time.Since(lastTestTime)
	if elapsed < testGapDuration {
		waitTime := testGapDuration - elapsed
		time.Sleep(waitTime)
	}
	lastTestTime = time.Now()
}

func (r *SequencedTestRunner) verifyResourceCleanup(initialGoroutines int) {
	// Give cleanup time to complete
	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	time.Sleep(25 * time.Millisecond)
	
	finalGoroutines := runtime.NumGoroutine()
	goroutineDiff := finalGoroutines - initialGoroutines
	
	// Set tolerance based on test category
	tolerance := 5
	switch r.category {
	case HeavyTest:
		tolerance = 15
	case MediumTest:
		tolerance = 10
	}
	
	if goroutineDiff > tolerance {
		r.logger.Warn("Potential goroutine leak detected",
			zap.Int("initial", initialGoroutines),
			zap.Int("final", finalGoroutines),
			zap.Int("diff", goroutineDiff),
			zap.Int("tolerance", tolerance),
			zap.String("category", r.categoryString()))
	}
}

func (r *SequencedTestRunner) categoryString() string {
	switch r.category {
	case LightTest:
		return "light"
	case MediumTest:
		return "medium"
	case HeavyTest:
		return "heavy"
	default:
		return "unknown"
	}
}

// MinimalTestHelper provides the most basic resource management for tests
type MinimalTestHelper struct {
	t       *testing.T
	servers []*MinimalTestServer
	conns   []*Connection
	mu      sync.Mutex
}

// NewMinimalTestHelper creates a minimal test helper
func NewMinimalTestHelper(t *testing.T) *MinimalTestHelper {
	h := &MinimalTestHelper{t: t}
	t.Cleanup(h.Cleanup)
	return h
}

// CreateServer creates a minimal test server
func (h *MinimalTestHelper) CreateServer() *MinimalTestServer {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	server := NewMinimalTestServer(h.t)
	h.servers = append(h.servers, server)
	return server
}

// CreateConnection creates a minimal connection
func (h *MinimalTestHelper) CreateConnection(url string) *Connection {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	config := &ConnectionConfig{
		URL:                        url,
		ReadTimeout:               2 * time.Second,
		WriteTimeout:              2 * time.Second,
		PingPeriod:                5 * time.Second,
		PongWait:                  3 * time.Second,
		DialTimeout:               3 * time.Second,
		HandshakeTimeout:          3 * time.Second,
		MaxReconnectAttempts:      2,
		InitialReconnectDelay:     50 * time.Millisecond,
		MaxReconnectDelay:         2 * time.Second,
		ReconnectBackoffMultiplier: 1.5,
		MaxMessageSize:            64 * 1024, // 64KB - smaller than default
		Logger:                    zaptest.NewLogger(h.t),
	}
	
	conn, err := NewConnection(config)
	if err != nil {
		h.t.Fatalf("Failed to create connection: %v", err)
	}
	
	h.conns = append(h.conns, conn)
	return conn
}

// Cleanup performs fast cleanup with aggressive timeouts
func (h *MinimalTestHelper) Cleanup() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Close connections with 1s total timeout
	if len(h.conns) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		
		var wg sync.WaitGroup
		for _, conn := range h.conns {
			if conn != nil {
				wg.Add(1)
				go func(c *Connection) {
					defer wg.Done()
					select {
					case <-ctx.Done():
						return
					default:
						c.Close()
					}
				}(conn)
			}
		}
		
		// Wait for cleanup or timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()
		
		select {
		case <-done:
		case <-ctx.Done():
			// Timeout - connections will be cleaned up by GC
		}
	}
	
	// Close servers immediately
	for _, server := range h.servers {
		if server != nil {
			server.Close()
		}
	}
	
	// Single GC pass
	runtime.GC()
}

// MinimalTestServer provides the simplest possible WebSocket test server
type MinimalTestServer struct {
	server *httptest.Server
	closed chan struct{}
}

// NewMinimalTestServer creates a minimal test server
func NewMinimalTestServer(t *testing.T) *MinimalTestServer {
	s := &MinimalTestServer{
		closed: make(chan struct{}),
	}
	
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	s.server = httptest.NewServer(mux)
	
	return s
}

func (s *MinimalTestServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	select {
	case <-s.closed:
		http.Error(w, "Server closed", http.StatusServiceUnavailable)
		return
	default:
	}
	
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	
	// Simple echo server with timeout
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	
	for {
		select {
		case <-s.closed:
			return
		default:
		}
		
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			return
		}
		
		if err := conn.WriteMessage(messageType, message); err != nil {
			return
		}
	}
}

// URL returns the server's WebSocket URL
func (s *MinimalTestServer) URL() string {
	return "ws" + s.server.URL[4:] + "/ws"
}

// Close closes the server
func (s *MinimalTestServer) Close() {
	select {
	case <-s.closed:
		return // Already closed
	default:
		close(s.closed)
	}
	s.server.Close()
}

// RunFastTest runs a light test with minimal overhead
func RunFastTest(t *testing.T, testFunc func(*MinimalTestHelper)) {
	runner := NewSequencedTestRunner(t, LightTest, 5*time.Second)
	runner.Run(testFunc)
}

// RunMediumTest runs a medium-complexity test with moderate resources
func RunMediumTest(t *testing.T, testFunc func(*MinimalTestHelper)) {
	runner := NewSequencedTestRunner(t, MediumTest, 10*time.Second)
	runner.Run(testFunc)
}

// RunHeavyTest runs a resource-intensive test with proper sequencing
func RunHeavyTest(t *testing.T, testFunc func(*MinimalTestHelper)) {
	runner := NewSequencedTestRunner(t, HeavyTest, 30*time.Second)
	runner.Run(testFunc)
}