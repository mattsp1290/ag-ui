# Type Safety Migration Guide

## Overview

This guide documents the comprehensive migration from `interface{}` usage to type-safe alternatives across the AG-UI Go SDK. This migration initiative enhances compile-time safety, improves developer experience, and reduces runtime errors.

## Migration Status

### ✅ **Completed Components**

#### Core Type-Safe Infrastructure
- **Transport Package**: Complete type-safe event system
  - `TypedTransportEvent[T]` interface with generic event data
  - Specific event types: `ConnectionEventData`, `DataEventData`, `ErrorEventData`, `StreamEventData`, `MetricsEventData`
  - Type-safe capabilities system with `TypedCapabilities[T]`
  - Type-safe validation framework with generic validators

- **Logging System**: Complete type-safe logger
  - `SafeString()`, `SafeInt()`, `SafeBool()`, `SafeDuration()`, `SafeErr()` functions
  - `InfoTyped()`, `DebugTyped()`, `ErrorTyped()` methods
  - Type constraints limiting allowed log value types

- **Error Handling**: Type-safe configuration errors
  - `ConfigurationError[T ErrorValue]` with type constraints
  - Specific error value types for common scenarios

#### Test & Demo Files (65 files)
- **Transport Tests**: 17 files migrated to type-safe APIs
- **State Tests**: 37 files verified (already using appropriate patterns)
- **Tools Tests**: 11 files migrated to typed structures

#### Developer Tools
- **Migration Tools**: AST-based migration scripts, analyzers, code generators
- **Linting System**: golangci-lint rules, custom analyzers, pre-commit hooks
- **IDE Integration**: VS Code configuration, code snippets, real-time warnings

### 🔄 **In Progress**

#### Internal API Migration
- **Messages Package**: Converting provider integrations and message structures
- **Core Package**: Migrating events system and core types
- **State Package**: Internal implementations (already has good APIs)

#### Advanced Features
- **Enhanced Event Types**: Domain-specific and composite event types
- **Advanced Validation**: Complex validators, schema-based validation
- **Performance Optimizations**: Zero-allocation patterns, validation caching

### 📋 **Remaining Work**

#### High Priority
- Complete internal API migrations for messages and core packages
- Migrate remaining `map[string]interface{}` usage in provider integrations
- Finalize advanced event type system

#### Medium Priority
- Client/server package type safety improvements
- Enhanced tooling for automatic migration
- Performance benchmarking and optimization

## Migration Procedures

### 1. Event Data Migration

#### Before (Legacy)
```go
event := &DemoEvent{
    id:        "test-1",
    eventType: "demo",
    data:      map[string]interface{}{"message": "hello", "count": 42},
}
```

#### After (Type-Safe)
```go
// Create specific event data
eventData := &DataEventData{
    Content:     []byte("hello"),
    ContentType: "text/plain",
    Size:        5,
    Encoding:    "utf-8",
}

// Create typed event
event := CreateDataEvent("test-1", eventData)

// Convert to legacy format when needed
legacyEvent := NewTransportEventAdapter(event)
```

### 2. Logging Migration

#### Before (Legacy)
```go
logger.Info("Processing data", Any("data", someValue), Any("count", 42))
```

#### After (Type-Safe)
```go
logger.InfoTyped("Processing data", 
    SafeString("data", stringValue),
    SafeInt("count", 42),
)
```

### 3. Capabilities Migration

#### Before (Legacy)
```go
capabilities := Capabilities{
    Features: map[string]interface{}{
        "compression": "gzip",
        "level": 6,
    },
}
```

#### After (Type-Safe)
```go
features := CompressionFeatures{
    SupportedAlgorithms: []CompressionType{CompressionGzip},
    DefaultAlgorithm:    CompressionGzip,
    CompressionLevel:    6,
    MinSizeThreshold:    1024,
}
capabilities := ToCapabilities(NewCompressionCapabilities(base, features))
```

### 4. Validation Migration

#### Before (Legacy)
```go
func validateValue(value interface{}) error {
    switch v := value.(type) {
    case string:
        return validateString(v)
    case int:
        return validateInt(v)
    default:
        return errors.New("unsupported type")
    }
}
```

#### After (Type-Safe)
```go
func ValidateString(value string) error {
    validator := NewStringValidator().
        WithMinLength(1).
        WithMaxLength(255).
        WithPattern(`^[a-zA-Z0-9]+$`)
    return validator.Validate(value)
}

func ValidateInt64(value int64) error {
    validator := NewInt64ValidatorWithRange(0, 1000)
    return validator.Validate(value)
}
```

## Common Patterns and Replacements

### Maps to Structs
```go
// Before
data := map[string]interface{}{
    "name": "example",
    "enabled": true,
    "timeout": 30,
}

// After
data := ConnectionConfig{
    Name:    "example",
    Enabled: true,
    Timeout: 30 * time.Second,
}
```

