# Recursion Depth Limits Implementation

This document summarizes the implementation of recursion depth limits to prevent stack overflow attacks in the AG-UI Go SDK validation system.

## Overview

The review identified that the codebase had no recursion depth limits, which could lead to stack overflow attacks. This implementation adds comprehensive depth tracking and limits to all recursive validation functions.

## Changes Made

### 1. Security Validation (pkg/encoding/validation/security.go)

#### Constants Added
```go
const (
    DefaultMaxValidationDepth = 100     // Default maximum depth for validation operations
    DefaultMaxSanitizationDepth = 50    // Default maximum depth for sanitization operations  
    StrictMaxValidationDepth = 50       // Strict maximum depth for security-critical operations
    StrictMaxSanitizationDepth = 25     // Strict maximum depth for security-critical sanitization
)
```

#### SecurityConfig Updated
- Added `MaxValidationDepth int` - Maximum recursion depth for validation operations
- Added `MaxSanitizationDepth int` - Maximum recursion depth for sanitization operations

#### Functions Enhanced

**validateNestingDepth**
- Replaced with `validateNestingDepthWithLimit` that includes recursion depth tracking
- Checks both recursion depth (to prevent stack overflow) and data nesting depth (existing logic)
- Returns specific error types for depth limit violations

**sanitizeValue**
- Replaced with `sanitizeValueWithDepth` that includes depth tracking
- Safely handles depth limit exceeded by returning unsanitized value rather than crashing
- Prevents stack overflow attacks while maintaining system stability

### 2. Schema Validation (pkg/tools/schema.go)

#### Constants Added
```go
const (
    DefaultMaxSchemaValidationDepth = 100  // Default maximum depth for schema validation
    StrictMaxSchemaValidationDepth = 50    // Strict maximum depth for security-critical schema validation
)
```

#### SchemaValidator Updated
- Added `maxValidationDepth int` field to track maximum allowed recursion depth
- Updated `ValidatorOptions` to include `MaxValidationDepth int`
- All constructors now properly initialize depth limits

#### Functions Enhanced

All recursive validation functions now have depth-tracking variants:

**Core Functions:**
- `validateValue` → `validateValueWithDepth`
- `validateObject` → `validateObjectWithDepth`
- `validateArray` → `validateArrayWithDepth`
- `validateObjectProperty` → `validateObjectPropertyWithDepth`

**Composition Functions:**
- `validateOneOf` → `validateOneOfWithDepth`
- `validateAnyOf` → `validateAnyOfWithDepth`
- `validateAllOf` → `validateAllOfWithDepth`
- `validateNot` → `validateNotWithDepth`
- `validateConditional` → `validateConditionalWithDepth`

Each function now:
1. Checks recursion depth limit at entry
2. Returns `RECURSION_DEPTH_EXCEEDED` error if limit exceeded
3. Passes `depth+1` to nested calls

### 3. Configuration Updates

#### Default Configuration
- `DefaultSecurityConfig()` includes proper depth limits
- `StrictSecurityConfig()` includes stricter depth limits

#### Backward Compatibility
- All existing APIs continue to work unchanged
- Depth tracking starts at 0 for public entry points
- Default limits are generous enough for legitimate use cases

## Error Handling

### Security Validation Errors
```go
// Example error for recursion depth exceeded
agerrors.NewSecurityError(agerrors.CodeDepthExceeded, 
    fmt.Sprintf("recursion depth %d exceeds maximum %d", depth, maxDepth)).
    WithViolationType("recursion_depth_limit").
    WithDetail("depth", depth).
    WithDetail("max_depth", maxDepth)
```

### Schema Validation Errors
```go
// Example error for schema validation depth exceeded
newValidationErrorWithCode(path, 
    fmt.Sprintf("validation recursion depth %d exceeds maximum %d", depth, maxDepth), 
    "RECURSION_DEPTH_EXCEEDED")
```

## Testing

### Test Files Created

**pkg/encoding/validation/recursion_depth_test.go**
- Tests for `validateNestingDepthWithLimit`
- Tests for `sanitizeValueWithDepth`
- Tests for security configuration depth limits
- Benchmark tests for performance verification
- Tests for error message content and structure

**pkg/tools/schema_recursion_depth_test.go**
- Tests for schema validation depth limits
- Tests for composition schema depth limits
- Tests for array validation depth limits
- Tests for conditional schema depth limits
- Tests for legitimate deep nesting scenarios
- Benchmark tests for schema validation performance

### Test Coverage

Tests verify:
1. **Attack Prevention**: Deep nesting beyond limits is properly rejected
2. **Legitimate Use Cases**: Reasonable deep nesting still works
3. **Error Quality**: Error messages contain useful information
4. **Performance**: Depth checking doesn't significantly impact performance
5. **Configuration**: Default and custom depth limits work correctly

## Security Benefits

### Stack Overflow Prevention
- Prevents recursive validation functions from causing stack overflow
- Protects against malicious deeply nested JSON/data structures
- Maintains system stability under attack conditions

### Configurable Limits
- Default limits allow legitimate deep nesting (100 levels)
- Strict limits for security-critical applications (50 levels)
- Administrators can customize limits based on their needs

### Graceful Handling
- Validation functions return errors instead of crashing
- Sanitization functions return unsanitized data instead of crashing
- System remains operational even under attack

## Depth Limit Rationale

### Default Limits (100/50)
- Generous enough for legitimate deeply nested configurations
- Well below typical stack limits (thousands of stack frames)
- Provides safety margin for complex data structures

### Strict Limits (50/25)
- Suitable for security-critical applications
- Reduces attack surface significantly
- Still allows reasonable nesting for most use cases

### Configurable
- Applications can adjust based on their specific needs
- Can be tuned based on available memory and security requirements

## Implementation Notes

### Backward Compatibility
- All existing public APIs remain unchanged
- Depth tracking is internal implementation detail
- Default limits are generous to avoid breaking existing code

### Performance Impact
- Minimal overhead (simple integer comparison per recursion level)
- No memory allocation for depth tracking
- Benchmark tests verify negligible performance impact

### Error Handling
- Specific error codes for different types of depth violations
- Detailed error messages with depth information
- Security errors include violation type and risk assessment

## Future Enhancements

### Potential Improvements
1. **Dynamic Limits**: Adjust limits based on available stack space
2. **Metrics**: Track depth limit violations for security monitoring
3. **Content-Aware Limits**: Different limits for different data types
4. **Recovery**: Attempt to validate with reduced nesting on limit exceeded

### Integration
- Consider integrating with security monitoring systems
- Add metrics collection for depth limit violations
- Enhance logging for security incident analysis

## Verification Checklist

- [x] All recursive validation functions have depth limits
- [x] Proper error handling for depth limit violations
- [x] Configuration options for customizing limits
- [x] Comprehensive test coverage
- [x] Backward compatibility maintained
- [x] Performance impact minimized
- [x] Documentation provided
- [x] Default limits are reasonable for legitimate use cases

## Conclusion

This implementation successfully addresses the security vulnerability identified in the review by adding comprehensive recursion depth limits to all validation operations. The solution prevents stack overflow attacks while maintaining functionality for legitimate use cases and provides configurable security levels for different deployment scenarios.