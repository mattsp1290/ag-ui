package di

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// Container is a dependency injection container that manages service lifecycle
// and resolves circular dependencies
type Container struct {
	services     map[string]*ServiceDefinition
	instances    map[string]interface{}
	singletons   map[string]interface{}
	building     map[string]bool // Track services currently being built to detect cycles
	mu           sync.RWMutex
	interceptors []Interceptor
}

// ServiceDefinition defines how a service should be created
type ServiceDefinition struct {
	Name         string
	Factory      Factory
	Lifecycle    Lifecycle
	Dependencies []string
	Interfaces   []reflect.Type
	Tags         []string
	Lazy         bool
}

// Factory is a function that creates a service instance
type Factory func(ctx context.Context, container *Container) (interface{}, error)

// Lifecycle defines the lifecycle of a service
type Lifecycle int

const (
	// LifecycleSingleton creates a single instance that is shared
	LifecycleSingleton Lifecycle = iota
	// LifecycleTransient creates a new instance every time
	LifecycleTransient
	// LifecycleScoped creates one instance per scope (e.g., per request)
	LifecycleScoped
)

// Interceptor allows interception of service creation
type Interceptor interface {
	// BeforeCreate is called before a service is created
	BeforeCreate(ctx context.Context, serviceName string) error
	// AfterCreate is called after a service is created
	AfterCreate(ctx context.Context, serviceName string, instance interface{}) error
}

// Scope represents a dependency injection scope
type Scope struct {
	container *Container
	instances map[string]interface{}
	mu        sync.RWMutex
}

// NewContainer creates a new dependency injection container
func NewContainer() *Container {
	return &Container{
		services:     make(map[string]*ServiceDefinition),
		instances:    make(map[string]interface{}),
		singletons:   make(map[string]interface{}),
		building:     make(map[string]bool),
		interceptors: make([]Interceptor, 0),
	}
}

// Register registers a service with the container
func (c *Container) Register(name string, factory Factory, lifecycle Lifecycle) *ServiceDefinition {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	definition := &ServiceDefinition{
		Name:         name,
		Factory:      factory,
		Lifecycle:    lifecycle,
		Dependencies: make([]string, 0),
		Interfaces:   make([]reflect.Type, 0),
		Tags:         make([]string, 0),
		Lazy:         false,
	}
	
	c.services[name] = definition
	return definition
}

// RegisterSingleton registers a singleton service
func (c *Container) RegisterSingleton(name string, factory Factory) *ServiceDefinition {
	return c.Register(name, factory, LifecycleSingleton)
}

// RegisterTransient registers a transient service
func (c *Container) RegisterTransient(name string, factory Factory) *ServiceDefinition {
	return c.Register(name, factory, LifecycleTransient)
}

// RegisterScoped registers a scoped service
func (c *Container) RegisterScoped(name string, factory Factory) *ServiceDefinition {
	return c.Register(name, factory, LifecycleScoped)
}

// RegisterInstance registers an existing instance as a singleton
func (c *Container) RegisterInstance(name string, instance interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.singletons[name] = instance
	c.services[name] = &ServiceDefinition{
		Name:      name,
		Lifecycle: LifecycleSingleton,
		Factory: func(ctx context.Context, container *Container) (interface{}, error) {
			return instance, nil
		},
	}
}

// WithDependencies sets the dependencies for a service
func (sd *ServiceDefinition) WithDependencies(deps ...string) *ServiceDefinition {
	sd.Dependencies = deps
	return sd
}

// WithInterfaces sets the interfaces implemented by the service
func (sd *ServiceDefinition) WithInterfaces(interfaces ...reflect.Type) *ServiceDefinition {
	sd.Interfaces = interfaces
	return sd
}

// WithTags sets the tags for the service
func (sd *ServiceDefinition) WithTags(tags ...string) *ServiceDefinition {
	sd.Tags = tags
	return sd
}

// WithLazy marks the service as lazy-loaded
func (sd *ServiceDefinition) WithLazy(lazy bool) *ServiceDefinition {
	sd.Lazy = lazy
	return sd
}

// Get retrieves a service from the container
func (c *Container) Get(ctx context.Context, name string) (interface{}, error) {
	return c.getService(ctx, name, nil)
}

// GetTyped retrieves a service and casts it to the specified type
func (c *Container) GetTyped(ctx context.Context, name string, target interface{}) error {
	service, err := c.Get(ctx, name)
	if err != nil {
		return err
	}
	
	// Use reflection to set the target
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}
	
	serviceValue := reflect.ValueOf(service)
	if !serviceValue.Type().AssignableTo(targetValue.Elem().Type()) {
		return fmt.Errorf("service %s of type %s is not assignable to %s", 
			name, serviceValue.Type(), targetValue.Elem().Type())
	}
	
	targetValue.Elem().Set(serviceValue)
	return nil
}

