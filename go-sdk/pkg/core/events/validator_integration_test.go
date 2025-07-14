package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestIntegration_CompleteUserAssistantConversation tests a complete user-assistant conversation flow
func TestIntegration_CompleteUserAssistantConversation(t *testing.T) {
	validator := NewEventValidator(ProductionValidationConfig())
	ctx := context.Background()

	// Simulate a complete conversation flow
	events := []Event{
		// 1. Start the run
		&RunStartedEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeRunStarted,
				TimestampMs: timePtr(time.Now().UnixMilli()),
			},
			RunID:    "run-conv-001",
			ThreadID: "thread-conv-001",
		},

		// 2. User message
		&TextMessageStartEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeTextMessageStart,
				TimestampMs: timePtr(time.Now().UnixMilli() + 100),
			},
			MessageID: "msg-user-001",
			Role:      stringPtr("user"),
		},
		&TextMessageContentEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeTextMessageContent,
				TimestampMs: timePtr(time.Now().UnixMilli() + 200),
			},
			MessageID: "msg-user-001",
			Delta:     "What's the weather like in San Francisco?",
		},
		&TextMessageEndEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeTextMessageEnd,
				TimestampMs: timePtr(time.Now().UnixMilli() + 300),
			},
			MessageID: "msg-user-001",
		},

		// 3. Assistant thinking and responding
		&TextMessageStartEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeTextMessageStart,
				TimestampMs: timePtr(time.Now().UnixMilli() + 400),
			},
			MessageID: "msg-assistant-001",
			Role:      stringPtr("assistant"),
		},
		&TextMessageContentEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeTextMessageContent,
				TimestampMs: timePtr(time.Now().UnixMilli() + 500),
			},
			MessageID: "msg-assistant-001",
			Delta:     "I'll check the weather in San Francisco for you.",
		},
		&TextMessageEndEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeTextMessageEnd,
				TimestampMs: timePtr(time.Now().UnixMilli() + 600),
			},
			MessageID: "msg-assistant-001",
		},

		// 4. Assistant uses a tool
		&ToolCallStartEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeToolCallStart,
				TimestampMs: timePtr(time.Now().UnixMilli() + 700),
			},
			ToolCallID:      "tool-weather-001",
			ToolCallName:    "get_weather",
			ParentMessageID: stringPtr("msg-assistant-001"),
		},
		&ToolCallArgsEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeToolCallArgs,
				TimestampMs: timePtr(time.Now().UnixMilli() + 800),
			},
			ToolCallID: "tool-weather-001",
			Delta:      `{"location": "San Francisco", "units": "fahrenheit"}`,
		},
		&ToolCallEndEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeToolCallEnd,
				TimestampMs: timePtr(time.Now().UnixMilli() + 900),
			},
			ToolCallID: "tool-weather-001",
		},

		// 5. Assistant provides final response
		&TextMessageStartEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeTextMessageStart,
				TimestampMs: timePtr(time.Now().UnixMilli() + 1000),
			},
			MessageID: "msg-assistant-002",
			Role:      stringPtr("assistant"),
		},
		&TextMessageContentEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeTextMessageContent,
				TimestampMs: timePtr(time.Now().UnixMilli() + 1100),
			},
			MessageID: "msg-assistant-002",
			Delta:     "Based on the current weather data, San Francisco is experiencing:\n",
		},
		&TextMessageContentEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeTextMessageContent,
				TimestampMs: timePtr(time.Now().UnixMilli() + 1200),
			},
			MessageID: "msg-assistant-002",
			Delta:     "- Temperature: 68°F (20°C)\n- Conditions: Partly cloudy\n- Humidity: 65%",
		},
		&TextMessageEndEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeTextMessageEnd,
				TimestampMs: timePtr(time.Now().UnixMilli() + 1300),
			},
			MessageID: "msg-assistant-002",
		},

		// 6. Finish the run
		&RunFinishedEvent{
			BaseEvent: &BaseEvent{
				EventType:   EventTypeRunFinished,
				TimestampMs: timePtr(time.Now().UnixMilli() + 1400),
			},
			RunID: "run-conv-001",
		},
	}

	// Validate the complete sequence
	result := validator.ValidateSequence(ctx, events)

	if !result.IsValid {
		t.Errorf("Complete conversation should be valid, got %d errors", len(result.Errors))
		for _, err := range result.Errors {
			t.Logf("Error: [%s] %s", err.RuleID, err.Message)
		}
	}

	// Note: ValidateSequence uses an isolated validator, so state tracking
	// is not visible in the main validator. This is by design to ensure
	// thread safety. For state tracking tests, we would need to use
	// ValidateEvent on individual events, but that would require proper
	// sequencing (can't validate message events without run started first)
}

