// Package tools provides analysis utilities for interface{} usage patterns
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// AnalysisConfig holds configuration for the analysis process
type AnalysisConfig struct {
	// Directory is the root directory to analyze
	Directory string
	
	// Recursive indicates whether to process subdirectories
	Recursive bool
	
	// OutputFile is the file to write the analysis report
	OutputFile string
	
	// OutputFormat specifies the output format (json, text, csv)
	OutputFormat string
	
	// IncludePatterns are patterns for files to include
	IncludePatterns []string
	
	// ExcludePatterns are patterns for files to exclude
	ExcludePatterns []string
	
	// Verbose enables detailed logging
	Verbose bool
	
	// ShowExamples includes code examples in the output
	ShowExamples bool
	
	// MaxExamples limits the number of examples per pattern
	MaxExamples int
}

// InterfaceUsage represents a single instance of interface{} usage
type InterfaceUsage struct {
	// File is the path to the file containing this usage
	File string `json:"file"`
	
	// Line is the line number
	Line int `json:"line"`
	
	// Column is the column number
	Column int `json:"column"`
	
	// Context is the surrounding code context
	Context string `json:"context"`
	
	// Pattern is the type of interface{} usage pattern
	Pattern string `json:"pattern"`
	
	// RiskLevel indicates the migration risk (high/medium/low)
	RiskLevel string `json:"risk_level"`
	
	// Description explains the usage pattern
	Description string `json:"description"`
	
	// Function is the function containing this usage (if applicable)
	Function string `json:"function,omitempty"`
	
	// Package is the package name
	Package string `json:"package"`
	
	// Suggestions for migration
	Suggestions []string `json:"suggestions"`
}

// PackageAnalysis contains analysis results for a single package
type PackageAnalysis struct {
	// Package name
	Package string `json:"package"`
	
	// Path to the package
	Path string `json:"path"`
	
	// Files analyzed in this package
	Files []string `json:"files"`
	
	// Total interface{} usages in this package
	TotalUsages int `json:"total_usages"`
	
	// Breakdown by pattern
	PatternBreakdown map[string]int `json:"pattern_breakdown"`
	
	// Breakdown by risk level
	RiskBreakdown map[string]int `json:"risk_breakdown"`
	
	// List of all usages
	Usages []InterfaceUsage `json:"usages"`
}

// AnalysisReport contains the complete analysis results
type AnalysisReport struct {
	// Timestamp of the analysis
	Timestamp string `json:"timestamp"`
	
	// Config used for the analysis
	Config AnalysisConfig `json:"config"`
	
	// Summary statistics
	Summary AnalysisSummary `json:"summary"`
	
	// Per-package analysis
	Packages []PackageAnalysis `json:"packages"`
	
	// Global patterns found
	Patterns []PatternAnalysis `json:"patterns"`
	
	// Migration recommendations
	Recommendations []string `json:"recommendations"`
}

// AnalysisSummary contains high-level statistics
type AnalysisSummary struct {
	// FilesAnalyzed is the total number of files analyzed
	FilesAnalyzed int `json:"files_analyzed"`
	
	// PackagesAnalyzed is the total number of packages analyzed
	PackagesAnalyzed int `json:"packages_analyzed"`
	
	// TotalUsages is the total number of interface{} usages found
	TotalUsages int `json:"total_usages"`
	
	// PatternBreakdown shows usage by pattern type
	PatternBreakdown map[string]int `json:"pattern_breakdown"`
	
	// RiskBreakdown shows usage by risk level
	RiskBreakdown map[string]int `json:"risk_breakdown"`
	
	// FileBreakdown shows files with the most usages
	FileBreakdown []FileUsageCount `json:"file_breakdown"`
	
	// PackageBreakdown shows packages with the most usages
	PackageBreakdown []PackageUsageCount `json:"package_breakdown"`
}

// FileUsageCount represents usage count for a file
type FileUsageCount struct {
	File  string `json:"file"`
	Count int    `json:"count"`
}

// PackageUsageCount represents usage count for a package
type PackageUsageCount struct {
	Package string `json:"package"`
	Count   int    `json:"count"`
}

// PatternAnalysis contains analysis for a specific pattern
type PatternAnalysis struct {
	// Pattern name
	Pattern string `json:"pattern"`
	
	// Description of the pattern
	Description string `json:"description"`
	
	// Risk level of this pattern
	RiskLevel string `json:"risk_level"`
	
	// Count of occurrences
	Count int `json:"count"`
	
	// Examples of this pattern
	Examples []InterfaceUsage `json:"examples"`
	
	// Migration suggestions
	MigrationSuggestions []string `json:"migration_suggestions"`
	
	// Difficulty of migration (1-5 scale)
	MigrationDifficulty int `json:"migration_difficulty"`
}

