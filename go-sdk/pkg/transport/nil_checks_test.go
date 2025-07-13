package transport

import (
	"context"
	"testing"
	"time"
)

// TestNilChecksTransportRegistry tests nil checks in transport registry methods
func TestNilChecksTransportRegistry(t *testing.T) {
	t.Run("DefaultTransportRegistry Register with nil registry", func(t *testing.T) {
		var registry *DefaultTransportRegistry
		err := registry.Register("test", nil)
		if err == nil {
			t.Error("Expected error when registry is nil")
		}
		if err.Error() != "transport registry is nil" {
			t.Errorf("Expected 'transport registry is nil', got '%s'", err.Error())
		}
	})

	t.Run("DefaultTransportRegistry Register with nil factory", func(t *testing.T) {
		registry := NewDefaultTransportRegistry()
		err := registry.Register("test", nil)
		if err == nil {
			t.Error("Expected error when factory is nil")
		}
		if err.Error() != "factory cannot be nil" {
			t.Errorf("Expected 'factory cannot be nil', got '%s'", err.Error())
		}
	})

	t.Run("DefaultTransportRegistry Register with empty transport type", func(t *testing.T) {
		registry := NewDefaultTransportRegistry()
		mockFactory := &MockTransportFactory{}
		err := registry.Register("", mockFactory)
		if err == nil {
			t.Error("Expected error when transport type is empty")
		}
		if err.Error() != "transport type cannot be empty" {
			t.Errorf("Expected 'transport type cannot be empty', got '%s'", err.Error())
		}
	})

	t.Run("DefaultTransportRegistry CreateWithContext with nil registry", func(t *testing.T) {
		var registry *DefaultTransportRegistry
		_, err := registry.CreateWithContext(context.Background(), &MockConfig{})
		if err == nil {
			t.Error("Expected error when registry is nil")
		}
		if err.Error() != "transport registry is nil" {
			t.Errorf("Expected 'transport registry is nil', got '%s'", err.Error())
		}
	})

	t.Run("DefaultTransportRegistry CreateWithContext with nil context", func(t *testing.T) {
		registry := NewDefaultTransportRegistry()
		_, err := registry.CreateWithContext(nil, &MockConfig{})
		if err == nil {
			t.Error("Expected error when context is nil")
		}
		if err.Error() != "context cannot be nil" {
			t.Errorf("Expected 'context cannot be nil', got '%s'", err.Error())
		}
	})

	t.Run("DefaultTransportRegistry CreateWithContext with nil config", func(t *testing.T) {
		registry := NewDefaultTransportRegistry()
		_, err := registry.CreateWithContext(context.Background(), nil)
		if err == nil {
			t.Error("Expected error when config is nil")
		}
		if err.Error() != "config cannot be nil" {
			t.Errorf("Expected 'config cannot be nil', got '%s'", err.Error())
		}
	})

	t.Run("DefaultTransportRegistry GetFactory with nil registry", func(t *testing.T) {
		var registry *DefaultTransportRegistry
		_, err := registry.GetFactory("test")
		if err == nil {
			t.Error("Expected error when registry is nil")
		}
		if err.Error() != "transport registry is nil" {
			t.Errorf("Expected 'transport registry is nil', got '%s'", err.Error())
		}
	})

	t.Run("DefaultTransportRegistry GetRegisteredTypes with nil registry", func(t *testing.T) {
		var registry *DefaultTransportRegistry
		types := registry.GetRegisteredTypes()
		if types == nil {
			t.Error("Expected empty slice, got nil")
		}
		if len(types) != 0 {
			t.Error("Expected empty slice for nil registry")
		}
	})

	t.Run("DefaultTransportRegistry IsRegistered with nil registry", func(t *testing.T) {
		var registry *DefaultTransportRegistry
		registered := registry.IsRegistered("test")
		if registered {
			t.Error("Expected false for nil registry")
		}
	})
}

