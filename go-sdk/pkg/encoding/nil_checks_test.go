package encoding

import (
	"context"
	"testing"
)

// TestNilChecksCodecFactory tests nil checks in codec factory methods
func TestNilChecksCodecFactory(t *testing.T) {
	t.Run("DefaultCodecFactory CreateCodec with nil factory", func(t *testing.T) {
		var factory *DefaultCodecFactory
		_, err := factory.CreateCodec(context.Background(), "application/json", nil, nil)
		if err == nil {
			t.Error("Expected error when factory is nil")
		}
		if err.Error() != "configuration error in codec_factory for setting 'factory': codec factory cannot be nil (value: <nil>)" {
			t.Errorf("Expected 'configuration error in codec_factory for setting 'factory': codec factory cannot be nil (value: <nil>)', got '%s'", err.Error())
		}
	})

	t.Run("DefaultCodecFactory CreateCodec with nil context", func(t *testing.T) {
		factory := NewDefaultCodecFactory()
		_, err := factory.CreateCodec(nil, "application/json", nil, nil)
		if err == nil {
			t.Error("Expected error when context is nil")
		}
		if err.Error() != "configuration error in codec_factory for setting 'context': context cannot be nil (value: <nil>)" {
			t.Errorf("Expected 'configuration error in codec_factory for setting 'context': context cannot be nil (value: <nil>)', got '%s'", err.Error())
		}
	})

	t.Run("DefaultCodecFactory CreateCodec with empty content type", func(t *testing.T) {
		factory := NewDefaultCodecFactory()
		_, err := factory.CreateCodec(context.Background(), "", nil, nil)
		if err == nil {
			t.Error("Expected error when content type is empty")
		}
		if err.Error() != "configuration error in codec_factory for setting 'content_type': content type cannot be empty (value: )" {
			t.Errorf("Expected 'configuration error in codec_factory for setting 'content_type': content type cannot be empty (value: )', got '%s'", err.Error())
		}
	})

	t.Run("DefaultCodecFactory CreateStreamCodec with nil factory", func(t *testing.T) {
		var factory *DefaultCodecFactory
		_, err := factory.CreateStreamCodec(context.Background(), "application/json", nil, nil)
		if err == nil {
			t.Error("Expected error when factory is nil")
		}
		if err.Error() != "configuration error in codec_factory for setting 'factory': codec factory cannot be nil (value: <nil>)" {
			t.Errorf("Expected 'configuration error in codec_factory for setting 'factory': codec factory cannot be nil (value: <nil>)', got '%s'", err.Error())
		}
	})

	t.Run("DefaultCodecFactory SupportedTypes with nil factory", func(t *testing.T) {
		var factory *DefaultCodecFactory
		types := factory.SupportedTypes()
		if types == nil {
			t.Error("Expected empty slice, got nil")
		}
		if len(types) != 0 {
			t.Error("Expected empty slice for nil factory")
		}
	})

	t.Run("DefaultCodecFactory SupportsStreaming with nil factory", func(t *testing.T) {
		var factory *DefaultCodecFactory
		supports := factory.SupportsStreaming("application/json")
		if supports {
			t.Error("Expected false for nil factory")
		}
	})

	t.Run("DefaultCodecFactory RegisterCodec with nil factory", func(t *testing.T) {
		var factory *DefaultCodecFactory
		// Should not panic
		factory.RegisterCodec("application/json", nil)
	})

	t.Run("DefaultCodecFactory RegisterStreamCodec with nil factory", func(t *testing.T) {
		var factory *DefaultCodecFactory
		// Should not panic
		factory.RegisterStreamCodec("application/json", nil)
	})
}

// TestNilChecksCachingCodecFactory tests nil checks in caching codec factory methods
func TestNilChecksCachingCodecFactory(t *testing.T) {
	t.Run("NewCachingCodecFactory with nil factory", func(t *testing.T) {
		factory := NewCachingCodecFactory(nil)
		if factory != nil {
			t.Error("Expected nil when underlying factory is nil")
		}
	})

	t.Run("CachingCodecFactory CreateCodec with nil factory", func(t *testing.T) {
		var factory *CachingCodecFactory
		_, err := factory.CreateCodec(context.Background(), "application/json", nil, nil)
		if err == nil {
			t.Error("Expected error when factory is nil")
		}
		if err.Error() != "caching codec factory is nil" {
			t.Errorf("Expected 'caching codec factory is nil', got '%s'", err.Error())
		}
	})

	t.Run("CachingCodecFactory CreateCodec with nil underlying factory", func(t *testing.T) {
		factory := &CachingCodecFactory{factory: nil}
		_, err := factory.CreateCodec(context.Background(), "application/json", nil, nil)
		if err == nil {
			t.Error("Expected error when underlying factory is nil")
		}
		if err.Error() != "underlying codec factory is nil" {
			t.Errorf("Expected 'underlying codec factory is nil', got '%s'", err.Error())
		}
	})

	t.Run("CachingCodecFactory SupportedTypes with nil factory", func(t *testing.T) {
		var factory *CachingCodecFactory
		types := factory.SupportedTypes()
		if types == nil {
			t.Error("Expected empty slice, got nil")
		}
		if len(types) != 0 {
			t.Error("Expected empty slice for nil factory")
		}
	})

	t.Run("CachingCodecFactory SupportsStreaming with nil factory", func(t *testing.T) {
		var factory *CachingCodecFactory
		supports := factory.SupportsStreaming("application/json")
		if supports {
			t.Error("Expected false for nil factory")
		}
	})
}

// TestNilChecksFactoryMethods tests nil parameter handling in factory methods
func TestNilChecksFactoryMethods(t *testing.T) {
	t.Run("Factory methods with nil constructors", func(t *testing.T) {
		factory := NewDefaultCodecFactory()
		
		// Should not panic with nil constructors
		factory.RegisterCodec("test", nil)
		factory.RegisterStreamCodec("test", nil)
		
		// Should handle nil constructor gracefully
		_, err := factory.CreateCodec(context.Background(), "test", nil, nil)
		if err == nil {
			t.Error("Expected error when constructor is nil")
		}
	})

	t.Run("Factory methods with empty content types", func(t *testing.T) {
		factory := NewDefaultCodecFactory()
		
		// Should not panic with empty content types
		factory.RegisterCodec("", func(*EncodingOptions, *DecodingOptions) (Codec, error) {
			return nil, nil
		})
		factory.RegisterStreamCodec("", func(*EncodingOptions, *DecodingOptions) (StreamCodec, error) {
			return nil, nil
		})
	})
}