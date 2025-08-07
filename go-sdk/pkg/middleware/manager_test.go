package middleware

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
)

func TestMiddlewareManager_CreateChain(t *testing.T) {
	manager := NewMiddlewareManager()

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{ID: req.ID, StatusCode: 200}, nil
	}

	chain := manager.CreateChain("test-chain", handler)
	if chain == nil {
		t.Fatal("Expected chain to be created")
	}

	// Verify chain was stored
	retrievedChain := manager.GetChain("test-chain")
	if retrievedChain == nil {
		t.Fatal("Expected to retrieve created chain")
	}

	if retrievedChain != chain {
		t.Error("Expected retrieved chain to be the same instance")
	}
}

func TestMiddlewareManager_GetDefaultChain(t *testing.T) {
	manager := NewMiddlewareManager()

	defaultChain := manager.GetDefaultChain()
	if defaultChain == nil {
		t.Fatal("Expected default chain to exist")
	}

	// Test processing with default chain
	req := &Request{
		ID:     "test-123",
		Method: "GET",
		Path:   "/test",
	}

	resp, err := defaultChain.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}
}

func TestMiddlewareManager_SetDefaultChain(t *testing.T) {
	manager := NewMiddlewareManager()

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{ID: req.ID, StatusCode: 201}, nil
	}

	// Create new chain
	manager.CreateChain("custom-chain", handler)

	// Set as default
	err := manager.SetDefaultChain("custom-chain")
	if err != nil {
		t.Fatalf("Unexpected error setting default chain: %v", err)
	}

	// Test that default chain is now the custom chain
	defaultChain := manager.GetDefaultChain()
	req := &Request{ID: "test", Method: "GET", Path: "/test"}

	resp, err := defaultChain.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if resp.StatusCode != 201 {
		t.Errorf("Expected status code 201 from custom chain, got %d", resp.StatusCode)
	}
}

func TestMiddlewareManager_ListChains(t *testing.T) {
	manager := NewMiddlewareManager()

	// Initial state should have default chain
	chains := manager.ListChains()
	if len(chains) != 1 || chains[0] != "default" {
		t.Errorf("Expected ['default'], got %v", chains)
	}

	// Add more chains
	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{ID: req.ID, StatusCode: 200}, nil
	}

	manager.CreateChain("chain1", handler)
	manager.CreateChain("chain2", handler)

	chains = manager.ListChains()
	if len(chains) != 3 {
		t.Errorf("Expected 3 chains, got %d", len(chains))
	}

	// Verify all chains exist
	chainMap := make(map[string]bool)
	for _, chain := range chains {
		chainMap[chain] = true
	}

	expectedChains := []string{"default", "chain1", "chain2"}
	for _, expected := range expectedChains {
		if !chainMap[expected] {
			t.Errorf("Expected chain %s not found", expected)
		}
	}
}

func TestMiddlewareManager_RemoveChain(t *testing.T) {
	manager := NewMiddlewareManager()

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{ID: req.ID, StatusCode: 200}, nil
	}

	// Create chain to remove
	manager.CreateChain("removable", handler)

	// Verify chain exists
	chains := manager.ListChains()
	if len(chains) != 2 {
		t.Errorf("Expected 2 chains before removal, got %d", len(chains))
	}

	// Remove chain
	removed := manager.RemoveChain("removable")
	if !removed {
		t.Error("Expected chain to be removed")
	}

	// Verify chain removed
	chains = manager.ListChains()
	if len(chains) != 1 {
		t.Errorf("Expected 1 chain after removal, got %d", len(chains))
	}

	// Try to remove default chain (should fail)
	removed = manager.RemoveChain("default")
	if removed {
		t.Error("Expected default chain removal to fail")
	}

	// Try to remove non-existent chain
	removed = manager.RemoveChain("nonexistent")
	if removed {
		t.Error("Expected removal of non-existent chain to fail")
	}
}

