package protobuf

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/proto/generated"
	"google.golang.org/protobuf/proto"
)

// StreamingProtobufEncoder implements StreamEncoder for protobuf format
type StreamingProtobufEncoder struct {
	*ProtobufEncoder
	writer io.Writer
	buffer []byte
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
	e.writer = output

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-input:
			if !ok {
				return nil // Channel closed
			}
			if err := e.WriteEvent(event); err != nil {
				return err
			}
		}
	}
}

// StartStream initializes a streaming session
func (e *StreamingProtobufEncoder) StartStream(w io.Writer) error {
	e.writer = w
	return nil
}

// WriteEvent writes a single event to the stream with length prefix
func (e *StreamingProtobufEncoder) WriteEvent(event events.Event) error {
	if e.writer == nil {
		return fmt.Errorf("stream not initialized")
	}

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

	// Write length prefix
	binary.BigEndian.PutUint32(e.buffer, uint32(len(data)))
	if _, err := e.writer.Write(e.buffer); err != nil {
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
func (e *StreamingProtobufEncoder) EndStream() error {
	e.writer = nil
	return nil
}

// StreamingProtobufDecoder implements StreamDecoder for protobuf format
type StreamingProtobufDecoder struct {
	*ProtobufDecoder
	reader io.Reader
	buffer []byte
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
	d.reader = input

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			event, err := d.ReadEvent()
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
func (d *StreamingProtobufDecoder) StartStream(r io.Reader) error {
	d.reader = r
	return nil
}

// ReadEvent reads a single event from the stream
func (d *StreamingProtobufDecoder) ReadEvent() (events.Event, error) {
	if d.reader == nil {
		return nil, fmt.Errorf("stream not initialized")
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
func (d *StreamingProtobufDecoder) EndStream() error {
	d.reader = nil
	return nil
}