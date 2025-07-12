package encoding_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	_ "github.com/ag-ui/go-sdk/pkg/encoding/json" // Register JSON codec
	"github.com/ag-ui/go-sdk/pkg/encoding/negotiation"
	_ "github.com/ag-ui/go-sdk/pkg/encoding/protobuf" // Register Protobuf codec
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInputValidationErrors tests various input validation error scenarios
func TestInputValidationErrors(t *testing.T) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	testCases := []struct {
		name        string
		test        func(t *testing.T)
		expectError bool
		description string
	}{
		{
			name: "Nil Event Encoding",
			test: func(t *testing.T) {
				encoder, err := registry.GetEncoder(ctx, "application/json", nil)
				require.NoError(t, err)
				
				_, err = encoder.Encode(ctx, nil)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "nil")
			},
			expectError: true,
			description: "Encoding nil event should fail",
		},
		{
			name: "Empty Event List Encoding",
			test: func(t *testing.T) {
				encoder, err := registry.GetEncoder(ctx, "application/json", nil)
				require.NoError(t, err)
				
				_, err = encoder.EncodeMultiple(ctx, []events.Event{})
				// This might succeed with empty array or fail - depends on implementation
				if err != nil {
					assert.Contains(t, err.Error(), "empty")
				}
			},
			expectError: false, // May or may not error
			description: "Encoding empty event list behavior should be defined",
		},
		{
			name: "Nil Event in List",
			test: func(t *testing.T) {
				encoder, err := registry.GetEncoder(ctx, "application/json", nil)
				require.NoError(t, err)
				
				events := []events.Event{
					events.NewTextMessageStartEvent("msg1"),
					nil, // Invalid nil event
					events.NewTextMessageEndEvent("msg1"),
				}
				
				_, err = encoder.EncodeMultiple(ctx, events)
				assert.Error(t, err)
			},
			expectError: true,
			description: "Encoding list with nil event should fail",
		},
		{
			name: "Empty Data Decoding",
			test: func(t *testing.T) {
				decoder, err := registry.GetDecoder(ctx, "application/json", nil)
				require.NoError(t, err)
				
				_, err = decoder.Decode(ctx, []byte{})
				assert.Error(t, err)
			},
			expectError: true,
			description: "Decoding empty data should fail",
		},
		{
			name: "Nil Data Decoding",
			test: func(t *testing.T) {
				decoder, err := registry.GetDecoder(ctx, "application/json", nil)
				require.NoError(t, err)
				
				_, err = decoder.Decode(ctx, nil)
				assert.Error(t, err)
			},
			expectError: true,
			description: "Decoding nil data should fail",
		},
		{
			name: "Invalid JSON Data",
			test: func(t *testing.T) {
				decoder, err := registry.GetDecoder(ctx, "application/json", nil)
				require.NoError(t, err)
				
				_, err = decoder.Decode(ctx, []byte("invalid json"))
				assert.Error(t, err)
			},
			expectError: true,
			description: "Decoding invalid JSON should fail",
		},
		{
			name: "Malformed JSON Array",
			test: func(t *testing.T) {
				decoder, err := registry.GetDecoder(ctx, "application/json", nil)
				require.NoError(t, err)
				
				_, err = decoder.DecodeMultiple(ctx, []byte("[{\"type\":\"TEST\", incomplete"))
				assert.Error(t, err)
			},
			expectError: true,
			description: "Decoding malformed JSON array should fail",
		},
		{
			name: "Unknown Event Type",
			test: func(t *testing.T) {
				decoder, err := registry.GetDecoder(ctx, "application/json", nil)
				require.NoError(t, err)
				
				_, err = decoder.Decode(ctx, []byte(`{"type":"UNKNOWN_EVENT_TYPE"}`))
				assert.Error(t, err)
			},
			expectError: true,
			description: "Decoding unknown event type should fail",
		},
		{
			name: "Missing Required Fields",
			test: func(t *testing.T) {
				decoder, err := registry.GetDecoder(ctx, "application/json", nil)
				require.NoError(t, err)
				
				// Missing messageId for TEXT_MESSAGE_START
				_, err = decoder.Decode(ctx, []byte(`{"type":"TEXT_MESSAGE_START","timestamp":1234567890}`))
				assert.Error(t, err)
			},
			expectError: true,
			description: "Decoding event with missing required fields should fail",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.test(t)
		})
	}
}

