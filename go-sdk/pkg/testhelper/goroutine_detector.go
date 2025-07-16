package testhelper

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// GoroutineLeakDetector helps detect goroutine leaks in tests
type GoroutineLeakDetector struct {
	initialGoroutines map[string]int
	t                 *testing.T
	threshold         int
	checkDelay        time.Duration
}

// NewGoroutineLeakDetector creates a new leak detector
func NewGoroutineLeakDetector(t *testing.T) *GoroutineLeakDetector {
	return &GoroutineLeakDetector{
		t:          t,
		threshold:  5, // Allow up to 5 extra goroutines
		checkDelay: 100 * time.Millisecond,
	}
}

// WithThreshold sets the allowed goroutine increase threshold
func (d *GoroutineLeakDetector) WithThreshold(threshold int) *GoroutineLeakDetector {
	d.threshold = threshold
	return d
}

// WithCheckDelay sets the delay before checking for leaks (to allow cleanup)
func (d *GoroutineLeakDetector) WithCheckDelay(delay time.Duration) *GoroutineLeakDetector {
	d.checkDelay = delay
	return d
}

// Start captures the initial goroutine state
func (d *GoroutineLeakDetector) Start() {
	d.initialGoroutines = getGoroutineStacks()
	d.t.Logf("Test started with %d goroutines", countGoroutines(d.initialGoroutines))
}

// Check verifies no goroutine leaks occurred
func (d *GoroutineLeakDetector) Check() {
	// Give goroutines time to clean up
	time.Sleep(d.checkDelay)

	finalGoroutines := getGoroutineStacks()
	leaked := compareGoroutines(d.initialGoroutines, finalGoroutines)

	if len(leaked) > d.threshold {
		d.t.Helper()
		d.reportLeak(leaked, finalGoroutines)
	} else if len(leaked) > 0 {
		d.t.Logf("Minor goroutine increase: %d new goroutines (threshold: %d)", len(leaked), d.threshold)
	}
}

// VerifyNoGoroutineLeaks is a convenience function that checks for leaks at test end
func VerifyNoGoroutineLeaks(t *testing.T) {
	t.Helper()
	detector := NewGoroutineLeakDetector(t)
	detector.Start()
	
	t.Cleanup(func() {
		detector.Check()
	})
}

// getGoroutineStacks returns a map of goroutine stacks to their count
func getGoroutineStacks() map[string]int {
	buf := make([]byte, 2<<20) // 2MB buffer
	n := runtime.Stack(buf, true)
	stacks := make(map[string]int)

	// Parse stack traces
	traces := strings.Split(string(buf[:n]), "\n\n")
	for _, trace := range traces {
		if trace == "" {
			continue
		}
		
		// Extract meaningful stack info (first few lines)
		lines := strings.Split(trace, "\n")
		if len(lines) > 0 {
			// Use first 3 lines as signature (goroutine header + function + location)
			signature := strings.Join(lines[0:min(3, len(lines))], "\n")
			stacks[signature]++
		}
	}

	return stacks
}

// countGoroutines returns the total number of goroutines
func countGoroutines(stacks map[string]int) int {
	count := 0
	for _, c := range stacks {
		count += c
	}
	return count
}

// compareGoroutines returns new goroutines that weren't in the initial set
func compareGoroutines(initial, final map[string]int) []string {
	var leaked []string

	for stack, count := range final {
		initialCount := initial[stack]
		if count > initialCount {
			diff := count - initialCount
			for i := 0; i < diff; i++ {
				leaked = append(leaked, stack)
			}
		}
	}

	// Sort for consistent output
	sort.Strings(leaked)
	return leaked
}

// reportLeak reports detected goroutine leaks
func (d *GoroutineLeakDetector) reportLeak(leaked []string, final map[string]int) {
	d.t.Helper()
	
	initialCount := countGoroutines(d.initialGoroutines)
	finalCount := countGoroutines(final)
	
	d.t.Errorf("Goroutine leak detected: %d -> %d goroutines (+%d)", 
		initialCount, finalCount, len(leaked))
	
	// Group similar stacks
	stackCounts := make(map[string]int)
	for _, stack := range leaked {
		stackCounts[stack]++
	}
	
	d.t.Log("Leaked goroutines:")
	for stack, count := range stackCounts {
		d.t.Logf("\n[%d instances]\n%s", count, stack)
	}
	
	// Provide debugging hints
	d.t.Log("\nCommon causes of goroutine leaks:")
	d.t.Log("- Unclosed channels causing goroutines to block")
	d.t.Log("- Missing context cancellation")
	d.t.Log("- Infinite loops without exit conditions")
	d.t.Log("- Unclosed network connections or listeners")
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MonitorGoroutines logs goroutine counts periodically during test execution
func MonitorGoroutines(t *testing.T, interval time.Duration) func() {
	t.Helper()
	
	done := make(chan struct{})
	
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				count := runtime.NumGoroutine()
				t.Logf("[Goroutine Monitor] Current count: %d", count)
			case <-done:
				return
			}
		}
	}()
	
	return func() {
		close(done)
	}
}

// WaitForGoroutines waits for goroutine count to stabilize
func WaitForGoroutines(t *testing.T, expectedDelta int, timeout time.Duration) {
	t.Helper()
	
	initialCount := runtime.NumGoroutine()
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		currentCount := runtime.NumGoroutine()
		if currentCount <= initialCount+expectedDelta {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	
	t.Errorf("Goroutine count did not stabilize: started with %d, have %d (expected delta: %d)",
		initialCount, runtime.NumGoroutine(), expectedDelta)
}

// DetectBlockedGoroutines checks for goroutines that might be blocked
func DetectBlockedGoroutines(t *testing.T) {
	t.Helper()
	
	buf := make([]byte, 2<<20)
	n := runtime.Stack(buf, true)
	stack := string(buf[:n])
	
	// Look for common blocking patterns
	blockingPatterns := []string{
		"chan send",
		"chan receive",
		"select",
		"sync.(*Mutex).Lock",
		"sync.(*RWMutex).RLock",
		"sync.(*WaitGroup).Wait",
	}
	
	var blocked []string
	traces := strings.Split(stack, "\n\n")
	
	for _, trace := range traces {
		for _, pattern := range blockingPatterns {
			if strings.Contains(trace, pattern) {
				blocked = append(blocked, fmt.Sprintf("Potentially blocked on %s:\n%s", pattern, trace))
			}
		}
	}
	
	if len(blocked) > 0 {
		t.Log("Potentially blocked goroutines detected:")
		for _, b := range blocked {
			t.Log(b)
		}
	}
}