package state

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// MonitoringConfig configures the monitoring system
type MonitoringConfig struct {
	// Metrics configuration
	EnablePrometheus    bool
	PrometheusNamespace string
	PrometheusSubsystem string
	MetricsEnabled      bool
	MetricsInterval     time.Duration

	// Logging configuration
	LogLevel          zapcore.Level
	LogOutput         io.Writer
	LogFormat         string // "json" or "console"
	StructuredLogging bool
	LogSampling       bool

	// Tracing configuration
	EnableTracing      bool
	TracingServiceName string
	TracingProvider    string
	TraceSampleRate    float64

	// Health check configuration
	EnableHealthChecks  bool
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration

	// Alert configuration
	AlertThresholds AlertThresholds
	AlertNotifiers  []AlertNotifier

	// Performance monitoring
	EnableProfiling       bool
	CPUProfileInterval    time.Duration
	MemoryProfileInterval time.Duration

	// Resource monitoring
	EnableResourceMonitoring bool
	ResourceSampleInterval   time.Duration

	// Audit integration
	AuditIntegration   bool
	AuditSeverityLevel AuditSeverityLevel
}

// AlertThresholds defines thresholds for various alerts
type AlertThresholds struct {
	// Error rate thresholds
	ErrorRate       float64 // Percentage
	ErrorRateWindow time.Duration

	// Latency thresholds
	P95LatencyMs float64
	P99LatencyMs float64

	// Memory thresholds
	MemoryUsagePercent float64
	GCPauseMs          float64

	// Connection pool thresholds
	ConnectionPoolUtil float64
	ConnectionErrors   int64

	// Queue thresholds
	QueueDepth     int64
	QueueLatencyMs float64

	// Rate limit thresholds
	RateLimitRejects int64
	RateLimitUtil    float64
}

// AlertNotifier defines the interface for alert notifications
type AlertNotifier interface {
	SendAlert(ctx context.Context, alert Alert) error
}

// Alert represents a monitoring alert
type Alert struct {
	Level       AlertLevel
	Title       string
	Description string
	Timestamp   time.Time
	Labels      map[string]string
	Value       float64
	Threshold   float64
	Component   string
	Severity    AuditSeverityLevel
}

// AlertLevel defines the severity of an alert
type AlertLevel int

const (
	AlertLevelInfo AlertLevel = iota
	AlertLevelWarning
	AlertLevelError
	AlertLevelCritical
)

// AuditSeverityLevel defines audit severity levels
type AuditSeverityLevel int

const (
	AuditSeverityDebug AuditSeverityLevel = iota
	AuditSeverityInfo
	AuditSeverityWarning
	AuditSeverityError
	AuditSeverityCritical
)

// DefaultMonitoringConfig returns default monitoring configuration
func DefaultMonitoringConfig() MonitoringConfig {
	return MonitoringConfig{
		EnablePrometheus:    true,
		PrometheusNamespace: "state_manager",
		PrometheusSubsystem: "core",
		MetricsEnabled:      true,
		MetricsInterval:     DefaultMetricsInterval,
		LogLevel:            zapcore.InfoLevel,
		LogOutput:           os.Stdout,
		LogFormat:           "json",
		StructuredLogging:   true,
		LogSampling:         true,
		EnableTracing:       false,
		TracingServiceName:  "state-manager",
		TracingProvider:     "jaeger",
		TraceSampleRate:     DefaultTraceSampleRate,
		EnableHealthChecks:  true,
		HealthCheckInterval: DefaultHealthCheckInterval,
		HealthCheckTimeout:  DefaultHealthCheckTimeout,
		AlertThresholds: AlertThresholds{
			ErrorRate:          DefaultErrorRateThreshold,
			ErrorRateWindow:    DefaultErrorRateWindow,
			P95LatencyMs:       DefaultP95LatencyThreshold,
			P99LatencyMs:       DefaultP99LatencyThreshold,
			MemoryUsagePercent: DefaultMemoryUsageThreshold,
			GCPauseMs:          DefaultGCPauseThreshold,
			ConnectionPoolUtil: DefaultConnectionPoolThreshold,
			ConnectionErrors:   DefaultConnectionErrorThreshold,
			QueueDepth:         DefaultQueueDepthThreshold,
			QueueLatencyMs:     DefaultQueueLatencyThreshold,
			RateLimitRejects:   DefaultRateLimitRejectThreshold,
			RateLimitUtil:      DefaultRateLimitUtilThreshold,
		},
		EnableProfiling:          false,
		CPUProfileInterval:       DefaultCPUProfileInterval,
		MemoryProfileInterval:    DefaultMemoryProfileInterval,
		EnableResourceMonitoring: true,
		ResourceSampleInterval:   DefaultResourceSampleInterval,
		AuditIntegration:         true,
		AuditSeverityLevel:       AuditSeverityInfo,
	}
}

// AuditEventLogger defines the interface for audit event logging
type AuditEventLogger interface {
	LogSecurityEvent(ctx context.Context, action AuditAction, contextID, userID, resource string, details map[string]interface{})
}

