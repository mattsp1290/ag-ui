# Encoding System Registration Changes

## Summary

This document describes the changes made to remove panic-prone init() functions from the encoding system and replace them with explicit registration functions that provide proper error handling.

## Files Modified

### 1. `/go-sdk/pkg/encoding/json/init.go`
- Added explicit registration functions: `Register()`, `EnsureRegistered()`, `RegisterTo()`
- Modified `init()` to call `Register()` without panicking
- Added thread-safe registration with `sync.Once`
- Updated factory methods to include `context.Context` parameter

### 2. `/go-sdk/pkg/encoding/protobuf/init.go`
- Added explicit registration functions: `Register()`, `EnsureRegistered()`, `RegisterTo()`
- Modified `init()` to call `Register()` without panicking
- Added thread-safe registration with `sync.Once`
- Updated factory methods to include `context.Context` parameter

### 3. `/go-sdk/pkg/encoding/registry.go`
- Updated `RegisterDefaults()` to handle errors gracefully
- Added deprecation notice for `RegisterDefaults()`
- Updated factory method calls to include `context.Background()`
- Fixed `compositeCodec` to implement the `Codec` interface with context parameters

### 4. `/go-sdk/pkg/encoding/factories.go`
- Added context import
- Updated `CachingEncoderFactory` methods to pass context to underlying factory

## New Files Created

### 1. `/go-sdk/pkg/encoding/MIGRATION_GUIDE.md`
- Comprehensive guide for users migrating to the new registration API
- Examples of different usage patterns
- Benefits and best practices

### 2. `/go-sdk/pkg/encoding/registration_test.go`
- Tests for explicit registration functions
- Tests for custom registry usage
- Tests for idempotent registration
- Tests for error handling

### 3. `/go-sdk/pkg/encoding/registration_simple_test.go`
- Simplified tests focusing only on registration functionality

### 4. `/go-sdk/pkg/encoding/example_registration_test.go`
- Example code demonstrating the new registration API
- Custom registry examples
- Error handling examples

## Key Changes

### Before
```go
func init() {
    if err := registry.RegisterFormat(formatInfo); err != nil {
        panic("failed to register format: " + err.Error())
    }
}
```

### After
```go
func init() {
    _ = Register() // No panic, errors captured internally
}

func Register() error {
    // Thread-safe, idempotent registration
}

func EnsureRegistered() error {
    // Verify registration succeeded
}

func RegisterTo(registry *encoding.FormatRegistry) error {
    // Register to custom registry
}
```

## Benefits

1. **No More Panics**: Registration failures return errors instead of crashing
2. **Better Testing**: Custom registries enable isolated testing
3. **Explicit Control**: Applications can verify registration succeeded
4. **Backward Compatible**: Existing code continues to work
5. **Thread-Safe**: All registration functions are idempotent and thread-safe

## Backward Compatibility

The changes maintain full backward compatibility:
- `init()` functions still exist and attempt registration
- Import side effects still work as before
- The only difference is that failures don't panic

## Testing Considerations

Due to the context parameter requirements in the encoding interfaces, some tests may need to be updated to pass context. The factory methods now require context parameters as per the interface definitions.

## Future Work

Consider adding context-aware versions of the registry methods:
- `GetEncoderWithContext(ctx context.Context, mimeType string, options *EncodingOptions)`
- `GetDecoderWithContext(ctx context.Context, mimeType string, options *DecodingOptions)`

This would allow proper context propagation throughout the encoding system.