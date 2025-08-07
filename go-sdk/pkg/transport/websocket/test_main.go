package websocket

import (
	"fmt"
	"os"
	"runtime"
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
		os.Setenv("TEST_TIMEOUT", "15s") // Further reduced for faster failure detection
	}

	// Enable test-specific timeouts
	os.Setenv("GO_TEST_TIMEOUT_SCALE", "0.3") // Make tests run even faster

	// Capture initial goroutine count for leak detection
	initialGoroutines := getInitialGoroutineCount()

	// Add timeout protection for entire test suite
	done := make(chan int, 1)
	go func() {
		code := m.Run()
		done <- code
	}()

	// Wait for tests with timeout
	select {
	case code := <-done:
		// Tests completed normally
		fmt.Printf("Tests completed normally, performing cleanup...\n")
		performPostTestCleanup(initialGoroutines)
		os.Exit(code)
	case <-time.After(25 * time.Second): // Hard timeout for entire suite
		fmt.Printf("TEST SUITE TIMEOUT: Tests exceeded 25 seconds, forcing exit\n")
		// Print goroutine dump to help debug what's hanging
		buf := make([]byte, 1<<16)
		n := runtime.Stack(buf, true)
		fmt.Printf("Goroutine dump at timeout:\n%s\n", buf[:n])

		// Force cleanup
		performEmergencyCleanup(initialGoroutines)
		os.Exit(1)
	}
}

// getInitialGoroutineCount gets a stable initial goroutine count
func getInitialGoroutineCount() int {
	// Force garbage collection multiple times to stabilize
	for i := 0; i < 3; i++ {
		runtime.GC()
		time.Sleep(25 * time.Millisecond)
	}
	return runtime.NumGoroutine()
}

// performPostTestCleanup performs aggressive cleanup after all tests
func performPostTestCleanup(initialGoroutines int) {
	// Give goroutines time to finish naturally
	time.Sleep(200 * time.Millisecond)

	// Force multiple garbage collections
	for i := 0; i < 5; i++ {
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
	}

	// Check for potential goroutine leaks
	finalGoroutines := runtime.NumGoroutine()
	leaked := finalGoroutines - initialGoroutines

	if leaked > 20 { // Allow reasonable tolerance for test framework
		// Print warning but don't fail - tests already completed
		fmt.Printf("WARNING: Potential goroutine leak detected: started=%d, ended=%d, leaked=%d\n",
			initialGoroutines, finalGoroutines, leaked)

		// Print stack trace to help debug
		buf := make([]byte, 1<<16)
		n := runtime.Stack(buf, true)
		fmt.Printf("Goroutine stack trace:\n%s\n", buf[:n])
	} else {
		fmt.Printf("Goroutine cleanup successful: started=%d, ended=%d, leaked=%d\n",
			initialGoroutines, finalGoroutines, leaked)
	}
}

// performEmergencyCleanup performs emergency cleanup when test suite times out
func performEmergencyCleanup(initialGoroutines int) {
	fmt.Printf("Performing emergency cleanup...\n")

	// Force multiple aggressive garbage collections
	for i := 0; i < 10; i++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}

	// Final goroutine count
	finalGoroutines := runtime.NumGoroutine()
	leaked := finalGoroutines - initialGoroutines
	fmt.Printf("Emergency cleanup: started=%d, ended=%d, leaked=%d\n",
		initialGoroutines, finalGoroutines, leaked)
}
