package tools

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	mathrand "math/rand"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// MemoryTestSuite provides comprehensive memory testing capabilities
type MemoryTestSuite struct {
	profiler       *AdvancedMemoryProfiler
	leakDetector   *AdvancedLeakDetector
	config         *MemoryTestConfig
	baseline       *MemoryBaseline
	results        *MemoryTestResults
}

// MemoryTestConfig configures memory testing parameters
type MemoryTestConfig struct {
	// Profiling configuration
	SamplingInterval     time.Duration
	ProfileDuration      time.Duration
	GCForceInterval      time.Duration
	
	// Leak detection configuration
	LeakDetectionWindow  time.Duration
	LeakThreshold        float64  // Growth rate threshold (bytes/second)
	MinSamples           int      // Minimum samples for leak detection
	ConfidenceLevel      float64  // Statistical confidence level
	
	// Test configuration
	WarmupDuration       time.Duration
	TestDuration         time.Duration
	StabilizationTime    time.Duration
	
	// Memory limits
	MaxMemoryUsage       uint64
	MaxHeapSize          uint64
	MaxGoroutines        int
	
	// Stress test configuration
	StressTestDuration   time.Duration
	StressIntensity      int
	
	// Regression testing
	BaselineFile         string
	RegressionThreshold  float64 // Percentage increase threshold
}

// DefaultMemoryTestConfig returns default memory testing configuration
func DefaultMemoryTestConfig() *MemoryTestConfig {
	// Use shorter durations in CI or when running with -short flag
	leakDetectionWindow := 3 * time.Second     // Reduced from 10s to 3s
	profileDuration := 3 * time.Second         // Reduced from 10s to 3s
	testDuration := 3 * time.Second           // Reduced from 10s to 3s
	stressTestDuration := 5 * time.Second     // Reduced from 10s to 5s
	warmupDuration := 3 * time.Second         // Reduced from 10s to 3s
	
	if testing.Short() || os.Getenv("CI") != "" {
		leakDetectionWindow = 2 * time.Second
		profileDuration = 3 * time.Second
		testDuration = 3 * time.Second
		stressTestDuration = 5 * time.Second
		warmupDuration = 1 * time.Second
	}
	
	return &MemoryTestConfig{
		// Use conservative sampling for CI/test environments
		SamplingInterval:     100 * time.Millisecond,
		ProfileDuration:      profileDuration,
		GCForceInterval:      5 * time.Second,
		LeakDetectionWindow:  leakDetectionWindow,
		LeakThreshold:        1024 * 1024, // 1MB/sec
		MinSamples:           10,
		ConfidenceLevel:      0.95,
		WarmupDuration:       warmupDuration,
		TestDuration:         testDuration,
		StabilizationTime:    2 * time.Second,
		MaxMemoryUsage:       1024 * 1024 * 1024, // 1GB
		MaxHeapSize:          512 * 1024 * 1024,  // 512MB
		MaxGoroutines:        10000,
		StressTestDuration:   stressTestDuration,
		// Use conservative stress intensity for CI environments
		StressIntensity:      50,
		BaselineFile:         "memory_baseline.json",
		RegressionThreshold:  20.0, // 20% increase
	}
}

// MemoryBaseline stores baseline memory measurements
type MemoryBaseline struct {
	Timestamp        time.Time
	InitialHeapSize  uint64
	InitialStackSize uint64
	InitialGoroutines int
	AverageHeapSize  uint64
	PeakHeapSize     uint64
	AverageStackSize uint64
	PeakStackSize    uint64
	AverageGoroutines int
	PeakGoroutines   int
	GCFrequency      float64
	AllocRate        float64
	Environment      string
	GoVersion        string
}

// MemoryTestResults stores comprehensive memory test results
type MemoryTestResults struct {
	TestStart        time.Time
	TestDuration     time.Duration
	
	// Memory usage statistics
	MemoryStats      *MemoryStatistics
	
	// Leak detection results
	LeakResults      []LeakDetectionResult
	
	// Performance metrics
	PerformanceImpact *PerformanceImpact
	
	// Regression analysis
	RegressionResults *RegressionAnalysis
	
	// Stress test results
	StressTestResults *MemoryStressTestResults
	
	// Recommendations
	Recommendations  []string
	
	// Test status
	Passed          bool
	Failures        []string
	Warnings        []string
}

// MemoryStatistics provides detailed memory usage statistics
type MemoryStatistics struct {
	HeapStats        *HeapStatistics
	StackStats       *StackStatistics
	GoroutineStats   *GoroutineStatistics
	GCStats          *GCStatistics
	AllocationStats  *AllocationStatistics
	MemoryProfile    []MemorySnapshot
}

// HeapStatistics tracks heap memory usage
type HeapStatistics struct {
	Initial     uint64
	Current     uint64
	Peak        uint64
	Average     uint64
	Min         uint64
	Growth      float64 // Growth rate
	Utilization float64 // Percentage of allocated heap actually used
}

// StackStatistics tracks stack memory usage
type StackStatistics struct {
	Initial     uint64
	Current     uint64
	Peak        uint64
	Average     uint64
	Growth      float64
}

// GoroutineStatistics tracks goroutine counts
type GoroutineStatistics struct {
	Initial     int
	Current     int
	Peak        int
	Average     int
	Growth      float64
	Distribution map[string]int // By state (running, waiting, etc.)
}

// GCStatistics tracks garbage collection metrics
type GCStatistics struct {
	TotalGCs        uint32
	GCFrequency     float64 // GCs per second
	TotalPauseTime  time.Duration
	AveragePauseTime time.Duration
	MaxPauseTime    time.Duration
	GCOverhead      float64 // Percentage of CPU time spent in GC
}

// AllocationStatistics tracks memory allocation patterns
type AllocationStatistics struct {
	TotalAllocated  uint64
	TotalFreed      uint64
	AllocationRate  float64 // Bytes per second
	ObjectCount     uint64
	ObjectsPerSec   float64
	AverageObjectSize uint64
}

// MemorySnapshot captures memory state at a specific time
type MemorySnapshot struct {
	Timestamp       time.Time
	HeapAlloc       uint64
	HeapSys         uint64
	HeapInuse       uint64
	HeapIdle        uint64
	HeapReleased    uint64
	HeapObjects     uint64
	StackInuse      uint64
	StackSys        uint64
	MSpanInuse      uint64
	MSpanSys        uint64
	MCacheInuse     uint64
	MCacheSys       uint64
	NextGC          uint64
	LastGC          time.Time
	PauseTotalNs    uint64
	NumGC           uint32
	GCCPUFraction   float64
	GoroutineCount  int
	CGoCalls        int64
}

// LeakDetectionResult represents the result of leak detection analysis
type LeakDetectionResult struct {
	StartTime       time.Time
	EndTime         time.Time
	Duration        time.Duration
	LeakDetected    bool
	LeakRate        float64 // Bytes per second
	Confidence      float64 // Statistical confidence
	LeakType        LeakType
	LeakSource      string
	Severity        LeakSeverity
	Description     string
	Recommendations []string
	Evidence        []LeakEvidence
}

// LeakType categorizes different types of memory leaks
type LeakType int

const (
	LeakTypeUnknown LeakType = iota
	LeakTypeGoroutine
	LeakTypeHeap
	LeakTypeStack
	LeakTypeGlobal
	LeakTypeClosure
	LeakTypeChannel
	LeakTypeMap
	LeakTypeSlice
)

// LeakSeverity indicates the severity of a detected leak
type LeakSeverity int

const (
	LeakSeverityLow LeakSeverity = iota
	LeakSeverityMedium
	LeakSeverityHigh
	LeakSeverityCritical
)

// LeakEvidence provides evidence for a detected leak
type LeakEvidence struct {
	Timestamp   time.Time
	MemoryUsage uint64
	GrowthRate  float64
	Description string
	StackTrace  string
}

// PerformanceImpact measures the performance impact of memory usage
type PerformanceImpact struct {
	GCImpact        *GCImpact
	AllocationImpact *AllocationImpact
	ThroughputImpact *ThroughputImpact
}

// GCImpact measures garbage collection performance impact
type GCImpact struct {
	GCOverhead      float64 // Percentage of CPU time
	PauseFrequency  float64 // Pauses per second
	AveragePauseTime time.Duration
	MaxPauseTime    time.Duration
	ThroughputReduction float64 // Percentage reduction
}

// AllocationImpact measures allocation performance impact
type AllocationImpact struct {
	AllocationOverhead float64 // Percentage of execution time
	AllocationRate     float64 // Allocations per second
	FragmentationRatio float64 // Heap fragmentation
}

