package state

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"time"
)

// LogValue defines the interface for type-safe log values
// This constraint ensures only safe, serializable types can be logged
type LogValue interface {
	~string | ~int | ~int8 | ~int16 | ~int32 |
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
	~float32 | ~float64 | ~bool |
	time.Time | time.Duration
}

// SafeInt64Value is a separate constraint for int64 to avoid overlap with time.Duration
type SafeInt64Value interface {
	~int64
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

// Field represents a structured logging field (legacy interface{} version for compatibility)
type Field struct {
	Key   string
	Value interface{}
}

// FieldProvider interface for type-safe field creation
type FieldProvider interface {
	ToField() Field
}

// Logger defines the interface for structured logging in the state management system
type Logger interface {
	// Legacy field-based methods for backward compatibility
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	WithFields(fields ...Field) Logger
	WithContext(ctx context.Context) Logger
	
	// Type-safe logging methods
	DebugTyped(msg string, fields ...FieldProvider)
	InfoTyped(msg string, fields ...FieldProvider)
	WarnTyped(msg string, fields ...FieldProvider)
	ErrorTyped(msg string, fields ...FieldProvider)
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

func Time(key string, value time.Time) Field {
	return Field{Key: key, Value: value}
}

func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

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

func SafeTime(key string, value time.Time) TypedField[time.Time] {
	return TypedField[time.Time]{Key: key, Value: value}
}

func SafeDuration(key string, value time.Duration) TypedField[time.Duration] {
	return TypedField[time.Duration]{Key: key, Value: value}
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

// structuredLogger implements the Logger interface using slog
type structuredLogger struct {
	logger *slog.Logger
	fields []Field
}

// NewLogger creates a new structured logger
func NewLogger(handler slog.Handler) Logger {
	if handler == nil {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	}
	return &structuredLogger{
		logger: slog.New(handler),
		fields: nil,
	}
}

// DefaultLogger creates a default logger with JSON output
func DefaultLogger() Logger {
	return NewLogger(nil)
}

func (l *structuredLogger) Debug(msg string, fields ...Field) {
	l.log(slog.LevelDebug, msg, fields...)
}

func (l *structuredLogger) Info(msg string, fields ...Field) {
	l.log(slog.LevelInfo, msg, fields...)
}

func (l *structuredLogger) Warn(msg string, fields ...Field) {
	l.log(slog.LevelWarn, msg, fields...)
}

func (l *structuredLogger) Error(msg string, fields ...Field) {
	l.log(slog.LevelError, msg, fields...)
}

// Type-safe logging methods
func (l *structuredLogger) DebugTyped(msg string, fields ...FieldProvider) {
	l.logTyped(slog.LevelDebug, msg, fields...)
}

func (l *structuredLogger) InfoTyped(msg string, fields ...FieldProvider) {
	l.logTyped(slog.LevelInfo, msg, fields...)
}

func (l *structuredLogger) WarnTyped(msg string, fields ...FieldProvider) {
	l.logTyped(slog.LevelWarn, msg, fields...)
}

func (l *structuredLogger) ErrorTyped(msg string, fields ...FieldProvider) {
	l.logTyped(slog.LevelError, msg, fields...)
}

func (l *structuredLogger) WithFields(fields ...Field) Logger {
	newFields := make([]Field, len(l.fields)+len(fields))
	copy(newFields, l.fields)
	copy(newFields[len(l.fields):], fields)
	return &structuredLogger{
		logger: l.logger,
		fields: newFields,
	}
}

func (l *structuredLogger) WithTypedFields(fields ...FieldProvider) Logger {
	newFields := make([]Field, len(l.fields)+len(fields))
	copy(newFields, l.fields)
	for i, field := range fields {
		newFields[len(l.fields)+i] = field.ToField()
	}
	return &structuredLogger{
		logger: l.logger,
		fields: newFields,
	}
}

func (l *structuredLogger) WithContext(ctx context.Context) Logger {
	// Extract context values if needed
	// For now, just return self
	return l
}

func (l *structuredLogger) log(level slog.Level, msg string, fields ...Field) {
	// Add goroutine ID
	gid := getGoroutineID()

	// Combine persistent fields with call fields
	allFields := make([]Field, 0, len(l.fields)+len(fields)+2)
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)
	allFields = append(allFields,
		Int64("goroutine_id", gid),
		Time("timestamp", time.Now()),
	)

	// Convert to slog attributes
	attrs := make([]slog.Attr, 0, len(allFields))
	for _, f := range allFields {
		attrs = append(attrs, slog.Any(f.Key, f.Value))
	}

	l.logger.LogAttrs(context.Background(), level, msg, attrs...)
}

// logTyped handles type-safe logging
func (l *structuredLogger) logTyped(level slog.Level, msg string, fields ...FieldProvider) {
	// Add goroutine ID
	gid := getGoroutineID()
	
	// Combine persistent fields with call fields
	allFields := make([]Field, 0, len(l.fields)+len(fields)+2)
	allFields = append(allFields, l.fields...)
	for _, field := range fields {
		allFields = append(allFields, field.ToField())
	}
	allFields = append(allFields, 
		Int64("goroutine_id", gid),
		Time("timestamp", time.Now()),
	)
	
	// Convert to slog attributes
	attrs := make([]slog.Attr, 0, len(allFields))
	for _, f := range allFields {
		attrs = append(attrs, slog.Any(f.Key, f.Value))
	}
	
	l.logger.LogAttrs(context.Background(), level, msg, attrs...)
}

// getGoroutineID extracts the current goroutine ID
func getGoroutineID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := string(buf[:n])
	var id int64
	fmt.Sscanf(idField, "goroutine %d", &id)
	return id
}

// NoOpLogger is a logger that discards all log messages
type NoOpLogger struct{}

func (n NoOpLogger) Debug(msg string, fields ...Field)      {}
func (n NoOpLogger) Info(msg string, fields ...Field)       {}
func (n NoOpLogger) Warn(msg string, fields ...Field)       {}
func (n NoOpLogger) Error(msg string, fields ...Field)      {}
func (n NoOpLogger) WithFields(fields ...Field) Logger      { return n }
func (n NoOpLogger) WithContext(ctx context.Context) Logger { return n }

// Type-safe NoOp methods
func (n NoOpLogger) DebugTyped(msg string, fields ...FieldProvider) {}
func (n NoOpLogger) InfoTyped(msg string, fields ...FieldProvider)  {}
func (n NoOpLogger) WarnTyped(msg string, fields ...FieldProvider)  {}
func (n NoOpLogger) ErrorTyped(msg string, fields ...FieldProvider) {}
func (n NoOpLogger) WithTypedFields(fields ...FieldProvider) Logger { return n }
