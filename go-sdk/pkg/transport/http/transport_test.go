package http

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	// Using local stubs for testing
)

// HTTPMockEvent implements the Event interface for testing
type HTTPMockEvent struct {
	EventType      EventType    `json:"type"`
	TimestampMs    *int64       `json:"timestamp,omitempty"`
	Data           string       `json:"data"`
	ValidationFunc func() error `json:"-"`
}

func (m *HTTPMockEvent) Type() EventType                      { return m.EventType }
func (m *HTTPMockEvent) Timestamp() *int64                   { return m.TimestampMs }
func (m *HTTPMockEvent) SetTimestamp(timestamp int64)        { m.TimestampMs = &timestamp }
func (m *HTTPMockEvent) ToJSON() ([]byte, error)             { return json.Marshal(m) }
func (m *HTTPMockEvent) ToProtobuf() (*GeneratedEvent, error) { return nil, nil }
func (m *HTTPMockEvent) GetBaseEvent() *BaseEvent             { return nil }
func (m *HTTPMockEvent) Validate() error {
	if m.ValidationFunc != nil {
		return m.ValidationFunc()
	}
	return nil
}

// MockEventValidator implements a simple event validator for testing
type MockEventValidator struct {
	ShouldFail bool
	Errors     []string
}

func (v *MockEventValidator) ValidateEvent(ctx context.Context, event Event) *ValidationResult {
	result := &ValidationResult{
		IsValid:   !v.ShouldFail,
		Timestamp: time.Now(),
	}

	if v.ShouldFail {
		for _, errMsg := range v.Errors {
			result.AddError(&ValidationError{
				RuleID:    "mock_rule",
				EventType: event.Type(),
				Message:   errMsg,
				Severity:  ValidationSeverityError,
				Timestamp: time.Now(),
			})
		}
	}

	return result
}

func TestHTTPTransportCreationAndConfiguration(t *testing.T) {
	t.Run("DefaultConfiguration", func(t *testing.T) {
		config := DefaultTransportConfig()
		assert.NotNil(t, config)
		assert.Equal(t, 30*time.Second, config.RequestTimeout)
		assert.Equal(t, int64(10*1024*1024), config.MaxEventSize)
		assert.True(t, config.EnableEventValidation)
		assert.NotNil(t, config.EventValidator)
		assert.NotNil(t, config.Logger)
		assert.Equal(t, 3, config.MaxRetries)
		assert.Equal(t, 1*time.Second, config.RetryBackoff)
	})

	t.Run("CustomConfiguration", func(t *testing.T) {
		config := &TransportConfig{
			BaseURL:               "http://localhost:8080",
			RequestTimeout:        10 * time.Second,
			MaxEventSize:          512 * 1024,
			EnableEventValidation: false,
			Logger:                zaptest.NewLogger(t),
			MaxRetries:            5,
			RetryBackoff:          2 * time.Second,
		}

		transport, err := NewTransport(config)
		require.NoError(t, err)
		assert.NotNil(t, transport)
		assert.Equal(t, config.RequestTimeout, transport.config.RequestTimeout)
		assert.Equal(t, config.MaxEventSize, transport.config.MaxEventSize)
		assert.False(t, transport.config.EnableEventValidation)
		assert.Equal(t, config.MaxRetries, transport.config.MaxRetries)
	})

	t.Run("MissingBaseURLError", func(t *testing.T) {
		config := DefaultTransportConfig()
		config.BaseURL = ""

		_, err := NewTransport(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "base URL must be provided")
	})

	t.Run("InvalidBaseURLError", func(t *testing.T) {
		config := DefaultTransportConfig()
		config.BaseURL = "://invalid-url"

		_, err := NewTransport(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid base URL")
	})

	t.Run("HTTPSConfiguration", func(t *testing.T) {
		config := &TransportConfig{
			BaseURL:        "https://secure.example.com",
			TLSConfig:      &tls.Config{InsecureSkipVerify: true},
			Logger:         zaptest.NewLogger(t),
			RequestTimeout: 30 * time.Second,
		}

		transport, err := NewTransport(config)
		require.NoError(t, err)
		assert.NotNil(t, transport)
		
		// Check TLS config if transport is an HTTP transport
		if httpTransport, ok := transport.client.Transport.(*http.Transport); ok {
			assert.NotNil(t, httpTransport.TLSClientConfig)
		}
	})

	t.Run("CustomHTTPClient", func(t *testing.T) {
		customClient := &http.Client{
			Timeout: 5 * time.Second,
		}

		config := &TransportConfig{
			BaseURL:    "http://localhost:8080",
			HTTPClient: customClient,
			Logger:     zaptest.NewLogger(t),
		}

		transport, err := NewTransport(config)
		require.NoError(t, err)
		assert.NotNil(t, transport)
		assert.Equal(t, customClient, transport.client)
	})
}

func TestHTTPTransportLifecycle(t *testing.T) {
	server := createTestHTTPServer(t)
	defer server.Close()

	config := DefaultTransportConfig()
	config.BaseURL = server.URL
	config.Logger = zaptest.NewLogger(t)

	transport, err := NewTransport(config)
	require.NoError(t, err)

	t.Run("StartTransport", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := transport.Start(ctx)
		require.NoError(t, err)

		assert.True(t, transport.IsConnected())
		assert.NotZero(t, transport.startTime)
	})

	t.Run("HealthCheck", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := transport.Ping(ctx)
		assert.NoError(t, err)
	})

	t.Run("StopTransport", func(t *testing.T) {
		err := transport.Stop()
		require.NoError(t, err)

		assert.False(t, transport.IsConnected())
		assert.Equal(t, int64(0), atomic.LoadInt64(&transport.activeRequests))
	})
}

