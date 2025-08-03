# Configurable Timeouts for Tests

This document describes the comprehensive solution for making all time-based configurations in the Go SDK configurable for tests, allowing for faster test execution without breaking production functionality.

## Overview

The solution provides a centralized time configuration system that automatically detects test mode and applies appropriate timeouts:
- **Test Mode**: Short timeouts (1s, 100ms, etc.) for fast test execution
- **Production Mode**: Full timeouts (30s, 60s, etc.) for reliable operation

## Key Features

### 1. Automatic Test Mode Detection

The system automatically detects test mode through multiple methods:
- Running under `go test`
- Environment variable `AG_SDK_TEST_MODE=true`
- CI environment detection (`CI=true` or `AG_SDK_CI=true`)

### 2. Centralized Configuration

All timeout configurations are centralized in `/pkg/internal/timeconfig/config.go`:
- Single source of truth for all timeouts
- Consistent behavior across all packages
- Easy to maintain and update

### 3. Backward Compatibility

The solution maintains full backward compatibility:
- Existing code continues to work without changes
- Production timeouts remain unchanged
- Test timeouts are automatically applied only in test environments

## Implementation Details

### Core Components

1. **TimeConfig Structure** (`pkg/internal/timeconfig/config.go`)
   - Contains all timeout configurations
   - Provides production and test configurations
   - Thread-safe access with sync.RWMutex

2. **State Management** (`pkg/state/constants.go`)
   - Replaced hardcoded constants with configurable functions
   - All state management timeouts now adapt to environment

3. **WebSocket Transport** (`pkg/transport/websocket/`)
   - Connection timeouts are now configurable
   - Performance configurations adapt to test/production
   - Security timeouts use centralized configuration

4. **Tools Package** (`pkg/tools/`)
   - HTTP timeouts are configurable
   - Tool execution timeouts adapt to environment
   - I/O and validation timeouts use centralized config

### Configuration Functions

Instead of constants, use these functions:

```go
// State Management
state.GetDefaultShutdownTimeout()    // 30s → 1s in tests
state.GetDefaultUpdateTimeout()      // 30s → 1s in tests
state.GetDefaultRetryDelay()         // 100ms → 10ms in tests

// WebSocket
websocket.DefaultConnectionConfig()  // Uses configurable timeouts
// DialTimeout: 10s → 500ms in tests
// ReadTimeout: 60s → 1s in tests

// Tools
timeconfig.HTTPTimeout()            // 60s → 1s in tests (internal use)
timeconfig.ToolExecutionTimeout()   // 30s → 1s in tests (internal use)
```

## Usage Examples

### Basic Usage

No code changes required for existing functionality:

```go
// This automatically uses appropriate timeouts based on environment
config := websocket.DefaultConnectionConfig()
manager := state.NewManager(state.DefaultManagerOptions())
```

### Advanced Usage

For testing scenarios requiring specific timeouts:

```go
// In test code only
cleanup := timeconfig.OverrideForTest(map[string]time.Duration{
    "shutdown": 100 * time.Millisecond,
    "http":     500 * time.Millisecond,
})
defer cleanup()

// Now all timeout functions return the overridden values
```

## Environment Configuration

### Test Mode Detection

The system automatically detects test mode when:
- Running under `go test`
- `AG_SDK_TEST_MODE=true` environment variable is set
- `CI=true` or `AG_SDK_CI=true` environment variables are set

### Manual Configuration

You can override the detection by setting environment variables:

```bash
export AG_SDK_TEST_MODE=true   # Force test mode
export AG_SDK_TEST_MODE=false  # Force production mode (if not under go test)
```

## Timeout Comparisons

| Component | Production | Test Mode | Speedup |
|-----------|------------|-----------|---------|
| Shutdown | 30s | 1s | 30x |
| HTTP Requests | 60s | 1s | 60x |
| WebSocket Dial | 10s | 500ms | 20x |
| WebSocket Read | 60s | 1s | 60x |
| Update Operations | 30s | 1s | 30x |
| Retry Delays | 100ms | 10ms | 10x |
| Batch Timeouts | 100ms | 10ms | 10x |

## Testing

### Running Tests

All existing tests continue to work without changes:

```bash
go test ./pkg/state/...        # Uses 1s timeouts
go test ./pkg/transport/...    # Uses 500ms-1s timeouts  
go test ./pkg/tools/...        # Uses 1s timeouts
```

### Verifying Configuration

Run the example to see current timeouts:

```bash
cd examples/configurable_timeouts
go run main.go
```

### Test Suite

The timeconfig package includes comprehensive tests:

```bash
go test ./pkg/internal/timeconfig/ -v
```

## Migration Guide

### For Existing Code

No changes required! The system automatically:
- Detects existing usage of timeout constants
- Provides backward-compatible function equivalents
- Applies appropriate timeouts based on environment

### For New Code

Use the new functions instead of hardcoded timeouts:

```go
// Instead of:
timeout := 30 * time.Second

// Use:
timeout := state.GetDefaultShutdownTimeout()
```

## Architecture Benefits

1. **Fast Tests**: Test execution is significantly faster with short timeouts
2. **Reliable Production**: Full timeouts ensure robust production operation
3. **Maintainable**: Single source of truth for all timeout configurations
4. **Flexible**: Easy to add new timeouts or adjust existing ones
5. **Safe**: Automatic detection prevents accidental production issues

## Files Modified

### Core Implementation
- `/pkg/internal/timeconfig/config.go` - New centralized configuration
- `/pkg/internal/timeconfig/config_test.go` - Comprehensive tests

### State Management
- `/pkg/state/constants.go` - Replaced constants with functions
- `/pkg/state/manager.go` - Updated to use configurable timeouts
- `/pkg/state/monitoring.go` - Updated metrics intervals
- `/pkg/state/performance.go` - Updated batch timeouts
- `/pkg/state/store.go` - Updated subscription timeouts

### WebSocket Transport
- `/pkg/transport/websocket/connection.go` - Configurable connection timeouts
- `/pkg/transport/websocket/performance.go` - Configurable performance timeouts
- `/pkg/transport/websocket/heartbeat.go` - Configurable heartbeat timeout
- `/pkg/transport/websocket/security.go` - Configurable security timeouts

### Tools
- `/pkg/tools/executor.go` - Configurable execution timeouts
- `/pkg/tools/builtin.go` - Configurable HTTP and I/O timeouts

### Examples
- `/examples/configurable_timeouts/main.go` - Demonstration of the system

## Future Enhancements

The system is designed to be easily extensible:
- Add new timeout categories as needed
- Implement environment-specific configurations
- Add runtime timeout adjustment capabilities
- Integrate with external configuration systems

## Troubleshooting

### Tests Still Slow

Verify test mode detection:
```go
import "github.com/mattsp1290/ag-ui/go-sdk/pkg/internal/timeconfig"
fmt.Printf("Test mode: %v\n", timeconfig.IsTestMode())
```

### Production Issues

Check that production mode is active:
```bash
unset AG_SDK_TEST_MODE  # Ensure not forcing test mode
```

### Custom Timeouts

For special cases, use the override system:
```go
cleanup := timeconfig.OverrideForTest(map[string]time.Duration{
    "custom_timeout": 50 * time.Millisecond,
})
defer cleanup()
```

This comprehensive solution provides fast, reliable, and maintainable timeout management across the entire Go SDK while maintaining full backward compatibility.