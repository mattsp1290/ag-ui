# Protocol Buffer Encoding for AG-UI Go SDK

This package provides high-performance Protocol Buffer encoding and decoding for AG-UI events.

## Features

- **Binary Efficiency**: 3-5x smaller than JSON encoding
- **Performance**: 2-10x faster encoding/decoding compared to JSON
- **Streaming Support**: Efficient streaming with length-prefixed format
- **Type Safety**: Strongly typed protobuf messages
- **Schema Evolution**: Built-in versioning support

## Usage

### Basic Encoding/Decoding

```go
// Create encoder and decoder
encoder := protobuf.NewProtobufEncoder(nil)
decoder := protobuf.NewProtobufDecoder(nil)

// Encode an event
data, err := encoder.Encode(event)

// Decode an event
decoded, err := decoder.Decode(data)
```

### Batch Operations

```go
// Encode multiple events
events := []events.Event{event1, event2, event3}
data, err := encoder.EncodeMultiple(events)

// Decode multiple events
decoded, err := decoder.DecodeMultiple(data)
```

### Streaming

```go
// Streaming encoder
streamEncoder := protobuf.NewStreamingProtobufEncoder(nil)
err := streamEncoder.StartStream(writer)
err = streamEncoder.WriteEvent(event)
err = streamEncoder.EndStream()

// Streaming decoder
streamDecoder := protobuf.NewStreamingProtobufDecoder(nil)
err := streamDecoder.DecodeStream(ctx, reader, eventChannel)
```

## Binary Format

### Single Event
Standard protobuf binary encoding.

### Multiple Events
Length-prefixed format:
```
[4-byte count][4-byte length][event1][4-byte length][event2]...
```

### Streaming Format
Each event is length-prefixed:
```
[4-byte length][event1][4-byte length][event2]...
```

## Configuration Options

### Encoding Options
- `MaxSize`: Maximum encoded size limit
- `ValidateOutput`: Enable output validation
- `BufferSize`: Buffer size for streaming

### Decoding Options
- `MaxSize`: Maximum input size limit
- `ValidateEvents`: Enable event validation after decoding
- `BufferSize`: Buffer size for streaming
- `Strict`: Enable strict validation

## Performance Tips

1. **Reuse Encoders/Decoders**: Create once and reuse for multiple operations
2. **Use Streaming**: For large sequences of events, use streaming to reduce memory usage
3. **Batch Operations**: Use `EncodeMultiple` for better performance with multiple events
4. **Buffer Sizes**: Adjust buffer sizes based on your event sizes and throughput needs

## Error Handling

All methods return typed errors:
- `EncodingError`: Errors during encoding
- `DecodingError`: Errors during decoding
- Both implement `error` and `Unwrap()` for error chain inspection

## Schema Compatibility

This implementation uses the protobuf schemas defined in `pkg/proto/`. Any changes to the `.proto` files require regenerating the Go code:

```bash
protoc --go_out=. --go_opt=paths=source_relative events.proto types.proto patch.proto
```