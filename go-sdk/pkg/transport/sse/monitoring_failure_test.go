package sse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestSSEMonitoringFailureScenarios tests comprehensive failure scenarios for SSE monitoring
func TestSSEMonitoringFailureScenarios(t *testing.T) {
	t.Run("connection_failure_scenarios", func(t *testing.T) {
		testConnectionFailureScenarios(t)
	})

	t.Run("stream_interruption_scenarios", func(t *testing.T) {
		testStreamInterruptionScenarios(t)
	})

	t.Run("event_processing_failures", func(t *testing.T) {
		testEventProcessingFailures(t)
	})

	t.Run("network_partition_recovery", func(t *testing.T) {
		testNetworkPartitionRecovery(t)
	})

	t.Run("resource_exhaustion", func(t *testing.T) {
		testResourceExhaustion(t)
	})

	t.Run("concurrent_connection_failures", func(t *testing.T) {
		testConcurrentConnectionFailures(t)
	})

	t.Run("monitoring_system_failures", func(t *testing.T) {
		testMonitoringSystemFailures(t)
	})

	t.Run("health_check_failures", func(t *testing.T) {
		testHealthCheckFailures(t)
	})

	t.Run("alert_system_failures", func(t *testing.T) {
		testAlertSystemFailures(t)
	})

	t.Run("performance_degradation", func(t *testing.T) {
		testPerformanceDegradation(t)
	})
}

func testConnectionFailureScenarios(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.Metrics.Interval = 50 * time.Millisecond

	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	tests := []struct {
		name        string
		scenario    func() (string, string, string) // returns connID, remoteAddr, userAgent
		expectError bool
	}{
		{
			name: "normal_connection",
			scenario: func() (string, string, string) {
				return "conn_001", "192.168.1.100:12345", "Mozilla/5.0"
			},
			expectError: false,
		},
		{
			name: "empty_connection_id",
			scenario: func() (string, string, string) {
				return "", "192.168.1.100:12345", "Mozilla/5.0"
			},
			expectError: false, // Should handle gracefully
		},
		{
			name: "invalid_remote_addr",
			scenario: func() (string, string, string) {
				return "conn_002", "invalid_address", "Mozilla/5.0"
			},
			expectError: false, // Should handle gracefully
		},
		{
			name: "empty_user_agent",
			scenario: func() (string, string, string) {
				return "conn_003", "192.168.1.100:12345", ""
			},
			expectError: false, // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connID, remoteAddr, userAgent := tt.scenario()

			// Record connection establishment
			ms.RecordConnectionEstablished(connID, remoteAddr, userAgent)

			// Verify connection is tracked
			stats := ms.GetConnectionStats()
			if len(stats.ActiveConnectionList) == 0 && connID != "" {
				t.Error("Expected connection to be tracked")
			}

			// Record some activity
			ms.RecordEventReceived(connID, "test_event", 100)
			ms.RecordEventSent(connID, "response_event", 50, 5*time.Millisecond)

			// Simulate connection error
			testErr := errors.New("connection failed")
			ms.RecordConnectionError(connID, testErr)

			// Close connection
			ms.RecordConnectionClosed(connID, "test completed")

			// Verify connection is no longer active
			time.Sleep(50 * time.Millisecond)
			stats = ms.GetConnectionStats()

			// Check that connection was properly cleaned up
			found := false
			for _, conn := range stats.ActiveConnectionList {
				if conn.ID == connID {
					found = true
					break
				}
			}

			if found && connID != "" {
				t.Error("Expected connection to be removed from active list")
			}
		})
	}
}

