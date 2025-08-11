package sse

import (
	"sync"
	"time"
)

// EventRateWindow tracks events in a rolling time window for rate calculation
type EventRateWindow struct {
	windowSize time.Duration
	events     []time.Time
	mu         sync.RWMutex
}

// NewEventRateWindow creates a new rolling window for event rate tracking
func NewEventRateWindow(windowSize time.Duration) *EventRateWindow {
	return &EventRateWindow{
		windowSize: windowSize,
		events:     make([]time.Time, 0, 1000), // Pre-allocate for efficiency
	}
}

// Record adds a new event timestamp to the window
func (w *EventRateWindow) Record(timestamp time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Add the new event
	w.events = append(w.events, timestamp)

	// Clean up old events outside the window
	w.cleanupOldEvents(timestamp)
}

// Rate returns the current events per second rate
func (w *EventRateWindow) Rate() float64 {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if len(w.events) == 0 {
		return 0
	}

	now := time.Now()
	// Clean up in read operation too (helps with accuracy)
	validEvents := w.countValidEvents(now)
	
	if validEvents == 0 {
		return 0
	}

	// Calculate the actual time span of events in the window
	oldestEvent := w.findOldestValidEvent(now)
	if oldestEvent.IsZero() {
		return 0
	}

	timeSpan := now.Sub(oldestEvent).Seconds()
	if timeSpan <= 0 {
		return 0
	}

	return float64(validEvents) / timeSpan
}

// Count returns the number of events in the current window
func (w *EventRateWindow) Count() int {
	w.mu.RLock()
	defer w.mu.RUnlock()

	now := time.Now()
	return w.countValidEvents(now)
}

// Reset clears all events from the window
func (w *EventRateWindow) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.events = w.events[:0] // Clear but keep capacity
}

// cleanupOldEvents removes events outside the window (must be called with lock held)
func (w *EventRateWindow) cleanupOldEvents(now time.Time) {
	cutoff := now.Add(-w.windowSize)
	
	// Find the first event that's within the window
	keepFrom := 0
	for i, event := range w.events {
		if event.After(cutoff) {
			keepFrom = i
			break
		}
	}

	// If we need to remove events, shift the slice
	if keepFrom > 0 {
		copy(w.events, w.events[keepFrom:])
		w.events = w.events[:len(w.events)-keepFrom]
	}

	// If the slice has grown too large, reallocate to save memory
	if cap(w.events) > 10000 && len(w.events) < cap(w.events)/4 {
		newEvents := make([]time.Time, len(w.events))
		copy(newEvents, w.events)
		w.events = newEvents
	}
}

// countValidEvents counts events within the window (must be called with lock held)
func (w *EventRateWindow) countValidEvents(now time.Time) int {
	cutoff := now.Add(-w.windowSize)
	count := 0
	
	for _, event := range w.events {
		if event.After(cutoff) {
			count++
		}
	}
	
	return count
}

// findOldestValidEvent finds the oldest event within the window (must be called with lock held)
func (w *EventRateWindow) findOldestValidEvent(now time.Time) time.Time {
	cutoff := now.Add(-w.windowSize)
	
	for _, event := range w.events {
		if event.After(cutoff) {
			return event
		}
	}
	
	return time.Time{}
}

// GetWindowSize returns the size of the rolling window
func (w *EventRateWindow) GetWindowSize() time.Duration {
	return w.windowSize
}

// SetWindowSize updates the window size (useful for dynamic adjustment)
func (w *EventRateWindow) SetWindowSize(size time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	w.windowSize = size
	// Immediately clean up based on new window size
	w.cleanupOldEvents(time.Now())
}