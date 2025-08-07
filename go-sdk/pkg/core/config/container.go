package config

import (
	"fmt"
	"reflect"
	"sync"
)

// ServiceRegistry implements ServiceContainer for dependency injection
type ServiceRegistry struct {
	services   map[string]interface{}
	factories  map[string]func() (interface{}, error)
	singletons map[string]interface{}
	mutex      sync.RWMutex
	started    bool
	startedMux sync.RWMutex
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services:   make(map[string]interface{}),
		factories:  make(map[string]func() (interface{}, error)),
		singletons: make(map[string]interface{}),
	}
}

// Register registers a service with the container
func (r *ServiceRegistry) Register(name string, service interface{}) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	if service == nil {
		return fmt.Errorf("service cannot be nil")
	}

	r.services[name] = service
	return nil
}

// RegisterFactory registers a factory function for a service
func (r *ServiceRegistry) RegisterFactory(name string, factory func() (interface{}, error)) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	if factory == nil {
		return fmt.Errorf("factory cannot be nil")
	}

	r.factories[name] = factory
	return nil
}

// RegisterSingleton registers a singleton service
func (r *ServiceRegistry) RegisterSingleton(name string, service interface{}) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	if service == nil {
		return fmt.Errorf("service cannot be nil")
	}

	r.singletons[name] = service
	return nil
}

// Get retrieves a service by name
func (r *ServiceRegistry) Get(name string) (interface{}, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Check singletons first
	if service, exists := r.singletons[name]; exists {
		return service, nil
	}

	// Check registered services
	if service, exists := r.services[name]; exists {
		return service, nil
	}

	// Check factories
	if factory, exists := r.factories[name]; exists {
		service, err := factory()
		if err != nil {
			return nil, fmt.Errorf("failed to create service %s: %w", name, err)
		}

		// Store as singleton if it's a singleton factory
		r.singletons[name] = service
		return service, nil
	}

	return nil, fmt.Errorf("service %s not found", name)
}

// GetTyped retrieves a service by name with type assertion
func (r *ServiceRegistry) GetTyped(name string, target interface{}) error {
	service, err := r.Get(name)
	if err != nil {
		return err
	}

	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	targetType := targetValue.Elem().Type()
	serviceValue := reflect.ValueOf(service)

	if !serviceValue.Type().AssignableTo(targetType) {
		return fmt.Errorf("service %s of type %s is not assignable to target type %s",
			name, serviceValue.Type(), targetType)
	}

	targetValue.Elem().Set(serviceValue)
	return nil
}

// Has checks if a service is registered
func (r *ServiceRegistry) Has(name string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	_, exists := r.singletons[name]
	if exists {
		return true
	}

	_, exists = r.services[name]
	if exists {
		return true
	}

	_, exists = r.factories[name]
	return exists
}

// Remove removes a service from the container
func (r *ServiceRegistry) Remove(name string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	delete(r.singletons, name)
	delete(r.services, name)
	delete(r.factories, name)

	return nil
}

// GetAll returns all registered services
func (r *ServiceRegistry) GetAll() map[string]interface{} {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make(map[string]interface{})

	// Add singletons
	for name, service := range r.singletons {
		result[name] = service
	}

	// Add regular services
	for name, service := range r.services {
		result[name] = service
	}

	// Add factories (as factory functions)
	for name, factory := range r.factories {
		result[name] = factory
	}

	return result
}