// TestIntegration_StreamingToolCallWithMultipleArgs tests streaming tool calls with chunked arguments
func TestIntegration_StreamingToolCallWithMultipleArgs(t *testing.T) {
	validator := NewEventValidator(ProductionValidationConfig())
	ctx := context.Background()

	// Simulate a tool call with streaming JSON arguments
	events := []Event{
		&RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
			RunID:     "run-stream-001",
			ThreadID:  "thread-stream-001",
		},

		// Assistant message with tool call
		&TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtr(time.Now().UnixMilli() + 100)},
			MessageID: "msg-stream-001",
			Role:      stringPtr("assistant"),
		},
		&TextMessageEndEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd, TimestampMs: timePtr(time.Now().UnixMilli() + 200)},
			MessageID: "msg-stream-001",
		},

		// Tool call with streaming arguments
		&ToolCallStartEvent{
			BaseEvent:       &BaseEvent{EventType: EventTypeToolCallStart, TimestampMs: timePtr(time.Now().UnixMilli() + 300)},
			ToolCallID:      "tool-stream-001",
			ToolCallName:    "complex_calculation",
			ParentMessageID: stringPtr("msg-stream-001"),
		},

		// Streaming JSON arguments in chunks
		&ToolCallArgsEvent{
			BaseEvent:  &BaseEvent{EventType: EventTypeToolCallArgs, TimestampMs: timePtr(time.Now().UnixMilli() + 400)},
			ToolCallID: "tool-stream-001",
			Delta:      `{"operation": "matrix_multiply",`,
		},
		&ToolCallArgsEvent{
			BaseEvent:  &BaseEvent{EventType: EventTypeToolCallArgs, TimestampMs: timePtr(time.Now().UnixMilli() + 500)},
			ToolCallID: "tool-stream-001",
			Delta:      ` "matrix_a": [[1, 2], [3, 4]],`,
		},
		&ToolCallArgsEvent{
			BaseEvent:  &BaseEvent{EventType: EventTypeToolCallArgs, TimestampMs: timePtr(time.Now().UnixMilli() + 600)},
			ToolCallID: "tool-stream-001",
			Delta:      ` "matrix_b": [[5, 6], [7, 8]]}`,
		},

		&ToolCallEndEvent{
			BaseEvent:  &BaseEvent{EventType: EventTypeToolCallEnd, TimestampMs: timePtr(time.Now().UnixMilli() + 700)},
			ToolCallID: "tool-stream-001",
		},

		&RunFinishedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtr(time.Now().UnixMilli() + 800)},
			RunID:     "run-stream-001",
		},
	}

	result := validator.ValidateSequence(ctx, events)

	if !result.IsValid {
		t.Errorf("Streaming tool call should be valid, got %d errors", len(result.Errors))
		for _, err := range result.Errors {
			t.Logf("Error: [%s] %s", err.RuleID, err.Message)
		}
	}
}

// TestIntegration_ErrorRecoveryScenario tests error handling and recovery
func TestIntegration_ErrorRecoveryScenario(t *testing.T) {
	validator := NewEventValidator(ProductionValidationConfig())
	ctx := context.Background()

	// Simulate a run that encounters an error
	events := []Event{
		&RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
			RunID:     "run-error-001",
			ThreadID:  "thread-error-001",
		},

		&TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtr(time.Now().UnixMilli() + 100)},
			MessageID: "msg-error-001",
			Role:      stringPtr("user"),
		},
		&TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtr(time.Now().UnixMilli() + 200)},
			MessageID: "msg-error-001",
			Delta:     "Process this invalid request",
		},
		&TextMessageEndEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd, TimestampMs: timePtr(time.Now().UnixMilli() + 300)},
			MessageID: "msg-error-001",
		},

		// Run encounters an error
		&RunErrorEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunError, TimestampMs: timePtr(time.Now().UnixMilli() + 400)},
			RunID:     "run-error-001",
			Message:   "Invalid request: missing required parameters",
		},
	}

	result := validator.ValidateSequence(ctx, events)

	if !result.IsValid {
		t.Errorf("Error recovery scenario should be valid, got %d errors", len(result.Errors))
		for _, err := range result.Errors {
			t.Logf("Error: [%s] %s", err.RuleID, err.Message)
		}
	}
}

