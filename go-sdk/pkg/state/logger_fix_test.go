package state

import (
	"io"
	"os"
	"sync"
	"testing"
)

// safeWriter wraps a writer to suppress write errors after tests complete
type safeWriter struct {
	mu     sync.Mutex
	w      io.Writer
	closed bool
}

func (sw *safeWriter) Write(p []byte) (n int, err error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	
	if sw.closed {
		return len(p), nil
	}
	
	n, err = sw.w.Write(p)
	if err != nil {
		// If we get a write error, mark as closed and suppress future writes
		sw.closed = true
		return len(p), nil
	}
	return n, err
}

var (
	stdoutWrapper = &safeWriter{w: os.Stdout}
	stderrWrapper = &safeWriter{w: os.Stderr}
)

// TestMain runs before all tests and ensures proper cleanup
func TestMain(m *testing.M) {
	// Simple approach: just redirect stdout/stderr to devnull during cleanup phase
	// Use the test wrapper
	code := SetupTestMain(m)
	
	// Exit with the test result code
	os.Exit(code)
}