package encoding_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/encoding/negotiation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContentNegotiationRegression tests for content negotiation regressions
func TestContentNegotiationRegression(t *testing.T) {
	negotiator := negotiation.NewNegotiator()
	
	// Add formats with different priorities
	negotiator.AddFormat("application/json", 1.0)
	negotiator.AddFormat("application/x-protobuf", 0.9)
	negotiator.AddFormat("text/plain", 0.8)
	negotiator.AddFormat("application/xml", 0.7)
	
	testCases := []struct {
		name          string
		acceptHeader  string
		expectedType  string
		shouldSucceed bool
		description   string
	}{
		{
			name:          "Simple JSON preference",
			acceptHeader:  "application/json",
			expectedType:  "application/json",
			shouldSucceed: true,
			description:   "Basic single format acceptance",
		},
		{
			name:          "Multiple formats with quality values",
			acceptHeader:  "application/x-protobuf;q=0.9,application/json;q=1.0,text/plain;q=0.8",
			expectedType:  "application/json",
			shouldSucceed: true,
			description:   "Should select highest quality value",
		},
		{
			name:          "Wildcard acceptance",
			acceptHeader:  "*/*",
			expectedType:  "application/json",
			shouldSucceed: true,
			description:   "Wildcard should select highest priority format",
		},
		{
			name:          "Complex browser-like header",
			acceptHeader:  "text/html,application/xhtml+xml,application/xml;q=0.9,application/json;q=0.8,*/*;q=0.7",
			expectedType:  "application/json",
			shouldSucceed: true,
			description:   "Complex header with unsupported types should fall back to supported type",
		},
		{
			name:          "Quality values with decimals",
			acceptHeader:  "application/json;q=0.95,application/x-protobuf;q=0.99",
			expectedType:  "application/x-protobuf",
			shouldSucceed: true,
			description:   "Should handle decimal quality values correctly",
		},
		{
			name:          "Case insensitive types",
			acceptHeader:  "APPLICATION/JSON",
			expectedType:  "application/json",
			shouldSucceed: true,
			description:   "MIME types should be case insensitive",
		},
		{
			name:          "With charset parameter",
			acceptHeader:  "application/json; charset=utf-8",
			expectedType:  "application/json",
			shouldSucceed: true,
			description:   "Should ignore charset and other parameters",
		},
		{
			name:          "Empty accept header",
			acceptHeader:  "",
			expectedType:  "application/json",
			shouldSucceed: true,
			description:   "Empty header should default to highest priority",
		},
		{
			name:          "Unsupported format only",
			acceptHeader:  "application/octet-stream",
			expectedType:  "",
			shouldSucceed: false,
			description:   "Should fail when no supported format is found",
		},
		{
			name:          "Zero quality value",
			acceptHeader:  "application/json;q=0.0,application/x-protobuf;q=0.9",
			expectedType:  "application/x-protobuf",
			shouldSucceed: true,
			description:   "Should exclude formats with zero quality",
		},
		{
			name:          "Spaces in header",
			acceptHeader:  " application/json ; q=0.8 , application/x-protobuf ; q=0.9 ",
			expectedType:  "application/x-protobuf",
			shouldSucceed: true,
			description:   "Should handle spaces correctly",
		},
		{
			name:          "Malformed quality value",
			acceptHeader:  "application/json;q=invalid,application/x-protobuf",
			expectedType:  "application/x-protobuf",
			shouldSucceed: true,
			description:   "Should handle malformed quality values gracefully",
		},
		{
			name:          "Subtype wildcard",
			acceptHeader:  "application/*",
			expectedType:  "application/json",
			shouldSucceed: true,
			description:   "Should handle subtype wildcards",
		},
		{
			name:          "Multiple same types with different qualities",
			acceptHeader:  "application/json;q=0.8,application/json;q=0.9",
			expectedType:  "application/json",
			shouldSucceed: true,
			description:   "Should handle duplicate types with different qualities",
		},
		{
			name:          "Quality value greater than 1",
			acceptHeader:  "application/json;q=1.5",
			expectedType:  "application/json",
			shouldSucceed: true,
			description:   "Should clamp quality values to 1.0",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			selectedType, err := negotiator.Negotiate(tc.acceptHeader)
			
			if tc.shouldSucceed {
				require.NoError(t, err, tc.description)
				assert.Equal(t, tc.expectedType, selectedType, tc.description)
			} else {
				assert.Error(t, err, tc.description)
			}
		})
	}
}

