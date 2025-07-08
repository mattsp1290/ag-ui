package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Registry manages the collection of available tools.
// It provides thread-safe registration, discovery, and management of tools.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*Tool

	// categoryIndex maps categories to tool IDs for fast lookup
	categoryIndex map[string]map[string]bool

	// tagIndex maps tags to tool IDs for fast lookup
	tagIndex map[string]map[string]bool

	// nameIndex maps tool names to IDs for fast lookup
	nameIndex map[string]string

	// validators for custom validation rules
	validators []RegistryValidator

	// Enhanced features
	categories        *CategoryTree
	conflictResolvers []ConflictResolver
	migrationHandlers map[string]MigrationHandler
	dynamicLoaders    map[string]DynamicLoader
	watchers          map[string]*FileWatcher
	dependencyGraph   *DependencyGraph
	loadingStrategies map[string]LoadingStrategy

	// Configuration
	config *RegistryConfig

	// Separate mutex for conflict resolution to prevent deadlocks
	conflictMu sync.Mutex
}

// RegistryValidator is a function that validates tools during registration.
type RegistryValidator func(tool *Tool) error

// RegistryConfig holds configuration for the registry.
type RegistryConfig struct {
	// EnableHotReloading enables automatic reloading of tools from files
	EnableHotReloading bool
	// HotReloadInterval specifies how often to check for file changes
	HotReloadInterval time.Duration
	// MaxDependencyDepth limits the depth of dependency resolution
	MaxDependencyDepth int
	// ConflictResolutionStrategy defines how to handle conflicts
	ConflictResolutionStrategy ConflictStrategy
	// EnableVersionMigration enables automatic version migration
	EnableVersionMigration bool
	// MigrationTimeout specifies the timeout for migration operations
	MigrationTimeout time.Duration
	// LoadingTimeout specifies the timeout for loading operations
	LoadingTimeout time.Duration
	// EnableCaching enables caching of loaded tools
	EnableCaching bool
	// CacheExpiration specifies how long cached tools remain valid
	CacheExpiration time.Duration
}

// DefaultRegistryConfig returns the default configuration.
func DefaultRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		EnableHotReloading:         false,
		HotReloadInterval:          30 * time.Second,
		MaxDependencyDepth:         10,
		ConflictResolutionStrategy: ConflictStrategyError,
		EnableVersionMigration:     true,
		MigrationTimeout:           30 * time.Second,
		LoadingTimeout:             10 * time.Second,
		EnableCaching:              true,
		CacheExpiration:            5 * time.Minute,
	}
}

// ConflictStrategy defines how to handle tool conflicts.
type ConflictStrategy int

const (
	ConflictStrategyError ConflictStrategy = iota
	ConflictStrategyOverwrite
	ConflictStrategySkip
	ConflictStrategyVersionBased
	ConflictStrategyPriorityBased
)

// ConflictResolver defines a function that resolves tool conflicts.
type ConflictResolver func(existing *Tool, new *Tool) (*Tool, error)

// MigrationHandler defines a function that handles tool version migrations.
type MigrationHandler func(ctx context.Context, oldTool, newTool *Tool) error

// DynamicLoader defines a function that loads tools from external sources.
type DynamicLoader func(ctx context.Context, source string) ([]*Tool, error)

// LoadingStrategy defines how tools are loaded and cached.
type LoadingStrategy int

const (
	LoadingStrategyImmediate LoadingStrategy = iota
	LoadingStrategyLazy
	LoadingStrategyPreemptive
)

// CategoryTree represents a hierarchical category structure for tools.
type CategoryTree struct {
	mu     sync.RWMutex
	root   *CategoryNode
	index  map[string]*CategoryNode
	tools  map[string]map[string]bool // category -> tool IDs
}

// CategoryNode represents a node in the category tree.
type CategoryNode struct {
	Name        string
	Path        string
	Parent      *CategoryNode
	Children    map[string]*CategoryNode
	Metadata    map[string]interface{}
	Inheritance *CategoryInheritance
}

// CategoryInheritance defines how categories inherit properties.
type CategoryInheritance struct {
	InheritTags         bool
	InheritCapabilities bool
	InheritMetadata     bool
	InheritValidators   bool
}

// NewCategoryTree creates a new category tree.
func NewCategoryTree() *CategoryTree {
	return &CategoryTree{
		root: &CategoryNode{
			Name:     "root",
			Path:     "",
			Children: make(map[string]*CategoryNode),
		},
		index: make(map[string]*CategoryNode),
		tools: make(map[string]map[string]bool),
	}
}