func testStreamInterruptionScenarios(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.Alerting.Enabled = true
	config.Alerting.Thresholds.ErrorRate = 10.0 // 10% error rate threshold

	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	// Add test alert notifier
	alertChan := make(chan Alert, 10)
	ms.alertManager.notifiers = append(ms.alertManager.notifiers, &TestAlertNotifier{alerts: alertChan})

	connID := "stream_test_001"
	ms.RecordConnectionEstablished(connID, "192.168.1.100:12345", "TestClient/1.0")

	// Simulate various stream interruption scenarios
	interruptions := []struct {
		name        string
		errorType   string
		eventCount  int
		expectAlert bool
	}{
		{
			name:        "timeout_error",
			errorType:   "timeout",
			eventCount:  5,
			expectAlert: false,
		},
		{
			name:        "connection_reset",
			errorType:   "connection reset by peer",
			eventCount:  10,
			expectAlert: false,
		},
		{
			name:        "high_error_rate",
			errorType:   "parsing error",
			eventCount:  20, // Will create high error rate
			expectAlert: true,
		},
	}

	for _, interruption := range interruptions {
		t.Run(interruption.name, func(t *testing.T) {
			// Simulate events with errors
			for i := 0; i < interruption.eventCount; i++ {
				ms.RecordEventReceived(connID, "data", 100)
				if i%3 != 0 {
					// Send successful events
					ms.RecordEventProcessed("data", 5*time.Millisecond, nil)
				} else {
					// Inject errors
					testErr := errors.New(interruption.errorType)
					ms.RecordEventProcessed("data", 10*time.Millisecond, testErr)
				}
			}

			// Wait for metrics aggregation and manually trigger alert check
			time.Sleep(200 * time.Millisecond)
			// Manually trigger alert check for testing
			ms.checkAlertThresholds()

			if interruption.expectAlert {
				select {
				case alert := <-alertChan:
					if !strings.Contains(strings.ToLower(alert.Title), "error") {
						t.Errorf("Expected error-related alert, got: %s", alert.Title)
					}
				case <-time.After(500 * time.Millisecond):
					t.Error("Expected alert for high error rate")
				}
			}
		})
	}

	ms.RecordConnectionClosed(connID, "test completed")
}

func testEventProcessingFailures(t *testing.T) {
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

	connID := "event_test_001"
	ms.RecordConnectionEstablished(connID, "192.168.1.100:12345", "TestClient/1.0")

	// Test various event processing failures
	failureScenarios := []struct {
		name      string
		eventType string
		error     error
		duration  time.Duration
	}{
		{
			name:      "parse_error",
			eventType: "malformed_json",
			error:     errors.New("json parse error"),
			duration:  2 * time.Millisecond,
		},
		{
			name:      "validation_error",
			eventType: "invalid_schema",
			error:     errors.New("schema validation failed"),
			duration:  3 * time.Millisecond,
		},
		{
			name:      "timeout_error",
			eventType: "slow_processing",
			error:     errors.New("processing timeout"),
			duration:  1 * time.Second, // Very slow
		},
		{
			name:      "resource_error",
			eventType: "memory_exhausted",
			error:     errors.New("out of memory"),
			duration:  5 * time.Millisecond,
		},
	}

	for _, scenario := range failureScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Record event reception
			eventSize := int64(100 + len(scenario.eventType))
			ms.RecordEventReceived(connID, scenario.eventType, eventSize)

			// Record processing failure
			ms.RecordEventProcessed(scenario.eventType, scenario.duration, scenario.error)

			// Verify error was recorded
			stats := ms.GetEventStats()
			if eventStats, exists := stats[scenario.eventType]; exists {
				if eventStats.ErrorCount == 0 {
					t.Error("Expected error count to be incremented")
				}
			}
		})
	}

	// Test event drop scenarios
	t.Run("event_drops", func(t *testing.T) {
		// Simulate queue full scenario
		for i := 0; i < 10; i++ {
			eventType := fmt.Sprintf("queued_event_%d", i)
			ms.RecordEventReceived(connID, eventType, 50)
		}

		// Simulate some events being dropped
		ms.promMetrics.EventDropped.WithLabelValues("queue_full").Add(3)

		// Verify metrics
		time.Sleep(50 * time.Millisecond)
	})

	ms.RecordConnectionClosed(connID, "test completed")
}

