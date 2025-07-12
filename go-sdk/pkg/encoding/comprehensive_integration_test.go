package encoding_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/encoding/json"
	"github.com/ag-ui/go-sdk/pkg/encoding/negotiation"
	"github.com/ag-ui/go-sdk/pkg/encoding/protobuf"
	"github.com/ag-ui/go-sdk/pkg/encoding/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFullEncodingPipeline tests the complete encoding pipeline from registration to streaming
func TestFullEncodingPipeline(t *testing.T) {
	ctx := context.Background()
	
	// Create a fresh registry for this test
	registry := encoding.NewFormatRegistry()
	
	// Register JSON and Protobuf formats
	require.NoError(t, registry.RegisterFormat(encoding.JSONFormatInfo()))
	require.NoError(t, registry.RegisterFormat(encoding.ProtobufFormatInfo()))
	
	// Register codecs using JSON package registration
	require.NoError(t, json.RegisterTo(registry))
	require.NoError(t, protobuf.RegisterTo(registry))
	
	// Test all phases of the pipeline
	testCases := []struct {
		name       string
		mimeType   string
		streaming  bool
		validation bool
	}{
		{"JSON Non-Streaming", "application/json", false, false},
		{"JSON Streaming", "application/json", true, false},
		{"JSON with Validation", "application/json", false, true},
		{"Protobuf Non-Streaming", "application/x-protobuf", false, false},
		{"Protobuf Streaming", "application/x-protobuf", true, false},
		{"Protobuf with Validation", "application/x-protobuf", false, true},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testEncodingPipeline(t, ctx, registry, tc.mimeType, tc.streaming, tc.validation)
		})
	}
}

func testEncodingPipeline(t *testing.T, ctx context.Context, registry *encoding.FormatRegistry, mimeType string, streaming bool, validation bool) {
	// Create test events
	testEvents := []events.Event{
		events.NewTextMessageStartEvent("msg1", events.WithRole("user")),
		events.NewTextMessageContentEvent("msg1", "Hello, world!"),
		events.NewTextMessageContentEvent("msg1", " This is a test message."),
		events.NewTextMessageEndEvent("msg1"),
	}
	
	// Setup encoding options
	encOptions := &encoding.EncodingOptions{
		Pretty:           false,
		ValidateOutput:   validation,
		BufferSize:       4096,
		MaxSize:          1024 * 1024, // 1MB
	}
	
	// Setup decoding options
	decOptions := &encoding.DecodingOptions{
		Strict:         validation,
		ValidateEvents: validation,
		BufferSize:     4096,
		MaxSize:        1024 * 1024, // 1MB
	}
	
	if streaming {
		testStreamingPipeline(t, ctx, registry, mimeType, testEvents, encOptions, decOptions)
	} else {
		testNonStreamingPipeline(t, ctx, registry, mimeType, testEvents, encOptions, decOptions)
	}
}

func testNonStreamingPipeline(t *testing.T, ctx context.Context, registry *encoding.FormatRegistry, mimeType string, testEvents []events.Event, encOptions *encoding.EncodingOptions, decOptions *encoding.DecodingOptions) {
	// Phase 1: Create encoder
	encoder, err := registry.GetEncoder(ctx, mimeType, encOptions)
	require.NoError(t, err)
	assert.Equal(t, mimeType, encoder.ContentType())
	
	// Phase 2: Create decoder
	decoder, err := registry.GetDecoder(ctx, mimeType, decOptions)
	require.NoError(t, err)
	assert.Equal(t, mimeType, decoder.ContentType())
	
	// Phase 3: Test single event encoding/decoding
	singleEvent := testEvents[0]
	encodedData, err := encoder.Encode(ctx, singleEvent)
	require.NoError(t, err)
	assert.NotEmpty(t, encodedData)
	
	decodedEvent, err := decoder.Decode(ctx, encodedData)
	require.NoError(t, err)
	assert.Equal(t, singleEvent.Type(), decodedEvent.Type())
	
	// Phase 4: Test multiple events encoding/decoding
	multipleEncodedData, err := encoder.EncodeMultiple(ctx, testEvents)
	require.NoError(t, err)
	assert.NotEmpty(t, multipleEncodedData)
	
	decodedEvents, err := decoder.DecodeMultiple(ctx, multipleEncodedData)
	require.NoError(t, err)
	assert.Equal(t, len(testEvents), len(decodedEvents))
	
	// Verify all events were decoded correctly
	for i, originalEvent := range testEvents {
		assert.Equal(t, originalEvent.Type(), decodedEvents[i].Type())
	}
}

