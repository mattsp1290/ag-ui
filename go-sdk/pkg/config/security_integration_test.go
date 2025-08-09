// Package config provides integration tests demonstrating the complete security system
// protecting against DoS attacks and resource exhaustion.
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/config/sources"
)

// TestDoSProtection_FileSizeAttack demonstrates protection against large file attacks
func TestDoSProtection_FileSizeAttack(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create a large configuration file (exceeding the limit)
	largeConfigFile := filepath.Join(tempDir, "large_config.json")
	largeConfig := make(map[string]interface{})
	
	// Create a large string value that would exceed file size limits
	largeValue := strings.Repeat("x", 15*1024*1024) // 15MB
	largeConfig["large_field"] = largeValue
	
	jsonData, err := json.Marshal(largeConfig)
	if err != nil {
		t.Fatalf("failed to marshal large config: %v", err)
	}
	
	err = os.WriteFile(largeConfigFile, jsonData, 0644)
	if err != nil {
		t.Fatalf("failed to write large config file: %v", err)
	}
	
	// Try to load the large file - should be blocked
	fileSource := sources.NewFileSource(largeConfigFile)
	_, err = fileSource.Load(context.Background())
	
	if err == nil {
		t.Error("large file should have been rejected")
	}
	
	if !strings.Contains(err.Error(), "file size") {
		t.Errorf("expected file size error, got: %v", err)
	}
}

// TestDoSProtection_MemoryExhaustionAttack demonstrates protection against memory exhaustion
func TestDoSProtection_MemoryExhaustionAttack(t *testing.T) {
	options := DefaultSecurityOptions()
	options.ResourceLimits.MaxMemoryUsage = 1024 * 1024 // 1MB limit
	options.ResourceLimits.MaxStringLength = 512 * 1024 // 512KB per string
	options.ResourceLimits.UpdateRateLimit = 1 * time.Millisecond // Reduce rate limit for testing
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	// Try to set multiple large values that would exhaust memory
	for i := 0; i < 10; i++ {
		if i > 0 {
			time.Sleep(2 * time.Millisecond) // Avoid rate limiting
		}
		
		largeValue := strings.Repeat("x", 400*1024) // 400KB each
		
		err := config.Set(fmt.Sprintf("large_key_%d", i), largeValue)
		
		if err != nil {
			// Memory limit should eventually be hit
			if strings.Contains(err.Error(), "memory") {
				t.Logf("Memory limit protection triggered at iteration %d: %v", i, err)
				return // Test passed - memory limit was enforced
			}
		}
	}
	
	t.Error("memory exhaustion attack should have been blocked")
}

// TestDoSProtection_DeepNestingAttack demonstrates protection against stack overflow attacks
func TestDoSProtection_DeepNestingAttack(t *testing.T) {
	options := DefaultSecurityOptions()
	options.ResourceLimits.MaxNestingDepth = 10
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	// Create deeply nested structure that would cause stack overflow
	deepNested := make(map[string]interface{})
	current := deepNested
	
	// Create nesting beyond the limit
	for i := 0; i < 20; i++ {
		next := make(map[string]interface{})
		current[fmt.Sprintf("level_%d", i)] = next
		current = next
	}
	current["final_value"] = "deep"
	
	err := config.Set("deeply_nested", deepNested)
	if err == nil {
		t.Error("deeply nested structure should have been rejected")
	}
	
	if !strings.Contains(err.Error(), "nesting depth") {
		t.Errorf("expected nesting depth error, got: %v", err)
	}
}

// TestDoSProtection_WatcherExhaustionAttack demonstrates protection against watcher exhaustion
func TestDoSProtection_WatcherExhaustionAttack(t *testing.T) {
	options := DefaultSecurityOptions()
	options.ResourceLimits.MaxWatchers = 10
	options.ResourceLimits.MaxWatchersPerKey = 3
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	callback := func(value interface{}) {
		// Dummy callback
	}
	
	watcherIDs := make([]CallbackID, 0)
	
	// Try to add many watchers to exhaust resources
	for i := 0; i < 20; i++ {
		id, err := config.Watch(fmt.Sprintf("key_%d", i), callback)
		if err != nil {
			// Watcher limit should be hit
			if strings.Contains(err.Error(), "watcher") {
				t.Logf("Watcher limit protection triggered at iteration %d: %v", i, err)
				break
			}
		} else {
			watcherIDs = append(watcherIDs, id)
		}
		
		if i >= 15 {
			t.Error("watcher exhaustion attack should have been blocked")
			break
		}
	}
	
	// Clean up watchers
	for i, id := range watcherIDs {
		config.UnWatch(fmt.Sprintf("key_%d", i), id)
	}
}

