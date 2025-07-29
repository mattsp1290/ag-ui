package encoding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"google.golang.org/protobuf/proto"
)

// createTestEvent creates a test event with specified size characteristics
func createTestEvent(eventType events.EventType, size int) events.Event {
	switch eventType {
	case events.EventTypeTextMessageStart:
		userRole := "user"
		return &events.TextMessageStartEvent{
			BaseEvent: &events.BaseEvent{
				EventType: eventType,
			},
			MessageID: "test-msg-id",
			Role:      &userRole,
		}
	case events.EventTypeTextMessageContent:
		return &events.TextMessageContentEvent{
			BaseEvent: &events.BaseEvent{
				EventType: eventType,
			},
			MessageID: "test-msg-id",
			Delta:     generateString(size),
		}
	case events.EventTypeToolCallArgs:
		return &events.ToolCallArgsEvent{
			BaseEvent: &events.BaseEvent{
				EventType: eventType,
			},
			ToolCallID: "test-tool-id",
			Delta:      generateString(size),
		}
	case events.EventTypeStateSnapshot:
		return &events.StateSnapshotEvent{
			BaseEvent: &events.BaseEvent{
				EventType: eventType,
			},
			Snapshot: createLargeSnapshot(size),
		}
	default:
		userRole := "user"
		return &events.TextMessageStartEvent{
			BaseEvent: &events.BaseEvent{
				EventType: eventType,
			},
			MessageID: "test-msg-id",
			Role:      &userRole,
		}
	}
}

// generateString generates a string of specified length
func generateString(size int) string {
	if size <= 0 {
		return ""
	}
	return string(make([]byte, size))
}

// createLargeSnapshot creates a large snapshot object
func createLargeSnapshot(size int) interface{} {
	data := make(map[string]interface{})
	for i := 0; i < size/100; i++ {
		data[generateString(10)] = generateString(90)
	}
	return data
}

// int64Ptr returns a pointer to an int64 value
func int64Ptr(v int64) *int64 {
	return &v
}

// BenchmarkJSONEncodingWithoutPooling benchmarks JSON encoding without buffer pooling
func BenchmarkJSONEncodingWithoutPooling(b *testing.B) {
	event := createTestEvent(events.EventTypeTextMessageContent, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Direct JSON marshalling without pooling
		data, err := json.Marshal(event)
		if err != nil {
			b.Fatal(err)
		}
		_ = data
	}
}

// BenchmarkJSONEncodingWithPooling benchmarks JSON encoding with buffer pooling
func BenchmarkJSONEncodingWithPooling(b *testing.B) {
	event := createTestEvent(events.EventTypeTextMessageContent, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use buffer pooling for JSON encoding
		buf := GetBuffer(GetOptimalBufferSizeForEvent(event))
		encoder := json.NewEncoder(buf)
		err := encoder.Encode(event)
		if err != nil {
			b.Fatal(err)
		}
		data := make([]byte, buf.Len())
		copy(data, buf.Bytes())
		PutBuffer(buf)
		_ = data
	}
}

// BenchmarkProtobufEncodingWithoutPooling benchmarks protobuf encoding without buffer pooling
func BenchmarkProtobufEncodingWithoutPooling(b *testing.B) {
	event := createTestEvent(events.EventTypeTextMessageContent, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Direct protobuf marshalling without pooling
		pbEvent, err := event.ToProtobuf()
		if err != nil {
			b.Fatal(err)
		}
		data, err := proto.Marshal(pbEvent)
		if err != nil {
			b.Fatal(err)
		}
		_ = data
	}
}

// BenchmarkProtobufEncodingWithPooling benchmarks protobuf encoding with buffer pooling
func BenchmarkProtobufEncodingWithPooling(b *testing.B) {
	event := createTestEvent(events.EventTypeTextMessageContent, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use buffer pooling for protobuf encoding
		pbEvent, err := event.ToProtobuf()
		if err != nil {
			b.Fatal(err)
		}

		// Marshal with buffer pooling
		buf := GetBuffer(GetOptimalBufferSizeForEvent(event) / 2) // Protobuf is more compact
		data, err := proto.Marshal(pbEvent)
		if err != nil {
			b.Fatal(err)
		}
		PutBuffer(buf)
		_ = data
	}
}