// TestIntegration_StateManagementFlow tests state snapshot and delta events
func TestIntegration_StateManagementFlow(t *testing.T) {
	validator := NewEventValidator(ProductionValidationConfig())
	ctx := context.Background()

	// Create initial state
	initialState := map[string]interface{}{
		"counter":   0,
		"messages":  []string{},
		"isRunning": true,
		"config": map[string]interface{}{
			"model":       "claude-3",
			"temperature": 0.7,
		},
	}

	events := []Event{
		&RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
			RunID:     "run-state-001",
			ThreadID:  "thread-state-001",
		},

		// Initial state snapshot
		&StateSnapshotEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeStateSnapshot, TimestampMs: timePtr(time.Now().UnixMilli() + 100)},
			Snapshot:  initialState,
		},

		// State updates via delta
		&StateDeltaEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeStateDelta, TimestampMs: timePtr(time.Now().UnixMilli() + 200)},
			Delta: []JSONPatchOperation{
				{Op: "replace", Path: "/counter", Value: json.RawMessage("1")},
				{Op: "add", Path: "/messages/-", Value: json.RawMessage(`"First message"`)},
			},
		},

		// More state updates
		&StateDeltaEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeStateDelta, TimestampMs: timePtr(time.Now().UnixMilli() + 300)},
			Delta: []JSONPatchOperation{
				{Op: "replace", Path: "/counter", Value: json.RawMessage("2")},
				{Op: "add", Path: "/messages/-", Value: json.RawMessage(`"Second message"`)},
				{Op: "replace", Path: "/config/temperature", Value: json.RawMessage("0.9")},
			},
		},

		// Messages snapshot
		&MessagesSnapshotEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeMessagesSnapshot, TimestampMs: timePtr(time.Now().UnixMilli() + 400)},
			Messages: []Message{
				{
					ID:      "msg-1",
					Role:    "user",
					Content: stringPtr("Hello"),
				},
				{
					ID:      "msg-2",
					Role:    "assistant",
					Content: stringPtr("Hi! How can I help you today?"),
				},
			},
		},

		&RunFinishedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtr(time.Now().UnixMilli() + 500)},
			RunID:     "run-state-001",
		},
	}

	result := validator.ValidateSequence(ctx, events)

	if !result.IsValid {
		t.Errorf("State management flow should be valid, got %d errors", len(result.Errors))
		for _, err := range result.Errors {
			t.Logf("Error: [%s] %s", err.RuleID, err.Message)
		}
	}
}

