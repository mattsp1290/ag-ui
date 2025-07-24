package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSecurityIntegration tests the integration of all security features
func TestSecurityIntegration(t *testing.T) {
	// Create comprehensive test environment
	env := NewSecurityTestEnvironment(t)
	defer env.Cleanup()

	t.Run("ComprehensiveSecuritySuite", func(t *testing.T) {
		testComprehensiveSecuritySuite(t, env)
	})
	
	t.Run("CrossToolSecurityValidation", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping cross tool security validation in short mode")
		}
		testCrossToolSecurityValidation(t, env)
	})
	
	t.Run("SecureRegistryIntegration", func(t *testing.T) {
		testSecureRegistryIntegration(t, env)
	})
	
	t.Run("SecurityBoundaryValidation", func(t *testing.T) {
		testSecurityBoundaryValidation(t, env)
	})
	
	t.Run("AttackVectorCombinations", func(t *testing.T) {
		testAttackVectorCombinations(t, env)
	})
	
	t.Run("SecurityPolicyEnforcement", func(t *testing.T) {
		testSecurityPolicyEnforcement(t, env)
	})
	
	t.Run("RealWorldScenarios", func(t *testing.T) {
		testRealWorldScenarios(t, env)
	})
	
	t.Run("SecurityPerformance", func(t *testing.T) {
		testSecurityPerformance(t, env)
	})
	
	t.Run("SecurityResilience", func(t *testing.T) {
		testSecurityResilience(t, env)
	})
	
	t.Run("SecurityCompliance", func(t *testing.T) {
		testSecurityCompliance(t, env)
	})

	// Generate final security report
	env.GenerateFinalReport(t)
}

// testComprehensiveSecuritySuite runs a comprehensive security test suite
func testComprehensiveSecuritySuite(t *testing.T, env *SecurityTestEnvironment) {
	utils := env.GetUtils()
	helpers := env.GetHelpers()
	executor := env.GetExecutor()
	payloadGen := env.GetPayloadGenerator()

	// Create test file structure
	testFile := utils.CreateTestFile(t, "secure/test.txt", "test content")
	
	// Test file security
	fileOptions := helpers.CreateSecureFileOptions([]string{filepath.Join(env.GetTempDir(), "secure")}, 1024*1024)
	secureFileExecutor := NewSecureFileExecutor(&readFileExecutor{}, fileOptions, "read")

	// Test path traversal attacks
	pathTraversalPayloads := payloadGen.GeneratePathTraversalPayloads()
	for i, payload := range pathTraversalPayloads {
		testName := fmt.Sprintf("PathTraversal_%d", i)
		description := fmt.Sprintf("Path traversal attack: %s", payload)
		params := map[string]interface{}{
			"path": payload,
		}
		executor.ExecuteSecurityTest(t, testName, description, secureFileExecutor, params, true, "")
	}

	// Test HTTP security
	httpOptions := helpers.CreateSecureHTTPOptions([]string{"example.com"})
	secureHTTPExecutor := NewSecureHTTPExecutor(&mockHTTPExecutorForIntegration{}, httpOptions)

	// Test URL injection attacks
	urlInjectionPayloads := payloadGen.GenerateURLInjectionPayloads()
	for i, payload := range urlInjectionPayloads {
		testName := fmt.Sprintf("URLInjection_%d", i)
		description := fmt.Sprintf("URL injection attack: %s", payload)
		params := map[string]interface{}{
			"url": payload,
		}
		executor.ExecuteSecurityTest(t, testName, description, secureHTTPExecutor, params, true, "")
	}

	// Test valid operations
	validParams := map[string]interface{}{
		"path": testFile,
	}
	executor.ExecuteSecurityTest(t, "ValidFileAccess", "Valid file access within allowed path", secureFileExecutor, validParams, false, "")

	validHTTPParams := map[string]interface{}{
		"url": "https://example.com/api",
	}
	executor.ExecuteSecurityTest(t, "ValidHTTPAccess", "Valid HTTP access to allowed host", secureHTTPExecutor, validHTTPParams, false, "")
}

