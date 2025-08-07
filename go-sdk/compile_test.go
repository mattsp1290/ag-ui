//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	// Get the current directory
	dir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	// Change to the go-sdk directory
	goSDKDir := filepath.Join(dir)
	if err := os.Chdir(goSDKDir); err != nil {
		fmt.Printf("Error changing directory: %v\n", err)
		os.Exit(1)
	}

	// Try to build the events package
	fmt.Println("Attempting to build pkg/core/events package...")

	// First, let's check if the package can be imported
	pkg, err := build.Import("github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events", ".", build.FindOnly)
	if err != nil {
		fmt.Printf("Error finding package: %v\n", err)
	} else {
		fmt.Printf("Package found at: %s\n", pkg.Dir)
	}

	// Run go build with verbose output
	cmd := exec.Command("go", "build", "-v", "./pkg/core/events")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("\nRunning: go build -v ./pkg/core/events")
	if err := cmd.Run(); err != nil {
		fmt.Printf("\nBuild failed with error: %v\n", err)

		// Try to get more detailed error information
		cmd2 := exec.Command("go", "list", "-e", "-json", "./pkg/core/events")
		output, err2 := cmd2.CombinedOutput()
		if err2 == nil {
			fmt.Println("\nPackage information:")
			fmt.Println(string(output))
		}

		// Also try to compile specific files
		fmt.Println("\nTrying to compile individual files...")
		files := []string{
			"pkg/core/events/events.go",
			"pkg/core/events/message_events.go",
			"pkg/core/events/run_events.go",
			"pkg/core/events/tool_events.go",
			"pkg/core/events/state_events.go",
			"pkg/core/events/custom_events.go",
		}

		for _, file := range files {
			cmd3 := exec.Command("go", "build", "-o", "/dev/null", file)
			if output, err3 := cmd3.CombinedOutput(); err3 != nil {
				fmt.Printf("\nError building %s:\n%s\n", file, strings.TrimSpace(string(output)))
			}
		}
	} else {
		fmt.Println("\nBuild successful!")
	}
}
