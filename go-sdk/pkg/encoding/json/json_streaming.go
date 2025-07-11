package json

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
)

// StreamingJSONEncoder implements the StreamEncoder interface for NDJSON format
type StreamingJSONEncoder struct {
	*JSONEncoder
	writer     io.Writer
	bufWriter  *bufio.Writer
	streamMu   sync.Mutex
	inStream   bool
}

// NewStreamingJSONEncoder creates a new streaming JSON encoder
func NewStreamingJSONEncoder(options *encoding.EncodingOptions) *StreamingJSONEncoder {
	return &StreamingJSONEncoder{
		JSONEncoder: NewJSONEncoder(options),
	}
}

// StartStream initializes a streaming session
func (e *StreamingJSONEncoder) StartStream(ctx context.Context, w io.Writer) error {
	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return &encoding.EncodingError{
			Format:  "json",
			Message: "context cancelled",
			Cause:   err,
		}
	}

	e.streamMu.Lock()
	defer e.streamMu.Unlock()

	if e.inStream {
		return &encoding.EncodingError{
			Format:  "json",
			Message: "stream already started",
		}
	}

	e.writer = w
	bufSize := e.options.BufferSize
	if bufSize <= 0 {
		bufSize = 4096 // Default 4KB buffer
	}
	e.bufWriter = bufio.NewWriterSize(w, bufSize)
	e.inStream = true

	return nil
}

// WriteEvent writes a single event to the stream
func (e *StreamingJSONEncoder) WriteEvent(ctx context.Context, event events.Event) error {
	e.streamMu.Lock()
	defer e.streamMu.Unlock()

	if !e.inStream {
		return &encoding.EncodingError{
			Format:  "json",
			Message: "stream not started",
		}
	}

	// Encode the event
	data, err := e.Encode(ctx, event)
	if err != nil {
		return err
	}

	// Write the event followed by a newline (NDJSON format)
	if _, err := e.bufWriter.Write(data); err != nil {
		return &encoding.EncodingError{
			Format:  "json",
			Event:   event,
			Message: "failed to write event to stream",
			Cause:   err,
		}
	}

	if _, err := e.bufWriter.Write([]byte("\n")); err != nil {
		return &encoding.EncodingError{
			Format:  "json",
			Event:   event,
			Message: "failed to write newline to stream",
			Cause:   err,
		}
	}

	// Flush the buffer to ensure timely delivery
	if err := e.bufWriter.Flush(); err != nil {
		return &encoding.EncodingError{
			Format:  "json",
			Event:   event,
			Message: "failed to flush stream",
			Cause:   err,
		}
	}

	return nil
}

// EndStream finalizes the streaming session
func (e *StreamingJSONEncoder) EndStream(ctx context.Context) error {
	e.streamMu.Lock()
	defer e.streamMu.Unlock()

	if !e.inStream {
		return nil
	}

	// Final flush
	if e.bufWriter != nil {
		if err := e.bufWriter.Flush(); err != nil {
			return &encoding.EncodingError{
				Format:  "json",
				Message: "failed to flush stream on close",
				Cause:   err,
			}
		}
	}

	e.writer = nil
	e.bufWriter = nil
	e.inStream = false

	return nil
}

// EncodeStream encodes events from a channel to a writer
func (e *StreamingJSONEncoder) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	if err := e.StartStream(ctx, output); err != nil {
		return err
	}
	defer e.EndStream(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-input:
			if !ok {
				// Channel closed, end streaming
				return nil
			}
			if err := e.WriteEvent(ctx, event); err != nil {
				return err
			}
		}
	}
}

// StreamingJSONDecoder implements the StreamDecoder interface for NDJSON format
type StreamingJSONDecoder struct {
	*JSONDecoder
	reader    io.Reader
	scanner   *bufio.Scanner
	streamMu  sync.Mutex
	inStream  bool
}

// NewStreamingJSONDecoder creates a new streaming JSON decoder
func NewStreamingJSONDecoder(options *encoding.DecodingOptions) *StreamingJSONDecoder {
	return &StreamingJSONDecoder{
		JSONDecoder: NewJSONDecoder(options),
	}
}

// StartStream initializes a streaming session
func (d *StreamingJSONDecoder) StartStream(ctx context.Context, r io.Reader) error {
	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return &encoding.DecodingError{
			Format:  "json",
			Message: "context cancelled",
			Cause:   err,
		}
	}

	d.streamMu.Lock()
	defer d.streamMu.Unlock()

	if d.inStream {
		return &encoding.DecodingError{
			Format:  "json",
			Message: "stream already started",
		}
	}

	d.reader = r
	d.scanner = bufio.NewScanner(r)
	
	// Set max token size based on options
	if d.options.MaxSize > 0 {
		d.scanner.Buffer(make([]byte, 0, d.options.BufferSize), int(d.options.MaxSize))
	} else if d.options.BufferSize > 0 {
		d.scanner.Buffer(make([]byte, 0, d.options.BufferSize), d.options.BufferSize*10)
	}

	d.inStream = true

	return nil
}

