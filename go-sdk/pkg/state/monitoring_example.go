package state

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.uber.org/zap/zapcore"
)

// ExampleBasicMonitoring demonstrates basic monitoring setup
func ExampleBasicMonitoring() {
	// Create a state manager (this would be your existing StateManager)
	opts := DefaultManagerOptions()
	stateManager, err := NewStateManager(opts)
	if err != nil {
		log.Fatalf("Failed to create state manager: %v", err)
	}
	defer stateManager.Close()
	
	// Set up basic monitoring
	monitoringIntegration, err := SetupBasicMonitoring(stateManager)
	if err != nil {
		log.Fatalf("Failed to setup monitoring: %v", err)
	}
	defer monitoringIntegration.Shutdown(context.Background())
	
	// Record some operations
	start := time.Now()
	
	// Simulate a state operation
	time.Sleep(50 * time.Millisecond)
	monitoringIntegration.RecordCustomMetric("get_state", time.Since(start), nil)
	
	// Record an operation with an error
	start = time.Now()
	time.Sleep(30 * time.Millisecond)
	monitoringIntegration.RecordCustomMetric("update_state", time.Since(start), fmt.Errorf("validation failed"))
	
	// Check health status
	healthStatus := monitoringIntegration.GetMonitoringSystem().GetHealthStatus()
	fmt.Printf("Health status: %+v\n", healthStatus)
	
	// Get metrics summary
	metrics := monitoringIntegration.GetMonitoringSystem().GetMetrics()
	fmt.Printf("Metrics:\n")
	fmt.Printf("  Timestamp: %v\n", metrics.Timestamp)
	fmt.Printf("  Operations: %d\n", len(metrics.Operations))
	fmt.Printf("  Memory Usage: %d bytes\n", metrics.Memory.Usage)
	fmt.Printf("  Goroutines: %d\n", metrics.Memory.Goroutines)
	fmt.Printf("  Connection Pool:\n")
	fmt.Printf("    Total: %d\n", metrics.ConnectionPool.TotalConnections)
	fmt.Printf("    Active: %d\n", metrics.ConnectionPool.ActiveConnections)
	fmt.Printf("    Waiting: %d\n", metrics.ConnectionPool.WaitingConnections)
	fmt.Printf("    Errors: %d\n", metrics.ConnectionPool.ErrorCount)
	fmt.Printf("  Health Checks: %d\n", len(metrics.Health))
}

// ExampleProductionMonitoring demonstrates production monitoring setup
func ExampleProductionMonitoring() {
	// Create a state manager
	opts := DefaultManagerOptions()
	stateManager, err := NewStateManager(opts)
	if err != nil {
		log.Fatalf("Failed to create state manager: %v", err)
	}
	defer stateManager.Close()
	
	// Set up production monitoring with Slack notifications
	slackWebhookURL := os.Getenv("SLACK_WEBHOOK_URL")
	if slackWebhookURL == "" {
		slackWebhookURL = "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK"
	}
	
	monitoringIntegration, err := SetupProductionMonitoring(stateManager, slackWebhookURL)
	if err != nil {
		log.Fatalf("Failed to setup production monitoring: %v", err)
	}
	defer monitoringIntegration.Shutdown(context.Background())
	
	// Start metrics server
	go func() {
		if err := monitoringIntegration.StartMetricsServer(); err != nil {
			log.Printf("Metrics server error: %v", err)
		}
	}()
	
	// Add custom alert notifiers
	fileNotifier, err := NewFileAlertNotifier("/tmp/state_manager_alerts.json")
	if err != nil {
		log.Printf("Failed to create file notifier: %v", err)
	} else {
		monitoringIntegration.AddAlertNotifier(fileNotifier)
	}
	
	// Simulate high-load operations
	for i := 0; i < 100; i++ {
		start := time.Now()
		
		// Simulate varying latencies
		latency := time.Duration(i%5+1) * 20 * time.Millisecond
		time.Sleep(latency)
		
		// Simulate some errors
		var err error
		if i%10 == 0 {
			err = fmt.Errorf("simulated error %d", i)
		}
		
		operation := "bulk_update"
		if i%3 == 0 {
			operation = "get_state"
		}
		
		monitoringIntegration.RecordCustomMetric(operation, time.Since(start), err)
		
		// Sleep between operations
		time.Sleep(100 * time.Millisecond)
	}
	
	// Final health check
	healthStatus := monitoringIntegration.GetMonitoringSystem().GetHealthStatus()
	fmt.Printf("Final health status: %+v\n", healthStatus)
}

