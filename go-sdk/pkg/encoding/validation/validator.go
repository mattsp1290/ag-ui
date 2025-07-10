package validation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
)

// FormatValidator defines the interface for format-specific validation
type FormatValidator interface {
	// ValidateFormat validates that data conforms to the expected format
	ValidateFormat(data []byte) error

	// ValidateEvent validates an event for format-specific requirements
	ValidateEvent(event events.Event) error

	// ValidateSchema validates data against a schema (if applicable)
	ValidateSchema(data []byte, schema interface{}) error

	// GetFormat returns the format name
	GetFormat() string
}

// JSONValidator implements FormatValidator for JSON format
type JSONValidator struct {
	strictMode bool
}

// NewJSONValidator creates a new JSON validator
func NewJSONValidator(strict bool) *JSONValidator {
	return &JSONValidator{
		strictMode: strict,
	}
}

// ValidateFormat validates JSON format
func (v *JSONValidator) ValidateFormat(data []byte) error {
	if len(data) == 0 {
		return errors.New("empty JSON data")
	}

	var js json.RawMessage
	if err := json.Unmarshal(data, &js); err != nil {
		return fmt.Errorf("invalid JSON format: %w", err)
	}

	if v.strictMode {
		// Additional strict validation
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		
		var temp interface{}
		if err := decoder.Decode(&temp); err != nil {
			return fmt.Errorf("strict JSON validation failed: %w", err)
		}
	}

	return nil
}

// ValidateEvent validates an event for JSON-specific requirements
func (v *JSONValidator) ValidateEvent(event events.Event) error {
	if event == nil {
		return errors.New("nil event")
	}

	// Check that the event can be marshaled to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("event cannot be marshaled to JSON: %w", err)
	}

	// Validate the resulting JSON
	return v.ValidateFormat(data)
}

// ValidateSchema validates JSON data against a schema
func (v *JSONValidator) ValidateSchema(data []byte, schema interface{}) error {
	// For JSON, we'll do structural validation
	// In a full implementation, this could use JSON Schema validation
	
	if schema == nil {
		return v.ValidateFormat(data)
	}

	// Basic structural validation
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return fmt.Errorf("failed to unmarshal JSON for schema validation: %w", err)
	}

	// If schema is a type, validate structure matches
	if schemaType := reflect.TypeOf(schema); schemaType != nil {
		dataType := reflect.TypeOf(jsonData)
		if !dataType.AssignableTo(schemaType) {
			return fmt.Errorf("JSON structure does not match schema type")
		}
	}

	return nil
}

// GetFormat returns the format name
func (v *JSONValidator) GetFormat() string {
	return "application/json"
}

// ProtobufValidator implements FormatValidator for Protobuf format
type ProtobufValidator struct {
	maxSize int64
}

// NewProtobufValidator creates a new Protobuf validator
func NewProtobufValidator(maxSize int64) *ProtobufValidator {
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024 // 10MB default
	}
	return &ProtobufValidator{
		maxSize: maxSize,
	}
}

// ValidateFormat validates Protobuf format
func (v *ProtobufValidator) ValidateFormat(data []byte) error {
	if len(data) == 0 {
		return errors.New("empty Protobuf data")
	}

	if int64(len(data)) > v.maxSize {
		return fmt.Errorf("Protobuf data exceeds maximum size of %d bytes", v.maxSize)
	}

	// Basic Protobuf validation
	// Check for valid varint encoding patterns
	if len(data) > 0 && data[0] == 0xFF && len(data) < 2 {
		return errors.New("invalid Protobuf varint encoding")
	}

	return nil
}

// ValidateEvent validates an event for Protobuf-specific requirements
func (v *ProtobufValidator) ValidateEvent(event events.Event) error {
	if event == nil {
		return errors.New("nil event")
	}

	// Events in this SDK should have proper field tags for Protobuf
	// This is a simplified check - real implementation would use proto reflection
	eventType := reflect.TypeOf(event)
	if eventType.Kind() == reflect.Ptr {
		eventType = eventType.Elem()
	}

	// Check for required Protobuf tags
	for i := 0; i < eventType.NumField(); i++ {
		field := eventType.Field(i)
		if tag := field.Tag.Get("protobuf"); tag == "" && field.Tag.Get("json") != "-" {
			// Warning: field without protobuf tag
			// In production, this might be logged rather than errored
		}
	}

	return nil
}

// ValidateSchema validates Protobuf data against a schema
func (v *ProtobufValidator) ValidateSchema(data []byte, schema interface{}) error {
	// For Protobuf, schema validation would involve proto descriptors
	// This is a simplified implementation
	return v.ValidateFormat(data)
}

// GetFormat returns the format name
func (v *ProtobufValidator) GetFormat() string {
	return "application/x-protobuf"
}

// RoundTripValidator performs round-trip validation
type RoundTripValidator struct {
	encoder encoding.Encoder
	decoder encoding.Decoder
}

// NewRoundTripValidator creates a new round-trip validator
func NewRoundTripValidator(encoder encoding.Encoder, decoder encoding.Decoder) *RoundTripValidator {
	return &RoundTripValidator{
		encoder: encoder,
		decoder: decoder,
	}
}

