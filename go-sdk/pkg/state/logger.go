package state

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"time"
)

// Logger defines the interface for structured logging in the state management system
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	WithFields(fields ...Field) Logger
	WithContext(ctx context.Context) Logger
}

// Field represents a structured logging field
type Field struct {
	Key   string
	Value interface{}
}

// Common field constructors
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

func (l *structuredLogger) WithFields(fields ...Field) Logger {
	newFields := make([]Field, len(l.fields)+len(fields))
	copy(newFields, l.fields)
	copy(newFields[len(l.fields):], fields)
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

func (n NoOpLogger) Debug(msg string, fields ...Field) {}
func (n NoOpLogger) Info(msg string, fields ...Field)  {}
func (n NoOpLogger) Warn(msg string, fields ...Field)  {}
func (n NoOpLogger) Error(msg string, fields ...Field) {}
func (n NoOpLogger) WithFields(fields ...Field) Logger { return n }
func (n NoOpLogger) WithContext(ctx context.Context) Logger { return n }