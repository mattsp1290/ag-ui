package events

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Helper function for string pointers
func stringPtr(s string) *string {
	return &s
}

// Helper function to create a test event
func createTestRunStartedEvent() *RunStartedEvent {
	return &RunStartedEvent{
		BaseEvent: &BaseEvent{
			EventType:   EventTypeRunStarted,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		RunID:    "run-123",
		ThreadID: "thread-456",
	}
}

// Helper function to generate a test sequence
func generateTestSequence(size int) []Event {
	events := make([]Event, 0, size)

	// Start with RUN_STARTED
	events = append(events, &RunStartedEvent{
		BaseEvent: &BaseEvent{
			EventType:   EventTypeRunStarted,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		RunID:    "run-bench",
		ThreadID: "thread-bench",
	})

	// Add message sequences
	msgCount := (size - 2) / 4 // Reserve 2 for run start/finish, 4 events per message
	for i := 0; i < msgCount; i++ {
		msgID := fmt.Sprintf("msg-%d", i)
		events = append(events,
			&TextMessageStartEvent{
				BaseEvent: &BaseEvent{
					EventType:   EventTypeTextMessageStart,
					TimestampMs: timePtr(time.Now().UnixMilli()),
				},
				MessageID: msgID,
				Role:      stringPtr("user"),
			},
			&TextMessageContentEvent{
				BaseEvent: &BaseEvent{
					EventType:   EventTypeTextMessageContent,
					TimestampMs: timePtr(time.Now().UnixMilli()),
				},
				MessageID: msgID,
				Delta:     fmt.Sprintf("Message content %d", i),
			},
			&TextMessageContentEvent{
				BaseEvent: &BaseEvent{
					EventType:   EventTypeTextMessageContent,
					TimestampMs: timePtr(time.Now().UnixMilli()),
				},
				MessageID: msgID,
				Delta:     " additional content",
			},
			&TextMessageEndEvent{
				BaseEvent: &BaseEvent{
					EventType:   EventTypeTextMessageEnd,
					TimestampMs: timePtr(time.Now().UnixMilli()),
				},
				MessageID: msgID,
			},
		)
	}

	// End with RUN_FINISHED
	events = append(events, &RunFinishedEvent{
		BaseEvent: &BaseEvent{
			EventType:   EventTypeRunFinished,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		RunID: "run-bench",
	})

	return events
}

// BenchmarkEventValidator_ValidateEvent tests single event validation performance
func BenchmarkEventValidator_ValidateEvent(b *testing.B) {
	validator := NewEventValidator(DefaultValidationConfig())
	event := createTestRunStartedEvent()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := validator.ValidateEvent(ctx, event)
		if result == nil {
			b.Fatal("Validation result should not be nil")
		}
	}
}

// BenchmarkEventValidator_ValidateEvent_Permissive tests permissive mode performance
func BenchmarkEventValidator_ValidateEvent_Permissive(b *testing.B) {
	validator := NewEventValidator(PermissiveValidationConfig())
	event := createTestRunStartedEvent()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := validator.ValidateEvent(ctx, event)
		if result == nil {
			b.Fatal("Validation result should not be nil")
		}
	}
}

// BenchmarkEventValidator_ValidateSequence_Small tests small sequence validation
func BenchmarkEventValidator_ValidateSequence_Small(b *testing.B) {
	validator := NewEventValidator(DefaultValidationConfig())
	events := generateTestSequence(10) // 10 events
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := validator.ValidateSequence(ctx, events)
		if result == nil {
			b.Fatal("Validation result should not be nil")
		}
	}
}

// BenchmarkEventValidator_ValidateSequence_Medium tests medium sequence validation
func BenchmarkEventValidator_ValidateSequence_Medium(b *testing.B) {
	validator := NewEventValidator(DefaultValidationConfig())
	events := generateTestSequence(100) // 100 events
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := validator.ValidateSequence(ctx, events)
		if result == nil {
			b.Fatal("Validation result should not be nil")
		}
	}
}

// BenchmarkEventValidator_ValidateSequence_Large tests large sequence validation
func BenchmarkEventValidator_ValidateSequence_Large(b *testing.B) {
	validator := NewEventValidator(DefaultValidationConfig())
	events := generateTestSequence(1000) // 1000 events
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := validator.ValidateSequence(ctx, events)
		if result == nil {
			b.Fatal("Validation result should not be nil")
		}
	}
}

