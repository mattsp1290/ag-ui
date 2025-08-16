package sse

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReconnectorBackoffCalculation tests exponential backoff with jitter
func TestReconnectorBackoffCalculation(t *testing.T) {
	tests := []struct {
		name              string
		attempt           int
		initialDelay      time.Duration
		multiplier        float64
		maxDelay          time.Duration
		expectedMin       time.Duration
		expectedMax       time.Duration
	}{
		{
			name:         "first attempt has no delay",
			attempt:      0,
			initialDelay: 250 * time.Millisecond,
			multiplier:   2.0,
			maxDelay:     30 * time.Second,
			expectedMin:  0,
			expectedMax:  0,
		},
		{
			name:         "second attempt uses initial delay",
			attempt:      1,
			initialDelay: 250 * time.Millisecond,
			multiplier:   2.0,
			maxDelay:     30 * time.Second,
			expectedMin:  200 * time.Millisecond,  // with jitter
			expectedMax:  300 * time.Millisecond,
		},
		{
			name:         "exponential growth",
			attempt:      4,
			initialDelay: 250 * time.Millisecond,
			multiplier:   2.0,
			maxDelay:     30 * time.Second,
			expectedMin:  1600 * time.Millisecond, // 250 * 2^3 * 0.8
			expectedMax:  2400 * time.Millisecond, // 250 * 2^3 * 1.2
		},
		{
			name:         "capped at max delay",
			attempt:      10,
			initialDelay: 250 * time.Millisecond,
			multiplier:   2.0,
			maxDelay:     5 * time.Second,
			expectedMin:  4 * time.Second,  // max * 0.8
			expectedMax:  6 * time.Second,  // max * 1.2 (can exceed max with jitter)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultReconnectionConfig()
			config.InitialDelay = tt.initialDelay
			config.BackoffMultiplier = tt.multiplier
			config.MaxDelay = tt.maxDelay
			config.JitterFactor = 0.2

			rc := &ReconnectingClient{
				config:       config,
				attemptCount: tt.attempt,
				logger:       logrus.New(),
			}

			// Test multiple times to account for jitter randomness
			for i := 0; i < 10; i++ {
				delay := rc.calculateBackoff()
				
				if tt.expectedMin == 0 && tt.expectedMax == 0 {
					assert.Equal(t, time.Duration(0), delay)
				} else {
					assert.GreaterOrEqual(t, delay, tt.expectedMin,
						"Delay should be at least %v, got %v", tt.expectedMin, delay)
					assert.LessOrEqual(t, delay, tt.expectedMax,
						"Delay should be at most %v, got %v", tt.expectedMax, delay)
				}
			}
		})
	}
}

// TestReconnectorErrorClassification tests error classification logic
func TestReconnectorErrorClassification(t *testing.T) {
	rc := &ReconnectingClient{
		logger: logrus.New(),
	}

	tests := []struct {
		name        string
		err         error
		shouldRetry bool
	}{
		{
			name:        "EOF is retryable",
			err:         io.EOF,
			shouldRetry: true,
		},
		{
			name:        "network error is retryable",
			err:         fmt.Errorf("connection refused"),
			shouldRetry: true,
		},
		{
			name:        "timeout is retryable",
			err:         fmt.Errorf("i/o timeout"),
			shouldRetry: true,
		},
		{
			name:        "401 is not retryable",
			err:         fmt.Errorf("unexpected status code 401: Unauthorized"),
			shouldRetry: false,
		},
		{
			name:        "403 is not retryable",
			err:         fmt.Errorf("unexpected status code 403: Forbidden"),
			shouldRetry: false,
		},
		{
			name:        "404 is not retryable",
			err:         fmt.Errorf("unexpected status code 404: Not Found"),
			shouldRetry: false,
		},
		{
			name:        "500 is retryable",
			err:         fmt.Errorf("unexpected status code 500: Internal Server Error"),
			shouldRetry: true,
		},
		{
			name:        "503 is retryable",
			err:         fmt.Errorf("unexpected status code 503: Service Unavailable"),
			shouldRetry: true,
		},
		{
			name:        "429 is retryable",
			err:         fmt.Errorf("unexpected status code 429: Too Many Requests"),
			shouldRetry: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldRetry, _ := rc.classifyError(tt.err)
			assert.Equal(t, tt.shouldRetry, shouldRetry)
		})
	}
}

