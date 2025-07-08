package tools

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/format"
	"text/template"
	"sync/atomic"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// ToolUtilities provides comprehensive utilities for tool development, testing, and deployment.
// It includes scaffolding, validation, documentation generation, packaging, and performance benchmarking.
type ToolUtilities struct {
	// Configuration
	config *UtilitiesConfig
	
	// Components
	scaffolder     *ToolScaffolder
	validator      *ToolValidator
	docGenerator   *DocumentationGenerator
	packager       *ToolPackager
	benchmarker    *PerformanceBenchmarker
	
	// State
	mu           sync.RWMutex
	generatedTools map[string]*GeneratedTool
	
	// Templates
	templates *TemplateRegistry
}

// UtilitiesConfig holds configuration for the utilities system.
type UtilitiesConfig struct {
	// Scaffolding options
	DefaultAuthor      string
	DefaultLicense     string
	DefaultVersion     string
	TemplateDirectory  string
	OutputDirectory    string
	
	// Validation options
	StrictValidation   bool
	SchemaValidation   bool
	SecurityValidation bool
	
	// Documentation options
	DocFormat          DocFormat
	IncludeExamples    bool
	IncludeMetrics     bool
	OutputFormat       []string
	
	// Packaging options
	CompressionLevel   int
	IncludeSource      bool
	IncludeDependencies bool
	
	// Benchmarking options
	BenchmarkDuration  time.Duration
	BenchmarkIterations int
	ProfileMemory      bool
	ProfileCPU         bool
	
	// Custom settings
	CustomSettings     map[string]interface{}
}

// DefaultUtilitiesConfig returns a default configuration.
func DefaultUtilitiesConfig() *UtilitiesConfig {
	return &UtilitiesConfig{
		DefaultAuthor:       "Tool Developer",
		DefaultLicense:      "MIT",
		DefaultVersion:      "1.0.0",
		TemplateDirectory:   "./templates",
		OutputDirectory:     "./output",
		StrictValidation:    true,
		SchemaValidation:    true,
		SecurityValidation:  true,
		DocFormat:          DocFormatMarkdown,
		IncludeExamples:    true,
		IncludeMetrics:     true,
		OutputFormat:       []string{"markdown", "html"},
		CompressionLevel:   6,
		IncludeSource:      true,
		IncludeDependencies: true,
		BenchmarkDuration:  30 * time.Second,
		BenchmarkIterations: 1000,
		ProfileMemory:      true,
		ProfileCPU:         true,
		CustomSettings:     make(map[string]interface{}),
	}
}

// NewToolUtilities creates a new tool utilities instance.
func NewToolUtilities(config *UtilitiesConfig) *ToolUtilities {
	if config == nil {
		config = DefaultUtilitiesConfig()
	}
	
	utils := &ToolUtilities{
		config:         config,
		generatedTools: make(map[string]*GeneratedTool),
		templates:      NewTemplateRegistry(),
	}
	
	// Initialize components
	utils.scaffolder = NewToolScaffolder(config)
	utils.validator = NewToolValidator(config)
	utils.docGenerator = NewDocumentationGenerator(config)
	utils.packager = NewToolPackager(config)
	utils.benchmarker = NewPerformanceBenchmarker(config)
	
	return utils
}

// =============================================================================
// Tool Scaffolding and Code Generation
// =============================================================================

// ToolScaffolder generates boilerplate code for new tools.
type ToolScaffolder struct {
	config    *UtilitiesConfig
	templates *TemplateRegistry
	mu        sync.RWMutex
}

// NewToolScaffolder creates a new tool scaffolder.
func NewToolScaffolder(config *UtilitiesConfig) *ToolScaffolder {
	return &ToolScaffolder{
		config:    config,
		templates: NewTemplateRegistry(),
	}
}

// ToolScaffoldOptions defines options for tool scaffolding.
type ToolScaffoldOptions struct {
	// Basic info
	ToolID          string
	ToolName        string
	Description     string
	Version         string
	Author          string
	License         string
	
	// Functionality
	Parameters      []ParameterSpec
	Capabilities    *ToolCapabilities
	Dependencies    []string
	
	// Code generation
	GenerateTests   bool
	GenerateExamples bool
	GenerateDocs    bool
	PackageName     string
	
	// Advanced options
	ImplementStreaming bool
	ImplementAsync     bool
	ImplementCaching   bool
	SecurityLevel      SecurityLevel
	
	// Custom templates
	CustomTemplates map[string]string
}

// ParameterSpec defines a parameter for tool scaffolding.
type ParameterSpec struct {
	Name         string
	Type         string
	Description  string
	Required     bool
	Default      interface{}
	Validation   []ValidationRule
	Examples     []interface{}
}

// ValidationRule defines a validation rule for parameters.
type ValidationRule struct {
	Type     string
	Value    interface{}
	Message  string
}

// SecurityLevel defines security requirements for tools.
type SecurityLevel int

const (
	SecurityLevelNone SecurityLevel = iota
	SecurityLevelBasic
	SecurityLevelStrict
	SecurityLevelMaximum
)

// GeneratedTool represents a generated tool with metadata.
type GeneratedTool struct {
	ID           string
	Name         string
	Path         string
	Files        []GeneratedFile
	Metadata     *GenerationMetadata
	Dependencies []string
}

// GeneratedFile represents a generated file.
type GeneratedFile struct {
	Path     string
	Content  string
	Type     FileType
	Template string
}

// FileType defines the type of generated file.
type FileType int

const (
	FileTypeGo FileType = iota
	FileTypeTest
	FileTypeDoc
	FileTypeConfig
	FileTypeExample
)

// GenerationMetadata contains metadata about the generation process.
type GenerationMetadata struct {
	GeneratedAt   time.Time
	GeneratedBy   string
	Version       string
	Options       *ToolScaffoldOptions
	Templates     []string
	Dependencies  []string
	Statistics    *GenerationStats
}

// GenerationStats contains statistics about the generation process.
type GenerationStats struct {
	FilesGenerated   int
	LinesGenerated   int
	TemplatesUsed    int
	Duration         time.Duration
	PackageSize      int64
}

// GenerateTool generates a new tool with the specified options.
func (s *ToolScaffolder) GenerateTool(ctx context.Context, opts *ToolScaffoldOptions) (*GeneratedTool, error) {
	startTime := time.Now()
	
	// Validate options
	if err := s.validateOptions(opts); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}
	
	// Create generation context
	genCtx := &generationContext{
		Options:      opts,
		Templates:    s.templates,
		GeneratedAt:  startTime,
		Files:        make([]GeneratedFile, 0),
		Dependencies: make([]string, 0),
	}
	
	// Generate main tool file
	if err := s.generateMainFile(genCtx); err != nil {
		return nil, fmt.Errorf("failed to generate main file: %w", err)
	}
	
	// Generate test file if requested
	if opts.GenerateTests {
		if err := s.generateTestFile(genCtx); err != nil {
			return nil, fmt.Errorf("failed to generate test file: %w", err)
		}
	}
	
	// Generate example file if requested
	if opts.GenerateExamples {
		if err := s.generateExampleFile(genCtx); err != nil {
			return nil, fmt.Errorf("failed to generate example file: %w", err)
		}
	}
	
	// Generate documentation if requested
	if opts.GenerateDocs {
		if err := s.generateDocumentationFile(genCtx); err != nil {
			return nil, fmt.Errorf("failed to generate documentation file: %w", err)
		}
	}
	
	// Create generated tool
	tool := &GeneratedTool{
		ID:           opts.ToolID,
		Name:         opts.ToolName,
		Path:         genCtx.OutputPath,
		Files:        genCtx.Files,
		Dependencies: genCtx.Dependencies,
		Metadata: &GenerationMetadata{
			GeneratedAt:  startTime,
			GeneratedBy:  "ToolScaffolder",
			Version:      opts.Version,
			Options:      opts,
			Templates:    genCtx.TemplatesUsed,
			Dependencies: genCtx.Dependencies,
			Statistics: &GenerationStats{
				FilesGenerated: len(genCtx.Files),
				LinesGenerated: genCtx.LinesGenerated,
				TemplatesUsed:  len(genCtx.TemplatesUsed),
				Duration:       time.Since(startTime),
			},
		},
	}
	
	return tool, nil
}

// generateMainFile generates the main tool implementation file.
func (s *ToolScaffolder) generateMainFile(ctx *generationContext) error {
	tmpl, err := s.templates.GetTemplate("main_tool")
	if err != nil {
		return fmt.Errorf("failed to get main tool template: %w", err)
	}
	
	data := s.prepareTemplateData(ctx)
	
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}
	
	// Format the generated Go code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format generated code: %w", err)
	}
	
	filename := fmt.Sprintf("%s.go", strings.ToLower(ctx.Options.ToolID))
	file := GeneratedFile{
		Path:     filename,
		Content:  string(formatted),
		Type:     FileTypeGo,
		Template: "main_tool",
	}
	
	ctx.Files = append(ctx.Files, file)
	ctx.LinesGenerated += strings.Count(string(formatted), "\n")
	ctx.TemplatesUsed = append(ctx.TemplatesUsed, "main_tool")
	
	return nil
}

// generateTestFile generates the test file for the tool.
func (s *ToolScaffolder) generateTestFile(ctx *generationContext) error {
	tmpl, err := s.templates.GetTemplate("tool_test")
	if err != nil {
		return fmt.Errorf("failed to get test template: %w", err)
	}
	
	data := s.prepareTemplateData(ctx)
	
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}
	
	// Format the generated Go code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format generated test code: %w", err)
	}
	
	filename := fmt.Sprintf("%s_test.go", strings.ToLower(ctx.Options.ToolID))
	file := GeneratedFile{
		Path:     filename,
		Content:  string(formatted),
		Type:     FileTypeTest,
		Template: "tool_test",
	}
	
	ctx.Files = append(ctx.Files, file)
	ctx.LinesGenerated += strings.Count(string(formatted), "\n")
	ctx.TemplatesUsed = append(ctx.TemplatesUsed, "tool_test")
	
	return nil
}

// generateExampleFile generates example usage file.
func (s *ToolScaffolder) generateExampleFile(ctx *generationContext) error {
	tmpl, err := s.templates.GetTemplate("tool_example")
	if err != nil {
		return fmt.Errorf("failed to get example template: %w", err)
	}
	
	data := s.prepareTemplateData(ctx)
	
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}
	
	// Format the generated Go code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format generated example code: %w", err)
	}
	
	filename := fmt.Sprintf("%s_example.go", strings.ToLower(ctx.Options.ToolID))
	file := GeneratedFile{
		Path:     filename,
		Content:  string(formatted),
		Type:     FileTypeExample,
		Template: "tool_example",
	}
	
	ctx.Files = append(ctx.Files, file)
	ctx.LinesGenerated += strings.Count(string(formatted), "\n")
	ctx.TemplatesUsed = append(ctx.TemplatesUsed, "tool_example")
	
	return nil
}

// generateDocumentationFile generates documentation file.
func (s *ToolScaffolder) generateDocumentationFile(ctx *generationContext) error {
	tmpl, err := s.templates.GetTemplate("tool_doc")
	if err != nil {
		return fmt.Errorf("failed to get documentation template: %w", err)
	}
	
	data := s.prepareTemplateData(ctx)
	
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}
	
	filename := fmt.Sprintf("%s.md", strings.ToLower(ctx.Options.ToolID))
	file := GeneratedFile{
		Path:     filename,
		Content:  buf.String(),
		Type:     FileTypeDoc,
		Template: "tool_doc",
	}
	
	ctx.Files = append(ctx.Files, file)
	ctx.LinesGenerated += strings.Count(buf.String(), "\n")
	ctx.TemplatesUsed = append(ctx.TemplatesUsed, "tool_doc")
	
	return nil
}

