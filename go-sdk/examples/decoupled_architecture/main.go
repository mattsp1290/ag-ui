package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events/cache"
)

// Event type constants
const (
	EventTypeAuthSuccess    = "auth.success"
	EventTypeAuthExpiration = "auth.expiration"
	EventTypeCacheHit       = "cache.hit"
	EventTypeCacheMiss      = "cache.miss"
	EventTypeNodeJoin       = "node.join"
	EventTypeNodeLeave      = "node.leave"
)

// AuthEventData represents authentication event data
type AuthEventData struct {
	Username  string `json:"username"`
	UserID    string `json:"user_id,omitempty"`
	Provider  string `json:"provider,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// NewAuthEvent creates a new authentication event
func NewAuthEvent(eventType, source, userID string, data AuthEventData) events.BusEvent {
	return events.BusEvent{
		ID:        fmt.Sprintf("auth-%d", time.Now().UnixNano()),
		Type:      eventType,
		Source:    source,
		Data:      data,
		Timestamp: time.Now(),
		Priority:  1,
	}
}

// CacheEventData represents cache event data
type CacheEventData struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value,omitempty"`
	TTL   int64       `json:"ttl,omitempty"`
	Hit   bool        `json:"hit"`
}

// NewCacheEvent creates a new cache event
func NewCacheEvent(eventType, source, key string, data interface{}) events.BusEvent {
	return events.BusEvent{
		ID:     fmt.Sprintf("cache-%d", time.Now().UnixNano()),
		Type:   eventType,
		Source: source,
		Data: CacheEventData{
			Key:   key,
			Value: data,
			Hit:   eventType == EventTypeCacheHit,
		},
		Timestamp: time.Now(),
		Priority:  0,
	}
}

// NodeEventData represents node event data
type NodeEventData struct {
	NodeID      string `json:"node_id"`
	NodeAddress string `json:"node_address,omitempty"`
	State       string `json:"state,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

// NewNodeEvent creates a new node event
func NewNodeEvent(eventType, source, nodeID string, data NodeEventData) events.BusEvent {
	return events.BusEvent{
		ID:        fmt.Sprintf("node-%d", time.Now().UnixNano()),
		Type:      eventType,
		Source:    source,
		Data:      data,
		Timestamp: time.Now(),
		Priority:  2,
	}
}

// DistributedEventData represents distributed system event data
type DistributedEventData struct {
	NodeID      string                 `json:"node_id"`
	NodeAddress string                 `json:"node_address,omitempty"`
	ClusterSize int                    `json:"cluster_size,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NewDistributedEvent creates a new distributed system event
func NewDistributedEvent(eventType, source, nodeID string, data DistributedEventData) events.BusEvent {
	data.NodeID = nodeID
	return events.BusEvent{
		ID:        fmt.Sprintf("dist-%d", time.Now().UnixNano()),
		Type:      eventType,
		Source:    source,
		Data:      data,
		Timestamp: time.Now(),
		Priority:  2,
	}
}

// DecoupledSystemExample demonstrates event-driven architecture
// This shows how auth, cache, and distributed modules communicate
// through events instead of direct coupling
type DecoupledSystemExample struct {
	eventBus           events.EventBus
	authManager        *AuthManager
	cacheManager       *CacheManager
	distributedManager *DistributedManager
}

// AuthManager handles authentication using event-driven approach
type AuthManager struct {
	eventBus  events.EventBus
	nodeID    string
	userStore map[string]bool // Simple user store for demo

	// Subscriptions
	subscriptions map[string]events.SubscriptionID
}

// CacheManager handles caching using event-driven approach
type CacheManager struct {
	eventBus       events.EventBus
	coordinator    *cache.EventDrivenCoordinator
	cacheValidator *cache.CacheValidator
	nodeID         string

	// Subscriptions
	subscriptions map[string]events.SubscriptionID
}

// DistributedManager handles distributed operations using event-driven approach
type DistributedManager struct {
	eventBus     events.EventBus
	nodeID       string
	clusterNodes map[string]*NodeStatus

	// Subscriptions
	subscriptions map[string]events.SubscriptionID
}

// NodeStatus tracks the status of nodes in the cluster
type NodeStatus struct {
	ID           string
	Address      string
	LastSeen     time.Time
	Status       string
	Capabilities []string
}

// NewDecoupledSystemExample creates a new decoupled system example
func NewDecoupledSystemExample(nodeID string) *DecoupledSystemExample {
	// Create event bus
	eventBus := events.NewEventBus(events.DefaultEventBusConfig())

	// Create managers
	authManager := &AuthManager{
		eventBus:      eventBus,
		nodeID:        nodeID,
		userStore:     make(map[string]bool),
		subscriptions: make(map[string]events.SubscriptionID),
	}

	cacheManager := &CacheManager{
		eventBus:      eventBus,
		nodeID:        nodeID,
		subscriptions: make(map[string]events.SubscriptionID),
	}

	distributedManager := &DistributedManager{
		eventBus:      eventBus,
		nodeID:        nodeID,
		clusterNodes:  make(map[string]*NodeStatus),
		subscriptions: make(map[string]events.SubscriptionID),
	}

	return &DecoupledSystemExample{
		eventBus:           eventBus,
		authManager:        authManager,
		cacheManager:       cacheManager,
		distributedManager: distributedManager,
	}
}

// Start initializes the decoupled system
func (dse *DecoupledSystemExample) Start(ctx context.Context) error {
	// Start all managers
	if err := dse.authManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start auth manager: %w", err)
	}

	if err := dse.cacheManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start cache manager: %w", err)
	}

	if err := dse.distributedManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start distributed manager: %w", err)
	}

	log.Printf("Decoupled system started with node ID: %s", dse.authManager.nodeID)
	return nil
}

