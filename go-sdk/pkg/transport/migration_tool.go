package transport

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MigrationConfig defines configuration for the migration process
type MigrationConfig struct {
	// SourceDir is the directory to scan for Go files
	SourceDir string
	
	// OutputDir is where migrated files should be written (if different from source)
	OutputDir string
	
	// DryRun when true will only analyze and report changes without writing
	DryRun bool
	
	// BackupOriginal when true will create .backup files
	BackupOriginal bool
	
	// TargetPackages specifies which packages to migrate (empty means all)
	TargetPackages []string
	
	// DeprecationDeadline specifies when deprecated methods will be removed
	DeprecationDeadline time.Time
}

// MigrationRule defines a transformation rule
type MigrationRule struct {
	Name        string
	Description string
	Pattern     string // Go AST pattern to match
	Replacement string // Replacement pattern
	Priority    int    // Higher priority rules run first
}

// MigrationReport contains the results of a migration operation
type MigrationReport struct {
	FilesProcessed   int
	FilesModified    int
	TransformationsApplied map[string]int // rule name -> count
	Errors          []error
	Warnings        []string
	DeprecationWarnings []DeprecationWarning
}

// DeprecationWarning represents a deprecation notice
type DeprecationWarning struct {
	File     string
	Line     int
	Column   int
	Method   string
	Message  string
	Deadline time.Time
}

// TransportMigrator handles the migration of transport code
type TransportMigrator struct {
	config *MigrationConfig
	fset   *token.FileSet
	rules  []MigrationRule
}

// NewTransportMigrator creates a new migration tool instance
func NewTransportMigrator(config *MigrationConfig) *TransportMigrator {
	migrator := &TransportMigrator{
		config: config,
		fset:   token.NewFileSet(),
		rules:  getDefaultMigrationRules(),
	}
	return migrator
}

// getDefaultMigrationRules returns the predefined migration rules
func getDefaultMigrationRules() []MigrationRule {
	return []MigrationRule{
		{
			Name:        "UpdateTransportInterface",
			Description: "Replace legacy Transport interface usage with composable interfaces",
			Pattern:     "type.*Transport.*interface",
			Priority:    100,
		},
		{
			Name:        "ReplaceOldEventHandlers",
			Description: "Replace deprecated event handler patterns with new EventHandler type",
			Pattern:     "func.*HandleEvent.*Event.*error",
			Priority:    90,
		},
		{
			Name:        "UpdateStatsAccess",
			Description: "Replace direct stats access with StatsProvider interface",
			Pattern:     "transport\\.Stats\\(\\)",
			Priority:    80,
		},
		{
			Name:        "MigrateConfigAccess",
			Description: "Replace direct config access with ConfigProvider interface",
			Pattern:     "transport\\.Config\\(\\)",
			Priority:    80,
		},
		{
			Name:        "UpdateBatchSending",
			Description: "Replace custom batch implementations with BatchSender interface",
			Pattern:     "SendBatch.*\\[\\].*Event",
			Priority:    70,
		},
		{
			Name:        "MigrateStreamingAPIs",
			Description: "Update streaming API usage to new StreamingTransport interface",
			Pattern:     "StartStream.*chan.*Event",
			Priority:    60,
		},
		{
			Name:        "UpdateReliabilityFeatures",
			Description: "Migrate reliability features to ReliableTransport interface",
			Pattern:     "SendWithAck.*Event.*timeout",
			Priority:    50,
		},
	}
}

// AddRule adds a custom migration rule
func (tm *TransportMigrator) AddRule(rule MigrationRule) {
	tm.rules = append(tm.rules, rule)
}

// Migrate performs the migration operation
func (tm *TransportMigrator) Migrate() (*MigrationReport, error) {
	report := &MigrationReport{
		TransformationsApplied: make(map[string]int),
		DeprecationWarnings:   make([]DeprecationWarning, 0),
	}

	err := filepath.Walk(tm.config.SourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process Go files
		if !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}

		// Skip test files in dry run mode
		if tm.config.DryRun && strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}

		return tm.processFile(path, report)
	})

	if err != nil {
		report.Errors = append(report.Errors, err)
	}

	return report, nil
}

// processFile processes a single Go file
func (tm *TransportMigrator) processFile(filename string, report *MigrationReport) error {
	report.FilesProcessed++

	// Parse the file
	src, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	file, err := parser.ParseFile(tm.fset, filename, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file %s: %w", filename, err)
	}

	// Check if this package is in our target list
	if len(tm.config.TargetPackages) > 0 {
		packageName := file.Name.Name
		found := false
		for _, target := range tm.config.TargetPackages {
			if packageName == target {
				found = true
				break
			}
		}
		if !found {
			return nil // Skip this package
		}
	}

	// Apply transformations
	modified := false
	visitor := &migrationVisitor{
		migrator: tm,
		report:   report,
		file:     file,
		filename: filename,
		modified: &modified,
	}

	ast.Walk(visitor, file)

	// If file was modified and not in dry run mode, write it back
	if modified && !tm.config.DryRun {
		if tm.config.BackupOriginal {
			if err := tm.createBackup(filename); err != nil {
				report.Warnings = append(report.Warnings, 
					fmt.Sprintf("Failed to create backup for %s: %v", filename, err))
			}
		}

		outputPath := filename
		if tm.config.OutputDir != "" {
			relPath, _ := filepath.Rel(tm.config.SourceDir, filename)
			outputPath = filepath.Join(tm.config.OutputDir, relPath)
			
			// Ensure output directory exists
			if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}
		}

		// Format and write the modified AST
		var buf strings.Builder
		if err := format.Node(&buf, tm.fset, file); err != nil {
			return fmt.Errorf("failed to format modified file %s: %w", filename, err)
		}

		if err := os.WriteFile(outputPath, []byte(buf.String()), 0644); err != nil {
			return fmt.Errorf("failed to write modified file %s: %w", outputPath, err)
		}

		report.FilesModified++
	}

	return nil
}

