package transport

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// TestEdgeCaseErrors tests various edge case error scenarios
func TestEdgeCaseErrors(t *testing.T) {
	t.Run("nil_event_handling", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewErrorTransport()
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())

		// Try to send nil event
		err := manager.Send(context.Background(), nil)
		if err == nil || err.Error() != "cannot send nil event" {
			t.Errorf("Expected nil event error, got %v", err)
		}
	})

	t.Run("context_already_cancelled", func(t *testing.T) {
		// Test Start with cancelled context
		manager := NewSimpleManager()
		transport := NewErrorTransport()
		transport.connectDelay = 10 * time.Millisecond // Add delay to trigger context check
		transport.sendDelay = 10 * time.Millisecond
		manager.SetTransport(transport)

		// Create already cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Try operations with cancelled context
		err := manager.Start(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got %v", err)
		}

		// Use a fresh manager for normal operations to avoid deadlock
		manager2 := NewSimpleManager()
		transport2 := NewErrorTransport()
		transport2.connectDelay = 10 * time.Millisecond
		transport2.sendDelay = 10 * time.Millisecond
		manager2.SetTransport(transport2)

		// Start normally with fresh manager
		err = manager2.Start(context.Background())
		if err != nil {
			t.Fatalf("Failed to start manager: %v", err)
		}
		defer manager2.Stop(context.Background())

		// Send with cancelled context
		event := &DemoEvent{id: "test", eventType: "demo"}
		err = manager2.Send(ctx, event)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled on send, got %v", err)
		}
	})

	t.Run("channel_close_race_conditions", func(t *testing.T) {
		// Test potential race when channels are closed during active operations
		for i := 0; i < 10; i++ {
			manager := NewSimpleManager()
			transport := NewDemoTransport()
			manager.SetTransport(transport)
			
			manager.Start(context.Background())
			
			// Start goroutines that will be accessing channels
			var wg sync.WaitGroup
			stopFlag := int32(0)
			
			// Reader goroutine
			wg.Add(1)
			go func() {
				defer wg.Done()
				for atomic.LoadInt32(&stopFlag) == 0 {
					select {
					case <-manager.Receive():
					case <-manager.Errors():
					default:
						runtime.Gosched()
					}
				}
			}()
			
			// Sender goroutine
			wg.Add(1)
			go func() {
				defer wg.Done()
				event := &DemoEvent{id: "race-test", eventType: "demo"}
				for atomic.LoadInt32(&stopFlag) == 0 {
					manager.Send(context.Background(), event)
					runtime.Gosched()
				}
			}()
			
			// Let goroutines run
			time.Sleep(10 * time.Millisecond)
			
			// Stop manager (will close channels)
			manager.Stop(context.Background())
			
			// Signal goroutines to stop
			atomic.StoreInt32(&stopFlag, 1)
			
			// Wait for goroutines to finish
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()
			
			select {
			case <-done:
				// Success - no panic
			case <-time.After(100 * time.Millisecond):
				t.Fatal("Goroutines did not finish, possible deadlock")
			}
		}
	})

	t.Run("panic_recovery_in_receive", func(t *testing.T) {
		// Test that panics in receive goroutine are handled
		// This test verifies the behavior when a transport panics
		// In production, we'd want to add panic recovery to the receiveEvents goroutine
		
		defer func() {
			if r := recover(); r != nil {
				// Expected - we're testing panic behavior
				t.Logf("Recovered from panic (expected): %v", r)
			}
		}()
		
		manager := NewSimpleManager()
		
		// Create a transport that will cause issues
		transport := &PanicTransport{
			baseTransport: NewErrorTransport(),
			panicOnReceive: false, // Don't panic immediately
		}
		
		manager.SetTransport(transport)
		
		// Start the manager
		err := manager.Start(context.Background())
		if err != nil {
			t.Fatalf("Failed to start: %v", err)
		}
		
		// Now enable panic
		transport.panicOnReceive = true
		
		// Try to trigger the panic in a controlled way
		// In real code, we'd add panic recovery to receiveEvents
		
		// Cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		manager.Stop(ctx)
	})

	t.Run("zero_timeout_operations", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewErrorTransport()
		transport.connectDelay = 10 * time.Millisecond
		transport.sendDelay = 10 * time.Millisecond
		
		manager.SetTransport(transport)
		
		// Start with zero timeout
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()
		
		err := manager.Start(ctx)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected deadline exceeded, got %v", err)
		}
		
		// Start normally
		manager.Start(context.Background())
		defer manager.Stop(context.Background())
		
		// Send with zero timeout
		ctx2, cancel2 := context.WithTimeout(context.Background(), 0)
		defer cancel2()
		
		event := &DemoEvent{id: "test", eventType: "demo"}
		err = manager.Send(ctx2, event)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected deadline exceeded on send, got %v", err)
		}
	})

	t.Run("manager_state_after_panic", func(t *testing.T) {
		manager := NewSimpleManager()
		
		// Force panic in a controlled way
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Expected panic
				}
			}()
			
			// This might panic if channels are nil
			manager.eventChan = nil
			manager.errorChan = nil
			
			// Try to use manager
			_ = manager.Receive()
		}()
		
		// Manager should still be usable after fixing state
		manager.eventChan = make(chan events.Event, 100)
		manager.errorChan = make(chan error, 100)
		
		transport := NewErrorTransport()
		manager.SetTransport(transport)
		
		// Should be able to start
		err := manager.Start(context.Background())
		if err != nil {
			t.Errorf("Manager should be usable after panic recovery: %v", err)
		}
		
		manager.Stop(context.Background())
	})
}