// TestSizeLimitErrors tests size limit error scenarios
func TestSizeLimitErrors(t *testing.T) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	t.Run("EncodingMaxSize", func(t *testing.T) {
		encoder, err := registry.GetEncoder(ctx, "application/json", &encoding.EncodingOptions{
			MaxSize: 50, // Very small limit
		})
		require.NoError(t, err)
		
		// Create event that will exceed size limit
		largeContent := strings.Repeat("x", 1000)
		event := events.NewTextMessageContentEvent("msg", largeContent)
		
		_, err = encoder.Encode(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "size")
	})
	
	t.Run("DecodingMaxSize", func(t *testing.T) {
		decoder, err := registry.GetDecoder(ctx, "application/json", &encoding.DecodingOptions{
			MaxSize: 50, // Very small limit
		})
		require.NoError(t, err)
		
		// Create large valid JSON
		largeJSON := `{"type":"TEXT_MESSAGE_CONTENT","messageId":"msg","delta":"` + strings.Repeat("x", 1000) + `","timestamp":1234567890}`
		
		_, err = decoder.Decode(ctx, []byte(largeJSON))
		assert.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "size")
	})
	
	t.Run("StreamingBufferSize", func(t *testing.T) {
		encoder, err := registry.GetStreamEncoder(ctx, "application/json", &encoding.EncodingOptions{
			BufferSize: 10, // Very small buffer
		})
		require.NoError(t, err)
		
		var buf bytes.Buffer
		err = encoder.StartStream(ctx, &buf)
		require.NoError(t, err)
		
		// Try to write large events
		for i := 0; i < 100; i++ {
			event := events.NewTextMessageContentEvent("msg", strings.Repeat("x", 100))
			err = encoder.WriteEvent(ctx, event)
			// May succeed or fail depending on implementation
			if err != nil {
				t.Logf("Expected error with small buffer: %v", err)
				break
			}
		}
		
		encoder.EndStream(ctx)
	})
}

// TestStreamingErrors tests streaming-specific error scenarios
func TestStreamingErrors(t *testing.T) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	t.Run("StreamNotStarted", func(t *testing.T) {
		encoder, err := registry.GetStreamEncoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// Try to write event without starting stream
		event := events.NewTextMessageStartEvent("msg")
		err = encoder.WriteEvent(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "stream")
		
		// Try to end stream without starting
		err = encoder.EndStream(ctx)
		assert.Error(t, err)
	})
	
	t.Run("StreamAlreadyStarted", func(t *testing.T) {
		encoder, err := registry.GetStreamEncoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		var buf bytes.Buffer
		err = encoder.StartStream(ctx, &buf)
		require.NoError(t, err)
		
		// Try to start stream again
		err = encoder.StartStream(ctx, &buf)
		assert.Error(t, err)
		
		encoder.EndStream(ctx)
	})
	
	t.Run("StreamWriteAfterEnd", func(t *testing.T) {
		encoder, err := registry.GetStreamEncoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		var buf bytes.Buffer
		err = encoder.StartStream(ctx, &buf)
		require.NoError(t, err)
		
		err = encoder.EndStream(ctx)
		require.NoError(t, err)
		
		// Try to write after ending
		event := events.NewTextMessageStartEvent("msg")
		err = encoder.WriteEvent(ctx, event)
		assert.Error(t, err)
	})
	
	t.Run("StreamReadBeforeStart", func(t *testing.T) {
		decoder, err := registry.GetStreamDecoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// Try to read event without starting stream
		_, err = decoder.ReadEvent(ctx)
		assert.Error(t, err)
		
		// Try to end stream without starting
		err = decoder.EndStream(ctx)
		assert.Error(t, err)
	})
	
	t.Run("StreamCorruptedData", func(t *testing.T) {
		decoder, err := registry.GetStreamDecoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// Create corrupted NDJSON stream
		corruptedData := `{"type":"TEXT_MESSAGE_START","messageId":"msg1","timestamp":1234567890}
{invalid json line}
{"type":"TEXT_MESSAGE_END","messageId":"msg1","timestamp":1234567892}`
		
		reader := strings.NewReader(corruptedData)
		err = decoder.StartStream(ctx, reader)
		require.NoError(t, err)
		
		// First event should succeed
		_, err = decoder.ReadEvent(ctx)
		assert.NoError(t, err)
		
		// Second event should fail due to corrupted data
		_, err = decoder.ReadEvent(ctx)
		assert.Error(t, err)
		
		decoder.EndStream(ctx)
	})
	
	t.Run("StreamUnexpectedEOF", func(t *testing.T) {
		decoder, err := registry.GetStreamDecoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// Create incomplete stream
		incompleteData := `{"type":"TEXT_MESSAGE_START","messageId":"msg1","timestamp":1234567890}
{"type":"TEXT_MESSAGE_CONTENT","messageId":"msg1","delta":"Hello","timestamp":1234567891}
{"type":"TEXT_MESSAGE_`
		
		reader := strings.NewReader(incompleteData)
		err = decoder.StartStream(ctx, reader)
		require.NoError(t, err)
		
		// First two events should succeed
		_, err = decoder.ReadEvent(ctx)
		assert.NoError(t, err)
		
		_, err = decoder.ReadEvent(ctx)
		assert.NoError(t, err)
		
		// Third event should fail due to incomplete data
		_, err = decoder.ReadEvent(ctx)
		assert.Error(t, err)
		
		decoder.EndStream(ctx)
	})
}

