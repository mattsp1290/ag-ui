package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Tool represents a function that can be called by an AI agent.
// It includes metadata, parameter schema, and execution logic.
//
// A tool encapsulates:
//   - Identity: Unique ID, name, and version
//   - Documentation: Description and usage examples
//   - Schema: JSON Schema defining expected parameters
//   - Executor: The actual implementation logic
//   - Capabilities: Feature support like streaming or async execution
//   - Metadata: Additional information like author, tags, and dependencies
//
// Example tool definition:
//
//	tool := &Tool{
//		ID:          "text-analyzer",
//		Name:        "Text Analyzer",
//		Description: "Analyzes text for sentiment, keywords, and statistics",
//		Version:     "1.0.0",
//		Schema: &ToolSchema{
//			Type: "object",
//			Properties: map[string]*Property{
//				"text": {
//					Type:        "string",
//					Description: "Text to analyze",
//					MinLength:   intPtr(1),
//					MaxLength:   intPtr(10000),
//				},
//			},
//			Required: []string{"text"},
//		},
//		Executor: &TextAnalyzerExecutor{},
//		Capabilities: &ToolCapabilities{
//			Cacheable: true,
//			Timeout:   30 * time.Second,
//		},
//	}
type Tool struct {
	// ID is the unique identifier for the tool
	ID string `json:"id"`

	// Name is the human-readable name of the tool
	Name string `json:"name"`

	// Description explains what the tool does
	Description string `json:"description"`

	// Version follows semantic versioning (e.g., "1.0.0")
	Version string `json:"version"`

	// Schema defines the JSON Schema for tool parameters
	Schema *ToolSchema `json:"schema"`

	// Metadata contains additional tool information
	Metadata *ToolMetadata `json:"metadata,omitempty"`

	// Executor implements the tool's execution logic
	Executor ToolExecutor `json:"-"`

	// Capabilities defines what features this tool supports
	Capabilities *ToolCapabilities `json:"capabilities,omitempty"`

	// Copy-on-write optimization fields
	refCount int32        `json:"-"`
	isShared bool         `json:"-"`
	mu       sync.RWMutex `json:"-"`
}

// ReadOnlyTool provides a read-only view of a tool to avoid cloning overhead.
// This interface prevents modifications while allowing access to tool properties.
//
// ReadOnlyTool is used by the execution engine and registry for:
//   - Memory efficiency (no cloning for read operations)
//   - Thread safety (prevents concurrent modifications)
//   - API clarity (explicit read-only semantics)
//
// If modification is needed, use the Clone() method to get a mutable copy.
type ReadOnlyTool interface {
	GetID() string
	GetName() string
	GetDescription() string
	GetVersion() string
	GetSchema() *ToolSchema
	GetMetadata() *ToolMetadata
	GetExecutor() ToolExecutor
	GetCapabilities() *ToolCapabilities
	// Clone returns a full copy if modification is needed
	Clone() *Tool
}

// readOnlyToolView implements ReadOnlyTool by wrapping a Tool pointer.
type readOnlyToolView struct {
	tool *Tool
}

// NewReadOnlyTool creates a read-only view of a tool without cloning.
// The returned view shares the underlying tool data, so the original
// tool should not be modified while the view is in use.
func NewReadOnlyTool(tool *Tool) ReadOnlyTool {
	return &readOnlyToolView{tool: tool}
}

func (r *readOnlyToolView) GetID() string {
	return r.tool.ID
}

func (r *readOnlyToolView) GetName() string {
	return r.tool.Name
}

func (r *readOnlyToolView) GetDescription() string {
	return r.tool.Description
}

func (r *readOnlyToolView) GetVersion() string {
	return r.tool.Version
}

func (r *readOnlyToolView) GetSchema() *ToolSchema {
	return r.tool.Schema
}

func (r *readOnlyToolView) GetMetadata() *ToolMetadata {
	return r.tool.Metadata
}

func (r *readOnlyToolView) GetExecutor() ToolExecutor {
	return r.tool.Executor
}

func (r *readOnlyToolView) GetCapabilities() *ToolCapabilities {
	return r.tool.Capabilities
}

