package events

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test Configuration and Setup
const (
	// Performance targets
	targetValidationLatency = time.Millisecond
	targetThroughput        = 10000 // events per second
	
	// Memory and concurrency limits
	maxMemoryUsageMB = 100
	maxGoroutines    = 1000
	
	// Test data sizes
	smallDatasetSize  = 100
	mediumDatasetSize = 1000
	largeDatasetSize  = 10000
	
	// Fuzzing parameters
	fuzzIterations     = 1000
	fuzzComplexity     = 10
	maxPropertyTests   = 100
)

// Test Fixtures and Helpers

type TestFixture struct {
	events     []Event
	validator  *EventValidator
	debugger   *ValidationDebugger
	metrics    *ValidationPerformanceMetrics
	optimizer  *PerformanceOptimizer
}

func setupTestFixture(t testing.TB) *TestFixture {
	// Create enhanced validator with all rules
	config := DefaultValidationConfig()
	validator := NewEventValidator(config)
	
	// Add all validation rules
	validator.AddDefaultRules()
	
	// Add enhanced rules
	protocolRule := NewProtocolSequenceRule()
	timingRule := NewEventTimingRule(DefaultTimingConstraints())
	stateRule := NewStateTransitionRule(DefaultStateTransitionConfig())
	
	validator.AddRule(protocolRule)
	validator.AddRule(timingRule)
	validator.AddRule(stateRule)
	
	// Setup debugging
	debugger := NewValidationDebugger(DebugLevelDebug, t.TempDir())
	
	// Setup metrics
	metricsConfig := DefaultMetricsConfig()
	metricsConfig.Level = MetricsLevelDetailed
	metrics, err := NewValidationPerformanceMetrics(metricsConfig)
	require.NoError(t, err)
	
	// Setup performance optimizer
	perfConfig := DefaultPerformanceConfig()
	perfConfig.Mode = BalancedMode
	optimizer, err := NewPerformanceOptimizer(perfConfig)
	require.NoError(t, err)
	
	return &TestFixture{
		events:    generateTestEventSequence(100),
		validator: validator,
		debugger:  debugger,
		metrics:   metrics,
		optimizer: optimizer,
	}
}

func (f *TestFixture) Cleanup() {
	if f.metrics != nil {
		f.metrics.Shutdown()
	}
	if f.optimizer != nil {
		f.optimizer.Stop()
	}
}

func generateTestEventSequence(count int) []Event {
	events := make([]Event, 0, count)
	
	// Start with RUN_STARTED
	runEvent := &RunStartedEvent{
		BaseEvent: &BaseEvent{
			EventType: EventTypeRunStarted,
			TimestampMs: timePtrEnhanced(time.Now().UnixMilli()),
		},
		RunID:    fmt.Sprintf("run_%d", time.Now().UnixNano()),
		ThreadID: fmt.Sprintf("thread_%d", time.Now().UnixNano()),
	}
	events = append(events, runEvent)
	
	// Add messages and tool calls
	for i := 1; i < count-1; i++ {
		var event Event
		eventType := rand.Intn(6)
		
		switch eventType {
		case 0:
			event = &TextMessageStartEvent{
				BaseEvent: &BaseEvent{
					EventType: EventTypeTextMessageStart,
					TimestampMs: timePtrEnhanced(time.Now().Add(time.Duration(i)*time.Millisecond).UnixMilli()),
				},
				MessageID: fmt.Sprintf("msg_%d", i),
			}
		case 1:
			event = &TextMessageContentEvent{
				BaseEvent: &BaseEvent{
					EventType: EventTypeTextMessageContent,
					TimestampMs: timePtrEnhanced(time.Now().Add(time.Duration(i)*time.Millisecond).UnixMilli()),
				},
				MessageID: fmt.Sprintf("msg_%d", i/2),
				Delta:     fmt.Sprintf("content_%d", i),
			}
		case 2:
			event = &TextMessageEndEvent{
				BaseEvent: &BaseEvent{
					EventType: EventTypeTextMessageEnd,
					TimestampMs: timePtrEnhanced(time.Now().Add(time.Duration(i)*time.Millisecond).UnixMilli()),
				},
				MessageID: fmt.Sprintf("msg_%d", i/3),
			}
		case 3:
			event = &ToolCallStartEvent{
				BaseEvent: &BaseEvent{
					EventType: EventTypeToolCallStart,
					TimestampMs: timePtrEnhanced(time.Now().Add(time.Duration(i)*time.Millisecond).UnixMilli()),
				},
				ToolCallID:   fmt.Sprintf("tool_%d", i),
				ToolCallName: fmt.Sprintf("tool_function_%d", i%5),
			}
		case 4:
			event = &ToolCallArgsEvent{
				BaseEvent: &BaseEvent{
					EventType: EventTypeToolCallArgs,
					TimestampMs: timePtrEnhanced(time.Now().Add(time.Duration(i)*time.Millisecond).UnixMilli()),
				},
				ToolCallID: fmt.Sprintf("tool_%d", i/2),
				Delta:      fmt.Sprintf(`{"arg_%d": "value_%d"}`, i, i),
			}
		case 5:
			event = &ToolCallEndEvent{
				BaseEvent: &BaseEvent{
					EventType: EventTypeToolCallEnd,
					TimestampMs: timePtrEnhanced(time.Now().Add(time.Duration(i)*time.Millisecond).UnixMilli()),
				},
				ToolCallID: fmt.Sprintf("tool_%d", i/3),
			}
		}
		
		events = append(events, event)
	}
	
	// End with RUN_FINISHED
	finishEvent := &RunFinishedEvent{
		BaseEvent: &BaseEvent{
			EventType: EventTypeRunFinished,
			TimestampMs: timePtrEnhanced(time.Now().Add(time.Duration(count)*time.Millisecond).UnixMilli()),
		},
		RunID: runEvent.RunID,
	}
	events = append(events, finishEvent)
	
	return events
}

func timePtrEnhanced(t int64) *int64 {
	return &t
}

func stringPtrEnhanced(s string) *string {
	return &s
}

// Unit Tests for Validation Rules

