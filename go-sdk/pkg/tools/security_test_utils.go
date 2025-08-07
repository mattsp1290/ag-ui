package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

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

// SecurityTestCase represents a security test case
type SecurityTestCase struct {
	Name        string
	Description string
	Input       interface{}
	Expected    SecurityTestExpectation
	Params      map[string]interface{}
}

// SecurityTestExpectation defines expected results for security tests
type SecurityTestExpectation struct {
	ShouldFail    bool
	ErrorContains string
	ShouldBlock   bool
	AllowedResult interface{}
}

// SecurityTestSuite provides a framework for running security tests
type SecurityTestSuite struct {
	Name      string
	TestCases []SecurityTestCase
	Setup     func(t *testing.T) interface{}
	Teardown  func(t *testing.T, context interface{})
}

// Run executes the security test suite
func (s *SecurityTestSuite) Run(t *testing.T) {
	var setupContext interface{}
	if s.Setup != nil {
		setupContext = s.Setup(t)
	}

	defer func() {
		if s.Teardown != nil {
			s.Teardown(t, setupContext)
		}
	}()

	for _, testCase := range s.TestCases {
		t.Run(testCase.Name, func(t *testing.T) {
			s.runTestCase(t, testCase, setupContext)
		})
	}
}

// runTestCase executes a single test case
func (s *SecurityTestSuite) runTestCase(t *testing.T, testCase SecurityTestCase, setupContext interface{}) {
	// This is a placeholder - actual implementation would depend on the specific test type
	t.Logf("Running security test: %s - %s", testCase.Name, testCase.Description)

	// Log test parameters for debugging
	if testCase.Params != nil {
		t.Logf("Test parameters: %+v", testCase.Params)
	}

	// Log expected outcome
	if testCase.Expected.ShouldFail {
		t.Logf("Expected outcome: should fail")
		if testCase.Expected.ErrorContains != "" {
			t.Logf("Expected error containing: %s", testCase.Expected.ErrorContains)
		}
	} else {
		t.Logf("Expected outcome: should succeed")
	}

	if testCase.Expected.ShouldBlock {
		t.Logf("Expected security blocking: enabled")
	}
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

// GenerateXSSPayloads generates XSS attack payloads
func (g *PayloadGenerator) GenerateXSSPayloads() []string {
	return []string{
		"<script>alert('XSS')</script>",
		"<img src=x onerror=alert('XSS')>",
		"<svg onload=alert('XSS')>",
		"<body onload=alert('XSS')>",
		"<input onfocus=alert('XSS') autofocus>",
		"<a href='javascript:alert(\"XSS\")'>Click me</a>",
		"<iframe src='data:text/html,<script>alert(\"XSS\")</script>'></iframe>",
		"<style>body{background:url(javascript:alert('XSS'))}</style>",
		"<meta http-equiv='refresh' content='0;url=javascript:alert(\"XSS\")'>",
		"<div onclick='alert(\"XSS\")'>Click me</div>",
		"<div style='width:expression(alert(\"XSS\"))'>",
		"<object data='javascript:alert(\"XSS\")'></object>",
		"<embed src='javascript:alert(\"XSS\")'></embed>",
		"<form action='javascript:alert(\"XSS\")'><input type=submit></form>",
		"<link rel='import' href='javascript:alert(\"XSS\")'>",
		"<base href='javascript:alert(\"XSS\")'>",
		"&lt;script&gt;alert('XSS')&lt;/script&gt;",
		"<script>alert('XSS')</script>",
		"javascript:/*--></title></style></textarea></script></xmp><svg/onload='+/\"/+/onmouseover=1/+/[*/[]/+alert(1)//'>",
		"<img src=x onerror=eval(String.fromCharCode(97,108,101,114,116,40,39,88,83,83,39,41))>",
	}
}

// GenerateSQLInjectionPayloads generates SQL injection payloads
func (g *PayloadGenerator) GenerateSQLInjectionPayloads() []string {
	return []string{
		"' OR '1'='1",
		"' UNION SELECT * FROM users --",
		"'; DROP TABLE users; --",
		"' AND (SELECT COUNT(*) FROM users) > 0 --",
		"'; WAITFOR DELAY '00:00:10' --",
		"'; INSERT INTO users (username, password) VALUES ('admin', 'admin'); --",
		"' AND (SELECT SUBSTRING(password, 1, 1) FROM users WHERE username = 'admin') = 'a",
		"' AND (SELECT * FROM (SELECT COUNT(*), CONCAT(version(), FLOOR(RAND(0)*2)) x FROM information_schema.tables GROUP BY x) a) --",
		"admin'; UPDATE users SET password = 'hacked' WHERE username = 'admin' --",
		"{'$ne': null}",
		"%27%20OR%20%271%27%3D%271",
		"＇　OR　＇1＇＝＇1",
		"' OR '1'='1'\x00",
		"0x27204F52202731273D2731",
		"CHAR(39) + OR + CHAR(39) + 1 + CHAR(39) + = + CHAR(39) + 1",
	}
}

// GenerateURLInjectionPayloads generates URL injection payloads
func (g *PayloadGenerator) GenerateURLInjectionPayloads() []string {
	return []string{
		"javascript:alert('XSS')",
		"data:text/html,<script>alert('XSS')</script>",
		"file:///etc/passwd",
		"ftp://user:pass@evil.com/malicious",
		"http://127.0.0.1:8080/admin",
		"http://192.168.1.1/router",
		"http://169.254.169.254/metadata",
		"http://localhost/admin",
		"http://2130706433/admin",
		"http://0x7f000001/admin",
		"http://0177.0000.0000.0001/admin",
		"http://[::1]/admin",
		"http://[fc00::1]/admin",
		"http://admin:password@internal.server/admin",
		"http://target.com:22/",
		"http://example.com/path\r\nHost: evil.com",
		"http://example.com/path\x00evil.com",
		"http://еxample.com/path", // Cyrillic 'e'
		"http://xn--e1afmkfd.xn--p1ai/path",
		"http://example.com/" + strings.Repeat("a", 10000),
	}
}

// GenerateTemplateInjectionPayloads generates template injection payloads
func (g *PayloadGenerator) GenerateTemplateInjectionPayloads() []string {
	return []string{
		"{{7*7}}",
		"{{#each this}}{{this}}{{/each}}",
		"{{#lambda}}{{/lambda}}",
		"{{_self.env.registerUndefinedFilterCallback('exec')}}",
		"{php}echo `id`;{/php}",
		"#set($str=$class.forName('java.lang.String'))",
		"<%= system('id') %>",
		"{% load module %}",
		"{{.}}",
		"${7*7}",
		"#{T(java.lang.Runtime).getRuntime().exec('cat /etc/passwd')}",
		"@java.lang.Runtime@getRuntime().exec('cat /etc/passwd')",
		"Runtime.getRuntime().exec('cat /etc/passwd')",
		"${Runtime.getRuntime().exec('cat /etc/passwd')}",
		"${new ProcessBuilder('cat','/etc/passwd').start()}",
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

	// Check for common sensitive data patterns, but exclude common false positives in security testing
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

	// Skip "private" detection if it's part of common configuration terms
	falsePositivePatterns := []string{
		"allowprivatenetworks",
		"privatenetworks",
		"private networks",
		"private ip",
		"private range",
		"private address",
		"allowprivate",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(resultStr, strings.ToLower(pattern)) {
			// Check if this is a false positive for "private" pattern
			if pattern == "private" {
				isFalsePositive := false
				for _, fpPattern := range falsePositivePatterns {
					if strings.Contains(resultStr, fpPattern) {
						isFalsePositive = true
						break
					}
				}
				if isFalsePositive {
					continue // Skip this detection as it's a false positive
				}
			}
			t.Errorf("Potential sensitive data leakage detected: %s", pattern)
		}
	}
}

// SecurityTestReporter generates reports for security tests
type SecurityTestReporter struct {
	testResults []SecurityTestResult
}

// SecurityTestResult represents the result of a security test
type SecurityTestResult struct {
	TestName    string
	Description string
	Passed      bool
	Error       string
	Duration    time.Duration
	Threat      string
	Blocked     bool
}

// NewSecurityTestReporter creates a new security test reporter
func NewSecurityTestReporter() *SecurityTestReporter {
	return &SecurityTestReporter{
		testResults: make([]SecurityTestResult, 0),
	}
}

// AddResult adds a test result to the reporter
func (r *SecurityTestReporter) AddResult(result SecurityTestResult) {
	r.testResults = append(r.testResults, result)
}

// GenerateReport generates a security test report
func (r *SecurityTestReporter) GenerateReport(t *testing.T) {
	t.Logf("=== Security Test Report ===")

	totalTests := len(r.testResults)
	passedTests := 0
	blockedThreats := 0

	for _, result := range r.testResults {
		if result.Passed {
			passedTests++
		}
		if result.Blocked {
			blockedThreats++
		}

		status := "PASS"
		if !result.Passed {
			status = "FAIL"
		}

		t.Logf("[%s] %s - %s (Duration: %v)", status, result.TestName, result.Description, result.Duration)

		if result.Error != "" {
			t.Logf("  Error: %s", result.Error)
		}

		if result.Threat != "" {
			blockStatus := "BLOCKED"
			if !result.Blocked {
				blockStatus = "NOT BLOCKED"
			}
			t.Logf("  Threat: %s - %s", result.Threat, blockStatus)
		}
	}

	t.Logf("=== Summary ===")
	t.Logf("Total Tests: %d", totalTests)
	t.Logf("Passed: %d", passedTests)
	t.Logf("Failed: %d", totalTests-passedTests)
	t.Logf("Threats Blocked: %d", blockedThreats)
	t.Logf("Success Rate: %.2f%%", float64(passedTests)/float64(totalTests)*100)
	t.Logf("Threat Block Rate: %.2f%%", float64(blockedThreats)/float64(totalTests)*100)
}

// SecurityTestExecutor executes security tests with consistent patterns
type SecurityTestExecutor struct {
	validator *SecurityTestValidator
	reporter  *SecurityTestReporter
}

// NewSecurityTestExecutor creates a new security test executor
func NewSecurityTestExecutor() *SecurityTestExecutor {
	return &SecurityTestExecutor{
		validator: NewSecurityTestValidator(),
		reporter:  NewSecurityTestReporter(),
	}
}

// ExecuteSecurityTest executes a security test with comprehensive validation
func (e *SecurityTestExecutor) ExecuteSecurityTest(t *testing.T, testName, description string, executor ToolExecutor, params map[string]interface{}, expectBlocked bool, expectedError string) {
	startTime := time.Now()

	result, err := executor.Execute(context.Background(), params)

	duration := time.Since(startTime)

	// Validate security blocking
	e.validator.ValidateSecurityBlocking(t, result, err, expectBlocked)

	// Validate error message if provided
	if expectedError != "" {
		e.validator.ValidateErrorMessage(t, result, expectedError)
	}

	// Validate no data leakage
	e.validator.ValidateNoDataLeakage(t, result)

	// Determine if test passed
	passed := true
	errorMsg := ""

	if expectBlocked {
		if err == nil && result.Success {
			passed = false
			errorMsg = "Expected security blocking but operation succeeded"
		}
	} else {
		if err != nil {
			passed = false
			errorMsg = fmt.Sprintf("Unexpected error: %v", err)
		}
		if result == nil || !result.Success {
			passed = false
			errorMsg = "Expected operation to succeed but it failed"
		}
	}

	// Add result to reporter
	e.reporter.AddResult(SecurityTestResult{
		TestName:    testName,
		Description: description,
		Passed:      passed,
		Error:       errorMsg,
		Duration:    duration,
		Threat:      "Security test",
		Blocked:     expectBlocked && !result.Success,
	})
}

// GenerateReport generates the final security test report
func (e *SecurityTestExecutor) GenerateReport(t *testing.T) {
	e.reporter.GenerateReport(t)
}

// SecurityTestHelpers provides common helper functions for security tests
type SecurityTestHelpers struct{}

// NewSecurityTestHelpers creates a new security test helpers instance
func NewSecurityTestHelpers() *SecurityTestHelpers {
	return &SecurityTestHelpers{}
}

// SkipOnWindows skips a test on Windows systems
func (h *SecurityTestHelpers) SkipOnWindows(t *testing.T, reason string) {
	if runtime.GOOS == "windows" {
		t.Skipf("Skipping on Windows: %s", reason)
	}
}

// SkipOnMacOS skips a test on macOS systems
func (h *SecurityTestHelpers) SkipOnMacOS(t *testing.T, reason string) {
	if runtime.GOOS == "darwin" {
		t.Skipf("Skipping on macOS: %s", reason)
	}
}

// SkipOnLinux skips a test on Linux systems
func (h *SecurityTestHelpers) SkipOnLinux(t *testing.T, reason string) {
	if runtime.GOOS == "linux" {
		t.Skipf("Skipping on Linux: %s", reason)
	}
}

// RequireRoot skips a test if not running as root
func (h *SecurityTestHelpers) RequireRoot(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("This test requires root privileges")
	}
}

// RequireNonRoot skips a test if running as root
func (h *SecurityTestHelpers) RequireNonRoot(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("This test should not run as root")
	}
}

