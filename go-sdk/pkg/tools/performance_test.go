// +build integration

package tools

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// PerformanceFramework provides comprehensive performance testing capabilities
type PerformanceFramework struct {
	baseline       *PerformanceBaseline
	config         *PerformanceConfig
	metrics        *PerformanceMetrics
	regressionTest *RegressionTester
	loadGenerator  *LoadGenerator
	memoryProfiler *MemoryProfiler
}


// PerformanceMetrics tracks comprehensive performance statistics
type PerformanceMetrics struct {
	mu sync.RWMutex
	
	// Execution metrics
	ExecutionTimes      []time.Duration
	ThroughputMetrics   []ThroughputMeasurement
	ConcurrencyMetrics  []ConcurrencyMeasurement
	ErrorMetrics        []ErrorMeasurement
	
	// Memory metrics
	MemoryUsage         []MemoryMeasurement
	GCMetrics           []GCMeasurement
	AllocMetrics        []AllocMeasurement
	
	// Resource metrics
	CPUUsage            []CPUMeasurement
	GoroutineCount      []GoroutineMeasurement
	
	// Latency metrics
	LatencyPercentiles  map[string]time.Duration // P50, P95, P99, P999
	LatencyDistribution []LatencyBucket
	
	// System metrics
	SystemLoad          []SystemLoadMeasurement
	FileDescriptors     []FDMeasurement
}

// ThroughputMeasurement records throughput at a specific time
type ThroughputMeasurement struct {
	Timestamp   time.Time
	Throughput  float64 // Operations per second
	Concurrency int
}

// ConcurrencyMeasurement records concurrency metrics
type ConcurrencyMeasurement struct {
	Timestamp         time.Time
	ActiveExecutions  int
	QueuedExecutions  int
	CompletedTotal    int64
	FailedTotal       int64
}

// ErrorMeasurement records error statistics
type ErrorMeasurement struct {
	Timestamp  time.Time
	ErrorRate  float64 // Percentage
	ErrorTypes map[string]int
}

// MemoryMeasurement records memory usage at a specific time
type MemoryMeasurement struct {
	Timestamp    time.Time
	HeapInuse    uint64
	HeapIdle     uint64
	HeapAlloc    uint64
	HeapObjects  uint64
	StackInuse   uint64
	GCCycles     uint32
	NextGC       uint64
}

// GCMeasurement records garbage collection metrics
type GCMeasurement struct {
	Timestamp    time.Time
	GCCount      uint32
	GCTotalTime  time.Duration
	GCPauseTime  time.Duration
	GCFrequency  float64 // GCs per second
}

// AllocMeasurement records allocation metrics
type AllocMeasurement struct {
	Timestamp    time.Time
	AllocRate    float64 // Bytes per second
	AllocCount   uint64
	FreeCount    uint64
	LiveObjects  uint64
}

// CPUMeasurement records CPU usage
type CPUMeasurement struct {
	Timestamp time.Time
	Usage     float64 // Percentage
	UserTime  time.Duration
	SysTime   time.Duration
}

// GoroutineMeasurement records goroutine metrics
type GoroutineMeasurement struct {
	Timestamp time.Time
	Count     int
	Running   int
	Waiting   int
}

// LatencyBucket represents a latency distribution bucket
type LatencyBucket struct {
	MinLatency time.Duration
	MaxLatency time.Duration
	Count      int64
}

// SystemLoadMeasurement records system load metrics
type SystemLoadMeasurement struct {
	Timestamp time.Time
	Load1     float64
	Load5     float64
	Load15    float64
}

// FDMeasurement records file descriptor usage
type FDMeasurement struct {
	Timestamp time.Time
	Open      int
	Max       int
	Usage     float64 // Percentage
}

// PerformanceBaseline stores baseline performance measurements
type PerformanceBaseline struct {
	mu sync.RWMutex
	
	// Execution baselines
	ExecutionTimeBaseline    time.Duration
	ThroughputBaseline       float64
	ConcurrencyBaseline      int
	ErrorRateBaseline        float64
	
	// Memory baselines
	MemoryUsageBaseline      uint64
	GCFrequencyBaseline      float64
	AllocRateBaseline        float64
	
	// Latency baselines
	LatencyP50Baseline       time.Duration
	LatencyP95Baseline       time.Duration
	LatencyP99Baseline       time.Duration
	
	// System baselines
	CPUUsageBaseline         float64
	GoroutineCountBaseline   int
	
	// Metadata
	CreatedAt                time.Time
	Environment              string
	GoVersion                string
	Platform                 string
	CommitHash               string
}


// LoadGenerator generates various load patterns for testing
type LoadGenerator struct {
	config    *PerformanceConfig
	engine    *ExecutionEngine
	registry  *Registry
	tools     []*Tool
	
	// State tracking
	mu            sync.RWMutex
	activeWorkers int
	totalOps      int64
	totalErrors   int64
	startTime     time.Time
	
	// Control channels
	stopChan     chan struct{}
	workersChan  chan struct{}
	resultsChan  chan *LoadResult
}

// LoadResult represents the result of a load test operation
type LoadResult struct {
	Timestamp    time.Time
	Duration     time.Duration
	Success      bool
	Error        error
	OpType       string
	WorkerID     int
	Concurrency  int
	MemoryUsage  uint64
}

// MemoryProfiler monitors memory usage and detects leaks
type MemoryProfiler struct {
	config          *PerformanceConfig
	measurements    []MemoryMeasurement
	leakDetector    *LeakDetector
	mu             sync.RWMutex
	stopChan       chan struct{}
	isRunning      bool
}

// LeakDetector detects memory leaks using various heuristics
type LeakDetector struct {
	baseline         uint64
	samples          []uint64
	threshold        uint64
	growthRate       float64
	detectionWindow  time.Duration
	confidenceLevel  float64
}

// RegressionTester compares current performance against baselines
type RegressionTester struct {
	config          *PerformanceConfig
	baseline        *PerformanceBaseline
	currentMetrics  *PerformanceMetrics
	regressions     []PerformanceRegression
	mu             sync.RWMutex
}

// PerformanceRegression represents a detected performance regression
type PerformanceRegression struct {
	Metric         string
	BaselineValue  float64
	CurrentValue   float64
	Degradation    float64 // Percentage
	Severity       PerfRegressionSeverity
	Description    string
	Timestamp      time.Time
	Recommendations []string
}

// PerfRegressionSeverity indicates the severity of a performance regression
type PerfRegressionSeverity int

const (
	PerfRegressionSeverityLow PerfRegressionSeverity = iota
	PerfRegressionSeverityMedium
	PerfRegressionSeverityHigh
	PerfRegressionSeverityCritical
)

