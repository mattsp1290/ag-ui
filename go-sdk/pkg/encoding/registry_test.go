package encoding_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
	_ "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json" // Register JSON codec
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
			return &mockRegistryTestEncoder{contentType: "application/test"}, nil
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
			func(encOptions *encoding.EncodingOptions, decOptions *encoding.DecodingOptions) (encoding.Codec, error) {
				return &mockRegistryTestCodec{contentType: "application/test"}, nil
			},
		)
		
		// Create codec through codec factory
		codec, err := factory.CreateCodec(context.Background(), "application/test", nil, nil)
		require.NoError(t, err)
		assert.Equal(t, "application/test", codec.ContentType())
		
		// Test that it can encode
		event := events.NewTextMessageStartEvent("test-msg", events.WithRole("user"))
		data, err := codec.Encode(context.Background(), event)
		require.NoError(t, err)
		assert.NotEmpty(t, data)
	})
}

func TestCachingFactory(t *testing.T) {
	baseFactory := encoding.NewDefaultEncoderFactory()
	createCount := 0
	
	// Register a test encoder that counts creations
	baseFactory.RegisterEncoder("application/test", func(opts *encoding.EncodingOptions) (encoding.Encoder, error) {
		createCount++
		return &mockRegistryTestEncoder{contentType: "application/test"}, nil
	})
	
	// Create caching factory
	cachingFactory := encoding.NewCachingEncoderFactoryWithConcrete(baseFactory)
	
	// Create encoder multiple times with same options
	opts := &encoding.EncodingOptions{Pretty: true}
	
	encoder1, err := cachingFactory.CreateEncoder(context.Background(), "application/test", opts)
	require.NoError(t, err)
	assert.Equal(t, 1, createCount)
	
	encoder2, err := cachingFactory.CreateEncoder(context.Background(), "application/test", opts)
	require.NoError(t, err)
	assert.Equal(t, 1, createCount) // Should not create new one
	// Note: Encoders are wrapped in adapters, so they won't be the same object
	// but the underlying codec creation should be cached
	assert.Equal(t, encoder1.ContentType(), encoder2.ContentType())
	
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

func (m *mockRegistryTestEncoder) SupportsStreaming() bool {
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

func (m *mockRegistryDecoder) SupportsStreaming() bool {
	return false
}

// mockRegistryTestCodec is a complete codec for testing
type mockRegistryTestCodec struct {
	contentType string
}

func (m *mockRegistryTestCodec) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return []byte("mock"), nil
}

func (m *mockRegistryTestCodec) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return []byte("mock"), nil
}

func (m *mockRegistryTestCodec) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return events.NewTextMessageStartEvent("test", events.WithRole("user")), nil
}

func (m *mockRegistryTestCodec) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return []events.Event{events.NewTextMessageStartEvent("test", events.WithRole("user"))}, nil
}

func (m *mockRegistryTestCodec) ContentType() string {
	return m.contentType
}

func (m *mockRegistryTestCodec) CanStream() bool {
	return false
}

func (m *mockRegistryTestCodec) SupportsStreaming() bool {
	return false
}

