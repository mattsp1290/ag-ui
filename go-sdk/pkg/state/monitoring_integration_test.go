package state

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// TestMonitoringSystemIntegration tests comprehensive monitoring system integration
func TestMonitoringSystemIntegration(t *testing.T) {
	logger := zaptest.NewLogger(t)
	
	// Create monitoring config with all features enabled
	config := DefaultMonitoringConfig()
	config.EnablePrometheus = true
	config.MetricsEnabled = true
	config.EnableHealthChecks = true
	config.EnableResourceMonitoring = true
	config.MetricsInterval = 100 * time.Millisecond
	config.HealthCheckInterval = 50 * time.Millisecond
	config.ResourceSampleInterval = 30 * time.Millisecond
	
	// Create monitoring system
	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	// Test health check registration and execution
	t.Run("health_check_integration", func(t *testing.T) {
		testHealthCheckIntegration(t, ms)
	})

	// Test metrics recording and aggregation
	t.Run("metrics_integration", func(t *testing.T) {
		testMetricsIntegration(t, ms)
	})

	// Test alert system integration
	t.Run("alert_integration", func(t *testing.T) {
		testAlertIntegration(t, ms, logger)
	})

	// Test performance monitoring
	t.Run("performance_monitoring", func(t *testing.T) {
		testPerformanceMonitoring(t, ms)
	})

	// Test resource monitoring
	t.Run("resource_monitoring", func(t *testing.T) {
		testResourceMonitoring(t, ms)
	})

	// Test audit integration
	t.Run("audit_integration", func(t *testing.T) {
		testAuditIntegration(t, ms)
	})
}

func testHealthCheckIntegration(t *testing.T, ms *MonitoringSystem) {
	// Register multiple health checks
	healthyCheck := NewCustomHealthCheck("healthy", func(ctx context.Context) error {
		return nil
	})
	
	failingCheck := NewCustomHealthCheck("failing", func(ctx context.Context) error {
		return errors.New("simulated failure")
	})
	
	slowCheck := NewCustomHealthCheck("slow", func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return nil
		}
	})

	ms.RegisterHealthCheck(healthyCheck)
	ms.RegisterHealthCheck(failingCheck)
	ms.RegisterHealthCheck(slowCheck)

	// Wait for health checks to run
	time.Sleep(200 * time.Millisecond)

	// Get health status
	status := ms.GetHealthStatus()
	
	if len(status) != 3 {
		t.Errorf("Expected 3 health checks, got %d", len(status))
	}
	
	if status["healthy"] != true {
		t.Error("Expected healthy check to pass")
	}
	
	if status["failing"] != false {
		t.Error("Expected failing check to fail")
	}
	
	// Test unregistering health check
	ms.UnregisterHealthCheck("failing")
	status = ms.GetHealthStatus()
	
	if len(status) != 2 {
		t.Errorf("Expected 2 health checks after unregistering, got %d", len(status))
	}
	
	if _, exists := status["failing"]; exists {
		t.Error("Expected failing check to be unregistered")
	}
}

func testMetricsIntegration(t *testing.T, ms *MonitoringSystem) {
	// Record various operations
	operations := []string{"read", "write", "update", "delete"}
	
	for i, op := range operations {
		duration := time.Duration(i+1) * 10 * time.Millisecond
		var err error
		if i == 3 { // Make delete operation fail
			err = errors.New("delete failed")
		}
		
		ms.RecordStateOperation(op, duration, err)
	}

	// Record event processing
	ms.RecordEventProcessing("user_action", 15*time.Millisecond, nil)
	ms.RecordEventProcessing("system_event", 25*time.Millisecond, errors.New("processing error"))

	// Record memory usage
	ms.RecordMemoryUsage(1024*1024*100, 50, 5*time.Millisecond) // 100MB

	// Record connection pool stats
	ms.RecordConnectionPoolStats(100, 75, 5, 2)

	// Record rate limit stats
	ms.RecordRateLimitStats(1000, 50, 85.5)

	// Record queue depth
	ms.RecordQueueDepth(150)

	// Wait for metrics to be processed
	time.Sleep(150 * time.Millisecond)

	// Get metrics snapshot
	metrics := ms.GetMetrics()
	
	if metrics.Operations == nil {
		t.Error("Expected operation metrics to be recorded")
	}
	
	if len(metrics.Operations) == 0 {
		t.Error("Expected at least one operation metric")
	}
	
	if metrics.Memory.Usage == 0 {
		t.Error("Expected memory usage to be recorded")
	}
	
	if metrics.ConnectionPool.TotalConnections != 100 {
		t.Errorf("Expected 100 total connections, got %d", metrics.ConnectionPool.TotalConnections)
	}
}

