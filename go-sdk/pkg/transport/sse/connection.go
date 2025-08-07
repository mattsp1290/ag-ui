package sse

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	mathrand "math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/messages"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/transport/common"
)

// ConnectionState represents the current state of a connection
type ConnectionState int32

const (
	// ConnectionStateDisconnected represents a disconnected state
	ConnectionStateDisconnected ConnectionState = iota
	// ConnectionStateConnecting represents a connecting state
	ConnectionStateConnecting
	// ConnectionStateConnected represents a connected state
	ConnectionStateConnected
	// ConnectionStateReconnecting represents a reconnecting state
	ConnectionStateReconnecting
	// ConnectionStateError represents an error state
	ConnectionStateError
	// ConnectionStateClosed represents a permanently closed state
	ConnectionStateClosed
)

// String returns the string representation of the connection state
func (s ConnectionState) String() string {
	switch s {
	case ConnectionStateDisconnected:
		return "disconnected"
	case ConnectionStateConnecting:
		return "connecting"
	case ConnectionStateConnected:
		return "connected"
	case ConnectionStateReconnecting:
		return "reconnecting"
	case ConnectionStateError:
		return "error"
	case ConnectionStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// ReconnectionPolicy defines the reconnection behavior
type ReconnectionPolicy struct {
	// Enabled indicates if reconnection is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// MaxAttempts is the maximum number of reconnection attempts (0 = unlimited)
	MaxAttempts int `json:"max_attempts" yaml:"max_attempts"`

	// InitialDelay is the initial delay before first reconnection attempt
	InitialDelay time.Duration `json:"initial_delay" yaml:"initial_delay"`

	// MaxDelay is the maximum delay between reconnection attempts
	MaxDelay time.Duration `json:"max_delay" yaml:"max_delay"`

	// BackoffMultiplier for exponential backoff (default: 2.0)
	BackoffMultiplier float64 `json:"backoff_multiplier" yaml:"backoff_multiplier"`

	// JitterFactor adds randomness to delays (0.0 to 1.0, default: 0.1)
	JitterFactor float64 `json:"jitter_factor" yaml:"jitter_factor"`

	// ResetInterval resets retry count after successful connection
	ResetInterval time.Duration `json:"reset_interval" yaml:"reset_interval"`
}

// DefaultReconnectionPolicy returns a default reconnection policy
func DefaultReconnectionPolicy() *ReconnectionPolicy {
	return &ReconnectionPolicy{
		Enabled:           true,
		MaxAttempts:       10,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		JitterFactor:      0.1,
		ResetInterval:     5 * time.Minute,
	}
}

// HeartbeatConfig defines heartbeat/keepalive configuration
type HeartbeatConfig struct {
	// Enabled indicates if heartbeat is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Interval between heartbeat checks
	Interval time.Duration `json:"interval" yaml:"interval"`

	// Timeout for heartbeat response
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// MaxMissed consecutive heartbeats before considering connection dead
	MaxMissed int `json:"max_missed" yaml:"max_missed"`

	// PingEndpoint is the endpoint to ping for heartbeat
	PingEndpoint string `json:"ping_endpoint" yaml:"ping_endpoint"`
}

// DefaultHeartbeatConfig returns a default heartbeat configuration
func DefaultHeartbeatConfig() *HeartbeatConfig {
	return &HeartbeatConfig{
		Enabled:      true,
		Interval:     30 * time.Second,
		Timeout:      15 * time.Second, // Increased timeout for better reliability
		MaxMissed:    5,                // Allow more missed heartbeats
		PingEndpoint: "/ping",
	}
}

// ConnectionMetrics tracks connection-related metrics
type ConnectionMetrics struct {
	// Connection lifecycle metrics
	ConnectAttempts  *events.AtomicCounter `json:"connect_attempts"`
	ConnectSuccesses *events.AtomicCounter `json:"connect_successes"`
	ConnectFailures  *events.AtomicCounter `json:"connect_failures"`

	// Reconnection metrics
	ReconnectAttempts  *events.AtomicCounter `json:"reconnect_attempts"`
	ReconnectSuccesses *events.AtomicCounter `json:"reconnect_successes"`
	ReconnectFailures  *events.AtomicCounter `json:"reconnect_failures"`

	// Duration metrics
	ConnectDurations *events.AtomicDuration `json:"connect_durations"`
	ConnectionUptime *events.AtomicDuration `json:"connection_uptime"`

	// Heartbeat metrics
	HeartbeatsSent    *events.AtomicCounter `json:"heartbeats_sent"`
	HeartbeatsSuccess *events.AtomicCounter `json:"heartbeats_success"`
	HeartbeatsFailed  *events.AtomicCounter `json:"heartbeats_failed"`

	// Network metrics
	BytesReceived  *events.AtomicCounter `json:"bytes_received"`
	BytesSent      *events.AtomicCounter `json:"bytes_sent"`
	EventsReceived *events.AtomicCounter `json:"events_received"`
	EventsSent     *events.AtomicCounter `json:"events_sent"`

	// Error metrics
	NetworkErrors  *events.AtomicCounter `json:"network_errors"`
	TimeoutErrors  *events.AtomicCounter `json:"timeout_errors"`
	ProtocolErrors *events.AtomicCounter `json:"protocol_errors"`

	// Timestamps
	lastConnectTime    int64 // Unix nanoseconds, atomic
	lastDisconnectTime int64 // Unix nanoseconds, atomic
	lastHeartbeatTime  int64 // Unix nanoseconds, atomic
}

// NewConnectionMetrics creates a new connection metrics instance
func NewConnectionMetrics() *ConnectionMetrics {
	return &ConnectionMetrics{
		ConnectAttempts:    events.NewAtomicCounter(),
		ConnectSuccesses:   events.NewAtomicCounter(),
		ConnectFailures:    events.NewAtomicCounter(),
		ReconnectAttempts:  events.NewAtomicCounter(),
		ReconnectSuccesses: events.NewAtomicCounter(),
		ReconnectFailures:  events.NewAtomicCounter(),
		ConnectDurations:   events.NewAtomicDuration(),
		ConnectionUptime:   events.NewAtomicDuration(),
		HeartbeatsSent:     events.NewAtomicCounter(),
		HeartbeatsSuccess:  events.NewAtomicCounter(),
		HeartbeatsFailed:   events.NewAtomicCounter(),
		BytesReceived:      events.NewAtomicCounter(),
		BytesSent:          events.NewAtomicCounter(),
		EventsReceived:     events.NewAtomicCounter(),
		EventsSent:         events.NewAtomicCounter(),
		NetworkErrors:      events.NewAtomicCounter(),
		TimeoutErrors:      events.NewAtomicCounter(),
		ProtocolErrors:     events.NewAtomicCounter(),
	}
}

// RecordConnectAttempt records a connection attempt
func (m *ConnectionMetrics) RecordConnectAttempt() {
	m.ConnectAttempts.Inc()
}

// RecordConnectSuccess records a successful connection
func (m *ConnectionMetrics) RecordConnectSuccess(duration time.Duration) {
	m.ConnectSuccesses.Inc()
	m.ConnectDurations.Add(duration)
	atomic.StoreInt64(&m.lastConnectTime, time.Now().UnixNano())
}

// RecordConnectFailure records a failed connection
func (m *ConnectionMetrics) RecordConnectFailure() {
	m.ConnectFailures.Inc()
}

// RecordDisconnect records a disconnection
func (m *ConnectionMetrics) RecordDisconnect() {
	atomic.StoreInt64(&m.lastDisconnectTime, time.Now().UnixNano())
}

// RecordHeartbeat records heartbeat metrics
func (m *ConnectionMetrics) RecordHeartbeat(success bool) {
	m.HeartbeatsSent.Inc()
	if success {
		m.HeartbeatsSuccess.Inc()
	} else {
		m.HeartbeatsFailed.Inc()
	}
	atomic.StoreInt64(&m.lastHeartbeatTime, time.Now().UnixNano())
}

// GetLastConnectTime returns the last connection time
func (m *ConnectionMetrics) GetLastConnectTime() time.Time {
	nanos := atomic.LoadInt64(&m.lastConnectTime)
	if nanos == 0 {
		return time.Time{}
	}
	return time.Unix(0, nanos)
}

// GetLastDisconnectTime returns the last disconnection time
func (m *ConnectionMetrics) GetLastDisconnectTime() time.Time {
	nanos := atomic.LoadInt64(&m.lastDisconnectTime)
	if nanos == 0 {
		return time.Time{}
	}
	return time.Unix(0, nanos)
}

// GetLastHeartbeatTime returns the last heartbeat time
func (m *ConnectionMetrics) GetLastHeartbeatTime() time.Time {
	nanos := atomic.LoadInt64(&m.lastHeartbeatTime)
	if nanos == 0 {
		return time.Time{}
	}
	return time.Unix(0, nanos)
}

// GetConnectSuccessRate returns the connection success rate as a percentage
func (m *ConnectionMetrics) GetConnectSuccessRate() float64 {
	attempts := m.ConnectAttempts.Load()
	if attempts == 0 {
		return 0
	}
	successes := m.ConnectSuccesses.Load()
	return float64(successes) / float64(attempts) * 100.0
}

// GetHeartbeatSuccessRate returns the heartbeat success rate as a percentage
func (m *ConnectionMetrics) GetHeartbeatSuccessRate() float64 {
	sent := m.HeartbeatsSent.Load()
	if sent == 0 {
		return 0
	}
	success := m.HeartbeatsSuccess.Load()
	return float64(success) / float64(sent) * 100.0
}

// Connection represents a managed SSE connection
type Connection struct {
	// Configuration
	config          *Config
	reconnectPolicy *ReconnectionPolicy
	heartbeatConfig *HeartbeatConfig

	// Connection state
	state int32 // ConnectionState, atomic
	id    string

	// HTTP connection
	httpConn   *http.Response
	httpClient *http.Client
	connMutex  sync.RWMutex

	// Reconnection management
	reconnectAttempt int32 // atomic
	lastConnectTime  int64 // Unix nanoseconds, atomic

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup // Track goroutines for proper cleanup

	// Channels
	eventChan chan events.Event
	errorChan chan error
	stateChan chan ConnectionState

	// Heartbeat management
	heartbeatTicker  *time.Ticker
	heartbeatMutex   sync.RWMutex // Protects heartbeatTicker access
	missedHeartbeats int32        // atomic

	// Metrics
	metrics *ConnectionMetrics

	// Cleanup coordination
	cleanupOnce    sync.Once
	channelsClosed int32 // atomic flag to indicate all channels are closed

	// Pool reference (if part of pool)
	pool *ConnectionPool
}

// NewConnection creates a new managed connection
func NewConnection(config *Config, pool *ConnectionPool) (*Connection, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required: %w", common.NewValidationError("config", "required", "config must be provided", nil))
	}

	// Generate unique connection ID
	id, err := generateConnectionID()
	if err != nil {
		return nil, messages.NewConnectionError("failed to generate connection ID", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	conn := &Connection{
		config:          config,
		reconnectPolicy: DefaultReconnectionPolicy(),
		heartbeatConfig: DefaultHeartbeatConfig(),
		state:           int32(ConnectionStateDisconnected),
		id:              id,
		httpClient:      config.Client,
		ctx:             ctx,
		cancel:          cancel,
		eventChan:       make(chan events.Event, config.BufferSize),
		errorChan:       make(chan error, 10),
		stateChan:       make(chan ConnectionState, 10),
		metrics:         NewConnectionMetrics(),
		pool:            pool,
	}

	// Apply configuration overrides for simple Config
	conn.reconnectPolicy.MaxAttempts = config.MaxReconnects
	conn.reconnectPolicy.InitialDelay = config.ReconnectDelay

	return conn, nil
}

// generateConnectionID generates a unique connection ID
func generateConnectionID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("conn_%x", bytes), nil
}

// ID returns the connection ID
func (c *Connection) ID() string {
	return c.id
}

// State returns the current connection state
func (c *Connection) State() ConnectionState {
	return ConnectionState(atomic.LoadInt32(&c.state))
}

// setState atomically sets the connection state and notifies listeners
func (c *Connection) setState(newState ConnectionState) {
	oldState := ConnectionState(atomic.SwapInt32(&c.state, int32(newState)))

	if oldState != newState {
		// Notify state change (non-blocking)
		select {
		case c.stateChan <- newState:
		default:
		}
	}
}

// Connect establishes the SSE connection
func (c *Connection) Connect(ctx context.Context) error {
	if c.State() == ConnectionStateClosed {
		return messages.NewConnectionError("connection is permanently closed", nil)
	}

	c.setState(ConnectionStateConnecting)
	c.metrics.RecordConnectAttempt()

	startTime := time.Now()

	// Create SSE request
	url := c.config.BaseURL + "/events/stream"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		c.setState(ConnectionStateError)
		c.metrics.RecordConnectFailure()
		return messages.NewConnectionError("failed to create SSE request", err)
	}

	// Set headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Add custom headers
	for key, value := range c.config.Headers {
		req.Header.Set(key, value)
	}

	// Apply authentication
	if err := c.applyAuthentication(req); err != nil {
		c.setState(ConnectionStateError)
		c.metrics.RecordConnectFailure()
		return messages.NewConnectionError("failed to apply authentication", err)
	}

	// Apply timeout (use client timeout from Config)
	if c.config.Client != nil && c.config.Client.Timeout > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, c.config.Client.Timeout)
		defer cancel()
		req = req.WithContext(timeoutCtx)
	}

	// Perform request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.setState(ConnectionStateError)
		c.metrics.RecordConnectFailure()
		c.metrics.NetworkErrors.Inc()
		return messages.NewConnectionError("failed to connect to SSE endpoint", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		c.setState(ConnectionStateError)
		c.metrics.RecordConnectFailure()
		return messages.NewConnectionError(
			fmt.Sprintf("SSE connection failed with status %d", resp.StatusCode), nil)
	}

	// Verify content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/event-stream" && contentType != "text/event-stream; charset=utf-8" {
		resp.Body.Close()
		c.setState(ConnectionStateError)
		c.metrics.RecordConnectFailure()
		c.metrics.ProtocolErrors.Inc()
		return messages.NewConnectionError(
			fmt.Sprintf("unexpected content type: %s", contentType), nil)
	}

	// Store connection
	c.connMutex.Lock()
	c.httpConn = resp
	c.connMutex.Unlock()

	// Update state and metrics
	connectDuration := time.Since(startTime)
	c.setState(ConnectionStateConnected)
	c.metrics.RecordConnectSuccess(connectDuration)
	atomic.StoreInt64(&c.lastConnectTime, time.Now().UnixNano())

	// Reset reconnection attempts on successful connection
	atomic.StoreInt32(&c.reconnectAttempt, 0)

	// Start heartbeat monitoring
	if c.heartbeatConfig.Enabled {
		c.startHeartbeat()
	}

	return nil
}

