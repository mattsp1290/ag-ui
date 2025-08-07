package middleware

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// MetricsCollector defines the interface for metrics collection
type MetricsCollector interface {
	// Counter metrics
	IncrementCounter(name string, labels map[string]string, value float64)

	// Histogram metrics
	RecordHistogram(name string, labels map[string]string, value float64)

	// Gauge metrics
	SetGauge(name string, labels map[string]string, value float64)

	// Summary metrics
	RecordSummary(name string, labels map[string]string, value float64)
}

// InMemoryMetricsCollector is a simple in-memory metrics collector
type InMemoryMetricsCollector struct {
	counters   map[string]*CounterMetric
	histograms map[string]*HistogramMetric
	gauges     map[string]*GaugeMetric
	summaries  map[string]*SummaryMetric
	mu         sync.RWMutex
}

// CounterMetric represents a counter metric
type CounterMetric struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
}

// HistogramMetric represents a histogram metric
type HistogramMetric struct {
	Name    string            `json:"name"`
	Labels  map[string]string `json:"labels"`
	Count   int64             `json:"count"`
	Sum     float64           `json:"sum"`
	Buckets map[string]int64  `json:"buckets"`
}

// GaugeMetric represents a gauge metric
type GaugeMetric struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Value  float64           `json:"value"`
}

// SummaryMetric represents a summary metric
type SummaryMetric struct {
	Name      string             `json:"name"`
	Labels    map[string]string  `json:"labels"`
	Count     int64              `json:"count"`
	Sum       float64            `json:"sum"`
	Quantiles map[string]float64 `json:"quantiles"`
}

// NewInMemoryMetricsCollector creates a new in-memory metrics collector
func NewInMemoryMetricsCollector() *InMemoryMetricsCollector {
	return &InMemoryMetricsCollector{
		counters:   make(map[string]*CounterMetric),
		histograms: make(map[string]*HistogramMetric),
		gauges:     make(map[string]*GaugeMetric),
		summaries:  make(map[string]*SummaryMetric),
	}
}

// IncrementCounter increments a counter metric
func (imc *InMemoryMetricsCollector) IncrementCounter(name string, labels map[string]string, value float64) {
	imc.mu.Lock()
	defer imc.mu.Unlock()

	key := imc.buildKey(name, labels)
	if counter, exists := imc.counters[key]; exists {
		counter.Value += value
	} else {
		imc.counters[key] = &CounterMetric{
			Name:   name,
			Labels: labels,
			Value:  value,
		}
	}
}

// RecordHistogram records a histogram metric
func (imc *InMemoryMetricsCollector) RecordHistogram(name string, labels map[string]string, value float64) {
	imc.mu.Lock()
	defer imc.mu.Unlock()

	key := imc.buildKey(name, labels)
	if histogram, exists := imc.histograms[key]; exists {
		histogram.Count++
		histogram.Sum += value
		// Simplified bucketing - in practice, you'd use predefined buckets
		bucket := imc.getBucket(value)
		histogram.Buckets[bucket]++
	} else {
		buckets := make(map[string]int64)
		bucket := imc.getBucket(value)
		buckets[bucket] = 1

		imc.histograms[key] = &HistogramMetric{
			Name:    name,
			Labels:  labels,
			Count:   1,
			Sum:     value,
			Buckets: buckets,
		}
	}
}

// SetGauge sets a gauge metric
func (imc *InMemoryMetricsCollector) SetGauge(name string, labels map[string]string, value float64) {
	imc.mu.Lock()
	defer imc.mu.Unlock()

	key := imc.buildKey(name, labels)
	imc.gauges[key] = &GaugeMetric{
		Name:   name,
		Labels: labels,
		Value:  value,
	}
}

