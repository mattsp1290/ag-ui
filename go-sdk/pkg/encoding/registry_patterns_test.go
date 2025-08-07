package encoding_test

import (
	"testing"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"
	_ "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json" // Register JSON codec
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/protobuf"
	_ "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/protobuf" // Register Protobuf codec
)

// TestRegistryInitializationPatterns demonstrates proper patterns for registry initialization
func TestRegistryInitializationPatterns(t *testing.T) {
	// Pattern 1: Use explicit registration with error checking
	if err := json.EnsureRegistered(); err != nil {
		t.Fatalf("Failed to ensure JSON codec is registered: %v", err)
	}

	if err := protobuf.EnsureRegistered(); err != nil {
		t.Fatalf("Failed to ensure Protobuf codec is registered: %v", err)
	}

	// Pattern 2: Use registry's EnsureRegistered for multiple formats at once
	registry := encoding.GetGlobalRegistry()
	if err := registry.EnsureRegistered("application/json", "application/x-protobuf"); err != nil {
		t.Fatalf("Required codecs not available: %v", err)
	}

	// Verify registration worked
	if !registry.SupportsEncoding("application/json") {
		t.Error("JSON encoding should be supported")
	}

	if !registry.SupportsEncoding("application/x-protobuf") {
		t.Error("Protobuf encoding should be supported")
	}
}

// TestCustomRegistryPattern shows how to test with a custom registry
func TestCustomRegistryPattern(t *testing.T) {
	// Create a custom registry for isolated testing
	registry := encoding.NewFormatRegistry()

	// Register only what we need for this test
	if err := json.RegisterTo(registry); err != nil {
		t.Fatalf("Failed to register JSON to custom registry: %v", err)
	}

	// Verify registration worked
	if err := registry.EnsureRegistered("application/json"); err != nil {
		t.Fatalf("JSON not properly registered: %v", err)
	}

	// Should not support Protobuf
	if registry.SupportsEncoding("application/x-protobuf") {
		t.Error("Should not support Protobuf without registration")
	}

	// Should support JSON
	if !registry.SupportsEncoding("application/json") {
		t.Error("Should support JSON after registration")
	}
}

// TestErrorHandlingPattern shows proper error handling for missing codecs
func TestErrorHandlingPattern(t *testing.T) {
	// Create registry with only format info, no codecs
	registry := encoding.NewFormatRegistry()
	registry.RegisterDefaults()

	// This should fail with helpful error message
	err := registry.EnsureRegistered("application/json")
	if err == nil {
		t.Error("Expected error for missing codec")
		return
	}

	t.Logf("Got expected error: %v", err)

	// Error should suggest import
	errorMsg := err.Error()
	if !containsAny(errorMsg, "import", "codec") {
		t.Errorf("Error should mention import or codec: %s", errorMsg)
	}
}

// TestConcurrentRegistration verifies thread-safe registration
func TestConcurrentRegistration(t *testing.T) {
	const numGoroutines = 10

	done := make(chan error, numGoroutines)

	// Multiple goroutines trying to register simultaneously
	for i := 0; i < numGoroutines; i++ {
		go func() {
			// This should be safe to call concurrently
			err := json.EnsureRegistered()
			done <- err
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent registration failed: %v", err)
		}
	}

	// Verify final state
	registry := encoding.GetGlobalRegistry()
	if !registry.SupportsEncoding("application/json") {
		t.Error("JSON should be registered after concurrent attempts")
	}
}

// TestGlobalRegistryInitialization verifies the global registry starts correctly
func TestGlobalRegistryInitialization(t *testing.T) {
	// Get the global registry (triggers initialization)
	registry := encoding.GetGlobalRegistry()

	// Should have format info registered
	formats := registry.ListFormats()
	if len(formats) < 2 {
		t.Errorf("Expected at least 2 formats, got %d", len(formats))
	}

	// Check for expected formats
	hasJSON := false
	hasProtobuf := false

	for _, format := range formats {
		switch format.MIMEType {
		case "application/json":
			hasJSON = true
		case "application/x-protobuf":
			hasProtobuf = true
		}
	}

	if !hasJSON {
		t.Error("Expected JSON format to be registered")
	}
	if !hasProtobuf {
		t.Error("Expected Protobuf format to be registered")
	}

	// Check for any initialization errors
	errors := encoding.GetGlobalRegistrationErrors()
	if len(errors) > 0 {
		t.Logf("Global registration errors (may be expected): %v", errors)
	}
}

// Helper function
func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
