package transport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// DeterministicErrorTransport is a wrapper around ErrorTransport that fails based on event ID
type DeterministicErrorTransport struct {
	*ErrorTransport
	mu             sync.RWMutex
	failurePattern map[int]bool
}

func (t *DeterministicErrorTransport) Send(ctx context.Context, event TransportEvent) error {
	// Extract the numeric ID from the event ID (format: "test-N")
	idStr := event.ID()
	if strings.HasPrefix(idStr, "test-") {
		idPart := strings.TrimPrefix(idStr, "test-")
		var id int
		if _, err := fmt.Sscanf(idPart, "%d", &id); err == nil {
			t.mu.RLock()
			shouldFail := t.failurePattern[id]
			t.mu.RUnlock()

			if shouldFail {
				return errors.New("deterministic failure")
			}
		}
	}

	// Otherwise, use the underlying transport behavior
	return t.ErrorTransport.Send(ctx, event)
}

// TestSimpleManagerConnectionFailures tests various connection failure scenarios
func TestSimpleManagerConnectionFailures(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(*SimpleManager, *ErrorTransport)
		expectedError error
		checkFunc     func(*testing.T, *SimpleManager)
	}{
		{
			name: "transport_connect_error",
			setupFunc: func(m *SimpleManager, et *ErrorTransport) {
				et.SetConnectError(ErrConnectionFailed)
				m.SetTransport(et)
			},
			expectedError: ErrConnectionFailed,
			checkFunc: func(t *testing.T, m *SimpleManager) {
				if atomic.LoadInt32(&m.running) != 0 {
					t.Error("Manager should not be running after failed start")
				}
			},
		},
		{
			name: "transport_connect_timeout",
			setupFunc: func(m *SimpleManager, et *ErrorTransport) {
				et.connectDelay = 200 * time.Millisecond
				m.SetTransport(et)
			},
			expectedError: context.DeadlineExceeded,
			checkFunc: func(t *testing.T, m *SimpleManager) {
				if atomic.LoadInt32(&m.running) != 0 {
					t.Error("Manager should not be running after timeout")
				}
			},
		},
		{
			name: "nil_transport_start",
			setupFunc: func(m *SimpleManager, et *ErrorTransport) {
				// Don't set any transport
			},
			expectedError: nil, // Should start successfully but with no transport
			checkFunc: func(t *testing.T, m *SimpleManager) {
				if atomic.LoadInt32(&m.running) != 1 {
					t.Error("Manager should be running even without transport")
				}
			},
		},
		{
			name: "custom_connection_error",
			setupFunc: func(m *SimpleManager, et *ErrorTransport) {
				et.SetConnectError(&ConnectionError{
					Endpoint: "ws://localhost:8080",
					Cause:    errors.New("network unreachable"),
				})
				m.SetTransport(et)
			},
			expectedError: nil, // We'll check the error type
			checkFunc: func(t *testing.T, m *SimpleManager) {
				// Should not be running after connection error
				if atomic.LoadInt32(&m.running) != 0 {
					t.Error("Manager should not be running after connection error")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewSimpleManager()
			transport := NewErrorTransport()

			tt.setupFunc(manager, transport)

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			err := manager.Start(ctx)

			if tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else if tt.name == "custom_connection_error" {
				var connErr *ConnectionError
				if !errors.As(err, &connErr) {
					t.Errorf("Expected ConnectionError type, got %T", err)
				}
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, manager)
			}

			// Cleanup
			manager.Stop(context.Background())
		})
	}
}

// TestSimpleManagerSendFailures tests send failure scenarios
func TestSimpleManagerSendFailures(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(*SimpleManager, *ErrorTransport)
		event         TransportEvent
		expectedError error
	}{
		{
			name: "send_without_transport",
			setupFunc: func(m *SimpleManager, et *ErrorTransport) {
				// Don't set transport
			},
			event:         &DemoEvent{id: "test-1", eventType: "demo"},
			expectedError: ErrNotConnected,
		},
		{
			name: "send_after_transport_removed",
			setupFunc: func(m *SimpleManager, et *ErrorTransport) {
				m.SetTransport(et)
				m.Start(context.Background())
				m.SetTransport(nil) // Remove transport
			},
			event:         &DemoEvent{id: "test-1", eventType: "demo"},
			expectedError: ErrNotConnected,
		},
		{
			name: "send_with_validation_error",
			setupFunc: func(m *SimpleManager, et *ErrorTransport) {
				m.SetTransport(et)
				m.SetValidationConfig(&ValidationConfig{
					Enabled:           true,
					AllowedEventTypes: []string{"allowed", "demo"}, // Only allow specific types
					FailFast:          true,                        // Fail on first error
				})
				m.Start(context.Background())
			},
			event:         &DemoEvent{id: "test-1", eventType: "forbidden"},
			expectedError: nil, // We'll check for validation error in content
		},
		{
			name: "send_transport_error",
			setupFunc: func(m *SimpleManager, et *ErrorTransport) {
				et.SetSendError(NewTransportError("websocket", "send", errors.New("broken pipe")))
				m.SetTransport(et)
				m.Start(context.Background())
			},
			event:         &DemoEvent{id: "test-1", eventType: "demo"},
			expectedError: nil, // We'll check error content
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewSimpleManager()
			transport := NewErrorTransport()

			tt.setupFunc(manager, transport)

			ctx := context.Background()
			err := manager.Send(ctx, tt.event)

			if tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else if tt.name == "send_transport_error" {
				var transportErr *TransportError
				if !errors.As(err, &transportErr) {
					t.Errorf("Expected TransportError type, got %T", err)
				}
			} else if tt.name == "send_with_validation_error" {
				if err == nil {
					t.Error("Expected validation error, got nil")
				} else {
					t.Logf("Got validation error (expected): %v", err)
				}
			}

			// Cleanup
			manager.Stop(context.Background())
		})
	}
}