// GetByInterface retrieves services that implement the specified interface
func (c *Container) GetByInterface(ctx context.Context, interfaceType reflect.Type) ([]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	var services []interface{}
	for name, definition := range c.services {
		for _, implementedInterface := range definition.Interfaces {
			if implementedInterface == interfaceType {
				service, err := c.getService(ctx, name, nil)
				if err != nil {
					return nil, err
				}
				services = append(services, service)
				break
			}
		}
	}
	
	return services, nil
}

// GetByTag retrieves services with the specified tag
func (c *Container) GetByTag(ctx context.Context, tag string) ([]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	var services []interface{}
	for name, definition := range c.services {
		for _, serviceTag := range definition.Tags {
			if serviceTag == tag {
				service, err := c.getService(ctx, name, nil)
				if err != nil {
					return nil, err
				}
				services = append(services, service)
				break
			}
		}
	}
	
	return services, nil
}

// getService is the internal method to retrieve a service
func (c *Container) getService(ctx context.Context, name string, scope *Scope) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check if we're already building this service (circular dependency)
	if c.building[name] {
		return nil, fmt.Errorf("circular dependency detected for service: %s", name)
	}
	
	// Get service definition
	definition, exists := c.services[name]
	if !exists {
		return nil, fmt.Errorf("service not found: %s", name)
	}
	
	// Handle different lifecycles
	switch definition.Lifecycle {
	case LifecycleSingleton:
		return c.getSingleton(ctx, name, definition)
	case LifecycleTransient:
		return c.getTransient(ctx, name, definition)
	case LifecycleScoped:
		return c.getScoped(ctx, name, definition, scope)
	default:
		return nil, fmt.Errorf("unknown lifecycle: %d", definition.Lifecycle)
	}
}

// getSingleton gets or creates a singleton instance
func (c *Container) getSingleton(ctx context.Context, name string, definition *ServiceDefinition) (interface{}, error) {
	// Check if singleton already exists
	if instance, exists := c.singletons[name]; exists {
		return instance, nil
	}
	
	// Create new singleton
	instance, err := c.createInstance(ctx, name, definition, nil)
	if err != nil {
		return nil, err
	}
	
	c.singletons[name] = instance
	return instance, nil
}

// getTransient creates a new transient instance
func (c *Container) getTransient(ctx context.Context, name string, definition *ServiceDefinition) (interface{}, error) {
	return c.createInstance(ctx, name, definition, nil)
}

// getScoped gets or creates a scoped instance
func (c *Container) getScoped(ctx context.Context, name string, definition *ServiceDefinition, scope *Scope) (interface{}, error) {
	if scope == nil {
		return nil, fmt.Errorf("scope is required for scoped service: %s", name)
	}
	
	scope.mu.RLock()
	if instance, exists := scope.instances[name]; exists {
		scope.mu.RUnlock()
		return instance, nil
	}
	scope.mu.RUnlock()
	
	// Create new scoped instance
	instance, err := c.createInstance(ctx, name, definition, scope)
	if err != nil {
		return nil, err
	}
	
	scope.mu.Lock()
	scope.instances[name] = instance
	scope.mu.Unlock()
	
	return instance, nil
}

// createInstance creates a new instance of a service
func (c *Container) createInstance(ctx context.Context, name string, definition *ServiceDefinition, scope *Scope) (interface{}, error) {
	// Mark as building to detect circular dependencies
	c.building[name] = true
	defer delete(c.building, name)
	
	// Call interceptors
	for _, interceptor := range c.interceptors {
		if err := interceptor.BeforeCreate(ctx, name); err != nil {
			return nil, fmt.Errorf("interceptor failed before creating %s: %w", name, err)
		}
	}
	
	// Create dependencies first
	for _, depName := range definition.Dependencies {
		_, err := c.getService(ctx, depName, scope)
		if err != nil {
			return nil, fmt.Errorf("failed to create dependency %s for service %s: %w", depName, name, err)
		}
	}
	
	// Create the service instance
	instance, err := definition.Factory(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("failed to create service %s: %w", name, err)
	}
	
	// Call interceptors
	for _, interceptor := range c.interceptors {
		if err := interceptor.AfterCreate(ctx, name, instance); err != nil {
			return nil, fmt.Errorf("interceptor failed after creating %s: %w", name, err)
		}
	}
	
	return instance, nil
}

// AddInterceptor adds an interceptor to the container
func (c *Container) AddInterceptor(interceptor Interceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.interceptors = append(c.interceptors, interceptor)
}

// CreateScope creates a new dependency injection scope
func (c *Container) CreateScope() *Scope {
	return &Scope{
		container: c,
		instances: make(map[string]interface{}),
	}
}

