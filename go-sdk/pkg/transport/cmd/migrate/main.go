package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ag-ui/go-sdk/pkg/transport"
)

func main() {
	var (
		sourceDir    = flag.String("source", ".", "Source directory to migrate")
		outputDir    = flag.String("output", "", "Output directory (empty means modify in place)")
		dryRun       = flag.Bool("dry-run", false, "Analyze without modifying files")
		backup       = flag.Bool("backup", true, "Create backup files")
		packages     = flag.String("packages", "", "Comma-separated list of packages to target")
		deadline     = flag.String("deadline", "2024-12-31", "Deprecation deadline (YYYY-MM-DD)")
		report       = flag.String("report", "", "Output migration report to file")
		deprecate    = flag.Bool("deprecate", false, "Add deprecation annotations")
		docs         = flag.Bool("docs", false, "Generate documentation")
		docsFormat   = flag.String("docs-format", "markdown", "Documentation format (markdown, html, json)")
		docsOutput   = flag.String("docs-output", "./docs", "Documentation output directory")
		help         = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	// Parse deadline
	deadlineTime, err := time.Parse("2006-01-02", *deadline)
	if err != nil {
		fmt.Printf("Error: Invalid deadline format: %v\n", err)
		os.Exit(1)
	}

	// Parse target packages
	var targetPackages []string
	if *packages != "" {
		// Split by comma if provided
		// targetPackages = strings.Split(*packages, ",")
	}

	// Run migration
	if !*docs {
		if err := runMigration(*sourceDir, *outputDir, *dryRun, *backup, targetPackages, deadlineTime, *report, *deprecate); err != nil {
			fmt.Printf("Migration failed: %v\n", err)
			os.Exit(1)
		}
	}

	// Generate documentation
	if *docs {
		if err := generateDocumentation(*sourceDir, *docsOutput, *docsFormat); err != nil {
			fmt.Printf("Documentation generation failed: %v\n", err)
			os.Exit(1)
		}
	}
}

func runMigration(sourceDir, outputDir string, dryRun, backup bool, targetPackages []string, deadline time.Time, reportFile string, addDeprecations bool) error {
	fmt.Printf("Starting transport migration...\n")
	fmt.Printf("Source: %s\n", sourceDir)
	if outputDir != "" {
		fmt.Printf("Output: %s\n", outputDir)
	}
	fmt.Printf("Mode: %s\n", map[bool]string{true: "dry-run", false: "modify"}[dryRun])
	fmt.Printf("Deadline: %s\n", deadline.Format("2006-01-02"))

	// Create migration configuration
	config := &transport.MigrationConfig{
		SourceDir:           sourceDir,
		OutputDir:           outputDir,
		DryRun:              dryRun,
		BackupOriginal:      backup,
		TargetPackages:      targetPackages,
		DeprecationDeadline: deadline,
	}

	// Create migrator
	migrator := transport.NewTransportMigrator(config)

	// Run migration
	report, err := migrator.Migrate()
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Add deprecation annotations if requested
	if addDeprecations {
		fmt.Println("Adding deprecation annotations...")
		if err := migrator.GenerateDeprecationAnnotations(); err != nil {
			fmt.Printf("Warning: Failed to add deprecation annotations: %v\n", err)
		}
	}

	// Print results
	printMigrationReport(report)

	// Save report to file if requested
	if reportFile != "" {
		if err := saveMigrationReport(report, reportFile); err != nil {
			fmt.Printf("Warning: Failed to save report: %v\n", err)
		} else {
			fmt.Printf("Report saved to: %s\n", reportFile)
		}
	}

	return nil
}

func generateDocumentation(sourceDir, outputDir, format string) error {
	fmt.Printf("Generating documentation...\n")
	fmt.Printf("Source: %s\n", sourceDir)
	fmt.Printf("Output: %s\n", outputDir)
	fmt.Printf("Format: %s\n", format)

	// Create documentation configuration
	config := &transport.DocConfig{
		OutputDir:         outputDir,
		Format:            format,
		IncludeExamples:   true,
		IncludeDeprecated: true,
		GenerateIndex:     true,
	}

	// Create generator
	generator := transport.NewDocumentationGenerator(config)

	// Generate documentation
	apiDoc, err := generator.GenerateDocumentation(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to generate documentation: %w", err)
	}

	// Write documentation
	if err := generator.WriteDocumentation(apiDoc); err != nil {
		return fmt.Errorf("failed to write documentation: %w", err)
	}

	// Print summary
	fmt.Printf("Documentation generated successfully!\n")
	fmt.Printf("- Package: %s\n", apiDoc.PackageName)
	fmt.Printf("- Interfaces: %d\n", len(apiDoc.Interfaces))
	fmt.Printf("- Types: %d\n", len(apiDoc.Types))
	fmt.Printf("- Functions: %d\n", len(apiDoc.Functions))
	fmt.Printf("- Examples: %d\n", len(apiDoc.Examples))
	fmt.Printf("- Deprecations: %d\n", len(apiDoc.Deprecations))

	return nil
}

func printMigrationReport(report *transport.MigrationReport) {
	fmt.Printf("\n=== Migration Report ===\n")
	fmt.Printf("Files processed: %d\n", report.FilesProcessed)
	fmt.Printf("Files modified: %d\n", report.FilesModified)

	if len(report.TransformationsApplied) > 0 {
		fmt.Printf("\nTransformations applied:\n")
		for rule, count := range report.TransformationsApplied {
			fmt.Printf("  %s: %d\n", rule, count)
		}
	}

	if len(report.DeprecationWarnings) > 0 {
		fmt.Printf("\nDeprecation warnings (%d):\n", len(report.DeprecationWarnings))
		for i, warning := range report.DeprecationWarnings {
			if i >= 10 { // Limit output
				fmt.Printf("  ... and %d more warnings\n", len(report.DeprecationWarnings)-10)
				break
			}
			fmt.Printf("  %s:%d:%d - %s: %s\n",
				filepath.Base(warning.File), warning.Line, warning.Column,
				warning.Method, warning.Message)
		}
	}

	if len(report.Warnings) > 0 {
		fmt.Printf("\nWarnings (%d):\n", len(report.Warnings))
		for i, warning := range report.Warnings {
			if i >= 5 { // Limit output
				fmt.Printf("  ... and %d more warnings\n", len(report.Warnings)-5)
				break
			}
			fmt.Printf("  %s\n", warning)
		}
	}

	if len(report.Errors) > 0 {
		fmt.Printf("\nErrors (%d):\n", len(report.Errors))
		for _, err := range report.Errors {
			fmt.Printf("  %v\n", err)
		}
	}

	fmt.Printf("\n=== End Report ===\n")
}

func saveMigrationReport(report *transport.MigrationReport, filename string) error {
	// Create output directory if needed
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Convert to JSON
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(filename, data, 0644)
}

func showHelp() {
	fmt.Printf(`Transport Migration Tool

Usage:
  migrate [flags]

Migration Flags:
  -source string      Source directory to migrate (default ".")
  -output string      Output directory (empty means modify in place)
  -dry-run           Analyze without modifying files
  -backup            Create backup files (default true)
  -packages string   Comma-separated list of packages to target
  -deadline string   Deprecation deadline in YYYY-MM-DD format (default "2024-12-31")
  -report string     Output migration report to file
  -deprecate         Add deprecation annotations

Documentation Flags:
  -docs              Generate documentation instead of migration
  -docs-format string  Documentation format: markdown, html, json (default "markdown")
  -docs-output string  Documentation output directory (default "./docs")

General Flags:
  -help              Show this help message

Examples:
  # Analyze current code without changes
  migrate -dry-run -source ./pkg/transport

  # Migrate with backup
  migrate -source ./pkg/transport -backup

  # Add deprecation annotations
  migrate -source ./pkg/transport -deprecate

  # Generate documentation
  migrate -docs -source ./pkg/transport -docs-output ./docs

  # Full migration with report
  migrate -source ./pkg/transport -report ./migration-report.json

Migration Process:
  1. Use -dry-run first to see what will change
  2. Review the deprecation warnings
  3. Run migration with -backup for safety
  4. Test your code thoroughly
  5. Use -deprecate to add deprecation comments
  6. Generate documentation with -docs

For more information, see MIGRATION_GUIDE.md
`)
}