package events_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	jsoncodec "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/json"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/sse"
	"github.com/stretchr/testify/require"
)

func TestEmptyPayloadsAcrossCodecs(t *testing.T) {
	cases := []struct{ kind, fields string }{
		{"TEXT_MESSAGE_CONTENT", `"messageId":"message","delta":""`},
		{"REASONING_MESSAGE_CONTENT", `"messageId":"message","delta":""`},
		{"THINKING_TEXT_MESSAGE_CONTENT", `"delta":""`},
		{"TOOL_CALL_ARGS", `"toolCallId":"call","delta":""`},
		{"TOOL_CALL_RESULT", `"messageId":"message","toolCallId":"call","content":"","role":"tool"`},
		{"RAW", `"event":null`},
		{"STATE_SNAPSHOT", `"snapshot":null,"subagentRunId":"child"`},
		{"STATE_DELTA", `"delta":[],"subagentRunId":"child"`},
		{"MESSAGES_SNAPSHOT", `"messages":[]`},
		{"ACTIVITY_DELTA", `"messageId":"activity","activityType":"PLAN","patch":[],"subagentRunId":"child"`},
		{"ACTIVITY_SNAPSHOT", `"messageId":"activity","activityType":"PLAN","content":{},"replace":false,"subagentRunId":"child"`},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			wire := `{"type":"` + tc.kind + `","metadata":{"nested":null},` + tc.fields + `}`
			decoders := map[string]func([]byte) (events.Event, error){
				"eventFromJSON": events.EventFromJSON,
				"eventDecoder":  func(data []byte) (events.Event, error) { return events.NewEventDecoder(nil).DecodeEvent(tc.kind, data) },
				"jsonDecoder": func(data []byte) (events.Event, error) {
					return jsoncodec.NewJSONDecoder(nil).Decode(context.Background(), data)
				},
			}
			for name, decode := range decoders {
				t.Run(name, func(t *testing.T) {
					event, err := decode([]byte(wire))
					require.NoError(t, err)
					require.NoError(t, event.Validate())
					direct, err := event.ToJSON()
					require.NoError(t, err)
					require.JSONEq(t, wire, string(direct))
					encoded, err := jsoncodec.NewJSONEncoder(nil).Encode(context.Background(), event)
					require.NoError(t, err)
					require.JSONEq(t, wire, string(encoded))
					var output bytes.Buffer
					require.NoError(t, sse.NewSSEWriter().WriteEvent(context.Background(), &output, event))
					found := false
					for _, line := range strings.Split(output.String(), "\n") {
						if strings.HasPrefix(line, "data: ") {
							require.JSONEq(t, wire, strings.TrimPrefix(line, "data: "))
							found = true
						}
					}
					require.True(t, found, "SSE frame must contain event data")
				})
			}
		})
	}
}

func TestActivityWireDefaultsDoNotMutate(t *testing.T) {
	snapshot := events.NewActivitySnapshotEvent("activity", "PLAN", map[string]any{})
	snapshot.TimestampMs = nil
	snapshot.Replace = nil
	data, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"ACTIVITY_SNAPSHOT","messageId":"activity","activityType":"PLAN","content":{},"replace":true}`, string(data))
	require.Nil(t, snapshot.Replace)
	snapshot.WithReplace(false)
	data, err = json.Marshal(*snapshot)
	require.NoError(t, err)
	require.Contains(t, string(data), `"replace":false`)
	delta := events.NewActivityDeltaEvent("activity", "PLAN", nil)
	delta.TimestampMs = nil
	require.Empty(t, delta.Patch)
	delta.Patch = nil
	data, err = json.Marshal(*delta)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"ACTIVITY_DELTA","messageId":"activity","activityType":"PLAN","patch":[]}`, string(data))
	require.Nil(t, delta.Patch)
}
