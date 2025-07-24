//go:build examples
// +build examples

package encoding

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// TestSuite represents a test suite configuration
type TestSuite struct {
	Name        string
	TestFiles   []string
	Tags        []string
	Timeout     time.Duration
	Parallel    bool
	Verbose     bool
	Short       bool
	Description string
}

// TestRunner manages and executes test suites
type TestRunner struct {
	suites []TestSuite
	config RunnerConfig
}

// RunnerConfig contains configuration for the test runner
type RunnerConfig struct {
	Verbose    bool
	Short      bool
	Parallel   bool
	Timeout    time.Duration
	OutputFile string
	JUnitFile  string
	Coverage   bool
	Race       bool
	Benchmarks bool
}

// NewTestRunner creates a new test runner
func NewTestRunner() *TestRunner {
	return &TestRunner{
		suites: []TestSuite{},
		config: RunnerConfig{
			Timeout:  30 * time.Minute,
			Parallel: true,
			Coverage: true,
			Race:     true,
		},
	}
}

// AddSuite adds a test suite to the runner
func (tr *TestRunner) AddSuite(suite TestSuite) {
	tr.suites = append(tr.suites, suite)
}

// SetConfig sets the runner configuration
func (tr *TestRunner) SetConfig(config RunnerConfig) {
	tr.config = config
}

// RunAll runs all test suites
func (tr *TestRunner) RunAll() error {
	fmt.Printf("Running %d test suites...\n", len(tr.suites))
	
	var results []SuiteResult
	
	for _, suite := range tr.suites {
		fmt.Printf("\n=== Running %s ===\n", suite.Name)
		fmt.Printf("Description: %s\n", suite.Description)
		
		result := tr.runSuite(suite)
		results = append(results, result)
		
		if result.Error != nil {
			fmt.Printf("❌ Suite %s failed: %v\n", suite.Name, result.Error)
		} else {
			fmt.Printf("✅ Suite %s passed in %s\n", suite.Name, result.Duration)
		}
	}
	
	// Print summary
	tr.printSummary(results)
	
	// Check for failures
	for _, result := range results {
		if result.Error != nil {
			return fmt.Errorf("test suite failures detected")
		}
	}
	
	return nil
}

// SuiteResult represents the result of running a test suite
type SuiteResult struct {
	Name     string
	Duration time.Duration
	Error    error
	Output   string
}

// runSuite runs a single test suite
func (tr *TestRunner) runSuite(suite TestSuite) SuiteResult {
	start := time.Now()
	
	// Build command
	args := []string{"test"}
	
	// Add test files
	for _, file := range suite.TestFiles {
		args = append(args, file)
	}
	
	// Add flags
	if tr.config.Verbose || suite.Verbose {
		args = append(args, "-v")
	}
	
	if tr.config.Short || suite.Short {
		args = append(args, "-short")
	}
	
	if tr.config.Parallel || suite.Parallel {
		args = append(args, "-parallel", "8")
	}
	
	if tr.config.Race {
		args = append(args, "-race")
	}
	
	if tr.config.Coverage {
		args = append(args, "-cover")
	}
	
	// Set timeout
	timeout := tr.config.Timeout
	if suite.Timeout > 0 {
		timeout = suite.Timeout
	}
	args = append(args, "-timeout", timeout.String())
	
	// Add tags
	if len(suite.Tags) > 0 {
		args = append(args, "-tags", strings.Join(suite.Tags, ","))
	}
	
	// Execute
	cmd := exec.Command("go", args...)
	output, err := cmd.CombinedOutput()
	
	return SuiteResult{
		Name:     suite.Name,
		Duration: time.Since(start),
		Error:    err,
		Output:   string(output),
	}
}

