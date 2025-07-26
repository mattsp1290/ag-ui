package tools_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Type-safe parameter structures for provider testing

// TestToolParams represents typed parameters for test tool execution
type TestToolParams struct {
	Message string                 `json:"message"`
	Count   int                    `json:"count,omitempty"`
	Options []string              `json:"options,omitempty"`
	Config  TestToolConfigParams  `json:"config,omitempty"`
}

// TestToolConfigParams represents configuration for test tools
type TestToolConfigParams struct {
	Enabled bool `json:"enabled"`
}

// OpenAIToolCallParams represents typed parameters for OpenAI tool calls
type OpenAIToolCallParams struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// AnthropicToolUseParams represents typed parameters for Anthropic tool use
type AnthropicToolUseParams struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// ToolExecutionResultData represents typed result data
type ToolExecutionResultData struct {
	Output string `json:"output"`
	Count  int    `json:"count"`
}

// ComplexResultData represents complex nested result data
type ComplexResultData struct {
	Nested ComplexNestedData `json:"nested"`
}

// ComplexNestedData represents nested data structure
type ComplexNestedData struct {
	Value string        `json:"value"`
	Array []int         `json:"array"`
}

// StreamingChunkData represents data in OpenAI streaming chunks
type StreamingChunkData struct {
	ToolCalls []StreamingToolCall `json:"tool_calls"`
}

// StreamingToolCall represents a tool call in streaming data
type StreamingToolCall struct {
	ID       string                  `json:"id,omitempty"`
	Function StreamingFunctionCall  `json:"function"`
}

// StreamingFunctionCall represents function call data in streaming
type StreamingFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// Helper functions to convert typed structures to map[string]interface{}

// testToolParamsToMap converts TestToolParams to map for legacy API compatibility
func testToolParamsToMap(params TestToolParams) map[string]interface{} {
	result := make(map[string]interface{})
	result["message"] = params.Message
	if params.Count > 0 {
		result["count"] = params.Count
	}
	if len(params.Options) > 0 {
		result["options"] = params.Options
	}
	if params.Config.Enabled {
		result["config"] = map[string]interface{}{
			"enabled": params.Config.Enabled,
		}
	}
	return result
}

// toolExecutionResultDataToMap converts typed result data to map
func toolExecutionResultDataToMap(data ToolExecutionResultData) map[string]interface{} {
	return map[string]interface{}{
		"output": data.Output,
		"count":  data.Count,
	}
}

// complexResultDataToMap converts complex result data to map
func complexResultDataToMap(data ComplexResultData) map[string]interface{} {
	return map[string]interface{}{
		"nested": map[string]interface{}{
			"value": data.Nested.Value,
			"array": data.Nested.Array,
		},
	}
}

// anthropicToolUseParamsToMap converts Anthropic tool use params to map
func anthropicToolUseParamsToMap(params AnthropicToolUseParams) map[string]interface{} {
	return map[string]interface{}{
		"message": params.Message,
		"count":   params.Count,
	}
}

// streamingChunkToMap converts streaming chunk data to map
func streamingChunkToMap(chunk StreamingChunkData) map[string]interface{} {
	toolCalls := make([]interface{}, len(chunk.ToolCalls))
	for i, tc := range chunk.ToolCalls {
		toolCall := make(map[string]interface{})
		if tc.ID != "" {
			toolCall["id"] = tc.ID
		}
		function := make(map[string]interface{})
		if tc.Function.Name != "" {
			function["name"] = tc.Function.Name
		}
		if tc.Function.Arguments != "" {
			function["arguments"] = tc.Function.Arguments
		}
		toolCall["function"] = function
		toolCalls[i] = toolCall
	}
	return map[string]interface{}{
		"tool_calls": toolCalls,
	}
}

// mockProviderExecutor is a test implementation of ToolExecutor for provider tests
type mockProviderExecutor struct {
	result *tools.ToolExecutionResult
	err    error
}

