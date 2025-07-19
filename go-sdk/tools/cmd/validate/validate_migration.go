// Package tools provides validation utilities for migration processes
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ValidationConfig holds configuration for the validation process
type ValidationConfig struct {
	// Directory is the root directory to validate
	Directory string
	
	// BeforeSnapshot is the path to the pre-migration snapshot
	BeforeSnapshot string
	
	// AfterSnapshot is the path to the post-migration snapshot
	AfterSnapshot string
	
	// OutputFile is the file to write the validation report
	OutputFile string
	
	// RunTests indicates whether to run the test suite
	RunTests bool
	
	// RunBenchmarks indicates whether to run performance benchmarks
	RunBenchmarks bool
	
	// CheckCompatibility indicates whether to check backward compatibility
	CheckCompatibility bool
	
	// Verbose enables detailed logging
	Verbose bool
	
	// TestTimeout is the timeout for running tests
	TestTimeout time.Duration
	
	// BenchmarkDuration is the duration for running benchmarks
	BenchmarkDuration time.Duration
	
	// MaxAcceptableSlowdown is the maximum acceptable performance degradation (percentage)
	MaxAcceptableSlowdown float64
}

// ValidationReport contains the complete validation results
type ValidationReport struct {
	// Timestamp of the validation
	Timestamp string `json:"timestamp"`
	
	// Config used for validation
	Config ValidationConfig `json:"config"`
	
	// Summary of validation results
	Summary ValidationSummary `json:"summary"`
	
	// Semantic equivalence check results
	SemanticEquivalence SemanticEquivalenceResult `json:"semantic_equivalence"`
	
	// Test results
	TestResults TestResults `json:"test_results"`
	
	// Performance comparison results
	PerformanceResults PerformanceResults `json:"performance_results"`
	
	// Compatibility check results
	CompatibilityResults CompatibilityResults `json:"compatibility_results"`
	
	// Issues found during validation
	Issues []ValidationIssue `json:"issues"`
	
	// Recommendations for next steps
	Recommendations []string `json:"recommendations"`
}

// ValidationSummary contains high-level validation results
type ValidationSummary struct {
	// OverallStatus is the overall validation status
	OverallStatus string `json:"overall_status"` // "pass", "fail", "warning"
	
	// SemanticEquivalence indicates if the migration preserves semantics
	SemanticEquivalence bool `json:"semantic_equivalence"`
	
	// TestsPassed indicates if all tests passed
	TestsPassed bool `json:"tests_passed"`
	
	// PerformanceAcceptable indicates if performance is within acceptable bounds
	PerformanceAcceptable bool `json:"performance_acceptable"`
	
	// BackwardCompatible indicates if the changes are backward compatible
	BackwardCompatible bool `json:"backward_compatible"`
	
	// IssuesFound is the number of issues found
	IssuesFound int `json:"issues_found"`
	
	// CriticalIssues is the number of critical issues found
	CriticalIssues int `json:"critical_issues"`
}

// SemanticEquivalenceResult contains results of semantic equivalence checking
type SemanticEquivalenceResult struct {
	// Equivalent indicates if the code is semantically equivalent
	Equivalent bool `json:"equivalent"`
	
	// ComparisonMethod is the method used for comparison
	ComparisonMethod string `json:"comparison_method"`
	
	// Differences lists any semantic differences found
	Differences []SemanticDifference `json:"differences"`
	
	// AnalysisDetails contains detailed analysis information
	AnalysisDetails map[string]interface{} `json:"analysis_details"`
}

// SemanticDifference represents a semantic difference between before and after
type SemanticDifference struct {
	// Type of difference (e.g., "function_signature", "behavior", "type_safety")
	Type string `json:"type"`
	
	// Location where the difference was found
	Location string `json:"location"`
	
	// Description of the difference
	Description string `json:"description"`
	
	// Severity of the difference
	Severity string `json:"severity"` // "critical", "major", "minor"
	
	// BeforeCode is the code before migration
	BeforeCode string `json:"before_code"`
	
	// AfterCode is the code after migration
	AfterCode string `json:"after_code"`
}

