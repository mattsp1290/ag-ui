package state

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"
)

// MonitoringConfigBuilder provides a fluent interface for building monitoring configuration
type MonitoringConfigBuilder struct {
	config MonitoringConfig
}

// NewMonitoringConfigBuilder creates a new monitoring configuration builder
func NewMonitoringConfigBuilder() *MonitoringConfigBuilder {
	return &MonitoringConfigBuilder{
		config: DefaultMonitoringConfig(),
	}
}

// WithPrometheus enables Prometheus metrics
func (b *MonitoringConfigBuilder) WithPrometheus(namespace, subsystem string) *MonitoringConfigBuilder {
	b.config.EnablePrometheus = true
	b.config.PrometheusNamespace = namespace
	b.config.PrometheusSubsystem = subsystem
	return b
}

// WithMetrics enables metrics collection
func (b *MonitoringConfigBuilder) WithMetrics(enabled bool, interval time.Duration) *MonitoringConfigBuilder {
	b.config.MetricsEnabled = enabled
	b.config.MetricsInterval = interval
	return b
}

// WithLogging configures logging
func (b *MonitoringConfigBuilder) WithLogging(level zapcore.Level, format string, output io.Writer) *MonitoringConfigBuilder {
	b.config.LogLevel = level
	b.config.LogFormat = format
	b.config.LogOutput = output
	b.config.StructuredLogging = format == "json"
	return b
}

// WithTracing enables distributed tracing
func (b *MonitoringConfigBuilder) WithTracing(serviceName, provider string, sampleRate float64) *MonitoringConfigBuilder {
	b.config.EnableTracing = true
	b.config.TracingServiceName = serviceName
	b.config.TracingProvider = provider
	b.config.TraceSampleRate = sampleRate
	return b
}

// WithHealthChecks configures health checks
func (b *MonitoringConfigBuilder) WithHealthChecks(enabled bool, interval, timeout time.Duration) *MonitoringConfigBuilder {
	b.config.EnableHealthChecks = enabled
	b.config.HealthCheckInterval = interval
	b.config.HealthCheckTimeout = timeout
	return b
}

// WithAlertThresholds configures alert thresholds
func (b *MonitoringConfigBuilder) WithAlertThresholds(thresholds AlertThresholds) *MonitoringConfigBuilder {
	b.config.AlertThresholds = thresholds
	return b
}

// WithAlertNotifiers adds alert notifiers
func (b *MonitoringConfigBuilder) WithAlertNotifiers(notifiers ...AlertNotifier) *MonitoringConfigBuilder {
	b.config.AlertNotifiers = append(b.config.AlertNotifiers, notifiers...)
	return b
}

// WithResourceMonitoring enables resource monitoring
func (b *MonitoringConfigBuilder) WithResourceMonitoring(enabled bool, interval time.Duration) *MonitoringConfigBuilder {
	b.config.EnableResourceMonitoring = enabled
	b.config.ResourceSampleInterval = interval
	return b
}

// WithAuditIntegration enables audit integration
func (b *MonitoringConfigBuilder) WithAuditIntegration(enabled bool, severity AuditSeverityLevel) *MonitoringConfigBuilder {
	b.config.AuditIntegration = enabled
	b.config.AuditSeverityLevel = severity
	return b
}

// WithProfiling enables profiling
func (b *MonitoringConfigBuilder) WithProfiling(enabled bool, cpuInterval, memoryInterval time.Duration) *MonitoringConfigBuilder {
	b.config.EnableProfiling = enabled
	b.config.CPUProfileInterval = cpuInterval
	b.config.MemoryProfileInterval = memoryInterval
	return b
}

// Build returns the configured monitoring configuration
func (b *MonitoringConfigBuilder) Build() MonitoringConfig {
	return b.config
}

// MonitoringConfigLoader provides methods to load configuration from various sources
type MonitoringConfigLoader struct{}

// NewMonitoringConfigLoader creates a new configuration loader
func NewMonitoringConfigLoader() *MonitoringConfigLoader {
	return &MonitoringConfigLoader{}
}

// LoadFromFile loads configuration from a JSON file
func (l *MonitoringConfigLoader) LoadFromFile(filename string) (MonitoringConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return MonitoringConfig{}, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	return l.LoadFromReader(file)
}

