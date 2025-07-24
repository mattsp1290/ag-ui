package state

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap/zapcore"
)

func TestIsolatedRegistries(t *testing.T) {
	// Create two monitoring systems to ensure they use separate registries
	config1 := DefaultMonitoringConfig()
	config1.LogLevel = zapcore.ErrorLevel
	config1.PrometheusNamespace = "test1"

	config2 := DefaultMonitoringConfig()
	config2.LogLevel = zapcore.ErrorLevel
	config2.PrometheusNamespace = "test2"

	ms1, err := NewMonitoringSystem(config1)
	if err != nil {
		t.Fatalf("Failed to create first monitoring system: %v", err)
	}
	defer ms1.Shutdown(context.Background())

	ms2, err := NewMonitoringSystem(config2)
	if err != nil {
		t.Fatalf("Failed to create second monitoring system: %v", err)
	}
	defer ms2.Shutdown(context.Background())

	// Both systems should work without conflicts
	if ms1.promMetrics.Registry == nil {
		t.Error("First monitoring system should have a registry")
	}

	if ms2.promMetrics.Registry == nil {
		t.Error("Second monitoring system should have a registry")
	}

	// Registries should be different
	if ms1.promMetrics.Registry == ms2.promMetrics.Registry {
		t.Error("Monitoring systems should have separate registries")
	}
}

func TestMemoryMetricsRecording(t *testing.T) {
	config := DefaultMonitoringConfig()
	config.LogLevel = zapcore.ErrorLevel
	config.MetricsInterval = 100 * time.Millisecond

	ms, err := NewMonitoringSystem(config)
	if err != nil {
		t.Fatalf("Failed to create monitoring system: %v", err)
	}
	defer ms.Shutdown(context.Background())

	// Record memory usage
	testMemoryUsage := uint64(1024 * 1024 * 50) // 50MB
	testAllocations := int64(1000)
	testGCPause := 5 * time.Millisecond

	ms.RecordMemoryUsage(testMemoryUsage, testAllocations, testGCPause)

	// Get metrics and verify both Prometheus and internal state are updated
	metrics := ms.GetMetrics()

	if metrics.Memory.Usage != testMemoryUsage {
		t.Errorf("Expected memory usage %d, got %d", testMemoryUsage, metrics.Memory.Usage)
	}

	// Verify that the internal resource monitor was updated
	ms.resourceMonitor.mu.RLock()
	recordedUsage := ms.resourceMonitor.memoryUsage
	ms.resourceMonitor.mu.RUnlock()

	if recordedUsage != testMemoryUsage {
		t.Errorf("Expected internal memory usage %d, got %d", testMemoryUsage, recordedUsage)
	}
}

func TestConcurrentMonitoringSystems(t *testing.T) {
	const numSystems = 5
	systems := make([]*MonitoringSystem, numSystems)
	configs := make([]MonitoringConfig, numSystems)

	// Create multiple monitoring systems concurrently
	for i := 0; i < numSystems; i++ {
		configs[i] = DefaultMonitoringConfig()
		configs[i].LogLevel = zapcore.ErrorLevel
		configs[i].PrometheusNamespace = "test_concurrent"
		configs[i].PrometheusSubsystem = "system_" + string(rune('a'+i))

		var err error
		systems[i], err = NewMonitoringSystem(configs[i])
		if err != nil {
			t.Fatalf("Failed to create monitoring system %d: %v", i, err)
		}
	}

	// Cleanup
	defer func() {
		for _, system := range systems {
			if system != nil {
				system.Shutdown(context.Background())
			}
		}
	}()

	// All systems should record metrics without conflicts
	for i, system := range systems {
		testMemoryUsage := uint64(1024 * 1024 * (10 + i)) // Different values for each system
		system.RecordMemoryUsage(testMemoryUsage, int64(100+i), time.Millisecond)

		metrics := system.GetMetrics()
		if metrics.Memory.Usage != testMemoryUsage {
			t.Errorf("System %d: Expected memory usage %d, got %d", i, testMemoryUsage, metrics.Memory.Usage)
		}
	}
}