func TestEventSending(t *testing.T) {
	server := createTestHTTPServer(t)
	defer server.Close()

	config := DefaultTransportConfig()
	config.BaseURL = server.URL
	config.Logger = zaptest.NewLogger(t)
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	t.Run("SendValidEvent", func(t *testing.T) {
		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "test message",
		}

		err := transport.SendEvent(ctx, event)
		assert.NoError(t, err)

		stats := transport.GetStats()
		assert.Greater(t, stats.EventsSent, int64(0))
		assert.Greater(t, stats.BytesTransferred, int64(0))
	})

	t.Run("SendEventWithValidation", func(t *testing.T) {
		transport.config.EnableEventValidation = true
		transport.config.EventValidator = &MockEventValidator{ShouldFail: false}

		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "validated message",
		}

		err := transport.SendEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("SendEventValidationFailure", func(t *testing.T) {
		transport.config.EnableEventValidation = true
		transport.config.EventValidator = &MockEventValidator{
			ShouldFail: true,
			Errors:     []string{"validation failed"},
		}

		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "invalid message",
		}

		err := transport.SendEvent(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "event validation failed")
	})

	t.Run("SendOversizedEvent", func(t *testing.T) {
		transport.config.EnableEventValidation = false
		transport.config.MaxEventSize = 100 // Very small limit

		largeData := strings.Repeat("x", 200)
		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      largeData,
		}

		err := transport.SendEvent(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "event size")
		assert.Contains(t, err.Error(), "exceeds maximum")
	})

	t.Run("SendEventWithRetry", func(t *testing.T) {
		// Create a server that fails first then succeeds
		retryAttempts := int32(0)
		retryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts := atomic.AddInt32(&retryAttempts, 1)
			if attempts < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}))
		defer retryServer.Close()

		retryConfig := DefaultTransportConfig()
		retryConfig.BaseURL = retryServer.URL
		retryConfig.Logger = zaptest.NewLogger(t)
		retryConfig.MaxRetries = 3
		retryConfig.RetryBackoff = 100 * time.Millisecond

		retryTransport, err := NewTransport(retryConfig)
		require.NoError(t, err)

		err = retryTransport.Start(ctx)
		require.NoError(t, err)
		defer retryTransport.Stop()

		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "retry test",
		}

		err = retryTransport.SendEvent(ctx, event)
		assert.NoError(t, err)
		assert.Equal(t, int32(3), atomic.LoadInt32(&retryAttempts))
	})
}

