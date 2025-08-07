package tools_test

import (
	"fmt"
	"testing"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
	"github.com/stretchr/testify/assert"
)

// Type-safe parameter structures for schema testing

// BasicUserParams represents basic user parameters for schema testing
type BasicUserParams struct {
	Name string `json:"name"`
}

// UserWithEmailParams represents user parameters with email
type UserWithEmailParams struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age,omitempty"`
}

// AddressParams represents address parameters
type AddressParams struct {
	Street string `json:"street,omitempty"`
	City   string `json:"city"`
	Zip    string `json:"zip,omitempty"`
}

// UserWithAddressParams represents user with address
type UserWithAddressParams struct {
	Address AddressParams `json:"address"`
}

// ComplexUserParams represents complex user structure
type ComplexUserParams struct {
	Name        string          `json:"name"`
	Email       string          `json:"email"`
	Age         int             `json:"age,omitempty"`
	Roles       []string        `json:"roles,omitempty"`
	Preferences UserPreferences `json:"preferences,omitempty"`
}

// UserPreferences represents user preferences
type UserPreferences struct {
	Theme         string `json:"theme,omitempty"`
	Notifications bool   `json:"notifications,omitempty"`
}

// ArrayTestParams represents array test parameters
type ArrayTestParams struct {
	Tags    []string            `json:"tags,omitempty"`
	Items   []string            `json:"items,omitempty"`
	Numbers []int               `json:"numbers,omitempty"`
	Users   []ComplexUserParams `json:"users,omitempty"`
}

// ValidationTestData represents typed test data with validation info
type ValidationTestData struct {
	ID    int    `json:"id"`
	Value string `json:"value"`
}

// ComplexValidationParams represents complex validation test parameters
type ComplexValidationParams struct {
	Data []ValidationTestData `json:"data"`
}

// Helper functions to convert typed structures to map[string]interface{}

// basicUserParamsToMap converts BasicUserParams to map
func basicUserParamsToMap(params BasicUserParams) map[string]interface{} {
	return map[string]interface{}{
		"name": params.Name,
	}
}

// userWithEmailParamsToMap converts UserWithEmailParams to map
func userWithEmailParamsToMap(params UserWithEmailParams) map[string]interface{} {
	result := map[string]interface{}{
		"name":  params.Name,
		"email": params.Email,
	}
	if params.Age > 0 {
		result["age"] = params.Age
	}
	return result
}

// complexUserParamsToMap converts ComplexUserParams to map
func complexUserParamsToMap(params ComplexUserParams) map[string]interface{} {
	result := map[string]interface{}{
		"name":  params.Name,
		"email": params.Email,
	}
	if params.Age > 0 {
		result["age"] = params.Age
	}
	if len(params.Roles) > 0 {
		roles := make([]interface{}, len(params.Roles))
		for i, role := range params.Roles {
			roles[i] = role
		}
		result["roles"] = roles
	}
	if params.Preferences.Theme != "" || params.Preferences.Notifications {
		prefs := make(map[string]interface{})
		if params.Preferences.Theme != "" {
			prefs["theme"] = params.Preferences.Theme
		}
		prefs["notifications"] = params.Preferences.Notifications
		result["preferences"] = prefs
	}
	return result
}

// arrayTestParamsToMap converts ArrayTestParams to map
func arrayTestParamsToMap(params ArrayTestParams) map[string]interface{} {
	result := make(map[string]interface{})
	if len(params.Users) > 0 {
		users := make([]interface{}, len(params.Users))
		for i, user := range params.Users {
			users[i] = complexUserParamsToMap(user)
		}
		result["users"] = users
	}
	return result
}

// validationTestDataToMap converts ValidationTestData to map
func validationTestDataToMap(data ValidationTestData) map[string]interface{} {
	return map[string]interface{}{
		"id":    data.ID,
		"value": data.Value,
	}
}

// complexValidationParamsToMap converts ComplexValidationParams to map
func complexValidationParamsToMap(params ComplexValidationParams) map[string]interface{} {
	data := make([]interface{}, len(params.Data))
	for i, item := range params.Data {
		data[i] = validationTestDataToMap(item)
	}
	return map[string]interface{}{
		"data": data,
	}
}

func TestNewSchemaValidator(t *testing.T) {
	schema := &tools.ToolSchema{
		Type: "object",
		Properties: map[string]*tools.Property{
			"name": {Type: "string"},
		},
	}

	validator := tools.NewSchemaValidator(schema)
	assert.NotNil(t, validator)
	// Note: Cannot test internal fields when using external test package
}

