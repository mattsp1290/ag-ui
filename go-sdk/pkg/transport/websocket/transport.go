package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/ag-ui/go-sdk/pkg/core"
	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/proto/generated"
)

// Transport implements the WebSocket transport for the AG-UI protocol
type Transport struct {
	// Configuration
	config *TransportConfig

	// Connection management
	pool *ConnectionPool

	// Performance management
	performanceManager *PerformanceManager

	// Event handling
	eventHandlers map[string][]*EventHandlerWrapper
	handlersMutex sync.RWMutex

	// Event channel for incoming messages
	eventCh chan []byte
	eventChClosed bool
	eventChMutex sync.RWMutex

	// Subscriptions
	subscriptions map[string]*Subscription
	subsMutex     sync.RWMutex

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Statistics
	stats *TransportStats
}

// TransportConfig contains configuration for the WebSocket transport
type TransportConfig struct {
	// URLs are the WebSocket server URLs
	URLs []string

	// PoolConfig configures the connection pool
	PoolConfig *PoolConfig

	// PerformanceConfig configures performance optimizations
	PerformanceConfig *PerformanceConfig

	// SecurityConfig configures security settings
	SecurityConfig *SecurityConfig

	// DialTimeout is the timeout for establishing WebSocket connections
	DialTimeout time.Duration

	// EventTimeout is the timeout for event processing
	EventTimeout time.Duration

	// MaxEventSize is the maximum size of events
	MaxEventSize int64

	// EnableEventValidation enables event validation
	EnableEventValidation bool

	// EventValidator is the event validator instance
	EventValidator *events.EventValidator

	// Logger is the logger instance
	Logger *zap.Logger
}

// EventHandler represents a function that handles events
type EventHandler func(ctx context.Context, event events.Event) error

// EventHandlerWrapper wraps an event handler with a unique ID
type EventHandlerWrapper struct {
	ID      string
	Handler EventHandler
}

// Subscription represents an event subscription
type Subscription struct {
	ID          string
	EventTypes  []string
	Handler     EventHandler
	HandlerIDs  []string // Track handler IDs for reliable removal
	Context     context.Context
	Cancel      context.CancelFunc
	CreatedAt   time.Time
	LastEventAt time.Time
	EventCount  int64
	mutex       sync.RWMutex
}

// TransportStats tracks transport statistics
type TransportStats struct {
	EventsSent          int64
	EventsReceived      int64
	EventsProcessed     int64
	EventsFailed        int64
	ActiveSubscriptions int64
	TotalSubscriptions  int64
	BytesTransferred    int64
	AverageLatency      time.Duration
	mutex               sync.RWMutex
}

// DefaultTransportConfig returns a default configuration for the WebSocket transport
func DefaultTransportConfig() *TransportConfig {
	return &TransportConfig{
		PoolConfig:            DefaultPoolConfig(),
		PerformanceConfig:     DefaultPerformanceConfig(),
		DialTimeout:           30 * time.Second,
		EventTimeout:          30 * time.Second,
		MaxEventSize:          1024 * 1024, // 1MB
		EnableEventValidation: true,
		EventValidator:        events.NewEventValidator(events.ProductionValidationConfig()),
		Logger:                zap.NewNop(),
	}
}