// LoadFromReader loads configuration from a reader
func (l *MonitoringConfigLoader) LoadFromReader(reader io.Reader) (MonitoringConfig, error) {
	var configData map[string]interface{}

	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&configData); err != nil {
		return MonitoringConfig{}, fmt.Errorf("failed to decode config: %w", err)
	}

	return l.parseConfig(configData)
}

// LoadFromEnv loads configuration from environment variables
func (l *MonitoringConfigLoader) LoadFromEnv() MonitoringConfig {
	config := DefaultMonitoringConfig()

	// Prometheus configuration
	if val := os.Getenv("MONITORING_PROMETHEUS_ENABLED"); val != "" {
		config.EnablePrometheus = val == "true"
	}
	if val := os.Getenv("MONITORING_PROMETHEUS_NAMESPACE"); val != "" {
		config.PrometheusNamespace = val
	}
	if val := os.Getenv("MONITORING_PROMETHEUS_SUBSYSTEM"); val != "" {
		config.PrometheusSubsystem = val
	}

	// Metrics configuration
	if val := os.Getenv("MONITORING_METRICS_ENABLED"); val != "" {
		config.MetricsEnabled = val == "true"
	}
	if val := os.Getenv("MONITORING_METRICS_INTERVAL"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.MetricsInterval = duration
		}
	}

	// Logging configuration
	if val := os.Getenv("MONITORING_LOG_LEVEL"); val != "" {
		if level, err := zapcore.ParseLevel(val); err == nil {
			config.LogLevel = level
		}
	}
	if val := os.Getenv("MONITORING_LOG_FORMAT"); val != "" {
		config.LogFormat = val
	}
	if val := os.Getenv("MONITORING_LOG_SAMPLING"); val != "" {
		config.LogSampling = val == "true"
	}

	// Tracing configuration
	if val := os.Getenv("MONITORING_TRACING_ENABLED"); val != "" {
		config.EnableTracing = val == "true"
	}
	if val := os.Getenv("MONITORING_TRACING_SERVICE_NAME"); val != "" {
		config.TracingServiceName = val
	}
	if val := os.Getenv("MONITORING_TRACING_PROVIDER"); val != "" {
		config.TracingProvider = val
	}
	if val := os.Getenv("MONITORING_TRACE_SAMPLE_RATE"); val != "" {
		if rate, err := parseFloat(val); err == nil {
			config.TraceSampleRate = rate
		}
	}

	// Health check configuration
	if val := os.Getenv("MONITORING_HEALTH_CHECKS_ENABLED"); val != "" {
		config.EnableHealthChecks = val == "true"
	}
	if val := os.Getenv("MONITORING_HEALTH_CHECK_INTERVAL"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.HealthCheckInterval = duration
		}
	}
	if val := os.Getenv("MONITORING_HEALTH_CHECK_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.HealthCheckTimeout = duration
		}
	}

	// Alert thresholds
	if val := os.Getenv("MONITORING_ALERT_ERROR_RATE"); val != "" {
		if rate, err := parseFloat(val); err == nil {
			config.AlertThresholds.ErrorRate = rate
		}
	}
	if val := os.Getenv("MONITORING_ALERT_P95_LATENCY_MS"); val != "" {
		if latency, err := parseFloat(val); err == nil {
			config.AlertThresholds.P95LatencyMs = latency
		}
	}
	if val := os.Getenv("MONITORING_ALERT_MEMORY_USAGE_PERCENT"); val != "" {
		if usage, err := parseFloat(val); err == nil {
			config.AlertThresholds.MemoryUsagePercent = usage
		}
	}

	// Resource monitoring
	if val := os.Getenv("MONITORING_RESOURCE_MONITORING_ENABLED"); val != "" {
		config.EnableResourceMonitoring = val == "true"
	}
	if val := os.Getenv("MONITORING_RESOURCE_SAMPLE_INTERVAL"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			config.ResourceSampleInterval = duration
		}
	}

	// Audit integration
	if val := os.Getenv("MONITORING_AUDIT_INTEGRATION"); val != "" {
		config.AuditIntegration = val == "true"
	}
	if val := os.Getenv("MONITORING_AUDIT_SEVERITY_LEVEL"); val != "" {
		config.AuditSeverityLevel = parseAuditSeverityLevel(val)
	}

	return config
}

