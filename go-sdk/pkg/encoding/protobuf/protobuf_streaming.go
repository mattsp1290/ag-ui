package protobuf

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/proto/generated"
	"google.golang.org/protobuf/proto"
)

// StreamingProtobufEncoder implements StreamEncoder for protobuf format
type StreamingProtobufEncoder struct {
	*ProtobufEncoder
	writer io.Writer
	buffer []byte
	mutex  sync.RWMutex // Protects concurrent access to writer and buffer
}

// NewStreamingProtobufEncoder creates a new streaming protobuf encoder
func NewStreamingProtobufEncoder(options *encoding.EncodingOptions) *StreamingProtobufEncoder {
	return &StreamingProtobufEncoder{
		ProtobufEncoder: NewProtobufEncoder(options),
		buffer:          make([]byte, 4), // for length prefix
	}
}

// EncodeStream encodes events from a channel to a writer
func (e *StreamingProtobufEncoder) EncodeStream(ctx context.Context, input <-chan events.Event, output io.Writer) error {
	e.mutex.Lock()
	e.writer = output
	e.mutex.Unlock()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-input:
			if !ok {
				return nil // Channel closed
			}
			if err := e.WriteEvent(ctx, event); err != nil {
				return err
			}
		}
	}
}

// StartStream initializes a streaming session
func (e *StreamingProtobufEncoder) StartStream(ctx context.Context, w io.Writer) error {
	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return &encoding.EncodingError{
			Format:  "protobuf",
			Message: "context cancelled",
			Cause:   err,
		}
	}
	
	e.mutex.Lock()
	e.writer = w
	e.mutex.Unlock()
	return nil
}

// WriteEvent writes a single event to the stream with length prefix
func (e *StreamingProtobufEncoder) WriteEvent(ctx context.Context, event events.Event) error {
	if event == nil {
		return &encoding.EncodingError{
			Format:  "protobuf",
			Message: "cannot encode nil event",
		}
	}

	// Convert to protobuf
	pbEvent, err := event.ToProtobuf()
	if err != nil {
		return &encoding.EncodingError{
			Format:  "protobuf",
			Event:   event,
			Message: "failed to convert event to protobuf",
			Cause:   err,
		}
	}

	// Marshal to binary
	data, err := proto.Marshal(pbEvent)
	if err != nil {
		return &encoding.EncodingError{
			Format:  "protobuf",
			Event:   event,
			Message: "failed to marshal protobuf",
			Cause:   err,
		}
	}

	// Use mutex to protect concurrent access to writer and buffer
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// Check if writer is initialized after acquiring lock
	if e.writer == nil {
		return errors.NewStreamingError("PROTOBUF_STREAM_NOT_INITIALIZED", "stream not initialized")
	}

	// Create a local buffer for this operation to avoid race conditions
	// This is more efficient than allocating a new buffer on each call
	lengthPrefix := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthPrefix, uint32(len(data)))
	
	// Write length prefix
	if _, err := e.writer.Write(lengthPrefix); err != nil {
		return &encoding.EncodingError{
			Format:  "protobuf",
			Event:   event,
			Message: "failed to write length prefix",
			Cause:   err,
		}
	}

	// Write event data
	if _, err := e.writer.Write(data); err != nil {
		return &encoding.EncodingError{
			Format:  "protobuf",
			Event:   event,
			Message: "failed to write event data",
			Cause:   err,
		}
	}

	return nil
}

// EndStream finalizes the streaming session
func (e *StreamingProtobufEncoder) EndStream(ctx context.Context) error {
	e.mutex.Lock()
	e.writer = nil
	e.mutex.Unlock()
	return nil
}

// StreamingProtobufDecoder implements StreamDecoder for protobuf format
type StreamingProtobufDecoder struct {
	*ProtobufDecoder
	reader io.Reader
	buffer []byte
	mutex  sync.RWMutex // Protects concurrent access to reader
}

// NewStreamingProtobufDecoder creates a new streaming protobuf decoder
func NewStreamingProtobufDecoder(options *encoding.DecodingOptions) *StreamingProtobufDecoder {
	bufferSize := 8192
	if options != nil && options.BufferSize > 0 {
		bufferSize = options.BufferSize
	}

	return &StreamingProtobufDecoder{
		ProtobufDecoder: NewProtobufDecoder(options),
		buffer:          make([]byte, bufferSize),
	}
}

// DecodeStream decodes events from a reader to a channel
func (d *StreamingProtobufDecoder) DecodeStream(ctx context.Context, input io.Reader, output chan<- events.Event) error {
	d.mutex.Lock()
	d.reader = input
	d.mutex.Unlock()

	defer close(output) // Always close the output channel when done
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			event, err := d.ReadEvent(ctx)
			if err != nil {
				if err == io.EOF {
					return nil // Normal end of stream
				}
				return err
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case output <- event:
			}
		}
	}
}

// StartStream initializes a streaming session
func (d *StreamingProtobufDecoder) StartStream(ctx context.Context, r io.Reader) error {
	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return &encoding.DecodingError{
			Format:  "protobuf",
			Message: "context cancelled",
			Cause:   err,
		}
	}
	
	d.mutex.Lock()
	d.reader = r
	d.mutex.Unlock()
	return nil
}

// ReadEvent reads a single event from the stream
func (d *StreamingProtobufDecoder) ReadEvent(ctx context.Context) (events.Event, error) {
	// Use mutex to protect concurrent access to reader
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Check if reader is initialized after acquiring lock
	if d.reader == nil {
		return nil, errors.NewStreamingError("PROTOBUF_STREAM_NOT_INITIALIZED", "stream not initialized")
	}

	// Read length prefix
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(d.reader, lengthBuf); err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, &encoding.DecodingError{
			Format:  "protobuf",
			Message: "failed to read length prefix",
			Cause:   err,
		}
	}

	length := binary.BigEndian.Uint32(lengthBuf)
	
	// Sanity check
	if length == 0 || length > 10*1024*1024 { // 10MB max per event
		return nil, &encoding.DecodingError{
			Format:  "protobuf",
			Message: fmt.Sprintf("invalid event length: %d", length),
		}
	}

	// Read event data
	eventData := make([]byte, length)
	if _, err := io.ReadFull(d.reader, eventData); err != nil {
		return nil, &encoding.DecodingError{
			Format:  "protobuf",
			Message: "failed to read event data",
			Cause:   err,
		}
	}

	// Unmarshal protobuf
	var pbEvent generated.Event
	if err := proto.Unmarshal(eventData, &pbEvent); err != nil {
		return nil, &encoding.DecodingError{
			Format:  "protobuf",
			Data:    eventData,
			Message: "failed to unmarshal protobuf",
			Cause:   err,
		}
	}

	// Convert to internal event type
	event, err := protobufToEvent(&pbEvent)
	if err != nil {
		return nil, &encoding.DecodingError{
			Format:  "protobuf",
			Data:    eventData,
			Message: "failed to convert protobuf to event",
			Cause:   err,
		}
	}

	// Validate event if requested
	if d.options != nil && d.options.ValidateEvents {
		if err := event.Validate(); err != nil {
			return nil, &encoding.DecodingError{
				Format:  "protobuf",
				Data:    eventData,
				Message: "event validation failed",
				Cause:   err,
			}
		}
	}

	return event, nil
}

// EndStream finalizes the streaming session
func (d *StreamingProtobufDecoder) EndStream(ctx context.Context) error {
	d.mutex.Lock()
	d.reader = nil
	d.mutex.Unlock()
	return nil
}