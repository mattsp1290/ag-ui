package marketplace

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// DependencyResolver handles package dependency resolution and management
type DependencyResolver struct {
	installed   map[string]*InstalledPackage // packageID -> installed package
	graph       *DependencyGraph
	constraints map[string]*DependencyConstraint
	cache       map[string]*ResolutionResult
	mu          sync.RWMutex
}

// InstalledPackage represents an installed package
type InstalledPackage struct {
	Package      *RulePackage
	Version      string
	InstallPath  string
	Dependencies map[string]string // dependencyID -> version
	Dependents   map[string]string // dependentID -> version
	InstallTime  int64
}

// DependencyGraph represents the dependency graph
type DependencyGraph struct {
	Nodes map[string]*DependencyNode
	Edges map[string][]*DependencyEdge
}

// DependencyNode represents a node in the dependency graph
type DependencyNode struct {
	PackageID  string
	Version    string
	Package    *RulePackage
	Level      int
	Visited    bool
	InProgress bool
	Conflicts  []*DependencyConflict
}

// DependencyEdge represents an edge in the dependency graph
type DependencyEdge struct {
	From       string
	To         string
	Constraint string
	Required   bool
	Type       DependencyType
}

// DependencyType defines the type of dependency
type DependencyType string

const (
	DependencyRuntime     DependencyType = "runtime"
	DependencyDevelopment DependencyType = "development"
	DependencyOptional    DependencyType = "optional"
	DependencyPeer        DependencyType = "peer"
)

// DependencyConstraint defines constraints for dependency resolution
type DependencyConstraint struct {
	PackageID        string
	AllowedVersions  []string
	BlockedVersions  []string
	MaxDepth         int
	AllowCycles      bool
	ConflictStrategy ConflictStrategy
}

// ConflictStrategy defines how to handle dependency conflicts
type ConflictStrategy string

const (
	ConflictStrict   ConflictStrategy = "strict"   // Fail on any conflict
	ConflictLatest   ConflictStrategy = "latest"   // Use latest compatible version
	ConflictExplicit ConflictStrategy = "explicit" // Require explicit resolution
	ConflictIgnore   ConflictStrategy = "ignore"   // Ignore conflicts
)

// DependencyConflict represents a dependency conflict
type DependencyConflict struct {
	PackageID   string
	RequestedBy []string
	Versions    []string
	Type        ConflictType
	Severity    ConflictSeverity
	Resolution  *ConflictResolution
}

// ConflictType defines the type of conflict
type ConflictType string

const (
	ConflictVersionMismatch ConflictType = "version_mismatch"
	ConflictCircular        ConflictType = "circular"
	ConflictMissing         ConflictType = "missing"
	ConflictIncompatible    ConflictType = "incompatible"
)

// ConflictSeverity defines the severity of a conflict
type ConflictSeverity string

const (
	SeverityLow      ConflictSeverity = "low"
	SeverityMedium   ConflictSeverity = "medium"
	SeverityHigh     ConflictSeverity = "high"
	SeverityCritical ConflictSeverity = "critical"
)

// ConflictResolution represents a conflict resolution
type ConflictResolution struct {
	Strategy      ConflictStrategy
	ChosenVersion string
	Reason        string
	Manual        bool
}

// ResolutionResult represents the result of dependency resolution
type ResolutionResult struct {
	Dependencies []*ResolvedDependency
	Conflicts    []*DependencyConflict
	InstallOrder []string
	Success      bool
	Error        error
}

// ResolvedDependency represents a resolved dependency
type ResolvedDependency struct {
	ID       string
	Version  string
	Package  *RulePackage
	Type     DependencyType
	Required bool
	Source   string
	Children []*ResolvedDependency
}

// NewDependencyResolver creates a new dependency resolver
func NewDependencyResolver() *DependencyResolver {
	return &DependencyResolver{
		installed:   make(map[string]*InstalledPackage),
		graph:       NewDependencyGraph(),
		constraints: make(map[string]*DependencyConstraint),
		cache:       make(map[string]*ResolutionResult),
	}
}

// NewDependencyGraph creates a new dependency graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		Nodes: make(map[string]*DependencyNode),
		Edges: make(map[string][]*DependencyEdge),
	}
}

