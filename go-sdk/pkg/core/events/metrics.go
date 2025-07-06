package events

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsLevel defines the level of metrics collection
type MetricsLevel int

const (
	// MetricsLevelBasic collects only essential metrics
	MetricsLevelBasic MetricsLevel = iota
	// MetricsLevelDetailed collects detailed performance metrics
	MetricsLevelDetailed
	// MetricsLevelDebug collects all metrics including debug information
	MetricsLevelDebug
)

// String returns the string representation of the metrics level
func (l MetricsLevel) String() string {
	switch l {
	case MetricsLevelBasic:
		return "Basic"
	case MetricsLevelDetailed:
		return "Detailed"
	case MetricsLevelDebug:
		return "Debug"
	default:
		return "Unknown"
	}
}

// MetricsConfig contains configuration for metrics collection
type MetricsConfig struct {
	Level            MetricsLevel
	SamplingRate     float64       // Sampling rate for metrics collection (0.0 to 1.0)
	FlushInterval    time.Duration // How often to flush metrics
	RetentionPeriod  time.Duration // How long to retain metrics data
	MaxMemoryUsage   int64         // Maximum memory usage in bytes before cleanup
	EnableLeakDetection bool       // Enable memory leak detection
	EnableRegression    bool       // Enable performance regression detection
	DashboardEnabled    bool       // Enable real-time dashboard data
	
	// Backends configuration
	PrometheusEnabled bool
	OTLPEnabled       bool
	PrometheusPort    int
	OTLPEndpoint      string
	
	// Thresholds
	SlowRuleThreshold     time.Duration // Threshold for slow rule execution
	MemoryLeakThreshold   int64         // Memory leak detection threshold
	RegressionThreshold   float64       // Performance regression threshold (percentage)
}

// DefaultMetricsConfig returns the default metrics configuration
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		Level:               MetricsLevelBasic,
		SamplingRate:        1.0,
		FlushInterval:       time.Minute,
		RetentionPeriod:     24 * time.Hour,
		MaxMemoryUsage:      100 * 1024 * 1024, // 100MB
		EnableLeakDetection: true,
		EnableRegression:    true,
		DashboardEnabled:    true,
		PrometheusEnabled:   false,
		OTLPEnabled:        false,
		PrometheusPort:     8080,
		OTLPEndpoint:       "localhost:4317",
		SlowRuleThreshold:  100 * time.Millisecond,
		MemoryLeakThreshold: 50 * 1024 * 1024, // 50MB
		RegressionThreshold: 0.2,               // 20%
	}
}

// ProductionMetricsConfig returns a configuration suitable for production
func ProductionMetricsConfig() *MetricsConfig {
	config := DefaultMetricsConfig()
	config.Level = MetricsLevelDetailed
	config.SamplingRate = 0.1 // Sample 10% of events
	config.PrometheusEnabled = true
	config.OTLPEnabled = true
	return config
}

// DevelopmentMetricsConfig returns a configuration suitable for development
func DevelopmentMetricsConfig() *MetricsConfig {
	config := DefaultMetricsConfig()
	config.Level = MetricsLevelDebug
	config.SamplingRate = 1.0
	config.FlushInterval = 30 * time.Second
	return config
}

// RuleExecutionMetric tracks execution metrics for a specific rule
type RuleExecutionMetric struct {
	RuleID        string
	ExecutionCount int64
	TotalDuration time.Duration
	MinDuration   time.Duration
	MaxDuration   time.Duration
	LastExecution time.Time
	ErrorCount    int64
	
	// Histogram buckets for detailed timing analysis
	DurationBuckets []time.Duration
	BucketCounts    []int64
	
	mutex sync.RWMutex
}

