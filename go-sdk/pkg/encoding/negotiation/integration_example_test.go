package negotiation_test

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/negotiation"
)

// Example of how to integrate content negotiation with an HTTP handler
func ExampleContentNegotiator_httpHandler() {
	// Create negotiator
	negotiator := negotiation.NewContentNegotiator("application/json")

	// Track performance from actual usage
	negotiator.UpdatePerformance("application/json", negotiation.PerformanceMetrics{
		EncodingTime: 12 * time.Millisecond,
		DecodingTime: 10 * time.Millisecond,
		PayloadSize:  2048,
		SuccessRate:  0.98,
		Throughput:   150000,
	})

	negotiator.UpdatePerformance("application/x-protobuf", negotiation.PerformanceMetrics{
		EncodingTime: 6 * time.Millisecond,
		DecodingTime: 4 * time.Millisecond,
		PayloadSize:  1024,
		SuccessRate:  0.99,
		Throughput:   300000,
	})

	// Example HTTP handler
	handler := func(w http.ResponseWriter, r *http.Request) {
		// Get Accept header
		acceptHeader := r.Header.Get("Accept")

		// Negotiate content type
		contentType, err := negotiator.Negotiate(acceptHeader)
		if err != nil {
			// Fall back to default
			contentType = negotiator.PreferredType()
		}

		// Set Content-Type header
		if w != nil {
			w.Header().Set("Content-Type", contentType)
		}

		// Select appropriate encoder based on contentType
		// encoder := encodingFactory.CreateEncoder(contentType)

		fmt.Printf("Client Accept: %s -> Selected: %s\n", acceptHeader, contentType)
	}

	// Simulate requests
	req1, _ := http.NewRequest("GET", "/", nil)
	req1.Header.Set("Accept", "application/json")
	handler(nil, req1)

	req2, _ := http.NewRequest("GET", "/", nil)
	req2.Header.Set("Accept", "application/x-protobuf, application/json;q=0.8")
	handler(nil, req2)

	// Output:
	// Client Accept: application/json -> Selected: application/json
	// Client Accept: application/x-protobuf, application/json;q=0.8 -> Selected: application/x-protobuf
}

// Example of using adaptive selection in a service
func ExampleAdaptiveSelector_service() {
	negotiator := negotiation.NewContentNegotiator("application/json")
	adaptive := negotiation.NewAdaptiveSelector(negotiator)

	// Simulate a service that tracks request outcomes
	processRequest := func(acceptHeader string) {
		start := time.Now()

		// Select content type adaptively
		contentType, err := adaptive.SelectAdaptive(acceptHeader, nil)
		if err != nil {
			log.Printf("Failed to negotiate: %v", err)
			return
		}

		// Simulate processing with different success rates
		var success bool
		var processingTime time.Duration

		switch contentType {
		case "application/json":
			// JSON has occasional failures in this example
			success = time.Now().UnixNano()%10 > 2 // 80% success rate
			processingTime = 20 * time.Millisecond
		case "application/x-protobuf":
			// Protobuf is more reliable
			success = time.Now().UnixNano()%100 > 1 // 99% success rate
			processingTime = 10 * time.Millisecond
		}

		// Simulate processing delay
		time.Sleep(processingTime)

		// Update history
		elapsed := time.Since(start)
		adaptive.UpdateHistory(contentType, success, elapsed)

		fmt.Printf("Request processed with %s: success=%v, time=%v\n",
			contentType, success, elapsed.Round(time.Millisecond))
	}

	// Process some requests
	for i := 0; i < 5; i++ {
		processRequest("application/json, application/x-protobuf")
	}

	// Output will show adaptive behavior favoring protobuf over time
}

// Example of using selection criteria for different client types
func ExampleFormatSelector_clientTypes() {
	negotiator := negotiation.NewContentNegotiator("application/json")

	// Update performance metrics to show difference
	negotiator.UpdatePerformance("application/json", negotiation.PerformanceMetrics{
		EncodingTime: 15 * time.Millisecond,
		SuccessRate:  0.95,
	})
	negotiator.UpdatePerformance("application/x-protobuf", negotiation.PerformanceMetrics{
		EncodingTime: 5 * time.Millisecond,
		SuccessRate:  0.99,
	})

	selector := negotiation.NewFormatSelector(negotiator)

	// Standard criteria - quality-based selection
	standardCriteria := &negotiation.SelectionCriteria{
		PreferPerformance: false,
		MinQuality:        0.5,
	}

	// Performance criteria - performance-based selection
	performanceCriteria := &negotiation.SelectionCriteria{
		PreferPerformance: true,
		MinQuality:        0.5,
	}

	// Different quality factors
	acceptHeader1 := "application/json;q=1.0, application/x-protobuf;q=0.8"
	standardType, _ := selector.SelectFormat(acceptHeader1, standardCriteria)
	fmt.Printf("Higher quality JSON: %s\n", standardType)

	// Equal quality factors - performance decides
	acceptHeader2 := "application/json;q=0.9, application/x-protobuf;q=0.9"
	perfType, _ := selector.SelectFormat(acceptHeader2, performanceCriteria)
	fmt.Printf("Equal quality (performance mode): %s\n", perfType)

	// Output:
	// Higher quality JSON: application/json
	// Equal quality (performance mode): application/x-protobuf
}

// Example of middleware that adds content negotiation
func ExampleContentNegotiator_middleware() {
	negotiator := negotiation.NewContentNegotiator("application/json")

	// Middleware function
	contentNegotiationMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Negotiate content type
			acceptHeader := r.Header.Get("Accept")
			contentType, err := negotiator.Negotiate(acceptHeader)
			if err != nil {
				// Check if client accepts our default
				if negotiator.CanHandle(negotiator.PreferredType()) {
					contentType = negotiator.PreferredType()
				} else {
					http.Error(w, "Not Acceptable", http.StatusNotAcceptable)
					return
				}
			}

			// Store in context for handlers to use
			// ctx := context.WithValue(r.Context(), "contentType", contentType)
			// r = r.WithContext(ctx)

			// Set response header
			if w != nil {
				w.Header().Set("Content-Type", contentType)
			}

			// Call next handler
			next(w, r)
		}
	}

	// Example handler
	apiHandler := func(w http.ResponseWriter, r *http.Request) {
		// contentType := r.Context().Value("contentType").(string)
		fmt.Println("API handler called")
	}

	// Wrap with middleware
	handler := contentNegotiationMiddleware(apiHandler)

	// Simulate request
	req, _ := http.NewRequest("GET", "/api/data", nil)
	req.Header.Set("Accept", "application/vnd.ag-ui+json, application/json;q=0.8")
	handler(nil, req)

	// Output:
	// API handler called
}
