package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// EventDrivenCoordinator coordinates cache operations using event-driven architecture
// This replaces direct coupling with event-based communication
type EventDrivenCoordinator struct {
	nodeID   string
	eventBus events.EventBus
	config   *EventDrivenConfig

	// Local state
	nodeInfo *NodeInfo

	// Event subscriptions
	subscriptions map[string]events.SubscriptionID

	// Registered cache instances for local operations
	caches map[string]CacheValidatorInterface

	// Synchronization
	mu         sync.RWMutex
	shutdownCh chan struct{}
	started    bool
}

// EventDrivenConfig configuration for event-driven coordinator
type EventDrivenConfig struct {
	// Event publishing settings
	EventTimeout    time.Duration `json:"event_timeout"`
	AsyncPublishing bool          `json:"async_publishing"`

	// Health check settings
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	NodeTimeout         time.Duration `json:"node_timeout"`

	// Metrics collection
	EnableMetrics   bool          `json:"enable_metrics"`
	MetricsInterval time.Duration `json:"metrics_interval"`

	// Consensus settings
	EnableConsensus  bool          `json:"enable_consensus"`
	ConsensusTimeout time.Duration `json:"consensus_timeout"`
	QuorumRatio      float64       `json:"quorum_ratio"`
}

// DefaultEventDrivenConfig returns default configuration
func DefaultEventDrivenConfig() *EventDrivenConfig {
	return &EventDrivenConfig{
		EventTimeout:        5 * time.Second,
		AsyncPublishing:     true,
		HealthCheckInterval: 30 * time.Second,
		NodeTimeout:         60 * time.Second,
		EnableMetrics:       true,
		MetricsInterval:     30 * time.Second,
		EnableConsensus:     false,
		ConsensusTimeout:    10 * time.Second,
		QuorumRatio:         0.51,
	}
}

// NewEventDrivenCoordinator creates a new event-driven coordinator
func NewEventDrivenCoordinator(nodeID string, eventBus events.EventBus, config *EventDrivenConfig) *EventDrivenCoordinator {
	if config == nil {
		config = DefaultEventDrivenConfig()
	}

	return &EventDrivenCoordinator{
		nodeID:   nodeID,
		eventBus: eventBus,
		config:   config,
		nodeInfo: &NodeInfo{
			ID:            nodeID,
			State:         NodeStateActive,
			LastHeartbeat: time.Now(),
		},
		subscriptions: make(map[string]events.SubscriptionID),
		caches:        make(map[string]CacheValidatorInterface),
		shutdownCh:    make(chan struct{}),
	}
}

// Start starts the event-driven coordinator
func (edc *EventDrivenCoordinator) Start(ctx context.Context) error {
	edc.mu.Lock()
	defer edc.mu.Unlock()

	if edc.started {
		return fmt.Errorf("coordinator already started")
	}

	// Subscribe to relevant events
	if err := edc.subscribeToEvents(); err != nil {
		return fmt.Errorf("failed to subscribe to events: %w", err)
	}

	edc.started = true

	// Start background workers
	go edc.healthCheckWorker(ctx)
	if edc.config.EnableMetrics {
		go edc.metricsWorker(ctx)
	}

	// Announce node joining
	joinEvent := events.NewDistributedEvent(
		events.EventTypeNodeJoin,
		"cache_coordinator",
		edc.nodeID,
		events.DistributedEventData{
			NodeAddress: edc.nodeID,
			ClusterSize: 1, // Will be updated as we discover other nodes
			Metadata: map[string]interface{}{
				"cache_enabled":    true,
				"coordinator_type": "event_driven",
			},
		},
	)

	if edc.config.AsyncPublishing {
		edc.eventBus.PublishAsync(ctx, joinEvent)
	} else {
		edc.eventBus.Publish(ctx, joinEvent)
	}

	return nil
}

