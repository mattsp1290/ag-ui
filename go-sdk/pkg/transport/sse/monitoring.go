package sse

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// sseMetricsOnce ensures Prometheus metrics are only registered once
	sseMetricsOnce sync.Once
	// ssePromMetrics holds the singleton instance of Prometheus metrics
	ssePromMetrics *SSEPrometheusMetrics
)

// MonitoringSystem provides comprehensive monitoring and observability for SSE transport
type MonitoringSystem struct {
	config MonitoringConfig
	logger *zap.Logger

	// Prometheus metrics
	promMetrics *SSEPrometheusMetrics

	// OpenTelemetry components
	tracer trace.Tracer
	meter  metric.Meter

	// Health checks
	healthChecks map[string]HealthCheck
	healthMu     sync.RWMutex

	// Alert management
	alertManager *AlertManager

	// Performance tracking
	performanceTracker *PerformanceTracker

	// Connection tracking
	connectionTracker *ConnectionTracker

	// Event tracking
	eventTracker *EventTracker

	// Resource monitoring
	resourceMonitor *ResourceMonitor

	// Context and lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Prometheus registry (for testing)
	registry *prometheus.Registry
}

// SSEPrometheusMetrics contains all Prometheus metrics for SSE transport
type SSEPrometheusMetrics struct {
	// Connection metrics
	ConnectionsTotal     *prometheus.CounterVec
	ConnectionsActive    prometheus.Gauge
	ConnectionDuration   *prometheus.HistogramVec
	ConnectionErrors     *prometheus.CounterVec
	ConnectionRetries    *prometheus.CounterVec
	ReconnectionAttempts *prometheus.CounterVec

	// Event metrics
	EventsReceived         *prometheus.CounterVec
	EventsSent             *prometheus.CounterVec
	EventsProcessed        *prometheus.CounterVec
	EventProcessingLatency *prometheus.HistogramVec
	EventSize              *prometheus.HistogramVec
	EventErrors            *prometheus.CounterVec
	EventQueueDepth        prometheus.Gauge
	EventDropped           *prometheus.CounterVec

	// Throughput metrics
	BytesReceived     *prometheus.CounterVec
	BytesSent         *prometheus.CounterVec
	MessagesPerSecond prometheus.Gauge
	BytesPerSecond    prometheus.Gauge

	// Performance metrics
	RequestLatency       *prometheus.HistogramVec
	StreamLatency        *prometheus.HistogramVec
	ParseLatency         *prometheus.HistogramVec
	SerializationLatency *prometheus.HistogramVec

	// Resource metrics
	MemoryUsage       prometheus.Gauge
	GoroutineCount    prometheus.Gauge
	CPUUsage          prometheus.Gauge
	BufferUtilization prometheus.Gauge

	// Health metrics
	HealthCheckStatus   *prometheus.GaugeVec
	HealthCheckDuration *prometheus.HistogramVec
	HealthCheckFailures *prometheus.CounterVec

	// Error metrics
	ErrorRate            *prometheus.GaugeVec
	ErrorsByType         *prometheus.CounterVec
	ErrorsByEndpoint     *prometheus.CounterVec
	CircuitBreakerStatus *prometheus.GaugeVec

	// Rate limiting metrics
	RateLimitHits        *prometheus.CounterVec
	RateLimitExceeded    *prometheus.CounterVec
	RateLimitUtilization prometheus.Gauge

	// Authentication metrics
	AuthAttempts *prometheus.CounterVec
	AuthFailures *prometheus.CounterVec
	AuthLatency  *prometheus.HistogramVec

	// Custom SSE-specific metrics
	KeepAlivesSent     *prometheus.CounterVec
	KeepAlivesReceived *prometheus.CounterVec
	StreamRestarts     *prometheus.CounterVec
	CompressionRatio   prometheus.Gauge
	LastEventID        *prometheus.GaugeVec
}

// PerformanceTracker tracks performance metrics
type PerformanceTracker struct {
	mu sync.RWMutex

	// Latency tracking
	latencyBuckets map[string]*LatencyBucket

	// Throughput tracking
	throughputStats *ThroughputStats

	// Performance benchmarks
	benchmarks map[string]*Benchmark
}

// LatencyBucket tracks latency statistics
type LatencyBucket struct {
	count   int64
	sum     int64
	min     int64
	max     int64
	p50     int64
	p95     int64
	p99     int64
	samples []int64
	mu      sync.RWMutex
}

// ThroughputStats tracks throughput statistics
type ThroughputStats struct {
	eventsPerSecond  float64
	bytesPerSecond   float64
	peakEventsPerSec float64
	peakBytesPerSec  float64
	lastUpdate       time.Time
	mu               sync.RWMutex
}

// Benchmark represents a performance benchmark
type Benchmark struct {
	name       string
	startTime  time.Time
	endTime    time.Time
	operations int64
	bytes      int64
	errors     int64
	avgLatency time.Duration
	minLatency time.Duration
	maxLatency time.Duration
}

// ConnectionTracker tracks connection statistics
type ConnectionTracker struct {
	mu sync.RWMutex

	// Active connections
	activeConnections map[string]*ConnectionStats

	// Connection history
	connectionHistory []ConnectionEvent
	maxHistory        int

	// Aggregate stats
	totalConnections  int64
	activeCount       int64
	failedConnections int64
	avgConnectionTime time.Duration
}

// ConnectionStats represents statistics for a single connection
type ConnectionStats struct {
	ID             string
	StartTime      time.Time
	LastActivity   time.Time
	BytesReceived  int64
	BytesSent      int64
	EventsReceived int64
	EventsSent     int64
	Errors         int64
	Reconnects     int64
	State          ConnectionState
	RemoteAddr     string
	UserAgent      string
}

// ConnectionEvent represents a connection lifecycle event
type ConnectionEvent struct {
	ConnectionID string
	EventType    ConnectionEventType
	Timestamp    time.Time
	Details      map[string]interface{}
}

// ConnectionEventType represents types of connection events
type ConnectionEventType string

const (
	ConnectionEventTypeEstablished  ConnectionEventType = "established"
	ConnectionEventTypeReconnected  ConnectionEventType = "reconnected"
	ConnectionEventTypeDisconnected ConnectionEventType = "disconnected"
	ConnectionEventTypeFailed       ConnectionEventType = "failed"
	ConnectionEventTypeTimeout      ConnectionEventType = "timeout"
)

// EventTracker tracks event-related metrics
type EventTracker struct {
	mu sync.RWMutex

	// Event statistics
	eventStats map[string]*EventStats

	// Event history
	recentEvents []EventRecord
	maxHistory   int

	// Error tracking
	errorStats map[string]*ErrorStats
}

// EventStats represents statistics for a specific event type
type EventStats struct {
	Count          int64
	TotalSize      int64
	AvgSize        int64
	MaxSize        int64
	MinSize        int64
	AvgProcessTime time.Duration
	LastReceived   time.Time
	ErrorCount     int64
}

// EventRecord represents a recorded event
type EventRecord struct {
	ID           string
	Type         string
	Size         int64
	ReceivedAt   time.Time
	ProcessedAt  time.Time
	ProcessTime  time.Duration
	Error        error
	ConnectionID string
}

// ErrorStats tracks error statistics
type ErrorStats struct {
	Count         int64
	FirstOccurred time.Time
	LastOccurred  time.Time
	Frequency     float64
	ErrorMessages map[string]int64
}

