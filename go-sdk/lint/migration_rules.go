// Package lint provides migration-specific linting rules for interface{} to type-safe alternatives.
package lint

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// MigrationRulesAnalyzer detects incomplete migrations and mixed usage patterns.
var MigrationRulesAnalyzer = &analysis.Analyzer{
	Name:     "migration",
	Doc:      "detects incomplete migrations and mixed API usage patterns",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      runMigrationAnalysis,
}

// MigrationIssue represents a migration-specific issue.
type MigrationIssue struct {
	Type        string    // incomplete, mixed-usage, inconsistent
	Pos         token.Pos
	Message     string
	Suggestion  string
	Priority    string    // high, medium, low
	FilePath    string
	LineNumber  int
	Category    string
	OldPattern  string
	NewPattern  string
}

// MigrationContext tracks the state of migration across files.
type MigrationContext struct {
	// Files that have been partially migrated
	PartiallyMigrated map[string]bool
	// Files that use old patterns
	LegacyFiles map[string][]string
	// Files that use new patterns
	ModernFiles map[string][]string
	// Mixed usage within single files
	MixedUsage map[string][]MigrationIssue
}

// Priority package paths for migration (high priority first)
var priorityPaths = []string{
	"pkg/messages/",
	"pkg/state/", 
	"pkg/transport/",
	"pkg/events/",
	"pkg/client/",
	"pkg/server/",
}

// Legacy patterns that should be migrated
var legacyPatterns = map[string]string{
	`interface\{\}`:                    "specific types or generics",
	`\[\]interface\{\}`:               "typed slices",
	`map\[string\]interface\{\}`:      "typed structs",
	`\.Any\(`:                         "typed logger methods",
	`logrus\.Any\(`:                   "typed logrus methods",
	`json\.Unmarshal.*interface\{\}`:  "typed JSON unmarshaling",
	`reflect\.ValueOf.*interface\{\}`: "typed reflection",
}

// Modern patterns that indicate successful migration
var modernPatterns = []string{
	`\[T any\]`,              // generics
	`\[T .*\]`,               // generic constraints
	`struct\s*\{`,            // typed structs
	`\.String\(`,             // typed logger
	`\.Int\(`,                // typed logger
	`\.Bool\(`,               // typed logger
	`\.Float64\(`,            // typed logger
	`json\.Unmarshal.*\&\w+`, // typed unmarshaling
}

// runMigrationAnalysis performs migration-specific analysis.
func runMigrationAnalysis(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	
	ctx := &MigrationContext{
		PartiallyMigrated: make(map[string]bool),
		LegacyFiles:      make(map[string][]string),
		ModernFiles:      make(map[string][]string),
		MixedUsage:       make(map[string][]MigrationIssue),
	}

	// Compile patterns
	legacyRegexes := make(map[*regexp.Regexp]string)
	for pattern, suggestion := range legacyPatterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid legacy pattern %q: %v", pattern, err)
		}
		legacyRegexes[regex] = suggestion
	}

	modernRegexes := make([]*regexp.Regexp, len(modernPatterns))
	for i, pattern := range modernPatterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid modern pattern %q: %v", pattern, err)
		}
		modernRegexes[i] = regex
	}

	// Track issues
	var issues []MigrationIssue

	// Analyze all files for migration patterns
	nodeFilter := []ast.Node{
		(*ast.File)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		if file, ok := n.(*ast.File); ok {
			analyzeFileForMigration(pass, file, ctx, legacyRegexes, modernRegexes, &issues)
		}
	})

	// Analyze migration completeness across the project
	analyzeProjectMigrationStatus(pass, ctx, &issues)

	// Report all migration issues
	for _, issue := range issues {
		pass.Reportf(issue.Pos, "[%s] %s: %s. Suggestion: %s", 
			issue.Priority, issue.Category, issue.Message, issue.Suggestion)
	}

	return ctx, nil
}

