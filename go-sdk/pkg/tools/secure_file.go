package tools

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// SecureFileOptions defines security options for file operations
type SecureFileOptions struct {
	// AllowedPaths defines paths that are allowed for file operations
	// If empty, all paths are allowed (not recommended for production)
	AllowedPaths []string

	// MaxFileSize is the maximum allowed file size in bytes
	// Default is 100MB
	MaxFileSize int64

	// DenyPaths defines paths that are explicitly denied
	// Takes precedence over AllowedPaths
	DenyPaths []string

	// AllowSymlinks determines if symbolic links can be followed
	AllowSymlinks bool
}

// DefaultSecureFileOptions returns secure default options
func DefaultSecureFileOptions() *SecureFileOptions {
	return &SecureFileOptions{
		MaxFileSize:   100 * 1024 * 1024, // 100MB
		AllowSymlinks: false,
		DenyPaths: []string{
			"/etc",
			"/sys",
			"/proc",
			"~/.ssh",
			"~/.aws",
			"~/.config",
		},
	}
}

// SecureFileExecutor wraps file operations with security checks
type SecureFileExecutor struct {
	options       *SecureFileOptions
	executor      ToolExecutor
	operationType string // "read" or "write"
}

// NewSecureFileExecutor creates a new secure file executor
func NewSecureFileExecutor(executor ToolExecutor, options *SecureFileOptions, operationType string) *SecureFileExecutor {
	if options == nil {
		options = DefaultSecureFileOptions()
	}
	return &SecureFileExecutor{
		options:       options,
		executor:      executor,
		operationType: operationType,
	}
}

// Execute performs the file operation with security checks
func (e *SecureFileExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	// Extract path from params
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter is required")
	}

	// Use atomic operations based on operation type to prevent TOCTOU race conditions
	if e.isReadOperation() {
		return e.executeAtomicRead(ctx, path)
	} else {
		return e.executeAtomicWrite(ctx, params)
	}
}

// validatePath checks if the path is allowed based on security options
func (e *SecureFileExecutor) validatePath(path string) error {
	// Check for null bytes and control characters first
	if strings.Contains(path, "\x00") || containsControlChars(path) {
		return fmt.Errorf("invalid path")
	}

	// Check for very long paths
	if len(path) > 1000 {
		return fmt.Errorf("path too long")
	}

	// Check for empty path
	if path == "" {
		return fmt.Errorf("invalid path")
	}

	// Decode URL-encoded characters to prevent encoded traversal attacks
	decodedPath, err := decodeURLPath(path)
	if err != nil {
		return fmt.Errorf("invalid path encoding: %v", err)
	}

	// Clean and resolve the decoded path
	cleanPath, err := filepath.Abs(filepath.Clean(decodedPath))
	if err != nil {
		return fmt.Errorf("invalid path")
	}

	// Special case: Always deny root directory access
	if cleanPath == "/" {
		return fmt.Errorf("access denied")
	}

	// Expand home directory in deny paths
	for _, denyPath := range e.options.DenyPaths {
		expandedDeny := expandPath(denyPath)
		absDeny, err := filepath.Abs(filepath.Clean(expandedDeny))
		if err != nil {
			continue // Skip invalid deny paths
		}
		rel, err := filepath.Rel(absDeny, cleanPath)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return fmt.Errorf("access denied")
		}
	}

	// Check symbolic links
	if info, err := os.Lstat(cleanPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if !e.options.AllowSymlinks {
				return fmt.Errorf("symbolic links are not allowed")
			}
			// Even if symlinks are allowed, validate the target path
			if target, err := os.Readlink(cleanPath); err == nil {
				// If target is relative, make it absolute relative to the symlink's directory
				if !filepath.IsAbs(target) {
					target = filepath.Join(filepath.Dir(cleanPath), target)
				}
				if err := e.validateSymlinkTarget(target); err != nil {
					return err
				}
			}
		}
	}

	// If no allowed paths are specified, allow all (except denied)
	if len(e.options.AllowedPaths) == 0 {
		return nil
	}

	// Check if path is within allowed paths
	for _, allowedPath := range e.options.AllowedPaths {
		expandedAllow := expandPath(allowedPath)
		absAllowed, err := filepath.Abs(expandedAllow)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absAllowed, cleanPath)
		if err == nil && !isPathTraversal(rel) {
			return nil
		}
	}

	return fmt.Errorf("access denied")
}

// checkFileSize verifies the file size is within limits
func (e *SecureFileExecutor) checkFileSize(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		// File doesn't exist yet, which is fine for write operations
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot stat file: %w", err)
	}

	if info.Size() > e.options.MaxFileSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size of %d bytes",
			info.Size(), e.options.MaxFileSize)
	}

	return nil
}