// TestDoSProtection_RapidFireAttacks demonstrates protection against rapid-fire attacks
func TestDoSProtection_RapidFireAttacks(t *testing.T) {
	options := DefaultSecurityOptions()
	options.ResourceLimits.UpdateRateLimit = 100 * time.Millisecond
	options.ResourceLimits.ReloadRateLimit = 500 * time.Millisecond
	options.ResourceLimits.ValidationRateLimit = 200 * time.Millisecond
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	// Rapid fire updates
	updateBlocked := 0
	for i := 0; i < 20; i++ {
		err := config.Set(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
		if err != nil && strings.Contains(err.Error(), "rate") {
			updateBlocked++
		}
		time.Sleep(10 * time.Millisecond) // Faster than rate limit
	}
	
	if updateBlocked == 0 {
		t.Error("rapid fire updates should have been rate limited")
	}
	t.Logf("Rate limiting blocked %d out of 20 rapid fire updates", updateBlocked)
	
	// Rapid fire validations
	validationBlocked := 0
	for i := 0; i < 10; i++ {
		err := config.ValidateSecurely()
		if err != nil && strings.Contains(err.Error(), "rate") {
			validationBlocked++
		}
		time.Sleep(50 * time.Millisecond) // Faster than validation rate limit
	}
	
	t.Logf("Rate limiting blocked %d out of 10 rapid fire validations", validationBlocked)
}

// TestDoSProtection_ConcurrentAttacks demonstrates protection under concurrent load
func TestDoSProtection_ConcurrentAttacks(t *testing.T) {
	options := DefaultSecurityOptions()
	options.ResourceLimits.MaxMemoryUsage = 2 * 1024 * 1024 // 2MB limit
	options.ResourceLimits.MaxWatchers = 50
	options.ResourceLimits.UpdateRateLimit = 50 * time.Millisecond
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	const numGoroutines = 20
	const operationsPerGoroutine = 50
	
	var wg sync.WaitGroup
	var mu sync.Mutex
	var totalBlocked int
	var totalErrors int
	
	wg.Add(numGoroutines)
	
	// Simulate concurrent attackers
	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			localBlocked := 0
			localErrors := 0
			
			for i := 0; i < operationsPerGoroutine; i++ {
				switch i % 4 {
				case 0:
					// Try to set large values
					largeValue := strings.Repeat("x", 50*1024) // 50KB
					err := config.Set(fmt.Sprintf("g%d_large_%d", goroutineID, i), largeValue)
					if err != nil {
						localErrors++
						if IsResourceError(err) {
							localBlocked++
						}
					}
					
				case 1:
					// Try to add watchers
					callback := func(value interface{}) {}
					_, err := config.Watch(fmt.Sprintf("g%d_key_%d", goroutineID, i), callback)
					if err != nil {
						localErrors++
						if IsResourceError(err) {
							localBlocked++
						}
					}
					
				case 2:
					// Try rapid updates
					err := config.Set(fmt.Sprintf("g%d_rapid_%d", goroutineID, i), "value")
					if err != nil {
						localErrors++
						if IsResourceError(err) {
							localBlocked++
						}
					}
					
				case 3:
					// Try validation
					err := config.ValidateSecurely()
					if err != nil {
						localErrors++
						if IsResourceError(err) {
							localBlocked++
						}
					}
				}
			}
			
			mu.Lock()
			totalBlocked += localBlocked
			totalErrors += localErrors
			mu.Unlock()
		}(g)
	}
	
	wg.Wait()
	
	t.Logf("Concurrent attack test: %d operations blocked out of %d total operations", 
		totalBlocked, numGoroutines*operationsPerGoroutine)
	t.Logf("Total errors: %d (includes both resource limits and other errors)", totalErrors)
	
	if totalBlocked == 0 {
		t.Error("some operations should have been blocked under concurrent attack")
	}
	
	// Verify system is still functional after attack
	err := config.Set("post_attack_test", "value")
	if err != nil {
		// May fail due to rate limiting or resource limits, which is OK
		t.Logf("Post-attack operation result: %v", err)
	}
}

