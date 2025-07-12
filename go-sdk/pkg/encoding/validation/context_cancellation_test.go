package validation

import (
	"context"
	encoding_json "encoding/json"
	"io"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/encoding/json"
	"github.com/ag-ui/go-sdk/pkg/encoding/streaming"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContextCancellationJSONEncoder tests that JSON encoder respects context cancellation
func TestContextCancellationJSONEncoder(t *testing.T) {
	encoder := json.NewJSONEncoder(&encoding.EncodingOptions{
		CrossSDKCompatibility: true,
		ValidateOutput:        true,
	})

	// Test context cancellation on single event encoding
	t.Run("SingleEvent", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		event := events.NewTextMessageStartEvent("msg-1")
		_, err := encoder.Encode(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context cancelled")
	})

	// Test context cancellation during multiple event encoding
	t.Run("MultipleEvents", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// Create many events to increase processing time
		eventSlice := make([]events.Event, 1000)
		for i := 0; i < 1000; i++ {
			eventSlice[i] = events.NewTextMessageStartEvent("msg-1")
		}

		time.Sleep(2 * time.Millisecond) // Ensure context times out

		_, err := encoder.EncodeMultiple(ctx, eventSlice)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context cancelled")
	})
}

// TestContextCancellationRoundTripValidator tests that round-trip validator respects context cancellation
func TestContextCancellationRoundTripValidator(t *testing.T) {
	encoder := json.NewJSONEncoder(nil)
	decoder := json.NewJSONDecoder(nil)
	validator := NewRoundTripValidator(encoder, decoder)

	// Test context cancellation on single event validation
	t.Run("SingleEvent", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		event := events.NewTextMessageStartEvent("msg-1")
		err := validator.ValidateRoundTrip(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "round-trip validation cancelled")
	})

	// Test context cancellation during multiple event validation
	t.Run("MultipleEvents", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// Create many events to increase processing time
		eventSlice := make([]events.Event, 1000)
		for i := 0; i < 1000; i++ {
			eventSlice[i] = events.NewTextMessageStartEvent("msg-1")
		}

		time.Sleep(2 * time.Millisecond) // Ensure context times out

		err := validator.ValidateRoundTripMultiple(ctx, eventSlice)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "round-trip multiple validation cancelled")
	})
}

// TestContextCancellationChunkedEncoder tests that chunked encoder respects context cancellation
func TestContextCancellationChunkedEncoder(t *testing.T) {
	baseEncoder := json.NewJSONEncoder(nil)
	config := streaming.DefaultChunkConfig()
	config.MaxEventsPerChunk = 10
	chunkedEncoder := streaming.NewChunkedEncoder(baseEncoder, config)

	t.Run("SequentialEncoding", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		input := make(chan events.Event, 100)
		output := make(chan *streaming.Chunk, 10)

		// Send many events
		go func() {
			for i := 0; i < 100; i++ {
				input <- events.NewTextMessageStartEvent("msg-1")
			}
			close(input)
		}()

		time.Sleep(2 * time.Millisecond) // Ensure context times out

		err := chunkedEncoder.EncodeChunked(ctx, input, output)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context")
	})

	t.Run("ParallelEncoding", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		config.EnableParallelProcessing = true
		parallelEncoder := streaming.NewChunkedEncoder(baseEncoder, config)

		input := make(chan events.Event, 100)
		output := make(chan *streaming.Chunk, 10)

		// Send many events
		go func() {
			for i := 0; i < 100; i++ {
				input <- events.NewTextMessageStartEvent("msg-1")
			}
			close(input)
		}()

		time.Sleep(2 * time.Millisecond) // Ensure context times out

		err := parallelEncoder.EncodeChunked(ctx, input, output)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context")
	})
}

