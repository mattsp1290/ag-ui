// Package tools provides migration utilities for converting interface{} usage to type-safe alternatives
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// MigrationConfig holds configuration for the migration process
type MigrationConfig struct {
	// DryRun indicates whether to only analyze without making changes
	DryRun bool
	
	// Directory is the root directory to process
	Directory string
	
	// Recursive indicates whether to process subdirectories
	Recursive bool
	
	// OutputFile is the file to write the migration report
	OutputFile string
	
	// IncludePatterns are patterns for files to include
	IncludePatterns []string
	
	// ExcludePatterns are patterns for files to exclude
	ExcludePatterns []string
	
	// LoggerMigration enables migration of Any() logger calls to SafeX() calls
	LoggerMigration bool
	
	// MapMigration enables migration of map[string]interface{} to typed structs
	MapMigration bool
	
	// ParamMigration enables migration of interface{} parameters to type constraints
	ParamMigration bool
	
	// Verbose enables detailed logging
	Verbose bool
}

// MigrationPattern represents a pattern for migration
type MigrationPattern struct {
	// Name of the pattern
	Name string
	
	// Pattern to match (regex or AST-based)
	Pattern string
	
	// Replacement template
	Replacement string
	
	// RiskLevel indicates the migration risk (high/medium/low)
	RiskLevel string
	
	// Description explains what this pattern does
	Description string
	
	// Validator is a function to validate the migration
	Validator func(original, replacement string) bool
}

// MigrationResult represents the result of a migration operation
type MigrationResult struct {
	// File is the path to the processed file
	File string
	
	// Patterns is a list of patterns that were applied
	Patterns []string
	
	// Changes is a list of changes made
	Changes []MigrationChange
	
	// Errors is a list of errors encountered
	Errors []string
	
	// RiskLevel is the overall risk level of the file
	RiskLevel string
}

// MigrationChange represents a single change made during migration
type MigrationChange struct {
	// Line number where the change was made
	Line int
	
	// Column number where the change was made
	Column int
	
	// Original code
	Original string
	
	// Replacement code
	Replacement string
	
	// Pattern used for the change
	Pattern string
	
	// RiskLevel of this specific change
	RiskLevel string
	
	// Description of what was changed
	Description string
}

// MigrationReport contains the overall migration report
type MigrationReport struct {
	// Config used for the migration
	Config MigrationConfig
	
	// Results for each file processed
	Results []MigrationResult
	
	// Summary statistics
	Summary MigrationSummary
	
	// Recommendations for next steps
	Recommendations []string
}

// MigrationSummary contains summary statistics
type MigrationSummary struct {
	// FilesProcessed is the total number of files processed
	FilesProcessed int
	
	// FilesChanged is the number of files that had changes
	FilesChanged int
	
	// TotalChanges is the total number of changes made
	TotalChanges int
	
	// RiskBreakdown shows the number of changes by risk level
	RiskBreakdown map[string]int
	
	// PatternBreakdown shows the number of times each pattern was used
	PatternBreakdown map[string]int
}

// Common migration patterns
var migrationPatterns = []MigrationPattern{
	{
		Name:        "any_logger_to_safe",
		Pattern:     `Any\s*\(\s*"([^"]+)"\s*,\s*([^)]+)\s*\)`,
		Replacement: `Safe{{TYPE}}("$1", $2)`,
		RiskLevel:   "medium",
		Description: "Migrate Any() logger calls to type-safe SafeX() calls",
	},
	{
		Name:        "map_interface_to_typed",
		Pattern:     `map\[string\]interface\{\}`,
		Replacement: `map[string]{{TYPED_VALUE}}`,
		RiskLevel:   "high",
		Description: "Migrate map[string]interface{} to typed alternatives",
	},
	{
		Name:        "interface_param_to_constraint",
		Pattern:     `func\s+(\w+)\s*\([^)]*interface\{\}[^)]*\)`,
		Replacement: `func $1[T {{CONSTRAINT}}](...T...)`,
		RiskLevel:   "high",
		Description: "Migrate interface{} parameters to generic type constraints",
	},
	{
		Name:        "interface_slice_to_typed",
		Pattern:     `\[\]interface\{\}`,
		Replacement: `[]{{TYPED_VALUE}}`,
		RiskLevel:   "medium",
		Description: "Migrate []interface{} to typed slices",
	},
	{
		Name:        "json_unmarshal_to_typed",
		Pattern:     `json\.Unmarshal\([^,]+,\s*&\w+\s*interface\{\}\)`,
		Replacement: `json.Unmarshal($1, &{{TYPED_STRUCT}})`,
		RiskLevel:   "medium",
		Description: "Migrate JSON unmarshaling to typed structs",
	},
}

