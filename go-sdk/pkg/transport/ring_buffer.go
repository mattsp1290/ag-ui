package transport

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// RingBuffer is a thread-safe circular buffer with configurable overflow policies
type RingBuffer struct {
	mu              sync.RWMutex
	buffer          []events.Event
	capacity        int
	head            int
	tail            int
	size            int
	overflowPolicy  OverflowPolicy
	
	// Statistics - cache line padded to prevent false sharing
	totalWritten    atomic.Uint64
	_               [56]byte // Cache line padding
	totalRead       atomic.Uint64
	_               [56]byte // Cache line padding
	totalDropped    atomic.Uint64
	_               [56]byte // Cache line padding
	totalOverflows  atomic.Uint64
	_               [56]byte // Cache line padding
	
	// Condition variables for blocking operations
	notEmpty        *sync.Cond
	notFull         *sync.Cond
	
	// Context for cancellation
	ctx             context.Context
	cancel          context.CancelFunc
	
	// Metrics
	metrics         *RingBufferMetrics
}

// OverflowPolicy defines how to handle buffer overflow
type OverflowPolicy int

const (
	// OverflowDropOldest drops the oldest item when buffer is full
	OverflowDropOldest OverflowPolicy = iota
	// OverflowDropNewest drops the newest item when buffer is full
	OverflowDropNewest
	// OverflowBlock blocks until space is available
	OverflowBlock
	// OverflowResize dynamically resizes the buffer (up to a limit)
	OverflowResize
)

// RingBufferConfig configures the ring buffer
type RingBufferConfig struct {
	Capacity       int
	OverflowPolicy OverflowPolicy
	MaxCapacity    int             // For OverflowResize policy
	ResizeFactor   float64         // Multiplier for resize (default 1.5)
	BlockTimeout   time.Duration   // Timeout for blocking operations
}

// RingBufferMetrics tracks ring buffer statistics
type RingBufferMetrics struct {
	mu               sync.RWMutex
	CurrentSize      int
	Capacity         int
	TotalWrites      uint64
	TotalReads       uint64
	TotalDrops       uint64
	TotalOverflows   uint64
	ResizeCount      uint64
	LastWriteTime    time.Time
	LastReadTime     time.Time
	LastDropTime     time.Time
	LastResizeTime   time.Time
	AverageWriteTime time.Duration
	AverageReadTime  time.Duration
}

// DefaultRingBufferConfig returns default configuration
func DefaultRingBufferConfig() *RingBufferConfig {
	return &RingBufferConfig{
		Capacity:       1024,
		OverflowPolicy: OverflowDropOldest,
		MaxCapacity:    8192,
		ResizeFactor:   1.5,
		BlockTimeout:   5 * time.Second,
	}
}

// NewRingBuffer creates a new ring buffer
func NewRingBuffer(config *RingBufferConfig) *RingBuffer {
	if config == nil {
		config = DefaultRingBufferConfig()
	}

	if config.Capacity <= 0 {
		config.Capacity = 1024
	}

	ctx, cancel := context.WithCancel(context.Background())

	rb := &RingBuffer{
		buffer:         make([]events.Event, config.Capacity),
		capacity:       config.Capacity,
		overflowPolicy: config.OverflowPolicy,
		ctx:            ctx,
		cancel:         cancel,
		metrics: &RingBufferMetrics{
			Capacity: config.Capacity,
		},
	}

	rb.notEmpty = sync.NewCond(&rb.mu)
	rb.notFull = sync.NewCond(&rb.mu)

	return rb
}

// Push adds an event to the buffer
func (rb *RingBuffer) Push(event events.Event) error {
	return rb.PushWithContext(rb.ctx, event)
}

// PushWithContext adds an event to the buffer with context
func (rb *RingBuffer) PushWithContext(ctx context.Context, event events.Event) error {
	if event == nil {
		return errors.New("cannot push nil event")
	}

	start := time.Now()
	defer func() {
		rb.updateWriteMetrics(time.Since(start))
	}()

	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Check if buffer is full
	if rb.size == rb.capacity {
		return rb.handleOverflow(ctx, event)
	}

	// Add event to buffer
	rb.buffer[rb.tail] = event
	rb.tail = (rb.tail + 1) % rb.capacity
	rb.size++
	
	rb.totalWritten.Add(1)
	rb.notEmpty.Signal()

	return nil
}

