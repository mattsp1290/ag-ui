package sdk_test

import (
	"os/exec"
	"testing"
	
	_ "github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// TestImport verifies that the events package can be imported
func TestImport(t *testing.T) {
	// The import statement above will fail at compile time if there's an issue
	t.Log("Import successful")
}

// TestBuild verifies that the events package can be built
func TestBuild(t *testing.T) {
	cmd := exec.Command("go", "build", "./pkg/core/events")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("Build failed with error: %v\nOutput: %s", err, string(output))
	} else {
		t.Log("Build successful!")
	}
}