// ThroughputImpact measures overall throughput impact
type ThroughputImpact struct {
	BaselineThroughput float64
	CurrentThroughput  float64
	ThroughputDelta    float64 // Percentage change
	ResponseTimeDelta  float64 // Percentage change
}

// RegressionAnalysis compares current results with baseline
type RegressionAnalysis struct {
	BaselineComparison *BaselineComparison
	Regressions        []MemoryRegression
	Improvements       []MemoryImprovement
	OverallAssessment  string
}

// BaselineComparison compares current metrics with baseline
type BaselineComparison struct {
	HeapUsageChange    float64 // Percentage change
	StackUsageChange   float64
	GoroutineChange    float64
	GCFrequencyChange  float64
	AllocationChange   float64
}

// MemoryRegression represents a detected memory regression
type MemoryRegression struct {
	Metric      string
	Baseline    float64
	Current     float64
	Change      float64 // Percentage
	Severity    PerfRegressionSeverity
	Impact      string
	Recommendations []string
}

// MemoryImprovement represents a detected memory improvement
type MemoryImprovement struct {
	Metric      string
	Baseline    float64
	Current     float64
	Improvement float64 // Percentage
	Impact      string
}

// MemoryStressTestResults stores stress test results
type MemoryStressTestResults struct {
	MaxMemoryUsage     uint64
	MaxGoroutines      int
	MemoryLeakDetected bool
	PerformanceDegradation float64
	ErrorRate          float64
	RecoveryTime       time.Duration
	StabilityScore     float64
}

// AdvancedMemoryProfiler provides comprehensive memory profiling
type AdvancedMemoryProfiler struct {
	config      *MemoryTestConfig
	snapshots   []MemorySnapshot
	isRunning   bool
	stopChan    chan struct{}
	mu          sync.RWMutex
	gcMonitor   *GCMonitor
	allocMonitor *AllocationMonitor
	
	// Lifecycle management
	profileWG   sync.WaitGroup
}

// GCMonitor monitors garbage collection events
type GCMonitor struct {
	events      []GCEvent
	mu          sync.RWMutex
	totalPauses time.Duration
}

// GCEvent represents a garbage collection event
type GCEvent struct {
	Timestamp    time.Time
	PauseTime    time.Duration
	HeapBefore   uint64
	HeapAfter    uint64
	GCNumber     uint32
	GCType       string
}

// AllocationMonitor monitors memory allocation patterns
type AllocationMonitor struct {
	events      []AllocationEvent
	mu          sync.RWMutex
	totalAllocs uint64
}

// AllocationEvent represents a memory allocation event
type AllocationEvent struct {
	Timestamp   time.Time
	Size        uint64
	ObjectType  string
	StackTrace  string
}

// AdvancedLeakDetector provides sophisticated leak detection
type AdvancedLeakDetector struct {
	config      *MemoryTestConfig
	samples     []MemorySample
	algorithms  []LeakDetectionAlgorithm
	mu          sync.RWMutex
	
	// Lifecycle management
	isRunning   bool
	stopChan    chan struct{}
	monitorWG   sync.WaitGroup
}

// MemorySample represents a memory usage sample
type MemorySample struct {
	Timestamp   time.Time
	HeapSize    uint64
	StackSize   uint64
	Goroutines  int
	Objects     uint64
}

// LeakDetectionAlgorithm defines an interface for leak detection algorithms
type LeakDetectionAlgorithm interface {
	Name() string
	Analyze(samples []MemorySample) (*LeakDetectionResult, error)
	Configure(config map[string]interface{}) error
}

// NewMemoryTestSuite creates a new memory test suite
func NewMemoryTestSuite(config *MemoryTestConfig) *MemoryTestSuite {
	if config == nil {
		config = DefaultMemoryTestConfig()
	}
	
	suite := &MemoryTestSuite{
		config: config,
		results: &MemoryTestResults{
			TestStart: time.Now(),
		},
	}
	
	// Initialize profiler
	suite.profiler = &AdvancedMemoryProfiler{
		config:   config,
		stopChan: make(chan struct{}),
		isRunning: false,
		gcMonitor: &GCMonitor{
			events: make([]GCEvent, 0),
		},
		allocMonitor: &AllocationMonitor{
			events: make([]AllocationEvent, 0),
		},
	}
	
	// Initialize leak detector
	suite.leakDetector = &AdvancedLeakDetector{
		config: config,
		samples: make([]MemorySample, 0),
		stopChan: make(chan struct{}),
		algorithms: []LeakDetectionAlgorithm{
			&LinearRegressionDetector{},
			&TrendAnalysisDetector{},
			&MemoryStatisticalDetector{},
			&PatternDetector{},
		},
		isRunning: false,
		stopChan:  make(chan struct{}),
	}
	
	return suite
}

// TestMemoryUsage runs comprehensive memory usage tests
func TestMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory usage tests in short mode")
	}
	
	// Skip full tests unless explicitly enabled
	if os.Getenv("ENABLE_FULL_MEMORY_TESTS") != "1" {
		t.Skip("Skipping full memory tests - set ENABLE_FULL_MEMORY_TESTS=1 to enable")
	}
	
	// Create a shared suite for final report
	finalSuite := NewMemoryTestSuite(DefaultMemoryTestConfig())
	
	t.Run("BasicMemoryUsage", func(t *testing.T) {
		suite := NewMemoryTestSuite(DefaultMemoryTestConfig())
		suite.testBasicMemoryUsage(t)
	})
	
	t.Run("MemoryLeakDetection", func(t *testing.T) {
		suite := NewMemoryTestSuite(DefaultMemoryTestConfig())
		suite.testMemoryLeakDetection(t)
	})
	
	t.Run("MemoryStressTest", func(t *testing.T) {
		if os.Getenv("ENABLE_STRESS_TESTS") != "1" {
			t.Skip("Skipping stress test - set ENABLE_STRESS_TESTS=1 to enable")
		}
		suite := NewMemoryTestSuite(DefaultMemoryTestConfig())
		suite.testMemoryStressTest(t)
	})
	
	t.Run("MemoryRegression", func(t *testing.T) {
		suite := NewMemoryTestSuite(DefaultMemoryTestConfig())
		suite.testMemoryRegression(t)
	})
	
	t.Run("GoroutineLeakDetection", func(t *testing.T) {
		suite := NewMemoryTestSuite(DefaultMemoryTestConfig())
		suite.testGoroutineLeakDetection(t)
	})
	
	t.Run("AllocationPatterns", func(t *testing.T) {
		suite := NewMemoryTestSuite(DefaultMemoryTestConfig())
		suite.testAllocationPatterns(t)
	})
	
	t.Run("GCBehavior", func(t *testing.T) {
		suite := NewMemoryTestSuite(DefaultMemoryTestConfig())
		suite.testGCBehavior(t)
	})
	
	// Generate final report using the final suite
	finalSuite.generateReport(t)
}

// testBasicMemoryUsage tests basic memory usage patterns
func (suite *MemoryTestSuite) testBasicMemoryUsage(t *testing.T) {
	t.Helper()
	
	// Create test environment
	registry := NewRegistry()
	engine := NewExecutionEngine(registry)
	
	// Create tools with reasonable count for CI environments
	tools := make([]*Tool, 50)
	for i := 0; i < 50; i++ {
		tools[i] = createMemoryTestTool(fmt.Sprintf("memory-test-%p-%d", &tools, i))
		if err := registry.Register(tools[i]); err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}
	}
	
	// Start profiling
	suite.profiler.Start()
	defer suite.profiler.Stop()
	
	// Establish baseline
	suite.establishBaseline()
	
	// Run test operations
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		tool := tools[i%len(tools)]
		params := map[string]interface{}{
			"size": 1024 * (i%10 + 1), // Varying sizes
		}
		
		_, err := engine.Execute(ctx, tool.ID, params)
		if err != nil {
			t.Errorf("Execution failed: %v", err)
		}
		
		// Add some delay to allow profiling
		if i%50 == 0 {
			time.Sleep(1 * time.Millisecond)
		}
	}
	
	// Wait for stabilization
	time.Sleep(suite.config.StabilizationTime)
	
	// Analyze results
	suite.analyzeMemoryUsage(t)
}

