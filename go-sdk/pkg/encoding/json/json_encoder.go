package json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
)

// JSONEncoder implements the Encoder interface for JSON format
type JSONEncoder struct {
	options *encoding.EncodingOptions
	mu      sync.Mutex
}

// NewJSONEncoder creates a new JSON encoder with the given options
func NewJSONEncoder(options *encoding.EncodingOptions) *JSONEncoder {
	if options == nil {
		options = &encoding.EncodingOptions{
			CrossSDKCompatibility: true,
			ValidateOutput:        true,
		}
	}
	return &JSONEncoder{
		options: options,
	}
}

// Encode encodes a single event to JSON
func (e *JSONEncoder) Encode(event events.Event) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if event == nil {
		return nil, &encoding.EncodingError{
			Format:  "json",
			Message: "cannot encode nil event",
		}
	}

	// Validate the event before encoding if requested
	if e.options.ValidateOutput {
		if err := event.Validate(); err != nil {
			return nil, &encoding.EncodingError{
				Format:  "json",
				Event:   event,
				Message: "event validation failed",
				Cause:   err,
			}
		}
	}

	// Use the event's ToJSON method for cross-SDK compatibility
	if e.options.CrossSDKCompatibility {
		data, err := event.ToJSON()
		if err != nil {
			return nil, &encoding.EncodingError{
				Format:  "json",
				Event:   event,
				Message: "failed to encode event",
				Cause:   err,
			}
		}

		// Pretty print if requested
		if e.options.Pretty {
			var buf bytes.Buffer
			if err := json.Indent(&buf, data, "", "  "); err != nil {
				return nil, &encoding.EncodingError{
					Format:  "json",
					Event:   event,
					Message: "failed to format JSON",
					Cause:   err,
				}
			}
			data = buf.Bytes()
		}

		// Check size limits
		if e.options.MaxSize > 0 && int64(len(data)) > e.options.MaxSize {
			return nil, &encoding.EncodingError{
				Format:  "json",
				Event:   event,
				Message: fmt.Sprintf("encoded event exceeds max size of %d bytes", e.options.MaxSize),
			}
		}

		return data, nil
	}

	// Standard JSON encoding
	var data []byte
	var err error

	if e.options.Pretty {
		data, err = json.MarshalIndent(event, "", "  ")
	} else {
		data, err = json.Marshal(event)
	}

	if err != nil {
		return nil, &encoding.EncodingError{
			Format:  "json",
			Event:   event,
			Message: "failed to marshal event",
			Cause:   err,
		}
	}

	// Check size limits
	if e.options.MaxSize > 0 && int64(len(data)) > e.options.MaxSize {
		return nil, &encoding.EncodingError{
			Format:  "json",
			Event:   event,
			Message: fmt.Sprintf("encoded event exceeds max size of %d bytes", e.options.MaxSize),
		}
	}

	return data, nil
}

// EncodeMultiple encodes multiple events efficiently
func (e *JSONEncoder) EncodeMultiple(events []events.Event) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(events) == 0 {
		return []byte("[]"), nil
	}

	// Validate all events first if requested
	if e.options.ValidateOutput {
		for i, event := range events {
			if event == nil {
				return nil, &encoding.EncodingError{
					Format:  "json",
					Message: fmt.Sprintf("cannot encode nil event at index %d", i),
				}
			}
			if err := event.Validate(); err != nil {
				return nil, &encoding.EncodingError{
					Format:  "json",
					Event:   event,
					Message: fmt.Sprintf("event validation failed at index %d", i),
					Cause:   err,
				}
			}
		}
	}

	// Create a slice to hold all encoded events
	encodedEvents := make([]json.RawMessage, 0, len(events))
	totalSize := int64(2) // Account for "[]"

	for i, event := range events {
		// Use ToJSON for cross-SDK compatibility
		var data []byte
		var err error

		if e.options.CrossSDKCompatibility {
			data, err = event.ToJSON()
		} else {
			data, err = json.Marshal(event)
		}

		if err != nil {
			return nil, &encoding.EncodingError{
				Format:  "json",
				Event:   event,
				Message: fmt.Sprintf("failed to encode event at index %d", i),
				Cause:   err,
			}
		}

		// Check cumulative size
		totalSize += int64(len(data))
		if i > 0 {
			totalSize++ // Account for comma separator
		}

		if e.options.MaxSize > 0 && totalSize > e.options.MaxSize {
			return nil, &encoding.EncodingError{
				Format:  "json",
				Message: fmt.Sprintf("encoded events exceed max size of %d bytes", e.options.MaxSize),
			}
		}

		encodedEvents = append(encodedEvents, json.RawMessage(data))
	}

	// Marshal the array of raw messages
	var result []byte
	var err error

	if e.options.Pretty {
		result, err = json.MarshalIndent(encodedEvents, "", "  ")
	} else {
		result, err = json.Marshal(encodedEvents)
	}

	if err != nil {
		return nil, &encoding.EncodingError{
			Format:  "json",
			Message: "failed to marshal event array",
			Cause:   err,
		}
	}

	return result, nil
}

// ContentType returns the MIME type for JSON
func (e *JSONEncoder) ContentType() string {
	return "application/json"
}

// CanStream indicates that JSON encoder supports streaming
func (e *JSONEncoder) CanStream() bool {
	return true
}