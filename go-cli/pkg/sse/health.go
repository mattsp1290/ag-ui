package sse

import (
	"sync"
	"sync/atomic"
	"time"
)

// SSEHealth tracks connection health and metrics for SSE streams
type SSEHealth struct {
	// Connection lifecycle timestamps
	connectedAt    atomic.Value // time.Time
	disconnectedAt atomic.Value // time.Time
	lastEventAt    atomic.Value // time.Time

	// Event counters (using atomic for thread safety)
	bytesRead         atomic.Uint64
	framesRead        atomic.Uint64
	parseErrors       atomic.Uint64
	reconnectAttempts atomic.Uint64
	totalEvents       atomic.Uint64

	// Connection state
	connectionID atomic.Value // string
	isConnected  atomic.Bool

	// Rolling window for events/sec calculation
	eventWindow     *EventRateWindow
	eventWindowLock sync.RWMutex

	// Error tracking
	lastError     atomic.Value // error
	errorCount    atomic.Uint64
	lastErrorTime atomic.Value // time.Time

	// Throughput tracking
	startTime time.Time
}

// NewSSEHealth creates a new SSE health tracker
func NewSSEHealth() *SSEHealth {
	h := &SSEHealth{
		eventWindow: NewEventRateWindow(10 * time.Second), // 10-second rolling window
		startTime:   time.Now(),
	}
	
	// Initialize atomic values with zero values
	h.connectedAt.Store(time.Time{})
	h.disconnectedAt.Store(time.Time{})
	h.lastEventAt.Store(time.Time{})
	h.connectionID.Store("")
	// Don't store nil in atomic.Value - use a struct wrapper or leave uninitialized
	h.lastErrorTime.Store(time.Time{})
	
	return h
}

// RecordConnect records a successful connection
func (h *SSEHealth) RecordConnect(connectionID string) {
	h.connectedAt.Store(time.Now())
	h.connectionID.Store(connectionID)
	h.isConnected.Store(true)
	h.disconnectedAt.Store(time.Time{}) // Clear disconnect time
}

// RecordDisconnect records a disconnection
func (h *SSEHealth) RecordDisconnect(err error) {
	h.disconnectedAt.Store(time.Now())
	h.isConnected.Store(false)
	if err != nil {
		h.RecordError(err)
	}
}

// RecordEvent records a successfully parsed event
func (h *SSEHealth) RecordEvent(bytesRead int) {
	now := time.Now()
	h.lastEventAt.Store(now)
	h.framesRead.Add(1)
	h.totalEvents.Add(1)
	h.bytesRead.Add(uint64(bytesRead))
	
	// Update rolling window
	h.eventWindowLock.Lock()
	h.eventWindow.Record(now)
	h.eventWindowLock.Unlock()
}

// RecordParseError records a parse error
func (h *SSEHealth) RecordParseError(err error) {
	h.parseErrors.Add(1)
	h.RecordError(err)
}

// RecordReconnectAttempt records a reconnection attempt
func (h *SSEHealth) RecordReconnectAttempt() {
	h.reconnectAttempts.Add(1)
}

// RecordError records a general error
func (h *SSEHealth) RecordError(err error) {
	if err != nil {
		h.lastError.Store(err)
		h.lastErrorTime.Store(time.Now())
		h.errorCount.Add(1)
	}
}

// GetEventsPerSecond returns the current events/sec rate
func (h *SSEHealth) GetEventsPerSecond() float64 {
	h.eventWindowLock.RLock()
	defer h.eventWindowLock.RUnlock()
	return h.eventWindow.Rate()
}

