package encoding

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// Tests for uncovered SSEWriter functions

func TestSSEWriter_WriteEventWithNegotiation(t *testing.T) {
	writer := NewSSEWriter()
	var buf bytes.Buffer
	bufWriter := bufio.NewWriter(&buf)
	ctx := context.Background()

	testEvent := &CustomEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventTypeCustom,
		},
	}
	testEvent.SetData(map[string]interface{}{"test": "negotiation"})
	testEvent.SetTimestamp(1234567890)

	err := writer.WriteEventWithNegotiation(ctx, bufWriter, testEvent, "application/json")
	if err != nil {
		t.Errorf("WriteEventWithNegotiation() error = %v", err)
		return
	}

	if err := bufWriter.Flush(); err != nil {
		t.Errorf("Failed to flush buffer: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "data: {") {
		t.Errorf("Expected JSON data in output, got: %s", output)
	}
}

func TestCustomEvent_DataMethods(t *testing.T) {
	event := &CustomEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventTypeCustom,
		},
	}

	// Test initial state
	data := event.Data()
	if data != nil {
		t.Errorf("Expected nil data initially, got: %v", data)
	}

	// Test SetData
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}
	event.SetData(testData)

	retrievedData := event.Data()
	if retrievedData == nil {
		t.Fatal("Expected non-nil data after SetData")
	}

	if retrievedData["key1"] != "value1" || retrievedData["key2"] != 42 {
		t.Errorf("Data not set correctly: %v", retrievedData)
	}

	// Test SetDataField
	event.SetDataField("key3", "value3")
	updatedData := event.Data()
	if updatedData["key3"] != "value3" {
		t.Errorf("SetDataField failed, got: %v", updatedData)
	}

	// Test ThreadID and RunID
	if event.ThreadID() != "" {
		t.Errorf("Expected empty ThreadID, got: %q", event.ThreadID())
	}
	if event.RunID() != "" {
		t.Errorf("Expected empty RunID, got: %q", event.RunID())
	}
}

func TestCustomEvent_ToProtobuf(t *testing.T) {
	event := &CustomEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventTypeCustom,
		},
	}
	event.SetData(map[string]interface{}{"test": "protobuf"})

	_, err := event.ToProtobuf()
	if err == nil {
		t.Error("Expected error for unimplemented protobuf encoding")
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("Expected 'not implemented' error, got: %v", err)
	}
}

func TestCustomEvent_ValidationFailure(t *testing.T) {
	event := &CustomEvent{
		BaseEvent: events.BaseEvent{
			EventType: "", // Empty type should cause validation failure
		},
	}

	err := event.Validate()
	if err == nil {
		t.Error("Expected validation error for empty event type")
	}
}

// Tests for error handling functions

func TestErrorTypes(t *testing.T) {
	// Test EncodingError
	baseErr := errors.New("base error")
	encErr := EncodingError{
		Operation: "test_op",
		Cause:     baseErr,
	}

	if !strings.Contains(encErr.Error(), "test_op") {
		t.Errorf("Unexpected encoding error message: %s", encErr.Error())
	}

	if encErr.Unwrap() != baseErr {
		t.Errorf("Unwrap() should return base error")
	}

	// Test ValidationError
	valErr := ValidationError{Message: "validation failed"}
	if !strings.Contains(valErr.Error(), "validation failed") {
		t.Errorf("Unexpected validation error message: %s", valErr.Error())
	}

	// Test NegotiationError
	negErr := NegotiationError{AcceptHeader: "test"}
	if negErr.Error() == "" {
		t.Errorf("Expected non-empty negotiation error message")
	}
}

func TestErrorHandler_MissingLogger(t *testing.T) {
	handler := NewErrorHandler(nil)
	if handler == nil {
		t.Fatal("Expected non-nil handler")
	}

	// Test that it doesn't panic with nil logger initially
	testErr := CreateEncodingError(nil, "test_op", errors.New("test error"), "req123")
	handler.HandleEncodingError(testErr)
}

func TestErrorHandlerAllTypes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	handler := NewErrorHandler(logger)

	// Test validation error
	valErr := CreateValidationError(nil, "test_field", "test_value", "validation message", "req123")
	handler.HandleValidationError(valErr)

	// Test negotiation error  
	negErr := CreateNegotiationError("application/xml", []string{"application/json"}, "req456")
	handler.HandleNegotiationError(negErr)
}

func TestSSEErrorHandling(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	handler := NewErrorHandler(logger)

	// Test SSE error handling
	err := errors.New("write failed")
	handler.HandleSSEError(err, "test_operation", "test123", map[string]interface{}{"key": "value"})

	// Test connection error detection
	connectionErrors := []error{
		errors.New("connection closed"),
		errors.New("broken pipe"),
		errors.New("some other error"),
	}

	for _, err := range connectionErrors {
		handler.HandleSSEError(err, "test_operation", "test123", map[string]interface{}{})
	}
}

func TestValidEventFlow(t *testing.T) {
	// Test that a valid event doesn't trigger validation errors
	validEvent := &CustomEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventTypeCustom,
		},
	}
	validEvent.SetData(map[string]interface{}{"test": "data"})

	err := validEvent.Validate()
	if err != nil {
		t.Errorf("Valid event should pass validation, got: %v", err)
	}

	// Test with invalid event
	invalidEvent := &InvalidEventForCoverage{
		BaseEvent: events.BaseEvent{
			EventType: events.EventTypeCustom,
		},
	}

	err = invalidEvent.Validate()
	if err == nil {
		t.Error("Invalid event should fail validation")
	}
}

func TestEncodingAPIEdgeCases(t *testing.T) {
	// Test error creation functions
	encErr := CreateEncodingError(nil, "test_op", errors.New("test"), "req123")
	if encErr.Operation != "test_op" {
		t.Errorf("Expected operation 'test_op', got %s", encErr.Operation)
	}

	valErr := CreateValidationError(nil, "field", "value", "message", "req123")
	if valErr.Field != "field" {
		t.Errorf("Expected field 'field', got %s", valErr.Field)
	}

	negErr := CreateNegotiationError("accept", []string{"json"}, "req123")
	if negErr.AcceptHeader != "accept" {
		t.Errorf("Expected AcceptHeader 'accept', got %s", negErr.AcceptHeader)
	}
}

// InvalidEvent for testing validation failures (reused from encoder_test.go)
type InvalidEventForCoverage struct {
	events.BaseEvent
}

func (e *InvalidEventForCoverage) ThreadID() string { return "" }
func (e *InvalidEventForCoverage) RunID() string    { return "" }
func (e *InvalidEventForCoverage) Validate() error {
	return fmt.Errorf("this event is always invalid")
}

func TestSSEWriterErrorPaths(t *testing.T) {
	writer := NewSSEWriter()
	var buf bytes.Buffer
	bufWriter := bufio.NewWriter(&buf)
	ctx := context.Background()

	// Test WriteEventWithType error path with invalid event
	invalidEvent := &InvalidEventForCoverage{
		BaseEvent: events.BaseEvent{
			EventType: events.EventTypeCustom,
		},
	}

	err := writer.WriteEventWithType(ctx, bufWriter, invalidEvent, "test")
	if err == nil {
		t.Error("Expected error for invalid event in WriteEventWithType")
	}

	// Test WriteEvent with nil event
	err = writer.WriteEvent(ctx, bufWriter, nil)
	if err == nil {
		t.Error("Expected error for nil event in WriteEvent")
	}
}