// NewPerformanceFramework creates a new performance testing framework
func NewPerformanceFramework(config *PerformanceConfig) *PerformanceFramework {
	if config == nil {
		config = DefaultPerformanceConfig()
	}
	
	framework := &PerformanceFramework{
		config:  config,
		metrics: &PerformanceMetrics{
			LatencyPercentiles: make(map[string]time.Duration),
		},
		baseline: &PerformanceBaseline{
			CreatedAt:   time.Now(),
			Environment: "test",
			GoVersion:   runtime.Version(),
			Platform:    runtime.GOOS + "/" + runtime.GOARCH,
		},
	}
	
	framework.regressionTest = &RegressionTester{
		config:   config,
		baseline: framework.baseline,
		currentMetrics: framework.metrics,
	}
	
	framework.memoryProfiler = &MemoryProfiler{
		config:       config,
		measurements: make([]MemoryMeasurement, 0),
		leakDetector: &LeakDetector{
			threshold:        uint64(config.MemoryLeakThreshold),
			detectionWindow:  60 * time.Second,
			confidenceLevel:  0.95,
			samples:          make([]uint64, 0),
		},
		stopChan: make(chan struct{}),
	}
	
	return framework
}

// RunComprehensivePerformanceTest runs all performance tests
func (pf *PerformanceFramework) RunComprehensivePerformanceTest(t *testing.T) *PerformanceReport {
	t.Helper()
	
	// Skip long-running tests in short mode
	if testing.Short() {
		t.Skip("Skipping comprehensive performance test in short mode")
	}
	
	report := &PerformanceReport{
		StartTime: time.Now(),
		Config:    pf.config,
		Results:   make(map[string]interface{}),
	}
	
	// 1. Establish baseline
	t.Run("Baseline", func(t *testing.T) {
		result := pf.EstablishBaseline(t)
		report.Results["baseline"] = result
	})
	
	// 2. Run load tests
	t.Run("LoadTests", func(t *testing.T) {
		result := pf.RunLoadTests(t)
		report.Results["load_tests"] = result
	})
	
	// 3. Run memory tests
	t.Run("MemoryTests", func(t *testing.T) {
		result := pf.RunMemoryTests(t)
		report.Results["memory_tests"] = result
	})
	
	// 4. Run scalability tests
	t.Run("ScalabilityTests", func(t *testing.T) {
		result := pf.RunScalabilityTests(t)
		report.Results["scalability_tests"] = result
	})
	
	// 5. Run stress tests
	t.Run("StressTests", func(t *testing.T) {
		result := pf.RunStressTests(t)
		report.Results["stress_tests"] = result
	})
	
	// 6. Run regression tests
	t.Run("RegressionTests", func(t *testing.T) {
		result := pf.RunRegressionTests(t)
		report.Results["regression_tests"] = result
	})
	
	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime)
	
	return report
}

// EstablishBaseline creates performance baselines
func (pf *PerformanceFramework) EstablishBaseline(t *testing.T) *BaselineResult {
	t.Helper()
	
	// Skip in short mode for faster CI tests
	if testing.Short() {
		t.Skip("Skipping baseline establishment in short mode")
	}
	
	result := &BaselineResult{
		StartTime: time.Now(),
	}
	
	// Create context with timeout to prevent infinite loops - use optimized timeout
	timeout := OptimizedTestTimeout()
	if testing.Short() {
		timeout = 5 * time.Second // Much shorter for short mode
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	// Create test environment
	registry := NewRegistry()
	engine := NewExecutionEngine(registry)
	
	// Create test tools
	tools := pf.createTestTools(10)
	for _, tool := range tools {
		if err := registry.Register(tool); err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}
	}
	
	// Warmup with context
	pf.warmupWithContext(ctx, engine, tools, pf.config.BaselineWarmupDuration)
	
	// Measure baseline metrics with context and timeouts
	result.ExecutionTime = pf.measureBaselineExecutionTimeWithContext(ctx, engine, tools)
	result.Throughput = pf.measureBaselineThroughputWithContext(ctx, engine, tools)
	result.MemoryUsage = pf.measureBaselineMemoryUsageWithContext(ctx, engine, tools)
	result.Latency = pf.measureBaselineLatencyWithContext(ctx, engine, tools)
	
	// Update baseline
	pf.baseline.mu.Lock()
	pf.baseline.ExecutionTimeBaseline = result.ExecutionTime
	pf.baseline.ThroughputBaseline = result.Throughput
	pf.baseline.MemoryUsageBaseline = result.MemoryUsage
	pf.baseline.LatencyP50Baseline = result.Latency.P50
	pf.baseline.LatencyP95Baseline = result.Latency.P95
	pf.baseline.LatencyP99Baseline = result.Latency.P99
	pf.baseline.mu.Unlock()
	
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	
	return result
}

// RunLoadTests runs various load testing patterns
func (pf *PerformanceFramework) RunLoadTests(t *testing.T) *LoadTestResult {
	t.Helper()
	
	// Skip heavy load tests in short mode
	if testing.Short() {
		t.Skip("Skipping load tests in short mode")
	}
	
	result := &LoadTestResult{
		StartTime: time.Now(),
		Patterns:  make(map[string]*LoadPatternResult),
	}
	
	// Create test environment
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(pf.config.MaxConcurrency))
	
	// Create test tools
	tools := pf.createTestTools(50)
	for _, tool := range tools {
		if err := registry.Register(tool); err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}
	}
	
	// Initialize load generator
	loadGen := &LoadGenerator{
		config:      pf.config,
		engine:      engine,
		registry:    registry,
		tools:       tools,
		stopChan:    make(chan struct{}),
		workersChan: make(chan struct{}, pf.config.MaxConcurrency),
		resultsChan: make(chan *LoadResult, pf.config.MaxConcurrency*2),
	}
	
	// Test each load pattern
	for _, pattern := range pf.config.LoadPatterns {
		t.Run(pattern.Name, func(t *testing.T) {
			patternResult := loadGen.RunLoadPattern(t, pattern)
			result.Patterns[pattern.Name] = patternResult
		})
	}
	
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	
	return result
}

// RunMemoryTests runs memory profiling and leak detection tests
func (pf *PerformanceFramework) RunMemoryTests(t *testing.T) *MemoryTestResult {
	t.Helper()
	
	// Skip resource-intensive memory tests in short mode
	if testing.Short() {
		t.Skip("Skipping memory tests in short mode")
	}
	
	result := &MemoryTestResult{
		StartTime: time.Now(),
	}
	
	// Start memory profiler
	pf.memoryProfiler.Start()
	defer pf.memoryProfiler.Stop()
	
	// Create test environment
	registry := NewRegistry()
	engine := NewExecutionEngine(registry)
	
	// Memory stress test
	t.Run("MemoryStress", func(t *testing.T) {
		result.StressTest = pf.runMemoryStressTest(t, engine, registry)
	})
	
	// Memory leak detection test
	t.Run("LeakDetection", func(t *testing.T) {
		result.LeakDetection = pf.runMemoryLeakTest(t, engine, registry)
	})
	
	// Memory efficiency test
	t.Run("MemoryEfficiency", func(t *testing.T) {
		result.EfficiencyTest = pf.runMemoryEfficiencyTest(t, engine, registry)
	})
	
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.FinalMemoryUsage = pf.memoryProfiler.GetCurrentMemoryUsage()
	
	return result
}

