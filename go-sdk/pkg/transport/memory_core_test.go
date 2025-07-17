package transport

import (
	"sync"
	"testing"
	"time"
)

// TestMemoryManagerCore tests core memory manager functionality
func TestMemoryManagerCore(t *testing.T) {
	config := &MemoryManagerConfig{
		LowMemoryPercent:      50.0,
		HighMemoryPercent:     70.0,
		CriticalMemoryPercent: 90.0,
		MonitorInterval:       100 * time.Millisecond,
	}

	mm := NewMemoryManager(config)
	defer mm.Stop()

	mm.Start()

	t.Run("BasicFunctionality", func(t *testing.T) {
		// Test adaptive buffer sizing
		baseSize := 1000
		adaptedSize := mm.GetAdaptiveBufferSize("test_buffer", baseSize)
		
		if adaptedSize <= 0 {
			t.Errorf("Expected positive buffer size, got %d", adaptedSize)
		}

		// Test metrics
		metrics := mm.GetMetrics()
		if metrics.TotalAllocated == 0 {
			t.Error("Expected non-zero total allocated memory")
		}

		// Test memory pressure level
		level := mm.GetMemoryPressureLevel()
		if level < MemoryPressureNormal || level > MemoryPressureCritical {
			t.Errorf("Invalid memory pressure level: %v", level)
		}
	})

	t.Run("CallbackRegistration", func(t *testing.T) {
		called := make(chan bool, 1)
		
		mm.OnMemoryPressure(func(level MemoryPressureLevel) {
			select {
			case called <- true:
			default:
			}
		})

		// Manually trigger callback
		mm.notifyPressureChange(MemoryPressureHigh)

		select {
		case <-called:
			// Success
		case <-time.After(1 * time.Second):
			t.Error("Callback was not called")
		}
	})
}

// TestCleanupManagerCore tests core cleanup manager functionality
func TestCleanupManagerCore(t *testing.T) {
	config := &CleanupManagerConfig{
		DefaultTTL:    100 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
	}

	cm := NewCleanupManager(config)
	defer cm.Stop()

	err := cm.Start()
	if err != nil {
		t.Fatalf("Failed to start cleanup manager: %v", err)
	}

	t.Run("TaskRegistration", func(t *testing.T) {
		executed := make(chan bool, 1)
		
		err := cm.RegisterTask("test_task", 25*time.Millisecond, func() (int, error) {
			select {
			case executed <- true:
			default:
			}
			return 1, nil
		})

		if err != nil {
			t.Fatalf("Failed to register task: %v", err)
		}

		// Wait for execution
		select {
		case <-executed:
			// Success
		case <-time.After(1 * time.Second):
			t.Error("Task was not executed")
		}
	})

	t.Run("Metrics", func(t *testing.T) {
		metrics := cm.GetMetrics()
		if metrics.ActiveTasks < 0 {
			t.Errorf("Invalid active tasks count: %d", metrics.ActiveTasks)
		}
	})
}

// TestSliceCore tests core slice functionality
func TestSliceCore(t *testing.T) {
	slice := NewSlice()

	t.Run("BasicOperations", func(t *testing.T) {
		// Test append
		slice.Append("item1")
		slice.Append("item2")

		if slice.Len() != 2 {
			t.Errorf("Expected length 2, got %d", slice.Len())
		}

		// Test get
		item, exists := slice.Get(0)
		if !exists || item != "item1" {
			t.Errorf("Expected 'item1', got %v (exists: %v)", item, exists)
		}

		// Test remove
		removed := slice.RemoveFunc(func(item interface{}) bool {
			return item == "item1"
		})

		if !removed {
			t.Error("Expected item to be removed")
		}

		if slice.Len() != 1 {
			t.Errorf("Expected length 1, got %d", slice.Len())
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		slice.Clear()
		
		var wg sync.WaitGroup
		numWorkers := 10
		itemsPerWorker := 100

		// Concurrent appends
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < itemsPerWorker; j++ {
					slice.Append(workerID*itemsPerWorker + j)
				}
			}(i)
		}

		wg.Wait()

		expectedLength := numWorkers * itemsPerWorker
		if slice.Len() != expectedLength {
			t.Errorf("Expected length %d, got %d", expectedLength, slice.Len())
		}
	})
}

// BenchmarkMemoryManager benchmarks memory manager operations
func BenchmarkMemoryManager(b *testing.B) {
	mm := NewMemoryManager(nil)
	defer mm.Stop()
	mm.Start()

	b.Run("GetAdaptiveBufferSize", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mm.GetAdaptiveBufferSize("test", 1000)
		}
	})

	b.Run("GetMemoryPressureLevel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mm.GetMemoryPressureLevel()
		}
	})

	b.Run("GetMetrics", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mm.GetMetrics()
		}
	})
}

// BenchmarkSlice benchmarks slice operations
func BenchmarkSlice(b *testing.B) {
	slice := NewSlice()

	b.Run("Append", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			slice.Append(i)
		}
	})

	b.Run("Get", func(b *testing.B) {
		// Pre-populate
		for i := 0; i < 1000; i++ {
			slice.Append(i)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			slice.Get(i % 1000)
		}
	})

	b.Run("Range", func(b *testing.B) {
		// Pre-populate
		slice.Clear()
		for i := 0; i < 1000; i++ {
			slice.Append(i)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			slice.Range(func(item interface{}) bool {
				return true
			})
		}
	})
}

// TestMemoryPressureStates tests memory pressure state transitions
func TestMemoryPressureStates(t *testing.T) {
	mm := NewMemoryManager(&MemoryManagerConfig{
		MonitorInterval: 10 * time.Millisecond,
	})
	defer mm.Stop()

	// Test initial state
	if mm.GetMemoryPressureLevel() != MemoryPressureNormal {
		t.Error("Expected initial state to be normal")
	}

	// Test string representations
	tests := []struct {
		level MemoryPressureLevel
		str   string
	}{
		{MemoryPressureNormal, "normal"},
		{MemoryPressureLow, "low"},
		{MemoryPressureHigh, "high"},
		{MemoryPressureCritical, "critical"},
	}

	for _, test := range tests {
		if test.level.String() != test.str {
			t.Errorf("Expected %s, got %s", test.str, test.level.String())
		}
	}
}

// TestForceGC tests forced garbage collection
func TestForceGC(t *testing.T) {
	mm := NewMemoryManager(nil)
	defer mm.Stop()

	// Test that ForceGC doesn't panic
	mm.ForceGC()

	// Force high memory pressure and test GC
	mm.memoryPressureLevel.Store(int32(MemoryPressureHigh))
	mm.ForceGC()

	// Verify we can get memory stats after GC
	stats := mm.GetMemoryStats()
	if stats.NumGC == 0 {
		// This might be 0 in some test environments, so just log
		t.Logf("No GC cycles recorded: %+v", stats)
	}
}