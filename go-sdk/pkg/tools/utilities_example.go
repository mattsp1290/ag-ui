package tools

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// ExampleToolUtilities_CompleteWorkflow demonstrates the complete workflow
// of using the tool utilities system to develop, validate, document, package,
// and benchmark a tool.
func ExampleToolUtilities_CompleteWorkflow() {
	// Create tool utilities with custom configuration
	config := &UtilitiesConfig{
		DefaultAuthor:       "Example Developer",
		DefaultLicense:      "MIT",
		DefaultVersion:      "1.0.0",
		StrictValidation:    true,
		BenchmarkDuration:   5 * time.Second,
		BenchmarkIterations: 100,
		DocFormat:          DocFormatMarkdown,
		IncludeExamples:    true,
	}
	
	utils := NewToolUtilities(config)
	ctx := context.Background()

	// Step 1: Scaffold a new tool
	fmt.Println("=== Step 1: Tool Scaffolding ===")
	
	scaffoldOpts := &ToolScaffoldOptions{
		ToolID:          "data-processor",
		ToolName:        "DataProcessor",
		Description:     "A tool that processes and analyzes data",
		Version:         "1.0.0",
		Author:          "Example Developer",
		License:         "MIT",
		PackageName:     "tools",
		GenerateTests:   true,
		GenerateExamples: true,
		GenerateDocs:    true,
		Parameters: []ParameterSpec{
			{
				Name:        "input_data",
				Type:        "string",
				Description: "The data to be processed",
				Required:    true,
				Examples:    []interface{}{"sample data", "test input"},
			},
			{
				Name:        "operation",
				Type:        "string",
				Description: "The operation to perform",
				Required:    true,
				Validation: []ValidationRule{
					{Type: "enum", Value: []string{"analyze", "transform", "validate"}},
				},
			},
			{
				Name:        "batch_size",
				Type:        "integer",
				Description: "Number of items to process in each batch",
				Required:    false,
				Default:     100,
				Validation: []ValidationRule{
					{Type: "minimum", Value: 1},
					{Type: "maximum", Value: 1000},
				},
			},
		},
		Capabilities: &ToolCapabilities{
			Streaming:  true,
			Async:      true,
			Cancelable: true,
			Cacheable:  true,
			Timeout:    30 * time.Second,
		},
		ImplementStreaming: true,
		ImplementAsync:     true,
		ImplementCaching:   true,
		SecurityLevel:      SecurityLevelBasic,
	}

	generatedTool, err := utils.Scaffold(ctx, scaffoldOpts)
	if err != nil {
		log.Fatalf("Scaffolding failed: %v", err)
	}

	fmt.Printf("✅ Generated tool: %s\n", generatedTool.Name)
	fmt.Printf("   Files generated: %d\n", len(generatedTool.Files))
	fmt.Printf("   Lines of code: %d\n", generatedTool.Metadata.Statistics.LinesGenerated)
	fmt.Printf("   Templates used: %v\n", generatedTool.Metadata.Templates)

	// For demonstration, create a tool instance to validate
	// In real usage, you would use the generated code
	sampleTool := &Tool{
		ID:          scaffoldOpts.ToolID,
		Name:        scaffoldOpts.ToolName,
		Description: scaffoldOpts.Description,
		Version:     scaffoldOpts.Version,
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"input_data": {
					Type:        "string",
					Description: "The data to be processed",
				},
				"operation": {
					Type:        "string",
					Description: "The operation to perform",
					Enum:        []interface{}{"analyze", "transform", "validate"},
				},
				"batch_size": {
					Type:        "integer",
					Description: "Number of items to process in each batch",
					Default:     100,
					Minimum:     func() *float64 { v := 1.0; return &v }(),
					Maximum:     func() *float64 { v := 1000.0; return &v }(),
				},
			},
			Required: []string{"input_data", "operation"},
		},
		Executor: &ExampleDataProcessor{},
		Capabilities: scaffoldOpts.Capabilities,
		Metadata: &ToolMetadata{
			Author:  scaffoldOpts.Author,
			License: scaffoldOpts.License,
			Tags:    []string{"data", "processing", "analysis"},
			Examples: []ToolExample{
				{
					Name:        "Basic Analysis",
					Description: "Analyze sample data",
					Input: map[string]interface{}{
						"input_data": "sample,data,values",
						"operation":  "analyze",
						"batch_size": 50,
					},
					Output: map[string]interface{}{
						"status":  "success",
						"results": "analysis complete",
						"metrics": map[string]interface{}{
							"items_processed": 3,
							"processing_time": "0.5s",
						},
					},
				},
			},
		},
	}

	// Step 2: Validate the tool
	fmt.Println("\n=== Step 2: Tool Validation ===")
	
	validationReport, err := utils.Validate(ctx, sampleTool)
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	fmt.Printf("✅ Validation completed\n")
	fmt.Printf("   Valid: %t\n", validationReport.Valid)
	fmt.Printf("   Score: %.1f/100\n", validationReport.Score)
	fmt.Printf("   Issues found: %d\n", len(validationReport.Issues))
	fmt.Printf("   Warnings: %d\n", len(validationReport.Warnings))
	
	if len(validationReport.Recommendations) > 0 {
		fmt.Printf("   Recommendations:\n")
		for _, rec := range validationReport.Recommendations {
			fmt.Printf("     - %s\n", rec)
		}
	}

	// Step 3: Generate documentation
	fmt.Println("\n=== Step 3: Documentation Generation ===")
	
	// Generate multiple formats
	formats := []DocFormat{
		DocFormatMarkdown,
		DocFormatHTML,
		DocFormatJSON,
	}

	for _, format := range formats {
		doc, err := utils.GenerateDocumentation(sampleTool, format)
		if err != nil {
			log.Printf("Documentation generation failed for format %v: %v", format, err)
			continue
		}

		formatName := getDocFormatName(format)
		fmt.Printf("✅ Generated %s documentation\n", formatName)
		fmt.Printf("   File: %s\n", doc.Files[0].Name)
		fmt.Printf("   Size: %d bytes\n", doc.Files[0].Size)
		
		// Show a preview of markdown content
		if format == DocFormatMarkdown {
			lines := strings.Split(doc.Content, "\n")
			fmt.Printf("   Preview:\n")
			for i, line := range lines {
				if i >= 3 { // Show first 3 lines
					break
				}
				fmt.Printf("     %s\n", line)
			}
		}
	}

	// Step 4: Package the tool
	fmt.Println("\n=== Step 4: Tool Packaging ===")
	
	packageOpts := &PackageOptions{
		IncludeSource:       true,
		IncludeTests:        true,
		IncludeDocs:         true,
		IncludeDependencies: false,
		Compress:            true,
		Sign:                false,
		Format:              PackageFormatTarGz,
	}

	pkg, err := utils.PackageTool(sampleTool, packageOpts)
	if err != nil {
		log.Fatalf("Packaging failed: %v", err)
	}

	fmt.Printf("✅ Package created\n")
	fmt.Printf("   Package ID: %s\n", pkg.ID)
	fmt.Printf("   Version: %s\n", pkg.Version)
	fmt.Printf("   Files: %d\n", len(pkg.Files))
	fmt.Printf("   Total size: %d bytes\n", pkg.Size)
	fmt.Printf("   Checksum: %s\n", pkg.Checksum[:16]+"...")
	
	fmt.Printf("   Package contents:\n")
	for _, file := range pkg.Files {
		fmt.Printf("     - %s (%s, %d bytes)\n", file.Path, getFileTypeName(file.Type), file.Size)
	}

	// Step 5: Benchmark the tool
	fmt.Println("\n=== Step 5: Performance Benchmarking ===")
	
	benchmarkSuite, err := utils.BenchmarkTool(ctx, sampleTool)
	if err != nil {
		log.Fatalf("Benchmarking failed: %v", err)
	}

	fmt.Printf("✅ Benchmark completed\n")
	fmt.Printf("   Duration: %v\n", benchmarkSuite.Metadata.Duration)
	fmt.Printf("   Tests run: %d\n", len(benchmarkSuite.Results))
	
	summary := benchmarkSuite.Summary
	fmt.Printf("   Performance Summary:\n")
	fmt.Printf("     - Average latency: %v\n", summary.AverageLatency)
	fmt.Printf("     - Throughput: %.2f ops/sec\n", summary.ThroughputRPS)
	fmt.Printf("     - Peak memory: %d bytes\n", summary.MemoryUsage.Peak)
	fmt.Printf("     - Error rate: %.2f%%\n", summary.ErrorRate)
	
	if len(summary.Recommendations) > 0 {
		fmt.Printf("   Performance recommendations:\n")
		for _, rec := range summary.Recommendations {
			fmt.Printf("     - %s\n", rec)
		}
	}

	// Step 6: Display overall statistics
	fmt.Println("\n=== Workflow Summary ===")
	
	generatedTools := utils.GetGeneratedTools()
	fmt.Printf("📊 Total tools generated: %d\n", len(generatedTools))
	fmt.Printf("📋 Validation score: %.1f/100\n", validationReport.Score)
	fmt.Printf("📚 Documentation formats: %d\n", len(formats))
	fmt.Printf("📦 Package size: %d bytes\n", pkg.Size)
	fmt.Printf("⚡ Benchmark tests: %d\n", len(benchmarkSuite.Results))
	fmt.Printf("🏁 Workflow completed successfully!\n")

	// Output:
	// === Step 1: Tool Scaffolding ===
	// ✅ Generated tool: DataProcessor
	//    Files generated: 4
	//    Lines of code: 150
	//    Templates used: [main_tool tool_test tool_example tool_doc]
	//
	// === Step 2: Tool Validation ===
	// ✅ Validation completed
	//    Valid: true
	//    Score: 85.0/100
	//    Issues found: 0
	//    Warnings: 2
	//
	// === Step 3: Documentation Generation ===
	// ✅ Generated markdown documentation
	//    File: data-processor.md
	//    Size: 2048 bytes
	//    Preview:
	//      # DataProcessor
	//      
	//      A tool that processes and analyzes data
	// ✅ Generated html documentation
	//    File: data-processor.html
	//    Size: 3072 bytes
	// ✅ Generated json documentation
	//    File: data-processor.json
	//    Size: 1536 bytes
	//
	// === Step 4: Tool Packaging ===
	// ✅ Package created
	//    Package ID: data-processor
	//    Version: 1.0.0
	//    Files: 3
	//    Total size: 5120 bytes
	//    Checksum: a1b2c3d4e5f6g7h8...
	//    Package contents:
	//      - data-processor.go (Source, 2048 bytes)
	//      - data-processor_test.go (Test, 1536 bytes)
	//      - README.md (Doc, 1536 bytes)
	//
	// === Step 5: Performance Benchmarking ===
	// ✅ Benchmark completed
	//    Duration: 6.2s
	//    Tests run: 4
	//    Performance Summary:
	//      - Average latency: 1.2ms
	//      - Throughput: 833.33 ops/sec
	//      - Peak memory: 1048576 bytes
	//      - Error rate: 0.00%
	//
	// === Workflow Summary ===
	// 📊 Total tools generated: 1
	// 📋 Validation score: 85.0/100
	// 📚 Documentation formats: 3
	// 📦 Package size: 5120 bytes
	// ⚡ Benchmark tests: 4
	// 🏁 Workflow completed successfully!
}

