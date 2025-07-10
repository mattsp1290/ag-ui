package streaming_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/encoding/json"
	"github.com/ag-ui/go-sdk/pkg/encoding/streaming"
)

// Example_basicStreaming demonstrates basic streaming usage
func Example_basicStreaming() {
	// Create base codec
	baseCodec := json.NewJSONStreamCodec(nil, nil)

	// Create unified streaming codec with default config
	config := streaming.DefaultUnifiedStreamConfig()
	unifiedCodec := streaming.NewUnifiedStreamCodec(baseCodec, config)

	// Create event channel
	eventChan := make(chan events.Event, 10)
	
	// Send some events
	go func() {
		defer close(eventChan)
		for i := 0; i < 5; i++ {
			event := &events.MessageEvent{
				BaseEvent: events.BaseEvent{
					ID:        fmt.Sprintf("msg-%d", i),
					Type:      "message",
					Timestamp: time.Now().Unix(),
				},
				Role:    "user",
				Content: fmt.Sprintf("Message %d", i),
			}
			eventChan <- event
		}
	}()

	// Stream encode
	var buf bytes.Buffer
	ctx := context.Background()
	err := unifiedCodec.StreamEncode(ctx, eventChan, &buf)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Streamed %d bytes\n", buf.Len())
	// Output: Streamed 525 bytes
}

// Example_chunkedStreaming demonstrates chunked streaming for large sequences
func Example_chunkedStreaming() {
	// Configure chunking
	config := streaming.DefaultUnifiedStreamConfig()
	config.EnableChunking = true
	config.ChunkConfig.MaxEventsPerChunk = 100
	config.ChunkConfig.MaxChunkSize = 10 * 1024 // 10KB

	// Create codec
	baseCodec := json.NewJSONStreamCodec(nil, nil)
	unifiedCodec := streaming.NewUnifiedStreamCodec(baseCodec, config)

	// Create large event stream
	eventChan := make(chan events.Event, 100)
	go func() {
		defer close(eventChan)
		for i := 0; i < 1000; i++ {
			event := &events.StateEvent{
				BaseEvent: events.BaseEvent{
					ID:        fmt.Sprintf("state-%d", i),
					Type:      "state",
					Timestamp: time.Now().Unix(),
				},
				Key:   fmt.Sprintf("key-%d", i),
				Value: fmt.Sprintf("value-%d", i),
			}
			eventChan <- event
		}
	}()

	// Stream with progress tracking
	processed := int64(0)
	unifiedCodec.RegisterProgressCallback(func(p, t int64) {
		processed = p
	})

	var buf bytes.Buffer
	ctx := context.Background()
	err := unifiedCodec.StreamEncode(ctx, eventChan, &buf)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Processed %d events in chunks\n", processed)
	// Output: Processed 1000 events in chunks
}

// Example_flowControl demonstrates flow control and backpressure
func Example_flowControl() {
	// Configure flow control
	config := streaming.DefaultUnifiedStreamConfig()
	config.EnableFlowControl = true
	config.StreamConfig.BackpressureThreshold = 50
	config.StreamConfig.OnBackpressure = func(pending int) {
		fmt.Printf("Backpressure triggered: %d pending\n", pending)
	}

	// Create codec
	baseCodec := json.NewJSONStreamCodec(nil, nil)
	unifiedCodec := streaming.NewUnifiedStreamCodec(baseCodec, config)

	// Create event stream
	eventChan := make(chan events.Event)
	go func() {
		defer close(eventChan)
		// Simulate fast producer
		for i := 0; i < 100; i++ {
			event := &events.RunEvent{
				BaseEvent: events.BaseEvent{
					ID:        fmt.Sprintf("run-%d", i),
					Type:      "run",
					Timestamp: time.Now().Unix(),
				},
				Status: "running",
			}
			eventChan <- event
		}
	}()

	// Slow writer to trigger backpressure
	slowWriter := &slowWriter{delay: 10 * time.Millisecond}
	ctx := context.Background()
	err := unifiedCodec.StreamEncode(ctx, eventChan, slowWriter)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Streaming completed with flow control")
	// Output:
	// Backpressure triggered: 50 pending
	// Streaming completed with flow control
}

