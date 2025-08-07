// Package lint provides custom static analysis tools for type safety enforcement.
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

// TypeSafetyAnalyzer detects interface{} patterns and suggests type-safe alternatives.
var TypeSafetyAnalyzer = &analysis.Analyzer{
	Name:     "typesafety",
	Doc:      "detects interface{} usage and suggests type-safe alternatives",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      runTypeSafetyAnalysis,
}

// AnalysisConfig holds configuration for the type safety analyzer.
type AnalysisConfig struct {
	// AllowedPatterns are regex patterns for allowed interface{} usage
	AllowedPatterns []string
	// ForbiddenPatterns are specific patterns to flag as errors
	ForbiddenPatterns []string
	// SuggestAlternatives enables auto-fix suggestions
	SuggestAlternatives bool
	// StrictMode enables stricter checking
	StrictMode bool
}

// Default configuration for the analyzer
var defaultConfig = &AnalysisConfig{
	AllowedPatterns: []string{
		`json\.(Marshal|Unmarshal)`,
		`context\.WithValue`,
		`reflect\.`,
		`fmt\.(Sprintf|Printf|Print)`,
		`testing\.`,
		`_test\.go$`,
	},
	ForbiddenPatterns: []string{
		`interface\{\}`,
		`\[\]interface\{\}`,
		`map\[string\]interface\{\}`,
		`map\[.*\]interface\{\}`,
		`\.Any\(`,
		`logrus\.Any\(`,
		`log\.Any\(`,
	},
	SuggestAlternatives: true,
	StrictMode:          false,
}

// Issue represents a type safety issue found in the code.
type Issue struct {
	Pos          token.Pos
	Message      string
	Suggestion   string
	Severity     string
	Category     string
	FixAction    string
	FilePath     string
	LineNumber   int
	ColumnNumber int
}

// runTypeSafetyAnalysis is the main analysis function.
func runTypeSafetyAnalysis(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Create regex patterns for matching
	allowedRegexes := make([]*regexp.Regexp, len(defaultConfig.AllowedPatterns))
	for i, pattern := range defaultConfig.AllowedPatterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid allowed pattern %q: %v", pattern, err)
		}
		allowedRegexes[i] = regex
	}

	forbiddenRegexes := make([]*regexp.Regexp, len(defaultConfig.ForbiddenPatterns))
	for i, pattern := range defaultConfig.ForbiddenPatterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid forbidden pattern %q: %v", pattern, err)
		}
		forbiddenRegexes[i] = regex
	}

	// Track issues found
	var issues []Issue

	// Node types to inspect
	nodeFilter := []ast.Node{
		(*ast.InterfaceType)(nil),
		(*ast.ArrayType)(nil),
		(*ast.MapType)(nil),
		(*ast.CallExpr)(nil),
		(*ast.TypeAssertExpr)(nil),
		(*ast.FuncDecl)(nil),
		(*ast.FuncType)(nil),
		(*ast.FieldList)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		fset := pass.Fset
		filename := fset.Position(n.Pos()).Filename

		// Skip if file is in allowed patterns (like test files)
		for _, regex := range allowedRegexes {
			if regex.MatchString(filename) {
				return
			}
		}

		switch node := n.(type) {
		case *ast.InterfaceType:
			analyzeInterfaceType(pass, node, &issues, fset)
		case *ast.ArrayType:
			analyzeArrayType(pass, node, &issues, fset)
		case *ast.MapType:
			analyzeMapType(pass, node, &issues, fset)
		case *ast.CallExpr:
			analyzeCallExpr(pass, node, &issues, fset, forbiddenRegexes)
		case *ast.TypeAssertExpr:
			analyzeTypeAssertion(pass, node, &issues, fset)
		case *ast.FuncDecl:
			analyzeFuncDecl(pass, node, &issues, fset)
		case *ast.FuncType:
			analyzeFuncType(pass, node, &issues, fset)
		}
	})

	// Report all issues found
	for _, issue := range issues {
		pass.Reportf(issue.Pos, "%s: %s. %s", issue.Category, issue.Message, issue.Suggestion)
	}

	return nil, nil
}

