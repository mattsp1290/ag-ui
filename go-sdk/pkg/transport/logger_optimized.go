package transport

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"sync"
	"time"
)

// Pre-allocated byte slices for common log levels
var (
	debugBytes = []byte("[DEBUG] ")
	infoBytes  = []byte("[INFO] ")
	warnBytes  = []byte("[WARN] ")
	errorBytes = []byte("[ERROR] ")
)

// Buffer pool for log formatting
var logBufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// OptimizedLogger implements Logger with reduced allocations
type OptimizedLogger struct {
	config *LoggerConfig
	fields []Field

	// Pre-allocated buffers
	timestampBuf []byte
	fieldsBuf    []byte

	// Cache formatted timestamps
	lastTimestamp    time.Time
	lastTimestampStr string
	timestampMutex   sync.RWMutex
}

// NewOptimizedLogger creates a new optimized logger
func NewOptimizedLogger(config *LoggerConfig) Logger {
	if config == nil {
		config = DefaultLoggerConfig()
	}

	return &OptimizedLogger{
		config:       config,
		fields:       make([]Field, 0),
		timestampBuf: make([]byte, 0, 64),
		fieldsBuf:    make([]byte, 0, 256),
	}
}

// formatTimestamp formats timestamp with caching
func (l *OptimizedLogger) formatTimestamp(t time.Time) string {
	// Check cache first
	l.timestampMutex.RLock()
	if t.Unix() == l.lastTimestamp.Unix() {
		ts := l.lastTimestampStr
		l.timestampMutex.RUnlock()
		return ts
	}
	l.timestampMutex.RUnlock()

	// Format new timestamp
	l.timestampMutex.Lock()
	defer l.timestampMutex.Unlock()

	// Double-check after acquiring write lock
	if t.Unix() == l.lastTimestamp.Unix() {
		return l.lastTimestampStr
	}

	l.lastTimestamp = t
	l.lastTimestampStr = t.Format(l.config.TimestampFormat)
	return l.lastTimestampStr
}

// Log implements the Logger interface with optimizations
func (l *OptimizedLogger) Log(level LogLevel, message string, fields ...Field) {
	if level < l.config.Level {
		return
	}

	// Get buffer from pool
	buf := logBufferPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		logBufferPool.Put(buf)
	}()

	// Format timestamp
	timestamp := l.formatTimestamp(time.Now())

	// Build log message efficiently
	buf.WriteByte('[')
	buf.WriteString(timestamp)
	buf.WriteString("] ")

	// Write level without allocation
	switch level {
	case LogLevelDebug:
		buf.Write(debugBytes[1:7]) // Skip opening bracket
	case LogLevelInfo:
		buf.Write(infoBytes[1:6])
	case LogLevelWarn:
		buf.Write(warnBytes[1:6])
	case LogLevelError:
		buf.Write(errorBytes[1:7])
	}

	buf.WriteByte(' ')
	buf.WriteString(message)

	// Add fields efficiently
	l.writeFields(buf, fields...)

	// Output the log
	l.output(buf.Bytes())
}

// writeFields writes fields to buffer efficiently
func (l *OptimizedLogger) writeFields(buf *bytes.Buffer, fields ...Field) {
	// Add pre-populated fields
	for _, field := range l.fields {
		l.writeField(buf, field)
	}

	// Add new fields
	for _, field := range fields {
		l.writeField(buf, field)
	}
}

