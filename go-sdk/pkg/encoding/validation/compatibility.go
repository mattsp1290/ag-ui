package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/errors"
)

// CrossSDKValidator validates compatibility with other SDK implementations
type CrossSDKValidator struct {
	testVectors map[string][]TestVector
	validators  map[string]FormatValidator
}

// TestVector represents a test case for cross-SDK validation
type TestVector struct {
	Name        string
	Description string
	Format      string
	SDK         string // "typescript", "python", "go"
	Version     string
	Input       []byte
	Expected    events.Event
	ShouldFail  bool
	FailureMsg  string
}

// NewCrossSDKValidator creates a new cross-SDK validator
func NewCrossSDKValidator() *CrossSDKValidator {
	v := &CrossSDKValidator{
		testVectors: make(map[string][]TestVector),
		validators:  make(map[string]FormatValidator),
	}
	
	// Register default validators
	v.validators["application/json"] = NewJSONValidator(true)
	v.validators["application/x-protobuf"] = NewProtobufValidator(10 * 1024 * 1024)
	
	// Load default test vectors
	v.loadDefaultTestVectors()
	
	return v
}

// RegisterTestVectors registers test vectors for a specific SDK
func (v *CrossSDKValidator) RegisterTestVectors(sdk string, vectors []TestVector) {
	v.testVectors[sdk] = append(v.testVectors[sdk], vectors...)
}

// ValidateCompatibility validates compatibility with a specific SDK
func (v *CrossSDKValidator) ValidateCompatibility(ctx context.Context, sdk string, decoder encoding.Decoder) error {
	vectors, ok := v.testVectors[sdk]
	if !ok {
		return errors.NewValidationError("COMPATIBILITY_NO_TEST_VECTORS", fmt.Sprintf("no test vectors found for SDK: %s", sdk)).WithDetail("sdk", sdk)
	}

	var errs []error
	for _, vector := range vectors {
		if err := v.validateVector(ctx, vector, decoder); err != nil {
			errs = append(errs, fmt.Errorf("vector '%s' failed: %w", vector.Name, err))
		}
	}

	if len(errs) > 0 {
		return errors.NewValidationError("COMPATIBILITY_VALIDATION_FAILED", fmt.Sprintf("compatibility validation failed with %d errors", len(errs))).WithDetail("error_count", len(errs)).WithDetail("errors", errs)
	}

	return nil
}

// validateVector validates a single test vector
func (v *CrossSDKValidator) validateVector(ctx context.Context, vector TestVector, decoder encoding.Decoder) error {
	// Validate format
	if validator, ok := v.validators[vector.Format]; ok {
		if err := validator.ValidateFormat(vector.Input); err != nil {
			if !vector.ShouldFail {
				return errors.NewValidationError("COMPATIBILITY_FORMAT_VALIDATION_FAILED", "format validation failed unexpectedly").WithCause(err)
			}
			return nil // Expected failure
		}
	}

	// Decode the input
	decoded, err := decoder.Decode(context.Background(), vector.Input)
	if err != nil {
		if vector.ShouldFail {
			return nil // Expected failure
		}
		return errors.NewDecodingError("COMPATIBILITY_DECODING_FAILED", "decoding failed").WithCause(err)
	}

	if vector.ShouldFail {
		return errors.NewValidationError("COMPATIBILITY_UNEXPECTED_SUCCESS", "expected failure but decoding succeeded")
	}

	// Compare with expected event
	if vector.Expected != nil {
		if err := compareEvents(vector.Expected, decoded); err != nil {
			return errors.NewValidationError("COMPATIBILITY_EVENT_COMPARISON_FAILED", "event comparison failed").WithCause(err)
		}
	}

	return nil
}

// ValidateAllSDKs validates compatibility with all registered SDKs
func (v *CrossSDKValidator) ValidateAllSDKs(ctx context.Context, decoder encoding.Decoder) map[string]error {
	results := make(map[string]error)
	
	for sdk := range v.testVectors {
		results[sdk] = v.ValidateCompatibility(ctx, sdk, decoder)
	}
	
	return results
}