// Stop stops the event-driven coordinator
func (edc *EventDrivenCoordinator) Stop(ctx context.Context) error {
	edc.mu.Lock()
	defer edc.mu.Unlock()

	if !edc.started {
		return nil
	}

	// Announce node leaving
	leaveEvent := events.NewDistributedEvent(
		events.EventTypeNodeLeave,
		"cache_coordinator",
		edc.nodeID,
		events.DistributedEventData{
			Metadata: map[string]interface{}{
				"graceful_shutdown": true,
			},
		},
	)

	// Use sync publishing for shutdown to ensure delivery
	edc.eventBus.Publish(ctx, leaveEvent)

	// Unsubscribe from events
	for _, subID := range edc.subscriptions {
		edc.eventBus.Unsubscribe(subID)
	}

	// Signal shutdown
	close(edc.shutdownCh)
	edc.started = false

	return nil
}

// RegisterCache registers a cache validator for coordination
func (edc *EventDrivenCoordinator) RegisterCache(cacheID string, cache CacheValidatorInterface) {
	edc.mu.Lock()
	defer edc.mu.Unlock()
	edc.caches[cacheID] = cache
}

// UnregisterCache unregisters a cache validator
func (edc *EventDrivenCoordinator) UnregisterCache(cacheID string) {
	edc.mu.Lock()
	defer edc.mu.Unlock()
	delete(edc.caches, cacheID)
}

// InvalidateCache publishes a cache invalidation event
func (edc *EventDrivenCoordinator) InvalidateCache(ctx context.Context, eventType, key string) error {
	invalidateEvent := events.NewCacheEvent(
		events.EventTypeCacheInvalidate,
		"cache_coordinator",
		key,
		events.CacheEventData{
			Key:    key,
			NodeID: edc.nodeID,
		},
	)

	invalidateEvent.SetMetadata("invalidation_type", eventType)
	invalidateEvent.SetMetadata("originating_node", edc.nodeID)

	if edc.config.AsyncPublishing {
		return edc.eventBus.PublishAsync(ctx, invalidateEvent)
	}
	return edc.eventBus.Publish(ctx, invalidateEvent)
}

// InvalidateByEventType invalidates all cache entries for a specific event type
func (edc *EventDrivenCoordinator) InvalidateByEventType(ctx context.Context, eventType string) error {
	invalidateEvent := events.BusEvent{
		ID:     fmt.Sprintf("invalidate_type_%d", time.Now().UnixNano()),
		Type:   events.EventTypeCacheInvalidate,
		Source: "cache_coordinator",
		Data: events.CacheEventData{
			NodeID: edc.nodeID,
		},
		Metadata: map[string]interface{}{
			"invalidation_scope": "event_type",
			"event_type":         eventType,
			"originating_node":   edc.nodeID,
		},
		Timestamp: time.Now(),
		Priority:  3, // High priority for invalidations
	}

	if edc.config.AsyncPublishing {
		return edc.eventBus.PublishAsync(ctx, invalidateEvent)
	}
	return edc.eventBus.Publish(ctx, invalidateEvent)
}

// ReportMetrics publishes metrics as events
func (edc *EventDrivenCoordinator) ReportMetrics(ctx context.Context, stats CacheStats) error {
	metricsEvent := events.BusEvent{
		ID:     fmt.Sprintf("metrics_%s_%d", edc.nodeID, time.Now().UnixNano()),
		Type:   "cache.metrics",
		Source: "cache_coordinator",
		Data: map[string]interface{}{
			"node_id": edc.nodeID,
			"stats":   stats,
		},
		Timestamp: time.Now(),
		Priority:  1,
	}

	if edc.config.AsyncPublishing {
		return edc.eventBus.PublishAsync(ctx, metricsEvent)
	}
	return edc.eventBus.Publish(ctx, metricsEvent)
}