// Pop removes and returns the oldest event from the buffer
func (rb *RingBuffer) Pop() (events.Event, error) {
	return rb.PopWithContext(rb.ctx)
}

// PopWithContext removes and returns the oldest event with context
func (rb *RingBuffer) PopWithContext(ctx context.Context) (events.Event, error) {
	start := time.Now()
	defer func() {
		rb.updateReadMetrics(time.Since(start))
	}()

	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Wait for data if buffer is empty
	for rb.size == 0 {
		// Check context cancellation before waiting
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		// Use a channel to coordinate between Wait() and context cancellation
		waitDone := make(chan struct{})
		ctxCancelled := make(chan struct{}, 1) // Buffered channel for safe communication
		
		// Start goroutine to handle context cancellation
		go func() {
			select {
			case <-ctx.Done():
				select {
				case ctxCancelled <- struct{}{}:
				default:
				}
				rb.notEmpty.Broadcast() // Wake up the Wait()
			case <-waitDone:
				// Wait completed normally
			}
		}()
		
		// Wait on the condition variable
		rb.notEmpty.Wait()
		close(waitDone) // Signal that Wait() completed
		
		// Check if we should exit due to context cancellation
		select {
		case <-ctxCancelled:
			return nil, ctx.Err()
		default:
		}
	}

	// Get event from buffer
	event := rb.buffer[rb.head]
	rb.buffer[rb.head] = nil // Clear reference to help GC
	rb.head = (rb.head + 1) % rb.capacity
	rb.size--
	
	rb.totalRead.Add(1)
	rb.notFull.Signal()

	return event, nil
}

// TryPop attempts to remove and return the oldest event without blocking
func (rb *RingBuffer) TryPop() (events.Event, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return nil, false
	}

	event := rb.buffer[rb.head]
	rb.buffer[rb.head] = nil
	rb.head = (rb.head + 1) % rb.capacity
	rb.size--
	
	rb.totalRead.Add(1)
	rb.notFull.Signal()

	rb.updateReadMetrics(0)

	return event, true
}

// Size returns the current number of events in the buffer
func (rb *RingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size
}

// Capacity returns the current capacity of the buffer
func (rb *RingBuffer) Capacity() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.capacity
}

// IsFull returns true if the buffer is full
func (rb *RingBuffer) IsFull() bool {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size == rb.capacity
}

// IsEmpty returns true if the buffer is empty
func (rb *RingBuffer) IsEmpty() bool {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size == 0
}

// Clear removes all events from the buffer
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Clear all references to help GC
	for i := 0; i < rb.capacity; i++ {
		rb.buffer[i] = nil
	}

	rb.head = 0
	rb.tail = 0
	rb.size = 0
	
	rb.notFull.Broadcast()
}

// GetMetrics returns current metrics
func (rb *RingBuffer) GetMetrics() RingBufferMetrics {
	rb.metrics.mu.RLock()
	defer rb.metrics.mu.RUnlock()
	
	return RingBufferMetrics{
		CurrentSize:      rb.Size(),
		Capacity:         rb.metrics.Capacity,
		TotalWrites:      rb.totalWritten.Load(),
		TotalReads:       rb.totalRead.Load(),
		TotalDrops:       rb.totalDropped.Load(),
		TotalOverflows:   rb.totalOverflows.Load(),
		ResizeCount:      rb.metrics.ResizeCount,
		LastWriteTime:    rb.metrics.LastWriteTime,
		LastReadTime:     rb.metrics.LastReadTime,
		LastDropTime:     rb.metrics.LastDropTime,
		LastResizeTime:   rb.metrics.LastResizeTime,
		AverageWriteTime: rb.metrics.AverageWriteTime,
		AverageReadTime:  rb.metrics.AverageReadTime,
	}
}

// Close closes the ring buffer
func (rb *RingBuffer) Close() {
	rb.cancel()
	rb.mu.Lock()
	defer rb.mu.Unlock()
	
	// Wake up any waiting goroutines
	rb.notEmpty.Broadcast()
	rb.notFull.Broadcast()
	
	// Clear buffer
	for i := 0; i < rb.capacity; i++ {
		rb.buffer[i] = nil
	}
}

