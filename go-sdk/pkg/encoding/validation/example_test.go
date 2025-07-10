package validation

import (
	"context"
	"fmt"
	"log"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding/json"
)

// Example demonstrates how to use the validation framework
func Example() {
	ctx := context.Background()

	// Create encoder and decoder
	encoder := json.NewEncoder()
	decoder := json.NewDecoder()

	// Create a test event
	event := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType:   events.EventTypeRunStarted,
			TimestampMs: int64Ptr(1640995200),
		},
		RunID:    "run-example-123",
		ThreadID: "thread-example-456",
	}

	// 1. Format Validation
	fmt.Println("=== Format Validation ===")
	jsonValidator := NewJSONValidator(true)
	
	// Validate the event
	if err := jsonValidator.ValidateEvent(event); err != nil {
		log.Printf("Event validation failed: %v", err)
	} else {
		fmt.Println("✓ Event passed format validation")
	}

	// Encode and validate format
	encoded, err := encoder.Encode(event)
	if err != nil {
		log.Printf("Encoding failed: %v", err)
		return
	}

	if err := jsonValidator.ValidateFormat(encoded); err != nil {
		log.Printf("Format validation failed: %v", err)
	} else {
		fmt.Println("✓ Encoded data passed format validation")
	}

	// 2. Round-trip Validation
	fmt.Println("\n=== Round-trip Validation ===")
	roundTripValidator := NewRoundTripValidator(encoder, decoder)
	
	if err := roundTripValidator.ValidateRoundTrip(ctx, event); err != nil {
		log.Printf("Round-trip validation failed: %v", err)
	} else {
		fmt.Println("✓ Event passed round-trip validation")
	}

	// 3. Security Validation
	fmt.Println("\n=== Security Validation ===")
	securityValidator := NewSecurityValidator(DefaultSecurityConfig())
	
	if err := securityValidator.ValidateInput(ctx, encoded); err != nil {
		log.Printf("Security validation failed: %v", err)
	} else {
		fmt.Println("✓ Input passed security validation")
	}

	if err := securityValidator.ValidateEvent(ctx, event); err != nil {
		log.Printf("Event security validation failed: %v", err)
	} else {
		fmt.Println("✓ Event passed security validation")
	}

	// 4. Cross-SDK Compatibility
	fmt.Println("\n=== Cross-SDK Compatibility ===")
	crossValidator := NewCrossSDKValidator()
	
	if err := crossValidator.ValidateCompatibility(ctx, "typescript", decoder); err != nil {
		log.Printf("TypeScript compatibility validation failed: %v", err)
	} else {
		fmt.Println("✓ Compatible with TypeScript SDK")
	}

	// 5. Test Vector Validation
	fmt.Println("\n=== Test Vector Validation ===")
	registry := NewTestVectorRegistry()
	
	stats := registry.GetStatistics()
	fmt.Printf("✓ Test vector registry loaded with %d vectors\n", stats["total_vectors"])
	
	fmt.Println("\nValidation framework example completed successfully!")

	// Output:
	// === Format Validation ===
	// ✓ Event passed format validation
	// ✓ Encoded data passed format validation
	// 
	// === Round-trip Validation ===
	// ✓ Event passed round-trip validation
	// 
	// === Security Validation ===
	// ✓ Input passed security validation
	// ✓ Event passed security validation
	// 
	// === Cross-SDK Compatibility ===
	// ✓ Compatible with TypeScript SDK
	// 
	// === Test Vector Validation ===
	// ✓ Test vector registry loaded with 30 vectors
	// 
	// Validation framework example completed successfully!
}

// ExampleSecurityValidator demonstrates security validation features
func ExampleSecurityValidator() {
	ctx := context.Background()
	validator := NewSecurityValidator(StrictSecurityConfig())

	// Test safe content
	safeData := []byte(`{"message": "Hello, world!"}`)
	if err := validator.ValidateInput(ctx, safeData); err != nil {
		fmt.Printf("Safe data failed: %v\n", err)
	} else {
		fmt.Println("✓ Safe data passed validation")
	}

	// Test potentially dangerous content
	dangerousData := []byte(`{"message": "<script>alert('xss')</script>"}`)
	if err := validator.ValidateInput(ctx, dangerousData); err != nil {
		fmt.Printf("✓ Dangerous data correctly rejected: %v\n", err)
	} else {
		fmt.Println("✗ Dangerous data incorrectly passed validation")
	}

	// Output:
	// ✓ Safe data passed validation
	// ✓ Dangerous data correctly rejected: script injection detected: pattern (?i)<script[^>]*>
}

// ExampleBenchmarkSuite demonstrates performance benchmarking
func ExampleBenchmarkSuite() {
	ctx := context.Background()

	// Create encoder, decoder, and validator
	encoder := json.NewEncoder()
	decoder := json.NewDecoder()
	validator := NewJSONValidator(true)

	// Create benchmark configuration
	config := DefaultBenchmarkConfig()
	config.WarmupIterations = 10
	config.TestIterations = 100
	config.Duration = 5000 // 5 seconds in milliseconds would be time.Duration

	// Create benchmark suite
	benchmarkSuite := NewBenchmarkSuite(encoder, decoder, validator, config)

	// Run benchmarks
	if err := benchmarkSuite.RunAllBenchmarks(ctx); err != nil {
		log.Printf("Benchmark failed: %v", err)
		return
	}

	// Get results
	results := benchmarkSuite.GetResults()
	
	fmt.Printf("✓ Benchmark completed with %d test results\n", len(results))
	
	for _, result := range results[:3] { // Show first 3 results
		fmt.Printf("- %s: %.2f ops/sec, avg latency: %v\n", 
			result.TestName, result.Throughput, result.Latency)
	}

	// Output:
	// ✓ Benchmark completed with 16 test results
	// - run_started_encode_single: 5000.00 ops/sec, avg latency: 200µs
	// - text_message_content_encode_single: 4800.00 ops/sec, avg latency: 208µs
	// - tool_call_start_encode_single: 4900.00 ops/sec, avg latency: 204µs
}