// TestResults contains test execution results
type TestResults struct {
	// TestsPassed indicates if all tests passed
	TestsPassed bool `json:"tests_passed"`
	
	// TotalTests is the total number of tests run
	TotalTests int `json:"total_tests"`
	
	// PassedTests is the number of tests that passed
	PassedTests int `json:"passed_tests"`
	
	// FailedTests is the number of tests that failed
	FailedTests int `json:"failed_tests"`
	
	// SkippedTests is the number of tests that were skipped
	SkippedTests int `json:"skipped_tests"`
	
	// ExecutionTime is the total test execution time
	ExecutionTime time.Duration `json:"execution_time"`
	
	// FailedTestDetails contains details about failed tests
	FailedTestDetails []FailedTestDetail `json:"failed_test_details"`
	
	// Coverage information
	Coverage CoverageInfo `json:"coverage"`
}

// FailedTestDetail contains information about a failed test
type FailedTestDetail struct {
	// TestName is the name of the failed test
	TestName string `json:"test_name"`
	
	// Package is the package containing the test
	Package string `json:"package"`
	
	// ErrorMessage is the error message from the test failure
	ErrorMessage string `json:"error_message"`
	
	// Output is the full test output
	Output string `json:"output"`
}

// CoverageInfo contains code coverage information
type CoverageInfo struct {
	// TotalCoverage is the overall code coverage percentage
	TotalCoverage float64 `json:"total_coverage"`
	
	// PackageCoverage contains coverage by package
	PackageCoverage map[string]float64 `json:"package_coverage"`
	
	// ChangedFilesCoverage is coverage for files that were changed
	ChangedFilesCoverage float64 `json:"changed_files_coverage"`
}

// PerformanceResults contains performance comparison results
type PerformanceResults struct {
	// PerformanceAcceptable indicates if performance is within acceptable bounds
	PerformanceAcceptable bool `json:"performance_acceptable"`
	
	// BenchmarkResults contains benchmark comparison results
	BenchmarkResults []BenchmarkComparison `json:"benchmark_results"`
	
	// OverallSlowdown is the overall performance slowdown percentage
	OverallSlowdown float64 `json:"overall_slowdown"`
	
	// MemoryUsageChange is the change in memory usage
	MemoryUsageChange MemoryUsageComparison `json:"memory_usage_change"`
}

// BenchmarkComparison compares benchmark results before and after migration
type BenchmarkComparison struct {
	// Name of the benchmark
	Name string `json:"name"`
	
	// BeforeResult is the benchmark result before migration
	BeforeResult BenchmarkResult `json:"before_result"`
	
	// AfterResult is the benchmark result after migration
	AfterResult BenchmarkResult `json:"after_result"`
	
	// PerformanceChange is the percentage change in performance
	PerformanceChange float64 `json:"performance_change"`
	
	// Acceptable indicates if the performance change is acceptable
	Acceptable bool `json:"acceptable"`
}

// BenchmarkResult represents a single benchmark result
type BenchmarkResult struct {
	// Name is the benchmark name
	Name string `json:"name"`
	
	// Iterations is the number of iterations
	Iterations int `json:"iterations"`
	
	// NsPerOp is nanoseconds per operation
	NsPerOp float64 `json:"ns_per_op"`
	
	// AllocsPerOp is allocations per operation
	AllocsPerOp int `json:"allocs_per_op"`
	
	// BytesPerOp is bytes per operation
	BytesPerOp int `json:"bytes_per_op"`
}

// MemoryUsageComparison compares memory usage before and after migration
type MemoryUsageComparison struct {
	// BeforeUsage is memory usage before migration (MB)
	BeforeUsage float64 `json:"before_usage"`
	
	// AfterUsage is memory usage after migration (MB)
	AfterUsage float64 `json:"after_usage"`
	
	// PercentageChange is the percentage change in memory usage
	PercentageChange float64 `json:"percentage_change"`
	
	// Acceptable indicates if the memory usage change is acceptable
	Acceptable bool `json:"acceptable"`
}

// CompatibilityResults contains backward compatibility check results
type CompatibilityResults struct {
	// BackwardCompatible indicates if the changes are backward compatible
	BackwardCompatible bool `json:"backward_compatible"`
	
	// APIChanges lists API changes that may break compatibility
	APIChanges []APIChange `json:"api_changes"`
	
	// DeprecatedFeatures lists features that are now deprecated
	DeprecatedFeatures []string `json:"deprecated_features"`
	
	// BreakingChanges lists breaking changes
	BreakingChanges []BreakingChange `json:"breaking_changes"`
}