func TestSchemaValidator_ValidateString(t *testing.T) {
	tests := []struct {
		name    string
		schema  *tools.ToolSchema
		params  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid string",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"name": {Type: "string"},
				},
			},
			params:  basicUserParamsToMap(BasicUserParams{Name: "John"}),
			wantErr: false,
		},
		{
			name: "invalid type for string",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"name": {Type: "string"},
				},
			},
			params:  map[string]interface{}{"name": 123}, // Keep as raw map for error case
			wantErr: true,
			errMsg:  "name: expected string, got int",
		},
		{
			name: "string with minLength valid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"name": {Type: "string", MinLength: intPtr2(3)},
				},
			},
			params:  map[string]interface{}{"name": "John"},
			wantErr: false,
		},
		{
			name: "string with minLength invalid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"name": {Type: "string", MinLength: intPtr2(5)},
				},
			},
			params:  map[string]interface{}{"name": "John"},
			wantErr: true,
			errMsg:  "name: string length 4 is less than minimum 5",
		},
		{
			name: "string with maxLength valid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"name": {Type: "string", MaxLength: intPtr2(10)},
				},
			},
			params:  map[string]interface{}{"name": "John"},
			wantErr: false,
		},
		{
			name: "string with maxLength invalid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"name": {Type: "string", MaxLength: intPtr2(3)},
				},
			},
			params:  map[string]interface{}{"name": "John"},
			wantErr: true,
			errMsg:  "name: string length 4 is greater than maximum 3",
		},
		{
			name: "string with pattern valid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"code": {Type: "string", Pattern: "^[A-Z]{3}$"},
				},
			},
			params:  map[string]interface{}{"code": "ABC"},
			wantErr: false,
		},
		{
			name: "string with pattern invalid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"code": {Type: "string", Pattern: "^[A-Z]{3}$"},
				},
			},
			params:  map[string]interface{}{"code": "abc"},
			wantErr: true,
			errMsg:  "code: string \"abc\" does not match pattern \"^[A-Z]{3}$\"",
		},
		{
			name: "string with enum valid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"status": {Type: "string", Enum: []interface{}{"active", "inactive", "pending"}},
				},
			},
			params:  map[string]interface{}{"status": "active"},
			wantErr: false,
		},
		{
			name: "string with enum invalid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"status": {Type: "string", Enum: []interface{}{"active", "inactive", "pending"}},
				},
			},
			params:  map[string]interface{}{"status": "deleted"},
			wantErr: true,
			errMsg:  "status: value \"deleted\" is not in enum [active inactive pending]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_ValidateStringFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
		value  string
		valid  bool
		errMsg string
	}{
		// Email format
		{
			name:   "valid email",
			format: "email",
			value:  "user@example.com",
			valid:  true,
		},
		{
			name:   "invalid email - no @",
			format: "email",
			value:  "userexample.com",
			valid:  false,
			errMsg: "email: \"userexample.com\" is not a valid email address",
		},
		{
			name:   "invalid email - no domain",
			format: "email",
			value:  "user@",
			valid:  false,
			errMsg: "email: \"user@\" is not a valid email address",
		},
		{
			name:   "invalid email - no dot in domain",
			format: "email",
			value:  "user@example",
			valid:  false,
			errMsg: "email: \"user@example\" is not a valid email address",
		},
		// URL format
		{
			name:   "valid http URL",
			format: "url",
			value:  "http://example.com",
			valid:  true,
		},
		{
			name:   "valid https URL",
			format: "url",
			value:  "https://example.com",
			valid:  true,
		},
		{
			name:   "invalid URL - no protocol",
			format: "url",
			value:  "example.com",
			valid:  false,
			errMsg: "url: \"example.com\" is not a valid URL",
		},
		// Date-time format
		{
			name:   "valid date-time",
			format: "date-time",
			value:  "2023-12-25T10:30:00Z",
			valid:  true,
		},
		{
			name:   "invalid date-time",
			format: "date-time",
			value:  "2023-12-25",
			valid:  false,
			errMsg: "date-time: \"2023-12-25\" is not a valid date-time",
		},
		// Date format
		{
			name:   "valid date",
			format: "date",
			value:  "2023-12-25",
			valid:  true,
		},
		{
			name:   "invalid date",
			format: "date",
			value:  "12-25-2023",
			valid:  false,
			errMsg: "date: \"12-25-2023\" is not a valid date",
		},
		// Time format
		{
			name:   "valid time",
			format: "time",
			value:  "10:30:00",
			valid:  true,
		},
		{
			name:   "invalid time",
			format: "time",
			value:  "10:30",
			valid:  false,
			errMsg: "time: \"10:30\" is not a valid time",
		},
		// UUID format
		{
			name:   "valid UUID",
			format: "uuid",
			value:  "550e8400-e29b-41d4-a716-446655440000",
			valid:  true,
		},
		{
			name:   "valid UUID uppercase",
			format: "uuid",
			value:  "550E8400-E29B-41D4-A716-446655440000",
			valid:  true,
		},
		{
			name:   "invalid UUID - wrong format",
			format: "uuid",
			value:  "550e8400-e29b-41d4-a716",
			valid:  false,
			errMsg: "uuid: \"550e8400-e29b-41d4-a716\" is not a valid UUID",
		},
		{
			name:   "invalid UUID - wrong characters",
			format: "uuid",
			value:  "550e8400-e29b-41d4-a716-44665544000g",
			valid:  false,
			errMsg: "uuid: \"550e8400-e29b-41d4-a716-44665544000g\" is not a valid UUID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					tt.format: {Type: "string", Format: tt.format},
				},
			}
			validator := tools.NewSchemaValidator(schema)
			err := validator.Validate(map[string]interface{}{tt.format: tt.value})
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestSchemaValidator_ValidateNumber(t *testing.T) {
	tests := []struct {
		name    string
		schema  *tools.ToolSchema
		params  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid float64 number",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"price": {Type: "number"},
				},
			},
			params:  map[string]interface{}{"price": 99.99},
			wantErr: false,
		},
		{
			name: "valid int as number",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"price": {Type: "number"},
				},
			},
			params:  map[string]interface{}{"price": 100},
			wantErr: false,
		},
		{
			name: "invalid type for number",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"price": {Type: "number"},
				},
			},
			params:  map[string]interface{}{"price": "100"},
			wantErr: true,
			errMsg:  "price: expected number, got string",
		},
		{
			name: "number with minimum valid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"age": {Type: "number", Minimum: float64Ptr2(18)},
				},
			},
			params:  map[string]interface{}{"age": 25},
			wantErr: false,
		},
		{
			name: "number with minimum invalid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"age": {Type: "number", Minimum: float64Ptr2(18)},
				},
			},
			params:  map[string]interface{}{"age": 17},
			wantErr: true,
			errMsg:  "age: value 17 is less than minimum 18",
		},
		{
			name: "number with maximum valid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"discount": {Type: "number", Maximum: float64Ptr2(100)},
				},
			},
			params:  map[string]interface{}{"discount": 50},
			wantErr: false,
		},
		{
			name: "number with maximum invalid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"discount": {Type: "number", Maximum: float64Ptr2(100)},
				},
			},
			params:  map[string]interface{}{"discount": 150},
			wantErr: true,
			errMsg:  "discount: value 150 is greater than maximum 100",
		},
		{
			name: "number with enum valid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"rating": {Type: "number", Enum: []interface{}{1.0, 1.5, 2.0, 2.5, 3.0}},
				},
			},
			params:  map[string]interface{}{"rating": 2.5},
			wantErr: false,
		},
		{
			name: "number with enum invalid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"rating": {Type: "number", Enum: []interface{}{1.0, 1.5, 2.0, 2.5, 3.0}},
				},
			},
			params:  map[string]interface{}{"rating": 3.5},
			wantErr: true,
			errMsg:  "rating: value 3.5 is not in enum [1 1.5 2 2.5 3]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_ValidateInteger(t *testing.T) {
	tests := []struct {
		name    string
		schema  *tools.ToolSchema
		params  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid integer",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"count": {Type: "integer"},
				},
			},
			params:  map[string]interface{}{"count": 42},
			wantErr: false,
		},
		{
			name: "float as integer - whole number",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"count": {Type: "integer"},
				},
			},
			params:  map[string]interface{}{"count": 42.0},
			wantErr: false,
		},
		{
			name: "float as integer - decimal",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"count": {Type: "integer"},
				},
			},
			params:  map[string]interface{}{"count": 42.5},
			wantErr: true,
			errMsg:  "count: expected integer, got float 42.5",
		},
		{
			name: "string as integer",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"count": {Type: "integer"},
				},
			},
			params:  map[string]interface{}{"count": "42"},
			wantErr: true,
			errMsg:  "count: expected integer, got string",
		},
		{
			name: "integer with minimum valid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"quantity": {Type: "integer", Minimum: float64Ptr2(1)},
				},
			},
			params:  map[string]interface{}{"quantity": 5},
			wantErr: false,
		},
		{
			name: "integer with minimum invalid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"quantity": {Type: "integer", Minimum: float64Ptr2(1)},
				},
			},
			params:  map[string]interface{}{"quantity": 0},
			wantErr: true,
			errMsg:  "quantity: value 0 is less than minimum 1",
		},
		{
			name: "integer with enum valid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"level": {Type: "integer", Enum: []interface{}{1, 2, 3, 4, 5}},
				},
			},
			params:  map[string]interface{}{"level": 3},
			wantErr: false,
		},
		{
			name: "integer with enum invalid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"level": {Type: "integer", Enum: []interface{}{1, 2, 3, 4, 5}},
				},
			},
			params:  map[string]interface{}{"level": 6},
			wantErr: true,
			errMsg:  "level: value 6 is not in enum [1 2 3 4 5]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_ValidateBoolean(t *testing.T) {
	tests := []struct {
		name    string
		schema  *tools.ToolSchema
		params  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid boolean true",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"active": {Type: "boolean"},
				},
			},
			params:  map[string]interface{}{"active": true},
			wantErr: false,
		},
		{
			name: "valid boolean false",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"active": {Type: "boolean"},
				},
			},
			params:  map[string]interface{}{"active": false},
			wantErr: false,
		},
		{
			name: "invalid type for boolean",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"active": {Type: "boolean"},
				},
			},
			params:  map[string]interface{}{"active": "true"},
			wantErr: true,
			errMsg:  "active: expected boolean, got string",
		},
		{
			name: "invalid type for boolean - number",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"active": {Type: "boolean"},
				},
			},
			params:  map[string]interface{}{"active": 1},
			wantErr: true,
			errMsg:  "active: expected boolean, got int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_ValidateArray(t *testing.T) {
	tests := []struct {
		name    string
		schema  *tools.ToolSchema
		params  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid array of strings",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"tags": {
						Type:  "array",
						Items: &tools.Property{Type: "string"},
					},
				},
			},
			params:  map[string]interface{}{"tags": []interface{}{"tag1", "tag2", "tag3"}},
			wantErr: false,
		},
		{
			name: "valid empty array",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"tags": {
						Type:  "array",
						Items: &tools.Property{Type: "string"},
					},
				},
			},
			params:  map[string]interface{}{"tags": []interface{}{}},
			wantErr: false,
		},
		{
			name: "invalid type for array",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"tags": {
						Type:  "array",
						Items: &tools.Property{Type: "string"},
					},
				},
			},
			params:  map[string]interface{}{"tags": "not-an-array"},
			wantErr: true,
			errMsg:  "tags: expected array, got string",
		},
		{
			name: "array with minLength valid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"items": {
						Type:      "array",
						MinLength: intPtr2(2),
						Items:     &tools.Property{Type: "string"},
					},
				},
			},
			params:  map[string]interface{}{"items": []interface{}{"a", "b", "c"}},
			wantErr: false,
		},
		{
			name: "array with minLength invalid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"items": {
						Type:      "array",
						MinLength: intPtr2(2),
						Items:     &tools.Property{Type: "string"},
					},
				},
			},
			params:  map[string]interface{}{"items": []interface{}{"a"}},
			wantErr: true,
			errMsg:  "items: array length 1 is less than minimum 2",
		},
		{
			name: "array with maxLength valid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"items": {
						Type:      "array",
						MaxLength: intPtr2(3),
						Items:     &tools.Property{Type: "string"},
					},
				},
			},
			params:  map[string]interface{}{"items": []interface{}{"a", "b"}},
			wantErr: false,
		},
		{
			name: "array with maxLength invalid",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"items": {
						Type:      "array",
						MaxLength: intPtr2(2),
						Items:     &tools.Property{Type: "string"},
					},
				},
			},
			params:  map[string]interface{}{"items": []interface{}{"a", "b", "c"}},
			wantErr: true,
			errMsg:  "items: array length 3 is greater than maximum 2",
		},
		{
			name: "array with invalid item type",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"numbers": {
						Type:  "array",
						Items: &tools.Property{Type: "number"},
					},
				},
			},
			params:  map[string]interface{}{"numbers": []interface{}{1, 2, "three"}},
			wantErr: true,
			errMsg:  "numbers[2]: expected number, got string",
		},
		{
			name: "array of objects",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"users": {
						Type: "array",
						Items: &tools.Property{
							Type: "object",
							Properties: map[string]*tools.Property{
								"name": {Type: "string"},
								"age":  {Type: "integer"},
							},
							Required: []string{"name"},
						},
					},
				},
			},
			params: map[string]interface{}{
				"users": []interface{}{
					map[string]interface{}{"name": "Alice", "age": 30},
					map[string]interface{}{"name": "Bob", "age": 25},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_ValidateObject(t *testing.T) {
	tests := []struct {
		name    string
		schema  *tools.ToolSchema
		params  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid nested object",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"address": {
						Type: "object",
						Properties: map[string]*tools.Property{
							"street": {Type: "string"},
							"city":   {Type: "string"},
							"zip":    {Type: "string"},
						},
						Required: []string{"city"},
					},
				},
			},
			params: map[string]interface{}{
				"address": map[string]interface{}{
					"street": "123 Main St",
					"city":   "New York",
					"zip":    "10001",
				},
			},
			wantErr: false,
		},
		{
			name: "nested object missing required field",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"address": {
						Type: "object",
						Properties: map[string]*tools.Property{
							"street": {Type: "string"},
							"city":   {Type: "string"},
						},
						Required: []string{"city"},
					},
				},
			},
			params: map[string]interface{}{
				"address": map[string]interface{}{
					"street": "123 Main St",
				},
			},
			wantErr: true,
			errMsg:  "address.city: required property is missing",
		},
		{
			name: "invalid type for object",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"config": {Type: "object"},
				},
			},
			params:  map[string]interface{}{"config": "not-an-object"},
			wantErr: true,
			errMsg:  "config: expected object, got string",
		},
		{
			name: "deeply nested objects",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"level1": {
						Type: "object",
						Properties: map[string]*tools.Property{
							"level2": {
								Type: "object",
								Properties: map[string]*tools.Property{
									"level3": {
										Type: "object",
										Properties: map[string]*tools.Property{
											"value": {Type: "string"},
										},
									},
								},
							},
						},
					},
				},
			},
			params: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"level3": map[string]interface{}{
							"value": "deep",
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_ValidateNull(t *testing.T) {
	tests := []struct {
		name    string
		schema  *tools.ToolSchema
		params  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid null value",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"optional": {Type: "null"},
				},
			},
			params:  map[string]interface{}{"optional": nil},
			wantErr: false,
		},
		{
			name: "invalid non-null for null type",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"mustBeNull": {Type: "null"},
				},
			},
			params:  map[string]interface{}{"mustBeNull": "not null"},
			wantErr: true,
			errMsg:  "mustBeNull: value must be null",
		},
		{
			name: "null value for non-null type",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"required": {Type: "string"},
				},
			},
			params:  map[string]interface{}{"required": nil},
			wantErr: true,
			errMsg:  "required: value cannot be null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_RequiredProperties(t *testing.T) {
	tests := []struct {
		name    string
		schema  *tools.ToolSchema
		params  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "all required properties present",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"name":  {Type: "string"},
					"email": {Type: "string"},
					"age":   {Type: "integer"},
				},
				Required: []string{"name", "email"},
			},
			params: userWithEmailParamsToMap(UserWithEmailParams{
				Name:  "John",
				Email: "john@example.com",
				Age:   30,
			}),
			wantErr: false,
		},
		{
			name: "missing required property",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"name":  {Type: "string"},
					"email": {Type: "string"},
				},
				Required: []string{"name", "email"},
			},
			params: map[string]interface{}{
				"name": "John",
			},
			wantErr: true,
			errMsg:  "email: required property is missing",
		},
		{
			name: "optional property can be omitted",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"name":     {Type: "string"},
					"nickname": {Type: "string"},
				},
				Required: []string{"name"},
			},
			params: map[string]interface{}{
				"name": "John",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_AdditionalProperties(t *testing.T) {
	tests := []struct {
		name    string
		schema  *tools.ToolSchema
		params  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "additional properties allowed by default",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"name": {Type: "string"},
				},
			},
			params: map[string]interface{}{
				"name":  "John",
				"extra": "allowed",
			},
			wantErr: false,
		},
		{
			name: "additional properties explicitly allowed",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"name": {Type: "string"},
				},
				AdditionalProperties: boolPtr2(true),
			},
			params: map[string]interface{}{
				"name":  "John",
				"extra": "allowed",
			},
			wantErr: false,
		},
		{
			name: "additional properties not allowed",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"name": {Type: "string"},
				},
				AdditionalProperties: boolPtr2(false),
			},
			params: map[string]interface{}{
				"name":  "John",
				"extra": "not allowed",
			},
			wantErr: true,
			errMsg:  ": additional property \"extra\" is not allowed",
		},
		{
			name: "multiple additional properties not allowed",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"name": {Type: "string"},
				},
				AdditionalProperties: boolPtr2(false),
			},
			params: map[string]interface{}{
				"name":   "John",
				"extra1": "not allowed",
				"extra2": "also not allowed",
			},
			wantErr: true,
			// Note: The error will report the first additional property found
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					// For additional properties, we just check that it's an error
					// The exact property reported may vary due to map iteration order
					assert.Contains(t, err.Error(), "additional property")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_ComplexScenarios(t *testing.T) {
	tests := []struct {
		name    string
		schema  *tools.ToolSchema
		params  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "complex nested structure with arrays and objects",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"users": {
						Type: "array",
						Items: &tools.Property{
							Type: "object",
							Properties: map[string]*tools.Property{
								"name": {
									Type:      "string",
									MinLength: intPtr2(2),
									MaxLength: intPtr2(50),
								},
								"email": {
									Type:   "string",
									Format: "email",
								},
								"age": {
									Type:    "integer",
									Minimum: float64Ptr2(0),
									Maximum: float64Ptr2(120),
								},
								"roles": {
									Type: "array",
									Items: &tools.Property{
										Type: "string",
										Enum: []interface{}{"admin", "user", "guest"},
									},
								},
								"preferences": {
									Type: "object",
									Properties: map[string]*tools.Property{
										"theme": {
											Type: "string",
											Enum: []interface{}{"light", "dark"},
										},
										"notifications": {
											Type: "boolean",
										},
									},
								},
							},
							Required: []string{"name", "email"},
						},
					},
				},
				Required: []string{"users"},
			},
			params: arrayTestParamsToMap(ArrayTestParams{
				Users: []ComplexUserParams{
					{
						Name:  "Alice Smith",
						Email: "alice@example.com",
						Age:   30,
						Roles: []string{"admin", "user"},
						Preferences: UserPreferences{
							Theme:         "dark",
							Notifications: true,
						},
					},
					{
						Name:  "Bob Johnson",
						Email: "bob@example.com",
						Age:   25,
						Roles: []string{"user"},
						Preferences: UserPreferences{
							Theme:         "light",
							Notifications: false,
						},
					},
				},
			}),
			wantErr: false,
		},
		{
			name: "complex validation failure in nested array",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"data": {
						Type: "array",
						Items: &tools.Property{
							Type: "object",
							Properties: map[string]*tools.Property{
								"id": {
									Type:    "integer",
									Minimum: float64Ptr2(1),
								},
								"value": {
									Type:    "string",
									Pattern: "^[A-Z]+$",
								},
							},
							Required: []string{"id", "value"},
						},
					},
				},
			},
			params: complexValidationParamsToMap(ComplexValidationParams{
				Data: []ValidationTestData{
					{
						ID:    1,
						Value: "ABC",
					},
					{
						ID:    2,
						Value: "abc", // This should fail the pattern
					},
				},
			}),
			wantErr: true,
			errMsg:  "data[1].value: string \"abc\" does not match pattern \"^[A-Z]+$\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_EdgeCases(t *testing.T) {
	t.Run("nil schema", func(t *testing.T) {
		validator := tools.NewSchemaValidator(nil)
		err := validator.Validate(map[string]interface{}{"any": "data"})
		assert.NoError(t, err, "nil schema should accept any data")
	})

	t.Run("empty parameters", func(t *testing.T) {
		schema := &tools.ToolSchema{
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]*tools.Property{
				"name": {Type: "string"},
			},
		}
		validator := tools.NewSchemaValidator(schema)
		err := validator.Validate(map[string]interface{}{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required property is missing")
	})

	t.Run("unknown property type", func(t *testing.T) {
		schema := &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"unknown": {Type: "unknownType"},
			},
		}
		validator := tools.NewSchemaValidator(schema)
		err := validator.Validate(map[string]interface{}{"unknown": "value"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown type")
	})
}

