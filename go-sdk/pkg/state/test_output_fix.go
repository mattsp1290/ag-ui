// +build !nooutputfix

package state

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// outputInterceptor intercepts and filters output
type outputInterceptor struct {
	mu            sync.Mutex
	originalOut   *os.File
	originalErr   *os.File
	pipeReader    *os.File
	pipeWriter    *os.File
	errPipeReader *os.File
	errPipeWriter *os.File
	done          chan struct{}
	wg            sync.WaitGroup
}

// newOutputInterceptor creates a new output interceptor
func newOutputInterceptor() *outputInterceptor {
	return &outputInterceptor{
		originalOut: os.Stdout,
		originalErr: os.Stderr,
		done:        make(chan struct{}),
	}
}

// Start begins intercepting output
func (oi *outputInterceptor) Start() error {
	// Create pipes for stdout
	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	oi.pipeReader = pr
	oi.pipeWriter = pw
	
	// Create pipes for stderr
	errPr, errPw, err := os.Pipe()
	if err != nil {
		pr.Close()
		pw.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	oi.errPipeReader = errPr
	oi.errPipeWriter = errPw
	
	// Replace stdout and stderr
	os.Stdout = oi.pipeWriter
	os.Stderr = oi.errPipeWriter
	
	// Start goroutines to process output
	oi.wg.Add(2)
	go oi.processOutput(oi.pipeReader, oi.originalOut)
	go oi.processOutput(oi.errPipeReader, oi.originalErr)
	
	return nil
}

// processOutput reads from the pipe and filters output
func (oi *outputInterceptor) processOutput(reader io.Reader, writer io.Writer) {
	defer oi.wg.Done()
	
	buf := make([]byte, 4096)
	for {
		select {
		case <-oi.done:
			return
		default:
			n, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF && !strings.Contains(err.Error(), "file already closed") {
					// Only return on real errors, not close errors
					return
				}
			}
			if n > 0 {
				output := string(buf[:n])
				// Filter out write error messages
				if !strings.Contains(output, "write error: write /dev/stdout: file already closed") &&
				   !strings.Contains(output, "write error: write /dev/stderr: file already closed") {
					writer.Write(buf[:n])
				}
			}
		}
	}
}

// Stop stops intercepting output and restores original streams
func (oi *outputInterceptor) Stop() {
	oi.mu.Lock()
	defer oi.mu.Unlock()
	
	// Signal goroutines to stop
	close(oi.done)
	
	// Restore original stdout and stderr
	os.Stdout = oi.originalOut
	os.Stderr = oi.originalErr
	
	// Close pipe writers (this will cause readers to get EOF)
	if oi.pipeWriter != nil {
		oi.pipeWriter.Close()
	}
	if oi.errPipeWriter != nil {
		oi.errPipeWriter.Close()
	}
	
	// Wait for goroutines to finish
	oi.wg.Wait()
	
	// Close pipe readers
	if oi.pipeReader != nil {
		oi.pipeReader.Close()
	}
	if oi.errPipeReader != nil {
		oi.errPipeReader.Close()
	}
}

// Global interceptor for tests
var testOutputInterceptor *outputInterceptor

// InitTestOutputInterceptor initializes the output interceptor for tests
func InitTestOutputInterceptor() {
	if os.Getenv("CI") == "true" || os.Getenv("SUPPRESS_WRITE_ERRORS") == "true" {
		testOutputInterceptor = newOutputInterceptor()
		if err := testOutputInterceptor.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to start output interceptor: %v\n", err)
		}
	}
}

// CleanupTestOutputInterceptor cleans up the output interceptor
func CleanupTestOutputInterceptor() {
	if testOutputInterceptor != nil {
		testOutputInterceptor.Stop()
		testOutputInterceptor = nil
	}
}