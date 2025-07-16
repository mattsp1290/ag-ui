package tools

import (
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
	ConcurrencyStep      int
	LoadTestDuration     time.Duration
	RampUpDuration       time.Duration
	RampDownDuration     time.Duration
	LoadPatterns         []LoadPattern
	StressTestDuration   time.Duration
	StressMaxConcurrency int
	SpikeTestDuration    time.Duration
	SoakTestDuration     time.Duration
	
	// Memory testing configuration
	MemoryCheckInterval  time.Duration
	MemoryLeakThreshold  int64 // Bytes
	GCForceInterval      time.Duration
	MemoryLimit          int64 // Bytes
	
	// Regression testing configuration
	RegressionThreshold  float64 // % performance degradation threshold
	WarmupIterations     int
	BenchmarkIterations  int
	
	// Resource limits
	CPUCores               int
	
	// Thresholds
	ErrorRateThreshold     float64
	LatencyP99Threshold    time.Duration
	ThroughputThreshold    float64
	
	// Monitoring configuration
	MonitoringInterval     time.Duration
	ProfilerEnabled        bool
	TracingEnabled         bool
	
	// Output settings
	VerboseOutput          bool
	GenerateReport         bool
	ReportFormat           string
}

// LoadPattern defines different load testing patterns
type LoadPattern struct {
	Name      string
	Type      LoadPatternType
	Intensity int           // Peak load level
	Duration  time.Duration // Pattern duration
	Settings  map[string]interface{} // Pattern-specific settings
}

// LoadPatternType defines the type of load pattern
type LoadPatternType int

const (
	LoadPatternConstant LoadPatternType = iota
	LoadPatternRamp
	LoadPatternSpike
	LoadPatternWave
	LoadPatternStair
	LoadPatternChaos
)

// DefaultPerformanceConfig returns default performance testing configuration
func DefaultPerformanceConfig() *PerformanceConfig {
	return &PerformanceConfig{
		BaselineIterations:      100,
		BaselineWarmupDuration:  5 * time.Second,
		BaselineStabilityFactor: 0.1,
		MaxConcurrency:          1000,
		ConcurrencyStep:         100,
		LoadTestDuration:        60 * time.Second,
		RampUpDuration:          10 * time.Second,
		RampDownDuration:        10 * time.Second,
		LoadPatterns: []LoadPattern{
			{Name: "constant", Type: LoadPatternConstant, Intensity: 100},
			{Name: "ramp", Type: LoadPatternRamp, Intensity: 200},
			{Name: "spike", Type: LoadPatternSpike, Intensity: 500},
			{Name: "wave", Type: LoadPatternWave, Intensity: 300},
		},
		StressTestDuration:      300 * time.Second,
		StressMaxConcurrency:    2000,
		SpikeTestDuration:       30 * time.Second,
		SoakTestDuration:        600 * time.Second,
		MemoryCheckInterval:     1 * time.Second,
		MemoryLeakThreshold:     100 * 1024 * 1024, // 100MB
		GCForceInterval:         10 * time.Second,
		MemoryLimit:             1024 * 1024 * 1024, // 1GB
		RegressionThreshold:     10.0, // 10% degradation threshold
		WarmupIterations:        10,
		BenchmarkIterations:     50,
		CPUCores:                4,
		ErrorRateThreshold:      0.01, // 1% error rate
		LatencyP99Threshold:     500 * time.Millisecond,
		ThroughputThreshold:     1000, // ops/sec
		MonitoringInterval:      500 * time.Millisecond,
		ProfilerEnabled:         true,
		TracingEnabled:          true,
		VerboseOutput:           true,
		GenerateReport:          true,
		ReportFormat:            "json",
	}
}