// RunScalabilityTests runs scalability tests with varying numbers of tools
func (pf *PerformanceFramework) RunScalabilityTests(t *testing.T) *ScalabilityTestResult {
	t.Helper()
	
	// Skip scalability tests in short mode
	if testing.Short() {
		t.Skip("Skipping scalability tests in short mode")
	}
	
	result := &ScalabilityTestResult{
		StartTime: time.Now(),
		Results:   make(map[int]*PerfScalabilityMeasurement),
	}
	
	// Test different tool counts (optimized for environment)
	toolCounts := []int{10, 50, 100}
	if !testing.Short() && !isCI() {
		// Add more extensive testing for local development
		toolCounts = append(toolCounts, 200, 500)
	}
	
	for _, toolCount := range toolCounts {
		t.Run(fmt.Sprintf("Tools_%d", toolCount), func(t *testing.T) {
			measurement := pf.runScalabilityTest(t, toolCount)
			result.Results[toolCount] = measurement
		})
	}
	
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	
	return result
}

// RunStressTests runs high-concurrency stress tests
func (pf *PerformanceFramework) RunStressTests(t *testing.T) *StressTestResult {
	t.Helper()
	
	// Skip stress tests in short mode
	if testing.Short() {
		t.Skip("Skipping stress tests in short mode")
	}
	
	result := &StressTestResult{
		StartTime: time.Now(),
	}
	
	// Create test environment
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(pf.config.StressMaxConcurrency))
	
	// Create test tools
	tools := pf.createTestTools(100)
	for _, tool := range tools {
		if err := registry.Register(tool); err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}
	}
	
	// Initialize stress test
	stressTest := &StressTest{
		config:      pf.config,
		engine:      engine,
		registry:    registry,
		tools:       tools,
		stopChan:    make(chan struct{}),
		resultsChan: make(chan *StressResult, pf.config.StressMaxConcurrency*2),
	}
	
	// Run stress test
	result.Measurements = stressTest.Run(t)
	
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	
	return result
}

// RunRegressionTests checks for performance regressions
func (pf *PerformanceFramework) RunRegressionTests(t *testing.T) *RegressionTestResult {
	t.Helper()
	
	result := &RegressionTestResult{
		StartTime: time.Now(),
	}
	
	// Run regression analysis
	regressions := pf.regressionTest.DetectRegressions()
	result.Regressions = regressions
	
	// Classify regressions by severity
	result.Severity = make(map[PerfRegressionSeverity]int)
	for _, regression := range regressions {
		result.Severity[regression.Severity]++
	}
	
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	
	return result
}

// Helper methods for creating test tools and measuring performance
func (pf *PerformanceFramework) createTestTools(count int) []*Tool {
	tools := make([]*Tool, count)
	
	for i := 0; i < count; i++ {
		tools[i] = &Tool{
			ID:          fmt.Sprintf("test-tool-%d", i),
			Name:        fmt.Sprintf("Test Tool %d", i),
			Description: fmt.Sprintf("Test tool for performance testing %d", i),
			Version:     "1.0.0",
			Schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"input": {
						Type:        "string",
						Description: "Test input",
					},
				},
				Required: []string{"input"},
			},
			Executor: &TestExecutor{
				processingTime: time.Duration(rand.Intn(100)) * time.Millisecond,
			},
			Capabilities: &ToolCapabilities{
				Cacheable:  true,
				Cancelable: true,
				Retryable:  true,
				Timeout:    30 * time.Second,
			},
		}
	}
	
	return tools
}

// TestExecutor implements ToolExecutor for performance testing
type TestExecutor struct {
	processingTime time.Duration
}

func (e *TestExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	// Limit processing time to prevent hangs
	maxProcessingTime := 1 * time.Second
	actualProcessingTime := e.processingTime
	if actualProcessingTime > maxProcessingTime {
		actualProcessingTime = maxProcessingTime
	}
	
	// Create a timer that respects both context and processing time
	processingTimer := time.NewTimer(actualProcessingTime)
	defer processingTimer.Stop()
	
	// Simulate processing time with proper context handling
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-processingTimer.C:
		// Processing completed normally
	}
	
	// Simulate some work
	result := make(map[string]interface{})
	result["processed"] = params["input"]
	result["timestamp"] = time.Now()
	result["processing_time"] = actualProcessingTime
	
	return &ToolExecutionResult{
		Success:   true,
		Data:      result,
		Timestamp: time.Now(),
	}, nil
}

// Result types for different test categories
type PerformanceReport struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Config    *PerformanceConfig
	Results   map[string]interface{}
}

type BaselineResult struct {
	StartTime     time.Time
	EndTime       time.Time
	Duration      time.Duration
	ExecutionTime time.Duration
	Throughput    float64
	MemoryUsage   uint64
	Latency       *LatencyMetrics
}

type LatencyMetrics struct {
	P50  time.Duration
	P95  time.Duration
	P99  time.Duration
	P999 time.Duration
	Min  time.Duration
	Max  time.Duration
	Mean time.Duration
}

type LoadTestResult struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Patterns  map[string]*LoadPatternResult
}

type LoadPatternResult struct {
	Pattern            LoadPattern
	TotalOps           int64
	SuccessfulOps      int64
	FailedOps          int64
	ErrorRate          float64
	AverageThroughput  float64
	PeakThroughput     float64
	AverageLatency     time.Duration
	LatencyPercentiles *LatencyMetrics
	ResourceUsage      *ResourceUsage
}

type ResourceUsage struct {
	MaxMemory       uint64
	AverageMemory   uint64
	MaxCPU          float64
	AverageCPU      float64
	MaxGoroutines   int
	AverageGoroutines int
}

type MemoryTestResult struct {
	StartTime         time.Time
	EndTime           time.Time
	Duration          time.Duration
	StressTest        *MemoryStressResult
	LeakDetection     *MemoryLeakResult
	EfficiencyTest    *MemoryEfficiencyResult
	FinalMemoryUsage  uint64
}

type MemoryStressResult struct {
	MaxMemoryUsage     uint64
	AverageMemoryUsage uint64
	GCFrequency        float64
	AllocRate          float64
	LeakDetected       bool
}