// MonitoringSystem provides comprehensive monitoring and observability
type MonitoringSystem struct {
	config MonitoringConfig
	logger *zap.Logger

	// Prometheus metrics
	promMetrics *PrometheusMetrics
	// Custom registry for isolated metrics
	registry *prometheus.Registry

	// Health checks
	healthChecks map[string]HealthCheck
	healthMu     sync.RWMutex

	// Alerting
	alertManager *AlertManager

	// Resource monitoring
	resourceMonitor *ResourceMonitor

	// Context and lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Audit integration
	auditManager AuditEventLogger

	// Performance tracking
	operationMetrics *OperationMetrics

	// Connection pool monitoring
	connectionPoolMetrics *ConnectionPoolMetrics
}

// PrometheusMetrics contains all Prometheus metrics
type PrometheusMetrics struct {
	// Custom registry for this monitoring system instance
	Registry *prometheus.Registry

	// State operation metrics
	StateOperationsTotal   *prometheus.CounterVec
	StateOperationDuration *prometheus.HistogramVec
	StateOperationErrors   *prometheus.CounterVec

	// Memory metrics
	MemoryUsage       prometheus.Gauge
	MemoryAllocations prometheus.Counter
	GCPauseDuration   prometheus.Histogram
	ObjectPoolHitRate prometheus.Gauge
	
	// System resource metrics
	CPUUsage       prometheus.Gauge
	GoroutineCount prometheus.Gauge

	// Event processing metrics
	EventsProcessed        *prometheus.CounterVec
	EventProcessingLatency *prometheus.HistogramVec
	EventQueueDepth        prometheus.Gauge

	// Storage backend metrics
	StorageOperations *prometheus.CounterVec
	StorageLatency    *prometheus.HistogramVec
	StorageErrors     *prometheus.CounterVec

	// Connection pool metrics
	ConnectionPoolSize    prometheus.Gauge
	ConnectionPoolActive  prometheus.Gauge
	ConnectionPoolWaiting prometheus.Gauge
	ConnectionPoolErrors  prometheus.Counter

	// Rate limiting metrics
	RateLimitRequests    *prometheus.CounterVec
	RateLimitRejects     *prometheus.CounterVec
	RateLimitUtilization prometheus.Gauge

	// Health check metrics
	HealthCheckStatus   *prometheus.GaugeVec
	HealthCheckDuration *prometheus.HistogramVec

	// Audit metrics
	AuditLogsWritten      *prometheus.CounterVec
	AuditLogErrors        prometheus.Counter
	AuditVerificationTime prometheus.Histogram
}

// HealthCheck defines the interface for health checks
type HealthCheck interface {
	Name() string
	Check(ctx context.Context) error
}

// AlertManager handles alert generation and notification
type AlertManager struct {
	thresholds AlertThresholds
	notifiers  []AlertNotifier
	mu         sync.RWMutex

	// Alert state tracking
	activeAlerts map[string]*Alert
	alertHistory []Alert
	maxHistory   int
}

// ResourceMonitor tracks system resource usage
type ResourceMonitor struct {
	mu          sync.RWMutex
	lastSample  time.Time
	cpuUsage    float64
	memoryUsage uint64
	goroutines  int

	// Metrics
	cpuGauge       prometheus.Gauge
	memoryGauge    prometheus.Gauge
	goroutineGauge prometheus.Gauge
}

// OperationMetrics tracks operation-level metrics
type OperationMetrics struct {
	// Operation counters
	operationCounts  map[string]*int64
	operationLatency map[string]*LatencyTracker
	operationErrors  map[string]*int64
	mu               sync.RWMutex
}

// LatencyTracker tracks latency statistics
type LatencyTracker struct {
	samples []float64
	mu      sync.RWMutex
}

// ConnectionPoolMetrics tracks connection pool statistics
type ConnectionPoolMetrics struct {
	totalConnections   int64
	activeConnections  int64
	waitingConnections int64
	errorCount         int64
	mu                 sync.RWMutex
}

// NewMonitoringSystem creates a new monitoring system
func NewMonitoringSystem(config MonitoringConfig) (*MonitoringSystem, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize logger
	logger, err := initializeLogger(config)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Note: OpenTelemetry integration can be added here when dependencies are available
	// For now, we focus on Prometheus metrics and structured logging

	// Create custom registry for isolated metrics
	registry := prometheus.NewRegistry()

	// Initialize Prometheus metrics
	promMetrics := initializePrometheusMetrics(config, registry)

	// Initialize alert manager
	alertManager := &AlertManager{
		thresholds:   config.AlertThresholds,
		notifiers:    config.AlertNotifiers,
		activeAlerts: make(map[string]*Alert),
		alertHistory: make([]Alert, 0),
		maxHistory:   DefaultMaxAlertHistory,
	}
	// Initialize operation metrics
	operationMetrics := &OperationMetrics{
		operationCounts:  make(map[string]*int64),
		operationLatency: make(map[string]*LatencyTracker),
		operationErrors:  make(map[string]*int64),
	}

	// Initialize connection pool metrics
	connectionPoolMetrics := &ConnectionPoolMetrics{}

	ms := &MonitoringSystem{
		config:                config,
		logger:                logger,
		promMetrics:           promMetrics,
		registry:              registry,
		healthChecks:          make(map[string]HealthCheck),
		alertManager:          alertManager,
		ctx:                   ctx,
		cancel:                cancel,
		operationMetrics:      operationMetrics,
		connectionPoolMetrics: connectionPoolMetrics,
	}

	// Initialize resource monitor
	resourceMonitor := &ResourceMonitor{
		cpuGauge:       ms.createCPUGauge(),
		memoryGauge:    ms.createMemoryGauge(),
		goroutineGauge: ms.createGoroutineGauge(),
	}

	ms.resourceMonitor = resourceMonitor

	// Start background monitoring
	if config.EnableResourceMonitoring {
		ms.startResourceMonitoring()
	}

	if config.EnableHealthChecks {
		ms.startHealthChecks()
	}

	if config.MetricsEnabled {
		ms.startMetricsCollection()
	}

	return ms, nil
}

