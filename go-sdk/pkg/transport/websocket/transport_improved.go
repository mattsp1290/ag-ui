package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/ag-ui/go-sdk/pkg/core"
	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/proto/generated"
	"github.com/ag-ui/go-sdk/pkg/transport"
)

// Atomic counter for generating unique subscription IDs in improved transport
var improvedSubscriptionIDCounter uint64

// ImprovedTransport implements the WebSocket transport with enhanced memory management
type ImprovedTransport struct {
	// Configuration
	config *TransportConfig

	// Connection management
	pool *ConnectionPool

	// Performance management
	performanceManager *PerformanceManager

	// Event handling with sync.Map for better concurrency
	eventHandlers sync.Map // map[string][]*EventHandlerWrapper

	// Event ring buffer instead of channel for better memory control
	eventBuffer *transport.RingBuffer

	// Subscriptions with sync.Map for better concurrency
	subscriptions sync.Map // map[string]*Subscription

	// Memory management
	memoryManager  *transport.MemoryManager
	cleanupManager *transport.CleanupManager

	// Backpressure handling
	backpressureHandler *transport.BackpressureHandler

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Statistics
	stats atomic.Value // *TransportStats

	// Last cleanup times
	lastHandlerCleanup      atomic.Int64
	lastSubscriptionCleanup atomic.Int64
}

// NewImprovedTransport creates a new WebSocket transport with improved memory management
func NewImprovedTransport(config *TransportConfig) (*ImprovedTransport, error) {
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

	// Create memory manager
	memoryManager := transport.NewMemoryManager(&transport.MemoryManagerConfig{
		Logger: config.Logger,
	})

	// Create cleanup manager
	cleanupManager := transport.NewCleanupManager(&transport.CleanupManagerConfig{
		DefaultTTL:    5 * time.Minute,
		CheckInterval: 30 * time.Second,
		Logger:        config.Logger,
	})

	// Create ring buffer with adaptive sizing
	bufferSize := memoryManager.GetAdaptiveBufferSize("event_buffer", 1000)
	ringBuffer := transport.NewRingBuffer(&transport.RingBufferConfig{
		Capacity:       bufferSize,
		OverflowPolicy: transport.OverflowDropOldest,
	})

	// Create backpressure handler
	backpressureHandler := transport.NewBackpressureHandler(transport.BackpressureConfig{
		Strategy:      transport.BackpressureDropOldest,
		BufferSize:    bufferSize,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  5 * time.Second,
		EnableMetrics: true,
	})

	ctx, cancel := context.WithCancel(context.Background())

	transport := &ImprovedTransport{
		config:              config,
		pool:                pool,
		performanceManager:  performanceManager,
		eventBuffer:         ringBuffer,
		memoryManager:       memoryManager,
		cleanupManager:      cleanupManager,
		backpressureHandler: backpressureHandler,
		ctx:                 ctx,
		cancel:              cancel,
	}

	// Initialize stats
	transport.stats.Store(&TransportStats{})

	// Set up connection pool handlers
	pool.SetOnConnectionStateChange(transport.onConnectionStateChange)
	pool.SetOnHealthChange(transport.onHealthChange)

	// Set up message handlers for all connections
	transport.setupMessageHandlers()

	// Register cleanup tasks
	transport.registerCleanupTasks()

	// Set up memory pressure callbacks
	memoryManager.OnMemoryPressure(transport.onMemoryPressure)

	return transport, nil
}

// Start initializes the WebSocket transport
func (t *ImprovedTransport) Start(ctx context.Context) error {
	t.config.Logger.Info("Starting improved WebSocket transport")

	// Start memory manager
	t.memoryManager.Start()

	// Start cleanup manager
	if err := t.cleanupManager.Start(); err != nil {
		return fmt.Errorf("failed to start cleanup manager: %w", err)
	}

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

	t.config.Logger.Info("Improved WebSocket transport started")
	return nil
}

// Stop gracefully shuts down the WebSocket transport
func (t *ImprovedTransport) Stop() error {
	t.config.Logger.Info("Stopping improved WebSocket transport")

	// Cancel context
	t.cancel()

	// Stop cleanup manager
	if err := t.cleanupManager.Stop(); err != nil {
		t.config.Logger.Error("Error stopping cleanup manager", zap.Error(err))
	}

	// Stop memory manager
	t.memoryManager.Stop()

	// Stop performance manager
	if err := t.performanceManager.Stop(); err != nil {
		t.config.Logger.Error("Error stopping performance manager", zap.Error(err))
	}

	// Stop connection pool
	if err := t.pool.Stop(); err != nil {
		t.config.Logger.Error("Error stopping connection pool", zap.Error(err))
	}

	// Cancel all subscriptions
	t.subscriptions.Range(func(key, value interface{}) bool {
		if sub, ok := value.(*Subscription); ok {
			sub.Cancel()
		}
		t.subscriptions.Delete(key)
		return true
	})

	// Close ring buffer
	t.eventBuffer.Close()

	// Stop backpressure handler
	t.backpressureHandler.Stop()

	// Wait for goroutines to finish
	t.wg.Wait()

	t.config.Logger.Info("Improved WebSocket transport stopped")
	return nil
}