// parseConfig parses configuration from a map
func (l *MonitoringConfigLoader) parseConfig(configData map[string]interface{}) (MonitoringConfig, error) {
	config := DefaultMonitoringConfig()

	// Prometheus configuration
	if prometheus, ok := configData["prometheus"].(map[string]interface{}); ok {
		if enabled, ok := prometheus["enabled"].(bool); ok {
			config.EnablePrometheus = enabled
		}
		if namespace, ok := prometheus["namespace"].(string); ok {
			config.PrometheusNamespace = namespace
		}
		if subsystem, ok := prometheus["subsystem"].(string); ok {
			config.PrometheusSubsystem = subsystem
		}
	}

	// Metrics configuration
	if metrics, ok := configData["metrics"].(map[string]interface{}); ok {
		if enabled, ok := metrics["enabled"].(bool); ok {
			config.MetricsEnabled = enabled
		}
		if interval, ok := metrics["interval"].(string); ok {
			if duration, err := time.ParseDuration(interval); err == nil {
				config.MetricsInterval = duration
			}
		}
	}

	// Logging configuration
	if logging, ok := configData["logging"].(map[string]interface{}); ok {
		if level, ok := logging["level"].(string); ok {
			if parsedLevel, err := zapcore.ParseLevel(level); err == nil {
				config.LogLevel = parsedLevel
			}
		}
		if format, ok := logging["format"].(string); ok {
			config.LogFormat = format
		}
		if structured, ok := logging["structured"].(bool); ok {
			config.StructuredLogging = structured
		}
		if sampling, ok := logging["sampling"].(bool); ok {
			config.LogSampling = sampling
		}
	}

	// Tracing configuration
	if tracing, ok := configData["tracing"].(map[string]interface{}); ok {
		if enabled, ok := tracing["enabled"].(bool); ok {
			config.EnableTracing = enabled
		}
		if serviceName, ok := tracing["service_name"].(string); ok {
			config.TracingServiceName = serviceName
		}
		if provider, ok := tracing["provider"].(string); ok {
			config.TracingProvider = provider
		}
		if sampleRate, ok := tracing["sample_rate"].(float64); ok {
			config.TraceSampleRate = sampleRate
		}
	}

	// Health checks configuration
	if healthChecks, ok := configData["health_checks"].(map[string]interface{}); ok {
		if enabled, ok := healthChecks["enabled"].(bool); ok {
			config.EnableHealthChecks = enabled
		}
		if interval, ok := healthChecks["interval"].(string); ok {
			if duration, err := time.ParseDuration(interval); err == nil {
				config.HealthCheckInterval = duration
			}
		}
		if timeout, ok := healthChecks["timeout"].(string); ok {
			if duration, err := time.ParseDuration(timeout); err == nil {
				config.HealthCheckTimeout = duration
			}
		}
	}

	// Alert thresholds
	if alertThresholds, ok := configData["alert_thresholds"].(map[string]interface{}); ok {
		config.AlertThresholds = parseAlertThresholds(alertThresholds)
	}

	// Resource monitoring
	if resourceMonitoring, ok := configData["resource_monitoring"].(map[string]interface{}); ok {
		if enabled, ok := resourceMonitoring["enabled"].(bool); ok {
			config.EnableResourceMonitoring = enabled
		}
		if interval, ok := resourceMonitoring["sample_interval"].(string); ok {
			if duration, err := time.ParseDuration(interval); err == nil {
				config.ResourceSampleInterval = duration
			}
		}
	}

	// Audit integration
	if audit, ok := configData["audit"].(map[string]interface{}); ok {
		if integration, ok := audit["integration"].(bool); ok {
			config.AuditIntegration = integration
		}
		if severityLevel, ok := audit["severity_level"].(string); ok {
			config.AuditSeverityLevel = parseAuditSeverityLevel(severityLevel)
		}
	}

	return config, nil
}

