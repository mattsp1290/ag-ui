package tools

import (
	"container/list"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ToolRegistryEntry wraps tool data with metadata for cleanup tracking
type ToolRegistryEntry struct {
	tool        *Tool
	createdAt   time.Time
	lastAccess  time.Time
	accessCount int64
}

// Registry manages the collection of available tools with bounded memory usage.
// It provides thread-safe registration, discovery, and management of tools.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*ToolRegistryEntry

	// categoryIndex maps categories to tool IDs for fast lookup
	categoryIndex map[string]map[string]bool

	// tagIndex maps tags to tool IDs for fast lookup
	tagIndex map[string]map[string]bool

	// nameIndex maps tool names to IDs for fast lookup
	nameIndex map[string]string

	// LRU tracking for tools map
	toolsLRU   *list.List
	toolsIndex map[string]*list.Element

	// validators for custom validation rules
	validators []RegistryValidator

	// Enhanced features
	categories        *CategoryTree
	conflictResolvers []ConflictResolver
	migrationHandlers map[string]MigrationHandler
	dynamicLoaders    map[string]DynamicLoader
	watchers          sync.Map // Use sync.Map for concurrent access
	dependencyGraph   *DependencyGraph
	loadingStrategies map[string]LoadingStrategy

	// Configuration
	config *RegistryConfig

	// Hook protection mutex
	hookMu sync.RWMutex

	// Performance optimization components
	listCache    *ListCache
	schemaCache  *SchemaCache
	memoryPool   *MemoryPool

	// Resource tracking
	currentMemoryUsage   int64  // Current memory usage in bytes
	activeRegistrations  int32  // Number of active registration operations
	
	// Tool cleanup control
	toolCleanupStop chan struct{}
	toolCleanupOnce sync.Once
}

// RegistryValidator is a function that validates tools during registration.
// It receives a tool and returns an error if validation fails.
// Custom validators can be added to enforce specific business rules or constraints
// beyond the standard schema validation.
//
// Example:
//
//	func requireAuthorValidator(tool *Tool) error {
//		if tool.Metadata == nil || tool.Metadata.Author == "" {
//			return fmt.Errorf("tool must have an author")
//		}
//		return nil
//	}
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
	// MaxTools limits the maximum number of tools that can be registered
	MaxTools int
	// MaxMemoryUsage limits the total memory usage of the registry in bytes
	MaxMemoryUsage int64
	// MaxConcurrentRegistrations limits concurrent registration operations
	MaxConcurrentRegistrations int32
	// ToolTTL is the time-to-live for individual tools (0 = no TTL)
	ToolTTL time.Duration
	// EnableToolLRU enables LRU eviction for tools when MaxTools is reached
	EnableToolLRU bool
	// ToolCleanupInterval is how often to run TTL cleanup for tools
	ToolCleanupInterval time.Duration
	// EnableBackgroundToolCleanup enables automatic TTL-based tool cleanup
	EnableBackgroundToolCleanup bool
}

// DefaultRegistryConfig returns the default configuration.
func DefaultRegistryConfig() *RegistryConfig {
	return &RegistryConfig{
		EnableHotReloading:             false,
		HotReloadInterval:              30 * time.Second,
		MaxDependencyDepth:             10,
		ConflictResolutionStrategy:     ConflictStrategyError,
		EnableVersionMigration:         true,
		MigrationTimeout:               30 * time.Second,
		LoadingTimeout:                 10 * time.Second,
		EnableCaching:                  true,
		CacheExpiration:                5 * time.Minute,
		MaxTools:                       10000,                // Limit to 10k tools
		MaxMemoryUsage:                 100 * 1024 * 1024,    // 100MB memory limit
		MaxConcurrentRegistrations:     10,                   // Max 10 concurrent registrations
		ToolTTL:                        2 * time.Hour,        // 2 hour TTL for tools
		EnableToolLRU:                  true,                 // Enable LRU eviction
		ToolCleanupInterval:            15 * time.Minute,     // Cleanup every 15 minutes
		EnableBackgroundToolCleanup:    true,                 // Enable background cleanup
	}
}

// ConflictStrategy defines how to handle tool conflicts when registering a tool
// with an ID or name that already exists in the registry.
type ConflictStrategy int

const (
	// ConflictStrategyError returns an error when a conflict is detected (default)
	ConflictStrategyError ConflictStrategy = iota
	// ConflictStrategyOverwrite replaces the existing tool with the new one
	ConflictStrategyOverwrite
	// ConflictStrategySkip keeps the existing tool and ignores the new one
	ConflictStrategySkip
	// ConflictStrategyVersionBased uses semantic versioning to decide (keeps newer version)
	ConflictStrategyVersionBased
	// ConflictStrategyPriorityBased uses tool metadata priority field to decide
	ConflictStrategyPriorityBased
)

// PaginationOptions defines options for paginated operations.
type PaginationOptions struct {
	// Page is the page number (1-based)
	Page int
	// Size is the number of items per page
	Size int
	// SortBy specifies the field to sort by
	SortBy string
	// SortOrder specifies the sort direction (asc/desc)
	SortOrder string
}

// PaginatedResult contains paginated results with metadata.
type PaginatedResult struct {
	// Tools contains the tools for the current page
	Tools []ReadOnlyTool
	// TotalCount is the total number of tools matching the filter
	TotalCount int
	// Page is the current page number
	Page int
	// Size is the page size
	Size int
	// TotalPages is the total number of pages
	TotalPages int
	// HasNext indicates if there's a next page
	HasNext bool
	// HasPrevious indicates if there's a previous page
	HasPrevious bool
}

// ListCache provides fast caching for list operations.
type ListCache struct {
	mu    sync.RWMutex
	cache map[string]*CachedListResult
	// LRU tracking
	accessOrder []string
	maxSize     int
	size        int
}

// CachedListResult represents a cached list result with expiration.
type CachedListResult struct {
	Result    *PaginatedResult
	ExpiresAt time.Time
	Filter    *ToolFilter
	Options   *PaginationOptions
}

// SchemaCache provides LRU caching for compiled schemas.
type SchemaCache struct {
	mu       sync.RWMutex
	cache    map[string]*CachedSchema
	order    []string
	maxSize  int
	size     int
	hitCount int64
	missCount int64
}

// CachedSchema represents a cached compiled schema.
type CachedSchema struct {
	Validator *SchemaValidator
	Schema    *ToolSchema
	Hash      string
	CreatedAt time.Time
	AccessCount int64
}

// MemoryPool provides object pooling for frequently allocated objects.
type MemoryPool struct {
	toolPool       sync.Pool
	resultPool     sync.Pool
	filterPool     sync.Pool
	stringSlicePool sync.Pool
	mapPool        sync.Pool
}

// ToolWrapper provides copy-on-write semantics for tools.
type ToolWrapper struct {
	tool      *Tool
	refCount  int32
	copyOnWrite bool
	mu        sync.RWMutex
}

// ConflictResolver defines a function that resolves tool conflicts.
// It receives the existing and new tools and returns the tool to keep,
// or nil to skip registration. Custom resolvers are called before
// the built-in conflict resolution strategy.
//
// Example:
//
//	func timestampResolver(existing, new *Tool) (*Tool, error) {
//		if getTimestamp(new) > getTimestamp(existing) {
//			return new, nil
//		}
//		return existing, nil
//	}
type ConflictResolver func(existing *Tool, new *Tool) (*Tool, error)

// MigrationHandler defines a function that handles tool version migrations.
// It is called when a tool with the same ID but different version is registered.
// The handler can perform data migration, compatibility checks, or other
// version-specific operations.
//
// Example:
//
//	func migrateV1ToV2(ctx context.Context, oldTool, newTool *Tool) error {
//		// Perform migration logic
//		return nil
//	}
type MigrationHandler func(ctx context.Context, oldTool, newTool *Tool) error

// DynamicLoader defines a function that loads tools from external sources.
// It enables runtime tool discovery and loading from files, URLs, or other systems.
// The function should return a slice of tools loaded from the specified source.
//
// Example:
//
//	func loadFromAPI(ctx context.Context, source string) ([]*Tool, error) {
//		// Fetch and parse tools from API endpoint
//		return tools, nil
//	}
type DynamicLoader func(ctx context.Context, source string) ([]*Tool, error)

// LoadingStrategy defines how tools are loaded and cached in the registry.
type LoadingStrategy int

