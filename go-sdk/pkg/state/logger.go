package state

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap/zapcore"
)

// LogLevel constants for backwards compatibility
const (
	LogLevelDebug = slog.LevelDebug
	LogLevelInfo  = slog.LevelInfo
	LogLevelWarn  = slog.LevelWarn
	LogLevelError = slog.LevelError
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
	// Check if we're in a test environment
	if isTestEnvironment() {
		return NewTestLogger()
	}
	return NewLogger(nil)
}

// NewStructuredLogger creates a new structured logger with a monitoring config
func NewStructuredLogger(config *MonitoringConfig) Logger {
	if config == nil {
		return DefaultLogger()
	}

	// Use the configured output or default to stdout
	output := config.LogOutput
	if output == nil {
		output = os.Stdout
	}

	// Create handler based on log format
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: convertZapLevelToSlog(config.LogLevel),
	}

	if config.LogFormat == "json" || config.StructuredLogging {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	return NewLogger(handler)
}

// isTestEnvironment checks if we're currently running in a test
func isTestEnvironment() bool {
	// Check for common test environment indicators
	if os.Getenv("GO_TEST") != "" {
		return true
	}

	// Check if any test flags are present in command line args
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			return true
		}
	}

	// Check if the binary name indicates test execution
	for _, arg := range os.Args {
		if strings.Contains(arg, ".test") || strings.HasSuffix(arg, "_test") {
			return true
		}
	}

	// Alternative: always use test-safe logger in go test runs
	// This is a more aggressive approach but should be safer
	return strings.Contains(os.Args[0], ".test") ||
		strings.Contains(os.Args[0], "_test") ||
		len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "-test.")
}

// convertZapLevelToSlog converts a zapcore.Level to slog.Level
func convertZapLevelToSlog(zapLevel zapcore.Level) slog.Level {
	switch zapLevel {
	case zapcore.DebugLevel:
		return slog.LevelDebug
	case zapcore.InfoLevel:
		return slog.LevelInfo
	case zapcore.WarnLevel:
		return slog.LevelWarn
	case zapcore.ErrorLevel:
		return slog.LevelError
	case zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		return slog.LevelError // Map fatal levels to error in slog
	default:
		return slog.LevelInfo // Default to info level
	}
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

// testSafeWriter wraps an io.Writer to handle write-after-close errors in tests
type testSafeWriter struct {
	writer io.Writer
	mu     sync.RWMutex
	closed bool
}

func (w *testSafeWriter) Write(p []byte) (n int, err error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.closed {
		// Silently succeed to prevent write errors
		return len(p), nil
	}

	// Try to write, but catch any errors defensively
	defer func() {
		if r := recover(); r != nil {
			// If writing panics (e.g., due to closed stdout), just silently succeed
			w.mu.RUnlock()
			w.mu.Lock()
			w.closed = true
			w.mu.Unlock()
			w.mu.RLock()
			n = len(p)
			err = nil
		}
	}()

	n, err = w.writer.Write(p)
	if err != nil {
		// If we detect any write error to stdout/stderr, mark as closed and succeed
		// This is more aggressive but prevents the write error messages
		if strings.Contains(err.Error(), "file already closed") ||
			strings.Contains(err.Error(), "closed pipe") ||
			strings.Contains(err.Error(), "broken pipe") ||
			strings.Contains(err.Error(), "bad file descriptor") ||
			strings.Contains(err.Error(), "/dev/stdout") ||
			strings.Contains(err.Error(), "/dev/stderr") {
			w.mu.RUnlock()
			w.mu.Lock()
			w.closed = true
			w.mu.Unlock()
			w.mu.RLock()
			return len(p), nil
		}
	}
	return n, err
}

func (w *testSafeWriter) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
}

// safeWriteFile wraps testSafeWriter to implement os.File interface for stdout/stderr replacement
type safeWriteFile struct {
	*testSafeWriter
}

func (f *safeWriteFile) Fd() uintptr {
	// Return a dummy file descriptor
	return uintptr(1) // stdout fd
}

func (f *safeWriteFile) Name() string {
	return "/dev/stdout"
}

func (f *safeWriteFile) Stat() (os.FileInfo, error) {
	// Return minimal file info
	return &safeFileInfo{name: f.Name()}, nil
}

func (f *safeWriteFile) Sync() error {
	return nil // No-op for safe writer
}

// safeFileInfo provides minimal file info implementation
type safeFileInfo struct {
	name string
}

func (fi *safeFileInfo) Name() string       { return fi.name }
func (fi *safeFileInfo) Size() int64        { return 0 }
func (fi *safeFileInfo) Mode() os.FileMode  { return 0 }
func (fi *safeFileInfo) ModTime() time.Time { return time.Now() }
func (fi *safeFileInfo) IsDir() bool        { return false }
func (fi *safeFileInfo) Sys() interface{}   { return nil }

