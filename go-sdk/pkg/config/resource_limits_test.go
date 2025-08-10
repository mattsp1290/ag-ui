// Package config provides tests for resource limits and security features
package config

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestResourceManager_ValidateFileSize(t *testing.T) {
	limits := DefaultResourceLimits()
	limits.MaxFileSize = 1024 // 1KB for testing
	
	rm := NewResourceManager(limits)
	
	tests := []struct {
		name        string
		size        int64
		expectError bool
	}{
		{"small file", 512, false},
		{"max size file", 1024, false},
		{"oversized file", 1025, true},
		{"very large file", 10 * 1024 * 1024, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rm.ValidateFileSize(tt.size)
			if tt.expectError && err == nil {
				t.Errorf("expected error for size %d, got nil", tt.size)
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error for size %d: %v", tt.size, err)
			}
			
			if tt.expectError && err != nil {
				if _, ok := err.(*ResourceLimitError); !ok {
					t.Errorf("expected ResourceLimitError, got %T", err)
				}
			}
		})
	}
}

func TestResourceManager_ValidateMemoryUsage(t *testing.T) {
	limits := DefaultResourceLimits()
	limits.MaxMemoryUsage = 1024 // 1KB for testing
	
	rm := NewResourceManager(limits)
	
	// Set current memory usage
	rm.UpdateMemoryUsage(512)
	
	tests := []struct {
		name        string
		additional  int64
		expectError bool
	}{
		{"small addition", 100, false},
		{"max addition", 512, false},
		{"over limit addition", 513, true},
		{"large addition", 1024, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rm.ValidateMemoryUsage(tt.additional)
			if tt.expectError && err == nil {
				t.Errorf("expected error for additional memory %d, got nil", tt.additional)
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error for additional memory %d: %v", tt.additional, err)
			}
		})
	}
}

func TestResourceManager_ValidateConfigStructure(t *testing.T) {
	limits := DefaultResourceLimits()
	limits.MaxNestingDepth = 3
	limits.MaxKeys = 5
	limits.MaxArraySize = 3
	limits.MaxStringLength = 10
	
	rm := NewResourceManager(limits)
	
	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		errorType   string
	}{
		{
			name: "simple valid config",
			config: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
			expectError: false,
		},
		{
			name: "too many keys",
			config: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
				"key4": "value4",
				"key5": "value5",
				"key6": "value6", // exceeds limit of 5
			},
			expectError: true,
			errorType:   "key_count",
		},
		{
			name: "too deep nesting",
			config: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": map[string]interface{}{
							"level4": "too deep", // exceeds depth of 3
						},
					},
				},
			},
			expectError: true,
			errorType:   "nesting_depth",
		},
		{
			name: "array too large",
			config: map[string]interface{}{
				"array": []interface{}{1, 2, 3, 4}, // exceeds limit of 3
			},
			expectError: true,
			errorType:   "array_size",
		},
		{
			name: "string too long",
			config: map[string]interface{}{
				"longstring": "this string is too long", // exceeds limit of 10
			},
			expectError: true,
			errorType:   "string_length",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rm.ValidateConfigStructure(tt.config)
			if tt.expectError && err == nil {
				t.Errorf("expected error for config %+v, got nil", tt.config)
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error for config %+v: %v", tt.config, err)
			}
			
			if tt.expectError && err != nil {
				if structErr, ok := err.(*StructureLimitError); ok {
					if structErr.StructureType != tt.errorType {
						t.Errorf("expected error type %s, got %s", tt.errorType, structErr.StructureType)
					}
				} else {
					t.Errorf("expected StructureLimitError, got %T", err)
				}
			}
		})
	}
}

func TestResourceManager_WatcherLimits(t *testing.T) {
	limits := DefaultResourceLimits()
	limits.MaxWatchers = 3
	limits.MaxWatchersPerKey = 2
	
	rm := NewResourceManager(limits)
	
	// Test total watcher limit
	err := rm.CanAddWatcher("key1")
	if err != nil {
		t.Errorf("unexpected error adding first watcher: %v", err)
	}
	rm.AddWatcher("key1")
	
	err = rm.CanAddWatcher("key2")
	if err != nil {
		t.Errorf("unexpected error adding second watcher: %v", err)
	}
	rm.AddWatcher("key2")
	
	err = rm.CanAddWatcher("key3")
	if err != nil {
		t.Errorf("unexpected error adding third watcher: %v", err)
	}
	rm.AddWatcher("key3")
	
	// Fourth watcher should fail (exceeds total limit)
	err = rm.CanAddWatcher("key4")
	if err == nil {
		t.Error("expected error when adding fourth watcher (exceeds total limit)")
	}
	
	// Test per-key watcher limit
	rm = NewResourceManager(limits)
	
	err = rm.CanAddWatcher("key1")
	if err != nil {
		t.Errorf("unexpected error adding first watcher to key1: %v", err)
	}
	rm.AddWatcher("key1")
	
	err = rm.CanAddWatcher("key1")
	if err != nil {
		t.Errorf("unexpected error adding second watcher to key1: %v", err)
	}
	rm.AddWatcher("key1")
	
	// Third watcher for same key should fail
	err = rm.CanAddWatcher("key1")
	if err == nil {
		t.Error("expected error when adding third watcher to key1 (exceeds per-key limit)")
	}
}

