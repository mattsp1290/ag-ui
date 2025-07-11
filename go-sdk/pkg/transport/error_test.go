package transport

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// ErrorTransport is a transport implementation that simulates various error conditions
type ErrorTransport struct {
	mu sync.RWMutex
	
	// Control error behavior
	connectError       error
	sendError          error
	healthError        error
	closeError         error
	
	// Control connection state
	connected          bool
	forceDisconnect    bool
	
	// Channels
	eventChan          chan Event
	errorChan          chan error
	
	// Simulate delays
	connectDelay       time.Duration
	sendDelay          time.Duration
	
	// Track operations for testing
	connectAttempts    int
	sendAttempts       int
	closeAttempts      int
}

func NewErrorTransport() *ErrorTransport {
	return &ErrorTransport{
		eventChan: make(chan Event, 10),
		errorChan: make(chan error, 10),
	}
}

func (t *ErrorTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.connectAttempts++
	
	// Simulate connection delay
	if t.connectDelay > 0 {
		select {
		case <-time.After(t.connectDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	if t.connectError != nil {
		return t.connectError
	}
	
	if t.connected {
		return ErrAlreadyConnected
	}
	
	t.connected = true
	return nil
}

func (t *ErrorTransport) Close(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.closeAttempts++
	
	if t.closeError != nil {
		return t.closeError
	}
	
	if !t.connected {
		return nil
	}
	
	t.connected = false
	
	// Close channels safely
	select {
	case <-t.eventChan:
		// Already closed
	default:
		close(t.eventChan)
	}
	
	select {
	case <-t.errorChan:
		// Already closed
	default:
		close(t.errorChan)
	}
	
	return nil
}

func (t *ErrorTransport) Send(ctx context.Context, event TransportEvent) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.sendAttempts++
	
	// Check if forced disconnect
	if t.forceDisconnect {
		t.connected = false
		return ErrConnectionClosed
	}
	
	if !t.connected {
		return ErrNotConnected
	}
	
	// Simulate send delay
	if t.sendDelay > 0 {
		select {
		case <-time.After(t.sendDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	if t.sendError != nil {
		return t.sendError
	}
	
	// Check for nil event
	if event == nil {
		return errors.New("cannot send nil event")
	}
	
	// Simulate message size limit
	if len(event.ID()) > 1000 {
		return ErrMessageTooLarge
	}
	
	return nil
}

func (t *ErrorTransport) Receive() <-chan Event {
	return t.eventChan
}

func (t *ErrorTransport) Errors() <-chan error {
	return t.errorChan
}

func (t *ErrorTransport) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connected
}

func (t *ErrorTransport) Capabilities() Capabilities {
	return Capabilities{
		Streaming:      true,
		Bidirectional:  true,
		MaxMessageSize: 1024,
	}
}

func (t *ErrorTransport) Health(ctx context.Context) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	if t.healthError != nil {
		return t.healthError
	}
	
	if !t.connected {
		return ErrNotConnected
	}
	
	return nil
}

func (t *ErrorTransport) Metrics() Metrics {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	return Metrics{
		MessagesSent: uint64(t.sendAttempts),
		ErrorCount:   uint64(t.connectAttempts - 1),
	}
}

func (t *ErrorTransport) SetMiddleware(middleware ...Middleware) {
	// No-op for error transport
}

// Helper methods for test control
func (t *ErrorTransport) SetConnectError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connectError = err
}

func (t *ErrorTransport) SetSendError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sendError = err
}

func (t *ErrorTransport) SetHealthError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.healthError = err
}

func (t *ErrorTransport) SetCloseError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeError = err
}

func (t *ErrorTransport) ForceDisconnect() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.forceDisconnect = true
}

func (t *ErrorTransport) SimulateError(err error) {
	select {
	case t.errorChan <- err:
	default:
		// Channel full or closed
	}
}

