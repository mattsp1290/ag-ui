package encoding_test

import (
	"fmt"
	"log"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
)

func ExampleFormatRegistry() {
	// Get the global registry
	registry := encoding.GetGlobalRegistry()
	
	// List all supported formats
	formats := registry.ListFormats()
	for _, format := range formats {
		fmt.Printf("Format: %s (%s)\n", format.Name, format.MIMEType)
	}
	
	// Output:
	// Format: JSON (application/json)
	// Format: Protocol Buffers (application/x-protobuf)
}

func ExampleFormatRegistry_GetEncoder() {
	registry := encoding.GetGlobalRegistry()
	
	// Get a JSON encoder
	encoder, err := registry.GetEncoder("application/json", &encoding.EncodingOptions{
		Pretty: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	
	// Use the encoder
	event := events.NewTextMessageStartEvent("msg-123", events.WithRole("assistant"))
	
	data, err := encoder.Encode(event)
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Encoded %d bytes\n", len(data))
}

func ExampleFormatRegistry_SelectFormat() {
	registry := encoding.GetGlobalRegistry()
	
	// Client accepts multiple formats
	acceptedFormats := []string{
		"application/json",
		"application/x-protobuf",
		"application/x-msgpack",
	}
	
	// But requires human-readable format
	requiredCapabilities := &encoding.FormatCapabilities{
		HumanReadable: true,
	}
	
	// Registry selects the best format
	selectedFormat, err := registry.SelectFormat(acceptedFormats, requiredCapabilities)
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Selected format: %s\n", selectedFormat)
	// Output: Selected format: application/json
}

func ExampleFormatRegistry_RegisterFormat() {
	// Create a custom registry
	registry := encoding.NewFormatRegistry()
	
	// Define a custom format
	customFormat := encoding.NewFormatInfo("Custom Binary", "application/x-custom")
	customFormat.Aliases = []string{"custom", "cbin"}
	customFormat.FileExtensions = []string{".cbin"}
	customFormat.Description = "Custom binary format for high performance"
	customFormat.Priority = 15 // Between JSON (10) and Protobuf (20)
	
	// Set capabilities
	customFormat.Capabilities = encoding.FormatCapabilities{
		Streaming:       true,
		BinaryEfficient: true,
		Compression:     true,
		CompressionAlgorithms: []string{"gzip", "zstd"},
		SchemaValidation: true,
		Versionable:     true,
	}
	
	// Set performance profile
	customFormat.Performance = encoding.PerformanceProfile{
		EncodingSpeed:  3.0, // 3x faster than JSON
		DecodingSpeed:  2.5, // 2.5x faster than JSON
		SizeEfficiency: 4.0, // 4x more compact than JSON
		MemoryUsage:    0.5, // Uses half the memory of JSON
	}
	
	// Register the format
	if err := registry.RegisterFormat(customFormat); err != nil {
		log.Fatal(err)
	}
	
	// Register a factory for the format
	factory := encoding.NewDefaultCodecFactory()
	factory.RegisterCodec(
		"application/x-custom",
		func(opts *encoding.EncodingOptions) (encoding.Encoder, error) {
			// Return custom encoder implementation
			return nil, fmt.Errorf("not implemented in example")
		},
		func(opts *encoding.DecodingOptions) (encoding.Decoder, error) {
			// Return custom decoder implementation
			return nil, fmt.Errorf("not implemented in example")
		},
		nil,
		nil,
	)
	
	if err := registry.RegisterCodec("application/x-custom", factory); err != nil {
		log.Fatal(err)
	}
	
	// Now the format can be used
	fmt.Printf("Custom format registered: %v\n", registry.SupportsFormat("custom"))
	// Output: Custom format registered: true
}

func ExampleFormatRegistry_plugin() {
	// Create a plugin-enabled factory
	factory := encoding.NewPluginEncoderFactory()
	
	// Define a plugin
	plugin := &exampleEncoderPlugin{
		name:         "MessagePack Plugin",
		contentTypes: []string{"application/x-msgpack", "application/msgpack"},
	}
	
	// Register the plugin
	if err := factory.RegisterPlugin(plugin); err != nil {
		log.Fatal(err)
	}
	
	// Now MessagePack encoding is available
	encoder, err := factory.CreateEncoder("application/x-msgpack", nil)
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("Plugin encoder created: %s\n", encoder.ContentType())
}

// Example plugin implementation
type exampleEncoderPlugin struct {
	name         string
	contentTypes []string
}

func (p *exampleEncoderPlugin) Name() string {
	return p.name
}

func (p *exampleEncoderPlugin) ContentTypes() []string {
	return p.contentTypes
}

func (p *exampleEncoderPlugin) CreateEncoder(contentType string, options *encoding.EncodingOptions) (encoding.Encoder, error) {
	// In a real implementation, this would create a MessagePack encoder
	return &mockEncoder{contentType: contentType}, nil
}

func (p *exampleEncoderPlugin) CreateStreamEncoder(contentType string, options *encoding.EncodingOptions) (encoding.StreamEncoder, error) {
	// In a real implementation, this would create a streaming MessagePack encoder
	return nil, fmt.Errorf("streaming not implemented in example")
}