// Pattern definitions for different types of interface{} usage
var usagePatterns = []struct {
	Name        string
	Regex       *regexp.Regexp
	RiskLevel   string
	Description string
	Suggestions []string
	Difficulty  int
}{
	{
		Name:        "map_string_interface",
		Regex:       regexp.MustCompile(`map\[string\]interface\{\}`),
		RiskLevel:   "medium",
		Description: "map[string]interface{} usage - can often be replaced with typed structs",
		Suggestions: []string{
			"Consider defining a specific struct type",
			"Use map[string]any for Go 1.18+",
			"Implement JSON tags for marshaling/unmarshaling",
		},
		Difficulty: 3,
	},
	{
		Name:        "slice_interface",
		Regex:       regexp.MustCompile(`\[\]interface\{\}`),
		RiskLevel:   "medium",
		Description: "[]interface{} usage - can be replaced with generic slices",
		Suggestions: []string{
			"Use generic slice types: []T",
			"Consider union types if elements have different types",
			"Use []any for Go 1.18+",
		},
		Difficulty: 2,
	},
	{
		Name:        "function_parameter",
		Regex:       regexp.MustCompile(`func\s+\w+[^{]*\binterface\{\}[^{]*\{`),
		RiskLevel:   "high",
		Description: "interface{} as function parameter - consider generic constraints",
		Suggestions: []string{
			"Use generic type parameters with constraints",
			"Define specific interface types",
			"Use type assertions with proper error handling",
		},
		Difficulty: 4,
	},
	{
		Name:        "function_return",
		Regex:       regexp.MustCompile(`func\s+\w+[^{]*\)\s+interface\{\}`),
		RiskLevel:   "high",
		Description: "interface{} as function return type - consider specific return types",
		Suggestions: []string{
			"Return specific types instead of interface{}",
			"Use union types for multiple possible return types",
			"Consider using generics for flexible return types",
		},
		Difficulty: 4,
	},
	{
		Name:        "struct_field",
		Regex:       regexp.MustCompile(`\w+\s+interface\{\}`),
		RiskLevel:   "medium",
		Description: "interface{} as struct field - can be made type-safe",
		Suggestions: []string{
			"Use specific types for struct fields",
			"Consider using generics for flexible struct types",
			"Use any for Go 1.18+ if truly needed",
		},
		Difficulty: 3,
	},
	{
		Name:        "json_unmarshal",
		Regex:       regexp.MustCompile(`json\.Unmarshal\([^,]+,\s*&?\w*interface\{\}`),
		RiskLevel:   "medium",
		Description: "JSON unmarshaling to interface{} - can use typed structs",
		Suggestions: []string{
			"Define struct types matching JSON structure",
			"Use json.RawMessage for deferred parsing",
			"Implement custom UnmarshalJSON methods",
		},
		Difficulty: 2,
	},
	{
		Name:        "type_assertion",
		Regex:       regexp.MustCompile(`\.\(interface\{\}\)`),
		RiskLevel:   "low",
		Description: "Type assertion to interface{} - usually safe but consider alternatives",
		Suggestions: []string{
			"Use specific type assertions when possible",
			"Consider type switches for multiple types",
			"Use any for Go 1.18+",
		},
		Difficulty: 1,
	},
	{
		Name:        "logger_any",
		Regex:       regexp.MustCompile(`Any\s*\(\s*"[^"]+"\s*,\s*[^)]+\s*\)`),
		RiskLevel:   "low",
		Description: "Logger Any() calls - can be replaced with type-safe alternatives",
		Suggestions: []string{
			"Use SafeString(), SafeInt64(), etc. for known types",
			"Implement type inference for automatic migration",
			"Keep Any() only for truly dynamic data",
		},
		Difficulty: 1,
	},
	{
		Name:        "empty_interface",
		Regex:       regexp.MustCompile(`\binterface\{\}`),
		RiskLevel:   "medium",
		Description: "Generic interface{} usage - context-dependent migration",
		Suggestions: []string{
			"Analyze the specific usage context",
			"Use any for Go 1.18+ if appropriate",
			"Consider more specific interface types",
		},
		Difficulty: 3,
	},
}