// ResourceMonitor monitors system resources
type ResourceMonitor struct {
	mu sync.RWMutex

	// Current resource usage
	memoryUsage    uint64
	cpuUsage       float64
	goroutineCount int

	// Historical data
	memoryHistory []ResourceSample
	cpuHistory    []ResourceSample

	// Alerts
	memoryAlert bool
	cpuAlert    bool

	// Metrics
	memoryGauge    prometheus.Gauge
	cpuGauge       prometheus.Gauge
	goroutineGauge prometheus.Gauge
}

// ResourceSample represents a resource usage sample
type ResourceSample struct {
	Timestamp time.Time
	Value     float64
}

// AlertManager manages monitoring alerts
type AlertManager struct {
	config    AlertingConfig
	notifiers []AlertNotifier
	mu        sync.RWMutex

	// Alert state
	activeAlerts map[string]*Alert
	alertHistory []Alert
	maxHistory   int

	// Alert suppression
	suppressedAlerts map[string]time.Time
}

// Alert represents a monitoring alert
type Alert struct {
	ID          string
	Level       AlertLevel
	Component   string
	Title       string
	Description string
	Value       float64
	Threshold   float64
	Timestamp   time.Time
	Labels      map[string]string
	Resolved    bool
	ResolvedAt  time.Time
}

// AlertLevel defines alert severity levels
type AlertLevel int

const (
	AlertLevelInfo AlertLevel = iota
	AlertLevelWarning
	AlertLevelError
	AlertLevelCritical
)

// AlertNotifier interface for alert notifications
type AlertNotifier interface {
	SendAlert(ctx context.Context, alert Alert) error
}

// HealthCheck interface for health checks
type HealthCheck interface {
	Name() string
	Check(ctx context.Context) error
}

// SSEHealthCheck implements health check for SSE connections
type SSEHealthCheck struct {
	name      string
	endpoint  string
	transport *SSETransport
}

// NewMonitoringSystem creates a new monitoring system for SSE transport
func NewMonitoringSystem(config MonitoringConfig) (*MonitoringSystem, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize logger
	logger, err := initializeSSELogger(config)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Initialize OpenTelemetry if enabled
	var tracer trace.Tracer
	var meter metric.Meter
	if config.Tracing.Enabled {
		tracer = otel.Tracer(config.Tracing.ServiceName)
		meter = otel.Meter(config.Tracing.ServiceName)
	}

	// Initialize Prometheus metrics
	promMetrics := initializeSSEPrometheusMetrics(config)

	// Initialize components
	alertManager := &AlertManager{
		config:           config.Alerting,
		notifiers:        []AlertNotifier{},
		activeAlerts:     make(map[string]*Alert),
		alertHistory:     make([]Alert, 0),
		maxHistory:       1000,
		suppressedAlerts: make(map[string]time.Time),
	}

	performanceTracker := &PerformanceTracker{
		latencyBuckets:  make(map[string]*LatencyBucket),
		throughputStats: &ThroughputStats{
			lastUpdate: time.Now(), // Initialize with current time to avoid zero time issues
		},
		benchmarks:      make(map[string]*Benchmark),
	}

	connectionTracker := &ConnectionTracker{
		activeConnections: make(map[string]*ConnectionStats),
		connectionHistory: make([]ConnectionEvent, 0),
		maxHistory:        10000,
	}

	eventTracker := &EventTracker{
		eventStats:   make(map[string]*EventStats),
		recentEvents: make([]EventRecord, 0),
		maxHistory:   1000,
		errorStats:   make(map[string]*ErrorStats),
	}

	resourceMonitor := &ResourceMonitor{
		memoryHistory:  make([]ResourceSample, 0, 1000),
		cpuHistory:     make([]ResourceSample, 0, 1000),
		memoryGauge:    promMetrics.MemoryUsage,
		cpuGauge:       promMetrics.CPUUsage,
		goroutineGauge: promMetrics.GoroutineCount,
	}

	ms := &MonitoringSystem{
		config:             config,
		logger:             logger,
		promMetrics:        promMetrics,
		tracer:             tracer,
		meter:              meter,
		healthChecks:       make(map[string]HealthCheck),
		alertManager:       alertManager,
		performanceTracker: performanceTracker,
		connectionTracker:  connectionTracker,
		eventTracker:       eventTracker,
		resourceMonitor:    resourceMonitor,
		ctx:                ctx,
		cancel:             cancel,
	}

	// Start background monitoring tasks
	if config.Enabled {
		ms.startBackgroundTasks()
	}

	return ms, nil
}

// Logger returns the structured logger
func (ms *MonitoringSystem) Logger() *zap.Logger {
	return ms.logger
}