// TestFormatRegistryThreadSafety verifies that the registry is thread-safe
func TestFormatRegistryThreadSafety(t *testing.T) {
	registry := encoding.NewFormatRegistry()
	ctx := context.Background()
	
	// Register some initial formats with aliases
	for i := 0; i < 5; i++ {
		info := encoding.NewFormatInfo(
			fmt.Sprintf("Format %d", i),
			fmt.Sprintf("application/test%d", i),
		)
		info.Aliases = []string{
			fmt.Sprintf("test%d", i),
			fmt.Sprintf("t%d", i),
			fmt.Sprintf("format%d", i),
		}
		
		err := registry.RegisterFormat(info)
		require.NoError(t, err)
		
		// Register factories
		factory := encoding.NewDefaultCodecFactory()
		factory.RegisterEncoder(info.MIMEType, func(opts *encoding.EncodingOptions) (encoding.Encoder, error) {
			return &mockRegistryTestEncoder{contentType: info.MIMEType}, nil
		})
		factory.RegisterDecoder(info.MIMEType, func(opts *encoding.DecodingOptions) (encoding.Decoder, error) {
			return &mockRegistryDecoder{contentType: info.MIMEType}, nil
		})
		
		err = registry.RegisterCodecFactory(info.MIMEType, factory)
		require.NoError(t, err)
	}
	
	// Run concurrent operations
	const numGoroutines = 100
	const numOperations = 1000
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	// Channel to collect errors
	errChan := make(chan error, numGoroutines*numOperations)
	
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				// Randomly choose an operation
				op := (goroutineID + j) % 10
				formatIdx := j % 5
				mimeType := fmt.Sprintf("application/test%d", formatIdx)
				
				// Use different aliases to test resolveAlias under concurrency
				aliases := []string{
					mimeType,
					fmt.Sprintf("test%d", formatIdx),
					fmt.Sprintf("t%d", formatIdx),
					fmt.Sprintf("format%d", formatIdx),
				}
				aliasToUse := aliases[j%len(aliases)]
				
				switch op {
				case 0, 1: // GetFormat with alias resolution
					_, err := registry.GetFormat(aliasToUse)
					if err != nil {
						errChan <- fmt.Errorf("GetFormat failed: %w", err)
					}
					
				case 2, 3: // GetEncoder with alias resolution
					encoder, err := registry.GetEncoder(ctx, aliasToUse, nil)
					if err != nil {
						errChan <- fmt.Errorf("GetEncoder failed: %w", err)
					} else if encoder == nil {
						errChan <- fmt.Errorf("GetEncoder returned nil")
					}
					
				case 4, 5: // GetDecoder with alias resolution
					decoder, err := registry.GetDecoder(ctx, aliasToUse, nil)
					if err != nil {
						errChan <- fmt.Errorf("GetDecoder failed: %w", err)
					} else if decoder == nil {
						errChan <- fmt.Errorf("GetDecoder returned nil")
					}
					
				case 6, 7: // GetCodec with alias resolution (tests the fixed deadlock issue)
					codec, err := registry.GetCodec(ctx, aliasToUse, nil, nil)
					if err != nil {
						errChan <- fmt.Errorf("GetCodec failed: %w", err)
					} else if codec == nil {
						errChan <- fmt.Errorf("GetCodec returned nil")
					}
					
				case 8: // SupportsFormat with alias resolution
					if !registry.SupportsFormat(aliasToUse) {
						errChan <- fmt.Errorf("SupportsFormat returned false for %s", aliasToUse)
					}
					
				case 9: // Register and unregister operations
					// Try to register a new format
					newFormat := fmt.Sprintf("application/concurrent-%d-%d", goroutineID, j)
					info := encoding.NewFormatInfo(
						fmt.Sprintf("Concurrent Format %d-%d", goroutineID, j),
						newFormat,
					)
					info.Aliases = []string{
						fmt.Sprintf("concurrent%d%d", goroutineID, j),
						fmt.Sprintf("c%d%d", goroutineID, j),
					}
					
					if err := registry.RegisterFormat(info); err != nil {
						// It's ok if registration fails due to conflicts
						continue
					}
					
					// Register a factory for it
					factory := encoding.NewDefaultCodecFactory()
					factory.RegisterEncoder(newFormat, func(opts *encoding.EncodingOptions) (encoding.Encoder, error) {
						return &mockRegistryTestEncoder{contentType: newFormat}, nil
					})
					
					if err := registry.RegisterCodecFactory(newFormat, factory); err != nil {
						// It's ok if registration fails
						continue
					}
					
					// Try to use it with an alias
					if _, err := registry.GetEncoder(ctx, fmt.Sprintf("c%d%d", goroutineID, j), nil); err != nil {
						// Only report error if it's not due to concurrent unregistration
						if !strings.Contains(err.Error(), "no encoder registered for format") {
							errChan <- fmt.Errorf("Failed to get encoder for newly registered format: %w", err)
						}
						// If it's "no encoder registered", it means another goroutine unregistered it concurrently,
						// which is a valid race condition in this stress test
					}
					
					// Unregister it
					if err := registry.UnregisterFormat(newFormat); err != nil {
						// It's ok if unregistration fails
						continue
					}
				}
			}
		}(i)
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)
	
	// Check for errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}
	
	if len(errors) > 0 {
		t.Errorf("Thread safety test encountered %d errors:", len(errors))
		// Print first 10 errors to avoid spam
		for i, err := range errors {
			if i >= 10 {
				t.Errorf("... and %d more errors", len(errors)-10)
				break
			}
			t.Errorf("  Error %d: %v", i+1, err)
		}
	}
}