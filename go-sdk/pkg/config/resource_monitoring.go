// Package config provides comprehensive monitoring and metrics for resource usage
// to help detect potential DoS attacks and resource exhaustion scenarios.
package config

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector collects and aggregates resource usage metrics
type MetricsCollector struct {
	// Basic counters (atomic for thread safety)
	OperationCount   int64 `json:"operation_count"`
	ErrorCount       int64 `json:"error_count"`
	LoadCount        int64 `json:"load_count"`
	ReloadCount      int64 `json:"reload_count"`
	ValidationCount  int64 `json:"validation_count"`
	WatcherCount     int64 `json:"watcher_count"`
	
	// Timing metrics (stored as nanoseconds, atomic)
	TotalLoadTime       int64 `json:"total_load_time_ns"`
	TotalValidationTime int64 `json:"total_validation_time_ns"`
	TotalProcessingTime int64 `json:"total_processing_time_ns"`
	
	// Resource usage metrics (atomic)
	PeakMemoryUsage     int64 `json:"peak_memory_usage"`
	PeakFileSize        int64 `json:"peak_file_size"`
	PeakNestingDepth    int64 `json:"peak_nesting_depth"`
	PeakKeyCount        int64 `json:"peak_key_count"`
	PeakWatcherCount    int64 `json:"peak_watcher_count"`
	
	// Rate limiting metrics (atomic)
	RateLimitHits       int64 `json:"rate_limit_hits"`
	ResourceLimitHits   int64 `json:"resource_limit_hits"`
	StructureLimitHits  int64 `json:"structure_limit_hits"`
	TimeoutHits         int64 `json:"timeout_hits"`
	SecurityViolations  int64 `json:"security_violations"`
	
	// Historical data
	mu             sync.RWMutex
	startTime      time.Time
	lastResetTime  time.Time
	histogramData  map[string]*Histogram
	alertHistory   []AlertEvent
	
	// Alert thresholds
	alertThresholds AlertThresholds
}

// Histogram tracks distribution of values over time
type Histogram struct {
	Buckets []HistogramBucket `json:"buckets"`
	Count   int64             `json:"count"`
	Sum     int64             `json:"sum"`
	Min     int64             `json:"min"`
	Max     int64             `json:"max"`
}

// HistogramBucket represents a bucket in the histogram
type HistogramBucket struct {
	UpperBound int64 `json:"upper_bound"`
	Count      int64 `json:"count"`
}

// AlertEvent represents a monitoring alert
type AlertEvent struct {
	Timestamp   time.Time   `json:"timestamp"`
	Severity    string      `json:"severity"`
	Type        string      `json:"type"`
	Message     string      `json:"message"`
	Details     interface{} `json:"details"`
	Resolved    bool        `json:"resolved"`
	ResolvedAt  time.Time   `json:"resolved_at,omitempty"`
}

// AlertThresholds defines thresholds for triggering alerts
type AlertThresholds struct {
	MaxMemoryUsage      int64         `json:"max_memory_usage"`
	MaxLoadTime         time.Duration `json:"max_load_time"`
	MaxValidationTime   time.Duration `json:"max_validation_time"`
	MaxErrorRate        float64       `json:"max_error_rate"`        // Errors per minute
	MaxRateLimitHitRate float64       `json:"max_rate_limit_hit_rate"` // Rate limit hits per minute
	WatcherCountAlert   int64         `json:"watcher_count_alert"`
}

// DefaultAlertThresholds returns sensible default alert thresholds
func DefaultAlertThresholds() AlertThresholds {
	return AlertThresholds{
		MaxMemoryUsage:      40 * 1024 * 1024, // 40MB (80% of default 50MB limit)
		MaxLoadTime:         10 * time.Second,  // 10s (33% of default 30s timeout)
		MaxValidationTime:   5 * time.Second,   // 5s (50% of default 10s timeout)
		MaxErrorRate:        10.0,              // 10 errors per minute
		MaxRateLimitHitRate: 20.0,              // 20 rate limit hits per minute
		WatcherCountAlert:   80,                // 80 watchers (80% of default 100 limit)
	}
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		startTime:       time.Now(),
		lastResetTime:   time.Now(),
		histogramData:   make(map[string]*Histogram),
		alertHistory:    make([]AlertEvent, 0),
		alertThresholds: DefaultAlertThresholds(),
	}
}