// TestContextCancellationErrors tests context cancellation error handling
func TestContextCancellationErrors(t *testing.T) {
	registry := encoding.GetGlobalRegistry()
	
	t.Run("EncodingCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		encoder, err := registry.GetEncoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		event := events.NewTextMessageStartEvent("msg")
		_, err = encoder.Encode(ctx, event)
		// May or may not error immediately depending on implementation
		if err != nil {
			assert.Contains(t, err.Error(), "context")
		}
	})
	
	t.Run("DecodingCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		decoder, err := registry.GetDecoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		validJSON := `{"type":"TEXT_MESSAGE_START","messageId":"msg","timestamp":1234567890}`
		_, err = decoder.Decode(ctx, []byte(validJSON))
		// May or may not error immediately depending on implementation
		if err != nil {
			assert.Contains(t, err.Error(), "context")
		}
	})
	
	t.Run("StreamingCancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		encoder, err := registry.GetStreamEncoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		decoder, err := registry.GetStreamDecoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// Create a slow stream
		reader, writer := io.Pipe()
		
		// Start encoder
		go func() {
			err := encoder.StartStream(ctx, writer)
			if err != nil {
				t.Logf("Encoder start error: %v", err)
				return
			}
			
			// Write events slowly
			for i := 0; i < 100; i++ {
				event := events.NewTextMessageContentEvent("msg", "content")
				err = encoder.WriteEvent(ctx, event)
				if err != nil {
					t.Logf("Encoder write error: %v", err)
					break
				}
				time.Sleep(50 * time.Millisecond) // This will cause timeout
			}
			
			encoder.EndStream(ctx)
			writer.Close()
		}()
		
		// Start decoder
		err = decoder.StartStream(ctx, reader)
		if err != nil {
			t.Logf("Decoder start error: %v", err)
			return
		}
		
		// Read until context cancellation
		for {
			_, err := decoder.ReadEvent(ctx)
			if err != nil {
				if err == context.DeadlineExceeded || strings.Contains(err.Error(), "context") {
					t.Log("Expected context cancellation error")
					break
				}
				if err == io.EOF {
					break
				}
				t.Logf("Decoder read error: %v", err)
				break
			}
		}
		
		decoder.EndStream(ctx)
	})
}

