package events_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding"
	jsoncodec "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/json"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/sse"
	"github.com/stretchr/testify/require"
)

func TestProtocolCodecAcceptsPeerValidEmptyValues(t *testing.T) {
	empty := ""
	eventsToCheck := []events.Event{
		events.NewTextMessageContentEvent("", ""),
		events.NewToolCallArgsEvent("", ""),
		events.NewReasoningMessageContentEvent("", ""),
		events.NewTextMessageChunkEvent(&empty, nil, &empty),
		events.NewToolCallChunkEvent(),
		events.NewReasoningMessageChunkEvent(&empty, &empty),
	}
	for _, event := range eventsToCheck {
		t.Run(string(event.Type()), func(t *testing.T) {
			event.GetBaseEvent().TimestampMs = nil
			require.NoError(t, event.Validate())
			wire, err := json.Marshal(event)
			require.NoError(t, err)
			decoders := map[string]func([]byte) (events.Event, error){
				"eventFromJSON": events.EventFromJSON,
				"eventDecoder": func(data []byte) (events.Event, error) {
					return events.NewEventDecoder(nil).DecodeEvent(string(event.Type()), data)
				},
				"jsonDecoder": func(data []byte) (events.Event, error) {
					return jsoncodec.NewJSONDecoder(nil).Decode(context.Background(), data)
				},
			}
			for name, decode := range decoders {
				t.Run(name, func(t *testing.T) {
					decoded, err := decode(wire)
					require.NoError(t, err)
					require.NoError(t, decoded.Validate())
					encoded, err := jsoncodec.NewJSONEncoder(nil).Encode(context.Background(), decoded)
					require.NoError(t, err)
					require.JSONEq(t, string(wire), string(encoded))
					var output bytes.Buffer
					require.NoError(t, sse.NewSSEWriter().WriteEvent(context.Background(), &output, decoded))
					require.Contains(t, output.String(), "data: ")
				})
			}
		})
	}
}

func TestProtocolCodecValidatesLiteralRoles(t *testing.T) {
	for _, role := range []string{"developer", "system", "assistant", "user"} {
		require.NoError(t, events.NewTextMessageStartEvent("", events.WithRole(role)).Validate())
	}
	for _, role := range []string{"", "tool", "reasoning", "other"} {
		require.Error(t, events.NewTextMessageStartEvent("", events.WithRole(role)).Validate())
	}
}

func TestProtocolCodecRejectsUnknownTerminalOutcomes(t *testing.T) {
	cases := []struct {
		name  string
		wire  string
		build events.Event
	}{
		{"run", `{"type":"RUN_FINISHED","threadId":"","runId":"","outcome":{"type":"unknown"}}`, func() events.Event {
			e := events.NewRunFinishedEvent("", "")
			e.Outcome = &events.RunFinishedOutcome{Type: "unknown"}
			return e
		}()},
		{"subagent", `{"type":"SUBAGENT_FINISHED","subagentRunId":"","outcome":{"type":"unknown"}}`, func() events.Event {
			e := events.NewSubagentFinishedEvent("")
			e.Outcome = &events.SubagentFinishedOutcome{Type: "unknown"}
			return e
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decoded, err := events.EventFromJSON([]byte(tc.wire))
			require.NoError(t, err)
			require.Error(t, decoded.Validate())
			decoded, err = events.NewEventDecoder(nil).DecodeEvent(string(decoded.Type()), []byte(tc.wire))
			require.NoError(t, err)
			require.Error(t, decoded.Validate())
			_, err = jsoncodec.NewJSONDecoder(nil).Decode(context.Background(), []byte(tc.wire))
			require.Error(t, err)
			_, err = jsoncodec.NewJSONEncoder(nil).Encode(context.Background(), tc.build)
			require.Error(t, err)
		})
	}
}