// NewRuleExecutionMetric creates a new rule execution metric
func NewRuleExecutionMetric(ruleID string) *RuleExecutionMetric {
	return &RuleExecutionMetric{
		RuleID:          ruleID,
		MinDuration:     time.Duration(math.MaxInt64),
		DurationBuckets: []time.Duration{
			time.Millisecond,
			5 * time.Millisecond,
			10 * time.Millisecond,
			25 * time.Millisecond,
			50 * time.Millisecond,
			100 * time.Millisecond,
			250 * time.Millisecond,
			500 * time.Millisecond,
			time.Second,
			5 * time.Second,
		},
		BucketCounts: make([]int64, 10),
	}
}

// RecordExecution records a rule execution
func (m *RuleExecutionMetric) RecordExecution(duration time.Duration, success bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.ExecutionCount++
	m.TotalDuration += duration
	m.LastExecution = time.Now()
	
	if duration < m.MinDuration {
		m.MinDuration = duration
	}
	if duration > m.MaxDuration {
		m.MaxDuration = duration
	}
	
	if !success {
		m.ErrorCount++
	}
	
	// Update histogram buckets
	for i, bucket := range m.DurationBuckets {
		if duration <= bucket {
			m.BucketCounts[i]++
			break
		}
	}
}

// GetAverageDuration returns the average execution duration
func (m *RuleExecutionMetric) GetAverageDuration() time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if m.ExecutionCount == 0 {
		return 0
	}
	return m.TotalDuration / time.Duration(m.ExecutionCount)
}

// GetErrorRate returns the error rate as a percentage
func (m *RuleExecutionMetric) GetErrorRate() float64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if m.ExecutionCount == 0 {
		return 0
	}
	return float64(m.ErrorCount) / float64(m.ExecutionCount) * 100
}

// MemoryUsageMetric tracks memory usage statistics
type MemoryUsageMetric struct {
	Timestamp    time.Time
	AllocBytes   uint64
	TotalAlloc   uint64
	HeapObjects  uint64
	HeapInuse    uint64
	StackInuse   uint64
	GCCycles     uint32
	GCPauseTotal time.Duration
}

// GetMemoryUsage returns current memory usage metrics
func GetMemoryUsage() *MemoryUsageMetric {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	return &MemoryUsageMetric{
		Timestamp:    time.Now(),
		AllocBytes:   m.Alloc,
		TotalAlloc:   m.TotalAlloc,
		HeapObjects:  m.HeapObjects,
		HeapInuse:    m.HeapInuse,
		StackInuse:   m.StackInuse,
		GCCycles:     m.NumGC,
		GCPauseTotal: time.Duration(m.PauseTotalNs),
	}
}

// ThroughputMetric tracks throughput statistics
type ThroughputMetric struct {
	EventsProcessed      int64
	EventsPerSecond      float64
	ValidationDuration   time.Duration
	LastMeasurement      time.Time
	WindowSize           time.Duration
	SampleCount          int64
	
	// SLA tracking
	SLATarget            float64 // Target events per second
	SLAViolations        int64   // Number of SLA violations
	SLAComplianceRate    float64 // Percentage of time meeting SLA
	
	mutex sync.RWMutex
}

// NewThroughputMetric creates a new throughput metric
func NewThroughputMetric(windowSize time.Duration, slaTarget float64) *ThroughputMetric {
	return &ThroughputMetric{
		WindowSize:        windowSize,
		SLATarget:         slaTarget,
		LastMeasurement:   time.Now(),
		SLAComplianceRate: 100.0,
	}
}

// RecordEvents records processed events
func (m *ThroughputMetric) RecordEvents(count int64, duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.EventsProcessed += count
	m.ValidationDuration += duration
	m.SampleCount++
	
	// Calculate current throughput
	now := time.Now()
	elapsed := now.Sub(m.LastMeasurement)
	
	if elapsed >= m.WindowSize {
		m.EventsPerSecond = float64(m.EventsProcessed) / elapsed.Seconds()
		
		// Check SLA compliance
		if m.EventsPerSecond < m.SLATarget {
			m.SLAViolations++
		}
		
		// Update compliance rate
		if m.SampleCount > 0 {
			m.SLAComplianceRate = float64(m.SampleCount-m.SLAViolations) / float64(m.SampleCount) * 100.0
		}
		
		m.LastMeasurement = now
		m.EventsProcessed = 0
		m.ValidationDuration = 0
	}
}

