package encoding_test

import (
	"context"
	"testing"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatRegistry(t *testing.T) {
	t.Run("new registry", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		assert.NotNil(t, registry)
		assert.Equal(t, "application/json", registry.GetDefaultFormat())
	})

	t.Run("register format", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		
		info := encoding.NewFormatInfo("Test Format", "application/test")
		info.Aliases = []string{"test", "tst"}
		
		err := registry.RegisterFormat(info)
		require.NoError(t, err)
		
		// Check format is registered
		retrieved, err := registry.GetFormat("application/test")
		require.NoError(t, err)
		assert.Equal(t, "Test Format", retrieved.Name)
		
		// Check aliases work
		retrieved, err = registry.GetFormat("test")
		require.NoError(t, err)
		assert.Equal(t, "application/test", retrieved.MIMEType)
		
		retrieved, err = registry.GetFormat("tst")
		require.NoError(t, err)
		assert.Equal(t, "application/test", retrieved.MIMEType)
	})

	t.Run("register factory", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		factory := encoding.NewDefaultCodecFactory()
		
		err := registry.RegisterCodec("application/test", factory)
		require.NoError(t, err)
		
		assert.True(t, registry.SupportsEncoding("application/test"))
		assert.True(t, registry.SupportsDecoding("application/test"))
	})

	t.Run("unregister format", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		
		info := encoding.NewFormatInfo("Test Format", "application/test")
		err := registry.RegisterFormat(info)
		require.NoError(t, err)
		
		err = registry.UnregisterFormat("application/test")
		require.NoError(t, err)
		
		_, err = registry.GetFormat("application/test")
		assert.Error(t, err)
	})

	t.Run("list formats", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		
		// Register multiple formats with different priorities
		info1 := encoding.NewFormatInfo("Format 1", "application/test1")
		info1.Priority = 10
		registry.RegisterFormat(info1)
		
		info2 := encoding.NewFormatInfo("Format 2", "application/test2")
		info2.Priority = 20
		registry.RegisterFormat(info2)
		
		info3 := encoding.NewFormatInfo("Format 3", "application/test3")
		info3.Priority = 5
		registry.RegisterFormat(info3)
		
		formats := registry.ListFormats()
		assert.Len(t, formats, 3)
		
		// Check they're sorted by priority
		assert.Equal(t, "application/test3", formats[0].MIMEType)
		assert.Equal(t, "application/test1", formats[1].MIMEType)
		assert.Equal(t, "application/test2", formats[2].MIMEType)
	})

	t.Run("format selection", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		
		// Register formats with different capabilities
		info1 := encoding.NewFormatInfo("JSON", "application/json")
		info1.Capabilities = encoding.TextFormatCapabilities()
		registry.RegisterFormat(info1)
		
		info2 := encoding.NewFormatInfo("Binary", "application/binary")
		info2.Capabilities = encoding.BinaryFormatCapabilities()
		registry.RegisterFormat(info2)
		
		// Select format requiring human readability
		required := &encoding.FormatCapabilities{
			HumanReadable: true,
		}
		
		selected, err := registry.SelectFormat([]string{"application/binary", "application/json"}, required)
		require.NoError(t, err)
		assert.Equal(t, "application/json", selected)
		
		// Select format requiring binary efficiency
		required = &encoding.FormatCapabilities{
			BinaryEfficient: true,
		}
		
		selected, err = registry.SelectFormat([]string{"application/json", "application/binary"}, required)
		require.NoError(t, err)
		assert.Equal(t, "application/binary", selected)
	})

	t.Run("default format", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		
		// Register a format
		info := encoding.NewFormatInfo("Custom", "application/custom")
		registry.RegisterFormat(info)
		
		// Set as default
		err := registry.SetDefaultFormat("application/custom")
		require.NoError(t, err)
		assert.Equal(t, "application/custom", registry.GetDefaultFormat())
		
		// Try to set non-existent format as default
		err = registry.SetDefaultFormat("application/nonexistent")
		assert.Error(t, err)
	})

	t.Run("mime type parameters", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		
		info := encoding.NewFormatInfo("JSON", "application/json")
		registry.RegisterFormat(info)
		
		// Check that MIME type with parameters resolves correctly
		retrieved, err := registry.GetFormat("application/json; charset=utf-8")
		require.NoError(t, err)
		assert.Equal(t, "application/json", retrieved.MIMEType)
	})
}

func TestGlobalRegistry(t *testing.T) {
	t.Run("singleton", func(t *testing.T) {
		registry1 := encoding.GetGlobalRegistry()
		registry2 := encoding.GetGlobalRegistry()
		
		assert.Same(t, registry1, registry2)
	})

	t.Run("defaults registered", func(t *testing.T) {
		registry := encoding.GetGlobalRegistry()
		
		// Check JSON is registered by default
		assert.True(t, registry.SupportsFormat("application/json"))
		assert.True(t, registry.SupportsFormat("json"))
		
		// Check Protobuf is registered by default
		assert.True(t, registry.SupportsFormat("application/x-protobuf"))
		assert.True(t, registry.SupportsFormat("protobuf"))
	})
}