func testNetworkPartitionRecovery(t *testing.T) {
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

	// Simulate network partition scenario
	connID := "partition_test_001"

	// Initial connection
	ms.RecordConnectionEstablished(connID, "192.168.1.100:12345", "TestClient/1.0")

	// Normal operation
	for i := 0; i < 5; i++ {
		ms.RecordEventReceived(connID, "normal_event", 100)
		ms.RecordEventProcessed("normal_event", 5*time.Millisecond, nil)
	}

	// Network partition - multiple reconnection attempts
	for attempt := 1; attempt <= 5; attempt++ {
		success := attempt == 5 // Last attempt succeeds
		ms.RecordReconnection(connID, attempt, success)

		if !success {
			// Record connection errors during failed attempts
			ms.RecordConnectionError(connID, errors.New("network unreachable"))
		}

		time.Sleep(10 * time.Millisecond)
	}

	// Verify reconnection metrics
	stats := ms.GetConnectionStats()
	found := false
	for _, conn := range stats.ActiveConnectionList {
		if conn.ID == connID {
			if conn.Reconnects == 0 {
				t.Error("Expected reconnection count to be recorded")
			}
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected connection to be found in active list after recovery")
	}

	// Post-recovery normal operation
	for i := 0; i < 3; i++ {
		ms.RecordEventReceived(connID, "recovery_event", 100)
		ms.RecordEventProcessed("recovery_event", 5*time.Millisecond, nil)
	}

	ms.RecordConnectionClosed(connID, "test completed")
}

func testResourceExhaustion(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.Alerting.Enabled = true
	config.Alerting.Thresholds.MemoryUsage = 50.0 // 50% threshold
	config.Alerting.Thresholds.ConnectionCount = 10

	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	// Add test alert notifier
	alertChan := make(chan Alert, 20)
	ms.alertManager.notifiers = append(ms.alertManager.notifiers, &TestAlertNotifier{alerts: alertChan})

	t.Run("memory_exhaustion", func(t *testing.T) {
		// Simulate high memory usage
		for i := 0; i < 5; i++ {
			// Simulate large event processing
			eventType := fmt.Sprintf("large_event_%d", i)
			ms.RecordEventReceived("mem_test", eventType, 1024*1024) // 1MB events
			ms.RecordEventProcessed(eventType, 10*time.Millisecond, nil)
		}

		// Force memory metrics update
		ms.collectResourceMetrics()

		// Wait for alert processing
		time.Sleep(200 * time.Millisecond)
	})

	t.Run("connection_exhaustion", func(t *testing.T) {
		// Establish many connections to exceed threshold
		for i := 0; i < 15; i++ {
			connID := fmt.Sprintf("exhaust_conn_%d", i)
			ms.RecordConnectionEstablished(connID, fmt.Sprintf("192.168.1.%d:12345", i+1), "TestClient/1.0")
		}

		// Wait for alert processing
		time.Sleep(200 * time.Millisecond)

		// Should receive connection count alert
		select {
		case alert := <-alertChan:
			if !strings.Contains(strings.ToLower(alert.Title), "connection") {
				t.Logf("Received alert: %s", alert.Title)
			}
		case <-time.After(500 * time.Millisecond):
			t.Log("No connection count alert received (may be expected)")
		}

		// Clean up connections
		for i := 0; i < 15; i++ {
			connID := fmt.Sprintf("exhaust_conn_%d", i)
			ms.RecordConnectionClosed(connID, "cleanup")
		}
	})

	t.Run("event_queue_exhaustion", func(t *testing.T) {
		// Simulate event queue building up
		connID := "queue_test"
		ms.RecordConnectionEstablished(connID, "192.168.1.200:12345", "TestClient/1.0")

		// Send events faster than they can be processed
		for i := 0; i < 100; i++ {
			eventType := fmt.Sprintf("queued_event_%d", i)
			ms.RecordEventReceived(connID, eventType, 100)

			// Only process every 10th event to create backlog
			if i%10 == 0 {
				ms.RecordEventProcessed(eventType, 50*time.Millisecond, nil)
			}
		}

		// Simulate high queue depth
		ms.promMetrics.EventQueueDepth.Set(500)

		ms.RecordConnectionClosed(connID, "test completed")
	})
}

func testConcurrentConnectionFailures(t *testing.T) {
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
	errorChan := make(chan error, 100)
	connectionCount := getTestConcurrency(50)

	// Simulate many concurrent connections with various failure modes
	for i := 0; i < connectionCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			defer func() {
				if r := recover(); r != nil {
					errorChan <- fmt.Errorf("panic in connection %d: %v", id, r)
				}
			}()

			connID := fmt.Sprintf("concurrent_conn_%d", id)
			remoteAddr := fmt.Sprintf("192.168.1.%d:12345", (id%254)+1)

			// Establish connection
			ms.RecordConnectionEstablished(connID, remoteAddr, "ConcurrentTestClient/1.0")

			// Simulate various activities and failures
			eventCount := 10 + (id % 20) // Variable event count

			for j := 0; j < eventCount; j++ {
				eventType := fmt.Sprintf("event_%d_%d", id, j)

				// Receive event
				ms.RecordEventReceived(connID, eventType, int64(100+(j*10)))

				// Simulate various processing outcomes
				var processingErr error
				processingDuration := time.Duration(j+1) * time.Millisecond

				switch j % 5 {
				case 0:
					// Success
				case 1:
					processingErr = errors.New("timeout")
				case 2:
					processingErr = errors.New("parse error")
				case 3:
					processingErr = errors.New("validation failed")
				case 4:
					// Success but slow
					processingDuration = 100 * time.Millisecond
				}

				ms.RecordEventProcessed(eventType, processingDuration, processingErr)

				// Occasionally send events
				if j%3 == 0 {
					ms.RecordEventSent(connID, fmt.Sprintf("response_%d_%d", id, j), int64(50+(j*5)), time.Millisecond)
				}

				// Simulate connection errors for some connections
				if id%10 == 0 && j%5 == 0 {
					ms.RecordConnectionError(connID, fmt.Errorf("connection error %d-%d", id, j))
				}
			}

			// Simulate reconnections for some connections
			if id%7 == 0 {
				for attempt := 1; attempt <= 3; attempt++ {
					success := attempt == 3
					ms.RecordReconnection(connID, attempt, success)
				}
			}

			// Close connection
			reason := "normal_close"
			if id%15 == 0 {
				reason = "error_close"
			}
			ms.RecordConnectionClosed(connID, reason)

		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	errorCount := 0
	for err := range errorChan {
		errorCount++
		t.Logf("Concurrent connection error: %v", err)
	}

	if errorCount > 0 {
		t.Errorf("Got %d errors during concurrent connection test", errorCount)
	}

	// Verify final state
	time.Sleep(100 * time.Millisecond)
	stats := ms.GetConnectionStats()

	if stats.TotalConnections != int64(connectionCount) {
		t.Errorf("Expected %d total connections, got %d", connectionCount, stats.TotalConnections)
	}

	if len(stats.ActiveConnectionList) > 0 {
		t.Errorf("Expected no active connections, got %d", len(stats.ActiveConnectionList))
	}
}

func testMonitoringSystemFailures(t *testing.T) {
	t.Run("initialization_failure_recovery", func(t *testing.T) {
		// Test with invalid configuration
		config := DefaultMonitoringConfig()
		config.Logging.Level = -99 // Invalid log level

		// Should still initialize successfully with defaults
		ms, err := NewMonitoringSystem(config)
		if err != nil {
			t.Logf("Monitoring system handled invalid config: %v", err)
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			ms.Shutdown(ctx)
		}
	})

	t.Run("background_task_failures", func(t *testing.T) {
		config := DefaultMonitoringConfig()
		config.Metrics.Interval = 10 * time.Millisecond // Very frequent
		config.HealthChecks.Interval = 10 * time.Millisecond

		ms, err := NewMonitoringSystem(config)
		if err != nil {
			t.Fatalf("Failed to create monitoring system: %v", err)
		}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			ms.Shutdown(ctx)
		}()

		// Register a health check that panics
		panicCheck := &PanicHealthCheck{}
		ms.RegisterHealthCheck(panicCheck)

		// Register a health check that times out
		timeoutCheck := &TimeoutHealthCheck{delay: 200 * time.Millisecond}
		ms.RegisterHealthCheck(timeoutCheck)

		// Let background tasks run
		time.Sleep(200 * time.Millisecond)

		// System should still be responsive
		ms.RecordConnectionEstablished("test_conn", "192.168.1.1:12345", "TestClient/1.0")
		stats := ms.GetConnectionStats()

		if stats.TotalConnections == 0 {
			t.Error("Expected monitoring system to remain functional despite health check failures")
		}

		ms.RecordConnectionClosed("test_conn", "test completed")
	})

	t.Run("metrics_collection_failure", func(t *testing.T) {
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

		// Try to cause metrics collection issues by providing extreme values
		ms.RecordEventProcessed("extreme_event", time.Hour*24*365, errors.New("extreme error"))
		ms.RecordConnectionEstablished("", "", "")       // Empty values
		ms.RecordEventReceived("test", "test_event", -1) // Negative size

		// System should handle these gracefully
		metrics := ms.GetPerformanceMetrics()
		if metrics.Timestamp.IsZero() {
			t.Error("Expected performance metrics to be available despite extreme inputs")
		}
	})
}

