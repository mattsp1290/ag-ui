package state

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestMonitoringSystemBasic(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.LogLevel = zapcore.DebugLevel
	config.MetricsInterval = 1 * time.Second
	config.HealthCheckInterval = 1 * time.Second
	
	monitoringSystem, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer monitoringSystem.Shutdown(context.Background())
	
	// Test basic functionality
	if monitoringSystem.Logger() == nil {
		t.Error("Logger should not be nil")
	}
	
	// Test recording metrics
	monitoringSystem.RecordStateOperation("test_operation", 50*time.Millisecond, nil)
	monitoringSystem.RecordStateOperation("test_operation_error", 30*time.Millisecond, ErrStateNotFound)
	
	// Test health checks
	monitoringSystem.RegisterHealthCheck(NewCustomHealthCheck("test", func(ctx context.Context) error {
		return nil
	}))
	
	healthStatus := monitoringSystem.GetHealthStatus()
	if len(healthStatus) == 0 {
		t.Error("Should have at least one health check")
	}
	
	// Test metrics
	metrics := monitoringSystem.GetMetrics()
	if metrics.Timestamp.IsZero() {
		t.Error("Metrics timestamp should be set")
	}
}

func TestMonitoringConfigBuilder(t *testing.T) {
	config := NewMonitoringConfigBuilder().
		WithPrometheus("test", "monitor").
		WithMetrics(true, 10*time.Second).
		WithLogging(zapcore.InfoLevel, "json", nil).
		WithHealthChecks(true, 30*time.Second, 5*time.Second).
		Build()
	
	if !config.EnablePrometheus {
		t.Error("Prometheus should be enabled")
	}
	
	if config.PrometheusNamespace != "test" {
		t.Error("Prometheus namespace should be 'test'")
	}
	
	if config.MetricsInterval != 10*time.Second {
		t.Error("Metrics interval should be 10 seconds")
	}
	
	if err := config.Validate(); err != nil {
		t.Errorf("Config should be valid: %v", err)
	}
}

func TestAlertNotifiers(t *testing.T) {
	// Test log notifier
	// Create a simple zap logger for testing
	zapLogger, _ := zap.NewDevelopment()
	logNotifier := NewLogAlertNotifier(zapLogger)
	
	alert := Alert{
		Level:       AlertLevelWarning,
		Title:       "Test Alert",
		Description: "This is a test alert",
		Timestamp:   time.Now(),
		Component:   "test",
		Value:       100.0,
		Threshold:   80.0,
	}
	
	if err := logNotifier.SendAlert(context.Background(), alert); err != nil {
		t.Errorf("Failed to send log alert: %v", err)
	}
	
	// Test composite notifier
	compositeNotifier := NewCompositeAlertNotifier(logNotifier, logNotifier)
	if err := compositeNotifier.SendAlert(context.Background(), alert); err != nil {
		t.Errorf("Failed to send composite alert: %v", err)
	}
	
	// Test conditional notifier
	conditionalNotifier := NewConditionalAlertNotifier(logNotifier, func(a Alert) bool {
		return a.Level >= AlertLevelError
	})
	
	// Should not send (warning < error)
	if err := conditionalNotifier.SendAlert(context.Background(), alert); err != nil {
		t.Errorf("Conditional notifier failed: %v", err)
	}
	
	// Should send (error >= error)
	alert.Level = AlertLevelError
	if err := conditionalNotifier.SendAlert(context.Background(), alert); err != nil {
		t.Errorf("Conditional notifier failed: %v", err)
	}
}

func TestHealthChecks(t *testing.T) {
	// Test memory health check
	memoryCheck := NewMemoryHealthCheck(1024, 100, 10000) // 1GB, 100ms GC, 10k goroutines
	if err := memoryCheck.Check(context.Background()); err != nil {
		t.Errorf("Memory health check failed: %v", err)
	}
	
	// Test custom health check
	customCheck := NewCustomHealthCheck("test", func(ctx context.Context) error {
		return nil
	})
	if err := customCheck.Check(context.Background()); err != nil {
		t.Errorf("Custom health check failed: %v", err)
	}
	
	// Test composite health check
	compositeCheck := NewCompositeHealthCheck("test_composite", false, memoryCheck, customCheck)
	if err := compositeCheck.Check(context.Background()); err != nil {
		t.Errorf("Composite health check failed: %v", err)
	}
}

