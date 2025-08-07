package events

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EventBus provides event-driven communication between modules
// Implements the Observer pattern for decoupled inter-module communication
type EventBus interface {
	// Subscribe subscribes to events of a specific type
	Subscribe(eventType string, handler EventHandler) (SubscriptionID, error)

	// SubscribeWithFilter subscribes with a custom filter
	SubscribeWithFilter(eventType string, filter EventFilter, handler EventHandler) (SubscriptionID, error)

	// Unsubscribe removes a subscription
	Unsubscribe(subscriptionID SubscriptionID) error

	// Publish publishes an event to all subscribers
	Publish(ctx context.Context, event BusEvent) error

	// PublishAsync publishes an event asynchronously
	PublishAsync(ctx context.Context, event BusEvent) error

	// Close closes the event bus and cleans up resources
	Close() error

	// GetStats returns event bus statistics
	GetStats() BusStats
}

// SubscriptionID uniquely identifies a subscription
type SubscriptionID string

// EventHandler processes events from the event bus
type EventHandler func(ctx context.Context, event BusEvent) error

// EventFilter filters events before delivery to handlers
type EventFilter func(event BusEvent) bool

// BusEvent represents an event in the event bus
type BusEvent struct {
	// ID uniquely identifies this event
	ID string `json:"id"`

	// Type categorizes the event
	Type string `json:"type"`

	// Source identifies the module/component that generated the event
	Source string `json:"source"`

	// Data contains the event payload
	Data interface{} `json:"data"`

	// Metadata contains additional event information
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Timestamp when the event was created
	Timestamp time.Time `json:"timestamp"`

	// Priority of the event (higher values processed first)
	Priority int `json:"priority"`

	// TTL is how long the event is valid
	TTL time.Duration `json:"ttl,omitempty"`
}

// IsExpired checks if the event has expired
func (e *BusEvent) IsExpired() bool {
	if e.TTL == 0 {
		return false
	}
	return time.Since(e.Timestamp) > e.TTL
}

// GetMetadata gets a metadata value
func (e *BusEvent) GetMetadata(key string) (interface{}, bool) {
	if e.Metadata == nil {
		return nil, false
	}
	value, exists := e.Metadata[key]
	return value, exists
}

// SetMetadata sets a metadata value
func (e *BusEvent) SetMetadata(key string, value interface{}) {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
}

// BusStats contains event bus statistics
type BusStats struct {
	// Subscription statistics
	TotalSubscriptions  int `json:"total_subscriptions"`
	ActiveSubscriptions int `json:"active_subscriptions"`

	// Event statistics
	EventsPublished int64 `json:"events_published"`
	EventsDelivered int64 `json:"events_delivered"`
	EventsDropped   int64 `json:"events_dropped"`
	EventsFiltered  int64 `json:"events_filtered"`

	// Performance statistics
	AverageDeliveryTime time.Duration `json:"average_delivery_time"`
	PendingEvents       int           `json:"pending_events"`
	QueueUtilization    float64       `json:"queue_utilization"`

	// Error statistics
	DeliveryErrors int64     `json:"delivery_errors"`
	LastError      error     `json:"last_error,omitempty"`
	LastErrorTime  time.Time `json:"last_error_time,omitempty"`
}

// EventBusConfig configures the event bus
type EventBusConfig struct {
	// BufferSize is the size of the event buffer
	BufferSize int `json:"buffer_size"`

	// WorkerCount is the number of worker goroutines
	WorkerCount int `json:"worker_count"`

	// DeliveryTimeout is the timeout for event delivery
	DeliveryTimeout time.Duration `json:"delivery_timeout"`

	// EnableMetrics enables statistics collection
	EnableMetrics bool `json:"enable_metrics"`

	// EnableAsync enables asynchronous event processing
	EnableAsync bool `json:"enable_async"`

	// MaxRetries is the maximum number of delivery retries
	MaxRetries int `json:"max_retries"`

	// RetryDelay is the delay between retry attempts
	RetryDelay time.Duration `json:"retry_delay"`

	// DropOnFullBuffer drops events if buffer is full
	DropOnFullBuffer bool `json:"drop_on_full_buffer"`
}

// DefaultEventBusConfig returns default event bus configuration
func DefaultEventBusConfig() *EventBusConfig {
	return &EventBusConfig{
		BufferSize:       1000,
		WorkerCount:      4,
		DeliveryTimeout:  30 * time.Second,
		EnableMetrics:    true,
		EnableAsync:      true,
		MaxRetries:       3,
		RetryDelay:       1 * time.Second,
		DropOnFullBuffer: false,
	}
}

// EventBusImpl implements the EventBus interface
type EventBusImpl struct {
	config        *EventBusConfig
	subscriptions map[string][]*subscription
	eventQueue    chan BusEvent
	stats         BusStats
	mu            sync.RWMutex
	workers       sync.WaitGroup
	shutdown      chan struct{}
	closed        bool
}

// subscription represents an event subscription
type subscription struct {
	id      SubscriptionID
	filter  EventFilter
	handler EventHandler
	stats   subscriptionStats
}