// applyAuthentication applies authentication to the request
func (c *Connection) applyAuthentication(req *http.Request) error {
	// For simple Config, authentication is handled via headers
	// The headers are already applied above
	return nil
}

// Disconnect closes the connection gracefully
func (c *Connection) Disconnect() error {
	currentState := c.State()
	if currentState == ConnectionStateDisconnected || currentState == ConnectionStateClosed {
		return nil
	}

	c.setState(ConnectionStateDisconnected)
	c.metrics.RecordDisconnect()

	// Stop heartbeat
	c.stopHeartbeat()

	// Close HTTP connection
	c.connMutex.Lock()
	if c.httpConn != nil {
		c.httpConn.Body.Close()
		c.httpConn = nil
	}
	c.connMutex.Unlock()

	return nil
}

// Close permanently closes the connection and releases resources
func (c *Connection) Close() error {
	c.cleanupOnce.Do(func() {
		c.setState(ConnectionStateClosed)

		// Cancel context to stop all goroutines immediately
		c.cancel()

		// Disconnect cleanly
		c.Disconnect()

		// Mark channels as closed to prevent new operations
		atomic.StoreInt32(&c.channelsClosed, 1)

		// Wait for all goroutines to finish
		c.wg.Wait()

		// Close channels after goroutines have stopped
		close(c.eventChan)
		close(c.errorChan)
		close(c.stateChan)

		// Remove from pool if part of one
		if c.pool != nil {
			c.pool.removeConnection(c)
		}
	})

	return nil
}