// printSummary prints a summary of all test results
func (tr *TestRunner) printSummary(results []SuiteResult) {
	fmt.Print("\n" + strings.Repeat("=", 80) + "\n")
	fmt.Print("TEST SUMMARY\n")
	fmt.Print(strings.Repeat("=", 80) + "\n")
	
	var totalDuration time.Duration
	var passed, failed int
	
	for _, result := range results {
		totalDuration += result.Duration
		
		status := "✅ PASS"
		if result.Error != nil {
			status = "❌ FAIL"
			failed++
		} else {
			passed++
		}
		
		fmt.Printf("%-30s %s (%s)\n", result.Name, status, result.Duration)
	}
	
	fmt.Print(strings.Repeat("-", 80) + "\n")
	fmt.Printf("Total: %d, Passed: %d, Failed: %d\n", len(results), passed, failed)
	fmt.Printf("Total Time: %s\n", totalDuration)
	
	if failed > 0 {
		fmt.Printf("\nFAILED SUITES:\n")
		for _, result := range results {
			if result.Error != nil {
				fmt.Printf("- %s: %v\n", result.Name, result.Error)
			}
		}
	}
}

// GetDefaultTestSuites returns the default test suites for the encoding system
func GetDefaultTestSuites() []TestSuite {
	return []TestSuite{
		{
			Name:        "Unit Tests",
			TestFiles:   []string{"./..."},
			Tags:        []string{"unit"},
			Timeout:     10 * time.Minute,
			Parallel:    true,
			Verbose:     false,
			Short:       false,
			Description: "Core unit tests for all encoding components",
		},
		{
			Name:        "Integration Tests",
			TestFiles:   []string{"./comprehensive_integration_test.go"},
			Tags:        []string{"integration"},
			Timeout:     15 * time.Minute,
			Parallel:    true,
			Verbose:     true,
			Short:       false,
			Description: "End-to-end integration tests verifying complete encoding pipeline",
		},
		{
			Name:        "Concurrency Tests",
			TestFiles:   []string{"./comprehensive_concurrency_test.go"},
			Tags:        []string{"concurrency"},
			Timeout:     20 * time.Minute,
			Parallel:    false, // Don't run concurrency tests in parallel
			Verbose:     true,
			Short:       false,
			Description: "Thread safety and race condition tests",
		},
		{
			Name:        "Performance Benchmarks",
			TestFiles:   []string{"./comprehensive_benchmark_test.go"},
			Tags:        []string{"benchmark"},
			Timeout:     30 * time.Minute,
			Parallel:    true,
			Verbose:     true,
			Short:       false,
			Description: "Performance benchmarks for encoding/decoding operations",
		},
		{
			Name:        "Regression Tests",
			TestFiles:   []string{"./comprehensive_regression_test.go"},
			Tags:        []string{"regression"},
			Timeout:     10 * time.Minute,
			Parallel:    true,
			Verbose:     true,
			Short:       false,
			Description: "Regression tests for content negotiation and format handling",
		},
		{
			Name:        "End-to-End Tests",
			TestFiles:   []string{"./comprehensive_e2e_test.go"},
			Tags:        []string{"e2e"},
			Timeout:     20 * time.Minute,
			Parallel:    true,
			Verbose:     true,
			Short:       false,
			Description: "Real-world usage scenarios and workflow tests",
		},
		{
			Name:        "Error Handling Tests",
			TestFiles:   []string{"./comprehensive_error_test.go"},
			Tags:        []string{"error"},
			Timeout:     15 * time.Minute,
			Parallel:    true,
			Verbose:     true,
			Short:       false,
			Description: "Comprehensive error handling and edge case tests",
		},
		{
			Name:        "Pool Tests",
			TestFiles:   []string{"./updated_pool_test.go"},
			Tags:        []string{"pool"},
			Timeout:     10 * time.Minute,
			Parallel:    true,
			Verbose:     true,
			Short:       false,
			Description: "Updated pool implementation tests with new interfaces",
		},
	}
}

// RunEncodingTests runs all encoding system tests
func RunEncodingTests() error {
	fmt.Println("AG-UI Go SDK Encoding System - Comprehensive Test Suite")
	fmt.Println("========================================================")
	
	runner := NewTestRunner()
	
	// Configure runner
	config := RunnerConfig{
		Verbose:    true,
		Short:      false,
		Parallel:   true,
		Timeout:    45 * time.Minute,
		Coverage:   true,
		Race:       true,
		Benchmarks: true,
	}
	
	// Check for environment variables
	if os.Getenv("SHORT") == "true" {
		config.Short = true
		config.Timeout = 10 * time.Minute
	}
	
	if os.Getenv("VERBOSE") == "false" {
		config.Verbose = false
	}
	
	if os.Getenv("NO_RACE") == "true" {
		config.Race = false
	}
	
	runner.SetConfig(config)
	
	// Add all test suites
	for _, suite := range GetDefaultTestSuites() {
		// Skip long-running tests in short mode
		if config.Short && (suite.Name == "Performance Benchmarks" || suite.Name == "Concurrency Tests") {
			fmt.Printf("Skipping %s in short mode\n", suite.Name)
			continue
		}
		
		runner.AddSuite(suite)
	}
	
	// Run all tests
	return runner.RunAll()
}

