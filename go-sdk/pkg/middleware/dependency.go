package middleware

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// DependencyGraph manages middleware dependencies and execution order
type DependencyGraph struct {
	nodes     map[string]*DependencyNode
	resolved  []string
	resolving map[string]bool
	mu        sync.RWMutex
}

// DependencyNode represents a middleware with its dependencies
type DependencyNode struct {
	Middleware   Middleware
	Dependencies []string            // Names of middleware this one depends on
	Dependents   []string            // Names of middleware that depend on this one
	Optional     bool                // Whether this dependency is optional
	Condition    DependencyCondition // Condition for when this dependency applies
}

// DependencyCondition defines when a dependency relationship applies
type DependencyCondition interface {
	// ShouldApply checks if this dependency condition applies for the given context and request
	ShouldApply(ctx context.Context, req *Request) bool
}

// AlwaysDependencyCondition always applies the dependency
type AlwaysDependencyCondition struct{}

func (adc *AlwaysDependencyCondition) ShouldApply(ctx context.Context, req *Request) bool {
	return true
}

// ConditionalDependencyCondition applies dependency based on custom logic
type ConditionalDependencyCondition struct {
	Condition func(ctx context.Context, req *Request) bool
}

func (cdc *ConditionalDependencyCondition) ShouldApply(ctx context.Context, req *Request) bool {
	if cdc.Condition == nil {
		return true
	}
	return cdc.Condition(ctx, req)
}

// PathBasedDependencyCondition applies dependency based on request path patterns
type PathBasedDependencyCondition struct {
	PathPatterns []string
}

func (pdc *PathBasedDependencyCondition) ShouldApply(ctx context.Context, req *Request) bool {
	for _, pattern := range pdc.PathPatterns {
		if matchPath(req.Path, pattern) {
			return true
		}
	}
	return false
}

// matchPath checks if a path matches a pattern (supports wildcards)
func matchPath(path, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		// Simple wildcard matching
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(path, parts[0]) && strings.HasSuffix(path, parts[1])
		}
	}
	return path == pattern
}

// NewDependencyGraph creates a new dependency graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes:     make(map[string]*DependencyNode),
		resolving: make(map[string]bool),
	}
}

// AddMiddleware adds middleware to the dependency graph
func (dg *DependencyGraph) AddMiddleware(middleware Middleware, dependencies []string, optional bool, condition DependencyCondition) error {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	name := middleware.Name()
	if condition == nil {
		condition = &AlwaysDependencyCondition{}
	}

	node := &DependencyNode{
		Middleware:   middleware,
		Dependencies: dependencies,
		Dependents:   make([]string, 0),
		Optional:     optional,
		Condition:    condition,
	}

	// Add to nodes
	dg.nodes[name] = node

	// Update dependents for dependencies
	for _, dep := range dependencies {
		if depNode, exists := dg.nodes[dep]; exists {
			depNode.Dependents = append(depNode.Dependents, name)
		}
	}

	// Clear resolved order to force re-resolution
	dg.resolved = nil

	return nil
}

// RemoveMiddleware removes middleware from the dependency graph
func (dg *DependencyGraph) RemoveMiddleware(name string) error {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	node, exists := dg.nodes[name]
	if !exists {
		return fmt.Errorf("middleware %s not found", name)
	}

	// Remove from dependents of dependencies
	for _, dep := range node.Dependencies {
		if depNode, exists := dg.nodes[dep]; exists {
			depNode.Dependents = removeName(depNode.Dependents, name)
		}
	}

	// Remove from dependencies of dependents
	for _, dependent := range node.Dependents {
		if depNode, exists := dg.nodes[dependent]; exists {
			depNode.Dependencies = removeName(depNode.Dependencies, name)
		}
	}

	delete(dg.nodes, name)
	dg.resolved = nil

	return nil
}

// removeName removes a name from a slice of names
func removeName(slice []string, name string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != name {
			result = append(result, item)
		}
	}
	return result
}

