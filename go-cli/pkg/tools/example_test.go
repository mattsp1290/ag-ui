package tools_test

import (
	"encoding/json"
	"fmt"
	"log"
	
	"github.com/mattsp1290/ag-ui/go-cli/pkg/tools"
)

func ExampleToolRegistry() {
	// Create a new registry
	registry := tools.NewToolRegistry()
	
	// Define a tool
	tool := &tools.Tool{
		Name:        "text_analyzer",
		Description: "Analyzes text for sentiment and keywords",
		Parameters: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"text": {
					Type:        "string",
					Description: "Text to analyze",
					MinLength:   intPtr(1),
					MaxLength:   intPtr(5000),
				},
				"language": {
					Type:        "string",
					Description: "Language of the text",
					Enum:        []interface{}{"en", "es", "fr", "de"},
					Default:     "en",
				},
			},
			Required: []string{"text"},
		},
	}
	
	// Register the tool
	if err := registry.Register(tool); err != nil {
		log.Fatal(err)
	}
	
	// Validate arguments
	args := json.RawMessage(`{"text": "Hello world", "language": "en"}`)
	if err := registry.ValidateArgs("text_analyzer", args); err != nil {
		log.Fatal(err)
	}
	
	// List all tools
	for _, t := range registry.List() {
		fmt.Printf("Tool: %s - %s\n", t.Name, t.Description)
	}
	
	// Output:
	// Tool: text_analyzer - Analyzes text for sentiment and keywords
}

func ExampleToolDiscovery() {
	// Create registry and discovery
	registry := tools.NewToolRegistry()
	discovery := tools.NewToolDiscovery(registry, nil)
	
	// Simulate a MESSAGES_SNAPSHOT event with tool calls
	messagesData := json.RawMessage(`[
		{
			"role": "assistant",
			"toolCalls": [
				{
					"id": "call-123",
					"type": "function",
					"function": {
						"name": "weather_lookup",
						"arguments": "{\"location\": \"San Francisco\", \"units\": \"celsius\"}"
					}
				}
			]
		}
	]`)
	
	// Discover tools from messages
	if err := discovery.DiscoverFromMessagesSnapshot(messagesData); err != nil {
		log.Fatal(err)
	}
	
	// Check if tool was discovered
	if registry.Has("weather_lookup") {
		fmt.Println("Tool 'weather_lookup' discovered successfully")
		
		// Get the tool
		if tool, exists := registry.Get("weather_lookup"); exists {
			// The schema was inferred from the arguments
			if tool.Parameters != nil && tool.Parameters.Properties != nil {
				fmt.Printf("Inferred %d parameters\n", len(tool.Parameters.Properties))
			}
		}
	}
	
	// Output:
	// Tool 'weather_lookup' discovered successfully
	// Inferred 2 parameters
}

func ExampleValidator() {
	// Create a schema
	schema := &tools.ToolSchema{
		Type: "object",
		Properties: map[string]*tools.Property{
			"query": {
				Type:        "string",
				Description: "Search query",
				MinLength:   intPtr(1),
			},
			"limit": {
				Type:        "integer",
				Description: "Maximum results",
				Minimum:     float64Ptr(1),
				Maximum:     float64Ptr(100),
				Default:     10,
			},
			"filters": {
				Type: "array",
				Items: &tools.Property{
					Type: "string",
				},
			},
		},
		Required: []string{"query"},
	}
	
	// Create validator
	validator := tools.NewValidator(schema)
	
	// Valid arguments
	validArgs := map[string]interface{}{
		"query":   "golang tools",
		"limit":   20,
		"filters": []interface{}{"recent", "popular"},
	}
	
	if err := validator.Validate(validArgs); err != nil {
		fmt.Printf("Validation error: %v\n", err)
	} else {
		fmt.Println("Arguments are valid")
	}
	
	// Invalid arguments (missing required field)
	invalidArgs := map[string]interface{}{
		"limit": 50,
	}
	
	if err := validator.Validate(invalidArgs); err != nil {
		fmt.Printf("Validation error: %v\n", err)
	}
	
	// Output:
	// Arguments are valid
	// Validation error: validation error for field 'query': required field is missing
}

// Helper functions for examples
func intPtr(i int) *int {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}