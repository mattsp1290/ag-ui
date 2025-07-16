package websocket

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/golang-lru/v2"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/ag-ui/go-sdk/pkg/core"
)

// ConnectionState represents the current state of a WebSocket connection
type ConnectionState int32

const (
	// StateDisconnected indicates the connection is not active
	StateDisconnected ConnectionState = iota
	// StateConnecting indicates connection is being established
	StateConnecting
	// StateConnected indicates connection is active and healthy
	StateConnected
	// StateReconnecting indicates connection is being re-established
	StateReconnecting
	// StateClosing indicates connection is being closed
	StateClosing
	// StateClosed indicates connection is permanently closed
	StateClosed
)

// String returns the string representation of the connection state
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	case StateClosing:
		return "closing"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// ConnectionConfig contains configuration for WebSocket connections
type ConnectionConfig struct {
	// URL is the WebSocket server URL
	URL string

	// MaxReconnectAttempts is the maximum number of reconnection attempts
	// Set to 0 for unlimited retries
	MaxReconnectAttempts int

	// InitialReconnectDelay is the initial delay between reconnection attempts
	InitialReconnectDelay time.Duration

	// MaxReconnectDelay is the maximum delay between reconnection attempts
	MaxReconnectDelay time.Duration

	// ReconnectBackoffMultiplier is the multiplier for exponential backoff
	ReconnectBackoffMultiplier float64

	// DialTimeout is the timeout for establishing the connection
	DialTimeout time.Duration

	// HandshakeTimeout is the timeout for the WebSocket handshake
	HandshakeTimeout time.Duration

	// ReadTimeout is the timeout for reading messages
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for writing messages
	WriteTimeout time.Duration

	// PingPeriod is the period between ping messages
	PingPeriod time.Duration

	// PongWait is the timeout for receiving pong messages
	PongWait time.Duration

	// MaxMessageSize is the maximum size of messages
	MaxMessageSize int64

	// WriteBufferSize is the size of the write buffer
	WriteBufferSize int

	// ReadBufferSize is the size of the read buffer
	ReadBufferSize int

	// EnableCompression enables message compression
	EnableCompression bool

	// Headers are additional headers to send during handshake
	Headers map[string]string

	// RateLimiter limits the rate of outgoing messages
	RateLimiter *rate.Limiter

	// Logger is the logger instance
	Logger *zap.Logger
}

// DefaultConnectionConfig returns a default configuration for WebSocket connections
func DefaultConnectionConfig() *ConnectionConfig {
	return &ConnectionConfig{
		MaxReconnectAttempts:       10,
		InitialReconnectDelay:      1 * time.Second,
		MaxReconnectDelay:          30 * time.Second,
		ReconnectBackoffMultiplier: 2.0,
		DialTimeout:                30 * time.Second,
		HandshakeTimeout:           10 * time.Second,
		ReadTimeout:                60 * time.Second,
		WriteTimeout:               10 * time.Second,
		PingPeriod:                 30 * time.Second,
		PongWait:                   35 * time.Second,
		MaxMessageSize:             1024 * 1024, // 1MB
		WriteBufferSize:            4096,
		ReadBufferSize:             4096,
		EnableCompression:          true,
		Headers:                    make(map[string]string),
		RateLimiter:                rate.NewLimiter(rate.Every(10*time.Millisecond), 10),
		Logger:                     zap.NewNop(),
	}
}

// Connection represents a managed WebSocket connection
type Connection struct {
	// Configuration
	config *ConnectionConfig

	// Connection state
	state        int32 // atomic access with ConnectionState
	conn         *websocket.Conn
	connMutex    sync.RWMutex
	url          *url.URL
	reconnectCh  chan struct{}
	closeCh      chan struct{}
	errorCh      chan error
	messageCh    chan []byte
	writeBacklog *lru.Cache[string, []byte]

	// Reconnection state
	reconnectAttempts int32
	lastConnected     time.Time
	lastError         error
	errorMutex        sync.RWMutex

	// Heartbeat management
	heartbeat *HeartbeatManager

	// Goroutine management
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once

	// Metrics
	metrics *ConnectionMetrics

	// Handlers
	onConnect     func()
	onDisconnect  func(error)
	onMessage     func([]byte)
	onError       func(error)
	handlersMutex sync.RWMutex
}