func (r *readOnlyToolView) Clone() *Tool {
	return r.tool.Clone()
}

// ToolSchema represents a JSON Schema for tool parameters.
// It validates and describes the expected input structure.
//
// The schema follows JSON Schema draft-07 with common patterns:
//   - Type: Usually "object" for tool parameters
//   - Properties: Defines individual parameters with their types and constraints
//   - Required: Lists mandatory parameters
//   - AdditionalProperties: Controls whether extra parameters are allowed
//
// Example schema:
//
//	schema := &ToolSchema{
//		Type: "object",
//		Properties: map[string]*Property{
//			"query": {
//				Type:        "string",
//				Description: "Search query",
//				MinLength:   intPtr(1),
//			},
//			"limit": {
//				Type:        "integer",
//				Description: "Maximum results",
//				Minimum:     float64Ptr(1),
//				Maximum:     float64Ptr(100),
//				Default:     10,
//			},
//		},
//		Required: []string{"query"},
//	}
type ToolSchema struct {
	// Type is typically "object" for tool parameters
	Type string `json:"type"`

	// Properties defines the individual parameters
	Properties map[string]*Property `json:"properties,omitempty"`

	// Required lists the mandatory parameter names
	Required []string `json:"required,omitempty"`

	// AdditionalProperties controls whether extra parameters are allowed
	AdditionalProperties *bool `json:"additionalProperties,omitempty"`

	// Description provides schema-level documentation
	Description string `json:"description,omitempty"`
}

// Property represents a single parameter in the tool schema.
// It supports all JSON Schema features including:
//   - Basic types: string, number, integer, boolean, array, object, null
//   - Validation: format, pattern, enum, min/max constraints
//   - Composition: oneOf, anyOf, allOf, not
//   - Conditional: if-then-else patterns
//   - References: $ref for schema reuse
//
// Example property definitions:
//
//	// Email with format validation
//	emailProp := &Property{
//		Type:        "string",
//		Format:      "email",
//		Description: "User email address",
//	}
//
//	// Enum with specific values
//	statusProp := &Property{
//		Type:        "string",
//		Enum:        []interface{}{"active", "inactive", "pending"},
//		Description: "Account status",
//	}
//
//	// Array with item constraints
//	tagsProp := &Property{
//		Type:        "array",
//		Items:       &Property{Type: "string", Pattern: "^[a-z]+$"},
//		MinItems:    intPtr(1),
//		MaxItems:    intPtr(10),
//		UniqueItems: boolPtr(true),
//	}
type Property struct {
	// Type defines the JSON type (string, number, integer, boolean, array, object)
	Type string `json:"type,omitempty"`

	// Description explains the parameter's purpose
	Description string `json:"description,omitempty"`

	// Format provides additional type constraints (e.g., "email", "uri", "date-time")
	Format string `json:"format,omitempty"`

	// Enum restricts the value to a set of allowed options
	Enum []interface{} `json:"enum,omitempty"`

	// Default provides a default value if the parameter is not supplied
	Default interface{} `json:"default,omitempty"`

	// Minimum/Maximum for numeric types
	Minimum *float64 `json:"minimum,omitempty"`
	Maximum *float64 `json:"maximum,omitempty"`

	// MinLength/MaxLength for string types
	MinLength *int `json:"minLength,omitempty"`
	MaxLength *int `json:"maxLength,omitempty"`

	// Pattern for regex validation of strings
	Pattern string `json:"pattern,omitempty"`

	// Items defines the schema for array elements
	Items *Property `json:"items,omitempty"`

	// Properties for nested objects
	Properties map[string]*Property `json:"properties,omitempty"`

	// Required properties for nested objects
	Required []string `json:"required,omitempty"`

	// Advanced JSON Schema features

	// OneOf specifies that the value must match exactly one of the schemas
	OneOf []*Property `json:"oneOf,omitempty"`

	// AnyOf specifies that the value must match at least one of the schemas
	AnyOf []*Property `json:"anyOf,omitempty"`

	// AllOf specifies that the value must match all of the schemas
	AllOf []*Property `json:"allOf,omitempty"`

	// Not specifies that the value must not match the schema
	Not *Property `json:"not,omitempty"`

	// Conditional validation
	If   *Property `json:"if,omitempty"`
	Then *Property `json:"then,omitempty"`
	Else *Property `json:"else,omitempty"`

	// Schema reference
	Ref string `json:"$ref,omitempty"`

	// Additional constraints
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
	MultipleOf       *float64 `json:"multipleOf,omitempty"`

	// Object-specific constraints
	MinProperties        *int  `json:"minProperties,omitempty"`
	MaxProperties        *int  `json:"maxProperties,omitempty"`
	AdditionalProperties *bool `json:"additionalProperties,omitempty"`

	// Array-specific constraints
	MinItems    *int  `json:"minItems,omitempty"`
	MaxItems    *int  `json:"maxItems,omitempty"`
	UniqueItems *bool `json:"uniqueItems,omitempty"`

	// String-specific constraints
	ContentMediaType *string `json:"contentMediaType,omitempty"`
	ContentEncoding  *string `json:"contentEncoding,omitempty"`

	// Metadata
	Title     string        `json:"title,omitempty"`
	Examples  []interface{} `json:"examples,omitempty"`
	ReadOnly  *bool         `json:"readOnly,omitempty"`
	WriteOnly *bool         `json:"writeOnly,omitempty"`

	// Custom extensions
	Extensions map[string]interface{} `json:"-"`
}

