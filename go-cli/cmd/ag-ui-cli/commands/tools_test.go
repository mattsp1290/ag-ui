package commands

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseToolArguments(t *testing.T) {
	tests := []struct {
		name     string
		argsJSON string
		argPairs []string
		expected map[string]interface{}
		wantErr  bool
	}{
		{
			name:     "JSON args only",
			argsJSON: `{"key1": "value1", "key2": 42}`,
			argPairs: []string{},
			expected: map[string]interface{}{
				"key1": "value1",
				"key2": float64(42),
			},
		},
		{
			name:     "Key-value pairs only",
			argsJSON: "",
			argPairs: []string{"key1=value1", "key2=42"},
			expected: map[string]interface{}{
				"key1": "value1",
				"key2": float64(42),
			},
		},
		{
			name:     "JSON args with key-value override",
			argsJSON: `{"key1": "original", "key2": 10}`,
			argPairs: []string{"key1=overridden", "key3=new"},
			expected: map[string]interface{}{
				"key1": "overridden",
				"key2": float64(10),
				"key3": "new",
			},
		},
		{
			name:     "Boolean and array values",
			argsJSON: "",
			argPairs: []string{"enabled=true", "items=[1,2,3]", "config={\"nested\":\"value\"}"},
			expected: map[string]interface{}{
				"enabled": true,
				"items":   []interface{}{float64(1), float64(2), float64(3)},
				"config":  map[string]interface{}{"nested": "value"},
			},
		},
		{
			name:     "Invalid JSON args",
			argsJSON: `{invalid json}`,
			argPairs: []string{},
			wantErr:  true,
		},
		{
			name:     "Invalid key-value format",
			argsJSON: "",
			argPairs: []string{"invalid-no-equals"},
			wantErr:  true,
		},
		{
			name:     "String values with spaces",
			argsJSON: "",
			argPairs: []string{"message=Hello World", "path=/some/path with spaces"},
			expected: map[string]interface{}{
				"message": "Hello World",
				"path":    "/some/path with spaces",
			},
		},
		{
			name:     "Empty args",
			argsJSON: "",
			argPairs: []string{},
			expected: map[string]interface{}{},
		},
		{
			name:     "Numeric string vs actual number",
			argsJSON: "",
			argPairs: []string{"port=8080", "host=\"localhost\""},
			expected: map[string]interface{}{
				"port": float64(8080),
				"host": "localhost",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseToolArguments(tt.argsJSON, tt.argPairs)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// Unmarshal result to compare
			var actual map[string]interface{}
			err = json.Unmarshal(result, &actual)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{
			name:     "Item exists",
			slice:    []string{"apple", "banana", "orange"},
			item:     "banana",
			expected: true,
		},
		{
			name:     "Item does not exist",
			slice:    []string{"apple", "banana", "orange"},
			item:     "grape",
			expected: false,
		},
		{
			name:     "Empty slice",
			slice:    []string{},
			item:     "apple",
			expected: false,
		},
		{
			name:     "Nil slice",
			slice:    nil,
			item:     "apple",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToolsRunCommand_Flags(t *testing.T) {
	// Test that required flags are properly configured
	cmd := toolsRunCmd
	
	// Check that tool flag is required
	toolFlag := cmd.Flag("tool")
	assert.NotNil(t, toolFlag)
	assert.Equal(t, "tool", toolFlag.Name)
	
	// Check other flags exist
	assert.NotNil(t, cmd.Flag("args-json"))
	assert.NotNil(t, cmd.Flag("arg"))
	assert.NotNil(t, cmd.Flag("timeout"))
}

func TestToolsListCommand_OutputFormat(t *testing.T) {
	// This is a placeholder for integration tests
	// In a real scenario, we would mock the HTTP client
	// and test the output formatting
}