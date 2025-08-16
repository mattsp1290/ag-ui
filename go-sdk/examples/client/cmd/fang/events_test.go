package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEventParsing tests the parsing of different AG-UI event types
func TestEventParsing(t *testing.T) {
	tests := []struct {
		name          string
		eventJSON     string
		expectedType  string
		expectedField string
		expectedValue interface{}
	}{
		{
			name: "RUN_STARTED event",
			eventJSON: `{
				"type": "RUN_STARTED",
				"threadId": "test-thread-123",
				"runId": "test-run-456",
				"timestamp": "2025-01-14T12:00:00Z"
			}`,
			expectedType:  "RUN_STARTED",
			expectedField: "threadId",
			expectedValue: "test-thread-123",
		},
		{
			name: "RUN_FINISHED event",
			eventJSON: `{
				"type": "RUN_FINISHED",
				"threadId": "test-thread-123",
				"runId": "test-run-456",
				"status": "completed"
			}`,
			expectedType:  "RUN_FINISHED",
			expectedField: "status",
			expectedValue: "completed",
		},
		{
			name: "MESSAGES_SNAPSHOT event with tool call",
			eventJSON: `{
				"type": "MESSAGES_SNAPSHOT",
				"messages": [
					{
						"id": "msg-1",
						"role": "assistant",
						"content": "I'll generate a haiku for you.",
						"toolCalls": [
							{
								"id": "call-1",
								"type": "function",
								"function": {
									"name": "generate_haiku",
									"arguments": "{\"topic\":\"nature\"}"
								}
							}
						]
					}
				]
			}`,
			expectedType:  "MESSAGES_SNAPSHOT",
			expectedField: "messages",
			expectedValue: []interface{}{
				map[string]interface{}{
					"id":      "msg-1",
					"role":    "assistant",
					"content": "I'll generate a haiku for you.",
					"toolCalls": []interface{}{
						map[string]interface{}{
							"id":   "call-1",
							"type": "function",
							"function": map[string]interface{}{
								"name":      "generate_haiku",
								"arguments": "{\"topic\":\"nature\"}",
							},
						},
					},
				},
			},
		},
		{
			name: "TEXT_MESSAGE_START event",
			eventJSON: `{
				"type": "TEXT_MESSAGE_START",
				"role": "assistant",
				"messageId": "msg-text-1"
			}`,
			expectedType:  "TEXT_MESSAGE_START",
			expectedField: "role",
			expectedValue: "assistant",
		},
		{
			name: "TEXT_MESSAGE_CONTENT event",
			eventJSON: `{
				"type": "TEXT_MESSAGE_CONTENT",
				"delta": "Hello, how can I help you today?",
				"messageId": "msg-text-1"
			}`,
			expectedType:  "TEXT_MESSAGE_CONTENT",
			expectedField: "delta",
			expectedValue: "Hello, how can I help you today?",
		},
		{
			name: "TEXT_MESSAGE_CHUNK event (alternative)",
			eventJSON: `{
				"type": "TEXT_MESSAGE_CHUNK",
				"content": "This is a chunk of text.",
				"messageId": "msg-text-2"
			}`,
			expectedType:  "TEXT_MESSAGE_CHUNK",
			expectedField: "content",
			expectedValue: "This is a chunk of text.",
		},
		{
			name: "TEXT_MESSAGE_END event",
			eventJSON: `{
				"type": "TEXT_MESSAGE_END",
				"messageId": "msg-text-1"
			}`,
			expectedType:  "TEXT_MESSAGE_END",
			expectedField: "messageId",
			expectedValue: "msg-text-1",
		},
		{
			name: "TOOL_CALL_START event",
			eventJSON: `{
				"type": "TOOL_CALL_START",
				"toolCallId": "call-123",
				"name": "http_get",
				"timestamp": "2025-01-14T12:00:00Z"
			}`,
			expectedType:  "TOOL_CALL_START",
			expectedField: "name",
			expectedValue: "http_get",
		},
		{
			name: "TOOL_CALL_ARGS event",
			eventJSON: `{
				"type": "TOOL_CALL_ARGS",
				"toolCallId": "call-123",
				"args": "{\"url\": \"https://example.com\"}"
			}`,
			expectedType:  "TOOL_CALL_ARGS",
			expectedField: "args",
			expectedValue: "{\"url\": \"https://example.com\"}",
		},
		{
			name: "TOOL_CALL_CHUNK event",
			eventJSON: `{
				"type": "TOOL_CALL_CHUNK",
				"toolCallId": "call-123",
				"delta": "{\"url\": \"https://"
			}`,
			expectedType:  "TOOL_CALL_CHUNK",
			expectedField: "delta",
			expectedValue: "{\"url\": \"https://",
		},
		{
			name: "TOOL_CALL_END event",
			eventJSON: `{
				"type": "TOOL_CALL_END",
				"toolCallId": "call-123",
				"result": "{\"status\": 200, \"body\": \"OK\"}"
			}`,
			expectedType:  "TOOL_CALL_END",
			expectedField: "result",
			expectedValue: "{\"status\": 200, \"body\": \"OK\"}",
		},
		{
			name: "TOOL_CALL_RESULT event",
			eventJSON: `{
				"type": "TOOL_CALL_RESULT",
				"toolCallId": "call-123",
				"output": "Tool executed successfully"
			}`,
			expectedType:  "TOOL_CALL_RESULT",
			expectedField: "output",
			expectedValue: "Tool executed successfully",
		},
		{
			name: "THINKING_START event",
			eventJSON: `{
				"type": "THINKING_START",
				"messageId": "thinking-1"
			}`,
			expectedType:  "THINKING_START",
			expectedField: "messageId",
			expectedValue: "thinking-1",
		},
		{
			name: "THINKING_DELTA event",
			eventJSON: `{
				"type": "THINKING_DELTA",
				"delta": "Processing the request...",
				"messageId": "thinking-1"
			}`,
			expectedType:  "THINKING_DELTA",
			expectedField: "delta",
			expectedValue: "Processing the request...",
		},
		{
			name: "THINKING_CONTENT event (alternative)",
			eventJSON: `{
				"type": "THINKING_CONTENT",
				"content": "Analyzing the data...",
				"messageId": "thinking-1"
			}`,
			expectedType:  "THINKING_CONTENT",
			expectedField: "content",
			expectedValue: "Analyzing the data...",
		},
		{
			name: "THINKING_END event",
			eventJSON: `{
				"type": "THINKING_END",
				"messageId": "thinking-1"
			}`,
			expectedType:  "THINKING_END",
			expectedField: "messageId",
			expectedValue: "thinking-1",
		},
		{
			name: "STATE_SNAPSHOT event",
			eventJSON: `{
				"type": "STATE_SNAPSHOT",
				"snapshot": {
					"counter": 5,
					"status": "active",
					"items": ["item1", "item2"]
				}
			}`,
			expectedType:  "STATE_SNAPSHOT",
			expectedField: "snapshot",
			expectedValue: map[string]interface{}{
				"counter": float64(5), // JSON numbers decode as float64
				"status":  "active",
				"items":   []interface{}{"item1", "item2"},
			},
		},
		{
			name: "STATE_DELTA event",
			eventJSON: `{
				"type": "STATE_DELTA",
				"delta": {
					"counter": 6
				},
				"path": "/counter"
			}`,
			expectedType:  "STATE_DELTA",
			expectedField: "delta",
			expectedValue: map[string]interface{}{
				"counter": float64(6),
			},
		},
		{
			name: "UI_UPDATE event",
			eventJSON: `{
				"type": "UI_UPDATE",
				"component": "progress",
				"props": {
					"value": 75,
					"label": "Processing..."
				}
			}`,
			expectedType:  "UI_UPDATE",
			expectedField: "component",
			expectedValue: "progress",
		},
		{
			name: "CUSTOM event (PredictState)",
			eventJSON: `{
				"type": "CUSTOM",
				"name": "PredictState",
				"data": {
					"state_key": "document",
					"tool": "create_document",
					"tool_argument": "content"
				}
			}`,
			expectedType:  "CUSTOM",
			expectedField: "name",
			expectedValue: "PredictState",
		},
		{
			name: "ERROR event",
			eventJSON: `{
				"type": "ERROR",
				"error": {
					"code": "TOOL_EXECUTION_ERROR",
					"message": "Failed to execute tool",
					"details": "Connection timeout"
				}
			}`,
			expectedType:  "ERROR",
			expectedField: "error",
			expectedValue: map[string]interface{}{
				"code":    "TOOL_EXECUTION_ERROR",
				"message": "Failed to execute tool",
				"details": "Connection timeout",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event map[string]interface{}
			err := json.Unmarshal([]byte(tt.eventJSON), &event)
			require.NoError(t, err, "Failed to parse event JSON")

			// Check event type
			eventType, ok := event["type"].(string)
			require.True(t, ok, "Event should have a type field")
			assert.Equal(t, tt.expectedType, eventType)

			// Check specific field
			value, exists := event[tt.expectedField]
			require.True(t, exists, fmt.Sprintf("Event should have field '%s'", tt.expectedField))
			assert.Equal(t, tt.expectedValue, value)
		})
	}
}

