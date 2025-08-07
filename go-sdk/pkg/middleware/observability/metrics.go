package observability

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// MetricType represents different types of metrics
type MetricType int

const (
	MetricTypeCounter MetricType = iota
	MetricTypeGauge
	MetricTypeHistogram
	MetricTypeSummary
)

// Metric represents a generic metric
type Metric interface {
	// Name returns the metric name
	Name() string

	// Type returns the metric type
	Type() MetricType

	// Labels returns the metric labels
	Labels() map[string]string

	// Value returns the current metric value
	Value() interface{}

	// Reset resets the metric to its initial state
	Reset()
}

// Counter represents a counter metric that only increases
type Counter struct {
	name   string
	labels map[string]string
	value  int64
}

// NewCounter creates a new counter metric
func NewCounter(name string, labels map[string]string) *Counter {
	return &Counter{
		name:   name,
		labels: labels,
		value:  0,
	}
}

// Name returns the metric name
func (c *Counter) Name() string {
	return c.name
}

// Type returns the metric type
func (c *Counter) Type() MetricType {
	return MetricTypeCounter
}

// Labels returns the metric labels
func (c *Counter) Labels() map[string]string {
	return c.labels
}

// Value returns the current counter value
func (c *Counter) Value() interface{} {
	return atomic.LoadInt64(&c.value)
}

// Reset resets the counter to zero
func (c *Counter) Reset() {
	atomic.StoreInt64(&c.value, 0)
}

// Inc increments the counter by 1
func (c *Counter) Inc() {
	atomic.AddInt64(&c.value, 1)
}

// Add adds the given value to the counter
func (c *Counter) Add(value int64) {
	atomic.AddInt64(&c.value, value)
}

// Gauge represents a gauge metric that can increase or decrease
type Gauge struct {
	name   string
	labels map[string]string
	value  int64
}

// NewGauge creates a new gauge metric
func NewGauge(name string, labels map[string]string) *Gauge {
	return &Gauge{
		name:   name,
		labels: labels,
		value:  0,
	}
}

// Name returns the metric name
func (g *Gauge) Name() string {
	return g.name
}

// Type returns the metric type
func (g *Gauge) Type() MetricType {
	return MetricTypeGauge
}

// Labels returns the metric labels
func (g *Gauge) Labels() map[string]string {
	return g.labels
}

// Value returns the current gauge value
func (g *Gauge) Value() interface{} {
	return atomic.LoadInt64(&g.value)
}

// Reset resets the gauge to zero
func (g *Gauge) Reset() {
	atomic.StoreInt64(&g.value, 0)
}

// Set sets the gauge to the given value
func (g *Gauge) Set(value int64) {
	atomic.StoreInt64(&g.value, value)
}

// Inc increments the gauge by 1
func (g *Gauge) Inc() {
	atomic.AddInt64(&g.value, 1)
}

// Dec decrements the gauge by 1
func (g *Gauge) Dec() {
	atomic.AddInt64(&g.value, -1)
}

// Add adds the given value to the gauge
func (g *Gauge) Add(value int64) {
	atomic.AddInt64(&g.value, value)
}

// Sub subtracts the given value from the gauge
func (g *Gauge) Sub(value int64) {
	atomic.AddInt64(&g.value, -value)
}

// Histogram represents a histogram metric for measuring distributions
type Histogram struct {
	name    string
	labels  map[string]string
	buckets []float64
	counts  []int64
	sum     int64
	count   int64
	mu      sync.RWMutex
}

// NewHistogram creates a new histogram metric
func NewHistogram(name string, labels map[string]string, buckets []float64) *Histogram {
	if buckets == nil {
		// Default buckets for response time in milliseconds
		buckets = []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}
	}

	return &Histogram{
		name:    name,
		labels:  labels,
		buckets: buckets,
		counts:  make([]int64, len(buckets)+1), // +1 for +Inf bucket
	}
}

// Name returns the metric name
func (h *Histogram) Name() string {
	return h.name
}

