package events

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToolCallResultEvent(t *testing.T) {
	t.Run("basic creation", func(t *testing.T) {
		messageID := "msg-123"
		toolCallID := "tool-456"
		content := "Tool execution successful"

		event := NewToolCallResultEvent(messageID, toolCallID, content)

		if event.Type() != EventTypeToolCallResult {
			t.Errorf("expected event type %s, got %s", EventTypeToolCallResult, event.Type())
		}

		if event.MessageID != messageID {
			t.Errorf("expected messageID %s, got %s", messageID, event.MessageID)
		}

		if event.ToolCallID != toolCallID {
			t.Errorf("expected toolCallID %s, got %s", toolCallID, event.ToolCallID)
		}

		if event.Content != content {
			t.Errorf("expected content %s, got %s", content, event.Content)
		}

		if event.Role == nil || *event.Role != "tool" {
			t.Errorf("expected role 'tool', got %v", event.Role)
		}

		if err := event.Validate(); err != nil {
			t.Errorf("validation failed: %v", err)
		}
	})

	t.Run("validation requirements", func(t *testing.T) {
		tests := []struct {
			name       string
			messageID  string
			toolCallID string
			content    string
			shouldFail bool
		}{
			{"all fields valid", "msg-1", "tool-1", "content", false},
			{"empty messageID", "", "tool-1", "content", true},
			{"empty toolCallID", "msg-1", "", "content", true},
			{"empty content", "msg-1", "tool-1", "", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				event := &ToolCallResultEvent{
					BaseEvent:  NewBaseEvent(EventTypeToolCallResult),
					MessageID:  tt.messageID,
					ToolCallID: tt.toolCallID,
					Content:    tt.content,
				}

				err := event.Validate()
				if tt.shouldFail && err == nil {
					t.Error("expected validation to fail but it passed")
				}
				if !tt.shouldFail && err != nil {
					t.Errorf("expected validation to pass but it failed: %v", err)
				}
			})
		}
	})

	t.Run("JSON serialization with camelCase", func(t *testing.T) {
		event := NewToolCallResultEvent("msg-123", "tool-456", "Result content")

		jsonData, err := event.ToJSON()
		if err != nil {
			t.Fatalf("failed to serialize to JSON: %v", err)
		}

		jsonStr := string(jsonData)

		// Check for camelCase field names
		if !strings.Contains(jsonStr, `"messageId"`) {
			t.Error("JSON should contain 'messageId' field in camelCase")
		}

		if !strings.Contains(jsonStr, `"toolCallId"`) {
			t.Error("JSON should contain 'toolCallId' field in camelCase")
		}

		// Should not contain snake_case
		if strings.Contains(jsonStr, `"message_id"`) {
			t.Error("JSON should not contain snake_case 'message_id'")
		}

		if strings.Contains(jsonStr, `"tool_call_id"`) {
			t.Error("JSON should not contain snake_case 'tool_call_id'")
		}

		// Verify the deserialized data
		var decoded map[string]interface{}
		if err := json.Unmarshal(jsonData, &decoded); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if decoded["type"] != string(EventTypeToolCallResult) {
			t.Errorf("expected type %s in JSON, got %v", EventTypeToolCallResult, decoded["type"])
		}

		if decoded["messageId"] != "msg-123" {
			t.Errorf("expected messageId 'msg-123' in JSON, got %v", decoded["messageId"])
		}

		if decoded["toolCallId"] != "tool-456" {
			t.Errorf("expected toolCallId 'tool-456' in JSON, got %v", decoded["toolCallId"])
		}

		if decoded["role"] != "tool" {
			t.Errorf("expected role 'tool' in JSON, got %v", decoded["role"])
		}
	})
}

