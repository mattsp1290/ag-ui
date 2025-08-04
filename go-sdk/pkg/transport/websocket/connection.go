package websocket

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/golang-lru/v2"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core"
	"github.com/mattsp1290/ag-ui/go-sdk/internal/timeconfig"
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

	// TLSClientConfig is the TLS configuration for secure WebSocket connections
	// If nil, default TLS settings are used
	TLSClientConfig *tls.Config
}

// DefaultConnectionConfig returns a default configuration for WebSocket connections
// Uses configurable timeouts that adapt to test/production environments
func DefaultConnectionConfig() *ConnectionConfig {
	config := timeconfig.GetConfig()
	return &ConnectionConfig{
		MaxReconnectAttempts:       10,
		InitialReconnectDelay:      config.DefaultInitialReconnectDelay,
		MaxReconnectDelay:          config.DefaultMaxReconnectDelay,
		ReconnectBackoffMultiplier: 2.0,
		DialTimeout:                config.DefaultDialTimeout,
		HandshakeTimeout:           config.DefaultHandshakeTimeout,
		ReadTimeout:                config.DefaultReadTimeout,
		WriteTimeout:               config.DefaultWriteTimeout,
		PingPeriod:                 config.DefaultPingPeriod,
		PongWait:                   config.DefaultPongTimeout,
		MaxMessageSize:             1024 * 1024, // 1MB
		WriteBufferSize:            4096,
		ReadBufferSize:             4096,
		EnableCompression:          true,
		Headers:                    make(map[string]string),
		RateLimiter:                NewProductionRateLimiter(),
		Logger:                     zap.NewNop(),
	}
}

// NewProductionRateLimiter creates a rate limiter suitable for production use
// Allows 100 messages per second with burst of 10
func NewProductionRateLimiter() *rate.Limiter {
	return rate.NewLimiter(rate.Every(10*time.Millisecond), 10)
}

// NewTestRateLimiter creates a rate limiter suitable for testing scenarios
// Allows 10,000 messages per second with burst of 1000 for high concurrency tests
func NewTestRateLimiter() *rate.Limiter {
	return rate.NewLimiter(rate.Limit(10000), 1000)
}

// NewUnlimitedRateLimiter creates a rate limiter with no practical limits
// Useful for load testing where rate limiting is not the focus
func NewUnlimitedRateLimiter() *rate.Limiter {
	return rate.NewLimiter(rate.Inf, 1000000)
}

// Connection represents a managed WebSocket connection
type Connection struct {
	// Configuration
	config *ConnectionConfig

	// Connection state
	state        int32 // atomic access with ConnectionState
	conn         *websocket.Conn
	connMutex    sync.RWMutex
	connGeneration int64 // atomic access - increments on each new connection
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
	
	// Connection-specific goroutine management
	connCtx      context.Context
	connCancel   context.CancelFunc
	connWg       sync.WaitGroup
	connMutexGr  sync.Mutex // protects connCtx, connCancel, connWg

	// Cleanup coordination
	cleanupOnce    sync.Once
	channelsClosed int32      // atomic flag to indicate all channels are closed
	channelMutex   sync.Mutex // mutex to synchronize channel operations

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
	return NewConnectionWithContext(context.Background(), config)
}

// NewConnectionWithContext creates a new managed WebSocket connection with a parent context
func NewConnectionWithContext(parentCtx context.Context, config *ConnectionConfig) (*Connection, error) {
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

	ctx, cancel := context.WithCancel(parentCtx)

	conn := &Connection{
		config:       config,
		state:        int32(StateDisconnected),
		url:          parsedURL,
		reconnectCh:  make(chan struct{}, 1),
		closeCh:      make(chan struct{}),
		errorCh:      make(chan error, 10),
		messageCh:    make(chan []byte, 1000), // Increased buffer for high throughput
		writeBacklog: writeBacklog,
		ctx:          ctx,
		cancel:       cancel,
		metrics:      &ConnectionMetrics{},
	}

	// Initialize heartbeat manager
	conn.heartbeat = NewHeartbeatManager(conn, config.PingPeriod, config.PongWait)

	return conn, nil
}

