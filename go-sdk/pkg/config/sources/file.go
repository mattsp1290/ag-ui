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
	// Check if file exists
	if _, err := os.Stat(f.filePath); os.IsNotExist(err) {
		return map[string]interface{}{}, nil // Return empty config if file doesn't exist
	}
	
	// Read file content
	data, err := ioutil.ReadFile(f.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", f.filePath, err)
	}
	
	// Update last modified time
	if info, err := os.Stat(f.filePath); err == nil {
		f.lastModTime = info.ModTime()
	}
	
	// Parse based on format
	config := make(map[string]interface{})
	
	switch f.format {
	case FileFormatJSON:
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config file %s: %w", f.filePath, err)
		}
	case FileFormatYAML:
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config file %s: %w", f.filePath, err)
		}
	default:
		return nil, fmt.Errorf("unsupported file format for %s", f.filePath)
	}
	
	// Expand environment variables
	config = f.expandEnvironmentVariables(config)
	
	return config, nil
}

// Watch starts watching the file for changes
func (f *FileSource) Watch(ctx context.Context, callback func(map[string]interface{})) error {
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
				if info, err := os.Stat(f.filePath); err == nil {
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