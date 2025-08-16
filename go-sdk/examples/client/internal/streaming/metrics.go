package streaming

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// StreamMetrics tracks performance metrics for SSE streaming
type StreamMetrics struct {
	// Connection metrics
	ConnectionsOpened   atomic.Int64
	ConnectionsClosed   atomic.Int64
	ConnectionsFailed   atomic.Int64
	ActiveConnections   atomic.Int64
	
	// Data metrics
	EventsReceived      atomic.Int64
	BytesReceived       atomic.Int64
	ErrorsEncountered   atomic.Int64
	
	// Performance metrics
	AverageLatency      atomic.Int64 // in microseconds
	MaxLatency          atomic.Int64 // in microseconds
	MinLatency          atomic.Int64 // in microseconds
	
	// Buffer metrics
	BufferOverflows     atomic.Int64
	BufferUnderruns     atomic.Int64
	
	// Retry metrics
	RetryAttempts       atomic.Int64
	RetrySuccesses      atomic.Int64
	RetryFailures       atomic.Int64
	
	// Time tracking
	startTime           time.Time
	lastResetTime       time.Time
	mu                  sync.RWMutex
	latencyMeasurements []int64
}

// NewStreamMetrics creates a new metrics tracker
func NewStreamMetrics() *StreamMetrics {
	now := time.Now()
	return &StreamMetrics{
		startTime:           now,
		lastResetTime:       now,
		latencyMeasurements: make([]int64, 0, 1000),
	}
}

// RecordConnectionOpened increments the connection opened counter
func (m *StreamMetrics) RecordConnectionOpened() {
	m.ConnectionsOpened.Add(1)
	m.ActiveConnections.Add(1)
}

// RecordConnectionClosed increments the connection closed counter
func (m *StreamMetrics) RecordConnectionClosed() {
	m.ConnectionsClosed.Add(1)
	m.ActiveConnections.Add(-1)
}

// RecordConnectionFailed increments the connection failed counter
func (m *StreamMetrics) RecordConnectionFailed() {
	m.ConnectionsFailed.Add(1)
}

// RecordEvent records an event received
func (m *StreamMetrics) RecordEvent(sizeBytes int64) {
	m.EventsReceived.Add(1)
	m.BytesReceived.Add(sizeBytes)
}

// RecordError records an error encountered
func (m *StreamMetrics) RecordError() {
	m.ErrorsEncountered.Add(1)
}

// RecordLatency records a latency measurement in microseconds
func (m *StreamMetrics) RecordLatency(latencyMicros int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Update running statistics
	m.latencyMeasurements = append(m.latencyMeasurements, latencyMicros)
	
	// Keep only last 1000 measurements to avoid memory growth
	if len(m.latencyMeasurements) > 1000 {
		m.latencyMeasurements = m.latencyMeasurements[len(m.latencyMeasurements)-1000:]
	}
	
	// Calculate average
	var sum int64
	for _, l := range m.latencyMeasurements {
		sum += l
	}
	if len(m.latencyMeasurements) > 0 {
		m.AverageLatency.Store(sum / int64(len(m.latencyMeasurements)))
	}
	
	// Update min/max
	currentMax := m.MaxLatency.Load()
	if latencyMicros > currentMax || currentMax == 0 {
		m.MaxLatency.Store(latencyMicros)
	}
	
	currentMin := m.MinLatency.Load()
	if latencyMicros < currentMin || currentMin == 0 {
		m.MinLatency.Store(latencyMicros)
	}
}

// RecordBufferOverflow records a buffer overflow event
func (m *StreamMetrics) RecordBufferOverflow() {
	m.BufferOverflows.Add(1)
}

// RecordBufferUnderrun records a buffer underrun event
func (m *StreamMetrics) RecordBufferUnderrun() {
	m.BufferUnderruns.Add(1)
}

// RecordRetryAttempt records a retry attempt
func (m *StreamMetrics) RecordRetryAttempt() {
	m.RetryAttempts.Add(1)
}

// RecordRetrySuccess records a successful retry
func (m *StreamMetrics) RecordRetrySuccess() {
	m.RetrySuccesses.Add(1)
}

// RecordRetryFailure records a failed retry
func (m *StreamMetrics) RecordRetryFailure() {
	m.RetryFailures.Add(1)
}