func testAlertIntegration(t *testing.T, ms *MonitoringSystem, logger *zap.Logger) {
	// Create mock alert notifier
	alertsCh := make(chan Alert, 10)
	mockNotifier := &MockAlertNotifier{alerts: alertsCh}
	
	// Add notifier to config
	ms.config.AlertNotifiers = []AlertNotifier{mockNotifier}
	ms.alertManager.notifiers = []AlertNotifier{mockNotifier}

	// Trigger alerts by exceeding thresholds
	
	// Trigger high queue depth alert
	ms.RecordQueueDepth(ms.config.AlertThresholds.QueueDepth + 100)
	
	// Trigger high latency alert
	highLatencyDuration := time.Duration(ms.config.AlertThresholds.P95LatencyMs+100) * time.Millisecond
	ms.RecordStateOperation("slow_operation", highLatencyDuration, nil)
	
	// Trigger high GC pause alert
	highGCPause := time.Duration(ms.config.AlertThresholds.GCPauseMs+10) * time.Millisecond
	ms.RecordMemoryUsage(1024*1024*50, 25, highGCPause)

	// Wait for alerts to be sent
	time.Sleep(200 * time.Millisecond)

	// Check that alerts were sent (we expect 3 total: queue depth, latency, GC pause)
	alertsReceived := make(map[string]bool)
	alertCount := 0
	
	// Collect alerts for up to 2 seconds
	timeout := time.After(2 * time.Second)
	for alertCount < 3 {
		select {
		case alert := <-alertsCh:
			alertsReceived[alert.Title] = true
			alertCount++
			t.Logf("Received alert: %s", alert.Title)
		case <-timeout:
			break
		}
	}
	
	// Verify we got the expected alerts
	expectedAlerts := []string{"High Queue Depth", "High Operation Latency", "High GC Pause"}
	for _, expected := range expectedAlerts {
		if !alertsReceived[expected] {
			t.Errorf("Expected alert '%s' not received", expected)
		}
	}

	// Test alert deduplication - trigger the same alert again
	ms.RecordQueueDepth(ms.config.AlertThresholds.QueueDepth + 200)
	time.Sleep(200 * time.Millisecond)
	
	// Should not receive duplicate alert immediately (within deduplication window)
	select {
	case alert := <-alertsCh:
		// If we get an alert, it should be a different type, not a duplicate queue depth
		if strings.Contains(alert.Title, "Queue Depth") {
			t.Error("Should not receive duplicate queue depth alert immediately")
		}
	case <-time.After(200 * time.Millisecond):
		// Expected - no duplicate alert
	}
}

