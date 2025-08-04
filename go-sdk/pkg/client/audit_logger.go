package client

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// AuditLogger handles security audit logging
type AuditLogger struct {
	config     *AuditLoggingConfig
	logger     *zap.Logger
	fileLogger *zap.Logger
	mu         sync.RWMutex
	logFile    *os.File
	rotator    *LogRotator
}

// LogRotator handles log file rotation
type LogRotator struct {
	config     *AuditLoggingConfig
	logger     *zap.Logger
	currentSize int64
	mu         sync.Mutex
}

// AuditLogEntry represents a structured audit log entry
type AuditLogEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	Level       string                 `json:"level"`
	EventType   string                 `json:"event_type"`
	UserID      string                 `json:"user_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	Resource    string                 `json:"resource,omitempty"`
	Action      string                 `json:"action,omitempty"`
	Result      string                 `json:"result"`
	Error       string                 `json:"error,omitempty"`
	Duration    string                 `json:"duration,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Sensitive   bool                   `json:"sensitive,omitempty"`
	TraceID     string                 `json:"trace_id,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(config *AuditLoggingConfig, logger *zap.Logger) (*AuditLogger, error) {
	if config == nil {
		return nil, fmt.Errorf("audit logging config cannot be nil")
	}
	
	if logger == nil {
		logger = zap.NewNop()
	}
	
	al := &AuditLogger{
		config: config,
		logger: logger,
	}
	
	// Initialize file logging if enabled
	if config.Enabled && config.LogFile != "" {
		if err := al.initializeFileLogging(); err != nil {
			return nil, fmt.Errorf("failed to initialize file logging: %w", err)
		}
	}
	
	return al, nil
}

// initializeFileLogging sets up file-based audit logging
func (al *AuditLogger) initializeFileLogging() error {
	// Create log directory if it doesn't exist
	logDir := filepath.Dir(al.config.LogFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	
	// Open log file
	var err error
	al.logFile, err = os.OpenFile(al.config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	
	// Get current file size for rotation
	if stat, err := al.logFile.Stat(); err == nil {
		al.rotator = &LogRotator{
			config:      al.config,
			logger:      al.logger,
			currentSize: stat.Size(),
		}
	}
	
	// Create file logger
	al.fileLogger = al.createFileLogger(al.logFile)
	
	al.logger.Info("Initialized audit file logging",
		zap.String("log_file", al.config.LogFile),
		zap.String("format", al.config.LogFormat))
	
	return nil
}

// createFileLogger creates a zap logger for file output
func (al *AuditLogger) createFileLogger(writer io.Writer) *zap.Logger {
	// Configure encoder based on format
	var encoder zapcore.Encoder
	if strings.ToLower(al.config.LogFormat) == "json" {
		encoder = zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		})
	} else {
		encoder = zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		})
	}
	
	// Parse log level
	level := zapcore.InfoLevel
	if err := level.UnmarshalText([]byte(al.config.LogLevel)); err != nil {
		al.logger.Warn("Invalid log level, using info", zap.String("level", al.config.LogLevel))
	}
	
	// Create core
	core := zapcore.NewCore(encoder, zapcore.AddSync(writer), level)
	
	return zap.New(core)
}

// LogEvent logs a security event
func (al *AuditLogger) LogEvent(event *SecurityEvent) {
	if !al.config.Enabled {
		return
	}
	
	// Check if this event type should be logged
	if !al.shouldLogEventType(event.Type) {
		return
	}
	
	// Create audit log entry
	entry := &AuditLogEntry{
		Timestamp: event.Timestamp,
		Level:     al.determineLogLevel(event),
		EventType: event.Type,
		UserID:    event.UserID,
		SessionID: event.SessionID,
		IPAddress: event.IPAddress,
		UserAgent: event.UserAgent,
		Resource:  event.Resource,
		Action:    event.Action,
		Result:    event.Result,
		Error:     event.Error,
		Metadata:  event.Metadata,
	}
	
	// Add trace/request IDs if available
	if event.Metadata != nil {
		if traceID, ok := event.Metadata["trace_id"].(string); ok {
			entry.TraceID = traceID
		}
		if requestID, ok := event.Metadata["request_id"].(string); ok {
			entry.RequestID = requestID
		}
		if duration, ok := event.Metadata["duration"].(time.Duration); ok {
			entry.Duration = duration.String()
		}
	}
	
	// Check if event contains sensitive data
	entry.Sensitive = al.containsSensitiveData(event)
	
	// Sanitize sensitive data if configured
	if entry.Sensitive && !al.config.LogSensitiveData {
		entry = al.sanitizeSensitiveData(entry)
	}
	
	// Log to file if configured
	if al.fileLogger != nil {
		al.logToFile(entry)
	}
	
	// Log to main logger
	al.logToMainLogger(entry)
}

// shouldLogEventType checks if an event type should be logged
func (al *AuditLogger) shouldLogEventType(eventType string) bool {
	if len(al.config.EventTypes) == 0 {
		return true // Log all events if no filter is configured
	}
	
	for _, configuredType := range al.config.EventTypes {
		if configuredType == eventType || configuredType == "*" {
			return true
		}
	}
	
	return false
}

// determineLogLevel determines the appropriate log level for an event
func (al *AuditLogger) determineLogLevel(event *SecurityEvent) string {
	switch event.Result {
	case "SUCCESS":
		return "info"
	case "FAILURE", "BLOCKED", "DENIED":
		return "warn"
	case "ERROR":
		return "error"
	default:
		return "info"
	}
}

// containsSensitiveData checks if an event contains sensitive information
func (al *AuditLogger) containsSensitiveData(event *SecurityEvent) bool {
	// Check for sensitive event types
	sensitiveTypes := []string{"password_change", "token_refresh", "api_key_create"}
	for _, sensitiveType := range sensitiveTypes {
		if event.Type == sensitiveType {
			return true
		}
	}
	
	// Check metadata for sensitive keys
	if event.Metadata != nil {
		sensitiveKeys := []string{"password", "token", "key", "secret", "credential"}
		for key := range event.Metadata {
			keyLower := strings.ToLower(key)
			for _, sensitiveKey := range sensitiveKeys {
				if strings.Contains(keyLower, sensitiveKey) {
					return true
				}
			}
		}
	}
	
	return false
}

// sanitizeSensitiveData removes or masks sensitive information
func (al *AuditLogger) sanitizeSensitiveData(entry *AuditLogEntry) *AuditLogEntry {
	// Create a copy to avoid modifying the original
	sanitized := *entry
	
	// Sanitize metadata
	if entry.Metadata != nil {
		sanitized.Metadata = make(map[string]interface{})
		for key, value := range entry.Metadata {
			keyLower := strings.ToLower(key)
			isSensitive := false
			
			sensitiveKeys := []string{"password", "token", "key", "secret", "credential"}
			for _, sensitiveKey := range sensitiveKeys {
				if strings.Contains(keyLower, sensitiveKey) {
					isSensitive = true
					break
				}
			}
			
			if isSensitive {
				if str, ok := value.(string); ok && len(str) > 4 {
					// Mask all but first 4 characters
					sanitized.Metadata[key] = str[:4] + strings.Repeat("*", len(str)-4)
				} else {
					sanitized.Metadata[key] = "***REDACTED***"
				}
			} else {
				sanitized.Metadata[key] = value
			}
		}
	}
	
	// Mask IP address (keep first 3 octets for IPv4)
	if entry.IPAddress != "" {
		parts := strings.Split(entry.IPAddress, ".")
		if len(parts) == 4 {
			sanitized.IPAddress = fmt.Sprintf("%s.%s.%s.***", parts[0], parts[1], parts[2])
		} else {
			sanitized.IPAddress = "***REDACTED***"
		}
	}
	
	return &sanitized
}

// logToFile logs the entry to the audit log file
func (al *AuditLogger) logToFile(entry *AuditLogEntry) {
	al.mu.Lock()
	defer al.mu.Unlock()
	
	// Check if rotation is needed
	if al.rotator != nil && al.config.RotateSize > 0 {
		if err := al.rotator.checkRotation(al); err != nil {
			al.logger.Error("Failed to rotate log file", zap.Error(err))
		}
	}
	
	// Log the entry
	if strings.ToLower(al.config.LogFormat) == "json" {
		// JSON format
		if jsonData, err := json.Marshal(entry); err == nil {
			al.fileLogger.Info(string(jsonData))
		} else {
			al.logger.Error("Failed to marshal audit log entry", zap.Error(err))
		}
	} else {
		// Plain text format
		message := fmt.Sprintf("[%s] %s - %s:%s %s on %s -> %s",
			entry.EventType, entry.UserID, entry.Action, entry.Resource,
			entry.IPAddress, entry.Timestamp.Format(time.RFC3339), entry.Result)
		
		if entry.Error != "" {
			message += " (Error: " + entry.Error + ")"
		}
		
		al.fileLogger.Info(message)
	}
}

// logToMainLogger logs the entry to the main application logger
func (al *AuditLogger) logToMainLogger(entry *AuditLogEntry) {
	fields := []zap.Field{
		zap.String("event_type", entry.EventType),
		zap.String("user_id", entry.UserID),
		zap.String("ip_address", entry.IPAddress),
		zap.String("result", entry.Result),
	}
	
	if entry.SessionID != "" {
		fields = append(fields, zap.String("session_id", entry.SessionID))
	}
	
	if entry.Resource != "" {
		fields = append(fields, zap.String("resource", entry.Resource))
	}
	
	if entry.Action != "" {
		fields = append(fields, zap.String("action", entry.Action))
	}
	
	if entry.Error != "" {
		fields = append(fields, zap.String("error", entry.Error))
	}
	
	if entry.TraceID != "" {
		fields = append(fields, zap.String("trace_id", entry.TraceID))
	}
	
	message := fmt.Sprintf("Security event: %s", entry.EventType)
	
	switch entry.Level {
	case "debug":
		al.logger.Debug(message, fields...)
	case "info":
		al.logger.Info(message, fields...)
	case "warn":
		al.logger.Warn(message, fields...)
	case "error":
		al.logger.Error(message, fields...)
	default:
		al.logger.Info(message, fields...)
	}
}

// checkRotation checks if log rotation is needed and performs it
func (lr *LogRotator) checkRotation(al *AuditLogger) error {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	
	if lr.currentSize >= lr.config.RotateSize {
		return lr.rotateLog(al)
	}
	
	return nil
}

// rotateLog performs log file rotation
func (lr *LogRotator) rotateLog(al *AuditLogger) error {
	// Close current log file
	if al.logFile != nil {
		al.logFile.Close()
	}
	
	// Rotate existing files
	for i := lr.config.RotateCount - 1; i > 0; i-- {
		oldName := fmt.Sprintf("%s.%d", lr.config.LogFile, i)
		newName := fmt.Sprintf("%s.%d", lr.config.LogFile, i+1)
		
		if _, err := os.Stat(oldName); err == nil {
			os.Rename(oldName, newName)
		}
	}
	
	// Move current log to .1
	if _, err := os.Stat(lr.config.LogFile); err == nil {
		os.Rename(lr.config.LogFile, fmt.Sprintf("%s.1", lr.config.LogFile))
	}
	
	// Create new log file
	var err error
	al.logFile, err = os.OpenFile(lr.config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("failed to create new log file: %w", err)
	}
	
	// Update file logger
	al.fileLogger = al.createFileLogger(al.logFile)
	
	// Reset size counter
	lr.currentSize = 0
	
	lr.logger.Info("Rotated audit log file",
		zap.String("log_file", lr.config.LogFile))
	
	return nil
}

// LogSecurityEvent is a convenience method for logging security events
func (al *AuditLogger) LogSecurityEvent(eventType, userID, sessionID, ipAddress, userAgent, resource, action, result, errorMsg string, metadata map[string]interface{}) {
	event := &SecurityEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		UserID:    userID,
		SessionID: sessionID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Resource:  resource,
		Action:    action,
		Result:    result,
		Error:     errorMsg,
		Metadata:  metadata,
	}
	
	al.LogEvent(event)
}

// LogAuthenticationEvent logs authentication-related events
func (al *AuditLogger) LogAuthenticationEvent(eventType string, userID string, ipAddress string, success bool, errorMsg string, metadata map[string]interface{}) {
	result := "SUCCESS"
	if !success {
		result = "FAILURE"
	}
	
	al.LogSecurityEvent(eventType, userID, "", ipAddress, "", "auth", "authenticate", result, errorMsg, metadata)
}

// LogAuthorizationEvent logs authorization-related events
func (al *AuditLogger) LogAuthorizationEvent(userID, resource, action, ipAddress string, success bool, errorMsg string, metadata map[string]interface{}) {
	result := "SUCCESS"
	if !success {
		result = "DENIED"
	}
	
	al.LogSecurityEvent("authorization", userID, "", ipAddress, "", resource, action, result, errorMsg, metadata)
}

// LogTokenEvent logs token-related events
func (al *AuditLogger) LogTokenEvent(eventType, userID, tokenType, ipAddress string, metadata map[string]interface{}) {
	al.LogSecurityEvent(eventType, userID, "", ipAddress, "", "token", tokenType, "SUCCESS", "", metadata)
}

// LogDataAccessEvent logs data access events
func (al *AuditLogger) LogDataAccessEvent(userID, resource, action, ipAddress string, success bool, metadata map[string]interface{}) {
	result := "SUCCESS"
	if !success {
		result = "FAILURE"
	}
	
	al.LogSecurityEvent("data_access", userID, "", ipAddress, "", resource, action, result, "", metadata)
}

// GetAuditStats returns statistics about audit logging
func (al *AuditLogger) GetAuditStats() map[string]interface{} {
	stats := map[string]interface{}{
		"enabled":     al.config.Enabled,
		"log_file":    al.config.LogFile,
		"log_format":  al.config.LogFormat,
		"log_level":   al.config.LogLevel,
		"event_types": al.config.EventTypes,
	}
	
	if al.rotator != nil {
		al.rotator.mu.Lock()
		stats["current_size"] = al.rotator.currentSize
		stats["rotate_size"] = al.config.RotateSize
		stats["rotate_count"] = al.config.RotateCount
		al.rotator.mu.Unlock()
	}
	
	return stats
}

// Cleanup performs cleanup operations
func (al *AuditLogger) Cleanup() error {
	// Close log file
	if al.logFile != nil {
		al.logFile.Close()
	}
	
	al.logger.Info("Audit logger cleanup completed")
	return nil
}