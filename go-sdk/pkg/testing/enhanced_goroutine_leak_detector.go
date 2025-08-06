package testing

import (
	"context"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// EnhancedGoroutineLeakDetector provides comprehensive goroutine leak detection
// with advanced features for complex systems with background goroutines.
type EnhancedGoroutineLeakDetector struct {
	t                testing.TB
	baseline         GoroutineSnapshot
	tolerance        int
	maxWaitTime      time.Duration
	checkInterval    time.Duration
	excludePatterns  []string
	includePatterns  []string
	stackBlacklist   map[string]bool
	mu               sync.RWMutex
}

// GoroutineSnapshot represents a snapshot of goroutines at a point in time
type GoroutineSnapshot struct {
	Count      int
	Stacks     map[string]int // stack signature -> count
	Timestamp  time.Time
	FullStacks []string // full stack traces for debugging
}

// NewEnhancedGoroutineLeakDetector creates a new enhanced leak detector
func NewEnhancedGoroutineLeakDetector(t testing.TB) *EnhancedGoroutineLeakDetector {
	detector := &EnhancedGoroutineLeakDetector{
		t:             t,
		tolerance:     3,
		maxWaitTime:   10 * time.Second,
		checkInterval: 100 * time.Millisecond,
		excludePatterns: []string{
			// Test framework goroutines
			"testing.(*T)",
			"testing.tRunner",
			"testing.(*M).Run",
			"runtime.goexit",
			
			// Standard library background goroutines
			"runtime.bgsweep",
			"runtime.bgscavenge",
			"runtime.forcegchelper",
			"finalizer",
			
			// Net/HTTP transport goroutines (expected for HTTP clients)
			"net/http.(*Transport).dialConnFor",
			"net/http.(*persistConn).readLoop",
			"net/http.(*persistConn).writeLoop",
			"internal/poll.runtime_pollWait",
			
			// Context and timer goroutines (often expected)
			"time.goFunc",
			"context.propagateCancel",
			
			// Third-party libraries
			"go.uber.org/zap",
			"go.uber.org/goleak",
			"github.com/stretchr/testify",
		},
		stackBlacklist: make(map[string]bool),
	}
	
	detector.captureBaseline()
	return detector
}

// WithTolerance sets the number of allowed extra goroutines
func (d *EnhancedGoroutineLeakDetector) WithTolerance(tolerance int) *EnhancedGoroutineLeakDetector {
	d.tolerance = tolerance
	return d
}

// WithMaxWaitTime sets the maximum time to wait for cleanup
func (d *EnhancedGoroutineLeakDetector) WithMaxWaitTime(duration time.Duration) *EnhancedGoroutineLeakDetector {
	d.maxWaitTime = duration
	return d
}

// WithExcludePatterns adds patterns to exclude from leak detection
func (d *EnhancedGoroutineLeakDetector) WithExcludePatterns(patterns ...string) *EnhancedGoroutineLeakDetector {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.excludePatterns = append(d.excludePatterns, patterns...)
	return d
}

// WithIncludePatterns adds patterns to specifically include in leak detection
func (d *EnhancedGoroutineLeakDetector) WithIncludePatterns(patterns ...string) *EnhancedGoroutineLeakDetector {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.includePatterns = append(d.includePatterns, patterns...)
	return d
}

// captureBaseline captures the initial state of goroutines
func (d *EnhancedGoroutineLeakDetector) captureBaseline() {
	// Force GC to clean up any lingering goroutines
	runtime.GC()
	runtime.GC()
	
	// Small delay to let cleanup complete
	time.Sleep(50 * time.Millisecond)
	
	d.baseline = d.captureSnapshot()
	d.t.Logf("Baseline captured: %d goroutines", d.baseline.Count)
}

// captureSnapshot captures current goroutine state
func (d *EnhancedGoroutineLeakDetector) captureSnapshot() GoroutineSnapshot {
	buf := make([]byte, 2<<20) // 2MB buffer for stack traces
	n := runtime.Stack(buf, true)
	
	snapshot := GoroutineSnapshot{
		Count:     runtime.NumGoroutine(),
		Stacks:    make(map[string]int),
		Timestamp: time.Now(),
	}
	
	if n > 0 {
		stackTrace := string(buf[:n])
		snapshot.FullStacks = strings.Split(stackTrace, "\n\n")
		
		// Parse individual goroutine stacks
		for _, stack := range snapshot.FullStacks {
			if strings.TrimSpace(stack) == "" {
				continue
			}
			
			signature := d.extractStackSignature(stack)
			if signature != "" {
				snapshot.Stacks[signature]++
			}
		}
	}
	
	return snapshot
}

// extractStackSignature creates a signature from a stack trace for comparison
func (d *EnhancedGoroutineLeakDetector) extractStackSignature(stack string) string {
	lines := strings.Split(stack, "\n")
	if len(lines) < 3 {
		return ""
	}
	
	// Use first line (goroutine info) + first function call as signature
	var signature strings.Builder
	signature.WriteString(strings.TrimSpace(lines[0]))
	
	// Find the first meaningful function call (skip runtime internals)
	for i := 1; i < len(lines) && i < 10; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		
		// Skip file:line information, look for function calls
		if !strings.Contains(line, ".go:") && !strings.HasPrefix(line, "\t") {
			// This is likely a function name
			signature.WriteString(" -> ")
			signature.WriteString(line)
			break
		}
	}
	
	return signature.String()
}