func TestProtocolSequenceValidation(t *testing.T) {
	tests := []struct {
		name           string
		events         []Event
		expectValid    bool
		expectedErrors int
	}{
		{
			name: "Valid run sequence",
			events: []Event{
				&RunStartedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtrEnhanced(1)},
					RunID:     "test-run",
					ThreadID:  "test-thread",
				},
				&RunFinishedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtrEnhanced(2)},
					RunID:     "test-run",
				},
			},
			expectValid:    true,
			expectedErrors: 0,
		},
		{
			name: "Invalid sequence - no run started",
			events: []Event{
				&TextMessageStartEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtrEnhanced(1)},
					MessageID: "msg-1",
				},
			},
			expectValid:    false,
			expectedErrors: 1,
		},
		{
			name: "Invalid message sequence - content before start",
			events: []Event{
				&RunStartedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtrEnhanced(1)},
					RunID:     "test-run",
					ThreadID:  "test-thread",
				},
				&TextMessageContentEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(2)},
					MessageID: "msg-1",
					Delta:     "content",
				},
			},
			expectValid:    false,
			expectedErrors: 1,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewProtocolSequenceRule()
			validator := NewEventValidator(DefaultValidationConfig())
			validator.AddRule(rule)
			
			result := validator.ValidateSequence(context.Background(), tt.events)
			
			assert.Equal(t, tt.expectValid, result.IsValid)
			assert.Equal(t, tt.expectedErrors, len(result.Errors))
		})
	}
}

func TestTimingConstraintsValidation(t *testing.T) {
	tests := []struct {
		name           string
		constraints    *TimingConstraints
		events         []Event
		expectValid    bool
		expectedErrors int
	}{
		{
			name:        "Valid timing",
			constraints: DefaultTimingConstraints(),
			events: []Event{
				&RunStartedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtrEnhanced(time.Now().UnixMilli())},
					RunID:     "test-run",
					ThreadID:  "test-thread",
				},
				&RunFinishedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtrEnhanced(time.Now().Add(time.Second).UnixMilli())},
					RunID:     "test-run",
				},
			},
			expectValid:    true,
			expectedErrors: 0,
		},
		{
			name:        "Rate limit exceeded",
			constraints: StrictTimingConstraints(),
			events:      generateHighFrequencyEvents(100),
			expectValid: false,
		},
		{
			name:        "Timestamp drift",
			constraints: StrictTimingConstraints(),
			events: []Event{
				&RunStartedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtrEnhanced(time.Now().Add(time.Hour).UnixMilli())},
					RunID:     "test-run",
					ThreadID:  "test-thread",
				},
			},
			expectValid:    false,
			expectedErrors: 1,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewEventTimingRule(tt.constraints)
			validator := NewEventValidator(DefaultValidationConfig())
			validator.AddRule(rule)
			
			result := validator.ValidateSequence(context.Background(), tt.events)
			
			assert.Equal(t, tt.expectValid, result.IsValid)
			if tt.expectedErrors > 0 {
				assert.GreaterOrEqual(t, len(result.Errors), tt.expectedErrors)
			}
		})
	}
}

func generateHighFrequencyEvents(count int) []Event {
	events := make([]Event, count)
	baseTime := time.Now()
	
	for i := 0; i < count; i++ {
		events[i] = &RunStartedEvent{
			BaseEvent: &BaseEvent{
				EventType: EventTypeRunStarted,
				TimestampMs: timePtrEnhanced(baseTime.Add(time.Duration(i)*time.Millisecond).UnixMilli()),
			},
			RunID:    fmt.Sprintf("run_%d", i),
			ThreadID: fmt.Sprintf("thread_%d", i),
		}
	}
	
	return events
}

func TestStateTransitionValidation(t *testing.T) {
	tests := []struct {
		name           string
		config         *StateTransitionConfig
		events         []Event
		expectValid    bool
		expectedErrors int
	}{
		{
			name:   "Valid state transitions",
			config: DefaultStateTransitionConfig(),
			events: []Event{
				&RunStartedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtrEnhanced(1)},
					RunID:     "test-run",
					ThreadID:  "test-thread",
				},
				&TextMessageStartEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtrEnhanced(2)},
					MessageID: "msg-1",
				},
				&TextMessageEndEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd, TimestampMs: timePtrEnhanced(3)},
					MessageID: "msg-1",
				},
				&RunFinishedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtrEnhanced(4)},
					RunID:     "test-run",
				},
			},
			expectValid:    true,
			expectedErrors: 0,
		},
		{
			name:   "Invalid state transition - duplicate run start",
			config: DefaultStateTransitionConfig(),
			events: []Event{
				&RunStartedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtrEnhanced(1)},
					RunID:     "test-run",
					ThreadID:  "test-thread",
				},
				&RunStartedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtrEnhanced(2)},
					RunID:     "test-run",
					ThreadID:  "test-thread",
				},
			},
			expectValid:    false,
			expectedErrors: 1,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewStateTransitionRule(tt.config)
			validator := NewEventValidator(DefaultValidationConfig())
			validator.AddRule(rule)
			
			result := validator.ValidateSequence(context.Background(), tt.events)
			
			assert.Equal(t, tt.expectValid, result.IsValid)
			assert.Equal(t, tt.expectedErrors, len(result.Errors))
		})
	}
}

func TestContentValidation(t *testing.T) {
	tests := []struct {
		name           string
		event          Event
		expectValid    bool
		expectedErrors int
	}{
		{
			name: "Valid content",
			event: &TextMessageContentEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(1)},
				MessageID: "msg-1",
				Delta:     "Valid content",
			},
			expectValid:    true,
			expectedErrors: 0,
		},
		{
			name: "Content with null bytes",
			event: &TextMessageContentEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(1)},
				MessageID: "msg-1",
				Delta:     "Content with\x00null bytes",
			},
			expectValid:    true, // Should be warning, not error
			expectedErrors: 0,
		},
		{
			name: "Very long content",
			event: &TextMessageContentEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(1)},
				MessageID: "msg-1",
				Delta:     strings.Repeat("a", 100000),
			},
			expectValid:    true, // Should be warning, not error
			expectedErrors: 0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewContentValidationRule()
			validator := NewEventValidator(DefaultValidationConfig())
			validator.AddRule(rule)
			
			result := validator.ValidateEvent(context.Background(), tt.event)
			
			assert.Equal(t, tt.expectValid, result.IsValid)
			assert.Equal(t, tt.expectedErrors, len(result.Errors))
		})
	}
}

func TestIDValidation(t *testing.T) {
	tests := []struct {
		name           string
		event          Event
		expectValid    bool
		expectedErrors int
	}{
		{
			name: "Valid ID format",
			event: &RunStartedEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtrEnhanced(1)},
				RunID:     "run-123",
				ThreadID:  "thread-456",
			},
			expectValid:    true,
			expectedErrors: 0,
		},
		{
			name: "Empty ID",
			event: &RunStartedEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtrEnhanced(1)},
				RunID:     "",
				ThreadID:  "thread-456",
			},
			expectValid:    false,
			expectedErrors: 1,
		},
		{
			name: "Special characters in ID",
			event: &RunStartedEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtrEnhanced(1)},
				RunID:     "run@#$%",
				ThreadID:  "thread-456",
			},
			expectValid:    true, // Should be warning, not error for format
			expectedErrors: 0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewIDFormatRule()
			validator := NewEventValidator(DefaultValidationConfig())
			validator.AddRule(rule)
			
			result := validator.ValidateEvent(context.Background(), tt.event)
			
			assert.Equal(t, tt.expectValid, result.IsValid)
			assert.Equal(t, tt.expectedErrors, len(result.Errors))
		})
	}
}