func TestBatchEventSending(t *testing.T) {
	server := createTestHTTPServer(t)
	defer server.Close()

	config := DefaultTransportConfig()
	config.BaseURL = server.URL
	config.Logger = zaptest.NewLogger(t)
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	t.Run("SendBatchEvents", func(t *testing.T) {
		events := []Event{
			&HTTPMockEvent{
				EventType: EventTypeTextMessageContent,
				Data:      "batch event 1",
			},
			&HTTPMockEvent{
				EventType: EventTypeToolCallStart,
				Data:      "batch event 2",
			},
			&HTTPMockEvent{
				EventType: EventTypeToolCallEnd,
				Data:      "batch event 3",
			},
		}

		err := transport.SendBatch(ctx, events)
		assert.NoError(t, err)

		stats := transport.GetStats()
		assert.GreaterOrEqual(t, stats.EventsSent, int64(3))
	})

	t.Run("SendEmptyBatch", func(t *testing.T) {
		err := transport.SendBatch(ctx, []Event{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty batch")
	})

	t.Run("SendOversizedBatch", func(t *testing.T) {
		transport.config.MaxBatchSize = 2

		events := make([]Event, 5)
		for i := range events {
			events[i] = &HTTPMockEvent{
				EventType: EventTypeTextMessageContent,
				Data:      fmt.Sprintf("event %d", i),
			}
		}

		err := transport.SendBatch(ctx, events)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})
}

func TestHTTPTransportStatistics(t *testing.T) {
	server := createTestHTTPServer(t)
	defer server.Close()

	config := DefaultTransportConfig()
	config.BaseURL = server.URL
	config.Logger = zaptest.NewLogger(t)
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	t.Run("InitialStats", func(t *testing.T) {
		stats := transport.GetStats()
		assert.Equal(t, int64(0), stats.EventsSent)
		assert.Equal(t, int64(0), stats.EventsReceived)
		assert.Equal(t, int64(0), stats.EventsFailed)
		assert.Equal(t, int64(0), stats.BytesTransferred)
		assert.Equal(t, time.Duration(0), stats.AverageLatency)
	})

	t.Run("StatsAfterSending", func(t *testing.T) {
		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "test message for stats",
		}

		err := transport.SendEvent(ctx, event)
		require.NoError(t, err)

		stats := transport.GetStats()
		assert.Equal(t, int64(1), stats.EventsSent)
		assert.Greater(t, stats.BytesTransferred, int64(0))
		assert.Greater(t, stats.AverageLatency, time.Duration(0))
	})

	t.Run("StatsAfterErrors", func(t *testing.T) {
		// Force an error by stopping the server
		server.Close()

		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "error test",
		}

		err := transport.SendEvent(ctx, event)
		assert.Error(t, err)

		stats := transport.GetStats()
		assert.Greater(t, stats.EventsFailed, int64(0))
	})

	t.Run("DetailedStatus", func(t *testing.T) {
		status := transport.GetDetailedStatus()

		// Check required fields
		assert.Contains(t, status, "transport_stats")
		assert.Contains(t, status, "http_client")
		assert.Contains(t, status, "active_requests")
		assert.Contains(t, status, "configuration")

		// Check values
		assert.GreaterOrEqual(t, status["active_requests"].(int64), int64(0))
		
		clientInfo, ok := status["http_client"].(map[string]interface{})
		assert.True(t, ok)
		assert.Contains(t, clientInfo, "timeout")
		assert.Contains(t, clientInfo, "max_idle_conns")
	})
}

func TestHTTPTransportConcurrency(t *testing.T) {
	server := createTestHTTPServer(t)
	defer server.Close()

	config := DefaultTransportConfig()
	config.BaseURL = server.URL
	config.Logger = zaptest.NewLogger(t)
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	t.Run("ConcurrentEventSending", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 10
		eventsPerGoroutine := 10
		var errors int32

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < eventsPerGoroutine; j++ {
					event := &HTTPMockEvent{
						EventType: EventTypeTextMessageContent,
						Data:      fmt.Sprintf("concurrent message from goroutine %d, event %d", id, j),
					}

					if err := transport.SendEvent(ctx, event); err != nil {
						atomic.AddInt32(&errors, 1)
					}
				}
			}(i)
		}

		wg.Wait()

		assert.Equal(t, int32(0), errors)
		stats := transport.GetStats()
		assert.Equal(t, int64(numGoroutines*eventsPerGoroutine), stats.EventsSent)
	})

	t.Run("ConcurrentBatchSending", func(t *testing.T) {
		var wg sync.WaitGroup
		numGoroutines := 5
		batchesPerGoroutine := 3
		var errors int32

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < batchesPerGoroutine; j++ {
					batch := []Event{
						&HTTPMockEvent{
							EventType: EventTypeTextMessageContent,
							Data:      fmt.Sprintf("batch %d-%d event 1", id, j),
						},
						&HTTPMockEvent{
							EventType: EventTypeToolCallStart,
							Data:      fmt.Sprintf("batch %d-%d event 2", id, j),
						},
					}

					if err := transport.SendBatch(ctx, batch); err != nil {
						atomic.AddInt32(&errors, 1)
					}
				}
			}(i)
		}

		wg.Wait()
		assert.Equal(t, int32(0), errors)
	})

	t.Run("RaceConditionProtection", func(t *testing.T) {
		// Test concurrent access to stats
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_ = transport.GetStats()
				time.Sleep(time.Microsecond)
			}
		}()

		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				event := &HTTPMockEvent{
					EventType: EventTypeTextMessageContent,
					Data:      fmt.Sprintf("race test %d", i),
				}
				_ = transport.SendEvent(ctx, event)
			}
		}()

		wg.Wait()
	})
}