// DashboardData contains real-time metrics for dashboard display
type DashboardData struct {
	Timestamp       time.Time                        `json:"timestamp"`
	TotalEvents     int64                           `json:"total_events"`
	EventsPerSecond float64                         `json:"events_per_second"`
	AverageLatency  time.Duration                   `json:"average_latency"`
	ErrorRate       float64                         `json:"error_rate"`
	MemoryUsage     *MemoryUsageMetric              `json:"memory_usage"`
	TopSlowRules    []RulePerformanceSnapshot       `json:"top_slow_rules"`
	SLACompliance   float64                         `json:"sla_compliance"`
	ActiveRules     int                             `json:"active_rules"`
	
	// Performance indicators
	HealthStatus    string                          `json:"health_status"`
	Warnings        []string                        `json:"warnings"`
	Alerts          []string                        `json:"alerts"`
}

// RulePerformanceSnapshot contains performance snapshot for a rule
type RulePerformanceSnapshot struct {
	RuleID         string        `json:"rule_id"`
	ExecutionCount int64         `json:"execution_count"`
	AverageDuration time.Duration `json:"average_duration"`
	ErrorRate      float64       `json:"error_rate"`
	LastExecution  time.Time     `json:"last_execution"`
}

// PerformanceRegression tracks performance regression data
type PerformanceRegression struct {
	RuleID             string        `json:"rule_id"`
	BaselineAverage    time.Duration `json:"baseline_average"`
	CurrentAverage     time.Duration `json:"current_average"`
	RegressionPercent  float64       `json:"regression_percent"`
	DetectedAt         time.Time     `json:"detected_at"`
	Severity           string        `json:"severity"`
}

// ValidationPerformanceMetrics is the main metrics collection struct
type ValidationPerformanceMetrics struct {
	config         *MetricsConfig
	startTime      time.Time
	
	// Core metrics
	totalEvents    int64
	totalErrors    int64
	totalWarnings  int64
	
	// Rule metrics
	ruleMetrics    map[string]*RuleExecutionMetric
	ruleBaselines  map[string]time.Duration
	rulesMutex     sync.RWMutex
	
	// Throughput metrics
	throughputMetric *ThroughputMetric
	
	// Memory tracking
	memoryHistory    []MemoryUsageMetric
	memoryMutex      sync.RWMutex
	
	// Performance regression tracking
	regressions      []PerformanceRegression
	regressionsMutex sync.RWMutex
	
	// OpenTelemetry metrics
	meterProvider metric.MeterProvider
	meter         metric.Meter
	
	// Metric instruments
	eventCounter         metric.Int64Counter
	validationHistogram  metric.Int64Histogram
	ruleHistogram        metric.Int64Histogram
	errorCounter         metric.Int64Counter
	memoryGauge          metric.Int64Gauge
	
	// Cleanup and lifecycle
	cleanupTicker *time.Ticker
	ctx           context.Context
	cancel        context.CancelFunc
	
	// Sampling
	sampleCounter int64
	
	mutex sync.RWMutex
}

