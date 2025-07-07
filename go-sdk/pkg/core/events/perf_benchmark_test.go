package events

import (
	"sync"
	"testing"
	"time"
)

// Simple mutex-based counter for comparison
type SimpleCounter struct {
	value int64
	mutex sync.RWMutex
}

func (c *SimpleCounter) Inc() {
	c.mutex.Lock()
	c.value++
	c.mutex.Unlock()
}

func (c *SimpleCounter) Load() int64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.value
}

// BenchmarkAtomicVsMutexCounter compares atomic vs mutex counter performance
func BenchmarkAtomicVsMutexCounter(b *testing.B) {
	b.Run("AtomicCounter", func(b *testing.B) {
		counter := NewAtomicCounter()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				counter.Inc()
			}
		})
	})
	
	b.Run("MutexCounter", func(b *testing.B) {
		counter := &SimpleCounter{}
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				counter.Inc()
			}
		})
	})
}

// BenchmarkAtomicVsMutexReadPerf compares read performance
func BenchmarkAtomicVsMutexReadPerf(b *testing.B) {
	atomicCounter := NewAtomicCounter()
	mutexCounter := &SimpleCounter{}
	
	// Pre-populate
	for i := 0; i < 1000; i++ {
		atomicCounter.Inc()
		mutexCounter.Inc()
	}
	
	b.Run("AtomicRead", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = atomicCounter.Load()
			}
		})
	})
	
	b.Run("MutexRead", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = mutexCounter.Load()
			}
		})
	})
}

// BenchmarkRuleMetricComparison compares atomic vs mutex rule metrics
func BenchmarkRuleMetricComparison(b *testing.B) {
	duration := 100 * time.Millisecond
	
	b.Run("AtomicRuleMetric", func(b *testing.B) {
		metric := NewRuleExecutionMetric("test")
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				metric.RecordExecution(duration, true)
			}
		})
	})
	
	b.Run("MutexRuleMetric", func(b *testing.B) {
		metric := NewMutexRuleMetric("test")
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				metric.RecordExecution(duration, true)
			}
		})
	})
}