// RecordOperation records a generic operation
func (mc *MetricsCollector) RecordOperation() {
	atomic.AddInt64(&mc.OperationCount, 1)
}

// RecordError records an error occurrence
func (mc *MetricsCollector) RecordError() {
	atomic.AddInt64(&mc.ErrorCount, 1)
}

// RecordLoad records a configuration load operation with timing
func (mc *MetricsCollector) RecordLoad(duration time.Duration) {
	atomic.AddInt64(&mc.LoadCount, 1)
	atomic.AddInt64(&mc.TotalLoadTime, int64(duration))
	
	// Check for alert conditions
	if duration > mc.alertThresholds.MaxLoadTime {
		mc.triggerAlert("high", "load_time", 
			"Configuration load time exceeded threshold", 
			map[string]interface{}{
				"duration":  duration,
				"threshold": mc.alertThresholds.MaxLoadTime,
			})
	}
	
	// Update histogram
	mc.updateHistogram("load_time", int64(duration))
}

// RecordReload records a configuration reload operation
func (mc *MetricsCollector) RecordReload() {
	atomic.AddInt64(&mc.ReloadCount, 1)
}

// RecordValidation records a validation operation with timing
func (mc *MetricsCollector) RecordValidation(duration time.Duration) {
	atomic.AddInt64(&mc.ValidationCount, 1)
	atomic.AddInt64(&mc.TotalValidationTime, int64(duration))
	
	// Check for alert conditions
	if duration > mc.alertThresholds.MaxValidationTime {
		mc.triggerAlert("medium", "validation_time",
			"Configuration validation time exceeded threshold",
			map[string]interface{}{
				"duration":  duration,
				"threshold": mc.alertThresholds.MaxValidationTime,
			})
	}
	
	// Update histogram
	mc.updateHistogram("validation_time", int64(duration))
}

// RecordWatcherAdded records a watcher addition
func (mc *MetricsCollector) RecordWatcherAdded() {
	count := atomic.AddInt64(&mc.WatcherCount, 1)
	mc.updatePeak(&mc.PeakWatcherCount, count)
	
	// Check for alert conditions
	if count > mc.alertThresholds.WatcherCountAlert {
		mc.triggerAlert("medium", "watcher_count",
			"Watcher count approaching limit",
			map[string]interface{}{
				"current":   count,
				"threshold": mc.alertThresholds.WatcherCountAlert,
			})
	}
}

// RecordWatcherRemoved records a watcher removal
func (mc *MetricsCollector) RecordWatcherRemoved() {
	if current := atomic.LoadInt64(&mc.WatcherCount); current > 0 {
		atomic.AddInt64(&mc.WatcherCount, -1)
	}
}

// RecordMemoryUsage records current memory usage
func (mc *MetricsCollector) RecordMemoryUsage(usage int64) {
	mc.updatePeak(&mc.PeakMemoryUsage, usage)
	
	// Check for alert conditions
	if usage > mc.alertThresholds.MaxMemoryUsage {
		mc.triggerAlert("high", "memory_usage",
			"Memory usage exceeded threshold",
			map[string]interface{}{
				"current":   usage,
				"threshold": mc.alertThresholds.MaxMemoryUsage,
			})
	}
}

// RecordFileSize records file size
func (mc *MetricsCollector) RecordFileSize(size int64) {
	mc.updatePeak(&mc.PeakFileSize, size)
}

// RecordNestingDepth records nesting depth
func (mc *MetricsCollector) RecordNestingDepth(depth int) {
	mc.updatePeak(&mc.PeakNestingDepth, int64(depth))
}

// RecordKeyCount records key count
func (mc *MetricsCollector) RecordKeyCount(count int) {
	mc.updatePeak(&mc.PeakKeyCount, int64(count))
}

// RecordRateLimitHit records a rate limit hit
func (mc *MetricsCollector) RecordRateLimitHit() {
	atomic.AddInt64(&mc.RateLimitHits, 1)
	mc.checkRateLimitHitRate()
}

// RecordResourceLimitHit records a resource limit hit
func (mc *MetricsCollector) RecordResourceLimitHit() {
	atomic.AddInt64(&mc.ResourceLimitHits, 1)
}

// RecordStructureLimitHit records a structure limit hit
func (mc *MetricsCollector) RecordStructureLimitHit() {
	atomic.AddInt64(&mc.StructureLimitHits, 1)
}

