package testing

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// GoroutineLeakDetector helps detect goroutine leaks in tests
type GoroutineLeakDetector struct {
	baselineCount int
	ignored       map[string]bool
	mu            sync.RWMutex
}

// NewGoroutineLeakDetector creates a new goroutine leak detector
func NewGoroutineLeakDetector() *GoroutineLeakDetector {
	return &GoroutineLeakDetector{
		baselineCount: runtime.NumGoroutine(),
		ignored: map[string]bool{
			"testing.(*T).Run":                    true,
			"testing.tRunner":                     true,
			"testing.(*M).Run":                    true,
			"runtime.goexit":                      true,
			"go.uber.org/goleak":                  true,
			"github.com/stretchr/testify":         true,
			"net/http.(*Server).Serve":            true,
			"net/http.(*Server).ListenAndServe":   true,
			"internal/poll.runtime_pollWait":      true,
			"go.uber.org/zap/zapcore.(*sampler)":  true,
			// Transport layer specific ignored patterns
			"net/http.(*Transport).dialConnFor":   true,
			"net/http.(*persistConn).readLoop":    true,
			"net/http.(*persistConn).writeLoop":   true,
		},
	}
}

// IgnoreFunction adds a function pattern to ignore in leak detection
func (g *GoroutineLeakDetector) IgnoreFunction(pattern string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.ignored[pattern] = true
}

// CheckForLeaks checks if there are any goroutine leaks since the detector was created
func (g *GoroutineLeakDetector) CheckForLeaks(t testing.TB) {
	g.CheckForLeaksWithTimeout(t, 5*time.Second)
}

// CheckForLeaksWithTimeout checks for leaks with a custom timeout
func (g *GoroutineLeakDetector) CheckForLeaksWithTimeout(t testing.TB, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Wait for goroutines to settle
	time.Sleep(100 * time.Millisecond)

	// Poll for goroutine count to stabilize
	stabilized := g.waitForStableGoroutineCount(ctx, 100*time.Millisecond)
	
	if !stabilized {
		t.Logf("Warning: Goroutine count did not stabilize within %v", timeout)
	}

	currentCount := runtime.NumGoroutine()
	
	// Allow for some variance in goroutine count
	tolerance := 2
	if currentCount > g.baselineCount+tolerance {
		leaks := g.detectLeaks()
		
		if len(leaks) > 0 {
			t.Errorf("Detected %d potential goroutine leaks (baseline: %d, current: %d):\n%s",
				len(leaks), g.baselineCount, currentCount, strings.Join(leaks, "\n"))
		} else {
			t.Logf("Goroutine count increased from %d to %d, but no obvious leaks detected",
				g.baselineCount, currentCount)
		}
	}
}

// waitForStableGoroutineCount waits for the goroutine count to stabilize
func (g *GoroutineLeakDetector) waitForStableGoroutineCount(ctx context.Context, checkInterval time.Duration) bool {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	// Initialize with current count to avoid initial mismatch
	previousCount := runtime.NumGoroutine()
	stableCount := 0
	requiredStableChecks := 3

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			currentCount := runtime.NumGoroutine()
			if currentCount == previousCount {
				stableCount++
				if stableCount >= requiredStableChecks {
					return true
				}
			} else {
				stableCount = 0
				previousCount = currentCount
			}
		}
	}
}

// detectLeaks returns a list of suspected goroutine leaks
func (g *GoroutineLeakDetector) detectLeaks() []string {
	buf := make([]byte, 64<<20) // 64MB buffer for stack traces
	stackSize := runtime.Stack(buf, true)
	stackTrace := string(buf[:stackSize])

	// Split stack trace into individual goroutines
	goroutines := strings.Split(stackTrace, "\n\n")
	var leaks []string

	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, goroutine := range goroutines {
		if g.isLikelyLeak(goroutine) {
			// Extract just the first few lines for readability
			lines := strings.Split(goroutine, "\n")
			if len(lines) > 10 {
				lines = lines[:10]
				lines = append(lines, "... (truncated)")
			}
			leaks = append(leaks, strings.Join(lines, "\n"))
		}
	}

	return leaks
}

// isLikelyLeak determines if a goroutine stack trace represents a likely leak
func (g *GoroutineLeakDetector) isLikelyLeak(stackTrace string) bool {
	// Skip empty or very short traces
	if len(stackTrace) < 50 {
		return false
	}

	// Check if this goroutine should be ignored
	for pattern := range g.ignored {
		if strings.Contains(stackTrace, pattern) {
			return false
		}
	}

	// Look for common leak patterns
	leakPatterns := []string{
		"time.Sleep",
		"select {",
		"chan receive",
		"chan send",
		"sync.(*WaitGroup).Wait",
		"sync.(*Mutex).Lock",
		"context.Context.Done",
	}

	for _, pattern := range leakPatterns {
		if strings.Contains(stackTrace, pattern) {
			return true
		}
	}

	return false
}

// ResetBaseline resets the baseline goroutine count to the current count
func (g *GoroutineLeakDetector) ResetBaseline() {
	g.baselineCount = runtime.NumGoroutine()
}

// GetCurrentCount returns the current number of goroutines
func (g *GoroutineLeakDetector) GetCurrentCount() int {
	return runtime.NumGoroutine()
}

// GetBaseline returns the baseline goroutine count
func (g *GoroutineLeakDetector) GetBaseline() int {
	return g.baselineCount
}

// WithGoroutineLeakDetection is a test helper that automatically checks for leaks
func WithGoroutineLeakDetection(t *testing.T, testFunc func(t *testing.T)) {
	detector := NewGoroutineLeakDetector()
	
	// Run the test
	testFunc(t)
	
	// Check for leaks after test completion
	detector.CheckForLeaks(t)
}

// TestingCleanupHelper helps with test cleanup and leak detection
type TestingCleanupHelper struct {
	detector    *GoroutineLeakDetector
	cleanupFuncs []func()
	mu          sync.Mutex
}

// NewTestingCleanupHelper creates a new testing cleanup helper
func NewTestingCleanupHelper() *TestingCleanupHelper {
	return &TestingCleanupHelper{
		detector: NewGoroutineLeakDetector(),
	}
}

// AddCleanup adds a cleanup function to be called during teardown
func (h *TestingCleanupHelper) AddCleanup(cleanup func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cleanupFuncs = append(h.cleanupFuncs, cleanup)
}

// Cleanup performs all registered cleanup functions and checks for leaks
func (h *TestingCleanupHelper) Cleanup(t testing.TB) {
	h.mu.Lock()
	cleanups := make([]func(), len(h.cleanupFuncs))
	copy(cleanups, h.cleanupFuncs)
	h.mu.Unlock()

	// Run cleanup functions in reverse order (LIFO)
	for i := len(cleanups) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Cleanup function panicked: %v", r)
				}
			}()
			cleanups[i]()
		}()
	}

	// Check for goroutine leaks after cleanup
	h.detector.CheckForLeaks(t)
}

// Example usage in tests:
//
// func TestSomething(t *testing.T) {
//     helper := NewTestingCleanupHelper()
//     defer helper.Cleanup(t)
//     
//     // Your test code here
//     ctx, cancel := context.WithCancel(context.Background())
//     helper.AddCleanup(cancel)
//     
//     // More test code...
// }