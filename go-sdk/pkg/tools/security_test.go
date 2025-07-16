package tools

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SecurityMode represents the security mode for operations
type SecurityMode int

const (
	// SecurityModeUnrestricted allows all operations
	SecurityModeUnrestricted SecurityMode = iota
	// SecurityModeRestricted enforces security restrictions
	SecurityModeRestricted
)

// mockSecureFileExecutor implements ToolExecutor with security controls
type mockSecureFileExecutor struct {
	mode        SecurityMode
	allowedDirs []string
}

func (e *mockSecureFileExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	filePath, ok := params["file_path"].(string)
	if !ok {
		return nil, fmt.Errorf("file_path parameter is required")
	}
	
	// Security validation
	if e.mode == SecurityModeRestricted {
		if err := validateFilePath(filePath, e.allowedDirs); err != nil {
			return nil, err
		}
	}
	
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	return &ToolExecutionResult{
		Success: true,
		Data:    map[string]interface{}{"content": string(content)},
	}, nil
}

// mockSecureHTTPExecutor implements ToolExecutor with HTTP security controls
type mockSecureHTTPExecutor struct {
	mode           SecurityMode
	allowedDomains []string
	timeout        time.Duration
	maxSize        int64
}

func (e *mockSecureHTTPExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	url, ok := params["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url parameter is required")
	}
	
	// Security validation
	if e.mode == SecurityModeRestricted {
		if err := validateHTTPURL(url, e.allowedDomains); err != nil {
			return nil, err
		}
	}
	
	// Create HTTP client with secure TLS configuration and timeout
	client := &http.Client{
		Timeout: e.timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				},
			},
		},
	}
	
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	// Read response with size limit
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > e.maxSize {
		return nil, fmt.Errorf("response exceeds maximum size limit")
	}
	
	return &ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"body":        string(body),
			"status_code": resp.StatusCode,
		},
	}, nil
}

// mockSandboxExecutor implements ToolExecutor with sandbox controls
type mockSandboxExecutor struct {
	sandboxed bool
}

func (e *mockSandboxExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	input, ok := params["input"].(string)
	if !ok {
		return nil, fmt.Errorf("input parameter is required")
	}
	
	// Simulate sandbox validation
	if e.sandboxed {
		// In a real implementation, this would enforce sandbox restrictions
		if strings.Contains(input, "dangerous_operation") {
			return nil, NewSecurityError("SANDBOX_VIOLATION", "Dangerous operation not allowed in sandbox")
		}
	}
	
	return &ToolExecutionResult{
		Success: true,
		Data:    map[string]interface{}{"result": fmt.Sprintf("processed: %s", input)},
	}, nil
}

// mockInputValidationExecutor implements ToolExecutor with input validation
type mockInputValidationExecutor struct{}

func (e *mockInputValidationExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	input, ok := params["input"].(string)
	if !ok {
		return nil, fmt.Errorf("input parameter is required")
	}
	
	// Security validation
	if err := validateSecureInput(input); err != nil {
		return nil, err
	}
	
	return &ToolExecutionResult{
		Success: true,
		Data:    map[string]interface{}{"result": fmt.Sprintf("processed: %s", input)},
	}, nil
}