// FileWatcher watches files for changes and triggers reloading.
type FileWatcher struct {
	mu       sync.RWMutex
	path     string
	modTime  time.Time
	callback func(string) error
	stop     chan struct{}
	stopped  bool
	wg       sync.WaitGroup // Track goroutine lifecycle
}

// DependencyGraph manages tool dependencies and resolution.
type DependencyGraph struct {
	mu           sync.RWMutex
	dependencies map[string]map[string]*DependencyConstraint
	resolved     map[string][]*Tool
}

// DependencyConstraint defines a dependency with version constraints.
type DependencyConstraint struct {
	ToolID           string
	VersionConstraint string
	Optional         bool
	Transitive       bool
}

// NewDependencyGraph creates a new dependency graph.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		dependencies: make(map[string]map[string]*DependencyConstraint),
		resolved:     make(map[string][]*Tool),
	}
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return NewRegistryWithConfig(DefaultRegistryConfig())
}

// NewRegistryWithConfig creates a new tool registry with custom configuration.
func NewRegistryWithConfig(config *RegistryConfig) *Registry {
	return &Registry{
		tools:             make(map[string]*Tool),
		categoryIndex:     make(map[string]map[string]bool),
		tagIndex:          make(map[string]map[string]bool),
		nameIndex:         make(map[string]string),
		validators:        []RegistryValidator{},
		categories:        NewCategoryTree(),
		conflictResolvers: []ConflictResolver{},
		migrationHandlers: make(map[string]MigrationHandler),
		dynamicLoaders:    make(map[string]DynamicLoader),
		watchers:          make(map[string]*FileWatcher),
		dependencyGraph:   NewDependencyGraph(),
		loadingStrategies: make(map[string]LoadingStrategy),
		config:            config,
	}
}

// Register adds a new tool to the registry.
// It returns an error if the tool is invalid or if a tool with the same ID already exists.
func (r *Registry) Register(tool *Tool) error {
	return r.RegisterWithContext(context.Background(), tool)
}

// RegisterWithContext adds a new tool to the registry with context support.
func (r *Registry) RegisterWithContext(ctx context.Context, tool *Tool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	// Validate the tool
	if err := tool.Validate(); err != nil {
		return fmt.Errorf("tool validation failed: %w", err)
	}

	// Run custom validators
	for _, validator := range r.validators {
		if err := validator(tool); err != nil {
			return fmt.Errorf("custom validation failed: %w", err)
		}
	}

	// Handle conflict resolution with separate mutex to prevent deadlocks
	r.mu.RLock()
	existingTool, idExists := r.tools[tool.ID]
	existingID, nameExists := r.nameIndex[tool.Name]
	var existingByName *Tool
	if nameExists && existingID != tool.ID {
		existingByName = r.tools[existingID]
	}
	r.mu.RUnlock()

	// Resolve conflicts outside of main mutex to prevent deadlocks
	if idExists {
		r.conflictMu.Lock()
		resolvedTool, err := r.resolveConflict(ctx, existingTool, tool)
		r.conflictMu.Unlock()
		if err != nil {
			return fmt.Errorf("conflict resolution failed: %w", err)
		}
		if resolvedTool == nil {
			// Conflict resolution decided to skip registration
			return nil
		}
		tool = resolvedTool
	}

	// Check for name conflicts
	if existingByName != nil {
		r.conflictMu.Lock()
		resolvedTool, err := r.resolveConflict(ctx, existingByName, tool)
		r.conflictMu.Unlock()
		if err != nil {
			return fmt.Errorf("name conflict resolution failed: %w", err)
		}
		if resolvedTool == nil {
			return nil
		}
		tool = resolvedTool
	}

	// Now acquire write lock for actual registration
	r.mu.Lock()
	defer r.mu.Unlock()

	// Handle version migration if enabled
	if r.config.EnableVersionMigration {
		if err := r.handleVersionMigration(ctx, tool); err != nil {
			return fmt.Errorf("version migration failed: %w", err)
		}
	}

	// Store a clone to prevent external modifications
	clonedTool := tool.Clone()
	r.tools[tool.ID] = clonedTool

	// Update indexes
	r.nameIndex[tool.Name] = tool.ID

	// Update tag index
	if tool.Metadata != nil && len(tool.Metadata.Tags) > 0 {
		for _, tag := range tool.Metadata.Tags {
			if r.tagIndex[tag] == nil {
				r.tagIndex[tag] = make(map[string]bool)
			}
			r.tagIndex[tag][tool.ID] = true
		}
	}

	// Update category tree
	if err := r.updateCategoryTree(tool); err != nil {
		// Log error but don't fail registration
		// In production, you might want to handle this differently
	}

	// Update dependency graph
	if err := r.dependencyGraph.AddTool(tool); err != nil {
		return fmt.Errorf("dependency graph update failed: %w", err)
	}

	return nil
}