func (m *mockProviderExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// createTypedProviderExecutor creates a mock executor with typed result data
func createTypedProviderExecutor(resultData ToolExecutionResultData) *mockProviderExecutor {
	return &mockProviderExecutor{
		result: &tools.ToolExecutionResult{
			Success: true,
			Data:    toolExecutionResultDataToMap(resultData),
		},
	}
}

// Helper function to create a test tool for provider tests
func createProviderTestTool() *tools.Tool {
	minLen := 3
	maxLen := 100
	minVal := 0.0
	maxVal := 100.0
	additionalProps := false

	return &tools.Tool{
		ID:          "test-tool",
		Name:        "TestTool",
		Description: "A test tool for unit testing",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"message": {
					Type:        "string",
					Description: "The message to process",
					MinLength:   &minLen,
					MaxLength:   &maxLen,
					Pattern:     "^[a-zA-Z0-9 ]+$",
				},
				"count": {
					Type:        "integer",
					Description: "Number of times to repeat",
					Minimum:     &minVal,
					Maximum:     &maxVal,
				},
				"options": {
					Type:        "array",
					Description: "List of options",
					Items: &tools.Property{
						Type: "string",
					},
				},
				"config": {
					Type:        "object",
					Description: "Configuration object",
					Properties: map[string]*tools.Property{
						"enabled": {
							Type:        "boolean",
							Description: "Whether feature is enabled",
							Default:     true,
						},
					},
					Required: []string{"enabled"},
				},
			},
			Required:             []string{"message"},
			AdditionalProperties: &additionalProps,
			Description:          "Test tool schema",
		},
		Metadata: &tools.ToolMetadata{
			Author:        "Test Author",
			Tags:          []string{"test", "example"},
			Documentation: "https://example.com/docs",
		},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  true,
			Async:      true,
			Cancelable: true,
			RateLimit:  60,
			Timeout:    30 * time.Second,
		},
		Executor: createTypedProviderExecutor(ToolExecutionResultData{
			Output: "success",
			Count:  1,
		}),
	}
}

func TestProviderConverter_Creation(t *testing.T) {
	pc := tools.NewProviderConverter()
	assert.NotNil(t, pc)
}

