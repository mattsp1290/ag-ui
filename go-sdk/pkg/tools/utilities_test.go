package tools

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper to create a sample tool for testing
func createSampleTool() *Tool {
	return &Tool{
		ID:          "test-tool",
		Name:        "TestTool",
		Description: "A test tool for utility testing",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"input": {
					Type:        "string",
					Description: "Input parameter",
				},
				"count": {
					Type:        "integer",
					Description: "Count parameter",
					Default:     10,
				},
			},
			Required: []string{"input"},
		},
		Executor: &mockUtilityExecutor{},
		Metadata: &ToolMetadata{
			Author:   "Test Author",
			License:  "MIT",
			Tags:     []string{"test", "utility"},
			Examples: []ToolExample{
				{
					Name:        "Basic Example",
					Description: "Basic usage example",
					Input:       map[string]interface{}{"input": "test"},
					Output:      "processed: test",
				},
			},
		},
		Capabilities: &ToolCapabilities{
			Streaming:  false,
			Async:      true,
			Cancelable: true,
			Cacheable:  true,
		},
	}
}

// Mock executor for utility testing
type mockUtilityExecutor struct{}

func (m *mockUtilityExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	return &ToolExecutionResult{
		Success:   true,
		Data:      "test result",
		Timestamp: time.Now(),
	}, nil
}

func TestToolScaffolder_GenerateTool(t *testing.T) {
	utils := NewToolUtilities(nil)
	ctx := context.Background()

	tests := []struct {
		name    string
		opts    *ToolScaffoldOptions
		wantErr bool
	}{
		{
			name: "valid scaffolding options",
			opts: &ToolScaffoldOptions{
				ToolID:      "example-tool",
				ToolName:    "ExampleTool",
				Description: "An example tool",
				Version:     "1.0.0",
				Author:      "Test Author",
				License:     "MIT",
				Parameters: []ParameterSpec{
					{
						Name:        "input",
						Type:        "string",
						Description: "Input parameter",
						Required:    true,
					},
				},
				GenerateTests:    true,
				GenerateExamples: true,
				GenerateDocs:     true,
				PackageName:      "tools",
			},
			wantErr: false,
		},
		{
			name: "missing tool ID",
			opts: &ToolScaffoldOptions{
				ToolName:    "ExampleTool",
				Description: "An example tool",
			},
			wantErr: true,
		},
		{
			name: "invalid tool ID format",
			opts: &ToolScaffoldOptions{
				ToolID:      "123-invalid",
				ToolName:    "ExampleTool",
				Description: "An example tool",
			},
			wantErr: true,
		},
		{
			name: "duplicate parameter names",
			opts: &ToolScaffoldOptions{
				ToolID:      "example-tool",
				ToolName:    "ExampleTool",
				Description: "An example tool",
				Parameters: []ParameterSpec{
					{Name: "param", Type: "string"},
					{Name: "param", Type: "integer"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := utils.Scaffold(ctx, tt.opts)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.opts.ToolID, result.Options.ToolID)
				assert.Equal(t, tt.opts.ToolName, result.Options.ToolName)
				assert.NotEmpty(t, result.SourceCode)
				assert.NotNil(t, result.Metadata)
			}
		})
	}
}

