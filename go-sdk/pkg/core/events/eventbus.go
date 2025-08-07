package events

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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
	// Subscription statistics (atomic access required)
	TotalSubscriptions  int64 `json:"total_subscriptions"`
	ActiveSubscriptions int64 `json:"active_subscriptions"`

	// Event statistics (atomic access required)
	EventsPublished int64 `json:"events_published"`
	EventsDelivered int64 `json:"events_delivered"`
	EventsDropped   int64 `json:"events_dropped"`
	EventsFiltered  int64 `json:"events_filtered"`

	// Performance statistics
	AverageDeliveryTime time.Duration `json:"average_delivery_time"`
	PendingEvents       int           `json:"pending_events"`
	QueueUtilization    float64       `json:"queue_utilization"`

	// Error statistics (atomic access required for counters, mutex for error details)
	DeliveryErrors int64     `json:"delivery_errors"`
	LastError      error     `json:"last_error,omitempty"`
	LastErrorTime  time.Time `json:"last_error_time,omitempty"`

	// Circuit breaker statistics
	CircuitBreakerState   string        `json:"circuit_breaker_state,omitempty"`
	CircuitBreakerFailures int64        `json:"circuit_breaker_failures,omitempty"`
	CircuitBreakerRecovery time.Duration `json:"circuit_breaker_recovery,omitempty"`
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

	// PublishTimeout is the timeout for publish operations (prevents deadlocks)
	PublishTimeout time.Duration `json:"publish_timeout"`

	// CircuitBreakerConfig configures circuit breaker for resilience
	CircuitBreakerConfig *CircuitBreakerConfig `json:"circuit_breaker_config,omitempty"`
}

// CircuitBreakerConfig configures the circuit breaker
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures to trigger circuit breaker
	FailureThreshold int `json:"failure_threshold"`

	// RecoveryTimeout is how long to wait before attempting recovery
	RecoveryTimeout time.Duration `json:"recovery_timeout"`

	// MaxConcurrentRequests limits concurrent requests during half-open state
	MaxConcurrentRequests int `json:"max_concurrent_requests"`
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
		PublishTimeout:   5 * time.Second, // Prevent deadlocks with timeout
		CircuitBreakerConfig: &CircuitBreakerConfig{
			FailureThreshold:      10,
			RecoveryTimeout:       30 * time.Second,
			MaxConcurrentRequests: 100,
		},
	}
}

// EventBusImpl implements the EventBus interface
type EventBusImpl struct {
	config         *EventBusConfig
	subscriptions  map[string][]*subscription
	eventQueue     chan BusEvent
	stats          BusStats
	mu             sync.RWMutex
	errorMu        sync.RWMutex // Separate mutex for error-related stats
	workers        sync.WaitGroup
	shutdown       chan struct{}
	closed         bool
	circuitBreaker *CircuitBreaker
}

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int32

const (
	Closed CircuitBreakerState = iota
	Open
	HalfOpen
)

// CircuitBreaker provides circuit breaker functionality for resilience
type CircuitBreaker struct {
	config           *CircuitBreakerConfig
	state            int32 // CircuitBreakerState
	failureCount     int64
	lastFailureTime  int64 // Unix timestamp
	concurrentReqs   int64
	mu               sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = &CircuitBreakerConfig{
			FailureThreshold:      10,
			RecoveryTimeout:       30 * time.Second,
			MaxConcurrentRequests: 100,
		}
	}
	return &CircuitBreaker{
		config: config,
		state:  int32(Closed),
	}
}

// CanExecute checks if the circuit breaker allows execution
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	state := CircuitBreakerState(atomic.LoadInt32(&cb.state))
	switch state {
	case Closed:
		return true
	case Open:
		// Check if we should transition to half-open
		lastFailure := atomic.LoadInt64(&cb.lastFailureTime)
		if time.Since(time.Unix(lastFailure, 0)) > cb.config.RecoveryTimeout {
			// Try to transition to half-open
			if atomic.CompareAndSwapInt32(&cb.state, int32(Open), int32(HalfOpen)) {
				return atomic.LoadInt64(&cb.concurrentReqs) < int64(cb.config.MaxConcurrentRequests)
			}
		}
		return false
	case HalfOpen:
		return atomic.LoadInt64(&cb.concurrentReqs) < int64(cb.config.MaxConcurrentRequests)
	default:
		return false
	}
}

