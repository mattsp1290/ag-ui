package json

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/encoding"
	"github.com/stretchr/testify/require"
)

type customJSONEvent struct{ events.BaseEvent }

func (e *customJSONEvent) Validate() error { return nil }
func (e *customJSONEvent) ToJSON() ([]byte, error) {
	return []byte(`{"type":"CUSTOM_APP","metadata":{"source":"custom"},"payload":7}`), nil
}

func TestJSONEncoderRoundTripAndBatchOrdering(t *testing.T) {
	ctx := context.Background()
	fixtures := decoderFixtures()
	enc := NewJSONEncoder(&encoding.EncodingOptions{CrossSDKCompatibility: true, ValidateOutput: true})
	dec := NewJSONDecoder(&encoding.DecodingOptions{Strict: true, ValidateEvents: true})
	for _, fixture := range fixtures {
		fixture.event.GetBaseEvent().Metadata = types.Metadata{"fixture": string(fixture.typ)}
		data, err := enc.Encode(ctx, fixture.event)
		require.NoError(t, err, string(fixture.typ))
		var wire map[string]any
		require.NoError(t, json.Unmarshal(data, &wire))
		require.Equal(t, string(fixture.typ), wire["type"])
		require.Equal(t, map[string]any{"fixture": string(fixture.typ)}, wire["metadata"])
	}

	batch := []events.Event{fixtures[31].event, fixtures[0].event, fixtures[35].event}
	data, err := enc.EncodeMultiple(ctx, batch)
	require.NoError(t, err)
	decoded, err := dec.DecodeMultiple(ctx, data)
	require.NoError(t, err)
	require.Len(t, decoded, len(batch))
	for i := range batch {
		require.Equal(t, batch[i].Type(), decoded[i].Type())
	}

	pretty, err := NewJSONEncoder(&encoding.EncodingOptions{CrossSDKCompatibility: true, ValidateOutput: true, Pretty: true}).Encode(ctx, fixtures[0].event)
	require.NoError(t, err)
	require.Contains(t, string(pretty), "\n")

	_, err = NewJSONEncoder(&encoding.EncodingOptions{CrossSDKCompatibility: true, ValidateOutput: false, MaxSize: 1}).Encode(ctx, fixtures[0].event)
	require.Error(t, err)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = enc.Encode(cancelled, fixtures[0].event)
	require.Error(t, err)
	limited := NewJSONEncoderWithConcurrencyLimit(&encoding.EncodingOptions{CrossSDKCompatibility: true, ValidateOutput: true}, 1)
	_, err = limited.EncodeMultiple(ctx, batch)
	require.NoError(t, err, "batch admission must work with MaxConcurrent=1")
}

func TestJSONEncoderPreservesCustomEventEncoding(t *testing.T) {
	event := &customJSONEvent{BaseEvent: *events.NewBaseEvent(events.EventTypeCustom)}
	data, err := NewJSONEncoder(&encoding.EncodingOptions{CrossSDKCompatibility: true, ValidateOutput: true}).Encode(context.Background(), event)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"CUSTOM_APP","metadata":{"source":"custom"},"payload":7}`, string(data))
}

func TestJSONEncoderValidationAndBatchErrors(t *testing.T) {
	ctx := context.Background()
	encoder := NewJSONEncoder(&encoding.EncodingOptions{CrossSDKCompatibility: true, ValidateOutput: true})
	valid := decoderFixtures()[0].event
	_, err := encoder.EncodeMultiple(ctx, []events.Event{valid, nil})
	require.Error(t, err)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = encoder.EncodeMultiple(cancelled, []events.Event{valid})
	require.Error(t, err)
	_, err = NewJSONEncoder(&encoding.EncodingOptions{CrossSDKCompatibility: true, ValidateOutput: false, MaxSize: 1}).EncodeMultiple(ctx, []events.Event{valid})
	require.Error(t, err)

	var nilBase events.TextMessageStartEvent
	require.NotPanics(t, func() {
		_, err = encoder.Encode(ctx, &nilBase)
	})
	require.Error(t, err)
	emptyType := events.NewTextMessageStartEvent("m")
	emptyType.GetBaseEvent().EventType = ""
	require.NotPanics(t, func() {
		_, err = encoder.Encode(ctx, emptyType)
	})
	require.Error(t, err)
}
