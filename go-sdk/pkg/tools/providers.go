package tools

import (
	"encoding/json"
	"fmt"
)

// OpenAITool represents a tool in OpenAI's function calling format.
// This structure is used when converting tools for use with OpenAI's API.
// The Type field is always "function" for tool definitions.
type OpenAITool struct {
	Type     string             `json:"type"`
	Function OpenAIToolFunction `json:"function"`
}

// OpenAIToolFunction represents the function definition in OpenAI format.
// It contains the function name, description, and parameter schema
// formatted according to OpenAI's specifications.
type OpenAIToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// OpenAIToolCall represents a tool call in OpenAI format.
// This is used when OpenAI's API requests a tool execution.
// The ID field is used to correlate responses with requests.
type OpenAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function OpenAIFunctionCall `json:"function"`
}

// OpenAIFunctionCall represents a function call in OpenAI format.
// The Arguments field contains JSON-encoded parameters for the function.
type OpenAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// OpenAIToolMessage represents a tool response message in OpenAI format.
// This is used to send tool execution results back to OpenAI.
// The role is always "tool" and ToolCallID must match the request ID.
type OpenAIToolMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id"`
}

// AnthropicTool represents a tool in Anthropic's format.
// This structure is used when converting tools for use with Anthropic's Claude API.
// The InputSchema follows JSON Schema format.
type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// AnthropicToolUse represents a tool use request in Anthropic format.
// This is used when Claude requests a tool execution.
// The ID field is used to track the tool use throughout the conversation.
type AnthropicToolUse struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// AnthropicToolResult represents a tool result in Anthropic format.
// This is used to send tool execution results back to Claude.
// The IsError field indicates whether the tool execution failed.
type AnthropicToolResult struct {
	ToolUseID string      `json:"tool_use_id"`
	Content   interface{} `json:"content"`
	IsError   bool        `json:"is_error,omitempty"`
}

// ProviderConverter handles conversion between AG-UI tools and provider formats.
// It provides methods to convert tool definitions and execution requests/results
// between AG-UI's internal format and various AI provider formats.
//
// Supported conversions:
//   - AG-UI Tool → OpenAI Tool
//   - AG-UI Tool → Anthropic Tool
//   - OpenAI Tool Call → AG-UI execution request
//   - Anthropic Tool Use → AG-UI execution request
//   - AG-UI Result → OpenAI Tool Message
//   - AG-UI Result → Anthropic Tool Result
type ProviderConverter struct{}

// NewProviderConverter creates a new provider converter.
// The converter is stateless and can be reused for multiple conversions.
func NewProviderConverter() *ProviderConverter {
	return &ProviderConverter{}
}

// ConvertToOpenAITool converts an AG-UI tool to OpenAI format.
// The tool's schema is transformed to match OpenAI's parameter format.
// Returns an error if the tool is nil or the schema cannot be converted.
func (pc *ProviderConverter) ConvertToOpenAITool(tool *Tool) (*OpenAITool, error) {
	if tool == nil {
		return nil, fmt.Errorf("tool cannot be nil")
	}

	// Convert schema to OpenAI parameters format
	parameters := pc.schemaToOpenAIParameters(tool.Schema)

	return &OpenAITool{
		Type: "function",
		Function: OpenAIToolFunction{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  parameters,
		},
	}, nil
}

// ConvertToAnthropicTool converts an AG-UI tool to Anthropic format.
// The tool's schema is transformed to match Anthropic's input schema format.
// Returns an error if the tool is nil or the schema cannot be converted.
func (pc *ProviderConverter) ConvertToAnthropicTool(tool *Tool) (*AnthropicTool, error) {
	if tool == nil {
		return nil, fmt.Errorf("tool cannot be nil")
	}

	// Convert schema to Anthropic input schema format
	inputSchema := pc.schemaToAnthropicInputSchema(tool.Schema)

	return &AnthropicTool{
		Name:        tool.Name,
		Description: tool.Description,
		InputSchema: inputSchema,
	}, nil
}

