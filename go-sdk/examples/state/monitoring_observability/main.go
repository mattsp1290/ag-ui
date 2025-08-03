// Package main demonstrates comprehensive monitoring and observability features
// for production state management systems including metrics, logging, tracing, and alerts.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/state"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap/zapcore"
)

// DemoApplication represents a monitored application
type DemoApplication struct {
	store          *state.StateStore
	monitor        *state.StateMonitor
	performanceOpt state.PerformanceOptimizer
	alertManager   *AlertManager
	healthChecker  *state.HealthChecker
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// AlertManager handles alert notifications
type AlertManager struct {
	alerts      chan state.Alert
	notifiers   []state.AlertNotifier
	mu          sync.RWMutex
	alertCounts map[string]int
}

func main() {
	// Create monitoring configuration
	monitoringConfig := createMonitoringConfig()

	// Initialize demo application
	app := initializeDemoApplication(monitoringConfig)
	defer app.Shutdown()

	// Start monitoring endpoints
	go startMetricsServer()
	go startHealthCheckServer(app.healthChecker)

	fmt.Println("=== State Management Monitoring & Observability Demo ===")

	// Run demonstrations
	demonstrateMetricsCollection(app)
	demonstrateLoggingAndTracing(app)
	demonstrateHealthChecks(app)
	demonstrateAlerting(app)
	demonstratePerformanceMonitoring(app)
	demonstrateDashboardMetrics(app)

	// Show final summary
	showMonitoringSummary(app)
}

func createMonitoringConfig() *state.MonitoringConfig {
	return &state.MonitoringConfig{
		// Metrics configuration
		EnablePrometheus:    true,
		PrometheusNamespace: "agui",
		PrometheusSubsystem: "state",
		MetricsEnabled:      true,
		MetricsInterval:     5 * time.Second,

		// Logging configuration
		LogLevel:          zapcore.InfoLevel,
		LogOutput:         os.Stdout,
		LogFormat:         "json",
		StructuredLogging: true,
		LogSampling:       true,

		// Tracing configuration
		EnableTracing:      true,
		TracingServiceName: "agui-state-demo",
		TracingProvider:    "jaeger",
		TraceSampleRate:    1.0, // 100% for demo

		// Health check configuration
		EnableHealthChecks:  true,
		HealthCheckInterval: 10 * time.Second,
		HealthCheckTimeout:  5 * time.Second,

		// Alert configuration
		AlertThresholds: state.AlertThresholds{
			ErrorRate:          5.0, // 5% error rate
			ErrorRateWindow:    1 * time.Minute,
			P95LatencyMs:       100,
			P99LatencyMs:       500,
			MemoryUsagePercent: 80,
			GCPauseMs:          100,
			ConnectionPoolUtil: 90,
			ConnectionErrors:   10,
			QueueDepth:         1000,
			QueueLatencyMs:     50,
			RateLimitRejects:   100,
			RateLimitUtil:      80,
		},

		// Performance monitoring
		EnableProfiling:       true,
		CPUProfileInterval:    30 * time.Second,
		MemoryProfileInterval: 30 * time.Second,

		// Resource monitoring
		EnableResourceMonitoring: true,
		ResourceSampleInterval:   5 * time.Second,

		// Audit integration
		AuditIntegration:   true,
		AuditSeverityLevel: state.AuditSeverityInfo,
	}
}

func initializeDemoApplication(config *state.MonitoringConfig) *DemoApplication {
	ctx, cancel := context.WithCancel(context.Background())

	// Create logger
	logger := state.NewStructuredLogger(config)

	// Create state store with monitoring
	store := state.NewStateStore(
		state.WithMaxHistory(100),
		state.WithLogger(logger),
		state.WithMetrics(true),
		state.WithAuditLog(true),
	)

	// Create monitor
	monitor := state.NewStateMonitor(store, config)

	// Create performance optimizer
	perfOptions := state.PerformanceOptions{
		EnablePooling:     true,
		EnableBatching:    true,
		EnableCompression: true,
		EnableLazyLoading: true,
		EnableSharding:    true,
		BatchSize:         100,
		BatchTimeout:      10 * time.Millisecond,
		MaxConcurrency:    runtime.NumCPU() * 2,
		MaxOpsPerSecond:   10000,
		ShardCount:        16,
	}
	performanceOpt := state.NewPerformanceOptimizer(perfOptions)

	// Create alert manager
	webhookURL := os.Getenv("ALERT_WEBHOOK_URL")
	if webhookURL == "" {
		webhookURL = "http://localhost:8080/alerts" // Default for local development
	}

	emailAddress := os.Getenv("ALERT_EMAIL_ADDRESS")
	if emailAddress == "" {
		emailAddress = "alerts@example.com" // Default for local development
	}

	alertManager := &AlertManager{
		alerts:      make(chan state.Alert, 100),
		alertCounts: make(map[string]int),
		notifiers: []state.AlertNotifier{
			state.NewConsoleNotifier(),
		},
	}

	// Add webhook notifier with error handling
	if webhookNotifier, err := state.NewWebhookNotifier(webhookURL); err == nil {
		alertManager.notifiers = append(alertManager.notifiers, webhookNotifier)
	} else {
		log.Printf("Warning: Failed to create webhook notifier: %v", err)
	}

	// Add email notifier with SMTP settings
	// For demo purposes, using dummy SMTP settings - replace with real values in production
	smtpServer := os.Getenv("SMTP_SERVER")
	if smtpServer == "" {
		smtpServer = "smtp.gmail.com" // Default for demo
	}

	smtpPort := 587 // Default SMTP port
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	fromEmail := os.Getenv("SMTP_FROM")
	if fromEmail == "" {
		fromEmail = emailAddress // Use the alert email as from address for demo
	}

	// Only add email notifier if we have required SMTP credentials
	if username != "" && password != "" {
		emailNotifier := state.NewEmailNotifier(smtpServer, smtpPort, username, password, fromEmail, []string{emailAddress})
		alertManager.notifiers = append(alertManager.notifiers, emailNotifier)
	} else {
		log.Println("Warning: SMTP credentials not provided, skipping email notifier")
	}

	// Set alert notifiers
	config.AlertNotifiers = alertManager.notifiers

	// Create health checker with a StateManager
	managerOpts := state.ManagerOptions{
		CustomStore:    store,
		MaxHistorySize: 100,
		EnableCaching:  true,
	}
	manager, err := state.NewStateManager(managerOpts)
	if err != nil {
		log.Fatalf("Failed to create state manager: %v", err)
	}
	healthChecker := state.NewHealthChecker(manager)

	// Initialize demo data
	initializeDemoData(store)

	app := &DemoApplication{
		store:          store,
		monitor:        monitor,
		performanceOpt: performanceOpt,
		alertManager:   alertManager,
		healthChecker:  healthChecker,
		ctx:            ctx,
		cancel:         cancel,
	}

	// Start monitoring
	monitor.Start()

	// Start alert processor
	app.wg.Add(1)
	go app.processAlerts()

	return app
}

func (app *DemoApplication) Shutdown() {
	app.cancel()
	app.wg.Wait()
	app.monitor.Stop()
}

func (app *DemoApplication) processAlerts() {
	defer app.wg.Done()

	for {
		select {
		case alert := <-app.alertManager.alerts:
			app.alertManager.mu.Lock()
			app.alertManager.alertCounts[alert.Name]++
			app.alertManager.mu.Unlock()

			// Process alert
			for _, notifier := range app.alertManager.notifiers {
				if err := notifier.SendAlert(app.ctx, alert); err != nil {
					log.Printf("Failed to send alert: %v", err)
				}
			}

		case <-app.ctx.Done():
			return
		}
	}
}

func demonstrateMetricsCollection(app *DemoApplication) {
	fmt.Println("1. Metrics Collection Demo")
	fmt.Println("--------------------------")

	// Enable detailed metrics
	app.monitor.EnableDetailedMetrics(true)

	// Simulate various operations
	fmt.Println("  Simulating state operations...")

	for i := 0; i < 100; i++ {
		// Mix of operations
		switch i % 4 {
		case 0: // Set operation
			path := fmt.Sprintf("/metrics/counter_%d", i)
			app.store.Set(path, i)

		case 1: // Get operation
			path := fmt.Sprintf("/metrics/counter_%d", i-1)
			app.store.Get(path)

		case 2: // Transaction
			tx := app.store.Begin()
			tx.Apply(state.JSONPatch{
				{Op: state.JSONPatchOpAdd, Path: fmt.Sprintf("/metrics/tx_%d", i), Value: i},
			})
			tx.Commit()

		case 3: // Subscribe
			path := fmt.Sprintf("/metrics/counter_%d", i%10)
			unsubscribe := app.store.Subscribe(path, func(change state.StateChange) {})
			defer unsubscribe()
		}

		// Simulate some errors
		if i%20 == 0 {
			app.store.Set("/invalid\x00path", "value") // Invalid path
		}

		time.Sleep(10 * time.Millisecond)
	}

	// Get current metrics
	metrics := app.monitor.GetMetrics()

	fmt.Println("\n  Current Metrics:")
	fmt.Printf("    Total operations: %d\n", metrics.TotalOperations)
	fmt.Printf("    Success rate: %.2f%%\n", metrics.SuccessRate*100)
	fmt.Printf("    Error rate: %.2f%%\n", metrics.ErrorRate*100)
	fmt.Printf("    Average latency: %.2fms\n", metrics.AverageLatency)
	fmt.Printf("    P95 latency: %.2fms\n", metrics.P95Latency)
	fmt.Printf("    P99 latency: %.2fms\n", metrics.P99Latency)
	fmt.Printf("    Active connections: %d\n", metrics.ActiveConnections)
	fmt.Printf("    Memory usage: %.2fMB\n", float64(metrics.MemoryUsage)/1024/1024)
	fmt.Println()
}

func demonstrateLoggingAndTracing(app *DemoApplication) {
	fmt.Println("2. Logging and Tracing Demo")
	fmt.Println("---------------------------")

	// Create a traced operation
	span := app.monitor.StartSpan("demo_operation", map[string]string{
		"component": "demo",
		"operation": "complex_update",
	})
	defer span.End()

	// Log structured events
	app.monitor.Logger().Info("Starting complex operation",
		state.String("operation_id", "demo-123"),
		state.Int("batch_size", 50),
		state.Float64("threshold", 0.95),
	)

	// Simulate multi-step operation with child spans
	steps := []string{"validate", "transform", "persist", "notify"}

	for _, step := range steps {
		childSpan := span.CreateChild(step)

		// Log step start
		app.monitor.Logger().Debug(fmt.Sprintf("Executing step: %s", step),
			state.String("step", step),
			state.String("parent_span", span.ID()),
		)

		// Simulate work
		time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)

		// Simulate occasional errors
		if rand.Float32() < 0.1 {
			err := fmt.Errorf("simulated error in step %s", step)
			childSpan.SetError(err)
			app.monitor.Logger().Error("Step failed",
				state.String("step", step),
				state.Err(err),
			)
		}

		childSpan.End()
	}

	// Log completion
	app.monitor.Logger().Info("Complex operation completed",
		state.String("operation_id", "demo-123"),
		state.Duration("total_duration", span.Duration()),
		state.Bool("success", true),
	)

	fmt.Println("  Logged structured events with tracing")
	fmt.Println("  Check your tracing backend (Jaeger) for distributed traces")
	fmt.Println()
}

