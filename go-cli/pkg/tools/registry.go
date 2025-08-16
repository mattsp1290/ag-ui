package tools

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

// ToolRegistry manages available tools discovered from server messages
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]*Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*Tool),
	}
}

// Register adds or updates a tool in the registry
func (r *ToolRegistry) Register(tool *Tool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}
	
	if err := tool.Validate(); err != nil {
		return fmt.Errorf("invalid tool: %w", err)
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Clone the tool to avoid external modifications
	r.tools[tool.Name] = tool.Clone()
	
	return nil
}

// RegisterMultiple registers multiple tools at once
func (r *ToolRegistry) RegisterMultiple(tools []*Tool) error {
	for _, tool := range tools {
		if err := r.Register(tool); err != nil {
			return fmt.Errorf("failed to register tool '%s': %w", tool.Name, err)
		}
	}
	return nil
}

// Get retrieves a tool by name
func (r *ToolRegistry) Get(name string) (*Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	tool, exists := r.tools[name]
	if !exists {
		return nil, false
	}
	
	// Return a clone to prevent external modifications
	return tool.Clone(), true
}

// List returns all registered tools
func (r *ToolRegistry) List() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	tools := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool.Clone())
	}
	
	// Sort by name for consistent output
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	
	return tools
}

// ListNames returns the names of all registered tools
func (r *ToolRegistry) ListNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	
	sort.Strings(names)
	return names
}

// Has checks if a tool is registered
func (r *ToolRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	_, exists := r.tools[name]
	return exists
}

// ValidateArgs validates arguments against a tool's schema
func (r *ToolRegistry) ValidateArgs(toolName string, args json.RawMessage) error {
	tool, exists := r.Get(toolName)
	if !exists {
		return fmt.Errorf("tool '%s' not found", toolName)
	}
	
	// If no parameters schema, any arguments are valid
	if tool.Parameters == nil {
		return nil
	}
	
	// Parse arguments
	var argsMap map[string]interface{}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &argsMap); err != nil {
			return fmt.Errorf("invalid JSON arguments: %w", err)
		}
	}
	
	validator := NewValidator(tool.Parameters)
	return validator.Validate(argsMap)
}

// Remove removes a tool from the registry
func (r *ToolRegistry) Remove(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.tools[name]; exists {
		delete(r.tools, name)
		return true
	}
	return false
}

// Clear removes all tools from the registry
func (r *ToolRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.tools = make(map[string]*Tool)
}

// Count returns the number of registered tools
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	return len(r.tools)
}

// ToJSON exports all tools as JSON
func (r *ToolRegistry) ToJSON() ([]byte, error) {
	tools := r.List()
	return json.MarshalIndent(tools, "", "  ")
}

// FromJSON imports tools from JSON
func (r *ToolRegistry) FromJSON(data []byte) error {
	var tools []*Tool
	if err := json.Unmarshal(data, &tools); err != nil {
		return fmt.Errorf("failed to parse tools JSON: %w", err)
	}
	
	return r.RegisterMultiple(tools)
}

// ToolSummary provides a brief summary of a tool
type ToolSummary struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Parameters   []string `json:"parameters,omitempty"`
	Required     []string `json:"required,omitempty"`
}

// GetSummary returns a summary of a tool
func (r *ToolRegistry) GetSummary(name string) (*ToolSummary, error) {
	tool, exists := r.Get(name)
	if !exists {
		return nil, fmt.Errorf("tool '%s' not found", name)
	}
	
	summary := &ToolSummary{
		Name:        tool.Name,
		Description: tool.Description,
	}
	
	if tool.Parameters != nil && tool.Parameters.Properties != nil {
		for paramName := range tool.Parameters.Properties {
			summary.Parameters = append(summary.Parameters, paramName)
		}
		sort.Strings(summary.Parameters)
		
		if len(tool.Parameters.Required) > 0 {
			summary.Required = make([]string, len(tool.Parameters.Required))
			copy(summary.Required, tool.Parameters.Required)
			sort.Strings(summary.Required)
		}
	}
	
	return summary, nil
}

// ListSummaries returns summaries of all tools
func (r *ToolRegistry) ListSummaries() []*ToolSummary {
	tools := r.List()
	summaries := make([]*ToolSummary, 0, len(tools))
	
	for _, tool := range tools {
		summary, _ := r.GetSummary(tool.Name)
		if summary != nil {
			summaries = append(summaries, summary)
		}
	}
	
	return summaries
}

// RegistrySnapshot represents a point-in-time snapshot of the registry
type RegistrySnapshot struct {
	Timestamp string   `json:"timestamp"`
	Tools     []*Tool  `json:"tools"`
	Count     int      `json:"count"`
}

// Snapshot creates a snapshot of the current registry state
func (r *ToolRegistry) Snapshot() *RegistrySnapshot {
	return &RegistrySnapshot{
		Timestamp: fmt.Sprintf("%d", timeNow()),
		Tools:     r.List(),
		Count:     r.Count(),
	}
}

// Restore restores the registry from a snapshot
func (r *ToolRegistry) Restore(snapshot *RegistrySnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot cannot be nil")
	}
	
	r.Clear()
	return r.RegisterMultiple(snapshot.Tools)
}

// timeNow is a variable to allow mocking in tests
var timeNow = func() int64 {
	return 0 // Should be overridden with actual time.Now().Unix() in production
}