// StartTrace starts a new trace span
func (ms *MonitoringSystem) StartTrace(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if ms.tracer == nil {
		return ctx, nil
	}

	return ms.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// RecordConnectionEstablished records a new connection
func (ms *MonitoringSystem) RecordConnectionEstablished(connID, remoteAddr, userAgent string) {
	// Update Prometheus metrics
	ms.promMetrics.ConnectionsTotal.WithLabelValues("established").Inc()
	ms.promMetrics.ConnectionsActive.Inc()

	// Track connection
	ms.connectionTracker.mu.Lock()
	ms.connectionTracker.activeConnections[connID] = &ConnectionStats{
		ID:           connID,
		StartTime:    time.Now(),
		LastActivity: time.Now(),
		State:        ConnectionStateConnected,
		RemoteAddr:   remoteAddr,
		UserAgent:    userAgent,
	}
	atomic.AddInt64(&ms.connectionTracker.totalConnections, 1)
	atomic.AddInt64(&ms.connectionTracker.activeCount, 1)
	ms.connectionTracker.mu.Unlock()

	// Record event
	ms.recordConnectionEvent(connID, ConnectionEventTypeEstablished, map[string]interface{}{
		"remote_addr": remoteAddr,
		"user_agent":  userAgent,
	})

	// Log
	ms.logger.Info("SSE connection established",
		zap.String("connection_id", connID),
		zap.String("remote_addr", remoteAddr),
		zap.String("user_agent", userAgent))
}

// RecordConnectionClosed records a closed connection
func (ms *MonitoringSystem) RecordConnectionClosed(connID string, reason string) {
	// Update Prometheus metrics
	ms.promMetrics.ConnectionsActive.Dec()
	ms.promMetrics.ConnectionsTotal.WithLabelValues("closed").Inc()

	// Update connection stats
	ms.connectionTracker.mu.Lock()
	if conn, exists := ms.connectionTracker.activeConnections[connID]; exists {
		duration := time.Since(conn.StartTime)
		ms.promMetrics.ConnectionDuration.WithLabelValues("normal").Observe(duration.Seconds())

		delete(ms.connectionTracker.activeConnections, connID)
		atomic.AddInt64(&ms.connectionTracker.activeCount, -1)
	}
	ms.connectionTracker.mu.Unlock()

	// Record event
	ms.recordConnectionEvent(connID, ConnectionEventTypeDisconnected, map[string]interface{}{
		"reason": reason,
	})

	// Log
	ms.logger.Info("SSE connection closed",
		zap.String("connection_id", connID),
		zap.String("reason", reason))
}

// RecordConnectionError records a connection error
func (ms *MonitoringSystem) RecordConnectionError(connID string, err error) {
	// Update Prometheus metrics
	errorType := categorizeSSEError(err)
	ms.promMetrics.ConnectionErrors.WithLabelValues(errorType).Inc()

	// Update connection stats
	ms.connectionTracker.mu.Lock()
	if conn, exists := ms.connectionTracker.activeConnections[connID]; exists {
		atomic.AddInt64(&conn.Errors, 1)
		conn.LastActivity = time.Now()
	}
	ms.connectionTracker.mu.Unlock()

	// Log error
	ms.logger.Error("SSE connection error",
		zap.String("connection_id", connID),
		zap.Error(err),
		zap.String("error_type", errorType))

	// Check if alert is needed
	ms.checkConnectionErrorThreshold()
}

// RecordEventReceived records a received event
func (ms *MonitoringSystem) RecordEventReceived(connID, eventType string, size int64) {
	start := time.Now()

	// Validate size to prevent counter issues
	if size < 0 {
		ms.logger.Warn("Invalid negative event size, using 0", 
			zap.String("connection_id", connID),
			zap.String("event_type", eventType),
			zap.Int64("size", size))
		size = 0
	}

	// Update Prometheus metrics
	ms.promMetrics.EventsReceived.WithLabelValues(eventType).Inc()
	ms.promMetrics.EventSize.WithLabelValues(eventType, "received").Observe(float64(size))
	ms.promMetrics.BytesReceived.WithLabelValues(connID).Add(float64(size))

	// Update connection stats
	ms.connectionTracker.mu.Lock()
	if conn, exists := ms.connectionTracker.activeConnections[connID]; exists {
		atomic.AddInt64(&conn.EventsReceived, 1)
		atomic.AddInt64(&conn.BytesReceived, size)
		conn.LastActivity = time.Now()
	}
	ms.connectionTracker.mu.Unlock()

	// Update event stats
	ms.eventTracker.mu.Lock()
	if stats, exists := ms.eventTracker.eventStats[eventType]; exists {
		atomic.AddInt64(&stats.Count, 1)
		atomic.AddInt64(&stats.TotalSize, size)
		stats.LastReceived = time.Now()
		if size > stats.MaxSize {
			stats.MaxSize = size
		}
		if stats.MinSize == 0 || size < stats.MinSize {
			stats.MinSize = size
		}
	} else {
		ms.eventTracker.eventStats[eventType] = &EventStats{
			Count:        1,
			TotalSize:    size,
			AvgSize:      size,
			MaxSize:      size,
			MinSize:      size,
			LastReceived: time.Now(),
		}
	}
	ms.eventTracker.mu.Unlock()

	// Record event for history
	ms.recordEventHistory(EventRecord{
		ID:           fmt.Sprintf("%s-%d", eventType, time.Now().UnixNano()),
		Type:         eventType,
		Size:         size,
		ReceivedAt:   start,
		ConnectionID: connID,
	})

	// Update throughput
	ms.updateThroughput(1, size)
}

// RecordEventSent records a sent event
func (ms *MonitoringSystem) RecordEventSent(connID, eventType string, size int64, duration time.Duration) {
	// Update Prometheus metrics
	ms.promMetrics.EventsSent.WithLabelValues(eventType).Inc()
	ms.promMetrics.EventSize.WithLabelValues(eventType, "sent").Observe(float64(size))
	ms.promMetrics.BytesSent.WithLabelValues(connID).Add(float64(size))
	ms.promMetrics.SerializationLatency.WithLabelValues(eventType).Observe(duration.Seconds())

	// Update connection stats
	ms.connectionTracker.mu.Lock()
	if conn, exists := ms.connectionTracker.activeConnections[connID]; exists {
		atomic.AddInt64(&conn.EventsSent, 1)
		atomic.AddInt64(&conn.BytesSent, size)
		conn.LastActivity = time.Now()
	}
	ms.connectionTracker.mu.Unlock()
}

// RecordEventProcessed records event processing completion
func (ms *MonitoringSystem) RecordEventProcessed(eventType string, duration time.Duration, err error) {
	// Update Prometheus metrics
	status := "success"
	if err != nil {
		status = "error"
		ms.promMetrics.EventErrors.WithLabelValues(eventType, categorizeSSEError(err)).Inc()
	}

	ms.promMetrics.EventsProcessed.WithLabelValues(eventType, status).Inc()
	ms.promMetrics.EventProcessingLatency.WithLabelValues(eventType).Observe(duration.Seconds())

	// Update event stats for error rate calculation
	ms.eventTracker.mu.Lock()
	if stats, exists := ms.eventTracker.eventStats[eventType]; exists {
		atomic.AddInt64(&stats.Count, 1)
		if err != nil {
			atomic.AddInt64(&stats.ErrorCount, 1)
		}
	} else {
		errorCount := int64(0)
		if err != nil {
			errorCount = 1
		}
		ms.eventTracker.eventStats[eventType] = &EventStats{
			Count:       1,
			ErrorCount:  errorCount,
			LastReceived: time.Now(),
		}
	}
	ms.eventTracker.mu.Unlock()

	// Update performance tracking
	ms.recordLatency("event_processing", duration)

	// Update event tracker error count
	if err != nil {
		ms.eventTracker.mu.Lock()
		if stats, exists := ms.eventTracker.eventStats[eventType]; exists {
			atomic.AddInt64(&stats.ErrorCount, 1)
		}
		ms.eventTracker.mu.Unlock()
	}

	// Log if slow or error
	if duration > 100*time.Millisecond || err != nil {
		fields := []zap.Field{
			zap.String("event_type", eventType),
			zap.Duration("duration", duration),
		}

		if err != nil {
			fields = append(fields, zap.Error(err))
			ms.logger.Error("Event processing failed", fields...)
		} else {
			ms.logger.Warn("Slow event processing", fields...)
		}
	}
}

// RecordKeepAlive records keep-alive activity
func (ms *MonitoringSystem) RecordKeepAlive(connID string, direction string) {
	switch direction {
	case "sent":
		ms.promMetrics.KeepAlivesSent.WithLabelValues(connID).Inc()
	case "received":
		ms.promMetrics.KeepAlivesReceived.WithLabelValues(connID).Inc()
	}

	// Update connection last activity
	ms.connectionTracker.mu.Lock()
	if conn, exists := ms.connectionTracker.activeConnections[connID]; exists {
		conn.LastActivity = time.Now()
	}
	ms.connectionTracker.mu.Unlock()
}

// RecordReconnection records a reconnection attempt
func (ms *MonitoringSystem) RecordReconnection(connID string, attempt int, success bool) {
	status := "success"
	if !success {
		status = "failed"
	}

	ms.promMetrics.ReconnectionAttempts.WithLabelValues(status).Inc()
	ms.promMetrics.ConnectionRetries.WithLabelValues(connID).Inc()

	// Update connection stats
	ms.connectionTracker.mu.Lock()
	if conn, exists := ms.connectionTracker.activeConnections[connID]; exists {
		atomic.AddInt64(&conn.Reconnects, 1)
		if success {
			conn.State = ConnectionStateConnected
		} else {
			conn.State = ConnectionStateReconnecting
		}
	}
	ms.connectionTracker.mu.Unlock()

	// Record event
	ms.recordConnectionEvent(connID, ConnectionEventTypeReconnected, map[string]interface{}{
		"attempt": attempt,
		"success": success,
	})

	// Log
	ms.logger.Info("SSE reconnection attempt",
		zap.String("connection_id", connID),
		zap.Int("attempt", attempt),
		zap.Bool("success", success))
}

// RecordAuthAttempt records authentication attempts
func (ms *MonitoringSystem) RecordAuthAttempt(method string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "failed"
		ms.promMetrics.AuthFailures.WithLabelValues(method).Inc()
	}

	ms.promMetrics.AuthAttempts.WithLabelValues(method, status).Inc()
	ms.promMetrics.AuthLatency.WithLabelValues(method).Observe(duration.Seconds())
}