// TestFormatRegistrationRegression tests for format registration regressions
func TestFormatRegistrationRegression(t *testing.T) {
	registry := encoding.NewFormatRegistry()
	
	// Test case 1: Basic registration
	t.Run("BasicRegistration", func(t *testing.T) {
		info := encoding.NewFormatInfo("JSON", "application/json")
		err := registry.RegisterFormat(info)
		require.NoError(t, err)
		
		// Verify it was registered
		assert.True(t, registry.SupportsFormat("application/json"))
		
		// Verify format info
		retrievedInfo, err := registry.GetFormat("application/json")
		require.NoError(t, err)
		assert.Equal(t, "JSON", retrievedInfo.Name)
		assert.Equal(t, "application/json", retrievedInfo.MIMEType)
	})
	
	// Test case 2: Alias registration
	t.Run("AliasRegistration", func(t *testing.T) {
		info := encoding.NewFormatInfo("JSON", "application/json")
		info.Aliases = []string{"json", "JSON"}
		err := registry.RegisterFormat(info)
		require.NoError(t, err)
		
		// Verify aliases work
		assert.True(t, registry.SupportsFormat("json"))
		assert.True(t, registry.SupportsFormat("JSON"))
		
		// Verify aliases resolve to canonical type
		retrievedInfo, err := registry.GetFormat("json")
		require.NoError(t, err)
		assert.Equal(t, "application/json", retrievedInfo.MIMEType)
	})
	
	// Test case 3: Priority ordering
	t.Run("PriorityOrdering", func(t *testing.T) {
		registry := encoding.NewFormatRegistry() // Fresh registry
		
		// Register in reverse priority order
		info1 := encoding.NewFormatInfo("Format 1", "format/1")
		info1.Priority = 30
		registry.RegisterFormat(info1)
		
		info2 := encoding.NewFormatInfo("Format 2", "format/2")
		info2.Priority = 10
		registry.RegisterFormat(info2)
		
		info3 := encoding.NewFormatInfo("Format 3", "format/3")
		info3.Priority = 20
		registry.RegisterFormat(info3)
		
		// List should be sorted by priority
		formats := registry.ListFormats()
		require.Len(t, formats, 3)
		assert.Equal(t, "format/2", formats[0].MIMEType) // Priority 10
		assert.Equal(t, "format/3", formats[1].MIMEType) // Priority 20
		assert.Equal(t, "format/1", formats[2].MIMEType) // Priority 30
	})
	
	// Test case 4: Duplicate registration
	t.Run("DuplicateRegistration", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		
		info1 := encoding.NewFormatInfo("Format 1", "application/test")
		err := registry.RegisterFormat(info1)
		require.NoError(t, err)
		
		// Register again with different info
		info2 := encoding.NewFormatInfo("Format 2", "application/test")
		err = registry.RegisterFormat(info2)
		require.NoError(t, err) // Should succeed and overwrite
		
		// Verify it was overwritten
		retrievedInfo, err := registry.GetFormat("application/test")
		require.NoError(t, err)
		assert.Equal(t, "Format 2", retrievedInfo.Name)
	})
	
	// Test case 5: Invalid registrations
	t.Run("InvalidRegistrations", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		
		// Nil format info
		err := registry.RegisterFormat(nil)
		assert.Error(t, err)
		
		// Empty MIME type
		info := encoding.NewFormatInfo("Test", "")
		err = registry.RegisterFormat(info)
		assert.Error(t, err)
	})
}

