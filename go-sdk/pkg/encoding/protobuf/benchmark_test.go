package protobuf

import (
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding/json"
)

// Benchmark comparing protobuf vs JSON encoding

func createTestEvent() events.Event {
	timestamp := time.Now().UnixMilli()
	return &events.StateSnapshotEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeStateSnapshot,
			TimestampMs: &timestamp,
		},
		Snapshot: map[string]interface{}{
			"counter":     42,
			"temperature": 23.5,
			"active":      true,
			"name":        "benchmark test",
			"tags":        []string{"test", "benchmark", "performance"},
			"nested": map[string]interface{}{
				"level2": map[string]interface{}{
					"value": "deep nested value",
					"array": []int{1, 2, 3, 4, 5},
				},
			},
		},
	}
}

func BenchmarkProtobufEncode(b *testing.B) {
	encoder := NewProtobufEncoder(nil)
	event := createTestEvent()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encoder.Encode(event)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONEncode(b *testing.B) {
	encoder := json.NewJSONEncoder(nil)
	event := createTestEvent()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encoder.Encode(event)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtobufDecode(b *testing.B) {
	encoder := NewProtobufEncoder(nil)
	decoder := NewProtobufDecoder(nil)
	event := createTestEvent()
	data, _ := encoder.Encode(event)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := decoder.Decode(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONDecode(b *testing.B) {
	encoder := json.NewJSONEncoder(nil)
	decoder := json.NewJSONDecoder(nil)
	event := createTestEvent()
	data, _ := encoder.Encode(event)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := decoder.Decode(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtobufEncodeMultiple(b *testing.B) {
	encoder := NewProtobufEncoder(nil)
	events := []events.Event{
		createTestEvent(),
		createTestEvent(),
		createTestEvent(),
		createTestEvent(),
		createTestEvent(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encoder.EncodeMultiple(events)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONEncodeMultiple(b *testing.B) {
	encoder := json.NewJSONEncoder(nil)
	events := []events.Event{
		createTestEvent(),
		createTestEvent(),
		createTestEvent(),
		createTestEvent(),
		createTestEvent(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encoder.EncodeMultiple(events)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark size comparison
func BenchmarkEncodingSize(b *testing.B) {
	pbEncoder := NewProtobufEncoder(nil)
	jsonEncoder := json.NewJSONEncoder(nil)
	event := createTestEvent()

	pbData, _ := pbEncoder.Encode(event)
	jsonData, _ := jsonEncoder.Encode(event)

	b.Logf("Protobuf size: %d bytes", len(pbData))
	b.Logf("JSON size: %d bytes", len(jsonData))
	b.Logf("Size ratio: %.2fx smaller", float64(len(jsonData))/float64(len(pbData)))
}

// Benchmark different event types
func BenchmarkProtobufEventTypes(b *testing.B) {
	encoder := NewProtobufEncoder(nil)

	benchmarks := []struct {
		name  string
		event events.Event
	}{
		{
			name: "TextMessage",
			event: &events.TextMessageContentEvent{
				BaseEvent: &events.BaseEvent{EventType: events.EventTypeTextMessageContent},
				MessageID: "msg-123",
				Delta:     "This is a sample message content for benchmarking purposes.",
			},
		},
		{
			name: "ToolCall",
			event: &events.ToolCallArgsEvent{
				BaseEvent:  &events.BaseEvent{EventType: events.EventTypeToolCallArgs},
				ToolCallID: "tool-123",
				Delta:      `{"function": "calculate", "params": {"a": 10, "b": 20}}`,
			},
		},
		{
			name: "StateSnapshot",
			event: createTestEvent(),
		},
		{
			name: "StateDelta",
			event: &events.StateDeltaEvent{
				BaseEvent: &events.BaseEvent{EventType: events.EventTypeStateDelta},
				Delta: []events.JSONPatchOperation{
					{Op: "add", Path: "/key1", Value: "value1"},
					{Op: "replace", Path: "/key2", Value: 42},
					{Op: "remove", Path: "/key3"},
				},
			},
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := encoder.Encode(bm.event)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}