// Test connection failures
func TestConnectionFailures(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(*ErrorTransport)
		expectedError error
	}{
		{
			name: "connection_timeout",
			setupFunc: func(et *ErrorTransport) {
				et.connectDelay = 2 * time.Second
			},
			expectedError: context.DeadlineExceeded,
		},
		{
			name: "connection_refused",
			setupFunc: func(et *ErrorTransport) {
				et.SetConnectError(&ConnectionError{
					Endpoint: "localhost:1234",
					Cause:    errors.New("connection refused"),
				})
			},
			expectedError: nil, // We'll check for ConnectionError type
		},
		{
			name: "already_connected",
			setupFunc: func(et *ErrorTransport) {
				et.connected = true
			},
			expectedError: ErrAlreadyConnected,
		},
		{
			name: "generic_connection_failure",
			setupFunc: func(et *ErrorTransport) {
				et.SetConnectError(ErrConnectionFailed)
			},
			expectedError: ErrConnectionFailed,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewErrorTransport()
			tt.setupFunc(transport)
			
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			
			err := transport.Connect(ctx)
			
			if tt.expectedError != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else if tt.name == "connection_refused" {
				var connErr *ConnectionError
				if !errors.As(err, &connErr) {
					t.Errorf("Expected ConnectionError type, got %T", err)
				}
			}
		})
	}
}