// Stop shuts down the decoupled system
func (dse *DecoupledSystemExample) Stop(ctx context.Context) error {
	dse.authManager.Stop(ctx)
	dse.cacheManager.Stop(ctx)
	dse.distributedManager.Stop(ctx)
	dse.eventBus.Close()

	log.Println("Decoupled system stopped")
	return nil
}

// Simulate demonstrates the event-driven interactions
func (dse *DecoupledSystemExample) Simulate(ctx context.Context) error {
	log.Println("Starting event-driven simulation...")

	// Simulate user authentication
	dse.simulateAuthentication(ctx, "user123", "password")

	// Wait a bit for events to propagate
	time.Sleep(100 * time.Millisecond)

	// Simulate cache operations
	dse.simulateCacheOperations(ctx)

	// Wait a bit for events to propagate
	time.Sleep(100 * time.Millisecond)

	// Simulate distributed node operations
	dse.simulateDistributedOperations(ctx)

	// Wait a bit for events to propagate
	time.Sleep(100 * time.Millisecond)

	// Simulate auth expiration (should trigger cache invalidation)
	dse.simulateAuthExpiration(ctx, "user123")

	// Wait for final events to propagate
	time.Sleep(100 * time.Millisecond)

	log.Println("Event-driven simulation completed")
	return nil
}

// AuthManager implementation

// Start starts the auth manager
func (am *AuthManager) Start(ctx context.Context) error {
	// Subscribe to relevant events
	subID, err := am.eventBus.Subscribe(EventTypeAuthExpiration, am.handleAuthExpiration)
	if err != nil {
		return err
	}
	am.subscriptions["auth_expiration"] = subID

	// Add some demo users
	am.userStore["user123"] = true
	am.userStore["admin"] = true

	log.Printf("Auth manager started on node %s", am.nodeID)
	return nil
}

// Stop stops the auth manager
func (am *AuthManager) Stop(ctx context.Context) error {
	for _, subID := range am.subscriptions {
		am.eventBus.Unsubscribe(subID)
	}
	log.Printf("Auth manager stopped on node %s", am.nodeID)
	return nil
}

