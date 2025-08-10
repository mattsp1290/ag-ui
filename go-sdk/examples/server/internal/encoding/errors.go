package encoding

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// EncodingError represents errors that occur during event encoding
type EncodingError struct {
	Event     events.Event
	Operation string
	Cause     error
	RequestID string
	Context   map[string]interface{}
}

func (e *EncodingError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("encoding error during %s: %v", e.Operation, e.Cause)
	}
	return fmt.Sprintf("encoding error during %s", e.Operation)
}

func (e *EncodingError) Unwrap() error {
	return e.Cause
}

// ValidationError represents errors during event validation
type ValidationError struct {
	Event     events.Event
	Field     string
	Value     interface{}
	Message   string
	RequestID string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field %s: %s", e.Field, e.Message)
}

// NegotiationError represents content negotiation failures
type NegotiationError struct {
	AcceptHeader   string
	SupportedTypes []string
	RequestedType  string
	RequestID      string
}

func (e *NegotiationError) Error() string {
	return fmt.Sprintf("content negotiation failed for %q, supported types: %v",
		e.AcceptHeader, e.SupportedTypes)
}

// ErrorHandler provides centralized error handling for encoding operations
type ErrorHandler struct {
	logger *slog.Logger
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger *slog.Logger) *ErrorHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ErrorHandler{logger: logger}
}

// HandleEncodingError handles encoding errors with structured logging
func (h *ErrorHandler) HandleEncodingError(err *EncodingError) {
	logContext := []any{
		"error", err.Error(),
		"operation", err.Operation,
		"request_id", err.RequestID,
	}

	if err.Event != nil {
		logContext = append(logContext,
			"event_type", err.Event.Type(),
			"event_timestamp", err.Event.Timestamp(),
		)
	}

	if err.Context != nil {
		for k, v := range err.Context {
			logContext = append(logContext, k, v)
		}
	}

	h.logger.Error("Event encoding failed", logContext...)
}

// HandleValidationError handles validation errors with structured logging
func (h *ErrorHandler) HandleValidationError(err *ValidationError) {
	logContext := []any{
		"error", err.Error(),
		"field", err.Field,
		"value", err.Value,
		"message", err.Message,
		"request_id", err.RequestID,
	}

	if err.Event != nil {
		logContext = append(logContext,
			"event_type", err.Event.Type(),
			"event_timestamp", err.Event.Timestamp(),
		)
	}

	h.logger.Warn("Event validation failed", logContext...)
}

// HandleNegotiationError handles content negotiation errors
func (h *ErrorHandler) HandleNegotiationError(err *NegotiationError) {
	h.logger.Warn("Content negotiation failed",
		"error", err.Error(),
		"accept_header", err.AcceptHeader,
		"requested_type", err.RequestedType,
		"supported_types", err.SupportedTypes,
		"request_id", err.RequestID,
	)
}

// HandleSSEError handles SSE-specific errors
func (h *ErrorHandler) HandleSSEError(err error, operation string, requestID string, context map[string]interface{}) {
	logContext := []any{
		"error", err.Error(),
		"operation", operation,
		"request_id", requestID,
	}

	for k, v := range context {
		logContext = append(logContext, k, v)
	}

	// Determine log level based on error type
	if isConnectionError(err) {
		h.logger.Debug("SSE connection error (expected)", logContext...)
	} else {
		h.logger.Error("SSE operation failed", logContext...)
	}
}

// CreateEncodingError creates a new EncodingError with context
func CreateEncodingError(event events.Event, operation string, cause error, requestID string) *EncodingError {
	return &EncodingError{
		Event:     event,
		Operation: operation,
		Cause:     cause,
		RequestID: requestID,
		Context:   make(map[string]interface{}),
	}
}

// CreateValidationError creates a new ValidationError
func CreateValidationError(event events.Event, field string, value interface{}, message string, requestID string) *ValidationError {
	return &ValidationError{
		Event:     event,
		Field:     field,
		Value:     value,
		Message:   message,
		RequestID: requestID,
	}
}

// CreateNegotiationError creates a new NegotiationError
func CreateNegotiationError(acceptHeader string, supportedTypes []string, requestID string) *NegotiationError {
	return &NegotiationError{
		AcceptHeader:   acceptHeader,
		SupportedTypes: supportedTypes,
		RequestID:      requestID,
	}
}

// HandleEncodingErrorResponse creates a proper error response for encoding failures
func HandleEncodingErrorResponse(c fiber.Ctx, err *EncodingError) error {
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error":      "Encoding Error",
		"message":    "Failed to encode event for streaming",
		"operation":  err.Operation,
		"request_id": err.RequestID,
		"details":    err.Cause.Error(),
	})
}

// HandleValidationErrorResponse creates a proper error response for validation failures
func HandleValidationErrorResponse(c fiber.Ctx, err *ValidationError) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error":      "Validation Error",
		"message":    err.Message,
		"field":      err.Field,
		"value":      err.Value,
		"request_id": err.RequestID,
	})
}

// HandleNegotiationErrorResponse creates a proper error response for negotiation failures
func HandleNegotiationErrorResponse(c fiber.Ctx, err *NegotiationError) error {
	return c.Status(fiber.StatusNotAcceptable).JSON(fiber.Map{
		"error":           "Content Negotiation Failed",
		"message":         "The requested content type is not supported",
		"accept_header":   err.AcceptHeader,
		"supported_types": err.SupportedTypes,
		"request_id":      err.RequestID,
	})
}

// isConnectionError checks if an error is a connection-related error (expected during SSE)
func isConnectionError(err error) bool {
	errStr := err.Error()
	connectionErrors := []string{
		"connection closed",
		"broken pipe",
		"client disconnected",
		"context cancelled",
		"use of closed network connection",
	}

	for _, connErr := range connectionErrors {
		if strings.Contains(strings.ToLower(errStr), connErr) {
			return true
		}
	}

	return false
}

// SafeEventValidation validates an event and returns detailed error information
func SafeEventValidation(event events.Event, requestID string) error {
	if event == nil {
		return CreateValidationError(nil, "event", nil, "event cannot be nil", requestID)
	}

	// Check event type
	if event.Type() == "" {
		return CreateValidationError(event, "type", "", "event type cannot be empty", requestID)
	}

	// Validate using the event's own validation
	if err := event.Validate(); err != nil {
		return CreateValidationError(event, "content", nil, err.Error(), requestID)
	}

	return nil
}

// RecoverFromPanic recovers from panics during encoding operations
func RecoverFromPanic(requestID string, logger *slog.Logger) {
	if r := recover(); r != nil {
		logger.Error("Panic recovered during encoding operation",
			"panic", r,
			"request_id", requestID,
		)
	}
}