// GetInScope retrieves a service within a specific scope
func (s *Scope) Get(ctx context.Context, name string) (interface{}, error) {
	return s.container.getService(ctx, name, s)
}

// Dispose disposes of all scoped instances
func (s *Scope) Dispose() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Call Dispose on any instances that implement it
	for _, instance := range s.instances {
		if disposable, ok := instance.(Disposable); ok {
			disposable.Dispose()
		}
	}
	
	s.instances = make(map[string]interface{})
}

// Disposable interface for services that need cleanup
type Disposable interface {
	Dispose()
}

// Clear clears all instances from the container
func (c *Container) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Call Dispose on singletons that implement it
	for _, instance := range c.singletons {
		if disposable, ok := instance.(Disposable); ok {
			disposable.Dispose()
		}
	}
	
	c.instances = make(map[string]interface{})
	c.singletons = make(map[string]interface{})
	c.building = make(map[string]bool)
}

// Validate validates the container configuration
func (c *Container) Validate() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Check for missing dependencies
	for name, definition := range c.services {
		for _, depName := range definition.Dependencies {
			if _, exists := c.services[depName]; !exists {
				return fmt.Errorf("service %s depends on non-existent service %s", name, depName)
			}
		}
	}
	
	// Check for circular dependencies (simplified check)
	return c.checkCircularDependencies()
}

// checkCircularDependencies performs a simplified circular dependency check
func (c *Container) checkCircularDependencies() error {
	visited := make(map[string]bool)
	stack := make(map[string]bool)
	
	var visit func(string) error
	visit = func(name string) error {
		if stack[name] {
			return fmt.Errorf("circular dependency detected involving service: %s", name)
		}
		if visited[name] {
			return nil
		}
		
		visited[name] = true
		stack[name] = true
		
		if definition, exists := c.services[name]; exists {
			for _, depName := range definition.Dependencies {
				if err := visit(depName); err != nil {
					return err
				}
			}
		}
		
		stack[name] = false
		return nil
	}
	
	for name := range c.services {
		if err := visit(name); err != nil {
			return err
		}
	}
	
	return nil
}

// GetServiceNames returns all registered service names
func (c *Container) GetServiceNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	names := make([]string, 0, len(c.services))
	for name := range c.services {
		names = append(names, name)
	}
	return names
}

// HasService checks if a service is registered
func (c *Container) HasService(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.services[name]
	return exists
}

// GetServiceDefinition returns the service definition for a service
func (c *Container) GetServiceDefinition(name string) (*ServiceDefinition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	definition, exists := c.services[name]
	return definition, exists
}

// Common interceptors

// LoggingInterceptor logs service creation
type LoggingInterceptor struct {
	Logger Logger
}

// Logger interface for logging
type Logger interface {
	Log(level string, message string, args ...interface{})
}

// BeforeCreate logs before service creation
func (li *LoggingInterceptor) BeforeCreate(ctx context.Context, serviceName string) error {
	if li.Logger != nil {
		li.Logger.Log("debug", "Creating service: %s", serviceName)
	}
	return nil
}

// AfterCreate logs after service creation
func (li *LoggingInterceptor) AfterCreate(ctx context.Context, serviceName string, instance interface{}) error {
	if li.Logger != nil {
		li.Logger.Log("debug", "Created service: %s, type: %T", serviceName, instance)
	}
	return nil
}

// TimingInterceptor measures service creation time
type TimingInterceptor struct {
	OnTiming func(serviceName string, duration int64) // duration in nanoseconds
}

// BeforeCreate starts timing
func (ti *TimingInterceptor) BeforeCreate(ctx context.Context, serviceName string) error {
	// Store start time in context (would need context value)
	return nil
}

// AfterCreate measures timing
func (ti *TimingInterceptor) AfterCreate(ctx context.Context, serviceName string, instance interface{}) error {
	// Calculate duration and call OnTiming
	// This is a simplified implementation
	if ti.OnTiming != nil {
		ti.OnTiming(serviceName, 0) // Would contain actual duration
	}
	return nil
}

// CachingInterceptor caches instances for performance
type CachingInterceptor struct {
	cache map[string]interface{}
	mu    sync.RWMutex
}

// NewCachingInterceptor creates a new caching interceptor
func NewCachingInterceptor() *CachingInterceptor {
	return &CachingInterceptor{
		cache: make(map[string]interface{}),
	}
}

// BeforeCreate checks cache
func (ci *CachingInterceptor) BeforeCreate(ctx context.Context, serviceName string) error {
	// Implementation would check cache and potentially skip creation
	return nil
}

// AfterCreate stores in cache
func (ci *CachingInterceptor) AfterCreate(ctx context.Context, serviceName string, instance interface{}) error {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	ci.cache[serviceName] = instance
	return nil
}