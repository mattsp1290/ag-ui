package fileoperations_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ag-ui/go-sdk/pkg/tools"
)

// MockFileOperationsExecutor provides a testable version of the file operations tool
type MockFileOperationsExecutor struct {
	// Mock file system for testing
	files map[string][]byte
	directories map[string]bool
}

func NewMockFileOperationsExecutor() *MockFileOperationsExecutor {
	return &MockFileOperationsExecutor{
		files:       make(map[string][]byte),
		directories: make(map[string]bool),
	}
}

func (f *MockFileOperationsExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	operation, ok := params["operation"].(string)
	if !ok {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "operation parameter is required",
		}, nil
	}

	filePath, ok := params["file_path"].(string)
	if !ok {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "file_path parameter is required",
		}, nil
	}

	// Validate file path for security
	if err := f.validatePath(filePath); err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	switch operation {
	case "read":
		return f.readFile(ctx, filePath, params)
	case "write":
		return f.writeFile(ctx, filePath, params)
	case "list":
		return f.listDirectory(ctx, filePath, params)
	case "copy":
		return f.copyFile(ctx, filePath, params)
	case "delete":
		return f.deleteFile(ctx, filePath, params)
	case "stat":
		return f.statFile(ctx, filePath, params)
	case "mkdir":
		return f.createDirectory(ctx, filePath, params)
	default:
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "unsupported operation: " + operation,
		}, nil
	}
}

func (f *MockFileOperationsExecutor) validatePath(path string) error {
	// Security validations
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal not allowed")
	}
	
	if strings.HasPrefix(path, "/etc") || strings.HasPrefix(path, "/sys") {
		return fmt.Errorf("access to system directories not allowed")
	}
	
	if len(path) > 1000 {
		return fmt.Errorf("path too long")
	}
	
	return nil
}

func (f *MockFileOperationsExecutor) readFile(ctx context.Context, filePath string, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	data, exists := f.files[filePath]
	if !exists {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "file not found",
		}, nil
	}

	// Check size limits
	maxSize := int64(1024 * 1024) // 1MB default
	if size, ok := params["max_size"].(int64); ok {
		maxSize = size
	}

	if int64(len(data)) > maxSize {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("file size exceeds limit of %d bytes", maxSize),
		}, nil
	}

	// Handle encoding
	encoding := "utf-8"
	if enc, ok := params["encoding"].(string); ok {
		encoding = enc
	}

	result := map[string]interface{}{
		"content":  string(data),
		"size":     len(data),
		"encoding": encoding,
		"path":     filePath,
	}

	return &tools.ToolExecutionResult{
		Success:   true,
		Data:      result,
		Timestamp: time.Now(),
		Duration:  time.Millisecond * 10,
		Metadata: map[string]interface{}{
			"operation": "read",
			"file_path": filePath,
			"file_size": len(data),
		},
	}, nil
}

func (f *MockFileOperationsExecutor) writeFile(ctx context.Context, filePath string, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	content, ok := params["content"].(string)
	if !ok {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "content parameter is required for write operation",
		}, nil
	}

	// Check size limits
	maxSize := int64(1024 * 1024) // 1MB default
	if size, ok := params["max_size"].(int64); ok {
		maxSize = size
	}

	if int64(len(content)) > maxSize {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("content size exceeds limit of %d bytes", maxSize),
		}, nil
	}

	// Check if file exists and handle backup
	backup := false
	if b, ok := params["backup"].(bool); ok {
		backup = b
	}

	if backup && f.fileExists(filePath) {
		backupPath := filePath + ".backup"
		f.files[backupPath] = f.files[filePath]
	}

	// Write file
	f.files[filePath] = []byte(content)

	result := map[string]interface{}{
		"bytes_written": len(content),
		"path":          filePath,
		"backup_created": backup && f.fileExists(filePath+".backup"),
	}

	return &tools.ToolExecutionResult{
		Success:   true,
		Data:      result,
		Timestamp: time.Now(),
		Duration:  time.Millisecond * 20,
		Metadata: map[string]interface{}{
			"operation":     "write",
			"file_path":     filePath,
			"bytes_written": len(content),
		},
	}, nil
}

