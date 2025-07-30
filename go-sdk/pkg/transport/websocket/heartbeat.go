package websocket

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	
	"github.com/ag-ui/go-sdk/pkg/internal/timeconfig"
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
	lastPongAt  int64 // atomic unix nano timestamp
	lastPingAt  int64 // atomic unix nano timestamp
	missedPongs int32 // atomic counter

	// Channels for control
	stopCh  chan struct{}
	resetCh chan struct{}

	// Statistics
	stats *HeartbeatStats

	// Goroutine management with enhanced shutdown control
	wg         sync.WaitGroup
	stopOnce   sync.Once
	ctx        context.Context    // Context for all heartbeat operations
	cancel     context.CancelFunc // Cancel function for immediate shutdown
	shutdownCh chan struct{}      // Channel to signal shutdown completion
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
	ctx, cancel := context.WithCancel(context.Background())
	
	return &HeartbeatManager{
		pingPeriod: pingPeriod,
		pongWait:   pongWait,
		connection: connection,
		state:      int32(HeartbeatStopped),
		isHealthy:  1, // Start as healthy
		lastPongAt: 0, // Will be set on first pong or start
		lastPingAt: 0, // Will be set on first ping
		stopCh:     make(chan struct{}),
		resetCh:    make(chan struct{}, 1),
		ctx:        ctx,
		cancel:     cancel,
		shutdownCh: make(chan struct{}),
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
	now := time.Now()
	atomic.StoreInt64(&h.lastPongAt, now.UnixNano())
	atomic.StoreInt64(&h.lastPingAt, 0) // Will be set when first ping is sent

	// Set state to running before starting goroutines to ensure proper cleanup
	h.setState(HeartbeatRunning)

	// Use heartbeat's own context for controlling goroutines
	h.wg.Add(2)
	go h.pingLoop(h.ctx)      // Use heartbeat context instead of external context
	go h.healthCheckLoop(h.ctx) // Use heartbeat context instead of external context
}

// Stop stops the heartbeat mechanism with enhanced aggressive shutdown timeout
func (h *HeartbeatManager) Stop() {
	h.stopOnce.Do(func() {
		h.connection.config.Logger.Debug("Stopping heartbeat manager with enhanced aggressive shutdown")

		h.setState(HeartbeatStopping)
		
		// Phase 1: Cancel context first to signal all goroutines to stop immediately
		h.connection.config.Logger.Debug("Cancelling heartbeat context")
		h.cancel()
		
		// Phase 2: Close stop channel as backup signal (only once)
		func() {
			defer func() { recover() }() // Ignore panic if already closed
			close(h.stopCh)
		}()

		// Phase 3: Wait for all heartbeat goroutines to finish with aggressive timeout and retry
		h.connection.config.Logger.Debug("Waiting for heartbeat goroutines to finish")
		
		// Simplified shutdown with single short timeout for tests
		timeout := 50 * time.Millisecond
		if !timeconfig.IsTestMode() {
			timeout = 200 * time.Millisecond
		}
		
		done := make(chan struct{})
		go func() {
			h.wg.Wait()
			close(done)
		}()

		// Wait for goroutines with very short timeout
		select {
		case <-done:
			h.connection.config.Logger.Debug("Heartbeat manager stopped successfully")
			h.setState(HeartbeatStopped)
			
			// Signal shutdown completion
			func() {
				defer func() { recover() }() // Ignore panic if already closed
				close(h.shutdownCh)
			}()
			return // Success - exit immediately
			
		case <-time.After(timeout):
			h.connection.config.Logger.Debug("Heartbeat shutdown timeout - forcing completion")
		}

		// Final state update regardless of goroutine completion status
		h.setState(HeartbeatStopped)
		
		// Signal shutdown completion
		func() {
			defer func() { recover() }() // Ignore panic if already closed
			close(h.shutdownCh)
		}()
		
		h.connection.config.Logger.Debug("Heartbeat manager shutdown completed")
	})
}

