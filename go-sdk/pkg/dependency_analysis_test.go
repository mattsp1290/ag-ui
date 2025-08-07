package pkg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDependencyAnalysis runs the dependency analysis and reports results
func TestDependencyAnalysis(t *testing.T) {
	// Get the root path of the SDK
	rootPath, err := filepath.Abs("..")
	require.NoError(t, err)

	// Run dependency analysis
	graph, err := AnalyzePackageDependencies(rootPath)
	require.NoError(t, err)

	// Generate report
	report := graph.PrintDependencyReport()

	// Print report
	t.Logf("\n%s", report)

	// Check for circular dependencies
	cycles := graph.FindCircularDependencies()
	if len(cycles) > 0 {
		t.Errorf("Found %d circular dependencies", len(cycles))
		for i, cycle := range cycles {
			t.Errorf("Cycle %d: %v", i+1, cycle)
		}
	}

	// Write report to file
	reportPath := filepath.Join(rootPath, "pkg", "DEPENDENCY_ANALYSIS.md")
	err = os.WriteFile(reportPath, []byte("# Dependency Analysis Report\n\n```\n"+report+"\n```\n"), 0644)
	require.NoError(t, err)

	t.Logf("Dependency analysis report written to: %s", reportPath)
}