// Reconnect attempts to reconnect with exponential backoff
func (c *Connection) Reconnect(ctx context.Context) error {
	if !c.reconnectPolicy.Enabled {
		return messages.NewConnectionError("reconnection is disabled", nil)
	}

	if c.State() == ConnectionStateClosed {
		return messages.NewConnectionError("connection is permanently closed", nil)
	}

	attempt := atomic.AddInt32(&c.reconnectAttempt, 1)

	if c.reconnectPolicy.MaxAttempts > 0 && int(attempt) > c.reconnectPolicy.MaxAttempts {
		c.setState(ConnectionStateError)
		return messages.NewConnectionError(
			fmt.Sprintf("maximum reconnection attempts (%d) exceeded", c.reconnectPolicy.MaxAttempts), nil)
	}

	c.setState(ConnectionStateReconnecting)
	c.metrics.ReconnectAttempts.Inc()

	// Calculate delay with exponential backoff and jitter
	delay := c.calculateReconnectDelay(int(attempt))

	// Wait for delay or context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
	}

	// Attempt to reconnect
	if err := c.Connect(ctx); err != nil {
		c.metrics.ReconnectFailures.Inc()
		return err
	}

	c.metrics.ReconnectSuccesses.Inc()
	return nil
}

// calculateReconnectDelay calculates the delay for reconnection attempt
func (c *Connection) calculateReconnectDelay(attempt int) time.Duration {
	// Calculate exponential backoff
	delay := float64(c.reconnectPolicy.InitialDelay) * math.Pow(c.reconnectPolicy.BackoffMultiplier, float64(attempt-1))

	// Apply maximum delay limit
	if delay > float64(c.reconnectPolicy.MaxDelay) {
		delay = float64(c.reconnectPolicy.MaxDelay)
	}

	// Add jitter to prevent thundering herd
	if c.reconnectPolicy.JitterFactor > 0 {
		jitter := delay * c.reconnectPolicy.JitterFactor * (mathrand.Float64() - 0.5) * 2
		delay += jitter
	}

	// Ensure delay is not negative
	if delay < 0 {
		delay = float64(c.reconnectPolicy.InitialDelay)
	}

	return time.Duration(delay)
}

