package encoding_test

import (
	"context"
	"fmt"
	"log"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
	_ "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"     // Register JSON codec
	_ "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/protobuf" // Register Protobuf codec
)

func ExampleFormatRegistry_integration() {
	// Get the global registry with all formats registered
	registry := encoding.GetGlobalRegistry()

	// Create a test event
	event := events.NewTextMessageStartEvent("msg-123", events.WithRole("assistant"))

	// Encode with JSON
	jsonEncoder, err := registry.GetEncoder(context.Background(), "application/json", &encoding.EncodingOptions{
		Pretty: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	jsonData, err := jsonEncoder.Encode(context.Background(), event)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("JSON encoded %d bytes\n", len(jsonData))

	// Encode with Protobuf
	protobufEncoder, err := registry.GetEncoder(context.Background(), "application/x-protobuf", nil)
	if err != nil {
		log.Fatal(err)
	}

	protobufData, err := protobufEncoder.Encode(context.Background(), event)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Protobuf encoded %d bytes\n", len(protobufData))

	// Compare sizes
	ratio := float64(len(protobufData)) / float64(len(jsonData))
	fmt.Printf("Protobuf is %.2fx smaller than JSON\n", 1.0/ratio)
}

func ExampleFormatRegistry_negotiation() {
	registry := encoding.GetGlobalRegistry()

	// Simulate HTTP Accept header negotiation
	acceptHeaders := [][]string{
		{"application/json", "application/x-protobuf"}, // Both formats
		{"application/x-protobuf", "application/json"}, // Prefer protobuf
		{"text/json", "application/octet-stream"},      // Uses JSON alias
		{"application/x-msgpack", "application/json"},  // Falls back to JSON
	}

	for _, accepts := range acceptHeaders {
		// Find the best format for efficiency
		format, err := registry.SelectFormat(accepts, &encoding.FormatCapabilities{
			BinaryEfficient: true,
		})

		if err != nil {
			// Fall back to first supported format
			for _, accept := range accepts {
				if registry.SupportsFormat(accept) {
					format = accept
					break
				}
			}
		}

		fmt.Printf("Accept: %v -> Selected: %s\n", accepts, format)
	}
}

func ExampleFormatRegistry_capabilities() {
	registry := encoding.GetGlobalRegistry()

	// Check what each format can do
	formats := []string{"application/json", "application/x-protobuf"}

	for _, format := range formats {
		info, err := registry.GetFormat(format)
		if err != nil {
			continue
		}

		caps := info.Capabilities
		fmt.Printf("\n%s capabilities:\n", info.Name)
		fmt.Printf("  Human readable: %v\n", caps.HumanReadable)
		fmt.Printf("  Binary efficient: %v\n", caps.BinaryEfficient)
		fmt.Printf("  Schema validation: %v\n", caps.SchemaValidation)
		fmt.Printf("  Streaming: %v\n", caps.Streaming)
		fmt.Printf("  Performance: %.1fx encoding, %.1fx size\n",
			info.Performance.EncodingSpeed,
			info.Performance.SizeEfficiency)
	}
}

func ExampleFormatRegistry_streaming() {
	registry := encoding.GetGlobalRegistry()

	// Check streaming support
	formats := registry.ListFormats()

	fmt.Println("Formats with streaming support:")
	for _, format := range formats {
		if format.Capabilities.Streaming {
			// Try to create a streaming encoder
			_, err := registry.GetStreamEncoder(context.Background(), format.MIMEType, nil)
			hasStreaming := err == nil

			fmt.Printf("  %s: %v\n", format.Name, hasStreaming)
		}
	}
}
