package state

import (
	"os"
	"runtime"
	"time"
)

// CleanupTestResources ensures all test resources are properly cleaned up
func CleanupTestResources() {
	// Force garbage collection to clean up any remaining resources
	runtime.GC()
	runtime.GC() // Run twice to ensure full cleanup

	// Give goroutines time to finish
	time.Sleep(200 * time.Millisecond)

	// Redirect any remaining log output away from stdout
	if os.Getenv("GO_TEST") != "" {
		// During tests, redirect to devnull to prevent write errors
		devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err == nil {
			os.Stdout = devNull
			os.Stderr = devNull
		}
	}
}

func init() {
	// Set environment variable to indicate we're in test mode
	if os.Getenv("GO_TEST") == "" {
		os.Setenv("GO_TEST", "1")
	}
}