const (
	// LoadingStrategyImmediate loads tools synchronously when requested
	LoadingStrategyImmediate LoadingStrategy = iota
	// LoadingStrategyLazy defers loading until the tool is actually used
	LoadingStrategyLazy
	// LoadingStrategyPreemptive loads tools in advance based on usage patterns
	LoadingStrategyPreemptive
)

// CategoryTree represents a hierarchical category structure for tools.
// It enables organizing tools into nested categories with inheritance
// of properties like tags, capabilities, and metadata.
//
// Example structure:
//
//	root
//	├── data-processing
//	│   ├── transformation
//	│   └── validation
//	└── communication
//	    ├── email
//	    └── messaging
type CategoryTree struct {
	mu     sync.RWMutex
	root   *CategoryNode
	index  map[string]*CategoryNode
	tools  map[string]map[string]bool // category -> tool IDs
}

// CategoryNode represents a node in the category tree.
// Each node can have child categories and associated tools.
// Properties can be inherited from parent categories based on
// the inheritance configuration.
type CategoryNode struct {
	Name        string
	Path        string
	Parent      *CategoryNode
	Children    map[string]*CategoryNode
	Metadata    map[string]interface{}
	Inheritance *CategoryInheritance
}

// CategoryInheritance defines how categories inherit properties from their parents.
// This enables consistent behavior and metadata across category hierarchies.
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
// It enables hot-reloading of tool definitions from JSON files,
// automatically updating the registry when files are modified.
type FileWatcher struct {
	mu       sync.Mutex
	path     string
	modTime  time.Time
	callback func(string) error
	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup // Track goroutine lifecycle
}

// DependencyGraph manages tool dependencies and resolution.
// It tracks which tools depend on others, enforces version constraints,
// and detects circular dependencies. The graph supports both required
// and optional dependencies with transitive resolution.
type DependencyGraph struct {
	mu           sync.RWMutex
	dependencies map[string]map[string]*DependencyConstraint
	cache        sync.Map // Use sync.Map for thread-safe caching
}

// DependencyConstraint defines a dependency with version constraints.
// It specifies which version of a tool is required, whether the
// dependency is optional, and if transitive dependencies should be resolved.
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
		// cache initialized as sync.Map (zero value)
	}
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return NewRegistryWithConfig(DefaultRegistryConfig())
}

// NewRegistryWithConfig creates a new tool registry with custom configuration.
func NewRegistryWithConfig(config *RegistryConfig) *Registry {
	if config == nil {
		config = DefaultRegistryConfig()
	}
	
	// Ensure cleanup interval is positive if background cleanup is enabled
	if config.EnableBackgroundToolCleanup && config.ToolCleanupInterval <= 0 {
		config.ToolCleanupInterval = 15 * time.Minute // Use default value
	}
	
	r := &Registry{
		tools:             make(map[string]*ToolRegistryEntry),
		categoryIndex:     make(map[string]map[string]bool),
		tagIndex:          make(map[string]map[string]bool),
		nameIndex:         make(map[string]string),
		toolsLRU:          list.New(),
		toolsIndex:        make(map[string]*list.Element),
		validators:        []RegistryValidator{},
		categories:        NewCategoryTree(),
		conflictResolvers: []ConflictResolver{},
		migrationHandlers: make(map[string]MigrationHandler),
		dynamicLoaders:    make(map[string]DynamicLoader),
		// watchers initialized as sync.Map (zero value)
		dependencyGraph:   NewDependencyGraph(),
		loadingStrategies: make(map[string]LoadingStrategy),
		config:            config,
		// Performance optimization components
		listCache:        NewListCache(),
		schemaCache:      NewSchemaCache(),
		memoryPool:       NewMemoryPool(),
		toolCleanupStop:  make(chan struct{}),
	}
	
	// Start background cleanup if enabled
	if config.EnableBackgroundToolCleanup && config.ToolCleanupInterval > 0 {
		go r.backgroundToolCleanup()
	}
	
	return r
}

// Register adds a new tool to the registry.
// It returns an error if the tool is invalid or if a tool with the same ID already exists.
func (r *Registry) Register(tool *Tool) error {
	return r.RegisterWithContext(context.Background(), tool)
}

// RegisterWithContext adds a new tool to the registry with context support.
// This implementation fixes the TOCTOU race condition by holding the write lock
// for the entire registration process.
func (r *Registry) RegisterWithContext(ctx context.Context, tool *Tool) error {
	if tool == nil {
		return NewToolError(ErrorTypeValidation, "NIL_TOOL", "tool cannot be nil")
	}

	// Check context cancellation early
	if err := ctx.Err(); err != nil {
		return NewToolError(ErrorTypeInternal, "CONTEXT_CANCELLED", "context was cancelled").WithCause(err)
	}

	// Check concurrency limits before acquiring lock
	if r.config.MaxConcurrentRegistrations > 0 {
		if current := atomic.LoadInt32(&r.activeRegistrations); current >= r.config.MaxConcurrentRegistrations {
			return NewToolError(ErrorTypeResource, "CONCURRENT_REGISTRATIONS_EXCEEDED", 
				fmt.Sprintf("maximum concurrent registrations (%d) exceeded", r.config.MaxConcurrentRegistrations))
		}
	}

	// Increment active registrations counter
	atomic.AddInt32(&r.activeRegistrations, 1)
	defer atomic.AddInt32(&r.activeRegistrations, -1)

	// Validate the tool
	if err := tool.Validate(); err != nil {
		return NewToolError(ErrorTypeValidation, "VALIDATION_FAILED", "tool validation failed").
		WithToolID(tool.ID).
		WithCause(err)
	}

	// Run custom validators with hook protection
	r.hookMu.RLock()
	validators := append([]RegistryValidator{}, r.validators...)
	r.hookMu.RUnlock()

	for _, validator := range validators {
		if err := validator(tool); err != nil {
			return NewToolError(ErrorTypeValidation, "CUSTOM_VALIDATION_FAILED", "custom validation failed").
			WithToolID(tool.ID).
			WithCause(err)
		}
	}

	// Acquire write lock for the entire registration process to prevent TOCTOU races
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check resource limits while holding the lock
	if r.config.MaxTools > 0 && len(r.tools) >= r.config.MaxTools {
		// If LRU is enabled, evict the least recently used tool to make room
		if r.config.EnableToolLRU {
			r.evictLRUTool()
		} else {
			return NewToolError(ErrorTypeResource, "MAX_TOOLS_EXCEEDED", 
				fmt.Sprintf("maximum number of tools (%d) exceeded", r.config.MaxTools))
		}
	}

	// Check memory usage limits
	if r.config.MaxMemoryUsage > 0 {
		estimatedSize := r.estimateToolMemoryUsage(tool)
		if r.currentMemoryUsage+estimatedSize > r.config.MaxMemoryUsage {
			return NewToolError(ErrorTypeResource, "MEMORY_LIMIT_EXCEEDED", 
				fmt.Sprintf("memory limit (%d bytes) would be exceeded by %d bytes", 
					r.config.MaxMemoryUsage, estimatedSize))
		}
	}

	// Check for existing tools while holding the write lock
	existingEntry, idExists := r.tools[tool.ID]
	existingID, nameExists := r.nameIndex[tool.Name]
	var existingByName *Tool
	if nameExists && existingID != tool.ID {
		if existingEntryByName, exists := r.tools[existingID]; exists {
			existingByName = existingEntryByName.tool
		}
	}
	
	var existingTool *Tool
	if idExists {
		existingTool = existingEntry.tool
	}

	// Resolve conflicts while holding the lock
	if idExists {
		resolvedTool, err := r.resolveConflict(ctx, existingTool, tool)
		if err != nil {
			return NewToolError(ErrorTypeInternal, "CONFLICT_RESOLUTION_FAILED", "conflict resolution failed").
			WithToolID(tool.ID).
			WithCause(err)
		}
		if resolvedTool == nil {
			// Conflict resolution decided to skip registration
			return nil
		}
		tool = resolvedTool
	}

	// Check for name conflicts
	if existingByName != nil {
		resolvedTool, err := r.resolveConflict(ctx, existingByName, tool)
		if err != nil {
			return NewConflictError(CodeNameConflict, "name conflict resolution failed", existingByName.ID, tool.ID).
				WithCause(err)
		}
		if resolvedTool == nil {
			return nil
		}
		tool = resolvedTool
	}

	// Handle version migration if enabled
	if r.config.EnableVersionMigration {
		if err := r.handleVersionMigration(ctx, tool); err != nil {
			return NewMigrationError(CodeMigrationFailed, "version migration failed", "", tool.Version).
				WithToolID(tool.ID).
				WithCause(err)
		}
	}

	
	// Store a clone to prevent external modifications
	clonedTool := tool.Clone()
	estimatedSize := r.estimateToolMemoryUsage(clonedTool)
	
	// Create registry entry with metadata
	entry := &ToolRegistryEntry{
		tool:        clonedTool,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}
	r.tools[tool.ID] = entry
	r.currentMemoryUsage += estimatedSize
	
	// Update LRU tracking
	if r.config.EnableToolLRU {
		elem := r.toolsLRU.PushFront(tool.ID)
		r.toolsIndex[tool.ID] = elem
	}

	// Update indexes
	r.nameIndex[clonedTool.Name] = clonedTool.ID

	// Update tag index
	if clonedTool.Metadata != nil && len(clonedTool.Metadata.Tags) > 0 {
		for _, tag := range clonedTool.Metadata.Tags {
			if r.tagIndex[tag] == nil {
				r.tagIndex[tag] = make(map[string]bool)
			}
			r.tagIndex[tag][clonedTool.ID] = true
		}
	}

	// Update category tree
	if err := r.updateCategoryTree(clonedTool); err != nil {
		// Log error but don't fail registration
	}

	// Update dependency graph
	if err := r.dependencyGraph.AddTool(clonedTool); err != nil {
		return NewDependencyError(CodeDependencyNotFound, "dependency graph update failed", clonedTool.ID).
			WithCause(err)
	}

	// Update category tree
	if err := r.updateCategoryTree(tool); err != nil {
		// Log error but don't fail registration
		// In production, you might want to handle this differently
	}

	// Update dependency graph
	if err := r.dependencyGraph.AddTool(tool); err != nil {
		return NewDependencyError(CodeDependencyNotFound, "dependency graph update failed", tool.ID).
			WithCause(err)
	}

	// Invalidate caches after successful registration
	r.invalidateListCache()

	return nil
}

