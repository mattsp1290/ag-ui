package monitoring

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusExporter exports metrics to Prometheus
type PrometheusExporter struct {
	config           *Config
	metricsCollector events.MetricsCollector
	server           *http.Server
	
	// Standard metrics
	eventCounter        *prometheus.CounterVec
	eventDuration       *prometheus.HistogramVec
	ruleExecutionHist   *prometheus.HistogramVec
	errorCounter        *prometheus.CounterVec
	warningCounter      *prometheus.CounterVec
	
	// Gauge metrics
	memoryUsage         *prometheus.GaugeVec
	throughput          prometheus.Gauge
	slaCompliance       *prometheus.GaugeVec
	activeRules         prometheus.Gauge
	
	// Summary metrics
	latencySummary      *prometheus.SummaryVec
	
	mu sync.RWMutex
}

// NewPrometheusExporter creates a new Prometheus exporter
func NewPrometheusExporter(config *Config, collector events.MetricsCollector) *PrometheusExporter {
	labels := []string{"status"}
	if config.EnableCustomLabels {
		for k := range config.CustomLabels {
			labels = append(labels, k)
		}
	}
	
	pe := &PrometheusExporter{
		config:           config,
		metricsCollector: collector,
		
		eventCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ag_ui",
				Subsystem: "events",
				Name:      "validation_total",
				Help:      "Total number of event validations",
			},
			labels,
		),
		
		eventDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ag_ui",
				Subsystem: "events",
				Name:      "validation_duration_seconds",
				Help:      "Event validation duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			labels,
		),
		
		ruleExecutionHist: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ag_ui",
				Subsystem: "events",
				Name:      "rule_execution_duration_seconds",
				Help:      "Rule execution duration in seconds",
				Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
			},
			append([]string{"rule_id"}, labels...),
		),
		
		errorCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ag_ui",
				Subsystem: "events",
				Name:      "validation_errors_total",
				Help:      "Total number of validation errors",
			},
			[]string{"error_type"},
		),
		
		warningCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ag_ui",
				Subsystem: "events",
				Name:      "validation_warnings_total",
				Help:      "Total number of validation warnings",
			},
			[]string{"warning_type"},
		),
		
		memoryUsage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "ag_ui",
				Subsystem: "events",
				Name:      "memory_usage_bytes",
				Help:      "Current memory usage in bytes",
			},
			[]string{"type"},
		),
		
		throughput: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "ag_ui",
				Subsystem: "events",
				Name:      "throughput_events_per_second",
				Help:      "Current event processing throughput",
			},
		),
		
		slaCompliance: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "ag_ui",
				Subsystem: "events",
				Name:      "sla_compliance_percent",
				Help:      "SLA compliance percentage",
			},
			[]string{"sla_name"},
		),
		
		activeRules: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "ag_ui",
				Subsystem: "events",
				Name:      "active_rules_count",
				Help:      "Number of active validation rules",
			},
		),
		
		latencySummary: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Namespace:  "ag_ui",
				Subsystem:  "events",
				Name:       "validation_latency_summary",
				Help:       "Summary of validation latencies",
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			},
			labels,
		),
	}
	
	// Register all metrics
	prometheus.MustRegister(
		pe.eventCounter,
		pe.eventDuration,
		pe.ruleExecutionHist,
		pe.errorCounter,
		pe.warningCounter,
		pe.memoryUsage,
		pe.throughput,
		pe.slaCompliance,
		pe.activeRules,
		pe.latencySummary,
	)
	
	// Start metrics update routine
	go pe.updateMetricsRoutine()
	
	return pe
}

// Start starts the Prometheus HTTP server
func (pe *PrometheusExporter) Start() error {
	mux := http.NewServeMux()
	mux.Handle(pe.config.PrometheusPath, promhttp.Handler())
	mux.HandleFunc("/health", pe.healthHandler)
	mux.HandleFunc("/ready", pe.readyHandler)
	
	pe.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", pe.config.PrometheusPort),
		Handler: mux,
	}
	
	return pe.server.ListenAndServe()
}

// Shutdown gracefully shuts down the Prometheus server
func (pe *PrometheusExporter) Shutdown() error {
	if pe.server != nil {
		return pe.server.Close()
	}
	return nil
}

// updateMetricsRoutine periodically updates Prometheus metrics from the collector
func (pe *PrometheusExporter) updateMetricsRoutine() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		pe.updateMetrics()
	}
}