// Logger returns the structured logger
func (ms *MonitoringSystem) Logger() *zap.Logger {
	return ms.logger
}

// Registry returns the Prometheus registry for this monitoring system instance
func (ms *MonitoringSystem) Registry() *prometheus.Registry {
	return ms.registry
}

// StartTrace starts a new trace span (placeholder for OpenTelemetry integration)
func (ms *MonitoringSystem) StartTrace(ctx context.Context, name string) (context.Context, interface{}) {
	// Placeholder for OpenTelemetry integration
	// For now, just return the original context and a nil span
	return ctx, nil
}

// RecordStateOperation records a state operation
func (ms *MonitoringSystem) RecordStateOperation(operation string, duration time.Duration, err error) {
	// Prometheus metrics
	ms.promMetrics.StateOperationsTotal.WithLabelValues(operation).Inc()
	ms.promMetrics.StateOperationDuration.WithLabelValues(operation).Observe(duration.Seconds())

	if err != nil {
		ms.promMetrics.StateOperationErrors.WithLabelValues(operation, categorizeMonitoringError(err)).Inc()
	}

	// Operation metrics
	ms.operationMetrics.recordOperation(operation, duration, err)

	// Log significant operations
	if duration > DefaultSlowOperationThreshold || err != nil {
		fields := []zap.Field{
			zap.String("operation", operation),
			zap.Duration("duration", duration),
		}

		if err != nil {
			fields = append(fields, zap.Error(err))
			ms.logger.Error("State operation failed", fields...)
		} else {
			ms.logger.Info("Slow state operation", fields...)
		}
	}

	// Check thresholds for alerts
	ms.checkOperationThresholds(operation, duration, err)
}

// RecordEventProcessing records event processing metrics
func (ms *MonitoringSystem) RecordEventProcessing(eventType string, duration time.Duration, err error) {
	ms.promMetrics.EventsProcessed.WithLabelValues(eventType).Inc()
	ms.promMetrics.EventProcessingLatency.WithLabelValues(eventType).Observe(duration.Seconds())

	if err != nil {
		ms.logger.Error("Event processing failed",
			zap.String("event_type", eventType),
			zap.Duration("duration", duration),
			zap.Error(err))
	}
}

// RecordMemoryUsage records memory usage metrics
func (ms *MonitoringSystem) RecordMemoryUsage(usage uint64, allocations int64, gcPause time.Duration) {
	// Update Prometheus metrics
	ms.promMetrics.MemoryUsage.Set(float64(usage))
	
	// Ensure allocations is non-negative to avoid Prometheus counter panic
	if allocations > 0 {
		ms.promMetrics.MemoryAllocations.Add(float64(allocations))
	}
	
	ms.promMetrics.GCPauseDuration.Observe(gcPause.Seconds())

	// Update resource monitor with proper locking and timestamp
	ms.resourceMonitor.mu.Lock()
	ms.resourceMonitor.memoryUsage = usage
	ms.resourceMonitor.lastSample = time.Now()
	ms.resourceMonitor.mu.Unlock()

	// Check memory thresholds
	ms.checkMemoryThresholds(usage, gcPause)
}

// RecordConnectionPoolStats records connection pool statistics
func (ms *MonitoringSystem) RecordConnectionPoolStats(total, active, waiting int64, errors int64) {
	// Ensure values are non-negative for gauges (can be set to any value)
	if total >= 0 {
		ms.promMetrics.ConnectionPoolSize.Set(float64(total))
	}
	if active >= 0 {
		ms.promMetrics.ConnectionPoolActive.Set(float64(active))
	}
	if waiting >= 0 {
		ms.promMetrics.ConnectionPoolWaiting.Set(float64(waiting))
	}
	
	// Ensure errors is non-negative to avoid Prometheus counter panic
	if errors > 0 {
		ms.promMetrics.ConnectionPoolErrors.Add(float64(errors))
	}

	// Update internal metrics
	ms.connectionPoolMetrics.mu.Lock()
	ms.connectionPoolMetrics.totalConnections = total
	ms.connectionPoolMetrics.activeConnections = active
	ms.connectionPoolMetrics.waitingConnections = waiting
	ms.connectionPoolMetrics.errorCount += errors
	ms.connectionPoolMetrics.mu.Unlock()

	// Check thresholds
	if total > 0 {
		utilization := float64(active) / float64(total) * 100
		ms.checkConnectionPoolThresholds(utilization, errors)
	}
}

// RecordRateLimitStats records rate limiting statistics
func (ms *MonitoringSystem) RecordRateLimitStats(requests, rejects int64, utilization float64) {
	ms.promMetrics.RateLimitRequests.WithLabelValues("allowed").Add(float64(requests - rejects))
	ms.promMetrics.RateLimitRejects.WithLabelValues("rejected").Add(float64(rejects))
	ms.promMetrics.RateLimitUtilization.Set(utilization)

	// Check thresholds
	ms.checkRateLimitThresholds(rejects, utilization)
}

