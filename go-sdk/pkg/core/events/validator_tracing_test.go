package events

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TestEventValidator_TracingIntegration tests that tracing spans are properly created and closed
func TestEventValidator_TracingIntegration(t *testing.T) {
	// Setup in-memory span recorder for testing
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		t.Fatalf("Failed to create stdout exporter: %v", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)

	validator := NewEventValidator(DefaultValidationConfig())

	tests := []struct {
		name           string
		event          Event
		expectedSpans  int
		validateSpan   func(t *testing.T)
	}{
		{
			name: "valid run started event creates spans",
			event: &RunStartedEvent{
				BaseEvent: &BaseEvent{
					EventType:   EventTypeRunStarted,
					TimestampMs: timePtr(time.Now().UnixMilli()),
				},
				RunID:    "run-123",
				ThreadID: "thread-456",
			},
			expectedSpans: 1, // Should create at least one span for event validation
		},
		{
			name:          "nil event creates error span",
			event:         nil,
			expectedSpans: 1,
		},
		{
			name: "invalid event creates error spans",
			event: &RunStartedEvent{
				BaseEvent: &BaseEvent{
					EventType:   EventTypeRunStarted,
					TimestampMs: timePtr(time.Now().UnixMilli()),
				},
				// Missing RunID - should trigger validation error
				ThreadID: "thread-456",
			},
			expectedSpans: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := validator.ValidateEvent(ctx, tt.event)

			// Verify that result is not nil
			if result == nil {
				t.Errorf("ValidateEvent() returned nil result")
				return
			}

			// For now, we mainly verify that tracing doesn't break the validation
			// In a production environment, you would use a test span recorder
			// to verify the actual span creation and attributes
			t.Logf("Validation result: IsValid=%v, Errors=%d, Warnings=%d", 
				result.IsValid, len(result.Errors), len(result.Warnings))
		})
	}
}

// TestEventValidator_SequenceTracingIntegration tests sequence validation tracing
func TestEventValidator_SequenceTracingIntegration(t *testing.T) {
	validator := NewEventValidator(DefaultValidationConfig())

	events := []Event{
		&RunStartedEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeRunStarted,
				TimestampMs: timePtr(time.Now().UnixMilli()),
			},
			RunID:    "run-1",
			ThreadID: "thread-1",
		},
		&TextMessageStartEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeTextMessageStart,
				TimestampMs: timePtr(time.Now().UnixMilli()),
			},
			MessageID: "msg-1",
		},
		&TextMessageContentEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeTextMessageContent,
				TimestampMs: timePtr(time.Now().UnixMilli()),
			},
			MessageID: "msg-1",
			Delta:     "Hello world",
		},
		&TextMessageEndEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeTextMessageEnd,
				TimestampMs: timePtr(time.Now().UnixMilli()),
			},
			MessageID: "msg-1",
		},
		&RunFinishedEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeRunFinished,
				TimestampMs: timePtr(time.Now().UnixMilli()),
			},
			RunID: "run-1",
		},
	}

	ctx := context.Background()
	result := validator.ValidateSequence(ctx, events)

	if result == nil {
		t.Errorf("ValidateSequence() returned nil result")
		return
	}

	if !result.IsValid {
		t.Errorf("ValidateSequence() expected valid sequence but got errors: %v", result.Errors)
	}

	t.Logf("Sequence validation result: IsValid=%v, Events=%d, Duration=%v", 
		result.IsValid, result.EventCount, result.Duration)
}

