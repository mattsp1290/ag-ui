// Package protobuf provides Protocol Buffer encoding and decoding implementations
// for the AG-UI Go SDK event system.
//
// This package implements efficient binary serialization using Protocol Buffers,
// offering significant performance advantages over text-based formats like JSON.
// It supports both single event encoding/decoding and batch operations, as well
// as streaming capabilities for real-time event processing.
//
// # Features
//
//   - High-performance binary serialization
//   - Support for all AG-UI event types
//   - Streaming encode/decode with length-prefixed format
//   - Batch event processing
//   - Schema evolution support through protobuf versioning
//   - Configurable validation and size limits
//
// # Usage
//
// Basic encoding/decoding:
//
//	encoder := protobuf.NewProtobufEncoder(nil)
//	data, err := encoder.Encode(event)
//
//	decoder := protobuf.NewProtobufDecoder(nil)
//	event, err := decoder.Decode(data)
//
// Streaming:
//
//	streamEncoder := protobuf.NewStreamingProtobufEncoder(nil)
//	err := streamEncoder.StartStream(writer)
//	err = streamEncoder.WriteEvent(event)
//	err = streamEncoder.EndStream()
//
// # Binary Format
//
// Single events are encoded as standard protobuf messages. Multiple events use
// a length-prefixed format:
//
//	[4-byte count][4-byte length][event1][4-byte length][event2]...
//
// Streaming format uses length-prefixed messages for each event:
//
//	[4-byte length][event1][4-byte length][event2]...
//
// All length values are encoded as big-endian 32-bit unsigned integers.
//
// # Performance Considerations
//
//   - Binary format is typically 3-5x smaller than JSON
//   - Encoding/decoding is 2-10x faster than JSON
//   - Streaming reduces memory usage for large event sequences
//   - Length-prefixed format enables efficient stream processing
//
// # Compatibility
//
// This implementation uses the generated protobuf code from pkg/proto/generated
// and maintains compatibility with the protobuf schema defined in the .proto files.
// Schema evolution is supported through protobuf's built-in versioning mechanisms.
package protobuf