// ExampleCustomMonitoring demonstrates custom monitoring configuration
func ExampleCustomMonitoring() {
	// Create custom monitoring configuration
	config := NewMonitoringConfigBuilder().
		WithPrometheus("myapp", "state_manager").
		WithMetrics(true, 10*time.Second).
		WithLogging(zapcore.InfoLevel, "json", os.Stdout).
		WithHealthChecks(true, 15*time.Second, 3*time.Second).
		WithResourceMonitoring(true, 5*time.Second).
		WithAuditIntegration(true, AuditSeverityInfo).
		WithAlertThresholds(AlertThresholds{
			ErrorRate:             2.0,
			ErrorRateWindow:       1 * time.Minute,
			P95LatencyMs:          30,
			P99LatencyMs:          100,
			MemoryUsagePercent:    60,
			GCPauseMs:             20,
			ConnectionPoolUtil:    75,
			ConnectionErrors:      3,
			QueueDepth:            100,
			QueueLatencyMs:        25,
			RateLimitRejects:      20,
			RateLimitUtil:         80,
		}).
		Build()
	
	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("Invalid monitoring configuration: %v", err)
	}
	
	// Create state manager
	opts := DefaultManagerOptions()
	stateManager, err := NewStateManager(opts)
	if err != nil {
		log.Fatalf("Failed to create state manager: %v", err)
	}
	defer stateManager.Close()
	
	// Create monitoring integration
	monitoringIntegration, err := NewMonitoringIntegration(stateManager, config)
	if err != nil {
		log.Fatalf("Failed to create monitoring integration: %v", err)
	}
	defer monitoringIntegration.Shutdown(context.Background())
	
	// Add custom health checks
	monitoringIntegration.RegisterCustomHealthCheck("database", func(ctx context.Context) error {
		// Simulate database health check
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	
	monitoringIntegration.RegisterCustomHealthCheck("external_api", func(ctx context.Context) error {
		// Simulate external API health check
		time.Sleep(50 * time.Millisecond)
		// Simulate occasional failures
		if time.Now().Second()%10 == 0 {
			return fmt.Errorf("external API timeout")
		}
		return nil
	})
	
	// Add multiple alert notifiers
	logNotifier := NewLogAlertNotifier(monitoringIntegration.GetMonitoringSystem().Logger())
	webhookNotifier, err := NewWebhookAlertNotifier("https://example.com/alerts", 5*time.Second)
	if err != nil {
		log.Printf("Failed to create webhook notifier: %v", err)
		return
	}
	
	// Use conditional notifier for different alert levels
	criticalNotifier := NewConditionalAlertNotifier(webhookNotifier, func(alert Alert) bool {
		return alert.Level >= AlertLevelCritical
	})
	
	// Use throttled notifier to prevent spam
	throttledLogNotifier := NewThrottledAlertNotifier(logNotifier, 1*time.Minute)
	
	monitoringIntegration.AddAlertNotifier(throttledLogNotifier)
	monitoringIntegration.AddAlertNotifier(criticalNotifier)
	
	// Simulate operations with custom context
	ctx := context.Background()
	ctx = context.WithValue(ctx, "user_id", "user123")
	ctx = context.WithValue(ctx, "session_id", "session456")
	ctx = context.WithValue(ctx, "ip_address", "192.168.1.100")
	
	for i := 0; i < 50; i++ {
		start := time.Now()
		
		// Simulate operation
		operation := fmt.Sprintf("operation_%d", i%5)
		time.Sleep(time.Duration(i%3+1) * 10 * time.Millisecond)
		
		// Simulate errors for testing
		var err error
		if i%15 == 0 {
			err = fmt.Errorf("operation failed: %d", i)
		}
		
		monitoringIntegration.RecordCustomMetric(operation, time.Since(start), err)
		
		// Log custom event with context
		monitoringIntegration.GetMonitoringSystem().LogAuditEvent(ctx, AuditActionStateUpdate, map[string]interface{}{
			"operation_id": i,
			"operation":    operation,
			"duration_ms":  time.Since(start).Milliseconds(),
		})
		
		time.Sleep(200 * time.Millisecond)
	}
}

// ExampleAlertTesting demonstrates alert testing
func ExampleAlertTesting() {
	// Create basic monitoring setup
	opts := DefaultManagerOptions()
	stateManager, err := NewStateManager(opts)
	if err != nil {
		log.Fatalf("Failed to create state manager: %v", err)
	}
	defer stateManager.Close()
	
	// Create monitoring with very low thresholds for testing
	config := NewMonitoringConfigBuilder().
		WithMetrics(true, 5*time.Second).
		WithAlertThresholds(AlertThresholds{
			ErrorRate:          1.0,  // 1% error rate
			P95LatencyMs:       10,   // 10ms P95 latency
			MemoryUsagePercent: 1,    // 1% memory usage (will always trigger)
			QueueDepth:         5,    // 5 queue depth
		}).
		Build()
	
	monitoringIntegration, err := NewMonitoringIntegration(stateManager, config)
	if err != nil {
		log.Fatalf("Failed to create monitoring integration: %v", err)
	}
	defer monitoringIntegration.Shutdown(context.Background())
	
	// Add log notifier to see alerts
	logNotifier := NewLogAlertNotifier(monitoringIntegration.GetMonitoringSystem().Logger())
	monitoringIntegration.AddAlertNotifier(logNotifier)
	
	fmt.Println("Testing alerts with intentionally low thresholds...")
	
	// Trigger latency alert
	start := time.Now()
	time.Sleep(50 * time.Millisecond) // This should trigger the P95 latency alert
	monitoringIntegration.RecordCustomMetric("slow_operation", time.Since(start), nil)
	
	// Trigger error rate alert
	for i := 0; i < 10; i++ {
		err := fmt.Errorf("test error %d", i)
		monitoringIntegration.RecordCustomMetric("failing_operation", 1*time.Millisecond, err)
	}
	
	// Trigger queue depth alert
	monitoringIntegration.GetMonitoringSystem().RecordQueueDepth(10)
	
	// Wait a bit for alerts to be processed
	time.Sleep(2 * time.Second)
	
	fmt.Println("Alert testing complete. Check logs for alert notifications.")
}

// ExampleConfigurationFromFile demonstrates loading configuration from file
func ExampleConfigurationFromFile() {
	// Create example configuration
	config := ExampleProductionConfig()
	
	// Save to file
	configFile := "/tmp/monitoring_config.json"
	if err := config.SaveToFile(configFile); err != nil {
		log.Fatalf("Failed to save config: %v", err)
	}
	
	fmt.Printf("Configuration saved to %s\n", configFile)
	
	// Load from file
	loader := NewMonitoringConfigLoader()
	loadedConfig, err := loader.LoadFromFile(configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	// Validate loaded configuration
	if err := loadedConfig.Validate(); err != nil {
		log.Fatalf("Loaded config is invalid: %v", err)
	}
	
	fmt.Println("Configuration loaded and validated successfully")
	
	// Use the loaded configuration
	opts := DefaultManagerOptions()
	stateManager, err := NewStateManager(opts)
	if err != nil {
		log.Fatalf("Failed to create state manager: %v", err)
	}
	defer stateManager.Close()
	
	monitoringIntegration, err := NewMonitoringIntegration(stateManager, loadedConfig)
	if err != nil {
		log.Fatalf("Failed to create monitoring integration: %v", err)
	}
	defer monitoringIntegration.Shutdown(context.Background())
	
	fmt.Println("Monitoring system created from loaded configuration")
}

// ExampleConfigurationFromEnvironment demonstrates loading configuration from environment variables
func ExampleConfigurationFromEnvironment() {
	// Set some environment variables for demonstration
	os.Setenv("MONITORING_PROMETHEUS_ENABLED", "true")
	os.Setenv("MONITORING_PROMETHEUS_NAMESPACE", "myapp")
	os.Setenv("MONITORING_METRICS_INTERVAL", "20s")
	os.Setenv("MONITORING_LOG_LEVEL", "debug")
	os.Setenv("MONITORING_ALERT_ERROR_RATE", "2.5")
	os.Setenv("MONITORING_ALERT_P95_LATENCY_MS", "75")
	
	// Load configuration from environment
	loader := NewMonitoringConfigLoader()
	config := loader.LoadFromEnv()
	
	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("Environment config is invalid: %v", err)
	}
	
	fmt.Printf("Configuration loaded from environment:\n")
	fmt.Printf("  Prometheus enabled: %v\n", config.EnablePrometheus)
	fmt.Printf("  Prometheus namespace: %s\n", config.PrometheusNamespace)
	fmt.Printf("  Metrics interval: %v\n", config.MetricsInterval)
	fmt.Printf("  Log level: %v\n", config.LogLevel)
	fmt.Printf("  Error rate threshold: %.1f%%\n", config.AlertThresholds.ErrorRate)
	fmt.Printf("  P95 latency threshold: %.0fms\n", config.AlertThresholds.P95LatencyMs)
	
	// Use the configuration
	opts := DefaultManagerOptions()
	stateManager, err := NewStateManager(opts)
	if err != nil {
		log.Fatalf("Failed to create state manager: %v", err)
	}
	defer stateManager.Close()
	
	monitoringIntegration, err := NewMonitoringIntegration(stateManager, config)
	if err != nil {
		log.Fatalf("Failed to create monitoring integration: %v", err)
	}
	defer monitoringIntegration.Shutdown(context.Background())
	
	fmt.Println("Monitoring system created from environment configuration")
}

// RunAllExamples runs all monitoring examples
func RunAllExamples() {
	fmt.Println("=== Running Basic Monitoring Example ===")
	ExampleBasicMonitoring()
	
	fmt.Println("\n=== Running Custom Monitoring Example ===")
	ExampleCustomMonitoring()
	
	fmt.Println("\n=== Running Alert Testing Example ===")
	ExampleAlertTesting()
	
	fmt.Println("\n=== Running Configuration from File Example ===")
	ExampleConfigurationFromFile()
	
	fmt.Println("\n=== Running Configuration from Environment Example ===")
	ExampleConfigurationFromEnvironment()
	
	fmt.Println("\n=== All examples completed ===")
}