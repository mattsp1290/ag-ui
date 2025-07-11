package validation

import (
	"context"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/encoding/json"
	// "github.com/ag-ui/go-sdk/pkg/encoding/protobuf" // Disabled until protobuf package is fixed
)

// TestEncodingDecodingPipeline tests the entire encoding/decoding pipeline
func TestEncodingDecodingPipeline(t *testing.T) {
	ctx := context.Background()

	// Create encoders and decoders
	jsonEncoder := json.NewEncoder()
	jsonDecoder := json.NewDecoder()
	
	// Note: Protobuf encoder/decoder would be created similarly when available
	// protobufEncoder := protobuf.NewEncoder()
	// protobufDecoder := protobuf.NewDecoder()

	tests := []struct {
		name    string
		encoder encoding.Encoder
		decoder encoding.Decoder
		format  string
	}{
		{
			name:    "JSON",
			encoder: jsonEncoder,
			decoder: jsonDecoder,
			format:  "application/json",
		},
		// Add protobuf when available
		// {
		//     name:    "Protobuf",
		//     encoder: protobufEncoder,
		//     decoder: protobufDecoder,
		//     format:  "application/x-protobuf",
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test basic pipeline
			if err := testBasicPipeline(ctx, t, tt.encoder, tt.decoder); err != nil {
				t.Errorf("Basic pipeline test failed: %v", err)
			}

			// Test validation pipeline
			if err := testValidationPipeline(ctx, t, tt.encoder, tt.decoder, tt.format); err != nil {
				t.Errorf("Validation pipeline test failed: %v", err)
			}

			// Test security pipeline
			if err := testSecurityPipeline(ctx, t, tt.encoder, tt.decoder); err != nil {
				t.Errorf("Security pipeline test failed: %v", err)
			}

			// Test round-trip validation
			if err := testRoundTripValidation(ctx, t, tt.encoder, tt.decoder); err != nil {
				t.Errorf("Round-trip validation test failed: %v", err)
			}
		})
	}
}

// testBasicPipeline tests basic encoding/decoding functionality
func testBasicPipeline(ctx context.Context, t *testing.T, encoder encoding.Encoder, decoder encoding.Decoder) error {
	// Create test events
	testEvents := []events.Event{
		&events.RunStartedEvent{
			BaseEvent: &events.BaseEvent{
				EventType: events.EventTypeRunStarted,
				TimestampMs: int64Ptr(time.Now().Unix()),
			},
			RunID:    "test-run-123",
			ThreadID: "test-thread-456",
		},
		&events.TextMessageContentEvent{
			BaseEvent: &events.BaseEvent{
				EventType: events.EventTypeTextMessageContent,
				TimestampMs: int64Ptr(time.Now().Unix()),
			},
			MessageID: "test-msg-789",
			Delta:     "Hello, integration test!",
		},
	}

	for _, event := range testEvents {
		// Encode the event
		encoded, err := encoder.Encode(context.Background(), event)
		if err != nil {
			return err
		}

		// Decode the event
		decoded, err := decoder.Decode(context.Background(), encoded)
		if err != nil {
			return err
		}

		// Verify event type matches
		if event.Type() != decoded.Type() {
			t.Errorf("Event type mismatch: expected %s, got %s", event.Type(), decoded.Type())
		}
	}

	return nil
}

// testValidationPipeline tests the validation pipeline
func testValidationPipeline(ctx context.Context, t *testing.T, encoder encoding.Encoder, decoder encoding.Decoder, format string) error {
	// Create format validator
	var validator FormatValidator
	switch format {
	case "application/json":
		validator = NewJSONValidator(true)
	case "application/x-protobuf":
		validator = NewProtobufValidator(10 * 1024 * 1024)
	default:
		t.Skipf("No validator for format: %s", format)
		return nil
	}

	// Test event validation
	testEvent := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeRunStarted,
			TimestampMs: int64Ptr(time.Now().Unix()),
		},
		RunID:    "validation-test-123",
		ThreadID: "validation-test-456",
	}

	// Validate the event
	if err := validator.ValidateEvent(testEvent); err != nil {
		return err
	}

	// Encode and validate format
	encoded, err := encoder.Encode(context.Background(), testEvent)
	if err != nil {
		return err
	}

	if err := validator.ValidateFormat(encoded); err != nil {
		return err
	}

	return nil
}