// subscribeToEvents subscribes to relevant events
func (edc *EventDrivenCoordinator) subscribeToEvents() error {
	// Subscribe to cache invalidation events
	invalidateSubID, err := edc.eventBus.Subscribe(
		events.EventTypeCacheInvalidate,
		edc.handleCacheInvalidationEvent,
	)
	if err != nil {
		return fmt.Errorf("failed to subscribe to cache invalidation events: %w", err)
	}
	edc.subscriptions["cache_invalidate"] = invalidateSubID

	// Subscribe to node join events
	joinSubID, err := edc.eventBus.Subscribe(
		events.EventTypeNodeJoin,
		edc.handleNodeJoinEvent,
	)
	if err != nil {
		return fmt.Errorf("failed to subscribe to node join events: %w", err)
	}
	edc.subscriptions["node_join"] = joinSubID

	// Subscribe to node leave events
	leaveSubID, err := edc.eventBus.Subscribe(
		events.EventTypeNodeLeave,
		edc.handleNodeLeaveEvent,
	)
	if err != nil {
		return fmt.Errorf("failed to subscribe to node leave events: %w", err)
	}
	edc.subscriptions["node_leave"] = leaveSubID

	// Subscribe to auth events for cache invalidation
	authSubID, err := edc.eventBus.Subscribe(
		events.EventTypeAuthExpiration,
		edc.handleAuthEvent,
	)
	if err != nil {
		return fmt.Errorf("failed to subscribe to auth events: %w", err)
	}
	edc.subscriptions["auth_expiration"] = authSubID

	return nil
}

// handleCacheInvalidationEvent handles cache invalidation events
func (edc *EventDrivenCoordinator) handleCacheInvalidationEvent(ctx context.Context, event events.BusEvent) error {
	// Don't process events from ourselves to avoid loops
	if originatingNode, exists := event.GetMetadata("originating_node"); exists {
		if nodeID, ok := originatingNode.(string); ok && nodeID == edc.nodeID {
			return nil
		}
	}

	edc.mu.RLock()
	caches := make(map[string]CacheValidatorInterface)
	for id, cache := range edc.caches {
		caches[id] = cache
	}
	edc.mu.RUnlock()

	// Determine invalidation scope
	if scope, exists := event.GetMetadata("invalidation_scope"); exists {
		if scope == "event_type" {
			eventType, _ := event.GetMetadata("event_type")
			if eventTypeStr, ok := eventType.(string); ok {
				// Invalidate by event type
				for _, cache := range caches {
					go func(c CacheValidatorInterface) {
						c.InvalidateEventType(ctx, eventTypeStr)
					}(cache)
				}
			}
		}
	} else {
		// Invalidate specific key
		if data, ok := event.Data.(events.CacheEventData); ok {
			for _, cache := range caches {
				go func(c CacheValidatorInterface, key string) {
					c.InvalidateByKeys(ctx, []string{key})
				}(cache, data.Key)
			}
		}
	}

	return nil
}

// handleNodeJoinEvent handles node join events
func (edc *EventDrivenCoordinator) handleNodeJoinEvent(ctx context.Context, event events.BusEvent) error {
	if data, ok := event.Data.(events.DistributedEventData); ok {
		// Update our knowledge of cluster topology
		// In a real implementation, this would update routing tables, etc.
		fmt.Printf("Node joined cluster: %s\n", data.NodeID)
	}
	return nil
}

// handleNodeLeaveEvent handles node leave events
func (edc *EventDrivenCoordinator) handleNodeLeaveEvent(ctx context.Context, event events.BusEvent) error {
	if data, ok := event.Data.(events.DistributedEventData); ok {
		// Handle node departure
		fmt.Printf("Node left cluster: %s\n", data.NodeID)
	}
	return nil
}