// NewTransport creates a new WebSocket transport
func NewTransport(config *TransportConfig) (*Transport, error) {
	if config == nil {
		config = DefaultTransportConfig()
	}

	if len(config.URLs) == 0 {
		return nil, &core.ConfigError{
			Field: "URLs",
			Value: config.URLs,
			Err:   errors.New("at least one WebSocket URL must be provided"),
		}
	}

	// Configure connection pool
	poolConfig := config.PoolConfig
	if poolConfig == nil {
		poolConfig = DefaultPoolConfig()
	}
	poolConfig.URLs = config.URLs

	// Configure connection template with dial timeout
	if poolConfig.ConnectionTemplate == nil {
		poolConfig.ConnectionTemplate = DefaultConnectionConfig()
	}
	// Apply the dial timeout from the transport config
	if config.DialTimeout > 0 {
		poolConfig.ConnectionTemplate.DialTimeout = config.DialTimeout
	}

	// Create connection pool
	pool, err := NewConnectionPool(poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Configure performance config
	perfConfig := config.PerformanceConfig
	if perfConfig == nil {
		perfConfig = DefaultPerformanceConfig()
	}
	perfConfig.Logger = config.Logger

	// Create performance manager
	performanceManager, err := NewPerformanceManager(perfConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create performance manager: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	transport := &Transport{
		config:             config,
		pool:               pool,
		performanceManager: performanceManager,
		eventHandlers:      make(map[string][]*EventHandlerWrapper),
		eventCh:            make(chan []byte, 1000), // Buffered channel for incoming events
		subscriptions:      make(map[string]*Subscription),
		ctx:                ctx,
		cancel:             cancel,
		stats:              &TransportStats{},
	}

	// Set up connection pool handlers
	pool.SetOnConnectionStateChange(transport.onConnectionStateChange)
	pool.SetOnHealthChange(transport.onHealthChange)

	// Set up message handlers for all connections
	transport.setupMessageHandlers()

	return transport, nil
}

// Start initializes the WebSocket transport
func (t *Transport) Start(ctx context.Context) error {
	t.config.Logger.Info("Starting WebSocket transport")

	// Recreate event channel if it was closed
	t.eventChMutex.Lock()
	if t.eventChClosed {
		t.eventCh = make(chan []byte, 1000)
		t.eventChClosed = false
	}
	t.eventChMutex.Unlock()

	// Start performance manager
	if err := t.performanceManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start performance manager: %w", err)
	}

	// Start connection pool
	if err := t.pool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start connection pool: %w", err)
	}

	// Start event processing
	t.wg.Add(1)
	go t.eventProcessingLoop()

	t.config.Logger.Info("WebSocket transport started")
	return nil
}

// Stop gracefully shuts down the WebSocket transport
func (t *Transport) Stop() error {
	t.config.Logger.Info("Stopping WebSocket transport")

	// Cancel context
	t.cancel()

	// Stop performance manager
	if err := t.performanceManager.Stop(); err != nil {
		t.config.Logger.Error("Error stopping performance manager", zap.Error(err))
	}

	// Stop connection pool
	if err := t.pool.Stop(); err != nil {
		t.config.Logger.Error("Error stopping connection pool", zap.Error(err))
	}

	// Cancel all subscriptions
	t.subsMutex.Lock()
	for _, sub := range t.subscriptions {
		sub.Cancel()
	}
	t.subscriptions = make(map[string]*Subscription)
	t.subsMutex.Unlock()

	// Close event channel to signal shutdown
	t.eventChMutex.Lock()
	if !t.eventChClosed {
		close(t.eventCh)
		t.eventChClosed = true
	}
	t.eventChMutex.Unlock()

	// Wait for goroutines to finish
	t.wg.Wait()

	t.config.Logger.Info("WebSocket transport stopped")
	return nil
}

