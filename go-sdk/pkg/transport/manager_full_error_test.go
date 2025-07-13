package transport

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestManagerConnectionErrors tests connection error scenarios for the full Manager
func TestManagerConnectionErrors(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(*Manager, *ErrorTransport)
		expectedError error
	}{
		{
			name: "start_already_running",
			setupFunc: func(m *Manager, et *ErrorTransport) {
				// Start the manager first
				atomic.StoreInt32(&m.running, 1)
			},
			expectedError: fmt.Errorf("transport manager already running"),
		},
		{
			name: "transport_crash_during_operation",
			setupFunc: func(m *Manager, et *ErrorTransport) {
				m.SetTransport(et)
				// Transport will disconnect after connect
				et.ForceDisconnect()
			},
			expectedError: nil, // Manager starts but transport fails later
		},
		{
			name: "nil_config_handling",
			setupFunc: func(m *Manager, et *ErrorTransport) {
				// Manager created with nil config should use defaults
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var manager *Manager
			if tt.name == "nil_config_handling" {
				manager = NewManager(nil)
			} else {
				manager = NewManager(&Config{
					Primary:    "websocket",
					BufferSize: 100,
				})
			}
			
			transport := NewErrorTransport()
			tt.setupFunc(manager, transport)

			ctx := context.Background()
			err := manager.Start(ctx)

			if tt.expectedError != nil {
				if err == nil || err.Error() != tt.expectedError.Error() {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			// Cleanup
			manager.Stop(context.Background())
		})
	}
}

// TestManagerSendErrors tests send error scenarios for the full Manager
func TestManagerSendErrors(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(*Manager, *ErrorTransport)
		event         TransportEvent
		expectedError error
		checkLogs     bool
	}{
		{
			name: "send_no_transport",
			setupFunc: func(m *Manager, et *ErrorTransport) {
				// Don't set any transport
			},
			event:         &DemoEvent{id: "test-1", eventType: "demo"},
			expectedError: ErrNotConnected,
			checkLogs:     true,
		},
		{
			name: "send_with_middleware_error",
			setupFunc: func(m *Manager, et *ErrorTransport) {
				m.SetTransport(et)
				// Add middleware that returns error
				m.AddMiddleware(MiddlewareFunc(func(next Transport) Transport {
					return &errorMiddleware{
						next:     next,
						sendErr:  errors.New("middleware error"),
					}
				}))
			},
			event:         &DemoEvent{id: "test-1", eventType: "demo"},
			expectedError: errors.New("middleware error"),
		},
		{
			name: "send_validation_failure",
			setupFunc: func(m *Manager, et *ErrorTransport) {
				// Connect the transport first so validation can happen
				et.Connect(context.Background())
				m.SetTransport(et)
				m.SetValidationConfig(&ValidationConfig{
					Enabled:           true,
					AllowedEventTypes: []string{"allowed"},
					FailFast:          true,
					CollectAllErrors:  true,
				})
			},
			event:         &DemoEvent{id: "test-1", eventType: "forbidden"},
			expectedError: ErrInvalidEventType,
		},
		{
			name: "send_after_stop",
			setupFunc: func(m *Manager, et *ErrorTransport) {
				m.SetTransport(et)
				m.Start(context.Background())
				m.Stop(context.Background())
			},
			event:         &DemoEvent{id: "test-1", eventType: "demo"},
			expectedError: ErrNotConnected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create manager with custom logger to capture logs
			var logBuffer sync.Map
			logger := &testLogger{logs: &logBuffer}
			
			manager := NewManagerWithLogger(&Config{
				Primary:       "websocket",
				BufferSize:    100,
				EnableMetrics: true,
			}, logger)
			
			transport := NewErrorTransport()
			tt.setupFunc(manager, transport)

			ctx := context.Background()
			err := manager.Send(ctx, tt.event)

			if tt.expectedError != nil {
				if err == nil || (err.Error() != tt.expectedError.Error() && !errors.Is(err, tt.expectedError)) {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			}

			if tt.checkLogs {
				// Verify error was logged
				hasErrorLog := false
				logBuffer.Range(func(key, value interface{}) bool {
					if log, ok := value.(string); ok && contains(log, "Cannot send event") {
						hasErrorLog = true
						return false
					}
					return true
				})
				
				if !hasErrorLog {
					t.Error("Expected error to be logged")
				}
			}

			// Cleanup
			manager.Stop(context.Background())
		})
	}
}

