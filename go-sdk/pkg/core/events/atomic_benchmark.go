package events

import (
	"sync"
	"testing"
	"time"
)

// MutexRuleMetric represents the old mutex-based implementation for comparison
type MutexRuleMetric struct {
	RuleID        string
	ExecutionCount int64
	TotalDuration time.Duration
	ErrorCount    int64
	mutex         sync.RWMutex
}

func NewMutexRuleMetric(ruleID string) *MutexRuleMetric {
	return &MutexRuleMetric{
		RuleID: ruleID,
	}
}

func (m *MutexRuleMetric) RecordExecution(duration time.Duration, success bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.ExecutionCount++
	m.TotalDuration += duration
	
	if !success {
		m.ErrorCount++
	}
}

func (m *MutexRuleMetric) GetExecutionCount() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.ExecutionCount
}

// BenchmarkAtomicVsMutexRuleMetric compares atomic vs mutex implementations
func BenchmarkAtomicVsMutexRuleMetric(b *testing.B) {
	duration := 100 * time.Millisecond
	
	b.Run("AtomicRuleMetric", func(b *testing.B) {
		metric := NewRuleExecutionMetric("atomic-test")
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				metric.RecordExecution(duration, true)
			}
		})
	})
	
	b.Run("MutexRuleMetric", func(b *testing.B) {
		metric := NewMutexRuleMetric("mutex-test")
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				metric.RecordExecution(duration, true)
			}
		})
	})
}

func BenchmarkAtomicVsMutexRead(b *testing.B) {
	// Setup
	atomicMetric := NewRuleExecutionMetric("atomic-test")
	mutexMetric := NewMutexRuleMetric("mutex-test")
	
	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		atomicMetric.RecordExecution(100*time.Millisecond, true)
		mutexMetric.RecordExecution(100*time.Millisecond, true)
	}
	
	b.Run("AtomicRead", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = atomicMetric.GetExecutionCount()
			}
		})
	})
	
	b.Run("MutexRead", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_ = mutexMetric.GetExecutionCount()
			}
		})
	})
}