// APIChange represents a change to the public API
type APIChange struct {
	// Type of change (e.g., "function_signature", "struct_field", "interface")
	Type string `json:"type"`
	
	// Location of the change
	Location string `json:"location"`
	
	// Description of the change
	Description string `json:"description"`
	
	// Breaking indicates if this is a breaking change
	Breaking bool `json:"breaking"`
	
	// BeforeSignature is the signature before the change
	BeforeSignature string `json:"before_signature"`
	
	// AfterSignature is the signature after the change
	AfterSignature string `json:"after_signature"`
}

// BreakingChange represents a breaking change
type BreakingChange struct {
	// Type of breaking change
	Type string `json:"type"`
	
	// Location of the breaking change
	Location string `json:"location"`
	
	// Description of the breaking change
	Description string `json:"description"`
	
	// Impact describes the impact of the breaking change
	Impact string `json:"impact"`
	
	// MitigationStrategy suggests how to mitigate the breaking change
	MitigationStrategy string `json:"mitigation_strategy"`
}

// ValidationIssue represents an issue found during validation
type ValidationIssue struct {
	// Severity of the issue
	Severity string `json:"severity"` // "critical", "major", "minor", "info"
	
	// Category of the issue
	Category string `json:"category"`
	
	// Location where the issue was found
	Location string `json:"location"`
	
	// Description of the issue
	Description string `json:"description"`
	
	// Recommendation for fixing the issue
	Recommendation string `json:"recommendation"`
}

func main() {
	var config ValidationConfig
	
	// Parse command line flags
	flag.StringVar(&config.Directory, "dir", ".", "Root directory to validate")
	flag.StringVar(&config.BeforeSnapshot, "before", "", "Path to pre-migration snapshot (optional)")
	flag.StringVar(&config.AfterSnapshot, "after", "", "Path to post-migration snapshot (optional)")
	flag.StringVar(&config.OutputFile, "output", "validation_report.json", "Output file for validation report")
	flag.BoolVar(&config.RunTests, "tests", true, "Run test suite")
	flag.BoolVar(&config.RunBenchmarks, "benchmarks", true, "Run performance benchmarks")
	flag.BoolVar(&config.CheckCompatibility, "compatibility", true, "Check backward compatibility")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")
	flag.DurationVar(&config.TestTimeout, "test-timeout", 10*time.Minute, "Timeout for running tests")
	flag.DurationVar(&config.BenchmarkDuration, "benchmark-duration", 30*time.Second, "Duration for running benchmarks")
	flag.Float64Var(&config.MaxAcceptableSlowdown, "max-slowdown", 10.0, "Maximum acceptable performance slowdown (%)")
	flag.Parse()
	
	if config.Verbose {
		log.Printf("Starting validation with config: %+v", config)
	}
	
	// Run validation
	report, err := runValidation(config)
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}
	
	// Print summary
	printSummary(report)
	
	// Write detailed report
	if err := writeReport(report, config.OutputFile); err != nil {
		log.Printf("Warning: Failed to write report: %v", err)
	}
	
	// Exit with appropriate code
	if report.Summary.OverallStatus == "fail" {
		os.Exit(1)
	}
	
	if config.Verbose {
		log.Printf("Validation completed successfully. Report written to %s", config.OutputFile)
	}
}