// testSecurityPipeline tests the security validation pipeline
func testSecurityPipeline(ctx context.Context, t *testing.T, encoder encoding.Encoder, decoder encoding.Decoder) error {
	securityValidator := NewSecurityValidator(DefaultSecurityConfig())

	// Test with safe content
	safeEvent := &events.TextMessageContentEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeTextMessageContent,
			TimestampMs: int64Ptr(time.Now().Unix()),
		},
		MessageID: "security-test-safe",
		Delta:     "This is safe content",
	}

	// Encode safe event
	encoded, err := encoder.Encode(context.Background(), safeEvent)
	if err != nil {
		return err
	}

	// Validate input security
	if err := securityValidator.ValidateInput(ctx, encoded); err != nil {
		return err
	}

	// Validate event security
	if err := securityValidator.ValidateEvent(ctx, safeEvent); err != nil {
		return err
	}

	// Test with potentially dangerous content
	dangerousEvent := &events.TextMessageContentEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeTextMessageContent,
			TimestampMs: int64Ptr(time.Now().Unix()),
		},
		MessageID: "security-test-dangerous",
		Delta:     "<script>alert('xss')</script>",
	}

	// This should fail security validation
	if err := securityValidator.ValidateEvent(ctx, dangerousEvent); err == nil {
		t.Error("Expected security validation to fail for dangerous content")
	}

	return nil
}

// testRoundTripValidation tests round-trip validation
func testRoundTripValidation(ctx context.Context, t *testing.T, encoder encoding.Encoder, decoder encoding.Decoder) error {
	roundTripValidator := NewRoundTripValidator(encoder, decoder)

	testEvents := []events.Event{
		&events.RunStartedEvent{
			BaseEvent: &events.BaseEvent{
				EventType: events.EventTypeRunStarted,
				TimestampMs: int64Ptr(time.Now().Unix()),
			},
			RunID:    "roundtrip-test-123",
			ThreadID: "roundtrip-test-456",
		},
		&events.StateSnapshotEvent{
			BaseEvent: &events.BaseEvent{
				EventType: events.EventTypeStateSnapshot,
				TimestampMs: int64Ptr(time.Now().Unix()),
			},
			Snapshot: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
				"key3": []interface{}{"a", "b", "c"},
			},
		},
	}

	// Test single event round-trip
	for _, event := range testEvents {
		if err := roundTripValidator.ValidateRoundTrip(ctx, event); err != nil {
			return err
		}
	}

	// Test multiple events round-trip
	if err := roundTripValidator.ValidateRoundTripMultiple(ctx, testEvents); err != nil {
		return err
	}

	return nil
}

// TestCrossSDKCompatibility tests cross-SDK compatibility
func TestCrossSDKCompatibility(t *testing.T) {
	ctx := context.Background()

	// Create JSON decoder for testing
	jsonDecoder := json.NewDecoder()

	// Create cross-SDK validator
	crossValidator := NewCrossSDKValidator()

	// Test compatibility with TypeScript SDK
	if err := crossValidator.ValidateCompatibility(ctx, "typescript", jsonDecoder); err != nil {
		t.Errorf("TypeScript compatibility test failed: %v", err)
	}

	// Test compatibility with Python SDK
	if err := crossValidator.ValidateCompatibility(ctx, "python", jsonDecoder); err != nil {
		t.Errorf("Python compatibility test failed: %v", err)
	}

	// Test all SDKs
	results := crossValidator.ValidateAllSDKs(ctx, jsonDecoder)
	for sdk, err := range results {
		if err != nil {
			t.Errorf("Cross-SDK validation failed for %s: %v", sdk, err)
		}
	}
}