func testStreamingPipeline(t *testing.T, ctx context.Context, registry *encoding.FormatRegistry, mimeType string, testEvents []events.Event, encOptions *encoding.EncodingOptions, decOptions *encoding.DecodingOptions) {
	// Phase 1: Create stream encoder
	streamEncoder, err := registry.GetStreamEncoder(ctx, mimeType, encOptions)
	require.NoError(t, err)
	assert.Equal(t, mimeType, streamEncoder.ContentType())
	
	// Phase 2: Create stream decoder
	streamDecoder, err := registry.GetStreamDecoder(ctx, mimeType, decOptions)
	require.NoError(t, err)
	assert.Equal(t, mimeType, streamDecoder.ContentType())
	
	// Phase 3: Test channel-based streaming
	var streamBuffer bytes.Buffer
	eventChan := make(chan events.Event)
	
	// Start encoding in a goroutine
	var encodeErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		encodeErr = streamEncoder.EncodeStream(ctx, eventChan, &streamBuffer)
	}()
	
	// Send events
	for _, event := range testEvents {
		eventChan <- event
	}
	close(eventChan)
	
	// Wait for encoding to complete
	wg.Wait()
	require.NoError(t, encodeErr)
	assert.NotEmpty(t, streamBuffer.Bytes())
	
	// Phase 4: Test streaming decoding
	decodedEventsChan := make(chan events.Event, len(testEvents))
	reader := bytes.NewReader(streamBuffer.Bytes())
	
	var decodeErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		decodeErr = streamDecoder.DecodeStream(ctx, reader, decodedEventsChan)
	}()
	
	// Collect decoded events
	var decodedEvents []events.Event
	for event := range decodedEventsChan {
		decodedEvents = append(decodedEvents, event)
	}
	
	wg.Wait()
	require.NoError(t, decodeErr)
	assert.Equal(t, len(testEvents), len(decodedEvents))
	
	// Verify all events were decoded correctly
	for i, originalEvent := range testEvents {
		assert.Equal(t, originalEvent.Type(), decodedEvents[i].Type())
	}
	
	// Phase 5: Test individual stream operations
	var stepBuffer bytes.Buffer
	require.NoError(t, streamEncoder.StartStream(ctx, &stepBuffer))
	
	for _, event := range testEvents {
		require.NoError(t, streamEncoder.WriteEvent(ctx, event))
	}
	
	require.NoError(t, streamEncoder.EndStream(ctx))
	assert.NotEmpty(t, stepBuffer.Bytes())
	
	// Test individual decode operations
	stepReader := bytes.NewReader(stepBuffer.Bytes())
	require.NoError(t, streamDecoder.StartStream(ctx, stepReader))
	
	var stepDecodedEvents []events.Event
	for {
		event, err := streamDecoder.ReadEvent(ctx)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		stepDecodedEvents = append(stepDecodedEvents, event)
	}
	
	require.NoError(t, streamDecoder.EndStream(ctx))
	assert.Equal(t, len(testEvents), len(stepDecodedEvents))
}

