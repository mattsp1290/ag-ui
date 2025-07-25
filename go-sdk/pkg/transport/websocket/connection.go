package websocket

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"runtime"
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

	// TLSClientConfig is the TLS configuration for secure WebSocket connections
	// If nil, default TLS settings are used
	TLSClientConfig *tls.Config
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
	conn.SetPongHandler(func(string) error {
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

	// Start heartbeat (manages its own goroutines)
	c.heartbeat.Start(c.ctx)

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
		// Force close to interrupt any blocked I/O operations
		if err := forceCloseConnection(c.conn); err != nil {
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

		// Set state to closing first to signal shutdown
		c.setState(StateClosing)

		// Mark channels as closed to prevent new operations early
		atomic.StoreInt32(&c.channelsClosed, 1)

		// Cancel context to signal all goroutines to stop immediately
		c.config.Logger.Debug("Cancelling connection context")
		c.cancel()

		// Close WebSocket connection to interrupt any blocked I/O operations
		c.connMutex.Lock()
		if c.conn != nil {
			if err := forceCloseConnection(c.conn); err != nil {
				c.config.Logger.Debug("Error closing connection", zap.Error(err))
			}
			c.conn = nil
		}
		c.connMutex.Unlock()

		// Stop heartbeat manager AFTER context cancellation to ensure clean shutdown
		c.config.Logger.Debug("Stopping heartbeat manager")
		c.heartbeat.Stop()

		// Close all channels in coordinated manner
		c.cleanupOnce.Do(func() {
			// Use mutex to synchronize channel closing with any ongoing channel operations
			c.channelMutex.Lock()
			defer c.channelMutex.Unlock()
			
			// Close channels - order matters for proper cleanup
			safeCloseChannel(c.reconnectCh, "reconnect")  // Stop reconnect attempts first
			safeCloseChannel(c.messageCh, "message")      // Stop message sending
			safeCloseChannel(c.closeCh, "close")          // Signal final closure
		})

		// Wait for all goroutines to finish with timeout
		c.config.Logger.Debug("Waiting for goroutines to finish")
		done := make(chan struct{})
		go func() {
			c.wg.Wait()
			close(done)
		}()

		// Wait with timeout to prevent hanging
		select {
		case <-done:
			c.config.Logger.Debug("All connection goroutines stopped")
		case <-time.After(3 * time.Second): // Increased timeout for graceful shutdown
			c.config.Logger.Warn("Timeout waiting for goroutines to stop")
			// Log current goroutine count for debugging
			c.config.Logger.Warn("Current goroutine count", zap.Int("count", runtime.NumGoroutine()))
			// Force GC to help with cleanup
			runtime.GC()
		}

		// Set final state
		c.setState(StateClosed)
		c.config.Logger.Info("WebSocket connection closed")
	})

	return nil
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
		if remaining := time.Until(deadline); remaining < sendTimeout {
			sendTimeout = remaining
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

	// Apply rate limiting
	if c.config.RateLimiter != nil {
		if err := c.config.RateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit exceeded: %w", err)
		}
	}

	// Get current metrics before sending
	currentMetrics := c.GetMetrics()
	expectedCount := currentMetrics.MessagesSent + 1

	// Send message through message channel
	select {
	case c.messageCh <- message:
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
		return to == StateConnecting || to == StateReconnecting || to == StateClosed
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
	
	c.config.Logger.Debug("Starting read pump")

	for {
		// First check context before any blocking operations
		select {
		case <-c.ctx.Done():
			c.config.Logger.Debug("Read pump stopped due to context cancellation")
			return
		default:
			// Check state before proceeding
			if c.State() == StateClosing || c.State() == StateClosed {
				c.config.Logger.Debug("Read pump exiting - connection closing/closed")
				return
			}

			c.connMutex.RLock()
			conn := c.conn
			c.connMutex.RUnlock()

			if conn == nil {
				// Check context before sleeping to be responsive to cancellation
				select {
				case <-c.ctx.Done():
					c.config.Logger.Debug("Read pump stopped while waiting for connection")
					return
				case <-time.After(50 * time.Millisecond): // Reduced sleep time for faster shutdown
					continue
				}
			}
			
			// Check if we're still connected before attempting to read
			// Allow reading in reconnecting state to detect recovery
			state := c.State()
			if state != StateConnected && state != StateReconnecting {
				c.config.Logger.Debug("Read pump exiting - not connected or reconnecting")
				return
			}

			// Set a short read deadline to periodically check context
			// This ensures the goroutine can exit promptly when context is cancelled
			readTimeout := 500 * time.Millisecond // Reduced for faster shutdown response
			if c.config.ReadTimeout < readTimeout {
				readTimeout = c.config.ReadTimeout
			}
			
			// Also respect context deadline if it's sooner
			readDeadline := time.Now().Add(readTimeout)
			if deadline, ok := c.ctx.Deadline(); ok && deadline.Before(readDeadline) {
				readDeadline = deadline
			}
			conn.SetReadDeadline(readDeadline)

			// Read message with panic recovery
			var message []byte
			var err error
			
			// Protect against panic from reading closed connection
			func() {
				defer func() {
					if r := recover(); r != nil {
						c.config.Logger.Debug("Recovered from read panic", zap.Any("panic", r))
						err = websocket.ErrCloseSent
					}
				}()
				_, message, err = conn.ReadMessage()
			}()
			if err != nil {
				// Check if error is due to context cancellation
				select {
				case <-c.ctx.Done():
					c.config.Logger.Debug("Read pump stopped during message read due to context cancellation")
					return
				default:
					// Check if it's a timeout error (expected for periodic context checks)
					if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
						// Timeout is expected, but check if we should still continue
						if c.State() == StateConnected {
							continue
						}
						return
					}
					
					// Check for normal close
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						c.config.Logger.Debug("Connection closed normally")
						return
					}
					
					c.setError(fmt.Errorf("failed to read message: %w", err))

					// Check context before attempting reconnect
					select {
					case <-c.ctx.Done():
						c.config.Logger.Debug("Read pump stopped due to context cancellation during error handling")
						return
					default:
						// Check if we should reconnect
						if c.State() == StateConnected {
							c.triggerReconnect()
						}
						return
					}
				}
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
	
	c.config.Logger.Debug("Starting write pump")

	// Create a timeout ticker to ensure we can exit even if contexts don't work
	timeout := time.NewTicker(100 * time.Millisecond)
	defer timeout.Stop()
	
	for {
		// Check exit conditions first before blocking on select
		select {
		case <-c.ctx.Done():
			c.config.Logger.Debug("Write pump stopped due to context cancellation")
			c.drainMessageChannel()
			return
		default:
		}
		
		// Check connection state
		state := c.State()
		if state == StateClosing || state == StateClosed {
			c.config.Logger.Debug("Write pump exiting - connection closing/closed", 
				zap.String("state", state.String()))
			c.drainMessageChannel()
			return
		}

		// Use a timeout-based select to ensure we can always exit
		select {
		case <-c.ctx.Done():
			c.config.Logger.Debug("Write pump stopped due to context cancellation")
			c.drainMessageChannel()
			return
		case <-timeout.C:
			// Periodic check - just continue the loop to re-check conditions
			continue
		case message, ok := <-c.messageCh:
			// Check if message channel was closed
			if !ok {
				c.config.Logger.Debug("Message channel closed, stopping write pump")
				return
			}

			// Check context before processing message
			select {
			case <-c.ctx.Done():
				c.config.Logger.Debug("Context cancelled while processing message in write pump")
				return
			default:
			}

			// Check if we're still in a valid state to write
			if c.State() == StateClosing || c.State() == StateClosed {
				c.config.Logger.Debug("Write pump exiting - connection closing/closed")
				return
			}

			// Process the message with timeout protection
			if err := c.writeMessageWithTimeout(message); err != nil {
				// Check if error is due to context cancellation
				select {
				case <-c.ctx.Done():
					c.config.Logger.Debug("Write pump stopped during message write due to context cancellation")
					return
				default:
					c.setError(fmt.Errorf("failed to write message: %w", err))
					
					// Check if we should reconnect
					if c.State() == StateConnected {
						c.triggerReconnect()
					}
					return
				}
			}

			// Update metrics
			c.metrics.mutex.Lock()
			c.metrics.MessagesSent++
			c.metrics.BytesSent += int64(len(message))
			c.metrics.mutex.Unlock()
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

// drainMessageChannel drains any remaining messages to prevent goroutines from blocking
func (c *Connection) drainMessageChannel() {
	// Use a done channel to ensure proper synchronization
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Ignore panics during draining (channel might be closed)
			}
			close(done)
		}()
		
		timeout := time.After(100 * time.Millisecond)
		for {
			select {
			case <-c.messageCh:
				// Drain message and update metrics to prevent count loss
				c.metrics.mutex.Lock()
				c.metrics.MessagesSent++
				c.metrics.mutex.Unlock()
			case <-timeout:
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
func (c *Connection) triggerReconnect() {
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
}

// StartAutoReconnect starts automatic reconnection handling
func (c *Connection) StartAutoReconnect(ctx context.Context) {
	c.wg.Add(1)
	go c.autoReconnectLoop(ctx)
}

// autoReconnectLoop handles automatic reconnection
func (c *Connection) autoReconnectLoop(ctx context.Context) {
	defer c.wg.Done()
	defer c.config.Logger.Debug("Auto reconnect loop goroutine exited")
	
	c.config.Logger.Debug("Starting auto reconnect loop")

	for {
		select {
		case <-ctx.Done():
			c.config.Logger.Debug("Auto reconnect loop stopped due to parent context cancellation")
			return
		case <-c.ctx.Done():
			c.config.Logger.Debug("Auto reconnect loop stopped due to connection context cancellation")
			return
		case _, ok := <-c.reconnectCh:
			// Check if channel was closed
			if !ok {
				c.config.Logger.Debug("Reconnect channel closed, stopping auto reconnect loop")
				return
			}
			
			// Check state before attempting reconnect
			if c.State() == StateClosing || c.State() == StateClosed {
				c.config.Logger.Debug("Auto reconnect loop exiting - connection closing/closed")
				return
			}
			
			// Check context again before starting reconnect
			select {
			case <-ctx.Done():
				c.config.Logger.Debug("Context cancelled before reconnect attempt")
				return
			case <-c.ctx.Done():
				c.config.Logger.Debug("Connection context cancelled before reconnect attempt")
				return
			default:
				c.performReconnect(ctx)
			}
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