// CreateSecureFileOptions creates secure file options for testing
func (h *SecurityTestHelpers) CreateSecureFileOptions(allowedPaths []string, maxFileSize int64) *SecureFileOptions {
	return &SecureFileOptions{
		AllowedPaths:  allowedPaths,
		MaxFileSize:   maxFileSize,
		AllowSymlinks: false,
		DenyPaths:     DefaultSecureFileOptions().DenyPaths,
	}
}

// CreateSecureHTTPOptions creates secure HTTP options for testing
func (h *SecurityTestHelpers) CreateSecureHTTPOptions(allowedHosts []string) *SecureHTTPOptions {
	return &SecureHTTPOptions{
		AllowedHosts:         allowedHosts,
		DenyHosts:            DefaultSecureHTTPOptions().DenyHosts,
		AllowPrivateNetworks: false,
		AllowedSchemes:       []string{"http", "https"},
		MaxRedirects:         5,
	}
}

// CreateMockExecutor creates a mock executor for testing
func (h *SecurityTestHelpers) CreateMockExecutor(shouldSucceed bool, resultData interface{}) ToolExecutor {
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

// SecurityTestEnvironment provides a controlled environment for security testing
type SecurityTestEnvironment struct {
	tempDir     string
	originalDir string
	cleanup     func()
	utils       *SecurityTestUtils
	helpers     *SecurityTestHelpers
	executor    *SecurityTestExecutor
	payloadGen  *PayloadGenerator
}

// NewSecurityTestEnvironment creates a new security test environment
func NewSecurityTestEnvironment(t *testing.T) *SecurityTestEnvironment {
	utils := NewSecurityTestUtils(t)
	helpers := NewSecurityTestHelpers()
	executor := NewSecurityTestExecutor()
	payloadGen := NewPayloadGenerator()

	// Save original directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	return &SecurityTestEnvironment{
		tempDir:     utils.GetTempDir(),
		originalDir: originalDir,
		cleanup: func() {
			os.Chdir(originalDir)
			utils.Cleanup()
		},
		utils:      utils,
		helpers:    helpers,
		executor:   executor,
		payloadGen: payloadGen,
	}
}

