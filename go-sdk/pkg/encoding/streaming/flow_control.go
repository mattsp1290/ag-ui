package streaming

import (
	"sync"
	"sync/atomic"
	"time"
)

// FlowControlConfig holds configuration for flow control
type FlowControlConfig struct {
	// MaxPendingWrites is the maximum number of pending writes
	MaxPendingWrites int

	// MaxPendingReads is the maximum number of pending reads
	MaxPendingReads int

	// RateLimit is the maximum events per second (0 for unlimited)
	RateLimit int

	// BurstSize is the burst capacity for rate limiting
	BurstSize int

	// BackpressureThreshold triggers backpressure
	BackpressureThreshold float64 // 0.0 to 1.0

	// ThrottleDuration is how long to throttle when backpressure is detected
	ThrottleDuration time.Duration
}

// DefaultFlowControlConfig returns default flow control configuration
func DefaultFlowControlConfig() *FlowControlConfig {
	return &FlowControlConfig{
		MaxPendingWrites:      1000,
		MaxPendingReads:       1000,
		RateLimit:             10000, // 10k events/sec
		BurstSize:             100,
		BackpressureThreshold: 0.8, // 80% full
		ThrottleDuration:      10 * time.Millisecond,
	}
}

// FlowController manages flow control and backpressure
type FlowController struct {
	config *FlowControlConfig

	// Counters
	pendingWrites atomic.Int64
	pendingReads  atomic.Int64
	totalWrites   atomic.Int64
	totalReads    atomic.Int64

	// Rate limiting
	rateLimiter *RateLimiter

	// Backpressure
	backpressureActive atomic.Bool
	lastThrottle       atomic.Int64 // Unix nano timestamp

	// Buffer management
	writeBuffer *CircularBuffer
	readBuffer  *CircularBuffer

	// Metrics
	metrics FlowMetrics
	mu      sync.RWMutex
}

// FlowMetrics contains flow control metrics
type FlowMetrics struct {
	PendingWrites      int64
	PendingReads       int64
	TotalWrites        int64
	TotalReads         int64
	BackpressureEvents int64
	ThrottleEvents     int64
	DroppedEvents      int64
}

// RateLimiter implements token bucket algorithm
type RateLimiter struct {
	rate       int
	burst      int
	tokens     atomic.Int64
	lastRefill atomic.Int64
	mu         sync.Mutex
}

// CircularBuffer implements a lock-free circular buffer
type CircularBuffer struct {
	size     int
	mask     int
	buffer   []interface{}
	writePos atomic.Uint64
	readPos  atomic.Uint64
}

// NewFlowController creates a new flow controller
func NewFlowController(backpressureThreshold int) *FlowController {
	config := DefaultFlowControlConfig()
	if backpressureThreshold > 0 {
		config.MaxPendingWrites = backpressureThreshold
	}

	fc := &FlowController{
		config:      config,
		rateLimiter: NewRateLimiter(config.RateLimit, config.BurstSize),
		writeBuffer: NewCircularBuffer(config.MaxPendingWrites),
		readBuffer:  NewCircularBuffer(config.MaxPendingReads),
	}

	return fc
}

// RecordWrite records a write operation
func (fc *FlowController) RecordWrite() {
	fc.pendingWrites.Add(1)
	fc.totalWrites.Add(1)
	fc.checkBackpressure()
}

// RecordWriteComplete records completion of a write
func (fc *FlowController) RecordWriteComplete() {
	fc.pendingWrites.Add(-1)
}

// RecordRead records a read operation
func (fc *FlowController) RecordRead() {
	fc.pendingReads.Add(1)
	fc.totalReads.Add(1)
}

// RecordReadComplete records completion of a read
func (fc *FlowController) RecordReadComplete() {
	fc.pendingReads.Add(-1)
}

// ShouldThrottle checks if throttling is needed
func (fc *FlowController) ShouldThrottle() bool {
	// Check rate limit
	if !fc.rateLimiter.Allow() {
		fc.metrics.ThrottleEvents++
		return true
	}

	// Check backpressure
	if fc.backpressureActive.Load() {
		return true
	}

	// Check pending operations
	pending := fc.pendingWrites.Load()
	if float64(pending) > float64(fc.config.MaxPendingWrites)*fc.config.BackpressureThreshold {
		fc.activateBackpressure()
		return true
	}

	return false
}

// ApplyBackpressure applies backpressure by throttling
func (fc *FlowController) ApplyBackpressure() {
	time.Sleep(fc.config.ThrottleDuration)
	fc.lastThrottle.Store(time.Now().UnixNano())
}

// checkBackpressure checks and manages backpressure state
func (fc *FlowController) checkBackpressure() {
	pending := fc.pendingWrites.Load()
	threshold := float64(fc.config.MaxPendingWrites) * fc.config.BackpressureThreshold

	if float64(pending) >= threshold {
		fc.activateBackpressure()
	} else if float64(pending) < threshold*0.5 { // Hysteresis at 50%
		fc.deactivateBackpressure()
	}
}

