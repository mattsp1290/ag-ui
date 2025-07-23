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
	
	// Backpressure management
	droppedEvents     int64
	backpressureActive bool
	lastDropTime      time.Time
	backpressureMutex sync.RWMutex
	
	// Resource management
	activeGoroutines  map[string]*GoroutineInfo
	goroutinesMutex   sync.RWMutex
	resourceCleanup   []func() error
	cleanupMutex      sync.Mutex
	
	// Monitoring
	monitoringCtx    context.Context
	monitoringCancel context.CancelFunc
}

// BackpressureConfig configures backpressure behavior for WebSocket transport
type BackpressureConfig struct {
	// EventChannelBuffer is the buffer size for event channel
	EventChannelBuffer int
	
	// MaxDroppedEvents is the maximum number of events that can be dropped before taking action
	MaxDroppedEvents int64
	
	// DropActionType defines what to do when max dropped events is reached
	DropActionType DropActionType
	
	// EnableBackpressureLogging enables detailed logging of backpressure events
	EnableBackpressureLogging bool
	
	// BackpressureThresholdPercent is the percentage at which to start applying backpressure (0-100)
	BackpressureThresholdPercent int
	
	// EnableChannelMonitoring enables monitoring of channel usage
	EnableChannelMonitoring bool
	
	// MonitoringInterval is the interval for channel monitoring
	MonitoringInterval time.Duration
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
	
	// DropActionSlowDown applies flow control
	DropActionSlowDown
)

// DefaultBackpressureConfig returns default backpressure configuration for WebSocket
func DefaultBackpressureConfig() *BackpressureConfig {
	return &BackpressureConfig{
		EventChannelBuffer:           10000,
		MaxDroppedEvents:             1000,
		DropActionType:               DropActionSlowDown,
		EnableBackpressureLogging:    true,
		BackpressureThresholdPercent: 85,
		EnableChannelMonitoring:      true,
		MonitoringInterval:           5 * time.Second,
	}
}

// ResourceCleanupConfig configures resource cleanup behavior
type ResourceCleanupConfig struct {
	// EnableGoroutineTracking enables tracking of goroutines
	EnableGoroutineTracking bool
	
	// CleanupInterval is the interval for resource cleanup
	CleanupInterval time.Duration
	
	// MaxGoroutineIdleTime is the maximum time a goroutine can be idle before cleanup
	MaxGoroutineIdleTime time.Duration
	
	// EnableResourceMonitoring enables monitoring of resource usage
	EnableResourceMonitoring bool
}

// DefaultResourceCleanupConfig returns default resource cleanup configuration
func DefaultResourceCleanupConfig() *ResourceCleanupConfig {
	return &ResourceCleanupConfig{
		EnableGoroutineTracking:  true,
		CleanupInterval:          30 * time.Second,
		MaxGoroutineIdleTime:     5 * time.Minute,
		EnableResourceMonitoring: true,
	}
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
	
	// BackpressureConfig configures backpressure behavior
	BackpressureConfig *BackpressureConfig
	
	// ResourceCleanupConfig configures resource cleanup
	ResourceCleanupConfig *ResourceCleanupConfig
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

// GoroutineInfo tracks information about active goroutines
type GoroutineInfo struct {
	Name      string
	StartTime time.Time
	LastSeen  time.Time
	Function  string
	Context   context.Context
	Cancel    context.CancelFunc
}

// TransportStats tracks transport statistics
type TransportStats struct {
	EventsSent          int64
	EventsReceived      int64
	EventsProcessed     int64
	EventsFailed        int64
	EventsDropped       int64
	ActiveSubscriptions int64
	TotalSubscriptions  int64
	BytesTransferred    int64
	AverageLatency      time.Duration
	ActiveGoroutines    int64
	BackpressureEvents  int64
	ResourceCleanups    int64
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
		BackpressureConfig:    DefaultBackpressureConfig(),
		ResourceCleanupConfig: DefaultResourceCleanupConfig(),
	}
}