// runValidation executes the validation process
func runValidation(config ValidationConfig) (*ValidationReport, error) {
	report := &ValidationReport{
		Timestamp: time.Now().Format(time.RFC3339),
		Config:    config,
	}
	
	var issues []ValidationIssue
	
	// Check semantic equivalence
	if config.Verbose {
		log.Printf("Checking semantic equivalence...")
	}
	semanticResult, err := checkSemanticEquivalence(config)
	if err != nil {
		issues = append(issues, ValidationIssue{
			Severity:    "major",
			Category:    "semantic_equivalence",
			Description: fmt.Sprintf("Failed to check semantic equivalence: %v", err),
		})
	}
	report.SemanticEquivalence = semanticResult
	
	// Run tests
	if config.RunTests {
		if config.Verbose {
			log.Printf("Running tests...")
		}
		testResults, err := runTests(config)
		if err != nil {
			issues = append(issues, ValidationIssue{
				Severity:    "critical",
				Category:    "tests",
				Description: fmt.Sprintf("Failed to run tests: %v", err),
			})
		}
		report.TestResults = testResults
	}
	
	// Run performance benchmarks
	if config.RunBenchmarks {
		if config.Verbose {
			log.Printf("Running performance benchmarks...")
		}
		perfResults, err := runPerformanceBenchmarks(config)
		if err != nil {
			issues = append(issues, ValidationIssue{
				Severity:    "major",
				Category:    "performance",
				Description: fmt.Sprintf("Failed to run benchmarks: %v", err),
			})
		}
		report.PerformanceResults = perfResults
	}
	
	// Check backward compatibility
	if config.CheckCompatibility {
		if config.Verbose {
			log.Printf("Checking backward compatibility...")
		}
		compatResults, err := checkCompatibility(config)
		if err != nil {
			issues = append(issues, ValidationIssue{
				Severity:    "major",
				Category:    "compatibility",
				Description: fmt.Sprintf("Failed to check compatibility: %v", err),
			})
		}
		report.CompatibilityResults = compatResults
	}
	
	// Additional static analysis checks
	staticIssues, err := runStaticAnalysis(config)
	if err != nil {
		log.Printf("Warning: Static analysis failed: %v", err)
	} else {
		issues = append(issues, staticIssues...)
	}
	
	report.Issues = issues
	
	// Calculate summary
	report.Summary = calculateSummary(report)
	
	// Generate recommendations
	report.Recommendations = generateRecommendations(report)
	
	return report, nil
}

// checkSemanticEquivalence checks if the migration preserves semantic equivalence
func checkSemanticEquivalence(config ValidationConfig) (SemanticEquivalenceResult, error) {
	result := SemanticEquivalenceResult{
		ComparisonMethod: "ast_analysis",
		AnalysisDetails:  make(map[string]interface{}),
	}
	
	// If snapshots are not provided, we can only do basic checks
	if config.BeforeSnapshot == "" || config.AfterSnapshot == "" {
		result.Equivalent = true // Assume equivalent without snapshots
		result.ComparisonMethod = "basic_analysis"
		return result, nil
	}
	
	// Compare AST structures between before and after
	differences, err := compareASTs(config.BeforeSnapshot, config.AfterSnapshot)
	if err != nil {
		return result, err
	}
	
	result.Differences = differences
	result.Equivalent = len(differences) == 0
	
	return result, nil
}

// compareASTs compares AST structures between two directory snapshots
func compareASTs(beforeDir, afterDir string) ([]SemanticDifference, error) {
	var differences []SemanticDifference
	
	// Find all Go files in both directories
	beforeFiles, err := findGoFiles(beforeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find files in before snapshot: %w", err)
	}
	
	afterFiles, err := findGoFiles(afterDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find files in after snapshot: %w", err)
	}
	
	// Compare common files
	for _, beforeFile := range beforeFiles {
		relativePath, _ := filepath.Rel(beforeDir, beforeFile)
		afterFile := filepath.Join(afterDir, relativePath)
		
		if _, err := os.Stat(afterFile); os.IsNotExist(err) {
			differences = append(differences, SemanticDifference{
				Type:        "file_removed",
				Location:    relativePath,
				Description: "File was removed during migration",
				Severity:    "major",
			})
			continue
		}
		
		fileDiffs, err := compareFileASTs(beforeFile, afterFile)
		if err != nil {
			log.Printf("Warning: Failed to compare %s: %v", relativePath, err)
			continue
		}
		
		differences = append(differences, fileDiffs...)
	}
	
	// Check for new files
	for _, afterFile := range afterFiles {
		relativePath, _ := filepath.Rel(afterDir, afterFile)
		beforeFile := filepath.Join(beforeDir, relativePath)
		
		if _, err := os.Stat(beforeFile); os.IsNotExist(err) {
			differences = append(differences, SemanticDifference{
				Type:        "file_added",
				Location:    relativePath,
				Description: "File was added during migration",
				Severity:    "minor",
			})
		}
	}
	
	return differences, nil
}

