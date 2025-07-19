// Package pkg provides dependency analysis tools
package pkg

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// DependencyGraph represents the package dependency structure
type DependencyGraph struct {
	// Package name -> list of imported packages
	Dependencies map[string][]string
	// Package name -> list of packages that import it
	Dependents map[string][]string
}

// NewDependencyGraph creates a new dependency graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		Dependencies: make(map[string][]string),
		Dependents:   make(map[string][]string),
	}
}

// AnalyzePackageDependencies analyzes the dependency structure of the SDK
func AnalyzePackageDependencies(rootPath string) (*DependencyGraph, error) {
	graph := NewDependencyGraph()
	
	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		// Skip non-Go files
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		
		// Skip vendor and other non-source directories
		if strings.Contains(path, "/vendor/") || strings.Contains(path, "/.git/") {
			return nil
		}
		
		// Read file to check for build tags
		content, err := os.ReadFile(path)
		if err == nil && len(content) > 0 {
			// Skip files with "//go:build ignore" or "// +build ignore"
			contentStr := string(content[:min(200, len(content))])
			if strings.Contains(contentStr, "//go:build ignore") || strings.Contains(contentStr, "// +build ignore") {
				return nil
			}
		}
		
		// Parse the file
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return nil // Skip files that can't be parsed
		}
		
		// Get package path
		pkgPath := getPackagePath(rootPath, path)
		
		// Extract imports
		for _, imp := range node.Imports {
			if imp.Path != nil {
				importPath := strings.Trim(imp.Path.Value, `"`)
				
				// Only track internal imports
				if strings.HasPrefix(importPath, "github.com/ag-ui/go-sdk/") {
					// Skip self-imports in test files (which is normal for _test packages)
					if strings.HasSuffix(path, "_test.go") && importPath == pkgPath {
						continue
					}
					
					// Add to dependencies
					if graph.Dependencies[pkgPath] == nil {
						graph.Dependencies[pkgPath] = []string{}
					}
					if !contains(graph.Dependencies[pkgPath], importPath) {
						graph.Dependencies[pkgPath] = append(graph.Dependencies[pkgPath], importPath)
					}
					
					// Add to dependents
					if graph.Dependents[importPath] == nil {
						graph.Dependents[importPath] = []string{}
					}
					if !contains(graph.Dependents[importPath], pkgPath) {
						graph.Dependents[importPath] = append(graph.Dependents[importPath], pkgPath)
					}
				}
			}
		}
		
		return nil
	})
	
	return graph, err
}

// FindCircularDependencies finds circular dependencies in the graph
func (g *DependencyGraph) FindCircularDependencies() [][]string {
	var cycles [][]string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := []string{}
	
	for pkg := range g.Dependencies {
		if !visited[pkg] {
			g.findCyclesDFS(pkg, visited, recStack, &path, &cycles)
		}
	}
	
	return cycles
}

func (g *DependencyGraph) findCyclesDFS(pkg string, visited, recStack map[string]bool, path *[]string, cycles *[][]string) {
	visited[pkg] = true
	recStack[pkg] = true
	*path = append(*path, pkg)
	
	for _, dep := range g.Dependencies[pkg] {
		if !visited[dep] {
			g.findCyclesDFS(dep, visited, recStack, path, cycles)
		} else if recStack[dep] {
			// Found a cycle
			cycleStart := -1
			for i, p := range *path {
				if p == dep {
					cycleStart = i
					break
				}
			}
			if cycleStart != -1 {
				cycle := make([]string, len(*path)-cycleStart)
				copy(cycle, (*path)[cycleStart:])
				*cycles = append(*cycles, cycle)
			}
		}
	}
	
	recStack[pkg] = false
	*path = (*path)[:len(*path)-1]
}

// PrintDependencyReport prints a human-readable dependency report
func (g *DependencyGraph) PrintDependencyReport() string {
	var report strings.Builder
	
	report.WriteString("Package Dependency Analysis\n")
	report.WriteString("===========================\n\n")
	
	// Find core packages
	corePackages := []string{
		"github.com/ag-ui/go-sdk/pkg/core",
		"github.com/ag-ui/go-sdk/pkg/core/events",
		"github.com/ag-ui/go-sdk/pkg/transport",
		"github.com/ag-ui/go-sdk/pkg/state",
		"github.com/ag-ui/go-sdk/pkg/messages",
		"github.com/ag-ui/go-sdk/pkg/tools",
	}
	
	report.WriteString("Core Package Dependencies:\n")
	report.WriteString("--------------------------\n")
	for _, pkg := range corePackages {
		if deps, ok := g.Dependencies[pkg]; ok && len(deps) > 0 {
			report.WriteString(fmt.Sprintf("\n%s imports:\n", pkg))
			for _, dep := range deps {
				report.WriteString(fmt.Sprintf("  - %s\n", dep))
			}
		}
	}
	
	// Check for circular dependencies
	cycles := g.FindCircularDependencies()
	report.WriteString("\n\nCircular Dependencies:\n")
	report.WriteString("----------------------\n")
	if len(cycles) == 0 {
		report.WriteString("No circular dependencies found!\n")
	} else {
		for i, cycle := range cycles {
			report.WriteString(fmt.Sprintf("\nCycle %d:\n", i+1))
			for j, pkg := range cycle {
				if j > 0 {
					report.WriteString(" -> ")
				}
				report.WriteString(pkg)
			}
			report.WriteString("\n")
		}
	}
	
	// Cross-package dependencies
	report.WriteString("\n\nCross-Package Dependencies:\n")
	report.WriteString("---------------------------\n")
	
	// Transport -> Events
	if deps := g.getDependencies("github.com/ag-ui/go-sdk/pkg/transport", "github.com/ag-ui/go-sdk/pkg/core/events"); len(deps) > 0 {
		report.WriteString("\nTransport -> Events:\n")
		for _, dep := range deps {
			report.WriteString(fmt.Sprintf("  - %s\n", dep))
		}
	}
	
	// State -> Events
	if deps := g.getDependencies("github.com/ag-ui/go-sdk/pkg/state", "github.com/ag-ui/go-sdk/pkg/core/events"); len(deps) > 0 {
		report.WriteString("\nState -> Events:\n")
		for _, dep := range deps {
			report.WriteString(fmt.Sprintf("  - %s\n", dep))
		}
	}
	
	// State -> Transport
	if deps := g.getDependencies("github.com/ag-ui/go-sdk/pkg/state", "github.com/ag-ui/go-sdk/pkg/transport"); len(deps) > 0 {
		report.WriteString("\nState -> Transport:\n")
		for _, dep := range deps {
			report.WriteString(fmt.Sprintf("  - %s\n", dep))
		}
	}
	
	return report.String()
}

func (g *DependencyGraph) getDependencies(fromPkg, toPkg string) []string {
	var result []string
	if deps, ok := g.Dependencies[fromPkg]; ok {
		for _, dep := range deps {
			if strings.HasPrefix(dep, toPkg) {
				result = append(result, dep)
			}
		}
	}
	return result
}

func getPackagePath(rootPath, filePath string) string {
	// Extract package path from file path
	rel, _ := filepath.Rel(rootPath, filePath)
	dir := filepath.Dir(rel)
	return "github.com/ag-ui/go-sdk/" + strings.ReplaceAll(dir, string(filepath.Separator), "/")
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}