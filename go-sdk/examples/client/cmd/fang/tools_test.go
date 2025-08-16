package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterTools(t *testing.T) {
	tools := []Tool{
		{
			Name:         "http_get",
			Description:  "Make HTTP GET requests",
			Tags:         []string{"network", "http"},
			Capabilities: []string{"async", "retry"},
		},
		{
			Name:         "http_post",
			Description:  "Make HTTP POST requests",
			Tags:         []string{"network", "http"},
			Capabilities: []string{"async", "streaming"},
		},
		{
			Name:         "file_read",
			Description:  "Read file contents",
			Tags:         []string{"filesystem", "io"},
			Capabilities: []string{"local"},
		},
		{
			Name:         "data_transform",
			Description:  "Transform data formats",
			Tags:         []string{"data", "transformation"},
			Capabilities: []string{"streaming"},
		},
	}

	tests := []struct {
		name             string
		nameFilter       string
		capabilityFilter string
		tagFilter        string
		expectedCount    int
		expectedTools    []string
	}{
		{
			name:          "no filters",
			expectedCount: 4,
			expectedTools: []string{"http_get", "http_post", "file_read", "data_transform"},
		},
		{
			name:          "filter by name - http",
			nameFilter:    "http",
			expectedCount: 2,
			expectedTools: []string{"http_get", "http_post"},
		},
		{
			name:          "filter by name - file",
			nameFilter:    "file",
			expectedCount: 1,
			expectedTools: []string{"file_read"},
		},
		{
			name:             "filter by capability - streaming",
			capabilityFilter: "streaming",
			expectedCount:    2,
			expectedTools:    []string{"http_post", "data_transform"},
		},
		{
			name:             "filter by capability - async",
			capabilityFilter: "async",
			expectedCount:    2,
			expectedTools:    []string{"http_get", "http_post"},
		},
		{
			name:          "filter by tag - network",
			tagFilter:     "network",
			expectedCount: 2,
			expectedTools: []string{"http_get", "http_post"},
		},
		{
			name:          "filter by tag - filesystem",
			tagFilter:     "filesystem",
			expectedCount: 1,
			expectedTools: []string{"file_read"},
		},
		{
			name:             "combined filters - http and async",
			nameFilter:       "http",
			capabilityFilter: "async",
			expectedCount:    2,
			expectedTools:    []string{"http_get", "http_post"},
		},
		{
			name:             "combined filters - restrictive",
			nameFilter:       "file",
			capabilityFilter: "streaming",
			expectedCount:    0,
			expectedTools:    []string{},
		},
		{
			name:          "case insensitive name filter",
			nameFilter:    "HTTP",
			expectedCount: 2,
			expectedTools: []string{"http_get", "http_post"},
		},
		{
			name:             "case insensitive capability filter",
			capabilityFilter: "STREAMING",
			expectedCount:    2,
			expectedTools:    []string{"http_post", "data_transform"},
		},
		{
			name:          "case insensitive tag filter",
			tagFilter:     "NETWORK",
			expectedCount: 2,
			expectedTools: []string{"http_get", "http_post"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterTools(tools, tt.nameFilter, tt.capabilityFilter, tt.tagFilter)
			
			assert.Equal(t, tt.expectedCount, len(result), "unexpected number of filtered tools")
			
			resultNames := make([]string, len(result))
			for i, tool := range result {
				resultNames[i] = tool.Name
			}
			
			assert.ElementsMatch(t, tt.expectedTools, resultNames, "unexpected tools in result")
		})
	}
}

func TestRenderToolsJSON(t *testing.T) {
	tools := []Tool{
		{
			Name:         "test_tool",
			Description:  "A test tool",
			Tags:         []string{"test", "example"},
			Capabilities: []string{"async"},
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"input": map[string]interface{}{
						"type":        "string",
						"description": "Test input",
					},
				},
				"required": []string{"input"},
			},
		},
	}

	t.Run("without verbose", func(t *testing.T) {
		var buf bytes.Buffer
		renderToolsJSON(&buf, tools, false)
		
		output := buf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		require.Len(t, lines, 1, "should output one line per tool")
		
		var result map[string]interface{}
		err := json.Unmarshal([]byte(lines[0]), &result)
		require.NoError(t, err, "output should be valid JSON")
		
		assert.Equal(t, "test_tool", result["name"])
		assert.Equal(t, "A test tool", result["description"])
		assert.Equal(t, []interface{}{"test", "example"}, result["tags"])
		assert.Equal(t, []interface{}{"async"}, result["capabilities"])
		assert.Nil(t, result["schema"], "schema should not be included without verbose")
	})

	t.Run("with verbose", func(t *testing.T) {
		var buf bytes.Buffer
		renderToolsJSON(&buf, tools, true)
		
		output := buf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		require.Len(t, lines, 1, "should output one line per tool")
		
		var result map[string]interface{}
		err := json.Unmarshal([]byte(lines[0]), &result)
		require.NoError(t, err, "output should be valid JSON")
		
		assert.Equal(t, "test_tool", result["name"])
		assert.Equal(t, "A test tool", result["description"])
		assert.NotNil(t, result["schema"], "schema should be included with verbose")
		
		schema := result["schema"].(map[string]interface{})
		assert.Equal(t, "object", schema["type"])
		assert.NotNil(t, schema["properties"])
		assert.Equal(t, []interface{}{"input"}, schema["required"])
	})

	t.Run("multiple tools", func(t *testing.T) {
		multiTools := []Tool{
			{Name: "tool1", Description: "First tool"},
			{Name: "tool2", Description: "Second tool"},
			{Name: "tool3", Description: "Third tool"},
		}
		
		var buf bytes.Buffer
		renderToolsJSON(&buf, multiTools, false)
		
		output := buf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		require.Len(t, lines, 3, "should output one line per tool")
		
		for i, line := range lines {
			var result map[string]interface{}
			err := json.Unmarshal([]byte(line), &result)
			require.NoError(t, err, "line %d should be valid JSON", i+1)
			assert.Equal(t, multiTools[i].Name, result["name"])
			assert.Equal(t, multiTools[i].Description, result["description"])
		}
	})

	t.Run("empty tools list", func(t *testing.T) {
		var buf bytes.Buffer
		renderToolsJSON(&buf, []Tool{}, false)
		
		output := buf.String()
		assert.Empty(t, strings.TrimSpace(output), "should output nothing for empty tools list")
	})
}