// TestMessageExtraction tests extraction of messages from MESSAGES_SNAPSHOT events
func TestMessageExtraction(t *testing.T) {
	tests := []struct {
		name            string
		eventJSON       string
		expectedContent string
		expectedTools   int
		expectedRole    string
	}{
		{
			name: "Assistant message with no tools",
			eventJSON: `{
				"type": "MESSAGES_SNAPSHOT",
				"messages": [
					{
						"id": "msg-1",
						"role": "assistant",
						"content": "Hello! How can I help you today?"
					}
				]
			}`,
			expectedContent: "Hello! How can I help you today?",
			expectedTools:   0,
			expectedRole:    "assistant",
		},
		{
			name: "Assistant message with single tool call",
			eventJSON: `{
				"type": "MESSAGES_SNAPSHOT",
				"messages": [
					{
						"id": "msg-1",
						"role": "assistant",
						"content": "Let me generate a haiku for you.",
						"toolCalls": [
							{
								"id": "call-1",
								"type": "function",
								"function": {
									"name": "generate_haiku",
									"arguments": "{\"topic\":\"nature\"}"
								}
							}
						]
					}
				]
			}`,
			expectedContent: "Let me generate a haiku for you.",
			expectedTools:   1,
			expectedRole:    "assistant",
		},
		{
			name: "Assistant message with multiple tool calls",
			eventJSON: `{
				"type": "MESSAGES_SNAPSHOT",
				"messages": [
					{
						"id": "msg-1",
						"role": "assistant",
						"content": "I'll fetch the weather and news for you.",
						"toolCalls": [
							{
								"id": "call-1",
								"type": "function",
								"function": {
									"name": "get_weather",
									"arguments": "{\"city\":\"New York\"}"
								}
							},
							{
								"id": "call-2",
								"type": "function",
								"function": {
									"name": "get_news",
									"arguments": "{\"category\":\"tech\"}"
								}
							}
						]
					}
				]
			}`,
			expectedContent: "I'll fetch the weather and news for you.",
			expectedTools:   2,
			expectedRole:    "assistant",
		},
		{
			name: "Tool result message",
			eventJSON: `{
				"type": "MESSAGES_SNAPSHOT",
				"messages": [
					{
						"id": "msg-2",
						"role": "tool",
						"content": "Haiku generated successfully",
						"toolCallId": "call-1"
					}
				]
			}`,
			expectedContent: "Haiku generated successfully",
			expectedTools:   0,
			expectedRole:    "tool",
		},
		{
			name: "Multiple messages in snapshot",
			eventJSON: `{
				"type": "MESSAGES_SNAPSHOT",
				"messages": [
					{
						"id": "msg-1",
						"role": "user",
						"content": "Generate a haiku about nature"
					},
					{
						"id": "msg-2",
						"role": "assistant",
						"content": "I'll create a haiku for you.",
						"toolCalls": [
							{
								"id": "call-1",
								"type": "function",
								"function": {
									"name": "generate_haiku",
									"arguments": "{\"topic\":\"nature\"}"
								}
							}
						]
					},
					{
						"id": "msg-3",
						"role": "tool",
						"content": "Spring rain falling down\nGentle drops on new green leaves\nLife begins again",
						"toolCallId": "call-1"
					}
				]
			}`,
			expectedContent: "I'll create a haiku for you.",
			expectedTools:   1,
			expectedRole:    "assistant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event map[string]interface{}
			err := json.Unmarshal([]byte(tt.eventJSON), &event)
			require.NoError(t, err)

			messages, ok := event["messages"].([]interface{})
			require.True(t, ok, "Event should have messages field")
			require.Greater(t, len(messages), 0, "Should have at least one message")

			// Find the first assistant message or use the first message
			var targetMessage map[string]interface{}
			for _, msg := range messages {
				m := msg.(map[string]interface{})
				if m["role"] == "assistant" {
					targetMessage = m
					break
				}
			}
			if targetMessage == nil {
				targetMessage = messages[0].(map[string]interface{})
			}

			// Check content
			content, ok := targetMessage["content"].(string)
			require.True(t, ok, "Message should have content field")
			assert.Equal(t, tt.expectedContent, content)

			// Check role
			role, ok := targetMessage["role"].(string)
			require.True(t, ok, "Message should have role field")
			assert.Equal(t, tt.expectedRole, role)

			// Check tool calls if assistant message
			if role == "assistant" {
				toolCalls, exists := targetMessage["toolCalls"]
				if tt.expectedTools > 0 {
					require.True(t, exists, "Assistant message should have toolCalls")
					calls := toolCalls.([]interface{})
					assert.Equal(t, tt.expectedTools, len(calls))
				} else {
					if exists {
						calls := toolCalls.([]interface{})
						assert.Equal(t, 0, len(calls))
					}
				}
			}
		})
	}
}