type MemoryLeakResult struct {
	LeakDetected    bool
	LeakRate        float64 // Bytes per second
	LeakConfidence  float64 // 0-1
	LeakSources     []string
	Recommendations []string
}

type MemoryEfficiencyResult struct {
	MemoryPerOperation uint64
	AllocationsPerOp   uint64
	GCOverhead         float64
	MemoryUtilization  float64
}

type ScalabilityTestResult struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Results   map[int]*PerfScalabilityMeasurement
}

type PerfScalabilityMeasurement struct {
	ToolCount        int
	RegistrationTime time.Duration
	LookupTime       time.Duration
	ExecutionTime    time.Duration
	MemoryUsage      uint64
	ThroughputLimit  float64
	ResourceScaling  float64
}

type StressTestResult struct {
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	Measurements []*StressMeasurement
}

type StressMeasurement struct {
	Timestamp      time.Time
	Concurrency    int
	Throughput     float64
	ErrorRate      float64
	MemoryUsage    uint64
	CPUUsage       float64
	GoroutineCount int
	ResponseTime   time.Duration
}

type RegressionTestResult struct {
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	Regressions []PerformanceRegression
	Severity    map[PerfRegressionSeverity]int
}

// StressTest implements high-concurrency stress testing
type StressTest struct {
	config      *PerformanceConfig
	engine      *ExecutionEngine
	registry    *Registry
	tools       []*Tool
	stopChan    chan struct{}
	resultsChan chan *StressResult
}

type StressResult struct {
	Timestamp   time.Time
	Success     bool
	Duration    time.Duration
	Error       error
	Concurrency int
	WorkerID    int
}

func (st *StressTest) Run(t *testing.T) []*StressMeasurement {
	t.Helper()
	
	var measurements []*StressMeasurement
	measurementsChan := make(chan *StressMeasurement, 100)
	
	// Start measurement collector
	go func() {
		for measurement := range measurementsChan {
			measurements = append(measurements, measurement)
		}
	}()
	
	// Start stress test workers with optimized concurrency levels
	var wg sync.WaitGroup
	concurrencyLevels := []int{5, 10, 25}
	if !testing.Short() {
		concurrencyLevels = append(concurrencyLevels, 50, 100)
		if !isCI() {
			// Only add heavy concurrency for local development
			concurrencyLevels = append(concurrencyLevels, st.config.StressMaxConcurrency)
		}
	}
	
	for _, concurrency := range concurrencyLevels {
		wg.Add(1)
		go func(conc int) {
			defer wg.Done()
			st.runStressLevel(t, conc, measurementsChan)
		}(concurrency)
	}
	
	wg.Wait()
	close(measurementsChan)
	
	return measurements
}

func (st *StressTest) runStressLevel(t *testing.T, concurrency int, measurementsChan chan<- *StressMeasurement) {
	t.Helper()
	
	ctx, cancel := context.WithTimeout(context.Background(), st.config.StressTestDuration)
	defer cancel()
	
	var wg sync.WaitGroup
	startTime := time.Now()
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			st.stressWorker(ctx, workerID, concurrency, measurementsChan)
		}(i)
	}
	
	wg.Wait()
	
	// Final measurement
	measurement := &StressMeasurement{
		Timestamp:      time.Now(),
		Concurrency:    concurrency,
		ResponseTime:   time.Since(startTime),
		MemoryUsage:    st.getCurrentMemoryUsage(),
		GoroutineCount: runtime.NumGoroutine(),
	}
	
	measurementsChan <- measurement
}

func (st *StressTest) stressWorker(ctx context.Context, workerID, concurrency int, measurementsChan chan<- *StressMeasurement) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Execute random tool
			tool := st.tools[rand.Intn(len(st.tools))]
			startTime := time.Now()
			
			_, err := st.engine.Execute(ctx, tool.ID, map[string]interface{}{
				"input": fmt.Sprintf("stress-test-%d", workerID),
			})
			
			duration := time.Since(startTime)
			
			// Record measurement
			measurement := &StressMeasurement{
				Timestamp:      time.Now(),
				Concurrency:    concurrency,
				ResponseTime:   duration,
				ErrorRate:      calculateErrorRate(err),
				MemoryUsage:    st.getCurrentMemoryUsage(),
				CPUUsage:       st.getCurrentCPUUsage(),
				GoroutineCount: runtime.NumGoroutine(),
			}
			
			select {
			case measurementsChan <- measurement:
			default:
				// Skip if channel is full
			}
		}
	}
}

func (st *StressTest) getCurrentMemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

func (st *StressTest) getCurrentCPUUsage() float64 {
	// Simplified CPU usage calculation
	// In a real implementation, you'd use more sophisticated methods
	return float64(runtime.NumCPU()) * 50.0 // Placeholder
}

func calculateErrorRate(err error) float64 {
	if err != nil {
		return 100.0
	}
	return 0.0
}

// Additional helper methods for the performance framework
func (pf *PerformanceFramework) warmupWithContext(parentCtx context.Context, engine *ExecutionEngine, tools []*Tool, duration time.Duration) {
	// Create a timeout context that respects both parent context and duration
	ctx, cancel := context.WithTimeout(parentCtx, duration)
	defer cancel()
	
	var wg sync.WaitGroup
	// Scale concurrency based on environment
	concurrency := 10
	if testing.Short() {
		concurrency = 3 // Minimal concurrency for short tests
	} else if isCI() {
		concurrency = 5 // Reduced concurrency for CI
	}
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					tool := tools[rand.Intn(len(tools))]
					// Use a short timeout for each operation
					opCtx, opCancel := context.WithTimeout(ctx, 50*time.Millisecond)
					engine.Execute(opCtx, tool.ID, map[string]interface{}{
						"input": "warmup",
					})
					opCancel()
				}
			}
		}()
	}
	wg.Wait()
}

// Legacy method for backwards compatibility
func (pf *PerformanceFramework) warmup(engine *ExecutionEngine, tools []*Tool, duration time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	pf.warmupWithContext(ctx, engine, tools, duration)
}

func (pf *PerformanceFramework) measureBaselineExecutionTimeWithContext(ctx context.Context, engine *ExecutionEngine, tools []*Tool) time.Duration {
	var totalTime time.Duration
	// Use optimized iterations for CI
	measurements := GetOptimizedMeasurements()
	iterations := measurements.LatencyIterations
	if !isCI() {
		iterations = pf.config.BaselineIterations
	}
	successfulIterations := 0
	
	for i := 0; i < iterations; i++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			// Context cancelled, return average of successful iterations
			if successfulIterations > 0 {
				return totalTime / time.Duration(successfulIterations)
			}
			return 0
		default:
			// Continue with iteration
		}
		
		tool := tools[rand.Intn(len(tools))]
		start := time.Now()
		
		// Create operation context with timeout
		opCtx, opCancel := context.WithTimeout(ctx, 1*time.Second)
		_, err := engine.Execute(opCtx, tool.ID, map[string]interface{}{
			"input": fmt.Sprintf("baseline-test-%d", i),
		})
		opCancel()
		
		elapsed := time.Since(start)
		if err == nil || err == context.DeadlineExceeded {
			totalTime += elapsed
			successfulIterations++
		}
	}
	
	if successfulIterations > 0 {
		return totalTime / time.Duration(successfulIterations)
	}
	return 0
}