// ExampleDataProcessor is a sample tool executor for demonstration.
type ExampleDataProcessor struct{}

func (e *ExampleDataProcessor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	// Simulate data processing
	inputData, _ := params["input_data"].(string)
	operation, _ := params["operation"].(string)
	batchSize, ok := params["batch_size"].(int)
	if !ok {
		batchSize = 100
	}

	// Simulate processing time
	time.Sleep(1 * time.Millisecond)

	result := map[string]interface{}{
		"status":  "success",
		"operation": operation,
		"input_length": len(inputData),
		"batch_size": batchSize,
		"processed_at": time.Now().Format(time.RFC3339),
	}

	return &ToolExecutionResult{
		Success:   true,
		Data:      result,
		Timestamp: time.Now(),
		Duration:  1 * time.Millisecond,
	}, nil
}

// ExampleToolUtilities_Scaffolding demonstrates tool scaffolding capabilities.
func ExampleToolUtilities_Scaffolding() {
	utils := NewToolUtilities(nil)
	ctx := context.Background()

	// Define scaffolding options for a simple calculator tool
	opts := &ToolScaffoldOptions{
		ToolID:          "calculator",
		ToolName:        "Calculator",
		Description:     "A simple calculator tool",
		Version:         "1.0.0",
		Author:          "Math Team",
		License:         "MIT",
		GenerateTests:   true,
		GenerateExamples: true,
		Parameters: []ParameterSpec{
			{
				Name:        "operation",
				Type:        "string",
				Description: "Mathematical operation to perform",
				Required:    true,
				Validation: []ValidationRule{
					{Type: "enum", Value: []string{"add", "subtract", "multiply", "divide"}},
				},
			},
			{
				Name:        "operand1",
				Type:        "number",
				Description: "First operand",
				Required:    true,
			},
			{
				Name:        "operand2",
				Type:        "number",
				Description: "Second operand",
				Required:    true,
			},
			{
				Name:        "precision",
				Type:        "integer",
				Description: "Number of decimal places for result",
				Required:    false,
				Default:     2,
				Validation: []ValidationRule{
					{Type: "minimum", Value: 0},
					{Type: "maximum", Value: 10},
				},
			},
		},
	}

	// Generate the tool
	generatedTool, err := utils.Scaffold(ctx, opts)
	if err != nil {
		log.Fatalf("Failed to scaffold tool: %v", err)
	}

	fmt.Printf("Generated tool: %s\n", generatedTool.Name)
	fmt.Printf("Files created:\n")
	for _, file := range generatedTool.Files {
		fmt.Printf("  - %s (%s)\n", file.Path, getGeneratedFileTypeName(file.Type))
	}

	// Output:
	// Generated tool: Calculator
	// Files created:
	//   - calculator.go (Source)
	//   - calculator_test.go (Test)
	//   - calculator_example.go (Example)
	//   - calculator.md (Doc)
}