// RecordQueueDepth records queue depth metrics
func (ms *MonitoringSystem) RecordQueueDepth(depth int64) {
	ms.promMetrics.EventQueueDepth.Set(float64(depth))

	// Check thresholds
	if depth > ms.config.AlertThresholds.QueueDepth {
		ms.sendAlert(Alert{
			Level:       AlertLevelWarning,
			Title:       "High Queue Depth",
			Description: fmt.Sprintf("Queue depth (%d) exceeds threshold (%d)", depth, ms.config.AlertThresholds.QueueDepth),
			Timestamp:   time.Now(),
			Component:   "queue",
			Value:       float64(depth),
			Threshold:   float64(ms.config.AlertThresholds.QueueDepth),
		})
	}
}

// RegisterHealthCheck registers a health check
func (ms *MonitoringSystem) RegisterHealthCheck(check HealthCheck) {
	ms.healthMu.Lock()
	defer ms.healthMu.Unlock()
	ms.healthChecks[check.Name()] = check
}

// UnregisterHealthCheck removes a health check
func (ms *MonitoringSystem) UnregisterHealthCheck(name string) {
	ms.healthMu.Lock()
	defer ms.healthMu.Unlock()
	delete(ms.healthChecks, name)
}

// GetHealthStatus returns the current health status
func (ms *MonitoringSystem) GetHealthStatus() map[string]bool {
	ms.healthMu.RLock()
	defer ms.healthMu.RUnlock()

	status := make(map[string]bool)
	for name, check := range ms.healthChecks {
		ctx, cancel := context.WithTimeout(ms.ctx, ms.config.HealthCheckTimeout)
		defer cancel()
		
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("health check panicked: %v", r)
				}
			}()
			err = check.Check(ctx)
		}()

		status[name] = err == nil
		ms.promMetrics.HealthCheckStatus.WithLabelValues(name).Set(boolToFloat(err == nil))
	}

	return status
}

// SetAuditManager sets the audit manager for integration
func (ms *MonitoringSystem) SetAuditManager(auditManager AuditEventLogger) {
	ms.auditManager = auditManager
}

// LogAuditEvent logs an audit event with monitoring correlation
func (ms *MonitoringSystem) LogAuditEvent(ctx context.Context, action AuditAction, details map[string]interface{}) {
	if ms.auditManager == nil {
		return
	}

	// Add monitoring correlation ID (placeholder for tracing integration)
	if details == nil {
		details = make(map[string]interface{})
	}
	details["monitoring_timestamp"] = time.Now().Unix()

	// Log to the audit manager
	contextID := ""
	userID := ""
	resource := "monitoring"
	
	if details != nil {
		if ctx, ok := details["context_id"].(string); ok {
			contextID = ctx
		}
		if user, ok := details["user_id"].(string); ok {
			userID = user
		}
		if res, ok := details["resource"].(string); ok {
			resource = res
		}
	}
	
	ms.auditManager.LogSecurityEvent(ctx, action, contextID, userID, resource, details)

	// Log based on severity
	switch ms.config.AuditSeverityLevel {
	case AuditSeverityDebug:
		ms.logger.Debug("Audit event", zap.String("action", string(action)), zap.Any("details", details))
	case AuditSeverityInfo:
		ms.logger.Info("Audit event", zap.String("action", string(action)), zap.Any("details", details))
	case AuditSeverityWarning:
		ms.logger.Warn("Audit event", zap.String("action", string(action)), zap.Any("details", details))
	case AuditSeverityError:
		ms.logger.Error("Audit event", zap.String("action", string(action)), zap.Any("details", details))
	case AuditSeverityCritical:
		ms.logger.Error("Critical audit event", zap.String("action", string(action)), zap.Any("details", details))
	}

	// Record audit metrics
	ms.promMetrics.AuditLogsWritten.WithLabelValues(string(action)).Inc()
}

// GetMetrics returns current metrics snapshot
func (ms *MonitoringSystem) GetMetrics() MonitoringMetrics {
	ms.operationMetrics.mu.RLock()
	defer ms.operationMetrics.mu.RUnlock()

	ms.connectionPoolMetrics.mu.RLock()
	defer ms.connectionPoolMetrics.mu.RUnlock()

	ms.resourceMonitor.mu.RLock()
	defer ms.resourceMonitor.mu.RUnlock()

	return MonitoringMetrics{
		Timestamp:  time.Now(),
		Operations: ms.getOperationMetrics(),
		Memory: MemoryMetrics{
			Usage:      ms.resourceMonitor.memoryUsage,
			Goroutines: ms.resourceMonitor.goroutines,
		},
		ConnectionPool: ConnectionPoolSnapshot{
			TotalConnections:   ms.connectionPoolMetrics.totalConnections,
			ActiveConnections:  ms.connectionPoolMetrics.activeConnections,
			WaitingConnections: ms.connectionPoolMetrics.waitingConnections,
			ErrorCount:         ms.connectionPoolMetrics.errorCount,
		},
		Health: ms.GetHealthStatus(),
	}
}

// MonitoringMetrics contains a snapshot of all metrics
type MonitoringMetrics struct {
	Timestamp      time.Time
	Operations     map[string]OperationMetric
	Memory         MemoryMetrics
	ConnectionPool ConnectionPoolSnapshot
	Health         map[string]bool
}