// ResolveDependencies resolves dependencies for a package
func (dr *DependencyResolver) ResolveDependencies(pkg *RulePackage) ([]*ResolvedDependency, error) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	// Check cache first
	cacheKey := fmt.Sprintf("%s@%s", pkg.ID, pkg.Version)
	if result, exists := dr.cache[cacheKey]; exists && result.Success {
		return result.Dependencies, result.Error
	}

	// Clear graph for new resolution
	dr.graph = NewDependencyGraph()

	// Start resolution
	result, err := dr.resolvePackageDependencies(pkg, 0)
	if err != nil {
		return nil, err
	}

	// Check for conflicts
	conflicts := dr.detectConflicts()
	if len(conflicts) > 0 {
		if err := dr.resolveConflicts(conflicts); err != nil {
			return nil, fmt.Errorf("unresolved conflicts: %w", err)
		}
	}

	// Generate install order
	installOrder, err := dr.generateInstallOrder()
	if err != nil {
		return nil, fmt.Errorf("failed to generate install order: %w", err)
	}

	// Cache result
	dr.cache[cacheKey] = &ResolutionResult{
		Dependencies: result,
		Conflicts:    conflicts,
		InstallOrder: installOrder,
		Success:      true,
	}

	return result, nil
}

// resolvePackageDependencies resolves dependencies for a specific package
func (dr *DependencyResolver) resolvePackageDependencies(pkg *RulePackage, depth int) ([]*ResolvedDependency, error) {
	// Check max depth
	if constraint, exists := dr.constraints[pkg.ID]; exists && constraint.MaxDepth > 0 && depth > constraint.MaxDepth {
		return nil, fmt.Errorf("maximum dependency depth exceeded for package %s", pkg.ID)
	}

	var resolved []*ResolvedDependency

	// Add package to graph
	nodeKey := fmt.Sprintf("%s@%s", pkg.ID, pkg.Version)
	dr.graph.Nodes[nodeKey] = &DependencyNode{
		PackageID: pkg.ID,
		Version:   pkg.Version,
		Package:   pkg,
		Level:     depth,
	}

	// Process each dependency
	for _, dep := range pkg.Dependencies {
		resolvedDep, err := dr.resolveDependency(dep, pkg, depth+1)
		if err != nil {
			if dep.Required {
				return nil, fmt.Errorf("failed to resolve required dependency %s: %w", dep.ID, err)
			}
			// Skip optional dependencies that can't be resolved
			continue
		}

		resolved = append(resolved, resolvedDep)

		// Add edge to graph
		fromKey := fmt.Sprintf("%s@%s", pkg.ID, pkg.Version)
		toKey := fmt.Sprintf("%s@%s", dep.ID, resolvedDep.Version)
		dr.graph.Edges[fromKey] = append(dr.graph.Edges[fromKey], &DependencyEdge{
			From:       fromKey,
			To:         toKey,
			Constraint: dep.VersionRange,
			Required:   dep.Required,
			Type:       DependencyType(dep.Type),
		})
	}

	return resolved, nil
}

// resolveDependency resolves a single dependency
func (dr *DependencyResolver) resolveDependency(dep *Dependency, parent *RulePackage, depth int) (*ResolvedDependency, error) {
	// Check if already installed and compatible
	if installed, exists := dr.installed[dep.ID]; exists {
		if dr.isVersionCompatible(installed.Version, dep.VersionRange) {
			return &ResolvedDependency{
				ID:       dep.ID,
				Version:  installed.Version,
				Package:  installed.Package,
				Type:     DependencyType(dep.Type),
				Required: dep.Required,
				Source:   "installed",
			}, nil
		}
	}

	// Resolve version from marketplace
	// This would be injected in real implementation
	// For now, simulate version resolution failure
	return nil, fmt.Errorf("marketplace not available for version resolution")
}

// ValidateDependencies validates package dependencies
func (dr *DependencyResolver) ValidateDependencies(pkg *RulePackage) error {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	// Check for basic dependency issues
	depIDs := make(map[string]bool)
	for _, dep := range pkg.Dependencies {
		// Check for duplicate dependencies
		if depIDs[dep.ID] {
			return fmt.Errorf("duplicate dependency: %s", dep.ID)
		}
		depIDs[dep.ID] = true

		// Check for self-dependency
		if dep.ID == pkg.ID {
			return fmt.Errorf("package cannot depend on itself")
		}

		// Validate version constraint format
		if err := dr.validateVersionConstraint(dep.VersionRange); err != nil {
			return fmt.Errorf("invalid version constraint for %s: %w", dep.ID, err)
		}
	}

	return nil
}