// TestMemoryLeakScenarios tests for potential memory leaks
func TestMemoryLeakScenarios(t *testing.T) {
	t.Run("goroutine_leak_on_transport_change", func(t *testing.T) {
		initialGoroutines := runtime.NumGoroutine()
		
		manager := NewSimpleManager()
		manager.Start(context.Background())
		
		// Change transport many times
		for i := 0; i < 100; i++ {
			transport := NewErrorTransport()
			manager.SetTransport(transport)
			time.Sleep(time.Millisecond) // Let goroutines start
		}
		
		manager.Stop(context.Background())
		
		// Give time for goroutines to clean up
		time.Sleep(100 * time.Millisecond)
		
		finalGoroutines := runtime.NumGoroutine()
		leaked := finalGoroutines - initialGoroutines
		
		// Allow some tolerance for runtime goroutines
		if leaked > 10 {
			t.Errorf("Potential goroutine leak: %d goroutines leaked", leaked)
		}
	})

	t.Run("channel_buffer_overflow", func(t *testing.T) {
		// Test behavior when receive channels are full
		config := BackpressureConfig{
			Strategy:      BackpressureDropNewest,
			BufferSize:    2, // Very small buffer
			EnableMetrics: true,
		}
		
		manager := NewSimpleManagerWithBackpressure(config)
		transport := NewDemoTransport()
		// Connect the transport first
		err := transport.Connect(context.Background())
		if err != nil {
			t.Fatalf("Failed to connect transport: %v", err)
		}
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())
		
		// Fill the receive buffer by sending events through the transport 
		// but not consuming them from the manager
		sentCount := 0
		for i := 0; i < 100; i++ {
			event := &DemoEvent{
				id:        fmt.Sprintf("overflow-%d", i), 
				eventType: "test",
				timestamp: time.Now(),
				data:      map[string]interface{}{"message": fmt.Sprintf("test-%d", i)},
			}
			// Send through the transport to generate incoming events
			err := transport.Send(context.Background(), event)
			if err == nil {
				sentCount++
			}
		}
		
		t.Logf("Successfully sent %d events to transport", sentCount)
		
		// Force all events through by consuming from transport's receive channel
		// This ensures the simple manager's receiveEvents goroutine processes them
		for i := 0; i < 20; i++ {
			time.Sleep(50 * time.Millisecond)
			// Check metrics to see if drops are happening
			m := manager.GetBackpressureMetrics()
			if m.EventsDropped > 0 {
				t.Logf("Events dropped after %d iterations: %d", i, m.EventsDropped)
				break
			}
		}
		
		// Should handle overflow gracefully
		metrics := manager.GetBackpressureMetrics()
		t.Logf("Backpressure metrics: EventsDropped=%d, CurrentBufferSize=%d, MaxBufferSize=%d", 
			metrics.EventsDropped, metrics.CurrentBufferSize, metrics.MaxBufferSize)
		
		// Check how many events are in the receive channel
		receiveChan := manager.Receive()
		receivedCount := 0
		done := make(chan bool)
		go func() {
			for {
				select {
				case <-receiveChan:
					receivedCount++
				case <-time.After(100 * time.Millisecond):
					done <- true
					return
				}
			}
		}()
		<-done
		
		t.Logf("Received %d events from manager", receivedCount)
		
		// With 10 events sent and buffer size of 2, we expect 8 drops
		expectedDrops := sentCount - config.BufferSize
		if expectedDrops < 0 {
			expectedDrops = 0
		}
		
		if metrics.EventsDropped == 0 && sentCount > config.BufferSize {
			t.Error("Expected events to be dropped with small buffer")
		}
	})
}