// SendEvent sends an event through the WebSocket transport
func (t *Transport) SendEvent(ctx context.Context, event events.Event) error {
	start := time.Now()

	// Validate event if enabled
	if t.config.EnableEventValidation && t.config.EventValidator != nil {
		if result := t.config.EventValidator.ValidateEvent(ctx, event); !result.IsValid {
			return fmt.Errorf("event validation failed: %v", result.Errors)
		}
	}

	// Use performance manager for optimized serialization
	data, err := t.performanceManager.OptimizeMessage(event)
	if err != nil {
		return fmt.Errorf("failed to optimize message: %w", err)
	}

	// Check event size
	if t.config.MaxEventSize > 0 && int64(len(data)) > t.config.MaxEventSize {
		return fmt.Errorf("event size %d exceeds maximum %d", len(data), t.config.MaxEventSize)
	}

	// Use performance manager for batching if available
	if t.performanceManager != nil {
		if err := t.performanceManager.BatchMessage(data); err != nil {
			// Fall back to direct sending if batching fails
			if err := t.pool.SendMessage(ctx, data); err != nil {
				t.stats.mutex.Lock()
				t.stats.EventsFailed++
				t.stats.mutex.Unlock()
				return fmt.Errorf("failed to send event: %w", err)
			}
		}
	} else {
		// Send through connection pool directly
		if err := t.pool.SendMessage(ctx, data); err != nil {
			t.stats.mutex.Lock()
			t.stats.EventsFailed++
			t.stats.mutex.Unlock()
			return fmt.Errorf("failed to send event: %w", err)
		}
	}

	// Update statistics
	latency := time.Since(start)
	t.stats.mutex.Lock()
	t.stats.EventsSent++
	t.stats.BytesTransferred += int64(len(data))
	if t.stats.AverageLatency == 0 {
		t.stats.AverageLatency = latency
	} else {
		t.stats.AverageLatency = time.Duration(
			float64(t.stats.AverageLatency)*0.9 + float64(latency)*0.1,
		)
	}
	t.stats.mutex.Unlock()

	// Track performance metrics
	if t.performanceManager != nil && t.performanceManager.metricsCollector != nil {
		t.performanceManager.metricsCollector.TrackMessageLatency(latency)
		t.performanceManager.metricsCollector.TrackMessageSize(len(data))
	}

	t.config.Logger.Debug("Event sent",
		zap.String("type", string(event.Type())),
		zap.Duration("latency", latency),
		zap.Int("size", len(data)))

	return nil
}

// AddEventHandler adds an event handler for a specific event type and returns a handler ID
func (t *Transport) AddEventHandler(eventType string, handler EventHandler) string {
	if handler == nil {
		return ""
	}

	handlerID := fmt.Sprintf("handler_%d_%d", time.Now().UnixNano(), rand.Int63())
	wrapper := &EventHandlerWrapper{
		ID:      handlerID,
		Handler: handler,
	}

	t.handlersMutex.Lock()
	defer t.handlersMutex.Unlock()

	if _, exists := t.eventHandlers[eventType]; !exists {
		t.eventHandlers[eventType] = make([]*EventHandlerWrapper, 0)
	}
	t.eventHandlers[eventType] = append(t.eventHandlers[eventType], wrapper)

	t.config.Logger.Debug("Added event handler",
		zap.String("id", handlerID),
		zap.String("event_type", eventType))

	return handlerID
}

