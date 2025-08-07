package streaming

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// StreamMetrics collects metrics for streaming operations
type StreamMetrics struct {
	// Event counts
	eventsProcessed atomic.Int64
	eventsDropped   atomic.Int64
	eventsErrored   atomic.Int64

	// Throughput
	bytesProcessed  atomic.Int64
	bytesPerSecond  atomic.Int64
	eventsPerSecond atomic.Int64

	// Latency tracking
	latencySum   atomic.Int64
	latencyCount atomic.Int64
	maxLatency   atomic.Int64

	// Memory usage
	memoryUsed     atomic.Int64
	peakMemoryUsed atomic.Int64
	bufferSize     atomic.Int64

	// Progress tracking
	startTime       time.Time
	lastUpdateTime  atomic.Int64
	progressPercent atomic.Uint32

	// Event type breakdown
	eventTypes   map[string]*EventTypeMetrics
	eventTypesMu sync.RWMutex

	// Sampling
	sampleInterval time.Duration
	sampleTicker   *time.Ticker
	stopChan       chan struct{}
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
}

// EventTypeMetrics tracks metrics for a specific event type
type EventTypeMetrics struct {
	Count   atomic.Int64
	Bytes   atomic.Int64
	AvgSize atomic.Int64
	Errors  atomic.Int64
}

// MetricsSnapshot represents a point-in-time snapshot of metrics
type MetricsSnapshot struct {
	Timestamp       time.Time
	EventsProcessed int64
	EventsDropped   int64
	EventsErrored   int64
	BytesProcessed  int64
	BytesPerSecond  int64
	EventsPerSecond int64
	AvgLatencyMs    float64
	MaxLatencyMs    int64
	MemoryUsedMB    float64
	PeakMemoryMB    float64
	BufferSize      int64
	ProgressPercent float32
	Duration        time.Duration
	EventTypes      map[string]EventTypeSnapshot
}

// EventTypeSnapshot represents metrics for a specific event type
type EventTypeSnapshot struct {
	Count   int64
	Bytes   int64
	AvgSize int64
	Errors  int64
}

// NewStreamMetrics creates a new metrics collector
func NewStreamMetrics() *StreamMetrics {
	return NewStreamMetricsWithContext(context.Background())
}

// NewStreamMetricsWithContext creates a new metrics collector with a parent context
func NewStreamMetricsWithContext(parentCtx context.Context) *StreamMetrics {
	ctx, cancel := context.WithCancel(parentCtx)
	sm := &StreamMetrics{
		startTime:      time.Now(),
		eventTypes:     make(map[string]*EventTypeMetrics),
		sampleInterval: time.Second,
		stopChan:       make(chan struct{}),
		ctx:            ctx,
		cancel:         cancel,
	}

	// Start background sampling
	sm.startSampling()

	return sm
}

// RecordEvent records metrics for a processed event
func (sm *StreamMetrics) RecordEvent(event events.Event) {
	sm.eventsProcessed.Add(1)

	// Record event type metrics
	eventType := string(event.Type())
	sm.recordEventType(eventType, event)

	// Update throughput
	// Note: Actual byte size calculation would depend on encoding
	estimatedSize := sm.estimateEventSize(event)
	sm.bytesProcessed.Add(estimatedSize)
}

// RecordError records an error processing an event
func (sm *StreamMetrics) RecordError(eventType string) {
	sm.eventsErrored.Add(1)

	sm.eventTypesMu.RLock()
	if metrics, exists := sm.eventTypes[eventType]; exists {
		metrics.Errors.Add(1)
	}
	sm.eventTypesMu.RUnlock()
}

// RecordDropped records a dropped event
func (sm *StreamMetrics) RecordDropped() {
	sm.eventsDropped.Add(1)
}

// RecordLatency records processing latency
func (sm *StreamMetrics) RecordLatency(latencyNs int64) {
	sm.latencySum.Add(latencyNs)
	sm.latencyCount.Add(1)

	// Update max latency
	for {
		current := sm.maxLatency.Load()
		if latencyNs <= current || sm.maxLatency.CompareAndSwap(current, latencyNs) {
			break
		}
	}
}

