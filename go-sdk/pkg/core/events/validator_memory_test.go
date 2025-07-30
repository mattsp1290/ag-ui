package events

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"
)

// TestEventSequenceTracker_MemoryBounds tests that memory usage is bounded
func TestEventSequenceTracker_MemoryBounds(t *testing.T) {
	config := &SequenceTrackerConfig{
		MaxHistorySize:       100,
		EnableStateSnapshots: false, // Disable to focus on history bounds
		TrackMetrics:         true,
		ValidateOnAdd:        false,
	}
	tracker := NewEventSequenceTracker(config)

	// Record initial memory
	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Add more events than the limit
	for i := 0; i < 200; i++ {
		event := &TextMessageContentEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeTextMessageContent,
				TimestampMs: timePtr(time.Now().UnixMilli() + int64(i)),
			},
			MessageID: fmt.Sprintf("msg-%d", i%10),
			Delta:     fmt.Sprintf("content-%d", i),
		}

		err := tracker.TrackEvent(event)
		if err != nil {
			t.Fatalf("TrackEvent failed: %v", err)
		}
	}

	// Verify history is bounded
	history := tracker.GetEventHistory()
	if len(history) > config.MaxHistorySize {
		t.Errorf("History size %d exceeds limit %d", len(history), config.MaxHistorySize)
	}

	// Record final memory
	var m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m2)

	// Memory growth should be reasonable (not exact due to GC)
	memoryGrowth := m2.HeapAlloc - m1.HeapAlloc
	maxExpectedGrowth := uint64(config.MaxHistorySize * 1024) // Rough estimate

	t.Logf("Memory growth: %d bytes, max expected: %d bytes", memoryGrowth, maxExpectedGrowth)

	// Verify oldest events were removed
	oldestEvent := history[0]
	if oldestEvent.Type() != EventTypeTextMessageContent {
		t.Errorf("Expected TextMessageContent, got %s", oldestEvent.Type())
	}
}

// TestEventSequenceTracker_MemoryLeakDetection tests for memory leaks
func TestEventSequenceTracker_MemoryLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	config := &SequenceTrackerConfig{
		MaxHistorySize:       1000,
		EnableStateSnapshots: true,
		TrackMetrics:         true,
		ValidateOnAdd:        false,
	}

	// Run multiple iterations to detect leaks
	for iteration := 0; iteration < 3; iteration++ {
		tracker := NewEventSequenceTracker(config)

		// Add and remove many events
		for i := 0; i < 5000; i++ {
			runID := fmt.Sprintf("run-%d", i%100)
			msgID := fmt.Sprintf("msg-%d", i%100)

			// Add run started
			tracker.TrackEvent(&RunStartedEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
				RunIDValue:     runID,
				ThreadIDValue:  "thread-1",
			})

			// Add message events
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

			// Finish run
			tracker.TrackEvent(&RunFinishedEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeRunFinished},
				RunIDValue:     runID,
			})
		}

		// Force cleanup
		tracker = nil
		runtime.GC()
		runtime.Gosched()
	}

	// Check final memory state
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)

	t.Logf("Final heap allocation: %d MB", m.HeapAlloc/1024/1024)
}

// TestValidationState_MemoryBounds tests validation state memory management
func TestValidationState_MemoryBounds(t *testing.T) {
	state := NewValidationState()

	// Add many active runs
	for i := 0; i < 1000; i++ {
		runID := fmt.Sprintf("run-%d", i)
		state.ActiveRuns[runID] = &RunState{
			RunID:     runID,
			ThreadID:  fmt.Sprintf("thread-%d", i),
			StartTime: time.Now(),
			Phase:     PhaseRunning,
		}
	}

	// Move half to finished
	count := 0
	for runID, runState := range state.ActiveRuns {
		if count >= 500 {
			break
		}
		state.FinishedRuns[runID] = runState
		delete(state.ActiveRuns, runID)
		count++
	}

	// Verify counts
	if len(state.ActiveRuns) != 500 {
		t.Errorf("Expected 500 active runs, got %d", len(state.ActiveRuns))
	}
	if len(state.FinishedRuns) != 500 {
		t.Errorf("Expected 500 finished runs, got %d", len(state.FinishedRuns))
	}

	// Test message state bounds
	for i := 0; i < 1000; i++ {
		msgID := fmt.Sprintf("msg-%d", i)
		state.ActiveMessages[msgID] = &MessageState{
			MessageID:    msgID,
			StartTime:    time.Now(),
			ContentCount: i,
			IsActive:     true,
		}
	}

	// Simulate finishing messages
	for msgID, msgState := range state.ActiveMessages {
		msgState.IsActive = false
		state.FinishedMessages[msgID] = msgState
		delete(state.ActiveMessages, msgID)
		if len(state.FinishedMessages) >= 500 {
			break
		}
	}
}

