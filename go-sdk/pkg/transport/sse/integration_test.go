package sse

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// TestStreamSSEIntegration tests integration between EventStream and SSE transport
func TestStreamSSEIntegration(t *testing.T) {
	// Create a test stream
	config := DefaultStreamConfig()
	config.WorkerCount = 1
	config.BatchEnabled = false
	config.CompressionEnabled = false
	config.SequenceEnabled = false
	config.EnableMetrics = true
	
	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}
	
	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()
	
	// Create a test HTTP server that streams SSE
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("Response writer doesn't support flushing")
			return
		}
		
		// Stream chunks as SSE
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		
		for {
			select {
			case chunk := <-stream.ReceiveChunks():
				if chunk == nil {
					return
				}
				
				// Write the chunk as SSE
				if err := WriteSSEChunk(w, chunk); err != nil {
					t.Errorf("Failed to write SSE chunk: %v", err)
					return
				}
				
				flusher.Flush()
				
			case <-ctx.Done():
				return
			}
		}
	}))
	defer server.Close()
	
	// Send test events
	testEvents := []events.Event{
		events.NewRunStartedEvent("test-thread", "test-run"),
		events.NewTextMessageStartEvent("msg-1"),
		events.NewTextMessageContentEvent("msg-1", "Hello, World!"),
		events.NewTextMessageEndEvent("msg-1"),
		events.NewRunFinishedEvent("test-thread", "test-run"),
	}
	
	// Start a goroutine to send events
	go func() {
		time.Sleep(100 * time.Millisecond) // Give server time to start
		
		for _, event := range testEvents {
			if err := stream.SendEvent(event); err != nil {
				t.Errorf("Failed to send event: %v", err)
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()
	
	// Make a request to the SSE endpoint
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to connect to SSE endpoint: %v", err)
	}
	defer resp.Body.Close()
	
	// Verify SSE headers
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type: text/event-stream, got: %s", resp.Header.Get("Content-Type"))
	}
	
	// Read and verify SSE data
	buf := make([]byte, 4096)
	var sseData bytes.Buffer
	
	// Read with timeout
	done := make(chan bool)
	go func() {
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				sseData.Write(buf[:n])
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("Error reading SSE data: %v", err)
				break
			}
		}
		done <- true
	}()
	
	select {
	case <-done:
		// Reading completed
	case <-time.After(3 * time.Second):
		// Timeout - this is expected as SSE streams continuously
	}
	
	// Verify we received SSE-formatted data
	sseContent := sseData.String()
	if sseContent == "" {
		t.Error("No SSE data received")
	}
	
	// Check for SSE event structure
	if !strings.Contains(sseContent, "event:") {
		t.Error("SSE data missing event field")
	}
	
	if !strings.Contains(sseContent, "data:") {
		t.Error("SSE data missing data field")
	}
	
	// Check for specific event types
	expectedEventTypes := []string{"RUN_STARTED", "TEXT_MESSAGE_START", "TEXT_MESSAGE_CONTENT", "TEXT_MESSAGE_END", "RUN_FINISHED"}
	for _, eventType := range expectedEventTypes {
		if !strings.Contains(sseContent, eventType) {
			t.Errorf("SSE data missing expected event type: %s", eventType)
		}
	}
	
	t.Logf("Received SSE data:\n%s", sseContent)
}

// TestStreamCompressionWithSSE tests compressed data over SSE
func TestStreamCompressionWithSSE(t *testing.T) {
	config := DefaultStreamConfig()
	config.WorkerCount = 1
	config.BatchEnabled = false
	config.CompressionEnabled = true
	config.CompressionType = CompressionGzip
	config.MinCompressionSize = 0 // Compress everything for testing
	config.SequenceEnabled = false
	config.EnableMetrics = false
	
	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}
	
	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()
	
	// Create large content that will benefit from compression
	largeContent := strings.Repeat("This is a test message that repeats many times. ", 100)
	event := events.NewTextMessageContentEvent("large-msg", largeContent)
	
	// Send the event
	err = stream.SendEvent(event)
	if err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}
	
	// Receive and verify the chunk
	select {
	case chunk := <-stream.ReceiveChunks():
		if !chunk.Compressed {
			t.Error("Expected compressed chunk")
		}
		
		// Format as SSE and verify it's valid
		sseData, err := FormatSSEChunk(chunk)
		if err != nil {
			t.Fatalf("Failed to format SSE chunk: %v", err)
		}
		
		if !strings.Contains(sseData, "compressed") {
			t.Error("SSE data should indicate compression")
		}
		
		t.Logf("Original size: %d, Compressed size: %d", len(largeContent), len(chunk.Data))
		
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for compressed chunk")
	}
}