// Type returns the metric type
func (h *Histogram) Type() MetricType {
	return MetricTypeHistogram
}

// Labels returns the metric labels
func (h *Histogram) Labels() map[string]string {
	return h.labels
}

// Value returns the histogram data
func (h *Histogram) Value() interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return map[string]interface{}{
		"buckets": h.buckets,
		"counts":  append([]int64(nil), h.counts...),
		"sum":     atomic.LoadInt64(&h.sum),
		"count":   atomic.LoadInt64(&h.count),
	}
}

// Reset resets the histogram to its initial state
func (h *Histogram) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i := range h.counts {
		atomic.StoreInt64(&h.counts[i], 0)
	}
	atomic.StoreInt64(&h.sum, 0)
	atomic.StoreInt64(&h.count, 0)
}

// Observe adds an observation to the histogram
func (h *Histogram) Observe(value float64) {
	atomic.AddInt64(&h.count, 1)
	atomic.AddInt64(&h.sum, int64(value*1000)) // Store as microseconds

	// Find the appropriate bucket
	bucketIndex := len(h.buckets) // Default to +Inf bucket
	for i, bucket := range h.buckets {
		if value <= bucket {
			bucketIndex = i
			break
		}
	}

	atomic.AddInt64(&h.counts[bucketIndex], 1)
}

// MetricsCollector collects and manages metrics
type MetricsCollector struct {
	metrics map[string]Metric
	mu      sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: make(map[string]Metric),
	}
}

// RegisterMetric registers a new metric
func (mc *MetricsCollector) RegisterMetric(metric Metric) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	key := mc.metricKey(metric.Name(), metric.Labels())
	mc.metrics[key] = metric
}

// GetMetric retrieves a metric by name and labels
func (mc *MetricsCollector) GetMetric(name string, labels map[string]string) Metric {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	key := mc.metricKey(name, labels)
	return mc.metrics[key]
}

// GetOrCreateCounter gets or creates a counter metric
func (mc *MetricsCollector) GetOrCreateCounter(name string, labels map[string]string) *Counter {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	key := mc.metricKey(name, labels)
	if metric, exists := mc.metrics[key]; exists {
		if counter, ok := metric.(*Counter); ok {
			return counter
		}
	}

	counter := NewCounter(name, labels)
	mc.metrics[key] = counter
	return counter
}

// GetOrCreateGauge gets or creates a gauge metric
func (mc *MetricsCollector) GetOrCreateGauge(name string, labels map[string]string) *Gauge {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	key := mc.metricKey(name, labels)
	if metric, exists := mc.metrics[key]; exists {
		if gauge, ok := metric.(*Gauge); ok {
			return gauge
		}
	}

	gauge := NewGauge(name, labels)
	mc.metrics[key] = gauge
	return gauge
}

// GetOrCreateHistogram gets or creates a histogram metric
func (mc *MetricsCollector) GetOrCreateHistogram(name string, labels map[string]string, buckets []float64) *Histogram {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	key := mc.metricKey(name, labels)
	if metric, exists := mc.metrics[key]; exists {
		if histogram, ok := metric.(*Histogram); ok {
			return histogram
		}
	}

	histogram := NewHistogram(name, labels, buckets)
	mc.metrics[key] = histogram
	return histogram
}

// GetAllMetrics returns all registered metrics
func (mc *MetricsCollector) GetAllMetrics() map[string]Metric {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]Metric, len(mc.metrics))
	for k, v := range mc.metrics {
		result[k] = v
	}

	return result
}

// Reset resets all metrics
func (mc *MetricsCollector) Reset() {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	for _, metric := range mc.metrics {
		metric.Reset()
	}
}

// metricKey generates a unique key for a metric
func (mc *MetricsCollector) metricKey(name string, labels map[string]string) string {
	key := name
	if len(labels) > 0 {
		key += "{"
		first := true
		for k, v := range labels {
			if !first {
				key += ","
			}
			key += k + "=" + v
			first = false
		}
		key += "}"
	}
	return key
}