// writeField writes a single field efficiently
func (l *OptimizedLogger) writeField(buf *bytes.Buffer, field Field) {
	buf.WriteByte(' ')
	buf.WriteString(field.Key)
	buf.WriteByte('=')

	// Type switch for common types to avoid reflection
	switch v := field.Value.(type) {
	case string:
		buf.WriteString(v)
	case int:
		buf.WriteString(strconv.Itoa(v))
	case int64:
		buf.WriteString(strconv.FormatInt(v, 10))
	case float64:
		buf.WriteString(strconv.FormatFloat(v, 'g', -1, 64))
	case bool:
		if v {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case time.Duration:
		buf.WriteString(v.String())
	case time.Time:
		buf.WriteString(v.Format(time.RFC3339))
	case error:
		buf.WriteString(v.Error())
	default:
		// Fall back to slower formatting for other types
		buf.WriteString(formatValue(v))
	}
}

// formatValue formats a value as string (fallback for uncommon types)
func formatValue(v interface{}) string {
	// This is the slow path - only used for uncommon types
	if v == nil {
		return "<nil>"
	}
	// Use a simple type assertion chain for other common types
	switch val := v.(type) {
	case []byte:
		return string(val)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	default:
		// Last resort - this allocates
		return "<unsupported>"
	}
}

// output writes the formatted log
func (l *OptimizedLogger) output(data []byte) {
	// In production, this would write to the configured output
	// For now, just ensure it compiles
	_, _ = l.config.Output.Write(data)
	_, _ = l.config.Output.Write([]byte{'\n'})
}

// Debug implements the Logger interface
func (l *OptimizedLogger) Debug(message string, fields ...Field) {
	if LogLevelDebug < l.config.Level {
		return
	}
	l.Log(LogLevelDebug, message, fields...)
}

// Info implements the Logger interface
func (l *OptimizedLogger) Info(message string, fields ...Field) {
	if LogLevelInfo < l.config.Level {
		return
	}
	l.Log(LogLevelInfo, message, fields...)
}

// Warn implements the Logger interface
func (l *OptimizedLogger) Warn(message string, fields ...Field) {
	if LogLevelWarn < l.config.Level {
		return
	}
	l.Log(LogLevelWarn, message, fields...)
}

// Error implements the Logger interface
func (l *OptimizedLogger) Error(message string, fields ...Field) {
	l.Log(LogLevelError, message, fields...)
}

// WithFields implements the Logger interface
func (l *OptimizedLogger) WithFields(fields ...Field) Logger {
	// Pre-allocate exact capacity
	newFields := make([]Field, len(l.fields)+len(fields))
	copy(newFields, l.fields)
	copy(newFields[len(l.fields):], fields)

	return &OptimizedLogger{
		config: l.config,
		fields: newFields,
	}
}

// WithContext implements the Logger interface
func (l *OptimizedLogger) WithContext(ctx context.Context) Logger {
	return l
}

// Type-safe methods implementation
func (l *OptimizedLogger) LogTyped(level LogLevel, message string, fields ...FieldProvider) {
	if level < l.config.Level {
		return
	}

	// Convert to Field slice efficiently
	convertedFields := make([]Field, len(fields))
	for i, fp := range fields {
		convertedFields[i] = fp.ToField()
	}

	l.Log(level, message, convertedFields...)
}

func (l *OptimizedLogger) DebugTyped(message string, fields ...FieldProvider) {
	l.LogTyped(LogLevelDebug, message, fields...)
}

func (l *OptimizedLogger) InfoTyped(message string, fields ...FieldProvider) {
	l.LogTyped(LogLevelInfo, message, fields...)
}

func (l *OptimizedLogger) WarnTyped(message string, fields ...FieldProvider) {
	l.LogTyped(LogLevelWarn, message, fields...)
}

func (l *OptimizedLogger) ErrorTyped(message string, fields ...FieldProvider) {
	l.LogTyped(LogLevelError, message, fields...)
}

func (l *OptimizedLogger) WithTypedFields(fields ...FieldProvider) Logger {
	convertedFields := make([]Field, len(fields))
	for i, fp := range fields {
		convertedFields[i] = fp.ToField()
	}
	return l.WithFields(convertedFields...)
}

// FastLogger provides zero-allocation logging for hot paths
type FastLogger struct {
	w io.Writer

	// Pre-allocated buffers
	buf      []byte
	scratch  []byte
	levelBuf []byte
}

// NewFastLogger creates a logger optimized for hot paths
func NewFastLogger(w io.Writer) *FastLogger {
	return &FastLogger{
		w:        w,
		buf:      make([]byte, 0, 1024),
		scratch:  make([]byte, 0, 64),
		levelBuf: make([]byte, 0, 10),
	}
}

// LogFast logs a message with minimal allocations
func (fl *FastLogger) LogFast(level LogLevel, message string) {
	fl.buf = fl.buf[:0]

	// Timestamp (simplified - just unix timestamp)
	fl.buf = append(fl.buf, '[')
	fl.buf = strconv.AppendInt(fl.buf, time.Now().Unix(), 10)
	fl.buf = append(fl.buf, "] "...)

	// Level
	switch level {
	case LogLevelDebug:
		fl.buf = append(fl.buf, "DEBUG "...)
	case LogLevelInfo:
		fl.buf = append(fl.buf, "INFO "...)
	case LogLevelWarn:
		fl.buf = append(fl.buf, "WARN "...)
	case LogLevelError:
		fl.buf = append(fl.buf, "ERROR "...)
	}

	// Message
	fl.buf = append(fl.buf, message...)
	fl.buf = append(fl.buf, '\n')

	// Write (ignore errors in hot path)
	_, _ = fl.w.Write(fl.buf)
}
