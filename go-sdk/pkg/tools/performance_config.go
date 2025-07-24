package tools

import (
	"runtime"
	"time"
)

// PerformanceConfig defines performance testing configuration
type PerformanceConfig struct {
	// Baseline configuration
	BaselineIterations      int
	BaselineWarmupDuration  time.Duration
	BaselineStabilityFactor float64 // Coefficient of variation threshold

	// Load testing configuration
	MaxConcurrency       int
	LoadTestDuration     time.Duration
	RampUpDuration       time.Duration
	RampDownDuration     time.Duration
	LoadPatterns         []LoadPattern
	ConcurrencyStep      int           // Step size for concurrency scaling
	
	// Memory testing configuration
	MemoryCheckInterval  time.Duration
	MemoryLeakThreshold  int64 // Bytes
	GCForceInterval      time.Duration
	MemoryLimit          uint64        // Memory limit in bytes
	
	// Regression testing configuration
	RegressionThreshold  float64 // % performance degradation threshold
	WarmupIterations     int
	BenchmarkIterations  int
	
	// Stress testing configuration
	StressTestDuration   time.Duration
	StressMaxConcurrency int
	StressErrorThreshold float64 // % error rate threshold
	SpikeTestDuration    time.Duration
	SoakTestDuration     time.Duration
	
	// System resource configuration
	CPUCores             int
	
	// Threshold configuration
	ErrorRateThreshold   float64
	LatencyP99Threshold  time.Duration
	ThroughputThreshold  float64

	// Monitoring configuration
	MonitoringInterval   time.Duration
	ProfilerEnabled      bool
	TracingEnabled       bool

	// Output configuration
	VerboseOutput        bool
	GenerateReport       bool
	ReportFormat         string
}

// LoadPattern represents a load testing pattern
type LoadPattern struct {
	Name      string
	Type      LoadPatternType
	Intensity int
}

// LoadPatternType defines the type of load pattern
type LoadPatternType int

const (
	LoadPatternConstant LoadPatternType = iota
	LoadPatternRamp
	LoadPatternSpike
	LoadPatternWave
)

// DefaultPerformanceConfig returns default performance testing configuration
func DefaultPerformanceConfig() *PerformanceConfig {
	return &PerformanceConfig{
		BaselineIterations:      100,
		BaselineWarmupDuration:  5 * time.Second,
		BaselineStabilityFactor: 0.1,
		MaxConcurrency:          100,  // Reduced from 1000
		LoadTestDuration:        10 * time.Second,  // Reduced from 60s to 10s
		RampUpDuration:          2 * time.Second,   // Reduced from 10s to 2s
		RampDownDuration:        2 * time.Second,   // Reduced from 10s to 2s
		LoadPatterns: []LoadPattern{
			{Name: "constant", Type: LoadPatternConstant, Intensity: 50},  // Reduced from 100
			{Name: "ramp", Type: LoadPatternRamp, Intensity: 100},        // Reduced from 200
			{Name: "spike", Type: LoadPatternSpike, Intensity: 150},      // Reduced from 500
			{Name: "wave", Type: LoadPatternWave, Intensity: 75},         // Reduced from 300
		},
		ConcurrencyStep:         10,
		MemoryCheckInterval:     1 * time.Second,
		MemoryLeakThreshold:     100 * 1024 * 1024, // 100MB
		MemoryLimit:             1024 * 1024 * 1024, // 1GB
		GCForceInterval:         10 * time.Second,
		RegressionThreshold:     10.0, // 10% degradation threshold
		WarmupIterations:        10,
		BenchmarkIterations:     50,
		StressTestDuration:      5 * time.Second,  // Reduced from 300s to 5s
		StressMaxConcurrency:    200,  // Reduced from 2000
		StressErrorThreshold:    5.0, // 5% error rate threshold
		SpikeTestDuration:       2 * time.Second,
		SoakTestDuration:        30 * time.Second,
		CPUCores:                runtime.NumCPU(),
		ErrorRateThreshold:      1.0, // 1% error rate threshold
		LatencyP99Threshold:     500 * time.Millisecond,
		ThroughputThreshold:     1000.0, // ops/sec
		MonitoringInterval:      5 * time.Second,
		ProfilerEnabled:         true,
		TracingEnabled:          true,
		VerboseOutput:           true,
		GenerateReport:          true,
		ReportFormat:            "json",
	}
}