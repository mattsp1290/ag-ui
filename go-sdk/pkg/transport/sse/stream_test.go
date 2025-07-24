package sse

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

func TestEventStreamCreation(t *testing.T) {
	config := DefaultStreamConfig()
	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}

	if stream == nil {
		t.Fatal("Expected stream to be created, got nil")
	}

	if stream.config != config {
		t.Error("Stream config not set correctly")
	}
}

func TestEventStreamStartStop(t *testing.T) {
	// Re-enabled after fixing Close() timeout issues
	config := DefaultStreamConfig()
	config.WorkerCount = 1 // Reduce workers for test
	config.MetricsInterval = 100 * time.Millisecond

	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}

	// Test start
	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}

	// Test double start
	err = stream.Start()
	if err == nil {
		t.Error("Expected error when starting already started stream")
	}

	// Test close
	err = stream.Close()
	if err != nil {
		t.Fatalf("Failed to close stream: %v", err)
	}

	// Test double close
	err = stream.Close()
	if err != nil {
		t.Fatalf("Unexpected error on double close: %v", err)
	}
}

func TestEventStreamProcessing(t *testing.T) {
	// Re-enabled after fixing timeout issues
	config := DefaultStreamConfig()
	config.WorkerCount = 1
	config.BatchEnabled = false       // Disable batching for simpler test
	config.CompressionEnabled = false // Disable compression for simpler test
	config.SequenceEnabled = false    // Disable sequencing for simpler test
	config.EnableMetrics = false      // Disable metrics for simpler test
	config.MaxChunkSize = 1024        // Small chunk size for testing

	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}

	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()

	// Create a test event
	testEvent := events.NewRunStartedEvent("test-thread", "test-run")

	// Send the event
	err = stream.SendEvent(testEvent)
	if err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}

	// Try to receive the processed chunk
	select {
	case chunk := <-stream.ReceiveChunks():
		if chunk == nil {
			t.Error("Received nil chunk")
		}
		if chunk.EventType != string(events.EventTypeRunStarted) {
			t.Errorf("Expected event type %s, got %s", events.EventTypeRunStarted, chunk.EventType)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for chunk")
	}
}