func TestConfigurationValidation(t *testing.T) {
	config := DefaultMonitoringConfig()
	
	if err := config.Validate(); err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}
	
	// Test invalid configurations
	invalidConfigs := []struct {
		name     string
		modifier func(*MonitoringConfig)
	}{
		{
			name: "negative metrics interval",
			modifier: func(c *MonitoringConfig) {
				c.MetricsInterval = -1 * time.Second
			},
		},
		{
			name: "invalid trace sample rate",
			modifier: func(c *MonitoringConfig) {
				c.TraceSampleRate = 1.5
			},
		},
		{
			name: "invalid error rate threshold",
			modifier: func(c *MonitoringConfig) {
				c.AlertThresholds.ErrorRate = 150.0
			},
		},
	}
	
	for _, tc := range invalidConfigs {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultMonitoringConfig()
			tc.modifier(&config)
			
			if err := config.Validate(); err == nil {
				t.Errorf("Config should be invalid for %s", tc.name)
			}
		})
	}
}

func TestMetricsRecording(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.MetricsInterval = 100 * time.Millisecond
	
	monitoringSystem, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer monitoringSystem.Shutdown(context.Background())
	
	// Record various operations
	operations := []struct {
		name     string
		duration time.Duration
		err      error
	}{
		{"get_state", 10 * time.Millisecond, nil},
		{"update_state", 25 * time.Millisecond, nil},
		{"delete_state", 15 * time.Millisecond, ErrStateNotFound},
		{"create_state", 30 * time.Millisecond, nil},
	}
	
	for _, op := range operations {
		monitoringSystem.RecordStateOperation(op.name, op.duration, op.err)
	}
	
	// Record event processing
	monitoringSystem.RecordEventProcessing("state_changed", 5*time.Millisecond, nil)
	monitoringSystem.RecordEventProcessing("validation_failed", 2*time.Millisecond, errors.New("validation failed"))
	
	// Record memory usage
	monitoringSystem.RecordMemoryUsage(1024*1024*100, 1000, 10*time.Millisecond) // 100MB, 1000 allocs, 10ms GC
	
	// Record connection pool stats
	monitoringSystem.RecordConnectionPoolStats(10, 5, 2, 1) // 10 total, 5 active, 2 waiting, 1 error
	
	// Record rate limiting
	monitoringSystem.RecordRateLimitStats(100, 5, 85.0) // 100 requests, 5 rejects, 85% util
	
	// Record queue depth
	monitoringSystem.RecordQueueDepth(50)
	
	// Get metrics and verify
	metrics := monitoringSystem.GetMetrics()
	if len(metrics.Operations) == 0 {
		t.Error("Should have recorded operation metrics")
	}
	
	if metrics.Memory.Usage == 0 {
		t.Error("Should have recorded memory metrics")
	}
	
	if metrics.ConnectionPool.TotalConnections == 0 {
		t.Error("Should have recorded connection pool metrics")
	}
}

func BenchmarkMetricsRecording(b *testing.B) {
	config := DefaultMonitoringConfig()
	config.EnableResourceMonitoring = false // Disable to reduce overhead
	
	monitoringSystem, err := NewMonitoringSystem(config)
	if err != nil {
		b.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer monitoringSystem.Shutdown(context.Background())
	
	b.ResetTimer()
	
	b.Run("RecordStateOperation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			monitoringSystem.RecordStateOperation("benchmark_op", 1*time.Millisecond, nil)
		}
	})
	
	b.Run("RecordEventProcessing", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			monitoringSystem.RecordEventProcessing("benchmark_event", 1*time.Millisecond, nil)
		}
	})
}