// ConnectionMetrics tracks connection statistics
type ConnectionMetrics struct {
	ConnectAttempts    int64
	SuccessfulConnects int64
	Disconnects        int64
	ReconnectAttempts  int64
	MessagesReceived   int64
	MessagesSent       int64
	BytesReceived      int64
	BytesSent          int64
	Errors             int64
	LastConnected      time.Time
	LastDisconnected   time.Time
	mutex              sync.RWMutex
}

// NewConnection creates a new managed WebSocket connection
func NewConnection(config *ConnectionConfig) (*Connection, error) {
	if config == nil {
		config = DefaultConnectionConfig()
	}

	if config.URL == "" {
		return nil, &core.ConfigError{
			Field: "URL",
			Value: config.URL,
			Err:   errors.New("WebSocket URL cannot be empty"),
		}
	}

	parsedURL, err := url.Parse(config.URL)
	if err != nil {
		return nil, &core.ConfigError{
			Field: "URL",
			Value: config.URL,
			Err:   fmt.Errorf("invalid WebSocket URL: %w", err),
		}
	}

	if parsedURL.Scheme != "ws" && parsedURL.Scheme != "wss" {
		return nil, &core.ConfigError{
			Field: "URL",
			Value: config.URL,
			Err:   errors.New("URL scheme must be 'ws' or 'wss'"),
		}
	}

	// Initialize write backlog cache
	writeBacklog, err := lru.New[string, []byte](100)
	if err != nil {
		return nil, fmt.Errorf("failed to create write backlog cache: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	conn := &Connection{
		config:       config,
		state:        int32(StateDisconnected),
		url:          parsedURL,
		reconnectCh:  make(chan struct{}, 1),
		closeCh:      make(chan struct{}),
		errorCh:      make(chan error, 10),
		messageCh:    make(chan []byte, 100),
		writeBacklog: writeBacklog,
		ctx:          ctx,
		cancel:       cancel,
		metrics:      &ConnectionMetrics{},
	}

	// Initialize heartbeat manager
	conn.heartbeat = NewHeartbeatManager(conn, config.PingPeriod, config.PongWait)

	return conn, nil
}

// Connect establishes a WebSocket connection
func (c *Connection) Connect(ctx context.Context) error {
	if !c.setState(StateConnecting) {
		return errors.New("connection is not in a state that allows connecting")
	}

	c.metrics.mutex.Lock()
	c.metrics.ConnectAttempts++
	c.metrics.mutex.Unlock()

	// Create dialer with configuration
	dialer := websocket.Dialer{
		HandshakeTimeout:  c.config.HandshakeTimeout,
		ReadBufferSize:    c.config.ReadBufferSize,
		WriteBufferSize:   c.config.WriteBufferSize,
		EnableCompression: c.config.EnableCompression,
	}

	// Create a context with dial timeout
	dialCtx := ctx
	if c.config.DialTimeout > 0 {
		var cancel context.CancelFunc
		dialCtx, cancel = context.WithTimeout(ctx, c.config.DialTimeout)
		defer cancel()
	}

	// Connect to WebSocket
	conn, _, err := dialer.DialContext(dialCtx, c.url.String(), c.getHeaders())
	if err != nil {
		c.setState(StateDisconnected)
		c.setError(fmt.Errorf("failed to connect to WebSocket: %w", err))
		return err
	}

	// Configure connection
	conn.SetReadLimit(c.config.MaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))

	// Set up pong handler
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
		c.heartbeat.OnPong()
		return nil
	})

	// Set up close handler
	conn.SetCloseHandler(func(code int, text string) error {
		c.config.Logger.Info("WebSocket connection closed by remote",
			zap.Int("code", code),
			zap.String("text", text))
		return nil
	})

	// Update connection state
	c.connMutex.Lock()
	c.conn = conn
	c.connMutex.Unlock()

	c.setState(StateConnected)
	c.lastConnected = time.Now()
	atomic.StoreInt32(&c.reconnectAttempts, 0)

	c.metrics.mutex.Lock()
	c.metrics.SuccessfulConnects++
	c.metrics.LastConnected = time.Now()
	c.metrics.mutex.Unlock()

	// Start connection goroutines
	c.wg.Add(2)
	go c.readPump()
	go c.writePump()

	// Note: Heartbeat is started manually via heartbeat.Start()

	// Call connect handler
	c.handlersMutex.RLock()
	onConnect := c.onConnect
	c.handlersMutex.RUnlock()

	if onConnect != nil {
		onConnect()
	}

	c.config.Logger.Info("WebSocket connection established",
		zap.String("url", c.url.String()),
		zap.String("state", c.State().String()))

	return nil
}