// startHeartbeat starts the heartbeat monitoring
func (c *Connection) startHeartbeat() {
	c.heartbeatMutex.Lock()
	defer c.heartbeatMutex.Unlock()

	if c.heartbeatTicker != nil {
		c.heartbeatTicker.Stop()
	}

	c.heartbeatTicker = time.NewTicker(c.heartbeatConfig.Interval)
	atomic.StoreInt32(&c.missedHeartbeats, 0)

	// Use WaitGroup to track heartbeat goroutine
	c.wg.Add(1)
	go c.heartbeatLoop()
}

// stopHeartbeat stops the heartbeat monitoring
func (c *Connection) stopHeartbeat() {
	c.heartbeatMutex.Lock()
	defer c.heartbeatMutex.Unlock()

	if c.heartbeatTicker != nil {
		c.heartbeatTicker.Stop()
		c.heartbeatTicker = nil
	}
}

// heartbeatLoop performs periodic heartbeat checks
func (c *Connection) heartbeatLoop() {
	defer c.wg.Done() // Properly signal goroutine completion

	for {
		// Get ticker channel safely
		c.heartbeatMutex.RLock()
		tickerC := c.heartbeatTicker.C
		c.heartbeatMutex.RUnlock()

		if tickerC == nil {
			// Ticker was stopped, exit
			return
		}

		select {
		case <-c.ctx.Done():
			return
		case <-tickerC:
			if c.State() != ConnectionStateConnected {
				return
			}

			if err := c.performHeartbeat(); err != nil {
				missed := atomic.AddInt32(&c.missedHeartbeats, 1)
				c.metrics.RecordHeartbeat(false)

				if int(missed) >= c.heartbeatConfig.MaxMissed {
					// Connection is considered dead
					c.setState(ConnectionStateError)
					// Non-blocking error notification
					select {
					case c.errorChan <- messages.NewConnectionError("heartbeat failed: connection appears to be dead", err):
					default:
					}
					return
				}
			} else {
				atomic.StoreInt32(&c.missedHeartbeats, 0)
				c.metrics.RecordHeartbeat(true)
			}
		}
	}
}