// Integration Tests

func TestCompleteEventSequenceValidation(t *testing.T) {
	fixture := setupTestFixture(t)
	defer fixture.Cleanup()
	
	t.Run("Valid complete sequence", func(t *testing.T) {
		events := generateValidEventSequence()
		
		start := time.Now()
		result := fixture.validator.ValidateSequence(context.Background(), events)
		duration := time.Since(start)
		
		assert.True(t, result.IsValid, "Expected valid sequence")
		assert.Empty(t, result.Errors, "Expected no errors")
		assert.Less(t, duration, 100*time.Millisecond, "Validation should be fast")
		
		t.Logf("Validated %d events in %v", len(events), duration)
	})
	
	t.Run("Invalid sequence with various errors", func(t *testing.T) {
		events := generateInvalidEventSequence()
		
		result := fixture.validator.ValidateSequence(context.Background(), events)
		
		assert.False(t, result.IsValid, "Expected invalid sequence")
		assert.NotEmpty(t, result.Errors, "Expected validation errors")
		
		// Verify error types
		errorTypes := make(map[string]int)
		for _, err := range result.Errors {
			errorTypes[err.RuleID]++
		}
		
		t.Logf("Error distribution: %+v", errorTypes)
	})
}

func generateValidEventSequence() []Event {
	now := time.Now()
	
	return []Event{
		&RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtrEnhanced(now.UnixMilli())},
			RunID:     "run-valid",
			ThreadID:  "thread-valid",
		},
		&TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtrEnhanced(now.Add(10*time.Millisecond).UnixMilli())},
			MessageID: "msg-1",
		},
		&TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(now.Add(20*time.Millisecond).UnixMilli())},
			MessageID: "msg-1",
			Delta:     "Hello, world!",
		},
		&TextMessageEndEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd, TimestampMs: timePtrEnhanced(now.Add(30*time.Millisecond).UnixMilli())},
			MessageID: "msg-1",
		},
		&ToolCallStartEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeToolCallStart, TimestampMs: timePtrEnhanced(now.Add(40*time.Millisecond).UnixMilli())},
			ToolCallID:   "tool-1",
			ToolCallName: "test_function",
		},
		&ToolCallArgsEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeToolCallArgs, TimestampMs: timePtrEnhanced(now.Add(50*time.Millisecond).UnixMilli())},
			ToolCallID: "tool-1",
			Delta:      `{"param": "value"}`,
		},
		&ToolCallEndEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeToolCallEnd, TimestampMs: timePtrEnhanced(now.Add(60*time.Millisecond).UnixMilli())},
			ToolCallID: "tool-1",
		},
		&RunFinishedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtrEnhanced(now.Add(70*time.Millisecond).UnixMilli())},
			RunID:     "run-valid",
		},
	}
}

func generateInvalidEventSequence() []Event {
	now := time.Now()
	
	return []Event{
		// Missing RunStarted
		&TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtrEnhanced(now.UnixMilli())},
			MessageID: "", // Empty ID
		},
		&TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(now.Add(10*time.Millisecond).UnixMilli())},
			MessageID: "msg-different", // Different ID
			Delta:     "", // Empty content
		},
		&ToolCallArgsEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeToolCallArgs, TimestampMs: timePtrEnhanced(now.Add(20*time.Millisecond).UnixMilli())},
			ToolCallID: "tool-nonexistent", // No corresponding start
			Delta:      "invalid json",
		},
	}
}

func TestConcurrentValidation(t *testing.T) {
	fixture := setupTestFixture(t)
	defer fixture.Cleanup()
	
	numGoroutines := 50
	
	var wg sync.WaitGroup
	results := make(chan *ValidationResult, numGoroutines)
	
	start := time.Now()
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			events := generateValidEventSequence()
			result := fixture.validator.ValidateSequence(context.Background(), events)
			results <- result
		}(i)
	}
	
	wg.Wait()
	close(results)
	
	duration := time.Since(start)
	
	// Collect results
	var totalEvents int
	var validResults int
	
	for result := range results {
		totalEvents += result.EventCount
		if result.IsValid {
			validResults++
		}
	}
	
	throughput := float64(totalEvents) / duration.Seconds()
	
	assert.Equal(t, numGoroutines, validResults, "All concurrent validations should succeed")
	assert.Greater(t, throughput, float64(targetThroughput)/2, "Throughput should meet minimum requirements")
	
	t.Logf("Concurrent validation: %d goroutines, %d total events, %.2f events/sec", 
		numGoroutines, totalEvents, throughput)
}

// Performance Benchmarks

func BenchmarkSingleEventValidation(b *testing.B) {
	fixture := setupTestFixture(b)
	defer fixture.Cleanup()
	
	event := &TextMessageContentEvent{
		BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(time.Now().UnixMilli())},
		MessageID: "msg-1",
		Delta:     "Test content",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		result := fixture.validator.ValidateEvent(context.Background(), event)
		if !result.IsValid {
			b.Fatal("Validation should succeed")
		}
	}
}

func BenchmarkSequenceValidation(b *testing.B) {
	fixture := setupTestFixture(b)
	defer fixture.Cleanup()
	
	events := generateValidEventSequence()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		result := fixture.validator.ValidateSequence(context.Background(), events)
		if !result.IsValid {
			b.Fatal("Validation should succeed")
		}
	}
}

func BenchmarkParallelValidation(b *testing.B) {
	fixture := setupTestFixture(b)
	defer fixture.Cleanup()
	
	events := generateValidEventSequence()
	
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result := fixture.validator.ValidateSequence(context.Background(), events)
			if !result.IsValid {
				b.Fatal("Validation should succeed")
			}
		}
	})
}

func BenchmarkValidationWithCaching(b *testing.B) {
	fixture := setupTestFixture(b)
	defer fixture.Cleanup()
	
	// Enable caching
	fixture.optimizer.Start()
	defer fixture.optimizer.Stop()
	
	event := &TextMessageContentEvent{
		BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(time.Now().UnixMilli())},
		MessageID: "msg-1",
		Delta:     "Test content",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		// Use optimizer for validation
		err := fixture.optimizer.OptimizeValidation(event, fixture.validator.GetRules())
		if err != nil {
			b.Fatal("Validation should succeed")
		}
	}
}

func BenchmarkLargeSequenceValidation(b *testing.B) {
	fixture := setupTestFixture(b)
	defer fixture.Cleanup()
	
	events := generateTestEventSequence(largeDatasetSize)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		result := fixture.validator.ValidateSequence(context.Background(), events)
		if !result.IsValid {
			b.Logf("Validation failed with %d errors", len(result.Errors))
		}
	}
}