// MetricsConfig represents metrics middleware configuration
type MetricsConfig struct {
	EnableRequestCount    bool          `json:"enable_request_count" yaml:"enable_request_count"`
	EnableRequestDuration bool          `json:"enable_request_duration" yaml:"enable_request_duration"`
	EnableRequestSize     bool          `json:"enable_request_size" yaml:"enable_request_size"`
	EnableResponseSize    bool          `json:"enable_response_size" yaml:"enable_response_size"`
	EnableActiveRequests  bool          `json:"enable_active_requests" yaml:"enable_active_requests"`
	DurationBuckets       []float64     `json:"duration_buckets" yaml:"duration_buckets"`
	SizeBuckets           []float64     `json:"size_buckets" yaml:"size_buckets"`
	SkipPaths             []string      `json:"skip_paths" yaml:"skip_paths"`
	SkipHealthCheck       bool          `json:"skip_health_check" yaml:"skip_health_check"`
	CollectionInterval    time.Duration `json:"collection_interval" yaml:"collection_interval"`
}

// MetricsMiddleware implements metrics collection middleware
type MetricsMiddleware struct {
	config    *MetricsConfig
	collector *MetricsCollector
	enabled   bool
	priority  int
	skipMap   map[string]bool

	// Pre-created metrics
	requestCount    *Counter
	requestDuration *Histogram
	requestSize     *Histogram
	responseSize    *Histogram
	activeRequests  *Gauge
	errorCount      *Counter
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware(config *MetricsConfig, collector *MetricsCollector) *MetricsMiddleware {
	if config == nil {
		config = &MetricsConfig{
			EnableRequestCount:    true,
			EnableRequestDuration: true,
			EnableRequestSize:     true,
			EnableResponseSize:    true,
			EnableActiveRequests:  true,
			SkipHealthCheck:       true,
			CollectionInterval:    30 * time.Second,
		}
	}

	if collector == nil {
		collector = NewMetricsCollector()
	}

	skipMap := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipMap[path] = true
	}

	// Add common health check paths
	if config.SkipHealthCheck {
		skipMap["/health"] = true
		skipMap["/healthz"] = true
		skipMap["/ping"] = true
		skipMap["/ready"] = true
		skipMap["/live"] = true
	}

	m := &MetricsMiddleware{
		config:    config,
		collector: collector,
		enabled:   true,
		priority:  5, // Low priority, should run early but after auth/logging
		skipMap:   skipMap,
	}

	// Initialize metrics
	m.initializeMetrics()

	return m
}

// Name returns middleware name
func (m *MetricsMiddleware) Name() string {
	return "metrics"
}

// Process processes the request through metrics middleware
func (m *MetricsMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Skip metrics for configured paths
	if m.skipMap[req.Path] {
		return next(ctx, req)
	}

	startTime := time.Now()

	// Increment active requests
	if m.config.EnableActiveRequests && m.activeRequests != nil {
		m.activeRequests.Inc()
		defer m.activeRequests.Dec()
	}

	// Record request size
	if m.config.EnableRequestSize && m.requestSize != nil && req.Body != nil {
		// Estimate request size (simplified)
		size := float64(len(req.Method) + len(req.Path))
		for k, v := range req.Headers {
			size += float64(len(k) + len(v))
		}
		// Note: In a real implementation, you'd properly calculate request body size
		m.requestSize.Observe(size)
	}

	// Process request through next middleware
	resp, err := next(ctx, req)

	// Calculate duration
	duration := time.Since(startTime)

	// Record metrics
	m.recordRequestMetrics(req, resp, err, duration)

	return resp, err
}

// Configure configures the middleware
func (m *MetricsMiddleware) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		m.enabled = enabled
	}

	if priority, ok := config["priority"].(int); ok {
		m.priority = priority
	}

	return nil
}

// Enabled returns whether the middleware is enabled
func (m *MetricsMiddleware) Enabled() bool {
	return m.enabled
}

// Priority returns the middleware priority
func (m *MetricsMiddleware) Priority() int {
	return m.priority
}