// NewValidationPerformanceMetrics creates a new performance metrics instance
func NewValidationPerformanceMetrics(config *MetricsConfig) (*ValidationPerformanceMetrics, error) {
	if config == nil {
		config = DefaultMetricsConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	m := &ValidationPerformanceMetrics{
		config:           config,
		startTime:        time.Now(),
		ruleMetrics:      make(map[string]*RuleExecutionMetric),
		ruleBaselines:    make(map[string]time.Duration),
		memoryHistory:    make([]MemoryUsageMetric, 0),
		regressions:      make([]PerformanceRegression, 0),
		throughputMetric: NewThroughputMetric(time.Minute, 100.0), // Default 100 events/sec SLA
		ctx:              ctx,
		cancel:           cancel,
	}
	
	// Initialize OpenTelemetry metrics
	if err := m.initializeOpenTelemetry(); err != nil {
		return nil, fmt.Errorf("failed to initialize OpenTelemetry metrics: %w", err)
	}
	
	// Start background routines
	m.startBackgroundTasks()
	
	return m, nil
}

// initializeOpenTelemetry initializes OpenTelemetry metrics
func (m *ValidationPerformanceMetrics) initializeOpenTelemetry() error {
	// Skip OpenTelemetry initialization if no provider is set
	if m.meterProvider == nil {
		return nil
	}
	
	// Create meter
	m.meter = m.meterProvider.Meter("ag-ui/events/validation")
	
	var err error
	
	// Create metric instruments
	m.eventCounter, err = m.meter.Int64Counter(
		"validation_events_total",
		metric.WithDescription("Total number of events processed"),
	)
	if err != nil {
		return fmt.Errorf("failed to create event counter: %w", err)
	}
	
	m.validationHistogram, err = m.meter.Int64Histogram(
		"validation_duration_ms",
		metric.WithDescription("Validation duration in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return fmt.Errorf("failed to create validation histogram: %w", err)
	}
	
	m.ruleHistogram, err = m.meter.Int64Histogram(
		"rule_execution_duration_ms",
		metric.WithDescription("Rule execution duration in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return fmt.Errorf("failed to create rule histogram: %w", err)
	}
	
	m.errorCounter, err = m.meter.Int64Counter(
		"validation_errors_total",
		metric.WithDescription("Total number of validation errors"),
	)
	if err != nil {
		return fmt.Errorf("failed to create error counter: %w", err)
	}
	
	m.memoryGauge, err = m.meter.Int64Gauge(
		"memory_usage_bytes",
		metric.WithDescription("Current memory usage in bytes"),
		metric.WithUnit("bytes"),
	)
	if err != nil {
		return fmt.Errorf("failed to create memory gauge: %w", err)
	}
	
	return nil
}

// startBackgroundTasks starts background maintenance tasks
func (m *ValidationPerformanceMetrics) startBackgroundTasks() {
	// Start cleanup routine
	m.cleanupTicker = time.NewTicker(m.config.FlushInterval)
	go m.cleanupRoutine()
	
	// Start memory monitoring
	if m.config.EnableLeakDetection {
		go m.memoryMonitoringRoutine()
	}
	
	// Start performance regression detection
	if m.config.EnableRegression {
		go m.regressionDetectionRoutine()
	}
}

// RecordEvent records an event processing operation
func (m *ValidationPerformanceMetrics) RecordEvent(duration time.Duration, success bool) {
	// Apply sampling
	if !m.shouldSample() {
		return
	}
	
	atomic.AddInt64(&m.totalEvents, 1)
	
	if !success {
		atomic.AddInt64(&m.totalErrors, 1)
	}
	
	// Record throughput
	m.throughputMetric.RecordEvents(1, duration)
	
	// Record OpenTelemetry metrics
	if m.eventCounter != nil {
		m.eventCounter.Add(m.ctx, 1, metric.WithAttributes(
			attribute.Bool("success", success),
		))
	}
	
	if m.validationHistogram != nil {
		m.validationHistogram.Record(m.ctx, duration.Milliseconds(), metric.WithAttributes(
			attribute.Bool("success", success),
		))
	}
	
	if !success && m.errorCounter != nil {
		m.errorCounter.Add(m.ctx, 1)
	}
}

// RecordWarning records a validation warning
func (m *ValidationPerformanceMetrics) RecordWarning() {
	atomic.AddInt64(&m.totalWarnings, 1)
}

// RecordRuleExecution records execution of a specific rule
func (m *ValidationPerformanceMetrics) RecordRuleExecution(ruleID string, duration time.Duration, success bool) {
	// Apply sampling
	if !m.shouldSample() {
		return
	}
	
	m.rulesMutex.Lock()
	if _, exists := m.ruleMetrics[ruleID]; !exists {
		m.ruleMetrics[ruleID] = NewRuleExecutionMetric(ruleID)
	}
	m.ruleMetrics[ruleID].RecordExecution(duration, success)
	m.rulesMutex.Unlock()
	
	// Record OpenTelemetry metrics
	if m.ruleHistogram != nil {
		m.ruleHistogram.Record(m.ctx, duration.Milliseconds(), metric.WithAttributes(
			attribute.String("rule_id", ruleID),
			attribute.Bool("success", success),
		))
	}
	
	// Check for slow rules
	if duration > m.config.SlowRuleThreshold {
		// Log slow rule execution if in debug mode
		if m.config.Level == MetricsLevelDebug {
			fmt.Printf("Slow rule execution detected: %s took %v\n", ruleID, duration)
		}
	}
}

// SetRuleBaseline sets a baseline performance for a rule
func (m *ValidationPerformanceMetrics) SetRuleBaseline(ruleID string, baseline time.Duration) {
	m.rulesMutex.Lock()
	defer m.rulesMutex.Unlock()
	m.ruleBaselines[ruleID] = baseline
}

// GetRuleMetrics returns metrics for a specific rule
func (m *ValidationPerformanceMetrics) GetRuleMetrics(ruleID string) *RuleExecutionMetric {
	m.rulesMutex.RLock()
	defer m.rulesMutex.RUnlock()
	if metric, exists := m.ruleMetrics[ruleID]; exists {
		return metric
	}
	return nil
}

// GetAllRuleMetrics returns all rule metrics
func (m *ValidationPerformanceMetrics) GetAllRuleMetrics() map[string]*RuleExecutionMetric {
	m.rulesMutex.RLock()
	defer m.rulesMutex.RUnlock()
	
	result := make(map[string]*RuleExecutionMetric)
	for k, v := range m.ruleMetrics {
		result[k] = v
	}
	return result
}

// GetDashboardData returns real-time dashboard data
func (m *ValidationPerformanceMetrics) GetDashboardData() *DashboardData {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	totalEvents := atomic.LoadInt64(&m.totalEvents)
	totalErrors := atomic.LoadInt64(&m.totalErrors)
	
	var errorRate float64
	if totalEvents > 0 {
		errorRate = float64(totalErrors) / float64(totalEvents) * 100.0
	}
	
	// Calculate average latency
	var avgLatency time.Duration
	if m.throughputMetric != nil {
		m.throughputMetric.mutex.RLock()
		if m.throughputMetric.SampleCount > 0 {
			avgLatency = m.throughputMetric.ValidationDuration / time.Duration(m.throughputMetric.SampleCount)
		}
		m.throughputMetric.mutex.RUnlock()
	}
	
	// Get top slow rules
	topSlowRules := m.getTopSlowRules(5)
	
	// Get current memory usage
	memoryUsage := GetMemoryUsage()
	
	// Record memory usage in gauge
	if m.memoryGauge != nil {
		m.memoryGauge.Record(m.ctx, int64(memoryUsage.AllocBytes))
	}
	
	// Determine health status
	healthStatus := m.determineHealthStatus()
	warnings := m.getWarnings()
	alerts := m.getAlerts()
	
	return &DashboardData{
		Timestamp:       time.Now(),
		TotalEvents:     totalEvents,
		EventsPerSecond: m.throughputMetric.EventsPerSecond,
		AverageLatency:  avgLatency,
		ErrorRate:       errorRate,
		MemoryUsage:     memoryUsage,
		TopSlowRules:    topSlowRules,
		SLACompliance:   m.throughputMetric.SLAComplianceRate,
		ActiveRules:     len(m.ruleMetrics),
		HealthStatus:    healthStatus,
		Warnings:        warnings,
		Alerts:          alerts,
	}
}

// getTopSlowRules returns the top N slowest rules
func (m *ValidationPerformanceMetrics) getTopSlowRules(n int) []RulePerformanceSnapshot {
	m.rulesMutex.RLock()
	defer m.rulesMutex.RUnlock()
	
	var snapshots []RulePerformanceSnapshot
	for _, metric := range m.ruleMetrics {
		snapshot := RulePerformanceSnapshot{
			RuleID:          metric.RuleID,
			ExecutionCount:  metric.ExecutionCount,
			AverageDuration: metric.GetAverageDuration(),
			ErrorRate:       metric.GetErrorRate(),
			LastExecution:   metric.LastExecution,
		}
		snapshots = append(snapshots, snapshot)
	}
	
	// Sort by average duration (descending)
	for i := 0; i < len(snapshots)-1; i++ {
		for j := i + 1; j < len(snapshots); j++ {
			if snapshots[i].AverageDuration < snapshots[j].AverageDuration {
				snapshots[i], snapshots[j] = snapshots[j], snapshots[i]
			}
		}
	}
	
	if len(snapshots) > n {
		snapshots = snapshots[:n]
	}
	
	return snapshots
}

// determineHealthStatus determines the overall health status
func (m *ValidationPerformanceMetrics) determineHealthStatus() string {
	totalEvents := atomic.LoadInt64(&m.totalEvents)
	totalErrors := atomic.LoadInt64(&m.totalErrors)
	
	if totalEvents == 0 {
		return "Unknown"
	}
	
	errorRate := float64(totalErrors) / float64(totalEvents) * 100.0
	
	// Check SLA compliance
	slaCompliance := m.throughputMetric.SLAComplianceRate
	
	// Check memory usage
	memUsage := GetMemoryUsage()
	memoryPressure := float64(memUsage.AllocBytes) / float64(m.config.MaxMemoryUsage) * 100.0
	
	// Determine status based on multiple factors
	if errorRate > 10.0 || slaCompliance < 80.0 || memoryPressure > 90.0 {
		return "Critical"
	} else if errorRate > 5.0 || slaCompliance < 90.0 || memoryPressure > 70.0 {
		return "Warning"
	} else {
		return "Healthy"
	}
}

// getWarnings returns current warnings
func (m *ValidationPerformanceMetrics) getWarnings() []string {
	var warnings []string
	
	// Check for slow rules
	for _, metric := range m.ruleMetrics {
		if metric.GetAverageDuration() > m.config.SlowRuleThreshold {
			warnings = append(warnings, fmt.Sprintf("Rule %s is running slow (avg: %v)", metric.RuleID, metric.GetAverageDuration()))
		}
	}
	
	// Check memory usage
	memUsage := GetMemoryUsage()
	if float64(memUsage.AllocBytes) > float64(m.config.MaxMemoryUsage)*0.7 {
		warnings = append(warnings, fmt.Sprintf("High memory usage: %d bytes", memUsage.AllocBytes))
	}
	
	return warnings
}

// getAlerts returns current alerts
func (m *ValidationPerformanceMetrics) getAlerts() []string {
	var alerts []string
	
	// Check for critical issues
	totalEvents := atomic.LoadInt64(&m.totalEvents)
	totalErrors := atomic.LoadInt64(&m.totalErrors)
	
	if totalEvents > 0 {
		errorRate := float64(totalErrors) / float64(totalEvents) * 100.0
		if errorRate > 10.0 {
			alerts = append(alerts, fmt.Sprintf("High error rate: %.2f%%", errorRate))
		}
	}
	
	// Check SLA violations
	if m.throughputMetric.SLAComplianceRate < 80.0 {
		alerts = append(alerts, fmt.Sprintf("SLA compliance below 80%%: %.2f%%", m.throughputMetric.SLAComplianceRate))
	}
	
	// Check memory leaks
	if m.config.EnableLeakDetection {
		memUsage := GetMemoryUsage()
		if memUsage.AllocBytes > uint64(m.config.MemoryLeakThreshold) {
			alerts = append(alerts, fmt.Sprintf("Potential memory leak detected: %d bytes", memUsage.AllocBytes))
		}
	}
	
	return alerts
}

// shouldSample determines if the current operation should be sampled
func (m *ValidationPerformanceMetrics) shouldSample() bool {
	if m.config.SamplingRate >= 1.0 {
		return true
	}
	
	sampleCount := atomic.AddInt64(&m.sampleCounter, 1)
	return float64(sampleCount%int64(1.0/m.config.SamplingRate)) == 0
}

// cleanupRoutine periodically cleans up old metrics data
func (m *ValidationPerformanceMetrics) cleanupRoutine() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.cleanupTicker.C:
			m.performCleanup()
		}
	}
}