// ToolMetadata contains additional information about a tool.
// This includes documentation, categorization, and dependency information
// that helps with tool discovery, organization, and management.
//
// Example metadata:
//
//	metadata := &ToolMetadata{
//		Author:        "ACME Corp",
//		License:       "MIT",
//		Documentation: "https://docs.example.com/tools/analyzer",
//		Tags:          []string{"nlp", "analysis", "text"},
//		Examples: []ToolExample{
//			{
//				Name:        "Basic sentiment analysis",
//				Description: "Analyze sentiment of customer feedback",
//				Input:       map[string]interface{}{"text": "Great product!"},
//				Output:      map[string]interface{}{"sentiment": "positive", "score": 0.95},
//			},
//		},
//	}
type ToolMetadata struct {
	// Author identifies who created the tool
	Author string `json:"author,omitempty"`

	// License specifies the tool's license
	License string `json:"license,omitempty"`

	// Documentation URL for detailed docs
	Documentation string `json:"documentation,omitempty"`

	// Examples of tool usage
	Examples []ToolExample `json:"examples,omitempty"`

	// Tags for categorization and discovery
	Tags []string `json:"tags,omitempty"`

	// Dependencies on other tools
	Dependencies []string `json:"dependencies,omitempty"`

	// Custom metadata fields
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// ToolExample shows how to use a tool.
// Examples help users understand the tool's purpose and correct usage.
// They can be used for documentation, testing, and AI model training.
type ToolExample struct {
	// Name identifies the example
	Name string `json:"name"`

	// Description explains what the example demonstrates
	Description string `json:"description"`

	// Input shows the parameters to pass
	Input map[string]interface{} `json:"input"`

	// Output shows the expected result
	Output interface{} `json:"output,omitempty"`
}

// ToolCapabilities defines what features a tool supports.
// These capabilities help the execution engine optimize tool execution
// and provide appropriate features to users.
//
// Capabilities include:
//   - Streaming: Tool can produce output incrementally
//   - Async: Tool supports background execution
//   - Cancelable: Tool execution can be interrupted
//   - Retryable: Tool can be safely retried on failure
//   - Cacheable: Tool results can be cached
//   - RateLimit: Maximum requests per minute
//   - Timeout: Maximum execution duration
type ToolCapabilities struct {
	// Streaming indicates if the tool supports streaming arguments/results
	Streaming bool `json:"streaming"`

	// Async indicates if the tool supports asynchronous execution
	Async bool `json:"async"`

	// Cancelable indicates if the tool execution can be canceled
	Cancelable bool `json:"cancelable"`

	// Retryable indicates if the tool supports automatic retry on failure
	Retryable bool `json:"retryable"`

	// Cacheable indicates if the tool results can be cached
	Cacheable bool `json:"cacheable"`

	// RateLimit defines requests per minute limit (0 = unlimited)
	RateLimit int `json:"rateLimit,omitempty"`

	// Timeout defines maximum execution time
	Timeout time.Duration `json:"timeout,omitempty"`
}

// ToolExecutor is the interface that tool implementations must satisfy.
// It defines the contract for tool execution logic.
//
// Implementation guidelines:
//   - Respect context cancellation for graceful shutdown
//   - Return appropriate errors with context
//   - Handle panics gracefully (the engine provides recovery)
//   - Validate critical invariants even though parameters are pre-validated
//
// Example implementation:
//
//	type MyToolExecutor struct{}
//
//	func (e *MyToolExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
//		// Check context
//		select {
//		case <-ctx.Done():
//			return nil, ctx.Err()
//		default:
//		}
//
//		// Execute tool logic
//		result := processData(params["input"].(string))
//
//		return &ToolExecutionResult{
//			Success: true,
//			Data:    result,
//		}, nil
//	}
type ToolExecutor interface {
	// Execute runs the tool with the given parameters.
	// The context can be used for cancellation and timeout.
	// Parameters are validated against the schema before execution.
	Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error)
}