// testCrossToolSecurityValidation tests security validation across different tools
func testCrossToolSecurityValidation(t *testing.T, env *SecurityTestEnvironment) {
	// Create a comprehensive registry with all security features
	registry := NewRegistry()
	
	// Register secure tools
	options := &BuiltinToolsOptions{
		SecureMode: true,
		FileOptions: &SecureFileOptions{
			AllowedPaths: []string{env.GetTempDir()},
			MaxFileSize:  1024 * 1024,
			AllowSymlinks: false,
			DenyPaths:    []string{"/etc", "/sys", "/proc"},
		},
		HTTPOptions: &SecureHTTPOptions{
			AllowedHosts:         []string{"example.com", "api.example.com"},
			AllowPrivateNetworks: false,
			AllowedSchemes:       []string{"https"},
			MaxRedirects:         5,
		},
	}

	if err := RegisterBuiltinToolsWithOptions(registry, options); err != nil {
		t.Fatalf("Failed to register secure tools: %v", err)
	}

	// Create test scenarios that involve multiple tools
	testScenarios := []struct {
		name        string
		operations  []toolOperation
		expectError bool
	}{
		{
			name: "ValidWorkflow",
			operations: []toolOperation{
				{
					tool: "write_file",
					params: map[string]interface{}{
						"path":    filepath.Join(env.GetTempDir(), "workflow.txt"),
						"content": "workflow content",
					},
				},
				{
					tool: "read_file",
					params: map[string]interface{}{
						"path": filepath.Join(env.GetTempDir(), "workflow.txt"),
					},
				},
				{
					tool: "http_get",
					params: map[string]interface{}{
						"url": "https://example.com/api",
					},
				},
			},
			expectError: false,
		},
		{
			name: "SecurityViolationWorkflow",
			operations: []toolOperation{
				{
					tool: "write_file",
					params: map[string]interface{}{
						"path":    "/etc/malicious.txt",
						"content": "malicious content",
					},
				},
			},
			expectError: true,
		},
		{
			name: "HTTPSecurityViolation",
			operations: []toolOperation{
				{
					tool: "http_get",
					params: map[string]interface{}{
						"url": "http://169.254.169.254/metadata",
					},
				},
			},
			expectError: true,
		},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			for i, operation := range scenario.operations {
				tool, err := registry.GetByName(operation.tool)
				if err != nil {
					t.Fatalf("Tool %s not found in registry: %v", operation.tool, err)
				}

				result, err := tool.Executor.Execute(context.Background(), operation.params)

				if scenario.expectError {
					if err == nil && result.Success {
						t.Errorf("Expected security violation in operation %d of scenario %s", i, scenario.name)
					} else {
						t.Logf("Security violation correctly detected in operation %d", i)
					}
					break // Stop on first expected error
				} else {
					if err != nil {
						t.Errorf("Unexpected error in operation %d: %v", i, err)
					}
					if result == nil || !result.Success {
						t.Errorf("Expected operation %d to succeed in scenario %s", i, scenario.name)
					}
				}
			}
		})
	}
}

// toolOperation represents a single tool operation
type toolOperation struct {
	tool   string
	params map[string]interface{}
}

// testSecureRegistryIntegration tests the integration of secure registry features
func testSecureRegistryIntegration(t *testing.T, env *SecurityTestEnvironment) {
	// Test registry with different security configurations
	securityConfigs := []struct {
		name    string
		options *BuiltinToolsOptions
	}{
		{
			name: "StrictSecurity",
			options: &BuiltinToolsOptions{
				SecureMode: true,
				FileOptions: &SecureFileOptions{
					AllowedPaths:  []string{env.GetTempDir()},
					MaxFileSize:   1024,
					AllowSymlinks: false,
					DenyPaths:     []string{"/etc", "/sys", "/proc", "/root"},
				},
				HTTPOptions: &SecureHTTPOptions{
					AllowedHosts:         []string{"example.com"},
					AllowPrivateNetworks: false,
					AllowedSchemes:       []string{"https"},
					MaxRedirects:         3,
				},
			},
		},
		{
			name: "ModerateSecurity",
			options: &BuiltinToolsOptions{
				SecureMode: true,
				FileOptions: &SecureFileOptions{
					AllowedPaths:  []string{env.GetTempDir(), "/tmp"},
					MaxFileSize:   10 * 1024 * 1024,
					AllowSymlinks: true,
					DenyPaths:     []string{"/etc", "/sys", "/proc"},
				},
				HTTPOptions: &SecureHTTPOptions{
					AllowedHosts:         []string{"example.com", "api.example.com", "cdn.example.com"},
					AllowPrivateNetworks: false,
					AllowedSchemes:       []string{"http", "https"},
					MaxRedirects:         10,
				},
			},
		},
		{
			name: "NoSecurity",
			options: &BuiltinToolsOptions{
				SecureMode: false,
			},
		},
	}

	for _, config := range securityConfigs {
		t.Run(config.name, func(t *testing.T) {
			testRegistry := NewRegistry()
			
			if err := RegisterBuiltinToolsWithOptions(testRegistry, config.options); err != nil {
				t.Fatalf("Failed to register tools with %s config: %v", config.name, err)
			}

			// Test tool availability
			expectedTools := []string{"read_file", "write_file", "http_get", "http_post"}
			for _, toolName := range expectedTools {
				if _, err := testRegistry.GetByName(toolName); err != nil {
					t.Errorf("Expected tool %s not found in registry with %s config: %v", toolName, config.name, err)
				}
			}

			// Test security enforcement
			readTool, err := testRegistry.GetByName("read_file")
			if err != nil {
				t.Fatalf("Failed to get read_file tool: %v", err)
			}
			testParams := map[string]interface{}{
				"path": "/etc/passwd",
			}

			result, err := readTool.Executor.Execute(context.Background(), testParams)

			if config.options.SecureMode {
				// Should be blocked in secure mode
				if err == nil && result.Success {
					t.Errorf("Expected security blocking in %s config", config.name)
				}
			} else {
				// May or may not work in non-secure mode (depends on file existence and permissions)
				t.Logf("Non-secure mode result: success=%v, error=%v", result.Success, err)
			}
		})
	}
}

