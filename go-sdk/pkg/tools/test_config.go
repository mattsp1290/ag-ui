package tools

import (
	"os"
	"strconv"
	"time"
)

// TestConfig holds configuration for test execution
type TestConfig struct {
	// Performance test settings
	PerformanceTestTimeout   time.Duration
	BaselineIterations       int
	ThroughputTestDuration   time.Duration
	
	// Regression test settings
	RegressionTestTimeout    time.Duration
	RegressionDataPoints     int
	
	// Load test settings
	LoadTestDuration         time.Duration
	MaxConcurrency           int
	
	// General settings
	EnableProfiling          bool
	EnableDetailedLogging    bool
	SkipSlowTests            bool
}

// GetTestConfig returns test configuration based on environment
func GetTestConfig() *TestConfig {
	config := &TestConfig{
		// Default values
		PerformanceTestTimeout:   30 * time.Second,
		BaselineIterations:       100,
		ThroughputTestDuration:   10 * time.Second,
		RegressionTestTimeout:    60 * time.Second,
		RegressionDataPoints:     50,
		LoadTestDuration:         30 * time.Second,
		MaxConcurrency:           1000,
		EnableProfiling:          false,
		EnableDetailedLogging:    false,
		SkipSlowTests:            false,
	}
	
	// Override with CI-optimized values
	if isCI() {
		config.PerformanceTestTimeout = 10 * time.Second
		config.BaselineIterations = 20
		config.ThroughputTestDuration = 2 * time.Second
		config.RegressionTestTimeout = 30 * time.Second
		config.RegressionDataPoints = 10
		config.LoadTestDuration = 5 * time.Second
		config.MaxConcurrency = 100
		config.SkipSlowTests = true
	}
	
	// Override with environment variables if set
	if v := os.Getenv("TEST_PERFORMANCE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			config.PerformanceTestTimeout = d
		}
	}
	
	if v := os.Getenv("TEST_BASELINE_ITERATIONS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			config.BaselineIterations = i
		}
	}
	
	if v := os.Getenv("TEST_THROUGHPUT_DURATION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			config.ThroughputTestDuration = d
		}
	}
	
	if v := os.Getenv("TEST_REGRESSION_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			config.RegressionTestTimeout = d
		}
	}
	
	if v := os.Getenv("TEST_LOAD_DURATION"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			config.LoadTestDuration = d
		}
	}
	
	if v := os.Getenv("TEST_MAX_CONCURRENCY"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			config.MaxConcurrency = i
		}
	}
	
	if v := os.Getenv("TEST_ENABLE_PROFILING"); v != "" {
		config.EnableProfiling = v == "true" || v == "1"
	}
	
	if v := os.Getenv("TEST_SKIP_SLOW"); v != "" {
		config.SkipSlowTests = v == "true" || v == "1"
	}
	
	return config
}

// ApplyTestConfig applies test configuration to performance config
func ApplyTestConfig(perfConfig *PerformanceConfig, testConfig *TestConfig) {
	perfConfig.BaselineIterations = testConfig.BaselineIterations
	perfConfig.MaxConcurrency = testConfig.MaxConcurrency
	perfConfig.LoadTestDuration = testConfig.LoadTestDuration
	perfConfig.ProfilerEnabled = testConfig.EnableProfiling
}

// ShouldSkipTest determines if a test should be skipped based on configuration
func ShouldSkipTest(testName string, estimatedDuration time.Duration) bool {
	config := GetTestConfig()
	
	if !config.SkipSlowTests {
		return false
	}
	
	// Skip tests that take longer than 10 seconds in CI
	slowTestThreshold := 10 * time.Second
	if isCI() {
		slowTestThreshold = 5 * time.Second
	}
	
	return estimatedDuration > slowTestThreshold
}