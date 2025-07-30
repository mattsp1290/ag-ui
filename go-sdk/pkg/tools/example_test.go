package tools_test

import (
	"context"
	"fmt"
	"time"

	"github.com/ag-ui/go-sdk/pkg/tools"
)

// Type-safe test parameter structures
type WeatherParams struct {
	Location string `json:"location"`
	Units    string `json:"units,omitempty"`
}

type CounterParams struct {
	Count int `json:"count"`
}

type SearchParams struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

// Helper function to convert typed params to map[string]interface{}
func paramsToMap(params interface{}) map[string]interface{} {
	switch p := params.(type) {
	case WeatherParams:
		result := map[string]interface{}{"location": p.Location}
		if p.Units != "" {
			result["units"] = p.Units
		}
		return result
	case CounterParams:
		return map[string]interface{}{"count": float64(p.Count)}
	case SearchParams:
		result := map[string]interface{}{"query": p.Query}
		if p.MaxResults > 0 {
			result["max_results"] = p.MaxResults
		}
		return result
	default:
		return map[string]interface{}{}
	}
}

// Example demonstrates basic tool usage
func Example() {
	// Create a new tool registry
	registry := tools.NewRegistry()

	// Define a simple weather tool
	weatherTool := &tools.Tool{
		ID:          "example.weather",
		Name:        "get_weather",
		Description: "Get current weather for a location",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"location": {
					Type:        "string",
					Description: "City name or coordinates",
				},
				"units": {
					Type:        "string",
					Description: "Temperature units",
					Enum:        []interface{}{"celsius", "fahrenheit"},
					Default:     "celsius",
				},
			},
			Required: []string{"location"},
		},
		Executor: &weatherExecutor{},
		Capabilities: &tools.ToolCapabilities{
			Timeout:   5 * time.Second,
			Cacheable: true,
		},
	}

	// Register the tool
	if err := registry.Register(weatherTool); err != nil {
		fmt.Printf("Error registering tool: %v\n", err)
		return
	}

	// Create execution engine
	engine := tools.NewExecutionEngine(registry)

	// Execute the tool
	weatherParams := WeatherParams{
		Location: "San Francisco",
		Units:    "fahrenheit",
	}

	result, err := engine.Execute(context.Background(), "example.weather", paramsToMap(weatherParams))
	if err != nil {
		fmt.Printf("Error executing tool: %v\n", err)
		return
	}

	if result.Success {
		fmt.Printf("Weather: %v\n", result.Data)
	} else {
		fmt.Printf("Error: %s\n", result.Error)
	}

	// Output:
	// Weather: San Francisco: 72°F, sunny
}

// ExampleTool_streaming demonstrates streaming tool usage
func ExampleTool_streaming() {
	// Create registry and register a streaming tool
	registry := tools.NewRegistry()

	streamingTool := &tools.Tool{
		ID:          "example.counter",
		Name:        "counter",
		Description: "Counts and streams numbers",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"count": {
					Type:        "integer",
					Description: "How many numbers to count",
					Minimum:     &[]float64{1}[0],
					Maximum:     &[]float64{10}[0],
				},
			},
			Required: []string{"count"},
		},
		Executor: &counterExecutor{},
		Capabilities: &tools.ToolCapabilities{
			Streaming: true,
			Timeout:   10 * time.Second,
		},
	}

	if err := registry.Register(streamingTool); err != nil {
		fmt.Printf("Error registering streaming tool: %v\n", err)
		return
	}

	// Create execution engine
	engine := tools.NewExecutionEngine(registry)

	// Execute streaming tool
	counterParams := CounterParams{
		Count: 5,
	}

	stream, err := engine.ExecuteStream(context.Background(), "example.counter", paramsToMap(counterParams))
	if err != nil {
		fmt.Printf("Error executing streaming tool: %v\n", err)
		return
	}

	// Process stream
	for chunk := range stream {
		switch chunk.Type {
		case "data":
			fmt.Printf("Received: %v\n", chunk.Data)
		case "complete":
			fmt.Println("Stream complete")
		case "error":
			fmt.Printf("Error: %v\n", chunk.Data)
		}
	}

	// Output:
	// Received: 1
	// Received: 2
	// Received: 3
	// Received: 4
	// Received: 5
	// Stream complete
}