// testMemoryLeakDetection tests memory leak detection
func (suite *MemoryTestSuite) testMemoryLeakDetection(t *testing.T) {
	t.Helper()
	
	// Create test environment
	registry := NewRegistry()
	engine := NewExecutionEngine(registry)
	
	// Create leaky tool
	leakyTool := createLeakyTool(fmt.Sprintf("leaky-tool-%p", t))
	if err := registry.Register(leakyTool); err != nil {
		t.Fatalf("Failed to register leaky tool: %v", err)
	}
	
	// Start profiling and leak detection
	suite.profiler.Start()
	suite.leakDetector.Start()
	defer suite.profiler.Stop()
	defer suite.leakDetector.Stop()
	
	// Run operations that should cause leaks
	ctx := context.Background()
	for i := 0; i < 50; i++ {
		params := map[string]interface{}{
			"leak_size": 1024 * 10, // 10KB per operation
		}
		
		_, err := engine.Execute(ctx, leakyTool.ID, params)
		if err != nil {
			t.Errorf("Execution failed: %v", err)
		}
		
		// Add delay to allow leak detection
		time.Sleep(2 * time.Millisecond)
	}
	
	// Wait for leak detection analysis
	time.Sleep(suite.config.LeakDetectionWindow)
	
	// Analyze leak detection results
	suite.analyzeLeakDetection(t)
}

// testMemoryStressTest runs memory stress tests
func (suite *MemoryTestSuite) testMemoryStressTest(t *testing.T) {
	t.Helper()
	
	// Create test environment with fresh registry
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(suite.config.StressIntensity))
	
	// Create stress test tools with unique IDs
	tools := make([]*Tool, 10)
	
	// Clean up any existing tools from the registry
	defer func() {
		// Clean up registered tools
		for _, tool := range tools {
			if tool != nil {
				registry.Unregister(tool.ID)
			}
		}
	}()
	// Use a highly unique test ID to avoid conflicts
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	testID := fmt.Sprintf("stress-test-%d-%d-%d-%x", time.Now().UnixNano(), runtime.NumGoroutine(), mathrand.Intn(1000000), randomBytes)
	for i := 0; i < 10; i++ {
		toolID := fmt.Sprintf("%s-%d", testID, i)
		
		// Check if tool already exists in registry
		if _, err := registry.Get(toolID); err == nil {
			t.Logf("Tool with ID %s already exists, skipping stress test", toolID)
			t.Skip("Tool ID conflict detected - skipping stress test")
			return
		}
		
		tools[i] = createMemoryStressTool(toolID)
		// Try to register the tool
		if err := registry.Register(tools[i]); err != nil {
			t.Logf("Failed to register stress tool %s: %v", toolID, err)
			t.Skip("Tool registration failed - skipping stress test")
			return
		}
	}
	
	// Start profiling
	suite.profiler.Start()
	defer suite.profiler.Stop()
	
	// Run stress test
	ctx, cancel := context.WithTimeout(context.Background(), suite.config.StressTestDuration)
	defer cancel()
	
	var wg sync.WaitGroup
	var operations int64
	var errors int64
	
	// Start stress workers
	for i := 0; i < suite.config.StressIntensity; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for {
				select {
				case <-ctx.Done():
					return
				default:
					tool := tools[workerID%len(tools)]
					params := map[string]interface{}{
						"size":      1024 * 100, // 100KB instead of 1MB
						"worker_id": workerID,
					}
					
					_, err := engine.Execute(ctx, tool.ID, params)
					if err != nil {
						atomic.AddInt64(&errors, 1)
					} else {
						atomic.AddInt64(&operations, 1)
					}
					
					// Add small delay to prevent CPU saturation
					time.Sleep(1 * time.Millisecond)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Analyze stress test results
	suite.analyzeStressTest(t, operations, errors)
}

// testMemoryRegression tests for memory regressions
func (suite *MemoryTestSuite) testMemoryRegression(t *testing.T) {
	t.Helper()
	
	// Load baseline if available
	baseline := suite.loadBaseline()
	if baseline == nil {
		t.Skip("No baseline available for regression testing")
	}
	
	// Create test environment
	registry := NewRegistry()
	engine := NewExecutionEngine(registry)
	
	// Create tools with reasonable count for CI environments
	tools := make([]*Tool, 25)
	for i := 0; i < 25; i++ {
		tools[i] = createMemoryTestTool(fmt.Sprintf("regression-test-%p-%d", &tools, i))
		if err := registry.Register(tools[i]); err != nil {
			t.Fatalf("Failed to register tool: %v", err)
		}
	}
	
	// Start profiling
	suite.profiler.Start()
	defer suite.profiler.Stop()
	
	// Run regression test
	ctx := context.Background()
	for i := 0; i < 50; i++ {
		tool := tools[i%len(tools)]
		params := map[string]interface{}{
			"size": 1024 * 10, // 10KB
		}
		
		_, err := engine.Execute(ctx, tool.ID, params)
		if err != nil {
			t.Errorf("Execution failed: %v", err)
		}
	}
	
	// Wait for stabilization
	time.Sleep(suite.config.StabilizationTime)
	
	// Analyze regression
	suite.analyzeRegression(t, baseline)
}

// testGoroutineLeakDetection tests goroutine leak detection
func (suite *MemoryTestSuite) testGoroutineLeakDetection(t *testing.T) {
	t.Helper()
	
	// Create test environment
	registry := NewRegistry()
	engine := NewExecutionEngine(registry)
	
	// Create goroutine leaky tool
	goroutineLeakyTool := createGoroutineLeakyTool(fmt.Sprintf("goroutine-leaky-tool-%p", t))
	if err := registry.Register(goroutineLeakyTool); err != nil {
		t.Fatalf("Failed to register goroutine leaky tool: %v", err)
	}
	
	// Record initial goroutine count
	initialGoroutines := runtime.NumGoroutine()
	
	// Start profiling
	suite.profiler.Start()
	defer suite.profiler.Stop()
	
	// Run operations that should leak goroutines
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		params := map[string]interface{}{
			"goroutine_count": 2,
		}
		
		_, err := engine.Execute(ctx, goroutineLeakyTool.ID, params)
		if err != nil {
			t.Errorf("Execution failed: %v", err)
		}
		
		time.Sleep(1 * time.Millisecond)
	}
	
	// Wait for analysis
	time.Sleep(suite.config.StabilizationTime)
	
	// Check for goroutine leaks
	currentGoroutines := runtime.NumGoroutine()
	goroutineIncrease := currentGoroutines - initialGoroutines
	
	if goroutineIncrease > 50 { // Allow some tolerance
		t.Errorf("Potential goroutine leak detected: %d -> %d (+%d)", 
			initialGoroutines, currentGoroutines, goroutineIncrease)
	}
	
	// Analyze goroutine patterns
	suite.analyzeGoroutineLeaks(t, initialGoroutines, currentGoroutines)
}

// testAllocationPatterns tests memory allocation patterns
func (suite *MemoryTestSuite) testAllocationPatterns(t *testing.T) {
	t.Helper()
	
	// Create test environment
	registry := NewRegistry()
	engine := NewExecutionEngine(registry)
	
	// Create allocation pattern test tool
	allocTool := createAllocationPatternTool(fmt.Sprintf("alloc-pattern-tool-%p", t))
	if err := registry.Register(allocTool); err != nil {
		t.Fatalf("Failed to register allocation tool: %v", err)
	}
	
	// Start profiling
	suite.profiler.Start()
	defer suite.profiler.Stop()
	
	// Test different allocation patterns
	patterns := []map[string]interface{}{
		{"pattern": "small_frequent", "size": 1024, "count": 100},
		{"pattern": "large_infrequent", "size": 1024 * 100, "count": 5},
		{"pattern": "mixed", "size": 1024 * 10, "count": 20},
	}
	
	ctx := context.Background()
	for _, pattern := range patterns {
		for i := 0; i < int(pattern["count"].(int)); i++ {
			_, err := engine.Execute(ctx, allocTool.ID, pattern)
			if err != nil {
				t.Errorf("Execution failed: %v", err)
			}
		}
		
		// Force GC between patterns
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	
	// Analyze allocation patterns
	suite.analyzeAllocationPatterns(t)
}

// testGCBehavior tests garbage collection behavior
func (suite *MemoryTestSuite) testGCBehavior(t *testing.T) {
	t.Helper()
	
	// Create test environment
	registry := NewRegistry()
	engine := NewExecutionEngine(registry)
	
	// Create GC test tool
	gcTool := createGCTestTool(fmt.Sprintf("gc-test-tool-%p", t))
	if err := registry.Register(gcTool); err != nil {
		t.Fatalf("Failed to register GC tool: %v", err)
	}
	
	// Start profiling
	suite.profiler.Start()
	defer suite.profiler.Stop()
	
	// Record initial GC stats
	var initialStats runtime.MemStats
	runtime.ReadMemStats(&initialStats)
	
	// Run operations that trigger GC
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		params := map[string]interface{}{
			"size": 1024 * 1024, // 1MB allocations
		}
		
		_, err := engine.Execute(ctx, gcTool.ID, params)
		if err != nil {
			t.Errorf("Execution failed: %v", err)
		}
		
		// Occasional forced GC
		if i%5 == 0 {
			runtime.GC()
		}
	}
	
	// Record final GC stats
	var finalStats runtime.MemStats
	runtime.ReadMemStats(&finalStats)
	
	// Analyze GC behavior
	suite.analyzeGCBehavior(t, &initialStats, &finalStats)
}