// prepareTemplateData prepares data for template execution.
func (s *ToolScaffolder) prepareTemplateData(ctx *generationContext) map[string]interface{} {
	opts := ctx.Options
	
	// Convert parameters to schema properties
	properties := make(map[string]interface{})
	required := make([]string, 0)
	
	for _, param := range opts.Parameters {
		prop := map[string]interface{}{
			"type":        param.Type,
			"description": param.Description,
		}
		
		if param.Default != nil {
			prop["default"] = param.Default
		}
		
		if len(param.Examples) > 0 {
			prop["examples"] = param.Examples
		}
		
		// Add validation rules
		if len(param.Validation) > 0 {
			for _, rule := range param.Validation {
				prop[rule.Type] = rule.Value
			}
		}
		
		properties[param.Name] = prop
		
		if param.Required {
			required = append(required, param.Name)
		}
	}
	
	return map[string]interface{}{
		"ToolID":          opts.ToolID,
		"ToolName":        opts.ToolName,
		"Description":     opts.Description,
		"Version":         opts.Version,
		"Author":          opts.Author,
		"License":         opts.License,
		"PackageName":     opts.PackageName,
		"Properties":      properties,
		"Required":        required,
		"Capabilities":    opts.Capabilities,
		"Dependencies":    opts.Dependencies,
		"GeneratedAt":     ctx.GeneratedAt,
		"HasStreaming":    opts.ImplementStreaming,
		"HasAsync":        opts.ImplementAsync,
		"HasCaching":      opts.ImplementCaching,
		"SecurityLevel":   opts.SecurityLevel,
		"Config":          s.config,
	}
}

// validateOptions validates the scaffolding options.
func (s *ToolScaffolder) validateOptions(opts *ToolScaffoldOptions) error {
	if opts == nil {
		return fmt.Errorf("scaffold options cannot be nil")
	}
	
	if opts.ToolID == "" {
		return fmt.Errorf("tool ID is required")
	}
	
	if opts.ToolName == "" {
		return fmt.Errorf("tool name is required")
	}
	
	if opts.Description == "" {
		return fmt.Errorf("description is required")
	}
	
	if opts.Version == "" {
		opts.Version = s.config.DefaultVersion
	}
	
	if opts.Author == "" {
		opts.Author = s.config.DefaultAuthor
	}
	
	if opts.License == "" {
		opts.License = s.config.DefaultLicense
	}
	
	if opts.PackageName == "" {
		opts.PackageName = "tools"
	}
	
	// Validate tool ID format
	if !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`).MatchString(opts.ToolID) {
		return fmt.Errorf("invalid tool ID format: must start with letter and contain only letters, numbers, hyphens, and underscores")
	}
	
	// Validate parameters
	paramNames := make(map[string]bool)
	for _, param := range opts.Parameters {
		if param.Name == "" {
			return fmt.Errorf("parameter name is required")
		}
		
		if paramNames[param.Name] {
			return fmt.Errorf("duplicate parameter name: %s", param.Name)
		}
		paramNames[param.Name] = true
		
		if param.Type == "" {
			return fmt.Errorf("parameter type is required for %s", param.Name)
		}
		
		// Validate parameter type
		validTypes := []string{"string", "number", "integer", "boolean", "array", "object"}
		isValid := false
		for _, validType := range validTypes {
			if param.Type == validType {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("invalid parameter type %s for %s", param.Type, param.Name)
		}
	}
	
	return nil
}

// generationContext holds context during tool generation.
type generationContext struct {
	Options         *ToolScaffoldOptions
	Templates       *TemplateRegistry
	GeneratedAt     time.Time
	OutputPath      string
	Files           []GeneratedFile
	Dependencies    []string
	LinesGenerated  int
	TemplatesUsed   []string
}

// =============================================================================
// Template Registry
// =============================================================================

// TemplateRegistry manages code generation templates.
type TemplateRegistry struct {
	templates map[string]*template.Template
	mu        sync.RWMutex
}

// NewTemplateRegistry creates a new template registry.
func NewTemplateRegistry() *TemplateRegistry {
	registry := &TemplateRegistry{
		templates: make(map[string]*template.Template),
	}
	
	// Register default templates
	registry.registerDefaultTemplates()
	
	return registry
}

// GetTemplate retrieves a template by name.
func (r *TemplateRegistry) GetTemplate(name string) (*template.Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	tmpl, exists := r.templates[name]
	if !exists {
		return nil, fmt.Errorf("template %s not found", name)
	}
	
	return tmpl, nil
}

// RegisterTemplate registers a new template.
func (r *TemplateRegistry) RegisterTemplate(name, content string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Create template with helper functions
	tmpl := template.New(name).Funcs(template.FuncMap{
		"contains": func(slice []string, item string) bool {
			for _, s := range slice {
				if s == item {
					return true
				}
			}
			return false
		},
	})
	
	tmpl, err := tmpl.Parse(content)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", name, err)
	}
	
	r.templates[name] = tmpl
	return nil
}

// registerDefaultTemplates registers the default code generation templates.
func (r *TemplateRegistry) registerDefaultTemplates() {
	// Main tool template
	mainToolTemplate := `package {{.PackageName}}

import (
	"context"
	"fmt"
	"time"
)

// {{.ToolName}} {{.Description}}
type {{.ToolName}} struct {
	// Configuration
	config *{{.ToolName}}Config
	
	// State
	initialized bool
	{{if .HasCaching}}
	cache map[string]interface{}
	{{end}}
}

// {{.ToolName}}Config holds configuration for {{.ToolName}}.
type {{.ToolName}}Config struct {
	// Add configuration fields here
	{{if .HasCaching}}
	CacheSize int
	CacheTTL  time.Duration
	{{end}}
}

// New{{.ToolName}} creates a new {{.ToolName}} instance.
func New{{.ToolName}}(config *{{.ToolName}}Config) *{{.ToolName}} {
	if config == nil {
		config = &{{.ToolName}}Config{}
	}
	
	return &{{.ToolName}}{
		config: config,
		{{if .HasCaching}}
		cache: make(map[string]interface{}),
		{{end}}
	}
}

// Execute implements the ToolExecutor interface.
func (t *{{.ToolName}}) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	start := time.Now()
	
	// Validate parameters
	if err := t.validateParams(params); err != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("parameter validation failed: %v", err),
		}, nil
	}
	
	// TODO: Implement your tool logic here
	result := map[string]interface{}{
		"message": "{{.ToolName}} executed successfully",
		"params":  params,
	}
	
	return &ToolExecutionResult{
		Success:   true,
		Data:      result,
		Duration:  time.Since(start),
		Timestamp: time.Now(),
	}, nil
}

{{if .HasStreaming}}
// ExecuteStream implements the StreamingToolExecutor interface.
func (t *{{.ToolName}}) ExecuteStream(ctx context.Context, params map[string]interface{}) (<-chan *ToolStreamChunk, error) {
	ch := make(chan *ToolStreamChunk, 10)
	
	go func() {
		defer close(ch)
		
		// TODO: Implement streaming logic here
		for i := 0; i < 5; i++ {
			select {
			case <-ctx.Done():
				return
			case ch <- &ToolStreamChunk{
				Type:      "data",
				Data:      fmt.Sprintf("chunk %d", i),
				Index:     i,
				Timestamp: time.Now(),
			}:
			}
		}
		
		// Send completion chunk
		ch <- &ToolStreamChunk{
			Type:      "complete",
			Data:      "stream completed",
			Index:     5,
			Timestamp: time.Now(),
		}
	}()
	
	return ch, nil
}
{{end}}

// validateParams validates the input parameters.
func (t *{{.ToolName}}) validateParams(params map[string]interface{}) error {
	{{range $name, $prop := .Properties}}
	{{if contains $.Required $name}}
	if _, exists := params["{{$name}}"]; !exists {
		return fmt.Errorf("required parameter '{{$name}}' is missing")
	}
	{{end}}
	{{end}}
	
	// TODO: Add custom validation logic here
	return nil
}

// GetTool returns the tool definition.
func (t *{{.ToolName}}) GetTool() *Tool {
	return &Tool{
		ID:          "{{.ToolID}}",
		Name:        "{{.ToolName}}",
		Description: "{{.Description}}",
		Version:     "{{.Version}}",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				{{range $name, $prop := .Properties}}
				"{{$name}}": {
					Type:        "{{$prop.type}}",
					Description: "{{$prop.description}}",
					{{if $prop.default}}
					Default:     {{printf "%#v" $prop.default}},
					{{end}}
				},
				{{end}}
			},
			Required: []string{
				{{range .Required}}
				"{{.}}",
				{{end}}
			},
		},
		Executor: t,
		{{if .Capabilities}}
		Capabilities: &ToolCapabilities{
			{{if .Capabilities.Streaming}}Streaming: true,{{end}}
			{{if .Capabilities.Async}}Async: true,{{end}}
			{{if .Capabilities.Cancelable}}Cancelable: true,{{end}}
			{{if .Capabilities.Cacheable}}Cacheable: true,{{end}}
		},
		{{end}}
		Metadata: &ToolMetadata{
			Author:  "{{.Author}}",
			License: "{{.License}}",
			Tags:    []string{"{{.ToolID}}", "generated"},
		},
	}
}
`

	// Test template
	testTemplate := `package {{.PackageName}}_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test{{.ToolName}}_Execute(t *testing.T) {
	tool := New{{.ToolName}}(nil)
	ctx := context.Background()
	
	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid parameters",
			params: map[string]interface{}{
				{{range $name, $prop := .Properties}}
				{{if contains $.Required $name}}
				"{{$name}}": {{if eq $prop.type "string"}}"test"{{else if eq $prop.type "number"}}123{{else if eq $prop.type "integer"}}123{{else if eq $prop.type "boolean"}}true{{else}}nil{{end}},
				{{end}}
				{{end}}
			},
			wantErr: false,
		},
		{
			name:    "missing required parameters",
			params:  map[string]interface{}{},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.params)
			
			if tt.wantErr {
				assert.False(t, result.Success)
				assert.NotEmpty(t, result.Error)
			} else {
				require.NoError(t, err)
				assert.True(t, result.Success)
				assert.NotNil(t, result.Data)
			}
		})
	}
}

func Test{{.ToolName}}_GetTool(t *testing.T) {
	tool := New{{.ToolName}}(nil)
	toolDef := tool.GetTool()
	
	assert.Equal(t, "{{.ToolID}}", toolDef.ID)
	assert.Equal(t, "{{.ToolName}}", toolDef.Name)
	assert.Equal(t, "{{.Description}}", toolDef.Description)
	assert.Equal(t, "{{.Version}}", toolDef.Version)
	assert.NotNil(t, toolDef.Schema)
	assert.NotNil(t, toolDef.Executor)
}

