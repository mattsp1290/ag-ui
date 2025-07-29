# JSON Encoding for AG-UI Go SDK

This package provides JSON encoding and decoding functionality for AG-UI events, with support for both standard JSON and NDJSON (newline-delimited JSON) streaming formats.

## Features

- **Standard JSON encoding/decoding** for single and multiple events
- **NDJSON streaming support** for real-time event processing
- **Cross-SDK compatibility** matching TypeScript/Python JSON format
- **Thread-safe** implementations
- **Configurable options** for validation, formatting, and compatibility
- **Efficient buffering** for streaming operations

## Usage

### Basic JSON Encoding/Decoding

```go
import (
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"
)

// Create a codec with default options
codec := json.NewDefaultJSONCodec()

// Create an event
event := events.NewTextMessageStartEvent("msg123", events.WithRole("assistant"))

// Encode the event
data, err := codec.Encode(event)
if err != nil {
    log.Fatal(err)
}

// Decode the event
decodedEvent, err := codec.Decode(data)
if err != nil {
    log.Fatal(err)
}
```

### Streaming with NDJSON

```go
// Create a streaming codec
streamCodec := json.NewJSONStreamCodec(nil, nil)

// Get encoder and decoder
encoder := streamCodec.GetStreamEncoder()
decoder := streamCodec.GetStreamDecoder()

// Encoding stream
var buf bytes.Buffer
encoder.StartStream(&buf)
encoder.WriteEvent(event1)
encoder.WriteEvent(event2)
encoder.EndStream()

// Decoding stream
reader := bytes.NewReader(buf.Bytes())
decoder.StartStream(reader)
for {
    event, err := decoder.ReadEvent()
    if err == io.EOF {
        break
    }
    // Process event
}
decoder.EndStream()
```

### Pre-configured Codecs

```go
// Default codec with standard options
codec := json.DefaultCodec

// Pretty-printing codec
prettyCodec := json.PrettyCodec

// Cross-SDK compatibility codec
compatCodec := json.CompatibilityCodec
```

### Custom Options

```go
// Create codec with custom options
codec := json.NewJSONCodec(
    &encoding.EncodingOptions{
        Pretty:                true,
        CrossSDKCompatibility: true,
        ValidateOutput:        true,
        BufferSize:            8192,
    },
    &encoding.DecodingOptions{
        Strict:             false,
        ValidateEvents:     true,
        AllowUnknownFields: true,
        BufferSize:         8192,
    },
)
```

## JSON Format

The JSON format matches the AG-UI protocol specification and is compatible with TypeScript and Python SDKs:

```json
{
  "type": "TEXT_MESSAGE_START",
  "timestamp": 1234567890123,
  "messageId": "msg123",
  "role": "assistant"
}
```

For streaming (NDJSON), each event is on a separate line:

```
{"type":"TEXT_MESSAGE_START","timestamp":1234567890123,"messageId":"msg1"}
{"type":"TEXT_MESSAGE_CONTENT","timestamp":1234567890124,"messageId":"msg1","delta":"Hello"}
{"type":"TEXT_MESSAGE_END","timestamp":1234567890125,"messageId":"msg1"}
```

## Thread Safety

All encoder and decoder implementations are thread-safe. Multiple goroutines can safely use the same codec instance concurrently.

## Error Handling

The package uses structured error types:

- `EncodingError`: Provides details about encoding failures
- `DecodingError`: Provides details about decoding failures

Both error types include the problematic data and underlying cause when available.

## Performance Considerations

- Use streaming codecs for large volumes of events
- Configure buffer sizes based on your use case
- Disable validation for better performance in trusted environments
- Use NDJSON format for real-time streaming applications