func main() {
	var config AnalysisConfig
	
	// Parse command line flags
	flag.StringVar(&config.Directory, "dir", ".", "Root directory to analyze")
	flag.BoolVar(&config.Recursive, "recursive", true, "Analyze subdirectories recursively")
	flag.StringVar(&config.OutputFile, "output", "interface_analysis.json", "Output file for analysis report")
	flag.StringVar(&config.OutputFormat, "format", "json", "Output format (json, text, csv)")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&config.ShowExamples, "examples", true, "Include code examples in output")
	flag.IntVar(&config.MaxExamples, "max-examples", 5, "Maximum examples per pattern")
	flag.Parse()
	
	// Default patterns
	config.IncludePatterns = []string{"*.go"}
	config.ExcludePatterns = []string{"vendor/*", ".git/*"}
	
	if config.Verbose {
		log.Printf("Starting analysis with config: %+v", config)
	}
	
	// Run analysis
	report, err := runAnalysis(config)
	if err != nil {
		log.Fatalf("Analysis failed: %v", err)
	}
	
	// Print summary
	printSummary(report)
	
	// Write detailed report
	if err := writeReport(report, config.OutputFile, config.OutputFormat); err != nil {
		log.Printf("Warning: Failed to write report: %v", err)
	}
	
	if config.Verbose {
		log.Printf("Analysis completed. Report written to %s", config.OutputFile)
	}
}

// runAnalysis executes the analysis process
func runAnalysis(config AnalysisConfig) (*AnalysisReport, error) {
	report := &AnalysisReport{
		Timestamp: time.Now().Format(time.RFC3339),
		Config:    config,
		Summary: AnalysisSummary{
			PatternBreakdown: make(map[string]int),
			RiskBreakdown:    make(map[string]int),
		},
	}
	
	// Find all Go files to analyze
	files, err := findGoFiles(config.Directory, config.Recursive, config.IncludePatterns, config.ExcludePatterns)
	if err != nil {
		return nil, fmt.Errorf("failed to find Go files: %w", err)
	}
	
	report.Summary.FilesAnalyzed = len(files)
	
	// Group files by package
	packageFiles := groupFilesByPackage(files)
	report.Summary.PackagesAnalyzed = len(packageFiles)
	
	// Analyze each package
	for pkgPath, pkgFiles := range packageFiles {
		if config.Verbose {
			log.Printf("Analyzing package: %s", pkgPath)
		}
		
		pkgAnalysis, err := analyzePackage(pkgPath, pkgFiles, config)
		if err != nil {
			log.Printf("Error analyzing package %s: %v", pkgPath, err)
			continue
		}
		
		report.Packages = append(report.Packages, pkgAnalysis)
		
		// Update summary statistics
		report.Summary.TotalUsages += pkgAnalysis.TotalUsages
		for pattern, count := range pkgAnalysis.PatternBreakdown {
			report.Summary.PatternBreakdown[pattern] += count
		}
		for risk, count := range pkgAnalysis.RiskBreakdown {
			report.Summary.RiskBreakdown[risk] += count
		}
	}
	
	// Generate pattern analysis
	report.Patterns = generatePatternAnalysis(report)
	
	// Generate file and package breakdowns
	report.Summary.FileBreakdown = generateFileBreakdown(report)
	report.Summary.PackageBreakdown = generatePackageBreakdown(report)
	
	// Generate recommendations
	report.Recommendations = generateRecommendations(report)
	
	return report, nil
}

// analyzePackage analyzes a single package
func analyzePackage(pkgPath string, files []string, config AnalysisConfig) (PackageAnalysis, error) {
	analysis := PackageAnalysis{
		Path:             pkgPath,
		Files:            files,
		PatternBreakdown: make(map[string]int),
		RiskBreakdown:    make(map[string]int),
	}
	
	var allUsages []InterfaceUsage
	
	// Analyze each file in the package
	for _, file := range files {
		if config.Verbose {
			log.Printf("  Analyzing file: %s", file)
		}
		
		usages, err := analyzeFile(file)
		if err != nil {
			log.Printf("Error analyzing file %s: %v", file, err)
			continue
		}
		
		// Set package name from first file
		if analysis.Package == "" && len(usages) > 0 {
			analysis.Package = usages[0].Package
		}
		
		allUsages = append(allUsages, usages...)
	}
	
	analysis.Usages = allUsages
	analysis.TotalUsages = len(allUsages)
	
	// Calculate breakdowns
	for _, usage := range allUsages {
		analysis.PatternBreakdown[usage.Pattern]++
		analysis.RiskBreakdown[usage.RiskLevel]++
	}
	
	return analysis, nil
}

