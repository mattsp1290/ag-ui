package observability

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// LogLevel represents logging levels
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

// String returns the string representation of LogLevel
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
	case LogLevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogFormat represents log output formats
type LogFormat int

const (
	LogFormatText LogFormat = iota
	LogFormatJSON
	LogFormatStructured
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Level         LogLevel               `json:"level"`
	Message       string                 `json:"message"`
	Timestamp     time.Time              `json:"timestamp"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	RequestID     string                 `json:"request_id,omitempty"`
	UserID        string                 `json:"user_id,omitempty"`
	Method        string                 `json:"method,omitempty"`
	Path          string                 `json:"path,omitempty"`
	StatusCode    int                    `json:"status_code,omitempty"`
	Duration      time.Duration          `json:"duration,omitempty"`
	RequestSize   int64                  `json:"request_size,omitempty"`
	ResponseSize  int64                  `json:"response_size,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Fields        map[string]interface{} `json:"fields,omitempty"`
}

// Logger interface for structured logging
type Logger interface {
	// Log writes a log entry
	Log(entry *LogEntry)

	// Debug logs debug level message
	Debug(msg string, fields map[string]interface{})

	// Info logs info level message
	Info(msg string, fields map[string]interface{})

	// Warn logs warning level message
	Warn(msg string, fields map[string]interface{})

	// Error logs error level message
	Error(msg string, err error, fields map[string]interface{})

	// Fatal logs fatal level message
	Fatal(msg string, err error, fields map[string]interface{})

	// WithCorrelationID returns a logger with correlation ID
	WithCorrelationID(correlationID string) Logger

	// WithRequestID returns a logger with request ID
	WithRequestID(requestID string) Logger

	// WithFields returns a logger with additional fields
	WithFields(fields map[string]interface{}) Logger
}