// TestManagerStopErrors tests stop error scenarios
func TestManagerStopErrors(t *testing.T) {
	t.Run("stop_with_transport_close_error", func(t *testing.T) {
		manager := NewManager(&Config{
			Primary:    "websocket",
			BufferSize: 100,
		})
		
		transport := NewErrorTransport()
		transport.SetCloseError(errors.New("transport close failed"))
		
		manager.SetTransport(transport)
		manager.Start(context.Background())

		ctx := context.Background()
		err := manager.Stop(ctx)
		
		if err == nil || err.Error() != "failed to close active transport: transport close failed" {
			t.Errorf("Expected transport close error, got %v", err)
		}

		// Manager should still be stopped
		if atomic.LoadInt32(&manager.running) != 0 {
			t.Error("Manager should not be running after stop with error")
		}
	})

	t.Run("stop_with_channel_drain_timeout", func(t *testing.T) {
		manager := NewManager(&Config{
			Primary:    "websocket",
			BufferSize: 100,
		})
		
		transport := NewDemoTransport()
		manager.SetTransport(transport)
		manager.Start(context.Background())

		// Fill channels
		for i := 0; i < 150; i++ {
			select {
			case manager.eventChan <- Event{Event: &DemoEvent{id: fmt.Sprintf("event-%d", i)}}:
			case manager.errorChan <- fmt.Errorf("error-%d", i):
			default:
			}
		}

		// Stop should handle drain timeout gracefully
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		
		err := manager.Stop(ctx)
		if err != nil {
			t.Errorf("Stop should succeed even with drain timeout: %v", err)
		}
	})

	t.Run("stop_not_running", func(t *testing.T) {
		manager := NewManager(&Config{
			Primary:    "websocket",
			BufferSize: 100,
		})
		
		// Stop without starting
		err := manager.Stop(context.Background())
		if err != nil {
			t.Errorf("Stop on non-running manager should succeed: %v", err)
		}
	})
}

// TestManagerReceiveErrors tests receive error scenarios
func TestManagerReceiveErrors(t *testing.T) {
	t.Run("receive_with_validation_errors", func(t *testing.T) {
		// Use a custom logger to capture debug info
		var logBuffer sync.Map
		logger := &testLogger{logs: &logBuffer}
		
		manager := NewManagerWithLogger(&Config{
			Primary:    "websocket",
			BufferSize: 100,
			Validation: &ValidationConfig{
				Enabled:           true,
				AllowedEventTypes: []string{"allowed"},
				RequiredFields:    []string{}, // Don't require fields to focus on event type validation
				FailFast:          true,
				CollectAllErrors:  true,
			},
		}, logger)
		
		transport := NewDemoTransport()
		// Connect the transport first
		err := transport.Connect(context.Background())
		if err != nil {
			t.Fatalf("Failed to connect transport: %v", err)
		}
		
		// Set transport and start manager
		manager.SetTransport(transport)
		err = manager.Start(context.Background())
		if err != nil {
			t.Fatalf("Failed to start manager: %v", err)
		}
		defer manager.Stop(context.Background())

		// Send invalid event through transport with proper data
		invalidEvent := &DemoEvent{
			id:        "invalid",
			eventType: "forbidden",
			timestamp: time.Now(),
			data:      make(map[string]interface{}), // Initialize data map
		}
		
		// Send event through transport - this should echo back through receiveEvents
		err = transport.Send(context.Background(), invalidEvent)
		if err != nil {
			t.Fatalf("Failed to send event through transport: %v", err)
		}

		// Give more time for event to be processed through the receiveEvents pipeline
		time.Sleep(300 * time.Millisecond)

		// Try to receive
		select {
		case event := <-manager.Receive():
			// Check validation metadata
			if event.Metadata.Headers == nil {
				t.Error("Expected headers to be initialized")
			} else {
				if event.Metadata.Headers["validation_failed"] != "true" {
					t.Errorf("Expected validation_failed header to be 'true', got %v", event.Metadata.Headers["validation_failed"])
				}
				if event.Metadata.Headers["validation_error"] == "" {
					t.Errorf("Expected validation_error header to be set, got %v", event.Metadata.Headers["validation_error"])
				}
			}
		case <-time.After(1 * time.Second):
			// Print debug logs if test fails
			t.Log("Debug logs:")
			logBuffer.Range(func(key, value interface{}) bool {
				t.Logf("  %v", value)
				return true
			})
			t.Error("Expected to receive event with validation error")
		}
	})

	t.Run("receive_backpressure_errors", func(t *testing.T) {
		config := &Config{
			Primary:    "websocket",
			BufferSize: 2, // Very small buffer
			Backpressure: BackpressureConfig{
				Strategy:      BackpressureDropNewest,
				BufferSize:    2, // Very small buffer to trigger backpressure quickly  
				HighWaterMark: 0.8,
				EnableMetrics: true,
			},
		}
		
		manager := NewManager(config)
		transport := NewDemoTransport()
		err := transport.Connect(context.Background())
		if err != nil {
			t.Fatalf("Failed to connect transport: %v", err)
		}
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())

		// Send more events rapidly and don't consume from manager.Receive()
		// This should cause the backpressure handler's buffer to fill up
		for i := 0; i < 20; i++ {
			event := &DemoEvent{
				id:        fmt.Sprintf("backpressure-%d", i),
				eventType: "test",
				timestamp: time.Now(),
				data:      make(map[string]interface{}),
			}
			// Send through transport - this will go through receiveEvents to backpressure handler
			// Ignore errors as some sends will fail when buffers are full
			transport.Send(context.Background(), event)
		}

		// Give time for events to propagate through receiveEvents -> backpressure handler
		time.Sleep(200 * time.Millisecond)

		// Check backpressure metrics
		metrics := manager.GetBackpressureMetrics()
		if metrics.EventsDropped == 0 {
			t.Errorf("Expected events to be dropped due to backpressure, but EventsDropped = %d", metrics.EventsDropped)
		}
	})
}

