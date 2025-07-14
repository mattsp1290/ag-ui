# Tool Development Utilities

The Tool Development Utilities provide a comprehensive suite of tools for developing, testing, validating, documenting, packaging, and benchmarking tools in the AG UI Go SDK. This system makes tool development easier and more efficient by automating common tasks and ensuring high-quality, well-documented tools.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Quick Start](#quick-start)
- [Components](#components)
  - [Tool Scaffolding](#tool-scaffolding)
  - [Tool Validation](#tool-validation)
  - [Documentation Generation](#documentation-generation)
  - [Tool Packaging](#tool-packaging)
  - [Performance Benchmarking](#performance-benchmarking)
- [Configuration](#configuration)
- [Examples](#examples)
- [Best Practices](#best-practices)
- [API Reference](#api-reference)

## Overview

The Tool Development Utilities system consists of five main components:

1. **Tool Scaffolding**: Generate boilerplate code for new tools
2. **Tool Validation**: Comprehensive validation including schema, security, and testing
3. **Documentation Generation**: Generate documentation in multiple formats
4. **Tool Packaging**: Package tools for distribution
5. **Performance Benchmarking**: Benchmark tool performance and identify optimization opportunities

## Features

### 🏗️ Tool Scaffolding
- Generate complete tool implementations from specifications
- Support for multiple parameter types with validation
- Automatic test generation
- Example code generation
- Documentation generation
- Support for streaming, async, and caching capabilities

### ✅ Tool Validation
- Schema validation with JSON Schema support
- Security vulnerability scanning
- Automated testing with coverage analysis
- Performance validation
- Best practices compliance checking
- Detailed reporting with recommendations

### 📚 Documentation Generation
- Multiple output formats (Markdown, HTML, JSON, Plain Text)
- Automatic API documentation
- Performance metrics inclusion
- Security documentation
- Code examples with proper formatting
- Metadata and build information

### 📦 Tool Packaging
- Multiple package formats (tar.gz, zip, custom)
- Selective file inclusion (source, tests, docs, dependencies)
- Compression support
- Digital signing capabilities
- Checksum generation
- Metadata preservation

### ⚡ Performance Benchmarking
- Latency measurement
- Throughput testing
- Memory profiling
- CPU profiling
- Concurrent testing
- Performance recommendations
- Environment reporting

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
)

func main() {
    // Create utilities with default configuration
    utils := tools.NewToolUtilities(nil)
    ctx := context.Background()

    // 1. Scaffold a new tool
    opts := &tools.ToolScaffoldOptions{
        ToolID:          "my-awesome-tool",
        ToolName:        "MyAwesomeTool",
        Description:     "An awesome tool that does amazing things",
        Version:         "1.0.0",
        Author:          "Your Name",
        License:         "MIT",
        GenerateTests:   true,
        GenerateExamples: true,
        GenerateDocs:    true,
        Parameters: []tools.ParameterSpec{
            {
                Name:        "input",
                Type:        "string",
                Description: "Input data to process",
                Required:    true,
            },
        },
    }

    generatedTool, err := utils.Scaffold(ctx, opts)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Generated tool: %s with %d files\n", 
        generatedTool.Name, len(generatedTool.Files))

    // 2. Create a tool instance for further operations
    tool := createYourToolInstance() // Your implementation

    // 3. Validate the tool
    report, err := utils.Validate(ctx, tool)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Validation score: %.1f/100\n", report.Score)

    // 4. Generate documentation
    doc, err := utils.GenerateDocumentation(tool, tools.DocFormatMarkdown)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Documentation generated: %d bytes\n", len(doc.Content))

    // 5. Package the tool
    pkg, err := utils.PackageTool(tool, nil)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Package created: %d bytes\n", pkg.Size)

    // 6. Benchmark the tool
    benchmarks, err := utils.BenchmarkTool(ctx, tool)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Benchmark completed: %d tests\n", len(benchmarks.Results))
}
```

## Components

### Tool Scaffolding

The scaffolding system generates complete tool implementations from specifications.

#### Features
- **Code Generation**: Generates Go source code with proper structure
- **Test Generation**: Creates comprehensive unit tests
- **Example Generation**: Produces usage examples
- **Documentation**: Generates initial documentation
- **Template System**: Extensible template system for customization

#### Basic Usage

```go
scaffoldOpts := &tools.ToolScaffoldOptions{
    ToolID:          "data-processor",
    ToolName:        "DataProcessor", 
    Description:     "Processes various data formats",
    Version:         "1.0.0",
    Author:          "Data Team",
    License:         "MIT",
    PackageName:     "tools",
    GenerateTests:   true,
    GenerateExamples: true,
    GenerateDocs:    true,
    Parameters: []tools.ParameterSpec{
        {
            Name:        "input_data",
            Type:        "string",
            Description: "The data to process",
            Required:    true,
            Examples:    []interface{}{"sample data", "test input"},
        },
        {
            Name:        "format",
            Type:        "string", 
            Description: "Output format",
            Required:    false,
            Default:     "json",
            Validation: []tools.ValidationRule{
                {Type: "enum", Value: []string{"json", "xml", "csv"}},
            },
        },
    },
    Capabilities: &tools.ToolCapabilities{
        Streaming:  true,
        Async:      true,
        Cancelable: true,
        Cacheable:  true,
    },
    ImplementStreaming: true,
    ImplementAsync:     true,
    SecurityLevel:      tools.SecurityLevelBasic,
}

generatedTool, err := utils.Scaffold(ctx, scaffoldOpts)
```

#### Parameter Types

Supported parameter types:
- `string`: Text data with optional pattern validation
- `number`: Floating-point numbers with range validation
- `integer`: Whole numbers with range validation  
- `boolean`: True/false values
- `array`: Lists of items with item type validation
- `object`: Nested objects with property validation

#### Validation Rules

- `enum`: Restrict to specific values
- `minimum`/`maximum`: Numeric range validation
- `minLength`/`maxLength`: String/array length validation
- `pattern`: Regular expression validation

### Tool Validation

Comprehensive validation system that checks multiple aspects of tool quality.

#### Validation Components

1. **Schema Validation**: Validates JSON Schema compliance
2. **Security Validation**: Scans for security vulnerabilities
3. **Test Validation**: Runs automated tests and measures coverage
4. **Performance Validation**: Basic performance checks

#### Usage

```go
report, err := utils.Validate(ctx, tool)
if err != nil {
    return err
}

fmt.Printf("Valid: %t\n", report.Valid)
fmt.Printf("Score: %.1f/100\n", report.Score) 
fmt.Printf("Issues: %d\n", len(report.Issues))
fmt.Printf("Warnings: %d\n", len(report.Warnings))

// Handle issues
for _, issue := range report.Issues {
    fmt.Printf("Issue: %s - %s\n", issue.Type, issue.Message)
    if issue.Suggestion != "" {
        fmt.Printf("  Suggestion: %s\n", issue.Suggestion)
    }
}

// Apply recommendations
for _, rec := range report.Recommendations {
    fmt.Printf("Recommendation: %s\n", rec)
}
```

#### Validation Levels

- **Basic**: Schema and structure validation
- **Strict**: Includes security scanning and best practices
- **Comprehensive**: Full validation with performance testing

### Documentation Generation

Generate professional documentation in multiple formats.

#### Supported Formats

- **Markdown**: GitHub-compatible markdown with tables and code blocks
- **HTML**: Styled HTML with CSS for web viewing
- **JSON**: Structured JSON for programmatic access
- **Plain Text**: Simple text format for basic viewing

#### Usage

```go
// Generate markdown documentation
doc, err := utils.GenerateDocumentation(tool, tools.DocFormatMarkdown)
if err != nil {
    return err
}

fmt.Printf("Generated: %s\n", doc.Files[0].Name)
fmt.Printf("Size: %d bytes\n", doc.Files[0].Size)

// Save to file
err = ioutil.WriteFile(doc.Files[0].Name, []byte(doc.Content), 0644)
```

#### Documentation Sections

Generated documentation includes:
- Tool overview and metadata
- Parameter reference with types and validation
- Usage examples with code samples  
- API reference
- Performance characteristics
- Security information
- Build and generation metadata

### Tool Packaging

Package tools for distribution with various options.

#### Package Options

```go
packageOpts := &tools.PackageOptions{
    IncludeSource:       true,  // Include source code
    IncludeTests:        true,  // Include test files
    IncludeDocs:         true,  // Include documentation
    IncludeDependencies: false, // Include dependencies
    Compress:            true,  // Enable compression
    Sign:                false, // Digital signing
    Format:              tools.PackageFormatTarGz,
    OutputPath:          "./dist",
}

pkg, err := utils.PackageTool(tool, packageOpts)
```

#### Package Formats

- `PackageFormatTarGz`: Gzipped tar archive
- `PackageFormatZip`: ZIP archive
- `PackageFormatDocker`: Docker container (future)
- `PackageFormatCustom`: Custom format

#### Package Contents

Packages can include:
- Source code files
- Test files and test data
- Documentation files
- Configuration files
- Dependency information
- Build metadata
- Digital signatures

### Performance Benchmarking

Comprehensive performance testing and analysis.

#### Benchmark Types

1. **Latency Benchmarks**: Measure response time
2. **Throughput Benchmarks**: Measure requests per second
3. **Memory Benchmarks**: Measure memory usage and allocations
4. **CPU Benchmarks**: Measure CPU utilization
5. **Concurrency Benchmarks**: Test under concurrent load

#### Usage

```go
benchmarkSuite, err := utils.BenchmarkTool(ctx, tool)
if err != nil {
    return err
}

// Access results
for _, result := range benchmarkSuite.Results {
    fmt.Printf("Benchmark: %s\n", result.Name)
    fmt.Printf("  Iterations: %d\n", result.Iterations)
    fmt.Printf("  Nanoseconds per op: %d\n", result.NsPerOp)
    
    if result.BytesPerOp > 0 {
        fmt.Printf("  Bytes per op: %d\n", result.BytesPerOp)
    }
    if result.AllocsPerOp > 0 {
        fmt.Printf("  Allocs per op: %d\n", result.AllocsPerOp)
    }
}

// Summary and recommendations
summary := benchmarkSuite.Summary
fmt.Printf("Average latency: %v\n", summary.AverageLatency)
fmt.Printf("Throughput: %.2f ops/sec\n", summary.ThroughputRPS)
fmt.Printf("Peak memory: %d bytes\n", summary.MemoryUsage.Peak)

for _, rec := range summary.Recommendations {
    fmt.Printf("Recommendation: %s\n", rec)
}
```

## Configuration

### UtilitiesConfig

```go
config := &tools.UtilitiesConfig{
    // Scaffolding options
    DefaultAuthor:      "Your Name",
    DefaultLicense:     "MIT", 
    DefaultVersion:     "1.0.0",
    TemplateDirectory:  "./templates",
    OutputDirectory:    "./output",
    
    // Validation options
    StrictValidation:   true,
    SchemaValidation:   true,
    SecurityValidation: true,
    
    // Documentation options
    DocFormat:          tools.DocFormatMarkdown,
    IncludeExamples:    true,
    IncludeMetrics:     true,
    OutputFormat:       []string{"markdown", "html"},
    
    // Packaging options
    CompressionLevel:   6,
    IncludeSource:      true,
    IncludeDependencies: true,
    
    // Benchmarking options
    BenchmarkDuration:  30 * time.Second,
    BenchmarkIterations: 1000,
    ProfileMemory:      true,
    ProfileCPU:         true,
    
    // Custom settings
    CustomSettings:     map[string]interface{}{
        "custom_option": "value",
    },
}

utils := tools.NewToolUtilities(config)
```

## Examples

### Complete Workflow Example

See `utilities_example.go` for a complete workflow example that demonstrates:
- Tool scaffolding with parameters and validation
- Comprehensive tool validation
- Multi-format documentation generation
- Packaging with different options
- Performance benchmarking with detailed analysis

### Individual Component Examples

Each component has detailed examples in the example file:
- `ExampleToolUtilities_Scaffolding`: Tool scaffolding
- `ExampleToolUtilities_Validation`: Tool validation  
- `ExampleToolUtilities_Documentation`: Documentation generation
- `ExampleToolUtilities_Packaging`: Tool packaging
- `ExampleToolUtilities_Benchmarking`: Performance benchmarking

## Best Practices

### Tool Design

1. **Clear Naming**: Use descriptive, consistent naming for tools and parameters
2. **Comprehensive Documentation**: Provide detailed descriptions for all parameters
3. **Validation**: Use appropriate validation rules for all parameters
4. **Examples**: Include realistic usage examples
5. **Error Handling**: Implement proper error handling and reporting

### Scaffolding

1. **Parameter Design**: Design parameters with clear types and validation
2. **Required vs Optional**: Carefully consider which parameters are required
3. **Default Values**: Provide sensible defaults for optional parameters  
4. **Capabilities**: Only enable capabilities that the tool actually supports
5. **Security**: Consider security implications of parameter validation

### Validation

1. **Regular Validation**: Run validation regularly during development
2. **Address Issues**: Fix validation issues promptly
3. **Score Targets**: Aim for validation scores above 80
4. **Security Focus**: Pay special attention to security warnings
5. **Testing**: Ensure comprehensive test coverage

### Documentation

1. **Multiple Formats**: Generate documentation in formats appropriate for your audience
2. **Keep Updated**: Regenerate documentation when tools change
3. **Include Examples**: Provide realistic, tested examples
4. **API Documentation**: Ensure API documentation is comprehensive
5. **Performance Info**: Include performance characteristics

### Packaging

1. **Selective Inclusion**: Only include necessary files in packages
2. **Compression**: Use compression for distribution packages
3. **Versioning**: Use semantic versioning for packages
4. **Signing**: Sign packages for production distribution
5. **Metadata**: Include comprehensive metadata

### Benchmarking

1. **Regular Benchmarking**: Benchmark tools regularly to catch performance regressions
2. **Realistic Workloads**: Use realistic data and parameters for benchmarking
3. **Environment Consistency**: Run benchmarks in consistent environments
4. **Trend Analysis**: Track performance trends over time
5. **Optimization**: Address performance recommendations promptly

## API Reference

### Core Types

#### ToolUtilities
Main utilities interface providing access to all components.

```go
type ToolUtilities struct {
    // Configuration and components
}

// Create new utilities
func NewToolUtilities(config *UtilitiesConfig) *ToolUtilities

// Core operations
func (u *ToolUtilities) Scaffold(ctx context.Context, opts *ToolScaffoldOptions) (*GeneratedTool, error)
func (u *ToolUtilities) Validate(ctx context.Context, tool *Tool) (*ValidationReport, error)  
func (u *ToolUtilities) GenerateDocumentation(tool *Tool, format DocFormat) (*GeneratedDocumentation, error)
func (u *ToolUtilities) PackageTool(tool *Tool, opts *PackageOptions) (*ToolPackage, error)
func (u *ToolUtilities) BenchmarkTool(ctx context.Context, tool *Tool) (*BenchmarkSuite, error)
```

#### ToolScaffoldOptions
Configuration for tool scaffolding.

```go
type ToolScaffoldOptions struct {
    ToolID          string
    ToolName        string  
    Description     string
    Version         string
    Author          string
    License         string
    Parameters      []ParameterSpec
    Capabilities    *ToolCapabilities
    GenerateTests   bool
    GenerateExamples bool
    GenerateDocs    bool
    // ... additional fields
}
```

#### ValidationReport
Results of tool validation.

```go
type ValidationReport struct {
    Valid      bool
    Score      float64
    Severity   ValidationSeverity
    Issues     []ValidationIssue
    Warnings   []ValidationWarning
    // ... additional fields
}
```

#### GeneratedDocumentation
Generated documentation output.

```go
type GeneratedDocumentation struct {
    Tool       *Tool
    Format     DocFormat
    Content    string
    Files      []DocFile
    Metadata   *DocMetadata
    // ... additional fields  
}
```

#### ToolPackage
Packaged tool output.

```go
type ToolPackage struct {
    ID           string
    Name         string
    Version      string
    Files        []PackageFile
    Metadata     *PackageMetadata
    Size         int64
    Checksum     string
    // ... additional fields
}
```

#### BenchmarkSuite
Performance benchmark results.

```go
type BenchmarkSuite struct {
    Tool        *Tool
    Results     []BenchmarkResult
    Summary     *BenchmarkSummary
    Metadata    *BenchmarkMetadata
    // ... additional fields
}
```

### Constants

#### DocFormat
Documentation output formats.

```go
const (
    DocFormatMarkdown DocFormat = iota
    DocFormatHTML
    DocFormatJSON  
    DocFormatPlainText
)
```

#### ValidationSeverity
Validation issue severity levels.

```go
const (
    SeverityInfo ValidationSeverity = iota
    SeverityWarning
    SeverityError
    SeverityCritical
)
```

#### SecurityLevel
Security requirement levels.

```go
const (
    SecurityLevelNone SecurityLevel = iota
    SecurityLevelBasic
    SecurityLevelStrict
    SecurityLevelMaximum
)
```

For detailed API documentation, see the Go documentation generated from the source code.

## Contributing

When contributing to the utilities system:

1. Follow the existing code patterns and conventions
2. Add comprehensive tests for new functionality
3. Update documentation for any API changes
4. Run the full test suite before submitting changes
5. Consider backward compatibility for API changes

## License

This utilities system is part of the AG UI Go SDK and is subject to the same license terms.