// UpdateMemoryUsage updates memory usage metrics
func (sm *StreamMetrics) UpdateMemoryUsage(used int64) {
	sm.memoryUsed.Store(used)

	// Update peak memory
	for {
		current := sm.peakMemoryUsed.Load()
		if used <= current || sm.peakMemoryUsed.CompareAndSwap(current, used) {
			break
		}
	}
}

// UpdateBufferSize updates buffer size metrics
func (sm *StreamMetrics) UpdateBufferSize(writeBuffer, readBuffer int) {
	sm.bufferSize.Store(int64(writeBuffer + readBuffer))
}

// UpdateProgress updates progress percentage
func (sm *StreamMetrics) UpdateProgress(processed, total int64) {
	if total > 0 {
		percent := uint32((float64(processed) / float64(total)) * 100)
		sm.progressPercent.Store(percent)
	}
}

// UpdateFlowControl updates flow control metrics
func (sm *StreamMetrics) UpdateFlowControl(flowMetrics FlowMetrics) {
	// This could be extended to track flow control specific metrics
	sm.lastUpdateTime.Store(time.Now().UnixNano())
}

// GetSnapshot returns a snapshot of current metrics
func (sm *StreamMetrics) GetSnapshot() MetricsSnapshot {
	now := time.Now()
	duration := now.Sub(sm.startTime)

	// Calculate averages
	latencyCount := sm.latencyCount.Load()
	avgLatencyNs := int64(0)
	if latencyCount > 0 {
		avgLatencyNs = sm.latencySum.Load() / latencyCount
	}

	// Get event type snapshots
	eventTypes := make(map[string]EventTypeSnapshot)
	sm.eventTypesMu.RLock()
	for name, metrics := range sm.eventTypes {
		count := metrics.Count.Load()
		bytes := metrics.Bytes.Load()
		avgSize := int64(0)
		if count > 0 {
			avgSize = bytes / count
		}
		eventTypes[name] = EventTypeSnapshot{
			Count:   count,
			Bytes:   bytes,
			AvgSize: avgSize,
			Errors:  metrics.Errors.Load(),
		}
	}
	sm.eventTypesMu.RUnlock()

	return MetricsSnapshot{
		Timestamp:       now,
		EventsProcessed: sm.eventsProcessed.Load(),
		EventsDropped:   sm.eventsDropped.Load(),
		EventsErrored:   sm.eventsErrored.Load(),
		BytesProcessed:  sm.bytesProcessed.Load(),
		BytesPerSecond:  sm.bytesPerSecond.Load(),
		EventsPerSecond: sm.eventsPerSecond.Load(),
		AvgLatencyMs:    float64(avgLatencyNs) / 1e6,
		MaxLatencyMs:    sm.maxLatency.Load() / 1e6,
		MemoryUsedMB:    float64(sm.memoryUsed.Load()) / (1024 * 1024),
		PeakMemoryMB:    float64(sm.peakMemoryUsed.Load()) / (1024 * 1024),
		BufferSize:      sm.bufferSize.Load(),
		ProgressPercent: float32(sm.progressPercent.Load()),
		Duration:        duration,
		EventTypes:      eventTypes,
	}
}

// Reset resets all metrics
func (sm *StreamMetrics) Reset() {
	sm.eventsProcessed.Store(0)
	sm.eventsDropped.Store(0)
	sm.eventsErrored.Store(0)
	sm.bytesProcessed.Store(0)
	sm.bytesPerSecond.Store(0)
	sm.eventsPerSecond.Store(0)
	sm.latencySum.Store(0)
	sm.latencyCount.Store(0)
	sm.maxLatency.Store(0)
	sm.memoryUsed.Store(0)
	sm.bufferSize.Store(0)
	sm.progressPercent.Store(0)

	sm.eventTypesMu.Lock()
	sm.eventTypes = make(map[string]*EventTypeMetrics)
	sm.eventTypesMu.Unlock()

	sm.startTime = time.Now()
}

