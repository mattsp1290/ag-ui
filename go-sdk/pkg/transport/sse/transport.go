package sse

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/messages"
	"github.com/ag-ui/go-sdk/pkg/transport/common"
)

// Transport interface defines the methods for event transport
type Transport interface {
	// Send sends an event to the server
	Send(ctx context.Context, event events.Event) error

	// Receive receives events from the server
	Receive(ctx context.Context) (<-chan events.Event, error)

	// Close closes the transport connection
	Close() error
}

// SSETransport implements the Transport interface for Server-Sent Events
type SSETransport struct {
	// Configuration
	baseURL            string
	client             *http.Client
	headers            map[string]string
	backpressureConfig *BackpressureConfig

	// Connection management
	conn      *http.Response
	reader    *bufio.Reader
	connMutex sync.RWMutex

	// Event channels
	eventChan chan events.Event
	errorChan chan error

	// Lifecycle management
	ctx        context.Context
	cancel     context.CancelFunc
	closed     bool
	closeMutex sync.RWMutex

	// Configuration options
	bufferSize     int
	readTimeout    time.Duration
	writeTimeout   time.Duration
	reconnectDelay time.Duration
	maxReconnects  int
	reconnectCount int
	
	// Backpressure tracking
	droppedEvents      int64
	droppedErrors      int64
	backpressureActive bool
	backpressureMutex  sync.RWMutex
	lastDropTime       time.Time
}

// BackpressureConfig configures backpressure behavior
type BackpressureConfig struct {
	// ErrorChannelBuffer is the buffer size for error channel
	ErrorChannelBuffer int
	
	// EventChannelBuffer is the buffer size for event channel
	EventChannelBuffer int
	
	// MaxDroppedEvents is the maximum number of events that can be dropped before taking action
	MaxDroppedEvents int
	
	// DropActionType defines what to do when max dropped events is reached
	DropActionType DropActionType
	
	// EnableBackpressureLogging enables detailed logging of backpressure events
	EnableBackpressureLogging bool
	
	// BackpressureThresholdPercent is the percentage at which to start applying backpressure (0-100)
	BackpressureThresholdPercent int
}

// DropActionType defines actions to take when events are dropped
type DropActionType int

const (
	// DropActionLog logs dropped events but continues
	DropActionLog DropActionType = iota
	
	// DropActionReconnect attempts to reconnect
	DropActionReconnect
	
	// DropActionStop stops the transport
	DropActionStop
)

// DefaultBackpressureConfig returns default backpressure configuration
func DefaultBackpressureConfig() *BackpressureConfig {
	return &BackpressureConfig{
		ErrorChannelBuffer:           200,   // Increased error buffer
		EventChannelBuffer:           5000,  // Much larger event buffer for high throughput
		MaxDroppedEvents:             1000,  // Higher tolerance for dropped events
		DropActionType:               DropActionReconnect,
		EnableBackpressureLogging:    true,
		BackpressureThresholdPercent: 90,    // Higher threshold for backpressure activation
	}
}

// Config holds configuration for SSE transport
type Config struct {
	BaseURL            string
	Headers            map[string]string
	BufferSize         int
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	ReconnectDelay     time.Duration
	MaxReconnects      int
	Client             *http.Client
	BackpressureConfig *BackpressureConfig
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		BaseURL:            "http://localhost:8080",
		Headers:            make(map[string]string),
		BufferSize:         5000,           // Increased buffer size for performance
		ReadTimeout:        120 * time.Second, // Much longer timeout for SSE streams
		WriteTimeout:       60 * time.Second,  // Longer write timeout
		ReconnectDelay:     100 * time.Millisecond, // Faster reconnection
		MaxReconnects:      5,
		BackpressureConfig: DefaultBackpressureConfig(),
		Client: &http.Client{
			Timeout: 120 * time.Second, // Much longer client timeout for SSE
			Transport: &http.Transport{
				MaxIdleConns:        200,        // Higher connection pool
				MaxIdleConnsPerHost: 100,        // Higher per-host connections
				IdleConnTimeout:     300 * time.Second, // Longer idle timeout
				ResponseHeaderTimeout: 60 * time.Second,  // Longer response timeout
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
					CipherSuites: []uint16{
						tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
						tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
						tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
						tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
						tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					},
				},
			},
		},
	}
}