// TestSimpleManagerConcurrentErrors tests concurrent error scenarios
func TestSimpleManagerConcurrentErrors(t *testing.T) {
	t.Run("concurrent_start_stop", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewErrorTransport()
		manager.SetTransport(transport)

		var wg sync.WaitGroup
		errorsChan := make(chan error, 20)

		// Launch concurrent starts
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx := context.Background()
				if err := manager.Start(ctx); err != nil && !errors.Is(err, ErrAlreadyConnected) {
					errorsChan <- fmt.Errorf("unexpected start error: %v", err)
				}
			}()
		}

		// Launch concurrent stops
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx := context.Background()
				if err := manager.Stop(ctx); err != nil {
					errorsChan <- fmt.Errorf("unexpected stop error: %v", err)
				}
			}()
		}

		wg.Wait()
		close(errorsChan)

		// Check for unexpected errors
		for err := range errorsChan {
			t.Error(err)
		}
	})

	t.Run("concurrent_transport_changes", func(t *testing.T) {
		manager := NewSimpleManager()
		manager.Start(context.Background())
		defer manager.Stop(context.Background())

		var wg sync.WaitGroup

		// Rapidly change transports
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				transport := NewErrorTransport()
				manager.SetTransport(transport)

				// Try to send
				event := &DemoEvent{id: fmt.Sprintf("concurrent-%d", id), eventType: "demo"}
				manager.Send(context.Background(), event)
			}(i)
		}

		wg.Wait()
	})

	t.Run("concurrent_send_with_errors", func(t *testing.T) {
		manager := NewSimpleManager()

		// Create a custom transport that deterministically fails on certain IDs
		baseTransport := NewErrorTransport()
		transport := &DeterministicErrorTransport{
			ErrorTransport: baseTransport,
			failurePattern: map[int]bool{
				2: true, 5: true, 8: true, 11: true, 14: true, 17: true, // Fail these IDs
			},
		}

		manager.SetTransport(transport)

		// Start the manager which will connect the transport
		ctx := context.Background()
		if err := manager.Start(ctx); err != nil {
			t.Fatalf("Failed to start manager: %v", err)
		}
		defer manager.Stop(ctx)

		var wg sync.WaitGroup
		successCount := int32(0)
		errorCount := int32(0)

		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				event := &DemoEvent{id: fmt.Sprintf("test-%d", id), eventType: "demo"}
				if err := manager.Send(context.Background(), event); err != nil {
					atomic.AddInt32(&errorCount, 1)
				} else {
					atomic.AddInt32(&successCount, 1)
				}
			}(i)
		}

		wg.Wait()

		// Should have some successes and some failures
		if atomic.LoadInt32(&successCount) == 0 {
			t.Error("Expected some successful sends")
		}
		if atomic.LoadInt32(&errorCount) == 0 {
			t.Error("Expected some failed sends")
		}

		// Log for debugging
		t.Logf("Success: %d, Errors: %d", atomic.LoadInt32(&successCount), atomic.LoadInt32(&errorCount))
	})
}