// StreamingToolExecutor extends ToolExecutor with streaming capabilities.
// Tools that produce large outputs or real-time data should implement this interface.
//
// The ExecuteStream method returns a channel that emits chunks of output.
// The channel must be closed when execution completes or an error occurs.
//
// Example streaming implementation:
//
//	func (e *LogReaderExecutor) ExecuteStream(ctx context.Context, params map[string]interface{}) (<-chan *ToolStreamChunk, error) {
//		stream := make(chan *ToolStreamChunk)
//
//		go func() {
//			defer close(stream)
//
//			file := params["file"].(string)
//			lines := readFileByLine(file)
//
//			for i, line := range lines {
//				select {
//				case <-ctx.Done():
//					return
//				case stream <- &ToolStreamChunk{
//					Type:  "data",
//					Data:  line,
//					Index: i,
//					Timestamp: time.Now(),
//				}:
//				}
//			}
//		}()
//
//		return stream, nil
//	}
type StreamingToolExecutor interface {
	ToolExecutor

	// ExecuteStream runs the tool and streams results through the channel.
	// The channel is closed when execution completes or an error occurs.
	ExecuteStream(ctx context.Context, params map[string]interface{}) (<-chan *ToolStreamChunk, error)
}

// ToolExecutionResult represents the outcome of a tool execution.
// It contains the result data, success status, and execution metadata.
//
// Fields:
//   - Success: Whether execution completed without errors
//   - Data: The tool's output (any JSON-serializable type)
//   - Error: Error message if execution failed
//   - Metadata: Additional execution information (timing, warnings, etc.)
//   - Duration: How long the execution took
//   - Timestamp: When execution completed
type ToolExecutionResult struct {
	// Success indicates if the execution completed successfully
	Success bool `json:"success"`

	// Data contains the tool's output
	Data interface{} `json:"data,omitempty"`

	// Error contains any error message
	Error string `json:"error,omitempty"`

	// Metadata contains execution information
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Duration is how long the execution took
	Duration time.Duration `json:"duration,omitempty"`

	// Timestamp is when the execution completed
	Timestamp time.Time `json:"timestamp"`
}

// ToolStreamChunk represents a piece of streaming output.
// Chunks are emitted sequentially during streaming execution.
//
// Chunk types:
//   - "data": Normal output data
//   - "error": Error information
//   - "metadata": Execution metadata (progress, stats)
//   - "complete": Final chunk indicating completion
//
// Example chunk sequence:
//
//	{Type: "metadata", Data: {"total": 100}}
//	{Type: "data", Data: "Processing item 1...", Index: 0}
//	{Type: "data", Data: "Processing item 2...", Index: 1}
//	{Type: "metadata", Data: {"progress": 50}}
//	{Type: "complete", Data: {"processed": 100}}
type ToolStreamChunk struct {
	// Type indicates the chunk type (data, error, metadata, complete)
	Type string `json:"type"`

	// Data contains the chunk payload
	Data interface{} `json:"data,omitempty"`

	// Index is the chunk sequence number
	Index int `json:"index"`

	// Timestamp is when the chunk was produced
	Timestamp time.Time `json:"timestamp"`
}