func (f *MockFileOperationsExecutor) listDirectory(ctx context.Context, dirPath string, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	// Mock directory listing
	var files []map[string]interface{}
	
	// Find files in this directory
	for path, data := range f.files {
		if strings.HasPrefix(path, dirPath+"/") && !strings.Contains(strings.TrimPrefix(path, dirPath+"/"), "/") {
			files = append(files, map[string]interface{}{
				"name":     filepath.Base(path),
				"path":     path,
				"size":     len(data),
				"type":     "file",
				"modified": time.Now().Add(-time.Hour).Format(time.RFC3339),
			})
		}
	}

	// Find subdirectories
	for dir := range f.directories {
		if strings.HasPrefix(dir, dirPath+"/") && !strings.Contains(strings.TrimPrefix(dir, dirPath+"/"), "/") {
			files = append(files, map[string]interface{}{
				"name":     filepath.Base(dir),
				"path":     dir,
				"size":     0,
				"type":     "directory",
				"modified": time.Now().Add(-time.Hour).Format(time.RFC3339),
			})
		}
	}

	// Apply filtering
	if pattern, ok := params["pattern"].(string); ok && pattern != "" {
		filtered := []map[string]interface{}{}
		for _, file := range files {
			name := file["name"].(string)
			if matched, _ := filepath.Match(pattern, name); matched {
				filtered = append(filtered, file)
			}
		}
		files = filtered
	}

	result := map[string]interface{}{
		"files": files,
		"path":  dirPath,
		"count": len(files),
	}

	return &tools.ToolExecutionResult{
		Success:   true,
		Data:      result,
		Timestamp: time.Now(),
		Duration:  time.Millisecond * 5,
		Metadata: map[string]interface{}{
			"operation":  "list",
			"directory":  dirPath,
			"file_count": len(files),
		},
	}, nil
}

func (f *MockFileOperationsExecutor) copyFile(ctx context.Context, sourcePath string, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	destPath, ok := params["destination"].(string)
	if !ok {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "destination parameter is required for copy operation",
		}, nil
	}

	if err := f.validatePath(destPath); err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "invalid destination path: " + err.Error(),
		}, nil
	}

	data, exists := f.files[sourcePath]
	if !exists {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "source file not found",
		}, nil
	}

	// Check if destination exists
	overwrite := false
	if o, ok := params["overwrite"].(bool); ok {
		overwrite = o
	}

	if f.fileExists(destPath) && !overwrite {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "destination file exists and overwrite not allowed",
		}, nil
	}

	// Copy file
	f.files[destPath] = make([]byte, len(data))
	copy(f.files[destPath], data)

	result := map[string]interface{}{
		"source":      sourcePath,
		"destination": destPath,
		"bytes_copied": len(data),
	}

	return &tools.ToolExecutionResult{
		Success:   true,
		Data:      result,
		Timestamp: time.Now(),
		Duration:  time.Millisecond * 15,
		Metadata: map[string]interface{}{
			"operation":    "copy",
			"source_path":  sourcePath,
			"dest_path":    destPath,
			"bytes_copied": len(data),
		},
	}, nil
}

func (f *MockFileOperationsExecutor) deleteFile(ctx context.Context, filePath string, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	if !f.fileExists(filePath) {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "file not found",
		}, nil
	}

	// Check if backup requested
	backup := false
	if b, ok := params["backup"].(bool); ok {
		backup = b
	}

	size := len(f.files[filePath])

	if backup {
		backupPath := filePath + ".deleted"
		f.files[backupPath] = f.files[filePath]
	}

	delete(f.files, filePath)

	result := map[string]interface{}{
		"path":           filePath,
		"bytes_deleted":  size,
		"backup_created": backup,
	}

	return &tools.ToolExecutionResult{
		Success:   true,
		Data:      result,
		Timestamp: time.Now(),
		Duration:  time.Millisecond * 5,
		Metadata: map[string]interface{}{
			"operation":     "delete",
			"file_path":     filePath,
			"bytes_deleted": size,
		},
	}, nil
}

func (f *MockFileOperationsExecutor) statFile(ctx context.Context, filePath string, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	if !f.fileExists(filePath) {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "file not found",
		}, nil
	}

	data := f.files[filePath]
	result := map[string]interface{}{
		"path":     filePath,
		"size":     len(data),
		"type":     "file",
		"exists":   true,
		"readable": true,
		"writable": true,
		"modified": time.Now().Add(-time.Hour).Format(time.RFC3339),
		"created":  time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
	}

	return &tools.ToolExecutionResult{
		Success:   true,
		Data:      result,
		Timestamp: time.Now(),
		Duration:  time.Millisecond * 2,
		Metadata: map[string]interface{}{
			"operation": "stat",
			"file_path": filePath,
			"file_size": len(data),
		},
	}, nil
}

