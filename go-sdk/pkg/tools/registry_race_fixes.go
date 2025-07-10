package tools

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

// RaceFixedRegistry is a race-condition-free implementation of the Registry
// This demonstrates the fixes for the race conditions identified in registry.go
type RaceFixedRegistry struct {
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
	watchers          sync.Map // Use sync.Map for concurrent access
	dependencyGraph   *RaceFixedDependencyGraph
	loadingStrategies map[string]LoadingStrategy

	// Configuration
	config *RegistryConfig

	// Hook protection
	hookMu sync.RWMutex
}

// RaceFixedDependencyGraph is a thread-safe dependency graph
type RaceFixedDependencyGraph struct {
	mu           sync.RWMutex
	dependencies map[string]map[string]*DependencyConstraint
	cache        sync.Map // Use sync.Map for concurrent cache access
}

// RegisterWithContext adds a new tool to the registry with context support (FIXED).
// This version holds the write lock for the entire operation to prevent TOCTOU races.
func (r *RaceFixedRegistry) RegisterWithContext(ctx context.Context, tool *Tool) error {
	if tool == nil {
		return NewToolError(ErrorTypeValidation, "NIL_TOOL", "tool cannot be nil")
	}

	// Validate the tool
	if err := tool.Validate(); err != nil {
		return NewToolError(ErrorTypeValidation, "VALIDATION_FAILED", "tool validation failed").
			WithToolID(tool.ID).
			WithCause(err)
	}

	// Run custom validators with read lock
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

	// Check for existing tools while holding the write lock
	existingTool, idExists := r.tools[tool.ID]
	existingID, nameExists := r.nameIndex[tool.Name]
	var existingByName *Tool
	if nameExists && existingID != tool.ID {
		existingByName = r.tools[existingID]
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
			return fmt.Errorf("name conflict resolution failed: %w", err)
		}
		if resolvedTool == nil {
			return nil
		}
		tool = resolvedTool
	}

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

// SafeFileWatcher is a race-condition-free file watcher
type SafeFileWatcher struct {
	path     string
	modTime  time.Time
	callback func(string) error
	stopOnce sync.Once
	stop     chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

// WatchFile watches a file for changes and reloads tools (FIXED).
func (r *RaceFixedRegistry) WatchFile(filename string) error {
	if !r.config.EnableHotReloading {
		return fmt.Errorf("hot reloading is disabled")
	}

	// Check if already watching
	if _, loaded := r.watchers.Load(filename); loaded {
		return fmt.Errorf("file %q is already being watched", filename)
	}

	// Get initial file info
	info, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	watcher := &SafeFileWatcher{
		path:     filename,
		stop:     make(chan struct{}),
		modTime:  info.ModTime(),
		callback: func(path string) error {
			return r.LoadFromFile(context.Background(), path)
		},
	}

	// Store watcher using sync.Map
	_, loaded := r.watchers.LoadOrStore(filename, watcher)
	if loaded {
		// Another goroutine beat us to it
		return fmt.Errorf("file %q is already being watched", filename)
	}

	// Start goroutine with proper lifecycle tracking
	watcher.wg.Add(1)
	go func() {
		defer watcher.wg.Done()
		r.watchFileChanges(watcher)
	}()

	return nil
}

// StopWatching stops watching a file (FIXED).
func (r *RaceFixedRegistry) StopWatching(filename string) error {
	value, exists := r.watchers.LoadAndDelete(filename)
	if !exists {
		return fmt.Errorf("file %q is not being watched", filename)
	}

	watcher := value.(*SafeFileWatcher)

	// Use sync.Once to ensure stop is closed only once
	watcher.stopOnce.Do(func() {
		close(watcher.stop)
	})

	// Wait for goroutine to complete
	watcher.wg.Wait()

	return nil
}

// watchFileChanges monitors a file for changes (FIXED).
func (r *RaceFixedRegistry) watchFileChanges(watcher *SafeFileWatcher) {
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

// checkFileForChanges checks if a file has been modified (FIXED).
func (r *RaceFixedRegistry) checkFileForChanges(watcher *SafeFileWatcher) error {
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

// ResolveDependencies resolves all dependencies for a tool (FIXED).
func (dg *RaceFixedDependencyGraph) ResolveDependencies(toolID string, tools map[string]*Tool) ([]*Tool, error) {
	// Check cache first using sync.Map
	if cached, exists := dg.cache.Load(toolID); exists {
		return cached.([]*Tool), nil
	}

	// Resolve dependencies
	visited := make(map[string]bool)
	var result []*Tool

	dg.mu.RLock()
	err := dg.resolveDependenciesRecursive(toolID, tools, visited, &result, 0, 10)
	dg.mu.RUnlock()

	if err != nil {
		return nil, err
	}

	// Cache the result using sync.Map
	dg.cache.Store(toolID, result)

	return result, nil
}

// resolveDependenciesRecursive recursively resolves dependencies.
func (dg *RaceFixedDependencyGraph) resolveDependenciesRecursive(toolID string, tools map[string]*Tool, visited map[string]bool, result *[]*Tool, depth, maxDepth int) error {
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

// AddValidator adds a custom validation function (FIXED).
func (r *RaceFixedRegistry) AddValidator(validator RegistryValidator) {
	r.hookMu.Lock()
	defer r.hookMu.Unlock()
	r.validators = append(r.validators, validator)
}

// AddConflictResolver adds a custom conflict resolver (FIXED).
func (r *RaceFixedRegistry) AddConflictResolver(resolver ConflictResolver) {
	r.hookMu.Lock()
	defer r.hookMu.Unlock()
	r.conflictResolvers = append(r.conflictResolvers, resolver)
}

// Placeholder methods for compatibility
func (r *RaceFixedRegistry) resolveConflict(ctx context.Context, existing, new *Tool) (*Tool, error) {
	// Implementation would be similar to original but called while holding lock
	return new, nil
}

func (r *RaceFixedRegistry) handleVersionMigration(ctx context.Context, tool *Tool) error {
	// Implementation would be similar to original but called while holding lock
	return nil
}

func (r *RaceFixedRegistry) updateCategoryTree(tool *Tool) error {
	// Implementation would be similar to original but called while holding lock
	return nil
}

func (r *RaceFixedRegistry) LoadFromFile(ctx context.Context, filename string) error {
	// Implementation would be similar to original
	return nil
}

func (dg *RaceFixedDependencyGraph) AddTool(tool *Tool) error {
	// Implementation would be similar to original
	return nil
}