// SendEvent sends an event through the WebSocket transport
func (t *ImprovedTransport) SendEvent(ctx context.Context, event events.Event) error {
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
				t.incrementEventsFailed()
				return fmt.Errorf("failed to send event: %w", err)
			}
		}
	} else {
		// Send through connection pool directly
		if err := t.pool.SendMessage(ctx, data); err != nil {
			t.incrementEventsFailed()
			return fmt.Errorf("failed to send event: %w", err)
		}
	}

	// Update statistics
	latency := time.Since(start)
	t.updateSendStats(len(data), latency)

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
func (t *ImprovedTransport) AddEventHandler(eventType string, handler EventHandler) string {
	if handler == nil {
		return ""
	}

	handlerID := fmt.Sprintf("handler_%d_%d", time.Now().UnixNano(), rand.Int63())
	wrapper := &EventHandlerWrapper{
		ID:      handlerID,
		Handler: handler,
	}

	// Load or create handler slice
	value, _ := t.eventHandlers.LoadOrStore(eventType, transport.NewSlice())
	handlers := value.(*transport.Slice)
	handlers.Append(wrapper)

	t.config.Logger.Debug("Added event handler",
		zap.String("id", handlerID),
		zap.String("event_type", eventType))

	return handlerID
}

// RemoveEventHandler removes an event handler by its ID
func (t *ImprovedTransport) RemoveEventHandler(eventType string, handlerID string) error {
	if handlerID == "" {
		return errors.New("handler ID cannot be empty")
	}

	value, exists := t.eventHandlers.Load(eventType)
	if !exists {
		return fmt.Errorf("no handlers found for event type: %s", eventType)
	}

	handlers := value.(*transport.Slice)
	var removedWrapper *EventHandlerWrapper
	removed := handlers.RemoveFunc(func(item interface{}) bool {
		wrapper := item.(*EventHandlerWrapper)
		if wrapper.ID == handlerID {
			removedWrapper = wrapper
			return true
		}
		return false
	})
	
	// Clear wrapper references to prevent memory leaks
	if removedWrapper != nil {
		runtime.SetFinalizer(removedWrapper, nil)
		removedWrapper.Handler = nil
		removedWrapper.ID = ""
	}

	if !removed {
		return fmt.Errorf("handler with ID %s not found for event type %s", handlerID, eventType)
	}

	// Remove event type if no handlers left
	if handlers.Len() == 0 {
		t.eventHandlers.Delete(eventType)
	}

	t.config.Logger.Debug("Removed event handler",
		zap.String("id", handlerID),
		zap.String("event_type", eventType))

	return nil
}