// RecordTimeoutHit records a timeout hit
func (mc *MetricsCollector) RecordTimeoutHit() {
	atomic.AddInt64(&mc.TimeoutHits, 1)
}

// RecordSecurityViolation records a security violation
func (mc *MetricsCollector) RecordSecurityViolation() {
	atomic.AddInt64(&mc.SecurityViolations, 1)
	
	mc.triggerAlert("critical", "security_violation",
		"Security violation detected",
		map[string]interface{}{
			"total_violations": atomic.LoadInt64(&mc.SecurityViolations),
		})
}

// GetMetrics returns a snapshot of current metrics
func (mc *MetricsCollector) GetMetrics() MetricsSnapshot {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	uptime := time.Since(mc.startTime)
	timeSinceReset := time.Since(mc.lastResetTime)
	
	// Calculate rates
	opsPerSecond := float64(atomic.LoadInt64(&mc.OperationCount)) / timeSinceReset.Seconds()
	errorRate := float64(atomic.LoadInt64(&mc.ErrorCount)) / timeSinceReset.Minutes()
	rateLimitHitRate := float64(atomic.LoadInt64(&mc.RateLimitHits)) / timeSinceReset.Minutes()
	
	// Calculate averages
	loadCount := atomic.LoadInt64(&mc.LoadCount)
	validationCount := atomic.LoadInt64(&mc.ValidationCount)
	
	var avgLoadTime, avgValidationTime time.Duration
	if loadCount > 0 {
		avgLoadTime = time.Duration(atomic.LoadInt64(&mc.TotalLoadTime) / loadCount)
	}
	if validationCount > 0 {
		avgValidationTime = time.Duration(atomic.LoadInt64(&mc.TotalValidationTime) / validationCount)
	}
	
	// Copy histogram data
	histograms := make(map[string]*Histogram)
	for k, v := range mc.histogramData {
		histograms[k] = mc.copyHistogram(v)
	}
	
	// Copy recent alerts (last 100)
	recentAlerts := make([]AlertEvent, 0)
	alertStart := len(mc.alertHistory) - 100
	if alertStart < 0 {
		alertStart = 0
	}
	for i := alertStart; i < len(mc.alertHistory); i++ {
		recentAlerts = append(recentAlerts, mc.alertHistory[i])
	}
	
	return MetricsSnapshot{
		Timestamp:           time.Now(),
		Uptime:              uptime,
		TimeSinceReset:      timeSinceReset,
		OperationCount:      atomic.LoadInt64(&mc.OperationCount),
		ErrorCount:          atomic.LoadInt64(&mc.ErrorCount),
		LoadCount:           loadCount,
		ReloadCount:         atomic.LoadInt64(&mc.ReloadCount),
		ValidationCount:     validationCount,
		WatcherCount:        atomic.LoadInt64(&mc.WatcherCount),
		PeakMemoryUsage:     atomic.LoadInt64(&mc.PeakMemoryUsage),
		PeakFileSize:        atomic.LoadInt64(&mc.PeakFileSize),
		PeakNestingDepth:    atomic.LoadInt64(&mc.PeakNestingDepth),
		PeakKeyCount:        atomic.LoadInt64(&mc.PeakKeyCount),
		PeakWatcherCount:    atomic.LoadInt64(&mc.PeakWatcherCount),
		RateLimitHits:       atomic.LoadInt64(&mc.RateLimitHits),
		ResourceLimitHits:   atomic.LoadInt64(&mc.ResourceLimitHits),
		StructureLimitHits:  atomic.LoadInt64(&mc.StructureLimitHits),
		TimeoutHits:         atomic.LoadInt64(&mc.TimeoutHits),
		SecurityViolations:  atomic.LoadInt64(&mc.SecurityViolations),
		OperationsPerSecond: opsPerSecond,
		ErrorRate:           errorRate,
		RateLimitHitRate:    rateLimitHitRate,
		AverageLoadTime:     avgLoadTime,
		AverageValidationTime: avgValidationTime,
		Histograms:          histograms,
		RecentAlerts:        recentAlerts,
		AlertThresholds:     mc.alertThresholds,
	}
}