// RecordSuccess records a successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	atomic.StoreInt64(&cb.failureCount, 0)
	if atomic.LoadInt32(&cb.state) == int32(HalfOpen) {
		atomic.StoreInt32(&cb.state, int32(Closed))
	}
	atomic.AddInt64(&cb.concurrentReqs, -1)
}

// RecordFailure records a failed operation
func (cb *CircuitBreaker) RecordFailure() {
	atomic.AddInt64(&cb.concurrentReqs, -1)
	failures := atomic.AddInt64(&cb.failureCount, 1)
	atomic.StoreInt64(&cb.lastFailureTime, time.Now().Unix())

	if failures >= int64(cb.config.FailureThreshold) {
		atomic.StoreInt32(&cb.state, int32(Open))
	}
}

// Execute executes an operation with circuit breaker protection
func (cb *CircuitBreaker) Execute(operation func() error) error {
	if !cb.CanExecute() {
		return fmt.Errorf("circuit breaker is open")
	}

	atomic.AddInt64(&cb.concurrentReqs, 1)
	err := operation()
	if err != nil {
		cb.RecordFailure()
		return err
	}

	cb.RecordSuccess()
	return nil
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
	EventsReceived int64     `json:"events_received"` // Atomic access required
	EventsHandled  int64     `json:"events_handled"`  // Atomic access required
	LastEventTime  time.Time `json:"last_event_time"` // Mutex protected
	Errors         int64     `json:"errors"`          // Atomic access required
	mu             sync.RWMutex // Protects LastEventTime
}

// NewEventBus creates a new event bus
func NewEventBus(config *EventBusConfig) *EventBusImpl {
	if config == nil {
		config = DefaultEventBusConfig()
	}

	bus := &EventBusImpl{
		config:         config,
		subscriptions:  make(map[string][]*subscription),
		eventQueue:     make(chan BusEvent, config.BufferSize),
		shutdown:       make(chan struct{}),
		circuitBreaker: NewCircuitBreaker(config.CircuitBreakerConfig),
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
	atomic.AddInt64(&bus.stats.TotalSubscriptions, 1)
	atomic.AddInt64(&bus.stats.ActiveSubscriptions, 1)

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
				atomic.AddInt64(&bus.stats.ActiveSubscriptions, -1)
				return nil
			}
		}
	}

	return fmt.Errorf("subscription not found: %s", subscriptionID)
}

