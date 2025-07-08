package tools

import (
	"context"
	"fmt"
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

// =============================================================================
// ToolUtilities Tests
// =============================================================================

func TestNewToolUtilities(t *testing.T) {
	tests := []struct {
		name   string
		config *UtilitiesConfig
	}{
		{
			name:   "with default config",
			config: nil,
		},
		{
			name:   "with custom config",
			config: DefaultUtilitiesConfig(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			utils := NewToolUtilities(tt.config)
			assert.NotNil(t, utils)
			assert.NotNil(t, utils.GetConfig())
		})
	}
}

func TestToolUtilities_SetConfig(t *testing.T) {
	utils := NewToolUtilities(nil)
	newConfig := &UtilitiesConfig{
		DefaultAuthor: "New Author",
		DefaultLicense: "Apache",
	}
	
	utils.SetConfig(newConfig)
	assert.Equal(t, newConfig, utils.GetConfig())
}

// =============================================================================
// Tool Scaffolding Tests
// =============================================================================

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
				assert.Equal(t, tt.opts.ToolID, result.ID)
				assert.Equal(t, tt.opts.ToolName, result.Name)
				assert.NotEmpty(t, result.Files)
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

// =============================================================================
// Tool Validation Tests
// =============================================================================

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

func TestValidationReport_ScoreCalculation(t *testing.T) {
	utils := NewToolUtilities(nil)
	ctx := context.Background()
	tool := createSampleTool()

	report, err := utils.Validate(ctx, tool)
	require.NoError(t, err)
	require.NotNil(t, report)

	// Score should be between 0 and 100
	assert.GreaterOrEqual(t, report.Score, 0.0)
	assert.LessOrEqual(t, report.Score, 100.0)

	// Valid tools should have high scores
	if report.Valid {
		assert.GreaterOrEqual(t, report.Score, 70.0)
	}
}

// =============================================================================
// Documentation Generation Tests
// =============================================================================

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

func TestDocumentationGenerator_InvalidTool(t *testing.T) {
	utils := NewToolUtilities(nil)
	
	invalidTool := &Tool{
		ID:   "invalid",
		Name: "Invalid",
		// Missing required fields
	}

	doc, err := utils.GenerateDocumentation(invalidTool, DocFormatMarkdown)
	assert.Error(t, err)
	assert.Nil(t, doc)
}

func TestDocumentationFormats(t *testing.T) {
	utils := NewToolUtilities(nil)
	tool := createSampleTool()

	t.Run("markdown format", func(t *testing.T) {
		doc, err := utils.GenerateDocumentation(tool, DocFormatMarkdown)
		require.NoError(t, err)
		assert.Contains(t, doc.Content, "# "+tool.Name)
		assert.Contains(t, doc.Content, "## Parameters")
		assert.Contains(t, doc.Content, "## Examples")
	})

	t.Run("html format", func(t *testing.T) {
		doc, err := utils.GenerateDocumentation(tool, DocFormatHTML)
		require.NoError(t, err)
		assert.Contains(t, doc.Content, "<html>")
		assert.Contains(t, doc.Content, "<title>")
		assert.Contains(t, doc.Content, tool.Name)
	})

	t.Run("json format", func(t *testing.T) {
		doc, err := utils.GenerateDocumentation(tool, DocFormatJSON)
		require.NoError(t, err)
		assert.Contains(t, doc.Content, `"Tool"`)
		assert.Contains(t, doc.Content, tool.ID)
	})

	t.Run("plain text format", func(t *testing.T) {
		doc, err := utils.GenerateDocumentation(tool, DocFormatPlainText)
		require.NoError(t, err)
		assert.Contains(t, doc.Content, tool.Name)
		assert.Contains(t, doc.Content, "OVERVIEW")
		assert.Contains(t, doc.Content, "PARAMETERS")
	})
}

// =============================================================================
// Tool Packaging Tests
// =============================================================================

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
	var hasSource, hasTest, hasDoc bool
	for _, file := range pkg.Files {
		switch file.Type {
		case PackageFileTypeSource:
			hasSource = true
		case PackageFileTypeTest:
			hasTest = true
		case PackageFileTypeDoc:
			hasDoc = true
		}
	}

	assert.True(t, hasSource, "Package should include source files")
	assert.True(t, hasTest, "Package should include test files")
	assert.True(t, hasDoc, "Package should include documentation files")
}

// =============================================================================
// Performance Benchmarking Tests
// =============================================================================

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

