// +build !race

package state

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestMainWrapper provides a clean way to handle test output
type TestMainWrapper struct {
	origStdout *os.File
	origStderr *os.File
	origLogOut io.Writer
	buffer     *bytes.Buffer
	mu         sync.Mutex
	done       chan struct{}
}

// NewTestMainWrapper creates a new test wrapper
func NewTestMainWrapper() *TestMainWrapper {
	return &TestMainWrapper{
		origStdout: os.Stdout,
		origStderr: os.Stderr,
		origLogOut: log.Writer(),
		buffer:     &bytes.Buffer{},
		done:       make(chan struct{}),
	}
}

// Setup redirects output
func (w *TestMainWrapper) Setup() {
	// Create a custom writer that filters output
	filterWriter := &filteringWriter{
		underlying: w.origStderr,
		buffer:     w.buffer,
		mu:         &w.mu,
	}
	
	// Redirect log output
	log.SetOutput(filterWriter)
	
	// For CI environments or when requested, also filter stdout/stderr
	if os.Getenv("CI") == "true" || os.Getenv("SUPPRESS_WRITE_ERRORS") == "true" {
		// Create null files that silently accept writes
		nullOut, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		nullErr, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		
		// Replace stdout/stderr
		os.Stdout = nullOut
		os.Stderr = nullErr
		
		// Schedule cleanup
		go func() {
			<-w.done
			nullOut.Close()
			nullErr.Close()
		}()
	}
}

// Cleanup restores original output and prints filtered content
func (w *TestMainWrapper) Cleanup() {
	close(w.done)
	
	// Restore original outputs
	os.Stdout = w.origStdout
	os.Stderr = w.origStderr
	log.SetOutput(w.origLogOut)
	
	// Print any buffered content that wasn't filtered
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if w.buffer.Len() > 0 {
		fmt.Fprint(w.origStderr, w.buffer.String())
	}
}

// filteringWriter filters out write error messages
type filteringWriter struct {
	underlying io.Writer
	buffer     *bytes.Buffer
	mu         *sync.Mutex
}

func (f *filteringWriter) Write(p []byte) (n int, err error) {
	content := string(p)
	
	// Filter out write error messages
	if strings.Contains(content, "write error: write /dev/stdout: file already closed") ||
	   strings.Contains(content, "write error: write /dev/stderr: file already closed") ||
	   strings.Contains(content, "write error: write |1: file already closed") {
		// Silently drop these messages
		return len(p), nil
	}
	
	// Pass through other content
	f.mu.Lock()
	defer f.mu.Unlock()
	
	return f.underlying.Write(p)
}

// SetupTestMain sets up the test environment
func SetupTestMain(m *testing.M) int {
	// Create wrapper
	wrapper := NewTestMainWrapper()
	wrapper.Setup()
	
	// Run tests
	code := m.Run()
	
	// Wait for background goroutines to finish
	time.Sleep(300 * time.Millisecond)
	
	// Cleanup resources
	CleanupTestResources()
	
	// Final cleanup
	wrapper.Cleanup()
	
	return code
}