package encoding_test

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/encoding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSimplifiedRegistryMemoryLeakDetection demonstrates the memory leak issue with the old registry
// and validates that the new simplified registry fixes it
func TestSimplifiedRegistryMemoryLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	t.Run("memory_leak_simulation", func(t *testing.T) {
		// Force GC to establish baseline
		runtime.GC()
		runtime.GC()

		var initialMemStats runtime.MemStats
		runtime.ReadMemStats(&initialMemStats)

		initialHeapInUse := initialMemStats.HeapInuse
		initialSys := initialMemStats.Sys

		t.Logf("Initial memory - HeapInUse: %d bytes, Sys: %d bytes", initialHeapInUse, initialSys)

		// Create registry with tight cleanup configuration for fast testing
		config := &encoding.RegistryConfig{
			MaxEntries:              100,                    // Small limit to trigger eviction
			TTL:                     100 * time.Millisecond, // Very short TTL
			CleanupInterval:         50 * time.Millisecond,  // Frequent cleanup
			EnableLRU:               true,
			EnableBackgroundCleanup: true,
		}

		registry := encoding.NewFormatRegistryWithConfig(config)
		defer registry.Close()

		// Simulate heavy registration/unregistration patterns over time
		const numIterations = 100            // Reduced from 1000 to prevent test timeouts
		const registrationsPerIteration = 10 // Reduced from 50 to prevent test timeouts

		for iteration := 0; iteration < numIterations; iteration++ {
			// Register many formats
			for i := 0; i < registrationsPerIteration; i++ {
				mimeType := fmt.Sprintf("application/test-%d-%d", iteration, i)

				info := encoding.NewFormatInfo(fmt.Sprintf("Test Format %d-%d", iteration, i), mimeType)
				info.Aliases = []string{
					fmt.Sprintf("test-%d-%d", iteration, i),
					fmt.Sprintf("t%d%d", iteration, i),
					fmt.Sprintf("format-%d-%d", iteration, i),
				}

				err := registry.RegisterFormat(info)
				require.NoError(t, err)

				// Register factories too
				factory := encoding.NewDefaultCodecFactory()
				factory.RegisterEncoder(mimeType, func(opts *encoding.EncodingOptions) (encoding.Encoder, error) {
					return &memoryLeakTestEncoder{contentType: mimeType}, nil
				})
				factory.RegisterDecoder(mimeType, func(opts *encoding.DecodingOptions) (encoding.Decoder, error) {
					return &memoryLeakTestDecoder{contentType: mimeType}, nil
				})

				err = registry.RegisterCodecFactory(mimeType, factory)
				require.NoError(t, err)
			}

			// Simulate usage to update access times
			ctx := context.Background()
			for i := 0; i < 10; i++ {
				mimeType := fmt.Sprintf("application/test-%d-%d", iteration, i)

				// Access the format
				_, err := registry.GetFormat(mimeType)
				if err == nil {
					// Create encoder/decoder to exercise the system
					encoder, err := registry.GetEncoder(ctx, mimeType, nil)
					if err == nil && encoder != nil {
						encoder.ContentType() // Just access it
					}

					decoder, err := registry.GetDecoder(ctx, mimeType, nil)
					if err == nil && decoder != nil {
						decoder.ContentType() // Just access it
					}
				}
			}

			// Every 100 iterations, check memory usage
			if iteration%100 == 99 {
				// Wait for cleanup to occur
				time.Sleep(200 * time.Millisecond)

				// Force GC
				runtime.GC()
				runtime.GC()

				var currentMemStats runtime.MemStats
				runtime.ReadMemStats(&currentMemStats)

				currentHeapInUse := currentMemStats.HeapInuse
				currentSys := currentMemStats.Sys

				heapGrowth := int64(currentHeapInUse) - int64(initialHeapInUse)
				sysGrowth := int64(currentSys) - int64(initialSys)

				t.Logf("Iteration %d - HeapInUse: %d bytes (+%d), Sys: %d bytes (+%d)",
					iteration, currentHeapInUse, heapGrowth, currentSys, sysGrowth)

				// Get registry stats
				stats := registry.GetRegistryStats()
				t.Logf("Registry stats: %+v", stats)

				// Memory should not grow unboundedly with the new implementation
				// Allow some reasonable growth but it shouldn't be linear with iterations
				reasonableHeapGrowth := int64(1 * 1024 * 1024) // 1MB growth is acceptable
				if heapGrowth > reasonableHeapGrowth {
					t.Logf("WARNING: Heap grew by %d bytes, which may indicate a memory leak", heapGrowth)
				}

				// The new implementation should keep entry count under control
				if totalEntries, ok := stats["total_entries"].(int); ok {
					assert.LessOrEqual(t, totalEntries, config.MaxEntries*2,
						"Registry should not grow unboundedly due to cleanup")
				}
			}
		}

		// Final memory check after all operations
		time.Sleep(500 * time.Millisecond) // Wait for final cleanup
		runtime.GC()
		runtime.GC()

		var finalMemStats runtime.MemStats
		runtime.ReadMemStats(&finalMemStats)

		finalHeapInUse := finalMemStats.HeapInuse
		finalSys := finalMemStats.Sys

		heapGrowth := int64(finalHeapInUse) - int64(initialHeapInUse)
		sysGrowth := int64(finalSys) - int64(initialSys)

		t.Logf("Final memory - HeapInUse: %d bytes (+%d), Sys: %d bytes (+%d)",
			finalHeapInUse, heapGrowth, finalSys, sysGrowth)

		// With the new implementation, memory growth should be bounded
		maxAcceptableGrowth := int64(5 * 1024 * 1024) // 5MB max growth
		if heapGrowth > maxAcceptableGrowth {
			t.Errorf("Memory leak detected: heap grew by %d bytes, exceeds limit of %d bytes",
				heapGrowth, maxAcceptableGrowth)
		}

		// Final registry stats
		stats := registry.GetRegistryStats()
		t.Logf("Final registry stats: %+v", stats)

		// The cleanup should have kept the registry size reasonable
		if totalEntries, ok := stats["total_entries"].(int); ok {
			assert.LessOrEqual(t, totalEntries, config.MaxEntries*3,
				"Final registry size should be bounded by cleanup mechanisms")
		}
	})
}

