package encoding

import (
	"time"
)

// FormatInfo describes a data format's characteristics and capabilities
type FormatInfo struct {
	// Name is the human-readable name of the format
	Name string

	// MIMEType is the canonical MIME type
	MIMEType string

	// Aliases are alternative names/MIME types
	Aliases []string

	// FileExtensions are common file extensions
	FileExtensions []string

	// Version is the format version
	Version string

	// Description provides format details
	Description string

	// Capabilities describes what the format supports
	Capabilities FormatCapabilities

	// Performance characteristics
	Performance PerformanceProfile

	// Compatibility information
	Compatibility CompatibilityInfo

	// Priority for format selection (lower is higher priority)
	Priority int

	// Metadata contains additional format-specific information
	Metadata map[string]interface{}
}

// FormatCapabilities describes what features a format supports
type FormatCapabilities struct {
	// Streaming indicates if the format supports streaming
	Streaming bool

	// Compression indicates if the format supports compression
	Compression bool

	// CompressionAlgorithms lists supported compression algorithms
	CompressionAlgorithms []string

	// SchemaValidation indicates if the format supports schema validation
	SchemaValidation bool

	// BinaryEfficient indicates if the format is binary-efficient
	BinaryEfficient bool

	// HumanReadable indicates if the format is human-readable
	HumanReadable bool

	// SelfDescribing indicates if the format is self-describing
	SelfDescribing bool

	// Versionable indicates if the format supports versioning
	Versionable bool

	// SupportsNull indicates if the format can represent null values
	SupportsNull bool

	// SupportsReferences indicates if the format supports references/pointers
	SupportsReferences bool

	// SupportsComments indicates if the format supports comments
	SupportsComments bool

	// MaxNesting indicates maximum nesting depth (0 for unlimited)
	MaxNesting int

	// SupportedDataTypes lists supported data types
	SupportedDataTypes []string

	// Features lists additional features
	Features []string
}

// PerformanceProfile describes performance characteristics
type PerformanceProfile struct {
	// EncodingSpeed relative to JSON (1.0 = same as JSON)
	EncodingSpeed float64

	// DecodingSpeed relative to JSON (1.0 = same as JSON)
	DecodingSpeed float64

	// SizeEfficiency relative to JSON (1.0 = same as JSON)
	SizeEfficiency float64

	// MemoryUsage relative to JSON (1.0 = same as JSON)
	MemoryUsage float64

	// StreamingOverhead for streaming operations
	StreamingOverhead float64

	// StartupTime for codec initialization
	StartupTime time.Duration

	// Benchmarks contains benchmark results
	Benchmarks map[string]BenchmarkResult
}

// BenchmarkResult represents a performance benchmark result
type BenchmarkResult struct {
	// Name of the benchmark
	Name string

	// Operations per second
	OpsPerSecond float64

	// Bytes per second
	BytesPerSecond float64

	// Average latency
	AverageLatency time.Duration

	// P99 latency
	P99Latency time.Duration

	// Memory allocated per operation
	MemoryPerOp int64

	// Allocations per operation
	AllocsPerOp int64
}

// CompatibilityInfo describes format compatibility
type CompatibilityInfo struct {
	// MinSDKVersion minimum SDK version required
	MinSDKVersion string

	// MaxSDKVersion maximum SDK version supported
	MaxSDKVersion string

	// CompatibleWith lists compatible format versions
	CompatibleWith []string

	// Breaking changes from previous versions
	BreakingChanges []string

	// CrossPlatform indicates cross-platform compatibility
	CrossPlatform bool

	// PlatformSpecific lists platform-specific considerations
	PlatformSpecific map[string]string

	// LanguageSupport lists supported programming languages
	LanguageSupport []LanguageSupport

	// Standards lists relevant standards (e.g., RFC numbers)
	Standards []string
}

