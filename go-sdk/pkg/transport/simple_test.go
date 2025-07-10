package transport

import (
	"testing"
)

// TestBasicTypes tests basic type definitions
func TestBasicTypes(t *testing.T) {
	// Test CompressionType constants
	if CompressionGzip != "gzip" {
		t.Errorf("Expected CompressionGzip to be 'gzip', got %s", CompressionGzip)
	}
	
	// Test SecurityFeature constants
	if SecurityTLS != "tls" {
		t.Errorf("Expected SecurityTLS to be 'tls', got %s", SecurityTLS)
	}
	
	// Test Capabilities struct
	caps := Capabilities{
		Streaming:     true,
		Bidirectional: true,
		Compression:   []CompressionType{CompressionGzip},
		Security:      []SecurityFeature{SecurityTLS},
	}
	
	if !caps.Streaming {
		t.Error("Expected streaming to be true")
	}
	
	if !caps.Bidirectional {
		t.Error("Expected bidirectional to be true")
	}
	
	if len(caps.Compression) != 1 || caps.Compression[0] != CompressionGzip {
		t.Error("Expected compression to contain gzip")
	}
	
	if len(caps.Security) != 1 || caps.Security[0] != SecurityTLS {
		t.Error("Expected security to contain TLS")
	}
}