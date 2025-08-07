package errors

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"
)

// ErrorHandler defines the interface for handling errors
type ErrorHandler interface {
	Handle(ctx context.Context, err error) error
	HandleWithSeverity(ctx context.Context, err error, severity Severity) error
}

// HandlerFunc is a function that processes an error
type HandlerFunc func(ctx context.Context, err error) error

// ErrorHandlerChain manages a chain of error handlers
type ErrorHandlerChain struct {
	handlers []HandlerFunc
	mu       sync.RWMutex
}

// NewErrorHandlerChain creates a new error handler chain
func NewErrorHandlerChain() *ErrorHandlerChain {
	return &ErrorHandlerChain{
		handlers: make([]HandlerFunc, 0),
	}
}

// AddHandler adds a handler to the chain
func (c *ErrorHandlerChain) AddHandler(handler HandlerFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers = append(c.handlers, handler)
}

// Handle processes an error through the handler chain
func (c *ErrorHandlerChain) Handle(ctx context.Context, err error) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, handler := range c.handlers {
		if handlerErr := handler(ctx, err); handlerErr != nil {
			// If a handler returns an error, stop processing
			return handlerErr
		}
	}
	return err
}

// SeverityHandler handles errors based on their severity
type SeverityHandler struct {
	handlers map[Severity]HandlerFunc
	fallback HandlerFunc
	mu       sync.RWMutex
}

// NewSeverityHandler creates a new severity-based error handler
func NewSeverityHandler() *SeverityHandler {
	return &SeverityHandler{
		handlers: make(map[Severity]HandlerFunc),
		fallback: defaultErrorHandler,
	}
}

// SetHandler sets a handler for a specific severity level
func (h *SeverityHandler) SetHandler(severity Severity, handler HandlerFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handlers[severity] = handler
}

// SetFallbackHandler sets the fallback handler for unhandled severities
func (h *SeverityHandler) SetFallbackHandler(handler HandlerFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.fallback = handler
}

// Handle processes an error based on its severity
func (h *SeverityHandler) Handle(ctx context.Context, err error) error {
	severity := GetSeverity(err)
	return h.HandleWithSeverity(ctx, err, severity)
}

// HandleWithSeverity processes an error with a specific severity
func (h *SeverityHandler) HandleWithSeverity(ctx context.Context, err error, severity Severity) error {
	h.mu.RLock()
	handler, exists := h.handlers[severity]
	if !exists {
		handler = h.fallback
	}
	h.mu.RUnlock()

	return handler(ctx, err)
}

// LoggingHandler logs errors with appropriate detail based on severity
type LoggingHandler struct {
	logger   *log.Logger
	minLevel Severity
}

// NewLoggingHandler creates a new logging error handler
func NewLoggingHandler(logger *log.Logger, minLevel Severity) *LoggingHandler {
	if logger == nil {
		logger = log.Default()
	}
	return &LoggingHandler{
		logger:   logger,
		minLevel: minLevel,
	}
}

// Handle logs the error if it meets the minimum severity level
func (h *LoggingHandler) Handle(ctx context.Context, err error) error {
	severity := GetSeverity(err)
	if severity < h.minLevel {
		return err
	}

	// Extract context information if available
	var contextInfo string
	if ctx != nil {
		if reqID := ctx.Value("request_id"); reqID != nil {
			contextInfo = fmt.Sprintf(" [request_id: %v]", reqID)
		}
	}

	// Log with appropriate detail based on severity
	switch severity {
	case SeverityDebug, SeverityInfo:
		h.logger.Printf("[%s]%s %v", severity, contextInfo, err)
	case SeverityWarning, SeverityError:
		h.logger.Printf("[%s]%s %v\nDetails: %+v", severity, contextInfo, err, err)
	case SeverityCritical, SeverityFatal:
		h.logger.Printf("[%s]%s %v\nDetails: %+v\nStack trace:\n%s",
			severity, contextInfo, err, err, string(debug.Stack()))
	}

	return err
}

// HandleWithSeverity logs the error with a specific severity
func (h *LoggingHandler) HandleWithSeverity(ctx context.Context, err error, severity Severity) error {
	if severity < h.minLevel {
		return err
	}

	// Create a wrapper error with the specified severity
	wrapped := &BaseError{
		Code:      "LOGGED",
		Message:   err.Error(),
		Severity:  severity,
		Timestamp: time.Now(),
		Cause:     err,
	}

	return h.Handle(ctx, wrapped)
}

// PanicHandler handles panics and converts them to errors
type PanicHandler struct {
	handler ErrorHandler
}

// NewPanicHandler creates a new panic handler
func NewPanicHandler(handler ErrorHandler) *PanicHandler {
	if handler == nil {
		handler = NewSeverityHandler()
	}
	return &PanicHandler{
		handler: handler,
	}
}