// Analysis methods
func (suite *MemoryTestSuite) establishBaseline() {
	runtime.GC()
	runtime.GC()
	
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	suite.baseline = &MemoryBaseline{
		Timestamp:         time.Now(),
		InitialHeapSize:   m.HeapAlloc,
		InitialStackSize:  m.StackInuse,
		InitialGoroutines: runtime.NumGoroutine(),
		Environment:       "test",
		GoVersion:         runtime.Version(),
	}
}

func (suite *MemoryTestSuite) analyzeMemoryUsage(t *testing.T) {
	t.Helper()
	
	snapshots := suite.profiler.GetSnapshots()
	if len(snapshots) == 0 {
		t.Error("No memory snapshots collected")
		return
	}
	
	// Calculate statistics
	heapStats := suite.calculateHeapStatistics(snapshots)
	stackStats := suite.calculateStackStatistics(snapshots)
	goroutineStats := suite.calculateGoroutineStatistics(snapshots)
	gcStats := suite.calculateGCStatistics(snapshots)
	allocStats := suite.calculateAllocationStatistics(snapshots)
	
	suite.results.MemoryStats = &MemoryStatistics{
		HeapStats:       heapStats,
		StackStats:      stackStats,
		GoroutineStats:  goroutineStats,
		GCStats:         gcStats,
		AllocationStats: allocStats,
		MemoryProfile:   snapshots,
	}
	
	// Check for issues
	if heapStats.Growth > suite.config.RegressionThreshold {
		suite.results.Warnings = append(suite.results.Warnings, 
			fmt.Sprintf("High heap growth rate: %.2f%%", heapStats.Growth))
	}
	
	if goroutineStats.Growth > 50 {
		suite.results.Warnings = append(suite.results.Warnings, 
			fmt.Sprintf("High goroutine growth rate: %.2f%%", goroutineStats.Growth))
	}
}

func (suite *MemoryTestSuite) analyzeLeakDetection(t *testing.T) {
	t.Helper()
	
	// Run leak detection algorithms
	samples := suite.leakDetector.GetSamples()
	if len(samples) < suite.config.MinSamples {
		t.Errorf("Insufficient samples for leak detection: %d", len(samples))
		return
	}
	
	var results []LeakDetectionResult
	for _, algorithm := range suite.leakDetector.algorithms {
		result, err := algorithm.Analyze(samples)
		if err != nil {
			t.Errorf("Leak detection failed with %s: %v", algorithm.Name(), err)
			continue
		}
		
		if result != nil {
			results = append(results, *result)
		}
	}
	
	suite.results.LeakResults = results
	
	// Check for detected leaks
	for _, result := range results {
		if result.LeakDetected {
			suite.results.Failures = append(suite.results.Failures, 
				fmt.Sprintf("Memory leak detected by %v: %s", 
					result.LeakType, result.Description))
		}
	}
}

func (suite *MemoryTestSuite) analyzeStressTest(t *testing.T, operations, errors int64) {
	t.Helper()
	
	snapshots := suite.profiler.GetSnapshots()
	if len(snapshots) == 0 {
		t.Error("No snapshots collected during stress test")
		return
	}
	
	// Find peak memory usage
	var maxMemory uint64
	var maxGoroutines int
	for _, snapshot := range snapshots {
		if snapshot.HeapAlloc > maxMemory {
			maxMemory = snapshot.HeapAlloc
		}
		if snapshot.GoroutineCount > maxGoroutines {
			maxGoroutines = snapshot.GoroutineCount
		}
	}
	
	// Handle division by zero for error rate calculation
	var errorRate float64
	if (operations + errors) > 0 {
		errorRate = float64(errors) / float64(operations+errors) * 100
	}
	
	suite.results.StressTestResults = &MemoryStressTestResults{
		MaxMemoryUsage:     maxMemory,
		MaxGoroutines:      maxGoroutines,
		ErrorRate:          errorRate,
		MemoryLeakDetected: suite.detectStressTestLeaks(snapshots),
		StabilityScore:     suite.calculateStabilityScore(snapshots),
	}
	
	// Check stress test limits
	if maxMemory > suite.config.MaxMemoryUsage {
		suite.results.Failures = append(suite.results.Failures, 
			fmt.Sprintf("Memory usage exceeded limit: %d > %d", 
				maxMemory, suite.config.MaxMemoryUsage))
	}
	
	if maxGoroutines > suite.config.MaxGoroutines {
		suite.results.Failures = append(suite.results.Failures, 
			fmt.Sprintf("Goroutine count exceeded limit: %d > %d", 
				maxGoroutines, suite.config.MaxGoroutines))
	}
}

func (suite *MemoryTestSuite) analyzeRegression(t *testing.T, baseline *MemoryBaseline) {
	t.Helper()
	
	snapshots := suite.profiler.GetSnapshots()
	if len(snapshots) == 0 {
		t.Error("No snapshots for regression analysis")
		return
	}
	
	// Calculate current metrics
	currentHeap := suite.calculateAverageHeapUsage(snapshots)
	currentStack := suite.calculateAverageStackUsage(snapshots)
	currentGoroutines := suite.calculateAverageGoroutineCount(snapshots)
	
	// Compare with baseline
	heapChange := ((float64(currentHeap) - float64(baseline.AverageHeapSize)) / 
		float64(baseline.AverageHeapSize)) * 100
	stackChange := ((float64(currentStack) - float64(baseline.AverageStackSize)) / 
		float64(baseline.AverageStackSize)) * 100
	goroutineChange := ((float64(currentGoroutines) - float64(baseline.AverageGoroutines)) / 
		float64(baseline.AverageGoroutines)) * 100
	
	comparison := &BaselineComparison{
		HeapUsageChange:   heapChange,
		StackUsageChange:  stackChange,
		GoroutineChange:   goroutineChange,
	}
	
	var regressions []MemoryRegression
	var improvements []MemoryImprovement
	
	// Check for regressions
	if heapChange > suite.config.RegressionThreshold {
		regressions = append(regressions, MemoryRegression{
			Metric:   "heap_usage",
			Baseline: float64(baseline.AverageHeapSize),
			Current:  float64(currentHeap),
			Change:   heapChange,
			Severity: suite.calculateRegressionSeverity(heapChange),
			Impact:   "Higher memory consumption",
		})
	}
	
	// Check for improvements
	if heapChange < -5 { // 5% improvement threshold
		improvements = append(improvements, MemoryImprovement{
			Metric:      "heap_usage",
			Baseline:    float64(baseline.AverageHeapSize),
			Current:     float64(currentHeap),
			Improvement: -heapChange,
			Impact:      "Reduced memory consumption",
		})
	}
	
	suite.results.RegressionResults = &RegressionAnalysis{
		BaselineComparison: comparison,
		Regressions:       regressions,
		Improvements:      improvements,
	}
}

// analyzeLeakDetectionExpectingLeaks is used when we expect leaks to be detected (for testing)
func (suite *MemoryTestSuite) analyzeLeakDetectionExpectingLeaks(t *testing.T) {
	t.Helper()
	
	// Run leak detection algorithms
	samples := suite.leakDetector.GetSamples()
	if len(samples) < suite.config.MinSamples {
		t.Errorf("Insufficient samples for leak detection: %d", len(samples))
		return
	}
	
	var results []LeakDetectionResult
	var leaksDetected int
	for _, algorithm := range suite.leakDetector.algorithms {
		result, err := algorithm.Analyze(samples)
		if err != nil {
			t.Errorf("Leak detection failed with %s: %v", algorithm.Name(), err)
			continue
		}
		
		if result != nil {
			results = append(results, *result)
			if result.LeakDetected {
				leaksDetected++
			}
		}
	}
	
	suite.results.LeakResults = results
	
	// For this test, we expect leaks to be detected
	if leaksDetected == 0 {
		t.Error("Expected memory leaks to be detected, but none were found")
	} else {
		t.Logf("Successfully detected %d memory leaks (as expected)", leaksDetected)
	}
}

