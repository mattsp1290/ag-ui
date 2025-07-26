package events

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestEventValidator_ConcurrentAccess tests thread safety of the validator
func TestEventValidator_ConcurrentAccess(t *testing.T) {
	validator := NewEventValidator(DefaultValidationConfig())

	// Test concurrent rule addition/removal
	t.Run("concurrent rule management", func(t *testing.T) {
		var wg sync.WaitGroup

		// Add rules concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				rule := &RunLifecycleRule{
					BaseValidationRule: NewBaseValidationRule(
						fmt.Sprintf("RULE_%d", id),
						fmt.Sprintf("Test rule %d", id),
						ValidationSeverityError,
					),
				}
				rule.SetEnabled(id%2 == 0)
				validator.AddRule(rule)
			}(i)
		}

		// Remove rules concurrently
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				validator.RemoveRule(fmt.Sprintf("RULE_%d", id))
			}(i)
		}

		// Get rules concurrently
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rules := validator.GetRules()
				if rules == nil {
					t.Error("GetRules should not return nil")
				}
			}()
		}

		wg.Wait()

		// Verify final state
		rules := validator.GetRules()
		if len(rules) == 0 {
			t.Error("Should have some rules remaining")
		}
	})

	// Test concurrent validation operations
	t.Run("concurrent validation", func(t *testing.T) {
		var wg sync.WaitGroup
		events := []Event{
			&RunStartedEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
				RunIDValue:     "run-1",
				ThreadIDValue:  "thread-1",
			},
			&TextMessageStartEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtr(time.Now().UnixMilli())},
				MessageID: "msg-1",
				Role:      stringPtr("user"),
			},
			&TextMessageContentEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtr(time.Now().UnixMilli())},
				MessageID: "msg-1",
				Delta:     "test content",
			},
			&TextMessageEndEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd, TimestampMs: timePtr(time.Now().UnixMilli())},
				MessageID: "msg-1",
			},
			&RunFinishedEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtr(time.Now().UnixMilli())},
				RunIDValue:     "run-1",
			},
		}

		// Validate events concurrently
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(eventIndex int) {
				defer wg.Done()
				event := events[eventIndex%len(events)]
				result := validator.ValidateEvent(context.Background(), event)
				if result == nil {
					t.Errorf("Validation result should not be nil")
				}
			}(i)
		}

		wg.Wait()
	})

	// Test concurrent state access
	t.Run("concurrent state access", func(t *testing.T) {
		var wg sync.WaitGroup

		// Get state concurrently
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				state := validator.GetState()
				if state == nil {
					t.Error("GetState should not return nil")
				}
			}()
		}

		// Get metrics concurrently
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				metrics := validator.GetMetrics()
				if metrics == nil {
					t.Error("GetMetrics should not return nil")
				}
			}()
		}

		wg.Wait()
	})
}

// TestEventValidator_ConcurrentSequenceValidation tests concurrent sequence validation
func TestEventValidator_ConcurrentSequenceValidation(t *testing.T) {
	// Fixed: ValidateSequence now uses isolated validator instances for thread safety

	validator := NewEventValidator(DefaultValidationConfig())

	// Create test sequences
	sequences := make([][]Event, 5)
	for i := 0; i < 5; i++ {
		sequences[i] = []Event{
			&RunStartedEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
				RunIDValue:     fmt.Sprintf("run-%d", i),
				ThreadIDValue:  fmt.Sprintf("thread-%d", i),
			},
			&TextMessageStartEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtr(time.Now().UnixMilli())},
				MessageID: fmt.Sprintf("msg-%d", i),
				Role:      stringPtr("user"),
			},
			&TextMessageEndEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd, TimestampMs: timePtr(time.Now().UnixMilli())},
				MessageID: fmt.Sprintf("msg-%d", i),
			},
			&RunFinishedEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtr(time.Now().UnixMilli())},
				RunIDValue:     fmt.Sprintf("run-%d", i),
			},
		}
	}

	var wg sync.WaitGroup

	// Validate sequences concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(seqIndex int) {
			defer wg.Done()
			result := validator.ValidateSequence(context.Background(), sequences[seqIndex])
			if !result.IsValid {
				t.Errorf("Sequence %d should be valid", seqIndex)
			}
		}(i)
	}

	wg.Wait()
}

// TestIDTracker_ConcurrentAccess tests thread safety of ID tracker
func TestIDTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewIDTracker()
	var wg sync.WaitGroup

	// Track events concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			msgID := fmt.Sprintf("msg-%d", id)

			// Track message start
			tracker.TrackEvent(&TextMessageStartEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart},
				MessageID: msgID,
			})

			// Track message content
			tracker.TrackEvent(&TextMessageContentEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent},
				MessageID: msgID,
				Delta:     fmt.Sprintf("content %d", id),
			})

			// Track message end
			tracker.TrackEvent(&TextMessageEndEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd},
				MessageID: msgID,
			})
		}(i)
	}

	// Validate consistency concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errors := tracker.ValidateIDConsistency()
			if errors == nil {
				t.Error("ValidateIDConsistency should not return nil")
			}
		}()
	}

	// Get statistics concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stats := tracker.GetStatistics()
			if stats == nil {
				t.Error("GetStatistics should not return nil")
			}
		}()
	}

	wg.Wait()

	// Final validation
	errors := tracker.ValidateIDConsistency()
	if len(errors) > 0 {
		t.Errorf("Should have no validation errors, got %d", len(errors))
	}
}