// NewSSETransport creates a new SSE transport with the given configuration
func NewSSETransport(config *Config) (*SSETransport, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if config.BaseURL == "" {
		return nil, fmt.Errorf("baseURL must be provided")
	}

	if config.Client == nil {
		config.Client = &http.Client{
			Timeout: 60 * time.Second,  // Increased fallback timeout
			Transport: &http.Transport{
				MaxIdleConns:        100,        // Increased for concurrency
				MaxIdleConnsPerHost: 50,         // Increased per host
				IdleConnTimeout:     90 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,  // Added response timeout
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
					CipherSuites: []uint16{
						tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
						tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
						tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
						tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
						tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					},
				},
			},
		}
	}
	
	if config.BackpressureConfig == nil {
		config.BackpressureConfig = DefaultBackpressureConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Set default values if not provided
	bufferSize := config.BufferSize
	if bufferSize == 0 {
		if config.BackpressureConfig.EventChannelBuffer > 0 {
			bufferSize = config.BackpressureConfig.EventChannelBuffer
		} else {
			bufferSize = 1000
		}
	}
	
	errorBufferSize := config.BackpressureConfig.ErrorChannelBuffer
	if errorBufferSize == 0 {
		errorBufferSize = 10
	}

	transport := &SSETransport{
		baseURL:            config.BaseURL,
		client:             config.Client,
		headers:            config.Headers,
		backpressureConfig: config.BackpressureConfig,
		eventChan:          make(chan events.Event, bufferSize),
		errorChan:          make(chan error, errorBufferSize),
		ctx:                ctx,
		cancel:             cancel,
		bufferSize:         bufferSize,
		readTimeout:        config.ReadTimeout,
		writeTimeout:       config.WriteTimeout,
		reconnectDelay:     config.ReconnectDelay,
		maxReconnects:      config.MaxReconnects,
	}

	// Set default headers for SSE
	if transport.headers == nil {
		transport.headers = make(map[string]string)
	}
	transport.headers["Accept"] = "text/event-stream"
	transport.headers["Cache-Control"] = "no-cache"
	transport.headers["Connection"] = "keep-alive"

	return transport, nil
}

// Send sends an event to the server via HTTP POST
func (t *SSETransport) Send(ctx context.Context, event events.Event) error {
	if t.isClosed() {
		return messages.NewStreamingError("transport", 0, "transport is closed")
	}

	if event == nil {
		return fmt.Errorf("validation error: event cannot be nil")
	}

	// Validate the event
	if err := event.Validate(); err != nil {
		return fmt.Errorf("validation error: event validation failed: %w", err)
	}

	// Serialize event to JSON
	eventData, err := event.ToJSON()
	if err != nil {
		return messages.NewConversionError("event", "json", string(event.Type()), err.Error())
	}

	// Create HTTP request
	sendURL := t.baseURL + "/events"
	req, err := http.NewRequestWithContext(ctx, "POST", sendURL, bytes.NewReader(eventData))
	if err != nil {
		return messages.NewStreamingError("transport", 0, fmt.Sprintf("failed to create request: %v", err))
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for key, value := range t.headers {
		if key != "Accept" && key != "Cache-Control" && key != "Connection" {
			req.Header.Set(key, value)
		}
	}

	// Apply timeout (default to 10 seconds if not set)
	writeTimeout := t.writeTimeout
	if writeTimeout == 0 {
		writeTimeout = 10 * time.Second
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	req = req.WithContext(timeoutCtx)

	// Send request
	resp, err := t.client.Do(req)
	if err != nil {
		return messages.NewStreamingError("transport", 0, fmt.Sprintf("failed to send event: %v", err))
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return messages.NewStreamingError("transport", 0,
			fmt.Sprintf("server returned status %d: %s", resp.StatusCode, string(bodyBytes)))
	}

	return nil
}

// Receive starts receiving events from the server via SSE
func (t *SSETransport) Receive(ctx context.Context) (<-chan events.Event, error) {
	if t.isClosed() {
		return nil, messages.NewStreamingError("transport", 0, "transport is closed")
	}

	// Start the connection if not already started
	if err := t.connect(ctx); err != nil {
		return nil, err
	}

	// Start reading events in a goroutine
	go t.readEvents()

	return t.eventChan, nil
}

// connect establishes SSE connection
func (t *SSETransport) connect(ctx context.Context) error {
	t.connMutex.Lock()
	defer t.connMutex.Unlock()

	if t.conn != nil {
		return nil // Already connected
	}

	// Create SSE request
	sseURL := t.baseURL + "/events/stream"
	req, err := http.NewRequestWithContext(ctx, "GET", sseURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create SSE request: %w", err)
	}

	// Set SSE headers
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}

	// Send request
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to SSE endpoint: %w", err)
	}

	// Check response status
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return messages.NewStreamingError("transport", 0,
			fmt.Sprintf("SSE connection failed with status %d", resp.StatusCode))
	}

	// Verify content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		resp.Body.Close()
		return messages.NewStreamingError("transport", 0,
			fmt.Sprintf("unexpected content type: %s", contentType))
	}

	t.conn = resp
	t.reader = bufio.NewReader(resp.Body)

	return nil
}