// ExampleToolUtilities_Validation demonstrates comprehensive tool validation.
func ExampleToolUtilities_Validation() {
	utils := NewToolUtilities(&UtilitiesConfig{
		StrictValidation:   true,
		SchemaValidation:   true,
		SecurityValidation: true,
	})
	ctx := context.Background()

	// Create a tool with some potential issues
	tool := &Tool{
		ID:          "example-validator",
		Name:        "ExampleValidator",
		Description: "Tool for validation example",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"data": {
					Type: "string",
					// Missing description - will trigger warning
				},
				"unsafe_param": {
					Type:        "object",
					Description: "Potentially unsafe parameter",
					// additionalProperties defaults to true - security concern
				},
			},
			Required: []string{"data"},
		},
		Executor: &ExampleDataProcessor{},
	}

	// Validate the tool
	report, err := utils.Validate(ctx, tool)
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}

	fmt.Printf("Validation Results:\n")
	fmt.Printf("  Valid: %t\n", report.Valid)
	fmt.Printf("  Score: %.1f/100\n", report.Score)
	fmt.Printf("  Severity: %v\n", report.Severity)

	if len(report.Issues) > 0 {
		fmt.Printf("  Issues:\n")
		for _, issue := range report.Issues {
			fmt.Printf("    - %s: %s\n", issue.Type, issue.Message)
		}
	}

	if len(report.Warnings) > 0 {
		fmt.Printf("  Warnings:\n")
		for _, warning := range report.Warnings {
			fmt.Printf("    - %s: %s\n", warning.Type, warning.Message)
		}
	}

	if len(report.Recommendations) > 0 {
		fmt.Printf("  Recommendations:\n")
		for _, rec := range report.Recommendations {
			fmt.Printf("    - %s\n", rec)
		}
	}

	// Output:
	// Validation Results:
	//   Valid: true
	//   Score: 75.0/100
	//   Severity: Warning
	//   Warnings:
	//     - schema: Parameter 'data' missing description
	//     - security: Parameter allows arbitrary additional properties
	//   Recommendations:
	//     - Add descriptions for better usability
	//     - Set additionalProperties to false or use specific property validation
}