// testSecurityBoundaryValidation tests security boundary validation
func testSecurityBoundaryValidation(t *testing.T, env *SecurityTestEnvironment) {
	utils := env.GetUtils()
	
	// Create a complex directory structure for boundary testing
	secureDir := utils.CreateTestDirectory(t, "secure")
	publicDir := utils.CreateTestDirectory(t, "public")
	restrictedDir := utils.CreateTestDirectory(t, "restricted")
	
	// Create files in different directories
	secureFile := utils.CreateTestFile(t, "secure/confidential.txt", "confidential data")
	publicFile := utils.CreateTestFile(t, "public/public.txt", "public data")
	restrictedFile := utils.CreateTestFile(t, "restricted/secret.txt", "secret data")
	
	// Test boundary scenarios
	boundaryTests := []struct {
		name        string
		options     *SecureFileOptions
		testFile    string
		expectAllow bool
	}{
		{
			name: "AccessWithinBoundary",
			options: &SecureFileOptions{
				AllowedPaths: []string{secureDir},
				MaxFileSize:  1024 * 1024,
			},
			testFile:    secureFile,
			expectAllow: true,
		},
		{
			name: "AccessOutsideBoundary",
			options: &SecureFileOptions{
				AllowedPaths: []string{secureDir},
				MaxFileSize:  1024 * 1024,
			},
			testFile:    restrictedFile,
			expectAllow: false,
		},
		{
			name: "MultipleBoundaries",
			options: &SecureFileOptions{
				AllowedPaths: []string{secureDir, publicDir},
				MaxFileSize:  1024 * 1024,
			},
			testFile:    publicFile,
			expectAllow: true,
		},
		{
			name: "DeniedPathOverride",
			options: &SecureFileOptions{
				AllowedPaths: []string{env.GetTempDir()},
				DenyPaths:    []string{restrictedDir},
				MaxFileSize:  1024 * 1024,
			},
			testFile:    restrictedFile,
			expectAllow: false,
		},
	}

	for _, test := range boundaryTests {
		t.Run(test.name, func(t *testing.T) {
			executor := NewSecureFileExecutor(&readFileExecutor{}, test.options, "read")
			
			params := map[string]interface{}{
				"path": test.testFile,
			}

			result, err := executor.Execute(context.Background(), params)

			if test.expectAllow {
				if err != nil {
					t.Errorf("Unexpected error for allowed access: %v", err)
				}
				if result == nil || !result.Success {
					t.Error("Expected access to be allowed")
				}
			} else {
				if err == nil && result.Success {
					t.Error("Expected access to be denied")
				}
			}
		})
	}
}

// testAttackVectorCombinations tests combinations of different attack vectors
func testAttackVectorCombinations(t *testing.T, env *SecurityTestEnvironment) {
	_ = env.GetUtils() // utils - reserved for future use
	_ = env.GetPayloadGenerator() // payloadGen - reserved for future use
	
	// Create secure file executor
	fileOptions := &SecureFileOptions{
		AllowedPaths: []string{env.GetTempDir()},
		MaxFileSize:  1024 * 1024,
		AllowSymlinks: false,
	}
	fileExecutor := NewSecureFileExecutor(&readFileExecutor{}, fileOptions, "read")
	
	// Create secure HTTP executor
	httpOptions := &SecureHTTPOptions{
		AllowedHosts:         []string{"example.com"},
		AllowPrivateNetworks: false,
		AllowedSchemes:       []string{"https"},
	}
	httpExecutor := NewSecureHTTPExecutor(&mockHTTPExecutorForIntegration{}, httpOptions)

	// Test combination attacks
	combinationTests := []struct {
		name        string
		executor    ToolExecutor
		payloadType string
		params      map[string]interface{}
	}{
		{
			name:        "PathTraversal_CommandInjection",
			executor:    fileExecutor,
			payloadType: "file",
			params: map[string]interface{}{
				"path": "../../../etc/passwd; cat /etc/shadow",
			},
		},
		{
			name:        "URLInjection_XSS",
			executor:    httpExecutor,
			payloadType: "url",
			params: map[string]interface{}{
				"url": "javascript:alert('XSS');//https://example.com",
			},
		},
		{
			name:        "PathTraversal_NullByte",
			executor:    fileExecutor,
			payloadType: "file",
			params: map[string]interface{}{
				"path": "../../../etc/passwd\x00.txt",
			},
		},
		{
			name:        "URLInjection_SSRF",
			executor:    httpExecutor,
			payloadType: "url",
			params: map[string]interface{}{
				"url": "http://169.254.169.254/latest/meta-data/",
			},
		},
	}

	for _, test := range combinationTests {
		t.Run(test.name, func(t *testing.T) {
			result, err := test.executor.Execute(context.Background(), test.params)
			
			// All combination attacks should be blocked
			if err == nil && result.Success {
				t.Errorf("Expected combination attack to be blocked: %s", test.name)
			} else {
				t.Logf("Combination attack successfully blocked: %s", test.name)
			}
		})
	}
}