// updateMetrics updates Prometheus metrics from the metrics collector
func (pe *PrometheusExporter) updateMetrics() {
	dashboard := pe.metricsCollector.GetDashboardData()
	if dashboard == nil {
		return
	}
	
	// Update throughput
	pe.throughput.Set(dashboard.EventsPerSecond)
	
	// Update active rules
	pe.activeRules.Set(float64(dashboard.ActiveRules))
	
	// Update memory usage
	if dashboard.MemoryUsage != nil {
		pe.memoryUsage.WithLabelValues("allocated").Set(float64(dashboard.MemoryUsage.AllocBytes))
		pe.memoryUsage.WithLabelValues("heap_inuse").Set(float64(dashboard.MemoryUsage.HeapInuse))
		pe.memoryUsage.WithLabelValues("stack_inuse").Set(float64(dashboard.MemoryUsage.StackInuse))
	}
	
	// Update SLA compliance
	pe.slaCompliance.WithLabelValues("overall").Set(dashboard.SLACompliance)
	
	// Update rule metrics
	ruleMetrics := pe.metricsCollector.GetAllRuleMetrics()
	for ruleID, metric := range ruleMetrics {
		labels := prometheus.Labels{
			"rule_id": ruleID,
			"status":  "success",
		}
		pe.addCustomLabels(labels)
		
		// Record execution counts and durations
		execCount := metric.GetExecutionCount()
		avgDuration := metric.GetAverageDuration()
		
		if execCount > 0 && avgDuration > 0 {
			pe.ruleExecutionHist.With(labels).Observe(avgDuration.Seconds())
		}
	}
}

// RecordEvent records an event validation
func (pe *PrometheusExporter) RecordEvent(duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	
	labels := prometheus.Labels{"status": status}
	pe.addCustomLabels(labels)
	
	pe.eventCounter.With(labels).Inc()
	pe.eventDuration.With(labels).Observe(duration.Seconds())
	pe.latencySummary.With(labels).Observe(duration.Seconds())
}

// RecordError records a validation error
func (pe *PrometheusExporter) RecordError(errorType string) {
	labels := prometheus.Labels{"error_type": errorType}
	pe.errorCounter.With(labels).Inc()
}

// RecordWarning records a validation warning
func (pe *PrometheusExporter) RecordWarning(warningType string) {
	labels := prometheus.Labels{"warning_type": warningType}
	pe.warningCounter.With(labels).Inc()
}

// addCustomLabels adds custom labels to a label set
func (pe *PrometheusExporter) addCustomLabels(labels prometheus.Labels) {
	if pe.config.EnableCustomLabels {
		for k, v := range pe.config.CustomLabels {
			labels[k] = v
		}
	}
}

// healthHandler handles health check requests
func (pe *PrometheusExporter) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// readyHandler handles readiness check requests
func (pe *PrometheusExporter) readyHandler(w http.ResponseWriter, r *http.Request) {
	// Check if metrics collector is ready
	stats := pe.metricsCollector.GetOverallStats()
	if stats == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Not Ready"))
		return
	}
	
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Ready"))
}

// Custom Prometheus Collectors

// EventValidationCollector collects event validation metrics
type EventValidationCollector struct {
	metricsCollector events.MetricsCollector
	customLabels     map[string]string
}

// NewEventValidationCollector creates a new event validation collector
func NewEventValidationCollector(collector events.MetricsCollector, customLabels map[string]string) *EventValidationCollector {
	return &EventValidationCollector{
		metricsCollector: collector,
		customLabels:     customLabels,
	}
}

// Describe sends the metric descriptions to Prometheus
func (c *EventValidationCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- prometheus.NewDesc("ag_ui_event_validation_rate", "Event validation rate", nil, c.customLabels)
	ch <- prometheus.NewDesc("ag_ui_event_error_rate", "Event error rate", nil, c.customLabels)
	ch <- prometheus.NewDesc("ag_ui_event_latency_p50", "Event latency 50th percentile", nil, c.customLabels)
	ch <- prometheus.NewDesc("ag_ui_event_latency_p99", "Event latency 99th percentile", nil, c.customLabels)
}

// Collect collects the metrics from the collector
func (c *EventValidationCollector) Collect(ch chan<- prometheus.Metric) {
	dashboard := c.metricsCollector.GetDashboardData()
	if dashboard == nil {
		return
	}
	
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("ag_ui_event_validation_rate", "Event validation rate", nil, c.customLabels),
		prometheus.GaugeValue,
		dashboard.EventsPerSecond,
	)
	
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("ag_ui_event_error_rate", "Event error rate", nil, c.customLabels),
		prometheus.GaugeValue,
		dashboard.ErrorRate,
	)
}

// RuleExecutionCollector collects rule execution metrics
type RuleExecutionCollector struct {
	metricsCollector events.MetricsCollector
	customLabels     map[string]string
}

// NewRuleExecutionCollector creates a new rule execution collector
func NewRuleExecutionCollector(collector events.MetricsCollector, customLabels map[string]string) *RuleExecutionCollector {
	return &RuleExecutionCollector{
		metricsCollector: collector,
		customLabels:     customLabels,
	}
}

// Describe sends the metric descriptions to Prometheus
func (c *RuleExecutionCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- prometheus.NewDesc("ag_ui_rule_execution_count", "Rule execution count", []string{"rule_id"}, c.customLabels)
	ch <- prometheus.NewDesc("ag_ui_rule_avg_duration", "Rule average duration", []string{"rule_id"}, c.customLabels)
	ch <- prometheus.NewDesc("ag_ui_rule_error_rate", "Rule error rate", []string{"rule_id"}, c.customLabels)
}

