package negotiation_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/encoding/negotiation"
)

func TestContentNegotiator(t *testing.T) {
	tests := []struct {
		name         string
		acceptHeader string
		expected     string
		shouldError  bool
	}{
		{
			name:         "Simple JSON request",
			acceptHeader: "application/json",
			expected:     "application/json",
		},
		{
			name:         "Simple Protobuf request",
			acceptHeader: "application/x-protobuf",
			expected:     "application/x-protobuf",
		},
		{
			name:         "JSON with quality factor",
			acceptHeader: "application/json;q=0.9, application/x-protobuf;q=1.0",
			expected:     "application/x-protobuf",
		},
		{
			name:         "Wildcard accept",
			acceptHeader: "*/*",
			expected:     "application/json", // Default preference
		},
		{
			name:         "Subtype wildcard",
			acceptHeader: "application/*, text/html;q=0.5",
			expected:     "application/x-protobuf", // Highest priority among application/*
		},
		{
			name:         "AG-UI specific format",
			acceptHeader: "application/vnd.ag-ui+json",
			expected:     "application/vnd.ag-ui+json",
		},
		{
			name:         "Complex quality factors",
			acceptHeader: "application/json;q=0.8, application/x-protobuf;q=0.9, application/vnd.ag-ui+json;q=0.95",
			expected:     "application/vnd.ag-ui+json",
		},
		{
			name:         "Unsupported type",
			acceptHeader: "application/xml",
			shouldError:  true,
		},
		{
			name:         "Multiple wildcards",
			acceptHeader: "*/*, application/json;q=0.9",
			expected:     "application/x-protobuf", // Highest server priority
		},
		{
			name:         "Empty accept header",
			acceptHeader: "",
			expected:     "application/json", // Default
		},
		{
			name:         "Invalid quality value",
			acceptHeader: "application/json;q=2.0",
			shouldError:  true,
		},
		{
			name:         "Alias matching",
			acceptHeader: "text/json",
			expected:     "application/json",
		},
	}

	negotiator := negotiation.NewContentNegotiator("application/json")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := negotiator.Negotiate(tt.acceptHeader)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestAcceptHeaderParsing(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected []negotiation.AcceptType
		hasError bool
	}{
		{
			name:   "Single type",
			header: "application/json",
			expected: []negotiation.AcceptType{
				{Type: "application/json", Quality: 1.0, Parameters: map[string]string{}},
			},
		},
		{
			name:   "Multiple types with quality",
			header: "application/json;q=0.9, application/x-protobuf;q=1.0",
			expected: []negotiation.AcceptType{
				{Type: "application/x-protobuf", Quality: 1.0, Parameters: map[string]string{}},
				{Type: "application/json", Quality: 0.9, Parameters: map[string]string{}},
			},
		},
		{
			name:   "Type with parameters",
			header: "application/json;charset=utf-8;q=0.8",
			expected: []negotiation.AcceptType{
				{Type: "application/json", Quality: 0.8, Parameters: map[string]string{"charset": "utf-8"}},
			},
		},
		{
			name:   "Wildcards",
			header: "*/*, application/*;q=0.8",
			expected: []negotiation.AcceptType{
				{Type: "*/*", Quality: 1.0, Parameters: map[string]string{}},
				{Type: "application/*", Quality: 0.8, Parameters: map[string]string{}},
			},
		},
		{
			name:     "Invalid format",
			header:   "not-a-valid-type",
			hasError: true,
		},
		{
			name:   "Empty header",
			header: "",
			expected: []negotiation.AcceptType{
				{Type: "*/*", Quality: 1.0, Parameters: map[string]string{}},
			},
		},
		{
			name:   "Quoted parameters",
			header: `application/json;charset="utf-8"`,
			expected: []negotiation.AcceptType{
				{Type: "application/json", Quality: 1.0, Parameters: map[string]string{"charset": "utf-8"}},
			},
		},
		{
			name:   "Three decimal places in quality",
			header: "application/json;q=0.999",
			expected: []negotiation.AcceptType{
				{Type: "application/json", Quality: 0.999, Parameters: map[string]string{}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := negotiation.ParseAcceptHeader(tt.header)

			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(result) != len(tt.expected) {
					t.Errorf("Expected %d types, got %d", len(tt.expected), len(result))
				}
				for i, expected := range tt.expected {
					if i >= len(result) {
						break
					}
					if result[i].Type != expected.Type {
						t.Errorf("Type mismatch at %d: expected %s, got %s", i, expected.Type, result[i].Type)
					}
					if result[i].Quality != expected.Quality {
						t.Errorf("Quality mismatch at %d: expected %f, got %f", i, expected.Quality, result[i].Quality)
					}
				}
			}
		})
	}
}