// IsInstalled checks if a package is installed
func (dr *DependencyResolver) IsInstalled(packageID, version string) bool {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	if installed, exists := dr.installed[packageID]; exists {
		return installed.Version == version
	}
	return false
}

// MarkInstalled marks a package as installed
func (dr *DependencyResolver) MarkInstalled(pkg *RulePackage, installPath string) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	dependencies := make(map[string]string)
	for _, dep := range pkg.Dependencies {
		dependencies[dep.ID] = dep.Version
	}

	dr.installed[pkg.ID] = &InstalledPackage{
		Package:      pkg,
		Version:      pkg.Version,
		InstallPath:  installPath,
		Dependencies: dependencies,
		Dependents:   make(map[string]string),
		InstallTime:  pkg.InstalledAt.Unix(),
	}

	// Update dependents
	for depID := range dependencies {
		if installed, exists := dr.installed[depID]; exists {
			installed.Dependents[pkg.ID] = pkg.Version
		}
	}
}

// UnmarkInstalled removes a package from installed packages
func (dr *DependencyResolver) UnmarkInstalled(packageID string) error {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	installed, exists := dr.installed[packageID]
	if !exists {
		return fmt.Errorf("package %s is not installed", packageID)
	}

	// Check if any packages depend on this one
	if len(installed.Dependents) > 0 {
		dependents := make([]string, 0, len(installed.Dependents))
		for depID := range installed.Dependents {
			dependents = append(dependents, depID)
		}
		return fmt.Errorf("cannot uninstall %s: required by %s", packageID, strings.Join(dependents, ", "))
	}

	// Remove from dependents of dependencies
	for depID := range installed.Dependencies {
		if dep, exists := dr.installed[depID]; exists {
			delete(dep.Dependents, packageID)
		}
	}

	delete(dr.installed, packageID)
	return nil
}

// GetInstallOrder returns the order in which packages should be installed
func (dr *DependencyResolver) GetInstallOrder(packages []*RulePackage) ([]string, error) {
	// Build temporary graph for these packages
	tempGraph := NewDependencyGraph()

	for _, pkg := range packages {
		nodeKey := fmt.Sprintf("%s@%s", pkg.ID, pkg.Version)
		tempGraph.Nodes[nodeKey] = &DependencyNode{
			PackageID: pkg.ID,
			Version:   pkg.Version,
			Package:   pkg,
		}

		for _, dep := range pkg.Dependencies {
			depKey := fmt.Sprintf("%s@%s", dep.ID, dep.Version)
			tempGraph.Edges[nodeKey] = append(tempGraph.Edges[nodeKey], &DependencyEdge{
				From:     nodeKey,
				To:       depKey,
				Required: dep.Required,
			})
		}
	}

	return dr.topologicalSort(tempGraph)
}

// detectConflicts detects dependency conflicts in the current graph
func (dr *DependencyResolver) detectConflicts() []*DependencyConflict {
	var conflicts []*DependencyConflict

	// Group nodes by package ID
	packageVersions := make(map[string][]string)
	packageRequesters := make(map[string]map[string][]string)

	for nodeKey, node := range dr.graph.Nodes {
		packageVersions[node.PackageID] = append(packageVersions[node.PackageID], node.Version)

		if packageRequesters[node.PackageID] == nil {
			packageRequesters[node.PackageID] = make(map[string][]string)
		}

		// Find who requested this version
		for fromKey, edges := range dr.graph.Edges {
			for _, edge := range edges {
				if edge.To == nodeKey {
					fromNode := dr.graph.Nodes[fromKey]
					packageRequesters[node.PackageID][node.Version] = append(
						packageRequesters[node.PackageID][node.Version],
						fromNode.PackageID,
					)
				}
			}
		}
	}

	// Check for version conflicts
	for packageID, versions := range packageVersions {
		if len(versions) > 1 {
			// Remove duplicates
			uniqueVersions := dr.removeDuplicates(versions)
			if len(uniqueVersions) > 1 {
				var requestedBy []string
				for _, version := range uniqueVersions {
					if requesters, exists := packageRequesters[packageID][version]; exists {
						requestedBy = append(requestedBy, requesters...)
					}
				}

				conflicts = append(conflicts, &DependencyConflict{
					PackageID:   packageID,
					RequestedBy: dr.removeDuplicates(requestedBy),
					Versions:    uniqueVersions,
					Type:        ConflictVersionMismatch,
					Severity:    SeverityMedium,
				})
			}
		}
	}

	return conflicts
}

