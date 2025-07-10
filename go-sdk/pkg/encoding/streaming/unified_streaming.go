package streaming

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/proto/generated"
)

// UnifiedStreamCodec provides a format-agnostic streaming wrapper
type UnifiedStreamCodec struct {
	// Base codec for encoding/decoding
	baseCodec encoding.StreamCodec

	// Stream management
	streamManager *StreamManager

	// Chunked encoding
	chunkedEncoder *ChunkedEncoder

	// Configuration
	config *UnifiedStreamConfig

	// Metrics
	metrics *StreamMetrics

	// State
	mu     sync.RWMutex
	active bool
}

// UnifiedStreamConfig holds configuration for unified streaming
type UnifiedStreamConfig struct {
	// EnableChunking enables automatic chunking for large streams
	EnableChunking bool

	// EnableFlowControl enables flow control and backpressure
	EnableFlowControl bool

	// EnableMetrics enables metrics collection
	EnableMetrics bool

	// EnableProgressTracking enables progress tracking
	EnableProgressTracking bool

	// StreamConfig is the configuration for stream management
	StreamConfig *StreamConfig

	// ChunkConfig is the configuration for chunked encoding
	ChunkConfig *ChunkConfig

	// Format is the underlying format (e.g., "json", "protobuf")
	Format string
}

// DefaultUnifiedStreamConfig returns default configuration
func DefaultUnifiedStreamConfig() *UnifiedStreamConfig {
	return &UnifiedStreamConfig{
		EnableChunking:         true,
		EnableFlowControl:      true,
		EnableMetrics:          true,
		EnableProgressTracking: true,
		StreamConfig:           DefaultStreamConfig(),
		ChunkConfig:            DefaultChunkConfig(),
	}
}

// NewUnifiedStreamCodec creates a new unified streaming codec
func NewUnifiedStreamCodec(baseCodec encoding.StreamCodec, config *UnifiedStreamConfig) *UnifiedStreamCodec {
	if config == nil {
		config = DefaultUnifiedStreamConfig()
	}

	// Set format from base codec if not specified
	if config.Format == "" {
		config.Format = baseCodec.ContentType()
	}

	usc := &UnifiedStreamCodec{
		baseCodec: baseCodec,
		config:    config,
	}

	// Initialize components based on configuration
	if config.EnableFlowControl || config.EnableChunking {
		usc.streamManager = NewStreamManager(
			baseCodec.GetStreamEncoder(),
			baseCodec.GetStreamDecoder(),
			config.StreamConfig,
		)
	}

	if config.EnableChunking {
		usc.chunkedEncoder = NewChunkedEncoder(baseCodec, config.ChunkConfig)
	}

	if config.EnableMetrics {
		usc.metrics = NewStreamMetrics()
	}

	return usc
}

// StreamEncode encodes events from a channel with all enhancements
func (usc *UnifiedStreamCodec) StreamEncode(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	usc.mu.Lock()
	if usc.active {
		usc.mu.Unlock()
		return fmt.Errorf("stream already active")
	}
	usc.active = true
	usc.mu.Unlock()

	defer func() {
		usc.mu.Lock()
		usc.active = false
		usc.mu.Unlock()
	}()

	// Start stream manager if enabled
	if usc.streamManager != nil {
		if err := usc.streamManager.Start(); err != nil {
			return fmt.Errorf("failed to start stream manager: %w", err)
		}
		defer usc.streamManager.Stop()
	}

	// Use appropriate encoding strategy
	if usc.config.EnableChunking {
		return usc.streamEncodeChunked(ctx, input, output)
	}

	// Use stream manager if available
	if usc.streamManager != nil {
		return usc.streamManager.WriteStream(ctx, input, output)
	}

	// Fall back to base codec
	return usc.baseCodec.GetStreamEncoder().EncodeStream(ctx, input, output)
}