// Authenticate authenticates a user and publishes events
func (am *AuthManager) Authenticate(ctx context.Context, username, password string) error {
	// Simple authentication check
	if !am.userStore[username] {
		return fmt.Errorf("user not found: %s", username)
	}

	// Publish auth success event
	authEvent := NewAuthEvent(
		EventTypeAuthSuccess,
		"auth_manager",
		username,
		AuthEventData{
			Username:  username,
			Provider:  "basic",
			TokenType: "session",
		},
	)

	err := am.eventBus.Publish(ctx, authEvent)
	if err != nil {
		log.Printf("Failed to publish auth success event: %v", err)
		return err
	}

	log.Printf("User %s authenticated successfully", username)
	return nil
}

// ExpireAuth simulates auth expiration
func (am *AuthManager) ExpireAuth(ctx context.Context, userID string) {
	authEvent := NewAuthEvent(
		EventTypeAuthExpiration,
		"auth_manager",
		userID,
		AuthEventData{
			Reason: "session_timeout",
		},
	)

	am.eventBus.PublishAsync(ctx, authEvent)
	log.Printf("Auth expired for user %s", userID)
}

// handleAuthExpiration handles auth expiration events
func (am *AuthManager) handleAuthExpiration(ctx context.Context, event events.BusEvent) error {
	if data, ok := event.Data.(AuthEventData); ok {
		log.Printf("Handling auth expiration for user %s", data.UserID)
		// Additional cleanup logic could go here
	}
	return nil
}

// CacheManager implementation

// Start starts the cache manager
func (cm *CacheManager) Start(ctx context.Context) error {
	// Create event-driven coordinator
	cm.coordinator = cache.NewEventDrivenCoordinator(
		cm.nodeID,
		cm.eventBus,
		cache.DefaultEventDrivenConfig(),
	)

	// Start coordinator
	if err := cm.coordinator.Start(ctx); err != nil {
		return err
	}

	// Subscribe to cache-related events
	subID, err := cm.eventBus.Subscribe(EventTypeCacheHit, cm.handleCacheHit)
	if err != nil {
		return err
	}
	cm.subscriptions["cache_hit"] = subID

	log.Printf("Cache manager started on node %s", cm.nodeID)
	return nil
}

// Stop stops the cache manager
func (cm *CacheManager) Stop(ctx context.Context) error {
	if cm.coordinator != nil {
		cm.coordinator.Stop(ctx)
	}

	for _, subID := range cm.subscriptions {
		cm.eventBus.Unsubscribe(subID)
	}

	log.Printf("Cache manager stopped on node %s", cm.nodeID)
	return nil
}

// CacheGet simulates a cache get operation
func (cm *CacheManager) CacheGet(ctx context.Context, key string) ([]byte, error) {
	// Simulate cache operation
	data := []byte(fmt.Sprintf("cached_data_for_%s", key))

	// Publish cache hit event
	cacheEvent := NewCacheEvent(EventTypeCacheHit, "cache_manager", key, data)
	cm.eventBus.PublishAsync(ctx, cacheEvent)

	log.Printf("Cache hit for key %s", key)
	return data, nil
}

// handleCacheHit handles cache hit events
func (cm *CacheManager) handleCacheHit(ctx context.Context, event events.BusEvent) error {
	if data, ok := event.Data.(CacheEventData); ok {
		log.Printf("Recorded cache hit for key %s", data.Key)
		// Update metrics, etc.
	}
	return nil
}

// DistributedManager implementation

// Start starts the distributed manager
func (dm *DistributedManager) Start(ctx context.Context) error {
	// Subscribe to distributed events
	joinSubID, err := dm.eventBus.Subscribe(EventTypeNodeJoin, dm.handleNodeJoin)
	if err != nil {
		return err
	}
	dm.subscriptions["node_join"] = joinSubID

	leaveSubID, err := dm.eventBus.Subscribe(EventTypeNodeLeave, dm.handleNodeLeave)
	if err != nil {
		return err
	}
	dm.subscriptions["node_leave"] = leaveSubID

	// Announce this node joining
	joinEvent := NewDistributedEvent(
		EventTypeNodeJoin,
		"distributed_manager",
		dm.nodeID,
		DistributedEventData{
			NodeAddress: fmt.Sprintf("node://%s", dm.nodeID),
			ClusterSize: 1,
			Metadata: map[string]interface{}{
				"capabilities": []string{"cache", "auth", "compute"},
			},
		},
	)

	dm.eventBus.PublishAsync(ctx, joinEvent)
	log.Printf("Distributed manager started on node %s", dm.nodeID)
	return nil
}