func main() {
	var config MigrationConfig
	
	// Parse command line flags
	flag.BoolVar(&config.DryRun, "dry-run", true, "Only analyze without making changes")
	flag.StringVar(&config.Directory, "dir", ".", "Root directory to process")
	flag.BoolVar(&config.Recursive, "recursive", true, "Process subdirectories recursively")
	flag.StringVar(&config.OutputFile, "output", "migration_report.json", "Output file for migration report")
	flag.BoolVar(&config.LoggerMigration, "logger", true, "Enable logger migration")
	flag.BoolVar(&config.MapMigration, "maps", true, "Enable map migration")
	flag.BoolVar(&config.ParamMigration, "params", false, "Enable parameter migration (high risk)")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")
	flag.Parse()
	
	// Default include patterns
	config.IncludePatterns = []string{"*.go"}
	config.ExcludePatterns = []string{"*_test.go", "vendor/*", ".git/*"}
	
	if config.Verbose {
		log.Printf("Starting migration with config: %+v", config)
	}
	
	// Run migration
	report, err := runMigration(config)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	
	// Print summary
	printSummary(report)
	
	// Write detailed report
	if err := writeReport(report, config.OutputFile); err != nil {
		log.Printf("Warning: Failed to write report: %v", err)
	}
	
	if config.Verbose {
		log.Printf("Migration completed. Report written to %s", config.OutputFile)
	}
}

// runMigration executes the migration process
func runMigration(config MigrationConfig) (*MigrationReport, error) {
	report := &MigrationReport{
		Config: config,
		Summary: MigrationSummary{
			RiskBreakdown:    make(map[string]int),
			PatternBreakdown: make(map[string]int),
		},
	}
	
	// Find all Go files to process
	files, err := findGoFiles(config.Directory, config.Recursive, config.IncludePatterns, config.ExcludePatterns)
	if err != nil {
		return nil, fmt.Errorf("failed to find Go files: %w", err)
	}
	
	report.Summary.FilesProcessed = len(files)
	
	// Process each file
	for _, file := range files {
		if config.Verbose {
			log.Printf("Processing file: %s", file)
		}
		
		result, err := processFile(file, config)
		if err != nil {
			log.Printf("Error processing %s: %v", file, err)
			result = MigrationResult{
				File:   file,
				Errors: []string{err.Error()},
			}
		}
		
		report.Results = append(report.Results, result)
		
		if len(result.Changes) > 0 {
			report.Summary.FilesChanged++
			report.Summary.TotalChanges += len(result.Changes)
			
			// Update breakdown statistics
			for _, change := range result.Changes {
				report.Summary.RiskBreakdown[change.RiskLevel]++
				report.Summary.PatternBreakdown[change.Pattern]++
			}
		}
	}
	
	// Generate recommendations
	report.Recommendations = generateRecommendations(report)
	
	return report, nil
}

