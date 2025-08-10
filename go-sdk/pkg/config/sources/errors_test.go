package sources

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileSourceError(t *testing.T) {
	tests := []struct {
		name        string
		sourceError *SourceError
		wantError   string
		wantOp      string
		wantSource  string
		wantCategory ErrorCategory
		isTemporary bool
	}{
		{
			name: "file read error",
			sourceError: &SourceError{
				Op:       "load",
				SubOp:    "file_read",
				Source:   "file:/config.yaml",
				FilePath: "/etc/config.yaml",
				Category: CategorySource,
				Err:      errors.New("permission denied"),
			},
			wantError:    "source load:file_read source=file:/config.yaml file=/etc/config.yaml category=source: permission denied",
			wantOp:       "load",
			wantSource:   "file:/config.yaml",
			wantCategory: CategorySource,
			isTemporary:  false,
		},
		{
			name: "network timeout error",
			sourceError: &SourceError{
				Op:       "load",
				SubOp:    "http_fetch",
				Source:   "http://example.com/config",
				Category: CategoryNetwork,
				Err:      errors.New("connection timeout"),
			},
			wantError:    "source load:http_fetch source=http://example.com/config category=network: connection timeout",
			wantOp:       "load",
			wantSource:   "http://example.com/config",
			wantCategory: CategoryNetwork,
			isTemporary:  true,
		},
		{
			name: "security validation error",
			sourceError: &SourceError{
				Op:       "load",
				SubOp:    "security_validation",
				Source:   "file:../../etc/passwd",
				FilePath: "/etc/passwd",
				Category: CategorySecurity,
				Err:      errors.New("path traversal detected"),
			},
			wantError:    "source load:security_validation source=file:../../etc/passwd file=/etc/passwd category=security: path traversal detected",
			wantOp:       "load",
			wantSource:   "file:../../etc/passwd",
			wantCategory: CategorySecurity,
			isTemporary:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sourceError.Error(); got != tt.wantError {
				t.Errorf("SourceError.Error() = %v, want %v", got, tt.wantError)
			}

			if got := tt.sourceError.IsTemporary(); got != tt.isTemporary {
				t.Errorf("SourceError.IsTemporary() = %v, want %v", got, tt.isTemporary)
			}

			if got := tt.sourceError.Unwrap(); got != tt.sourceError.Err {
				t.Error("SourceError.Unwrap() should return underlying error")
			}
		})
	}
}

func TestEnvSourceError(t *testing.T) {
	envErr := &EnvSourceError{
		Op:       "load",
		SubOp:    "key_conflict",
		Source:   "env:APP",
		Prefix:   "APP",
		Category: CategoryValidation,
		Err:      errors.New("conflicting keys"),
	}

	expectedMsg := "env load:key_conflict source=env:APP prefix=APP category=validation: conflicting keys"
	if got := envErr.Error(); got != expectedMsg {
		t.Errorf("EnvSourceError.Error() = %v, want %v", got, expectedMsg)
	}

	if envErr.Unwrap() != envErr.Err {
		t.Error("EnvSourceError.Unwrap() should return underlying error")
	}

	if envErr.IsTemporary() != envErr.Category.IsTemporary() {
		t.Error("EnvSourceError.IsTemporary() should match category")
	}
}

