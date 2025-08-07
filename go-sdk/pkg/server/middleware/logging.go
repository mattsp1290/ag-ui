package middleware

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogLevel represents logging levels
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogFormat represents logging formats
type LogFormat string

const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

// LoggingConfig contains logging middleware configuration
type LoggingConfig struct {
	BaseConfig `json:",inline" yaml:",inline"`

	// Level specifies the minimum log level
	Level LogLevel `json:"level" yaml:"level"`

	// Format specifies the log format
	Format LogFormat `json:"format" yaml:"format"`

	// IncludeRequestBody logs request body for specified methods
	IncludeRequestBody bool     `json:"include_request_body" yaml:"include_request_body"`
	RequestBodyMethods []string `json:"request_body_methods" yaml:"request_body_methods"`
	MaxRequestBodySize int64    `json:"max_request_body_size" yaml:"max_request_body_size"`

	// IncludeResponseBody logs response body
	IncludeResponseBody bool     `json:"include_response_body" yaml:"include_response_body"`
	ResponseBodyMethods []string `json:"response_body_methods" yaml:"response_body_methods"`
	MaxResponseBodySize int64    `json:"max_response_body_size" yaml:"max_response_body_size"`

	// Request/Response headers to log
	IncludeRequestHeaders  []string `json:"include_request_headers" yaml:"include_request_headers"`
	IncludeResponseHeaders []string `json:"include_response_headers" yaml:"include_response_headers"`
	ExcludeHeaders         []string `json:"exclude_headers" yaml:"exclude_headers"`

	// Sensitive data handling
	SanitizeHeaders    []string `json:"sanitize_headers" yaml:"sanitize_headers"`
	SanitizeQueries    []string `json:"sanitize_queries" yaml:"sanitize_queries"`
	SanitizeFormFields []string `json:"sanitize_form_fields" yaml:"sanitize_form_fields"`

	// Request filtering
	ExcludePaths       []string `json:"exclude_paths" yaml:"exclude_paths"`
	ExcludeUserAgents  []string `json:"exclude_user_agents" yaml:"exclude_user_agents"`
	ExcludeStatusCodes []int    `json:"exclude_status_codes" yaml:"exclude_status_codes"`

	// Performance settings
	LogSlowRequests      bool          `json:"log_slow_requests" yaml:"log_slow_requests"`
	SlowRequestThreshold time.Duration `json:"slow_request_threshold" yaml:"slow_request_threshold"`

	// Additional fields
	IncludeClientIP  bool `json:"include_client_ip" yaml:"include_client_ip"`
	IncludeUserAgent bool `json:"include_user_agent" yaml:"include_user_agent"`
	IncludeReferer   bool `json:"include_referer" yaml:"include_referer"`
	IncludeUserID    bool `json:"include_user_id" yaml:"include_user_id"`
	IncludeRequestID bool `json:"include_request_id" yaml:"include_request_id"`
	IncludeTraceID   bool `json:"include_trace_id" yaml:"include_trace_id"`

	// Custom fields to extract from headers or context
	CustomFields map[string]string `json:"custom_fields" yaml:"custom_fields"`
}

// LoggingMiddleware implements structured request/response logging
type LoggingMiddleware struct {
	config *LoggingConfig
	logger *zap.Logger

	// Precomputed maps for performance
	requestBodyMethodMap  map[string]bool
	responseBodyMethodMap map[string]bool
	excludePathMap        map[string]bool
	excludeUserAgentMap   map[string]bool
	excludeStatusCodeMap  map[int]bool
	sanitizeHeaderMap     map[string]bool
	sanitizeQueryMap      map[string]bool
	sanitizeFormFieldMap  map[string]bool
	excludeHeaderMap      map[string]bool
}

// LoggingResponseWriter wraps http.ResponseWriter to capture response data
type LoggingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	responseSize int64
	startTime    time.Time
	bodyBuffer   []byte
	captureBody  bool
	maxBodySize  int64
}

// NewLoggingResponseWriter creates a new logging response writer
func NewLoggingResponseWriter(w http.ResponseWriter, captureBody bool, maxBodySize int64) *LoggingResponseWriter {
	return &LoggingResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		startTime:      time.Now(),
		captureBody:    captureBody,
		maxBodySize:    maxBodySize,
	}
}

// WriteHeader captures the status code
func (lrw *LoggingResponseWriter) WriteHeader(code int) {
	if lrw.statusCode == http.StatusOK {
		lrw.statusCode = code
	}
	lrw.ResponseWriter.WriteHeader(code)
}