// validateFileDescriptor performs security validation on an open file descriptor
// This helps prevent TOCTOU race conditions by validating after opening
func (e *SecureFileExecutor) validateFileDescriptor(file *os.File) error {
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("cannot stat file descriptor: %w", err)
	}

	// Check file size limit
	if stat.Size() > e.options.MaxFileSize {
		return fmt.Errorf("file size %d exceeds maximum allowed size of %d bytes",
			stat.Size(), e.options.MaxFileSize)
	}

	// Check that it's a regular file (not a device, pipe, etc.)
	if !stat.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}

	// For additional security, we could check ownership, permissions, etc.
	// but this provides basic protection against special files

	return nil
}

// isReadOperation checks if this executor is for a read operation
func (e *SecureFileExecutor) isReadOperation() bool {
	return e.operationType == "read"
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// containsControlChars checks if a string contains control characters
func containsControlChars(s string) bool {
	for _, r := range s {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return true
		}
	}
	return false
}

// isPathTraversal checks if a relative path represents path traversal
// It distinguishes between actual ".." path components and filenames starting with ".."
func isPathTraversal(rel string) bool {
	if rel == "" {
		return false
	}
	
	// Split the path into components
	components := strings.Split(rel, string(filepath.Separator))
	
	// Check if any component is exactly ".."
	for _, component := range components {
		if component == ".." {
			return true
		}
	}
	
	return false
}

// validateSymlinkTarget validates a symlink target path (without recursion)
func (e *SecureFileExecutor) validateSymlinkTarget(path string) error {
	// Check for null bytes and control characters first
	if strings.Contains(path, "\x00") || containsControlChars(path) {
		return fmt.Errorf("invalid path")
	}

	// Check for very long paths
	if len(path) > 1000 {
		return fmt.Errorf("path too long")
	}

	// Check for empty path
	if path == "" {
		return fmt.Errorf("invalid path")
	}

	// Decode URL-encoded characters to prevent encoded traversal attacks
	decodedPath, err := decodeURLPath(path)
	if err != nil {
		return fmt.Errorf("invalid path encoding: %v", err)
	}

	// Clean and resolve the decoded path
	cleanPath, err := filepath.Abs(filepath.Clean(decodedPath))
	if err != nil {
		return fmt.Errorf("invalid path")
	}

	// Expand home directory in deny paths
	for _, denyPath := range e.options.DenyPaths {
		expandedDeny := expandPath(denyPath)
		absDeny, err := filepath.Abs(filepath.Clean(expandedDeny))
		if err != nil {
			continue // Skip invalid deny paths
		}
		rel, err := filepath.Rel(absDeny, cleanPath)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return fmt.Errorf("access denied")
		}
	}

	// If no allowed paths are specified, allow all (except denied)
	if len(e.options.AllowedPaths) == 0 {
		return nil
	}

	// Check if path is within allowed paths
	for _, allowedPath := range e.options.AllowedPaths {
		expandedAllow := expandPath(allowedPath)
		absAllowed, err := filepath.Abs(expandedAllow)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absAllowed, cleanPath)
		if err == nil && !isPathTraversal(rel) {
			return nil
		}
	}

	return fmt.Errorf("access denied")
}



// NewSecureReadFileTool creates a secure file reading tool
func NewSecureReadFileTool(options *SecureFileOptions) *Tool {
	baseTool := NewReadFileTool()
	baseTool.Executor = NewSecureFileExecutor(&readFileExecutor{}, options, "read")
	return baseTool
}

// NewSecureWriteFileTool creates a secure file writing tool
func NewSecureWriteFileTool(options *SecureFileOptions) *Tool {
	baseTool := NewWriteFileTool()
	baseTool.Executor = NewSecureFileExecutor(&writeFileExecutor{}, options, "write")
	return baseTool
}

// executeAtomicRead performs atomic read operation to prevent TOCTOU race conditions
func (e *SecureFileExecutor) executeAtomicRead(ctx context.Context, path string) (*ToolExecutionResult, error) {
	// First validate the path
	if err := e.validatePath(path); err != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("path validation failed: %v", err),
		}, nil
	}

	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to open file: %v", err),
		}, nil
	}
	defer file.Close()

	// Validate the opened file descriptor to prevent TOCTOU attacks
	if validationErr := e.validateFileDescriptor(file); validationErr != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("file validation failed: %v", validationErr),
		}, nil
	}

	// Read the file content with size limit
	const maxReadSize = 100 * 1024 * 1024 // 100MB limit
	limitedReader := io.LimitReader(file, maxReadSize)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to read file: %v", err),
		}, nil
	}

	return &ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"content": string(data),
			"size":    len(data),
		},
	}, nil
}