// Legacy method for backwards compatibility
func (pf *PerformanceFramework) measureBaselineExecutionTime(engine *ExecutionEngine, tools []*Tool) time.Duration {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return pf.measureBaselineExecutionTimeWithContext(ctx, engine, tools)
}

func (pf *PerformanceFramework) measureBaselineThroughputWithContext(parentCtx context.Context, engine *ExecutionEngine, tools []*Tool) float64 {
	// Use optimized duration for CI environments
	measurements := GetOptimizedMeasurements()
	duration := measurements.ThroughputDuration
	// Further reduce in short mode
	if testing.Short() {
		duration = 1 * time.Second
	}
	ctx, cancel := context.WithTimeout(parentCtx, duration)
	defer cancel()
	
	var ops int64
	var wg sync.WaitGroup
	concurrency := 10
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					tool := tools[rand.Intn(len(tools))]
					// Use short timeout for each operation
					opCtx, opCancel := context.WithTimeout(ctx, 100*time.Millisecond)
					_, err := engine.Execute(opCtx, tool.ID, map[string]interface{}{
						"input": "throughput-test",
					})
					opCancel()
					
					if err == nil || err == context.DeadlineExceeded {
						atomic.AddInt64(&ops, 1)
					}
				}
			}
		}()
	}
	
	wg.Wait()
	return float64(atomic.LoadInt64(&ops)) / duration.Seconds()
}

// Legacy method for backwards compatibility
func (pf *PerformanceFramework) measureBaselineThroughput(engine *ExecutionEngine, tools []*Tool) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return pf.measureBaselineThroughputWithContext(ctx, engine, tools)
}

func (pf *PerformanceFramework) measureBaselineMemoryUsageWithContext(ctx context.Context, engine *ExecutionEngine, tools []*Tool) uint64 {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	
	// Execute operations with context and timeout - scale operations based on environment
	operations := 1000
	if testing.Short() {
		operations = 100 // Much fewer operations in short mode
	} else if isCI() {
		operations = 500 // Moderate operations in CI
	}
	for i := 0; i < operations; i++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			// Context cancelled, return current measurement
			runtime.GC()
			var current runtime.MemStats
			runtime.ReadMemStats(&current)
			return current.Alloc - before.Alloc
		default:
			// Continue with operation
		}
		
		tool := tools[rand.Intn(len(tools))]
		
		// Create operation context with short timeout
		opCtx, opCancel := context.WithTimeout(ctx, 100*time.Millisecond)
		_, err := engine.Execute(opCtx, tool.ID, map[string]interface{}{
			"input": fmt.Sprintf("memory-test-%d", i),
		})
		opCancel()
		
		if err != nil && err != context.DeadlineExceeded {
			// Log error but continue
			continue
		}
	}
	
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	
	return after.Alloc - before.Alloc
}

// Legacy method for backwards compatibility
func (pf *PerformanceFramework) measureBaselineMemoryUsage(engine *ExecutionEngine, tools []*Tool) uint64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return pf.measureBaselineMemoryUsageWithContext(ctx, engine, tools)
}

func (pf *PerformanceFramework) measureBaselineLatencyWithContext(ctx context.Context, engine *ExecutionEngine, tools []*Tool) *LatencyMetrics {
	var latencies []time.Duration
	// Use optimized iterations for CI
	measurements := GetOptimizedMeasurements()
	iterations := measurements.LatencyIterations
	if !isCI() {
		iterations = pf.config.BaselineIterations
	}
	
	for i := 0; i < iterations; i++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			// Context cancelled, return metrics from collected latencies
			return calculateLatencyMetrics(latencies)
		default:
			// Continue with iteration
		}
		
		tool := tools[rand.Intn(len(tools))]
		start := time.Now()
		
		// Create operation context with timeout
		opCtx, opCancel := context.WithTimeout(ctx, 1*time.Second)
		_, err := engine.Execute(opCtx, tool.ID, map[string]interface{}{
			"input": fmt.Sprintf("latency-test-%d", i),
		})
		opCancel()
		
		elapsed := time.Since(start)
		if err == nil || err == context.DeadlineExceeded {
			latencies = append(latencies, elapsed)
		}
	}
	
	return calculateLatencyMetrics(latencies)
}

// Legacy method for backwards compatibility
func (pf *PerformanceFramework) measureBaselineLatency(engine *ExecutionEngine, tools []*Tool) *LatencyMetrics {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return pf.measureBaselineLatencyWithContext(ctx, engine, tools)
}

func calculateLatencyMetrics(latencies []time.Duration) *LatencyMetrics {
	if len(latencies) == 0 {
		return &LatencyMetrics{}
	}
	
	// Sort latencies
	for i := 0; i < len(latencies); i++ {
		for j := i + 1; j < len(latencies); j++ {
			if latencies[i] > latencies[j] {
				latencies[i], latencies[j] = latencies[j], latencies[i]
			}
		}
	}
	
	metrics := &LatencyMetrics{
		Min: latencies[0],
		Max: latencies[len(latencies)-1],
	}
	
	// Calculate percentiles
	metrics.P50 = latencies[len(latencies)*50/100]
	metrics.P95 = latencies[len(latencies)*95/100]
	metrics.P99 = latencies[len(latencies)*99/100]
	metrics.P999 = latencies[len(latencies)*999/1000]
	
	// Calculate mean
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	metrics.Mean = total / time.Duration(len(latencies))
	
	return metrics
}

func (pf *PerformanceFramework) runScalabilityTest(t *testing.T, toolCount int) *PerfScalabilityMeasurement {
	t.Helper()
	
	measurement := &PerfScalabilityMeasurement{
		ToolCount: toolCount,
	}
	
	// Create registry
	registry := NewRegistry()
	
	// Measure registration time
	start := time.Now()
	tools := pf.createTestTools(toolCount)
	for _, tool := range tools {
		if err := registry.Register(tool); err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}
	}
	measurement.RegistrationTime = time.Since(start)
	
	// Measure lookup time
	start = time.Now()
	for i := 0; i < 1000; i++ {
		toolID := fmt.Sprintf("test-tool-%d", rand.Intn(toolCount))
		registry.Get(toolID)
	}
	measurement.LookupTime = time.Since(start) / 1000
	
	// Measure execution time
	engine := NewExecutionEngine(registry)
	start = time.Now()
	for i := 0; i < 100; i++ {
		toolID := fmt.Sprintf("test-tool-%d", rand.Intn(toolCount))
		engine.Execute(context.Background(), toolID, map[string]interface{}{
			"input": fmt.Sprintf("scalability-test-%d", i),
		})
	}
	measurement.ExecutionTime = time.Since(start) / 100
	
	// Measure memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	measurement.MemoryUsage = m.Alloc
	
	return measurement
}