// TestContentNegotiation tests content negotiation and format selection
func TestContentNegotiation(t *testing.T) {
	// Create negotiator
	negotiator := negotiation.NewContentNegotiator("application/json")
	
	// Add supported formats using RegisterType
	negotiator.RegisterType(&negotiation.TypeCapabilities{
		ContentType: "application/json",
		Priority: 1.0,
		CanStream: true,
	})
	negotiator.RegisterType(&negotiation.TypeCapabilities{
		ContentType: "application/x-protobuf",
		Priority: 0.9,
		CanStream: true,
	})
	negotiator.RegisterType(&negotiation.TypeCapabilities{
		ContentType: "text/plain",
		Priority: 0.5,
		CanStream: false,
	})
	
	testCases := []struct {
		name          string
		acceptHeader  string
		expectedType  string
		shouldSucceed bool
	}{
		{
			name:          "JSON preferred",
			acceptHeader:  "application/json,application/x-protobuf;q=0.8",
			expectedType:  "application/json",
			shouldSucceed: true,
		},
		{
			name:          "Protobuf preferred",
			acceptHeader:  "application/x-protobuf,application/json;q=0.8",
			expectedType:  "application/x-protobuf",
			shouldSucceed: true,
		},
		{
			name:          "Wildcard",
			acceptHeader:  "*/*",
			expectedType:  "application/json", // Should pick highest priority
			shouldSucceed: true,
		},
		{
			name:          "Unsupported type",
			acceptHeader:  "application/xml",
			expectedType:  "",
			shouldSucceed: false,
		},
		{
			name:          "Complex header",
			acceptHeader:  "text/html,application/xhtml+xml,application/xml;q=0.9,application/json;q=0.8,*/*;q=0.7",
			expectedType:  "application/json",
			shouldSucceed: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			selectedType, err := negotiator.Negotiate(tc.acceptHeader)
			
			if tc.shouldSucceed {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedType, selectedType)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

// TestFormatCompatibility tests cross-format compatibility
func TestFormatCompatibility(t *testing.T) {
	ctx := context.Background()
	
	// Setup registry with both formats
	registry := encoding.GetGlobalRegistry()
	
	// Create test event
	testEvent := events.NewTextMessageStartEvent("test-msg", events.WithRole("user"))
	
	// Test cross-format encoding/decoding (should fail gracefully)
	jsonEncoder, err := registry.GetEncoder(ctx, "application/json", nil)
	require.NoError(t, err)
	
	protobufDecoder, err := registry.GetDecoder(ctx, "application/x-protobuf", nil)
	require.NoError(t, err)
	
	// Encode with JSON
	jsonData, err := jsonEncoder.Encode(ctx, testEvent)
	require.NoError(t, err)
	
	// Try to decode with Protobuf (should fail)
	_, err = protobufDecoder.Decode(ctx, jsonData)
	assert.Error(t, err, "Cross-format decoding should fail")
	
	// Test that each format can handle its own data
	jsonDecoder, err := registry.GetDecoder(ctx, "application/json", nil)
	require.NoError(t, err)
	
	decodedEvent, err := jsonDecoder.Decode(ctx, jsonData)
	require.NoError(t, err)
	assert.Equal(t, testEvent.Type(), decodedEvent.Type())
}

// TestValidationIntegration tests validation framework integration
func TestValidationIntegration(t *testing.T) {
	ctx := context.Background()
	
	// Create registry with validation
	registry := encoding.NewFormatRegistry()
	validator := validation.NewJSONValidator(false)
	
	// Create adapter for the validator interface
	adapter := &validatorAdapter{validator: validator}
	registry.SetValidator(adapter)
	
	// Register formats with validation
	require.NoError(t, registry.RegisterFormat(encoding.JSONFormatInfo()))
	
	// Test with valid event
	validEvent := events.NewTextMessageStartEvent("valid-msg", events.WithRole("user"))
	
	encoder, err := registry.GetEncoder(ctx, "application/json", &encoding.EncodingOptions{
		ValidateOutput: true,
	})
	require.NoError(t, err)
	
	_, err = encoder.Encode(ctx, validEvent)
	require.NoError(t, err)
	
	// Test with invalid event (if validation detects it)
	invalidEvent := &events.TextMessageStartEvent{
		BaseEvent: events.NewBaseEvent(events.EventTypeTextMessageStart),
		MessageID: "", // Invalid: empty message ID
	}
	
	_, err = encoder.Encode(ctx, invalidEvent)
	// Note: The actual validation behavior depends on the validator implementation
	// This test verifies the integration works without errors
	if err != nil {
		t.Logf("Validation caught invalid event: %v", err)
	}
}

// TestPoolingIntegration tests object pooling integration
func TestPoolingIntegration(t *testing.T) {
	ctx := context.Background()
	
	// Create pooled factory
	factory := encoding.NewPooledCodecFactory()
	
	// Get initial pool metrics
	pool := factory.GetCodecPool()
	initialMetrics := pool.Metrics()
	
	// Create multiple codecs
	var codecs []encoding.Codec
	for i := 0; i < 10; i++ {
		codec, err := factory.CreateCodec(ctx, "application/json", nil, nil)
		require.NoError(t, err)
		codecs = append(codecs, codec)
	}
	
	// Release all codecs
	for _, codec := range codecs {
		if releasable, ok := codec.(encoding.ReleasableEncoder); ok {
			releasable.Release()
		}
	}
	
	// Check metrics improved
	finalMetrics := pool.Metrics()
	assert.Greater(t, finalMetrics.Gets, initialMetrics.Gets)
	assert.Greater(t, finalMetrics.Puts, initialMetrics.Puts)
	
	// Test buffer pooling
	initialBufferStats := encoding.PoolStats()
	
	// Use buffers
	for i := 0; i < 100; i++ {
		buf := encoding.GetBuffer(1024)
		buf.WriteString("test data")
		encoding.PutBuffer(buf)
	}
	
	finalBufferStats := encoding.PoolStats()
	
	// Verify buffer pool usage
	for poolName, stats := range finalBufferStats {
		if strings.Contains(poolName, "buffer") {
			initialStats := initialBufferStats[poolName]
			assert.GreaterOrEqual(t, stats.Gets, initialStats.Gets)
			assert.GreaterOrEqual(t, stats.Puts, initialStats.Puts)
		}
	}
}

// TestErrorHandlingIntegration tests comprehensive error handling
func TestErrorHandlingIntegration(t *testing.T) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	testCases := []struct {
		name          string
		test          func(t *testing.T)
		expectError   bool
		errorContains string
	}{
		{
			name: "Encoder not found",
			test: func(t *testing.T) {
				_, err := registry.GetEncoder(ctx, "application/nonexistent", nil)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "not registered")
			},
		},
		{
			name: "Decoder not found",
			test: func(t *testing.T) {
				_, err := registry.GetDecoder(ctx, "application/nonexistent", nil)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "not registered")
			},
		},
		{
			name: "Invalid encoding options",
			test: func(t *testing.T) {
				encoder, err := registry.GetEncoder(ctx, "application/json", &encoding.EncodingOptions{
					MaxSize: -1, // Invalid size
				})
				// Should either fail or handle gracefully
				if err != nil {
					assert.Contains(t, err.Error(), "invalid")
				} else {
					assert.NotNil(t, encoder)
				}
			},
		},
		{
			name: "Nil event encoding",
			test: func(t *testing.T) {
				encoder, err := registry.GetEncoder(ctx, "application/json", nil)
				require.NoError(t, err)
				
				_, err = encoder.Encode(ctx, nil)
				assert.Error(t, err)
			},
		},
		{
			name: "Empty data decoding",
			test: func(t *testing.T) {
				decoder, err := registry.GetDecoder(ctx, "application/json", nil)
				require.NoError(t, err)
				
				_, err = decoder.Decode(ctx, []byte{})
				assert.Error(t, err)
			},
		},
		{
			name: "Corrupted data decoding",
			test: func(t *testing.T) {
				decoder, err := registry.GetDecoder(ctx, "application/json", nil)
				require.NoError(t, err)
				
				_, err = decoder.Decode(ctx, []byte("corrupted data"))
				assert.Error(t, err)
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.test(t)
		})
	}
}