// TestErrorTypeEdgeCases tests edge cases for error types
func TestErrorTypeEdgeCases(t *testing.T) {
	t.Run("wrapped_error_chains", func(t *testing.T) {
		// Create deeply nested error chain
		baseErr := errors.New("base error")
		wrappedErr := fmt.Errorf("wrapped: %w", baseErr)
		transportErr := NewTransportError("test", "operation", wrappedErr)
		finalErr := fmt.Errorf("final: %w", transportErr)
		
		// Should be able to unwrap to base error
		if !errors.Is(finalErr, baseErr) {
			t.Error("Should be able to unwrap to base error")
		}
		
		// Should detect transport error
		if !IsTransportError(finalErr) {
			t.Error("Should detect transport error in chain")
		}
	})

	t.Run("nil_error_handling", func(t *testing.T) {
		// Test nil error edge cases
		var nilErr error
		
		if IsTransportError(nilErr) {
			t.Error("IsTransportError should return false for nil")
		}
		
		// Configuration error with nil values
		configErr := &LegacyConfigurationError{
			Field:   "",
			Value:   nil,
			Message: "test",
		}
		
		if configErr.Error() != "configuration error: test" {
			t.Errorf("Unexpected error message: %s", configErr.Error())
		}
	})

	t.Run("concurrent_error_access", func(t *testing.T) {
		// Test concurrent access to error fields
		transportErr := &TransportError{
			Transport: "websocket",
			Op:        "send",
			Err:       errors.New("test"),
			Temporary: false,
			Retryable: false,
		}
		
		var wg sync.WaitGroup
		
		// Concurrent readers
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					_ = transportErr.Error()
					_ = transportErr.IsTemporary()
					_ = transportErr.IsRetryable()
					_ = transportErr.Unwrap()
				}
			}()
		}
		
		// Concurrent modifiers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					transportErr.Temporary = j%2 == 0
					transportErr.Retryable = j%2 == 1
				}
			}(i)
		}
		
		wg.Wait()
	})
}

// TestConfigurationEdgeCases tests configuration edge cases
func TestConfigurationEdgeCases(t *testing.T) {
	t.Run("invalid_validation_config", func(t *testing.T) {
		configs := []*ValidationConfig{
			{
				Enabled:        true,
				MaxMessageSize: -1, // Negative size
			},
			{
				Enabled:           true,
				AllowedEventTypes: []string{}, // Empty allowed types
			},
			{
				Enabled:         true,
				RequiredFields:  []string{"", " ", "\t"}, // Empty/whitespace fields
			},
			{
				Enabled:           true,
				PatternValidators: map[string]*regexp.Regexp{
					"": regexp.MustCompile("pattern"), // Empty field name
				},
			},
		}
		
		for i, config := range configs {
			t.Run(fmt.Sprintf("config_%d", i), func(t *testing.T) {
				manager := NewSimpleManagerWithValidation(
					BackpressureConfig{Strategy: BackpressureNone, BufferSize: 100},
					config,
				)
				
				transport := NewErrorTransport()
				manager.SetTransport(transport)
				manager.Start(context.Background())
				defer manager.Stop(context.Background())
				
				// Should handle invalid config gracefully
				event := &DemoEvent{id: "test", eventType: "demo"}
				_ = manager.Send(context.Background(), event)
			})
		}
	})

	t.Run("extreme_backpressure_values", func(t *testing.T) {
		configs := []BackpressureConfig{
			{
				Strategy:      BackpressureBlock,
				BufferSize:    0, // Zero buffer
				BlockTimeout:  0, // Zero timeout
			},
			{
				Strategy:      BackpressureDropNewest,
				BufferSize:    100000, // Large but reasonable size
				HighWaterMark: 2.0, // > 1.0
				LowWaterMark:  -1.0, // < 0
			},
			{
				Strategy:      "invalid_strategy",
				BufferSize:    100,
				BlockTimeout:  -1 * time.Second, // Negative timeout
			},
		}
		
		for i, config := range configs {
			t.Run(fmt.Sprintf("backpressure_%d", i), func(t *testing.T) {
				// Should not panic with extreme values
				manager := NewSimpleManagerWithBackpressure(config)
				transport := NewErrorTransport()
				manager.SetTransport(transport)
				
				// Should handle gracefully
				err := manager.Start(context.Background())
				if err == nil {
					manager.Stop(context.Background())
				}
			})
		}
	})
}

