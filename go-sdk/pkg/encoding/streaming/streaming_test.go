package streaming

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/proto/generated"
)

// mockEvent implements the Event interface for testing
type mockEvent struct {
	EventType   events.EventType `json:"type"`
	TimestampMs *int64           `json:"timestamp,omitempty"`
	Data        string           `json:"data,omitempty"`
}

func (m *mockEvent) Type() events.EventType                { return m.EventType }
func (m *mockEvent) Timestamp() *int64                     { return m.TimestampMs }
func (m *mockEvent) SetTimestamp(timestamp int64)          { m.TimestampMs = &timestamp }
func (m *mockEvent) ToJSON() ([]byte, error)               { return json.Marshal(m) }
func (m *mockEvent) ToProtobuf() (*generated.Event, error) { return nil, nil }
func (m *mockEvent) GetBaseEvent() *events.BaseEvent       { return nil }
func (m *mockEvent) Validate() error                       { return nil }
func (m *mockEvent) ThreadID() string                      { return "test-thread" }
func (m *mockEvent) RunID() string                         { return "test-run" }

// mockStreamCodec implements a basic StreamCodec for testing
type mockStreamCodec struct {
	encoder *mockStreamEncoder
	decoder *mockStreamDecoder
}

type mockStreamEncoder struct {
	events []events.Event
}

type mockStreamDecoder struct {
	events []events.Event
	index  int
}

func newMockStreamCodec() *mockStreamCodec {
	return &mockStreamCodec{
		encoder: &mockStreamEncoder{},
		decoder: &mockStreamDecoder{},
	}
}

func (c *mockStreamCodec) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	// Use event type as a simple identifier for testing
	return []byte(string(event.Type())), nil
}

func (c *mockStreamCodec) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	var buf bytes.Buffer
	for _, e := range events {
		buf.WriteString(string(e.Type()) + "\n")
	}
	return buf.Bytes(), nil
}

func (c *mockStreamCodec) Decode(ctx context.Context, data []byte) (events.Event, error) {
	timestamp := time.Now().UnixMilli()
	return &mockEvent{
		EventType:   events.EventType("test"),
		TimestampMs: &timestamp,
		Data:        string(data),
	}, nil
}

func (c *mockStreamCodec) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	// Simple implementation for testing
	return nil, nil
}

func (c *mockStreamCodec) ContentType() string {
	return "application/test"
}

func (c *mockStreamCodec) CanStream() bool {
	return true
}

func (c *mockStreamCodec) GetStreamEncoder() encoding.StreamEncoder {
	return c.encoder
}

func (c *mockStreamCodec) GetStreamDecoder() encoding.StreamDecoder {
	return c.decoder
}

func (c *mockStreamCodec) SupportsStreaming() bool {
	return true
}

func (c *mockStreamCodec) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	return c.encoder.EncodeStream(ctx, input, output)
}

func (c *mockStreamCodec) DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	return c.decoder.DecodeStream(ctx, input, output)
}

func (c *mockStreamCodec) StartEncoding(ctx context.Context, w io.Writer) error {
	return c.encoder.StartStream(ctx, w)
}

func (c *mockStreamCodec) WriteEvent(ctx context.Context, event events.Event) error {
	return c.encoder.WriteEvent(ctx, event)
}

func (c *mockStreamCodec) EndEncoding(ctx context.Context) error {
	return c.encoder.EndStream(ctx)
}

func (c *mockStreamCodec) StartDecoding(ctx context.Context, r io.Reader) error {
	return c.decoder.StartStream(ctx, r)
}

func (c *mockStreamCodec) ReadEvent(ctx context.Context) (events.Event, error) {
	return c.decoder.ReadEvent(ctx)
}

func (c *mockStreamCodec) EndDecoding(ctx context.Context) error {
	return c.decoder.EndStream(ctx)
}

// mockStreamEncoder implementation
func (e *mockStreamEncoder) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return []byte(string(event.Type())), nil
}

func (e *mockStreamEncoder) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	var buf bytes.Buffer
	for _, ev := range events {
		buf.WriteString(string(ev.Type()) + "\n")
	}
	return buf.Bytes(), nil
}