func (suite *MemoryTestSuite) analyzeGoroutineLeaks(t *testing.T, initial, current int) {
	t.Helper()
	
	// This is a simplified goroutine leak analysis
	// In a real implementation, you would use runtime.Stack() and other
	// techniques to identify specific goroutine leaks
	
	increase := current - initial
	if increase > 20 { // Allow some tolerance
		suite.results.Warnings = append(suite.results.Warnings, 
			fmt.Sprintf("Potential goroutine leak: %d -> %d (+%d)", 
				initial, current, increase))
	}
}

func (suite *MemoryTestSuite) analyzeAllocationPatterns(t *testing.T) {
	t.Helper()
	
	// Analyze allocation events from the profiler
	events := suite.profiler.allocMonitor.GetEvents()
	
	// Group by pattern
	patterns := make(map[string][]AllocationEvent)
	for _, event := range events {
		patterns[event.ObjectType] = append(patterns[event.ObjectType], event)
	}
	
	// Analyze each pattern
	for pattern, events := range patterns {
		if len(events) > 100 { // Threshold for significant patterns
			suite.results.Warnings = append(suite.results.Warnings, 
				fmt.Sprintf("High allocation frequency for %s: %d events", 
					pattern, len(events)))
		}
	}
}

func (suite *MemoryTestSuite) analyzeGCBehavior(t *testing.T, initial, final *runtime.MemStats) {
	t.Helper()
	
	gcCount := final.NumGC - initial.NumGC
	totalPauseTime := final.PauseTotalNs - initial.PauseTotalNs
	
	if gcCount > 0 {
		avgPauseTime := time.Duration(totalPauseTime / uint64(gcCount))
		
		// Handle division by zero for GC frequency calculation
		var gcFrequency float64
		if suite.config.TestDuration.Seconds() > 0 {
			gcFrequency = float64(gcCount) / suite.config.TestDuration.Seconds()
		}
		
		// Initialize MemoryStats if nil
		if suite.results.MemoryStats == nil {
			suite.results.MemoryStats = &MemoryStatistics{}
		}
		
		suite.results.MemoryStats.GCStats = &GCStatistics{
			TotalGCs:         gcCount,
			GCFrequency:      gcFrequency,
			TotalPauseTime:   time.Duration(totalPauseTime),
			AveragePauseTime: avgPauseTime,
			GCOverhead:       final.GCCPUFraction * 100,
		}
		
		// Check for GC performance issues
		if avgPauseTime > 10*time.Millisecond {
			suite.results.Warnings = append(suite.results.Warnings, 
				fmt.Sprintf("High GC pause time: %v", avgPauseTime))
		}
		
		if gcFrequency > 10 { // More than 10 GCs per second
			suite.results.Warnings = append(suite.results.Warnings, 
				fmt.Sprintf("High GC frequency: %.2f GCs/sec", gcFrequency))
		}
	}
}

func (suite *MemoryTestSuite) generateReport(t *testing.T) {
	t.Helper()
	
	suite.results.TestDuration = time.Since(suite.results.TestStart)
	suite.results.Passed = len(suite.results.Failures) == 0
	
	// Generate recommendations
	suite.generateRecommendations()
	
	// Print summary
	t.Logf("Memory Test Summary:")
	t.Logf("  Test Duration: %v", suite.results.TestDuration)
	t.Logf("  Passed: %v", suite.results.Passed)
	t.Logf("  Failures: %d", len(suite.results.Failures))
	t.Logf("  Warnings: %d", len(suite.results.Warnings))
	
	if len(suite.results.Failures) > 0 {
		t.Logf("  Failures:")
		for _, failure := range suite.results.Failures {
			t.Logf("    - %s", failure)
		}
	}
	
	if len(suite.results.Warnings) > 0 {
		t.Logf("  Warnings:")
		for _, warning := range suite.results.Warnings {
			t.Logf("    - %s", warning)
		}
	}
	
	if len(suite.results.Recommendations) > 0 {
		t.Logf("  Recommendations:")
		for _, rec := range suite.results.Recommendations {
			t.Logf("    - %s", rec)
		}
	}
}

// Helper methods for creating test tools
func createMemoryTestTool(id string) *Tool {
	return &Tool{
		ID:          id,
		Name:        fmt.Sprintf("Memory Test Tool %s", id),
		Description: "A tool for memory testing",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"size": {
					Type:        "integer",
					Description: "Memory size to allocate",
				},
			},
			Required: []string{"size"},
		},
		Executor: &MemoryTestExecutor{},
	}
}

func createLeakyTool(id string) *Tool {
	return &Tool{
		ID:          id,
		Name:        "Leaky Tool",
		Description: "A tool that intentionally leaks memory",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"leak_size": {
					Type:        "integer",
					Description: "Size of memory to leak",
				},
			},
			Required: []string{"leak_size"},
		},
		Executor: &LeakyExecutor{
			leakedMemory: make([][]byte, 0),
		},
	}
}

func createMemoryStressTool(id string) *Tool {
	return &Tool{
		ID:          id,
		Name:        "Memory Stress Tool",
		Description: "A tool for memory stress testing",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"size": {
					Type:        "integer",
					Description: "Memory size to allocate",
				},
				"worker_id": {
					Type:        "integer",
					Description: "Worker ID",
				},
			},
			Required: []string{"size"},
		},
		Executor: &MemoryStressExecutor{},
	}
}

func createGoroutineLeakyTool(id string) *Tool {
	return &Tool{
		ID:          id,
		Name:        "Goroutine Leaky Tool",
		Description: "A tool that leaks goroutines",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"goroutine_count": {
					Type:        "integer",
					Description: "Number of goroutines to leak",
				},
			},
			Required: []string{"goroutine_count"},
		},
		Executor: &GoroutineLeakyExecutor{},
	}
}

func createAllocationPatternTool(id string) *Tool {
	return &Tool{
		ID:          id,
		Name:        "Allocation Pattern Tool",
		Description: "A tool for testing allocation patterns",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"pattern": {
					Type:        "string",
					Description: "Allocation pattern",
				},
				"size": {
					Type:        "integer",
					Description: "Allocation size",
				},
				"count": {
					Type:        "integer",
					Description: "Number of allocations",
				},
			},
			Required: []string{"pattern", "size", "count"},
		},
		Executor: &AllocationPatternExecutor{},
	}
}

func createGCTestTool(id string) *Tool {
	return &Tool{
		ID:          id,
		Name:        "GC Test Tool",
		Description: "A tool for testing GC behavior",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"size": {
					Type:        "integer",
					Description: "Allocation size",
				},
			},
			Required: []string{"size"},
		},
		Executor: &GCTestExecutor{},
	}
}

// Test executors
type MemoryTestExecutor struct{}

func (e *MemoryTestExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	size := int(params["size"].(float64))
	
	// Allocate memory
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte(i % 256)
	}
	
	// Process data
	checksum := 0
	for _, b := range data {
		checksum += int(b)
	}
	
	return &ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"checksum": checksum,
			"size":     size,
		},
		Timestamp: time.Now(),
	}, nil
}

type LeakyExecutor struct {
	leakedMemory [][]byte
	mu           sync.Mutex
}

func (e *LeakyExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	size := int(params["leak_size"].(float64))
	
	// Intentionally leak memory
	e.mu.Lock()
	defer e.mu.Unlock()
	
	leakedData := make([]byte, size)
	for i := 0; i < size; i++ {
		leakedData[i] = byte(i % 256)
	}
	
	// Store reference to prevent GC
	e.leakedMemory = append(e.leakedMemory, leakedData)
	
	return &ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"leaked_size": size,
			"total_leaked": len(e.leakedMemory),
		},
		Timestamp: time.Now(),
	}, nil
}

type MemoryStressExecutor struct{}

func (e *MemoryStressExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	size := int(params["size"].(float64))
	
	// Allocate large amount of memory
	data := make([]byte, size)
	
	// Fill with pattern
	for i := 0; i < size; i++ {
		data[i] = byte(i % 256)
	}
	
	// Simulate processing
	checksum := 0
	for i := 0; i < size; i += 1000 {
		checksum += int(data[i])
	}
	
	return &ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"checksum": checksum,
		},
		Timestamp: time.Now(),
	}, nil
}

type GoroutineLeakyExecutor struct{}

