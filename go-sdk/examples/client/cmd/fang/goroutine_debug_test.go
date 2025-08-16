package main

import (
	"runtime"
	"testing"
	"time"
)

// TestGoroutineLeaksAfterChat tests for leaks after running chat commands
func TestGoroutineLeaksAfterChat(t *testing.T) {
	// Get baseline
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()
	
	t.Logf("Baseline goroutine count: %d", baseline)
	
	// Run a chat command that will fail (no server)
	cmd := newRootCommand()
	cmd.SetArgs([]string{"chat", "--message", "test", "--server", "http://localhost:99999"})
	
	// We expect this to fail due to connection error
	_ = cmd.Execute()
	
	// Allow cleanup
	time.Sleep(1 * time.Second)
	runtime.GC()
	
	// Check for leaks
	current := runtime.NumGoroutine()
	t.Logf("After chat command: %d goroutines", current)
	
	if current > baseline+3 {
		// Print stack traces for debugging
		buf := make([]byte, 1<<20)
		stackLen := runtime.Stack(buf, true)
		t.Logf("=== GOROUTINE LEAK DETECTED ===")
		t.Logf("Baseline: %d, Current: %d, Leaked: %d", baseline, current, current-baseline)
		t.Logf("\n=== STACK TRACES ===\n%s", buf[:stackLen])
		
		t.Errorf("Goroutine leak detected: baseline=%d, current=%d", baseline, current)
	}
}

// TestMultipleCommandsSequential tests running multiple commands in sequence
func TestMultipleCommandsSequential(t *testing.T) {
	// Get baseline
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()
	
	t.Logf("Baseline goroutine count: %d", baseline)
	
	// Run multiple chat commands
	for i := 0; i < 3; i++ {
		cmd := newRootCommand()
		cmd.SetArgs([]string{"chat", "--message", "test", "--server", "http://localhost:99999"})
		
		// We expect this to fail
		_ = cmd.Execute()
		
		// Small delay between commands
		time.Sleep(200 * time.Millisecond)
		runtime.GC()
		
		current := runtime.NumGoroutine()
		t.Logf("After command %d: %d goroutines", i+1, current)
	}
	
	// Final cleanup wait
	time.Sleep(1 * time.Second)
	runtime.GC()
	
	final := runtime.NumGoroutine()
	t.Logf("Final goroutine count: %d", final)
	
	if final > baseline+3 {
		// Print stack traces for debugging
		buf := make([]byte, 1<<20)
		stackLen := runtime.Stack(buf, true)
		t.Logf("=== GOROUTINE LEAK DETECTED ===")
		t.Logf("Baseline: %d, Final: %d, Leaked: %d", baseline, final, final-baseline)
		t.Logf("\n=== STACK TRACES ===\n%s", buf[:stackLen])
		
		t.Errorf("Goroutine leak after multiple commands: baseline=%d, final=%d", baseline, final)
	}
}