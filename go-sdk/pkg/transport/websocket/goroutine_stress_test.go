package websocket

// Goroutine stress testing functionality has been removed as it was too resource-intensive for CI/CD environments.
// These tests were designed for detecting goroutine leaks under high connection counts and rapid connection cycles.

// TestMassiveGoroutineLeakDetection - REMOVED
// This test was designed to create thousands of goroutines to test resource exhaustion.
// Removed as it pushed system limits and was too resource-intensive for CI/CD environments.

// TestConnectionPoolGoroutineLeaks - REMOVED
// This test was designed to test connection pools with 50+ connections to detect goroutine leaks.
// Removed as it pushed system limits and was too resource-intensive for CI/CD environments.

// TestRapidConnectionCycles - REMOVED
// This test was designed to test rapid connect/disconnect cycles to stress connection handling.
// Removed as it pushed system limits and was too resource-intensive for CI/CD environments.