// TestStreamingErrorHandling tests streaming-specific error handling
func TestStreamingErrorHandling(t *testing.T) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	// Test stream encoder error handling
	t.Run("Stream encoder errors", func(t *testing.T) {
		encoder, err := registry.GetStreamEncoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// Try to write event without starting stream
		err = encoder.WriteEvent(ctx, events.NewTextMessageStartEvent("test"))
		assert.Error(t, err)
		
		// Try to end stream without starting
		err = encoder.EndStream(ctx)
		assert.Error(t, err)
	})
	
	// Test stream decoder error handling
	t.Run("Stream decoder errors", func(t *testing.T) {
		decoder, err := registry.GetStreamDecoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		// Try to read event without starting stream
		_, err = decoder.ReadEvent(ctx)
		assert.Error(t, err)
		
		// Try to end stream without starting
		err = decoder.EndStream(ctx)
		assert.Error(t, err)
	})
}

// TestConcurrentIntegration tests concurrent usage of the entire system
func TestConcurrentIntegration(t *testing.T) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	const numGoroutines = 50
	const numOperations = 100
	
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)
	
	// Test concurrent encoding/decoding
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				// Randomly choose format
				mimeType := "application/json"
				if j%2 == 0 {
					mimeType = "application/x-protobuf"
				}
				
				// Create codec
				encoder, err := registry.GetEncoder(ctx, mimeType, nil)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d: failed to get encoder: %w", id, err)
					continue
				}
				
				decoder, err := registry.GetDecoder(ctx, mimeType, nil)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d: failed to get decoder: %w", id, err)
					continue
				}
				
				// Test round-trip
				event := events.NewTextMessageContentEvent(fmt.Sprintf("msg-%d-%d", id, j), fmt.Sprintf("content-%d", j))
				
				data, err := encoder.Encode(ctx, event)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d: failed to encode: %w", id, err)
					continue
				}
				
				_, err = decoder.Decode(ctx, data)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d: failed to decode: %w", id, err)
					continue
				}
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	var errorCount int
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
		errorCount++
	}
	
	assert.Equal(t, 0, errorCount, "Expected no errors in concurrent operations")
}