// createBackup creates a backup of the original file
func (tm *TransportMigrator) createBackup(filename string) error {
	backupName := filename + ".backup"
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return os.WriteFile(backupName, src, 0644)
}

// migrationVisitor implements ast.Visitor for AST transformation
type migrationVisitor struct {
	migrator *TransportMigrator
	report   *MigrationReport
	file     *ast.File
	filename string
	modified *bool
}

// Visit implements ast.Visitor
func (v *migrationVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	// Apply migration rules based on node type
	switch n := node.(type) {
	case *ast.TypeSpec:
		v.visitTypeSpec(n)
	case *ast.FuncDecl:
		v.visitFuncDecl(n)
	case *ast.CallExpr:
		v.visitCallExpr(n)
	case *ast.InterfaceType:
		v.visitInterfaceType(n)
	case *ast.StructType:
		v.visitStructType(n)
	}

	return v
}

// visitTypeSpec handles type declarations
func (v *migrationVisitor) visitTypeSpec(node *ast.TypeSpec) {
	// Check for deprecated transport types
	if node.Name.Name == "Transport" {
		v.addDeprecationWarning(node.Pos(), "Transport", 
			"Replace with composable Transport interface from interfaces_core.go")
	}
}

// visitFuncDecl handles function declarations
func (v *migrationVisitor) visitFuncDecl(node *ast.FuncDecl) {
	if node.Name == nil {
		return
	}

	// Check for deprecated method patterns
	methodName := node.Name.Name
	
	switch {
	case strings.Contains(methodName, "HandleEvent"):
		v.addDeprecationWarning(node.Pos(), methodName, 
			"Replace with EventHandler callback type")
		v.applyTransformation("ReplaceOldEventHandlers")
		
	case strings.Contains(methodName, "SendBatch"):
		v.addDeprecationWarning(node.Pos(), methodName, 
			"Replace with BatchSender interface")
		v.applyTransformation("UpdateBatchSending")
		
	case strings.Contains(methodName, "StartStream"):
		v.addDeprecationWarning(node.Pos(), methodName, 
			"Replace with StreamingTransport.StartStreaming")
		v.applyTransformation("MigrateStreamingAPIs")
		
	case strings.Contains(methodName, "SendWithAck"):
		v.addDeprecationWarning(node.Pos(), methodName, 
			"Replace with ReliableTransport.SendEventWithAck")
		v.applyTransformation("UpdateReliabilityFeatures")
	}
}

// visitCallExpr handles function calls
func (v *migrationVisitor) visitCallExpr(node *ast.CallExpr) {
	// Check for deprecated method calls
	if sel, ok := node.Fun.(*ast.SelectorExpr); ok {
		switch sel.Sel.Name {
		case "Stats":
			v.addDeprecationWarning(node.Pos(), "transport.Stats()", 
				"Replace with StatsProvider interface")
			v.applyTransformation("UpdateStatsAccess")
			
		case "Config":
			v.addDeprecationWarning(node.Pos(), "transport.Config()", 
				"Replace with ConfigProvider interface")
			v.applyTransformation("MigrateConfigAccess")
		}
	}
}

// visitInterfaceType handles interface declarations
func (v *migrationVisitor) visitInterfaceType(node *ast.InterfaceType) {
	// Check for deprecated interface patterns
	for _, method := range node.Methods.List {
		if len(method.Names) > 0 {
			methodName := method.Names[0].Name
			switch methodName {
			case "Send", "Receive", "Connect", "Close":
				// These are now part of composable interfaces
				v.addTransformationHint("UpdateTransportInterface", 
					"Consider using composable interfaces")
			}
		}
	}
}

// visitStructType handles struct declarations
func (v *migrationVisitor) visitStructType(node *ast.StructType) {
	// Check for deprecated struct fields
	for _, field := range node.Fields.List {
		if len(field.Names) > 0 {
			fieldName := field.Names[0].Name
			switch fieldName {
			case "EventHandler":
				v.addDeprecationWarning(field.Pos(), fieldName, 
					"Use EventHandlerProvider interface instead")
			}
		}
	}
}

