package validation

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// TestVectorSet represents a collection of test vectors
type TestVectorSet struct {
	Version     string       `json:"version"`
	Format      string       `json:"format"`
	Description string       `json:"description"`
	Vectors     []TestVector `json:"vectors"`
}

// StandardTestVectors contains all standard test vectors for event types
var StandardTestVectors = map[string]TestVectorSet{
	"run_events": {
		Version:     "1.0.0",
		Format:      "application/json",
		Description: "Standard test vectors for run events",
		Vectors: []TestVector{
			{
				Name:        "run_started_basic",
				Description: "Basic RunStarted event",
				Format:      "application/json",
				SDK:         "go",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"RUN_STARTED","timestamp":1640995200,"runId":"run-12345","threadId":"thread-67890"}`),
				Expected: &events.RunStartedEvent{
					BaseEvent: &events.BaseEvent{
						EventType:   events.EventTypeRunStarted,
						TimestampMs: int64Ptr(1640995200),
					},
					RunID:    "run-12345",
					ThreadID: "thread-67890",
				},
			},
			{
				Name:        "run_started_with_sequence",
				Description: "RunStarted event with sequence number",
				Format:      "application/json",
				SDK:         "go",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"RUN_STARTED","timestamp":1640995200,"runId":"run-12345","threadId":"thread-67890"}`),
				Expected: &events.RunStartedEvent{
					BaseEvent: &events.BaseEvent{
						EventType:   events.EventTypeRunStarted,
						TimestampMs: int64Ptr(1640995200),
					},
					RunID:    "run-12345",
					ThreadID: "thread-67890",
				},
			},
			{
				Name:        "run_finished_basic",
				Description: "Basic RunFinished event",
				Format:      "application/json",
				SDK:         "go",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"RUN_FINISHED","timestamp":1640995300,"runId":"run-12345","threadId":"thread-67890"}`),
				Expected: &events.RunFinishedEvent{
					BaseEvent: &events.BaseEvent{
						EventType: events.EventTypeRunFinished,
						TimestampMs: int64Ptr(1640995300),
					},
					RunID: "run-12345",
					ThreadID: "thread-67890",
				},
			},
		},
	},
	"message_events": {
		Version:     "1.0.0",
		Format:      "application/json",
		Description: "Standard test vectors for message events",
		Vectors: []TestVector{
			{
				Name:        "text_message_start",
				Description: "Basic TextMessageStart event",
				Format:      "application/json",
				SDK:         "go",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"TEXT_MESSAGE_START","timestamp":1640995200,"messageId":"msg-abc123"}`),
				Expected: &events.TextMessageStartEvent{
					BaseEvent: &events.BaseEvent{
						EventType: events.EventTypeTextMessageStart,
						TimestampMs: int64Ptr(1640995200),
					},
					MessageID: "msg-abc123",
				},
			},
			{
				Name:        "text_message_content",
				Description: "Basic TextMessageContent event",
				Format:      "application/json",
				SDK:         "go",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"TEXT_MESSAGE_CONTENT","timestamp":1640995200,"messageId":"msg-abc123","delta":"Hello, world!"}`),
				Expected: &events.TextMessageContentEvent{
					BaseEvent: &events.BaseEvent{
						EventType: events.EventTypeTextMessageContent,
						TimestampMs: int64Ptr(1640995200),
					},
					MessageID: "msg-abc123",
					Delta:     "Hello, world!",
				},
			},
			{
				Name:        "text_message_content_unicode",
				Description: "TextMessageContent with Unicode characters",
				Format:      "application/json",
				SDK:         "go",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"TEXT_MESSAGE_CONTENT","timestamp":1640995200,"messageId":"msg-abc123","delta":"Hello 🌍! こんにちは世界"}`),
				Expected: &events.TextMessageContentEvent{
					BaseEvent: &events.BaseEvent{
						EventType: events.EventTypeTextMessageContent,
						TimestampMs: int64Ptr(1640995200),
					},
					MessageID: "msg-abc123",
					Delta:     "Hello 🌍! こんにちは世界",
				},
			},
			{
				Name:        "text_message_end",
				Description: "Basic TextMessageEnd event",
				Format:      "application/json",
				SDK:         "go",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"TEXT_MESSAGE_END","timestamp":1640995300,"messageId":"msg-abc123"}`),
				Expected: &events.TextMessageEndEvent{
					BaseEvent: &events.BaseEvent{
						EventType: events.EventTypeTextMessageEnd,
						TimestampMs: int64Ptr(1640995300),
					},
					MessageID: "msg-abc123",
				},
			},
		},
	},
	"tool_events": {
		Version:     "1.0.0",
		Format:      "application/json",
		Description: "Standard test vectors for tool events",
		Vectors: []TestVector{
			{
				Name:        "tool_call_start",
				Description: "Basic ToolCallStart event",
				Format:      "application/json",
				SDK:         "go",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"TOOL_CALL_START","timestamp":1640995200,"toolCallId":"tool-xyz789","toolCallName":"calculator"}`),
				Expected: &events.ToolCallStartEvent{
					BaseEvent: &events.BaseEvent{
						EventType: events.EventTypeToolCallStart,
						TimestampMs: int64Ptr(1640995200),
					},
					ToolCallID:   "tool-xyz789",
					ToolCallName: "calculator",
				},
			},
			{
				Name:        "tool_call_args",
				Description: "Basic ToolCallArgs event",
				Format:      "application/json",
				SDK:         "go",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"TOOL_CALL_ARGS","timestamp":1640995200,"toolCallId":"tool-xyz789","delta":"{\"operation\":\"add\",\"a\":5,\"b\":3}"}`),
				Expected: &events.ToolCallArgsEvent{
					BaseEvent: &events.BaseEvent{
						EventType: events.EventTypeToolCallArgs,
						TimestampMs: int64Ptr(1640995200),
					},
					ToolCallID: "tool-xyz789",
					Delta:      `{"operation":"add","a":5,"b":3}`,
				},
			},
			{
				Name:        "tool_call_end",
				Description: "Basic ToolCallEnd event",
				Format:      "application/json",
				SDK:         "go",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"TOOL_CALL_END","timestamp":1640995300,"toolCallId":"tool-xyz789"}`),
				Expected: &events.ToolCallEndEvent{
					BaseEvent: &events.BaseEvent{
						EventType: events.EventTypeToolCallEnd,
						TimestampMs: int64Ptr(1640995300),
					},
					ToolCallID: "tool-xyz789",
				},
			},
		},
	},
	"state_events": {
		Version:     "1.0.0",
		Format:      "application/json",
		Description: "Standard test vectors for state events",
		Vectors: []TestVector{
			{
				Name:        "state_snapshot_simple",
				Description: "Simple StateSnapshot event",
				Format:      "application/json",
				SDK:         "go",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"STATE_SNAPSHOT","timestamp":1640995200,"snapshot":{"counter":42,"status":"active"}}`),
				Expected: &events.StateSnapshotEvent{
					BaseEvent: &events.BaseEvent{
						EventType: events.EventTypeStateSnapshot,
						TimestampMs: int64Ptr(1640995200),
					},
					Snapshot: map[string]interface{}{
						"counter": float64(42),
						"status":  "active",
					},
				},
			},
			{
				Name:        "state_snapshot_complex",
				Description: "Complex StateSnapshot event with nested structures",
				Format:      "application/json",
				SDK:         "go",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"STATE_SNAPSHOT","timestamp":1640995200,"snapshot":{"user":{"id":123,"name":"John","preferences":{"theme":"dark","lang":"en"}},"sessions":[{"id":"sess1","active":true},{"id":"sess2","active":false}]}}`),
				Expected: &events.StateSnapshotEvent{
					BaseEvent: &events.BaseEvent{
						EventType: events.EventTypeStateSnapshot,
						TimestampMs: int64Ptr(1640995200),
					},
					Snapshot: map[string]interface{}{
						"user": map[string]interface{}{
							"id":   float64(123),
							"name": "John",
							"preferences": map[string]interface{}{
								"theme": "dark",
								"lang":  "en",
							},
						},
						"sessions": []interface{}{
							map[string]interface{}{
								"id":     "sess1",
								"active": true,
							},
							map[string]interface{}{
								"id":     "sess2",
								"active": false,
							},
						},
					},
				},
			},
			{
				Name:        "state_delta_add",
				Description: "StateDelta event with add operation",
				Format:      "application/json",
				SDK:         "go",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"STATE_DELTA","timestamp":1640995200,"delta":[{"op":"add","path":"/counter","value":1}]}`),
				Expected: &events.StateDeltaEvent{
					BaseEvent: &events.BaseEvent{
						EventType: events.EventTypeStateDelta,
						TimestampMs: int64Ptr(1640995200),
					},
					Delta: []events.JSONPatchOperation{
						{
							Op:    "add",
							Path:  "/counter",
							Value: float64(1),
						},
					},
				},
			},
		},
	},
}

// EdgeCaseTestVectors contains test vectors for edge cases and corner cases
var EdgeCaseTestVectors = TestVectorSet{
	Version:     "1.0.0",
	Format:      "application/json",
	Description: "Edge cases and corner cases for encoding validation",
	Vectors: []TestVector{
		{
			Name:        "empty_message_delta",
			Description: "TextMessageContent with empty delta",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"TEXT_MESSAGE_CONTENT","timestamp":1640995200,"messageId":"msg-empty","delta":""}`),
			ShouldFail:  true,
			FailureMsg:  "Empty delta not allowed",
		},
		{
			Name:        "very_large_timestamp",
			Description: "Event with very large timestamp",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"RUN_STARTED","timestamp":9223372036854775807,"runId":"run-max","threadId":"thread-max"}`),
			Expected: &events.RunStartedEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeRunStarted,
					TimestampMs: int64Ptr(9223372036854775807), // max int64
				},
				RunID:    "run-max",
				ThreadID: "thread-max",
			},
		},
		{
			Name:        "zero_timestamp",
			Description: "Event with zero timestamp",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"RUN_STARTED","timestamp":0,"runId":"run-zero","threadId":"thread-zero"}`),
			Expected: &events.RunStartedEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeRunStarted,
					TimestampMs: int64Ptr(0),
				},
				RunID:    "run-zero",
				ThreadID: "thread-zero",
			},
		},
		{
			Name:        "missing_optional_fields",
			Description: "Event with only required fields",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"RUN_STARTED","runId":"run-minimal","threadId":"thread-minimal"}`),
			Expected: &events.RunStartedEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeRunStarted,
				},
				RunID:    "run-minimal",
				ThreadID: "thread-minimal",
			},
		},
		{
			Name:        "unicode_ids",
			Description: "Event with Unicode characters in IDs",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"RUN_STARTED","timestamp":1640995200,"runId":"run-測試-🚀","threadId":"thread-тест-🔧"}`),
			Expected: &events.RunStartedEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeRunStarted,
					TimestampMs: int64Ptr(1640995200),
				},
				RunID:    "run-測試-🚀",
				ThreadID: "thread-тест-🔧",
			},
		},
		{
			Name:        "very_long_string",
			Description: "Event with very long string field",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"TEXT_MESSAGE_CONTENT","timestamp":1640995200,"messageId":"msg-long","delta":"` + generateLongString(10000) + `"}`),
			Expected: &events.TextMessageContentEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeTextMessageContent,
					TimestampMs: int64Ptr(1640995200),
				},
				MessageID: "msg-long",
				Delta:     generateLongString(10000),
			},
		},
		{
			Name:        "deeply_nested_snapshot",
			Description: "StateSnapshot with deeply nested structure",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"STATE_SNAPSHOT","timestamp":1640995200,"snapshot":` + generateDeeplyNestedJSON(10) + `}`),
			Expected: &events.StateSnapshotEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeStateSnapshot,
					TimestampMs: int64Ptr(1640995200),
				},
				Snapshot: generateDeeplyNestedMap(10),
			},
		},
		{
			Name:        "null_values",
			Description: "Event with explicit null values",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"STATE_SNAPSHOT","timestamp":1640995200,"snapshot":{"nullValue":null,"emptyString":"","zeroNumber":0}}`),
			Expected: &events.StateSnapshotEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeStateSnapshot,
					TimestampMs: int64Ptr(1640995200),
				},
				Snapshot: map[string]interface{}{
					"nullValue":   nil,
					"emptyString": "",
					"zeroNumber":  float64(0),
				},
			},
		},
	},
}

// MalformedTestVectors contains test vectors for malformed inputs that should fail
var MalformedTestVectors = TestVectorSet{
	Version:     "1.0.0",
	Format:      "application/json",
	Description: "Malformed input test vectors that should fail validation",
	Vectors: []TestVector{
		{
			Name:        "invalid_json_syntax",
			Description: "Invalid JSON syntax",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"RUN_STARTED", "runId":"run-123" invalid json`),
			ShouldFail:  true,
			FailureMsg:  "Invalid JSON syntax",
		},
		{
			Name:        "missing_required_event_type",
			Description: "Event missing required eventType field",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"timestamp":1640995200,"runId":"run-123","threadId":"thread-456"}`),
			ShouldFail:  true,
			FailureMsg:  "Missing required eventType field",
		},
		{
			Name:        "unknown_event_type",
			Description: "Event with unknown event type",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"UNKNOWN_EVENT_TYPE","timestamp":1640995200}`),
			ShouldFail:  true,
			FailureMsg:  "Unknown event type",
		},
		{
			Name:        "negative_timestamp",
			Description: "Event with negative timestamp",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"RUN_STARTED","timestamp":-1,"runId":"run-123","threadId":"thread-456"}`),
			Expected: &events.RunStartedEvent{
				BaseEvent: &events.BaseEvent{
					EventType:   events.EventTypeRunStarted,
					TimestampMs: int64Ptr(-1),
				},
				RunID:    "run-123",
				ThreadID: "thread-456",
			},
		},
		{
			Name:        "missing_required_run_id",
			Description: "RunStarted event missing required runId",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"RUN_STARTED","timestamp":1640995200,"threadId":"thread-456"}`),
			ShouldFail:  true,
			FailureMsg:  "Missing required runId field",
		},
		{
			Name:        "empty_message_id",
			Description: "TextMessage event with empty messageId",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"TEXT_MESSAGE_START","timestamp":1640995200,"messageId":""}`),
			ShouldFail:  true,
			FailureMsg:  "Empty messageId not allowed",
		},
		{
			Name:        "invalid_state_delta_operation",
			Description: "StateDelta with invalid operation",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"STATE_DELTA","timestamp":1640995200,"delta":[{"op":"invalid","path":"/test","value":123}]}`),
			ShouldFail:  true,
			FailureMsg:  "Invalid delta operation",
		},
		{
			Name:        "malformed_utf8",
			Description: "Data with malformed UTF-8",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte("{\x22type\x22:\x22RUN_STARTED\x22,\x22runId\x22:\x22run-\xff\xfe\x22}"),
			ShouldFail:  true,
			FailureMsg:  "Malformed UTF-8 encoding",
		},
		{
			Name:        "circular_reference",
			Description: "Data with circular reference structure",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"STATE_SNAPSHOT","timestamp":1640995200,"snapshot":{"self":{"$ref":"#"}}}`),
			Expected: &events.StateSnapshotEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeStateSnapshot,
					TimestampMs: int64Ptr(1640995200),
				},
				Snapshot: map[string]interface{}{
					"self": map[string]interface{}{
						"$ref": "#",
					},
				},
			},
		},
		{
			Name:        "script_injection",
			Description: "Data with script injection attempt",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"TEXT_MESSAGE_CONTENT","timestamp":1640995200,"messageId":"msg-123","delta":"<script>alert('xss')</script>"}`),
			Expected: &events.TextMessageContentEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeTextMessageContent,
					TimestampMs: int64Ptr(1640995200),
				},
				MessageID: "msg-123",
				Delta:     "<script>alert('xss')</script>",
			},
		},
	},
}