// parseAlertThresholds parses alert thresholds from configuration
func parseAlertThresholds(thresholds map[string]interface{}) AlertThresholds {
	result := AlertThresholds{
		ErrorRate:          5.0,
		ErrorRateWindow:    5 * time.Minute,
		P95LatencyMs:       100,
		P99LatencyMs:       500,
		MemoryUsagePercent: 80,
		GCPauseMs:          50,
		ConnectionPoolUtil: 85,
		ConnectionErrors:   10,
		QueueDepth:         1000,
		QueueLatencyMs:     100,
		RateLimitRejects:   100,
		RateLimitUtil:      90,
	}

	if errorRate, ok := thresholds["error_rate"].(float64); ok {
		result.ErrorRate = errorRate
	}
	if errorRateWindow, ok := thresholds["error_rate_window"].(string); ok {
		if duration, err := time.ParseDuration(errorRateWindow); err == nil {
			result.ErrorRateWindow = duration
		}
	}
	if p95Latency, ok := thresholds["p95_latency_ms"].(float64); ok {
		result.P95LatencyMs = p95Latency
	}
	if p99Latency, ok := thresholds["p99_latency_ms"].(float64); ok {
		result.P99LatencyMs = p99Latency
	}
	if memoryUsage, ok := thresholds["memory_usage_percent"].(float64); ok {
		result.MemoryUsagePercent = memoryUsage
	}
	if gcPause, ok := thresholds["gc_pause_ms"].(float64); ok {
		result.GCPauseMs = gcPause
	}
	if connectionPoolUtil, ok := thresholds["connection_pool_util"].(float64); ok {
		result.ConnectionPoolUtil = connectionPoolUtil
	}
	if connectionErrors, ok := thresholds["connection_errors"].(float64); ok {
		result.ConnectionErrors = int64(connectionErrors)
	}
	if queueDepth, ok := thresholds["queue_depth"].(float64); ok {
		result.QueueDepth = int64(queueDepth)
	}
	if queueLatency, ok := thresholds["queue_latency_ms"].(float64); ok {
		result.QueueLatencyMs = queueLatency
	}
	if rateLimitRejects, ok := thresholds["rate_limit_rejects"].(float64); ok {
		result.RateLimitRejects = int64(rateLimitRejects)
	}
	if rateLimitUtil, ok := thresholds["rate_limit_util"].(float64); ok {
		result.RateLimitUtil = rateLimitUtil
	}

	return result
}

// SaveToFile saves configuration to a JSON file
func (c *MonitoringConfig) SaveToFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	configMap := c.ToMap()
	if err := encoder.Encode(configMap); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}

// ToMap converts configuration to a map
func (c *MonitoringConfig) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"prometheus": map[string]interface{}{
			"enabled":   c.EnablePrometheus,
			"namespace": c.PrometheusNamespace,
			"subsystem": c.PrometheusSubsystem,
		},
		"metrics": map[string]interface{}{
			"enabled":  c.MetricsEnabled,
			"interval": c.MetricsInterval.String(),
		},
		"logging": map[string]interface{}{
			"level":      c.LogLevel.String(),
			"format":     c.LogFormat,
			"structured": c.StructuredLogging,
			"sampling":   c.LogSampling,
		},
		"tracing": map[string]interface{}{
			"enabled":      c.EnableTracing,
			"service_name": c.TracingServiceName,
			"provider":     c.TracingProvider,
			"sample_rate":  c.TraceSampleRate,
		},
		"health_checks": map[string]interface{}{
			"enabled":  c.EnableHealthChecks,
			"interval": c.HealthCheckInterval.String(),
			"timeout":  c.HealthCheckTimeout.String(),
		},
		"alert_thresholds": map[string]interface{}{
			"error_rate":           c.AlertThresholds.ErrorRate,
			"error_rate_window":    c.AlertThresholds.ErrorRateWindow.String(),
			"p95_latency_ms":       c.AlertThresholds.P95LatencyMs,
			"p99_latency_ms":       c.AlertThresholds.P99LatencyMs,
			"memory_usage_percent": c.AlertThresholds.MemoryUsagePercent,
			"gc_pause_ms":          c.AlertThresholds.GCPauseMs,
			"connection_pool_util": c.AlertThresholds.ConnectionPoolUtil,
			"connection_errors":    c.AlertThresholds.ConnectionErrors,
			"queue_depth":          c.AlertThresholds.QueueDepth,
			"queue_latency_ms":     c.AlertThresholds.QueueLatencyMs,
			"rate_limit_rejects":   c.AlertThresholds.RateLimitRejects,
			"rate_limit_util":      c.AlertThresholds.RateLimitUtil,
		},
		"resource_monitoring": map[string]interface{}{
			"enabled":         c.EnableResourceMonitoring,
			"sample_interval": c.ResourceSampleInterval.String(),
		},
		"audit": map[string]interface{}{
			"integration":    c.AuditIntegration,
			"severity_level": auditSeverityToString(c.AuditSeverityLevel),
		},
		"profiling": map[string]interface{}{
			"enabled":                 c.EnableProfiling,
			"cpu_profile_interval":    c.CPUProfileInterval.String(),
			"memory_profile_interval": c.MemoryProfileInterval.String(),
		},
	}
}

