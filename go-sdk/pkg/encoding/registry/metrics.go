package registry

import (
	"sync/atomic"
	"time"
)

// Metrics handles statistics and monitoring with enhanced leak prevention tracking
type Metrics struct {
	entryCount               int64 // atomic counter for better memory pressure detection
	cleanupOperations        int64 // total cleanup operations performed
	lastCleanupTime          int64 // atomic: Unix nano timestamp of last cleanup
	totalCleaned            int64 // total entries cleaned up
	memoryPressureAdaptations int64 // number of memory pressure adaptations
	lastMemoryPressureLevel  int32 // last memory pressure level handled
	maxEntriesReached       int64 // number of times max entries was reached
}

// NewMetrics creates a new metrics manager with enhanced tracking
func NewMetrics() *Metrics {
	return &Metrics{}
}

// IncrementEntryCount atomically increments the entry count
func (m *Metrics) IncrementEntryCount() {
	atomic.AddInt64(&m.entryCount, 1)
}

// DecrementEntryCount atomically decrements the entry count
func (m *Metrics) DecrementEntryCount() {
	atomic.AddInt64(&m.entryCount, -1)
}

// GetEntryCount returns the current entry count
func (m *Metrics) GetEntryCount() int64 {
	return atomic.LoadInt64(&m.entryCount)
}

// ResetEntryCount resets the entry count to zero
func (m *Metrics) ResetEntryCount() {
	atomic.StoreInt64(&m.entryCount, 0)
}

// Reset resets all metrics
func (m *Metrics) Reset() {
	atomic.StoreInt64(&m.entryCount, 0)
	atomic.StoreInt64(&m.cleanupOperations, 0)
	atomic.StoreInt64(&m.lastCleanupTime, 0)
	atomic.StoreInt64(&m.totalCleaned, 0)
	atomic.StoreInt64(&m.memoryPressureAdaptations, 0)
	atomic.StoreInt32(&m.lastMemoryPressureLevel, 0)
	atomic.StoreInt64(&m.maxEntriesReached, 0)
}

// RecordCleanupOperation records a cleanup operation with metrics
func (m *Metrics) RecordCleanupOperation(cleaned int, duration time.Duration) {
	atomic.AddInt64(&m.cleanupOperations, 1)
	atomic.StoreInt64(&m.lastCleanupTime, time.Now().UnixNano())
	atomic.AddInt64(&m.totalCleaned, int64(cleaned))
}

// RecordMemoryPressureAdaptation records memory pressure adaptation
func (m *Metrics) RecordMemoryPressureAdaptation(level int, cleaned int, duration time.Duration) {
	atomic.AddInt64(&m.memoryPressureAdaptations, 1)
	atomic.StoreInt32(&m.lastMemoryPressureLevel, int32(level))
	atomic.AddInt64(&m.totalCleaned, int64(cleaned))
}

// RecordMaxEntriesReached records when max entries limit is hit
func (m *Metrics) RecordMaxEntriesReached() {
	atomic.AddInt64(&m.maxEntriesReached, 1)
}

// GetCleanupOperations returns the total number of cleanup operations
func (m *Metrics) GetCleanupOperations() int64 {
	return atomic.LoadInt64(&m.cleanupOperations)
}

// GetLastCleanupTime returns the time of the last cleanup
func (m *Metrics) GetLastCleanupTime() time.Time {
	nanos := atomic.LoadInt64(&m.lastCleanupTime)
	if nanos == 0 {
		return time.Time{}
	}
	return time.Unix(0, nanos)
}

// GetTotalCleaned returns the total number of entries cleaned
func (m *Metrics) GetTotalCleaned() int64 {
	return atomic.LoadInt64(&m.totalCleaned)
}

// GetMemoryPressureAdaptations returns the number of memory pressure adaptations
func (m *Metrics) GetMemoryPressureAdaptations() int64 {
	return atomic.LoadInt64(&m.memoryPressureAdaptations)
}

// GetLastMemoryPressureLevel returns the last memory pressure level handled
func (m *Metrics) GetLastMemoryPressureLevel() int {
	return int(atomic.LoadInt32(&m.lastMemoryPressureLevel))
}

// GetMaxEntriesReached returns the number of times max entries was reached
func (m *Metrics) GetMaxEntriesReached() int64 {
	return atomic.LoadInt64(&m.maxEntriesReached)
}

// GetHealthMetrics returns comprehensive health metrics for monitoring
func (m *Metrics) GetHealthMetrics() map[string]interface{} {
	return map[string]interface{}{
		"entry_count":                   m.GetEntryCount(),
		"cleanup_operations":            m.GetCleanupOperations(),
		"last_cleanup_time":             m.GetLastCleanupTime(),
		"total_cleaned":                 m.GetTotalCleaned(),
		"memory_pressure_adaptations":   m.GetMemoryPressureAdaptations(),
		"last_memory_pressure_level":    m.GetLastMemoryPressureLevel(),
		"max_entries_reached_count":     m.GetMaxEntriesReached(),
	}
}