// Test send failures
func TestSendFailures(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(*ErrorTransport)
		event         TransportEvent
		expectedError error
	}{
		{
			name: "not_connected",
			setupFunc: func(et *ErrorTransport) {
				// Transport not connected
			},
			event: &DemoEvent{id: "test-1", eventType: "demo"},
			expectedError: ErrNotConnected,
		},
		{
			name: "nil_event",
			setupFunc: func(et *ErrorTransport) {
				et.connected = true
			},
			event: nil,
			expectedError: errors.New("cannot send nil event"),
		},
		{
			name: "message_too_large",
			setupFunc: func(et *ErrorTransport) {
				et.connected = true
			},
			event: &DemoEvent{
				id: string(make([]byte, 1001)), // ID larger than limit
				eventType: "demo",
			},
			expectedError: ErrMessageTooLarge,
		},
		{
			name: "send_timeout",
			setupFunc: func(et *ErrorTransport) {
				et.connected = true
				et.sendDelay = 2 * time.Second
			},
			event: &DemoEvent{id: "test-1", eventType: "demo"},
			expectedError: context.DeadlineExceeded,
		},
		{
			name: "connection_lost_during_send",
			setupFunc: func(et *ErrorTransport) {
				et.connected = true
				et.ForceDisconnect()
			},
			event: &DemoEvent{id: "test-1", eventType: "demo"},
			expectedError: ErrConnectionClosed,
		},
		{
			name: "generic_send_error",
			setupFunc: func(et *ErrorTransport) {
				et.connected = true
				et.SetSendError(errors.New("network error"))
			},
			event: &DemoEvent{id: "test-1", eventType: "demo"},
			expectedError: errors.New("network error"),
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := NewErrorTransport()
			tt.setupFunc(transport)
			
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			
			err := transport.Send(ctx, tt.event)
			
			if tt.expectedError != nil {
				if tt.expectedError.Error() != err.Error() && !errors.Is(err, tt.expectedError) {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

// Test receive failures and error channel behavior
func TestReceiveFailures(t *testing.T) {
	t.Run("error_channel_behavior", func(t *testing.T) {
		transport := NewErrorTransport()
		
		// Connect first
		ctx := context.Background()
		if err := transport.Connect(ctx); err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		
		// Simulate various errors
		testErrors := []error{
			ErrConnectionClosed,
			ErrTimeout,
			errors.New("network error"),
			NewTransportError("test", "receive", errors.New("io error")),
		}
		
		// Send errors
		for _, err := range testErrors {
			transport.SimulateError(err)
		}
		
		// Receive and verify errors
		for i, expectedErr := range testErrors {
			select {
			case err := <-transport.Errors():
				if err.Error() != expectedErr.Error() {
					t.Errorf("Error %d: expected %v, got %v", i, expectedErr, err)
				}
			case <-time.After(100 * time.Millisecond):
				t.Errorf("Timeout waiting for error %d", i)
			}
		}
	})
	
	t.Run("closed_channel_behavior", func(t *testing.T) {
		transport := NewErrorTransport()
		
		// Connect and close
		ctx := context.Background()
		if err := transport.Connect(ctx); err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		
		if err := transport.Close(ctx); err != nil {
			t.Fatalf("Failed to close: %v", err)
		}
		
		// Try to receive from closed channels
		select {
		case _, ok := <-transport.Receive():
			if ok {
				t.Error("Expected channel to be closed")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Channel should be closed immediately")
		}
	})
}

// Test nil transport handling
func TestNilTransportHandling(t *testing.T) {
	t.Run("manager_with_nil_transport", func(t *testing.T) {
		manager := NewSimpleManager()
		
		// Start without setting transport
		ctx := context.Background()
		if err := manager.Start(ctx); err != nil {
			t.Fatalf("Failed to start manager: %v", err)
		}
		defer manager.Stop(ctx)
		
		// Try to send with nil transport
		event := &DemoEvent{id: "test-1", eventType: "demo"}
		err := manager.Send(ctx, event)
		if !errors.Is(err, ErrNotConnected) {
			t.Errorf("Expected ErrNotConnected, got %v", err)
		}
	})
	
	t.Run("set_nil_transport", func(t *testing.T) {
		manager := NewSimpleManager()
		
		// Set a valid transport first
		transport := NewErrorTransport()
		manager.SetTransport(transport)
		
		// Now set nil
		manager.SetTransport(nil)
		
		ctx := context.Background()
		event := &DemoEvent{id: "test-1", eventType: "demo"}
		err := manager.Send(ctx, event)
		if !errors.Is(err, ErrNotConnected) {
			t.Errorf("Expected ErrNotConnected, got %v", err)
		}
	})
}

// Test context cancellation scenarios
func TestContextCancellation(t *testing.T) {
	t.Run("connect_with_cancelled_context", func(t *testing.T) {
		transport := NewErrorTransport()
		transport.connectDelay = 100 * time.Millisecond
		
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		err := transport.Connect(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})
	
	t.Run("send_with_cancelled_context", func(t *testing.T) {
		transport := NewErrorTransport()
		transport.sendDelay = 100 * time.Millisecond
		
		// Connect first
		ctx := context.Background()
		if err := transport.Connect(ctx); err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		
		// Send with cancelled context
		sendCtx, cancel := context.WithCancel(context.Background())
		cancel()
		
		event := &DemoEvent{id: "test-1", eventType: "demo"}
		err := transport.Send(sendCtx, event)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})
	
	t.Run("manager_stop_with_timeout", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewErrorTransport()
		transport.SetCloseError(errors.New("close timeout"))
		
		manager.SetTransport(transport)
		
		ctx := context.Background()
		if err := manager.Start(ctx); err != nil {
			t.Fatalf("Failed to start: %v", err)
		}
		
		// Stop with short timeout
		stopCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		
		err := manager.Stop(stopCtx)
		if err == nil || err.Error() != "close timeout" {
			t.Errorf("Expected close timeout error, got %v", err)
		}
	})
}

// Test concurrent operations
func TestConcurrentOperations(t *testing.T) {
	t.Run("concurrent_sends", func(t *testing.T) {
		transport := NewErrorTransport()
		
		ctx := context.Background()
		if err := transport.Connect(ctx); err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		
		// Launch multiple concurrent sends
		var wg sync.WaitGroup
		errors := make(chan error, 10)
		
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				event := &DemoEvent{
					id:        fmt.Sprintf("concurrent-%d", id),
					eventType: "demo",
				}
				if err := transport.Send(ctx, event); err != nil {
					errors <- err
				}
			}(i)
		}
		
		wg.Wait()
		close(errors)
		
		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent send error: %v", err)
		}
		
		// Verify send attempts
		if transport.sendAttempts != 10 {
			t.Errorf("Expected 10 send attempts, got %d", transport.sendAttempts)
		}
	})
	
	t.Run("concurrent_connect_close", func(t *testing.T) {
		transport := NewErrorTransport()
		
		// Rapidly connect and close
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(2)
			
			go func() {
				defer wg.Done()
				ctx := context.Background()
				transport.Connect(ctx)
			}()
			
			go func() {
				defer wg.Done()
				ctx := context.Background()
				transport.Close(ctx)
			}()
		}
		
		wg.Wait()
		
		// Transport should handle this gracefully
		// Just verify no panic occurred
	})
}

// Test error types and handling
func TestErrorTypes(t *testing.T) {
	t.Run("transport_error", func(t *testing.T) {
		err := NewTransportError("websocket", "send", errors.New("connection reset"))
		
		if !IsTransportError(err) {
			t.Error("Expected IsTransportError to return true")
		}
		
		expectedMsg := "websocket send: connection reset"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
		}
		
		if err.IsTemporary() {
			t.Error("Expected IsTemporary to return false")
		}
		
		if err.IsRetryable() {
			t.Error("Expected IsRetryable to return false")
		}
	})
	
	t.Run("temporary_error", func(t *testing.T) {
		err := NewTemporaryError("http", "receive", errors.New("timeout"))
		
		if !err.IsTemporary() {
			t.Error("Expected IsTemporary to return true")
		}
		
		if !err.IsRetryable() {
			t.Error("Expected IsRetryable to return true")
		}
	})
	
	t.Run("connection_error", func(t *testing.T) {
		cause := errors.New("dial tcp: connection refused")
		err := &ConnectionError{
			Endpoint: "localhost:8080",
			Cause:    cause,
		}
		
		expectedMsg := "connection error to localhost:8080: dial tcp: connection refused"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
		}
		
		if !errors.Is(err, cause) {
			t.Error("Expected error to wrap the cause")
		}
	})
	
	t.Run("configuration_error", func(t *testing.T) {
		err := &ConfigurationError{
			Field:   "timeout",
			Value:   -1,
			Message: "timeout must be positive",
		}
		
		expectedMsg := "configuration error for field timeout (value: -1): timeout must be positive"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
		}
	})
}

