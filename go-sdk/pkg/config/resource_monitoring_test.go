// Package config provides tests for resource monitoring and metrics
package config

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMetricsCollector_BasicOperations(t *testing.T) {
	mc := NewMetricsCollector()
	
	// Test recording operations
	mc.RecordOperation()
	mc.RecordOperation()
	mc.RecordError()
	
	stats := mc.GetMetrics()
	if stats.OperationCount != 2 {
		t.Errorf("expected 2 operations, got %d", stats.OperationCount)
	}
	if stats.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", stats.ErrorCount)
	}
}

func TestMetricsCollector_LoadOperations(t *testing.T) {
	mc := NewMetricsCollector()
	
	// Record some load operations
	mc.RecordLoad(100 * time.Millisecond)
	mc.RecordLoad(200 * time.Millisecond)
	mc.RecordLoad(50 * time.Millisecond)
	
	stats := mc.GetMetrics()
	if stats.LoadCount != 3 {
		t.Errorf("expected 3 loads, got %d", stats.LoadCount)
	}
	
	expectedAvg := (100 + 200 + 50) / 3 * time.Millisecond
	tolerance := time.Millisecond
	if stats.AverageLoadTime < expectedAvg-tolerance || stats.AverageLoadTime > expectedAvg+tolerance {
		t.Errorf("expected average load time around %v, got %v", expectedAvg, stats.AverageLoadTime)
	}
}

func TestMetricsCollector_ValidationOperations(t *testing.T) {
	mc := NewMetricsCollector()
	
	// Record some validation operations
	mc.RecordValidation(50 * time.Millisecond)
	mc.RecordValidation(100 * time.Millisecond)
	
	stats := mc.GetMetrics()
	if stats.ValidationCount != 2 {
		t.Errorf("expected 2 validations, got %d", stats.ValidationCount)
	}
	
	expectedAvg := 75 * time.Millisecond
	if stats.AverageValidationTime != expectedAvg {
		t.Errorf("expected average validation time %v, got %v", expectedAvg, stats.AverageValidationTime)
	}
}

func TestMetricsCollector_WatcherTracking(t *testing.T) {
	mc := NewMetricsCollector()
	
	// Add watchers
	mc.RecordWatcherAdded()
	mc.RecordWatcherAdded()
	mc.RecordWatcherAdded()
	
	stats := mc.GetMetrics()
	if stats.WatcherCount != 3 {
		t.Errorf("expected 3 watchers, got %d", stats.WatcherCount)
	}
	if stats.PeakWatcherCount != 3 {
		t.Errorf("expected peak watcher count 3, got %d", stats.PeakWatcherCount)
	}
	
	// Remove some watchers
	mc.RecordWatcherRemoved()
	mc.RecordWatcherRemoved()
	
	stats = mc.GetMetrics()
	if stats.WatcherCount != 1 {
		t.Errorf("expected 1 watcher after removal, got %d", stats.WatcherCount)
	}
	if stats.PeakWatcherCount != 3 {
		t.Errorf("peak watcher count should remain 3, got %d", stats.PeakWatcherCount)
	}
}

func TestMetricsCollector_MemoryTracking(t *testing.T) {
	mc := NewMetricsCollector()
	
	// Record memory usage
	mc.RecordMemoryUsage(1024)
	mc.RecordMemoryUsage(2048)
	mc.RecordMemoryUsage(1536)
	
	stats := mc.GetMetrics()
	if stats.PeakMemoryUsage != 2048 {
		t.Errorf("expected peak memory usage 2048, got %d", stats.PeakMemoryUsage)
	}
}