func TestProviderConverter_ConvertToOpenAITool(t *testing.T) {
	pc := tools.NewProviderConverter()

	t.Run("Success with full tool", func(t *testing.T) {
		tool := createProviderTestTool()
		openAITool, err := pc.ConvertToOpenAITool(tool)

		require.NoError(t, err)
		assert.NotNil(t, openAITool)
		assert.Equal(t, "function", openAITool.Type)
		assert.Equal(t, tool.Name, openAITool.Function.Name)
		assert.Equal(t, tool.Description, openAITool.Function.Description)
		assert.NotNil(t, openAITool.Function.Parameters)

		// Verify parameters structure
		params := openAITool.Function.Parameters
		assert.Equal(t, "object", params["type"])
		assert.NotNil(t, params["properties"])
		assert.NotNil(t, params["required"])
		assert.Equal(t, false, params["additionalProperties"])
		assert.Equal(t, "Test tool schema", params["description"])

		// Verify properties
		props := params["properties"].(map[string]interface{})
		assert.NotNil(t, props["message"])
		assert.NotNil(t, props["count"])
		assert.NotNil(t, props["options"])
		assert.NotNil(t, props["config"])
	})

	t.Run("Success with minimal tool", func(t *testing.T) {
		tool := &tools.Tool{
			ID:          "minimal",
			Name:        "MinimalTool",
			Description: "A minimal tool",
			Version:     "1.0.0",
			Schema: &tools.ToolSchema{
				Type: "object",
			},
			Executor: &mockProviderExecutor{},
		}

		openAITool, err := pc.ConvertToOpenAITool(tool)

		require.NoError(t, err)
		assert.NotNil(t, openAITool)
		assert.Equal(t, "function", openAITool.Type)
		assert.Equal(t, tool.Name, openAITool.Function.Name)
		assert.Equal(t, tool.Description, openAITool.Function.Description)

		params := openAITool.Function.Parameters
		assert.Equal(t, "object", params["type"])
		assert.NotNil(t, params["properties"])
	})

	t.Run("Error with nil tool", func(t *testing.T) {
		_, err := pc.ConvertToOpenAITool(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool cannot be nil")
	})

	t.Run("Tool with nil schema", func(t *testing.T) {
		tool := &tools.Tool{
			ID:          "no-schema",
			Name:        "NoSchemaTest",
			Description: "Tool without schema",
			Version:     "1.0.0",
			Executor:    &mockProviderExecutor{},
		}

		openAITool, err := pc.ConvertToOpenAITool(tool)

		require.NoError(t, err)
		params := openAITool.Function.Parameters
		assert.Equal(t, "object", params["type"])
		assert.NotNil(t, params["properties"])
	})
}

func TestProviderConverter_ConvertToAnthropicTool(t *testing.T) {
	pc := tools.NewProviderConverter()

	t.Run("Success with full tool", func(t *testing.T) {
		tool := createProviderTestTool()
		anthropicTool, err := pc.ConvertToAnthropicTool(tool)

		require.NoError(t, err)
		assert.NotNil(t, anthropicTool)
		assert.Equal(t, tool.Name, anthropicTool.Name)
		assert.Equal(t, tool.Description, anthropicTool.Description)
		assert.NotNil(t, anthropicTool.InputSchema)

		// Verify input schema structure
		assert.Equal(t, "object", anthropicTool.InputSchema["type"])
		assert.NotNil(t, anthropicTool.InputSchema["properties"])
		assert.NotNil(t, anthropicTool.InputSchema["required"])
	})

	t.Run("Error with nil tool", func(t *testing.T) {
		_, err := pc.ConvertToAnthropicTool(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool cannot be nil")
	})
}

func TestProviderConverter_ConvertOpenAIToolCall(t *testing.T) {
	pc := tools.NewProviderConverter()

	t.Run("Success", func(t *testing.T) {
		call := &tools.OpenAIToolCall{
			ID:   "call_123",
			Type: "function",
			Function: tools.OpenAIFunctionCall{
				Name:      "TestTool",
				Arguments: `{"message": "hello", "count": 5}`,
			},
		}

		name, args, err := pc.ConvertOpenAIToolCall(call)

		// Verify type-safe parameter extraction
		require.NoError(t, err)
		assert.Equal(t, "TestTool", name)
		assert.Equal(t, "hello", args["message"])
		assert.Equal(t, float64(5), args["count"])
		
		// Verify parameters can be safely converted to typed structure
		var params OpenAIToolCallParams
		paramBytes, _ := json.Marshal(args)
		err = json.Unmarshal(paramBytes, &params)
		require.NoError(t, err)
		assert.Equal(t, "hello", params.Message)
		assert.Equal(t, 5, params.Count)
	})

	t.Run("Error with nil call", func(t *testing.T) {
		_, _, err := pc.ConvertOpenAIToolCall(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool call cannot be nil")
	})

	t.Run("Error with invalid JSON arguments", func(t *testing.T) {
		call := &tools.OpenAIToolCall{
			ID:   "call_123",
			Type: "function",
			Function: tools.OpenAIFunctionCall{
				Name:      "TestTool",
				Arguments: `{"invalid json`,
			},
		}

		_, _, err := pc.ConvertOpenAIToolCall(call)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse tool arguments")
	})

	t.Run("Success with empty arguments", func(t *testing.T) {
		call := &tools.OpenAIToolCall{
			ID:   "call_123",
			Type: "function",
			Function: tools.OpenAIFunctionCall{
				Name:      "TestTool",
				Arguments: `{}`,
			},
		}

		name, args, err := pc.ConvertOpenAIToolCall(call)

		require.NoError(t, err)
		assert.Equal(t, "TestTool", name)
		assert.Empty(t, args)
	})
}

func TestProviderConverter_ConvertAnthropicToolUse(t *testing.T) {
	pc := tools.NewProviderConverter()

	t.Run("Success", func(t *testing.T) {
		// Create typed parameters and convert to map
		typedParams := AnthropicToolUseParams{
			Message: "hello",
			Count:   5,
		}
		use := &tools.AnthropicToolUse{
			ID:    "use_123",
			Name:  "TestTool",
			Input: anthropicToolUseParamsToMap(typedParams),
		}

		name, args, err := pc.ConvertAnthropicToolUse(use)

		// Verify type-safe parameter extraction
		require.NoError(t, err)
		assert.Equal(t, "TestTool", name)
		assert.Equal(t, "hello", args["message"])
		assert.Equal(t, 5, args["count"])
		
		// Verify parameters can be safely converted to typed structure
		var params AnthropicToolUseParams
		paramBytes, _ := json.Marshal(args)
		err = json.Unmarshal(paramBytes, &params)
		require.NoError(t, err)
		assert.Equal(t, "hello", params.Message)
		assert.Equal(t, 5, params.Count)
	})

	t.Run("Error with nil use", func(t *testing.T) {
		_, _, err := pc.ConvertAnthropicToolUse(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool use cannot be nil")
	})

	t.Run("Success with empty input", func(t *testing.T) {
		use := &tools.AnthropicToolUse{
			ID:    "use_123",
			Name:  "TestTool",
			Input: map[string]interface{}{},
		}

		name, args, err := pc.ConvertAnthropicToolUse(use)

		require.NoError(t, err)
		assert.Equal(t, "TestTool", name)
		assert.Empty(t, args)
	})
}

func TestProviderConverter_ConvertResultToOpenAI(t *testing.T) {
	pc := tools.NewProviderConverter()

	t.Run("Success result", func(t *testing.T) {
		// Create typed result data
		typedData := ToolExecutionResultData{
			Output: "processed",
			Count:  42,
		}
		result := &tools.ToolExecutionResult{
			Success:   true,
			Data:      toolExecutionResultDataToMap(typedData),
			Timestamp: time.Now(),
		}

		msg, err := pc.ConvertResultToOpenAI(result, "call_123")

		require.NoError(t, err)
		assert.NotNil(t, msg)
		assert.Equal(t, "tool", msg.Role)
		assert.Equal(t, "call_123", msg.ToolCallID)

		// Verify the content is valid JSON and can be converted to typed structure
		var data ToolExecutionResultData
		err = json.Unmarshal([]byte(msg.Content), &data)
		require.NoError(t, err)
		assert.Equal(t, "processed", data.Output)
		assert.Equal(t, 42, data.Count)
	})

	t.Run("Error result", func(t *testing.T) {
		result := &tools.ToolExecutionResult{
			Success:   false,
			Error:     "Something went wrong",
			Timestamp: time.Now(),
		}

		msg, err := pc.ConvertResultToOpenAI(result, "call_123")

		require.NoError(t, err)
		assert.NotNil(t, msg)
		assert.Equal(t, "tool", msg.Role)
		assert.Equal(t, "call_123", msg.ToolCallID)
		assert.Equal(t, "Something went wrong", msg.Content)
	})

	t.Run("Error with nil result", func(t *testing.T) {
		_, err := pc.ConvertResultToOpenAI(nil, "call_123")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "result cannot be nil")
	})

	t.Run("Success with complex data", func(t *testing.T) {
		// Create complex typed result data
		typedData := ComplexResultData{
			Nested: ComplexNestedData{
				Value: "test",
				Array: []int{1, 2, 3},
			},
		}
		result := &tools.ToolExecutionResult{
			Success: true,
			Data:    complexResultDataToMap(typedData),
		}

		msg, err := pc.ConvertResultToOpenAI(result, "call_123")

		require.NoError(t, err)
		assert.NotNil(t, msg)

		var data map[string]interface{}
		err = json.Unmarshal([]byte(msg.Content), &data)
		require.NoError(t, err)
		assert.NotNil(t, data["nested"])
	})
}

func TestProviderConverter_ConvertResultToAnthropic(t *testing.T) {
	pc := tools.NewProviderConverter()

	t.Run("Success result", func(t *testing.T) {
		// Create typed result data for Anthropic conversion
		typedData := ToolExecutionResultData{
			Output: "processed",
			Count:  0, // Default value
		}
		result := &tools.ToolExecutionResult{
			Success:   true,
			Data:      toolExecutionResultDataToMap(typedData),
			Timestamp: time.Now(),
		}

		anthropicResult, err := pc.ConvertResultToAnthropic(result, "use_123")

		require.NoError(t, err)
		assert.NotNil(t, anthropicResult)
		assert.Equal(t, "use_123", anthropicResult.ToolUseID)
		assert.Equal(t, result.Data, anthropicResult.Content)
		assert.False(t, anthropicResult.IsError)
	})

	t.Run("Error result", func(t *testing.T) {
		result := &tools.ToolExecutionResult{
			Success:   false,
			Error:     "Something went wrong",
			Data:      "error details",
			Timestamp: time.Now(),
		}

		anthropicResult, err := pc.ConvertResultToAnthropic(result, "use_123")

		require.NoError(t, err)
		assert.NotNil(t, anthropicResult)
		assert.Equal(t, "use_123", anthropicResult.ToolUseID)
		assert.Equal(t, "error details", anthropicResult.Content)
		assert.True(t, anthropicResult.IsError)
	})

	t.Run("Error with nil result", func(t *testing.T) {
		_, err := pc.ConvertResultToAnthropic(nil, "use_123")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "result cannot be nil")
	})
}

func TestStreamingToolCallConverter(t *testing.T) {
	t.Run("Creation", func(t *testing.T) {
		stc := tools.NewStreamingToolCallConverter()
		assert.NotNil(t, stc)
	})

	t.Run("AddOpenAIChunk with complete tool call", func(t *testing.T) {
		stc := tools.NewStreamingToolCallConverter()

		// Create typed streaming chunk data
		typedChunk := StreamingChunkData{
			ToolCalls: []StreamingToolCall{
				{
					ID: "call_123",
					Function: StreamingFunctionCall{
						Name:      "TestTool",
						Arguments: `{"message": "hello"}`,
					},
				},
			},
		}
		chunk := streamingChunkToMap(typedChunk)

		err := stc.AddOpenAIChunk(chunk)
		require.NoError(t, err)

		// Get the tool call
		id, name, args, err := stc.GetToolCall()
		require.NoError(t, err)
		assert.Equal(t, "call_123", id)
		assert.Equal(t, "TestTool", name)
		assert.Equal(t, "hello", args["message"])
	})

	t.Run("AddOpenAIChunk with streaming arguments", func(t *testing.T) {
		stc := tools.NewStreamingToolCallConverter()

		// First chunk - tool info with typed data
		typedChunk1 := StreamingChunkData{
			ToolCalls: []StreamingToolCall{
				{
					ID: "call_123",
					Function: StreamingFunctionCall{
						Name:      "TestTool",
						Arguments: `{"mes`,
					},
				},
			},
		}
		chunk1 := streamingChunkToMap(typedChunk1)
		err := stc.AddOpenAIChunk(chunk1)
		require.NoError(t, err)

		// Second chunk - more arguments with typed data
		typedChunk2 := StreamingChunkData{
			ToolCalls: []StreamingToolCall{
				{
					Function: StreamingFunctionCall{
						Arguments: `sage": "hel`,
					},
				},
			},
		}
		chunk2 := streamingChunkToMap(typedChunk2)
		err = stc.AddOpenAIChunk(chunk2)
		require.NoError(t, err)

		// Third chunk - complete arguments with typed data
		typedChunk3 := StreamingChunkData{
			ToolCalls: []StreamingToolCall{
				{
					Function: StreamingFunctionCall{
						Arguments: `lo"}`,
					},
				},
			},
		}
		chunk3 := streamingChunkToMap(typedChunk3)
		err = stc.AddOpenAIChunk(chunk3)
		require.NoError(t, err)

		// Get the complete tool call
		id, name, args, err := stc.GetToolCall()
		require.NoError(t, err)
		assert.Equal(t, "call_123", id)
		assert.Equal(t, "TestTool", name)
		assert.Equal(t, "hello", args["message"])
	})

	t.Run("GetToolCall with incomplete arguments", func(t *testing.T) {
		stc := tools.NewStreamingToolCallConverter()

		// Create typed chunk with incomplete arguments
		typedChunk := StreamingChunkData{
			ToolCalls: []StreamingToolCall{
				{
					ID: "call_123",
					Function: StreamingFunctionCall{
						Name:      "TestTool",
						Arguments: `{"message": "incomplete`,
					},
				},
			},
		}
		chunk := streamingChunkToMap(typedChunk)

		err := stc.AddOpenAIChunk(chunk)
		require.NoError(t, err)

		_, _, _, err = stc.GetToolCall()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "incomplete or invalid arguments")
	})

	t.Run("GetToolCall without tool name", func(t *testing.T) {
		stc := tools.NewStreamingToolCallConverter()

		_, _, _, err := stc.GetToolCall()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tool name not available")
	})

	t.Run("AddOpenAIChunk with empty chunk", func(t *testing.T) {
		stc := tools.NewStreamingToolCallConverter()

		err := stc.AddOpenAIChunk(map[string]interface{}{})
		assert.NoError(t, err)
	})

	t.Run("AddOpenAIChunk with malformed tool_calls", func(t *testing.T) {
		stc := tools.NewStreamingToolCallConverter()

		// Create malformed chunk (not using typed conversion for this error case)
		chunk := map[string]interface{}{
			"tool_calls": "not an array",
		}

		err := stc.AddOpenAIChunk(chunk)
		assert.NoError(t, err) // Should not error, just ignore malformed data
	})
}

func TestProviderToolRegistry(t *testing.T) {
	t.Run("Creation", func(t *testing.T) {
		registry := tools.NewRegistry()
		ptr := tools.NewProviderToolRegistry(registry)
		assert.NotNil(t, ptr)
	})

	t.Run("GetOpenAITools", func(t *testing.T) {
		registry := tools.NewRegistry()
		ptr := tools.NewProviderToolRegistry(registry)

		// Register test tools
		tool1 := createProviderTestTool()
		tool1.ID = "tool1"
		tool1.Name = "Tool1"

		tool2 := createProviderTestTool()
		tool2.ID = "tool2"
		tool2.Name = "Tool2"

		err := registry.Register(tool1)
		require.NoError(t, err)
		err = registry.Register(tool2)
		require.NoError(t, err)

		// Get OpenAI tools
		openAITools, err := ptr.GetOpenAITools()
		require.NoError(t, err)
		assert.Len(t, openAITools, 2)

		// Verify tools are converted correctly
		names := []string{}
		for _, tool := range openAITools {
			names = append(names, tool.Function.Name)
			assert.Equal(t, "function", tool.Type)
			assert.NotEmpty(t, tool.Function.Description)
			assert.NotNil(t, tool.Function.Parameters)
		}
		assert.Contains(t, names, "Tool1")
		assert.Contains(t, names, "Tool2")
	})

	t.Run("GetAnthropicTools", func(t *testing.T) {
		registry := tools.NewRegistry()
		ptr := tools.NewProviderToolRegistry(registry)

		// Register test tools
		tool1 := createProviderTestTool()
		tool1.ID = "tool1"
		tool1.Name = "Tool1"

		err := registry.Register(tool1)
		require.NoError(t, err)

		// Get Anthropic tools
		anthropicTools, err := ptr.GetAnthropicTools()
		require.NoError(t, err)
		assert.Len(t, anthropicTools, 1)

		// Verify tool is converted correctly
		assert.Equal(t, "Tool1", anthropicTools[0].Name)
		assert.NotEmpty(t, anthropicTools[0].Description)
		assert.NotNil(t, anthropicTools[0].InputSchema)
	})

	t.Run("GetOpenAITools with empty registry", func(t *testing.T) {
		registry := tools.NewRegistry()
		ptr := tools.NewProviderToolRegistry(registry)

		openAITools, err := ptr.GetOpenAITools()
		require.NoError(t, err)
		assert.Empty(t, openAITools)
	})

	t.Run("GetAnthropicTools with empty registry", func(t *testing.T) {
		registry := tools.NewRegistry()
		ptr := tools.NewProviderToolRegistry(registry)

		anthropicTools, err := ptr.GetAnthropicTools()
		require.NoError(t, err)
		assert.Empty(t, anthropicTools)
	})
}

// TestSchemaConversion_EdgeCases tests internal implementation details
// and cannot be run from an external test package (tools_test).
// These tests should be moved to an internal test file if needed.
/*
func TestSchemaConversion_EdgeCases(t *testing.T) {
	pc := tools.NewProviderConverter()

	t.Run("Property with all string constraints", func(t *testing.T) {
		minLen := 5
		maxLen := 50
		prop := &tools.Property{
			Type:        "string",
			Description: "A constrained string",
			MinLength:   &minLen,
			MaxLength:   &maxLen,
			Pattern:     "^[A-Z][a-z]+$",
			Format:      "email",
			Enum:        []interface{}{"option1", "option2"},
			Default:     "option1",
		}

		result := pc.propertyToOpenAI(prop)

		assert.Equal(t, "string", result["type"])
		assert.Equal(t, "A constrained string", result["description"])
		assert.Equal(t, 5, result["minLength"])
		assert.Equal(t, 50, result["maxLength"])
		assert.Equal(t, "^[A-Z][a-z]+$", result["pattern"])
		assert.Equal(t, "email", result["format"])
		assert.Equal(t, []interface{}{"option1", "option2"}, result["enum"])
		assert.Equal(t, "option1", result["default"])
	})

	t.Run("Property with all number constraints", func(t *testing.T) {
		minVal := 0.0
		maxVal := 100.0
		prop := &tools.Property{
			Type:        "number",
			Description: "A constrained number",
			Minimum:     &minVal,
			Maximum:     &maxVal,
			Enum:        []interface{}{1.0, 2.5, 5.0},
			Default:     2.5,
		}

		result := pc.propertyToOpenAI(prop)

		assert.Equal(t, "number", result["type"])
		assert.Equal(t, 0.0, result["minimum"])
		assert.Equal(t, 100.0, result["maximum"])
		assert.Equal(t, []interface{}{1.0, 2.5, 5.0}, result["enum"])
		assert.Equal(t, 2.5, result["default"])
	})

	t.Run("Property with array constraints", func(t *testing.T) {
		minItems := 1
		maxItems := 10
		prop := &tools.Property{
			Type:        "array",
			Description: "An array with constraints",
			MinLength:   &minItems,
			MaxLength:   &maxItems,
			Items: &tools.Property{
				Type:   "string",
				Format: "uuid",
			},
		}

		result := pc.propertyToOpenAI(prop)

		assert.Equal(t, "array", result["type"])
		assert.Equal(t, 1, result["minItems"])
		assert.Equal(t, 10, result["maxItems"])
		assert.NotNil(t, result["items"])

		items := result["items"].(map[string]interface{})
		assert.Equal(t, "string", items["type"])
		assert.Equal(t, "uuid", items["format"])
	})

	t.Run("Nested object property", func(t *testing.T) {
		prop := &tools.Property{
			Type: "object",
			Properties: map[string]*tools.Property{
				"nested": {
					Type: "object",
					Properties: map[string]*tools.Property{
						"deep": {
							Type:    "string",
							Default: "value",
						},
					},
					Required: []string{"deep"},
				},
			},
			Required: []string{"nested"},
		}

		result := pc.propertyToOpenAI(prop)

		assert.Equal(t, "object", result["type"])
		assert.NotNil(t, result["properties"])
		assert.Equal(t, []string{"nested"}, result["required"])

		props := result["properties"].(map[string]interface{})
		nested := props["nested"].(map[string]interface{})
		assert.Equal(t, "object", nested["type"])
		assert.NotNil(t, nested["properties"])
		assert.Equal(t, []string{"deep"}, nested["required"])
	})

	t.Run("Property with nil values", func(t *testing.T) {
		prop := &tools.Property{
			Type: "string",
		}

		result := pc.propertyToOpenAI(prop)

		assert.Equal(t, "string", result["type"])
		assert.NotContains(t, result, "description")
		assert.NotContains(t, result, "minLength")
		assert.NotContains(t, result, "maxLength")
		assert.NotContains(t, result, "pattern")
		assert.NotContains(t, result, "enum")
		assert.NotContains(t, result, "default")
	})

	t.Run("Nil property", func(t *testing.T) {
		result := pc.propertyToOpenAI(nil)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("Schema with additionalProperties true", func(t *testing.T) {
		additionalProps := true
		tool := &tools.Tool{
			ID:          "test",
			Name:        "Test",
			Description: "Test",
			Version:     "1.0.0",
			Schema: &tools.ToolSchema{
				Type:                 "object",
				AdditionalProperties: &additionalProps,
			},
			Executor: &mockProviderExecutor{},
		}

		openAITool, err := pc.ConvertToOpenAITool(tool)
		require.NoError(t, err)

		params := openAITool.Function.Parameters
		assert.Equal(t, true, params["additionalProperties"])
	})

	t.Run("Complex enum values", func(t *testing.T) {
		prop := &tools.Property{
			Type: "string",
			Enum: []interface{}{
				"simple",
				"with-dash",
				"with_underscore",
				"CamelCase",
				"123numeric",
			},
		}

		result := pc.propertyToOpenAI(prop)
		assert.Equal(t, prop.Enum, result["enum"])
	})
}
*/