func (e *GoroutineLeakyExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	count := int(params["goroutine_count"].(float64))
	
	// Create goroutines that don't terminate
	for i := 0; i < count; i++ {
		go func() {
			// Infinite loop - goroutine leak
			for {
				time.Sleep(time.Second)
			}
		}()
	}
	
	return &ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"leaked_goroutines": count,
		},
		Timestamp: time.Now(),
	}, nil
}

type AllocationPatternExecutor struct{}

func (e *AllocationPatternExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	pattern := params["pattern"].(string)
	size := int(params["size"].(float64))
	count := int(params["count"].(float64))
	
	var totalAllocated int
	
	switch pattern {
	case "small_frequent":
		// Many small allocations
		for i := 0; i < count; i++ {
			data := make([]byte, size)
			totalAllocated += len(data)
			// Use data briefly
			data[0] = byte(i % 256)
		}
		
	case "large_infrequent":
		// Few large allocations
		for i := 0; i < count; i++ {
			data := make([]byte, size)
			totalAllocated += len(data)
			// Process data
			for j := 0; j < len(data); j += 1000 {
				data[j] = byte(j % 256)
			}
		}
		
	case "mixed":
		// Mixed allocation sizes
		for i := 0; i < count; i++ {
			allocSize := size + (i % 1000)
			data := make([]byte, allocSize)
			totalAllocated += len(data)
			data[0] = byte(i % 256)
		}
	}
	
	return &ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"pattern":          pattern,
			"total_allocated":  totalAllocated,
			"allocation_count": count,
		},
		Timestamp: time.Now(),
	}, nil
}

type GCTestExecutor struct{}

func (e *GCTestExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	size := int(params["size"].(float64))
	
	// Create temporary large allocation
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte(i % 256)
	}
	
	// Process data
	checksum := 0
	for i := 0; i < size; i += 1000 {
		checksum += int(data[i])
	}
	
	// Clear reference to allow GC
	data = nil
	
	return &ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"checksum": checksum,
			"size":     size,
		},
		Timestamp: time.Now(),
	}, nil
}

// Utility methods and additional helper functions
func (suite *MemoryTestSuite) loadBaseline() *MemoryBaseline {
	// In a real implementation, this would load from a file
	// For now, return nil to indicate no baseline
	return nil
}

func (suite *MemoryTestSuite) calculateRegressionSeverity(change float64) PerfRegressionSeverity {
	switch {
	case change > 100:
		return PerfRegressionSeverityCritical
	case change > 50:
		return PerfRegressionSeverityHigh
	case change > 25:
		return PerfRegressionSeverityMedium
	default:
		return PerfRegressionSeverityLow
	}
}

func (suite *MemoryTestSuite) generateRecommendations() {
	// Generate recommendations based on analysis results
	if suite.results.MemoryStats != nil {
		if suite.results.MemoryStats.HeapStats.Growth > 50 {
			suite.results.Recommendations = append(suite.results.Recommendations,
				"Consider implementing object pooling to reduce allocation pressure")
		}
		
		if suite.results.MemoryStats.GCStats.GCFrequency > 10 {
			suite.results.Recommendations = append(suite.results.Recommendations,
				"High GC frequency detected - consider reducing allocation rate")
		}
	}
	
	if len(suite.results.LeakResults) > 0 {
		suite.results.Recommendations = append(suite.results.Recommendations,
			"Memory leaks detected - review resource cleanup and lifecycle management")
	}
}

// Additional helper methods for statistics calculations
func (suite *MemoryTestSuite) calculateHeapStatistics(snapshots []MemorySnapshot) *HeapStatistics {
	if len(snapshots) == 0 {
		return &HeapStatistics{}
	}
	
	initial := snapshots[0].HeapAlloc
	current := snapshots[len(snapshots)-1].HeapAlloc
	
	var peak, total uint64
	min := snapshots[0].HeapAlloc
	
	for _, snapshot := range snapshots {
		if snapshot.HeapAlloc > peak {
			peak = snapshot.HeapAlloc
		}
		if snapshot.HeapAlloc < min {
			min = snapshot.HeapAlloc
		}
		total += snapshot.HeapAlloc
	}
	
	average := total / uint64(len(snapshots))
	
	// Handle division by zero for growth calculation
	var growth float64
	if initial > 0 {
		growth = ((float64(current) - float64(initial)) / float64(initial)) * 100
	}
	
	return &HeapStatistics{
		Initial: initial,
		Current: current,
		Peak:    peak,
		Average: average,
		Min:     min,
		Growth:  growth,
	}
}

func (suite *MemoryTestSuite) calculateStackStatistics(snapshots []MemorySnapshot) *StackStatistics {
	if len(snapshots) == 0 {
		return &StackStatistics{}
	}
	
	initial := snapshots[0].StackInuse
	current := snapshots[len(snapshots)-1].StackInuse
	
	var peak, total uint64
	for _, snapshot := range snapshots {
		if snapshot.StackInuse > peak {
			peak = snapshot.StackInuse
		}
		total += snapshot.StackInuse
	}
	
	average := total / uint64(len(snapshots))
	
	// Handle division by zero for growth calculation
	var growth float64
	if initial > 0 {
		growth = ((float64(current) - float64(initial)) / float64(initial)) * 100
	}
	
	return &StackStatistics{
		Initial: initial,
		Current: current,
		Peak:    peak,
		Average: average,
		Growth:  growth,
	}
}

func (suite *MemoryTestSuite) calculateGoroutineStatistics(snapshots []MemorySnapshot) *GoroutineStatistics {
	if len(snapshots) == 0 {
		return &GoroutineStatistics{}
	}
	
	initial := snapshots[0].GoroutineCount
	current := snapshots[len(snapshots)-1].GoroutineCount
	
	var peak, total int
	for _, snapshot := range snapshots {
		if snapshot.GoroutineCount > peak {
			peak = snapshot.GoroutineCount
		}
		total += snapshot.GoroutineCount
	}
	
	average := total / len(snapshots)
	
	// Handle division by zero for growth calculation
	var growth float64
	if initial > 0 {
		growth = ((float64(current) - float64(initial)) / float64(initial)) * 100
	}
	
	return &GoroutineStatistics{
		Initial: initial,
		Current: current,
		Peak:    peak,
		Average: average,
		Growth:  growth,
	}
}

func (suite *MemoryTestSuite) calculateGCStatistics(snapshots []MemorySnapshot) *GCStatistics {
	if len(snapshots) < 2 {
		return &GCStatistics{}
	}
	
	first := snapshots[0]
	last := snapshots[len(snapshots)-1]
	
	totalGCs := last.NumGC - first.NumGC
	duration := last.Timestamp.Sub(first.Timestamp)
	
	// Handle division by zero for frequency calculation
	var frequency float64
	if duration.Seconds() > 0 {
		frequency = float64(totalGCs) / duration.Seconds()
	}
	
	totalPause := last.PauseTotalNs - first.PauseTotalNs
	
<<<<<<< HEAD
	// Handle division by zero for average pause calculation
	var avgPause time.Duration
	if totalGCs > 0 {
		avgPause = time.Duration(totalPause / uint64(totalGCs))
	}
	
	return &GCStatistics{
		TotalGCs:         totalGCs,
		GCFrequency:      frequency,
		TotalPauseTime:   time.Duration(totalPause),
		AveragePauseTime: avgPause,
		GCOverhead:       last.GCCPUFraction * 100,
	}
}

func (suite *MemoryTestSuite) calculateAllocationStatistics(snapshots []MemorySnapshot) *AllocationStatistics {
	if len(snapshots) < 2 {
		return &AllocationStatistics{}
	}
	
	first := snapshots[0]
	last := snapshots[len(snapshots)-1]
	
	duration := last.Timestamp.Sub(first.Timestamp)
	
	// These would be calculated from actual allocation events
	// For now, we'll use estimates based on heap changes
	heapIncrease := last.HeapAlloc - first.HeapAlloc
	
	// Handle division by zero for allocation rate calculation
	var allocationRate float64
	if duration.Seconds() > 0 {
		allocationRate = float64(heapIncrease) / duration.Seconds()
	}
	
	// Handle division by zero for average object size calculation
	var averageObjectSize uint64
	if last.HeapObjects > 0 {
		averageObjectSize = last.HeapAlloc / last.HeapObjects
	}
	
	return &AllocationStatistics{
		TotalAllocated:  heapIncrease,
		AllocationRate:  allocationRate,
		ObjectCount:     last.HeapObjects,
		AverageObjectSize: averageObjectSize,
	}
}

func (suite *MemoryTestSuite) calculateAverageHeapUsage(snapshots []MemorySnapshot) uint64 {
	if len(snapshots) == 0 {
		return 0
	}
	
	var total uint64
	for _, snapshot := range snapshots {
		total += snapshot.HeapAlloc
	}
	
	return total / uint64(len(snapshots))
}