// TestStreamBatchingWithSSE tests batched events over SSE
func TestStreamBatchingWithSSE(t *testing.T) {
	config := DefaultStreamConfig()
	config.WorkerCount = 1
	config.BatchEnabled = true
	config.BatchSize = 3
	config.BatchTimeout = 100 * time.Millisecond
	config.CompressionEnabled = false
	config.SequenceEnabled = false
	config.EnableMetrics = false
	
	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}
	
	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()
	
	// Send events that will be batched
	events := []events.Event{
		events.NewTextMessageStartEvent("msg-1"),
		events.NewTextMessageContentEvent("msg-1", "Hello"),
		events.NewTextMessageEndEvent("msg-1"),
	}
	
	for _, event := range events {
		err = stream.SendEvent(event)
		if err != nil {
			t.Fatalf("Failed to send event: %v", err)
		}
	}
	
	// Wait for batch processing
	select {
	case chunk := <-stream.ReceiveChunks():
		if chunk.EventType != "batch" {
			t.Errorf("Expected batch event type, got: %s", chunk.EventType)
		}
		
		// Format as SSE
		sseData, err := FormatSSEChunk(chunk)
		if err != nil {
			t.Fatalf("Failed to format SSE chunk: %v", err)
		}
		
		if !strings.Contains(sseData, "event: batch") {
			t.Error("SSE data should indicate batch event type")
		}
		
		t.Logf("Batch SSE data:\n%s", sseData)
		
	case <-time.After(500 * time.Millisecond):
		t.Error("Timeout waiting for batch")
	}
}

// TestStreamChunkingWithSSE tests large event chunking over SSE
func TestStreamChunkingWithSSE(t *testing.T) {
	config := DefaultStreamConfig()
	config.WorkerCount = 1
	config.MaxChunkSize = 1024 // Small chunk size for testing
	config.BatchEnabled = false
	config.CompressionEnabled = false
	config.SequenceEnabled = false
	config.EnableMetrics = false
	
	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}
	
	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()
	
	// Create large content that will require chunking
	largeContent := strings.Repeat("Large message content. ", 200)
	event := events.NewTextMessageContentEvent("large-msg", largeContent)
	
	err = stream.SendEvent(event)
	if err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}
	
	// Collect all chunks
	var chunks []*StreamChunk
	timeout := time.After(1 * time.Second)
	
	for {
		select {
		case chunk := <-stream.ReceiveChunks():
			chunks = append(chunks, chunk)
			
			// If this is the last chunk, break
			if chunk.ChunkIndex == chunk.TotalChunks-1 {
				goto done
			}
			
		case <-timeout:
			t.Error("Timeout waiting for chunks")
			goto done
		}
	}
	