// BenchmarkJSONMultipleEncodingWithoutPooling benchmarks JSON multiple encoding without pooling
func BenchmarkJSONMultipleEncodingWithoutPooling(b *testing.B) {
	testEvents := make([]events.Event, 100)
	for i := 0; i < 100; i++ {
		testEvents[i] = createTestEvent(events.EventTypeTextMessageContent, 500)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Direct JSON marshalling without pooling
		data, err := json.Marshal(testEvents)
		if err != nil {
			b.Fatal(err)
		}
		_ = data
	}
}

// BenchmarkJSONMultipleEncodingWithPooling benchmarks JSON multiple encoding with pooling
func BenchmarkJSONMultipleEncodingWithPooling(b *testing.B) {
	testEvents := make([]events.Event, 100)
	for i := 0; i < 100; i++ {
		testEvents[i] = createTestEvent(events.EventTypeTextMessageContent, 500)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use buffer pooling for encoding multiple events
		buf := GetBuffer(GetOptimalBufferSizeForMultiple(testEvents))
		encoder := json.NewEncoder(buf)
		err := encoder.Encode(testEvents)
		if err != nil {
			b.Fatal(err)
		}
		data := make([]byte, buf.Len())
		copy(data, buf.Bytes())
		PutBuffer(buf)
		_ = data
	}
}

// BenchmarkProtobufMultipleEncodingWithoutPooling benchmarks protobuf multiple encoding without pooling
func BenchmarkProtobufMultipleEncodingWithoutPooling(b *testing.B) {
	testEvents := make([]events.Event, 100)
	for i := 0; i < 100; i++ {
		testEvents[i] = createTestEvent(events.EventTypeTextMessageContent, 500)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Manual multiple encoding without pooling
		output := bytes.NewBuffer(nil)
		for _, event := range testEvents {
			pbEvent, err := event.ToProtobuf()
			if err != nil {
				b.Fatal(err)
			}
			data, err := proto.Marshal(pbEvent)
			if err != nil {
				b.Fatal(err)
			}
			output.Write(data)
		}
		_ = output.Bytes()
	}
}

// BenchmarkProtobufMultipleEncodingWithPooling benchmarks protobuf multiple encoding with pooling
func BenchmarkProtobufMultipleEncodingWithPooling(b *testing.B) {
	testEvents := make([]events.Event, 100)
	for i := 0; i < 100; i++ {
		testEvents[i] = createTestEvent(events.EventTypeTextMessageContent, 500)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use buffer pooling for encoding multiple events
		buf := GetBuffer(GetOptimalBufferSizeForMultiple(testEvents) / 2) // Protobuf is more compact
		defer PutBuffer(buf)

		// Encode each event and append to buffer
		for _, event := range testEvents {
			pbEvent, err := event.ToProtobuf()
			if err != nil {
				b.Fatal(err)
			}
			data, err := proto.Marshal(pbEvent)
			if err != nil {
				b.Fatal(err)
			}
			buf.Write(data)
		}
		_ = buf.Bytes()
	}
}

// BenchmarkJSONDecodingWithoutPooling benchmarks JSON decoding without buffer pooling
func BenchmarkJSONDecodingWithoutPooling(b *testing.B) {
	event := createTestEvent(events.EventTypeTextMessageContent, 1000)
	data, err := json.Marshal(event)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decodedEvent events.TextMessageContentEvent
		err := json.Unmarshal(data, &decodedEvent)
		if err != nil {
			b.Fatal(err)
		}
		_ = decodedEvent
	}
}