// RecordSummary records a summary metric
func (imc *InMemoryMetricsCollector) RecordSummary(name string, labels map[string]string, value float64) {
	imc.mu.Lock()
	defer imc.mu.Unlock()

	key := imc.buildKey(name, labels)
	if summary, exists := imc.summaries[key]; exists {
		summary.Count++
		summary.Sum += value
		// Simplified quantile calculation
		summary.Quantiles["0.5"] = summary.Sum / float64(summary.Count) // Simple average as median
		summary.Quantiles["0.95"] = value                               // Simplified - use latest value
		summary.Quantiles["0.99"] = value
	} else {
		quantiles := map[string]float64{
			"0.5":  value,
			"0.95": value,
			"0.99": value,
		}

		imc.summaries[key] = &SummaryMetric{
			Name:      name,
			Labels:    labels,
			Count:     1,
			Sum:       value,
			Quantiles: quantiles,
		}
	}
}

// GetMetrics returns all collected metrics
func (imc *InMemoryMetricsCollector) GetMetrics() map[string]interface{} {
	imc.mu.RLock()
	defer imc.mu.RUnlock()

	metrics := make(map[string]interface{})

	counters := make([]*CounterMetric, 0, len(imc.counters))
	for _, counter := range imc.counters {
		counters = append(counters, counter)
	}
	metrics["counters"] = counters

	histograms := make([]*HistogramMetric, 0, len(imc.histograms))
	for _, histogram := range imc.histograms {
		histograms = append(histograms, histogram)
	}
	metrics["histograms"] = histograms

	gauges := make([]*GaugeMetric, 0, len(imc.gauges))
	for _, gauge := range imc.gauges {
		gauges = append(gauges, gauge)
	}
	metrics["gauges"] = gauges

	summaries := make([]*SummaryMetric, 0, len(imc.summaries))
	for _, summary := range imc.summaries {
		summaries = append(summaries, summary)
	}
	metrics["summaries"] = summaries

	return metrics
}

// buildKey builds a unique key for a metric
func (imc *InMemoryMetricsCollector) buildKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}

	var parts []string
	parts = append(parts, name)

	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(parts, ",")
}

// getBucket determines the histogram bucket for a value
func (imc *InMemoryMetricsCollector) getBucket(value float64) string {
	// Simplified bucketing - in practice, you'd use predefined buckets
	buckets := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

	for _, bucket := range buckets {
		if value <= bucket {
			return fmt.Sprintf("%.3f", bucket)
		}
	}

	return "+Inf"
}

// MetricsConfig contains metrics middleware configuration
type MetricsConfig struct {
	BaseConfig `json:",inline" yaml:",inline"`

	// Collector is the metrics collector to use
	Collector MetricsCollector `json:"-" yaml:"-"`

	// Request metrics
	EnableRequestMetrics bool   `json:"enable_request_metrics" yaml:"enable_request_metrics"`
	RequestMetricName    string `json:"request_metric_name" yaml:"request_metric_name"`

	// Response metrics
	EnableResponseMetrics  bool   `json:"enable_response_metrics" yaml:"enable_response_metrics"`
	ResponseSizeMetricName string `json:"response_size_metric_name" yaml:"response_size_metric_name"`

	// Duration metrics
	EnableDurationMetrics bool   `json:"enable_duration_metrics" yaml:"enable_duration_metrics"`
	DurationMetricName    string `json:"duration_metric_name" yaml:"duration_metric_name"`

	// Active requests gauge
	EnableActiveRequests     bool   `json:"enable_active_requests" yaml:"enable_active_requests"`
	ActiveRequestsMetricName string `json:"active_requests_metric_name" yaml:"active_requests_metric_name"`

	// Custom metrics
	CustomCounters   []CustomMetricConfig `json:"custom_counters" yaml:"custom_counters"`
	CustomHistograms []CustomMetricConfig `json:"custom_histograms" yaml:"custom_histograms"`
	CustomGauges     []CustomMetricConfig `json:"custom_gauges" yaml:"custom_gauges"`

	// Label configuration
	IncludeMethod    bool     `json:"include_method" yaml:"include_method"`
	IncludeStatus    bool     `json:"include_status" yaml:"include_status"`
	IncludePath      bool     `json:"include_path" yaml:"include_path"`
	IncludeUserAgent bool     `json:"include_user_agent" yaml:"include_user_agent"`
	IncludeUserID    bool     `json:"include_user_id" yaml:"include_user_id"`
	CustomLabels     []string `json:"custom_labels" yaml:"custom_labels"`

	// Path normalization
	NormalizePaths   bool              `json:"normalize_paths" yaml:"normalize_paths"`
	PathReplacements map[string]string `json:"path_replacements" yaml:"path_replacements"`

	// Filtering
	ExcludePaths       []string `json:"exclude_paths" yaml:"exclude_paths"`
	ExcludeStatusCodes []int    `json:"exclude_status_codes" yaml:"exclude_status_codes"`
	ExcludeUserAgents  []string `json:"exclude_user_agents" yaml:"exclude_user_agents"`

	// Performance
	SampleRate float64 `json:"sample_rate" yaml:"sample_rate"`
}

