package encoding_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json"
	_ "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/json" // Register JSON codec
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/protobuf"
	_ "github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding/protobuf" // Register Protobuf codec
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegistryLifecycleManagement tests that the registry properly manages background goroutines
func TestRegistryLifecycleManagement(t *testing.T) {
	t.Run("background_cleanup_starts_and_stops", func(t *testing.T) {
		// Create registry with background cleanup enabled
		config := &encoding.RegistryConfig{
			MaxEntries:              100,
			TTL:                     1 * time.Second,
			CleanupInterval:         100 * time.Millisecond,
			EnableLRU:               true,
			EnableBackgroundCleanup: true,
		}
		
		registry := encoding.NewFormatRegistryWithConfig(config)
		
		// Give the background goroutine time to start
		time.Sleep(50 * time.Millisecond)
		
		// Close the registry
		err := registry.Close()
		require.NoError(t, err)
		
		// Give time for cleanup
		time.Sleep(50 * time.Millisecond)
		
		// Registry should be closed
		stats := registry.GetRegistryStats()
		assert.True(t, stats["is_closed"].(bool))
	})
	
	t.Run("global_registry_cleanup_works", func(t *testing.T) {
		// Close any existing global registry
		encoding.CloseGlobalRegistry()
		
		// Get global registry (creates a new one)
		registry := encoding.GetGlobalRegistry()
		require.NotNil(t, registry)
		
		// Close global registry
		err := encoding.CloseGlobalRegistry()
		require.NoError(t, err)
		
		// Getting it again should create a new instance
		newRegistry := encoding.GetGlobalRegistry()
		require.NotNil(t, newRegistry)
		
		// Clean up and ensure we have a functional global registry
		encoding.CloseGlobalRegistry()
		
		// Restore a functional global registry for other tests
		// The GetGlobalRegistry() call will create a new one, but we need to
		// ensure codecs are available for subsequent tests
		finalRegistry := encoding.GetGlobalRegistry()
		
		// Force re-registration of JSON and protobuf codecs for other tests
		// Since auto-registration via init() only happens once, we need explicit registration
		if err := json.RegisterTo(finalRegistry); err != nil {
			t.Logf("Warning: Failed to re-register JSON codec: %v", err)
		}
		if err := protobuf.RegisterTo(finalRegistry); err != nil {
			t.Logf("Warning: Failed to re-register Protobuf codec: %v", err)
		}
	})
	
	t.Run("multiple_registries_independent", func(t *testing.T) {
		// Create multiple independent registries
		config := &encoding.RegistryConfig{
			MaxEntries:              50,
			TTL:                     500 * time.Millisecond,
			CleanupInterval:         50 * time.Millisecond,
			EnableLRU:               true,
			EnableBackgroundCleanup: true,
		}
		
		registry1 := encoding.NewFormatRegistryWithConfig(config)
		registry2 := encoding.NewFormatRegistryWithConfig(config)
		
		// Both should be independent
		assert.NotEqual(t, registry1, registry2)
		
		// Close first registry
		err1 := registry1.Close()
		require.NoError(t, err1)
		
		// Second should still be open
		stats2 := registry2.GetRegistryStats()
		assert.False(t, stats2["is_closed"].(bool))
		
		// Close second registry
		err2 := registry2.Close()
		require.NoError(t, err2)
	})
}

// TestGoroutineLeakPrevention specifically tests for goroutine leaks
func TestGoroutineLeakPrevention(t *testing.T) {
	// Get initial goroutine count
	initialGoroutines := runtime.NumGoroutine()
	
	// Create and close multiple registries
	for i := 0; i < 10; i++ {
		config := &encoding.RegistryConfig{
			MaxEntries:              100,
			TTL:                     1 * time.Second,
			CleanupInterval:         50 * time.Millisecond,
			EnableLRU:               true,
			EnableBackgroundCleanup: true,
		}
		
		registry := encoding.NewFormatRegistryWithConfig(config)
		
		// Do some operations
		info := encoding.NewFormatInfo("Test Format", "application/test")
		registry.RegisterFormat(info)
		
		// Close the registry
		registry.Close()
	}
	
	// Give time for all goroutines to clean up
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	
	// Final goroutine count should be close to initial (allowing for some variance)
	finalGoroutines := runtime.NumGoroutine()
	
	// Allow for some variance but shouldn't have significantly more goroutines
	maxExpectedGoroutines := initialGoroutines + 5 // Allow 5 extra goroutines for variance
	
	if finalGoroutines > maxExpectedGoroutines {
		t.Errorf("Potential goroutine leak detected: started with %d, ended with %d goroutines", 
			initialGoroutines, finalGoroutines)
	}
	
	t.Logf("Goroutine count: initial=%d, final=%d", initialGoroutines, finalGoroutines)
}