// StreamDecode decodes events to a channel with all enhancements
func (usc *UnifiedStreamCodec) StreamDecode(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	usc.mu.Lock()
	if usc.active {
		usc.mu.Unlock()
		return fmt.Errorf("stream already active")
	}
	usc.active = true
	usc.mu.Unlock()

	defer func() {
		usc.mu.Lock()
		usc.active = false
		usc.mu.Unlock()
	}()

	// Start stream manager if enabled
	if usc.streamManager != nil {
		if err := usc.streamManager.Start(); err != nil {
			return fmt.Errorf("failed to start stream manager: %w", err)
		}
		defer usc.streamManager.Stop()

		return usc.streamManager.ReadStream(ctx, input, output)
	}

	// Fall back to base codec
	return usc.baseCodec.GetStreamDecoder().DecodeStream(ctx, input, output)
}

// streamEncodeChunked handles chunked encoding
func (usc *UnifiedStreamCodec) streamEncodeChunked(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	// Create chunk channel
	chunkChan := make(chan *Chunk, usc.config.ChunkConfig.ProcessorCount)

	// Start chunk encoder
	encodeErr := make(chan error, 1)
	go func() {
		encodeErr <- usc.chunkedEncoder.EncodeChunked(ctx, input, chunkChan)
	}()

	// Start base encoder for output
	encoder := usc.baseCodec.GetStreamEncoder()
	if err := encoder.StartStream(output); err != nil {
		return fmt.Errorf("failed to start output stream: %w", err)
	}
	defer encoder.EndStream()

	// Process chunks
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-encodeErr:
			if err != nil {
				return fmt.Errorf("chunk encoding error: %w", err)
			}
			return nil // Encoding completed
		case chunk, ok := <-chunkChan:
			if !ok {
				// All chunks processed
				select {
				case err := <-encodeErr:
					return err
				default:
					return nil
				}
			}

			// Create chunk event
			chunkEvent := NewChunkEvent(chunk.Header, chunk.Data)

			// Write chunk as event
			if err := encoder.WriteEvent(chunkEvent); err != nil {
				return fmt.Errorf("failed to write chunk: %w", err)
			}

			// Update metrics
			if usc.metrics != nil {
				usc.metrics.RecordEvent(chunkEvent)
			}

			// Report progress
			if usc.config.EnableProgressTracking {
				metrics := usc.chunkedEncoder.GetMetrics()
				usc.reportProgress(metrics.EventsProcessed.Load(), 0)
			}
		}
	}
}

// Encode implements encoding.Encoder
func (usc *UnifiedStreamCodec) Encode(event events.Event) ([]byte, error) {
	return usc.baseCodec.Encode(event)
}

// EncodeMultiple implements encoding.Encoder
func (usc *UnifiedStreamCodec) EncodeMultiple(events []events.Event) ([]byte, error) {
	return usc.baseCodec.EncodeMultiple(events)
}

// Decode implements encoding.Decoder
func (usc *UnifiedStreamCodec) Decode(data []byte) (events.Event, error) {
	return usc.baseCodec.Decode(data)
}

// DecodeMultiple implements encoding.Decoder
func (usc *UnifiedStreamCodec) DecodeMultiple(data []byte) ([]events.Event, error) {
	return usc.baseCodec.DecodeMultiple(data)
}

// ContentType returns the content type
func (usc *UnifiedStreamCodec) ContentType() string {
	return usc.baseCodec.ContentType()
}

// CanStream returns true
func (usc *UnifiedStreamCodec) CanStream() bool {
	return true
}

// GetStreamEncoder returns the stream encoder
func (usc *UnifiedStreamCodec) GetStreamEncoder() encoding.StreamEncoder {
	return &unifiedStreamEncoder{codec: usc}
}

// GetStreamDecoder returns the stream decoder
func (usc *UnifiedStreamCodec) GetStreamDecoder() encoding.StreamDecoder {
	return &unifiedStreamDecoder{codec: usc}
}

// GetMetrics returns current metrics
func (usc *UnifiedStreamCodec) GetMetrics() *StreamMetrics {
	return usc.metrics
}

// GetStreamManager returns the stream manager
func (usc *UnifiedStreamCodec) GetStreamManager() *StreamManager {
	return usc.streamManager
}

// RegisterProgressCallback registers a progress callback
func (usc *UnifiedStreamCodec) RegisterProgressCallback(callback func(processed, total int64)) {
	if usc.chunkedEncoder != nil {
		usc.chunkedEncoder.RegisterProgressCallback(callback)
	}
}

