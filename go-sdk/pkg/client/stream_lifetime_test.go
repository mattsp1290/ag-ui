package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStreamLifetimeProtection tests the core stream lifetime protection functionality
func TestStreamLifetimeProtection(t *testing.T) {
	tests := []struct {
		name                string
		maxStreamLifetime   time.Duration
		testDuration        time.Duration
		expectedTermination bool
		description         string
	}{
		{
			name:                "ShortLifetime_ShouldTerminate",
			maxStreamLifetime:   100 * time.Millisecond,
			testDuration:        200 * time.Millisecond,
			expectedTermination: true,
			description:         "Short lifetime should terminate stream",
		},
		{
			name:                "LongLifetime_ShouldNotTerminate",
			maxStreamLifetime:   500 * time.Millisecond,
			testDuration:        200 * time.Millisecond,
			expectedTermination: false,
			description:         "Long lifetime should not terminate stream",
		},
		{
			name:                "VeryShortLifetime_ShouldTerminateQuickly",
			maxStreamLifetime:   50 * time.Millisecond,
			testDuration:        100 * time.Millisecond,
			expectedTermination: true,
			description:         "Very short lifetime should terminate quickly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test server
			eventsSent := int32(0)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")

				flusher, ok := w.(http.Flusher)
				require.True(t, ok)

				for {
					select {
					case <-r.Context().Done():
						return
					default:
						eventData := fmt.Sprintf("data: test event %d\n\n", atomic.AddInt32(&eventsSent, 1))
						_, err := w.Write([]byte(eventData))
						if err != nil {
							return
						}
						flusher.Flush()
						time.Sleep(10 * time.Millisecond)
					}
				}
			}))
			defer server.Close()

			// Create SSE client config
			config := SSEClientConfig{
				URL:               server.URL,
				MaxStreamLifetime: tt.maxStreamLifetime,
				EventBufferSize:   100,
			}

			// Track termination and errors
			var terminated atomic.Bool
			var errorReceived atomic.Bool
			var terminationReason string
			var mu sync.Mutex

			config.OnError = func(err error) {
				mu.Lock()
				if strings.Contains(err.Error(), "stream lifetime exceeded") {
					terminated.Store(true)
					terminationReason = err.Error()
				}
				errorReceived.Store(true)
				mu.Unlock()
			}

			// Create and start client
			client, err := NewSSEClient(config)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), tt.testDuration*2)
			defer cancel()

			err = client.Connect(ctx)
			require.NoError(t, err)

			// Wait for test duration
			time.Sleep(tt.testDuration)

			// Check results
			if tt.expectedTermination {
				assert.True(t, terminated.Load(), "Stream should have been terminated due to lifetime expiry")
				assert.True(t, errorReceived.Load(), "Error callback should have been triggered")
				mu.Lock()
				assert.Contains(t, terminationReason, "stream lifetime exceeded", "Termination reason should mention lifetime")
				mu.Unlock()
			} else {
				assert.False(t, terminated.Load(), "Stream should not have been terminated yet")
			}

			// Cleanup
			client.Close()
			assert.True(t, atomic.LoadInt32(&eventsSent) > 0, "Should have received some events")
		})
	}
}

// TestStreamLifetimeGoroutineCleanup verifies that goroutines are properly cleaned up after lifetime expiration
func TestStreamLifetimeGoroutineCleanup(t *testing.T) {
	// Record initial goroutine count
	runtime.GC()
	initialGoroutines := runtime.NumGoroutine()

	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		for i := 0; i < 100; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
				eventData := fmt.Sprintf("data: test event %d\n\n", i)
				w.Write([]byte(eventData))
				flusher.Flush()
				time.Sleep(5 * time.Millisecond)
			}
		}
	}))
	defer server.Close()

	// Create multiple clients with short lifetimes
	numClients := 5
	maxStreamLifetime := 50 * time.Millisecond
	clients := make([]*SSEClient, numClients)
	terminatedCount := int32(0)

	for i := 0; i < numClients; i++ {
		config := SSEClientConfig{
			URL:               server.URL,
			MaxStreamLifetime: maxStreamLifetime,
			EventBufferSize:   50,
			OnError: func(err error) {
				if strings.Contains(err.Error(), "stream lifetime exceeded") {
					atomic.AddInt32(&terminatedCount, 1)
				}
			},
		}

		client, err := NewSSEClient(config)
		require.NoError(t, err)
		clients[i] = client

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		err = client.Connect(ctx)
		require.NoError(t, err)
	}

	// Wait for all streams to be terminated by lifetime expiry
	timeout := time.After(200 * time.Millisecond)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

waitLoop:
	for {
		select {
		case <-timeout:
			break waitLoop
		case <-ticker.C:
			if atomic.LoadInt32(&terminatedCount) >= int32(numClients) {
				break waitLoop
			}
		}
	}

	// Close all clients
	for _, client := range clients {
		client.Close()
	}

	// Wait for cleanup and check goroutine count
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	finalGoroutines := runtime.NumGoroutine()

	assert.GreaterOrEqual(t, atomic.LoadInt32(&terminatedCount), int32(numClients-1), "Most streams should have been terminated")
	assert.LessOrEqual(t, finalGoroutines, initialGoroutines+2, "Goroutine count should not increase significantly")
}