// TestIntegration_ConcurrentMessagesAndTools tests complex concurrent operations
func TestIntegration_ConcurrentMessagesAndTools(t *testing.T) {
	validator := NewEventValidator(ProductionValidationConfig())
	ctx := context.Background()

	// Simulate multiple concurrent messages and tool calls
	events := []Event{
		&RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
			RunID:     "run-concurrent-001",
			ThreadID:  "thread-concurrent-001",
		},

		// Start multiple messages
		&TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtr(time.Now().UnixMilli() + 100)},
			MessageID: "msg-1",
			Role:      stringPtr("assistant"),
		},
		&TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtr(time.Now().UnixMilli() + 110)},
			MessageID: "msg-2",
			Role:      stringPtr("assistant"),
		},

		// Content for first message
		&TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtr(time.Now().UnixMilli() + 200)},
			MessageID: "msg-1",
			Delta:     "Processing your request...",
		},

		// Start tool calls while messages are active
		&ToolCallStartEvent{
			BaseEvent:       &BaseEvent{EventType: EventTypeToolCallStart, TimestampMs: timePtr(time.Now().UnixMilli() + 300)},
			ToolCallID:      "tool-1",
			ToolCallName:    "search",
			ParentMessageID: stringPtr("msg-1"),
		},
		&ToolCallStartEvent{
			BaseEvent:       &BaseEvent{EventType: EventTypeToolCallStart, TimestampMs: timePtr(time.Now().UnixMilli() + 310)},
			ToolCallID:      "tool-2",
			ToolCallName:    "calculate",
			ParentMessageID: stringPtr("msg-2"),
		},

		// Interleaved operations
		&TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtr(time.Now().UnixMilli() + 400)},
			MessageID: "msg-2",
			Delta:     "Calculating results...",
		},
		&ToolCallArgsEvent{
			BaseEvent:  &BaseEvent{EventType: EventTypeToolCallArgs, TimestampMs: timePtr(time.Now().UnixMilli() + 500)},
			ToolCallID: "tool-1",
			Delta:      `{"query": "latest news"}`,
		},
		&TextMessageEndEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd, TimestampMs: timePtr(time.Now().UnixMilli() + 600)},
			MessageID: "msg-1",
		},
		&ToolCallEndEvent{
			BaseEvent:  &BaseEvent{EventType: EventTypeToolCallEnd, TimestampMs: timePtr(time.Now().UnixMilli() + 700)},
			ToolCallID: "tool-1",
		},
		&ToolCallArgsEvent{
			BaseEvent:  &BaseEvent{EventType: EventTypeToolCallArgs, TimestampMs: timePtr(time.Now().UnixMilli() + 800)},
			ToolCallID: "tool-2",
			Delta:      `{"expression": "2 + 2"}`,
		},
		&ToolCallEndEvent{
			BaseEvent:  &BaseEvent{EventType: EventTypeToolCallEnd, TimestampMs: timePtr(time.Now().UnixMilli() + 900)},
			ToolCallID: "tool-2",
		},
		&TextMessageEndEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd, TimestampMs: timePtr(time.Now().UnixMilli() + 1000)},
			MessageID: "msg-2",
		},

		&RunFinishedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtr(time.Now().UnixMilli() + 1100)},
			RunID:     "run-concurrent-001",
		},
	}

	result := validator.ValidateSequence(ctx, events)

	if !result.IsValid {
		t.Errorf("Concurrent operations should be valid, got %d errors", len(result.Errors))
		for _, err := range result.Errors {
			t.Logf("Error: [%s] %s", err.RuleID, err.Message)
		}
	}
}

// TestIntegration_LongRunningApplicationSimulation simulates a long-running app with memory management
func TestIntegration_LongRunningApplicationSimulation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running simulation in short mode")
	}

	// Create validator with cleanup
	validator := NewEventValidator(ProductionValidationConfig())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start cleanup routine with aggressive settings for testing
	validator.StartCleanupRoutine(ctx, 100*time.Millisecond, 200*time.Millisecond)

	// Simulate multiple runs over time
	for i := 0; i < 10; i++ {
		runID := fmt.Sprintf("run-%03d", i)
		threadID := fmt.Sprintf("thread-%03d", i)

		events := []Event{
			&RunStartedEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
				RunID:     runID,
				ThreadID:  threadID,
			},
			&TextMessageStartEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtr(time.Now().UnixMilli() + 10)},
				MessageID: fmt.Sprintf("msg-%03d", i),
				Role:      stringPtr("user"),
			},
			&TextMessageContentEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtr(time.Now().UnixMilli() + 20)},
				MessageID: fmt.Sprintf("msg-%03d", i),
				Delta:     fmt.Sprintf("Request %d", i),
			},
			&TextMessageEndEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd, TimestampMs: timePtr(time.Now().UnixMilli() + 30)},
				MessageID: fmt.Sprintf("msg-%03d", i),
			},
			&RunFinishedEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtr(time.Now().UnixMilli() + 40)},
				RunID:     runID,
			},
		}

		result := validator.ValidateSequence(ctx, events)
		if !result.IsValid {
			t.Errorf("Run %d should be valid", i)
		}

		// Simulate time passing between runs
		if i > 5 {
			time.Sleep(250 * time.Millisecond) // Allow cleanup to trigger
		}
	}

	// Wait for cleanup to run
	time.Sleep(500 * time.Millisecond)

	// Check memory stats
	stats := validator.GetState().GetMemoryStats()
	t.Logf("Final memory stats: %+v", stats)

	// Verify cleanup is working
	if stats["finished_runs"] > 5 {
		t.Errorf("Expected cleanup to limit finished runs, got %d", stats["finished_runs"])
	}
}

