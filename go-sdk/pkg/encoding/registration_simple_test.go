package encoding_test

import (
	"testing"
	
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/protobuf"
)

// TestJSONRegistration tests the new registration functions for JSON
func TestJSONRegistration(t *testing.T) {
	// Test explicit registration
	err := json.Register()
	if err != nil {
		t.Fatalf("Failed to register JSON: %v", err)
	}
	
	// Test idempotent registration
	err = json.Register()
	if err != nil {
		t.Fatalf("Second registration should not fail: %v", err)
	}
	
	// Test EnsureRegistered
	err = json.EnsureRegistered()
	if err != nil {
		t.Fatalf("EnsureRegistered failed: %v", err)
	}
}

// TestProtobufRegistration tests the new registration functions for Protobuf
func TestProtobufRegistration(t *testing.T) {
	// Test explicit registration
	err := protobuf.Register()
	if err != nil {
		t.Fatalf("Failed to register Protobuf: %v", err)
	}
	
	// Test idempotent registration
	err = protobuf.Register()
	if err != nil {
		t.Fatalf("Second registration should not fail: %v", err)
	}
	
	// Test EnsureRegistered
	err = protobuf.EnsureRegistered()
	if err != nil {
		t.Fatalf("EnsureRegistered failed: %v", err)
	}
}

// TestRegistrationToNilRegistry tests error handling
func TestRegistrationToNilRegistry(t *testing.T) {
	// Test JSON
	err := json.RegisterTo(nil)
	if err == nil {
		t.Error("Expected error when registering JSON to nil registry")
	}
	
	// Test Protobuf
	err = protobuf.RegisterTo(nil)
	if err == nil {
		t.Error("Expected error when registering Protobuf to nil registry")
	}
}