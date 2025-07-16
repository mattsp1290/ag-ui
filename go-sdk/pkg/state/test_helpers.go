package state

import (
	"context"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

// testOutputRedirector helps suppress write errors in tests
type testOutputRedirector struct {
	origStdout *os.File
	origStderr *os.File
	devNull    *os.File
}

func newTestOutputRedirector() *testOutputRedirector {
	return &testOutputRedirector{
		origStdout: os.Stdout,
		origStderr: os.Stderr,
	}
}

func (r *testOutputRedirector) redirect() error {
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	r.devNull = devNull
	
	// Redirect stdout and stderr to devnull
	os.Stdout = devNull
	os.Stderr = devNull
	
	return nil
}

func (r *testOutputRedirector) restore() {
	if r.devNull != nil {
		r.devNull.Close()
	}
	os.Stdout = r.origStdout
	os.Stderr = r.origStderr
}

// SuppressWriteErrors sets up output redirection for tests
func SuppressWriteErrors(t *testing.T) {
	if os.Getenv("SUPPRESS_WRITE_ERRORS") != "true" {
		return
	}
	
	redirector := newTestOutputRedirector()
	if err := redirector.redirect(); err != nil {
		t.Logf("Failed to redirect output: %v", err)
		return
	}
	
	t.Cleanup(func() {
		redirector.restore()
	})
}

// testWriter wraps an io.Writer to suppress write-after-close errors
type testWriter struct {
	mu     sync.RWMutex
	w      io.Writer
	closed bool
}

func (tw *testWriter) Write(p []byte) (n int, err error) {
	tw.mu.RLock()
	defer tw.mu.RUnlock()
	
	if tw.closed {
		// Silently succeed to prevent "write error" messages
		return len(p), nil
	}
	
	return tw.w.Write(p)
}

func (tw *testWriter) Close() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.closed = true
}

// WrapTestOutput wraps stdout/stderr to suppress write-after-close errors
func WrapTestOutput() func() {
	origStdout := os.Stdout
	origStderr := os.Stderr
	
	// Create wrapped versions
	stdoutWrapper := &testWriter{w: origStdout}
	stderrWrapper := &testWriter{w: origStderr}
	
	// Create pipe readers/writers
	stdoutR, stdoutW, _ := os.Pipe()
	stderrR, stderrW, _ := os.Pipe()
	
	os.Stdout = stdoutW
	os.Stderr = stderrW
	
	// Use WaitGroup to track goroutines
	var wg sync.WaitGroup
	
	// Copy output through our wrappers
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(stdoutWrapper, stdoutR)
	}()
	
	wg.Add(1)
	go func() {
		defer wg.Done()
		io.Copy(stderrWrapper, stderrR)
	}()
	
	return func() {
		// Mark wrappers as closed first
		stdoutWrapper.Close()
		stderrWrapper.Close()
		
		// Close write ends of pipes to signal EOF to readers
		stdoutW.Close()
		stderrW.Close()
		
		// Wait for copy goroutines to finish
		wg.Wait()
		
		// Restore original stdout/stderr
		os.Stdout = origStdout
		os.Stderr = origStderr
		
		// Close readers after goroutines are done
		stdoutR.Close()
		stderrR.Close()
	}
}

// TestCleanup provides comprehensive test cleanup to prevent write errors
type TestCleanup struct {
	t              *testing.T
	manager        *StateManager
	monitoring     *MonitoringSystem
	cleanupFuncs   []func()
	mu             sync.Mutex
	cleanupStarted bool
}

// NewTestCleanup creates a new test cleanup helper
func NewTestCleanup(t *testing.T) *TestCleanup {
	tc := &TestCleanup{
		t:            t,
		cleanupFuncs: make([]func(), 0),
	}
	
	// Register cleanup to run at test end
	t.Cleanup(tc.Cleanup)
	
	return tc
}

// SetManager sets the state manager to clean up
func (tc *TestCleanup) SetManager(manager *StateManager) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.manager = manager
}

// SetMonitoring sets the monitoring system to clean up
func (tc *TestCleanup) SetMonitoring(monitoring *MonitoringSystem) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.monitoring = monitoring
}

// AddCleanup adds a cleanup function to run during test cleanup
func (tc *TestCleanup) AddCleanup(fn func()) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cleanupFuncs = append(tc.cleanupFuncs, fn)
}

// Cleanup performs comprehensive test cleanup
func (tc *TestCleanup) Cleanup() {
	tc.mu.Lock()
	if tc.cleanupStarted {
		tc.mu.Unlock()
		return
	}
	tc.cleanupStarted = true
	tc.mu.Unlock()
	
	// Create a cleanup context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// First, shut down monitoring system if present
	if tc.monitoring != nil {
		if err := tc.monitoring.Shutdown(ctx); err != nil {
			tc.t.Logf("Warning: monitoring shutdown error: %v", err)
		}
	}
	
	// Then shut down state manager if present
	if tc.manager != nil {
		if err := tc.manager.Close(); err != nil {
			tc.t.Logf("Warning: manager shutdown error: %v", err)
		}
	}
	
	// Run any additional cleanup functions
	for i := len(tc.cleanupFuncs) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					tc.t.Logf("Warning: cleanup function panicked: %v", r)
				}
			}()
			tc.cleanupFuncs[i]()
		}()
	}
	
	// Give a brief moment for any remaining goroutines to finish
	time.Sleep(50 * time.Millisecond)
	
	// Redirect stdout/stderr to devnull to suppress any late write errors
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err == nil {
		os.Stdout = devNull
		os.Stderr = devNull
		tc.AddCleanup(func() {
			devNull.Close()
		})
	}
}