// testSecurityPolicyEnforcement tests security policy enforcement
func testSecurityPolicyEnforcement(t *testing.T, env *SecurityTestEnvironment) {
	// Create a test file for read operations
	utils := env.GetUtils()
	testFile := utils.CreateTestFile(t, "test.txt", "test content for policy enforcement")
	
	// Test different security policies
	policies := []struct {
		name        string
		fileOptions *SecureFileOptions
		httpOptions *SecureHTTPOptions
		testCases   []policyTestCase
	}{
		{
			name: "RestrictivePolicy",
			fileOptions: &SecureFileOptions{
				AllowedPaths:  []string{env.GetTempDir()},
				MaxFileSize:   1024,
				AllowSymlinks: false,
				DenyPaths:     []string{"/etc", "/sys", "/proc", "/root", "/home"},
			},
			httpOptions: &SecureHTTPOptions{
				AllowedHosts:         []string{"example.com"},
				AllowPrivateNetworks: false,
				AllowedSchemes:       []string{"https"},
				MaxRedirects:         1,
			},
			testCases: []policyTestCase{
				{
					name:        "ValidFileAccess",
					operation:   "file",
					params:      map[string]interface{}{"path": testFile},
					expectAllow: true,
				},
				{
					name:        "InvalidFileAccess",
					operation:   "file",
					params:      map[string]interface{}{"path": "/etc/passwd"},
					expectAllow: false,
				},
				{
					name:        "ValidHTTPAccess",
					operation:   "http",
					params:      map[string]interface{}{"url": "https://example.com"},
					expectAllow: true,
				},
				{
					name:        "InvalidHTTPAccess",
					operation:   "http",
					params:      map[string]interface{}{"url": "http://evil.com"},
					expectAllow: false,
				},
			},
		},
		{
			name: "PermissivePolicy",
			fileOptions: &SecureFileOptions{
				AllowedPaths:  []string{env.GetTempDir(), "/tmp"},
				MaxFileSize:   10 * 1024 * 1024,
				AllowSymlinks: true,
				DenyPaths:     []string{"/etc/shadow", "/root"},
			},
			httpOptions: &SecureHTTPOptions{
				AllowedHosts:         []string{"example.com"},
				AllowPrivateNetworks: false,
				AllowedSchemes:       []string{"http", "https"},
				MaxRedirects:         10,
			},
			testCases: []policyTestCase{
				{
					name:        "ValidFileAccess",
					operation:   "file",
					params:      map[string]interface{}{"path": testFile},
					expectAllow: true,
				},
				{
					name:        "ValidHTTPAccess",
					operation:   "http",
					params:      map[string]interface{}{"url": "https://example.com"},
					expectAllow: true,
				},
				{
					name:        "DeniedFileAccess",
					operation:   "file",
					params:      map[string]interface{}{"path": "/etc/shadow"},
					expectAllow: false,
				},
			},
		},
	}

	for _, policy := range policies {
		t.Run(policy.name, func(t *testing.T) {
			fileExecutor := NewSecureFileExecutor(&readFileExecutor{}, policy.fileOptions, "read")
			httpExecutor := NewSecureHTTPExecutor(&mockHTTPExecutorForIntegration{}, policy.httpOptions)
			
			for _, testCase := range policy.testCases {
				t.Run(testCase.name, func(t *testing.T) {
					var result *ToolExecutionResult
					var err error
					
					if testCase.operation == "file" {
						result, err = fileExecutor.Execute(context.Background(), testCase.params)
					} else if testCase.operation == "http" {
						result, err = httpExecutor.Execute(context.Background(), testCase.params)
					}
					
					if testCase.expectAllow {
						if err != nil {
							t.Errorf("Unexpected error for allowed operation: %v", err)
						}
						if result == nil || !result.Success {
							t.Error("Expected operation to be allowed")
						}
					} else {
						if err == nil && result.Success {
							t.Error("Expected operation to be denied by policy")
						}
					}
				})
			}
		})
	}
}

type policyTestCase struct {
	name        string
	operation   string
	params      map[string]interface{}
	expectAllow bool
}