// TestManagerConcurrentErrors tests concurrent error scenarios
func TestManagerConcurrentErrors(t *testing.T) {
	t.Run("concurrent_transport_operations", func(t *testing.T) {
		manager := NewManager(&Config{
			Primary:    "websocket",
			BufferSize: 100,
		})

		var wg sync.WaitGroup
		errorCount := int32(0)

		// Concurrent starts
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := manager.Start(context.Background()); err != nil {
					atomic.AddInt32(&errorCount, 1)
				}
			}()
		}

		// Concurrent transport changes
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				transport := NewErrorTransport()
				manager.SetTransport(transport)
			}(i)
		}

		// Concurrent stops
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				manager.Stop(context.Background())
			}()
		}

		wg.Wait()

		// Should have some errors from concurrent starts
		if atomic.LoadInt32(&errorCount) == 0 {
			t.Error("Expected some errors from concurrent starts")
		}
	})

	t.Run("concurrent_send_receive_errors", func(t *testing.T) {
		manager := NewManager(&Config{
			Primary:    "websocket",
			BufferSize: 100,
		})
		
		transport := NewErrorTransport()
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())

		var wg sync.WaitGroup
		
		// Concurrent sends with intermittent errors
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				// Every third operation fails
				if id%3 == 0 {
					transport.SetSendError(errors.New("send error"))
				} else {
					transport.SetSendError(nil)
				}
				
				event := &DemoEvent{id: fmt.Sprintf("test-%d", id), eventType: "demo"}
				manager.Send(context.Background(), event)
			}(i)
		}

		// Concurrent error injection
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				transport.SimulateError(fmt.Errorf("simulated error %d", id))
			}(i)
		}

		wg.Wait()
		
		// Consume some errors
		errorCount := 0
		timeout := time.After(100 * time.Millisecond)
		
		for {
			select {
			case <-manager.Errors():
				errorCount++
			case <-timeout:
				if errorCount == 0 {
					t.Error("Expected to receive some errors")
				}
				return
			}
		}
	})
}

// TestManagerMetricsErrors tests metrics tracking during errors
func TestManagerMetricsErrors(t *testing.T) {
	manager := NewManager(&Config{
		Primary:       "websocket",
		BufferSize:    100,
		EnableMetrics: true,
	})
	
	transport := NewErrorTransport()
	// Connect the transport so it can accept Send() calls
	if err := transport.Connect(context.Background()); err != nil {
		t.Fatalf("Failed to connect transport: %v", err)
	}
	manager.SetTransport(transport)
	manager.Start(context.Background())
	defer manager.Stop(context.Background())

	// Send some successful and failed messages
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			transport.SetSendError(errors.New("metric test error"))
		} else {
			transport.SetSendError(nil)
		}
		
		event := &DemoEvent{id: fmt.Sprintf("metric-%d", i), eventType: "demo"}
		manager.Send(context.Background(), event)
	}

	metrics := manager.GetMetrics()
	
	// Should track successful sends
	if metrics.TotalMessagesSent == 0 {
		t.Error("Expected some messages to be sent")
	}
	
	// Verify metrics are properly copied
	metrics2 := manager.GetMetrics()
	if &metrics.TransportHealthScores == &metrics2.TransportHealthScores {
		t.Error("Metrics should be deep copied")
	}
}