// ReadEvent reads a single event from the stream
func (d *StreamingJSONDecoder) ReadEvent(ctx context.Context) (events.Event, error) {
	d.streamMu.Lock()
	defer d.streamMu.Unlock()

	if !d.inStream {
		return nil, &encoding.DecodingError{
			Format:  "json",
			Message: "stream not started",
		}
	}

	// Read next line (NDJSON format)
	if !d.scanner.Scan() {
		if err := d.scanner.Err(); err != nil {
			return nil, &encoding.DecodingError{
				Format:  "json",
				Message: "failed to read from stream",
				Cause:   err,
			}
		}
		// EOF
		return nil, io.EOF
	}

	line := d.scanner.Bytes()
	
	// Skip empty lines
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		// Try next line in a loop to avoid stack overflow from recursion
		for {
			if !d.scanner.Scan() {
				if err := d.scanner.Err(); err != nil {
					return nil, &encoding.DecodingError{
						Format:  "json",
						Message: "failed to read from stream",
						Cause:   err,
					}
				}
				// EOF
				return nil, io.EOF
			}
			line = bytes.TrimSpace(d.scanner.Bytes())
			if len(line) > 0 {
				break
			}
		}
	}

	// Decode the event
	return d.Decode(ctx, line)
}

// EndStream finalizes the streaming session
func (d *StreamingJSONDecoder) EndStream(ctx context.Context) error {
	d.streamMu.Lock()
	defer d.streamMu.Unlock()

	if !d.inStream {
		return nil
	}

	d.reader = nil
	d.scanner = nil
	d.inStream = false

	return nil
}

// DecodeStream decodes events from a reader to a channel
func (d *StreamingJSONDecoder) DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	if err := d.StartStream(ctx, input); err != nil {
		return err
	}
	defer d.EndStream(ctx)
	defer close(output)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			event, err := d.ReadEvent(ctx)
			if err == io.EOF {
				// Normal end of stream
				return nil
			}
			if err != nil {
				return err
			}

			// Send event to output channel
			select {
			case <-ctx.Done():
				return ctx.Err()
			case output <- event:
				// Event sent successfully
			}
		}
	}
}

// JSONStreamCodec implements the simplified StreamCodec interface for JSON/NDJSON format
type JSONStreamCodec struct {
	encoder *StreamingJSONEncoder
	decoder *StreamingJSONDecoder
}

// NewJSONStreamCodec creates a new JSON stream codec
func NewJSONStreamCodec(encOptions *encoding.EncodingOptions, decOptions *encoding.DecodingOptions) *JSONStreamCodec {
	return &JSONStreamCodec{
		encoder: NewStreamingJSONEncoder(encOptions),
		decoder: NewStreamingJSONDecoder(decOptions),
	}
}

// ContentType returns the MIME type for NDJSON streaming
func (c *JSONStreamCodec) ContentType() string {
	return "application/x-ndjson"
}

// EncodeStream encodes events from a channel to a writer
func (c *JSONStreamCodec) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	return c.encoder.EncodeStream(ctx, input, output)
}

// DecodeStream decodes events from a reader to a channel
func (c *JSONStreamCodec) DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	return c.decoder.DecodeStream(ctx, input, output)
}

// StartEncoding initializes a streaming encoding session
func (c *JSONStreamCodec) StartEncoding(ctx context.Context, w io.Writer) error {
	return c.encoder.StartStream(ctx, w)
}

// WriteEvent writes a single event to the encoding stream
func (c *JSONStreamCodec) WriteEvent(ctx context.Context, event events.Event) error {
	return c.encoder.WriteEvent(ctx, event)
}

// EndEncoding finalizes the streaming encoding session
func (c *JSONStreamCodec) EndEncoding(ctx context.Context) error {
	return c.encoder.EndStream(ctx)
}

// StartDecoding initializes a streaming decoding session
func (c *JSONStreamCodec) StartDecoding(ctx context.Context, r io.Reader) error {
	return c.decoder.StartStream(ctx, r)
}

// ReadEvent reads a single event from the decoding stream
func (c *JSONStreamCodec) ReadEvent(ctx context.Context) (events.Event, error) {
	return c.decoder.ReadEvent(ctx)
}

// EndDecoding finalizes the streaming decoding session
func (c *JSONStreamCodec) EndDecoding(ctx context.Context) error {
	return c.decoder.EndStream(ctx)
}

// GetStreamEncoder returns the underlying stream encoder
func (c *JSONStreamCodec) GetStreamEncoder() encoding.StreamEncoder {
	return c.encoder
}

// GetStreamDecoder returns the underlying stream decoder
func (c *JSONStreamCodec) GetStreamDecoder() encoding.StreamDecoder {
	return c.decoder
}

// Encode implements the Codec interface - single event encoding
func (c *JSONStreamCodec) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return c.encoder.Encode(ctx, event)
}

// EncodeMultiple implements the Codec interface - multiple event encoding
func (c *JSONStreamCodec) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	return c.encoder.EncodeMultiple(ctx, events)
}

// Decode implements the Codec interface - single event decoding
func (c *JSONStreamCodec) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return c.decoder.Decode(ctx, data)
}

// DecodeMultiple implements the Codec interface - multiple event decoding
func (c *JSONStreamCodec) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return c.decoder.DecodeMultiple(ctx, data)
}

// SupportsStreaming indicates if this codec has streaming capabilities
func (c *JSONStreamCodec) SupportsStreaming() bool {
	return true
}

// CanStream indicates if this codec supports streaming (backward compatibility)
func (c *JSONStreamCodec) CanStream() bool {
	return true
}