// stopConnectionGoroutines stops any existing connection-specific goroutines
// Implements aggressive shutdown with retry logic and comprehensive timeout handling
func (c *Connection) stopConnectionGoroutines() {
	c.connMutexGr.Lock()
	connCancel := c.connCancel
	c.connMutexGr.Unlock()
	
	if connCancel != nil {
		c.config.Logger.Debug("Stopping connection goroutines with enhanced aggressive shutdown")
		
		// Phase 1: Cancel context first to signal goroutines to exit immediately
		c.config.Logger.Debug("Cancelling connection context to signal goroutines")
		connCancel()
		
		// Phase 2: Force close WebSocket connection to unblock any I/O operations
		c.connMutex.Lock()
		if c.conn != nil {
			c.config.Logger.Debug("Force closing WebSocket connection to unblock I/O")
			// Use force close to interrupt any blocked operations
			if err := forceCloseConnection(c.conn); err != nil {
				c.config.Logger.Debug("Error force closing connection", zap.Error(err))
			}
		}
		c.connMutex.Unlock()
		
		// Phase 3: Wait for connection-specific goroutines with enhanced timeout and retry logic
		maxRetries := 3
		baseTimeout := 100 * time.Millisecond
		
		for retry := 0; retry < maxRetries; retry++ {
			timeout := time.Duration(retry+1) * baseTimeout // Progressive timeout: 100ms, 200ms, 300ms
			
			done := make(chan struct{})
			go func() {
				c.connWg.Wait()
				close(done)
			}()
			
			select {
			case <-done:
				c.config.Logger.Debug("Connection goroutines stopped cleanly", 
					zap.Int("retry", retry), 
					zap.Duration("timeout", timeout))
				return // Success - exit immediately
			case <-time.After(timeout):
				c.config.Logger.Debug("Connection goroutines did not stop within timeout", 
					zap.Int("retry", retry), 
					zap.Duration("timeout", timeout))
				
				// On final retry, give up gracefully
				if retry == maxRetries-1 {
					c.config.Logger.Debug("Final retry timeout - proceeding with cleanup")
					return
				}
				
				// Intermediate retry - force additional cleanup
				c.config.Logger.Debug("Forcing additional cleanup before retry",
					zap.Int("retry", retry))
				
				// Additional forced cleanup measures
				c.connMutex.Lock()
				if c.conn != nil {
					// Set immediate deadlines to force closure
					now := time.Now()
					c.conn.SetReadDeadline(now)
					c.conn.SetWriteDeadline(now)
					c.conn.Close() // Force close again
				}
				c.connMutex.Unlock()
				
				// Brief pause before retry
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
}

// Connect establishes a WebSocket connection
func (c *Connection) Connect(ctx context.Context) error {
	// Atomically transition from Disconnected or Reconnecting to Connecting
	// This prevents multiple concurrent Connect calls while allowing reconnection
	_, success := c.trySetStateFromMultiple([]ConnectionState{StateDisconnected, StateReconnecting}, StateConnecting)
	if !success {
		return errors.New("connection is not in a state that allows connecting")
	}
	
	// Stop any existing connection goroutines before starting new ones
	c.stopConnectionGoroutines()

	c.metrics.mutex.Lock()
	c.metrics.ConnectAttempts++
	c.metrics.mutex.Unlock()

	// Create dialer with configuration
	dialer := websocket.Dialer{
		HandshakeTimeout:  c.config.HandshakeTimeout,
		ReadBufferSize:    c.config.ReadBufferSize,
		WriteBufferSize:   c.config.WriteBufferSize,
		EnableCompression: c.config.EnableCompression,
		TLSClientConfig:   c.config.TLSClientConfig,
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
	conn.SetPongHandler(func(data string) error {
		c.config.Logger.Debug("Client received pong", zap.String("data", data))
		conn.SetReadDeadline(time.Now().Add(c.config.PongWait))
		c.heartbeat.OnPong()
		
		// If we're in reconnecting state and received a pong, we can recover
		if c.State() == StateReconnecting && c.heartbeat.IsHealthy() {
			c.config.Logger.Info("Connection recovered from reconnecting state")
			c.setState(StateConnected)
		}
		
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
	// Increment connection generation to invalidate stale references
	atomic.AddInt64(&c.connGeneration, 1)
	c.connMutex.Unlock()

	c.setState(StateConnected)
	c.lastConnected = time.Now()
	atomic.StoreInt32(&c.reconnectAttempts, 0)

	c.metrics.mutex.Lock()
	c.metrics.SuccessfulConnects++
	c.metrics.LastConnected = time.Now()
	c.metrics.mutex.Unlock()

	// Create new connection-specific context for this connection's goroutines
	c.connMutexGr.Lock()
	c.connCtx, c.connCancel = context.WithCancel(c.ctx)
	c.connMutexGr.Unlock()

	// Start connection goroutines with proper error handling
	c.wg.Add(2)
	c.connWg.Add(2)
	
	// Start readPump with proper cleanup tracking
	go func() {
		defer func() {
			if r := recover(); r != nil {
				c.config.Logger.Error("ReadPump panic recovered", zap.Any("panic", r))
			}
		}()
		c.readPump()
	}()
	
	// Start writePump with proper cleanup tracking
	go func() {
		defer func() {
			if r := recover(); r != nil {
				c.config.Logger.Error("WritePump panic recovered", zap.Any("panic", r))
			}
		}()
		c.writePump()
	}()

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
	// Atomically transition to Closing state from any valid state
	// This prevents multiple concurrent disconnect calls
	prevState, success := c.trySetStateFromMultiple([]ConnectionState{StateConnected, StateConnecting, StateReconnecting, StateDisconnected}, StateClosing)
	if !success {
		// Already closing or closed
		c.config.Logger.Debug("Disconnect called but connection already closing/closed",
			zap.String("current_state", prevState.String()))
		
		// If we were in StateReconnecting and trying to disconnect, we should still call the disconnect handler
		// This fixes the issue where disconnect handler wasn't called during state transitions
		if prevState == StateReconnecting && err != nil {
			c.handlersMutex.RLock()
			onDisconnect := c.onDisconnect
			c.handlersMutex.RUnlock()
			
			if onDisconnect != nil {
				c.config.Logger.Debug("Calling disconnect handler for reconnecting state")
				onDisconnect(err)
			}
		}
		return nil
	}

	c.config.Logger.Info("Disconnecting WebSocket connection",
		zap.String("url", c.url.String()),
		zap.Error(err))

	// Stop connection-specific goroutines first
	c.stopConnectionGoroutines()

	// Close the WebSocket connection
	c.connMutex.Lock()
	if c.conn != nil {
		// Force close to interrupt any blocked I/O operations
		if err := forceCloseConnection(c.conn); err != nil {
			c.config.Logger.Warn("Error closing WebSocket connection", zap.Error(err))
		}
		c.conn = nil
		// Increment generation to invalidate any outstanding references
		atomic.AddInt64(&c.connGeneration, 1)
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

// disconnectForReconnect closes the physical connection but keeps the state as StateReconnecting
// This is used during reconnection to avoid transitioning to StateDisconnected
func (c *Connection) disconnectForReconnect() {
	c.config.Logger.Info("Disconnecting WebSocket connection for reconnection",
		zap.String("url", c.url.String()))

	// Stop connection-specific goroutines first
	c.stopConnectionGoroutines()

	// Close the WebSocket connection
	c.connMutex.Lock()
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.config.Logger.Warn("Error closing WebSocket connection", zap.Error(err))
		}
		c.conn = nil
		// Increment generation to invalidate any outstanding references
		atomic.AddInt64(&c.connGeneration, 1)
	}
	c.connMutex.Unlock()

	// Update metrics
	c.metrics.mutex.Lock()
	c.metrics.Disconnects++
	c.metrics.LastDisconnected = time.Now()
	c.metrics.mutex.Unlock()

	// Call disconnect handler (but don't change state to StateDisconnected)
	c.handlersMutex.RLock()
	onDisconnect := c.onDisconnect
	c.handlersMutex.RUnlock()

	if onDisconnect != nil {
		onDisconnect(nil)
	}

	// Note: State remains StateReconnecting, not changed to StateDisconnected
}

// Close permanently closes the connection and stops all goroutines
func (c *Connection) Close() error {
	c.stopOnce.Do(func() {
		c.config.Logger.Debug("Closing WebSocket connection permanently")

		// Set state to closing first to signal shutdown
		c.setState(StateClosing)

		// Mark channels as closed to prevent new operations early
		atomic.StoreInt32(&c.channelsClosed, 1)

		// Stop heartbeat manager first to prevent it from interfering
		c.config.Logger.Debug("Stopping heartbeat manager")
		if c.heartbeat != nil {
			c.heartbeat.Stop()
		}

		// Cancel main context to signal all goroutines to stop immediately
		c.config.Logger.Debug("Cancelling main connection context")
		c.cancel()
		
		// Stop connection-specific goroutines aggressively
		c.stopConnectionGoroutines()

		// Close the WebSocket connection immediately to unblock I/O
		c.connMutex.Lock()
		if c.conn != nil {
			if err := forceCloseConnection(c.conn); err != nil {
				c.config.Logger.Debug("Error force closing connection during Close", zap.Error(err))
			}
			c.conn = nil
			atomic.AddInt64(&c.connGeneration, 1)
		}
		c.connMutex.Unlock()

		// Close channels to unblock waiting goroutines
		c.closeChannelsSafely()

		// Wait for goroutines with aggressive timeout
		c.config.Logger.Debug("Waiting for all goroutines to finish")
		done := make(chan struct{})
		go func() {
			c.wg.Wait()
			close(done)
		}()
		
		select {
		case <-done:
			c.config.Logger.Debug("All goroutines stopped cleanly")
		case <-time.After(100 * time.Millisecond): // Very short timeout for tests
			c.config.Logger.Debug("Goroutines did not stop within 100ms - forcing cleanup")
			// Since we cancelled contexts and closed connections, they should exit soon
		}

		// Final cleanup of channels
		c.cleanupOnce.Do(func() {
			c.channelMutex.Lock()
			defer c.channelMutex.Unlock()
			
			// Close channels - order matters for proper cleanup
			safeCloseChannel(c.reconnectCh, "reconnect")
			safeCloseChannel(c.messageCh, "message")
			safeCloseChannel(c.closeCh, "close")
		})

		// Set final state
		c.setState(StateClosed)
		c.config.Logger.Info("WebSocket connection closed")
	})

	return nil
}

// closeChannelsSafely safely closes connection channels
func (c *Connection) closeChannelsSafely() {
	defer func() {
		if r := recover(); r != nil {
			c.config.Logger.Debug("Recovered from panic while closing channels", zap.Any("panic", r))
		}
	}()
	
	// Close channels to unblock waiting goroutines with proper synchronization
	c.channelMutex.Lock()
	defer c.channelMutex.Unlock()
	
	// Only close if channels haven't been closed already
	if atomic.LoadInt32(&c.channelsClosed) == 0 {
		// Close message channel first to stop message processing
		safeCloseChannel(c.messageCh, "message")
		
		// Close close channel to unblock any waiting operations
		safeCloseChannel(c.closeCh, "close")
		
		// Close reconnect channel last
		safeCloseChannel(c.reconnectCh, "reconnect")
	}
}

// forceCloseConnection forcefully closes a websocket connection
func forceCloseConnection(conn *websocket.Conn) error {
	if conn == nil {
		return nil
	}
	
	// Set immediate deadlines to interrupt any blocked operations
	now := time.Now()
	conn.SetReadDeadline(now)
	conn.SetWriteDeadline(now)
	
	// Try to send close message
	err := conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseGoingAway, "connection closing"),
		now.Add(50*time.Millisecond))
	
	// Close the connection
	closeErr := conn.Close()
	
	// Return the write error if any, otherwise the close error
	if err != nil {
		return err
	}
	return closeErr
}

// forceCloseConnectionNow immediately force closes the connection to unblock I/O
func (c *Connection) forceCloseConnectionNow() {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()
	
	if c.conn != nil {
		// Set immediate deadlines to unblock any I/O operations
		now := time.Now()
		c.conn.SetReadDeadline(now)
		c.conn.SetWriteDeadline(now)
		
		// Force close underlying TCP connection if possible
		if tcpConn, ok := c.conn.UnderlyingConn().(*net.TCPConn); ok {
			tcpConn.SetLinger(0) // Force immediate close
			tcpConn.Close()
		}
		
		// Close WebSocket connection
		c.conn.Close()
		c.conn = nil
		atomic.AddInt64(&c.connGeneration, 1)
	}
}

// safeCloseChannel safely closes a channel with recovery
func safeCloseChannel(ch interface{}, name string) {
	defer func() {
		if r := recover(); r != nil {
			// Channel was already closed, which is fine
		}
	}()
	
	switch c := ch.(type) {
	case chan struct{}:
		close(c)
	case chan []byte:
		close(c)
	case chan error:
		close(c)
	}
}

// SendMessage sends a message through the WebSocket connection
func (c *Connection) SendMessage(ctx context.Context, message []byte) error {
	c.config.Logger.Debug("Connection sending message",
		zap.String("url", c.url.String()),
		zap.String("state", c.State().String()),
		zap.Int("message_size", len(message)),
		zap.String("message", string(message)))

	// Check state atomically - while there's still a small window for state change
	// after this check, the writePump will handle the actual validation
	state := c.State()
	if state != StateConnected && state != StateReconnecting {
		return fmt.Errorf("connection is not in a valid state for sending: %s", state.String())
	}

	// Apply rate limiting
	if c.config.RateLimiter != nil {
		if err := c.config.RateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit exceeded: %w", err)
		}
	}

	// Check if channels are closed before attempting to send
	if atomic.LoadInt32(&c.channelsClosed) != 0 {
		return errors.New("connection is closed")
	}

	// Use a timeout to prevent indefinite blocking during shutdown
	sendTimeout := 5 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < sendTimeout {
			sendTimeout = remaining
		} else if remaining <= 0 {
			// Context deadline has already passed, return immediately
			return context.DeadlineExceeded
		}
	}
	
	timeoutCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	// Send message through message channel with timeout protection
	select {
	case c.messageCh <- message:
		return nil
	case <-timeoutCtx.Done():
		if timeoutCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("message send timeout after %v", sendTimeout)
		}
		return timeoutCtx.Err()
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

// SendMessageSync sends a message synchronously and waits for it to be processed
func (c *Connection) SendMessageSync(ctx context.Context, message []byte) error {
	if c.State() != StateConnected {
		return errors.New("connection is not connected")
	}

	// Apply rate limiting with timeout to prevent indefinite blocking
	if c.config.RateLimiter != nil {
		// Create a timeout context for rate limiting to prevent goroutine leaks
		rateLimitCtx, cancel := context.WithTimeout(ctx, c.config.WriteTimeout)
		defer cancel()
		
		if err := c.config.RateLimiter.Wait(rateLimitCtx); err != nil {
			return fmt.Errorf("rate limit exceeded: %w", err)
		}
	}

	// Get current metrics before sending
	currentMetrics := c.GetMetrics()
	expectedCount := currentMetrics.MessagesSent + 1

	// Send message through message channel
	select {
	case c.messageCh <- message:
		// Update metrics when message is accepted for sending
		c.metrics.mutex.Lock()
		c.metrics.MessagesSent++
		c.metrics.BytesSent += int64(len(message))
		c.metrics.mutex.Unlock()
		
		c.config.Logger.Debug("Connection message queued successfully",
			zap.String("url", c.url.String()),
			zap.Int("channel_size", len(c.messageCh)),
			zap.Int("channel_capacity", cap(c.messageCh)))
		// Wait for message to be processed by checking metrics
		for i := 0; i < 100; i++ { // Max 100ms wait
			time.Sleep(1 * time.Millisecond)
			metrics := c.GetMetrics()
			if metrics.MessagesSent >= expectedCount {
				return nil
			}
		}
		return nil
	case <-ctx.Done():
		c.config.Logger.Error("Connection send cancelled by context",
			zap.String("url", c.url.String()),
			zap.Error(ctx.Err()))
		return ctx.Err()
	case <-c.ctx.Done():
		c.config.Logger.Error("Connection send cancelled by connection context",
			zap.String("url", c.url.String()),
			zap.Error(c.ctx.Err()))
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

// trySetState atomically attempts to change state from expectedState to newState
// Returns true if the state was successfully changed, false otherwise
func (c *Connection) trySetState(expectedState, newState ConnectionState) bool {
	// Use atomic compare-and-swap for race-free state transition
	for {
		currentState := ConnectionState(atomic.LoadInt32(&c.state))
		
		// Check if current state matches expected state
		if currentState != expectedState {
			return false
		}
		
		// Check if state transition is valid
		if !c.isValidStateTransition(currentState, newState) {
			return false
		}
		
		// Attempt atomic compare-and-swap
		if atomic.CompareAndSwapInt32(&c.state, int32(currentState), int32(newState)) {
			c.config.Logger.Debug("Connection state changed atomically",
				zap.String("from", currentState.String()),
				zap.String("to", newState.String()))
			return true
		}
		
		// CAS failed, retry with updated current state
		// This handles the case where state changed between our read and CAS attempt
	}
}

// trySetStateFromMultiple atomically attempts to change state from any of the expected states to newState
// Returns the previous state and whether the change was successful
func (c *Connection) trySetStateFromMultiple(expectedStates []ConnectionState, newState ConnectionState) (ConnectionState, bool) {
	for {
		currentState := ConnectionState(atomic.LoadInt32(&c.state))
		
		// Check if current state is one of the expected states
		found := false
		for _, expected := range expectedStates {
			if currentState == expected {
				found = true
				break
			}
		}
		if !found {
			return currentState, false
		}
		
		// Check if state transition is valid
		if !c.isValidStateTransition(currentState, newState) {
			return currentState, false
		}
		
		// Attempt atomic compare-and-swap
		if atomic.CompareAndSwapInt32(&c.state, int32(currentState), int32(newState)) {
			c.config.Logger.Debug("Connection state changed atomically from multiple",
				zap.String("from", currentState.String()),
				zap.String("to", newState.String()))
			return currentState, true
		}
		
		// CAS failed, retry with updated current state
	}
}

// isValidStateTransition checks if a state transition is valid
func (c *Connection) isValidStateTransition(from, to ConnectionState) bool {
	switch from {
	case StateDisconnected:
		return to == StateConnecting || to == StateReconnecting || to == StateClosed
	case StateConnecting:
		return to == StateConnected || to == StateDisconnected || to == StateClosed
	case StateConnected:
		return to == StateReconnecting || to == StateClosing || to == StateClosed
	case StateReconnecting:
		return to == StateConnecting || to == StateConnected || to == StateDisconnected || to == StateClosed
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
	return ConnectionMetrics{
		ConnectAttempts:    c.metrics.ConnectAttempts,
		SuccessfulConnects: c.metrics.SuccessfulConnects,
		Disconnects:        c.metrics.Disconnects,
		ReconnectAttempts:  c.metrics.ReconnectAttempts,
		MessagesReceived:   c.metrics.MessagesReceived,
		MessagesSent:       c.metrics.MessagesSent,
		BytesReceived:      c.metrics.BytesReceived,
		BytesSent:          c.metrics.BytesSent,
		Errors:             c.metrics.Errors,
		LastConnected:      c.metrics.LastConnected,
		LastDisconnected:   c.metrics.LastDisconnected,
	}
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
	c.config.Logger.Debug("Setting message handler on connection",
		zap.String("url", c.url.String()),
		zap.String("state", c.State().String()))
	
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
// Uses connection generation tracking to prevent race conditions with reconnection
func (c *Connection) readPump() {
	defer func() {
		c.wg.Done()
		c.connWg.Done()
		c.config.Logger.Debug("ReadPump: Goroutine fully exited")
	}()

	// Check if connection context is available at start
	c.connMutexGr.Lock()
	hasConnCtx := c.connCtx != nil
	c.connMutexGr.Unlock()
	
	if !hasConnCtx {
		c.config.Logger.Debug("ReadPump: No connection context, exiting immediately")
		return // No connection context available
	}

	c.config.Logger.Debug("ReadPump: Starting read loop")

	// Use aggressive exit strategy with immediate force close on cancellation
	exitTicker := time.NewTicker(5 * time.Millisecond) // Very aggressive check
	defer exitTicker.Stop()

	for {
		// Check exit conditions first - prioritize immediate exit
		select {
		case <-c.ctx.Done():
			c.config.Logger.Debug("ReadPump: Main context cancelled, force closing connection and exiting")
			c.forceCloseConnectionNow()
			return
		case <-exitTicker.C:
			// Check connection context on each tick
			c.connMutexGr.Lock()
			currentConnCtx := c.connCtx
			c.connMutexGr.Unlock()
			
			if currentConnCtx != nil {
				select {
				case <-currentConnCtx.Done():
					c.config.Logger.Debug("ReadPump: Connection context cancelled, force closing and exiting")
					c.forceCloseConnectionNow()
					return
				default:
				}
			}
			
			// Check connection state - exit if closing/closed
			if c.State() >= StateClosing {
				c.config.Logger.Debug("ReadPump: Connection closing/closed, exiting")
				return
			}
		default:
		}
		
		// Proceed to read operation - all checks done above

		// Get connection reference and its generation atomically
		c.connMutex.RLock()
		conn := c.conn
		currentGeneration := atomic.LoadInt64(&c.connGeneration)
		c.connMutex.RUnlock()

		if conn == nil {
			c.config.Logger.Debug("ReadPump: No connection available, exiting")
			return // Exit immediately if no connection
		}

		// Validate connection is still current before using it
		if !c.isConnectionValid(conn, currentGeneration) {
			c.config.Logger.Debug("Connection became stale, exiting readPump",
				zap.Int64("generation", currentGeneration))
			return
		}

		// Get current connection context for read operation
		c.connMutexGr.Lock()
		currentConnCtx := c.connCtx
		c.connMutexGr.Unlock()

		// Attempt to read with timeout and cancellation handling
		message, err := c.readMessageWithTimeout(conn, currentGeneration, currentConnCtx)
		if err != nil {
			// Always check for cancellation first on any error
			select {
			case <-c.ctx.Done():
				c.config.Logger.Debug("ReadPump: Main context cancelled during error, exiting")
				return
			default:
			}
			
			if currentConnCtx != nil {
				select {
				case <-currentConnCtx.Done():
					c.config.Logger.Debug("ReadPump: Connection context cancelled during error, exiting")
					return
				default:
				}
			}
			
			// Handle specific error types

			// Check if this indicates connection should be closed
			if isStaleConnectionError(err) ||
				websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) ||
				strings.Contains(err.Error(), "use of closed network connection") ||
				strings.Contains(err.Error(), "connection reset by peer") ||
				strings.Contains(err.Error(), "timeout") {
				c.config.Logger.Debug("ReadPump: Connection closed, invalid, or timeout, exiting", zap.Error(err))
				return
			}

			// For other errors, exit if connection is invalid
			if !c.isConnectionValid(conn, currentGeneration) {
				c.config.Logger.Debug("ReadPump: Connection became invalid, exiting")
				return
			}

			c.config.Logger.Debug("ReadPump: Read error, exiting", zap.Error(err))
			return // Exit on any error to prevent hanging
		}

		// Message read successfully - process it
		if message != nil {
			c.processReceivedMessage(message)
		}
	}
}

// readMessageWithTimeout reads a message with enhanced timeout and cancellation support
func (c *Connection) readMessageWithTimeout(conn *websocket.Conn, generation int64, connCtx context.Context) ([]byte, error) {
	// Enhanced deadline calculation with context awareness - use longer timeout to allow proper I/O completion
	baseTimeout := 100 * time.Millisecond // Reasonable timeout that allows I/O completion
	if timeconfig.IsTestMode() {
		baseTimeout = 50 * time.Millisecond // Shorter for tests but not too aggressive
	}
	readDeadline := time.Now().Add(baseTimeout)
	
	// Respect context deadline if it's sooner
	if deadline, ok := c.ctx.Deadline(); ok && deadline.Before(readDeadline) {
		readDeadline = deadline
	}
	if connCtx != nil {
		if deadline, ok := connCtx.Deadline(); ok && deadline.Before(readDeadline) {
			readDeadline = deadline
		}
	}
	
	if err := conn.SetReadDeadline(readDeadline); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	// Enhanced cancellation check before I/O operation
	select {
	case <-c.ctx.Done():
		c.config.Logger.Debug("ReadPump: Main context cancelled before read")
		return nil, c.ctx.Err()
	default:
	}
	
	if connCtx != nil {
		select {
		case <-connCtx.Done():
			c.config.Logger.Debug("ReadPump: Connection context cancelled before read")
			return nil, connCtx.Err()
		default:
		}
	}
	
	// Read message with connection validation and enhanced error handling
	_, message, err := c.readMessageSafely(conn, generation)
	if err != nil {
		// Priority check for cancellation errors with detailed logging
		select {
		case <-c.ctx.Done():
			c.config.Logger.Debug("ReadPump: Main context cancelled during read operation")
			return nil, c.ctx.Err()
		default:
		}
		
		if connCtx != nil {
			select {
			case <-connCtx.Done():
				c.config.Logger.Debug("ReadPump: Connection context cancelled during read operation")
				return nil, connCtx.Err()
			default:
			}
		}
		
		return nil, err
	}

	return message, nil
}

// processReceivedMessage handles received message processing
func (c *Connection) processReceivedMessage(message []byte) {
	// Update metrics
	c.metrics.mutex.Lock()
	c.metrics.MessagesReceived++
	c.metrics.BytesReceived += int64(len(message))
	c.metrics.mutex.Unlock()

	// Call message handler
	c.handlersMutex.RLock()
	onMessage := c.onMessage
	c.handlersMutex.RUnlock()

	c.config.Logger.Debug("Connection received message",
		zap.String("url", c.url.String()),
		zap.Int("size", len(message)),
		zap.String("message", string(message)),
		zap.Bool("has_handler", onMessage != nil))

	if onMessage != nil {
		onMessage(message)
	}
}


// writePump handles writing messages to the WebSocket connection
// Uses connection generation tracking to prevent race conditions with reconnection
func (c *Connection) writePump() {
	defer func() {
		c.wg.Done()
		c.connWg.Done()
		// Always drain remaining messages on exit to prevent leaks
		c.drainMessageChannel()
		c.config.Logger.Debug("WritePump: Goroutine fully exited")
	}()

	c.config.Logger.Debug("WritePump: Starting write loop")
	
	// Use aggressive exit strategy similar to readPump
	exitTicker := time.NewTicker(5 * time.Millisecond)
	defer exitTicker.Stop()
	
	for {
		// Check exit conditions first - prioritize immediate exit
		select {
		case <-c.ctx.Done():
			c.config.Logger.Debug("WritePump: Main context cancelled, force closing and exiting")
			c.forceCloseConnectionNow()
			return
		case <-exitTicker.C:
			// Check connection context on each tick
			c.connMutexGr.Lock()
			currentConnCtx := c.connCtx
			c.connMutexGr.Unlock()
			
			if currentConnCtx != nil {
				select {
				case <-currentConnCtx.Done():
					c.config.Logger.Debug("WritePump: Connection context cancelled, force closing and exiting")
					c.forceCloseConnectionNow()
					return
				default:
				}
			}
			
			// Check connection state - exit if closing/closed
			state := c.State()
			if state >= StateClosing {
				c.config.Logger.Debug("WritePump: Connection closing/closed, exiting")
				return
			}
		default:
		}
		
		// Get current connection context for message processing
		c.connMutexGr.Lock()
		currentConnCtx := c.connCtx
		c.connMutexGr.Unlock()
		
		// Final cancellation check before message processing
		if currentConnCtx != nil {
			select {
			case <-currentConnCtx.Done():
				c.config.Logger.Debug("WritePump: Connection context cancelled, exiting immediately")
				return
			default:
			}
		}
		
		// Try to read messages non-blocking to prevent hang
		select {
		case <-c.ctx.Done():
			c.config.Logger.Debug("WritePump: Main context cancelled during message wait, exiting")
			return
		case message, ok := <-c.messageCh:
			if !ok {
				c.config.Logger.Debug("WritePump: Message channel closed, exiting")
				return
			}
			
			// Final cancellation check before processing
			select {
			case <-c.ctx.Done():
				c.config.Logger.Debug("WritePump: Main context cancelled before message processing, exiting")
				return
			default:
			}
			
			// Process the message with proper error handling
			if err := c.processMessage(message, currentConnCtx); err != nil {
				// Don't exit on message processing errors in most cases
				c.config.Logger.Debug("WritePump: Error processing message", zap.Error(err))
				// Only exit if it's a cancellation error
				if err == context.Canceled || err == context.DeadlineExceeded {
					return
				}
				// For other errors, continue processing
				continue
			}
			
		default:
			// Non-blocking - continue to next iteration to check exit conditions
			time.Sleep(5 * time.Millisecond) // Brief pause to prevent busy loop
			continue
		}
	}
}

// processMessage handles individual message processing with enhanced cancellation checks and deadline enforcement
func (c *Connection) processMessage(message []byte, connCtx context.Context) error {
	// Enhanced cancellation check with detailed logging before processing
	select {
	case <-c.ctx.Done():
		c.config.Logger.Debug("WritePump: Main context cancelled before processing message")
		return c.ctx.Err()
	default:
	}
	
	if connCtx != nil {
		select {
		case <-connCtx.Done():
			c.config.Logger.Debug("WritePump: Connection context cancelled before processing message")
			return connCtx.Err()
		default:
		}
	}

	// Get connection reference and its generation atomically
	c.connMutex.RLock()
	conn := c.conn
	currentGeneration := atomic.LoadInt64(&c.connGeneration)
	c.connMutex.RUnlock()

	if conn == nil {
		return fmt.Errorf("no connection available")
	}

	// Validate connection is still current before using it
	if !c.isConnectionValid(conn, currentGeneration) {
		return &StaleConnectionError{Generation: currentGeneration}
	}

	// Enhanced write deadline calculation with context awareness
	baseTimeout := 50 * time.Millisecond // Base timeout for faster exit
	writeDeadline := time.Now().Add(baseTimeout)
	
	// Respect context deadline if it's sooner
	if deadline, ok := c.ctx.Deadline(); ok && deadline.Before(writeDeadline) {
		writeDeadline = deadline
	}
	if connCtx != nil {
		if deadline, ok := connCtx.Deadline(); ok && deadline.Before(writeDeadline) {
			writeDeadline = deadline
		}
	}
	
	if err := conn.SetWriteDeadline(writeDeadline); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	// Enhanced cancellation check before I/O operation
	select {
	case <-c.ctx.Done():
		c.config.Logger.Debug("WritePump: Main context cancelled before write")
		return c.ctx.Err()
	default:
	}
	
	if connCtx != nil {
		select {
		case <-connCtx.Done():
			c.config.Logger.Debug("WritePump: Connection context cancelled before write")
			return connCtx.Err()
		default:
		}
	}
	
	// Write message with connection validation and enhanced error handling
	if err := c.writeMessageSafely(conn, currentGeneration, message); err != nil {
		// Priority check for cancellation errors with detailed logging
		select {
		case <-c.ctx.Done():
			c.config.Logger.Debug("WritePump: Main context cancelled during write operation")
			return c.ctx.Err()
		default:
		}
		
		if connCtx != nil {
			select {
			case <-connCtx.Done():
				c.config.Logger.Debug("WritePump: Connection context cancelled during write operation")
				return connCtx.Err()
			default:
			}
		}
		
		return err
	}

	// Message was successfully written - update metrics
	c.metrics.mutex.Lock()
	c.metrics.MessagesSent++
	c.metrics.BytesSent += int64(len(message))
	c.metrics.mutex.Unlock()

	return nil
}


// drainMessageChannel drains the message channel to prevent goroutine leaks
func (c *Connection) drainMessageChannel() {
	for {
		select {
		case <-c.messageCh:
			// Discard message
		default:
			return
		}
	}
}

// writeMessageWithTimeout writes a message with timeout protection
func (c *Connection) writeMessageWithTimeout(message []byte) error {
	// Check connection state first to avoid writing to closing connections
	state := c.State()
	if state == StateClosing || state == StateClosed {
		c.config.Logger.Debug("Connection is closing/closed, cannot write message")
		return websocket.ErrCloseSent
	}
	
	// Acquire connection mutex to prevent concurrent writes
	c.connMutex.Lock()
	defer c.connMutex.Unlock()
	
	conn := c.conn
	if conn == nil {
		c.config.Logger.Debug("No connection available for write")
		return websocket.ErrCloseSent
	}

	// Double-check state after acquiring mutex
	if c.State() == StateClosing || c.State() == StateClosed {
		c.config.Logger.Debug("Connection state changed to closing/closed while acquiring mutex")
		return websocket.ErrCloseSent
	}

	// Set write deadline with context check
	writeDeadline := time.Now().Add(c.config.WriteTimeout)
	if deadline, ok := c.ctx.Deadline(); ok && deadline.Before(writeDeadline) {
		writeDeadline = deadline
	}
	conn.SetWriteDeadline(writeDeadline)

	// Write message with panic recovery
	var writeErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				c.config.Logger.Debug("Recovered from write panic", zap.Any("panic", r))
				writeErr = websocket.ErrCloseSent
			}
		}()
		writeErr = conn.WriteMessage(websocket.TextMessage, message)
	}()
	
	// Check for timeout errors which might be due to context deadline
	if writeErr != nil {
		if netErr, ok := writeErr.(interface{ Timeout() bool }); ok && netErr.Timeout() {
			// Check if timeout was due to context cancellation
			select {
			case <-c.ctx.Done():
				c.config.Logger.Debug("Write timeout due to context cancellation")
				return c.ctx.Err()
			default:
				// Regular timeout
			}
		}
	}
	
	return writeErr
}

// drainMessageChannelWithMetrics drains any remaining messages to prevent goroutines from blocking
func (c *Connection) drainMessageChannelWithMetrics() {
	// Use a done channel to ensure proper synchronization
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Ignore panics during draining (channel might be closed)
			}
			close(done)
		}()
		
		// Use timer to prevent goroutine leak
		timer := time.NewTimer(100 * time.Millisecond)
		defer func() {
			if !timer.Stop() {
				// Drain the timer channel if it wasn't stopped
				select {
				case <-timer.C:
				default:
				}
			}
		}()
		
		for {
			select {
			case <-c.messageCh:
				// Drain message and update metrics to prevent count loss
				c.metrics.mutex.Lock()
				c.metrics.MessagesSent++
				c.metrics.mutex.Unlock()
			case <-timer.C:
				return
			default:
				return
			}
		}
	}()
	
	// Wait for draining to complete with timeout
	timeout := time.NewTimer(200 * time.Millisecond)
	defer timeout.Stop()
	select {
	case <-done:
		// Draining completed
	case <-timeout.C:
		// Timeout - continue anyway
	}
}