// TestStandardTestVectors tests all standard test vectors
func TestStandardTestVectors(t *testing.T) {
	// Create JSON decoder
	jsonDecoder := json.NewDecoder()

	// Create test vector registry
	registry := NewTestVectorRegistry()

	// Get all vector sets
	vectorSets := registry.GetAllVectorSets()

	for setName, vectorSet := range vectorSets {
		t.Run(setName, func(t *testing.T) {
			for _, vector := range vectorSet.Vectors {
				t.Run(vector.Name, func(t *testing.T) {
					// Decode the input
					decoded, err := jsonDecoder.Decode(context.Background(), vector.Input)

					if vector.ShouldFail {
						// This vector should fail
						if err == nil {
							t.Errorf("Expected vector %s to fail, but it succeeded", vector.Name)
						}
						return
					}

					// This vector should succeed
					if err != nil {
						t.Errorf("Vector %s failed unexpectedly: %v", vector.Name, err)
						return
					}

					// Compare with expected result if provided
					if vector.Expected != nil {
						if err := compareEvents(vector.Expected, decoded); err != nil {
							t.Errorf("Vector %s comparison failed: %v", vector.Name, err)
						}
					}
				})
			}
		})
	}
}

// TestSecurityValidation tests security validation with security test vectors
func TestSecurityValidation(t *testing.T) {
	ctx := context.Background()

	// Create security validator with strict config
	securityValidator := NewSecurityValidator(StrictSecurityConfig())

	// Test security vectors
	for _, vector := range SecurityTestVectors.Vectors {
		t.Run(vector.Name, func(t *testing.T) {
			err := securityValidator.ValidateInput(ctx, vector.Input)

			if vector.ShouldFail {
				if err == nil {
					t.Errorf("Expected security vector %s to fail, but it passed", vector.Name)
				}
			} else {
				if err != nil {
					t.Errorf("Security vector %s failed unexpectedly: %v", vector.Name, err)
				}
			}
		})
	}
}

// TestPerformanceBenchmark tests performance benchmarking
func TestPerformanceBenchmark(t *testing.T) {
	ctx := context.Background()

	// Create encoder and decoder
	jsonEncoder := json.NewEncoder()
	jsonDecoder := json.NewDecoder()
	jsonValidator := NewJSONValidator(true)

	// Create benchmark suite with minimal config for testing
	config := DefaultBenchmarkConfig()
	config.WarmupIterations = 10
	config.TestIterations = 100
	config.Duration = 5 * time.Second
	config.ThroughputDuration = 2 * time.Second

	benchmarkSuite := NewBenchmarkSuite(jsonEncoder, jsonDecoder, jsonValidator, config)

	// Run benchmarks
	if err := benchmarkSuite.RunAllBenchmarks(ctx); err != nil {
		t.Errorf("Performance benchmark failed: %v", err)
	}

	// Get results
	results := benchmarkSuite.GetResults()
	if len(results) == 0 {
		t.Error("No benchmark results generated")
	}

	// Verify results have reasonable values
	for _, result := range results {
		if result.Throughput <= 0 {
			t.Errorf("Invalid throughput for test %s: %f", result.TestName, result.Throughput)
		}
		if result.Latency <= 0 {
			t.Errorf("Invalid latency for test %s: %v", result.TestName, result.Latency)
		}
	}
}

// TestValidationIntegration tests integration with existing events validation
func TestValidationIntegration(t *testing.T) {
	ctx := context.Background()

	// Create a validator with custom configuration
	customConfig := &events.ValidationConfig{
		Level:                   events.ValidationStrict,
		SkipTimestampValidation: false,
		AllowEmptyIDs:           false,
	}
	eventsValidator := events.NewValidator(customConfig)

	// Create encoding validator
	jsonValidator := NewJSONValidator(true)

	// Test event that should pass both validations
	validEvent := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeRunStarted,
			TimestampMs: int64Ptr(time.Now().Unix()),
		},
		RunID:    "integration-test-123",
		ThreadID: "integration-test-456",
	}

	// Validate with events validator
	if err := eventsValidator.ValidateEvent(ctx, validEvent); err != nil {
		t.Errorf("Events validation failed: %v", err)
	}

	// Validate with encoding validator
	if err := jsonValidator.ValidateEvent(validEvent); err != nil {
		t.Errorf("Encoding validation failed: %v", err)
	}

	// Test event that should fail validation
	invalidEvent := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeRunStarted,
			TimestampMs: int64Ptr(-1), // Invalid negative timestamp
		},
		RunID:    "", // Empty required field
		ThreadID: "integration-test-456",
	}

	// This should fail events validation
	if err := eventsValidator.ValidateEvent(ctx, invalidEvent); err == nil {
		t.Error("Expected events validation to fail for invalid event")
	}
}