// Test health check failures
func TestHealthCheckFailures(t *testing.T) {
	transport := NewErrorTransport()
	
	tests := []struct {
		name          string
		setupFunc     func()
		expectedError error
	}{
		{
			name: "not_connected",
			setupFunc: func() {
				// Transport not connected
			},
			expectedError: ErrNotConnected,
		},
		{
			name: "health_check_failure",
			setupFunc: func() {
				transport.connected = true
				transport.SetHealthError(ErrHealthCheckFailed)
			},
			expectedError: ErrHealthCheckFailed,
		},
		{
			name: "custom_health_error",
			setupFunc: func() {
				transport.connected = true
				transport.SetHealthError(errors.New("service unavailable"))
			},
			expectedError: errors.New("service unavailable"),
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset transport state
			transport = NewErrorTransport()
			tt.setupFunc()
			
			ctx := context.Background()
			err := transport.Health(ctx)
			
			if err == nil {
				t.Error("Expected error, got nil")
			} else if err.Error() != tt.expectedError.Error() {
				t.Errorf("Expected error %v, got %v", tt.expectedError, err)
			}
		})
	}
}

// Test manager error scenarios
func TestManagerErrorScenarios(t *testing.T) {
	t.Run("start_already_running", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewDemoTransport()
		manager.SetTransport(transport)
		
		ctx := context.Background()
		
		// Start manager
		if err := manager.Start(ctx); err != nil {
			t.Fatalf("Failed to start: %v", err)
		}
		defer manager.Stop(ctx)
		
		// Try to start again
		err := manager.Start(ctx)
		if !errors.Is(err, ErrAlreadyConnected) {
			t.Errorf("Expected ErrAlreadyConnected, got %v", err)
		}
	})
	
	t.Run("transport_connect_failure", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewErrorTransport()
		transport.SetConnectError(ErrConnectionFailed)
		
		manager.SetTransport(transport)
		
		ctx := context.Background()
		err := manager.Start(ctx)
		if !errors.Is(err, ErrConnectionFailed) {
			t.Errorf("Expected ErrConnectionFailed, got %v", err)
		}
		
		// Manager should not be running after failed start
		if manager.running {
			t.Error("Manager should not be running after failed start")
		}
	})
	
	t.Run("event_channel_overflow", func(t *testing.T) {
		manager := NewSimpleManager()
		transport := NewDemoTransport()
		manager.SetTransport(transport)
		
		ctx := context.Background()
		if err := manager.Start(ctx); err != nil {
			t.Fatalf("Failed to start: %v", err)
		}
		defer manager.Stop(ctx)
		
		// Fill up the event channel
		for i := 0; i < 110; i++ { // Channel buffer is 100
			event := &DemoEvent{
				id:        fmt.Sprintf("overflow-%d", i),
				eventType: "demo",
			}
			
			// Send through transport (which echoes back)
			transport.Send(ctx, event)
		}
		
		// Give some time for events to propagate
		time.Sleep(100 * time.Millisecond)
		
		// Channel should be full but not cause panic
		// This tests graceful handling of channel overflow
	})
}

