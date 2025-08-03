package negotiation_test

import (
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/negotiation"
)

func BenchmarkContentNegotiation(b *testing.B) {
	negotiator := negotiation.NewContentNegotiator("application/json")

	// Update some performance metrics
	negotiator.UpdatePerformance("application/json", negotiation.PerformanceMetrics{
		EncodingTime: 10 * time.Millisecond,
		SuccessRate:  0.95,
	})
	negotiator.UpdatePerformance("application/x-protobuf", negotiation.PerformanceMetrics{
		EncodingTime: 5 * time.Millisecond,
		SuccessRate:  0.99,
	})

	acceptHeaders := []string{
		"application/json",
		"application/json;q=0.9, application/x-protobuf;q=1.0",
		"application/*, text/html;q=0.5",
		"*/*, application/json;q=0.9",
		"application/vnd.ag-ui+json;charset=utf-8;q=0.95",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		header := acceptHeaders[i%len(acceptHeaders)]
		_, err := negotiator.Negotiate(header)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAcceptHeaderParsing(b *testing.B) {
	headers := []string{
		"application/json",
		"application/json;q=0.9, application/x-protobuf;q=1.0",
		"application/json;charset=utf-8;q=0.8, application/x-protobuf;q=0.9, text/html;q=0.5",
		"*/*, application/json;q=0.9, application/x-protobuf;q=0.8",
		"application/vnd.ag-ui+json;charset=utf-8;q=0.95, application/json;q=0.9",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		header := headers[i%len(headers)]
		_, err := negotiation.ParseAcceptHeader(header)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPerformanceTracking(b *testing.B) {
	tracker := negotiation.NewPerformanceTracker()
	
	metrics := negotiation.PerformanceMetrics{
		EncodingTime: 10 * time.Millisecond,
		DecodingTime: 8 * time.Millisecond,
		PayloadSize:  1024,
		SuccessRate:  0.95,
		MemoryUsage:  1024 * 1024,
		CPUUsage:     15.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		contentType := "application/json"
		if i%2 == 0 {
			contentType = "application/x-protobuf"
		}
		tracker.UpdateMetrics(contentType, metrics)
	}
}

func BenchmarkFormatSelection(b *testing.B) {
	negotiator := negotiation.NewContentNegotiator("application/json")
	selector := negotiation.NewFormatSelector(negotiator)

	criteria := &negotiation.SelectionCriteria{
		RequireStreaming:  true,
		PreferPerformance: true,
		MinQuality:        0.5,
	}

	acceptHeaders := []string{
		"application/json, application/x-protobuf",
		"application/json;q=0.9, application/x-protobuf;q=1.0",
		"*/*, application/json;q=0.8",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		header := acceptHeaders[i%len(acceptHeaders)]
		_, err := selector.SelectFormat(header, criteria)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConcurrentNegotiation(b *testing.B) {
	negotiator := negotiation.NewContentNegotiator("application/json")

	// Warm up with some data
	negotiator.UpdatePerformance("application/json", negotiation.PerformanceMetrics{
		EncodingTime: 10 * time.Millisecond,
		SuccessRate:  0.95,
	})

	b.RunParallel(func(pb *testing.PB) {
		headers := []string{
			"application/json",
			"application/x-protobuf",
			"application/json;q=0.9, application/x-protobuf;q=1.0",
		}
		i := 0
		for pb.Next() {
			header := headers[i%len(headers)]
			_, err := negotiator.Negotiate(header)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}