func TestResourceManager_RateLimiting(t *testing.T) {
	limits := DefaultResourceLimits()
	limits.ReloadRateLimit = 100 * time.Millisecond
	limits.UpdateRateLimit = 50 * time.Millisecond
	limits.ValidationRateLimit = 200 * time.Millisecond
	
	rm := NewResourceManager(limits)
	
	// Test reload rate limiting
	err := rm.CanReload()
	if err != nil {
		t.Errorf("first reload should be allowed: %v", err)
	}
	
	// Immediate second reload should be blocked
	err = rm.CanReload()
	if err == nil {
		t.Error("immediate second reload should be blocked")
	}
	
	// Wait for rate limit to expire
	time.Sleep(150 * time.Millisecond)
	err = rm.CanReload()
	if err != nil {
		t.Errorf("reload after rate limit should be allowed: %v", err)
	}
	
	// Test update rate limiting
	err = rm.CanUpdate()
	if err != nil {
		t.Errorf("first update should be allowed: %v", err)
	}
	
	err = rm.CanUpdate()
	if err == nil {
		t.Error("immediate second update should be blocked")
	}
	
	// Test validation rate limiting
	err = rm.CanValidate()
	if err != nil {
		t.Errorf("first validation should be allowed: %v", err)
	}
	
	err = rm.CanValidate()
	if err == nil {
		t.Error("immediate second validation should be blocked")
	}
}

func TestResourceManager_Stats(t *testing.T) {
	rm := NewResourceManager(DefaultResourceLimits())
	
	// Initial stats should be zero
	stats := rm.GetStats()
	if stats.CurrentMemoryUsage != 0 {
		t.Errorf("initial memory usage should be 0, got %d", stats.CurrentMemoryUsage)
	}
	if stats.CurrentWatchers != 0 {
		t.Errorf("initial watcher count should be 0, got %d", stats.CurrentWatchers)
	}
	
	// Update some stats
	rm.UpdateMemoryUsage(1024)
	rm.AddWatcher("key1")
	rm.AddWatcher("key2")
	
	stats = rm.GetStats()
	if stats.CurrentMemoryUsage != 1024 {
		t.Errorf("expected memory usage 1024, got %d", stats.CurrentMemoryUsage)
	}
	if stats.CurrentWatchers != 2 {
		t.Errorf("expected watcher count 2, got %d", stats.CurrentWatchers)
	}
	if len(stats.WatchersByKey) != 2 {
		t.Errorf("expected 2 keys in WatchersByKey, got %d", len(stats.WatchersByKey))
	}
	
	// Remove a watcher
	rm.RemoveWatcher("key1")
	stats = rm.GetStats()
	if stats.CurrentWatchers != 1 {
		t.Errorf("expected watcher count 1 after removal, got %d", stats.CurrentWatchers)
	}
}

