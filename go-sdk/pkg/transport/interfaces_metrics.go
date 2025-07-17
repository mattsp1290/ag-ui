package transport

import (
	"time"
)

// MetricsCollector collects and reports transport metrics.
type MetricsCollector interface {
	// RecordEvent records an event metric.
	RecordEvent(eventType string, size int64, latency time.Duration)

	// RecordError records an error metric.
	RecordError(errorType string, err error)

	// RecordConnection records a connection metric.
	RecordConnection(connected bool, duration time.Duration)

	// GetMetrics returns collected metrics.
	GetMetrics() map[string]any

	// Reset resets all collected metrics.
	Reset()
}