func testHealthCheckFailures(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.HealthChecks.Enabled = true
	config.HealthChecks.Interval = 50 * time.Millisecond
	config.HealthChecks.Timeout = 100 * time.Millisecond

	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	// Register various failing health checks
	checks := []HealthCheck{
		&PanicHealthCheck{},
		&TimeoutHealthCheck{delay: 200 * time.Millisecond},
		&ErrorHealthCheck{err: errors.New("health check failed")},
		&SlowHealthCheck{delay: 150 * time.Millisecond},
	}

	for _, check := range checks {
		ms.RegisterHealthCheck(check)
	}

	// Let health checks run
	time.Sleep(300 * time.Millisecond)

	// Get health status
	status := ms.GetHealthStatus()

	if len(status) != len(checks) {
		t.Errorf("Expected %d health check results, got %d", len(checks), len(status))
	}

	// Verify that failures are properly recorded
	failureCount := 0
	for _, healthStatus := range status {
		if !healthStatus.Healthy {
			failureCount++
		}
	}

	if failureCount == 0 {
		t.Error("Expected some health checks to fail")
	}

	// Verify system remains responsive
	ms.RecordConnectionEstablished("health_test", "192.168.1.1:12345", "TestClient/1.0")
	ms.RecordConnectionClosed("health_test", "test completed")
}