func (f *MockFileOperationsExecutor) createDirectory(ctx context.Context, dirPath string, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	if f.directories[dirPath] {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "directory already exists",
		}, nil
	}

	// Create parent directories if requested
	recursive := false
	if r, ok := params["recursive"].(bool); ok {
		recursive = r
	}

	if recursive {
		parent := filepath.Dir(dirPath)
		if parent != "." && !f.directories[parent] {
			f.directories[parent] = true
		}
	}

	f.directories[dirPath] = true

	result := map[string]interface{}{
		"path":      dirPath,
		"created":   true,
		"recursive": recursive,
	}

	return &tools.ToolExecutionResult{
		Success:   true,
		Data:      result,
		Timestamp: time.Now(),
		Duration:  time.Millisecond * 3,
		Metadata: map[string]interface{}{
			"operation": "mkdir",
			"directory": dirPath,
		},
	}, nil
}

func (f *MockFileOperationsExecutor) fileExists(path string) bool {
	_, exists := f.files[path]
	return exists
}

func createFileOperationsTool() *tools.Tool {
	return &tools.Tool{
		ID:          "file-operations",
		Name:        "FileOperations",
		Description: "Safe file operations with security constraints and validation",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"operation": {
					Type:        "string",
					Description: "File operation to perform",
					Enum:        []interface{}{"read", "write", "list", "copy", "delete", "stat", "mkdir"},
				},
				"file_path": {
					Type:        "string",
					Description: "Path to the file or directory",
					Pattern:     `^[a-zA-Z0-9_/.-]+$`,
				},
				"content": {
					Type:        "string",
					Description: "Content to write (for write operation)",
				},
				"destination": {
					Type:        "string",
					Description: "Destination path (for copy operation)",
				},
				"encoding": {
					Type:        "string",
					Description: "File encoding",
					Enum:        []interface{}{"utf-8", "ascii", "latin-1"},
					Default:     "utf-8",
				},
				"max_size": {
					Type:        "integer",
					Description: "Maximum file size in bytes",
					Default:     1048576, // 1MB
				},
				"backup": {
					Type:        "boolean",
					Description: "Create backup before operation",
					Default:     false,
				},
				"overwrite": {
					Type:        "boolean",
					Description: "Allow overwriting existing files",
					Default:     false,
				},
				"recursive": {
					Type:        "boolean",
					Description: "Recursive operation for directories",
					Default:     false,
				},
				"pattern": {
					Type:        "string",
					Description: "Pattern for filtering files (for list operation)",
				},
			},
			Required: []string{"operation", "file_path"},
		},
		Executor: NewMockFileOperationsExecutor(),
		Capabilities: &tools.ToolCapabilities{
			Streaming:  false,
			Async:      false,
			Cancelable: true,
			Cacheable:  false, // File operations shouldn't be cached
			Timeout:    30 * time.Second,
		},
		Metadata: &tools.ToolMetadata{
			Author:   "Security Team",
			License:  "MIT",
			Tags:     []string{"file", "io", "security", "operations"},
			Examples: []tools.ToolExample{
				{
					Name:        "Read File",
					Description: "Read content from a file",
					Input: map[string]interface{}{
						"operation":  "read",
						"file_path":  "/data/sample.txt",
					},
				},
				{
					Name:        "Write File with Backup",
					Description: "Write content to file with backup",
					Input: map[string]interface{}{
						"operation":  "write",
						"file_path":  "/data/output.txt",
						"content":    "Hello, World!",
						"backup":     true,
					},
				},
			},
		},
	}
}

// Setup and teardown helpers
func setupMockFileSystem(executor *MockFileOperationsExecutor) {
	// Create some mock files and directories
	executor.files["/data/sample.txt"] = []byte("Hello, World!")
	executor.files["/data/config.json"] = []byte(`{"key": "value"}`)
	executor.files["/data/large.txt"] = []byte(strings.Repeat("A", 2000))
	executor.directories["/data"] = true
	executor.directories["/temp"] = true
}

