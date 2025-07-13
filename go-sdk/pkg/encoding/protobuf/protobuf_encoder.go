package protobuf

import (
	"context"
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
func (e *ProtobufEncoder) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return nil, &encoding.EncodingError{
			Format:  "protobuf",
			Message: "context cancelled",
			Cause:   err,
		}
	}

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

	// Marshal to binary using buffer pooling with optimized size
	optimalSize := encoding.GetOptimalBufferSizeForEvent(event)
	buf := encoding.GetBuffer(optimalSize / 2) // Protobuf is typically more compact than JSON
	defer encoding.PutBuffer(buf)
	
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
		// Use buffer pooling for validation operations
		validationBuf := encoding.GetBuffer(len(data))
		defer encoding.PutBuffer(validationBuf)
		
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
func (e *ProtobufEncoder) EncodeMultiple(ctx context.Context, events []events.Event) ([]byte, error) {
	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return nil, &encoding.EncodingError{
			Format:  "protobuf",
			Message: "context cancelled",
			Cause:   err,
		}
	}

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
	
	// Use buffer pooling for better memory efficiency with optimized sizing
	estimatedSize := encoding.GetOptimalBufferSizeForMultiple(events) / 2 // Protobuf is more compact
	workingBuf := encoding.GetBuffer(estimatedSize)
	defer encoding.PutBuffer(workingBuf)
	
	// First pass: encode all events and calculate total size
	for i, event := range events {
		if event == nil {
			return nil, &encoding.EncodingError{
				Format:  "protobuf",
				Message: fmt.Sprintf("nil event at index %d", i),
			}
		}

		encoded, err := e.Encode(ctx, event)
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

	// Second pass: build the final output using buffer pooling
	outputBuf := encoding.GetBuffer(totalSize)
	defer encoding.PutBuffer(outputBuf)
	
	// Write event count
	countBytes := make([]byte, 4)
	writeUint32(countBytes, uint32(len(events)))
	outputBuf.Write(countBytes)
	
	// Write each event with its length
	for _, encoded := range encodedEvents {
		lengthBytes := make([]byte, 4)
		writeUint32(lengthBytes, uint32(len(encoded)))
		outputBuf.Write(lengthBytes)
		outputBuf.Write(encoded)
	}

	// Copy final result
	output := make([]byte, outputBuf.Len())
	copy(output, outputBuf.Bytes())
	
	return output, nil
}

// ContentType returns the MIME type for protobuf
func (e *ProtobufEncoder) ContentType() string {
	return "application/x-protobuf"
}

// CanStream indicates that protobuf supports streaming (backward compatibility)
func (e *ProtobufEncoder) CanStream() bool {
	return true
}

// SupportsStreaming indicates that protobuf encoder supports streaming
func (e *ProtobufEncoder) SupportsStreaming() bool {
	return true
}

// writeUint32 writes a 32-bit unsigned integer in big-endian format
func writeUint32(b []byte, v uint32) {
	binary.BigEndian.PutUint32(b, v)
}

// Reset resets the encoder with new options (for pooling)
func (e *ProtobufEncoder) Reset(options *encoding.EncodingOptions) {
	if options == nil {
		options = &encoding.EncodingOptions{}
	}
	e.options = options
}