// Unregister removes a tool from the registry.
// It returns an error if the tool is not found.
func (r *Registry) Unregister(toolID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.tools[toolID]
	if !exists {
		return NewToolError(ErrorTypeValidation, "TOOL_NOT_FOUND", "tool not found").
		WithToolID(toolID)
	}

	// Calculate memory usage to be freed
	estimatedSize := r.estimateToolMemoryUsage(entry.tool)

	// Remove from main storage
	delete(r.tools, toolID)
	r.currentMemoryUsage -= estimatedSize
	if r.currentMemoryUsage < 0 {
		r.currentMemoryUsage = 0 // Prevent negative values
	}

	// Remove from LRU tracking
	if r.config.EnableToolLRU {
		if elem, exists := r.toolsIndex[toolID]; exists {
			r.toolsLRU.Remove(elem)
			delete(r.toolsIndex, toolID)
		}
	}

	// Remove from name index
	delete(r.nameIndex, entry.tool.Name)

	// Remove from tag index
	if entry.tool.Metadata != nil && len(entry.tool.Metadata.Tags) > 0 {
		for _, tag := range entry.tool.Metadata.Tags {
			if tagMap := r.tagIndex[tag]; tagMap != nil {
				delete(tagMap, toolID)
				if len(tagMap) == 0 {
					delete(r.tagIndex, tag)
				}
			}
		}
	}

	// Invalidate caches after successful unregistration
	r.invalidateListCache()

	return nil
}

