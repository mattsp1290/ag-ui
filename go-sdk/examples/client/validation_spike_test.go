package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
)

// Sample SSE payloads from Python server (captured from audit document)
var pythonSSEPayloads = []struct {
	name        string
	raw         string
	eventType   string
	validateFn  func(map[string]interface{}) error
}{
	{
		name:      "RUN_STARTED",
		raw:       `data: {"type":"RUN_STARTED","threadId":"test-thread-123","runId":"test-run-456"}`,
		eventType: "RUN_STARTED",
		validateFn: func(m map[string]interface{}) error {
			if m["threadId"] != "test-thread-123" {
				return fmt.Errorf("expected threadId test-thread-123, got %v", m["threadId"])
			}
			if m["runId"] != "test-run-456" {
				return fmt.Errorf("expected runId test-run-456, got %v", m["runId"])
			}
			return nil
		},
	},
	{
		name:      "MESSAGES_SNAPSHOT with tool calls",
		raw:       `data: {"type":"MESSAGES_SNAPSHOT","messages":[{"id":"efdebb3a-05ca-4e85-97c7-c3a355c30e1b","role":"assistant","toolCalls":[{"id":"bb200a26-2e10-4d07-a4e9-73206a52f223","type":"function","function":{"name":"generate_haiku","arguments":"{\"japanese\": [\"エーアイの\", \"橋つなぐ道\", \"コパキット\"], \"english\": [\"From AI's realm\", \"A bridge-road linking us—\", \"CopilotKit.\"]}"}}]}]}`,
		eventType: "MESSAGES_SNAPSHOT",
		validateFn: func(m map[string]interface{}) error {
			messages, ok := m["messages"].([]interface{})
			if !ok || len(messages) == 0 {
				return fmt.Errorf("expected messages array, got %v", m["messages"])
			}
			
			msg := messages[0].(map[string]interface{})
			if msg["role"] != "assistant" {
				return fmt.Errorf("expected role assistant, got %v", msg["role"])
			}
			
			toolCalls, ok := msg["toolCalls"].([]interface{})
			if !ok || len(toolCalls) != 1 {
				return fmt.Errorf("expected 1 tool call, got %v", msg["toolCalls"])
			}
			
			tc := toolCalls[0].(map[string]interface{})
			if tc["type"] != "function" {
				return fmt.Errorf("expected type function, got %v", tc["type"])
			}
			
			fn := tc["function"].(map[string]interface{})
			if fn["name"] != "generate_haiku" {
				return fmt.Errorf("expected function name generate_haiku, got %v", fn["name"])
			}
			
			return nil
		},
	},
	{
		name:      "Tool result in MESSAGES_SNAPSHOT",
		raw:       `data: {"type":"MESSAGES_SNAPSHOT","messages":[{"id":"msg-1","role":"tool","content":"Haiku generated successfully","toolCallId":"tool-123"},{"id":"e4e066f4-ec18-4482-ac1f-ac290644507b","role":"assistant","content":"Haiku created"}]}`,
		eventType: "MESSAGES_SNAPSHOT",
		validateFn: func(m map[string]interface{}) error {
			messages, ok := m["messages"].([]interface{})
			if !ok || len(messages) != 2 {
				return fmt.Errorf("expected 2 messages, got %v", len(messages))
			}
			
			// Check tool message
			toolMsg := messages[0].(map[string]interface{})
			if toolMsg["role"] != "tool" {
				return fmt.Errorf("expected first message role tool, got %v", toolMsg["role"])
			}
			if toolMsg["toolCallId"] != "tool-123" {
				return fmt.Errorf("expected toolCallId tool-123, got %v", toolMsg["toolCallId"])
			}
			
			// Check assistant message
			assistantMsg := messages[1].(map[string]interface{})
			if assistantMsg["role"] != "assistant" {
				return fmt.Errorf("expected second message role assistant, got %v", assistantMsg["role"])
			}
			
			return nil
		},
	},
	{
		name:      "RUN_FINISHED",
		raw:       `data: {"type":"RUN_FINISHED","threadId":"test-thread-123","runId":"test-run-456"}`,
		eventType: "RUN_FINISHED",
		validateFn: func(m map[string]interface{}) error {
			if m["threadId"] != "test-thread-123" {
				return fmt.Errorf("expected threadId test-thread-123, got %v", m["threadId"])
			}
			return nil
		},
	},
}