// Stop stops the distributed manager
func (dm *DistributedManager) Stop(ctx context.Context) error {
	// Announce leaving
	leaveEvent := NewDistributedEvent(
		EventTypeNodeLeave,
		"distributed_manager",
		dm.nodeID,
		DistributedEventData{
			Metadata: map[string]interface{}{
				"graceful_shutdown": true,
			},
		},
	)

	dm.eventBus.Publish(ctx, leaveEvent)

	for _, subID := range dm.subscriptions {
		dm.eventBus.Unsubscribe(subID)
	}

	log.Printf("Distributed manager stopped on node %s", dm.nodeID)
	return nil
}

// handleNodeJoin handles node join events
func (dm *DistributedManager) handleNodeJoin(ctx context.Context, event events.BusEvent) error {
	if data, ok := event.Data.(DistributedEventData); ok {
		if data.NodeID != dm.nodeID { // Don't track ourselves
			dm.clusterNodes[data.NodeID] = &NodeStatus{
				ID:           data.NodeID,
				Address:      data.NodeAddress,
				LastSeen:     time.Now(),
				Status:       "active",
				Capabilities: []string{"cache", "auth"},
			}
			log.Printf("Node %s joined the cluster", data.NodeID)
		}
	}
	return nil
}

// handleNodeLeave handles node leave events
func (dm *DistributedManager) handleNodeLeave(ctx context.Context, event events.BusEvent) error {
	if data, ok := event.Data.(DistributedEventData); ok {
		if data.NodeID != dm.nodeID {
			delete(dm.clusterNodes, data.NodeID)
			log.Printf("Node %s left the cluster", data.NodeID)
		}
	}
	return nil
}

// Simulation helpers

func (dse *DecoupledSystemExample) simulateAuthentication(ctx context.Context, username, password string) {
	log.Printf("Simulating authentication for user: %s", username)
	dse.authManager.Authenticate(ctx, username, password)
}

func (dse *DecoupledSystemExample) simulateCacheOperations(ctx context.Context) {
	log.Println("Simulating cache operations")
	dse.cacheManager.CacheGet(ctx, "user:user123:profile")
	dse.cacheManager.CacheGet(ctx, "user:user123:preferences")
}

func (dse *DecoupledSystemExample) simulateDistributedOperations(ctx context.Context) {
	log.Println("Simulating distributed operations")
	// The distributed manager automatically announced its presence on start
	// Here we could simulate other distributed operations
}

func (dse *DecoupledSystemExample) simulateAuthExpiration(ctx context.Context, userID string) {
	log.Printf("Simulating auth expiration for user: %s", userID)
	dse.authManager.ExpireAuth(ctx, userID)
}

// main demonstrates the decoupled event-driven architecture
func main() {
	fmt.Println("AG-UI Decoupled Architecture Example")
	fmt.Println("====================================")

	ctx := context.Background()

	// Create the system
	system := NewDecoupledSystemExample("node-1")

	// Start all components
	if err := system.Start(ctx); err != nil {
		log.Fatalf("Failed to start system: %v", err)
	}

	// Run simulations
	time.Sleep(500 * time.Millisecond)

	// Simulate authentication
	system.simulateAuthentication(ctx, "user123", "password")
	time.Sleep(100 * time.Millisecond)

	// Simulate cache operations
	system.simulateCacheOperations(ctx)
	time.Sleep(100 * time.Millisecond)

	// Simulate distributed operations
	system.simulateDistributedOperations(ctx)
	time.Sleep(100 * time.Millisecond)

	// Simulate auth expiration
	system.simulateAuthExpiration(ctx, "user123")
	time.Sleep(500 * time.Millisecond)

	// Stop the system
	if err := system.Stop(ctx); err != nil {
		log.Printf("Error stopping system: %v", err)
	}

	fmt.Println("\nExample completed!")
}