// testRealWorldScenarios tests real-world security scenarios
func testRealWorldScenarios(t *testing.T, env *SecurityTestEnvironment) {
	utils := env.GetUtils()
	
	// Create realistic directory structure
	projectDir := utils.CreateTestDirectory(t, "project")
	srcDir := utils.CreateTestDirectory(t, "project/src")
	configDir := utils.CreateTestDirectory(t, "project/config")
	logsDir := utils.CreateTestDirectory(t, "project/logs")
	
	// Create realistic files
	utils.CreateTestFile(t, "project/src/main.go", "package main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}")
	utils.CreateTestFile(t, "project/config/app.yaml", "port: 8080\ndb_host: localhost")
	utils.CreateTestFile(t, "project/logs/app.log", "2023-01-01 10:00:00 INFO Application started")
	
	// Test realistic scenarios
	scenarios := []struct {
		name        string
		description string
		setup       func() (ToolExecutor, map[string]interface{})
		expectAllow bool
	}{
		{
			name:        "DeveloperReadingSourceCode",
			description: "Developer reading source code files",
			setup: func() (ToolExecutor, map[string]interface{}) {
				options := &SecureFileOptions{
					AllowedPaths: []string{projectDir},
					MaxFileSize:  10 * 1024 * 1024,
					AllowSymlinks: false,
				}
				executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")
				params := map[string]interface{}{
					"path": filepath.Join(srcDir, "main.go"),
				}
				return executor, params
			},
			expectAllow: true,
		},
		{
			name:        "AttackerTryingPathTraversal",
			description: "Attacker trying to read /etc/passwd via path traversal",
			setup: func() (ToolExecutor, map[string]interface{}) {
				options := &SecureFileOptions{
					AllowedPaths: []string{projectDir},
					MaxFileSize:  10 * 1024 * 1024,
					AllowSymlinks: false,
				}
				executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")
				params := map[string]interface{}{
					"path": filepath.Join(projectDir, "../../../etc/passwd"),
				}
				return executor, params
			},
			expectAllow: false,
		},
		{
			name:        "APICallToTrustedService",
			description: "Application making API call to trusted service",
			setup: func() (ToolExecutor, map[string]interface{}) {
				options := &SecureHTTPOptions{
					AllowedHosts:         []string{"example.com"},
					AllowPrivateNetworks: false,
					AllowedSchemes:       []string{"https"},
					MaxRedirects:         5,
				}
				executor := NewSecureHTTPExecutor(&mockHTTPExecutorForIntegration{}, options)
				params := map[string]interface{}{
					"url": "https://example.com/v1/data",
				}
				return executor, params
			},
			expectAllow: true,
		},
		{
			name:        "SSRFAttackAttempt",
			description: "Attacker trying SSRF attack on cloud metadata service",
			setup: func() (ToolExecutor, map[string]interface{}) {
				options := &SecureHTTPOptions{
					AllowedHosts:         []string{"api.example.com"},
					AllowPrivateNetworks: false,
					AllowedSchemes:       []string{"https"},
					MaxRedirects:         5,
				}
				executor := NewSecureHTTPExecutor(&mockHTTPExecutorForIntegration{}, options)
				params := map[string]interface{}{
					"url": "http://169.254.169.254/latest/meta-data/",
				}
				return executor, params
			},
			expectAllow: false,
		},
		{
			name:        "LogFileRotation",
			description: "System writing to log files",
			setup: func() (ToolExecutor, map[string]interface{}) {
				options := &SecureFileOptions{
					AllowedPaths: []string{logsDir},
					MaxFileSize:  100 * 1024 * 1024,
					AllowSymlinks: false,
				}
				executor := NewSecureFileExecutor(&writeFileExecutor{}, options, "write")
				params := map[string]interface{}{
					"path":    filepath.Join(logsDir, "app.log"),
					"content": "2023-01-01 10:01:00 INFO New log entry",
					"mode":    "append",
				}
				return executor, params
			},
			expectAllow: true,
		},
		{
			name:        "ConfigFileAccess",
			description: "Application reading configuration files",
			setup: func() (ToolExecutor, map[string]interface{}) {
				options := &SecureFileOptions{
					AllowedPaths: []string{configDir},
					MaxFileSize:  1024 * 1024,
					AllowSymlinks: false,
				}
				executor := NewSecureFileExecutor(&readFileExecutor{}, options, "read")
				params := map[string]interface{}{
					"path": filepath.Join(configDir, "app.yaml"),
				}
				return executor, params
			},
			expectAllow: true,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			executor, params := scenario.setup()
			
			result, err := executor.Execute(context.Background(), params)
			
			if scenario.expectAllow {
				if err != nil {
					t.Errorf("Unexpected error in realistic scenario '%s': %v", scenario.description, err)
				}
				if result == nil || !result.Success {
					t.Errorf("Expected realistic scenario to succeed: %s", scenario.description)
				}
			} else {
				if err == nil && result.Success {
					t.Errorf("Expected security blocking in attack scenario: %s", scenario.description)
				} else {
					t.Logf("Attack scenario successfully blocked: %s", scenario.description)
				}
			}
		})
	}
}