// Disconnect closes the WebSocket connection
func (c *Connection) Disconnect() error {
	return c.disconnect(nil)
}

// disconnect closes the connection with an optional error
func (c *Connection) disconnect(err error) error {
	if !c.setState(StateClosing) {
		return nil // Already closing or closed
	}

	c.config.Logger.Info("Disconnecting WebSocket connection",
		zap.String("url", c.url.String()),
		zap.Error(err))

	// Close the WebSocket connection
	c.connMutex.Lock()
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.config.Logger.Warn("Error closing WebSocket connection", zap.Error(err))
		}
		c.conn = nil
	}
	c.connMutex.Unlock()

	// Update metrics
	c.metrics.mutex.Lock()
	c.metrics.Disconnects++
	c.metrics.LastDisconnected = time.Now()
	c.metrics.mutex.Unlock()

	// Call disconnect handler
	c.handlersMutex.RLock()
	onDisconnect := c.onDisconnect
	c.handlersMutex.RUnlock()

	if onDisconnect != nil {
		onDisconnect(err)
	}

	c.setState(StateDisconnected)
	return nil
}

// Close permanently closes the connection and stops all goroutines
func (c *Connection) Close() error {
	c.stopOnce.Do(func() {
		c.config.Logger.Info("Closing WebSocket connection permanently")

		// Stop heartbeat first
		c.heartbeat.Stop()

		c.disconnect(nil)
		c.setState(StateClosed)

		// Cancel context to stop all goroutines
		c.cancel()

		// Close channels
		close(c.closeCh)
		close(c.reconnectCh)

		// Wait for goroutines to finish
		c.wg.Wait()

		c.config.Logger.Info("WebSocket connection closed")
	})

	return nil
}