// CustomMetricConfig defines a custom metric
type CustomMetricConfig struct {
	Name      string   `json:"name" yaml:"name"`
	Help      string   `json:"help" yaml:"help"`
	Labels    []string `json:"labels" yaml:"labels"`
	ValueFrom string   `json:"value_from" yaml:"value_from"` // header, context, etc.
	Condition string   `json:"condition" yaml:"condition"`   // when to record
}

// MetricsMiddleware implements performance monitoring middleware
type MetricsMiddleware struct {
	config    *MetricsConfig
	logger    *zap.Logger
	collector MetricsCollector

	// Active request tracking
	activeRequests sync.Map

	// Precomputed maps for performance
	excludePathMap       map[string]bool
	excludeStatusCodeMap map[int]bool
	excludeUserAgentMap  map[string]bool
}

// ActiveRequest tracks an active request
type ActiveRequest struct {
	StartTime time.Time
	Method    string
	Path      string
	UserID    string
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware(config *MetricsConfig, logger *zap.Logger) (*MetricsMiddleware, error) {
	if config == nil {
		return nil, fmt.Errorf("metrics config cannot be nil")
	}

	if err := ValidateBaseConfig(&config.BaseConfig); err != nil {
		return nil, fmt.Errorf("invalid base config: %w", err)
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// Set defaults
	if config.Name == "" {
		config.Name = "metrics"
	}
	if config.Priority == 0 {
		config.Priority = 20 // Medium priority
	}
	if config.RequestMetricName == "" {
		config.RequestMetricName = "http_requests_total"
	}
	if config.ResponseSizeMetricName == "" {
		config.ResponseSizeMetricName = "http_response_size_bytes"
	}
	if config.DurationMetricName == "" {
		config.DurationMetricName = "http_request_duration_seconds"
	}
	if config.ActiveRequestsMetricName == "" {
		config.ActiveRequestsMetricName = "http_requests_active"
	}
	if config.SampleRate == 0 {
		config.SampleRate = 1.0 // Default to 100% sampling
	}

	// Use provided collector or create default
	collector := config.Collector
	if collector == nil {
		collector = NewInMemoryMetricsCollector()
	}

	middleware := &MetricsMiddleware{
		config:               config,
		logger:               logger,
		collector:            collector,
		excludePathMap:       make(map[string]bool),
		excludeStatusCodeMap: make(map[int]bool),
		excludeUserAgentMap:  make(map[string]bool),
	}

	// Build maps for performance
	middleware.buildMaps()

	return middleware, nil
}

// Handler implements the Middleware interface
func (mm *MetricsMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !mm.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Check if request should be excluded
		if mm.shouldExcludeRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Sample requests if configured
		if mm.config.SampleRate < 1.0 {
			// Use math/rand for unbiased random sampling
			if rand.Float64() >= mm.config.SampleRate {
				next.ServeHTTP(w, r)
				return
			}
		}

		startTime := time.Now()
		requestID := GenerateRequestID()

		// Track active request
		if mm.config.EnableActiveRequests {
			activeReq := &ActiveRequest{
				StartTime: startTime,
				Method:    r.Method,
				Path:      mm.normalizePath(r.URL.Path),
				UserID:    GetUserID(r.Context()),
			}
			mm.activeRequests.Store(requestID, activeReq)

			// Update active requests gauge
			mm.updateActiveRequestsGauge(1)
		}

		// Create metrics response writer
		mrw := NewResponseWriter(w)

		// Process request
		next.ServeHTTP(mrw, r)

		// Record metrics
		mm.recordMetrics(r, mrw, startTime)

		// Remove from active requests
		if mm.config.EnableActiveRequests {
			mm.activeRequests.Delete(requestID)
			mm.updateActiveRequestsGauge(-1)
		}
	})
}

