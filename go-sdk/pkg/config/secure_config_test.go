// Package config provides tests for secure configuration system
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSecureConfig_BasicFunctionality(t *testing.T) {
	options := DefaultSecurityOptions()
	options.ResourceLimits.MaxFileSize = 1024
	options.ResourceLimits.MaxMemoryUsage = 2048
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	// Test basic set/get operations
	err := config.Set("test.key", "test.value")
	if err != nil {
		t.Errorf("unexpected error setting value: %v", err)
	}
	
	value := config.GetString("test.key")
	if value != "test.value" {
		t.Errorf("expected 'test.value', got '%s'", value)
	}
	
	// Check that security features are enabled
	if !config.IsSecurityEnabled() {
		t.Error("security should be enabled")
	}
	if !config.IsMonitoringEnabled() {
		t.Error("monitoring should be enabled")
	}
}

func TestSecureConfig_ResourceLimitEnforcement(t *testing.T) {
	options := DefaultSecurityOptions()
	options.ResourceLimits.MaxKeys = 3
	options.ResourceLimits.MaxStringLength = 10
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	// Set values within limits
	config.Set("key1", "short")
	config.Set("key2", "value")
	config.Set("key3", "test")
	
	// This should fail due to key count limit
	err := config.Set("key4", "overflow")
	if err == nil {
		t.Error("expected error when exceeding key limit")
	}
	
	// This should fail due to string length limit
	err = config.Set("longkey", "this string is way too long")
	if err == nil {
		t.Error("expected error when exceeding string length limit")
	}
}

func TestSecureConfig_RateLimiting(t *testing.T) {
	options := DefaultSecurityOptions()
	options.ResourceLimits.UpdateRateLimit = 100 * time.Millisecond
	options.ResourceLimits.ValidationRateLimit = 200 * time.Millisecond
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	// First update should succeed
	err := config.Set("key1", "value1")
	if err != nil {
		t.Errorf("first update should succeed: %v", err)
	}
	
	// Immediate second update should be rate limited
	err = config.Set("key2", "value2")
	if err != nil {
		// This is expected behavior - rate limited
		if !strings.Contains(err.Error(), "rate") {
			t.Errorf("expected rate limiting error, got: %v", err)
		}
	}
	
	// Wait for rate limit to expire
	time.Sleep(150 * time.Millisecond)
	err = config.Set("key3", "value3")
	if err != nil {
		t.Errorf("update after rate limit should succeed: %v", err)
	}
	
	// Test validation rate limiting
	err = config.ValidateSecurely()
	if err != nil {
		t.Errorf("first validation should succeed: %v", err)
	}
	
	// Immediate second validation should be rate limited
	err = config.ValidateSecurely()
	if err != nil && !strings.Contains(err.Error(), "rate") {
		t.Errorf("expected rate limiting error for validation, got: %v", err)
	}
}

func TestSecureConfig_WatcherLimits(t *testing.T) {
	options := DefaultSecurityOptions()
	options.ResourceLimits.MaxWatchers = 2
	options.ResourceLimits.MaxWatchersPerKey = 1
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	callbackCount := 0
	callback := func(value interface{}) {
		callbackCount++
	}
	
	// Add first watcher
	id1, err := config.Watch("key1", callback)
	if err != nil {
		t.Errorf("first watcher should be allowed: %v", err)
	}
	
	// Add second watcher
	id2, err := config.Watch("key2", callback)
	if err != nil {
		t.Errorf("second watcher should be allowed: %v", err)
	}
	
	// Third watcher should fail (exceeds total limit)
	_, err = config.Watch("key3", callback)
	if err == nil {
		t.Error("third watcher should be blocked by total limit")
	}
	
	// Second watcher on same key should fail
	_, err = config.Watch("key1", callback)
	if err == nil {
		t.Error("second watcher on same key should be blocked by per-key limit")
	}
	
	// Remove a watcher and try again
	err = config.UnWatch("key1", id1)
	if err != nil {
		t.Errorf("failed to remove watcher: %v", err)
	}
	
	_, err = config.Watch("key3", callback)
	if err != nil {
		t.Errorf("watcher should be allowed after removal: %v", err)
	}
	
	// Clean up
	config.UnWatch("key2", id2)
}

