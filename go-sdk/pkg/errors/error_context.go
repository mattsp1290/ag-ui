package errors

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// ErrorContext holds contextual information about errors
type ErrorContext struct {
	// ID is a unique identifier for this error context
	ID string

	// OperationName describes the operation being performed
	OperationName string

	// StartTime is when the operation started
	StartTime time.Time

	// EndTime is when the operation ended (if completed)
	EndTime *time.Time

	// UserID identifies the user associated with the operation
	UserID string

	// RequestID identifies the request
	RequestID string

	// TraceID is for distributed tracing
	TraceID string

	// SpanID is for distributed tracing
	SpanID string

	// SourceLocation contains file, line, and function information
	SourceLocation *SourceLocation

	// Tags are key-value pairs for categorization
	Tags map[string]string

	// Metadata contains additional contextual data
	Metadata map[string]interface{}

	// Errors collected in this context
	errors []error

	// mu protects concurrent access
	mu sync.RWMutex
}

// SourceLocation represents where an error occurred in code
type SourceLocation struct {
	File     string
	Line     int
	Function string
}

// NewErrorContext creates a new error context
func NewErrorContext(operationName string) *ErrorContext {
	return &ErrorContext{
		ID:            generateContextID(),
		OperationName: operationName,
		StartTime:     time.Now(),
		Tags:          make(map[string]string),
		Metadata:      make(map[string]interface{}),
		errors:        make([]error, 0),
	}
}

// WithUserID sets the user ID
func (ec *ErrorContext) WithUserID(userID string) *ErrorContext {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.UserID = userID
	return ec
}

// WithRequestID sets the request ID
func (ec *ErrorContext) WithRequestID(requestID string) *ErrorContext {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.RequestID = requestID
	return ec
}

// WithTracing sets tracing information
func (ec *ErrorContext) WithTracing(traceID, spanID string) *ErrorContext {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.TraceID = traceID
	ec.SpanID = spanID
	return ec
}

// SetTag sets a tag value
func (ec *ErrorContext) SetTag(key, value string) *ErrorContext {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.Tags[key] = value
	return ec
}

// SetMetadata sets a metadata value
func (ec *ErrorContext) SetMetadata(key string, value interface{}) *ErrorContext {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.Metadata[key] = value
	return ec
}

// RecordError records an error in this context
func (ec *ErrorContext) RecordError(err error) *ErrorContext {
	if err == nil {
		return ec
	}

	ec.mu.Lock()
	defer ec.mu.Unlock()

	// Capture source location if not already set
	if ec.SourceLocation == nil {
		ec.SourceLocation = captureSourceLocation(2)
	}

	// Enhance error with context
	enhanced := ec.enhanceError(err)
	ec.errors = append(ec.errors, enhanced)

	return ec
}

// GetErrors returns all recorded errors
func (ec *ErrorContext) GetErrors() []error {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	result := make([]error, len(ec.errors))
	copy(result, ec.errors)
	return result
}

// HasErrors returns true if any errors have been recorded
func (ec *ErrorContext) HasErrors() bool {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return len(ec.errors) > 0
}

// ErrorCount returns the number of recorded errors
func (ec *ErrorContext) ErrorCount() int {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return len(ec.errors)
}

// Complete marks the operation as complete
func (ec *ErrorContext) Complete() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	now := time.Now()
	ec.EndTime = &now
}

// Duration returns the operation duration
func (ec *ErrorContext) Duration() time.Duration {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	if ec.EndTime != nil {
		return ec.EndTime.Sub(ec.StartTime)
	}
	return time.Since(ec.StartTime)
}

// ToContext adds this error context to a context.Context
func (ec *ErrorContext) ToContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, errorContextKey{}, ec)
}

// enhanceError adds contextual information to an error
func (ec *ErrorContext) enhanceError(err error) error {
	// If it's already one of our errors, enhance it
	switch e := err.(type) {
	case *BaseError:
		return ec.enhanceBaseError(e)
	case *StateError:
		ec.enhanceBaseError(e.BaseError)
		return e
	case *ValidationError:
		ec.enhanceBaseError(e.BaseError)
		return e
	case *ConflictError:
		ec.enhanceBaseError(e.BaseError)
		return e
	}

	// Create a new base error with context
	baseErr := &BaseError{
		Code:      "CONTEXT_ERROR",
		Message:   err.Error(),
		Severity:  SeverityError,
		Timestamp: time.Now(),
		Cause:     err,
		Details:   make(map[string]interface{}),
	}

	return ec.enhanceBaseError(baseErr)
}