// TestSimpleManagerResourceCleanup tests resource cleanup on errors
func TestSimpleManagerResourceCleanup(t *testing.T) {
	t.Run("cleanup_after_connect_failure", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewErrorTransport()
		transport.SetConnectError(ErrConnectionFailed)

		manager.SetTransport(transport)

		ctx := context.Background()
		err := manager.Start(ctx)
		if !errors.Is(err, ErrConnectionFailed) {
			t.Errorf("Expected ErrConnectionFailed, got %v", err)
		}

		// Manager should be in clean state
		if atomic.LoadInt32(&manager.running) != 0 {
			t.Error("Manager should not be running")
		}

		// Should be able to start again with different transport
		newTransport := NewErrorTransport()
		manager.SetTransport(newTransport)

		if err := manager.Start(ctx); err != nil {
			t.Errorf("Should be able to start after previous failure: %v", err)
		}

		manager.Stop(ctx)
	})

	t.Run("cleanup_on_stop_error", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewErrorTransport()
		transport.SetCloseError(errors.New("close failed"))

		manager.SetTransport(transport)
		manager.Start(context.Background())

		ctx := context.Background()
		err := manager.Stop(ctx)

		// Even with close error, manager should complete stop
		if err == nil || err.Error() != "close failed" {
			t.Errorf("Expected close error, got %v", err)
		}

		// Manager should not be running
		if atomic.LoadInt32(&manager.running) != 0 {
			t.Error("Manager should not be running after stop")
		}
	})

	t.Run("goroutine_cleanup_on_transport_change", func(t *testing.T) {
		manager := NewSimpleManager()

		// Start with first transport
		transport1 := NewErrorTransport()
		manager.SetTransport(transport1)
		manager.Start(context.Background())

		// Change transport multiple times
		for i := 0; i < 5; i++ {
			transport := NewErrorTransport()
			manager.SetTransport(transport)
			time.Sleep(10 * time.Millisecond) // Let goroutines start/stop
		}

		// Stop and wait for cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		manager.Stop(ctx)

		// All goroutines should be cleaned up
		if !manager.waitForReceiveGoroutines(500 * time.Millisecond) {
			t.Error("Goroutines not cleaned up properly")
		}
	})
}

// TestSimpleManagerBackpressureErrors tests backpressure error scenarios
func TestSimpleManagerBackpressureErrors(t *testing.T) {
	t.Run("backpressure_event_overflow", func(t *testing.T) {
		// Create manager with small buffer and drop strategy
		config := BackpressureConfig{
			Strategy:      BackpressureDropNewest,
			BufferSize:    2, // Small buffer to force drops
			HighWaterMark: 0.8,
			LowWaterMark:  0.2,
			BlockTimeout:  50 * time.Millisecond,
			EnableMetrics: true,
		}

		manager := NewSimpleManagerWithBackpressure(config)
		transport := NewDemoTransport() // Echo transport
		manager.SetTransport(transport)

		ctx := context.Background()
		manager.Start(ctx)
		defer manager.Stop(ctx)

		// Send many events rapidly
		var sent int32
		var wg sync.WaitGroup

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				event := &DemoEvent{
					id:        fmt.Sprintf("backpressure-%d", id),
					eventType: "test",
				}
				if err := manager.Send(ctx, event); err == nil {
					atomic.AddInt32(&sent, 1)
				}
			}(i)
		}

		wg.Wait()

		// Give time for echo events to be processed
		time.Sleep(200 * time.Millisecond)

		// Check metrics
		metrics := manager.GetBackpressureMetrics()
		if metrics.EventsDropped == 0 && atomic.LoadInt32(&sent) > int32(config.BufferSize) {
			t.Error("Expected some events to be dropped due to backpressure")
		}

		t.Logf("Sent: %d, Dropped: %d", atomic.LoadInt32(&sent), metrics.EventsDropped)
	})

	t.Run("backpressure_block_timeout", func(t *testing.T) {
		config := BackpressureConfig{
			Strategy:      BackpressureBlock,
			BufferSize:    5,
			HighWaterMark: 0.8,
			LowWaterMark:  0.2,
			BlockTimeout:  50 * time.Millisecond,
			EnableMetrics: true,
		}

		manager := NewSimpleManagerWithBackpressure(config)
		transport := NewDemoTransport()
		manager.SetTransport(transport)

		ctx := context.Background()
		manager.Start(ctx)
		defer manager.Stop(ctx)

		// Fill the buffer by not consuming events
		for i := 0; i < 10; i++ {
			event := &DemoEvent{
				id:        fmt.Sprintf("fill-%d", i),
				eventType: "test",
			}
			go manager.Send(ctx, event)
		}

		// Give time for buffer to fill
		time.Sleep(100 * time.Millisecond)

		// Try to send when buffer is full
		event := &DemoEvent{id: "blocked", eventType: "test"}
		start := time.Now()
		err := transport.Send(ctx, event)
		duration := time.Since(start)

		// Should succeed but possibly with delay
		if err != nil {
			t.Logf("Send completed with error: %v", err)
		}

		// Check that blocking occurred
		if duration < 10*time.Millisecond {
			t.Log("Warning: Send completed too quickly, blocking may not have occurred")
		}

		metrics := manager.GetBackpressureMetrics()
		t.Logf("Backpressure metrics - Blocked: %d, Backpressure Active: %v",
			metrics.EventsBlocked, metrics.BackpressureActive)
	})
}