// TestFormatCompatibility tests format compatibility checking
func TestFormatCompatibility(t *testing.T) {
	checker := NewFormatCompatibilityChecker()

	tests := []struct {
		format1    string
		format2    string
		compatible bool
	}{
		{"application/json", "application/json", true},
		{"application/json", "text/json", true},
		{"application/json", "application/x-protobuf", false},
		{"application/x-protobuf", "application/protobuf", true},
		{"text/plain", "application/json", false},
	}

	for _, test := range tests {
		result := checker.AreFormatsCompatible(test.format1, test.format2)
		if result != test.compatible {
			t.Errorf("Format compatibility check failed for %s vs %s: expected %v, got %v",
				test.format1, test.format2, test.compatible, result)
		}
	}
}

// TestVersionCompatibility tests version compatibility validation
func TestVersionCompatibility(t *testing.T) {
	validator := NewVersionCompatibilityValidator()

	tests := []struct {
		component string
		version   string
		valid     bool
	}{
		{"encoding", "1.0.0", true},
		{"encoding", "1.5.0", true},
		{"encoding", "2.0.0", true},
		{"encoding", "3.0.0", false},
		{"events", "1.0.0", true},
		{"events", "2.0.0", true},
		{"events", "2.1.0", false},
		{"unknown", "1.0.0", false},
	}

	for _, test := range tests {
		err := validator.ValidateVersion(test.component, test.version)
		if test.valid && err != nil {
			t.Errorf("Expected version %s for component %s to be valid, but got error: %v",
				test.version, test.component, err)
		}
		if !test.valid && err == nil {
			t.Errorf("Expected version %s for component %s to be invalid, but validation passed",
				test.version, test.component)
		}
	}
}

// Benchmark tests for performance measurement

func BenchmarkJSONEncoding(b *testing.B) {
	encoder := json.NewEncoder()
	event := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeRunStarted,
			TimestampMs: int64Ptr(time.Now().Unix()),
		},
		RunID:    "benchmark-run-123",
		ThreadID: "benchmark-thread-456",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encoder.Encode(context.Background(), event)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONDecoding(b *testing.B) {
	encoder := json.NewEncoder()
	decoder := json.NewDecoder()
	event := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeRunStarted,
			TimestampMs: int64Ptr(time.Now().Unix()),
		},
		RunID:    "benchmark-run-123",
		ThreadID: "benchmark-thread-456",
	}

	encoded, err := encoder.Encode(context.Background(), event)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := decoder.Decode(context.Background(), encoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRoundTrip(b *testing.B) {
	encoder := json.NewEncoder()
	decoder := json.NewDecoder()
	validator := NewRoundTripValidator(encoder, decoder)
	ctx := context.Background()
	
	event := &events.RunStartedEvent{
		BaseEvent: &events.BaseEvent{
			EventType: events.EventTypeRunStarted,
			TimestampMs: int64Ptr(time.Now().Unix()),
		},
		RunID:    "benchmark-run-123",
		ThreadID: "benchmark-thread-456",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := validator.ValidateRoundTrip(ctx, event)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSecurityValidation(b *testing.B) {
	validator := NewSecurityValidator(DefaultSecurityConfig())
	ctx := context.Background()
	
	data := []byte(`{"eventType":"text.message.content","messageId":"msg-123","delta":"This is a test message for security validation"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := validator.ValidateInput(ctx, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}