// MetricsSnapshot represents a point-in-time snapshot of metrics
type MetricsSnapshot struct {
	Timestamp             time.Time                  `json:"timestamp"`
	Uptime                time.Duration              `json:"uptime"`
	TimeSinceReset        time.Duration              `json:"time_since_reset"`
	OperationCount        int64                      `json:"operation_count"`
	ErrorCount            int64                      `json:"error_count"`
	LoadCount             int64                      `json:"load_count"`
	ReloadCount           int64                      `json:"reload_count"`
	ValidationCount       int64                      `json:"validation_count"`
	WatcherCount          int64                      `json:"watcher_count"`
	PeakMemoryUsage       int64                      `json:"peak_memory_usage"`
	PeakFileSize          int64                      `json:"peak_file_size"`
	PeakNestingDepth      int64                      `json:"peak_nesting_depth"`
	PeakKeyCount          int64                      `json:"peak_key_count"`
	PeakWatcherCount      int64                      `json:"peak_watcher_count"`
	RateLimitHits         int64                      `json:"rate_limit_hits"`
	ResourceLimitHits     int64                      `json:"resource_limit_hits"`
	StructureLimitHits    int64                      `json:"structure_limit_hits"`
	TimeoutHits           int64                      `json:"timeout_hits"`
	SecurityViolations    int64                      `json:"security_violations"`
	OperationsPerSecond   float64                    `json:"operations_per_second"`
	ErrorRate             float64                    `json:"error_rate"`
	RateLimitHitRate      float64                    `json:"rate_limit_hit_rate"`
	AverageLoadTime       time.Duration              `json:"average_load_time"`
	AverageValidationTime time.Duration              `json:"average_validation_time"`
	Histograms            map[string]*Histogram      `json:"histograms"`
	RecentAlerts          []AlertEvent               `json:"recent_alerts"`
	AlertThresholds       AlertThresholds            `json:"alert_thresholds"`
}

// ToJSON returns the metrics snapshot as JSON
func (ms *MetricsSnapshot) ToJSON() ([]byte, error) {
	return json.MarshalIndent(ms, "", "  ")
}

// Reset resets all metrics counters but preserves peak values and history
func (mc *MetricsCollector) Reset() {
	atomic.StoreInt64(&mc.OperationCount, 0)
	atomic.StoreInt64(&mc.ErrorCount, 0)
	atomic.StoreInt64(&mc.LoadCount, 0)
	atomic.StoreInt64(&mc.ReloadCount, 0)
	atomic.StoreInt64(&mc.ValidationCount, 0)
	atomic.StoreInt64(&mc.TotalLoadTime, 0)
	atomic.StoreInt64(&mc.TotalValidationTime, 0)
	atomic.StoreInt64(&mc.TotalProcessingTime, 0)
	atomic.StoreInt64(&mc.RateLimitHits, 0)
	atomic.StoreInt64(&mc.ResourceLimitHits, 0)
	atomic.StoreInt64(&mc.StructureLimitHits, 0)
	atomic.StoreInt64(&mc.TimeoutHits, 0)
	
	mc.mu.Lock()
	mc.lastResetTime = time.Now()
	// Clear histograms
	for k := range mc.histogramData {
		delete(mc.histogramData, k)
	}
	mc.mu.Unlock()
}

// UpdateAlertThresholds updates the alert thresholds
func (mc *MetricsCollector) UpdateAlertThresholds(thresholds AlertThresholds) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.alertThresholds = thresholds
}

// GetAlerts returns recent alerts
func (mc *MetricsCollector) GetAlerts(limit int) []AlertEvent {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	if limit <= 0 || limit > len(mc.alertHistory) {
		result := make([]AlertEvent, len(mc.alertHistory))
		copy(result, mc.alertHistory)
		return result
	}
	
	start := len(mc.alertHistory) - limit
	result := make([]AlertEvent, limit)
	copy(result, mc.alertHistory[start:])
	return result
}

// Helper methods

// updatePeak atomically updates a peak value if the new value is higher
func (mc *MetricsCollector) updatePeak(peak *int64, value int64) {
	for {
		current := atomic.LoadInt64(peak)
		if value <= current {
			break
		}
		if atomic.CompareAndSwapInt64(peak, current, value) {
			break
		}
	}
}