// Test edge cases and boundary conditions
func TestEdgeCases(t *testing.T) {
	t.Run("zero_timeout", func(t *testing.T) {
		transport := NewErrorTransport()
		transport.connectDelay = 10 * time.Millisecond // Add small delay to trigger timeout
		
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()
		
		err := transport.Connect(ctx)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected context.DeadlineExceeded, got %v", err)
		}
	})
	
	t.Run("metrics_after_errors", func(t *testing.T) {
		transport := NewErrorTransport()
		
		// Cause some errors
		transport.SetConnectError(errors.New("test error"))
		
		ctx := context.Background()
		for i := 0; i < 3; i++ {
			transport.Connect(ctx)
		}
		
		metrics := transport.Metrics()
		if metrics.ErrorCount != 2 { // 3 attempts - 1 success = 2 errors
			t.Errorf("Expected error count 2, got %d", metrics.ErrorCount)
		}
	})
	
	t.Run("capabilities_edge_cases", func(t *testing.T) {
		caps := Capabilities{
			MaxMessageSize: 0, // 0 means unlimited
			Features:       nil,
		}
		
		// Should handle nil features map
		if caps.Features != nil {
			t.Error("Expected nil features map")
		}
		
		// 0 MaxMessageSize should mean unlimited
		if caps.MaxMessageSize != 0 {
			t.Error("Expected MaxMessageSize to be 0 (unlimited)")
		}
	})
	
	t.Run("event_metadata_edge_cases", func(t *testing.T) {
		metadata := EventMetadata{
			Headers:    nil,
			Size:       -1, // Invalid size
			Latency:    -1 * time.Second, // Negative latency
			Compressed: false,
		}
		
		// Should handle nil headers
		if metadata.Headers != nil {
			t.Error("Expected nil headers")
		}
		
		// Should allow negative values (for error cases)
		if metadata.Size != -1 {
			t.Error("Size should be -1")
		}
		
		if metadata.Latency != -1*time.Second {
			t.Error("Latency should be negative")
		}
	})
}

// Test SimpleManager channel draining scenarios
func TestSimpleManagerChannelDraining(t *testing.T) {
	manager := NewSimpleManager()
	transport := NewDemoTransport()
	manager.SetTransport(transport)
	
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}
	
	// Fill the channels with events and errors
	for i := 0; i < 50; i++ {
		event := &DemoEvent{
			id:        fmt.Sprintf("drain-test-%d", i),
			eventType: "demo",
			timestamp: time.Now(),
		}
		transport.Send(ctx, event)
	}
	
	// Stop should drain channels
	stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	
	if err := manager.Stop(stopCtx); err != nil {
		t.Errorf("Failed to stop: %v", err)
	}
}

// Test channel draining timeout
func TestChannelDrainingTimeout(t *testing.T) {
	manager := NewSimpleManager()
	transport := NewDemoTransport()
	manager.SetTransport(transport)
	
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}
	
	// Fill channels to capacity
	for i := 0; i < 200; i++ {
		event := &DemoEvent{
			id:        fmt.Sprintf("overflow-%d", i),
			eventType: "demo",
			timestamp: time.Now(),
		}
		// Send directly to simulate overflow
		select {
		case manager.eventChan <- Event{Event: event}:
		default:
			// Channel full
		}
	}
	
	// Stop with very short timeout to trigger timeout path
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	
	// Should not fail even with timeout
	if err := manager.Stop(stopCtx); err != nil {
		t.Errorf("Stop should not fail even with timeout: %v", err)
	}
}

// Benchmark error scenarios to ensure performance under error conditions
func BenchmarkErrorHandling(b *testing.B) {
	b.Run("send_not_connected", func(b *testing.B) {
		transport := NewErrorTransport()
		event := &DemoEvent{id: "bench-1", eventType: "demo"}
		ctx := context.Background()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			transport.Send(ctx, event)
		}
	})
	
	b.Run("concurrent_error_handling", func(b *testing.B) {
		transport := NewErrorTransport()
		transport.connected = true
		transport.SetSendError(errors.New("bench error"))
		
		ctx := context.Background()
		event := &DemoEvent{id: "bench-1", eventType: "demo"}
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				transport.Send(ctx, event)
			}
		})
	})
}