func (e *mockStreamEncoder) ContentType() string {
	return "application/test"
}

func (e *mockStreamEncoder) CanStream() bool {
	return true
}

func (e *mockStreamEncoder) SupportsStreaming() bool {
	return true
}

func (e *mockStreamEncoder) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	for event := range input {
		e.events = append(e.events, event)
		data, _ := e.Encode(ctx, event)
		output.Write(data)
		output.Write([]byte("\n"))
	}
	return nil
}

func (e *mockStreamEncoder) StartStream(ctx context.Context, w io.Writer) error {
	return nil
}

func (e *mockStreamEncoder) WriteEvent(ctx context.Context, event events.Event) error {
	e.events = append(e.events, event)
	return nil
}

func (e *mockStreamEncoder) EndStream(ctx context.Context) error {
	return nil
}

// mockStreamDecoder implementation
func (d *mockStreamDecoder) Decode(ctx context.Context, data []byte) (events.Event, error) {
	timestamp := time.Now().UnixMilli()
	return &mockEvent{
		EventType:   events.EventType("test"),
		TimestampMs: &timestamp,
		Data:        string(data),
	}, nil
}

func (d *mockStreamDecoder) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return nil, nil
}

func (d *mockStreamDecoder) ContentType() string {
	return "application/test"
}

func (d *mockStreamDecoder) CanStream() bool {
	return true
}

func (d *mockStreamDecoder) SupportsStreaming() bool {
	return true
}

func (d *mockStreamDecoder) DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	defer close(output)
	for _, event := range d.events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case output <- event:
		}
	}
	return nil
}

func (d *mockStreamDecoder) StartStream(ctx context.Context, r io.Reader) error {
	return nil
}

func (d *mockStreamDecoder) ReadEvent(ctx context.Context) (events.Event, error) {
	if d.index >= len(d.events) {
		return nil, io.EOF
	}
	event := d.events[d.index]
	d.index++
	return event, nil
}

func (d *mockStreamDecoder) EndStream(ctx context.Context) error {
	return nil
}

// Tests

func TestUnifiedStreamCodec_BasicFunctionality(t *testing.T) {
	baseCodec := newMockStreamCodec()
	config := DefaultUnifiedStreamConfig()
	config.EnableMetrics = true
	
	codec := NewUnifiedStreamCodec(baseCodec, config)

	// Test basic encoding
	timestamp := time.Now().UnixMilli()
	event := &mockEvent{
		EventType:   events.EventType("test"),
		TimestampMs: &timestamp,
		Data:        "test-1",
	}

	data, err := codec.Encode(context.Background(), event)
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	if string(data) != "test" {
		t.Errorf("Expected 'test', got %s", string(data))
	}

	// Test content type
	if codec.ContentType() != "application/test" {
		t.Errorf("Wrong content type: %s", codec.ContentType())
	}

	// Test streaming capability
	if !codec.CanStream() {
		t.Error("Should support streaming")
	}
}

func TestStreamManager_Lifecycle(t *testing.T) {
	encoder := &mockStreamEncoder{}
	decoder := &mockStreamDecoder{}
	config := DefaultStreamConfig()

	sm := NewStreamManager(encoder, decoder, config)

	// Test start
	if err := sm.Start(); err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	if sm.GetState() != StreamStateActive {
		t.Errorf("Expected active state, got %v", sm.GetState())
	}

	// Test double start
	if err := sm.Start(); err == nil {
		t.Error("Should fail on double start")
	}

	// Test stop
	if err := sm.Stop(); err != nil {
		t.Fatalf("Failed to stop: %v", err)
	}

	if sm.GetState() != StreamStateClosed {
		t.Errorf("Expected closed state, got %v", sm.GetState())
	}
}