// TestManagerErrorPropagation tests error propagation through the system
func TestManagerErrorPropagation(t *testing.T) {
	t.Run("transport_error_to_error_channel", func(t *testing.T) {
		manager := NewManager(&Config{
			Primary:    "websocket",
			BufferSize: 100,
		})
		
		transport := NewErrorTransport()
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())

		// Simulate various transport errors
		testErrors := []error{
			NewTransportError("websocket", "receive", errors.New("connection reset")),
			NewTemporaryError("websocket", "send", errors.New("buffer full")),
			&ConnectionError{Endpoint: "ws://localhost:8080", Cause: errors.New("refused")},
			ErrHealthCheckFailed,
		}

		for _, err := range testErrors {
			transport.SimulateError(err)
		}

		// Verify all errors are propagated
		for i, expectedErr := range testErrors {
			select {
			case err := <-manager.Errors():
				if err.Error() != expectedErr.Error() {
					t.Errorf("Error %d mismatch: expected %v, got %v", i, expectedErr, err)
				}
			case <-time.After(100 * time.Millisecond):
				t.Errorf("Timeout waiting for error %d", i)
			}
		}
	})

	t.Run("backpressure_error_propagation", func(t *testing.T) {
		config := &Config{
			Primary:    "websocket",
			BufferSize: 5,
			Backpressure: BackpressureConfig{
				Strategy:      BackpressureBlockWithTimeout,
				BufferSize:    5,
				BlockTimeout:  50 * time.Millisecond,
				EnableMetrics: true,
			},
		}
		
		// Create manager with test logger
		var logBuffer sync.Map
		logger := &testLogger{logs: &logBuffer}
		manager := NewManagerWithLogger(config, logger)
		
		transport := NewDemoTransport()
		
		// Connect transport first
		ctx := context.Background()
		err := transport.Connect(ctx)
		if err != nil {
			t.Fatalf("Failed to connect transport: %v", err)
		}
		
		manager.SetTransport(transport)
		manager.Start(ctx)
		defer manager.Stop(ctx)

		// Fill buffer to trigger backpressure - send many more events
		// to ensure the backpressure handler buffer (size 5) gets full
		for i := 0; i < 20; i++ {
			event := &DemoEvent{id: fmt.Sprintf("bp-%d", i), eventType: "test"}
			go transport.Send(ctx, event)
			time.Sleep(5 * time.Millisecond) // Small delay to allow processing
		}

		time.Sleep(200 * time.Millisecond)

		// Check for backpressure warnings in logs
		hasBackpressureLog := false
		logBuffer.Range(func(key, value interface{}) bool {
			if log, ok := value.(string); ok && contains(log, "backpressure") {
				hasBackpressureLog = true
				return false
			}
			return true
		})

		if !hasBackpressureLog {
			t.Error("Expected backpressure to be logged")
		}
	})
}

// TestManagerInvalidConfiguration tests invalid configuration handling
func TestManagerInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
	}{
		{
			name: "negative_buffer_size",
			config: &Config{
				Primary:    "websocket",
				BufferSize: -1,
			},
		},
		{
			name: "empty_primary_transport",
			config: &Config{
				Primary:    "",
				BufferSize: 100,
			},
		},
		{
			name: "invalid_backpressure_config",
			config: &Config{
				Primary:    "websocket",
				BufferSize: 100,
				Backpressure: BackpressureConfig{
					Strategy:      "invalid",
					BufferSize:    -1,
					HighWaterMark: 1.5, // > 1.0
					LowWaterMark:  -0.1, // < 0
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Manager should handle invalid config gracefully
			manager := NewManager(tt.config)
			
			// Should still be able to start/stop
			err := manager.Start(context.Background())
			if err != nil {
				t.Logf("Start with invalid config returned: %v", err)
			}
			
			manager.Stop(context.Background())
		})
	}
}

// errorMiddleware is a test middleware that returns errors
type errorMiddleware struct {
	next    Transport
	sendErr error
}

func (m *errorMiddleware) Connect(ctx context.Context) error {
	return m.next.Connect(ctx)
}

func (m *errorMiddleware) Close(ctx context.Context) error {
	return m.next.Close(ctx)
}

func (m *errorMiddleware) Send(ctx context.Context, event TransportEvent) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	return m.next.Send(ctx, event)
}