func TestParameterSpec_Validation(t *testing.T) {
	utils := NewToolUtilities(nil)
	ctx := context.Background()

	validTypes := []string{"string", "number", "integer", "boolean", "array", "object"}
	
	for _, validType := range validTypes {
		t.Run("valid type: "+validType, func(t *testing.T) {
			opts := &ToolScaffoldOptions{
				ToolID:      "test-tool",
				ToolName:    "TestTool",
				Description: "Test tool",
				Parameters: []ParameterSpec{
					{
						Name: "param",
						Type: validType,
					},
				},
			}
			
			result, err := utils.Scaffold(ctx, opts)
			assert.NoError(t, err)
			assert.NotNil(t, result)
		})
	}

	t.Run("invalid type", func(t *testing.T) {
		opts := &ToolScaffoldOptions{
			ToolID:      "test-tool",
			ToolName:    "TestTool",
			Description: "Test tool",
			Parameters: []ParameterSpec{
				{
					Name: "param",
					Type: "invalid-type",
				},
			},
		}
		
		result, err := utils.Scaffold(ctx, opts)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestToolValidator_ValidateTool(t *testing.T) {
	utils := NewToolUtilities(nil)
	ctx := context.Background()

	tests := []struct {
		name     string
		tool     *Tool
		wantErr  bool
		minScore float64
	}{
		{
			name:     "valid tool",
			tool:     createSampleTool(),
			wantErr:  false,
			minScore: 80.0,
		},
		{
			name: "tool with missing schema",
			tool: &Tool{
				ID:          "invalid-tool",
				Name:        "InvalidTool",
				Description: "An invalid tool",
				Version:     "1.0.0",
				Executor:    &mockUtilityExecutor{},
			},
			wantErr: true,
		},
		{
			name: "tool with invalid executor",
			tool: &Tool{
				ID:          "invalid-tool",
				Name:        "InvalidTool",
				Description: "An invalid tool",
				Version:     "1.0.0",
				Schema:      &ToolSchema{Type: "object"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := utils.Validate(ctx, tt.tool)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, report)
				assert.GreaterOrEqual(t, report.Score, tt.minScore)
				assert.NotNil(t, report.SchemaValidation)
				assert.NotNil(t, report.SecurityValidation)
				assert.NotNil(t, report.TestValidation)
			}
		})
	}
}

func TestDocumentationGenerator_GenerateDocumentation(t *testing.T) {
	utils := NewToolUtilities(nil)
	tool := createSampleTool()

	formats := []DocFormat{
		DocFormatMarkdown,
		DocFormatHTML,
		DocFormatJSON,
		DocFormatPlainText,
	}

	for _, format := range formats {
		t.Run("format: "+getFormatName(format), func(t *testing.T) {
			doc, err := utils.GenerateDocumentation(tool, format)
			assert.NoError(t, err)
			assert.NotNil(t, doc)
			assert.Equal(t, format, doc.Format)
			assert.NotEmpty(t, doc.Content)
			assert.Len(t, doc.Files, 1)
			assert.Contains(t, doc.Content, tool.Name)
			assert.Contains(t, doc.Content, tool.Description)
		})
	}
}

func getFormatName(format DocFormat) string {
	switch format {
	case DocFormatMarkdown:
		return "markdown"
	case DocFormatHTML:
		return "html"
	case DocFormatJSON:
		return "json"
	case DocFormatPlainText:
		return "plaintext"
	default:
		return "unknown"
	}
}

func TestToolPackager_CreatePackage(t *testing.T) {
	utils := NewToolUtilities(nil)
	tool := createSampleTool()

	tests := []struct {
		name    string
		opts    *PackageOptions
		wantErr bool
	}{
		{
			name: "default options",
			opts: nil,
			wantErr: false,
		},
		{
			name: "include all files",
			opts: &PackageOptions{
				IncludeSource: true,
				IncludeTests:  true,
				IncludeDocs:   true,
				Compress:      true,
			},
			wantErr: false,
		},
		{
			name: "source only",
			opts: &PackageOptions{
				IncludeSource: true,
				IncludeTests:  false,
				IncludeDocs:   false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, err := utils.PackageTool(tool, tt.opts)
			
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, pkg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pkg)
				assert.Equal(t, tool.ID, pkg.ID)
				assert.Equal(t, tool.Name, pkg.Name)
				assert.Equal(t, tool.Version, pkg.Version)
				assert.NotEmpty(t, pkg.Files)
				assert.Greater(t, pkg.Size, int64(0))
				assert.NotEmpty(t, pkg.Checksum)
				assert.NotNil(t, pkg.Metadata)
			}
		})
	}
}

func TestPackageFileTypes(t *testing.T) {
	utils := NewToolUtilities(nil)
	tool := createSampleTool()

	opts := &PackageOptions{
		IncludeSource: true,
		IncludeTests:  true,
		IncludeDocs:   true,
	}

	pkg, err := utils.PackageTool(tool, opts)
	require.NoError(t, err)
	require.NotNil(t, pkg)

	// Verify different file types are included
	var hasSource, hasDoc bool
	for _, file := range pkg.Files {
		switch file.Type {
		case PackageFileTypeSource:
			hasSource = true
		case PackageFileTypeDoc:
			hasDoc = true
		}
	}

	assert.True(t, hasSource, "Package should include source files")
	assert.True(t, hasDoc, "Package should include documentation files")
}

func TestPerformanceBenchmarker_RunBenchmark(t *testing.T) {
	utils := NewToolUtilities(nil)
	tool := createSampleTool()
	ctx := context.Background()

	suite, err := utils.BenchmarkTool(ctx, tool)
	assert.NoError(t, err)
	assert.NotNil(t, suite)
	assert.Equal(t, tool, suite.Tool)
	assert.NotEmpty(t, suite.Results)
	assert.NotNil(t, suite.Summary)
	assert.NotNil(t, suite.Metadata)
	assert.Greater(t, suite.Metadata.Duration, time.Duration(0))
}