func demonstrateHealthChecks(app *DemoApplication) {
	fmt.Println("3. Health Checks Demo")
	fmt.Println("---------------------")

	// Run health checks
	fmt.Println("  Running comprehensive health checks...")

	healthReport := app.healthChecker.CheckHealth(app.ctx)

	fmt.Printf("  Overall Status: %s\n", healthReport.Status)
	fmt.Printf("  Timestamp: %s\n\n", healthReport.Timestamp.Format(time.RFC3339))

	// Display component health
	fmt.Println("  Component Health:")
	for name, check := range healthReport.Checks {
		status := "✓"
		if check.Status != state.HealthStatusHealthy {
			status = "✗"
		}
		fmt.Printf("    %s %-20s: %s (%.2fms)\n",
			status, name, check.Status, check.ResponseTime.Seconds()*1000)
		if check.Message != "" {
			fmt.Printf("      Message: %s\n", check.Message)
		}
	}

	// Display resource usage
	fmt.Println("\n  Resource Usage:")
	fmt.Printf("    CPU Usage: %.1f%%\n", healthReport.Metrics.CPUUsage)
	fmt.Printf("    Memory Usage: %.1f%% (%.2fMB)\n",
		healthReport.Metrics.MemoryUsage,
		float64(healthReport.Metrics.MemoryAllocated)/1024/1024)
	fmt.Printf("    Goroutines: %d\n", healthReport.Metrics.GoroutineCount)
	fmt.Printf("    GC Pause (avg): %.2fms\n", healthReport.Metrics.GCPauseAvg)

	// Test degraded scenario
	fmt.Println("\n  Simulating degraded state...")

	// Create high memory usage
	largeData := make([]byte, 100*1024*1024) // 100MB
	_ = largeData

	// Re-run health check
	degradedReport := app.healthChecker.CheckHealth(app.ctx)
	if degradedReport.Status != state.HealthStatusHealthy {
		fmt.Printf("  Status changed to: %s\n", degradedReport.Status)
	}

	fmt.Println()
}