// executeAtomicWrite performs atomic write operation to prevent TOCTOU race conditions
func (e *SecureFileExecutor) executeAtomicWrite(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	path, ok := params["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path parameter is required")
	}

	content, ok := params["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content parameter is required")
	}

	mode, _ := params["mode"].(string)
	if mode == "" {
		mode = "write"
	}

	// First validate the path
	if err := e.validatePath(path); err != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("path validation failed: %v", err),
		}, nil
	}

	// Validate parent directory path
	dir := filepath.Dir(path)
	if err := e.validatePath(dir); err != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("parent directory access denied: %v", err),
		}, nil
	}

	// Create directory if needed
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create directory: %v", err),
		}, nil
	}

	// Choose the appropriate file opening mode
	var flags int
	switch mode {
	case "append":
		flags = os.O_APPEND | os.O_CREATE | os.O_WRONLY
	default: // "write" or default
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}

	// Open the file atomically
	file, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to open file for writing: %v", err),
		}, nil
	}
	defer file.Close()

	// Validate the file descriptor for security
	if validationErr := e.validateFileDescriptor(file); validationErr != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("file validation failed: %v", validationErr),
		}, nil
	}

	// Write the data
	_, err = file.WriteString(content)
	if err != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to write to file: %v", err),
		}, nil
	}

	// Sync to ensure data is written
	if err := file.Sync(); err != nil {
		return &ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to sync file: %v", err),
		}, nil
	}

	return &ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"path":          path,
			"bytes_written": len(content),
		},
	}, nil
}

// checkSymlinksInPath checks if any component in the path is a symbolic link
func (e *SecureFileExecutor) checkSymlinksInPath(path string) error {
	// Check each component of the path from root down
	current := ""
	components := strings.Split(path, string(filepath.Separator))
	
	for i, component := range components {
		if component == "" {
			if i == 0 {
				current = string(filepath.Separator)
			}
			continue
		}
		
		if current == string(filepath.Separator) {
			current = filepath.Join(current, component)
		} else {
			current = filepath.Join(current, component)
		}
		
		// Check if this component is a symbolic link
		if info, err := os.Lstat(current); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				// Allow certain system symlinks that are safe (like /var -> private/var on macOS)
				if e.isAllowedSystemSymlink(current) {
					continue
				}
				return fmt.Errorf("symbolic links are not allowed")
			}
		}
		// If the component doesn't exist, we can't check it, but that's okay for write operations
	}
	return nil
}

// isAllowedSystemSymlink checks if a symlink is a known safe system symlink
func (e *SecureFileExecutor) isAllowedSystemSymlink(path string) bool {
	// Allow common macOS system symlinks
	allowedSystemSymlinks := []string{
		"/var",        // /var -> private/var
		"/tmp",        // /tmp -> private/tmp
		"/etc",        // /etc -> private/etc on some systems
	}
	
	for _, allowed := range allowedSystemSymlinks {
		if path == allowed {
			return true
		}
	}
	
	return false
}

// decodeURLPath safely decodes URL-encoded characters in a file path
// This prevents attackers from bypassing security checks using encoded traversal sequences
func decodeURLPath(path string) (string, error) {
	// Start with the original path
	decoded := path
	
	// Perform multiple rounds of decoding to handle double/triple encoding
	// This prevents attacks like %252e%252e%252f (triple-encoded ../)
	maxDecodeRounds := 5
	for i := 0; i < maxDecodeRounds; i++ {
		newDecoded, err := url.PathUnescape(decoded)
		if err != nil {
			break // Invalid encoding, stop here
		}
		if newDecoded == decoded {
			break // No more changes, we're done
		}
		decoded = newDecoded
	}
	
	// Normalize various representations of path separators and dangerous characters
	decoded = normalizePathSeparators(decoded)
	
	// Additional security check: reject paths with non-printable characters after decoding
	for _, r := range decoded {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return "", fmt.Errorf("decoded path contains invalid characters")
		}
	}
	
	return decoded, nil
}

// normalizePathSeparators converts various encoded path separator representations
// to standard forward slashes to prevent bypass attempts
func normalizePathSeparators(path string) string {
	// Convert common path separator representations
	// These can be used to bypass validation in various encoding schemes
	
	// Convert backslashes to forward slashes (Windows-style paths)
	path = strings.ReplaceAll(path, "\\", "/")
	
	// Convert UTF-8 encoded separators
	path = strings.ReplaceAll(path, "\u002f", "/")  // UTF-8 encoded /
	path = strings.ReplaceAll(path, "\u005c", "/")  // UTF-8 encoded \
	path = strings.ReplaceAll(path, "\uff0f", "/")  // UTF-8 fullwidth solidus
	path = strings.ReplaceAll(path, "\uff3c", "/")  // UTF-8 fullwidth reverse solidus
	
	// Convert overlong UTF-8 sequences (like %c0%af for /)
	// These are invalid UTF-8 but can sometimes bypass filters
	path = strings.ReplaceAll(path, "\xc0\xaf", "/")
	path = strings.ReplaceAll(path, "\xc1\x9c", "/")
	
	return path
}