// OnPong is called when a pong message is received
func (h *HeartbeatManager) OnPong() {
	now := time.Now()
	atomic.StoreInt64(&h.lastPongAt, now.UnixNano())
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
	defer h.connection.config.Logger.Debug("Ping loop goroutine exited")

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
			h.connection.config.Logger.Debug("Ping loop stopped due to heartbeat context cancellation")
			return
		case <-h.connection.ctx.Done():
			h.connection.config.Logger.Debug("Ping loop stopped due to connection context cancellation")
			return
		case <-h.stopCh:
			h.connection.config.Logger.Debug("Ping loop stopped due to stop signal")
			return
		case <-h.resetCh:
			// Check exit conditions before reset
			if h.GetState() != HeartbeatRunning {
				h.connection.config.Logger.Debug("Ping loop exiting - heartbeat not running")
				return
			}
			ticker.Reset(h.pingPeriod)
			continue
		case <-ticker.C:
			// Check all exit conditions before sending ping
			select {
			case <-ctx.Done():
				h.connection.config.Logger.Debug("Ping loop stopped due to heartbeat context cancellation before sending ping")
				return
			case <-h.connection.ctx.Done():
				h.connection.config.Logger.Debug("Ping loop stopped due to connection context cancellation before sending ping")
				return
			case <-h.stopCh:
				h.connection.config.Logger.Debug("Ping loop stopped due to stop signal before sending ping")
				return
			default:
			}
			
			// Check heartbeat state
			if h.GetState() != HeartbeatRunning {
				h.connection.config.Logger.Debug("Ping loop exiting - heartbeat not running")
				return
			}
			
			// Check connection state
			connState := h.connection.State()
			if connState == StateClosing || connState == StateClosed {
				h.connection.config.Logger.Debug("Ping loop exiting - connection closing/closed")
				return
			}
			
			// Send ping with error handling and timeout
			if err := h.sendPingWithTimeout(); err != nil {
				// Check if error is due to context cancellation
				select {
				case <-ctx.Done():
					h.connection.config.Logger.Debug("Ping loop stopped during ping send due to heartbeat context cancellation")
					return
				case <-h.connection.ctx.Done():
					h.connection.config.Logger.Debug("Ping loop stopped during ping send due to connection context cancellation")
					return
				default:
				}
				
				h.connection.config.Logger.Error("Failed to send ping", zap.Error(err))

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

// healthCheckLoop monitors connection health based on pong responses
func (h *HeartbeatManager) healthCheckLoop(ctx context.Context) {
	defer h.wg.Done()
	defer h.connection.config.Logger.Debug("Health check loop goroutine exited")

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
			h.connection.config.Logger.Debug("Health check loop stopped due to heartbeat context cancellation")
			return
		case <-h.connection.ctx.Done():
			h.connection.config.Logger.Debug("Health check loop stopped due to connection context cancellation")
			return
		case <-h.stopCh:
			h.connection.config.Logger.Debug("Health check loop stopped due to stop signal")
			return
		case <-ticker.C:
			// Check all exit conditions before health check
			select {
			case <-ctx.Done():
				h.connection.config.Logger.Debug("Health check loop stopped due to heartbeat context cancellation before health check")
				return
			case <-h.connection.ctx.Done():
				h.connection.config.Logger.Debug("Health check loop stopped due to connection context cancellation before health check")
				return
			case <-h.stopCh:
				h.connection.config.Logger.Debug("Health check loop stopped due to stop signal before health check")
				return
			default:
			}
			
			// Check heartbeat state
			if h.GetState() != HeartbeatRunning {
				h.connection.config.Logger.Debug("Health check loop exiting - heartbeat not running")
				return
			}
			
			// Check connection state
			connState := h.connection.State()
			if connState == StateClosing || connState == StateClosed {
				h.connection.config.Logger.Debug("Health check loop exiting - connection closing/closed")
				return
			}
			
			// Perform health check
			h.checkHealth()
		}
	}
}