// LanguageSupport describes support for a programming language
type LanguageSupport struct {
	// Language name
	Language string

	// MinVersion minimum language version
	MinVersion string

	// Libraries available libraries
	Libraries []Library

	// NativeSupport indicates if native support exists
	NativeSupport bool

	// Notes additional notes
	Notes string
}

// Library describes a library for format support
type Library struct {
	// Name of the library
	Name string

	// Version of the library
	Version string

	// Repository URL
	Repository string

	// License of the library
	License string

	// Maintained indicates if actively maintained
	Maintained bool
}

// Common format capabilities presets

// TextFormatCapabilities returns capabilities for text-based formats
func TextFormatCapabilities() FormatCapabilities {
	return FormatCapabilities{
		Streaming:          true,
		HumanReadable:      true,
		SelfDescribing:     true,
		SupportsNull:       true,
		SupportsComments:   true,
		SupportedDataTypes: []string{"string", "number", "boolean", "null", "array", "object"},
	}
}

// BinaryFormatCapabilities returns capabilities for binary formats
func BinaryFormatCapabilities() FormatCapabilities {
	return FormatCapabilities{
		Streaming:          true,
		BinaryEfficient:    true,
		Compression:        true,
		SupportsNull:       true,
		Versionable:        true,
		SupportedDataTypes: []string{"string", "number", "boolean", "null", "array", "object", "binary"},
	}
}

// SchemaBasedFormatCapabilities returns capabilities for schema-based formats
func SchemaBasedFormatCapabilities() FormatCapabilities {
	return FormatCapabilities{
		Streaming:          true,
		SchemaValidation:   true,
		BinaryEfficient:    true,
		Versionable:        true,
		SupportsNull:       true,
		SupportsReferences: true,
		SupportedDataTypes: []string{"string", "number", "boolean", "null", "array", "object", "binary", "enum"},
	}
}

// NewFormatInfo creates a new FormatInfo with defaults
func NewFormatInfo(name, mimeType string) *FormatInfo {
	return &FormatInfo{
		Name:           name,
		MIMEType:       mimeType,
		Aliases:        []string{},
		FileExtensions: []string{},
		Version:        "1.0",
		Priority:       100,
		Metadata:       make(map[string]interface{}),
		Performance: PerformanceProfile{
			EncodingSpeed:  1.0,
			DecodingSpeed:  1.0,
			SizeEfficiency: 1.0,
			MemoryUsage:    1.0,
			Benchmarks:     make(map[string]BenchmarkResult),
		},
		Compatibility: CompatibilityInfo{
			CrossPlatform:    true,
			PlatformSpecific: make(map[string]string),
			LanguageSupport:  []LanguageSupport{},
		},
	}
}

// Common format definitions

// JSONFormatInfo returns format info for JSON
func JSONFormatInfo() *FormatInfo {
	info := NewFormatInfo("JSON", "application/json")
	info.Aliases = []string{"json", "text/json", "application/x-json"}
	info.FileExtensions = []string{".json"}
	info.Description = "JavaScript Object Notation - human-readable text format"
	info.Priority = 10

	info.Capabilities = FormatCapabilities{
		Streaming:          true,
		HumanReadable:      true,
		SelfDescribing:     true,
		SupportsNull:       true,
		SupportsComments:   false,
		SupportedDataTypes: []string{"string", "number", "boolean", "null", "array", "object"},
		Features:           []string{"unicode", "nested-objects", "arrays"},
	}

	info.Performance = PerformanceProfile{
		EncodingSpeed:  1.0,
		DecodingSpeed:  1.0,
		SizeEfficiency: 1.0,
		MemoryUsage:    1.0,
	}

	info.Compatibility.Standards = []string{"RFC 8259", "ECMA-404"}
	info.Compatibility.LanguageSupport = []LanguageSupport{
		{
			Language:      "Go",
			MinVersion:    "1.0",
			NativeSupport: true,
			Libraries: []Library{
				{
					Name:       "encoding/json",
					Version:    "stdlib",
					Repository: "https://golang.org/pkg/encoding/json/",
					License:    "BSD",
					Maintained: true,
				},
			},
		},
	}

	return info
}