// GetMetrics returns a snapshot of current metrics
func (h *SSEHealth) GetMetrics() Metrics {
	var connectedAt, disconnectedAt, lastEventAt, lastErrorTime time.Time
	var connectionID string
	var lastError error

	// Safely load atomic values
	if t, ok := h.connectedAt.Load().(time.Time); ok {
		connectedAt = t
	}
	if t, ok := h.disconnectedAt.Load().(time.Time); ok {
		disconnectedAt = t
	}
	if t, ok := h.lastEventAt.Load().(time.Time); ok {
		lastEventAt = t
	}
	if t, ok := h.lastErrorTime.Load().(time.Time); ok {
		lastErrorTime = t
	}
	if id, ok := h.connectionID.Load().(string); ok {
		connectionID = id
	}
	// Handle both error and nil case
	if val := h.lastError.Load(); val != nil {
		if err, ok := val.(error); ok {
			lastError = err
		}
	}

	// Calculate duration
	var connectionDuration time.Duration
	if !connectedAt.IsZero() {
		if h.isConnected.Load() {
			connectionDuration = time.Since(connectedAt)
		} else if !disconnectedAt.IsZero() {
			connectionDuration = disconnectedAt.Sub(connectedAt)
		}
	}

	return Metrics{
		// Connection info
		ConnectionID:       connectionID,
		IsConnected:        h.isConnected.Load(),
		ConnectedAt:        connectedAt,
		DisconnectedAt:     disconnectedAt,
		ConnectionDuration: connectionDuration,

		// Event metrics
		BytesRead:         h.bytesRead.Load(),
		FramesRead:        h.framesRead.Load(),
		TotalEvents:       h.totalEvents.Load(),
		EventsPerSecond:   h.GetEventsPerSecond(),
		LastEventAt:       lastEventAt,

		// Error metrics
		ParseErrors:       h.parseErrors.Load(),
		ReconnectAttempts: h.reconnectAttempts.Load(),
		ErrorCount:        h.errorCount.Load(),
		LastError:         lastError,
		LastErrorTime:     lastErrorTime,

		// Overall metrics
		UptimeSeconds: time.Since(h.startTime).Seconds(),
	}
}

// Reset resets all metrics (useful for testing)
func (h *SSEHealth) Reset() {
	// Reset counters
	h.bytesRead.Store(0)
	h.framesRead.Store(0)
	h.parseErrors.Store(0)
	h.reconnectAttempts.Store(0)
	h.totalEvents.Store(0)
	h.errorCount.Store(0)
	
	// Reset state
	h.isConnected.Store(false)
	
	// Reset atomic values
	h.connectedAt.Store(time.Time{})
	h.disconnectedAt.Store(time.Time{})
	h.lastEventAt.Store(time.Time{})
	h.connectionID.Store("")
	// Don't store nil - leave uninitialized (Load will return nil)
	h.lastErrorTime.Store(time.Time{})
	
	// Reset window
	h.eventWindowLock.Lock()
	h.eventWindow = NewEventRateWindow(10 * time.Second)
	h.eventWindowLock.Unlock()
	
	// Reset start time
	h.startTime = time.Now()
}

// Metrics represents a snapshot of SSE health metrics
type Metrics struct {
	// Connection info
	ConnectionID       string        `json:"connectionId"`
	IsConnected        bool          `json:"isConnected"`
	ConnectedAt        time.Time     `json:"connectedAt,omitempty"`
	DisconnectedAt     time.Time     `json:"disconnectedAt,omitempty"`
	ConnectionDuration time.Duration `json:"connectionDuration,omitempty"`

	// Event metrics
	BytesRead       uint64    `json:"bytesRead"`
	FramesRead      uint64    `json:"framesRead"`
	TotalEvents     uint64    `json:"totalEvents"`
	EventsPerSecond float64   `json:"eventsPerSecond"`
	LastEventAt     time.Time `json:"lastEventAt,omitempty"`

	// Error metrics
	ParseErrors       uint64    `json:"parseErrors"`
	ReconnectAttempts uint64    `json:"reconnectAttempts"`
	ErrorCount        uint64    `json:"errorCount"`
	LastError         error     `json:"lastError,omitempty"`
	LastErrorTime     time.Time `json:"lastErrorTime,omitempty"`

	// Overall metrics
	UptimeSeconds float64 `json:"uptimeSeconds"`
}

// GetAverageEventRate returns the average events per second over the lifetime
func (m *Metrics) GetAverageEventRate() float64 {
	if m.UptimeSeconds > 0 {
		return float64(m.TotalEvents) / m.UptimeSeconds
	}
	return 0
}

// GetErrorRate returns the error rate as a percentage
func (m *Metrics) GetErrorRate() float64 {
	total := m.TotalEvents + m.ParseErrors
	if total > 0 {
		return float64(m.ParseErrors) / float64(total) * 100
	}
	return 0
}

// GetBytesPerEvent returns the average bytes per event
func (m *Metrics) GetBytesPerEvent() float64 {
	if m.FramesRead > 0 {
		return float64(m.BytesRead) / float64(m.FramesRead)
	}
	return 0
}