// performHeartbeat performs a single heartbeat check
func (c *Connection) performHeartbeat() error {
	ctx, cancel := context.WithTimeout(c.ctx, c.heartbeatConfig.Timeout)
	defer cancel()

	// Create heartbeat request
	url := c.config.BaseURL + c.heartbeatConfig.PingEndpoint
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	// Apply authentication
	if err := c.applyAuthentication(req); err != nil {
		return err
	}

	// Perform request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.metrics.NetworkErrors.Inc()
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.metrics.NetworkErrors.Inc()
		return fmt.Errorf("heartbeat failed with status %d: %w", resp.StatusCode, common.NewNetworkError("heartbeat", "http", c.config.BaseURL, nil))
	}

	return nil
}

// ReadEvents returns the event channel for reading events
func (c *Connection) ReadEvents() <-chan events.Event {
	return c.eventChan
}

// ReadErrors returns the error channel for reading errors
func (c *Connection) ReadErrors() <-chan error {
	return c.errorChan
}

// ReadStateChanges returns the state change channel
func (c *Connection) ReadStateChanges() <-chan ConnectionState {
	return c.stateChan
}

// IsConnected returns true if the connection is in connected state
func (c *Connection) IsConnected() bool {
	return c.State() == ConnectionStateConnected
}

// IsAlive returns true if the connection is not closed or in error state
func (c *Connection) IsAlive() bool {
	state := c.State()
	return state != ConnectionStateClosed && state != ConnectionStateError
}