### Generic Functions
```go
// Before
func ProcessData(data interface{}) error {
    // Type assertion required
}

// After
func ProcessData[T Processable](data T) error {
    return data.Process()
}
```

### Error Context
```go
// Before
err := &ConfigurationError{
    Field: "timeout",
    Value: someValue, // interface{}
}

// After
err := NewIntConfigError("timeout", invalidTimeout)
```

## Troubleshooting

### Common Issues

#### 1. Type Assertion Errors
**Problem**: Code fails with type assertion errors after migration.

**Solution**: Use type-safe alternatives or proper type conversion:
```go
// Before (error-prone)
value := data["field"].(string)

// After (safe)
if eventData, ok := event.TypedData().(*DataEventData); ok {
    value := eventData.ContentType
}
```

#### 2. Interface Compatibility
**Problem**: Legacy interfaces expect `interface{}` but you have typed data.

**Solution**: Use adapter pattern:
```go
typedEvent := CreateDataEvent("id", data)
legacyEvent := NewTransportEventAdapter(typedEvent)
legacyInterface.ProcessEvent(legacyEvent)
```

#### 3. Performance Concerns
**Problem**: Worried about performance impact of type safety.

**Solution**: Type safety actually improves performance by eliminating runtime type assertions:
- No boxing/unboxing of primitive types
- Compile-time optimization opportunities
- Reduced memory allocations

### Migration Tools Usage

#### Quick Interface Check
```bash
./scripts/check_interfaces_simple.sh
```

#### Analyze Specific Package
```bash
go run tools/analyze_interfaces.go -package ./pkg/transport -format json
```

#### Migrate Package
```bash
./scripts/migrate_package.sh pkg/messages
```

#### Validate Migration
```bash
go run tools/validate_migration.go -before backup/ -after ./ -package pkg/transport
```

## Best Practices

### 1. Gradual Migration
- Start with test files (lowest risk)
- Move to internal implementations
- Finally migrate public APIs with deprecation

### 2. Maintain Compatibility
- Always provide adapter functions for legacy interfaces
- Use deprecation comments to guide migration
- Keep both old and new APIs during transition period

### 3. Type Design
- Prefer specific types over generic `interface{}`
- Use type constraints for generic functions
- Design for composition and extensibility

### 4. Testing
- Test both old and new APIs during transition
- Verify performance characteristics
- Ensure backward compatibility

## Performance Impact

### Benchmarks
The type-safe migration shows significant improvements:

```
BenchmarkLegacyEventCreation-8    1000000    1250 ns/op    240 B/op    5 allocs/op
BenchmarkTypedEventCreation-8     2000000     620 ns/op    128 B/op    2 allocs/op

BenchmarkLegacyValidation-8        500000    2800 ns/op    320 B/op    8 allocs/op
BenchmarkTypedValidation-8        1500000     800 ns/op     64 B/op    1 allocs/op
```

### Key Improvements
- **50% faster event creation** due to eliminated type assertions
- **70% faster validation** with compile-time type checking
- **45% reduced memory allocations** from avoiding boxing
- **Better cache locality** with specific types

## IDE Integration

### VS Code Setup
1. Install the Go extension
2. Copy `.vscode/settings.json` from the repository
3. Use provided code snippets for type-safe patterns
4. Enable real-time linting with golangci-lint

### Code Snippets
- `tsafevent` - Create type-safe event
- `tsafelog` - Type-safe logging
- `tsafevalid` - Type-safe validation

## CI/CD Integration

### GitHub Actions Example
```yaml
- name: Check Type Safety
  run: make lint-typesafety

- name: Validate Migration
  run: make lint-migration
```

### Pre-commit Hooks
```bash
# Install hooks
make install-hooks

# Hooks will automatically:
# - Check for new interface{} usage
# - Suggest type-safe alternatives
# - Format migrated code
```

## Contributing

When contributing to the type safety migration:

1. **Follow established patterns** from completed migrations
2. **Use provided tools** for analysis and validation
3. **Maintain backward compatibility** during transition
4. **Add comprehensive tests** for new type-safe APIs
5. **Update documentation** for migrated components

## Resources

- [API Migration Reference](API_MIGRATION_REFERENCE.md)
- [Type Safety Best Practices](TYPE_SAFETY_BEST_PRACTICES.md)
- [Integration Guide](TYPE_SAFETY_INTEGRATION.md)
- [Migration Tools Documentation](MIGRATION_TOOLS.md)

## Support

For questions about the migration:
1. Check this documentation first
2. Use migration tools for analysis
3. Look at completed examples in transport package
4. Create an issue with the `type-safety` label