// Example_metrics demonstrates metrics collection
func Example_metrics() {
	// Enable metrics
	config := streaming.DefaultUnifiedStreamConfig()
	config.EnableMetrics = true

	// Create codec
	baseCodec := json.NewJSONStreamCodec(nil, nil)
	unifiedCodec := streaming.NewUnifiedStreamCodec(baseCodec, config)

	// Create varied event stream
	eventChan := make(chan events.Event)
	go func() {
		defer close(eventChan)
		// Mix of event types
		for i := 0; i < 100; i++ {
			var event events.Event
			switch i % 3 {
			case 0:
				event = &events.MessageEvent{
					BaseEvent: events.BaseEvent{
						ID:        fmt.Sprintf("msg-%d", i),
						Type:      "message",
						Timestamp: time.Now().Unix(),
					},
					Role:    "assistant",
					Content: "Response message",
				}
			case 1:
				event = &events.ToolEvent{
					BaseEvent: events.BaseEvent{
						ID:        fmt.Sprintf("tool-%d", i),
						Type:      "tool",
						Timestamp: time.Now().Unix(),
					},
					Name:  "calculator",
					Input: map[string]interface{}{"a": 1, "b": 2},
				}
			case 2:
				event = &events.StateEvent{
					BaseEvent: events.BaseEvent{
						ID:        fmt.Sprintf("state-%d", i),
						Type:      "state",
						Timestamp: time.Now().Unix(),
					},
					Key:   "config",
					Value: "updated",
				}
			}
			eventChan <- event
		}
	}()

	// Stream encode
	var buf bytes.Buffer
	ctx := context.Background()
	err := unifiedCodec.StreamEncode(ctx, eventChan, &buf)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Get metrics
	metrics := unifiedCodec.GetMetrics().GetSnapshot()
	fmt.Printf("Processed %d events\n", metrics.EventsProcessed)
	fmt.Printf("Event types: %d\n", len(metrics.EventTypes))
	fmt.Printf("Total bytes: %d\n", metrics.BytesProcessed)
	
	// Output:
	// Processed 100 events
	// Event types: 3
	// Total bytes: 13400
}

// Example_streamManager demonstrates direct StreamManager usage
func Example_streamManager() {
	// Create encoders/decoders
	encOpts := &encoding.EncodingOptions{BufferSize: 4096}
	decOpts := &encoding.DecodingOptions{BufferSize: 4096}
	encoder := json.NewStreamingJSONEncoder(encOpts)
	decoder := json.NewStreamingJSONDecoder(decOpts)

	// Create stream manager
	config := streaming.DefaultStreamConfig()
	config.EnableMetrics = true
	streamMgr := streaming.NewStreamManager(encoder, decoder, config)

	// Start manager
	if err := streamMgr.Start(); err != nil {
		fmt.Printf("Error starting: %v\n", err)
		return
	}
	defer streamMgr.Stop()

	// Create bidirectional pipe for testing
	reader, writer := io.Pipe()

	// Write stream in background
	eventChan := make(chan events.Event)
	go func() {
		defer close(eventChan)
		defer writer.Close()
		for i := 0; i < 10; i++ {
			event := &events.RunEvent{
				BaseEvent: events.BaseEvent{
					ID:        fmt.Sprintf("run-%d", i),
					Type:      "run",
					Timestamp: time.Now().Unix(),
				},
				Status: "completed",
			}
			eventChan <- event
		}
	}()

	// Write in background
	ctx := context.Background()
	writeErr := make(chan error)
	go func() {
		writeErr <- streamMgr.WriteStream(ctx, eventChan, writer)
	}()

	// Read stream
	outputChan := make(chan events.Event, 10)
	readErr := make(chan error)
	go func() {
		readErr <- streamMgr.ReadStream(ctx, reader, outputChan)
	}()

	// Collect events
	count := 0
	for event := range outputChan {
		if event != nil {
			count++
		}
	}

	// Check errors
	if err := <-writeErr; err != nil {
		fmt.Printf("Write error: %v\n", err)
	}
	if err := <-readErr; err != nil {
		fmt.Printf("Read error: %v\n", err)
	}

	fmt.Printf("Transferred %d events through stream manager\n", count)
	// Output: Transferred 10 events through stream manager
}

// slowWriter simulates a slow writer for testing backpressure
type slowWriter struct {
	delay time.Duration
	buf   bytes.Buffer
}

func (w *slowWriter) Write(p []byte) (n int, err error) {
	time.Sleep(w.delay)
	return w.buf.Write(p)
}