// Check verifies no goroutines have leaked with sophisticated retry logic
func (d *EnhancedGoroutineLeakDetector) Check() {
	d.CheckWithContext(context.Background())
}

// CheckWithContext performs leak detection with a context for cancellation
func (d *EnhancedGoroutineLeakDetector) CheckWithContext(ctx context.Context) {
	d.t.Helper()
	
	// Create a context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, d.maxWaitTime)
	defer cancel()
	
	ticker := time.NewTicker(d.checkInterval)
	defer ticker.Stop()
	
	var currentSnapshot GoroutineSnapshot
	
	// Wait for goroutines to stabilize
	for {
		select {
		case <-checkCtx.Done():
			// Timeout reached, perform final check
			currentSnapshot = d.captureSnapshot()
			break
		case <-ticker.C:
			// Force garbage collection to help cleanup
			runtime.GC()
			runtime.GC()
			
			currentSnapshot = d.captureSnapshot()
			leaked := d.analyzeLeak(currentSnapshot)
			
			// If within tolerance, we're good
			if len(leaked) <= d.tolerance {
				d.t.Logf("Goroutine cleanup successful: baseline=%d, current=%d, leaked=%d (within tolerance %d)",
					d.baseline.Count, currentSnapshot.Count, len(leaked), d.tolerance)
				return
			}
			
			// Continue waiting for cleanup
			continue
		}
		break
	}
	
	// Final analysis
	leaked := d.analyzeLeak(currentSnapshot)
	if len(leaked) > d.tolerance {
		d.reportLeak(currentSnapshot, leaked)
	} else {
		d.t.Logf("Goroutine cleanup successful after timeout: baseline=%d, current=%d, leaked=%d",
			d.baseline.Count, currentSnapshot.Count, len(leaked))
	}
}

// analyzeLeak analyzes the difference between baseline and current snapshots
func (d *EnhancedGoroutineLeakDetector) analyzeLeak(current GoroutineSnapshot) []LeakedGoroutine {
	d.mu.RLock()
	defer d.mu.RUnlock()
	
	var leaked []LeakedGoroutine
	
	// Compare stack signatures
	for signature, currentCount := range current.Stacks {
		baselineCount := d.baseline.Stacks[signature]
		if currentCount > baselineCount {
			// This stack appears more often than in baseline
			leakCount := currentCount - baselineCount
			
			if d.shouldIncludeLeak(signature) {
				leaked = append(leaked, LeakedGoroutine{
					Signature: signature,
					Count:     leakCount,
					Stack:     d.findFullStack(current.FullStacks, signature),
				})
			}
		}
	}
	
	// Sort by count (most leaked first)
	sort.Slice(leaked, func(i, j int) bool {
		return leaked[i].Count > leaked[j].Count
	})
	
	return leaked
}

