package errors_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// Example_basicErrorTypes demonstrates the use of custom error types
func Example_basicErrorTypes() {
	// Create a state error
	stateErr := errors.NewStateError("STATE_INVALID", "Invalid state transition").
		WithStateID("user-123").
		WithTransition("active -> deleted").
		WithStates("active", "suspended")

	fmt.Printf("State Error: %v\n", stateErr)

	// Create a validation error
	validationErr := errors.NewValidationError("VALIDATION_FAILED", "Input validation failed").
		WithField("email", "user@invalid").
		WithRule("email_format").
		AddFieldError("email", "Invalid email format").
		AddFieldError("age", "Age must be positive")

	fmt.Printf("Validation Error: %v\n", validationErr)

	// Create a conflict error
	conflictErr := errors.NewConflictError("RESOURCE_CONFLICT", "Resource already exists").
		WithResource("user", "user-123").
		WithOperation("create").
		WithResolution("Use update instead of create")

	fmt.Printf("Conflict Error: %v\n", conflictErr)
}

// Example_severityHandling demonstrates severity-based error handling
func Example_severityHandling() {
	// Create a severity handler
	handler := errors.NewSeverityHandler()

	// Set custom handlers for different severities
	handler.SetHandler(errors.SeverityWarning, func(ctx context.Context, err error) error {
		log.Printf("Warning logged: %v", err)
		return err
	})

	handler.SetHandler(errors.SeverityCritical, func(ctx context.Context, err error) error {
		log.Printf("CRITICAL ERROR: %v - Sending alert!", err)
		// In real application, send alerts here
		return err
	})

	// Create errors with different severities
	warningErr := errors.NewValidationError("FIELD_WARNING", "Optional field missing")
	warningErr.BaseError.Severity = errors.SeverityWarning

	criticalErr := errors.NewStateError("STATE_CORRUPTED", "Database state corrupted")
	criticalErr.BaseError.Severity = errors.SeverityCritical

	// Handle errors
	ctx := context.Background()
	_ = handler.Handle(ctx, warningErr)
	_ = handler.Handle(ctx, criticalErr)
}

// Example_errorContext demonstrates error context usage
func Example_errorContext() {
	// Create an error context for an operation
	ctx := errors.NewContextBuilder("ProcessUserRegistration").
		WithUser("user-123").
		WithRequest("req-456").
		WithTrace("trace-789", "span-012").
		WithTag("environment", "production").
		WithTag("version", "1.2.3").
		WithMetadata("ip_address", "192.168.1.1").
		BuildContext(context.Background())

	// Simulate operation with error recording
	if ec, ok := errors.FromContext(ctx); ok {
		// Record various errors during operation
		ec.RecordError(fmt.Errorf("database connection failed"))
		ec.RecordError(errors.NewValidationError("EMAIL_INVALID", "Invalid email format"))
		ec.RecordError(errors.NewStateError("USER_EXISTS", "User already exists"))

		// Complete the operation
		ec.Complete()

		// Get summary
		fmt.Printf("Operation Summary: %s\n", ec.Summary())
		fmt.Printf("Total Errors: %d\n", ec.ErrorCount())

		// Process collected errors
		for i, err := range ec.GetErrors() {
			fmt.Printf("Error %d: %v\n", i+1, err)
		}
	}
}

// Example_retryLogic demonstrates retry functionality
func Example_retryLogic() {
	// Configure retry behavior
	config := &errors.RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
		RetryIf: func(err error) bool {
			// Only retry if the error is marked as retryable
			return errors.IsRetryable(err)
		},
		OnRetry: func(attempt int, err error, delay time.Duration) {
			fmt.Printf("Retry attempt %d after %v due to: %v\n", attempt, delay, err)
		},
	}

	// Simulate a retryable operation
	attemptCount := 0
	err := errors.Retry(context.Background(), config, func() error {
		attemptCount++
		if attemptCount < 3 {
			// Simulate transient failures
			return errors.NewBaseError("TEMP_FAILURE", "Temporary failure").
				WithRetry(time.Second)
		}
		// Success on third attempt
		return nil
	})

	if err != nil {
		fmt.Printf("Operation failed after retries: %v\n", err)
	} else {
		fmt.Printf("Operation succeeded after %d attempts\n", attemptCount)
	}
}

// Example_errorChaining demonstrates error chaining and wrapping
func Example_errorChaining() {
	// Create a chain of errors
	err1 := errors.NewValidationError("FIELD_MISSING", "Required field missing")
	err2 := errors.NewValidationError("FIELD_INVALID", "Invalid field format")
	err3 := errors.NewStateError("STATE_INVALID", "Cannot process in current state")

	chainedErr := errors.Chain(err1, err2, err3)
	fmt.Printf("Chained Error: %v\n", chainedErr)

	// Wrap errors with context
	dbErr := fmt.Errorf("connection refused")
	wrappedErr := errors.Wrap(dbErr, "failed to connect to database")
	wrappedErr2 := errors.Wrapf(wrappedErr, "during user creation for ID %s", "user-123")

	fmt.Printf("Wrapped Error: %v\n", wrappedErr2)

	// Get root cause
	cause := errors.Cause(wrappedErr2)
	fmt.Printf("Root Cause: %v\n", cause)
}