// Name returns the middleware name
func (mm *MetricsMiddleware) Name() string {
	return mm.config.Name
}

// Priority returns the middleware priority
func (mm *MetricsMiddleware) Priority() int {
	return mm.config.Priority
}

// Config returns the middleware configuration
func (mm *MetricsMiddleware) Config() interface{} {
	return mm.config
}

// Cleanup performs cleanup
func (mm *MetricsMiddleware) Cleanup() error {
	// Clear active requests
	mm.activeRequests.Range(func(key, value interface{}) bool {
		mm.activeRequests.Delete(key)
		return true
	})

	return nil
}

// shouldExcludeRequest checks if the request should be excluded from metrics
func (mm *MetricsMiddleware) shouldExcludeRequest(r *http.Request) bool {
	// Check excluded paths
	path := r.URL.Path
	if mm.excludePathMap[path] {
		return true
	}

	// Check path prefixes
	for excludePath := range mm.excludePathMap {
		if strings.HasPrefix(path, excludePath) {
			return true
		}
	}

	// Check excluded user agents
	userAgent := r.Header.Get("User-Agent")
	if userAgent != "" && mm.excludeUserAgentMap[userAgent] {
		return true
	}

	return false
}

// recordMetrics records all configured metrics
func (mm *MetricsMiddleware) recordMetrics(r *http.Request, rw *ResponseWriter, startTime time.Time) {
	duration := time.Since(startTime)
	labels := mm.buildLabels(r, rw)

	// Check if status code should be excluded
	if mm.excludeStatusCodeMap[rw.Status()] {
		return
	}

	// Record request metrics
	if mm.config.EnableRequestMetrics {
		mm.collector.IncrementCounter(mm.config.RequestMetricName, labels, 1)
	}

	// Record response size metrics
	if mm.config.EnableResponseMetrics {
		mm.collector.RecordHistogram(mm.config.ResponseSizeMetricName, labels, float64(rw.Written()))
	}

	// Record duration metrics
	if mm.config.EnableDurationMetrics {
		mm.collector.RecordHistogram(mm.config.DurationMetricName, labels, duration.Seconds())
	}

	// Record custom counter metrics
	for _, customCounter := range mm.config.CustomCounters {
		if mm.shouldRecordCustomMetric(&customCounter, r, rw) {
			value := mm.extractCustomMetricValue(&customCounter, r, rw)
			customLabels := mm.buildCustomLabels(&customCounter, r, rw)
			mm.collector.IncrementCounter(customCounter.Name, customLabels, value)
		}
	}

	// Record custom histogram metrics
	for _, customHistogram := range mm.config.CustomHistograms {
		if mm.shouldRecordCustomMetric(&customHistogram, r, rw) {
			value := mm.extractCustomMetricValue(&customHistogram, r, rw)
			customLabels := mm.buildCustomLabels(&customHistogram, r, rw)
			mm.collector.RecordHistogram(customHistogram.Name, customLabels, value)
		}
	}

	// Record custom gauge metrics
	for _, customGauge := range mm.config.CustomGauges {
		if mm.shouldRecordCustomMetric(&customGauge, r, rw) {
			value := mm.extractCustomMetricValue(&customGauge, r, rw)
			customLabels := mm.buildCustomLabels(&customGauge, r, rw)
			mm.collector.SetGauge(customGauge.Name, customLabels, value)
		}
	}
}

