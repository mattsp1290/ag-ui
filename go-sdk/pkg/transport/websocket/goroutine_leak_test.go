package websocket

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// GoroutineLeakDetector helps detect goroutine leaks in tests
type GoroutineLeakDetector struct {
	t                testing.TB
	startGoroutines  int
	startStack       string
	tolerance        int
	excludePatterns  []string
	maxWaitTime      time.Duration
	checkInterval    time.Duration
}

// NewGoroutineLeakDetector creates a new leak detector
func NewGoroutineLeakDetector(t testing.TB) *GoroutineLeakDetector {
	detector := &GoroutineLeakDetector{
		t:               t,
		tolerance:       5, // Allow up to 5 goroutines growth (for test framework overhead)
		maxWaitTime:     5 * time.Second, // Maximum time to wait for goroutines to clean up
		checkInterval:   100 * time.Millisecond, // How often to check goroutine count
		excludePatterns: []string{
			"testing.(*T)",
			"runtime.goexit",
			"created by runtime",
			"created by net/http",
			"database/sql",
			"go.uber.org/zap",
			"context.WithCancel",
			"time.NewTicker",
		},
	}
	detector.snapshot()
	return detector
}

// snapshot captures current goroutine state
func (d *GoroutineLeakDetector) snapshot() {
	d.startGoroutines = runtime.NumGoroutine()
	d.startStack = getGoroutineStack()
}

// Check verifies no goroutines leaked with retry logic
func (d *GoroutineLeakDetector) Check() {
	// Wait for goroutines to clean up with periodic checks
	timeout := time.After(d.maxWaitTime)
	ticker := time.NewTicker(d.checkInterval)
	defer ticker.Stop()
	
	var endGoroutines int
	var leaked int
	
	for {
		select {
		case <-timeout:
			// Timeout reached, perform final check
			endGoroutines = runtime.NumGoroutine()
			leaked = endGoroutines - d.startGoroutines
			if leaked > d.tolerance {
				d.reportLeak(endGoroutines, leaked)
			}
			return
		case <-ticker.C:
			// Force garbage collection to help clean up
			runtime.GC()
			runtime.GC() // Run GC twice to ensure finalization
			
			endGoroutines = runtime.NumGoroutine()
			leaked = endGoroutines - d.startGoroutines
			
			// If we're within tolerance, we're good
			if leaked <= d.tolerance {
				d.t.Logf("Goroutine cleanup successful: started=%d, ended=%d, leaked=%d (within tolerance %d)",
					d.startGoroutines, endGoroutines, leaked, d.tolerance)
				return
			}
			
			// Continue waiting if we still have time
		}
	}
}

// reportLeak reports the goroutine leak with detailed information
func (d *GoroutineLeakDetector) reportLeak(endGoroutines, leaked int) {
	endStack := getGoroutineStack()
	d.t.Errorf("Goroutine leak detected: %d goroutines leaked (started with %d, ended with %d)",
		leaked, d.startGoroutines, endGoroutines)
	
	d.t.Logf("Start stack:\n%s", d.startStack)
	d.t.Logf("End stack:\n%s", endStack)
	
	// Try to identify the leaked goroutines
	d.identifyLeakedGoroutines(d.startStack, endStack)
	
	d.t.FailNow()
}

// identifyLeakedGoroutines tries to identify which goroutines are leaked
func (d *GoroutineLeakDetector) identifyLeakedGoroutines(startStack, endStack string) {
	startGoroutines := parseGoroutineStacks(startStack)
	endGoroutines := parseGoroutineStacks(endStack)
	
	d.t.Log("Potentially leaked goroutines:")
	for id, stack := range endGoroutines {
		if _, existed := startGoroutines[id]; !existed {
			excluded := false
			for _, pattern := range d.excludePatterns {
				if strings.Contains(stack, pattern) {
					excluded = true
					break
				}
			}
			if !excluded {
				d.t.Logf("New goroutine %s:\n%s", id, stack)
			}
		}
	}
}

// getGoroutineStack returns current goroutine stack traces
func getGoroutineStack() string {
	buf := make([]byte, 1<<20) // 1MB buffer
	n := runtime.Stack(buf, true)
	return string(buf[:n])
}

// WithTestTimeout wraps a test function with a timeout to prevent hanging
func WithTestTimeout(t testing.TB, timeout time.Duration, testFunc func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		testFunc()
	}()
	
	select {
	case <-done:
		// Test completed successfully
		return
	case <-time.After(timeout):
		t.Fatalf("Test timed out after %v - possible hang detected", timeout)
	}
}

// parseGoroutineStacks parses stack trace into individual goroutines
func parseGoroutineStacks(stack string) map[string]string {
	goroutines := make(map[string]string)
	lines := strings.Split(stack, "\n")
	
	var currentID string
	var currentStack strings.Builder
	
	for _, line := range lines {
		if strings.HasPrefix(line, "goroutine ") {
			if currentID != "" {
				goroutines[currentID] = currentStack.String()
			}
			currentID = strings.TrimSpace(strings.Split(line, "[")[0])
			currentStack.Reset()
			currentStack.WriteString(line + "\n")
		} else if currentID != "" {
			currentStack.WriteString(line + "\n")
		}
	}
	
	if currentID != "" {
		goroutines[currentID] = currentStack.String()
	}
	
	return goroutines
}