// TestMIMETypeHandlingRegression tests for MIME type handling regressions
func TestMIMETypeHandlingRegression(t *testing.T) {
	registry := encoding.NewFormatRegistry()
	
	// Register test format
	info := encoding.NewFormatInfo("JSON", "application/json")
	info.Aliases = []string{"json"}
	require.NoError(t, registry.RegisterFormat(info))
	
	testCases := []struct {
		name        string
		mimeType    string
		shouldMatch bool
		description string
	}{
		{
			name:        "Exact match",
			mimeType:    "application/json",
			shouldMatch: true,
			description: "Exact MIME type should match",
		},
		{
			name:        "Alias match",
			mimeType:    "json",
			shouldMatch: true,
			description: "Alias should match",
		},
		{
			name:        "Case insensitive",
			mimeType:    "APPLICATION/JSON",
			shouldMatch: true,
			description: "MIME type should be case insensitive",
		},
		{
			name:        "With charset",
			mimeType:    "application/json; charset=utf-8",
			shouldMatch: true,
			description: "MIME type with parameters should match",
		},
		{
			name:        "With multiple parameters",
			mimeType:    "application/json; charset=utf-8; boundary=something",
			shouldMatch: true,
			description: "MIME type with multiple parameters should match",
		},
		{
			name:        "Spaces around semicolon",
			mimeType:    "application/json ; charset=utf-8",
			shouldMatch: true,
			description: "Spaces around parameters should be handled",
		},
		{
			name:        "No match",
			mimeType:    "application/xml",
			shouldMatch: false,
			description: "Different MIME type should not match",
		},
		{
			name:        "Partial match",
			mimeType:    "application/json-patch",
			shouldMatch: false,
			description: "Partial MIME type should not match",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			supported := registry.SupportsFormat(tc.mimeType)
			assert.Equal(t, tc.shouldMatch, supported, tc.description)
			
			if tc.shouldMatch {
				// Should also be able to get format info
				_, err := registry.GetFormat(tc.mimeType)
				assert.NoError(t, err, tc.description)
			}
		})
	}
}

