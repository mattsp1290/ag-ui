package validation

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"
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
		RunIDValue:    "run-example-123",
		ThreadIDValue: "thread-example-456",
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
	encoded, err := encoder.Encode(context.Background(), event)
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
	// ✓ Test vector registry loaded with 31 vectors
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
	// ✓ Dangerous data correctly rejected: [CRITICAL] SECURITY_VIOLATION: input matches blocked pattern (violation: blocked_pattern) (pattern: <script[^>]*>.*?</script>) (risk: high)
}

// ExampleBenchmarkSuite demonstrates performance benchmarking
func ExampleBenchmarkSuite() {
	ctx := context.Background()

	// Create encoder, decoder, and validator
	encoder := json.NewEncoder()
	decoder := json.NewDecoder()
	validator := NewJSONValidator(true)

	// Create benchmark configuration with short duration for examples
	config := DefaultBenchmarkConfig()
	config.WarmupIterations = 2
	config.TestIterations = 10
	config.Duration = 500 * time.Millisecond           // 500ms maximum duration
	config.ThroughputDuration = 200 * time.Millisecond // 200ms for throughput tests
	config.EnableThroughputTest = false                // Disable throughput tests that may be slow

	// Create benchmark suite
	benchmarkSuite := NewBenchmarkSuite(encoder, decoder, validator, config)

	// Suppress verbose benchmark output for the example
	oldStdout := os.Stdout
	os.Stdout, _ = os.Open(os.DevNull)
	defer func() { os.Stdout = oldStdout }()

	// Run benchmarks
	err := benchmarkSuite.RunAllBenchmarks(ctx)

	// Restore stdout before printing results
	os.Stdout = oldStdout

	if err != nil {
		// Just return silently to avoid cluttering example output
		return
	}

	// Get results
	results := benchmarkSuite.GetResults()

	// Show that benchmarks completed successfully
	if len(results) > 0 {
		fmt.Println("✓ Benchmark suite completed successfully")
		fmt.Printf("✓ Generated %d benchmark results\n", len(results))
		fmt.Println("✓ Results include encoding, decoding, validation, and round-trip tests")
	}

	// Output:
	// ✓ Benchmark suite completed successfully
	// ✓ Generated 52 benchmark results
	// ✓ Results include encoding, decoding, validation, and round-trip tests
}