// Write captures response data
func (lrw *LoggingResponseWriter) Write(data []byte) (int, error) {
	n, err := lrw.ResponseWriter.Write(data)
	lrw.responseSize += int64(n)

	// Capture response body if enabled
	if lrw.captureBody && len(lrw.bodyBuffer) < int(lrw.maxBodySize) {
		remaining := int(lrw.maxBodySize) - len(lrw.bodyBuffer)
		if remaining > 0 {
			captureSize := n
			if captureSize > remaining {
				captureSize = remaining
			}
			lrw.bodyBuffer = append(lrw.bodyBuffer, data[:captureSize]...)
		}
	}

	return n, err
}

// Hijack implements http.Hijacker interface
func (lrw *LoggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := lrw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not support hijacking")
	}
	return hijacker.Hijack()
}

// Status returns the response status code
func (lrw *LoggingResponseWriter) Status() int {
	return lrw.statusCode
}

// Size returns the response size
func (lrw *LoggingResponseWriter) Size() int64 {
	return lrw.responseSize
}

// Duration returns the response duration
func (lrw *LoggingResponseWriter) Duration() time.Duration {
	return time.Since(lrw.startTime)
}

// Body returns the captured response body
func (lrw *LoggingResponseWriter) Body() []byte {
	return lrw.bodyBuffer
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(config *LoggingConfig, logger *zap.Logger) (*LoggingMiddleware, error) {
	if config == nil {
		return nil, fmt.Errorf("logging config cannot be nil")
	}

	if err := ValidateBaseConfig(&config.BaseConfig); err != nil {
		return nil, fmt.Errorf("invalid base config: %w", err)
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// Set defaults
	if config.Name == "" {
		config.Name = "logging"
	}
	if config.Priority == 0 {
		config.Priority = 10 // Low priority, should run early to catch everything
	}
	if config.Level == "" {
		config.Level = LogLevelInfo
	}
	if config.Format == "" {
		config.Format = LogFormatJSON
	}
	if config.MaxRequestBodySize == 0 {
		config.MaxRequestBodySize = 64 * 1024 // 64KB
	}
	if config.MaxResponseBodySize == 0 {
		config.MaxResponseBodySize = 64 * 1024 // 64KB
	}
	if config.SlowRequestThreshold == 0 {
		config.SlowRequestThreshold = 1 * time.Second
	}
	if len(config.RequestBodyMethods) == 0 {
		config.RequestBodyMethods = []string{"POST", "PUT", "PATCH"}
	}
	if len(config.ResponseBodyMethods) == 0 {
		config.ResponseBodyMethods = []string{"GET", "POST", "PUT", "PATCH"}
	}
	if len(config.SanitizeHeaders) == 0 {
		config.SanitizeHeaders = []string{
			"authorization", "cookie", "set-cookie", "x-api-key",
			"x-auth-token", "x-access-token", "x-csrf-token",
		}
	}

	middleware := &LoggingMiddleware{
		config:                config,
		logger:                logger,
		requestBodyMethodMap:  make(map[string]bool),
		responseBodyMethodMap: make(map[string]bool),
		excludePathMap:        make(map[string]bool),
		excludeUserAgentMap:   make(map[string]bool),
		excludeStatusCodeMap:  make(map[int]bool),
		sanitizeHeaderMap:     make(map[string]bool),
		sanitizeQueryMap:      make(map[string]bool),
		sanitizeFormFieldMap:  make(map[string]bool),
		excludeHeaderMap:      make(map[string]bool),
	}

	// Build maps for performance
	middleware.buildMaps()

	return middleware, nil
}

// Handler implements the Middleware interface
func (lm *LoggingMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !lm.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Check if request should be excluded
		if lm.shouldExcludeRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		startTime := time.Now()

		// Set start time in context
		ctx := SetStartTime(r.Context(), startTime)
		r = r.WithContext(ctx)

		// Determine if we should capture response body
		captureResponseBody := lm.config.IncludeResponseBody &&
			lm.responseBodyMethodMap[r.Method]

		// Create logging response writer
		lrw := NewLoggingResponseWriter(w, captureResponseBody, lm.config.MaxResponseBodySize)

		// Process the request
		next.ServeHTTP(lrw, r)

		// Check if response should be excluded by status code
		if lm.excludeStatusCodeMap[lrw.Status()] {
			return
		}

		// Log the request/response
		lm.logRequest(r, lrw, startTime)
	})
}

