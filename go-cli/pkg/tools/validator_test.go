package tools

import (
	"testing"
)

func TestValidator(t *testing.T) {
	t.Run("ValidateString", func(t *testing.T) {
		schema := &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"text": {
					Type:      "string",
					MinLength: intPtr(2),
					MaxLength: intPtr(10),
				},
				"email": {
					Type:   "string",
					Format: "email",
				},
				"pattern": {
					Type:    "string",
					Pattern: "^[A-Z]+$",
				},
			},
		}
		
		validator := NewValidator(schema)
		
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name:    "valid strings",
				args:    map[string]interface{}{"text": "hello", "email": "test@example.com", "pattern": "ABC"},
				wantErr: false,
			},
			{
				name:    "text too short",
				args:    map[string]interface{}{"text": "a"},
				wantErr: true,
			},
			{
				name:    "text too long",
				args:    map[string]interface{}{"text": "this is way too long"},
				wantErr: true,
			},
			{
				name:    "invalid email",
				args:    map[string]interface{}{"email": "not-an-email"},
				wantErr: true,
			},
			{
				name:    "pattern mismatch",
				args:    map[string]interface{}{"pattern": "abc"},
				wantErr: true,
			},
			{
				name:    "wrong type",
				args:    map[string]interface{}{"text": 123},
				wantErr: true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator.Validate(tt.args)
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
	
	t.Run("ValidateNumber", func(t *testing.T) {
		schema := &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"age": {
					Type:    "integer",
					Minimum: float64Ptr(0),
					Maximum: float64Ptr(150),
				},
				"score": {
					Type:         "number",
					Minimum:      float64Ptr(0),
					Maximum:      float64Ptr(100),
					MultipleOf:   float64Ptr(0.5),
				},
				"temperature": {
					Type:         "number",
					ExclusiveMin: float64Ptr(-273.15),
					ExclusiveMax: float64Ptr(1000),
				},
			},
		}
		
		validator := NewValidator(schema)
		
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name:    "valid numbers",
				args:    map[string]interface{}{"age": 25, "score": 87.5, "temperature": 20.5},
				wantErr: false,
			},
			{
				name:    "integer as float",
				args:    map[string]interface{}{"age": 25.5},
				wantErr: true,
			},
			{
				name:    "below minimum",
				args:    map[string]interface{}{"age": -1},
				wantErr: true,
			},
			{
				name:    "above maximum",
				args:    map[string]interface{}{"score": 101},
				wantErr: true,
			},
			{
				name:    "not multiple of",
				args:    map[string]interface{}{"score": 87.3},
				wantErr: true,
			},
			{
				name:    "exclusive min violated",
				args:    map[string]interface{}{"temperature": -273.15},
				wantErr: true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator.Validate(tt.args)
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
	
	t.Run("ValidateBoolean", func(t *testing.T) {
		schema := &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"enabled": {Type: "boolean"},
			},
		}
		
		validator := NewValidator(schema)
		
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name:    "valid true",
				args:    map[string]interface{}{"enabled": true},
				wantErr: false,
			},
			{
				name:    "valid false",
				args:    map[string]interface{}{"enabled": false},
				wantErr: false,
			},
			{
				name:    "wrong type",
				args:    map[string]interface{}{"enabled": "true"},
				wantErr: true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator.Validate(tt.args)
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
	
	t.Run("ValidateArray", func(t *testing.T) {
		schema := &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"tags": {
					Type: "array",
					Items: &Property{
						Type: "string",
					},
					MinItems:    intPtr(1),
					MaxItems:    intPtr(5),
					UniqueItems: boolPtr(true),
				},
				"numbers": {
					Type: "array",
					Items: &Property{
						Type:    "number",
						Minimum: float64Ptr(0),
					},
				},
			},
		}
		
		validator := NewValidator(schema)
		
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name:    "valid arrays",
				args:    map[string]interface{}{"tags": []interface{}{"a", "b", "c"}, "numbers": []interface{}{1.0, 2.0, 3.0}},
				wantErr: false,
			},
			{
				name:    "empty array when min required",
				args:    map[string]interface{}{"tags": []interface{}{}},
				wantErr: true,
			},
			{
				name:    "too many items",
				args:    map[string]interface{}{"tags": []interface{}{"a", "b", "c", "d", "e", "f"}},
				wantErr: true,
			},
			{
				name:    "duplicate items when unique required",
				args:    map[string]interface{}{"tags": []interface{}{"a", "b", "a"}},
				wantErr: true,
			},
			{
				name:    "wrong item type",
				args:    map[string]interface{}{"tags": []interface{}{"a", 123, "c"}},
				wantErr: true,
			},
			{
				name:    "item constraint violation",
				args:    map[string]interface{}{"numbers": []interface{}{1.0, -1.0, 3.0}},
				wantErr: true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator.Validate(tt.args)
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
	
	t.Run("ValidateObject", func(t *testing.T) {
		schema := &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"config": {
					Type: "object",
					Properties: map[string]*Property{
						"host": {Type: "string"},
						"port": {Type: "integer"},
					},
					Required:             []string{"host"},
					AdditionalProperties: boolPtr(false),
				},
			},
		}
		
		validator := NewValidator(schema)
		
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name: "valid object",
				args: map[string]interface{}{
					"config": map[string]interface{}{
						"host": "localhost",
						"port": 8080,
					},
				},
				wantErr: false,
			},
			{
				name: "missing required in nested",
				args: map[string]interface{}{
					"config": map[string]interface{}{
						"port": 8080,
					},
				},
				wantErr: true,
			},
			{
				name: "additional property not allowed",
				args: map[string]interface{}{
					"config": map[string]interface{}{
						"host":  "localhost",
						"extra": "not allowed",
					},
				},
				wantErr: true,
			},
			{
				name:    "wrong type",
				args:    map[string]interface{}{"config": "not an object"},
				wantErr: true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator.Validate(tt.args)
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
	
	t.Run("ValidateEnum", func(t *testing.T) {
		schema := &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"status": {
					Type: "string",
					Enum: []interface{}{"pending", "active", "completed"},
				},
				"priority": {
					Type: "integer",
					Enum: []interface{}{1, 2, 3},
				},
			},
		}
		
		validator := NewValidator(schema)
		
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name:    "valid enum values",
				args:    map[string]interface{}{"status": "active", "priority": 2},
				wantErr: false,
			},
			{
				name:    "invalid string enum",
				args:    map[string]interface{}{"status": "invalid"},
				wantErr: true,
			},
			{
				name:    "invalid number enum",
				args:    map[string]interface{}{"priority": 4},
				wantErr: true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator.Validate(tt.args)
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
	
	t.Run("ValidateRequired", func(t *testing.T) {
		schema := &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"required1": {Type: "string"},
				"required2": {Type: "number"},
				"optional":  {Type: "boolean"},
			},
			Required: []string{"required1", "required2"},
		}
		
		validator := NewValidator(schema)
		
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name:    "all required present",
				args:    map[string]interface{}{"required1": "value", "required2": 42},
				wantErr: false,
			},
			{
				name:    "with optional",
				args:    map[string]interface{}{"required1": "value", "required2": 42, "optional": true},
				wantErr: false,
			},
			{
				name:    "missing required1",
				args:    map[string]interface{}{"required2": 42},
				wantErr: true,
			},
			{
				name:    "missing required2",
				args:    map[string]interface{}{"required1": "value"},
				wantErr: true,
			},
			{
				name:    "missing both required",
				args:    map[string]interface{}{"optional": true},
				wantErr: true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator.Validate(tt.args)
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
	
	t.Run("ValidateAdditionalProperties", func(t *testing.T) {
		schemaNoAdditional := &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"known": {Type: "string"},
			},
			AdditionalProperties: boolPtr(false),
		}
		
		schemaAllowAdditional := &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"known": {Type: "string"},
			},
			AdditionalProperties: boolPtr(true),
		}
		
		t.Run("no additional properties allowed", func(t *testing.T) {
			validator := NewValidator(schemaNoAdditional)
			
			// Should pass with only known properties
			err := validator.Validate(map[string]interface{}{"known": "value"})
			if err != nil {
				t.Errorf("Validate() failed: %v", err)
			}
			
			// Should fail with additional properties
			err = validator.Validate(map[string]interface{}{"known": "value", "unknown": "extra"})
			if err == nil {
				t.Error("Validate() did not fail with additional properties")
			}
		})
		
		t.Run("additional properties allowed", func(t *testing.T) {
			validator := NewValidator(schemaAllowAdditional)
			
			// Should pass with additional properties
			err := validator.Validate(map[string]interface{}{"known": "value", "unknown": "extra"})
			if err != nil {
				t.Errorf("Validate() failed: %v", err)
			}
		})
	})
	
	t.Run("ValidateNull", func(t *testing.T) {
		schema := &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"nullable": {Type: "null"},
				"optional": {Type: "string"},
			},
		}
		
		validator := NewValidator(schema)
		
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name:    "valid null",
				args:    map[string]interface{}{"nullable": nil},
				wantErr: false,
			},
			{
				name:    "non-null for null type",
				args:    map[string]interface{}{"nullable": "not null"},
				wantErr: true,
			},
			{
				name:    "null for optional field",
				args:    map[string]interface{}{"optional": nil},
				wantErr: false,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator.Validate(tt.args)
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
	
	t.Run("ValidateFormats", func(t *testing.T) {
		schema := &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"email": {Type: "string", Format: "email"},
				"url":   {Type: "string", Format: "url"},
				"date":  {Type: "string", Format: "date"},
				"time":  {Type: "string", Format: "time"},
				"uuid":  {Type: "string", Format: "uuid"},
			},
		}
		
		validator := NewValidator(schema)
		
		tests := []struct {
			name    string
			args    map[string]interface{}
			wantErr bool
		}{
			{
				name: "valid formats",
				args: map[string]interface{}{
					"email": "test@example.com",
					"url":   "https://example.com",
					"date":  "2024-01-15",
					"time":  "14:30:00",
					"uuid":  "550e8400-e29b-41d4-a716-446655440000",
				},
				wantErr: false,
			},
			{
				name:    "invalid email",
				args:    map[string]interface{}{"email": "not-email"},
				wantErr: true,
			},
			{
				name:    "invalid url",
				args:    map[string]interface{}{"url": "not a url"},
				wantErr: true,
			},
			{
				name:    "invalid date",
				args:    map[string]interface{}{"date": "2024-13-40"},
				wantErr: true,
			},
			{
				name:    "invalid uuid",
				args:    map[string]interface{}{"uuid": "not-a-uuid"},
				wantErr: true,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator.Validate(tt.args)
				if (err != nil) != tt.wantErr {
					t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	})
	
	t.Run("NoSchema", func(t *testing.T) {
		validator := NewValidator(nil)
		
		// Any arguments should be valid with no schema
		err := validator.Validate(map[string]interface{}{
			"anything": "goes",
			"number":   123,
			"nested":   map[string]interface{}{"key": "value"},
		})
		
		if err != nil {
			t.Errorf("Validate() with nil schema failed: %v", err)
		}
	})
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field:   "test_field",
		Message: "validation failed",
		Value:   "bad_value",
	}
	
	errStr := err.Error()
	if errStr != "validation error for field 'test_field': validation failed (value: bad_value)" {
		t.Errorf("Error() = %v", errStr)
	}
	
	// Without value
	err2 := &ValidationError{
		Field:   "other_field",
		Message: "is required",
	}
	
	errStr2 := err2.Error()
	if errStr2 != "validation error for field 'other_field': is required" {
		t.Errorf("Error() without value = %v", errStr2)
	}
}