func TestMetricsCollector_ErrorTracking(t *testing.T) {
	mc := NewMetricsCollector()
	
	// Record different types of errors
	mc.RecordRateLimitHit()
	mc.RecordRateLimitHit()
	mc.RecordResourceLimitHit()
	mc.RecordStructureLimitHit()
	mc.RecordTimeoutHit()
	mc.RecordSecurityViolation()
	
	stats := mc.GetMetrics()
	if stats.RateLimitHits != 2 {
		t.Errorf("expected 2 rate limit hits, got %d", stats.RateLimitHits)
	}
	if stats.ResourceLimitHits != 1 {
		t.Errorf("expected 1 resource limit hit, got %d", stats.ResourceLimitHits)
	}
	if stats.StructureLimitHits != 1 {
		t.Errorf("expected 1 structure limit hit, got %d", stats.StructureLimitHits)
	}
	if stats.TimeoutHits != 1 {
		t.Errorf("expected 1 timeout hit, got %d", stats.TimeoutHits)
	}
	if stats.SecurityViolations != 1 {
		t.Errorf("expected 1 security violation, got %d", stats.SecurityViolations)
	}
	
	// Check that security violation triggered an alert
	alerts := mc.GetAlerts(10)
	found := false
	for _, alert := range alerts {
		if alert.Type == "security_violation" && alert.Severity == "critical" {
			found = true
			break
		}
	}
	if !found {
		t.Error("security violation should have triggered a critical alert")
	}
}

func TestMetricsCollector_AlertThresholds(t *testing.T) {
	mc := NewMetricsCollector()
	
	// Set low thresholds for testing
	thresholds := AlertThresholds{
		MaxMemoryUsage:    1024,
		MaxLoadTime:       100 * time.Millisecond,
		MaxValidationTime: 50 * time.Millisecond,
		WatcherCountAlert: 2,
	}
	mc.UpdateAlertThresholds(thresholds)
	
	// Trigger memory alert
	mc.RecordMemoryUsage(2048) // Exceeds 1024 threshold
	
	// Trigger load time alert
	mc.RecordLoad(200 * time.Millisecond) // Exceeds 100ms threshold
	
	// Trigger validation time alert
	mc.RecordValidation(100 * time.Millisecond) // Exceeds 50ms threshold
	
	// Trigger watcher count alert
	mc.RecordWatcherAdded()
	mc.RecordWatcherAdded()
	mc.RecordWatcherAdded() // Exceeds threshold of 2
	
	alerts := mc.GetAlerts(10)
	if len(alerts) < 4 {
		t.Errorf("expected at least 4 alerts, got %d", len(alerts))
	}
	
	// Verify alert types
	alertTypes := make(map[string]bool)
	for _, alert := range alerts {
		alertTypes[alert.Type] = true
	}
	
	expectedTypes := []string{"memory_usage", "load_time", "validation_time", "watcher_count"}
	for _, expectedType := range expectedTypes {
		if !alertTypes[expectedType] {
			t.Errorf("expected alert type %s not found", expectedType)
		}
	}
}

func TestMetricsCollector_HealthCheck(t *testing.T) {
	mc := NewMetricsCollector()
	
	// Initially should be healthy
	health := mc.HealthCheck()
	if health.Overall != "healthy" {
		t.Errorf("expected healthy status, got %s", health.Overall)
	}
	
	// Set low thresholds and trigger violations
	thresholds := AlertThresholds{
		MaxErrorRate:      1.0, // 1 error per minute
		MaxMemoryUsage:    1024,
		WatcherCountAlert: 1,
	}
	mc.UpdateAlertThresholds(thresholds)
	
	// Wait a bit to ensure we have some time window for rate calculation
	time.Sleep(10 * time.Millisecond)
	
	// Trigger multiple errors to exceed error rate
	for i := 0; i < 5; i++ {
		mc.RecordError()
	}
	
	// Trigger memory alert
	mc.RecordMemoryUsage(2048)
	
	// Trigger security violation (should make status critical)
	mc.RecordSecurityViolation()
	
	health = mc.HealthCheck()
	if health.Overall != "critical" {
		t.Errorf("expected critical status due to security violation, got %s", health.Overall)
	}
	
	// Check individual checks
	if health.Checks["security"].Status != "critical" {
		t.Errorf("expected security check to be critical, got %s", health.Checks["security"].Status)
	}
}