func demonstrateAlerting(app *DemoApplication) {
	fmt.Println("4. Alerting Demo")
	fmt.Println("----------------")

	// Configure alert rules
	app.monitor.ConfigureAlertRule("high_error_rate", state.AlertRule{
		Name:        "High Error Rate",
		Condition:   "error_rate > 5",
		Threshold:   5.0,
		Window:      1 * time.Minute,
		Severity:    state.AlertSeverityCritical,
		Description: "Error rate exceeded 5% threshold",
	})

	app.monitor.ConfigureAlertRule("high_latency", state.AlertRule{
		Name:        "High Latency",
		Condition:   "p99_latency > 500",
		Threshold:   500.0,
		Window:      5 * time.Minute,
		Severity:    state.AlertSeverityWarning,
		Description: "P99 latency exceeded 500ms",
	})

	fmt.Println("  Configured alert rules")
	fmt.Println("  Simulating conditions that trigger alerts...")

	// Generate errors to trigger alert
	for i := 0; i < 20; i++ {
		// Force errors
		app.store.Set("/\x00invalid", "value")
		app.store.Get("/nonexistent/path/that/does/not/exist")

		// Add some successful operations
		if i%3 == 0 {
			app.store.Set(fmt.Sprintf("/alert_test_%d", i), i)
		}

		time.Sleep(100 * time.Millisecond)
	}

	// Generate high latency operations
	for i := 0; i < 10; i++ {
		start := time.Now()

		// Simulate slow operation
		tx := app.store.Begin()
		for j := 0; j < 100; j++ {
			tx.Apply(state.JSONPatch{
				{Op: state.JSONPatchOpAdd, Path: fmt.Sprintf("/slow_%d_%d", i, j), Value: j},
			})
		}
		tx.Commit()

		// Record synthetic high latency
		app.monitor.RecordLatency("slow_operation", time.Since(start)+500*time.Millisecond)

		time.Sleep(200 * time.Millisecond)
	}

	// Wait for alerts to be processed
	time.Sleep(2 * time.Second)

	// Display alert summary
	app.alertManager.mu.RLock()
	defer app.alertManager.mu.RUnlock()

	fmt.Println("\n  Alert Summary:")
	for alertName, count := range app.alertManager.alertCounts {
		fmt.Printf("    %s: %d alerts triggered\n", alertName, count)
	}
	fmt.Println()
}

