package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/internal"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	pkgerrors "github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// SSEEvent represents a parsed Server-Sent Event
type SSEEvent struct {
	ID        string            `json:"id,omitempty"`
	Event     string            `json:"event,omitempty"`
	Data      string            `json:"data"`
	Retry     *time.Duration    `json:"retry,omitempty"`
	Raw       string            `json:"raw"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Sequence  uint64            `json:"sequence"`
}

// SSEConnectionState represents the state of the SSE connection
type SSEConnectionState int

const (
	SSEStateDisconnected SSEConnectionState = iota
	SSEStateConnecting
	SSEStateConnected
	SSEStateReconnecting
	SSEStateClosed
)

// String returns the string representation of the connection state
func (s SSEConnectionState) String() string {
	switch s {
	case SSEStateDisconnected:
		return "disconnected"
	case SSEStateConnecting:
		return "connecting"
	case SSEStateConnected:
		return "connected"
	case SSEStateReconnecting:
		return "reconnecting"
	case SSEStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// SSEClientConfig contains configuration options for the SSE client
type SSEClientConfig struct {
	// URL is the SSE endpoint URL
	URL string

	// Headers contains custom headers to send with the request
	Headers map[string]string

	// InitialBackoff is the initial backoff duration for reconnection (default: 1s)
	InitialBackoff time.Duration

	// MaxBackoff is the maximum backoff duration for reconnection (default: 30s)
	MaxBackoff time.Duration

	// BackoffMultiplier is the multiplier for exponential backoff (default: 2.0)
	BackoffMultiplier float64

	// MaxReconnectAttempts is the maximum number of reconnect attempts (0 = unlimited)
	MaxReconnectAttempts int

	// EventBufferSize is the size of the event buffer (default: 1000)
	EventBufferSize int

	// ReadTimeout is the timeout for reading from the connection (default: 0 = no timeout)
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for writing to the connection (default: 10s)
	WriteTimeout time.Duration

	// HealthCheckInterval is the interval for connection health checks (default: 30s)
	HealthCheckInterval time.Duration

	// MaxStreamLifetime is the maximum lifetime for a streaming connection (default: 30m)
	// After this duration, the stream will be automatically terminated to prevent goroutine leaks
	MaxStreamLifetime time.Duration

	// LastEventID is the last event ID for resuming connections
	LastEventID string

	// RetryInterval is the default retry interval suggested by the server
	RetryInterval time.Duration

	// TLSConfig for HTTPS connections
	TLSConfig *tls.Config

	// SkipTLSVerify skips TLS certificate verification (insecure)
	SkipTLSVerify bool

	// UserAgent is the User-Agent header value
	UserAgent string

	// EnableCompression enables gzip compression
	EnableCompression bool

	// EventFilter filters events by event type (nil = no filtering)
	EventFilter func(eventType string) bool

	// OnConnect is called when the connection is established
	OnConnect func()

	// OnDisconnect is called when the connection is lost
	OnDisconnect func(err error)

	// OnReconnect is called when reconnection starts
	OnReconnect func(attempt int)

	// OnError is called when an error occurs
	OnError func(err error)

	// FlowControlEnabled enables backpressure handling
	FlowControlEnabled bool

	// FlowControlThreshold is the buffer threshold for flow control (default: 80%)
	FlowControlThreshold float64
}

// SSEClientState holds all client state that needs to be read atomically
type SSEClientState struct {
	state          SSEConnectionState
	closed         bool
	sequence       uint64
	lastEventID    string
	reconnectCount int
	backpressure   bool
	lastActivity   time.Time
}

// SSEClient is a robust Server-Sent Events streaming client
type SSEClient struct {
	config  SSEClientConfig
	client  *http.Client
	conn    *http.Response
	scanner *bufio.Scanner

	// Unified state management (protected by single mutex)
	clientState SSEClientState
	stateMutex  sync.RWMutex

	// Event handling
	eventChan   chan *SSEEvent
	eventBuffer []*SSEEvent
	bufferMutex sync.RWMutex

	// Reconnection (separated from main state for independent access)
	currentBackoff time.Duration
	reconnectTimer *time.Timer
	reconnectMutex sync.RWMutex

	// Health monitoring
	healthTicker *time.Ticker
	healthMutex  sync.RWMutex

	// Callback pool for efficient callback execution
	callbackPool *internal.CallbackPool

	// Context and cancellation
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewSSEClient creates a new SSE client with the given configuration
func NewSSEClient(config SSEClientConfig) (*SSEClient, error) {
	if config.URL == "" {
		return nil, pkgerrors.NewValidationErrorWithField("URL", "required", "URL cannot be empty", config.URL)
	}

	// Validate URL
	parsedURL, err := url.Parse(config.URL)
	if err != nil {
		return nil, pkgerrors.NewValidationErrorWithField("URL", "format", "invalid URL format", config.URL).WithCause(err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, pkgerrors.NewValidationErrorWithField("URL", "scheme", "URL must use http or https scheme", parsedURL.Scheme)
	}

	// Set default values
	if config.InitialBackoff == 0 {
		config.InitialBackoff = time.Second
	}
	if config.MaxBackoff == 0 {
		config.MaxBackoff = 30 * time.Second
	}
	if config.BackoffMultiplier == 0 {
		config.BackoffMultiplier = 2.0
	}
	if config.EventBufferSize == 0 {
		config.EventBufferSize = 1000
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 10 * time.Second
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 30 * time.Second
	}
	if config.MaxStreamLifetime == 0 {
		config.MaxStreamLifetime = 30 * time.Minute
	}
	if config.UserAgent == "" {
		config.UserAgent = "ag-ui-go-sdk-sse/1.0.0"
	}
	if config.FlowControlThreshold == 0 {
		config.FlowControlThreshold = 0.8
	}
	if config.Headers == nil {
		config.Headers = make(map[string]string)
	}

	// Create HTTP client
	transport := &http.Transport{
		TLSClientConfig: config.TLSConfig,
	}

	if config.SkipTLSVerify {
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.InsecureSkipVerify = true
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   0, // No timeout for streaming connections
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &SSEClient{
		config:         config,
		client:         httpClient,
		eventChan:      make(chan *SSEEvent, config.EventBufferSize),
		eventBuffer:    make([]*SSEEvent, 0),
		currentBackoff: config.InitialBackoff,
		callbackPool:   internal.NewCallbackPool(0), // Use default worker count (runtime.NumCPU())
		ctx:            ctx,
		cancel:         cancel,
	}

	// Initialize client state atomically
	client.clientState = SSEClientState{
		state:          SSEStateDisconnected,
		closed:         false,
		sequence:       0,
		lastEventID:    config.LastEventID,
		reconnectCount: 0,
		backpressure:   false,
		lastActivity:   time.Now(),
	}

	// Last event ID is already set in clientState initialization

	return client, nil
}

// Connect establishes the SSE connection
func (c *SSEClient) Connect(ctx context.Context) error {
	if c.isClosed() {
		return pkgerrors.NewOperationError("Connect", "SSEClient", fmt.Errorf("client is closed"))
	}

	c.setState(SSEStateConnecting)

	req, err := c.createRequest(ctx)
	if err != nil {
		c.setState(SSEStateDisconnected)
		return pkgerrors.NewAgentError(
			pkgerrors.ErrorTypeValidation,
			"failed to create HTTP request",
			"SSEClient",
		).WithCause(err).WithDetail("operation", "Connect")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.setState(SSEStateDisconnected)
		c.callOnError(err)
		return pkgerrors.NewAgentError(
			pkgerrors.ErrorTypeExternal,
			"HTTP request failed",
			"SSEClient",
		).WithCause(err).WithDetail("operation", "Connect")
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		c.setState(SSEStateDisconnected)
		err := fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		c.callOnError(err)
		return pkgerrors.NewAgentError(
			pkgerrors.ErrorTypeExternal,
			"unexpected HTTP status code",
			"SSEClient",
		).WithCause(err).WithDetail("operation", "Connect").WithDetail("status_code", resp.StatusCode)
	}

	// Verify content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		resp.Body.Close()
		c.setState(SSEStateDisconnected)
		err := fmt.Errorf("unexpected content type: %s", contentType)
		c.callOnError(err)
		return pkgerrors.NewValidationError("invalid_content_type", "expected text/event-stream content type").
			WithField("Content-Type", contentType).WithDetail("operation", "Connect")
	}

	c.stateMutex.Lock()
	c.conn = resp
	c.scanner = bufio.NewScanner(resp.Body)
	c.stateMutex.Unlock()
	c.setState(SSEStateConnected)
	c.updateActivity()
	c.resetReconnectState()

	// Start background goroutines including stream lifetime monitor
	c.wg.Add(4)
	go c.readLoop()
	go c.healthCheck()
	go c.processEvents()
	go c.streamLifetimeMonitor()

	c.callOnConnect()
	return nil
}

// Events returns a channel that receives SSE events
func (c *SSEClient) Events() <-chan *SSEEvent {
	return c.eventChan
}

// Close closes the SSE client and releases resources
func (c *SSEClient) Close() error {
	if !c.setClosed() {
		return pkgerrors.NewAgentError(
			pkgerrors.ErrorTypeInvalidState,
			"client already closed",
			"SSEClient",
		).WithDetail("operation", "Close")
	}

	c.setState(SSEStateClosed)
	c.cancel()

	// Close connection
	c.stateMutex.Lock()
	if c.conn != nil {
		c.conn.Body.Close()
		c.conn = nil
	}
	c.stateMutex.Unlock()

	// Stop timers
	c.healthMutex.Lock()
	if c.reconnectTimer != nil {
		c.reconnectTimer.Stop()
		c.reconnectTimer = nil
	}
	if c.healthTicker != nil {
		c.healthTicker.Stop()
		c.healthTicker = nil
	}
	c.healthMutex.Unlock()

	// Wait for goroutines
	c.wg.Wait()

	// Stop callback pool
	if c.callbackPool != nil {
		c.callbackPool.Stop()
	}

	// Close event channel
	close(c.eventChan)

	return nil
}

// State returns the current connection state
func (c *SSEClient) State() SSEConnectionState {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.clientState.state
}

// LastEventID returns the last received event ID
func (c *SSEClient) LastEventID() string {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.clientState.lastEventID
}

// ReconnectCount returns the number of reconnection attempts
func (c *SSEClient) ReconnectCount() int {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.clientState.reconnectCount
}

// BufferLength returns the current buffer length
func (c *SSEClient) BufferLength() int {
	c.bufferMutex.RLock()
	defer c.bufferMutex.RUnlock()
	return len(c.eventBuffer)
}

// IsBackpressureActive returns true if backpressure is currently active
func (c *SSEClient) IsBackpressureActive() bool {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.clientState.backpressure
}

// createRequest creates an HTTP request for the SSE connection
func (c *SSEClient) createRequest(ctx context.Context) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.config.URL, nil)
	if err != nil {
		return nil, err
	}

	// Set SSE headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", c.config.UserAgent)

	if c.config.EnableCompression {
		req.Header.Set("Accept-Encoding", "gzip")
	}

	// Set last event ID for resuming
	if lastEventID := c.LastEventID(); lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}

	// Set custom headers
	for key, value := range c.config.Headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

// readLoop reads events from the SSE stream
func (c *SSEClient) readLoop() {
	defer c.wg.Done()

	var event *SSEEvent
	var eventLines []string

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		if c.scanner == nil {
			return
		}

		// Set read timeout if configured
		c.stateMutex.RLock()
		hasConn := c.conn != nil
		c.stateMutex.RUnlock()
		if c.config.ReadTimeout > 0 && hasConn {
			// Note: Read timeout is handled at the HTTP client level or connection level
			// For SSE, we rely on the connection's underlying timeout mechanisms
		}

		if !c.scanner.Scan() {
			if err := c.scanner.Err(); err != nil {
				c.callOnError(err)
				c.handleConnectionError(err)
			} else {
				// EOF reached
				c.handleConnectionError(io.EOF)
			}
			return
		}

		line := c.scanner.Text()
		c.updateActivity()

		// Parse SSE event
		if line == "" {
			// Empty line indicates end of event
			if event != nil && len(eventLines) > 0 {
				// Check context before processing event
				select {
				case <-c.ctx.Done():
					return
				default:
				}
				c.finalizeEvent(event, eventLines)
				eventLines = eventLines[:0]
				event = nil
			}
			continue
		}

		if event == nil {
			// Check context before creating new event
			select {
			case <-c.ctx.Done():
				return
			default:
			}
			event = &SSEEvent{
				Timestamp: time.Now(),
				Sequence:  c.getNextSequence(),
				Headers:   make(map[string]string),
			}
		}

		eventLines = append(eventLines, line)

		// Parse event fields
		if err := c.parseEventLine(event, line); err != nil {
			c.callOnError(err)
			continue
		}
	}
}

// streamLifetimeMonitor monitors the stream lifetime and terminates the connection
// when the maximum lifetime is exceeded to prevent goroutine leaks
func (c *SSEClient) streamLifetimeMonitor() {
	defer c.wg.Done()

	// Set up stream lifetime timer
	streamLifetimeTimer := time.NewTimer(c.config.MaxStreamLifetime)
	defer streamLifetimeTimer.Stop()

	select {
	case <-c.ctx.Done():
		// Connection was closed normally
		return
	case <-streamLifetimeTimer.C:
		// Stream lifetime exceeded, terminate to prevent goroutine leaks
		c.callOnError(fmt.Errorf("stream lifetime exceeded (%v), terminating connection to prevent goroutine leaks", c.config.MaxStreamLifetime))
		c.handleConnectionError(fmt.Errorf("stream lifetime exceeded"))
		return
	}
}

// parseEventLine parses a single line of an SSE event
func (c *SSEClient) parseEventLine(event *SSEEvent, line string) error {
	// Handle comments
	if strings.HasPrefix(line, ":") {
		return nil
	}

	// Find field separator
	colonIndex := strings.Index(line, ":")
	if colonIndex == -1 {
		// Line without colon is treated as field name with empty value
		field := strings.TrimSpace(line)
		return c.setEventField(event, field, "")
	}

	field := line[:colonIndex]
	value := line[colonIndex+1:]

	// Remove leading space from value
	if len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}

	return c.setEventField(event, field, value)
}

// setEventField sets a field value on an SSE event
func (c *SSEClient) setEventField(event *SSEEvent, field, value string) error {
	switch field {
	case "id":
		event.ID = value
	case "event":
		event.Event = value
	case "data":
		if event.Data != "" {
			event.Data += "\n"
		}
		event.Data += value
	case "retry":
		if value != "" {
			retryMs, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid retry value: %s", value)
			}
			retry := time.Duration(retryMs) * time.Millisecond
			event.Retry = &retry
			c.config.RetryInterval = retry
		}
	default:
		// Store unknown fields in headers
		event.Headers[field] = value
	}
	return nil
}

// finalizeEvent completes event parsing and sends it for processing
func (c *SSEClient) finalizeEvent(event *SSEEvent, eventLines []string) {
	event.Raw = strings.Join(eventLines, "\n")

	// Update last event ID
	if event.ID != "" {
		c.setLastEventID(event.ID)
	}

	// Apply event filter if configured
	if c.config.EventFilter != nil && !c.config.EventFilter(event.Event) {
		return
	}

	// Handle flow control
	if c.config.FlowControlEnabled {
		c.handleFlowControl(event)
	} else {
		c.sendEvent(event)
	}
}

// handleFlowControl manages backpressure for high-frequency streams
func (c *SSEClient) handleFlowControl(event *SSEEvent) {
	c.bufferMutex.Lock()
	defer c.bufferMutex.Unlock()

	bufferLen := len(c.eventBuffer)
	threshold := int(float64(c.config.EventBufferSize) * c.config.FlowControlThreshold)

	if bufferLen >= threshold {
		c.setBackpressure(true)

		// Buffer management strategy: keep recent events, drop oldest
		if bufferLen >= c.config.EventBufferSize {
			// Remove oldest 10% of events
			removeCount := c.config.EventBufferSize / 10
			if removeCount < 1 {
				removeCount = 1
			}
			copy(c.eventBuffer, c.eventBuffer[removeCount:])
			c.eventBuffer = c.eventBuffer[:bufferLen-removeCount]
		}
	} else if bufferLen < threshold/2 {
		c.setBackpressure(false)
	}

	c.eventBuffer = append(c.eventBuffer, event)
}

// sendEvent sends an event to the event channel
func (c *SSEClient) sendEvent(event *SSEEvent) {
	select {
	case c.eventChan <- event:
	case <-c.ctx.Done():
	default:
		// Channel is full, apply backpressure
		if c.config.FlowControlEnabled {
			c.setBackpressure(true)
			c.bufferMutex.Lock()
			c.eventBuffer = append(c.eventBuffer, event)
			c.bufferMutex.Unlock()
		}
		// If flow control is disabled, drop the event
	}
}

// processEvents processes buffered events when backpressure is released
func (c *SSEClient) processEvents() {
	defer c.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.flushBuffer()
		}
	}
}

// flushBuffer sends buffered events when possible
func (c *SSEClient) flushBuffer() {
	if !c.config.FlowControlEnabled {
		return
	}

	c.bufferMutex.Lock()
	events := make([]*SSEEvent, len(c.eventBuffer))
	copy(events, c.eventBuffer)
	c.eventBuffer = c.eventBuffer[:0]
	c.bufferMutex.Unlock()

	for _, event := range events {
		select {
		case c.eventChan <- event:
		case <-c.ctx.Done():
			return
		default:
			// Channel still full, re-buffer
			c.bufferMutex.Lock()
			c.eventBuffer = append([]*SSEEvent{event}, c.eventBuffer...)
			c.bufferMutex.Unlock()
			return
		}
	}

	if len(events) > 0 {
		c.setBackpressure(false)
	}
}

// healthCheck monitors connection health
func (c *SSEClient) healthCheck() {
	defer c.wg.Done()

	c.healthMutex.Lock()
	c.healthTicker = time.NewTicker(c.config.HealthCheckInterval)
	ticker := c.healthTicker
	c.healthMutex.Unlock()
	defer func() {
		c.healthMutex.Lock()
		if c.healthTicker != nil {
			c.healthTicker.Stop()
			c.healthTicker = nil
		}
		c.healthMutex.Unlock()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.checkConnectionHealth()
		}
	}
}

// checkConnectionHealth verifies the connection is still active
func (c *SSEClient) checkConnectionHealth() {
	lastActivity := c.getLastActivity()

	// Check if we've received data recently
	if time.Since(lastActivity) > c.config.HealthCheckInterval*2 {
		err := fmt.Errorf("connection health check failed: no activity for %v", time.Since(lastActivity))
		c.callOnError(err)
		c.handleConnectionError(err)
	}
}

// handleConnectionError handles connection errors and triggers reconnection
func (c *SSEClient) handleConnectionError(err error) {
	if c.isClosed() {
		return
	}

	c.setState(SSEStateDisconnected)
	c.callOnDisconnect(err)

	// Close current connection
	c.stateMutex.Lock()
	if c.conn != nil {
		c.conn.Body.Close()
		c.conn = nil
	}
	c.stateMutex.Unlock()

	// Attempt reconnection
	if c.shouldReconnect() {
		c.scheduleReconnect()
	}
}

// shouldReconnect determines if reconnection should be attempted
func (c *SSEClient) shouldReconnect() bool {
	if c.isClosed() {
		return false
	}

	reconnectCount := c.ReconnectCount()

	if c.config.MaxReconnectAttempts > 0 && reconnectCount >= c.config.MaxReconnectAttempts {
		return false
	}

	return true
}

// scheduleReconnect schedules a reconnection attempt
func (c *SSEClient) scheduleReconnect() {
	c.setState(SSEStateReconnecting)

	reconnectCount := c.incrementReconnectCount()

	c.callOnReconnect(reconnectCount)

	// Calculate backoff duration
	backoff := c.calculateBackoff()

	c.healthMutex.Lock()
	c.reconnectTimer = time.AfterFunc(backoff, func() {
		if c.isClosed() {
			return
		}

		if err := c.Connect(c.ctx); err != nil {
			c.callOnError(err)
			if c.shouldReconnect() {
				c.scheduleReconnect()
			}
		}
	})
	c.healthMutex.Unlock()
}

// calculateBackoff calculates the next backoff duration
func (c *SSEClient) calculateBackoff() time.Duration {
	// Use server-suggested retry interval if available
	if c.config.RetryInterval > 0 {
		return c.config.RetryInterval
	}

	c.reconnectMutex.Lock()
	defer c.reconnectMutex.Unlock()

	// Exponential backoff with jitter
	backoff := c.currentBackoff
	c.currentBackoff = time.Duration(float64(c.currentBackoff) * c.config.BackoffMultiplier)

	if c.currentBackoff > c.config.MaxBackoff {
		c.currentBackoff = c.config.MaxBackoff
	}

	// Add jitter (±25%)
	jitter := time.Duration(float64(backoff) * 0.25 * (2*math.Pi - 1))
	return backoff + jitter
}

// resetReconnectState resets reconnection state after successful connection
func (c *SSEClient) resetReconnectState() {
	c.resetReconnectCount()

	c.reconnectMutex.Lock()
	c.currentBackoff = c.config.InitialBackoff
	c.reconnectMutex.Unlock()

	c.healthMutex.Lock()
	if c.reconnectTimer != nil {
		c.reconnectTimer.Stop()
		c.reconnectTimer = nil
	}
	c.healthMutex.Unlock()
}

// setState safely updates the connection state
func (c *SSEClient) setState(state SSEConnectionState) {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	c.clientState.state = state
}

// isClosed safely checks if the client is closed
func (c *SSEClient) isClosed() bool {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.clientState.closed
}

// setClosed safely sets the client as closed and returns true if it was not already closed
func (c *SSEClient) setClosed() bool {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	if c.clientState.closed {
		return false
	}
	c.clientState.closed = true
	return true
}

// getNextSequence safely increments and returns the next sequence number
func (c *SSEClient) getNextSequence() uint64 {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	c.clientState.sequence++
	return c.clientState.sequence
}

// setLastEventID safely updates the last event ID
func (c *SSEClient) setLastEventID(eventID string) {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	c.clientState.lastEventID = eventID
}

// getLastActivity safely gets the last activity time
func (c *SSEClient) getLastActivity() time.Time {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.clientState.lastActivity
}

// incrementReconnectCount safely increments and returns the reconnect count
func (c *SSEClient) incrementReconnectCount() int {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	c.clientState.reconnectCount++
	return c.clientState.reconnectCount
}

// resetReconnectCount safely resets the reconnect count
func (c *SSEClient) resetReconnectCount() {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	c.clientState.reconnectCount = 0
}

// setBackpressure safely updates the backpressure state
func (c *SSEClient) setBackpressure(active bool) {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	c.clientState.backpressure = active
}

// updateActivity updates the last activity timestamp
func (c *SSEClient) updateActivity() {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	c.clientState.lastActivity = time.Now()
}

// Callback helpers
func (c *SSEClient) callOnConnect() {
	if c.config.OnConnect != nil {
		c.callbackPool.Submit(c.config.OnConnect)
	}
}

func (c *SSEClient) callOnDisconnect(err error) {
	if c.config.OnDisconnect != nil {
		c.callbackPool.Submit(func() {
			c.config.OnDisconnect(err)
		})
	}
}

func (c *SSEClient) callOnReconnect(attempt int) {
	if c.config.OnReconnect != nil {
		c.callbackPool.Submit(func() {
			c.config.OnReconnect(attempt)
		})
	}
}

func (c *SSEClient) callOnError(err error) {
	if c.config.OnError != nil {
		c.callbackPool.Submit(func() {
			c.config.OnError(err)
		})
	}
}

// ConvertSSEToEvent converts an SSE event to an AG-UI event
func ConvertSSEToEvent(sseEvent *SSEEvent) (events.Event, error) {
	if sseEvent == nil {
		return nil, pkgerrors.NewValidationErrorWithField("sseEvent", "required", "SSE event cannot be nil", sseEvent)
	}

	// Determine event type
	var eventType events.EventType
	if sseEvent.Event != "" {
		eventType = events.EventType(sseEvent.Event)
	} else {
		eventType = events.EventTypeRaw
	}

	// Create base event
	baseEvent := events.NewBaseEvent(eventType)

	// Set timestamp if available
	if !sseEvent.Timestamp.IsZero() {
		baseEvent.SetTimestamp(sseEvent.Timestamp.UnixMilli())
	}

	// Set raw data
	baseEvent.RawEvent = map[string]interface{}{
		"id":       sseEvent.ID,
		"event":    sseEvent.Event,
		"data":     sseEvent.Data,
		"sequence": sseEvent.Sequence,
		"headers":  sseEvent.Headers,
	}

	return baseEvent, nil
}

// ConvertEventToSSE converts an AG-UI event to an SSE event
func ConvertEventToSSE(event events.Event) (*SSEEvent, error) {
	if event == nil {
		return nil, pkgerrors.NewValidationErrorWithField("event", "required", "event cannot be nil", event)
	}

	sseEvent := &SSEEvent{
		Event:     string(event.Type()),
		Timestamp: time.Now(),
		Headers:   make(map[string]string),
	}

	// Set timestamp
	if event.Timestamp() != nil {
		sseEvent.Timestamp = time.UnixMilli(*event.Timestamp())
	}

	// Convert event to JSON data
	jsonData, err := event.ToJSON()
	if err != nil {
		return nil, pkgerrors.NewEncodingError("json_marshal_failed", "failed to marshal event to JSON").
			WithCause(err).WithOperation("json_marshal").WithDetail("event_type", string(event.Type()))
	}
	sseEvent.Data = string(jsonData)

	// Generate ID if not present
	if sseEvent.ID == "" {
		sseEvent.ID = fmt.Sprintf("%s_%d", event.Type(), sseEvent.Timestamp.UnixMilli())
	}

	return sseEvent, nil
}