// enhanceBaseError adds context to a BaseError
func (ec *ErrorContext) enhanceBaseError(err *BaseError) *BaseError {
	if err.Details == nil {
		err.Details = make(map[string]interface{})
	}

	err.Details["context_id"] = ec.ID
	err.Details["operation"] = ec.OperationName

	if ec.UserID != "" {
		err.Details["user_id"] = ec.UserID
	}
	if ec.RequestID != "" {
		err.Details["request_id"] = ec.RequestID
	}
	if ec.TraceID != "" {
		err.Details["trace_id"] = ec.TraceID
		err.Details["span_id"] = ec.SpanID
	}

	// Add source location
	if ec.SourceLocation != nil {
		err.Details["source"] = map[string]interface{}{
			"file":     ec.SourceLocation.File,
			"line":     ec.SourceLocation.Line,
			"function": ec.SourceLocation.Function,
		}
	}

	// Add tags
	if len(ec.Tags) > 0 {
		err.Details["tags"] = ec.Tags
	}

	// Add selected metadata
	for k, v := range ec.Metadata {
		if _, exists := err.Details[k]; !exists {
			err.Details[k] = v
		}
	}

	return err
}

// Summary generates a summary of the error context
func (ec *ErrorContext) Summary() string {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	summary := fmt.Sprintf("ErrorContext[%s] Operation: %s, Duration: %s, Errors: %d",
		ec.ID, ec.OperationName, ec.Duration(), len(ec.errors))

	if ec.UserID != "" {
		summary += fmt.Sprintf(", User: %s", ec.UserID)
	}
	if ec.RequestID != "" {
		summary += fmt.Sprintf(", Request: %s", ec.RequestID)
	}

	return summary
}

// errorContextKey is the context key for error contexts
type errorContextKey struct{}

// FromContext retrieves an error context from a context.Context
func FromContext(ctx context.Context) (*ErrorContext, bool) {
	ec, ok := ctx.Value(errorContextKey{}).(*ErrorContext)
	return ec, ok
}

// GetOrCreateContext gets an existing error context or creates a new one
func GetOrCreateContext(ctx context.Context, operationName string) (*ErrorContext, context.Context) {
	if ec, ok := FromContext(ctx); ok {
		return ec, ctx
	}

	ec := NewErrorContext(operationName)
	newCtx := ec.ToContext(ctx)
	return ec, newCtx
}

// captureSourceLocation captures the source location of the caller
func captureSourceLocation(skip int) *SourceLocation {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return nil
	}

	fn := runtime.FuncForPC(pc)
	var fnName string
	if fn != nil {
		fnName = fn.Name()
	}

	return &SourceLocation{
		File:     file,
		Line:     line,
		Function: fnName,
	}
}

// generateContextID generates a unique context ID
func generateContextID() string {
	return fmt.Sprintf("ec_%d_%d", time.Now().UnixNano(), runtime.NumGoroutine())
}

// ContextBuilder provides a fluent interface for building error contexts
type ContextBuilder struct {
	ec *ErrorContext
}

// NewContextBuilder creates a new context builder
func NewContextBuilder(operationName string) *ContextBuilder {
	return &ContextBuilder{
		ec: NewErrorContext(operationName),
	}
}

// WithUser sets the user ID
func (b *ContextBuilder) WithUser(userID string) *ContextBuilder {
	b.ec.WithUserID(userID)
	return b
}

// WithRequest sets the request ID
func (b *ContextBuilder) WithRequest(requestID string) *ContextBuilder {
	b.ec.WithRequestID(requestID)
	return b
}

// WithTrace sets tracing information
func (b *ContextBuilder) WithTrace(traceID, spanID string) *ContextBuilder {
	b.ec.WithTracing(traceID, spanID)
	return b
}

// WithTag adds a tag
func (b *ContextBuilder) WithTag(key, value string) *ContextBuilder {
	b.ec.SetTag(key, value)
	return b
}

// WithMetadata adds metadata
func (b *ContextBuilder) WithMetadata(key string, value interface{}) *ContextBuilder {
	b.ec.SetMetadata(key, value)
	return b
}

// Build returns the built error context
func (b *ContextBuilder) Build() *ErrorContext {
	return b.ec
}

// BuildContext returns a context.Context with the error context
func (b *ContextBuilder) BuildContext(ctx context.Context) context.Context {
	return b.ec.ToContext(ctx)
}

// RecordErrorInContext records an error in the context from context.Context
func RecordErrorInContext(ctx context.Context, err error) {
	if ec, ok := FromContext(ctx); ok {
		ec.RecordError(err)
	}
}

// GetErrorsFromContext retrieves errors from the context
func GetErrorsFromContext(ctx context.Context) []error {
	if ec, ok := FromContext(ctx); ok {
		return ec.GetErrors()
	}
	return nil
}

// HasErrorsInContext checks if the context has any errors
func HasErrorsInContext(ctx context.Context) bool {
	if ec, ok := FromContext(ctx); ok {
		return ec.HasErrors()
	}
	return false
}
