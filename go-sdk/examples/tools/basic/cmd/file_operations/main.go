package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
)

// FileOperationsExecutor implements safe file operations.
// This example demonstrates file I/O, path validation, error handling,
// and security considerations for file system tools.
type FileOperationsExecutor struct {
	// allowedPaths defines directories where operations are permitted
	allowedPaths []string
	// maxFileSize limits the size of files that can be processed
	maxFileSize int64
}

// NewFileOperationsExecutor creates a new file operations executor with safety constraints.
func NewFileOperationsExecutor(allowedPaths []string, maxFileSize int64) *FileOperationsExecutor {
	return &FileOperationsExecutor{
		allowedPaths: allowedPaths,
		maxFileSize:  maxFileSize,
	}
}

// Execute performs file operations based on the provided parameters.
func (f *FileOperationsExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	// Extract operation type
	operation, ok := params["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation parameter must be a string")
	}

	switch operation {
	case "read":
		return f.readFile(ctx, params)
	case "write":
		return f.writeFile(ctx, params)
	case "list":
		return f.listDirectory(ctx, params)
	case "exists":
		return f.checkExists(ctx, params)
	case "info":
		return f.getFileInfo(ctx, params)
	case "copy":
		return f.copyFile(ctx, params)
	case "delete":
		return f.deleteFile(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// validatePath ensures the path is within allowed directories
func (f *FileOperationsExecutor) validatePath(path string) error {
	// Clean the path to resolve any relative references
	cleanPath := filepath.Clean(path)
	
	// Convert to absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if path is within allowed directories
	for _, allowedPath := range f.allowedPaths {
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

// readFile reads the contents of a file
func (f *FileOperationsExecutor) readFile(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter must be a string")
	}

	if err := f.validatePath(path); err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("path validation failed: %v", err),
		}, nil
	}

	// Check file size before reading
	info, err := os.Stat(path)
	if err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get file info: %v", err),
		}, nil
	}

	if info.Size() > f.maxFileSize {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("file size %d exceeds maximum allowed size %d", info.Size(), f.maxFileSize),
		}, nil
	}

	// Determine read mode (text or binary)
	encoding := "text"
	if encodingParam, exists := params["encoding"]; exists {
		if encodingStr, ok := encodingParam.(string); ok {
			encoding = encodingStr
		}
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read file: %v", err),
		}, nil
	}

	response := map[string]interface{}{
		"path":     path,
		"size":     len(content),
		"encoding": encoding,
		"file_info": map[string]interface{}{
			"mode":    info.Mode().String(),
			"mod_time": info.ModTime().Format(time.RFC3339),
			"is_dir":  info.IsDir(),
		},
	}

	if encoding == "text" {
		response["content"] = string(content)
		response["line_count"] = len(strings.Split(string(content), "\n"))
	} else {
		// For binary files, return base64 or hex representation
		response["content"] = fmt.Sprintf("Binary content (%d bytes)", len(content))
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    response,
	}, nil
}

// writeFile writes content to a file
func (f *FileOperationsExecutor) writeFile(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter must be a string")
	}

	content, ok := params["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content parameter must be a string")
	}

	if err := f.validatePath(path); err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("path validation failed: %v", err),
		}, nil
	}

	// Check content size
	if int64(len(content)) > f.maxFileSize {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("content size %d exceeds maximum allowed size %d", len(content), f.maxFileSize),
		}, nil
	}

	// Determine write mode (create or append)
	mode := "create"
	if modeParam, exists := params["mode"]; exists {
		if modeStr, ok := modeParam.(string); ok {
			mode = modeStr
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create directory: %v", err),
		}, nil
	}

	var err error
	switch mode {
	case "create", "overwrite":
		err = os.WriteFile(path, []byte(content), 0644)
	case "append":
		file, openErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if openErr != nil {
			err = openErr
		} else {
			_, err = file.WriteString(content)
			file.Close()
		}
	default:
		return nil, fmt.Errorf("unsupported write mode: %s", mode)
	}

	if err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to write file: %v", err),
		}, nil
	}

	// Get updated file info
	info, _ := os.Stat(path)
	response := map[string]interface{}{
		"path":         path,
		"bytes_written": len(content),
		"mode":         mode,
		"file_info": map[string]interface{}{
			"size":     info.Size(),
			"mod_time": info.ModTime().Format(time.RFC3339),
		},
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    response,
	}, nil
}

