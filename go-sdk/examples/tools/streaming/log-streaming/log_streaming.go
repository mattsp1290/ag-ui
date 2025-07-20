package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ag-ui/go-sdk/pkg/tools"
)

// LogStreamingExecutor implements real-time log file streaming.
// This example demonstrates the StreamingToolExecutor interface,
// real-time data processing, and proper resource cleanup.
type LogStreamingExecutor struct {
	allowedPaths []string
	maxFileSize  int64
}

// NewLogStreamingExecutor creates a new log streaming executor.
func NewLogStreamingExecutor(allowedPaths []string, maxFileSize int64) *LogStreamingExecutor {
	return &LogStreamingExecutor{
		allowedPaths: allowedPaths,
		maxFileSize:  maxFileSize,
	}
}

// Execute implements the regular ToolExecutor interface for non-streaming operations.
func (l *LogStreamingExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	return &tools.ToolExecutionResult{
		Success: false,
		Error:   "this tool only supports streaming execution, use ExecuteStream instead",
	}, nil
}

// ExecuteStream implements the StreamingToolExecutor interface for real-time log streaming.
func (l *LogStreamingExecutor) ExecuteStream(ctx context.Context, params map[string]interface{}) (<-chan *tools.ToolStreamChunk, error) {
	// Extract and validate parameters
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter must be a string")
	}

	if err := l.validatePath(path); err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}

	// Check if file exists and is readable
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to access file: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file")
	}

	if info.Size() > l.maxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum allowed size %d", info.Size(), l.maxFileSize)
	}

	// Extract streaming options
	mode := "tail" // Default mode
	if modeParam, exists := params["mode"]; exists {
		if modeStr, ok := modeParam.(string); ok {
			mode = modeStr
		}
	}

	lines := 10 // Default number of lines for tail mode
	if linesParam, exists := params["lines"]; exists {
		if linesFloat, ok := linesParam.(float64); ok {
			lines = int(linesFloat)
		}
	}

	follow := false // Default: don't follow file changes
	if followParam, exists := params["follow"]; exists {
		if followBool, ok := followParam.(bool); ok {
			follow = followBool
		}
	}

	filter := "" // Optional filter pattern
	if filterParam, exists := params["filter"]; exists {
		if filterStr, ok := filterParam.(string); ok {
			filter = filterStr
		}
	}

	// Create output channel
	outputCh := make(chan *tools.ToolStreamChunk, 100) // Buffer for better performance

	// Start streaming in a goroutine
	go l.streamLogs(ctx, path, mode, lines, follow, filter, outputCh)

	return outputCh, nil
}

// validatePath ensures the path is within allowed directories
func (l *LogStreamingExecutor) validatePath(path string) error {
	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	for _, allowedPath := range l.allowedPaths {
		allowedAbs, err := filepath.Abs(allowedPath)
		if err != nil {
			continue
		}
		
		if strings.HasPrefix(absPath, allowedAbs) {
			return nil
		}
	}

	return fmt.Errorf("path %q is not within allowed directories", path)
}

// streamLogs handles the actual log streaming logic
func (l *LogStreamingExecutor) streamLogs(ctx context.Context, path, mode string, lines int, follow bool, filter string, outputCh chan<- *tools.ToolStreamChunk) {
	defer close(outputCh)

	chunkIndex := 0
	
	// Send initial metadata
	l.sendChunk(outputCh, "metadata", map[string]interface{}{
		"file_path": path,
		"mode":      mode,
		"follow":    follow,
		"filter":    filter,
		"started_at": time.Now().Format(time.RFC3339),
	}, &chunkIndex)

	switch mode {
	case "head":
		l.streamHead(ctx, path, lines, filter, outputCh, &chunkIndex)
	case "tail":
		l.streamTail(ctx, path, lines, follow, filter, outputCh, &chunkIndex)
	case "full":
		l.streamFull(ctx, path, filter, outputCh, &chunkIndex)
	default:
		l.sendErrorChunk(outputCh, fmt.Sprintf("unsupported mode: %s", mode), &chunkIndex)
		return
	}

	// Send completion chunk
	l.sendChunk(outputCh, "complete", map[string]interface{}{
		"total_chunks": chunkIndex,
		"completed_at": time.Now().Format(time.RFC3339),
	}, &chunkIndex)
}

// streamHead streams the first N lines of the file
func (l *LogStreamingExecutor) streamHead(ctx context.Context, path string, lines int, filter string, outputCh chan<- *tools.ToolStreamChunk, chunkIndex *int) {
	file, err := os.Open(path)
	if err != nil {
		l.sendErrorChunk(outputCh, fmt.Sprintf("failed to open file: %v", err), chunkIndex)
		return
	}
	defer file.Close()

	scanner := newLineScanner(file)
	lineCount := 0

	for scanner.Scan() && lineCount < lines {
		select {
		case <-ctx.Done():
			l.sendErrorChunk(outputCh, "streaming cancelled", chunkIndex)
			return
		default:
		}

		line := scanner.Text()
		if filter == "" || strings.Contains(line, filter) {
			l.sendChunk(outputCh, "data", map[string]interface{}{
				"line_number": lineCount + 1,
				"content":     line,
				"timestamp":   time.Now().Format(time.RFC3339),
			}, chunkIndex)
		}
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		l.sendErrorChunk(outputCh, fmt.Sprintf("error reading file: %v", err), chunkIndex)
	}
}