// StructuredLogger implements Logger interface with structured logging
type StructuredLogger struct {
	level         LogLevel
	format        LogFormat
	correlationID string
	requestID     string
	baseFields    map[string]interface{}
	slogger       *slog.Logger
	mu            sync.RWMutex
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(level LogLevel, format LogFormat) *StructuredLogger {
	var handler slog.Handler

	switch format {
	case LogFormatJSON:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	default:
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	return &StructuredLogger{
		level:      level,
		format:     format,
		baseFields: make(map[string]interface{}),
		slogger:    slog.New(handler),
	}
}

// Log writes a log entry
func (l *StructuredLogger) Log(entry *LogEntry) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if entry.Level < l.level {
		return
	}

	// Convert to slog level
	var slogLevel slog.Level
	switch entry.Level {
	case LogLevelDebug:
		slogLevel = slog.LevelDebug
	case LogLevelInfo:
		slogLevel = slog.LevelInfo
	case LogLevelWarn:
		slogLevel = slog.LevelWarn
	case LogLevelError, LogLevelFatal:
		slogLevel = slog.LevelError
	}

	// Build attributes
	attrs := []slog.Attr{}

	if entry.CorrelationID != "" {
		attrs = append(attrs, slog.String("correlation_id", entry.CorrelationID))
	}

	if entry.RequestID != "" {
		attrs = append(attrs, slog.String("request_id", entry.RequestID))
	}

	if entry.UserID != "" {
		attrs = append(attrs, slog.String("user_id", entry.UserID))
	}

	if entry.Method != "" {
		attrs = append(attrs, slog.String("method", entry.Method))
	}

	if entry.Path != "" {
		attrs = append(attrs, slog.String("path", entry.Path))
	}

	if entry.StatusCode != 0 {
		attrs = append(attrs, slog.Int("status_code", entry.StatusCode))
	}

	if entry.Duration > 0 {
		attrs = append(attrs, slog.Duration("duration", entry.Duration))
	}

	if entry.RequestSize > 0 {
		attrs = append(attrs, slog.Int64("request_size", entry.RequestSize))
	}

	if entry.ResponseSize > 0 {
		attrs = append(attrs, slog.Int64("response_size", entry.ResponseSize))
	}

	if entry.Error != "" {
		attrs = append(attrs, slog.String("error", entry.Error))
	}

	// Add custom fields
	for k, v := range entry.Fields {
		attrs = append(attrs, slog.Any(k, v))
	}

	// Add base fields
	for k, v := range l.baseFields {
		attrs = append(attrs, slog.Any(k, v))
	}

	l.slogger.LogAttrs(context.Background(), slogLevel, entry.Message, attrs...)
}

// Debug logs debug level message
func (l *StructuredLogger) Debug(msg string, fields map[string]interface{}) {
	l.Log(&LogEntry{
		Level:         LogLevelDebug,
		Message:       msg,
		Timestamp:     time.Now(),
		CorrelationID: l.correlationID,
		RequestID:     l.requestID,
		Fields:        fields,
	})
}

// Info logs info level message
func (l *StructuredLogger) Info(msg string, fields map[string]interface{}) {
	l.Log(&LogEntry{
		Level:         LogLevelInfo,
		Message:       msg,
		Timestamp:     time.Now(),
		CorrelationID: l.correlationID,
		RequestID:     l.requestID,
		Fields:        fields,
	})
}

// Warn logs warning level message
func (l *StructuredLogger) Warn(msg string, fields map[string]interface{}) {
	l.Log(&LogEntry{
		Level:         LogLevelWarn,
		Message:       msg,
		Timestamp:     time.Now(),
		CorrelationID: l.correlationID,
		RequestID:     l.requestID,
		Fields:        fields,
	})
}

// Error logs error level message
func (l *StructuredLogger) Error(msg string, err error, fields map[string]interface{}) {
	entry := &LogEntry{
		Level:         LogLevelError,
		Message:       msg,
		Timestamp:     time.Now(),
		CorrelationID: l.correlationID,
		RequestID:     l.requestID,
		Fields:        fields,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	l.Log(entry)
}

// Fatal logs fatal level message
func (l *StructuredLogger) Fatal(msg string, err error, fields map[string]interface{}) {
	entry := &LogEntry{
		Level:         LogLevelFatal,
		Message:       msg,
		Timestamp:     time.Now(),
		CorrelationID: l.correlationID,
		RequestID:     l.requestID,
		Fields:        fields,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	l.Log(entry)
}

// WithCorrelationID returns a logger with correlation ID
func (l *StructuredLogger) WithCorrelationID(correlationID string) Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	newLogger := &StructuredLogger{
		level:         l.level,
		format:        l.format,
		correlationID: correlationID,
		requestID:     l.requestID,
		baseFields:    l.copyFields(),
		slogger:       l.slogger,
		mu:            sync.RWMutex{},
	}
	return newLogger
}

// WithRequestID returns a logger with request ID
func (l *StructuredLogger) WithRequestID(requestID string) Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	newLogger := &StructuredLogger{
		level:         l.level,
		format:        l.format,
		correlationID: l.correlationID,
		requestID:     requestID,
		baseFields:    l.copyFields(),
		slogger:       l.slogger,
		mu:            sync.RWMutex{},
	}
	return newLogger
}

// WithFields returns a logger with additional fields
func (l *StructuredLogger) WithFields(fields map[string]interface{}) Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Copy existing fields and add new ones
	newFields := make(map[string]interface{})
	for k, v := range l.baseFields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}

	newLogger := &StructuredLogger{
		level:         l.level,
		format:        l.format,
		correlationID: l.correlationID,
		requestID:     l.requestID,
		baseFields:    newFields,
		slogger:       l.slogger,
		mu:            sync.RWMutex{},
	}
	return newLogger
}

// copyFields creates a deep copy of the baseFields map
func (l *StructuredLogger) copyFields() map[string]interface{} {
	if l.baseFields == nil {
		return nil
	}
	copied := make(map[string]interface{}, len(l.baseFields))
	for k, v := range l.baseFields {
		copied[k] = v
	}
	return copied
}

// LoggingConfig represents logging middleware configuration
type LoggingConfig struct {
	Level             LogLevel  `json:"level" yaml:"level"`
	Format            LogFormat `json:"format" yaml:"format"`
	EnableCorrelation bool      `json:"enable_correlation" yaml:"enable_correlation"`
	LogRequestBody    bool      `json:"log_request_body" yaml:"log_request_body"`
	LogResponseBody   bool      `json:"log_response_body" yaml:"log_response_body"`
	MaxBodySize       int64     `json:"max_body_size" yaml:"max_body_size"`
	SkipPaths         []string  `json:"skip_paths" yaml:"skip_paths"`
	SkipHealthCheck   bool      `json:"skip_health_check" yaml:"skip_health_check"`
}