func demonstratePerformanceMonitoring(app *DemoApplication) {
	fmt.Println("5. Performance Monitoring Demo")
	fmt.Println("------------------------------")

	// Enable performance profiling
	app.monitor.EnableProfiling(true)

	fmt.Println("  Running performance benchmark...")

	// Benchmark different operation types
	operations := []struct {
		name string
		fn   func(int)
	}{
		{
			name: "Sequential Writes",
			fn: func(i int) {
				app.store.Set(fmt.Sprintf("/perf/seq_%d", i), i)
			},
		},
		{
			name: "Random Reads",
			fn: func(i int) {
				app.store.Get(fmt.Sprintf("/perf/seq_%d", rand.Intn(100)))
			},
		},
		{
			name: "Batch Transactions",
			fn: func(i int) {
				tx := app.store.Begin()
				for j := 0; j < 10; j++ {
					tx.Apply(state.JSONPatch{
						{Op: state.JSONPatchOpAdd, Path: fmt.Sprintf("/perf/batch_%d_%d", i, j), Value: j},
					})
				}
				tx.Commit()
			},
		},
		{
			name: "Concurrent Updates",
			fn: func(i int) {
				var wg sync.WaitGroup
				for j := 0; j < 5; j++ {
					wg.Add(1)
					go func(idx int) {
						defer wg.Done()
						app.store.Set(fmt.Sprintf("/perf/concurrent_%d_%d", i, idx), idx)
					}(j)
				}
				wg.Wait()
			},
		},
	}

	// Run benchmarks
	for _, op := range operations {
		fmt.Printf("\n  %s:\n", op.name)

		// Warm up
		for i := 0; i < 10; i++ {
			op.fn(i)
		}

		// Measure
		iterations := 1000
		start := time.Now()

		for i := 0; i < iterations; i++ {
			op.fn(i)
		}

		duration := time.Since(start)
		opsPerSec := float64(iterations) / duration.Seconds()
		avgLatency := duration.Nanoseconds() / int64(iterations) / 1e6 // Convert to ms

		fmt.Printf("    Operations/sec: %.2f\n", opsPerSec)
		fmt.Printf("    Avg latency: %.2fms\n", float64(avgLatency))

		// Get operation-specific metrics
		metrics := app.monitor.GetOperationMetrics(op.name)
		if metrics != nil {
			fmt.Printf("    P95 latency: %.2fms\n", metrics.P95Latency.Seconds()*1000)
			fmt.Printf("    P99 latency: %.2fms\n", metrics.P99Latency.Seconds()*1000)
		}
	}

	// Show resource usage
	fmt.Println("\n  Resource Usage During Benchmark:")
	perfStats := app.performanceOpt.GetStats()
	fmt.Printf("    Memory allocations: %d\n", perfStats.Allocations)
	fmt.Printf("    Pool efficiency: %.2f%% (hits: %d, misses: %d)\n",
		perfStats.PoolEfficiency*100, perfStats.PoolHits, perfStats.PoolMisses)
	fmt.Printf("    GC pauses: %d (avg: %.2fms)\n",
		perfStats.GCPauses, float64(perfStats.AvgGCPause)/1e6)
	fmt.Printf("    Cache hit rate: %.2f%%\n", perfStats.CacheHitRate*100)

	fmt.Println()
}