// TestFileOperationsTool_ReadOperations tests file reading functionality
func TestFileOperationsTool_ReadOperations(t *testing.T) {
	tool := createFileOperationsTool()
	executor := tool.Executor.(*MockFileOperationsExecutor)
	setupMockFileSystem(executor)
	ctx := context.Background()

	testCases := []struct {
		name            string
		params          map[string]interface{}
		expectedSuccess bool
		expectedError   string
	}{
		{
			name: "Read existing file",
			params: map[string]interface{}{
				"operation":  "read",
				"file_path":  "/data/sample.txt",
			},
			expectedSuccess: true,
		},
		{
			name: "Read non-existent file",
			params: map[string]interface{}{
				"operation":  "read",
				"file_path":  "/data/nonexistent.txt",
			},
			expectedSuccess: false,
			expectedError:   "file not found",
		},
		{
			name: "Read with size limit exceeded",
			params: map[string]interface{}{
				"operation":  "read",
				"file_path":  "/data/large.txt",
				"max_size":   int64(100),
			},
			expectedSuccess: false,
			expectedError:   "file size exceeds limit",
		},
		{
			name: "Read with custom encoding",
			params: map[string]interface{}{
				"operation":  "read",
				"file_path":  "/data/sample.txt",
				"encoding":   "ascii",
			},
			expectedSuccess: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Executor.Execute(ctx, tc.params)
			
			require.NoError(t, err)
			require.NotNil(t, result)
			
			if tc.expectedSuccess {
				assert.True(t, result.Success)
				
				// Check result structure
				data, ok := result.Data.(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, data, "content")
				assert.Contains(t, data, "size")
				assert.Contains(t, data, "path")
			} else {
				assert.False(t, result.Success)
				assert.Contains(t, result.Error, tc.expectedError)
			}
		})
	}
}

// TestFileOperationsTool_WriteOperations tests file writing functionality
func TestFileOperationsTool_WriteOperations(t *testing.T) {
	tool := createFileOperationsTool()
	executor := tool.Executor.(*MockFileOperationsExecutor)
	setupMockFileSystem(executor)
	ctx := context.Background()

	testCases := []struct {
		name            string
		params          map[string]interface{}
		expectedSuccess bool
		expectedError   string
	}{
		{
			name: "Write new file",
			params: map[string]interface{}{
				"operation":  "write",
				"file_path":  "/data/new.txt",
				"content":    "New content",
			},
			expectedSuccess: true,
		},
		{
			name: "Write with backup",
			params: map[string]interface{}{
				"operation":  "write",
				"file_path":  "/data/sample.txt",
				"content":    "Updated content",
				"backup":     true,
			},
			expectedSuccess: true,
		},
		{
			name: "Write without content",
			params: map[string]interface{}{
				"operation":  "write",
				"file_path":  "/data/empty.txt",
			},
			expectedSuccess: false,
			expectedError:   "content parameter is required",
		},
		{
			name: "Write content too large",
			params: map[string]interface{}{
				"operation":  "write",
				"file_path":  "/data/huge.txt",
				"content":    strings.Repeat("X", 2000000),
				"max_size":   int64(1000),
			},
			expectedSuccess: false,
			expectedError:   "content size exceeds limit",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Executor.Execute(ctx, tc.params)
			
			require.NoError(t, err)
			require.NotNil(t, result)
			
			if tc.expectedSuccess {
				assert.True(t, result.Success)
				
				// Check result structure
				data, ok := result.Data.(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, data, "bytes_written")
				assert.Contains(t, data, "path")
				
				// Verify file was actually written
				filePath := tc.params["file_path"].(string)
				assert.True(t, executor.fileExists(filePath))
			} else {
				assert.False(t, result.Success)
				assert.Contains(t, result.Error, tc.expectedError)
			}
		})
	}
}

