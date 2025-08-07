package registry

import "sync/atomic"

// Metrics handles statistics and monitoring
type Metrics struct {
	entryCount int64 // atomic counter for better memory pressure detection
}

// NewMetrics creates a new metrics manager
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
}