// readEvents reads events from the SSE stream
func (t *SSETransport) readEvents() {
	defer t.closeConnection()

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		// Create a timeout context for each read operation
		readCtx, cancel := context.WithTimeout(t.ctx, 30*time.Second)
		
		// Use a channel to signal completion of read operation
		type readResult struct {
			event events.Event
			err   error
		}
		
		resultChan := make(chan readResult, 1)
		
		// Perform read in a separate goroutine
		go func() {
			defer cancel()
			event, err := t.readEvent()
			select {
			case resultChan <- readResult{event: event, err: err}:
			case <-readCtx.Done():
				// Read was cancelled, don't send result
			}
		}()

		// Wait for either the read to complete or context cancellation
		select {
		case result := <-resultChan:
			cancel() // Clean up the read context
			
			if result.err != nil {
				if !t.isClosed() {
					// Handle error with backpressure control
					t.handleErrorWithBackpressure(result.err)
					
					// Try to reconnect
					if t.shouldReconnect(result.err) {
						if reconnectErr := t.reconnect(); reconnectErr != nil {
							// Handle reconnection error with backpressure control
							if !t.isClosed() {
								t.handleErrorWithBackpressure(reconnectErr)
							}
							return
						}
						continue
					}
				}
				return
			}

			if result.event != nil && !t.isClosed() {
				t.handleEventWithBackpressure(result.event)
			}
			
		case <-readCtx.Done():
			cancel() // Clean up the read context
			// Check if it's the parent context or timeout
			if t.ctx.Err() != nil {
				return // Parent context was cancelled
			}
			// Otherwise, it was a read timeout - continue to try again
			continue
			
		case <-t.ctx.Done():
			cancel() // Clean up the read context
			return
		}
	}
}

// readEvent reads a single event from the SSE stream
func (t *SSETransport) readEvent() (events.Event, error) {
	// Safely access reader with lock to prevent race conditions
	t.connMutex.RLock()
	reader := t.reader
	t.connMutex.RUnlock()
	
	if reader == nil {
		return nil, messages.NewStreamingError("transport", 0, "no active connection")
	}

	var eventType, data, id string
	var retry int

	for {
		// Check for cancellation before each read
		select {
		case <-t.ctx.Done():
			return nil, t.ctx.Err()
		default:
		}

		// Read line with timeout awareness
		line, err := t.readLineWithTimeout()
		if err != nil {
			if err == io.EOF {
				return nil, messages.NewStreamingError("transport", 0, "connection closed")
			}
			if err == context.Canceled || err == context.DeadlineExceeded {
				return nil, err
			}
			return nil, messages.NewStreamingError("transport", 0, fmt.Sprintf("failed to read line: %v", err))
		}

		line = strings.TrimRight(line, "\n\r")

		// Empty line indicates end of event
		if line == "" {
			if data != "" {
				return t.parseSSEEvent(eventType, data, id, retry)
			}
			continue
		}

		// Parse SSE fields
		if strings.HasPrefix(line, "data:") {
			data += strings.TrimPrefix(line, "data:")
			if strings.HasPrefix(data, " ") {
				data = data[1:]
			}
		} else if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "id:") {
			id = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		} else if strings.HasPrefix(line, "retry:") {
			retryStr := strings.TrimSpace(strings.TrimPrefix(line, "retry:"))
			if r, err := time.ParseDuration(retryStr + "ms"); err == nil {
				retry = int(r.Milliseconds())
			}
		}
		// Ignore comment lines (starting with :)
	}
}