func TestToolCallChunkEvent(t *testing.T) {
	t.Run("basic creation", func(t *testing.T) {
		event := NewToolCallChunkEvent()

		if event.Type() != EventTypeToolCallChunk {
			t.Errorf("expected event type %s, got %s", EventTypeToolCallChunk, event.Type())
		}
	})

	t.Run("with all fields", func(t *testing.T) {
		toolCallID := "tool-123"
		toolCallName := "get_weather"
		delta := "{\"temperature\":"

		event := NewToolCallChunkEvent().
			WithToolCallChunkID(toolCallID).
			WithToolCallChunkName(toolCallName).
			WithToolCallChunkDelta(delta)

		if event.ToolCallID == nil || *event.ToolCallID != toolCallID {
			t.Errorf("expected toolCallID %s, got %v", toolCallID, event.ToolCallID)
		}

		if event.ToolCallName == nil || *event.ToolCallName != toolCallName {
			t.Errorf("expected toolCallName %s, got %v", toolCallName, event.ToolCallName)
		}

		if event.Delta == nil || *event.Delta != delta {
			t.Errorf("expected delta %s, got %v", delta, event.Delta)
		}

		if err := event.Validate(); err != nil {
			t.Errorf("validation failed: %v", err)
		}
	})

	t.Run("validation requires at least one field", func(t *testing.T) {
		event := &ToolCallChunkEvent{
			BaseEvent: NewBaseEvent(EventTypeToolCallChunk),
		}

		if err := event.Validate(); err == nil {
			t.Error("expected validation to fail for empty chunk")
		}
	})

	t.Run("JSON serialization with optional fields", func(t *testing.T) {
		toolCallID := "tool-789"
		event := NewToolCallChunkEvent().WithToolCallChunkID(toolCallID)

		jsonData, err := event.ToJSON()
		if err != nil {
			t.Fatalf("failed to serialize to JSON: %v", err)
		}

		jsonStr := string(jsonData)

		// Check for camelCase field names
		if !strings.Contains(jsonStr, `"toolCallId"`) {
			t.Error("JSON should contain 'toolCallId' field in camelCase")
		}

		// Should not contain unset optional fields
		if strings.Contains(jsonStr, `"toolCallName"`) {
			t.Error("JSON should not contain unset 'toolCallName' field")
		}

		if strings.Contains(jsonStr, `"delta"`) {
			t.Error("JSON should not contain unset 'delta' field")
		}
	})
}

func TestTextMessageChunkEvent(t *testing.T) {
	t.Run("basic creation", func(t *testing.T) {
		event := NewTextMessageChunkEvent()

		if event.Type() != EventTypeTextMessageChunk {
			t.Errorf("expected event type %s, got %s", EventTypeTextMessageChunk, event.Type())
		}
	})

	t.Run("with all fields", func(t *testing.T) {
		messageID := "msg-123"
		role := "assistant"
		delta := "Hello, how can I help"

		event := NewTextMessageChunkEvent().
			WithChunkMessageID(messageID).
			WithChunkRole(role).
			WithChunkDelta(delta)

		if event.MessageID == nil || *event.MessageID != messageID {
			t.Errorf("expected messageID %s, got %v", messageID, event.MessageID)
		}

		if event.Role == nil || *event.Role != role {
			t.Errorf("expected role %s, got %v", role, event.Role)
		}

		if event.Delta == nil || *event.Delta != delta {
			t.Errorf("expected delta %s, got %v", delta, event.Delta)
		}

		if err := event.Validate(); err != nil {
			t.Errorf("validation failed: %v", err)
		}
	})

	t.Run("validation requires at least one field", func(t *testing.T) {
		event := &TextMessageChunkEvent{
			BaseEvent: NewBaseEvent(EventTypeTextMessageChunk),
		}

		if err := event.Validate(); err == nil {
			t.Error("expected validation to fail for empty chunk")
		}
	})

	t.Run("JSON serialization with camelCase", func(t *testing.T) {
		messageID := "msg-456"
		role := "user"
		delta := "What's the weather?"

		event := NewTextMessageChunkEvent().
			WithChunkMessageID(messageID).
			WithChunkRole(role).
			WithChunkDelta(delta)

		jsonData, err := event.ToJSON()
		if err != nil {
			t.Fatalf("failed to serialize to JSON: %v", err)
		}

		jsonStr := string(jsonData)

		// Check for camelCase field names
		if !strings.Contains(jsonStr, `"messageId"`) {
			t.Error("JSON should contain 'messageId' field in camelCase")
		}

		// Should not contain snake_case
		if strings.Contains(jsonStr, `"message_id"`) {
			t.Error("JSON should not contain snake_case 'message_id'")
		}

		// Verify the deserialized data
		var decoded map[string]interface{}
		if err := json.Unmarshal(jsonData, &decoded); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if decoded["type"] != string(EventTypeTextMessageChunk) {
			t.Errorf("expected type %s in JSON, got %v", EventTypeTextMessageChunk, decoded["type"])
		}

		if decoded["messageId"] != messageID {
			t.Errorf("expected messageId %s in JSON, got %v", messageID, decoded["messageId"])
		}

		if decoded["role"] != role {
			t.Errorf("expected role %s in JSON, got %v", role, decoded["role"])
		}

		if decoded["delta"] != delta {
			t.Errorf("expected delta %s in JSON, got %v", delta, decoded["delta"])
		}
	})

	t.Run("JSON omits empty optional fields", func(t *testing.T) {
		delta := "Just the delta"
		event := NewTextMessageChunkEvent().WithChunkDelta(delta)

		jsonData, err := event.ToJSON()
		if err != nil {
			t.Fatalf("failed to serialize to JSON: %v", err)
		}

		jsonStr := string(jsonData)

		// Should contain delta
		if !strings.Contains(jsonStr, `"delta"`) {
			t.Error("JSON should contain 'delta' field")
		}

		// Should not contain unset optional fields
		if strings.Contains(jsonStr, `"messageId"`) {
			t.Error("JSON should not contain unset 'messageId' field")
		}

		if strings.Contains(jsonStr, `"role"`) {
			t.Error("JSON should not contain unset 'role' field")
		}
	})
}

