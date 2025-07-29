package memory

import (
	"context"
	"sync"
	"time"
	
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// BackpressureStrategy defines how backpressure should be handled
type BackpressureStrategy string

const (
	// BackpressureNone disables backpressure handling (default behavior)
	BackpressureNone BackpressureStrategy = "none"
	
	// BackpressureDropOldest drops the oldest events when buffer is full
	BackpressureDropOldest BackpressureStrategy = "drop_oldest"
	
	// BackpressureDropNewest drops the newest events when buffer is full
	BackpressureDropNewest BackpressureStrategy = "drop_newest"
	
	// BackpressureBlock blocks the producer when buffer is full
	BackpressureBlock BackpressureStrategy = "block"
	
	// BackpressureBlockWithTimeout blocks the producer with a timeout
	BackpressureBlockWithTimeout BackpressureStrategy = "block_timeout"
)

// BackpressureConfig configures backpressure handling
type BackpressureConfig struct {
	// Strategy defines the backpressure strategy to use
	Strategy BackpressureStrategy `yaml:"strategy" json:"strategy" default:"none"`
	
	// BufferSize is the size of the event buffer
	BufferSize int `yaml:"buffer_size" json:"buffer_size" default:"1024"`
	
	// HighWaterMark is the percentage of buffer fullness that triggers backpressure
	HighWaterMark float64 `yaml:"high_water_mark" json:"high_water_mark" default:"0.8"`
	
	// LowWaterMark is the percentage of buffer fullness that releases backpressure
	LowWaterMark float64 `yaml:"low_water_mark" json:"low_water_mark" default:"0.2"`
	
	// BlockTimeout is the maximum time to block when using block_timeout strategy
	BlockTimeout time.Duration `yaml:"block_timeout" json:"block_timeout" default:"5s"`
	
	// EnableMetrics enables backpressure metrics collection
	EnableMetrics bool `yaml:"enable_metrics" json:"enable_metrics" default:"true"`
}

// BackpressureMetrics contains metrics for backpressure handling
type BackpressureMetrics struct {
	mu                    sync.RWMutex
	EventsDropped         uint64    `json:"events_dropped"`
	EventsBlocked         uint64    `json:"events_blocked"`
	BlockedDuration       time.Duration `json:"blocked_duration"`
	CurrentBufferSize     int       `json:"current_buffer_size"`
	MaxBufferSize         int       `json:"max_buffer_size"`
	HighWaterMarkHits     uint64    `json:"high_water_mark_hits"`
	LowWaterMarkHits      uint64    `json:"low_water_mark_hits"`
	LastDropTime          time.Time `json:"last_drop_time"`
	LastBlockTime         time.Time `json:"last_block_time"`
	BackpressureActive    bool      `json:"backpressure_active"`
}

// BackpressureHandler manages backpressure for event channels
type BackpressureHandler struct {
	mu              sync.RWMutex
	config          BackpressureConfig
	metrics         *BackpressureMetrics
	eventChan       chan events.Event
	errorChan       chan error
	backpressureOn  bool
	stopChan        chan struct{}
	ctx             context.Context
	cancel          context.CancelFunc
	stopped         bool
}

// NewBackpressureHandler creates a new backpressure handler
func NewBackpressureHandler(config BackpressureConfig) *BackpressureHandler {
	ctx, cancel := context.WithCancel(context.Background())
	
	handler := &BackpressureHandler{
		config:     config,
		metrics:    &BackpressureMetrics{MaxBufferSize: config.BufferSize},
		eventChan:  make(chan events.Event, config.BufferSize),
		errorChan:  make(chan error, config.BufferSize),
		stopChan:   make(chan struct{}),
		ctx:        ctx,
		cancel:     cancel,
	}
	
	// Start the backpressure monitor if needed
	if config.Strategy != BackpressureNone {
		go handler.monitorBackpressure()
	}
	
	return handler
}

// SendEvent sends an event through the backpressure handler
func (h *BackpressureHandler) SendEvent(event events.Event) error {
	// Check if stopped
	h.mu.RLock()
	if h.stopped {
		h.mu.RUnlock()
		return ErrConnectionClosed
	}
	h.mu.RUnlock()
	
	if h.config.EnableMetrics {
		h.metrics.mu.Lock()
		h.metrics.CurrentBufferSize = len(h.eventChan)
		h.metrics.mu.Unlock()
	}
	
	// For non-blocking strategies, we can hold the lock
	// For blocking strategies, we need to release it
	switch h.config.Strategy {
	case BackpressureNone:
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.sendEventNone(event)
	case BackpressureDropOldest:
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.sendEventDropOldest(event)
	case BackpressureDropNewest:
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.sendEventDropNewest(event)
	case BackpressureBlock:
		// Don't hold lock for blocking operations
		return h.sendEventBlock(event)
	case BackpressureBlockWithTimeout:
		// Don't hold lock for blocking operations
		return h.sendEventBlockTimeout(event)
	default:
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.sendEventNone(event)
	}
}

// SendError sends an error through the backpressure handler
func (h *BackpressureHandler) SendError(err error) error {
	// Errors use the same backpressure strategy as events
	select {
	case h.errorChan <- err:
		return nil
	default:
		if h.config.Strategy == BackpressureDropOldest || h.config.Strategy == BackpressureDropNewest {
			// For drop strategies, just drop the error
			if h.config.EnableMetrics {
				h.metrics.mu.Lock()
				h.metrics.EventsDropped++
				h.metrics.LastDropTime = time.Now()
				h.metrics.mu.Unlock()
			}
			return nil
		}
		return ErrBackpressureActive
	}
}

// EventChan returns the event channel
// Deprecated: Use Channels() instead for consistency
func (h *BackpressureHandler) EventChan() <-chan events.Event {
	return h.eventChan
}

// ErrorChan returns the error channel
// Deprecated: Use Channels() instead for consistency
func (h *BackpressureHandler) ErrorChan() <-chan error {
	return h.errorChan
}

// Channels returns both event and error channels together
func (h *BackpressureHandler) Channels() (<-chan events.Event, <-chan error) {
	return h.eventChan, h.errorChan
}

// GetMetrics returns the current backpressure metrics
func (h *BackpressureHandler) GetMetrics() BackpressureMetrics {
	h.metrics.mu.RLock()
	defer h.metrics.mu.RUnlock()
	return *h.metrics
}

// Stop stops the backpressure handler
func (h *BackpressureHandler) Stop() {
	// Cancel context first to signal all goroutines to stop
	h.cancel()
	
	// Close stopChan to signal monitor goroutine
	h.mu.Lock()
	if !h.stopped {
		h.stopped = true
		select {
		case <-h.stopChan:
			// Already closed
		default:
			close(h.stopChan)
		}
	}
	h.mu.Unlock()
	
	// Give monitor goroutine time to exit
	time.Sleep(10 * time.Millisecond)
}

// sendEventNone sends event with no backpressure handling (original behavior)
func (h *BackpressureHandler) sendEventNone(event events.Event) error {
	select {
	case h.eventChan <- event:
		return nil
	default:
		return ErrBackpressureActive
	}
}

// sendEventDropOldest sends event and drops oldest if buffer is full
func (h *BackpressureHandler) sendEventDropOldest(event events.Event) error {
	select {
	case h.eventChan <- event:
		return nil
	default:
		// Channel is full, try to drop oldest
		select {
		case <-h.eventChan: // Remove oldest event
			h.eventChan <- event // Add new event
			if h.config.EnableMetrics {
				h.metrics.mu.Lock()
				h.metrics.EventsDropped++
				h.metrics.LastDropTime = time.Now()
				h.metrics.mu.Unlock()
			}
			return nil
		default:
			return ErrBackpressureActive
		}
	}
}

// sendEventDropNewest sends event and drops newest if buffer is full
func (h *BackpressureHandler) sendEventDropNewest(event events.Event) error {
	select {
	case h.eventChan <- event:
		return nil
	default:
		// Channel is full, drop the new event
		if h.config.EnableMetrics {
			h.metrics.mu.Lock()
			h.metrics.EventsDropped++
			h.metrics.LastDropTime = time.Now()
			h.metrics.mu.Unlock()
		}
		return nil // Don't return error for drop_newest
	}
}

// sendEventBlock sends event and blocks if buffer is full
func (h *BackpressureHandler) sendEventBlock(event events.Event) error {
	startTime := time.Now()
	
	select {
	case h.eventChan <- event:
		return nil
	case <-h.ctx.Done():
		return ErrBackpressureActive
	default:
		// Channel is full, block
		if h.config.EnableMetrics {
			h.metrics.mu.Lock()
			h.metrics.EventsBlocked++
			h.metrics.LastBlockTime = time.Now()
			h.metrics.mu.Unlock()
		}
		
		select {
		case h.eventChan <- event:
			if h.config.EnableMetrics {
				h.metrics.mu.Lock()
				h.metrics.BlockedDuration += time.Since(startTime)
				h.metrics.mu.Unlock()
			}
			return nil
		case <-h.ctx.Done():
			return ErrBackpressureActive
		}
	}
}

// sendEventBlockTimeout sends event and blocks with timeout if buffer is full
func (h *BackpressureHandler) sendEventBlockTimeout(event events.Event) error {
	startTime := time.Now()
	
	select {
	case h.eventChan <- event:
		return nil
	case <-h.ctx.Done():
		return ErrBackpressureActive
	default:
		// Channel is full, block with timeout
		if h.config.EnableMetrics {
			h.metrics.mu.Lock()
			h.metrics.EventsBlocked++
			h.metrics.LastBlockTime = time.Now()
			h.metrics.mu.Unlock()
		}
		
		ctx, cancel := context.WithTimeout(h.ctx, h.config.BlockTimeout)
		defer cancel()
		
		select {
		case h.eventChan <- event:
			if h.config.EnableMetrics {
				h.metrics.mu.Lock()
				h.metrics.BlockedDuration += time.Since(startTime)
				h.metrics.mu.Unlock()
			}
			return nil
		case <-ctx.Done():
			return ErrBackpressureTimeout
		case <-h.ctx.Done():
			return ErrBackpressureActive
		}
	}
}

// monitorBackpressure monitors backpressure conditions and updates metrics
func (h *BackpressureHandler) monitorBackpressure() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			h.checkBackpressureConditions()
		case <-h.stopChan:
			return
		case <-h.ctx.Done():
			return
		}
	}
}

// checkBackpressureConditions checks if backpressure conditions are met
func (h *BackpressureHandler) checkBackpressureConditions() {
	// Quick check if we're stopped
	select {
	case <-h.ctx.Done():
		return
	default:
	}
	
	// Get channel info without holding main lock
	currentSize := len(h.eventChan)
	maxSize := cap(h.eventChan)
	
	if !h.config.EnableMetrics {
		return
	}
	
	fillPercentage := float64(currentSize) / float64(maxSize)
	
	// Update metrics
	h.metrics.mu.Lock()
	h.metrics.CurrentBufferSize = currentSize
	
	// Check high water mark
	if fillPercentage >= h.config.HighWaterMark && !h.metrics.BackpressureActive {
		h.metrics.BackpressureActive = true
		h.metrics.HighWaterMarkHits++
	}
	
	// Check low water mark
	if fillPercentage <= h.config.LowWaterMark && h.metrics.BackpressureActive {
		h.metrics.BackpressureActive = false
		h.metrics.LowWaterMarkHits++
	}
	h.metrics.mu.Unlock()
}