// streamTail streams the last N lines of the file, optionally following changes
func (l *LogStreamingExecutor) streamTail(ctx context.Context, path string, lines int, follow bool, filter string, outputCh chan<- *tools.ToolStreamChunk, chunkIndex *int) {
	// First, read the last N lines
	tailLines, err := l.readTailLines(path, lines)
	if err != nil {
		l.sendErrorChunk(outputCh, fmt.Sprintf("failed to read tail lines: %v", err), chunkIndex)
		return
	}

	// Send the tail lines
	for i, line := range tailLines {
		select {
		case <-ctx.Done():
			l.sendErrorChunk(outputCh, "streaming cancelled", chunkIndex)
			return
		default:
		}

		if filter == "" || strings.Contains(line, filter) {
			l.sendChunk(outputCh, "data", map[string]interface{}{
				"line_number": i + 1,
				"content":     line,
				"timestamp":   time.Now().Format(time.RFC3339),
				"is_historical": true,
			}, chunkIndex)
		}
	}

	// If follow is enabled, continue monitoring the file
	if follow {
		l.followFile(ctx, path, filter, outputCh, chunkIndex)
	}
}

// streamFull streams the entire file
func (l *LogStreamingExecutor) streamFull(ctx context.Context, path string, filter string, outputCh chan<- *tools.ToolStreamChunk, chunkIndex *int) {
	file, err := os.Open(path)
	if err != nil {
		l.sendErrorChunk(outputCh, fmt.Sprintf("failed to open file: %v", err), chunkIndex)
		return
	}
	defer file.Close()

	scanner := newLineScanner(file)
	lineCount := 0

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			l.sendErrorChunk(outputCh, "streaming cancelled", chunkIndex)
			return
		default:
		}

		line := scanner.Text()
		if filter == "" || strings.Contains(line, filter) {
			l.sendChunk(outputCh, "data", map[string]interface{}{
				"line_number": lineCount + 1,
				"content":     line,
				"timestamp":   time.Now().Format(time.RFC3339),
			}, chunkIndex)
		}
		lineCount++

		// Add small delay to prevent overwhelming the consumer
		if lineCount%100 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	if err := scanner.Err(); err != nil {
		l.sendErrorChunk(outputCh, fmt.Sprintf("error reading file: %v", err), chunkIndex)
	}
}

// readTailLines reads the last N lines from a file
func (l *LogStreamingExecutor) readTailLines(path string, lines int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Simple implementation: read all lines and return last N
	// In production, you might want to optimize this for large files
	scanner := newLineScanner(file)
	var allLines []string

	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Return last N lines
	start := len(allLines) - lines
	if start < 0 {
		start = 0
	}

	return allLines[start:], nil
}

// followFile monitors file changes and streams new lines
func (l *LogStreamingExecutor) followFile(ctx context.Context, path string, filter string, outputCh chan<- *tools.ToolStreamChunk, chunkIndex *int) {
	// Get initial file info
	initialInfo, err := os.Stat(path)
	if err != nil {
		l.sendErrorChunk(outputCh, fmt.Sprintf("failed to get file info: %v", err), chunkIndex)
		return
	}

	currentSize := initialInfo.Size()
	ticker := time.NewTicker(500 * time.Millisecond) // Check every 500ms
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check for file changes
			info, err := os.Stat(path)
			if err != nil {
				l.sendErrorChunk(outputCh, fmt.Sprintf("file monitoring error: %v", err), chunkIndex)
				return
			}

			if info.Size() > currentSize {
				// File has grown, read new content
				newLines, err := l.readNewLines(path, currentSize)
				if err != nil {
					l.sendErrorChunk(outputCh, fmt.Sprintf("error reading new lines: %v", err), chunkIndex)
					continue
				}

				for _, line := range newLines {
					if filter == "" || strings.Contains(line, filter) {
						l.sendChunk(outputCh, "data", map[string]interface{}{
							"content":     line,
							"timestamp":   time.Now().Format(time.RFC3339),
							"is_new":      true,
						}, chunkIndex)
					}
				}

				currentSize = info.Size()
			}
		}
	}
}

// readNewLines reads new lines from a file starting from a specific offset
func (l *LogStreamingExecutor) readNewLines(path string, offset int64) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Seek to the offset
	if _, err := file.Seek(offset, 0); err != nil {
		return nil, err
	}

	scanner := newLineScanner(file)
	var lines []string

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, scanner.Err()
}