// resolveConflicts attempts to resolve dependency conflicts
func (dr *DependencyResolver) resolveConflicts(conflicts []*DependencyConflict) error {
	for _, conflict := range conflicts {
		constraint := dr.constraints[conflict.PackageID]
		strategy := ConflictStrict
		if constraint != nil {
			strategy = constraint.ConflictStrategy
		}

		switch strategy {
		case ConflictStrict:
			return fmt.Errorf("conflict detected for package %s: multiple versions requested %v",
				conflict.PackageID, conflict.Versions)

		case ConflictLatest:
			// Choose the latest version
			latestVersion := dr.findLatestVersion(conflict.Versions)
			conflict.Resolution = &ConflictResolution{
				Strategy:      ConflictLatest,
				ChosenVersion: latestVersion,
				Reason:        "Chose latest compatible version",
			}

		case ConflictExplicit:
			return fmt.Errorf("explicit conflict resolution required for package %s", conflict.PackageID)

		case ConflictIgnore:
			// Use the first version encountered
			conflict.Resolution = &ConflictResolution{
				Strategy:      ConflictIgnore,
				ChosenVersion: conflict.Versions[0],
				Reason:        "Ignored conflict, used first version",
			}
		}
	}

	return nil
}

// generateInstallOrder generates the order in which packages should be installed
func (dr *DependencyResolver) generateInstallOrder() ([]string, error) {
	return dr.topologicalSort(dr.graph)
}

// topologicalSort performs topological sorting on the dependency graph
func (dr *DependencyResolver) topologicalSort(graph *DependencyGraph) ([]string, error) {
	// Kahn's algorithm for topological sorting
	inDegree := make(map[string]int)

	// Initialize in-degree count
	for nodeKey := range graph.Nodes {
		inDegree[nodeKey] = 0
	}

	// Calculate in-degree for each node
	for _, edges := range graph.Edges {
		for _, edge := range edges {
			inDegree[edge.To]++
		}
	}

	// Find nodes with no incoming edges
	queue := make([]string, 0)
	for nodeKey, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, nodeKey)
		}
	}

	var result []string

	for len(queue) > 0 {
		// Remove node from queue
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// For each neighbor of current node
		if edges, exists := graph.Edges[current]; exists {
			for _, edge := range edges {
				inDegree[edge.To]--
				if inDegree[edge.To] == 0 {
					queue = append(queue, edge.To)
				}
			}
		}
	}

	// Check for cycles
	if len(result) != len(graph.Nodes) {
		return nil, fmt.Errorf("circular dependency detected")
	}

	return result, nil
}

// Helper methods

func (dr *DependencyResolver) isVersionCompatible(installedVersion, constraint string) bool {
	// This would use the version manager to check compatibility
	// For now, simple equality check
	return installedVersion == constraint
}

func (dr *DependencyResolver) hasCircularDependency(depID, parentID string) bool {
	// Simple cycle detection - in real implementation would be more sophisticated
	return depID == parentID
}

func (dr *DependencyResolver) validateVersionConstraint(constraint string) error {
	// Validate version constraint format
	if constraint == "" {
		return fmt.Errorf("version constraint cannot be empty")
	}

	// Add more validation logic here
	return nil
}

func (dr *DependencyResolver) removeDuplicates(slice []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

func (dr *DependencyResolver) findLatestVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}

	// Sort versions and return the latest
	sort.Strings(versions)
	return versions[len(versions)-1]
}

// SetConstraint sets a dependency constraint
func (dr *DependencyResolver) SetConstraint(constraint *DependencyConstraint) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	dr.constraints[constraint.PackageID] = constraint
}

// GetInstalledPackages returns all installed packages
func (dr *DependencyResolver) GetInstalledPackages() map[string]*InstalledPackage {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	// Return a copy to avoid races
	result := make(map[string]*InstalledPackage)
	for k, v := range dr.installed {
		result[k] = v
	}

	return result
}

// ClearCache clears the resolution cache
func (dr *DependencyResolver) ClearCache() {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	dr.cache = make(map[string]*ResolutionResult)
}