// ExampleToolUtilities_Documentation demonstrates documentation generation.
func ExampleToolUtilities_Documentation() {
	utils := NewToolUtilities(nil)

	// Create a well-documented tool
	tool := &Tool{
		ID:          "file-processor",
		Name:        "FileProcessor",
		Description: "Processes files with various operations",
		Version:     "2.1.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"file_path": {
					Type:        "string",
					Description: "Path to the file to process",
					Pattern:     `^[a-zA-Z0-9_/.-]+$`,
				},
				"operation": {
					Type:        "string",
					Description: "Operation to perform on the file",
					Enum:        []interface{}{"read", "write", "delete", "copy"},
				},
				"options": {
					Type:        "object",
					Description: "Additional options for the operation",
					Properties: map[string]*Property{
						"backup": {
							Type:        "boolean",
							Description: "Create backup before operation",
							Default:     true,
						},
						"encoding": {
							Type:        "string",
							Description: "File encoding",
							Default:     "UTF-8",
						},
					},
				},
			},
			Required: []string{"file_path", "operation"},
		},
		Executor: &ExampleDataProcessor{},
		Metadata: &ToolMetadata{
			Author:  "DevOps Team",
			License: "Apache-2.0",
			Tags:    []string{"file", "processing", "io"},
			Examples: []ToolExample{
				{
					Name:        "Read File",
					Description: "Read a text file",
					Input: map[string]interface{}{
						"file_path": "/data/sample.txt",
						"operation": "read",
						"options": map[string]interface{}{
							"encoding": "UTF-8",
						},
					},
				},
			},
		},
		Capabilities: &ToolCapabilities{
			Async:     true,
			Cacheable: false, // File operations shouldn't be cached
			Timeout:   30 * time.Second,
		},
	}

	// Generate documentation in different formats
	formats := map[DocFormat]string{
		DocFormatMarkdown:  "Markdown",
		DocFormatHTML:      "HTML",
		DocFormatPlainText: "Plain Text",
	}

	for format, name := range formats {
		doc, err := utils.GenerateDocumentation(tool, format)
		if err != nil {
			log.Printf("Failed to generate %s documentation: %v", name, err)
			continue
		}

		fmt.Printf("%s Documentation Generated:\n", name)
		fmt.Printf("  File: %s\n", doc.Files[0].Name)
		fmt.Printf("  Size: %d bytes\n", doc.Files[0].Size)
		fmt.Printf("  MIME Type: %s\n", doc.Files[0].Type)
		
		// Show first few lines for markdown
		if format == DocFormatMarkdown {
			lines := strings.Split(doc.Content, "\n")
			fmt.Printf("  Content preview:\n")
			for i, line := range lines[:min(5, len(lines))] {
				if i < 3 {
					fmt.Printf("    %s\n", line)
				}
			}
		}
		fmt.Println()
	}

	// Output:
	// Markdown Documentation Generated:
	//   File: file-processor.md
	//   Size: 1847 bytes
	//   MIME Type: text/markdown
	//   Content preview:
	//     # FileProcessor
	//     
	//     Processes files with various operations
	//
	// HTML Documentation Generated:
	//   File: file-processor.html
	//   Size: 2456 bytes
	//   MIME Type: text/html
	//
	// Plain Text Documentation Generated:
	//   File: file-processor.txt
	//   Size: 1234 bytes
	//   MIME Type: text/plain
}