// BenchmarkIDTracker_ValidateConsistency tests ID validation performance
func BenchmarkIDTracker_ValidateConsistency(b *testing.B) {
	tracker := NewIDTracker()

	// Populate with test data
	for i := 0; i < 100; i++ {
		msgID := fmt.Sprintf("msg-%d", i)
		tracker.TrackEvent(&TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart},
			MessageID: msgID,
		})
		tracker.TrackEvent(&TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent},
			MessageID: msgID,
			Delta:     "content",
		})
		tracker.TrackEvent(&TextMessageEndEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd},
			MessageID: msgID,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tracker.ValidateIDConsistency()
	}
}

// BenchmarkEventSequenceTracker_TrackEvent tests event tracking performance
func BenchmarkEventSequenceTracker_TrackEvent(b *testing.B) {
	config := &SequenceTrackerConfig{
		MaxHistorySize: 10000,
		TrackMetrics:   true,
		ValidateOnAdd:  false,
	}
	tracker := NewEventSequenceTracker(config)
	event := createTestRunStartedEvent()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := tracker.TrackEvent(event)
		if err != nil {
			b.Fatalf("TrackEvent failed: %v", err)
		}
	}
}

// BenchmarkEventSequenceTracker_GetEventsByType tests event filtering performance
func BenchmarkEventSequenceTracker_GetEventsByType(b *testing.B) {
	config := DefaultSequenceTrackerConfig()
	tracker := NewEventSequenceTracker(config)

	// Populate with mixed events
	events := generateTestSequence(1000)
	for _, event := range events {
		tracker.TrackEvent(event)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := tracker.GetEventsByType(EventTypeTextMessageContent)
		if results == nil {
			b.Fatal("GetEventsByType should return a slice")
		}
	}
}

// BenchmarkValidationRule_MessageLifecycle tests message lifecycle rule performance
func BenchmarkValidationRule_MessageLifecycle(b *testing.B) {
	rule := NewMessageLifecycleRule()
	event := &TextMessageContentEvent{
		BaseEvent: &BaseEvent{
			EventType:   EventTypeTextMessageContent,
			TimestampMs: timePtr(time.Now().UnixMilli()),
		},
		MessageID: "msg-bench",
		Delta:     "benchmark content",
	}

	context := &ValidationContext{
		State:    NewValidationState(),
		Config:   DefaultValidationConfig(),
		Metadata: make(map[string]interface{}),
	}

	// Set up state
	context.State.ActiveMessages["msg-bench"] = &MessageState{
		MessageID: "msg-bench",
		StartTime: time.Now(),
		IsActive:  true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := rule.Validate(event, context)
		if result == nil {
			b.Fatal("Validate should return a result")
		}
	}
}

// BenchmarkValidationMetrics_RecordEvent tests metrics recording performance
func BenchmarkValidationMetrics_RecordEvent(b *testing.B) {
	metrics := NewValidationMetrics()
	duration := 100 * time.Microsecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RecordEvent(duration)
	}
}

// BenchmarkConcurrentValidation tests concurrent validation performance
func BenchmarkConcurrentValidation(b *testing.B) {
	validator := NewEventValidator(DefaultValidationConfig())
	event := createTestRunStartedEvent()
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result := validator.ValidateEvent(ctx, event)
			if result == nil {
				b.Fatal("Validation result should not be nil")
			}
		}
	})
}

// BenchmarkMemoryAllocation_ValidateEvent measures memory allocations
func BenchmarkMemoryAllocation_ValidateEvent(b *testing.B) {
	validator := NewEventValidator(DefaultValidationConfig())
	event := createTestRunStartedEvent()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = validator.ValidateEvent(ctx, event)
	}
}

// BenchmarkMemoryAllocation_ValidateSequence measures sequence validation allocations
func BenchmarkMemoryAllocation_ValidateSequence(b *testing.B) {
	validator := NewEventValidator(DefaultValidationConfig())
	events := generateTestSequence(100)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = validator.ValidateSequence(ctx, events)
	}
}