// TestSafeLogger wraps a Logger to prevent write errors during test cleanup
type TestSafeLogger struct {
	underlying Logger
	mu         sync.RWMutex
	shutdown   bool
}

// NewTestSafeLogger creates a logger that safely handles test cleanup scenarios
func NewTestSafeLogger(handler slog.Handler) Logger {
	if handler == nil {
		// Create a test-safe handler that writes to a test-safe writer
		safeStdout := &testSafeWriter{writer: os.Stdout}
		handler = slog.NewJSONHandler(safeStdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	baseLogger := &structuredLogger{
		logger: slog.New(handler),
		fields: nil,
	}

	testLogger := &TestSafeLogger{
		underlying: baseLogger,
	}

	// Register for shutdown
	registerTestLogger(testLogger)

	return testLogger
}

// Shutdown marks the logger as shut down, preventing further logging
func (t *TestSafeLogger) Shutdown() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.shutdown = true
}

// IsShutdown returns true if the logger has been shut down
func (t *TestSafeLogger) IsShutdown() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.shutdown
}

func (t *TestSafeLogger) Debug(msg string, fields ...Field) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if !t.shutdown {
		t.underlying.Debug(msg, fields...)
	}
}

func (t *TestSafeLogger) Info(msg string, fields ...Field) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if !t.shutdown {
		t.underlying.Info(msg, fields...)
	}
}

func (t *TestSafeLogger) Warn(msg string, fields ...Field) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if !t.shutdown {
		t.underlying.Warn(msg, fields...)
	}
}

func (t *TestSafeLogger) Error(msg string, fields ...Field) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if !t.shutdown {
		t.underlying.Error(msg, fields...)
	}
}

func (t *TestSafeLogger) WithFields(fields ...Field) Logger {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.shutdown {
		return &NoOpLogger{}
	}
	return &TestSafeLogger{
		underlying: t.underlying.WithFields(fields...),
	}
}

func (t *TestSafeLogger) WithContext(ctx context.Context) Logger {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.shutdown {
		return &NoOpLogger{}
	}
	return &TestSafeLogger{
		underlying: t.underlying.WithContext(ctx),
	}
}

// Implement the type-safe methods for TestSafeLogger
func (t *TestSafeLogger) DebugTyped(msg string, fields ...FieldProvider) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if !t.shutdown {
		t.underlying.DebugTyped(msg, fields...)
	}
}

func (t *TestSafeLogger) InfoTyped(msg string, fields ...FieldProvider) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if !t.shutdown {
		t.underlying.InfoTyped(msg, fields...)
	}
}

func (t *TestSafeLogger) WarnTyped(msg string, fields ...FieldProvider) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if !t.shutdown {
		t.underlying.WarnTyped(msg, fields...)
	}
}

func (t *TestSafeLogger) ErrorTyped(msg string, fields ...FieldProvider) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if !t.shutdown {
		t.underlying.ErrorTyped(msg, fields...)
	}
}

func (t *TestSafeLogger) WithTypedFields(fields ...FieldProvider) Logger {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.shutdown {
		return &NoOpLogger{}
	}
	return &TestSafeLogger{
		underlying: t.underlying.WithTypedFields(fields...),
	}
}

// Test helper functions

// NewTestLogger creates a test-safe logger for use in tests
func NewTestLogger() Logger {
	return NewTestSafeLogger(nil)
}

// DefaultTestLogger creates a default test-safe logger
func DefaultTestLogger() Logger {
	return NewTestLogger()
}

// WithTestSafeHandler creates a test-safe logger with a custom handler
func WithTestSafeHandler(handler slog.Handler) Logger {
	return NewTestSafeLogger(handler)
}

// testLoggerRegistry tracks active test loggers for proper shutdown
var testLoggerRegistry struct {
	mu      sync.RWMutex
	loggers []*TestSafeLogger
}

// registerTestLogger adds a logger to the shutdown registry
func registerTestLogger(logger *TestSafeLogger) {
	testLoggerRegistry.mu.Lock()
	defer testLoggerRegistry.mu.Unlock()
	testLoggerRegistry.loggers = append(testLoggerRegistry.loggers, logger)
}

// ShutdownAllTestLoggers shuts down all registered test loggers
func ShutdownAllTestLoggers() {
	testLoggerRegistry.mu.Lock()
	defer testLoggerRegistry.mu.Unlock()

	for _, logger := range testLoggerRegistry.loggers {
		logger.Shutdown()
	}
	// Clear the registry
	testLoggerRegistry.loggers = testLoggerRegistry.loggers[:0]
}
