package websocket

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// HeartbeatState represents the state of the heartbeat mechanism
type HeartbeatState int32

const (
	// HeartbeatStopped indicates the heartbeat is not running
	HeartbeatStopped HeartbeatState = iota
	// HeartbeatStarting indicates the heartbeat is starting
	HeartbeatStarting
	// HeartbeatRunning indicates the heartbeat is active
	HeartbeatRunning
	// HeartbeatStopping indicates the heartbeat is stopping
	HeartbeatStopping
)

// String returns the string representation of the heartbeat state
func (s HeartbeatState) String() string {
	switch s {
	case HeartbeatStopped:
		return "stopped"
	case HeartbeatStarting:
		return "starting"
	case HeartbeatRunning:
		return "running"
	case HeartbeatStopping:
		return "stopping"
	default:
		return "unknown"
	}
}

// HeartbeatManager manages the ping/pong heartbeat mechanism for WebSocket connections
type HeartbeatManager struct {
	// Configuration
	pingPeriod time.Duration
	pongWait   time.Duration

	// Connection reference
	connection *Connection

	// State management
	state       int32 // atomic access with HeartbeatState
	isHealthy   int32 // atomic boolean (0 = unhealthy, 1 = healthy)
	lastPongAt  int64 // atomic unix timestamp
	lastPingAt  int64 // atomic unix timestamp
	missedPongs int32 // atomic counter

	// Channels for control
	stopCh  chan struct{}
	resetCh chan struct{}

	// Statistics
	stats *HeartbeatStats

	// Goroutine management
	wg       sync.WaitGroup
	stopOnce sync.Once
}

// HeartbeatStats tracks heartbeat statistics
type HeartbeatStats struct {
	PingsSent        int64
	PongsReceived    int64
	MissedPongs      int64
	HealthChecks     int64
	UnhealthyPeriods int64
	LastPingAt       time.Time
	LastPongAt       time.Time
	AverageRTT       time.Duration
	MinRTT           time.Duration
	MaxRTT           time.Duration
	mutex            sync.RWMutex
}

// NewHeartbeatManager creates a new heartbeat manager
func NewHeartbeatManager(connection *Connection, pingPeriod, pongWait time.Duration) *HeartbeatManager {
	now := time.Now()
	return &HeartbeatManager{
		pingPeriod: pingPeriod,
		pongWait:   pongWait,
		connection: connection,
		state:      int32(HeartbeatStopped),
		isHealthy:  1, // Start as healthy
		lastPongAt: now.Unix(), // Initialize to current time
		stopCh:     make(chan struct{}),
		resetCh:    make(chan struct{}, 1),
		stats:      &HeartbeatStats{
			LastPongAt: now, // Initialize stats too
		},
	}
}

// Start begins the heartbeat mechanism
func (h *HeartbeatManager) Start(ctx context.Context) {
	if !h.setState(HeartbeatStarting) {
		return // Already starting or running
	}

	h.connection.config.Logger.Debug("Starting heartbeat manager",
		zap.Duration("ping_period", h.pingPeriod),
		zap.Duration("pong_wait", h.pongWait))

	// Mark as healthy initially
	atomic.StoreInt32(&h.isHealthy, 1)
	atomic.StoreInt64(&h.lastPongAt, time.Now().Unix())

	h.setState(HeartbeatRunning)

	h.wg.Add(2)
	go h.pingLoop(ctx)
	go h.healthCheckLoop(ctx)
}

// Stop stops the heartbeat mechanism
func (h *HeartbeatManager) Stop() {
	h.stopOnce.Do(func() {
		h.connection.config.Logger.Debug("Stopping heartbeat manager")

		h.setState(HeartbeatStopping)
		close(h.stopCh)

		// Wait for goroutines with timeout
		done := make(chan struct{})
		go func() {
			h.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			h.connection.config.Logger.Debug("Heartbeat manager stopped")
		case <-time.After(1 * time.Second):
			h.connection.config.Logger.Warn("Timeout waiting for heartbeat goroutines to stop")
		}

		h.setState(HeartbeatStopped)
	})
}