// Helper functions for creating pointers
func intPtr2(i int) *int {
	return &i
}

func float64Ptr2(f float64) *float64 {
	return &f
}

func boolPtr2(b bool) *bool {
	return &b
}

// Benchmarks
func BenchmarkSchemaValidator_Simple(b *testing.B) {
	schema := &tools.ToolSchema{
		Type: "object",
		Properties: map[string]*tools.Property{
			"name": {Type: "string"},
			"age":  {Type: "number"},
		},
		Required: []string{"name"},
	}

	validator := tools.NewSchemaValidator(schema)
	params := map[string]interface{}{
		"name": "John Doe",
		"age":  30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(params)
	}
}

func BenchmarkSchemaValidator_Complex(b *testing.B) {
	schema := &tools.ToolSchema{
		Type: "object",
		Properties: map[string]*tools.Property{
			"user": {
				Type: "object",
				Properties: map[string]*tools.Property{
					"name":  {Type: "string", MinLength: intPtr2(1), MaxLength: intPtr2(100)},
					"email": {Type: "string", Format: "email"},
					"age":   {Type: "integer", Minimum: float64Ptr2(0), Maximum: float64Ptr2(150)},
					"tags":  {Type: "array", Items: &tools.Property{Type: "string"}},
				},
				Required: []string{"name", "email"},
			},
			"metadata": {
				Type: "object",
				Properties: map[string]*tools.Property{
					"created_at": {Type: "string", Format: "date-time"},
					"version":    {Type: "string", Pattern: "^v\\d+\\.\\d+\\.\\d+$"},
				},
			},
		},
		Required: []string{"user"},
	}

	validator := tools.NewSchemaValidator(schema)
	params := map[string]interface{}{
		"user": map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
			"age":   30,
			"tags":  []interface{}{"admin", "user"},
		},
		"metadata": map[string]interface{}{
			"created_at": "2023-01-01T00:00:00Z",
			"version":    "v1.2.3",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(params)
	}
}