// RecordRateLimit records rate limiting events
func (ms *MonitoringSystem) RecordRateLimit(endpoint string, exceeded bool) {
	ms.promMetrics.RateLimitHits.WithLabelValues(endpoint).Inc()

	if exceeded {
		ms.promMetrics.RateLimitExceeded.WithLabelValues(endpoint).Inc()

		ms.logger.Warn("Rate limit exceeded",
			zap.String("endpoint", endpoint))
	}
}

// RegisterHealthCheck registers a new health check
func (ms *MonitoringSystem) RegisterHealthCheck(check HealthCheck) {
	ms.healthMu.Lock()
	defer ms.healthMu.Unlock()
	ms.healthChecks[check.Name()] = check
}

// GetHealthStatus returns current health status
func (ms *MonitoringSystem) GetHealthStatus() map[string]HealthStatus {
	ms.healthMu.RLock()
	defer ms.healthMu.RUnlock()

	status := make(map[string]HealthStatus)
	for name, check := range ms.healthChecks {
		ctx, cancel := context.WithTimeout(ms.ctx, 5*time.Second)
		start := time.Now()
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("health check panicked: %v", r)
				}
			}()
			err = check.Check(ctx)
		}()
		duration := time.Since(start)
		cancel()

		healthy := err == nil
		status[name] = HealthStatus{
			Name:      name,
			Healthy:   healthy,
			Error:     err,
			Duration:  duration,
			LastCheck: time.Now(),
		}

		// Update metrics
		healthValue := 0.0
		if healthy {
			healthValue = 1.0
		}
		ms.promMetrics.HealthCheckStatus.WithLabelValues(name).Set(healthValue)
		ms.promMetrics.HealthCheckDuration.WithLabelValues(name).Observe(duration.Seconds())

		if !healthy {
			ms.promMetrics.HealthCheckFailures.WithLabelValues(name).Inc()
		}
	}

	return status
}

// GetConnectionStats returns current connection statistics
func (ms *MonitoringSystem) GetConnectionStats() ConnectionStatsSummary {
	ms.connectionTracker.mu.RLock()
	defer ms.connectionTracker.mu.RUnlock()

	activeConns := make([]ConnectionStats, 0, len(ms.connectionTracker.activeConnections))
	var totalBytes, totalEvents int64

	for _, conn := range ms.connectionTracker.activeConnections {
		activeConns = append(activeConns, *conn)
		totalBytes += conn.BytesReceived + conn.BytesSent
		totalEvents += conn.EventsReceived + conn.EventsSent
	}

	return ConnectionStatsSummary{
		TotalConnections:     ms.connectionTracker.totalConnections,
		ActiveConnections:    ms.connectionTracker.activeCount,
		FailedConnections:    ms.connectionTracker.failedConnections,
		TotalBytes:           totalBytes,
		TotalEvents:          totalEvents,
		ActiveConnectionList: activeConns,
	}
}

// GetEventStats returns event statistics
func (ms *MonitoringSystem) GetEventStats() map[string]EventStats {
	ms.eventTracker.mu.RLock()
	defer ms.eventTracker.mu.RUnlock()

	stats := make(map[string]EventStats)
	for eventType, stat := range ms.eventTracker.eventStats {
		// Calculate average size
		if stat.Count > 0 {
			stat.AvgSize = stat.TotalSize / stat.Count
		}
		stats[eventType] = *stat
	}

	return stats
}

// GetPerformanceMetrics returns current performance metrics
func (ms *MonitoringSystem) GetPerformanceMetrics() PerformanceMetrics {
	ms.performanceTracker.mu.RLock()
	defer ms.performanceTracker.mu.RUnlock()

	latencies := make(map[string]LatencyStats)
	for name, bucket := range ms.performanceTracker.latencyBuckets {
		latencies[name] = bucket.getStats()
	}

	throughput := ThroughputMetrics{
		EventsPerSecond:  ms.performanceTracker.throughputStats.eventsPerSecond,
		BytesPerSecond:   ms.performanceTracker.throughputStats.bytesPerSecond,
		PeakEventsPerSec: ms.performanceTracker.throughputStats.peakEventsPerSec,
		PeakBytesPerSec:  ms.performanceTracker.throughputStats.peakBytesPerSec,
	}

	return PerformanceMetrics{
		Latencies:  latencies,
		Throughput: throughput,
		Timestamp:  time.Now(),
	}
}

// StartBenchmark starts a performance benchmark
func (ms *MonitoringSystem) StartBenchmark(name string) *Benchmark {
	benchmark := &Benchmark{
		name:      name,
		startTime: time.Now(),
	}

	ms.performanceTracker.mu.Lock()
	ms.performanceTracker.benchmarks[name] = benchmark
	ms.performanceTracker.mu.Unlock()

	return benchmark
}

// CompleteBenchmark completes a benchmark
func (ms *MonitoringSystem) CompleteBenchmark(benchmark *Benchmark) {
	benchmark.endTime = time.Now()
	duration := benchmark.endTime.Sub(benchmark.startTime)

	if benchmark.operations > 0 {
		benchmark.avgLatency = duration / time.Duration(benchmark.operations)
	}

	ms.logger.Info("Benchmark completed",
		zap.String("name", benchmark.name),
		zap.Duration("duration", duration),
		zap.Int64("operations", benchmark.operations),
		zap.Int64("bytes", benchmark.bytes),
		zap.Int64("errors", benchmark.errors),
		zap.Duration("avg_latency", benchmark.avgLatency))
}

// Shutdown gracefully shuts down the monitoring system
func (ms *MonitoringSystem) Shutdown(ctx context.Context) error {
	ms.logger.Info("Shutting down SSE monitoring system")

	// Cancel background tasks
	ms.cancel()

	// Wait for background tasks to complete
	done := make(chan struct{})
	go func() {
		ms.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		ms.logger.Info("SSE monitoring system shut down successfully")
	case <-ctx.Done():
		ms.logger.Warn("SSE monitoring system shutdown timed out")
		return ctx.Err()
	}

	// Sync logger
	if err := ms.logger.Sync(); err != nil {
		// Ignore sync errors on stdout/stderr
		if !strings.Contains(err.Error(), "sync /dev/stdout") &&
			!strings.Contains(err.Error(), "sync /dev/stderr") {
			return err
		}
	}

	return nil
}

// Helper methods

func (ms *MonitoringSystem) startBackgroundTasks() {
	// Resource monitoring
	if ms.config.Metrics.Enabled {
		ms.wg.Add(1)
		go ms.resourceMonitoringLoop()
	}

	// Health checks
	if ms.config.HealthChecks.Enabled {
		ms.wg.Add(1)
		go ms.healthCheckLoop()
	}

	// Metrics aggregation
	ms.wg.Add(1)
	go ms.metricsAggregationLoop()

	// Alert checking
	if ms.config.Alerting.Enabled {
		ms.wg.Add(1)
		go ms.alertCheckLoop()
	}
}

