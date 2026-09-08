package parity_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/client/sse"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding"
	jsoncodec "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding/json"
	"github.com/stretchr/testify/require"
)

type compatibilityFixture struct {
	Request         json.RawMessage `json:"request"`
	ExpectedRequest json.RawMessage `json:"expectedRequest"`
	SnakeRequest    json.RawMessage `json:"snakeRequest"`
	Events          []struct {
		Name         string          `json:"name"`
		Data         json.RawMessage `json:"data"`
		ExpectedType string          `json:"expectedType"`
		Expected     json.RawMessage `json:"expected"`
		Decoder      string          `json:"decoder"`
	} `json:"events"`
}

func loadCompatibilityFixture(t *testing.T) compatibilityFixture {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "parity", "compatibility.json"))
	require.NoError(t, err)
	var f compatibilityFixture
	require.NoError(t, json.Unmarshal(b, &f))
	// Keep the exact pinned identities: a duplicate must not replace a case.
	names := make([]string, 0, len(f.Events))
	for _, event := range f.Events {
		names = append(names, event.Name)
	}
	require.ElementsMatch(t, []string{"THINKING_TEXT_MESSAGE_CONTENT", "REASONING_ENCRYPTED_VALUE", "RUN_FINISHED", "TEXT_MESSAGE_START"}, names)
	require.NotEmpty(t, f.Request)
	require.NotEmpty(t, f.ExpectedRequest)
	require.NotEmpty(t, f.SnakeRequest)
	return f
}

func TestCompatibilityPublicSignatures(t *testing.T) {
	var _ events.Event = (*explicitEvent)(nil)
	var _ encoding.Encoder = jsoncodec.NewJSONEncoder(nil)
	var _ encoding.Decoder = jsoncodec.NewJSONDecoder(nil)
	var _ = sse.Config{Endpoint: "http://example.invalid", APIKey: "k"}
	var _ = sse.Frame{Data: []byte("x")}
	var _ = sse.StreamOptions{Payload: types.RunAgentInput{}}
	client := sse.NewClient(sse.Config{Endpoint: "http://example.invalid"})
	var _ func(sse.StreamOptions) (<-chan sse.Frame, <-chan error, error) = client.Stream
	var _ types.Message = events.Message{}
	var _ types.ToolCall = events.ToolCall{}
	var _ types.FunctionCall = events.Function{}
	require.Equal(t, "activity", events.RoleActivity)

	start := events.NewTextMessageStartEvent("m", events.WithRole("user"), events.WithName("n"))
	require.Equal(t, events.EventTypeTextMessageStart, start.Type())
	require.NoError(t, start.Validate())
	content := events.NewTextMessageContentEvent("m", "hello")
	require.Equal(t, "hello", content.Delta)
	tool := types.ToolCall{ID: "c", Type: types.ToolCallTypeFunction, Function: types.FunctionCall{Name: "f", Arguments: "{}"}}
	require.Equal(t, "f", tool.Function.Name)
	text, ok := (types.Message{Role: types.RoleAssistant, Content: "hello"}).ContentString()
	require.True(t, ok)
	require.Equal(t, "hello", text)
	activity, ok := (types.Message{Role: types.RoleActivity, Content: map[string]any{"x": 1}}).ContentActivity()
	require.True(t, ok)
	require.Equal(t, 1, activity["x"])
	parts, ok := (types.Message{Role: types.RoleUser, Content: []types.InputContent{{Type: types.InputContentTypeImage, Source: &types.InputContentSource{Type: types.InputContentSourceTypeURL, Value: "https://example.invalid/x"}}}}).ContentInputContents()
	require.True(t, ok)
	require.Len(t, parts, 1)
}

func TestCompatibilityRequestsAndRoundTrip(t *testing.T) {
	f := loadCompatibilityFixture(t)
	var in types.RunAgentInput
	require.NoError(t, json.Unmarshal(f.Request, &in))
	require.Len(t, in.Messages, 1)
	require.Len(t, in.Messages[0].ToolCalls, 1)
	require.Len(t, in.Tools, 1)
	require.Len(t, in.Resume, 1)
	require.Equal(t, "camel-thread", in.ThreadID)
	require.Equal(t, "camel-run", in.RunID)
	require.Equal(t, "enc-content", in.Messages[0].EncryptedContent)
	require.Equal(t, types.Metadata{"trace": "t1"}, in.Messages[0].Metadata)
	require.Equal(t, "fixture", in.Messages[0].ToolCalls[0].Metadata["origin"])
	require.Equal(t, types.Metadata{"ui": "card"}, in.Tools[0].Metadata)
	require.Equal(t, types.Metadata{"signed": true}, in.Resume[0].Metadata)
	out, err := json.Marshal(in)
	require.NoError(t, err)
	require.Equal(t, semanticJSON(t, f.ExpectedRequest), semanticJSON(t, out))
	var snake types.RunAgentInput
	require.NoError(t, json.Unmarshal(f.SnakeRequest, &snake))
	require.Equal(t, "snake-only-thread", snake.ThreadID)
	require.Equal(t, "snake-only-run", snake.RunID)
}

