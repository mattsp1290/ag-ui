package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// FileSource loads configuration from files (JSON, YAML)
type FileSource struct {
	filePath    string
	format      FileFormat
	priority    int
	lastModTime time.Time
}

// FileFormat represents supported file formats
type FileFormat int

const (
	FileFormatAuto FileFormat = iota
	FileFormatJSON
	FileFormatYAML
)

// FileSourceOptions configures file source behavior
type FileSourceOptions struct {
	FilePath string
	Format   FileFormat
	Priority int
}

// NewFileSource creates a new file configuration source
func NewFileSource(filePath string) *FileSource {
	return NewFileSourceWithOptions(&FileSourceOptions{
		FilePath: filePath,
		Format:   FileFormatAuto,
		Priority: 20,
	})
}

// NewFileSourceWithOptions creates a new file source with options
func NewFileSourceWithOptions(options *FileSourceOptions) *FileSource {
	if options == nil {
		options = &FileSourceOptions{}
	}
	
	format := options.Format
	if format == FileFormatAuto {
		format = detectFormat(options.FilePath)
	}
	
	return &FileSource{
		filePath: options.FilePath,
		format:   format,
		priority: options.Priority,
	}
}

// Name returns the source name
func (f *FileSource) Name() string {
	return fmt.Sprintf("file:%s", f.filePath)
}

// Priority returns the source priority
func (f *FileSource) Priority() int {
	return f.priority
}

// Load loads configuration from the file
func (f *FileSource) Load(ctx context.Context) (map[string]interface{}, error) {
	// Validate and sanitize file path to prevent path traversal attacks
	sanitizedPath, err := f.sanitizeFilePath(f.filePath)
	if err != nil {
		return nil, f.wrapError("load", "path_validation", fmt.Errorf("invalid file path %s: %w", f.filePath, err))
	}
	
	// Check if file exists and get file info for size validation
	fileInfo, err := os.Stat(sanitizedPath)
	if os.IsNotExist(err) {
		return map[string]interface{}{}, nil // Return empty config if file doesn't exist
	}
	if err != nil {
		return nil, f.wrapError("load", "file_stat", fmt.Errorf("failed to get file info for %s: %w", sanitizedPath, err))
	}
	
	// Validate file size to prevent DoS attacks through large files
	const maxFileSize = 10 * 1024 * 1024 // 10MB default limit
	if fileInfo.Size() > maxFileSize {
		return nil, f.wrapError("load", "file_size", fmt.Errorf("file size %d bytes exceeds maximum allowed %d bytes", fileInfo.Size(), maxFileSize))
	}
	
	// Additional security check: verify file is still within allowed paths after sanitization
	if err := f.validateFilePath(sanitizedPath); err != nil {
		return nil, f.wrapError("load", "security_validation", fmt.Errorf("file path security validation failed for %s: %w", sanitizedPath, err))
	}
	
	// Read file content
	data, err := ioutil.ReadFile(sanitizedPath)
	if err != nil {
		return nil, f.wrapError("load", "file_read", fmt.Errorf("failed to read config file %s: %w", sanitizedPath, err))
	}
	
	// Update last modified time
	f.lastModTime = fileInfo.ModTime()
	
	// Parse based on format
	config := make(map[string]interface{})
	
	switch f.format {
	case FileFormatJSON:
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, f.wrapError("load", "json_parse", fmt.Errorf("failed to parse JSON config file %s: %w", sanitizedPath, err))
		}
	case FileFormatYAML:
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, f.wrapError("load", "yaml_parse", fmt.Errorf("failed to parse YAML config file %s: %w", sanitizedPath, err))
		}
	default:
		return nil, f.wrapError("load", "format_unsupported", fmt.Errorf("unsupported file format for %s", sanitizedPath))
	}
	
	// Expand environment variables
	config = f.expandEnvironmentVariables(config)
	
	return config, nil
}

// Watch starts watching the file for changes
func (f *FileSource) Watch(ctx context.Context, callback func(map[string]interface{})) error {
	// Validate file path before starting watch
	sanitizedPath, err := f.sanitizeFilePath(f.filePath)
	if err != nil {
		return f.wrapError("watch", "path_validation", fmt.Errorf("invalid file path for watching %s: %w", f.filePath, err))
	}
	
	if err := f.validateFilePath(sanitizedPath); err != nil {
		return f.wrapError("watch", "security_validation", fmt.Errorf("file path security validation failed for watching %s: %w", sanitizedPath, err))
	}
	
	// Simple polling implementation
	// In production, you might want to use fsnotify or similar
	go func() {
		ticker := time.NewTicker(time.Second * 5) // Check every 5 seconds
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if info, err := os.Stat(sanitizedPath); err == nil {
					if info.ModTime().After(f.lastModTime) {
						// File has been modified
						if config, err := f.Load(ctx); err == nil {
							callback(config)
						}
					}
				}
			}
		}
	}()
	
	return nil
}