// OnPong is called when a pong message is received
func (h *HeartbeatManager) OnPong() {
	now := time.Now()
	atomic.StoreInt64(&h.lastPongAt, now.Unix())
	atomic.StoreInt32(&h.isHealthy, 1)
	atomic.StoreInt32(&h.missedPongs, 0)

	// Update statistics
	h.stats.mutex.Lock()
	h.stats.PongsReceived++
	h.stats.LastPongAt = now

	// Calculate RTT if we have a recent ping
	if h.stats.LastPingAt.After(now.Add(-h.pingPeriod)) {
		rtt := now.Sub(h.stats.LastPingAt)
		h.updateRTTStats(rtt)
	}
	h.stats.mutex.Unlock()

	h.connection.config.Logger.Debug("Received pong",
		zap.Time("at", now),
		zap.Int32("missed_pongs", atomic.LoadInt32(&h.missedPongs)))
}

// IsHealthy returns true if the connection is considered healthy
func (h *HeartbeatManager) IsHealthy() bool {
	return atomic.LoadInt32(&h.isHealthy) == 1
}

// GetState returns the current heartbeat state
func (h *HeartbeatManager) GetState() HeartbeatState {
	return HeartbeatState(atomic.LoadInt32(&h.state))
}

// GetStats returns a copy of the heartbeat statistics
func (h *HeartbeatManager) GetStats() HeartbeatStats {
	h.stats.mutex.RLock()
	defer h.stats.mutex.RUnlock()
	return *h.stats
}

// Reset resets the heartbeat state
func (h *HeartbeatManager) Reset() {
	select {
	case h.resetCh <- struct{}{}:
	default:
	}
}

// setState atomically sets the heartbeat state
func (h *HeartbeatManager) setState(state HeartbeatState) bool {
	oldState := HeartbeatState(atomic.LoadInt32(&h.state))

	// Check if state transition is valid
	if !h.isValidStateTransition(oldState, state) {
		return false
	}

	atomic.StoreInt32(&h.state, int32(state))

	h.connection.config.Logger.Debug("Heartbeat state changed",
		zap.String("from", oldState.String()),
		zap.String("to", state.String()))

	return true
}

// isValidStateTransition checks if a state transition is valid
func (h *HeartbeatManager) isValidStateTransition(from, to HeartbeatState) bool {
	switch from {
	case HeartbeatStopped:
		return to == HeartbeatStarting
	case HeartbeatStarting:
		return to == HeartbeatRunning || to == HeartbeatStopping
	case HeartbeatRunning:
		return to == HeartbeatStopping
	case HeartbeatStopping:
		return to == HeartbeatStopped
	default:
		return false
	}
}

// pingLoop sends periodic ping messages
func (h *HeartbeatManager) pingLoop(ctx context.Context) {
	defer h.wg.Done()

	// Skip ping loop if pingPeriod is not configured
	if h.pingPeriod <= 0 {
		h.connection.config.Logger.Debug("Ping loop disabled (pingPeriod=0)")
		return
	}

	ticker := time.NewTicker(h.pingPeriod)
	defer ticker.Stop()

	h.connection.config.Logger.Debug("Starting ping loop")

	for {
		select {
		case <-ctx.Done():
			h.connection.config.Logger.Debug("Ping loop stopped due to context cancellation")
			return
		case <-h.stopCh:
			h.connection.config.Logger.Debug("Ping loop stopped due to stop signal")
			return
		case <-h.resetCh:
			ticker.Reset(h.pingPeriod)
			continue
		case <-ticker.C:
			// Check if we should still be running before sending ping
			select {
			case <-ctx.Done():
				h.connection.config.Logger.Debug("Context cancelled before sending ping")
				return
			case <-h.stopCh:
				h.connection.config.Logger.Debug("Stop signal received before sending ping")
				return
			default:
				if err := h.sendPing(); err != nil {
					h.connection.config.Logger.Error("Failed to send ping",
						zap.Error(err))

					// Mark as unhealthy and potentially trigger reconnection
					atomic.StoreInt32(&h.isHealthy, 0)
					if h.connection.State() == StateConnected {
						h.connection.triggerReconnect()
					}
					return
				}
			}
		}
	}
}