// shouldIncludeLeak determines if a stack signature should be considered a leak
func (d *EnhancedGoroutineLeakDetector) shouldIncludeLeak(signature string) bool {
	// Check blacklist first
	if d.stackBlacklist[signature] {
		return false
	}
	
	// If we have include patterns, the signature must match at least one
	if len(d.includePatterns) > 0 {
		included := false
		for _, pattern := range d.includePatterns {
			if strings.Contains(signature, pattern) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}
	
	// Check exclude patterns
	for _, pattern := range d.excludePatterns {
		if strings.Contains(signature, pattern) {
			return false
		}
	}
	
	return true
}

// findFullStack finds the full stack trace for a given signature
func (d *EnhancedGoroutineLeakDetector) findFullStack(stacks []string, signature string) string {
	for _, stack := range stacks {
		if d.extractStackSignature(stack) == signature {
			return stack
		}
	}
	return "Stack trace not found"
}

// reportLeak reports detected goroutine leaks with detailed information
func (d *EnhancedGoroutineLeakDetector) reportLeak(current GoroutineSnapshot, leaked []LeakedGoroutine) {
	d.t.Helper()
	
	totalLeaked := 0
	for _, leak := range leaked {
		totalLeaked += leak.Count
	}
	
	d.t.Errorf("Goroutine leak detected: %d total leaked goroutines (baseline: %d, current: %d)",
		totalLeaked, d.baseline.Count, current.Count)
	
	d.t.Logf("Baseline captured at: %v", d.baseline.Timestamp)
	d.t.Logf("Current snapshot at: %v", current.Timestamp)
	d.t.Logf("Elapsed time: %v", current.Timestamp.Sub(d.baseline.Timestamp))
	
	d.t.Log("\nLeaked goroutines by type:")
	for i, leak := range leaked {
		if i >= 5 { // Limit output to top 5 leak types
			d.t.Logf("... and %d more leak types", len(leaked)-i)
			break
		}
		
		d.t.Logf("\n[%d instances] %s", leak.Count, leak.Signature)
		
		// Show truncated stack trace
		stackLines := strings.Split(leak.Stack, "\n")
		maxLines := 10
		if len(stackLines) > maxLines {
			stackLines = stackLines[:maxLines]
			stackLines = append(stackLines, "... (truncated)")
		}
		
		for _, line := range stackLines {
			if strings.TrimSpace(line) != "" {
				d.t.Logf("  %s", line)
			}
		}
	}
	
	d.t.Log("\nDebugging hints:")
	d.t.Log("- Check for goroutines that don't respect context cancellation")
	d.t.Log("- Verify all background goroutines have proper shutdown mechanisms")
	d.t.Log("- Ensure channels are closed and goroutines have exit conditions")
	d.t.Log("- Look for missing defer statements in cleanup functions")
	
	// Provide specific suggestions based on common patterns
	d.provideSuggestions(leaked)
}

// provideSuggestions provides specific debugging suggestions based on leak patterns
func (d *EnhancedGoroutineLeakDetector) provideSuggestions(leaked []LeakedGoroutine) {
	suggestions := make(map[string]bool)
	
	for _, leak := range leaked {
		stack := leak.Stack
		signature := leak.Signature
		
		if strings.Contains(stack, "time.Sleep") || strings.Contains(signature, "time.Sleep") {
			suggestions["Check for goroutines using time.Sleep without context cancellation"] = true
		}
		
		if strings.Contains(stack, "chan send") || strings.Contains(stack, "chan receive") {
			suggestions["Check for blocked channel operations - ensure channels are closed properly"] = true
		}
		
		if strings.Contains(stack, "select") {
			suggestions["Check select statements - ensure they have context.Done() or shutdown channels"] = true
		}
		
		if strings.Contains(stack, "sync.(*WaitGroup).Wait") {
			suggestions["Check WaitGroup usage - ensure all Add() calls have corresponding Done() calls"] = true
		}
		
		if strings.Contains(stack, "sync.(*Mutex).Lock") || strings.Contains(stack, "sync.(*RWMutex).") {
			suggestions["Check for deadlocks - ensure proper mutex unlock patterns"] = true
		}
		
		if strings.Contains(signature, "http") || strings.Contains(stack, "net/http") {
			suggestions["Check HTTP client usage - ensure connections are closed and contexts are used"] = true
		}
	}
	
	if len(suggestions) > 0 {
		d.t.Log("\nSpecific suggestions based on detected patterns:")
		for suggestion := range suggestions {
			d.t.Logf("- %s", suggestion)
		}
	}
}

// LeakedGoroutine represents information about a leaked goroutine
type LeakedGoroutine struct {
	Signature string // Short signature for identification
	Count     int    // Number of leaked goroutines with this signature
	Stack     string // Full stack trace
}

// BlacklistStack adds a stack signature to the blacklist (won't be reported as leak)
func (d *EnhancedGoroutineLeakDetector) BlacklistStack(signature string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stackBlacklist[signature] = true
}

// GetCurrentSnapshot returns current goroutine snapshot for debugging
func (d *EnhancedGoroutineLeakDetector) GetCurrentSnapshot() GoroutineSnapshot {
	return d.captureSnapshot()
}

// CompareSnapshots compares two snapshots and returns the differences
func (d *EnhancedGoroutineLeakDetector) CompareSnapshots(before, after GoroutineSnapshot) []LeakedGoroutine {
	var leaked []LeakedGoroutine
	
	for signature, afterCount := range after.Stacks {
		beforeCount := before.Stacks[signature]
		if afterCount > beforeCount {
			leaked = append(leaked, LeakedGoroutine{
				Signature: signature,
				Count:     afterCount - beforeCount,
				Stack:     d.findFullStack(after.FullStacks, signature),
			})
		}
	}
	
	return leaked
}

// VerifyNoGoroutineLeaks is a convenience function for simple leak detection
func VerifyNoGoroutineLeaks(t testing.TB, testFunc func()) {
	detector := NewEnhancedGoroutineLeakDetector(t)
	defer detector.Check()
	
	testFunc()
}

// VerifyNoGoroutineLeaksWithOptions runs a test with custom leak detection options
func VerifyNoGoroutineLeaksWithOptions(t testing.TB, options func(*EnhancedGoroutineLeakDetector), testFunc func()) {
	detector := NewEnhancedGoroutineLeakDetector(t)
	if options != nil {
		options(detector)
	}
	defer detector.Check()
	
	testFunc()
}

// Example usage patterns:
//
// Basic usage:
//   func TestSomething(t *testing.T) {
//       VerifyNoGoroutineLeaks(t, func() {
//           // Test code that might leak goroutines
//       })
//   }
//
// Advanced usage:
//   func TestAdvanced(t *testing.T) {
//       detector := NewEnhancedGoroutineLeakDetector(t).
//           WithTolerance(5).
//           WithMaxWaitTime(30*time.Second).
//           WithExcludePatterns("myapp.backgroundWorker")
//       defer detector.Check()
//       
//       // Test code
//   }
//
// Manual snapshot comparison:
//   func TestManual(t *testing.T) {
//       detector := NewEnhancedGoroutineLeakDetector(t)
//       
//       before := detector.GetCurrentSnapshot()
//       // Test operations
//       after := detector.GetCurrentSnapshot()
//       
//       leaked := detector.CompareSnapshots(before, after)
//       if len(leaked) > 0 {
//           t.Errorf("Found %d types of leaked goroutines", len(leaked))
//       }
//   }