// subscriptionStats tracks statistics for a subscription
type subscriptionStats struct {
	EventsReceived int64     `json:"events_received"`
	EventsHandled  int64     `json:"events_handled"`
	LastEventTime  time.Time `json:"last_event_time"`
	Errors         int64     `json:"errors"`
}

// NewEventBus creates a new event bus
func NewEventBus(config *EventBusConfig) *EventBusImpl {
	if config == nil {
		config = DefaultEventBusConfig()
	}

	bus := &EventBusImpl{
		config:        config,
		subscriptions: make(map[string][]*subscription),
		eventQueue:    make(chan BusEvent, config.BufferSize),
		shutdown:      make(chan struct{}),
	}

	// Start worker goroutines
	for i := 0; i < config.WorkerCount; i++ {
		bus.workers.Add(1)
		go bus.worker()
	}

	return bus
}

// Subscribe subscribes to events of a specific type
func (bus *EventBusImpl) Subscribe(eventType string, handler EventHandler) (SubscriptionID, error) {
	return bus.SubscribeWithFilter(eventType, nil, handler)
}

// SubscribeWithFilter subscribes with a custom filter
func (bus *EventBusImpl) SubscribeWithFilter(eventType string, filter EventFilter, handler EventHandler) (SubscriptionID, error) {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	if bus.closed {
		return "", fmt.Errorf("event bus is closed")
	}

	if handler == nil {
		return "", fmt.Errorf("handler cannot be nil")
	}

	id := SubscriptionID(fmt.Sprintf("%s_%d_%d", eventType, len(bus.subscriptions[eventType]), time.Now().UnixNano()))

	sub := &subscription{
		id:      id,
		filter:  filter,
		handler: handler,
	}

	bus.subscriptions[eventType] = append(bus.subscriptions[eventType], sub)
	bus.stats.TotalSubscriptions++
	bus.stats.ActiveSubscriptions++

	return id, nil
}

// Unsubscribe removes a subscription
func (bus *EventBusImpl) Unsubscribe(subscriptionID SubscriptionID) error {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	for eventType, subs := range bus.subscriptions {
		for i, sub := range subs {
			if sub.id == subscriptionID {
				// Remove subscription
				bus.subscriptions[eventType] = append(subs[:i], subs[i+1:]...)
				bus.stats.ActiveSubscriptions--
				return nil
			}
		}
	}

	return fmt.Errorf("subscription not found: %s", subscriptionID)
}

// Publish publishes an event to all subscribers
func (bus *EventBusImpl) Publish(ctx context.Context, event BusEvent) error {
	if bus.closed {
		return fmt.Errorf("event bus is closed")
	}

	if event.IsExpired() {
		bus.stats.EventsDropped++
		return fmt.Errorf("event expired")
	}

	// Set timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Set ID if not set
	if event.ID == "" {
		event.ID = fmt.Sprintf("%s_%d", event.Type, time.Now().UnixNano())
	}

	select {
	case bus.eventQueue <- event:
		bus.stats.EventsPublished++
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		if bus.config.DropOnFullBuffer {
			bus.stats.EventsDropped++
			return nil
		}
		// Block until space is available
		select {
		case bus.eventQueue <- event:
			bus.stats.EventsPublished++
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// PublishAsync publishes an event asynchronously
func (bus *EventBusImpl) PublishAsync(ctx context.Context, event BusEvent) error {
	if !bus.config.EnableAsync {
		return bus.Publish(ctx, event)
	}

	go func() {
		if err := bus.Publish(context.Background(), event); err != nil {
			bus.stats.DeliveryErrors++
			bus.stats.LastError = err
			bus.stats.LastErrorTime = time.Now()
		}
	}()

	return nil
}

// Close closes the event bus
func (bus *EventBusImpl) Close() error {
	bus.mu.Lock()
	if bus.closed {
		bus.mu.Unlock()
		return nil
	}
	bus.closed = true
	bus.mu.Unlock()

	close(bus.shutdown)
	bus.workers.Wait()
	close(bus.eventQueue)

	return nil
}

// GetStats returns event bus statistics
func (bus *EventBusImpl) GetStats() BusStats {
	bus.mu.RLock()
	defer bus.mu.RUnlock()

	stats := bus.stats
	stats.PendingEvents = len(bus.eventQueue)
	stats.QueueUtilization = float64(stats.PendingEvents) / float64(bus.config.BufferSize)

	return stats
}

// worker processes events from the queue
func (bus *EventBusImpl) worker() {
	defer bus.workers.Done()

	for {
		select {
		case event, ok := <-bus.eventQueue:
			if !ok {
				return
			}
			bus.deliverEvent(event)
		case <-bus.shutdown:
			return
		}
	}
}

// deliverEvent delivers an event to all matching subscribers
func (bus *EventBusImpl) deliverEvent(event BusEvent) {
	bus.mu.RLock()
	subs := bus.subscriptions[event.Type]
	bus.mu.RUnlock()

	if len(subs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), bus.config.DeliveryTimeout)
	defer cancel()

	for _, sub := range subs {
		// Apply filter if present
		if sub.filter != nil && !sub.filter(event) {
			bus.stats.EventsFiltered++
			continue
		}

		sub.stats.EventsReceived++
		sub.stats.LastEventTime = time.Now()

		// Deliver event with retry logic
		if err := bus.deliverToHandler(ctx, sub, event); err != nil {
			sub.stats.Errors++
			bus.stats.DeliveryErrors++
			bus.stats.LastError = err
			bus.stats.LastErrorTime = time.Now()
		} else {
			sub.stats.EventsHandled++
			bus.stats.EventsDelivered++
		}
	}
}

// deliverToHandler delivers an event to a specific handler with retry logic
func (bus *EventBusImpl) deliverToHandler(ctx context.Context, sub *subscription, event BusEvent) error {
	var lastErr error

	for attempt := 0; attempt <= bus.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(bus.config.RetryDelay):
			}
		}

		// Create a new context for each attempt
		handlerCtx, cancel := context.WithTimeout(ctx, bus.config.DeliveryTimeout)
		err := sub.handler(handlerCtx, event)
		cancel()

		if err == nil {
			return nil
		}

		lastErr = err
	}

	return fmt.Errorf("failed to deliver event after %d attempts: %w", bus.config.MaxRetries+1, lastErr)
}