// HighConcurrencyTransportConfig returns a transport configuration optimized for high concurrency testing
func HighConcurrencyTransportConfig() *TransportConfig {
	// Create high concurrency pool config
	poolConfig := DefaultPoolConfig()
	poolConfig.MinConnections = 50
	poolConfig.MaxConnections = 500
	poolConfig.ConnectionTemplate = DefaultConnectionConfig()
	poolConfig.ConnectionTemplate.RateLimiter = NewTestRateLimiter() // Use test rate limiter
	
	// Create high concurrency backpressure config  
	backpressureConfig := DefaultBackpressureConfig()
	backpressureConfig.EventChannelBuffer = 50000 // Larger buffer for high concurrency
	backpressureConfig.MaxDroppedEvents = 10000   // Allow more dropped events
	backpressureConfig.BackpressureThresholdPercent = 95 // Higher threshold
	
	return &TransportConfig{
		PoolConfig:            poolConfig,
		PerformanceConfig:     HighConcurrencyPerformanceConfig(),
		DialTimeout:           10 * time.Second, // Faster for tests
		EventTimeout:          10 * time.Second, // Faster for tests 
		MaxEventSize:          1024 * 1024, // 1MB
		EnableEventValidation: false, // Disable validation for speed
		EventValidator:        nil,
		Logger:                zap.NewNop(),
		BackpressureConfig:    backpressureConfig,
		ResourceCleanupConfig: DefaultResourceCleanupConfig(),
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
	poolConfig.Logger = config.Logger // Ensure pool uses the transport's logger

	// Configure connection template with dial timeout
	if poolConfig.ConnectionTemplate == nil {
		poolConfig.ConnectionTemplate = DefaultConnectionConfig()
	}
	// Apply the dial timeout from the transport config
	if config.DialTimeout > 0 {
		poolConfig.ConnectionTemplate.DialTimeout = config.DialTimeout
	}
	// Ensure connection template uses the transport's logger
	poolConfig.ConnectionTemplate.Logger = config.Logger

	// Create connection pool
	pool, err := NewConnectionPool(poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Configure performance config
	perfConfig := config.PerformanceConfig
	var performanceManager *PerformanceManager
	
	// Create performance manager only if config is provided (not nil)
	if config.PerformanceConfig != nil {
		if perfConfig == nil {
			perfConfig = DefaultPerformanceConfig()
		}
		perfConfig.Logger = config.Logger
		
		var err error
		performanceManager, err = NewPerformanceManager(perfConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create performance manager: %w", err)
		}
	}

	// Set default configs if nil
	if config.BackpressureConfig == nil {
		config.BackpressureConfig = DefaultBackpressureConfig()
	}
	if config.ResourceCleanupConfig == nil {
		config.ResourceCleanupConfig = DefaultResourceCleanupConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())
	monitoringCtx, monitoringCancel := context.WithCancel(context.Background())

	// Determine event channel buffer size
	eventChannelSize := config.BackpressureConfig.EventChannelBuffer
	if eventChannelSize <= 0 {
		eventChannelSize = 1000
	}

	transport := &Transport{
		config:             config,
		pool:               pool,
		performanceManager: performanceManager,
		eventHandlers:      make(map[string][]*EventHandlerWrapper),
		eventCh:            make(chan []byte, eventChannelSize),
		subscriptions:      make(map[string]*Subscription),
		ctx:                ctx,
		cancel:             cancel,
		stats:              &TransportStats{},
		activeGoroutines:   make(map[string]*GoroutineInfo),
		resourceCleanup:    make([]func() error, 0),
		monitoringCtx:      monitoringCtx,
		monitoringCancel:   monitoringCancel,
	}

	// Set up connection pool handlers
	pool.SetOnConnectionStateChange(transport.onConnectionStateChange)
	pool.SetOnHealthChange(transport.onHealthChange)

	// Note: setupMessageHandlers() is now called in Start() after pool initialization
	// to ensure proper message handler propagation to all connections

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

	// Start performance manager if available
	if t.performanceManager != nil {
		if err := t.performanceManager.Start(ctx); err != nil {
			return fmt.Errorf("failed to start performance manager: %w", err)
		}
	}

	// Start connection pool
	if err := t.pool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start connection pool: %w", err)
	}

	// Set up message handlers for all connections after pool initialization
	// This ensures proper message handler propagation to all established connections
	t.setupMessageHandlers()

	// Start event processing
	t.startGoroutine("event-processing", t.eventProcessingLoop)

	// Start monitoring if enabled
	if t.config.BackpressureConfig.EnableChannelMonitoring {
		t.startGoroutine("channel-monitor", t.channelMonitoringLoop)
	}
	
	// Start resource cleanup if enabled
	if t.config.ResourceCleanupConfig.EnableResourceMonitoring {
		t.startGoroutine("resource-cleanup", t.resourceCleanupLoop)
	}

	t.config.Logger.Info("WebSocket transport started")
	return nil
}

// Stop gracefully shuts down the WebSocket transport
func (t *Transport) Stop() error {
	t.config.Logger.Debug("Stopping WebSocket transport with aggressive cleanup")

	// Cancel contexts immediately to signal all goroutines
	t.monitoringCancel()
	t.cancel()

	// Close event channel immediately to unblock goroutines
	t.eventChMutex.Lock()
	if !t.eventChClosed {
		close(t.eventCh)
		t.eventChClosed = true
	}
	t.eventChMutex.Unlock()

	// Stop connection pool first (this closes all WebSocket connections)
	if err := t.pool.Stop(); err != nil {
		t.config.Logger.Debug("Error stopping connection pool (expected)", zap.Error(err))
	}

	// Stop performance manager if available
	if t.performanceManager != nil {
		if err := t.performanceManager.Stop(); err != nil {
			t.config.Logger.Debug("Error stopping performance manager (expected)", zap.Error(err))
		}
	}

	// Cancel all subscriptions
	t.subsMutex.Lock()
	for _, sub := range t.subscriptions {
		sub.Cancel()
	}
	t.subscriptions = make(map[string]*Subscription)
	t.subsMutex.Unlock()

	// Cancel all active goroutines
	t.goroutinesMutex.Lock()
	for _, goroutineInfo := range t.activeGoroutines {
		if goroutineInfo.Cancel != nil {
			goroutineInfo.Cancel()
		}
	}
	t.activeGoroutines = make(map[string]*GoroutineInfo)
	t.goroutinesMutex.Unlock()

	// Execute resource cleanup functions quickly
	t.cleanupMutex.Lock()
	for _, cleanup := range t.resourceCleanup {
		if err := cleanup(); err != nil {
			t.config.Logger.Debug("Resource cleanup error (expected)", zap.Error(err))
		}
	}
	t.stats.mutex.Lock()
	t.stats.ResourceCleanups += int64(len(t.resourceCleanup))
	t.stats.mutex.Unlock()
	t.cleanupMutex.Unlock()

	// Wait for goroutines to finish with very short timeout
	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		t.config.Logger.Debug("WebSocket transport stopped cleanly")
	case <-time.After(200 * time.Millisecond): // Very short timeout for tests
		t.config.Logger.Debug("WebSocket transport stop timeout - connections force closed so proceeding")
	}

	t.config.Logger.Debug("WebSocket transport stop completed")
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

	// Use performance manager for optimized serialization if available
	var data []byte
	var err error
	if t.performanceManager != nil {
		data, err = t.performanceManager.OptimizeMessage(event)
		if err != nil {
			return fmt.Errorf("failed to optimize message: %w", err)
		}
	} else {
		// Fall back to JSON serialization
		data, err = event.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize event: %w", err)
		}
	}

	// Check event size
	if t.config.MaxEventSize > 0 && int64(len(data)) > t.config.MaxEventSize {
		return fmt.Errorf("event size %d exceeds maximum %d", len(data), t.config.MaxEventSize)
	}

	// Use performance manager for batching if available, otherwise send directly
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

	// Track performance metrics if available
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

	// Register event handlers and track their IDs BEFORE adding to map
	// This ensures the subscription is fully functional when added
	var registeredHandlers []struct {
		eventType string
		handlerID string
	}
	
	for _, eventType := range eventTypes {
		handlerID := t.AddEventHandler(eventType, handler)
		if handlerID == "" {
			// Clean up any handlers we've already registered
			for _, registered := range registeredHandlers {
				t.RemoveEventHandler(registered.eventType, registered.handlerID)
			}
			cancel() // Cancel the subscription context
			return nil, fmt.Errorf("failed to register handler for event type: %s", eventType)
		}
		sub.HandlerIDs = append(sub.HandlerIDs, handlerID)
		registeredHandlers = append(registeredHandlers, struct {
			eventType string
			handlerID string
		}{eventType, handlerID})
	}

	// Only add to subscriptions map after handlers are registered
	t.subsMutex.Lock()
	t.subscriptions[sub.ID] = sub
	t.subsMutex.Unlock()

	// Update statistics with proper mutex AFTER subscription is fully functional
	t.stats.mutex.Lock()
	t.stats.TotalSubscriptions++
	t.stats.ActiveSubscriptions++
	t.stats.mutex.Unlock()

	t.config.Logger.Info("Created subscription",
		zap.String("id", sub.ID),
		zap.Strings("event_types", eventTypes))

	return sub, nil
}