// processFile processes a single Go file
func processFile(filename string, config MigrationConfig) (MigrationResult, error) {
	result := MigrationResult{
		File: filename,
	}
	
	// Read the file
	content, err := os.ReadFile(filename)
	if err != nil {
		return result, fmt.Errorf("failed to read file: %w", err)
	}
	
	// Parse the file
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if err != nil {
		return result, fmt.Errorf("failed to parse file: %w", err)
	}
	
	// Analyze and migrate
	modified := string(content)
	
	// Apply logger migrations
	if config.LoggerMigration {
		loggerChanges, newContent := migrateLoggerCalls(modified, fset, node)
		result.Changes = append(result.Changes, loggerChanges...)
		modified = newContent
	}
	
	// Apply map migrations
	if config.MapMigration {
		mapChanges, newContent := migrateMapTypes(modified, fset, node)
		result.Changes = append(result.Changes, mapChanges...)
		modified = newContent
	}
	
	// Apply parameter migrations (high risk)
	if config.ParamMigration {
		paramChanges, newContent := migrateParameters(modified, fset, node)
		result.Changes = append(result.Changes, paramChanges...)
		modified = newContent
	}
	
	// Determine overall risk level
	result.RiskLevel = calculateRiskLevel(result.Changes)
	
	// Write changes if not dry run
	if !config.DryRun && len(result.Changes) > 0 {
		if err := writeModifiedFile(filename, modified); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to write file: %v", err))
		}
	}
	
	return result, nil
}

// migrateLoggerCalls migrates Any() logger calls to type-safe alternatives
func migrateLoggerCalls(content string, fset *token.FileSet, node *ast.File) ([]MigrationChange, string) {
	var changes []MigrationChange
	modified := content
	
	// Pattern for Any() calls
	anyPattern := regexp.MustCompile(`Any\s*\(\s*"([^"]+)"\s*,\s*([^)]+)\s*\)`)
	
	matches := anyPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			key := match[1]
			value := match[2]
			
			// Try to infer type from value
			safeFunc := inferSafeFunction(value)
			if safeFunc != "" {
				replacement := fmt.Sprintf(`%s("%s", %s)`, safeFunc, key, value)
				
				change := MigrationChange{
					Original:    match[0],
					Replacement: replacement,
					Pattern:     "any_logger_to_safe",
					RiskLevel:   "medium",
					Description: fmt.Sprintf("Migrate Any() to %s() for type safety", safeFunc),
				}
				
				changes = append(changes, change)
				modified = strings.Replace(modified, match[0], replacement, 1)
			}
		}
	}
	
	return changes, modified
}

// migrateMapTypes migrates map[string]interface{} to typed alternatives
func migrateMapTypes(content string, fset *token.FileSet, node *ast.File) ([]MigrationChange, string) {
	var changes []MigrationChange
	modified := content
	
	// Pattern for map[string]interface{}
	mapPattern := regexp.MustCompile(`map\[string\]interface\{\}`)
	
	matches := mapPattern.FindAllString(content, -1)
	for _, match := range matches {
		// For now, suggest using map[string]any (Go 1.18+) as a safer alternative
		replacement := "map[string]any"
		
		change := MigrationChange{
			Original:    match,
			Replacement: replacement,
			Pattern:     "map_interface_to_typed",
			RiskLevel:   "low", // any is still type-safe compared to interface{}
			Description: "Migrate map[string]interface{} to map[string]any for better type safety",
		}
		
		changes = append(changes, change)
		modified = strings.Replace(modified, match, replacement, 1)
	}
	
	return changes, modified
}

// migrateParameters migrates interface{} parameters to generic constraints
func migrateParameters(content string, fset *token.FileSet, node *ast.File) ([]MigrationChange, string) {
	var changes []MigrationChange
	// This is a complex migration that requires careful AST analysis
	// For now, we'll just identify interface{} parameters and suggest manual review
	
	paramPattern := regexp.MustCompile(`func\s+\w+[^{]*interface\{\}[^{]*\{`)
	matches := paramPattern.FindAllString(content, -1)
	
	for _, match := range matches {
		change := MigrationChange{
			Original:    match,
			Replacement: "// TODO: Consider using generic type constraints instead of interface{}",
			Pattern:     "interface_param_to_constraint",
			RiskLevel:   "high",
			Description: "Manual review needed: function parameter uses interface{}",
		}
		changes = append(changes, change)
	}
	
	return changes, content // Don't modify for high-risk changes
}