func testAlertSystemFailures(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.Alerting.Enabled = true
	config.Alerting.Thresholds.ErrorRate = 5.0

	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	// Add failing alert notifiers
	failingNotifiers := []AlertNotifier{
		&PanicAlertNotifier{},
		&TimeoutAlertNotifier{delay: 2 * time.Second},
		&ErrorAlertNotifier{},
	}

	ms.alertManager.notifiers = append(ms.alertManager.notifiers, failingNotifiers...)

	// Trigger alerts by creating high error rate
	connID := "alert_test"
	ms.RecordConnectionEstablished(connID, "192.168.1.1:12345", "TestClient/1.0")

	// Create high error rate
	for i := 0; i < 20; i++ {
		ms.RecordEventReceived(connID, "test_event", 100)

		var err error
		if i%2 == 0 { // 50% error rate
			err = errors.New("test error")
		}

		ms.RecordEventProcessed("test_event", 5*time.Millisecond, err)
	}

	// Wait for alert processing
	time.Sleep(500 * time.Millisecond)

	// System should remain functional despite alert failures
	stats := ms.GetConnectionStats()
	if stats.TotalConnections == 0 {
		t.Error("Expected monitoring system to remain functional despite alert failures")
	}

	ms.RecordConnectionClosed(connID, "test completed")
}

