package negotiation_test

import (
	"fmt"
	"log"
	"time"

	"github.com/ag-ui/go-sdk/pkg/encoding/negotiation"
)

func ExampleContentNegotiator_basic() {
	// Create a content negotiator with JSON as default
	negotiator := negotiation.NewContentNegotiator("application/json")

	// Simple negotiation
	contentType, err := negotiator.Negotiate("application/json")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Selected:", contentType)

	// Output: Selected: application/json
}

func ExampleContentNegotiator_qualityFactors() {
	negotiator := negotiation.NewContentNegotiator("application/json")

	// Negotiate with quality factors
	contentType, err := negotiator.Negotiate("application/json;q=0.9, application/x-protobuf;q=1.0")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Selected:", contentType)

	// Output: Selected: application/x-protobuf
}

func ExampleContentNegotiator_wildcards() {
	negotiator := negotiation.NewContentNegotiator("application/json")

	// Handle wildcard accepts
	examples := []string{
		"*/*",
		"application/*, text/html;q=0.5",
		"*/*, application/json;q=0.9",
	}

	for _, accept := range examples {
		contentType, err := negotiator.Negotiate(accept)
		if err != nil {
			log.Printf("Error for %s: %v", accept, err)
			continue
		}
		fmt.Printf("Accept: %s -> %s\n", accept, contentType)
	}

	// Output:
	// Accept: */* -> application/json
	// Accept: application/*, text/html;q=0.5 -> application/x-protobuf
	// Accept: */*, application/json;q=0.9 -> application/x-protobuf
}

func ExampleContentNegotiator_customTypes() {
	negotiator := negotiation.NewContentNegotiator("application/json")

	// Register a custom type
	negotiator.RegisterType(&negotiation.TypeCapabilities{
		ContentType:        "application/vnd.myapp+json",
		CanStream:          true,
		CompressionSupport: []string{"gzip", "br"},
		Priority:           0.85,
		Extensions:         []string{".myapp.json"},
		Aliases:            []string{"application/x-myapp-json"},
	})

	// Negotiate with custom type
	contentType, err := negotiator.Negotiate("application/vnd.myapp+json")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Custom type:", contentType)

	// Also works with alias
	contentType, err = negotiator.Negotiate("application/x-myapp-json")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Via alias:", contentType)

	// Output:
	// Custom type: application/vnd.myapp+json
	// Via alias: application/vnd.myapp+json
}

func ExampleFormatSelector_withCriteria() {
	negotiator := negotiation.NewContentNegotiator("application/json")
	selector := negotiation.NewFormatSelector(negotiator)

	// Select with specific criteria
	criteria := &negotiation.SelectionCriteria{
		RequireStreaming: true,
		MinQuality:       0.7,
		ClientCapabilities: &negotiation.ClientCapabilities{
			SupportsStreaming:  true,
			CompressionSupport: []string{"gzip"},
			MaxPayloadSize:     10 * 1024 * 1024, // 10MB
		},
	}

	contentType, err := selector.SelectFormat("application/json;q=0.8, application/x-protobuf;q=0.9", criteria)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Selected with criteria: %s\n", contentType)

	// Output: Selected with criteria: application/x-protobuf
}

func ExamplePerformanceTracker() {
	negotiator := negotiation.NewContentNegotiator("application/json")

	// Simulate performance measurements
	formats := []string{"application/json", "application/x-protobuf"}
	
	for _, format := range formats {
		// Simulate multiple requests
		for i := 0; i < 5; i++ {
			var metrics negotiation.PerformanceMetrics
			
			if format == "application/json" {
				metrics = negotiation.PerformanceMetrics{
					EncodingTime: time.Duration(10+i) * time.Millisecond,
					DecodingTime: time.Duration(8+i) * time.Millisecond,
					PayloadSize:  int64(1024 + i*100),
					SuccessRate:  0.95,
					MemoryUsage:  int64(1024 * 1024),
					CPUUsage:     15.0 + float64(i),
				}
			} else {
				metrics = negotiation.PerformanceMetrics{
					EncodingTime: time.Duration(5+i) * time.Millisecond,
					DecodingTime: time.Duration(3+i) * time.Millisecond,
					PayloadSize:  int64(512 + i*50),
					SuccessRate:  0.99,
					MemoryUsage:  int64(512 * 1024),
					CPUUsage:     10.0 + float64(i)*0.5,
				}
			}
			
			negotiator.UpdatePerformance(format, metrics)
		}
	}

	// Now negotiate with equal quality factors
	contentType, err := negotiator.Negotiate("application/json;q=0.9, application/x-protobuf;q=0.9")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Performance-based selection: %s\n", contentType)

	// Output: Performance-based selection: application/x-protobuf
}

func ExampleAdaptiveSelector() {
	negotiator := negotiation.NewContentNegotiator("application/json")
	adaptive := negotiation.NewAdaptiveSelector(negotiator)

	// Simulate request history
	// JSON has some failures
	for i := 0; i < 8; i++ {
		adaptive.UpdateHistory("application/json", true, 20*time.Millisecond)
	}
	for i := 0; i < 4; i++ {
		adaptive.UpdateHistory("application/json", false, 100*time.Millisecond)
	}

	// Protobuf is more reliable
	for i := 0; i < 10; i++ {
		adaptive.UpdateHistory("application/x-protobuf", true, 10*time.Millisecond)
	}

	// Select adaptively
	contentType, err := adaptive.SelectAdaptive("application/json, application/x-protobuf", nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Adaptive selection: %s\n", contentType)

	// Output: Adaptive selection: application/x-protobuf
}

func ExampleParseAcceptHeader() {
	// Parse complex Accept header
	header := "application/json;q=0.9, application/x-protobuf;q=1.0, */*, text/html;q=0.8"
	
	acceptTypes, err := negotiation.ParseAcceptHeader(header)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Parsed Accept types:")
	for _, at := range acceptTypes {
		fmt.Printf("  %s (q=%.1f)\n", at.Type, at.Quality)
	}

	// Output:
	// Parsed Accept types:
	//   application/x-protobuf (q=1.0)
	//   */* (q=1.0)
	//   application/json (q=0.9)
	//   text/html (q=0.8)
}

func ExampleParseMediaType() {
	// Parse Content-Type header
	contentType := "application/json;charset=utf-8"
	
	mediaType, params, err := negotiation.ParseMediaType(contentType)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Media type: %s\n", mediaType)
	fmt.Printf("Parameters: %v\n", params)

	// Output:
	// Media type: application/json
	// Parameters: map[charset:utf-8]
}

func ExamplePreferenceOrderSelector() {
	negotiator := negotiation.NewContentNegotiator("application/json")
	
	// Define explicit preference order
	preferences := []string{
		"application/vnd.ag-ui+json",
		"application/x-protobuf",
		"application/json",
	}
	
	selector := negotiation.NewPreferenceOrderSelector(negotiator, preferences)
	
	// Will select first matching preference
	contentType, err := selector.SelectByPreference("application/json, application/x-protobuf, application/vnd.ag-ui+json")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("By preference order: %s\n", contentType)

	// Output: By preference order: application/vnd.ag-ui+json
}