// analyzeInterfaceType checks for empty interface{} usage.
func analyzeInterfaceType(pass *analysis.Pass, node *ast.InterfaceType, issues *[]Issue, fset *token.FileSet) {
	// Check if it's an empty interface
	if len(node.Methods.List) == 0 {
		pos := fset.Position(node.Pos())
		issue := Issue{
			Pos:          node.Pos(),
			Message:      "empty interface{} detected",
			Suggestion:   "consider using specific types, interfaces with methods, or generics",
			Severity:     "error",
			Category:     "type-safety",
			FixAction:    "replace-with-typed-alternative",
			FilePath:     pos.Filename,
			LineNumber:   pos.Line,
			ColumnNumber: pos.Column,
		}

		// Add context-specific suggestions
		context := getNodeContext(pass, node)
		if context != "" {
			issue.Suggestion += fmt.Sprintf(" (context: %s)", context)
		}

		*issues = append(*issues, issue)
	}
}

// analyzeArrayType checks for []interface{} usage.
func analyzeArrayType(pass *analysis.Pass, node *ast.ArrayType, issues *[]Issue, fset *token.FileSet) {
	if isEmptyInterface(node.Elt) {
		pos := fset.Position(node.Pos())
		issue := Issue{
			Pos:          node.Pos(),
			Message:      "[]interface{} slice detected",
			Suggestion:   "use typed slices ([]string, []int) or generics ([]T)",
			Severity:     "error",
			Category:     "type-safety",
			FixAction:    "replace-with-typed-slice",
			FilePath:     pos.Filename,
			LineNumber:   pos.Line,
			ColumnNumber: pos.Column,
		}

		*issues = append(*issues, issue)
	}
}

// analyzeMapType checks for map[string]interface{} usage.
func analyzeMapType(pass *analysis.Pass, node *ast.MapType, issues *[]Issue, fset *token.FileSet) {
	if isEmptyInterface(node.Value) {
		pos := fset.Position(node.Pos())
		keyType := getTypeString(node.Key)

		issue := Issue{
			Pos:          node.Pos(),
			Message:      fmt.Sprintf("map[%s]interface{} detected", keyType),
			Suggestion:   "use typed structs or specific map value types",
			Severity:     "error",
			Category:     "type-safety",
			FixAction:    "replace-with-typed-map",
			FilePath:     pos.Filename,
			LineNumber:   pos.Line,
			ColumnNumber: pos.Column,
		}

		// Add specific suggestions based on key type
		if keyType == "string" {
			issue.Suggestion += " (consider using structs for string-keyed data)"
		}

		*issues = append(*issues, issue)
	}
}

// analyzeCallExpr checks for deprecated logging methods and other patterns.
func analyzeCallExpr(pass *analysis.Pass, node *ast.CallExpr, issues *[]Issue, fset *token.FileSet, forbiddenRegexes []*regexp.Regexp) {
	// Get the call expression as string
	callStr := getCallString(node)

	for _, regex := range forbiddenRegexes {
		if regex.MatchString(callStr) {
			pos := fset.Position(node.Pos())

			var suggestion string
			var category string

			switch {
			case strings.Contains(callStr, ".Any("):
				suggestion = "use typed logging methods like .String(), .Int(), .Bool() instead"
				category = "deprecated-api"
			case strings.Contains(callStr, "interface{}"):
				suggestion = "avoid interface{} in function calls, use specific types"
				category = "type-safety"
			default:
				suggestion = "consider using type-safe alternatives"
				category = "type-safety"
			}

			issue := Issue{
				Pos:          node.Pos(),
				Message:      fmt.Sprintf("forbidden pattern detected: %s", callStr),
				Suggestion:   suggestion,
				Severity:     "warning",
				Category:     category,
				FixAction:    "replace-with-typed-alternative",
				FilePath:     pos.Filename,
				LineNumber:   pos.Line,
				ColumnNumber: pos.Column,
			}

			*issues = append(*issues, issue)
		}
	}
}

// analyzeTypeAssertion checks for unsafe type assertions.
func analyzeTypeAssertion(pass *analysis.Pass, node *ast.TypeAssertExpr, issues *[]Issue, fset *token.FileSet) {
	// Check if this is a type assertion without comma ok idiom
	parent := getParentNode(pass, node)

	// Look for assignment patterns
	if assign, ok := parent.(*ast.AssignStmt); ok {
		// Check if it's using the comma ok idiom
		if len(assign.Lhs) == 1 && len(assign.Rhs) == 1 {
			pos := fset.Position(node.Pos())
			issue := Issue{
				Pos:          node.Pos(),
				Message:      "unsafe type assertion without ok check",
				Suggestion:   "use val, ok := x.(Type) pattern for safe type assertions",
				Severity:     "warning",
				Category:     "safety",
				FixAction:    "add-ok-check",
				FilePath:     pos.Filename,
				LineNumber:   pos.Line,
				ColumnNumber: pos.Column,
			}

			*issues = append(*issues, issue)
		}
	}
}