// updateHistogram updates histogram data for a metric
func (mc *MetricsCollector) updateHistogram(metricName string, value int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	histogram := mc.histogramData[metricName]
	if histogram == nil {
		histogram = mc.createHistogram(metricName)
		mc.histogramData[metricName] = histogram
	}
	
	// Update histogram stats
	histogram.Count++
	histogram.Sum += value
	if histogram.Count == 1 {
		histogram.Min = value
		histogram.Max = value
	} else {
		if value < histogram.Min {
			histogram.Min = value
		}
		if value > histogram.Max {
			histogram.Max = value
		}
	}
	
	// Update buckets
	for i := range histogram.Buckets {
		if value <= histogram.Buckets[i].UpperBound {
			histogram.Buckets[i].Count++
			break
		}
	}
}

// createHistogram creates a new histogram for a metric
func (mc *MetricsCollector) createHistogram(metricName string) *Histogram {
	// Define different bucket sets for different metrics
	var buckets []HistogramBucket
	
	switch metricName {
	case "load_time", "validation_time":
		// Time-based buckets (nanoseconds)
		bounds := []int64{
			int64(1 * time.Millisecond),
			int64(10 * time.Millisecond),
			int64(100 * time.Millisecond),
			int64(500 * time.Millisecond),
			int64(1 * time.Second),
			int64(5 * time.Second),
			int64(10 * time.Second),
			int64(30 * time.Second),
			int64(60 * time.Second),
		}
		for _, bound := range bounds {
			buckets = append(buckets, HistogramBucket{UpperBound: bound, Count: 0})
		}
	default:
		// Generic buckets
		bounds := []int64{1, 10, 100, 1000, 10000, 100000, 1000000}
		for _, bound := range bounds {
			buckets = append(buckets, HistogramBucket{UpperBound: bound, Count: 0})
		}
	}
	
	return &Histogram{
		Buckets: buckets,
		Count:   0,
		Sum:     0,
		Min:     0,
		Max:     0,
	}
}

// copyHistogram creates a deep copy of a histogram
func (mc *MetricsCollector) copyHistogram(original *Histogram) *Histogram {
	buckets := make([]HistogramBucket, len(original.Buckets))
	copy(buckets, original.Buckets)
	
	return &Histogram{
		Buckets: buckets,
		Count:   original.Count,
		Sum:     original.Sum,
		Min:     original.Min,
		Max:     original.Max,
	}
}

// triggerAlert creates and records an alert
func (mc *MetricsCollector) triggerAlert(severity, alertType, message string, details interface{}) {
	alert := AlertEvent{
		Timestamp: time.Now(),
		Severity:  severity,
		Type:      alertType,
		Message:   message,
		Details:   details,
		Resolved:  false,
	}
	
	mc.mu.Lock()
	mc.alertHistory = append(mc.alertHistory, alert)
	
	// Keep only the last 1000 alerts to prevent unbounded growth
	if len(mc.alertHistory) > 1000 {
		mc.alertHistory = mc.alertHistory[len(mc.alertHistory)-1000:]
	}
	mc.mu.Unlock()
}

// checkRateLimitHitRate checks if rate limit hit rate exceeds threshold
func (mc *MetricsCollector) checkRateLimitHitRate() {
	mc.mu.RLock()
	timeSinceReset := time.Since(mc.lastResetTime)
	mc.mu.RUnlock()
	
	if timeSinceReset < time.Minute {
		return // Not enough data
	}
	
	hitRate := float64(atomic.LoadInt64(&mc.RateLimitHits)) / timeSinceReset.Minutes()
	if hitRate > mc.alertThresholds.MaxRateLimitHitRate {
		mc.triggerAlert("medium", "rate_limit_hit_rate",
			"Rate limit hit rate exceeded threshold",
			map[string]interface{}{
				"current":   hitRate,
				"threshold": mc.alertThresholds.MaxRateLimitHitRate,
			})
	}
}

// MonitoringContext provides monitoring capabilities for configuration operations
type MonitoringContext struct {
	metrics   *MetricsCollector
	startTime time.Time
	operation string
}

// NewMonitoringContext creates a new monitoring context for an operation
func NewMonitoringContext(metrics *MetricsCollector, operation string) *MonitoringContext {
	return &MonitoringContext{
		metrics:   metrics,
		startTime: time.Now(),
		operation: operation,
	}
}