// GetMetrics returns connection metrics
func (c *Connection) GetMetrics() *ConnectionMetrics {
	return c.metrics
}

// GetUptime returns the connection uptime
func (c *Connection) GetUptime() time.Duration {
	lastConnect := atomic.LoadInt64(&c.lastConnectTime)
	if lastConnect == 0 {
		return 0
	}

	connectTime := time.Unix(0, lastConnect)
	if c.State() == ConnectionStateConnected {
		return time.Since(connectTime)
	}

	// If disconnected, calculate uptime until last disconnect
	lastDisconnect := c.metrics.GetLastDisconnectTime()
	if !lastDisconnect.IsZero() && lastDisconnect.After(connectTime) {
		return lastDisconnect.Sub(connectTime)
	}

	return time.Since(connectTime)
}

// GetConnectionInfo returns comprehensive connection information
func (c *Connection) GetConnectionInfo() map[string]interface{} {
	return map[string]interface{}{
		"id":                     c.id,
		"state":                  c.State().String(),
		"uptime":                 c.GetUptime(),
		"reconnect_attempts":     atomic.LoadInt32(&c.reconnectAttempt),
		"missed_heartbeats":      atomic.LoadInt32(&c.missedHeartbeats),
		"last_connect_time":      c.metrics.GetLastConnectTime(),
		"last_disconnect_time":   c.metrics.GetLastDisconnectTime(),
		"last_heartbeat_time":    c.metrics.GetLastHeartbeatTime(),
		"connect_success_rate":   c.metrics.GetConnectSuccessRate(),
		"heartbeat_success_rate": c.metrics.GetHeartbeatSuccessRate(),
		"bytes_received":         c.metrics.BytesReceived.Load(),
		"bytes_sent":             c.metrics.BytesSent.Load(),
		"events_received":        c.metrics.EventsReceived.Load(),
		"events_sent":            c.metrics.EventsSent.Load(),
		"network_errors":         c.metrics.NetworkErrors.Load(),
		"timeout_errors":         c.metrics.TimeoutErrors.Load(),
		"protocol_errors":        c.metrics.ProtocolErrors.Load(),
	}
}

// ConnectionPool manages a pool of connections for load balancing and failover
type ConnectionPool struct {
	config      *Config
	connections map[string]*Connection
	mutex       sync.RWMutex

	// Pool configuration
	maxConnections int
	minConnections int
	idleTimeout    time.Duration
	maxIdleTime    time.Duration

	// Load balancing
	roundRobinIndex int64 // atomic

	// Health monitoring
	healthCheckInterval time.Duration
	healthTicker        *time.Ticker

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc

	// Metrics
	poolMetrics *PoolMetrics
}

