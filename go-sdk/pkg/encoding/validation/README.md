# Encoding Validation Framework

The validation package provides comprehensive validation and compatibility testing for the AG-UI Go SDK encoding system.

## Features

### 🔍 Format Validation
- **JSONValidator**: Validates JSON format with strict mode support
- **ProtobufValidator**: Validates Protobuf format with size limits
- **Schema validation**: Validates data against schemas
- **Round-trip validation**: Ensures encode→decode→compare integrity

### 🌐 Cross-SDK Compatibility  
- **CrossSDKValidator**: Tests compatibility with TypeScript/Python SDKs
- **Test vectors**: Reference implementations from other SDKs
- **Format compatibility**: Checks format compatibility across platforms
- **Version compatibility**: Validates version compatibility

### 🔒 Security Validation
- **Input sanitization**: Prevents injection attacks
- **XSS prevention**: Detects cross-site scripting attempts
- **SQL injection prevention**: Blocks SQL injection patterns
- **Size limit enforcement**: Prevents resource exhaustion
- **Malformed data detection**: Identifies corrupted data

### ⚡ Performance Benchmarking
- **BenchmarkSuite**: Performance regression detection
- **Throughput measurement**: Operations per second tracking
- **Memory profiling**: Memory usage analysis
- **Latency analysis**: Response time measurement

### 📋 Test Vectors
- **Standard test vectors**: All event types covered
- **Edge cases**: Corner cases and boundary conditions
- **Cross-SDK test data**: Compatibility test cases
- **Security test vectors**: Malicious input examples

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/ag-ui/go-sdk/pkg/core/events"
    "github.com/ag-ui/go-sdk/pkg/encoding/json"
    "github.com/ag-ui/go-sdk/pkg/encoding/validation"
)

func main() {
    ctx := context.Background()
    
    // Create encoder/decoder
    encoder := json.NewEncoder()
    decoder := json.NewDecoder()
    
    // Create event
    event := &events.RunStartedEvent{
        BaseEvent: &events.BaseEvent{
            EventType: events.EventTypeRunStarted,
            TimestampMs: int64Ptr(1640995200),
        },
        RunID: "run-123",
        ThreadID: "thread-456",
    }
    
    // 1. Format validation
    validator := validation.NewJSONValidator(true)
    if err := validator.ValidateEvent(event); err != nil {
        panic(err)
    }
    
    // 2. Round-trip validation
    rtValidator := validation.NewRoundTripValidator(encoder, decoder)
    if err := rtValidator.ValidateRoundTrip(ctx, event); err != nil {
        panic(err)
    }
    
    // 3. Security validation
    secValidator := validation.NewSecurityValidator(validation.DefaultSecurityConfig())
    if err := secValidator.ValidateEvent(ctx, event); err != nil {
        panic(err)
    }
    
    fmt.Println("✅ All validations passed!")
}
```

## Validation Types

### Format Validation

```go
// JSON validation
jsonValidator := validation.NewJSONValidator(true) // strict mode
err := jsonValidator.ValidateFormat(data)
err = jsonValidator.ValidateEvent(event)

// Protobuf validation  
protobufValidator := validation.NewProtobufValidator(10 * 1024 * 1024) // 10MB limit
err := protobufValidator.ValidateFormat(data)
```

### Security Validation

```go
// Create security validator
config := validation.StrictSecurityConfig()
secValidator := validation.NewSecurityValidator(config)

// Validate input
err := secValidator.ValidateInput(ctx, data)
err = secValidator.ValidateEvent(ctx, event)

// Sanitize input
sanitized, err := secValidator.SanitizeInput(data)
```

### Cross-SDK Compatibility

```go
// Test compatibility with other SDKs
crossValidator := validation.NewCrossSDKValidator()

// Test specific SDK
err := crossValidator.ValidateCompatibility(ctx, "typescript", decoder)

// Test all SDKs
results := crossValidator.ValidateAllSDKs(ctx, decoder)
```

### Performance Benchmarking

```go
// Create benchmark suite
config := validation.DefaultBenchmarkConfig()
config.TestIterations = 1000

benchSuite := validation.NewBenchmarkSuite(encoder, decoder, validator, config)

// Run benchmarks
err := benchSuite.RunAllBenchmarks(ctx)

// Get results
results := benchSuite.GetResults()
for _, result := range results {
    fmt.Printf("%s: %.2f ops/sec\n", result.TestName, result.Throughput)
}
```

### Test Vectors

```go
// Access test vector registry
registry := validation.NewTestVectorRegistry()

// Get vectors by SDK
vectors := registry.GetVectorsBySDK("typescript")

// Get failure test cases
failureVectors := registry.GetFailureVectors()

// Get statistics
stats := registry.GetStatistics()
fmt.Printf("Total vectors: %d\n", stats["total_vectors"])
```

## Security Features

The security validator protects against:

- **XSS Attacks**: `<script>alert('xss')</script>`
- **SQL Injection**: `'; DROP TABLE users; --`
- **JavaScript Protocol**: `javascript:alert('xss')`
- **Data URI HTML**: `data:text/html,<script>alert('xss')</script>`
- **XML Entity Expansion**: Billion laughs attacks
- **Null Byte Injection**: Binary data injection
- **Buffer Overflow**: Oversized payloads
- **Resource Exhaustion**: Memory/CPU limits

## Configuration

### Security Configuration

```go
// Default configuration
config := validation.DefaultSecurityConfig()

// Strict configuration  
config := validation.StrictSecurityConfig()

// Custom configuration
config := validation.SecurityConfig{
    MaxInputSize:     1 * 1024 * 1024,  // 1MB
    MaxStringLength:  64 * 1024,        // 64KB
    MaxNestingDepth:  20,
    AllowHTMLContent: false,
    EnableXSSPrevention: true,
}
```

### Benchmark Configuration

```go
config := validation.BenchmarkConfig{
    WarmupIterations:     100,
    TestIterations:       1000, 
    Duration:            30 * time.Second,
    ThroughputDuration:  10 * time.Second,
    TrackMemory:         true,
    EnableRegressionTest: true,
}
```

## Integration

The validation framework integrates with:

- **Events Package**: Uses existing event validation
- **Encoding System**: Works with all encoder/decoder implementations  
- **JSON Package**: Provides JSON-specific validation
- **Protobuf Package**: Provides Protobuf-specific validation
- **Testing Framework**: Automated test integration

## Error Handling

```go
// Validation errors provide detailed context
if err := validator.ValidateEvent(event); err != nil {
    switch e := err.(type) {
    case *encoding.ValidationError:
        fmt.Printf("Field: %s, Message: %s\n", e.Field, e.Message)
    case *encoding.EncodingError:
        fmt.Printf("Format: %s, Message: %s\n", e.Format, e.Message)
    default:
        fmt.Printf("Validation failed: %v\n", err)
    }
}
```

## Best Practices

1. **Use strict validation in production**
2. **Run cross-SDK compatibility tests regularly**  
3. **Monitor performance benchmarks for regressions**
4. **Enable security validation for all external input**
5. **Use test vectors for comprehensive testing**
6. **Configure appropriate resource limits**
7. **Sanitize input when dealing with untrusted data**

## Testing

Run the validation tests:

```bash
# Run all tests
go test ./pkg/encoding/validation -v

# Run specific tests
go test ./pkg/encoding/validation -run="TestJSON" -v

# Run benchmarks
go test ./pkg/encoding/validation -bench=. -v
```

## Contributing

When adding new validation features:

1. Add corresponding test vectors
2. Update security patterns if needed
3. Add performance benchmarks
4. Update cross-SDK compatibility tests
5. Document new configuration options