// TestMonitoringAndAlerting demonstrates the monitoring and alerting system
func TestMonitoringAndAlerting(t *testing.T) {
	options := DefaultSecurityOptions()
	
	// Set low thresholds to trigger alerts easily
	thresholds := AlertThresholds{
		MaxMemoryUsage:      1024 * 100, // 100KB
		MaxLoadTime:         50 * time.Millisecond,
		MaxValidationTime:   25 * time.Millisecond,
		MaxErrorRate:        5.0, // 5 errors per minute
		MaxRateLimitHitRate: 10.0, // 10 rate limit hits per minute
		WatcherCountAlert:   5,
	}
	options.AlertThresholds = &thresholds
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	if config.metrics == nil {
		t.Skip("monitoring not enabled, skipping test")
	}
	
	// Update alert thresholds
	config.metrics.UpdateAlertThresholds(thresholds)
	
	// Generate activities that should trigger alerts
	
	// 1. Memory usage alert
	largeValue := strings.Repeat("x", 150*1024) // 150KB
	config.Set("large_memory", largeValue)
	
	// 2. Watcher count alert
	callback := func(value interface{}) {}
	for i := 0; i < 7; i++ {
		config.Watch(fmt.Sprintf("alert_key_%d", i), callback)
	}
	
	// 3. Error rate alert (generate errors quickly)
	for i := 0; i < 10; i++ {
		config.Set(fmt.Sprintf("error_key_%d", i), strings.Repeat("x", 200*1024)) // Should fail
		time.Sleep(1 * time.Millisecond)
	}
	
	// 4. Rate limit hits
	for i := 0; i < 15; i++ {
		config.ValidateSecurely()
		time.Sleep(1 * time.Millisecond)
	}
	
	// Give some time for metrics to be collected
	time.Sleep(100 * time.Millisecond)
	
	// Check health status
	health := config.GetHealthStatus()
	t.Logf("Health status: %s", health.Overall)
	
	for checkName, result := range health.Checks {
		t.Logf("Health check %s: %s - %s", checkName, result.Status, result.Message)
	}
	
	// Get alerts
	alerts := config.metrics.GetAlerts(20)
	t.Logf("Generated %d alerts", len(alerts))
	
	alertTypes := make(map[string]int)
	for _, alert := range alerts {
		alertTypes[alert.Type]++
		t.Logf("Alert: %s [%s] - %s", alert.Type, alert.Severity, alert.Message)
	}
	
	// Verify we got different types of alerts
	if len(alertTypes) == 0 {
		t.Error("expected to generate some alerts")
	}
	
	// Check metrics
	metrics := config.GetMetricsSnapshot()
	t.Logf("Metrics summary:")
	t.Logf("  Operations: %d", metrics.OperationCount)
	t.Logf("  Errors: %d", metrics.ErrorCount)
	t.Logf("  Rate limit hits: %d", metrics.RateLimitHits)
	t.Logf("  Resource limit hits: %d", metrics.ResourceLimitHits)
	t.Logf("  Peak memory usage: %d bytes", metrics.PeakMemoryUsage)
	t.Logf("  Peak watcher count: %d", metrics.PeakWatcherCount)
}

// TestGracefulDegradation demonstrates how the system gracefully handles resource limits
func TestGracefulDegradation(t *testing.T) {
	options := DefaultSecurityOptions()
	options.EnableGracefulDegradation = true
	options.ResourceLimits.UpdateRateLimit = 100 * time.Millisecond
	options.ResourceLimits.MaxStringLength = 1024
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	// Test graceful degradation with rate limiting
	successCount := 0
	skipCount := 0
	errorCount := 0
	
	for i := 0; i < 20; i++ {
		err := config.Set(fmt.Sprintf("key_%d", i), "value")
		if err == nil {
			successCount++
		} else if strings.Contains(err.Error(), "handled") || 
		          strings.Contains(err.Error(), "gracefully") {
			skipCount++
		} else {
			errorCount++
		}
		time.Sleep(10 * time.Millisecond) // Faster than rate limit
	}
	
	t.Logf("Graceful degradation results: %d success, %d skipped, %d errors", 
		successCount, skipCount, errorCount)
	
	// With graceful degradation, we should have some operations succeed,
	// some skipped, and minimal hard errors
	if successCount == 0 {
		t.Error("some operations should succeed with graceful degradation")
	}
	
	// Test that the system is still responsive after degradation
	time.Sleep(200 * time.Millisecond) // Wait for rate limit to clear
	err := config.Set("post_degradation", "test")
	if err != nil {
		t.Errorf("system should be responsive after graceful degradation: %v", err)
	}
}

