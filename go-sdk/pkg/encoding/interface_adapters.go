package encoding

import (
	"context"
	"fmt"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// ==============================================================================
// UNIFIED INTERFACE ADAPTERS - Simplified Architecture
// ==============================================================================
// Consolidated adapter system that reduces complexity while maintaining backward
// compatibility. Uses composition over complex inheritance hierarchies.

// EnsureFullEncoder creates an adapter that implements all required encoder interfaces
func EnsureFullEncoder(encoder Encoder) interface {
	Encoder
	ContentTypeProvider
	StreamingCapabilityProvider
} {
	if fullEncoder, ok := encoder.(interface {
		Encoder
		ContentTypeProvider
		StreamingCapabilityProvider
	}); ok {
		return fullEncoder
	}

	// Create unified adapter
	return NewEncoderAdapter(encoder)
}

// EnsureFullDecoder creates an adapter that implements all required decoder interfaces
func EnsureFullDecoder(decoder Decoder) interface {
	Decoder
	StreamingCapabilityProvider
} {
	if fullDecoder, ok := decoder.(interface {
		Decoder
		StreamingCapabilityProvider
	}); ok {
		return fullDecoder
	}

	// Create unified adapter
	return NewDecoderAdapter(decoder)
}

// EnsureFullDecoderWithContentType creates an adapter that implements all required decoder interfaces including ContentTypeProvider
func EnsureFullDecoderWithContentType(decoder Decoder) interface {
	Decoder
	ContentTypeProvider
	StreamingCapabilityProvider
} {
	if fullDecoder, ok := decoder.(interface {
		Decoder
		ContentTypeProvider
		StreamingCapabilityProvider
	}); ok {
		return fullDecoder
	}

	// Create unified adapter
	return NewDecoderAdapter(decoder)
}

// ==============================================================================
// UNIFIED ADAPTER IMPLEMENTATION
// ==============================================================================

// UniversalAdapter provides a single adapter type that can handle multiple interface requirements
// This replaces the previous multiple adapter types with a single, flexible implementation
type UniversalAdapter struct {
	encoder Encoder
	decoder Decoder
}

// NewEncoderAdapter creates a universal adapter focused on encoding
func NewEncoderAdapter(encoder Encoder) *UniversalAdapter {
	return &UniversalAdapter{encoder: encoder}
}

// NewDecoderAdapter creates a universal adapter focused on decoding
func NewDecoderAdapter(decoder Decoder) *UniversalAdapter {
	return &UniversalAdapter{decoder: decoder}
}

// NewCodecAdapter creates a universal adapter for both encoding and decoding
func NewCodecAdapter(encoder Encoder, decoder Decoder) *UniversalAdapter {
	return &UniversalAdapter{encoder: encoder, decoder: decoder}
}

// Encoder interface methods
func (a *UniversalAdapter) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	if a.encoder == nil {
		return nil, fmt.Errorf("encoder not available")
	}
	return a.encoder.Encode(ctx, event)
}

func (a *UniversalAdapter) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	if a.encoder == nil {
		return nil, fmt.Errorf("encoder not available")
	}
	return a.encoder.EncodeMultiple(ctx, events)
}

// Decoder interface methods
func (a *UniversalAdapter) Decode(ctx context.Context, data []byte) (events.Event, error) {
	if a.decoder == nil {
		return nil, fmt.Errorf("decoder not available")
	}
	return a.decoder.Decode(ctx, data)
}

func (a *UniversalAdapter) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	if a.decoder == nil {
		return nil, fmt.Errorf("decoder not available")
	}
	return a.decoder.DecodeMultiple(ctx, data)
}

// ContentTypeProvider interface method
func (a *UniversalAdapter) ContentType() string {
	// Try encoder first
	if a.encoder != nil {
		if provider, ok := a.encoder.(ContentTypeProvider); ok {
			return provider.ContentType()
		}
	}
	// Try decoder second
	if a.decoder != nil {
		if provider, ok := a.decoder.(ContentTypeProvider); ok {
			return provider.ContentType()
		}
	}
	return "application/octet-stream" // Default fallback
}

// StreamingCapabilityProvider interface method
func (a *UniversalAdapter) SupportsStreaming() bool {
	// Try encoder first
	if a.encoder != nil {
		if provider, ok := a.encoder.(StreamingCapabilityProvider); ok {
			return provider.SupportsStreaming()
		}
	}
	// Try decoder second
	if a.decoder != nil {
		if provider, ok := a.decoder.(StreamingCapabilityProvider); ok {
			return provider.SupportsStreaming()
		}
	}
	return false // Default fallback
}

// Legacy compatibility methods
func (a *UniversalAdapter) CanStream() bool {
	return a.SupportsStreaming()
}

// ==============================================================================
// COMPATIBILITY FUNCTIONS - Simplified API
// ==============================================================================

// Simplified replacement for the old specific adapter types
type universalEncoderAdapter = UniversalAdapter
type universalDecoderAdapter = UniversalAdapter
type universalDecoderWithContentTypeAdapter = UniversalAdapter
