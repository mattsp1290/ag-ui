package transport

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

// LogValue defines the interface for type-safe log values
// This constraint ensures only safe, serializable types can be logged
type LogValue interface {
	~string | ~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | ~bool |
		time.Time
}

// TypedField represents a type-safe structured logging field
type TypedField[T LogValue] struct {
	Key   string
	Value T
}

// ToField converts a TypedField to a legacy Field for backward compatibility
func (tf TypedField[T]) ToField() Field {
	return Field{Key: tf.Key, Value: tf.Value}
}

// Int64Field represents an int64 field with special handling
type Int64Field struct {
	Key   string
	Value int64
}

// ToField converts an Int64Field to a legacy Field
func (i64f Int64Field) ToField() Field {
	return Field{Key: i64f.Key, Value: i64f.Value}
}

// ErrorField represents an error field with special handling
type ErrorField struct {
	Key   string
	Value error
}

// ToField converts an ErrorField to a legacy Field
func (ef ErrorField) ToField() Field {
	return Field{Key: ef.Key, Value: ef.Value}
}

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

// Field represents a key-value pair for structured logging (legacy interface{} version for compatibility)
type Field struct {
	Key   string
	Value interface{}
}

// FieldProvider interface for type-safe field creation
type FieldProvider interface {
	ToField() Field
}

// Logger defines the interface for structured logging within the transport layer
type Logger interface {
	// Legacy field-based methods for backward compatibility
	Log(level LogLevel, message string, fields ...Field)
	Debug(message string, fields ...Field)
	Info(message string, fields ...Field)
	Warn(message string, fields ...Field)
	Error(message string, fields ...Field)
	WithFields(fields ...Field) Logger
	WithContext(ctx context.Context) Logger

	// Type-safe logging methods
	LogTyped(level LogLevel, message string, fields ...FieldProvider)
	DebugTyped(message string, fields ...FieldProvider)
	InfoTyped(message string, fields ...FieldProvider)
	WarnTyped(message string, fields ...FieldProvider)
	ErrorTyped(message string, fields ...FieldProvider)
	WithTypedFields(fields ...FieldProvider) Logger
}

// Backward compatible field constructors - return Field for existing code
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

func Time(key string, value time.Time) Field {
	return Field{Key: key, Value: value}
}

func Error(err error) Field {
	return Field{Key: "error", Value: err}
}

// Err creates an error field with the standard "error" key
func Err(err error) Field {
	return Field{Key: "error", Value: err}
}

func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// New type-safe field constructors with SafeXxx naming
func SafeString(key, value string) TypedField[string] {
	return TypedField[string]{Key: key, Value: value}
}

func SafeInt(key string, value int) TypedField[int] {
	return TypedField[int]{Key: key, Value: value}
}

func SafeInt8(key string, value int8) TypedField[int8] {
	return TypedField[int8]{Key: key, Value: value}
}

func SafeInt16(key string, value int16) TypedField[int16] {
	return TypedField[int16]{Key: key, Value: value}
}

func SafeInt32(key string, value int32) TypedField[int32] {
	return TypedField[int32]{Key: key, Value: value}
}

func SafeInt64(key string, value int64) Int64Field {
	return Int64Field{Key: key, Value: value}
}

func SafeUint(key string, value uint) TypedField[uint] {
	return TypedField[uint]{Key: key, Value: value}
}

func SafeUint8(key string, value uint8) TypedField[uint8] {
	return TypedField[uint8]{Key: key, Value: value}
}

func SafeUint16(key string, value uint16) TypedField[uint16] {
	return TypedField[uint16]{Key: key, Value: value}
}

func SafeUint32(key string, value uint32) TypedField[uint32] {
	return TypedField[uint32]{Key: key, Value: value}
}

func SafeUint64(key string, value uint64) TypedField[uint64] {
	return TypedField[uint64]{Key: key, Value: value}
}

func SafeFloat32(key string, value float32) TypedField[float32] {
	return TypedField[float32]{Key: key, Value: value}
}

func SafeFloat64(key string, value float64) TypedField[float64] {
	return TypedField[float64]{Key: key, Value: value}
}

func SafeBool(key string, value bool) TypedField[bool] {
	return TypedField[bool]{Key: key, Value: value}
}

func SafeDuration(key string, value time.Duration) TypedField[time.Duration] {
	return TypedField[time.Duration]{Key: key, Value: value}
}

func SafeTime(key string, value time.Time) TypedField[time.Time] {
	return TypedField[time.Time]{Key: key, Value: value}
}

func SafeError(key string, err error) ErrorField {
	return ErrorField{Key: key, Value: err}
}

func SafeErr(err error) ErrorField {
	return ErrorField{Key: "error", Value: err}
}

// Type-safe generic field constructor
func TypedValue[T LogValue](key string, value T) TypedField[T] {
	return TypedField[T]{Key: key, Value: value}
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
		Level:            LogLevelInfo,
		Format:           "text",
		Output:           os.Stdout,
		TimestampFormat:  time.RFC3339,
		EnableCaller:     false,
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

// LogTyped implements type-safe logging
func (l *defaultLogger) LogTyped(level LogLevel, message string, fields ...FieldProvider) {
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

	// Add new type-safe fields
	for _, fieldProvider := range fields {
		field := fieldProvider.ToField()
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

// Type-safe logging methods
func (l *defaultLogger) DebugTyped(message string, fields ...FieldProvider) {
	l.LogTyped(LogLevelDebug, message, fields...)
}

func (l *defaultLogger) InfoTyped(message string, fields ...FieldProvider) {
	l.LogTyped(LogLevelInfo, message, fields...)
}

func (l *defaultLogger) WarnTyped(message string, fields ...FieldProvider) {
	l.LogTyped(LogLevelWarn, message, fields...)
}

func (l *defaultLogger) ErrorTyped(message string, fields ...FieldProvider) {
	l.LogTyped(LogLevelError, message, fields...)
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

// WithTypedFields implements type-safe field chaining
func (l *defaultLogger) WithTypedFields(fields ...FieldProvider) Logger {
	newFields := make([]Field, 0, len(l.fields)+len(fields))
	newFields = append(newFields, l.fields...)
	for _, fieldProvider := range fields {
		newFields = append(newFields, fieldProvider.ToField())
	}

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

// Type-safe NoOp methods
func (n *NoopLogger) LogTyped(level LogLevel, message string, fields ...FieldProvider) {}
func (n *NoopLogger) DebugTyped(message string, fields ...FieldProvider)               {}
func (n *NoopLogger) InfoTyped(message string, fields ...FieldProvider)                {}
func (n *NoopLogger) WarnTyped(message string, fields ...FieldProvider)                {}
func (n *NoopLogger) ErrorTyped(message string, fields ...FieldProvider)               {}
func (n *NoopLogger) WithTypedFields(fields ...FieldProvider) Logger                   { return n }