// GetCollector returns the metrics collector
func (m *MetricsMiddleware) GetCollector() *MetricsCollector {
	return m.collector
}

// initializeMetrics initializes the middleware metrics
func (m *MetricsMiddleware) initializeMetrics() {
	// Request count
	if m.config.EnableRequestCount {
		m.requestCount = m.collector.GetOrCreateCounter("http_requests_total", nil)
	}

	// Request duration histogram
	if m.config.EnableRequestDuration {
		buckets := m.config.DurationBuckets
		if buckets == nil {
			// Default duration buckets in milliseconds
			buckets = []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}
		}
		m.requestDuration = m.collector.GetOrCreateHistogram("http_request_duration_ms", nil, buckets)
	}

	// Request size histogram
	if m.config.EnableRequestSize {
		buckets := m.config.SizeBuckets
		if buckets == nil {
			// Default size buckets in bytes
			buckets = []float64{100, 500, 1000, 5000, 10000, 50000, 100000, 500000, 1000000}
		}
		m.requestSize = m.collector.GetOrCreateHistogram("http_request_size_bytes", nil, buckets)
	}

	// Response size histogram
	if m.config.EnableResponseSize {
		buckets := m.config.SizeBuckets
		if buckets == nil {
			// Default size buckets in bytes
			buckets = []float64{100, 500, 1000, 5000, 10000, 50000, 100000, 500000, 1000000}
		}
		m.responseSize = m.collector.GetOrCreateHistogram("http_response_size_bytes", nil, buckets)
	}

	// Active requests gauge
	if m.config.EnableActiveRequests {
		m.activeRequests = m.collector.GetOrCreateGauge("http_requests_active", nil)
	}

	// Error count
	m.errorCount = m.collector.GetOrCreateCounter("http_errors_total", nil)
}

// recordRequestMetrics records metrics for a completed request
func (m *MetricsMiddleware) recordRequestMetrics(req *Request, resp *Response, err error, duration time.Duration) {
	// Request count with labels
	if m.requestCount != nil {
		labels := map[string]string{
			"method": req.Method,
			"path":   req.Path,
		}

		if resp != nil {
			labels["status_code"] = getStatusCodeClass(resp.StatusCode)
		}

		counter := m.collector.GetOrCreateCounter("http_requests_total", labels)
		counter.Inc()
	}

	// Request duration
	if m.requestDuration != nil {
		labels := map[string]string{
			"method": req.Method,
			"path":   req.Path,
		}

		if resp != nil {
			labels["status_code"] = getStatusCodeClass(resp.StatusCode)
		}

		histogram := m.collector.GetOrCreateHistogram("http_request_duration_ms", labels, m.config.DurationBuckets)
		histogram.Observe(float64(duration.Nanoseconds()) / 1000000) // Convert to milliseconds
	}

	// Response size
	if m.config.EnableResponseSize && m.responseSize != nil && resp != nil && resp.Body != nil {
		// Estimate response size (simplified)
		size := float64(0)
		for k, v := range resp.Headers {
			size += float64(len(k) + len(v))
		}
		// Note: In a real implementation, you'd properly calculate response body size
		m.responseSize.Observe(size)
	}

	// Error count
	if err != nil || (resp != nil && resp.StatusCode >= 400) {
		if m.errorCount != nil {
			labels := map[string]string{
				"method": req.Method,
				"path":   req.Path,
			}

			if resp != nil {
				labels["status_code"] = getStatusCodeClass(resp.StatusCode)
			}

			errorCounter := m.collector.GetOrCreateCounter("http_errors_total", labels)
			errorCounter.Inc()
		}
	}
}

// getStatusCodeClass returns the status code class (1xx, 2xx, etc.)
func getStatusCodeClass(statusCode int) string {
	switch {
	case statusCode >= 100 && statusCode < 200:
		return "1xx"
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500 && statusCode < 600:
		return "5xx"
	default:
		return "unknown"
	}
}

// MetricsExporter exports metrics in various formats
type MetricsExporter struct {
	collector *MetricsCollector
}