// ExampleProviderConverter demonstrates AI provider integration
func ExampleProviderConverter() {
	// Create a tool
	tool := &tools.Tool{
		ID:          "example.search",
		Name:        "web_search",
		Description: "Search the web for information",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"query": {
					Type:        "string",
					Description: "Search query",
					MinLength:   &[]int{1}[0],
				},
				"max_results": {
					Type:        "integer",
					Description: "Maximum number of results",
					Default:     10,
					Minimum:     &[]float64{1}[0],
					Maximum:     &[]float64{100}[0],
				},
			},
			Required: []string{"query"},
		},
		Executor: &searchExecutor{},
	}

	// Convert to OpenAI format
	converter := tools.NewProviderConverter()
	openAITool, err := converter.ConvertToOpenAITool(tool)
	if err != nil {
		fmt.Printf("Error converting to OpenAI tool: %v\n", err)
		return
	}

	fmt.Printf("OpenAI Tool Name: %s\n", openAITool.Function.Name)
	fmt.Printf("OpenAI Tool Type: %s\n", openAITool.Type)

	// Convert to Anthropic format
	anthropicTool, err := converter.ConvertToAnthropicTool(tool)
	if err != nil {
		fmt.Printf("Error converting to Anthropic tool: %v\n", err)
		return
	}

	fmt.Printf("Anthropic Tool Name: %s\n", anthropicTool.Name)

	// Output:
	// OpenAI Tool Name: web_search
	// OpenAI Tool Type: function
	// Anthropic Tool Name: web_search
}

// ExampleRegistry demonstrates tool registry operations
func ExampleRegistry() {
	registry := tools.NewRegistry()

	// Register built-in tools
	if err := tools.RegisterBuiltinTools(registry); err != nil {
		fmt.Printf("Error registering builtin tools: %v\n", err)
		return
	}

	// List all tools
	allTools, err := registry.ListAll()
	if err != nil {
		fmt.Printf("Error listing all tools: %v\n", err)
		return
	}

	fmt.Printf("Total tools: %d\n", len(allTools))

	// Filter tools by capability
	filter := &tools.ToolFilter{
		Capabilities: &tools.ToolCapabilities{
			Cacheable: true,
		},
	}

	cacheableTools, err := registry.List(filter)
	if err != nil {
		fmt.Printf("Error listing cacheable tools: %v\n", err)
		return
	}

	fmt.Printf("Cacheable tools: %d\n", len(cacheableTools))

	// Get specific tool
	jsonTool, err := registry.GetByName("json_parse")
	if err != nil {
		fmt.Printf("Error getting json_parse tool: %v\n", err)
		return
	}

	fmt.Printf("Found tool: %s\n", jsonTool.Description)

	// Output:
	// Total tools: 8
	// Cacheable tools: 2
	// Found tool: Parse JSON string into structured data
}

// Mock executors for examples

type weatherExecutor struct{}

func (e *weatherExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	location := params["location"].(string)
	units := "celsius"
	if u, ok := params["units"].(string); ok {
		units = u
	}

	// Mock weather data
	temp := "22°C"
	if units == "fahrenheit" {
		temp = "72°F"
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    fmt.Sprintf("%s: %s, sunny", location, temp),
	}, nil
}

type counterExecutor struct{}

func (e *counterExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	return &tools.ToolExecutionResult{
		Success: true,
		Data:    "Use streaming for this tool",
	}, nil
}

func (e *counterExecutor) ExecuteStream(ctx context.Context, params map[string]interface{}) (<-chan *tools.ToolStreamChunk, error) {
	count := int(params["count"].(float64))
	ch := make(chan *tools.ToolStreamChunk)

	go func() {
		defer close(ch)

		for i := 1; i <= count; i++ {
			select {
			case ch <- &tools.ToolStreamChunk{
				Type:  "data",
				Data:  i,
				Index: i - 1,
			}:
				time.Sleep(100 * time.Millisecond) // Simulate work
			case <-ctx.Done():
				return
			}
		}

		ch <- &tools.ToolStreamChunk{
			Type:  "complete",
			Index: count,
		}
	}()

	return ch, nil
}

type searchExecutor struct{}

func (e *searchExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	query := params["query"].(string)
	maxResults := 10
	if mr, ok := params["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	// Mock search results
	results := []string{}
	for i := 1; i <= maxResults && i <= 3; i++ {
		results = append(results, fmt.Sprintf("Result %d for '%s'", i, query))
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    results,
	}, nil
}