// GetSnapshot returns a snapshot of current metrics
func (m *StreamMetrics) GetSnapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	uptime := time.Since(m.startTime)
	timeSinceReset := time.Since(m.lastResetTime)
	
	return MetricsSnapshot{
		// Connection metrics
		ConnectionsOpened: m.ConnectionsOpened.Load(),
		ConnectionsClosed: m.ConnectionsClosed.Load(),
		ConnectionsFailed: m.ConnectionsFailed.Load(),
		ActiveConnections: m.ActiveConnections.Load(),
		
		// Data metrics
		EventsReceived:    m.EventsReceived.Load(),
		BytesReceived:     m.BytesReceived.Load(),
		ErrorsEncountered: m.ErrorsEncountered.Load(),
		
		// Performance metrics
		AverageLatencyMicros: m.AverageLatency.Load(),
		MaxLatencyMicros:     m.MaxLatency.Load(),
		MinLatencyMicros:     m.MinLatency.Load(),
		
		// Buffer metrics
		BufferOverflows:  m.BufferOverflows.Load(),
		BufferUnderruns:  m.BufferUnderruns.Load(),
		
		// Retry metrics
		RetryAttempts:  m.RetryAttempts.Load(),
		RetrySuccesses: m.RetrySuccesses.Load(),
		RetryFailures:  m.RetryFailures.Load(),
		
		// Calculated metrics
		EventsPerSecond:      calculateRate(m.EventsReceived.Load(), timeSinceReset),
		BytesPerSecond:       calculateRate(m.BytesReceived.Load(), timeSinceReset),
		ErrorRate:            calculateRate(m.ErrorsEncountered.Load(), timeSinceReset),
		ConnectionSuccessRate: calculateSuccessRate(m.ConnectionsOpened.Load(), m.ConnectionsFailed.Load()),
		RetrySuccessRate:     calculateSuccessRate(m.RetrySuccesses.Load(), m.RetryFailures.Load()),
		
		// Time tracking
		Uptime:         uptime,
		TimeSinceReset: timeSinceReset,
	}
}

// Reset resets the metrics
func (m *StreamMetrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Reset counters
	m.EventsReceived.Store(0)
	m.BytesReceived.Store(0)
	m.ErrorsEncountered.Store(0)
	m.BufferOverflows.Store(0)
	m.BufferUnderruns.Store(0)
	m.RetryAttempts.Store(0)
	m.RetrySuccesses.Store(0)
	m.RetryFailures.Store(0)
	
	// Reset latency measurements
	m.latencyMeasurements = make([]int64, 0, 1000)
	m.AverageLatency.Store(0)
	m.MaxLatency.Store(0)
	m.MinLatency.Store(0)
	
	// Update reset time
	m.lastResetTime = time.Now()
}

// MetricsSnapshot represents a point-in-time snapshot of metrics
type MetricsSnapshot struct {
	// Connection metrics
	ConnectionsOpened int64
	ConnectionsClosed int64
	ConnectionsFailed int64
	ActiveConnections int64
	
	// Data metrics
	EventsReceived    int64
	BytesReceived     int64
	ErrorsEncountered int64
	
	// Performance metrics
	AverageLatencyMicros int64
	MaxLatencyMicros     int64
	MinLatencyMicros     int64
	
	// Buffer metrics
	BufferOverflows  int64
	BufferUnderruns  int64
	
	// Retry metrics
	RetryAttempts  int64
	RetrySuccesses int64
	RetryFailures  int64
	
	// Calculated metrics
	EventsPerSecond       float64
	BytesPerSecond        float64
	ErrorRate             float64
	ConnectionSuccessRate float64
	RetrySuccessRate      float64
	
	// Time tracking
	Uptime         time.Duration
	TimeSinceReset time.Duration
}

// calculateRate calculates a rate per second
func calculateRate(count int64, duration time.Duration) float64 {
	if duration.Seconds() == 0 {
		return 0
	}
	return float64(count) / duration.Seconds()
}

// calculateSuccessRate calculates success rate as a percentage
func calculateSuccessRate(successes, failures int64) float64 {
	total := successes + failures
	if total == 0 {
		return 0
	}
	return (float64(successes) / float64(total)) * 100
}

// FormatMetrics formats metrics snapshot as a human-readable string
func FormatMetrics(snapshot MetricsSnapshot) string {
	return fmt.Sprintf(`
=== Stream Performance Metrics ===
Uptime: %v | Time Since Reset: %v

Connections:
  Opened: %d | Closed: %d | Failed: %d | Active: %d
  Success Rate: %.2f%%

Data Transfer:
  Events: %d (%.2f/sec)
  Bytes: %d (%.2f bytes/sec)
  Errors: %d (%.2f/sec)

Latency (microseconds):
  Average: %d | Min: %d | Max: %d

Buffer Performance:
  Overflows: %d | Underruns: %d

Retry Statistics:
  Attempts: %d | Successes: %d | Failures: %d
  Success Rate: %.2f%%
`,
		snapshot.Uptime,
		snapshot.TimeSinceReset,
		snapshot.ConnectionsOpened,
		snapshot.ConnectionsClosed,
		snapshot.ConnectionsFailed,
		snapshot.ActiveConnections,
		snapshot.ConnectionSuccessRate,
		snapshot.EventsReceived,
		snapshot.EventsPerSecond,
		snapshot.BytesReceived,
		snapshot.BytesPerSecond,
		snapshot.ErrorsEncountered,
		snapshot.ErrorRate,
		snapshot.AverageLatencyMicros,
		snapshot.MinLatencyMicros,
		snapshot.MaxLatencyMicros,
		snapshot.BufferOverflows,
		snapshot.BufferUnderruns,
		snapshot.RetryAttempts,
		snapshot.RetrySuccesses,
		snapshot.RetryFailures,
		snapshot.RetrySuccessRate,
	)
}