// handleAuthEvent handles authentication events for cache invalidation
func (edc *EventDrivenCoordinator) handleAuthEvent(ctx context.Context, event events.BusEvent) error {
	if data, ok := event.Data.(events.AuthEventData); ok {
		// Invalidate user-specific cache entries when auth expires
		userCachePattern := fmt.Sprintf("user:%s:*", data.UserID)

		edc.mu.RLock()
		caches := make(map[string]CacheValidatorInterface)
		for id, cache := range edc.caches {
			caches[id] = cache
		}
		edc.mu.RUnlock()

		// Publish cache invalidation for user-specific entries
		invalidateEvent := events.BusEvent{
			ID:     fmt.Sprintf("auth_invalidate_%s_%d", data.UserID, time.Now().UnixNano()),
			Type:   events.EventTypeCacheInvalidate,
			Source: "cache_coordinator",
			Data: events.CacheEventData{
				Key:    userCachePattern,
				NodeID: edc.nodeID,
			},
			Metadata: map[string]interface{}{
				"invalidation_scope": "pattern",
				"pattern":            userCachePattern,
				"reason":             "auth_expiration",
				"user_id":            data.UserID,
			},
			Timestamp: time.Now(),
			Priority:  3,
		}

		return edc.eventBus.Publish(ctx, invalidateEvent)
	}
	return nil
}

// healthCheckWorker periodically publishes health status
func (edc *EventDrivenCoordinator) healthCheckWorker(ctx context.Context) {
	ticker := time.NewTicker(edc.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-edc.shutdownCh:
			return
		case <-ticker.C:
			edc.publishHealthStatus(ctx)
		}
	}
}

// metricsWorker periodically publishes metrics
func (edc *EventDrivenCoordinator) metricsWorker(ctx context.Context) {
	ticker := time.NewTicker(edc.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-edc.shutdownCh:
			return
		case <-ticker.C:
			edc.publishMetrics(ctx)
		}
	}
}

// publishHealthStatus publishes the current health status
func (edc *EventDrivenCoordinator) publishHealthStatus(ctx context.Context) {
	healthEvent := events.BusEvent{
		ID:     fmt.Sprintf("health_%s_%d", edc.nodeID, time.Now().UnixNano()),
		Type:   "system.health",
		Source: "cache_coordinator",
		Data: map[string]interface{}{
			"node_id":     edc.nodeID,
			"status":      "healthy",
			"uptime":      time.Since(edc.nodeInfo.LastHeartbeat),
			"cache_count": len(edc.caches),
		},
		Timestamp: time.Now(),
		Priority:  1,
	}

	edc.eventBus.PublishAsync(ctx, healthEvent)
}

// publishMetrics publishes aggregated metrics from all managed caches
func (edc *EventDrivenCoordinator) publishMetrics(ctx context.Context) {
	edc.mu.RLock()
	defer edc.mu.RUnlock()

	// Aggregate metrics from all caches
	aggregatedMetrics := make(map[string]interface{})

	for cacheID, cache := range edc.caches {
		if cv, ok := cache.(*CacheValidatorSimple); ok {
			stats := cv.GetStats()
			aggregatedMetrics[cacheID] = stats
		}
	}

	if len(aggregatedMetrics) > 0 {
		metricsEvent := events.BusEvent{
			ID:     fmt.Sprintf("cache_metrics_%s_%d", edc.nodeID, time.Now().UnixNano()),
			Type:   "cache.metrics",
			Source: "cache_coordinator",
			Data: map[string]interface{}{
				"node_id": edc.nodeID,
				"metrics": aggregatedMetrics,
			},
			Timestamp: time.Now(),
			Priority:  1,
		}

		edc.eventBus.PublishAsync(ctx, metricsEvent)
	}
}

// GetStats returns coordinator statistics
func (edc *EventDrivenCoordinator) GetStats() map[string]interface{} {
	edc.mu.RLock()
	defer edc.mu.RUnlock()

	return map[string]interface{}{
		"node_id":         edc.nodeID,
		"started":         edc.started,
		"cache_count":     len(edc.caches),
		"subscriptions":   len(edc.subscriptions),
		"event_bus_stats": edc.eventBus.GetStats(),
	}
}