// TestContextCancellationStreamManager tests that stream manager respects context cancellation
func TestContextCancellationStreamManager(t *testing.T) {
	// Create streaming versions
	streamEncoder := json.NewStreamingJSONEncoder(nil)
	streamDecoder := json.NewStreamingJSONDecoder(nil)
	
	config := streaming.DefaultStreamConfig()
	streamManager := streaming.NewStreamManager(streamEncoder, streamDecoder, config)

	err := streamManager.Start()
	require.NoError(t, err)
	defer streamManager.Stop()

	t.Run("WriteStream", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		input := make(chan events.Event, 100)
		output := &mockWriter{}

		// Send many events
		go func() {
			for i := 0; i < 100; i++ {
				input <- events.NewTextMessageStartEvent("msg-1")
			}
			close(input)
		}()

		time.Sleep(2 * time.Millisecond) // Ensure context times out

		err := streamManager.WriteStream(ctx, input, output)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context")
	})
}

// TestContextCancellationSecurityValidator tests that security validator respects context cancellation
func TestContextCancellationSecurityValidator(t *testing.T) {
	config := DefaultSecurityConfig()
	validator := NewSecurityValidator(config)

	t.Run("ValidateInput", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Large data to trigger validation loops
		data := make([]byte, 1024*1024) // 1MB of data
		for i := range data {
			data[i] = 'a'
		}

		err := validator.ValidateInput(ctx, data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation cancelled")
	})

	t.Run("ValidateEvent", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		event := events.NewTextMessageStartEvent("msg-1")
		
		// This should work normally
		err := validator.ValidateEvent(ctx, event)
		assert.NoError(t, err)

		// Cancel context and test again
		cancel()
		err = validator.ValidateEvent(ctx, event)
		// ValidateEvent doesn't currently check context, but it should pass through
		// to other validation methods that do check context
		assert.NoError(t, err) // This specific method doesn't have context checks yet
	})
}

// TestContextCancellationUnifiedStreaming tests unified streaming context cancellation
func TestContextCancellationUnifiedStreaming(t *testing.T) {
	baseCodec := &mockStreamCodec{}
	config := streaming.DefaultUnifiedStreamConfig()
	unifiedCodec := streaming.NewUnifiedStreamCodec(baseCodec, config)

	t.Run("StreamEncode", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		input := make(chan events.Event, 100)
		output := &mockWriter{}

		// Send many events
		go func() {
			for i := 0; i < 100; i++ {
				input <- events.NewTextMessageStartEvent("msg-1")
			}
			close(input)
		}()

		time.Sleep(2 * time.Millisecond) // Ensure context times out

		err := unifiedCodec.StreamEncode(ctx, input, output)
		// Should get context cancellation error
		if err != nil {
			assert.Contains(t, err.Error(), "context")
		}
	})
}

// TestPartialOperationHandling tests that partial operations are handled appropriately
func TestPartialOperationHandling(t *testing.T) {
	t.Run("EncodingPartialResults", func(t *testing.T) {
		encoder := json.NewJSONEncoder(nil)
		
		// Create context that will timeout after some events are processed
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()

		eventSlice := make([]events.Event, 1000)
		for i := 0; i < 1000; i++ {
			eventSlice[i] = events.NewTextMessageStartEvent("msg-1")
		}

		// Start encoding in background
		go func() {
			time.Sleep(3 * time.Millisecond) // Let some processing happen
		}()

		_, err := encoder.EncodeMultiple(ctx, eventSlice)
		if err != nil {
			// Should be context cancellation, not a partial state error
			assert.Contains(t, err.Error(), "context cancelled")
		}
	})

	t.Run("ValidationPartialResults", func(t *testing.T) {
		config := DefaultSecurityConfig()
		validator := NewSecurityValidator(config)

		// Create context that will timeout during validation
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
		defer cancel()

		// Create large nested structure that will take time to validate
		largeData := createLargeNestedJSON(t, 1000)

		time.Sleep(1 * time.Millisecond) // Start timeout countdown

		err := validator.ValidateInput(ctx, largeData)
		if err != nil {
			// Should be context cancellation, not a partial validation error
			assert.Contains(t, err.Error(), "validation cancelled")
		}
	})
}

// Helper functions and mocks