// PoolMetrics tracks pool-level metrics
type PoolMetrics struct {
	ActiveConnections *events.AtomicCounter
	IdleConnections   *events.AtomicCounter
	TotalConnections  *events.AtomicCounter
	FailedConnections *events.AtomicCounter
	PoolUtilization   float64 // Calculated metric

	// Pool operations
	AcquireRequests  *events.AtomicCounter
	AcquireSuccesses *events.AtomicCounter
	AcquireTimeouts  *events.AtomicCounter

	mutex sync.RWMutex
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(config *Config) (*ConnectionPool, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required: %w", common.NewValidationError("config", "required", "config must be provided", nil))
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &ConnectionPool{
		config:              config,
		connections:         make(map[string]*Connection),
		maxConnections:      100, // Increased for high concurrency load testing
		minConnections:      2,
		idleTimeout:         90 * time.Second,
		maxIdleTime:         30 * time.Minute,
		healthCheckInterval: 30 * time.Second,
		ctx:                 ctx,
		cancel:              cancel,
		poolMetrics: &PoolMetrics{
			ActiveConnections: events.NewAtomicCounter(),
			IdleConnections:   events.NewAtomicCounter(),
			TotalConnections:  events.NewAtomicCounter(),
			FailedConnections: events.NewAtomicCounter(),
			AcquireRequests:   events.NewAtomicCounter(),
			AcquireSuccesses:  events.NewAtomicCounter(),
			AcquireTimeouts:   events.NewAtomicCounter(),
		},
	}

	// Start health monitoring
	pool.startHealthMonitoring()

	return pool, nil
}

// AcquireConnection acquires a connection from the pool
func (p *ConnectionPool) AcquireConnection(ctx context.Context) (*Connection, error) {
	p.poolMetrics.AcquireRequests.Inc()

	// Try to get an existing healthy connection
	if conn := p.getHealthyConnection(); conn != nil {
		p.poolMetrics.AcquireSuccesses.Inc()
		return conn, nil
	}

	// Create new connection if under limit
	p.mutex.Lock()
	currentCount := len(p.connections)
	if currentCount < p.maxConnections {
		conn, err := NewConnection(p.config, p)
		if err != nil {
			p.mutex.Unlock()
			p.poolMetrics.FailedConnections.Inc()
			return nil, err
		}

		p.connections[conn.ID()] = conn
		p.poolMetrics.TotalConnections.Inc()
		p.mutex.Unlock()

		// Connect the new connection
		if err := conn.Connect(ctx); err != nil {
			p.removeConnection(conn)
			p.poolMetrics.FailedConnections.Inc()
			return nil, err
		}

		p.poolMetrics.AcquireSuccesses.Inc()
		p.poolMetrics.ActiveConnections.Inc()
		return conn, nil
	}
	p.mutex.Unlock()

	// Pool is full, wait for available connection or timeout
	p.poolMetrics.AcquireTimeouts.Inc()
	return nil, messages.NewConnectionError("connection pool is full", nil)
}

// getHealthyConnection returns a healthy connection using round-robin
func (p *ConnectionPool) getHealthyConnection() *Connection {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if len(p.connections) == 0 {
		return nil
	}

	// Convert to slice for round-robin access
	connections := make([]*Connection, 0, len(p.connections))
	for _, conn := range p.connections {
		if conn.IsConnected() {
			connections = append(connections, conn)
		}
	}

	if len(connections) == 0 {
		return nil
	}

	// Round-robin selection
	index := atomic.AddInt64(&p.roundRobinIndex, 1) % int64(len(connections))
	return connections[index]
}

// ReleaseConnection releases a connection back to the pool
func (p *ConnectionPool) ReleaseConnection(conn *Connection) {
	if conn == nil {
		return
	}

	// Connection remains in pool but marked as idle
	p.poolMetrics.ActiveConnections.Dec()
	p.poolMetrics.IdleConnections.Inc()
}

// removeConnection removes a connection from the pool
func (p *ConnectionPool) removeConnection(conn *Connection) {
	if conn == nil {
		return
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if _, exists := p.connections[conn.ID()]; exists {
		delete(p.connections, conn.ID())
		p.poolMetrics.TotalConnections.Dec()

		// Update active/idle counts
		if conn.IsConnected() {
			p.poolMetrics.ActiveConnections.Dec()
		} else {
			p.poolMetrics.IdleConnections.Dec()
		}
	}
}

// startHealthMonitoring starts health monitoring for the pool
func (p *ConnectionPool) startHealthMonitoring() {
	p.healthTicker = time.NewTicker(p.healthCheckInterval)

	// Note: This goroutine will be cleaned up when context is cancelled
	// and healthTicker is stopped in Close()
	go func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-p.healthTicker.C:
				p.performHealthCheck()
			}
		}
	}()
}