// TestStreamLifetimeConfigValidation tests various configuration scenarios
func TestStreamLifetimeConfigValidation(t *testing.T) {
	tests := []struct {
		name              string
		maxStreamLifetime time.Duration
		expectError       bool
		errorMessage      string
	}{
		{
			name:              "ValidLifetime_30Minutes",
			maxStreamLifetime: 30 * time.Minute,
			expectError:       false,
		},
		{
			name:              "ValidLifetime_1Second",
			maxStreamLifetime: 1 * time.Second,
			expectError:       false,
		},
		{
			name:              "ZeroLifetime_ShouldUseDefault",
			maxStreamLifetime: 0,
			expectError:       false,
		},
		{
			name:              "NegativeLifetime_ValidationCheck",
			maxStreamLifetime: -1 * time.Second,
			expectError:       false, // NewSSEClient doesn't validate this, but we can check it separately
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := SSEClientConfig{
				URL:               "http://example.com/events",
				MaxStreamLifetime: tt.maxStreamLifetime,
				EventBufferSize:   100,
			}

			client, err := NewSSEClient(config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)

				if tt.maxStreamLifetime == 0 {
					// Should use default value
					assert.Equal(t, 30*time.Minute, client.config.MaxStreamLifetime)
				} else {
					assert.Equal(t, tt.maxStreamLifetime, client.config.MaxStreamLifetime)
				}

				// Test config validation separately if negative lifetime
				if tt.name == "NegativeLifetime_ValidationCheck" {
					err := config.Validate()
					assert.Error(t, err, "Config.Validate() should catch negative lifetime")
					if err != nil {
						assert.Contains(t, err.Error(), "max stream lifetime cannot be negative")
					}
				}

				client.Close()
			}
		})
	}
}

// TestConcurrentStreamsWithDifferentLifetimes tests multiple streams with different lifetime configurations
func TestConcurrentStreamsWithDifferentLifetimes(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		for i := 0; i < 50; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
				eventData := fmt.Sprintf("data: concurrent event %d\n\n", i)
				w.Write([]byte(eventData))
				flusher.Flush()
				time.Sleep(10 * time.Millisecond)
			}
		}
	}))
	defer server.Close()

	// Create streams with different lifetimes
	streamConfigs := []struct {
		lifetime time.Duration
		name     string
	}{
		{50 * time.Millisecond, "short"},
		{100 * time.Millisecond, "medium"},
		{200 * time.Millisecond, "long"},
	}

	terminationResults := make(map[string]bool)
	terminationTimes := make(map[string]time.Time)
	var mu sync.Mutex

	startTime := time.Now()
	clients := make([]*SSEClient, len(streamConfigs))

	for i, cfg := range streamConfigs {
		config := SSEClientConfig{
			URL:               server.URL,
			MaxStreamLifetime: cfg.lifetime,
			EventBufferSize:   50,
			OnError: func(err error) {
				if strings.Contains(err.Error(), "stream lifetime exceeded") {
					mu.Lock()
					terminationResults[cfg.name] = true
					terminationTimes[cfg.name] = time.Now()
					mu.Unlock()
				}
			},
		}

		client, err := NewSSEClient(config)
		require.NoError(t, err)
		clients[i] = client

		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()

		err = client.Connect(ctx)
		require.NoError(t, err)
	}

	// Wait for all streams to potentially terminate
	time.Sleep(250 * time.Millisecond)

	// Verify termination order and timing
	mu.Lock()
	shortTerminated := terminationResults["short"]
	mediumTerminated := terminationResults["medium"]
	longTerminated := terminationResults["long"]

	shortTime, shortTimeExists := terminationTimes["short"]
	mediumTime, mediumTimeExists := terminationTimes["medium"]
	mu.Unlock()

	// Short lifetime stream should have terminated
	assert.True(t, shortTerminated, "Short lifetime stream should have terminated")
	assert.True(t, mediumTerminated, "Medium lifetime stream should have terminated")

	// Verify ordering
	if shortTimeExists && mediumTimeExists {
		assert.True(t, shortTime.Before(mediumTime), "Short stream should terminate before medium stream")
	}

	// Long stream might or might not have terminated depending on timing
	// But if it did, it should be after the others
	if longTerminated {
		longTime := terminationTimes["long"]
		if shortTimeExists {
			assert.True(t, shortTime.Before(longTime), "Short stream should terminate before long stream")
		}
		if mediumTimeExists {
			assert.True(t, mediumTime.Before(longTime), "Medium stream should terminate before long stream")
		}
	}

	// Cleanup
	for _, client := range clients {
		client.Close()
	}

	// Verify all streams terminated within reasonable time bounds
	elapsedTime := time.Since(startTime)
	assert.Less(t, elapsedTime, 350*time.Millisecond, "Test should complete within reasonable time")
}
