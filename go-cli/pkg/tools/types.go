package tools

import (
	"encoding/json"
	"fmt"
)

// Tool represents a tool definition received from the server
type Tool struct {
	// Name is the unique identifier for the tool
	Name string `json:"name"`
	
	// Description explains what the tool does
	Description string `json:"description"`
	
	// Parameters defines the JSON Schema for tool parameters
	Parameters *ToolSchema `json:"parameters"`
}

// ToolSchema represents a JSON Schema for tool parameters
// This is a practical subset of JSON Schema draft-07
type ToolSchema struct {
	// Type is typically "object" for tool parameters
	Type string `json:"type"`
	
	// Properties defines the individual parameters
	Properties map[string]*Property `json:"properties,omitempty"`
	
	// Required lists the mandatory parameter names
	Required []string `json:"required,omitempty"`
	
	// AdditionalProperties controls whether extra parameters are allowed
	AdditionalProperties *bool `json:"additionalProperties,omitempty"`
	
	// Description provides schema-level documentation
	Description string `json:"description,omitempty"`
}

// Property represents a single parameter in the tool schema
// Supports a practical subset of JSON Schema features
type Property struct {
	// Type can be: string, number, integer, boolean, array, object, null
	Type string `json:"type"`
	
	// Description explains the property's purpose
	Description string `json:"description,omitempty"`
	
	// Format provides semantic validation (e.g., "email", "uri", "date")
	Format string `json:"format,omitempty"`
	
	// Pattern for regex validation (strings only)
	Pattern string `json:"pattern,omitempty"`
	
	// Enum restricts values to a fixed set
	Enum []interface{} `json:"enum,omitempty"`
	
	// Default value if not provided
	Default interface{} `json:"default,omitempty"`
	
	// String constraints
	MinLength *int `json:"minLength,omitempty"`
	MaxLength *int `json:"maxLength,omitempty"`
	
	// Number/Integer constraints
	Minimum       *float64 `json:"minimum,omitempty"`
	Maximum       *float64 `json:"maximum,omitempty"`
	ExclusiveMin  *float64 `json:"exclusiveMinimum,omitempty"`
	ExclusiveMax  *float64 `json:"exclusiveMaximum,omitempty"`
	MultipleOf    *float64 `json:"multipleOf,omitempty"`
	
	// Array constraints
	Items       *Property `json:"items,omitempty"`
	MinItems    *int      `json:"minItems,omitempty"`
	MaxItems    *int      `json:"maxItems,omitempty"`
	UniqueItems *bool     `json:"uniqueItems,omitempty"`
	
	// Object constraints (for nested objects)
	Properties           map[string]*Property `json:"properties,omitempty"`
	Required             []string             `json:"required,omitempty"`
	AdditionalProperties *bool                `json:"additionalProperties,omitempty"`
}

// ToolCall represents a tool invocation request from an assistant message
type ToolCall struct {
	// ID is the unique identifier for this tool call
	ID string `json:"id"`
	
	// Type is always "function" per OpenAI spec
	Type string `json:"type"`
	
	// Function contains the function call details
	Function FunctionCall `json:"function"`
}