// healthCheckLoop monitors connection health based on pong responses
func (h *HeartbeatManager) healthCheckLoop(ctx context.Context) {
	defer h.wg.Done()

	// Skip health checks if pongWait is not configured
	if h.pongWait <= 0 {
		h.connection.config.Logger.Debug("Health check loop disabled (pongWait=0)")
		return
	}

	ticker := time.NewTicker(h.pongWait / 2) // Check twice per pong wait period
	defer ticker.Stop()

	h.connection.config.Logger.Debug("Starting health check loop")

	for {
		select {
		case <-ctx.Done():
			h.connection.config.Logger.Debug("Health check loop stopped due to context cancellation")
			return
		case <-h.stopCh:
			h.connection.config.Logger.Debug("Health check loop stopped due to stop signal")
			return
		case <-ticker.C:
			// Check if we should still be running before checking health
			select {
			case <-ctx.Done():
				h.connection.config.Logger.Debug("Context cancelled before health check")
				return
			case <-h.stopCh:
				h.connection.config.Logger.Debug("Stop signal received before health check")
				return
			default:
				h.checkHealth()
			}
		}
	}
}

// sendPing sends a ping message to the WebSocket connection
func (h *HeartbeatManager) sendPing() error {
	h.connection.connMutex.RLock()
	conn := h.connection.conn
	h.connection.connMutex.RUnlock()

	if conn == nil {
		return websocket.ErrCloseSent
	}

	now := time.Now()

	// Set write deadline
	conn.SetWriteDeadline(now.Add(h.connection.config.WriteTimeout))

	// Send ping
	if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
		return err
	}

	// Update statistics
	atomic.StoreInt64(&h.lastPingAt, now.Unix())

	h.stats.mutex.Lock()
	h.stats.PingsSent++
	h.stats.LastPingAt = now
	h.stats.mutex.Unlock()

	h.connection.config.Logger.Debug("Sent ping",
		zap.Time("at", now))

	return nil
}

// checkHealth monitors the connection health based on pong responses
func (h *HeartbeatManager) checkHealth() {
	now := time.Now()
	lastPongAt := time.Unix(atomic.LoadInt64(&h.lastPongAt), 0)

	h.stats.mutex.Lock()
	h.stats.HealthChecks++
	h.stats.mutex.Unlock()

	// Check if we've missed a pong
	if now.Sub(lastPongAt) > h.pongWait {
		missedPongs := atomic.AddInt32(&h.missedPongs, 1)

		h.stats.mutex.Lock()
		h.stats.MissedPongs++
		h.stats.mutex.Unlock()

		h.connection.config.Logger.Warn("Missed pong response",
			zap.Time("last_pong", lastPongAt),
			zap.Duration("since", now.Sub(lastPongAt)),
			zap.Int32("missed_count", missedPongs))

		// Mark as unhealthy after first missed pong
		if atomic.LoadInt32(&h.isHealthy) == 1 {
			atomic.StoreInt32(&h.isHealthy, 0)

			h.stats.mutex.Lock()
			h.stats.UnhealthyPeriods++
			h.stats.mutex.Unlock()

			h.connection.config.Logger.Warn("Connection marked as unhealthy")
		}

		// Trigger reconnection after multiple missed pongs
		if missedPongs >= 3 {
			h.connection.config.Logger.Error("Too many missed pongs, triggering reconnection",
				zap.Int32("missed_pongs", missedPongs))

			if h.connection.State() == StateConnected {
				h.connection.triggerReconnect()
			}
		}
	}
}