// performCleanup performs cleanup of old metrics data
func (m *ValidationPerformanceMetrics) performCleanup() {
	cutoff := time.Now().Add(-m.config.RetentionPeriod)
	
	// Clean up memory history
	m.memoryMutex.Lock()
	var newHistory []MemoryUsageMetric
	for _, usage := range m.memoryHistory {
		if usage.Timestamp.After(cutoff) {
			newHistory = append(newHistory, usage)
		}
	}
	m.memoryHistory = newHistory
	m.memoryMutex.Unlock()
	
	// Clean up old regressions
	m.regressionsMutex.Lock()
	var newRegressions []PerformanceRegression
	for _, regression := range m.regressions {
		if regression.DetectedAt.After(cutoff) {
			newRegressions = append(newRegressions, regression)
		}
	}
	m.regressions = newRegressions
	m.regressionsMutex.Unlock()
	
	// Force garbage collection if memory usage is high
	memUsage := GetMemoryUsage()
	if memUsage.AllocBytes > uint64(m.config.MaxMemoryUsage) {
		runtime.GC()
	}
}

// memoryMonitoringRoutine monitors memory usage for leaks
func (m *ValidationPerformanceMetrics) memoryMonitoringRoutine() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			memUsage := GetMemoryUsage()
			
			m.memoryMutex.Lock()
			m.memoryHistory = append(m.memoryHistory, *memUsage)
			
			// Keep only recent history
			if len(m.memoryHistory) > 60 { // Keep last hour
				m.memoryHistory = m.memoryHistory[len(m.memoryHistory)-60:]
			}
			m.memoryMutex.Unlock()
			
			// Check for memory leaks
			if len(m.memoryHistory) > 10 {
				m.detectMemoryLeaks()
			}
		}
	}
}

