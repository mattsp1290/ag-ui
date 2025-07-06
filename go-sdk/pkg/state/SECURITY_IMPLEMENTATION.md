# Security Implementation for AG-UI State Management

## Overview

This document describes the comprehensive security limits and input validation implemented to prevent DoS attacks in the AG-UI Go SDK state management system.

## Implemented Security Features

### 1. Security Limits

The following security limits are enforced as constants in `security.go`:

```go
const (
    MaxPatchSize    = 1 << 20  // 1MB - Maximum size for JSON patches
    MaxStateSize    = 10 << 20 // 10MB - Maximum size for complete state
    MaxJSONDepth    = 10       // 10 levels - Maximum nesting depth
    MaxStringLength = 1 << 16  // 64KB - Maximum string length
    MaxArrayLength  = 10000    // 10000 items - Maximum array length
)
```

### 2. Input Validation

#### SecurityValidator (`security.go`)

The `SecurityValidator` provides comprehensive validation for:

- **JSON Patches**: Validates patch size, operation types, and paths
- **State Objects**: Validates complete state size and structure
- **Metadata**: Validates metadata size and content
- **JSON Depth**: Prevents stack overflow from deeply nested structures
- **Path Security**: Blocks access to forbidden paths (e.g., `/admin`, `/config`)
- **Content Security**: Detects and blocks potentially malicious content

Key validation methods:
- `ValidatePatch(patch JSONPatch)`: Validates JSON patches before applying
- `ValidateState(state interface{})`: Validates complete state objects
- `ValidateJSONDepth(data interface{})`: Checks JSON nesting depth
- `ValidateJSONPointer(pointer string)`: Validates JSON Pointer paths

### 3. Rate Limiting

#### ClientRateLimiter (`client_ratelimit.go`)

Implements per-client rate limiting using the `golang.org/x/time/rate` package:

- **Per-client limits**: Each client (identified by context ID) has independent rate limits
- **Configurable rates**: Default 100 operations/second with burst of 200
- **Automatic cleanup**: Removes inactive clients after 30 minutes
- **Memory protection**: Limits to 10,000 tracked clients

Configuration:
```go
type ClientRateLimiterConfig struct {
    RatePerSecond   int           // Operations per second
    BurstSize       int           // Burst capacity
    MaxClients      int           // Maximum tracked clients
    ClientTTL       time.Duration // Client inactivity timeout
    CleanupInterval time.Duration // Cleanup frequency
}
```

### 4. Integration in StateManager

The `StateManager` (`manager.go`) integrates all security features:

1. **Dual Rate Limiting**: 
   - Global rate limiter for overall system protection
   - Per-client rate limiter to prevent individual client abuse

2. **Multi-stage Validation**:
   - Validates input updates before processing
   - Validates computed deltas before applying
   - Validates resulting state after changes

3. **Error Handling**: Returns specific error types for different violations

### 5. Error Types

Defined error constants in `errors.go`:

```go
var (
    // Security errors
    ErrPatchTooLarge    = errors.New("patch size exceeds maximum allowed size")
    ErrStateTooLarge    = errors.New("state size exceeds maximum allowed size")
    ErrJSONTooDeep      = errors.New("JSON structure exceeds maximum allowed depth")
    ErrPathTooLong      = errors.New("JSON pointer path exceeds maximum allowed length")
    ErrValueTooLarge    = errors.New("value size exceeds maximum allowed size")
    ErrStringTooLong    = errors.New("string length exceeds maximum allowed length")
    ErrArrayTooLong     = errors.New("array length exceeds maximum allowed length")
    ErrTooManyKeys      = errors.New("object has too many keys")
    ErrInvalidOperation = errors.New("invalid patch operation")
    ErrForbiddenPath    = errors.New("access to path is forbidden")
    
    // Rate limiting errors
    ErrRateLimited      = errors.New("rate limit exceeded")
    ErrTooManyContexts  = errors.New("too many active contexts")
)
```

## Usage Example

```go
// Create state manager with security enabled by default
sm, err := NewStateManager(DefaultManagerOptions())

// Security validation happens automatically
_, err = sm.UpdateState(ctx, contextID, stateID, updates, opts)
if err != nil {
    switch err {
    case ErrRateLimited:
        // Handle rate limiting
    case ErrPatchTooLarge:
        // Handle oversized patch
    case ErrJSONTooDeep:
        // Handle deeply nested JSON
    default:
        // Handle other errors
    }
}
```

## Testing

Comprehensive tests are provided in:

- `security_limits_test.go`: Tests all security limits
- `ratelimit_test.go`: Tests rate limiting functionality
- `security_demo_test.go`: Demonstrates all features working together

## Performance Considerations

1. **Efficient Validation**: Uses streaming validation where possible
2. **Lazy Cleanup**: Rate limiter cleanup runs periodically, not on every request
3. **Concurrent Safe**: All operations are thread-safe with minimal locking
4. **Memory Bounded**: All caches and trackers have size limits

## Security Best Practices

1. Always use the default security configuration unless you have specific requirements
2. Monitor rate limiting metrics to tune limits for your use case
3. Log security violations for audit and threat detection
4. Regularly review and update forbidden paths based on your security model
5. Consider implementing additional application-specific validation rules