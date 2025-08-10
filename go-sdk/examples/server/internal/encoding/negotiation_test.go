package encoding

import (
	"testing"
)

// TestNegotiateContentType tests the content type negotiation logic
func TestNegotiateContentType(t *testing.T) {
	supportedTypes := []string{"application/json", "application/vnd.ag-ui+json"}
	defaultType := "application/json"

	tests := []struct {
		name         string
		acceptHeader string
		expected     string
	}{
		{
			name:         "empty accept header",
			acceptHeader: "",
			expected:     "application/json",
		},
		{
			name:         "exact JSON match",
			acceptHeader: "application/json",
			expected:     "application/json",
		},
		{
			name:         "exact AG-UI JSON match",
			acceptHeader: "application/vnd.ag-ui+json",
			expected:     "application/vnd.ag-ui+json",
		},
		{
			name:         "wildcard match",
			acceptHeader: "*/*",
			expected:     "application/json", // First supported type
		},
		{
			name:         "application wildcard",
			acceptHeader: "application/*",
			expected:     "application/json", // First matching application type
		},
		{
			name:         "multiple types with JSON first",
			acceptHeader: "application/json,text/html,application/xml",
			expected:     "application/json",
		},
		{
			name:         "multiple types with AG-UI JSON first",
			acceptHeader: "application/vnd.ag-ui+json,application/json,text/html",
			expected:     "application/vnd.ag-ui+json",
		},
		{
			name:         "quality values ignored in simple parser",
			acceptHeader: "text/html;q=0.9,application/json;q=0.8",
			expected:     "application/json", // First match wins in simple parser
		},
		{
			name:         "unsupported type falls back to default",
			acceptHeader: "application/xml",
			expected:     "application/json",
		},
		{
			name:         "complex accept header",
			acceptHeader: "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
			expected:     "application/json", // Wildcard matches
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := negotiateContentType(tt.acceptHeader, supportedTypes, defaultType)
			if result != tt.expected {
				t.Errorf("negotiateContentType() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestParseAcceptHeader tests Accept header parsing
func TestParseAcceptHeader(t *testing.T) {
	tests := []struct {
		name         string
		acceptHeader string
		expected     []string
	}{
		{
			name:         "single type",
			acceptHeader: "application/json",
			expected:     []string{"application/json"},
		},
		{
			name:         "multiple types",
			acceptHeader: "text/html,application/json,application/xml",
			expected:     []string{"text/html", "application/json", "application/xml"},
		},
		{
			name:         "types with quality values",
			acceptHeader: "text/html;q=0.9,application/json;q=0.8,*/*;q=0.1",
			expected:     []string{"text/html", "application/json", "*/*"},
		},
		{
			name:         "types with spaces",
			acceptHeader: "text/html , application/json , application/xml",
			expected:     []string{"text/html", "application/json", "application/xml"},
		},
		{
			name:         "empty header",
			acceptHeader: "",
			expected:     []string{},
		},
		{
			name:         "complex header",
			acceptHeader: "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
			expected:     []string{"text/html", "application/xhtml+xml", "application/xml", "image/webp", "image/apng", "*/*", "application/signed-exchange"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAcceptHeader(tt.acceptHeader)

			if len(result) != len(tt.expected) {
				t.Errorf("parseAcceptHeader() returned %d types, expected %d", len(result), len(tt.expected))
				t.Errorf("Got: %v, Expected: %v", result, tt.expected)
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("parseAcceptHeader() result[%d] = %v, expected %v", i, result[i], expected)
				}
			}
		})
	}
}

// TestMatches tests the content type matching logic
func TestMatches(t *testing.T) {
	tests := []struct {
		name          string
		acceptedType  string
		supportedType string
		expected      bool
	}{
		{
			name:          "exact match",
			acceptedType:  "application/json",
			supportedType: "application/json",
			expected:      true,
		},
		{
			name:          "no match",
			acceptedType:  "application/xml",
			supportedType: "application/json",
			expected:      false,
		},
		{
			name:          "wildcard match all",
			acceptedType:  "*/*",
			supportedType: "application/json",
			expected:      true,
		},
		{
			name:          "application wildcard match",
			acceptedType:  "application/*",
			supportedType: "application/json",
			expected:      true,
		},
		{
			name:          "application wildcard no match",
			acceptedType:  "text/*",
			supportedType: "application/json",
			expected:      false,
		},
		{
			name:          "subtype wildcard matches",
			acceptedType:  "application/*",
			supportedType: "application/vnd.ag-ui+json",
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matches(tt.acceptedType, tt.supportedType)
			if result != tt.expected {
				t.Errorf("matches() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestIsSSEEndpoint tests SSE endpoint detection
func TestIsSSEEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "main SSE endpoint",
			path:     "/examples/_internal/stream",
			expected: true,
		},
		{
			name:     "legacy events endpoint",
			path:     "/events",
			expected: true,
		},
		{
			name:     "generic stream endpoint",
			path:     "/stream",
			expected: true,
		},
		{
			name:     "nested stream path",
			path:     "/api/v1/stream/data",
			expected: true,
		},
		{
			name:     "not an SSE endpoint",
			path:     "/api/users",
			expected: false,
		},
		{
			name:     "root path",
			path:     "/",
			expected: false,
		},
		{
			name:     "empty path",
			path:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSSEEndpoint(tt.path)
			if result != tt.expected {
				t.Errorf("isSSEEndpoint() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestDefaultContentNegotiationConfig tests default configuration
func TestDefaultContentNegotiationConfig(t *testing.T) {
	config := DefaultContentNegotiationConfig()

	if config.DefaultContentType != "application/json" {
		t.Errorf("DefaultContentType = %v, expected application/json", config.DefaultContentType)
	}

	if len(config.SupportedTypes) == 0 {
		t.Error("SupportedTypes should not be empty")
	}

	// Should include JSON
	foundJSON := false
	for _, contentType := range config.SupportedTypes {
		if contentType == "application/json" {
			foundJSON = true
			break
		}
	}

	if !foundJSON {
		t.Error("SupportedTypes should include application/json")
	}
}

// Benchmark content negotiation performance
func BenchmarkNegotiateContentType(b *testing.B) {
	supportedTypes := []string{"application/json", "application/vnd.ag-ui+json"}
	defaultType := "application/json"
	acceptHeader := "text/html,application/xhtml+xml,application/xml;q=0.9,application/json;q=0.8,*/*;q=0.7"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		negotiateContentType(acceptHeader, supportedTypes, defaultType)
	}
}

func BenchmarkParseAcceptHeader(b *testing.B) {
	acceptHeader := "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseAcceptHeader(acceptHeader)
	}
}

// Test edge cases
func TestNegotiateContentType_EdgeCases(t *testing.T) {
	supportedTypes := []string{"application/json"}
	defaultType := "application/json"

	tests := []struct {
		name         string
		acceptHeader string
		expected     string
	}{
		{
			name:         "malformed header with extra commas",
			acceptHeader: ",,application/json,,",
			expected:     "application/json",
		},
		{
			name:         "header with only spaces",
			acceptHeader: "   ,  ,  ",
			expected:     "application/json",
		},
		{
			name:         "header with semicolons but no content type",
			acceptHeader: ";q=0.8;charset=utf-8",
			expected:     "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := negotiateContentType(tt.acceptHeader, supportedTypes, defaultType)
			if result != tt.expected {
				t.Errorf("negotiateContentType() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