// activateBackpressure activates backpressure
func (fc *FlowController) activateBackpressure() {
	if fc.backpressureActive.CompareAndSwap(false, true) {
		fc.metrics.BackpressureEvents++
	}
}

// deactivateBackpressure deactivates backpressure
func (fc *FlowController) deactivateBackpressure() {
	fc.backpressureActive.Store(false)
}

// GetMetrics returns current flow control metrics
func (fc *FlowController) GetMetrics() FlowMetrics {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	return FlowMetrics{
		PendingWrites:      fc.pendingWrites.Load(),
		PendingReads:       fc.pendingReads.Load(),
		TotalWrites:        fc.totalWrites.Load(),
		TotalReads:         fc.totalReads.Load(),
		BackpressureEvents: fc.metrics.BackpressureEvents,
		ThrottleEvents:     fc.metrics.ThrottleEvents,
		DroppedEvents:      fc.metrics.DroppedEvents,
	}
}

// IsBackpressureActive returns if backpressure is currently active
func (fc *FlowController) IsBackpressureActive() bool {
	return fc.backpressureActive.Load()
}

// GetPendingWrites returns the number of pending writes
func (fc *FlowController) GetPendingWrites() int64 {
	return fc.pendingWrites.Load()
}

// GetPendingReads returns the number of pending reads
func (fc *FlowController) GetPendingReads() int64 {
	return fc.pendingReads.Load()
}

// Reset resets the flow controller state
func (fc *FlowController) Reset() {
	fc.pendingWrites.Store(0)
	fc.pendingReads.Store(0)
	fc.totalWrites.Store(0)
	fc.totalReads.Store(0)
	fc.backpressureActive.Store(false)
	fc.lastThrottle.Store(0)
	fc.metrics = FlowMetrics{}
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate, burst int) *RateLimiter {
	rl := &RateLimiter{
		rate:  rate,
		burst: burst,
	}
	rl.tokens.Store(int64(burst))
	rl.lastRefill.Store(time.Now().UnixNano())
	return rl
}

// Allow checks if an operation is allowed
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens
	now := time.Now().UnixNano()
	lastRefill := rl.lastRefill.Load()
	elapsed := float64(now-lastRefill) / float64(time.Second)
	tokensToAdd := int64(elapsed * float64(rl.rate))

	if tokensToAdd > 0 {
		newTokens := rl.tokens.Load() + tokensToAdd
		if newTokens > int64(rl.burst) {
			newTokens = int64(rl.burst)
		}
		rl.tokens.Store(newTokens)
		rl.lastRefill.Store(now)
	}

	// Try to consume a token
	current := rl.tokens.Load()
	if current > 0 {
		rl.tokens.Store(current - 1)
		return true
	}

	return false
}

// NewCircularBuffer creates a new circular buffer
func NewCircularBuffer(size int) *CircularBuffer {
	// Ensure size is power of 2 for efficient masking
	actualSize := 1
	for actualSize < size {
		actualSize *= 2
	}

	return &CircularBuffer{
		size:   actualSize,
		mask:   actualSize - 1,
		buffer: make([]interface{}, actualSize),
	}
}

// Push adds an item to the buffer
func (cb *CircularBuffer) Push(item interface{}) bool {
	for {
		writePos := cb.writePos.Load()
		readPos := cb.readPos.Load()

		// Check if buffer is full
		if writePos-readPos >= uint64(cb.size) {
			return false // Buffer full
		}

		// Try to advance write position
		if cb.writePos.CompareAndSwap(writePos, writePos+1) {
			cb.buffer[writePos&uint64(cb.mask)] = item
			return true
		}
		// Retry if CAS failed
	}
}

// Pop removes an item from the buffer
func (cb *CircularBuffer) Pop() (interface{}, bool) {
	for {
		readPos := cb.readPos.Load()
		writePos := cb.writePos.Load()

		// Check if buffer is empty
		if readPos >= writePos {
			return nil, false
		}

		// Try to advance read position
		if cb.readPos.CompareAndSwap(readPos, readPos+1) {
			item := cb.buffer[readPos&uint64(cb.mask)]
			return item, true
		}
		// Retry if CAS failed
	}
}

// Size returns the current number of items in the buffer
func (cb *CircularBuffer) Size() int {
	writePos := cb.writePos.Load()
	readPos := cb.readPos.Load()
	return int(writePos - readPos)
}

// IsFull returns true if the buffer is full
func (cb *CircularBuffer) IsFull() bool {
	return cb.Size() >= cb.size
}

// IsEmpty returns true if the buffer is empty
func (cb *CircularBuffer) IsEmpty() bool {
	return cb.Size() == 0
}
