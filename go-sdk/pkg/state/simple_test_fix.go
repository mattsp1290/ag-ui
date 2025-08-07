package state

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// simpleErrorFilter wraps stderr to filter write errors
type simpleErrorFilter struct {
	mu sync.Mutex
}

var errorFilter = &simpleErrorFilter{}
var originalStderr = os.Stderr

func (f *simpleErrorFilter) Write(p []byte) (n int, err error) {
	content := string(p)

	// Filter out write error messages
	if strings.Contains(content, "write error: write") &&
		strings.Contains(content, "file already closed") {
		// Return success without writing
		return len(p), nil
	}

	// Write to original stderr
	return originalStderr.Write(p)
}

// SetupErrorFiltering sets up stderr filtering
func SetupErrorFiltering() {
	if os.Getenv("SUPPRESS_WRITE_ERRORS") == "true" || os.Getenv("CI") == "true" {
		// Create a pipe
		r, w, _ := os.Pipe()

		// Replace stderr with the write end
		os.Stderr = w

		// Start filtering goroutine
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := r.Read(buf)
				if err == io.EOF {
					break
				}
				if n > 0 {
					errorFilter.Write(buf[:n])
				}
			}
		}()
	}
}

// init sets up filtering on package initialization
func init() {
	// Check if we should suppress errors
	if os.Getenv("SUPPRESS_WRITE_ERRORS") == "true" || os.Getenv("CI") == "true" {
		// Wrap stderr with our filter
		fmt.Fprintln(originalStderr, "Note: Write errors will be suppressed in test output")
	}
}