// Cleanup cleans up the test environment
func (e *SecurityTestEnvironment) Cleanup() {
	if e.cleanup != nil {
		e.cleanup()
	}
}

// GetUtils returns the security test utilities
func (e *SecurityTestEnvironment) GetUtils() *SecurityTestUtils {
	return e.utils
}

// GetHelpers returns the security test helpers
func (e *SecurityTestEnvironment) GetHelpers() *SecurityTestHelpers {
	return e.helpers
}

// GetExecutor returns the security test executor
func (e *SecurityTestEnvironment) GetExecutor() *SecurityTestExecutor {
	return e.executor
}

// GetPayloadGenerator returns the payload generator
func (e *SecurityTestEnvironment) GetPayloadGenerator() *PayloadGenerator {
	return e.payloadGen
}

// GetTempDir returns the temporary directory
func (e *SecurityTestEnvironment) GetTempDir() string {
	return e.tempDir
}

// ChangeToTempDir changes the current working directory to the temp directory
func (e *SecurityTestEnvironment) ChangeToTempDir(t *testing.T) {
	if err := os.Chdir(e.tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
}

// RunSecurityTestBatch runs a batch of security tests with consistent setup
func (e *SecurityTestEnvironment) RunSecurityTestBatch(t *testing.T, testName string, executor ToolExecutor, payloads []string, expectBlocked bool, paramBuilder func(payload string) map[string]interface{}) {
	for i, payload := range payloads {
		testCaseName := fmt.Sprintf("%s_%d", testName, i)
		description := fmt.Sprintf("Testing payload: %s", payload)

		params := paramBuilder(payload)

		e.executor.ExecuteSecurityTest(t, testCaseName, description, executor, params, expectBlocked, "")
	}
}

// GenerateFinalReport generates the final security test report
func (e *SecurityTestEnvironment) GenerateFinalReport(t *testing.T) {
	e.executor.GenerateReport(t)
}