func TestBenchmarkSummary_Recommendations(t *testing.T) {
	utils := NewToolUtilities(nil)
	tool := createSampleTool()
	ctx := context.Background()

	suite, err := utils.BenchmarkTool(ctx, tool)
	require.NoError(t, err)
	require.NotNil(t, suite)
	require.NotNil(t, suite.Summary)

	// Summary should have meaningful data
	assert.GreaterOrEqual(t, suite.Summary.TotalTests, 1)
	assert.NotNil(t, suite.Summary.Recommendations)
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestToolUtilities_FullWorkflow(t *testing.T) {
	utils := NewToolUtilities(nil)
	ctx := context.Background()

	// Step 1: Scaffold a new tool
	scaffoldOpts := &ToolScaffoldOptions{
		ToolID:           "workflow-test-tool",
		ToolName:         "WorkflowTestTool",
		Description:      "A tool for testing the full workflow",
		Version:          "1.0.0",
		Author:           "Test Author",
		License:          "MIT",
		GenerateTests:    true,
		GenerateExamples: true,
		GenerateDocs:     true,
		Parameters: []ParameterSpec{
			{
				Name:        "input",
				Type:        "string",
				Description: "Input parameter",
				Required:    true,
			},
			{
				Name:        "count",
				Type:        "integer",
				Description: "Count parameter",
				Default:     5,
			},
		},
	}

	generatedTool, err := utils.Scaffold(ctx, scaffoldOpts)
	require.NoError(t, err)
	require.NotNil(t, generatedTool)

	// Create a tool object for further testing
	tool := createSampleTool()
	tool.ID = scaffoldOpts.ToolID
	tool.Name = scaffoldOpts.ToolName

	// Step 2: Validate the tool
	validationReport, err := utils.Validate(ctx, tool)
	require.NoError(t, err)
	require.NotNil(t, validationReport)
	assert.True(t, validationReport.Valid)

	// Step 3: Generate documentation
	doc, err := utils.GenerateDocumentation(tool, DocFormatMarkdown)
	require.NoError(t, err)
	require.NotNil(t, doc)

	// Step 4: Package the tool
	pkg, err := utils.PackageTool(tool, &PackageOptions{
		IncludeSource: true,
		IncludeTests:  true,
		IncludeDocs:   true,
	})
	require.NoError(t, err)
	require.NotNil(t, pkg)

	// Step 5: Benchmark the tool
	benchmarkSuite, err := utils.BenchmarkTool(ctx, tool)
	require.NoError(t, err)
	require.NotNil(t, benchmarkSuite)

	// Verify workflow completion
	assert.Equal(t, scaffoldOpts.ToolID, generatedTool.ID)
	assert.True(t, validationReport.Valid)
	assert.NotEmpty(t, doc.Content)
	assert.NotEmpty(t, pkg.Files)
	assert.NotEmpty(t, benchmarkSuite.Results)
}

func TestToolUtilities_ConfigurationPersistence(t *testing.T) {
	config := &UtilitiesConfig{
		DefaultAuthor:       "Custom Author",
		DefaultLicense:      "Apache-2.0",
		DefaultVersion:      "2.0.0",
		StrictValidation:    true,
		BenchmarkDuration:   10 * time.Second,
		BenchmarkIterations: 500,
	}

	utils := NewToolUtilities(config)
	
	// Verify configuration is applied
	assert.Equal(t, config, utils.GetConfig())

	// Test configuration update
	newConfig := &UtilitiesConfig{
		DefaultAuthor:  "Updated Author",
		DefaultLicense: "BSD-3-Clause",
	}

	utils.SetConfig(newConfig)
	assert.Equal(t, newConfig, utils.GetConfig())
}

// =============================================================================
// Error Handling Tests
// =============================================================================

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

// =============================================================================
// Performance Tests
// =============================================================================

func BenchmarkToolUtilities_Scaffold(b *testing.B) {
	utils := NewToolUtilities(nil)
	ctx := context.Background()

	opts := &ToolScaffoldOptions{
		ToolID:      "benchmark-tool",
		ToolName:    "BenchmarkTool",
		Description: "A tool for benchmarking",
		Parameters: []ParameterSpec{
			{Name: "input", Type: "string", Required: true},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		opts.ToolID = "benchmark-tool-" + string(rune(i))
		_, err := utils.Scaffold(ctx, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkToolUtilities_Validate(b *testing.B) {
	utils := NewToolUtilities(nil)
	ctx := context.Background()
	tool := createSampleTool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := utils.Validate(ctx, tool)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkToolUtilities_GenerateDocumentation(b *testing.B) {
	utils := NewToolUtilities(nil)
	tool := createSampleTool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := utils.GenerateDocumentation(tool, DocFormatMarkdown)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestToolUtilities_EdgeCases(t *testing.T) {
	utils := NewToolUtilities(nil)

	t.Run("empty tool ID", func(t *testing.T) {
		opts := &ToolScaffoldOptions{
			ToolID:      "",
			ToolName:    "TestTool",
			Description: "Test tool",
		}
		
		result, err := utils.Scaffold(context.Background(), opts)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("very long tool name", func(t *testing.T) {
		longName := string(make([]byte, 1000))
		for i := range longName {
			longName = longName[:i] + "a" + longName[i+1:]
		}

		opts := &ToolScaffoldOptions{
			ToolID:      "long-name-tool",
			ToolName:    longName,
			Description: "Tool with very long name",
		}
		
		result, err := utils.Scaffold(context.Background(), opts)
		assert.NoError(t, err) // Should handle long names gracefully
		assert.NotNil(t, result)
	})

	t.Run("tool with no parameters", func(t *testing.T) {
		tool := &Tool{
			ID:          "no-params-tool",
			Name:        "NoParamsTool",
			Description: "Tool with no parameters",
			Version:     "1.0.0",
			Schema: &ToolSchema{
				Type:       "object",
				Properties: map[string]*Property{},
				Required:   []string{},
			},
			Executor: &mockUtilityExecutor{},
		}

		// Should validate successfully
		report, err := utils.Validate(context.Background(), tool)
		assert.NoError(t, err)
		assert.NotNil(t, report)

		// Should generate documentation
		doc, err := utils.GenerateDocumentation(tool, DocFormatMarkdown)
		assert.NoError(t, err)
		assert.NotNil(t, doc)

		// Should package successfully
		pkg, err := utils.PackageTool(tool, nil)
		assert.NoError(t, err)
		assert.NotNil(t, pkg)
	})
}

// =============================================================================
// Concurrency Tests
// =============================================================================

func TestToolUtilities_Concurrency(t *testing.T) {
	utils := NewToolUtilities(nil)
	ctx := context.Background()

	const numGoroutines = 10
	const numOperationsPerGoroutine = 5

	t.Run("concurrent scaffolding", func(t *testing.T) {
		var results []*GeneratedTool
		var errors []error
		resultChan := make(chan *GeneratedTool, numGoroutines*numOperationsPerGoroutine)
		errorChan := make(chan error, numGoroutines*numOperationsPerGoroutine)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				for j := 0; j < numOperationsPerGoroutine; j++ {
					opts := &ToolScaffoldOptions{
						ToolID:      fmt.Sprintf("concurrent-tool-%d-%d", id, j),
						ToolName:    fmt.Sprintf("ConcurrentTool%d%d", id, j),
						Description: "Concurrent test tool",
						Parameters: []ParameterSpec{
							{Name: "input", Type: "string", Required: true},
						},
					}

					result, err := utils.Scaffold(ctx, opts)
					if err != nil {
						errorChan <- err
					} else {
						resultChan <- result
					}
				}
			}(i)
		}

		// Collect results
		for i := 0; i < numGoroutines*numOperationsPerGoroutine; i++ {
			select {
			case result := <-resultChan:
				results = append(results, result)
			case err := <-errorChan:
				errors = append(errors, err)
			case <-time.After(30 * time.Second):
				t.Fatal("Test timed out")
			}
		}

		// Verify results
		assert.Empty(t, errors, "Should not have errors in concurrent scaffolding")
		assert.Len(t, results, numGoroutines*numOperationsPerGoroutine)

		// Verify all results are unique
		ids := make(map[string]bool)
		for _, result := range results {
			assert.False(t, ids[result.ID], "Tool ID should be unique: %s", result.ID)
			ids[result.ID] = true
		}
	})

	t.Run("concurrent validation", func(t *testing.T) {
		tool := createSampleTool()
		var reports []*ValidationReport
		var errors []error
		reportChan := make(chan *ValidationReport, numGoroutines)
		errorChan := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				report, err := utils.Validate(ctx, tool)
				if err != nil {
					errorChan <- err
				} else {
					reportChan <- report
				}
			}()
		}

		// Collect results
		for i := 0; i < numGoroutines; i++ {
			select {
			case report := <-reportChan:
				reports = append(reports, report)
			case err := <-errorChan:
				errors = append(errors, err)
			case <-time.After(10 * time.Second):
				t.Fatal("Test timed out")
			}
		}

		assert.Empty(t, errors, "Should not have errors in concurrent validation")
		assert.Len(t, reports, numGoroutines)

		// All reports should be consistent
		for _, report := range reports {
			assert.True(t, report.Valid)
			assert.Greater(t, report.Score, 0.0)
		}
	})
}

// =============================================================================
// Memory Usage Tests
// =============================================================================

func TestToolUtilities_MemoryUsage(t *testing.T) {
	utils := NewToolUtilities(nil)
	ctx := context.Background()

	t.Run("scaffold many tools", func(t *testing.T) {
		const numTools = 100
		
		for i := 0; i < numTools; i++ {
			opts := &ToolScaffoldOptions{
				ToolID:      fmt.Sprintf("memory-test-tool-%d", i),
				ToolName:    fmt.Sprintf("MemoryTestTool%d", i),
				Description: "Memory test tool",
				Parameters: []ParameterSpec{
					{Name: "input", Type: "string", Required: true},
				},
			}

			result, err := utils.Scaffold(ctx, opts)
			assert.NoError(t, err)
			assert.NotNil(t, result)
		}

		// Verify generated tools are tracked
		generatedTools := utils.GetGeneratedTools()
		assert.Len(t, generatedTools, numTools)
	})
}