// Unregister removes a tool from the registry.
// It returns an error if the tool is not found.
func (r *Registry) Unregister(toolID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tool, exists := r.tools[toolID]
	if !exists {
		return fmt.Errorf("tool with ID %q not found", toolID)
	}

	// Remove from main storage
	delete(r.tools, toolID)

	// Remove from name index
	delete(r.nameIndex, tool.Name)

	// Remove from tag index
	if tool.Metadata != nil && len(tool.Metadata.Tags) > 0 {
		for _, tag := range tool.Metadata.Tags {
			if tagMap := r.tagIndex[tag]; tagMap != nil {
				delete(tagMap, toolID)
				if len(tagMap) == 0 {
					delete(r.tagIndex, tag)
				}
			}
		}
	}

	return nil
}

// Get retrieves a tool by its ID.
// It returns nil if the tool is not found.
// This method returns a clone for backward compatibility.
func (r *Registry) Get(toolID string) (*Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[toolID]
	if !exists {
		return nil, fmt.Errorf("tool with ID %q not found", toolID)
	}

	// Return a clone to prevent external modifications
	return tool.Clone(), nil
}

// GetReadOnly retrieves a read-only view of a tool by its ID.
// This is more memory-efficient than Get() as it avoids cloning.
func (r *Registry) GetReadOnly(toolID string) (ReadOnlyTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[toolID]
	if !exists {
		return nil, fmt.Errorf("tool with ID %q not found", toolID)
	}

	// Return a read-only view without cloning
	return NewReadOnlyTool(tool), nil
}

// GetByName retrieves a tool by its name.
// It returns nil if the tool is not found.
// This method returns a clone for backward compatibility.
func (r *Registry) GetByName(name string) (*Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	toolID, exists := r.nameIndex[name]
	if !exists {
		return nil, fmt.Errorf("tool with name %q not found", name)
	}

	tool := r.tools[toolID]
	return tool.Clone(), nil
}

// GetByNameReadOnly retrieves a read-only view of a tool by its name.
// This is more memory-efficient than GetByName() as it avoids cloning.
func (r *Registry) GetByNameReadOnly(name string) (ReadOnlyTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	toolID, exists := r.nameIndex[name]
	if !exists {
		return nil, fmt.Errorf("tool with name %q not found", name)
	}

	tool := r.tools[toolID]
	return NewReadOnlyTool(tool), nil
}

// List returns all tools that match the given filter.
// If filter is nil, all tools are returned.
// This method returns clones for backward compatibility.
func (r *Registry) List(filter *ToolFilter) ([]*Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*Tool

	for _, tool := range r.tools {
		if filter == nil || r.matchesFilter(tool, filter) {
			results = append(results, tool.Clone())
		}
	}

	return results, nil
}

// ListReadOnly returns read-only views of all tools that match the given filter.
// This is more memory-efficient than List() as it avoids cloning.
func (r *Registry) ListReadOnly(filter *ToolFilter) ([]ReadOnlyTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []ReadOnlyTool

	for _, tool := range r.tools {
		if filter == nil || r.matchesFilter(tool, filter) {
			results = append(results, NewReadOnlyTool(tool))
		}
	}

	return results, nil
}

// ListAll returns all registered tools.
func (r *Registry) ListAll() ([]*Tool, error) {
	return r.List(nil)
}

// Count returns the number of registered tools.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// Clear removes all tools from the registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools = make(map[string]*Tool)
	r.categoryIndex = make(map[string]map[string]bool)
	r.tagIndex = make(map[string]map[string]bool)
	r.nameIndex = make(map[string]string)
}

// AddValidator adds a custom validation function that will be run
// during tool registration.
func (r *Registry) AddValidator(validator RegistryValidator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.validators = append(r.validators, validator)
}