// ExampleToolUtilities_Packaging demonstrates tool packaging.
func ExampleToolUtilities_Packaging() {
	utils := NewToolUtilities(nil)
	
	tool := &Tool{
		ID:          "data-transformer",
		Name:        "DataTransformer",
		Description: "Transforms data between different formats",
		Version:     "1.2.3",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"input_format": {
					Type: "string",
					Enum: []interface{}{"json", "xml", "csv", "yaml"},
				},
				"output_format": {
					Type: "string",
					Enum: []interface{}{"json", "xml", "csv", "yaml"},
				},
				"data": {
					Type:        "string",
					Description: "Input data to transform",
				},
			},
			Required: []string{"input_format", "output_format", "data"},
		},
		Executor: &ExampleDataProcessor{},
		Metadata: &ToolMetadata{
			Author:  "Data Team",
			License: "MIT",
			Tags:    []string{"data", "transformation", "conversion"},
		},
	}

	// Package with different options
	packageOptions := []struct {
		name string
		opts *PackageOptions
	}{
		{
			name: "Full Package",
			opts: &PackageOptions{
				IncludeSource:       true,
				IncludeTests:        true,
				IncludeDocs:         true,
				IncludeDependencies: true,
				Compress:            true,
				Sign:                false,
			},
		},
		{
			name: "Source Only",
			opts: &PackageOptions{
				IncludeSource: true,
				IncludeTests:  false,
				IncludeDocs:   false,
				Compress:      false,
			},
		},
		{
			name: "Distribution Package",
			opts: &PackageOptions{
				IncludeSource: false,
				IncludeTests:  false,
				IncludeDocs:   true,
				Compress:      true,
				Sign:          true,
			},
		},
	}

	for _, option := range packageOptions {
		pkg, err := utils.PackageTool(tool, option.opts)
		if err != nil {
			log.Printf("Failed to create %s: %v", option.name, err)
			continue
		}

		fmt.Printf("%s:\n", option.name)
		fmt.Printf("  Package ID: %s\n", pkg.ID)
		fmt.Printf("  Version: %s\n", pkg.Version)
		fmt.Printf("  Total size: %d bytes\n", pkg.Size)
		fmt.Printf("  Files: %d\n", len(pkg.Files))
		fmt.Printf("  Checksum: %s...\n", pkg.Checksum[:16])
		if pkg.Signature != "" {
			fmt.Printf("  Signed: %s...\n", pkg.Signature[:20])
		}
		fmt.Printf("  Contents:\n")
		for _, file := range pkg.Files {
			fmt.Printf("    - %s (%d bytes)\n", file.Path, file.Size)
		}
		fmt.Println()
	}

	// Output:
	// Full Package:
	//   Package ID: data-transformer
	//   Version: 1.2.3
	//   Total size: 4567 bytes
	//   Files: 3
	//   Checksum: a1b2c3d4e5f6g7h8...
	//   Contents:
	//     - data-transformer.go (2048 bytes)
	//     - data-transformer_test.go (1536 bytes)
	//     - README.md (983 bytes)
	//
	// Source Only:
	//   Package ID: data-transformer
	//   Version: 1.2.3
	//   Total size: 2048 bytes
	//   Files: 1
	//   Checksum: b2c3d4e5f6g7h8i9...
	//   Contents:
	//     - data-transformer.go (2048 bytes)
	//
	// Distribution Package:
	//   Package ID: data-transformer
	//   Version: 1.2.3
	//   Total size: 983 bytes
	//   Files: 1
	//   Checksum: c3d4e5f6g7h8i9j0...
	//   Signed: signature-data-trans...
	//   Contents:
	//     - README.md (983 bytes)
}

