package events_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	jsoncodec "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/json"
	"github.com/stretchr/testify/require"
)

func TestRunStartedPreservesNestedRequest(t *testing.T) {
	const wire = `{
  "type":"RUN_STARTED","threadId":"thread","runId":"run","parentRunId":"parent",
  "metadata":{"trace":"trace","nested":null},
  "input":{
   "threadId":"thread","runId":"run","parentRunId":"parent",
   "state":{"ready":false,"count":0,"nested":null},
   "messages":[{"id":"message","role":"assistant","toolCalls":[{
    "id":"call","type":"function","function":{"name":"lookup","arguments":"{}"},
    "encryptedValue":"cipher","metadata":{"key":null}
   }]}],
   "tools":[{"name":"lookup","description":"","parameters":{"type":"object"}}],
   "context":[{"description":"context","value":"value"}],
   "forwardedProps":{"nested":null},
   "resume":[{"interruptId":"interrupt","status":"resolved","payload":{"approved":true}}]
  }
 }`
	paths := map[string]func([]byte) (events.Event, error){
		"eventFromJSON": events.EventFromJSON,
		"eventDecoder": func(data []byte) (events.Event, error) {
			return events.NewEventDecoder(nil).DecodeEvent("RUN_STARTED", data)
		},
		"jsonDecoder": func(data []byte) (events.Event, error) {
			return jsoncodec.NewJSONDecoder(nil).Decode(context.Background(), data)
		},
	}
	for name, decode := range paths {
		t.Run(name, func(t *testing.T) {
			decoded, err := decode([]byte(wire))
			require.NoError(t, err)
			require.IsType(t, &events.RunStartedEvent{}, decoded)
			run := decoded.(*events.RunStartedEvent)
			require.NotNil(t, run.ParentRunID)
			require.Equal(t, "parent", *run.ParentRunID)
			require.NotNil(t, run.Input)
			require.Len(t, run.Input.Messages, 1)
			require.Len(t, run.Input.Resume, 1)
			direct, err := json.Marshal(run)
			require.NoError(t, err)
			require.JSONEq(t, wire, string(direct))
			marshaled, err := run.ToJSON()
			require.NoError(t, err)
			require.JSONEq(t, wire, string(marshaled))
			encoded, err := jsoncodec.NewJSONEncoder(nil).Encode(context.Background(), run)
			require.NoError(t, err)
			require.JSONEq(t, wire, string(encoded))
		})
	}
}

func TestRunStartedOptionsAndAbsentWireFields(t *testing.T) {
	old := events.NewRunStartedEvent("thread", "run")
	old.TimestampMs = nil
	data, err := old.ToJSON()
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"RUN_STARTED","threadId":"thread","runId":"run"}`, string(data))
	input := &types.RunAgentInput{ThreadID: "thread", RunID: "run"}
	with := events.NewRunStartedEventWithOptions("thread", "run", events.WithParentRunID(""), events.WithRunInput(input))
	require.Same(t, input, with.Input)
	require.NotNil(t, with.ParentRunID)
	require.Empty(t, *with.ParentRunID)
	data, err = with.ToJSON()
	require.NoError(t, err)
	var object map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &object))
	require.Equal(t, `""`, string(object["parentRunId"]))
	require.Contains(t, object, "input")
	events.WithRunInput(nil)(with)
	data, err = with.ToJSON()
	require.NoError(t, err)
	object = nil
	require.NoError(t, json.Unmarshal(data, &object))
	require.NotContains(t, object, "input")
}

func TestMessagesSnapshotPreservesToolEncryption(t *testing.T) {
	for _, cipher := range []string{"", "opaque-cipher"} {
		wire := `{"type":"MESSAGES_SNAPSHOT","messages":[{"id":"message","role":"assistant","toolCalls":[{"id":"call","type":"function","function":{"name":"lookup","arguments":"{}"},"encryptedValue":"` + cipher + `","metadata":{"ag-ui":{"trace":true},"nested":null}}]}]}`
		decoders := map[string]func([]byte) (events.Event, error){
			"eventFromJSON": events.EventFromJSON,
			"eventDecoder": func(data []byte) (events.Event, error) {
				return events.NewEventDecoder(nil).DecodeEvent("MESSAGES_SNAPSHOT", data)
			},
			"jsonDecoder": func(data []byte) (events.Event, error) {
				return jsoncodec.NewJSONDecoder(nil).Decode(context.Background(), data)
			},
		}
		for name, decode := range decoders {
			t.Run(name+"/"+cipher, func(t *testing.T) {
				event, err := decode([]byte(wire))
				require.NoError(t, err)
				encoded, err := jsoncodec.NewJSONEncoder(nil).Encode(context.Background(), event)
				require.NoError(t, err)
				require.JSONEq(t, wire, string(encoded))
			})
		}
	}
}