// CanWatch returns whether this source supports watching
func (f *FileSource) CanWatch() bool {
	return true
}

// LastModified returns when the source was last modified
func (f *FileSource) LastModified() time.Time {
	return f.lastModTime
}

// detectFormat detects file format from extension
func detectFormat(filePath string) FileFormat {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		return FileFormatJSON
	case ".yaml", ".yml":
		return FileFormatYAML
	default:
		// Default to YAML for unknown extensions
		return FileFormatYAML
	}
}

// expandEnvironmentVariables recursively expands environment variables in config values
func (f *FileSource) expandEnvironmentVariables(config map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	for key, value := range config {
		result[key] = f.expandValue(value)
	}
	
	return result
}

// expandValue expands environment variables in a single value
func (f *FileSource) expandValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return f.expandString(v)
	case map[string]interface{}:
		return f.expandEnvironmentVariables(v)
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = f.expandValue(item)
		}
		return result
	case map[interface{}]interface{}:
		// Handle YAML's interface{} keys
		result := make(map[string]interface{})
		for k, val := range v {
			if strKey, ok := k.(string); ok {
				result[strKey] = f.expandValue(val)
			}
		}
		return result
	default:
		return value
	}
}

// expandString expands environment variables in a string
// Supports formats: ${VAR}, ${VAR:default}, $VAR
func (f *FileSource) expandString(s string) string {
	result := s
	
	// Handle ${VAR:default} format
	for {
		start := strings.Index(result, "${")
		if start == -1 {
			break
		}
		
		end := strings.Index(result[start:], "}")
		if end == -1 {
			break
		}
		end += start
		
		// Extract variable expression
		expr := result[start+2 : end]
		
		var envVar, defaultVal string
		if colonIndex := strings.Index(expr, ":"); colonIndex != -1 {
			envVar = expr[:colonIndex]
			defaultVal = expr[colonIndex+1:]
		} else {
			envVar = expr
		}
		
		// Get environment variable value
		envVal := os.Getenv(envVar)
		if envVal == "" && defaultVal != "" {
			envVal = defaultVal
		}
		
		// Replace in result
		result = result[:start] + envVal + result[end+1:]
	}
	
	// Handle $VAR format (simple)
	words := strings.Fields(result)
	for i, word := range words {
		if strings.HasPrefix(word, "$") && !strings.HasPrefix(word, "${") {
			envVar := strings.TrimPrefix(word, "$")
			if envVal := os.Getenv(envVar); envVal != "" {
				words[i] = envVal
			}
		}
	}
	
	if len(words) > 0 {
		result = strings.Join(words, " ")
	}
	
	return result
}

// sanitizeFilePath sanitizes a file path to prevent path traversal attacks
func (f *FileSource) sanitizeFilePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}
	
	// Clean the path to resolve . and .. elements
	cleanPath := filepath.Clean(path)
	
	// Convert to absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	
	return absPath, nil
}

// validateFilePath performs additional security validation on file paths
func (f *FileSource) validateFilePath(path string) error {
	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal attempt detected")
	}
	
	// Define allowed directories for configuration files
	allowedDirs := []string{
		"/etc/ag-ui",        // System config
		"/opt/ag-ui/config", // Application config
		"./config",          // Relative config directory
		"./",                // Current directory
	}
	
	// Check if current working directory should be allowed
	currentDir, err := os.Getwd()
	if err == nil {
		allowedDirs = append(allowedDirs, currentDir)
		allowedDirs = append(allowedDirs, filepath.Join(currentDir, "config"))
	}
	
	// Allow temporary directories (for tests and system temp files)
	tempDir := os.TempDir()
	allowedDirs = append(allowedDirs, tempDir)
	
	// Check if the path is within allowed directories
	pathAllowed := false
	for _, allowedDir := range allowedDirs {
		absAllowedDir, err := filepath.Abs(allowedDir)
		if err != nil {
			continue
		}
		
		// Check if path is within or exactly matches allowed directory
		if strings.HasPrefix(path, absAllowedDir) {
			pathAllowed = true
			break
		}
	}
	
	if !pathAllowed {
		return fmt.Errorf("file path is not within allowed directories")
	}
	
	// Check for dangerous file extensions
	dangerousExts := []string{
		".exe", ".bat", ".cmd", ".com", ".scr", ".pif",
		".sh", ".bash", ".zsh", ".fish", ".ps1", ".vbs",
	}
	
	fileExt := strings.ToLower(filepath.Ext(path))
	for _, dangerousExt := range dangerousExts {
		if fileExt == dangerousExt {
			return fmt.Errorf("dangerous file extension detected: %s", fileExt)
		}
	}
	
	// Check for system-sensitive paths
	systemPaths := []string{
		"/etc/passwd", "/etc/shadow", "/etc/hosts", "/etc/sudoers",
		"/proc/", "/sys/", "/dev/", "/root/", "/boot/",
		"C:\\Windows\\", "C:\\System32\\", "C:\\Program Files\\",
	}
	
	for _, systemPath := range systemPaths {
		if strings.HasPrefix(path, systemPath) {
			return fmt.Errorf("access to system path not allowed: %s", systemPath)
		}
	}
	
	return nil
}