// ConnectionPoolSnapshot is a snapshot of connection pool metrics without mutex
type ConnectionPoolSnapshot struct {
	TotalConnections   int64
	ActiveConnections  int64
	WaitingConnections int64
	ErrorCount         int64
}

// OperationMetric contains metrics for a specific operation
type OperationMetric struct {
	Count      int64
	AvgLatency time.Duration
	P95Latency time.Duration
	P99Latency time.Duration
	ErrorRate  float64
}

// MemoryMetrics contains memory-related metrics
type MemoryMetrics struct {
	Usage      uint64
	Goroutines int
}

// Shutdown gracefully shuts down the monitoring system
func (ms *MonitoringSystem) Shutdown(ctx context.Context) error {
	ms.logger.Info("Shutting down monitoring system")

	// Cancel background processes
	ms.cancel()

	// Wait for background goroutines with timeout
	done := make(chan struct{})
	go func() {
		ms.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		ms.logger.Info("Monitoring system shut down successfully")
	case <-ctx.Done():
		ms.logger.Warn("Monitoring system shutdown timed out")
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

// Helper functions

func initializeLogger(config MonitoringConfig) (*zap.Logger, error) {
	// Configure logger based on config
	zapConfig := zap.Config{
		Level:    zap.NewAtomicLevelAt(config.LogLevel),
		Encoding: config.LogFormat,
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
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	if config.LogSampling {
		zapConfig.Sampling = &zap.SamplingConfig{
			Initial:    DefaultLogSamplingInitial,
			Thereafter: DefaultLogSamplingThereafter,
		}
	}

	return zapConfig.Build()
}

// safeRegister safely registers a Prometheus collector, handling duplicate registrations
func safeRegister(registry *prometheus.Registry, collector prometheus.Collector) {
	err := registry.Register(collector)
	if err != nil {
		// Check if it's a duplicate registration error
		if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
			// Metric already registered, this is fine in test scenarios
			return
		}
		// For other errors, panic as before
		panic(err)
	}
}
func initializePrometheusMetrics(config MonitoringConfig, registry *prometheus.Registry) *PrometheusMetrics {
	namespace := config.PrometheusNamespace
	subsystem := config.PrometheusSubsystem

	// State operation metrics
	stateOperationsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "state_operations_total",
			Help:      "Total number of state operations",
		},
		[]string{"operation"},
	)
	safeRegister(registry, stateOperationsTotal)

	stateOperationDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "state_operation_duration_seconds",
			Help:      "Duration of state operations",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
	safeRegister(registry, stateOperationDuration)

	stateOperationErrors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "state_operation_errors_total",
			Help:      "Total number of state operation errors",
		},
		[]string{"operation", "error_type"},
	)
	safeRegister(registry, stateOperationErrors)

	// Memory metrics
	memoryUsage := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "memory_usage_bytes",
			Help:      "Current memory usage in bytes",
		},
	)
	safeRegister(registry, memoryUsage)

	memoryAllocations := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "memory_allocations_total",
			Help:      "Total number of memory allocations",
		},
	)
	safeRegister(registry, memoryAllocations)

	gcPauseDuration := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "gc_pause_duration_seconds",
			Help:      "Duration of garbage collection pauses",
			Buckets:   prometheus.DefBuckets,
		},
	)
	safeRegister(registry, gcPauseDuration)

	objectPoolHitRate := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "object_pool_hit_rate",
			Help:      "Object pool hit rate percentage",
		},
	)
	safeRegister(registry, objectPoolHitRate)

	// Event processing metrics
	eventsProcessed := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "events_processed_total",
			Help:      "Total number of events processed",
		},
		[]string{"event_type"},
	)
	safeRegister(registry, eventsProcessed)

	eventProcessingLatency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "event_processing_latency_seconds",
			Help:      "Event processing latency",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)
	safeRegister(registry, eventProcessingLatency)

	eventQueueDepth := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "event_queue_depth",
			Help:      "Current event queue depth",
		},
	)
	safeRegister(registry, eventQueueDepth)

	// Storage backend metrics
	storageOperations := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "storage_operations_total",
			Help:      "Total number of storage operations",
		},
		[]string{"operation"},
	)
	safeRegister(registry, storageOperations)

	storageLatency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "storage_latency_seconds",
			Help:      "Storage operation latency",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
	safeRegister(registry, storageLatency)

	storageErrors := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "storage_errors_total",
			Help:      "Total number of storage errors",
		},
		[]string{"operation", "error_type"},
	)
	safeRegister(registry, storageErrors)

	// Connection pool metrics
	connectionPoolSize := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "connection_pool_size",
			Help:      "Current connection pool size",
		},
	)
	safeRegister(registry, connectionPoolSize)

	connectionPoolActive := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "connection_pool_active",
			Help:      "Number of active connections in pool",
		},
	)
	safeRegister(registry, connectionPoolActive)

	connectionPoolWaiting := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "connection_pool_waiting",
			Help:      "Number of waiting connections in pool",
		},
	)
	safeRegister(registry, connectionPoolWaiting)

	connectionPoolErrors := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "connection_pool_errors_total",
			Help:      "Total number of connection pool errors",
		},
	)
	safeRegister(registry, connectionPoolErrors)

	// Rate limiting metrics
	rateLimitRequests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "rate_limit_requests_total",
			Help:      "Total number of rate limit requests",
		},
		[]string{"status"},
	)
	safeRegister(registry, rateLimitRequests)

	rateLimitRejects := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "rate_limit_rejects_total",
			Help:      "Total number of rate limit rejects",
		},
		[]string{"status"},
	)
	safeRegister(registry, rateLimitRejects)

	rateLimitUtilization := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "rate_limit_utilization",
			Help:      "Rate limit utilization percentage",
		},
	)
	safeRegister(registry, rateLimitUtilization)

	// Health check metrics
	healthCheckStatus := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "health_check_status",
			Help:      "Health check status (1 = healthy, 0 = unhealthy)",
		},
		[]string{"check_name"},
	)
	safeRegister(registry, healthCheckStatus)

	healthCheckDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "health_check_duration_seconds",
			Help:      "Health check duration",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"check_name"},
	)
	safeRegister(registry, healthCheckDuration)

	// Audit metrics
	auditLogsWritten := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "audit_logs_written_total",
			Help:      "Total number of audit logs written",
		},
		[]string{"action"},
	)
	safeRegister(registry, auditLogsWritten)

	auditLogErrors := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "audit_log_errors_total",
			Help:      "Total number of audit log errors",
		},
	)
	safeRegister(registry, auditLogErrors)

	auditVerificationTime := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "audit_verification_time_seconds",
			Help:      "Time taken to verify audit logs",
			Buckets:   prometheus.DefBuckets,
		},
	)
	safeRegister(registry, auditVerificationTime)

	return &PrometheusMetrics{
		StateOperationsTotal:   stateOperationsTotal,
		StateOperationDuration: stateOperationDuration,
		StateOperationErrors:   stateOperationErrors,
		MemoryUsage:            memoryUsage,
		MemoryAllocations:      memoryAllocations,
		GCPauseDuration:        gcPauseDuration,
		ObjectPoolHitRate:      objectPoolHitRate,
		EventsProcessed:        eventsProcessed,
		EventProcessingLatency: eventProcessingLatency,
		EventQueueDepth:        eventQueueDepth,
		StorageOperations:      storageOperations,
		StorageLatency:         storageLatency,
		StorageErrors:          storageErrors,
		ConnectionPoolSize:     connectionPoolSize,
		ConnectionPoolActive:   connectionPoolActive,
		ConnectionPoolWaiting:  connectionPoolWaiting,
		ConnectionPoolErrors:   connectionPoolErrors,
		RateLimitRequests:      rateLimitRequests,
		RateLimitRejects:       rateLimitRejects,
		RateLimitUtilization:   rateLimitUtilization,
		HealthCheckStatus:      healthCheckStatus,
		HealthCheckDuration:    healthCheckDuration,
		AuditLogsWritten:       auditLogsWritten,
		AuditLogErrors:         auditLogErrors,
		AuditVerificationTime:  auditVerificationTime,
	}
}