func TestRenderToolsPretty(t *testing.T) {
	t.Run("empty tools list", func(t *testing.T) {
		var buf bytes.Buffer
		renderToolsPretty(&buf, []Tool{}, false, true)
		
		output := buf.String()
		assert.Contains(t, output, "No tools found")
	})

	t.Run("single tool without verbose", func(t *testing.T) {
		tools := []Tool{
			{
				Name:         "test_tool",
				Description:  "A test tool",
				Tags:         []string{"test", "example"},
				Capabilities: []string{"async", "retry"},
			},
		}
		
		var buf bytes.Buffer
		renderToolsPretty(&buf, tools, false, true)
		
		output := buf.String()
		assert.Contains(t, output, "Available Tools:")
		assert.Contains(t, output, "test_tool")
		assert.Contains(t, output, "A test tool")
		assert.Contains(t, output, "Tags: test, example")
		assert.Contains(t, output, "Capabilities: async, retry")
		assert.Contains(t, output, "Total: 1 tools")
		assert.NotContains(t, output, "Parameters:", "should not show parameters without verbose")
	})

	t.Run("single tool with verbose", func(t *testing.T) {
		tools := []Tool{
			{
				Name:         "test_tool",
				Description:  "A test tool",
				Tags:         []string{"test"},
				Capabilities: []string{"async"},
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"input": map[string]interface{}{
							"type":        "string",
							"description": "Test input",
							"format":      "email",
						},
						"count": map[string]interface{}{
							"type":        "integer",
							"description": "Item count",
						},
					},
					"required": []string{"input"},
				},
			},
		}
		
		var buf bytes.Buffer
		renderToolsPretty(&buf, tools, true, true)
		
		output := buf.String()
		assert.Contains(t, output, "Parameters:")
		assert.Contains(t, output, "Type: object")
		assert.Contains(t, output, "Properties:")
		assert.Contains(t, output, "input:")
		assert.Contains(t, output, "type: string")
		assert.Contains(t, output, "description: Test input")
		assert.Contains(t, output, "format: email")
		assert.Contains(t, output, "count:")
		assert.Contains(t, output, "type: integer")
		assert.Contains(t, output, "Required: [input]")
	})

	t.Run("multiple tools", func(t *testing.T) {
		tools := []Tool{
			{
				Name:        "tool1",
				Description: "First tool",
				Tags:        []string{"tag1"},
			},
			{
				Name:        "tool2",
				Description: "Second tool",
				Tags:        []string{"tag2"},
			},
		}
		
		var buf bytes.Buffer
		renderToolsPretty(&buf, tools, false, true)
		
		output := buf.String()
		assert.Contains(t, output, "tool1")
		assert.Contains(t, output, "First tool")
		assert.Contains(t, output, "tool2")
		assert.Contains(t, output, "Second tool")
		assert.Contains(t, output, "Total: 2 tools")
	})

	t.Run("with color", func(t *testing.T) {
		tools := []Tool{
			{
				Name:        "colored_tool",
				Description: "Tool with color",
			},
		}
		
		var buf bytes.Buffer
		renderToolsPretty(&buf, tools, false, false)
		
		output := buf.String()
		// Check for ANSI color codes
		assert.Contains(t, output, "\033[1;34mcolored_tool\033[0m")
	})

	t.Run("without color", func(t *testing.T) {
		tools := []Tool{
			{
				Name:        "plain_tool",
				Description: "Tool without color",
			},
		}
		
		var buf bytes.Buffer
		renderToolsPretty(&buf, tools, false, true)
		
		output := buf.String()
		// Check that there are no ANSI color codes
		assert.NotContains(t, output, "\033[")
		assert.Contains(t, output, "plain_tool")
	})
}

func TestRenderParameterSchema(t *testing.T) {
	t.Run("with enum values", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"format": map[string]interface{}{
					"type":        "string",
					"description": "Output format",
					"enum":        []interface{}{"json", "xml", "yaml"},
				},
			},
		}
		
		var buf bytes.Buffer
		renderParameterSchema(&buf, schema, "  ")
		
		output := buf.String()
		assert.Contains(t, output, "Type: object")
		assert.Contains(t, output, "format:")
		assert.Contains(t, output, "enum: [json, xml, yaml]")
	})

	t.Run("nested properties", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"config": map[string]interface{}{
					"type":        "object",
					"description": "Configuration object",
				},
			},
		}
		
		var buf bytes.Buffer
		renderParameterSchema(&buf, schema, "  ")
		
		output := buf.String()
		assert.Contains(t, output, "config:")
		assert.Contains(t, output, "type: object")
		assert.Contains(t, output, "description: Configuration object")
	})

	t.Run("required fields", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"field1": map[string]interface{}{"type": "string"},
				"field2": map[string]interface{}{"type": "number"},
			},
			"required": []interface{}{"field1", "field2"},
		}
		
		var buf bytes.Buffer
		renderParameterSchema(&buf, schema, "  ")
		
		output := buf.String()
		assert.Contains(t, output, "Required: [field1, field2]")
	})
}