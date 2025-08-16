package clienttools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
)

// Registry manages registration and discovery of client-side tools
type Registry struct {
	mu       sync.RWMutex
	executor *Executor
	tools    map[string]*ToolDefinition
}

// ToolDefinition contains the full definition of a client-side tool
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Author      string                 `json:"author,omitempty"`
	Parameters  *ToolParameters        `json:"parameters,omitempty"`
	Examples    []ToolExample          `json:"examples,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ToolParameters defines the parameters for a tool
type ToolParameters struct {
	Type       string                    `json:"type"`
	Properties map[string]ParameterDef   `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

// ParameterDef defines a single parameter
type ParameterDef struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Default     interface{} `json:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
}

// ToolExample shows how to use a tool
type ToolExample struct {
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Expected    interface{}            `json:"expected,omitempty"`
}

// NewRegistry creates a new client-side tool registry
func NewRegistry() *Registry {
	return &Registry{
		executor: NewExecutor(),
		tools:    make(map[string]*ToolDefinition),
	}
}

// RegisterBuiltinTools registers all built-in client-side tools
func (r *Registry) RegisterBuiltinTools() error {
	// Register shell command tool
	if err := r.RegisterTool(&ToolDefinition{
		Name:        "shell_command",
		Description: "Execute a shell command locally",
		Category:    "system",
		Version:     "1.0.0",
		Parameters: &ToolParameters{
			Type: "object",
			Properties: map[string]ParameterDef{
				"command": {
					Type:        "string",
					Description: "The shell command to execute",
				},
				"working_dir": {
					Type:        "string",
					Description: "Working directory for command execution",
					Default:     ".",
				},
				"timeout": {
					Type:        "integer",
					Description: "Timeout in seconds",
					Default:     30,
				},
			},
			Required: []string{"command"},
		},
		Examples: []ToolExample{
			{
				Description: "List files in current directory",
				Parameters:  map[string]interface{}{"command": "ls -la"},
			},
		},
	}, NewShellCommandTool()); err != nil {
		return fmt.Errorf("failed to register shell_command: %w", err)
	}

	// Register file reader tool
	if err := r.RegisterTool(&ToolDefinition{
		Name:        "file_reader",
		Description: "Read contents of a local file",
		Category:    "filesystem",
		Version:     "1.0.0",
		Parameters: &ToolParameters{
			Type: "object",
			Properties: map[string]ParameterDef{
				"path": {
					Type:        "string",
					Description: "Path to the file to read",
				},
				"encoding": {
					Type:        "string",
					Description: "File encoding",
					Default:     "utf-8",
					Enum:        []string{"utf-8", "ascii", "base64"},
				},
			},
			Required: []string{"path"},
		},
		Examples: []ToolExample{
			{
				Description: "Read a text file",
				Parameters:  map[string]interface{}{"path": "/tmp/example.txt"},
			},
		},
	}, NewFileReaderTool()); err != nil {
		return fmt.Errorf("failed to register file_reader: %w", err)
	}

	// Register calculator tool
	if err := r.RegisterTool(&ToolDefinition{
		Name:        "calculator",
		Description: "Perform mathematical calculations",
		Category:    "utility",
		Version:     "1.0.0",
		Parameters: &ToolParameters{
			Type: "object",
			Properties: map[string]ParameterDef{
				"expression": {
					Type:        "string",
					Description: "Mathematical expression to evaluate",
				},
			},
			Required: []string{"expression"},
		},
		Examples: []ToolExample{
			{
				Description: "Calculate 2 + 2",
				Parameters:  map[string]interface{}{"expression": "2 + 2"},
				Expected:    4,
			},
		},
	}, NewCalculatorTool()); err != nil {
		return fmt.Errorf("failed to register calculator: %w", err)
	}

	// Register system info tool
	if err := r.RegisterTool(&ToolDefinition{
		Name:        "system_info",
		Description: "Get system information",
		Category:    "system",
		Version:     "1.0.0",
		Parameters: &ToolParameters{
			Type: "object",
			Properties: map[string]ParameterDef{
				"info_type": {
					Type:        "string",
					Description: "Type of system information to retrieve",
					Default:     "all",
					Enum:        []string{"all", "os", "cpu", "memory", "disk", "network"},
				},
			},
		},
		Examples: []ToolExample{
			{
				Description: "Get all system information",
				Parameters:  map[string]interface{}{"info_type": "all"},
			},
		},
	}, NewSystemInfoTool()); err != nil {
		return fmt.Errorf("failed to register system_info: %w", err)
	}

	return nil
}

// RegisterTool registers a tool with its definition and implementation
func (r *Registry) RegisterTool(definition *ToolDefinition, toolFunc ToolFunc) error {
	if definition == nil {
		return fmt.Errorf("tool definition cannot be nil")
	}
	if toolFunc == nil {
		return fmt.Errorf("tool function cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Register in executor
	tool := &Tool{
		Name:        definition.Name,
		Description: definition.Description,
		Parameters:  r.parametersToMap(definition.Parameters),
		Execute:     toolFunc,
		ParamDef:    definition.Parameters,
	}

	if err := r.executor.RegisterTool(tool); err != nil {
		return fmt.Errorf("failed to register tool in executor: %w", err)
	}

	// Store definition
	r.tools[definition.Name] = definition

	return nil
}

// LoadToolsFromDirectory loads tool definitions from JSON files in a directory
func (r *Registry) LoadToolsFromDirectory(dir string) error {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, file.Name())
		if err := r.LoadToolFromFile(path); err != nil {
			return fmt.Errorf("failed to load tool from %s: %w", path, err)
		}
	}

	return nil
}

// LoadToolFromFile loads a tool definition from a JSON file
func (r *Registry) LoadToolFromFile(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var definition ToolDefinition
	if err := json.Unmarshal(data, &definition); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// For external tools, create a generic executor that calls shell commands
	toolFunc := r.createExternalToolFunc(&definition)

	return r.RegisterTool(&definition, toolFunc)
}

// createExternalToolFunc creates a tool function for external tools
func (r *Registry) createExternalToolFunc(definition *ToolDefinition) ToolFunc {
	return func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		// This is a placeholder - in a real implementation, this would
		// execute an external script or binary based on the definition
		return map[string]interface{}{
			"message": fmt.Sprintf("External tool %s executed", definition.Name),
			"params":  params,
		}, nil
	}
}

// parametersToMap converts ToolParameters to a map for the executor
func (r *Registry) parametersToMap(params *ToolParameters) map[string]interface{} {
	if params == nil {
		return nil
	}

	result := make(map[string]interface{})
	result["type"] = params.Type
	
	if params.Properties != nil {
		props := make(map[string]interface{})
		for key, def := range params.Properties {
			props[key] = map[string]interface{}{
				"type":        def.Type,
				"description": def.Description,
				"default":     def.Default,
				"enum":        def.Enum,
			}
		}
		result["properties"] = props
	}
	
	if len(params.Required) > 0 {
		result["required"] = params.Required
	}
	
	return result
}

// GetTool returns a tool definition by name
func (r *Registry) GetTool(name string) (*ToolDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, exists := r.tools[name]
	return def, exists
}

// ListTools returns all registered tool definitions
func (r *Registry) ListTools() []*ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]*ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ExecuteTool executes a tool by name with the given parameters
func (r *Registry) ExecuteTool(ctx context.Context, name string, params interface{}) (*ExecutionResult, error) {
	return r.executor.Execute(ctx, name, params)
}

// GetExecutor returns the underlying executor
func (r *Registry) GetExecutor() *Executor {
	return r.executor
}

// ValidateParameters validates tool parameters against the schema
func (r *Registry) ValidateParameters(toolName string, params map[string]interface{}) error {
	r.mu.RLock()
	definition, exists := r.tools[toolName]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("tool %s not found", toolName)
	}

	if definition.Parameters == nil {
		return nil // No parameters to validate
	}

	// Check required parameters
	for _, required := range definition.Parameters.Required {
		if _, exists := params[required]; !exists {
			return fmt.Errorf("missing required parameter: %s", required)
		}
	}

	// Validate parameter types
	for name, value := range params {
		def, exists := definition.Parameters.Properties[name]
		if !exists {
			// Unknown parameter - could be strict or lenient
			continue
		}

		// Basic type validation
		if err := r.validateType(value, def.Type); err != nil {
			return fmt.Errorf("parameter %s: %w", name, err)
		}

		// Enum validation
		if len(def.Enum) > 0 {
			if !r.isInEnum(value, def.Enum) {
				return fmt.Errorf("parameter %s must be one of %v", name, def.Enum)
			}
		}
	}

	return nil
}

// validateType performs basic type validation
func (r *Registry) validateType(value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case "integer", "number":
		switch value.(type) {
		case int, int32, int64, float32, float64:
			// Valid numeric type
		default:
			return fmt.Errorf("expected number, got %T", value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("expected object, got %T", value)
		}
	case "array":
		switch value.(type) {
		case []interface{}, []string, []int:
			// Valid array type
		default:
			return fmt.Errorf("expected array, got %T", value)
		}
	}
	return nil
}

// isInEnum checks if a value is in the enum list
func (r *Registry) isInEnum(value interface{}, enum []string) bool {
	strValue := fmt.Sprintf("%v", value)
	for _, e := range enum {
		if strValue == e {
			return true
		}
	}
	return false
}