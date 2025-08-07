package server

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	pkgerrors "github.com/mattsp1290/ag-ui/go-sdk/pkg/errors"
)

// StreamingServerConfig configures the streaming server
type StreamingServerConfig struct {
	// SSE Configuration
	SSE SSEConfig `yaml:"sse" json:"sse"`

	// WebSocket Configuration
	WebSocket WebSocketConfig `yaml:"websocket" json:"websocket"`

	// Server Configuration
	Address           string        `yaml:"address" json:"address"`
	ReadTimeout       time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout      time.Duration `yaml:"write_timeout" json:"write_timeout"`
	MaxConnections    int           `yaml:"max_connections" json:"max_connections"`
	EnableCompression bool          `yaml:"enable_compression" json:"enable_compression"`
	EnableHealthCheck bool          `yaml:"enable_health_check" json:"enable_health_check"`
	HealthCheckPath   string        `yaml:"health_check_path" json:"health_check_path"`
	EnableMetrics     bool          `yaml:"enable_metrics" json:"enable_metrics"`
	MetricsPath       string        `yaml:"metrics_path" json:"metrics_path"`

	// CORS Configuration
	CORS CORSConfig `yaml:"cors" json:"cors"`

	// Security Configuration
	Security SecurityConfig `yaml:"security" json:"security"`
}

// SSEConfig configures Server-Sent Events
type SSEConfig struct {
	BufferSize        int           `yaml:"buffer_size" json:"buffer_size"`
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval" json:"heartbeat_interval"`
	MaxClients        int           `yaml:"max_clients" json:"max_clients"`
	EnableCompression bool          `yaml:"enable_compression" json:"enable_compression"`
	EventTimeout      time.Duration `yaml:"event_timeout" json:"event_timeout"`
	RetryInterval     time.Duration `yaml:"retry_interval" json:"retry_interval"`
}

// WebSocketConfig configures WebSocket connections
type WebSocketConfig struct {
	BufferSize     int           `yaml:"buffer_size" json:"buffer_size"`
	PingPeriod     time.Duration `yaml:"ping_period" json:"ping_period"`
	PongWait       time.Duration `yaml:"pong_wait" json:"pong_wait"`
	WriteWait      time.Duration `yaml:"write_wait" json:"write_wait"`
	MaxMessageSize int64         `yaml:"max_message_size" json:"max_message_size"`
	MaxClients     int           `yaml:"max_clients" json:"max_clients"`
	Subprotocols   []string      `yaml:"subprotocols" json:"subprotocols"`
}

// CORSConfig configures Cross-Origin Resource Sharing
type CORSConfig struct {
	Enabled      bool     `yaml:"enabled" json:"enabled"`
	AllowOrigins []string `yaml:"allow_origins" json:"allow_origins"`
	AllowMethods []string `yaml:"allow_methods" json:"allow_methods"`
	AllowHeaders []string `yaml:"allow_headers" json:"allow_headers"`
}

// SecurityConfig configures security settings
type SecurityConfig struct {
	EnableRateLimit bool          `yaml:"enable_rate_limit" json:"enable_rate_limit"`
	RateLimit       int           `yaml:"rate_limit" json:"rate_limit"`
	RateLimitWindow time.Duration `yaml:"rate_limit_window" json:"rate_limit_window"`
	// Memory protection for rate limiters
	MaxRateLimiters int           `yaml:"max_rate_limiters" json:"max_rate_limiters"`
	RateLimiterTTL  time.Duration `yaml:"rate_limiter_ttl" json:"rate_limiter_ttl"`
	RequireAuth     bool          `yaml:"require_auth" json:"require_auth"`
	AuthHeaderName  string        `yaml:"auth_header_name" json:"auth_header_name"`
	MaxRequestSize  int64         `yaml:"max_request_size" json:"max_request_size"`
	TrustedProxies  []string      `yaml:"trusted_proxies" json:"trusted_proxies"`
}

// StreamingServer implements server-side event streaming with SSE and WebSocket support
type StreamingServer struct {
	config *StreamingServerConfig

	// HTTP server
	httpServer *http.Server
	mux        *http.ServeMux

	// Client management
	sseClients       map[string]*SSEClient
	websocketClients map[string]*WebSocketClient
	clientsMutex     sync.RWMutex

	// Event broadcasting
	eventBroadcaster *EventBroadcaster

	// Connection management
	connectionManager *ConnectionManager

	// Performance monitoring
	metrics *StreamingMetrics

	// Health monitoring
	healthChecker *HealthChecker

	// Lifecycle management
	ctx        context.Context
	cancel     context.CancelFunc
	shutdownWg sync.WaitGroup
	running    int32 // atomic

	// WebSocket upgrader
	upgrader websocket.Upgrader
}

// SSEClient represents a Server-Sent Events client connection
type SSEClient struct {
	ID            string
	Writer        http.ResponseWriter
	Flusher       http.Flusher
	Context       context.Context
	Cancel        context.CancelFunc
	LastEventID   string
	ConnectedAt   time.Time
	LastActivity  time.Time
	EventChannel  chan *StreamEvent
	EventCount    int64
	BytesSent     int64
	Subscriptions map[string]bool
	Compression   bool
	mutex         sync.RWMutex
	channelClosed int32     // atomic flag to track if channel is closed
	closeOnce     sync.Once // ensures channel is closed only once
}

// SafeCloseEventChannel safely closes the event channel, preventing double-close panics
func (c *SSEClient) SafeCloseEventChannel() {
	c.closeOnce.Do(func() {
		atomic.StoreInt32(&c.channelClosed, 1)
		close(c.EventChannel)
	})
}

// WebSocketClient represents a WebSocket client connection
type WebSocketClient struct {
	ID            string
	Conn          *websocket.Conn
	Context       context.Context
	Cancel        context.CancelFunc
	ConnectedAt   time.Time
	LastActivity  time.Time
	EventChannel  chan *StreamEvent
	EventCount    int64
	BytesSent     int64
	BytesReceived int64
	Subscriptions map[string]bool
	mutex         sync.RWMutex
	channelClosed int32     // atomic flag to track if channel is closed
	closeOnce     sync.Once // ensures channel is closed only once
}

