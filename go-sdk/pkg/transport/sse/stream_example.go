package sse

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// ExampleEventStreaming demonstrates how to use the EventStream for efficient SSE event streaming
func ExampleEventStreaming() {
	// Create a custom stream configuration
	config := &StreamConfig{
		EventBufferSize:     500,
		ChunkBufferSize:     100,
		MaxChunkSize:        32 * 1024, // 32KB chunks
		FlushInterval:       50 * time.Millisecond,
		BatchEnabled:        true,
		BatchSize:           25,
		BatchTimeout:        25 * time.Millisecond,
		MaxBatchSize:        250,
		CompressionEnabled:  true,
		CompressionType:     CompressionGzip,
		CompressionLevel:    6,
		MinCompressionSize:  512,
		MaxConcurrentEvents: 50,
		BackpressureTimeout: 3 * time.Second,
		DrainTimeout:        15 * time.Second,
		SequenceEnabled:     true,
		OrderingRequired:    false,
		OutOfOrderBuffer:    500,
		WorkerCount:         2,
		EnableMetrics:       true,
		MetricsInterval:     10 * time.Second,
	}

	// Create the event stream
	stream, err := NewEventStream(config)
	if err != nil {
		log.Fatalf("Failed to create event stream: %v", err)
	}

	// Start the stream
	if err := stream.Start(); err != nil {
		log.Fatalf("Failed to start event stream: %v", err)
	}
	defer stream.Close()

	// Start monitoring errors
	go func() {
		for err := range stream.GetErrorChannel() {
			log.Printf("Stream error: %v", err)
		}
	}()

	// Start processing chunks (this would typically send to SSE clients)
	go func() {
		for chunk := range stream.ReceiveChunks() {
			fmt.Printf("Received chunk: Type=%s, ID=%s, Size=%d bytes, Compressed=%t\n",
				chunk.EventType, chunk.EventID, len(chunk.Data), chunk.Compressed)

			// In a real implementation, you would write this to SSE clients
			sseData, err := FormatSSEChunk(chunk)
			if err != nil {
				log.Printf("Failed to format SSE chunk: %v", err)
				continue
			}

			fmt.Printf("SSE Data:\n%s\n", sseData)
		}
	}()

	// Send various types of events
	events := []events.Event{
		events.NewRunStartedEvent("thread-1", "run-1"),
		events.NewTextMessageStartEvent("msg-1"),
		events.NewTextMessageContentEvent("msg-1", "Hello, "),
		events.NewTextMessageContentEvent("msg-1", "world!"),
		events.NewTextMessageEndEvent("msg-1"),
		events.NewToolCallStartEvent("tool-1", "calculate"),
		events.NewToolCallArgsEvent("tool-1", `{"value": 42}`),
		events.NewToolCallEndEvent("tool-1"),
		events.NewRunFinishedEvent("thread-1", "run-1"),
	}

	// Send events with some delay to see batching in action
	for i, event := range events {
		if err := stream.SendEvent(event); err != nil {
			log.Printf("Failed to send event %d: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait a bit to see all processing complete
	time.Sleep(500 * time.Millisecond)

	// Print metrics
	if metrics := stream.GetMetrics(); metrics != nil {
		fmt.Printf("\nStream Metrics:\n")
		fmt.Printf("  Total Events: %d\n", metrics.TotalEvents)
		fmt.Printf("  Events Processed: %d\n", metrics.EventsProcessed)
		fmt.Printf("  Events Per Second: %.2f\n", metrics.EventsPerSecond)
		fmt.Printf("  Events Compressed: %d\n", metrics.EventsCompressed)
		fmt.Printf("  Compression Ratio: %.2f\n", metrics.CompressionRatio)
		fmt.Printf("  Average Latency: %v\n", time.Duration(metrics.AverageLatency))
		fmt.Printf("  Total Batches: %d\n", metrics.TotalBatches)
		fmt.Printf("  Average Batch Size: %.2f\n", metrics.AverageBatchSize)
		if metrics.FlowControl != nil {
			fmt.Printf("  Flow Control - Current Concurrent: %d\n", metrics.FlowControl.CurrentConcurrent)
			fmt.Printf("  Flow Control - Backpressure Events: %d\n", metrics.FlowControl.BackpressureEvents)
		}
	}
}

// ExampleCompressionComparison demonstrates compression effectiveness
func ExampleCompressionComparison() {
	fmt.Println("=== Compression Comparison ===")

	// Create large state snapshot for compression testing
	largeState := map[string]interface{}{
		"users": make([]map[string]interface{}, 100),
		"settings": map[string]interface{}{
			"theme":         "dark",
			"notifications": true,
			"language":      "en",
			"timezone":      "UTC",
		},
		"data": make([]string, 50),
	}

	// Fill with repetitive data (good for compression)
	for i := range largeState["users"].([]map[string]interface{}) {
		largeState["users"].([]map[string]interface{})[i] = map[string]interface{}{
			"id":     fmt.Sprintf("user-%d", i),
			"name":   fmt.Sprintf("User %d", i),
			"email":  fmt.Sprintf("user%d@example.com", i),
			"status": "active",
			"role":   "member",
			"settings": map[string]interface{}{
				"theme":         "dark",
				"notifications": true,
				"language":      "en",
			},
		}
	}

	for i := range largeState["data"].([]string) {
		largeState["data"].([]string)[i] = fmt.Sprintf("This is repetitive data item number %d with lots of repeated text", i)
	}

	stateEvent := events.NewStateSnapshotEvent(largeState)

	// Test different compression configurations
	compressionTypes := []CompressionType{CompressionNone, CompressionGzip, CompressionDeflate}
	compressionLevels := []int{1, 6, 9}

	for _, compType := range compressionTypes {
		if compType == CompressionNone {
			// Test without compression
			config := DefaultStreamConfig()
			config.CompressionEnabled = false
			config.EnableMetrics = false
			config.WorkerCount = 1
			config.BatchEnabled = false

			_, err := NewEventStream(config)
			if err != nil {
				log.Printf("Failed to create stream: %v", err)
				continue
			}

			data, err := stateEvent.ToJSON()
			if err != nil {
				log.Printf("Failed to serialize event: %v", err)
				continue
			}

			fmt.Printf("No Compression: %d bytes\n", len(data))
			continue
		}

		for _, level := range compressionLevels {
			config := DefaultStreamConfig()
			config.CompressionEnabled = true
			config.CompressionType = compType
			config.CompressionLevel = level
			config.MinCompressionSize = 0 // Compress everything for testing
			config.EnableMetrics = false
			config.WorkerCount = 1
			config.BatchEnabled = false

			stream, err := NewEventStream(config)
			if err != nil {
				log.Printf("Failed to create stream: %v", err)
				continue
			}

			data, err := stateEvent.ToJSON()
			if err != nil {
				log.Printf("Failed to serialize event: %v", err)
				continue
			}

			start := time.Now()
			compressedData, err := stream.compressData(data)
			compressionTime := time.Since(start)

			if err != nil {
				log.Printf("Compression failed: %v", err)
				continue
			}

			ratio := float64(len(compressedData)) / float64(len(data))
			savings := len(data) - len(compressedData)

			fmt.Printf("%s Level %d: %d -> %d bytes (%.1f%% ratio, %d bytes saved, %v compression time)\n",
				compType, level, len(data), len(compressedData), ratio*100, savings, compressionTime)
		}
	}
}

// ExampleChunking demonstrates how large events are chunked
func ExampleChunking() {
	fmt.Println("=== Event Chunking Example ===")

	config := DefaultStreamConfig()
	config.MaxChunkSize = 1024 // Small chunk size for demonstration
	config.CompressionEnabled = false
	config.BatchEnabled = false
	config.SequenceEnabled = false
	config.EnableMetrics = false
	config.WorkerCount = 1

	stream, err := NewEventStream(config)
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	if err := stream.Start(); err != nil {
		log.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()

	// Create large event that will require chunking
	largeContent := strings.Repeat("This is a large message that will be chunked. ", 100)
	largeEvent := events.NewTextMessageContentEvent("large-msg", largeContent)

	// Track chunks
	chunks := make([]*StreamChunk, 0)
	go func() {
		for chunk := range stream.ReceiveChunks() {
			chunks = append(chunks, chunk)
			fmt.Printf("Chunk %d/%d: %d bytes\n",
				chunk.ChunkIndex+1, chunk.TotalChunks, len(chunk.Data))
		}
	}()

	// Send the large event
	if err := stream.SendEvent(largeEvent); err != nil {
		log.Fatalf("Failed to send event: %v", err)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("Original event size: %d bytes\n", len(largeContent))
	fmt.Printf("Number of chunks: %d\n", len(chunks))

	// Verify chunks can be reassembled
	var reassembled []byte
	for _, chunk := range chunks {
		reassembled = append(reassembled, chunk.Data...)
	}

	fmt.Printf("Reassembled size: %d bytes\n", len(reassembled))
	fmt.Printf("Data integrity: %t\n", string(reassembled) == largeContent)
}

// ExampleBatching demonstrates event batching
func ExampleBatching() {
	fmt.Println("=== Event Batching Example ===")

	config := DefaultStreamConfig()
	config.BatchEnabled = true
	config.BatchSize = 3 // Small batch for demonstration
	config.BatchTimeout = 100 * time.Millisecond
	config.CompressionEnabled = false
	config.SequenceEnabled = false
	config.EnableMetrics = false
	config.WorkerCount = 1

	stream, err := NewEventStream(config)
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	if err := stream.Start(); err != nil {
		log.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()

	// Monitor batches
	go func() {
		for chunk := range stream.ReceiveChunks() {
			if chunk.EventType == "batch" {
				fmt.Printf("Received batch: ID=%s, Size=%d bytes\n",
					chunk.EventID, len(chunk.Data))
			} else {
				fmt.Printf("Received single event: Type=%s, Size=%d bytes\n",
					chunk.EventType, len(chunk.Data))
			}
		}
	}()

	// Send events that will be batched
	events := []events.Event{
		events.NewTextMessageStartEvent("msg-1"),
		events.NewTextMessageContentEvent("msg-1", "Hello"),
		events.NewTextMessageEndEvent("msg-1"),
		events.NewTextMessageStartEvent("msg-2"),
		events.NewTextMessageContentEvent("msg-2", "World"),
	}

	for i, event := range events {
		fmt.Printf("Sending event %d: %s\n", i+1, event.Type())
		if err := stream.SendEvent(event); err != nil {
			log.Printf("Failed to send event: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for batch timeout
	time.Sleep(200 * time.Millisecond)
	fmt.Println("Batching complete")
}

// ExampleFlowControl demonstrates backpressure handling
func ExampleFlowControl() {
	fmt.Println("=== Flow Control Example ===")

	config := DefaultStreamConfig()
	config.MaxConcurrentEvents = 2 // Very low for demonstration
	config.BackpressureTimeout = 100 * time.Millisecond
	config.BatchEnabled = false
	config.CompressionEnabled = false
	config.SequenceEnabled = false
	config.EnableMetrics = true
	config.WorkerCount = 1

	stream, err := NewEventStream(config)
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	if err := stream.Start(); err != nil {
		log.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Close()

	// Slow chunk consumer to create backpressure
	go func() {
		for chunk := range stream.ReceiveChunks() {
			fmt.Printf("Processing chunk: %s\n", chunk.EventType)
			time.Sleep(200 * time.Millisecond) // Slow processing
		}
	}()

	// Monitor errors
	go func() {
		for err := range stream.GetErrorChannel() {
			fmt.Printf("Flow control error: %v\n", err)
		}
	}()

	// Send events rapidly to trigger backpressure
	for i := 0; i < 10; i++ {
		event := events.NewTextMessageContentEvent(fmt.Sprintf("msg-%d", i), fmt.Sprintf("Content %d", i))

		fmt.Printf("Sending event %d...\n", i+1)
		if err := stream.SendEvent(event); err != nil {
			fmt.Printf("Event %d rejected due to backpressure: %v\n", i+1, err)
		} else {
			fmt.Printf("Event %d accepted\n", i+1)
		}
	}

	// Wait for processing to complete
	time.Sleep(2 * time.Second)

	// Show flow control metrics
	if metrics := stream.GetMetrics(); metrics != nil && metrics.FlowControl != nil {
		fmt.Printf("Flow Control Metrics:\n")
		fmt.Printf("  Events Processed: %d\n", metrics.FlowControl.EventsProcessed)
		fmt.Printf("  Events Dropped: %d\n", metrics.FlowControl.EventsDropped)
		fmt.Printf("  Backpressure Events: %d\n", metrics.FlowControl.BackpressureEvents)
	}
}