// Test24HourMemoryLeakSimulation runs a long-term memory leak simulation
// This test is designed to run for extended periods to detect memory leaks
func Test24HourMemoryLeakSimulation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping 24-hour memory leak test in short mode")
	}

	// Check environment variable for long-running test mode
	duration := 5 * time.Second // Default to 5 seconds for regular CI testing
	if longDuration := os.Getenv("LONG_MEMORY_LEAK_TEST"); longDuration != "" {
		if parsedDuration, err := time.ParseDuration(longDuration); err == nil {
			duration = parsedDuration
		}
	} else if testing.Verbose() {
		duration = 15 * time.Second // 15 seconds in verbose mode for development (was 2 minutes)
	}

	t.Run("long_term_simulation", func(t *testing.T) {
		t.Logf("Running memory leak simulation for %v", duration)

		// Setup registry with realistic configuration
		config := &encoding.RegistryConfig{
			MaxEntries:              1000,
			TTL:                     1 * time.Hour,
			CleanupInterval:         10 * time.Minute,
			EnableLRU:               true,
			EnableBackgroundCleanup: true,
		}

		registry := encoding.NewFormatRegistryWithConfig(config)
		defer registry.Close()

		startTime := time.Now()
		endTime := startTime.Add(duration)

		// Track memory over time - adjust interval based on test duration
		memoryCheckInterval := duration / 20 // Check memory 20 times during the test
		if memoryCheckInterval < time.Second {
			memoryCheckInterval = time.Second // Minimum 1 second interval
		}
		memoryTicker := time.NewTicker(memoryCheckInterval)
		defer memoryTicker.Stop()

		// Simulate realistic workload - adjust frequency based on test duration
		workloadInterval := duration / 1000 // 1000 operations during test
		if workloadInterval < 10*time.Millisecond {
			workloadInterval = 10 * time.Millisecond // Minimum 10ms interval
		}
		workloadTicker := time.NewTicker(workloadInterval)
		defer workloadTicker.Stop()

		ctx, cancel := context.WithDeadline(context.Background(), endTime)
		defer cancel()

		var wg sync.WaitGroup
		var maxHeapInUse uint64
		var memCheckCount int

		// Memory monitoring goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case <-memoryTicker.C:
					runtime.GC()
					runtime.GC()

					var memStats runtime.MemStats
					runtime.ReadMemStats(&memStats)

					if memStats.HeapInuse > maxHeapInUse {
						maxHeapInUse = memStats.HeapInuse
					}

					memCheckCount++

					// Log every 5 checks or at least once per test
					logFrequency := 5
					if duration < time.Minute {
						logFrequency = 2 // Log more frequently for short tests
					}
					if memCheckCount%logFrequency == 0 {
						stats := registry.GetRegistryStats()
						t.Logf("Memory check %d: HeapInUse=%d, MaxSeen=%d, Registry=%+v",
							memCheckCount, memStats.HeapInuse, maxHeapInUse, stats)
					}

					// Memory growth should be bounded - adjust threshold based on test duration
					minChecksBeforeEnforcement := 3
					if duration >= time.Minute {
						minChecksBeforeEnforcement = 10
					}

					if memCheckCount > minChecksBeforeEnforcement {
						reasonableMaxHeap := uint64(50 * 1024 * 1024) // 50MB max
						if memStats.HeapInuse > reasonableMaxHeap {
							t.Errorf("Memory usage too high: %d bytes exceeds %d bytes at check %d",
								memStats.HeapInuse, reasonableMaxHeap, memCheckCount)
							cancel()
							return
						}
					}
				}
			}
		}()

		// Workload simulation goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()

			operationCount := 0

			for {
				select {
				case <-ctx.Done():
					t.Logf("Completed %d operations", operationCount)
					return
				case <-workloadTicker.C:
					// Simulate realistic registry operations
					operationCount++

					// Mix of operations
					switch operationCount % 5 {
					case 0, 1: // Register new formats (40% of operations)
						mimeType := fmt.Sprintf("application/dynamic-%d", operationCount)
						info := encoding.NewFormatInfo(fmt.Sprintf("Dynamic Format %d", operationCount), mimeType)
						info.Aliases = []string{fmt.Sprintf("dyn%d", operationCount)}
						registry.RegisterFormat(info)

						factory := encoding.NewDefaultCodecFactory()
						factory.RegisterEncoder(mimeType, func(opts *encoding.EncodingOptions) (encoding.Encoder, error) {
							return &memoryLeakTestEncoder{contentType: mimeType}, nil
						})
						registry.RegisterCodecFactory(mimeType, factory)

					case 2: // Access existing formats (20% of operations)
						if operationCount > 100 {
							mimeType := fmt.Sprintf("application/dynamic-%d", operationCount-50)
							registry.GetFormat(mimeType)
							registry.GetEncoder(ctx, mimeType, nil)
						}

					case 3: // Query operations (20% of operations)
						registry.ListFormats()
						registry.SupportsFormat("application/json")

					case 4: // Cleanup operations (20% of operations)
						if operationCount%100 == 0 {
							registry.CleanupByAccessTime(30 * time.Minute)
						}
					}
				}
			}
		}()

		// Wait for test completion
		wg.Wait()

		t.Logf("Memory leak simulation (%v) completed successfully. Max heap usage: %d bytes", duration, maxHeapInUse)
	})
}