// buildLabels builds labels for metrics
func (mm *MetricsMiddleware) buildLabels(r *http.Request, rw *ResponseWriter) map[string]string {
	labels := make(map[string]string)

	if mm.config.IncludeMethod {
		labels["method"] = r.Method
	}

	if mm.config.IncludeStatus {
		labels["status"] = strconv.Itoa(rw.Status())
	}

	if mm.config.IncludePath {
		labels["path"] = mm.normalizePath(r.URL.Path)
	}

	if mm.config.IncludeUserAgent {
		if userAgent := r.Header.Get("User-Agent"); userAgent != "" {
			labels["user_agent"] = mm.normalizeUserAgent(userAgent)
		}
	}

	if mm.config.IncludeUserID {
		if userID := GetUserID(r.Context()); userID != "" {
			labels["user_id"] = userID
		}
	}

	// Add custom labels from headers
	for _, labelName := range mm.config.CustomLabels {
		if value := r.Header.Get(labelName); value != "" {
			labels[strings.ToLower(labelName)] = value
		}
	}

	return labels
}

// buildCustomLabels builds labels for custom metrics
func (mm *MetricsMiddleware) buildCustomLabels(config *CustomMetricConfig, r *http.Request, rw *ResponseWriter) map[string]string {
	labels := make(map[string]string)

	for _, labelName := range config.Labels {
		// Extract label value based on configuration
		if value := mm.extractLabelValue(labelName, r, rw); value != "" {
			labels[labelName] = value
		}
	}

	return labels
}

// extractLabelValue extracts a label value from request/response
func (mm *MetricsMiddleware) extractLabelValue(labelName string, r *http.Request, rw *ResponseWriter) string {
	switch labelName {
	case "method":
		return r.Method
	case "status":
		return strconv.Itoa(rw.Status())
	case "path":
		return mm.normalizePath(r.URL.Path)
	case "user_id":
		return GetUserID(r.Context())
	case "request_id":
		return GetRequestID(r.Context())
	default:
		// Try to get from header
		return r.Header.Get(labelName)
	}
}

// shouldRecordCustomMetric checks if a custom metric should be recorded
func (mm *MetricsMiddleware) shouldRecordCustomMetric(config *CustomMetricConfig, r *http.Request, rw *ResponseWriter) bool {
	if config.Condition == "" {
		return true
	}

	// Simple condition evaluation - in practice, use a proper expression evaluator
	switch config.Condition {
	case "error":
		return rw.Status() >= 400
	case "success":
		return rw.Status() < 400
	case "client_error":
		return rw.Status() >= 400 && rw.Status() < 500
	case "server_error":
		return rw.Status() >= 500
	default:
		return true
	}
}

// extractCustomMetricValue extracts the value for a custom metric
func (mm *MetricsMiddleware) extractCustomMetricValue(config *CustomMetricConfig, r *http.Request, rw *ResponseWriter) float64 {
	switch config.ValueFrom {
	case "response_size":
		return float64(rw.Written())
	case "status_code":
		return float64(rw.Status())
	case "constant":
		return 1.0
	default:
		// Try to parse from header
		if value := r.Header.Get(config.ValueFrom); value != "" {
			if parsed, err := strconv.ParseFloat(value, 64); err == nil {
				return parsed
			}
		}
		return 1.0
	}
}

// normalizePath normalizes request paths for consistent metrics
func (mm *MetricsMiddleware) normalizePath(path string) string {
	if !mm.config.NormalizePaths {
		return path
	}

	// Apply path replacements
	for pattern, replacement := range mm.config.PathReplacements {
		if strings.Contains(path, pattern) {
			path = strings.ReplaceAll(path, pattern, replacement)
		}
	}

	// Simple ID replacement - in practice, use regex
	// Replace common ID patterns like /users/123 -> /users/{id}
	parts := strings.Split(path, "/")
	for i, part := range parts {
		// Check if part looks like an ID (all digits)
		if len(part) > 0 && mm.isNumeric(part) {
			parts[i] = "{id}"
		}
		// Check if part looks like a UUID
		if len(part) == 36 && strings.Count(part, "-") == 4 {
			parts[i] = "{uuid}"
		}
	}

	return strings.Join(parts, "/")
}

// normalizeUserAgent normalizes user agent strings
func (mm *MetricsMiddleware) normalizeUserAgent(userAgent string) string {
	// Simplified user agent normalization
	if strings.Contains(userAgent, "Chrome") {
		return "Chrome"
	}
	if strings.Contains(userAgent, "Firefox") {
		return "Firefox"
	}
	if strings.Contains(userAgent, "Safari") && !strings.Contains(userAgent, "Chrome") {
		return "Safari"
	}
	if strings.Contains(userAgent, "curl") {
		return "curl"
	}
	if strings.Contains(userAgent, "wget") {
		return "wget"
	}

	return "other"
}