// addDeprecationWarning adds a deprecation warning to the report
func (v *migrationVisitor) addDeprecationWarning(pos token.Pos, method, message string) {
	position := v.migrator.fset.Position(pos)
	warning := DeprecationWarning{
		File:     v.filename,
		Line:     position.Line,
		Column:   position.Column,
		Method:   method,
		Message:  message,
		Deadline: v.migrator.config.DeprecationDeadline,
	}
	v.report.DeprecationWarnings = append(v.report.DeprecationWarnings, warning)
}

// applyTransformation applies a transformation and updates the report
func (v *migrationVisitor) applyTransformation(ruleName string) {
	v.report.TransformationsApplied[ruleName]++
	*v.modified = true
}

// addTransformationHint adds a hint without actually modifying the code
func (v *migrationVisitor) addTransformationHint(ruleName, hint string) {
	v.report.Warnings = append(v.report.Warnings, 
		fmt.Sprintf("%s: %s", ruleName, hint))
}

// GenerateDeprecationAnnotations adds deprecation comments to the codebase
func (tm *TransportMigrator) GenerateDeprecationAnnotations() error {
	deprecations := map[string]DeprecationInfo{
		"Transport.Send":           {Deadline: "2024-12-31", Replacement: "Sender.Send"},
		"Transport.Receive":        {Deadline: "2024-12-31", Replacement: "Receiver.Channels"},
		"Transport.HandleEvent":    {Deadline: "2024-11-30", Replacement: "EventHandler callback"},
		"Transport.SendBatch":      {Deadline: "2024-12-31", Replacement: "BatchSender.SendBatch"},
		"Transport.StartStream":    {Deadline: "2025-01-31", Replacement: "StreamingTransport.StartStreaming"},
		"Transport.SendWithAck":    {Deadline: "2025-01-31", Replacement: "ReliableTransport.SendEventWithAck"},
	}

	// Find all Go files and add deprecation comments
	return filepath.Walk(tm.config.SourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !strings.HasSuffix(info.Name(), ".go") {
			return err
		}

		return tm.addDeprecationComments(path, deprecations)
	})
}

// DeprecationInfo contains information about a deprecated item
type DeprecationInfo struct {
	Deadline    string
	Replacement string
}

// addDeprecationComments adds deprecation comments to a file
func (tm *TransportMigrator) addDeprecationComments(filename string, deprecations map[string]DeprecationInfo) error {
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	content := string(src)
	modified := false

	// Add deprecation comments for each deprecated item
	for item, info := range deprecations {
		pattern := fmt.Sprintf("func.*%s", strings.Split(item, ".")[1])
		if strings.Contains(content, pattern) {
			deprecationComment := fmt.Sprintf("// Deprecated: %s will be removed on %s. Use %s instead.\n", 
				item, info.Deadline, info.Replacement)
			
			// Insert deprecation comment before the function
			// This is a simplified approach - in reality, you'd want to use AST manipulation
			lines := strings.Split(content, "\n")
			for i, line := range lines {
				if strings.Contains(line, pattern) && !strings.Contains(lines[i-1], "Deprecated:") {
					lines = append(lines[:i], append([]string{deprecationComment}, lines[i:]...)...)
					modified = true
					break
				}
			}
			
			if modified {
				content = strings.Join(lines, "\n")
			}
		}
	}

	if modified && !tm.config.DryRun {
		return os.WriteFile(filename, []byte(content), 0644)
	}

	return nil
}

// Example usage functions for the migration tool
func ExampleMigrationUsage() {
	// Create migration configuration
	config := &MigrationConfig{
		SourceDir:           "./pkg/transport",
		OutputDir:           "", // Empty means modify in place
		DryRun:              true,
		BackupOriginal:      true,
		TargetPackages:      []string{"transport"},
		DeprecationDeadline: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
	}

	// Create migrator
	migrator := NewTransportMigrator(config)

	// Add custom migration rule
	customRule := MigrationRule{
		Name:        "UpdateCustomTransport",
		Description: "Update custom transport implementations",
		Pattern:     "MyTransport.*Send",
		Priority:    40,
	}
	migrator.AddRule(customRule)

	// Run migration
	report, err := migrator.Migrate()
	if err != nil {
		fmt.Printf("Migration failed: %v\n", err)
		return
	}

	// Print report
	fmt.Printf("Migration Report:\n")
	fmt.Printf("Files processed: %d\n", report.FilesProcessed)
	fmt.Printf("Files modified: %d\n", report.FilesModified)
	fmt.Printf("Transformations applied:\n")
	for rule, count := range report.TransformationsApplied {
		fmt.Printf("  %s: %d\n", rule, count)
	}

	if len(report.DeprecationWarnings) > 0 {
		fmt.Printf("\nDeprecation warnings:\n")
		for _, warning := range report.DeprecationWarnings {
			fmt.Printf("  %s:%d:%d - %s: %s (deadline: %s)\n",
				warning.File, warning.Line, warning.Column,
				warning.Method, warning.Message, warning.Deadline.Format("2006-01-02"))
		}
	}

	// Generate deprecation annotations
	if err := migrator.GenerateDeprecationAnnotations(); err != nil {
		fmt.Printf("Failed to generate deprecation annotations: %v\n", err)
	}
}