// NewMetricsExporter creates a new metrics exporter
func NewMetricsExporter(collector *MetricsCollector) *MetricsExporter {
	return &MetricsExporter{
		collector: collector,
	}
}

// ExportPrometheus exports metrics in Prometheus format
func (me *MetricsExporter) ExportPrometheus() string {
	metrics := me.collector.GetAllMetrics()
	output := ""

	for _, metric := range metrics {
		switch m := metric.(type) {
		case *Counter:
			output += me.formatPrometheusMetric(m.Name(), "counter", m.Labels(), m.Value())
		case *Gauge:
			output += me.formatPrometheusMetric(m.Name(), "gauge", m.Labels(), m.Value())
		case *Histogram:
			output += me.formatPrometheusHistogram(m)
		}
	}

	return output
}

// ExportJSON exports metrics in JSON format
func (me *MetricsExporter) ExportJSON() map[string]interface{} {
	metrics := me.collector.GetAllMetrics()
	result := make(map[string]interface{})

	for key, metric := range metrics {
		result[key] = map[string]interface{}{
			"name":   metric.Name(),
			"type":   metric.Type(),
			"labels": metric.Labels(),
			"value":  metric.Value(),
		}
	}

	return result
}

// formatPrometheusMetric formats a metric in Prometheus format
func (me *MetricsExporter) formatPrometheusMetric(name, metricType string, labels map[string]string, value interface{}) string {
	output := fmt.Sprintf("# TYPE %s %s\n", name, metricType)

	labelsStr := ""
	if len(labels) > 0 {
		labelsStr = "{"
		first := true
		for k, v := range labels {
			if !first {
				labelsStr += ","
			}
			labelsStr += fmt.Sprintf(`%s="%s"`, k, v)
			first = false
		}
		labelsStr += "}"
	}

	output += fmt.Sprintf("%s%s %v\n", name, labelsStr, value)
	return output
}

// formatPrometheusHistogram formats a histogram in Prometheus format
func (me *MetricsExporter) formatPrometheusHistogram(histogram *Histogram) string {
	name := histogram.Name()
	labels := histogram.Labels()
	value := histogram.Value()

	histData, ok := value.(map[string]interface{})
	if !ok {
		return ""
	}

	output := fmt.Sprintf("# TYPE %s histogram\n", name)

	buckets, _ := histData["buckets"].([]float64)
	counts, _ := histData["counts"].([]int64)
	sum, _ := histData["sum"].(int64)
	count, _ := histData["count"].(int64)

	// Format bucket metrics
	for i, bucket := range buckets {
		labelsStr := me.formatLabels(labels, map[string]string{"le": fmt.Sprintf("%.1f", bucket)})
		output += fmt.Sprintf("%s_bucket%s %d\n", name, labelsStr, counts[i])
	}

	// +Inf bucket
	infLabelsStr := me.formatLabels(labels, map[string]string{"le": "+Inf"})
	output += fmt.Sprintf("%s_bucket%s %d\n", name, infLabelsStr, counts[len(counts)-1])

	// Sum and count
	sumLabelsStr := me.formatLabels(labels, nil)
	output += fmt.Sprintf("%s_sum%s %d\n", name, sumLabelsStr, sum)
	output += fmt.Sprintf("%s_count%s %d\n", name, sumLabelsStr, count)

	return output
}

// formatLabels formats labels for Prometheus output
func (me *MetricsExporter) formatLabels(baseLabels, extraLabels map[string]string) string {
	allLabels := make(map[string]string)

	// Copy base labels
	for k, v := range baseLabels {
		allLabels[k] = v
	}

	// Add extra labels
	for k, v := range extraLabels {
		allLabels[k] = v
	}

	if len(allLabels) == 0 {
		return ""
	}

	labelsStr := "{"
	first := true
	for k, v := range allLabels {
		if !first {
			labelsStr += ","
		}
		labelsStr += fmt.Sprintf(`%s="%s"`, k, v)
		first = false
	}
	labelsStr += "}"

	return labelsStr
}