func (ms *MonitoringSystem) resourceMonitoringLoop() {
	defer ms.wg.Done()

	ticker := time.NewTicker(ms.config.Metrics.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ms.collectResourceMetrics()
		case <-ms.ctx.Done():
			return
		}
	}
}

func (ms *MonitoringSystem) healthCheckLoop() {
	defer ms.wg.Done()

	ticker := time.NewTicker(ms.config.HealthChecks.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ms.runHealthChecks()
		case <-ms.ctx.Done():
			return
		}
	}
}

func (ms *MonitoringSystem) metricsAggregationLoop() {
	defer ms.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ms.aggregateMetrics()
		case <-ms.ctx.Done():
			return
		}
	}
}

func (ms *MonitoringSystem) alertCheckLoop() {
	defer ms.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ms.checkAlertThresholds()
		case <-ms.ctx.Done():
			return
		}
	}
}

func (ms *MonitoringSystem) collectResourceMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	ms.resourceMonitor.mu.Lock()
	ms.resourceMonitor.memoryUsage = memStats.Alloc
	ms.resourceMonitor.goroutineCount = runtime.NumGoroutine()

	// Add to history
	ms.resourceMonitor.memoryHistory = append(ms.resourceMonitor.memoryHistory, ResourceSample{
		Timestamp: time.Now(),
		Value:     float64(memStats.Alloc),
	})

	// Trim history if needed
	if len(ms.resourceMonitor.memoryHistory) > 1000 {
		ms.resourceMonitor.memoryHistory = ms.resourceMonitor.memoryHistory[1:]
	}
	ms.resourceMonitor.mu.Unlock()

	// Update Prometheus metrics
	ms.promMetrics.MemoryUsage.Set(float64(memStats.Alloc))
	ms.promMetrics.GoroutineCount.Set(float64(runtime.NumGoroutine()))
}

func (ms *MonitoringSystem) runHealthChecks() {
	status := ms.GetHealthStatus()

	unhealthy := 0
	for _, s := range status {
		if !s.Healthy {
			unhealthy++
		}
	}

	if unhealthy > 0 {
		ms.sendAlert(Alert{
			Level:       AlertLevelWarning,
			Component:   "health_check",
			Title:       "Health Check Failures",
			Description: fmt.Sprintf("%d health checks are failing", unhealthy),
			Value:       float64(unhealthy),
			Timestamp:   time.Now(),
		})
	}
}

func (ms *MonitoringSystem) aggregateMetrics() {
	// Update throughput metrics
	ms.performanceTracker.mu.RLock()
	throughput := ms.performanceTracker.throughputStats
	ms.performanceTracker.mu.RUnlock()

	ms.promMetrics.MessagesPerSecond.Set(throughput.eventsPerSecond)
	ms.promMetrics.BytesPerSecond.Set(throughput.bytesPerSecond)

	// Update error rates
	ms.calculateErrorRates()

	// Update connection metrics
	ms.connectionTracker.mu.RLock()
	activeCount := ms.connectionTracker.activeCount
	ms.connectionTracker.mu.RUnlock()

	ms.promMetrics.ConnectionsActive.Set(float64(activeCount))
}

func (ms *MonitoringSystem) checkAlertThresholds() {
	// Check error rate
	errorRate := ms.calculateOverallErrorRate()
	if errorRate > ms.config.Alerting.Thresholds.ErrorRate {
		ms.sendAlert(Alert{
			Level:       AlertLevelError,
			Component:   "error_rate",
			Title:       "High Error Rate",
			Description: fmt.Sprintf("Error rate (%.2f%%) exceeds threshold", errorRate),
			Value:       errorRate,
			Threshold:   ms.config.Alerting.Thresholds.ErrorRate,
			Timestamp:   time.Now(),
		})
	}

	// Check connection count
	activeConns := atomic.LoadInt64(&ms.connectionTracker.activeCount)
	if int(activeConns) > ms.config.Alerting.Thresholds.ConnectionCount {
		ms.sendAlert(Alert{
			Level:       AlertLevelWarning,
			Component:   "connections",
			Title:       "High Connection Count",
			Description: fmt.Sprintf("Active connections (%d) exceed threshold", activeConns),
			Value:       float64(activeConns),
			Threshold:   float64(ms.config.Alerting.Thresholds.ConnectionCount),
			Timestamp:   time.Now(),
		})
	}

	// Check memory usage
	ms.resourceMonitor.mu.RLock()
	memoryUsage := ms.resourceMonitor.memoryUsage
	ms.resourceMonitor.mu.RUnlock()

	// Read current memory stats to get Sys value
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// Calculate memory percentage - avoid divide by zero
	var memoryPercent float64
	if memStats.Sys > 0 {
		memoryPercent = float64(memoryUsage) / float64(memStats.Sys) * 100
	}
	
	if memoryPercent > ms.config.Alerting.Thresholds.MemoryUsage {
		ms.sendAlert(Alert{
			Level:       AlertLevelWarning,
			Component:   "memory",
			Title:       "High Memory Usage",
			Description: fmt.Sprintf("Memory usage (%.2f%%) exceeds threshold", memoryPercent),
			Value:       memoryPercent,
			Threshold:   ms.config.Alerting.Thresholds.MemoryUsage,
			Timestamp:   time.Now(),
		})
	}
}

func (ms *MonitoringSystem) recordConnectionEvent(connID string, eventType ConnectionEventType, details map[string]interface{}) {
	event := ConnectionEvent{
		ConnectionID: connID,
		EventType:    eventType,
		Timestamp:    time.Now(),
		Details:      details,
	}

	ms.connectionTracker.mu.Lock()
	ms.connectionTracker.connectionHistory = append(ms.connectionTracker.connectionHistory, event)

	// Trim history if needed
	if len(ms.connectionTracker.connectionHistory) > ms.connectionTracker.maxHistory {
		ms.connectionTracker.connectionHistory = ms.connectionTracker.connectionHistory[1:]
	}
	ms.connectionTracker.mu.Unlock()
}

func (ms *MonitoringSystem) recordEventHistory(record EventRecord) {
	ms.eventTracker.mu.Lock()
	ms.eventTracker.recentEvents = append(ms.eventTracker.recentEvents, record)

	// Trim history if needed
	if len(ms.eventTracker.recentEvents) > ms.eventTracker.maxHistory {
		ms.eventTracker.recentEvents = ms.eventTracker.recentEvents[1:]
	}
	ms.eventTracker.mu.Unlock()
}

func (ms *MonitoringSystem) recordLatency(operation string, duration time.Duration) {
	ms.performanceTracker.mu.Lock()
	defer ms.performanceTracker.mu.Unlock()

	bucket, exists := ms.performanceTracker.latencyBuckets[operation]
	if !exists {
		bucket = &LatencyBucket{
			samples: make([]int64, 0, 1000),
		}
		ms.performanceTracker.latencyBuckets[operation] = bucket
	}

	bucket.addSample(duration.Nanoseconds())
}