func demonstrateDashboardMetrics(app *DemoApplication) {
	fmt.Println("6. Dashboard Metrics Demo")
	fmt.Println("-------------------------")

	// Create dashboard data structure
	dashboard := &DashboardData{
		SystemMetrics:      make(map[string]interface{}),
		StateMetrics:       make(map[string]interface{}),
		PerformanceMetrics: make(map[string]interface{}),
		Alerts:             []interface{}{},
		Timestamp:          time.Now(),
	}

	// Collect comprehensive metrics
	fmt.Println("  Collecting dashboard metrics...")

	// System metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	dashboard.SystemMetrics["cpu_cores"] = runtime.NumCPU()
	dashboard.SystemMetrics["goroutines"] = runtime.NumGoroutine()
	dashboard.SystemMetrics["memory_alloc_mb"] = float64(m.Alloc) / 1024 / 1024
	dashboard.SystemMetrics["memory_sys_mb"] = float64(m.Sys) / 1024 / 1024
	dashboard.SystemMetrics["gc_runs"] = m.NumGC
	dashboard.SystemMetrics["gc_pause_ms"] = float64(m.PauseNs[(m.NumGC+255)%256]) / 1e6

	// State metrics
	stateMetrics := app.monitor.GetMetrics()
	dashboard.StateMetrics["total_operations"] = stateMetrics.TotalOperations
	dashboard.StateMetrics["success_rate"] = stateMetrics.SuccessRate
	dashboard.StateMetrics["error_rate"] = stateMetrics.ErrorRate
	dashboard.StateMetrics["active_subscriptions"] = stateMetrics.ActiveSubscriptions
	dashboard.StateMetrics["state_size_bytes"] = stateMetrics.StateSize
	dashboard.StateMetrics["version_count"] = app.store.GetVersion()

	// Performance metrics
	perfStats := app.performanceOpt.GetStats()
	dashboard.PerformanceMetrics["ops_per_second"] = perfStats.OpsPerSecond
	dashboard.PerformanceMetrics["avg_latency_ms"] = perfStats.AvgLatency
	dashboard.PerformanceMetrics["p95_latency_ms"] = perfStats.P95Latency
	dashboard.PerformanceMetrics["p99_latency_ms"] = perfStats.P99Latency
	dashboard.PerformanceMetrics["queue_depth"] = perfStats.QueueDepth
	dashboard.PerformanceMetrics["cache_hit_rate"] = perfStats.CacheHitRate

	// Recent alerts
	app.alertManager.mu.RLock()
	for alert, count := range app.alertManager.alertCounts {
		dashboard.Alerts = append(dashboard.Alerts, map[string]interface{}{
			"name":  alert,
			"count": count,
		})
	}
	app.alertManager.mu.RUnlock()

	// Display dashboard
	fmt.Println("\n  === Real-time Dashboard ===")
	fmt.Printf("  Updated: %s\n\n", dashboard.Timestamp.Format("15:04:05"))

	fmt.Println("  System Health:")
	fmt.Printf("    CPU Cores: %d | Goroutines: %d\n",
		dashboard.SystemMetrics["cpu_cores"],
		dashboard.SystemMetrics["goroutines"])
	fmt.Printf("    Memory: %.1fMB / %.1fMB\n",
		dashboard.SystemMetrics["memory_alloc_mb"],
		dashboard.SystemMetrics["memory_sys_mb"])
	fmt.Printf("    GC: %d runs | Last pause: %.2fms\n",
		dashboard.SystemMetrics["gc_runs"],
		dashboard.SystemMetrics["gc_pause_ms"])

	fmt.Println("\n  State Operations:")
	fmt.Printf("    Total: %d | Success Rate: %.2f%%\n",
		dashboard.StateMetrics["total_operations"],
		dashboard.StateMetrics["success_rate"].(float64)*100)
	fmt.Printf("    Active Subscriptions: %d\n",
		dashboard.StateMetrics["active_subscriptions"])
	fmt.Printf("    State Size: %.2fKB | Versions: %d\n",
		float64(dashboard.StateMetrics["state_size_bytes"].(int64))/1024,
		dashboard.StateMetrics["version_count"])

	fmt.Println("\n  Performance:")
	fmt.Printf("    Throughput: %.0f ops/sec\n",
		dashboard.PerformanceMetrics["ops_per_second"])
	fmt.Printf("    Latency - Avg: %.2fms | P95: %.2fms | P99: %.2fms\n",
		dashboard.PerformanceMetrics["avg_latency_ms"],
		dashboard.PerformanceMetrics["p95_latency_ms"],
		dashboard.PerformanceMetrics["p99_latency_ms"])
	fmt.Printf("    Cache Hit Rate: %.2f%%\n",
		dashboard.PerformanceMetrics["cache_hit_rate"].(float64)*100)

	if len(dashboard.Alerts) > 0 {
		fmt.Println("\n  Active Alerts:")
		for _, alert := range dashboard.Alerts {
			a := alert.(map[string]interface{})
			fmt.Printf("    - %s: %d occurrences\n", a["name"], a["count"])
		}
	}

	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9090"
	}
	fmt.Printf("\n  Dashboard URL: http://localhost:%s/metrics\n", metricsPort)
	fmt.Println()
}