// ToolFilter is used to query tools in the registry.
// It supports flexible querying by various tool attributes.
//
// Filter examples:
//
//	// Find all text processing tools
//	filter := &ToolFilter{
//		Tags: []string{"text", "nlp"},
//	}
//
//	// Find tools by name pattern
//	filter := &ToolFilter{
//		Name: "*analyzer*",  // Wildcards supported
//	}
//
//	// Find tools with specific capabilities
//	filter := &ToolFilter{
//		Capabilities: &ToolCapabilities{
//			Streaming: true,
//			Cacheable: true,
//		},
//	}
type ToolFilter struct {
	// Name filters by tool name (supports wildcards)
	Name string

	// Tags filters by tool tags (tools must have all specified tags)
	Tags []string

	// Category filters by tool category
	Category string

	// Capabilities filters by required capabilities
	Capabilities *ToolCapabilities

	// Version filters by version constraints (e.g., ">=1.0.0")
	Version string

	// Keywords searches in name and description
	Keywords []string
}

// Validate checks if the tool definition is valid.
// It ensures all required fields are present and properly formatted.
// This method is called automatically during tool registration.
//
// Validation includes:
//   - Required fields (ID, name, description, version)
//   - Schema validity
//   - Executor presence
//
// Returns nil if valid, or an error describing the validation failure.
func (t *Tool) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("tool ID is required")
	}
	if t.Name == "" {
		return fmt.Errorf("tool name is required")
	}
	if t.Description == "" {
		return fmt.Errorf("tool description is required")
	}
	if t.Version == "" {
		return fmt.Errorf("tool version is required")
	}
	if t.Schema == nil {
		return fmt.Errorf("tool schema is required")
	}
	if t.Executor == nil {
		return fmt.Errorf("tool executor is required")
	}

	// Validate schema
	if err := t.Schema.Validate(); err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}

	return nil
}

// Validate checks if the tool schema is valid.
// It ensures the schema follows JSON Schema rules and all
// referenced properties are properly defined.
//
// Validation includes:
//   - Schema type must be "object"
//   - All properties must be valid
//   - Required properties must exist in properties map
//
// Returns nil if valid, or an error describing the validation failure.
func (s *ToolSchema) Validate() error {
	if s.Type != "object" {
		return fmt.Errorf("schema type must be 'object', got %q", s.Type)
	}

	// Validate properties
	for name, prop := range s.Properties {
		if err := prop.Validate(); err != nil {
			return fmt.Errorf("property %q: %w", name, err)
		}
	}

	// Check that required properties exist
	for _, req := range s.Required {
		if _, ok := s.Properties[req]; !ok {
			return fmt.Errorf("required property %q not defined in schema", req)
		}
	}

	return nil
}

// Validate checks if the property definition is valid.
// It ensures the property type is recognized and nested
// properties (for arrays and objects) are also valid.
//
// Returns nil if valid, or an error describing the validation failure.
func (p *Property) Validate() error {
	validTypes := map[string]bool{
		"string":  true,
		"number":  true,
		"integer": true,
		"boolean": true,
		"array":   true,
		"object":  true,
		"null":    true,
	}

	if !validTypes[p.Type] {
		return fmt.Errorf("invalid type %q", p.Type)
	}

	// Validate array items
	if p.Type == "array" && p.Items != nil {
		if err := p.Items.Validate(); err != nil {
			return fmt.Errorf("array items: %w", err)
		}
	}

	// Validate nested object properties
	if p.Type == "object" && p.Properties != nil {
		for name, prop := range p.Properties {
			if err := prop.Validate(); err != nil {
				return fmt.Errorf("nested property %q: %w", name, err)
			}
		}
	}

	return nil
}