// TestFileOperationsTool_ListOperations tests directory listing functionality
func TestFileOperationsTool_ListOperations(t *testing.T) {
	tool := createFileOperationsTool()
	executor := tool.Executor.(*MockFileOperationsExecutor)
	setupMockFileSystem(executor)
	ctx := context.Background()

	testCases := []struct {
		name            string
		params          map[string]interface{}
		expectedSuccess bool
		minFileCount    int
	}{
		{
			name: "List directory",
			params: map[string]interface{}{
				"operation":  "list",
				"file_path":  "/data",
			},
			expectedSuccess: true,
			minFileCount:    2, // At least sample.txt and config.json
		},
		{
			name: "List with pattern filter",
			params: map[string]interface{}{
				"operation":  "list",
				"file_path":  "/data",
				"pattern":    "*.txt",
			},
			expectedSuccess: true,
			minFileCount:    1, // At least sample.txt
		},
		{
			name: "List empty directory",
			params: map[string]interface{}{
				"operation":  "list",
				"file_path":  "/temp",
			},
			expectedSuccess: true,
			minFileCount:    0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Executor.Execute(ctx, tc.params)
			
			require.NoError(t, err)
			require.NotNil(t, result)
			
			if tc.expectedSuccess {
				assert.True(t, result.Success)
				
				// Check result structure
				data, ok := result.Data.(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, data, "files")
				assert.Contains(t, data, "count")
				
				files, ok := data["files"].([]map[string]interface{})
				require.True(t, ok)
				assert.GreaterOrEqual(t, len(files), tc.minFileCount)
				
				// Check file structure if files exist
				if len(files) > 0 {
					file := files[0]
					assert.Contains(t, file, "name")
					assert.Contains(t, file, "path")
					assert.Contains(t, file, "size")
					assert.Contains(t, file, "type")
				}
			} else {
				assert.False(t, result.Success)
			}
		})
	}
}

// TestFileOperationsTool_CopyOperations tests file copying functionality
func TestFileOperationsTool_CopyOperations(t *testing.T) {
	tool := createFileOperationsTool()
	executor := tool.Executor.(*MockFileOperationsExecutor)
	setupMockFileSystem(executor)
	ctx := context.Background()

	testCases := []struct {
		name            string
		params          map[string]interface{}
		expectedSuccess bool
		expectedError   string
	}{
		{
			name: "Copy existing file",
			params: map[string]interface{}{
				"operation":   "copy",
				"file_path":   "/data/sample.txt",
				"destination": "/data/sample_copy.txt",
			},
			expectedSuccess: true,
		},
		{
			name: "Copy with overwrite",
			params: map[string]interface{}{
				"operation":   "copy",
				"file_path":   "/data/sample.txt",
				"destination": "/data/config.json",
				"overwrite":   true,
			},
			expectedSuccess: true,
		},
		{
			name: "Copy without overwrite to existing file",
			params: map[string]interface{}{
				"operation":   "copy",
				"file_path":   "/data/sample.txt",
				"destination": "/data/config.json",
				"overwrite":   false,
			},
			expectedSuccess: false,
			expectedError:   "destination file exists",
		},
		{
			name: "Copy non-existent source",
			params: map[string]interface{}{
				"operation":   "copy",
				"file_path":   "/data/nonexistent.txt",
				"destination": "/data/copy.txt",
			},
			expectedSuccess: false,
			expectedError:   "source file not found",
		},
		{
			name: "Copy without destination",
			params: map[string]interface{}{
				"operation":  "copy",
				"file_path":  "/data/sample.txt",
			},
			expectedSuccess: false,
			expectedError:   "destination parameter is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Executor.Execute(ctx, tc.params)
			
			require.NoError(t, err)
			require.NotNil(t, result)
			
			if tc.expectedSuccess {
				assert.True(t, result.Success)
				
				// Check result structure
				data, ok := result.Data.(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, data, "source")
				assert.Contains(t, data, "destination")
				assert.Contains(t, data, "bytes_copied")
				
				// Verify file was actually copied
				if dest, ok := tc.params["destination"].(string); ok {
					assert.True(t, executor.fileExists(dest))
				}
			} else {
				assert.False(t, result.Success)
				assert.Contains(t, result.Error, tc.expectedError)
			}
		})
	}
}

// TestFileOperationsTool_DeleteOperations tests file deletion functionality
func TestFileOperationsTool_DeleteOperations(t *testing.T) {
	tool := createFileOperationsTool()
	executor := tool.Executor.(*MockFileOperationsExecutor)
	setupMockFileSystem(executor)
	ctx := context.Background()

	testCases := []struct {
		name            string
		params          map[string]interface{}
		expectedSuccess bool
		expectedError   string
	}{
		{
			name: "Delete existing file",
			params: map[string]interface{}{
				"operation":  "delete",
				"file_path":  "/data/config.json",
			},
			expectedSuccess: true,
		},
		{
			name: "Delete with backup",
			params: map[string]interface{}{
				"operation":  "delete",
				"file_path":  "/data/large.txt",
				"backup":     true,
			},
			expectedSuccess: true,
		},
		{
			name: "Delete non-existent file",
			params: map[string]interface{}{
				"operation":  "delete",
				"file_path":  "/data/nonexistent.txt",
			},
			expectedSuccess: false,
			expectedError:   "file not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := tc.params["file_path"].(string)
			fileExistedBefore := executor.fileExists(filePath)
			
			result, err := tool.Executor.Execute(ctx, tc.params)
			
			require.NoError(t, err)
			require.NotNil(t, result)
			
			if tc.expectedSuccess {
				assert.True(t, result.Success)
				
				// Check result structure
				data, ok := result.Data.(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, data, "path")
				assert.Contains(t, data, "bytes_deleted")
				
				// Verify file was actually deleted
				assert.False(t, executor.fileExists(filePath))
				
				// Check backup if requested
				if backup, ok := tc.params["backup"].(bool); ok && backup && fileExistedBefore {
					assert.True(t, executor.fileExists(filePath+".deleted"))
				}
			} else {
				assert.False(t, result.Success)
				assert.Contains(t, result.Error, tc.expectedError)
			}
		})
	}
}