// Validate runs validation on all registered tools.
// This is useful for ensuring registry consistency.
func (r *Registry) Validate() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for id, tool := range r.tools {
		if err := tool.Validate(); err != nil {
			return fmt.Errorf("tool %q validation failed: %w", id, err)
		}

		for _, validator := range r.validators {
			if err := validator(tool); err != nil {
				return fmt.Errorf("tool %q custom validation failed: %w", id, err)
			}
		}
	}

	return nil
}

// matchesFilter checks if a tool matches the given filter criteria.
func (r *Registry) matchesFilter(tool *Tool, filter *ToolFilter) bool {
	// Check name filter (supports wildcards with *)
	if filter.Name != "" {
		if strings.Contains(filter.Name, "*") {
			pattern := strings.ReplaceAll(filter.Name, "*", "")
			if !strings.Contains(tool.Name, pattern) {
				return false
			}
		} else if tool.Name != filter.Name {
			return false
		}
	}

	// Check tags filter (tool must have all specified tags)
	if len(filter.Tags) > 0 && tool.Metadata != nil {
		toolTags := make(map[string]bool)
		for _, tag := range tool.Metadata.Tags {
			toolTags[tag] = true
		}

		for _, requiredTag := range filter.Tags {
			if !toolTags[requiredTag] {
				return false
			}
		}
	} else if len(filter.Tags) > 0 {
		// Tool has no metadata/tags but filter requires tags
		return false
	}

	// Check capabilities filter
	if filter.Capabilities != nil && tool.Capabilities != nil {
		caps := filter.Capabilities
		toolCaps := tool.Capabilities

		if caps.Streaming && !toolCaps.Streaming {
			return false
		}
		if caps.Async && !toolCaps.Async {
			return false
		}
		if caps.Cancelable && !toolCaps.Cancelable {
			return false
		}
		if caps.Retryable && !toolCaps.Retryable {
			return false
		}
		if caps.Cacheable && !toolCaps.Cacheable {
			return false
		}
	} else if filter.Capabilities != nil {
		// Tool has no capabilities but filter requires them
		return false
	}

	// Check keywords in name and description
	if len(filter.Keywords) > 0 {
		searchText := strings.ToLower(tool.Name + " " + tool.Description)
		for _, keyword := range filter.Keywords {
			if !strings.Contains(searchText, strings.ToLower(keyword)) {
				return false
			}
		}
	}

	// Check version constraint
	if filter.Version != "" {
		matches, err := matchesVersionConstraint(tool.Version, filter.Version)
		if err != nil {
			// Log error but don't fail the match - treat as no constraint
			// In production, you might want to handle this differently
			return true
		}
		if !matches {
			return false
		}
	}

	return true
}

// GetDependencies returns all tools that the specified tool depends on.
func (r *Registry) GetDependencies(toolID string) ([]*Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, exists := r.tools[toolID]
	if !exists {
		return nil, fmt.Errorf("tool with ID %q not found", toolID)
	}

	if tool.Metadata == nil || len(tool.Metadata.Dependencies) == 0 {
		return []*Tool{}, nil
	}

	var dependencies []*Tool
	for _, depID := range tool.Metadata.Dependencies {
		dep, exists := r.tools[depID]
		if !exists {
			return nil, fmt.Errorf("dependency %q not found for tool %q", depID, toolID)
		}
		dependencies = append(dependencies, dep.Clone())
	}

	return dependencies, nil
}

// HasCircularDependency checks if registering a tool would create a circular dependency.
func (r *Registry) HasCircularDependency(tool *Tool) bool {
	if tool.Metadata == nil || len(tool.Metadata.Dependencies) == 0 {
		return false
	}

	visited := make(map[string]bool)
	stack := make(map[string]bool)

	var hasCycle func(toolID string) bool
	hasCycle = func(toolID string) bool {
		visited[toolID] = true
		stack[toolID] = true

		t, exists := r.tools[toolID]
		if !exists && toolID == tool.ID {
			t = tool // Check the tool being registered
		}

		if t != nil && t.Metadata != nil {
			for _, depID := range t.Metadata.Dependencies {
				if stack[depID] {
					return true // Cycle detected
				}

				if !visited[depID] && hasCycle(depID) {
					return true
				}
			}
		}

		stack[toolID] = false
		return false
	}

	return hasCycle(tool.ID)
}

