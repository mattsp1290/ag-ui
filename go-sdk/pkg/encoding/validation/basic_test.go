package validation

import (
	"context"
	"testing"
)

// TestJSONValidator tests the JSON validator functionality
func TestJSONValidator(t *testing.T) {
	validator := NewJSONValidator(true)

	// Test valid JSON
	validJSON := []byte(`{"key": "value", "number": 42}`)
	if err := validator.ValidateFormat(validJSON); err != nil {
		t.Errorf("Valid JSON failed validation: %v", err)
	}

	// Test invalid JSON
	invalidJSON := []byte(`{"key": "value", invalid}`)
	if err := validator.ValidateFormat(invalidJSON); err == nil {
		t.Error("Invalid JSON passed validation")
	}
}

// TestProtobufValidator tests the Protobuf validator functionality
func TestProtobufValidator(t *testing.T) {
	validator := NewProtobufValidator(1024 * 1024) // 1MB

	// Test valid protobuf-like data
	validData := []byte{0x08, 0x96, 0x01} // Simple varint
	if err := validator.ValidateFormat(validData); err != nil {
		t.Errorf("Valid protobuf data failed validation: %v", err)
	}

	// Test oversized data
	oversizedData := make([]byte, 2*1024*1024) // 2MB
	if err := validator.ValidateFormat(oversizedData); err == nil {
		t.Error("Oversized protobuf data passed validation")
	}
}

// TestSecurityValidator tests the security validator functionality
func TestSecurityValidator(t *testing.T) {
	ctx := context.Background()
	validator := NewSecurityValidator(DefaultSecurityConfig())

	// Test safe data
	safeData := []byte(`{"message": "Hello, world!"}`)
	if err := validator.ValidateInput(ctx, safeData); err != nil {
		t.Errorf("Safe data failed validation: %v", err)
	}

	// Test malicious data
	maliciousData := []byte(`{"message": "<script>alert('xss')</script>"}`)
	if err := validator.ValidateInput(ctx, maliciousData); err == nil {
		t.Error("Malicious data passed validation")
	}
}

// TestTestVectorRegistry tests the test vector registry functionality
func TestTestVectorRegistry(t *testing.T) {
	registry := NewTestVectorRegistry()

	// Test getting vector sets
	vectorSets := registry.GetAllVectorSets()
	if len(vectorSets) == 0 {
		t.Error("No vector sets found in registry")
	}

	// Test getting vectors by SDK
	goVectors := registry.GetVectorsBySDK("go")
	if len(goVectors) == 0 {
		t.Error("No Go vectors found")
	}

	// Test getting failure vectors
	failureVectors := registry.GetFailureVectors()
	if len(failureVectors) == 0 {
		t.Error("No failure vectors found")
	}

	// Test statistics
	stats := registry.GetStatistics()
	if stats["total_vectors"].(int) == 0 {
		t.Error("No vectors counted in statistics")
	}
}

// TestBasicFormatCompatibility tests format compatibility checking
func TestBasicFormatCompatibility(t *testing.T) {
	checker := NewFormatCompatibilityChecker()

	// Test compatible formats
	if !checker.AreFormatsCompatible("application/json", "application/json") {
		t.Error("Same formats should be compatible")
	}

	if !checker.AreFormatsCompatible("application/json", "text/json") {
		t.Error("JSON variants should be compatible")
	}

	// Test incompatible formats
	if checker.AreFormatsCompatible("application/json", "application/x-protobuf") {
		t.Error("JSON and Protobuf should not be compatible")
	}
}

// TestBasicVersionCompatibility tests version compatibility validation
func TestBasicVersionCompatibility(t *testing.T) {
	validator := NewVersionCompatibilityValidator()

	// Test valid versions
	if err := validator.ValidateVersion("encoding", "1.0.0"); err != nil {
		t.Errorf("Valid version failed validation: %v", err)
	}

	// Test invalid versions
	if err := validator.ValidateVersion("encoding", "3.0.0"); err == nil {
		t.Error("Invalid version passed validation")
	}

	// Test unknown component
	if err := validator.ValidateVersion("unknown", "1.0.0"); err == nil {
		t.Error("Unknown component passed validation")
	}
}