// analyzeFile analyzes a single Go file for interface{} usage
func analyzeFile(filename string) ([]InterfaceUsage, error) {
	var usages []InterfaceUsage
	
	// Read the file
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	// Parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}
	
	packageName := node.Name.Name
	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")
	
	// Apply each pattern to find usages
	for _, pattern := range usagePatterns {
		matches := pattern.Regex.FindAllStringIndex(contentStr, -1)
		
		for _, match := range matches {
			start := match[0]
			end := match[1]
			
			// Convert byte offset to line/column
			position := fset.Position(fset.PositionFor(token.Pos(start+1), false))
			
			// Extract context (current line and surrounding lines)
			lineNum := position.Line - 1 // Convert to 0-based
			context := extractContext(lines, lineNum, 2)
			
			// Find the function containing this usage
			functionName := findContainingFunction(node, fset, token.Pos(start+1))
			
			usage := InterfaceUsage{
				File:        filename,
				Line:        position.Line,
				Column:      position.Column,
				Context:     context,
				Pattern:     pattern.Name,
				RiskLevel:   pattern.RiskLevel,
				Description: pattern.Description,
				Function:    functionName,
				Package:     packageName,
				Suggestions: pattern.Suggestions,
			}
			
			usages = append(usages, usage)
		}
	}
	
	return usages, nil
}

// extractContext extracts surrounding lines for context
func extractContext(lines []string, lineNum, contextLines int) string {
	start := max(0, lineNum-contextLines)
	end := min(len(lines), lineNum+contextLines+1)
	
	var context strings.Builder
	for i := start; i < end; i++ {
		if i == lineNum {
			context.WriteString(fmt.Sprintf(">>> %s\n", lines[i]))
		} else {
			context.WriteString(fmt.Sprintf("    %s\n", lines[i]))
		}
	}
	
	return context.String()
}

// findContainingFunction finds the function that contains a given position
func findContainingFunction(node *ast.File, fset *token.FileSet, pos token.Pos) string {
	var functionName string
	
	ast.Inspect(node, func(n ast.Node) bool {
		switch fn := n.(type) {
		case *ast.FuncDecl:
			if fn.Pos() <= pos && pos <= fn.End() {
				functionName = fn.Name.Name
				return false // Found it, stop searching
			}
		}
		return true
	})
	
	return functionName
}

// groupFilesByPackage groups files by their package directory
func groupFilesByPackage(files []string) map[string][]string {
	packageFiles := make(map[string][]string)
	
	for _, file := range files {
		dir := filepath.Dir(file)
		packageFiles[dir] = append(packageFiles[dir], file)
	}
	
	return packageFiles
}

// generatePatternAnalysis generates detailed analysis for each pattern
func generatePatternAnalysis(report *AnalysisReport) []PatternAnalysis {
	patternMap := make(map[string]*PatternAnalysis)
	
	// Initialize pattern analysis from definitions
	for _, pattern := range usagePatterns {
		patternMap[pattern.Name] = &PatternAnalysis{
			Pattern:             pattern.Name,
			Description:         pattern.Description,
			RiskLevel:           pattern.RiskLevel,
			MigrationSuggestions: pattern.Suggestions,
			MigrationDifficulty: pattern.Difficulty,
		}
	}
	
	// Collect examples and count occurrences
	for _, pkg := range report.Packages {
		for _, usage := range pkg.Usages {
			if analysis, exists := patternMap[usage.Pattern]; exists {
				analysis.Count++
				
				// Add examples (limited by MaxExamples)
				if len(analysis.Examples) < report.Config.MaxExamples {
					analysis.Examples = append(analysis.Examples, usage)
				}
			}
		}
	}
	
	// Convert map to slice and sort by count
	var patterns []PatternAnalysis
	for _, analysis := range patternMap {
		if analysis.Count > 0 {
			patterns = append(patterns, *analysis)
		}
	}
	
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Count > patterns[j].Count
	})
	
	return patterns
}

// generateFileBreakdown generates a breakdown of files with the most usages
func generateFileBreakdown(report *AnalysisReport) []FileUsageCount {
	fileCounts := make(map[string]int)
	
	for _, pkg := range report.Packages {
		for _, usage := range pkg.Usages {
			fileCounts[usage.File]++
		}
	}
	
	var breakdown []FileUsageCount
	for file, count := range fileCounts {
		breakdown = append(breakdown, FileUsageCount{File: file, Count: count})
	}
	
	sort.Slice(breakdown, func(i, j int) bool {
		return breakdown[i].Count > breakdown[j].Count
	})
	
	// Limit to top 10
	if len(breakdown) > 10 {
		breakdown = breakdown[:10]
	}
	
	return breakdown
}

