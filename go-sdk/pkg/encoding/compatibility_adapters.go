package encoding

import (
	"context"
	"fmt"
	"io"

	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// ==============================================================================
// BACKWARD COMPATIBILITY ADAPTERS
// ==============================================================================
// These adapters help bridge between the old monolithic interfaces and the new
// focused interfaces, ensuring existing code continues to work.

// codecToLegacyEncoder adapts a Codec to implement LegacyEncoder
type codecToLegacyEncoder struct {
	codec Codec
	cap   StreamingCapabilityProvider
}

// NewLegacyEncoderFromCodec creates a LegacyEncoder from focused interfaces
func NewLegacyEncoderFromCodec(codec Codec, cap StreamingCapabilityProvider) LegacyEncoder {
	return &codecToLegacyEncoder{
		codec: codec,
		cap:   cap,
	}
}

func (a *codecToLegacyEncoder) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return a.codec.Encode(ctx, event)
}

func (a *codecToLegacyEncoder) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return a.codec.EncodeMultiple(ctx, events)
}

func (a *codecToLegacyEncoder) ContentType() string {
	return a.codec.ContentType()
}

func (a *codecToLegacyEncoder) SupportsStreaming() bool {
	if a.cap != nil {
		return a.cap.SupportsStreaming()
	}
	return false
}

func (a *codecToLegacyEncoder) CanStream() bool {
	return a.SupportsStreaming()
}

// codecToLegacyDecoder adapts a Codec to implement LegacyDecoder
type codecToLegacyDecoder struct {
	codec Codec
	cap   StreamingCapabilityProvider
}

// NewLegacyDecoderFromCodec creates a LegacyDecoder from focused interfaces
func NewLegacyDecoderFromCodec(codec Codec, cap StreamingCapabilityProvider) LegacyDecoder {
	return &codecToLegacyDecoder{
		codec: codec,
		cap:   cap,
	}
}

func (a *codecToLegacyDecoder) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return a.codec.Decode(ctx, data)
}

func (a *codecToLegacyDecoder) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return a.codec.DecodeMultiple(ctx, data)
}

func (a *codecToLegacyDecoder) ContentType() string {
	return a.codec.ContentType()
}

func (a *codecToLegacyDecoder) SupportsStreaming() bool {
	if a.cap != nil {
		return a.cap.SupportsStreaming()
	}
	return false
}

func (a *codecToLegacyDecoder) CanStream() bool {
	return a.SupportsStreaming()
}

// streamCodecToLegacy adapts new streaming interfaces to the old LegacyStreamCodec
type streamCodecToLegacy struct {
	codec     Codec
	stream    StreamCodec
	session   StreamSessionManager
	processor StreamEventProcessor
}

// NewLegacyStreamCodecFromInterfaces creates a LegacyStreamCodec from focused interfaces
func NewLegacyStreamCodecFromInterfaces(
	codec Codec,
	stream StreamCodec,
	session StreamSessionManager,
	processor StreamEventProcessor,
) LegacyStreamCodec {
	return &streamCodecToLegacy{
		codec:     codec,
		stream:    stream,
		session:   session,
		processor: processor,
	}
}

// Basic Codec interface methods
func (a *streamCodecToLegacy) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return a.codec.Encode(ctx, event)
}

func (a *streamCodecToLegacy) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return a.codec.EncodeMultiple(ctx, events)
}

func (a *streamCodecToLegacy) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return a.codec.Decode(ctx, data)
}

func (a *streamCodecToLegacy) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return a.codec.DecodeMultiple(ctx, data)
}

func (a *streamCodecToLegacy) ContentType() string {
	return a.codec.ContentType()
}

func (a *streamCodecToLegacy) SupportsStreaming() bool {
	return a.stream.SupportsStreaming()
}

func (a *streamCodecToLegacy) CanStream() bool {
	return a.SupportsStreaming()
}

// Stream operations
func (a *streamCodecToLegacy) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	return a.stream.EncodeStream(ctx, input, output)
}

func (a *streamCodecToLegacy) DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	return a.stream.DecodeStream(ctx, input, output)
}

// Session management (legacy naming)
func (a *streamCodecToLegacy) StartEncoding(ctx context.Context, w io.Writer) error {
	if a.session != nil {
		return a.session.StartEncodingSession(ctx, w)
	}
	return fmt.Errorf("session management not available")
}

func (a *streamCodecToLegacy) StartDecoding(ctx context.Context, r io.Reader) error {
	if a.session != nil {
		return a.session.StartDecodingSession(ctx, r)
	}
	return fmt.Errorf("session management not available")
}

func (a *streamCodecToLegacy) EndEncoding(ctx context.Context) error {
	if a.session != nil {
		return a.session.EndSession(ctx)
	}
	return fmt.Errorf("session management not available")
}

