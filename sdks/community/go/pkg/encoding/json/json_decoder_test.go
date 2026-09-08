package json

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

type decoderFixture struct {
	typ   events.EventType
	want  reflect.Type
	event events.Event
}

func decoderFixtures() []decoderFixture {
	str := func(s string) *string { return &s }
	return []decoderFixture{
		{events.EventTypeTextMessageStart, reflect.TypeOf(&events.TextMessageStartEvent{}), events.NewTextMessageStartEvent("m-1")},
		{events.EventTypeTextMessageContent, reflect.TypeOf(&events.TextMessageContentEvent{}), events.NewTextMessageContentEvent("m-1", "hello")},
		{events.EventTypeTextMessageEnd, reflect.TypeOf(&events.TextMessageEndEvent{}), events.NewTextMessageEndEvent("m-1")},
		{events.EventTypeTextMessageChunk, reflect.TypeOf(&events.TextMessageChunkEvent{}), events.NewTextMessageChunkEvent(str("m-1"), str("assistant"), str("hello"))},
		{events.EventTypeToolCallStart, reflect.TypeOf(&events.ToolCallStartEvent{}), events.NewToolCallStartEvent("tc-1", "search")},
		{events.EventTypeToolCallArgs, reflect.TypeOf(&events.ToolCallArgsEvent{}), events.NewToolCallArgsEvent("tc-1", "{\"q\":1}")},
		{events.EventTypeToolCallEnd, reflect.TypeOf(&events.ToolCallEndEvent{}), events.NewToolCallEndEvent("tc-1")},
		{events.EventTypeToolCallChunk, reflect.TypeOf(&events.ToolCallChunkEvent{}), events.NewToolCallChunkEvent().WithToolCallChunkID("tc-1").WithToolCallChunkDelta("x")},
		{events.EventTypeToolCallResult, reflect.TypeOf(&events.ToolCallResultEvent{}), events.NewToolCallResultEvent("m-1", "tc-1", "result")},
		{events.EventTypeStateSnapshot, reflect.TypeOf(&events.StateSnapshotEvent{}), events.NewStateSnapshotEvent(map[string]any{"answer": 42})},
		{events.EventTypeStateDelta, reflect.TypeOf(&events.StateDeltaEvent{}), events.NewStateDeltaEvent([]events.JSONPatchOperation{{Op: "add", Path: "/answer", Value: 42}})},
		{events.EventTypeMessagesSnapshot, reflect.TypeOf(&events.MessagesSnapshotEvent{}), events.NewMessagesSnapshotEvent(nil)},
		{events.EventTypeActivitySnapshot, reflect.TypeOf(&events.ActivitySnapshotEvent{}), events.NewActivitySnapshotEvent("m-1", "progress", map[string]any{"step": 1})},
		{events.EventTypeActivityDelta, reflect.TypeOf(&events.ActivityDeltaEvent{}), events.NewActivityDeltaEvent("m-1", "progress", []events.JSONPatchOperation{{Op: "replace", Path: "/step", Value: 2}})},
		{events.EventTypeRaw, reflect.TypeOf(&events.RawEvent{}), events.NewRawEvent(map[string]any{"x": true})},
		{events.EventTypeCustom, reflect.TypeOf(&events.CustomEvent{}), events.NewCustomEvent("custom", events.WithValue(map[string]any{"x": true}))},
		{events.EventTypeRunStarted, reflect.TypeOf(&events.RunStartedEvent{}), events.NewRunStartedEvent("thread-1", "run-1")},
		{events.EventTypeRunFinished, reflect.TypeOf(&events.RunFinishedEvent{}), events.NewRunFinishedEvent("thread-1", "run-1")},
		{events.EventTypeRunError, reflect.TypeOf(&events.RunErrorEvent{}), events.NewRunErrorEvent("boom")},
		{events.EventTypeStepStarted, reflect.TypeOf(&events.StepStartedEvent{}), events.NewStepStartedEvent("step-1")},
		{events.EventTypeStepFinished, reflect.TypeOf(&events.StepFinishedEvent{}), events.NewStepFinishedEvent("step-1")},
		{events.EventTypeThinkingStart, reflect.TypeOf(&events.ThinkingStartEvent{}), events.NewThinkingStartEvent()},
		{events.EventTypeThinkingEnd, reflect.TypeOf(&events.ThinkingEndEvent{}), events.NewThinkingEndEvent()},
		{events.EventTypeThinkingTextMessageStart, reflect.TypeOf(&events.ThinkingTextMessageStartEvent{}), events.NewThinkingTextMessageStartEvent()},
		{events.EventTypeThinkingTextMessageContent, reflect.TypeOf(&events.ThinkingTextMessageContentEvent{}), events.NewThinkingTextMessageContentEvent("thought")},
		{events.EventTypeThinkingTextMessageEnd, reflect.TypeOf(&events.ThinkingTextMessageEndEvent{}), events.NewThinkingTextMessageEndEvent()},
		{events.EventTypeReasoningStart, reflect.TypeOf(&events.ReasoningStartEvent{}), events.NewReasoningStartEvent("rm-1")},
		{events.EventTypeReasoningMessageStart, reflect.TypeOf(&events.ReasoningMessageStartEvent{}), events.NewReasoningMessageStartEvent("rm-1", "assistant")},
		{events.EventTypeReasoningMessageContent, reflect.TypeOf(&events.ReasoningMessageContentEvent{}), events.NewReasoningMessageContentEvent("rm-1", "because")},
		{events.EventTypeReasoningMessageEnd, reflect.TypeOf(&events.ReasoningMessageEndEvent{}), events.NewReasoningMessageEndEvent("rm-1")},
		{events.EventTypeReasoningMessageChunk, reflect.TypeOf(&events.ReasoningMessageChunkEvent{}), events.NewReasoningMessageChunkEvent(str("rm-1"), str("part"))},
		{events.EventTypeReasoningEnd, reflect.TypeOf(&events.ReasoningEndEvent{}), events.NewReasoningEndEvent("rm-1")},
		{events.EventTypeReasoningEncryptedValue, reflect.TypeOf(&events.ReasoningEncryptedValueEvent{}), events.NewReasoningEncryptedValueEvent(events.ReasoningEncryptedValueSubtypeMessage, "rm-1", "cipher")},
		{events.EventTypeSubagentStarted, reflect.TypeOf(&events.SubagentStartedEvent{}), events.NewSubagentStartedEvent("sub-1", "research", events.WithSubagentDescription("find facts"))},
		{events.EventTypeSubagentFinished, reflect.TypeOf(&events.SubagentFinishedEvent{}), events.NewSubagentFinishedEvent("sub-1", events.WithSubagentResult(map[string]any{"ok": true}), events.WithSubagentSuccessOutcome())},
		{events.EventTypeSubagentError, reflect.TypeOf(&events.SubagentErrorEvent{}), events.NewSubagentErrorEvent("sub-1", "failed", events.WithSubagentErrorCode("E1"))},
	}
}