// generatePackageBreakdown generates a breakdown of packages with the most usages
func generatePackageBreakdown(report *AnalysisReport) []PackageUsageCount {
	var breakdown []PackageUsageCount
	
	for _, pkg := range report.Packages {
		if pkg.TotalUsages > 0 {
			breakdown = append(breakdown, PackageUsageCount{
				Package: pkg.Package,
				Count:   pkg.TotalUsages,
			})
		}
	}
	
	sort.Slice(breakdown, func(i, j int) bool {
		return breakdown[i].Count > breakdown[j].Count
	})
	
	return breakdown
}

// generateRecommendations generates migration recommendations
func generateRecommendations(report *AnalysisReport) []string {
	var recommendations []string
	
	if report.Summary.TotalUsages == 0 {
		recommendations = append(recommendations, "✅ No interface{} usage found in the analyzed code.")
		return recommendations
	}
	
	// High-priority recommendations
	if report.Summary.RiskBreakdown["high"] > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("🚨 %d high-risk interface{} usages found. These require careful manual migration.", 
				report.Summary.RiskBreakdown["high"]))
	}
	
	// Pattern-specific recommendations
	for _, pattern := range report.Patterns {
		if pattern.Count > 0 {
			switch pattern.Pattern {
			case "logger_any":
				recommendations = append(recommendations,
					fmt.Sprintf("🔧 %d logger Any() calls found. These can be automatically migrated to type-safe alternatives.", pattern.Count))
			case "map_string_interface":
				recommendations = append(recommendations,
					fmt.Sprintf("📋 %d map[string]interface{} usages found. Consider defining typed structs for better type safety.", pattern.Count))
			case "function_parameter", "function_return":
				recommendations = append(recommendations,
					fmt.Sprintf("⚡ %d function interface{} usages found. Consider using generics for type-safe alternatives.", pattern.Count))
			}
		}
	}
	
	// Migration strategy recommendations
	if report.Summary.TotalUsages > 50 {
		recommendations = append(recommendations,
			"📈 Large number of interface{} usages detected. Consider a phased migration approach.",
			"🏗️  Start with low-risk patterns (logger calls, type assertions) before tackling high-risk patterns.")
	}
	
	// File-specific recommendations
	if len(report.Summary.FileBreakdown) > 0 {
		topFile := report.Summary.FileBreakdown[0]
		if topFile.Count > 10 {
			recommendations = append(recommendations,
				fmt.Sprintf("📁 File %s has the most interface{} usages (%d). Consider prioritizing this file for migration.", 
					filepath.Base(topFile.File), topFile.Count))
		}
	}
	
	// General recommendations
	recommendations = append(recommendations,
		"🧪 Run comprehensive tests after any migration to ensure functionality is preserved.",
		"📚 Consider adding type assertions and validation for interface{} usages that must remain.",
		"🎯 Use the migration tools to automatically fix low-risk patterns.",
	)
	
	return recommendations
}

// findGoFiles finds all Go files matching the given patterns
func findGoFiles(root string, recursive bool, includePatterns, excludePatterns []string) ([]string, error) {
	var files []string
	
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories if not recursive
		if d.IsDir() && path != root && !recursive {
			return filepath.SkipDir
		}
		
		// Skip non-files
		if d.IsDir() {
			return nil
		}
		
		// Check exclude patterns
		for _, pattern := range excludePatterns {
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				return nil
			}
			if matched, _ := filepath.Match(pattern, path); matched {
				return nil
			}
		}
		
		// Check include patterns
		for _, pattern := range includePatterns {
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				files = append(files, path)
				break
			}
		}
		
		return nil
	})
	
	return files, err
}

