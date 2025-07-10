# Content Negotiation System

The content negotiation system provides RFC 7231 compliant HTTP content negotiation for the AG-UI Go SDK. It intelligently selects the best content type based on client preferences, server capabilities, and performance characteristics.

## Features

- **RFC 7231 Compliant**: Full support for HTTP Accept headers with quality factors
- **Performance-Based Selection**: Adaptive selection based on real-time performance metrics
- **Wildcard Support**: Handles `*/*` and `type/*` patterns
- **Custom Types**: Easy registration of custom content types with capabilities
- **Thread-Safe**: All operations are safe for concurrent use
- **Extensible**: Plugin architecture for custom selection algorithms

## Architecture

```
pkg/encoding/negotiation/
├── negotiator.go      # Core content negotiation logic
├── parser.go          # Accept header and media type parsing
├── selector.go        # Advanced selection algorithms
├── performance.go     # Performance tracking and metrics
├── doc.go            # Package documentation
└── README.md         # This file
```

## Components

### ContentNegotiator

The main entry point for content negotiation:

```go
negotiator := negotiation.NewContentNegotiator("application/json")
contentType, err := negotiator.Negotiate("application/json;q=0.9, application/x-protobuf;q=1.0")
```

### Parser

RFC-compliant parsing of Accept headers and media types:

```go
acceptTypes, err := negotiation.ParseAcceptHeader("application/json;q=0.9, */*, text/html;q=0.8")
mediaType, params, err := negotiation.ParseMediaType("application/json;charset=utf-8")
```

### FormatSelector

Advanced selection with custom criteria:

```go
selector := negotiation.NewFormatSelector(negotiator)
criteria := &SelectionCriteria{
    RequireStreaming: true,
    PreferPerformance: true,
}
contentType, err := selector.SelectFormat(acceptHeader, criteria)
```

### PerformanceTracker

Tracks and analyzes performance metrics:

```go
negotiator.UpdatePerformance("application/json", PerformanceMetrics{
    EncodingTime: 10 * time.Millisecond,
    SuccessRate:  0.95,
})
```

## Usage Examples

### Basic Content Negotiation

```go
negotiator := negotiation.NewContentNegotiator("application/json")

// Simple negotiation
contentType, _ := negotiator.Negotiate("application/json")
// Returns: "application/json"

// With quality factors
contentType, _ := negotiator.Negotiate("application/json;q=0.8, application/x-protobuf")
// Returns: "application/x-protobuf" (higher default quality)
```

### Handling Complex Accept Headers

```go
// Multiple types with parameters
accept := "application/json;q=0.9, application/x-protobuf;q=1.0, */*, text/html;q=0.8"
contentType, _ := negotiator.Negotiate(accept)
// Returns: "application/x-protobuf" (highest quality)

// Wildcards
contentType, _ := negotiator.Negotiate("application/*, text/*;q=0.5")
// Returns best matching application/* type
```

### Performance-Based Selection

```go
// Update performance metrics
negotiator.UpdatePerformance("application/json", PerformanceMetrics{
    EncodingTime: 20 * time.Millisecond,
    SuccessRate:  0.90,
})

negotiator.UpdatePerformance("application/x-protobuf", PerformanceMetrics{
    EncodingTime: 5 * time.Millisecond,
    SuccessRate:  0.99,
})

// When quality is equal, performance decides
contentType, _ := negotiator.Negotiate("application/json;q=0.9, application/x-protobuf;q=0.9")
// Returns: "application/x-protobuf" (better performance)
```

### Custom Content Types

```go
// Register AG-UI specific format
negotiator.RegisterType(&TypeCapabilities{
    ContentType:        "application/vnd.ag-ui+protobuf",
    CanStream:          true,
    CompressionSupport: []string{"gzip", "snappy"},
    Priority:           0.95,
    Aliases:            []string{"application/x-ag-ui-pb"},
})

// Use custom type
contentType, _ := negotiator.Negotiate("application/vnd.ag-ui+protobuf")
```

### Adaptive Selection

```go
adaptive := negotiation.NewAdaptiveSelector(negotiator)

// Track request history
adaptive.UpdateHistory("application/json", true, 20*time.Millisecond)
adaptive.UpdateHistory("application/json", false, 100*time.Millisecond) // failure

// Adaptive selection considers history
contentType, _ := adaptive.SelectAdaptive("application/json, application/x-protobuf", nil)
// May return protobuf if JSON has high failure rate
```

## Selection Algorithm

The negotiator uses a multi-factor scoring system:

1. **Quality Factor** (from Accept header): Primary selection criterion
2. **Server Priority**: Configured priority for each content type
3. **Performance Score**: Based on:
   - Success rate (40% weight)
   - Speed (30% weight)
   - Payload size efficiency (20% weight)
   - Resource usage (10% weight)

## Performance Considerations

- **Caching**: Parser results are candidates for caching in high-traffic scenarios
- **Metrics Collection**: Use sampling for performance metrics in production
- **Memory Usage**: The performance tracker maintains a sliding window of recent samples

## Best Practices

1. **Set Appropriate Defaults**: Choose a default content type that works for most clients
2. **Register All Supported Types**: Include aliases for better compatibility
3. **Update Performance Metrics**: Regular updates improve selection quality
4. **Use Selection Criteria**: Leverage client capabilities for optimal results
5. **Monitor Adaptive Behavior**: Review selection patterns in production

## Thread Safety

All types are safe for concurrent use:
- `ContentNegotiator` uses read-write locks
- `PerformanceTracker` synchronizes metric updates
- `AdaptiveSelector` safely manages history

## Error Handling

Common errors:
- `ErrNoAcceptableType`: No supported type matches Accept header
- `ErrInvalidAcceptHeader`: Malformed Accept header
- `ErrNoSupportedTypes`: No content types registered

## Integration with AG-UI SDK

The negotiation system integrates seamlessly with the encoding package:

```go
// In HTTP handler
acceptHeader := req.Header.Get("Accept")
contentType, err := negotiator.Negotiate(acceptHeader)
if err != nil {
    // Fall back to default
    contentType = negotiator.PreferredType()
}

// Select appropriate encoder
encoder, err := factory.CreateEncoder(contentType, options)
```

## Testing

Comprehensive test coverage includes:
- RFC compliance tests
- Performance tracking accuracy
- Concurrent operation safety
- Edge cases (empty headers, wildcards, invalid formats)

Run tests:
```bash
go test ./pkg/encoding/negotiation/...
```

## Future Enhancements

- Content encoding negotiation (Accept-Encoding)
- Language negotiation (Accept-Language)
- Profile-based negotiation
- Machine learning-based selection