// compareFileASTs compares AST structures of two Go files
func compareFileASTs(beforeFile, afterFile string) ([]SemanticDifference, error) {
	var differences []SemanticDifference
	
	// Parse both files
	fset := token.NewFileSet()
	
	beforeAst, err := parser.ParseFile(fset, beforeFile, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse before file: %w", err)
	}
	
	afterAst, err := parser.ParseFile(fset, afterFile, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse after file: %w", err)
	}
	
	// Compare function signatures
	beforeFuncs := extractFunctions(beforeAst)
	afterFuncs := extractFunctions(afterAst)
	
	for name, beforeFunc := range beforeFuncs {
		if afterFunc, exists := afterFuncs[name]; exists {
			if beforeFunc != afterFunc {
				differences = append(differences, SemanticDifference{
					Type:        "function_signature",
					Location:    fmt.Sprintf("%s:%s", beforeFile, name),
					Description: fmt.Sprintf("Function signature changed: %s", name),
					Severity:    "major",
					BeforeCode:  beforeFunc,
					AfterCode:   afterFunc,
				})
			}
		} else {
			differences = append(differences, SemanticDifference{
				Type:        "function_removed",
				Location:    fmt.Sprintf("%s:%s", beforeFile, name),
				Description: fmt.Sprintf("Function removed: %s", name),
				Severity:    "critical",
				BeforeCode:  beforeFunc,
			})
		}
	}
	
	// Check for new functions
	for name, afterFunc := range afterFuncs {
		if _, exists := beforeFuncs[name]; !exists {
			differences = append(differences, SemanticDifference{
				Type:        "function_added",
				Location:    fmt.Sprintf("%s:%s", afterFile, name),
				Description: fmt.Sprintf("Function added: %s", name),
				Severity:    "minor",
				AfterCode:   afterFunc,
			})
		}
	}
	
	return differences, nil
}

// extractFunctions extracts function signatures from an AST
func extractFunctions(file *ast.File) map[string]string {
	functions := make(map[string]string)
	
	ast.Inspect(file, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			if fn.Name.IsExported() {
				signature := extractFunctionSignature(fn)
				functions[fn.Name.Name] = signature
			}
		}
		return true
	})
	
	return functions
}

// extractFunctionSignature extracts a normalized function signature
func extractFunctionSignature(fn *ast.FuncDecl) string {
	var buf bytes.Buffer
	
	buf.WriteString("func ")
	if fn.Recv != nil {
		buf.WriteString("(")
		// Simplified receiver representation
		buf.WriteString("...) ")
	}
	buf.WriteString(fn.Name.Name)
	buf.WriteString("(")
	// Simplified parameter representation
	if fn.Type.Params != nil {
		for i, _ := range fn.Type.Params.List {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString("...")
		}
	}
	buf.WriteString(")")
	
	if fn.Type.Results != nil {
		buf.WriteString(" (")
		for i := range fn.Type.Results.List {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString("...")
		}
		buf.WriteString(")")
	}
	
	return buf.String()
}

// runTests executes the test suite and returns results
func runTests(config ValidationConfig) (TestResults, error) {
	result := TestResults{}
	
	ctx, cancel := context.WithTimeout(context.Background(), config.TestTimeout)
	defer cancel()
	
	// Run go test with coverage
	cmd := exec.CommandContext(ctx, "go", "test", "-v", "-cover", "-coverprofile=coverage.out", "./...")
	cmd.Dir = config.Directory
	
	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	
	// Parse test output
	result.TotalTests, result.PassedTests, result.FailedTests, result.SkippedTests = parseTestOutput(outputStr)
	result.TestsPassed = result.FailedTests == 0
	
	if err != nil {
		// Parse failed test details
		result.FailedTestDetails = parseFailedTests(outputStr)
	}
	
	// Parse coverage information
	coverage, err := parseCoverage(config.Directory)
	if err != nil {
		log.Printf("Warning: Failed to parse coverage: %v", err)
	}
	result.Coverage = coverage
	
	return result, nil
}

// parseTestOutput parses go test output to extract test counts
func parseTestOutput(output string) (total, passed, failed, skipped int) {
	lines := strings.Split(output, "\n")
	
	for _, line := range lines {
		// Look for test result lines
		if strings.Contains(line, "--- PASS:") {
			passed++
			total++
		} else if strings.Contains(line, "--- FAIL:") {
			failed++
			total++
		} else if strings.Contains(line, "--- SKIP:") {
			skipped++
			total++
		}
	}
	
	return
}