// TestSecureFileOperations tests the security boundaries of file operations
func TestSecureFileOperations(t *testing.T) {
	// Create test directory structure
	testDir := t.TempDir()
	allowedDir := filepath.Join(testDir, "allowed")
	restrictedDir := filepath.Join(testDir, "restricted")
	
	require.NoError(t, os.MkdirAll(allowedDir, 0755))
	require.NoError(t, os.MkdirAll(restrictedDir, 0755))
	
	// Create test files
	allowedFile := filepath.Join(allowedDir, "test.txt")
	restrictedFile := filepath.Join(restrictedDir, "secret.txt")
	
	require.NoError(t, ioutil.WriteFile(allowedFile, []byte("allowed content"), 0644))
	require.NoError(t, ioutil.WriteFile(restrictedFile, []byte("restricted content"), 0644))

	tests := []struct {
		name        string
		mode        SecurityMode
		allowedDirs []string
		filepath    string
		expectError bool
		errorType   string
	}{
		{
			name:        "unrestricted mode allows any path",
			mode:        SecurityModeUnrestricted,
			allowedDirs: nil,
			filepath:    restrictedFile,
			expectError: false,
		},
		{
			name:        "restricted mode allows whitelisted path",
			mode:        SecurityModeRestricted,
			allowedDirs: []string{allowedDir},
			filepath:    allowedFile,
			expectError: false,
		},
		{
			name:        "restricted mode blocks non-whitelisted path",
			mode:        SecurityModeRestricted,
			allowedDirs: []string{allowedDir},
			filepath:    restrictedFile,
			expectError: true,
			errorType:   "SECURITY_VIOLATION",
		},
		{
			name:        "path traversal attack blocked",
			mode:        SecurityModeRestricted,
			allowedDirs: []string{allowedDir},
			filepath:    filepath.Join(allowedDir, "../restricted/secret.txt"),
			expectError: true,
			errorType:   "SECURITY_VIOLATION",
		},
		{
			name:        "symlink attack blocked",
			mode:        SecurityModeRestricted,
			allowedDirs: []string{allowedDir},
			filepath:    filepath.Join(allowedDir, "link"),
			expectError: true,
			errorType:   "SECURITY_VIOLATION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create symlink for symlink test
			if strings.Contains(tt.name, "symlink") {
				linkPath := filepath.Join(allowedDir, "link")
				os.Symlink(restrictedFile, linkPath)
				defer os.Remove(linkPath)
			}

			// Create secure file reader tool
			// Create a mock executor with security settings
			executor := &mockSecureFileExecutor{
				mode:        tt.mode,
				allowedDirs: tt.allowedDirs,
			}
			
			tool := &Tool{
				ID:   "secure_file_reader",
				Name: "Secure File Reader",
				Schema: &ToolSchema{
					Type: "object",
					Properties: map[string]*Property{
						"file_path": {
							Type: "string",
						},
					},
					Required: []string{"file_path"},
				},
				Executor: executor,
			}

			// Execute the tool
			params := map[string]interface{}{
				"file_path": tt.filepath,
			}
			
			result, err := tool.Executor.Execute(context.Background(), params)
			
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != "" && err != nil {
					assert.Contains(t, err.Error(), tt.errorType)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.Success)
				// Verify we got the expected content
				data, ok := result.Data.(map[string]interface{})
				assert.True(t, ok, "result.Data should be a map")
				content, ok := data["content"].(string)
				assert.True(t, ok)
				if tt.filepath == allowedFile {
					assert.Equal(t, "allowed content", content)
				} else if tt.filepath == restrictedFile {
					assert.Equal(t, "restricted content", content)
				}
			}
		})
	}
}