func showMonitoringSummary(app *DemoApplication) {
	fmt.Println("=== Monitoring Summary ===")
	fmt.Println()

	// Get final metrics
	metrics := app.monitor.GetMetrics()
	perfStats := app.performanceOpt.GetStats()

	fmt.Println("Session Statistics:")
	fmt.Printf("  Total Operations: %d\n", metrics.TotalOperations)
	fmt.Printf("  Success Rate: %.2f%%\n", metrics.SuccessRate*100)
	fmt.Printf("  Error Rate: %.2f%%\n", metrics.ErrorRate*100)

	fmt.Println("\nPerformance Summary:")
	fmt.Printf("  Average Throughput: %.0f ops/sec\n", perfStats.OpsPerSecond)
	fmt.Printf("  Average Latency: %.2fms\n", perfStats.AvgLatency)
	fmt.Printf("  P99 Latency: %.2fms\n", perfStats.P99Latency)

	fmt.Println("\nResource Usage:")
	fmt.Printf("  Peak Memory: %.2fMB\n", float64(perfStats.PeakMemory)/1024/1024)
	fmt.Printf("  Total Allocations: %d\n", perfStats.Allocations)
	fmt.Printf("  GC Runs: %d\n", perfStats.GCPauses)

	app.alertManager.mu.RLock()
	totalAlerts := 0
	for _, count := range app.alertManager.alertCounts {
		totalAlerts += count
	}
	app.alertManager.mu.RUnlock()

	fmt.Println("\nAlert Summary:")
	fmt.Printf("  Total Alerts Triggered: %d\n", totalAlerts)
	fmt.Printf("  Alert Types: %d\n", len(app.alertManager.alertCounts))

	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9090"
	}

	healthPort := os.Getenv("HEALTH_PORT")
	if healthPort == "" {
		healthPort = "8080"
	}

	jaegerPort := os.Getenv("JAEGER_UI_PORT")
	if jaegerPort == "" {
		jaegerPort = "16686"
	}

	fmt.Println("\nMonitoring Endpoints:")
	fmt.Printf("  Metrics: http://localhost:%s/metrics (Prometheus)\n", metricsPort)
	fmt.Printf("  Health:  http://localhost:%s/health\n", healthPort)
	fmt.Printf("  Traces:  http://localhost:%s (Jaeger UI)\n", jaegerPort)
}

