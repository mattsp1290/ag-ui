package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProtocolJSONValidation(t *testing.T) {
	cases := []struct {
		name           string
		validate       func([]byte) error
		valid, invalid []string
	}{
		{"content", ValidateInputContentJSON, []string{
			`{"type":"text","text":""}`,
			`{"type":"binary","mimeType":"","data":"x"}`,
			`{"type":"image","source":{"type":"data","value":"","mimeType":""},"metadata":null}`,
			`{"type":"audio","source":{"type":"url","value":"not a URL"}}`,
			`{"type":"video","source":{"type":"url","value":""}}`,
			`{"type":"document","source":{"type":"data","value":"","mimeType":""}}`,
		}, []string{
			`null`, `[]`, `{}`, `{"type":"unknown"}`, `{"type":"text"}`, `{"type":"text","text":null}`,
			`{"type":"binary","data":"x"}`, `{"type":"binary","mimeType":"text/plain"}`,
			`{"type":"binary","mimeType":"","id":"","url":"","data":""}`,
			`{"type":"image"}`, `{"type":"image","source":null}`, `{"type":"image","source":{"type":"unknown","value":""}}`,
			`{"type":"image","source":{"type":"data","value":""}}`,
			`{"type":"image","source":{"type":"data","mimeType":""}}`,
			`{"type":"image","source":{"type":"data","value":"","mimeType":null}}`,
		}},
		{"message", ValidateMessageJSON, []string{
			`{"id":"","role":"developer","content":""}`,
			`{"id":"","role":"system","content":""}`,
			`{"id":"","role":"assistant","toolCalls":[{"id":"","type":"function","function":{"name":"","arguments":""},"encryptedValue":"","metadata":{"nested":null}}]}`,
			`{"id":"","role":"user","content":[]}`,
			`{"id":"","role":"tool","toolCallId":"","content":"","error":""}`,
			`{"id":"","role":"activity","activityType":"","content":{}}`,
			`{"id":"","role":"reasoning","content":"","encryptedValue":""}`,
		}, []string{
			`{"role":"assistant"}`, `{"id":"","role":"unknown"}`, `{"id":"","role":"system"}`,
			`{"id":"","role":"tool","content":""}`, `{"id":"","role":"activity","activityType":"","content":null}`,
			`{"id":"","role":"user","content":[{"type":"text"}]}`,
			`{"id":"","role":"assistant","content":null}`,
			`{"id":"","role":"assistant","toolCalls":[{"id":"","type":"unknown","function":{"name":"","arguments":""}}]}`,
			`{"id":"","role":"assistant","toolCalls":[{"id":"","type":"function","function":{"name":""}}]}`,
			`{"id":"","role":"assistant","metadata":null}`,
		}},
		{"interrupt", ValidateInterruptJSON, []string{
			`{"id":"","reason":""}`,
			`{"id":"","reason":"","message":"","responseSchema":{},"metadata":{"nested":null},"subagentRunId":""}`,
		}, []string{`{}`, `{"id":""}`, `{"reason":""}`, `{"id":null,"reason":""}`, `{"id":"","reason":1}`, `{"id":"","reason":"","metadata":[]}`}},
		{"request", ValidateRunAgentInputJSON, []string{
			`{"threadId":"","runId":"","messages":[],"tools":[],"context":[],"forwardedProps":null}`,
			`{"threadId":"","runId":"","state":{"nested":null},"messages":[],"tools":[{"name":"","description":"","parameters":null}],"context":[{"description":"","value":""}],"forwardedProps":{},"resume":[{"interruptId":"","status":"resolved","payload":null,"metadata":{"nested":null}}]}`,
		}, []string{
			`{}`, `{"threadId":"","runId":"","messages":[],"tools":[],"context":[]}`,
			`{"threadId":"","runId":"","messages":null,"tools":[],"context":[],"forwardedProps":{}}`,
			`{"threadId":"","runId":"","messages":[],"tools":[],"context":[],"forwardedProps":{},"resume":[{"interruptId":"","status":"pending"}]}`,
			`{"threadId":"","runId":"","messages":[],"tools":[{"name":""}],"context":[],"forwardedProps":{}}`,
			`{"threadId":"","runId":"","messages":[],"tools":[],"context":[{"value":""}],"forwardedProps":{}}`,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, wire := range tc.valid {
				require.NoError(t, tc.validate([]byte(wire)), wire)
			}
			for _, wire := range tc.invalid {
				require.Error(t, tc.validate([]byte(wire)), wire)
			}
		})
	}
}

func TestProducerProtocolValidation(t *testing.T) {
	request := RunAgentInput{ThreadID: "", RunID: ""}
	require.NoError(t, request.ValidateProtocol())
	require.Nil(t, request.Messages)
	require.Nil(t, request.Tools)
	require.Nil(t, request.Context)
	for _, role := range []Role{RoleDeveloper, RoleSystem, RoleAssistant, RoleUser, RoleTool, RoleActivity, RoleReasoning} {
		message := Message{Role: role, Content: ""}
		if role == RoleActivity {
			message.Content = map[string]any{}
		}
		require.NoError(t, message.ValidateProtocol(), role)
	}
	content := InputContent{Type: InputContentTypeImage, Source: &InputContentSource{Type: InputContentSourceTypeData}}
	require.NoError(t, content.ValidateProtocol())
	require.Error(t, ValidateInputContentJSON([]byte(`{"type":"image","source":{"type":"data","value":""}}`)))
	invalid := Message{Role: RoleUser, Content: []InputContent{{Type: "unknown"}}}
	require.Error(t, invalid.ValidateProtocol())
	request.Messages = []Message{invalid}
	require.Error(t, request.ValidateProtocol())
	require.NoError(t, (ToolCall{Type: ToolCallTypeFunction}).ValidateProtocol())
	require.Error(t, (ToolCall{}).ValidateProtocol())
	require.NoError(t, (Interrupt{}).ValidateProtocol())
	require.Error(t, ValidateInterruptJSON([]byte(`{}`)))
	require.Error(t, (Message{Role: RoleAssistant, Content: func() {}}).ValidateProtocol())
}

func TestOptInValidationPreservesPermissiveAliases(t *testing.T) {
	wire := []byte(`{"thread_id":"alias","threadId":"canonical","run_id":"run","messages":[],"tools":[],"context":[],"forwarded_props":{},"resume":[{"interrupt_id":"i","status":"pending"}]}`)
	var input RunAgentInput
	require.NoError(t, json.Unmarshal(wire, &input))
	require.Equal(t, "canonical", input.ThreadID)
	require.Error(t, input.ValidateProtocol())
	input.Resume[0].Status = ResumeStatusResolved
	require.NoError(t, input.ValidateProtocol())
	// Original key presence is only checked by the opt-in canonical-wire API.
	require.Error(t, ValidateRunAgentInputJSON(wire))
	var message Message
	require.NoError(t, json.Unmarshal([]byte(`{"role":"unknown"}`), &message))
	require.Error(t, message.ValidateProtocol())
}
