package tools

import (
	"testing"
)

// SecurityTestEnvironment provides a testing environment for security tests
type SecurityTestEnvironment struct {
	utils            *SecurityTestUtils
	helpers          *SecurityTestHelpers
	executor         *SecurityTestExecutor
	payloadGenerator *PayloadGenerator
	t                *testing.T
}

// NewSecurityTestEnvironment creates a new security test environment
func NewSecurityTestEnvironment(t *testing.T) *SecurityTestEnvironment {
	env := &SecurityTestEnvironment{
		t:                t,
		utils:            NewSecurityTestUtils(t),
		helpers:          NewSecurityTestHelpers(),
		executor:         NewSecurityTestExecutor(),
		payloadGenerator: NewPayloadGenerator(),
	}

	return env
}

// Cleanup cleans up the test environment
func (e *SecurityTestEnvironment) Cleanup() {
	if e.utils != nil {
		e.utils.Cleanup()
	}
}

// GetTempDir returns the temporary directory path
func (e *SecurityTestEnvironment) GetTempDir() string {
	return e.utils.GetTempDir()
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

// GetPayloadGenerator returns the security payload generator
func (e *SecurityTestEnvironment) GetPayloadGenerator() *PayloadGenerator {
	return e.payloadGenerator
}

// GenerateFinalReport generates a final security test report
func (e *SecurityTestEnvironment) GenerateFinalReport(t *testing.T) {
	t.Log("Security integration test completed successfully")
}

// SecurityTestHelpers provides helper functions for security testing
type SecurityTestHelpers struct{}

// NewSecurityTestHelpers creates new security test helpers
func NewSecurityTestHelpers() *SecurityTestHelpers {
	return &SecurityTestHelpers{}
}

// CreateSecureFileOptions creates secure file options for testing
func (h *SecurityTestHelpers) CreateSecureFileOptions(allowedPaths []string, maxFileSize int64) *SecureFileOptions {
	return &SecureFileOptions{
		AllowedPaths:  allowedPaths,
		MaxFileSize:   maxFileSize,
		AllowSymlinks: false,
		DenyPaths:     []string{"/etc", "/sys", "/proc", "/root"},
	}
}

// CreateSecureHTTPOptions creates secure HTTP options for testing
func (h *SecurityTestHelpers) CreateSecureHTTPOptions(allowedHosts []string) *SecureHTTPOptions {
	return &SecureHTTPOptions{
		AllowedHosts:         allowedHosts,
		AllowPrivateNetworks: false,
		AllowedSchemes:       []string{"https"},
		MaxRedirects:         5,
	}
}

// SecurityTestExecutor executes security tests
type SecurityTestExecutor struct{}

// NewSecurityTestExecutor creates a new security test executor
func NewSecurityTestExecutor() *SecurityTestExecutor {
	return &SecurityTestExecutor{}
}

// ExecuteSecurityTest executes a security test
func (e *SecurityTestExecutor) ExecuteSecurityTest(t *testing.T, testName, description string, executor ToolExecutor, params map[string]interface{}, expectError bool, expectedErrorMsg string) {
	t.Run(testName, func(t *testing.T) {
		result, err := executor.Execute(nil, params)
		
		if expectError {
			if err == nil && (result == nil || result.Success) {
				t.Errorf("Expected security test to fail: %s", description)
			} else {
				t.Logf("Security test correctly failed: %s", description)
				if expectedErrorMsg != "" && err != nil {
					// You could add more sophisticated error message checking here
					t.Logf("Error message: %v", err)
				}
			}
		} else {
			if err != nil {
				t.Errorf("Security test unexpectedly failed: %s - Error: %v", description, err)
			}
			if result == nil || !result.Success {
				t.Errorf("Security test should have succeeded: %s", description)
			}
		}
	})
}


