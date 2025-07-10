# AG-UI Go SDK Encoding Registry

The encoding registry provides a centralized system for managing data formats, encoders, and decoders in the AG-UI Go SDK. It supports dynamic format registration, capability-based selection, and plugin architecture.

## Features

- **Global Registry**: Centralized format management with thread-safe operations
- **Format Discovery**: Query supported formats and their capabilities
- **Priority-based Selection**: Automatic format selection based on requirements
- **Plugin Support**: Extensible architecture for custom formats
- **Factory Pattern**: Clean separation of codec creation logic
- **Format Aliases**: Support for multiple names per format (e.g., "json" → "application/json")
- **Capability Matching**: Select formats based on required features
- **Performance Profiles**: Track encoding/decoding performance characteristics
- **Validation Integration**: Optional format validation framework

## Usage

### Basic Usage

```go
// Get the global registry
registry := encoding.GetGlobalRegistry()

// Create an encoder
encoder, err := registry.GetEncoder("application/json", &encoding.EncodingOptions{
    Pretty: true,
})

// Create a decoder
decoder, err := registry.GetDecoder("application/json", nil)

// Create a codec (encoder + decoder)
codec, err := registry.GetCodec("application/json", nil, nil)
```

### Format Discovery

```go
// List all formats
formats := registry.ListFormats()
for _, format := range formats {
    fmt.Printf("%s: %s\n", format.Name, format.MIMEType)
}

// Check format support
if registry.SupportsFormat("application/json") {
    // JSON is supported
}

// Get format capabilities
caps, err := registry.GetCapabilities("application/x-protobuf")
if caps.BinaryEfficient {
    // Use for binary data
}
```

### Format Selection

```go
// Client accepts multiple formats
accepted := []string{"application/json", "application/x-protobuf"}

// Require human-readable format
required := &encoding.FormatCapabilities{
    HumanReadable: true,
}

// Registry selects best format
format, err := registry.SelectFormat(accepted, required)
// Returns: "application/json"
```

### Custom Format Registration

```go
// Define format info
info := encoding.NewFormatInfo("MessagePack", "application/x-msgpack")
info.Aliases = []string{"msgpack", "mp"}
info.FileExtensions = []string{".msgpack"}
info.Capabilities = encoding.BinaryFormatCapabilities()
info.Priority = 25 // Between JSON (10) and Protobuf (20)

// Register format
registry.RegisterFormat(info)

// Register codec factory
factory := &msgpackFactory{}
registry.RegisterCodec("application/x-msgpack", factory)
```

### Plugin Architecture

```go
// Create plugin-enabled factory
factory := encoding.NewPluginEncoderFactory()

// Register plugin
plugin := &MessagePackPlugin{}
factory.RegisterPlugin(plugin)

// Use in registry
registry.RegisterEncoder("application/x-msgpack", factory)
```

## Built-in Formats

### JSON
- **MIME Type**: `application/json`
- **Aliases**: `json`, `text/json`
- **Features**: Human-readable, self-describing, streaming
- **Priority**: 10 (highest)

### Protocol Buffers
- **MIME Type**: `application/x-protobuf`
- **Aliases**: `protobuf`, `proto`
- **Features**: Binary-efficient, schema-based, versionable
- **Priority**: 20

## Format Capabilities

```go
type FormatCapabilities struct {
    Streaming        bool     // Supports streaming encode/decode
    Compression      bool     // Supports compression
    SchemaValidation bool     // Has schema validation
    BinaryEfficient  bool     // Optimized for binary data
    HumanReadable    bool     // Human-readable format
    SelfDescribing   bool     // Self-describing data
    Versionable      bool     // Supports versioning
    // ... more capabilities
}
```

## Performance Profiles

```go
type PerformanceProfile struct {
    EncodingSpeed  float64  // Relative to JSON (1.0)
    DecodingSpeed  float64  // Relative to JSON (1.0)
    SizeEfficiency float64  // Size relative to JSON (1.0)
    MemoryUsage    float64  // Memory relative to JSON (1.0)
}
```

## Factory Pattern

The registry uses factories for codec creation:

```go
type CodecFactory interface {
    EncoderFactory
    DecoderFactory
}

type EncoderFactory interface {
    CreateEncoder(contentType string, options *EncodingOptions) (Encoder, error)
    CreateStreamEncoder(contentType string, options *EncodingOptions) (StreamEncoder, error)
    SupportedEncoders() []string
}
```

### Caching Factory

```go
// Wrap factory with caching
cached := encoding.NewCachingEncoderFactory(baseFactory)

// Subsequent calls with same options return cached instance
encoder1, _ := cached.CreateEncoder("application/json", opts)
encoder2, _ := cached.CreateEncoder("application/json", opts)
// encoder1 == encoder2
```

## Thread Safety

All registry operations are thread-safe:
- Format registration/unregistration
- Factory registration
- Encoder/decoder creation
- Format queries

## Integration with Validation

```go
// Set validation framework
validator := &CustomValidator{}
registry.SetValidator(validator)

// Validation is called automatically during encode/decode
```

## Future Formats

The registry is designed to support future formats:
- MessagePack (`application/x-msgpack`)
- CBOR (`application/cbor`)
- Apache Avro (`application/avro`)
- Custom binary formats

## Best Practices

1. **Use the Global Registry**: For most cases, use `GetGlobalRegistry()`
2. **Register Early**: Register custom formats during initialization
3. **Set Priorities**: Use priority values to control format selection
4. **Document Capabilities**: Clearly define what each format supports
5. **Benchmark Performance**: Provide accurate performance profiles
6. **Handle Aliases**: Register common aliases for user convenience

## Examples

See `example_registry_test.go` for complete examples:
- Basic registry usage
- Custom format registration
- Plugin implementation
- Format selection
- Performance profiling