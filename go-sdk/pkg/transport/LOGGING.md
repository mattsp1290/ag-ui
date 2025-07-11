# Structured Logging in Transport Layer

This document describes the structured logging implementation for the transport layer, which provides comprehensive logging capabilities for transport lifecycle events, errors, and performance metrics.

## Overview

The transport layer now includes a structured logging system that:

1. **Provides a unified logging interface** that can be implemented by different loggers
2. **Logs key transport events** including lifecycle, errors, and performance metrics
3. **Supports configurable log levels** and output formats
4. **Follows Go logging best practices** with structured fields and context
5. **Is optional and configurable** - can be disabled or customized per transport

## Components

### Logger Interface

The core `Logger` interface provides methods for structured logging:

```go
type Logger interface {
    Log(level LogLevel, message string, fields ...Field)
    Debug(message string, fields ...Field)
    Info(message string, fields ...Field)
    Warn(message string, fields ...Field)
    Error(message string, fields ...Field)
    WithFields(fields ...Field) Logger
    WithContext(ctx context.Context) Logger
}
```

### Log Levels

Four log levels are supported:

- `LogLevelDebug` - Detailed debugging information
- `LogLevelInfo` - General information about transport operations
- `LogLevelWarn` - Warning conditions that should be noted
- `LogLevelError` - Error conditions that require attention

### Structured Fields

Fields provide structured context to log messages:

```go
logger.Info("Event sent", 
    String("event_id", "msg-123"),
    String("transport_type", "websocket"),
    Duration("duration", 50*time.Millisecond),
    Int("retry_count", 2))
```

Available field types:
- `String(key, value)` - String values
- `Int(key, value)` - Integer values  
- `Int64(key, value)` - 64-bit integers
- `Float64(key, value)` - Float values
- `Bool(key, value)` - Boolean values
- `Duration(key, value)` - Time durations
- `Time(key, value)` - Time values
- `Error(err)` - Error values
- `Any(key, value)` - Any value type

### Logger Configuration

Logger behavior is controlled by `LoggerConfig`:

```go
type LoggerConfig struct {
    Level           LogLevel    // Minimum log level to output
    Format          string      // Output format ("json" or "text")
    Output          *os.File    // Output destination
    TimestampFormat string      // Timestamp format
    EnableCaller    bool        // Include caller information
    EnableStacktrace bool       // Include stacktrace for errors
}
```

## Usage Examples

### Basic Logging

```go
// Create a logger with default configuration
logger := transport.NewLogger(transport.DefaultLoggerConfig())

// Log messages at different levels
logger.Info("Transport started")
logger.Debug("Connection details", transport.String("endpoint", "ws://localhost:8080"))
logger.Warn("High latency detected", transport.Duration("latency", 500*time.Millisecond))
logger.Error("Connection failed", transport.Error(err))
```

### Manager with Logging

```go
// Create a simple manager with logging
logger := transport.NewLogger(transport.DefaultLoggerConfig())
manager := transport.NewSimpleManagerWithLogger(logger)

// Or for the full manager
config := &transport.Config{
    Primary:     "websocket",
    Fallback:    []string{"sse", "http"},
    BufferSize:  1024,
    LogLevel:    "info",
    EnableMetrics: true,
}
manager := transport.NewManagerWithLogger(config, logger)
```

### Error Logging

```go
// Create errors with automatic logging
logger := transport.NewLogger(transport.DefaultLoggerConfig())

// Transport error with logging
err := transport.NewTransportErrorWithLogger(
    "websocket",
    "connect", 
    fmt.Errorf("connection refused"),
    logger,
)

// Temporary error with logging
tempErr := transport.NewTemporaryErrorWithLogger(
    "websocket",
    "send",
    fmt.Errorf("network timeout"),
    logger,
)
```

### Custom Logger Implementation