func TestMetricsCollector_Reset(t *testing.T) {
	mc := NewMetricsCollector()
	
	// Generate some metrics
	mc.RecordOperation()
	mc.RecordError()
	mc.RecordLoad(100 * time.Millisecond)
	mc.RecordRateLimitHit()
	mc.RecordMemoryUsage(1024) // This should be preserved as peak
	
	statsBefore := mc.GetMetrics()
	if statsBefore.OperationCount == 0 {
		t.Error("should have some operations before reset")
	}
	
	// Reset metrics
	mc.Reset()
	
	statsAfter := mc.GetMetrics()
	
	// Counters should be reset
	if statsAfter.OperationCount != 0 {
		t.Errorf("operation count should be 0 after reset, got %d", statsAfter.OperationCount)
	}
	if statsAfter.ErrorCount != 0 {
		t.Errorf("error count should be 0 after reset, got %d", statsAfter.ErrorCount)
	}
	if statsAfter.LoadCount != 0 {
		t.Errorf("load count should be 0 after reset, got %d", statsAfter.LoadCount)
	}
	if statsAfter.RateLimitHits != 0 {
		t.Errorf("rate limit hits should be 0 after reset, got %d", statsAfter.RateLimitHits)
	}
	
	// Peak values should be preserved
	if statsAfter.PeakMemoryUsage != statsBefore.PeakMemoryUsage {
		t.Errorf("peak memory usage should be preserved, expected %d, got %d", 
			statsBefore.PeakMemoryUsage, statsAfter.PeakMemoryUsage)
	}
	
	// Time since reset should be recent
	if statsAfter.TimeSinceReset > time.Second {
		t.Errorf("time since reset should be recent, got %v", statsAfter.TimeSinceReset)
	}
}

func TestMetricsCollector_JSONSerialization(t *testing.T) {
	mc := NewMetricsCollector()
	
	// Generate some data
	mc.RecordOperation()
	mc.RecordLoad(100 * time.Millisecond)
	mc.RecordMemoryUsage(1024)
	mc.RecordWatcherAdded()
	
	snapshot := mc.GetMetrics()
	
	// Test JSON serialization
	jsonData, err := snapshot.ToJSON()
	if err != nil {
		t.Errorf("failed to serialize metrics to JSON: %v", err)
	}
	
	// Test that it can be unmarshaled
	var unmarshaled map[string]interface{}
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Errorf("failed to unmarshal JSON: %v", err)
	}
	
	// Check some key fields
	if unmarshaled["operation_count"].(float64) != float64(snapshot.OperationCount) {
		t.Error("operation count not properly serialized")
	}
	if unmarshaled["peak_memory_usage"].(float64) != float64(snapshot.PeakMemoryUsage) {
		t.Error("peak memory usage not properly serialized")
	}
}

func TestMonitoringContext(t *testing.T) {
	mc := NewMetricsCollector()
	
	// Test successful operation
	ctx := NewMonitoringContext(mc, "test_operation")
	time.Sleep(10 * time.Millisecond) // Simulate some work
	ctx.Complete(nil)
	
	stats := mc.GetMetrics()
	if stats.OperationCount != 1 {
		t.Errorf("expected 1 operation, got %d", stats.OperationCount)
	}
	if stats.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", stats.ErrorCount)
	}
	
	// Test operation with error
	ctx = NewMonitoringContext(mc, "test_operation")
	resourceErr := NewResourceLimitError("memory", 2048, 1024, "memory exceeded")
	ctx.Complete(resourceErr)
	
	stats = mc.GetMetrics()
	if stats.OperationCount != 2 {
		t.Errorf("expected 2 operations, got %d", stats.OperationCount)
	}
	if stats.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", stats.ErrorCount)
	}
	if stats.ResourceLimitHits != 1 {
		t.Errorf("expected 1 resource limit hit, got %d", stats.ResourceLimitHits)
	}
}