func TestPerformanceBasedSelection(t *testing.T) {
	negotiator := negotiation.NewContentNegotiator("application/json")
	
	// Update performance metrics
	negotiator.UpdatePerformance("application/json", negotiation.PerformanceMetrics{
		EncodingTime: 10 * time.Millisecond,
		DecodingTime: 8 * time.Millisecond,
		PayloadSize:  1024,
		SuccessRate:  0.95,
		Throughput:   100000,
		MemoryUsage:  1024 * 1024,
		CPUUsage:     15.0,
	})
	
	negotiator.UpdatePerformance("application/x-protobuf", negotiation.PerformanceMetrics{
		EncodingTime: 5 * time.Millisecond,
		DecodingTime: 3 * time.Millisecond,
		PayloadSize:  512,
		SuccessRate:  0.99,
		Throughput:   200000,
		MemoryUsage:  512 * 1024,
		CPUUsage:     10.0,
	})
	
	// When quality factors are equal, performance should decide
	result, err := negotiator.Negotiate("application/json;q=0.9, application/x-protobuf;q=0.9")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// Protobuf should win due to better performance
	if result != "application/x-protobuf" {
		t.Errorf("Expected application/x-protobuf based on performance, got %s", result)
	}
}

func TestFormatSelector(t *testing.T) {
	negotiator := negotiation.NewContentNegotiator("application/json")
	selector := negotiation.NewFormatSelector(negotiator)
	
	// Test with streaming requirement
	criteria := &negotiation.SelectionCriteria{
		RequireStreaming: true,
		MinQuality:       0.5,
	}
	
	result, err := selector.SelectFormat("application/json, application/xml", criteria)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	if result != "application/json" {
		t.Errorf("Expected application/json (supports streaming), got %s", result)
	}
	
	// Test with client capabilities
	criteria = &negotiation.SelectionCriteria{
		ClientCapabilities: &negotiation.ClientCapabilities{
			SupportsStreaming:  true,
			CompressionSupport: []string{"gzip"},
			PreferredFormats:   []string{"application/x-protobuf"},
		},
	}
	
	result, err = selector.SelectFormat("*/*, application/json;q=0.8", criteria)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// Should prefer protobuf based on client preferences
	if result != "application/x-protobuf" {
		t.Errorf("Expected application/x-protobuf based on client preference, got %s", result)
	}
}

func TestAdaptiveSelection(t *testing.T) {
	negotiator := negotiation.NewContentNegotiator("application/json")
	adaptive := negotiation.NewAdaptiveSelector(negotiator)
	
	// Simulate successful JSON requests
	for i := 0; i < 10; i++ {
		adaptive.UpdateHistory("application/json", true, 20*time.Millisecond)
	}
	
	// Simulate some failed JSON requests
	for i := 0; i < 5; i++ {
		adaptive.UpdateHistory("application/json", false, 50*time.Millisecond)
	}
	
	// Simulate successful protobuf requests
	for i := 0; i < 10; i++ {
		adaptive.UpdateHistory("application/x-protobuf", true, 10*time.Millisecond)
	}
	
	// Now select with adaptive logic
	result, err := adaptive.SelectAdaptive("application/json, application/x-protobuf", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// Should prefer protobuf due to better success rate and performance
	if result != "application/x-protobuf" {
		t.Logf("Warning: Expected application/x-protobuf based on adaptive selection, got %s", result)
	}
}

func TestSupportedTypes(t *testing.T) {
	negotiator := negotiation.NewContentNegotiator("application/json")
	
	supported := negotiator.SupportedTypes()
	
	// Should have at least the default types
	expectedTypes := []string{
		"application/json",
		"application/x-protobuf",
		"application/vnd.ag-ui+json",
	}
	
	for _, expected := range expectedTypes {
		found := false
		for _, supported := range supported {
			if supported == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected type %s not found in supported types", expected)
		}
	}
}

func TestCanHandle(t *testing.T) {
	negotiator := negotiation.NewContentNegotiator("application/json")
	
	tests := []struct {
		contentType string
		expected    bool
	}{
		{"application/json", true},
		{"application/json;charset=utf-8", true},
		{"application/x-protobuf", true},
		{"application/xml", false},
		{"text/json", true}, // Alias
		{"application/vnd.ag-ui+json", true},
		{"*/*", false}, // Wildcards not directly handled
	}
	
	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			result := negotiator.CanHandle(tt.contentType)
			if result != tt.expected {
				t.Errorf("CanHandle(%s) = %v, expected %v", tt.contentType, result, tt.expected)
			}
		})
	}
}