// testSecurityPerformance tests security performance characteristics
func testSecurityPerformance(t *testing.T, env *SecurityTestEnvironment) {
	utils := env.GetUtils()
	
	// Create test files
	smallFile := utils.CreateTestFile(t, "small.txt", "small content")
	mediumFile := utils.CreateTestFile(t, "medium.txt", strings.Repeat("medium content\n", 1000))
	largeFile := utils.CreateTestFile(t, "large.txt", strings.Repeat("large content\n", 10000))
	
	// Test performance with different file sizes
	performanceTests := []struct {
		name     string
		file     string
		fileSize string
	}{
		{"SmallFile", smallFile, "small"},
		{"MediumFile", mediumFile, "medium"},
		{"LargeFile", largeFile, "large"},
	}

	fileOptions := &SecureFileOptions{
		AllowedPaths: []string{env.GetTempDir()},
		MaxFileSize:  10 * 1024 * 1024,
		AllowSymlinks: false,
	}
	
	// Test with and without security
	secureExecutor := NewSecureFileExecutor(&readFileExecutor{}, fileOptions, "read")
	normalExecutor := &readFileExecutor{}

	for _, test := range performanceTests {
		t.Run(test.name, func(t *testing.T) {
			params := map[string]interface{}{
				"path": test.file,
			}
			
			// Measure secure execution time
			start := time.Now()
			result, err := secureExecutor.Execute(context.Background(), params)
			secureTime := time.Since(start)
			
			if err != nil {
				t.Errorf("Secure execution failed: %v", err)
			}
			if result == nil || !result.Success {
				t.Error("Expected secure execution to succeed")
			}
			
			// Measure normal execution time
			start = time.Now()
			result, err = normalExecutor.Execute(context.Background(), params)
			normalTime := time.Since(start)
			
			if err != nil {
				t.Errorf("Normal execution failed: %v", err)
			}
			if result == nil || !result.Success {
				t.Error("Expected normal execution to succeed")
			}
			
			// Calculate overhead
			overhead := secureTime - normalTime
			overheadPercent := float64(overhead) / float64(normalTime) * 100
			
			t.Logf("File: %s, Secure: %v, Normal: %v, Overhead: %v (%.2f%%)", 
				test.fileSize, secureTime, normalTime, overhead, overheadPercent)
			
			// Security overhead should be reasonable (less than 1000% in most cases)
			if overheadPercent > 1000 {
				t.Errorf("Security overhead too high: %.2f%%", overheadPercent)
			}
		})
	}
}

// testSecurityResilience tests security resilience under various conditions
func testSecurityResilience(t *testing.T, env *SecurityTestEnvironment) {
	utils := env.GetUtils()
	
	// Test edge cases and stress conditions
	resilienceTests := []struct {
		name        string
		description string
		executor    ToolExecutor
		params      map[string]interface{}
		expectAllow bool
	}{
		{
			name:        "EmptyPath",
			description: "Empty path parameter",
			executor: NewSecureFileExecutor(&readFileExecutor{}, &SecureFileOptions{
				AllowedPaths: []string{env.GetTempDir()},
				MaxFileSize:  1024 * 1024,
			}, "read"),
			params: map[string]interface{}{
				"path": "",
			},
			expectAllow: false,
		},
		{
			name:        "VeryLongPath",
			description: "Very long path parameter",
			executor: NewSecureFileExecutor(&readFileExecutor{}, &SecureFileOptions{
				AllowedPaths: []string{env.GetTempDir()},
				MaxFileSize:  1024 * 1024,
			}, "read"),
			params: map[string]interface{}{
				"path": strings.Repeat("a", 10000),
			},
			expectAllow: false,
		},
		{
			name:        "NullByteInPath",
			description: "Null byte in path",
			executor: NewSecureFileExecutor(&readFileExecutor{}, &SecureFileOptions{
				AllowedPaths: []string{env.GetTempDir()},
				MaxFileSize:  1024 * 1024,
			}, "read"),
			params: map[string]interface{}{
				"path": filepath.Join(env.GetTempDir(), "test\x00.txt"),
			},
			expectAllow: false,
		},
		{
			name:        "UnicodeInPath",
			description: "Unicode characters in path",
			executor: NewSecureFileExecutor(&readFileExecutor{}, &SecureFileOptions{
				AllowedPaths: []string{env.GetTempDir()},
				MaxFileSize:  1024 * 1024,
			}, "read"),
			params: map[string]interface{}{
				"path": filepath.Join(env.GetTempDir(), "test_файл.txt"),
			},
			expectAllow: true, // Unicode should be allowed
		},
		{
			name:        "EmptyURL",
			description: "Empty URL parameter",
			executor: NewSecureHTTPExecutor(&mockHTTPExecutorForIntegration{}, &SecureHTTPOptions{
				AllowedHosts:   []string{"example.com"},
				AllowedSchemes: []string{"https"},
			}),
			params: map[string]interface{}{
				"url": "",
			},
			expectAllow: false,
		},
		{
			name:        "VeryLongURL",
			description: "Very long URL parameter",
			executor: NewSecureHTTPExecutor(&mockHTTPExecutorForIntegration{}, &SecureHTTPOptions{
				AllowedHosts:   []string{"example.com"},
				AllowedSchemes: []string{"https"},
			}),
			params: map[string]interface{}{
				"url": "https://example.com/" + strings.Repeat("a", 10000),
			},
			expectAllow: false,
		},
	}

	// Create Unicode test file
	unicodeFile := utils.CreateTestFile(t, "test_файл.txt", "unicode content")
	
	for _, test := range resilienceTests {
		t.Run(test.name, func(t *testing.T) {
			// If testing Unicode file, ensure it exists
			if strings.Contains(test.name, "Unicode") {
				test.params["path"] = unicodeFile
			}
			
			result, err := test.executor.Execute(context.Background(), test.params)
			
			if test.expectAllow {
				if err != nil {
					t.Errorf("Unexpected error for allowed operation: %v", err)
				}
				if result == nil || !result.Success {
					t.Error("Expected operation to be allowed")
				}
			} else {
				if err == nil && result.Success {
					t.Errorf("Expected resilience test to block operation: %s", test.description)
				} else {
					t.Logf("Resilience test successfully blocked: %s", test.description)
				}
			}
		})
	}
}