// analyzeFileForMigration analyzes a single file for migration patterns.
func analyzeFileForMigration(pass *analysis.Pass, file *ast.File, ctx *MigrationContext, 
	legacyRegexes map[*regexp.Regexp]string, modernRegexes []*regexp.Regexp, issues *[]MigrationIssue) {
	
	fset := pass.Fset
	filename := fset.Position(file.Pos()).Filename
	
	// Get file content as string for pattern matching
	// Read file content (simplified - in practice you'd want to use the token.FileSet properly)
	fileContent := getFileContent(pass, file)
	
	// Track patterns found in this file
	legacyCount := 0
	modernCount := 0
	var legacyMatches []string
	var modernMatches []string

	// Check for legacy patterns
	for regex, suggestion := range legacyRegexes {
		matches := regex.FindAllString(fileContent, -1)
		if len(matches) > 0 {
			legacyCount += len(matches)
			legacyMatches = append(legacyMatches, matches...)
			
			// Create issue for each match
			for _, match := range matches {
				pos := file.Pos() // Simplified position
				issue := MigrationIssue{
					Type:       "legacy-usage",
					Pos:        pos,
					Message:    fmt.Sprintf("legacy pattern found: %s", match),
					Suggestion: fmt.Sprintf("migrate to %s", suggestion),
					Priority:   getMigrationPriority(filename),
					FilePath:   filename,
					Category:   "migration-needed",
					OldPattern: match,
					NewPattern: suggestion,
				}
				*issues = append(*issues, issue)
			}
		}
	}

	// Check for modern patterns
	for _, regex := range modernRegexes {
		matches := regex.FindAllString(fileContent, -1)
		if len(matches) > 0 {
			modernCount += len(matches)
			modernMatches = append(modernMatches, matches...)
		}
	}

	// Determine file migration status
	if legacyCount > 0 && modernCount > 0 {
		// Mixed usage - this is concerning
		ctx.PartiallyMigrated[filename] = true
		ctx.MixedUsage[filename] = []MigrationIssue{{
			Type:       "mixed-usage",
			Pos:        file.Pos(),
			Message:    fmt.Sprintf("file contains both legacy (%d) and modern (%d) patterns", legacyCount, modernCount),
			Suggestion: "complete migration by replacing all legacy patterns",
			Priority:   "high",
			FilePath:   filename,
			Category:   "mixed-usage",
		}}
		
		*issues = append(*issues, ctx.MixedUsage[filename]...)
		
	} else if legacyCount > 0 {
		// Legacy file
		ctx.LegacyFiles[filename] = legacyMatches
	} else if modernCount > 0 {
		// Modern file
		ctx.ModernFiles[filename] = modernMatches
	}

	// Check for incomplete migration patterns
	checkIncompletePatterns(pass, file, issues, filename)
}

// checkIncompletePatterns looks for specific incomplete migration patterns.
func checkIncompletePatterns(pass *analysis.Pass, file *ast.File, issues *[]MigrationIssue, filename string) {
	// Check for struct definitions that could replace map[string]interface{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.MapType:
			if isStringInterfaceMap(node) {
				// Check if there's a corresponding struct that could be used
				if hasNearbyStruct(file, node) {
					issue := MigrationIssue{
						Type:       "incomplete",
						Pos:        node.Pos(),
						Message:    "map[string]interface{} found with potential struct alternative",
						Suggestion: "consider using the nearby struct type instead",
						Priority:   getMigrationPriority(filename),
						FilePath:   filename,
						Category:   "incomplete-migration",
						OldPattern: "map[string]interface{}",
						NewPattern: "typed struct",
					}
					*issues = append(*issues, issue)
				}
			}
		case *ast.FuncDecl:
			// Check for functions that accept interface{} but could use generics
			if hasInterfaceParams(node) && couldUseGenerics(node) {
				issue := MigrationIssue{
					Type:       "incomplete",
					Pos:        node.Pos(),
					Message:    "function uses interface{} parameters but could use generics",
					Suggestion: "consider using type parameters for better type safety",
					Priority:   getMigrationPriority(filename),
					FilePath:   filename,
					Category:   "generics-opportunity",
					OldPattern: "func(...interface{})",
					NewPattern: "func[T any](...T)",
				}
				*issues = append(*issues, issue)
			}
		}
		return true
	})
}

// analyzeProjectMigrationStatus provides project-wide migration insights.
func analyzeProjectMigrationStatus(pass *analysis.Pass, ctx *MigrationContext, issues *[]MigrationIssue) {
	totalFiles := len(ctx.LegacyFiles) + len(ctx.ModernFiles) + len(ctx.PartiallyMigrated)
	
	if totalFiles == 0 {
		return
	}

	modernFiles := len(ctx.ModernFiles)
	mixedFiles := len(ctx.PartiallyMigrated)
	
	migrationProgress := float64(modernFiles) / float64(totalFiles) * 100

	// Report overall project status
	if migrationProgress < 25 {
		issue := MigrationIssue{
			Type:       "project-status",
			Pos:        token.NoPos,
			Message:    fmt.Sprintf("low migration progress: %.1f%% (%d/%d files)", migrationProgress, modernFiles, totalFiles),
			Suggestion: "prioritize migrating high-usage files first",
			Priority:   "high",
			Category:   "project-migration",
		}
		*issues = append(*issues, issue)
	}

	// Report mixed usage files as high priority
	if mixedFiles > 0 {
		issue := MigrationIssue{
			Type:       "project-status",
			Pos:        token.NoPos,
			Message:    fmt.Sprintf("%d files have mixed legacy/modern usage", mixedFiles),
			Suggestion: "complete migration in mixed-usage files to avoid inconsistency",
			Priority:   "high",
			Category:   "consistency",
		}
		*issues = append(*issues, issue)
	}

	// Suggest migration order based on priority paths
	for _, priorityPath := range priorityPaths {
		pathLegacyCount := 0
		for filename := range ctx.LegacyFiles {
			if strings.Contains(filename, priorityPath) {
				pathLegacyCount++
			}
		}
		
		if pathLegacyCount > 0 {
			issue := MigrationIssue{
				Type:       "migration-order",
				Pos:        token.NoPos,
				Message:    fmt.Sprintf("priority path %s has %d legacy files", priorityPath, pathLegacyCount),
				Suggestion: fmt.Sprintf("prioritize migrating files in %s", priorityPath),
				Priority:   "medium",
				Category:   "migration-planning",
			}
			*issues = append(*issues, issue)
		}
	}
}