done:
	if len(chunks) == 0 {
		t.Fatal("No chunks received")
	}
	
	// Verify chunk consistency
	totalChunks := chunks[0].TotalChunks
	if len(chunks) != totalChunks {
		t.Errorf("Expected %d chunks, got %d", totalChunks, len(chunks))
	}
	
	// Verify all chunks have the same event ID
	eventID := chunks[0].EventID
	for i, chunk := range chunks {
		if chunk.EventID != eventID {
			t.Errorf("Chunk %d has different event ID: %s vs %s", i, chunk.EventID, eventID)
		}
		
		if chunk.TotalChunks != totalChunks {
			t.Errorf("Chunk %d has different total chunks: %d vs %d", i, chunk.TotalChunks, totalChunks)
		}
		
		if chunk.ChunkIndex != i {
			t.Errorf("Chunk has wrong index: expected %d, got %d", i, chunk.ChunkIndex)
		}
	}
	
	// Format chunks as SSE and verify
	for i, chunk := range chunks {
		sseData, err := FormatSSEChunk(chunk)
		if err != nil {
			t.Fatalf("Failed to format chunk %d as SSE: %v", i, err)
		}
		
		// Verify chunk metadata is present
		if !strings.Contains(sseData, "chunk_index") {
			t.Errorf("Chunk %d SSE data missing chunk metadata", i)
		}
		
		t.Logf("Chunk %d SSE data:\n%s", i, sseData)
	}
	
	// Verify data can be reassembled
	var reassembled []byte
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk.Data...)
	}
	
	if string(reassembled) != largeContent {
		t.Error("Reassembled data doesn't match original")
	}
}

// TestStreamMetricsCollection tests metrics collection during streaming
func TestStreamMetricsCollection(t *testing.T) {
	config := DefaultStreamConfig()
	config.WorkerCount = 1
	config.EnableMetrics = true
	config.MetricsInterval = 100 * time.Millisecond
	config.BatchEnabled = false
	config.CompressionEnabled = true
	config.MinCompressionSize = 0
	
	stream, err := NewEventStream(config)
	if err != nil {
		t.Fatalf("Failed to create event stream: %v", err)
	}
	
	err = stream.Start()
	if err != nil {
		t.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()
	
	// Send multiple events
	numEvents := 10
	for i := 0; i < numEvents; i++ {
		event := events.NewTextMessageContentEvent("msg", "test content for compression")
		err = stream.SendEvent(event)
		if err != nil {
			t.Fatalf("Failed to send event %d: %v", i, err)
		}
	}
	
	// Consume chunks
	for i := 0; i < numEvents; i++ {
		select {
		case <-stream.ReceiveChunks():
		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for chunk")
		}
	}
	
	// Wait for metrics collection
	time.Sleep(200 * time.Millisecond)
	
	// Verify metrics
	metrics := stream.GetMetrics()
	if metrics == nil {
		t.Fatal("Metrics not available")
	}
	
	if metrics.TotalEvents != uint64(numEvents) {
		t.Errorf("Expected %d total events, got %d", numEvents, metrics.TotalEvents)
	}
	
	if metrics.EventsProcessed != uint64(numEvents) {
		t.Errorf("Expected %d processed events, got %d", numEvents, metrics.EventsProcessed)
	}
	
	if metrics.EventsCompressed == 0 {
		t.Error("Expected some events to be compressed")
	}
	
	if metrics.AverageLatency == 0 {
		t.Error("Expected non-zero average latency")
	}
	
	if metrics.FlowControl == nil {
		t.Error("Flow control metrics not available")
	}
	
	t.Logf("Stream metrics: TotalEvents=%d, EventsProcessed=%d, EventsCompressed=%d, AvgLatency=%v",
		metrics.TotalEvents, metrics.EventsProcessed, metrics.EventsCompressed, 
		time.Duration(metrics.AverageLatency))
}

// BenchmarkStreamSSEIntegration benchmarks the complete stream-to-SSE pipeline
func BenchmarkStreamSSEIntegration(b *testing.B) {
	config := DefaultStreamConfig()
	config.WorkerCount = 4
	config.BatchEnabled = true
	config.BatchSize = 50
	config.CompressionEnabled = true
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
	
	// Consumer goroutine to prevent blocking
	go func() {
		for chunk := range stream.ReceiveChunks() {
			// Simulate SSE formatting
			_, _ = FormatSSEChunk(chunk)
		}
	}()
	
	// Benchmark event sending
	event := events.NewTextMessageContentEvent("msg", "Benchmark test content")
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := stream.SendEvent(event)
			if err != nil {
				b.Fatalf("Failed to send event: %v", err)
			}
		}
	})
}