// Validate validates the configuration
func (c *MonitoringConfig) Validate() error {
	if c.MetricsInterval <= 0 {
		return fmt.Errorf("metrics interval must be positive")
	}

	if c.HealthCheckInterval <= 0 {
		return fmt.Errorf("health check interval must be positive")
	}

	if c.HealthCheckTimeout <= 0 {
		return fmt.Errorf("health check timeout must be positive")
	}

	if c.TraceSampleRate < 0 || c.TraceSampleRate > 1 {
		return fmt.Errorf("trace sample rate must be between 0 and 1")
	}

	if c.AlertThresholds.ErrorRate < 0 || c.AlertThresholds.ErrorRate > 100 {
		return fmt.Errorf("error rate threshold must be between 0 and 100")
	}

	if c.AlertThresholds.MemoryUsagePercent < 0 || c.AlertThresholds.MemoryUsagePercent > 100 {
		return fmt.Errorf("memory usage threshold must be between 0 and 100")
	}

	if c.AlertThresholds.ConnectionPoolUtil < 0 || c.AlertThresholds.ConnectionPoolUtil > 100 {
		return fmt.Errorf("connection pool utilization threshold must be between 0 and 100")
	}

	if c.AlertThresholds.RateLimitUtil < 0 || c.AlertThresholds.RateLimitUtil > 100 {
		return fmt.Errorf("rate limit utilization threshold must be between 0 and 100")
	}

	return nil
}

// Helper functions

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

func parseAuditSeverityLevel(level string) AuditSeverityLevel {
	switch strings.ToLower(level) {
	case "debug":
		return AuditSeverityDebug
	case "info":
		return AuditSeverityInfo
	case "warning", "warn":
		return AuditSeverityWarning
	case "error":
		return AuditSeverityError
	case "critical":
		return AuditSeverityCritical
	default:
		return AuditSeverityInfo
	}
}

// Example configurations

// ExampleBasicConfig returns a basic monitoring configuration
func ExampleBasicConfig() MonitoringConfig {
	return NewMonitoringConfigBuilder().
		WithMetrics(true, 30*time.Second).
		WithLogging(zapcore.InfoLevel, "json", os.Stdout).
		WithHealthChecks(true, 30*time.Second, 5*time.Second).
		WithResourceMonitoring(true, 10*time.Second).
		Build()
}

// ExampleProductionConfig returns a production monitoring configuration
func ExampleProductionConfig() MonitoringConfig {
	return NewMonitoringConfigBuilder().
		WithPrometheus("myapp", "state_manager").
		WithMetrics(true, 15*time.Second).
		WithLogging(zapcore.InfoLevel, "json", os.Stdout).
		WithTracing("myapp-state-manager", "jaeger", 0.01).
		WithHealthChecks(true, 30*time.Second, 5*time.Second).
		WithResourceMonitoring(true, 10*time.Second).
		WithAuditIntegration(true, AuditSeverityInfo).
		WithAlertThresholds(AlertThresholds{
			ErrorRate:          1.0,
			ErrorRateWindow:    5 * time.Minute,
			P95LatencyMs:       50,
			P99LatencyMs:       200,
			MemoryUsagePercent: 70,
			GCPauseMs:          30,
			ConnectionPoolUtil: 80,
			ConnectionErrors:   5,
			QueueDepth:         500,
			QueueLatencyMs:     50,
			RateLimitRejects:   50,
			RateLimitUtil:      85,
		}).
		Build()
}

// ExampleDevelopmentConfig returns a development monitoring configuration
func ExampleDevelopmentConfig() MonitoringConfig {
	return NewMonitoringConfigBuilder().
		WithMetrics(true, 60*time.Second).
		WithLogging(zapcore.DebugLevel, "console", os.Stdout).
		WithTracing("myapp-state-manager-dev", "jaeger", 1.0).
		WithHealthChecks(true, 60*time.Second, 10*time.Second).
		WithResourceMonitoring(true, 30*time.Second).
		WithAuditIntegration(false, AuditSeverityDebug).
		Build()
}
