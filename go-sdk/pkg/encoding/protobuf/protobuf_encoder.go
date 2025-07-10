package protobuf

import (
	"encoding/binary"
	"fmt"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"github.com/ag-ui/go-sdk/pkg/proto/generated"
	"google.golang.org/protobuf/proto"
)

// ProtobufEncoder implements the Encoder interface for Protocol Buffer encoding
type ProtobufEncoder struct {
	options *encoding.EncodingOptions
}

// NewProtobufEncoder creates a new ProtobufEncoder with optional configuration
func NewProtobufEncoder(options *encoding.EncodingOptions) *ProtobufEncoder {
	if options == nil {
		options = &encoding.EncodingOptions{}
	}
	return &ProtobufEncoder{
		options: options,
	}
}

// Encode encodes a single event to protobuf binary format
func (e *ProtobufEncoder) Encode(event events.Event) ([]byte, error) {
	if event == nil {
		return nil, &encoding.EncodingError{
			Format:  "protobuf",
			Message: "cannot encode nil event",
		}
	}

	// Convert to protobuf representation
	pbEvent, err := event.ToProtobuf()
	if err != nil {
		return nil, &encoding.EncodingError{
			Format:  "protobuf",
			Event:   event,
			Message: "failed to convert event to protobuf",
			Cause:   err,
		}
	}

	// Marshal to binary
	data, err := proto.Marshal(pbEvent)
	if err != nil {
		return nil, &encoding.EncodingError{
			Format:  "protobuf",
			Event:   event,
			Message: "failed to marshal protobuf",
			Cause:   err,
		}
	}

	// Check size limit if configured
	if e.options.MaxSize > 0 && int64(len(data)) > e.options.MaxSize {
		return nil, &encoding.EncodingError{
			Format:  "protobuf",
			Event:   event,
			Message: fmt.Sprintf("encoded size %d exceeds maximum %d", len(data), e.options.MaxSize),
		}
	}

	// Validate output if requested
	if e.options.ValidateOutput {
		var validateEvent generated.Event
		if err := proto.Unmarshal(data, &validateEvent); err != nil {
			return nil, &encoding.EncodingError{
				Format:  "protobuf",
				Event:   event,
				Message: "output validation failed",
				Cause:   err,
			}
		}
	}

	return data, nil
}

// EncodeMultiple encodes multiple events using length-prefixed format
func (e *ProtobufEncoder) EncodeMultiple(events []events.Event) ([]byte, error) {
	if len(events) == 0 {
		return nil, &encoding.EncodingError{
			Format:  "protobuf",
			Message: "cannot encode empty event slice",
		}
	}

	// Use a simple length-prefixed format for multiple events
	// Format: [4-byte count][4-byte length][event1][4-byte length][event2]...
	
	totalSize := 4 // 4 bytes for count
	encodedEvents := make([][]byte, 0, len(events))
	
	// First pass: encode all events and calculate total size
	for i, event := range events {
		if event == nil {
			return nil, &encoding.EncodingError{
				Format:  "protobuf",
				Message: fmt.Sprintf("nil event at index %d", i),
			}
		}

		encoded, err := e.Encode(event)
		if err != nil {
			return nil, &encoding.EncodingError{
				Format:  "protobuf",
				Event:   event,
				Message: fmt.Sprintf("failed to encode event at index %d", i),
				Cause:   err,
			}
		}

		encodedEvents = append(encodedEvents, encoded)
		totalSize += 4 + len(encoded) // 4 bytes for length + event data
	}

	// Check size limit
	if e.options.MaxSize > 0 && int64(totalSize) > e.options.MaxSize {
		return nil, &encoding.EncodingError{
			Format:  "protobuf",
			Message: fmt.Sprintf("encoded batch size %d exceeds maximum %d", totalSize, e.options.MaxSize),
		}
	}

	// Second pass: build the final output
	output := make([]byte, totalSize)
	offset := 0
	
	// Write event count
	writeUint32(output[offset:], uint32(len(events)))
	offset += 4
	
	// Write each event with its length
	for _, encoded := range encodedEvents {
		writeUint32(output[offset:], uint32(len(encoded)))
		offset += 4
		copy(output[offset:], encoded)
		offset += len(encoded)
	}

	return output, nil
}

// ContentType returns the MIME type for protobuf
func (e *ProtobufEncoder) ContentType() string {
	return "application/x-protobuf"
}

// CanStream indicates that protobuf supports streaming
func (e *ProtobufEncoder) CanStream() bool {
	return true
}

// writeUint32 writes a 32-bit unsigned integer in big-endian format
func writeUint32(b []byte, v uint32) {
	binary.BigEndian.PutUint32(b, v)
}