func (pf *PerformanceFramework) runMemoryStressTest(t *testing.T, engine *ExecutionEngine, registry *Registry) *MemoryStressResult {
	t.Helper()
	
	result := &MemoryStressResult{}
	
	// Create many tools (reduced for faster execution)
	tools := pf.createTestTools(100)
	for _, tool := range tools {
		if err := registry.Register(tool); err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}
	}
	
	// Stress test with high memory allocation (reduced duration and concurrency)
	timeout := OptimizedTestTimeout()
	if testing.Short() {
		timeout = 2 * time.Second // Very short for testing.Short()
	} else if isCI() {
		timeout = 5 * time.Second // Much shorter for CI
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	var wg sync.WaitGroup
	// Scale workers based on environment
	workers := 20
	if testing.Short() {
		workers = 5 // Minimal workers for short tests
	} else if isCI() {
		workers = 10 // Moderate workers for CI
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					tool := tools[rand.Intn(len(tools))]
					engine.Execute(ctx, tool.ID, map[string]interface{}{
						"input": make([]byte, 1024*1024), // 1MB allocation
					})
				}
			}
		}()
	}
	
	wg.Wait()
	
	// Measure final memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	result.MaxMemoryUsage = m.Alloc
	result.AverageMemoryUsage = m.Alloc / 2 // Simplified
	
	return result
}

func (pf *PerformanceFramework) runMemoryLeakTest(t *testing.T, engine *ExecutionEngine, registry *Registry) *MemoryLeakResult {
	t.Helper()
	
	result := &MemoryLeakResult{}
	
	// Monitor memory over time
	var memoryReadings []uint64
	
	// Create tools
	tools := pf.createTestTools(100)
	for _, tool := range tools {
		if err := registry.Register(tool); err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}
	}
	
	// Run operations and monitor memory
	// Scale operations based on environment
	iterations := 10
	operationsPerIteration := 1000
	if testing.Short() {
		iterations = 3
		operationsPerIteration = 100
	} else if isCI() {
		iterations = 5
		operationsPerIteration = 500
	}
	for i := 0; i < iterations; i++ {
		// Execute operations
		for j := 0; j < operationsPerIteration; j++ {
			tool := tools[rand.Intn(len(tools))]
			engine.Execute(context.Background(), tool.ID, map[string]interface{}{
				"input": fmt.Sprintf("leak-test-%d-%d", i, j),
			})
		}
		
		// Force GC and measure memory
		runtime.GC()
		runtime.GC()
		
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		memoryReadings = append(memoryReadings, m.Alloc)
	}
	
	// Simple leak detection
	if len(memoryReadings) > 2 {
		firstReading := memoryReadings[0]
		lastReading := memoryReadings[len(memoryReadings)-1]
		
		// If memory consistently increases, it might be a leak
		if lastReading > firstReading*2 {
			result.LeakDetected = true
			result.LeakRate = float64(lastReading-firstReading) / float64(len(memoryReadings))
			result.LeakConfidence = 0.8 // Simplified confidence calculation
		}
	}
	
	return result
}

func (pf *PerformanceFramework) runMemoryEfficiencyTest(t *testing.T, engine *ExecutionEngine, registry *Registry) *MemoryEfficiencyResult {
	t.Helper()
	
	result := &MemoryEfficiencyResult{}
	
	// Create tools
	tools := pf.createTestTools(10)
	for _, tool := range tools {
		if err := registry.Register(tool); err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}
	}
	
	// Measure memory per operation
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	
	// Scale operations based on environment
	operations := 1000
	if testing.Short() {
		operations = 100
	} else if isCI() {
		operations = 500
	}
	for i := 0; i < operations; i++ {
		tool := tools[rand.Intn(len(tools))]
		engine.Execute(context.Background(), tool.ID, map[string]interface{}{
			"input": fmt.Sprintf("efficiency-test-%d", i),
		})
	}
	
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	
	result.MemoryPerOperation = (after.Alloc - before.Alloc) / uint64(operations)
	result.AllocationsPerOp = (after.Mallocs - before.Mallocs) / uint64(operations)
	result.GCOverhead = float64(after.GCCPUFraction) * 100
	
	return result
}

// LoadGenerator methods
func (lg *LoadGenerator) RunLoadPattern(t *testing.T, pattern LoadPattern) *LoadPatternResult {
	t.Helper()
	
	result := &LoadPatternResult{
		Pattern: pattern,
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), lg.config.LoadTestDuration)
	defer cancel()
	
	// Start result collector
	go lg.collectResults(result)
	
	// Generate load based on pattern
	switch pattern.Type {
	case LoadPatternConstant:
		lg.generateConstantLoad(ctx, pattern.Intensity)
	case LoadPatternRamp:
		lg.generateRampLoad(ctx, pattern.Intensity)
	case LoadPatternSpike:
		lg.generateSpikeLoad(ctx, pattern.Intensity)
	case LoadPatternWave:
		lg.generateWaveLoad(ctx, pattern.Intensity)
	}
	
	close(lg.resultsChan)
	
	return result
}

func (lg *LoadGenerator) generateConstantLoad(ctx context.Context, intensity int) {
	var wg sync.WaitGroup
	
	for i := 0; i < intensity; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			lg.worker(ctx, workerID)
		}(i)
	}
	
	wg.Wait()
}

func (lg *LoadGenerator) generateRampLoad(ctx context.Context, maxIntensity int) {
	_ = lg.config.LoadTestDuration // duration - might be used for future enhancements
	rampUpDuration := lg.config.RampUpDuration
	
	// Calculate intensity increase rate
	intensityIncreaseRate := float64(maxIntensity) / rampUpDuration.Seconds()
	
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	startTime := time.Now()
	var activeWorkers int
	var wg sync.WaitGroup
	
	for {
		select {
		case <-ctx.Done():
			// Wait for all workers to finish
			wg.Wait()
			return
		case <-ticker.C:
			elapsed := time.Since(startTime)
			
			var targetIntensity int
			if elapsed < rampUpDuration {
				targetIntensity = int(elapsed.Seconds() * intensityIncreaseRate)
			} else {
				targetIntensity = maxIntensity
			}
			
			// Adjust worker count
			for activeWorkers < targetIntensity {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					lg.worker(ctx, workerID)
				}(activeWorkers)
				activeWorkers++
			}
		}
	}
}