func (ms *MonitoringSystem) updateThroughput(events int64, bytes int64) {
	ms.performanceTracker.mu.Lock()
	defer ms.performanceTracker.mu.Unlock()

	now := time.Now()
	stats := ms.performanceTracker.throughputStats

	// Check if this is the first update
	if stats.lastUpdate.IsZero() {
		// For the first update, initialize lastUpdate and skip rate calculation
		stats.lastUpdate = now
		return
	}

	elapsed := now.Sub(stats.lastUpdate).Seconds()

	// Only update if enough time has passed (avoid division by very small numbers)
	if elapsed > 0.1 { // At least 100ms
		eventsPerSec := float64(events) / elapsed
		bytesPerSec := float64(bytes) / elapsed

		// Update current rates
		stats.eventsPerSecond = eventsPerSec
		stats.bytesPerSecond = bytesPerSec

		// Update peaks
		if eventsPerSec > stats.peakEventsPerSec {
			stats.peakEventsPerSec = eventsPerSec
		}
		if bytesPerSec > stats.peakBytesPerSec {
			stats.peakBytesPerSec = bytesPerSec
		}

		stats.lastUpdate = now
	}
}

func (ms *MonitoringSystem) checkConnectionErrorThreshold() {
	// Simple threshold check - can be made more sophisticated
	ms.connectionTracker.mu.RLock()
	errorCount := int64(0)
	for _, conn := range ms.connectionTracker.activeConnections {
		errorCount += conn.Errors
	}
	ms.connectionTracker.mu.RUnlock()

	if errorCount > 100 { // Example threshold
		ms.sendAlert(Alert{
			Level:       AlertLevelError,
			Component:   "connection_errors",
			Title:       "High Connection Error Count",
			Description: fmt.Sprintf("Connection errors (%d) exceed threshold", errorCount),
			Value:       float64(errorCount),
			Threshold:   100,
			Timestamp:   time.Now(),
		})
	}
}

func (ms *MonitoringSystem) calculateErrorRates() {
	ms.eventTracker.mu.RLock()
	defer ms.eventTracker.mu.RUnlock()

	for eventType, stats := range ms.eventTracker.eventStats {
		if stats.Count > 0 {
			errorRate := float64(stats.ErrorCount) / float64(stats.Count) * 100
			ms.promMetrics.ErrorRate.WithLabelValues(eventType).Set(errorRate)
		}
	}
}

func (ms *MonitoringSystem) calculateOverallErrorRate() float64 {
	ms.eventTracker.mu.RLock()
	defer ms.eventTracker.mu.RUnlock()

	var totalEvents, totalErrors int64
	for _, stats := range ms.eventTracker.eventStats {
		totalEvents += stats.Count
		totalErrors += stats.ErrorCount
	}

	if totalEvents == 0 {
		return 0
	}

	return float64(totalErrors) / float64(totalEvents) * 100
}

func (ms *MonitoringSystem) sendAlert(alert Alert) {
	ms.alertManager.mu.Lock()
	defer ms.alertManager.mu.Unlock()

	// Check if alert is suppressed
	if suppressedUntil, exists := ms.alertManager.suppressedAlerts[alert.Component]; exists {
		if time.Now().Before(suppressedUntil) {
			return
		}
	}

	// Generate alert ID
	alert.ID = fmt.Sprintf("%s-%d", alert.Component, time.Now().Unix())

	// Store alert
	ms.alertManager.activeAlerts[alert.Component] = &alert
	ms.alertManager.alertHistory = append(ms.alertManager.alertHistory, alert)

	// Trim history if needed
	if len(ms.alertManager.alertHistory) > ms.alertManager.maxHistory {
		ms.alertManager.alertHistory = ms.alertManager.alertHistory[1:]
	}

	// Send to notifiers
	for _, notifier := range ms.alertManager.notifiers {
		ms.wg.Add(1)
		go func(n AlertNotifier) {
			defer ms.wg.Done()
			
			// Use the monitoring system's context to allow for cancellation during shutdown
			ctx, cancel := context.WithTimeout(ms.ctx, 5*time.Second)
			defer cancel()
			
			if err := n.SendAlert(ctx, alert); err != nil {
				// Only log error if not due to context cancellation
				if !errors.Is(err, context.Canceled) {
					ms.logger.Error("Failed to send alert", zap.Error(err))
				}
			}
		}(notifier)
	}

	// Log alert
	ms.logger.Warn("Alert triggered",
		zap.String("component", alert.Component),
		zap.String("title", alert.Title),
		zap.String("description", alert.Description),
		zap.Float64("value", alert.Value),
		zap.Float64("threshold", alert.Threshold))

	// Suppress similar alerts for a period
	ms.alertManager.suppressedAlerts[alert.Component] = time.Now().Add(5 * time.Minute)
}

// LatencyBucket methods

func (lb *LatencyBucket) addSample(nanos int64) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	atomic.AddInt64(&lb.count, 1)
	atomic.AddInt64(&lb.sum, nanos)

	if lb.min == 0 || nanos < lb.min {
		lb.min = nanos
	}
	if nanos > lb.max {
		lb.max = nanos
	}

	lb.samples = append(lb.samples, nanos)
	if len(lb.samples) > 1000 {
		lb.samples = lb.samples[1:]
	}

	// Calculate percentiles
	lb.calculatePercentilesLocked()
}

// calculatePercentilesLocked calculates percentiles from samples
// Note: caller must hold the lock
func (lb *LatencyBucket) calculatePercentilesLocked() {
	if len(lb.samples) == 0 {
		return
	}

	// Create a copy of samples for sorting
	sorted := make([]int64, len(lb.samples))
	copy(sorted, lb.samples)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate percentile indices (0-based)
	// For percentile p, the index is (n-1) * p / 100
	p50Index := ((len(sorted) - 1) * 50) / 100
	p95Index := ((len(sorted) - 1) * 95) / 100
	p99Index := ((len(sorted) - 1) * 99) / 100

	// Ensure indices are within bounds
	if p50Index >= len(sorted) {
		p50Index = len(sorted) - 1
	}
	if p95Index >= len(sorted) {
		p95Index = len(sorted) - 1
	}
	if p99Index >= len(sorted) {
		p99Index = len(sorted) - 1
	}

	// Update percentile values
	lb.p50 = sorted[p50Index]
	lb.p95 = sorted[p95Index]
	lb.p99 = sorted[p99Index]
}

func (lb *LatencyBucket) getStats() LatencyStats {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if lb.count == 0 {
		return LatencyStats{}
	}

	avg := lb.sum / lb.count

	// Calculate percentiles (simplified)
	stats := LatencyStats{
		Count: lb.count,
		Min:   time.Duration(lb.min),
		Max:   time.Duration(lb.max),
		Avg:   time.Duration(avg),
		P50:   time.Duration(lb.p50),
		P95:   time.Duration(lb.p95),
		P99:   time.Duration(lb.p99),
	}

	return stats
}

// SSEHealthCheck methods

func (hc *SSEHealthCheck) Name() string {
	return hc.name
}

func (hc *SSEHealthCheck) Check(ctx context.Context) error {
	// Implement SSE-specific health check
	// For example, try to establish a test connection
	req, err := http.NewRequestWithContext(ctx, "GET", hc.endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				},
			},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// Helper functions

