package sse

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// ClientState represents the SSE client connection state
type ClientState int32

const (
	StateDisconnected ClientState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateClosed
)

func (s ClientState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// Event represents a parsed SSE event
type Event struct {
	ID        string
	Type      string
	Data      string
	Retry     int
	Timestamp time.Time
}

// ClientConfig holds SSE client configuration
type ClientConfig struct {
	URL     string
	Headers map[string]string
	
	// Reconnection settings
	EnableReconnect      bool
	InitialBackoff       time.Duration
	MaxBackoff           time.Duration
	BackoffMultiplier    float64
	MaxReconnectAttempts int
	
	// Timeouts
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	
	// Buffer settings
	BufferSize int
	
	// Metrics
	EnableMetrics    bool
	MetricsReporter  MetricsReporter
	MetricsInterval  time.Duration
	
	// Callbacks
	OnConnect    func(connectionID string)
	OnDisconnect func(err error)
	OnReconnect  func(attempt int)
	OnError      func(err error)
	
	// HTTP client (optional)
	HTTPClient *http.Client
	
	// Logger
	Logger *logrus.Logger
}

// Client implements an instrumented SSE client
type Client struct {
	config ClientConfig
	
	// State management
	state      atomic.Int32 // ClientState
	connID     atomic.Value // string
	lastEventID atomic.Value // string
	
	// Connection
	httpClient   *http.Client
	response     *http.Response
	reader       *bufio.Reader
	
	// Channels
	events   chan *Event
	stopChan chan struct{}
	doneChan chan struct{}
	
	// Metrics
	health   *SSEHealth
	reporter MetricsReporter
	
	// Synchronization
	mu            sync.RWMutex
	reconnectLock sync.Mutex
	
	// Logger
	logger *logrus.Logger
}

// NewClient creates a new instrumented SSE client
func NewClient(config ClientConfig) (*Client, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("URL is required")
	}
	
	// Set defaults
	if config.InitialBackoff == 0 {
		config.InitialBackoff = time.Second
	}
	if config.MaxBackoff == 0 {
		config.MaxBackoff = 30 * time.Second
	}
	if config.BackoffMultiplier == 0 {
		config.BackoffMultiplier = 2.0
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 10 * time.Second
	}
	if config.BufferSize == 0 {
		config.BufferSize = 100
	}
	if config.MetricsInterval == 0 {
		config.MetricsInterval = 10 * time.Second
	}
	if config.Logger == nil {
		config.Logger = logrus.New()
	}
	
	// Create HTTP client if not provided
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{
			Timeout: config.ConnectTimeout,
		}
	}
	
	client := &Client{
		config:     config,
		httpClient: config.HTTPClient,
		events:     make(chan *Event, config.BufferSize),
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
		health:     NewSSEHealth(),
		reporter:   config.MetricsReporter,
		logger:     config.Logger,
	}
	
	client.state.Store(int32(StateDisconnected))
	client.connID.Store("")
	client.lastEventID.Store("")
	
	// Set up default reporter if metrics are enabled but no reporter provided
	if config.EnableMetrics && client.reporter == nil {
		client.reporter = NewLoggerReporter(config.Logger, "human")
	}
	
	return client, nil
}

// Connect establishes the SSE connection
func (c *Client) Connect(ctx context.Context) error {
	if !c.setState(StateDisconnected, StateConnecting) {
		currentState := ClientState(c.state.Load())
		return fmt.Errorf("cannot connect from state: %s", currentState)
	}
	
	// Generate connection ID
	connID := c.generateConnectionID()
	c.connID.Store(connID)
	
	c.logger.WithFields(logrus.Fields{
		"url":           c.config.URL,
		"connection_id": connID,
	}).Info("Connecting to SSE endpoint")
	
	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", c.config.URL, nil)
	if err != nil {
		c.setState(StateConnecting, StateDisconnected)
		c.health.RecordError(err)
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	
	// Add Last-Event-ID if we have one
	if lastID, ok := c.lastEventID.Load().(string); ok && lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
	}
	
	// Add custom headers
	for key, value := range c.config.Headers {
		req.Header.Set(key, value)
	}
	
	// Make the connection
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.setState(StateConnecting, StateDisconnected)
		c.health.RecordError(err)
		return fmt.Errorf("connection failed: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		c.setState(StateConnecting, StateDisconnected)
		err := fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		c.health.RecordError(err)
		return err
	}
	
	// Store connection
	c.mu.Lock()
	c.response = resp
	c.reader = bufio.NewReader(resp.Body)
	c.mu.Unlock()
	
	// Update state
	c.setState(StateConnecting, StateConnected)
	c.health.RecordConnect(connID)
	
	// Call callback
	if c.config.OnConnect != nil {
		c.config.OnConnect(connID)
	}
	
	// Start processing
	go c.processStream(ctx)
	
	// Start metrics reporting if enabled
	if c.config.EnableMetrics && c.reporter != nil {
		c.reporter.Start(ctx, c.health, c.config.MetricsInterval)
	}
	
	return nil
}

// processStream reads and processes the SSE stream
func (c *Client) processStream(ctx context.Context) {
	defer close(c.doneChan)
	defer c.cleanup()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		default:
			event, err := c.readEvent()
			if err != nil {
				if err == io.EOF {
					c.logger.Info("SSE stream ended")
				} else {
					c.logger.WithError(err).Error("Error reading SSE event")
					c.health.RecordError(err)
				}
				
				// Attempt reconnection if enabled
				if c.config.EnableReconnect && ClientState(c.state.Load()) == StateConnected {
					go c.reconnect(ctx)
				}
				return
			}
			
			if event != nil {
				// Record metrics
				c.health.RecordEvent(len(event.Data))
				
				// Update last event ID
				if event.ID != "" {
					c.lastEventID.Store(event.ID)
				}
				
				// Send event to channel
				select {
				case c.events <- event:
				case <-time.After(time.Second):
					c.logger.Warn("Event channel full, dropping event")
				}
			}
		}
	}
}