func TestJSONDecoderAllProtocolEventsPreserveTypedFields(t *testing.T) {
	ctx := context.Background()
	for _, tc := range decoderFixtures() {
		t.Run(string(tc.typ), func(t *testing.T) {
			tc.event.GetBaseEvent().Metadata = types.Metadata{"trace": "abc", "nullable": nil}
			data, err := tc.event.ToJSON()
			require.NoError(t, err)
			var expected map[string]any
			require.NoError(t, json.Unmarshal(data, &expected))

			paths := []struct {
				name string
				fn   func() (events.Event, error)
			}{
				{"event-from-json", func() (events.Event, error) { return events.EventFromJSON(data) }},
				{"event-decoder", func() (events.Event, error) {
					return events.NewEventDecoder(logrus.New()).DecodeEvent(string(tc.typ), data)
				}},
				{"json-decoder", func() (events.Event, error) {
					return NewJSONDecoder(&encoding.DecodingOptions{Strict: true, ValidateEvents: true}).Decode(ctx, data)
				}},
			}
			for _, path := range paths {
				t.Run(path.name, func(t *testing.T) {
					got, err := path.fn()
					require.NoError(t, err)
					require.NotNil(t, got)
					require.Equal(t, tc.want, reflect.TypeOf(got))
					require.Equal(t, tc.typ, got.Type())
					assertJSONFieldSet(t, expected, got)
				})
			}
		})
	}
}

func assertJSONFieldSet(t *testing.T, expected map[string]any, event events.Event) {
	data, err := event.ToJSON()
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, expected, got)
}

