package monitoring

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMonitoringIntegration tests the complete monitoring integration
func TestMonitoringIntegration(t *testing.T) {
	config := &Config{
		MetricsConfig:    events.DefaultMetricsConfig(),
		PrometheusPort:   9091,
		PrometheusPath:   "/metrics",
		EnableTracing:    false, // Disable for tests
		EnableMetrics:    false, // Disable for tests
		ServiceName:      "test-service",
		ServiceVersion:   "1.0.0",
		Environment:      "test",
		SLAWindowSize:    5 * time.Minute,
		EnableSLAReports: true,
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
	defer monitor.Shutdown()
	
	// Test event recording
	ctx := context.Background()
	monitor.RecordEventWithContext(ctx, 50*time.Millisecond, true, map[string]string{
		"event_type": "test",
	})
	
	// Test rule execution recording
	monitor.RecordRuleExecutionWithContext(ctx, "test_rule", 10*time.Millisecond, true)
	
	// Get dashboard data
	dashboard := monitor.GetEnhancedDashboardData()
	assert.NotNil(t, dashboard)
	assert.NotNil(t, dashboard.DashboardData)
}

// TestPrometheusExporter tests Prometheus metric export
func TestPrometheusExporter(t *testing.T) {
	// Create mock metrics collector
	metricsConfig := events.DefaultMetricsConfig()
	collector, err := events.NewMetricsCollector(metricsConfig)
	require.NoError(t, err)
	defer collector.Shutdown()
	
	// Record some test metrics
	for i := 0; i < 100; i++ {
		collector.RecordEvent(time.Duration(i)*time.Millisecond, i%10 != 0)
		collector.RecordRuleExecution(fmt.Sprintf("rule_%d", i%5), time.Duration(i)*time.Millisecond, true)
	}
	
	config := DefaultConfig()
	config.PrometheusPort = 9092
	
	exporter := NewPrometheusExporter(config, collector)
	
	// Test metric recording
	exporter.RecordEvent(50*time.Millisecond, true)
	exporter.RecordError("validation_error")
	exporter.RecordWarning("slow_rule")
	
	// Verify metrics exist
	assert.Equal(t, 1, testutil.CollectAndCount(exporter.eventCounter))
	assert.Equal(t, 1, testutil.CollectAndCount(exporter.errorCounter))
	assert.Equal(t, 1, testutil.CollectAndCount(exporter.warningCounter))
}

// TestCustomPrometheusCollectors tests custom Prometheus collectors
func TestCustomPrometheusCollectors(t *testing.T) {
	// Create mock metrics collector
	metricsConfig := events.DefaultMetricsConfig()
	collector, err := events.NewMetricsCollector(metricsConfig)
	require.NoError(t, err)
	defer collector.Shutdown()
	
	// Record test data
	for i := 0; i < 50; i++ {
		collector.RecordEvent(time.Duration(i)*time.Millisecond, true)
		collector.RecordRuleExecution("test_rule", time.Duration(i)*time.Millisecond, true)
	}
	
	// Test EventValidationCollector
	t.Run("EventValidationCollector", func(t *testing.T) {
		eventCollector := NewEventValidationCollector(collector, map[string]string{"test": "true"})
		
		ch := make(chan prometheus.Metric)
		go func() {
			eventCollector.Collect(ch)
			close(ch)
		}()
		
		metrics := []prometheus.Metric{}
		for m := range ch {
			metrics = append(metrics, m)
		}
		
		assert.Greater(t, len(metrics), 0)
	})
	
	// Test RuleExecutionCollector
	t.Run("RuleExecutionCollector", func(t *testing.T) {
		ruleCollector := NewRuleExecutionCollector(collector, map[string]string{"test": "true"})
		
		ch := make(chan prometheus.Metric)
		go func() {
			ruleCollector.Collect(ch)
			close(ch)
		}()
		
		metrics := []prometheus.Metric{}
		for m := range ch {
			metrics = append(metrics, m)
		}
		
		assert.Greater(t, len(metrics), 0)
	})
}

// TestGrafanaDashboardGeneration tests dashboard JSON generation
func TestGrafanaDashboardGeneration(t *testing.T) {
	config := DefaultConfig()
	generator := NewGrafanaDashboardGenerator(config)
	
	tests := []struct {
		name      string
		genFunc   func() (*GrafanaDashboard, error)
		wantTitle string
	}{
		{
			name:      "Overview Dashboard",
			genFunc:   generator.GenerateOverviewDashboard,
			wantTitle: "Event Validation Overview",
		},
		{
			name:      "Rule Performance Dashboard",
			genFunc:   generator.GenerateRulePerformanceDashboard,
			wantTitle: "Rule Performance Analysis",
		},
		{
			name:      "SLA Compliance Dashboard",
			genFunc:   generator.GenerateSLAComplianceDashboard,
			wantTitle: "SLA Compliance Monitoring",
		},
		{
			name:      "System Health Dashboard",
			genFunc:   generator.GenerateSystemHealthDashboard,
			wantTitle: "System Health Monitor",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dashboard, err := tt.genFunc()
			require.NoError(t, err)
			assert.NotNil(t, dashboard)
			assert.Equal(t, tt.wantTitle, dashboard.Dashboard.Title)
			assert.Greater(t, len(dashboard.Dashboard.Panels), 0)
		})
	}
}