// TestNilChecksTransportManager tests nil checks in transport manager methods
func TestNilChecksTransportManager(t *testing.T) {
	t.Run("NewDefaultTransportManager with nil registry", func(t *testing.T) {
		manager := NewDefaultTransportManager(nil)
		if manager != nil {
			t.Error("Expected nil when registry is nil")
		}
	})

	t.Run("DefaultTransportManager AddTransport with nil manager", func(t *testing.T) {
		var manager *DefaultTransportManager
		err := manager.AddTransport("test", &MockTransport{})
		if err == nil {
			t.Error("Expected error when manager is nil")
		}
		if err.Error() != "transport manager is nil" {
			t.Errorf("Expected 'transport manager is nil', got '%s'", err.Error())
		}
	})

	t.Run("DefaultTransportManager AddTransport with nil transport", func(t *testing.T) {
		registry := NewDefaultTransportRegistry()
		manager := NewDefaultTransportManager(registry)
		err := manager.AddTransport("test", nil)
		if err == nil {
			t.Error("Expected error when transport is nil")
		}
		if err.Error() != "transport cannot be nil" {
			t.Errorf("Expected 'transport cannot be nil', got '%s'", err.Error())
		}
	})

	t.Run("DefaultTransportManager GetTransport with nil manager", func(t *testing.T) {
		var manager *DefaultTransportManager
		_, err := manager.GetTransport("test")
		if err == nil {
			t.Error("Expected error when manager is nil")
		}
		if err.Error() != "transport manager is nil" {
			t.Errorf("Expected 'transport manager is nil', got '%s'", err.Error())
		}
	})

	t.Run("DefaultTransportManager GetActiveTransports with nil manager", func(t *testing.T) {
		var manager *DefaultTransportManager
		transports := manager.GetActiveTransports()
		if transports == nil {
			t.Error("Expected empty map, got nil")
		}
		if len(transports) != 0 {
			t.Error("Expected empty map for nil manager")
		}
	})

	t.Run("DefaultTransportManager SendEvent with nil manager", func(t *testing.T) {
		var manager *DefaultTransportManager
		err := manager.SendEvent(context.Background(), "test event")
		if err == nil {
			t.Error("Expected error when manager is nil")
		}
		if err.Error() != "transport manager is nil" {
			t.Errorf("Expected 'transport manager is nil', got '%s'", err.Error())
		}
	})

	t.Run("DefaultTransportManager SendEvent with nil context", func(t *testing.T) {
		registry := NewDefaultTransportRegistry()
		manager := NewDefaultTransportManager(registry)
		err := manager.SendEvent(nil, "test event")
		if err == nil {
			t.Error("Expected error when context is nil")
		}
		if err.Error() != "context cannot be nil" {
			t.Errorf("Expected 'context cannot be nil', got '%s'", err.Error())
		}
	})

	t.Run("DefaultTransportManager SendEvent with nil event", func(t *testing.T) {
		registry := NewDefaultTransportRegistry()
		manager := NewDefaultTransportManager(registry)
		err := manager.SendEvent(context.Background(), nil)
		if err == nil {
			t.Error("Expected error when event is nil")
		}
		if err.Error() != "event cannot be nil" {
			t.Errorf("Expected 'event cannot be nil', got '%s'", err.Error())
		}
	})
}

// Mock implementations for testing
type MockTransportFactory struct{}

func (f *MockTransportFactory) Create(config Config) (Transport, error) {
	return &MockTransport{}, nil
}

func (f *MockTransportFactory) CreateWithContext(ctx context.Context, config Config) (Transport, error) {
	return &MockTransport{}, nil
}

func (f *MockTransportFactory) SupportedTypes() []string {
	return []string{"mock"}
}

func (f *MockTransportFactory) Name() string {
	return "mock"
}

func (f *MockTransportFactory) Version() string {
	return "1.0.0"
}

type MockTransport struct{}

func (t *MockTransport) Connect(ctx context.Context) error {
	return nil
}

func (t *MockTransport) Disconnect(ctx context.Context) error {
	return nil
}

func (t *MockTransport) IsConnected() bool {
	return true
}

func (t *MockTransport) SendEvent(ctx context.Context, event any) error {
	return nil
}

func (t *MockTransport) ReceiveEvents(ctx context.Context) (<-chan any, error) {
	ch := make(chan any)
	close(ch)
	return ch, nil
}

func (t *MockTransport) Close() error {
	return nil
}

func (t *MockTransport) Stats() TransportStats {
	return TransportStats{}
}

func (t *MockTransport) Config() Config {
	return &MockConfig{}
}

type MockConfig struct{}

func (c *MockConfig) GetType() string {
	return "mock"
}

func (c *MockConfig) Validate() error {
	return nil
}

func (c *MockConfig) Clone() Config {
	return &MockConfig{}
}

func (c *MockConfig) GetEndpoint() string {
	return "mock://localhost"
}

func (c *MockConfig) GetTimeout() time.Duration {
	return 30 * time.Second
}

func (c *MockConfig) GetHeaders() map[string]string {
	return map[string]string{}
}

func (c *MockConfig) IsSecure() bool {
	return false
}