func TestBenchmarkSuite_Results(t *testing.T) {
	utils := NewToolUtilities(nil)
	tool := createSampleTool()
	ctx := context.Background()

	suite, err := utils.BenchmarkTool(ctx, tool)
	require.NoError(t, err)
	require.NotNil(t, suite)

	// Should have at least latency and throughput benchmarks
	assert.GreaterOrEqual(t, len(suite.Results), 2)

	// Verify benchmark names
	var hasLatency, hasThroughput bool
	for _, result := range suite.Results {
		if result.Name == tool.Name+"_latency" {
			hasLatency = true
		}
		if result.Name == tool.Name+"_throughput" {
			hasThroughput = true
		}
	}

	assert.True(t, hasLatency, "Should have latency benchmark")
	assert.True(t, hasThroughput, "Should have throughput benchmark")
}

func TestToolUtilities_ErrorHandling(t *testing.T) {
	utils := NewToolUtilities(nil)
	ctx := context.Background()

	t.Run("scaffold with nil options", func(t *testing.T) {
		result, err := utils.Scaffold(ctx, nil)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("validate with nil tool", func(t *testing.T) {
		report, err := utils.Validate(ctx, nil)
		assert.Error(t, err)
		assert.Nil(t, report)
	})

	t.Run("generate documentation with nil tool", func(t *testing.T) {
		doc, err := utils.GenerateDocumentation(nil, DocFormatMarkdown)
		assert.Error(t, err)
		assert.Nil(t, doc)
	})

	t.Run("package with nil tool", func(t *testing.T) {
		pkg, err := utils.PackageTool(nil, nil)
		assert.Error(t, err)
		assert.Nil(t, pkg)
	})

	t.Run("benchmark with nil tool", func(t *testing.T) {
		suite, err := utils.BenchmarkTool(ctx, nil)
		assert.Error(t, err)
		assert.Nil(t, suite)
	})
}

// CreateTestContext creates a context with an appropriate timeout for the test environment
func CreateTestContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	
	timeout := OptimizedTestTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	
	// Set a deadline on the test itself
	t.Cleanup(func() {
		cancel()
	})
	
	return ctx, cancel
}

// PerformanceTestContext creates a context for performance tests with appropriate timeout
func PerformanceTestContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	
	var timeout time.Duration
	if isCI() {
		timeout = 10 * time.Second // Shorter for CI
	} else {
		timeout = 30 * time.Second // Longer for local testing
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	
	t.Cleanup(func() {
		cancel()
	})
	
	return ctx, cancel
}

// SkipIfCI skips a test if running in CI environment
func SkipIfCI(t *testing.T, reason string) {
	t.Helper()
	
	if isCI() {
		t.Skipf("Skipping in CI: %s", reason)
	}
}

// SkipSlowTestInCI skips slow tests in CI environment
func SkipSlowTestInCI(t *testing.T) {
	t.Helper()
	
	if isCI() {
		t.Skip("Skipping slow test in CI environment")
	}
}

// AdjustIterationsForEnvironment adjusts iteration count based on environment
func AdjustIterationsForEnvironment(baseIterations int) int {
	if isCI() {
		// Reduce iterations by 80% in CI
		return max(1, baseIterations/5)
	}
	return baseIterations
}

// AdjustDurationForEnvironment adjusts duration based on environment
func AdjustDurationForEnvironment(baseDuration time.Duration) time.Duration {
	if isCI() {
		// Reduce duration by 70% in CI
		return baseDuration * 3 / 10
	}
	return baseDuration
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// RunTestWithTimeout runs a test function with an appropriate timeout
func RunTestWithTimeout(t *testing.T, name string, timeout time.Duration, fn func(t *testing.T)) {
	t.Helper()
	
	adjustedTimeout := AdjustDurationForEnvironment(timeout)
	
	t.Run(name, func(t *testing.T) {
		done := make(chan struct{})
		
		go func() {
			defer close(done)
			fn(t)
		}()
		
		select {
		case <-done:
			// Test completed successfully
		case <-time.After(adjustedTimeout):
			t.Fatalf("Test timed out after %v", adjustedTimeout)
		}
	})
}

// RunBenchmarkWithOptimizedConfig runs a benchmark with optimized configuration
func RunBenchmarkWithOptimizedConfig(b *testing.B, fn func(b *testing.B, config *PerformanceConfig)) {
	b.Helper()
	
	config := OptimizedPerformanceConfig()
	
	// Further reduce for benchmarks
	if isCI() {
		config.BaselineIterations = 5
		config.LoadTestDuration = 2 * time.Second
	}
	
	fn(b, config)
}

// Helper functions for test configuration


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