// printSummary prints a summary of the analysis results
func printSummary(report *AnalysisReport) {
	fmt.Printf("\n=== Interface{} Usage Analysis ===\n")
	fmt.Printf("Files analyzed: %d\n", report.Summary.FilesAnalyzed)
	fmt.Printf("Packages analyzed: %d\n", report.Summary.PackagesAnalyzed)
	fmt.Printf("Total interface{} usages: %d\n", report.Summary.TotalUsages)
	
	if len(report.Summary.RiskBreakdown) > 0 {
		fmt.Printf("\nRisk breakdown:\n")
		risks := []string{"high", "medium", "low"}
		for _, risk := range risks {
			if count, exists := report.Summary.RiskBreakdown[risk]; exists && count > 0 {
				fmt.Printf("  %s: %d\n", risk, count)
			}
		}
	}
	
	if len(report.Summary.PatternBreakdown) > 0 {
		fmt.Printf("\nTop patterns:\n")
		// Sort patterns by count
		type patternCount struct {
			pattern string
			count   int
		}
		var patterns []patternCount
		for pattern, count := range report.Summary.PatternBreakdown {
			patterns = append(patterns, patternCount{pattern, count})
		}
		sort.Slice(patterns, func(i, j int) bool {
			return patterns[i].count > patterns[j].count
		})
		
		for i, pc := range patterns {
			if i >= 5 { // Show top 5
				break
			}
			fmt.Printf("  %s: %d\n", pc.pattern, pc.count)
		}
	}
	
	if len(report.Recommendations) > 0 {
		fmt.Printf("\nKey recommendations:\n")
		for i, rec := range report.Recommendations {
			if i >= 3 { // Show top 3
				break
			}
			fmt.Printf("  %s\n", rec)
		}
	}
}

// writeReport writes the analysis report in the specified format
func writeReport(report *AnalysisReport, filename, format string) error {
	switch format {
	case "json":
		return writeJSONReport(report, filename)
	case "text":
		return writeTextReport(report, filename)
	case "csv":
		return writeCSVReport(report, filename)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// writeJSONReport writes the report in JSON format
func writeJSONReport(report *AnalysisReport, filename string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	
	return os.WriteFile(filename, data, 0644)
}

// writeTextReport writes the report in human-readable text format
func writeTextReport(report *AnalysisReport, filename string) error {
	var content strings.Builder
	
	content.WriteString("# Interface{} Usage Analysis Report\n\n")
	content.WriteString(fmt.Sprintf("**Generated:** %s\n", report.Timestamp))
	content.WriteString(fmt.Sprintf("**Directory:** %s\n\n", report.Config.Directory))
	
	// Summary
	content.WriteString("## Summary\n\n")
	content.WriteString(fmt.Sprintf("- Files analyzed: %d\n", report.Summary.FilesAnalyzed))
	content.WriteString(fmt.Sprintf("- Packages analyzed: %d\n", report.Summary.PackagesAnalyzed))
	content.WriteString(fmt.Sprintf("- Total interface{} usages: %d\n\n", report.Summary.TotalUsages))
	
	// Pattern breakdown
	if len(report.Patterns) > 0 {
		content.WriteString("## Pattern Analysis\n\n")
		for _, pattern := range report.Patterns {
			content.WriteString(fmt.Sprintf("### %s (%d occurrences)\n\n", pattern.Pattern, pattern.Count))
			content.WriteString(fmt.Sprintf("**Risk Level:** %s\n", pattern.RiskLevel))
			content.WriteString(fmt.Sprintf("**Description:** %s\n\n", pattern.Description))
			
			if len(pattern.MigrationSuggestions) > 0 {
				content.WriteString("**Migration Suggestions:**\n")
				for _, suggestion := range pattern.MigrationSuggestions {
					content.WriteString(fmt.Sprintf("- %s\n", suggestion))
				}
				content.WriteString("\n")
			}
		}
	}
	
	// Recommendations
	if len(report.Recommendations) > 0 {
		content.WriteString("## Recommendations\n\n")
		for _, rec := range report.Recommendations {
			content.WriteString(fmt.Sprintf("- %s\n", rec))
		}
		content.WriteString("\n")
	}
	
	return os.WriteFile(filename, []byte(content.String()), 0644)
}

// writeCSVReport writes the report in CSV format
func writeCSVReport(report *AnalysisReport, filename string) error {
	var content strings.Builder
	
	// CSV header
	content.WriteString("File,Package,Line,Column,Pattern,RiskLevel,Function,Description\n")
	
	// Data rows
	for _, pkg := range report.Packages {
		for _, usage := range pkg.Usages {
			content.WriteString(fmt.Sprintf("\"%s\",\"%s\",%d,%d,\"%s\",\"%s\",\"%s\",\"%s\"\n",
				usage.File, usage.Package, usage.Line, usage.Column,
				usage.Pattern, usage.RiskLevel, usage.Function, usage.Description))
		}
	}
	
	return os.WriteFile(filename, []byte(content.String()), 0644)
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}