// TestFormatCapabilitiesRegression tests for format capabilities regressions
func TestFormatCapabilitiesRegression(t *testing.T) {
	registry := encoding.NewFormatRegistry()
	
	// Register formats with different capabilities
	jsonInfo := encoding.NewFormatInfo("JSON", "application/json")
	jsonInfo.Capabilities = encoding.FormatCapabilities{
		Streaming:        true,
		Compression:      false,
		SchemaValidation: false,
		BinaryEfficient:  false,
		HumanReadable:    true,
		SelfDescribing:   true,
		Versionable:      false,
	}
	require.NoError(t, registry.RegisterFormat(jsonInfo))
	
	binaryInfo := encoding.NewFormatInfo("Binary", "application/binary")
	binaryInfo.Capabilities = encoding.FormatCapabilities{
		Streaming:        true,
		Compression:      true,
		SchemaValidation: true,
		BinaryEfficient:  true,
		HumanReadable:    false,
		SelfDescribing:   false,
		Versionable:      true,
	}
	require.NoError(t, registry.RegisterFormat(binaryInfo))
	
	testCases := []struct {
		name                string
		acceptedFormats     []string
		requiredCapabilities *encoding.FormatCapabilities
		expectedFormat      string
		shouldSucceed       bool
		description         string
	}{
		{
			name:            "No requirements",
			acceptedFormats: []string{"application/json", "application/binary"},
			requiredCapabilities: nil,
			expectedFormat:  "application/json", // First in list
			shouldSucceed:   true,
			description:     "Should select first format when no requirements",
		},
		{
			name:            "Human readable required",
			acceptedFormats: []string{"application/binary", "application/json"},
			requiredCapabilities: &encoding.FormatCapabilities{
				HumanReadable: true,
			},
			expectedFormat: "application/json",
			shouldSucceed:  true,
			description:    "Should select JSON for human readable requirement",
		},
		{
			name:            "Binary efficient required",
			acceptedFormats: []string{"application/json", "application/binary"},
			requiredCapabilities: &encoding.FormatCapabilities{
				BinaryEfficient: true,
			},
			expectedFormat: "application/binary",
			shouldSucceed:  true,
			description:    "Should select binary for efficiency requirement",
		},
		{
			name:            "Multiple requirements",
			acceptedFormats: []string{"application/json", "application/binary"},
			requiredCapabilities: &encoding.FormatCapabilities{
				Streaming:    true,
				Compression:  true,
				Versionable:  true,
			},
			expectedFormat: "application/binary",
			shouldSucceed:  true,
			description:    "Should select format matching all requirements",
		},
		{
			name:            "Impossible requirements",
			acceptedFormats: []string{"application/json", "application/binary"},
			requiredCapabilities: &encoding.FormatCapabilities{
				HumanReadable:   true,
				BinaryEfficient: true,
			},
			expectedFormat: "",
			shouldSucceed:  false,
			description:    "Should fail when no format matches conflicting requirements",
		},
		{
			name:            "Schema validation required",
			acceptedFormats: []string{"application/json", "application/binary"},
			requiredCapabilities: &encoding.FormatCapabilities{
				SchemaValidation: true,
			},
			expectedFormat: "application/binary",
			shouldSucceed:  true,
			description:    "Should select format with schema validation",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			selectedFormat, err := registry.SelectFormat(tc.acceptedFormats, tc.requiredCapabilities)
			
			if tc.shouldSucceed {
				require.NoError(t, err, tc.description)
				assert.Equal(t, tc.expectedFormat, selectedFormat, tc.description)
			} else {
				assert.Error(t, err, tc.description)
			}
		})
	}
}

// TestFactoryRegistrationRegression tests for factory registration regressions
func TestFactoryRegistrationRegression(t *testing.T) {
	registry := encoding.NewFormatRegistry()
	
	// Test encoder factory registration
	t.Run("EncoderFactoryRegistration", func(t *testing.T) {
		factory := encoding.NewDefaultEncoderFactory()
		
		// Register mock encoder
		factory.RegisterEncoder("application/test", func(opts *encoding.EncodingOptions) (encoding.Encoder, error) {
			return &mockEncoder{contentType: "application/test"}, nil
		})
		
		err := registry.RegisterEncoderFactory("application/test", factory)
		require.NoError(t, err)
		
		// Verify we can get encoder
		assert.True(t, registry.SupportsEncoding("application/test"))
		
		encoder, err := registry.GetEncoder(context.Background(), "application/test", nil)
		require.NoError(t, err)
		assert.Equal(t, "application/test", encoder.ContentType())
	})
	
	// Test decoder factory registration
	t.Run("DecoderFactoryRegistration", func(t *testing.T) {
		factory := encoding.NewDefaultDecoderFactory()
		
		// Register mock decoder
		factory.RegisterDecoder("application/test", func(opts *encoding.DecodingOptions) (encoding.Decoder, error) {
			return &mockDecoder{contentType: "application/test"}, nil
		})
		
		err := registry.RegisterDecoderFactory("application/test", factory)
		require.NoError(t, err)
		
		// Verify we can get decoder
		assert.True(t, registry.SupportsDecoding("application/test"))
		
		decoder, err := registry.GetDecoder(context.Background(), "application/test", nil)
		require.NoError(t, err)
		assert.Equal(t, "application/test", decoder.ContentType())
	})
	
	// Test codec factory registration
	t.Run("CodecFactoryRegistration", func(t *testing.T) {
		factory := encoding.NewDefaultCodecFactory()
		
		// Register mock codec
		factory.RegisterCodec(
			"application/test",
			func(opts *encoding.EncodingOptions) (encoding.Encoder, error) {
				return &mockEncoder{contentType: "application/test"}, nil
			},
			func(opts *encoding.DecodingOptions) (encoding.Decoder, error) {
				return &mockDecoder{contentType: "application/test"}, nil
			},
			nil, // No stream encoder
			nil, // No stream decoder
		)
		
		err := registry.RegisterCodecFactory("application/test", factory)
		require.NoError(t, err)
		
		// Verify we can get both encoder and decoder
		assert.True(t, registry.SupportsEncoding("application/test"))
		assert.True(t, registry.SupportsDecoding("application/test"))
		
		codec, err := registry.GetCodec(context.Background(), "application/test", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "application/test", codec.ContentType())
	})
}

