package tools

import (
	"encoding/json"
	"fmt"
	"log"
)

// DiscoverySource represents where tools can be discovered from
type DiscoverySource string

const (
	// SourceMessagesSnapshot indicates tools discovered from MESSAGES_SNAPSHOT events
	SourceMessagesSnapshot DiscoverySource = "messages_snapshot"
	
	// SourceToolDefinition indicates tools explicitly defined in a TOOL_DEFINITION event
	SourceToolDefinition DiscoverySource = "tool_definition"
	
	// SourceConfig indicates tools loaded from configuration
	SourceConfig DiscoverySource = "config"
	
	// SourceCLI indicates tools provided via CLI arguments
	SourceCLI DiscoverySource = "cli"
)

// ToolDiscovery handles discovering tools from various sources
type ToolDiscovery struct {
	registry *ToolRegistry
	logger   *log.Logger
	
	// Track discovered tools by source
	sources map[string]DiscoverySource
}

// NewToolDiscovery creates a new tool discovery instance
func NewToolDiscovery(registry *ToolRegistry, logger *log.Logger) *ToolDiscovery {
	if registry == nil {
		registry = NewToolRegistry()
	}
	
	return &ToolDiscovery{
		registry: registry,
		logger:   logger,
		sources:  make(map[string]DiscoverySource),
	}
}

// GetRegistry returns the underlying tool registry
func (d *ToolDiscovery) GetRegistry() *ToolRegistry {
	return d.registry
}

// DiscoverFromMessagesSnapshot discovers tools from a MESSAGES_SNAPSHOT event
// This parses assistant messages looking for tool calls
func (d *ToolDiscovery) DiscoverFromMessagesSnapshot(messagesData json.RawMessage) error {
	// Parse the messages array
	var messages []json.RawMessage
	if err := json.Unmarshal(messagesData, &messages); err != nil {
		return fmt.Errorf("failed to parse messages array: %w", err)
	}
	
	toolsFound := 0
	
	// Iterate through messages looking for assistant messages with tool calls
	for i, msgData := range messages {
		// Parse message to check role
		var msgBase struct {
			Role      string          `json:"role"`
			ToolCalls []ToolCall      `json:"toolCalls,omitempty"`
		}
		
		if err := json.Unmarshal(msgData, &msgBase); err != nil {
			d.logDebug("Failed to parse message %d: %v", i, err)
			continue
		}
		
		// Only process assistant messages with tool calls
		if msgBase.Role != "assistant" || len(msgBase.ToolCalls) == 0 {
			continue
		}
		
		// Extract tool information from tool calls
		for _, toolCall := range msgBase.ToolCalls {
			if toolCall.Type != "function" {
				continue
			}
			
			toolName := toolCall.Function.Name
			if toolName == "" {
				continue
			}
			
			// Check if we already have this tool
			if d.registry.Has(toolName) {
				d.logDebug("Tool '%s' already registered", toolName)
				continue
			}
			
			// Create a basic tool definition from the tool call
			// Note: We don't have the full schema from just the tool call,
			// so we create a minimal definition that can be enhanced later
			tool := &Tool{
				Name:        toolName,
				Description: fmt.Sprintf("Tool discovered from tool call %s", toolCall.ID),
				Parameters:  nil, // Schema unknown from tool call alone
			}
			
			// Try to infer parameters from the arguments if present
			if toolCall.Function.Arguments != "" {
				tool.Parameters = d.inferSchemaFromArguments(toolCall.Function.Arguments)
			}
			
			// Register the tool
			if err := d.registry.Register(tool); err != nil {
				d.logDebug("Failed to register tool '%s': %v", toolName, err)
				continue
			}
			
			d.sources[toolName] = SourceMessagesSnapshot
			toolsFound++
			d.logInfo("Discovered tool '%s' from messages snapshot", toolName)
		}
	}
	
	if toolsFound > 0 {
		d.logInfo("Discovered %d tools from messages snapshot", toolsFound)
	}
	
	return nil
}

// DiscoverFromToolDefinitions discovers tools from explicit tool definitions
// This would come from a TOOL_DEFINITION event or similar
func (d *ToolDiscovery) DiscoverFromToolDefinitions(toolsData json.RawMessage) error {
	var tools []*Tool
	if err := json.Unmarshal(toolsData, &tools); err != nil {
		return fmt.Errorf("failed to parse tool definitions: %w", err)
	}
	
	registered := 0
	for _, tool := range tools {
		if err := d.registry.Register(tool); err != nil {
			d.logDebug("Failed to register tool '%s': %v", tool.Name, err)
			continue
		}
		d.sources[tool.Name] = SourceToolDefinition
		registered++
		d.logInfo("Registered tool '%s' from definition", tool.Name)
	}
	
	d.logInfo("Registered %d tools from definitions", registered)
	return nil
}

// DiscoverFromConfig loads tools from a configuration file or object
func (d *ToolDiscovery) DiscoverFromConfig(config interface{}) error {
	// Handle different config formats
	switch cfg := config.(type) {
	case string:
		// Assume it's a file path
		return d.discoverFromConfigFile(cfg)
	case []byte:
		// Raw JSON data
		return d.discoverFromConfigData(cfg)
	case []*Tool:
		// Direct tool list
		return d.registerTools(cfg, SourceConfig)
	case map[string]interface{}:
		// Configuration object
		return d.discoverFromConfigMap(cfg)
	default:
		return fmt.Errorf("unsupported config type: %T", config)
	}
}

