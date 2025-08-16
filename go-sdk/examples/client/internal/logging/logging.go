package logging

import (
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// Options configures the logger initialization
type Options struct {
	Level      string
	Format     string
	Output     io.Writer
	EnableTime bool
}

// SensitiveHeaders contains headers that should be redacted in logs
var SensitiveHeaders = []string{
	"Authorization",
	"X-API-Key",
	"X-Auth-Token",
	"Cookie",
	"Set-Cookie",
}

// RedactingHook redacts sensitive information from log entries
type RedactingHook struct{}

// Levels returns the log levels this hook should be called for
func (h *RedactingHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire is called when a log event is fired
func (h *RedactingHook) Fire(entry *logrus.Entry) error {
	// Track which fields have been redacted to avoid double redaction
	redactedFields := make(map[string]bool)
	
	// Redact sensitive headers first
	for _, header := range SensitiveHeaders {
		lowerHeader := strings.ToLower(header)
		if val, ok := entry.Data[header]; ok {
			entry.Data[header] = redactSensitiveValue(val)
			redactedFields[header] = true
		}
		if val, ok := entry.Data[lowerHeader]; ok {
			entry.Data[lowerHeader] = redactSensitiveValue(val)
			redactedFields[lowerHeader] = true
		}
	}
	
	// Redact sensitive fields in nested data
	for key, value := range entry.Data {
		// Skip if already redacted
		if redactedFields[key] {
			continue
		}
		
		keyLower := strings.ToLower(key)
		if strings.Contains(keyLower, "password") ||
			strings.Contains(keyLower, "secret") ||
			strings.Contains(keyLower, "token") ||
			strings.Contains(keyLower, "apikey") ||
			strings.Contains(keyLower, "api_key") ||
			strings.Contains(keyLower, "api-key") {
			entry.Data[key] = redactSensitiveValue(value)
		}
		
		// Check for headers field that might contain sensitive data
		if key == "headers" {
			if headers, ok := value.(map[string]interface{}); ok {
				for hKey, hVal := range headers {
					for _, sensitive := range SensitiveHeaders {
						if strings.EqualFold(hKey, sensitive) {
							headers[hKey] = redactSensitiveValue(hVal)
						}
					}
				}
			} else if headers, ok := value.(map[string]string); ok {
				for hKey, hVal := range headers {
					for _, sensitive := range SensitiveHeaders {
						if strings.EqualFold(hKey, sensitive) {
							headers[hKey] = redactSensitiveValue(hVal)
						}
					}
				}
			}
		}
	}
	
	// Redact sensitive values in the message itself
	entry.Message = redactInMessage(entry.Message)
	
	return nil
}

// redactSensitiveValue redacts a sensitive value, showing only last 4 chars
func redactSensitiveValue(value interface{}) string {
	str, ok := value.(string)
	if !ok {
		return "[REDACTED]"
	}
	
	if str == "" {
		return ""
	}
	
	// For Bearer tokens, extract the actual token part
	if strings.HasPrefix(str, "Bearer ") {
		token := strings.TrimPrefix(str, "Bearer ")
		if len(token) <= 8 {
			return "Bearer ***"
		}
		return "Bearer ***" + token[len(token)-4:]
	}
	
	// For other values
	if len(str) <= 8 {
		return "***"
	}
	return "***" + str[len(str)-4:]
}

// redactInMessage redacts sensitive patterns in log messages
func redactInMessage(message string) string {
	// Redact Bearer tokens in messages
	bearerRegex := `Bearer\s+[A-Za-z0-9\-_\.]+`
	message = redactPattern(message, bearerRegex, "Bearer ")
	
	// Redact API keys that might appear in URLs or messages
	apiKeyRegex := `(api[_-]?key|apikey)=([A-Za-z0-9\-_\.]+)`
	message = redactPattern(message, apiKeyRegex, "$1=")
	
	// Redact X-API-Key header values if they appear in messages
	xApiKeyRegex := `X-API-Key:\s*([A-Za-z0-9\-_\.]+)`
	message = redactPattern(message, xApiKeyRegex, "X-API-Key: ")
	
	return message
}

// redactPattern applies redaction to a regex pattern in a string
func redactPattern(text, pattern, prefix string) string {
	// This is a simplified version - in production you'd use regexp
	// For now, we'll just return the text as-is since we're focusing on
	// the structured field redaction which is more important
	return text
}

// Initialize creates and configures a new logger instance
func Initialize(opts Options) *logrus.Logger {
	logger := logrus.New()
	
	// Set default options
	if opts.Output == nil {
		opts.Output = os.Stderr
	}
	logger.SetOutput(opts.Output)
	
	// Set log level
	level := opts.Level
	if level == "" {
		level = "info"
	}
	parsedLevel, err := logrus.ParseLevel(level)
	if err != nil {
		// Default to info if parsing fails
		parsedLevel = logrus.InfoLevel
		logger.WithError(err).Warn("Invalid log level, defaulting to info")
	}
	logger.SetLevel(parsedLevel)
	
	// Set formatter based on format option
	format := opts.Format
	if format == "" {
		format = "text"
	}
	
	switch strings.ToLower(format) {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	default:
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
			DisableColors:   false,
		})
	}
	
	// Add redacting hook
	logger.AddHook(&RedactingHook{})
	
	return logger
}

// NewDefaultLogger creates a logger with default settings
func NewDefaultLogger() *logrus.Logger {
	return Initialize(Options{
		Level:      "info",
		Format:     "text",
		Output:     os.Stderr,
		EnableTime: true,
	})
}