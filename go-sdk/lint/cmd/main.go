// Package main provides a command-line interface for running custom static analyzers.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ag-ui/go-sdk/lint"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	// Parse command line flags
	var (
		analyzer = flag.String("analyzer", "typesafety", "Analyzer to run (typesafety, migration)")
		output   = flag.String("output", "docs/linting", "Output directory for documentation")
		docs     = flag.Bool("docs", false, "Generate documentation instead of running analysis")
	)
	flag.Parse()

	if *docs {
		// Generate documentation
		if err := generateDocumentation(*output); err != nil {
			log.Fatalf("Failed to generate documentation: %v", err)
		}
		fmt.Printf("Documentation generated in: %s\n", *output)
		return
	}

	// Run static analyzer
	switch *analyzer {
	case "typesafety":
		singlechecker.Main(lint.TypeSafetyAnalyzer)
	case "migration":
		singlechecker.Main(lint.MigrationRulesAnalyzer)
	default:
		fmt.Fprintf(os.Stderr, "Unknown analyzer: %s\n", *analyzer)
		fmt.Fprintf(os.Stderr, "Available analyzers: typesafety, migration\n")
		os.Exit(1)
	}
}

// generateDocumentation generates all documentation files.
func generateDocumentation(outputDir string) error {
	generator := lint.NewRuleDocGenerator(outputDir)
	return generator.GenerateAll()
}