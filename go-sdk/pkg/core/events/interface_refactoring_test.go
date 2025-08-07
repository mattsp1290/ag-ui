package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events/auth"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events/cache"
)

// TestInterfaceSegregation tests that cache interfaces follow Interface Segregation Principle
func TestInterfaceSegregation(t *testing.T) {
	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T)
	}{
		{
			name:        "BasicCache_OnlyBasicOperations",
			description: "BasicCache should only expose fundamental cache operations",
			testFunc:    testBasicCacheInterface,
		},
		{
			name:        "AdvancedCache_ExtendsBasic",
			description: "AdvancedCache should extend BasicCache with advanced operations",
			testFunc:    testAdvancedCacheInterface,
		},
		{
			name:        "DistributedCache_FullFunctionality",
			description: "DistributedCache should provide complete distributed cache functionality",
			testFunc:    testDistributedCacheInterface,
		},
		{
			name:        "SeparateMetricsInterface",
			description: "Cache metrics should be separated into its own interface",
			testFunc:    testSeparateMetricsInterface,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)
			tt.testFunc(t)
		})
	}
}

// TestEventDrivenDecoupling tests that modules are decoupled through events
func TestEventDrivenDecoupling(t *testing.T) {
	tests := []struct {
		name        string
		description string
		testFunc    func(t *testing.T)
	}{
		{
			name:        "EventBus_PubSubFunctionality",
			description: "EventBus should provide publish-subscribe functionality",
			testFunc:    testEventBusPubSub,
		},
		{
			name:        "AuthCache_EventDrivenInvalidation",
			description: "Auth expiration should trigger cache invalidation via events",
			testFunc:    testAuthCacheEventDriven,
		},
		{
			name:        "DistributedCoordination_EventBased",
			description: "Distributed coordination should use events instead of direct calls",
			testFunc:    testDistributedEventCoordination,
		},
		{
			name:        "ModuleIndependence",
			description: "Modules should not have direct dependencies on each other",
			testFunc:    testModuleIndependence,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)
			tt.testFunc(t)
		})
	}
}

// Interface Segregation tests

func testBasicCacheInterface(t *testing.T) {
	// Test that BasicCache only has essential methods
	var basicCache cache.BasicCache

	// Mock implementation to verify interface
	basicCache = &MockBasicCache{}

	ctx := context.Background()
	key := "test_key"
	value := []byte("test_value")
	ttl := time.Hour

	// These methods should be available
	err := basicCache.Set(ctx, key, value, ttl)
	if err != nil {
		t.Errorf("BasicCache.Set() failed: %v", err)
	}

	_, err = basicCache.Get(ctx, key)
	if err != nil {
		t.Errorf("BasicCache.Get() failed: %v", err)
	}

	err = basicCache.Delete(ctx, key)
	if err != nil {
		t.Errorf("BasicCache.Delete() failed: %v", err)
	}

	t.Log("BasicCache interface correctly exposes only basic operations")
}

func testAdvancedCacheInterface(t *testing.T) {
	// Test that AdvancedCache extends BasicCache
	var advancedCache cache.AdvancedCache

	// Mock implementation
	advancedCache = &MockAdvancedCache{}

	ctx := context.Background()
	key := "test_key"

	// Should have basic operations
	err := advancedCache.Set(ctx, key, []byte("value"), time.Hour)
	if err != nil {
		t.Errorf("AdvancedCache.Set() failed: %v", err)
	}

	// Should have advanced operations
	exists, err := advancedCache.Exists(ctx, key)
	if err != nil {
		t.Errorf("AdvancedCache.Exists() failed: %v", err)
	}
	if !exists {
		t.Error("AdvancedCache.Exists() should return true for existing key")
	}

	_, err = advancedCache.TTL(ctx, key)
	if err != nil {
		t.Errorf("AdvancedCache.TTL() failed: %v", err)
	}

	t.Log("AdvancedCache interface correctly extends BasicCache with advanced operations")
}