// Publish publishes an event to all subscribers with deadlock prevention
func (bus *EventBusImpl) Publish(ctx context.Context, event BusEvent) error {
	if bus.closed {
		return fmt.Errorf("event bus is closed")
	}

	if event.IsExpired() {
		atomic.AddInt64(&bus.stats.EventsDropped, 1)
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

	// Use circuit breaker to prevent cascade failures
	return bus.circuitBreaker.Execute(func() error {
		return bus.publishWithTimeout(ctx, event)
	})
}

// publishWithTimeout implements non-blocking publish with configurable timeout
func (bus *EventBusImpl) publishWithTimeout(ctx context.Context, event BusEvent) error {
	// Create timeout context to prevent deadlocks
	publishCtx := ctx
	if bus.config.PublishTimeout > 0 {
		var cancel context.CancelFunc
		publishCtx, cancel = context.WithTimeout(ctx, bus.config.PublishTimeout)
		defer cancel()
	}

	// First, try non-blocking send
	select {
	case bus.eventQueue <- event:
		atomic.AddInt64(&bus.stats.EventsPublished, 1)
		return nil
	case <-publishCtx.Done():
		return publishCtx.Err()
	default:
		// Queue is full, handle based on configuration
		if bus.config.DropOnFullBuffer {
			atomic.AddInt64(&bus.stats.EventsDropped, 1)
			return nil // Successfully dropped, not an error
		}

		// Apply backpressure with timeout to prevent deadlocks
		return bus.publishWithBackpressure(publishCtx, event)
	}
}

// publishWithBackpressure handles backpressure scenarios with timeout protection
func (bus *EventBusImpl) publishWithBackpressure(ctx context.Context, event BusEvent) error {
	// Use a ticker to periodically check if we should abort due to high load
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Track queue utilization for adaptive behavior
	startTime := time.Now()
	maxWaitTime := bus.config.PublishTimeout
	if maxWaitTime <= 0 {
		maxWaitTime = 5 * time.Second // Default safety timeout
	}

	for {
		select {
		case bus.eventQueue <- event:
			atomic.AddInt64(&bus.stats.EventsPublished, 1)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Periodic check for abort conditions
			if time.Since(startTime) > maxWaitTime {
				atomic.AddInt64(&bus.stats.EventsDropped, 1)
				return fmt.Errorf("publish timeout after %v - preventing deadlock", maxWaitTime)
			}

			// Adaptive behavior: if queue is consistently full, consider dropping
			queueUtilization := float64(len(bus.eventQueue)) / float64(bus.config.BufferSize)
			if queueUtilization > 0.95 && time.Since(startTime) > time.Second {
				// Very high utilization for over 1 second, drop to prevent system overload
				atomic.AddInt64(&bus.stats.EventsDropped, 1)
				return fmt.Errorf("dropping event due to sustained high queue utilization (%.2f%%)", queueUtilization*100)
			}
		}
	}
}

// PublishAsync publishes an event asynchronously with enhanced error handling
func (bus *EventBusImpl) PublishAsync(ctx context.Context, event BusEvent) error {
	if !bus.config.EnableAsync {
		return bus.Publish(ctx, event)
	}

	// Use a worker pool approach to prevent goroutine explosion under high load
	select {
	case bus.eventQueue <- event:
		// Direct queue insertion succeeded, count as published
		atomic.AddInt64(&bus.stats.EventsPublished, 1)
		return nil
	default:
		// Queue is full, use goroutine with timeout protection
		go func() {
			// Create a timeout context for the async operation
			asyncCtx, cancel := context.WithTimeout(context.Background(), bus.config.PublishTimeout)
			defer cancel()

			if err := bus.publishWithTimeout(asyncCtx, event); err != nil {
				atomic.AddInt64(&bus.stats.DeliveryErrors, 1)
				bus.errorMu.Lock()
				bus.stats.LastError = err
				bus.stats.LastErrorTime = time.Now()
				bus.errorMu.Unlock()

				// Log the error for monitoring (in production, this would go to your logger)
				fmt.Printf("Async event publish failed: %v\n", err)
			}
		}()
		return nil
	}
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

// GetStats returns event bus statistics with circuit breaker info
func (bus *EventBusImpl) GetStats() BusStats {
	// Create a snapshot of stats using atomic reads for thread-safe access
	stats := BusStats{
		TotalSubscriptions:  atomic.LoadInt64(&bus.stats.TotalSubscriptions),
		ActiveSubscriptions: atomic.LoadInt64(&bus.stats.ActiveSubscriptions),
		EventsPublished:     atomic.LoadInt64(&bus.stats.EventsPublished),
		EventsDelivered:     atomic.LoadInt64(&bus.stats.EventsDelivered),
		EventsDropped:       atomic.LoadInt64(&bus.stats.EventsDropped),
		EventsFiltered:      atomic.LoadInt64(&bus.stats.EventsFiltered),
		DeliveryErrors:      atomic.LoadInt64(&bus.stats.DeliveryErrors),
	}

	// Get error-related stats with proper locking
	bus.errorMu.RLock()
	stats.LastError = bus.stats.LastError
	stats.LastErrorTime = bus.stats.LastErrorTime
	bus.errorMu.RUnlock()

	// Calculate current queue metrics
	stats.PendingEvents = len(bus.eventQueue)
	stats.QueueUtilization = float64(stats.PendingEvents) / float64(bus.config.BufferSize)

	// Add circuit breaker statistics
	cbState := bus.GetCircuitBreakerState()
	switch cbState {
	case Closed:
		stats.CircuitBreakerState = "closed"
	case Open:
		stats.CircuitBreakerState = "open"
	case HalfOpen:
		stats.CircuitBreakerState = "half_open"
	}
	stats.CircuitBreakerFailures = atomic.LoadInt64(&bus.circuitBreaker.failureCount)
	if bus.config.CircuitBreakerConfig != nil {
		stats.CircuitBreakerRecovery = bus.config.CircuitBreakerConfig.RecoveryTimeout
	}

	return stats
}

// GetCircuitBreakerState returns the current circuit breaker state for monitoring
func (bus *EventBusImpl) GetCircuitBreakerState() CircuitBreakerState {
	return CircuitBreakerState(atomic.LoadInt32(&bus.circuitBreaker.state))
}

// GetCircuitBreakerStats returns circuit breaker statistics
func (bus *EventBusImpl) GetCircuitBreakerStats() map[string]interface{} {
	return map[string]interface{}{
		"state":            bus.GetCircuitBreakerState(),
		"failure_count":    atomic.LoadInt64(&bus.circuitBreaker.failureCount),
		"concurrent_reqs":  atomic.LoadInt64(&bus.circuitBreaker.concurrentReqs),
		"last_failure":     time.Unix(atomic.LoadInt64(&bus.circuitBreaker.lastFailureTime), 0),
	}
}

// worker processes events from the queue with enhanced error handling
func (bus *EventBusImpl) worker() {
	defer bus.workers.Done()

	// Recovery mechanism for panics in event processing
	defer func() {
		if r := recover(); r != nil {
			atomic.AddInt64(&bus.stats.DeliveryErrors, 1)
			bus.errorMu.Lock()
			bus.stats.LastError = fmt.Errorf("worker panic: %v", r)
			bus.stats.LastErrorTime = time.Now()
			bus.errorMu.Unlock()
			
			// Log the panic for debugging
			fmt.Printf("EventBus worker panic recovered: %v\n", r)
		}
	}()

	for {
		select {
		case event, ok := <-bus.eventQueue:
			if !ok {
				return
			}
			// Process event with timeout to prevent worker blocking
			bus.deliverEventSafely(event)
		case <-bus.shutdown:
			return
		}
	}
}

// deliverEventSafely delivers an event with additional safety measures
func (bus *EventBusImpl) deliverEventSafely(event BusEvent) {
	// Create a timeout context for event delivery to prevent worker blocking
	ctx, cancel := context.WithTimeout(context.Background(), bus.config.DeliveryTimeout)
	defer cancel()

	// Use a goroutine with timeout to prevent blocking the worker
	done := make(chan struct{})
	go func() {
		defer close(done)
		bus.deliverEvent(event)
	}()

	select {
	case <-done:
		// Event delivered successfully
		return
	case <-ctx.Done():
		// Event delivery timed out
		atomic.AddInt64(&bus.stats.DeliveryErrors, 1)
		atomic.AddInt64(&bus.stats.EventsDropped, 1)
		bus.errorMu.Lock()
		bus.stats.LastError = fmt.Errorf("event delivery timeout: %s", event.Type)
		bus.stats.LastErrorTime = time.Now()
		bus.errorMu.Unlock()
	}
}

// deliverEvent delivers an event to all matching subscribers with enhanced safety
func (bus *EventBusImpl) deliverEvent(event BusEvent) {
	bus.mu.RLock()
	subs := bus.subscriptions[event.Type]
	bus.mu.RUnlock()

	if len(subs) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), bus.config.DeliveryTimeout)
	defer cancel()

	// Use a sync.WaitGroup to ensure all deliveries complete or timeout
	var wg sync.WaitGroup
	errorChan := make(chan error, len(subs))

	for _, sub := range subs {
		// Apply filter if present
		if sub.filter != nil && !sub.filter(event) {
			atomic.AddInt64(&bus.stats.EventsFiltered, 1)
			continue
		}

		atomic.AddInt64(&sub.stats.EventsReceived, 1)
		sub.stats.mu.Lock()
		sub.stats.LastEventTime = time.Now()
		sub.stats.mu.Unlock()

		wg.Add(1)
		go func(subscription *subscription) {
			defer wg.Done()
			
			// Deliver event with retry logic
			if err := bus.deliverToHandler(ctx, subscription, event); err != nil {
				atomic.AddInt64(&subscription.stats.Errors, 1)
				atomic.AddInt64(&bus.stats.DeliveryErrors, 1)
				bus.errorMu.Lock()
				bus.stats.LastError = err
				bus.stats.LastErrorTime = time.Now()
				bus.errorMu.Unlock()
				
				errorChan <- err
			} else {
				atomic.AddInt64(&subscription.stats.EventsHandled, 1)
				atomic.AddInt64(&bus.stats.EventsDelivered, 1)
			}
		}(sub)
	}

	// Wait for all deliveries to complete with timeout protection
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All deliveries completed
	case <-ctx.Done():
		// Timeout occurred, some deliveries may still be pending
		// This is logged for monitoring but doesn't block the system
	}

	// Close error channel and drain any errors for logging
	close(errorChan)
	errorCount := 0
	for err := range errorChan {
		errorCount++
		if errorCount <= 5 { // Log only first few errors to avoid spam
			fmt.Printf("Event delivery error: %v\n", err)
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