// detectMemoryLeaks detects potential memory leaks
func (m *ValidationPerformanceMetrics) detectMemoryLeaks() {
	m.memoryMutex.RLock()
	defer m.memoryMutex.RUnlock()
	
	if len(m.memoryHistory) < 10 {
		return
	}
	
	// Simple leak detection: check if memory usage is consistently increasing
	recent := m.memoryHistory[len(m.memoryHistory)-5:]
	older := m.memoryHistory[len(m.memoryHistory)-10 : len(m.memoryHistory)-5]
	
	var recentAvg, olderAvg uint64
	for _, usage := range recent {
		recentAvg += usage.AllocBytes
	}
	recentAvg /= uint64(len(recent))
	
	for _, usage := range older {
		olderAvg += usage.AllocBytes
	}
	olderAvg /= uint64(len(older))
	
	// If recent average is significantly higher than older average, potential leak
	if recentAvg > olderAvg && float64(recentAvg-olderAvg)/float64(olderAvg) > 0.3 {
		if m.config.Level == MetricsLevelDebug {
			fmt.Printf("Potential memory leak detected: %d -> %d bytes\n", olderAvg, recentAvg)
		}
	}
}

// regressionDetectionRoutine detects performance regressions
func (m *ValidationPerformanceMetrics) regressionDetectionRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.detectPerformanceRegressions()
		}
	}
}