func BenchmarkSchemaValidator_ValidationFailure(b *testing.B) {
	schema := &tools.ToolSchema{
		Type: "object",
		Properties: map[string]*tools.Property{
			"email": {Type: "string", Format: "email"},
			"age":   {Type: "integer", Minimum: float64Ptr2(0)},
		},
		Required: []string{"email", "age"},
	}

	validator := tools.NewSchemaValidator(schema)
	params := map[string]interface{}{
		"email": "invalid-email",
		"age":   -5,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.Validate(params)
	}
}

// Tests for advanced JSON Schema features

func TestSchemaValidator_OneOf(t *testing.T) {
	tests := []struct {
		name    string
		schema  *tools.ToolSchema
		params  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid oneOf - matches first schema",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"value": {
						OneOf: []*tools.Property{
							{Type: "string", MinLength: intPtr2(5)},
							{Type: "integer", Minimum: float64Ptr2(10)},
						},
					},
				},
			},
			params:  map[string]interface{}{"value": "hello"},
			wantErr: false,
		},
		{
			name: "valid oneOf - matches second schema",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"value": {
						OneOf: []*tools.Property{
							{Type: "string", MinLength: intPtr2(10)},
							{Type: "integer", Minimum: float64Ptr2(5)},
						},
					},
				},
			},
			params:  map[string]interface{}{"value": 15},
			wantErr: false,
		},
		{
			name: "invalid oneOf - matches no schemas",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"value": {
						OneOf: []*tools.Property{
							{Type: "string", MinLength: intPtr2(10)},
							{Type: "integer", Minimum: float64Ptr2(20)},
						},
					},
				},
			},
			params:  map[string]interface{}{"value": "short"},
			wantErr: true,
		},
		{
			name: "invalid oneOf - matches multiple schemas",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"value": {
						OneOf: []*tools.Property{
							{Type: "string"},
							{Type: "string", MinLength: intPtr2(1)},
						},
					},
				},
			},
			params:  map[string]interface{}{"value": "test"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_AnyOf(t *testing.T) {
	tests := []struct {
		name    string
		schema  *tools.ToolSchema
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid anyOf - matches first schema",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"value": {
						AnyOf: []*tools.Property{
							{Type: "string"},
							{Type: "integer"},
						},
					},
				},
			},
			params:  map[string]interface{}{"value": "test"},
			wantErr: false,
		},
		{
			name: "valid anyOf - matches multiple schemas",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"value": {
						AnyOf: []*tools.Property{
							{Type: "string"},
							{Type: "string", MinLength: intPtr2(1)},
						},
					},
				},
			},
			params:  map[string]interface{}{"value": "test"},
			wantErr: false,
		},
		{
			name: "invalid anyOf - matches no schemas",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"value": {
						AnyOf: []*tools.Property{
							{Type: "string", MinLength: intPtr2(10)},
							{Type: "integer", Minimum: float64Ptr2(100)},
						},
					},
				},
			},
			params:  map[string]interface{}{"value": "short"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_AllOf(t *testing.T) {
	tests := []struct {
		name    string
		schema  *tools.ToolSchema
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid allOf - matches all schemas",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"value": {
						AllOf: []*tools.Property{
							{Type: "string"},
							{MinLength: intPtr2(3)},
							{MaxLength: intPtr2(10)},
						},
					},
				},
			},
			params:  map[string]interface{}{"value": "test"},
			wantErr: false,
		},
		{
			name: "invalid allOf - fails one schema",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"value": {
						AllOf: []*tools.Property{
							{Type: "string"},
							{MinLength: intPtr2(10)},
						},
					},
				},
			},
			params:  map[string]interface{}{"value": "short"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_Not(t *testing.T) {
	tests := []struct {
		name    string
		schema  *tools.ToolSchema
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid not - value does not match",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"value": {
						Not: &tools.Property{Type: "string"},
					},
				},
			},
			params:  map[string]interface{}{"value": 123},
			wantErr: false,
		},
		{
			name: "invalid not - value matches",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"value": {
						Not: &tools.Property{Type: "string"},
					},
				},
			},
			params:  map[string]interface{}{"value": "test"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_Conditional(t *testing.T) {
	tests := []struct {
		name    string
		schema  *tools.ToolSchema
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "conditional - if true, then applies",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"value": {
						If:   &tools.Property{Type: "string"},
						Then: &tools.Property{MinLength: intPtr2(5)},
					},
				},
			},
			params:  map[string]interface{}{"value": "hello"},
			wantErr: false,
		},
		{
			name: "conditional - if true, then fails",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"value": {
						If:   &tools.Property{Type: "string"},
						Then: &tools.Property{MinLength: intPtr2(10)},
					},
				},
			},
			params:  map[string]interface{}{"value": "short"},
			wantErr: true,
		},
		{
			name: "conditional - if false, else applies",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"value": {
						If:   &tools.Property{Type: "string"},
						Else: &tools.Property{Minimum: float64Ptr2(10)},
					},
				},
			},
			params:  map[string]interface{}{"value": 15},
			wantErr: false,
		},
		{
			name: "conditional - if false, else fails",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"value": {
						If:   &tools.Property{Type: "string"},
						Else: &tools.Property{Minimum: float64Ptr2(20)},
					},
				},
			},
			params:  map[string]interface{}{"value": 15},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_TypeCoercion(t *testing.T) {
	tests := []struct {
		name           string
		schema         *tools.ToolSchema
		params         map[string]interface{}
		expectedParams map[string]interface{}
		wantErr        bool
	}{
		{
			name: "string to number coercion",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"age": {Type: "number"},
				},
			},
			params:         map[string]interface{}{"age": "25"},
			expectedParams: map[string]interface{}{"age": 25.0},
			wantErr:        false,
		},
		{
			name: "number to string coercion",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"id": {Type: "string"},
				},
			},
			params:         map[string]interface{}{"id": 123},
			expectedParams: map[string]interface{}{"id": "123"},
			wantErr:        false,
		},
		{
			name: "string to boolean coercion",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"active": {Type: "boolean"},
				},
			},
			params:         map[string]interface{}{"active": "true"},
			expectedParams: map[string]interface{}{"active": true},
			wantErr:        false,
		},
		{
			name: "default value injection",
			schema: &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"count": {Type: "integer", Default: 10},
				},
			},
			params:         map[string]interface{}{},
			expectedParams: map[string]interface{}{"count": 10},
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(tt.schema)
			validator.SetCoercionEnabled(true)

			result := validator.ValidateWithResult(tt.params)
			if tt.wantErr {
				assert.False(t, result.Valid)
			} else {
				assert.True(t, result.Valid)
				assert.Equal(t, tt.expectedParams, result.Data)
			}
		})
	}
}

