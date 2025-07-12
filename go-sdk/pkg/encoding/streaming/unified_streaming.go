package streaming

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/errors"
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
		return errors.NewStreamingError("STREAM_ALREADY_ACTIVE", "stream already active")
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
			return errors.NewStreamingError("STREAM_MANAGER_START_FAILED", "failed to start stream manager").WithCause(err)
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
		return errors.NewStreamingError("STREAM_ALREADY_ACTIVE", "stream already active")
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
			return errors.NewStreamingError("STREAM_MANAGER_START_FAILED", "failed to start stream manager").WithCause(err)
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
	if err := encoder.StartStream(ctx, output); err != nil {
		return errors.NewStreamingError("OUTPUT_STREAM_START_FAILED", "failed to start output stream").WithCause(err)
	}
	defer encoder.EndStream(ctx)

	// Process chunks with proper cleanup on cancellation
	for {
		select {
		case <-ctx.Done():
			// Context cancelled - ensure proper cleanup
			// Drain any remaining chunks to prevent goroutine leaks
			go func() {
				for range chunkChan {
					// Drain remaining chunks
				}
			}()
			return ctx.Err()
		case err := <-encodeErr:
			if err != nil {
				return errors.NewStreamingError("CHUNK_ENCODING_FAILED", "chunk encoding error").WithCause(err)
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
			if err := encoder.WriteEvent(ctx, chunkEvent); err != nil {
				return errors.NewStreamingError("CHUNK_WRITE_FAILED", "failed to write chunk").WithCause(err)
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
func (usc *UnifiedStreamCodec) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return usc.baseCodec.Encode(ctx, event)
}

// EncodeMultiple implements encoding.Encoder
func (usc *UnifiedStreamCodec) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return usc.baseCodec.EncodeMultiple(ctx, events)
}

// Decode implements encoding.Decoder
func (usc *UnifiedStreamCodec) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return usc.baseCodec.Decode(ctx, data)
}

// DecodeMultiple implements encoding.Decoder
func (usc *UnifiedStreamCodec) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return usc.baseCodec.DecodeMultiple(ctx, data)
}

// ContentType returns the content type
func (usc *UnifiedStreamCodec) ContentType() string {
	return usc.baseCodec.ContentType()
}

// CanStream returns true
func (usc *UnifiedStreamCodec) CanStream() bool {
	return true
}

// SupportsStreaming indicates if this codec has streaming capabilities
func (usc *UnifiedStreamCodec) SupportsStreaming() bool {
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

// EncodeStream implements StreamCodec - encodes events from a channel to a writer
func (usc *UnifiedStreamCodec) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	return usc.StreamEncode(ctx, input, output)
}

// DecodeStream implements StreamCodec - decodes events from a reader to a channel
func (usc *UnifiedStreamCodec) DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	return usc.StreamDecode(ctx, input, output)
}

// StartEncoding implements StreamCodec - initializes a streaming encoding session
func (usc *UnifiedStreamCodec) StartEncoding(ctx context.Context, w io.Writer) error {
	return usc.baseCodec.StartEncoding(ctx, w)
}

// WriteEvent implements StreamCodec - writes a single event to the encoding stream
func (usc *UnifiedStreamCodec) WriteEvent(ctx context.Context, event events.Event) error {
	return usc.baseCodec.WriteEvent(ctx, event)
}

// EndEncoding implements StreamCodec - finalizes the streaming encoding session
func (usc *UnifiedStreamCodec) EndEncoding(ctx context.Context) error {
	return usc.baseCodec.EndEncoding(ctx)
}

// StartDecoding implements StreamCodec - initializes a streaming decoding session
func (usc *UnifiedStreamCodec) StartDecoding(ctx context.Context, r io.Reader) error {
	return usc.baseCodec.StartDecoding(ctx, r)
}

// ReadEvent implements StreamCodec - reads a single event from the decoding stream
func (usc *UnifiedStreamCodec) ReadEvent(ctx context.Context) (events.Event, error) {
	return usc.baseCodec.ReadEvent(ctx)
}