// Fuzzing Tests

func TestFuzzEventValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping fuzz test in short mode")
	}
	
	fixture := setupTestFixture(t)
	defer fixture.Cleanup()
	
	t.Run("Fuzz single events", func(t *testing.T) {
		for i := 0; i < fuzzIterations; i++ {
			event := generateRandomEvent()
			
			// Validation should not panic
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Validation panicked on iteration %d: %v", i, r)
					}
				}()
				
				result := fixture.validator.ValidateEvent(context.Background(), event)
				_ = result // We don't care about the result, just that it doesn't panic
			}()
		}
	})
	
	t.Run("Fuzz event sequences", func(t *testing.T) {
		for i := 0; i < fuzzIterations/10; i++ {
			events := generateRandomEventSequence(rand.Intn(50) + 1)
			
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Sequence validation panicked on iteration %d: %v", i, r)
					}
				}()
				
				result := fixture.validator.ValidateSequence(context.Background(), events)
				_ = result
			}()
		}
	})
}

func generateRandomEvent() Event {
	eventTypes := []EventType{
		EventTypeRunStarted,
		EventTypeRunFinished,
		EventTypeRunError,
		EventTypeTextMessageStart,
		EventTypeTextMessageContent,
		EventTypeTextMessageEnd,
		EventTypeToolCallStart,
		EventTypeToolCallArgs,
		EventTypeToolCallEnd,
		EventTypeStateSnapshot,
		EventTypeStateDelta,
	}
	
	eventType := eventTypes[rand.Intn(len(eventTypes))]
	timestamp := time.Now().Add(time.Duration(rand.Intn(1000)) * time.Millisecond).UnixMilli()
	
	switch eventType {
	case EventTypeRunStarted:
		return &RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			RunID:     generateRandomString(rand.Intn(50)),
			ThreadID:  generateRandomString(rand.Intn(50)),
		}
	case EventTypeRunFinished:
		return &RunFinishedEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			RunID:     generateRandomString(rand.Intn(50)),
		}
	case EventTypeRunError:
		return &RunErrorEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			RunID:     generateRandomString(rand.Intn(50)),
			Message:   generateRandomString(rand.Intn(200)),
		}
	case EventTypeTextMessageStart:
		return &TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			MessageID: generateRandomString(rand.Intn(50)),
		}
	case EventTypeTextMessageContent:
		return &TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			MessageID: generateRandomString(rand.Intn(50)),
			Delta:     generateRandomString(rand.Intn(1000)),
		}
	case EventTypeTextMessageEnd:
		return &TextMessageEndEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			MessageID: generateRandomString(rand.Intn(50)),
		}
	case EventTypeToolCallStart:
		return &ToolCallStartEvent{
			BaseEvent:    &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			ToolCallID:   generateRandomString(rand.Intn(50)),
			ToolCallName: generateRandomString(rand.Intn(50)),
		}
	case EventTypeToolCallArgs:
		return &ToolCallArgsEvent{
			BaseEvent:  &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			ToolCallID: generateRandomString(rand.Intn(50)),
			Delta:      generateRandomString(rand.Intn(500)),
		}
	case EventTypeToolCallEnd:
		return &ToolCallEndEvent{
			BaseEvent:  &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			ToolCallID: generateRandomString(rand.Intn(50)),
		}
	case EventTypeStateSnapshot:
		return &StateSnapshotEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			Snapshot:  generateRandomJSON(),
		}
	case EventTypeStateDelta:
		return &StateDeltaEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			Delta:     generateRandomJSONPatch(),
		}
	default:
		return &RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: &timestamp},
			RunID:     "default",
			ThreadID:  "default",
		}
	}
}

func generateRandomEventSequence(count int) []Event {
	events := make([]Event, count)
	for i := 0; i < count; i++ {
		events[i] = generateRandomEvent()
	}
	return events
}

func generateRandomString(maxLen int) string {
	if maxLen == 0 {
		return ""
	}
	
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	length := rand.Intn(maxLen) + 1
	result := make([]byte, length)
	
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	
	// Sometimes inject special characters for edge case testing
	if rand.Float32() < 0.1 {
		specialChars := []byte{0, '\n', '\t', '"', '\\', '\x7f'}
		if length > 0 {
			result[rand.Intn(length)] = specialChars[rand.Intn(len(specialChars))]
		}
	}
	
	return string(result)
}

func generateRandomJSON() map[string]interface{} {
	result := make(map[string]interface{})
	numFields := rand.Intn(10) + 1
	
	for i := 0; i < numFields; i++ {
		key := generateRandomString(20)
		switch rand.Intn(4) {
		case 0:
			result[key] = generateRandomString(100)
		case 1:
			result[key] = rand.Int63()
		case 2:
			result[key] = rand.Float64()
		case 3:
			result[key] = rand.Int()%2 == 0
		}
	}
	
	return result
}

func generateRandomJSONPatch() []JSONPatchOperation {
	numOps := rand.Intn(5) + 1
	ops := make([]JSONPatchOperation, numOps)
	
	for i := 0; i < numOps; i++ {
		opTypes := []string{"add", "remove", "replace", "move", "copy", "test"}
		ops[i] = JSONPatchOperation{
			Op:    opTypes[rand.Intn(len(opTypes))],
			Path:  "/" + generateRandomString(20),
			Value: generateRandomString(50),
		}
		
		if ops[i].Op == "move" || ops[i].Op == "copy" {
			ops[i].From = "/" + generateRandomString(20)
		}
	}
	
	return ops
}

// Property-Based Testing

func TestPropertyBasedValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping property-based tests in short mode")
	}
	
	fixture := setupTestFixture(t)
	defer fixture.Cleanup()
	
	t.Run("Property: Valid sequences never produce panics", func(t *testing.T) {
		for i := 0; i < maxPropertyTests; i++ {
			events := generatePropertyValidEventSequence()
			
			assert.NotPanics(t, func() {
				result := fixture.validator.ValidateSequence(context.Background(), events)
				_ = result
			})
		}
	})
	
	t.Run("Property: Event validation is deterministic", func(t *testing.T) {
		for i := 0; i < maxPropertyTests; i++ {
			event := generateRandomEvent()
			
			result1 := fixture.validator.ValidateEvent(context.Background(), event)
			result2 := fixture.validator.ValidateEvent(context.Background(), event)
			
			assert.Equal(t, result1.IsValid, result2.IsValid, "Validation should be deterministic")
			assert.Equal(t, len(result1.Errors), len(result2.Errors), "Error count should be deterministic")
		}
	})
	
	t.Run("Property: Valid run lifecycle preserves order", func(t *testing.T) {
		for i := 0; i < maxPropertyTests; i++ {
			runID := fmt.Sprintf("run_%d", i)
			threadID := fmt.Sprintf("thread_%d", i)
			
			events := []Event{
				&RunStartedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtrEnhanced(1)},
					RunID:     runID,
					ThreadID:  threadID,
				},
				&RunFinishedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtrEnhanced(2)},
					RunID:     runID,
				},
			}
			
			result := fixture.validator.ValidateSequence(context.Background(), events)
			assert.True(t, result.IsValid, "Valid run lifecycle should always pass")
		}
	})
	
	t.Run("Property: Timestamp ordering invariant", func(t *testing.T) {
		for i := 0; i < maxPropertyTests; i++ {
			events := generateOrderedTimestampSequence(rand.Intn(20) + 5)
			
			result := fixture.validator.ValidateSequence(context.Background(), events)
			
			// If events have proper ordering, timing rule should not fail on order
			hasTimingErrors := false
			for _, err := range result.Errors {
				if strings.Contains(err.RuleID, "TIMING") && strings.Contains(err.Message, "order") {
					hasTimingErrors = true
					break
				}
			}
			
			assert.False(t, hasTimingErrors, "Properly ordered timestamps should not cause timing errors")
		}
	})
}

func generatePropertyValidEventSequence() []Event {
	// Generate a sequence that follows protocol rules
	runID := fmt.Sprintf("run_%d", time.Now().UnixNano())
	threadID := fmt.Sprintf("thread_%d", time.Now().UnixNano())
	baseTime := time.Now()
	
	events := []Event{
		&RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtrEnhanced(baseTime.UnixMilli())},
			RunID:     runID,
			ThreadID:  threadID,
		},
	}
	
	// Add some message/tool sequences
	numSequences := rand.Intn(5) + 1
	currentTime := baseTime.Add(10 * time.Millisecond)
	
	for i := 0; i < numSequences; i++ {
		if rand.Float32() < 0.5 {
			// Add message sequence
			msgID := fmt.Sprintf("msg_%d", i)
			events = append(events,
				&TextMessageStartEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtrEnhanced(currentTime.UnixMilli())},
					MessageID: msgID,
				},
				&TextMessageContentEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(currentTime.Add(10*time.Millisecond).UnixMilli())},
					MessageID: msgID,
					Delta:     fmt.Sprintf("content_%d", i),
				},
				&TextMessageEndEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd, TimestampMs: timePtrEnhanced(currentTime.Add(20*time.Millisecond).UnixMilli())},
					MessageID: msgID,
				},
			)
			currentTime = currentTime.Add(30 * time.Millisecond)
		} else {
			// Add tool sequence
			toolID := fmt.Sprintf("tool_%d", i)
			events = append(events,
				&ToolCallStartEvent{
					BaseEvent:    &BaseEvent{EventType: EventTypeToolCallStart, TimestampMs: timePtrEnhanced(currentTime.UnixMilli())},
					ToolCallID:   toolID,
					ToolCallName: fmt.Sprintf("function_%d", i),
				},
				&ToolCallArgsEvent{
					BaseEvent:  &BaseEvent{EventType: EventTypeToolCallArgs, TimestampMs: timePtrEnhanced(currentTime.Add(10*time.Millisecond).UnixMilli())},
					ToolCallID: toolID,
					Delta:      fmt.Sprintf(`{"param_%d": "value_%d"}`, i, i),
				},
				&ToolCallEndEvent{
					BaseEvent:  &BaseEvent{EventType: EventTypeToolCallEnd, TimestampMs: timePtrEnhanced(currentTime.Add(20*time.Millisecond).UnixMilli())},
					ToolCallID: toolID,
				},
			)
			currentTime = currentTime.Add(30 * time.Millisecond)
		}
	}
	
	// Finish the run
	events = append(events, &RunFinishedEvent{
		BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtrEnhanced(currentTime.UnixMilli())},
		RunID:     runID,
	})
	
	return events
}

func generateOrderedTimestampSequence(count int) []Event {
	events := make([]Event, count)
	baseTime := time.Now()
	
	for i := 0; i < count; i++ {
		events[i] = &RunStartedEvent{
			BaseEvent: &BaseEvent{
				EventType: EventTypeRunStarted,
				TimestampMs: timePtrEnhanced(baseTime.Add(time.Duration(i)*time.Millisecond).UnixMilli()),
			},
			RunID:    fmt.Sprintf("run_%d", i),
			ThreadID: fmt.Sprintf("thread_%d", i),
		}
	}
	
	return events
}

// Load Testing

func TestHighThroughputValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}
	
	fixture := setupTestFixture(t)
	defer fixture.Cleanup()
	
	fixture.optimizer.Start()
	defer fixture.optimizer.Stop()
	
	numEvents := targetThroughput // 10,000 events
	events := make([]Event, numEvents)
	
	// Pre-generate events
	for i := 0; i < numEvents; i++ {
		events[i] = &TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(time.Now().UnixMilli())},
			MessageID: fmt.Sprintf("msg_%d", i%100), // Reuse message IDs for caching
			Delta:     fmt.Sprintf("content_%d", i),
		}
	}
	
	start := time.Now()
	
	// Process events
	validCount := 0
	for _, event := range events {
		result := fixture.validator.ValidateEvent(context.Background(), event)
		if result.IsValid {
			validCount++
		}
	}
	
	duration := time.Since(start)
	throughput := float64(numEvents) / duration.Seconds()
	avgLatency := duration / time.Duration(numEvents)
	
	t.Logf("High throughput test: %d events in %v", numEvents, duration)
	t.Logf("Throughput: %.2f events/sec", throughput)
	t.Logf("Average latency: %v", avgLatency)
	t.Logf("Valid events: %d/%d", validCount, numEvents)
	
	assert.Greater(t, throughput, float64(targetThroughput)/2, "Throughput should meet minimum requirements")
	assert.Less(t, avgLatency, targetValidationLatency*10, "Average latency should be reasonable")
}

func TestMemoryUsageUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}
	
	fixture := setupTestFixture(t)
	defer fixture.Cleanup()
	
	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)
	
	// Process many events
	numEvents := 50000
	for i := 0; i < numEvents; i++ {
		event := &TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(time.Now().UnixMilli())},
			MessageID: fmt.Sprintf("msg_%d", i),
			Delta:     fmt.Sprintf("content_%d", i),
		}
		
		result := fixture.validator.ValidateEvent(context.Background(), event)
		_ = result
		
		// Force GC every 1000 events
		if i%1000 == 0 {
			runtime.GC()
		}
	}
	
	var memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memAfter)
	
	memUsedMB := float64(memAfter.Alloc-memBefore.Alloc) / 1024 / 1024
	
	t.Logf("Memory usage: %.2f MB for %d events", memUsedMB, numEvents)
	t.Logf("Memory per event: %.2f KB", memUsedMB*1024/float64(numEvents))
	
	assert.Less(t, memUsedMB, float64(maxMemoryUsageMB), "Memory usage should be within limits")
}