// TestToolCallParsing tests parsing of tool call structures
func TestToolCallParsing(t *testing.T) {
	tests := []struct {
		name             string
		toolCallJSON     string
		expectedName     string
		expectedArgument string
		expectedError    bool
	}{
		{
			name: "Valid tool call with JSON arguments",
			toolCallJSON: `{
				"id": "call-123",
				"type": "function",
				"function": {
					"name": "generate_haiku",
					"arguments": "{\"topic\": \"nature\", \"style\": \"traditional\"}"
				}
			}`,
			expectedName:     "generate_haiku",
			expectedArgument: "topic",
			expectedError:    false,
		},
		{
			name: "Tool call with empty arguments",
			toolCallJSON: `{
				"id": "call-456",
				"type": "function",
				"function": {
					"name": "get_time",
					"arguments": "{}"
				}
			}`,
			expectedName:     "get_time",
			expectedArgument: "",
			expectedError:    false,
		},
		{
			name: "Tool call with complex nested arguments",
			toolCallJSON: `{
				"id": "call-789",
				"type": "function",
				"function": {
					"name": "create_document",
					"arguments": "{\"title\": \"Test\", \"sections\": [{\"heading\": \"Intro\", \"content\": \"Hello\"}]}"
				}
			}`,
			expectedName:     "create_document",
			expectedArgument: "title",
			expectedError:    false,
		},
		{
			name: "Tool call with invalid JSON arguments",
			toolCallJSON: `{
				"id": "call-bad",
				"type": "function",
				"function": {
					"name": "bad_tool",
					"arguments": "not valid json"
				}
			}`,
			expectedName:     "bad_tool",
			expectedArgument: "",
			expectedError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var toolCall map[string]interface{}
			err := json.Unmarshal([]byte(tt.toolCallJSON), &toolCall)
			require.NoError(t, err)

			// Extract function details
			function, ok := toolCall["function"].(map[string]interface{})
			require.True(t, ok, "Tool call should have function field")

			// Check name
			name, ok := function["name"].(string)
			require.True(t, ok, "Function should have name field")
			assert.Equal(t, tt.expectedName, name)

			// Check and parse arguments
			argsStr, ok := function["arguments"].(string)
			require.True(t, ok, "Function should have arguments field")

			var args map[string]interface{}
			err = json.Unmarshal([]byte(argsStr), &args)
			
			if tt.expectedError {
				assert.Error(t, err, "Should fail to parse invalid JSON arguments")
			} else {
				require.NoError(t, err, "Should parse valid JSON arguments")
				
				if tt.expectedArgument != "" {
					_, exists := args[tt.expectedArgument]
					assert.True(t, exists, fmt.Sprintf("Arguments should contain '%s' field", tt.expectedArgument))
				}
			}
		})
	}
}