// WaitForMessages waits for all pending messages to be processed
func (c *Connection) WaitForMessages(ctx context.Context, expectedCount int64) error {
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return fmt.Errorf("timeout waiting for messages to be processed")
		default:
			metrics := c.GetMetrics()
			if metrics.MessagesSent >= expectedCount {
				return nil
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// triggerReconnect triggers a reconnection attempt
// Uses atomic compare-and-swap to avoid race conditions between state check and change
func (c *Connection) triggerReconnect() {
	// Atomically transition from Connected to Reconnecting
	if c.trySetState(StateConnected, StateReconnecting) {
		select {
		case c.reconnectCh <- struct{}{}:
		default:
			// Channel is full, reconnection already pending
		}
		return
	}
	
	// Fallback approach for other states
	currentState := c.State()
	if currentState == StateConnected || currentState == StateDisconnected {
		c.setState(StateReconnecting)

		// Use mutex to synchronize channel access with Close()
		c.channelMutex.Lock()
		defer c.channelMutex.Unlock()
		
		// Only trigger reconnect if channels are still open
		if atomic.LoadInt32(&c.channelsClosed) == 0 {
			select {
			case c.reconnectCh <- struct{}{}:
			default: // Channel full, skip this reconnect attempt
			}
		}
	}
}

// ForceConnectionCheck forces a connection check to detect disconnection
func (c *Connection) ForceConnectionCheck() {
	if c.State() == StateConnected {
		c.connMutex.Lock()
		conn := c.conn
		if conn != nil {
			// Try to write a test message to detect disconnection (must be done while holding the lock)
			conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
			err := conn.WriteMessage(websocket.TextMessage, []byte("connection-check"))
			c.connMutex.Unlock()
			
			if err != nil {
				c.config.Logger.Debug("Force connection check detected disconnection", zap.Error(err))
				c.triggerReconnect()
			} else {
				c.config.Logger.Debug("Force connection check - connection still alive")
			}
		} else {
			c.connMutex.Unlock()
		}
	}
	// If state transition failed, it means we're not in StateConnected,
	// so no reconnection is needed
}

// StartAutoReconnect starts automatic reconnection handling
func (c *Connection) StartAutoReconnect(ctx context.Context) {
	// Check if context is already cancelled before starting
	select {
	case <-ctx.Done():
		c.config.Logger.Debug("StartAutoReconnect: Context already cancelled, not starting auto-reconnect loop")
		return
	case <-c.ctx.Done():
		c.config.Logger.Debug("StartAutoReconnect: Connection context already cancelled, not starting auto-reconnect loop")
		return
	default:
	}
	
	c.wg.Add(1)
	go c.autoReconnectLoop(ctx)
}

// autoReconnectLoop handles automatic reconnection
func (c *Connection) autoReconnectLoop(ctx context.Context) {
	defer func() {
		c.wg.Done()
		c.config.Logger.Debug("AutoReconnectLoop: Goroutine fully exited")
	}()

	c.config.Logger.Debug("AutoReconnectLoop: Starting auto-reconnect loop")

	// Use a ticker to periodically check for exit conditions
	timeoutTicker := time.NewTicker(10 * time.Millisecond)
	defer timeoutTicker.Stop()

	for {
		// Priority check for shutdown with immediate exit
		select {
		case <-ctx.Done():
			c.config.Logger.Debug("AutoReconnectLoop: Parent context cancelled, exiting immediately")
			return
		case <-c.ctx.Done():
			c.config.Logger.Debug("AutoReconnectLoop: Connection context cancelled, exiting immediately")
			return
		case <-c.closeCh:
			c.config.Logger.Debug("AutoReconnectLoop: Close channel signaled, exiting immediately")
			return
		case <-c.reconnectCh:
			// Double-check if contexts are still valid before reconnecting
			select {
			case <-ctx.Done():
				c.config.Logger.Debug("AutoReconnectLoop: Parent context cancelled during reconnect, exiting")
				return
			case <-c.ctx.Done():
				c.config.Logger.Debug("AutoReconnectLoop: Connection context cancelled during reconnect, exiting")
				return
			case <-c.closeCh:
				c.config.Logger.Debug("AutoReconnectLoop: Close channel signaled during reconnect, exiting")
				return
			default:
				c.config.Logger.Debug("AutoReconnectLoop: Performing reconnect")
				c.performReconnect(ctx)
			}
		case <-timeoutTicker.C:
			// Periodic check to ensure responsiveness
			continue
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

	// Close existing connection but maintain reconnecting state
	c.disconnectForReconnect()

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
	case <-c.closeCh:
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

// isConnectionValid checks if the given connection is still the current active connection
// by comparing its generation with the current connection generation
func (c *Connection) isConnectionValid(conn *websocket.Conn, generation int64) bool {
	if conn == nil {
		return false
	}
	
	c.connMutex.RLock()
	currentConn := c.conn
	currentGeneration := atomic.LoadInt64(&c.connGeneration)
	c.connMutex.RUnlock()
	
	// Connection is valid if it's the same pointer and generation matches
	return currentConn == conn && currentGeneration == generation
}

// readMessageSafely reads a message with connection validation
func (c *Connection) readMessageSafely(conn *websocket.Conn, generation int64) (messageType int, message []byte, err error) {
	// Add comprehensive panic recovery for gorilla websocket panics
	defer func() {
		if r := recover(); r != nil {
			// Convert panic to error
			messageType = 0
			message = nil
			
			err = fmt.Errorf("websocket read panic: %v", r)
			c.config.Logger.Debug("WebSocket read panic recovered - connection likely closed", 
				zap.Any("panic", r),
				zap.Int64("generation", generation))
		}
	}()
	
	// Validate connection before reading
	if !c.isConnectionValid(conn, generation) {
		return 0, nil, &StaleConnectionError{Generation: generation}
	}
	
	// Double-check that the connection is not nil (additional safety)
	if conn == nil {
		return 0, nil, fmt.Errorf("websocket connection is nil")
	}
	
	// Use very short read deadline to prevent hanging
	readDeadline := time.Now().Add(100 * time.Millisecond)
	if err := conn.SetReadDeadline(readDeadline); err != nil {
		return 0, nil, fmt.Errorf("failed to set read deadline: %w", err)
	}
	
	// Attempt to read the message with safety checks
	messageType, message, err = conn.ReadMessage()
	if err != nil {
		// Log specific error types for debugging
		if strings.Contains(err.Error(), "use of closed network connection") {
			c.config.Logger.Debug("Attempted read on closed network connection", zap.Error(err))
		} else if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			c.config.Logger.Debug("WebSocket closed normally during read", zap.Error(err))
		} else if strings.Contains(err.Error(), "timeout") {
			c.config.Logger.Debug("Read timeout - this is expected for non-blocking reads", zap.Error(err))
		}
		return messageType, message, err
	}
	
	// Validate connection again after reading to ensure it wasn't replaced during read
	if !c.isConnectionValid(conn, generation) {
		return 0, nil, &StaleConnectionError{Generation: generation}
	}
	
	return messageType, message, nil
}

// StaleConnectionError indicates that a connection reference has become stale
type StaleConnectionError struct {
	Generation int64
}

func (e *StaleConnectionError) Error() string {
	return fmt.Sprintf("connection became stale (generation: %d)", e.Generation)
}

// isStaleConnectionError checks if an error indicates a stale connection
func isStaleConnectionError(err error) bool {
	var staleErr *StaleConnectionError
	return errors.As(err, &staleErr)
}

// writeMessageSafely writes a message with connection validation
func (c *Connection) writeMessageSafely(conn *websocket.Conn, generation int64, message []byte) error {
	// Add panic recovery for websocket write operations
	defer func() {
		if r := recover(); r != nil {
			c.config.Logger.Debug("WebSocket write panic recovered - connection likely closed", 
				zap.Any("panic", r),
				zap.Int64("generation", generation),
				zap.Int("message_size", len(message)))
		}
	}()
	
	// Validate connection before writing
	if !c.isConnectionValid(conn, generation) {
		return &StaleConnectionError{Generation: generation}
	}
	
	// Double-check that the connection is not nil (additional safety)
	if conn == nil {
		return fmt.Errorf("websocket connection is nil")
	}
	
	// Use very short write deadline to prevent hanging
	writeDeadline := time.Now().Add(100 * time.Millisecond)
	if err := conn.SetWriteDeadline(writeDeadline); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}
	
	// Attempt to write the message
	err := conn.WriteMessage(websocket.TextMessage, message)
	if err != nil {
		// Log specific error types for debugging
		if strings.Contains(err.Error(), "use of closed network connection") {
			c.config.Logger.Debug("Attempted write on closed network connection", zap.Error(err))
		} else if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			c.config.Logger.Debug("WebSocket closed normally during write", zap.Error(err))
		} else if strings.Contains(err.Error(), "timeout") {
			c.config.Logger.Debug("Write timeout - this is expected for non-blocking writes", zap.Error(err))
		}
		return err
	}
	
	// Validate connection again after writing to ensure it wasn't replaced during write
	if !c.isConnectionValid(conn, generation) {
		return &StaleConnectionError{Generation: generation}
	}
	
	return nil
}