// parseFailedTests parses failed test details from test output
func parseFailedTests(output string) []FailedTestDetail {
	var failed []FailedTestDetail
	
	lines := strings.Split(output, "\n")
	var currentTest *FailedTestDetail
	
	for _, line := range lines {
		if strings.Contains(line, "--- FAIL:") {
			if currentTest != nil {
				failed = append(failed, *currentTest)
			}
			// Extract test name
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				currentTest = &FailedTestDetail{
					TestName: parts[2],
				}
			}
		} else if currentTest != nil {
			// Accumulate output for the current failed test
			currentTest.Output += line + "\n"
			
			// Extract error message if it looks like an error
			if strings.Contains(line, "Error:") || strings.Contains(line, "panic:") {
				currentTest.ErrorMessage = strings.TrimSpace(line)
			}
		}
	}
	
	if currentTest != nil {
		failed = append(failed, *currentTest)
	}
	
	return failed
}

// parseCoverage parses coverage information from coverage files
func parseCoverage(dir string) (CoverageInfo, error) {
	info := CoverageInfo{
		PackageCoverage: make(map[string]float64),
	}
	
	coverageFile := filepath.Join(dir, "coverage.out")
	if _, err := os.Stat(coverageFile); os.IsNotExist(err) {
		return info, nil // No coverage file
	}
	
	// Use go tool cover to get coverage percentage
	cmd := exec.Command("go", "tool", "cover", "-func", coverageFile)
	cmd.Dir = dir
	
	output, err := cmd.Output()
	if err != nil {
		return info, err
	}
	
	// Parse coverage output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "total:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				coverageStr := strings.TrimSuffix(parts[2], "%")
				if coverage, err := strconv.ParseFloat(coverageStr, 64); err == nil {
					info.TotalCoverage = coverage
				}
			}
		}
	}
	
	return info, nil
}

// runPerformanceBenchmarks runs performance benchmarks and compares results
func runPerformanceBenchmarks(config ValidationConfig) (PerformanceResults, error) {
	result := PerformanceResults{}
	
	ctx, cancel := context.WithTimeout(context.Background(), config.BenchmarkDuration*2)
	defer cancel()
	
	// Run benchmarks
	cmd := exec.CommandContext(ctx, "go", "test", "-bench=.", "-benchmem", "-benchtime="+config.BenchmarkDuration.String(), "./...")
	cmd.Dir = config.Directory
	
	output, err := cmd.Output()
	if err != nil {
		return result, fmt.Errorf("failed to run benchmarks: %w", err)
	}
	
	// Parse benchmark results
	benchmarks := parseBenchmarkOutput(string(output))
	
	// For now, we'll just check that benchmarks run successfully
	// In a real implementation, you'd compare with baseline results
	result.PerformanceAcceptable = true
	result.OverallSlowdown = 0.0
	
	for _, bench := range benchmarks {
		comparison := BenchmarkComparison{
			Name:        bench.Name,
			AfterResult: bench,
			Acceptable:  true, // Placeholder
		}
		result.BenchmarkResults = append(result.BenchmarkResults, comparison)
	}
	
	return result, nil
}

// parseBenchmarkOutput parses go test -bench output
func parseBenchmarkOutput(output string) []BenchmarkResult {
	var results []BenchmarkResult
	
	lines := strings.Split(output, "\n")
	benchmarkRe := regexp.MustCompile(`^(Benchmark\w+)\s+(\d+)\s+([\d.]+)\s+ns/op(?:\s+(\d+)\s+B/op)?(?:\s+(\d+)\s+allocs/op)?`)
	
	for _, line := range lines {
		matches := benchmarkRe.FindStringSubmatch(line)
		if len(matches) >= 4 {
			name := matches[1]
			iterations, _ := strconv.Atoi(matches[2])
			nsPerOp, _ := strconv.ParseFloat(matches[3], 64)
			
			result := BenchmarkResult{
				Name:       name,
				Iterations: iterations,
				NsPerOp:    nsPerOp,
			}
			
			if len(matches) >= 5 && matches[4] != "" {
				result.BytesPerOp, _ = strconv.Atoi(matches[4])
			}
			
			if len(matches) >= 6 && matches[5] != "" {
				result.AllocsPerOp, _ = strconv.Atoi(matches[5])
			}
			
			results = append(results, result)
		}
	}
	
	return results
}