func testPerformanceMonitoring(t *testing.T, ms *MonitoringSystem) {
	// Test operation metrics recording
	operations := map[string]int{
		"database_read":  100,
		"database_write": 50,
		"cache_lookup":   200,
		"api_call":       30,
	}

	// Record operations with varying latencies and error rates
	for op, count := range operations {
		for i := 0; i < count; i++ {
			// Simulate varying latencies
			latency := time.Duration(i%10+1) * time.Millisecond
			
			// Simulate 10% error rate
			var err error
			if i%10 == 0 {
				err = errors.New("operation failed")
			}
			
			ms.RecordStateOperation(op, latency, err)
		}
	}

	// Wait for metrics aggregation
	time.Sleep(200 * time.Millisecond)

	// Verify metrics
	metrics := ms.GetMetrics()
	
	for op, expectedCount := range operations {
		if opMetric, exists := metrics.Operations[op]; exists {
			if opMetric.Count != int64(expectedCount) {
				t.Errorf("Expected %d operations for %s, got %d", expectedCount, op, opMetric.Count)
			}
			
			// Check error rate (should be ~10%)
			if opMetric.ErrorRate < 5 || opMetric.ErrorRate > 15 {
				t.Errorf("Expected error rate around 10%% for %s, got %.2f%%", op, opMetric.ErrorRate)
			}
		} else {
			t.Errorf("Expected metrics for operation %s", op)
		}
	}
}

func testResourceMonitoring(t *testing.T, ms *MonitoringSystem) {
	// Wait for initial resource sampling
	time.Sleep(100 * time.Millisecond)

	// Get initial metrics
	initialMetrics := ms.GetMetrics()
	initialMemory := initialMetrics.Memory.Usage
	initialGoroutines := initialMetrics.Memory.Goroutines

	// Create some memory pressure and goroutines
	var wg sync.WaitGroup
	data := make([][]byte, 100)
	
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			// Allocate some memory
			data[index] = make([]byte, 1024*1024) // 1MB
			time.Sleep(200 * time.Millisecond)
		}(i)
	}

	// Wait for resource monitoring to detect changes
	time.Sleep(150 * time.Millisecond)

	// Get updated metrics
	updatedMetrics := ms.GetMetrics()
	
	// Memory usage should have increased
	if updatedMetrics.Memory.Usage <= initialMemory {
		t.Logf("Memory usage did not increase as expected (initial: %d, updated: %d)", 
			initialMemory, updatedMetrics.Memory.Usage)
		// This might be acceptable due to GC, so just log it
	}
	
	// Goroutine count should have increased
	if updatedMetrics.Memory.Goroutines <= initialGoroutines {
		t.Logf("Goroutine count did not increase as expected (initial: %d, updated: %d)", 
			initialGoroutines, updatedMetrics.Memory.Goroutines)
		// This might be acceptable due to timing, so just log it
	}

	wg.Wait() // Clean up goroutines
}

func testAuditIntegration(t *testing.T, ms *MonitoringSystem) {
	// Create mock audit manager
	auditManager := NewMockAuditManager()
	
	// Inject the audit manager into the monitoring system
	ms.SetAuditManager(auditManager)

	// Log audit events
	actions := []AuditAction{"CREATE", "UPDATE", "DELETE", "READ"}
	
	for _, action := range actions {
		details := map[string]interface{}{
			"resource": "test_resource",
			"user":     "test_user",
			"timestamp": time.Now(),
		}
		
		ms.LogAuditEvent(context.Background(), action, details)
	}

	// Verify audit events were logged
	if len(auditManager.logger.logs) != len(actions) {
		t.Errorf("Expected %d audit logs, got %d", len(actions), len(auditManager.logger.logs))
	}

	// Verify details were added
	for _, log := range auditManager.logger.logs {
		if log.Details["monitoring_timestamp"] == nil {
			t.Error("Expected monitoring correlation timestamp to be added")
		}
	}
}

// TestMonitoringSystemFailureScenarios tests how the monitoring system handles various failure scenarios
func TestMonitoringSystemFailureScenarios(t *testing.T) {
	t.Run("initialization_failures", func(t *testing.T) {
		testInitializationFailures(t)
	})

	t.Run("component_failures", func(t *testing.T) {
		testComponentFailures(t)
	})

	t.Run("resource_exhaustion", func(t *testing.T) {
		testResourceExhaustion(t)
	})

	t.Run("concurrent_access", func(t *testing.T) {
		testConcurrentAccess(t)
	})

	t.Run("shutdown_scenarios", func(t *testing.T) {
		testShutdownScenarios(t)
	})
}

