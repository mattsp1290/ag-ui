package factory

import (
	"context"
	"fmt"
	"sync"

	"github.com/ag-ui/go-sdk/pkg/transport"
)

// TransportFactory creates transport instances based on configuration
type TransportFactory interface {
	// Create creates a new transport instance with the given configuration
	Create(ctx context.Context, config interface{}) (transport.Transport, error)

	// Name returns the name of the transport type this factory creates
	Name() string

	// ValidateConfig validates the configuration for this transport type
	ValidateConfig(config interface{}) error
}

// Factory is the main transport factory that manages all registered transport factories
type Factory struct {
	mu        sync.RWMutex
	factories map[string]TransportFactory
	defaults  map[string]interface{}
}

// New creates a new transport factory
func New() *Factory {
	return &Factory{
		factories: make(map[string]TransportFactory),
		defaults:  make(map[string]interface{}),
	}
}

// Register registers a new transport factory
func (f *Factory) Register(factory TransportFactory) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	name := factory.Name()
	if name == "" {
		return fmt.Errorf("transport factory name cannot be empty")
	}

	if _, exists := f.factories[name]; exists {
		return fmt.Errorf("transport factory %q already registered", name)
	}

	f.factories[name] = factory
	return nil
}

// Unregister removes a transport factory
func (f *Factory) Unregister(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.factories[name]; !exists {
		return fmt.Errorf("transport factory %q not found", name)
	}

	delete(f.factories, name)
	delete(f.defaults, name)
	return nil
}

// Create creates a new transport instance
func (f *Factory) Create(ctx context.Context, transportType string, config interface{}) (transport.Transport, error) {
	f.mu.RLock()
	factory, exists := f.factories[transportType]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("transport type %q not registered", transportType)
	}

	// Use default config if none provided
	if config == nil {
		f.mu.RLock()
		config = f.defaults[transportType]
		f.mu.RUnlock()
	}

	// Validate configuration
	if err := factory.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration for transport %q: %w", transportType, err)
	}

	// Create transport instance
	transport, err := factory.Create(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport %q: %w", transportType, err)
	}

	return transport, nil
}

// SetDefault sets the default configuration for a transport type
func (f *Factory) SetDefault(transportType string, config interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	factory, exists := f.factories[transportType]
	if !exists {
		return fmt.Errorf("transport type %q not registered", transportType)
	}

	// Validate the default configuration
	if err := factory.ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid default configuration for transport %q: %w", transportType, err)
	}

	f.defaults[transportType] = config
	return nil
}

// GetRegistered returns a list of registered transport types
func (f *Factory) GetRegistered() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	types := make([]string, 0, len(f.factories))
	for name := range f.factories {
		types = append(types, name)
	}
	return types
}

// HasTransport checks if a transport type is registered
func (f *Factory) HasTransport(transportType string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	_, exists := f.factories[transportType]
	return exists
}

// BaseTransportFactory provides common functionality for transport factories
type BaseTransportFactory struct {
	name            string
	createFunc      func(context.Context, interface{}) (transport.Transport, error)
	validateFunc    func(interface{}) error
	defaultConfig   interface{}
}

// NewBaseFactory creates a new base transport factory
func NewBaseFactory(
	name string,
	createFunc func(context.Context, interface{}) (transport.Transport, error),
	validateFunc func(interface{}) error,
) *BaseTransportFactory {
	return &BaseTransportFactory{
		name:         name,
		createFunc:   createFunc,
		validateFunc: validateFunc,
	}
}

// Name returns the transport type name
func (f *BaseTransportFactory) Name() string {
	return f.name
}

// Create creates a new transport instance
func (f *BaseTransportFactory) Create(ctx context.Context, config interface{}) (transport.Transport, error) {
	if f.createFunc == nil {
		return nil, fmt.Errorf("create function not set for transport %q", f.name)
	}
	return f.createFunc(ctx, config)
}

// ValidateConfig validates the transport configuration
func (f *BaseTransportFactory) ValidateConfig(config interface{}) error {
	if f.validateFunc == nil {
		return nil // No validation if function not set
	}
	return f.validateFunc(config)
}

// SetDefaultConfig sets the default configuration
func (f *BaseTransportFactory) SetDefaultConfig(config interface{}) {
	f.defaultConfig = config
}