// MarshalJSON customizes JSON marshaling for Tool.
// It includes the executor type name in the JSON output for debugging
// and logging purposes, since the actual executor cannot be serialized.
func (t *Tool) MarshalJSON() ([]byte, error) {
	type Alias Tool
	return json.Marshal(&struct {
		*Alias
		Executor string `json:"executor,omitempty"`
	}{
		Alias:    (*Alias)(t),
		Executor: fmt.Sprintf("%T", t.Executor),
	})
}

// Clone creates a deep copy of the tool.
// This is useful when you need to modify a tool without affecting
// the original instance in the registry.
//
// Note: The Executor field is shared (not cloned) since executors
// are typically stateless and safe to share.
func (t *Tool) Clone() *Tool {
	clone := &Tool{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		Version:     t.Version,
		Executor:    t.Executor,
	}

	if t.Schema != nil {
		clone.Schema = t.Schema.Clone()
	}

	if t.Metadata != nil {
		clone.Metadata = t.Metadata.Clone()
	}

	if t.Capabilities != nil {
		clone.Capabilities = &ToolCapabilities{
			Streaming:  t.Capabilities.Streaming,
			Async:      t.Capabilities.Async,
			Cancelable: t.Capabilities.Cancelable,
			Retryable:  t.Capabilities.Retryable,
			Cacheable:  t.Capabilities.Cacheable,
			RateLimit:  t.Capabilities.RateLimit,
			Timeout:    t.Capabilities.Timeout,
		}
	}

	return clone
}

// CloneOptimized creates a copy-on-write clone of the tool for performance optimization.
// This is much faster than regular Clone() for read-only operations.
func (t *Tool) CloneOptimized() *Tool {
	if t == nil {
		return nil
	}

	// If this tool is already shared, just increase reference count
	if t.isShared {
		atomic.AddInt32(&t.refCount, 1)
		return t
	}

	// Create a shallow copy with copy-on-write semantics
	clone := &Tool{
		ID:           t.ID,
		Name:         t.Name,
		Description:  t.Description,
		Version:      t.Version,
		Schema:       t.Schema,       // Shared initially
		Metadata:     t.Metadata,     // Shared initially
		Executor:     t.Executor,     // Always shared
		Capabilities: t.Capabilities, // Shared initially
		refCount:     1,
		isShared:     true,
	}

	// Mark original as shared too
	t.isShared = true
	t.refCount = 1

	return clone
}

