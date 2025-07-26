package transport

import (
	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// Serializer handles serialization and deserialization of events for transport.
type Serializer interface {
	// Serialize converts an event to bytes for transport.
	Serialize(event TransportEvent) ([]byte, error)

	// Deserialize converts bytes back to an event.
	Deserialize(data []byte) (events.Event, error)

	// ContentType returns the content type for the serialized data.
	ContentType() string

	// SupportedTypes returns the types that this serializer can handle.
	SupportedTypes() []string
}

// Compressor handles compression and decompression of serialized data.
type Compressor interface {
	// Compress compresses the input data.
	Compress(data []byte) ([]byte, error)

	// Decompress decompresses the input data.
	Decompress(data []byte) ([]byte, error)

	// Algorithm returns the compression algorithm name.
	Algorithm() string

	// CompressionRatio returns the achieved compression ratio.
	CompressionRatio() float64
}