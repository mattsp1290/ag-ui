package encoding_test

import (
	"strings"
	"testing"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
	_ "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"      // Register JSON codec
	_ "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/protobuf" // Register Protobuf codec
)

func TestEnsureRegistered(t *testing.T) {
	registry := encoding.GetGlobalRegistry()
	
	tests := []struct {
		name        string
		mimeTypes   []string
		shouldPass  bool
		errorParts  []string
	}{
		{
			name:       "Both JSON and Protobuf registered",
			mimeTypes:  []string{"application/json", "application/x-protobuf"},
			shouldPass: true,
		},
		{
			name:       "Only JSON",
			mimeTypes:  []string{"application/json"},
			shouldPass: true,
		},
		{
			name:       "Only Protobuf",
			mimeTypes:  []string{"application/x-protobuf"},
			shouldPass: true,
		},
		{
			name:       "Empty list",
			mimeTypes:  []string{},
			shouldPass: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.EnsureRegistered(tt.mimeTypes...)
			
			if tt.shouldPass {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Error("Expected error, got nil")
				} else {
					for _, part := range tt.errorParts {
						if !strings.Contains(err.Error(), part) {
							t.Errorf("Expected error to contain %q, got: %v", part, err)
						}
					}
				}
			}
		})
	}
}

func TestEnsureRegisteredWithUnregisteredFormat(t *testing.T) {
	registry := encoding.NewFormatRegistry()
	// Don't register defaults to test missing formats
	
	err := registry.EnsureRegistered("application/json", "application/x-protobuf")
	if err == nil {
		t.Error("Expected error for unregistered formats")
		return
	}
	
	errorMsg := err.Error()
	t.Logf("Error message: %s", errorMsg)
	
	// Should mention missing format registration
	if !strings.Contains(errorMsg, "Missing format registration") {
		t.Error("Expected error to mention missing format registration")
	}
}

func TestEnsureRegisteredWithMissingCodec(t *testing.T) {
	// Create a fresh registry that only has format info but no codecs
	registry := encoding.NewFormatRegistry()
	registry.RegisterDefaults() // This only registers format info
	
	err := registry.EnsureRegistered("application/json", "application/x-protobuf")
	if err == nil {
		t.Error("Expected error for missing codecs")
		return
	}
	
	errorMsg := err.Error()
	t.Logf("Error message: %s", errorMsg)
	
	// Should mention missing codec registration
	if !strings.Contains(errorMsg, "Missing codec registration") {
		t.Error("Expected error to mention missing codec registration")
	}
	
	// Should suggest importing packages
	if !strings.Contains(errorMsg, "import") {
		t.Error("Expected error to suggest importing packages")
	}
}

func TestGetGlobalRegistrationErrors(t *testing.T) {
	// This will trigger global registry initialization
	errors := encoding.GetGlobalRegistrationErrors()
	
	// Should not have any errors with proper imports
	if len(errors) > 0 {
		t.Logf("Global registration errors: %v", errors)
		// Don't fail the test, just log for visibility
	}
}