// TestBackwardCompatibilityRegression tests for backward compatibility regressions
func TestBackwardCompatibilityRegression(t *testing.T) {
	registry := encoding.NewFormatRegistry()
	
	// Test legacy interface registration
	t.Run("LegacyInterfaceRegistration", func(t *testing.T) {
		// Create a mock factory that implements the legacy interface
		factory := &mockLegacyFactory{
			encoderFunc: func(ctx context.Context, contentType string, options *encoding.EncodingOptions) (encoding.Encoder, error) {
				return &mockEncoder{contentType: contentType}, nil
			},
			decoderFunc: func(ctx context.Context, contentType string, options *encoding.DecodingOptions) (encoding.Decoder, error) {
				return &mockDecoder{contentType: contentType}, nil
			},
		}
		
		// Register using legacy method
		err := registry.RegisterCodec("application/legacy", factory)
		require.NoError(t, err)
		
		// Verify it works
		encoder, err := registry.GetEncoder(context.Background(), "application/legacy", nil)
		require.NoError(t, err)
		assert.Equal(t, "application/legacy", encoder.ContentType())
		
		decoder, err := registry.GetDecoder(context.Background(), "application/legacy", nil)
		require.NoError(t, err)
		assert.Equal(t, "application/legacy", decoder.ContentType())
	})
	
	// Test mixed registration (legacy + new)
	t.Run("MixedRegistration", func(t *testing.T) {
		// Register legacy codec factory
		legacyFactory := &mockLegacyFactory{
			encoderFunc: func(ctx context.Context, contentType string, options *encoding.EncodingOptions) (encoding.Encoder, error) {
				return &mockEncoder{contentType: contentType}, nil
			},
			decoderFunc: func(ctx context.Context, contentType string, options *encoding.DecodingOptions) (encoding.Decoder, error) {
				return &mockDecoder{contentType: contentType}, nil
			},
		}
		
		err := registry.RegisterCodec("application/mixed", legacyFactory)
		require.NoError(t, err)
		
		// Register new concrete factory for same type (should overwrite)
		newFactory := encoding.NewDefaultCodecFactory()
		newFactory.RegisterCodec(
			"application/mixed",
			func(opts *encoding.EncodingOptions) (encoding.Encoder, error) {
				return &mockEncoder{contentType: "application/mixed-new"}, nil
			},
			func(opts *encoding.DecodingOptions) (encoding.Decoder, error) {
				return &mockDecoder{contentType: "application/mixed-new"}, nil
			},
			nil,
			nil,
		)
		
		err = registry.RegisterCodecFactory("application/mixed", newFactory)
		require.NoError(t, err)
		
		// Should use new factory
		encoder, err := registry.GetEncoder(context.Background(), "application/mixed", nil)
		require.NoError(t, err)
		assert.Equal(t, "application/mixed-new", encoder.ContentType())
	})
}