func TestResourceManager_ConcurrentAccess(t *testing.T) {
	rm := NewResourceManager(DefaultResourceLimits())
	
	const numGoroutines = 10
	const numOperations = 100
	
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	
	// Simulate concurrent operations
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numOperations; j++ {
				// Mix different types of operations
				switch j % 4 {
				case 0:
					rm.UpdateMemoryUsage(10)
				case 1:
					rm.AddWatcher("key")
				case 2:
					rm.RemoveWatcher("key")
				case 3:
					rm.CanReload()
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Just verify we didn't crash - exact values are hard to predict
	stats := rm.GetStats()
	if stats.ReloadAttempts < 0 {
		t.Error("reload attempts counter corrupted")
	}
}

func TestResourceErrors(t *testing.T) {
	// Test ResourceLimitError
	err := NewResourceLimitError("memory_usage", 2048, 1024, "memory exceeded")
	if !err.IsRecoverable() {
		t.Error("memory limit error should be recoverable")
	}
	if err.GetSeverity() != "high" {
		t.Errorf("expected high severity, got %s", err.GetSeverity())
	}
	expectedMsg := "resource limit exceeded [memory_usage]: memory exceeded (current: 2048, limit: 1024)"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
	
	// Test RateLimitError
	rateErr := NewRateLimitError("update", time.Second, 500*time.Millisecond, "rate limited")
	if !rateErr.IsRecoverable() {
		t.Error("rate limit error should be recoverable")
	}
	retryAfter := rateErr.GetRetryAfter()
	if retryAfter != 500*time.Millisecond {
		t.Errorf("expected retry after 500ms, got %v", retryAfter)
	}
	
	// Test StructureLimitError
	structErr := NewStructureLimitError("nesting_depth", 5, 3, "too deep")
	if structErr.IsRecoverable() {
		t.Error("structure limit error should not be recoverable")
	}
	if structErr.GetSeverity() != "high" {
		t.Errorf("expected high severity for nesting depth, got %s", structErr.GetSeverity())
	}
	
	// Test error classification
	if !IsResourceError(err) {
		t.Error("ResourceLimitError should be classified as resource error")
	}
	if !IsResourceError(rateErr) {
		t.Error("RateLimitError should be classified as resource error")
	}
	if !IsResourceError(structErr) {
		t.Error("StructureLimitError should be classified as resource error")
	}
	
	// Test non-resource error
	normalErr := fmt.Errorf("normal error")
	if IsResourceError(normalErr) {
		t.Error("normal error should not be classified as resource error")
	}
}

func TestResourceErrorHandler(t *testing.T) {
	handler := NewResourceErrorHandler()
	
	// Test resource limit error handling
	resourceErr := NewResourceLimitError("memory_usage", 2048, 1024, "memory exceeded")
	handledErr := handler.HandleError(resourceErr)
	if handledErr == nil {
		t.Error("resource limit error should not be completely handled")
	}
	if !strings.Contains(handledErr.Error(), "handled") {
		t.Error("handled error should indicate it was handled")
	}
	
	// Test rate limit error handling
	rateErr := NewRateLimitError("update", time.Second, 500*time.Millisecond, "rate limited")
	handledErr = handler.HandleError(rateErr)
	if handledErr == nil {
		t.Error("rate limit error should not be completely handled")
	}
	if !strings.Contains(handledErr.Error(), "retry after") {
		t.Error("handled rate limit error should include retry information")
	}
	
	// Test security error handling (should never be handled gracefully)
	secErr := NewSecurityError("path_traversal", "/etc/passwd", "security violation")
	handledErr = handler.HandleError(secErr)
	if handledErr != secErr {
		t.Error("security error should never be modified by handler")
	}
	
	// Test normal error pass-through
	normalErr := fmt.Errorf("normal error")
	handledErr = handler.HandleError(normalErr)
	if handledErr != normalErr {
		t.Error("normal error should be passed through unchanged")
	}
}

func TestResourceLimitsValidation(t *testing.T) {
	rm := NewResourceManager(nil) // Should use defaults
	
	tests := []struct {
		name    string
		limits  *ResourceLimits
		isValid bool
	}{
		{
			name:    "nil limits",
			limits:  nil,
			isValid: false,
		},
		{
			name: "valid limits",
			limits: &ResourceLimits{
				MaxFileSize:         1024,
				MaxMemoryUsage:      2048,
				MaxNestingDepth:     10,
				MaxKeys:             100,
				MaxWatchers:         50,
				MaxWatchersPerKey:   5,
				ReloadRateLimit:     time.Second,
				UpdateRateLimit:     time.Millisecond * 100,
				ValidationRateLimit: time.Millisecond * 500,
			},
			isValid: true,
		},
		{
			name: "negative file size",
			limits: &ResourceLimits{
				MaxFileSize:         -1,
				MaxMemoryUsage:      2048,
				MaxNestingDepth:     10,
				MaxKeys:             100,
				MaxWatchers:         50,
				MaxWatchersPerKey:   5,
				ReloadRateLimit:     time.Second,
				UpdateRateLimit:     time.Millisecond * 100,
				ValidationRateLimit: time.Millisecond * 500,
			},
			isValid: false,
		},
		{
			name: "zero watchers limit",
			limits: &ResourceLimits{
				MaxFileSize:         1024,
				MaxMemoryUsage:      2048,
				MaxNestingDepth:     10,
				MaxKeys:             100,
				MaxWatchers:         0, // Invalid
				MaxWatchersPerKey:   5,
				ReloadRateLimit:     time.Second,
				UpdateRateLimit:     time.Millisecond * 100,
				ValidationRateLimit: time.Millisecond * 500,
			},
			isValid: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rm.UpdateLimits(tt.limits)
			if tt.isValid && err != nil {
				t.Errorf("expected valid limits to be accepted, got error: %v", err)
			}
			if !tt.isValid && err == nil {
				t.Errorf("expected invalid limits to be rejected, got nil error")
			}
		})
	}
}

func TestResourceManagerTimeout(t *testing.T) {
	limits := DefaultResourceLimits()
	limits.LoadTimeout = 100 * time.Millisecond
	limits.ValidationTimeout = 50 * time.Millisecond
	limits.WatcherTimeout = 200 * time.Millisecond
	
	rm := NewResourceManager(limits)
	
	tests := []struct {
		name        string
		operation   string
		expectedTimeout time.Duration
	}{
		{"load operation", "load", 100 * time.Millisecond},
		{"validate operation", "validate", 50 * time.Millisecond},
		{"watcher operation", "watcher", 200 * time.Millisecond},
		{"unknown operation", "unknown", 100 * time.Millisecond}, // Should default to LoadTimeout
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := rm.WithTimeout(context.Background(), tt.operation)
			defer cancel()
			
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Error("context should have a deadline")
			}
			
			timeUntilDeadline := time.Until(deadline)
			// Allow some tolerance for timing
			if timeUntilDeadline < tt.expectedTimeout-10*time.Millisecond || 
			   timeUntilDeadline > tt.expectedTimeout+10*time.Millisecond {
				t.Errorf("expected timeout around %v, got %v", tt.expectedTimeout, timeUntilDeadline)
			}
		})
	}
}