// handleOverflow handles buffer overflow based on policy
func (rb *RingBuffer) handleOverflow(ctx context.Context, event events.Event) error {
	rb.totalOverflows.Add(1)

	switch rb.overflowPolicy {
	case OverflowDropOldest:
		// Drop oldest event
		rb.buffer[rb.head] = nil
		rb.head = (rb.head + 1) % rb.capacity
		
		// Add new event
		rb.buffer[rb.tail] = event
		rb.tail = (rb.tail + 1) % rb.capacity
		
		rb.totalDropped.Add(1)
		rb.totalWritten.Add(1)
		rb.updateDropMetrics()
		return nil

	case OverflowDropNewest:
		// Drop the new event
		rb.totalDropped.Add(1)
		rb.updateDropMetrics()
		return nil

	case OverflowBlock:
		// Wait for space
		for rb.size == rb.capacity {
			// Check context cancellation before waiting
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			
			// Use a channel to coordinate between Wait() and context cancellation
			waitDone := make(chan struct{})
			ctxCancelled := make(chan struct{}, 1) // Buffered channel for safe communication
			
			// Start goroutine to handle context cancellation
			go func() {
				select {
				case <-ctx.Done():
					select {
					case ctxCancelled <- struct{}{}:
					default:
					}
					rb.notFull.Broadcast() // Wake up the Wait()
				case <-waitDone:
					// Wait completed normally
				}
			}()
			
			// Wait on the condition variable
			rb.notFull.Wait()
			close(waitDone) // Signal that Wait() completed
			
			// Check if we should exit due to context cancellation
			select {
			case <-ctxCancelled:
				return ctx.Err()
			default:
			}
		}
		
		// Add event
		rb.buffer[rb.tail] = event
		rb.tail = (rb.tail + 1) % rb.capacity
		rb.size++
		rb.totalWritten.Add(1)
		rb.notEmpty.Signal()
		return nil

	case OverflowResize:
		// Resize buffer if possible
		if rb.resize() {
			// Add event after resize
			rb.buffer[rb.tail] = event
			rb.tail = (rb.tail + 1) % rb.capacity
			rb.size++
			rb.totalWritten.Add(1)
			rb.notEmpty.Signal()
			return nil
		}
		// If resize failed, drop oldest
		return rb.handleOverflow(ctx, event)

	default:
		return errors.New("unknown overflow policy")
	}
}

// resize attempts to resize the buffer
func (rb *RingBuffer) resize() bool {
	// This would need additional configuration for max capacity
	// For now, return false to indicate resize is not supported
	return false
}

// updateWriteMetrics updates write metrics
func (rb *RingBuffer) updateWriteMetrics(duration time.Duration) {
	rb.metrics.mu.Lock()
	defer rb.metrics.mu.Unlock()
	
	rb.metrics.LastWriteTime = time.Now()
	if rb.metrics.AverageWriteTime == 0 {
		rb.metrics.AverageWriteTime = duration
	} else {
		// Exponential moving average
		rb.metrics.AverageWriteTime = time.Duration(
			float64(rb.metrics.AverageWriteTime)*0.9 + float64(duration)*0.1,
		)
	}
}

// updateReadMetrics updates read metrics
func (rb *RingBuffer) updateReadMetrics(duration time.Duration) {
	rb.metrics.mu.Lock()
	defer rb.metrics.mu.Unlock()
	
	rb.metrics.LastReadTime = time.Now()
	if rb.metrics.AverageReadTime == 0 {
		rb.metrics.AverageReadTime = duration
	} else {
		// Exponential moving average
		rb.metrics.AverageReadTime = time.Duration(
			float64(rb.metrics.AverageReadTime)*0.9 + float64(duration)*0.1,
		)
	}
}

// updateDropMetrics updates drop metrics
func (rb *RingBuffer) updateDropMetrics() {
	rb.metrics.mu.Lock()
	defer rb.metrics.mu.Unlock()
	
	rb.metrics.LastDropTime = time.Now()
}

// Drain removes all events from the buffer and returns them
func (rb *RingBuffer) Drain() []events.Event {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return nil
	}

	events := make([]events.Event, rb.size)
	idx := 0

	for rb.size > 0 {
		events[idx] = rb.buffer[rb.head]
		rb.buffer[rb.head] = nil
		rb.head = (rb.head + 1) % rb.capacity
		rb.size--
		idx++
	}

	rb.totalRead.Add(uint64(idx))
	rb.notFull.Broadcast()

	return events
}