// ExportTools returns all tools in a format suitable for serialization.
func (r *Registry) ExportTools() map[string]*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	export := make(map[string]*Tool, len(r.tools))
	for id, tool := range r.tools {
		export[id] = tool.Clone()
	}
	return export
}

// ImportTools bulk imports tools into the registry.
// It returns a slice of errors for any tools that failed to import.
func (r *Registry) ImportTools(tools map[string]*Tool) []error {
	var errors []error

	for _, tool := range tools {
		if err := r.Register(tool); err != nil {
			errors = append(errors, fmt.Errorf("failed to import tool %q: %w", tool.ID, err))
		}
	}

	return errors
}

// Enhanced Methods for New Features

// resolveConflict resolves conflicts between existing and new tools.
func (r *Registry) resolveConflict(ctx context.Context, existing, new *Tool) (*Tool, error) {
	// Apply custom conflict resolvers first
	for _, resolver := range r.conflictResolvers {
		resolved, err := resolver(existing, new)
		if err != nil {
			return nil, err
		}
		if resolved != nil {
			return resolved, nil
		}
	}

	// Apply built-in conflict resolution strategy
	switch r.config.ConflictResolutionStrategy {
	case ConflictStrategyError:
		return nil, fmt.Errorf("tool with ID %q already exists", existing.ID)
	case ConflictStrategyOverwrite:
		return new, nil
	case ConflictStrategySkip:
		return nil, nil // Skip registration
	case ConflictStrategyVersionBased:
		return r.resolveVersionBasedConflict(existing, new)
	case ConflictStrategyPriorityBased:
		return r.resolvePriorityBasedConflict(existing, new)
	default:
		return nil, fmt.Errorf("unknown conflict resolution strategy: %v", r.config.ConflictResolutionStrategy)
	}
}

// resolveVersionBasedConflict resolves conflicts based on version comparison.
func (r *Registry) resolveVersionBasedConflict(existing, new *Tool) (*Tool, error) {
	matches, err := matchesVersionConstraint(new.Version, ">"+existing.Version)
	if err != nil {
		return nil, fmt.Errorf("version comparison failed: %w", err)
	}
	if matches {
		return new, nil // New version is higher
	}
	return nil, nil // Keep existing version
}

// resolvePriorityBasedConflict resolves conflicts based on priority metadata.
func (r *Registry) resolvePriorityBasedConflict(existing, new *Tool) (*Tool, error) {
	existingPriority := r.getToolPriority(existing)
	newPriority := r.getToolPriority(new)
	
	if newPriority > existingPriority {
		return new, nil
	}
	return nil, nil
}

// getToolPriority extracts priority from tool metadata.
func (r *Registry) getToolPriority(tool *Tool) int {
	if tool.Metadata == nil || tool.Metadata.Custom == nil {
		return 0
	}
	if priority, ok := tool.Metadata.Custom["priority"]; ok {
		if p, ok := priority.(int); ok {
			return p
		}
		if p, ok := priority.(float64); ok {
			return int(p)
		}
	}
	return 0
}

// handleVersionMigration handles version migration for tools.
func (r *Registry) handleVersionMigration(ctx context.Context, tool *Tool) error {
	if existing, exists := r.tools[tool.ID]; exists {
		if handler, exists := r.migrationHandlers[existing.Version]; exists {
			return handler(ctx, existing, tool)
		}
		
		// Default migration behavior
		return r.defaultVersionMigration(ctx, existing, tool)
	}
	return nil
}

// defaultVersionMigration provides default migration behavior.
func (r *Registry) defaultVersionMigration(ctx context.Context, oldTool, newTool *Tool) error {
	// Check if migration is needed
	if oldTool.Version == newTool.Version {
		return nil
	}
	
	// Perform basic compatibility checks
	if err := r.validateMigrationCompatibility(oldTool, newTool); err != nil {
		return fmt.Errorf("migration compatibility check failed: %w", err)
	}
	
	return nil
}