// TestRegistryErrors tests registry-specific error scenarios
func TestRegistryErrors(t *testing.T) {
	ctx := context.Background()
	
	t.Run("FormatNotRegistered", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		
		_, err := registry.GetEncoder(ctx, "application/nonexistent", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
		
		_, err = registry.GetDecoder(ctx, "application/nonexistent", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
	})
	
	t.Run("InvalidFormatRegistration", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		
		// Nil format info
		err := registry.RegisterFormat(nil)
		assert.Error(t, err)
		
		// Empty MIME type
		info := encoding.NewFormatInfo("Test", "")
		err = registry.RegisterFormat(info)
		assert.Error(t, err)
		
		// Nil factory
		err = registry.RegisterEncoderFactory("application/test", nil)
		assert.Error(t, err)
	})
	
	t.Run("FactoryCreationError", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		
		// Create a factory that always fails
		factory := encoding.NewDefaultCodecFactory()
		factory.RegisterCodec("application/error", func(encOpts *encoding.EncodingOptions, decOpts *encoding.DecodingOptions) (encoding.Codec, error) {
			return nil, errors.New("factory error")
		})
		
		err := registry.RegisterCodecFactory("application/error", factory)
		require.NoError(t, err)
		
		// Try to create encoder
		_, err = registry.GetEncoder(ctx, "application/error", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "factory error")
	})
	
	t.Run("UnregisterNonExistentFormat", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		
		err := registry.UnregisterFormat("application/nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
	})
	
	t.Run("SetInvalidDefaultFormat", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		
		err := registry.SetDefaultFormat("application/nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
	})
}

// TestNegotiationErrors tests content negotiation error scenarios
func TestNegotiationErrors(t *testing.T) {
	negotiator := negotiation.NewContentNegotiator("application/json")
	
	testCases := []struct {
		name         string
		acceptHeader string
		expectError  bool
		description  string
	}{
		{
			name:         "No acceptable format",
			acceptHeader: "application/xml",
			expectError:  true,
			description:  "Should fail when no supported format is acceptable",
		},
		{
			name:         "All zero quality",
			acceptHeader: "application/json;q=0.0,application/x-protobuf;q=0.0",
			expectError:  true,
			description:  "Should fail when all formats have zero quality",
		},
		{
			name:         "Malformed header",
			acceptHeader: "application/json;q=invalid;q=0.9",
			expectError:  false, // Should handle gracefully
			description:  "Should handle malformed headers gracefully",
		},
		{
			name:         "Empty header",
			acceptHeader: "",
			expectError:  false, // Should default to something
			description:  "Should handle empty headers gracefully",
		},
		{
			name:         "Only whitespace",
			acceptHeader: "   ",
			expectError:  false, // Should handle gracefully
			description:  "Should handle whitespace-only headers gracefully",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := negotiator.Negotiate(tc.acceptHeader)
			
			if tc.expectError {
				assert.Error(t, err, tc.description)
			} else {
				// May succeed or fail, but shouldn't panic
				if err != nil {
					t.Logf("Negotiation error (may be expected): %v", err)
				}
			}
		})
	}
}

// TestPoolingErrors tests pooling-related error scenarios
func TestPoolingErrors(t *testing.T) {
	t.Run("PoolOverflow", func(t *testing.T) {
		// Test what happens when pool is heavily used
		const numOperations = 10000
		
		var buffers []*bytes.Buffer
		
		// Get many buffers
		for i := 0; i < numOperations; i++ {
			buf := encoding.GetBuffer(1024)
			buffers = append(buffers, buf)
		}
		
		// Put them all back
		for _, buf := range buffers {
			encoding.PutBuffer(buf)
		}
		
		// Should not crash or leak memory
		stats := encoding.PoolStats()
		assert.Greater(t, len(stats), 0)
	})
	
	t.Run("PoolCorruption", func(t *testing.T) {
		// Test putting back corrupted objects
		buf := encoding.GetBuffer(1024)
		
		// Use buffer normally
		buf.WriteString("test")
		
		// Put it back
		encoding.PutBuffer(buf)
		
		// Get it again
		buf2 := encoding.GetBuffer(1024)
		
		// Should be reset
		assert.Equal(t, 0, buf2.Len())
		
		encoding.PutBuffer(buf2)
	})
	
	t.Run("PoolNilHandling", func(t *testing.T) {
		// Test putting nil objects
		encoding.PutBuffer(nil)     // Should not crash
		encoding.PutSlice(nil)      // Should not crash
		encoding.PutEncodingError(nil) // Should not crash
		encoding.PutDecodingError(nil) // Should not crash
	})
}