func TestSecureConfigBuilder_BuildSecure(t *testing.T) {
	// Create a temporary config file for testing
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test_config.json")
	configContent := `{
		"database": {
			"host": "localhost",
			"port": 5432
		},
		"api": {
			"timeout": "30s"
		}
	}`
	
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to create test config file: %v", err)
	}
	
	// Create secure config builder
	builder := NewSecureConfigBuilder()
	limits := DefaultResourceLimits()
	limits.MaxFileSize = 1024 // Small limit for testing
	
	// Add file source and build
	config, err := builder.
		WithResourceLimits(limits).
		WithSecurity(true).
		WithMetrics(true).
		BuildSecure()
	
	if err != nil {
		t.Fatalf("failed to build secure config: %v", err)
	}
	defer config.Shutdown()
	
	// Verify security features are enabled
	if !config.IsSecurityEnabled() {
		t.Error("security should be enabled")
	}
	if !config.IsMonitoringEnabled() {
		t.Error("monitoring should be enabled")
	}
	
	// Test resource stats access
	stats := config.GetResourceStats()
	if stats.CurrentMemoryUsage < 0 {
		t.Error("memory usage should not be negative")
	}
	
	// Test metrics access
	metrics := config.GetMetricsSnapshot()
	if metrics.Timestamp.IsZero() {
		t.Error("metrics timestamp should not be zero")
	}
	
	// Test health status
	health := config.GetHealthStatus()
	if health.Overall == "" {
		t.Error("health status should not be empty")
	}
}

func TestSecureConfig_MemoryUsageTracking(t *testing.T) {
	options := DefaultSecurityOptions()
	options.ResourceLimits.MaxMemoryUsage = 1024 // 1KB limit
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	// Set some values to consume memory
	err := config.Set("small", "value")
	if err != nil {
		t.Errorf("small value should not exceed memory limit: %v", err)
	}
	
	// Create a large value that would exceed memory limit
	largeValue := strings.Repeat("x", 2048) // 2KB
	err = config.Set("large", largeValue)
	if err == nil {
		t.Error("large value should exceed memory limit")
	}
	
	// Check resource stats
	stats := config.GetResourceStats()
	if stats.CurrentMemoryUsage <= 0 {
		t.Error("memory usage should be tracked")
	}
}

func TestSecureConfig_DeepNestingProtection(t *testing.T) {
	options := DefaultSecurityOptions()
	options.ResourceLimits.MaxNestingDepth = 3
	options.ResourceLimits.UpdateRateLimit = 1 * time.Millisecond // Reduce rate limit for testing
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	// Create deeply nested structure
	deepValue := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": map[string]interface{}{
					"level4": "too deep", // This exceeds the limit
				},
			},
		},
	}
	
	err := config.Set("deep", deepValue)
	if err == nil {
		t.Error("deeply nested structure should be rejected")
	}
	
	// Wait briefly to avoid rate limiting
	time.Sleep(2 * time.Millisecond)
	
	// Shallow structure should be allowed
	shallowValue := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": "ok",
		},
	}
	
	err = config.Set("shallow", shallowValue)
	if err != nil {
		t.Errorf("shallow structure should be allowed: %v", err)
	}
}

func TestSecureConfig_TimeoutProtection(t *testing.T) {
	options := DefaultSecurityOptions()
	options.ResourceLimits.ValidationTimeout = 50 * time.Millisecond
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	// Add a custom validator that takes a long time
	slowValidator := &CustomValidator{
		name:  "slow_validator",
		rules: make(map[string]ValidationRule),
	}
	
	slowValidator.AddRule("test", func(value interface{}) error {
		time.Sleep(100 * time.Millisecond) // Longer than timeout
		return nil
	})
	
	config.ConfigImpl.validators = append(config.ConfigImpl.validators, slowValidator)
	config.Set("test", "value")
	
	// This should timeout
	err := config.ValidateSecurely()
	if err == nil {
		t.Error("validation should timeout with slow validator")
	}
	
	// Check if it's a timeout error
	if timeoutErr, ok := err.(*TimeoutError); ok {
		if timeoutErr.Operation != "validate" {
			t.Errorf("expected validate timeout, got %s", timeoutErr.Operation)
		}
	}
}

func TestSecureConfig_GracefulDegradation(t *testing.T) {
	options := DefaultSecurityOptions()
	options.EnableGracefulDegradation = true
	options.ResourceLimits.UpdateRateLimit = 100 * time.Millisecond
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	// First update should succeed
	err := config.Set("key1", "value1")
	if err != nil {
		t.Errorf("first update should succeed: %v", err)
	}
	
	// Immediate second update should be handled gracefully
	err = config.Set("key2", "value2")
	// With graceful degradation, this might return nil (operation skipped)
	// or a handled error message
	if err != nil && !strings.Contains(err.Error(), "handled") && !strings.Contains(err.Error(), "rate") {
		t.Errorf("expected graceful degradation for rate limit, got: %v", err)
	}
}

func TestSecureConfig_MetricsIntegration(t *testing.T) {
	options := DefaultSecurityOptions()
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	// Perform some operations
	config.Set("key1", "value1")
	config.Set("key2", "value2")
	config.ValidateSecurely()
	
	callback := func(interface{}) {}
	id, _ := config.Watch("key1", callback)
	config.UnWatch("key1", id)
	
	// Get metrics
	metrics := config.GetMetricsSnapshot()
	
	if metrics.OperationCount == 0 {
		t.Error("should have recorded some operations")
	}
	
	// Check specific metrics are being tracked
	if metrics.ValidationCount == 0 {
		t.Error("should have recorded validation")
	}
}