func TestMiddlewareManager_LoadConfiguration(t *testing.T) {
	manager := NewMiddlewareManager()

	// Create temporary config file
	config := MiddlewareConfiguration{
		DefaultChain: "api",
		Chains: []ChainConfiguration{
			{
				Name:    "api",
				Enabled: true,
				Handler: HandlerConfiguration{
					Type: "echo",
				},
				Middleware: []MiddlewareConfig{
					{
						Name:     "logging",
						Type:     "logging",
						Enabled:  true,
						Priority: 100,
						Config: map[string]interface{}{
							"level": "info",
						},
					},
					{
						Name:     "metrics",
						Type:     "metrics",
						Enabled:  true,
						Priority: 90,
					},
				},
			},
		},
	}

	// Write config to temporary file
	tmpFile, err := ioutil.TempFile("", "middleware-config-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	configData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if _, err := tmpFile.Write(configData); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	tmpFile.Close()

	// Load configuration
	err = manager.LoadConfiguration(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Verify configuration loaded
	chains := manager.ListChains()
	if len(chains) != 1 || chains[0] != "api" {
		t.Errorf("Expected ['api'], got %v", chains)
	}

	// Verify default chain is set correctly
	defaultChain := manager.GetDefaultChain()
	if defaultChain == nil {
		t.Fatal("Expected default chain to exist")
	}

	// Test processing with loaded chain
	req := &Request{
		ID:     "test-123",
		Method: "GET",
		Path:   "/test",
		Body:   map[string]interface{}{"test": "data"},
	}

	resp, err := defaultChain.Process(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Echo handler should return request body
	if resp.Body == nil {
		t.Error("Expected response body to contain echoed request")
	}
}

func TestMiddlewareManager_GetMetrics(t *testing.T) {
	manager := NewMiddlewareManager()

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{ID: req.ID, StatusCode: 200}, nil
	}

	// Create chain with middleware
	chain := manager.CreateChain("test-chain", handler)
	chain.Add(NewTestMiddleware("test-middleware", 10))

	// Get metrics
	metrics := manager.GetMetrics()
	if metrics == nil {
		t.Fatal("Expected metrics to be returned")
	}

	// Verify default chain metrics
	if defaultMetrics, ok := metrics["default"]; ok {
		if defaultMap, ok := defaultMetrics.(map[string]interface{}); ok {
			if count, ok := defaultMap["middleware_count"].(int); !ok || count != 0 {
				t.Errorf("Expected default chain to have 0 middleware, got %v", count)
			}
		}
	}

	// Verify test chain metrics
	if testMetrics, ok := metrics["test-chain"]; ok {
		if testMap, ok := testMetrics.(map[string]interface{}); ok {
			if count, ok := testMap["middleware_count"].(int); !ok || count != 1 {
				t.Errorf("Expected test chain to have 1 middleware, got %v", count)
			}
		}
	}
}

func TestDefaultMiddlewareRegistry_Register(t *testing.T) {
	registry := NewDefaultMiddlewareRegistry()

	// Test registering custom factory
	factory := &TestMiddlewareFactory{}
	err := registry.Register("test", factory)
	if err != nil {
		t.Fatalf("Unexpected error registering factory: %v", err)
	}

	// Test creating middleware from registered factory
	config := &MiddlewareConfig{
		Name:    "test-middleware",
		Type:    "test",
		Enabled: true,
		Config:  map[string]interface{}{},
	}

	middleware, err := registry.Create(config)
	if err != nil {
		t.Fatalf("Unexpected error creating middleware: %v", err)
	}

	if middleware == nil {
		t.Fatal("Expected middleware to be created")
	}

	if middleware.Name() != "test-middleware" {
		t.Errorf("Expected middleware name 'test-middleware', got %s", middleware.Name())
	}
}

func TestDefaultMiddlewareRegistry_ListTypes(t *testing.T) {
	registry := NewDefaultMiddlewareRegistry()

	types := registry.ListTypes()
	if len(types) == 0 {
		t.Error("Expected default middleware types to be registered")
	}

	// Verify some expected types are present
	expectedTypes := []string{"jwt_auth", "logging", "metrics", "security"}
	typeMap := make(map[string]bool)
	for _, t := range types {
		typeMap[t] = true
	}

	for _, expected := range expectedTypes {
		if !typeMap[expected] {
			t.Errorf("Expected middleware type %s not found", expected)
		}
	}
}

func TestDefaultMiddlewareRegistry_Unregister(t *testing.T) {
	registry := NewDefaultMiddlewareRegistry()

	// Register custom factory
	factory := &TestMiddlewareFactory{}
	registry.Register("test", factory)

	// Verify it exists
	types := registry.ListTypes()
	found := false
	for _, t := range types {
		if t == "test" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Expected test middleware type to be registered")
	}

	// Unregister
	err := registry.Unregister("test")
	if err != nil {
		t.Fatalf("Unexpected error unregistering: %v", err)
	}

	// Verify it's gone
	types = registry.ListTypes()
	for _, middlewareType := range types {
		if middlewareType == "test" {
			t.Error("Expected test middleware type to be unregistered")
		}
	}

	// Try to unregister non-existent type
	err = registry.Unregister("nonexistent")
	if err == nil {
		t.Error("Expected error when unregistering non-existent type")
	}
}

// TestMiddlewareFactory for testing
type TestMiddlewareFactory struct{}

func (f *TestMiddlewareFactory) Create(config *MiddlewareConfig) (Middleware, error) {
	middleware := NewTestMiddleware(config.Name, config.Priority)
	middleware.Configure(config.Config)
	return middleware, nil
}

func (f *TestMiddlewareFactory) SupportedTypes() []string {
	return []string{"test"}
}

// Benchmark middleware manager operations
func BenchmarkMiddlewareManager_GetChain(b *testing.B) {
	manager := NewMiddlewareManager()

	handler := func(ctx context.Context, req *Request) (*Response, error) {
		return &Response{ID: req.ID, StatusCode: 200}, nil
	}

	manager.CreateChain("bench-chain", handler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chain := manager.GetChain("bench-chain")
		if chain == nil {
			b.Fatal("Expected chain to exist")
		}
	}
}

func BenchmarkDefaultMiddlewareRegistry_Create(b *testing.B) {
	registry := NewDefaultMiddlewareRegistry()

	config := &MiddlewareConfig{
		Name:    "bench-middleware",
		Type:    "logging",
		Enabled: true,
		Config: map[string]interface{}{
			"level": "info",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		middleware, err := registry.Create(config)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
		if middleware == nil {
			b.Fatal("Expected middleware to be created")
		}
	}
}