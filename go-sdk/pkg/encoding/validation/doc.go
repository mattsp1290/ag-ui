// Package validation provides comprehensive validation and compatibility framework
// for the AG-UI Go SDK encoding system.
//
// This package includes:
//
// 1. Format Validation:
//    - FormatValidator interface for format-specific validation
//    - JSONValidator for JSON format validation
//    - ProtobufValidator for Protobuf format validation
//    - Schema validation support
//
// 2. Round-trip Validation:
//    - RoundTripValidator for encode->decode->compare validation
//    - Ensures data integrity through the encoding/decoding pipeline
//
// 3. Cross-SDK Compatibility:
//    - CrossSDKValidator for testing compatibility with TypeScript/Python SDKs
//    - Test vectors from other SDKs
//    - Format compatibility checks
//    - Version compatibility validation
//
// 4. Security Validation:
//    - SecurityValidator for input sanitization and attack prevention
//    - Injection attack prevention (XSS, SQL injection, etc.)
//    - Size limit enforcement
//    - Malformed data detection
//    - Resource exhaustion prevention
//
// 5. Performance Benchmarking:
//    - BenchmarkSuite for performance regression detection
//    - Throughput measurement
//    - Memory usage profiling
//    - Latency analysis
//
// 6. Test Vectors:
//    - Comprehensive test vectors for all event types
//    - Edge cases and corner cases
//    - Cross-SDK test data
//    - Malformed input examples
//    - Security test vectors
//
// Usage Examples:
//
//   // Format validation
//   validator := NewJSONValidator(true) // strict mode
//   err := validator.ValidateFormat(data)
//   err = validator.ValidateEvent(event)
//
//   // Round-trip validation
//   rtValidator := NewRoundTripValidator(encoder, decoder)
//   err := rtValidator.ValidateRoundTrip(ctx, event)
//
//   // Cross-SDK compatibility
//   crossValidator := NewCrossSDKValidator()
//   err := crossValidator.ValidateCompatibility(ctx, "typescript", decoder)
//
//   // Security validation
//   secValidator := NewSecurityValidator(DefaultSecurityConfig())
//   err := secValidator.ValidateInput(ctx, data)
//   err = secValidator.ValidateEvent(ctx, event)
//
//   // Performance benchmarking
//   config := DefaultBenchmarkConfig()
//   benchSuite := NewBenchmarkSuite(encoder, decoder, validator, config)
//   err := benchSuite.RunAllBenchmarks(ctx)
//   results := benchSuite.GetResults()
//
//   // Test vectors
//   registry := NewTestVectorRegistry()
//   vectors := registry.GetVectorsBySDK("typescript")
//   malformedVectors := registry.GetFailureVectors()
//
// Integration with Events Package:
//
// This validation framework integrates seamlessly with the existing validation
// in the pkg/core/events package. It provides additional layers of validation
// specifically for encoding/decoding operations while respecting the event
// validation configuration.
//
// Security Considerations:
//
// The security validation component provides protection against:
// - Cross-site scripting (XSS) attacks
// - SQL injection attempts
// - XML entity expansion attacks (billion laughs)
// - Buffer overflow attempts
// - Resource exhaustion attacks
// - Malformed UTF-8 data
// - Null byte injection
//
// Performance Monitoring:
//
// The benchmarking framework helps detect performance regressions by:
// - Measuring encoding/decoding throughput
// - Tracking memory allocation patterns
// - Monitoring latency distributions
// - Comparing against baseline performance metrics
//
// Cross-SDK Compatibility:
//
// The compatibility framework ensures interoperability by:
// - Testing against reference implementations from other SDKs
// - Validating format compatibility across different platforms
// - Supporting version compatibility checking
// - Providing comprehensive test vectors for validation
package validation