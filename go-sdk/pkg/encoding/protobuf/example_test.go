package protobuf_test

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/protobuf"
)

func ExampleProtobufEncoder() {
	// Create an encoder
	encoder := protobuf.NewProtobufEncoder(nil)

	// Create a sample event
	event := events.NewTextMessageStartEvent("msg-123", events.WithRole("assistant"))

	// Encode the event
	data, err := encoder.Encode(context.Background(), event)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Encoded %d bytes\n", len(data))
	// Output: Encoded 31 bytes
}

func ExampleProtobufDecoder() {
	// Create encoder and decoder
	encoder := protobuf.NewProtobufEncoder(nil)
	decoder := protobuf.NewProtobufDecoder(nil)

	// Create and encode an event
	event := events.NewToolCallStartEvent("call-456", "calculate")

	data, _ := encoder.Encode(context.Background(), event)

	// Decode the event
	decoded, err := decoder.Decode(context.Background(), data)
	if err != nil {
		log.Fatal(err)
	}

	if toolCall, ok := decoded.(*events.ToolCallStartEvent); ok {
		fmt.Printf("Tool: %s\n", toolCall.ToolCallName)
	}
	// Output: Tool: calculate
}

func ExampleStreamingProtobufEncoder() {
	// Create a streaming encoder
	encoder := protobuf.NewStreamingProtobufEncoder(nil)

	// Create a buffer to write to
	var buf bytes.Buffer

	// Initialize the stream
	if err := encoder.StartStream(context.Background(), &buf); err != nil {
		log.Fatal(err)
	}

	// Write multiple events
	events := []events.Event{
		events.NewRunStartedEvent("thread-123", "run-123"),
		events.NewStepStartedEvent("step-456"),
	}

	for _, event := range events {
		if err := encoder.WriteEvent(context.Background(), event); err != nil {
			log.Fatal(err)
		}
	}

	// End the stream
	if err := encoder.EndStream(context.Background()); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Streamed %d bytes\n", buf.Len())
}

func ExampleStreamingProtobufDecoder() {
	// Create encoder and decoder
	encoder := protobuf.NewStreamingProtobufEncoder(nil)
	decoder := protobuf.NewStreamingProtobufDecoder(nil)

	// Create a pipe
	var buf bytes.Buffer

	// Encode some events
	encoder.StartStream(context.Background(), &buf)
	encoder.WriteEvent(context.Background(), events.NewRunStartedEvent("thread-123", "run-123"))
	encoder.EndStream(context.Background())

	// Decode events from stream
	ctx := context.Background()
	eventChan := make(chan events.Event, 10)

	go func() {
		decoder.DecodeStream(ctx, &buf, eventChan)
		// Don't close eventChan here - DecodeStream closes it
	}()

	// Read decoded events
	for event := range eventChan {
		fmt.Printf("Event type: %s\n", event.Type())
	}
	// Output: Event type: RUN_STARTED
}