// Complete completes the monitoring context and records metrics
func (mc *MonitoringContext) Complete(err error) {
	duration := time.Since(mc.startTime)
	
	mc.metrics.RecordOperation()
	
	if err != nil {
		mc.metrics.RecordError()
		
		// Record specific error types
		if IsResourceError(err) {
			switch err.(type) {
			case *RateLimitError:
				mc.metrics.RecordRateLimitHit()
			case *ResourceLimitError:
				mc.metrics.RecordResourceLimitHit()
			case *StructureLimitError:
				mc.metrics.RecordStructureLimitHit()
			case *TimeoutError:
				mc.metrics.RecordTimeoutHit()
			case *SecurityError:
				mc.metrics.RecordSecurityViolation()
			}
		}
	}
	
	// Record operation-specific metrics
	switch mc.operation {
	case "load":
		mc.metrics.RecordLoad(duration)
	case "reload":
		mc.metrics.RecordReload()
	case "validate":
		mc.metrics.RecordValidation(duration)
	}
}

// WithMonitoring wraps a function call with monitoring
func (mc *MetricsCollector) WithMonitoring(operation string, fn func() error) error {
	ctx := NewMonitoringContext(mc, operation)
	err := fn()
	ctx.Complete(err)
	return err
}

// HealthCheck performs a health check based on current metrics
func (mc *MetricsCollector) HealthCheck() HealthStatus {
	snapshot := mc.GetMetrics()
	status := HealthStatus{
		Timestamp: time.Now(),
		Overall:   "healthy",
		Checks:    make(map[string]CheckResult),
	}
	
	// Check error rate
	if snapshot.ErrorRate > mc.alertThresholds.MaxErrorRate {
		status.Checks["error_rate"] = CheckResult{
			Status:  "unhealthy",
			Message: "Error rate too high",
			Value:   snapshot.ErrorRate,
			Threshold: mc.alertThresholds.MaxErrorRate,
		}
		status.Overall = "unhealthy"
	} else {
		status.Checks["error_rate"] = CheckResult{
			Status:  "healthy",
			Message: "Error rate within limits",
			Value:   snapshot.ErrorRate,
			Threshold: mc.alertThresholds.MaxErrorRate,
		}
	}
	
	// Check memory usage
	if snapshot.PeakMemoryUsage > mc.alertThresholds.MaxMemoryUsage {
		status.Checks["memory_usage"] = CheckResult{
			Status:  "degraded",
			Message: "Memory usage high",
			Value:   float64(snapshot.PeakMemoryUsage),
			Threshold: float64(mc.alertThresholds.MaxMemoryUsage),
		}
		if status.Overall == "healthy" {
			status.Overall = "degraded"
		}
	} else {
		status.Checks["memory_usage"] = CheckResult{
			Status:  "healthy",
			Message: "Memory usage within limits",
			Value:   float64(snapshot.PeakMemoryUsage),
			Threshold: float64(mc.alertThresholds.MaxMemoryUsage),
		}
	}
	
	// Check watcher count
	if snapshot.WatcherCount > mc.alertThresholds.WatcherCountAlert {
		status.Checks["watcher_count"] = CheckResult{
			Status:  "degraded",
			Message: "Watcher count high",
			Value:   float64(snapshot.WatcherCount),
			Threshold: float64(mc.alertThresholds.WatcherCountAlert),
		}
		if status.Overall == "healthy" {
			status.Overall = "degraded"
		}
	} else {
		status.Checks["watcher_count"] = CheckResult{
			Status:  "healthy",
			Message: "Watcher count within limits",
			Value:   float64(snapshot.WatcherCount),
			Threshold: float64(mc.alertThresholds.WatcherCountAlert),
		}
	}
	
	// Check for recent security violations
	if snapshot.SecurityViolations > 0 {
		status.Checks["security"] = CheckResult{
			Status:  "critical",
			Message: "Security violations detected",
			Value:   float64(snapshot.SecurityViolations),
			Threshold: 0,
		}
		status.Overall = "critical"
	} else {
		status.Checks["security"] = CheckResult{
			Status:  "healthy",
			Message: "No security violations",
			Value:   0,
			Threshold: 0,
		}
	}
	
	return status
}

// HealthStatus represents the overall health status
type HealthStatus struct {
	Timestamp time.Time              `json:"timestamp"`
	Overall   string                 `json:"overall"` // healthy, degraded, unhealthy, critical
	Checks    map[string]CheckResult `json:"checks"`
}

// CheckResult represents the result of an individual health check
type CheckResult struct {
	Status    string  `json:"status"`    // healthy, degraded, unhealthy, critical
	Message   string  `json:"message"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
}