// BenchmarkJSONDecodingWithPooling benchmarks JSON decoding with buffer pooling
func BenchmarkJSONDecodingWithPooling(b *testing.B) {
	event := createTestEvent(events.EventTypeTextMessageContent, 1000)
	data, err := json.Marshal(event)
	if err != nil {
		b.Fatal(err)
	}

	// This line should be removed as it's not used in the updated implementation

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use buffer pooling for JSON decoding
		buf := GetBuffer(len(data))
		buf.Write(data)

		var decodedEvent events.TextMessageContentEvent
		decoder := json.NewDecoder(buf)
		err := decoder.Decode(&decodedEvent)
		if err != nil {
			b.Fatal(err)
		}
		PutBuffer(buf)
		_ = decodedEvent
	}
}

// BenchmarkDifferentEventSizes benchmarks encoding with different event sizes
func BenchmarkDifferentEventSizes(b *testing.B) {
	sizes := []int{100, 500, 1000, 5000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			event := createTestEvent(events.EventTypeTextMessageContent, size)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Use buffer pooling for encoding different event sizes
				buf := GetBuffer(GetOptimalBufferSizeForEvent(event))
				encoder := json.NewEncoder(buf)
				err := encoder.Encode(event)
				if err != nil {
					b.Fatal(err)
				}
				data := make([]byte, buf.Len())
				copy(data, buf.Bytes())
				PutBuffer(buf)
				_ = data
			}
		})
	}
}

// BenchmarkBufferPoolEfficiency benchmarks buffer pool efficiency
func BenchmarkBufferPoolEfficiency(b *testing.B) {
	event := createTestEvent(events.EventTypeTextMessageContent, 1000)

	// Warm up the pool
	for i := 0; i < 100; i++ {
		buf := GetBuffer(GetOptimalBufferSizeForEvent(event))
		encoder := json.NewEncoder(buf)
		encoder.Encode(event)
		PutBuffer(buf)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := GetBuffer(GetOptimalBufferSizeForEvent(event))
		encoder := json.NewEncoder(buf)
		err := encoder.Encode(event)
		if err != nil {
			b.Fatal(err)
		}
		data := make([]byte, buf.Len())
		copy(data, buf.Bytes())
		PutBuffer(buf)
		_ = data
	}
}

// BenchmarkConcurrentEncoding benchmarks concurrent encoding with pooling
func BenchmarkConcurrentEncoding(b *testing.B) {
	event := createTestEvent(events.EventTypeTextMessageContent, 1000)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := GetBuffer(GetOptimalBufferSizeForEvent(event))
			encoder := json.NewEncoder(buf)
			err := encoder.Encode(event)
			if err != nil {
				b.Fatal(err)
			}
			data := make([]byte, buf.Len())
			copy(data, buf.Bytes())
			PutBuffer(buf)
			_ = data
		}
	})
}

// BenchmarkMemoryAllocation benchmarks memory allocation with and without pooling
func BenchmarkMemoryAllocation(b *testing.B) {
	event := createTestEvent(events.EventTypeTextMessageContent, 1000)

	b.Run("WithoutPooling", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			data, err := json.Marshal(event)
			if err != nil {
				b.Fatal(err)
			}
			_ = data
		}
	})

	b.Run("WithPooling", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf := GetBuffer(GetOptimalBufferSizeForEvent(event))
			encoder := json.NewEncoder(buf)
			err := encoder.Encode(event)
			if err != nil {
				b.Fatal(err)
			}
			data := make([]byte, buf.Len())
			copy(data, buf.Bytes())
			PutBuffer(buf)
			_ = data
		}
	})
}

// BenchmarkOptimalBufferSizing benchmarks the optimal buffer sizing function
func BenchmarkOptimalBufferSizing(b *testing.B) {
	testEvents := []events.Event{
		createTestEvent(events.EventTypeTextMessageStart, 0),
		createTestEvent(events.EventTypeTextMessageContent, 1000),
		createTestEvent(events.EventTypeToolCallArgs, 2000),
		createTestEvent(events.EventTypeStateSnapshot, 5000),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, event := range testEvents {
			size := GetOptimalBufferSizeForEvent(event)
			_ = size
		}
	}
}