// Memory Leak Detection Tests

func TestMemoryLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}
	
	fixture := setupTestFixture(t)
	defer fixture.Cleanup()
	
	// Enable memory monitoring
	fixture.metrics.config.EnableLeakDetection = true
	
	var memStats []runtime.MemStats
	
	// Take baseline measurement
	var baseline runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&baseline)
	memStats = append(memStats, baseline)
	
	// Process events in batches and measure memory
	batchSize := 1000
	numBatches := 10
	
	for batch := 0; batch < numBatches; batch++ {
		// Process a batch of events
		for i := 0; i < batchSize; i++ {
			event := &TextMessageContentEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(time.Now().UnixMilli())},
				MessageID: fmt.Sprintf("msg_%d_%d", batch, i),
				Delta:     fmt.Sprintf("content_%d_%d", batch, i),
			}
			
			result := fixture.validator.ValidateEvent(context.Background(), event)
			_ = result
		}
		
		// Force GC and measure
		runtime.GC()
		var memStat runtime.MemStats
		runtime.ReadMemStats(&memStat)
		memStats = append(memStats, memStat)
		
		time.Sleep(100 * time.Millisecond) // Allow GC to settle
	}
	
	// Analyze memory growth
	var allocGrowth []uint64
	for i := 1; i < len(memStats); i++ {
		growth := memStats[i].Alloc - memStats[i-1].Alloc
		allocGrowth = append(allocGrowth, growth)
	}
	
	// Check for consistent memory growth (potential leak)
	consistentGrowth := true
	for _, growth := range allocGrowth[1:] { // Skip first measurement
		if growth == 0 {
			consistentGrowth = false
			break
		}
	}
	
	finalMemMB := float64(memStats[len(memStats)-1].Alloc) / 1024 / 1024
	baseMemMB := float64(baseline.Alloc) / 1024 / 1024
	totalGrowthMB := finalMemMB - baseMemMB
	
	t.Logf("Memory baseline: %.2f MB", baseMemMB)
	t.Logf("Memory after %d batches: %.2f MB", numBatches, finalMemMB)
	t.Logf("Total growth: %.2f MB", totalGrowthMB)
	t.Logf("Consistent growth detected: %v", consistentGrowth)
	
	// Memory should stabilize and not grow indefinitely
	assert.Less(t, totalGrowthMB, 50.0, "Total memory growth should be reasonable")
	assert.False(t, consistentGrowth, "Memory should not grow consistently (indicates leak)")
}

// Parallel Execution Tests

func TestParallelValidationStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping parallel stress test in short mode")
	}
	
	fixture := setupTestFixture(t)
	defer fixture.Cleanup()
	
	numGoroutines := runtime.NumCPU() * 10
	eventsPerGoroutine := 1000
	
	var wg sync.WaitGroup
	errorCh := make(chan error, numGoroutines)
	
	start := time.Now()
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < eventsPerGoroutine; j++ {
				event := &TextMessageContentEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(time.Now().UnixMilli())},
					MessageID: fmt.Sprintf("msg_%d_%d", goroutineID, j),
					Delta:     fmt.Sprintf("content_%d_%d", goroutineID, j),
				}
				
				result := fixture.validator.ValidateEvent(context.Background(), event)
				if result == nil {
					errorCh <- fmt.Errorf("goroutine %d: got nil result", goroutineID)
					return
				}
			}
		}(i)
	}
	
	wg.Wait()
	close(errorCh)
	
	duration := time.Since(start)
	totalEvents := numGoroutines * eventsPerGoroutine
	throughput := float64(totalEvents) / duration.Seconds()
	
	// Check for errors
	var errors []error
	for err := range errorCh {
		errors = append(errors, err)
	}
	
	t.Logf("Parallel stress test: %d goroutines, %d events each", numGoroutines, eventsPerGoroutine)
	t.Logf("Total: %d events in %v", totalEvents, duration)
	t.Logf("Throughput: %.2f events/sec", throughput)
	t.Logf("Errors: %d", len(errors))
	
	assert.Empty(t, errors, "No errors should occur during parallel execution")
	assert.Greater(t, throughput, float64(targetThroughput)/4, "Parallel throughput should be reasonable")
	
	// Check goroutine count
	finalGoroutines := runtime.NumGoroutine()
	assert.Less(t, finalGoroutines, maxGoroutines, "Should not leak goroutines")
}

// Debugging and Metrics Tests

func TestDebuggingCapabilities(t *testing.T) {
	fixture := setupTestFixture(t)
	defer fixture.Cleanup()
	
	t.Run("Debug session capture", func(t *testing.T) {
		sessionID := fixture.debugger.StartSession("test_session")
		
		events := generateValidEventSequence()
		result := fixture.validator.ValidateSequence(context.Background(), events)
		
		fixture.debugger.EndSession()
		
		session := fixture.debugger.GetSession(sessionID)
		require.NotNil(t, session)
		assert.Equal(t, "test_session", session.Name)
		assert.NotNil(t, session.EndTime)
		assert.True(t, result.IsValid)
	})
	
	t.Run("Error pattern detection", func(t *testing.T) {
		// Generate events that will cause similar errors
		for i := 0; i < 10; i++ {
			invalidEvent := &TextMessageContentEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(time.Now().UnixMilli())},
				MessageID: "", // This will cause validation errors
				Delta:     "content",
			}
			
			result := fixture.validator.ValidateEvent(context.Background(), invalidEvent)
			_ = result
		}
		
		patterns := fixture.debugger.AnalyzeErrorPatterns()
		assert.NotEmpty(t, patterns, "Should detect error patterns")
		
		if len(patterns) > 0 {
			t.Logf("Detected pattern: %s with %d occurrences", patterns[0].Pattern, patterns[0].Count)
		}
	})
	
	t.Run("Performance metrics collection", func(t *testing.T) {
		// Process some events to generate metrics
		events := generateValidEventSequence()
		start := time.Now()
		
		for _, event := range events {
			eventStart := time.Now()
			result := fixture.validator.ValidateEvent(context.Background(), event)
			duration := time.Since(eventStart)
			
			fixture.metrics.RecordEvent(duration, result.IsValid)
			
			// Record rule-level metrics
			for _, rule := range fixture.validator.GetRules() {
				fixture.metrics.RecordRuleExecution(rule.ID(), duration, result.IsValid)
			}
		}
		
		totalDuration := time.Since(start)
		fixture.metrics.RecordEvent(totalDuration, true)
		
		// Get dashboard data
		dashboard := fixture.metrics.GetDashboardData()
		require.NotNil(t, dashboard)
		
		assert.Greater(t, dashboard.TotalEvents, int64(0))
		assert.GreaterOrEqual(t, dashboard.ErrorRate, 0.0)
		assert.NotNil(t, dashboard.MemoryUsage)
		
		t.Logf("Dashboard data: %d events, %.2f%% error rate, %s health", 
			dashboard.TotalEvents, dashboard.ErrorRate, dashboard.HealthStatus)
	})
}

