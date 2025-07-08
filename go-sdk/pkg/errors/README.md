# Error Handling Package

The `errors` package provides comprehensive error handling utilities for the ag-ui Go SDK, offering rich error types, contextual information, severity-based handling, and advanced retry mechanisms.

## Features

- **Custom Error Types**: Specialized error types for different scenarios (StateError, ValidationError, ConflictError)
- **Severity-Based Handling**: Route and handle errors based on their severity level
- **Error Context**: Rich contextual information including operation tracking, user/request IDs, and distributed tracing
- **Retry Logic**: Sophisticated retry mechanisms with exponential backoff and jitter
- **Error Collection**: Collect and manage multiple errors from batch operations
- **Pattern Matching**: Match errors based on various criteria
- **Handler Chains**: Compose multiple error handlers for comprehensive error processing

## Quick Start

```go
import "github.com/ag-ui/go-sdk/pkg/errors"

// Create a validation error
err := errors.NewValidationError("INVALID_EMAIL", "Email validation failed").
    WithField("email", userEmail).
    AddFieldError("email", "Invalid format")

// Handle with severity
errors.HandleWithSeverity(ctx, err, errors.SeverityWarning)

// Retry an operation
err = errors.RetryWithBackoff(ctx, func() error {
    return someFlakeyOperation()
})
```

## Error Types

### BaseError

The foundation for all custom errors:

```go
err := errors.NewBaseError("ERROR_CODE", "Error message").
    WithDetail("key", "value").
    WithCause(originalError).
    WithRetry(5 * time.Second)
```

### StateError

For state-related errors:

```go
err := errors.NewStateError("INVALID_TRANSITION", "Cannot transition").
    WithStateID("state-123").
    WithTransition("pending -> active").
    WithStates(currentState, expectedState)
```

### ValidationError

For validation failures:

```go
err := errors.NewValidationError("VALIDATION_FAILED", "Input validation failed").
    WithField("email", value).
    WithRule("email_format").
    AddFieldError("email", "Invalid format").
    AddFieldError("age", "Must be positive")
```

### ConflictError

For resource conflicts:

```go
err := errors.NewConflictError("RESOURCE_EXISTS", "Resource already exists").
    WithResource("user", "user-123").
    WithOperation("create").
    WithResolution("Use update instead")
```

## Severity Levels

Errors can be classified by severity:

- `SeverityDebug`: Debug information
- `SeverityInfo`: Informational messages
- `SeverityWarning`: Warnings that don't prevent operation
- `SeverityError`: Recoverable errors
- `SeverityCritical`: Critical errors requiring attention
- `SeverityFatal`: Fatal errors requiring termination

## Error Context

Track operations with rich contextual information:

```go
// Create context for an operation
ec := errors.NewContextBuilder("UserRegistration").
    WithUser(userID).
    WithRequest(requestID).
    WithTrace(traceID, spanID).
    WithTag("environment", "production").
    WithMetadata("ip_address", clientIP).
    Build()

ctx := ec.ToContext(context.Background())

// Record errors during operation
errors.RecordErrorInContext(ctx, err)

// Complete operation and get summary
ec.Complete()
fmt.Printf("Operation: %s\n", ec.Summary())
```

## Retry Logic

Implement sophisticated retry strategies:

```go
config := &errors.RetryConfig{
    MaxAttempts:  5,
    InitialDelay: 100 * time.Millisecond,
    MaxDelay:     30 * time.Second,
    Multiplier:   2.0,
    Jitter:       0.1,
    RetryIf: func(err error) bool {
        return errors.IsRetryable(err)
    },
    OnRetry: func(attempt int, err error, delay time.Duration) {
        log.Printf("Retry %d after %v: %v", attempt, delay, err)
    },
}

err := errors.Retry(ctx, config, operation)
```

## Error Handlers

### Severity Handler

Route errors based on severity:

```go
handler := errors.NewSeverityHandler()
handler.SetHandler(errors.SeverityWarning, warningHandler)
handler.SetHandler(errors.SeverityCritical, criticalHandler)
```

### Logging Handler

Log errors with appropriate detail:

```go
handler := errors.NewLoggingHandler(logger, errors.SeverityInfo)
```

### Notification Handler

Send alerts for critical errors:

```go
handler := errors.NewNotificationHandler(
    func(ctx context.Context, err error) error {
        // Send alert
        return sendAlert(err)
    },
    errors.SeverityCritical,
)
```

### Handler Chain

Compose multiple handlers:

```go
chain := errors.NewErrorHandlerChain()
chain.AddHandler(loggingHandler.Handle)
chain.AddHandler(metricsHandler.Handle)
chain.AddHandler(notificationHandler.Handle)
```

## Error Collection

Manage multiple errors from batch operations:

```go
collector := errors.NewErrorCollector()

for _, item := range items {
    if err := process(item); err != nil {
        collector.AddWithContext(err, fmt.Sprintf("processing %s", item.ID))
    }
}

if collector.HasErrors() {
    log.Printf("Batch failed with %d errors", collector.Count())
    return collector.Error()
}
```

## Pattern Matching

Match errors based on criteria:

```go
matcher := errors.NewErrorMatcher().
    WithCode("VALIDATION_FAILED").
    WithSeverity(errors.SeverityWarning).
    WithMessage("email")

if matcher.Matches(err) {
    // Handle validation error
}
```

## Utility Functions

### Error Wrapping

```go
err := errors.Wrap(originalErr, "additional context")
err = errors.Wrapf(err, "operation failed for user %s", userID)
```

### Error Inspection

```go
if errors.IsRetryable(err) {
    // Retry the operation
}

severity := errors.GetSeverity(err)
retryAfter := errors.GetRetryAfter(err)
```

### Error Chaining

```go
chainedErr := errors.Chain(err1, err2, err3)
```

## Best Practices

1. **Use specific error types** for better error handling and debugging
2. **Set appropriate severity levels** to enable proper routing and alerting
3. **Include contextual information** to aid in debugging and monitoring
4. **Mark errors as retryable** when appropriate
5. **Use error collectors** for batch operations
6. **Implement proper error handlers** based on your application needs
7. **Leverage error context** for operation tracking

## Examples

See the `examples_test.go` file for comprehensive examples demonstrating all features of the error handling package.