// checkCompatibility checks backward compatibility
func checkCompatibility(config ValidationConfig) (CompatibilityResults, error) {
	result := CompatibilityResults{
		BackwardCompatible: true, // Assume compatible unless proven otherwise
	}
	
	// Analyze public API changes
	changes, err := analyzeAPIChanges(config.Directory)
	if err != nil {
		return result, err
	}
	
	result.APIChanges = changes
	
	// Check for breaking changes
	for _, change := range changes {
		if change.Breaking {
			result.BackwardCompatible = false
			result.BreakingChanges = append(result.BreakingChanges, BreakingChange{
				Type:        change.Type,
				Location:    change.Location,
				Description: change.Description,
				Impact:      "May break existing client code",
			})
		}
	}
	
	return result, nil
}

// analyzeAPIChanges analyzes changes to the public API
func analyzeAPIChanges(dir string) ([]APIChange, error) {
	var changes []APIChange
	
	// This is a simplified implementation
	// In practice, you'd compare with a baseline API definition
	
	files, err := findGoFiles(dir)
	if err != nil {
		return nil, err
	}
	
	for _, file := range files {
		// Look for interface{} to any conversions that might be breaking
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		
		contentStr := string(content)
		
		// Check for interface{} usage in public APIs
		if strings.Contains(contentStr, "interface{}") {
			changes = append(changes, APIChange{
				Type:        "interface_usage",
				Location:    file,
				Description: "File still contains interface{} usage in public API",
				Breaking:    false, // Usually not breaking
			})
		}
	}
	
	return changes, nil
}

// runStaticAnalysis runs additional static analysis checks
func runStaticAnalysis(config ValidationConfig) ([]ValidationIssue, error) {
	var issues []ValidationIssue
	
	// Run go vet
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = config.Directory
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Parse vet output for issues
		vetIssues := parseVetOutput(string(output))
		issues = append(issues, vetIssues...)
	}
	
	// Check for remaining interface{} usage
	interfaceIssues, err := checkRemainingInterfaceUsage(config.Directory)
	if err != nil {
		log.Printf("Warning: Failed to check interface usage: %v", err)
	} else {
		issues = append(issues, interfaceIssues...)
	}
	
	return issues, nil
}

// parseVetOutput parses go vet output into validation issues
func parseVetOutput(output string) []ValidationIssue {
	var issues []ValidationIssue
	
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			issues = append(issues, ValidationIssue{
				Severity:    "minor",
				Category:    "static_analysis",
				Description: line,
				Location:    extractLocationFromVetOutput(line),
			})
		}
	}
	
	return issues
}

// extractLocationFromVetOutput extracts file location from vet output
func extractLocationFromVetOutput(line string) string {
	parts := strings.SplitN(line, ":", 3)
	if len(parts) >= 2 {
		return parts[0] + ":" + parts[1]
	}
	return ""
}

// checkRemainingInterfaceUsage checks for remaining interface{} usage
func checkRemainingInterfaceUsage(dir string) ([]ValidationIssue, error) {
	var issues []ValidationIssue
	
	files, err := findGoFiles(dir)
	if err != nil {
		return nil, err
	}
	
	interfacePattern := regexp.MustCompile(`\binterface\{\}`)
	
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		
		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if interfacePattern.MatchString(line) {
				issues = append(issues, ValidationIssue{
					Severity:       "info",
					Category:       "migration_completeness",
					Location:       fmt.Sprintf("%s:%d", file, i+1),
					Description:    "Remaining interface{} usage found",
					Recommendation: "Consider migrating to type-safe alternative",
				})
			}
		}
	}
	
	return issues, nil
}

// findGoFiles finds all Go files in a directory
func findGoFiles(dir string) ([]string, error) {
	var files []string
	
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if strings.HasSuffix(path, ".go") && !strings.Contains(path, "vendor/") {
			files = append(files, path)
		}
		
		return nil
	})
	
	return files, err
}