func TestMetricsAccuracy(t *testing.T) {
	fixture := setupTestFixture(t)
	defer fixture.Cleanup()
	
	// Known test parameters
	numValidEvents := 100
	numInvalidEvents := 20
	expectedErrorRate := float64(numInvalidEvents) / float64(numValidEvents+numInvalidEvents) * 100
	
	// Process valid events
	for i := 0; i < numValidEvents; i++ {
		event := &TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(time.Now().UnixMilli())},
			MessageID: fmt.Sprintf("msg_%d", i),
			Delta:     "valid content",
		}
		
		result := fixture.validator.ValidateEvent(context.Background(), event)
		fixture.metrics.RecordEvent(time.Millisecond, result.IsValid)
	}
	
	// Process invalid events
	for i := 0; i < numInvalidEvents; i++ {
		event := &TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(time.Now().UnixMilli())},
			MessageID: "", // Invalid empty ID
			Delta:     "content",
		}
		
		result := fixture.validator.ValidateEvent(context.Background(), event)
		fixture.metrics.RecordEvent(time.Millisecond, result.IsValid)
	}
	
	// Check metrics accuracy
	stats := fixture.metrics.GetOverallStats()
	
	totalEvents := stats["total_events"].(int64)
	errorRate := stats["error_rate"].(float64)
	
	assert.Equal(t, int64(numValidEvents+numInvalidEvents), totalEvents)
	assert.InDelta(t, expectedErrorRate, errorRate, 1.0, "Error rate should be accurate within 1%")
	
	t.Logf("Metrics accuracy test: expected %.2f%% error rate, got %.2f%%", expectedErrorRate, errorRate)
}

// Coverage and Integration Tests

func TestValidationCoverage(t *testing.T) {
	fixture := setupTestFixture(t)
	defer fixture.Cleanup()
	
	// Test all event types
	eventTypes := []EventType{
		EventTypeRunStarted,
		EventTypeRunFinished,
		EventTypeRunError,
		EventTypeTextMessageStart,
		EventTypeTextMessageContent,
		EventTypeTextMessageEnd,
		EventTypeToolCallStart,
		EventTypeToolCallArgs,
		EventTypeToolCallEnd,
		EventTypeStepStarted,
		EventTypeStepFinished,
		EventTypeStateSnapshot,
		EventTypeStateDelta,
		EventTypeMessagesSnapshot,
		EventTypeCustom,
		EventTypeRaw,
	}
	
	rulesCovered := make(map[string]bool)
	
	for _, eventType := range eventTypes {
		event := createEventOfType(eventType)
		result := fixture.validator.ValidateEvent(context.Background(), event)
		
		// Track which rules were triggered
		for _, err := range result.Errors {
			rulesCovered[err.RuleID] = true
		}
		for _, warning := range result.Warnings {
			rulesCovered[warning.RuleID] = true
		}
		
		t.Logf("Event type %s: %s (errors: %d, warnings: %d)", 
			eventType, map[bool]string{true: "valid", false: "invalid"}[result.IsValid],
			len(result.Errors), len(result.Warnings))
	}
	
	rules := fixture.validator.GetRules()
	enabledRules := 0
	for _, rule := range rules {
		if rule.IsEnabled() {
			enabledRules++
		}
	}
	
	coveragePercent := float64(len(rulesCovered)) / float64(enabledRules) * 100
	
	t.Logf("Rule coverage: %d/%d rules triggered (%.1f%%)", len(rulesCovered), enabledRules, coveragePercent)
	
	// We expect high coverage but not necessarily 100% since some rules may only trigger in specific scenarios
	assert.Greater(t, coveragePercent, 70.0, "Should achieve good rule coverage")
}

func createEventOfType(eventType EventType) Event {
	timestamp := time.Now().UnixMilli()
	
	switch eventType {
	case EventTypeRunStarted:
		return &RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			RunID:     "test-run",
			ThreadID:  "test-thread",
		}
	case EventTypeRunFinished:
		return &RunFinishedEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			RunID:     "test-run",
		}
	case EventTypeRunError:
		return &RunErrorEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			RunID:     "test-run",
			Message:   "Test error",
		}
	case EventTypeTextMessageStart:
		return &TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			MessageID: "test-msg",
		}
	case EventTypeTextMessageContent:
		return &TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			MessageID: "test-msg",
			Delta:     "Test content",
		}
	case EventTypeTextMessageEnd:
		return &TextMessageEndEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			MessageID: "test-msg",
		}
	case EventTypeToolCallStart:
		return &ToolCallStartEvent{
			BaseEvent:    &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			ToolCallID:   "test-tool",
			ToolCallName: "test_function",
		}
	case EventTypeToolCallArgs:
		return &ToolCallArgsEvent{
			BaseEvent:  &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			ToolCallID: "test-tool",
			Delta:      `{"param": "value"}`,
		}
	case EventTypeToolCallEnd:
		return &ToolCallEndEvent{
			BaseEvent:  &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			ToolCallID: "test-tool",
		}
	case EventTypeStepStarted:
		return &StepStartedEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			StepName:  "test-step",
		}
	case EventTypeStepFinished:
		return &StepFinishedEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			StepName:  "test-step",
		}
	case EventTypeStateSnapshot:
		return &StateSnapshotEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			Snapshot:  map[string]interface{}{"key": "value"},
		}
	case EventTypeStateDelta:
		return &StateDeltaEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			Delta: []JSONPatchOperation{
				{Op: "add", Path: "/test", Value: "value"},
			},
		}
	case EventTypeMessagesSnapshot:
		return &MessagesSnapshotEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			Messages: []Message{
				{Role: "user", Content: stringPtrEnhanced("test")},
			},
		}
	case EventTypeCustom:
		return &CustomEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			Name:      "test-custom",
			Value:     map[string]interface{}{"test": "value"},
		}
	case EventTypeRaw:
		return &RawEvent{
			BaseEvent: &BaseEvent{EventType: eventType, TimestampMs: &timestamp},
			Event:     map[string]interface{}{"raw": "data"},
		}
	default:
		return &RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: &timestamp},
			RunID:     "default",
			ThreadID:  "default",
		}
	}
}

// Final Integration Test