// LoggingMiddleware implements structured logging middleware
type LoggingMiddleware struct {
	config   *LoggingConfig
	logger   Logger
	enabled  bool
	priority int
	skipMap  map[string]bool
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(config *LoggingConfig) *LoggingMiddleware {
	if config == nil {
		config = &LoggingConfig{
			Level:             LogLevelInfo,
			Format:            LogFormatJSON,
			EnableCorrelation: true,
			MaxBodySize:       1024 * 1024, // 1MB
			SkipHealthCheck:   true,
		}
	}

	logger := NewStructuredLogger(config.Level, config.Format)

	skipMap := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipMap[path] = true
	}

	// Add common health check paths
	if config.SkipHealthCheck {
		skipMap["/health"] = true
		skipMap["/healthz"] = true
		skipMap["/ping"] = true
		skipMap["/ready"] = true
		skipMap["/live"] = true
	}

	return &LoggingMiddleware{
		config:   config,
		logger:   logger,
		enabled:  true,
		priority: 10, // Low priority, should run early but after auth
		skipMap:  skipMap,
	}
}

// Name returns middleware name
func (l *LoggingMiddleware) Name() string {
	return "logging"
}

// Process processes the request through logging middleware
func (l *LoggingMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	startTime := time.Now()

	// Skip logging for configured paths
	if l.skipMap[req.Path] {
		return next(ctx, req)
	}

	// Generate correlation ID if enabled
	correlationID := ""
	if l.config.EnableCorrelation {
		if existing, ok := req.Metadata["correlation_id"].(string); ok && existing != "" {
			correlationID = existing
		} else {
			correlationID = uuid.New().String()
			if req.Metadata == nil {
				req.Metadata = make(map[string]interface{})
			}
			req.Metadata["correlation_id"] = correlationID
		}
	}

	// Create logger with correlation and request IDs
	reqLogger := l.logger
	if correlationID != "" {
		reqLogger = reqLogger.WithCorrelationID(correlationID)
	}
	if req.ID != "" {
		reqLogger = reqLogger.WithRequestID(req.ID)
	}

	// Extract user ID from auth context if available
	userID := ""
	if authCtx, ok := req.Metadata["auth_context"]; ok {
		if auth, ok := authCtx.(map[string]interface{}); ok {
			if uid, ok := auth["user_id"].(string); ok {
				userID = uid
			}
		}
	}

	// Log request start
	reqFields := map[string]interface{}{
		"method": req.Method,
		"path":   req.Path,
	}

	if userID != "" {
		reqFields["user_id"] = userID
	}

	if len(req.Headers) > 0 {
		reqFields["headers"] = l.sanitizeHeaders(req.Headers)
	}

	// Log request body if enabled and not too large
	if l.config.LogRequestBody && req.Body != nil {
		if bodyBytes, err := json.Marshal(req.Body); err == nil {
			if int64(len(bodyBytes)) <= l.config.MaxBodySize {
				reqFields["request_body"] = string(bodyBytes)
			} else {
				reqFields["request_body"] = "[BODY TOO LARGE]"
			}
			reqFields["request_size"] = len(bodyBytes)
		}
	}

	reqLogger.Info("Request started", reqFields)

	// Process request through next middleware
	resp, err := next(ctx, req)

	duration := time.Since(startTime)

	// Log response
	respFields := map[string]interface{}{
		"method":   req.Method,
		"path":     req.Path,
		"duration": duration,
	}

	if userID != "" {
		respFields["user_id"] = userID
	}

	if resp != nil {
		respFields["status_code"] = resp.StatusCode

		if len(resp.Headers) > 0 {
			respFields["response_headers"] = l.sanitizeHeaders(resp.Headers)
		}

		// Log response body if enabled and not too large
		if l.config.LogResponseBody && resp.Body != nil {
			if bodyBytes, err := json.Marshal(resp.Body); err == nil {
				if int64(len(bodyBytes)) <= l.config.MaxBodySize {
					respFields["response_body"] = string(bodyBytes)
				} else {
					respFields["response_body"] = "[BODY TOO LARGE]"
				}
				respFields["response_size"] = len(bodyBytes)
			}
		}
	}

	if err != nil {
		respFields["error"] = err.Error()
		reqLogger.Error("Request failed", err, respFields)
	} else {
		reqLogger.Info("Request completed", respFields)
	}

	return resp, err
}