// Collect collects the metrics from the collector
func (c *RuleExecutionCollector) Collect(ch chan<- prometheus.Metric) {
	ruleMetrics := c.metricsCollector.GetAllRuleMetrics()
	
	for ruleID, metric := range ruleMetrics {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("ag_ui_rule_execution_count", "Rule execution count", []string{"rule_id"}, c.customLabels),
			prometheus.CounterValue,
			float64(metric.GetExecutionCount()),
			ruleID,
		)
		
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("ag_ui_rule_avg_duration", "Rule average duration", []string{"rule_id"}, c.customLabels),
			prometheus.GaugeValue,
			metric.GetAverageDuration().Seconds(),
			ruleID,
		)
		
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("ag_ui_rule_error_rate", "Rule error rate", []string{"rule_id"}, c.customLabels),
			prometheus.GaugeValue,
			metric.GetErrorRate(),
			ruleID,
		)
	}
}

// SLAComplianceCollector collects SLA compliance metrics
type SLAComplianceCollector struct {
	slaMonitor   *SLAMonitor
	customLabels map[string]string
}

// NewSLAComplianceCollector creates a new SLA compliance collector
func NewSLAComplianceCollector(monitor *SLAMonitor, customLabels map[string]string) *SLAComplianceCollector {
	return &SLAComplianceCollector{
		slaMonitor:   monitor,
		customLabels: customLabels,
	}
}

// Describe sends the metric descriptions to Prometheus
func (c *SLAComplianceCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- prometheus.NewDesc("ag_ui_sla_compliance", "SLA compliance status", []string{"sla_name", "target_type"}, c.customLabels)
	ch <- prometheus.NewDesc("ag_ui_sla_current_value", "SLA current value", []string{"sla_name", "unit"}, c.customLabels)
	ch <- prometheus.NewDesc("ag_ui_sla_violations", "SLA violation count", []string{"sla_name"}, c.customLabels)
}

// Collect collects the metrics from the collector
func (c *SLAComplianceCollector) Collect(ch chan<- prometheus.Metric) {
	slaStatus := c.slaMonitor.GetCurrentStatus()
	
	for slaName, status := range slaStatus {
		compliance := 1.0
		if status.IsViolated {
			compliance = 0.0
		}
		
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("ag_ui_sla_compliance", "SLA compliance status", []string{"sla_name", "target_type"}, c.customLabels),
			prometheus.GaugeValue,
			compliance,
			slaName,
			status.Target.Name,
		)
		
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("ag_ui_sla_current_value", "SLA current value", []string{"sla_name", "unit"}, c.customLabels),
			prometheus.GaugeValue,
			status.CurrentValue,
			slaName,
			status.Target.Unit,
		)
	}
}

// MemoryUsageCollector collects memory usage metrics
type MemoryUsageCollector struct {
	metricsCollector events.MetricsCollector
	customLabels     map[string]string
}

// NewMemoryUsageCollector creates a new memory usage collector
func NewMemoryUsageCollector(collector events.MetricsCollector, customLabels map[string]string) *MemoryUsageCollector {
	return &MemoryUsageCollector{
		metricsCollector: collector,
		customLabels:     customLabels,
	}
}

// Describe sends the metric descriptions to Prometheus
func (c *MemoryUsageCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- prometheus.NewDesc("ag_ui_memory_allocated_bytes", "Allocated memory in bytes", nil, c.customLabels)
	ch <- prometheus.NewDesc("ag_ui_memory_heap_objects", "Number of heap objects", nil, c.customLabels)
	ch <- prometheus.NewDesc("ag_ui_gc_cycles_total", "Total GC cycles", nil, c.customLabels)
	ch <- prometheus.NewDesc("ag_ui_gc_pause_total_seconds", "Total GC pause time", nil, c.customLabels)
}

// Collect collects the metrics from the collector
func (c *MemoryUsageCollector) Collect(ch chan<- prometheus.Metric) {
	memHistory := c.metricsCollector.GetMemoryHistory()
	if len(memHistory) == 0 {
		return
	}
	
	// Get the most recent memory stats
	latest := memHistory[len(memHistory)-1]
	
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("ag_ui_memory_allocated_bytes", "Allocated memory in bytes", nil, c.customLabels),
		prometheus.GaugeValue,
		float64(latest.AllocBytes),
	)
	
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("ag_ui_memory_heap_objects", "Number of heap objects", nil, c.customLabels),
		prometheus.GaugeValue,
		float64(latest.HeapObjects),
	)
	
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("ag_ui_gc_cycles_total", "Total GC cycles", nil, c.customLabels),
		prometheus.CounterValue,
		float64(latest.GCCycles),
	)
	
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc("ag_ui_gc_pause_total_seconds", "Total GC pause time", nil, c.customLabels),
		prometheus.CounterValue,
		latest.GCPauseTotal.Seconds(),
	)
}