func (a *streamCodecToLegacy) EndDecoding(ctx context.Context) error {
	if a.session != nil {
		return a.session.EndSession(ctx)
	}
	return fmt.Errorf("session management not available")
}

// Event processing
func (a *streamCodecToLegacy) WriteEvent(ctx context.Context, event events.Event) error {
	if a.processor != nil {
		return a.processor.WriteEvent(ctx, event)
	}
	return fmt.Errorf("event processing not available")
}

func (a *streamCodecToLegacy) ReadEvent(ctx context.Context) (events.Event, error) {
	if a.processor != nil {
		return a.processor.ReadEvent(ctx)
	}
	return nil, fmt.Errorf("event processing not available")
}

// StreamCodec methods (inherited from embedded interface)
func (a *streamCodecToLegacy) GetStreamEncoder() StreamEncoder {
	return a.stream.GetStreamEncoder()
}

func (a *streamCodecToLegacy) GetStreamDecoder() StreamDecoder {
	return a.stream.GetStreamDecoder()
}

// Stub implementations for legacy stream encoder/decoder getters
type legacyStreamEncoderStub struct {
	codec  Codec
	stream StreamCodec
}

func (s *legacyStreamEncoderStub) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return s.codec.Encode(ctx, event)
}

func (s *legacyStreamEncoderStub) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return s.codec.EncodeMultiple(ctx, events)
}

func (s *legacyStreamEncoderStub) ContentType() string {
	return s.codec.ContentType()
}

func (s *legacyStreamEncoderStub) SupportsStreaming() bool {
	return s.stream.SupportsStreaming()
}

func (s *legacyStreamEncoderStub) CanStream() bool {
	return s.SupportsStreaming()
}

func (s *legacyStreamEncoderStub) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	return s.stream.EncodeStream(ctx, input, output)
}

func (s *legacyStreamEncoderStub) StartStream(ctx context.Context, w io.Writer) error {
	return fmt.Errorf("session management not available in stub")
}

func (s *legacyStreamEncoderStub) WriteEvent(ctx context.Context, event events.Event) error {
	return fmt.Errorf("event processing not available in stub")
}

func (s *legacyStreamEncoderStub) EndStream(ctx context.Context) error {
	return fmt.Errorf("session management not available in stub")
}

type legacyStreamDecoderStub struct {
	codec  Codec
	stream StreamCodec
}

func (s *legacyStreamDecoderStub) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return s.codec.Decode(ctx, data)
}

func (s *legacyStreamDecoderStub) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return s.codec.DecodeMultiple(ctx, data)
}

func (s *legacyStreamDecoderStub) ContentType() string {
	return s.codec.ContentType()
}

func (s *legacyStreamDecoderStub) SupportsStreaming() bool {
	return s.stream.SupportsStreaming()
}

func (s *legacyStreamDecoderStub) CanStream() bool {
	return s.SupportsStreaming()
}

func (s *legacyStreamDecoderStub) DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	return s.stream.DecodeStream(ctx, input, output)
}

func (s *legacyStreamDecoderStub) StartStream(ctx context.Context, r io.Reader) error {
	return fmt.Errorf("session management not available in stub")
}

func (s *legacyStreamDecoderStub) ReadEvent(ctx context.Context) (events.Event, error) {
	return nil, fmt.Errorf("event processing not available in stub")
}

func (s *legacyStreamDecoderStub) EndStream(ctx context.Context) error {
	return fmt.Errorf("session management not available in stub")
}

// ==============================================================================
// FACTORY ADAPTERS
// ==============================================================================

// legacyCodecFactoryAdapter adapts the old CodecFactory interface to the new one
type legacyCodecFactoryAdapter struct {
	factory FullCodecFactory
}

// NewLegacyCodecFactory creates a legacy-compatible codec factory from a modern factory
func NewLegacyCodecFactory(factory FullCodecFactory) *legacyCodecFactoryAdapter {
	return &legacyCodecFactoryAdapter{factory: factory}
}

// CreateCodec implements the old CodecFactory interface
func (a *legacyCodecFactoryAdapter) CreateCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (Codec, error) {
	return a.factory.CreateCodec(ctx, contentType, encOptions, decOptions)
}

// CreateStreamCodec implements the old CodecFactory interface for streaming
func (a *legacyCodecFactoryAdapter) CreateStreamCodec(ctx context.Context, contentType string, encOptions *EncodingOptions, decOptions *DecodingOptions) (StreamCodec, error) {
	return a.factory.CreateStreamCodec(ctx, contentType, encOptions, decOptions)
}

// SupportedTypes returns list of supported content types
func (a *legacyCodecFactoryAdapter) SupportedTypes() []string {
	return a.factory.SupportedTypes()
}

// SupportsStreaming indicates if streaming is supported for the given content type
func (a *legacyCodecFactoryAdapter) SupportsStreaming(contentType string) bool {
	return a.factory.SupportsStreaming(contentType)
}