func TestCompleteValidationSystemIntegration(t *testing.T) {
	fixture := setupTestFixture(t)
	defer fixture.Cleanup()
	
	// Start all components
	fixture.optimizer.Start()
	defer fixture.optimizer.Stop()
	
	sessionID := fixture.debugger.StartSession("integration_test")
	defer fixture.debugger.EndSession()
	
	// Test scenario: Complete workflow with debugging and metrics
	t.Run("Complete workflow validation", func(t *testing.T) {
		events := generateComplexEventSequence()
		
		var results []*ValidationResult
		
		for i, event := range events {
			start := time.Now()
			result := fixture.validator.ValidateEvent(context.Background(), event)
			duration := time.Since(start)
			
			// Record metrics
			fixture.metrics.RecordEvent(duration, result.IsValid)
			for _, rule := range fixture.validator.GetRules() {
				fixture.metrics.RecordRuleExecution(rule.ID(), duration, result.IsValid)
			}
			
			results = append(results, result)
			
			// Validate performance requirements
			assert.Less(t, duration, targetValidationLatency*5, 
				"Event %d validation should be fast", i)
		}
		
		// Analyze overall results
		totalEvents := len(results)
		validEvents := 0
		totalErrors := 0
		totalWarnings := 0
		
		for _, result := range results {
			if result.IsValid {
				validEvents++
			}
			totalErrors += len(result.Errors)
			totalWarnings += len(result.Warnings)
		}
		
		validationRate := float64(validEvents) / float64(totalEvents) * 100
		
		t.Logf("Integration test results:")
		t.Logf("  Total events: %d", totalEvents)
		t.Logf("  Valid events: %d (%.1f%%)", validEvents, validationRate)
		t.Logf("  Total errors: %d", totalErrors)
		t.Logf("  Total warnings: %d", totalWarnings)
		
		// Get final metrics
		dashboard := fixture.metrics.GetDashboardData()
		t.Logf("  Final dashboard: %s health, %.2f events/sec", 
			dashboard.HealthStatus, dashboard.EventsPerSecond)
		
		// Verify session data
		session := fixture.debugger.GetSession(sessionID)
		require.NotNil(t, session)
		assert.Greater(t, len(session.Events), 0, "Session should capture events")
		
		// System should handle the complete workflow successfully
		assert.Greater(t, validationRate, 80.0, "Most events should be valid in integration test")
		assert.NotEqual(t, "Critical", dashboard.HealthStatus, "System health should not be critical")
	})
}

func generateComplexEventSequence() []Event {
	events := make([]Event, 0, 100)
	baseTime := time.Now()
	
	// Multiple runs with overlapping messages and tools
	for run := 0; run < 3; run++ {
		runID := fmt.Sprintf("run_%d", run)
		threadID := fmt.Sprintf("thread_%d", run)
		runStartTime := baseTime.Add(time.Duration(run*1000) * time.Millisecond)
		
		// Start run
		events = append(events, &RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtrEnhanced(runStartTime.UnixMilli())},
			RunID:     runID,
			ThreadID:  threadID,
		})
		
		currentTime := runStartTime.Add(10 * time.Millisecond)
		
		// Add complex message and tool sequences
		for seq := 0; seq < 5; seq++ {
			// Message sequence
			msgID := fmt.Sprintf("msg_%d_%d", run, seq)
			events = append(events,
				&TextMessageStartEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtrEnhanced(currentTime.UnixMilli())},
					MessageID: msgID,
				},
				&TextMessageContentEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtrEnhanced(currentTime.Add(5*time.Millisecond).UnixMilli())},
					MessageID: msgID,
					Delta:     fmt.Sprintf("Message content for run %d, sequence %d", run, seq),
				},
				&TextMessageEndEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd, TimestampMs: timePtrEnhanced(currentTime.Add(10*time.Millisecond).UnixMilli())},
					MessageID: msgID,
				},
			)
			currentTime = currentTime.Add(15 * time.Millisecond)
			
			// Tool sequence
			toolID := fmt.Sprintf("tool_%d_%d", run, seq)
			events = append(events,
				&ToolCallStartEvent{
					BaseEvent:    &BaseEvent{EventType: EventTypeToolCallStart, TimestampMs: timePtrEnhanced(currentTime.UnixMilli())},
					ToolCallID:   toolID,
					ToolCallName: fmt.Sprintf("function_%d", seq),
				},
				&ToolCallArgsEvent{
					BaseEvent:  &BaseEvent{EventType: EventTypeToolCallArgs, TimestampMs: timePtrEnhanced(currentTime.Add(5*time.Millisecond).UnixMilli())},
					ToolCallID: toolID,
					Delta:      fmt.Sprintf(`{"run": %d, "seq": %d, "data": "test"}`, run, seq),
				},
				&ToolCallEndEvent{
					BaseEvent:  &BaseEvent{EventType: EventTypeToolCallEnd, TimestampMs: timePtrEnhanced(currentTime.Add(10*time.Millisecond).UnixMilli())},
					ToolCallID: toolID,
				},
			)
			currentTime = currentTime.Add(15 * time.Millisecond)
			
			// Occasional state snapshots
			if seq%2 == 0 {
				events = append(events, &StateSnapshotEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeStateSnapshot, TimestampMs: timePtrEnhanced(currentTime.UnixMilli())},
					Snapshot: map[string]interface{}{
						"run_id":       runID,
						"sequence":     seq,
						"timestamp":    currentTime.Unix(),
						"active_tools": []string{toolID},
						"messages":     []string{msgID},
					},
				})
				currentTime = currentTime.Add(5 * time.Millisecond)
			}
		}
		
		// Finish run
		events = append(events, &RunFinishedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtrEnhanced(currentTime.UnixMilli())},
			RunID:     runID,
		})
	}
	
	return events
}

// Utility functions for testing

func TestUtilities(t *testing.T) {
	t.Run("Test fixtures work correctly", func(t *testing.T) {
		fixture := setupTestFixture(t)
		defer fixture.Cleanup()
		
		assert.NotNil(t, fixture.validator)
		assert.NotNil(t, fixture.debugger)
		assert.NotNil(t, fixture.metrics)
		assert.NotNil(t, fixture.optimizer)
		assert.NotEmpty(t, fixture.events)
	})
	
	t.Run("Event generation produces valid events", func(t *testing.T) {
		events := generateTestEventSequence(10)
		assert.Len(t, events, 10)
		
		// First should be RunStarted
		assert.Equal(t, EventTypeRunStarted, events[0].Type())
		
		// Last should be RunFinished
		assert.Equal(t, EventTypeRunFinished, events[len(events)-1].Type())
	})
	
	t.Run("Random event generation doesn't panic", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			assert.NotPanics(t, func() {
				event := generateRandomEvent()
				assert.NotNil(t, event)
			})
		}
	})
}