// Helper functions

// getMigrationPriority determines migration priority based on file path.
func getMigrationPriority(filename string) string {
	for _, priorityPath := range priorityPaths {
		if strings.Contains(filename, priorityPath) {
			return "high"
		}
	}
	
	if strings.Contains(filename, "_test.go") {
		return "low"
	}
	
	return "medium"
}

// isStringInterfaceMap checks if a map type is map[string]interface{}.
func isStringInterfaceMap(mapType *ast.MapType) bool {
	// Check key type is string
	if ident, ok := mapType.Key.(*ast.Ident); !ok || ident.Name != "string" {
		return false
	}
	
	// Check value type is interface{}
	return isEmptyInterface(mapType.Value)
}

// hasNearbyStruct checks if there's a struct definition near the map usage.
func hasNearbyStruct(file *ast.File, mapNode *ast.MapType) bool {
	// Simplified check - in practice you'd want more sophisticated analysis
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if _, ok := n.(*ast.StructType); ok {
			found = true
			return false
		}
		return true
	})
	return found
}

// hasInterfaceParams checks if a function has interface{} parameters.
func hasInterfaceParams(funcDecl *ast.FuncDecl) bool {
	if funcDecl.Type.Params == nil {
		return false
	}
	
	for _, field := range funcDecl.Type.Params.List {
		if isEmptyInterface(field.Type) {
			return true
		}
	}
	return false
}

// couldUseGenerics determines if a function could benefit from generics.
func couldUseGenerics(funcDecl *ast.FuncDecl) bool {
	// Simple heuristic: if function name suggests generic behavior
	if funcDecl.Name == nil {
		return false
	}
	
	name := strings.ToLower(funcDecl.Name.Name)
	genericIndicators := []string{"process", "handle", "convert", "transform", "map", "filter", "reduce"}
	
	for _, indicator := range genericIndicators {
		if strings.Contains(name, indicator) {
			return true
		}
	}
	
	return false
}

// getFileContent extracts file content as string (simplified implementation).
func getFileContent(pass *analysis.Pass, file *ast.File) string {
	// In a real implementation, you'd read the actual file content
	// For now, we'll return a placeholder that would be filled by actual file reading
	return "" // This would be replaced with actual file content reading
}

// GenerateMigrationPlan creates a migration plan based on analysis results.
func GenerateMigrationPlan(ctx *MigrationContext) string {
	var plan strings.Builder
	
	plan.WriteString("# Interface{} Migration Plan\n\n")
	
	plan.WriteString("## Current Status\n")
	plan.WriteString(fmt.Sprintf("- Legacy files: %d\n", len(ctx.LegacyFiles)))
	plan.WriteString(fmt.Sprintf("- Modern files: %d\n", len(ctx.ModernFiles)))
	plan.WriteString(fmt.Sprintf("- Mixed usage files: %d\n", len(ctx.PartiallyMigrated)))
	
	plan.WriteString("\n## Priority Actions\n")
	plan.WriteString("1. Complete migration in mixed-usage files\n")
	plan.WriteString("2. Migrate high-priority packages (messages, state, transport)\n") 
	plan.WriteString("3. Migrate remaining legacy files\n")
	
	plan.WriteString("\n## Mixed Usage Files (High Priority)\n")
	for filename := range ctx.MixedUsage {
		plan.WriteString(fmt.Sprintf("- %s\n", filename))
	}
	
	plan.WriteString("\n## Legacy Files by Priority\n")
	for _, priorityPath := range priorityPaths {
		plan.WriteString(fmt.Sprintf("\n### %s\n", priorityPath))
		for filename := range ctx.LegacyFiles {
			if strings.Contains(filename, priorityPath) {
				plan.WriteString(fmt.Sprintf("- %s\n", filename))
			}
		}
	}
	
	return plan.String()
}