// readEvent reads a single SSE event from the stream
func (c *Client) readEvent() (*Event, error) {
	c.mu.RLock()
	reader := c.reader
	c.mu.RUnlock()
	
	if reader == nil {
		return nil, fmt.Errorf("no active connection")
	}
	
	event := &Event{
		Timestamp: time.Now(),
	}
	
	var dataLines []string
	
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		
		line = strings.TrimSpace(line)
		
		// Empty line signals end of event
		if line == "" {
			if len(dataLines) > 0 {
				event.Data = strings.Join(dataLines, "\n")
				return event, nil
			}
			continue
		}
		
		// Parse field
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(line[5:])
			dataLines = append(dataLines, data)
		} else if strings.HasPrefix(line, "id:") {
			event.ID = strings.TrimSpace(line[3:])
		} else if strings.HasPrefix(line, "event:") {
			event.Type = strings.TrimSpace(line[6:])
		} else if strings.HasPrefix(line, "retry:") {
			// Parse retry value if needed
			// For now, we ignore it as we have our own backoff strategy
		} else if strings.HasPrefix(line, ":") {
			// Comment line, ignore
			continue
		}
	}
}

// reconnect attempts to reconnect to the SSE endpoint
func (c *Client) reconnect(ctx context.Context) {
	c.reconnectLock.Lock()
	defer c.reconnectLock.Unlock()
	
	// Check if we're already reconnecting
	if ClientState(c.state.Load()) == StateReconnecting {
		return
	}
	
	c.setState(StateConnected, StateReconnecting)
	
	backoff := c.config.InitialBackoff
	attempt := 0
	
	for {
		attempt++
		c.health.RecordReconnectAttempt()
		
		// Check max attempts
		if c.config.MaxReconnectAttempts > 0 && attempt > c.config.MaxReconnectAttempts {
			c.logger.Error("Max reconnection attempts reached")
			c.setState(StateReconnecting, StateDisconnected)
			if c.config.OnDisconnect != nil {
				c.config.OnDisconnect(fmt.Errorf("max reconnection attempts reached"))
			}
			return
		}
		
		c.logger.WithField("attempt", attempt).Info("Attempting reconnection")
		
		// Call callback
		if c.config.OnReconnect != nil {
			c.config.OnReconnect(attempt)
		}
		
		// Wait before reconnecting
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		}
		
		// Try to connect
		c.setState(StateReconnecting, StateDisconnected)
		err := c.Connect(ctx)
		if err == nil {
			c.logger.Info("Reconnection successful")
			return
		}
		
		c.logger.WithError(err).Warn("Reconnection failed")
		
		// Increase backoff
		backoff = time.Duration(float64(backoff) * c.config.BackoffMultiplier)
		if backoff > c.config.MaxBackoff {
			backoff = c.config.MaxBackoff
		}
	}
}

// Events returns the event channel
func (c *Client) Events() <-chan *Event {
	return c.events
}

// Close closes the SSE connection
func (c *Client) Close() error {
	if !c.setState(StateConnected, StateClosed) && 
	   !c.setState(StateReconnecting, StateClosed) &&
	   !c.setState(StateDisconnected, StateClosed) {
		return fmt.Errorf("already closed")
	}
	
	// Signal shutdown
	close(c.stopChan)
	
	// Wait for processing to complete
	select {
	case <-c.doneChan:
	case <-time.After(5 * time.Second):
		c.logger.Warn("Timeout waiting for stream processing to complete")
	}
	
	// Emit final summary if metrics enabled
	if c.config.EnableMetrics && c.reporter != nil {
		metrics := c.health.GetMetrics()
		if err := c.reporter.ReportSummary(context.Background(), metrics); err != nil {
			c.logger.WithError(err).Warn("Failed to report final metrics")
		}
		c.reporter.Stop()
	}
	
	// Cleanup
	c.cleanup()
	
	// Call disconnect callback
	if c.config.OnDisconnect != nil {
		c.config.OnDisconnect(nil)
	}
	
	return nil
}

// cleanup closes the connection and cleans up resources
func (c *Client) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.response != nil {
		c.response.Body.Close()
		c.response = nil
	}
	c.reader = nil
	
	// Record disconnection
	c.health.RecordDisconnect(nil)
}

// GetState returns the current client state
func (c *Client) GetState() ClientState {
	return ClientState(c.state.Load())
}

// GetConnectionID returns the current connection ID
func (c *Client) GetConnectionID() string {
	if id, ok := c.connID.Load().(string); ok {
		return id
	}
	return ""
}

// GetLastEventID returns the last received event ID
func (c *Client) GetLastEventID() string {
	if id, ok := c.lastEventID.Load().(string); ok {
		return id
	}
	return ""
}

// GetMetrics returns current health metrics
func (c *Client) GetMetrics() Metrics {
	return c.health.GetMetrics()
}

// setState atomically updates the state if it matches the expected value
func (c *Client) setState(expected, new ClientState) bool {
	return c.state.CompareAndSwap(int32(expected), int32(new))
}

// generateConnectionID generates a unique connection ID
func (c *Client) generateConnectionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}