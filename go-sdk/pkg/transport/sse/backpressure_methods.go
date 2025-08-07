package sse

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// handleErrorWithBackpressure handles errors with proper backpressure control
func (t *SSETransport) handleErrorWithBackpressure(err error) {
	// Check if transport is closed
	if t.isClosed() {
		return
	}

	select {
	case t.errorChan <- err:
		// Successfully sent error
	case <-t.ctx.Done():
		// Transport is shutting down
		return
	default:
		// Error channel is full, apply backpressure
		t.handleDroppedError(err)
	}
}

// handleEventWithBackpressure handles events with proper backpressure control
func (t *SSETransport) handleEventWithBackpressure(event events.Event) {
	// Check if transport is closed
	if t.isClosed() {
		return
	}

	// Check if backpressure threshold is reached
	currentUsage := float64(len(t.eventChan)) / float64(cap(t.eventChan)) * 100
	
	if int(currentUsage) >= t.backpressureConfig.BackpressureThresholdPercent {
		t.activateBackpressure()
	}
	
	select {
	case t.eventChan <- event:
		// Successfully sent event
	case <-t.ctx.Done():
		// Transport is shutting down
		return
	default:
		// Event channel is full, apply backpressure
		t.handleDroppedEvent(event)
	}
}

// handleDroppedError handles dropped errors due to channel backpressure
func (t *SSETransport) handleDroppedError(err error) {
	droppedCount := atomic.AddInt64(&t.droppedErrors, 1)
	
	// Protect time field with mutex
	t.backpressureMutex.Lock()
	t.lastDropTime = time.Now()
	t.backpressureMutex.Unlock()
	
	if t.backpressureConfig.EnableBackpressureLogging {
		log.Printf("SSE Transport: Dropped error due to backpressure (total dropped errors: %d): %v", droppedCount, err)
	}
	
	// Check if we need to take action
	if droppedCount >= int64(t.backpressureConfig.MaxDroppedEvents) {
		t.handleBackpressureAction("errors")
	}
}

// handleDroppedEvent handles dropped events due to channel backpressure
func (t *SSETransport) handleDroppedEvent(event events.Event) {
	droppedCount := atomic.AddInt64(&t.droppedEvents, 1)
	
	// Protect time field with mutex
	t.backpressureMutex.Lock()
	t.lastDropTime = time.Now()
	t.backpressureMutex.Unlock()
	
	if t.backpressureConfig.EnableBackpressureLogging {
		log.Printf("SSE Transport: Dropped event due to backpressure (total dropped events: %d): %s", droppedCount, event.Type())
	}
	
	// Check if we need to take action
	if droppedCount >= int64(t.backpressureConfig.MaxDroppedEvents) {
		t.handleBackpressureAction("events")
	}
}

// activateBackpressure activates backpressure mode
func (t *SSETransport) activateBackpressure() {
	t.backpressureMutex.Lock()
	defer t.backpressureMutex.Unlock()
	
	if !t.backpressureActive {
		t.backpressureActive = true
		if t.backpressureConfig.EnableBackpressureLogging {
			log.Printf("SSE Transport: Backpressure activated - channel usage exceeded %d%%", t.backpressureConfig.BackpressureThresholdPercent)
		}
	}
}

// deactivateBackpressure deactivates backpressure mode
func (t *SSETransport) deactivateBackpressure() {
	t.backpressureMutex.Lock()
	defer t.backpressureMutex.Unlock()
	
	if t.backpressureActive {
		t.backpressureActive = false
		if t.backpressureConfig.EnableBackpressureLogging {
			log.Printf("SSE Transport: Backpressure deactivated")
		}
	}
}

// handleBackpressureAction takes action when maximum dropped items is reached
func (t *SSETransport) handleBackpressureAction(itemType string) {
	switch t.backpressureConfig.DropActionType {
	case DropActionLog:
		log.Printf("SSE Transport: Maximum dropped %s reached (%d), continuing with logging only", itemType, t.backpressureConfig.MaxDroppedEvents)
		
	case DropActionReconnect:
		log.Printf("SSE Transport: Maximum dropped %s reached (%d), attempting reconnection", itemType, t.backpressureConfig.MaxDroppedEvents)
		if err := t.reconnect(); err != nil {
			log.Printf("SSE Transport: Reconnection failed: %v", err)
		} else {
			// Reset counters on successful reconnection
			atomic.StoreInt64(&t.droppedEvents, 0)
			atomic.StoreInt64(&t.droppedErrors, 0)
		}
		
	case DropActionStop:
		log.Printf("SSE Transport: Maximum dropped %s reached (%d), stopping transport", itemType, t.backpressureConfig.MaxDroppedEvents)
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := t.Close(closeCtx); err != nil {
			log.Printf("SSE Transport: Error during shutdown: %v", err)
		}
		cancel()
	}
}

// GetBackpressureStats returns current backpressure statistics
func (t *SSETransport) GetBackpressureStats() BackpressureStats {
	t.backpressureMutex.RLock()
	defer t.backpressureMutex.RUnlock()
	
	eventChannelUsage := float64(len(t.eventChan)) / float64(cap(t.eventChan)) * 100
	errorChannelUsage := float64(len(t.errorChan)) / float64(cap(t.errorChan)) * 100
	
	return BackpressureStats{
		DroppedEvents:        atomic.LoadInt64(&t.droppedEvents),
		DroppedErrors:        atomic.LoadInt64(&t.droppedErrors),
		BackpressureActive:   t.backpressureActive,
		EventChannelUsage:    eventChannelUsage,
		ErrorChannelUsage:    errorChannelUsage,
		LastDropTime:         t.lastDropTime,
		EventChannelCapacity: cap(t.eventChan),
		ErrorChannelCapacity: cap(t.errorChan),
	}
}

// BackpressureStats contains backpressure monitoring statistics
type BackpressureStats struct {
	DroppedEvents        int64     `json:"dropped_events"`
	DroppedErrors        int64     `json:"dropped_errors"`
	BackpressureActive   bool      `json:"backpressure_active"`
	EventChannelUsage    float64   `json:"event_channel_usage_percent"`
	ErrorChannelUsage    float64   `json:"error_channel_usage_percent"`
	LastDropTime         time.Time `json:"last_drop_time"`
	EventChannelCapacity int       `json:"event_channel_capacity"`
	ErrorChannelCapacity int       `json:"error_channel_capacity"`
}

// ResetBackpressureStats resets backpressure statistics
func (t *SSETransport) ResetBackpressureStats() {
	atomic.StoreInt64(&t.droppedEvents, 0)
	atomic.StoreInt64(&t.droppedErrors, 0)
	t.deactivateBackpressure()
}