// reportProgress reports progress
func (usc *UnifiedStreamCodec) reportProgress(processed, total int64) {
	if usc.metrics != nil {
		usc.metrics.UpdateProgress(processed, total)
	}
}

// unifiedStreamEncoder wraps UnifiedStreamCodec as StreamEncoder
type unifiedStreamEncoder struct {
	codec  *UnifiedStreamCodec
	writer io.Writer
}

func (e *unifiedStreamEncoder) Encode(event events.Event) ([]byte, error) {
	return e.codec.Encode(event)
}

func (e *unifiedStreamEncoder) EncodeMultiple(events []events.Event) ([]byte, error) {
	return e.codec.EncodeMultiple(events)
}

func (e *unifiedStreamEncoder) ContentType() string {
	return e.codec.ContentType()
}

func (e *unifiedStreamEncoder) CanStream() bool {
	return true
}

func (e *unifiedStreamEncoder) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	return e.codec.StreamEncode(ctx, input, output)
}

func (e *unifiedStreamEncoder) StartStream(w io.Writer) error {
	e.writer = w
	return nil
}

func (e *unifiedStreamEncoder) WriteEvent(event events.Event) error {
	if e.writer == nil {
		return fmt.Errorf("stream not started")
	}
	data, err := e.codec.Encode(event)
	if err != nil {
		return err
	}
	_, err = e.writer.Write(data)
	return err
}

func (e *unifiedStreamEncoder) EndStream() error {
	e.writer = nil
	return nil
}

// unifiedStreamDecoder wraps UnifiedStreamCodec as StreamDecoder
type unifiedStreamDecoder struct {
	codec  *UnifiedStreamCodec
	reader io.Reader
}

func (d *unifiedStreamDecoder) Decode(data []byte) (events.Event, error) {
	return d.codec.Decode(data)
}

func (d *unifiedStreamDecoder) DecodeMultiple(data []byte) ([]events.Event, error) {
	return d.codec.DecodeMultiple(data)
}

func (d *unifiedStreamDecoder) ContentType() string {
	return d.codec.ContentType()
}

func (d *unifiedStreamDecoder) CanStream() bool {
	return true
}

func (d *unifiedStreamDecoder) DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	return d.codec.StreamDecode(ctx, input, output)
}

func (d *unifiedStreamDecoder) StartStream(r io.Reader) error {
	d.reader = r
	return nil
}

func (d *unifiedStreamDecoder) ReadEvent() (events.Event, error) {
	if d.reader == nil {
		return nil, fmt.Errorf("stream not started")
	}
	// This is a simplified implementation
	// In practice, this would need proper buffering and parsing
	return nil, fmt.Errorf("not implemented")
}

func (d *unifiedStreamDecoder) EndStream() error {
	d.reader = nil
	return nil
}

// ChunkEvent represents a chunk as an event
type ChunkEvent struct {
	*events.BaseEvent
	Header ChunkHeader
	Data   []byte
}

// NewChunkEvent creates a new chunk event
func NewChunkEvent(header ChunkHeader, data []byte) *ChunkEvent {
	return &ChunkEvent{
		BaseEvent: events.NewBaseEvent(events.EventType("chunk")),
		Header:    header,
		Data:      data,
	}
}

// Validate implements events.Event
func (ce *ChunkEvent) Validate() error {
	if err := ce.BaseEvent.Validate(); err != nil {
		return err
	}
	if ce.Header.ChunkID == "" {
		return fmt.Errorf("chunk ID is required")
	}
	if len(ce.Data) == 0 {
		return fmt.Errorf("chunk data is empty")
	}
	return nil
}

// ToJSON implements events.Event
func (ce *ChunkEvent) ToJSON() ([]byte, error) {
	// Simplified JSON representation
	return []byte(fmt.Sprintf(`{"type":"chunk","id":"%s","size":%d}`, 
		ce.Header.ChunkID, ce.Header.ByteSize)), nil
}

// ToProtobuf implements events.Event
func (ce *ChunkEvent) ToProtobuf() (*generated.Event, error) {
	// This would need proper protobuf definition
	return nil, fmt.Errorf("chunk events don't support protobuf")
}