// sendChunk sends a stream chunk with proper formatting
func (l *LogStreamingExecutor) sendChunk(outputCh chan<- *tools.ToolStreamChunk, chunkType string, data interface{}, chunkIndex *int) {
	chunk := &tools.ToolStreamChunk{
		Type:      chunkType,
		Data:      data,
		Index:     *chunkIndex,
		Timestamp: time.Now(),
	}

	select {
	case outputCh <- chunk:
		*chunkIndex++
	default:
		// Channel is full or closed, drop the chunk
	}
}

// sendErrorChunk sends an error chunk
func (l *LogStreamingExecutor) sendErrorChunk(outputCh chan<- *tools.ToolStreamChunk, errorMsg string, chunkIndex *int) {
	l.sendChunk(outputCh, "error", map[string]interface{}{
		"error":   errorMsg,
		"fatal":   true,
	}, chunkIndex)
}

// Custom line scanner for better performance
type lineScanner struct {
	file   *os.File
	buffer []byte
	err    error
	line   string
}

func newLineScanner(file *os.File) *lineScanner {
	return &lineScanner{
		file:   file,
		buffer: make([]byte, 1024),
	}
}

func (s *lineScanner) Scan() bool {
	// Simple line scanning implementation
	// In production, you might want to use bufio.Scanner or similar
	var lineBuilder strings.Builder
	
	for {
		n, err := s.file.Read(s.buffer[:1])
		if err != nil {
			if err.Error() != "EOF" {
				s.err = err
			}
			if lineBuilder.Len() > 0 {
				s.line = lineBuilder.String()
				return true
			}
			return false
		}

		if n == 0 {
			if lineBuilder.Len() > 0 {
				s.line = lineBuilder.String()
				return true
			}
			return false
		}

		char := s.buffer[0]
		if char == '\n' {
			s.line = lineBuilder.String()
			return true
		}

		if char != '\r' { // Skip carriage returns
			lineBuilder.WriteByte(char)
		}
	}
}

func (s *lineScanner) Text() string {
	return s.line
}

func (s *lineScanner) Err() error {
	return s.err
}

// CreateLogStreamingTool creates and configures the log streaming tool.
func CreateLogStreamingTool() *tools.Tool {
	allowedPaths := []string{
		"/tmp",
		"./logs",
		"./temp",
		"/var/log",
	}
	
	maxFileSize := int64(100 * 1024 * 1024) // 100MB limit

	return &tools.Tool{
		ID:          "log_streaming",
		Name:        "Real-time Log Streaming Tool",
		Description: "Streams log files in real-time with filtering, tail/head modes, and follow capability",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"path": {
					Type:        "string",
					Description: "Path to the log file to stream",
					MinLength:   &[]int{1}[0],
					MaxLength:   &[]int{500}[0],
				},
				"mode": {
					Type:        "string",
					Description: "Streaming mode",
					Enum: []interface{}{
						"head", "tail", "full",
					},
					Default: "tail",
				},
				"lines": {
					Type:        "number",
					Description: "Number of lines for head/tail modes",
					Minimum:     &[]float64{1}[0],
					Maximum:     &[]float64{1000}[0],
					Default:     10,
				},
				"follow": {
					Type:        "boolean",
					Description: "Whether to follow file changes (like tail -f)",
					Default:     false,
				},
				"filter": {
					Type:        "string",
					Description: "Optional filter string to match lines",
					MaxLength:   &[]int{200}[0],
				},
			},
			Required: []string{"path"},
		},
		Metadata: &tools.ToolMetadata{
			Author:        "AG-UI SDK Examples",
			License:       "MIT",
			Documentation: "https://github.com/mattsp1290/ag-ui/blob/main/go-sdk/examples/tools/streaming/README.md",
			Tags:          []string{"streaming", "logs", "real-time", "monitoring"},
			Examples: []tools.ToolExample{
				{
					Name:        "Tail Log File",
					Description: "Stream the last 20 lines of a log file",
					Input: map[string]interface{}{
						"path":  "/tmp/app.log",
						"mode":  "tail",
						"lines": 20,
					},
				},
				{
					Name:        "Follow Log File",
					Description: "Follow a log file for new entries",
					Input: map[string]interface{}{
						"path":   "/tmp/app.log",
						"mode":   "tail",
						"follow": true,
					},
				},
				{
					Name:        "Filter Error Logs",
					Description: "Stream only lines containing 'ERROR'",
					Input: map[string]interface{}{
						"path":   "/tmp/app.log",
						"mode":   "full",
						"filter": "ERROR",
					},
				},
			},
		},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  true,  // This tool supports streaming
			Async:      false,
			Cancelable: true,  // Streaming can be cancelled
			Retryable:  false,
			Cacheable:  false, // Streaming results should not be cached
			Timeout:    5 * time.Minute, // Longer timeout for streaming
		},
		Executor: NewLogStreamingExecutor(allowedPaths, maxFileSize),
	}
}