// detectPerformanceRegressions detects performance regressions in rules
func (m *ValidationPerformanceMetrics) detectPerformanceRegressions() {
	m.rulesMutex.RLock()
	defer m.rulesMutex.RUnlock()
	
	for ruleID, metric := range m.ruleMetrics {
		if baseline, exists := m.ruleBaselines[ruleID]; exists {
			current := metric.GetAverageDuration()
			if current > 0 && baseline > 0 {
				regressionPercent := (float64(current-baseline) / float64(baseline)) * 100.0
				
				if regressionPercent > m.config.RegressionThreshold*100 {
					regression := PerformanceRegression{
						RuleID:            ruleID,
						BaselineAverage:   baseline,
						CurrentAverage:    current,
						RegressionPercent: regressionPercent,
						DetectedAt:        time.Now(),
						Severity:          m.determineSeverity(regressionPercent),
					}
					
					m.regressionsMutex.Lock()
					m.regressions = append(m.regressions, regression)
					m.regressionsMutex.Unlock()
					
					if m.config.Level == MetricsLevelDebug {
						fmt.Printf("Performance regression detected in rule %s: %.2f%% slower\n", ruleID, regressionPercent)
					}
				}
			}
		}
	}
}

// determineSeverity determines the severity of a performance regression
func (m *ValidationPerformanceMetrics) determineSeverity(regressionPercent float64) string {
	if regressionPercent > 100.0 {
		return "Critical"
	} else if regressionPercent > 50.0 {
		return "High"
	} else if regressionPercent > 20.0 {
		return "Medium"
	} else {
		return "Low"
	}
}