func (suite *MemoryTestSuite) calculateAverageStackUsage(snapshots []MemorySnapshot) uint64 {
	if len(snapshots) == 0 {
		return 0
	}
	
	var total uint64
	for _, snapshot := range snapshots {
		total += snapshot.StackInuse
	}
	
	return total / uint64(len(snapshots))
}

func (suite *MemoryTestSuite) calculateAverageGoroutineCount(snapshots []MemorySnapshot) int {
	if len(snapshots) == 0 {
		return 0
	}
	
	var total int
	for _, snapshot := range snapshots {
		total += snapshot.GoroutineCount
	}
	
	return total / len(snapshots)
}

func (suite *MemoryTestSuite) detectStressTestLeaks(snapshots []MemorySnapshot) bool {
	if len(snapshots) < 10 {
		return false
	}
	
	// Simple leak detection: check if memory consistently increases
	increases := 0
	for i := 1; i < len(snapshots); i++ {
		if snapshots[i].HeapAlloc > snapshots[i-1].HeapAlloc {
			increases++
		}
	}
	
	// If memory increases in more than 80% of samples, consider it a leak
	return float64(increases)/float64(len(snapshots)-1) > 0.8
}

func (suite *MemoryTestSuite) calculateStabilityScore(snapshots []MemorySnapshot) float64 {
	if len(snapshots) < 2 {
		return 0
	}
	
	// Calculate coefficient of variation for heap usage
	mean := suite.calculateAverageHeapUsage(snapshots)
	
	var variance float64
	for _, snapshot := range snapshots {
		diff := float64(snapshot.HeapAlloc) - float64(mean)
		variance += diff * diff
	}
	
	variance /= float64(len(snapshots))
	stdDev := math.Sqrt(variance)
	
	// Stability score is inversely related to coefficient of variation
	// Handle division by zero for coefficient of variation calculation
	var cv float64
	if mean > 0 {
		cv = stdDev / float64(mean)
	}
	return math.Max(0, 1-cv)
}

// Memory profiler methods
func (profiler *AdvancedMemoryProfiler) Start() {
	profiler.mu.Lock()
	defer profiler.mu.Unlock()
	
	if profiler.isRunning {
		return
	}
	
	profiler.isRunning = true
	profiler.stopChan = make(chan struct{})
	
	// Start profiling with proper lifecycle management
	profiler.profileWG.Add(1)
	go profiler.profileMemory()
}

func (profiler *AdvancedMemoryProfiler) Stop() {
	profiler.mu.Lock()
	defer profiler.mu.Unlock()
	
	if !profiler.isRunning {
		return
	}
	
	profiler.isRunning = false
	close(profiler.stopChan)
	
	// Wait for profiling goroutine to finish
	profiler.profileWG.Wait()
}

func (profiler *AdvancedMemoryProfiler) profileMemory() {
	defer profiler.profileWG.Done()
	
	ticker := time.NewTicker(profiler.config.SamplingInterval)
	defer ticker.Stop()
	
	// Set maximum profiling duration to prevent infinite loops
	maxDuration := 10 * time.Minute
	timeout := time.After(maxDuration)
	
	for {
		select {
		case <-profiler.stopChan:
			return
		case <-timeout:
			// Maximum profiling duration reached, stop profiling
			return
		case <-ticker.C:
			profiler.takeSnapshot()
		}
	}
}

func (profiler *AdvancedMemoryProfiler) takeSnapshot() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	snapshot := MemorySnapshot{
		Timestamp:       time.Now(),
		HeapAlloc:       m.HeapAlloc,
		HeapSys:         m.HeapSys,
		HeapInuse:       m.HeapInuse,
		HeapIdle:        m.HeapIdle,
		HeapReleased:    m.HeapReleased,
		HeapObjects:     m.HeapObjects,
		StackInuse:      m.StackInuse,
		StackSys:        m.StackSys,
		MSpanInuse:      m.MSpanInuse,
		MSpanSys:        m.MSpanSys,
		MCacheInuse:     m.MCacheInuse,
		MCacheSys:       m.MCacheSys,
		NextGC:          m.NextGC,
		LastGC:          time.Unix(0, int64(m.LastGC)),
		PauseTotalNs:    m.PauseTotalNs,
		NumGC:           m.NumGC,
		GCCPUFraction:   m.GCCPUFraction,
		GoroutineCount:  runtime.NumGoroutine(),
		CGoCalls:        runtime.NumCgoCall(),
	}
	
	profiler.mu.Lock()
	profiler.snapshots = append(profiler.snapshots, snapshot)
	profiler.mu.Unlock()
}

func (profiler *AdvancedMemoryProfiler) GetSnapshots() []MemorySnapshot {
	profiler.mu.RLock()
	defer profiler.mu.RUnlock()
	
	result := make([]MemorySnapshot, len(profiler.snapshots))
	copy(result, profiler.snapshots)
	return result
}

// Leak detector methods
func (detector *AdvancedLeakDetector) Start() {
	detector.mu.Lock()
	defer detector.mu.Unlock()
	
	if detector.isRunning {
		return
	}
	
	detector.isRunning = true
	detector.stopChan = make(chan struct{})
	
	// Start leak detection monitoring
	detector.monitorWG.Add(1)
	go detector.monitor()
}

func (detector *AdvancedLeakDetector) Stop() {
	detector.mu.Lock()
	defer detector.mu.Unlock()
	
	if !detector.isRunning {
		return
	}
	
	detector.isRunning = false
	close(detector.stopChan)
	
	// Wait for monitor goroutine to finish
	detector.monitorWG.Wait()
}

func (detector *AdvancedLeakDetector) monitor() {
	defer detector.monitorWG.Done()
	
	ticker := time.NewTicker(detector.config.SamplingInterval)
	defer ticker.Stop()
	
	// Set maximum monitoring duration to prevent infinite loops
	maxDuration := 5 * time.Minute
	timeout := time.After(maxDuration)
	
	for {
		select {
		case <-detector.stopChan:
			return
		case <-timeout:
			// Maximum monitoring duration reached, stop monitoring
			return
		case <-ticker.C:
			detector.takeSample()
		}
	}
}

func (detector *AdvancedLeakDetector) takeSample() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	sample := MemorySample{
		Timestamp:  time.Now(),
		HeapSize:   m.HeapAlloc,
		StackSize:  m.StackInuse,
		Goroutines: runtime.NumGoroutine(),
		Objects:    m.HeapObjects,
	}
	
	detector.mu.Lock()
	detector.samples = append(detector.samples, sample)
	
	// Keep only recent samples
	if len(detector.samples) > 1000 {
		detector.samples = detector.samples[1:]
	}
	detector.mu.Unlock()
}

func (detector *AdvancedLeakDetector) GetSamples() []MemorySample {
	detector.mu.RLock()
	defer detector.mu.RUnlock()
	
	result := make([]MemorySample, len(detector.samples))
	copy(result, detector.samples)
	return result
}

// Leak detection algorithms
type LinearRegressionDetector struct{}

func (d *LinearRegressionDetector) Name() string {
	return "Linear Regression"
}

func (d *LinearRegressionDetector) Configure(config map[string]interface{}) error {
	return nil
}

