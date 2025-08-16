package tools

import (
	"encoding/json"
	"testing"
)

func TestToolValidation(t *testing.T) {
	tests := []struct {
		name    string
		tool    *Tool
		wantErr bool
	}{
		{
			name: "valid tool with parameters",
			tool: &Tool{
				Name:        "test_tool",
				Description: "A test tool",
				Parameters: &ToolSchema{
					Type: "object",
					Properties: map[string]*Property{
						"input": {Type: "string"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid tool without parameters",
			tool: &Tool{
				Name:        "simple_tool",
				Description: "A simple tool",
			},
			wantErr: false,
		},
		{
			name: "invalid tool - missing name",
			tool: &Tool{
				Description: "A tool without name",
			},
			wantErr: true,
		},
		{
			name: "invalid tool - bad schema",
			tool: &Tool{
				Name: "bad_tool",
				Parameters: &ToolSchema{
					Type:     "object",
					Required: []string{"missing_prop"},
				},
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tool.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Tool.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToolSchemaValidation(t *testing.T) {
	tests := []struct {
		name    string
		schema  *ToolSchema
		wantErr bool
	}{
		{
			name: "valid object schema",
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"name": {Type: "string"},
					"age":  {Type: "integer"},
				},
				Required: []string{"name"},
			},
			wantErr: false,
		},
		{
			name: "invalid schema - wrong type",
			schema: &ToolSchema{
				Type: "array",
			},
			wantErr: true,
		},
		{
			name: "invalid schema - required property not defined",
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"name": {Type: "string"},
				},
				Required: []string{"name", "missing"},
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ToolSchema.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPropertyValidation(t *testing.T) {
	tests := []struct {
		name    string
		prop    *Property
		wantErr bool
	}{
		{
			name:    "valid string property",
			prop:    &Property{Type: "string"},
			wantErr: false,
		},
		{
			name:    "valid number property",
			prop:    &Property{Type: "number"},
			wantErr: false,
		},
		{
			name: "valid array property",
			prop: &Property{
				Type:  "array",
				Items: &Property{Type: "string"},
			},
			wantErr: false,
		},
		{
			name:    "invalid property - unknown type",
			prop:    &Property{Type: "unknown"},
			wantErr: true,
		},
		{
			name:    "invalid array - missing items",
			prop:    &Property{Type: "array"},
			wantErr: true,
		},
		{
			name: "invalid string - minLength > maxLength",
			prop: &Property{
				Type:      "string",
				MinLength: intPtr(10),
				MaxLength: intPtr(5),
			},
			wantErr: true,
		},
		{
			name: "invalid number - min > max",
			prop: &Property{
				Type:    "number",
				Minimum: float64Ptr(10),
				Maximum: float64Ptr(5),
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.prop.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Property.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToolCallParseArguments(t *testing.T) {
	tests := []struct {
		name    string
		toolCall *ToolCall
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid JSON arguments",
			toolCall: &ToolCall{
				ID:   "call-1",
				Type: "function",
				Function: FunctionCall{
					Name:      "test",
					Arguments: `{"key": "value", "number": 42}`,
				},
			},
			want: map[string]interface{}{
				"key":    "value",
				"number": float64(42),
			},
			wantErr: false,
		},
		{
			name: "empty arguments",
			toolCall: &ToolCall{
				ID:   "call-2",
				Type: "function",
				Function: FunctionCall{
					Name:      "test",
					Arguments: "",
				},
			},
			want:    map[string]interface{}{},
			wantErr: false,
		},
		{
			name: "invalid JSON",
			toolCall: &ToolCall{
				ID:   "call-3",
				Type: "function",
				Function: FunctionCall{
					Name:      "test",
					Arguments: `{invalid json}`,
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.toolCall.ParseArguments()
			if (err != nil) != tt.wantErr {
				t.Errorf("ToolCall.ParseArguments() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("ToolCall.ParseArguments() = %v, want %v", got, tt.want)
				}
				for k, v := range tt.want {
					if got[k] != v {
						t.Errorf("ToolCall.ParseArguments()[%s] = %v, want %v", k, got[k], v)
					}
				}
			}
		})
	}
}

func TestToolClone(t *testing.T) {
	original := &Tool{
		Name:        "original",
		Description: "Original tool",
		Parameters: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"field": {Type: "string"},
			},
			Required: []string{"field"},
		},
	}
	
	clone := original.Clone()
	
	// Verify clone is equal
	if clone.Name != original.Name {
		t.Errorf("Clone name = %v, want %v", clone.Name, original.Name)
	}
	if clone.Description != original.Description {
		t.Errorf("Clone description = %v, want %v", clone.Description, original.Description)
	}
	
	// Verify deep copy by modifying clone
	clone.Name = "modified"
	clone.Parameters.Properties["new"] = &Property{Type: "number"}
	
	if original.Name != "original" {
		t.Error("Original was modified when clone was changed")
	}
	if _, exists := original.Parameters.Properties["new"]; exists {
		t.Error("Original parameters were modified when clone was changed")
	}
}

func TestToolSchemaSerialization(t *testing.T) {
	schema := &ToolSchema{
		Type: "object",
		Properties: map[string]*Property{
			"text": {
				Type:        "string",
				Description: "Input text",
				MinLength:   intPtr(1),
				MaxLength:   intPtr(1000),
			},
			"count": {
				Type:    "integer",
				Minimum: float64Ptr(0),
				Maximum: float64Ptr(100),
			},
			"tags": {
				Type: "array",
				Items: &Property{
					Type: "string",
				},
				MinItems: intPtr(1),
				MaxItems: intPtr(10),
			},
			"options": {
				Type: "object",
				Properties: map[string]*Property{
					"verbose": {Type: "boolean"},
				},
			},
		},
		Required: []string{"text"},
	}
	
	// Test JSON marshaling
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("Failed to marshal schema: %v", err)
	}
	
	// Test JSON unmarshaling
	var decoded ToolSchema
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal schema: %v", err)
	}
	
	// Verify fields
	if decoded.Type != schema.Type {
		t.Errorf("Decoded type = %v, want %v", decoded.Type, schema.Type)
	}
	if len(decoded.Properties) != len(schema.Properties) {
		t.Errorf("Decoded properties count = %v, want %v", len(decoded.Properties), len(schema.Properties))
	}
	if len(decoded.Required) != len(schema.Required) {
		t.Errorf("Decoded required count = %v, want %v", len(decoded.Required), len(schema.Required))
	}
	
	// Verify a specific property
	if textProp, exists := decoded.Properties["text"]; exists {
		if textProp.Type != "string" {
			t.Errorf("Text property type = %v, want string", textProp.Type)
		}
		if textProp.MinLength == nil || *textProp.MinLength != 1 {
			t.Error("Text property MinLength not preserved")
		}
	} else {
		t.Error("Text property not found in decoded schema")
	}
}

// Helper functions for tests
func intPtr(i int) *int {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}