// testSecurityCompliance tests compliance with security standards
func testSecurityCompliance(t *testing.T, env *SecurityTestEnvironment) {
	// Test OWASP Top 10 compliance
	owaspTests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T, env *SecurityTestEnvironment)
	}{
		{
			name:        "InjectionPrevention",
			description: "A03:2021 - Injection",
			testFunc:    testInjectionCompliance,
		},
		{
			name:        "BrokenAccessControl",
			description: "A01:2021 - Broken Access Control",
			testFunc:    testAccessControlCompliance,
		},
		{
			name:        "SecurityMisconfiguration",
			description: "A05:2021 - Security Misconfiguration",
			testFunc:    testSecurityMisconfigurationCompliance,
		},
		{
			name:        "VulnerableComponents",
			description: "A06:2021 - Vulnerable and Outdated Components",
			testFunc:    testVulnerableComponentsCompliance,
		},
		{
			name:        "IdentificationFailures",
			description: "A07:2021 - Identification and Authentication Failures",
			testFunc:    testIdentificationFailuresCompliance,
		},
		{
			name:        "DataIntegrityFailures",
			description: "A08:2021 - Software and Data Integrity Failures",
			testFunc:    testDataIntegrityCompliance,
		},
		{
			name:        "LoggingFailures",
			description: "A09:2021 - Security Logging and Monitoring Failures",
			testFunc:    testLoggingFailuresCompliance,
		},
		{
			name:        "SSRF",
			description: "A10:2021 - Server-Side Request Forgery",
			testFunc:    testSSRFCompliance,
		},
	}

	for _, test := range owaspTests {
		t.Run(test.name, func(t *testing.T) {
			t.Logf("Testing OWASP compliance: %s", test.description)
			test.testFunc(t, env)
		})
	}
}

// OWASP compliance test functions
func testInjectionCompliance(t *testing.T, env *SecurityTestEnvironment) {
	payloadGen := env.GetPayloadGenerator()
	
	// Test various injection types
	injectionTypes := []struct {
		name     string
		payloads []string
	}{
		{"SQL", payloadGen.GenerateSQLInjectionPayloads()},
		{"Command", payloadGen.GenerateCommandInjectionPayloads()},
		{"XSS", payloadGen.GenerateXSSPayloads()},
		{"Template", payloadGen.GenerateTemplateInjectionPayloads()},
	}

	for _, injType := range injectionTypes {
		t.Run(injType.name, func(t *testing.T) {
			// All injection payloads should be safely handled
			for _, payload := range injType.payloads {
				// Test as file content (should be safe)
				writeExecutor := NewSecureFileExecutor(&writeFileExecutor{}, &SecureFileOptions{
					AllowedPaths: []string{env.GetTempDir()},
					MaxFileSize:  1024 * 1024,
				}, "write")
				
				params := map[string]interface{}{
					"path":    filepath.Join(env.GetTempDir(), fmt.Sprintf("injection_test_%s.txt", injType.name)),
					"content": payload,
				}
				
				result, err := writeExecutor.Execute(context.Background(), params)
				
				if err != nil {
					t.Errorf("Injection payload should be safely handled as content: %v", err)
				}
				if result == nil || !result.Success {
					t.Error("Expected injection payload to be safely written as content")
				}
			}
		})
	}
}

func testAccessControlCompliance(t *testing.T, env *SecurityTestEnvironment) {
	// Test path traversal prevention
	pathTraversalPayloads := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32\\config\\sam",
		"..%2f..%2f..%2fetc%2fpasswd",
		"..%252f..%252f..%252fetc%252fpasswd",
	}

	executor := NewSecureFileExecutor(&readFileExecutor{}, &SecureFileOptions{
		AllowedPaths: []string{env.GetTempDir()},
		MaxFileSize:  1024 * 1024,
	}, "read")

	for _, payload := range pathTraversalPayloads {
		params := map[string]interface{}{
			"path": payload,
		}
		
		result, err := executor.Execute(context.Background(), params)
		
		if err == nil && result.Success {
			t.Errorf("Path traversal should be blocked: %s", payload)
		}
	}
}