// discoverFromConfigFile loads tools from a configuration file
func (d *ToolDiscovery) discoverFromConfigFile(filepath string) error {
	// This would read the file and parse it
	// For now, returning an error as file operations should be handled by the caller
	return fmt.Errorf("file-based discovery not implemented: use DiscoverFromConfig with []byte instead")
}

// discoverFromConfigData loads tools from JSON data
func (d *ToolDiscovery) discoverFromConfigData(data []byte) error {
	var tools []*Tool
	if err := json.Unmarshal(data, &tools); err != nil {
		return fmt.Errorf("failed to parse config data: %w", err)
	}
	return d.registerTools(tools, SourceConfig)
}

// discoverFromConfigMap loads tools from a configuration map
func (d *ToolDiscovery) discoverFromConfigMap(config map[string]interface{}) error {
	// Look for a "tools" key in the config
	if toolsRaw, exists := config["tools"]; exists {
		// Marshal and unmarshal to convert to []*Tool
		data, err := json.Marshal(toolsRaw)
		if err != nil {
			return fmt.Errorf("failed to marshal tools config: %w", err)
		}
		
		var tools []*Tool
		if err := json.Unmarshal(data, &tools); err != nil {
			return fmt.Errorf("failed to parse tools config: %w", err)
		}
		
		return d.registerTools(tools, SourceConfig)
	}
	
	return fmt.Errorf("no tools found in config")
}

// registerTools registers multiple tools with a given source
func (d *ToolDiscovery) registerTools(tools []*Tool, source DiscoverySource) error {
	registered := 0
	for _, tool := range tools {
		if err := d.registry.Register(tool); err != nil {
			d.logDebug("Failed to register tool '%s': %v", tool.Name, err)
			continue
		}
		d.sources[tool.Name] = source
		registered++
	}
	
	d.logInfo("Registered %d tools from %s", registered, source)
	return nil
}

// inferSchemaFromArguments attempts to infer a schema from example arguments
func (d *ToolDiscovery) inferSchemaFromArguments(argsJSON string) *ToolSchema {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil
	}
	
	schema := &ToolSchema{
		Type:       "object",
		Properties: make(map[string]*Property),
	}
	
	// Infer property types from the argument values
	for key, value := range args {
		prop := &Property{}
		
		switch v := value.(type) {
		case string:
			prop.Type = "string"
		case float64:
			// Check if it's an integer
			if v == float64(int64(v)) {
				prop.Type = "integer"
			} else {
				prop.Type = "number"
			}
		case bool:
			prop.Type = "boolean"
		case []interface{}:
			prop.Type = "array"
			// Try to infer item type from first element
			if len(v) > 0 {
				prop.Items = d.inferPropertyFromValue(v[0])
			}
		case map[string]interface{}:
			prop.Type = "object"
		case nil:
			prop.Type = "null"
		default:
			// Unknown type, leave it unspecified
		}
		
		schema.Properties[key] = prop
	}
	
	return schema
}

// inferPropertyFromValue infers a property schema from a value
func (d *ToolDiscovery) inferPropertyFromValue(value interface{}) *Property {
	prop := &Property{}
	
	switch v := value.(type) {
	case string:
		prop.Type = "string"
	case float64:
		if v == float64(int64(v)) {
			prop.Type = "integer"
		} else {
			prop.Type = "number"
		}
	case bool:
		prop.Type = "boolean"
	case []interface{}:
		prop.Type = "array"
	case map[string]interface{}:
		prop.Type = "object"
	case nil:
		prop.Type = "null"
	}
	
	return prop
}

// GetToolSource returns the discovery source for a tool
func (d *ToolDiscovery) GetToolSource(toolName string) (DiscoverySource, bool) {
	source, exists := d.sources[toolName]
	return source, exists
}

// GetToolsBySource returns all tools discovered from a specific source
func (d *ToolDiscovery) GetToolsBySource(source DiscoverySource) []string {
	var tools []string
	for name, src := range d.sources {
		if src == source {
			tools = append(tools, name)
		}
	}
	return tools
}

// Clear clears all discovered tools
func (d *ToolDiscovery) Clear() {
	d.registry.Clear()
	d.sources = make(map[string]DiscoverySource)
}

// Logging helpers

func (d *ToolDiscovery) logInfo(format string, args ...interface{}) {
	if d.logger != nil {
		d.logger.Printf("[INFO] "+format, args...)
	}
}

func (d *ToolDiscovery) logDebug(format string, args ...interface{}) {
	if d.logger != nil {
		d.logger.Printf("[DEBUG] "+format, args...)
	}
}

// DiscoveryStats provides statistics about discovered tools
type DiscoveryStats struct {
	TotalTools int                        `json:"total_tools"`
	BySources  map[DiscoverySource]int    `json:"by_sources"`
	ToolNames  []string                   `json:"tool_names"`
}

// GetStats returns statistics about discovered tools
func (d *ToolDiscovery) GetStats() *DiscoveryStats {
	stats := &DiscoveryStats{
		TotalTools: d.registry.Count(),
		BySources:  make(map[DiscoverySource]int),
		ToolNames:  d.registry.ListNames(),
	}
	
	// Count tools by source
	for _, source := range d.sources {
		stats.BySources[source]++
	}
	
	return stats
}