func (m *errorMiddleware) Receive() <-chan Event {
	return m.next.Receive()
}

func (m *errorMiddleware) Errors() <-chan error {
	return m.next.Errors()
}

func (m *errorMiddleware) IsConnected() bool {
	return m.next.IsConnected()
}

func (m *errorMiddleware) Capabilities() Capabilities {
	return m.next.Capabilities()
}

func (m *errorMiddleware) Health(ctx context.Context) error {
	return m.next.Health(ctx)
}

func (m *errorMiddleware) Metrics() Metrics {
	return m.next.Metrics()
}

func (m *errorMiddleware) SetMiddleware(middleware ...Middleware) {
	m.next.SetMiddleware(middleware...)
}

// testLogger captures logs for testing
type testLogger struct {
	logs *sync.Map
}

func (l *testLogger) Log(level LogLevel, message string, fields ...Field) {
	l.logInternal(level.String(), message, fields...)
}

func (l *testLogger) Debug(msg string, fields ...Field) {
	l.logInternal("DEBUG", msg, fields...)
}

func (l *testLogger) Info(msg string, fields ...Field) {
	l.logInternal("INFO", msg, fields...)
}

func (l *testLogger) Warn(msg string, fields ...Field) {
	l.logInternal("WARN", msg, fields...)
}

func (l *testLogger) Error(msg string, fields ...Field) {
	l.logInternal("ERROR", msg, fields...)
}

func (l *testLogger) WithFields(fields ...Field) Logger {
	return l // Simple implementation
}

func (l *testLogger) WithContext(ctx context.Context) Logger {
	return l // Simple implementation
}

// Type-safe logging methods (convert to Field and call legacy methods)
func (l *testLogger) LogTyped(level LogLevel, message string, fields ...FieldProvider) {
	legacyFields := make([]Field, len(fields))
	for i, field := range fields {
		legacyFields[i] = field.ToField()
	}
	l.Log(level, message, legacyFields...)
}

func (l *testLogger) DebugTyped(message string, fields ...FieldProvider) {
	legacyFields := make([]Field, len(fields))
	for i, field := range fields {
		legacyFields[i] = field.ToField()
	}
	l.Debug(message, legacyFields...)
}

func (l *testLogger) InfoTyped(message string, fields ...FieldProvider) {
	legacyFields := make([]Field, len(fields))
	for i, field := range fields {
		legacyFields[i] = field.ToField()
	}
	l.Info(message, legacyFields...)
}

func (l *testLogger) WarnTyped(message string, fields ...FieldProvider) {
	legacyFields := make([]Field, len(fields))
	for i, field := range fields {
		legacyFields[i] = field.ToField()
	}
	l.Warn(message, legacyFields...)
}

func (l *testLogger) ErrorTyped(message string, fields ...FieldProvider) {
	legacyFields := make([]Field, len(fields))
	for i, field := range fields {
		legacyFields[i] = field.ToField()
	}
	l.Error(message, legacyFields...)
}

func (l *testLogger) WithTypedFields(fields ...FieldProvider) Logger {
	return l // Simple implementation for testing
}

func (l *testLogger) logInternal(level, msg string, fields ...Field) {
	logEntry := fmt.Sprintf("[%s] %s", level, msg)
	for _, field := range fields {
		logEntry += fmt.Sprintf(" %s=%v", field.Key, field.Value)
	}
	l.logs.Store(time.Now().UnixNano(), logEntry)
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && s != "" && substr != "" && 
		(s == substr || (len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// BenchmarkManagerErrorHandling benchmarks error handling in the full manager
func BenchmarkManagerErrorHandling(b *testing.B) {
	b.Run("send_with_validation_error", func(b *testing.B) {
		manager := NewManager(&Config{
			Primary:    "websocket",
			BufferSize: 1000,
			Validation: &ValidationConfig{
				Enabled:           true,
				AllowedEventTypes: []string{"allowed"},
			},
		})
		
		transport := NewErrorTransport()
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())
		
		event := &DemoEvent{id: "bench", eventType: "forbidden"}
		ctx := context.Background()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			manager.Send(ctx, event)
		}
	})

	b.Run("concurrent_error_propagation", func(b *testing.B) {
		manager := NewManager(&Config{
			Primary:    "websocket",
			BufferSize: 1000,
		})
		
		transport := NewErrorTransport()
		transport.SetSendError(errors.New("bench error"))
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())
		
		ctx := context.Background()
		event := &DemoEvent{id: "bench", eventType: "test"}
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				manager.Send(ctx, event)
			}
		})
	})
}