// analyzeFuncDecl checks function declarations for interface{} parameters/returns.
func analyzeFuncDecl(pass *analysis.Pass, node *ast.FuncDecl, issues *[]Issue, fset *token.FileSet) {
	if node.Type != nil {
		analyzeFuncType(pass, node.Type, issues, fset)
	}
}

// analyzeFuncType checks function types for interface{} usage.
func analyzeFuncType(pass *analysis.Pass, node *ast.FuncType, issues *[]Issue, fset *token.FileSet) {
	// Check parameters
	if node.Params != nil {
		for _, field := range node.Params.List {
			if isEmptyInterface(field.Type) {
				pos := fset.Position(field.Pos())
				issue := Issue{
					Pos:          field.Pos(),
					Message:      "function parameter uses interface{}",
					Suggestion:   "use specific parameter types or generics",
					Severity:     "warning",
					Category:     "type-safety",
					FixAction:    "replace-with-typed-parameter",
					FilePath:     pos.Filename,
					LineNumber:   pos.Line,
					ColumnNumber: pos.Column,
				}

				*issues = append(*issues, issue)
			}
		}
	}

	// Check return types
	if node.Results != nil {
		for _, field := range node.Results.List {
			if isEmptyInterface(field.Type) {
				pos := fset.Position(field.Pos())
				issue := Issue{
					Pos:          field.Pos(),
					Message:      "function return type uses interface{}",
					Suggestion:   "use specific return types or generics",
					Severity:     "warning",
					Category:     "type-safety",
					FixAction:    "replace-with-typed-return",
					FilePath:     pos.Filename,
					LineNumber:   pos.Line,
					ColumnNumber: pos.Column,
				}

				*issues = append(*issues, issue)
			}
		}
	}
}

// Helper functions

// isEmptyInterface checks if a type expression represents interface{}.
func isEmptyInterface(expr ast.Expr) bool {
	if iface, ok := expr.(*ast.InterfaceType); ok {
		return len(iface.Methods.List) == 0
	}
	return false
}

// getTypeString returns a string representation of a type expression.
func getTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return getTypeString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + getTypeString(t.Elt)
	case *ast.MapType:
		return "map[" + getTypeString(t.Key) + "]" + getTypeString(t.Value)
	case *ast.InterfaceType:
		if len(t.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface{...}"
	default:
		return "unknown"
	}
}

// getCallString returns a string representation of a call expression.
func getCallString(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return getTypeString(fun.X) + "." + fun.Sel.Name
	default:
		return "unknown"
	}
}

// getNodeContext analyzes the context where interface{} is used to provide better suggestions.
func getNodeContext(pass *analysis.Pass, node ast.Node) string {
	// This is a simplified context analysis
	// In a real implementation, you might want to walk up the AST
	// to understand the broader context

	// Check if it's in a struct field
	parent := getParentNode(pass, node)
	if _, ok := parent.(*ast.Field); ok {
		return "struct field"
	}

	// Check if it's in a function parameter
	if _, ok := parent.(*ast.FuncType); ok {
		return "function signature"
	}

	return ""
}

// getParentNode attempts to find the parent node (simplified implementation).
func getParentNode(pass *analysis.Pass, node ast.Node) ast.Node {
	// This is a simplified implementation
	// In practice, you might need to maintain a parent map during traversal
	return nil
}

// GenerateFixSuggestion generates specific fix suggestions based on the issue type.
func GenerateFixSuggestion(issue Issue) string {
	switch issue.FixAction {
	case "replace-with-typed-alternative":
		return "// TODO: Replace interface{} with specific type\n// Example: interface{} -> string, int, CustomStruct, etc."
	case "replace-with-typed-slice":
		return "// TODO: Replace []interface{} with typed slice\n// Example: []interface{} -> []string, []int, []CustomType"
	case "replace-with-typed-map":
		return "// TODO: Replace map[K]interface{} with typed map or struct\n// Example: map[string]interface{} -> CustomStruct or map[string]string"
	case "add-ok-check":
		return "// TODO: Add ok check for safe type assertion\n// Example: val := x.(Type) -> val, ok := x.(Type); if !ok { handle error }"
	case "replace-with-typed-parameter":
		return "// TODO: Replace interface{} parameter with specific type\n// Example: func(v interface{}) -> func(v string) or func[T any](v T)"
	case "replace-with-typed-return":
		return "// TODO: Replace interface{} return with specific type\n// Example: func() interface{} -> func() string or func[T any]() T"
	default:
		return "// TODO: Replace with type-safe alternative"
	}
}