func (lg *LoadGenerator) generateSpikeLoad(ctx context.Context, spikeIntensity int) {
	// Generate spikes at regular intervals with proper cleanup
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Generate spike with timeout and cleanup
			spikeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			var wg sync.WaitGroup
			
			// Limit spike intensity to prevent resource exhaustion
			limitedIntensity := spikeIntensity
			if limitedIntensity > 100 {
				limitedIntensity = 100
			}
			
			for i := 0; i < limitedIntensity; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					lg.worker(spikeCtx, workerID)
				}(i)
			}
			
			// Ensure all workers complete before next spike
			wg.Wait()
			cancel()
		}
	}
}

func (lg *LoadGenerator) generateWaveLoad(ctx context.Context, amplitude int) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	startTime := time.Now()
	var activeWorkers int
	var wg sync.WaitGroup
	
	for {
		select {
		case <-ctx.Done():
			// Wait for all workers to finish
			wg.Wait()
			return
		case <-ticker.C:
			elapsed := time.Since(startTime).Seconds()
			
			// Sine wave pattern
			waveValue := math.Sin(elapsed / 30 * 2 * math.Pi) // 60-second period
			targetIntensity := int(float64(amplitude) * (0.5 + 0.5*waveValue))
			
			// Limit intensity to prevent resource exhaustion
			if targetIntensity > 50 {
				targetIntensity = 50
			}
			
			// Adjust worker count
			for activeWorkers < targetIntensity {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					lg.worker(ctx, workerID)
				}(activeWorkers)
				activeWorkers++
			}
		}
	}
}

func (lg *LoadGenerator) worker(ctx context.Context, workerID int) {
	// Add maximum work duration to prevent infinite loops
	maxWorkDuration := 30 * time.Second
	workTimeout := time.After(maxWorkDuration)
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-workTimeout:
			// Maximum work duration reached, exit worker
			return
		case lg.workersChan <- struct{}{}:
			// Throttle worker and execute operation
			lg.executeOperation(ctx, workerID)
			select {
			case <-lg.workersChan:
				// Successfully released throttle
			case <-ctx.Done():
				// Context cancelled while waiting to release throttle
				return
			case <-time.After(1 * time.Second):
				// Timeout waiting to release throttle, force release
				select {
				case <-lg.workersChan:
				default:
					// Channel might be full, just continue
				}
			}
		default:
			// Workers channel is full, yield and try again
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Millisecond):
				// Short delay before retrying
			}
		}
	}
}

func (lg *LoadGenerator) executeOperation(ctx context.Context, workerID int) {
	start := time.Now()
	
	// Select random tool
	tool := lg.tools[rand.Intn(len(lg.tools))]
	
	// Execute tool
	result, err := lg.engine.Execute(ctx, tool.ID, map[string]interface{}{
		"input": fmt.Sprintf("load-test-worker-%d", workerID),
	})
	
	duration := time.Since(start)
	
	// Record result
	loadResult := &LoadResult{
		Timestamp:   time.Now(),
		Duration:    duration,
		Success:     err == nil && result.Success,
		Error:       err,
		OpType:      "execute",
		WorkerID:    workerID,
		Concurrency: lg.GetActiveConcurrency(),
		MemoryUsage: lg.getCurrentMemoryUsage(),
	}
	
	select {
	case lg.resultsChan <- loadResult:
	default:
		// Skip if channel is full
	}
}

func (lg *LoadGenerator) collectResults(result *LoadPatternResult) {
	var latencies []time.Duration
	var throughputMeasurements []float64
	
	measurementWindow := 5 * time.Second
	ticker := time.NewTicker(measurementWindow)
	defer ticker.Stop()
	
	var windowOps int64
	
	for {
		select {
		case loadResult, ok := <-lg.resultsChan:
			if !ok {
				result.LatencyPercentiles = calculateLatencyMetrics(latencies)
				if len(throughputMeasurements) > 0 {
					result.AverageThroughput = average(throughputMeasurements)
					result.PeakThroughput = maxFloat64(throughputMeasurements)
				}
				return
			}
			
			result.TotalOps++
			windowOps++
			
			if loadResult.Success {
				result.SuccessfulOps++
				latencies = append(latencies, loadResult.Duration)
			} else {
				result.FailedOps++
			}
			
		case <-ticker.C:
			// Calculate throughput for this window
			throughput := float64(windowOps) / measurementWindow.Seconds()
			throughputMeasurements = append(throughputMeasurements, throughput)
			windowOps = 0
		}
	}
}

func (lg *LoadGenerator) GetActiveConcurrency() int {
	lg.mu.RLock()
	defer lg.mu.RUnlock()
	return lg.activeWorkers
}

func (lg *LoadGenerator) getCurrentMemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

// MemoryProfiler methods
func (mp *MemoryProfiler) Start() {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	
	if mp.isRunning {
		return
	}
	
	mp.isRunning = true
	go mp.monitorMemory()
}

func (mp *MemoryProfiler) Stop() {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	
	if !mp.isRunning {
		return
	}
	
	mp.isRunning = false
	close(mp.stopChan)
}

func (mp *MemoryProfiler) monitorMemory() {
	// Ensure we have a valid interval to prevent panic
	interval := mp.config.MemoryCheckInterval
	if interval <= 0 {
		interval = 1 * time.Second // Default to 1 second if not set
	}
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-mp.stopChan:
			return
		case <-ticker.C:
			mp.recordMemoryMeasurement()
		}
	}
}

func (mp *MemoryProfiler) recordMemoryMeasurement() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	measurement := MemoryMeasurement{
		Timestamp:   time.Now(),
		HeapInuse:   m.HeapInuse,
		HeapIdle:    m.HeapIdle,
		HeapAlloc:   m.HeapAlloc,
		HeapObjects: m.HeapObjects,
		StackInuse:  m.StackInuse,
		GCCycles:    m.NumGC,
		NextGC:      m.NextGC,
	}
	
	mp.mu.Lock()
	mp.measurements = append(mp.measurements, measurement)
	mp.mu.Unlock()
	
	// Update leak detector
	mp.leakDetector.AddSample(m.HeapAlloc)
}

func (mp *MemoryProfiler) GetCurrentMemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}

// LeakDetector methods
func (ld *LeakDetector) AddSample(memoryUsage uint64) {
	ld.samples = append(ld.samples, memoryUsage)
	
	// Keep only recent samples
	if len(ld.samples) > 100 {
		ld.samples = ld.samples[1:]
	}
	
	// Update growth rate
	if len(ld.samples) > 1 {
		ld.growthRate = ld.calculateGrowthRate()
	}
}