// Subscribe creates a subscription for specific event types
func (t *ImprovedTransport) Subscribe(ctx context.Context, eventTypes []string, handler EventHandler) (*Subscription, error) {
	if len(eventTypes) == 0 {
		return nil, errors.New("at least one event type must be specified")
	}

	if handler == nil {
		return nil, errors.New("event handler cannot be nil")
	}

	// Create subscription
	subCtx, cancel := context.WithCancel(ctx)
	sub := &Subscription{
		ID:         fmt.Sprintf("sub_%d", atomic.AddUint64(&improvedSubscriptionIDCounter, 1)),
		EventTypes: eventTypes,
		Handler:    handler,
		HandlerIDs: make([]string, 0, len(eventTypes)),
		Context:    subCtx,
		Cancel:     cancel,
		CreatedAt:  time.Now(),
	}

	// Add to subscriptions
	t.subscriptions.Store(sub.ID, sub)

	// Update statistics
	t.incrementSubscriptions()

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
func (t *ImprovedTransport) Unsubscribe(subscriptionID string) error {
	value, exists := t.subscriptions.LoadAndDelete(subscriptionID)
	if !exists {
		return errors.New("subscription not found")
	}

	sub := value.(*Subscription)

	// Update statistics
	t.decrementSubscriptions()

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
func (t *ImprovedTransport) GetStats() TransportStats {
	stats := t.stats.Load().(*TransportStats)
	return *stats
}

// setupMessageHandlers sets up message handlers for all connections
func (t *ImprovedTransport) setupMessageHandlers() {
	// Set up a message handler that forwards messages to the ring buffer
	messageHandler := func(data []byte) {
		if err := t.eventBuffer.Push(&rawEvent{data: data}); err != nil {
			t.config.Logger.Warn("Failed to buffer message",
				zap.Error(err),
				zap.Int("buffer_size", t.eventBuffer.Size()),
				zap.Int("buffer_capacity", t.eventBuffer.Capacity()))
			t.incrementEventsFailed()
		}
	}

	// This will be called by the pool when setting up connections
	t.pool.SetMessageHandler(messageHandler)
}

// eventProcessingLoop processes incoming events
func (t *ImprovedTransport) eventProcessingLoop() {
	defer t.wg.Done()

	t.config.Logger.Info("Starting event processing loop")

	for {
		select {
		case <-t.ctx.Done():
			t.config.Logger.Info("Stopping event processing loop")
			return
		default:
			// Try to get event from ring buffer
			event, err := t.eventBuffer.PopWithContext(t.ctx)
			if err != nil {
				if err != context.Canceled {
					t.config.Logger.Error("Failed to get event from buffer", zap.Error(err))
				}
				continue
			}

			// Process the incoming event
			if rawEvt, ok := event.(*rawEvent); ok {
				if err := t.processIncomingEvent(rawEvt.data); err != nil {
					t.config.Logger.Error("Failed to process incoming event",
						zap.Error(err),
						zap.Int("data_size", len(rawEvt.data)))
				}
			}
		}
	}
}

// registerCleanupTasks registers periodic cleanup tasks
func (t *ImprovedTransport) registerCleanupTasks() {
	// Cleanup old event handlers
	t.cleanupManager.RegisterTask("event_handlers", 5*time.Minute, func() (int, error) {
		cleaned := 0
		t.eventHandlers.Range(func(key, value interface{}) bool {
			handlers := value.(*transport.Slice)
			
			// Clean up nil handlers within slices
			handlers.RemoveFunc(func(item interface{}) bool {
				wrapper := item.(*EventHandlerWrapper)
				if wrapper == nil || wrapper.Handler == nil {
					if wrapper != nil {
						runtime.SetFinalizer(wrapper, nil)
						wrapper.ID = ""
					}
					cleaned++
					return true
				}
				return false
			})
			
			// Remove entire event type if no handlers left
			if handlers.Len() == 0 {
				t.eventHandlers.Delete(key)
				cleaned++
			}
			
			return true
		})
		
		// Force GC if we cleaned up handlers
		if cleaned > 0 {
			runtime.GC()
		}
		
		return cleaned, nil
	})

	// Cleanup expired subscriptions
	t.cleanupManager.RegisterTask("subscriptions", 5*time.Minute, func() (int, error) {
		cleaned := 0
		now := time.Now()
		t.subscriptions.Range(func(key, value interface{}) bool {
			sub := value.(*Subscription)
			sub.mutex.RLock()
			lastEvent := sub.LastEventAt
			sub.mutex.RUnlock()

			// Remove subscriptions that haven't received events for over 1 hour
			if !lastEvent.IsZero() && now.Sub(lastEvent) > time.Hour {
				if err := t.Unsubscribe(sub.ID); err == nil {
					cleaned++
				}
			}
			return true
		})
		return cleaned, nil
	})
}

// onMemoryPressure handles memory pressure events
func (t *ImprovedTransport) onMemoryPressure(level transport.MemoryPressureLevel) {
	t.config.Logger.Info("Memory pressure detected",
		zap.String("level", level.String()))

	switch level {
	case transport.MemoryPressureCritical:
		// Force immediate cleanup
		t.cleanupManager.RunTaskNow("event_handlers")
		t.cleanupManager.RunTaskNow("subscriptions")
		// Clear event buffer if needed
		if t.eventBuffer.Size() > t.eventBuffer.Capacity()/2 {
			dropped := t.eventBuffer.Size() / 2
			for i := 0; i < dropped; i++ {
				t.eventBuffer.TryPop()
			}
			t.config.Logger.Warn("Dropped events due to critical memory pressure",
				zap.Int("dropped", dropped))
		}

	case transport.MemoryPressureHigh:
		// Run cleanup tasks
		go t.cleanupManager.RunTaskNow("event_handlers")
		go t.cleanupManager.RunTaskNow("subscriptions")

	case transport.MemoryPressureLow:
		// Adjust buffer sizes
		newSize := t.memoryManager.GetAdaptiveBufferSize("event_buffer", 1000)
		t.config.Logger.Info("Adjusted buffer size due to memory pressure",
			zap.Int("new_size", newSize))
	}
}

// Helper methods for stats management

func (t *ImprovedTransport) incrementEventsFailed() {
	for {
		oldStats := t.stats.Load().(*TransportStats)
		newStats := *oldStats
		newStats.EventsFailed++
		if t.stats.CompareAndSwap(oldStats, &newStats) {
			break
		}
	}
}

func (t *ImprovedTransport) incrementSubscriptions() {
	for {
		oldStats := t.stats.Load().(*TransportStats)
		newStats := *oldStats
		newStats.TotalSubscriptions++
		newStats.ActiveSubscriptions++
		if t.stats.CompareAndSwap(oldStats, &newStats) {
			break
		}
	}
}

func (t *ImprovedTransport) decrementSubscriptions() {
	for {
		oldStats := t.stats.Load().(*TransportStats)
		newStats := *oldStats
		newStats.ActiveSubscriptions--
		if t.stats.CompareAndSwap(oldStats, &newStats) {
			break
		}
	}
}

func (t *ImprovedTransport) updateSendStats(size int, latency time.Duration) {
	for {
		oldStats := t.stats.Load().(*TransportStats)
		newStats := *oldStats
		newStats.EventsSent++
		newStats.BytesTransferred += int64(size)
		if newStats.AverageLatency == 0 {
			newStats.AverageLatency = latency
		} else {
			newStats.AverageLatency = time.Duration(
				float64(newStats.AverageLatency)*0.9 + float64(latency)*0.1,
			)
		}
		if t.stats.CompareAndSwap(oldStats, &newStats) {
			break
		}
	}
}

// onConnectionStateChange handles connection state changes
func (t *ImprovedTransport) onConnectionStateChange(connID string, state ConnectionState) {
	t.config.Logger.Debug("Connection state changed",
		zap.String("connection_id", connID),
		zap.String("state", state.String()))
}

// onHealthChange handles health changes
func (t *ImprovedTransport) onHealthChange(connID string, healthy bool) {
	t.config.Logger.Debug("Connection health changed",
		zap.String("connection_id", connID),
		zap.Bool("healthy", healthy))
}

// processIncomingEvent processes an incoming event
func (t *ImprovedTransport) processIncomingEvent(data []byte) error {
	// Parse event
	var eventData map[string]interface{}
	if err := json.Unmarshal(data, &eventData); err != nil {
		return fmt.Errorf("failed to parse event: %w", err)
	}

	eventTypeStr, ok := eventData["type"].(string)
	if !ok {
		return fmt.Errorf("event type not found or invalid")
	}

	// Update statistics
	for {
		oldStats := t.stats.Load().(*TransportStats)
		newStats := *oldStats
		newStats.EventsReceived++
		newStats.BytesTransferred += int64(len(data))
		if t.stats.CompareAndSwap(oldStats, &newStats) {
			break
		}
	}

	// Find and execute handlers
	value, exists := t.eventHandlers.Load(eventTypeStr)
	if !exists {
		t.config.Logger.Debug("No handlers for event type",
			zap.String("type", eventTypeStr))
		return nil
	}

	handlers := value.(*transport.Slice)
	
	// Create a mock event for handler execution
	event := &mockEvent{
		eventType: events.EventType(eventTypeStr),
		data:      eventData,
	}

	// Execute handlers
	handlers.Range(func(item interface{}) bool {
		wrapper := item.(*EventHandlerWrapper)
		handlerCtx, cancel := context.WithTimeout(context.Background(), t.config.EventTimeout)
		defer cancel()

		if err := wrapper.Handler(handlerCtx, event); err != nil {
			t.config.Logger.Error("Event handler failed",
				zap.String("event_type", eventTypeStr),
				zap.String("handler_id", wrapper.ID),
				zap.Error(err))
			t.incrementEventsFailed()
		}
		return true
	})

	// Update processed count
	for {
		oldStats := t.stats.Load().(*TransportStats)
		newStats := *oldStats
		newStats.EventsProcessed++
		if t.stats.CompareAndSwap(oldStats, &newStats) {
			break
		}
	}

	return nil
}

// rawEvent wraps raw event data
type rawEvent struct {
	data []byte
}

func (r *rawEvent) Type() events.EventType          { return "raw" }
func (r *rawEvent) Timestamp() *int64               { return nil }
func (r *rawEvent) SetTimestamp(int64)              {}
func (r *rawEvent) Validate() error                 { return nil }
func (r *rawEvent) ToJSON() ([]byte, error)         { return r.data, nil }
func (r *rawEvent) ToProtobuf() (*generated.Event, error) { return nil, nil }
func (r *rawEvent) GetBaseEvent() *events.BaseEvent { return nil }
func (r *rawEvent) ThreadID() string                { return "" }
func (r *rawEvent) RunID() string                   { return "" }

