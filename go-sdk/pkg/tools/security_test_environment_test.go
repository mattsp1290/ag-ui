package tools

import (
	"testing"
)

// This file contains additional tests for the SecurityTestEnvironment
// All type definitions are in security_test_utils.go

// TestSecurityTestEnvironmentIntegration tests the security test environment integration
func TestSecurityTestEnvironmentIntegration(t *testing.T) {
	env := NewSecurityTestEnvironment(t)
	defer env.Cleanup()

	// Test that all components are properly initialized
	if env.GetUtils() == nil {
		t.Error("SecurityTestUtils should be initialized")
	}
	
	if env.GetHelpers() == nil {
		t.Error("SecurityTestHelpers should be initialized")
	}
	
	if env.GetExecutor() == nil {
		t.Error("SecurityTestExecutor should be initialized")
	}
	
	if env.GetPayloadGenerator() == nil {
		t.Error("PayloadGenerator should be initialized")
	}
	
	// Test temp directory functionality
	tempDir := env.GetTempDir()
	if tempDir == "" {
		t.Error("Temp directory should not be empty")
	}
	
	// Test final report generation
	env.GenerateFinalReport(t)
}

// TestSecurityTestEnvironmentCleanup tests the cleanup functionality
func TestSecurityTestEnvironmentCleanup(t *testing.T) {
	env := NewSecurityTestEnvironment(t)
	
	// Verify environment is set up
	if env.GetUtils() == nil {
		t.Error("SecurityTestUtils should be initialized before cleanup")
	}
	
	// Test cleanup
	env.Cleanup()
	
	// After cleanup, the environment should still be accessible
	// (cleanup doesn't nil out the references, just cleans up resources)
	if env.GetUtils() == nil {
		t.Error("SecurityTestUtils should still be accessible after cleanup")
	}
}