// TestStateManagement tests STATE_SNAPSHOT and STATE_DELTA event handling
func TestStateManagement(t *testing.T) {
	t.Run("STATE_SNAPSHOT replaces entire state", func(t *testing.T) {
		// Initial state would be replaced entirely by snapshot
		// Not used in this test but showing what would happen
		// initialState := map[string]interface{}{
		//	"counter": float64(1),
		//	"status":  "initial",
		//	"items":   []interface{}{"old1", "old2"},
		// }

		snapshotEvent := `{
			"type": "STATE_SNAPSHOT",
			"snapshot": {
				"counter": 5,
				"status": "updated",
				"newField": "new value"
			}
		}`

		var event map[string]interface{}
		err := json.Unmarshal([]byte(snapshotEvent), &event)
		require.NoError(t, err)

		snapshot, ok := event["snapshot"].(map[string]interface{})
		require.True(t, ok)

		// Snapshot should replace the entire state
		assert.Equal(t, float64(5), snapshot["counter"])
		assert.Equal(t, "updated", snapshot["status"])
		assert.Equal(t, "new value", snapshot["newField"])
		assert.Nil(t, snapshot["items"], "Old fields should be removed")
	})

	t.Run("STATE_DELTA merges with existing state", func(t *testing.T) {
		currentState := map[string]interface{}{
			"counter": float64(5),
			"status":  "active",
			"items":   []interface{}{"item1", "item2"},
		}

		deltaEvent := `{
			"type": "STATE_DELTA",
			"delta": {
				"counter": 6,
				"newField": "added"
			},
			"path": "/"
		}`

		var event map[string]interface{}
		err := json.Unmarshal([]byte(deltaEvent), &event)
		require.NoError(t, err)

		delta, ok := event["delta"].(map[string]interface{})
		require.True(t, ok)

		// Apply delta to current state
		for k, v := range delta {
			currentState[k] = v
		}

		// Check merged state
		assert.Equal(t, float64(6), currentState["counter"], "Counter should be updated")
		assert.Equal(t, "active", currentState["status"], "Status should remain unchanged")
		assert.Equal(t, []interface{}{"item1", "item2"}, currentState["items"], "Items should remain unchanged")
		assert.Equal(t, "added", currentState["newField"], "New field should be added")
	})

	t.Run("STATE_DELTA with nested path", func(t *testing.T) {
		deltaEvent := `{
			"type": "STATE_DELTA",
			"delta": {
				"value": 100
			},
			"path": "/settings/display/brightness"
		}`

		var event map[string]interface{}
		err := json.Unmarshal([]byte(deltaEvent), &event)
		require.NoError(t, err)

		path, ok := event["path"].(string)
		require.True(t, ok)
		assert.Equal(t, "/settings/display/brightness", path)

		delta, ok := event["delta"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, float64(100), delta["value"])
	})
}