// FunctionCall contains the details of a function to call
type FunctionCall struct {
	// Name is the name of the tool to invoke
	Name string `json:"name"`
	
	// Arguments is a JSON string containing the tool arguments
	Arguments string `json:"arguments"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	// ToolCallID references the originating tool call
	ToolCallID string `json:"toolCallId"`
	
	// Content is the result of the tool execution
	Content string `json:"content"`
	
	// Error contains any error that occurred during execution
	Error string `json:"error,omitempty"`
}

// ParseArguments parses the JSON arguments string into a map
func (tc *ToolCall) ParseArguments() (map[string]interface{}, error) {
	if tc.Function.Arguments == "" {
		return make(map[string]interface{}), nil
	}
	
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
	}
	
	return args, nil
}

// Validate performs basic validation on the tool definition
func (t *Tool) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("tool name is required")
	}
	
	if t.Parameters != nil {
		if err := t.Parameters.Validate(); err != nil {
			return fmt.Errorf("invalid parameters schema: %w", err)
		}
	}
	
	return nil
}

// Validate performs basic validation on the schema
func (ts *ToolSchema) Validate() error {
	if ts.Type != "object" && ts.Type != "" {
		return fmt.Errorf("tool schema type must be 'object' or empty, got %s", ts.Type)
	}
	
	// Validate that required properties are actually defined
	for _, req := range ts.Required {
		if ts.Properties == nil || ts.Properties[req] == nil {
			return fmt.Errorf("required property '%s' is not defined in properties", req)
		}
	}
	
	// Validate each property
	for name, prop := range ts.Properties {
		if err := prop.Validate(); err != nil {
			return fmt.Errorf("property '%s': %w", name, err)
		}
	}
	
	return nil
}

// Validate performs basic validation on the property
func (p *Property) Validate() error {
	// Validate type
	validTypes := map[string]bool{
		"string":  true,
		"number":  true,
		"integer": true,
		"boolean": true,
		"array":   true,
		"object":  true,
		"null":    true,
	}
	
	if p.Type != "" && !validTypes[p.Type] {
		return fmt.Errorf("invalid type: %s", p.Type)
	}
	
	// Type-specific validation
	switch p.Type {
	case "array":
		if p.Items == nil {
			return fmt.Errorf("array type requires 'items' property")
		}
	case "string":
		if p.MinLength != nil && p.MaxLength != nil && *p.MinLength > *p.MaxLength {
			return fmt.Errorf("minLength cannot be greater than maxLength")
		}
	case "number", "integer":
		if p.Minimum != nil && p.Maximum != nil && *p.Minimum > *p.Maximum {
			return fmt.Errorf("minimum cannot be greater than maximum")
		}
	}
	
	return nil
}

// Clone creates a deep copy of the tool
func (t *Tool) Clone() *Tool {
	if t == nil {
		return nil
	}
	
	clone := &Tool{
		Name:        t.Name,
		Description: t.Description,
	}
	
	if t.Parameters != nil {
		clone.Parameters = t.Parameters.Clone()
	}
	
	return clone
}

// Clone creates a deep copy of the schema
func (ts *ToolSchema) Clone() *ToolSchema {
	if ts == nil {
		return nil
	}
	
	clone := &ToolSchema{
		Type:        ts.Type,
		Description: ts.Description,
	}
	
	if ts.Properties != nil {
		clone.Properties = make(map[string]*Property)
		for k, v := range ts.Properties {
			clone.Properties[k] = v.Clone()
		}
	}
	
	if ts.Required != nil {
		clone.Required = make([]string, len(ts.Required))
		copy(clone.Required, ts.Required)
	}
	
	if ts.AdditionalProperties != nil {
		b := *ts.AdditionalProperties
		clone.AdditionalProperties = &b
	}
	
	return clone
}

// Clone creates a deep copy of the property
func (p *Property) Clone() *Property {
	if p == nil {
		return nil
	}
	
	clone := &Property{
		Type:        p.Type,
		Description: p.Description,
		Format:      p.Format,
		Pattern:     p.Pattern,
		Default:     p.Default,
	}
	
	// Copy enum values
	if p.Enum != nil {
		clone.Enum = make([]interface{}, len(p.Enum))
		copy(clone.Enum, p.Enum)
	}
	
	// Copy pointer fields
	if p.MinLength != nil {
		v := *p.MinLength
		clone.MinLength = &v
	}
	if p.MaxLength != nil {
		v := *p.MaxLength
		clone.MaxLength = &v
	}
	if p.Minimum != nil {
		v := *p.Minimum
		clone.Minimum = &v
	}
	if p.Maximum != nil {
		v := *p.Maximum
		clone.Maximum = &v
	}
	if p.ExclusiveMin != nil {
		v := *p.ExclusiveMin
		clone.ExclusiveMin = &v
	}
	if p.ExclusiveMax != nil {
		v := *p.ExclusiveMax
		clone.ExclusiveMax = &v
	}
	if p.MultipleOf != nil {
		v := *p.MultipleOf
		clone.MultipleOf = &v
	}
	if p.MinItems != nil {
		v := *p.MinItems
		clone.MinItems = &v
	}
	if p.MaxItems != nil {
		v := *p.MaxItems
		clone.MaxItems = &v
	}
	if p.UniqueItems != nil {
		v := *p.UniqueItems
		clone.UniqueItems = &v
	}
	if p.AdditionalProperties != nil {
		v := *p.AdditionalProperties
		clone.AdditionalProperties = &v
	}
	
	// Deep copy nested structures
	if p.Items != nil {
		clone.Items = p.Items.Clone()
	}
	
	if p.Properties != nil {
		clone.Properties = make(map[string]*Property)
		for k, v := range p.Properties {
			clone.Properties[k] = v.Clone()
		}
	}
	
	if p.Required != nil {
		clone.Required = make([]string, len(p.Required))
		copy(clone.Required, p.Required)
	}
	
	return clone
}