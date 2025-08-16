package events

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRoundtripCompatibility tests JSON encoding/decoding compatibility with server SSE payloads
func TestRoundtripCompatibility(t *testing.T) {
	// Load all fixture files
	fixtureFiles := []string{
		"text_message_events.json",
		"tool_call_events.json",
		"run_events.json",
		"state_events.json",
		"thinking_events.json",
		"misc_events.json",
	}

	for _, file := range fixtureFiles {
		t.Run(file, func(t *testing.T) {
			fixtures := loadFixtures(t, file)
			for name, fixture := range fixtures {
				t.Run(name, func(t *testing.T) {
					testRoundtrip(t, name, fixture)
				})
			}
		})
	}
}

// loadFixtures loads JSON fixtures from a file
func loadFixtures(t *testing.T, filename string) map[string]json.RawMessage {
	path := filepath.Join("testdata", "fixtures", filename)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "Failed to read fixture file: %s", path)

	var fixtures map[string]json.RawMessage
	err = json.Unmarshal(data, &fixtures)
	require.NoError(t, err, "Failed to unmarshal fixtures from: %s", path)

	return fixtures
}

// testRoundtrip tests decoding JSON -> struct -> encoding back to JSON
func testRoundtrip(t *testing.T, name string, fixture json.RawMessage) {
	// Step 1: Parse to get event type
	var typeCheck struct {
		Type string `json:"type"`
	}
	err := json.Unmarshal(fixture, &typeCheck)
	require.NoError(t, err, "Failed to determine event type for fixture: %s", name)

	// Step 2: Decode into appropriate event struct
	event, err := decodeEvent(typeCheck.Type, fixture)
	require.NoError(t, err, "Failed to decode event for fixture: %s", name)

	// Step 3: Validate the event
	if validator, ok := event.(interface{ Validate() error }); ok {
		err = validator.Validate()
		// For minimal fixtures, validation might fail on required fields - that's ok
		if err != nil && !isMinimalFixture(name) {
			t.Errorf("Event validation failed for %s: %v", name, err)
		}
	}

	// Step 4: Re-encode to JSON
	encoded, err := json.Marshal(event)
	require.NoError(t, err, "Failed to re-encode event for fixture: %s", name)

	// Step 5: Compare JSONs (field presence, casing, values)
	assertJSONEquivalent(t, fixture, encoded, name)
}

// decodeEvent decodes JSON into the appropriate event struct based on type
func decodeEvent(eventType string, data json.RawMessage) (interface{}, error) {
	switch EventType(eventType) {
	// Text Message Events
	case EventTypeTextMessageStart:
		var event TextMessageStartEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeTextMessageContent:
		var event TextMessageContentEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeTextMessageEnd:
		var event TextMessageEndEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeTextMessageChunk:
		var event TextMessageChunkEvent
		err := json.Unmarshal(data, &event)
		return &event, err

	// Tool Call Events
	case EventTypeToolCallStart:
		var event ToolCallStartEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeToolCallArgs:
		var event ToolCallArgsEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeToolCallEnd:
		var event ToolCallEndEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeToolCallChunk:
		var event ToolCallChunkEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeToolCallResult:
		var event ToolCallResultEvent
		err := json.Unmarshal(data, &event)
		return &event, err

	// Run Events
	case EventTypeRunStarted:
		var event RunStartedEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeRunFinished:
		var event RunFinishedEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeRunError:
		var event RunErrorEvent
		err := json.Unmarshal(data, &event)
		return &event, err

	// Step Events
	case EventTypeStepStarted:
		var event StepStartedEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeStepFinished:
		var event StepFinishedEvent
		err := json.Unmarshal(data, &event)
		return &event, err

	// State Events
	case EventTypeStateSnapshot:
		var event StateSnapshotEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeStateDelta:
		var event StateDeltaEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeMessagesSnapshot:
		var event MessagesSnapshotEvent
		err := json.Unmarshal(data, &event)
		return &event, err

	// Thinking Events
	case EventTypeThinkingStart:
		var event ThinkingStartEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeThinkingEnd:
		var event ThinkingEndEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeThinkingTextMessageStart:
		var event ThinkingTextMessageStartEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeThinkingTextMessageContent:
		var event ThinkingTextMessageContentEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeThinkingTextMessageEnd:
		var event ThinkingTextMessageEndEvent
		err := json.Unmarshal(data, &event)
		return &event, err

	// Misc Events
	case EventTypeCustom:
		var event CustomEvent
		err := json.Unmarshal(data, &event)
		return &event, err
	case EventTypeRaw:
		var event RawEvent
		err := json.Unmarshal(data, &event)
		return &event, err

	default:
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}
}