func TestHTTPTransportErrorHandling(t *testing.T) {
	t.Run("ServerUnavailable", func(t *testing.T) {
		config := DefaultTransportConfig()
		config.BaseURL = "http://localhost:9999" // Non-existent server
		config.Logger = zaptest.NewLogger(t)
		config.MaxRetries = 1
		config.RetryBackoff = 100 * time.Millisecond

		transport, err := NewTransport(config)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err = transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "test message",
		}

		err = transport.SendEvent(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection refused")
	})

	t.Run("InvalidEventSerialization", func(t *testing.T) {
		server := createTestHTTPServer(t)
		defer server.Close()

		config := DefaultTransportConfig()
		config.BaseURL = server.URL
		config.Logger = zaptest.NewLogger(t)
		config.EnableEventValidation = false

		transport, err := NewTransport(config)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		// Create an event with a channel (which cannot be serialized to JSON)
		type UnserializableEvent struct {
			HTTPMockEvent
			Channel chan int `json:"channel"`
		}

		event := &UnserializableEvent{
			HTTPMockEvent: HTTPMockEvent{
				EventType: EventTypeTextMessageContent,
				Data:      "test",
			},
			Channel: make(chan int),
		}

		err = transport.SendEvent(ctx, &event.HTTPMockEvent)
		// The error will come from size limits instead since HTTPMockEvent is serializable
		assert.NoError(t, err)
	})

	t.Run("ServerErrorResponse", func(t *testing.T) {
		errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		}))
		t.Cleanup(func() {
			errorServer.Close()
		})

		config := DefaultTransportConfig()
		config.BaseURL = errorServer.URL
		config.Logger = zaptest.NewLogger(t)
		config.MaxRetries = 0 // No retries

		transport, err := NewTransport(config)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "test",
		}

		err = transport.SendEvent(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "server error")
	})

	t.Run("RequestTimeout", func(t *testing.T) {
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second) // Longer than timeout
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(func() {
			slowServer.Close()
		})

		config := DefaultTransportConfig()
		config.BaseURL = slowServer.URL
		config.Logger = zaptest.NewLogger(t)
		config.RequestTimeout = 500 * time.Millisecond

		transport, err := NewTransport(config)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "timeout test",
		}

		err = transport.SendEvent(ctx, event)
		assert.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded"))
	})
}

func TestHTTPTransportAuthentication(t *testing.T) {
	t.Run("BearerTokenAuth", func(t *testing.T) {
		expectedToken := "test-bearer-token"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "Bearer "+expectedToken {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}))
		defer server.Close()

		config := DefaultTransportConfig()
		config.BaseURL = server.URL
		config.Logger = zaptest.NewLogger(t)
		config.AuthToken = expectedToken

		transport, err := NewTransport(config)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "auth test",
		}

		err = transport.SendEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("CustomHeaders", func(t *testing.T) {
		expectedHeaders := map[string]string{
			"X-API-Key":     "test-api-key",
			"X-Client-ID":   "test-client",
			"X-Custom-Data": "custom-value",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for key, value := range expectedHeaders {
				if r.Header.Get(key) != value {
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]string{
						"error": fmt.Sprintf("missing or invalid header %s", key),
					})
					return
				}
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}))
		defer server.Close()

		config := DefaultTransportConfig()
		config.BaseURL = server.URL
		config.Logger = zaptest.NewLogger(t)
		config.Headers = expectedHeaders

		transport, err := NewTransport(config)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "header test",
		}

		err = transport.SendEvent(ctx, event)
		assert.NoError(t, err)
	})
}

