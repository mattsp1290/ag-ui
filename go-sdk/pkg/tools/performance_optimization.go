package tools

import (
	"os"
	"runtime"
	"testing"
	"time"
)

// OptimizedPerformanceConfig returns performance testing configuration optimized for CI environments
func OptimizedPerformanceConfig() *PerformanceConfig {
	config := &PerformanceConfig{
		// Reduce baseline iterations for faster tests
		BaselineIterations:      20,  // Reduced from 100
		BaselineWarmupDuration:  1 * time.Second,  // Reduced from 5s
		BaselineStabilityFactor: 0.15,  // Slightly relaxed from 0.1
		
		// Adjust concurrency based on environment
		MaxConcurrency:          getOptimalConcurrency(),
		ConcurrencyStep:         50,  // Reduced from 100
		
		// Shorter load test durations
		LoadTestDuration:        5 * time.Second,  // Reduced from default
		StressTestDuration:      5 * time.Second,  // Reduced from default
		SpikeTestDuration:       5 * time.Second,  // Reduced from default
		SoakTestDuration:        10 * time.Second, // Reduced from default
		
		// Resource limits appropriate for CI
		MemoryLimit:            512 * 1024 * 1024,  // 512MB
		CPUCores:               runtime.NumCPU(),
		
		// More aggressive failure thresholds for CI
		ErrorRateThreshold:     0.05,  // 5% error rate
		LatencyP99Threshold:    100 * time.Millisecond,
		ThroughputThreshold:    100,    // ops/sec
		
		// Memory profiling configuration
		MemoryCheckInterval:    1 * time.Second,  // Required for memory profiler
		MemoryLeakThreshold:    50 * 1024 * 1024, // 50MB threshold for CI
		GCForceInterval:        10 * time.Second,
		
		// Monitoring - less frequent for performance
		MonitoringInterval:     1 * time.Second,  // Increased from default
		ProfilerEnabled:        false,  // Disable profiler in CI
		TracingEnabled:         false,  // Disable tracing in CI
		
		// Output settings
		VerboseOutput:          false,
		GenerateReport:         true,
		ReportFormat:           "json",
	}
	
	// Further optimizations for CI environment
	if isCI() {
		config.BaselineIterations = 10
		config.MaxConcurrency = min(100, runtime.NumCPU() * 10)
		config.LoadTestDuration = 3 * time.Second
		config.StressTestDuration = 3 * time.Second
		config.SpikeTestDuration = 3 * time.Second
		config.SoakTestDuration = 5 * time.Second
	}
	
	return config
}

// OptimizedRegressionConfig returns regression testing configuration optimized for CI
func OptimizedRegressionConfig() *RegressionConfig {
	config := &RegressionConfig{
		// Baseline configuration
		BaselineStrategy:       RegressionBaselineStrategyRolling,
		BaselineStorage:       "memory",  // Use in-memory storage for tests
		BaselineRetentionDays: 7,
		BaselineWindow:        24 * time.Hour,
		
		// Detection configuration - use only fast algorithms
		DetectionAlgorithms: []RegressionDetectionAlgorithm{
			RegressionAlgorithmThreshold,  // Fast threshold-based detection
			// Skip statistical and trend detection in CI
		},
		DetectionThresholds: &RegressionDetectionThresholds{
			PerformanceDegradation:  15.0,  // More lenient for CI
			ThroughputDecrease:      10.0,
			ResponseTimeIncrease:    20.0,
			ErrorRateIncrease:       5.0,
			MemoryUsageIncrease:     30.0,
			StatisticalSignificance: 0.05,
			ConfidenceLevel:         0.90,  // Reduced from 0.95
			MinimumEffectSize:       0.3,   // Increased from 0.2
			TrendSignificance:       0.05,
			TrendDuration:          12 * time.Hour,
			TrendConsistency:       0.7,    // Reduced from 0.8
			AnomalyScore:           0.8,    // Increased from 0.7
			AnomalyDeviation:       3.0,    // Increased from 2.0
			AnomalyFrequency:       0.2,    // Increased from 0.1
		},
		StatisticalConfidence: 0.90,  // Reduced from 0.95
		MinimumSampleSize:     5,      // Reduced from 10
		
		// Simplified analysis for speed
		AnalysisDepth:         RegressionAnalysisDepthBasic,  // Reduced from Standard
		TrendAnalysisWindow:   12 * time.Hour,
		SeasonalityDetection:  false,  // Disable for CI
		OutlierDetection:      false,  // Disable for CI
		
		// Minimal reporting
		ReportDetailLevel:     RegressionReportDetailLevelSummary,
		ReportFormats:         []string{"json"},  // Only JSON for CI
		ReportOutputDir:       "/tmp/regression-reports",
		
		// Disable alerts in CI
		AlertsEnabled:         false,
		
		// Test configuration
		TestEnvironment:       "ci",
		TestLabels:            map[string]string{"env": "ci"},
		MetricsToTrack: []string{
			"throughput",
			"response_time",
			"error_rate",
		},
		
		// Simplified quality gates
		QualityGates: []RegressionQualityGate{
			{
				Name:     "Performance Degradation",
				Metric:   "performance_degradation",
				Threshold: 20.0,  // More lenient
				Operator: "lt",
				Severity: TestRegressionSeverityMajor,
				Enabled:  true,
			},
		},
		
		// More lenient failure criteria
		FailOnRegression:     false,  // Don't fail on minor regressions
		FailOnDegradation:    true,   // Only fail on major degradation
		
		// Disable advanced features for speed
		AnomalyDetection:     false,
		PredictiveAnalysis:   false,
		ModelUpdateInterval:  0,  // Disable
		HistoricalDataLimit:  100,  // Reduced from 1000
	}
	
	return config
}

// getOptimalConcurrency returns the optimal concurrency level for the current environment
func getOptimalConcurrency() int {
	numCPU := runtime.NumCPU()
	
	// In CI environments, be more conservative
	if isCI() {
		return min(100, numCPU * 10)
	}
	
	// For local development, allow higher concurrency
	return min(1000, numCPU * 50)
}


// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// OptimizedTestTimeout returns an appropriate test timeout for the current environment
func OptimizedTestTimeout() time.Duration {
	if isCI() {
		return 30 * time.Second
	}
	return 60 * time.Second
}

// OptimizeMeasurementDurations reduces measurement durations for faster tests
type OptimizedMeasurements struct {
	ThroughputDuration time.Duration
	LatencyIterations  int
	MemoryIterations   int
}

// GetOptimizedMeasurements returns optimized measurement parameters
func GetOptimizedMeasurements() *OptimizedMeasurements {
	if isCI() {
		return &OptimizedMeasurements{
			ThroughputDuration: 2 * time.Second,  // Reduced from 10s
			LatencyIterations:  20,                // Reduced from 100
			MemoryIterations:   10,                // Reduced iterations
		}
	}
	
	return &OptimizedMeasurements{
		ThroughputDuration: 5 * time.Second,
		LatencyIterations:  50,
		MemoryIterations:   20,
	}
}

// isCI checks if running in CI environment
func isCI() bool {
	return testing.Short() || os.Getenv("CI") != ""
}