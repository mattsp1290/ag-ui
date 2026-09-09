package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessageExplicitEmptyOptionalFieldsRoundTrip(t *testing.T) {
	input := []byte(`{"id":"m","role":"tool","content":"ok","toolCallId":"call","error":"","encryptedValue":"","subagentRunId":""}`)
	var message Message
	require.NoError(t, json.Unmarshal(input, &message))
	encoded, err := json.Marshal(message)
	require.NoError(t, err)
	require.JSONEq(t, string(input), string(encoded))
}

func TestMessageOmittedOptionalFieldsRemainOmitted(t *testing.T) {
	message := Message{ID: "m", Role: RoleAssistant, Content: "ok"}
	encoded, err := json.Marshal(message)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"m","role":"assistant","content":"ok"}`, string(encoded))
}

func TestMessageRoleRequiredEmptyFieldsAreEmitted(t *testing.T) {
	tool := Message{ID: "tool", Role: RoleTool, Content: "ok", ToolCallID: ""}
	encoded, err := json.Marshal(tool)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"tool","role":"tool","content":"ok","toolCallId":""}`, string(encoded))

	activity := Message{ID: "activity", Role: RoleActivity, Content: map[string]any{}, ActivityType: ""}
	encoded, err = json.Marshal(activity)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"activity","role":"activity","content":{},"activityType":""}`, string(encoded))
}

func TestBinaryInputContentAllowsEmptyMimeTypeDuringUnmarshal(t *testing.T) {
	var content InputContent
	require.NoError(t, json.Unmarshal([]byte(`{"type":"binary","mimeType":"","data":"payload"}`), &content))
	require.Equal(t, InputContentTypeBinary, content.Type)
	require.Empty(t, content.MimeType)
	require.Equal(t, "payload", content.Data)
}

func TestBinaryInputContentStillRequiresPayload(t *testing.T) {
	var content InputContent
	err := json.Unmarshal([]byte(`{"type":"binary","mimeType":"application/octet-stream"}`), &content)
	require.Error(t, err)
}

func TestMessageOptionalEmptyPresenceAndLegacyValues(t *testing.T) {
	for _, wire := range []string{
		`{"id":"m","role":"assistant","encryptedValue":null,"subagentRunId":null,"error":null}`,
		`{"id":"m","role":"assistant"}`,
	} {
		var message Message
		require.NoError(t, json.Unmarshal([]byte(wire), &message))
		encoded, err := json.Marshal(message)
		require.NoError(t, err)
		require.JSONEq(t, `{"id":"m","role":"assistant"}`, string(encoded))
	}
	original := Message{ID: "m", Role: RoleAssistant, EncryptedValue: "cipher", SubagentRunID: "child"}
	encoded, err := json.Marshal(original)
	require.NoError(t, err)
	var decoded Message
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, original, decoded, "nonempty values need no presence bookkeeping")
	require.NoError(t, json.Unmarshal([]byte(`{"encryptedValue":"","encrypted_value":"ignored","subagent_run_id":""}`), &decoded))
	encoded, err = json.Marshal(decoded)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"encryptedValue":""`)
	require.Contains(t, string(encoded), `"subagentRunId":""`)
}
