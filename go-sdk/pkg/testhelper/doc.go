// Package testhelper provides utilities for testing, including goroutine leak detection,
// resource cleanup helpers, and context management utilities.
//
// The package is designed to help identify and prevent common testing issues such as:
// - Goroutine leaks that can cause tests to hang or fail intermittently
// - Resource leaks from unclosed channels, connections, or workers
// - Context cancellation issues that leave operations running
//
// Example usage:
//
//	func TestExample(t *testing.T) {
//	    // Detect goroutine leaks
//	    defer testhelper.VerifyNoGoroutineLeaks(t)
//
//	    // Use auto-cleanup context
//	    ctx := testhelper.NewTestContext(t)
//
//	    // Your test code here
//	}
package testhelper
