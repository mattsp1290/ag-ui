# Standardized Error Handling Guide

This document outlines the standardized error handling approach implemented across the AG-UI Go SDK codebase.

## Overview

The codebase now follows a consistent error handling strategy that ensures:
- **Error chain preservation** using Go 1.13+ error wrapping
- **Meaningful error context** in all error messages
- **Consistent error patterns** across all modules
- **Proper debugging information** for troubleshooting

## Standardized Patterns

### 1. Error Wrapping with fmt.Errorf

**Use `fmt.Errorf` with `%w` verb** for wrapping errors to maintain error chains:

```go
// ✅ GOOD: Use fmt.Errorf with %w for error wrapping
if err := someOperation(); err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// ✅ GOOD: Add meaningful context
if err := validateUser(userID); err != nil {
    return fmt.Errorf("user validation failed for ID %s: %w", userID, err)
}

// ❌ AVOID: Using errors.Wrap (except in documentation/examples)
return errors.Wrap(err, "operation failed")

// ❌ AVOID: Losing error context
return err
```

### 2. Error Context and Messages

**Include relevant context** in error messages to aid debugging:

```go
// ✅ GOOD: Include component and operation context
return fmt.Errorf("websocket performance manager: rate limit exceeded for message batching")

// ✅ GOOD: Include resource identifiers
return fmt.Errorf("state update failed for context '%s': %w", contextID, err)

// ✅ GOOD: Include method context for unimplemented features
return fmt.Errorf("PerfJSONSerializer.Deserialize: method not yet implemented")

// ❌ AVOID: Generic error messages without context
return errors.New("rate limit exceeded")
return errors.New("not implemented")
```

### 3. Custom Error Types

**Preserve custom error types** where they provide structured information:

```go
// ✅ GOOD: Use rich custom error types for complex scenarios
func NewValidationError(code, message string) *ValidationError {
    return &ValidationError{
        BaseError: &BaseError{
            Code:      code,
            Message:   message,
            Severity:  SeverityWarning,
            Timestamp: time.Now(),
        },
    }
}

// ✅ GOOD: Chain custom errors with standard wrapping
validationErr := NewValidationError("FIELD_INVALID", "field validation failed")
return fmt.Errorf("user registration failed: %w", validationErr)
```

### 4. Error Chain Inspection

**Use errors.Is() and errors.As()** for error inspection:

```go
// ✅ GOOD: Check for specific error types
if errors.Is(err, ErrValidationFailed) {
    // Handle validation error
}

// ✅ GOOD: Extract custom error types
var validationErr *ValidationError
if errors.As(err, &validationErr) {
    log.Printf("Validation failed with code: %s", validationErr.Code)
}
```

## Implementation Status

### ✅ Completed Modules

- **pkg/errors**: Core error types and utilities
- **pkg/encoding**: All encoding/validation files updated
- **pkg/transport**: Factory and transport implementations
- **pkg/state**: State management error handling
- **pkg/tools**: Tool execution error handling
- **pkg/client**: Client configuration and operations

### 🔧 Changes Made

1. **Replaced inconsistent `errors.Wrap` calls** with `fmt.Errorf` pattern:
   - `/pkg/encoding/validation/security.go`: 5 instances updated
   - `/pkg/encoding/validation/compatibility.go`: 1 instance updated

2. **Enhanced error messages** with better context:
   - WebSocket performance manager errors
   - Method not implemented errors
   - Component-specific error messages

3. **Preserved error chains** throughout the codebase:
   - All error wrapping maintains `%w` pattern
   - Custom error types support proper unwrapping
   - Error inspection works correctly with `errors.Is()`/`errors.As()`

## Error Handling Best Practices

### 1. Error Creation

```go
// Simple errors with context
return fmt.Errorf("failed to connect to database: %w", err)

// Component-specific errors
return fmt.Errorf("websocket client: connection timeout after %v", timeout)

// Operation-specific errors with resource IDs
return fmt.Errorf("user service: failed to delete user %s: %w", userID, err)
```

### 2. Error Propagation

```go
func processUser(userID string) error {
    user, err := fetchUser(userID)
    if err != nil {
        // Add context while preserving the chain
        return fmt.Errorf("user processing failed for ID %s: %w", userID, err)
    }
    
    if err := validateUser(user); err != nil {
        // Include operation context
        return fmt.Errorf("user validation failed: %w", err)
    }
    
    return nil
}
```

### 3. Error Handling

```go
func handleError(err error) {
    // Check for specific error types
    if errors.Is(err, context.DeadlineExceeded) {
        log.Warn("Operation timed out, retrying...")
        return
    }
    
    // Extract custom error information
    var validationErr *ValidationError
    if errors.As(err, &validationErr) {
        log.Errorf("Validation failed: %s (code: %s)", 
            validationErr.Message, validationErr.Code)
        return
    }
    
    // Log the full error chain
    log.Errorf("Unexpected error: %v", err)
}
```

## Testing

**Comprehensive test suite** in `/pkg/errors/standardization_test.go` verifies:

- Error chain preservation across modules
- Consistent error wrapping patterns
- Proper error context inclusion
- Error unwrapping behavior
- Concurrent error handling safety

Run tests with:
```bash
go test ./pkg/errors -v -run "Test.*"
```

## Migration Guide

### For New Code

1. **Always use `fmt.Errorf` with `%w`** for error wrapping
2. **Include meaningful context** in error messages
3. **Use custom error types** for structured error information
4. **Test error handling** with `errors.Is()` and `errors.As()`

### For Existing Code

1. **Replace `errors.Wrap` calls** with `fmt.Errorf` pattern:
   ```go
   // Before
   return errors.Wrap(err, "operation failed")
   
   // After
   return fmt.Errorf("operation failed: %w", err)
   ```

2. **Enhance simple error messages** with context:
   ```go
   // Before
   return errors.New("not implemented")
   
   // After
   return fmt.Errorf("MethodName: method not yet implemented")
   ```

3. **Preserve error chains** in all error handling:
   ```go
   // Before
   if err != nil {
       return errors.New("operation failed")
   }
   
   // After
   if err != nil {
       return fmt.Errorf("operation failed: %w", err)
   }
   ```

## Benefits

1. **Improved Debugging**: Consistent error messages with proper context make debugging easier
2. **Better Error Inspection**: Standard error wrapping enables proper use of `errors.Is()` and `errors.As()`
3. **Consistent User Experience**: Standardized error patterns provide consistent behavior across modules
4. **Maintainability**: Clear error handling patterns make code easier to understand and maintain
5. **Compatibility**: Full compatibility with Go's standard error handling mechanisms

## Verification

The standardization has been verified through:

- ✅ **Code Analysis**: All error handling patterns reviewed and updated
- ✅ **Automated Testing**: Comprehensive test suite ensures compliance
- ✅ **Error Chain Testing**: Verification that all error chains are preserved
- ✅ **Context Preservation**: Testing that error context is maintained throughout the codebase
- ✅ **Concurrent Safety**: Tests ensure error handling is thread-safe

For questions or clarifications about error handling patterns, refer to the examples in `/pkg/errors/` or consult this documentation.