// Close stops the metrics collector
func (sm *StreamMetrics) Close() {
	sm.cancel()
	close(sm.stopChan)
	sm.wg.Wait()
	if sm.sampleTicker != nil {
		sm.sampleTicker.Stop()
	}
}

// startSampling starts background sampling
func (sm *StreamMetrics) startSampling() {
	sm.sampleTicker = time.NewTicker(sm.sampleInterval)
	sm.wg.Add(1)

	go func() {
		defer sm.wg.Done()

		lastEvents := int64(0)
		lastBytes := int64(0)
		lastTime := time.Now()

		for {
			select {
			case <-sm.ctx.Done():
				return
			case <-sm.stopChan:
				return
			case <-sm.sampleTicker.C:
				now := time.Now()
				elapsed := now.Sub(lastTime).Seconds()

				// Calculate rates
				currentEvents := sm.eventsProcessed.Load()
				currentBytes := sm.bytesProcessed.Load()

				if elapsed > 0 {
					eventsPerSec := int64(float64(currentEvents-lastEvents) / elapsed)
					bytesPerSec := int64(float64(currentBytes-lastBytes) / elapsed)

					sm.eventsPerSecond.Store(eventsPerSec)
					sm.bytesPerSecond.Store(bytesPerSec)
				}

				lastEvents = currentEvents
				lastBytes = currentBytes
				lastTime = now
			}
		}
	}()
}

// recordEventType records metrics for a specific event type
func (sm *StreamMetrics) recordEventType(eventType string, event events.Event) {
	sm.eventTypesMu.RLock()
	metrics, exists := sm.eventTypes[eventType]
	sm.eventTypesMu.RUnlock()

	if !exists {
		sm.eventTypesMu.Lock()
		// Double-check after acquiring write lock
		if metrics, exists = sm.eventTypes[eventType]; !exists {
			metrics = &EventTypeMetrics{}
			sm.eventTypes[eventType] = metrics
		}
		sm.eventTypesMu.Unlock()
	}

	metrics.Count.Add(1)
	size := sm.estimateEventSize(event)
	metrics.Bytes.Add(size)
}

// estimateEventSize estimates the size of an event
func (sm *StreamMetrics) estimateEventSize(event events.Event) int64 {
	// This is a simplified estimation
	// In practice, this would depend on the actual encoding
	switch event.Type() {
	case events.EventTypeTextMessageStart, events.EventTypeTextMessageContent, events.EventTypeTextMessageEnd:
		return 100 // Base size for message events
	case events.EventTypeToolCallStart, events.EventTypeToolCallArgs, events.EventTypeToolCallEnd:
		return 200 // Base size for tool events
	case events.EventTypeStateSnapshot, events.EventTypeStateDelta:
		return 500 // State events vary widely
	default:
		return 256 // Default estimate
	}
}

// GetEventTypeMetrics returns metrics for a specific event type
func (sm *StreamMetrics) GetEventTypeMetrics(eventType string) (EventTypeSnapshot, bool) {
	sm.eventTypesMu.RLock()
	defer sm.eventTypesMu.RUnlock()

	metrics, exists := sm.eventTypes[eventType]
	if !exists {
		return EventTypeSnapshot{}, false
	}

	count := metrics.Count.Load()
	bytes := metrics.Bytes.Load()
	avgSize := int64(0)
	if count > 0 {
		avgSize = bytes / count
	}

	return EventTypeSnapshot{
		Count:   count,
		Bytes:   bytes,
		AvgSize: avgSize,
		Errors:  metrics.Errors.Load(),
	}, true
}

// GetThroughput returns current throughput metrics
func (sm *StreamMetrics) GetThroughput() (eventsPerSec, bytesPerSec int64) {
	return sm.eventsPerSecond.Load(), sm.bytesPerSecond.Load()
}

// GetProgress returns current progress percentage
func (sm *StreamMetrics) GetProgress() float32 {
	return float32(sm.progressPercent.Load())
}

// GetDuration returns the duration since metrics started
func (sm *StreamMetrics) GetDuration() time.Duration {
	return time.Since(sm.startTime)
}
