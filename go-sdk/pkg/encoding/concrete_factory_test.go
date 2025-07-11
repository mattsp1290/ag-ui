package encoding_test

import (
	"context"
	"testing"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/stretchr/testify/assert"
)

func TestConcreteFactoryFunctions(t *testing.T) {
	t.Run("concrete encoder factory", func(t *testing.T) {
		// Create factory using concrete function
		factory := encoding.NewEncoderFactory()
		assert.NotNil(t, factory)
		
		// Verify it returns the correct concrete type
		assert.IsType(t, &encoding.DefaultEncoderFactory{}, factory)
		
		// Test it can be used with the registry
		registry := encoding.NewFormatRegistry()
		err := registry.RegisterEncoderFactory("application/test", factory)
		assert.NoError(t, err)
		
		// Verify we can get it back
		retrievedFactory, err := registry.GetEncoderFactory("application/test")
		assert.NoError(t, err)
		assert.Same(t, factory, retrievedFactory)
	})

	t.Run("concrete decoder factory", func(t *testing.T) {
		// Create factory using concrete function
		factory := encoding.NewDecoderFactory()
		assert.NotNil(t, factory)
		
		// Verify it returns the correct concrete type
		assert.IsType(t, &encoding.DefaultDecoderFactory{}, factory)
		
		// Test it can be used with the registry
		registry := encoding.NewFormatRegistry()
		err := registry.RegisterDecoderFactory("application/test", factory)
		assert.NoError(t, err)
		
		// Verify we can get it back
		retrievedFactory, err := registry.GetDecoderFactory("application/test")
		assert.NoError(t, err)
		assert.Same(t, factory, retrievedFactory)
	})

	t.Run("concrete codec factory", func(t *testing.T) {
		// Create factory using concrete function
		factory := encoding.NewCodecFactory()
		assert.NotNil(t, factory)
		
		// Verify it returns the correct concrete type
		assert.IsType(t, &encoding.DefaultCodecFactory{}, factory)
		
		// Test it can be used with the registry
		registry := encoding.NewFormatRegistry()
		err := registry.RegisterCodecFactory("application/test", factory)
		assert.NoError(t, err)
		
		// Verify we can get it back
		retrievedFactory, err := registry.GetCodecFactory("application/test")
		assert.NoError(t, err)
		assert.Same(t, factory, retrievedFactory)
	})

	t.Run("concrete plugin encoder factory", func(t *testing.T) {
		// Create factory using concrete function
		factory := encoding.NewPluginBasedEncoderFactory()
		assert.NotNil(t, factory)
		
		// Verify it returns the correct concrete type
		assert.IsType(t, &encoding.PluginEncoderFactory{}, factory)
		
		// Test it has the expected methods
		assert.NotNil(t, factory.SupportedEncoders())
	})

	t.Run("concrete plugin decoder factory", func(t *testing.T) {
		// Create factory using concrete function
		factory := encoding.NewPluginBasedDecoderFactory()
		assert.NotNil(t, factory)
		
		// Verify it returns the correct concrete type
		assert.IsType(t, &encoding.PluginDecoderFactory{}, factory)
		
		// Test it has the expected methods
		assert.NotNil(t, factory.SupportedDecoders())
	})

	t.Run("concrete caching encoder factory", func(t *testing.T) {
		// Create base factory
		baseFactory := encoding.NewEncoderFactory()
		
		// Create caching factory using concrete function
		cachingFactory := encoding.NewCachingEncoderFactoryWithConcrete(baseFactory)
		assert.NotNil(t, cachingFactory)
		
		// Verify it returns the correct concrete type
		assert.IsType(t, &encoding.CachingEncoderFactory{}, cachingFactory)
		
		// Test it has the expected methods
		assert.NotNil(t, cachingFactory.SupportedEncoders())
	})
}

func TestBackwardCompatibility(t *testing.T) {
	t.Run("legacy interface methods still work", func(t *testing.T) {
		registry := encoding.NewFormatRegistry()
		
		// Create a concrete factory and register constructors
		concreteFactory := encoding.NewDefaultCodecFactory()
		concreteFactory.RegisterCodec("application/test", 
			func(options *encoding.EncodingOptions) (encoding.Encoder, error) {
				return &mockTestEncoder{}, nil
			},
			func(options *encoding.DecodingOptions) (encoding.Decoder, error) {
				return &mockTestDecoder{}, nil
			},
			nil, // no stream encoder
			nil, // no stream decoder
		)
		
		// Register using legacy interface method
		err := registry.RegisterCodec("application/test", concreteFactory)
		assert.NoError(t, err)
		
		// Verify we can get encoder and decoder
		encoder, err := registry.GetEncoder(context.Background(), "application/test", nil)
		assert.NoError(t, err)
		assert.NotNil(t, encoder)
		
		decoder, err := registry.GetDecoder(context.Background(), "application/test", nil)
		assert.NoError(t, err)
		assert.NotNil(t, decoder)
	})
}

// Mock test encoder/decoder for backward compatibility tests
type mockTestEncoder struct{}

func (m *mockTestEncoder) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return []byte("test"), nil
}

func (m *mockTestEncoder) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return []byte("test"), nil
}

func (m *mockTestEncoder) ContentType() string {
	return "application/test"
}

func (m *mockTestEncoder) CanStream() bool {
	return false
}

type mockTestDecoder struct{}

func (m *mockTestDecoder) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return nil, nil
}

func (m *mockTestDecoder) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return nil, nil
}

func (m *mockTestDecoder) ContentType() string {
	return "application/test"
}

func (m *mockTestDecoder) CanStream() bool {
	return false
}