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

// createSafeFileWrapper creates a file wrapper around a testSafeWriter
func createSafeFileWrapper(writer *testSafeWriter) (*os.File, error) {
	// Create a pipe where the writer end will be wrapped with our safe writer
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	// Start a goroutine that copies from the pipe reader to our safe writer
	go func() {
		io.Copy(writer, r)
		r.Close()
	}()

	return w, nil
}

// SetupTestMain sets up the test environment
func SetupTestMain(m *testing.M) int {
	// Create wrapper
	wrapper := NewTestMainWrapper()
	wrapper.Setup()

	// Run tests
	code := m.Run()

	// Shutdown all test loggers first to stop new log messages
	ShutdownAllTestLoggers()

	// Give generous time for all background goroutines to complete
	time.Sleep(2 * time.Second)

	// Redirect stdout/stderr to devnull to absorb any remaining writes
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err == nil {
		origStdout := os.Stdout
		origStderr := os.Stderr
		os.Stdout = devNull
		os.Stderr = devNull

		// Additional wait while writes go to devnull
		time.Sleep(500 * time.Millisecond)

		// Restore and close
		os.Stdout = origStdout
		os.Stderr = origStderr
		devNull.Close()
	}

	// Cleanup resources
	CleanupTestResources()

	// Final cleanup
	wrapper.Cleanup()

	return code
}