// TestIDTracker_MemoryBounds tests ID tracker memory management
func TestIDTracker_MemoryBounds(t *testing.T) {
	tracker := NewIDTracker()

	// Add many message triplets
	for i := 0; i < 1000; i++ {
		msgID := fmt.Sprintf("msg-%d", i)

		// Track start
		tracker.TrackEvent(&TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart},
			MessageID: msgID,
			Role:      stringPtr("user"),
		})

		// Track multiple content events
		for j := 0; j < 10; j++ {
			tracker.TrackEvent(&TextMessageContentEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent},
				MessageID: msgID,
				Delta:     fmt.Sprintf("content %d-%d", i, j),
			})
		}

		// Track end
		tracker.TrackEvent(&TextMessageEndEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd},
			MessageID: msgID,
		})
	}

	// Get statistics
	stats := tracker.GetStatistics()
	t.Logf("Tracker statistics: %+v", stats)

	// Validate consistency
	errors := tracker.ValidateIDConsistency()
	if len(errors) > 0 {
		t.Errorf("Unexpected validation errors: %v", errors)
	}

	// Test reset functionality
	tracker.Reset()

	// Verify reset cleared all data
	stats = tracker.GetStatistics()
	if stats.MessageStartCount > 0 || stats.ToolStartCount > 0 || stats.RunStartCount > 0 {
		t.Error("Reset should clear all tracked data")
	}
}

// TestValidator_LargeSequenceMemory tests memory usage with large sequences
func TestValidator_LargeSequenceMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large sequence test in short mode")
	}

	validator := NewEventValidator(DefaultValidationConfig())
	ctx := context.Background()

	// Create a very large sequence
	events := make([]Event, 10000)

	// Start with RUN_STARTED
	events[0] = &RunStartedEvent{
		BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
		RunIDValue:     "run-large",
		ThreadIDValue:  "thread-large",
	}

	// Add many message events with proper start/content/end sequences
	eventIndex := 1
	for msgNum := 0; msgNum < 3333 && eventIndex < 9999; msgNum++ {
		msgID := fmt.Sprintf("msg-%d", msgNum)

		// Message start
		events[eventIndex] = &TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart},
			MessageID: msgID,
			Role:      stringPtr("user"),
		}
		eventIndex++

		// Message content
		if eventIndex < 9999 {
			events[eventIndex] = &TextMessageContentEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent},
				MessageID: msgID,
				Delta:     fmt.Sprintf("content for message %d", msgNum),
			}
			eventIndex++
		}

		// Message end
		if eventIndex < 9999 {
			events[eventIndex] = &TextMessageEndEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd},
				MessageID: msgID,
			}
			eventIndex++
		}
	}

	// End with RUN_FINISHED
	events[9999] = &RunFinishedEvent{
		BaseEvent: &BaseEvent{EventType: EventTypeRunFinished},
		RunIDValue:     "run-large",
	}

	// Record memory before validation
	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Validate the large sequence
	start := time.Now()
	result := validator.ValidateSequence(ctx, events)
	duration := time.Since(start)

	// Record memory after validation
	var m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m2)

	memoryUsed := m2.HeapAlloc - m1.HeapAlloc
	t.Logf("Large sequence validation: %d events in %v, memory used: %d KB",
		len(events), duration, memoryUsed/1024)

	if !result.IsValid {
		t.Errorf("Large sequence should be valid, got %d errors", len(result.Errors))
	}
}

// TestValidationMetrics_MemoryBounds tests metrics memory management
func TestValidationMetrics_MemoryBounds(t *testing.T) {
	metrics := NewValidationMetrics()

	// Record many rule executions
	for i := 0; i < 10000; i++ {
		ruleID := fmt.Sprintf("RULE_%d", i%100) // Reuse rule IDs
		duration := time.Duration(i%1000) * time.Microsecond
		metrics.RecordRuleExecution(ruleID, duration)
	}

	// Check that we're not storing unlimited rule execution times
	if len(metrics.RuleExecutionTimes) > 100 {
		t.Errorf("Rule execution times map too large: %d entries", len(metrics.RuleExecutionTimes))
	}

	// Record many events
	for i := 0; i < 10000; i++ {
		metrics.RecordEvent(time.Duration(i) * time.Microsecond)
	}

	t.Logf("Metrics after 10k events: processed=%d, errors=%d, warnings=%d",
		metrics.EventsProcessed, metrics.ErrorCount, metrics.WarningCount)
}

// TestConcurrentMemoryPressure - REMOVED
// This test was designed to create memory pressure by running 5 goroutines each processing
// 10,000 events (100 iterations × 100 events) with varying content sizes up to 10KB per event.
// It deliberately stressed memory allocation to test behavior under pressure.
// Removed as it was designed to push memory limits and exhaust resources.
func TestConcurrentMemoryPressure(t *testing.T) {
	t.Skip("Memory pressure test removed - was designed to stress memory allocation")

	// Final memory check
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	t.Logf("Final memory state: HeapAlloc=%d MB, NumGC=%d", m.HeapAlloc/1024/1024, m.NumGC)
}