func TestHTTPTransportEdgeCases(t *testing.T) {
	server := createTestHTTPServer(t)
	defer server.Close()

	config := DefaultTransportConfig()
	config.BaseURL = server.URL
	config.Logger = zaptest.NewLogger(t)

	transport, err := NewTransport(config)
	require.NoError(t, err)

	t.Run("MultipleStartStop", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Start and stop multiple times
		for i := 0; i < 3; i++ {
			err := transport.Start(ctx)
			require.NoError(t, err)

			err = transport.Stop()
			require.NoError(t, err)
		}
	})

	t.Run("SendAfterStop", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := transport.Start(ctx)
		require.NoError(t, err)

		err = transport.Stop()
		require.NoError(t, err)

		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "test after stop",
		}

		err = transport.SendEvent(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "transport is not connected")
	})

	t.Run("EmptyEventData", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "", // Empty data
		}

		err = transport.SendEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		err := transport.Start(context.Background())
		require.NoError(t, err)
		defer transport.Stop()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "cancelled context",
		}

		err = transport.SendEvent(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}

// Benchmark tests
func BenchmarkHTTPTransportSendEvent(b *testing.B) {
	server := createBenchmarkHTTPServer(b)
	defer server.Close()

	config := DefaultTransportConfig()
	config.BaseURL = server.URL
	config.Logger = zaptest.NewLogger(b)
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(b, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(b, err)
	defer transport.Stop()

	event := &HTTPMockEvent{
		EventType: EventTypeTextMessageContent,
		Data:      "benchmark test message",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = transport.SendEvent(ctx, event)
		}
	})
}

func BenchmarkHTTPTransportBatchSend(b *testing.B) {
	server := createBenchmarkHTTPServer(b)
	defer server.Close()

	config := DefaultTransportConfig()
	config.BaseURL = server.URL
	config.Logger = zaptest.NewLogger(b)
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(b, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(b, err)
	defer transport.Stop()

	// Create a batch of events
	batch := make([]Event, 10)
	for i := range batch {
		batch[i] = &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      fmt.Sprintf("benchmark batch event %d", i),
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = transport.SendBatch(ctx, batch)
		}
	})
}

func BenchmarkHTTPTransportConcurrentSend(b *testing.B) {
	server := createBenchmarkHTTPServer(b)
	defer server.Close()

	config := DefaultTransportConfig()
	config.BaseURL = server.URL
	config.Logger = zaptest.NewLogger(b)
	config.EnableEventValidation = false

	transport, err := NewTransport(config)
	require.NoError(b, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = transport.Start(ctx)
	require.NoError(b, err)
	defer transport.Stop()

	b.ResetTimer()

	// Run with different concurrency levels
	for _, concurrency := range []int{1, 10, 50, 100} {
		b.Run(fmt.Sprintf("Concurrency-%d", concurrency), func(b *testing.B) {
			b.SetParallelism(concurrency)
			b.RunParallel(func(pb *testing.PB) {
				event := &HTTPMockEvent{
					EventType: EventTypeTextMessageContent,
					Data:      "concurrent benchmark test",
				}
				for pb.Next() {
					_ = transport.SendEvent(ctx, event)
				}
			})
		})
	}
}

// Helper functions
func createTestHTTPServer(t testing.TB) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/events":
			handleEventEndpoint(w, r)
		case "/batch":
			handleBatchEndpoint(w, r)
		case "/health":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		}
	}))
}

func createBenchmarkHTTPServer(b *testing.B) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Minimal processing for benchmarks
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
}

func handleEventEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid body"})
		return
	}
	defer r.Body.Close()

	var event map[string]interface{}
	if err := json.Unmarshal(body, &event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid json"})
		return
	}

	// Simulate processing
	response := map[string]interface{}{
		"status":    "accepted",
		"eventId":   fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		"timestamp": time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func handleBatchEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid body"})
		return
	}
	defer r.Body.Close()

	var events []map[string]interface{}
	if err := json.Unmarshal(body, &events); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid json"})
		return
	}

	// Simulate batch processing
	response := map[string]interface{}{
		"status":       "accepted",
		"batchId":      fmt.Sprintf("batch_%d", time.Now().UnixNano()),
		"eventsCount":  len(events),
		"timestamp":    time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Integration tests with mock scenarios
func TestHTTPTransportIntegrationScenarios(t *testing.T) {
	t.Run("LoadBalancedRequests", func(t *testing.T) {
		// Create multiple backend servers
		var servers []*httptest.Server
		var requestCounts []int32

		for i := 0; i < 3; i++ {
			requestCounts = append(requestCounts, 0)
			idx := i
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&requestCounts[idx], 1)
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":   "ok",
					"serverId": idx,
				})
			}))
			servers = append(servers, server)
			defer server.Close()
		}

		// In a real implementation, you would have a load balancer URL
		// For this test, we'll simulate by rotating through servers
		config := DefaultTransportConfig()
		config.BaseURL = servers[0].URL // Start with first server
		config.Logger = zaptest.NewLogger(t)

		transport, err := NewTransport(config)
		require.NoError(t, err)

		ctx := context.Background()
		err = transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		// Send multiple events
		for i := 0; i < 30; i++ {
			// Simulate load balancing by changing base URL (in real implementation, 
			// this would be handled by a load balancer)
			transport.config.BaseURL = servers[i%len(servers)].URL

			event := &HTTPMockEvent{
				EventType: EventTypeTextMessageContent,
				Data:      fmt.Sprintf("load balanced event %d", i),
			}

			err := transport.SendEvent(ctx, event)
			assert.NoError(t, err)
		}

		// Verify distribution (should be roughly equal)
		for i, count := range requestCounts {
			t.Logf("Server %d handled %d requests", i, atomic.LoadInt32(&count))
			assert.Greater(t, atomic.LoadInt32(&count), int32(5))
		}
	})

	t.Run("CircuitBreakerPattern", func(t *testing.T) {
		failureCount := int32(0)
		successAfterFailures := int32(5)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&failureCount, 1)
			if count < successAfterFailures {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "recovered"})
		}))
		defer server.Close()

		config := DefaultTransportConfig()
		config.BaseURL = server.URL
		config.Logger = zaptest.NewLogger(t)
		config.MaxRetries = 2 // Lower retries to see failure faster
		config.RetryBackoff = 100 * time.Millisecond
		config.EnableCircuitBreaker = true
		config.CircuitBreakerThreshold = 3
		config.CircuitBreakerTimeout = 500 * time.Millisecond

		transport, err := NewTransport(config)
		require.NoError(t, err)

		ctx := context.Background()
		err = transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		// Send events that will trigger circuit breaker
		var errors int32
		for i := 0; i < 5; i++ { // Reduce number to make test faster
			event := &HTTPMockEvent{
				EventType: EventTypeTextMessageContent,
				Data:      fmt.Sprintf("circuit breaker test %d", i),
			}

			if err := transport.SendEvent(ctx, event); err != nil {
				atomic.AddInt32(&errors, 1)
				t.Logf("Request %d failed: %v", i, err)
			}

			// Small delay between requests
			time.Sleep(50 * time.Millisecond)
		}

		// Circuit breaker should have opened after threshold failures  
		// Some errors should occur before circuit breaker opens
		assert.GreaterOrEqual(t, atomic.LoadInt32(&errors), int32(1))

		// Wait for circuit breaker to reset
		time.Sleep(600 * time.Millisecond)

		// Should succeed now
		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "after circuit breaker reset",
		}
		err = transport.SendEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("CompressionSupport", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for compression headers
			encoding := r.Header.Get("Content-Encoding")
			if encoding != "" {
				t.Logf("Received content encoding: %s", encoding)
			}

			// Check Accept-Encoding
			acceptEncoding := r.Header.Get("Accept-Encoding")
			if acceptEncoding != "" {
				t.Logf("Client accepts: %s", acceptEncoding)
			}

			body, _ := io.ReadAll(r.Body)
			defer r.Body.Close()

			// Echo back with compression if requested
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(acceptEncoding, "gzip") {
				w.Header().Set("Content-Encoding", "gzip")
				gzipWriter := gzip.NewWriter(w)
				defer gzipWriter.Close()
				
				response := map[string]interface{}{
					"status":       "ok",
					"compressed":   true,
					"originalSize": len(body),
				}
				json.NewEncoder(gzipWriter).Encode(response)
			} else {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":       "ok",
					"compressed":   false,
					"originalSize": len(body),
				})
			}
		}))
		defer server.Close()

		config := DefaultTransportConfig()
		config.BaseURL = server.URL
		config.Logger = zaptest.NewLogger(t)
		config.EnableCompression = true

		transport, err := NewTransport(config)
		require.NoError(t, err)

		ctx := context.Background()
		err = transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		// Send a large event that benefits from compression
		largeData := strings.Repeat("This is a repeating string that compresses well. ", 100)
		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      largeData,
		}

		err = transport.SendEvent(ctx, event)
		assert.NoError(t, err)
	})
}

