package transport

import (
	"sync"
	"unsafe"
)

// SlicePool manages pools of slices with different capacities
type SlicePool[T any] struct {
	pools map[int]*sync.Pool
	mutex sync.RWMutex
}

// NewSlicePool creates a new slice pool for the given type
func NewSlicePool[T any]() *SlicePool[T] {
	return &SlicePool[T]{
		pools: make(map[int]*sync.Pool),
	}
}

// getPool returns a pool for the given capacity
func (sp *SlicePool[T]) getPool(capacity int) *sync.Pool {
	sp.mutex.RLock()
	if pool, exists := sp.pools[capacity]; exists {
		sp.mutex.RUnlock()
		return pool
	}
	sp.mutex.RUnlock()

	// Create new pool
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	// Double-check after acquiring write lock
	if pool, exists := sp.pools[capacity]; exists {
		return pool
	}

	pool := &sync.Pool{
		New: func() interface{} {
			return make([]T, 0, capacity)
		},
	}
	sp.pools[capacity] = pool
	return pool
}

// Get returns a slice with the given capacity
func (sp *SlicePool[T]) Get(capacity int) []T {
	// Round up to nearest power of 2 for better pooling
	cap := roundUpPowerOf2(capacity)
	pool := sp.getPool(cap)
	slice := pool.Get().([]T)
	return slice[:0] // Reset length but keep capacity
}

// Put returns a slice to the pool
func (sp *SlicePool[T]) Put(slice []T) {
	if slice == nil {
		return
	}

	// Clear the slice to avoid memory leaks
	for i := range slice {
		var zero T
		slice[i] = zero
	}

	capacity := cap(slice)
	pool := sp.getPool(capacity)
	pool.Put(slice[:0])
}

// roundUpPowerOf2 rounds up to the nearest power of 2
func roundUpPowerOf2(n int) int {
	if n <= 0 {
		return 1
	}

	// Use bit manipulation to find next power of 2
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	n++

	return n
}

// Common slice pools for frequently used types
var (
	StringSlicePool = NewSlicePool[string]()
	IntSlicePool    = NewSlicePool[int]()
	ByteSlicePool   = NewSlicePool[byte]()
	EventSlicePool  = NewSlicePool[interface{}]() // For events
)

// EventHandlerSlicePool manages event handler slices
type EventHandlerSlicePool struct {
	pool sync.Pool
}

// NewEventHandlerSlicePool creates a pool for event handler slices
func NewEventHandlerSlicePool() *EventHandlerSlicePool {
	return &EventHandlerSlicePool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]interface{}, 0, 8)
			},
		},
	}
}

// Get returns an event handler slice
func (p *EventHandlerSlicePool) Get() []interface{} {
	return p.pool.Get().([]interface{})
}

// Put returns a slice to the pool
func (p *EventHandlerSlicePool) Put(slice []interface{}) {
	if slice == nil {
		return
	}

	// Clear references to avoid memory leaks
	for i := range slice {
		slice[i] = nil
	}

	p.pool.Put(slice[:0])
}

// PreAllocatedSlices provides pre-allocated slices for common use cases
type PreAllocatedSlices struct {
	// Event handler slices
	smallHandlers  [4]interface{}
	mediumHandlers [16]interface{}
	largeHandlers  [64]interface{}

	// String slices for event types
	eventTypes [8]string

	// Field slices for logging
	logFields [16]Field
}

// GetHandlerSlice returns a handler slice with appropriate capacity
func (p *PreAllocatedSlices) GetHandlerSlice(size int) []interface{} {
	switch {
	case size <= 4:
		return p.smallHandlers[:0]
	case size <= 16:
		return p.mediumHandlers[:0]
	case size <= 64:
		return p.largeHandlers[:0]
	default:
		return make([]interface{}, 0, size)
	}
}

// GetEventTypeSlice returns an event type slice
func (p *PreAllocatedSlices) GetEventTypeSlice() []string {
	return p.eventTypes[:0]
}

// GetLogFieldSlice returns a log field slice
func (p *PreAllocatedSlices) GetLogFieldSlice() []Field {
	return p.logFields[:0]
}

// Zero-allocation slice operations
type SliceOps struct{}

// AppendString appends a string to a slice without allocation if capacity allows
func (SliceOps) AppendString(slice []string, s string) []string {
	if len(slice) < cap(slice) {
		slice = slice[:len(slice)+1]
		slice[len(slice)-1] = s
		return slice
	}
	return append(slice, s)
}

// AppendInt appends an int to a slice without allocation if capacity allows
func (SliceOps) AppendInt(slice []int, i int) []int {
	if len(slice) < cap(slice) {
		slice = slice[:len(slice)+1]
		slice[len(slice)-1] = i
		return slice
	}
	return append(slice, i)
}

// RemoveString removes a string from a slice without allocation
func (SliceOps) RemoveString(slice []string, index int) []string {
	if index < 0 || index >= len(slice) {
		return slice
	}

	// Move elements to fill the gap
	copy(slice[index:], slice[index+1:])

	// Clear the last element to avoid memory leak
	slice[len(slice)-1] = ""

	return slice[:len(slice)-1]
}

// FastSliceGrow grows a slice to the target capacity efficiently
func FastSliceGrow[T any](slice []T, targetCap int) []T {
	if cap(slice) >= targetCap {
		return slice
	}

	// Allocate new slice with target capacity
	newSlice := make([]T, len(slice), targetCap)
	copy(newSlice, slice)
	return newSlice
}

// SliceStats provides statistics about slice usage
type SliceStats struct {
	TotalAllocations   int64
	TotalDeallocations int64
	PeakUsage          int64
	CurrentUsage       int64
	mutex              sync.RWMutex
}

// RecordAllocation records a slice allocation
func (s *SliceStats) RecordAllocation(size int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.TotalAllocations++
	s.CurrentUsage += int64(size)
	if s.CurrentUsage > s.PeakUsage {
		s.PeakUsage = s.CurrentUsage
	}
}

// RecordDeallocation records a slice deallocation
func (s *SliceStats) RecordDeallocation(size int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.TotalDeallocations++
	s.CurrentUsage -= int64(size)
}

// GetStats returns current statistics
func (s *SliceStats) GetStats() (total, peak, current int64) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.TotalAllocations, s.PeakUsage, s.CurrentUsage
}

// UnsafeSliceOps provides unsafe operations for maximum performance
type UnsafeSliceOps struct{}

// BytesToString converts bytes to string without allocation
func (UnsafeSliceOps) BytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// StringToBytes converts string to bytes without allocation
func (UnsafeSliceOps) StringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&s))
}

// Note: These unsafe operations should only be used when you're certain
// the resulting slice/string won't be modified and has the same lifetime
// as the original data.