// VerifyNoLeaks runs a test function and verifies no goroutines leak
func VerifyNoLeaks(t testing.TB, testFunc func()) {
	detector := NewGoroutineLeakDetector(t)
	defer detector.Check()
	
	testFunc()
}

// TestGoroutineLeakInLoadTest verifies that goroutines are properly cleaned up
func TestGoroutineLeakInLoadTest(t *testing.T) {
	WithTestTimeout(t, 30*time.Second, func() {
	// Get baseline goroutine count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineGoroutines := runtime.NumGoroutine()
	t.Logf("Baseline goroutines: %d", baselineGoroutines)

	// Run multiple iterations to ensure consistent cleanup
	for i := 0; i < 3; i++ {
		t.Run(fmt.Sprintf("Iteration_%d", i), func(t *testing.T) {
			server := NewLoadTestServer(t)
			defer server.Close()

			config := DefaultConnectionConfig()
			config.URL = server.URL()
			config.Logger = zap.NewNop()
			config.MaxReconnectAttempts = 3
			config.InitialReconnectDelay = 50 * time.Millisecond

			// Create and test multiple connections
			const numConnections = 10
			var wg sync.WaitGroup
			connections := make([]*Connection, numConnections)

			// Create connections
			for j := 0; j < numConnections; j++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					
					conn, err := NewConnection(config)
					require.NoError(t, err)
					connections[idx] = conn

					ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
					defer cancel()

					err = conn.Connect(ctx)
					require.NoError(t, err)

					// Start auto-reconnect
					conn.StartAutoReconnect(ctx)

					// Send some messages
					for k := 0; k < 5; k++ {
						msg := []byte(fmt.Sprintf("test message %d-%d", idx, k))
						_ = conn.SendMessage(ctx, msg)
						time.Sleep(10 * time.Millisecond)
					}
				}(j)
			}

			wg.Wait()

			// Verify all connections are established
			activeCount := 0
			for _, conn := range connections {
				if conn != nil && conn.IsConnected() {
					activeCount++
				}
			}
			t.Logf("Active connections: %d/%d", activeCount, numConnections)

			// Close all connections
			var closeWg sync.WaitGroup
			for _, conn := range connections {
				if conn != nil {
					closeWg.Add(1)
					go func(c *Connection) {
						defer closeWg.Done()
						c.Close()
					}(conn)
				}
			}

			closeWg.Wait()

			// Wait for goroutines to clean up - reduced wait time
			time.Sleep(200 * time.Millisecond)
			runtime.GC()
			time.Sleep(50 * time.Millisecond)

			currentGoroutines := runtime.NumGoroutine()
			t.Logf("Goroutines after cleanup: %d", currentGoroutines)

			// Allow some tolerance for test framework goroutines
			tolerance := 10
			assert.LessOrEqual(t, currentGoroutines, baselineGoroutines+tolerance,
				"Goroutine leak detected: baseline=%d, current=%d", 
				baselineGoroutines, currentGoroutines)
		})
	}
	})
}

// TestConnectionWritePumpGoroutineLeak specifically tests the writePump goroutine leak scenario
func TestConnectionWritePumpGoroutineLeak(t *testing.T) {
	WithTestTimeout(t, 20*time.Second, func() {
	server := NewLoadTestServer(t)
	defer server.Close()

	config := DefaultConnectionConfig()
	config.URL = server.URL()
	config.Logger = zap.NewNop()

	// Track goroutines before test
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	beforeGoroutines := runtime.NumGoroutine()

	// Create connection with shorter context to avoid long waits
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	conn, err := NewConnection(config)
	require.NoError(t, err)

	err = conn.Connect(ctx)
	require.NoError(t, err)

	// Start sending messages in a goroutine - reduce iterations and sleep for faster execution
	messageDone := make(chan struct{})
	go func() {
		defer close(messageDone)
		for i := 0; i < 20; i++ { // Reduced from 100 to 20 iterations
			select {
			case <-ctx.Done():
				return
			default:
				msg := []byte(fmt.Sprintf("test message %d", i))
				_ = conn.SendMessage(ctx, msg)
				time.Sleep(5 * time.Millisecond) // Reduced from 10ms to 5ms
			}
		}
	}()

	// Wait for either context timeout or message sending to complete
	select {
	case <-ctx.Done():
		t.Log("Context timed out as expected")
	case <-messageDone:
		t.Log("Message sending completed before timeout")
	}

	// Close connection
	err = conn.Close()
	assert.NoError(t, err)

	// Reduced cleanup wait time - Connection.Close() already waits up to 2s for goroutines
	time.Sleep(200 * time.Millisecond) // Reduced from 1 second to 200ms
	runtime.GC()
	time.Sleep(50 * time.Millisecond) // Reduced from 100ms to 50ms

	afterGoroutines := runtime.NumGoroutine()
	t.Logf("Goroutines: before=%d, after=%d", beforeGoroutines, afterGoroutines)

	// Check for leaks
	assert.LessOrEqual(t, afterGoroutines, beforeGoroutines+5,
		"Goroutine leak detected in writePump test")
	})
}