func TestMetricsCollector_WithMonitoring(t *testing.T) {
	mc := NewMetricsCollector()
	
	// Test successful operation
	err := mc.WithMonitoring("load", func() error {
		time.Sleep(10 * time.Millisecond) // Simulate work
		return nil
	})
	
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	stats := mc.GetMetrics()
	if stats.LoadCount != 1 {
		t.Errorf("expected 1 load operation, got %d", stats.LoadCount)
	}
	if stats.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", stats.ErrorCount)
	}
	
	// Test operation with error
	testError := NewRateLimitError("test", time.Second, 500*time.Millisecond, "rate limited")
	err = mc.WithMonitoring("validate", func() error {
		return testError
	})
	
	if err != testError {
		t.Errorf("expected original error to be returned, got %v", err)
	}
	
	stats = mc.GetMetrics()
	if stats.ValidationCount != 1 {
		t.Errorf("expected 1 validation operation, got %d", stats.ValidationCount)
	}
	if stats.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", stats.ErrorCount)
	}
	if stats.RateLimitHits != 1 {
		t.Errorf("expected 1 rate limit hit, got %d", stats.RateLimitHits)
	}
}

func TestHistogram_Functionality(t *testing.T) {
	mc := NewMetricsCollector()
	
	// Record load operations with different durations to test histogram
	durations := []time.Duration{
		1 * time.Millisecond,
		10 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
		5 * time.Second,
	}
	
	for _, duration := range durations {
		mc.RecordLoad(duration)
	}
	
	stats := mc.GetMetrics()
	loadHistogram := stats.Histograms["load_time"]
	if loadHistogram == nil {
		t.Fatal("load_time histogram should exist")
	}
	
	if loadHistogram.Count != int64(len(durations)) {
		t.Errorf("expected histogram count %d, got %d", len(durations), loadHistogram.Count)
	}
	
	// Check that buckets have appropriate counts
	totalBucketCount := int64(0)
	for _, bucket := range loadHistogram.Buckets {
		totalBucketCount += bucket.Count
		if bucket.Count < 0 {
			t.Errorf("bucket count should not be negative: %d", bucket.Count)
		}
	}
	
	if totalBucketCount != int64(len(durations)) {
		t.Errorf("total bucket count %d should equal number of samples %d", totalBucketCount, len(durations))
	}
	
	// Check min/max values
	expectedMin := int64(1 * time.Millisecond)
	expectedMax := int64(5 * time.Second)
	if loadHistogram.Min != expectedMin {
		t.Errorf("expected min %d, got %d", expectedMin, loadHistogram.Min)
	}
	if loadHistogram.Max != expectedMax {
		t.Errorf("expected max %d, got %d", expectedMax, loadHistogram.Max)
	}
}

func TestAlertHistory(t *testing.T) {
	mc := NewMetricsCollector()
	
	// Set very low thresholds to trigger alerts easily
	thresholds := AlertThresholds{
		MaxMemoryUsage: 100,
		MaxLoadTime:    10 * time.Millisecond,
	}
	mc.UpdateAlertThresholds(thresholds)
	
	// Trigger several alerts
	mc.RecordMemoryUsage(200)  // Should trigger memory alert
	mc.RecordLoad(50 * time.Millisecond)  // Should trigger load time alert
	mc.RecordSecurityViolation()  // Should trigger security alert
	
	// Get alerts
	alerts := mc.GetAlerts(10)
	if len(alerts) < 3 {
		t.Errorf("expected at least 3 alerts, got %d", len(alerts))
	}
	
	// Check alert structure
	for _, alert := range alerts {
		if alert.Timestamp.IsZero() {
			t.Error("alert timestamp should not be zero")
		}
		if alert.Severity == "" {
			t.Error("alert severity should not be empty")
		}
		if alert.Type == "" {
			t.Error("alert type should not be empty")
		}
		if alert.Message == "" {
			t.Error("alert message should not be empty")
		}
		if alert.Resolved {
			t.Error("new alerts should not be marked as resolved")
		}
	}
	
	// Test alert limiting
	limitedAlerts := mc.GetAlerts(2)
	if len(limitedAlerts) != 2 {
		t.Errorf("expected 2 alerts with limit, got %d", len(limitedAlerts))
	}
}