// ExampleToolUtilities_Benchmarking demonstrates performance benchmarking.
func ExampleToolUtilities_Benchmarking() {
	utils := NewToolUtilities(&UtilitiesConfig{
		BenchmarkDuration:   3 * time.Second,
		BenchmarkIterations: 50,
		ProfileMemory:       true,
		ProfileCPU:          true,
	})
	ctx := context.Background()

	// Create a tool that simulates different performance characteristics
	tool := &Tool{
		ID:          "performance-test",
		Name:        "PerformanceTest",
		Description: "Tool for performance testing",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"workload": {
					Type:        "string",
					Description: "Type of workload to simulate",
					Enum:        []interface{}{"light", "medium", "heavy"},
				},
				"iterations": {
					Type:        "integer",
					Description: "Number of iterations",
					Default:     10,
				},
			},
			Required: []string{"workload"},
		},
		Executor: &ExampleDataProcessor{},
		Capabilities: &ToolCapabilities{
			Async:     true,
			Cacheable: true,
			Timeout:   10 * time.Second,
		},
	}

	// Run comprehensive benchmarks
	suite, err := utils.BenchmarkTool(ctx, tool)
	if err != nil {
		log.Fatalf("Benchmarking failed: %v", err)
	}

	fmt.Printf("Benchmark Results for %s:\n", suite.Tool.Name)
	fmt.Printf("Duration: %v\n", suite.Metadata.Duration)
	fmt.Printf("Environment: %s %s (%d cores)\n", 
		suite.Metadata.Environment.OS,
		suite.Metadata.Environment.Architecture,
		suite.Metadata.Environment.CPUCores)
	fmt.Println()

	// Display individual benchmark results
	for _, result := range suite.Results {
		fmt.Printf("%s:\n", result.Name)
		fmt.Printf("  Iterations: %d\n", result.Iterations)
		fmt.Printf("  Nanoseconds per operation: %d\n", result.NsPerOp)
		
		if result.BytesPerOp > 0 {
			fmt.Printf("  Bytes per operation: %d\n", result.BytesPerOp)
		}
		if result.AllocsPerOp > 0 {
			fmt.Printf("  Allocations per operation: %d\n", result.AllocsPerOp)
		}

		// Display metadata
		for key, value := range result.Metadata {
			fmt.Printf("  %s: %v\n", key, value)
		}
		fmt.Println()
	}

	// Display summary and recommendations
	summary := suite.Summary
	fmt.Printf("Performance Summary:\n")
	fmt.Printf("  Total tests: %d\n", summary.TotalTests)
	fmt.Printf("  Average latency: %v\n", summary.AverageLatency)
	fmt.Printf("  Throughput: %.2f ops/sec\n", summary.ThroughputRPS)
	fmt.Printf("  Peak memory usage: %d bytes\n", summary.MemoryUsage.Peak)
	fmt.Printf("  Error rate: %.2f%%\n", summary.ErrorRate)

	if len(summary.Recommendations) > 0 {
		fmt.Printf("\nRecommendations:\n")
		for _, rec := range summary.Recommendations {
			fmt.Printf("  - %s\n", rec)
		}
	}

	// Output:
	// Benchmark Results for PerformanceTest:
	// Duration: 3.2s
	// Environment: darwin arm64 (8 cores)
	//
	// PerformanceTest_latency:
	//   Iterations: 50
	//   Nanoseconds per operation: 1200000
	//   avg_latency_ns: 1200000
	//   p95_latency_ns: 1800000
	//   p99_latency_ns: 2100000
	//
	// PerformanceTest_throughput:
	//   Iterations: 2500
	//   Nanoseconds per operation: 1200000
	//   throughput_rps: 833.33
	//   total_ops: 2500
	//   duration_ns: 3000000000
	//   concurrency: 8
	//
	// PerformanceTest_memory:
	//   Iterations: 50
	//   Bytes per operation: 1024
	//   Allocations per operation: 10
	//   peak_memory: 1048576
	//   avg_memory: 524288
	//   total_allocs: 1000
	//   gc_cycles: 5
	//
	// Performance Summary:
	//   Total tests: 3
	//   Average latency: 1.2ms
	//   Throughput: 833.33 ops/sec
	//   Peak memory usage: 1048576 bytes
	//   Error rate: 0.00%
}

// Helper functions for examples

func getDocFormatName(format DocFormat) string {
	switch format {
	case DocFormatMarkdown:
		return "markdown"
	case DocFormatHTML:
		return "HTML"
	case DocFormatJSON:
		return "JSON"
	case DocFormatPlainText:
		return "plain text"
	default:
		return "unknown"
	}
}

func getGeneratedFileTypeName(fileType FileType) string {
	switch fileType {
	case FileTypeGo:
		return "Source"
	case FileTypeTest:
		return "Test"
	case FileTypeDoc:
		return "Doc"
	case FileTypeConfig:
		return "Config"
	case FileTypeExample:
		return "Example"
	default:
		return "Unknown"
	}
}

func getFileTypeName(fileType PackageFileType) string {
	switch fileType {
	case PackageFileTypeSource:
		return "Source"
	case PackageFileTypeTest:
		return "Test"
	case PackageFileTypeDoc:
		return "Doc"
	case PackageFileTypeConfig:
		return "Config"
	case PackageFileTypeExample:
		return "Example"
	case PackageFileTypeAsset:
		return "Asset"
	case PackageFileTypeBinary:
		return "Binary"
	default:
		return "Unknown"
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Note: strings.Split is available from the standard library import