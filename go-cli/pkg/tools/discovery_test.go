package tools

import (
	"encoding/json"
	"log"
	"os"
	"testing"
)

func TestToolDiscovery(t *testing.T) {
	t.Run("DiscoverFromMessagesSnapshot", func(t *testing.T) {
		registry := NewToolRegistry()
		logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
		discovery := NewToolDiscovery(registry, logger)
		
		// Create a sample MESSAGES_SNAPSHOT data
		messagesData := json.RawMessage(`[
			{
				"id": "msg-1",
				"role": "user",
				"content": "Hello"
			},
			{
				"id": "msg-2",
				"role": "assistant",
				"toolCalls": [
					{
						"id": "call-1",
						"type": "function",
						"function": {
							"name": "generate_haiku",
							"arguments": "{\"japanese\": [\"エーアイの\", \"橋つなぐ道\", \"コパキット\"], \"english\": [\"From AI's realm\", \"A bridge-road linking us—\", \"CopilotKit.\"]}"
						}
					}
				]
			},
			{
				"id": "msg-3",
				"role": "tool",
				"content": "Haiku generated",
				"toolCallId": "call-1"
			}
		]`)
		
		err := discovery.DiscoverFromMessagesSnapshot(messagesData)
		if err != nil {
			t.Fatalf("DiscoverFromMessagesSnapshot() failed: %v", err)
		}
		
		// Check if tool was discovered
		if !registry.Has("generate_haiku") {
			t.Error("Tool 'generate_haiku' was not discovered")
		}
		
		// Get the discovered tool
		tool, exists := registry.Get("generate_haiku")
		if !exists {
			t.Fatal("Tool not found after discovery")
		}
		
		if tool.Name != "generate_haiku" {
			t.Errorf("Tool name = %v, want generate_haiku", tool.Name)
		}
		
		// Check inferred schema
		if tool.Parameters == nil {
			t.Error("Expected schema to be inferred from arguments")
		} else {
			// Check that properties were inferred
			if len(tool.Parameters.Properties) != 2 {
				t.Errorf("Expected 2 properties, got %d", len(tool.Parameters.Properties))
			}
			
			// Check specific properties
			if _, exists := tool.Parameters.Properties["japanese"]; !exists {
				t.Error("Property 'japanese' not found in inferred schema")
			}
			if _, exists := tool.Parameters.Properties["english"]; !exists {
				t.Error("Property 'english' not found in inferred schema")
			}
		}
		
		// Check source tracking
		source, exists := discovery.GetToolSource("generate_haiku")
		if !exists {
			t.Error("Tool source not tracked")
		}
		if source != SourceMessagesSnapshot {
			t.Errorf("Tool source = %v, want %v", source, SourceMessagesSnapshot)
		}
	})
	
	t.Run("DiscoverFromToolDefinitions", func(t *testing.T) {
		registry := NewToolRegistry()
		discovery := NewToolDiscovery(registry, nil)
		
		toolsData := json.RawMessage(`[
			{
				"name": "calculator",
				"description": "Performs basic calculations",
				"parameters": {
					"type": "object",
					"properties": {
						"operation": {"type": "string", "enum": ["add", "subtract", "multiply", "divide"]},
						"a": {"type": "number"},
						"b": {"type": "number"}
					},
					"required": ["operation", "a", "b"]
				}
			},
			{
				"name": "weather",
				"description": "Gets weather information",
				"parameters": {
					"type": "object",
					"properties": {
						"location": {"type": "string"}
					},
					"required": ["location"]
				}
			}
		]`)
		
		err := discovery.DiscoverFromToolDefinitions(toolsData)
		if err != nil {
			t.Fatalf("DiscoverFromToolDefinitions() failed: %v", err)
		}
		
		// Check both tools were registered
		if !registry.Has("calculator") {
			t.Error("Tool 'calculator' not registered")
		}
		if !registry.Has("weather") {
			t.Error("Tool 'weather' not registered")
		}
		
		// Check tool details
		calc, _ := registry.Get("calculator")
		if calc.Description != "Performs basic calculations" {
			t.Errorf("Calculator description = %v", calc.Description)
		}
		
		// Check source tracking
		source, _ := discovery.GetToolSource("calculator")
		if source != SourceToolDefinition {
			t.Errorf("Tool source = %v, want %v", source, SourceToolDefinition)
		}
	})
	
	t.Run("DiscoverFromConfig", func(t *testing.T) {
		registry := NewToolRegistry()
		discovery := NewToolDiscovery(registry, nil)
		
		// Test with direct tool list
		tools := []*Tool{
			{
				Name:        "config_tool_1",
				Description: "First config tool",
			},
			{
				Name:        "config_tool_2",
				Description: "Second config tool",
			},
		}
		
		err := discovery.DiscoverFromConfig(tools)
		if err != nil {
			t.Fatalf("DiscoverFromConfig() with tool list failed: %v", err)
		}
		
		if !registry.Has("config_tool_1") || !registry.Has("config_tool_2") {
			t.Error("Config tools not registered")
		}
		
		// Test with JSON data
		discovery.Clear()
		jsonData := []byte(`[
			{"name": "json_tool", "description": "Tool from JSON"}
		]`)
		
		err = discovery.DiscoverFromConfig(jsonData)
		if err != nil {
			t.Fatalf("DiscoverFromConfig() with JSON failed: %v", err)
		}
		
		if !registry.Has("json_tool") {
			t.Error("JSON tool not registered")
		}
		
		// Test with config map
		discovery.Clear()
		configMap := map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{
					"name":        "map_tool",
					"description": "Tool from map",
				},
			},
		}
		
		err = discovery.DiscoverFromConfig(configMap)
		if err != nil {
			t.Fatalf("DiscoverFromConfig() with map failed: %v", err)
		}
		
		if !registry.Has("map_tool") {
			t.Error("Map tool not registered")
		}
	})
	
	t.Run("InferSchemaFromArguments", func(t *testing.T) {
		discovery := NewToolDiscovery(nil, nil)
		
		argsJSON := `{
			"text": "hello",
			"count": 42,
			"price": 19.99,
			"enabled": true,
			"tags": ["a", "b", "c"],
			"config": {"host": "localhost", "port": 8080},
			"nullable": null
		}`
		
		schema := discovery.inferSchemaFromArguments(argsJSON)
		
		if schema == nil {
			t.Fatal("Schema inference returned nil")
		}
		
		if schema.Type != "object" {
			t.Errorf("Schema type = %v, want object", schema.Type)
		}
		
		// Check inferred types
		tests := []struct {
			prop     string
			wantType string
		}{
			{"text", "string"},
			{"count", "integer"},
			{"price", "number"},
			{"enabled", "boolean"},
			{"tags", "array"},
			{"config", "object"},
			{"nullable", "null"},
		}
		
		for _, tt := range tests {
			prop, exists := schema.Properties[tt.prop]
			if !exists {
				t.Errorf("Property %s not found", tt.prop)
				continue
			}
			if prop.Type != tt.wantType {
				t.Errorf("Property %s type = %v, want %v", tt.prop, prop.Type, tt.wantType)
			}
		}
		
		// Check array items inference
		if tagsProp, exists := schema.Properties["tags"]; exists {
			if tagsProp.Items == nil {
				t.Error("Array items not inferred")
			} else if tagsProp.Items.Type != "string" {
				t.Errorf("Array items type = %v, want string", tagsProp.Items.Type)
			}
		}
	})
	
	t.Run("GetToolsBySource", func(t *testing.T) {
		registry := NewToolRegistry()
		discovery := NewToolDiscovery(registry, nil)
		
		// Register tools from different sources
		discovery.registry.Register(&Tool{Name: "msg_tool_1"})
		discovery.sources["msg_tool_1"] = SourceMessagesSnapshot
		
		discovery.registry.Register(&Tool{Name: "msg_tool_2"})
		discovery.sources["msg_tool_2"] = SourceMessagesSnapshot
		
		discovery.registry.Register(&Tool{Name: "config_tool"})
		discovery.sources["config_tool"] = SourceConfig
		
		// Get tools by source
		msgTools := discovery.GetToolsBySource(SourceMessagesSnapshot)
		if len(msgTools) != 2 {
			t.Errorf("GetToolsBySource(MessagesSnapshot) = %d tools, want 2", len(msgTools))
		}
		
		configTools := discovery.GetToolsBySource(SourceConfig)
		if len(configTools) != 1 {
			t.Errorf("GetToolsBySource(Config) = %d tools, want 1", len(configTools))
		}
		
		cliTools := discovery.GetToolsBySource(SourceCLI)
		if len(cliTools) != 0 {
			t.Errorf("GetToolsBySource(CLI) = %d tools, want 0", len(cliTools))
		}
	})
	
	t.Run("GetStats", func(t *testing.T) {
		registry := NewToolRegistry()
		discovery := NewToolDiscovery(registry, nil)
		
		// Register tools from different sources
		tools := []struct {
			name   string
			source DiscoverySource
		}{
			{"tool1", SourceMessagesSnapshot},
			{"tool2", SourceMessagesSnapshot},
			{"tool3", SourceConfig},
			{"tool4", SourceToolDefinition},
		}
		
		for _, t := range tools {
			discovery.registry.Register(&Tool{Name: t.name})
			discovery.sources[t.name] = t.source
		}
		
		stats := discovery.GetStats()
		
		if stats.TotalTools != 4 {
			t.Errorf("Stats.TotalTools = %d, want 4", stats.TotalTools)
		}
		
		if len(stats.ToolNames) != 4 {
			t.Errorf("Stats.ToolNames has %d items, want 4", len(stats.ToolNames))
		}
		
		// Check source counts
		if stats.BySources[SourceMessagesSnapshot] != 2 {
			t.Errorf("Stats.BySources[MessagesSnapshot] = %d, want 2", stats.BySources[SourceMessagesSnapshot])
		}
		if stats.BySources[SourceConfig] != 1 {
			t.Errorf("Stats.BySources[Config] = %d, want 1", stats.BySources[SourceConfig])
		}
		if stats.BySources[SourceToolDefinition] != 1 {
			t.Errorf("Stats.BySources[ToolDefinition] = %d, want 1", stats.BySources[SourceToolDefinition])
		}
	})
	
	t.Run("Clear", func(t *testing.T) {
		registry := NewToolRegistry()
		discovery := NewToolDiscovery(registry, nil)
		
		// Add some tools
		discovery.registry.Register(&Tool{Name: "tool1"})
		discovery.sources["tool1"] = SourceConfig
		discovery.registry.Register(&Tool{Name: "tool2"})
		discovery.sources["tool2"] = SourceCLI
		
		if registry.Count() != 2 {
			t.Errorf("Registry has %d tools before clear, want 2", registry.Count())
		}
		
		// Clear
		discovery.Clear()
		
		if registry.Count() != 0 {
			t.Errorf("Registry has %d tools after clear, want 0", registry.Count())
		}
		
		if len(discovery.sources) != 0 {
			t.Errorf("Sources has %d entries after clear, want 0", len(discovery.sources))
		}
	})
	
	t.Run("HandleInvalidJSON", func(t *testing.T) {
		registry := NewToolRegistry()
		discovery := NewToolDiscovery(registry, nil)
		
		// Invalid messages JSON
		err := discovery.DiscoverFromMessagesSnapshot(json.RawMessage(`{not valid json`))
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
		
		// Invalid tool definitions JSON
		err = discovery.DiscoverFromToolDefinitions(json.RawMessage(`{not valid json`))
		if err == nil {
			t.Error("Expected error for invalid tool definitions JSON")
		}
	})
	
	t.Run("SkipNonFunctionToolCalls", func(t *testing.T) {
		registry := NewToolRegistry()
		discovery := NewToolDiscovery(registry, nil)
		
		messagesData := json.RawMessage(`[
			{
				"role": "assistant",
				"toolCalls": [
					{
						"id": "call-1",
						"type": "not-function",
						"function": {
							"name": "should_not_register"
						}
					}
				]
			}
		]`)
		
		err := discovery.DiscoverFromMessagesSnapshot(messagesData)
		if err != nil {
			t.Fatalf("DiscoverFromMessagesSnapshot() failed: %v", err)
		}
		
		if registry.Has("should_not_register") {
			t.Error("Non-function tool call should not be registered")
		}
	})
}