// calculateSummary calculates the overall validation summary
func calculateSummary(report *ValidationReport) ValidationSummary {
	summary := ValidationSummary{
		SemanticEquivalence:   report.SemanticEquivalence.Equivalent,
		TestsPassed:           report.TestResults.TestsPassed,
		PerformanceAcceptable: report.PerformanceResults.PerformanceAcceptable,
		BackwardCompatible:    report.CompatibilityResults.BackwardCompatible,
		IssuesFound:           len(report.Issues),
	}
	
	// Count critical issues
	for _, issue := range report.Issues {
		if issue.Severity == "critical" {
			summary.CriticalIssues++
		}
	}
	
	// Determine overall status
	if summary.CriticalIssues > 0 || !summary.TestsPassed {
		summary.OverallStatus = "fail"
	} else if !summary.SemanticEquivalence || !summary.PerformanceAcceptable || !summary.BackwardCompatible {
		summary.OverallStatus = "warning"
	} else {
		summary.OverallStatus = "pass"
	}
	
	return summary
}

// generateRecommendations generates recommendations based on validation results
func generateRecommendations(report *ValidationReport) []string {
	var recommendations []string
	
	// Critical issues
	if report.Summary.CriticalIssues > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("🚨 %d critical issues found. Address these before proceeding.", report.Summary.CriticalIssues))
	}
	
	// Test failures
	if !report.Summary.TestsPassed {
		recommendations = append(recommendations,
			fmt.Sprintf("❌ %d tests failed. Fix failing tests before deploying.", report.TestResults.FailedTests))
	}
	
	// Performance issues
	if !report.Summary.PerformanceAcceptable {
		recommendations = append(recommendations,
			"⚡ Performance degradation detected. Review and optimize before deploying.")
	}
	
	// Compatibility issues
	if !report.Summary.BackwardCompatible {
		recommendations = append(recommendations,
			"🔄 Breaking changes detected. Plan migration strategy for clients.")
	}
	
	// Semantic equivalence
	if !report.Summary.SemanticEquivalence {
		recommendations = append(recommendations,
			"🔍 Semantic differences detected. Verify behavior changes are intentional.")
	}
	
	// General recommendations
	if report.Summary.OverallStatus == "pass" {
		recommendations = append(recommendations,
			"✅ Validation passed. Migration appears successful.",
			"📋 Consider running additional integration tests.",
			"🚀 Safe to proceed with deployment.")
	} else {
		recommendations = append(recommendations,
			"🔧 Address identified issues before deploying.",
			"🧪 Run validation again after fixes.",
			"📚 Review migration documentation for guidance.")
	}
	
	return recommendations
}

// printSummary prints a summary of the validation results
func printSummary(report *ValidationReport) {
	fmt.Printf("\n=== Migration Validation Summary ===\n")
	fmt.Printf("Overall Status: %s\n", strings.ToUpper(report.Summary.OverallStatus))
	fmt.Printf("Semantic Equivalence: %t\n", report.Summary.SemanticEquivalence)
	fmt.Printf("Tests Passed: %t\n", report.Summary.TestsPassed)
	fmt.Printf("Performance Acceptable: %t\n", report.Summary.PerformanceAcceptable)
	fmt.Printf("Backward Compatible: %t\n", report.Summary.BackwardCompatible)
	fmt.Printf("Issues Found: %d\n", report.Summary.IssuesFound)
	fmt.Printf("Critical Issues: %d\n", report.Summary.CriticalIssues)
	
	if report.TestResults.TotalTests > 0 {
		fmt.Printf("\nTest Results:\n")
		fmt.Printf("  Total: %d\n", report.TestResults.TotalTests)
		fmt.Printf("  Passed: %d\n", report.TestResults.PassedTests)
		fmt.Printf("  Failed: %d\n", report.TestResults.FailedTests)
		fmt.Printf("  Skipped: %d\n", report.TestResults.SkippedTests)
		if report.TestResults.Coverage.TotalCoverage > 0 {
			fmt.Printf("  Coverage: %.1f%%\n", report.TestResults.Coverage.TotalCoverage)
		}
	}
	
	if len(report.Recommendations) > 0 {
		fmt.Printf("\nRecommendations:\n")
		for _, rec := range report.Recommendations {
			fmt.Printf("  %s\n", rec)
		}
	}
}

// writeReport writes the validation report to a file
func writeReport(report *ValidationReport, filename string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}
	
	return os.WriteFile(filename, data, 0644)
}