// Name returns the middleware name
func (lm *LoggingMiddleware) Name() string {
	return lm.config.Name
}

// Priority returns the middleware priority
func (lm *LoggingMiddleware) Priority() int {
	return lm.config.Priority
}

// Config returns the middleware configuration
func (lm *LoggingMiddleware) Config() interface{} {
	return lm.config
}

// Cleanup performs cleanup
func (lm *LoggingMiddleware) Cleanup() error {
	// Nothing to cleanup for logging middleware
	return nil
}

// shouldExcludeRequest checks if the request should be excluded from logging
func (lm *LoggingMiddleware) shouldExcludeRequest(r *http.Request) bool {
	// Check excluded paths
	if lm.excludePathMap[r.URL.Path] {
		return true
	}

	// Check path prefixes
	for excludePath := range lm.excludePathMap {
		if strings.HasPrefix(r.URL.Path, excludePath) {
			return true
		}
	}

	// Check excluded user agents
	userAgent := r.Header.Get("User-Agent")
	if userAgent != "" && lm.excludeUserAgentMap[userAgent] {
		return true
	}

	return false
}

// logRequest logs the request and response details
func (lm *LoggingMiddleware) logRequest(r *http.Request, lrw *LoggingResponseWriter, startTime time.Time) {
	duration := time.Since(startTime)

	// Determine log level
	level := lm.getLogLevel(lrw.Status(), duration)

	// Build log fields
	fields := lm.buildLogFields(r, lrw, duration)

	// Log based on level
	switch level {
	case zapcore.DebugLevel:
		lm.logger.Debug("HTTP request", fields...)
	case zapcore.InfoLevel:
		lm.logger.Info("HTTP request", fields...)
	case zapcore.WarnLevel:
		lm.logger.Warn("HTTP request", fields...)
	case zapcore.ErrorLevel:
		lm.logger.Error("HTTP request", fields...)
	}
}