// TestEventSequenceTracker_ConcurrentAccess tests thread safety of sequence tracker
func TestEventSequenceTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewEventSequenceTracker(DefaultSequenceTrackerConfig())
	var wg sync.WaitGroup

	// Track events concurrently
	events := []Event{
		&RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
			RunIDValue:     "run-concurrent",
			ThreadIDValue:  "thread-concurrent",
		},
		&TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart},
			MessageID: "msg-concurrent",
		},
		&TextMessageEndEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd},
			MessageID: "msg-concurrent",
		},
	}

	// Track events from multiple goroutines
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			event := events[index%len(events)]
			err := tracker.TrackEvent(event)
			if err != nil {
				t.Errorf("TrackEvent failed: %v", err)
			}
		}(i)
	}

	// Query events concurrently
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Get events by type
			_ = tracker.GetEventsByType(EventTypeTextMessageStart)

			// Get event history
			_ = tracker.GetEventHistory()

			// Get events by run ID
			_ = tracker.GetEventsByRunID("run-concurrent")

			// Check active states
			_ = tracker.IsMessageActive("msg-concurrent")
			_ = tracker.IsRunActive("run-concurrent")
		}()
	}

	// Generate compliance reports concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			report := tracker.CheckSequenceCompliance()
			if report == nil {
				t.Error("CheckSequenceCompliance should not return nil")
			}
		}()
	}

	wg.Wait()
}

// TestValidationMetrics_ConcurrentAccess tests thread safety of metrics
func TestValidationMetrics_ConcurrentAccess(t *testing.T) {
	metrics := NewValidationMetrics()
	var wg sync.WaitGroup

	// Record events concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			duration := time.Duration(id) * time.Microsecond
			metrics.RecordEvent(duration)
		}(i)
	}

	// Record errors and warnings concurrently
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics.RecordError()
			metrics.RecordWarning()
		}()
	}

	// Record rule executions concurrently
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ruleID := fmt.Sprintf("RULE_%d", id%5)
			duration := time.Duration(id*10) * time.Microsecond
			metrics.RecordRuleExecution(ruleID, duration)
		}(i)
	}

	wg.Wait()

	// Verify metrics
	if metrics.EventsProcessed == 0 {
		t.Error("Should have recorded events")
	}
	if metrics.ErrorCount == 0 {
		t.Error("Should have recorded errors")
	}
	if metrics.WarningCount == 0 {
		t.Error("Should have recorded warnings")
	}
}

// TestValidator_RaceConditions uses Go's race detector
func TestValidator_RaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	validator := NewEventValidator(DefaultValidationConfig())
	ctx := context.Background()

	// Create shared events
	event1 := &RunStartedEvent{
		BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
		RunIDValue:     "run-race",
		ThreadIDValue:  "thread-race",
	}

	event2 := &TextMessageStartEvent{
		BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart},
		MessageID: "msg-race",
		Role:      stringPtr("user"),
	}

	var wg sync.WaitGroup

	// Start multiple goroutines doing various operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = validator.ValidateEvent(ctx, event1)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = validator.ValidateEvent(ctx, event2)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = validator.GetState()
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = validator.GetMetrics()
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			validator.AddRule(&RunLifecycleRule{
				BaseValidationRule: NewBaseValidationRule(
					fmt.Sprintf("RULE_%d", i),
					fmt.Sprintf("Test rule %d", i),
					ValidationSeverityError,
				),
			})
		}()
	}

	wg.Wait()
}

// TestValidator_ContextCancellation tests context cancellation handling
func TestValidator_ContextCancellation(t *testing.T) {
	validator := NewEventValidator(DefaultValidationConfig())

	t.Run("immediate cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		event := &RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
			RunIDValue:     "run-1",
			ThreadIDValue:  "thread-1",
		}

		result := validator.ValidateEvent(ctx, event)
		if result.IsValid {
			t.Error("Validation should fail with cancelled context")
		}

		found := false
		for _, err := range result.Errors {
			if err.RuleID == RuleIDContextCancelled {
				found = true
				break
			}
		}
		if !found {
			t.Error("Should have context cancellation error")
		}
	})

	t.Run("cancellation during sequence", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
		defer cancel()

		// Create a large sequence
		events := make([]Event, 200)
		for i := 0; i < 200; i++ {
			events[i] = &TextMessageContentEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent},
				MessageID: fmt.Sprintf("msg-%d", i),
				Delta:     fmt.Sprintf("content %d", i),
			}
		}

		// Add small delay to ensure context times out
		time.Sleep(2 * time.Microsecond)

		result := validator.ValidateSequence(ctx, events)
		if result.IsValid {
			t.Error("Validation should fail with cancelled context")
		}
	})
}