// RunBenchmarkOnly runs only the benchmark tests
func RunBenchmarkOnly() error {
	fmt.Println("Running Performance Benchmarks Only")
	fmt.Println("===================================")
	
	runner := NewTestRunner()
	
	config := RunnerConfig{
		Verbose:    true,
		Short:      false,
		Parallel:   true,
		Timeout:    30 * time.Minute,
		Coverage:   false,
		Race:       false,
		Benchmarks: true,
	}
	
	runner.SetConfig(config)
	
	// Add only benchmark suite
	benchmarkSuite := TestSuite{
		Name:        "Performance Benchmarks",
		TestFiles:   []string{"./comprehensive_benchmark_test.go"},
		Tags:        []string{"benchmark"},
		Timeout:     30 * time.Minute,
		Parallel:    true,
		Verbose:     true,
		Short:       false,
		Description: "Performance benchmarks for encoding/decoding operations",
	}
	
	runner.AddSuite(benchmarkSuite)
	
	return runner.RunAll()
}

// RunQuickTests runs a quick subset of tests for development
func RunQuickTests() error {
	fmt.Println("Running Quick Test Suite")
	fmt.Println("========================")
	
	runner := NewTestRunner()
	
	config := RunnerConfig{
		Verbose:    true,
		Short:      true,
		Parallel:   true,
		Timeout:    5 * time.Minute,
		Coverage:   false,
		Race:       false,
		Benchmarks: false,
	}
	
	runner.SetConfig(config)
	
	// Add quick test suites
	quickSuites := []TestSuite{
		{
			Name:        "Unit Tests (Quick)",
			TestFiles:   []string{"./..."},
			Tags:        []string{"unit"},
			Timeout:     5 * time.Minute,
			Parallel:    true,
			Verbose:     false,
			Short:       true,
			Description: "Quick unit tests for development",
		},
		{
			Name:        "Integration Tests (Quick)",
			TestFiles:   []string{"./comprehensive_integration_test.go"},
			Tags:        []string{"integration"},
			Timeout:     3 * time.Minute,
			Parallel:    true,
			Verbose:     true,
			Short:       true,
			Description: "Quick integration tests",
		},
	}
	
	for _, suite := range quickSuites {
		runner.AddSuite(suite)
	}
	
	return runner.RunAll()
}

// Example usage function
func ExampleUsage() {
	fmt.Println(`
Test Runner Usage Examples:

1. Run all tests:
   go run test_runner.go

2. Run with environment variables:
   SHORT=true go run test_runner.go
   VERBOSE=false go run test_runner.go
   NO_RACE=true go run test_runner.go

3. Run specific test suites:
   go test -v ./comprehensive_integration_test.go
   go test -v ./comprehensive_benchmark_test.go -bench=.

4. Run tests with coverage:
   go test -v -cover ./...

5. Run tests with race detection:
   go test -v -race ./...

Available Test Suites:
- Unit Tests: Core functionality tests
- Integration Tests: End-to-end pipeline tests
- Concurrency Tests: Thread safety and race condition tests
- Performance Benchmarks: Throughput and efficiency tests
- Regression Tests: Content negotiation and format handling
- End-to-End Tests: Real-world usage scenarios
- Error Handling Tests: Edge cases and error conditions
- Pool Tests: Updated pool implementation tests

Environment Variables:
- SHORT: Run tests in short mode (skip long-running tests)
- VERBOSE: Enable/disable verbose output
- NO_RACE: Disable race detection
- TIMEOUT: Set custom timeout (e.g., TIMEOUT=60m)`)
}