// listDirectory lists the contents of a directory
func (f *FileOperationsExecutor) listDirectory(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter must be a string")
	}

	if err := f.validatePath(path); err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("path validation failed: %v", err),
		}, nil
	}

	// Check if path is a directory
	info, err := os.Stat(path)
	if err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to access path: %v", err),
		}, nil
	}

	if !info.IsDir() {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "path is not a directory",
		}, nil
	}

	// Read directory contents
	entries, err := os.ReadDir(path)
	if err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read directory: %v", err),
		}, nil
	}

	// Process entries
	var files, directories []map[string]interface{}
	totalSize := int64(0)

	for _, entry := range entries {
		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}

		entryData := map[string]interface{}{
			"name":     entry.Name(),
			"size":     entryInfo.Size(),
			"mode":     entryInfo.Mode().String(),
			"mod_time": entryInfo.ModTime().Format(time.RFC3339),
			"is_dir":   entry.IsDir(),
		}

		if entry.IsDir() {
			directories = append(directories, entryData)
		} else {
			files = append(files, entryData)
			totalSize += entryInfo.Size()
		}
	}

	response := map[string]interface{}{
		"path":            path,
		"file_count":      len(files),
		"directory_count": len(directories),
		"total_size":      totalSize,
		"files":           files,
		"directories":     directories,
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    response,
	}, nil
}

// checkExists checks if a file or directory exists
func (f *FileOperationsExecutor) checkExists(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter must be a string")
	}

	if err := f.validatePath(path); err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("path validation failed: %v", err),
		}, nil
	}

	info, err := os.Stat(path)
	exists := err == nil

	response := map[string]interface{}{
		"path":   path,
		"exists": exists,
	}

	if exists {
		response["is_dir"] = info.IsDir()
		response["size"] = info.Size()
		response["mod_time"] = info.ModTime().Format(time.RFC3339)
		response["mode"] = info.Mode().String()
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    response,
	}, nil
}

// getFileInfo retrieves detailed information about a file
func (f *FileOperationsExecutor) getFileInfo(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter must be a string")
	}

	if err := f.validatePath(path); err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("path validation failed: %v", err),
		}, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get file info: %v", err),
		}, nil
	}

	response := map[string]interface{}{
		"path":     path,
		"name":     info.Name(),
		"size":     info.Size(),
		"mode":     info.Mode().String(),
		"mod_time": info.ModTime().Format(time.RFC3339),
		"is_dir":   info.IsDir(),
	}

	// Add system-specific information
	if sysInfo, ok := info.Sys().(*fs.FileInfo); ok && sysInfo != nil {
		response["system_info"] = "Available"
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    response,
	}, nil
}

// copyFile copies a file from source to destination
func (f *FileOperationsExecutor) copyFile(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	source, ok := params["source"].(string)
	if !ok {
		return nil, fmt.Errorf("source parameter must be a string")
	}

	destination, ok := params["destination"].(string)
	if !ok {
		return nil, fmt.Errorf("destination parameter must be a string")
	}

	// Validate both paths
	if err := f.validatePath(source); err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("source path validation failed: %v", err),
		}, nil
	}

	if err := f.validatePath(destination); err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("destination path validation failed: %v", err),
		}, nil
	}

	// Check source file
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to access source file: %v", err),
		}, nil
	}

	if sourceInfo.IsDir() {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "source is a directory, not a file",
		}, nil
	}

	if sourceInfo.Size() > f.maxFileSize {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("source file size %d exceeds maximum allowed size %d", sourceInfo.Size(), f.maxFileSize),
		}, nil
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destination)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create destination directory: %v", err),
		}, nil
	}

	// Copy the file
	sourceFile, err := os.Open(source)
	if err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to open source file: %v", err),
		}, nil
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destination)
	if err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create destination file: %v", err),
		}, nil
	}
	defer destFile.Close()

	bytesCopied, err := io.Copy(destFile, sourceFile)
	if err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to copy file: %v", err),
		}, nil
	}

	response := map[string]interface{}{
		"source":       source,
		"destination":  destination,
		"bytes_copied": bytesCopied,
		"original_size": sourceInfo.Size(),
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    response,
	}, nil
}