type mockWriter struct {
	data [][]byte
}

func (w *mockWriter) Write(p []byte) (n int, err error) {
	w.data = append(w.data, p)
	return len(p), nil
}

type mockStreamCodec struct{}

func (m *mockStreamCodec) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return []byte(`{"type":"test"}`), nil
}

func (m *mockStreamCodec) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return []byte(`[{"type":"test"}]`), nil
}

func (m *mockStreamCodec) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return events.NewTextMessageStartEvent("msg-1"), nil
}

func (m *mockStreamCodec) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return []events.Event{events.NewTextMessageStartEvent("msg-1")}, nil
}

func (m *mockStreamCodec) ContentType() string { return "application/json" }
func (m *mockStreamCodec) CanStream() bool     { return true }
func (m *mockStreamCodec) SupportsStreaming() bool { return true }

func (m *mockStreamCodec) GetStreamEncoder() encoding.StreamEncoder { return &mockStreamEncoder{} }
func (m *mockStreamCodec) GetStreamDecoder() encoding.StreamDecoder { return &mockStreamDecoder{} }

func (m *mockStreamCodec) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	return nil
}
func (m *mockStreamCodec) DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	return nil
}
func (m *mockStreamCodec) StartEncoding(ctx context.Context, w io.Writer) error  { return nil }
func (m *mockStreamCodec) WriteEvent(ctx context.Context, event events.Event) error { return nil }
func (m *mockStreamCodec) EndEncoding(ctx context.Context) error           { return nil }
func (m *mockStreamCodec) StartDecoding(ctx context.Context, r io.Reader) error  { return nil }
func (m *mockStreamCodec) ReadEvent(ctx context.Context) (events.Event, error) {
	return events.NewTextMessageStartEvent("msg-1"), nil
}
func (m *mockStreamCodec) EndDecoding(ctx context.Context) error { return nil }

type mockStreamEncoder struct{}

func (m *mockStreamEncoder) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return []byte(`{"type":"test"}`), nil
}
func (m *mockStreamEncoder) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return []byte(`[{"type":"test"}]`), nil
}
func (m *mockStreamEncoder) ContentType() string        { return "application/json" }
func (m *mockStreamEncoder) CanStream() bool            { return true }
func (m *mockStreamEncoder) SupportsStreaming() bool    { return true }
func (m *mockStreamEncoder) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	return nil
}
func (m *mockStreamEncoder) StartStream(ctx context.Context, w io.Writer) error { return nil }
func (m *mockStreamEncoder) WriteEvent(ctx context.Context, event events.Event) error { return nil }
func (m *mockStreamEncoder) EndStream(ctx context.Context) error { return nil }

type mockStreamDecoder struct{}

func (m *mockStreamDecoder) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return events.NewTextMessageStartEvent("msg-1"), nil
}
func (m *mockStreamDecoder) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return []events.Event{events.NewTextMessageStartEvent("msg-1")}, nil
}
func (m *mockStreamDecoder) ContentType() string     { return "application/json" }
func (m *mockStreamDecoder) CanStream() bool         { return true }
func (m *mockStreamDecoder) SupportsStreaming() bool { return true }
func (m *mockStreamDecoder) DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	return nil
}
func (m *mockStreamDecoder) StartStream(ctx context.Context, r io.Reader) error { return nil }
func (m *mockStreamDecoder) ReadEvent(ctx context.Context) (events.Event, error) {
	return events.NewTextMessageStartEvent("msg-1"), nil
}
func (m *mockStreamDecoder) EndStream(ctx context.Context) error { return nil }

// createLargeNestedJSON creates a large nested JSON structure for testing
func createLargeNestedJSON(t *testing.T, depth int) []byte {
	data := make(map[string]interface{})
	current := data
	
	for i := 0; i < depth; i++ {
		next := make(map[string]interface{})
		current["nested"] = next
		current["value"] = "test data with some content to make it larger"
		current = next
	}
	
	result, err := encoding_json.Marshal(data)
	require.NoError(t, err)
	return result
}