// readLineWithTimeout reads a line from the reader with timeout handling
func (t *SSETransport) readLineWithTimeout() (string, error) {
	type readResult struct {
		line string
		err  error
	}
	
	resultChan := make(chan readResult, 1)
	
	// Perform the blocking read in a separate goroutine
	go func() {
		line, err := t.reader.ReadString('\n')
		select {
		case resultChan <- readResult{line: line, err: err}:
		case <-t.ctx.Done():
			// Read was cancelled, don't send result
		}
	}()
	
	// Wait for either the read to complete or context cancellation
	select {
	case result := <-resultChan:
		return result.line, result.err
	case <-t.ctx.Done():
		return "", t.ctx.Err()
	}
}

// parseSSEEvent parses an SSE event into an Event object
func (t *SSETransport) parseSSEEvent(eventType, data, id string, retry int) (events.Event, error) {
	if data == "" {
		return nil, nil // Skip empty events
	}

	// Parse JSON data
	var eventData map[string]interface{}
	if err := json.Unmarshal([]byte(data), &eventData); err != nil {
		return nil, messages.NewConversionError("json", "event", eventType,
			fmt.Sprintf("failed to parse event data: %v", err))
	}

	// Determine event type from data if not specified in SSE event field
	if eventType == "" {
		if typeField, ok := eventData["type"].(string); ok {
			eventType = typeField
		} else {
			eventType = "unknown"
		}
	}

	// Create appropriate event based on type
	event, err := t.createEventFromData(eventType, eventData)
	if err != nil {
		return nil, err
	}

	return event, nil
}

// createEventFromData creates an Event object from parsed data
func (t *SSETransport) createEventFromData(eventType string, data map[string]interface{}) (events.Event, error) {
	switch events.EventType(eventType) {
	case events.EventTypeTextMessageStart:
		return t.parseTextMessageStartEvent(data)
	case events.EventTypeTextMessageContent:
		return t.parseTextMessageContentEvent(data)
	case events.EventTypeTextMessageEnd:
		return t.parseTextMessageEndEvent(data)
	case events.EventTypeToolCallStart:
		return t.parseToolCallStartEvent(data)
	case events.EventTypeToolCallArgs:
		return t.parseToolCallArgsEvent(data)
	case events.EventTypeToolCallEnd:
		return t.parseToolCallEndEvent(data)
	case events.EventTypeStateSnapshot:
		return t.parseStateSnapshotEvent(data)
	case events.EventTypeStateDelta:
		return t.parseStateDeltaEvent(data)
	case events.EventTypeMessagesSnapshot:
		return t.parseMessagesSnapshotEvent(data)
	case events.EventTypeRunStarted:
		return t.parseRunStartedEvent(data)
	case events.EventTypeRunFinished:
		return t.parseRunFinishedEvent(data)
	case events.EventTypeRunError:
		return t.parseRunErrorEvent(data)
	case events.EventTypeStepStarted:
		return t.parseStepStartedEvent(data)
	case events.EventTypeStepFinished:
		return t.parseStepFinishedEvent(data)
	case events.EventTypeRaw:
		return t.parseRawEvent(data)
	case events.EventTypeCustom:
		return t.parseCustomEvent(data)
	default:
		return t.parseUnknownEvent(eventType, data)
	}
}