func (ms *MonitoringSystem) createCPUGauge() prometheus.Gauge {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: ms.config.PrometheusNamespace,
		Subsystem: ms.config.PrometheusSubsystem,
		Name:      "cpu_usage_percent",
		Help:      "Current CPU usage percentage",
	})
	safeRegister(ms.registry, gauge)
	return gauge
}

func (ms *MonitoringSystem) createMemoryGauge() prometheus.Gauge {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: ms.config.PrometheusNamespace,
		Subsystem: ms.config.PrometheusSubsystem,
		Name:      "memory_usage_bytes_resource",
		Help:      "Current memory usage in bytes (resource monitor)",
	})
	safeRegister(ms.registry, gauge)
	return gauge
}

func (ms *MonitoringSystem) createGoroutineGauge() prometheus.Gauge {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: ms.config.PrometheusNamespace,
		Subsystem: ms.config.PrometheusSubsystem,
		Name:      "goroutines_count",
		Help:      "Current number of goroutines",
	})
	safeRegister(ms.registry, gauge)
	return gauge
}

func (ms *MonitoringSystem) startResourceMonitoring() {
	ms.wg.Add(1)
	go func() {
		defer ms.wg.Done()

		// Validate interval before creating ticker
		interval := ms.config.ResourceSampleInterval
		if interval <= 0 {
			interval = 1 * time.Second // Use a reasonable default
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ms.collectResourceMetrics()
			case <-ms.ctx.Done():
				return
			}
		}
	}()
}

func (ms *MonitoringSystem) startHealthChecks() {
	ms.wg.Add(1)
	go func() {
		defer ms.wg.Done()

		// Validate interval before creating ticker
		interval := ms.config.HealthCheckInterval
		if interval <= 0 {
			interval = 5 * time.Second // Use a reasonable default
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ms.runHealthChecks()
			case <-ms.ctx.Done():
				return
			}
		}
	}()
}

func (ms *MonitoringSystem) startMetricsCollection() {
	ms.wg.Add(1)
	go func() {
		defer ms.wg.Done()

		// Validate interval before creating ticker
		interval := ms.config.MetricsInterval
		if interval <= 0 {
			interval = DefaultMetricsInterval // Use default interval
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ms.collectMetrics()
			case <-ms.ctx.Done():
				return
			}
		}
	}()
}