// inferSafeFunction infers the appropriate SafeX function based on the value
func inferSafeFunction(value string) string {
	value = strings.TrimSpace(value)
	
	// Simple type inference based on common patterns
	if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return "SafeString"
	}
	if strings.Contains(value, "time.") || strings.Contains(value, "Duration") {
		if strings.Contains(value, "Duration") {
			return "SafeDuration"
		}
		return "SafeTime"
	}
	if value == "true" || value == "false" {
		return "SafeBool"
	}
	if strings.Contains(value, ".") && !strings.Contains(value, "(") {
		return "SafeFloat64"
	}
	if regexp.MustCompile(`^\d+$`).MatchString(value) {
		return "SafeInt64"
	}
	if strings.Contains(value, "err") || strings.Contains(value, "error") {
		return "SafeError"
	}
	
	return "" // Unable to infer, manual review needed
}

// calculateRiskLevel determines the overall risk level for a set of changes
func calculateRiskLevel(changes []MigrationChange) string {
	if len(changes) == 0 {
		return "none"
	}
	
	hasHigh := false
	hasMedium := false
	
	for _, change := range changes {
		switch change.RiskLevel {
		case "high":
			hasHigh = true
		case "medium":
			hasMedium = true
		}
	}
	
	if hasHigh {
		return "high"
	}
	if hasMedium {
		return "medium"
	}
	return "low"
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

// writeModifiedFile writes the modified content to a file
func writeModifiedFile(filename, content string) error {
	// Format the Go code
	formatted, err := format.Source([]byte(content))
	if err != nil {
		// If formatting fails, write the original content
		log.Printf("Warning: Failed to format %s: %v", filename, err)
		formatted = []byte(content)
	}
	
	return os.WriteFile(filename, formatted, 0644)
}

// generateRecommendations generates recommendations based on the migration results
func generateRecommendations(report *MigrationReport) []string {
	var recommendations []string
	
	if report.Summary.TotalChanges == 0 {
		recommendations = append(recommendations, "No interface{} usage patterns found that can be automatically migrated.")
		return recommendations
	}
	
	// High-risk changes
	if report.Summary.RiskBreakdown["high"] > 0 {
		recommendations = append(recommendations, 
			fmt.Sprintf("⚠️  %d high-risk changes detected. Manual review required before applying.", 
				report.Summary.RiskBreakdown["high"]))
	}
	
	// Medium-risk changes
	if report.Summary.RiskBreakdown["medium"] > 0 {
		recommendations = append(recommendations, 
			fmt.Sprintf("🔍 %d medium-risk changes detected. Review and test thoroughly.", 
				report.Summary.RiskBreakdown["medium"]))
	}
	
	// Logger migrations
	if report.Summary.PatternBreakdown["any_logger_to_safe"] > 0 {
		recommendations = append(recommendations, 
			"✅ Logger migrations look safe. Consider running with --dry-run=false to apply.")
	}
	
	// Map migrations
	if report.Summary.PatternBreakdown["map_interface_to_typed"] > 0 {
		recommendations = append(recommendations, 
			"📋 Map migrations found. Consider defining specific struct types instead of map[string]any.")
	}
	
	// General recommendations
	recommendations = append(recommendations, 
		"🔧 Run tests after applying changes to ensure functionality is preserved.",
		"📚 Consider adding type assertions and validation where appropriate.",
		"🏗️  For complex types, consider using code generation tools.",
	)
	
	return recommendations
}

// printSummary prints a summary of the migration results
func printSummary(report *MigrationReport) {
	fmt.Printf("\n=== Migration Summary ===\n")
	fmt.Printf("Files processed: %d\n", report.Summary.FilesProcessed)
	fmt.Printf("Files with changes: %d\n", report.Summary.FilesChanged)
	fmt.Printf("Total changes: %d\n", report.Summary.TotalChanges)
	
	if len(report.Summary.RiskBreakdown) > 0 {
		fmt.Printf("\nRisk breakdown:\n")
		for risk, count := range report.Summary.RiskBreakdown {
			fmt.Printf("  %s: %d\n", risk, count)
		}
	}
	
	if len(report.Summary.PatternBreakdown) > 0 {
		fmt.Printf("\nPattern breakdown:\n")
		for pattern, count := range report.Summary.PatternBreakdown {
			fmt.Printf("  %s: %d\n", pattern, count)
		}
	}
	
	if len(report.Recommendations) > 0 {
		fmt.Printf("\nRecommendations:\n")
		for _, rec := range report.Recommendations {
			fmt.Printf("  %s\n", rec)
		}
	}
}

// writeReport writes the detailed migration report to a file
func writeReport(report *MigrationReport, filename string) error {
	var buf bytes.Buffer
	
	// Write detailed report in a readable format
	buf.WriteString("# Interface{} Migration Report\n\n")
	buf.WriteString(fmt.Sprintf("**Generated:** %s\n", ""))
	buf.WriteString(fmt.Sprintf("**Directory:** %s\n", report.Config.Directory))
	buf.WriteString(fmt.Sprintf("**Dry Run:** %t\n\n", report.Config.DryRun))
	
	// Summary
	buf.WriteString("## Summary\n\n")
	buf.WriteString(fmt.Sprintf("- Files processed: %d\n", report.Summary.FilesProcessed))
	buf.WriteString(fmt.Sprintf("- Files with changes: %d\n", report.Summary.FilesChanged))
	buf.WriteString(fmt.Sprintf("- Total changes: %d\n\n", report.Summary.TotalChanges))
	
	// Risk breakdown
	if len(report.Summary.RiskBreakdown) > 0 {
		buf.WriteString("### Risk Breakdown\n\n")
		risks := make([]string, 0, len(report.Summary.RiskBreakdown))
		for risk := range report.Summary.RiskBreakdown {
			risks = append(risks, risk)
		}
		sort.Strings(risks)
		for _, risk := range risks {
			buf.WriteString(fmt.Sprintf("- %s: %d\n", risk, report.Summary.RiskBreakdown[risk]))
		}
		buf.WriteString("\n")
	}
	
	// Detailed results
	if len(report.Results) > 0 {
		buf.WriteString("## Detailed Results\n\n")
		for _, result := range report.Results {
			if len(result.Changes) > 0 || len(result.Errors) > 0 {
				buf.WriteString(fmt.Sprintf("### %s\n\n", result.File))
				
				if len(result.Errors) > 0 {
					buf.WriteString("**Errors:**\n")
					for _, err := range result.Errors {
						buf.WriteString(fmt.Sprintf("- %s\n", err))
					}
					buf.WriteString("\n")
				}
				
				if len(result.Changes) > 0 {
					buf.WriteString("**Changes:**\n")
					for _, change := range result.Changes {
						buf.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", change.Pattern, change.RiskLevel, change.Description))
						buf.WriteString(fmt.Sprintf("  - Original: `%s`\n", change.Original))
						buf.WriteString(fmt.Sprintf("  - Replacement: `%s`\n", change.Replacement))
					}
					buf.WriteString("\n")
				}
			}
		}
	}
	
	// Recommendations
	if len(report.Recommendations) > 0 {
		buf.WriteString("## Recommendations\n\n")
		for _, rec := range report.Recommendations {
			buf.WriteString(fmt.Sprintf("- %s\n", rec))
		}
	}
	
	return os.WriteFile(filename, buf.Bytes(), 0644)
}