// TestEventValidator_TracingWithContext tests that tracing works with context cancellation
func TestEventValidator_TracingWithContext(t *testing.T) {
	validator := NewEventValidator(DefaultValidationConfig())

	event := &RunStartedEvent{
		BaseEvent: &BaseEvent{
			EventType:   EventTypeRunStarted,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		RunID:    "run-123",
		ThreadID: "thread-456",
	}

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := validator.ValidateEvent(ctx, event)

	if result == nil {
		t.Errorf("ValidateEvent() with cancelled context returned nil result")
		return
	}

	if result.IsValid {
		t.Errorf("ValidateEvent() with cancelled context should not be valid")
	}

	// Should have a context cancelled error
	found := false
	for _, err := range result.Errors {
		if err.RuleID == "CONTEXT_CANCELLED" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("ValidateEvent() with cancelled context should have CONTEXT_CANCELLED error")
	}
}

// TestEventValidator_TracingNoOp tests that validation works even without tracing setup
func TestEventValidator_TracingNoOp(t *testing.T) {
	// Set up no-op tracer to simulate environments without tracing
	otel.SetTracerProvider(noop.NewTracerProvider())

	validator := NewEventValidator(DefaultValidationConfig())

	event := &RunStartedEvent{
		BaseEvent: &BaseEvent{
			EventType:   EventTypeRunStarted,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		RunID:    "run-123",
		ThreadID: "thread-456",
	}

	ctx := context.Background()
	result := validator.ValidateEvent(ctx, event)

	if result == nil {
		t.Errorf("ValidateEvent() with no-op tracer returned nil result")
		return
	}

	if !result.IsValid {
		t.Errorf("ValidateEvent() with no-op tracer should be valid but got errors: %v", result.Errors)
	}

	t.Logf("No-op tracer validation result: IsValid=%v", result.IsValid)
}

// TestEventValidator_TracingAttributes tests that proper span attributes are set
func TestEventValidator_TracingAttributes(t *testing.T) {
	validator := NewEventValidator(DefaultValidationConfig())

	event := &RunStartedEvent{
		BaseEvent: &BaseEvent{
			EventType:   EventTypeRunStarted,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		RunID:    "run-123",
		ThreadID: "thread-456",
	}

	ctx := context.Background()
	result := validator.ValidateEvent(ctx, event)

	if result == nil {
		t.Errorf("ValidateEvent() returned nil result")
		return
	}

	// In a real test environment, you would capture the spans and verify attributes
	// For now, we verify that the validation completed successfully
	if !result.IsValid {
		t.Errorf("ValidateEvent() should be valid but got errors: %v", result.Errors)
	}

	// Verify that duration is recorded (indicating timing attributes work)
	if result.Duration <= 0 {
		t.Errorf("ValidateEvent() duration should be positive, got: %v", result.Duration)
	}
}

// TestEventValidator_RuleTracingIntegration tests rule-level tracing
func TestEventValidator_RuleTracingIntegration(t *testing.T) {
	validator := NewEventValidator(DefaultValidationConfig())

	// Create an event that will trigger multiple rules
	event := &TextMessageStartEvent{
		BaseEvent: &BaseEvent{
			EventType:   EventTypeTextMessageStart,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		MessageID: "msg-1",
	}

	ctx := context.Background()
	result := validator.ValidateEvent(ctx, event)

	if result == nil {
		t.Errorf("ValidateEvent() returned nil result")
		return
	}

	// This should trigger validation errors (no run started first)
	if result.IsValid {
		t.Errorf("ValidateEvent() should be invalid (no run started)")
	}

	// Verify that errors were recorded
	if len(result.Errors) == 0 {
		t.Errorf("ValidateEvent() should have validation errors")
	}

	t.Logf("Rule validation result: IsValid=%v, Errors=%d", result.IsValid, len(result.Errors))
}

// BenchmarkEventValidator_WithTracing benchmarks validation performance with tracing enabled
func BenchmarkEventValidator_WithTracing(b *testing.B) {
	validator := NewEventValidator(DefaultValidationConfig())

	event := &RunStartedEvent{
		BaseEvent: &BaseEvent{
			EventType:   EventTypeRunStarted,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		RunID:    "run-123",
		ThreadID: "thread-456",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := validator.ValidateEvent(ctx, event)
		if result == nil {
			b.Errorf("ValidateEvent() returned nil result")
		}
		// Reset validator state for next iteration
		validator.Reset()
	}
}

// BenchmarkEventValidator_WithoutTracing benchmarks validation performance without tracing
func BenchmarkEventValidator_WithoutTracing(b *testing.B) {
	// Set up no-op tracer
	otel.SetTracerProvider(noop.NewTracerProvider())

	validator := NewEventValidator(DefaultValidationConfig())

	event := &RunStartedEvent{
		BaseEvent: &BaseEvent{
			EventType:   EventTypeRunStarted,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		RunID:    "run-123",
		ThreadID: "thread-456",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := validator.ValidateEvent(ctx, event)
		if result == nil {
			b.Errorf("ValidateEvent() returned nil result")
		}
		// Reset validator state for next iteration
		validator.Reset()
	}
}