func TestStreamConfigValidation(t *testing.T) {
	tests := []struct {
		name         string
		modifyConfig func(*StreamConfig)
		expectError  bool
	}{
		{
			name:         "valid config",
			modifyConfig: func(c *StreamConfig) {},
			expectError:  false,
		},
		{
			name: "zero event buffer size",
			modifyConfig: func(c *StreamConfig) {
				c.EventBufferSize = 0
			},
			expectError: true,
		},
		{
			name: "zero chunk buffer size",
			modifyConfig: func(c *StreamConfig) {
				c.ChunkBufferSize = 0
			},
			expectError: true,
		},
		{
			name: "zero max chunk size",
			modifyConfig: func(c *StreamConfig) {
				c.MaxChunkSize = 0
			},
			expectError: true,
		},
		{
			name: "invalid batch size",
			modifyConfig: func(c *StreamConfig) {
				c.BatchEnabled = true
				c.BatchSize = 0
			},
			expectError: true,
		},
		{
			name: "batch size exceeds max",
			modifyConfig: func(c *StreamConfig) {
				c.BatchEnabled = true
				c.BatchSize = 100
				c.MaxBatchSize = 50
			},
			expectError: true,
		},
		{
			name: "zero worker count",
			modifyConfig: func(c *StreamConfig) {
				c.WorkerCount = 0
			},
			expectError: true,
		},
		{
			name: "invalid compression level",
			modifyConfig: func(c *StreamConfig) {
				c.CompressionEnabled = true
				c.CompressionLevel = 15
			},
			expectError: true,
		},
		{
			name: "invalid compression type",
			modifyConfig: func(c *StreamConfig) {
				c.CompressionEnabled = true
				c.CompressionType = "invalid"
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultStreamConfig()
			tt.modifyConfig(config)

			_, err := NewEventStream(config)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestFlowController(t *testing.T) {
	maxConcurrent := 2
	timeout := 100 * time.Millisecond
	drainTimeout := 500 * time.Millisecond

	fc := NewFlowController(maxConcurrent, timeout, drainTimeout)
	if fc == nil {
		t.Fatal("Failed to create flow controller")
	}

	ctx := context.Background()

	// Test successful acquisition
	err := fc.Acquire(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire flow control: %v", err)
	}

	err = fc.Acquire(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire second flow control: %v", err)
	}

	// Test timeout on third acquisition
	err = fc.Acquire(ctx)
	if err == nil {
		t.Error("Expected timeout error on third acquisition")
	}

	// Release one and try again
	fc.Release()
	err = fc.Acquire(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire after release: %v", err)
	}

	// Clean up
	fc.Release()
	fc.Release()
}

func TestEventSequencer(t *testing.T) {
	sequencer := NewEventSequencer(true, false, 100)
	if sequencer == nil {
		t.Fatal("Failed to create event sequencer")
	}

	// Create test event
	testEvent := events.NewRunStartedEvent("test-thread", "test-run")

	// Add event
	seqEvent := sequencer.AddEvent(testEvent)
	if seqEvent == nil {
		t.Error("Expected sequenced event, got nil")
	}

	if seqEvent.SequenceNum == 0 {
		t.Error("Expected non-zero sequence number")
	}

	if seqEvent.Event != testEvent {
		t.Error("Event not preserved in sequenced event")
	}
}

func TestBufferPool(t *testing.T) {
	pool := NewBufferPool(1024)
	if pool == nil {
		t.Fatal("Failed to create buffer pool")
	}

	// Get buffer
	buf := pool.Get()
	if buf == nil {
		t.Error("Failed to get buffer from pool")
	}

	// Write some data
	buf.WriteString("test data")
	if buf.Len() == 0 {
		t.Error("Buffer should contain data")
	}

	// Put back
	pool.Put(buf)

	// Get again and verify it's reset
	buf2 := pool.Get()
	if buf2.Len() != 0 {
		t.Error("Buffer should be reset when returned to pool")
	}
}

func TestChunkBuffer(t *testing.T) {
	cb := NewChunkBuffer(1024)
	if cb == nil {
		t.Fatal("Failed to create chunk buffer")
	}

	// Create test chunk
	chunk := &StreamChunk{
		Data:      []byte("test data"),
		EventType: "test",
		EventID:   "test-id",
		Timestamp: time.Now(),
	}

	// Add chunk
	cb.AddChunk(chunk)

	// Create output channel
	outputChan := make(chan *StreamChunk, 10)

	// Flush
	cb.Flush(outputChan)

	// Check if chunk was sent
	select {
	case receivedChunk := <-outputChan:
		if receivedChunk != chunk {
			t.Error("Received chunk doesn't match sent chunk")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for flushed chunk")
	}
}

func TestStreamMetrics(t *testing.T) {
	// Re-enabled after fixing timeout issues
	metrics := NewStreamMetrics()
	if metrics == nil {
		t.Fatal("Failed to create stream metrics")
	}

	if metrics.StartTime.IsZero() {
		t.Error("Start time should be set")
	}
}

func TestSSEChunkFormatting(t *testing.T) {
	chunk := &StreamChunk{
		Data:        []byte(`{"test": "data"}`),
		EventType:   "test-event",
		EventID:     "test-id",
		Compressed:  false,
		ChunkIndex:  0,
		TotalChunks: 1,
		Timestamp:   time.Now(),
	}

	sseData, err := FormatSSEChunk(chunk)
	if err != nil {
		t.Fatalf("Failed to format SSE chunk: %v", err)
	}

	if sseData == "" {
		t.Error("SSE data should not be empty")
	}

	// Check for required SSE fields
	if !strings.Contains(sseData, "event: test-event") {
		t.Error("SSE data should contain event type")
	}

	if !strings.Contains(sseData, "id: test-id") {
		t.Error("SSE data should contain event ID")
	}

	if !strings.Contains(sseData, `data: {"test": "data"}`) {
		t.Error("SSE data should contain event data")
	}
}

func TestCompression(t *testing.T) {
	config := DefaultStreamConfig()
	config.CompressionEnabled = true
	config.CompressionType = CompressionGzip
	config.CompressionLevel = 6
	config.EnableMetrics = false

	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}

	testData := []byte(strings.Repeat("test data for compression ", 100))

	compressedData, err := stream.compressData(testData)
	if err != nil {
		t.Fatalf("Failed to compress data: %v", err)
	}

	if len(compressedData) >= len(testData) {
		t.Error("Compressed data should be smaller than original")
	}
}

func BenchmarkEventProcessing(b *testing.B) {
	config := DefaultStreamConfig()
	config.WorkerCount = 1
	config.BatchEnabled = false
	config.CompressionEnabled = false
	config.SequenceEnabled = false
	config.EnableMetrics = false

	stream, err := NewEventStream(config)
	if err != nil {
		b.Fatalf("Failed to create event stream: %v", err)
	}

	err = stream.Start()
	if err != nil {
		b.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()

	// Create test event
	testEvent := events.NewRunStartedEvent("test-thread", "test-run")

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err = stream.SendEvent(testEvent)
		if err != nil {
			b.Fatalf("Failed to send event: %v", err)
		}

		// Consume the chunk to avoid blocking
		select {
		case <-stream.ReceiveChunks():
		case <-time.After(100 * time.Millisecond):
			b.Fatal("Timeout waiting for chunk")
		}
	}
}