```go
// Implement your own logger
type CustomLogger struct {
    // your logger implementation
}

func (l *CustomLogger) Log(level LogLevel, message string, fields ...Field) {
    // implement your logging logic
}

func (l *CustomLogger) Info(message string, fields ...Field) {
    l.Log(LogLevelInfo, message, fields...)
}

// ... implement other methods

// Use with transport
manager := transport.NewSimpleManagerWithLogger(&CustomLogger{})
```

## Logged Events

### Transport Manager Events

#### SimpleManager

- **Lifecycle Events**:
  - Manager start/stop
  - Transport connect/disconnect
  - Transport switching
  
- **Operation Events**:
  - Event sending/receiving
  - Error handling
  - Channel draining

- **Performance Events**:
  - Send durations
  - Event processing metrics

#### Full Manager

- **State Changes**:
  - Manager start/stop
  - Transport switching
  - Middleware configuration
  - Failover events

- **Performance Metrics**:
  - Message send/receive counts
  - Byte transfer metrics
  - Latency measurements
  - Transport health scores

- **Error Conditions**:
  - Transport failures
  - Failover triggers
  - Configuration errors

### Transport-Specific Events

Each transport implementation can log:

- Connection establishment/teardown
- Data transmission events
- Protocol-specific events
- Health check results
- Performance metrics

## Configuration

### Environment Variables

Configure logging behavior through environment variables:

```bash
# Log level
export AG_UI_LOG_LEVEL=info

# Log format
export AG_UI_LOG_FORMAT=json

# Enable debug logging
export AG_UI_LOG_LEVEL=debug
```

### Code Configuration

```go
// Custom logger configuration
config := &transport.LoggerConfig{
    Level:           transport.LogLevelInfo,
    Format:          "json",
    Output:          os.Stdout,
    TimestampFormat: time.RFC3339,
    EnableCaller:    true,
    EnableStacktrace: true,
}

logger := transport.NewLogger(config)
```

### Disable Logging

```go
// Use no-op logger to disable logging
manager := transport.NewSimpleManagerWithLogger(transport.NewNoopLogger())

// Or use nil logger (will default to no-op)
manager := transport.NewSimpleManagerWithLogger(nil)
```

## Best Practices

1. **Use appropriate log levels**:
   - Debug for detailed troubleshooting
   - Info for normal operation events
   - Warn for concerning but non-fatal conditions
   - Error for failures requiring attention

2. **Include relevant context**:
   - Transport type and endpoint
   - Operation being performed
   - Relevant identifiers (event ID, connection ID)
   - Performance metrics (duration, size)

3. **Avoid logging sensitive data**:
   - Don't log authentication tokens
   - Don't log personal information
   - Consider sanitizing URLs with credentials

4. **Use structured fields**:
   - Prefer structured fields over string formatting
   - Use consistent field names across the codebase
   - Include units for measurements

5. **Configure appropriately for production**:
   - Use Info level or higher in production
   - Consider JSON format for log aggregation
   - Ensure log rotation is configured

## Integration with External Loggers

The logging interface is designed to be easily integrated with popular Go logging libraries:

- **Logrus**: Implement the Logger interface wrapping logrus
- **Zap**: Implement the Logger interface wrapping zap
- **slog**: Implement the Logger interface wrapping slog (Go 1.21+)

Example integration with logrus:

```go
type LogrusLogger struct {
    logger *logrus.Logger
}

func (l *LogrusLogger) Info(message string, fields ...transport.Field) {
    entry := l.logger.WithFields(l.convertFields(fields))
    entry.Info(message)
}

func (l *LogrusLogger) convertFields(fields []transport.Field) logrus.Fields {
    result := make(logrus.Fields)
    for _, field := range fields {
        result[field.Key] = field.Value
    }
    return result
}
```

## Performance Considerations

- The default logger is thread-safe and efficient
- No-op logger has minimal overhead when logging is disabled
- Structured fields are only evaluated when the log level is enabled
- Consider buffering for high-throughput scenarios

## Future Enhancements

- Integration with distributed tracing (OpenTelemetry)
- Metrics collection and export
- Log sampling for high-volume scenarios
- Automated log analysis and alerting