func (ms *MonitoringSystem) collectResourceMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	ms.resourceMonitor.mu.Lock()
	ms.resourceMonitor.memoryUsage = memStats.Alloc
	ms.resourceMonitor.goroutines = runtime.NumGoroutine()
	ms.resourceMonitor.lastSample = time.Now()
	ms.resourceMonitor.mu.Unlock()

	// Update Prometheus metrics (both main metrics and resource monitor gauges)
	ms.promMetrics.MemoryUsage.Set(float64(memStats.Alloc))
	ms.resourceMonitor.memoryGauge.Set(float64(memStats.Alloc))
	ms.resourceMonitor.goroutineGauge.Set(float64(runtime.NumGoroutine()))
}

func (ms *MonitoringSystem) runHealthChecks() {
	ms.healthMu.RLock()
	defer ms.healthMu.RUnlock()

	// Create a local wait group for this batch of health checks
	var localWG sync.WaitGroup
	
	for name, check := range ms.healthChecks {
		// Check if we're shutting down before launching new goroutines
		select {
		case <-ms.ctx.Done():
			return
		default:
		}
		
		localWG.Add(1)
		go func(name string, check HealthCheck) {
			defer localWG.Done()
			
			// Check if context is cancelled before proceeding
			select {
			case <-ms.ctx.Done():
				return
			default:
			}
			
			start := time.Now()
			// Ensure health check timeout is reasonable and doesn't exceed shutdown timeout
			timeout := ms.config.HealthCheckTimeout
			if timeout <= 0 || timeout > 30*time.Second {
				timeout = 5 * time.Second // Reasonable default
			}
			
			ctx, cancel := context.WithTimeout(ms.ctx, timeout)
			defer cancel()

			var err error
			
			// Use a channel to handle timeouts more reliably
			done := make(chan bool, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("health check panicked: %v", r)
					}
					done <- true
				}()
				err = check.Check(ctx)
			}()
			
			// Wait for health check completion or timeout
			select {
			case <-done:
				// Health check completed
			case <-ctx.Done():
				// Context cancelled or timed out
				if ctx.Err() != nil {
					err = fmt.Errorf("health check timed out: %w", ctx.Err())
				}
			case <-ms.ctx.Done():
				// System shutting down
				return
			}
			
			duration := time.Since(start)

			// Record metrics only if we haven't shut down
			select {
			case <-ms.ctx.Done():
				return
			default:
				ms.promMetrics.HealthCheckDuration.WithLabelValues(name).Observe(duration.Seconds())
				ms.promMetrics.HealthCheckStatus.WithLabelValues(name).Set(boolToFloat(err == nil))

				if err != nil {
					// Only log health check failures at debug level for tests to reduce noise
					ms.logger.Debug("Health check failed",
						zap.String("check_name", name),
						zap.Duration("duration", duration),
						zap.Error(err))
				}
			}
		}(name, check)
	}
	
	// Wait for all health checks to complete with a timeout
	done := make(chan struct{})
	go func() {
		localWG.Wait()
		close(done)
	}()
	
	// Wait for completion or system shutdown
	select {
	case <-done:
		// All health checks completed
	case <-ms.ctx.Done():
		// System is shutting down, don't wait any longer
		return
	case <-time.After(ms.config.HealthCheckTimeout + 5*time.Second):
		// Extra timeout buffer to prevent hanging
		ms.logger.Warn("Health checks taking too long, continuing without waiting")
		return
	}
}

func (ms *MonitoringSystem) collectMetrics() {
	// This is where you would collect custom metrics
	// For example, from the performance optimizer
	ms.logger.Debug("Collecting metrics")
}

func (ms *MonitoringSystem) checkOperationThresholds(operation string, duration time.Duration, err error) {
	latencyMs := float64(duration.Nanoseconds()) / 1e6

	if latencyMs > ms.config.AlertThresholds.P95LatencyMs {
		ms.sendAlert(Alert{
			Level:       AlertLevelWarning,
			Title:       "High Operation Latency",
			Description: fmt.Sprintf("Operation %s took %.2fms, exceeding P95 threshold", operation, latencyMs),
			Timestamp:   time.Now(),
			Component:   "operation",
			Value:       latencyMs,
			Threshold:   ms.config.AlertThresholds.P95LatencyMs,
		})
	}
}

func (ms *MonitoringSystem) checkMemoryThresholds(usage uint64, gcPause time.Duration) {
	gcPauseMs := float64(gcPause.Nanoseconds()) / 1e6

	if gcPauseMs > ms.config.AlertThresholds.GCPauseMs {
		ms.sendAlert(Alert{
			Level:       AlertLevelWarning,
			Title:       "High GC Pause",
			Description: fmt.Sprintf("GC pause (%.2fms) exceeds threshold", gcPauseMs),
			Timestamp:   time.Now(),
			Component:   "memory",
			Value:       gcPauseMs,
			Threshold:   ms.config.AlertThresholds.GCPauseMs,
		})
	}
}

func (ms *MonitoringSystem) checkConnectionPoolThresholds(utilization float64, errors int64) {
	if utilization > ms.config.AlertThresholds.ConnectionPoolUtil {
		ms.sendAlert(Alert{
			Level:       AlertLevelWarning,
			Title:       "High Connection Pool Utilization",
			Description: fmt.Sprintf("Connection pool utilization (%.2f%%) exceeds threshold", utilization),
			Timestamp:   time.Now(),
			Component:   "connection_pool",
			Value:       utilization,
			Threshold:   ms.config.AlertThresholds.ConnectionPoolUtil,
		})
	}
}