func TestProtocolCodecPreservesOptionalPresence(t *testing.T) {
	zero := int64(0)
	run := events.NewRunFinishedEvent("", "")
	run.Usage = []events.TokenUsage{}
	data, err := json.Marshal(run)
	require.NoError(t, err)
	require.Contains(t, string(data), `"usage":[]`)

	subagent := events.NewSubagentFinishedEvent("", events.WithSubagentSuspendedOutcome([]string{}))
	data, err = json.Marshal(subagent)
	require.NoError(t, err)
	require.Contains(t, string(data), `"interruptIds":[]`)
	subagent.Outcome.InterruptIDs = nil
	data, err = json.Marshal(subagent)
	require.NoError(t, err)
	require.NotContains(t, string(data), `"interruptIds"`)

	usage := events.NewRunFinishedEvent("", "")
	usage.Usage = []events.TokenUsage{{InputTokens: &zero}}
	data, err = json.Marshal(usage)
	require.NoError(t, err)
	var decoded struct {
		Usage []events.TokenUsage `json:"usage"`
	}
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Len(t, decoded.Usage, 1)
	require.NotNil(t, decoded.Usage[0].InputTokens)
	require.EqualValues(t, 0, *decoded.Usage[0].InputTokens)
}

func TestProtocolCodecRejectsMalformedNestedInterruptsAcrossDecoders(t *testing.T) {
	bad := []string{
		`{"id":"i"}`,
		`{"reason":"r"}`,
		`{"id":null,"reason":"r"}`,
		`{"id":"i","reason":1}`,
	}
	for _, interrupt := range bad {
		wire := []byte(`{"type":"RUN_FINISHED","threadId":"","runId":"","outcome":{"type":"interrupt","interrupts":[` + interrupt + `]}}`)
		for _, decode := range []struct {
			name string
			fn   func([]byte) (events.Event, error)
		}{
			{"eventFromJSON", events.EventFromJSON},
			{"eventDecoder", func(data []byte) (events.Event, error) {
				return events.NewEventDecoder(nil).DecodeEvent("RUN_FINISHED", data)
			}},
			{"jsonDecoder", func(data []byte) (events.Event, error) {
				return jsoncodec.NewJSONDecoder(nil).Decode(context.Background(), data)
			}},
		} {
			t.Run(decode.name+"/"+interrupt, func(t *testing.T) {
				_, err := decode.fn(wire)
				require.Error(t, err)
			})
		}
	}
	valid := []byte(`{"type":"RUN_FINISHED","threadId":"","runId":"","outcome":{"type":"interrupt","interrupts":[{"id":"","reason":""}]}}`)
	_, err := events.EventFromJSON(valid)
	require.NoError(t, err)
}

func TestProtocolCodecStrictUnknownEnvelopeAndDefaultEncoders(t *testing.T) {
	wire := []byte(`{"type":"TEXT_MESSAGE_CONTENT","messageId":"","delta":"","unknown":1}`)
	_, err := jsoncodec.NewJSONDecoder(&encoding.DecodingOptions{Strict: true, ValidateEvents: true}).Decode(context.Background(), wire)
	require.Error(t, err)

	valid := events.NewTextMessageContentEvent("", "")
	var output bytes.Buffer
	require.NoError(t, sse.NewSSEWriter().WriteEvent(context.Background(), &output, valid))
	require.Contains(t, output.String(), "data:")
	require.NoError(t, valid.Validate())
}

func TestProtocolCodecOutcomeUnknownMemberFollowsDecoderMode(t *testing.T) {
	wire := []byte(`{"type":"RUN_FINISHED","threadId":"","runId":"","outcome":{"type":"success","extension":true}}`)
	_, err := jsoncodec.NewJSONDecoder(&encoding.DecodingOptions{Strict: true, ValidateEvents: true}).Decode(context.Background(), wire)
	require.Error(t, err)
	decoded, err := jsoncodec.NewJSONDecoder(&encoding.DecodingOptions{AllowUnknownFields: true, ValidateEvents: true}).Decode(context.Background(), wire)
	require.NoError(t, err)
	require.NoError(t, decoded.Validate())
}