// EndDecoding implements StreamCodec - finalizes the streaming decoding session
func (usc *UnifiedStreamCodec) EndDecoding(ctx context.Context) error {
	return usc.baseCodec.EndDecoding(ctx)
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

func (e *unifiedStreamEncoder) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return e.codec.Encode(ctx, event)
}

func (e *unifiedStreamEncoder) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return e.codec.EncodeMultiple(ctx, events)
}

func (e *unifiedStreamEncoder) ContentType() string {
	return e.codec.ContentType()
}

func (e *unifiedStreamEncoder) CanStream() bool {
	return true
}

func (e *unifiedStreamEncoder) SupportsStreaming() bool {
	return true
}

func (e *unifiedStreamEncoder) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	return e.codec.StreamEncode(ctx, input, output)
}

func (e *unifiedStreamEncoder) StartStream(ctx context.Context, w io.Writer) error {
	e.writer = w
	return nil
}

func (e *unifiedStreamEncoder) WriteEvent(ctx context.Context, event events.Event) error {
	if e.writer == nil {
		return errors.NewStreamingError("STREAM_NOT_STARTED", "stream not started")
	}
	data, err := e.codec.Encode(ctx, event)
	if err != nil {
		return err
	}
	_, err = e.writer.Write(data)
	return err
}

func (e *unifiedStreamEncoder) EndStream(ctx context.Context) error {
	e.writer = nil
	return nil
}

// unifiedStreamDecoder wraps UnifiedStreamCodec as StreamDecoder
type unifiedStreamDecoder struct {
	codec  *UnifiedStreamCodec
	reader io.Reader
}

func (d *unifiedStreamDecoder) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return d.codec.Decode(ctx, data)
}

func (d *unifiedStreamDecoder) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return d.codec.DecodeMultiple(ctx, data)
}

func (d *unifiedStreamDecoder) ContentType() string {
	return d.codec.ContentType()
}

func (d *unifiedStreamDecoder) CanStream() bool {
	return true
}

func (d *unifiedStreamDecoder) SupportsStreaming() bool {
	return true
}

func (d *unifiedStreamDecoder) DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	return d.codec.StreamDecode(ctx, input, output)
}

func (d *unifiedStreamDecoder) StartStream(ctx context.Context, r io.Reader) error {
	d.reader = r
	return nil
}

func (d *unifiedStreamDecoder) ReadEvent(ctx context.Context) (events.Event, error) {
	if d.reader == nil {
		return nil, errors.NewStreamingError("STREAM_NOT_STARTED", "stream not started")
	}
	// This is a simplified implementation
	// In practice, this would need proper buffering and parsing
	return nil, errors.NewStreamingError("NOT_IMPLEMENTED", "streaming decode not implemented")
}

func (d *unifiedStreamDecoder) EndStream(ctx context.Context) error {
	d.reader = nil
	return nil
}

// ChunkEvent represents a chunk as a custom event
type ChunkEvent struct {
	*events.CustomEvent
	Header ChunkHeader
	Data   []byte
}

// NewChunkEvent creates a new chunk event as a custom event
func NewChunkEvent(header ChunkHeader, data []byte) *ChunkEvent {
	// Create custom event with chunk data
	customEvent := events.NewCustomEvent("streaming.chunk", 
		events.WithValue(map[string]interface{}{
			"chunk_id":     header.ChunkID,
			"sequence_num": header.SequenceNum,
			"event_count":  header.EventCount,
			"byte_size":    header.ByteSize,
			"compressed":   header.Compressed,
			"checksum":     header.Checksum,
		}),
	)
	
	return &ChunkEvent{
		CustomEvent: customEvent,
		Header:      header,
		Data:        data,
	}
}

// Validate implements events.Event
func (ce *ChunkEvent) Validate() error {
	if err := ce.CustomEvent.Validate(); err != nil {
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