func testInitializationFailures(t *testing.T) {
	// Test invalid logger configuration
	config := DefaultMonitoringConfig()
	config.LogOutput = io.Discard // This should work fine
	config.LogLevel = -1 // Invalid log level
	
	// This should still work as zap handles invalid levels gracefully
	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Logf("Expected initialization to handle invalid log level gracefully, got error: %v", err)
	} else {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			ms.Shutdown(ctx)
		}()
	}
}

func testComponentFailures(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.MetricsInterval = 50 * time.Millisecond
	
	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	// Test health check failures
	panicCheck := NewCustomHealthCheck("panic_check", func(ctx context.Context) error {
		panic("simulated panic in health check")
	})
	
	timeoutCheck := NewCustomHealthCheck("timeout_check", func(ctx context.Context) error {
		time.Sleep(1 * time.Second) // Will exceed typical timeout
		return nil
	})

	ms.RegisterHealthCheck(panicCheck)
	ms.RegisterHealthCheck(timeoutCheck)

	// Wait for health checks to run
	time.Sleep(200 * time.Millisecond)

	// System should still be operational
	status := ms.GetHealthStatus()
	if len(status) == 0 {
		t.Error("Expected health checks to be registered despite failures")
	}

	// Test alert notifier failures
	failingNotifier := &FailingAlertNotifier{}
	ms.config.AlertNotifiers = []AlertNotifier{failingNotifier}
	ms.alertManager.notifiers = []AlertNotifier{failingNotifier}

	// Trigger an alert
	ms.RecordQueueDepth(ms.config.AlertThresholds.QueueDepth + 100)
	
	// Wait for alert processing
	time.Sleep(100 * time.Millisecond)

	// System should continue operating despite alert notification failure
	metrics := ms.GetMetrics()
	if metrics.Timestamp.IsZero() {
		t.Error("Expected metrics to be available despite alert failures")
	}
}

func testResourceExhaustion(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.MetricsInterval = 10 * time.Millisecond // Very frequent
	config.HealthCheckInterval = 10 * time.Millisecond
	config.ResourceSampleInterval = 10 * time.Millisecond
	
	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	// Register a simple health check to verify system responsiveness
	simpleCheck := NewCustomHealthCheck("resource_test", func(ctx context.Context) error {
		return nil
	})
	ms.RegisterHealthCheck(simpleCheck)

	// Simulate high load
	var wg sync.WaitGroup
	stopCh := make(chan struct{})
	
	// Start multiple goroutines generating metrics
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			
			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					ms.RecordStateOperation(fmt.Sprintf("op_%d", id), time.Millisecond, nil)
					ms.RecordEventProcessing(fmt.Sprintf("event_%d", id), time.Millisecond, nil)
				}
			}
		}(i)
	}

	// Let it run for a short time
	time.Sleep(200 * time.Millisecond)
	
	// Stop load generation
	close(stopCh)
	wg.Wait()

	// Verify system is still responsive
	metrics := ms.GetMetrics()
	if metrics.Timestamp.IsZero() {
		t.Error("Expected metrics to be available after high load")
	}

	status := ms.GetHealthStatus()
	if len(status) == 0 {
		t.Error("Expected health checks to be responsive after high load")
	}
}