// RemoveEventHandler removes an event handler by its ID
func (t *Transport) RemoveEventHandler(eventType string, handlerID string) error {
	if handlerID == "" {
		return errors.New("handler ID cannot be empty")
	}

	t.handlersMutex.Lock()
	defer t.handlersMutex.Unlock()

	handlers, exists := t.eventHandlers[eventType]
	if !exists {
		return fmt.Errorf("no handlers found for event type: %s", eventType)
	}

	// Find and remove the handler
	found := false
	for i, wrapper := range handlers {
		if wrapper.ID == handlerID {
			// Remove handler from slice
			t.eventHandlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("handler with ID %s not found for event type %s", handlerID, eventType)
	}

	// Remove event type if no handlers left
	if len(t.eventHandlers[eventType]) == 0 {
		delete(t.eventHandlers, eventType)
	}

	t.config.Logger.Debug("Removed event handler",
		zap.String("id", handlerID),
		zap.String("event_type", eventType))

	return nil
}

// Subscribe creates a subscription for specific event types
func (t *Transport) Subscribe(ctx context.Context, eventTypes []string, handler EventHandler) (*Subscription, error) {
	if len(eventTypes) == 0 {
		return nil, errors.New("at least one event type must be specified")
	}

	if handler == nil {
		return nil, errors.New("event handler cannot be nil")
	}

	// Create subscription
	subCtx, cancel := context.WithCancel(ctx)
	sub := &Subscription{
		ID:         fmt.Sprintf("sub_%d", time.Now().UnixNano()),
		EventTypes: eventTypes,
		Handler:    handler,
		HandlerIDs: make([]string, 0, len(eventTypes)),
		Context:    subCtx,
		Cancel:     cancel,
		CreatedAt:  time.Now(),
	}

	// Add to subscriptions
	t.subsMutex.Lock()
	t.subscriptions[sub.ID] = sub
	t.subsMutex.Unlock()

	// Update statistics with proper mutex
	t.stats.mutex.Lock()
	t.stats.TotalSubscriptions++
	t.stats.ActiveSubscriptions++
	t.stats.mutex.Unlock()

	// Register event handlers and track their IDs
	for _, eventType := range eventTypes {
		handlerID := t.AddEventHandler(eventType, handler)
		sub.HandlerIDs = append(sub.HandlerIDs, handlerID)
	}

	t.config.Logger.Info("Created subscription",
		zap.String("id", sub.ID),
		zap.Strings("event_types", eventTypes))

	return sub, nil
}

// Unsubscribe removes a subscription
func (t *Transport) Unsubscribe(subscriptionID string) error {
	t.subsMutex.Lock()
	sub, exists := t.subscriptions[subscriptionID]
	if exists {
		delete(t.subscriptions, subscriptionID)
	}
	t.subsMutex.Unlock()

	if exists {
		// Update statistics with proper mutex
		t.stats.mutex.Lock()
		t.stats.ActiveSubscriptions--
		t.stats.mutex.Unlock()
	}

	if !exists {
		return errors.New("subscription not found")
	}

	// Cancel subscription
	sub.Cancel()

	// Remove event handlers using the stored handler IDs
	for i, eventType := range sub.EventTypes {
		if i < len(sub.HandlerIDs) {
			if err := t.RemoveEventHandler(eventType, sub.HandlerIDs[i]); err != nil {
				t.config.Logger.Warn("Failed to remove event handler",
					zap.String("subscription_id", subscriptionID),
					zap.String("event_type", eventType),
					zap.String("handler_id", sub.HandlerIDs[i]),
					zap.Error(err))
			}
		}
	}

	t.config.Logger.Info("Removed subscription",
		zap.String("id", subscriptionID))

	return nil
}

// GetStats returns a copy of the transport statistics
func (t *Transport) GetStats() TransportStats {
	t.stats.mutex.RLock()
	defer t.stats.mutex.RUnlock()
	return *t.stats
}

// GetConnectionPoolStats returns the connection pool statistics
func (t *Transport) GetConnectionPoolStats() PoolStats {
	return t.pool.GetStats()
}

// GetDetailedStatus returns detailed status information
func (t *Transport) GetDetailedStatus() map[string]interface{} {
	t.subsMutex.RLock()
	subscriptions := make(map[string]interface{})
	for id, sub := range t.subscriptions {
		sub.mutex.RLock()
		subscriptions[id] = map[string]interface{}{
			"event_types":   sub.EventTypes,
			"created_at":    sub.CreatedAt,
			"last_event_at": sub.LastEventAt,
			"event_count":   sub.EventCount,
		}
		sub.mutex.RUnlock()
	}
	t.subsMutex.RUnlock()

	// Get event handlers count safely
	t.handlersMutex.RLock()
	eventHandlersCount := len(t.eventHandlers)
	t.handlersMutex.RUnlock()

	return map[string]interface{}{
		"transport_stats":      t.GetStats(),
		"connection_pool":      t.pool.GetDetailedStatus(),
		"subscriptions":        subscriptions,
		"active_subscriptions": len(subscriptions),
		"event_handlers":       eventHandlersCount,
	}
}

// setupMessageHandlers sets up message handlers for all connections
func (t *Transport) setupMessageHandlers() {
	// Set up a message handler that forwards messages to the event channel
	messageHandler := func(data []byte) {
		// Check if channel is closed before attempting to send
		t.eventChMutex.RLock()
		if t.eventChClosed {
			t.eventChMutex.RUnlock()
			return
		}
		t.eventChMutex.RUnlock()

		select {
		case t.eventCh <- data:
			// Successfully queued the event
		case <-t.ctx.Done():
			// Transport is shutting down
		default:
			// Channel is full, log and drop the message
			t.config.Logger.Warn("Event channel full, dropping message",
				zap.Int("channel_size", len(t.eventCh)),
				zap.Int("channel_capacity", cap(t.eventCh)))
			t.stats.mutex.Lock()
			t.stats.EventsFailed++
			t.stats.mutex.Unlock()
		}
	}

	// This will be called by the pool when setting up connections
	t.pool.SetMessageHandler(messageHandler)
}

// onConnectionStateChange handles connection state changes
func (t *Transport) onConnectionStateChange(connID string, state ConnectionState) {
	t.config.Logger.Debug("Connection state changed",
		zap.String("connection_id", connID),
		zap.String("state", state.String()))
}

// onHealthChange handles health changes
func (t *Transport) onHealthChange(connID string, healthy bool) {
	t.config.Logger.Debug("Connection health changed",
		zap.String("connection_id", connID),
		zap.Bool("healthy", healthy))
}

// eventProcessingLoop processes incoming events
func (t *Transport) eventProcessingLoop() {
	defer t.wg.Done()

	t.config.Logger.Info("Starting event processing loop")

	for {
		select {
		case <-t.ctx.Done():
			t.config.Logger.Info("Stopping event processing loop")
			return
		case data := <-t.eventCh:
			// Process the incoming event
			if err := t.processIncomingEvent(data); err != nil {
				t.config.Logger.Error("Failed to process incoming event",
					zap.Error(err),
					zap.Int("data_size", len(data)))
			}
		}
	}
}

// processIncomingEvent processes an incoming event
func (t *Transport) processIncomingEvent(data []byte) error {
	// Parse event - using a generic map first, then convert to specific event type
	var eventData map[string]interface{}
	if err := json.Unmarshal(data, &eventData); err != nil {
		return fmt.Errorf("failed to parse event: %w", err)
	}

	// For now, create a simple wrapper that implements the Event interface
	// In a real implementation, we would have proper event type detection and parsing
	eventTypeStr, ok := eventData["type"].(string)
	if !ok {
		return fmt.Errorf("event type not found or invalid")
	}

	// Update statistics
	t.stats.mutex.Lock()
	t.stats.EventsReceived++
	t.stats.BytesTransferred += int64(len(data))
	t.stats.mutex.Unlock()

	// Find and execute handlers
	t.handlersMutex.RLock()
	handlers, exists := t.eventHandlers[eventTypeStr]
	t.handlersMutex.RUnlock()

	if !exists {
		t.config.Logger.Debug("No handlers for event type",
			zap.String("type", eventTypeStr))
		return nil
	}

	// Create a mock event for handler execution
	// In a real implementation, this would be properly parsed
	event := &mockEvent{
		eventType: events.EventType(eventTypeStr),
		data:      eventData,
	}

	// Execute handlers
	for _, wrapper := range handlers {
		handlerCtx, cancel := context.WithTimeout(context.Background(), t.config.EventTimeout)

		if err := wrapper.Handler(handlerCtx, event); err != nil {
			t.config.Logger.Error("Event handler failed",
				zap.String("event_type", eventTypeStr),
				zap.String("handler_id", wrapper.ID),
				zap.Error(err))
			t.stats.mutex.Lock()
			t.stats.EventsFailed++
			t.stats.mutex.Unlock()
			cancel()
			continue
		}

		cancel()
	}

	t.stats.mutex.Lock()
	t.stats.EventsProcessed++
	t.stats.mutex.Unlock()

	return nil
}

// mockEvent is a simple implementation of the Event interface for testing
type mockEvent struct {
	eventType events.EventType
	data      map[string]interface{}
}

func (m *mockEvent) Type() events.EventType { return m.eventType }
func (m *mockEvent) Timestamp() *int64      { return nil }
func (m *mockEvent) SetTimestamp(int64)     {}
func (m *mockEvent) Validate() error        { return nil }
func (m *mockEvent) ToJSON() ([]byte, error) {
	return json.Marshal(m.data)
}
func (m *mockEvent) ToProtobuf() (*generated.Event, error) { return nil, nil }
func (m *mockEvent) GetBaseEvent() *events.BaseEvent       { return nil }
func (m *mockEvent) ThreadID() string                     { return "" }
func (m *mockEvent) RunID() string                        { return "" }

// IsConnected returns true if the transport has healthy connections
func (t *Transport) IsConnected() bool {
	return t.pool.GetHealthyConnectionCount() > 0
}

// GetActiveConnectionCount returns the number of active connections
func (t *Transport) GetActiveConnectionCount() int {
	return t.pool.GetActiveConnectionCount()
}

// GetHealthyConnectionCount returns the number of healthy connections
func (t *Transport) GetHealthyConnectionCount() int {
	return t.pool.GetHealthyConnectionCount()
}

// Ping sends a ping through all connections
func (t *Transport) Ping(ctx context.Context) error {
	// This would trigger ping messages through all connections
	// The actual implementation would depend on the connection pool
	return nil
}

// GetSubscription returns a subscription by ID
func (t *Transport) GetSubscription(subscriptionID string) (*Subscription, error) {
	t.subsMutex.RLock()
	defer t.subsMutex.RUnlock()

	sub, exists := t.subscriptions[subscriptionID]
	if !exists {
		return nil, errors.New("subscription not found")
	}

	return sub, nil
}

// ListSubscriptions returns all active subscriptions
func (t *Transport) ListSubscriptions() []*Subscription {
	t.subsMutex.RLock()
	defer t.subsMutex.RUnlock()

	subs := make([]*Subscription, 0, len(t.subscriptions))
	for _, sub := range t.subscriptions {
		subs = append(subs, sub)
	}

	return subs
}

// GetEventHandlerCount returns the number of event handlers
func (t *Transport) GetEventHandlerCount() int {
	t.handlersMutex.RLock()
	defer t.handlersMutex.RUnlock()

	count := 0
	for _, handlers := range t.eventHandlers {
		count += len(handlers)
	}

	return count
}

// Close closes the transport and releases all resources
func (t *Transport) Close() error {
	return t.Stop()
}

// GetPerformanceMetrics returns current performance metrics
func (t *Transport) GetPerformanceMetrics() *PerformanceMetrics {
	if t.performanceManager == nil {
		return nil
	}
	return t.performanceManager.GetMetrics()
}

// GetMemoryUsage returns current memory usage
func (t *Transport) GetMemoryUsage() int64 {
	if t.performanceManager == nil {
		return 0
	}
	return t.performanceManager.GetMemoryUsage()
}

// OptimizeForThroughput optimizes the transport for maximum throughput
func (t *Transport) OptimizeForThroughput() {
	if t.performanceManager == nil {
		return
	}
	optimizer := NewPerformanceOptimizer(t.performanceManager)
	optimizer.OptimizeForThroughput()
}

// OptimizeForLatency optimizes the transport for minimum latency
func (t *Transport) OptimizeForLatency() {
	if t.performanceManager == nil {
		return
	}
	optimizer := NewPerformanceOptimizer(t.performanceManager)
	optimizer.OptimizeForLatency()
}

// OptimizeForMemory optimizes the transport for minimum memory usage
func (t *Transport) OptimizeForMemory() {
	if t.performanceManager == nil {
		return
	}
	optimizer := NewPerformanceOptimizer(t.performanceManager)
	optimizer.OptimizeForMemory()
}

// EnableAdaptiveOptimization enables adaptive performance optimization
func (t *Transport) EnableAdaptiveOptimization() {
	if t.performanceManager == nil {
		return
	}

	adaptiveOptimizer := NewAdaptiveOptimizer(t.performanceManager)

	// Start adaptive optimizer in background
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		var wg sync.WaitGroup
		wg.Add(1)
		adaptiveOptimizer.Start(t.ctx, &wg)
	}()
}