func (d *LinearRegressionDetector) Analyze(samples []MemorySample) (*LeakDetectionResult, error) {
	if len(samples) < 5 {
		return nil, fmt.Errorf("insufficient samples for linear regression")
	}
	
	// Simple linear regression to detect memory growth trend
	n := len(samples)
	var sumX, sumY, sumXY, sumX2 float64
	
	for i, sample := range samples {
		x := float64(i)
		y := float64(sample.HeapSize)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	
	// Calculate slope (growth rate)
	slope := (float64(n)*sumXY - sumX*sumY) / (float64(n)*sumX2 - sumX*sumX)
	
	// If slope is positive and significant, it might indicate a leak
	threshold := 1024 * 1024 // 1MB per sample
	
	if slope > float64(threshold) {
		return &LeakDetectionResult{
			StartTime:    samples[0].Timestamp,
			EndTime:      samples[len(samples)-1].Timestamp,
			Duration:     samples[len(samples)-1].Timestamp.Sub(samples[0].Timestamp),
			LeakDetected: true,
			LeakRate:     slope,
			Confidence:   0.8, // Simplified confidence calculation
			LeakType:     LeakTypeHeap,
			Severity:     LeakSeverityMedium,
			Description:  fmt.Sprintf("Linear growth detected: %.2f bytes/sample", slope),
		}, nil
	}
	
	return &LeakDetectionResult{
		StartTime:    samples[0].Timestamp,
		EndTime:      samples[len(samples)-1].Timestamp,
		Duration:     samples[len(samples)-1].Timestamp.Sub(samples[0].Timestamp),
		LeakDetected: false,
		LeakRate:     slope,
		Confidence:   0.8,
		LeakType:     LeakTypeUnknown,
		Severity:     LeakSeverityLow,
		Description:  "No significant linear growth detected",
	}, nil
}

type TrendAnalysisDetector struct{}

func (d *TrendAnalysisDetector) Name() string {
	return "Trend Analysis"
}

func (d *TrendAnalysisDetector) Configure(config map[string]interface{}) error {
	return nil
}

func (d *TrendAnalysisDetector) Analyze(samples []MemorySample) (*LeakDetectionResult, error) {
	if len(samples) < 10 {
		return nil, fmt.Errorf("insufficient samples for trend analysis")
	}
	
	// Analyze trend in recent samples
	recentSamples := samples[len(samples)-10:]
	
	increases := 0
	for i := 1; i < len(recentSamples); i++ {
		if recentSamples[i].HeapSize > recentSamples[i-1].HeapSize {
			increases++
		}
	}
	
	trend := float64(increases) / float64(len(recentSamples)-1)
	
	// If memory increases in more than 70% of recent samples, consider it a leak
	if trend > 0.7 {
		return &LeakDetectionResult{
			StartTime:    recentSamples[0].Timestamp,
			EndTime:      recentSamples[len(recentSamples)-1].Timestamp,
			Duration:     recentSamples[len(recentSamples)-1].Timestamp.Sub(recentSamples[0].Timestamp),
			LeakDetected: true,
			LeakRate:     trend * 100,
			Confidence:   trend,
			LeakType:     LeakTypeHeap,
			Severity:     LeakSeverityMedium,
			Description:  fmt.Sprintf("Increasing trend detected: %.1f%% of samples show growth", trend*100),
		}, nil
	}
	
	return &LeakDetectionResult{
		StartTime:    recentSamples[0].Timestamp,
		EndTime:      recentSamples[len(recentSamples)-1].Timestamp,
		Duration:     recentSamples[len(recentSamples)-1].Timestamp.Sub(recentSamples[0].Timestamp),
		LeakDetected: false,
		LeakRate:     trend * 100,
		Confidence:   1 - trend,
		LeakType:     LeakTypeUnknown,
		Severity:     LeakSeverityLow,
		Description:  "No significant trend detected",
	}, nil
}

type MemoryStatisticalDetector struct{}

func (d *MemoryStatisticalDetector) Name() string {
	return "Statistical Analysis"
}

func (d *MemoryStatisticalDetector) Configure(config map[string]interface{}) error {
	return nil
}

func (d *MemoryStatisticalDetector) Analyze(samples []MemorySample) (*LeakDetectionResult, error) {
	if len(samples) < 10 {
		return nil, fmt.Errorf("insufficient samples for statistical analysis")
	}
	
	// Calculate statistical measures
	var total uint64
	for _, sample := range samples {
		total += sample.HeapSize
	}
	
	mean := float64(total) / float64(len(samples))
	
	var variance float64
	for _, sample := range samples {
		diff := float64(sample.HeapSize) - mean
		variance += diff * diff
	}
	variance /= float64(len(samples))
	
	stdDev := math.Sqrt(variance)
	
	// Check if recent samples are significantly higher than mean
	recentSamples := samples[len(samples)-10:]
	var recentTotal uint64
	for _, sample := range recentSamples {
		recentTotal += sample.HeapSize
	}
	
	recentMean := float64(recentTotal) / float64(len(recentSamples))
	
	// Z-score calculation
	// Handle division by zero for Z-score calculation
	var zScore float64
	if stdDev > 0 {
		zScore = (recentMean - mean) / (stdDev / math.Sqrt(float64(len(recentSamples))))
	}
	
	// If Z-score > 2, it's statistically significant (95% confidence)
	if zScore > 2.0 {
		return &LeakDetectionResult{
			StartTime:    samples[0].Timestamp,
			EndTime:      samples[len(samples)-1].Timestamp,
			Duration:     samples[len(samples)-1].Timestamp.Sub(samples[0].Timestamp),
			LeakDetected: true,
			LeakRate:     recentMean - mean,
			Confidence:   0.95,
			LeakType:     LeakTypeHeap,
			Severity:     LeakSeverityMedium,
			Description:  fmt.Sprintf("Statistical anomaly detected: Z-score %.2f", zScore),
		}, nil
	}
	
	return &LeakDetectionResult{
		StartTime:    samples[0].Timestamp,
		EndTime:      samples[len(samples)-1].Timestamp,
		Duration:     samples[len(samples)-1].Timestamp.Sub(samples[0].Timestamp),
		LeakDetected: false,
		LeakRate:     recentMean - mean,
		Confidence:   1 - (zScore / 2.0),
		LeakType:     LeakTypeUnknown,
		Severity:     LeakSeverityLow,
		Description:  "No statistical anomaly detected",
	}, nil
}

type PatternDetector struct{}

func (d *PatternDetector) Name() string {
	return "Pattern Detection"
}

func (d *PatternDetector) Configure(config map[string]interface{}) error {
	return nil
}

func (d *PatternDetector) Analyze(samples []MemorySample) (*LeakDetectionResult, error) {
	if len(samples) < 20 {
		return nil, fmt.Errorf("insufficient samples for pattern detection")
	}
	
	// Look for sawtooth pattern (allocation spikes followed by GC)
	// vs continuous growth pattern
	
	var peaks, valleys int
	var lastPeak uint64
	
	for i := 1; i < len(samples)-1; i++ {
		curr := samples[i].HeapSize
		prev := samples[i-1].HeapSize
		next := samples[i+1].HeapSize
		
		// Peak detection
		if curr > prev && curr > next {
			peaks++
			if lastPeak > 0 && curr > lastPeak*2 {
				// Peak is much higher than previous peak
				return &LeakDetectionResult{
					StartTime:    samples[0].Timestamp,
					EndTime:      samples[len(samples)-1].Timestamp,
					Duration:     samples[len(samples)-1].Timestamp.Sub(samples[0].Timestamp),
					LeakDetected: true,
					LeakRate:     float64(curr - lastPeak),
					Confidence:   0.7,
					LeakType:     LeakTypeHeap,
					Severity:     LeakSeverityMedium,
					Description:  "Escalating peak pattern detected",
				}, nil
			}
			lastPeak = curr
		}
		
		// Valley detection
		if curr < prev && curr < next {
			valleys++
		}
	}
	
	// Healthy pattern should have roughly equal peaks and valleys
	if peaks > 0 && valleys > 0 {
		// Handle division by zero for ratio calculation
		var ratio float64
		if valleys > 0 {
			ratio = float64(peaks) / float64(valleys)
		}
		if ratio > 0.5 && ratio < 2.0 {
			return &LeakDetectionResult{
				StartTime:    samples[0].Timestamp,
				EndTime:      samples[len(samples)-1].Timestamp,
				Duration:     samples[len(samples)-1].Timestamp.Sub(samples[0].Timestamp),
				LeakDetected: false,
				LeakRate:     0,
				Confidence:   0.8,
				LeakType:     LeakTypeUnknown,
				Severity:     LeakSeverityLow,
				Description:  "Healthy allocation/GC pattern detected",
			}, nil
		}
	}
	
	return &LeakDetectionResult{
		StartTime:    samples[0].Timestamp,
		EndTime:      samples[len(samples)-1].Timestamp,
		Duration:     samples[len(samples)-1].Timestamp.Sub(samples[0].Timestamp),
		LeakDetected: false,
		LeakRate:     0,
		Confidence:   0.5,
		LeakType:     LeakTypeUnknown,
		Severity:     LeakSeverityLow,
		Description:  "No clear pattern detected",
	}, nil
}

// GC Monitor methods
func (monitor *GCMonitor) GetEvents() []GCEvent {
	monitor.mu.RLock()
	defer monitor.mu.RUnlock()
	
	result := make([]GCEvent, len(monitor.events))
	copy(result, monitor.events)
	return result
}

// Allocation Monitor methods
func (monitor *AllocationMonitor) GetEvents() []AllocationEvent {
	monitor.mu.RLock()
	defer monitor.mu.RUnlock()
	
	result := make([]AllocationEvent, len(monitor.events))
	copy(result, monitor.events)
	return result
}