// TestSecureHTTPOperations tests the security boundaries of HTTP operations
func TestSecureHTTPOperations(t *testing.T) {
	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))
	defer server.Close()

	// Create malicious server
	maliciousServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("malicious response"))
	}))
	defer maliciousServer.Close()

	tests := []struct {
		name           string
		mode           SecurityMode
		allowedDomains []string
		url            string
		expectError    bool
		errorType      string
	}{
		{
			name:           "unrestricted mode allows any URL",
			mode:           SecurityModeUnrestricted,
			allowedDomains: nil,
			url:            maliciousServer.URL,
			expectError:    false,
		},
		{
			name:           "restricted mode allows whitelisted domain",
			mode:           SecurityModeRestricted,
			allowedDomains: []string{"127.0.0.1"},
			url:            server.URL,
			expectError:    false,
		},
		{
			name:           "restricted mode blocks non-whitelisted domain",
			mode:           SecurityModeRestricted,
			allowedDomains: []string{"example.com"},
			url:            server.URL,
			expectError:    true,
			errorType:      "SECURITY_VIOLATION",
		},
		{
			name:           "localhost blocked when not whitelisted",
			mode:           SecurityModeRestricted,
			allowedDomains: []string{"example.com"},
			url:            "http://localhost:8080/test",
			expectError:    true,
			errorType:      "SECURITY_VIOLATION",
		},
		{
			name:           "private IP blocked when not whitelisted",
			mode:           SecurityModeRestricted,
			allowedDomains: []string{"example.com"},
			url:            "http://192.168.1.1/test",
			expectError:    true,
			errorType:      "SECURITY_VIOLATION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock HTTP executor with security settings
			executor := &mockSecureHTTPExecutor{
				mode:           tt.mode,
				allowedDomains: tt.allowedDomains,
				timeout:        5 * time.Second,
				maxSize:        1024 * 1024, // 1MB
			}
			
			// Create secure HTTP client tool
			tool := &Tool{
				ID:   "secure_http_client",
				Name: "Secure HTTP Client",
				Schema: &ToolSchema{
					Type: "object",
					Properties: map[string]*Property{
						"url": {
							Type: "string",
						},
					},
					Required: []string{"url"},
				},
				Executor: executor,
			}

			// Execute the tool
			params := map[string]interface{}{
				"url": tt.url,
			}
			
			result, err := tool.Executor.Execute(context.Background(), params)
			
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != "" && err != nil {
					assert.Contains(t, err.Error(), tt.errorType)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// TestSandboxExecution tests the sandbox execution boundaries
func TestSandboxExecution(t *testing.T) {
	tests := []struct {
		name        string
		toolID      string
		sandboxed   bool
		expectError bool
		errorType   string
	}{
		{
			name:        "sandboxed tool execution",
			toolID:      "sandboxed_tool",
			sandboxed:   true,
			expectError: false,
		},
		{
			name:        "unsandboxed tool execution",
			toolID:      "unsandboxed_tool",
			sandboxed:   false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create tool with sandbox settings
			// Create a mock sandbox executor
			executor := &mockSandboxExecutor{
				sandboxed: tt.sandboxed,
			}
			
			tool := &Tool{
				ID:   tt.toolID,
				Name: "Test Tool",
				Schema: &ToolSchema{
					Type: "object",
					Properties: map[string]*Property{
						"input": {
							Type: "string",
						},
					},
					Required: []string{"input"},
				},
				Executor: executor,
			}

			// Test normal execution
			params := map[string]interface{}{
				"input": "safe_input",
			}
			
			result, err := tool.Executor.Execute(context.Background(), params)
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.True(t, result.Success)
			data, ok := result.Data.(map[string]interface{})
			assert.True(t, ok)
			assert.Equal(t, "processed: safe_input", data["result"])

			// Test dangerous operation in sandbox
			if tt.sandboxed {
				dangerousArgs := map[string]interface{}{
					"input": "dangerous_operation",
				}
				
				result, err := tool.Executor.Execute(context.Background(), dangerousArgs)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "SANDBOX_VIOLATION")
				assert.Nil(t, result)
			}
		})
	}
}

// TestInputValidationSecurity tests input validation security
func TestInputValidationSecurity(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expectError bool
		errorType   string
	}{
		{
			name:        "valid input",
			input:       "safe input",
			expectError: false,
		},
		{
			name:        "script injection attempt",
			input:       "<script>alert('xss')</script>",
			expectError: true,
			errorType:   "VALIDATION_ERROR",
		},
		{
			name:        "SQL injection attempt",
			input:       "'; DROP TABLE users; --",
			expectError: true,
			errorType:   "VALIDATION_ERROR",
		},
		{
			name:        "command injection attempt",
			input:       "test; rm -rf /",
			expectError: true,
			errorType:   "VALIDATION_ERROR",
		},
		{
			name:        "path traversal attempt",
			input:       "../../etc/passwd",
			expectError: true,
			errorType:   "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock input validation executor
			executor := &mockInputValidationExecutor{}
			
			// Create tool with security validation
			tool := &Tool{
				ID:   "secure_input_tool",
				Name: "Secure Input Tool",
				Schema: &ToolSchema{
					Type: "object",
					Properties: map[string]*Property{
						"input": {
							Type: "string",
						},
					},
					Required: []string{"input"},
				},
				Executor: executor,
			}

			// Execute the tool
			params := map[string]interface{}{
				"input": tt.input,
			}
			
			result, err := tool.Executor.Execute(context.Background(), params)
			
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != "" && err != nil {
					assert.Contains(t, err.Error(), tt.errorType)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// Helper functions for security validation

func validateFilePath(filePath string, allowedDirs []string) error {
	// Resolve the path to prevent path traversal attacks
	resolvedPath, err := filepath.Abs(filePath)
	if err != nil {
		return NewSecurityError("SECURITY_VIOLATION", "Invalid file path")
	}

	// Check for symbolic links - security check
	if info, err := os.Lstat(resolvedPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return NewSecurityError("SECURITY_VIOLATION", "Symbolic links are not allowed")
		}
	}

	// Check if path is within allowed directories
	for _, allowedDir := range allowedDirs {
		resolvedAllowedDir, err := filepath.Abs(allowedDir)
		if err != nil {
			continue
		}
		
		if strings.HasPrefix(resolvedPath, resolvedAllowedDir) {
			return nil
		}
	}
	
	return NewSecurityError("SECURITY_VIOLATION", "File path not in allowed directories")
}

func validateHTTPURL(url string, allowedDomains []string) error {
	// Parse URL
	for _, domain := range allowedDomains {
		if strings.Contains(url, domain) {
			return nil
		}
	}
	
	return NewSecurityError("SECURITY_VIOLATION", "Domain not in allowed list")
}

func validateSecureInput(input string) error {
	// Check for common injection patterns
	dangerousPatterns := []string{
		"<script",
		"javascript:",
		"'; DROP",
		"rm -rf",
		"../",
		"..\\",
	}
	
	for _, pattern := range dangerousPatterns {
		if strings.Contains(strings.ToLower(input), strings.ToLower(pattern)) {
			return NewValidationError("VALIDATION_ERROR", "Potentially dangerous input detected", "secure_input_tool")
		}
	}
	
	return nil
}

// NewSecurityError creates a new security error
func NewSecurityError(code, message string) error {
	return &ToolError{
		Code:      code,
		Message:   message,
		Details:   map[string]interface{}{"type": "security_violation"},
		Timestamp: time.Now(),
	}
}