// deleteFile deletes a file
func (f *FileOperationsExecutor) deleteFile(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter must be a string")
	}

	if err := f.validatePath(path); err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("path validation failed: %v", err),
		}, nil
	}

	// Get file info before deletion
	info, err := os.Stat(path)
	if err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to access file: %v", err),
		}, nil
	}

	// Confirm it's not a directory (for safety)
	if info.IsDir() {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "cannot delete directory with file delete operation",
		}, nil
	}

	// Delete the file
	if err := os.Remove(path); err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to delete file: %v", err),
		}, nil
	}

	response := map[string]interface{}{
		"path":    path,
		"deleted": true,
		"was_size": info.Size(),
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    response,
	}, nil
}

// CreateFileOperationsTool creates and configures the file operations tool.
func CreateFileOperationsTool() *tools.Tool {
	// Define allowed paths (in a real application, this would be configurable)
	allowedPaths := []string{
		"/tmp",
		"./temp",
		"./data",
	}
	
	// Maximum file size (1MB for this example)
	maxFileSize := int64(1024 * 1024)

	return &tools.Tool{
		ID:          "file_operations",
		Name:        "File Operations Tool",
		Description: "Performs safe file system operations including read, write, list, copy, and delete within restricted directories",
		Version:     "2.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"operation": {
					Type:        "string",
					Description: "The file operation to perform",
					Enum: []interface{}{
						"read", "write", "list", "exists", "info", "copy", "delete",
					},
				},
				"path": {
					Type:        "string",
					Description: "File or directory path",
					MinLength:   &[]int{1}[0],
					MaxLength:   &[]int{500}[0],
				},
				"content": {
					Type:        "string",
					Description: "Content to write (required for write operation)",
				},
				"source": {
					Type:        "string",
					Description: "Source file path (required for copy operation)",
				},
				"destination": {
					Type:        "string",
					Description: "Destination file path (required for copy operation)",
				},
				"mode": {
					Type:        "string",
					Description: "Write mode for file operations",
					Enum: []interface{}{
						"create", "overwrite", "append",
					},
					Default: "create",
				},
				"encoding": {
					Type:        "string",
					Description: "File encoding for read operations",
					Enum: []interface{}{
						"text", "binary",
					},
					Default: "text",
				},
			},
			Required: []string{"operation", "path"},
			// Note: For write operations, content parameter is also required
			// This would be enforced in the Execute method
		},
		Metadata: &tools.ToolMetadata{
			Author:        "AG-UI SDK Examples",
			License:       "MIT",
			Documentation: "https://github.com/mattsp1290/ag-ui/blob/main/go-sdk/examples/tools/basic/README.md",
			Tags:          []string{"file", "filesystem", "io", "security"},
			Examples: []tools.ToolExample{
				{
					Name:        "Read File",
					Description: "Read the contents of a text file",
					Input: map[string]interface{}{
						"operation": "read",
						"path":      "/tmp/example.txt",
					},
				},
				{
					Name:        "Write File",
					Description: "Write content to a new file",
					Input: map[string]interface{}{
						"operation": "write",
						"path":      "/tmp/output.txt",
						"content":   "Hello, World!",
					},
				},
				{
					Name:        "List Directory",
					Description: "List the contents of a directory",
					Input: map[string]interface{}{
						"operation": "list",
						"path":      "/tmp",
					},
				},
			},
		},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  false,
			Async:      false,
			Cancelable: true,
			Retryable:  false, // File operations should not be automatically retried
			Cacheable:  false, // File operations should not be cached
			Timeout:    30 * time.Second,
		},
		Executor: NewFileOperationsExecutor(allowedPaths, maxFileSize),
	}
}