// TestDefaultFormatRegression tests for default format handling regressions
func TestDefaultFormatRegression(t *testing.T) {
	registry := encoding.NewFormatRegistry()
	
	// Test initial default
	t.Run("InitialDefault", func(t *testing.T) {
		defaultFormat := registry.GetDefaultFormat()
		assert.Equal(t, "application/json", defaultFormat)
	})
	
	// Test setting custom default
	t.Run("CustomDefault", func(t *testing.T) {
		// Register custom format
		info := encoding.NewFormatInfo("Custom", "application/custom")
		require.NoError(t, registry.RegisterFormat(info))
		
		// Set as default
		err := registry.SetDefaultFormat("application/custom")
		require.NoError(t, err)
		
		defaultFormat := registry.GetDefaultFormat()
		assert.Equal(t, "application/custom", defaultFormat)
	})
	
	// Test setting non-existent default
	t.Run("NonExistentDefault", func(t *testing.T) {
		err := registry.SetDefaultFormat("application/nonexistent")
		assert.Error(t, err)
		
		// Default should remain unchanged
		defaultFormat := registry.GetDefaultFormat()
		assert.NotEqual(t, "application/nonexistent", defaultFormat)
	})
	
	// Test format selection with no accepted formats
	t.Run("EmptyAcceptedFormats", func(t *testing.T) {
		selectedFormat, err := registry.SelectFormat([]string{}, nil)
		require.NoError(t, err)
		
		// Should return default format
		assert.Equal(t, registry.GetDefaultFormat(), selectedFormat)
	})
}

// TestUnregistrationRegression tests for unregistration regressions
func TestUnregistrationRegression(t *testing.T) {
	registry := encoding.NewFormatRegistry()
	
	// Register format with aliases
	info := encoding.NewFormatInfo("Test", "application/test")
	info.Aliases = []string{"test", "tst"}
	require.NoError(t, registry.RegisterFormat(info))
	
	// Register factory
	factory := encoding.NewDefaultCodecFactory()
	factory.RegisterCodec(
		"application/test",
		func(opts *encoding.EncodingOptions) (encoding.Encoder, error) {
			return &mockEncoder{contentType: "application/test"}, nil
		},
		func(opts *encoding.DecodingOptions) (encoding.Decoder, error) {
			return &mockDecoder{contentType: "application/test"}, nil
		},
		nil,
		nil,
	)
	require.NoError(t, registry.RegisterCodecFactory("application/test", factory))
	
	// Verify format is registered
	assert.True(t, registry.SupportsFormat("application/test"))
	assert.True(t, registry.SupportsFormat("test"))
	assert.True(t, registry.SupportsEncoding("application/test"))
	assert.True(t, registry.SupportsDecoding("application/test"))
	
	// Unregister format
	err := registry.UnregisterFormat("application/test")
	require.NoError(t, err)
	
	// Verify everything is gone
	assert.False(t, registry.SupportsFormat("application/test"))
	assert.False(t, registry.SupportsFormat("test"))
	assert.False(t, registry.SupportsFormat("tst"))
	assert.False(t, registry.SupportsEncoding("application/test"))
	assert.False(t, registry.SupportsDecoding("application/test"))
	
	// Verify can't get format info
	_, err = registry.GetFormat("application/test")
	assert.Error(t, err)
	
	// Verify can't get encoder/decoder
	_, err = registry.GetEncoder(context.Background(), "application/test", nil)
	assert.Error(t, err)
	
	_, err = registry.GetDecoder(context.Background(), "application/test", nil)
	assert.Error(t, err)
}