func TestChunkedEncoder_BasicChunking(t *testing.T) {
	baseEncoder := &mockStreamEncoder{}
	config := DefaultChunkConfig()
	config.MaxEventsPerChunk = 2
	config.EnableParallelProcessing = false // Disable for deterministic test

	encoder := NewChunkedEncoder(baseEncoder, config)

	ctx := context.Background()
	input := make(chan events.Event, 5)
	output := make(chan *Chunk, 10) // Increase buffer to ensure we don't block

	// Send events
	go func() {
		defer close(input)
		for i := 0; i < 5; i++ {
			timestamp := time.Now().UnixMilli()
			input <- &mockEvent{
				EventType:   events.EventType("test"),
				TimestampMs: &timestamp,
				Data:        string(rune('a' + i)),
			}
		}
	}()

	// Encode chunks
	go func() {
		encoder.EncodeChunked(ctx, input, output)
	}()

	// Collect chunks
	chunks := []*Chunk{}
	for chunk := range output {
		chunks = append(chunks, chunk)
	}

	// Should have 3 chunks (2+2+1 events)
	if len(chunks) != 3 {
		t.Errorf("Expected 3 chunks, got %d", len(chunks))
	}

	// Verify chunk sizes
	if len(chunks) >= 1 && chunks[0].Header.EventCount != 2 {
		t.Errorf("First chunk should have 2 events, got %d", chunks[0].Header.EventCount)
	}
	if len(chunks) >= 3 && chunks[2].Header.EventCount != 1 {
		t.Errorf("Last chunk should have 1 event, got %d", chunks[2].Header.EventCount)
	}
}

func TestFlowController_RateLimiting(t *testing.T) {
	fc := NewFlowController(100)

	// Record some operations
	for i := 0; i < 10; i++ {
		fc.RecordWrite()
	}

	metrics := fc.GetMetrics()
	if metrics.PendingWrites != 10 {
		t.Errorf("Expected 10 pending writes, got %d", metrics.PendingWrites)
	}

	// Complete writes
	for i := 0; i < 10; i++ {
		fc.RecordWriteComplete()
	}

	metrics = fc.GetMetrics()
	if metrics.PendingWrites != 0 {
		t.Errorf("Expected 0 pending writes, got %d", metrics.PendingWrites)
	}
}

func TestStreamMetrics_Collection(t *testing.T) {
	metrics := NewStreamMetrics()
	defer metrics.Close()

	// Record some events
	for i := 0; i < 100; i++ {
		timestamp := time.Now().UnixMilli()
		event := &mockEvent{
			EventType:   events.EventType("test"),
			TimestampMs: &timestamp,
			Data:        string(rune('a' + i%26)),
		}
		metrics.RecordEvent(event)
	}

	// Record some latencies
	metrics.RecordLatency(1000000)  // 1ms
	metrics.RecordLatency(2000000)  // 2ms
	metrics.RecordLatency(500000)   // 0.5ms

	// Get snapshot
	snapshot := metrics.GetSnapshot()

	if snapshot.EventsProcessed != 100 {
		t.Errorf("Expected 100 events processed, got %d", snapshot.EventsProcessed)
	}

	if snapshot.MaxLatencyMs != 2 {
		t.Errorf("Expected max latency 2ms, got %d", snapshot.MaxLatencyMs)
	}

	if snapshot.AvgLatencyMs < 1.0 || snapshot.AvgLatencyMs > 1.5 {
		t.Errorf("Expected avg latency ~1.17ms, got %.2f", snapshot.AvgLatencyMs)
	}
}

func TestCircularBuffer_Operations(t *testing.T) {
	buf := NewCircularBuffer(4)

	// Test push
	for i := 0; i < 4; i++ {
		if !buf.Push(i) {
			t.Errorf("Failed to push item %d", i)
		}
	}

	// Buffer should be full
	if !buf.IsFull() {
		t.Error("Buffer should be full")
	}

	// Next push should fail
	if buf.Push(5) {
		t.Error("Push should fail when buffer is full")
	}

	// Test pop
	for i := 0; i < 4; i++ {
		item, ok := buf.Pop()
		if !ok {
			t.Errorf("Failed to pop item %d", i)
		}
		if item.(int) != i {
			t.Errorf("Expected %d, got %v", i, item)
		}
	}

	// Buffer should be empty
	if !buf.IsEmpty() {
		t.Error("Buffer should be empty")
	}

	// Next pop should fail
	_, ok := buf.Pop()
	if ok {
		t.Error("Pop should fail when buffer is empty")
	}
}