// CrossSDKTestVectors contains test vectors for cross-SDK compatibility
var CrossSDKTestVectors = map[string]TestVectorSet{
	"typescript": {
		Version:     "1.0.0",
		Format:      "application/json",
		Description: "Test vectors from TypeScript SDK",
		Vectors: []TestVector{
			{
				Name:        "typescript_camel_case",
				Description: "TypeScript event with camelCase fields",
				Format:      "application/json",
				SDK:         "typescript",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"RUN_STARTED","timestamp":1640995200,"runId":"run-ts-123","threadId":"thread-ts-456"}`),
				Expected: &events.RunStartedEvent{
					BaseEvent: &events.BaseEvent{
						EventType: events.EventTypeRunStarted,
						TimestampMs: int64Ptr(1640995200),
					},
					RunID:    "run-ts-123",
					ThreadID: "thread-ts-456",
				},
			},
			{
				Name:        "typescript_iso_timestamp",
				Description: "TypeScript event with ISO timestamp format",
				Format:      "application/json",
				SDK:         "typescript",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"RUN_STARTED","timestamp":"2022-01-01T12:00:00Z","runId":"run-ts-iso","threadId":"thread-ts-iso"}`),
				ShouldFail:  true, // Our Go SDK expects numeric timestamps
				FailureMsg:  "ISO timestamp format not supported",
			},
		},
	},
	"python": {
		Version:     "1.0.0",
		Format:      "application/json",
		Description: "Test vectors from Python SDK",
		Vectors: []TestVector{
			{
				Name:        "python_snake_case",
				Description: "Python event with snake_case fields",
				Format:      "application/json",
				SDK:         "python",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"RUN_STARTED","timestamp":1640995200,"runId":"run-py-123","threadId":"thread-py-456"}`),
				Expected: &events.RunStartedEvent{
					BaseEvent: &events.BaseEvent{
						EventType: events.EventTypeRunStarted,
						TimestampMs: int64Ptr(1640995200),
					},
					RunID:    "run-py-123",
					ThreadID: "thread-py-456",
				},
			},
			{
				Name:        "python_none_values",
				Description: "Python event with None values",
				Format:      "application/json",
				SDK:         "python",
				Version:     "1.0.0",
				Input:       []byte(`{"type":"STATE_SNAPSHOT","timestamp":1640995200,"snapshot":{"key":null,"value":"test"}}`),
				Expected: &events.StateSnapshotEvent{
					BaseEvent: &events.BaseEvent{
						EventType: events.EventTypeStateSnapshot,
						TimestampMs: int64Ptr(1640995200),
					},
					Snapshot: map[string]interface{}{
						"key":   nil,
						"value": "test",
					},
				},
			},
		},
	},
}

// SecurityTestVectors contains test vectors for security validation
var SecurityTestVectors = TestVectorSet{
	Version:     "1.0.0",
	Format:      "application/json",
	Description: "Security test vectors for injection and attack prevention",
	Vectors: []TestVector{
		{
			Name:        "xss_attempt_basic",
			Description: "Basic XSS injection attempt",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"TEXT_MESSAGE_CONTENT","timestamp":1640995200,"messageId":"msg-xss","delta":"<script>alert('XSS')</script>"}`),
			ShouldFail:  true,
			FailureMsg:  "XSS attempt detected",
		},
		{
			Name:        "sql_injection_attempt",
			Description: "SQL injection attempt in string field",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"TEXT_MESSAGE_CONTENT","timestamp":1640995200,"messageId":"msg-sql","delta":"'; DROP TABLE users; --"}`),
			ShouldFail:  true,
			FailureMsg:  "SQL injection attempt detected",
		},
		{
			Name:        "javascript_protocol",
			Description: "JavaScript protocol injection",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"TEXT_MESSAGE_CONTENT","timestamp":1640995200,"messageId":"msg-js","delta":"javascript:alert('XSS')"}`),
			ShouldFail:  true,
			FailureMsg:  "JavaScript protocol injection detected",
		},
		{
			Name:        "data_uri_html",
			Description: "Data URI with HTML content",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"TEXT_MESSAGE_CONTENT","timestamp":1640995200,"messageId":"msg-data","delta":"data:text/html,<script>alert('XSS')</script>"}`),
			ShouldFail:  true,
			FailureMsg:  "Data URI HTML injection detected",
		},
		{
			Name:        "oversized_payload",
			Description: "Oversized payload attack",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"TEXT_MESSAGE_CONTENT","timestamp":1640995200,"messageId":"msg-large","delta":"` + generateLongString(1000000) + `"}`),
			ShouldFail:  true,
			FailureMsg:  "Payload size exceeds limit",
		},
		{
			Name:        "billion_laughs",
			Description: "Billion laughs XML entity expansion attack",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"TEXT_MESSAGE_CONTENT","timestamp":1640995200,"messageId":"msg-xml","delta":"<!DOCTYPE lolz [<!ENTITY lol \"lol\"><!ENTITY lol2 \"&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;\">]><lolz>&lol2;</lolz>"}`),
			ShouldFail:  true,
			FailureMsg:  "XML entity expansion attack detected",
		},
		{
			Name:        "null_byte_injection",
			Description: "Null byte injection attempt",
			Format:      "application/json",
			SDK:         "go",
			Version:     "1.0.0",
			Input:       append(append([]byte(`{"eventType":"text.message.content","messageId":"msg-null","delta":"test`), 0), []byte(`injection"}`)...),
			ShouldFail:  true,
			FailureMsg:  "Null byte injection detected",
		},
	},
}

// Helper functions for generating test data

func generateLongString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[i%len(charset)]
	}
	return string(result)
}

func generateDeeplyNestedJSON(depth int) string {
	if depth <= 0 {
		return `"leaf"`
	}
	return `{"level":` + fmt.Sprintf("%d", depth) + `,"nested":` + generateDeeplyNestedJSON(depth-1) + `}`
}

func generateDeeplyNestedMap(depth int) interface{} {
	if depth <= 0 {
		return "leaf"
	}
	return map[string]interface{}{
		"level":  float64(depth),
		"nested": generateDeeplyNestedMap(depth - 1),
	}
}

// TestVectorRegistry manages test vector collections
type TestVectorRegistry struct {
	vectorSets map[string]TestVectorSet
}

// NewTestVectorRegistry creates a new test vector registry
func NewTestVectorRegistry() *TestVectorRegistry {
	registry := &TestVectorRegistry{
		vectorSets: make(map[string]TestVectorSet),
	}
	
	// Register standard test vectors
	for name, vectorSet := range StandardTestVectors {
		registry.vectorSets[name] = vectorSet
	}
	
	// Register edge case vectors
	registry.vectorSets["edge_cases"] = EdgeCaseTestVectors
	
	// Register malformed vectors
	registry.vectorSets["malformed"] = MalformedTestVectors
	
	// Register cross-SDK vectors
	for sdk, vectorSet := range CrossSDKTestVectors {
		registry.vectorSets["cross_sdk_"+sdk] = vectorSet
	}
	
	// Register security vectors
	registry.vectorSets["security"] = SecurityTestVectors
	
	return registry
}

// GetVectorSet returns a test vector set by name
func (r *TestVectorRegistry) GetVectorSet(name string) (TestVectorSet, bool) {
	vectorSet, ok := r.vectorSets[name]
	return vectorSet, ok
}

// GetAllVectorSets returns all registered test vector sets
func (r *TestVectorRegistry) GetAllVectorSets() map[string]TestVectorSet {
	result := make(map[string]TestVectorSet)
	for name, vectorSet := range r.vectorSets {
		result[name] = vectorSet
	}
	return result
}

// RegisterVectorSet registers a new test vector set
func (r *TestVectorRegistry) RegisterVectorSet(name string, vectorSet TestVectorSet) {
	r.vectorSets[name] = vectorSet
}

// GetVectorsBySDK returns all test vectors for a specific SDK
func (r *TestVectorRegistry) GetVectorsBySDK(sdk string) []TestVector {
	var vectors []TestVector
	for _, vectorSet := range r.vectorSets {
		for _, vector := range vectorSet.Vectors {
			if vector.SDK == sdk {
				vectors = append(vectors, vector)
			}
		}
	}
	return vectors
}

// GetVectorsByFormat returns all test vectors for a specific format
func (r *TestVectorRegistry) GetVectorsByFormat(format string) []TestVector {
	var vectors []TestVector
	for _, vectorSet := range r.vectorSets {
		for _, vector := range vectorSet.Vectors {
			if vector.Format == format {
				vectors = append(vectors, vector)
			}
		}
	}
	return vectors
}

// GetFailureVectors returns all test vectors that should fail
func (r *TestVectorRegistry) GetFailureVectors() []TestVector {
	var vectors []TestVector
	for _, vectorSet := range r.vectorSets {
		for _, vector := range vectorSet.Vectors {
			if vector.ShouldFail {
				vectors = append(vectors, vector)
			}
		}
	}
	return vectors
}

// ExportToJSON exports test vectors to JSON format
func (r *TestVectorRegistry) ExportToJSON(name string) ([]byte, error) {
	vectorSet, ok := r.vectorSets[name]
	if !ok {
		return nil, fmt.Errorf("test vector set not found: %s", name)
	}
	
	return json.MarshalIndent(vectorSet, "", "  ")
}

// ImportFromJSON imports test vectors from JSON format
func (r *TestVectorRegistry) ImportFromJSON(name string, data []byte) error {
	var vectorSet TestVectorSet
	if err := json.Unmarshal(data, &vectorSet); err != nil {
		return fmt.Errorf("failed to unmarshal test vector set: %w", err)
	}
	
	r.vectorSets[name] = vectorSet
	return nil
}

// ValidateVectorSet validates that a test vector set is well-formed
func (r *TestVectorRegistry) ValidateVectorSet(vectorSet TestVectorSet) error {
	if vectorSet.Version == "" {
		return errors.New("vector set version is required")
	}
	
	if vectorSet.Format == "" {
		return errors.New("vector set format is required")
	}
	
	if len(vectorSet.Vectors) == 0 {
		return errors.New("vector set must contain at least one test vector")
	}
	
	// Validate each vector
	for i, vector := range vectorSet.Vectors {
		if err := r.validateVector(vector); err != nil {
			return fmt.Errorf("vector %d validation failed: %w", i, err)
		}
	}
	
	return nil
}

// validateVector validates a single test vector
func (r *TestVectorRegistry) validateVector(vector TestVector) error {
	if vector.Name == "" {
		return errors.New("vector name is required")
	}
	
	if vector.Format == "" {
		return errors.New("vector format is required")
	}
	
	if vector.SDK == "" {
		return errors.New("vector SDK is required")
	}
	
	if len(vector.Input) == 0 {
		return errors.New("vector input is required")
	}
	
	if !vector.ShouldFail && vector.Expected == nil {
		return errors.New("vector expected result is required when ShouldFail is false")
	}
	
	if vector.ShouldFail && vector.FailureMsg == "" {
		return errors.New("vector failure message is required when ShouldFail is true")
	}
	
	return nil
}

// GetStatistics returns statistics about the test vector registry
func (r *TestVectorRegistry) GetStatistics() map[string]interface{} {
	stats := map[string]interface{}{
		"total_vector_sets": len(r.vectorSets),
		"total_vectors":     0,
		"by_sdk":           make(map[string]int),
		"by_format":        make(map[string]int),
		"failure_vectors":  0,
	}
	
	totalVectors := 0
	failureVectors := 0
	bySdk := make(map[string]int)
	byFormat := make(map[string]int)
	
	for _, vectorSet := range r.vectorSets {
		totalVectors += len(vectorSet.Vectors)
		for _, vector := range vectorSet.Vectors {
			bySdk[vector.SDK]++
			byFormat[vector.Format]++
			if vector.ShouldFail {
				failureVectors++
			}
		}
	}
	
	stats["total_vectors"] = totalVectors
	stats["failure_vectors"] = failureVectors
	stats["by_sdk"] = bySdk
	stats["by_format"] = byFormat
	
	return stats
}