// Helper methods for parsing specific event types
func (t *SSETransport) parseRunStartedEvent(data map[string]interface{}) (events.Event, error) {
	threadID, _ := data["threadId"].(string)
	runID, _ := data["runId"].(string)

	event := events.NewRunStartedEvent(threadID, runID)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseRunFinishedEvent(data map[string]interface{}) (events.Event, error) {
	threadID, _ := data["threadId"].(string)
	runID, _ := data["runId"].(string)

	event := events.NewRunFinishedEvent(threadID, runID)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseRunErrorEvent(data map[string]interface{}) (events.Event, error) {
	message, _ := data["message"].(string)

	options := []events.RunErrorOption{}

	if code, ok := data["code"].(string); ok {
		options = append(options, events.WithErrorCode(code))
	}

	if runID, ok := data["runId"].(string); ok {
		options = append(options, events.WithRunID(runID))
	}

	event := events.NewRunErrorEvent(message, options...)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseStepStartedEvent(data map[string]interface{}) (events.Event, error) {
	stepName, _ := data["stepName"].(string)

	event := events.NewStepStartedEvent(stepName)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseStepFinishedEvent(data map[string]interface{}) (events.Event, error) {
	stepName, _ := data["stepName"].(string)

	event := events.NewStepFinishedEvent(stepName)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseTextMessageStartEvent(data map[string]interface{}) (events.Event, error) {
	messageID, _ := data["messageId"].(string)

	options := []events.TextMessageStartOption{}

	if role, ok := data["role"].(string); ok {
		options = append(options, events.WithRole(role))
	}

	event := events.NewTextMessageStartEvent(messageID, options...)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseTextMessageContentEvent(data map[string]interface{}) (events.Event, error) {
	messageID, _ := data["messageId"].(string)
	delta, _ := data["delta"].(string)

	event := events.NewTextMessageContentEvent(messageID, delta)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseTextMessageEndEvent(data map[string]interface{}) (events.Event, error) {
	messageID, _ := data["messageId"].(string)

	event := events.NewTextMessageEndEvent(messageID)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseToolCallStartEvent(data map[string]interface{}) (events.Event, error) {
	toolCallID, _ := data["toolCallId"].(string)
	toolCallName, _ := data["toolCallName"].(string)

	options := []events.ToolCallStartOption{}

	if parentMessageID, ok := data["parentMessageId"].(string); ok {
		options = append(options, events.WithParentMessageID(parentMessageID))
	}

	event := events.NewToolCallStartEvent(toolCallID, toolCallName, options...)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseToolCallArgsEvent(data map[string]interface{}) (events.Event, error) {
	toolCallID, _ := data["toolCallId"].(string)
	delta, _ := data["delta"].(string)

	event := events.NewToolCallArgsEvent(toolCallID, delta)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseToolCallEndEvent(data map[string]interface{}) (events.Event, error) {
	toolCallID, _ := data["toolCallId"].(string)

	event := events.NewToolCallEndEvent(toolCallID)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseStateSnapshotEvent(data map[string]interface{}) (events.Event, error) {
	snapshot, ok := data["snapshot"]
	if !ok {
		return nil, messages.NewConversionError("json", "event", "STATE_SNAPSHOT",
			"snapshot field is required")
	}

	event := events.NewStateSnapshotEvent(snapshot)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseStateDeltaEvent(data map[string]interface{}) (events.Event, error) {
	deltaData, ok := data["delta"].([]interface{})
	if !ok {
		return nil, messages.NewConversionError("json", "event", "STATE_DELTA",
			"delta field is required and must be an array")
	}

	var delta []events.JSONPatchOperation
	for _, opData := range deltaData {
		opMap, ok := opData.(map[string]interface{})
		if !ok {
			return nil, messages.NewConversionError("json", "event", "STATE_DELTA",
				"delta operations must be objects")
		}

		op := events.JSONPatchOperation{}

		if opType, ok := opMap["op"].(string); ok {
			op.Op = opType
		}

		if path, ok := opMap["path"].(string); ok {
			op.Path = path
		}

		if value, ok := opMap["value"]; ok {
			op.Value = value
		}

		if from, ok := opMap["from"].(string); ok {
			op.From = from
		}

		delta = append(delta, op)
	}

	event := events.NewStateDeltaEvent(delta)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseMessagesSnapshotEvent(data map[string]interface{}) (events.Event, error) {
	messagesData, ok := data["messages"].([]interface{})
	if !ok {
		return nil, messages.NewConversionError("json", "event", "MESSAGES_SNAPSHOT",
			"messages field is required and must be an array")
	}

	var messagesList []events.Message
	for _, msgData := range messagesData {
		msgMap, ok := msgData.(map[string]interface{})
		if !ok {
			return nil, messages.NewConversionError("json", "event", "MESSAGES_SNAPSHOT",
				"messages must be objects")
		}

		msg := events.Message{}

		if id, ok := msgMap["id"].(string); ok {
			msg.ID = id
		}

		if role, ok := msgMap["role"].(string); ok {
			msg.Role = role
		}

		if content, ok := msgMap["content"].(string); ok {
			msg.Content = &content
		}

		if name, ok := msgMap["name"].(string); ok {
			msg.Name = &name
		}

		if toolCallID, ok := msgMap["toolCallId"].(string); ok {
			msg.ToolCallID = &toolCallID
		}

		// Parse tool calls if present
		if toolCallsData, ok := msgMap["toolCalls"].([]interface{}); ok {
			for _, tcData := range toolCallsData {
				tcMap, ok := tcData.(map[string]interface{})
				if !ok {
					continue
				}

				toolCall := events.ToolCall{}

				if id, ok := tcMap["id"].(string); ok {
					toolCall.ID = id
				}

				if tcType, ok := tcMap["type"].(string); ok {
					toolCall.Type = tcType
				}

				if functionData, ok := tcMap["function"].(map[string]interface{}); ok {
					if name, ok := functionData["name"].(string); ok {
						toolCall.Function.Name = name
					}

					if args, ok := functionData["arguments"].(string); ok {
						toolCall.Function.Arguments = args
					}
				}

				msg.ToolCalls = append(msg.ToolCalls, toolCall)
			}
		}

		messagesList = append(messagesList, msg)
	}

	event := events.NewMessagesSnapshotEvent(messagesList)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseRawEvent(data map[string]interface{}) (events.Event, error) {
	eventData, ok := data["event"]
	if !ok {
		return nil, messages.NewConversionError("json", "event", "RAW",
			"event field is required")
	}

	options := []events.RawEventOption{}

	if source, ok := data["source"].(string); ok {
		options = append(options, events.WithSource(source))
	}

	event := events.NewRawEvent(eventData, options...)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseCustomEvent(data map[string]interface{}) (events.Event, error) {
	name, ok := data["name"].(string)
	if !ok {
		return nil, messages.NewConversionError("json", "event", "CUSTOM",
			"name field is required")
	}

	options := []events.CustomEventOption{}

	if value, ok := data["value"]; ok {
		options = append(options, events.WithValue(value))
	}

	event := events.NewCustomEvent(name, options...)

	// Set timestamp if present
	if timestamp, ok := data["timestamp"].(float64); ok {
		event.SetTimestamp(int64(timestamp))
	}

	return event, nil
}

func (t *SSETransport) parseUnknownEvent(eventType string, data map[string]interface{}) (events.Event, error) {
	return nil, messages.NewConversionError("json", "event", eventType,
		fmt.Sprintf("unknown event type: %s", eventType))
}

// shouldReconnect determines if we should attempt to reconnect
func (t *SSETransport) shouldReconnect(err error) bool {
	if t.isClosed() {
		return false
	}

	if t.reconnectCount >= t.maxReconnects {
		return false
	}

	// Check if it's a recoverable error
	if messages.IsStreamingError(err) {
		return true
	}

	return false
}

// reconnect attempts to reconnect to the SSE endpoint
func (t *SSETransport) reconnect() error {
	t.reconnectCount++

	// Close existing connection
	t.closeConnection()

	// Wait before reconnecting with context cancellation support
	select {
	case <-time.After(t.reconnectDelay):
		// Continue with reconnection
	case <-t.ctx.Done():
		return fmt.Errorf("reconnect cancelled: %w", t.ctx.Err())
	}

	// Attempt to reconnect
	return t.connect(t.ctx)
}

// closeConnection closes the SSE connection
func (t *SSETransport) closeConnection() {
	t.connMutex.Lock()
	defer t.connMutex.Unlock()

	if t.conn != nil {
		t.conn.Body.Close()
		t.conn = nil
		t.reader = nil
	}
}


// Close closes the transport and releases resources
func (t *SSETransport) Close() error {
	t.closeMutex.Lock()
	defer t.closeMutex.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true

	// Cancel context to stop all operations
	t.cancel()

	// Close connection
	t.closeConnection()

	// Give goroutines time to finish and then close channels
	// The readEvents goroutine will exit when context is cancelled
	go func() {
		// Brief delay to allow goroutines to finish
		time.Sleep(100 * time.Millisecond)
		
		// Close channels safely
		defer func() {
			recover() // Handle any panic from closing already-closed channels
		}()
		
		close(t.eventChan)
		close(t.errorChan)
	}()

	return nil
}

// isClosed checks if the transport is closed
func (t *SSETransport) isClosed() bool {
	t.closeMutex.RLock()
	defer t.closeMutex.RUnlock()
	return t.closed
}

// SetHeader sets a custom header for requests
func (t *SSETransport) SetHeader(key, value string) {
	t.connMutex.Lock()
	defer t.connMutex.Unlock()

	if t.headers == nil {
		t.headers = make(map[string]string)
	}
	t.headers[key] = value
}

// GetErrorChannel returns the error channel for monitoring transport errors
func (t *SSETransport) GetErrorChannel() <-chan error {
	return t.errorChan
}

// GetConnectionStatus returns the current connection status
func (t *SSETransport) GetConnectionStatus() ConnectionStatus {
	t.connMutex.RLock()
	defer t.connMutex.RUnlock()

	if t.isClosed() {
		return ConnectionClosed
	}

	if t.conn == nil {
		return ConnectionDisconnected
	}

	return ConnectionConnected
}

// ConnectionStatus represents the connection status
type ConnectionStatus int

const (
	ConnectionDisconnected ConnectionStatus = iota
	ConnectionConnected
	ConnectionClosed
)

// String returns the string representation of the connection status
func (s ConnectionStatus) String() string {
	switch s {
	case ConnectionDisconnected:
		return "disconnected"
	case ConnectionConnected:
		return "connected"
	case ConnectionClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// Additional helper methods for the SSE transport

// FormatSSEEvent formats an event as SSE data
func FormatSSEEvent(event events.Event) (string, error) {
	if event == nil {
		return "", fmt.Errorf("event cannot be nil: %w", common.NewValidationError("event", "required", "event must not be nil", nil))
	}

	eventData, err := event.ToJSON()
	if err != nil {
		return "", messages.NewConversionError("event", "json", string(event.Type()), err.Error())
	}

	var sse strings.Builder
	sse.WriteString(fmt.Sprintf("event: %s\n", event.Type()))
	sse.WriteString(fmt.Sprintf("data: %s\n", string(eventData)))

	if event.Timestamp() != nil {
		sse.WriteString(fmt.Sprintf("id: %d\n", *event.Timestamp()))
	}

	sse.WriteString("\n")

	return sse.String(), nil
}

// WriteSSEEvent writes an SSE event to a writer (useful for server implementations)
func WriteSSEEvent(w io.Writer, event events.Event) error {
	sseData, err := FormatSSEEvent(event)
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(sseData))
	return err
}

// GetStats returns transport statistics
func (t *SSETransport) Stats() TransportStats {
	t.connMutex.RLock()
	defer t.connMutex.RUnlock()

	return TransportStats{
		ConnectionStatus: t.GetConnectionStatus(),
		ReconnectCount:   t.reconnectCount,
		BaseURL:          t.baseURL,
		BufferSize:       t.bufferSize,
		ReadTimeout:      t.readTimeout,
		WriteTimeout:     t.writeTimeout,
	}
}

// TransportStats contains transport statistics and configuration
type TransportStats struct {
	ConnectionStatus ConnectionStatus `json:"connectionStatus"`
	ReconnectCount   int              `json:"reconnectCount"`
	BaseURL          string           `json:"baseUrl"`
	BufferSize       int              `json:"bufferSize"`
	ReadTimeout      time.Duration    `json:"readTimeout"`
	WriteTimeout     time.Duration    `json:"writeTimeout"`
}

// String returns a string representation of the transport stats
func (s TransportStats) String() string {
	return fmt.Sprintf("SSETransport{status=%s, reconnects=%d, baseURL=%s, bufferSize=%d}",
		s.ConnectionStatus, s.ReconnectCount, s.BaseURL, s.BufferSize)
}

// Reset resets the transport to a clean state (useful for testing)
func (t *SSETransport) Reset() error {
	if t.isClosed() {
		return messages.NewStreamingError("transport", 0, "cannot reset closed transport")
	}

	// Close existing connection
	t.closeConnection()

	// Reset reconnect count
	t.connMutex.Lock()
	t.reconnectCount = 0
	t.connMutex.Unlock()

	return nil
}

// Ping sends a ping to the server to check connectivity
func (t *SSETransport) Ping(ctx context.Context) error {
	if t.isClosed() {
		return messages.NewStreamingError("transport", 0, "transport is closed")
	}

	// Create a simple ping request
	pingURL := t.baseURL + "/ping"
	req, err := http.NewRequestWithContext(ctx, "GET", pingURL, nil)
	if err != nil {
		return messages.NewStreamingError("transport", 0, fmt.Sprintf("failed to create ping request: %v", err))
	}

	// Set headers (excluding SSE-specific ones)
	for key, value := range t.headers {
		if key != "Accept" && key != "Cache-Control" && key != "Connection" {
			req.Header.Set(key, value)
		}
	}

	// Apply timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, t.writeTimeout)
	defer cancel()
	req = req.WithContext(timeoutCtx)

	// Send request
	resp, err := t.client.Do(req)
	if err != nil {
		return messages.NewStreamingError("transport", 0, fmt.Sprintf("ping failed: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return messages.NewStreamingError("transport", 0, fmt.Sprintf("ping returned status %d", resp.StatusCode))
	}

	return nil
}

// SendBatch sends multiple events in a single request (useful for bulk operations)
func (t *SSETransport) SendBatch(ctx context.Context, events []events.Event) error {
	if t.isClosed() {
		return messages.NewStreamingError("transport", 0, "transport is closed")
	}

	if len(events) == 0 {
		return fmt.Errorf("events list cannot be empty: %w", common.NewValidationError("events", "required", "events list must contain at least one event", len(events)))
	}

	// Validate all events first and collect validation errors
	batchErr := common.NewBatchError("SendBatch validation", len(events))
	for i, event := range events {
		if event == nil {
			batchErr.AddError(i, fmt.Errorf("event at index %d cannot be nil: %w", i, common.NewValidationError("event", "required", "event must not be nil", nil)))
			continue
		}

		if err := event.Validate(); err != nil {
			batchErr.AddError(i, fmt.Errorf("event at index %d validation failed: %w", i, err))
		}
	}
	
	// Return combined validation errors if any occurred
	if batchErr.HasErrors() {
		return fmt.Errorf("batch validation failed: %w", batchErr)
	}

	// Serialize events to JSON array and collect serialization errors
	var eventDataList []json.RawMessage
	serializationErr := common.NewBatchError("SendBatch serialization", len(events))
	for i, event := range events {
		eventData, err := event.ToJSON()
		if err != nil {
			serializationErr.AddError(i, fmt.Errorf("event at index %d serialization failed: %w", i, err))
			continue
		}
		eventDataList = append(eventDataList, eventData)
	}
	
	// Return combined serialization errors if any occurred
	if serializationErr.HasErrors() {
		return fmt.Errorf("batch serialization failed: %w", serializationErr)
	}

	batchData, err := json.Marshal(eventDataList)
	if err != nil {
		return messages.NewConversionError("event-batch", "json", "batch", err.Error())
	}

	// Create HTTP request
	batchURL := t.baseURL + "/events/batch"
	req, err := http.NewRequestWithContext(ctx, "POST", batchURL, bytes.NewReader(batchData))
	if err != nil {
		return messages.NewStreamingError("transport", 0, fmt.Sprintf("failed to create batch request: %v", err))
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for key, value := range t.headers {
		if key != "Accept" && key != "Cache-Control" && key != "Connection" {
			req.Header.Set(key, value)
		}
	}

	// Apply timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, t.writeTimeout)
	defer cancel()
	req = req.WithContext(timeoutCtx)

	// Send request
	resp, err := t.client.Do(req)
	if err != nil {
		return messages.NewStreamingError("transport", 0, fmt.Sprintf("failed to send batch: %v", err))
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return messages.NewStreamingError("transport", 0,
			fmt.Sprintf("server returned status %d: %s", resp.StatusCode, string(bodyBytes)))
	}

	return nil
}