// TestIntegration_InvalidSequences tests various invalid event sequences
func TestIntegration_InvalidSequences(t *testing.T) {
	testCases := []struct {
		name   string
		events []Event
		errors []string // Expected error rule IDs
	}{
		{
			name: "No RUN_STARTED first",
			events: []Event{
				&TextMessageStartEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart},
					MessageID: "msg-1",
					Role:      stringPtr("user"),
				},
			},
			errors: []string{"EVENT_ORDERING"},
		},
		{
			name: "Events after RUN_FINISHED",
			events: []Event{
				&RunStartedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
					RunID:     "run-1",
					ThreadID:  "thread-1",
				},
				&RunFinishedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunFinished},
					RunID:     "run-1",
				},
				&TextMessageStartEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart},
					MessageID: "msg-1",
					Role:      stringPtr("user"),
				},
			},
			errors: []string{"EVENT_ORDERING"},
		},
		{
			name: "Orphaned message content",
			events: []Event{
				&RunStartedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
					RunID:     "run-1",
					ThreadID:  "thread-1",
				},
				&TextMessageContentEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent},
					MessageID: "msg-1",
					Delta:     "Orphaned content",
				},
			},
			errors: []string{"MESSAGE_LIFECYCLE"},
		},
		{
			name: "Tool args without start",
			events: []Event{
				&RunStartedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
					RunID:     "run-1",
					ThreadID:  "thread-1",
				},
				&ToolCallArgsEvent{
					BaseEvent:  &BaseEvent{EventType: EventTypeToolCallArgs},
					ToolCallID: "tool-1",
					Delta:      `{"query": "test"}`,
				},
				&RunFinishedEvent{
					BaseEvent: &BaseEvent{EventType: EventTypeRunFinished},
					RunID:     "run-1",
				},
			},
			errors: []string{"TOOL_CALL_LIFECYCLE"},
		},
	}

	validator := NewEventValidator(ProductionValidationConfig())
	ctx := context.Background()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := validator.ValidateSequence(ctx, tc.events)

			if result.IsValid {
				t.Error("Expected validation to fail")
			}

			// Check expected errors
			foundErrors := make(map[string]bool)
			for _, err := range result.Errors {
				foundErrors[err.RuleID] = true
			}

			for _, expectedError := range tc.errors {
				found := false
				for ruleID := range foundErrors {
					if strings.Contains(ruleID, expectedError) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error with rule ID containing %s, but not found", expectedError)
					t.Logf("Found errors: %v", foundErrors)
				}
			}
		})
	}
}

// TestIntegration_CustomEventsFlow tests custom event handling
func TestIntegration_CustomEventsFlow(t *testing.T) {
	validator := NewEventValidator(ProductionValidationConfig())
	ctx := context.Background()

	// Create custom event data
	customData := map[string]interface{}{
		"action":    "model_switch",
		"from":      "claude-3-opus",
		"to":        "claude-3-sonnet",
		"reason":    "optimize_cost",
		"timestamp": time.Now().Unix(),
	}

	events := []Event{
		&RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
			RunID:     "run-custom-001",
			ThreadID:  "thread-custom-001",
		},

		// Custom event for model switching
		&CustomEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeCustom, TimestampMs: timePtr(time.Now().UnixMilli() + 100)},
			Name:      "model_switch",
			Value:     customData,
		},

		// Continue with normal flow
		&TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtr(time.Now().UnixMilli() + 200)},
			MessageID: "msg-custom-001",
			Role:      stringPtr("assistant"),
		},
		&TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtr(time.Now().UnixMilli() + 300)},
			MessageID: "msg-custom-001",
			Delta:     "Switched to Sonnet model for this response.",
		},
		&TextMessageEndEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd, TimestampMs: timePtr(time.Now().UnixMilli() + 400)},
			MessageID: "msg-custom-001",
		},

		// Raw event (passthrough)
		&RawEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRaw, TimestampMs: timePtr(time.Now().UnixMilli() + 500)},
			Event: map[string]interface{}{
				"type":      "internal_metric",
				"metric":    "token_count",
				"value":     1234,
				"timestamp": time.Now().Unix(),
			},
		},

		&RunFinishedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtr(time.Now().UnixMilli() + 600)},
			RunID:     "run-custom-001",
		},
	}

	result := validator.ValidateSequence(ctx, events)

	if !result.IsValid {
		t.Errorf("Custom events flow should be valid, got %d errors", len(result.Errors))
		for _, err := range result.Errors {
			t.Logf("Error: [%s] %s", err.RuleID, err.Message)
		}
	}
}