// validateMigrationCompatibility validates that migration is safe.
func (r *Registry) validateMigrationCompatibility(oldTool, newTool *Tool) error {
	// Check if required parameters are maintained
	if oldTool.Schema != nil && newTool.Schema != nil {
		for _, req := range oldTool.Schema.Required {
			found := false
			for _, newReq := range newTool.Schema.Required {
				if req == newReq {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("required parameter %q removed in new version", req)
			}
		}
	}
	
	return nil
}

// updateCategoryTree updates the category tree with the tool.
func (r *Registry) updateCategoryTree(tool *Tool) error {
	if tool.Metadata == nil || len(tool.Metadata.Tags) == 0 {
		return nil
	}
	
	// Add tool to each category tag
	for _, tag := range tool.Metadata.Tags {
		// Add the category if it doesn't exist
		if err := r.categories.AddCategory(tag, nil); err != nil {
			return err
		}
		// Add the tool to the category
		if err := r.categories.AddTool(tag, tool.ID); err != nil {
			return err
		}
	}
	
	return nil
}

// AddConflictResolver adds a custom conflict resolver.
func (r *Registry) AddConflictResolver(resolver ConflictResolver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.conflictResolvers = append(r.conflictResolvers, resolver)
}

// AddMigrationHandler adds a custom migration handler for a specific version.
func (r *Registry) AddMigrationHandler(version string, handler MigrationHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.migrationHandlers[version] = handler
}

// LoadFromFile loads tools from a JSON file.
func (r *Registry) LoadFromFile(ctx context.Context, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file %q: %w", filename, err)
	}
	defer file.Close()
	
	return r.LoadFromReader(ctx, file)
}

// LoadFromReader loads tools from a JSON reader.
func (r *Registry) LoadFromReader(ctx context.Context, reader io.Reader) error {
	var tools []*Tool
	decoder := json.NewDecoder(reader)
	
	if err := decoder.Decode(&tools); err != nil {
		return fmt.Errorf("failed to decode tools: %w", err)
	}
	
	for _, tool := range tools {
		// Add a default executor if none exists (for tools loaded from JSON)
		if tool.Executor == nil {
			tool.Executor = &DefaultExecutor{}
		}
		
		if err := r.RegisterWithContext(ctx, tool); err != nil {
			return fmt.Errorf("failed to register tool %q: %w", tool.ID, err)
		}
	}
	
	return nil
}

// DefaultExecutor is a simple executor for tools loaded from JSON.
type DefaultExecutor struct{}

// Execute provides a default implementation for loaded tools.
func (e *DefaultExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	return &ToolExecutionResult{
		Success:   true,
		Data:      "Tool executed successfully",
		Timestamp: time.Now(),
	}, nil
}

// LoadFromURL loads tools from a URL.
func (r *Registry) LoadFromURL(ctx context.Context, url string) error {
	client := &http.Client{
		Timeout: r.config.LoadingTimeout,
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch from URL: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}
	
	return r.LoadFromReader(ctx, resp.Body)
}

// WatchFile watches a file for changes and reloads tools.
func (r *Registry) WatchFile(filename string) error {
	if !r.config.EnableHotReloading {
		return fmt.Errorf("hot reloading is disabled")
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Check and insert atomically to prevent TOCTOU race condition
	if _, exists := r.watchers[filename]; exists {
		return fmt.Errorf("file %q is already being watched", filename)
	}
	
	watcher := &FileWatcher{
		path: filename,
		stop: make(chan struct{}),
		callback: func(path string) error {
			return r.LoadFromFile(context.Background(), path)
		},
	}
	
	// Insert watcher before starting goroutine to prevent races
	r.watchers[filename] = watcher
	
	// Start goroutine with proper lifecycle tracking
	watcher.wg.Add(1)
	go func() {
		defer watcher.wg.Done()
		r.watchFileChanges(watcher)
	}()
	
	return nil
}

// watchFileChanges monitors a file for changes.
func (r *Registry) watchFileChanges(watcher *FileWatcher) {
	ticker := time.NewTicker(r.config.HotReloadInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-watcher.stop:
			return
		case <-ticker.C:
			if err := r.checkFileForChanges(watcher); err != nil {
				// Log error in production
				continue
			}
		}
	}
}

// checkFileForChanges checks if a file has been modified.
func (r *Registry) checkFileForChanges(watcher *FileWatcher) error {
	watcher.mu.Lock()
	defer watcher.mu.Unlock()
	
	info, err := os.Stat(watcher.path)
	if err != nil {
		return err
	}
	
	if info.ModTime().After(watcher.modTime) {
		watcher.modTime = info.ModTime()
		return watcher.callback(watcher.path)
	}
	
	return nil
}