// ensureWritable ensures the tool is writable by creating a deep copy if needed.
func (t *Tool) ensureWritable() {
	if !t.isShared {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Double-check after acquiring lock
	if !t.isShared {
		return
	}

	// Create deep copies of shared structures
	if t.Schema != nil {
		t.Schema = t.Schema.Clone()
	}
	if t.Metadata != nil {
		t.Metadata = t.Metadata.Clone()
	}
	if t.Capabilities != nil {
		t.Capabilities = &ToolCapabilities{
			Streaming:  t.Capabilities.Streaming,
			Async:      t.Capabilities.Async,
			Cancelable: t.Capabilities.Cancelable,
			Retryable:  t.Capabilities.Retryable,
			Cacheable:  t.Capabilities.Cacheable,
			RateLimit:  t.Capabilities.RateLimit,
			Timeout:    t.Capabilities.Timeout,
		}
	}

	// Mark as no longer shared
	t.isShared = false
	t.refCount = 0
}

// SetName sets the tool name (triggers copy-on-write if needed).
func (t *Tool) SetName(name string) {
	t.ensureWritable()
	t.Name = name
}

// SetDescription sets the tool description (triggers copy-on-write if needed).
func (t *Tool) SetDescription(description string) {
	t.ensureWritable()
	t.Description = description
}

// SetVersion sets the tool version (triggers copy-on-write if needed).
func (t *Tool) SetVersion(version string) {
	t.ensureWritable()
	t.Version = version
}

// SetSchema sets the tool schema (triggers copy-on-write if needed).
func (t *Tool) SetSchema(schema *ToolSchema) {
	t.ensureWritable()
	t.Schema = schema
}

// SetMetadata sets the tool metadata (triggers copy-on-write if needed).
func (t *Tool) SetMetadata(metadata *ToolMetadata) {
	t.ensureWritable()
	t.Metadata = metadata
}

// SetCapabilities sets the tool capabilities (triggers copy-on-write if needed).
func (t *Tool) SetCapabilities(capabilities *ToolCapabilities) {
	t.ensureWritable()
	t.Capabilities = capabilities
}

// IsShared returns true if the tool is using copy-on-write semantics.
func (t *Tool) IsShared() bool {
	return t.isShared
}

// GetRefCount returns the current reference count for debugging.
func (t *Tool) GetRefCount() int32 {
	return atomic.LoadInt32(&t.refCount)
}

// Clone creates a deep copy of the schema.
// All nested properties and arrays are recursively cloned
// to ensure complete isolation from the original.
func (s *ToolSchema) Clone() *ToolSchema {
	clone := &ToolSchema{
		Type:        s.Type,
		Description: s.Description,
	}

	if s.Properties != nil {
		clone.Properties = make(map[string]*Property, len(s.Properties))
		for k, v := range s.Properties {
			clone.Properties[k] = v.Clone()
		}
	}

	if s.Required != nil {
		clone.Required = make([]string, len(s.Required))
		copy(clone.Required, s.Required)
	}

	if s.AdditionalProperties != nil {
		b := *s.AdditionalProperties
		clone.AdditionalProperties = &b
	}

	return clone
}

// Clone creates a deep copy of the property.
// All nested structures are recursively cloned.
// Pointer fields are dereferenced and copied to prevent sharing.
func (p *Property) Clone() *Property {
	clone := &Property{
		Type:        p.Type,
		Description: p.Description,
		Format:      p.Format,
		Pattern:     p.Pattern,
		Default:     p.Default,
	}

	if p.Enum != nil {
		clone.Enum = make([]interface{}, len(p.Enum))
		copy(clone.Enum, p.Enum)
	}

	if p.Minimum != nil {
		m := *p.Minimum
		clone.Minimum = &m
	}

	if p.Maximum != nil {
		m := *p.Maximum
		clone.Maximum = &m
	}

	if p.MinLength != nil {
		m := *p.MinLength
		clone.MinLength = &m
	}

	if p.MaxLength != nil {
		m := *p.MaxLength
		clone.MaxLength = &m
	}

	if p.Items != nil {
		clone.Items = p.Items.Clone()
	}

	if p.Properties != nil {
		clone.Properties = make(map[string]*Property, len(p.Properties))
		for k, v := range p.Properties {
			clone.Properties[k] = v.Clone()
		}
	}

	if p.Required != nil {
		clone.Required = make([]string, len(p.Required))
		copy(clone.Required, p.Required)
	}

	return clone
}

// Clone creates a deep copy of the metadata.
// All arrays and maps are copied to ensure isolation.
// The custom map is shallow-copied (values are not deep-cloned).
func (m *ToolMetadata) Clone() *ToolMetadata {
	clone := &ToolMetadata{
		Author:        m.Author,
		License:       m.License,
		Documentation: m.Documentation,
	}

	if m.Examples != nil {
		clone.Examples = make([]ToolExample, len(m.Examples))
		for i, ex := range m.Examples {
			clone.Examples[i] = ToolExample{
				Name:        ex.Name,
				Description: ex.Description,
				Input:       cloneMap(ex.Input),
				Output:      ex.Output,
			}
		}
	}

	if m.Tags != nil {
		clone.Tags = make([]string, len(m.Tags))
		copy(clone.Tags, m.Tags)
	}

	if m.Dependencies != nil {
		clone.Dependencies = make([]string, len(m.Dependencies))
		copy(clone.Dependencies, m.Dependencies)
	}

	if m.Custom != nil {
		clone.Custom = cloneMap(m.Custom)
	}

	return clone
}

// cloneMap creates a shallow copy of a map.
func cloneMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	clone := make(map[string]interface{}, len(m))
	for k, v := range m {
		clone[k] = v
	}
	return clone
}