// TestSecurityViolationDetection demonstrates security violation detection
func TestSecurityViolationDetection(t *testing.T) {
	config := NewSecureConfig(DefaultSecurityOptions())
	defer config.Shutdown()
	
	if config.metrics == nil {
		t.Skip("monitoring not enabled, skipping test")
	}
	
	// Simulate a security violation by directly calling the metrics
	config.metrics.RecordSecurityViolation()
	
	// Check that it was recorded
	metrics := config.GetMetricsSnapshot()
	if metrics.SecurityViolations != 1 {
		t.Errorf("expected 1 security violation, got %d", metrics.SecurityViolations)
	}
	
	// Check health status - should be critical
	health := config.GetHealthStatus()
	if health.Overall != "critical" {
		t.Errorf("expected critical health status after security violation, got %s", health.Overall)
	}
	
	// Check alerts
	alerts := config.metrics.GetAlerts(5)
	found := false
	for _, alert := range alerts {
		if alert.Type == "security_violation" && alert.Severity == "critical" {
			found = true
			break
		}
	}
	if !found {
		t.Error("security violation should trigger critical alert")
	}
}

// TestPerformanceUnderLimits demonstrates that normal operations perform well within limits
func TestPerformanceUnderLimits(t *testing.T) {
	options := DefaultSecurityOptions()
	// Reduce rate limits for performance testing
	options.ResourceLimits.UpdateRateLimit = 1 * time.Millisecond
	options.ResourceLimits.ValidationRateLimit = 1 * time.Millisecond
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	start := time.Now()
	
	// Perform normal operations within limits
	for i := 0; i < 100; i++ {
		if i > 0 {
			time.Sleep(2 * time.Millisecond) // Avoid rate limiting
		}
		
		config.Set(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
		
		if i%10 == 0 {
			config.ValidateSecurely()
		}
		
		if i%20 == 0 {
			callback := func(value interface{}) {}
			id, err := config.Watch(fmt.Sprintf("watch_key_%d", i/20), callback)
			if err == nil {
				defer config.UnWatch(fmt.Sprintf("watch_key_%d", i/20), id)
			}
		}
	}
	
	elapsed := time.Since(start)
	t.Logf("100 operations completed in %v", elapsed)
	
	// Operations within limits should complete quickly
	if elapsed > 5*time.Second {
		t.Errorf("operations took too long: %v", elapsed)
	}
	
	// Check metrics
	if config.metrics != nil {
		metrics := config.GetMetricsSnapshot()
		t.Logf("Performance metrics:")
		t.Logf("  Operations per second: %.2f", metrics.OperationsPerSecond)
		t.Logf("  Average load time: %v", metrics.AverageLoadTime)
		t.Logf("  Error rate: %.2f", metrics.ErrorRate)
		
		// Error rate should be low for normal operations
		if metrics.ErrorRate > 1.0 {
			t.Errorf("error rate too high for normal operations: %.2f", metrics.ErrorRate)
		}
	}
}

// TestResourceCleanup demonstrates proper resource cleanup
func TestResourceCleanup(t *testing.T) {
	options := DefaultSecurityOptions()
	config := NewSecureConfig(options)
	
	// Create some resources
	config.Set("test1", "value1")
	config.Set("test2", strings.Repeat("x", 1024)) // 1KB value
	
	callback := func(value interface{}) {}
	id1, _ := config.Watch("key1", callback)
	id2, _ := config.Watch("key2", callback)
	
	// Check initial resource usage
	if config.resourceManager != nil {
		initialStats := config.GetResourceStats()
		if initialStats.CurrentWatchers == 0 {
			t.Error("should have some watchers before cleanup")
		}
	}
	
	// Manually remove some watchers
	config.UnWatch("key1", id1)
	
	// Shutdown should clean up remaining resources
	config.Shutdown()
	
	// Verify cleanup
	if !config.IsShutdown() {
		t.Error("config should be marked as shutdown")
	}
	
	// Subsequent operations should fail gracefully
	err := config.Set("after_shutdown", "value")
	if err == nil {
		t.Error("operations should fail after shutdown")
	}
	
	// Adding watchers should fail
	_, err = config.Watch("new_key", callback)
	if err == nil {
		t.Error("adding watchers should fail after shutdown")
	}
	
	// But removing watchers should still work (for cleanup)
	config.UnWatch("key2", id2) // Should not crash
}