func (ld *LeakDetector) calculateGrowthRate() float64 {
	if len(ld.samples) < 2 {
		return 0
	}
	
	// Simple linear regression for growth rate
	n := len(ld.samples)
	var sumX, sumY, sumXY, sumX2 float64
	
	for i, sample := range ld.samples {
		x := float64(i)
		y := float64(sample)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	
	// Calculate slope (growth rate)
	slope := (float64(n)*sumXY - sumX*sumY) / (float64(n)*sumX2 - sumX*sumX)
	return slope
}

func (ld *LeakDetector) IsLeakDetected() bool {
	if len(ld.samples) < 10 {
		return false
	}
	
	// Check if growth rate exceeds threshold
	return ld.growthRate > float64(ld.threshold)/100.0
}

// RegressionTester methods
func (rt *RegressionTester) DetectRegressions() []PerformanceRegression {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	
	var regressions []PerformanceRegression
	
	// Check execution time regression
	if regression := rt.checkExecutionTimeRegression(); regression != nil {
		regressions = append(regressions, *regression)
	}
	
	// Check throughput regression
	if regression := rt.checkThroughputRegression(); regression != nil {
		regressions = append(regressions, *regression)
	}
	
	// Check memory regression
	if regression := rt.checkMemoryRegression(); regression != nil {
		regressions = append(regressions, *regression)
	}
	
	// Check latency regression
	if regression := rt.checkLatencyRegression(); regression != nil {
		regressions = append(regressions, *regression)
	}
	
	return regressions
}

func (rt *RegressionTester) checkExecutionTimeRegression() *PerformanceRegression {
	baselineTime := rt.baseline.ExecutionTimeBaseline.Seconds()
	
	// Calculate current average execution time
	var currentTime float64
	if len(rt.currentMetrics.ExecutionTimes) > 0 {
		var total time.Duration
		for _, t := range rt.currentMetrics.ExecutionTimes {
			total += t
		}
		currentTime = (total / time.Duration(len(rt.currentMetrics.ExecutionTimes))).Seconds()
	}
	
	if currentTime == 0 {
		return nil
	}
	
	degradation := ((currentTime - baselineTime) / baselineTime) * 100
	
	if degradation > rt.config.RegressionThreshold {
		return &PerformanceRegression{
			Metric:        "execution_time",
			BaselineValue: baselineTime,
			CurrentValue:  currentTime,
			Degradation:   degradation,
			Severity:      rt.calculateSeverity(degradation),
			Description:   fmt.Sprintf("Execution time increased by %.2f%%", degradation),
			Timestamp:     time.Now(),
			Recommendations: []string{
				"Review recent code changes",
				"Check for resource contention",
				"Analyze profiling data",
			},
		}
	}
	
	return nil
}

func (rt *RegressionTester) checkThroughputRegression() *PerformanceRegression {
	baselineThroughput := rt.baseline.ThroughputBaseline
	
	// Calculate current average throughput
	var currentThroughput float64
	if len(rt.currentMetrics.ThroughputMetrics) > 0 {
		var total float64
		for _, t := range rt.currentMetrics.ThroughputMetrics {
			total += t.Throughput
		}
		currentThroughput = total / float64(len(rt.currentMetrics.ThroughputMetrics))
	}
	
	if currentThroughput == 0 {
		return nil
	}
	
	degradation := ((baselineThroughput - currentThroughput) / baselineThroughput) * 100
	
	if degradation > rt.config.RegressionThreshold {
		return &PerformanceRegression{
			Metric:        "throughput",
			BaselineValue: baselineThroughput,
			CurrentValue:  currentThroughput,
			Degradation:   degradation,
			Severity:      rt.calculateSeverity(degradation),
			Description:   fmt.Sprintf("Throughput decreased by %.2f%%", degradation),
			Timestamp:     time.Now(),
			Recommendations: []string{
				"Check for bottlenecks",
				"Review concurrency settings",
				"Analyze system resources",
			},
		}
	}
	
	return nil
}

func (rt *RegressionTester) checkMemoryRegression() *PerformanceRegression {
	baselineMemory := float64(rt.baseline.MemoryUsageBaseline)
	
	// Calculate current average memory usage
	var currentMemory float64
	if len(rt.currentMetrics.MemoryUsage) > 0 {
		var total uint64
		for _, m := range rt.currentMetrics.MemoryUsage {
			total += m.HeapAlloc
		}
		currentMemory = float64(total) / float64(len(rt.currentMetrics.MemoryUsage))
	}
	
	if currentMemory == 0 {
		return nil
	}
	
	degradation := ((currentMemory - baselineMemory) / baselineMemory) * 100
	
	if degradation > rt.config.RegressionThreshold {
		return &PerformanceRegression{
			Metric:        "memory_usage",
			BaselineValue: baselineMemory,
			CurrentValue:  currentMemory,
			Degradation:   degradation,
			Severity:      rt.calculateSeverity(degradation),
			Description:   fmt.Sprintf("Memory usage increased by %.2f%%", degradation),
			Timestamp:     time.Now(),
			Recommendations: []string{
				"Check for memory leaks",
				"Review object lifecycle",
				"Analyze memory allocation patterns",
			},
		}
	}
	
	return nil
}

func (rt *RegressionTester) checkLatencyRegression() *PerformanceRegression {
	baselineLatency := rt.baseline.LatencyP95Baseline.Seconds()
	
	// Get current P95 latency
	currentLatency := rt.currentMetrics.LatencyPercentiles["P95"].Seconds()
	
	if currentLatency == 0 {
		return nil
	}
	
	degradation := ((currentLatency - baselineLatency) / baselineLatency) * 100
	
	if degradation > rt.config.RegressionThreshold {
		return &PerformanceRegression{
			Metric:        "latency_p95",
			BaselineValue: baselineLatency,
			CurrentValue:  currentLatency,
			Degradation:   degradation,
			Severity:      rt.calculateSeverity(degradation),
			Description:   fmt.Sprintf("P95 latency increased by %.2f%%", degradation),
			Timestamp:     time.Now(),
			Recommendations: []string{
				"Analyze slow operations",
				"Check for resource contention",
				"Review request patterns",
			},
		}
	}
	
	return nil
}

func (rt *RegressionTester) calculateSeverity(degradation float64) PerfRegressionSeverity {
	switch {
	case degradation > 50:
		return PerfRegressionSeverityCritical
	case degradation > 30:
		return PerfRegressionSeverityHigh
	case degradation > 20:
		return PerfRegressionSeverityMedium
	default:
		return PerfRegressionSeverityLow
	}
}

// Utility functions
func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func maxFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	return max
}

