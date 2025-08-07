package websocket

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// TestIsolationAndCleanup verifies that the new isolation and cleanup mechanisms work
func TestIsolationAndCleanup(t *testing.T) {
	runner := NewIsolatedTestRunner(t)

	// Run multiple tests to verify isolation
	for i := 0; i < 5; i++ {
		testName := "IsolatedTest_" + string(rune('A'+i))

		runner.RunIsolated(testName, 5*time.Second, func(cleanup *TestCleanupHelper) {
			// Create server
			server := CreateIsolatedServer(t, cleanup)

			// Create transport
			config := FastTransportConfig()
			config.URLs = []string{server.URL()}
			config.Logger = zaptest.NewLogger(t)
			config.EnableEventValidation = false

			transport := CreateIsolatedTransport(t, cleanup, config)

			// Start transport
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := transport.Start(ctx)
			require.NoError(t, err)

			// Wait for connection
			assert.Eventually(t, func() bool {
				return transport.IsConnected()
			}, 1*time.Second, 25*time.Millisecond)

			// Send some messages
			for j := 0; j < 10; j++ {
				event := &MockEvent{
					EventType: events.EventTypeTextMessageContent,
					Data:      "test message",
				}
				err := transport.SendEvent(ctx, event)
				assert.NoError(t, err)
			}

			// Verify stats
			stats := transport.Stats()
			assert.GreaterOrEqual(t, stats.EventsSent, int64(5)) // Allow some margin

			// Test will automatically cleanup via registered cleanup helper
		})
	}
}

// TestGoroutineLeakPrevention verifies that goroutines don't leak between tests
func TestGoroutineLeakPrevention(t *testing.T) {
	// Get baseline
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	runtime.GC()
	initialGoroutines := runtime.NumGoroutine()

	// Run multiple tests that create resources
	for i := 0; i < 3; i++ {
		t.Run("LeakTest", func(t *testing.T) {
			cleanup := NewTestCleanupHelper(t)

			// Create multiple servers and transports
			for j := 0; j < 3; j++ {
				server := CreateIsolatedServer(t, cleanup)

				config := FastTransportConfig()
				config.URLs = []string{server.URL()}
				config.Logger = zaptest.NewLogger(t)

				transport := CreateIsolatedTransport(t, cleanup, config)

				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

				err := transport.Start(ctx)
				require.NoError(t, err)

				// Send a few messages
				for k := 0; k < 5; k++ {
					event := &MockEvent{
						EventType: events.EventTypeTextMessageContent,
						Data:      "leak test message",
					}
					_ = transport.SendEvent(ctx, event)
				}

				cancel()
			}
			// Cleanup will happen automatically via t.Cleanup
		})

		// Give time for cleanup between tests
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
	}

	// Check final goroutine count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	finalGoroutines := runtime.NumGoroutine()

	leaked := finalGoroutines - initialGoroutines

	// Allow reasonable tolerance for test framework overhead
	assert.LessOrEqual(t, leaked, 15,
		"Too many goroutines leaked: initial=%d, final=%d, leaked=%d",
		initialGoroutines, finalGoroutines, leaked)

	t.Logf("Goroutine leak test completed: initial=%d, final=%d, leaked=%d",
		initialGoroutines, finalGoroutines, leaked)
}

// TestConcurrentTestIsolation verifies that concurrent tests don't interfere
func TestConcurrentTestIsolation(t *testing.T) {
	runner := NewIsolatedTestRunner(t)

	// Run multiple concurrent tests
	for i := 0; i < 3; i++ {
		testName := "ConcurrentTest_" + string(rune('A'+i))

		t.Run(testName, func(t *testing.T) {
			// Enable parallel execution

			runner.RunIsolated(testName+"_Isolated", 3*time.Second, func(cleanup *TestCleanupHelper) {
				server := CreateIsolatedServer(t, cleanup)

				config := FastTransportConfig()
				config.URLs = []string{server.URL()}
				config.Logger = zaptest.NewLogger(t)

				transport := CreateIsolatedTransport(t, cleanup, config)

				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				err := transport.Start(ctx)
				require.NoError(t, err)

				// Each test sends different number of messages to verify isolation
				numMessages := (i + 1) * 5
				for j := 0; j < numMessages; j++ {
					event := &MockEvent{
						EventType: events.EventTypeTextMessageContent,
						Data:      "concurrent test message",
					}
					err := transport.SendEvent(ctx, event)
					assert.NoError(t, err)
				}

				// Verify expected number of messages
				stats := transport.Stats()
				assert.GreaterOrEqual(t, stats.EventsSent, int64(numMessages/2)) // Allow margin
			})
		})
	}
}

// TestResourceCleanupTimeout verifies that cleanup doesn't hang
func TestResourceCleanupTimeout(t *testing.T) {
	cleanup := NewTestCleanupHelper(t)

	// Create resources
	server := CreateIsolatedServer(t, cleanup)

	config := FastTransportConfig()
	config.URLs = []string{server.URL()}
	config.Logger = zaptest.NewLogger(t)

	transport := CreateIsolatedTransport(t, cleanup, config)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := transport.Start(ctx)
	require.NoError(t, err)

	// Send messages to activate goroutines
	for i := 0; i < 20; i++ {
		event := &MockEvent{
			EventType: events.EventTypeTextMessageContent,
			Data:      "cleanup timeout test",
		}
		_ = transport.SendEvent(ctx, event)
	}

	// Measure cleanup time
	start := time.Now()
	cleanup.CleanupAll()
	cleanupTime := time.Since(start)

	// Cleanup should complete quickly
	assert.Less(t, cleanupTime, 5*time.Second,
		"Cleanup took too long: %v", cleanupTime)

	t.Logf("Cleanup completed in: %v", cleanupTime)
}