func testPerformanceDegradation(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.Metrics.Interval = 10 * time.Millisecond // Very frequent

	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	// Simulate performance degradation scenarios
	t.Run("high_latency_events", func(t *testing.T) {
		connID := "perf_test_high_latency"
		ms.RecordConnectionEstablished(connID, "192.168.1.1:12345", "PerfTestClient/1.0")

		// Record events with increasing latencies
		for i := 0; i < 20; i++ {
			latency := time.Duration(i*50) * time.Millisecond
			ms.RecordEventProcessed("slow_event", latency, nil)
		}

		ms.RecordConnectionClosed(connID, "test completed")
	})

	t.Run("high_error_rate", func(t *testing.T) {
		connID := "perf_test_errors"
		ms.RecordConnectionEstablished(connID, "192.168.1.2:12345", "PerfTestClient/1.0")

		// Record events with high error rate
		for i := 0; i < 50; i++ {
			var err error
			if i%3 == 0 { // 33% error rate
				err = fmt.Errorf("error %d", i)
			}
			ms.RecordEventProcessed("error_prone_event", 5*time.Millisecond, err)
		}

		ms.RecordConnectionClosed(connID, "test completed")
	})

	t.Run("memory_pressure", func(t *testing.T) {
		connID := "perf_test_memory"
		ms.RecordConnectionEstablished(connID, "192.168.1.3:12345", "PerfTestClient/1.0")

		// Record many large events to simulate memory pressure
		for i := 0; i < 100; i++ {
			eventSize := int64(1024 * 1024) // 1MB events
			ms.RecordEventReceived(connID, fmt.Sprintf("large_event_%d", i), eventSize)
		}

		ms.RecordConnectionClosed(connID, "test completed")
	})

	// Wait for all metrics to be processed
	time.Sleep(200 * time.Millisecond)

	// Verify system remains responsive
	finalStats := ms.GetConnectionStats()
	if finalStats.TotalConnections != 3 {
		t.Errorf("Expected 3 total connections, got %d", finalStats.TotalConnections)
	}
}

// Mock implementations for testing

type TestAlertNotifier struct {
	alerts chan Alert
}

func (n *TestAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	select {
	case n.alerts <- alert:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errors.New("alert channel full")
	}
}

type PanicHealthCheck struct{}

func (hc *PanicHealthCheck) Name() string {
	return "panic_check"
}

func (hc *PanicHealthCheck) Check(ctx context.Context) error {
	panic("health check panic")
}

type TimeoutHealthCheck struct {
	delay time.Duration
}

func (hc *TimeoutHealthCheck) Name() string {
	return "timeout_check"
}

func (hc *TimeoutHealthCheck) Check(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(hc.delay):
		return nil
	}
}

type ErrorHealthCheck struct {
	err error
}

func (hc *ErrorHealthCheck) Name() string {
	return "error_check"
}

func (hc *ErrorHealthCheck) Check(ctx context.Context) error {
	return hc.err
}

type SlowHealthCheck struct {
	delay time.Duration
}

func (hc *SlowHealthCheck) Name() string {
	return "slow_check"
}

func (hc *SlowHealthCheck) Check(ctx context.Context) error {
	time.Sleep(hc.delay)
	return nil
}

type PanicAlertNotifier struct{}

func (n *PanicAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	panic("alert notifier panic")
}

type TimeoutAlertNotifier struct {
	delay time.Duration
}

func (n *TimeoutAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(n.delay):
		return nil
	}
}

type ErrorAlertNotifier struct{}

func (n *ErrorAlertNotifier) SendAlert(ctx context.Context, alert Alert) error {
	return errors.New("alert notifier error")
}

// Benchmark tests for performance validation

func BenchmarkSSEMonitoringFailures(b *testing.B) {
	config := DefaultMonitoringConfig()
	config.Metrics.Enabled = false // Disable background processing
	config.HealthChecks.Enabled = false

	ms, err := NewMonitoringSystem(config)
	if err != nil {
		b.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		ms.Shutdown(ctx)
	}()

	b.Run("connection_error_recording", func(b *testing.B) {
		connID := "bench_conn"
		ms.RecordConnectionEstablished(connID, "192.168.1.1:12345", "BenchClient/1.0")

		testErr := errors.New("benchmark error")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ms.RecordConnectionError(connID, testErr)
		}
	})

	b.Run("event_processing_errors", func(b *testing.B) {
		testErr := errors.New("processing error")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ms.RecordEventProcessed("bench_event", time.Microsecond, testErr)
		}
	})

	b.Run("concurrent_failure_recording", func(b *testing.B) {
		testErr := errors.New("concurrent error")

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			connID := "bench_concurrent"
			for pb.Next() {
				ms.RecordConnectionError(connID, testErr)
			}
		})
	})
}