// ConvertOpenAIToolCall converts an OpenAI tool call to AG-UI format.
// Extracts the tool name and parses the JSON arguments.
// Returns the tool name and parsed parameters, or an error if parsing fails.
func (pc *ProviderConverter) ConvertOpenAIToolCall(call *OpenAIToolCall) (string, map[string]interface{}, error) {
	if call == nil {
		return "", nil, fmt.Errorf("tool call cannot be nil")
	}

	// Parse arguments from JSON string
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return "", nil, fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	return call.Function.Name, args, nil
}

// ConvertAnthropicToolUse converts an Anthropic tool use to AG-UI format.
// Extracts the tool name and input parameters.
// Returns the tool name and parameters, or an error if the input is nil.
func (pc *ProviderConverter) ConvertAnthropicToolUse(use *AnthropicToolUse) (string, map[string]interface{}, error) {
	if use == nil {
		return "", nil, fmt.Errorf("tool use cannot be nil")
	}

	return use.Name, use.Input, nil
}

// ConvertResultToOpenAI converts an AG-UI tool result to OpenAI format.
// Creates a tool message with the execution result formatted as JSON.
// The toolCallID must match the ID from the original OpenAI tool call.
func (pc *ProviderConverter) ConvertResultToOpenAI(result *ToolExecutionResult, toolCallID string) (*OpenAIToolMessage, error) {
	if result == nil {
		return nil, fmt.Errorf("result cannot be nil")
	}

	var content string
	if result.Success {
		// Marshal the data to JSON for OpenAI
		data, err := json.Marshal(result.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result data: %w", err)
		}
		content = string(data)
	} else {
		content = result.Error
	}

	return &OpenAIToolMessage{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	}, nil
}

// ConvertResultToAnthropic converts an AG-UI tool result to Anthropic format.
func (pc *ProviderConverter) ConvertResultToAnthropic(result *ToolExecutionResult, toolUseID string) (*AnthropicToolResult, error) {
	if result == nil {
		return nil, fmt.Errorf("result cannot be nil")
	}

	return &AnthropicToolResult{
		ToolUseID: toolUseID,
		Content:   result.Data,
		IsError:   !result.Success,
	}, nil
}

// schemaToOpenAIParameters converts AG-UI schema to OpenAI parameters format.
func (pc *ProviderConverter) schemaToOpenAIParameters(schema *ToolSchema) map[string]interface{} {
	if schema == nil {
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}

	params := map[string]interface{}{
		"type": schema.Type,
	}

	// Always include properties, even if empty
	properties := make(map[string]interface{})
	for name, prop := range schema.Properties {
		properties[name] = pc.propertyToOpenAI(prop)
	}
	params["properties"] = properties

	if len(schema.Required) > 0 {
		params["required"] = schema.Required
	}

	if schema.AdditionalProperties != nil {
		params["additionalProperties"] = *schema.AdditionalProperties
	}

	if schema.Description != "" {
		params["description"] = schema.Description
	}

	return params
}

// schemaToAnthropicInputSchema converts AG-UI schema to Anthropic input schema format.
func (pc *ProviderConverter) schemaToAnthropicInputSchema(schema *ToolSchema) map[string]interface{} {
	// Anthropic uses the same JSON Schema format as OpenAI
	return pc.schemaToOpenAIParameters(schema)
}

// propertyToOpenAI converts an AG-UI property to OpenAI format.
func (pc *ProviderConverter) propertyToOpenAI(prop *Property) map[string]interface{} {
	if prop == nil {
		return map[string]interface{}{}
	}

	result := map[string]interface{}{
		"type": prop.Type,
	}

	if prop.Description != "" {
		result["description"] = prop.Description
	}

	if prop.Format != "" {
		result["format"] = prop.Format
	}

	if len(prop.Enum) > 0 {
		result["enum"] = prop.Enum
	}

	if prop.Default != nil {
		result["default"] = prop.Default
	}

	// Add constraints based on type
	switch prop.Type {
	case "string":
		if prop.MinLength != nil {
			result["minLength"] = *prop.MinLength
		}
		if prop.MaxLength != nil {
			result["maxLength"] = *prop.MaxLength
		}
		if prop.Pattern != "" {
			result["pattern"] = prop.Pattern
		}

	case "number", "integer":
		if prop.Minimum != nil {
			result["minimum"] = *prop.Minimum
		}
		if prop.Maximum != nil {
			result["maximum"] = *prop.Maximum
		}

	case "array":
		if prop.Items != nil {
			result["items"] = pc.propertyToOpenAI(prop.Items)
		}
		if prop.MinLength != nil {
			result["minItems"] = *prop.MinLength
		}
		if prop.MaxLength != nil {
			result["maxItems"] = *prop.MaxLength
		}

	case "object":
		if len(prop.Properties) > 0 {
			properties := make(map[string]interface{})
			for name, subProp := range prop.Properties {
				properties[name] = pc.propertyToOpenAI(subProp)
			}
			result["properties"] = properties
		}
		if len(prop.Required) > 0 {
			result["required"] = prop.Required
		}
	}

	return result
}

