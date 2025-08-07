package transport

import (
	"testing"
)

// TestBasicTypes tests basic type definitions
func TestBasicTypes(t *testing.T) {
	// Test simplified Capabilities struct
	caps := Capabilities{
		Streaming:       true,
		Bidirectional:   true,
		MaxMessageSize:  1024 * 1024,
		ProtocolVersion: "1.0",
	}

	if !caps.Streaming {
		t.Error("Expected streaming to be true")
	}

	if !caps.Bidirectional {
		t.Error("Expected bidirectional to be true")
	}

	if caps.MaxMessageSize != 1024*1024 {
		t.Error("Expected max message size to be 1MB")
	}

	if caps.ProtocolVersion != "1.0" {
		t.Error("Expected protocol version to be 1.0")
	}
}