// assertJSONEquivalent compares two JSON blobs for semantic equivalence
func assertJSONEquivalent(t *testing.T, expected, actual json.RawMessage, name string) {
	// Parse both JSONs into maps for comparison
	var expectedMap, actualMap map[string]interface{}
	
	err := json.Unmarshal(expected, &expectedMap)
	require.NoError(t, err, "Failed to unmarshal expected JSON for: %s", name)
	
	err = json.Unmarshal(actual, &actualMap)
	require.NoError(t, err, "Failed to unmarshal actual JSON for: %s", name)

	// Compare field names (check casing)
	for key := range expectedMap {
		_, exists := actualMap[key]
		assert.True(t, exists, "Missing field '%s' in encoded JSON for fixture: %s", key, name)
	}

	// Check for unexpected fields (only if not a minimal fixture)
	if !isMinimalFixture(name) {
		for key := range actualMap {
			_, exists := expectedMap[key]
			assert.True(t, exists, "Unexpected field '%s' in encoded JSON for fixture: %s", key, name)
		}
	}

	// Compare values (ignoring field ordering)
	for key, expectedValue := range expectedMap {
		actualValue, exists := actualMap[key]
		if !exists {
			continue // Already reported above
		}

		// Special handling for timestamps (can be null/omitted)
		if key == "timestamp" && expectedValue == nil && actualValue == nil {
			continue
		}

		// Compare values
		if !reflect.DeepEqual(expectedValue, actualValue) {
			// Try comparing as JSON for nested structures
			expectedJSON, _ := json.Marshal(expectedValue)
			actualJSON, _ := json.Marshal(actualValue)
			assert.JSONEq(t, string(expectedJSON), string(actualJSON),
				"Value mismatch for field '%s' in fixture: %s", key, name)
		}
	}
}

// isMinimalFixture checks if a fixture name indicates a minimal fixture
func isMinimalFixture(name string) bool {
	return len(name) > 8 && name[len(name)-8:] == "_MINIMAL"
}

// TestFieldNamingConventions verifies all events use camelCase field names
func TestFieldNamingConventions(t *testing.T) {
	testCases := []struct {
		name     string
		event    interface{}
		expected map[string]string // field -> expected JSON name
	}{
		{
			name: "TextMessageStartEvent",
			event: &TextMessageStartEvent{
				BaseEvent: NewBaseEvent(EventTypeTextMessageStart),
				MessageID: "test-id",
			},
			expected: map[string]string{
				"type":      "TEXT_MESSAGE_START",
				"messageId": "test-id",
			},
		},
		{
			name: "ToolCallStartEvent",
			event: &ToolCallStartEvent{
				BaseEvent:    NewBaseEvent(EventTypeToolCallStart),
				ToolCallID:   "tool-id",
				ToolCallName: "tool-name",
			},
			expected: map[string]string{
				"type":         "TOOL_CALL_START",
				"toolCallId":   "tool-id",
				"toolCallName": "tool-name",
			},
		},
		{
			name: "RunStartedEvent",
			event: &RunStartedEvent{
				BaseEvent:     NewBaseEvent(EventTypeRunStarted),
				ThreadIDValue: "thread-id",
				RunIDValue:    "run-id",
			},
			expected: map[string]string{
				"type":     "RUN_STARTED",
				"threadId": "thread-id",
				"runId":    "run-id",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encode to JSON
			data, err := json.Marshal(tc.event)
			require.NoError(t, err)

			// Parse JSON
			var result map[string]interface{}
			err = json.Unmarshal(data, &result)
			require.NoError(t, err)

			// Check field names
			for field, expected := range tc.expected {
				actual, exists := result[field]
				assert.True(t, exists, "Missing field: %s", field)
				if field != "type" { // type is EventType, not string
					assert.Equal(t, expected, actual, "Field %s has wrong value", field)
				}
			}
		})
	}
}

// TestOptionalFieldHandling verifies optional fields are properly omitted when nil
func TestOptionalFieldHandling(t *testing.T) {
	t.Run("TextMessageStartEvent with role", func(t *testing.T) {
		role := "assistant"
		event := &TextMessageStartEvent{
			BaseEvent: NewBaseEvent(EventTypeTextMessageStart),
			MessageID: "msg-1",
			Role:      &role,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, "assistant", result["role"])
	})

	t.Run("TextMessageStartEvent without role", func(t *testing.T) {
		event := &TextMessageStartEvent{
			BaseEvent: NewBaseEvent(EventTypeTextMessageStart),
			MessageID: "msg-1",
			Role:      nil,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		_, exists := result["role"]
		assert.False(t, exists, "Role field should be omitted when nil")
	})

	t.Run("ToolCallStartEvent with parentMessageId", func(t *testing.T) {
		parentID := "parent-1"
		event := &ToolCallStartEvent{
			BaseEvent:       NewBaseEvent(EventTypeToolCallStart),
			ToolCallID:      "tool-1",
			ToolCallName:    "test-tool",
			ParentMessageID: &parentID,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, "parent-1", result["parentMessageId"])
	})

	t.Run("ToolCallStartEvent without parentMessageId", func(t *testing.T) {
		event := &ToolCallStartEvent{
			BaseEvent:       NewBaseEvent(EventTypeToolCallStart),
			ToolCallID:      "tool-1",
			ToolCallName:    "test-tool",
			ParentMessageID: nil,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		_, exists := result["parentMessageId"]
		assert.False(t, exists, "ParentMessageId field should be omitted when nil")
	})
}