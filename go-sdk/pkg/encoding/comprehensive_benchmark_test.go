package encoding_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"testing"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/negotiation"
)

// BenchmarkEncodingThroughput benchmarks encoding throughput for different formats
func BenchmarkEncodingThroughput(b *testing.B) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()

	// Create test events of different sizes
	smallEvent := events.NewTextMessageContentEvent("msg1", "small")
	mediumEvent := events.NewTextMessageContentEvent("msg2", strings.Repeat("medium", 100))
	largeEvent := events.NewTextMessageContentEvent("msg3", strings.Repeat("large", 1000))

	formats := []string{
		"application/json",
	}

	eventSizes := []struct {
		name  string
		event events.Event
	}{
		{"Small", smallEvent},
		{"Medium", mediumEvent},
		{"Large", largeEvent},
	}

	for _, format := range formats {
		for _, eventSize := range eventSizes {
			b.Run(fmt.Sprintf("%s_%s", format, eventSize.name), func(b *testing.B) {
				encoder, err := registry.GetEncoder(ctx, format, nil)
				if err != nil {
					b.Fatalf("Failed to get encoder: %v", err)
				}

				b.ResetTimer()
				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					_, err := encoder.Encode(ctx, eventSize.event)
					if err != nil {
						b.Fatalf("Encoding failed: %v", err)
					}
				}
			})
		}
	}
}

// BenchmarkDecodingThroughput benchmarks decoding throughput for different formats
func BenchmarkDecodingThroughput(b *testing.B) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()

	// Pre-encode test data
	testData := make(map[string]map[string][]byte)

	events := []events.Event{
		events.NewTextMessageContentEvent("msg1", "small"),
		events.NewTextMessageContentEvent("msg2", strings.Repeat("medium", 100)),
		events.NewTextMessageContentEvent("msg3", strings.Repeat("large", 1000)),
	}

	sizes := []string{"Small", "Medium", "Large"}
	formats := []string{"application/json"}

	for _, format := range formats {
		testData[format] = make(map[string][]byte)
		encoder, err := registry.GetEncoder(ctx, format, nil)
		if err != nil {
			b.Fatalf("Failed to get encoder: %v", err)
		}

		for i, event := range events {
			data, err := encoder.Encode(ctx, event)
			if err != nil {
				b.Fatalf("Failed to encode: %v", err)
			}
			testData[format][sizes[i]] = data
		}
	}

	for _, format := range formats {
		for _, size := range sizes {
			b.Run(fmt.Sprintf("%s_%s", format, size), func(b *testing.B) {
				decoder, err := registry.GetDecoder(ctx, format, nil)
				if err != nil {
					b.Fatalf("Failed to get decoder: %v", err)
				}

				data := testData[format][size]
				b.ResetTimer()
				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					_, err := decoder.Decode(ctx, data)
					if err != nil {
						b.Fatalf("Decoding failed: %v", err)
					}
				}
			})
		}
	}
}

// BenchmarkStreamingThroughput benchmarks streaming throughput
func BenchmarkStreamingThroughput(b *testing.B) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()

	formats := []string{
		"application/json",
	}

	eventCounts := []int{10, 100, 1000}

	for _, format := range formats {
		for _, count := range eventCounts {
			b.Run(fmt.Sprintf("%s_%d_events", format, count), func(b *testing.B) {
				encoder, err := registry.GetStreamEncoder(ctx, format, nil)
				if err != nil {
					b.Fatalf("Failed to get stream encoder: %v", err)
				}

				decoder, err := registry.GetStreamDecoder(ctx, format, nil)
				if err != nil {
					b.Fatalf("Failed to get stream decoder: %v", err)
				}

				// Generate test events
				eventsList := make([]events.Event, count)
				for i := 0; i < count; i++ {
					eventsList[i] = events.NewTextMessageContentEvent(fmt.Sprintf("msg-%d", i), fmt.Sprintf("content-%d", i))
				}

				b.ResetTimer()
				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					// Encode stream
					var buf bytes.Buffer
					eventChan := make(chan events.Event, count)

					go func() {
						for _, event := range eventsList {
							eventChan <- event
						}
						close(eventChan)
					}()

					err := encoder.EncodeStream(ctx, eventChan, &buf)
					if err != nil {
						b.Fatalf("Stream encoding failed: %v", err)
					}

					// Decode stream
					reader := bytes.NewReader(buf.Bytes())
					decodedChan := make(chan events.Event, count)

					go func() {
						err := decoder.DecodeStream(ctx, reader, decodedChan)
						if err != nil {
							b.Errorf("Stream decoding failed: %v", err)
						}
					}()

					// Count decoded events
					decodedCount := 0
					for range decodedChan {
						decodedCount++
					}

					if decodedCount != count {
						b.Errorf("Expected %d events, got %d", count, decodedCount)
					}
				}
			})
		}
	}
}