// HandlePanic recovers from panics and converts them to errors
func (h *PanicHandler) HandlePanic(ctx context.Context) {
	if r := recover(); r != nil {
		err := &BaseError{
			Code:      "PANIC",
			Message:   fmt.Sprintf("panic recovered: %v", r),
			Severity:  SeverityFatal,
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"panic_value": r,
				"stack_trace": string(debug.Stack()),
			},
		}
		_ = h.handler.HandleWithSeverity(ctx, err, SeverityFatal)
	}
}

// NotificationHandler sends notifications for critical errors
type NotificationHandler struct {
	notifier func(ctx context.Context, err error) error
	minLevel Severity
}

// NewNotificationHandler creates a new notification handler
func NewNotificationHandler(notifier func(context.Context, error) error, minLevel Severity) *NotificationHandler {
	return &NotificationHandler{
		notifier: notifier,
		minLevel: minLevel,
	}
}

// Handle sends a notification if the error meets the minimum severity
func (h *NotificationHandler) Handle(ctx context.Context, err error) error {
	severity := GetSeverity(err)
	if severity >= h.minLevel && h.notifier != nil {
		// Run notification in background to avoid blocking
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log panic but don't propagate
					// Since we don't have a logger in NotificationHandler, we'll just ignore the panic
					_ = fmt.Errorf("recovered panic in notification handler: %v", r)
				}
			}()
			notifyCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = h.notifier(notifyCtx, err)
		}()
	}
	return err
}

// HandleWithSeverity handles error with specific severity
func (h *NotificationHandler) HandleWithSeverity(ctx context.Context, err error, severity Severity) error {
	if severity >= h.minLevel && h.notifier != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log panic but don't propagate
					// Since we don't have a logger in NotificationHandler, we'll just ignore the panic
					_ = fmt.Errorf("recovered panic in notification handler with severity: %v", r)
				}
			}()
			notifyCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = h.notifier(notifyCtx, err)
		}()
	}
	return err
}

// MetricsHandler records error metrics
type MetricsHandler struct {
	recorder func(severity Severity, code string, count int)
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(recorder func(Severity, string, int)) *MetricsHandler {
	return &MetricsHandler{
		recorder: recorder,
	}
}

// Handle records error metrics
func (h *MetricsHandler) Handle(ctx context.Context, err error) error {
	if h.recorder == nil {
		return err
	}

	severity := GetSeverity(err)
	code := "UNKNOWN"

	// Extract error code
	switch e := err.(type) {
	case *BaseError:
		code = e.Code
	case *StateError:
		code = e.BaseError.Code
	case *ValidationError:
		code = e.BaseError.Code
	case *ConflictError:
		code = e.BaseError.Code
	}

	h.recorder(severity, code, 1)
	return err
}

// HandleWithSeverity handles error with specific severity
func (h *MetricsHandler) HandleWithSeverity(ctx context.Context, err error, severity Severity) error {
	if h.recorder != nil {
		code := "UNKNOWN"
		if baseErr, ok := err.(*BaseError); ok {
			code = baseErr.Code
		}
		h.recorder(severity, code, 1)
	}
	return err
}

// Default handlers
var (
	defaultErrorHandler HandlerFunc = func(ctx context.Context, err error) error {
		log.Printf("Error: %v", err)
		return err
	}

	debugHandler HandlerFunc = func(ctx context.Context, err error) error {
		log.Printf("DEBUG: %+v", err)
		return err
	}

	infoHandler HandlerFunc = func(ctx context.Context, err error) error {
		log.Printf("INFO: %v", err)
		return err
	}

	warningHandler HandlerFunc = func(ctx context.Context, err error) error {
		log.Printf("WARNING: %v", err)
		return err
	}

	errorHandler HandlerFunc = func(ctx context.Context, err error) error {
		log.Printf("ERROR: %+v", err)
		return err
	}

	criticalHandler HandlerFunc = func(ctx context.Context, err error) error {
		log.Printf("CRITICAL: %+v\nStack: %s", err, debug.Stack())
		return err
	}

	fatalHandler HandlerFunc = func(ctx context.Context, err error) error {
		log.Fatalf("FATAL: %+v\nStack: %s", err, debug.Stack())
		return err // Never reached
	}
)

// CreateDefaultSeverityHandler creates a severity handler with default handlers
func CreateDefaultSeverityHandler() *SeverityHandler {
	handler := NewSeverityHandler()
	handler.SetHandler(SeverityDebug, debugHandler)
	handler.SetHandler(SeverityInfo, infoHandler)
	handler.SetHandler(SeverityWarning, warningHandler)
	handler.SetHandler(SeverityError, errorHandler)
	handler.SetHandler(SeverityCritical, criticalHandler)
	handler.SetHandler(SeverityFatal, fatalHandler)
	return handler
}

// GlobalHandler is the default global error handler
var GlobalHandler = CreateDefaultSeverityHandler()

// Handle processes an error through the global handler
func Handle(ctx context.Context, err error) error {
	return GlobalHandler.Handle(ctx, err)
}

// HandleWithSeverity processes an error with a specific severity through the global handler
func HandleWithSeverity(ctx context.Context, err error, severity Severity) error {
	return GlobalHandler.HandleWithSeverity(ctx, err, severity)
}