func TestFormatCapabilities(t *testing.T) {
	t.Run("text format preset", func(t *testing.T) {
		caps := encoding.TextFormatCapabilities()
		assert.True(t, caps.HumanReadable)
		assert.True(t, caps.SelfDescribing)
		assert.False(t, caps.BinaryEfficient)
	})

	t.Run("binary format preset", func(t *testing.T) {
		caps := encoding.BinaryFormatCapabilities()
		assert.True(t, caps.BinaryEfficient)
		assert.True(t, caps.Compression)
		assert.False(t, caps.HumanReadable)
	})

	t.Run("schema based preset", func(t *testing.T) {
		caps := encoding.SchemaBasedFormatCapabilities()
		assert.True(t, caps.SchemaValidation)
		assert.True(t, caps.Versionable)
		assert.True(t, caps.BinaryEfficient)
	})
}

func TestDefaultFactories(t *testing.T) {
	t.Run("encoder factory", func(t *testing.T) {
		factory := encoding.NewDefaultEncoderFactory()
		
		// Register a test encoder
		factory.RegisterEncoder("application/test", func(opts *encoding.EncodingOptions) (encoding.Encoder, error) {
			return &mockEncoder{contentType: "application/test"}, nil
		})
		
		// Create encoder
		encoder, err := factory.CreateEncoder(context.Background(), "application/test", nil)
		require.NoError(t, err)
		assert.Equal(t, "application/test", encoder.ContentType())
		
		// Check supported types
		supported := factory.SupportedEncoders()
		assert.Contains(t, supported, "application/test")
	})

	t.Run("decoder factory", func(t *testing.T) {
		factory := encoding.NewDefaultDecoderFactory()
		
		// Register a test decoder
		factory.RegisterDecoder("application/test", func(opts *encoding.DecodingOptions) (encoding.Decoder, error) {
			return &mockRegistryDecoder{contentType: "application/test"}, nil
		})
		
		// Create decoder
		decoder, err := factory.CreateDecoder(context.Background(), "application/test", nil)
		require.NoError(t, err)
		assert.Equal(t, "application/test", decoder.ContentType())
		
		// Check supported types
		supported := factory.SupportedDecoders()
		assert.Contains(t, supported, "application/test")
	})

	t.Run("codec factory", func(t *testing.T) {
		factory := encoding.NewDefaultCodecFactory()
		
		// Register a test codec
		factory.RegisterCodec(
			"application/test",
			func(opts *encoding.EncodingOptions) (encoding.Encoder, error) {
				return &mockEncoder{contentType: "application/test"}, nil
			},
			func(opts *encoding.DecodingOptions) (encoding.Decoder, error) {
				return &mockRegistryDecoder{contentType: "application/test"}, nil
			},
			nil,
			nil,
		)
		
		// Create encoder through codec factory
		encoder, err := factory.CreateEncoder(context.Background(), "application/test", nil)
		require.NoError(t, err)
		assert.Equal(t, "application/test", encoder.ContentType())
		
		// Create decoder through codec factory
		decoder, err := factory.CreateDecoder(context.Background(), "application/test", nil)
		require.NoError(t, err)
		assert.Equal(t, "application/test", decoder.ContentType())
	})
}

func TestCachingFactory(t *testing.T) {
	baseFactory := encoding.NewDefaultEncoderFactory()
	createCount := 0
	
	// Register a test encoder that counts creations
	baseFactory.RegisterEncoder("application/test", func(opts *encoding.EncodingOptions) (encoding.Encoder, error) {
		createCount++
		return &mockEncoder{contentType: "application/test"}, nil
	})
	
	// Create caching factory
	cachingFactory := encoding.NewCachingEncoderFactory(baseFactory)
	
	// Create encoder multiple times with same options
	opts := &encoding.EncodingOptions{Pretty: true}
	
	encoder1, err := cachingFactory.CreateEncoder(context.Background(), "application/test", opts)
	require.NoError(t, err)
	assert.Equal(t, 1, createCount)
	
	encoder2, err := cachingFactory.CreateEncoder(context.Background(), "application/test", opts)
	require.NoError(t, err)
	assert.Equal(t, 1, createCount) // Should not create new one
	assert.Same(t, encoder1, encoder2)
	
	// Create with different options
	opts2 := &encoding.EncodingOptions{Pretty: false}
	encoder3, err := cachingFactory.CreateEncoder(context.Background(), "application/test", opts2)
	require.NoError(t, err)
	assert.Equal(t, 2, createCount) // Should create new one
	assert.NotSame(t, encoder1, encoder3)
}

// Mock types for testing

type mockRegistryTestEncoder struct {
	contentType string
}

func (m *mockRegistryTestEncoder) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return []byte("mock"), nil
}

func (m *mockRegistryTestEncoder) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return []byte("mock"), nil
}

func (m *mockRegistryTestEncoder) ContentType() string {
	return m.contentType
}

func (m *mockRegistryTestEncoder) CanStream() bool {
	return false
}

type mockRegistryDecoder struct {
	contentType string
}

func (m *mockRegistryDecoder) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return nil, nil
}

func (m *mockRegistryDecoder) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return nil, nil
}

func (m *mockRegistryDecoder) ContentType() string {
	return m.contentType
}

func (m *mockRegistryDecoder) CanStream() bool {
	return false
}