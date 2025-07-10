package streaming

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
)

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

func (c *mockStreamCodec) Encode(event events.Event) ([]byte, error) {
	return []byte(event.GetID()), nil
}

func (c *mockStreamCodec) EncodeMultiple(events []events.Event) ([]byte, error) {
	var buf bytes.Buffer
	for _, e := range events {
		buf.WriteString(e.GetID() + "\n")
	}
	return buf.Bytes(), nil
}

func (c *mockStreamCodec) Decode(data []byte) (events.Event, error) {
	return &events.BaseEvent{
		ID:   string(data),
		Type: "test",
	}, nil
}

func (c *mockStreamCodec) DecodeMultiple(data []byte) ([]events.Event, error) {
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

// mockStreamEncoder implementation
func (e *mockStreamEncoder) Encode(event events.Event) ([]byte, error) {
	return []byte(event.GetID()), nil
}

func (e *mockStreamEncoder) EncodeMultiple(events []events.Event) ([]byte, error) {
	var buf bytes.Buffer
	for _, ev := range events {
		buf.WriteString(ev.GetID() + "\n")
	}
	return buf.Bytes(), nil
}

func (e *mockStreamEncoder) ContentType() string {
	return "application/test"
}

func (e *mockStreamEncoder) CanStream() bool {
	return true
}

func (e *mockStreamEncoder) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	for event := range input {
		e.events = append(e.events, event)
		data, _ := e.Encode(event)
		output.Write(data)
		output.Write([]byte("\n"))
	}
	return nil
}

func (e *mockStreamEncoder) StartStream(w io.Writer) error {
	return nil
}

func (e *mockStreamEncoder) WriteEvent(event events.Event) error {
	e.events = append(e.events, event)
	return nil
}

func (e *mockStreamEncoder) EndStream() error {
	return nil
}

// mockStreamDecoder implementation
func (d *mockStreamDecoder) Decode(data []byte) (events.Event, error) {
	return &events.BaseEvent{
		ID:   string(data),
		Type: "test",
	}, nil
}

func (d *mockStreamDecoder) DecodeMultiple(data []byte) ([]events.Event, error) {
	return nil, nil
}

func (d *mockStreamDecoder) ContentType() string {
	return "application/test"
}

func (d *mockStreamDecoder) CanStream() bool {
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

func (d *mockStreamDecoder) StartStream(r io.Reader) error {
	return nil
}

func (d *mockStreamDecoder) ReadEvent() (events.Event, error) {
	if d.index >= len(d.events) {
		return nil, io.EOF
	}
	event := d.events[d.index]
	d.index++
	return event, nil
}

func (d *mockStreamDecoder) EndStream() error {
	return nil
}

// Tests

func TestUnifiedStreamCodec_BasicFunctionality(t *testing.T) {
	baseCodec := newMockStreamCodec()
	config := DefaultUnifiedStreamConfig()
	config.EnableMetrics = true
	
	codec := NewUnifiedStreamCodec(baseCodec, config)

	// Test basic encoding
	event := &events.BaseEvent{
		ID:        "test-1",
		Type:      "test",
		Timestamp: time.Now().Unix(),
	}

	data, err := codec.Encode(event)
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	if string(data) != "test-1" {
		t.Errorf("Expected 'test-1', got %s", string(data))
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

	encoder := NewChunkedEncoder(baseEncoder, config)

	ctx := context.Background()
	input := make(chan events.Event, 5)
	output := make(chan *Chunk, 3)

	// Send events
	go func() {
		defer close(input)
		for i := 0; i < 5; i++ {
			input <- &events.BaseEvent{
				ID:   string(rune('a' + i)),
				Type: "test",
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
	if chunks[0].Header.EventCount != 2 {
		t.Errorf("First chunk should have 2 events, got %d", chunks[0].Header.EventCount)
	}
	if chunks[2].Header.EventCount != 1 {
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
		event := &events.BaseEvent{
			ID:   string(rune('a' + i%26)),
			Type: "test",
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