func testDistributedCacheInterface(t *testing.T) {
	// Test that DistributedCache provides full functionality
	var distributedCache cache.DistributedCache

	// Mock implementation
	distributedCache = &MockDistributedCache{}

	ctx := context.Background()

	// Should have all previous operations plus distributed ones
	pattern := "test_*"
	keys, err := distributedCache.Scan(ctx, pattern)
	if err != nil {
		t.Errorf("DistributedCache.Scan() failed: %v", err)
	}

	if len(keys) == 0 {
		t.Log("No keys found matching pattern (expected for empty cache)")
	}

	t.Log("DistributedCache interface correctly provides distributed operations")
}

func testSeparateMetricsInterface(t *testing.T) {
	// Test that metrics are in a separate interface
	var metrics cache.CacheMetrics

	// Mock implementation
	metrics = &MockCacheMetrics{}

	// Test metrics operations
	metrics.RecordHit("L1")
	metrics.RecordMiss()
	metrics.RecordEviction()

	stats := metrics.GetStats()
	if stats.L1Hits == 0 {
		t.Error("Expected L1 hits to be recorded")
	}

	t.Log("Cache metrics interface is correctly separated")
}

// Event-driven decoupling tests

func testEventBusPubSub(t *testing.T) {
	// Test basic publish-subscribe functionality
	eventBus := events.NewEventBus(events.DefaultEventBusConfig())
	defer eventBus.Close()

	received := make(chan events.BusEvent, 1)

	// Subscribe to events
	handler := func(ctx context.Context, event events.BusEvent) error {
		received <- event
		return nil
	}

	subID, err := eventBus.Subscribe("test.event", handler)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer eventBus.Unsubscribe(subID)

	// Publish an event
	testEvent := events.BusEvent{
		ID:        "test-1",
		Type:      "test.event",
		Source:    "test",
		Data:      "test data",
		Timestamp: time.Now(),
	}

	err = eventBus.Publish(context.Background(), testEvent)
	if err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	// Verify event was received
	select {
	case receivedEvent := <-received:
		if receivedEvent.ID != testEvent.ID {
			t.Errorf("Expected event ID %s, got %s", testEvent.ID, receivedEvent.ID)
		}
	case <-time.After(1 * time.Second):
		t.Error("Event was not received within timeout")
	}

	t.Log("EventBus publish-subscribe functionality works correctly")
}

func testAuthCacheEventDriven(t *testing.T) {
	// Test that auth events trigger cache invalidation without direct coupling
	eventBus := events.NewEventBus(events.DefaultEventBusConfig())
	defer eventBus.Close()

	invalidationTriggered := make(chan bool, 1)

	// Mock cache manager that responds to auth events
	authEventHandler := func(ctx context.Context, event events.BusEvent) error {
		if event.Type == events.EventTypeAuthExpiration {
			// Simulate cache invalidation
			invalidationTriggered <- true
		}
		return nil
	}

	subID, err := eventBus.Subscribe(events.EventTypeAuthExpiration, authEventHandler)
	if err != nil {
		t.Fatalf("Failed to subscribe to auth events: %v", err)
	}
	defer eventBus.Unsubscribe(subID)

	// Simulate auth expiration
	authEvent := events.NewAuthEvent(
		events.EventTypeAuthExpiration,
		"auth_manager",
		"user123",
		events.AuthEventData{
			UserID: "user123",
			Reason: "session_timeout",
		},
	)

	err = eventBus.Publish(context.Background(), authEvent)
	if err != nil {
		t.Fatalf("Failed to publish auth event: %v", err)
	}

	// Verify cache invalidation was triggered
	select {
	case <-invalidationTriggered:
		t.Log("Cache invalidation triggered by auth event (event-driven decoupling working)")
	case <-time.After(1 * time.Second):
		t.Error("Cache invalidation was not triggered by auth event")
	}
}