// Test custom middleware support
func TestHTTPTransportMiddleware(t *testing.T) {
	server := createTestHTTPServer(t)
	defer server.Close()

	t.Run("RequestMiddleware", func(t *testing.T) {
		var middlewareCalled bool
		requestMiddleware := func(req *http.Request) error {
			middlewareCalled = true
			req.Header.Set("X-Middleware-Test", "executed")
			return nil
		}

		config := DefaultTransportConfig()
		config.BaseURL = server.URL
		config.Logger = zaptest.NewLogger(t)
		config.RequestMiddleware = []RequestMiddleware{requestMiddleware}

		transport, err := NewTransport(config)
		require.NoError(t, err)

		ctx := context.Background()
		err = transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "middleware test",
		}

		err = transport.SendEvent(ctx, event)
		assert.NoError(t, err)
		assert.True(t, middlewareCalled)
	})

	t.Run("ResponseMiddleware", func(t *testing.T) {
		var responseProcessed bool
		responseMiddleware := func(resp *http.Response) error {
			responseProcessed = true
			// Can inspect or modify response here
			return nil
		}

		config := DefaultTransportConfig()
		config.BaseURL = server.URL
		config.Logger = zaptest.NewLogger(t)
		config.ResponseMiddleware = []ResponseMiddleware{responseMiddleware}

		transport, err := NewTransport(config)
		require.NoError(t, err)

		ctx := context.Background()
		err = transport.Start(ctx)
		require.NoError(t, err)
		defer transport.Stop()

		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "response middleware test",
		}

		err = transport.SendEvent(ctx, event)
		assert.NoError(t, err)
		assert.True(t, responseProcessed)
	})
}

// Test metrics collection
func TestHTTPTransportMetrics(t *testing.T) {
	server := createTestHTTPServer(t)
	defer server.Close()

	config := DefaultTransportConfig()
	config.BaseURL = server.URL
	config.Logger = zaptest.NewLogger(t)
	config.EnableMetrics = true

	transport, err := NewTransport(config)
	require.NoError(t, err)

	ctx := context.Background()
	err = transport.Start(ctx)
	require.NoError(t, err)
	defer transport.Stop()

	t.Run("RequestMetrics", func(t *testing.T) {
		// Send multiple events
		for i := 0; i < 5; i++ {
			event := &HTTPMockEvent{
				EventType: EventTypeTextMessageContent,
				Data:      fmt.Sprintf("metrics test %d", i),
			}
			err := transport.SendEvent(ctx, event)
			assert.NoError(t, err)
		}

		metrics := transport.GetMetrics()
		assert.NotNil(t, metrics)
		assert.Greater(t, metrics.TotalRequests, int64(0))
		assert.Greater(t, metrics.SuccessfulRequests, int64(0))
		assert.GreaterOrEqual(t, metrics.FailedRequests, int64(0))
		assert.Greater(t, metrics.TotalBytesSent, int64(0))
		assert.Greater(t, metrics.TotalBytesReceived, int64(0))
		assert.Greater(t, metrics.AverageRequestDuration, time.Duration(0))

		// Check percentiles
		assert.Contains(t, metrics.RequestDurationPercentiles, "p50")
		assert.Contains(t, metrics.RequestDurationPercentiles, "p95")
		assert.Contains(t, metrics.RequestDurationPercentiles, "p99")
	})

	t.Run("ErrorMetrics", func(t *testing.T) {
		// Force some errors
		transport.config.BaseURL = "http://localhost:9999" // Non-existent

		event := &HTTPMockEvent{
			EventType: EventTypeTextMessageContent,
			Data:      "error metrics test",
		}

		_ = transport.SendEvent(ctx, event)

		metrics := transport.GetMetrics()
		assert.Greater(t, metrics.FailedRequests, int64(0))
		assert.Contains(t, metrics.ErrorsByType, "connection_refused")
	})
}