// TestFileOperationsTool_StatOperations tests file stat functionality
func TestFileOperationsTool_StatOperations(t *testing.T) {
	tool := createFileOperationsTool()
	executor := tool.Executor.(*MockFileOperationsExecutor)
	setupMockFileSystem(executor)
	ctx := context.Background()

	testCases := []struct {
		name            string
		params          map[string]interface{}
		expectedSuccess bool
	}{
		{
			name: "Stat existing file",
			params: map[string]interface{}{
				"operation":  "stat",
				"file_path":  "/data/sample.txt",
			},
			expectedSuccess: true,
		},
		{
			name: "Stat non-existent file",
			params: map[string]interface{}{
				"operation":  "stat",
				"file_path":  "/data/nonexistent.txt",
			},
			expectedSuccess: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Executor.Execute(ctx, tc.params)
			
			require.NoError(t, err)
			require.NotNil(t, result)
			
			if tc.expectedSuccess {
				assert.True(t, result.Success)
				
				// Check result structure
				data, ok := result.Data.(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, data, "path")
				assert.Contains(t, data, "size")
				assert.Contains(t, data, "type")
				assert.Contains(t, data, "exists")
				assert.Contains(t, data, "readable")
				assert.Contains(t, data, "writable")
				assert.Contains(t, data, "modified")
			} else {
				assert.False(t, result.Success)
			}
		})
	}
}

// TestFileOperationsTool_SecurityValidation tests security constraints
func TestFileOperationsTool_SecurityValidation(t *testing.T) {
	tool := createFileOperationsTool()
	ctx := context.Background()

	securityTestCases := []struct {
		name          string
		params        map[string]interface{}
		expectedError string
	}{
		{
			name: "Path traversal attempt",
			params: map[string]interface{}{
				"operation":  "read",
				"file_path":  "/data/../etc/passwd",
			},
			expectedError: "path traversal not allowed",
		},
		{
			name: "System directory access",
			params: map[string]interface{}{
				"operation":  "read",
				"file_path":  "/etc/passwd",
			},
			expectedError: "access to system directories not allowed",
		},
		{
			name: "Very long path",
			params: map[string]interface{}{
				"operation":  "read",
				"file_path":  strings.Repeat("a", 1500),
			},
			expectedError: "path too long",
		},
	}

	for _, tc := range securityTestCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Executor.Execute(ctx, tc.params)
			
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.False(t, result.Success)
			assert.Contains(t, result.Error, tc.expectedError)
		})
	}
}

// TestFileOperationsTool_DirectoryOperations tests directory creation
func TestFileOperationsTool_DirectoryOperations(t *testing.T) {
	tool := createFileOperationsTool()
	executor := tool.Executor.(*MockFileOperationsExecutor)
	setupMockFileSystem(executor)
	ctx := context.Background()

	testCases := []struct {
		name            string
		params          map[string]interface{}
		expectedSuccess bool
		expectedError   string
	}{
		{
			name: "Create new directory",
			params: map[string]interface{}{
				"operation":  "mkdir",
				"file_path":  "/data/newdir",
			},
			expectedSuccess: true,
		},
		{
			name: "Create directory recursively",
			params: map[string]interface{}{
				"operation":  "mkdir",
				"file_path":  "/data/nested/deep/dir",
				"recursive":  true,
			},
			expectedSuccess: true,
		},
		{
			name: "Create existing directory",
			params: map[string]interface{}{
				"operation":  "mkdir",
				"file_path":  "/data",
			},
			expectedSuccess: false,
			expectedError:   "directory already exists",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Executor.Execute(ctx, tc.params)
			
			require.NoError(t, err)
			require.NotNil(t, result)
			
			if tc.expectedSuccess {
				assert.True(t, result.Success)
				
				// Check result structure
				data, ok := result.Data.(map[string]interface{})
				require.True(t, ok)
				assert.Contains(t, data, "path")
				assert.Contains(t, data, "created")
				
				// Verify directory was created
				dirPath := tc.params["file_path"].(string)
				assert.True(t, executor.directories[dirPath])
			} else {
				assert.False(t, result.Success)
				assert.Contains(t, result.Error, tc.expectedError)
			}
		})
	}
}