// Preserve number tokens so the comparison cannot round away wire corruption.
func semanticJSON(t *testing.T, data []byte) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	require.NoError(t, decoder.Decode(&value))
	return value
}

func TestCompatibilityStrictAndPermissiveCodecErrors(t *testing.T) {
	ctx := context.Background()
	data := []byte(`{"type":"TEXT_MESSAGE_START","messageId":"m","futureField":true}`)
	strict := jsoncodec.NewJSONDecoder(&encoding.DecodingOptions{Strict: true, ValidateEvents: true})
	_, err := strict.Decode(ctx, data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown field")
	permissive := jsoncodec.NewJSONDecoder(&encoding.DecodingOptions{Strict: false, AllowUnknownFields: true, ValidateEvents: true})
	got, err := permissive.Decode(ctx, data)
	require.NoError(t, err)
	require.IsType(t, &events.TextMessageStartEvent{}, got)
	_, err = strict.Decode(ctx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty data")
}

func TestCompatibilityEventDecoderAndConcreteTypes(t *testing.T) {
	f := loadCompatibilityFixture(t)
	decoder := events.NewEventDecoder(nil)
	codec := jsoncodec.NewDefaultJSONCodec()
	ctx := context.Background()
	for _, tc := range f.Events {
		t.Run(tc.Name, func(t *testing.T) {
			data := tc.Data
			var err error
			var got events.Event
			switch tc.Decoder {
			case "sse":
				got, err = decoder.DecodeEvent(tc.Name, data)
			case "eventFromJSON":
				got, err = events.EventFromJSON(data)
			case "json":
				got, err = codec.Decode(ctx, data)
			default:
				t.Fatalf("unknown fixture decoder %q", tc.Decoder)
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, tc.ExpectedType, reflect.TypeOf(got).String())
			encoded, err := got.ToJSON()
			require.NoError(t, err)
			require.Equal(t, semanticJSON(t, tc.Expected), semanticJSON(t, encoded))
		})
	}
}

func TestCompatibilityMetadataUsageAndAbsentOptionals(t *testing.T) {
	e := events.NewRunFinishedEventWithOptions("thread", "run", events.WithSuccessOutcome(), events.WithUsage([]events.TokenUsage{{Provider: "p", Model: "m", InputTokens: events.TokenCount(0)}}))
	require.NoError(t, e.Validate())
	b, err := e.ToJSON()
	require.NoError(t, err)
	var wire struct {
		Result json.RawMessage     `json:"result"`
		Usage  []events.TokenUsage `json:"usage"`
	}
	require.NoError(t, json.Unmarshal(b, &wire))
	require.Nil(t, wire.Result)
	require.Len(t, wire.Usage, 1)
	require.NotNil(t, wire.Usage[0].InputTokens)
	require.Equal(t, int64(0), *wire.Usage[0].InputTokens)
	message := types.Message{ID: "m", Role: types.RoleAssistant, Metadata: types.Metadata{"nullable": nil}}
	b, err = json.Marshal(message)
	require.NoError(t, err)
	require.Contains(t, string(b), `"nullable":null`)
}

// explicitEvent protects the Event interface's complete method set without
// embedding BaseEvent, so a future interface addition breaks this fixture.
type explicitEvent struct{ base events.BaseEvent }

func (e *explicitEvent) Type() events.EventType { return e.base.EventType }
func (e *explicitEvent) Timestamp() *int64      { return e.base.TimestampMs }
func (e *explicitEvent) SetTimestamp(v int64)   { e.base.TimestampMs = &v }
func (e *explicitEvent) ThreadID() string       { return e.base.ThreadID() }
func (e *explicitEvent) RunID() string          { return e.base.RunID() }
func (e *explicitEvent) Validate() error        { return nil }
func (e *explicitEvent) ToJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"type": e.Type()})
}
func (e *explicitEvent) GetBaseEvent() *events.BaseEvent { return &e.base }
