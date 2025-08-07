package encoding

import (
	"runtime"
	"testing"
)

// BenchmarkBufferPooling benchmarks buffer pooling
func BenchmarkSimpleBufferPooling(b *testing.B) {
	b.Run("WithoutPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf := make([]byte, 1024)
			_ = buf
		}
	})

	b.Run("WithPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buf := GetBuffer(1024)
			PutBuffer(buf)
		}
	})
}

// BenchmarkSlicePooling benchmarks slice pooling
func BenchmarkSimpleSlicePooling(b *testing.B) {
	b.Run("WithoutPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			slice := make([]byte, 0, 1024)
			_ = slice
		}
	})

	b.Run("WithPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			slice := GetSlice(1024)
			PutSlice(slice)
		}
	})
}

// BenchmarkErrorPooling benchmarks error pooling
func BenchmarkSimpleErrorPooling(b *testing.B) {
	b.Run("WithoutPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			err := &EncodingError{
				Format:  "test",
				Message: "test error",
			}
			_ = err
		}
	})

	b.Run("WithPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			err := GetEncodingError()
			err.Format = "test"
			err.Message = "test error"
			PutEncodingError(err)
		}
	})
}

// BenchmarkGCPressure measures GC pressure with and without pooling
func BenchmarkSimpleGCPressure(b *testing.B) {
	b.Run("WithoutPool", func(b *testing.B) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := make([]byte, 1024)
			_ = buf
		}
		b.StopTimer()

		runtime.GC()
		runtime.ReadMemStats(&m2)

		b.ReportMetric(float64(m2.NumGC-m1.NumGC), "gc-runs")
		b.ReportMetric(float64(m2.PauseTotalNs-m1.PauseTotalNs)/1000000, "gc-pause-ms")
	})

	b.Run("WithPool", func(b *testing.B) {
		var m1, m2 runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m1)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := GetBuffer(1024)
			PutBuffer(buf)
		}
		b.StopTimer()

		runtime.GC()
		runtime.ReadMemStats(&m2)

		b.ReportMetric(float64(m2.NumGC-m1.NumGC), "gc-runs")
		b.ReportMetric(float64(m2.PauseTotalNs-m1.PauseTotalNs)/1000000, "gc-pause-ms")
	})
}

// BenchmarkConcurrentUsage benchmarks concurrent usage of pools
func BenchmarkSimpleConcurrentUsage(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := GetBuffer(1024)
			buf.WriteString("test")
			PutBuffer(buf)
		}
	})
}