// TestFileOperationsTool_ErrorHandling tests error conditions
func TestFileOperationsTool_ErrorHandling(t *testing.T) {
	tool := createFileOperationsTool()
	ctx := context.Background()

	testCases := []struct {
		name          string
		params        map[string]interface{}
		expectedError string
	}{
		{
			name:          "Missing operation",
			params:        map[string]interface{}{"file_path": "/data/test.txt"},
			expectedError: "operation parameter is required",
		},
		{
			name:          "Missing file_path",
			params:        map[string]interface{}{"operation": "read"},
			expectedError: "file_path parameter is required",
		},
		{
			name: "Invalid operation",
			params: map[string]interface{}{
				"operation":  "invalid",
				"file_path":  "/data/test.txt",
			},
			expectedError: "unsupported operation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Executor.Execute(ctx, tc.params)
			
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.False(t, result.Success)
			assert.Contains(t, result.Error, tc.expectedError)
		})
	}
}

// TestFileOperationsTool_Performance tests performance characteristics
func TestFileOperationsTool_Performance(t *testing.T) {
	tool := createFileOperationsTool()
	executor := tool.Executor.(*MockFileOperationsExecutor)
	setupMockFileSystem(executor)
	ctx := context.Background()

	// Test read performance
	params := map[string]interface{}{
		"operation":  "read",
		"file_path":  "/data/sample.txt",
	}

	start := time.Now()
	result, err := tool.Executor.Execute(ctx, params)
	duration := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	
	// Should complete quickly (this is a mock, so very fast)
	assert.Less(t, duration, time.Millisecond*100)

	t.Logf("Read operation completed in %v", duration)
}

// TestFileOperationsTool_Concurrency tests concurrent file operations
func TestFileOperationsTool_Concurrency(t *testing.T) {
	tool := createFileOperationsTool()
	executor := tool.Executor.(*MockFileOperationsExecutor)
	setupMockFileSystem(executor)
	ctx := context.Background()

	const numGoroutines = 10
	const operationsPerGoroutine = 10

	results := make(chan *tools.ToolExecutionResult, numGoroutines*operationsPerGoroutine)
	errors := make(chan error, numGoroutines*operationsPerGoroutine)

	// Start multiple goroutines performing file operations
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < operationsPerGoroutine; j++ {
				// Mix of read and write operations
				var params map[string]interface{}
				if j%2 == 0 {
					params = map[string]interface{}{
						"operation":  "read",
						"file_path":  "/data/sample.txt",
					}
				} else {
					params = map[string]interface{}{
						"operation":  "write",
						"file_path":  fmt.Sprintf("/data/test_%d_%d.txt", goroutineID, j),
						"content":    fmt.Sprintf("Content from goroutine %d operation %d", goroutineID, j),
					}
				}

				result, err := tool.Executor.Execute(ctx, params)
				if err != nil {
					errors <- err
					return
				}
				results <- result
			}
		}(i)
	}

	// Collect results
	var successCount int
	var errorCount int

	timeout := time.After(5 * time.Second)
	for i := 0; i < numGoroutines*operationsPerGoroutine; i++ {
		select {
		case result := <-results:
			if result.Success {
				successCount++
			} else {
				errorCount++
			}
		case err := <-errors:
			t.Errorf("Unexpected error: %v", err)
			errorCount++
		case <-timeout:
			t.Fatal("Test timed out")
		}
	}

	assert.Equal(t, numGoroutines*operationsPerGoroutine, successCount)
	assert.Equal(t, 0, errorCount)
}

