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
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	pkgerrors "github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// SSEEvent represents a parsed Server-Sent Event
type SSEEvent struct {
	ID    string            `json:"id,omitempty"`
	Event string            `json:"event,omitempty"`
	Data  string            `json:"data"`
	Retry *time.Duration    `json:"retry,omitempty"`
	Raw   string            `json:"raw"`
	Headers map[string]string `json:"headers,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Sequence  uint64          `json:"sequence"`
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

// SSEClient is a robust Server-Sent Events streaming client
type SSEClient struct {
	config   SSEClientConfig
	client   *http.Client
	conn     *http.Response
	scanner  *bufio.Scanner
	
	// State management
	state         SSEConnectionState
	stateMutex    sync.RWMutex
	closed        int32
	
	// Event handling
	eventChan     chan *SSEEvent
	eventBuffer   []*SSEEvent
	bufferMutex   sync.RWMutex
	sequence      uint64
	lastEventID   string
	
	// Reconnection
	reconnectCount     int
	currentBackoff     time.Duration
	reconnectTimer     *time.Timer
	reconnectMutex     sync.RWMutex
	
	// Health monitoring
	lastActivity      time.Time
	activityMutex     sync.RWMutex
	healthTicker      *time.Ticker
	
	// Flow control
	backpressure      bool
	backpressureMutex sync.RWMutex
	
	// Context and cancellation
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
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
		state:          SSEStateDisconnected,
		eventChan:      make(chan *SSEEvent, config.EventBufferSize),
		eventBuffer:    make([]*SSEEvent, 0),
		currentBackoff: config.InitialBackoff,
		lastActivity:   time.Now(),
		ctx:            ctx,
		cancel:         cancel,
	}

	// Set last event ID if provided
	if config.LastEventID != "" {
		client.lastEventID = config.LastEventID
	}

	return client, nil
}

// Connect establishes the SSE connection
func (c *SSEClient) Connect(ctx context.Context) error {
	if atomic.LoadInt32(&c.closed) == 1 {
		return pkgerrors.NewOperationError("Connect", "SSEClient", fmt.Errorf("client is closed"))
	}

	c.setState(SSEStateConnecting)

	req, err := c.createRequest(ctx)
	if err != nil {
		c.setState(SSEStateDisconnected)
		return pkgerrors.WrapWithContext(err, "Connect", "create_request")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		c.setState(SSEStateDisconnected)
		c.callOnError(err)
		return pkgerrors.WrapWithContext(err, "Connect", "http_request")
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		c.setState(SSEStateDisconnected)
		err := fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		c.callOnError(err)
		return pkgerrors.NewOperationError("Connect", "http_status", err)
	}

	// Verify content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		resp.Body.Close()
		c.setState(SSEStateDisconnected)
		err := fmt.Errorf("unexpected content type: %s", contentType)
		c.callOnError(err)
		return pkgerrors.NewValidationErrorWithField("Content-Type", "format", "expected text/event-stream", contentType)
	}

	c.stateMutex.Lock()
	c.conn = resp
	c.scanner = bufio.NewScanner(resp.Body)
	c.stateMutex.Unlock()
	c.setState(SSEStateConnected)
	c.updateActivity()
	c.resetReconnectState()

	// Start background goroutines
	c.wg.Add(3)
	go c.readLoop()
	go c.healthCheck()
	go c.processEvents()

	c.callOnConnect()
	return nil
}

// Events returns a channel that receives SSE events
func (c *SSEClient) Events() <-chan *SSEEvent {
	return c.eventChan
}

// Close closes the SSE client and releases resources
func (c *SSEClient) Close() error {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return pkgerrors.NewOperationError("Close", "SSEClient", fmt.Errorf("client already closed"))
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
	c.activityMutex.Lock()
	if c.reconnectTimer != nil {
		c.reconnectTimer.Stop()
		c.reconnectTimer = nil
	}
	c.activityMutex.Unlock()
	c.activityMutex.Lock()
	if c.healthTicker != nil {
		c.healthTicker.Stop()
		c.healthTicker = nil
	}
	c.activityMutex.Unlock()

	// Wait for goroutines
	c.wg.Wait()

	// Close event channel
	close(c.eventChan)

	return nil
}

// State returns the current connection state
func (c *SSEClient) State() SSEConnectionState {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.state
}

// LastEventID returns the last received event ID
func (c *SSEClient) LastEventID() string {
	c.bufferMutex.RLock()
	defer c.bufferMutex.RUnlock()
	return c.lastEventID
}

// ReconnectCount returns the number of reconnection attempts
func (c *SSEClient) ReconnectCount() int {
	c.reconnectMutex.RLock()
	defer c.reconnectMutex.RUnlock()
	return c.reconnectCount
}

// BufferLength returns the current buffer length
func (c *SSEClient) BufferLength() int {
	c.bufferMutex.RLock()
	defer c.bufferMutex.RUnlock()
	return len(c.eventBuffer)
}

// IsBackpressureActive returns true if backpressure is currently active
func (c *SSEClient) IsBackpressureActive() bool {
	c.backpressureMutex.RLock()
	defer c.backpressureMutex.RUnlock()
	return c.backpressure
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
	if c.lastEventID != "" {
		req.Header.Set("Last-Event-ID", c.lastEventID)
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
				c.finalizeEvent(event, eventLines)
				eventLines = eventLines[:0]
				event = nil
			}
			continue
		}

		if event == nil {
			event = &SSEEvent{
				Timestamp: time.Now(),
				Sequence:  atomic.AddUint64(&c.sequence, 1),
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
		c.bufferMutex.Lock()
		c.lastEventID = event.ID
		c.bufferMutex.Unlock()
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

	c.activityMutex.Lock()
	c.healthTicker = time.NewTicker(c.config.HealthCheckInterval)
	ticker := c.healthTicker
	c.activityMutex.Unlock()
	defer func() {
		c.activityMutex.Lock()
		if c.healthTicker != nil {
			c.healthTicker.Stop()
			c.healthTicker = nil
		}
		c.activityMutex.Unlock()
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
	c.activityMutex.RLock()
	lastActivity := c.lastActivity
	c.activityMutex.RUnlock()

	// Check if we've received data recently
	if time.Since(lastActivity) > c.config.HealthCheckInterval*2 {
		err := fmt.Errorf("connection health check failed: no activity for %v", time.Since(lastActivity))
		c.callOnError(err)
		c.handleConnectionError(err)
	}
}

// handleConnectionError handles connection errors and triggers reconnection
func (c *SSEClient) handleConnectionError(err error) {
	if atomic.LoadInt32(&c.closed) == 1 {
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
	if atomic.LoadInt32(&c.closed) == 1 {
		return false
	}

	c.reconnectMutex.RLock()
	reconnectCount := c.reconnectCount
	c.reconnectMutex.RUnlock()

	if c.config.MaxReconnectAttempts > 0 && reconnectCount >= c.config.MaxReconnectAttempts {
		return false
	}

	return true
}

// scheduleReconnect schedules a reconnection attempt
func (c *SSEClient) scheduleReconnect() {
	c.setState(SSEStateReconnecting)
	
	c.reconnectMutex.Lock()
	c.reconnectCount++
	reconnectCount := c.reconnectCount
	c.reconnectMutex.Unlock()
	
	c.callOnReconnect(reconnectCount)

	// Calculate backoff duration
	backoff := c.calculateBackoff()
	
	c.activityMutex.Lock()
	c.reconnectTimer = time.AfterFunc(backoff, func() {
		if atomic.LoadInt32(&c.closed) == 1 {
			return
		}

		if err := c.Connect(c.ctx); err != nil {
			c.callOnError(err)
			if c.shouldReconnect() {
				c.scheduleReconnect()
			}
		}
	})
	c.activityMutex.Unlock()
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
	c.reconnectMutex.Lock()
	c.reconnectCount = 0
	c.currentBackoff = c.config.InitialBackoff
	c.reconnectMutex.Unlock()
	
	c.activityMutex.Lock()
	if c.reconnectTimer != nil {
		c.reconnectTimer.Stop()
		c.reconnectTimer = nil
	}
	c.activityMutex.Unlock()
}

// setState safely updates the connection state
func (c *SSEClient) setState(state SSEConnectionState) {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	c.state = state
}

// setBackpressure safely updates the backpressure state
func (c *SSEClient) setBackpressure(active bool) {
	c.backpressureMutex.Lock()
	defer c.backpressureMutex.Unlock()
	c.backpressure = active
}

// updateActivity updates the last activity timestamp
func (c *SSEClient) updateActivity() {
	c.activityMutex.Lock()
	defer c.activityMutex.Unlock()
	c.lastActivity = time.Now()
}

// Callback helpers
func (c *SSEClient) callOnConnect() {
	if c.config.OnConnect != nil {
		go c.config.OnConnect()
	}
}

func (c *SSEClient) callOnDisconnect(err error) {
	if c.config.OnDisconnect != nil {
		go c.config.OnDisconnect(err)
	}
}

func (c *SSEClient) callOnReconnect(attempt int) {
	if c.config.OnReconnect != nil {
		go c.config.OnReconnect(attempt)
	}
}

func (c *SSEClient) callOnError(err error) {
	if c.config.OnError != nil {
		go c.config.OnError(err)
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
		return nil, pkgerrors.WrapWithContext(err, "ConvertEventToSSE", "json_marshal")
	}
	sseEvent.Data = string(jsonData)

	// Generate ID if not present
	if sseEvent.ID == "" {
		sseEvent.ID = fmt.Sprintf("%s_%d", event.Type(), sseEvent.Timestamp.UnixMilli())
	}

	return sseEvent, nil
}