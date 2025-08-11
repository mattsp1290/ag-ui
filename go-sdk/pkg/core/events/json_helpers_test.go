package events

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareJSON(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
		options  JSONCompareOptions
		wantErr  bool
	}{
		{
			name:     "identical JSON",
			expected: `{"type":"TEST","messageId":"123"}`,
			actual:   `{"type":"TEST","messageId":"123"}`,
			options:  DefaultJSONCompareOptions(),
			wantErr:  false,
		},
		{
			name:     "different field order",
			expected: `{"messageId":"123","type":"TEST"}`,
			actual:   `{"type":"TEST","messageId":"123"}`,
			options:  DefaultJSONCompareOptions(),
			wantErr:  false,
		},
		{
			name:     "null vs absent field",
			expected: `{"type":"TEST","role":null}`,
			actual:   `{"type":"TEST"}`,
			options:  DefaultJSONCompareOptions(),
			wantErr:  false,
		},
		{
			name:     "value mismatch",
			expected: `{"type":"TEST","messageId":"123"}`,
			actual:   `{"type":"TEST","messageId":"456"}`,
			options:  DefaultJSONCompareOptions(),
			wantErr:  true,
		},
		{
			name:     "ignore timestamps",
			expected: `{"type":"TEST","timestamp":1000}`,
			actual:   `{"type":"TEST","timestamp":2000}`,
			options: JSONCompareOptions{
				IgnoreFieldOrder: true,
				IgnoreTimestamps: true,
			},
			wantErr: false,
		},
		{
			name:     "nested objects",
			expected: `{"type":"TEST","data":{"nested":"value"}}`,
			actual:   `{"type":"TEST","data":{"nested":"value"}}`,
			options:  DefaultJSONCompareOptions(),
			wantErr:  false,
		},
		{
			name:     "arrays in order",
			expected: `{"items":["a","b","c"]}`,
			actual:   `{"items":["a","b","c"]}`,
			options:  DefaultJSONCompareOptions(),
			wantErr:  false,
		},
		{
			name:     "arrays out of order - should fail",
			expected: `{"items":["a","b","c"]}`,
			actual:   `{"items":["c","b","a"]}`,
			options:  DefaultJSONCompareOptions(),
			wantErr:  true,
		},
		{
			name:     "arrays out of order - ignore order",
			expected: `{"items":["a","b","c"]}`,
			actual:   `{"items":["c","b","a"]}`,
			options: JSONCompareOptions{
				IgnoreFieldOrder: true,
				IgnoreArrayOrder: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CompareJSON([]byte(tt.expected), []byte(tt.actual), tt.options)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNormalizeJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove null fields",
			input:    `{"type":"TEST","role":null,"messageId":"123"}`,
			expected: `{"messageId":"123","type":"TEST"}`,
		},
		{
			name:     "preserve non-null fields",
			input:    `{"type":"TEST","role":"assistant","messageId":"123"}`,
			expected: `{"messageId":"123","role":"assistant","type":"TEST"}`,
		},
		{
			name:     "nested null removal",
			input:    `{"type":"TEST","data":{"field1":"value","field2":null}}`,
			expected: `{"data":{"field1":"value"},"type":"TEST"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeJSON([]byte(tt.input))
			require.NoError(t, err)

			// Parse both to compare as objects (field order doesn't matter)
			var expectedObj, resultObj map[string]interface{}
			err = json.Unmarshal([]byte(tt.expected), &expectedObj)
			require.NoError(t, err)
			err = json.Unmarshal(result, &resultObj)
			require.NoError(t, err)

			assert.Equal(t, expectedObj, resultObj)
		})
	}
}

func TestGetJSONFieldNames(t *testing.T) {
	input := `{"type":"TEST","messageId":"123","role":"assistant"}`
	fields, err := GetJSONFieldNames([]byte(input))
	require.NoError(t, err)

	expected := []string{"messageId", "role", "type"}
	assert.Equal(t, expected, fields)
}

func TestValidateCamelCase(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "all camelCase",
			input:   `{"type":"TEST","messageId":"123","toolCallId":"456"}`,
			wantErr: false,
		},
		{
			name:    "snake_case field",
			input:   `{"type":"TEST","message_id":"123"}`,
			wantErr: true,
		},
		{
			name:    "PascalCase allowed for type",
			input:   `{"Type":"TEST","messageId":"123"}`,
			wantErr: false, // We're lenient with PascalCase
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCamelCase([]byte(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestJSONHelpers_EventRoundtrip(t *testing.T) {
	// Test with actual event structures
	event := NewTextMessageStartEvent("msg-123", WithRole("assistant"))
	event.SetTimestamp(1672531200000)

	// Encode to JSON
	data, err := json.Marshal(event)
	require.NoError(t, err)

	// Validate camelCase
	err = ValidateCamelCase(data)
	assert.NoError(t, err)

	// Get field names
	fields, err := GetJSONFieldNames(data)
	require.NoError(t, err)
	assert.Contains(t, fields, "type")
	assert.Contains(t, fields, "messageId")
	assert.Contains(t, fields, "role")
	assert.Contains(t, fields, "timestamp")

	// Test normalization (removes nulls)
	eventWithoutRole := NewTextMessageStartEvent("msg-456")
	dataWithoutRole, err := json.Marshal(eventWithoutRole)
	require.NoError(t, err)

	normalized, err := NormalizeJSON(dataWithoutRole)
	require.NoError(t, err)

	// Should not contain "role" field after normalization
	var normalizedObj map[string]interface{}
	err = json.Unmarshal(normalized, &normalizedObj)
	require.NoError(t, err)
	_, hasRole := normalizedObj["role"]
	assert.False(t, hasRole, "Normalized JSON should not contain null 'role' field")
}