func TestJSONDecoderFixturesMatchManifest(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/parity/manifest.json")
	require.NoError(t, err)
	var manifest struct {
		Events []string `json:"events"`
	}
	require.NoError(t, json.Unmarshal(data, &manifest))
	want := append([]string(nil), manifest.Events...)
	got := make([]string, 0, len(decoderFixtures()))
	for _, fixture := range decoderFixtures() {
		got = append(got, string(fixture.typ))
	}
	sort.Strings(want)
	sort.Strings(got)
	require.Equal(t, want, got)
}

func TestJSONDecoderOptionsAndBatchContracts(t *testing.T) {
	fixture := decoderFixtures()[0]
	data, err := fixture.event.ToJSON()
	require.NoError(t, err)

	unknownField := []byte(string(data[:len(data)-1]) + `,"future":1}`)
	strict := NewJSONDecoder(&encoding.DecodingOptions{Strict: true, ValidateEvents: true})
	_, err = strict.Decode(context.Background(), unknownField)
	require.Error(t, err)
	permissive := NewJSONDecoder(&encoding.DecodingOptions{Strict: false, AllowUnknownFields: true, ValidateEvents: false})
	_, err = permissive.Decode(context.Background(), unknownField)
	require.NoError(t, err)
	strictAllow := NewJSONDecoder(&encoding.DecodingOptions{Strict: true, AllowUnknownFields: true, ValidateEvents: true})
	_, err = strictAllow.Decode(context.Background(), unknownField)
	require.NoError(t, err)

	invalid := []byte(`{"type":"TEXT_MESSAGE_START"}`)
	_, err = NewJSONDecoder(&encoding.DecodingOptions{Strict: true, ValidateEvents: true}).Decode(context.Background(), invalid)
	require.Error(t, err)
	_, err = NewJSONDecoder(&encoding.DecodingOptions{Strict: false, AllowUnknownFields: true, ValidateEvents: false}).Decode(context.Background(), invalid)
	require.NoError(t, err)
	_, err = strict.Decode(context.Background(), []byte(`{"type":"TEXT_MESSAGE_START","messageId":7}`))
	require.Error(t, err)
	_, err = strict.Decode(context.Background(), []byte(`{"type":"NO_SUCH_EVENT"}`))
	require.Error(t, err)
	_, err = strict.Decode(context.Background(), []byte(`{"type":`))
	require.Error(t, err)
	_, err = strict.Decode(context.Background(), []byte(`{"type":"TEXT_MESSAGE_START","messageId":"m"}`))
	require.NoError(t, err)

	batch := []events.Event{decoderFixtures()[0].event, decoderFixtures()[1].event, decoderFixtures()[2].event}
	batchData, err := NewJSONEncoder(nil).EncodeMultiple(context.Background(), batch)
	require.NoError(t, err)
	decoded, err := strict.DecodeMultiple(context.Background(), batchData)
	require.NoError(t, err)
	require.Len(t, decoded, len(batch))
	for i := range decoded {
		require.Equal(t, batch[i].Type(), decoded[i].Type())
	}
	first, _ := decoderFixtures()[0].event.ToJSON()
	_, err = strict.DecodeMultiple(context.Background(), []byte("["+string(first)+`,{"type":"NO_SUCH_EVENT"}]`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "index 1")

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = strict.Decode(cancelled, data)
	require.Error(t, err)
	_, err = strict.DecodeMultiple(cancelled, batchData)
	require.Error(t, err)
	_, err = NewJSONDecoder(&encoding.DecodingOptions{ValidateEvents: false, MaxSize: 1}).DecodeMultiple(context.Background(), batchData)
	require.Error(t, err)
	limited := NewJSONDecoderWithConcurrencyLimit(&encoding.DecodingOptions{ValidateEvents: false}, 1)
	decoded, err = limited.DecodeMultiple(context.Background(), batchData)
	require.NoError(t, err, "batch admission must not exhaust its own MaxConcurrent=1")
	_, err = NewJSONDecoder(&encoding.DecodingOptions{ValidateEvents: false, MaxSize: 1}).Decode(context.Background(), data)
	require.Error(t, err)

	badData, _ := json.Marshal(map[string]any{"type": "TEXT_MESSAGE_START", "messageId": "m"})
	_, err = NewJSONDecoder(&encoding.DecodingOptions{ValidateEvents: true}).Decode(context.Background(), badData)
	require.NoError(t, err) // wire type is valid; W3 owns semantic payload policy.
}