// Common event types for inter-module communication
const (
	// Cache events
	EventTypeCacheHit        = "cache.hit"
	EventTypeCacheMiss       = "cache.miss"
	EventTypeCacheEviction   = "cache.eviction"
	EventTypeCacheInvalidate = "cache.invalidate"

	// Auth events
	EventTypeAuthSuccess    = "auth.success"
	EventTypeAuthFailure    = "auth.failure"
	EventTypeAuthExpiration = "auth.expiration"
	EventTypeAuthRevocation = "auth.revocation"

	// Distributed events
	EventTypeNodeJoin         = "distributed.node_join"
	EventTypeNodeLeave        = "distributed.node_leave"
	EventTypeNodeFailure      = "distributed.node_failure"
	EventTypeConsensusReached = "distributed.consensus_reached"

	// Validation events
	EventTypeValidationSuccess = "validation.success"
	EventTypeValidationFailure = "validation.failure"
	EventTypeValidationTimeout = "validation.timeout"

	// System events
	EventTypeSystemStart    = "system.start"
	EventTypeSystemShutdown = "system.shutdown"
	EventTypeSystemError    = "system.error"
)

// Event payload types
type CacheEventData struct {
	Key    string        `json:"key"`
	Value  interface{}   `json:"value,omitempty"`
	TTL    time.Duration `json:"ttl,omitempty"`
	Level  string        `json:"level"` // L1, L2
	NodeID string        `json:"node_id,omitempty"`
}

type AuthEventData struct {
	UserID    string     `json:"user_id"`
	Username  string     `json:"username,omitempty"`
	Provider  string     `json:"provider"`
	Reason    string     `json:"reason,omitempty"`
	TokenType string     `json:"token_type,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type DistributedEventData struct {
	NodeID      string                 `json:"node_id"`
	NodeAddress string                 `json:"node_address,omitempty"`
	ClusterSize int                    `json:"cluster_size,omitempty"`
	ShardInfo   map[string]interface{} `json:"shard_info,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type ValidationEventData struct {
	EventID     string        `json:"event_id"`
	EventType   string        `json:"event_type"`
	Valid       bool          `json:"valid"`
	Errors      []string      `json:"errors,omitempty"`
	Duration    time.Duration `json:"duration"`
	ValidatorID string        `json:"validator_id"`
}

// NewCacheEvent creates a cache-related event
func NewCacheEvent(eventType, source, key string, data interface{}) BusEvent {
	return BusEvent{
		ID:        fmt.Sprintf("cache_%d", time.Now().UnixNano()),
		Type:      eventType,
		Source:    source,
		Data:      CacheEventData{Key: key, Value: data},
		Timestamp: time.Now(),
		Priority:  1,
	}
}

// NewAuthEvent creates an auth-related event
func NewAuthEvent(eventType, source, userID string, data AuthEventData) BusEvent {
	data.UserID = userID
	return BusEvent{
		ID:        fmt.Sprintf("auth_%d", time.Now().UnixNano()),
		Type:      eventType,
		Source:    source,
		Data:      data,
		Timestamp: time.Now(),
		Priority:  2,
	}
}

// NewDistributedEvent creates a distributed system event
func NewDistributedEvent(eventType, source, nodeID string, data DistributedEventData) BusEvent {
	data.NodeID = nodeID
	return BusEvent{
		ID:        fmt.Sprintf("distributed_%d", time.Now().UnixNano()),
		Type:      eventType,
		Source:    source,
		Data:      data,
		Timestamp: time.Now(),
		Priority:  3,
	}
}

// NewValidationEvent creates a validation-related event
func NewValidationEvent(eventType, source, eventID string, data ValidationEventData) BusEvent {
	data.EventID = eventID
	return BusEvent{
		ID:        fmt.Sprintf("validation_%d", time.Now().UnixNano()),
		Type:      eventType,
		Source:    source,
		Data:      data,
		Timestamp: time.Now(),
		Priority:  1,
	}
}
