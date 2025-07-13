// Package encoding provides event encoding and decoding for the AG-UI protocol.
//
// This package handles the serialization and deserialization of AG-UI events
// for transmission over various transport layers. It supports multiple encoding
// formats including JSON and Protocol Buffers, with automatic format detection
// and conversion capabilities.
//
// The encoding system is designed to be extensible, allowing new formats to be
// added without breaking existing code. It also provides validation and
// schema enforcement for event data.
//
// # Format Registry
//
// The package includes a global format registry that manages all available
// encoders and decoders. The registry supports:
//   - Dynamic format registration
//   - Capability-based format selection
//   - Plugin architecture for custom formats
//   - Performance profiling
//   - Format aliases (e.g., "json" → "application/json")
//
// # Supported Formats
//
//   - JSON: Human-readable format for debugging and development
//   - Protocol Buffers: High-performance binary format for production
//   - MessagePack: Compact binary format (planned)
//   - CBOR: Concise Binary Object Representation (planned)
//
// # Basic Usage
//
//	import "github.com/ag-ui/go-sdk/pkg/encoding"
//
//	// Using the global registry
//	registry := encoding.GetGlobalRegistry()
//	
//	// Get an encoder by MIME type
//	encoder, err := registry.GetEncoder("application/json", nil)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Encode an event
//	data, err := encoder.Encode(event)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Get a decoder
//	decoder, err := registry.GetDecoder("application/json", nil)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Decode an event
//	event, err := decoder.Decode(data)
//	if err != nil {
//		log.Fatal(err)
//	}
//
// # Format Selection
//
//	// Select format based on capabilities
//	required := &encoding.FormatCapabilities{
//		BinaryEfficient: true,
//		SchemaValidation: true,
//	}
//	
//	format, err := registry.SelectFormat(
//		[]string{"application/json", "application/x-protobuf"},
//		required,
//	)
//	// Returns: "application/x-protobuf"
//
// # Custom Format Registration
//
//	// Register a custom format
//	info := encoding.NewFormatInfo("Custom", "application/x-custom")
//	info.Capabilities = encoding.BinaryFormatCapabilities()
//	registry.RegisterFormat(info)
//	
//	// Register codec factory
//	factory := &customCodecFactory{}
//	registry.RegisterCodec("application/x-custom", factory)
//
// For more information, see the REGISTRY_README.md file.
package encoding