// getLogLevel determines the appropriate log level based on status code and duration
func (lm *LoggingMiddleware) getLogLevel(statusCode int, duration time.Duration) zapcore.Level {
	// Error responses
	if statusCode >= 500 {
		return zapcore.ErrorLevel
	}

	// Client errors
	if statusCode >= 400 {
		return zapcore.WarnLevel
	}

	// Slow requests
	if lm.config.LogSlowRequests && duration > lm.config.SlowRequestThreshold {
		return zapcore.WarnLevel
	}

	// Determine level based on config
	switch lm.config.Level {
	case LogLevelDebug:
		return zapcore.DebugLevel
	case LogLevelInfo:
		return zapcore.InfoLevel
	case LogLevelWarn:
		return zapcore.WarnLevel
	case LogLevelError:
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// buildLogFields builds structured log fields for the request
func (lm *LoggingMiddleware) buildLogFields(r *http.Request, lrw *LoggingResponseWriter, duration time.Duration) []zap.Field {
	fields := []zap.Field{
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.Int("status", lrw.Status()),
		zap.Int64("size", lrw.Size()),
		zap.Duration("duration", duration),
		zap.String("protocol", r.Proto),
	}

	// Add query parameters
	if len(r.URL.RawQuery) > 0 {
		fields = append(fields, zap.String("query", lm.sanitizeQuery(r.URL.RawQuery)))
	}

	// Add client IP
	if lm.config.IncludeClientIP {
		fields = append(fields, zap.String("client_ip", GetClientIP(r)))
	}

	// Add user agent
	if lm.config.IncludeUserAgent {
		if userAgent := r.Header.Get("User-Agent"); userAgent != "" {
			fields = append(fields, zap.String("user_agent", userAgent))
		}
	}

	// Add referer
	if lm.config.IncludeReferer {
		if referer := r.Header.Get("Referer"); referer != "" {
			fields = append(fields, zap.String("referer", referer))
		}
	}

	// Add request ID
	if lm.config.IncludeRequestID {
		if requestID := GetRequestID(r.Context()); requestID != "" {
			fields = append(fields, zap.String("request_id", requestID))
		}
	}

	// Add user ID
	if lm.config.IncludeUserID {
		if userID := GetUserID(r.Context()); userID != "" {
			fields = append(fields, zap.String("user_id", userID))
		}
	}

	// Add trace ID
	if lm.config.IncludeTraceID {
		if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
			fields = append(fields, zap.String("trace_id", traceID))
		}
	}

	// Add request headers
	if len(lm.config.IncludeRequestHeaders) > 0 {
		headers := make(map[string]string)
		for _, headerName := range lm.config.IncludeRequestHeaders {
			if !lm.excludeHeaderMap[strings.ToLower(headerName)] {
				if value := r.Header.Get(headerName); value != "" {
					headers[headerName] = lm.sanitizeHeaderValue(headerName, value)
				}
			}
		}
		if len(headers) > 0 {
			fields = append(fields, zap.Any("request_headers", headers))
		}
	}

	// Add response headers
	if len(lm.config.IncludeResponseHeaders) > 0 {
		headers := make(map[string]string)
		for _, headerName := range lm.config.IncludeResponseHeaders {
			if !lm.excludeHeaderMap[strings.ToLower(headerName)] {
				if value := lrw.Header().Get(headerName); value != "" {
					headers[headerName] = lm.sanitizeHeaderValue(headerName, value)
				}
			}
		}
		if len(headers) > 0 {
			fields = append(fields, zap.Any("response_headers", headers))
		}
	}

	// Add request body
	if lm.config.IncludeRequestBody && lm.requestBodyMethodMap[r.Method] {
		if body := lm.extractRequestBody(r); body != "" {
			fields = append(fields, zap.String("request_body", body))
		}
	}

	// Add response body
	if lm.config.IncludeResponseBody && lm.responseBodyMethodMap[r.Method] {
		if body := lrw.Body(); len(body) > 0 {
			fields = append(fields, zap.String("response_body", string(body)))
		}
	}

	// Add custom fields
	for fieldName, headerName := range lm.config.CustomFields {
		if value := r.Header.Get(headerName); value != "" {
			fields = append(fields, zap.String(fieldName, value))
		}
	}

	return fields
}

// extractRequestBody safely extracts request body
func (lm *LoggingMiddleware) extractRequestBody(r *http.Request) string {
	// This is a simplified implementation
	// In practice, you'd need to read the body without consuming it
	// or use a request body capture mechanism

	if r.ContentLength == 0 {
		return ""
	}

	if r.ContentLength > lm.config.MaxRequestBodySize {
		return fmt.Sprintf("[body too large: %d bytes]", r.ContentLength)
	}

	// For demonstration purposes, return placeholder
	// Real implementation would need careful body handling
	return "[request body capture not fully implemented]"
}

// sanitizeHeaderValue sanitizes sensitive header values
func (lm *LoggingMiddleware) sanitizeHeaderValue(headerName, value string) string {
	if lm.sanitizeHeaderMap[strings.ToLower(headerName)] {
		if len(value) <= 8 {
			return "[REDACTED]"
		}
		return value[:4] + "[REDACTED]" + value[len(value)-4:]
	}
	return value
}

// sanitizeQuery sanitizes sensitive query parameters
func (lm *LoggingMiddleware) sanitizeQuery(query string) string {
	if len(lm.config.SanitizeQueries) == 0 {
		return query
	}

	// Simple implementation - in practice, you'd parse and sanitize individually
	for _, sensitiveParam := range lm.config.SanitizeQueries {
		if strings.Contains(query, sensitiveParam+"=") {
			// Replace the value part with [REDACTED]
			parts := strings.Split(query, "&")
			for i, part := range parts {
				if strings.HasPrefix(part, sensitiveParam+"=") {
					parts[i] = sensitiveParam + "=[REDACTED]"
				}
			}
			query = strings.Join(parts, "&")
		}
	}

	return query
}

// buildMaps precomputes maps for performance
func (lm *LoggingMiddleware) buildMaps() {
	// Build request body method map
	for _, method := range lm.config.RequestBodyMethods {
		lm.requestBodyMethodMap[strings.ToUpper(method)] = true
	}

	// Build response body method map
	for _, method := range lm.config.ResponseBodyMethods {
		lm.responseBodyMethodMap[strings.ToUpper(method)] = true
	}

	// Build exclude path map
	for _, path := range lm.config.ExcludePaths {
		lm.excludePathMap[path] = true
	}

	// Build exclude user agent map
	for _, userAgent := range lm.config.ExcludeUserAgents {
		lm.excludeUserAgentMap[userAgent] = true
	}

	// Build exclude status code map
	for _, statusCode := range lm.config.ExcludeStatusCodes {
		lm.excludeStatusCodeMap[statusCode] = true
	}

	// Build sanitize header map
	for _, header := range lm.config.SanitizeHeaders {
		lm.sanitizeHeaderMap[strings.ToLower(header)] = true
	}

	// Build sanitize query map
	for _, query := range lm.config.SanitizeQueries {
		lm.sanitizeQueryMap[strings.ToLower(query)] = true
	}

	// Build sanitize form field map
	for _, field := range lm.config.SanitizeFormFields {
		lm.sanitizeFormFieldMap[strings.ToLower(field)] = true
	}

	// Build exclude header map
	for _, header := range lm.config.ExcludeHeaders {
		lm.excludeHeaderMap[strings.ToLower(header)] = true
	}
}

// Default configurations

// DefaultLoggingConfig returns a default logging configuration
func DefaultLoggingConfig() *LoggingConfig {
	return &LoggingConfig{
		BaseConfig: BaseConfig{
			Enabled:  true,
			Priority: 10,
			Name:     "logging",
		},
		Level:                  LogLevelInfo,
		Format:                 LogFormatJSON,
		IncludeRequestBody:     false,
		RequestBodyMethods:     []string{"POST", "PUT", "PATCH"},
		MaxRequestBodySize:     64 * 1024,
		IncludeResponseBody:    false,
		ResponseBodyMethods:    []string{"GET", "POST", "PUT", "PATCH"},
		MaxResponseBodySize:    64 * 1024,
		IncludeRequestHeaders:  []string{},
		IncludeResponseHeaders: []string{},
		ExcludeHeaders:         []string{"cookie", "set-cookie"},
		SanitizeHeaders:        []string{"authorization", "x-api-key", "cookie", "set-cookie"},
		SanitizeQueries:        []string{"token", "key", "secret", "password"},
		SanitizeFormFields:     []string{"password", "secret", "token"},
		ExcludePaths:           []string{"/health", "/metrics", "/favicon.ico"},
		ExcludeUserAgents:      []string{},
		ExcludeStatusCodes:     []int{},
		LogSlowRequests:        true,
		SlowRequestThreshold:   1 * time.Second,
		IncludeClientIP:        true,
		IncludeUserAgent:       true,
		IncludeReferer:         false,
		IncludeUserID:          true,
		IncludeRequestID:       true,
		IncludeTraceID:         true,
		CustomFields:           make(map[string]string),
	}
}

// DebugLoggingConfig returns a debug logging configuration
func DebugLoggingConfig() *LoggingConfig {
	config := DefaultLoggingConfig()
	config.Level = LogLevelDebug
	config.IncludeRequestBody = true
	config.IncludeResponseBody = true
	config.IncludeReferer = true
	config.IncludeRequestHeaders = []string{"Content-Type", "Accept", "User-Agent"}
	config.IncludeResponseHeaders = []string{"Content-Type", "Content-Length"}
	return config
}

// ProductionLoggingConfig returns a production-safe logging configuration
func ProductionLoggingConfig() *LoggingConfig {
	config := DefaultLoggingConfig()
	config.Level = LogLevelInfo
	config.IncludeRequestBody = false
	config.IncludeResponseBody = false
	config.ExcludePaths = []string{"/health", "/ready", "/metrics", "/favicon.ico", "/robots.txt"}
	config.ExcludeStatusCodes = []int{200, 204, 304} // Exclude successful responses to reduce noise
	return config
}

// AccessLogMiddleware creates a simple access log middleware
func AccessLogMiddleware(logger *zap.Logger) (*LoggingMiddleware, error) {
	config := &LoggingConfig{
		BaseConfig: BaseConfig{
			Enabled:  true,
			Priority: 10,
			Name:     "access-log",
		},
		Level:              LogLevelInfo,
		Format:             LogFormatText,
		IncludeClientIP:    true,
		IncludeUserAgent:   false,
		IncludeRequestID:   true,
		ExcludePaths:       []string{"/health", "/metrics"},
		ExcludeStatusCodes: []int{200},
	}

	return NewLoggingMiddleware(config, logger)
}

// RequestIDMiddleware creates middleware that adds request ID to each request
func RequestIDMiddleware(logger *zap.Logger) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := GenerateRequestID()

			// Set request ID in context
			ctx := SetRequestID(r.Context(), requestID)

			// Set request ID in response header
			w.Header().Set("X-Request-ID", requestID)

			// Continue with the request
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