// ResolveOrder resolves the execution order based on dependencies
func (dg *DependencyGraph) ResolveOrder(ctx context.Context, req *Request) ([]Middleware, error) {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	// If already resolved and no changes, return cached result
	if dg.resolved != nil {
		result := make([]Middleware, 0, len(dg.resolved))
		for _, name := range dg.resolved {
			if node, exists := dg.nodes[name]; exists {
				// Check if condition applies
				if node.Condition.ShouldApply(ctx, req) {
					result = append(result, node.Middleware)
				}
			}
		}
		return result, nil
	}

	// Perform topological sort
	resolved := make([]string, 0, len(dg.nodes))
	resolving := make(map[string]bool)

	for name := range dg.nodes {
		if err := dg.visit(name, &resolved, resolving, ctx, req); err != nil {
			return nil, err
		}
	}

	// Cache the resolved order
	dg.resolved = resolved

	// Convert to middleware slice
	result := make([]Middleware, 0, len(resolved))
	for _, name := range resolved {
		if node, exists := dg.nodes[name]; exists {
			// Check if condition applies
			if node.Condition.ShouldApply(ctx, req) {
				result = append(result, node.Middleware)
			}
		}
	}

	return result, nil
}

// visit performs depth-first search for topological sorting
func (dg *DependencyGraph) visit(name string, resolved *[]string, resolving map[string]bool, ctx context.Context, req *Request) error {
	// Check if already resolved
	for _, r := range *resolved {
		if r == name {
			return nil
		}
	}

	// Check for circular dependency
	if resolving[name] {
		return fmt.Errorf("circular dependency detected involving %s", name)
	}

	node, exists := dg.nodes[name]
	if !exists {
		return fmt.Errorf("middleware %s not found", name)
	}

	resolving[name] = true

	// Visit dependencies first
	for _, dep := range node.Dependencies {
		if depNode, exists := dg.nodes[dep]; exists {
			// Check if dependency condition applies
			if depNode.Condition.ShouldApply(ctx, req) {
				if err := dg.visit(dep, resolved, resolving, ctx, req); err != nil {
					return err
				}
			}
		} else if !node.Optional {
			return fmt.Errorf("required dependency %s not found for %s", dep, name)
		}
	}

	// Add this node to resolved list
	*resolved = append(*resolved, name)
	delete(resolving, name)

	return nil
}

// GetDependencies returns the dependencies of a middleware
func (dg *DependencyGraph) GetDependencies(name string) ([]string, error) {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	node, exists := dg.nodes[name]
	if !exists {
		return nil, fmt.Errorf("middleware %s not found", name)
	}

	return append([]string(nil), node.Dependencies...), nil
}

// GetDependents returns the dependents of a middleware
func (dg *DependencyGraph) GetDependents(name string) ([]string, error) {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	node, exists := dg.nodes[name]
	if !exists {
		return nil, fmt.Errorf("middleware %s not found", name)
	}

	return append([]string(nil), node.Dependents...), nil
}

// ValidateGraph validates the dependency graph for consistency
func (dg *DependencyGraph) ValidateGraph() []error {
	dg.mu.RLock()
	defer dg.mu.RUnlock()

	var errors []error

	// Check for missing dependencies
	for name, node := range dg.nodes {
		for _, dep := range node.Dependencies {
			if _, exists := dg.nodes[dep]; !exists {
				if !node.Optional {
					errors = append(errors, fmt.Errorf("middleware %s has missing required dependency: %s", name, dep))
				}
			}
		}
	}

	// Check for circular dependencies by attempting resolution with dummy context
	// We need to release the read lock before calling ResolveOrder to avoid deadlock
	dg.mu.RUnlock()
	dummyReq := &Request{Path: "/"}
	if _, err := dg.ResolveOrder(context.Background(), dummyReq); err != nil {
		errors = append(errors, err)
	}
	dg.mu.RLock()

	return errors
}