// SendMessage sends a message through the WebSocket connection
func (c *Connection) SendMessage(ctx context.Context, message []byte) error {
	if c.State() != StateConnected {
		return errors.New("connection is not connected")
	}

	// Apply rate limiting
	if c.config.RateLimiter != nil {
		if err := c.config.RateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit exceeded: %w", err)
		}
	}

	// Send message through message channel
	select {
	case c.messageCh <- message:
		// Update metrics when message is accepted for sending
		c.metrics.mutex.Lock()
		c.metrics.MessagesSent++
		c.metrics.BytesSent += int64(len(message))
		c.metrics.mutex.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

// State returns the current connection state
func (c *Connection) State() ConnectionState {
	return ConnectionState(atomic.LoadInt32(&c.state))
}

// setState atomically sets the connection state
func (c *Connection) setState(state ConnectionState) bool {
	oldState := ConnectionState(atomic.LoadInt32(&c.state))

	// Check if state transition is valid
	if !c.isValidStateTransition(oldState, state) {
		return false
	}

	atomic.StoreInt32(&c.state, int32(state))

	c.config.Logger.Debug("Connection state changed",
		zap.String("from", oldState.String()),
		zap.String("to", state.String()))

	return true
}

// isValidStateTransition checks if a state transition is valid
func (c *Connection) isValidStateTransition(from, to ConnectionState) bool {
	switch from {
	case StateDisconnected:
		return to == StateConnecting || to == StateClosed
	case StateConnecting:
		return to == StateConnected || to == StateDisconnected || to == StateClosed
	case StateConnected:
		return to == StateReconnecting || to == StateClosing || to == StateClosed
	case StateReconnecting:
		return to == StateConnected || to == StateDisconnected || to == StateClosed
	case StateClosing:
		return to == StateDisconnected || to == StateClosed
	case StateClosed:
		return false // Cannot transition from closed state
	default:
		return false
	}
}

// setError sets the last error
func (c *Connection) setError(err error) {
	c.errorMutex.Lock()
	c.lastError = err
	c.errorMutex.Unlock()

	c.metrics.mutex.Lock()
	c.metrics.Errors++
	c.metrics.mutex.Unlock()

	// Call error handler
	c.handlersMutex.RLock()
	onError := c.onError
	c.handlersMutex.RUnlock()

	if onError != nil && err != nil {
		onError(err)
	}
}

// LastError returns the last error encountered
func (c *Connection) LastError() error {
	c.errorMutex.RLock()
	defer c.errorMutex.RUnlock()
	return c.lastError
}

// GetMetrics returns a copy of the connection metrics
func (c *Connection) GetMetrics() ConnectionMetrics {
	c.metrics.mutex.RLock()
	defer c.metrics.mutex.RUnlock()
	return *c.metrics
}

// SetOnConnect sets the connect handler
func (c *Connection) SetOnConnect(handler func()) {
	c.handlersMutex.Lock()
	c.onConnect = handler
	c.handlersMutex.Unlock()
}

// SetOnDisconnect sets the disconnect handler
func (c *Connection) SetOnDisconnect(handler func(error)) {
	c.handlersMutex.Lock()
	c.onDisconnect = handler
	c.handlersMutex.Unlock()
}

// SetOnMessage sets the message handler
func (c *Connection) SetOnMessage(handler func([]byte)) {
	c.handlersMutex.Lock()
	c.onMessage = handler
	c.handlersMutex.Unlock()
}

// SetOnError sets the error handler
func (c *Connection) SetOnError(handler func(error)) {
	c.handlersMutex.Lock()
	c.onError = handler
	c.handlersMutex.Unlock()
}

// getHeaders returns the headers for the WebSocket handshake
func (c *Connection) getHeaders() map[string][]string {
	headers := make(map[string][]string)
	for k, v := range c.config.Headers {
		headers[k] = []string{v}
	}
	return headers
}

// readPump handles reading messages from the WebSocket connection
func (c *Connection) readPump() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.connMutex.RLock()
			conn := c.conn
			c.connMutex.RUnlock()

			if conn == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Set read deadline
			conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))

			// Read message
			_, message, err := conn.ReadMessage()
			if err != nil {
				c.setError(fmt.Errorf("failed to read message: %w", err))

				// Check if we should reconnect
				if c.State() == StateConnected {
					c.triggerReconnect()
				}
				// Don't return immediately - let the reconnection logic handle it
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Update metrics
			c.metrics.mutex.Lock()
			c.metrics.MessagesReceived++
			c.metrics.BytesReceived += int64(len(message))
			c.metrics.mutex.Unlock()

			// Call message handler
			c.handlersMutex.RLock()
			onMessage := c.onMessage
			c.handlersMutex.RUnlock()

			if onMessage != nil {
				onMessage(message)
			}
		}
	}
}

// writePump handles writing messages to the WebSocket connection
func (c *Connection) writePump() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case message := <-c.messageCh:
			c.connMutex.RLock()
			conn := c.conn
			c.connMutex.RUnlock()

			if conn == nil {
				continue
			}

			// Set write deadline
			conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))

			// Write message
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.setError(fmt.Errorf("failed to write message: %w", err))

				// Check if we should reconnect
				if c.State() == StateConnected {
					c.triggerReconnect()
				}
				// Don't return immediately - let the reconnection logic handle it
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Message was successfully written (metrics already updated in SendMessage)
		}
	}
}