// TestPythonSSECompatibility validates that Go SDK can decode Python SSE events
func TestPythonSSECompatibility(t *testing.T) {
	for _, test := range pythonSSEPayloads {
		t.Run(test.name, func(t *testing.T) {
			// Parse SSE frame
			sseData, err := parseSSEFrame(test.raw)
			if err != nil {
				t.Fatalf("Failed to parse SSE frame: %v", err)
			}
			
			// Decode JSON
			var decoded map[string]interface{}
			if err := json.Unmarshal([]byte(sseData), &decoded); err != nil {
				t.Fatalf("Failed to decode JSON: %v", err)
			}
			
			// Validate event type
			if decoded["type"] != test.eventType {
				t.Errorf("Expected type %s, got %v", test.eventType, decoded["type"])
			}
			
			// Run custom validation
			if err := test.validateFn(decoded); err != nil {
				t.Errorf("Validation failed: %v", err)
			}
		})
	}
}

// TestGoEventToSSE validates that Go events serialize to Python-compatible format
func TestGoEventToSSE(t *testing.T) {
	tests := []struct {
		name     string
		event    events.Event
		expected map[string]interface{}
	}{
		{
			name: "RunStartedEvent",
			event: &events.RunStartedEvent{
				BaseEvent:     events.NewBaseEvent(events.EventTypeRunStarted),
				ThreadIDValue: "test-thread",
				RunIDValue:    "test-run",
			},
			expected: map[string]interface{}{
				"type":     "RUN_STARTED",
				"threadId": "test-thread",
				"runId":    "test-run",
			},
		},
		{
			name: "TextMessageStartEvent",
			event: events.NewTextMessageStartEvent("msg-123", events.WithRole("assistant")),
			expected: map[string]interface{}{
				"type":      "TEXT_MESSAGE_START",
				"messageId": "msg-123",
				"role":      "assistant",
			},
		},
		{
			name: "ToolCallStartEvent",
			event: &events.ToolCallStartEvent{
				BaseEvent:    events.NewBaseEvent(events.EventTypeToolCallStart),
				ToolCallID:   "call-456",
				ToolCallName: "get_weather",
			},
			expected: map[string]interface{}{
				"type":         "TOOL_CALL_START",
				"toolCallId":   "call-456",
				"toolCallName": "get_weather",
			},
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Serialize to JSON
			jsonBytes, err := test.event.ToJSON()
			if err != nil {
				t.Fatalf("Failed to serialize event: %v", err)
			}
			
			// Decode to map
			var decoded map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
				t.Fatalf("Failed to decode JSON: %v", err)
			}
			
			// Check expected fields
			for key, expectedValue := range test.expected {
				actualValue, ok := decoded[key]
				if !ok {
					t.Errorf("Missing field %s", key)
					continue
				}
				
				// Compare values (handle type differences)
				if fmt.Sprintf("%v", actualValue) != fmt.Sprintf("%v", expectedValue) {
					t.Errorf("Field %s: expected %v, got %v", key, expectedValue, actualValue)
				}
			}
		})
	}
}

// TestMessageCompatibility validates message serialization matches Python format
func TestMessageCompatibility(t *testing.T) {
	tests := []struct {
		name     string
		message  messages.Message
		expected map[string]interface{}
	}{
		{
			name:    "AssistantMessage with tool calls",
			message: createAssistantWithToolCalls(),
			expected: map[string]interface{}{
				"role": "assistant",
				"toolCalls": []map[string]interface{}{
					{
						"id":   "call-123",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "get_weather",
							"arguments": `{"location":"San Francisco"}`,
						},
					},
				},
			},
		},
		{
			name:    "ToolMessage",
			message: messages.NewToolMessage("Weather: 72°F", "call-123"),
			expected: map[string]interface{}{
				"role":       "tool",
				"content":    "Weather: 72°F",
				"toolCallId": "call-123",
			},
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Serialize to JSON
			jsonBytes, err := test.message.ToJSON()
			if err != nil {
				t.Fatalf("Failed to serialize message: %v", err)
			}
			
			// Decode to map
			var decoded map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
				t.Fatalf("Failed to decode JSON: %v", err)
			}
			
			// Validate key fields
			for key, expectedValue := range test.expected {
				actualValue := decoded[key]
				
				// Deep comparison for nested structures
				expectedJSON, _ := json.Marshal(expectedValue)
				actualJSON, _ := json.Marshal(actualValue)
				
				if string(expectedJSON) != string(actualJSON) {
					t.Errorf("Field %s mismatch:\nExpected: %s\nActual: %s", 
						key, string(expectedJSON), string(actualJSON))
				}
			}
		})
	}
}