// DependencyAwareMiddlewareChain extends MiddlewareChain with dependency management
type DependencyAwareMiddlewareChain struct {
	*MiddlewareChain
	dependencyGraph *DependencyGraph
}

// NewDependencyAwareMiddlewareChain creates a new dependency-aware middleware chain
func NewDependencyAwareMiddlewareChain(handler Handler) *DependencyAwareMiddlewareChain {
	return &DependencyAwareMiddlewareChain{
		MiddlewareChain: NewMiddlewareChain(handler),
		dependencyGraph: NewDependencyGraph(),
	}
}

// AddMiddlewareWithDependencies adds middleware with dependency information
func (dac *DependencyAwareMiddlewareChain) AddMiddlewareWithDependencies(
	middleware Middleware,
	dependencies []string,
	optional bool,
	condition DependencyCondition,
) error {
	// Add to dependency graph
	if err := dac.dependencyGraph.AddMiddleware(middleware, dependencies, optional, condition); err != nil {
		return err
	}

	// Don't add to regular chain yet - will be added in proper order during execution
	return nil
}

// Process executes the middleware chain with proper dependency order
func (dac *DependencyAwareMiddlewareChain) Process(ctx context.Context, req *Request) (*Response, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// Set request timestamp if not already set
	if req.Timestamp.IsZero() {
		req.Timestamp = time.Now()
	}

	// Resolve middleware order based on dependencies
	orderedMiddleware, err := dac.dependencyGraph.ResolveOrder(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve middleware dependencies: %w", err)
	}

	// Execute in resolved order
	return dac.executeOrderedChain(ctx, req, orderedMiddleware, 0)
}

// executeOrderedChain executes middleware in the resolved dependency order
func (dac *DependencyAwareMiddlewareChain) executeOrderedChain(ctx context.Context, req *Request, orderedMiddleware []Middleware, index int) (*Response, error) {
	// If we've processed all middleware, call the final handler
	if index >= len(orderedMiddleware) {
		if dac.handler == nil {
			return &Response{
				ID:         req.ID,
				StatusCode: 404,
				Error:      fmt.Errorf("no handler configured"),
				Timestamp:  time.Now(),
			}, nil
		}

		startTime := time.Now()
		resp, err := dac.handler(ctx, req)
		if resp != nil && resp.Duration == 0 {
			resp.Duration = time.Since(startTime)
		}
		return resp, err
	}

	middleware := orderedMiddleware[index]

	// Skip disabled middleware
	if !middleware.Enabled() {
		return dac.executeOrderedChain(ctx, req, orderedMiddleware, index+1)
	}

	// Create the next handler for this middleware
	next := func(ctx context.Context, req *Request) (*Response, error) {
		return dac.executeOrderedChain(ctx, req, orderedMiddleware, index+1)
	}

	// Execute the middleware
	startTime := time.Now()
	resp, err := middleware.Process(ctx, req, next)
	if resp != nil && resp.Duration == 0 {
		resp.Duration = time.Since(startTime)
	}

	return resp, err
}

// GetDependencyGraph returns the dependency graph
func (dac *DependencyAwareMiddlewareChain) GetDependencyGraph() *DependencyGraph {
	return dac.dependencyGraph
}

// ValidateDependencies validates the dependency graph
func (dac *DependencyAwareMiddlewareChain) ValidateDependencies() []error {
	return dac.dependencyGraph.ValidateGraph()
}

// DependencyManager manages multiple dependency-aware chains
type DependencyManager struct {
	chains map[string]*DependencyAwareMiddlewareChain
	mu     sync.RWMutex
}

// NewDependencyManager creates a new dependency manager
func NewDependencyManager() *DependencyManager {
	return &DependencyManager{
		chains: make(map[string]*DependencyAwareMiddlewareChain),
	}
}

// CreateChain creates a new dependency-aware chain
func (dm *DependencyManager) CreateChain(name string, handler Handler) *DependencyAwareMiddlewareChain {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	chain := NewDependencyAwareMiddlewareChain(handler)
	dm.chains[name] = chain
	return chain
}