func initializeSSELogger(config MonitoringConfig) (*zap.Logger, error) {
	zapConfig := zap.Config{
		Level:    zap.NewAtomicLevelAt(config.Logging.Level),
		Encoding: config.Logging.Format,
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      config.Logging.Output,
		ErrorOutputPaths: []string{"stderr"},
	}

	if config.Logging.Sampling.Enabled {
		zapConfig.Sampling = &zap.SamplingConfig{
			Initial:    config.Logging.Sampling.Initial,
			Thereafter: config.Logging.Sampling.Thereafter,
		}
	}

	logger, err := zapConfig.Build()
	if err != nil {
		return nil, err
	}

	return logger.With(zap.String("component", "sse_transport")), nil
}

// Global variable to track metrics initialization with sync.Once
var (
	metricsOnce sync.Once
	globalMetrics *SSEPrometheusMetrics
)

func initializeSSEPrometheusMetrics(config MonitoringConfig) *SSEPrometheusMetrics {
	sseMetricsOnce.Do(func() {
		namespace := config.Metrics.Prometheus.Namespace
		subsystem := config.Metrics.Prometheus.Subsystem

	// Determine which registry to use
	registry := config.Metrics.Prometheus.Registry
	if registry == nil {
		// Use sync.Once for default registry to prevent duplicate registration
		metricsOnce.Do(func() {
			globalMetrics = createSSEPrometheusMetrics(namespace, subsystem, prometheus.DefaultRegisterer)
		})
		return globalMetrics
	}

	// For custom registry, always create new metrics
	return createSSEPrometheusMetricsWithRegistry(namespace, subsystem, registry)
}

func createSSEPrometheusMetrics(namespace, subsystem string, registerer prometheus.Registerer) *SSEPrometheusMetrics {
	metrics := &SSEPrometheusMetrics{
		// Connection metrics
		ConnectionsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "connections_total",
				Help:      "Total number of SSE connections",
			},
			[]string{"status"},
		),
		ConnectionsActive: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "connections_active",
				Help:      "Number of active SSE connections",
			},
		),
		ConnectionDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "connection_duration_seconds",
				Help:      "Duration of SSE connections",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"status"},
		),
		ConnectionErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "connection_errors_total",
				Help:      "Total number of connection errors",
			},
			[]string{"error_type"},
		),
		ConnectionRetries: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "connection_retries_total",
				Help:      "Total number of connection retries",
			},
			[]string{"connection_id"},
		),
		ReconnectionAttempts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "reconnection_attempts_total",
				Help:      "Total number of reconnection attempts",
			},
			[]string{"status"},
		),

		// Event metrics
		EventsReceived: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "events_received_total",
				Help:      "Total number of events received",
			},
			[]string{"event_type"},
		),
		EventsSent: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "events_sent_total",
				Help:      "Total number of events sent",
			},
			[]string{"event_type"},
		),
		EventsProcessed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "events_processed_total",
				Help:      "Total number of events processed",
			},
			[]string{"event_type", "status"},
		),
		EventProcessingLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "event_processing_latency_seconds",
				Help:      "Event processing latency",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"event_type"},
		),
		EventSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "event_size_bytes",
				Help:      "Size of events in bytes",
				Buckets:   []float64{100, 500, 1000, 5000, 10000, 50000, 100000},
			},
			[]string{"event_type", "direction"},
		),
		EventErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "event_errors_total",
				Help:      "Total number of event processing errors",
			},
			[]string{"event_type", "error_type"},
		),
		EventQueueDepth: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "event_queue_depth",
				Help:      "Current event queue depth",
			},
		),
		EventDropped: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "events_dropped_total",
				Help:      "Total number of dropped events",
			},
			[]string{"reason"},
		),

		// Throughput metrics
		BytesReceived: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "bytes_received_total",
				Help:      "Total bytes received",
			},
			[]string{"connection_id"},
		),
		BytesSent: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "bytes_sent_total",
				Help:      "Total bytes sent",
			},
			[]string{"connection_id"},
		),
		MessagesPerSecond: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "messages_per_second",
				Help:      "Current message throughput per second",
			},
		),
		BytesPerSecond: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "bytes_per_second",
				Help:      "Current bytes throughput per second",
			},
		),

		// Performance metrics
		RequestLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "request_latency_seconds",
				Help:      "HTTP request latency",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
		StreamLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "stream_latency_seconds",
				Help:      "SSE stream latency",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"stream_type"},
		),
		ParseLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "parse_latency_seconds",
				Help:      "Event parsing latency",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"format"},
		),
		SerializationLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "serialization_latency_seconds",
				Help:      "Event serialization latency",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"format"},
		),

		// Resource metrics
		MemoryUsage: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "memory_usage_bytes",
				Help:      "Current memory usage in bytes",
			},
		),
		GoroutineCount: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "goroutine_count",
				Help:      "Current number of goroutines",
			},
		),
		CPUUsage: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "cpu_usage_percent",
				Help:      "Current CPU usage percentage",
			},
		),
		BufferUtilization: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "buffer_utilization_percent",
				Help:      "Buffer utilization percentage",
			},
		),

		// Health metrics
		HealthCheckStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "health_check_status",
				Help:      "Health check status (1=healthy, 0=unhealthy)",
			},
			[]string{"check_name"},
		),
		HealthCheckDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "health_check_duration_seconds",
				Help:      "Health check duration",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"check_name"},
		),
		HealthCheckFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "health_check_failures_total",
				Help:      "Total number of health check failures",
			},
			[]string{"check_name"},
		),

		// Error metrics
		ErrorRate: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "error_rate_percent",
				Help:      "Current error rate percentage",
			},
			[]string{"component"},
		),
		ErrorsByType: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "errors_by_type_total",
				Help:      "Total errors by type",
			},
			[]string{"error_type"},
		),
		ErrorsByEndpoint: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "errors_by_endpoint_total",
				Help:      "Total errors by endpoint",
			},
			[]string{"endpoint", "error_type"},
		),
		CircuitBreakerStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "circuit_breaker_status",
				Help:      "Circuit breaker status (0=closed, 1=open, 2=half-open)",
			},
			[]string{"circuit_name"},
		),

		// Rate limiting metrics
		RateLimitHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "rate_limit_hits_total",
				Help:      "Total rate limit hits",
			},
			[]string{"endpoint"},
		),
		RateLimitExceeded: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "rate_limit_exceeded_total",
				Help:      "Total rate limit exceeded events",
			},
			[]string{"endpoint"},
		),
		RateLimitUtilization: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "rate_limit_utilization_percent",
				Help:      "Rate limit utilization percentage",
			},
		),

		// Authentication metrics
		AuthAttempts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "auth_attempts_total",
				Help:      "Total authentication attempts",
			},
			[]string{"method", "status"},
		),
		AuthFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "auth_failures_total",
				Help:      "Total authentication failures",
			},
			[]string{"method"},
		),
		AuthLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "auth_latency_seconds",
				Help:      "Authentication latency",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method"},
		),

		// SSE-specific metrics
		KeepAlivesSent: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "keepalives_sent_total",
				Help:      "Total keep-alive messages sent",
			},
			[]string{"connection_id"},
		),
		KeepAlivesReceived: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "keepalives_received_total",
				Help:      "Total keep-alive messages received",
			},
			[]string{"connection_id"},
		),
		StreamRestarts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "stream_restarts_total",
				Help:      "Total SSE stream restarts",
			},
			[]string{"reason"},
		),
		CompressionRatio: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "compression_ratio",
				Help:      "Current compression ratio",
			},
		),
		LastEventID: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "last_event_id",
				Help:      "Last processed event ID",
			},
			[]string{"stream"},
		),
	}
	
	// Register metrics with the default registry if registerer is provided
	if registerer != nil {
		registerer.MustRegister(
			metrics.ConnectionsTotal,
			metrics.ConnectionsActive,
			metrics.ConnectionDuration,
			metrics.ConnectionErrors,
			metrics.ConnectionRetries,
			metrics.ReconnectionAttempts,
			metrics.EventsReceived,
			metrics.EventsSent,
			metrics.EventsProcessed,
			metrics.EventProcessingLatency,
			metrics.EventSize,
			metrics.EventErrors,
			metrics.EventQueueDepth,
			metrics.EventDropped,
			metrics.BytesReceived,
			metrics.BytesSent,
			metrics.MessagesPerSecond,
			metrics.BytesPerSecond,
			metrics.RequestLatency,
			metrics.StreamLatency,
			metrics.ParseLatency,
			metrics.SerializationLatency,
			metrics.MemoryUsage,
			metrics.GoroutineCount,
			metrics.CPUUsage,
			metrics.BufferUtilization,
			metrics.HealthCheckStatus,
			metrics.HealthCheckDuration,
			metrics.HealthCheckFailures,
			metrics.ErrorRate,
			metrics.ErrorsByType,
			metrics.ErrorsByEndpoint,
			metrics.CircuitBreakerStatus,
			metrics.RateLimitHits,
			metrics.RateLimitExceeded,
			metrics.RateLimitUtilization,
			metrics.AuthAttempts,
			metrics.AuthFailures,
			metrics.AuthLatency,
			metrics.KeepAlivesSent,
			metrics.KeepAlivesReceived,
			metrics.StreamRestarts,
			metrics.CompressionRatio,
			metrics.LastEventID,
		)
	}
	
	return metrics
}