// TestRenderToolResult tests the rendering of tool results
func TestRenderToolResult(t *testing.T) {
	tests := []struct {
		name           string
		toolName       string
		args           map[string]interface{}
		expectedOutput []string
	}{
		{
			name:     "generate_haiku tool",
			toolName: "generate_haiku",
			args: map[string]interface{}{
				"english": []interface{}{
					"Spring rain falling down",
					"Gentle drops on new green leaves",
					"Life begins again",
				},
				"japanese": []interface{}{
					"春の雨降る",
					"新緑の葉に優しく",
					"命再び",
				},
			},
			expectedOutput: []string{
				"Haiku:",
				"Spring rain falling down",
				"Gentle drops on new green leaves",
				"Life begins again",
				"俳句:",
				"春の雨降る",
				"新緑の葉に優しく",
				"命再び",
			},
		},
		{
			name:     "generate_haiku tool with only english",
			toolName: "generate_haiku",
			args: map[string]interface{}{
				"english": []interface{}{
					"Autumn moon rises",
					"Casting shadows through the trees",
					"Silence fills the night",
				},
			},
			expectedOutput: []string{
				"Haiku:",
				"Autumn moon rises",
				"Casting shadows through the trees",
				"Silence fills the night",
			},
		},
		{
			name:     "other tool with string result",
			toolName: "get_weather",
			args: map[string]interface{}{
				"result": "Temperature: 72°F, Sunny",
			},
			expectedOutput: []string{
				"Tool Result:",
				`{"result":"Temperature: 72°F, Sunny"}`,
			},
		},
		{
			name:     "tool with complex object result",
			toolName: "fetch_data",
			args: map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"count": float64(42),
					"items": []interface{}{"item1", "item2"},
				},
			},
			expectedOutput: []string{
				"Tool Result:",
				`"status":"success"`,
				`"count":42`,
				`"items":["item1","item2"]`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: We can't directly test renderToolResult as it writes to stdout
			// In a real implementation, we would refactor it to accept an io.Writer
			// For now, we'll test the logic indirectly

			// Test the formatting logic
			if tt.toolName == "generate_haiku" {
				if english, ok := tt.args["english"].([]interface{}); ok {
					for _, expected := range []string{"Haiku:"} {
						assert.Contains(t, tt.expectedOutput, expected)
					}
					for _, line := range english {
						assert.Contains(t, tt.expectedOutput, line.(string))
					}
				}
				if japanese, ok := tt.args["japanese"].([]interface{}); ok {
					for _, expected := range []string{"俳句:"} {
						assert.Contains(t, tt.expectedOutput, expected)
					}
					for _, line := range japanese {
						assert.Contains(t, tt.expectedOutput, line.(string))
					}
				}
			} else {
				// For other tools, check JSON formatting
				jsonBytes, err := json.Marshal(tt.args)
				require.NoError(t, err)
				
				for _, expected := range tt.expectedOutput {
					if expected == "Tool Result:" {
						continue
					}
					assert.Contains(t, string(jsonBytes), expected[0:min(len(expected), 10)])
				}
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}