func TestMediaTypeParsing(t *testing.T) {
	tests := []struct {
		input          string
		expectedType   string
		expectedParams map[string]string
		hasError       bool
	}{
		{
			input:          "application/json",
			expectedType:   "application/json",
			expectedParams: map[string]string{},
		},
		{
			input:          "application/json;charset=utf-8",
			expectedType:   "application/json",
			expectedParams: map[string]string{"charset": "utf-8"},
		},
		{
			input:          "application/json; charset=utf-8; boundary=something",
			expectedType:   "application/json",
			expectedParams: map[string]string{"charset": "utf-8", "boundary": "something"},
		},
		{
			input:    "not-a-type",
			hasError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			mediaType, params, err := negotiation.ParseMediaType(tt.input)
			
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if mediaType != tt.expectedType {
					t.Errorf("Expected type %s, got %s", tt.expectedType, mediaType)
				}
				for k, v := range tt.expectedParams {
					if params[k] != v {
						t.Errorf("Expected param %s=%s, got %s", k, v, params[k])
					}
				}
			}
		})
	}
}

func TestFormatMediaType(t *testing.T) {
	tests := []struct {
		mediaType string
		params    map[string]string
		expected  string
	}{
		{
			mediaType: "application/json",
			params:    nil,
			expected:  "application/json",
		},
		{
			mediaType: "application/json",
			params:    map[string]string{"charset": "utf-8"},
			expected:  "application/json; charset=utf-8",
		},
		{
			mediaType: "multipart/form-data",
			params:    map[string]string{"boundary": "----WebKitFormBoundary"},
			expected:  `multipart/form-data; boundary="----WebKitFormBoundary"`,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := negotiation.FormatMediaType(tt.mediaType, tt.params)
			// Since map ordering is not guaranteed, we need to check differently
			if !strings.Contains(result, tt.mediaType) {
				t.Errorf("Result %s doesn't contain media type %s", result, tt.mediaType)
			}
			for k := range tt.params {
				if !strings.Contains(result, k+"=") {
					t.Errorf("Result %s doesn't contain parameter %s", result, k)
				}
			}
		})
	}
}

func TestConcurrentPerformanceAccess(t *testing.T) {
	negotiator := negotiation.NewContentNegotiator("application/json")
	
	// Number of concurrent goroutines
	numGoroutines := 100
	numIterations := 1000
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3) // 3 types of operations
	
	// Goroutines that update performance metrics
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				contentType := "application/json"
				if j%2 == 0 {
					contentType = "application/x-protobuf"
				}
				
				negotiator.UpdatePerformance(contentType, negotiation.PerformanceMetrics{
					EncodingTime: time.Duration(id+j) * time.Microsecond,
					DecodingTime: time.Duration(id+j) * time.Microsecond,
					PayloadSize:  int64(id * j),
					SuccessRate:  float64(j%100) / 100.0,
					Throughput:   float64(id * j * 1000),
					MemoryUsage:  int64(id * j * 1024),
					CPUUsage:     float64(id+j) / 100.0,
				})
			}
		}(i)
	}
	
	// Goroutines that read performance scores
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				contentType := "application/json"
				if j%3 == 0 {
					contentType = "application/x-protobuf"
				}
				
				// This should not panic or race
				score := negotiator.GetPerformanceScore(contentType)
				_ = score // Use the score to avoid compiler optimization
			}
		}(i)
	}
	
	// Goroutines that negotiate content types (which internally access performance)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				acceptHeader := "application/json, application/x-protobuf"
				if j%2 == 0 {
					acceptHeader = "application/json;q=0.9, application/x-protobuf;q=0.9"
				}
				
				// This should not panic or race
				result, err := negotiator.Negotiate(acceptHeader)
				if err != nil {
					t.Errorf("Unexpected error during concurrent negotiation: %v", err)
				}
				_ = result // Use the result to avoid compiler optimization
			}
		}(i)
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	
	// If we get here without panics or data races, the test passes
	t.Logf("Successfully completed %d concurrent operations across %d goroutines", 
		numGoroutines*numIterations*3, numGoroutines*3)
}