// Get retrieves a tool by its ID.
// It returns nil if the tool is not found.
// This method returns a clone for backward compatibility.
func (r *Registry) Get(toolID string) (*Tool, error) {
	if r == nil {
		return nil, NewToolError(ErrorTypeValidation, "REGISTRY_NIL", "registry is nil")
	}
	if toolID == "" {
		return nil, NewToolError(ErrorTypeValidation, "EMPTY_TOOL_ID", "tool ID cannot be empty")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.tools == nil {
		return nil, NewToolError(ErrorTypeInternal, "TOOLS_MAP_NIL", "tools map is nil")
	}

	entry, exists := r.tools[toolID]
	if !exists {
		return nil, NewToolError(ErrorTypeValidation, "TOOL_NOT_FOUND", "tool not found").
			WithToolID(toolID)
	}

	if entry == nil || entry.tool == nil {
		return nil, NewToolError(ErrorTypeInternal, "TOOL_NIL", "stored tool is nil").
			WithToolID(toolID)
	}

	// Update access tracking
	entry.lastAccess = time.Now()
	atomic.AddInt64(&entry.accessCount, 1)

	// Update LRU position if enabled
	if r.config.EnableToolLRU {
		if elem, found := r.toolsIndex[toolID]; found {
			r.toolsLRU.MoveToFront(elem)
		}
	}

	// Return a clone to prevent external modifications
	return entry.tool.Clone(), nil
}

// GetReadOnly retrieves a read-only view of a tool by its ID.
// This is more memory-efficient than Get() as it avoids cloning.
func (r *Registry) GetReadOnly(toolID string) (ReadOnlyTool, error) {
	if r == nil {
		return nil, NewToolError(ErrorTypeValidation, "REGISTRY_NIL", "registry is nil")
	}
	if toolID == "" {
		return nil, NewToolError(ErrorTypeValidation, "EMPTY_TOOL_ID", "tool ID cannot be empty")
	}

	// First, read the tool with read lock
	r.mu.RLock()
	if r.tools == nil {
		r.mu.RUnlock()
		return nil, NewToolError(ErrorTypeInternal, "TOOLS_MAP_NIL", "tools map is nil")
	}

	entry, exists := r.tools[toolID]
	if !exists {
		r.mu.RUnlock()
		return nil, NewToolError(ErrorTypeValidation, "TOOL_NOT_FOUND", "tool not found").
			WithToolID(toolID)
	}

	if entry == nil || entry.tool == nil {
		r.mu.RUnlock()
		return nil, NewToolError(ErrorTypeInternal, "TOOL_NIL", "stored tool is nil").
			WithToolID(toolID)
	}

	// Get the tool reference while holding read lock
	tool := entry.tool
	needsLRUUpdate := r.config.EnableToolLRU
	r.mu.RUnlock()

	// Update access tracking atomically (safe without lock)
	atomic.AddInt64(&entry.accessCount, 1)

	// Update access time and LRU position with write lock only if needed
	if needsLRUUpdate {
		r.mu.Lock()
		// Double-check entry still exists after acquiring write lock
		if currentEntry, stillExists := r.tools[toolID]; stillExists && currentEntry == entry {
			currentEntry.lastAccess = time.Now()
			if elem, found := r.toolsIndex[toolID]; found {
				r.toolsLRU.MoveToFront(elem)
			}
		}
		r.mu.Unlock()
	} else {
		// For non-LRU case, just update lastAccess atomically
		// We can't update time.Time atomically, so we use a separate lock-free approach
		// or accept that lastAccess might not be perfectly accurate in high-concurrency scenarios
		r.mu.Lock()
		if currentEntry, stillExists := r.tools[toolID]; stillExists && currentEntry == entry {
			currentEntry.lastAccess = time.Now()
		}
		r.mu.Unlock()
	}

	// Return a read-only view without cloning
	return NewReadOnlyTool(tool), nil
}

// GetByName retrieves a tool by its name.
// It returns nil if the tool is not found.
// This method returns a clone for backward compatibility.
func (r *Registry) GetByName(name string) (*Tool, error) {
	if r == nil {
		return nil, NewValidationError(CodeToolNotFound, "registry is nil", "")
	}
	if name == "" {
		return nil, NewValidationError(CodeToolNotFound, "tool name cannot be empty", "")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.nameIndex == nil {
		return nil, NewValidationError(CodeToolNotFound, "name index is nil", "")
	}

	toolID, exists := r.nameIndex[name]
	if !exists {
		return nil, NewValidationError(CodeToolNotFound, fmt.Sprintf("tool with name %q not found", name), "")
	}

	if r.tools == nil {
		return nil, NewValidationError(CodeToolNotFound, "tools map is nil", toolID)
	}

	entry := r.tools[toolID]
	if entry == nil || entry.tool == nil {
		return nil, NewValidationError(CodeToolNotFound, fmt.Sprintf("tool with name %q has nil tool reference", name), toolID)
	}

	// Update access tracking
	entry.lastAccess = time.Now()
	atomic.AddInt64(&entry.accessCount, 1)

	// Update LRU position if enabled
	if r.config.EnableToolLRU {
		if elem, found := r.toolsIndex[toolID]; found {
			r.toolsLRU.MoveToFront(elem)
		}
	}

	return entry.tool.Clone(), nil
}

// GetByNameReadOnly retrieves a read-only view of a tool by its name.
// This is more memory-efficient than GetByName() as it avoids cloning.
func (r *Registry) GetByNameReadOnly(name string) (ReadOnlyTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	toolID, exists := r.nameIndex[name]
	if !exists {
		return nil, NewValidationError(CodeToolNotFound, fmt.Sprintf("tool with name %q not found", name), "")
	}

	entry := r.tools[toolID]
	if entry == nil || entry.tool == nil {
		return nil, NewValidationError(CodeToolNotFound, fmt.Sprintf("tool with name %q has nil tool reference", name), toolID)
	}

	// Update access tracking
	entry.lastAccess = time.Now()
	atomic.AddInt64(&entry.accessCount, 1)

	// Update LRU position if enabled
	if r.config.EnableToolLRU {
		if elem, found := r.toolsIndex[toolID]; found {
			r.toolsLRU.MoveToFront(elem)
		}
	}

	return NewReadOnlyTool(entry.tool), nil
}

// List returns all tools that match the given filter.
// If filter is nil, all tools are returned.
// This method returns clones for backward compatibility.
func (r *Registry) List(filter *ToolFilter) ([]*Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*Tool

	for _, entry := range r.tools {
		if entry != nil && entry.tool != nil {
			if filter == nil || r.matchesFilter(entry.tool, filter) {
				results = append(results, entry.tool.Clone())
			}
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

	for _, entry := range r.tools {
		if entry != nil && entry.tool != nil {
			if filter == nil || r.matchesFilter(entry.tool, filter) {
				results = append(results, NewReadOnlyTool(entry.tool))
			}
		}
	}

	return results, nil
}

// ListPaginated returns paginated results for tools matching the filter.
// This method provides efficient pagination with caching and optimized filtering.
func (r *Registry) ListPaginated(filter *ToolFilter, options *PaginationOptions) (*PaginatedResult, error) {
	// Set default pagination options
	if options == nil {
		options = &PaginationOptions{
			Page:      1,
			Size:      50,
			SortBy:    "name",
			SortOrder: "asc",
		}
	}

	// Validate options
	if options.Page < 1 {
		options.Page = 1
	}
	if options.Size < 1 || options.Size > 1000 {
		options.Size = 50
	}
	if options.SortBy == "" {
		options.SortBy = "name"
	}
	if options.SortOrder != "asc" && options.SortOrder != "desc" {
		options.SortOrder = "asc"
	}

	// Check cache first
	cacheKey := r.generateListCacheKey(filter, options)
	if cached, found := r.listCache.Get(cacheKey); found {
		return cached.Result, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Get filtered tools using optimized approach
	filteredTools := r.getFilteredToolsOptimized(filter)

	// Sort tools
	r.sortTools(filteredTools, options.SortBy, options.SortOrder)

	// Calculate pagination
	totalCount := len(filteredTools)
	totalPages := (totalCount + options.Size - 1) / options.Size
	startIndex := (options.Page - 1) * options.Size
	endIndex := startIndex + options.Size
	
	if endIndex > totalCount {
		endIndex = totalCount
	}

	// Create paginated result
	result := &PaginatedResult{
		TotalCount:  totalCount,
		Page:        options.Page,
		Size:        options.Size,
		TotalPages:  totalPages,
		HasNext:     options.Page < totalPages,
		HasPrevious: options.Page > 1,
	}

	// Get tools for current page
	if startIndex < totalCount {
		pageTools := filteredTools[startIndex:endIndex]
		result.Tools = make([]ReadOnlyTool, len(pageTools))
		for i, tool := range pageTools {
			result.Tools[i] = NewReadOnlyTool(tool)
		}
	} else {
		result.Tools = []ReadOnlyTool{}
	}

	// Cache the result
	r.listCache.Set(cacheKey, &CachedListResult{
		Result:    result,
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Filter:    filter,
		Options:   options,
	})

	return result, nil
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

	r.tools = make(map[string]*ToolRegistryEntry)
	r.categoryIndex = make(map[string]map[string]bool)
	r.tagIndex = make(map[string]map[string]bool)
	r.nameIndex = make(map[string]string)
}

// AddValidator adds a custom validation function that will be run
// during tool registration.
func (r *Registry) AddValidator(validator RegistryValidator) {
	if r == nil || validator == nil {
		return // Silently ignore to prevent panics
	}
	
	r.hookMu.Lock()
	defer r.hookMu.Unlock()
	
	if r.validators == nil {
		r.validators = []RegistryValidator{}
	}
	
	r.validators = append(r.validators, validator)
}

// Validate runs validation on all registered tools.
// This is useful for ensuring registry consistency.
func (r *Registry) Validate() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for id, entry := range r.tools {
		if entry == nil || entry.tool == nil {
			return NewValidationError(CodeValidationFailed, fmt.Sprintf("tool entry %q is nil", id), id)
		}

		if err := entry.tool.Validate(); err != nil {
			return NewValidationError(CodeValidationFailed, fmt.Sprintf("tool %q validation failed", id), id).
				WithCause(err)
		}

		for _, validator := range r.validators {
			if err := validator(entry.tool); err != nil {
				return NewValidationError(CodeCustomValidationFailed, fmt.Sprintf("tool %q custom validation failed", id), id).
					WithCause(err)
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

	entry, exists := r.tools[toolID]
	if !exists {
		return nil, NewToolError(ErrorTypeValidation, "TOOL_NOT_FOUND", "tool not found").
			WithToolID(toolID)
	}

	if entry == nil || entry.tool == nil {
		return nil, NewToolError(ErrorTypeInternal, "TOOL_NIL", "stored tool is nil").
			WithToolID(toolID)
	}

	if entry.tool.Metadata == nil || len(entry.tool.Metadata.Dependencies) == 0 {
		return []*Tool{}, nil
	}

	var dependencies []*Tool
	for _, depID := range entry.tool.Metadata.Dependencies {
		depEntry, exists := r.tools[depID]
		if !exists {
			return nil, NewDependencyError(CodeDependencyNotFound, fmt.Sprintf("dependency %q not found for tool %q", depID, toolID), toolID)
		}
		if depEntry == nil || depEntry.tool == nil {
			return nil, NewDependencyError(CodeDependencyNotFound, fmt.Sprintf("dependency %q has nil tool reference", depID), toolID)
		}
		dependencies = append(dependencies, depEntry.tool.Clone())
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

		var t *Tool
		if entry, exists := r.tools[toolID]; exists {
			if entry != nil && entry.tool != nil {
				t = entry.tool
			}
		} else if toolID == tool.ID {
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
	for id, entry := range r.tools {
		if entry != nil && entry.tool != nil {
			export[id] = entry.tool.Clone()
		}
	}
	return export
}

// ImportTools bulk imports tools into the registry.
// It returns a slice of errors for any tools that failed to import.
func (r *Registry) ImportTools(tools map[string]*Tool) []error {
	var errors []error

	for _, tool := range tools {
		if err := r.Register(tool); err != nil {
			errors = append(errors, NewIOError(CodeRegistrationFailed, fmt.Sprintf("failed to import tool %q", tool.ID), tool.ID, err))
		}
	}

	return errors
}

// Enhanced Methods for New Features

// resolveConflict resolves conflicts between existing and new tools.
func (r *Registry) resolveConflict(ctx context.Context, existing, new *Tool) (*Tool, error) {
	// Apply custom conflict resolvers first with hook protection
	r.hookMu.RLock()
	resolvers := append([]ConflictResolver{}, r.conflictResolvers...)
	r.hookMu.RUnlock()
	
	for _, resolver := range resolvers {
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
		return nil, NewConflictError(CodeConflictResolutionFailed, fmt.Sprintf("tool with ID %q already exists", existing.ID), existing.ID, new.ID)
	case ConflictStrategyOverwrite:
		return new, nil
	case ConflictStrategySkip:
		return nil, nil // Skip registration
	case ConflictStrategyVersionBased:
		return r.resolveVersionBasedConflict(existing, new)
	case ConflictStrategyPriorityBased:
		return r.resolvePriorityBasedConflict(existing, new)
	default:
		return nil, NewToolError(ErrorTypeConfiguration, CodeUnknownConflictStrategy, fmt.Sprintf("unknown conflict resolution strategy: %v", r.config.ConflictResolutionStrategy))
	}
}

// resolveVersionBasedConflict resolves conflicts based on version comparison.
func (r *Registry) resolveVersionBasedConflict(existing, new *Tool) (*Tool, error) {
	matches, err := matchesVersionConstraint(new.Version, ">"+existing.Version)
	if err != nil {
		return nil, NewConflictError(CodeVersionComparisonFailed, "version comparison failed", existing.ID, new.ID).
			WithCause(err)
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
	if entry, exists := r.tools[tool.ID]; exists {
		if entry == nil || entry.tool == nil {
			return NewMigrationError(CodeMigrationFailed, "existing tool is nil", "", tool.Version).
				WithToolID(tool.ID)
		}
		
		if handler, exists := r.migrationHandlers[entry.tool.Version]; exists {
			return handler(ctx, entry.tool, tool)
		}
		
		// Default migration behavior
		return r.defaultVersionMigration(ctx, entry.tool, tool)
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
		return NewMigrationError(CodeMigrationCompatibilityFailed, "migration compatibility check failed", oldTool.Version, newTool.Version).
			WithToolID(newTool.ID).
			WithCause(err)
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
				return NewMigrationError(CodeParameterRemoved, fmt.Sprintf("required parameter %q removed in new version", req), oldTool.Version, newTool.Version).
					WithToolID(newTool.ID)
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
	if r == nil || resolver == nil {
		return // Silently ignore to prevent panics
	}
	
	r.hookMu.Lock()
	defer r.hookMu.Unlock()
	
	if r.conflictResolvers == nil {
		r.conflictResolvers = []ConflictResolver{}
	}
	
	r.conflictResolvers = append(r.conflictResolvers, resolver)
}

// AddMigrationHandler adds a custom migration handler for a specific version.
func (r *Registry) AddMigrationHandler(version string, handler MigrationHandler) {
	if r == nil || version == "" || handler == nil {
		return // Silently ignore to prevent panics
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.migrationHandlers == nil {
		r.migrationHandlers = make(map[string]MigrationHandler)
	}
	
	r.migrationHandlers[version] = handler
}

// LoadFromFile loads tools from a JSON file.
func (r *Registry) LoadFromFile(ctx context.Context, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return NewIOError(CodeFileOpenFailed, fmt.Sprintf("failed to open file %q", filename), filename, err)
	}
	defer file.Close()
	
	return r.LoadFromReader(ctx, file)
}

// LoadFromReader loads tools from a JSON reader.
func (r *Registry) LoadFromReader(ctx context.Context, reader io.Reader) error {
	var tools []*Tool
	decoder := json.NewDecoder(reader)
	
	if err := decoder.Decode(&tools); err != nil {
		return NewIOError(CodeDecodeFailed, "failed to decode tools", "", err)
	}
	
	for _, tool := range tools {
		// Add a default executor if none exists (for tools loaded from JSON)
		if tool.Executor == nil {
			tool.Executor = &DefaultExecutor{}
		}
		
		if err := r.RegisterWithContext(ctx, tool); err != nil {
			return NewIOError(CodeRegistrationFailed, fmt.Sprintf("failed to register tool %q", tool.ID), tool.ID, err)
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
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				},
			},
		},
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return NewNetworkError(CodeRequestCreationFailed, "failed to create request", url, err)
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return NewNetworkError(CodeHTTPRequestFailed, "failed to fetch from URL", url, err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return NewNetworkError(CodeHTTPError, fmt.Sprintf("HTTP error: %s", resp.Status), url, nil).
			WithDetail("status_code", resp.StatusCode)
	}
	
	return r.LoadFromReader(ctx, resp.Body)
}

// WatchFile watches a file for changes and reloads tools.
func (r *Registry) WatchFile(filename string) error {
	if !r.config.EnableHotReloading {
		return NewToolError(ErrorTypeConfiguration, CodeHotReloadingDisabled, "hot reloading is disabled")
	}
	
	// Check if already watching using sync.Map
	if _, loaded := r.watchers.Load(filename); loaded {
		return NewToolError(ErrorTypeConfiguration, CodeFileAlreadyWatched, fmt.Sprintf("file %q is already being watched", filename)).
			WithDetail("filename", filename)
	}
	
	// Get initial file info
	info, err := os.Stat(filename)
	if err != nil {
		return NewIOError(CodeFileOpenFailed, fmt.Sprintf("failed to stat file %q", filename), filename, err)
	}
	
	watcher := &FileWatcher{
		path:    filename,
		modTime: info.ModTime(),
		stop:    make(chan struct{}),
		callback: func(path string) error {
			return r.LoadFromFile(context.Background(), path)
		},
	}
	
	// Use LoadOrStore to prevent race conditions
	_, loaded := r.watchers.LoadOrStore(filename, watcher)
	if loaded {
		// Another goroutine beat us to it
		return NewToolError(ErrorTypeConfiguration, CodeFileAlreadyWatched, fmt.Sprintf("file %q is already being watched", filename)).
			WithDetail("filename", filename)
	}
	
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
	value, exists := r.watchers.LoadAndDelete(filename)
	if !exists {
		return NewToolError(ErrorTypeConfiguration, CodeFileNotWatched, fmt.Sprintf("file %q is not being watched", filename)).
			WithDetail("filename", filename)
	}
	
	watcher := value.(*FileWatcher)
	
	// Use sync.Once to ensure stop is closed only once
	watcher.stopOnce.Do(func() {
		close(watcher.stop)
	})
	
	// Wait for goroutine to complete
	watcher.wg.Wait()
	
	return nil
}

// GetDependenciesWithConstraints returns all tools that match the dependency constraints.
func (r *Registry) GetDependenciesWithConstraints(toolID string) ([]*Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Convert ToolRegistryEntry map to Tool map for the dependency graph
	toolsMap := make(map[string]*Tool)
	for id, entry := range r.tools {
		if entry != nil && entry.tool != nil {
			toolsMap[id] = entry.tool
		}
	}
	
	return r.dependencyGraph.ResolveDependencies(toolID, toolsMap)
}

// GetByCategory returns all tools in a category.
func (r *Registry) GetByCategory(category string) ([]*Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	toolIDs := r.categories.GetToolsInCategory(category)
	tools := make([]*Tool, 0, len(toolIDs))
	
	for _, toolID := range toolIDs {
		if entry, exists := r.tools[toolID]; exists {
			if entry != nil && entry.tool != nil {
				tools = append(tools, entry.tool.Clone())
			}
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
	if r == nil {
		return // Silently ignore to prevent panics
	}
	if config == nil {
		config = DefaultRegistryConfig()
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = config
}

// GetConfig returns the current registry configuration.
func (r *Registry) GetConfig() *RegistryConfig {
	if r == nil {
		return DefaultRegistryConfig()
	}
	
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	if r.config == nil {
		return DefaultRegistryConfig()
	}
	
	return r.config
}

// Performance optimization helper methods

// getFilteredToolsOptimized efficiently filters tools using indexes.
func (r *Registry) getFilteredToolsOptimized(filter *ToolFilter) []*Tool {
	if filter == nil {
		// Return all tools
		result := make([]*Tool, 0, len(r.tools))
		for _, entry := range r.tools {
			if entry != nil && entry.tool != nil {
				result = append(result, entry.tool)
			}
		}
		return result
	}

	// Use tag index for efficient filtering if tags are specified
	var candidateTools map[string]*Tool
	if len(filter.Tags) > 0 {
		candidateTools = r.getToolsByTags(filter.Tags)
	} else {
		// Convert ToolRegistryEntry map to Tool map
		candidateTools = make(map[string]*Tool)
		for id, entry := range r.tools {
			if entry != nil && entry.tool != nil {
				candidateTools[id] = entry.tool
			}
		}
	}

	// Apply remaining filters
	result := make([]*Tool, 0, len(candidateTools))
	for _, tool := range candidateTools {
		if r.matchesFilter(tool, filter) {
			result = append(result, tool)
		}
	}

	return result
}

// getToolsByTags efficiently retrieves tools by tags using the tag index.
func (r *Registry) getToolsByTags(tags []string) map[string]*Tool {
	if len(tags) == 0 {
		// Convert ToolRegistryEntry map to Tool map
		result := make(map[string]*Tool)
		for id, entry := range r.tools {
			if entry != nil && entry.tool != nil {
				result[id] = entry.tool
			}
		}
		return result
	}

	// Find tools that have all required tags
	var result map[string]*Tool
	for i, tag := range tags {
		toolsWithTag := r.tagIndex[tag]
		if toolsWithTag == nil {
			// No tools have this tag
			return make(map[string]*Tool)
		}

		if i == 0 {
			// First tag - initialize result
			result = make(map[string]*Tool)
			for toolID := range toolsWithTag {
				if entry, exists := r.tools[toolID]; exists {
					if entry != nil && entry.tool != nil {
						result[toolID] = entry.tool
					}
				}
			}
		} else {
			// Subsequent tags - intersect with existing result
			for toolID := range result {
				if !toolsWithTag[toolID] {
					delete(result, toolID)
				}
			}
		}
	}

	return result
}

// sortTools sorts tools by the specified field and order.
func (r *Registry) sortTools(tools []*Tool, sortBy, sortOrder string) {
	sort.Slice(tools, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "name":
			less = tools[i].Name < tools[j].Name
		case "id":
			less = tools[i].ID < tools[j].ID
		case "version":
			less = tools[i].Version < tools[j].Version
		case "description":
			less = tools[i].Description < tools[j].Description
		default:
			less = tools[i].Name < tools[j].Name
		}

		if sortOrder == "desc" {
			return !less
		}
		return less
	})
}

// generateListCacheKey generates a cache key for list operations.
func (r *Registry) generateListCacheKey(filter *ToolFilter, options *PaginationOptions) string {
	key := fmt.Sprintf("list:%d:%d:%s:%s", options.Page, options.Size, options.SortBy, options.SortOrder)
	if filter != nil {
		key += fmt.Sprintf(":%s:%v:%s:%s:%v", filter.Name, filter.Tags, filter.Category, filter.Version, filter.Keywords)
	}
	return key
}

// invalidateListCache invalidates the list cache after tool changes.
func (r *Registry) invalidateListCache() {
	r.listCache.Clear()
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
	// Check cache first using sync.Map
	if cached, exists := dg.cache.Load(toolID); exists {
		return cached.([]*Tool), nil
	}
	
	dg.mu.RLock()
	defer dg.mu.RUnlock()
	
	visited := make(map[string]bool)
	var result []*Tool
	
	if err := dg.resolveDependenciesRecursive(toolID, tools, visited, &result, 0, 10); err != nil {
		return nil, err
	}
	
	// Cache the result using sync.Map
	dg.cache.Store(toolID, result)
	return result, nil
}

// resolveDependenciesRecursive recursively resolves dependencies.
func (dg *DependencyGraph) resolveDependenciesRecursive(toolID string, tools map[string]*Tool, visited map[string]bool, result *[]*Tool, depth, maxDepth int) error {
	if depth > maxDepth {
		return NewDependencyError(CodeDependencyDepthExceeded, fmt.Sprintf("dependency resolution depth exceeded for tool %q", toolID), toolID).
			WithDetail("depth", depth).
			WithDetail("max_depth", maxDepth)
	}
	
	if visited[toolID] {
		return NewDependencyError(CodeCircularDependency, fmt.Sprintf("circular dependency detected for tool %q", toolID), toolID)
	}
	
	visited[toolID] = true
	defer func() { visited[toolID] = false }()
	
	dependencies := dg.dependencies[toolID]
	for _, constraint := range dependencies {
		tool, exists := tools[constraint.ToolID]
		if !exists {
			if !constraint.Optional {
				return NewDependencyError(CodeDependencyNotFound, fmt.Sprintf("required dependency %q not found for tool %q", constraint.ToolID, toolID), toolID).
					WithDetail("dependency", constraint.ToolID)
			}
			continue
		}
		
		// Check version constraint
		if constraint.VersionConstraint != "" {
			matches, err := matchesVersionConstraint(tool.Version, constraint.VersionConstraint)
			if err != nil {
				return NewDependencyError(CodeVersionConstraintFailed, fmt.Sprintf("version constraint check failed for %q", constraint.ToolID), toolID).
					WithDetail("dependency", constraint.ToolID).
					WithDetail("constraint", constraint.VersionConstraint).
					WithCause(err)
			}
			if !matches {
				if !constraint.Optional {
					return NewDependencyError(CodeVersionConstraintFailed, fmt.Sprintf("version constraint %q not satisfied for dependency %q", constraint.VersionConstraint, constraint.ToolID), toolID).
						WithDetail("dependency", constraint.ToolID).
						WithDetail("constraint", constraint.VersionConstraint).
						WithDetail("actual_version", tool.Version)
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

// Performance optimization implementations

// NewListCache creates a new list cache with default settings.
func NewListCache() *ListCache {
	return &ListCache{
		cache:   make(map[string]*CachedListResult),
		maxSize: 1000,
		accessOrder: make([]string, 0),
	}
}

// Get retrieves a cached list result if it exists and hasn't expired.
func (lc *ListCache) Get(key string) (*CachedListResult, bool) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	cached, exists := lc.cache[key]
	if !exists {
		return nil, false
	}

	// Check expiration
	if time.Now().After(cached.ExpiresAt) {
		// Remove expired entry
		delete(lc.cache, key)
		lc.removeFromAccessOrder(key)
		return nil, false
	}

	// Move to end of access order
	lc.moveToEnd(key)
	return cached, true
}

// Set stores a cached list result with LRU eviction.
func (lc *ListCache) Set(key string, result *CachedListResult) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// If key already exists, update and move to end
	if _, exists := lc.cache[key]; exists {
		lc.cache[key] = result
		lc.moveToEnd(key)
		return
	}

	// If cache is full, evict least recently used
	if lc.size >= lc.maxSize {
		lc.evictLRU()
	}

	// Add new entry
	lc.cache[key] = result
	lc.accessOrder = append(lc.accessOrder, key)
	lc.size++
}

// Clear removes all cached entries.
func (lc *ListCache) Clear() {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	lc.cache = make(map[string]*CachedListResult)
	lc.accessOrder = make([]string, 0)
	lc.size = 0
}

// moveToEnd moves a key to the end of the access order.
func (lc *ListCache) moveToEnd(key string) {
	lc.removeFromAccessOrder(key)
	lc.accessOrder = append(lc.accessOrder, key)
}

// removeFromAccessOrder removes a key from the access order.
func (lc *ListCache) removeFromAccessOrder(key string) {
	for i, k := range lc.accessOrder {
		if k == key {
			lc.accessOrder = append(lc.accessOrder[:i], lc.accessOrder[i+1:]...)
			break
		}
	}
}

// evictLRU removes the least recently used entry.
func (lc *ListCache) evictLRU() {
	if len(lc.accessOrder) == 0 {
		return
	}

	// Remove the first entry (least recently used)
	lruKey := lc.accessOrder[0]
	lc.accessOrder = lc.accessOrder[1:]
	delete(lc.cache, lruKey)
	lc.size--
}

// NewSchemaCache creates a new schema cache with default settings.
func NewSchemaCache() *SchemaCache {
	return &SchemaCache{
		cache:   make(map[string]*CachedSchema),
		order:   make([]string, 0),
		maxSize: 500,
	}
}

// Get retrieves a cached schema validator.
func (sc *SchemaCache) Get(schemaHash string) (*SchemaValidator, bool) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	cached, exists := sc.cache[schemaHash]
	if !exists {
		sc.missCount++
		return nil, false
	}

	// Update access statistics
	cached.AccessCount++
	sc.hitCount++

	// Move to end of access order
	sc.moveToEnd(schemaHash)
	return cached.Validator, true
}

// Set stores a cached schema validator.
func (sc *SchemaCache) Set(schemaHash string, validator *SchemaValidator, schema *ToolSchema) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// If key already exists, update and move to end
	if _, exists := sc.cache[schemaHash]; exists {
		sc.cache[schemaHash] = &CachedSchema{
			Validator:   validator,
			Schema:      schema,
			Hash:        schemaHash,
			CreatedAt:   time.Now(),
			AccessCount: 1,
		}
		sc.moveToEnd(schemaHash)
		return
	}

	// If cache is full, evict least recently used
	if sc.size >= sc.maxSize {
		sc.evictLRU()
	}

	// Add new entry
	sc.cache[schemaHash] = &CachedSchema{
		Validator:   validator,
		Schema:      schema,
		Hash:        schemaHash,
		CreatedAt:   time.Now(),
		AccessCount: 1,
	}
	sc.order = append(sc.order, schemaHash)
	sc.size++
}

// Clear removes all cached schema validators.
func (sc *SchemaCache) Clear() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.cache = make(map[string]*CachedSchema)
	sc.order = make([]string, 0)
	sc.size = 0
}

// GetStats returns cache statistics.
func (sc *SchemaCache) GetStats() (hitCount, missCount int64, hitRate float64) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	total := sc.hitCount + sc.missCount
	hitRate = 0.0
	if total > 0 {
		hitRate = float64(sc.hitCount) / float64(total)
	}

	return sc.hitCount, sc.missCount, hitRate
}

// moveToEnd moves a key to the end of the access order.
func (sc *SchemaCache) moveToEnd(key string) {
	sc.removeFromOrder(key)
	sc.order = append(sc.order, key)
}

// removeFromOrder removes a key from the access order.
func (sc *SchemaCache) removeFromOrder(key string) {
	for i, k := range sc.order {
		if k == key && i < len(sc.order) {
			if i == len(sc.order)-1 {
				sc.order = sc.order[:i]
			} else {
				sc.order = append(sc.order[:i], sc.order[i+1:]...)
			}
			break
		}
	}
}

// evictLRU removes the least recently used entry.
func (sc *SchemaCache) evictLRU() {
	if len(sc.order) == 0 {
		return
	}

	// Remove the first entry (least recently used)
	lruKey := sc.order[0]
	sc.order = sc.order[1:]
	delete(sc.cache, lruKey)
	sc.size--
}

// NewMemoryPool creates a new memory pool.
func NewMemoryPool() *MemoryPool {
	return &MemoryPool{
		toolPool: sync.Pool{
			New: func() interface{} {
				return &Tool{}
			},
		},
		resultPool: sync.Pool{
			New: func() interface{} {
				return &PaginatedResult{}
			},
		},
		filterPool: sync.Pool{
			New: func() interface{} {
				return &ToolFilter{}
			},
		},
		stringSlicePool: sync.Pool{
			New: func() interface{} {
				return make([]string, 0, 10)
			},
		},
		mapPool: sync.Pool{
			New: func() interface{} {
				return make(map[string]interface{})
			},
		},
	}
}

// GetTool retrieves a tool from the pool.
func (mp *MemoryPool) GetTool() *Tool {
	return mp.toolPool.Get().(*Tool)
}

// PutTool returns a tool to the pool.
func (mp *MemoryPool) PutTool(tool *Tool) {
	// Reset the tool to avoid memory leaks
	*tool = Tool{}
	mp.toolPool.Put(tool)
}

// GetResult retrieves a paginated result from the pool.
func (mp *MemoryPool) GetResult() *PaginatedResult {
	result := mp.resultPool.Get().(*PaginatedResult)
	// Reset the result
	*result = PaginatedResult{}
	return result
}

// PutResult returns a paginated result to the pool.
func (mp *MemoryPool) PutResult(result *PaginatedResult) {
	mp.resultPool.Put(result)
}

// GetFilter retrieves a filter from the pool.
func (mp *MemoryPool) GetFilter() *ToolFilter {
	filter := mp.filterPool.Get().(*ToolFilter)
	// Reset the filter
	*filter = ToolFilter{}
	return filter
}

// PutFilter returns a filter to the pool.
func (mp *MemoryPool) PutFilter(filter *ToolFilter) {
	mp.filterPool.Put(filter)
}

// GetStringSlice retrieves a string slice from the pool.
func (mp *MemoryPool) GetStringSlice() []string {
	slice := mp.stringSlicePool.Get().([]string)
	return slice[:0] // Reset length but keep capacity
}

// PutStringSlice returns a string slice to the pool.
func (mp *MemoryPool) PutStringSlice(slice []string) {
	if cap(slice) > 100 { // Avoid keeping very large slices
		return
	}
	mp.stringSlicePool.Put(slice)
}

// GetMap retrieves a map from the pool.
func (mp *MemoryPool) GetMap() map[string]interface{} {
	m := mp.mapPool.Get().(map[string]interface{})
	// Clear the map
	for k := range m {
		delete(m, k)
	}
	return m
}

// PutMap returns a map to the pool.
func (mp *MemoryPool) PutMap(m map[string]interface{}) {
	if len(m) > 100 { // Avoid keeping very large maps
		return
	}
	mp.mapPool.Put(m)
}

// estimateToolMemoryUsage estimates the memory usage of a tool in bytes
func (r *Registry) estimateToolMemoryUsage(tool *Tool) int64 {
	if tool == nil {
		return 0
	}

	size := int64(0)

	// Basic fields
	size += int64(len(tool.ID))
	size += int64(len(tool.Name))
	size += int64(len(tool.Description))
	size += int64(len(tool.Version))

	// Schema size
	if tool.Schema != nil {
		size += r.estimateSchemaSize(tool.Schema)
	}

	// Metadata size
	if tool.Metadata != nil {
		size += int64(len(tool.Metadata.Author))
		size += int64(len(tool.Metadata.License))
		size += int64(len(tool.Metadata.Documentation))
		for _, tag := range tool.Metadata.Tags {
			size += int64(len(tag))
		}
		for _, dep := range tool.Metadata.Dependencies {
			size += int64(len(dep))
		}
		if tool.Metadata.Custom != nil {
			size += r.estimateMapSize(tool.Metadata.Custom)
		}
	}

	// Add overhead for Go object headers and pointers
	size += 200 // Approximate overhead

	return size
}

// estimateSchemaSize estimates the memory usage of a tool schema
func (r *Registry) estimateSchemaSize(schema *ToolSchema) int64 {
	if schema == nil {
		return 0
	}

	size := int64(0)
	size += int64(len(schema.Type))
	size += int64(len(schema.Description))

	for _, req := range schema.Required {
		size += int64(len(req))
	}

	if schema.Properties != nil {
		size += r.estimatePropertyMapSize(schema.Properties)
	}

	return size
}

// estimatePropertyMapSize estimates the memory usage of a map[string]*Property
func (r *Registry) estimatePropertyMapSize(properties map[string]*Property) int64 {
	if properties == nil {
		return 0
	}

	size := int64(0)
	for k, v := range properties {
		size += int64(len(k))
		if v != nil {
			size += int64(len(v.Type))
			size += int64(len(v.Description))
			size += int64(len(v.Format))
			// Add some overhead for the Property struct itself
			size += 50
		}
	}

	// Map overhead
	size += int64(len(properties) * 24) // Approximate overhead per entry

	return size
}

// estimateMapSize estimates the memory usage of a map[string]interface{}
func (r *Registry) estimateMapSize(m map[string]interface{}) int64 {
	if m == nil {
		return 0
	}

	size := int64(0)
	for k, v := range m {
		size += int64(len(k))
		size += r.estimateInterfaceSize(v)
	}

	// Map overhead
	size += int64(len(m) * 24) // Approximate overhead per entry

	return size
}

// estimateInterfaceSize estimates the memory usage of an interface{} value
func (r *Registry) estimateInterfaceSize(v interface{}) int64 {
	if v == nil {
		return 0
	}

	switch val := v.(type) {
	case string:
		return int64(len(val))
	case []string:
		size := int64(0)
		for _, s := range val {
			size += int64(len(s))
		}
		return size
	case map[string]interface{}:
		return r.estimateMapSize(val)
	case []interface{}:
		size := int64(0)
		for _, item := range val {
			size += r.estimateInterfaceSize(item)
		}
		return size
	default:
		// For other types, use a conservative estimate
		return 50
	}
}

// GetResourceUsage returns current resource usage statistics
func (r *Registry) GetResourceUsage() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]interface{}{
		"tool_count":          len(r.tools),
		"max_tools":           r.config.MaxTools,
		"memory_usage":        r.currentMemoryUsage,
		"max_memory_usage":    r.config.MaxMemoryUsage,
		"active_registrations": atomic.LoadInt32(&r.activeRegistrations),
		"max_concurrent_registrations": r.config.MaxConcurrentRegistrations,
		"memory_utilization": func() float64 {
			if r.config.MaxMemoryUsage == 0 {
				return 0.0
			}
			return float64(r.currentMemoryUsage) / float64(r.config.MaxMemoryUsage)
		}(),
		"tool_utilization": func() float64 {
			if r.config.MaxTools == 0 {
				return 0.0
			}
			return float64(len(r.tools)) / float64(r.config.MaxTools)
		}(),
		"tool_ttl_seconds":           r.config.ToolTTL.Seconds(),
		"tool_lru_enabled":           r.config.EnableToolLRU,
		"background_cleanup_enabled": r.config.EnableBackgroundToolCleanup,
	}
}

// Tool Cleanup Methods for Memory Management

// evictLRUTool removes the least recently used tool from the registry
func (r *Registry) evictLRUTool() {
	if r.toolsLRU.Len() == 0 {
		return
	}
	
	// Get the least recently used tool
	elem := r.toolsLRU.Back()
	if elem == nil {
		return
	}
	
	toolID := elem.Value.(string)
	
	// Remove from all structures
	r.toolsLRU.Remove(elem)
	delete(r.toolsIndex, toolID)
	
	// Remove the tool entry and update memory tracking
	if entry, exists := r.tools[toolID]; exists {
		estimatedSize := r.estimateToolMemoryUsage(entry.tool)
		r.currentMemoryUsage -= estimatedSize
		if r.currentMemoryUsage < 0 {
			r.currentMemoryUsage = 0
		}
		delete(r.tools, toolID)
		
		// Clean up indexes
		r.removeFromIndexes(entry.tool)
	}
}

// removeFromIndexes removes a tool from all secondary indexes
func (r *Registry) removeFromIndexes(tool *Tool) {
	// Remove from name index
	delete(r.nameIndex, tool.Name)
	
	// Remove from tag index
	if tool.Metadata != nil && len(tool.Metadata.Tags) > 0 {
		for _, tag := range tool.Metadata.Tags {
			if tagMap := r.tagIndex[tag]; tagMap != nil {
				delete(tagMap, tool.ID)
				if len(tagMap) == 0 {
					delete(r.tagIndex, tag)
				}
			}
		}
	}
	
	// Remove from category index (if using categories)
	if tool.Metadata != nil && len(tool.Metadata.Tags) > 0 {
		for _, category := range tool.Metadata.Tags {
			if categoryMap := r.categoryIndex[category]; categoryMap != nil {
				delete(categoryMap, tool.ID)
				if len(categoryMap) == 0 {
					delete(r.categoryIndex, category)
				}
			}
		}
	}
}

// backgroundToolCleanup runs periodic cleanup of expired tools
func (r *Registry) backgroundToolCleanup() {
	// Safely read the config with proper locking
	r.mu.RLock()
	interval := r.config.ToolCleanupInterval
	r.mu.RUnlock()
	
	// Ensure we have a positive interval, use a minimum of 1 second if invalid
	if interval <= 0 {
		interval = 1 * time.Second // Fallback to prevent panic
	}
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			r.CleanupExpiredTools()
		case <-r.toolCleanupStop:
			return
		}
	}
}

// CleanupExpiredTools removes all expired tools based on TTL
func (r *Registry) CleanupExpiredTools() (int, error) {
	if r.config.ToolTTL <= 0 {
		return 0, nil // TTL not configured
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	cutoff := time.Now().Add(-r.config.ToolTTL)
	var toRemove []string
	
	// Collect expired tools
	for toolID, entry := range r.tools {
		if entry.createdAt.Before(cutoff) {
			toRemove = append(toRemove, toolID)
		}
	}
	
	// Remove expired tools
	for _, toolID := range toRemove {
		if entry, exists := r.tools[toolID]; exists {
			estimatedSize := r.estimateToolMemoryUsage(entry.tool)
			r.currentMemoryUsage -= estimatedSize
			if r.currentMemoryUsage < 0 {
				r.currentMemoryUsage = 0
			}
			
			// Remove from LRU tracking
			if elem, exists := r.toolsIndex[toolID]; exists {
				r.toolsLRU.Remove(elem)
				delete(r.toolsIndex, toolID)
			}
			
			// Remove from all indexes
			r.removeFromIndexes(entry.tool)
			
			// Remove the tool entry
			delete(r.tools, toolID)
		}
	}
	
	// Invalidate caches after cleanup
	r.invalidateListCache()
	
	return len(toRemove), nil
}

// CleanupByAccessTime removes tools that haven't been accessed within the given duration
func (r *Registry) CleanupByAccessTime(maxAge time.Duration) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	cutoff := time.Now().Add(-maxAge)
	var toRemove []string
	
	// Collect old tools
	for toolID, entry := range r.tools {
		if entry.lastAccess.Before(cutoff) {
			toRemove = append(toRemove, toolID)
		}
	}
	
	// Remove old tools
	for _, toolID := range toRemove {
		if entry, exists := r.tools[toolID]; exists {
			estimatedSize := r.estimateToolMemoryUsage(entry.tool)
			r.currentMemoryUsage -= estimatedSize
			if r.currentMemoryUsage < 0 {
				r.currentMemoryUsage = 0
			}
			
			// Remove from LRU tracking
			if elem, exists := r.toolsIndex[toolID]; exists {
				r.toolsLRU.Remove(elem)
				delete(r.toolsIndex, toolID)
			}
			
			// Remove from all indexes
			r.removeFromIndexes(entry.tool)
			
			// Remove the tool entry
			delete(r.tools, toolID)
		}
	}
	
	// Invalidate caches after cleanup
	r.invalidateListCache()
	
	return len(toRemove), nil
}

// ClearAllTools removes all tools from the registry
func (r *Registry) ClearAllTools() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Clear main tools map
	r.tools = make(map[string]*ToolRegistryEntry)
	
	// Clear all indexes
	r.categoryIndex = make(map[string]map[string]bool)
	r.tagIndex = make(map[string]map[string]bool)
	r.nameIndex = make(map[string]string)
	
	// Clear LRU tracking
	r.toolsLRU = list.New()
	r.toolsIndex = make(map[string]*list.Element)
	
	// Reset memory usage
	r.currentMemoryUsage = 0
	
	// Invalidate caches
	r.invalidateListCache()
	
	return nil
}

// GetToolsCleanupStats returns statistics about tool cleanup
func (r *Registry) GetToolsCleanupStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	stats := map[string]interface{}{
		"total_tools":        len(r.tools),
		"lru_list_length":    r.toolsLRU.Len(),
		"lru_index_length":   len(r.toolsIndex),
		"ttl_seconds":        r.config.ToolTTL.Seconds(),
		"cleanup_interval":   r.config.ToolCleanupInterval.Seconds(),
		"lru_enabled":        r.config.EnableToolLRU,
		"cleanup_enabled":    r.config.EnableBackgroundToolCleanup,
	}
	
	// Find oldest and newest tools
	var oldestCreated, newestCreated time.Time
	var oldestAccess, newestAccess time.Time
	totalAccessCount := int64(0)
	
	for _, entry := range r.tools {
		if oldestCreated.IsZero() || entry.createdAt.Before(oldestCreated) {
			oldestCreated = entry.createdAt
		}
		if newestCreated.IsZero() || entry.createdAt.After(newestCreated) {
			newestCreated = entry.createdAt
		}
		if oldestAccess.IsZero() || entry.lastAccess.Before(oldestAccess) {
			oldestAccess = entry.lastAccess
		}
		if newestAccess.IsZero() || entry.lastAccess.After(newestAccess) {
			newestAccess = entry.lastAccess
		}
		totalAccessCount += entry.accessCount
	}
	
	if !oldestCreated.IsZero() {
		stats["oldest_created"] = oldestCreated
		stats["newest_created"] = newestCreated
		stats["oldest_access"] = oldestAccess
		stats["newest_access"] = newestAccess
		stats["total_access_count"] = totalAccessCount
		stats["average_access_count"] = float64(totalAccessCount) / float64(len(r.tools))
	}
	
	return stats
}

// UpdateToolsConfig updates the tool cleanup configuration
func (r *Registry) UpdateToolsConfig(config *RegistryConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	
	// Ensure cleanup interval is positive if background cleanup is enabled
	if config.EnableBackgroundToolCleanup && config.ToolCleanupInterval <= 0 {
		config.ToolCleanupInterval = 15 * time.Minute // Use default value
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	oldConfig := r.config
	r.config = config
	
	// If background cleanup settings changed, restart background cleanup
	if oldConfig.EnableBackgroundToolCleanup != config.EnableBackgroundToolCleanup ||
		oldConfig.ToolCleanupInterval != config.ToolCleanupInterval {
		
		// Stop existing cleanup
		r.toolCleanupOnce.Do(func() {
			close(r.toolCleanupStop)
		})
		
		// Start new cleanup if enabled
		if config.EnableBackgroundToolCleanup && config.ToolCleanupInterval > 0 {
			r.toolCleanupStop = make(chan struct{})
			r.toolCleanupOnce = sync.Once{}
			go r.backgroundToolCleanup()
		}
	}
	
	return nil
}

// CloseToolsCleanup stops background cleanup and releases resources
func (r *Registry) CloseToolsCleanup() error {
	r.toolCleanupOnce.Do(func() {
		close(r.toolCleanupStop)
	})
	return nil
}
