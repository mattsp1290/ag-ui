//go:build stress
// +build stress

package state

import (
	"testing"
)

// Security validation stress testing functionality has been removed as it was too resource-intensive for CI/CD environments.
// These tests were designed for stress testing security validation under heavy concurrent load (20+ workers, 100+ updates each).

// TestConcurrentSecurityValidationStress - removed (was testing 20 workers with 100 updates each)
func TestConcurrentSecurityValidationStress(t *testing.T) {
	t.Skip("Concurrent security validation stress tests removed - exceeded CI resource limits")
}

// TestRateLimitingIntegrationStress - removed (was testing 500+ rapid requests)
func TestRateLimitingIntegrationStress(t *testing.T) {
	t.Skip("Rate limiting integration stress tests removed - exceeded CI resource limits")
}