// TestFileOperationsTool_Schema tests schema validation
func TestFileOperationsTool_Schema(t *testing.T) {
	tool := createFileOperationsTool()

	// Test schema structure
	assert.NotNil(t, tool.Schema)
	assert.Equal(t, "object", tool.Schema.Type)
	assert.Contains(t, tool.Schema.Properties, "operation")
	assert.Contains(t, tool.Schema.Properties, "file_path")
	assert.Contains(t, tool.Schema.Required, "operation")
	assert.Contains(t, tool.Schema.Required, "file_path")

	// Test operation enum values
	operationProp := tool.Schema.Properties["operation"]
	assert.NotNil(t, operationProp.Enum)
	assert.Contains(t, operationProp.Enum, "read")
	assert.Contains(t, operationProp.Enum, "write")
	assert.Contains(t, operationProp.Enum, "list")
	assert.Contains(t, operationProp.Enum, "copy")
	assert.Contains(t, operationProp.Enum, "delete")

	// Test pattern validation
	filePathProp := tool.Schema.Properties["file_path"]
	assert.NotEmpty(t, filePathProp.Pattern)
}

// TestFileOperationsTool_Metadata tests tool metadata
func TestFileOperationsTool_Metadata(t *testing.T) {
	tool := createFileOperationsTool()

	assert.Equal(t, "file-operations", tool.ID)
	assert.Equal(t, "FileOperations", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Equal(t, "1.0.0", tool.Version)

	assert.NotNil(t, tool.Metadata)
	assert.Equal(t, "Security Team", tool.Metadata.Author)
	assert.Equal(t, "MIT", tool.Metadata.License)
	assert.Contains(t, tool.Metadata.Tags, "file")
	assert.Contains(t, tool.Metadata.Tags, "security")

	// Test examples
	assert.NotEmpty(t, tool.Metadata.Examples)
	assert.Len(t, tool.Metadata.Examples, 2)
}

// TestFileOperationsTool_Capabilities tests tool capabilities
func TestFileOperationsTool_Capabilities(t *testing.T) {
	tool := createFileOperationsTool()

	assert.NotNil(t, tool.Capabilities)
	assert.False(t, tool.Capabilities.Streaming)
	assert.False(t, tool.Capabilities.Async)
	assert.True(t, tool.Capabilities.Cancelable)
	assert.False(t, tool.Capabilities.Cacheable) // File operations shouldn't be cached
	assert.Equal(t, 30*time.Second, tool.Capabilities.Timeout)
}

// BenchmarkFileOperationsTool_Operations benchmarks different file operations
func BenchmarkFileOperationsTool_Operations(b *testing.B) {
	tool := createFileOperationsTool()
	executor := tool.Executor.(*MockFileOperationsExecutor)
	setupMockFileSystem(executor)
	ctx := context.Background()

	operations := []struct {
		name   string
		params map[string]interface{}
	}{
		{
			name: "Read",
			params: map[string]interface{}{
				"operation":  "read",
				"file_path":  "/data/sample.txt",
			},
		},
		{
			name: "Write",
			params: map[string]interface{}{
				"operation":  "write",
				"file_path":  "/data/benchmark.txt",
				"content":    "Benchmark content",
			},
		},
		{
			name: "List",
			params: map[string]interface{}{
				"operation":  "list",
				"file_path":  "/data",
			},
		},
		{
			name: "Stat",
			params: map[string]interface{}{
				"operation":  "stat",
				"file_path":  "/data/sample.txt",
			},
		},
	}

	for _, op := range operations {
		b.Run(op.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := tool.Executor.Execute(ctx, op.params)
				if err != nil || !result.Success {
					b.Fatalf("Operation failed: %v", err)
				}
			}
		})
	}
}

// Example test showing how to use the file operations tool
func Example_fileOperationsTool_basicUsage() {
	tool := createFileOperationsTool()
	executor := tool.Executor.(*MockFileOperationsExecutor)
	executor.files["/example/test.txt"] = []byte("Hello, World!")
	ctx := context.Background()

	// Read a file
	params := map[string]interface{}{
		"operation":  "read",
		"file_path":  "/example/test.txt",
	}

	result, err := tool.Executor.Execute(ctx, params)
	if err != nil {
		panic(err)
	}

	if result.Success {
		data := result.Data.(map[string]interface{})
		fmt.Println("Content:", data["content"])
		fmt.Println("Size:", data["size"])
	} else {
		fmt.Println("Error:", result.Error)
	}

	// Output: Content: Hello, World!
	// Size: 13
}