func TestSchemaValidator_CustomFormats(t *testing.T) {
	validator := tools.NewSchemaValidator(&tools.ToolSchema{
		Type: "object",
		Properties: map[string]*tools.Property{
			"code": {Type: "string", Format: "product-code"},
		},
	})

	// Add custom format validator
	validator.AddCustomFormat("product-code", func(value string) error {
		if len(value) != 8 || value[:2] != "PC" {
			return fmt.Errorf("invalid product code format")
		}
		return nil
	})

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid custom format",
			params:  map[string]interface{}{"code": "PC123456"},
			wantErr: false,
		},
		{
			name:    "invalid custom format",
			params:  map[string]interface{}{"code": "AB123456"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSchemaValidator_EnhancedFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
		value  string
		valid  bool
	}{
		// Enhanced email validation
		{
			name:   "valid RFC5322 email",
			format: "email",
			value:  "user@example.com",
			valid:  true,
		},
		{
			name:   "valid email with display name",
			format: "email",
			value:  "John Doe <john@example.com>",
			valid:  true,
		},
		// Enhanced URL validation
		{
			name:   "valid URL with path",
			format: "url",
			value:  "https://example.com/path?query=value",
			valid:  true,
		},
		{
			name:   "invalid URL missing scheme",
			format: "url",
			value:  "example.com",
			valid:  false,
		},
		// IPv4 validation
		{
			name:   "valid IPv4",
			format: "ipv4",
			value:  "192.168.1.1",
			valid:  true,
		},
		{
			name:   "invalid IPv4",
			format: "ipv4",
			value:  "256.1.1.1",
			valid:  false,
		},
		// IPv6 validation
		{
			name:   "valid IPv6",
			format: "ipv6",
			value:  "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			valid:  true,
		},
		{
			name:   "valid IPv6 compressed",
			format: "ipv6",
			value:  "2001:db8::8a2e:370:7334",
			valid:  true,
		},
		// Hostname validation
		{
			name:   "valid hostname",
			format: "hostname",
			value:  "example.com",
			valid:  true,
		},
		{
			name:   "invalid hostname",
			format: "hostname",
			value:  "ex ample.com",
			valid:  false,
		},
		// UUID validation
		{
			name:   "valid UUID v4",
			format: "uuid",
			value:  "550e8400-e29b-41d4-a716-446655440000",
			valid:  true,
		},
		{
			name:   "invalid UUID",
			format: "uuid",
			value:  "550e8400-e29b-41d4-a716",
			valid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &tools.ToolSchema{
				Type: "object",
				Properties: map[string]*tools.Property{
					"field": {Type: "string", Format: tt.format},
				},
			}
			validator := tools.NewSchemaValidator(schema)
			err := validator.Validate(map[string]interface{}{"field": tt.value})
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestSchemaValidator_ValidationCache(t *testing.T) {
	schema := &tools.ToolSchema{
		Type: "object",
		Properties: map[string]*tools.Property{
			"name": {Type: "string"},
		},
	}

	validator := tools.NewSchemaValidator(schema)
	params := map[string]interface{}{"name": "test"}

	// First validation
	result1 := validator.ValidateWithResult(params)
	assert.True(t, result1.Valid)

	// Second validation should use cache
	result2 := validator.ValidateWithResult(params)
	assert.True(t, result2.Valid)
	assert.Equal(t, result1.Data, result2.Data)

	// Clear cache
	validator.ClearCache()

	// Third validation should work after cache clear
	result3 := validator.ValidateWithResult(params)
	assert.True(t, result3.Valid)
}

func TestSchemaValidator_AdvancedOptions(t *testing.T) {
	schema := &tools.ToolSchema{
		Type: "object",
		Properties: map[string]*tools.Property{
			"age": {Type: "number"},
		},
	}

	opts := &tools.ValidatorOptions{
		CoercionEnabled: false,
		Debug:           true,
		CacheSize:       100,
	}

	validator := tools.NewAdvancedSchemaValidator(schema, opts)

	// Without coercion, string should fail for number type
	result := validator.ValidateWithResult(map[string]interface{}{"age": "25"})
	assert.False(t, result.Valid)

	// Enable coercion
	validator.SetCoercionEnabled(true)
	result = validator.ValidateWithResult(map[string]interface{}{"age": "25"})
	assert.True(t, result.Valid)
	assert.Equal(t, 25.0, result.Data.(map[string]interface{})["age"])
}

func TestSchemaValidator_ComplexComposition(t *testing.T) {
	// Test complex schema with multiple composition types
	schema := &tools.ToolSchema{
		Type: "object",
		Properties: map[string]*tools.Property{
			"data": {
				AllOf: []*tools.Property{
					{
						OneOf: []*tools.Property{
							{Type: "string", MinLength: intPtr2(1)},
							{Type: "number", Minimum: float64Ptr2(0)},
						},
					},
					{
						Not: &tools.Property{
							Type: "boolean",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid string",
			params:  map[string]interface{}{"data": "test"},
			wantErr: false,
		},
		{
			name:    "valid number",
			params:  map[string]interface{}{"data": 42},
			wantErr: false,
		},
		{
			name:    "invalid boolean (matches not)",
			params:  map[string]interface{}{"data": true},
			wantErr: true,
		},
		{
			name:    "invalid empty string",
			params:  map[string]interface{}{"data": ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tools.NewSchemaValidator(schema)
			err := validator.Validate(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationResult_ErrorCodes(t *testing.T) {
	schema := &tools.ToolSchema{
		Type: "object",
		Properties: map[string]*tools.Property{
			"value": {
				OneOf: []*tools.Property{
					{Type: "string"},
					{Type: "number"},
				},
			},
		},
	}

	validator := tools.NewSchemaValidator(schema)
	result := validator.ValidateWithResult(map[string]interface{}{"value": true})

	assert.False(t, result.Valid)
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Code, "ONEOF")
}