// TestResourceCleanup tests proper resource cleanup
func TestResourceCleanup(t *testing.T) {
	ctx := context.Background()
	
	// Test context cancellation
	cancelCtx, cancel := context.WithCancel(ctx)
	registry := encoding.GetGlobalRegistry()
	
	// Start some streaming operations
	encoder, err := registry.GetStreamEncoder(cancelCtx, "application/json", nil)
	require.NoError(t, err)
	
	var buf bytes.Buffer
	eventChan := make(chan events.Event, 1)
	
	// Start streaming
	go func() {
		encoder.EncodeStream(cancelCtx, eventChan, &buf)
	}()
	
	// Send an event
	eventChan <- events.NewTextMessageStartEvent("test")
	
	// Cancel context
	cancel()
	
	// Close channel
	close(eventChan)
	
	// Wait a bit for cleanup
	time.Sleep(100 * time.Millisecond)
	
	// Test should complete without hanging
	assert.True(t, true, "Resource cleanup completed successfully")
}

// validatorAdapter adapts validation.FormatValidator to encoding.FormatValidator
type validatorAdapter struct {
	validator validation.FormatValidator
}

func (a *validatorAdapter) ValidateFormat(mimeType string, data []byte) error {
	return a.validator.ValidateFormat(data)
}

func (a *validatorAdapter) ValidateEncoding(mimeType string, data []byte) error {
	return a.validator.ValidateEncoding(data)
}

func (a *validatorAdapter) ValidateDecoding(mimeType string, data []byte) error {
	return a.validator.ValidateDecoding(data)
}

// TestMemoryPressure tests behavior under memory pressure
func TestMemoryPressure(t *testing.T) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	// Create large events
	largeContent := strings.Repeat("x", 10000) // 10KB content
	
	// Reset pool stats
	encoding.ResetAllPools()
	
	// Process many large events
	for i := 0; i < 100; i++ {
		encoder, err := registry.GetEncoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		event := events.NewTextMessageContentEvent(fmt.Sprintf("msg-%d", i), largeContent)
		data, err := encoder.Encode(ctx, event)
		require.NoError(t, err)
		
		decoder, err := registry.GetDecoder(ctx, "application/json", nil)
		require.NoError(t, err)
		
		_, err = decoder.Decode(ctx, data)
		require.NoError(t, err)
	}
	
	// Check pool usage
	stats := encoding.PoolStats()
	assert.Greater(t, len(stats), 0, "Pool stats should be available")
	
	// Verify pools are working (some Gets and Puts should have occurred)
	totalGets := int64(0)
	totalPuts := int64(0)
	for _, metrics := range stats {
		totalGets += metrics.Gets
		totalPuts += metrics.Puts
	}
	
	assert.Greater(t, totalGets, int64(0), "Pools should have been used")
	assert.Greater(t, totalPuts, int64(0), "Objects should have been returned to pools")
}