// SafeCloseEventChannel safely closes the event channel, preventing double-close panics
func (c *WebSocketClient) SafeCloseEventChannel() {
	c.closeOnce.Do(func() {
		atomic.StoreInt32(&c.channelClosed, 1)
		close(c.EventChannel)
	})
}

// StreamEvent represents an event that can be streamed to clients
type StreamEvent struct {
	ID        string                 `json:"id,omitempty"`
	Event     string                 `json:"event,omitempty"`
	Data      interface{}            `json:"data"`
	Retry     *int                   `json:"retry,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// EventBroadcaster manages event broadcasting to connected clients
type EventBroadcaster struct {
	eventChannel  chan *BroadcastEvent
	subscriptions map[string]map[string]*ClientSubscription
	subsMutex     sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	config        *StreamingServerConfig
	metrics       *StreamingMetrics
}

// BroadcastEvent represents an event to be broadcast
type BroadcastEvent struct {
	Event     *StreamEvent
	EventType string
	TargetIDs []string // If empty, broadcast to all
	Exclude   []string // Client IDs to exclude
	Multicast bool     // If true, send to multiple specific clients
}

// ClientSubscription represents a client's subscription to an event type
type ClientSubscription struct {
	ClientID    string
	ClientType  string // "sse" or "websocket"
	EventType   string
	LastEventID string
	CreatedAt   time.Time
	Active      bool
}

// ConnectionManager manages client connections and enforces limits
type ConnectionManager struct {
	config          *StreamingServerConfig
	activeSSE       int32 // atomic
	activeWebSocket int32 // atomic
	rateLimiters    *BoundedMap[string, *RateLimiter]
	metrics         *StreamingMetrics
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
	mutex      sync.Mutex
}

// StreamingMetrics collects streaming server metrics
type StreamingMetrics struct {
	// Connection metrics
	SSEConnections       int64 `json:"sse_connections"`
	WebSocketConnections int64 `json:"websocket_connections"`
	TotalConnections     int64 `json:"total_connections"`

	// Event metrics
	EventsSent      int64 `json:"events_sent"`
	EventsReceived  int64 `json:"events_received"`
	BytesSent       int64 `json:"bytes_sent"`
	BytesReceived   int64 `json:"bytes_received"`
	BroadcastEvents int64 `json:"broadcast_events"`
	MulticastEvents int64 `json:"multicast_events"`

	// Performance metrics
	AverageLatency       time.Duration `json:"average_latency"`
	EventsPerSecond      float64       `json:"events_per_second"`
	ConnectionsPerSecond float64       `json:"connections_per_second"`

	// Error metrics
	ConnectionErrors int64 `json:"connection_errors"`
	EventErrors      int64 `json:"event_errors"`
	RateLimitHits    int64 `json:"rate_limit_hits"`

	// Buffer metrics
	BufferUtilization float64 `json:"buffer_utilization"`
	DroppedEvents     int64   `json:"dropped_events"`

	// Health metrics
	Uptime        time.Duration `json:"uptime"`
	StartTime     time.Time     `json:"start_time"`
	LastEventTime time.Time     `json:"last_event_time"`

	mutex sync.RWMutex
}

// HealthChecker performs health checks on the streaming server
type HealthChecker struct {
	server    *StreamingServer
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	interval  time.Duration
	healthy   int32 // atomic
	lastCheck time.Time
	mutex     sync.RWMutex
}

// DefaultStreamingServerConfig returns a default configuration
func DefaultStreamingServerConfig() *StreamingServerConfig {
	return &StreamingServerConfig{
		SSE: SSEConfig{
			BufferSize:        1000,
			HeartbeatInterval: 30 * time.Second,
			MaxClients:        1000,
			EnableCompression: true,
			EventTimeout:      10 * time.Second,
			RetryInterval:     3 * time.Second,
		},
		WebSocket: WebSocketConfig{
			BufferSize:     1024,
			PingPeriod:     54 * time.Second,
			PongWait:       60 * time.Second,
			WriteWait:      10 * time.Second,
			MaxMessageSize: 512 * 1024, // 512KB
			MaxClients:     1000,
			Subprotocols:   []string{"ag-ui-v1"},
		},
		Address:           ":8080",
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		MaxConnections:    2000,
		EnableCompression: true,
		EnableHealthCheck: true,
		HealthCheckPath:   "/health",
		EnableMetrics:     true,
		MetricsPath:       "/metrics",
		CORS: CORSConfig{
			Enabled:      true,
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"GET", "POST", "OPTIONS"},
			AllowHeaders: []string{"Content-Type", "Authorization", "Last-Event-ID", "Cache-Control"},
		},
		Security: SecurityConfig{
			EnableRateLimit: true,
			RateLimit:       100,
			RateLimitWindow: time.Minute,
			MaxRateLimiters: 10000,
			RateLimiterTTL:  10 * time.Minute,
			RequireAuth:     false,
			AuthHeaderName:  "Authorization",
			MaxRequestSize:  1024 * 1024, // 1MB
			TrustedProxies:  []string{"127.0.0.1", "::1"},
		},
	}
}

// NewStreamingServer creates a new streaming server
func NewStreamingServer(config *StreamingServerConfig) (*StreamingServer, error) {
	if config == nil {
		config = DefaultStreamingServerConfig()
	}

	if err := validateConfig(config); err != nil {
		return nil, pkgerrors.NewValidationError("invalid_config", "streaming server configuration validation failed").WithCause(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	server := &StreamingServer{
		config:           config,
		sseClients:       make(map[string]*SSEClient),
		websocketClients: make(map[string]*WebSocketClient),
		ctx:              ctx,
		cancel:           cancel,
		mux:              http.NewServeMux(),
	}

	// Initialize upgrader after server is created so we can reference server.checkOrigin
	server.upgrader = websocket.Upgrader{
		ReadBufferSize:    config.WebSocket.BufferSize,
		WriteBufferSize:   config.WebSocket.BufferSize,
		CheckOrigin:       server.checkOrigin,
		Subprotocols:      config.WebSocket.Subprotocols,
		EnableCompression: config.EnableCompression,
	}

	// Initialize components
	server.eventBroadcaster = NewEventBroadcaster(config, ctx)
	server.connectionManager = NewConnectionManager(config)
	server.metrics = NewStreamingMetrics()

	if config.EnableHealthCheck {
		server.healthChecker = NewHealthChecker(server, config)
	}

	// Setup HTTP routes
	server.setupRoutes()

	return server, nil
}

// Start starts the streaming server
func (s *StreamingServer) Start() error {
	if !atomic.CompareAndSwapInt32(&s.running, 0, 1) {
		return pkgerrors.NewOperationError("Start", "StreamingServer", fmt.Errorf("server is already running"))
	}

	// Start event broadcaster
	s.eventBroadcaster.Start()

	// Start health checker
	if s.healthChecker != nil {
		s.healthChecker.Start()
	}

	// Start metrics collection
	s.metrics.StartTime = time.Now()

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         s.config.Address,
		Handler:      s.mux,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	// Start server in goroutine
	s.shutdownWg.Add(1)
	go func() {
		defer s.shutdownWg.Done()
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Streaming server error: %v\n", err)
		}
	}()

	fmt.Printf("Streaming server started on %s\n", s.config.Address)
	return nil
}

// Stop stops the streaming server gracefully
func (s *StreamingServer) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&s.running, 1, 0) {
		return pkgerrors.NewOperationError("Stop", "StreamingServer", fmt.Errorf("server is not running"))
	}

	// Cancel context
	s.cancel()

	// Stop health checker
	if s.healthChecker != nil {
		s.healthChecker.Stop()
	}

	// Stop event broadcaster
	s.eventBroadcaster.Stop()

	// Close all client connections
	s.closeAllClients()

	// Shutdown HTTP server
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			return pkgerrors.NewOperationError("Stop", "HTTPServer", err)
		}
	}

	// Wait for shutdown completion
	s.shutdownWg.Wait()

	return nil
}

// BroadcastEvent broadcasts an event to all connected clients
func (s *StreamingServer) BroadcastEvent(event *StreamEvent) error {
	if atomic.LoadInt32(&s.running) == 0 {
		return pkgerrors.NewOperationError("BroadcastEvent", "StreamingServer", fmt.Errorf("server is not running"))
	}

	broadcastEvent := &BroadcastEvent{
		Event:     event,
		EventType: event.Event,
		Multicast: false,
	}

	return s.eventBroadcaster.Broadcast(broadcastEvent)
}

// MulticastEvent sends an event to specific clients
func (s *StreamingServer) MulticastEvent(event *StreamEvent, clientIDs []string) error {
	if atomic.LoadInt32(&s.running) == 0 {
		return pkgerrors.NewOperationError("MulticastEvent", "StreamingServer", fmt.Errorf("server is not running"))
	}

	broadcastEvent := &BroadcastEvent{
		Event:     event,
		EventType: event.Event,
		TargetIDs: clientIDs,
		Multicast: true,
	}

	return s.eventBroadcaster.Broadcast(broadcastEvent)
}

// GetMetrics returns current streaming metrics
func (s *StreamingServer) GetMetrics() *StreamingMetrics {
	s.metrics.mutex.RLock()
	defer s.metrics.mutex.RUnlock()

	// Create a copy without the mutex to avoid copying lock value
	metrics := &StreamingMetrics{
		SSEConnections:       s.metrics.SSEConnections,
		WebSocketConnections: s.metrics.WebSocketConnections,
		TotalConnections:     s.metrics.TotalConnections,
		EventsSent:           s.metrics.EventsSent,
		EventsReceived:       s.metrics.EventsReceived,
		BytesSent:            s.metrics.BytesSent,
		BytesReceived:        s.metrics.BytesReceived,
		BroadcastEvents:      s.metrics.BroadcastEvents,
		MulticastEvents:      s.metrics.MulticastEvents,
		AverageLatency:       s.metrics.AverageLatency,
		EventsPerSecond:      s.metrics.EventsPerSecond,
		ConnectionsPerSecond: s.metrics.ConnectionsPerSecond,
		ConnectionErrors:     s.metrics.ConnectionErrors,
		EventErrors:          s.metrics.EventErrors,
		RateLimitHits:        s.metrics.RateLimitHits,
		BufferUtilization:    s.metrics.BufferUtilization,
		DroppedEvents:        s.metrics.DroppedEvents,
		Uptime:               time.Since(s.metrics.StartTime), // Calculate uptime
		StartTime:            s.metrics.StartTime,
		LastEventTime:        s.metrics.LastEventTime,
		// Note: deliberately omitting mutex field to avoid copying the lock
	}

	return metrics
}

// GetActiveConnections returns the number of active connections
func (s *StreamingServer) GetActiveConnections() (int, int) {
	s.clientsMutex.RLock()
	defer s.clientsMutex.RUnlock()

	return len(s.sseClients), len(s.websocketClients)
}

// setupRoutes configures HTTP routes
func (s *StreamingServer) setupRoutes() {
	// SSE endpoint
	s.mux.HandleFunc("/events", s.handleSSE)

	// WebSocket endpoint
	s.mux.HandleFunc("/ws", s.handleWebSocket)

	// Health check endpoint
	if s.config.EnableHealthCheck {
		s.mux.HandleFunc(s.config.HealthCheckPath, s.handleHealth)
	}

	// Metrics endpoint
	if s.config.EnableMetrics {
		s.mux.HandleFunc(s.config.MetricsPath, s.handleMetrics)
	}

	// CORS preflight
	if s.config.CORS.Enabled {
		s.mux.HandleFunc("/", s.handleCORS)
	}
}

// handleSSE handles Server-Sent Events connections
func (s *StreamingServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	// Check if server is running
	if atomic.LoadInt32(&s.running) == 0 {
		http.Error(w, "Server not running", http.StatusServiceUnavailable)
		return
	}

	// Apply CORS headers
	if s.config.CORS.Enabled {
		s.applyCORSHeaders(w, r)
	}

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only allow GET requests
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check connection limits
	if !s.connectionManager.CanAcceptSSEConnection() {
		http.Error(w, "Too many connections", http.StatusTooManyRequests)
		return
	}

	// Apply rate limiting
	if !s.connectionManager.AllowRequest(s.getClientIP(r)) {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		s.metrics.IncrementRateLimitHits()
		return
	}

	// Check for flusher support
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

	// Enable compression if requested and configured
	enableCompression := s.config.SSE.EnableCompression &&
		strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")

	var writer http.ResponseWriter = w
	var gzipWriter *gzip.Writer

	if enableCompression {
		w.Header().Set("Content-Encoding", "gzip")
		gzipWriter = gzip.NewWriter(w)
		writer = &gzipResponseWriter{ResponseWriter: w, gzipWriter: gzipWriter}
	}

	// Get last event ID for resuming connections
	lastEventID := r.Header.Get("Last-Event-ID")
	if lastEventID == "" {
		lastEventID = r.URL.Query().Get("lastEventId")
	}

	// Create client
	clientCtx, clientCancel := context.WithCancel(s.ctx)
	client := &SSEClient{
		ID:            generateClientID("sse"),
		Writer:        writer,
		Flusher:       flusher,
		Context:       clientCtx,
		Cancel:        clientCancel,
		LastEventID:   lastEventID,
		ConnectedAt:   time.Now(),
		LastActivity:  time.Now(),
		EventChannel:  make(chan *StreamEvent, s.config.SSE.BufferSize),
		Subscriptions: make(map[string]bool),
		Compression:   enableCompression,
	}

	// Register client
	s.clientsMutex.Lock()
	s.sseClients[client.ID] = client
	s.clientsMutex.Unlock()

	// Update metrics
	s.connectionManager.IncrementSSEConnections()
	s.metrics.IncrementSSEConnections()

	// Subscribe to events based on query parameters
	eventTypes := r.URL.Query()["events"]
	if len(eventTypes) == 0 {
		eventTypes = []string{"*"} // Subscribe to all events by default
	}

	for _, eventType := range eventTypes {
		s.eventBroadcaster.Subscribe(client.ID, "sse", eventType)
		client.Subscriptions[eventType] = true
	}

	// Send connection event
	connectionEvent := &StreamEvent{
		ID:        generateEventID(),
		Event:     "connection",
		Data:      map[string]interface{}{"status": "connected", "client_id": client.ID},
		Timestamp: time.Now(),
	}

	if err := s.sendSSEEvent(client, connectionEvent); err != nil {
		fmt.Printf("Error sending connection event: %v\n", err)
	}

	// Send buffered events if resuming
	if lastEventID != "" {
		// TODO: Implement event replay from last event ID
	}

	// Start heartbeat
	heartbeatTicker := time.NewTicker(s.config.SSE.HeartbeatInterval)
	defer heartbeatTicker.Stop()

	// Event processing loop
	for {
		select {
		case <-s.ctx.Done():
			goto cleanup
		case <-clientCtx.Done():
			goto cleanup
		case <-r.Context().Done():
			goto cleanup
		case event := <-client.EventChannel:
			if err := s.sendSSEEvent(client, event); err != nil {
				fmt.Printf("Error sending event to SSE client %s: %v\n", client.ID, err)
				goto cleanup
			}
		case <-heartbeatTicker.C:
			heartbeat := &StreamEvent{
				Event:     "heartbeat",
				Data:      map[string]interface{}{"timestamp": time.Now().Unix()},
				Timestamp: time.Now(),
			}
			if err := s.sendSSEEvent(client, heartbeat); err != nil {
				fmt.Printf("Error sending heartbeat to SSE client %s: %v\n", client.ID, err)
				goto cleanup
			}
		}
	}

cleanup:
	// Cleanup client
	clientCancel()

	// Close gzip writer if used
	if gzipWriter != nil {
		gzipWriter.Close()
	}

	// Unregister client
	s.clientsMutex.Lock()
	delete(s.sseClients, client.ID)
	s.clientsMutex.Unlock()

	// Unsubscribe from events
	for eventType := range client.Subscriptions {
		s.eventBroadcaster.Unsubscribe(client.ID, eventType)
	}

	// Update metrics
	s.connectionManager.DecrementSSEConnections()
	s.metrics.DecrementSSEConnections()

	// Close event channel safely
	client.SafeCloseEventChannel()
}

// handleWebSocket handles WebSocket connections
func (s *StreamingServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check if server is running
	if atomic.LoadInt32(&s.running) == 0 {
		http.Error(w, "Server not running", http.StatusServiceUnavailable)
		return
	}

	// Apply CORS headers
	if s.config.CORS.Enabled {
		s.applyCORSHeaders(w, r)
	}

	// Check connection limits
	if !s.connectionManager.CanAcceptWebSocketConnection() {
		http.Error(w, "Too many connections", http.StatusTooManyRequests)
		return
	}

	// Apply rate limiting
	if !s.connectionManager.AllowRequest(s.getClientIP(r)) {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		s.metrics.IncrementRateLimitHits()
		return
	}

	// Upgrade to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade error: %v\n", err)
		s.metrics.IncrementConnectionErrors()
		return
	}

	// Set connection limits
	conn.SetReadLimit(s.config.WebSocket.MaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(s.config.WebSocket.PongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(s.config.WebSocket.PongWait))
		return nil
	})

	// Create client
	clientCtx, clientCancel := context.WithCancel(s.ctx)
	client := &WebSocketClient{
		ID:            generateClientID("ws"),
		Conn:          conn,
		Context:       clientCtx,
		Cancel:        clientCancel,
		ConnectedAt:   time.Now(),
		LastActivity:  time.Now(),
		EventChannel:  make(chan *StreamEvent, s.config.WebSocket.BufferSize),
		Subscriptions: make(map[string]bool),
	}

	// Register client
	s.clientsMutex.Lock()
	s.websocketClients[client.ID] = client
	s.clientsMutex.Unlock()

	// Update metrics
	s.connectionManager.IncrementWebSocketConnections()
	s.metrics.IncrementWebSocketConnections()

	// Send connection event
	connectionEvent := &StreamEvent{
		ID:        generateEventID(),
		Event:     "connection",
		Data:      map[string]interface{}{"status": "connected", "client_id": client.ID},
		Timestamp: time.Now(),
	}

	if err := s.sendWebSocketEvent(client, connectionEvent); err != nil {
		fmt.Printf("Error sending connection event: %v\n", err)
	}

	// Start ping ticker
	pingTicker := time.NewTicker(s.config.WebSocket.PingPeriod)
	defer pingTicker.Stop()

	// Start read and write goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	// Read goroutine
	go func() {
		defer wg.Done()
		s.handleWebSocketRead(client)
	}()

	// Write goroutine
	go func() {
		defer wg.Done()
		s.handleWebSocketWrite(client, pingTicker)
	}()

	// Wait for goroutines to complete
	wg.Wait()

	// Cleanup
	clientCancel()
	conn.Close()

	// Unregister client
	s.clientsMutex.Lock()
	delete(s.websocketClients, client.ID)
	s.clientsMutex.Unlock()

	// Unsubscribe from events
	for eventType := range client.Subscriptions {
		s.eventBroadcaster.Unsubscribe(client.ID, eventType)
	}

	// Update metrics
	s.connectionManager.DecrementWebSocketConnections()
	s.metrics.DecrementWebSocketConnections()

	// Close event channel safely
	client.SafeCloseEventChannel()
}

// handleWebSocketRead handles reading messages from WebSocket clients
func (s *StreamingServer) handleWebSocketRead(client *WebSocketClient) {
	defer client.Cancel()

	for {
		select {
		case <-client.Context.Done():
			return
		default:
		}

		messageType, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("WebSocket read error: %v\n", err)
			}
			return
		}

		client.mutex.Lock()
		client.LastActivity = time.Now()
		client.BytesReceived += int64(len(message))
		client.mutex.Unlock()

		// Update metrics
		s.metrics.IncrementBytesReceived(int64(len(message)))
		s.metrics.IncrementEventsReceived()

		// Handle different message types
		switch messageType {
		case websocket.TextMessage:
			s.handleWebSocketTextMessage(client, message)
		case websocket.BinaryMessage:
			s.handleWebSocketBinaryMessage(client, message)
		}
	}
}

// handleWebSocketWrite handles writing messages to WebSocket clients
func (s *StreamingServer) handleWebSocketWrite(client *WebSocketClient, pingTicker *time.Ticker) {
	defer client.Cancel()

	for {
		select {
		case <-client.Context.Done():
			return
		case event := <-client.EventChannel:
			if err := s.sendWebSocketEvent(client, event); err != nil {
				fmt.Printf("Error sending event to WebSocket client %s: %v\n", client.ID, err)
				return
			}
		case <-pingTicker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(s.config.WebSocket.WriteWait))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleWebSocketTextMessage handles text messages from WebSocket clients
func (s *StreamingServer) handleWebSocketTextMessage(client *WebSocketClient, message []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		fmt.Printf("Error parsing WebSocket message: %v\n", err)
		return
	}

	// Handle subscription requests
	if action, ok := msg["action"].(string); ok {
		switch action {
		case "subscribe":
			if eventType, ok := msg["event_type"].(string); ok {
				s.eventBroadcaster.Subscribe(client.ID, "websocket", eventType)
				client.mutex.Lock()
				client.Subscriptions[eventType] = true
				client.mutex.Unlock()
			}
		case "unsubscribe":
			if eventType, ok := msg["event_type"].(string); ok {
				s.eventBroadcaster.Unsubscribe(client.ID, eventType)
				client.mutex.Lock()
				delete(client.Subscriptions, eventType)
				client.mutex.Unlock()
			}
		}
	}
}

// handleWebSocketBinaryMessage handles binary messages from WebSocket clients
func (s *StreamingServer) handleWebSocketBinaryMessage(client *WebSocketClient, message []byte) {
	// Handle binary messages (e.g., compressed data, protobuf)
	// For now, just log
	fmt.Printf("Received binary message from client %s: %d bytes\n", client.ID, len(message))
}

// sendSSEEvent sends an event to an SSE client
func (s *StreamingServer) sendSSEEvent(client *SSEClient, event *StreamEvent) error {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	// Set write deadline
	if err := setWriteDeadline(client.Writer, s.config.SSE.EventTimeout); err != nil {
		return err
	}

	// Format SSE event
	var data []byte
	var err error

	if event.Data != nil {
		data, err = json.Marshal(event.Data)
		if err != nil {
			return pkgerrors.NewEncodingError("json_marshal_failed", "failed to marshal event data").WithCause(err)
		}
	}

	// Write event fields
	if event.ID != "" {
		if _, err := fmt.Fprintf(client.Writer, "id: %s\n", event.ID); err != nil {
			return err
		}
		client.LastEventID = event.ID
	}

	if event.Event != "" {
		if _, err := fmt.Fprintf(client.Writer, "event: %s\n", event.Event); err != nil {
			return err
		}
	}

	if event.Retry != nil {
		if _, err := fmt.Fprintf(client.Writer, "retry: %d\n", *event.Retry); err != nil {
			return err
		}
	}

	// Write data (can be multi-line)
	if data != nil {
		dataStr := string(data)
		for _, line := range strings.Split(dataStr, "\n") {
			if _, err := fmt.Fprintf(client.Writer, "data: %s\n", line); err != nil {
				return err
			}
		}
	}

	// End event with blank line
	if _, err := fmt.Fprint(client.Writer, "\n"); err != nil {
		return err
	}

	// Flush
	client.Flusher.Flush()

	// Update client stats
	client.LastActivity = time.Now()
	client.EventCount++
	client.BytesSent += int64(len(data) + 50) // Approximate SSE overhead

	// Update metrics
	s.metrics.IncrementEventsSent()
	s.metrics.IncrementBytesSent(int64(len(data)))

	return nil
}

// sendWebSocketEvent sends an event to a WebSocket client
func (s *StreamingServer) sendWebSocketEvent(client *WebSocketClient, event *StreamEvent) error {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	// Set write deadline
	client.Conn.SetWriteDeadline(time.Now().Add(s.config.WebSocket.WriteWait))

	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return pkgerrors.NewEncodingError("json_marshal_failed", "failed to marshal event").WithCause(err)
	}

	// Send message
	if err := client.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return err
	}

	// Update client stats
	client.LastActivity = time.Now()
	client.EventCount++
	client.BytesSent += int64(len(data))

	// Update metrics
	s.metrics.IncrementEventsSent()
	s.metrics.IncrementBytesSent(int64(len(data)))

	return nil
}

// handleHealth handles health check requests
func (s *StreamingServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	healthy := atomic.LoadInt32(&s.running) == 1
	if s.healthChecker != nil {
		healthy = healthy && atomic.LoadInt32(&s.healthChecker.healthy) == 1
	}

	status := "healthy"
	code := http.StatusOK

	if !healthy {
		status = "unhealthy"
		code = http.StatusServiceUnavailable
	}

	sseCount, wsCount := s.GetActiveConnections()
	metrics := s.GetMetrics()

	response := map[string]interface{}{
		"status":    status,
		"timestamp": time.Now().UTC(),
		"uptime":    metrics.Uptime.String(),
		"connections": map[string]interface{}{
			"sse":       sseCount,
			"websocket": wsCount,
			"total":     sseCount + wsCount,
		},
		"metrics": map[string]interface{}{
			"events_sent":     metrics.EventsSent,
			"events_received": metrics.EventsReceived,
			"bytes_sent":      metrics.BytesSent,
			"bytes_received":  metrics.BytesReceived,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(response)
}

// handleMetrics handles metrics requests
func (s *StreamingServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics := s.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metrics)
}

// handleCORS handles CORS preflight requests
func (s *StreamingServer) handleCORS(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		s.applyCORSHeaders(w, r)
		w.WriteHeader(http.StatusOK)
		return
	}

	http.NotFound(w, r)
}

// applyCORSHeaders applies CORS headers to the response
func (s *StreamingServer) applyCORSHeaders(w http.ResponseWriter, r *http.Request) {
	if !s.config.CORS.Enabled {
		return
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}

	// Check allowed origins
	allowed := false
	for _, allowedOrigin := range s.config.CORS.AllowOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			allowed = true
			break
		}
	}

	if !allowed {
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(s.config.CORS.AllowMethods, ", "))
	w.Header().Set("Access-Control-Allow-Headers", strings.Join(s.config.CORS.AllowHeaders, ", "))
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}

// checkOrigin checks if the WebSocket origin is allowed
func (s *StreamingServer) checkOrigin(r *http.Request) bool {
	if !s.config.CORS.Enabled {
		return true
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	for _, allowedOrigin := range s.config.CORS.AllowOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			return true
		}
	}

	return false
}

// closeAllClients closes all connected clients
func (s *StreamingServer) closeAllClients() {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	// Close SSE clients
	for _, client := range s.sseClients {
		client.Cancel()
	}

	// Close WebSocket clients
	for _, client := range s.websocketClients {
		client.Cancel()
		client.Conn.Close()
	}
}

// getClientIP extracts the client IP address from the request
func (s *StreamingServer) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return strings.Split(r.RemoteAddr, ":")[0]
}

// validateConfig validates the streaming server configuration
func validateConfig(config *StreamingServerConfig) error {
	if config.Address == "" {
		return fmt.Errorf("address cannot be empty")
	}

	if config.MaxConnections <= 0 {
		return fmt.Errorf("max_connections must be positive")
	}

	if config.SSE.BufferSize <= 0 {
		return fmt.Errorf("sse.buffer_size must be positive")
	}

	if config.WebSocket.BufferSize <= 0 {
		return fmt.Errorf("websocket.buffer_size must be positive")
	}

	if config.WebSocket.MaxMessageSize <= 0 {
		return fmt.Errorf("websocket.max_message_size must be positive")
	}

	return nil
}

// generateClientID generates a unique client ID
func generateClientID(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), atomic.AddInt64(new(int64), 1))
}

// generateEventID generates a unique event ID
func generateEventID() string {
	return fmt.Sprintf("event_%d_%d", time.Now().UnixNano(), atomic.AddInt64(new(int64), 1))
}

// setWriteDeadline sets a write deadline on the response writer if supported
func setWriteDeadline(w http.ResponseWriter, timeout time.Duration) error {
	if timeout <= 0 {
		return nil
	}

	if conn, ok := w.(interface{ SetWriteDeadline(time.Time) error }); ok {
		return conn.SetWriteDeadline(time.Now().Add(timeout))
	}

	return nil
}

// gzipResponseWriter wraps a ResponseWriter with gzip compression
type gzipResponseWriter struct {
	http.ResponseWriter
	gzipWriter *gzip.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.gzipWriter.Write(b)
}

// NewEventBroadcaster creates a new event broadcaster
func NewEventBroadcaster(config *StreamingServerConfig, ctx context.Context) *EventBroadcaster {
	broadcastCtx, cancel := context.WithCancel(ctx)

	return &EventBroadcaster{
		eventChannel:  make(chan *BroadcastEvent, config.SSE.BufferSize*2),
		subscriptions: make(map[string]map[string]*ClientSubscription),
		ctx:           broadcastCtx,
		cancel:        cancel,
		config:        config,
		metrics:       NewStreamingMetrics(),
	}
}

// Start starts the event broadcaster
func (eb *EventBroadcaster) Start() {
	eb.wg.Add(1)
	go eb.run()
}

// Stop stops the event broadcaster
func (eb *EventBroadcaster) Stop() {
	eb.cancel()
	eb.wg.Wait()
}

// Broadcast broadcasts an event
func (eb *EventBroadcaster) Broadcast(event *BroadcastEvent) error {
	select {
	case eb.eventChannel <- event:
		return nil
	case <-eb.ctx.Done():
		return pkgerrors.NewOperationError("Broadcast", "EventBroadcaster", eb.ctx.Err())
	default:
		// Channel is full, drop event or apply backpressure
		eb.metrics.IncrementDroppedEvents()
		return pkgerrors.NewResourceLimitError("event channel is full", nil)
	}
}

// Subscribe subscribes a client to an event type
func (eb *EventBroadcaster) Subscribe(clientID, clientType, eventType string) {
	eb.subsMutex.Lock()
	defer eb.subsMutex.Unlock()

	if eb.subscriptions[eventType] == nil {
		eb.subscriptions[eventType] = make(map[string]*ClientSubscription)
	}

	eb.subscriptions[eventType][clientID] = &ClientSubscription{
		ClientID:   clientID,
		ClientType: clientType,
		EventType:  eventType,
		CreatedAt:  time.Now(),
		Active:     true,
	}
}

// Unsubscribe unsubscribes a client from an event type
func (eb *EventBroadcaster) Unsubscribe(clientID, eventType string) {
	eb.subsMutex.Lock()
	defer eb.subsMutex.Unlock()

	if subs, ok := eb.subscriptions[eventType]; ok {
		delete(subs, clientID)
		if len(subs) == 0 {
			delete(eb.subscriptions, eventType)
		}
	}
}

// run runs the event broadcaster loop
func (eb *EventBroadcaster) run() {
	defer eb.wg.Done()

	for {
		select {
		case <-eb.ctx.Done():
			return
		case broadcastEvent := <-eb.eventChannel:
			eb.processBroadcastEvent(broadcastEvent)
		}
	}
}

// processBroadcastEvent processes a broadcast event
func (eb *EventBroadcaster) processBroadcastEvent(broadcastEvent *BroadcastEvent) {
	eb.subsMutex.RLock()
	defer eb.subsMutex.RUnlock()

	eventType := broadcastEvent.EventType
	if eventType == "" {
		eventType = "*"
	}

	// Get subscribers for this event type
	var subscribers []*ClientSubscription

	// Add subscribers for specific event type
	if subs, ok := eb.subscriptions[eventType]; ok {
		for _, sub := range subs {
			subscribers = append(subscribers, sub)
		}
	}

	// Add subscribers for wildcard (*) if not a wildcard event
	if eventType != "*" {
		if subs, ok := eb.subscriptions["*"]; ok {
			for _, sub := range subs {
				subscribers = append(subscribers, sub)
			}
		}
	}

	// Filter subscribers based on target IDs and exclusions
	var targetSubscribers []*ClientSubscription

	if broadcastEvent.Multicast && len(broadcastEvent.TargetIDs) > 0 {
		// Multicast to specific clients
		targetMap := make(map[string]bool)
		for _, id := range broadcastEvent.TargetIDs {
			targetMap[id] = true
		}

		for _, sub := range subscribers {
			if targetMap[sub.ClientID] {
				targetSubscribers = append(targetSubscribers, sub)
			}
		}
		eb.metrics.IncrementMulticastEvents()
	} else {
		// Broadcast to all subscribers
		targetSubscribers = subscribers
		eb.metrics.IncrementBroadcastEvents()
	}

	// Apply exclusions
	if len(broadcastEvent.Exclude) > 0 {
		excludeMap := make(map[string]bool)
		for _, id := range broadcastEvent.Exclude {
			excludeMap[id] = true
		}

		var filtered []*ClientSubscription
		for _, sub := range targetSubscribers {
			if !excludeMap[sub.ClientID] {
				filtered = append(filtered, sub)
			}
		}
		targetSubscribers = filtered
	}

	// Send event to target subscribers
	// Note: This would typically interact with the main server to send events
	// For now, we'll just update metrics
	eb.metrics.IncrementEventsSent()
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(config *StreamingServerConfig) *ConnectionManager {
	// Configure bounded map for rate limiters to prevent memory exhaustion attacks
	mapConfig := BoundedMapConfig{
		MaxSize:        config.Security.MaxRateLimiters,
		EnableTimeouts: true,
		TTL:            config.Security.RateLimiterTTL,
	}

	// Use defaults if not configured
	if mapConfig.MaxSize <= 0 {
		mapConfig.MaxSize = 10000
	}
	if mapConfig.TTL <= 0 {
		mapConfig.TTL = 10 * time.Minute
	}

	return &ConnectionManager{
		config:       config,
		rateLimiters: NewBoundedMap[string, *RateLimiter](mapConfig),
		metrics:      NewStreamingMetrics(),
	}
}

// CanAcceptSSEConnection checks if a new SSE connection can be accepted
func (cm *ConnectionManager) CanAcceptSSEConnection() bool {
	current := atomic.LoadInt32(&cm.activeSSE)
	return int(current) < cm.config.SSE.MaxClients
}

// CanAcceptWebSocketConnection checks if a new WebSocket connection can be accepted
func (cm *ConnectionManager) CanAcceptWebSocketConnection() bool {
	current := atomic.LoadInt32(&cm.activeWebSocket)
	return int(current) < cm.config.WebSocket.MaxClients
}

// IncrementSSEConnections increments the SSE connection count
func (cm *ConnectionManager) IncrementSSEConnections() {
	atomic.AddInt32(&cm.activeSSE, 1)
}

// DecrementSSEConnections decrements the SSE connection count
func (cm *ConnectionManager) DecrementSSEConnections() {
	atomic.AddInt32(&cm.activeSSE, -1)
}

// IncrementWebSocketConnections increments the WebSocket connection count
func (cm *ConnectionManager) IncrementWebSocketConnections() {
	atomic.AddInt32(&cm.activeWebSocket, 1)
}

// DecrementWebSocketConnections decrements the WebSocket connection count
func (cm *ConnectionManager) DecrementWebSocketConnections() {
	atomic.AddInt32(&cm.activeWebSocket, -1)
}

// AllowRequest checks if a request from the given IP is allowed by rate limiting
func (cm *ConnectionManager) AllowRequest(ip string) bool {
	if !cm.config.Security.EnableRateLimit {
		return true
	}

	// Get or create rate limiter for this IP using bounded map
	limiter := cm.rateLimiters.GetOrSet(ip, func() *RateLimiter {
		return NewRateLimiter(
			float64(cm.config.Security.RateLimit),
			float64(cm.config.Security.RateLimit),
			float64(cm.config.Security.RateLimit)/cm.config.Security.RateLimitWindow.Seconds(),
		)
	})

	return limiter.Allow()
}

// CleanupRateLimiters removes expired rate limiters to free memory
func (cm *ConnectionManager) CleanupRateLimiters() int {
	return cm.rateLimiters.Cleanup()
}

// GetRateLimiterStats returns statistics about the rate limiter map
func (cm *ConnectionManager) GetRateLimiterStats() BoundedMapStats {
	return cm.rateLimiters.Stats()
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(tokens, maxTokens, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:     tokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed
func (rl *RateLimiter) Allow() bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()

	// Refill tokens
	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}

	rl.lastRefill = now

	// Check if we have tokens
	if rl.tokens >= 1.0 {
		rl.tokens--
		return true
	}

	return false
}

// NewStreamingMetrics creates a new streaming metrics instance
func NewStreamingMetrics() *StreamingMetrics {
	return &StreamingMetrics{
		StartTime: time.Now(),
	}
}

// IncrementSSEConnections increments SSE connection count
func (sm *StreamingMetrics) IncrementSSEConnections() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.SSEConnections++
	sm.TotalConnections++
}

// DecrementSSEConnections decrements SSE connection count
func (sm *StreamingMetrics) DecrementSSEConnections() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	if sm.SSEConnections > 0 {
		sm.SSEConnections--
	}
}

// IncrementWebSocketConnections increments WebSocket connection count
func (sm *StreamingMetrics) IncrementWebSocketConnections() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.WebSocketConnections++
	sm.TotalConnections++
}

// DecrementWebSocketConnections decrements WebSocket connection count
func (sm *StreamingMetrics) DecrementWebSocketConnections() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	if sm.WebSocketConnections > 0 {
		sm.WebSocketConnections--
	}
}

// IncrementEventsSent increments events sent count
func (sm *StreamingMetrics) IncrementEventsSent() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.EventsSent++
	sm.LastEventTime = time.Now()
}

// IncrementEventsReceived increments events received count
func (sm *StreamingMetrics) IncrementEventsReceived() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.EventsReceived++
}

// IncrementBytesSent increments bytes sent count
func (sm *StreamingMetrics) IncrementBytesSent(bytes int64) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.BytesSent += bytes
}

// IncrementBytesReceived increments bytes received count
func (sm *StreamingMetrics) IncrementBytesReceived(bytes int64) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.BytesReceived += bytes
}

// IncrementBroadcastEvents increments broadcast events count
func (sm *StreamingMetrics) IncrementBroadcastEvents() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.BroadcastEvents++
}

// IncrementMulticastEvents increments multicast events count
func (sm *StreamingMetrics) IncrementMulticastEvents() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.MulticastEvents++
}

// IncrementConnectionErrors increments connection errors count
func (sm *StreamingMetrics) IncrementConnectionErrors() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.ConnectionErrors++
}

// IncrementEventErrors increments event errors count
func (sm *StreamingMetrics) IncrementEventErrors() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.EventErrors++
}

// IncrementRateLimitHits increments rate limit hits count
func (sm *StreamingMetrics) IncrementRateLimitHits() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.RateLimitHits++
}

// IncrementDroppedEvents increments dropped events count
func (sm *StreamingMetrics) IncrementDroppedEvents() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	sm.DroppedEvents++
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(server *StreamingServer, config *StreamingServerConfig) *HealthChecker {
	ctx, cancel := context.WithCancel(server.ctx)

	return &HealthChecker{
		server:   server,
		ctx:      ctx,
		cancel:   cancel,
		interval: 30 * time.Second, // Default health check interval
		healthy:  1,                // Start healthy
	}
}

// Start starts the health checker
func (hc *HealthChecker) Start() {
	hc.wg.Add(1)
	go hc.run()
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	hc.cancel()
	hc.wg.Wait()
}

// run runs the health check loop
func (hc *HealthChecker) run() {
	defer hc.wg.Done()

	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.ctx.Done():
			return
		case <-ticker.C:
			hc.performHealthCheck()
		}
	}
}

// performHealthCheck performs a health check
func (hc *HealthChecker) performHealthCheck() {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	hc.lastCheck = time.Now()

	// Check if server is running
	if atomic.LoadInt32(&hc.server.running) == 0 {
		atomic.StoreInt32(&hc.healthy, 0)
		return
	}

	// Check connection counts
	sseCount, wsCount := hc.server.GetActiveConnections()
	totalConnections := sseCount + wsCount

	// Check if connections are within limits
	if totalConnections > hc.server.config.MaxConnections {
		atomic.StoreInt32(&hc.healthy, 0)
		return
	}

	// Check metrics for anomalies
	metrics := hc.server.GetMetrics()

	// Check error rates
	if metrics.ConnectionErrors > 100 || metrics.EventErrors > 100 {
		atomic.StoreInt32(&hc.healthy, 0)
		return
	}

	// All checks passed
	atomic.StoreInt32(&hc.healthy, 1)
}
