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
	// Redact sensitive headers
	for _, header := range SensitiveHeaders {
		lowerHeader := strings.ToLower(header)
		if _, ok := entry.Data[header]; ok {
			entry.Data[header] = "[REDACTED]"
		}
		if _, ok := entry.Data[lowerHeader]; ok {
			entry.Data[lowerHeader] = "[REDACTED]"
		}
	}
	
	// Redact sensitive fields in nested data
	for key, value := range entry.Data {
		if strings.Contains(strings.ToLower(key), "password") ||
			strings.Contains(strings.ToLower(key), "secret") ||
			strings.Contains(strings.ToLower(key), "token") {
			entry.Data[key] = "[REDACTED]"
		}
		
		// Check for headers field that might contain sensitive data
		if key == "headers" {
			if headers, ok := value.(map[string]interface{}); ok {
				for hKey := range headers {
					for _, sensitive := range SensitiveHeaders {
						if strings.EqualFold(hKey, sensitive) {
							headers[hKey] = "[REDACTED]"
						}
					}
				}
			}
		}
	}
	
	return nil
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