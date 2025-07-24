package websocket

import (
	"os"
	"testing"
	"time"
)

// TestMain sets up global test configuration to prevent hanging tests
func TestMain(m *testing.M) {
	// Set aggressive defaults for test environment
	originalTimeout := os.Getenv("GOMAXPROCS")
	defer func() {
		if originalTimeout != "" {
			os.Setenv("GOMAXPROCS", originalTimeout)
		}
	}()
	
	// Limit goroutine creation for resource-constrained environments
	os.Setenv("GOMAXPROCS", "4")
	
	// Set global test timeout if not already set
	if os.Getenv("TEST_TIMEOUT") == "" {
		os.Setenv("TEST_TIMEOUT", "30s")
	}
	
	// Run tests
	code := m.Run()
	
	// Cleanup
	time.Sleep(100 * time.Millisecond) // Give goroutines time to finish
	
	os.Exit(code)
}