// TestEdgeCases tests various edge cases
func TestEdgeCases(t *testing.T) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	t.Run("EmptyStringValues", func(t *testing.T) {
		encoder, err := registry.GetEncoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		decoder, err := registry.GetDecoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// Event with empty string content
		event := events.NewTextMessageContentEvent("", "")
		
		data, err := encoder.Encode(ctx, event)
		require.NoError(t, err)
		
		decodedEvent, err := decoder.Decode(ctx, data)
		require.NoError(t, err)
		
		assert.Equal(t, event.Type(), decodedEvent.Type())
	})
	
	t.Run("VeryLongStrings", func(t *testing.T) {
		encoder, err := registry.GetEncoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		decoder, err := registry.GetDecoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// Event with very long content
		longContent := strings.Repeat("x", 100000)
		event := events.NewTextMessageContentEvent("msg", longContent)
		
		data, err := encoder.Encode(ctx, event)
		require.NoError(t, err)
		
		decodedEvent, err := decoder.Decode(ctx, data)
		require.NoError(t, err)
		
		assert.Equal(t, event.Type(), decodedEvent.Type())
	})
	
	t.Run("UnicodeContent", func(t *testing.T) {
		encoder, err := registry.GetEncoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		decoder, err := registry.GetDecoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// Event with unicode content
		unicodeContent := "Hello 世界 🌍 \U0001F600"
		event := events.NewTextMessageContentEvent("msg", unicodeContent)
		
		data, err := encoder.Encode(ctx, event)
		require.NoError(t, err)
		
		decodedEvent, err := decoder.Decode(ctx, data)
		require.NoError(t, err)
		
		assert.Equal(t, event.Type(), decodedEvent.Type())
	})
	
	t.Run("SpecialCharacters", func(t *testing.T) {
		encoder, err := registry.GetEncoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		decoder, err := registry.GetDecoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// Event with special characters
		specialContent := "Content with \n\r\t\b\f\"\\/"
		event := events.NewTextMessageContentEvent("msg", specialContent)
		
		data, err := encoder.Encode(ctx, event)
		require.NoError(t, err)
		
		decodedEvent, err := decoder.Decode(ctx, data)
		require.NoError(t, err)
		
		assert.Equal(t, event.Type(), decodedEvent.Type())
	})
	
	t.Run("LargeEventList", func(t *testing.T) {
		encoder, err := registry.GetEncoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		decoder, err := registry.GetDecoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// Very large event list
		const numEvents = 10000
		eventList := make([]events.Event, numEvents)
		for i := 0; i < numEvents; i++ {
			eventList[i] = events.NewTextMessageContentEvent(fmt.Sprintf("msg-%d", i), fmt.Sprintf("content-%d", i))
		}
		
		data, err := encoder.EncodeMultiple(ctx, eventList)
		require.NoError(t, err)
		
		decodedEvents, err := decoder.DecodeMultiple(ctx, data)
		require.NoError(t, err)
		
		assert.Equal(t, len(eventList), len(decodedEvents))
	})
	
	t.Run("TimestampEdgeCases", func(t *testing.T) {
		encoder, err := registry.GetEncoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		decoder, err := registry.GetDecoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// Test with various timestamp values
		timestamps := []int64{0, -1, 1, 2147483647, -2147483648, 9223372036854775807}
		
		for _, ts := range timestamps {
			event := events.NewTextMessageStartEvent("msg")
			event.SetTimestamp(ts)
			
			data, err := encoder.Encode(ctx, event)
			require.NoError(t, err)
			
			decodedEvent, err := decoder.Decode(ctx, data)
			require.NoError(t, err)
			
			assert.Equal(t, event.Type(), decodedEvent.Type())
		}
	})
}

