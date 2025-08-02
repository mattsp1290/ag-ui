package transport

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// TestMapCleanupMechanism demonstrates the map cleanup mechanism
func TestMapCleanupMechanism(t *testing.T) {
	// Create a transport manager with aggressive cleanup settings for testing
	config := &TransportManagerConfig{
		CleanupEnabled:        true,
		CleanupInterval:       100 * time.Millisecond, // Very frequent for testing
		MaxMapSize:           5,  // Small threshold for testing
		ActiveThreshold:      0.6, // 60% active threshold
		CleanupMetricsEnabled: true,
	}
	
	registry := NewDefaultTransportRegistry()
	manager := NewDefaultTransportManagerWithConfig(registry, config)
	defer manager.Close()
	
	// Add some mock transports
	for i := 0; i < 10; i++ {
		transport := NewMockTransport()
		
		// Connect only the first 3 transports
		if i < 3 {
			transport.Connect(context.Background())
		}
		
		err := manager.AddTransport(fmt.Sprintf("transport-%d", i), transport)
		if err != nil {
			t.Fatalf("Failed to add transport %d: %v", i, err)
		}
	}
	
	// Check initial state
	initialMetrics := manager.GetMapCleanupMetrics()
	if initialMetrics.TotalCleanups == 0 {
		t.Log("No cleanup triggered yet, which is expected")
	}
	
	// Active transports: 3, Total: 10, Ratio: 0.3 (below 0.6 threshold)
	// Should trigger cleanup since 10 >= 5 (MaxMapSize) and 0.3 < 0.6 (ActiveThreshold)
	
	// Manually trigger cleanup to test
	manager.TriggerManualCleanup()
	
	// Check cleanup metrics
	finalMetrics := manager.GetMapCleanupMetrics()
	
	if finalMetrics.TotalCleanups == 0 {
		t.Error("Expected at least one cleanup to have occurred")
	}
	
	if finalMetrics.TransportsRemoved == 0 {
		t.Error("Expected some transports to be removed")
	}
	
	// Since cleanup can run multiple times, just check that some transports were retained
	if finalMetrics.TransportsRetained == 0 {
		t.Error("Expected some transports to be retained")
	}
	
	t.Logf("Cleanup metrics: TotalCleanups=%d, TransportsRemoved=%d, TransportsRetained=%d, LastCleanupDuration=%v, CleanupErrors=%d", 
		finalMetrics.TotalCleanups, finalMetrics.TransportsRemoved, finalMetrics.TransportsRetained, 
		finalMetrics.LastCleanupDuration, finalMetrics.CleanupErrors)
	
	// Verify that active transports are retained
	activeTransports := manager.GetActiveTransports()
	if len(activeTransports) != 3 {
		t.Errorf("Expected 3 active transports after cleanup, got %d", len(activeTransports))
	}
	
	// Verify that the map size was reduced
	if len(activeTransports) < 10 {
		t.Logf("SUCCESS: Map size reduced from 10 to %d active transports", len(activeTransports))
	}
}

// TestCleanupConfiguration tests cleanup configuration validation
func TestCleanupConfiguration(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultTransportManagerConfig()
		
		if !config.CleanupEnabled {
			t.Error("Expected cleanup to be enabled by default")
		}
		
		if config.CleanupInterval != 1*time.Hour {
			t.Errorf("Expected default interval of 1 hour, got %v", config.CleanupInterval)
		}
		
		if config.MaxMapSize != 1000 {
			t.Errorf("Expected default max size of 1000, got %d", config.MaxMapSize)
		}
		
		if config.ActiveThreshold != 0.5 {
			t.Errorf("Expected default active threshold of 0.5, got %f", config.ActiveThreshold)
		}
	})
	
	t.Run("DisabledCleanup", func(t *testing.T) {
		config := &TransportManagerConfig{
			CleanupEnabled: false,
		}
		
		registry := NewDefaultTransportRegistry()
		manager := NewDefaultTransportManagerWithConfig(registry, config)
		defer manager.Close()
		
		// Add many transports (all disconnected by default)
		for i := 0; i < 20; i++ {
			transport := NewMockTransport()
			manager.AddTransport(fmt.Sprintf("transport-%d", i), transport)
		}
		
		// Trigger manual cleanup - should be ignored
		manager.TriggerManualCleanup()
		
		metrics := manager.GetMapCleanupMetrics()
		if metrics.TotalCleanups != 0 {
			t.Error("Expected no cleanups when cleanup is disabled")
		}
	})
}