// BenchmarkPoolEfficiency benchmarks pool efficiency
func BenchmarkPoolEfficiency(b *testing.B) {
	ctx := context.Background()

	// Test with pooled factory
	b.Run("Pooled", func(b *testing.B) {
		factory := encoding.NewPooledCodecFactory()

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			codec, err := factory.CreateCodec(ctx, "application/json", nil, nil)
			if err != nil {
				b.Fatalf("Failed to create encoder: %v", err)
			}

			// Use encoder
			event := events.NewTextMessageContentEvent("msg", "content")
			_, err = codec.Encode(ctx, event)
			if err != nil {
				b.Fatalf("Encoding failed: %v", err)
			}

			// Release back to pool
			if releasable, ok := codec.(encoding.ReleasableEncoder); ok {
				releasable.Release()
			}
		}
	})

	// Test without pooling (create new each time)
	b.Run("NonPooled", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			encoder := json.NewJSONEncoder(nil)

			// Use encoder
			event := events.NewTextMessageContentEvent("msg", "content")
			_, err := encoder.Encode(ctx, event)
			if err != nil {
				b.Fatalf("Encoding failed: %v", err)
			}
		}
	})
}

// BenchmarkBufferPooling benchmarks buffer pooling efficiency
func BenchmarkBufferPooling(b *testing.B) {
	b.Run("WithPooling", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			buf := encoding.GetBuffer(1024)
			if buf == nil {
				// Use safe version for benchmarks
				buf = encoding.GetBufferSafe(1024)
				if buf == nil {
					b.Fatal("Failed to allocate buffer")
				}
			}
			buf.WriteString("test data")
			buf.WriteString(strings.Repeat("x", 500))
			encoding.PutBuffer(buf)
		}
	})

	b.Run("WithoutPooling", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			buf := &bytes.Buffer{}
			buf.WriteString("test data")
			buf.WriteString(strings.Repeat("x", 500))
			// No pooling - just let GC handle it
		}
	})
}

// BenchmarkConcurrentEncoding benchmarks concurrent encoding performance
func BenchmarkConcurrentEncoding(b *testing.B) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()

	goroutineCounts := []int{1, 2, 4, 8, 16}

	for _, goroutines := range goroutineCounts {
		b.Run(fmt.Sprintf("Goroutines_%d", goroutines), func(b *testing.B) {
			encoder, err := registry.GetEncoder(ctx, "application/json", nil)
			if err != nil {
				b.Fatalf("Failed to get encoder: %v", err)
			}

			event := events.NewTextMessageContentEvent("msg", "test content")

			b.ResetTimer()
			b.ReportAllocs()

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_, err := encoder.Encode(ctx, event)
					if err != nil {
						b.Fatalf("Encoding failed: %v", err)
					}
				}
			})
		})
	}
}

