package monitoring

import (
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSimpleMonitoringCreation tests basic monitoring integration creation
func TestSimpleMonitoringCreation(t *testing.T) {
	config := &Config{
		MetricsConfig:    events.DefaultMetricsConfig(),
		PrometheusPort:   0, // Disable HTTP server
		PrometheusPath:   "/metrics",
		EnableTracing:    false,
		EnableMetrics:    false,
		ServiceName:      "test-service",
		ServiceVersion:   "1.0.0",
		Environment:      "test",
		SLAWindowSize:    5 * time.Minute,
		EnableSLAReports: false,
		AlertThresholds: AlertThresholds{
			ErrorRatePercent:     5.0,
			LatencyP99Millis:     100.0,
			MemoryUsagePercent:   80.0,
			ThroughputMinEvents:  10.0,
			SLAViolationPercent:  10.0,
		},
	}

	monitor, err := NewMonitoringIntegration(config)
	require.NoError(t, err)
	require.NotNil(t, monitor)
	defer monitor.Shutdown()

	// Verify basic components are initialized
	assert.NotNil(t, monitor.metricsCollector)
	assert.NotNil(t, monitor.prometheusExporter)
	assert.NotNil(t, monitor.alertManager)
}

// TestSimplePrometheusExporter tests basic Prometheus exporter creation
func TestSimplePrometheusExporter(t *testing.T) {
	metricsConfig := events.DefaultMetricsConfig()
	collector, err := events.NewMetricsCollector(metricsConfig)
	require.NoError(t, err)
	defer collector.Shutdown()

	config := DefaultConfig()
	config.PrometheusPort = 0 // Disable HTTP server

	exporter := NewPrometheusExporter(config, collector)
	assert.NotNil(t, exporter)

	// Record some basic metrics
	exporter.RecordEvent(50*time.Millisecond, true)
	exporter.RecordError("test_error")
	exporter.RecordWarning("test_warning")
}

// TestSimpleSLAMonitor tests basic SLA monitor creation
func TestSimpleSLAMonitor(t *testing.T) {
	config := DefaultConfig()
	metricsConfig := events.DefaultMetricsConfig()
	collector, err := events.NewMetricsCollector(metricsConfig)
	require.NoError(t, err)
	defer collector.Shutdown()

	slaMonitor := NewSLAMonitor(config, collector)
	assert.NotNil(t, slaMonitor)

	// Record a few events
	for i := 0; i < 10; i++ {
		slaMonitor.RecordEvent(10*time.Millisecond, true)
	}

	// Get status
	status := slaMonitor.GetCurrentStatus()
	assert.NotNil(t, status)
}

// TestSimpleAlertManager tests basic alert manager creation
func TestSimpleAlertManager(t *testing.T) {
	config := DefaultConfig()
	metricsConfig := events.DefaultMetricsConfig()
	collector, err := events.NewMetricsCollector(metricsConfig)
	require.NoError(t, err)
	defer collector.Shutdown()

	alertManager := NewAlertManager(config, collector)
	assert.NotNil(t, alertManager)

	// Get active alerts (should be empty initially)
	alerts := alertManager.GetActiveAlerts()
	assert.NotNil(t, alerts)
}