// isNumeric checks if a string is numeric
func (mm *MetricsMiddleware) isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// updateActiveRequestsGauge updates the active requests gauge
func (mm *MetricsMiddleware) updateActiveRequestsGauge(delta int) {
	count := 0
	mm.activeRequests.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	labels := map[string]string{}
	mm.collector.SetGauge(mm.config.ActiveRequestsMetricName, labels, float64(count))
}

// buildMaps precomputes maps for performance
func (mm *MetricsMiddleware) buildMaps() {
	// Build exclude path map
	for _, path := range mm.config.ExcludePaths {
		mm.excludePathMap[path] = true
	}

	// Build exclude status code map
	for _, statusCode := range mm.config.ExcludeStatusCodes {
		mm.excludeStatusCodeMap[statusCode] = true
	}

	// Build exclude user agent map
	for _, userAgent := range mm.config.ExcludeUserAgents {
		mm.excludeUserAgentMap[userAgent] = true
	}
}

// GetCollector returns the metrics collector
func (mm *MetricsMiddleware) GetCollector() MetricsCollector {
	return mm.collector
}

// GetActiveRequests returns the number of active requests
func (mm *MetricsMiddleware) GetActiveRequests() int {
	count := 0
	mm.activeRequests.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// Default configurations

// DefaultMetricsConfig returns a default metrics configuration
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		BaseConfig: BaseConfig{
			Enabled:  true,
			Priority: 20,
			Name:     "metrics",
		},
		EnableRequestMetrics:     true,
		RequestMetricName:        "http_requests_total",
		EnableResponseMetrics:    true,
		ResponseSizeMetricName:   "http_response_size_bytes",
		EnableDurationMetrics:    true,
		DurationMetricName:       "http_request_duration_seconds",
		EnableActiveRequests:     true,
		ActiveRequestsMetricName: "http_requests_active",
		IncludeMethod:            true,
		IncludeStatus:            true,
		IncludePath:              true,
		IncludeUserAgent:         false,
		IncludeUserID:            true,
		NormalizePaths:           true,
		PathReplacements: map[string]string{
			"/api/v1": "/api/{version}",
			"/api/v2": "/api/{version}",
		},
		ExcludePaths:       []string{"/health", "/metrics", "/favicon.ico"},
		ExcludeStatusCodes: []int{},
		SampleRate:         1.0,
	}
}

// BasicMetricsConfig returns a basic metrics configuration
func BasicMetricsConfig(collector MetricsCollector) *MetricsConfig {
	config := DefaultMetricsConfig()
	config.Collector = collector
	config.EnableResponseMetrics = false
	config.EnableActiveRequests = false
	config.IncludeUserAgent = false
	config.IncludeUserID = false
	return config
}

// DetailedMetricsConfig returns a detailed metrics configuration
func DetailedMetricsConfig(collector MetricsCollector) *MetricsConfig {
	config := DefaultMetricsConfig()
	config.Collector = collector
	config.IncludeUserAgent = true
	config.IncludeUserID = true
	config.CustomCounters = []CustomMetricConfig{
		{
			Name:      "http_errors_total",
			Help:      "Total HTTP errors",
			Labels:    []string{"method", "status"},
			ValueFrom: "constant",
			Condition: "error",
		},
	}
	return config
}

// MetricsHandler creates an HTTP handler that exposes metrics
func MetricsHandler(collector MetricsCollector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Return metrics in JSON format
		// In practice, you might want to support Prometheus format
		w.Header().Set("Content-Type", "application/json")

		if imc, ok := collector.(*InMemoryMetricsCollector); ok {
			metrics := imc.GetMetrics()
			if err := json.NewEncoder(w).Encode(metrics); err != nil {
				http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
				return
			}
		} else {
			w.Write([]byte(`{"error": "metrics not available"}`))
		}
	}
}