func TestFileSourceErrorHandling(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name         string
		setup        func() string
		wantErr      bool
		errorOp      string
		errorSubOp   string
		errorCategory ErrorCategory
	}{
		{
			name: "invalid JSON syntax",
			setup: func() string {
				filePath := filepath.Join(tempDir, "invalid.json")
				content := `{"key": invalid json}`
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return filePath
			},
			wantErr:       true,
			errorOp:       "load",
			errorSubOp:    "json_parse",
			errorCategory: CategorySource,
		},
		{
			name: "invalid YAML syntax",
			setup: func() string {
				filePath := filepath.Join(tempDir, "invalid.yaml")
				content := `key: [unclosed array`
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return filePath
			},
			wantErr:       true,
			errorOp:       "load",
			errorSubOp:    "yaml_parse",
			errorCategory: CategorySource,
		},
		{
			name: "unsupported file format",
			setup: func() string {
				filePath := filepath.Join(tempDir, "config.txt")
				content := `key=value`
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return filePath
			},
			wantErr:       true,
			errorOp:       "load",
			errorSubOp:    "format_unsupported",
			errorCategory: CategoryValidation,
		},
		{
			name: "valid JSON file",
			setup: func() string {
				filePath := filepath.Join(tempDir, "valid.json")
				content := `{"key": "value"}`
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
				return filePath
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := tt.setup()
			source := NewFileSource(filePath)

			_, err := source.Load(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error but got none")
				}

				sourceErr, ok := err.(*SourceError)
				if !ok {
					t.Fatalf("Expected SourceError, got %T", err)
				}

				if sourceErr.Op != tt.errorOp {
					t.Errorf("Expected operation %s, got %s", tt.errorOp, sourceErr.Op)
				}

				if sourceErr.SubOp != tt.errorSubOp {
					t.Errorf("Expected sub-operation %s, got %s", tt.errorSubOp, sourceErr.SubOp)
				}

				if sourceErr.Category != tt.errorCategory {
					t.Errorf("Expected category %v, got %v", tt.errorCategory, sourceErr.Category)
				}

				if sourceErr.Source == "" {
					t.Error("Source name should not be empty")
				}

				if sourceErr.FilePath != filePath {
					t.Errorf("Expected file path %s, got %s", filePath, sourceErr.FilePath)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestFileSourceSecurityErrors(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		wantErr      bool
		errorSubOp   string
		errorCategory ErrorCategory
	}{
		{
			name:          "path traversal attempt",
			filePath:      "../../../etc/passwd",
			wantErr:       true,
			errorSubOp:    "security_validation",
			errorCategory: CategorySecurity,
		},
		{
			name:          "empty file path",
			filePath:      "",
			wantErr:       true,
			errorSubOp:    "path_validation",
			errorCategory: CategorySecurity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewFileSource(tt.filePath)
			_, err := source.Load(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error but got none")
				}

				sourceErr, ok := err.(*SourceError)
				if !ok {
					t.Fatalf("Expected SourceError, got %T", err)
				}

				if sourceErr.SubOp != tt.errorSubOp {
					t.Errorf("Expected sub-operation %s, got %s", tt.errorSubOp, sourceErr.SubOp)
				}

				if sourceErr.Category != tt.errorCategory {
					t.Errorf("Expected category %v, got %v", tt.errorCategory, sourceErr.Category)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestEnvSourceErrorHandling(t *testing.T) {
	// Set up environment variables with conflicting structure
	os.Setenv("TEST_KEY", "scalar_value")
	os.Setenv("TEST_KEY_SUB", "nested_value") // This should cause a conflict
	defer func() {
		os.Unsetenv("TEST_KEY")
		os.Unsetenv("TEST_KEY_SUB")
	}()

	source := NewEnvSource("TEST")
	_, err := source.Load(context.Background())

	if err != nil {
		envErr, ok := err.(*EnvSourceError)
		if ok {
			if envErr.Op != "load" {
				t.Errorf("Expected operation 'load', got %s", envErr.Op)
			}
			if envErr.SubOp != "key_conflict" {
				t.Errorf("Expected sub-operation 'key_conflict', got %s", envErr.SubOp)
			}
			if envErr.Category != CategoryValidation {
				t.Errorf("Expected category %v, got %v", CategoryValidation, envErr.Category)
			}
		}
	}
}

func TestErrorCategorization(t *testing.T) {
	source := &FileSource{filePath: "test.json"}

	tests := []struct {
		name        string
		subOp       string
		err         error
		expected    ErrorCategory
	}{
		{"path validation", "path_validation", errors.New("invalid path"), CategorySecurity},
		{"security validation", "security_validation", errors.New("security error"), CategorySecurity},
		{"file read - permission", "file_read", errors.New("permission denied"), CategoryAccess},
		{"file read - not found", "file_read", errors.New("no such file or directory"), CategorySource},
		{"file read - other", "file_read", errors.New("io error"), CategorySource},
		{"json parse", "json_parse", errors.New("syntax error"), CategorySource},
		{"yaml parse", "yaml_parse", errors.New("parse error"), CategorySource},
		{"format unsupported", "format_unsupported", errors.New("unknown format"), CategoryValidation},
		{"unknown", "unknown_op", errors.New("some error"), CategoryUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := source.categorizeError(tt.subOp, tt.err)
			if got != tt.expected {
				t.Errorf("categorizeError(%s, %v) = %v, want %v", tt.subOp, tt.err, got, tt.expected)
			}
		})
	}
}

func TestErrorCategoryString(t *testing.T) {
	tests := []struct {
		category ErrorCategory
		expected string
	}{
		{CategoryUnknown, "unknown"},
		{CategorySource, "source"},
		{CategoryAccess, "access"},
		{CategoryValidation, "validation"},
		{CategorySecurity, "security"},
		{CategoryNetwork, "network"},
		{CategoryTimeout, "timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.category.String(); got != tt.expected {
				t.Errorf("ErrorCategory.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrorCategoryIsTemporary(t *testing.T) {
	temporaryCategories := []ErrorCategory{CategoryNetwork, CategoryTimeout}
	permanentCategories := []ErrorCategory{CategoryUnknown, CategorySource, CategoryAccess, CategoryValidation, CategorySecurity}

	for _, cat := range temporaryCategories {
		t.Run(cat.String()+"_temporary", func(t *testing.T) {
			if !cat.IsTemporary() {
				t.Errorf("ErrorCategory %v should be temporary", cat)
			}
		})
	}

	for _, cat := range permanentCategories {
		t.Run(cat.String()+"_permanent", func(t *testing.T) {
			if cat.IsTemporary() {
				t.Errorf("ErrorCategory %v should not be temporary", cat)
			}
		})
	}
}

func TestFileSourceWrapError(t *testing.T) {
	source := &FileSource{
		filePath: "/test/config.yaml",
	}

	originalErr := errors.New("test error")
	wrappedErr := source.wrapError("load", "file_read", originalErr)

	sourceErr, ok := wrappedErr.(*SourceError)
	if !ok {
		t.Fatal("wrapError should return SourceError")
	}

	if sourceErr.Op != "load" {
		t.Errorf("Expected operation 'load', got %s", sourceErr.Op)
	}
	if sourceErr.SubOp != "file_read" {
		t.Errorf("Expected sub-operation 'file_read', got %s", sourceErr.SubOp)
	}
	if sourceErr.FilePath != source.filePath {
		t.Errorf("Expected file path %s, got %s", source.filePath, sourceErr.FilePath)
	}
	if !strings.Contains(sourceErr.Source, "file:") {
		t.Error("Source name should contain 'file:'")
	}
	if !errors.Is(wrappedErr, originalErr) {
		t.Error("Wrapped error should preserve original error for errors.Is")
	}

	// Test nil error
	if source.wrapError("load", "test", nil) != nil {
		t.Error("wrapError should return nil for nil error")
	}
}

func TestEnvSourceWrapError(t *testing.T) {
	source := &EnvSource{
		prefix: "TEST",
	}

	originalErr := errors.New("test error")
	wrappedErr := source.wrapError("load", "key_conflict", originalErr)

	envErr, ok := wrappedErr.(*EnvSourceError)
	if !ok {
		t.Fatal("wrapError should return EnvSourceError")
	}

	if envErr.Op != "load" {
		t.Errorf("Expected operation 'load', got %s", envErr.Op)
	}
	if envErr.SubOp != "key_conflict" {
		t.Errorf("Expected sub-operation 'key_conflict', got %s", envErr.SubOp)
	}
	if envErr.Prefix != source.prefix {
		t.Errorf("Expected prefix %s, got %s", source.prefix, envErr.Prefix)
	}
	if !strings.Contains(envErr.Source, "env:") {
		t.Error("Source name should contain 'env:'")
	}
	if !errors.Is(wrappedErr, originalErr) {
		t.Error("Wrapped error should preserve original error for errors.Is")
	}

	// Test nil error
	if source.wrapError("load", "test", nil) != nil {
		t.Error("wrapError should return nil for nil error")
	}
}

func TestErrorIntegration(t *testing.T) {
	// Test that source errors integrate properly with the main config error system
	tempDir, err := os.MkdirTemp("", "config_integration_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create an invalid JSON file
	invalidFile := filepath.Join(tempDir, "invalid.json")
	if err := os.WriteFile(invalidFile, []byte(`{"key": invalid}`), 0644); err != nil {
		t.Fatal(err)
	}

	source := NewFileSource(invalidFile)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = source.Load(ctx)

	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}

	// Check that the error has proper structure
	sourceErr, ok := err.(*SourceError)
	if !ok {
		t.Fatalf("Expected SourceError, got %T: %v", err, err)
	}

	// Verify error details
	if sourceErr.Op != "load" {
		t.Errorf("Expected operation 'load', got %s", sourceErr.Op)
	}
	if sourceErr.Category != CategorySource {
		t.Errorf("Expected category %v, got %v", CategorySource, sourceErr.Category)
	}
	if sourceErr.IsTemporary() {
		t.Error("Parse errors should not be temporary")
	}

	// Test error message quality
	errorMsg := err.Error()
	if !strings.Contains(errorMsg, "source") {
		t.Error("Error message should contain 'source'")
	}
	if !strings.Contains(errorMsg, "load") {
		t.Error("Error message should contain operation")
	}
	if !strings.Contains(errorMsg, invalidFile) {
		t.Error("Error message should contain file path")
	}
}

func BenchmarkSourceError(b *testing.B) {
	err := errors.New("base error")
	
	b.Run("SourceError_Error", func(b *testing.B) {
		sourceErr := &SourceError{
			Op:       "load",
			SubOp:    "file_read",
			Source:   "file:/very/long/path/to/config/file.yaml",
			FilePath: "/very/long/path/to/config/file.yaml",
			Category: CategorySource,
			Err:      err,
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = sourceErr.Error()
		}
	})

	b.Run("FileSource_wrapError", func(b *testing.B) {
		source := &FileSource{filePath: "/test/config.yaml"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			source.wrapError("load", "file_read", err)
		}
	})

	b.Run("FileSource_categorizeError", func(b *testing.B) {
		source := &FileSource{filePath: "/test/config.yaml"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			source.categorizeError("file_read", err)
		}
	})
}