func (ms *MonitoringSystem) checkRateLimitThresholds(rejects int64, utilization float64) {
	if rejects > ms.config.AlertThresholds.RateLimitRejects {
		ms.sendAlert(Alert{
			Level:       AlertLevelWarning,
			Title:       "High Rate Limit Rejects",
			Description: fmt.Sprintf("Rate limit rejects (%d) exceeds threshold", rejects),
			Timestamp:   time.Now(),
			Component:   "rate_limit",
			Value:       float64(rejects),
			Threshold:   float64(ms.config.AlertThresholds.RateLimitRejects),
		})
	}
}

func (ms *MonitoringSystem) sendAlert(alert Alert) {
	ms.alertManager.mu.Lock()
	defer ms.alertManager.mu.Unlock()

	// Check if this is a duplicate alert
	alertKey := fmt.Sprintf("%s_%s", alert.Component, alert.Title)
	if existing, exists := ms.alertManager.activeAlerts[alertKey]; exists {
		// If the alert is recent, don't send again
		if time.Since(existing.Timestamp) < DefaultDuplicateAlertWindow {
			return
		}
	}

	// Store the alert
	ms.alertManager.activeAlerts[alertKey] = &alert
	ms.alertManager.alertHistory = append(ms.alertManager.alertHistory, alert)

	// Trim history if needed
	if len(ms.alertManager.alertHistory) > ms.alertManager.maxHistory {
		ms.alertManager.alertHistory = ms.alertManager.alertHistory[1:]
	}

	// Log the alert
	ms.logger.Warn("Alert triggered",
		zap.String("title", alert.Title),
		zap.String("description", alert.Description),
		zap.String("component", alert.Component),
		zap.Float64("value", alert.Value),
		zap.Float64("threshold", alert.Threshold))

	// Send to notifiers
	for _, notifier := range ms.alertManager.notifiers {
		// Check if system is shutting down before launching goroutine
		select {
		case <-ms.ctx.Done():
			return
		default:
		}
		
		ms.wg.Add(1)
		go func(notifier AlertNotifier) {
			defer ms.wg.Done()
			
			// Use monitoring system context instead of background context
			ctx, cancel := context.WithTimeout(ms.ctx, 5*time.Second)
			defer cancel()
			
			// Check again if system is shutting down
			select {
			case <-ms.ctx.Done():
				return
			default:
			}
			
			if err := notifier.SendAlert(ctx, alert); err != nil {
				ms.logger.Error("Failed to send alert", zap.Error(err))
			}
		}(notifier)
	}
}

func (om *OperationMetrics) recordOperation(operation string, duration time.Duration, err error) {
	om.mu.Lock()
	defer om.mu.Unlock()

	// Initialize if needed
	if om.operationCounts[operation] == nil {
		count := int64(0)
		om.operationCounts[operation] = &count
	}
	if om.operationLatency[operation] == nil {
		om.operationLatency[operation] = &LatencyTracker{}
	}
	if om.operationErrors[operation] == nil {
		errorCount := int64(0)
		om.operationErrors[operation] = &errorCount
	}

	// Update metrics
	atomic.AddInt64(om.operationCounts[operation], 1)
	om.operationLatency[operation].addSample(duration.Seconds())

	if err != nil {
		atomic.AddInt64(om.operationErrors[operation], 1)
	}
}

func (lt *LatencyTracker) addSample(latency float64) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	lt.samples = append(lt.samples, latency)

	// Keep only last samples within limit
	if len(lt.samples) > DefaultLatencySampleSize {
		lt.samples = lt.samples[1:]
	}
}

func (ms *MonitoringSystem) getOperationMetrics() map[string]OperationMetric {
	metrics := make(map[string]OperationMetric)

	for operation, count := range ms.operationMetrics.operationCounts {
		latencyTracker := ms.operationMetrics.operationLatency[operation]
		errorCount := ms.operationMetrics.operationErrors[operation]

		metric := OperationMetric{
			Count: atomic.LoadInt64(count),
		}

		if latencyTracker != nil {
			latencyTracker.mu.RLock()
			if len(latencyTracker.samples) > 0 {
				// Calculate percentiles
				samples := make([]float64, len(latencyTracker.samples))
				copy(samples, latencyTracker.samples)

				// Simple percentile calculation
				if len(samples) > 0 {
					sum := 0.0
					for _, sample := range samples {
						sum += sample
					}
					metric.AvgLatency = time.Duration(sum / float64(len(samples)) * 1e9)

					// For P95 and P99, we'd need to sort the samples
					// This is a simplified version
					metric.P95Latency = metric.AvgLatency
					metric.P99Latency = metric.AvgLatency
				}
			}
			latencyTracker.mu.RUnlock()
		}

		if errorCount != nil {
			errors := atomic.LoadInt64(errorCount)
			if metric.Count > 0 {
				metric.ErrorRate = float64(errors) / float64(metric.Count) * 100
			}
		}

		metrics[operation] = metric
	}

	return metrics
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

func categorizeMonitoringError(err error) string {
	if err == nil {
		return "none"
	}

	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "timeout"):
		return "timeout"
	case strings.Contains(errStr, "connection"):
		return "connection"
	case strings.Contains(errStr, "validation"):
		return "validation"
	case strings.Contains(errStr, "conflict"):
		return "conflict"
	case strings.Contains(errStr, "rate limit"):
		return "rate_limit"
	case strings.Contains(errStr, "storage"):
		return "storage"
	default:
		return "other"
	}
}