// TestIntegration_JSONSerializationRoundTrip tests JSON serialization in real scenarios
func TestIntegration_JSONSerializationRoundTrip(t *testing.T) {
	validator := NewEventValidator(ProductionValidationConfig())
	ctx := context.Background()

	// Create events
	originalEvents := []Event{
		&RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
			RunID:     "run-json-001",
			ThreadID:  "thread-json-001",
		},
		&StateSnapshotEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeStateSnapshot, TimestampMs: timePtr(time.Now().UnixMilli() + 100)},
			Snapshot: map[string]interface{}{
				"nested": map[string]interface{}{
					"array": []interface{}{1, 2, 3},
					"bool":  true,
					"null":  nil,
				},
			},
		},
		&RunFinishedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtr(time.Now().UnixMilli() + 200)},
			RunID:     "run-json-001",
		},
	}

	// Serialize to JSON
	var jsonEvents []string
	for _, event := range originalEvents {
		jsonData, err := event.ToJSON()
		if err != nil {
			t.Fatalf("Failed to serialize event: %v", err)
		}
		jsonEvents = append(jsonEvents, string(jsonData))
	}

	// Deserialize back
	var deserializedEvents []Event
	for _, jsonData := range jsonEvents {
		event, err := EventFromJSON([]byte(jsonData))
		if err != nil {
			t.Fatalf("Failed to deserialize event: %v", err)
		}
		deserializedEvents = append(deserializedEvents, event)
	}

	// Validate deserialized events
	result := validator.ValidateSequence(ctx, deserializedEvents)

	if !result.IsValid {
		t.Errorf("Deserialized events should be valid, got %d errors", len(result.Errors))
		for _, err := range result.Errors {
			t.Logf("Error: [%s] %s", err.RuleID, err.Message)
		}
	}
}

// Benchmark integration test
func BenchmarkIntegration_CompleteConversationFlow(b *testing.B) {
	validator := NewEventValidator(ProductionValidationConfig())
	ctx := context.Background()

	// Pre-create events to avoid allocation in benchmark loop
	events := []Event{
		&RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
			RunID:     "run-bench-001",
			ThreadID:  "thread-bench-001",
		},
		&TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtr(time.Now().UnixMilli() + 100)},
			MessageID: "msg-bench-001",
			Role:      stringPtr("user"),
		},
		&TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtr(time.Now().UnixMilli() + 200)},
			MessageID: "msg-bench-001",
			Delta:     "Hello, can you help me?",
		},
		&TextMessageEndEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd, TimestampMs: timePtr(time.Now().UnixMilli() + 300)},
			MessageID: "msg-bench-001",
		},
		&TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtr(time.Now().UnixMilli() + 400)},
			MessageID: "msg-bench-002",
			Role:      stringPtr("assistant"),
		},
		&TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtr(time.Now().UnixMilli() + 500)},
			MessageID: "msg-bench-002",
			Delta:     "Of course! I'd be happy to help you.",
		},
		&TextMessageEndEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd, TimestampMs: timePtr(time.Now().UnixMilli() + 600)},
			MessageID: "msg-bench-002",
		},
		&RunFinishedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtr(time.Now().UnixMilli() + 700)},
			RunID:     "run-bench-001",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := validator.ValidateSequence(ctx, events)
		if !result.IsValid {
			b.Fatal("Validation failed")
		}
	}
}