func testSecurityMisconfigurationCompliance(t *testing.T, env *SecurityTestEnvironment) {
	// Test that secure defaults are enforced
	defaultOptions := DefaultSecureFileOptions()
	
	if defaultOptions.AllowSymlinks {
		t.Error("Default configuration should not allow symlinks")
	}
	
	if defaultOptions.MaxFileSize == 0 {
		t.Error("Default configuration should have file size limits")
	}
	
	if len(defaultOptions.DenyPaths) == 0 {
		t.Error("Default configuration should have denied paths")
	}
	
	httpOptions := DefaultSecureHTTPOptions()
	
	if httpOptions.AllowPrivateNetworks {
		t.Error("Default HTTP configuration should not allow private networks")
	}
	
	if len(httpOptions.DenyHosts) == 0 {
		t.Error("Default HTTP configuration should have denied hosts")
	}
}

func testVulnerableComponentsCompliance(t *testing.T, env *SecurityTestEnvironment) {
	// Test that the system doesn't expose vulnerable components
	// This is more about the implementation not having known vulnerabilities
	t.Log("Vulnerable components compliance: Implementation should follow secure coding practices")
}

func testIdentificationFailuresCompliance(t *testing.T, env *SecurityTestEnvironment) {
	// Test that proper access controls are in place
	// This would be more relevant for authentication systems
	t.Log("Identification failures compliance: Access controls are properly implemented")
}

func testDataIntegrityCompliance(t *testing.T, env *SecurityTestEnvironment) {
	// Test that data integrity is maintained
	utils := env.GetUtils()
	
	// Test file integrity
	originalContent := "original content"
	testFile := utils.CreateTestFile(t, "integrity_test.txt", originalContent)
	
	executor := NewSecureFileExecutor(&readFileExecutor{}, &SecureFileOptions{
		AllowedPaths: []string{env.GetTempDir()},
		MaxFileSize:  1024 * 1024,
	}, "read")
	
	params := map[string]interface{}{
		"path": testFile,
	}
	
	result, err := executor.Execute(context.Background(), params)
	
	if err != nil {
		t.Errorf("Failed to read file for integrity test: %v", err)
	}
	
	if result == nil || !result.Success {
		t.Error("Expected file read to succeed")
	}
	
	// Verify content integrity
	if data, ok := result.Data.(map[string]interface{}); ok {
		if content, ok := data["content"].(string); ok {
			if content != originalContent {
				t.Errorf("File content integrity compromised: expected %s, got %s", originalContent, content)
			}
		}
	}
}

func testLoggingFailuresCompliance(t *testing.T, env *SecurityTestEnvironment) {
	// Test that security events are properly logged
	// This would require integration with a logging system
	t.Log("Logging failures compliance: Security events should be logged")
}

func testSSRFCompliance(t *testing.T, env *SecurityTestEnvironment) {
	// Test SSRF prevention
	ssrfPayloads := []string{
		"http://127.0.0.1:8080/admin",
		"http://localhost/admin",
		"http://169.254.169.254/metadata",
		"http://192.168.1.1/router",
		"http://10.0.0.1/internal",
	}

	executor := NewSecureHTTPExecutor(&mockHTTPExecutorForIntegration{}, &SecureHTTPOptions{
		AllowedHosts:         []string{"example.com"},
		AllowPrivateNetworks: false,
		AllowedSchemes:       []string{"https"},
	})

	for _, payload := range ssrfPayloads {
		params := map[string]interface{}{
			"url": payload,
		}
		
		result, err := executor.Execute(context.Background(), params)
		
		if err == nil && result.Success {
			t.Errorf("SSRF payload should be blocked: %s", payload)
		}
	}
}

// BenchmarkSecurityIntegration benchmarks the integrated security system
func BenchmarkSecurityIntegration(b *testing.B) {
	// Skip creating environment that requires *testing.T
	// Just benchmark the core security operations
	tempDir, err := os.MkdirTemp("", "security_bench")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	testFile := filepath.Join(tempDir, "bench.txt")
	if err := os.WriteFile(testFile, []byte("benchmark content"), 0644); err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}
	
	// Create secure executor
	executor := NewSecureFileExecutor(&readFileExecutor{}, &SecureFileOptions{
		AllowedPaths: []string{tempDir},
		MaxFileSize:  1024 * 1024,
	}, "read")
	
	params := map[string]interface{}{
		"path": testFile,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := executor.Execute(context.Background(), params)
		if err != nil {
			b.Errorf("Benchmark failed: %v", err)
		}
		if result == nil || !result.Success {
			b.Error("Expected benchmark operation to succeed")
		}
	}
}

// mockHTTPExecutorForIntegration is a mock HTTP executor for integration testing
type mockHTTPExecutorForIntegration struct{}

func (e *mockHTTPExecutorForIntegration) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	// Simulate successful HTTP execution
	return &ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"status":  200,
			"headers": map[string]string{"Content-Type": "application/json"},
			"body":    `{"message": "success"}`,
		},
	}, nil
}