// TestSimpleManagerValidationErrors tests validation error scenarios
func TestSimpleManagerValidationErrors(t *testing.T) {
	tests := []struct {
		name          string
		config        *ValidationConfig
		event         TransportEvent
		expectedError error
		isIncoming    bool
	}{
		{
			name: "outgoing_size_validation",
			config: &ValidationConfig{
				Enabled:        true,
				MaxMessageSize: 50,
			},
			event: &DemoEvent{
				id:        "test",
				eventType: "demo",
				data:      map[string]interface{}{"content": string(make([]byte, 100))}, // Large data
			},
			expectedError: ErrInvalidMessageSize,
			isIncoming:    false,
		},
		{
			name: "outgoing_type_validation",
			config: &ValidationConfig{
				Enabled:           true,
				AllowedEventTypes: []string{"allowed", "demo"},
			},
			event: &DemoEvent{
				id:        "test",
				eventType: "forbidden",
			},
			expectedError: ErrInvalidEventType,
			isIncoming:    false,
		},
		{
			name: "outgoing_required_fields",
			config: &ValidationConfig{
				Enabled:        true,
				RequiredFields: []string{"user_id", "session_id"},
			},
			event: &DemoEvent{
				id:        "test",
				eventType: "demo",
				// Missing required fields
			},
			expectedError: ErrMissingRequiredFields,
			isIncoming:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewSimpleManagerWithValidation(
				BackpressureConfig{
					Strategy:   BackpressureNone,
					BufferSize: 100,
				},
				tt.config,
			)

			transport := NewErrorTransport()
			manager.SetTransport(transport)

			ctx := context.Background()
			manager.Start(ctx)
			defer manager.Stop(ctx)

			if !tt.isIncoming {
				err := manager.Send(ctx, tt.event)
				if !errors.Is(err, tt.expectedError) {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			}
		})
	}
}

// TestSimpleManagerTimeoutScenarios tests various timeout scenarios
func TestSimpleManagerTimeoutScenarios(t *testing.T) {
	t.Run("stop_with_channel_drain_timeout", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewDemoTransport()
		manager.SetTransport(transport)

		ctx := context.Background()
		manager.Start(ctx)

		// Fill event channel
		for i := 0; i < 200; i++ {
			// Convert DemoEvent to events.Event
			demoEvent := &DemoEvent{id: fmt.Sprintf("drain-%d", i), eventType: "test", timestamp: time.Now()}
			baseEvent := &events.BaseEvent{
				EventType: events.EventType(demoEvent.Type()),
			}
			baseEvent.SetTimestamp(demoEvent.Timestamp().UnixMilli())

			select {
			case manager.eventChan <- baseEvent:
			default:
				// Channel full
			}
		}

		// Stop with very short context
		stopCtx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// Should handle timeout gracefully
		err := manager.Stop(stopCtx)
		if err != nil {
			t.Errorf("Stop should not return error on drain timeout: %v", err)
		}

		// Manager should not be running
		if atomic.LoadInt32(&manager.running) != 0 {
			t.Error("Manager should not be running after stop")
		}
	})

	t.Run("stop_with_transport_close_timeout", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewErrorTransport()
		transport.SetCloseError(NewTemporaryError("websocket", "close", context.DeadlineExceeded))

		manager.SetTransport(transport)
		manager.Start(context.Background())

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := manager.Stop(ctx)
		if err == nil {
			t.Error("Expected error from transport close timeout")
		}

		// Manager should still be marked as stopped
		if atomic.LoadInt32(&manager.running) != 0 {
			t.Error("Manager should not be running even after close error")
		}
	})
}

// BenchmarkSimpleManagerErrorPaths benchmarks error handling performance
func BenchmarkSimpleManagerErrorPaths(b *testing.B) {
	b.Run("send_not_connected", func(b *testing.B) {
		manager := NewSimpleManager()
		event := &DemoEvent{id: "bench-1", eventType: "demo"}
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			manager.Send(ctx, event)
		}
	})

	b.Run("concurrent_start_stop", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			manager := NewSimpleManager()
			transport := NewErrorTransport()
			manager.SetTransport(transport)
			ctx := context.Background()

			for pb.Next() {
				manager.Start(ctx)
				manager.Stop(ctx)
			}
		})
	})

	b.Run("validation_errors", func(b *testing.B) {
		config := &ValidationConfig{
			Enabled:           true,
			AllowedEventTypes: []string{"allowed"},
		}

		manager := NewSimpleManagerWithValidation(
			BackpressureConfig{Strategy: BackpressureNone, BufferSize: 100},
			config,
		)

		transport := NewErrorTransport()
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())

		event := &DemoEvent{id: "bench-1", eventType: "forbidden"}
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			manager.Send(ctx, event)
		}
	})
}