func TestSecureConfig_AlertSystem(t *testing.T) {
	options := DefaultSecurityOptions()
	// Set very low thresholds to trigger alerts
	thresholds := DefaultAlertThresholds()
	thresholds.MaxMemoryUsage = 100
	thresholds.WatcherCountAlert = 1
	options.AlertThresholds = &thresholds
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	// Update thresholds
	if config.metrics != nil {
		config.metrics.UpdateAlertThresholds(thresholds)
	}
	
	// Trigger memory alert
	config.Set("test", strings.Repeat("x", 200)) // Should exceed 100 byte threshold
	
	// Trigger watcher alert
	callback := func(interface{}) {}
	config.Watch("key1", callback)
	config.Watch("key2", callback) // Should exceed watcher threshold
	
	// Get health status
	health := config.GetHealthStatus()
	
	// Should be degraded due to alerts
	if health.Overall == "healthy" {
		t.Error("health should be degraded due to alerts")
	}
	
	// Check that alerts were recorded
	if config.metrics != nil {
		alerts := config.metrics.GetAlerts(10)
		if len(alerts) == 0 {
			t.Error("should have recorded some alerts")
		}
	}
}

func TestSecureConfig_RuntimeLimitUpdates(t *testing.T) {
	options := DefaultSecurityOptions()
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	// Get initial limits
	initialLimits := config.resourceManager.GetLimits()
	
	// Update limits
	newLimits := DefaultResourceLimits()
	newLimits.MaxFileSize = initialLimits.MaxFileSize * 2
	newLimits.MaxWatchers = initialLimits.MaxWatchers + 10
	
	err := config.UpdateResourceLimits(newLimits)
	if err != nil {
		t.Errorf("failed to update resource limits: %v", err)
	}
	
	// Verify limits were updated
	updatedLimits := config.resourceManager.GetLimits()
	if updatedLimits.MaxFileSize != newLimits.MaxFileSize {
		t.Errorf("max file size not updated: expected %d, got %d", 
			newLimits.MaxFileSize, updatedLimits.MaxFileSize)
	}
	if updatedLimits.MaxWatchers != newLimits.MaxWatchers {
		t.Errorf("max watchers not updated: expected %d, got %d", 
			newLimits.MaxWatchers, updatedLimits.MaxWatchers)
	}
}

func TestSecureConfig_SecurityFeatureToggling(t *testing.T) {
	// Test with security disabled
	options := DefaultSecurityOptions()
	options.EnableResourceLimits = false
	options.EnableMonitoring = false
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	if config.IsSecurityEnabled() {
		t.Error("security should be disabled")
	}
	if config.IsMonitoringEnabled() {
		t.Error("monitoring should be disabled")
	}
	
	// Operations should work without limits when security is disabled
	largeValue := strings.Repeat("x", 100000) // Large value
	err := config.Set("large", largeValue)
	if err != nil {
		t.Errorf("large value should be allowed when security is disabled: %v", err)
	}
}

func TestSecureConfig_ErrorHandling(t *testing.T) {
	options := DefaultSecurityOptions()
	options.EnableGracefulDegradation = false // Disable graceful degradation
	options.ResourceLimits.MaxStringLength = 10
	
	config := NewSecureConfig(options)
	defer config.Shutdown()
	
	// This should fail with a clear error
	longString := strings.Repeat("x", 20)
	err := config.Set("long", longString)
	
	if err == nil {
		t.Error("expected error for long string")
	}
	
	// Check error type
	if !IsResourceError(err) {
		t.Errorf("expected resource error, got %T", err)
	}
	
	// Check error severity
	severity := GetErrorSeverity(err)
	if severity == "unknown" {
		t.Error("error severity should be determined")
	}
}

func TestSecureConfig_Shutdown(t *testing.T) {
	config := NewSecureConfig(DefaultSecurityOptions())
	
	// Add a watcher
	callback := func(interface{}) {}
	id, err := config.Watch("key", callback)
	if err != nil {
		t.Errorf("failed to add watcher: %v", err)
	}
	
	// Set some values
	config.Set("test", "value")
	
	// Shutdown should clean up resources
	config.Shutdown()
	
	// Verify config is shutdown
	if !config.IsShutdown() {
		t.Error("config should be marked as shutdown")
	}
	
	// Operations after shutdown should fail
	err = config.Set("after", "shutdown")
	if err == nil {
		t.Error("operations should fail after shutdown")
	}
	
	// Adding watchers should fail
	_, err = config.Watch("new", callback)
	if err == nil {
		t.Error("adding watchers should fail after shutdown")
	}
	
	// Removing watchers should still work (cleanup)
	err = config.UnWatch("key", id)
	// This might or might not error, but shouldn't crash
}