// TestReconnectorHTTPStatusClassification tests HTTP status classification
func TestReconnectorHTTPStatusClassification(t *testing.T) {
	rc := &ReconnectingClient{
		logger: logrus.New(),
	}

	tests := []struct {
		statusCode  int
		shouldRetry bool
		desc        string
	}{
		{200, true, "200 OK (shouldn't happen but retry)"},
		{401, false, "401 Unauthorized"},
		{403, false, "403 Forbidden"},
		{404, false, "404 Not Found"},
		{408, true, "408 Request Timeout"},
		{429, true, "429 Too Many Requests"},
		{500, true, "500 Internal Server Error"},
		{502, true, "502 Bad Gateway"},
		{503, true, "503 Service Unavailable"},
		{504, true, "504 Gateway Timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			shouldRetry, _ := rc.classifyHTTPStatus(tt.statusCode)
			assert.Equal(t, tt.shouldRetry, shouldRetry)
		})
	}
}

// TestReconnectorStreamWithDrops simulates connection drops and verifies reconnection
func TestReconnectorStreamWithDrops(t *testing.T) {
	var (
		connectionCount int32
		dropAfter       = 3 // Drop connection after 3 messages
		totalMessages   = 10
	)

	// Create test server that drops connections
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&connectionCount, 1)
		
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		
		// Send a few messages then drop
		startMsg := (int(count) - 1) * dropAfter
		for i := 0; i < dropAfter && startMsg+i < totalMessages; i++ {
			msg := fmt.Sprintf("data: message %d\n\n", startMsg+i)
			_, err := w.Write([]byte(msg))
			if err != nil {
				return
			}
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
		
		// Simulate connection drop by not sending anything else
		// The client should detect this and reconnect
	}))
	defer server.Close()

	// Configure client with aggressive reconnection
	config := Config{
		Endpoint:       server.URL,
		ConnectTimeout: 1 * time.Second,
		ReadTimeout:    100 * time.Millisecond, // Short timeout to detect drops quickly
		BufferSize:     10,
		Logger:         logrus.New(),
	}
	
	reconnectConfig := ReconnectionConfig{
		Enabled:           true,
		InitialDelay:      50 * time.Millisecond,
		MaxDelay:          500 * time.Millisecond,
		BackoffMultiplier: 2.0,
		JitterFactor:      0.1,
		MaxRetries:        5,
		IdleTimeout:       200 * time.Millisecond,
	}
	
	client := NewReconnectingClient(config, reconnectConfig)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	frames, errors, err := client.StreamWithReconnect(StreamOptions{
		Context: ctx,
	})
	require.NoError(t, err)
	
	// Collect messages
	var messages []string
	done := make(chan struct{})
	
	go func() {
		defer close(done)
		for {
			select {
			case frame, ok := <-frames:
				if !ok {
					return
				}
				messages = append(messages, string(frame.Data))
			case err, ok := <-errors:
				if !ok {
					return
				}
				// Non-retryable error
				t.Logf("Error: %v", err)
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	
	// Wait for completion
	<-done
	
	// Verify we got messages from multiple connections
	assert.Greater(t, atomic.LoadInt32(&connectionCount), int32(1), "Should have reconnected at least once")
	assert.NotEmpty(t, messages, "Should have received messages")
	t.Logf("Received %d messages across %d connections", len(messages), atomic.LoadInt32(&connectionCount))
}

// TestReconnectorContextCancellation verifies proper cleanup on context cancellation
func TestReconnectorContextCancellation(t *testing.T) {
	// Create a server that sends messages slowly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		
		for i := 0; i < 100; i++ {
			msg := fmt.Sprintf("data: message %d\n\n", i)
			_, err := w.Write([]byte(msg))
			if err != nil {
				return
			}
			flusher.Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer server.Close()

	config := Config{
		Endpoint:   server.URL,
		BufferSize: 10,
		Logger:     logrus.New(),
	}
	
	reconnectConfig := DefaultReconnectionConfig()
	client := NewReconnectingClient(config, reconnectConfig)
	
	ctx, cancel := context.WithCancel(context.Background())
	
	frames, errors, err := client.StreamWithReconnect(StreamOptions{
		Context: ctx,
	})
	require.NoError(t, err)
	
	// Receive a few messages
	var messageCount int
	go func() {
		for {
			select {
			case _, ok := <-frames:
				if !ok {
					return
				}
				messageCount++
				if messageCount == 3 {
					// Cancel after receiving 3 messages
					cancel()
				}
			case <-errors:
				return
			}
		}
	}()
	
	// Wait a bit for cancellation to propagate
	time.Sleep(500 * time.Millisecond)
	
	// Verify channels are closed
	select {
	case _, ok := <-frames:
		assert.False(t, ok, "Frames channel should be closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Frames channel not closed after context cancellation")
	}
	
	select {
	case _, ok := <-errors:
		assert.False(t, ok, "Errors channel should be closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Errors channel not closed after context cancellation")
	}
}

// TestReconnectorMaxRetries verifies retry limit enforcement
func TestReconnectorMaxRetries(t *testing.T) {
	var attemptCount int32
	
	// Create a server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	config := Config{
		Endpoint:       server.URL,
		ConnectTimeout: 100 * time.Millisecond,
		Logger:         logrus.New(),
	}
	
	reconnectConfig := ReconnectionConfig{
		Enabled:           true,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		MaxRetries:        3,
	}
	
	client := NewReconnectingClient(config, reconnectConfig)
	
	ctx := context.Background()
	frames, errors, err := client.StreamWithReconnect(StreamOptions{
		Context: ctx,
	})
	require.NoError(t, err)
	
	// Wait for max retries error
	select {
	case err := <-errors:
		assert.Contains(t, err.Error(), "max reconnection attempts")
	case <-time.After(2 * time.Second):
		t.Fatal("Expected max retries error")
	}
	
	// Verify channels are closed
	_, ok := <-frames
	assert.False(t, ok, "Frames channel should be closed")
	
	// Verify attempt count
	assert.Equal(t, int32(3), atomic.LoadInt32(&attemptCount))
}

// TestReconnectorMaxElapsedTime verifies elapsed time limit enforcement
func TestReconnectorMaxElapsedTime(t *testing.T) {
	// Create a server that always fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	config := Config{
		Endpoint:       server.URL,
		ConnectTimeout: 100 * time.Millisecond,
		Logger:         logrus.New(),
	}
	
	reconnectConfig := ReconnectionConfig{
		Enabled:           true,
		InitialDelay:      50 * time.Millisecond,
		MaxDelay:          100 * time.Millisecond,
		BackoffMultiplier: 1.5,
		MaxElapsedTime:    500 * time.Millisecond,
	}
	
	client := NewReconnectingClient(config, reconnectConfig)
	
	ctx := context.Background()
	start := time.Now()
	
	frames, errors, err := client.StreamWithReconnect(StreamOptions{
		Context: ctx,
	})
	require.NoError(t, err)
	
	// Wait for max elapsed time error
	select {
	case err := <-errors:
		assert.Contains(t, err.Error(), "max elapsed time")
		elapsed := time.Since(start)
		// Should fail around 500ms, give some buffer
		assert.Less(t, elapsed, 1*time.Second)
	case <-time.After(2 * time.Second):
		t.Fatal("Expected max elapsed time error")
	}
	
	// Verify channel is closed
	_, ok := <-frames
	assert.False(t, ok, "Frames channel should be closed")
}

// TestReconnectorIdleTimeout verifies reconnection on idle timeout
func TestReconnectorIdleTimeout(t *testing.T) {
	var (
		connectionCount int32
		messageSent     = make(chan struct{}, 1)
	)
	
	// Create a server that sends one message then goes idle
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&connectionCount, 1)
		
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		
		// Send one message
		msg := fmt.Sprintf("data: connection %d\n\n", count)
		_, err := w.Write([]byte(msg))
		require.NoError(t, err)
		flusher.Flush()
		
		select {
		case messageSent <- struct{}{}:
		default:
		}
		
		// Then go idle (don't close connection, just stop sending)
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	config := Config{
		Endpoint:    server.URL,
		ReadTimeout: 5 * time.Second,
		BufferSize:  10,
		Logger:      logrus.New(),
	}
	
	reconnectConfig := ReconnectionConfig{
		Enabled:           true,
		InitialDelay:      50 * time.Millisecond,
		MaxDelay:          200 * time.Millisecond,
		BackoffMultiplier: 2.0,
		IdleTimeout:       200 * time.Millisecond, // Short idle timeout
		MaxRetries:        3,
	}
	
	client := NewReconnectingClient(config, reconnectConfig)
	
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	
	frames, _, err := client.StreamWithReconnect(StreamOptions{
		Context: ctx,
	})
	require.NoError(t, err)
	
	// Collect messages with thread-safe access
	var messages []string
	var messagesMu sync.Mutex
	go func() {
		for frame := range frames {
			messagesMu.Lock()
			messages = append(messages, string(frame.Data))
			messagesMu.Unlock()
		}
	}()
	
	// Wait for timeout
	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)
	
	// Should have reconnected at least once due to idle timeout
	assert.Greater(t, atomic.LoadInt32(&connectionCount), int32(1),
		"Should have reconnected at least once due to idle timeout")
	
	// Check message count with thread-safe access
	messagesMu.Lock()
	messageCount := len(messages)
	messagesMu.Unlock()
	assert.Equal(t, messageCount, int(atomic.LoadInt32(&connectionCount)),
		"Should have one message per connection")
}