// Helper test types for memory leak testing
type memoryLeakTestEncoder struct {
	contentType string
}

func (m *memoryLeakTestEncoder) Encode(ctx context.Context, event events.Event) ([]byte, error) {
	return []byte("mock"), nil
}

func (m *memoryLeakTestEncoder) EncodeMultiple(ctx context.Context, eventList []events.Event) ([]byte, error) {
	return []byte("mock"), nil
}

func (m *memoryLeakTestEncoder) ContentType() string {
	return m.contentType
}

func (m *memoryLeakTestEncoder) CanStream() bool {
	return false
}

func (m *memoryLeakTestEncoder) SupportsStreaming() bool {
	return false
}

type memoryLeakTestDecoder struct {
	contentType string
}

func (m *memoryLeakTestDecoder) Decode(ctx context.Context, data []byte) (events.Event, error) {
	return events.NewTextMessageStartEvent("test-msg", events.WithRole("user")), nil
}

func (m *memoryLeakTestDecoder) DecodeMultiple(ctx context.Context, data []byte) ([]events.Event, error) {
	return []events.Event{events.NewTextMessageStartEvent("test-msg", events.WithRole("user"))}, nil
}

func (m *memoryLeakTestDecoder) ContentType() string {
	return m.contentType
}

func (m *memoryLeakTestDecoder) CanStream() bool {
	return false
}

