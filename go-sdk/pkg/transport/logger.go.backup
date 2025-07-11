package transport

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	// LogLevelDebug represents debug level messages
	LogLevelDebug LogLevel = iota
	// LogLevelInfo represents info level messages
	LogLevelInfo
	// LogLevelWarn represents warning level messages
	LogLevelWarn
	// LogLevelError represents error level messages
	LogLevelError
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Level     LogLevel
	Timestamp time.Time
	Message   string
	Fields    map[string]interface{}
	Error     error
}

// Logger defines the interface for structured logging within the transport layer
type Logger interface {
	// Log logs a message with the given level and fields
	Log(level LogLevel, message string, fields ...Field)
	
	// Debug logs a debug message
	Debug(message string, fields ...Field)
	
	// Info logs an info message
	Info(message string, fields ...Field)
	
	// Warn logs a warning message
	Warn(message string, fields ...Field)
	
	// Error logs an error message
	Error(message string, fields ...Field)
	
	// WithFields returns a new logger with the given fields pre-populated
	WithFields(fields ...Field) Logger
	
	// WithContext returns a new logger with context
	WithContext(ctx context.Context) Logger
}

// Field represents a key-value pair for structured logging
type Field struct {
	Key   string
	Value interface{}
}

// String creates a string field
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an integer field
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 creates an int64 field
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Float64 creates a float64 field
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a boolean field
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Duration creates a duration field
func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

// Time creates a time field
func Time(key string, value time.Time) Field {
	return Field{Key: key, Value: value}
}

// Error creates an error field
func Error(err error) Field {
	return Field{Key: "error", Value: err}
}

// Any creates a field with any value
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// LoggerConfig defines configuration for the logger
type LoggerConfig struct {
	// Level is the minimum log level to output
	Level LogLevel
	
	// Format is the output format ("json" or "text")
	Format string
	
	// Output is the output destination (os.Stdout, os.Stderr, or file)
	Output *os.File
	
	// TimestampFormat is the format for timestamps
	TimestampFormat string
	
	// EnableCaller enables caller information in logs
	EnableCaller bool
	
	// EnableStacktrace enables stacktrace for error logs
	EnableStacktrace bool
}

// DefaultLoggerConfig returns a default logger configuration
func DefaultLoggerConfig() *LoggerConfig {
	return &LoggerConfig{
		Level:           LogLevelInfo,
		Format:          "text",
		Output:          os.Stdout,
		TimestampFormat: time.RFC3339,
		EnableCaller:    false,
		EnableStacktrace: false,
	}
}

// defaultLogger implements the Logger interface using Go's standard log package
type defaultLogger struct {
	config *LoggerConfig
	logger *log.Logger
	fields []Field
}

// NewLogger creates a new logger with the given configuration
func NewLogger(config *LoggerConfig) Logger {
	if config == nil {
		config = DefaultLoggerConfig()
	}
	
	return &defaultLogger{
		config: config,
		logger: log.New(config.Output, "", 0),
		fields: make([]Field, 0),
	}
}

// Log implements the Logger interface
func (l *defaultLogger) Log(level LogLevel, message string, fields ...Field) {
	if level < l.config.Level {
		return
	}
	
	entry := LogEntry{
		Level:     level,
		Timestamp: time.Now(),
		Message:   message,
		Fields:    make(map[string]interface{}),
	}
	
	// Add pre-populated fields
	for _, field := range l.fields {
		entry.Fields[field.Key] = field.Value
	}
	
	// Add new fields
	for _, field := range fields {
		entry.Fields[field.Key] = field.Value
		if field.Key == "error" {
			if err, ok := field.Value.(error); ok {
				entry.Error = err
			}
		}
	}
	
	l.write(entry)
}

// Debug implements the Logger interface
func (l *defaultLogger) Debug(message string, fields ...Field) {
	l.Log(LogLevelDebug, message, fields...)
}

// Info implements the Logger interface
func (l *defaultLogger) Info(message string, fields ...Field) {
	l.Log(LogLevelInfo, message, fields...)
}

// Warn implements the Logger interface
func (l *defaultLogger) Warn(message string, fields ...Field) {
	l.Log(LogLevelWarn, message, fields...)
}

// Error implements the Logger interface
func (l *defaultLogger) Error(message string, fields ...Field) {
	l.Log(LogLevelError, message, fields...)
}

// WithFields implements the Logger interface
func (l *defaultLogger) WithFields(fields ...Field) Logger {
	newFields := make([]Field, 0, len(l.fields)+len(fields))
	newFields = append(newFields, l.fields...)
	newFields = append(newFields, fields...)
	
	return &defaultLogger{
		config: l.config,
		logger: l.logger,
		fields: newFields,
	}
}

// WithContext implements the Logger interface
func (l *defaultLogger) WithContext(ctx context.Context) Logger {
	// For now, just return the same logger
	// In a real implementation, this could extract context values
	return l
}

// write writes a log entry to the output
func (l *defaultLogger) write(entry LogEntry) {
	var output string
	
	switch l.config.Format {
	case "json":
		output = l.formatJSON(entry)
	default:
		output = l.formatText(entry)
	}
	
	l.logger.Print(output)
}

// formatText formats a log entry as text
func (l *defaultLogger) formatText(entry LogEntry) string {
	timestamp := entry.Timestamp.Format(l.config.TimestampFormat)
	
	output := fmt.Sprintf("[%s] %s %s", timestamp, entry.Level.String(), entry.Message)
	
	// Add fields
	for key, value := range entry.Fields {
		if key == "error" {
			continue // Handle error separately
		}
		output += fmt.Sprintf(" %s=%v", key, value)
	}
	
	// Add error if present
	if entry.Error != nil {
		output += fmt.Sprintf(" error=%v", entry.Error)
	}
	
	return output
}

// formatJSON formats a log entry as JSON
func (l *defaultLogger) formatJSON(entry LogEntry) string {
	// Simple JSON formatting - in a real implementation, you'd use json.Marshal
	output := fmt.Sprintf(`{"timestamp":"%s","level":"%s","message":"%s"`,
		entry.Timestamp.Format(l.config.TimestampFormat),
		entry.Level.String(),
		entry.Message)
	
	// Add fields
	for key, value := range entry.Fields {
		if key == "error" {
			continue // Handle error separately
		}
		output += fmt.Sprintf(`,"%s":"%v"`, key, value)
	}
	
	// Add error if present
	if entry.Error != nil {
		output += fmt.Sprintf(`,"error":"%v"`, entry.Error)
	}
	
	output += "}"
	return output
}

// NoopLogger is a logger that does nothing
type NoopLogger struct{}

// NewNoopLogger creates a new noop logger
func NewNoopLogger() Logger {
	return &NoopLogger{}
}

// Log implements the Logger interface
func (n *NoopLogger) Log(level LogLevel, message string, fields ...Field) {}

// Debug implements the Logger interface
func (n *NoopLogger) Debug(message string, fields ...Field) {}

// Info implements the Logger interface
func (n *NoopLogger) Info(message string, fields ...Field) {}

// Warn implements the Logger interface
func (n *NoopLogger) Warn(message string, fields ...Field) {}

// Error implements the Logger interface
func (n *NoopLogger) Error(message string, fields ...Field) {}

// WithFields implements the Logger interface
func (n *NoopLogger) WithFields(fields ...Field) Logger {
	return n
}

// WithContext implements the Logger interface
func (n *NoopLogger) WithContext(ctx context.Context) Logger {
	return n
}