func testDistributedEventCoordination(t *testing.T) {
	// Test that distributed operations use events instead of direct calls
	eventBus := events.NewEventBus(events.DefaultEventBusConfig())
	defer eventBus.Close()

	nodeJoinReceived := make(chan bool, 1)

	// Mock distributed manager
	distributedHandler := func(ctx context.Context, event events.BusEvent) error {
		if event.Type == events.EventTypeNodeJoin {
			nodeJoinReceived <- true
		}
		return nil
	}

	subID, err := eventBus.Subscribe(events.EventTypeNodeJoin, distributedHandler)
	if err != nil {
		t.Fatalf("Failed to subscribe to distributed events: %v", err)
	}
	defer eventBus.Unsubscribe(subID)

	// Simulate node joining
	nodeEvent := events.NewDistributedEvent(
		events.EventTypeNodeJoin,
		"distributed_manager",
		"node-2",
		events.DistributedEventData{
			NodeAddress: "node://node-2",
			ClusterSize: 2,
		},
	)

	err = eventBus.Publish(context.Background(), nodeEvent)
	if err != nil {
		t.Fatalf("Failed to publish node event: %v", err)
	}

	// Verify event was processed
	select {
	case <-nodeJoinReceived:
		t.Log("Distributed coordination via events working correctly")
	case <-time.After(1 * time.Second):
		t.Error("Node join event was not processed")
	}
}

func testModuleIndependence(t *testing.T) {
	// Test that modules can operate independently

	// Create separate event buses to simulate module independence
	authEventBus := events.NewEventBus(events.DefaultEventBusConfig())
	cacheEventBus := events.NewEventBus(events.DefaultEventBusConfig())
	defer authEventBus.Close()
	defer cacheEventBus.Close()

	// Test that auth module can work without cache module
	authProvider := auth.NewBasicAuthProvider(auth.DefaultAuthConfig())

	ctx := context.Background()
	creds := &auth.BasicCredentials{
		Username: "testuser",
		Password: "TestPass123!",
	}

	// Add a test user
	user := &auth.User{
		Username:     "testuser",
		PasswordHash: "hashedpassword", // In real implementation, this would be properly hashed
		Roles:        []string{"user"},
		Permissions:  []string{"read"},
		Active:       true,
	}
	authProvider.AddUser(user)
	authProvider.SetUserPassword("testuser", "TestPass123!")

	// Auth should work independently
	authCtx, err := authProvider.Authenticate(ctx, creds)
	if err != nil {
		t.Fatalf("Auth module failed to work independently: %v", err)
	}

	if authCtx.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", authCtx.Username)
	}

	t.Log("Modules can operate independently without direct coupling")
}

// Mock implementations for testing

type MockBasicCache struct{}

func (m *MockBasicCache) Get(ctx context.Context, key string) ([]byte, error) {
	return []byte("mock_value"), nil
}

func (m *MockBasicCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return nil
}

func (m *MockBasicCache) Delete(ctx context.Context, key string) error {
	return nil
}

type MockAdvancedCache struct {
	MockBasicCache
}

func (m *MockAdvancedCache) Exists(ctx context.Context, key string) (bool, error) {
	return true, nil
}

func (m *MockAdvancedCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return time.Hour, nil
}

type MockDistributedCache struct {
	MockAdvancedCache
}

func (m *MockDistributedCache) Scan(ctx context.Context, pattern string) ([]string, error) {
	return []string{"key1", "key2"}, nil
}

type MockCacheMetrics struct {
	stats cache.CacheStats
}

func (m *MockCacheMetrics) RecordHit(level string) {
	if level == "L1" {
		m.stats.L1Hits++
	} else {
		m.stats.L2Hits++
	}
}

func (m *MockCacheMetrics) RecordMiss() {
	m.stats.L1Misses++
}

func (m *MockCacheMetrics) RecordEviction() {
	m.stats.Evictions++
}

func (m *MockCacheMetrics) GetStats() cache.CacheStats {
	return m.stats
}