// StopWatching stops watching a file.
func (r *Registry) StopWatching(filename string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	watcher, exists := r.watchers[filename]
	if !exists {
		return fmt.Errorf("file %q is not being watched", filename)
	}
	
	watcher.mu.Lock()
	if !watcher.stopped {
		close(watcher.stop)
		watcher.stopped = true
	}
	watcher.mu.Unlock()
	
	// Wait for goroutine to complete before removing from registry
	watcher.wg.Wait()
	
	delete(r.watchers, filename)
	return nil
}

// GetDependenciesWithConstraints returns all tools that match the dependency constraints.
func (r *Registry) GetDependenciesWithConstraints(toolID string) ([]*Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	return r.dependencyGraph.ResolveDependencies(toolID, r.tools)
}

// GetByCategory returns all tools in a category.
func (r *Registry) GetByCategory(category string) ([]*Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	toolIDs := r.categories.GetToolsInCategory(category)
	tools := make([]*Tool, 0, len(toolIDs))
	
	for _, toolID := range toolIDs {
		if tool, exists := r.tools[toolID]; exists {
			tools = append(tools, tool.Clone())
		}
	}
	
	return tools, nil
}

// GetCategoryTree returns the category tree.
func (r *Registry) GetCategoryTree() *CategoryTree {
	return r.categories
}

// SetConfig updates the registry configuration.
func (r *Registry) SetConfig(config *RegistryConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = config
}

// GetConfig returns the current registry configuration.
func (r *Registry) GetConfig() *RegistryConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// CategoryTree Methods

// AddCategory adds a new category to the tree.
func (ct *CategoryTree) AddCategory(path string, metadata map[string]interface{}) error {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	
	parts := strings.Split(path, "/")
	current := ct.root
	
	for _, part := range parts {
		if part == "" {
			continue
		}
		
		if current.Children == nil {
			current.Children = make(map[string]*CategoryNode)
		}
		
		if _, exists := current.Children[part]; !exists {
			node := &CategoryNode{
				Name:     part,
				Path:     path,
				Parent:   current,
				Children: make(map[string]*CategoryNode),
				Metadata: metadata,
				Inheritance: &CategoryInheritance{
					InheritTags:         true,
					InheritCapabilities: true,
					InheritMetadata:     true,
					InheritValidators:   true,
				},
			}
			current.Children[part] = node
			ct.index[path] = node
		}
		
		current = current.Children[part]
	}
	
	return nil
}

// AddTool adds a tool to a category.
func (ct *CategoryTree) AddTool(category, toolID string) error {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	
	if ct.tools[category] == nil {
		ct.tools[category] = make(map[string]bool)
	}
	
	ct.tools[category][toolID] = true
	return nil
}

// GetToolsInCategory returns all tools in a category.
func (ct *CategoryTree) GetToolsInCategory(category string) []string {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	toolMap := ct.tools[category]
	if toolMap == nil {
		return []string{}
	}
	
	tools := make([]string, 0, len(toolMap))
	for toolID := range toolMap {
		tools = append(tools, toolID)
	}
	
	return tools
}

// GetCategoryNode returns a category node by path.
func (ct *CategoryTree) GetCategoryNode(path string) *CategoryNode {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	return ct.index[path]
}

// GetAllCategories returns all categories.
func (ct *CategoryTree) GetAllCategories() []string {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	categories := make([]string, 0, len(ct.index))
	for path := range ct.index {
		categories = append(categories, path)
	}
	
	return categories
}

// DependencyGraph Methods

// AddTool adds a tool to the dependency graph.
func (dg *DependencyGraph) AddTool(tool *Tool) error {
	dg.mu.Lock()
	defer dg.mu.Unlock()
	
	if tool.Metadata == nil || len(tool.Metadata.Dependencies) == 0 {
		return nil
	}
	
	if dg.dependencies[tool.ID] == nil {
		dg.dependencies[tool.ID] = make(map[string]*DependencyConstraint)
	}
	
	// Add dependencies
	for _, depID := range tool.Metadata.Dependencies {
		constraint := &DependencyConstraint{
			ToolID:           depID,
			VersionConstraint: "", // Default to any version
			Optional:         false,
			Transitive:       true,
		}
		
		dg.dependencies[tool.ID][depID] = constraint
	}
	
	return nil
}