// Unsubscribe removes a subscription
func (t *Transport) Unsubscribe(subscriptionID string) error {
	// First, check if subscription exists and get it under lock
	t.subsMutex.Lock()
	sub, exists := t.subscriptions[subscriptionID]
	if !exists {
		t.subsMutex.Unlock()
		return errors.New("subscription not found")
	}
	// Don't remove from map yet - do it after cleanup
	t.subsMutex.Unlock()

	// Cancel subscription first
	sub.Cancel()

	// Remove event handlers using the stored handler IDs
	// Track any errors but continue cleanup
	var handlerErrors []error
	for i, eventType := range sub.EventTypes {
		if i < len(sub.HandlerIDs) {
			if err := t.RemoveEventHandler(eventType, sub.HandlerIDs[i]); err != nil {
				handlerErrors = append(handlerErrors, err)
				t.config.Logger.Warn("Failed to remove event handler",
					zap.String("subscription_id", subscriptionID),
					zap.String("event_type", eventType),
					zap.String("handler_id", sub.HandlerIDs[i]),
					zap.Error(err))
			}
		}
	}

	// Only after all cleanup is done, remove from map and update statistics atomically
	t.subsMutex.Lock()
	// Check again in case subscription was removed by another goroutine
	if _, stillExists := t.subscriptions[subscriptionID]; stillExists {
		delete(t.subscriptions, subscriptionID)
		
		// Update statistics with proper mutex AFTER cleanup and removal
		t.stats.mutex.Lock()
		t.stats.ActiveSubscriptions--
		t.stats.mutex.Unlock()
		
		t.subsMutex.Unlock()
		
		// If there were handler removal errors, log them but don't fail the unsubscribe
		if len(handlerErrors) > 0 {
			t.config.Logger.Warn("Some event handlers could not be removed during unsubscribe",
				zap.String("subscription_id", subscriptionID),
				zap.Int("failed_handlers", len(handlerErrors)))
		}

		t.config.Logger.Info("Removed subscription",
			zap.String("id", subscriptionID))
	} else {
		t.subsMutex.Unlock()
		// Subscription was already removed by another goroutine
		t.config.Logger.Debug("Subscription was already removed during cleanup",
			zap.String("id", subscriptionID))
	}

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
	t.config.Logger.Debug("Setting up message handlers for transport")
	
	// Set up a message handler that forwards messages to the event channel
	messageHandler := func(data []byte) {
		t.config.Logger.Debug("Message handler received data", 
			zap.Int("size", len(data)), 
			zap.String("data", string(data)))
		
		// Check if channel is closed before attempting to send
		t.eventChMutex.RLock()
		if t.eventChClosed {
			t.config.Logger.Debug("Event channel is closed, dropping message")
			t.eventChMutex.RUnlock()
			return
		}
		t.eventChMutex.RUnlock()

		// Handle message with backpressure control
		t.handleEventWithBackpressure(data)
	}

	// This will be called by the pool when setting up connections
	t.pool.SetMessageHandler(messageHandler)
	
	t.config.Logger.Debug("Message handlers setup completed")
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
// Note: WaitGroup management is handled by startGoroutine
func (t *Transport) eventProcessingLoop() {
	t.config.Logger.Debug("Starting event processing loop")
	defer t.config.Logger.Debug("Event processing loop fully exited")

	for {
		// Check for shutdown first with immediate exit
		select {
		case <-t.ctx.Done():
			t.config.Logger.Debug("Event processing: Main context cancelled, exiting immediately")
			t.drainEventChannel()
			return
		case <-t.monitoringCtx.Done():
			t.config.Logger.Debug("Event processing: Monitoring context cancelled, exiting immediately")
			t.drainEventChannel()
			return
		default:
			// Continue to event processing
		}

		// Try to get an event with immediate timeout
		select {
		case <-t.ctx.Done():
			t.config.Logger.Debug("Event processing: Context cancelled during event wait, exiting")
			t.drainEventChannel()
			return
		case <-t.monitoringCtx.Done():
			t.config.Logger.Debug("Event processing: Monitoring context cancelled during event wait, exiting")
			t.drainEventChannel()
			return
		case data, ok := <-t.eventCh:
			if !ok {
				t.config.Logger.Debug("Event channel closed, exiting event processing loop")
				return
			}
			
			// Check for cancellation before processing
			select {
			case <-t.ctx.Done():
				t.config.Logger.Debug("Context cancelled before event processing, exiting")
				return
			case <-t.monitoringCtx.Done():
				t.config.Logger.Debug("Monitoring context cancelled before event processing, exiting")
				return
			default:
				// Process the incoming event quickly
				if err := t.processIncomingEvent(data); err != nil {
					t.config.Logger.Debug("Failed to process incoming event",
						zap.Error(err),
						zap.Int("data_size", len(data)))
				}
			}
		case <-time.After(1 * time.Millisecond): // Very short timeout to check for shutdown
			// Just loop back to check for shutdown
			continue
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
	totalHandlers := len(t.eventHandlers)
	t.handlersMutex.RUnlock()

	t.config.Logger.Debug("Looking for event handlers",
		zap.String("event_type", eventTypeStr),
		zap.Bool("handlers_exist", exists),
		zap.Int("handler_count", len(handlers)),
		zap.Int("total_event_types", totalHandlers))

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

// drainEventChannel drains any remaining events from the event channel
func (t *Transport) drainEventChannel() {
	for {
		select {
		case _, ok := <-t.eventCh:
			if !ok {
				// Channel is closed
				return
			}
			// Discard the event
		default:
			// No more events to drain
			return
		}
	}
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