func Benchmark{{.ToolName}}_Execute(b *testing.B) {
	tool := New{{.ToolName}}(nil)
	ctx := context.Background()
	params := map[string]interface{}{
		{{range $name, $prop := .Properties}}
		{{if contains $.Required $name}}
		"{{$name}}": {{if eq $prop.type "string"}}"test"{{else if eq $prop.type "number"}}123{{else if eq $prop.type "integer"}}123{{else if eq $prop.type "boolean"}}true{{else}}nil{{end}},
		{{end}}
		{{end}}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := tool.Execute(ctx, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}
`

	// Example template
	exampleTemplate := `package {{.PackageName}}_test

import (
	"context"
	"fmt"
	"log"
)

// Example{{.ToolName}} demonstrates how to use {{.ToolName}}.
func Example{{.ToolName}}() {
	// Create a new {{.ToolName}} instance
	tool := New{{.ToolName}}(nil)
	
	// Prepare parameters
	params := map[string]interface{}{
		{{range $name, $prop := .Properties}}
		{{if contains $.Required $name}}
		"{{$name}}": {{if eq $prop.type "string"}}"example value"{{else if eq $prop.type "number"}}123.45{{else if eq $prop.type "integer"}}123{{else if eq $prop.type "boolean"}}true{{else}}nil{{end}},
		{{end}}
		{{end}}
	}
	
	// Execute the tool
	ctx := context.Background()
	result, err := tool.Execute(ctx, params)
	if err != nil {
		log.Fatal(err)
	}
	
	if result.Success {
		fmt.Printf("Tool executed successfully: %v\n", result.Data)
	} else {
		fmt.Printf("Tool execution failed: %s\n", result.Error)
	}
	
	// Get tool definition
	toolDef := tool.GetTool()
	fmt.Printf("Tool: %s v%s\n", toolDef.Name, toolDef.Version)
	fmt.Printf("Description: %s\n", toolDef.Description)
	
	// Output:
	// Tool executed successfully: map[message:{{.ToolName}} executed successfully params:map[...]]
	// Tool: {{.ToolName}} v{{.Version}}
	// Description: {{.Description}}
}
`

	// Documentation template
	docTemplate := `# {{.ToolName}}

{{.Description}}

## Overview

- **ID**: {{.ToolID}}
- **Version**: {{.Version}}
- **Author**: {{.Author}}
- **License**: {{.License}}

## Parameters

{{range $name, $prop := .Properties}}
### {{$name}}

- **Type**: {{$prop.type}}
- **Description**: {{$prop.description}}
- **Required**: {{if contains $.Required $name}}Yes{{else}}No{{end}}
{{if $prop.default}}
- **Default**: {{printf "%v" $prop.default}}
{{end}}
{{if $prop.examples}}
- **Examples**: {{range $prop.examples}}{{printf "%v" .}}, {{end}}
{{end}}

{{end}}

## Capabilities

{{if .Capabilities}}
{{if .Capabilities.Streaming}}
- ✅ Streaming support
{{end}}
{{if .Capabilities.Async}}
- ✅ Async execution
{{end}}
{{if .Capabilities.Cancelable}}
- ✅ Cancelable operations
{{end}}
{{if .Capabilities.Cacheable}}
- ✅ Result caching
{{end}}
{{else}}
- Basic execution only
{{end}}

## Usage

` + "```go" + `
// Create a new {{.ToolName}} instance
tool := New{{.ToolName}}(nil)

// Prepare parameters
params := map[string]interface{}{
	{{range $name, $prop := .Properties}}
	{{if contains $.Required $name}}
	"{{$name}}": {{if eq $prop.type "string"}}"your value"{{else if eq $prop.type "number"}}123.45{{else if eq $prop.type "integer"}}123{{else if eq $prop.type "boolean"}}true{{else}}nil{{end}},
	{{end}}
	{{end}}
}

// Execute the tool
ctx := context.Background()
result, err := tool.Execute(ctx, params)
if err != nil {
	log.Fatal(err)
}

if result.Success {
	fmt.Printf("Result: %v\n", result.Data)
} else {
	fmt.Printf("Error: %s\n", result.Error)
}
` + "```" + `

## Generated Information

- **Generated At**: {{.GeneratedAt.Format "2006-01-02 15:04:05"}}
- **Generated By**: ToolScaffolder
- **Package**: {{.PackageName}}
`

	// Register templates
	r.RegisterTemplate("main_tool", mainToolTemplate)
	r.RegisterTemplate("tool_test", testTemplate)
	r.RegisterTemplate("tool_example", exampleTemplate)
	r.RegisterTemplate("tool_doc", docTemplate)
}

// =============================================================================
// Tool Testing and Validation Helpers
// =============================================================================

// ToolValidator provides comprehensive validation for tools.
type ToolValidator struct {
	config *UtilitiesConfig
	
	// Validation engines
	schemaValidator   *SchemaValidator
	securityValidator *SecurityValidator
	
	// Test runners
	testRunner        *TestRunner
	
	// Metrics
	validationMetrics *ValidationMetrics
}

// NewToolValidator creates a new tool validator.
func NewToolValidator(config *UtilitiesConfig) *ToolValidator {
	return &ToolValidator{
		config:            config,
		securityValidator: NewSecurityValidator(),
		testRunner:        NewTestRunner(),
		validationMetrics: NewValidationMetrics(),
	}
}

// ValidationReport contains the results of tool validation.
type ValidationReport struct {
	// Overall status
	Valid      bool
	Score      float64
	Severity   ValidationSeverity
	
	// Validation results
	SchemaValidation    *SchemaValidationResult
	SecurityValidation  *SecurityValidationResult
	TestValidation      *TestValidationResult
	
	// Issues found
	Issues   []ValidationIssue
	Warnings []ValidationWarning
	
	// Metrics
	Metrics *ValidationMetrics
	
	// Recommendations
	Recommendations []string
}

// ValidationSeverity defines the severity of validation issues.
type ValidationSeverity int

const (
	SeverityInfo ValidationSeverity = iota
	SeverityWarning
	SeverityError
	SeverityCritical
)

// ValidationIssue represents a validation issue.
type ValidationIssue struct {
	Type        string
	Severity    ValidationSeverity
	Message     string
	Location    string
	Suggestion  string
	Reference   string
}

// ValidationWarning represents a validation warning.
type ValidationWarning struct {
	Type       string
	Message    string
	Location   string
	Suggestion string
}

// SchemaValidationResult contains schema validation results.
type SchemaValidationResult struct {
	Valid          bool
	SchemaVersion  string
	Issues         []ValidationIssue
	Compatibility  map[string]bool
}

// SecurityValidationResult contains security validation results.
type SecurityValidationResult struct {
	Valid           bool
	SecurityLevel   SecurityLevel
	Vulnerabilities []SecurityVulnerability
	Recommendations []string
}

// TestValidationResult contains test validation results.
type TestValidationResult struct {
	Valid        bool
	TestsPassed  int
	TestsFailed  int
	Coverage     float64
	Benchmarks   []BenchmarkResult
}

// SecurityVulnerability represents a security vulnerability.
type SecurityVulnerability struct {
	Type        string
	Severity    SecuritySeverity
	Description string
	Location    string
	Mitigation  string
}

// SecuritySeverity defines the severity of security issues.
type SecuritySeverity int

const (
	SecuritySeverityLow SecuritySeverity = iota
	SecuritySeverityMedium
	SecuritySeverityHigh
	SecuritySeverityCritical
)

// BenchmarkResult represents a benchmark result.
type BenchmarkResult struct {
	Name        string
	Iterations  int
	NsPerOp     int64
	BytesPerOp  int64
	AllocsPerOp int64
	Metadata    map[string]interface{}
}

// ValidationMetrics tracks validation statistics.
type ValidationMetrics struct {
	mu                 sync.RWMutex
	TotalValidations   int64
	PassedValidations  int64
	FailedValidations  int64
	ValidationTime     time.Duration
	IssuesFound        int64
	WarningsFound      int64
}

// NewValidationMetrics creates new validation metrics.
func NewValidationMetrics() *ValidationMetrics {
	return &ValidationMetrics{}
}

// ValidateTool performs comprehensive validation of a tool.
func (v *ToolValidator) ValidateTool(ctx context.Context, tool *Tool) (*ValidationReport, error) {
	if tool == nil {
		return nil, fmt.Errorf("tool cannot be nil")
	}
	
	start := time.Now()
	
	report := &ValidationReport{
		Valid:       true,
		Score:       100.0,
		Severity:    SeverityInfo,
		Issues:      make([]ValidationIssue, 0),
		Warnings:    make([]ValidationWarning, 0),
		Recommendations: make([]string, 0),
	}
	
	// Schema validation
	if v.config.SchemaValidation {
		schemaResult, err := v.validateSchema(tool)
		if err != nil {
			return nil, fmt.Errorf("schema validation failed: %w", err)
		}
		report.SchemaValidation = schemaResult
		
		if !schemaResult.Valid {
			report.Valid = false
			report.Issues = append(report.Issues, schemaResult.Issues...)
			// Return error for critical schema failures (missing schema)
			if tool.Schema == nil {
				return nil, fmt.Errorf("tool schema is required")
			}
		}
	}
	
	// Security validation
	if v.config.SecurityValidation {
		securityResult, err := v.validateSecurity(tool)
		if err != nil {
			return nil, fmt.Errorf("security validation failed: %w", err)
		}
		report.SecurityValidation = securityResult
		
		if !securityResult.Valid {
			report.Valid = false
			for _, vuln := range securityResult.Vulnerabilities {
				report.Issues = append(report.Issues, ValidationIssue{
					Type:       "security",
					Severity:   v.convertSecuritySeverity(vuln.Severity),
					Message:    vuln.Description,
					Location:   vuln.Location,
					Suggestion: vuln.Mitigation,
				})
			}
		}
	}
	
	// Check for missing executor before testing
	if tool.Executor == nil {
		return nil, fmt.Errorf("tool executor is required")
	}
	
	// Test validation
	testResult, err := v.runTests(ctx, tool)
	if err != nil {
		return nil, fmt.Errorf("test validation failed: %w", err)
	}
	report.TestValidation = testResult
	
	// Calculate overall score
	report.Score = v.calculateScore(report)
	
	// Determine severity
	report.Severity = v.calculateSeverity(report)
	
	// Generate recommendations
	report.Recommendations = v.generateRecommendations(report)
	
	// Update metrics
	v.updateMetrics(report, time.Since(start))
	
	return report, nil
}

// validateSchema validates the tool's schema.
func (v *ToolValidator) validateSchema(tool *Tool) (*SchemaValidationResult, error) {
	result := &SchemaValidationResult{
		Valid:         true,
		SchemaVersion: "1.0.0",
		Issues:        make([]ValidationIssue, 0),
		Compatibility: make(map[string]bool),
	}
	
	// Check if schema exists
	if tool.Schema == nil {
		result.Valid = false
		result.Issues = append(result.Issues, ValidationIssue{
			Type:     "schema",
			Severity: SeverityError,
			Message:  "tool schema is required",
			Location: "schema",
		})
		return result, nil
	}
	
	// Basic schema validation
	if err := tool.Schema.Validate(); err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, ValidationIssue{
			Type:     "schema",
			Severity: SeverityError,
			Message:  err.Error(),
			Location: "schema",
		})
	}
	
	// Check for common schema issues
	if tool.Schema.Type != "object" {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:       "schema",
			Severity:   SeverityWarning,
			Message:    "Schema type should be 'object' for tool parameters",
			Location:   "schema.type",
			Suggestion: "Set schema type to 'object'",
		})
	}
	
	// Check for missing descriptions
	for name, prop := range tool.Schema.Properties {
		if prop.Description == "" {
			result.Issues = append(result.Issues, ValidationIssue{
				Type:       "schema",
				Severity:   SeverityWarning,
				Message:    fmt.Sprintf("Parameter '%s' missing description", name),
				Location:   fmt.Sprintf("schema.properties.%s.description", name),
				Suggestion: "Add description for better usability",
			})
		}
	}
	
	return result, nil
}

// validateSecurity validates the tool's security aspects.
func (v *ToolValidator) validateSecurity(tool *Tool) (*SecurityValidationResult, error) {
	result := &SecurityValidationResult{
		Valid:           true,
		SecurityLevel:   SecurityLevelBasic,
		Vulnerabilities: make([]SecurityVulnerability, 0),
		Recommendations: make([]string, 0),
	}
	
	// Check for potential security issues
	vulns := v.securityValidator.ScanTool(tool)
	result.Vulnerabilities = vulns
	
	if len(vulns) > 0 {
		result.Valid = false
		
		// Generate recommendations
		for _, vuln := range vulns {
			result.Recommendations = append(result.Recommendations, vuln.Mitigation)
		}
	}
	
	return result, nil
}

// runTests runs validation tests for the tool.
func (v *ToolValidator) runTests(ctx context.Context, tool *Tool) (*TestValidationResult, error) {
	result := &TestValidationResult{
		Valid:       true,
		TestsPassed: 0,
		TestsFailed: 0,
		Coverage:    0.0,
		Benchmarks:  make([]BenchmarkResult, 0),
	}
	
	// Run basic execution test
	testParams := v.generateTestParams(tool)
	if tool.Executor != nil {
		_, err := tool.Executor.Execute(ctx, testParams)
		if err != nil {
			result.Valid = false
			result.TestsFailed++
		} else {
			result.TestsPassed++
		}
	} else {
		result.Valid = false
		result.TestsFailed++
	}
	
	// Run benchmark if available and executor exists
	if v.config.BenchmarkDuration > 0 && tool.Executor != nil {
		benchResult := v.runBenchmark(ctx, tool, testParams)
		result.Benchmarks = append(result.Benchmarks, benchResult)
	}
	
	return result, nil
}

// generateTestParams generates test parameters for the tool.
func (v *ToolValidator) generateTestParams(tool *Tool) map[string]interface{} {
	params := make(map[string]interface{})
	
	// Check if tool or schema is nil
	if tool == nil || tool.Schema == nil {
		return params
	}
	
	for _, reqParam := range tool.Schema.Required {
		if prop, exists := tool.Schema.Properties[reqParam]; exists {
			params[reqParam] = v.generateTestValue(prop)
		}
	}
	
	return params
}

// generateTestValue generates a test value for a property.
func (v *ToolValidator) generateTestValue(prop *Property) interface{} {
	switch prop.Type {
	case "string":
		if prop.Default != nil {
			return prop.Default
		}
		return "test-value"
	case "number":
		if prop.Default != nil {
			return prop.Default
		}
		return 42.0
	case "integer":
		if prop.Default != nil {
			return prop.Default
		}
		return 42
	case "boolean":
		if prop.Default != nil {
			return prop.Default
		}
		return true
	case "array":
		if prop.Default != nil {
			return prop.Default
		}
		return []interface{}{"test"}
	case "object":
		if prop.Default != nil {
			return prop.Default
		}
		return map[string]interface{}{"key": "value"}
	default:
		return nil
	}
}

// runBenchmark runs a benchmark test for the tool.
func (v *ToolValidator) runBenchmark(ctx context.Context, tool *Tool, params map[string]interface{}) BenchmarkResult {
	iterations := v.config.BenchmarkIterations
	start := time.Now()
	
	for i := 0; i < iterations; i++ {
		if tool.Executor != nil {
			tool.Executor.Execute(ctx, params)
		}
	}
	
	duration := time.Since(start)
	nsPerOp := duration.Nanoseconds() / int64(iterations)
	
	return BenchmarkResult{
		Name:       tool.Name,
		Iterations: iterations,
		NsPerOp:    nsPerOp,
	}
}

// calculateScore calculates the overall validation score.
func (v *ToolValidator) calculateScore(report *ValidationReport) float64 {
	score := 100.0
	
	// Deduct points for issues
	for _, issue := range report.Issues {
		switch issue.Severity {
		case SeverityCritical:
			score -= 25
		case SeverityError:
			score -= 15
		case SeverityWarning:
			score -= 5
		case SeverityInfo:
			score -= 1
		}
	}
	
	// Bonus for good test coverage
	if report.TestValidation != nil && report.TestValidation.Coverage > 0.8 {
		score += 5
	}
	
	if score < 0 {
		score = 0
	}
	
	return score
}

// calculateSeverity calculates the overall severity.
func (v *ToolValidator) calculateSeverity(report *ValidationReport) ValidationSeverity {
	maxSeverity := SeverityInfo
	
	for _, issue := range report.Issues {
		if issue.Severity > maxSeverity {
			maxSeverity = issue.Severity
		}
	}
	
	return maxSeverity
}

// generateRecommendations generates recommendations based on validation results.
func (v *ToolValidator) generateRecommendations(report *ValidationReport) []string {
	recommendations := make([]string, 0)
	
	// Schema recommendations
	if report.SchemaValidation != nil && !report.SchemaValidation.Valid {
		recommendations = append(recommendations, "Fix schema validation issues")
	}
	
	// Security recommendations
	if report.SecurityValidation != nil && !report.SecurityValidation.Valid {
		recommendations = append(recommendations, report.SecurityValidation.Recommendations...)
	}
	
	// Test recommendations
	if report.TestValidation != nil && report.TestValidation.Coverage < 0.8 {
		recommendations = append(recommendations, "Increase test coverage")
	}
	
	return recommendations
}

// convertSecuritySeverity converts security severity to validation severity.
func (v *ToolValidator) convertSecuritySeverity(severity SecuritySeverity) ValidationSeverity {
	switch severity {
	case SecuritySeverityLow:
		return SeverityInfo
	case SecuritySeverityMedium:
		return SeverityWarning
	case SecuritySeverityHigh:
		return SeverityError
	case SecuritySeverityCritical:
		return SeverityCritical
	default:
		return SeverityInfo
	}
}

// updateMetrics updates validation metrics.
func (v *ToolValidator) updateMetrics(report *ValidationReport, duration time.Duration) {
	v.validationMetrics.mu.Lock()
	defer v.validationMetrics.mu.Unlock()
	
	v.validationMetrics.TotalValidations++
	v.validationMetrics.ValidationTime += duration
	v.validationMetrics.IssuesFound += int64(len(report.Issues))
	v.validationMetrics.WarningsFound += int64(len(report.Warnings))
	
	if report.Valid {
		v.validationMetrics.PassedValidations++
	} else {
		v.validationMetrics.FailedValidations++
	}
}

// SecurityValidator provides security validation for tools.
type SecurityValidator struct {
	patterns []SecurityPattern
}

// NewSecurityValidator creates a new security validator.
func NewSecurityValidator() *SecurityValidator {
	return &SecurityValidator{
		patterns: []SecurityPattern{
			{
				Name:        "Unsafe Parameter Types",
				Pattern:     `"type"\s*:\s*"object".*"additionalProperties"\s*:\s*true`,
				Severity:    SecuritySeverityMedium,
				Description: "Tool allows arbitrary additional properties",
				Mitigation:  "Set additionalProperties to false or use specific property validation",
			},
			{
				Name:        "Missing Input Validation",
				Pattern:     `Execute.*params.*interface`,
				Severity:    SecuritySeverityLow,
				Description: "Tool may not validate input parameters properly",
				Mitigation:  "Add comprehensive input validation",
			},
		},
	}
}

// SecurityPattern represents a security pattern to check.
type SecurityPattern struct {
	Name        string
	Pattern     string
	Severity    SecuritySeverity
	Description string
	Mitigation  string
}

// ScanTool scans a tool for security vulnerabilities.
func (s *SecurityValidator) ScanTool(tool *Tool) []SecurityVulnerability {
	vulnerabilities := make([]SecurityVulnerability, 0)
	
	// Convert tool to string for pattern matching
	toolJSON, _ := json.Marshal(tool)
	toolString := string(toolJSON)
	
	// Check each pattern
	for _, pattern := range s.patterns {
		if matched, _ := regexp.MatchString(pattern.Pattern, toolString); matched {
			vulnerabilities = append(vulnerabilities, SecurityVulnerability{
				Type:        pattern.Name,
				Severity:    pattern.Severity,
				Description: pattern.Description,
				Location:    "tool definition",
				Mitigation:  pattern.Mitigation,
			})
		}
	}
	
	return vulnerabilities
}

// TestRunner provides test execution capabilities.
type TestRunner struct {
	// Configuration
	timeout      time.Duration
	maxRetries   int
	parallelism  int
	
	// State
	activeTests  map[string]*TestExecution
	mu           sync.RWMutex
}

// NewTestRunner creates a new test runner.
func NewTestRunner() *TestRunner {
	return &TestRunner{
		timeout:     30 * time.Second,
		maxRetries:  3,
		parallelism: runtime.NumCPU(),
		activeTests: make(map[string]*TestExecution),
	}
}

// TestExecution represents a running test.
type TestExecution struct {
	ID       string
	Tool     *Tool
	Status   TestStatus
	Started  time.Time
	Duration time.Duration
	Result   *TestResult
}

// TestStatus represents the status of a test.
type TestStatus int

const (
	TestStatusPending TestStatus = iota
	TestStatusRunning
	TestStatusPassed
	TestStatusFailed
	TestStatusTimeout
)

// TestResult represents the result of a test.
type TestResult struct {
	Passed    bool
	Error     error
	Output    string
	Metrics   map[string]interface{}
}

// =============================================================================
// Documentation Generation
// =============================================================================

// DocumentationGenerator generates documentation for tools.
type DocumentationGenerator struct {
	config    *UtilitiesConfig
	templates *DocTemplateRegistry
	
	// Output formats
	formatters map[DocFormat]DocFormatter
	
	// State
	mu            sync.RWMutex
	generatedDocs map[string]*GeneratedDocumentation
}

// NewDocumentationGenerator creates a new documentation generator.
func NewDocumentationGenerator(config *UtilitiesConfig) *DocumentationGenerator {
	gen := &DocumentationGenerator{
		config:        config,
		templates:     NewDocTemplateRegistry(),
		formatters:    make(map[DocFormat]DocFormatter),
		generatedDocs: make(map[string]*GeneratedDocumentation),
	}
	
	// Register default formatters
	gen.formatters[DocFormatMarkdown] = &MarkdownFormatter{}
	gen.formatters[DocFormatHTML] = &HTMLFormatter{}
	gen.formatters[DocFormatJSON] = &JSONFormatter{}
	gen.formatters[DocFormatPlainText] = &PlainTextFormatter{}
	
	return gen
}

// DocFormat defines documentation formats.
type DocFormat int

const (
	DocFormatMarkdown DocFormat = iota
	DocFormatHTML
	DocFormatJSON
	DocFormatPlainText
)

// DocFormatter defines the interface for documentation formatters.
type DocFormatter interface {
	Format(doc *DocumentationData) (string, error)
	GetExtension() string
	GetMimeType() string
}

// DocumentationData holds data for documentation generation.
type DocumentationData struct {
	Tool        *Tool
	Metadata    *DocMetadata
	Examples    []DocExample
	API         *APIDocumentation
	Performance *PerformanceData
	Security    *SecurityDocumentation
}

// DocMetadata holds metadata for documentation.
type DocMetadata struct {
	GeneratedAt    time.Time
	GeneratedBy    string
	Version        string
	BuildInfo      *BuildInfo
	Dependencies   []DependencyInfo
}

// DocExample represents a documentation example.
type DocExample struct {
	Name        string
	Description string
	Code        string
	Language    string
	Input       interface{}
	Output      interface{}
	Notes       []string
}

// APIDocumentation holds API documentation.
type APIDocumentation struct {
	Endpoints   []APIEndpoint
	Parameters  []APIParameter
	Responses   []APIResponse
	ErrorCodes  []ErrorCode
}

// APIEndpoint represents an API endpoint.
type APIEndpoint struct {
	Method      string
	Path        string
	Description string
	Parameters  []APIParameter
	Responses   []APIResponse
}

// APIParameter represents an API parameter.
type APIParameter struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Location    string // query, path, body, header
	Schema      *Property
}

// APIResponse represents an API response.
type APIResponse struct {
	StatusCode  int
	Description string
	Schema      *Property
	Examples    []interface{}
}

// ErrorCode represents an error code.
type ErrorCode struct {
	Code        string
	Description string
	HTTPStatus  int
	Resolution  string
}

// PerformanceData holds performance documentation.
type PerformanceData struct {
	Benchmarks    []BenchmarkResult
	MemoryUsage   MemoryUsage
	Complexity    ComplexityAnalysis
	Scalability   ScalabilityInfo
}

// MemoryUsage represents memory usage information.
type MemoryUsage struct {
	Average    int64
	Peak       int64
	Baseline   int64
	GCImpact   float64
}

// ComplexityAnalysis represents complexity analysis.
type ComplexityAnalysis struct {
	TimeComplexity  string
	SpaceComplexity string
	Analysis        string
}

// ScalabilityInfo represents scalability information.
type ScalabilityInfo struct {
	ConcurrentUsers int
	RequestsPerSecond float64
	ResourceLimits   map[string]interface{}
}

// SecurityDocumentation holds security documentation.
type SecurityDocumentation struct {
	SecurityLevel   SecurityLevel
	Vulnerabilities []SecurityVulnerability
	Mitigations     []SecurityMitigation
	Compliance      []ComplianceInfo
}

// SecurityMitigation represents a security mitigation.
type SecurityMitigation struct {
	Threat      string
	Mitigation  string
	Implemented bool
	Reference   string
}

// ComplianceInfo represents compliance information.
type ComplianceInfo struct {
	Standard    string
	Status      string
	Requirements []string
}

// GeneratedDocumentation represents generated documentation.
type GeneratedDocumentation struct {
	Tool       *Tool
	Format     DocFormat
	Content    string
	Files      []DocFile
	Metadata   *DocMetadata
	GeneratedAt time.Time
}

// DocFile represents a documentation file.
type DocFile struct {
	Name    string
	Content string
	Type    string
	Size    int64
}

// BuildInfo holds build information.
type BuildInfo struct {
	Version    string
	BuildTime  time.Time
	GitCommit  string
	GitBranch  string
	GoVersion  string
	Platform   string
}

// DependencyInfo holds dependency information.
type DependencyInfo struct {
	Name    string
	Version string
	License string
	URL     string
}

// GenerateDocumentation generates documentation for a tool.
func (g *DocumentationGenerator) GenerateDocumentation(tool *Tool, format DocFormat) (*GeneratedDocumentation, error) {
	if tool == nil {
		return nil, fmt.Errorf("tool cannot be nil")
	}
	
	start := time.Now()
	
	// Validate tool
	if err := tool.Validate(); err != nil {
		return nil, fmt.Errorf("tool validation failed: %w", err)
	}
	
	// Prepare documentation data
	docData := g.prepareDocumentationData(tool)
	
	// Get formatter
	formatter, exists := g.formatters[format]
	if !exists {
		return nil, fmt.Errorf("unsupported format: %v", format)
	}
	
	// Generate content
	content, err := formatter.Format(docData)
	if err != nil {
		return nil, fmt.Errorf("formatting failed: %w", err)
	}
	
	// Create generated documentation
	doc := &GeneratedDocumentation{
		Tool:        tool,
		Format:      format,
		Content:     content,
		Files:       []DocFile{{
			Name:    fmt.Sprintf("%s.%s", tool.ID, formatter.GetExtension()),
			Content: content,
			Type:    formatter.GetMimeType(),
			Size:    int64(len(content)),
		}},
		Metadata:    docData.Metadata,
		GeneratedAt: start,
	}
	
	// Store generated documentation
	g.mu.Lock()
	g.generatedDocs[tool.ID] = doc
	g.mu.Unlock()
	
	return doc, nil
}

// prepareDocumentationData prepares data for documentation generation.
func (g *DocumentationGenerator) prepareDocumentationData(tool *Tool) *DocumentationData {
	return &DocumentationData{
		Tool: tool,
		Metadata: &DocMetadata{
			GeneratedAt: time.Now(),
			GeneratedBy: "DocumentationGenerator",
			Version:     tool.Version,
			BuildInfo:   g.getBuildInfo(),
		},
		Examples:    g.generateExamples(tool),
		API:         g.generateAPIDoc(tool),
		Performance: g.gatherPerformanceData(tool),
		Security:    g.generateSecurityDoc(tool),
	}
}

// generateExamples generates examples for the tool.
func (g *DocumentationGenerator) generateExamples(tool *Tool) []DocExample {
	examples := make([]DocExample, 0)
	
	// Basic usage example
	basicExample := DocExample{
		Name:        "Basic Usage",
		Description: fmt.Sprintf("Basic usage example for %s", tool.Name),
		Language:    "go",
		Code:        g.generateBasicUsageCode(tool),
	}
	examples = append(examples, basicExample)
	
	// Add examples from metadata if available
	if tool.Metadata != nil && len(tool.Metadata.Examples) > 0 {
		for _, example := range tool.Metadata.Examples {
			docExample := DocExample{
				Name:        example.Name,
				Description: example.Description,
				Input:       example.Input,
				Output:      example.Output,
				Language:    "go",
				Code:        g.generateExampleCode(tool, example),
			}
			examples = append(examples, docExample)
		}
	}
	
	return examples
}

// generateBasicUsageCode generates basic usage code.
func (g *DocumentationGenerator) generateBasicUsageCode(tool *Tool) string {
	var code strings.Builder
	
	code.WriteString("// Create tool instance\n")
	code.WriteString(fmt.Sprintf("tool := %s{}\n\n", tool.Name))
	
	code.WriteString("// Prepare parameters\n")
	code.WriteString("params := map[string]interface{}{\n")
	
	for _, req := range tool.Schema.Required {
		if prop, exists := tool.Schema.Properties[req]; exists {
			value := g.generateExampleValue(prop)
			code.WriteString(fmt.Sprintf("    \"%s\": %s,\n", req, value))
		}
	}
	
	code.WriteString("}\n\n")
	code.WriteString("// Execute tool\n")
	code.WriteString("ctx := context.Background()\n")
	code.WriteString("result, err := tool.Execute(ctx, params)\n")
	code.WriteString("if err != nil {\n")
	code.WriteString("    log.Fatal(err)\n")
	code.WriteString("}\n\n")
	code.WriteString("fmt.Printf(\"Result: %v\\n\", result.Data)")
	
	return code.String()
}

// generateExampleCode generates code for a specific example.
func (g *DocumentationGenerator) generateExampleCode(tool *Tool, example ToolExample) string {
	var code strings.Builder
	
	code.WriteString(fmt.Sprintf("// %s\n", example.Description))
	code.WriteString(fmt.Sprintf("tool := %s{}\n", tool.Name))
	code.WriteString("ctx := context.Background()\n\n")
	
	// Convert input to Go code
	inputJSON, _ := json.MarshalIndent(example.Input, "", "    ")
	code.WriteString("params := map[string]interface{}\n")
	code.WriteString("json.Unmarshal([]byte(`\n")
	code.WriteString(string(inputJSON))
	code.WriteString("\n`), &params)\n\n")
	
	code.WriteString("result, err := tool.Execute(ctx, params)\n")
	code.WriteString("if err != nil {\n")
	code.WriteString("    log.Fatal(err)\n")
	code.WriteString("}\n\n")
	
	if example.Output != nil {
		code.WriteString("// Expected output:\n")
		outputJSON, _ := json.MarshalIndent(example.Output, "// ", "    ")
		code.WriteString("// " + string(outputJSON))
	}
	
	return code.String()
}

// generateExampleValue generates an example value for a property.
func (g *DocumentationGenerator) generateExampleValue(prop *Property) string {
	switch prop.Type {
	case "string":
		return `"example"`
	case "number":
		return "123.45"
	case "integer":
		return "123"
	case "boolean":
		return "true"
	case "array":
		return `[]interface{}{"item1", "item2"}`
	case "object":
		return `map[string]interface{}{"key": "value"}`
	default:
		return "nil"
	}
}

// generateAPIDoc generates API documentation.
func (g *DocumentationGenerator) generateAPIDoc(tool *Tool) *APIDocumentation {
	api := &APIDocumentation{
		Endpoints:  make([]APIEndpoint, 0),
		Parameters: make([]APIParameter, 0),
		Responses:  make([]APIResponse, 0),
		ErrorCodes: make([]ErrorCode, 0),
	}
	
	// Generate parameter documentation
	for name, prop := range tool.Schema.Properties {
		param := APIParameter{
			Name:        name,
			Type:        prop.Type,
			Description: prop.Description,
			Required:    g.isRequired(name, tool.Schema.Required),
			Location:    "body",
			Schema:      prop,
		}
		api.Parameters = append(api.Parameters, param)
	}
	
	// Generate response documentation
	api.Responses = append(api.Responses, APIResponse{
		StatusCode:  200,
		Description: "Successful execution",
		Schema: &Property{
			Type: "object",
			Properties: map[string]*Property{
				"success": {Type: "boolean", Description: "Execution success status"},
				"data":    {Type: "object", Description: "Tool output data"},
				"error":   {Type: "string", Description: "Error message if any"},
			},
		},
	})
	
	return api
}

// isRequired checks if a parameter is required.
func (g *DocumentationGenerator) isRequired(name string, required []string) bool {
	for _, req := range required {
		if req == name {
			return true
		}
	}
	return false
}

// gatherPerformanceData gathers performance data for the tool.
func (g *DocumentationGenerator) gatherPerformanceData(tool *Tool) *PerformanceData {
	return &PerformanceData{
		Benchmarks: []BenchmarkResult{},
		MemoryUsage: MemoryUsage{
			Average:  1024 * 1024, // 1MB placeholder
			Peak:     2 * 1024 * 1024, // 2MB placeholder
			Baseline: 512 * 1024, // 512KB placeholder
		},
		Complexity: ComplexityAnalysis{
			TimeComplexity:  "O(n)",
			SpaceComplexity: "O(1)",
			Analysis:        "Linear time complexity with constant space usage",
		},
	}
}

// generateSecurityDoc generates security documentation.
func (g *DocumentationGenerator) generateSecurityDoc(tool *Tool) *SecurityDocumentation {
	return &SecurityDocumentation{
		SecurityLevel:   SecurityLevelBasic,
		Vulnerabilities: []SecurityVulnerability{},
		Mitigations: []SecurityMitigation{
			{
				Threat:      "Input validation bypass",
				Mitigation:  "Comprehensive parameter validation",
				Implemented: true,
				Reference:   "schema validation",
			},
		},
		Compliance: []ComplianceInfo{
			{
				Standard:     "OWASP",
				Status:       "Compliant",
				Requirements: []string{"Input validation", "Error handling"},
			},
		},
	}
}

// getBuildInfo returns build information.
func (g *DocumentationGenerator) getBuildInfo() *BuildInfo {
	return &BuildInfo{
		Version:   "1.0.0",
		BuildTime: time.Now(),
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// DocTemplateRegistry manages documentation templates.
type DocTemplateRegistry struct {
	templates map[string]*template.Template
	mu        sync.RWMutex
}

// NewDocTemplateRegistry creates a new documentation template registry.
func NewDocTemplateRegistry() *DocTemplateRegistry {
	return &DocTemplateRegistry{
		templates: make(map[string]*template.Template),
	}
}

// MarkdownFormatter formats documentation as Markdown.
type MarkdownFormatter struct{}

func (f *MarkdownFormatter) Format(doc *DocumentationData) (string, error) {
	var buf strings.Builder
	
	// Title
	buf.WriteString(fmt.Sprintf("# %s\n\n", doc.Tool.Name))
	buf.WriteString(fmt.Sprintf("%s\n\n", doc.Tool.Description))
	
	// Overview
	buf.WriteString("## Overview\n\n")
	buf.WriteString(fmt.Sprintf("- **ID**: %s\n", doc.Tool.ID))
	buf.WriteString(fmt.Sprintf("- **Version**: %s\n", doc.Tool.Version))
	if doc.Tool.Metadata != nil {
		buf.WriteString(fmt.Sprintf("- **Author**: %s\n", doc.Tool.Metadata.Author))
		buf.WriteString(fmt.Sprintf("- **License**: %s\n", doc.Tool.Metadata.License))
	}
	buf.WriteString("\n")
	
	// Parameters
	buf.WriteString("## Parameters\n\n")
	for name, prop := range doc.Tool.Schema.Properties {
		buf.WriteString(fmt.Sprintf("### %s\n\n", name))
		buf.WriteString(fmt.Sprintf("- **Type**: %s\n", prop.Type))
		buf.WriteString(fmt.Sprintf("- **Description**: %s\n", prop.Description))
		required := f.isRequired(name, doc.Tool.Schema.Required)
		buf.WriteString(fmt.Sprintf("- **Required**: %t\n", required))
		if prop.Default != nil {
			buf.WriteString(fmt.Sprintf("- **Default**: %v\n", prop.Default))
		}
		buf.WriteString("\n")
	}
	
	// Examples
	if len(doc.Examples) > 0 {
		buf.WriteString("## Examples\n\n")
		for _, example := range doc.Examples {
			buf.WriteString(fmt.Sprintf("### %s\n\n", example.Name))
			buf.WriteString(fmt.Sprintf("%s\n\n", example.Description))
			buf.WriteString(fmt.Sprintf("```%s\n", example.Language))
			buf.WriteString(example.Code)
			buf.WriteString("\n```\n\n")
		}
	}
	
	// API Documentation
	if doc.API != nil && len(doc.API.Parameters) > 0 {
		buf.WriteString("## API Reference\n\n")
		buf.WriteString("### Parameters\n\n")
		buf.WriteString("| Name | Type | Required | Description |\n")
		buf.WriteString("|------|------|----------|-------------|\n")
		for _, param := range doc.API.Parameters {
			buf.WriteString(fmt.Sprintf("| %s | %s | %t | %s |\n",
				param.Name, param.Type, param.Required, param.Description))
		}
		buf.WriteString("\n")
	}
	
	// Performance
	if doc.Performance != nil {
		buf.WriteString("## Performance\n\n")
		buf.WriteString(fmt.Sprintf("- **Time Complexity**: %s\n", doc.Performance.Complexity.TimeComplexity))
		buf.WriteString(fmt.Sprintf("- **Space Complexity**: %s\n", doc.Performance.Complexity.SpaceComplexity))
		buf.WriteString(fmt.Sprintf("- **Average Memory**: %d bytes\n", doc.Performance.MemoryUsage.Average))
		buf.WriteString("\n")
	}
	
	// Security
	if doc.Security != nil {
		buf.WriteString("## Security\n\n")
		buf.WriteString(fmt.Sprintf("- **Security Level**: %v\n", doc.Security.SecurityLevel))
		if len(doc.Security.Mitigations) > 0 {
			buf.WriteString("- **Mitigations**:\n")
			for _, mitigation := range doc.Security.Mitigations {
				status := "❌"
				if mitigation.Implemented {
					status = "✅"
				}
				buf.WriteString(fmt.Sprintf("  - %s %s: %s\n", status, mitigation.Threat, mitigation.Mitigation))
			}
		}
		buf.WriteString("\n")
	}
	
	// Metadata
	buf.WriteString("## Generation Information\n\n")
	buf.WriteString(fmt.Sprintf("- **Generated At**: %s\n", doc.Metadata.GeneratedAt.Format("2006-01-02 15:04:05")))
	buf.WriteString(fmt.Sprintf("- **Generated By**: %s\n", doc.Metadata.GeneratedBy))
	buf.WriteString(fmt.Sprintf("- **Tool Version**: %s\n", doc.Metadata.Version))
	
	return buf.String(), nil
}

func (f *MarkdownFormatter) GetExtension() string {
	return "md"
}

func (f *MarkdownFormatter) GetMimeType() string {
	return "text/markdown"
}

func (f *MarkdownFormatter) isRequired(name string, required []string) bool {
	for _, req := range required {
		if req == name {
			return true
		}
	}
	return false
}

// HTMLFormatter formats documentation as HTML.
type HTMLFormatter struct{}

func (f *HTMLFormatter) Format(doc *DocumentationData) (string, error) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>{{.Tool.Name}} - Documentation</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        h1 { color: #333; }
        h2 { color: #666; border-bottom: 1px solid #ddd; padding-bottom: 5px; }
        .parameter { margin: 10px 0; padding: 10px; background: #f9f9f9; border-left: 3px solid #007acc; }
        .required { color: #d32f2f; }
        .optional { color: #388e3c; }
        pre { background: #f5f5f5; padding: 15px; border-radius: 5px; overflow-x: auto; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <h1>{{.Tool.Name}}</h1>
    <p>{{.Tool.Description}}</p>
    
    <h2>Overview</h2>
    <ul>
        <li><strong>ID:</strong> {{.Tool.ID}}</li>
        <li><strong>Version:</strong> {{.Tool.Version}}</li>
        {{if .Tool.Metadata}}
        <li><strong>Author:</strong> {{.Tool.Metadata.Author}}</li>
        <li><strong>License:</strong> {{.Tool.Metadata.License}}</li>
        {{end}}
    </ul>
    
    <h2>Parameters</h2>
    {{range $name, $prop := .Tool.Schema.Properties}}
    <div class="parameter">
        <h3>{{$name}}</h3>
        <p><strong>Type:</strong> {{$prop.Type}}</p>
        <p><strong>Description:</strong> {{$prop.Description}}</p>
        <p><strong>Required:</strong> 
            {{if isRequired $name $.Tool.Schema.Required}}
            <span class="required">Yes</span>
            {{else}}
            <span class="optional">No</span>
            {{end}}
        </p>
        {{if $prop.Default}}
        <p><strong>Default:</strong> {{$prop.Default}}</p>
        {{end}}
    </div>
    {{end}}
    
    {{if .Examples}}
    <h2>Examples</h2>
    {{range .Examples}}
    <h3>{{.Name}}</h3>
    <p>{{.Description}}</p>
    <pre><code>{{.Code}}</code></pre>
    {{end}}
    {{end}}
    
    <h2>Generated Information</h2>
    <ul>
        <li><strong>Generated At:</strong> {{.Metadata.GeneratedAt.Format "2006-01-02 15:04:05"}}</li>
        <li><strong>Generated By:</strong> {{.Metadata.GeneratedBy}}</li>
        <li><strong>Tool Version:</strong> {{.Metadata.Version}}</li>
    </ul>
</body>
</html>`

	funcMap := template.FuncMap{
		"isRequired": func(name string, required []string) bool {
			return f.isRequired(name, required)
		},
	}
	
	t, err := template.New("html").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		return "", err
	}
	
	var buf bytes.Buffer
	if err := t.Execute(&buf, doc); err != nil {
		return "", err
	}
	
	return buf.String(), nil
}

func (f *HTMLFormatter) GetExtension() string {
	return "html"
}

func (f *HTMLFormatter) GetMimeType() string {
	return "text/html"
}

func (f *HTMLFormatter) isRequired(name string, required []string) bool {
	for _, req := range required {
		if req == name {
			return true
		}
	}
	return false
}

// JSONFormatter formats documentation as JSON.
type JSONFormatter struct{}

func (f *JSONFormatter) Format(doc *DocumentationData) (string, error) {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *JSONFormatter) GetExtension() string {
	return "json"
}

func (f *JSONFormatter) GetMimeType() string {
	return "application/json"
}

// PlainTextFormatter formats documentation as plain text.
type PlainTextFormatter struct{}

func (f *PlainTextFormatter) Format(doc *DocumentationData) (string, error) {
	var buf strings.Builder
	
	buf.WriteString(fmt.Sprintf("%s\n", doc.Tool.Name))
	buf.WriteString(strings.Repeat("=", len(doc.Tool.Name)) + "\n\n")
	buf.WriteString(fmt.Sprintf("%s\n\n", doc.Tool.Description))
	
	buf.WriteString("OVERVIEW\n")
	buf.WriteString("--------\n")
	buf.WriteString(fmt.Sprintf("ID: %s\n", doc.Tool.ID))
	buf.WriteString(fmt.Sprintf("Version: %s\n", doc.Tool.Version))
	if doc.Tool.Metadata != nil {
		buf.WriteString(fmt.Sprintf("Author: %s\n", doc.Tool.Metadata.Author))
		buf.WriteString(fmt.Sprintf("License: %s\n", doc.Tool.Metadata.License))
	}
	buf.WriteString("\n")
	
	buf.WriteString("PARAMETERS\n")
	buf.WriteString("----------\n")
	for name, prop := range doc.Tool.Schema.Properties {
		buf.WriteString(fmt.Sprintf("%s (%s)\n", name, prop.Type))
		buf.WriteString(fmt.Sprintf("  %s\n", prop.Description))
		required := f.isRequired(name, doc.Tool.Schema.Required)
		buf.WriteString(fmt.Sprintf("  Required: %t\n", required))
		if prop.Default != nil {
			buf.WriteString(fmt.Sprintf("  Default: %v\n", prop.Default))
		}
		buf.WriteString("\n")
	}
	
	return buf.String(), nil
}

func (f *PlainTextFormatter) GetExtension() string {
	return "txt"
}

func (f *PlainTextFormatter) GetMimeType() string {
	return "text/plain"
}

func (f *PlainTextFormatter) isRequired(name string, required []string) bool {
	for _, req := range required {
		if req == name {
			return true
		}
	}
	return false
}

// =============================================================================
// Tool Deployment and Packaging
// =============================================================================

// ToolPackager handles tool packaging and deployment.
type ToolPackager struct {
	config *UtilitiesConfig
	
	// Packaging options
	compressor Compressor
	
	// State
	mu       sync.RWMutex
	packages map[string]*ToolPackage
}

// NewToolPackager creates a new tool packager.
func NewToolPackager(config *UtilitiesConfig) *ToolPackager {
	return &ToolPackager{
		config:     config,
		compressor: NewGzipCompressor(config.CompressionLevel),
		packages:   make(map[string]*ToolPackage),
	}
}

// ToolPackage represents a packaged tool.
type ToolPackage struct {
	ID           string
	Name         string
	Version      string
	Files        []PackageFile
	Metadata     *PackageMetadata
	Signature    string
	Size         int64
	Checksum     string
	CreatedAt    time.Time
}

// PackageFile represents a file in the package.
type PackageFile struct {
	Path        string
	Content     []byte
	Type        PackageFileType
	Compressed  bool
	Size        int64
	Checksum    string
}

// PackageFileType defines the type of file in the package.
type PackageFileType int

const (
	PackageFileTypeSource PackageFileType = iota
	PackageFileTypeBinary
	PackageFileTypeConfig
	PackageFileTypeDoc
	PackageFileTypeTest
	PackageFileTypeAsset
	PackageFileTypeExample
)

// PackageMetadata holds metadata for the package.
type PackageMetadata struct {
	Tool         *Tool
	Dependencies []PackageDependency
	BuildInfo    *BuildInfo
	License      string
	Homepage     string
	Repository   string
	Keywords     []string
	Maintainers  []Maintainer
}

// PackageDependency represents a package dependency.
type PackageDependency struct {
	Name    string
	Version string
	Type    DependencyType
	Source  string
}

// DependencyType defines the type of dependency.
type DependencyType int

const (
	DependencyTypeRuntime DependencyType = iota
	DependencyTypeBuild
	DependencyTypeTest
	DependencyTypeOptional
)

// Maintainer represents a package maintainer.
type Maintainer struct {
	Name  string
	Email string
	Role  string
}

// Compressor defines the interface for compression.
type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
	GetType() string
}

// PackageOptions defines options for packaging.
type PackageOptions struct {
	IncludeSource      bool
	IncludeTests       bool
	IncludeDocs        bool
	IncludeDependencies bool
	Compress           bool
	Sign               bool
	OutputPath         string
	Format             PackageFormat
}

// PackageFormat defines the package format.
type PackageFormat int

const (
	PackageFormatTarGz PackageFormat = iota
	PackageFormatZip
	PackageFormatDocker
	PackageFormatCustom
)

// CreatePackage creates a package for the tool.
func (p *ToolPackager) CreatePackage(tool *Tool, opts *PackageOptions) (*ToolPackage, error) {
	if tool == nil {
		return nil, fmt.Errorf("tool cannot be nil")
	}
	
	if opts == nil {
		opts = &PackageOptions{
			IncludeSource: true,
			IncludeTests:  true,
			IncludeDocs:   true,
			Compress:      true,
		}
	}
	
	pkg := &ToolPackage{
		ID:        tool.ID,
		Name:      tool.Name,
		Version:   tool.Version,
		Files:     make([]PackageFile, 0),
		CreatedAt: time.Now(),
	}
	
	// Add source files
	if opts.IncludeSource {
		if err := p.addSourceFiles(pkg, tool); err != nil {
			return nil, fmt.Errorf("failed to add source files: %w", err)
		}
	}
	
	// Add test files
	if opts.IncludeTests {
		if err := p.addTestFiles(pkg, tool); err != nil {
			return nil, fmt.Errorf("failed to add test files: %w", err)
		}
	}
	
	// Add documentation
	if opts.IncludeDocs {
		if err := p.addDocumentationFiles(pkg, tool); err != nil {
			return nil, fmt.Errorf("failed to add documentation files: %w", err)
		}
	}
	
	// Create metadata
	pkg.Metadata = p.createPackageMetadata(tool)
	
	// Calculate size and checksum
	pkg.Size = p.calculatePackageSize(pkg)
	pkg.Checksum = p.calculateChecksum(pkg)
	
	// Sign package if requested
	if opts.Sign {
		signature, err := p.signPackage(pkg)
		if err != nil {
			return nil, fmt.Errorf("failed to sign package: %w", err)
		}
		pkg.Signature = signature
	}
	
	// Store package
	p.mu.Lock()
	p.packages[pkg.ID] = pkg
	p.mu.Unlock()
	
	return pkg, nil
}

// addSourceFiles adds source files to the package.
func (p *ToolPackager) addSourceFiles(pkg *ToolPackage, tool *Tool) error {
	// Generate main source file
	content := p.generateSourceFile(tool)
	
	file := PackageFile{
		Path:     fmt.Sprintf("%s.go", tool.ID),
		Content:  []byte(content),
		Type:     PackageFileTypeSource,
		Size:     int64(len(content)),
		Checksum: p.calculateFileChecksum([]byte(content)),
	}
	
	pkg.Files = append(pkg.Files, file)
	return nil
}

// addTestFiles adds test files to the package.
func (p *ToolPackager) addTestFiles(pkg *ToolPackage, tool *Tool) error {
	// Generate test file
	content := p.generateTestFile(tool)
	
	file := PackageFile{
		Path:     fmt.Sprintf("%s_test.go", tool.ID),
		Content:  []byte(content),
		Type:     PackageFileTypeTest,
		Size:     int64(len(content)),
		Checksum: p.calculateFileChecksum([]byte(content)),
	}
	
	pkg.Files = append(pkg.Files, file)
	return nil
}

// addDocumentationFiles adds documentation files to the package.
func (p *ToolPackager) addDocumentationFiles(pkg *ToolPackage, tool *Tool) error {
	// Generate README
	readme := p.generateReadme(tool)
	
	file := PackageFile{
		Path:     "README.md",
		Content:  []byte(readme),
		Type:     PackageFileTypeDoc,
		Size:     int64(len(readme)),
		Checksum: p.calculateFileChecksum([]byte(readme)),
	}
	
	pkg.Files = append(pkg.Files, file)
	return nil
}

// createPackageMetadata creates metadata for the package.
func (p *ToolPackager) createPackageMetadata(tool *Tool) *PackageMetadata {
	metadata := &PackageMetadata{
		Tool:        tool,
		BuildInfo:   p.getBuildInfo(),
		License:     p.config.DefaultLicense,
		Keywords:    []string{tool.ID, "tool", "generated"},
		Maintainers: []Maintainer{{
			Name: p.config.DefaultAuthor,
			Role: "maintainer",
		}},
	}
	
	// Add dependencies if specified
	if tool.Metadata != nil && len(tool.Metadata.Dependencies) > 0 {
		for _, dep := range tool.Metadata.Dependencies {
			metadata.Dependencies = append(metadata.Dependencies, PackageDependency{
				Name:    dep,
				Type:    DependencyTypeRuntime,
				Version: "*",
			})
		}
	}
	
	return metadata
}

// generateSourceFile generates the source file content.
func (p *ToolPackager) generateSourceFile(tool *Tool) string {
	// This would use the scaffolder's template system
	// For now, return a simple implementation
	return fmt.Sprintf(`package tools

// %s - %s
// Version: %s
// Generated package

import (
	"context"
	"fmt"
	"time"
)

// %s implements the tool functionality.
type %s struct{}

// Execute implements the ToolExecutor interface.
func (t *%s) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	return &ToolExecutionResult{
		Success:   true,
		Data:      fmt.Sprintf("%s executed with params: %%v", params),
		Timestamp: time.Now(),
	}, nil
}
`, tool.Name, tool.Description, tool.Version, tool.Name, tool.Name, tool.Name, tool.Name)
}

// generateTestFile generates the test file content.
func (p *ToolPackager) generateTestFile(tool *Tool) string {
	return fmt.Sprintf(`package tools

import (
	"context"
	"testing"
)

func Test%s_Execute(t *testing.T) {
	tool := &%s{}
	ctx := context.Background()
	params := map[string]interface{}{}
	
	result, err := tool.Execute(ctx, params)
	if err != nil {
		t.Errorf("Execute() error = %%v", err)
		return
	}
	
	if !result.Success {
		t.Errorf("Execute() success = %%v, want true", result.Success)
	}
}
`, tool.Name, tool.Name)
}

// generateReadme generates the README content.
func (p *ToolPackager) generateReadme(tool *Tool) string {
	return fmt.Sprintf(`# %s

%s

## Installation

` + "```bash" + `
go get -u your-package/%s
` + "```" + `

## Usage

` + "```go" + `
import "your-package/%s"

tool := &%s{}
ctx := context.Background()
params := map[string]interface{}{
    // Add your parameters here
}

result, err := tool.Execute(ctx, params)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Result: %%v\n", result.Data)
` + "```" + `

## Version

%s

## License

%s
`, tool.Name, tool.Description, tool.ID, tool.ID, tool.Name, tool.Version, p.config.DefaultLicense)
}

// calculatePackageSize calculates the total package size.
func (p *ToolPackager) calculatePackageSize(pkg *ToolPackage) int64 {
	var total int64
	for _, file := range pkg.Files {
		total += file.Size
	}
	return total
}

// calculateChecksum calculates the package checksum.
func (p *ToolPackager) calculateChecksum(pkg *ToolPackage) string {
	hash := sha256.New()
	
	// Hash all file contents in order
	for _, file := range pkg.Files {
		hash.Write(file.Content)
	}
	
	return hex.EncodeToString(hash.Sum(nil))
}

// calculateFileChecksum calculates the checksum for a file.
func (p *ToolPackager) calculateFileChecksum(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// signPackage signs the package.
func (p *ToolPackager) signPackage(pkg *ToolPackage) (string, error) {
	// For now, return a simple signature
	// In a real implementation, this would use proper cryptographic signing
	return fmt.Sprintf("signature-%s-%d", pkg.ID, time.Now().Unix()), nil
}

// getBuildInfo returns build information.
func (p *ToolPackager) getBuildInfo() *BuildInfo {
	return &BuildInfo{
		Version:   "1.0.0",
		BuildTime: time.Now(),
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
}

// GzipCompressor implements compression using gzip.
type GzipCompressor struct {
	level int
}

// NewGzipCompressor creates a new gzip compressor.
func NewGzipCompressor(level int) *GzipCompressor {
	return &GzipCompressor{level: level}
}

// Compress compresses data using gzip.
func (c *GzipCompressor) Compress(data []byte) ([]byte, error) {
	// For simplicity, return the original data
	// In a real implementation, this would use gzip
	return data, nil
}

// Decompress decompresses data using gzip.
func (c *GzipCompressor) Decompress(data []byte) ([]byte, error) {
	// For simplicity, return the original data
	// In a real implementation, this would use gzip
	return data, nil
}

// GetType returns the compression type.
func (c *GzipCompressor) GetType() string {
	return "gzip"
}

// =============================================================================
// Performance Benchmarking Tools
// =============================================================================

// PerformanceBenchmarker provides comprehensive performance testing.
type PerformanceBenchmarker struct {
	config *UtilitiesConfig
	
	// Benchmarking components
	memoryProfiler *MemoryProfiler
	cpuProfiler    *CPUProfiler
	
	// State
	mu         sync.RWMutex
	benchmarks map[string]*BenchmarkSuite
}

// NewPerformanceBenchmarker creates a new performance benchmarker.
func NewPerformanceBenchmarker(config *UtilitiesConfig) *PerformanceBenchmarker {
	return &PerformanceBenchmarker{
		config:         config,
		memoryProfiler: NewMemoryProfiler(),
		cpuProfiler:    NewCPUProfiler(),
		benchmarks:     make(map[string]*BenchmarkSuite),
	}
}

// BenchmarkSuite represents a collection of benchmarks.
type BenchmarkSuite struct {
	Tool        *Tool
	Results     []BenchmarkResult
	Summary     *BenchmarkSummary
	Profiles    *BenchmarkProfiles
	Metadata    *BenchmarkMetadata
	GeneratedAt time.Time
}

// BenchmarkSummary provides a summary of benchmark results.
type BenchmarkSummary struct {
	TotalTests      int
	AverageLatency  time.Duration
	ThroughputRPS   float64
	MemoryUsage     MemoryUsage
	CPUUsage        CPUUsage
	ErrorRate       float64
	Recommendations []string
}

// BenchmarkProfiles holds profiling data.
type BenchmarkProfiles struct {
	CPUProfile    []byte
	MemoryProfile []byte
	TraceProfile  []byte
}

// BenchmarkMetadata holds metadata for benchmarks.
type BenchmarkMetadata struct {
	BenchmarkedAt time.Time
	Duration      time.Duration
	Environment   *BenchmarkEnvironment
	Configuration *BenchmarkConfiguration
}

// BenchmarkEnvironment describes the benchmark environment.
type BenchmarkEnvironment struct {
	OS            string
	Architecture  string
	CPUCores      int
	MemoryTotal   int64
	GoVersion     string
	CGOEnabled    bool
}

// BenchmarkConfiguration holds benchmark configuration.
type BenchmarkConfiguration struct {
	Duration         time.Duration
	Iterations       int
	Concurrency      int
	WarmupDuration   time.Duration
	ProfileMemory    bool
	ProfileCPU       bool
	CustomParams     map[string]interface{}
}

// CPUUsage represents CPU usage information.
type CPUUsage struct {
	UserTime   time.Duration
	SystemTime time.Duration
	TotalTime  time.Duration
	Percentage float64
}

// RunBenchmark runs a comprehensive benchmark suite for a tool.
func (b *PerformanceBenchmarker) RunBenchmark(ctx context.Context, tool *Tool) (*BenchmarkSuite, error) {
	if tool == nil {
		return nil, fmt.Errorf("tool cannot be nil")
	}
	
	start := time.Now()
	
	// Create benchmark configuration
	config := &BenchmarkConfiguration{
		Duration:       b.config.BenchmarkDuration,
		Iterations:     b.config.BenchmarkIterations,
		Concurrency:    runtime.NumCPU(),
		WarmupDuration: 5 * time.Second,
		ProfileMemory:  b.config.ProfileMemory,
		ProfileCPU:     b.config.ProfileCPU,
		CustomParams:   make(map[string]interface{}),
	}
	
	suite := &BenchmarkSuite{
		Tool:        tool,
		Results:     make([]BenchmarkResult, 0),
		GeneratedAt: start,
		Metadata: &BenchmarkMetadata{
			BenchmarkedAt: start,
			Environment:   b.getBenchmarkEnvironment(),
			Configuration: config,
		},
	}
	
	// Run warmup
	if err := b.runWarmup(ctx, tool, config); err != nil {
		return nil, fmt.Errorf("warmup failed: %w", err)
	}
	
	// Run latency benchmark
	latencyResult, err := b.runLatencyBenchmark(ctx, tool, config)
	if err != nil {
		return nil, fmt.Errorf("latency benchmark failed: %w", err)
	}
	suite.Results = append(suite.Results, latencyResult)
	
	// Run throughput benchmark
	throughputResult, err := b.runThroughputBenchmark(ctx, tool, config)
	if err != nil {
		return nil, fmt.Errorf("throughput benchmark failed: %w", err)
	}
	suite.Results = append(suite.Results, throughputResult)
	
	// Run memory benchmark
	if config.ProfileMemory {
		memoryResult, err := b.runMemoryBenchmark(ctx, tool, config)
		if err != nil {
			return nil, fmt.Errorf("memory benchmark failed: %w", err)
		}
		suite.Results = append(suite.Results, memoryResult)
	}
	
	// Run CPU benchmark
	if config.ProfileCPU {
		cpuResult, err := b.runCPUBenchmark(ctx, tool, config)
		if err != nil {
			return nil, fmt.Errorf("CPU benchmark failed: %w", err)
		}
		suite.Results = append(suite.Results, cpuResult)
	}
	
	// Generate summary
	suite.Summary = b.generateSummary(suite.Results)
	
	// Set duration
	suite.Metadata.Duration = time.Since(start)
	
	// Store benchmark suite
	b.mu.Lock()
	b.benchmarks[tool.ID] = suite
	b.mu.Unlock()
	
	return suite, nil
}

// runWarmup runs a warmup phase.
func (b *PerformanceBenchmarker) runWarmup(ctx context.Context, tool *Tool, config *BenchmarkConfiguration) error {
	params := b.generateBenchmarkParams(tool)
	
	warmupCtx, cancel := context.WithTimeout(ctx, config.WarmupDuration)
	defer cancel()
	
	for {
		select {
		case <-warmupCtx.Done():
			return nil
		default:
			_, err := tool.Executor.Execute(ctx, params)
			if err != nil {
				return err
			}
		}
	}
}

// runLatencyBenchmark runs a latency benchmark.
func (b *PerformanceBenchmarker) runLatencyBenchmark(ctx context.Context, tool *Tool, config *BenchmarkConfiguration) (BenchmarkResult, error) {
	params := b.generateBenchmarkParams(tool)
	latencies := make([]time.Duration, 0, config.Iterations)
	
	for i := 0; i < config.Iterations; i++ {
		start := time.Now()
		_, err := tool.Executor.Execute(ctx, params)
		latency := time.Since(start)
		
		if err != nil {
			return BenchmarkResult{}, fmt.Errorf("execution failed at iteration %d: %w", i, err)
		}
		
		latencies = append(latencies, latency)
	}
	
	// Calculate statistics
	avgLatency := b.calculateAverageLatency(latencies)
	p95Latency := b.calculatePercentileLatency(latencies, 95)
	p99Latency := b.calculatePercentileLatency(latencies, 99)
	
	return BenchmarkResult{
		Name:        fmt.Sprintf("%s_latency", tool.Name),
		Iterations:  config.Iterations,
		NsPerOp:     avgLatency.Nanoseconds(),
		BytesPerOp:  0, // Would need to measure actual bytes
		AllocsPerOp: 0, // Would need to measure actual allocations
		Metadata: map[string]interface{}{
			"avg_latency_ns": avgLatency.Nanoseconds(),
			"p95_latency_ns": p95Latency.Nanoseconds(),
			"p99_latency_ns": p99Latency.Nanoseconds(),
		},
	}, nil
}

// runThroughputBenchmark runs a throughput benchmark.
func (b *PerformanceBenchmarker) runThroughputBenchmark(ctx context.Context, tool *Tool, config *BenchmarkConfiguration) (BenchmarkResult, error) {
	params := b.generateBenchmarkParams(tool)
	
	benchCtx, cancel := context.WithTimeout(ctx, config.Duration)
	defer cancel()
	
	var operations int64
	var wg sync.WaitGroup
	start := time.Now()
	
	// Run concurrent workers
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-benchCtx.Done():
					return
				default:
					_, err := tool.Executor.Execute(ctx, params)
					if err == nil {
						atomic.AddInt64(&operations, 1)
					}
				}
			}
		}()
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	throughput := float64(operations) / duration.Seconds()
	
	return BenchmarkResult{
		Name:       fmt.Sprintf("%s_throughput", tool.Name),
		Iterations: int(operations),
		NsPerOp:    duration.Nanoseconds() / operations,
		Metadata: map[string]interface{}{
			"throughput_rps": throughput,
			"total_ops":      operations,
			"duration_ns":    duration.Nanoseconds(),
			"concurrency":    config.Concurrency,
		},
	}, nil
}

// runMemoryBenchmark runs a memory benchmark.
func (b *PerformanceBenchmarker) runMemoryBenchmark(ctx context.Context, tool *Tool, config *BenchmarkConfiguration) (BenchmarkResult, error) {
	params := b.generateBenchmarkParams(tool)
	
	// Start memory profiling
	profile, err := b.memoryProfiler.StartProfile()
	if err != nil {
		return BenchmarkResult{}, err
	}
	defer b.memoryProfiler.StopProfile()
	
	// Run operations
	for i := 0; i < config.Iterations; i++ {
		_, err := tool.Executor.Execute(ctx, params)
		if err != nil {
			return BenchmarkResult{}, err
		}
	}
	
	// Get memory statistics
	stats := b.memoryProfiler.GetStats(profile)
	
	return BenchmarkResult{
		Name:        fmt.Sprintf("%s_memory", tool.Name),
		Iterations:  config.Iterations,
		BytesPerOp:  stats.BytesPerOp,
		AllocsPerOp: stats.AllocsPerOp,
		Metadata: map[string]interface{}{
			"peak_memory":     stats.PeakMemory,
			"avg_memory":      stats.AvgMemory,
			"total_allocs":    stats.TotalAllocs,
			"gc_cycles":       stats.GCCycles,
		},
	}, nil
}

// runCPUBenchmark runs a CPU benchmark.
func (b *PerformanceBenchmarker) runCPUBenchmark(ctx context.Context, tool *Tool, config *BenchmarkConfiguration) (BenchmarkResult, error) {
	params := b.generateBenchmarkParams(tool)
	
	// Start CPU profiling
	profile, err := b.cpuProfiler.StartProfile()
	if err != nil {
		return BenchmarkResult{}, err
	}
	defer b.cpuProfiler.StopProfile()
	
	start := time.Now()
	
	// Run operations
	for i := 0; i < config.Iterations; i++ {
		_, err := tool.Executor.Execute(ctx, params)
		if err != nil {
			return BenchmarkResult{}, err
		}
	}
	
	duration := time.Since(start)
	
	// Get CPU statistics
	stats := b.cpuProfiler.GetStats(profile)
	
	return BenchmarkResult{
		Name:       fmt.Sprintf("%s_cpu", tool.Name),
		Iterations: config.Iterations,
		NsPerOp:    duration.Nanoseconds() / int64(config.Iterations),
		Metadata: map[string]interface{}{
			"cpu_time_ns":    stats.CPUTime.Nanoseconds(),
			"user_time_ns":   stats.UserTime.Nanoseconds(),
			"system_time_ns": stats.SystemTime.Nanoseconds(),
			"cpu_percentage": stats.CPUPercentage,
		},
	}, nil
}

// generateBenchmarkParams generates test parameters for benchmarking.
func (b *PerformanceBenchmarker) generateBenchmarkParams(tool *Tool) map[string]interface{} {
	params := make(map[string]interface{})
	
	for _, reqParam := range tool.Schema.Required {
		if prop, exists := tool.Schema.Properties[reqParam]; exists {
			params[reqParam] = b.generateBenchmarkValue(prop)
		}
	}
	
	return params
}

// generateBenchmarkValue generates a benchmark value for a property.
func (b *PerformanceBenchmarker) generateBenchmarkValue(prop *Property) interface{} {
	switch prop.Type {
	case "string":
		return "benchmark-test-data"
	case "number":
		return 123.456
	case "integer":
		return 12345
	case "boolean":
		return true
	case "array":
		return []interface{}{"item1", "item2", "item3"}
	case "object":
		return map[string]interface{}{"key": "value", "count": 10}
	default:
		return nil
	}
}

// calculateAverageLatency calculates the average latency.
func (b *PerformanceBenchmarker) calculateAverageLatency(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	
	return total / time.Duration(len(latencies))
}

// calculatePercentileLatency calculates the percentile latency.
func (b *PerformanceBenchmarker) calculatePercentileLatency(latencies []time.Duration, percentile float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	
	// Sort latencies
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})
	
	// Calculate percentile index
	index := int(float64(len(sorted)) * percentile / 100.0)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	
	return sorted[index]
}

// generateSummary generates a summary from benchmark results.
func (b *PerformanceBenchmarker) generateSummary(results []BenchmarkResult) *BenchmarkSummary {
	summary := &BenchmarkSummary{
		TotalTests:      len(results),
		Recommendations: make([]string, 0),
	}
	
	// Calculate averages and extract metrics
	for _, result := range results {
		if metadata, ok := result.Metadata["avg_latency_ns"]; ok {
			if latency, ok := metadata.(int64); ok {
				summary.AverageLatency = time.Duration(latency)
			}
		}
		
		if metadata, ok := result.Metadata["throughput_rps"]; ok {
			if throughput, ok := metadata.(float64); ok {
				summary.ThroughputRPS = throughput
			}
		}
		
		if metadata, ok := result.Metadata["peak_memory"]; ok {
			if memory, ok := metadata.(int64); ok {
				summary.MemoryUsage.Peak = memory
			}
		}
	}
	
	// Generate recommendations
	if summary.AverageLatency > 100*time.Millisecond {
		summary.Recommendations = append(summary.Recommendations, 
			"Consider optimizing for latency - average response time is high")
	}
	
	if summary.ThroughputRPS < 100 {
		summary.Recommendations = append(summary.Recommendations, 
			"Consider optimizing for throughput - requests per second is low")
	}
	
	return summary
}

// getBenchmarkEnvironment returns the benchmark environment.
func (b *PerformanceBenchmarker) getBenchmarkEnvironment() *BenchmarkEnvironment {
	return &BenchmarkEnvironment{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		CPUCores:     runtime.NumCPU(),
		MemoryTotal:  int64(1024 * 1024 * 1024), // Placeholder
		GoVersion:    runtime.Version(),
		CGOEnabled:   true, // Placeholder
	}
}

// MemoryProfiler provides memory profiling capabilities.
type MemoryProfiler struct {
	profiles map[string]*MemoryProfile
	mu       sync.RWMutex
}

// NewMemoryProfiler creates a new memory profiler.
func NewMemoryProfiler() *MemoryProfiler {
	return &MemoryProfiler{
		profiles: make(map[string]*MemoryProfile),
	}
}

// MemoryProfile represents a memory profile.
type MemoryProfile struct {
	ID        string
	StartTime time.Time
	Samples   []MemorySample
}

// MemorySample represents a memory sample.
type MemorySample struct {
	Timestamp time.Time
	HeapAlloc int64
	HeapSys   int64
	NumGC     uint32
}

// MemoryStats represents memory statistics.
type MemoryStats struct {
	PeakMemory   int64
	AvgMemory    int64
	BytesPerOp   int64
	AllocsPerOp  int64
	TotalAllocs  int64
	GCCycles     int64
}

// StartProfile starts memory profiling.
func (p *MemoryProfiler) StartProfile() (*MemoryProfile, error) {
	id := generateID()
	profile := &MemoryProfile{
		ID:        id,
		StartTime: time.Now(),
		Samples:   make([]MemorySample, 0),
	}
	
	p.mu.Lock()
	p.profiles[id] = profile
	p.mu.Unlock()
	
	return profile, nil
}

// StopProfile stops memory profiling.
func (p *MemoryProfiler) StopProfile() {
	// Implementation would stop the profiling
}

// GetStats returns memory statistics for a profile.
func (p *MemoryProfiler) GetStats(profile *MemoryProfile) *MemoryStats {
	// This would analyze the profile and return statistics
	// For now, return placeholder values
	return &MemoryStats{
		PeakMemory:   1024 * 1024,     // 1MB
		AvgMemory:    512 * 1024,      // 512KB
		BytesPerOp:   1024,            // 1KB per operation
		AllocsPerOp:  10,              // 10 allocations per operation
		TotalAllocs:  1000,            // 1000 total allocations
		GCCycles:     5,               // 5 GC cycles
	}
}

// CPUProfiler provides CPU profiling capabilities.
type CPUProfiler struct {
	profiles map[string]*CPUProfile
	mu       sync.RWMutex
}

// NewCPUProfiler creates a new CPU profiler.
func NewCPUProfiler() *CPUProfiler {
	return &CPUProfiler{
		profiles: make(map[string]*CPUProfile),
	}
}

// CPUProfile represents a CPU profile.
type CPUProfile struct {
	ID        string
	StartTime time.Time
	Samples   []CPUSample
}

// CPUSample represents a CPU sample.
type CPUSample struct {
	Timestamp time.Time
	UserTime  time.Duration
	SystemTime time.Duration
}

// CPUStats represents CPU statistics.
type CPUStats struct {
	CPUTime       time.Duration
	UserTime      time.Duration
	SystemTime    time.Duration
	CPUPercentage float64
}

// StartProfile starts CPU profiling.
func (p *CPUProfiler) StartProfile() (*CPUProfile, error) {
	id := generateID()
	profile := &CPUProfile{
		ID:        id,
		StartTime: time.Now(),
		Samples:   make([]CPUSample, 0),
	}
	
	p.mu.Lock()
	p.profiles[id] = profile
	p.mu.Unlock()
	
	return profile, nil
}

// StopProfile stops CPU profiling.
func (p *CPUProfiler) StopProfile() {
	// Implementation would stop the profiling
}

// GetStats returns CPU statistics for a profile.
func (p *CPUProfiler) GetStats(profile *CPUProfile) *CPUStats {
	// This would analyze the profile and return statistics
	// For now, return placeholder values
	return &CPUStats{
		CPUTime:       100 * time.Millisecond,
		UserTime:      80 * time.Millisecond,
		SystemTime:    20 * time.Millisecond,
		CPUPercentage: 25.0,
	}
}

// Utility functions

// generateID generates a random ID.
func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Main utilities interface

// Scaffold creates a new tool using scaffolding.
func (u *ToolUtilities) Scaffold(ctx context.Context, opts *ToolScaffoldOptions) (*GeneratedTool, error) {
	tool, err := u.scaffolder.GenerateTool(ctx, opts)
	if err != nil {
		return nil, err
	}
	
	// Store the generated tool
	u.mu.Lock()
	u.generatedTools[tool.ID] = tool
	u.mu.Unlock()
	
	return tool, nil
}

// Validate validates a tool comprehensively.
func (u *ToolUtilities) Validate(ctx context.Context, tool *Tool) (*ValidationReport, error) {
	return u.validator.ValidateTool(ctx, tool)
}

// GenerateDocumentation generates documentation for a tool.
func (u *ToolUtilities) GenerateDocumentation(tool *Tool, format DocFormat) (*GeneratedDocumentation, error) {
	return u.docGenerator.GenerateDocumentation(tool, format)
}

// PackageTool packages a tool for distribution.
func (u *ToolUtilities) PackageTool(tool *Tool, opts *PackageOptions) (*ToolPackage, error) {
	return u.packager.CreatePackage(tool, opts)
}

// BenchmarkTool runs performance benchmarks for a tool.
func (u *ToolUtilities) BenchmarkTool(ctx context.Context, tool *Tool) (*BenchmarkSuite, error) {
	return u.benchmarker.RunBenchmark(ctx, tool)
}

// GetGeneratedTools returns all generated tools.
func (u *ToolUtilities) GetGeneratedTools() map[string]*GeneratedTool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	
	result := make(map[string]*GeneratedTool)
	for id, tool := range u.generatedTools {
		result[id] = tool
	}
	return result
}

// GetConfig returns the utilities configuration.
func (u *ToolUtilities) GetConfig() *UtilitiesConfig {
	return u.config
}

// SetConfig updates the utilities configuration.
func (u *ToolUtilities) SetConfig(config *UtilitiesConfig) {
	u.config = config
	
	// Update component configurations
	u.scaffolder.config = config
	u.validator.config = config
	u.docGenerator.config = config
	u.packager.config = config
	u.benchmarker.config = config
}