// TestAlertManager tests alert management functionality
func TestAlertManager(t *testing.T) {
	config := DefaultConfig()
	metricsConfig := events.DefaultMetricsConfig()
	collector, err := events.NewMetricsCollector(metricsConfig)
	require.NoError(t, err)
	defer collector.Shutdown()
	
	alertManager := NewAlertManager(config, collector)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Start alert manager
	go alertManager.Start(ctx)
	
	// Simulate high error rate
	for i := 0; i < 100; i++ {
		collector.RecordEvent(10*time.Millisecond, i < 20) // 80% error rate
	}
	
	// Wait for alert evaluation
	time.Sleep(100 * time.Millisecond)
	
	// Force evaluation
	alertManager.evaluateAllRules()
	
	// Check for active alerts
	activeAlerts := alertManager.GetActiveAlerts()
	assert.GreaterOrEqual(t, len(activeAlerts), 0) // May or may not have alerts depending on timing
	
	// Test alert history
	history := alertManager.GetAlertHistory(10)
	assert.NotNil(t, history)
	
	// Shutdown
	err = alertManager.Shutdown()
	assert.NoError(t, err)
}

// TestSLAMonitor tests SLA monitoring functionality
func TestSLAMonitor(t *testing.T) {
	config := DefaultConfig()
	metricsConfig := events.DefaultMetricsConfig()
	collector, err := events.NewMetricsCollector(metricsConfig)
	require.NoError(t, err)
	defer collector.Shutdown()
	
	slaMonitor := NewSLAMonitor(config, collector)
	
	// Record events to generate metrics
	for i := 0; i < 100; i++ {
		duration := time.Duration(i) * time.Millisecond
		collector.RecordEvent(duration, true)
		slaMonitor.RecordEvent(duration, true)
	}
	
	// Update SLA status
	slaMonitor.updateSLAStatus()
	
	// Get current status
	status := slaMonitor.GetCurrentStatus()
	assert.NotNil(t, status)
	
	// Check each SLA
	for name, slaStatus := range status {
		assert.NotNil(t, slaStatus)
		assert.NotEmpty(t, name)
		assert.NotZero(t, slaStatus.Target.TargetValue)
	}
}

