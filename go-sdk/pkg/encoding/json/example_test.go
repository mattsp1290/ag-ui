package json_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/encoding/json"
)

func ExampleJSONCodec() {
	// Create a codec with default options
	codec := json.NewDefaultJSONCodec()

	// Create an event
	event := events.NewTextMessageStartEvent("msg123", events.WithRole("assistant"))

	// Encode the event
	data, err := codec.Encode(context.Background(), event)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Encoded JSON: %s\n", string(data))

	// Decode the event
	decoded, err := codec.Decode(context.Background(), data)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Decoded event type: %s\n", decoded.Type())
}

func ExampleJSONCodec_pretty() {
	// Create a codec with pretty printing
	codec := json.NewJSONCodec(
		&encoding.EncodingOptions{
			Pretty:                true,
			CrossSDKCompatibility: true,
		},
		nil,
	)

	// Create multiple events
	events := []events.Event{
		events.NewTextMessageStartEvent("msg1", events.WithRole("user")),
		events.NewTextMessageContentEvent("msg1", "Hello, how are you?"),
		events.NewTextMessageEndEvent("msg1"),
	}

	// Encode multiple events
	data, err := codec.EncodeMultiple(context.Background(), events)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Pretty JSON:\n%s\n", string(data))
}

func ExampleStreamingJSONEncoder() {
	// Create a streaming encoder
	encoder := json.NewStreamingJSONEncoder(nil)

	// Buffer to write to
	var buf bytes.Buffer

	// Start streaming
	if err := encoder.StartStream(context.Background(), &buf); err != nil {
		log.Fatal(err)
	}
	defer encoder.EndStream(context.Background())

	// Write events one by one
	events := []events.Event{
		events.NewTextMessageStartEvent("msg1"),
		events.NewTextMessageContentEvent("msg1", "Streaming "),
		events.NewTextMessageContentEvent("msg1", "message "),
		events.NewTextMessageContentEvent("msg1", "content"),
		events.NewTextMessageEndEvent("msg1"),
	}

	for _, event := range events {
		if err := encoder.WriteEvent(context.Background(), event); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("NDJSON output:\n%s", buf.String())
}

func ExampleStreamingJSONDecoder() {
	// NDJSON input
	ndjson := `{"type":"TEXT_MESSAGE_START","messageId":"msg1","timestamp":1234567890}
{"type":"TEXT_MESSAGE_CONTENT","messageId":"msg1","delta":"Hello","timestamp":1234567891}
{"type":"TEXT_MESSAGE_END","messageId":"msg1","timestamp":1234567892}
`

	// Create a streaming decoder
	decoder := json.NewStreamingJSONDecoder(nil)

	// Start streaming from reader
	reader := bytes.NewReader([]byte(ndjson))
	if err := decoder.StartStream(context.Background(), reader); err != nil {
		log.Fatal(err)
	}
	defer decoder.EndStream(context.Background())

	// Read events one by one
	for {
		event, err := decoder.ReadEvent(context.Background())
		if err != nil {
			break // EOF or error
		}
		fmt.Printf("Read event: %s\n", event.Type())
	}
}

func ExampleJSONStreamCodec_concurrent() {
	// Create a stream codec
	streamCodec := json.NewJSONStreamCodec(
		json.StreamingCodecOptions().EncodingOptions,
		json.StreamingCodecOptions().DecodingOptions,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create channels for events
	inputChan := make(chan events.Event)
	outputChan := make(chan events.Event)

	// Buffer for encoded data
	var buf bytes.Buffer

	// Start encoder in a goroutine
	go func() {
		if err := streamCodec.GetStreamEncoder().EncodeStream(ctx, inputChan, &buf); err != nil {
			log.Printf("Encoding error: %v", err)
		}
	}()

	// Send some events
	go func() {
		defer close(inputChan)
		for i := 0; i < 3; i++ {
			inputChan <- events.NewTextMessageContentEvent("msg1", fmt.Sprintf("Message %d", i))
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Wait for encoding to complete
	time.Sleep(500 * time.Millisecond)

	// Now decode the stream
	go func() {
		reader := bytes.NewReader(buf.Bytes())
		if err := streamCodec.GetStreamDecoder().DecodeStream(ctx, reader, outputChan); err != nil {
			log.Printf("Decoding error: %v", err)
		}
	}()

	// Read decoded events
	for event := range outputChan {
		if msgEvent, ok := event.(*events.TextMessageContentEvent); ok {
			fmt.Printf("Decoded message: %s\n", msgEvent.Delta)
		}
	}
}