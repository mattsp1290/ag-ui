package errors

import (
	"fmt"
	"log"
	"os"
)

// Logger interface for error logging
type Logger interface {
	Logf(format string, args ...interface{})
	Error(msg string)
	Warn(msg string)
	Info(msg string)
}

// DefaultLogger implements Logger using the standard log package
type DefaultLogger struct {
	logger *log.Logger
	prefix string
}

// NewDefaultLogger creates a new DefaultLogger
func NewDefaultLogger(prefix string) *DefaultLogger {
	return &DefaultLogger{
		logger: log.New(os.Stderr, fmt.Sprintf("[%s] ", prefix), log.LstdFlags|log.Lshortfile),
		prefix: prefix,
	}
}

func (l *DefaultLogger) Logf(format string, args ...interface{}) {
	l.logger.Printf(format, args...)
}

func (l *DefaultLogger) Error(msg string) {
	l.logger.Printf("ERROR: %s", msg)
}

func (l *DefaultLogger) Warn(msg string) {
	l.logger.Printf("WARN: %s", msg)
}

func (l *DefaultLogger) Info(msg string) {
	l.logger.Printf("INFO: %s", msg)
}

// NoOpLogger is a logger that does nothing
type NoOpLogger struct{}

func (l *NoOpLogger) Logf(format string, args ...interface{}) {}
func (l *NoOpLogger) Error(msg string)                       {}
func (l *NoOpLogger) Warn(msg string)                        {}
func (l *NoOpLogger) Info(msg string)                        {}