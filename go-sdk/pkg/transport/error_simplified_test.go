package transport

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

)

// TestConnectionFailuresSimplified demonstrates how test infrastructure simplifies error testing
func TestConnectionFailuresSimplified(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(*MockTransport)
		expectedError error
	}{
		{
			name: "connection_timeout",
			setupFunc: func(mt *MockTransport) {
				mt.SetConnectBehavior(func(ctx context.Context) error {
					select {
					case <-time.After(2 * time.Second):
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
			},
			expectedError: context.DeadlineExceeded,
		},
		{
			name: "connection_refused",
			setupFunc: func(mt *MockTransport) {
				mt.SetConnectBehavior(func(ctx context.Context) error {
					return &ConnectionError{
						Endpoint: "localhost:1234",
						Cause:    errors.New("connection refused"),
					}
				})
			},
			expectedError: nil, // We'll check for ConnectionError type
		},
		{
			name: "already_connected",
			setupFunc: func(mt *MockTransport) {
				mt.connected.Store(true)
			},
			expectedError: ErrAlreadyConnected,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use test fixture for cleaner setup
			fixture := NewTestFixture(t)
			tt.setupFunc(fixture.Transport)
			
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			
			err := fixture.Transport.Connect(ctx)
			
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

// TestSendFailuresSimplified uses test infrastructure for cleaner send failure tests
func TestSendFailuresSimplified(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(*TestFixture)
		event         TransportEvent
		expectedError error
	}{
		{
			name: "not_connected",
			setupFunc: func(f *TestFixture) {
				// Transport not connected by default
			},
			event:         NewTestEvent("test-1", "demo"),
			expectedError: ErrNotConnected,
		},
		{
			name: "nil_event",
			setupFunc: func(f *TestFixture) {
				f.ConnectTransport(t)
			},
			event:         nil,
			expectedError: nil, // MockTransport doesn't check for nil
		},
		{
			name: "send_timeout",
			setupFunc: func(f *TestFixture) {
				f.ConnectTransport(t)
				f.Transport.SetSendBehavior(func(ctx context.Context, event TransportEvent) error {
					select {
					case <-time.After(2 * time.Second):
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
			},
			event:         NewTestEvent("test-1", "demo"),
			expectedError: context.DeadlineExceeded,
		},
		{
			name: "custom_send_error",
			setupFunc: func(f *TestFixture) {
				f.ConnectTransport(t)
				f.Transport.SetSendBehavior(func(ctx context.Context, event TransportEvent) error {
					return errors.New("network error")
				})
			},
			event:         NewTestEvent("test-1", "demo"),
			expectedError: errors.New("network error"),
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := NewTestFixture(t)
			tt.setupFunc(fixture)
			
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			
			err := fixture.Transport.Send(ctx, tt.event)
			
			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("Expected error %v, got nil", tt.expectedError)
				} else if tt.expectedError.Error() != err.Error() && !errors.Is(err, tt.expectedError) {
					t.Errorf("Expected error %v, got %v", tt.expectedError, err)
				}
			} else if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

// TestReceiveFailuresSimplified uses assertion helpers for cleaner receive tests
func TestReceiveFailuresSimplified(t *testing.T) {
	t.Run("error_channel_behavior", func(t *testing.T) {
		fixture := NewTestFixture(t)
		fixture.ConnectTransport(t)
		
		// Simulate various errors
		testErrors := []error{
			ErrConnectionClosed,
			ErrTimeout,
			errors.New("network error"),
			NewTransportError("test", "receive", errors.New("io error")),
		}
		
		// Send errors
		for _, err := range testErrors {
			if simErr := fixture.Transport.SimulateError(err); simErr != nil {
				t.Fatalf("Failed to simulate error: %v", simErr)
			}
		}
		
		// Use assertion helpers to verify errors
		for i, expectedErr := range testErrors {
			err := AssertErrorReceived(t, fixture.Transport.Errors(), 100*time.Millisecond)
			if err.Error() != expectedErr.Error() {
				t.Errorf("Error %d: expected %v, got %v", i, expectedErr, err)
			}
		}
		
		// Verify no more errors
		AssertNoError(t, fixture.Transport.Errors(), 50*time.Millisecond)
	})
}

// TestContextCancellationSimplified uses WithTimeout helper
func TestContextCancellationSimplified(t *testing.T) {
	t.Run("connect_with_cancelled_context", func(t *testing.T) {
		transport := NewAdvancedMockTransport()
		transport.SetNetworkConditions(100*time.Millisecond, 0, 0, 0)
		
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		err := transport.Connect(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	})
	
	t.Run("operation_with_timeout", func(t *testing.T) {
		transport := NewAdvancedMockTransport()
		transport.SetNetworkConditions(200*time.Millisecond, 0, 0, 0)
		
		// Use WithTimeoutExpected helper since we expect a timeout
		var connectErr error
		WithTimeoutExpected(t, 100*time.Millisecond, func(ctx context.Context) {
			connectErr = transport.Connect(ctx)
		})
		
		// Verify that the operation timed out as expected
		if !errors.Is(connectErr, context.DeadlineExceeded) {
			t.Errorf("Expected deadline exceeded, got %v", connectErr)
		}
	})
}

// TestConcurrentOperationsSimplified uses ConcurrentTest helper
func TestConcurrentOperationsSimplified(t *testing.T) {
	t.Run("concurrent_sends", func(t *testing.T) {
		fixture := NewTestFixture(t)
		fixture.ConnectTransport(t)
		
		ct := NewConcurrentTest()
		
		// Run concurrent sends
		ct.Run(100, func(id int) error {
			event := NewTestEvent(fmt.Sprintf("concurrent-%d", id), "test")
			return fixture.Transport.Send(fixture.Ctx, event)
		})
		
		// Wait and check for errors
		errors := ct.Wait()
		if len(errors) > 0 {
			t.Errorf("Got %d errors during concurrent sends", len(errors))
		}
		
		// Verify all events were sent
		sentEvents := fixture.Transport.GetSentEvents()
		if len(sentEvents) != 100 {
			t.Errorf("Expected 100 sent events, got %d", len(sentEvents))
		}
	})
}

// TestNetworkConditionsSimplified demonstrates network simulation
func TestNetworkConditionsSimplified(t *testing.T) {
	scenarios := []struct {
		name      string
		transport *ScenarioTransport
	}{
		{
			name:      "flaky_network",
			transport: NewScenarioTransport("flaky-network"),
		},
		{
			name:      "slow_connection",
			transport: NewScenarioTransport("slow-connection"),
		},
		{
			name:      "unreliable",
			transport: NewScenarioTransport("unreliable"),
		},
	}
	
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Add timeout to prevent hanging on unreliable network conditions
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			// Test connection with network conditions
			start := time.Now()
			err := scenario.transport.Connect(ctx)
			connectTime := time.Since(start)
			
			if err != nil {
				t.Logf("Connection failed in %s scenario: %v (took %v)", scenario.name, err, connectTime)
				return
			}
			
			t.Logf("Connected in %s scenario after %v", scenario.name, connectTime)
			
			// Try sending some events
			successCount := 0
			for i := 0; i < 10; i++ {
				event := NewTestEvent(fmt.Sprintf("%s-%d", scenario.name, i), "test")
				if err := scenario.transport.Send(ctx, event); err == nil {
					successCount++
				}
			}
			
			t.Logf("%s scenario: %d/10 sends succeeded", scenario.name, successCount)
		})
	}
}

// TestErrorSimulatorUsage demonstrates the error simulator
func TestErrorSimulatorUsage(t *testing.T) {
	fixture := NewTestFixture(t)
	simulator := NewErrorSimulator()
	
	// Configure error patterns
	simulator.SetError("connect", ErrConnectionFailed)
	simulator.SetError("send", ErrMessageTooLarge)
	simulator.SetErrorFrequency("send", 3) // Every 3rd send fails
	
	// Use simulator in transport behavior
	fixture.Transport.SetConnectBehavior(func(ctx context.Context) error {
		if err, shouldError := simulator.ShouldError("connect"); shouldError {
			return err
		}
		return nil
	})
	
	fixture.Transport.SetSendBehavior(func(ctx context.Context, event TransportEvent) error {
		if err, shouldError := simulator.ShouldError("send"); shouldError {
			return err
		}
		return nil
	})
	
	// Test connection (should fail)
	err := fixture.Transport.Connect(context.Background())
	if !errors.Is(err, ErrConnectionFailed) {
		t.Errorf("Expected ErrConnectionFailed, got %v", err)
	}
	
	// Manually mark as connected for send tests
	fixture.Transport.connected.Store(true)
	
	// Test sends (every 3rd should fail)
	sendResults := make([]bool, 10)
	for i := 0; i < 10; i++ {
		event := NewTestEvent(fmt.Sprintf("sim-%d", i), "test")
		err := fixture.Transport.Send(context.Background(), event)
		sendResults[i] = err == nil
	}
	
	// Check pattern: fail on 3, 6, 9
	expectedPattern := []bool{true, true, false, true, true, false, true, true, false, true}
	for i, expected := range expectedPattern {
		if sendResults[i] != expected {
			t.Errorf("Send %d: expected success=%v, got success=%v", i+1, expected, sendResults[i])
		}
	}
}

// TestRecordingUsage demonstrates operation recording
func TestRecordingUsage(t *testing.T) {
	base := NewMockTransport()
	recorder := NewRecordingTransport(base)
	
	ctx := context.Background()
	
	// Perform various operations
	recorder.Connect(ctx)
	
	// Send events with different sizes
	events := []TransportEvent{
		NewTestEvent("small", "test"),
		NewTestEventWithData("medium", "test", map[string]interface{}{
			"data": "some medium sized data here",
		}),
		NewTestEventWithData("large", "test", map[string]interface{}{
			"data": string(make([]byte, 1000)),
		}),
	}
	
	for _, event := range events {
		recorder.Send(ctx, event)
	}
	
	recorder.Close(ctx)
	
	// Analyze operations
	ops := recorder.GetOperations()
	
	t.Logf("Recorded %d operations:", len(ops))
	for i, op := range ops {
		t.Logf("  %d. %s - Duration: %v, Error: %v", i+1, op.Type, op.Duration, op.Error)
	}
	
	// Find slowest operation
	var slowest Operation
	for _, op := range ops {
		if op.Duration > slowest.Duration {
			slowest = op
		}
	}
	
	t.Logf("Slowest operation: %s took %v", slowest.Type, slowest.Duration)
}

// TestMetricsCollection demonstrates advanced metrics tracking
func TestMetricsCollection(t *testing.T) {
	transport := NewAdvancedMockTransport()
	
	// Configure some network conditions
	transport.SetNetworkConditions(
		10*time.Millisecond,  // latency
		5*time.Millisecond,   // jitter
		0.1,                  // 10% packet loss
		0,                    // unlimited bandwidth
	)
	
	ctx := context.Background()
	
	// Connect and disconnect multiple times
	for i := 0; i < 3; i++ {
		if err := transport.Connect(ctx); err != nil {
			t.Logf("Connect attempt %d failed: %v", i+1, err)
			continue
		}
		
		// Send some events
		for j := 0; j < 10; j++ {
			event := NewTestEvent(fmt.Sprintf("metric-%d-%d", i, j), "test")
			transport.Send(ctx, event)
		}
		
		transport.Close(ctx)
	}
	
	// Get metrics summary
	summary := transport.metrics.GetSummary()
	
	t.Logf("Metrics Summary:")
	for key, value := range summary {
		t.Logf("  %s: %v", key, value)
	}
	
	// Verify some metrics
	if dropped, ok := summary["events_dropped"].(int64); ok && dropped == 0 {
		t.Error("Expected some events to be dropped with 10% packet loss")
	}
}

// BenchmarkSimplifiedTransport uses the benchmark helpers
func BenchmarkSimplifiedTransport(b *testing.B) {
	b.Run("mock_transport", func(b *testing.B) {
		transport := NewMockTransport()
		BenchmarkTransport(b, transport)
	})
	
	b.Run("scenario_perfect", func(b *testing.B) {
		transport := NewScenarioTransport("perfect")
		BenchmarkTransport(b, transport)
	})
	
	b.Run("scenario_flaky", func(b *testing.B) {
		transport := NewScenarioTransport("flaky-network")
		BenchmarkTransport(b, transport)
	})
}

// BenchmarkConcurrentSimplified uses concurrent benchmark helpers
func BenchmarkConcurrentSimplified(b *testing.B) {
	concurrencyLevels := []int{1, 10, 100}
	
	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("concurrency_%d", concurrency), func(b *testing.B) {
			transport := NewMockTransport()
			BenchmarkConcurrentSend(b, transport, concurrency)
		})
	}
}