// Package json provides JSON encoding and decoding for AG-UI events.
//
// This package implements the encoding interfaces defined in the parent encoding package,
// providing full support for JSON and NDJSON (newline-delimited JSON) formats.
//
// Features:
//   - Standard JSON encoding/decoding for single and multiple events
//   - NDJSON streaming support for real-time event processing
//   - Cross-SDK compatibility (matching TypeScript/Python format)
//   - Thread-safe implementations
//   - Configurable options for validation, formatting, and compatibility
//   - Efficient buffering for streaming operations
//
// Basic Usage:
//
//	// Create a codec with default options
//	codec := json.NewDefaultJSONCodec()
//
//	// Encode an event
//	data, err := codec.Encode(event)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Decode an event
//	decodedEvent, err := codec.Decode(data)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Streaming Usage:
//
//	// Create a streaming codec
//	streamCodec := json.NewJSONStreamCodec(
//	    json.StreamingCodecOptions().EncodingOptions,
//	    json.StreamingCodecOptions().DecodingOptions,
//	)
//
//	// Start streaming
//	err := streamCodec.StartStream(writer)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer streamCodec.EndStream()
//
//	// Write events to stream
//	for _, event := range events {
//	    if err := streamCodec.WriteEvent(event); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// Cross-SDK Compatibility:
//
// The JSON format produced by this package is designed to be compatible with
// TypeScript and Python SDKs. When CrossSDKCompatibility is enabled in the
// encoding options, the encoder uses the event's ToJSON() method to ensure
// consistent formatting across all SDKs.
//
// Thread Safety:
//
// All encoder and decoder implementations in this package are thread-safe.
// Multiple goroutines can safely use the same codec instance concurrently.
//
// Error Handling:
//
// This package uses structured error types (EncodingError and DecodingError)
// that provide detailed information about what went wrong, including the
// problematic data and the underlying cause when available.
package json