// GetPerformanceRegressions returns detected performance regressions
func (m *ValidationPerformanceMetrics) GetPerformanceRegressions() []PerformanceRegression {
	m.regressionsMutex.RLock()
	defer m.regressionsMutex.RUnlock()
	
	result := make([]PerformanceRegression, len(m.regressions))
	copy(result, m.regressions)
	return result
}

// GetMemoryHistory returns memory usage history
func (m *ValidationPerformanceMetrics) GetMemoryHistory() []MemoryUsageMetric {
	m.memoryMutex.RLock()
	defer m.memoryMutex.RUnlock()
	
	result := make([]MemoryUsageMetric, len(m.memoryHistory))
	copy(result, m.memoryHistory)
	return result
}

// Export exports metrics to configured backends
func (m *ValidationPerformanceMetrics) Export() error {
	// Implementation would depend on the specific backends configured
	// This is a placeholder for the export functionality
	return nil
}

// Shutdown gracefully shuts down the metrics collection
func (m *ValidationPerformanceMetrics) Shutdown() error {
	if m.cancel != nil {
		m.cancel()
	}
	
	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
	}
	
	// Final cleanup
	m.performCleanup()
	
	return nil
}

// GetOverallStats returns overall performance statistics
func (m *ValidationPerformanceMetrics) GetOverallStats() map[string]interface{} {
	totalEvents := atomic.LoadInt64(&m.totalEvents)
	totalErrors := atomic.LoadInt64(&m.totalErrors)
	totalWarnings := atomic.LoadInt64(&m.totalWarnings)
	
	var errorRate, warningRate float64
	if totalEvents > 0 {
		errorRate = float64(totalErrors) / float64(totalEvents) * 100.0
		warningRate = float64(totalWarnings) / float64(totalEvents) * 100.0
	}
	
	uptime := time.Since(m.startTime)
	
	return map[string]interface{}{
		"uptime":                uptime,
		"total_events":          totalEvents,
		"total_errors":          totalErrors,
		"total_warnings":        totalWarnings,
		"error_rate":           errorRate,
		"warning_rate":         warningRate,
		"events_per_second":    m.throughputMetric.EventsPerSecond,
		"sla_compliance":       m.throughputMetric.SLAComplianceRate,
		"active_rules":         len(m.ruleMetrics),
		"config_level":         m.config.Level.String(),
		"sampling_rate":        m.config.SamplingRate,
		"memory_usage":         GetMemoryUsage(),
		"regression_count":     len(m.regressions),
	}
}