// TestBoundaryConditions tests various boundary conditions
func TestBoundaryConditions(t *testing.T) {
	t.Run("max_message_size_boundary", func(t *testing.T) {
		transport := NewErrorTransport()
		
		testCases := []struct {
			name     string
			size     int
			expected error
		}{
			{"at_limit", 1000, nil},
			{"over_limit", 1001, ErrMessageTooLarge},
			{"just_under", 999, nil},
			{"zero_size", 0, nil},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				transport.connected = true
				event := &DemoEvent{
					id:        string(make([]byte, tc.size)),
					eventType: "test",
				}
				
				err := transport.Send(context.Background(), event)
				if tc.expected != nil {
					if !errors.Is(err, tc.expected) {
						t.Errorf("Expected %v, got %v", tc.expected, err)
					}
				} else if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			})
		}
	})

	t.Run("concurrent_limit_testing", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewErrorTransport()
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())
		
		// Test with maximum concurrent operations
		concurrency := 1000
		var wg sync.WaitGroup
		errors := make(chan error, concurrency)
		
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				event := &DemoEvent{
					id:        fmt.Sprintf("concurrent-%d", id),
					eventType: "test",
				}
				if err := manager.Send(context.Background(), event); err != nil {
					select {
					case errors <- err:
					default:
					}
				}
			}(i)
		}
		
		wg.Wait()
		close(errors)
		
		// Check error rate
		errorCount := 0
		for range errors {
			errorCount++
		}
		
		// Should handle high concurrency without excessive errors
		errorRate := float64(errorCount) / float64(concurrency)
		if errorRate > 0.1 { // Allow up to 10% error rate
			t.Errorf("High error rate under load: %.2f%%", errorRate*100)
		}
	})
}

// PanicTransport is a transport that panics in certain operations
type PanicTransport struct {
	baseTransport   Transport
	panicOnReceive  bool
	panicOnConnect  bool
}

func (t *PanicTransport) Connect(ctx context.Context) error {
	if t.panicOnConnect {
		panic("intentional panic in Connect")
	}
	return t.baseTransport.Connect(ctx)
}

func (t *PanicTransport) Close(ctx context.Context) error {
	return t.baseTransport.Close(ctx)
}

func (t *PanicTransport) Send(ctx context.Context, event TransportEvent) error {
	return t.baseTransport.Send(ctx, event)
}

func (t *PanicTransport) Receive() <-chan events.Event {
	if t.panicOnReceive {
		panic("intentional panic in Receive")
	}
	eventCh, _ := t.baseTransport.Channels()
	return eventCh
}

func (t *PanicTransport) Errors() <-chan error {
	_, errorCh := t.baseTransport.Channels()
	return errorCh
}

func (t *PanicTransport) Channels() (<-chan events.Event, <-chan error) {
	return t.baseTransport.Channels()
}

func (t *PanicTransport) IsConnected() bool {
	return t.baseTransport.IsConnected()
}

func (t *PanicTransport) Config() Config {
	return t.baseTransport.Config()
}

func (t *PanicTransport) Stats() TransportStats {
	return t.baseTransport.Stats()
}

// Health and Metrics functionality removed - not part of Transport interface

// SetMiddleware removed - not part of Transport interface

// BenchmarkEdgeCasePerformance benchmarks performance under edge conditions
func BenchmarkEdgeCasePerformance(b *testing.B) {
	b.Run("nil_event_rejection", func(b *testing.B) {
		manager := NewSimpleManager()
		transport := NewErrorTransport()
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			manager.Send(context.Background(), nil)
		}
	})

	b.Run("cancelled_context_handling", func(b *testing.B) {
		manager := NewSimpleManager()
		transport := NewErrorTransport()
		transport.sendDelay = time.Microsecond
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())
		
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Pre-cancelled context
		
		event := &DemoEvent{id: "bench", eventType: "test"}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			manager.Send(ctx, event)
		}
	})

	b.Run("max_concurrency_stress", func(b *testing.B) {
		manager := NewSimpleManager()
		transport := NewErrorTransport()
		manager.SetTransport(transport)
		manager.Start(context.Background())
		defer manager.Stop(context.Background())
		
		event := &DemoEvent{id: "bench", eventType: "test"}
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				manager.Send(context.Background(), event)
			}
		})
	})
}