// performHealthCheck performs health checks on all connections
func (p *ConnectionPool) performHealthCheck() {
	p.mutex.RLock()
	connections := make([]*Connection, 0, len(p.connections))
	for _, conn := range p.connections {
		connections = append(connections, conn)
	}
	p.mutex.RUnlock()

	for _, conn := range connections {
		// Remove unhealthy connections
		if !conn.IsAlive() {
			p.removeConnection(conn)
			conn.Close()
			continue
		}

		// Check for idle timeout
		if p.idleTimeout > 0 {
			uptime := conn.GetUptime()
			if uptime > p.idleTimeout && !conn.IsConnected() {
				p.removeConnection(conn)
				conn.Close()
			}
		}
	}

	// Update pool utilization
	p.updatePoolUtilization()
}

// updatePoolUtilization updates the pool utilization metric
func (p *ConnectionPool) updatePoolUtilization() {
	p.poolMetrics.mutex.Lock()
	defer p.poolMetrics.mutex.Unlock()

	total := p.poolMetrics.TotalConnections.Load()
	if total > 0 {
		active := p.poolMetrics.ActiveConnections.Load()
		p.poolMetrics.PoolUtilization = float64(active) / float64(total) * 100.0
	} else {
		p.poolMetrics.PoolUtilization = 0.0
	}
}

// GetPoolStats returns pool statistics
func (p *ConnectionPool) GetPoolStats() map[string]interface{} {
	p.poolMetrics.mutex.RLock()
	defer p.poolMetrics.mutex.RUnlock()

	return map[string]interface{}{
		"total_connections":  p.poolMetrics.TotalConnections.Load(),
		"active_connections": p.poolMetrics.ActiveConnections.Load(),
		"idle_connections":   p.poolMetrics.IdleConnections.Load(),
		"failed_connections": p.poolMetrics.FailedConnections.Load(),
		"pool_utilization":   p.poolMetrics.PoolUtilization,
		"acquire_requests":   p.poolMetrics.AcquireRequests.Load(),
		"acquire_successes":  p.poolMetrics.AcquireSuccesses.Load(),
		"acquire_timeouts":   p.poolMetrics.AcquireTimeouts.Load(),
		"max_connections":    p.maxConnections,
		"min_connections":    p.minConnections,
	}
}

// GetHealthyConnectionCount returns the number of healthy connections
func (p *ConnectionPool) GetHealthyConnectionCount() int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	count := 0
	for _, conn := range p.connections {
		if conn.IsConnected() {
			count++
		}
	}
	return count
}

// Close closes the connection pool and all connections
func (p *ConnectionPool) Close() error {
	// Cancel context to stop health monitoring
	p.cancel()

	// Stop health ticker
	if p.healthTicker != nil {
		p.healthTicker.Stop()
	}

	// Close all connections
	p.mutex.Lock()
	connections := make([]*Connection, 0, len(p.connections))
	for _, conn := range p.connections {
		connections = append(connections, conn)
	}
	p.connections = make(map[string]*Connection)
	p.mutex.Unlock()

	// Close connections in parallel
	var wg sync.WaitGroup
	for _, conn := range connections {
		wg.Add(1)
		go func(c *Connection) {
			defer wg.Done()
			c.Close()
		}(conn)
	}

	wg.Wait()
	return nil
}
