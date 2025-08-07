// Package errors provides comprehensive error handling utilities for the ag-ui Go SDK.
//
// This package offers a rich set of error handling features including:
//   - Custom error types (StateError, ValidationError, ConflictError)
//   - Severity-based error handling
//   - Error context management
//   - Retry logic with exponential backoff
//   - Error chaining and collection
//   - Pattern matching for errors
//
// # Error Types
//
// The package defines several specialized error types:
//
//   - BaseError: Foundation for all custom errors with severity, timestamps, and metadata
//   - StateError: For state-related errors with transition information
//   - ValidationError: For validation failures with field-level details
//   - ConflictError: For resource conflicts with resolution strategies
//
// # Severity Levels
//
// Errors can be classified by severity:
//   - SeverityDebug: Debug information
//   - SeverityInfo: Informational messages
//   - SeverityWarning: Warnings that don't prevent operation
//   - SeverityError: Recoverable errors
//   - SeverityCritical: Critical errors requiring attention
//   - SeverityFatal: Fatal errors requiring termination
//
// # Error Context
//
// ErrorContext provides rich contextual information:
//   - Operation tracking with timing
//   - User and request identification
//   - Distributed tracing support
//   - Source location capture
//   - Tags and metadata
//
// # Retry Logic
//
// The package includes sophisticated retry capabilities:
//   - Configurable retry policies
//   - Exponential backoff with jitter
//   - Conditional retry based on error type
//   - Context-aware cancellation
//
// # Basic Usage
//
//	// Create a validation error
//	err := errors.NewValidationError("INVALID_EMAIL", "Email validation failed").
//	    WithField("email", userEmail).
//	    AddFieldError("email", "Invalid format")
//
//	// Handle with severity
//	errors.HandleWithSeverity(ctx, err, errors.SeverityWarning)
//
//	// Retry an operation
//	err = errors.RetryWithBackoff(ctx, func() error {
//	    return someFlakeyOperation()
//	})
//
// # Advanced Usage
//
//	// Create error context for operation tracking
//	ec := errors.NewContextBuilder("UserRegistration").
//	    WithUser(userID).
//	    WithRequest(requestID).
//	    Build()
//	ctx := ec.ToContext(context.Background())
//
//	// Use error collector for batch operations
//	collector := errors.NewErrorCollector()
//	for _, item := range items {
//	    if err := process(item); err != nil {
//	        collector.AddWithContext(err, fmt.Sprintf("processing %s", item.ID))
//	    }
//	}
//	if collector.HasErrors() {
//	    return collector.Error()
//	}
//
// # Error Handlers
//
// The package provides various error handlers:
//   - SeverityHandler: Routes errors based on severity
//   - LoggingHandler: Logs errors with appropriate detail
//   - NotificationHandler: Sends alerts for critical errors
//   - MetricsHandler: Records error metrics
//   - PanicHandler: Recovers from panics
//
// # Best Practices
//
// 1. Use specific error types for better error handling:
//
//	stateErr := errors.NewStateError("INVALID_TRANSITION", "Cannot transition").
//	    WithStateID(stateID).
//	    WithTransition("pending -> active")
//
// 2. Always set appropriate severity levels:
//
//	err.BaseError.Severity = errors.SeverityCritical
//
// 3. Use error context for operation tracking:
//
//	ec, ctx := errors.GetOrCreateContext(ctx, "OperationName")
//	defer ec.Complete()
//
// 4. Implement retry for transient failures:
//
//	config := errors.DefaultRetryConfig()
//	config.RetryIf = func(err error) bool {
//	    return errors.IsRetryable(err) || isNetworkError(err)
//	}
//	err := errors.Retry(ctx, config, operation)
//
// 5. Chain multiple error handlers:
//
//	chain := errors.NewErrorHandlerChain()
//	chain.AddHandler(logHandler)
//	chain.AddHandler(metricsHandler)
//	chain.AddHandler(alertHandler)
package errors
