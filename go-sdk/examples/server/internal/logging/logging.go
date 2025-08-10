package logging

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// Config holds logging configuration
type Config struct {
	Level        string
	EnableCaller bool
}

// Init initializes and returns a configured logrus logger
func Init(cfg Config) (*logrus.Logger, error) {
	logger := logrus.New()

	// Use JSON formatter for structured logging
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00", // RFC3339 with milliseconds
	})

	// Set output to stdout for container friendliness
	logger.SetOutput(os.Stdout)

	// Set log level
	level, err := parseLogLevel(cfg.Level)
	if err != nil {
		// Default to info level on error
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// Enable caller reporting if requested
	logger.SetReportCaller(cfg.EnableCaller)

	return logger, nil
}

// parseLogLevel converts string log level to logrus.Level
func parseLogLevel(levelStr string) (logrus.Level, error) {
	switch strings.ToLower(levelStr) {
	case "debug":
		return logrus.DebugLevel, nil
	case "info":
		return logrus.InfoLevel, nil
	case "warn", "warning":
		return logrus.WarnLevel, nil
	case "error":
		return logrus.ErrorLevel, nil
	case "fatal":
		return logrus.FatalLevel, nil
	case "panic":
		return logrus.PanicLevel, nil
	default:
		return logrus.InfoLevel, nil
	}
}

// WithRequestFields creates a logrus.Entry with request-scoped fields
func WithRequestFields(logger *logrus.Logger, requestID, method, path, remoteIP, userAgent string) *logrus.Entry {
	return logger.WithFields(logrus.Fields{
		"request_id": requestID,
		"method":     method,
		"path":       path,
		"remote_ip":  remoteIP,
		"user_agent": userAgent,
	})
}