// Example_errorCollector demonstrates error collection
func Example_errorCollector() {
	// Create an error collector for batch operations
	collector := errors.NewErrorCollector()

	// Simulate batch processing
	users := []string{"user1", "user2", "user3", "user4"}
	for _, user := range users {
		if user == "user2" || user == "user4" {
			// Simulate validation failures
			err := errors.NewValidationError("USER_INVALID", "Invalid user data").
				WithField("username", user)
			collector.AddWithContext(err, fmt.Sprintf("processing user %s", user))
		}
	}

	// Check results
	if collector.HasErrors() {
		fmt.Printf("Batch processing completed with %d errors\n", collector.Count())

		// Get all errors
		for i, err := range collector.Errors() {
			fmt.Printf("Error %d: %v\n", i+1, err)
		}

		// Get combined error
		combinedErr := collector.Error()
		fmt.Printf("Combined Error: %v\n", combinedErr)
	}
}

// Example_errorMatching demonstrates error pattern matching
func Example_errorMatching() {
	// Create various errors
	errorList := []error{
		errors.NewStateError("STATE_INVALID", "Invalid state").
			WithStateID("state-1"),
		errors.NewValidationError("FIELD_REQUIRED", "Required field missing"),
		errors.NewConflictError("DUPLICATE_KEY", "Duplicate key error").
			WithResource("user", "user-123"),
		errors.NewBaseError("CRITICAL_ERROR", "System critical error").
			WithDetail("severity", errors.SeverityCritical),
	}

	// Create matchers
	validationMatcher := errors.NewErrorMatcher().
		WithCode("FIELD_REQUIRED").
		WithSeverity(errors.SeverityWarning)

	criticalMatcher := errors.NewErrorMatcher().
		WithMessage("critical").
		WithSeverity(errors.SeverityError)

	// Match errors
	for i, err := range errorList {
		if validationMatcher.Matches(err) {
			fmt.Printf("Error %d matches validation pattern\n", i+1)
		}
		if criticalMatcher.Matches(err) {
			fmt.Printf("Error %d matches critical pattern\n", i+1)
		}
	}
}

// Example_errorHandlerChain demonstrates chained error handlers
func Example_errorHandlerChain() {
	// Create a chain of error handlers
	chain := errors.NewErrorHandlerChain()

	// Add logging handler
	chain.AddHandler(func(ctx context.Context, err error) error {
		log.Printf("Error occurred: %v", err)
		return nil // Continue to next handler
	})

	// Add metric recording handler
	chain.AddHandler(func(ctx context.Context, err error) error {
		severity := errors.GetSeverity(err)
		fmt.Printf("Recording metric for %s error\n", severity)
		return nil
	})

	// Add notification handler for critical errors
	chain.AddHandler(func(ctx context.Context, err error) error {
		if errors.GetSeverity(err) >= errors.SeverityCritical {
			fmt.Println("Sending critical error notification!")
		}
		return nil
	})

	// Process an error through the chain
	criticalErr := errors.NewStateError("SYSTEM_FAILURE", "System component failed")
	criticalErr.BaseError.Severity = errors.SeverityCritical

	ctx := context.Background()
	_ = chain.Handle(ctx, criticalErr)
}

// Example_panicHandling demonstrates panic recovery
func Example_panicHandling() {
	// Create a panic handler
	handler := errors.NewPanicHandler(errors.CreateDefaultSeverityHandler())

	// Function that might panic
	riskyOperation := func(ctx context.Context) {
		defer handler.HandlePanic(ctx)

		// Simulate a panic
		panic("unexpected condition")
	}

	// Create context with error tracking
	ec := errors.NewErrorContext("RiskyOperation")
	ctx := ec.ToContext(context.Background())

	// Execute risky operation (panic will be handled)
	func() {
		defer func() {
			if r := recover(); r == nil {
				// Panic was handled by PanicHandler
				fmt.Println("Operation completed (panic was handled)")
			}
		}()
		riskyOperation(ctx)
	}()
}

// Example_comprehensiveErrorHandling shows a complete error handling setup
func Example_comprehensiveErrorHandling() {
	// Setup comprehensive error handling for an application

	// 1. Create main error handler with severity handling (not used in this example)
	// mainHandler := errors.CreateDefaultSeverityHandler()

	// 2. Create specialized handlers
	loggingHandler := errors.NewLoggingHandler(log.Default(), errors.SeverityInfo)

	notificationHandler := errors.NewNotificationHandler(
		func(ctx context.Context, err error) error {
			fmt.Printf("ALERT: Critical error: %v\n", err)
			return nil
		},
		errors.SeverityCritical,
	)

	metricsHandler := errors.NewMetricsHandler(
		func(severity errors.Severity, code string, count int) {
			fmt.Printf("Metric: %s errors with code %s: %d\n", severity, code, count)
		},
	)

	// 3. Create handler chain
	chain := errors.NewErrorHandlerChain()
	chain.AddHandler(loggingHandler.Handle)
	chain.AddHandler(notificationHandler.Handle)
	chain.AddHandler(metricsHandler.Handle)

	// 4. Simulate application errors
	ctx := context.Background()

	// Normal error
	normalErr := errors.NewValidationError("INPUT_INVALID", "Invalid input")
	_ = chain.Handle(ctx, normalErr)

	// Critical error
	criticalErr := errors.NewStateError("DB_CORRUPTED", "Database corruption detected")
	criticalErr.BaseError.Severity = errors.SeverityCritical
	_ = chain.Handle(ctx, criticalErr)

	fmt.Println("\nError handling demonstration complete")
}