func main() {
	// Create registry and register the file operations tool
	registry := tools.NewRegistry()
	fileOpsTool := CreateFileOperationsTool()

	if err := registry.Register(fileOpsTool); err != nil {
		log.Fatalf("Failed to register file operations tool: %v", err)
	}

	// Create execution engine
	engine := tools.NewExecutionEngine(registry)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := engine.Shutdown(ctx); err != nil {
			log.Printf("Engine shutdown error: %v", err)
		}
	}()

	// Ensure temp directory exists
	if err := os.MkdirAll("./temp", 0755); err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}

	// Example usage
	ctx := context.Background()

	fmt.Println("=== File Operations Tool Example ===")
	fmt.Println("Demonstrates: File I/O, path validation, security constraints, and error handling")
	fmt.Println()

	// Create a test file
	testContent := "Hello, World!\nThis is a test file for the file operations tool.\nLine 3 of the test file."
	
	fmt.Println("1. Writing test file...")
	writeResult, err := engine.Execute(ctx, "file_operations", map[string]interface{}{
		"operation": "write",
		"path":      "./temp/test.txt",
		"content":   testContent,
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if !writeResult.Success {
		fmt.Printf("  Failed: %s\n", writeResult.Error)
	} else {
		fmt.Printf("  Success: %v\n", writeResult.Data)
	}
	fmt.Println()

	// Read the file back
	fmt.Println("2. Reading test file...")
	readResult, err := engine.Execute(ctx, "file_operations", map[string]interface{}{
		"operation": "read",
		"path":      "./temp/test.txt",
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if !readResult.Success {
		fmt.Printf("  Failed: %s\n", readResult.Error)
	} else {
		data := readResult.Data.(map[string]interface{})
		fmt.Printf("  Content: %s\n", data["content"])
		fmt.Printf("  Size: %v bytes\n", data["size"])
		fmt.Printf("  Lines: %v\n", data["line_count"])
	}
	fmt.Println()

	// List directory
	fmt.Println("3. Listing temp directory...")
	listResult, err := engine.Execute(ctx, "file_operations", map[string]interface{}{
		"operation": "list",
		"path":      "./temp",
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if !listResult.Success {
		fmt.Printf("  Failed: %s\n", listResult.Error)
	} else {
		data := listResult.Data.(map[string]interface{})
		fmt.Printf("  File count: %v\n", data["file_count"])
		fmt.Printf("  Files: %v\n", data["files"])
	}
	fmt.Println()

	// Copy file
	fmt.Println("4. Copying test file...")
	copyResult, err := engine.Execute(ctx, "file_operations", map[string]interface{}{
		"operation":   "copy",
		"source":      "./temp/test.txt",
		"destination": "./temp/test_copy.txt",
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else if !copyResult.Success {
		fmt.Printf("  Failed: %s\n", copyResult.Error)
	} else {
		fmt.Printf("  Success: %v\n", copyResult.Data)
	}
	fmt.Println()

	// Demonstrate security constraints
	fmt.Println("=== Security Constraint Examples ===")
	
	securityExamples := []map[string]interface{}{
		{"operation": "read", "path": "/etc/passwd"},        // Outside allowed paths
		{"operation": "read", "path": "../../../etc/passwd"}, // Path traversal attempt
		{"operation": "write", "path": "./temp/test.txt", "content": strings.Repeat("A", 2*1024*1024)}, // Exceeds size limit
	}

	for i, params := range securityExamples {
		fmt.Printf("Security Example %d: %v\n", i+1, params)
		
		result, err := engine.Execute(ctx, "file_operations", params)
		if err != nil {
			fmt.Printf("  Validation Error: %v\n", err)
		} else if !result.Success {
			fmt.Printf("  Security Error: %s\n", result.Error)
		} else {
			fmt.Printf("  Unexpected Success: %v\n", result.Data)
		}
		fmt.Println()
	}

	// Cleanup
	fmt.Println("5. Cleaning up test files...")
	for _, filename := range []string{"./temp/test.txt", "./temp/test_copy.txt"} {
		deleteResult, err := engine.Execute(ctx, "file_operations", map[string]interface{}{
			"operation": "delete",
			"path":      filename,
		})
		if err != nil {
			fmt.Printf("  Error deleting %s: %v\n", filename, err)
		} else if !deleteResult.Success {
			fmt.Printf("  Failed to delete %s: %s\n", filename, deleteResult.Error)
		} else {
			fmt.Printf("  Deleted %s successfully\n", filename)
		}
	}
}