// GetChain gets a dependency-aware chain by name
func (dm *DependencyManager) GetChain(name string) *DependencyAwareMiddlewareChain {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	return dm.chains[name]
}

// ValidateAllChains validates all dependency-aware chains
func (dm *DependencyManager) ValidateAllChains() map[string][]error {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	result := make(map[string][]error)
	for name, chain := range dm.chains {
		if errors := chain.ValidateDependencies(); len(errors) > 0 {
			result[name] = errors
		}
	}

	return result
}

// GetDependencyReport returns a report of all dependencies
func (dm *DependencyManager) GetDependencyReport() DependencyReport {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	report := DependencyReport{
		Chains: make(map[string]ChainDependencyInfo),
	}

	for name, chain := range dm.chains {
		dg := chain.GetDependencyGraph()
		chainInfo := ChainDependencyInfo{
			MiddlewareCount: len(dg.nodes),
			Dependencies:    make(map[string][]string),
			Dependents:      make(map[string][]string),
		}

		for middlewareName, node := range dg.nodes {
			chainInfo.Dependencies[middlewareName] = append([]string(nil), node.Dependencies...)
			chainInfo.Dependents[middlewareName] = append([]string(nil), node.Dependents...)
		}

		report.Chains[name] = chainInfo
	}

	return report
}

// DependencyReport contains information about all dependency relationships
type DependencyReport struct {
	Chains map[string]ChainDependencyInfo
}

// ChainDependencyInfo contains dependency information for a chain
type ChainDependencyInfo struct {
	MiddlewareCount int
	Dependencies    map[string][]string
	Dependents      map[string][]string
}

// PriorityBasedDependency provides automatic dependency resolution based on priorities
type PriorityBasedDependency struct {
	graph *DependencyGraph
}

// NewPriorityBasedDependency creates a new priority-based dependency resolver
func NewPriorityBasedDependency() *PriorityBasedDependency {
	return &PriorityBasedDependency{
		graph: NewDependencyGraph(),
	}
}

// AddMiddlewareWithPriority adds middleware and automatically resolves dependencies based on priority
func (pbd *PriorityBasedDependency) AddMiddlewareWithPriority(middleware Middleware) error {
	// Get all existing middleware
	existing := make([]Middleware, 0)
	for _, node := range pbd.graph.nodes {
		existing = append(existing, node.Middleware)
	}

	// Sort by priority
	sort.Slice(existing, func(i, j int) bool {
		return existing[i].Priority() > existing[j].Priority()
	})

	// Determine dependencies based on priority
	// Lower priority middleware should depend on higher priority middleware
	dependencies := make([]string, 0)
	for _, existingMiddleware := range existing {
		if existingMiddleware.Priority() > middleware.Priority() {
			dependencies = append(dependencies, existingMiddleware.Name())
		}
	}

	// Add the new middleware
	err := pbd.graph.AddMiddleware(middleware, dependencies, false, &AlwaysDependencyCondition{})
	if err != nil {
		return err
	}

	// Update existing middleware with lower priority to depend on this new higher priority middleware
	for _, existingMiddleware := range existing {
		if existingMiddleware.Priority() < middleware.Priority() {
			// Get current dependencies of the existing middleware
			existingNode := pbd.graph.nodes[existingMiddleware.Name()]
			existingDeps := append([]string(nil), existingNode.Dependencies...)

			// Add this new middleware as a dependency
			existingDeps = append(existingDeps, middleware.Name())

			// Remove and re-add the existing middleware with updated dependencies
			pbd.graph.RemoveMiddleware(existingMiddleware.Name())
			pbd.graph.AddMiddleware(existingMiddleware, existingDeps, false, &AlwaysDependencyCondition{})
		}
	}

	return nil
}

// ResolveOrder resolves execution order based on priorities
func (pbd *PriorityBasedDependency) ResolveOrder(ctx context.Context, req *Request) ([]Middleware, error) {
	return pbd.graph.ResolveOrder(ctx, req)
}