// SourceError represents an error from a configuration source
type SourceError struct {
	Op       string        // Operation that failed (load, watch, etc.)
	SubOp    string        // Sub-operation (parse, read, validate, etc.)
	Source   string        // Source name
	FilePath string        // File path (for file sources)
	Category ErrorCategory // Error category
	Err      error         // Underlying error
}

// ErrorCategory represents the category of source error
type ErrorCategory int

const (
	CategoryUnknown ErrorCategory = iota
	CategorySource     // Source-related (file not found, parse error)
	CategoryAccess     // Access/permission errors
	CategoryValidation // Validation errors
	CategorySecurity   // Security violations
	CategoryNetwork    // Network-related errors
	CategoryTimeout    // Timeout errors
)

// String returns the string representation of the error category
func (c ErrorCategory) String() string {
	switch c {
	case CategorySource:
		return "source"
	case CategoryAccess:
		return "access"
	case CategoryValidation:
		return "validation"
	case CategorySecurity:
		return "security"
	case CategoryNetwork:
		return "network"
	case CategoryTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

// IsTemporary returns true if the error is likely temporary
func (c ErrorCategory) IsTemporary() bool {
	switch c {
	case CategoryNetwork, CategoryTimeout:
		return true
	default:
		return false
	}
}

// Error implements the error interface
func (e *SourceError) Error() string {
	var parts []string
	
	if e.Op != "" {
		if e.SubOp != "" {
			parts = append(parts, fmt.Sprintf("source %s:%s", e.Op, e.SubOp))
		} else {
			parts = append(parts, fmt.Sprintf("source %s", e.Op))
		}
	} else {
		parts = append(parts, "source operation")
	}
	
	if e.Source != "" {
		parts = append(parts, fmt.Sprintf("source=%s", e.Source))
	}
	
	if e.FilePath != "" {
		parts = append(parts, fmt.Sprintf("file=%s", e.FilePath))
	}
	
	if e.Category != CategoryUnknown {
		parts = append(parts, fmt.Sprintf("category=%s", e.Category))
	}
	
	message := strings.Join(parts, " ")
	
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", message, e.Err)
	}
	
	return message
}

// Unwrap returns the underlying error
func (e *SourceError) Unwrap() error {
	return e.Err
}

// IsTemporary returns true if the error is temporary
func (e *SourceError) IsTemporary() bool {
	return e.Category.IsTemporary()
}

// wrapError wraps an error with source context and categorization
func (f *FileSource) wrapError(op, subOp string, err error) error {
	if err == nil {
		return nil
	}
	
	// Determine error category based on operation and error
	category := f.categorizeError(subOp, err)
	
	// Create structured error with context
	return &SourceError{
		Op:       op,
		SubOp:    subOp,
		Source:   f.Name(),
		FilePath: f.filePath,
		Category: category,
		Err:      err,
	}
}

// categorizeError categorizes errors based on operation and error content
func (f *FileSource) categorizeError(subOp string, err error) ErrorCategory {
	if err == nil {
		return CategoryUnknown
	}
	
	errMsg := strings.ToLower(err.Error())
	
	switch subOp {
	case "path_validation", "security_validation":
		return CategorySecurity
	case "file_read":
		if strings.Contains(errMsg, "permission denied") {
			return CategoryAccess
		}
		if strings.Contains(errMsg, "no such file") {
			return CategorySource
		}
		return CategorySource
	case "json_parse", "yaml_parse":
		return CategorySource
	case "format_unsupported":
		return CategoryValidation
	default:
		return CategoryUnknown
	}
}