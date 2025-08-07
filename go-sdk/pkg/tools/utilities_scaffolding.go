package tools

import (
	"context"
	"fmt"
	"regexp"
	"time"
)

// ToolScaffolder generates boilerplate code for new tools.
type ToolScaffolder struct {
	config *UtilitiesConfig
}

// NewToolScaffolder creates a new tool scaffolder.
func NewToolScaffolder(config *UtilitiesConfig) *ToolScaffolder {
	return &ToolScaffolder{
		config: config,
	}
}

// ToolScaffoldOptions defines options for tool scaffolding.
type ToolScaffoldOptions struct {
	// Basic info
	ToolID      string
	ToolName    string
	Description string
	Version     string
	Author      string
	License     string

	// Functionality
	Parameters   []ParameterSpec
	Capabilities *ToolCapabilities
	Dependencies []string

	// Code generation
	GenerateTests    bool
	GenerateExamples bool
	GenerateDocs     bool
	PackageName      string

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
	Name        string
	Type        string
	Description string
	Required    bool
	Default     interface{}
	Validation  []ValidationRule
	Examples    []interface{}
}

// ValidationRule defines a validation rule for parameters.
type ValidationRule struct {
	Type    string
	Value   interface{}
	Message string
}

// SecurityLevel defines security requirements for tools.
type SecurityLevel int

const (
	SecurityLevelNone SecurityLevel = iota
	SecurityLevelBasic
	SecurityLevelMedium
	SecurityLevelHigh
	SecurityLevelCritical
)

// GeneratedTool represents a generated tool with all its components.
type GeneratedTool struct {
	Options    *ToolScaffoldOptions
	SourceCode map[string]string
	Tests      map[string]string
	Examples   map[string]string
	Docs       map[string]string
	Config     map[string]string
	Metadata   map[string]interface{}
	CreatedAt  time.Time
}

// ScaffoldTool generates a new tool based on the provided options.
func (s *ToolScaffolder) ScaffoldTool(ctx context.Context, options *ToolScaffoldOptions) (*GeneratedTool, error) {
	// Validate options
	if err := s.validateOptions(options); err != nil {
		return nil, fmt.Errorf("invalid scaffold options: %w", err)
	}

	// Set defaults
	s.setDefaults(options)

	generated := &GeneratedTool{
		Options:    options,
		SourceCode: make(map[string]string),
		Tests:      make(map[string]string),
		Examples:   make(map[string]string),
		Docs:       make(map[string]string),
		Config:     make(map[string]string),
		Metadata:   make(map[string]interface{}),
		CreatedAt:  time.Now(),
	}

	// Generate basic source code
	generated.SourceCode["main.go"] = s.generateBasicSourceCode(options)

	// Generate basic schema
	generated.SourceCode["schema.json"] = s.generateSchemaJSON(options)

	return generated, nil
}

// validateOptions validates the scaffold options.
func (s *ToolScaffolder) validateOptions(options *ToolScaffoldOptions) error {
	if options == nil {
		return fmt.Errorf("scaffold options cannot be nil")
	}

	if options.ToolID == "" {
		return fmt.Errorf("tool ID is required")
	}

	if options.ToolName == "" {
		return fmt.Errorf("tool name is required")
	}

	// Validate tool ID format - must start with a letter and contain only letters, numbers, and hyphens
	if !isValidToolID(options.ToolID) {
		return fmt.Errorf("invalid tool ID format: %s", options.ToolID)
	}

	// Validate parameters
	paramNames := make(map[string]bool)
	for i, param := range options.Parameters {
		if param.Name == "" {
			return fmt.Errorf("parameter %d: name is required", i)
		}
		if param.Type == "" {
			return fmt.Errorf("parameter %s: type is required", param.Name)
		}
		if !isValidParameterType(param.Type) {
			return fmt.Errorf("parameter %s: invalid type %s", param.Name, param.Type)
		}
		// Check for duplicate parameter names
		if paramNames[param.Name] {
			return fmt.Errorf("duplicate parameter name: %s", param.Name)
		}
		paramNames[param.Name] = true
	}

	return nil
}

// setDefaults sets default values for scaffold options.
func (s *ToolScaffolder) setDefaults(options *ToolScaffoldOptions) {
	if options.Version == "" {
		options.Version = "1.0.0"
	}
	if options.Author == "" {
		options.Author = "Tool Developer"
	}
	if options.License == "" {
		options.License = "MIT"
	}
	if options.PackageName == "" {
		options.PackageName = "tools"
	}
	if options.Capabilities == nil {
		options.Capabilities = &ToolCapabilities{}
	}
}

// generateBasicSourceCode generates basic source code for the tool.
func (s *ToolScaffolder) generateBasicSourceCode(options *ToolScaffoldOptions) string {
	return fmt.Sprintf(`package %s

import (
	"context"
	"fmt"
	"time"
)

// %s implements the %s tool.
type %s struct {
	// Configuration
	config *Config
}

// Config holds configuration for %s.
type Config struct {
	// Add configuration fields here
}

// Execute implements the ToolExecutor interface.
func (t *%s) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	// Validate parameters
	if err := t.validateParams(params); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %%w", err)
	}
	
	// Process the request
	result := map[string]interface{}{
		"status": "success",
		"processed_at": time.Now(),
	}
	
	return &ToolExecutionResult{
		Success:   true,
		Data:      result,
		Timestamp: time.Now(),
	}, nil
}

// validateParams validates the input parameters.
func (t *%s) validateParams(params map[string]interface{}) error {
	// Add parameter validation logic here
	return nil
}
`, options.PackageName, options.ToolName, options.Description, options.ToolName, options.ToolName, options.ToolName, options.ToolName)
}

// generateSchemaJSON generates the JSON schema for the tool.
func (s *ToolScaffolder) generateSchemaJSON(options *ToolScaffoldOptions) string {
	if len(options.Parameters) == 0 {
		return `{
  "type": "object",
  "properties": {},
  "required": []
}`
	}

	// For simplicity, generate a basic schema
	return `{
  "type": "object",
  "properties": {
    "input": {
      "type": "string",
      "description": "Input parameter"
    }
  },
  "required": ["input"]
}`
}

// isValidToolID checks if a tool ID is valid.
// Tool ID must start with a letter and contain only letters, numbers, and hyphens.
func isValidToolID(id string) bool {
	// Tool ID must be lowercase alphanumeric with hyphens, but cannot start with a number
	validID := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	return validID.MatchString(id)
}

// isValidParameterType checks if a parameter type is valid.
func isValidParameterType(typ string) bool {
	// Support both JSON Schema types and Go types
	validTypes := []string{
		// JSON Schema types
		"string", "number", "integer", "boolean", "array", "object",
		// Go types
		"int", "int32", "int64", "uint", "uint32", "uint64",
		"float32", "float64", "bool", "[]string", "[]int", "[]float64",
		"map[string]interface{}", "interface{}",
	}

	for _, validType := range validTypes {
		if typ == validType {
			return true
		}
	}

	return false
}
