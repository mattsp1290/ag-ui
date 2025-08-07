package timeconfig

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestTimeConfigTestMode(t *testing.T) {
	// Test that IsTestMode returns true when running tests
	if !IsTestMode() {
		t.Error("IsTestMode should return true when running under 'go test'")
	}
}

func TestTimeConfigEnvironmentVariable(t *testing.T) {
	// Save original value
	original := os.Getenv("AG_SDK_TEST_MODE")
	defer func() {
		if original == "" {
			os.Unsetenv("AG_SDK_TEST_MODE")
		} else {
			os.Setenv("AG_SDK_TEST_MODE", original)
		}
	}()

	// Test setting environment variable
	os.Setenv("AG_SDK_TEST_MODE", "true")
	if !IsTestMode() {
		t.Error("IsTestMode should return true when AG_SDK_TEST_MODE=true")
	}

	os.Setenv("AG_SDK_TEST_MODE", "false")
	// Note: Even with AG_SDK_TEST_MODE=false, we're still in test mode because we're running under 'go test'
	if !IsTestMode() {
		t.Log("Expected: IsTestMode returns true because we're running under 'go test' even when AG_SDK_TEST_MODE=false")
	}
}

func TestDefaultTestConfiguration(t *testing.T) {
	// Reset config to ensure we get a fresh one
	ResetConfig()

	config := GetConfig()

	// Verify that test timeouts are much shorter than production defaults
	if config.DefaultShutdownTimeout >= 30*time.Second {
		t.Error("Test shutdown timeout should be much shorter than production (30s)")
	}

	if config.DefaultHTTPTimeout >= 60*time.Second {
		t.Error("Test HTTP timeout should be much shorter than production (60s)")
	}

	if config.DefaultDialTimeout >= 10*time.Second {
		t.Error("Test dial timeout should be much shorter than production (10s)")
	}

	// Verify test values are appropriate
	if config.DefaultShutdownTimeout != 1*time.Second {
		t.Errorf("Expected test shutdown timeout to be 1s, got %v", config.DefaultShutdownTimeout)
	}

	if config.DefaultHTTPTimeout != 1*time.Second {
		t.Errorf("Expected test HTTP timeout to be 1s, got %v", config.DefaultHTTPTimeout)
	}
}

func TestConfigurationOverride(t *testing.T) {
	// Test override functionality
	cleanup := OverrideForTest(map[string]time.Duration{
		"shutdown": 500 * time.Millisecond,
		"http":     2 * time.Second,
	})
	defer cleanup()

	config := GetConfig()
	if config.DefaultShutdownTimeout != 500*time.Millisecond {
		t.Errorf("Expected overridden shutdown timeout to be 500ms, got %v", config.DefaultShutdownTimeout)
	}

	if config.DefaultHTTPTimeout != 2*time.Second {
		t.Errorf("Expected overridden HTTP timeout to be 2s, got %v", config.DefaultHTTPTimeout)
	}

	// Verify cleanup restores original values
	cleanup()

	config = GetConfig()
	if config.DefaultShutdownTimeout == 500*time.Millisecond {
		t.Error("Cleanup should have restored original shutdown timeout")
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test that helper functions return expected values
	shutdownTimeout := ShutdownTimeout()
	if shutdownTimeout != GetConfig().DefaultShutdownTimeout {
		t.Error("ShutdownTimeout() should return the same value as GetConfig().DefaultShutdownTimeout")
	}

	httpTimeout := HTTPTimeout()
	if httpTimeout != GetConfig().DefaultHTTPTimeout {
		t.Error("HTTPTimeout() should return the same value as GetConfig().DefaultHTTPTimeout")
	}

	dialTimeout := DialTimeout()
	if dialTimeout != GetConfig().DefaultDialTimeout {
		t.Error("DialTimeout() should return the same value as GetConfig().DefaultDialTimeout")
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that the old constants would have been much longer in production
	// This verifies that our test configuration is actually providing shorter timeouts
	config := GetConfig()

	// These should be the test values (much shorter)
	testTimeouts := []struct {
		name    string
		actual  time.Duration
		prodMin time.Duration // Minimum production value it should be less than
	}{
		{"ShutdownTimeout", config.DefaultShutdownTimeout, 10 * time.Second},
		{"HTTPTimeout", config.DefaultHTTPTimeout, 30 * time.Second},
		{"DialTimeout", config.DefaultDialTimeout, 5 * time.Second},
		{"ReadTimeout", config.DefaultReadTimeout, 30 * time.Second},
		{"WriteTimeout", config.DefaultWriteTimeout, 5 * time.Second},
	}

	for _, tt := range testTimeouts {
		if tt.actual >= tt.prodMin {
			t.Errorf("%s should be much shorter in test mode: got %v, should be less than production minimum %v",
				tt.name, tt.actual, tt.prodMin)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	// Test that concurrent access to GetConfig is safe
	const numGoroutines = 100
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			config := GetConfig()
			// Just access some fields to ensure the config is valid
			_ = config.DefaultShutdownTimeout
			_ = config.DefaultHTTPTimeout
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent access test timed out")
		}
	}
}

// Example test showing how to use the timeconfig in application code
func TestExampleUsage(t *testing.T) {
	// This demonstrates how existing code can migrate from hardcoded timeouts
	// to configurable ones without breaking changes

	// Old way (hardcoded):
	// timeout := 30 * time.Second

	// New way (configurable):
	timeout := ShutdownTimeout()

	// In test mode, this will be 1 second
	// In production mode, this will be 30 seconds
	if timeout > 5*time.Second {
		t.Error("In test mode, timeouts should be short to speed up tests")
	}

	// Example of using in a context
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// The context will use appropriate timeout based on environment
	select {
	case <-ctx.Done():
		// This will happen quickly in test mode
	case <-time.After(100 * time.Millisecond):
		// Normal operation continues
	}
}
