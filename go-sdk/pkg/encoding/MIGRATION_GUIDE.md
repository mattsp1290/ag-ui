# Encoding Registration Migration Guide

## Overview

The encoding system has been updated to replace panic-prone `init()` functions with explicit registration functions. This change improves error handling and testability while maintaining backward compatibility.

## What Changed

### Before
```go
// init() functions would panic on registration failure
func init() {
    if err := registry.RegisterFormat(formatInfo); err != nil {
        panic("failed to register format: " + err.Error())
    }
}
```

### After
```go
// init() still exists but doesn't panic
func init() {
    _ = Register() // Errors are captured but not panicked
}

// New explicit registration functions
func Register() error
func EnsureRegistered() error
func RegisterTo(registry *encoding.FormatRegistry) error
```

## Migration Steps

### For Most Users (No Changes Required)

If you're simply importing the encoding packages, **no changes are required**:

```go
import (
    _ "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"
    _ "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/protobuf"
)
```

The `init()` functions still exist and will attempt registration automatically. The only difference is they won't panic on failure.

### For Applications Requiring Registration Verification

If your application needs to ensure codecs are properly registered:

```go
import (
    "log"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/protobuf"
)

func main() {
    // Ensure JSON codec is registered
    if err := json.EnsureRegistered(); err != nil {
        log.Fatalf("Failed to register JSON codec: %v", err)
    }
    
    // Ensure Protobuf codec is registered
    if err := protobuf.EnsureRegistered(); err != nil {
        log.Fatalf("Failed to register Protobuf codec: %v", err)
    }
    
    // Your application code...
}
```

### For Testing with Custom Registries

The new API makes it easy to use custom registries for testing:

```go
func TestMyFeature(t *testing.T) {
    // Create an isolated registry for testing
    testRegistry := encoding.NewFormatRegistry()
    
    // Register only the codecs you need
    if err := json.RegisterTo(testRegistry); err != nil {
        t.Fatalf("Failed to register JSON: %v", err)
    }
    
    // Use testRegistry for your tests...
}
```

### For Library Authors

If you're building a library that depends on specific codecs:

```go
package mylib

import (
    "sync"
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"
)

var initOnce sync.Once
var initErr error

// Init ensures required codecs are registered
func Init() error {
    initOnce.Do(func() {
        initErr = json.EnsureRegistered()
    })
    return initErr
}
```

## Benefits

1. **No More Panics**: Registration failures return errors instead of panicking
2. **Better Testing**: Use custom registries for isolated testing
3. **Explicit Control**: Applications can verify registration succeeded
4. **Backward Compatible**: Existing code continues to work without changes
5. **Thread-Safe**: Registration functions are idempotent and thread-safe

## API Reference

### JSON Package (`encoding/json`)

- `Register() error` - Register with global registry (idempotent)
- `EnsureRegistered() error` - Verify registration succeeded
- `RegisterTo(registry *encoding.FormatRegistry) error` - Register with custom registry

### Protobuf Package (`encoding/protobuf`)

- `Register() error` - Register with global registry (idempotent)
- `EnsureRegistered() error` - Verify registration succeeded
- `RegisterTo(registry *encoding.FormatRegistry) error` - Register with custom registry

## Common Patterns

### Pattern 1: Fail-Fast Application Startup
```go
func main() {
    // Verify all required codecs at startup
    if err := json.EnsureRegistered(); err != nil {
        log.Fatal(err)
    }
    if err := protobuf.EnsureRegistered(); err != nil {
        log.Fatal(err)
    }
    
    // Start application...
}
```

### Pattern 2: Lazy Registration Check
```go
func processData(format string, data []byte) error {
    switch format {
    case "json":
        if err := json.EnsureRegistered(); err != nil {
            return fmt.Errorf("JSON codec not available: %w", err)
        }
        // Process JSON...
    case "protobuf":
        if err := protobuf.EnsureRegistered(); err != nil {
            return fmt.Errorf("Protobuf codec not available: %w", err)
        }
        // Process Protobuf...
    }
}
```

### Pattern 3: Custom Registry for Multi-Tenancy
```go
func createTenantRegistry(tenantID string) (*encoding.FormatRegistry, error) {
    registry := encoding.NewFormatRegistry()
    
    // Register formats based on tenant configuration
    if tenantConfig[tenantID].EnableJSON {
        if err := json.RegisterTo(registry); err != nil {
            return nil, err
        }
    }
    
    if tenantConfig[tenantID].EnableProtobuf {
        if err := protobuf.RegisterTo(registry); err != nil {
            return nil, err
        }
    }
    
    return registry, nil
}
```