// TestLoadTestServerGoroutineLeak tests the load test server cleanup
func TestLoadTestServerGoroutineLeak(t *testing.T) {
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	beforeGoroutines := runtime.NumGoroutine()

	// Create and use server
	server := NewLoadTestServer(t)
	
	// Create multiple connections to the server
	config := DefaultConnectionConfig()
	config.URL = server.URL()
	config.Logger = zap.NewNop()

	connections := make([]*Connection, 5)
	for i := 0; i < 5; i++ {
		conn, err := NewConnection(config)
		require.NoError(t, err)
		connections[i] = conn

		ctx := context.Background()
		err = conn.Connect(ctx)
		require.NoError(t, err)

		// Send some messages
		for j := 0; j < 10; j++ {
			msg := []byte(fmt.Sprintf("test %d-%d", i, j))
			_ = conn.SendMessage(ctx, msg)
		}
	}

	// Close connections first
	for _, conn := range connections {
		conn.Close()
	}

	// Then close server
	server.Close()

	// Wait for cleanup - reduced wait time
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	afterGoroutines := runtime.NumGoroutine()
	t.Logf("Server goroutines: before=%d, after=%d", beforeGoroutines, afterGoroutines)

	assert.LessOrEqual(t, afterGoroutines, beforeGoroutines+5,
		"Goroutine leak detected in load test server")
}


// TestMultipleConnectionsGoroutineCleanup tests cleanup of multiple connections
func TestMultipleConnectionsGoroutineCleanup(t *testing.T) {
	WithTestTimeout(t, 20*time.Second, func() {
		detector := NewGoroutineLeakDetector(t)
		defer detector.Check()
		
		server := NewLoadTestServer(t)
		defer server.Close()

		const numConnections = 5
		connections := make([]*Connection, numConnections)

		// Create multiple connections
		for i := 0; i < numConnections; i++ {
			config := DefaultConnectionConfig()
			config.URL = server.URL()
			config.Logger = zap.NewNop()
			config.PingPeriod = 50 * time.Millisecond
			config.PongWait = 100 * time.Millisecond

			conn, err := NewConnection(config)
			require.NoError(t, err)
			connections[i] = conn

			ctx := context.Background()
			err = conn.Connect(ctx)
			require.NoError(t, err)

			// Start auto-reconnect for some connections
			if i%2 == 0 {
				conn.StartAutoReconnect(ctx)
			}

			// Send some messages
			for j := 0; j < 5; j++ {
				msg := []byte(fmt.Sprintf("test message %d-%d", i, j))
				_ = conn.SendMessage(ctx, msg)
				time.Sleep(10 * time.Millisecond)
			}
		}

		// Close all connections
		for _, conn := range connections {
			err := conn.Close()
			require.NoError(t, err)
		}

		// Allow extra time for cleanup
		time.Sleep(500 * time.Millisecond)
	})
}

// TestConcurrentConnectionClose tests goroutine cleanup with concurrent operations
func TestConcurrentConnectionClose(t *testing.T) {
	WithTestTimeout(t, 15*time.Second, func() {
	server := NewLoadTestServer(t)
	defer server.Close()

	config := DefaultConnectionConfig()
	config.URL = server.URL()
	config.Logger = zap.NewNop()

	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	beforeGoroutines := runtime.NumGoroutine()

	// Create connection
	conn, err := NewConnection(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = conn.Connect(ctx)
	require.NoError(t, err)

	// Start multiple concurrent operations
	var wg sync.WaitGroup

	// Message sender
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			// Check if connection is closed before sending
			if conn.State() == StateClosed || conn.State() == StateClosing {
				return
			}
			msg := []byte(fmt.Sprintf("concurrent test %d", i))
			if err := conn.SendMessage(ctx, msg); err != nil {
				return
			}
			// Short sleep with cancellation check
			select {
			case <-time.After(5 * time.Millisecond):
			case <-ctx.Done():
				return
			}
		}
	}()

	// Auto-reconnect
	conn.StartAutoReconnect(ctx)

	// Let operations run
	time.Sleep(200 * time.Millisecond)

	// Close connection while operations are running
	err = conn.Close()
	assert.NoError(t, err)

	// Wait for any remaining operations with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good, operations completed
	case <-time.After(2 * time.Second):
		t.Log("Warning: Some operations did not complete in time")
	}

	// Check for goroutine leaks - reduced wait time
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	afterGoroutines := runtime.NumGoroutine()
	t.Logf("Concurrent test goroutines: before=%d, after=%d", beforeGoroutines, afterGoroutines)

	assert.LessOrEqual(t, afterGoroutines, beforeGoroutines+5,
		"Goroutine leak detected in concurrent operations test")
	})
}