// AddDependency adds a dependency with constraints.
func (dg *DependencyGraph) AddDependency(toolID, depID, versionConstraint string, optional bool) error {
	dg.mu.Lock()
	defer dg.mu.Unlock()
	
	if dg.dependencies[toolID] == nil {
		dg.dependencies[toolID] = make(map[string]*DependencyConstraint)
	}
	
	constraint := &DependencyConstraint{
		ToolID:           depID,
		VersionConstraint: versionConstraint,
		Optional:         optional,
		Transitive:       true,
	}
	
	dg.dependencies[toolID][depID] = constraint
	return nil
}

// ResolveDependencies resolves all dependencies for a tool.
func (dg *DependencyGraph) ResolveDependencies(toolID string, tools map[string]*Tool) ([]*Tool, error) {
	dg.mu.RLock()
	defer dg.mu.RUnlock()
	
	// Check cache first
	if resolved, exists := dg.resolved[toolID]; exists {
		return resolved, nil
	}
	
	visited := make(map[string]bool)
	var result []*Tool
	
	if err := dg.resolveDependenciesRecursive(toolID, tools, visited, &result, 0, 10); err != nil {
		return nil, err
	}
	
	// Cache the result
	dg.resolved[toolID] = result
	return result, nil
}

// resolveDependenciesRecursive recursively resolves dependencies.
func (dg *DependencyGraph) resolveDependenciesRecursive(toolID string, tools map[string]*Tool, visited map[string]bool, result *[]*Tool, depth, maxDepth int) error {
	if depth > maxDepth {
		return fmt.Errorf("dependency resolution depth exceeded for tool %q", toolID)
	}
	
	if visited[toolID] {
		return fmt.Errorf("circular dependency detected for tool %q", toolID)
	}
	
	visited[toolID] = true
	defer func() { visited[toolID] = false }()
	
	dependencies := dg.dependencies[toolID]
	for _, constraint := range dependencies {
		tool, exists := tools[constraint.ToolID]
		if !exists {
			if !constraint.Optional {
				return fmt.Errorf("required dependency %q not found for tool %q", constraint.ToolID, toolID)
			}
			continue
		}
		
		// Check version constraint
		if constraint.VersionConstraint != "" {
			matches, err := matchesVersionConstraint(tool.Version, constraint.VersionConstraint)
			if err != nil {
				return fmt.Errorf("version constraint check failed for %q: %w", constraint.ToolID, err)
			}
			if !matches {
				if !constraint.Optional {
					return fmt.Errorf("version constraint %q not satisfied for dependency %q", constraint.VersionConstraint, constraint.ToolID)
				}
				continue
			}
		}
		
		// Add to result
		*result = append(*result, tool.Clone())
		
		// Resolve transitive dependencies
		if constraint.Transitive {
			if err := dg.resolveDependenciesRecursive(constraint.ToolID, tools, visited, result, depth+1, maxDepth); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// HasCircularDependencies checks if the graph has circular dependencies.
func (dg *DependencyGraph) HasCircularDependencies() bool {
	dg.mu.RLock()
	defer dg.mu.RUnlock()
	
	visited := make(map[string]bool)
	stack := make(map[string]bool)
	
	for toolID := range dg.dependencies {
		if !visited[toolID] {
			if dg.hasCycleDFS(toolID, visited, stack) {
				return true
			}
		}
	}
	
	return false
}

// hasCycleDFS performs DFS to detect cycles.
func (dg *DependencyGraph) hasCycleDFS(toolID string, visited, stack map[string]bool) bool {
	visited[toolID] = true
	stack[toolID] = true
	
	for _, constraint := range dg.dependencies[toolID] {
		if !visited[constraint.ToolID] {
			if dg.hasCycleDFS(constraint.ToolID, visited, stack) {
				return true
			}
		} else if stack[constraint.ToolID] {
			return true
		}
	}
	
	stack[toolID] = false
	return false
}

// GetDependencyGraph returns the dependency graph.
func (dg *DependencyGraph) GetDependencyGraph() map[string]map[string]*DependencyConstraint {
	dg.mu.RLock()
	defer dg.mu.RUnlock()
	
	result := make(map[string]map[string]*DependencyConstraint)
	for toolID, deps := range dg.dependencies {
		result[toolID] = make(map[string]*DependencyConstraint)
		for depID, constraint := range deps {
			result[toolID][depID] = &DependencyConstraint{
				ToolID:           constraint.ToolID,
				VersionConstraint: constraint.VersionConstraint,
				Optional:         constraint.Optional,
				Transitive:       constraint.Transitive,
			}
		}
	}
	
	return result
}