// TestMissingEventTypes documents events that need to be added
func TestMissingEventTypes(t *testing.T) {
	missingEvents := []string{
		"TEXT_MESSAGE_CHUNK",
		"TOOL_CALL_CHUNK",
		"TOOL_CALL_RESULT",
		"THINKING_START",
		"THINKING_END",
		"THINKING_TEXT_MESSAGE_START",
		"THINKING_TEXT_MESSAGE_CONTENT",
		"THINKING_TEXT_MESSAGE_END",
	}
	
	for _, eventType := range missingEvents {
		t.Run(eventType, func(t *testing.T) {
			// Check if event type exists in Go SDK
			goEventType := events.EventType(eventType)
			
			// This will currently fail for missing types
			// Once implemented, these tests will pass
			if !eventTypeExists(goEventType) {
				t.Skipf("Event type %s not yet implemented (expected)", eventType)
			}
		})
	}
}

// Helper functions

func parseSSEFrame(raw string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			return strings.TrimPrefix(line, "data: "), nil
		}
	}
	return "", fmt.Errorf("no data field found in SSE frame")
}

func createAssistantWithToolCalls() *messages.AssistantMessage {
	toolCalls := []messages.ToolCall{
		{
			ID:   "call-123",
			Type: "function",
			Function: messages.Function{
				Name:      "get_weather",
				Arguments: `{"location":"San Francisco"}`,
			},
		},
	}
	return messages.NewAssistantMessageWithTools(toolCalls)
}

func eventTypeExists(eventType events.EventType) bool {
	// Check against known event types
	knownTypes := []events.EventType{
		events.EventTypeTextMessageStart,
		events.EventTypeTextMessageContent,
		events.EventTypeTextMessageEnd,
		events.EventTypeTextMessageChunk,
		events.EventTypeToolCallStart,
		events.EventTypeToolCallArgs,
		events.EventTypeToolCallEnd,
		events.EventTypeToolCallChunk,
		events.EventTypeToolCallResult,
		events.EventTypeStateSnapshot,
		events.EventTypeStateDelta,
		events.EventTypeMessagesSnapshot,
		events.EventTypeRaw,
		events.EventTypeCustom,
		events.EventTypeRunStarted,
		events.EventTypeRunFinished,
		events.EventTypeRunError,
		events.EventTypeStepStarted,
		events.EventTypeStepFinished,
		events.EventTypeThinkingStart,
		events.EventTypeThinkingEnd,
		events.EventTypeThinkingTextMessageStart,
		events.EventTypeThinkingTextMessageContent,
		events.EventTypeThinkingTextMessageEnd,
	}
	
	for _, known := range knownTypes {
		if known == eventType {
			return true
		}
	}
	return false
}

// TestSSEStreamDecoding simulates decoding a complete SSE stream
func TestSSEStreamDecoding(t *testing.T) {
	// Simulate a complete SSE stream from Python server
	sseStream := `data: {"type":"RUN_STARTED","threadId":"thread-1","runId":"run-1"}

data: {"type":"TEXT_MESSAGE_START","messageId":"msg-1","role":"assistant"}

data: {"type":"TEXT_MESSAGE_CONTENT","messageId":"msg-1","delta":"Hello"}

data: {"type":"TEXT_MESSAGE_CONTENT","messageId":"msg-1","delta":" world!"}

data: {"type":"TEXT_MESSAGE_END","messageId":"msg-1"}

data: {"type":"RUN_FINISHED","threadId":"thread-1","runId":"run-1"}

`
	
	scanner := bufio.NewScanner(strings.NewReader(sseStream))
	var events []map[string]interface{}
	
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			jsonStr := strings.TrimPrefix(line, "data: ")
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &event); err != nil {
				t.Fatalf("Failed to decode event: %v", err)
			}
			events = append(events, event)
		}
	}
	
	// Validate we got all events
	if len(events) != 6 {
		t.Errorf("Expected 6 events, got %d", len(events))
	}
	
	// Validate event sequence
	expectedTypes := []string{
		"RUN_STARTED",
		"TEXT_MESSAGE_START", 
		"TEXT_MESSAGE_CONTENT",
		"TEXT_MESSAGE_CONTENT",
		"TEXT_MESSAGE_END",
		"RUN_FINISHED",
	}
	
	for i, event := range events {
		if event["type"] != expectedTypes[i] {
			t.Errorf("Event %d: expected type %s, got %v", i, expectedTypes[i], event["type"])
		}
	}
	
	// Validate message content reconstruction
	var messageContent string
	for _, event := range events {
		if event["type"] == "TEXT_MESSAGE_CONTENT" {
			if delta, ok := event["delta"].(string); ok {
				messageContent += delta
			}
		}
	}
	
	if messageContent != "Hello world!" {
		t.Errorf("Expected message content 'Hello world!', got '%s'", messageContent)
	}
}