// loadDefaultTestVectors loads standard test vectors
func (v *CrossSDKValidator) loadDefaultTestVectors() {
	// TypeScript test vectors
	v.testVectors["typescript"] = []TestVector{
		{
			Name:        "typescript_run_started",
			Description: "RunStarted event from TypeScript SDK",
			Format:      "application/json",
			SDK:         "typescript",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"RUN_STARTED","timestamp":1234567890,"runId":"run-123","threadId":"thread-456"}`),
			Expected: &events.RunStartedEvent{
				BaseEvent: &events.BaseEvent{
					EventType:   events.EventTypeRunStarted,
					TimestampMs: int64Ptr(1234567890),
				},
				RunID:    "run-123",
				ThreadID: "thread-456",
			},
		},
		{
			Name:        "typescript_message_content",
			Description: "TextMessageContent event from TypeScript SDK",
			Format:      "application/json",
			SDK:         "typescript",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"TEXT_MESSAGE_CONTENT","timestamp":1234567890,"messageId":"msg-789","delta":"Hello, world!"}`),
			Expected: &events.TextMessageContentEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeTextMessageContent,
					TimestampMs: int64Ptr(1234567890),
				},
				MessageID: "msg-789",
				Delta:     "Hello, world!",
			},
		},
		{
			Name:        "typescript_malformed_json",
			Description: "Malformed JSON from TypeScript SDK",
			Format:      "application/json",
			SDK:         "typescript",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"RUN_STARTED", invalid json`),
			ShouldFail:  true,
			FailureMsg:  "Invalid JSON format",
		},
	}

	// Python test vectors
	v.testVectors["python"] = []TestVector{
		{
			Name:        "python_tool_call_start",
			Description: "ToolCallStart event from Python SDK",
			Format:      "application/json",
			SDK:         "python",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"TOOL_CALL_START","timestamp":1234567890,"toolCallId":"tool-abc","toolCallName":"calculator"}`),
			Expected: &events.ToolCallStartEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeToolCallStart,
					TimestampMs: int64Ptr(1234567890),
				},
				ToolCallID:   "tool-abc",
				ToolCallName: "calculator",
			},
		},
		{
			Name:        "python_state_snapshot",
			Description: "StateSnapshot event from Python SDK",
			Format:      "application/json",
			SDK:         "python",
			Version:     "1.0.0",
			Input:       []byte(`{"type":"STATE_SNAPSHOT","timestamp":1234567890,"snapshot":{"key":"value","count":42}}`),
			Expected: &events.StateSnapshotEvent{
				BaseEvent: &events.BaseEvent{
					EventType: events.EventTypeStateSnapshot,
					TimestampMs: int64Ptr(1234567890),
				},
				Snapshot: map[string]interface{}{
					"key":   "value",
					"count": float64(42), // JSON numbers decode as float64
				},
			},
		},
	}
}

// VersionCompatibilityValidator validates version compatibility
type VersionCompatibilityValidator struct {
	supportedVersions map[string]VersionRange
}

// VersionRange represents a range of compatible versions
type VersionRange struct {
	MinVersion string
	MaxVersion string
}

// NewVersionCompatibilityValidator creates a new version compatibility validator
func NewVersionCompatibilityValidator() *VersionCompatibilityValidator {
	return &VersionCompatibilityValidator{
		supportedVersions: map[string]VersionRange{
			"encoding": {MinVersion: "1.0.0", MaxVersion: "2.0.0"},
			"events":   {MinVersion: "1.0.0", MaxVersion: "2.0.0"},
			"protocol": {MinVersion: "1.0.0", MaxVersion: "1.5.0"},
		},
	}
}

// ValidateVersion validates a version against supported ranges
func (v *VersionCompatibilityValidator) ValidateVersion(component, version string) error {
	versionRange, ok := v.supportedVersions[component]
	if !ok {
		return errors.NewValidationError("COMPATIBILITY_UNKNOWN_COMPONENT", fmt.Sprintf("unknown component: %s", component)).WithDetail("component", component)
	}

	if !isVersionInRange(version, versionRange.MinVersion, versionRange.MaxVersion) {
		return errors.NewValidationError("COMPATIBILITY_VERSION_NOT_SUPPORTED", fmt.Sprintf("version %s is not in supported range [%s, %s] for component %s", version, versionRange.MinVersion, versionRange.MaxVersion, component)).
			WithDetail("version", version).
			WithDetail("min_version", versionRange.MinVersion).
			WithDetail("max_version", versionRange.MaxVersion).
			WithDetail("component", component)
	}

	return nil
}

// FormatCompatibilityChecker checks format compatibility
type FormatCompatibilityChecker struct {
	formatMap map[string][]string // format -> compatible formats
}

// NewFormatCompatibilityChecker creates a new format compatibility checker
func NewFormatCompatibilityChecker() *FormatCompatibilityChecker {
	return &FormatCompatibilityChecker{
		formatMap: map[string][]string{
			"application/json": {
				"application/json",
				"text/json",
				"application/json; charset=utf-8",
			},
			"application/x-protobuf": {
				"application/x-protobuf",
				"application/protobuf",
				"application/vnd.google.protobuf",
			},
		},
	}
}

// AreFormatsCompatible checks if two formats are compatible
func (c *FormatCompatibilityChecker) AreFormatsCompatible(format1, format2 string) bool {
	// Normalize formats
	format1 = normalizeContentType(format1)
	format2 = normalizeContentType(format2)

	// Check direct equality
	if format1 == format2 {
		return true
	}

	// Check compatibility map
	if compatibleFormats, ok := c.formatMap[format1]; ok {
		for _, compatible := range compatibleFormats {
			if normalizeContentType(compatible) == format2 {
				return true
			}
		}
	}

	// Check reverse compatibility
	if compatibleFormats, ok := c.formatMap[format2]; ok {
		for _, compatible := range compatibleFormats {
			if normalizeContentType(compatible) == format1 {
				return true
			}
		}
	}

	return false
}