// TestEventTypeHandlingRegression tests for event type handling regressions
func TestEventTypeHandlingRegression(t *testing.T) {
	ctx := context.Background()
	registry := encoding.GetGlobalRegistry()
	
	// Test all known event types
	testEvents := []events.Event{
		events.NewTextMessageStartEvent("msg1", events.WithRole("user")),
		events.NewTextMessageContentEvent("msg1", "Hello"),
		events.NewTextMessageEndEvent("msg1"),
		// Add more event types as they become available
	}
	
	formats := []string{"application/json", "application/x-protobuf"}
	
	for _, format := range formats {
		t.Run(fmt.Sprintf("Format_%s", format), func(t *testing.T) {
			encoder, err := registry.GetEncoder(ctx, format, nil)
			require.NoError(t, err)
			
			decoder, err := registry.GetDecoder(ctx, format, nil)
			require.NoError(t, err)
			
			for _, event := range testEvents {
				t.Run(fmt.Sprintf("Event_%s", event.Type()), func(t *testing.T) {
					// Test single event round-trip
					data, err := encoder.Encode(ctx, event)
					require.NoError(t, err, "Failed to encode %s", event.Type())
					
					decodedEvent, err := decoder.Decode(ctx, data)
					require.NoError(t, err, "Failed to decode %s", event.Type())
					
					assert.Equal(t, event.Type(), decodedEvent.Type(), "Event type mismatch")
				})
			}
			
			// Test multiple events
			t.Run("MultipleEvents", func(t *testing.T) {
				data, err := encoder.EncodeMultiple(ctx, testEvents)
				require.NoError(t, err)
				
				decodedEvents, err := decoder.DecodeMultiple(ctx, data)
				require.NoError(t, err)
				
				require.Equal(t, len(testEvents), len(decodedEvents))
				
				for i, originalEvent := range testEvents {
					assert.Equal(t, originalEvent.Type(), decodedEvents[i].Type())
				}
			})
		})
	}
}

// Mock implementations for testing

type mockEncoder struct {
	contentType string
}

func (m *mockEncoder) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return []byte(fmt.Sprintf("encoded:%s", event.Type())), nil
}

func (m *mockEncoder) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	var result strings.Builder
	for _, event := range events {
		result.WriteString(fmt.Sprintf("encoded:%s;", event.Type()))
	}
	return []byte(result.String()), nil
}

func (m *mockEncoder) ContentType() string {
	return m.contentType
}

func (m *mockEncoder) CanStream() bool {
	return false
}

type mockDecoder struct {
	contentType string
}

func (m *mockDecoder) Decode(ctx context.Context, data []byte) (events.Event, error) {
	// Create a mock event
	return events.NewTextMessageContentEvent("mock", "mock"), nil
}

func (m *mockDecoder) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	// Count semicolons to determine number of events
	count := strings.Count(string(data), ";")
	events := make([]events.Event, count)
	for i := 0; i < count; i++ {
		events[i] = events.NewTextMessageContentEvent("mock", "mock")
	}
	return events, nil
}

func (m *mockDecoder) ContentType() string {
	return m.contentType
}

func (m *mockDecoder) CanStream() bool {
	return false
}

type mockLegacyFactory struct {
	encoderFunc func(ctx context.Context, contentType string, options *encoding.EncodingOptions) (encoding.Encoder, error)
	decoderFunc func(ctx context.Context, contentType string, options *encoding.DecodingOptions) (encoding.Decoder, error)
}

func (m *mockLegacyFactory) CreateEncoder(ctx context.Context, contentType string, options *encoding.EncodingOptions) (encoding.Encoder, error) {
	return m.encoderFunc(ctx, contentType, options)
}

func (m *mockLegacyFactory) CreateDecoder(ctx context.Context, contentType string, options *encoding.DecodingOptions) (encoding.Decoder, error) {
	return m.decoderFunc(ctx, contentType, options)
}

func (m *mockLegacyFactory) CreateStreamEncoder(ctx context.Context, contentType string, options *encoding.EncodingOptions) (encoding.StreamEncoder, error) {
	return nil, fmt.Errorf("streaming not supported")
}

func (m *mockLegacyFactory) CreateStreamDecoder(ctx context.Context, contentType string, options *encoding.DecodingOptions) (encoding.StreamDecoder, error) {
	return nil, fmt.Errorf("streaming not supported")
}

func (m *mockLegacyFactory) SupportedEncoders() []string {
	return []string{"application/legacy"}
}

func (m *mockLegacyFactory) SupportedDecoders() []string {
	return []string{"application/legacy"}
}