// updateRTTStats updates the round-trip time statistics
func (h *HeartbeatManager) updateRTTStats(rtt time.Duration) {
	// Update average RTT using exponential moving average
	if h.stats.AverageRTT == 0 {
		h.stats.AverageRTT = rtt
	} else {
		// Use a smoothing factor of 0.125 (1/8) for exponential moving average
		h.stats.AverageRTT = time.Duration(
			float64(h.stats.AverageRTT)*0.875 + float64(rtt)*0.125,
		)
	}

	// Update min RTT
	if h.stats.MinRTT == 0 || rtt < h.stats.MinRTT {
		h.stats.MinRTT = rtt
	}

	// Update max RTT
	if rtt > h.stats.MaxRTT {
		h.stats.MaxRTT = rtt
	}
}

// GetLastPongTime returns the timestamp of the last received pong
func (h *HeartbeatManager) GetLastPongTime() time.Time {
	return time.Unix(atomic.LoadInt64(&h.lastPongAt), 0)
}

// GetLastPingTime returns the timestamp of the last sent ping
func (h *HeartbeatManager) GetLastPingTime() time.Time {
	return time.Unix(atomic.LoadInt64(&h.lastPingAt), 0)
}

// GetMissedPongCount returns the number of consecutive missed pongs
func (h *HeartbeatManager) GetMissedPongCount() int32 {
	return atomic.LoadInt32(&h.missedPongs)
}

// GetPingPeriod returns the ping period
func (h *HeartbeatManager) GetPingPeriod() time.Duration {
	return h.pingPeriod
}

// GetPongWait returns the pong wait timeout
func (h *HeartbeatManager) GetPongWait() time.Duration {
	return h.pongWait
}

// SetPingPeriod updates the ping period
func (h *HeartbeatManager) SetPingPeriod(period time.Duration) {
	h.pingPeriod = period
	h.Reset() // Reset the ping loop with new period
}

// SetPongWait updates the pong wait timeout
func (h *HeartbeatManager) SetPongWait(wait time.Duration) {
	h.pongWait = wait
}

// GetConnectionHealth returns a health score between 0 and 1
// 1 = perfectly healthy, 0 = completely unhealthy
func (h *HeartbeatManager) GetConnectionHealth() float64 {
	if !h.IsHealthy() {
		return 0.0
	}

	now := time.Now()
	lastPongAt := h.GetLastPongTime()

	// Calculate health based on how recent the last pong was
	// within the pong wait period
	timeSinceLastPong := now.Sub(lastPongAt)
	if timeSinceLastPong > h.pongWait {
		return 0.0
	}

	// Linear interpolation: 1.0 at t=0, 0.0 at t=pongWait
	health := 1.0 - (float64(timeSinceLastPong) / float64(h.pongWait))
	if health < 0.0 {
		health = 0.0
	}

	return health
}

// GetDetailedHealthStatus returns detailed health information
func (h *HeartbeatManager) GetDetailedHealthStatus() map[string]interface{} {
	stats := h.GetStats()
	now := time.Now()

	return map[string]interface{}{
		"is_healthy":           h.IsHealthy(),
		"health_score":         h.GetConnectionHealth(),
		"state":                h.GetState().String(),
		"last_ping_at":         h.GetLastPingTime(),
		"last_pong_at":         h.GetLastPongTime(),
		"time_since_last_pong": now.Sub(h.GetLastPongTime()),
		"missed_pongs":         h.GetMissedPongCount(),
		"ping_period":          h.GetPingPeriod(),
		"pong_wait":            h.GetPongWait(),
		"total_pings_sent":     stats.PingsSent,
		"total_pongs_received": stats.PongsReceived,
		"total_missed_pongs":   stats.MissedPongs,
		"health_checks":        stats.HealthChecks,
		"unhealthy_periods":    stats.UnhealthyPeriods,
		"average_rtt":          stats.AverageRTT,
		"min_rtt":              stats.MinRTT,
		"max_rtt":              stats.MaxRTT,
	}
}