// TestReconnectorGoroutineLeaks verifies no goroutine leaks
func TestReconnectorGoroutineLeaks(t *testing.T) {
	// Get initial goroutine count
	initialCount := countGoroutines()
	
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		
		for i := 0; i < 5; i++ {
			msg := fmt.Sprintf("data: message %d\n\n", i)
			_, _ = w.Write([]byte(msg))
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	// Run multiple connection cycles
	for i := 0; i < 10; i++ {
		config := Config{
			Endpoint:    server.URL,
			ReadTimeout: 100 * time.Millisecond,
			BufferSize:  10,
			Logger:      logrus.New(),
		}
		
		reconnectConfig := DefaultReconnectionConfig()
		reconnectConfig.MaxRetries = 2
		
		client := NewReconnectingClient(config, reconnectConfig)
		
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		
		frames, _, err := client.StreamWithReconnect(StreamOptions{
			Context: ctx,
		})
		require.NoError(t, err)
		
		// Drain messages
		go func() {
			for range frames {
			}
		}()
		
		// Let it run briefly
		time.Sleep(100 * time.Millisecond)
		
		// Cancel and cleanup
		cancel()
		client.Close()
		
		// Wait for cleanup
		time.Sleep(50 * time.Millisecond)
	}
	
	// Wait for goroutines to cleanup
	time.Sleep(500 * time.Millisecond)
	
	// Check final goroutine count
	finalCount := countGoroutines()
	
	// Allow for some variance but should not grow significantly
	assert.LessOrEqual(t, finalCount, initialCount+5,
		"Goroutine leak detected: initial=%d, final=%d", initialCount, finalCount)
}

// TestParseRetryAfter tests Retry-After header parsing
func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
	}{
		{
			input:    "120",
			expected: 120 * time.Second,
			hasError: false,
		},
		{
			input:    "0",
			expected: 0,
			hasError: false,
		},
		{
			input:    "invalid",
			expected: 0,
			hasError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			duration, err := ParseRetryAfter(tt.input)
			
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, duration)
			}
		})
	}
}

// TestReconnectorLastEventID tests Last-Event-ID header propagation
func TestReconnectorLastEventID(t *testing.T) {
	var lastEventIDReceived string
	var mu sync.Mutex
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture Last-Event-ID header
		mu.Lock()
		lastEventIDReceived = r.Header.Get("Last-Event-ID")
		mu.Unlock()
		
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		
		// Send a message
		msg := "data: test message\n\n"
		_, _ = w.Write([]byte(msg))
		flusher.Flush()
	}))
	defer server.Close()

	config := Config{
		Endpoint: server.URL,
		Logger:   logrus.New(),
	}
	
	reconnectConfig := DefaultReconnectionConfig()
	client := NewReconnectingClient(config, reconnectConfig)
	
	// Set a Last-Event-ID
	testEventID := "test-event-123"
	client.SetLastEventID(testEventID)
	
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	_, _, err := client.StreamWithReconnect(StreamOptions{
		Context: ctx,
	})
	require.NoError(t, err)
	
	// Wait a bit for connection
	time.Sleep(100 * time.Millisecond)
	
	// Verify Last-Event-ID was sent
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, testEventID, lastEventIDReceived)
}

// Helper function to count goroutines
func countGoroutines() int {
	return runtime.NumGoroutine()
}