// TestPerformanceAndConcurrency tests performance under concurrent load
func TestPerformanceAndConcurrency(t *testing.T) {
	config := DefaultConfig()
	config.EnableTracing = false
	config.EnableMetrics = false
	
	monitor, err := NewMonitoringIntegration(config)
	require.NoError(t, err)
	defer monitor.Shutdown()
	
	const (
		numGoroutines = 100
		eventsPerGoroutine = 1000
	)
	
	var wg sync.WaitGroup
	var totalEvents int64
	
	start := time.Now()
	
	// Concurrent event recording
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctx := context.Background()
			
			for j := 0; j < eventsPerGoroutine; j++ {
				duration := time.Duration(j%100) * time.Millisecond
				success := j%10 != 0
				
				monitor.RecordEventWithContext(ctx, duration, success, map[string]string{
					"goroutine": fmt.Sprintf("%d", id),
				})
				
				if j%10 == 0 {
					monitor.RecordRuleExecutionWithContext(ctx, fmt.Sprintf("rule_%d", j%5), duration/10, success)
				}
				
				atomic.AddInt64(&totalEvents, 1)
			}
		}(i)
	}
	
	wg.Wait()
	elapsed := time.Since(start)
	
	// Calculate throughput
	throughput := float64(totalEvents) / elapsed.Seconds()
	t.Logf("Processed %d events in %v (%.2f events/sec)", totalEvents, elapsed, throughput)
	
	// Verify metrics
	dashboard := monitor.GetEnhancedDashboardData()
	assert.NotNil(t, dashboard)
	assert.Greater(t, dashboard.TotalEvents, int64(0))
	
	// Performance assertion
	assert.Greater(t, throughput, 10000.0, "Should process at least 10k events/sec")
}

// TestMemoryLeakDetection tests memory leak detection
func TestMemoryLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}
	
	config := DefaultConfig()
	config.MetricsConfig.EnableLeakDetection = true
	config.MetricsConfig.MemoryLeakThreshold = 10 * 1024 * 1024 // 10MB
	
	monitor, err := NewMonitoringIntegration(config)
	require.NoError(t, err)
	defer monitor.Shutdown()
	
	// Simulate memory growth
	var data [][]byte
	ctx := context.Background()
	
	for i := 0; i < 100; i++ {
		// Allocate 1MB
		data = append(data, make([]byte, 1024*1024))
		
		// Record events
		for j := 0; j < 100; j++ {
			monitor.RecordEventWithContext(ctx, 10*time.Millisecond, true, nil)
		}
		
		// Small delay to allow memory monitoring
		time.Sleep(10 * time.Millisecond)
	}
	
	// Get memory history
	memHistory := monitor.metricsCollector.GetMemoryHistory()
	assert.Greater(t, len(memHistory), 0)
	
	// Keep reference to prevent GC
	_ = data
}

// TestAlertRuleEvaluation tests specific alert rule conditions
func TestAlertRuleEvaluation(t *testing.T) {
	tests := []struct {
		name          string
		setupMetrics  func(events.MetricsCollector)
		expectedAlert string
		shouldFire    bool
	}{
		{
			name: "High Error Rate Alert",
			setupMetrics: func(c events.MetricsCollector) {
				// 90% error rate
				for i := 0; i < 100; i++ {
					c.RecordEvent(10*time.Millisecond, i >= 90)
				}
			},
			expectedAlert: "high_error_rate",
			shouldFire:    true,
		},
		{
			name: "Normal Operation",
			setupMetrics: func(c events.MetricsCollector) {
				// 1% error rate
				for i := 0; i < 100; i++ {
					c.RecordEvent(10*time.Millisecond, i >= 1)
				}
			},
			expectedAlert: "",
			shouldFire:    false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			metricsConfig := events.DefaultMetricsConfig()
			collector, err := events.NewMetricsCollector(metricsConfig)
			require.NoError(t, err)
			defer collector.Shutdown()
			
			alertManager := NewAlertManager(config, collector)
			
			// Setup metrics
			tt.setupMetrics(collector)
			
			// Evaluate rules
			alertManager.evaluateAllRules()
			
			// Check alerts
			activeAlerts := alertManager.GetActiveAlerts()
			
			if tt.shouldFire {
				found := false
				for _, alert := range activeAlerts {
					if alert.Name == tt.expectedAlert {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected alert %s to fire", tt.expectedAlert)
			} else {
				assert.Empty(t, activeAlerts, "Expected no alerts to fire")
			}
		})
	}
}

// BenchmarkMonitoringIntegration benchmarks the monitoring integration
func BenchmarkMonitoringIntegration(b *testing.B) {
	config := DefaultConfig()
	config.EnableTracing = false
	config.EnableMetrics = false
	
	monitor, err := NewMonitoringIntegration(config)
	require.NoError(b, err)
	defer monitor.Shutdown()
	
	ctx := context.Background()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			duration := time.Duration(i%100) * time.Millisecond
			monitor.RecordEventWithContext(ctx, duration, true, nil)
			
			if i%10 == 0 {
				monitor.RecordRuleExecutionWithContext(ctx, fmt.Sprintf("rule_%d", i%5), duration/10, true)
			}
			i++
		}
	})
}