// ProtobufFormatInfo returns format info for Protocol Buffers
func ProtobufFormatInfo() *FormatInfo {
	info := NewFormatInfo("Protocol Buffers", "application/x-protobuf")
	info.Aliases = []string{"protobuf", "proto", "application/protobuf", "application/vnd.google.protobuf"}
	info.FileExtensions = []string{".pb", ".proto"}
	info.Description = "Google's language-neutral, platform-neutral, extensible mechanism for serializing structured data"
	info.Version = "3"
	info.Priority = 20

	info.Capabilities = FormatCapabilities{
		Streaming:          true,
		SchemaValidation:   true,
		BinaryEfficient:    true,
		Versionable:        true,
		SupportsNull:       true,
		SupportsReferences: false,
		SupportedDataTypes: []string{"string", "number", "boolean", "bytes", "array", "message", "enum"},
		Features:           []string{"schema-evolution", "compact-binary", "strong-typing"},
	}

	info.Performance = PerformanceProfile{
		EncodingSpeed:  2.5, // Faster than JSON
		DecodingSpeed:  3.0, // Faster than JSON
		SizeEfficiency: 3.0, // More compact than JSON
		MemoryUsage:    0.7, // Less memory than JSON
	}

	info.Compatibility.Standards = []string{"Protocol Buffers Language Guide"}
	info.Compatibility.LanguageSupport = []LanguageSupport{
		{
			Language:      "Go",
			MinVersion:    "1.12",
			NativeSupport: false,
			Libraries: []Library{
				{
					Name:       "google.golang.org/protobuf",
					Version:    "v1.28+",
					Repository: "https://github.com/protocolbuffers/protobuf-go",
					License:    "BSD",
					Maintained: true,
				},
			},
		},
	}

	return info
}

// MessagePackFormatInfo returns format info for MessagePack
func MessagePackFormatInfo() *FormatInfo {
	info := NewFormatInfo("MessagePack", "application/x-msgpack")
	info.Aliases = []string{"msgpack", "application/msgpack"}
	info.FileExtensions = []string{".msgpack", ".mp"}
	info.Description = "Efficient binary serialization format"
	info.Priority = 30

	info.Capabilities = FormatCapabilities{
		Streaming:          true,
		BinaryEfficient:    true,
		SelfDescribing:     true,
		SupportsNull:       true,
		SupportedDataTypes: []string{"string", "number", "boolean", "null", "binary", "array", "map", "extension"},
		Features:           []string{"compact", "fast", "typed-arrays"},
	}

	info.Performance = PerformanceProfile{
		EncodingSpeed:  2.0,
		DecodingSpeed:  2.2,
		SizeEfficiency: 2.5,
		MemoryUsage:    0.8,
	}

	return info
}

// CBORFormatInfo returns format info for CBOR
func CBORFormatInfo() *FormatInfo {
	info := NewFormatInfo("CBOR", "application/cbor")
	info.Aliases = []string{"cbor"}
	info.FileExtensions = []string{".cbor"}
	info.Description = "Concise Binary Object Representation"
	info.Priority = 40

	info.Capabilities = FormatCapabilities{
		Streaming:          true,
		BinaryEfficient:    true,
		SelfDescribing:     true,
		SupportsNull:       true,
		SupportsReferences: true,
		SupportedDataTypes: []string{"string", "number", "boolean", "null", "binary", "array", "map", "tag"},
		Features:           []string{"indefinite-length", "tags", "bignum"},
	}

	info.Performance = PerformanceProfile{
		EncodingSpeed:  1.8,
		DecodingSpeed:  2.0,
		SizeEfficiency: 2.2,
		MemoryUsage:    0.9,
	}

	info.Compatibility.Standards = []string{"RFC 8949"}

	return info
}

// GetAliases returns the aliases for this format
func (f *FormatInfo) GetAliases() []string {
	return f.Aliases
}