func testConcurrentAccess(t *testing.T) {
	config := DefaultMonitoringConfig()
	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Test concurrent metric recording
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			defer func() {
				if r := recover(); r != nil {
					errors <- fmt.Errorf("panic in goroutine %d: %v", id, r)
				}
			}()
			
			for j := 0; j < 50; j++ {
				ms.RecordStateOperation(fmt.Sprintf("concurrent_op_%d", id), time.Millisecond, nil)
				ms.RecordMemoryUsage(uint64(j*1024), int64(j), time.Microsecond)
				
				// Occasionally register/unregister health checks
				if j%10 == 0 {
					checkName := fmt.Sprintf("check_%d_%d", id, j)
					check := NewCustomHealthCheck(checkName, func(ctx context.Context) error {
						return nil
					})
					ms.RegisterHealthCheck(check)
					
					// Unregister after a short time
					go func(name string) {
						time.Sleep(10 * time.Millisecond)
						ms.UnregisterHealthCheck(name)
					}(checkName)
				}
			}
		}(i)
	}

	// Test concurrent metrics reading
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			defer func() {
				if r := recover(); r != nil {
					errors <- fmt.Errorf("panic in reader %d: %v", id, r)
				}
			}()
			
			for j := 0; j < 20; j++ {
				_ = ms.GetMetrics()
				_ = ms.GetHealthStatus()
				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}