// TestPythonCompatibility tests compatibility with Python server SSE format
func TestPythonCompatibility(t *testing.T) {
	// Sample SSE payloads from Python server based on the analysis document
	pythonSSEExamples := []struct {
		name     string
		jsonStr  string
		expected map[string]interface{}
	}{
		{
			name:    "TOOL_CALL_RESULT event",
			jsonStr: `{"type":"TOOL_CALL_RESULT","messageId":"msg-1","toolCallId":"tool-123","content":"Result","role":"tool"}`,
			expected: map[string]interface{}{
				"type":       "TOOL_CALL_RESULT",
				"messageId":  "msg-1",
				"toolCallId": "tool-123",
				"content":    "Result",
				"role":       "tool",
			},
		},
		{
			name:    "TEXT_MESSAGE_CHUNK event",
			jsonStr: `{"type":"TEXT_MESSAGE_CHUNK","messageId":"msg-2","role":"assistant","delta":"Hello"}`,
			expected: map[string]interface{}{
				"type":      "TEXT_MESSAGE_CHUNK",
				"messageId": "msg-2",
				"role":      "assistant",
				"delta":     "Hello",
			},
		},
		{
			name:    "TOOL_CALL_CHUNK event",
			jsonStr: `{"type":"TOOL_CALL_CHUNK","toolCallId":"tool-456","delta":"{\"temp\":"}`,
			expected: map[string]interface{}{
				"type":       "TOOL_CALL_CHUNK",
				"toolCallId": "tool-456",
				"delta":      "{\"temp\":",
			},
		},
		{
			name:    "THINKING_START event",
			jsonStr: `{"type":"THINKING_START","title":"Analyzing request"}`,
			expected: map[string]interface{}{
				"type":  "THINKING_START",
				"title": "Analyzing request",
			},
		},
	}

	for _, tt := range pythonSSEExamples {
		t.Run(tt.name, func(t *testing.T) {
			// Verify we can decode the Python JSON format
			var decoded map[string]interface{}
			if err := json.Unmarshal([]byte(tt.jsonStr), &decoded); err != nil {
				t.Fatalf("failed to unmarshal Python JSON: %v", err)
			}

			// Check all expected fields match
			for key, expectedValue := range tt.expected {
				if decodedValue, ok := decoded[key]; !ok {
					t.Errorf("missing expected field %s", key)
				} else if decodedValue != expectedValue {
					t.Errorf("field %s: expected %v, got %v", key, expectedValue, decodedValue)
				}
			}

			// Ensure we're using camelCase (not snake_case)
			for key := range decoded {
				if strings.Contains(key, "_") && key != "type" {
					t.Errorf("found snake_case field %s, expected camelCase", key)
				}
			}
		})
	}
}