// Configure configures the middleware
func (l *LoggingMiddleware) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		l.enabled = enabled
	}

	if priority, ok := config["priority"].(int); ok {
		l.priority = priority
	}

	if level, ok := config["level"].(string); ok {
		switch strings.ToUpper(level) {
		case "DEBUG":
			l.config.Level = LogLevelDebug
		case "INFO":
			l.config.Level = LogLevelInfo
		case "WARN":
			l.config.Level = LogLevelWarn
		case "ERROR":
			l.config.Level = LogLevelError
		case "FATAL":
			l.config.Level = LogLevelFatal
		}
	}

	if format, ok := config["format"].(string); ok {
		switch strings.ToUpper(format) {
		case "JSON":
			l.config.Format = LogFormatJSON
		case "TEXT":
			l.config.Format = LogFormatText
		case "STRUCTURED":
			l.config.Format = LogFormatStructured
		}
	}

	if logRequestBody, ok := config["log_request_body"].(bool); ok {
		l.config.LogRequestBody = logRequestBody
	}

	if logResponseBody, ok := config["log_response_body"].(bool); ok {
		l.config.LogResponseBody = logResponseBody
	}

	return nil
}

// Enabled returns whether the middleware is enabled
func (l *LoggingMiddleware) Enabled() bool {
	return l.enabled
}

// Priority returns the middleware priority
func (l *LoggingMiddleware) Priority() int {
	return l.priority
}

// sanitizeHeaders removes sensitive headers from logging
func (l *LoggingMiddleware) sanitizeHeaders(headers map[string]string) map[string]string {
	sanitized := make(map[string]string)
	sensitiveHeaders := map[string]bool{
		"authorization": true,
		"cookie":        true,
		"set-cookie":    true,
		"x-api-key":     true,
		"x-auth-token":  true,
	}

	for k, v := range headers {
		key := strings.ToLower(k)
		if sensitiveHeaders[key] {
			sanitized[k] = "[REDACTED]"
		} else {
			sanitized[k] = v
		}
	}

	return sanitized
}

// CorrelationIDMiddleware adds correlation IDs to requests
type CorrelationIDMiddleware struct {
	headerName string
	enabled    bool
	priority   int
}

// NewCorrelationIDMiddleware creates a new correlation ID middleware
func NewCorrelationIDMiddleware(headerName string) *CorrelationIDMiddleware {
	if headerName == "" {
		headerName = "X-Correlation-ID"
	}

	return &CorrelationIDMiddleware{
		headerName: headerName,
		enabled:    true,
		priority:   1000, // Very high priority, should run first
	}
}

// Name returns middleware name
func (c *CorrelationIDMiddleware) Name() string {
	return "correlation_id"
}

// Process processes the request through correlation ID middleware
func (c *CorrelationIDMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Check if correlation ID already exists
	correlationID := ""
	if req.Headers != nil {
		if existing, ok := req.Headers[c.headerName]; ok && existing != "" {
			correlationID = existing
		}
	}

	// Generate new correlation ID if not present
	if correlationID == "" {
		correlationID = uuid.New().String()
		if req.Headers == nil {
			req.Headers = make(map[string]string)
		}
		req.Headers[c.headerName] = correlationID
	}

	// Add to request metadata
	if req.Metadata == nil {
		req.Metadata = make(map[string]interface{})
	}
	req.Metadata["correlation_id"] = correlationID

	// Process request
	resp, err := next(ctx, req)

	// Add correlation ID to response headers
	if resp != nil {
		if resp.Headers == nil {
			resp.Headers = make(map[string]string)
		}
		resp.Headers[c.headerName] = correlationID
	}

	return resp, err
}

// Configure configures the middleware
func (c *CorrelationIDMiddleware) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		c.enabled = enabled
	}

	if priority, ok := config["priority"].(int); ok {
		c.priority = priority
	}

	if headerName, ok := config["header_name"].(string); ok && headerName != "" {
		c.headerName = headerName
	}

	return nil
}

// Enabled returns whether the middleware is enabled
func (c *CorrelationIDMiddleware) Enabled() bool {
	return c.enabled
}

// Priority returns the middleware priority
func (c *CorrelationIDMiddleware) Priority() int {
	return c.priority
}