// sendPing sends a ping message to the WebSocket connection
func (h *HeartbeatManager) sendPing() error {
	return h.sendPingWithTimeout()
}

// sendPingWithTimeout sends a ping message with timeout protection
func (h *HeartbeatManager) sendPingWithTimeout() error {
	h.connection.connMutex.Lock()
	defer h.connection.connMutex.Unlock()
	
	conn := h.connection.conn
	if conn == nil {
		return websocket.ErrCloseSent
	}

	now := time.Now()

	// Set write deadline with context check
	writeTimeout := h.connection.config.WriteTimeout
	if writeTimeout > 1*time.Second {
		writeTimeout = 1 * time.Second // Cap ping timeout at 1 second
	}
	writeDeadline := now.Add(writeTimeout)
	
	// Check if connection context has a sooner deadline
	if deadline, ok := h.connection.ctx.Deadline(); ok && deadline.Before(writeDeadline) {
		writeDeadline = deadline
	}
	
	conn.SetWriteDeadline(writeDeadline)

	// Send ping with panic recovery (must be done while holding the lock)
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				h.connection.config.Logger.Debug("Recovered from ping send panic", zap.Any("panic", r))
				err = websocket.ErrCloseSent
			}
		}()
		err = conn.WriteMessage(websocket.PingMessage, nil)
	}()
	
	if err != nil {
		// Check for timeout errors which might be due to context deadline
		if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
			// Check if timeout was due to context cancellation
			select {
			case <-h.connection.ctx.Done():
				h.connection.config.Logger.Debug("Ping timeout due to context cancellation")
				return h.connection.ctx.Err()
			default:
				// Regular timeout
			}
		}
		return err
	}

	// Update statistics
	atomic.StoreInt64(&h.lastPingAt, now.UnixNano())

	h.stats.mutex.Lock()
	h.stats.PingsSent++
	h.stats.LastPingAt = now
	h.stats.mutex.Unlock()

	h.connection.config.Logger.Debug("Sent ping", zap.Time("at", now))

	return nil
}

// checkHealth monitors the connection health based on pong responses
func (h *HeartbeatManager) checkHealth() {
	// Check if heartbeat is still running
	if h.GetState() != HeartbeatRunning {
		return
	}

	now := time.Now()
	lastPongAtNano := atomic.LoadInt64(&h.lastPongAt)
	
	// If lastPongAt is 0, we haven't received any pongs yet
	// Use the start time for initial health check
	if lastPongAtNano == 0 {
		lastPongAtNano = atomic.LoadInt64(&h.lastPingAt)
		// If we also haven't sent any pings, use current time
		if lastPongAtNano == 0 {
			lastPongAtNano = now.UnixNano()
		}
	}
	
	lastPongAt := time.Unix(0, lastPongAtNano)

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
		if missedPongs >= 5 { // Increased threshold to allow for recovery
			h.connection.config.Logger.Error("Too many missed pongs, triggering reconnection",
				zap.Int32("missed_pongs", missedPongs))

			if h.connection.State() == StateConnected {
				h.connection.triggerReconnect()
			}
		}
	} else {
		// We received a pong recently, ensure we're marked as healthy
		if atomic.LoadInt32(&h.isHealthy) == 0 {
			atomic.StoreInt32(&h.isHealthy, 1)
			atomic.StoreInt32(&h.missedPongs, 0)
			h.connection.config.Logger.Info("Connection recovered - marked as healthy")
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
	nano := atomic.LoadInt64(&h.lastPongAt)
	if nano == 0 {
		return time.Time{}
	}
	return time.Unix(0, nano)
}

// GetLastPingTime returns the timestamp of the last sent ping
func (h *HeartbeatManager) GetLastPingTime() time.Time {
	nano := atomic.LoadInt64(&h.lastPingAt)
	if nano == 0 {
		return time.Time{}
	}
	return time.Unix(0, nano)
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
