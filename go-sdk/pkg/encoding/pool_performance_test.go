package encoding

import (
	"bytes"
	"runtime"
	"testing"
	"unsafe"
)

// BenchmarkBufferZeringPerformance compares buffer zeroing vs non-zeroing
func BenchmarkBufferZeringPerformance(b *testing.B) {
	const bufferSize = 4096

	b.Run("WithZeroing", func(b *testing.B) {
		b.ReportAllocs()
		pool := NewBufferPoolWithOptions(bufferSize, 100, true) // secure mode enabled

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := pool.Get()
			if buf != nil {
				buf.Write(make([]byte, bufferSize/2)) // Fill half the buffer
				pool.Put(buf)
			}
		}
	})

	b.Run("WithoutZeroing", func(b *testing.B) {
		b.ReportAllocs()
		pool := NewBufferPoolWithOptions(bufferSize, 100, false) // fast mode

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := pool.Get()
			if buf != nil {
				buf.Write(make([]byte, bufferSize/2)) // Fill half the buffer
				pool.Put(buf)
			}
		}
	})
}

// BenchmarkSliceZeringPerformance compares slice zeroing vs non-zeroing
func BenchmarkSliceZeringPerformance(b *testing.B) {
	const sliceSize = 4096

	b.Run("WithZeroing", func(b *testing.B) {
		b.ReportAllocs()
		pool := NewSlicePoolWithOptions(1024, sliceSize, 100, true) // secure mode enabled

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			slice := pool.Get()
			if slice != nil {
				slice = append(slice, make([]byte, sliceSize/2)...) // Fill half the slice
				pool.Put(slice)
			}
		}
	})

	b.Run("WithoutZeroing", func(b *testing.B) {
		b.ReportAllocs()
		pool := NewSlicePoolWithOptions(1024, sliceSize, 100, false) // fast mode

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			slice := pool.Get()
			if slice != nil {
				slice = append(slice, make([]byte, sliceSize/2)...) // Fill half the slice
				pool.Put(slice)
			}
		}
	})
}

// BenchmarkGCPressureComparison measures GC impact
func BenchmarkGCPressureComparison(b *testing.B) {
	b.Run("PooledBuffers", func(b *testing.B) {
		pool := NewBufferPoolWithOptions(4096, 100, false)
		var m1, m2 runtime.MemStats

		runtime.GC()
		runtime.ReadMemStats(&m1)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			buf := pool.Get()
			if buf != nil {
				buf.WriteString("test data")
				pool.Put(buf)
			}
		}

		b.StopTimer()
		runtime.GC()
		runtime.ReadMemStats(&m2)

		b.ReportMetric(float64(m2.NumGC-m1.NumGC), "gc-runs")
		b.ReportMetric(float64(m2.PauseTotalNs-m1.PauseTotalNs)/1000000, "gc-pause-ms")
	})

	b.Run("NonPooledBuffers", func(b *testing.B) {
		var m1, m2 runtime.MemStats

		runtime.GC()
		runtime.ReadMemStats(&m1)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			buf := &bytes.Buffer{}
			buf.WriteString("test data")
		}

		b.StopTimer()
		runtime.GC()
		runtime.ReadMemStats(&m2)

		b.ReportMetric(float64(m2.NumGC-m1.NumGC), "gc-runs")
		b.ReportMetric(float64(m2.PauseTotalNs-m1.PauseTotalNs)/1000000, "gc-pause-ms")
	})
}

// BenchmarkErrorPooling tests error object pooling performance
func BenchmarkErrorPooling(b *testing.B) {
	b.Run("PooledErrors", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err := GetOperationError()
			err.Operation = "test"
			err.Component = "benchmark"
			err.Message = "test error"
			PutOperationError(err)
		}
	})

	b.Run("NonPooledErrors", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err := &OperationError{
				Operation: "test",
				Component: "benchmark",
				Message:   "test error",
			}
			_ = err
		}
	})
}

// BenchmarkMemoryUsage measures actual memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("OptimizedPools", func(b *testing.B) {
		pools := []*BufferPool{
			NewBufferPoolWithOptions(1024, 50, false),  // small, fast
			NewBufferPoolWithOptions(8192, 25, false),  // medium, fast
			NewBufferPoolWithOptions(65536, 10, false), // large, fast
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			poolIdx := i % len(pools)
			buf := pools[poolIdx].Get()
			if buf != nil {
				buf.Write(make([]byte, 512)) // Write some data
				pools[poolIdx].Put(buf)
			}
		}
	})

	b.Run("LegacyPools", func(b *testing.B) {
		pools := []*BufferPool{
			NewBufferPoolWithOptions(1024, 50, true),  // small, with zeroing
			NewBufferPoolWithOptions(8192, 25, true),  // medium, with zeroing
			NewBufferPoolWithOptions(65536, 10, true), // large, with zeroing
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			poolIdx := i % len(pools)
			buf := pools[poolIdx].Get()
			if buf != nil {
				buf.Write(make([]byte, 512)) // Write some data
				pools[poolIdx].Put(buf)
			}
		}
	})
}

// BenchmarkConcurrentPoolAccess tests concurrent access performance
func BenchmarkConcurrentPoolAccess(b *testing.B) {
	pool := NewBufferPoolWithOptions(4096, 200, false)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
			if buf != nil {
				buf.WriteString("concurrent test data")
				pool.Put(buf)
			}
		}
	})
}

// BenchmarkStructuredErrorTypes compares structured vs simple errors
func BenchmarkStructuredErrorTypes(b *testing.B) {
	b.Run("StructuredErrors", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err := NewOperationError("encode", "json", "test error", nil)
			err.WithContext("attempt", i)
			_ = err.Error()
		}
	})

	b.Run("SimpleErrors", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			err := &EncodingError{
				Format:  "json",
				Message: "test error",
			}
			_ = err.Error()
		}
	})
}

// BenchmarkZeroingMethods compares different zeroing approaches
func BenchmarkZeroingMethods(b *testing.B) {
	const size = 4096
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256) // Fill with test data
	}

	b.Run("LoopZeroing", func(b *testing.B) {
		testData := make([]byte, size)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			copy(testData, data)
			// Manual loop zeroing (current approach)
			for j := range testData {
				testData[j] = 0
			}
		}
	})

	b.Run("BuiltinClear", func(b *testing.B) {
		testData := make([]byte, size)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			copy(testData, data)
			// Using built-in clear (Go 1.21+, if available)
			clear(testData)
		}
	})

	b.Run("UnsafeZeroing", func(b *testing.B) {
		testData := make([]byte, size)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			copy(testData, data)
			// Unsafe memory zeroing (fastest but unsafe)
			if len(testData) > 0 {
				// Zero out memory using unsafe operations
				ptr := unsafe.Pointer(&testData[0])
				for i := uintptr(0); i < uintptr(len(testData)); i++ {
					*(*byte)(unsafe.Pointer(uintptr(ptr) + i)) = 0
				}
			}
		}
	})
}