// StreamingToolCallConverter handles streaming tool call conversion.
type StreamingToolCallConverter struct {
	converter *ProviderConverter
	buffer    string
	toolName  string
	toolID    string
}

// NewStreamingToolCallConverter creates a new streaming tool call converter.
func NewStreamingToolCallConverter() *StreamingToolCallConverter {
	return &StreamingToolCallConverter{
		converter: NewProviderConverter(),
	}
}

// AddOpenAIChunk adds an OpenAI streaming chunk.
func (stc *StreamingToolCallConverter) AddOpenAIChunk(chunk map[string]interface{}) error {
	// Extract tool call information from chunk
	if toolCalls, ok := chunk["tool_calls"].([]interface{}); ok && len(toolCalls) > 0 {
		if toolCall, ok := toolCalls[0].(map[string]interface{}); ok {
			if id, ok := toolCall["id"].(string); ok && id != "" {
				stc.toolID = id
			}
			if function, ok := toolCall["function"].(map[string]interface{}); ok {
				if name, ok := function["name"].(string); ok && name != "" {
					stc.toolName = name
				}
				if args, ok := function["arguments"].(string); ok {
					stc.buffer += args
				}
			}
		}
	}
	return nil
}

// GetToolCall returns the accumulated tool call if complete.
func (stc *StreamingToolCallConverter) GetToolCall() (string, string, map[string]interface{}, error) {
	if stc.toolName == "" {
		return "", "", nil, fmt.Errorf("tool name not available")
	}

	// Try to parse the accumulated arguments
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(stc.buffer), &args); err != nil {
		return "", "", nil, fmt.Errorf("incomplete or invalid arguments")
	}

	return stc.toolID, stc.toolName, args, nil
}

// ProviderToolRegistry manages tools for different AI providers.
type ProviderToolRegistry struct {
	registry  *Registry
	converter *ProviderConverter
}

// NewProviderToolRegistry creates a new provider tool registry.
func NewProviderToolRegistry(registry *Registry) *ProviderToolRegistry {
	return &ProviderToolRegistry{
		registry:  registry,
		converter: NewProviderConverter(),
	}
}

// GetOpenAITools returns all tools in OpenAI format.
func (ptr *ProviderToolRegistry) GetOpenAITools() ([]*OpenAITool, error) {
	tools, err := ptr.registry.ListAll()
	if err != nil {
		return nil, err
	}

	openAITools := make([]*OpenAITool, 0, len(tools))
	for _, tool := range tools {
		openAITool, err := ptr.converter.ConvertToOpenAITool(tool)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool %q: %w", tool.Name, err)
		}
		openAITools = append(openAITools, openAITool)
	}

	return openAITools, nil
}

// GetAnthropicTools returns all tools in Anthropic format.
func (ptr *ProviderToolRegistry) GetAnthropicTools() ([]*AnthropicTool, error) {
	tools, err := ptr.registry.ListAll()
	if err != nil {
		return nil, err
	}

	anthropicTools := make([]*AnthropicTool, 0, len(tools))
	for _, tool := range tools {
		anthropicTool, err := ptr.converter.ConvertToAnthropicTool(tool)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool %q: %w", tool.Name, err)
		}
		anthropicTools = append(anthropicTools, anthropicTool)
	}

	return anthropicTools, nil
}