// BenchmarkFormatComparison benchmarks different formats against each other
func BenchmarkFormatComparison(b *testing.B) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()

	// Create various test events
	testEvents := []events.Event{
		events.NewTextMessageStartEvent("msg1", events.WithRole("user")),
		events.NewTextMessageContentEvent("msg1", "Hello world"),
		events.NewTextMessageContentEvent("msg1", strings.Repeat("Long content ", 100)),
		events.NewTextMessageEndEvent("msg1"),
	}

	formats := []string{
		"application/json",
	}

	for _, format := range formats {
		b.Run(fmt.Sprintf("Format_%s", format), func(b *testing.B) {
			encoder, err := registry.GetEncoder(ctx, format, nil)
			if err != nil {
				b.Fatalf("Failed to get encoder: %v", err)
			}

			decoder, err := registry.GetDecoder(ctx, format, nil)
			if err != nil {
				b.Fatalf("Failed to get decoder: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Round-trip test
				data, err := encoder.EncodeMultiple(ctx, testEvents)
				if err != nil {
					b.Fatalf("Encoding failed: %v", err)
				}

				_, err = decoder.DecodeMultiple(ctx, data)
				if err != nil {
					b.Fatalf("Decoding failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()

	// Generate random content of various sizes
	contentSizes := []int{100, 1000, 10000, 100000}

	for _, size := range contentSizes {
		b.Run(fmt.Sprintf("ContentSize_%d", size), func(b *testing.B) {
			content := generateRandomContent(size)
			event := events.NewTextMessageContentEvent("msg", content)

			encoder, err := registry.GetEncoder(ctx, "application/json", nil)
			if err != nil {
				b.Fatalf("Failed to get encoder: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := encoder.Encode(ctx, event)
				if err != nil {
					b.Fatalf("Encoding failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkRegistryLookup benchmarks registry lookup performance
func BenchmarkRegistryLookup(b *testing.B) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()

	mimeTypes := []string{
		"application/json",
		"application/x-protobuf",
		"json",
		"protobuf",
	}

	b.Run("EncoderLookup", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			mimeType := mimeTypes[i%len(mimeTypes)]
			_, err := registry.GetEncoder(ctx, mimeType, nil)
			if err != nil {
				b.Fatalf("Failed to get encoder for %s: %v", mimeType, err)
			}
		}
	})

	b.Run("DecoderLookup", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			mimeType := mimeTypes[i%len(mimeTypes)]
			_, err := registry.GetDecoder(ctx, mimeType, nil)
			if err != nil {
				b.Fatalf("Failed to get decoder for %s: %v", mimeType, err)
			}
		}
	})
}

// BenchmarkValidationOverhead benchmarks validation overhead
func BenchmarkValidationOverhead(b *testing.B) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()

	event := events.NewTextMessageStartEvent("msg", events.WithRole("user"))

	b.Run("WithValidation", func(b *testing.B) {
		encoder, err := registry.GetEncoder(ctx, "application/json", &encoding.EncodingOptions{
			ValidateOutput: true,
		})
		if err != nil {
			b.Fatalf("Failed to get encoder: %v", err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := encoder.Encode(ctx, event)
			if err != nil {
				b.Fatalf("Encoding failed: %v", err)
			}
		}
	})

	b.Run("WithoutValidation", func(b *testing.B) {
		encoder, err := registry.GetEncoder(ctx, "application/json", &encoding.EncodingOptions{
			ValidateOutput: false,
		})
		if err != nil {
			b.Fatalf("Failed to get encoder: %v", err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := encoder.Encode(ctx, event)
			if err != nil {
				b.Fatalf("Encoding failed: %v", err)
			}
		}
	})
}

// BenchmarkStreamingLatency benchmarks streaming latency
func BenchmarkStreamingLatency(b *testing.B) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()

	encoder, err := registry.GetStreamEncoder(ctx, "application/json", nil)
	if err != nil {
		b.Fatalf("Failed to get stream encoder: %v", err)
	}

	decoder, err := registry.GetStreamDecoder(ctx, "application/json", nil)
	if err != nil {
		b.Fatalf("Failed to get stream decoder: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Measure end-to-end latency
		var buf bytes.Buffer

		// Start streams
		err := encoder.StartStream(ctx, &buf)
		if err != nil {
			b.Fatalf("Failed to start encoder stream: %v", err)
		}

		// Write event
		event := events.NewTextMessageContentEvent("msg", "content")
		err = encoder.WriteEvent(ctx, event)
		if err != nil {
			b.Fatalf("Failed to write event: %v", err)
		}

		// End encoder stream
		err = encoder.EndStream(ctx)
		if err != nil {
			b.Fatalf("Failed to end encoder stream: %v", err)
		}

		// Start decoder stream
		reader := bytes.NewReader(buf.Bytes())
		err = decoder.StartStream(ctx, reader)
		if err != nil {
			b.Fatalf("Failed to start decoder stream: %v", err)
		}

		// Read event
		_, err = decoder.ReadEvent(ctx)
		if err != nil && err != io.EOF {
			b.Fatalf("Failed to read event: %v", err)
		}

		// End decoder stream
		err = decoder.EndStream(ctx)
		if err != nil {
			b.Fatalf("Failed to end decoder stream: %v", err)
		}
	}
}

// BenchmarkNegotiationPerformance benchmarks content negotiation performance
func BenchmarkNegotiationPerformance(b *testing.B) {
	negotiator := negotiation.NewContentNegotiator("application/json")

	acceptHeaders := []string{
		"application/json",
		"application/json;q=0.8",
		"text/html,application/xhtml+xml,application/xml;q=0.9,application/json;q=0.8,*/*;q=0.7",
		"*/*",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		header := acceptHeaders[i%len(acceptHeaders)]
		_, err := negotiator.Negotiate(header)
		if err != nil {
			b.Fatalf("Negotiation failed: %v", err)
		}
	}
}

// BenchmarkBulkOperations benchmarks bulk operations
func BenchmarkBulkOperations(b *testing.B) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()

	// Generate bulk events
	eventCounts := []int{10, 100, 1000}

	for _, count := range eventCounts {
		eventsList := make([]events.Event, count)
		for i := 0; i < count; i++ {
			eventsList[i] = events.NewTextMessageContentEvent(fmt.Sprintf("msg-%d", i), fmt.Sprintf("content-%d", i))
		}

		b.Run(fmt.Sprintf("BulkEncode_%d", count), func(b *testing.B) {
			encoder, err := registry.GetEncoder(ctx, "application/json", nil)
			if err != nil {
				b.Fatalf("Failed to get encoder: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := encoder.EncodeMultiple(ctx, eventsList)
				if err != nil {
					b.Fatalf("Bulk encoding failed: %v", err)
				}
			}
		})

		b.Run(fmt.Sprintf("BulkDecode_%d", count), func(b *testing.B) {
			encoder, err := registry.GetEncoder(ctx, "application/json", nil)
			if err != nil {
				b.Fatalf("Failed to get encoder: %v", err)
			}

			decoder, err := registry.GetDecoder(ctx, "application/json", nil)
			if err != nil {
				b.Fatalf("Failed to get decoder: %v", err)
			}

			// Pre-encode the data
			data, err := encoder.EncodeMultiple(ctx, eventsList)
			if err != nil {
				b.Fatalf("Failed to pre-encode: %v", err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := decoder.DecodeMultiple(ctx, data)
				if err != nil {
					b.Fatalf("Bulk decoding failed: %v", err)
				}
			}
		})
	}
}

// Helper function to generate random content
func generateRandomContent(size int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 "
	b := make([]byte, size)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// BenchmarkErrorHandlingPerformance benchmarks error handling performance
func BenchmarkErrorHandlingPerformance(b *testing.B) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()

	decoder, err := registry.GetDecoder(ctx, "application/json", nil)
	if err != nil {
		b.Fatalf("Failed to get decoder: %v", err)
	}

	// Invalid JSON data
	invalidData := []byte("invalid json data")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := decoder.Decode(ctx, invalidData)
		if err == nil {
			b.Fatal("Expected error but got none")
		}
	}
}

// BenchmarkAllocationPattern benchmarks allocation patterns
func BenchmarkAllocationPattern(b *testing.B) {
	ctx := context.Background()

	// Test different allocation strategies
	b.Run("FreshEncoder", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			encoder := json.NewJSONEncoder(nil)
			event := events.NewTextMessageContentEvent("msg", "content")
			_, err := encoder.Encode(ctx, event)
			if err != nil {
				b.Fatalf("Encoding failed: %v", err)
			}
		}
	})

	b.Run("ReusedEncoder", func(b *testing.B) {
		encoder := json.NewJSONEncoder(nil)
		event := events.NewTextMessageContentEvent("msg", "content")

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := encoder.Encode(ctx, event)
			if err != nil {
				b.Fatalf("Encoding failed: %v", err)
			}
		}
	})

	b.Run("PooledEncoder", func(b *testing.B) {
		factory := encoding.NewPooledCodecFactory()
		event := events.NewTextMessageContentEvent("msg", "content")

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			codec, err := factory.CreateCodec(ctx, "application/json", nil, nil)
			if err != nil {
				b.Fatalf("Failed to create encoder: %v", err)
			}

			_, err = codec.Encode(ctx, event)
			if err != nil {
				b.Fatalf("Encoding failed: %v", err)
			}

			if releasable, ok := codec.(encoding.ReleasableEncoder); ok {
				releasable.Release()
			}
		}
	})
}

// BenchmarkCacheEfficiency benchmarks caching efficiency
func BenchmarkCacheEfficiency(b *testing.B) {
	ctx := context.Background()

	baseFactory := encoding.NewDefaultCodecFactory()
	cachingFactory := encoding.NewCachingCodecFactory(baseFactory)

	options := &encoding.EncodingOptions{Pretty: true}

	b.Run("CachedFactory", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := cachingFactory.CreateCodec(ctx, "application/json", options, nil)
			if err != nil {
				b.Fatalf("Failed to create codec: %v", err)
			}
		}
	})

	b.Run("NonCachedFactory", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := baseFactory.CreateCodec(ctx, "application/json", options, nil)
			if err != nil {
				b.Fatalf("Failed to create codec: %v", err)
			}
		}
	})
}

// init ensures benchmarks have reproducible results
func init() {
	rand.Seed(42)
}