// Start starts all registered services that implement Startable
func (r *ServiceRegistry) Start() error {
	r.startedMux.Lock()
	defer r.startedMux.Unlock()

	if r.started {
		return fmt.Errorf("service registry already started")
	}

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var errors []error

	// Start singletons
	for name, service := range r.singletons {
		if startable, ok := service.(Startable); ok {
			if err := startable.Start(); err != nil {
				errors = append(errors, fmt.Errorf("failed to start singleton service %s: %w", name, err))
			}
		}
	}

	// Start regular services
	for name, service := range r.services {
		if startable, ok := service.(Startable); ok {
			if err := startable.Start(); err != nil {
				errors = append(errors, fmt.Errorf("failed to start service %s: %w", name, err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to start some services: %v", errors)
	}

	r.started = true
	return nil
}

// Stop stops all registered services that implement Stoppable
func (r *ServiceRegistry) Stop() error {
	r.startedMux.Lock()
	defer r.startedMux.Unlock()

	if !r.started {
		return nil
	}

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var errors []error

	// Stop singletons
	for name, service := range r.singletons {
		if stoppable, ok := service.(Stoppable); ok {
			if err := stoppable.Stop(); err != nil {
				errors = append(errors, fmt.Errorf("failed to stop singleton service %s: %w", name, err))
			}
		}
	}

	// Stop regular services
	for name, service := range r.services {
		if stoppable, ok := service.(Stoppable); ok {
			if err := stoppable.Stop(); err != nil {
				errors = append(errors, fmt.Errorf("failed to stop service %s: %w", name, err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to stop some services: %v", errors)
	}

	r.started = false
	return nil
}

// Validate validates all registered services
func (r *ServiceRegistry) Validate() error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var errors []error

	// Validate singletons
	for name, service := range r.singletons {
		if validatable, ok := service.(Validatable); ok {
			if err := validatable.Validate(); err != nil {
				errors = append(errors, fmt.Errorf("validation failed for singleton service %s: %w", name, err))
			}
		}
	}

	// Validate regular services
	for name, service := range r.services {
		if validatable, ok := service.(Validatable); ok {
			if err := validatable.Validate(); err != nil {
				errors = append(errors, fmt.Errorf("validation failed for service %s: %w", name, err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation failed for some services: %v", errors)
	}

	return nil
}

// IsStarted returns whether the service registry has been started
func (r *ServiceRegistry) IsStarted() bool {
	r.startedMux.RLock()
	defer r.startedMux.RUnlock()
	return r.started
}

// ConfigRegistry implements ConfigRegistry for managing configuration providers
type ConfigRegistryImpl struct {
	providers map[string]ConfigProvider
	global    ConfigProvider
	mutex     sync.RWMutex
}

// NewConfigRegistry creates a new configuration registry
func NewConfigRegistry() *ConfigRegistryImpl {
	return &ConfigRegistryImpl{
		providers: make(map[string]ConfigProvider),
	}
}

// Register registers a configuration provider for a module
func (r *ConfigRegistryImpl) Register(moduleName string, provider ConfigProvider) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if moduleName == "" {
		return fmt.Errorf("module name cannot be empty")
	}

	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	r.providers[moduleName] = provider
	return nil
}

// Unregister removes a configuration provider
func (r *ConfigRegistryImpl) Unregister(moduleName string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	delete(r.providers, moduleName)
	return nil
}

// Get retrieves a configuration provider for a module
func (r *ConfigRegistryImpl) Get(moduleName string) (ConfigProvider, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if provider, exists := r.providers[moduleName]; exists {
		return provider, nil
	}

	return nil, fmt.Errorf("configuration provider for module %s not found", moduleName)
}

// GetAll returns all registered configuration providers
func (r *ConfigRegistryImpl) GetAll() map[string]ConfigProvider {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make(map[string]ConfigProvider)
	for name, provider := range r.providers {
		result[name] = provider
	}

	return result
}

// GetGlobal returns the global configuration provider
func (r *ConfigRegistryImpl) GetGlobal() ConfigProvider {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return r.global
}

// SetGlobal sets the global configuration provider
func (r *ConfigRegistryImpl) SetGlobal(provider ConfigProvider) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if provider == nil {
		return fmt.Errorf("global provider cannot be nil")
	}

	r.global = provider
	return nil
}

// Validate validates all registered configurations
func (r *ConfigRegistryImpl) Validate() error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var errors []error

	// Validate global provider
	if r.global != nil {
		if err := r.global.Validate(); err != nil {
			errors = append(errors, fmt.Errorf("global configuration validation failed: %w", err))
		}
	}

	// Validate module providers
	for moduleName, provider := range r.providers {
		if err := provider.Validate(); err != nil {
			errors = append(errors, fmt.Errorf("configuration validation failed for module %s: %w", moduleName, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed: %v", errors)
	}

	return nil
}