// triggerReconnect triggers a reconnection attempt
func (c *Connection) triggerReconnect() {
	if c.State() == StateConnected {
		c.setState(StateReconnecting)

		select {
		case c.reconnectCh <- struct{}{}:
		default:
		}
	}
}

// StartAutoReconnect starts automatic reconnection handling
func (c *Connection) StartAutoReconnect(ctx context.Context) {
	c.wg.Add(1)
	go c.autoReconnectLoop(ctx)
}

// autoReconnectLoop handles automatic reconnection
func (c *Connection) autoReconnectLoop(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.ctx.Done():
			return
		case <-c.reconnectCh:
			c.performReconnect(ctx)
		}
	}
}

// performReconnect performs the actual reconnection logic
func (c *Connection) performReconnect(ctx context.Context) {
	attempts := atomic.LoadInt32(&c.reconnectAttempts)

	// Check if we've exceeded max attempts
	if c.config.MaxReconnectAttempts > 0 && int(attempts) >= c.config.MaxReconnectAttempts {
		c.config.Logger.Error("Max reconnection attempts exceeded",
			zap.Int32("attempts", attempts),
			zap.Int("max", c.config.MaxReconnectAttempts))
		c.setState(StateDisconnected)
		return
	}

	// Close existing connection
	c.disconnect(nil)

	// Calculate backoff delay
	delay := c.calculateBackoffDelay(int(attempts))

	c.config.Logger.Info("Attempting to reconnect",
		zap.Int32("attempt", attempts+1),
		zap.Duration("delay", delay))

	// Wait for backoff period
	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return
	case <-c.ctx.Done():
		return
	}

	// Increment attempt counter
	atomic.AddInt32(&c.reconnectAttempts, 1)

	c.metrics.mutex.Lock()
	c.metrics.ReconnectAttempts++
	c.metrics.mutex.Unlock()

	// Attempt to reconnect
	if err := c.Connect(ctx); err != nil {
		c.config.Logger.Error("Reconnection failed",
			zap.Error(err),
			zap.Int32("attempt", attempts+1))

		// Trigger another reconnection attempt
		c.triggerReconnect()
	} else {
		c.config.Logger.Info("Successfully reconnected",
			zap.Int32("after_attempts", attempts+1))
	}
}

// calculateBackoffDelay calculates the exponential backoff delay
func (c *Connection) calculateBackoffDelay(attempts int) time.Duration {
	if attempts == 0 {
		return c.config.InitialReconnectDelay
	}

	// Calculate exponential backoff: base * multiplier^attempts
	base := float64(c.config.InitialReconnectDelay)
	multiplier := c.config.ReconnectBackoffMultiplier

	delay := base
	for i := 0; i < attempts; i++ {
		delay *= multiplier
	}

	// Cap at maximum delay
	if delay > float64(c.config.MaxReconnectDelay) {
		delay = float64(c.config.MaxReconnectDelay)
	}

	return time.Duration(delay)
}

// IsConnected returns true if the connection is currently connected
func (c *Connection) IsConnected() bool {
	return c.State() == StateConnected
}

// IsReconnecting returns true if the connection is currently reconnecting
func (c *Connection) IsReconnecting() bool {
	return c.State() == StateReconnecting
}

// GetURL returns the WebSocket URL
func (c *Connection) GetURL() string {
	return c.url.String()
}

// GetLastConnected returns the timestamp of the last successful connection
func (c *Connection) GetLastConnected() time.Time {
	return c.lastConnected
}

// GetReconnectAttempts returns the current number of reconnection attempts
func (c *Connection) GetReconnectAttempts() int32 {
	return atomic.LoadInt32(&c.reconnectAttempts)
}

// GetHeartbeat returns the heartbeat manager for this connection
func (c *Connection) GetHeartbeat() *HeartbeatManager {
	return c.heartbeat
}
