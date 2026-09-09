package events

import (
	"bytes"
	"encoding/json"
	"testing"

	coretypes "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONPatchOperationMarshal_AllOperationsAndRootPointers(t *testing.T) {
	tests := []struct {
		name string
		op   JSONPatchOperation
		want string
	}{
		{"add null at root", JSONPatchOperation{Op: "add", Path: "", Value: nil}, `{"op":"add","path":"","value":null}`},
		{"remove escaped", JSONPatchOperation{Op: "remove", Path: "/a~1b/~0key"}, `{"op":"remove","path":"/a~1b/~0key"}`},
		{"replace null", JSONPatchOperation{Op: "replace", Path: "/value", Value: nil}, `{"op":"replace","path":"/value","value":null}`},
		{"move from root", JSONPatchOperation{Op: "move", Path: "/moved", From: ""}, `{"from":"","op":"move","path":"/moved"}`},
		{"copy escaped", JSONPatchOperation{Op: "copy", Path: "/copy", From: "/a~1b/~0key"}, `{"from":"/a~1b/~0key","op":"copy","path":"/copy"}`},
		{"test null", JSONPatchOperation{Op: "test", Path: "/value", Value: nil}, `{"op":"test","path":"/value","value":null}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.op)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(got))
		})
	}
}

func TestJSONPatchOperationUnmarshal_AllOperations(t *testing.T) {
	inputs := []string{
		`{"op":"add","path":"","value":null}`,
		`{"op":"remove","path":"/a~1b/~0key"}`,
		`{"op":"replace","path":"/value","value":null}`,
		`{"op":"move","path":"/moved","from":""}`,
		`{"op":"copy","path":"/copy","from":"/a~1b/~0key"}`,
		`{"op":"test","path":"/value","value":null}`,
	}

	for _, input := range inputs {
		var op JSONPatchOperation
		require.NoError(t, json.Unmarshal([]byte(input), &op), input)
		encoded, err := json.Marshal(op)
		require.NoError(t, err)
		assert.JSONEq(t, input, string(encoded))
	}
}

func TestJSONPatchOperationUnmarshal_RejectsMalformedRequiredMembers(t *testing.T) {
	tests := []struct {
		name string
		json string
		err  string
	}{
		{"zero operation", `{}`, "op field is required"},
		{"missing path", `{"op":"remove"}`, "path field is required"},
		{"null path", `{"op":"remove","path":null}`, "path field must be a string"},
		{"non-string path", `{"op":"remove","path":1}`, "path field must be a string"},
		{"missing value", `{"op":"add","path":""}`, "value field is required for add operation"},
		{"missing from", `{"op":"move","path":""}`, "from field is required for move operation"},
		{"null from", `{"op":"copy","path":"","from":null}`, "from field must be a string"},
		{"non-string from", `{"op":"copy","path":"","from":false}`, "from field must be a string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var op JSONPatchOperation
			err := json.Unmarshal([]byte(tt.json), &op)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.err)
		})
	}
}

func TestJSONPatchOperationValidate_RejectsInvalidOperation(t *testing.T) {
	for _, op := range []JSONPatchOperation{{}, {Op: "merge", Path: "/x"}} {
		err := validateJSONPatchOperation(op)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "op field must be one of")
	}
}

func TestJSONPatchOperationMarshal_PreservesPopulatedOptionalMembers(t *testing.T) {
	op := JSONPatchOperation{Op: "remove", Path: "/old", Value: "metadata", From: "/source"}
	encoded, err := json.Marshal(op)
	require.NoError(t, err)
	assert.JSONEq(t, `{"op":"remove","path":"/old","value":"metadata","from":"/source"}`, string(encoded))

	var decoded JSONPatchOperation
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, op, decoded)
}

func TestStatePayloadNullAndEmptyArraySemantics(t *testing.T) {
	t.Run("null snapshot is valid", func(t *testing.T) {
		event := NewStateSnapshotEvent(nil)
		require.NoError(t, event.Validate())
		encoded, err := event.ToJSON()
		require.NoError(t, err)
		assert.Contains(t, string(encoded), `"snapshot":null`)
	})

	t.Run("nil and empty delta encode as arrays", func(t *testing.T) {
		for _, delta := range [][]JSONPatchOperation{nil, {}} {
			event := NewStateDeltaEvent(delta)
			event.Delta = delta // Also exercise nil slices supplied through public fields.
			require.NoError(t, event.Validate())
			encoded, err := event.ToJSON()
			require.NoError(t, err)
			assert.Contains(t, string(encoded), `"delta":[]`)
			if delta == nil {
				assert.Nil(t, event.Delta, "marshaling must not mutate the event")
			}
		}
	})

	t.Run("nil messages encode as an array", func(t *testing.T) {
		event := NewMessagesSnapshotEvent(nil)
		encoded, err := event.ToJSON()
		require.NoError(t, err)
		assert.Contains(t, string(encoded), `"messages":[]`)
		assert.Nil(t, event.Messages, "marshaling must not mutate the event")
	})
}

func TestStateDeltaStrictDecoderStillRejectsUnknownEventFields(t *testing.T) {
	decoder := json.NewDecoder(bytes.NewBufferString(`{"type":"STATE_DELTA","delta":[],"unexpected":true}`))
	decoder.DisallowUnknownFields()
	var event StateDeltaEvent
	err := decoder.Decode(&event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field")
}

func TestMessageMarshalUnmarshal_Text(t *testing.T) {
	msg := Message{
		ID:      "msg-1",
		Role:    "user",
		Content: "hello",
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded Message
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "msg-1", decoded.ID)
	assert.Equal(t, "user", string(decoded.Role))
	content, ok := decoded.ContentString()
	require.True(t, ok)
	assert.Equal(t, "hello", content)
	assert.Empty(t, decoded.ActivityType)
}

func TestMessageMarshalUnmarshal_Activity(t *testing.T) {
	msg := Message{
		ID:           "activity-1",
		Role:         RoleActivity,
		ActivityType: "PLAN",
		Content:      map[string]any{"status": "working"},
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded Message
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "activity-1", decoded.ID)
	assert.Equal(t, "activity", string(decoded.Role))
	assert.Equal(t, "PLAN", decoded.ActivityType)
	_, ok := decoded.ContentString()
	assert.False(t, ok)

	content, ok := decoded.ContentActivity()
	require.True(t, ok)
	assert.Equal(t, "working", content["status"])
}

func TestMessageMarshalUnmarshal_Reasoning(t *testing.T) {
	msg := Message{
		ID:             "reasoning-1",
		Role:           coretypes.RoleReasoning,
		Content:        "summary",
		EncryptedValue: "enc-reasoning-1",
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded Message
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "reasoning-1", decoded.ID)
	assert.Equal(t, coretypes.RoleReasoning, decoded.Role)
	assert.Equal(t, "enc-reasoning-1", decoded.EncryptedValue)
	content, ok := decoded.ContentString()
	require.True(t, ok)
	assert.Equal(t, "summary", content)
}

func TestValidateMessage_NonActivityRejectsActivityFields(t *testing.T) {
	msg := Message{
		ID:           "msg-1",
		Role:         "user",
		Content:      "hello",
		ActivityType: "PLAN",
	}

	err := validateMessage(msg)
	assert.Error(t, err)
}

func TestValidateMessage_ActivityRequiresFields(t *testing.T) {
	msg := Message{
		ID:   "activity-1",
		Role: RoleActivity,
	}

	err := validateMessage(msg)
	assert.Error(t, err)

	msg.ActivityType = "PLAN"
	err = validateMessage(msg)
	assert.Error(t, err)

	msg.Content = map[string]any{"status": "draft"}
	err = validateMessage(msg)
	assert.NoError(t, err)

	msg.Content = "not-an-object"
	err = validateMessage(msg)
	assert.Error(t, err)
}

func TestValidateMessage_UserAllowsTextOrMultimodal(t *testing.T) {
	msg := Message{
		ID:      "msg-1",
		Role:    "user",
		Content: "hello",
	}

	assert.NoError(t, validateMessage(msg))

	msg.Content = []coretypes.InputContent{
		{Type: coretypes.InputContentTypeText, Text: "hi"},
		{Type: coretypes.InputContentTypeBinary, MimeType: "image/png", URL: "https://example.com/test.png"},
	}
	assert.NoError(t, validateMessage(msg))

	msg.Content = map[string]any{"unexpected": true}
	assert.Error(t, validateMessage(msg))
}

func TestValidateMessage_AssistantContentMustBeStringWhenPresent(t *testing.T) {
	msg := Message{
		ID:      "msg-1",
		Role:    "assistant",
		Content: map[string]any{"unexpected": true},
	}
	assert.Error(t, validateMessage(msg))

	msg.Content = "ok"
	assert.NoError(t, validateMessage(msg))
}

func TestValidateMessage_ReasoningRequiresStringContent(t *testing.T) {
	msg := Message{
		ID:      "reasoning-1",
		Role:    coretypes.RoleReasoning,
		Content: "summary",
	}

	assert.NoError(t, validateMessage(msg))

	msg.Content = map[string]any{"unexpected": true}
	assert.Error(t, validateMessage(msg))
}

func TestValidateMessage_ToolRequiresToolCallIDAndStringContent(t *testing.T) {
	msg := Message{
		ID:      "msg-1",
		Role:    "tool",
		Content: "ok",
	}
	assert.Error(t, validateMessage(msg))

	msg.ToolCallID = "tool-1"
	assert.NoError(t, validateMessage(msg))

	msg.Content = map[string]any{"unexpected": true}
	assert.Error(t, validateMessage(msg))
}

func TestMessageMarshalJSON_IncludesOptionalFields_Assistant(t *testing.T) {
	msg := Message{
		ID:               "msg-1",
		Role:             "assistant",
		Content:          "hello",
		Name:             "bob",
		EncryptedContent: "enc-content-msg-1",
		ToolCalls: []ToolCall{
			{
				ID:   "tool-1",
				Type: "function",
				Function: Function{
					Name:      "f",
					Arguments: "{}",
				},
			},
		},
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "msg-1", decoded["id"])
	assert.Equal(t, "assistant", decoded["role"])
	assert.Equal(t, "hello", decoded["content"])
	assert.Equal(t, "bob", decoded["name"])
	assert.Equal(t, "enc-content-msg-1", decoded["encryptedContent"])
	toolCalls, ok := decoded["toolCalls"].([]any)
	require.True(t, ok)
	assert.Len(t, toolCalls, 1)
}

func TestMessageMarshalJSON_IncludesOptionalFields_Tool(t *testing.T) {
	msg := Message{
		ID:         "msg-1",
		Role:       "tool",
		Content:    "ok",
		ToolCallID: "tool-123",
		Error:      "boom",
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "msg-1", decoded["id"])
	assert.Equal(t, "tool", decoded["role"])
	assert.Equal(t, "ok", decoded["content"])
	assert.Equal(t, "tool-123", decoded["toolCallId"])
	assert.Equal(t, "boom", decoded["error"])
}
