package main

import (
	"runtime"
	"testing"
	"time"
)

// TestNoGoroutineLeaks verifies that tests don't leave goroutines running
func TestNoGoroutineLeaks(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping goroutine leak test in short mode")
	}

	// Get initial goroutine count
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	initialCount := runtime.NumGoroutine()
	
	t.Logf("Initial goroutine count: %d", initialCount)
	
	// Run a simple command
	cmd := newRootCommand()
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Failed to execute help command: %v", err)
	}
	
	// Allow time for cleanup
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	
	// Check final count
	finalCount := runtime.NumGoroutine()
	t.Logf("Final goroutine count: %d", finalCount)
	
	// Allow some variance for test framework
	if finalCount > initialCount+2 {
		// Print stack traces for debugging
		buf := make([]byte, 1<<20)
		stackLen := runtime.Stack(buf, true)
		t.Logf("Goroutine stack traces:\n%s", buf[:stackLen])
		
		t.Errorf("Potential goroutine leak: started with %d, ended with %d goroutines",
			initialCount, finalCount)
	}
}

// TestCommandCleanup tests that commands clean up properly between executions
func TestCommandCleanup(t *testing.T) {
	// Get baseline
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()
	
	t.Logf("Baseline goroutine count: %d", baseline)
	
	// Run multiple commands in sequence
	for i := 0; i < 3; i++ {
		cmd := newRootCommand()
		cmd.SetArgs([]string{"--help"})
		
		err := cmd.Execute()
		if err != nil {
			t.Fatalf("Failed to execute command %d: %v", i, err)
		}
		
		// Allow cleanup between commands
		time.Sleep(100 * time.Millisecond)
		runtime.GC()
		
		current := runtime.NumGoroutine()
		t.Logf("After command %d: %d goroutines", i, current)
		
		// Check for leak after each command
		if current > baseline+5 {
			t.Errorf("Goroutine leak after command %d: baseline=%d, current=%d", 
				i, baseline, current)
		}
	}
	
	// Final check
	time.Sleep(500 * time.Millisecond)
	runtime.GC()
	final := runtime.NumGoroutine()
	
	t.Logf("Final goroutine count: %d", final)
	
	if final > baseline+2 {
		t.Errorf("Final goroutine leak: baseline=%d, final=%d", baseline, final)
	}
}