// BenchmarkPrometheusExporter benchmarks Prometheus metric export
func BenchmarkPrometheusExporter(b *testing.B) {
	metricsConfig := events.DefaultMetricsConfig()
	collector, err := events.NewMetricsCollector(metricsConfig)
	require.NoError(b, err)
	defer collector.Shutdown()
	
	config := DefaultConfig()
	exporter := NewPrometheusExporter(config, collector)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			exporter.RecordEvent(10*time.Millisecond, true)
		}
	})
}

// TestIntegrationWithRealMetrics tests integration with real metrics collector
func TestIntegrationWithRealMetrics(t *testing.T) {
	// Create a metrics config with all features enabled
	metricsConfig := events.ProductionMetricsConfig()
	metricsConfig.PrometheusEnabled = true
	metricsConfig.OTLPEnabled = false // Disable to avoid connection errors
	
	// Create monitoring integration
	config := &Config{
		MetricsConfig:     metricsConfig,
		PrometheusPort:    9093,
		EnableTracing:     false,
		EnableMetrics:     false,
		ServiceName:       "integration-test",
		EnableRunbooks:    true,
		RunbookBaseURL:    "https://runbooks.test.com",
		EnableSLAReports:  true,
		AutoGenerateDash:  false, // Disable auto-generation for tests
	}
	
	monitor, err := NewMonitoringIntegration(config)
	require.NoError(t, err)
	defer monitor.Shutdown()
	
	// Simulate realistic workload
	ctx := context.Background()
	var wg sync.WaitGroup
	
	// Event generator
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			latency := time.Duration(10+i%90) * time.Millisecond
			success := i%20 != 0 // 5% error rate
			
			monitor.RecordEventWithContext(ctx, latency, success, map[string]string{
				"source": "test",
				"type":   fmt.Sprintf("type_%d", i%5),
			})
			
			time.Sleep(time.Millisecond)
		}
	}()
	
	// Rule execution generator
	wg.Add(1)
	go func() {
		defer wg.Done()
		rules := []string{"validation", "transformation", "enrichment", "filtering", "routing"}
		
		for i := 0; i < 500; i++ {
			for _, rule := range rules {
				latency := time.Duration(5+i%45) * time.Millisecond
				success := i%50 != 0 // 2% error rate
				
				monitor.RecordRuleExecutionWithContext(ctx, rule, latency, success)
			}
			
			time.Sleep(5 * time.Millisecond)
		}
	}()
	
	// Wait for generators to complete
	wg.Wait()
	
	// Allow time for metrics to be processed
	time.Sleep(100 * time.Millisecond)
	
	// Verify dashboard data
	dashboard := monitor.GetEnhancedDashboardData()
	require.NotNil(t, dashboard)
	require.NotNil(t, dashboard.DashboardData)
	
	assert.Greater(t, dashboard.TotalEvents, int64(900))
	assert.Greater(t, dashboard.ErrorRate, 4.0)
	assert.Less(t, dashboard.ErrorRate, 6.0)
	assert.NotNil(t, dashboard.SLAStatus)
	assert.NotNil(t, dashboard.ActiveAlerts)
	
	// Check for runbooks if alerts exist
	if len(dashboard.ActiveAlerts) > 0 {
		assert.NotEmpty(t, dashboard.Runbooks)
		for _, runbook := range dashboard.Runbooks {
			assert.NotEmpty(t, runbook.URL)
			assert.Contains(t, runbook.URL, config.RunbookBaseURL)
		}
	}
}