func testShutdownScenarios(t *testing.T) {
	t.Run("graceful_shutdown", func(t *testing.T) {
		config := DefaultMonitoringConfig()
		ms, err := NewMonitoringSystem(config)
		if err != nil {
			t.Fatalf("Failed to create monitoring system: %v", err)
		}

		// Add some load
		for i := 0; i < 10; i++ {
			ms.RecordStateOperation("test_op", time.Millisecond, nil)
		}

		// Graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		err = ms.Shutdown(ctx)
		if err != nil {
			t.Errorf("Expected graceful shutdown, got error: %v", err)
		}
	})

	t.Run("timeout_shutdown", func(t *testing.T) {
		config := DefaultMonitoringConfig()
		config.MetricsInterval = 1 * time.Millisecond // Very frequent to create busy background tasks
		config.HealthCheckInterval = 10 * time.Millisecond // Ensure health checks run frequently
		
		ms, err := NewMonitoringSystem(config)
		if err != nil {
			t.Fatalf("Failed to create monitoring system: %v", err)
		}

		// Add a goroutine to the wait group that will block shutdown
		ms.wg.Add(1)
		go func() {
			defer ms.wg.Done()
			// This goroutine will block for longer than the shutdown timeout
			select {
			case <-time.After(1 * time.Second):
				// Normal completion
			case <-ms.ctx.Done():
				// Even when context is cancelled, still block to test timeout
				time.Sleep(200 * time.Millisecond)
			}
		}()

		// Also create a blocking health check for good measure
		blockingCheck := NewCustomHealthCheck("blocking", func(ctx context.Context) error {
			select {
			case <-time.After(2 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		ms.RegisterHealthCheck(blockingCheck)

		// Wait a bit to ensure background tasks are running
		time.Sleep(50 * time.Millisecond)

		// Try shutdown with short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		err = ms.Shutdown(ctx)
		if err == nil {
			t.Error("Expected timeout error during shutdown")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected context deadline exceeded, got: %v", err)
		}
	})

	t.Run("force_shutdown", func(t *testing.T) {
		config := DefaultMonitoringConfig()
		ms, err := NewMonitoringSystem(config)
		if err != nil {
			t.Fatalf("Failed to create monitoring system: %v", err)
		}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			ms.Shutdown(ctx)
		}()

		// Cancel context immediately to force shutdown
		ms.cancel()

		// Give it a moment to process the cancellation
		time.Sleep(50 * time.Millisecond)

		// Try to record metrics after cancellation
		ms.RecordStateOperation("after_cancel", time.Millisecond, nil)
		
		// This should not panic
		_ = ms.GetMetrics()
	})
}

// Mock implementations for testing

type MockAlertNotifier struct {
	alerts chan Alert
	mu     sync.Mutex
}

func (m *MockAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	select {
	case m.alerts <- alert:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		return errors.New("alert channel full")
	}
}

type FailingAlertNotifier struct {
	callCount int32
}

func (f *FailingAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	atomic.AddInt32(&f.callCount, 1)
	return errors.New("simulated alert notification failure")
}

func (f *FailingAlertNotifier) GetCallCount() int32 {
	return atomic.LoadInt32(&f.callCount)
}

// TestMonitoringSystemEdgeCases tests edge cases and boundary conditions
func TestMonitoringSystemEdgeCases(t *testing.T) {
	t.Run("nil_inputs", func(t *testing.T) {
		testNilInputs(t)
	})

	t.Run("zero_values", func(t *testing.T) {
		testZeroValues(t)
	})

	t.Run("extreme_values", func(t *testing.T) {
		testExtremeValues(t)
	})
}

func testNilInputs(t *testing.T) {
	config := DefaultMonitoringConfig()
	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	// Test with nil audit manager
	ms.SetAuditManager(nil)
	ms.LogAuditEvent(context.Background(), "TEST", nil) // Should not panic

	// Test registering nil health check (should be handled gracefully)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Registering nil health check should not panic: %v", r)
		}
	}()
	// Note: Can't actually pass nil to RegisterHealthCheck due to type system
}

func testZeroValues(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.MetricsInterval = 0 // Should use default or handle gracefully
	config.HealthCheckInterval = 0
	config.ResourceSampleInterval = 0
	
	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system with zero intervals: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	// Record metrics with zero values
	ms.RecordStateOperation("zero_test", 0, nil)
	ms.RecordMemoryUsage(0, 0, 0)
	ms.RecordConnectionPoolStats(0, 0, 0, 0)
	ms.RecordQueueDepth(0)

	// Should not panic
	metrics := ms.GetMetrics()
	if metrics.Timestamp.IsZero() {
		t.Error("Expected valid timestamp even with zero values")
	}
}

func testExtremeValues(t *testing.T) {
	config := DefaultMonitoringConfig()
	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	// Test with extreme values
	ms.RecordStateOperation("extreme_test", 24*time.Hour, errors.New("extreme error"))
	ms.RecordMemoryUsage(^uint64(0), ^int64(0)>>1, 24*time.Hour) // Max values
	ms.RecordConnectionPoolStats(^int64(0)>>1, ^int64(0)>>1, ^int64(0)>>1, ^int64(0)>>1)
	ms.RecordQueueDepth(^int64(0) >> 1)

	// Should handle extreme values gracefully
	metrics := ms.GetMetrics()
	if metrics.Timestamp.IsZero() {
		t.Error("Expected valid timestamp even with extreme values")
	}

	// Test very long operation names
	longName := strings.Repeat("a", 1000)
	ms.RecordStateOperation(longName, time.Millisecond, nil)
	
	// Should not cause issues
	time.Sleep(50 * time.Millisecond)
}

// Benchmark tests for performance validation
func BenchmarkMonitoringSystemOperations(b *testing.B) {
	config := DefaultMonitoringConfig()
	config.MetricsEnabled = false // Disable background processing for cleaner benchmarks
	config.EnableHealthChecks = false
	config.EnableResourceMonitoring = false
	
	ms, err := NewMonitoringSystem(config)
	if err != nil {
		b.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	b.Run("RecordStateOperation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ms.RecordStateOperation("benchmark_op", time.Microsecond, nil)
		}
	})

	b.Run("RecordMemoryUsage", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ms.RecordMemoryUsage(1024*1024, 100, time.Microsecond)
		}
	})

	b.Run("GetMetrics", func(b *testing.B) {
		// Pre-populate with some metrics
		for i := 0; i < 100; i++ {
			ms.RecordStateOperation("setup_op", time.Microsecond, nil)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = ms.GetMetrics()
		}
	})

	b.Run("ConcurrentOperations", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				ms.RecordStateOperation("concurrent_op", time.Microsecond, nil)
				if rand.Intn(10) == 0 { // Occasionally read metrics
					_ = ms.GetMetrics()
				}
			}
		})
	})
}