// Helper functions

func initializeDemoData(store *state.StateStore) {
	// Initialize with sample data
	data := map[string]interface{}{
		"config": map[string]interface{}{
			"app_name":    "monitoring-demo",
			"version":     "1.0.0",
			"environment": "production",
		},
		"features": map[string]interface{}{
			"monitoring": true,
			"alerting":   true,
			"profiling":  true,
		},
		"metrics": map[string]interface{}{
			"startup_time": time.Now().Unix(),
			"counters":     make(map[string]int),
		},
	}

	for key, value := range data {
		store.Set("/"+key, value)
	}
}

func startMetricsServer() {
	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9090"
	}

	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Metrics server listening on :%s", metricsPort)
	if err := http.ListenAndServe(":"+metricsPort, nil); err != nil {
		log.Printf("Metrics server error: %v", err)
	}
}

func startHealthCheckServer(checker *state.HealthChecker) {
	healthPort := os.Getenv("HEALTH_PORT")
	if healthPort == "" {
		healthPort = "8080"
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		report := checker.CheckHealth(r.Context())

		status := http.StatusOK
		if report.Status != state.HealthStatusHealthy {
			status = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(report)
	})

	log.Printf("Health check server listening on :%s", healthPort)
	if err := http.ListenAndServe(":"+healthPort, nil); err != nil {
		log.Printf("Health server error: %v", err)
	}
}

// DashboardData represents real-time dashboard metrics
type DashboardData struct {
	SystemMetrics      map[string]interface{} `json:"system_metrics"`
	StateMetrics       map[string]interface{} `json:"state_metrics"`
	PerformanceMetrics map[string]interface{} `json:"performance_metrics"`
	Alerts             []interface{}          `json:"alerts"`
	Timestamp          time.Time              `json:"timestamp"`
}