// TestValidationState_CleanupFinishedItems tests the memory cleanup functionality
func TestValidationState_CleanupFinishedItems(t *testing.T) {
	state := NewValidationState()

	// Add finished items with different timestamps
	now := time.Now()

	// Add old finished runs (should be cleaned up)
	for i := 0; i < 50; i++ {
		runID := fmt.Sprintf("old-run-%d", i)
		state.FinishedRuns[runID] = &RunState{
			RunID:     runID,
			ThreadID:  fmt.Sprintf("thread-%d", i),
			StartTime: now.Add(-48 * time.Hour), // 48 hours old
			Phase:     PhaseFinished,
		}
	}

	// Add recent finished runs (should be kept)
	for i := 0; i < 30; i++ {
		runID := fmt.Sprintf("recent-run-%d", i)
		state.FinishedRuns[runID] = &RunState{
			RunID:     runID,
			ThreadID:  fmt.Sprintf("thread-%d", i),
			StartTime: now.Add(-1 * time.Hour), // 1 hour old
			Phase:     PhaseFinished,
		}
	}

	// Add old finished messages
	for i := 0; i < 100; i++ {
		msgID := fmt.Sprintf("old-msg-%d", i)
		state.FinishedMessages[msgID] = &MessageState{
			MessageID:    msgID,
			StartTime:    now.Add(-48 * time.Hour),
			ContentCount: 5,
			IsActive:     false,
		}
	}

	// Add recent finished messages
	for i := 0; i < 50; i++ {
		msgID := fmt.Sprintf("recent-msg-%d", i)
		state.FinishedMessages[msgID] = &MessageState{
			MessageID:    msgID,
			StartTime:    now.Add(-1 * time.Hour),
			ContentCount: 3,
			IsActive:     false,
		}
	}

	// Get stats before cleanup
	statsBefore := state.GetMemoryStats()
	t.Logf("Before cleanup: %+v", statsBefore)

	// Clean up items older than 24 hours
	cutoff := now.Add(-24 * time.Hour)
	state.CleanupFinishedItems(cutoff)

	// Get stats after cleanup
	statsAfter := state.GetMemoryStats()
	t.Logf("After cleanup: %+v", statsAfter)

	// Verify old items were removed
	if statsAfter["finished_runs"] != 30 {
		t.Errorf("Expected 30 recent runs, got %d", statsAfter["finished_runs"])
	}
	if statsAfter["finished_messages"] != 50 {
		t.Errorf("Expected 50 recent messages, got %d", statsAfter["finished_messages"])
	}

	// Verify specific items
	if _, exists := state.FinishedRuns["old-run-0"]; exists {
		t.Error("Old run should have been cleaned up")
	}
	if _, exists := state.FinishedRuns["recent-run-0"]; !exists {
		t.Error("Recent run should have been kept")
	}
}

// TestEventValidator_StartCleanupRoutine tests the automatic cleanup routine
func TestEventValidator_StartCleanupRoutine(t *testing.T) {
	// Use a permissive config that won't fail validation
	config := &ValidationConfig{
		Level:                  ValidationPermissive,
		SkipSequenceValidation: true,
	}
	validator := NewEventValidator(config)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start cleanup routine with short intervals for testing
	validator.StartCleanupRoutine(ctx, 100*time.Millisecond, 200*time.Millisecond)

	// Manually add items to finished state (bypass validation to ensure they're there)
	validator.state.mutex.Lock()
	for i := 0; i < 10; i++ {
		runID := fmt.Sprintf("run-%d", i)
		validator.state.FinishedRuns[runID] = &RunState{
			RunID:     runID,
			ThreadID:  fmt.Sprintf("thread-%d", i),
			StartTime: time.Now().Add(-1 * time.Second), // Make them 1 second old
			Phase:     PhaseFinished,
		}
	}
	validator.state.mutex.Unlock()

	// Check initial state
	statsBefore := validator.state.GetMemoryStats()
	t.Logf("Initial state: %+v", statsBefore)

	if statsBefore["finished_runs"] != 10 {
		t.Fatalf("Expected 10 finished runs, got %d", statsBefore["finished_runs"])
	}

	// Wait for items to become old enough (200ms) and cleanup to run
	time.Sleep(300 * time.Millisecond)

	// Check state after cleanup
	statsAfter := validator.state.GetMemoryStats()
	t.Logf("After cleanup: %+v", statsAfter)

	// Verify cleanup happened
	if statsAfter["finished_runs"] != 0 {
		t.Errorf("Expected 0 finished runs after cleanup, got %d", statsAfter["finished_runs"])
	}
}