// Helper functions

func int64Ptr(v int64) *int64 {
	return &v
}

func normalizeContentType(contentType string) string {
	// Remove parameters and normalize
	parts := strings.Split(contentType, ";")
	return strings.TrimSpace(strings.ToLower(parts[0]))
}

func isVersionInRange(version, minVersion, maxVersion string) bool {
	// Simple version comparison - in production, use a proper semver library
	return version >= minVersion && version <= maxVersion
}

// TestVectorLoader loads test vectors from external sources
type TestVectorLoader struct {
	sources map[string]TestVectorSource
}

// TestVectorSource defines an interface for loading test vectors
type TestVectorSource interface {
	LoadVectors(ctx context.Context) ([]TestVector, error)
}

// JSONTestVectorSource loads test vectors from JSON
type JSONTestVectorSource struct {
	data []byte
}

// LoadVectors loads test vectors from JSON data
func (s *JSONTestVectorSource) LoadVectors(ctx context.Context) ([]TestVector, error) {
	var vectors []TestVector
	if err := json.Unmarshal(s.data, &vectors); err != nil {
		return nil, errors.NewValidationError("COMPATIBILITY_UNMARSHAL_FAILED", "failed to unmarshal test vectors").WithCause(err)
	}
	return vectors, nil
}

// CrossSDKTestSuite provides a comprehensive test suite for cross-SDK compatibility
type CrossSDKTestSuite struct {
	validator *CrossSDKValidator
	encoder   encoding.Codec
	decoder   encoding.Codec
}

// NewCrossSDKTestSuite creates a new cross-SDK test suite
func NewCrossSDKTestSuite(codec encoding.Codec) *CrossSDKTestSuite {
	return &CrossSDKTestSuite{
		validator: NewCrossSDKValidator(),
		encoder:   codec,
		decoder:   codec,
	}
}

// RunCompatibilityTests runs all compatibility tests
func (s *CrossSDKTestSuite) RunCompatibilityTests(ctx context.Context) error {
	// Test decoding compatibility with all SDKs
	results := s.validator.ValidateAllSDKs(ctx, s.decoder)
	
	var failures []string
	for sdk, err := range results {
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", sdk, err))
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("compatibility tests failed:\n%s", strings.Join(failures, "\n"))
	}

	// Test encoding compatibility
	testEvents := s.generateTestEvents()
	for _, event := range testEvents {
		if err := s.testEncodingCompatibility(ctx, event); err != nil {
			return fmt.Errorf("encoding compatibility test failed: %w", err)
		}
	}

	return nil
}

// generateTestEvents generates a set of test events
func (s *CrossSDKTestSuite) generateTestEvents() []events.Event {
	return []events.Event{
		&events.RunStartedEvent{
			BaseEvent: &events.BaseEvent{
				EventType: events.EventTypeRunStarted,
				TimestampMs: int64Ptr(1234567890),
			},
			RunID:    "run-test-123",
			ThreadID: "thread-test-456",
		},
		&events.TextMessageContentEvent{
			BaseEvent: &events.BaseEvent{
				EventType: events.EventTypeTextMessageContent,
				TimestampMs: int64Ptr(1234567890),
			},
			MessageID: "msg-test-789",
			Delta:     "Test message content",
		},
		&events.ToolCallStartEvent{
			BaseEvent: &events.BaseEvent{
				EventType: events.EventTypeToolCallStart,
				TimestampMs: int64Ptr(1234567890),
			},
			ToolCallID:   "tool-test-abc",
			ToolCallName: "test_tool",
		},
	}
}

// testEncodingCompatibility tests encoding compatibility
func (s *CrossSDKTestSuite) testEncodingCompatibility(ctx context.Context, event events.Event) error {
	// Encode the event
	encoded, err := s.encoder.Encode(context.Background(), event)
	if err != nil {
		return fmt.Errorf("encoding failed: %w", err)
	}

	// Validate the encoded format
	format := s.encoder.ContentType()
	if validator, ok := s.validator.validators[format]; ok {
		if err := validator.ValidateFormat(encoded); err != nil {
			return fmt.Errorf("encoded data validation failed: %w", err)
		}
	}

	// Test round-trip
	decoded, err := s.decoder.Decode(context.Background(), encoded)
	if err != nil {
		return errors.NewDecodingError("COMPATIBILITY_DECODING_FAILED", "decoding failed").WithCause(err)
	}

	// Compare events
	if err := compareEvents(event, decoded); err != nil {
		return fmt.Errorf("round-trip comparison failed: %w", err)
	}

	return nil
}