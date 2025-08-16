package tools

import (
	"encoding/json"
	"testing"
)

func TestValidateArguments(t *testing.T) {
	tests := []struct {
		name      string
		args      string
		schema    *ToolSchema
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid object with required fields",
			args: `{"url": "https://example.com", "method": "GET"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"url": {
						Type:   "string",
						Format: "uri",
					},
					"method": {
						Type: "string",
						Enum: []interface{}{"GET", "POST", "PUT", "DELETE"},
					},
				},
				Required: []string{"url", "method"},
			},
			wantError: false,
		},
		{
			name: "missing required field",
			args: `{"method": "GET"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"url":    {Type: "string"},
					"method": {Type: "string"},
				},
				Required: []string{"url", "method"},
			},
			wantError: true,
			errorMsg:  "url",
		},
		{
			name: "invalid type",
			args: `{"count": "not a number"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"count": {Type: "integer"},
				},
			},
			wantError: true,
			errorMsg:  "expected type 'integer'",
		},
		{
			name: "string too short",
			args: `{"name": "ab"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"name": {
						Type:      "string",
						MinLength: intPtr(3),
					},
				},
			},
			wantError: true,
			errorMsg:  "at least 3 characters",
		},
		{
			name: "string too long",
			args: `{"name": "verylongname"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"name": {
						Type:      "string",
						MaxLength: intPtr(5),
					},
				},
			},
			wantError: true,
			errorMsg:  "at most 5 characters",
		},
		{
			name: "number out of range",
			args: `{"age": 150}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"age": {
						Type:    "integer",
						Minimum: float64Ptr(0),
						Maximum: float64Ptr(120),
					},
				},
			},
			wantError: true,
			errorMsg:  "at most 120",
		},
		{
			name: "invalid enum value",
			args: `{"status": "unknown"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"status": {
						Type: "string",
						Enum: []interface{}{"active", "inactive", "pending"},
					},
				},
			},
			wantError: true,
			errorMsg:  "must be one of",
		},
		{
			name: "array with invalid items",
			args: `{"tags": ["valid", 123, "another"]}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"tags": {
						Type: "array",
						Items: &Property{
							Type: "string",
						},
					},
				},
			},
			wantError: true,
			errorMsg:  "expected type 'string'",
		},
		{
			name: "pattern mismatch",
			args: `{"code": "ABC123"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"code": {
						Type:    "string",
						Pattern: "^[A-Z]{3}-[0-9]{3}$",
					},
				},
			},
			wantError: true,
			errorMsg:  "does not match pattern",
		},
		{
			name: "valid pattern match",
			args: `{"code": "ABC-123"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"code": {
						Type:    "string",
						Pattern: "^[A-Z]{3}-[0-9]{3}$",
					},
				},
			},
			wantError: false,
		},
		{
			name: "nested object validation",
			args: `{"user": {"name": "John", "age": 30}}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"user": {
						Type: "object",
						Properties: map[string]*Property{
							"name": {Type: "string"},
							"age":  {Type: "integer"},
						},
						Required: []string{"name"},
					},
				},
			},
			wantError: false,
		},
		{
			name: "nested object validation failure",
			args: `{"user": {"age": "not a number"}}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"user": {
						Type: "object",
						Properties: map[string]*Property{
							"name": {Type: "string"},
							"age":  {Type: "integer"},
						},
						Required: []string{"name"},
					},
				},
			},
			wantError: true,
			errorMsg:  "required property is missing",
		},
		{
			name: "additional properties not allowed",
			args: `{"allowed": "yes", "notAllowed": "no"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"allowed": {Type: "string"},
				},
				AdditionalProperties: boolPtr(false),
			},
			wantError: true,
			errorMsg:  "additional property not allowed",
		},
		{
			name: "no schema means no validation",
			args: `{"anything": "goes"}`,
			schema: nil,
			wantError: false,
		},
		{
			name: "invalid JSON",
			args: `{invalid json}`,
			schema: &ToolSchema{Type: "object"},
			wantError: true,
			errorMsg:  "invalid JSON",
		},
		{
			name: "format validation - valid email",
			args: `{"email": "user@example.com"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"email": {
						Type:   "string",
						Format: "email",
					},
				},
			},
			wantError: false,
		},
		{
			name: "format validation - invalid email",
			args: `{"email": "not-an-email"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"email": {
						Type:   "string",
						Format: "email",
					},
				},
			},
			wantError: true,
			errorMsg:  "invalid email format",
		},
		{
			name: "format validation - valid URI",
			args: `{"website": "https://example.com/path"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"website": {
						Type:   "string",
						Format: "uri",
					},
				},
			},
			wantError: false,
		},
		{
			name: "format validation - invalid URI",
			args: `{"website": "not a uri"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"website": {
						Type:   "string",
						Format: "uri",
					},
				},
			},
			wantError: true,
			errorMsg:  "invalid URI format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArguments(json.RawMessage(tt.args), tt.schema)
			
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateArguments() expected error but got none")
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateArguments() error = %v, want error containing %v", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateArguments() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidationErrors(t *testing.T) {
	errors := &SchemaValidationErrors{}
	
	// Test empty errors
	if errors.HasErrors() {
		t.Error("HasErrors() should return false for empty errors")
	}
	
	// Add single error
	errors.Add("field1", "error message 1", "value1", "schema1")
	if !errors.HasErrors() {
		t.Error("HasErrors() should return true after adding error")
	}
	
	// Check single error message
	errMsg := errors.Error()
	if !contains(errMsg, "field1") || !contains(errMsg, "error message 1") {
		t.Errorf("Error message doesn't contain expected text: %s", errMsg)
	}
	
	// Add multiple errors
	errors.Add("field2", "error message 2", "value2", "schema2")
	errors.Add("field3", "error message 3", "value3", "schema3")
	
	// Check multiple errors message
	errMsg = errors.Error()
	if !contains(errMsg, "multiple validation errors") {
		t.Errorf("Error message should mention multiple errors: %s", errMsg)
	}
	if !contains(errMsg, "field2") || !contains(errMsg, "field3") {
		t.Errorf("Error message should contain all field paths: %s", errMsg)
	}
}

func TestGetJSONType(t *testing.T) {
	tests := []struct {
		value    interface{}
		expected string
	}{
		{nil, "null"},
		{true, "boolean"},
		{false, "boolean"},
		{42, "number"},
		{3.14, "number"},
		{json.Number("42"), "number"},
		{"hello", "string"},
		{[]interface{}{}, "array"},
		{map[string]interface{}{}, "object"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := getJSONType(tt.value)
			if result != tt.expected {
				t.Errorf("getJSONType(%v) = %s, want %s", tt.value, result, tt.expected)
			}
		})
	}
}

func TestJoinSchemaPath(t *testing.T) {
	tests := []struct {
		base     string
		segment  string
		expected string
	}{
		{"", "field", "field"},
		{"root", "field", "root.field"},
		{"root.nested", "field", "root.nested.field"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := joinSchemaPath(tt.base, tt.segment)
			if result != tt.expected {
				t.Errorf("joinSchemaPath(%s, %s) = %s, want %s", tt.base, tt.segment, result, tt.expected)
			}
		})
	}
}

func TestFormatValidation(t *testing.T) {
	tests := []struct {
		name      string
		args      string
		schema    *ToolSchema
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid date format",
			args: `{"date": "2023-12-25"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"date": {
						Type:   "string",
						Format: "date",
					},
				},
			},
			wantError: false,
		},
		{
			name: "invalid date format",
			args: `{"date": "12/25/2023"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"date": {
						Type:   "string",
						Format: "date",
					},
				},
			},
			wantError: true,
			errorMsg:  "invalid date format",
		},
		{
			name: "valid UUID format",
			args: `{"id": "550e8400-e29b-41d4-a716-446655440000"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"id": {
						Type:   "string",
						Format: "uuid",
					},
				},
			},
			wantError: false,
		},
		{
			name: "invalid UUID format",
			args: `{"id": "not-a-uuid"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"id": {
						Type:   "string",
						Format: "uuid",
					},
				},
			},
			wantError: true,
			errorMsg:  "invalid UUID format",
		},
		{
			name: "valid IPv4 address",
			args: `{"ip": "192.168.1.1"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"ip": {
						Type:   "string",
						Format: "ipv4",
					},
				},
			},
			wantError: false,
		},
		{
			name: "invalid IPv4 address - out of range",
			args: `{"ip": "256.256.256.256"}`,
			schema: &ToolSchema{
				Type: "object",
				Properties: map[string]*Property{
					"ip": {
						Type:   "string",
						Format: "ipv4",
					},
				},
			},
			wantError: true,
			errorMsg:  "invalid IPv4 address",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArguments(json.RawMessage(tt.args), tt.schema)
			
			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateArguments() expected error but got none")
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("ValidateArguments() error = %v, want error containing %v", err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateArguments() unexpected error = %v", err)
				}
			}
		})
	}
}

// Helper functions for tests
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && containsSubstring(s, substr)
}

func containsSubstring(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}