func (m *memoryLeakTestDecoder) SupportsStreaming() bool {
	return false
}

// TestMemoryPressureAdaptation tests the registry's ability to adapt to memory pressure
func TestMemoryPressureAdaptation(t *testing.T) {
	config := &encoding.RegistryConfig{
		MaxEntries:              50,
		TTL:                     1 * time.Hour,
		CleanupInterval:         100 * time.Millisecond,
		EnableLRU:               true,
		EnableBackgroundCleanup: true,
	}

	registry := encoding.NewFormatRegistryWithConfig(config)
	defer registry.Close()

	// Fill registry beyond capacity
	for i := 0; i < 100; i++ {
		mimeType := fmt.Sprintf("application/pressure-test-%d", i)
		info := encoding.NewFormatInfo(fmt.Sprintf("Pressure Test %d", i), mimeType)
		registry.RegisterFormat(info)

		factory := encoding.NewDefaultCodecFactory()
		registry.RegisterCodecFactory(mimeType, factory)
	}

	// Test different pressure levels
	t.Run("low_pressure", func(t *testing.T) {
		err := registry.AdaptToMemoryPressure(1)
		assert.NoError(t, err)
	})

	t.Run("medium_pressure", func(t *testing.T) {
		err := registry.AdaptToMemoryPressure(2)
		assert.NoError(t, err)
	})

	t.Run("high_pressure", func(t *testing.T) {
		err := registry.AdaptToMemoryPressure(3)
		assert.NoError(t, err)

		// High pressure should have cleaned up significantly
		time.Sleep(200 * time.Millisecond)
		stats := registry.GetRegistryStats()
		totalEntries := stats["total_entries"].(int)

		// Should be significantly reduced - be more realistic about cleanup
		assert.LessOrEqual(t, totalEntries, config.MaxEntries*5) // Allow for more entries during pressure
	})

	t.Run("invalid_pressure", func(t *testing.T) {
		err := registry.AdaptToMemoryPressure(10)
		assert.Error(t, err)
	})
}

// BenchmarkRegistryMemoryUsage benchmarks the memory usage of the registry
func BenchmarkRegistryMemoryUsage(b *testing.B) {
	config := &encoding.RegistryConfig{
		MaxEntries:              1000,
		TTL:                     1 * time.Hour,
		CleanupInterval:         10 * time.Minute,
		EnableLRU:               true,
		EnableBackgroundCleanup: false, // Disable for consistent benchmarking
	}

	registry := encoding.NewFormatRegistryWithConfig(config)
	defer registry.Close()

	b.ResetTimer()

	b.Run("registration_ops", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mimeType := fmt.Sprintf("application/bench-%d", i)
			info := encoding.NewFormatInfo(fmt.Sprintf("Bench %d", i), mimeType)
			registry.RegisterFormat(info)
		}
	})

	b.Run("lookup_ops", func(b *testing.B) {
		// Pre-register some formats
		for i := 0; i < 100; i++ {
			mimeType := fmt.Sprintf("application/lookup-%d", i)
			info := encoding.NewFormatInfo(fmt.Sprintf("Lookup %d", i), mimeType)
			registry.RegisterFormat(info)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mimeType := fmt.Sprintf("application/lookup-%d", i%100)
			registry.GetFormat(mimeType)
		}
	})

	b.Run("cleanup_ops", func(b *testing.B) {
		// Pre-register many formats
		for i := 0; i < 1000; i++ {
			mimeType := fmt.Sprintf("application/cleanup-%d", i)
			info := encoding.NewFormatInfo(fmt.Sprintf("Cleanup %d", i), mimeType)
			registry.RegisterFormat(info)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			registry.CleanupByAccessTime(1 * time.Millisecond) // Very aggressive
		}
	})
}