// ValidateRoundTrip validates that an event survives encode->decode->compare
func (v *RoundTripValidator) ValidateRoundTrip(ctx context.Context, event events.Event) error {
	// Encode the event
	encoded, err := v.encoder.Encode(event)
	if err != nil {
		return fmt.Errorf("round-trip encode failed: %w", err)
	}

	// Decode the event
	decoded, err := v.decoder.Decode(encoded)
	if err != nil {
		return fmt.Errorf("round-trip decode failed: %w", err)
	}

	// Compare events
	if err := compareEvents(event, decoded); err != nil {
		return fmt.Errorf("round-trip comparison failed: %w", err)
	}

	return nil
}

// ValidateRoundTripMultiple validates multiple events through round-trip
func (v *RoundTripValidator) ValidateRoundTripMultiple(ctx context.Context, events []events.Event) error {
	// Encode the events
	encoded, err := v.encoder.EncodeMultiple(events)
	if err != nil {
		return fmt.Errorf("round-trip encode multiple failed: %w", err)
	}

	// Decode the events
	decoded, err := v.decoder.DecodeMultiple(encoded)
	if err != nil {
		return fmt.Errorf("round-trip decode multiple failed: %w", err)
	}

	// Compare event counts
	if len(events) != len(decoded) {
		return fmt.Errorf("round-trip event count mismatch: expected %d, got %d", len(events), len(decoded))
	}

	// Compare each event
	for i, original := range events {
		if err := compareEvents(original, decoded[i]); err != nil {
			return fmt.Errorf("round-trip comparison failed for event %d: %w", i, err)
		}
	}

	return nil
}

// compareEvents compares two events for equality
func compareEvents(original, decoded events.Event) error {
	if original == nil && decoded == nil {
		return nil
	}
	if original == nil || decoded == nil {
		return errors.New("one event is nil")
	}

	// Compare event types
	if original.Type() != decoded.Type() {
		return fmt.Errorf("event type mismatch: %s vs %s", original.Type(), decoded.Type())
	}

	// Compare base events
	origBase := original.GetBaseEvent()
	decodedBase := decoded.GetBaseEvent()

	if origBase.EventType != decodedBase.EventType {
		return fmt.Errorf("base event type mismatch")
	}

	// Compare timestamps if present
	if origBase.TimestampMs != nil && decodedBase.TimestampMs != nil {
		if *origBase.TimestampMs != *decodedBase.TimestampMs {
			return fmt.Errorf("timestamp mismatch: %d vs %d", *origBase.TimestampMs, *decodedBase.TimestampMs)
		}
	}

	// Type-specific comparisons
	switch original.Type() {
	case events.EventTypeRunStarted:
		return compareRunStartedEvents(original.(*events.RunStartedEvent), decoded.(*events.RunStartedEvent))
	case events.EventTypeTextMessageContent:
		return compareTextMessageContentEvents(original.(*events.TextMessageContentEvent), decoded.(*events.TextMessageContentEvent))
	case events.EventTypeToolCallStart:
		return compareToolCallStartEvents(original.(*events.ToolCallStartEvent), decoded.(*events.ToolCallStartEvent))
	// Add more type-specific comparisons as needed
	}

	return nil
}

// Type-specific comparison functions
func compareRunStartedEvents(a, b *events.RunStartedEvent) error {
	if a.RunID != b.RunID {
		return fmt.Errorf("RunID mismatch: %s vs %s", a.RunID, b.RunID)
	}
	if a.ThreadID != b.ThreadID {
		return fmt.Errorf("ThreadID mismatch: %s vs %s", a.ThreadID, b.ThreadID)
	}
	return nil
}

func compareTextMessageContentEvents(a, b *events.TextMessageContentEvent) error {
	if a.MessageID != b.MessageID {
		return fmt.Errorf("MessageID mismatch: %s vs %s", a.MessageID, b.MessageID)
	}
	if a.Delta != b.Delta {
		return fmt.Errorf("Delta mismatch: %s vs %s", a.Delta, b.Delta)
	}
	return nil
}

func compareToolCallStartEvents(a, b *events.ToolCallStartEvent) error {
	if a.ToolCallID != b.ToolCallID {
		return fmt.Errorf("ToolCallID mismatch: %s vs %s", a.ToolCallID, b.ToolCallID)
	}
	if a.ToolCallName != b.ToolCallName {
		return fmt.Errorf("ToolCallName mismatch: %s vs %s", a.ToolCallName, b.ToolCallName)
	}
	return nil
}

// SchemaValidator provides schema validation capabilities
type SchemaValidator struct {
	validators map[string]FormatValidator
}

// NewSchemaValidator creates a new schema validator
func NewSchemaValidator() *SchemaValidator {
	return &SchemaValidator{
		validators: make(map[string]FormatValidator),
	}
}

// RegisterValidator registers a format validator
func (s *SchemaValidator) RegisterValidator(format string, validator FormatValidator) {
	s.validators[format] = validator
}

// ValidateWithSchema validates data against a schema for a given format
func (s *SchemaValidator) ValidateWithSchema(format string, data []byte, schema interface{}) error {
	validator, ok := s.validators[format]
	if !ok {
		return fmt.Errorf("no validator registered for format: %s", format)
	}

	return validator.ValidateSchema(data, schema)
}