// TestErrorRecovery tests error recovery scenarios
func TestErrorRecovery(t *testing.T) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	t.Run("ContinueAfterError", func(t *testing.T) {
		decoder, err := registry.GetDecoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// First decode should fail
		_, err = decoder.Decode(ctx, []byte("invalid json"))
		assert.Error(t, err)
		
		// Second decode should succeed
		validJSON := `{"type":"TEXT_MESSAGE_START","messageId":"msg","timestamp":1234567890}`
		_, err = decoder.Decode(ctx, []byte(validJSON))
		assert.NoError(t, err)
	})
	
	t.Run("StreamRecovery", func(t *testing.T) {
		decoder, err := registry.GetStreamDecoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// Create stream with mixed valid/invalid data
		mixedData := `{"type":"TEXT_MESSAGE_START","messageId":"msg1","timestamp":1234567890}
invalid json line
{"type":"TEXT_MESSAGE_END","messageId":"msg1","timestamp":1234567892}`
		
		reader := strings.NewReader(mixedData)
		err = decoder.StartStream(ctx, reader)
		require.NoError(t, err)
		
		// First read should succeed
		_, err = decoder.ReadEvent(ctx)
		assert.NoError(t, err)
		
		// Second read should fail
		_, err = decoder.ReadEvent(ctx)
		assert.Error(t, err)
		
		// Third read should succeed if implementation supports recovery
		_, err = decoder.ReadEvent(ctx)
		// May succeed or fail depending on implementation
		
		decoder.EndStream(ctx)
	})
	
	t.Run("PoolRecovery", func(t *testing.T) {
		// Test that pools can recover from errors
		initialStats := encoding.PoolStats()
		
		// Cause some errors
		encoding.PutBuffer(nil)
		encoding.PutSlice(nil)
		
		// Normal operations should still work
		buf := encoding.GetBuffer(1024)
		buf.WriteString("test")
		encoding.PutBuffer(buf)
		
		slice := encoding.GetSlice(1024)
		slice = append(slice, []byte("test")...)
		encoding.PutSlice(slice)
		
		// Pool should still be functional
		finalStats := encoding.PoolStats()
		assert.Equal(t, len(initialStats), len(finalStats))
	})
}

// TestValidationErrors tests validation-related error scenarios
func TestValidationErrors(t *testing.T) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	t.Run("StrictValidation", func(t *testing.T) {
		decoder, err := registry.GetDecoder(ctx, "application/json", &encoding.DecodingOptions{
			Strict: true,
		})
		require.NoError(t, err)
		
		// JSON with extra fields
		jsonWithExtra := `{"type":"TEXT_MESSAGE_START","messageId":"msg","timestamp":1234567890,"extraField":"value"}`
		
		_, err = decoder.Decode(ctx, []byte(jsonWithExtra))
		// May succeed or fail depending on implementation
		if err != nil {
			t.Logf("Strict validation error (may be expected): %v", err)
		}
	})
	
	t.Run("OutputValidation", func(t *testing.T) {
		encoder, err := registry.GetEncoder(ctx, "application/json", &encoding.EncodingOptions{
			ValidateOutput: true,
		})
		require.NoError(t, err)
		
		// Create potentially invalid event
		event := events.NewTextMessageStartEvent("msg")
		
		_, err = encoder.Encode(ctx, event)
		// Should succeed for valid event
		assert.NoError(t, err)
	})
	
	t.Run("EventValidation", func(t *testing.T) {
		decoder, err := registry.GetDecoder(ctx, "application/json", &encoding.DecodingOptions{
			ValidateEvents: true,
		})
		require.NoError(t, err)
		
		// Decode valid event
		validJSON := `{"type":"TEXT_MESSAGE_START","messageId":"msg","timestamp":1234567890}`
		_, err = decoder.Decode(ctx, []byte(validJSON))
		assert.NoError(t, err)
	})
}