func createSSEPrometheusMetricsWithRegistry(namespace, subsystem string, registry *prometheus.Registry) *SSEPrometheusMetrics {
	metrics := createSSEPrometheusMetrics(namespace, subsystem, nil)
	
	// Register all metrics with the custom registry
	registry.MustRegister(
		metrics.ConnectionsTotal,
		metrics.ConnectionsActive,
		metrics.ConnectionDuration,
		metrics.ConnectionErrors,
		metrics.ConnectionRetries,
		metrics.ReconnectionAttempts,
		metrics.EventsReceived,
		metrics.EventsSent,
		metrics.EventsProcessed,
		metrics.EventProcessingLatency,
		metrics.EventSize,
		metrics.EventErrors,
		metrics.EventQueueDepth,
		metrics.EventDropped,
		metrics.BytesReceived,
		metrics.BytesSent,
		metrics.MessagesPerSecond,
		metrics.BytesPerSecond,
		metrics.RequestLatency,
		metrics.StreamLatency,
		metrics.ParseLatency,
		metrics.SerializationLatency,
		metrics.MemoryUsage,
		metrics.GoroutineCount,
		metrics.CPUUsage,
		metrics.BufferUtilization,
		metrics.HealthCheckStatus,
		metrics.HealthCheckDuration,
		metrics.HealthCheckFailures,
		metrics.ErrorRate,
		metrics.ErrorsByType,
		metrics.ErrorsByEndpoint,
		metrics.CircuitBreakerStatus,
		metrics.RateLimitHits,
		metrics.RateLimitExceeded,
		metrics.RateLimitUtilization,
		metrics.AuthAttempts,
		metrics.AuthFailures,
		metrics.AuthLatency,
		metrics.KeepAlivesSent,
		metrics.KeepAlivesReceived,
		metrics.StreamRestarts,
		metrics.CompressionRatio,
		metrics.LastEventID,
	)
	
		return metrics
}

func categorizeSSEError(err error) string {
	if err == nil {
		return "none"
	}

	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "timeout"):
		return "timeout"
	case strings.Contains(errStr, "refused"):
		return "refused"
	case strings.Contains(errStr, "reset"):
		return "reset"
	case strings.Contains(errStr, "closed"):
		return "closed"
	case strings.Contains(errStr, "eof"):
		return "eof"
	case strings.Contains(errStr, "parse"):
		return "parse"
	case strings.Contains(errStr, "auth"):
		return "auth"
	case strings.Contains(errStr, "rate"):
		return "rate_limit"
	case strings.Contains(errStr, "connection"):
		return "connection"
	default:
		return "other"
	}
}

// Types for monitoring data

// HealthStatus represents the status of a health check
type HealthStatus struct {
	Name      string
	Healthy   bool
	Error     error
	Duration  time.Duration
	LastCheck time.Time
}

// ConnectionStatsSummary provides a summary of connection statistics
type ConnectionStatsSummary struct {
	TotalConnections     int64
	ActiveConnections    int64
	FailedConnections    int64
	TotalBytes           int64
	TotalEvents          int64
	ActiveConnectionList []ConnectionStats
}

// LatencyStats represents latency statistics
type LatencyStats struct {
	Count int64
	Min   time.Duration
	Max   time.Duration
	Avg   time.Duration
	P50   time.Duration
	P95   time.Duration
	P99   time.Duration
}

// PerformanceMetrics represents overall performance metrics
type PerformanceMetrics struct {
	Latencies  map[string]LatencyStats
	Throughput ThroughputMetrics
	Timestamp  time.Time
}

// ThroughputMetrics represents throughput metrics
type ThroughputMetrics struct {
	EventsPerSecond  float64
	BytesPerSecond   float64
	PeakEventsPerSec float64
	PeakBytesPerSec  float64
}

// Exported functions for creating monitoring components

// NewSSEHealthCheck creates a new SSE health check
func NewSSEHealthCheck(name, endpoint string, transport *SSETransport) HealthCheck {
	return &SSEHealthCheck{
		name:      name,
		endpoint:  endpoint,
		transport: transport,
	}
}

// WritePrometheusMetrics writes Prometheus metrics to the given writer
func WritePrometheusMetrics(w io.Writer) error {
	// This would typically be handled by the Prometheus HTTP handler
	// but can be useful for debugging or custom endpoints
	return nil
}

// DefaultMonitoringConfig returns a default monitoring configuration
func DefaultMonitoringConfig() MonitoringConfig {
	return MonitoringConfig{
		Enabled: true,
		Metrics: MetricsConfig{
			Enabled:  true,
			Interval: 30 * time.Second,
			Prometheus: PrometheusConfig{
				Enabled:   true,
				Namespace: "sse",
				Subsystem: "transport",
			},
		},
		Logging: LoggingConfig{
			Enabled:    true,
			Level:      zapcore.InfoLevel,
			Format:     "json",
			Output:     []string{"stdout"},
			Structured: true,
			Sampling: LogSamplingConfig{
				Enabled:    true,
				Initial:    100,
				Thereafter: 100,
			},
		},
		HealthChecks: HealthChecksConfig{
			Enabled:  true,
			Interval: 30 * time.Second,
			Timeout:  5 * time.Second,
		},
		Tracing: TracingConfig{
			Enabled:      false,
			ServiceName:  "sse-transport",
			SamplingRate: 0.1,
		},
		Alerting: AlertingConfig{
			Enabled: true,
			Thresholds: AlertThresholds{
				ErrorRate:       5.0,
				Latency:         1000,
				MemoryUsage:     80,
				CPUUsage:        80,
				ConnectionCount: 1000,
			},
		},
	}
}
