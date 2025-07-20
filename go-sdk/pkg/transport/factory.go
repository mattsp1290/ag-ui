package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"
	
	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// TransportFactory creates transport instances based on configuration.
type TransportFactory interface {
	// Create creates a new transport instance with the given configuration.
	Create(config Config) (Transport, error)

	// CreateWithContext creates a new transport instance with context.
	CreateWithContext(ctx context.Context, config Config) (Transport, error)

	// SupportedTypes returns the transport types supported by this factory.
	SupportedTypes() []string

	// Name returns the factory name.
	Name() string

	// Version returns the factory version.
	Version() string
}

// TransportRegistry manages transport factories and provides transport creation services.
type TransportRegistry interface {
	// Register registers a transport factory for a specific type.
	Register(transportType string, factory TransportFactory) error

	// Unregister removes a transport factory for a specific type.
	Unregister(transportType string) error

	// Create creates a transport instance using the appropriate factory.
	Create(config Config) (Transport, error)

	// CreateWithContext creates a transport instance with context.
	CreateWithContext(ctx context.Context, config Config) (Transport, error)

	// GetFactory returns the factory for a specific transport type.
	GetFactory(transportType string) (TransportFactory, error)

	// GetRegisteredTypes returns all registered transport types.
	GetRegisteredTypes() []string

	// IsRegistered checks if a transport type is registered.
	IsRegistered(transportType string) bool

	// Clear removes all registered factories.
	Clear()
}

// DefaultTransportRegistry is the default implementation of TransportRegistry.
type DefaultTransportRegistry struct {
	mu        sync.RWMutex
	factories map[string]TransportFactory
}

// NewDefaultTransportRegistry creates a new default transport registry.
func NewDefaultTransportRegistry() *DefaultTransportRegistry {
	return &DefaultTransportRegistry{
		factories: make(map[string]TransportFactory),
	}
}

// Register registers a transport factory for a specific type.
func (r *DefaultTransportRegistry) Register(transportType string, factory TransportFactory) error {
	if r == nil {
		return fmt.Errorf("transport registry is nil")
	}
	if transportType == "" {
		return fmt.Errorf("transport type cannot be empty")
	}

	if factory == nil {
		return fmt.Errorf("factory cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.factories == nil {
		r.factories = make(map[string]TransportFactory)
	}

	if _, exists := r.factories[transportType]; exists {
		return fmt.Errorf("transport type %s is already registered", transportType)
	}

	r.factories[transportType] = factory
	return nil
}

// Unregister removes a transport factory for a specific type.
func (r *DefaultTransportRegistry) Unregister(transportType string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[transportType]; !exists {
		return fmt.Errorf("transport type %s is not registered", transportType)
	}

	delete(r.factories, transportType)
	return nil
}

// Create creates a transport instance using the appropriate factory.
func (r *DefaultTransportRegistry) Create(config Config) (Transport, error) {
	return r.CreateWithContext(context.Background(), config)
}

// CreateWithContext creates a transport instance with context.
func (r *DefaultTransportRegistry) CreateWithContext(ctx context.Context, config Config) (Transport, error) {
	if r == nil {
		return nil, fmt.Errorf("transport registry is nil")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	transportType := config.GetType()
	if transportType == "" {
		return nil, fmt.Errorf("transport type cannot be empty")
	}

	factory, err := r.GetFactory(transportType)
	if err != nil {
		return nil, err
	}

	if factory == nil {
		return nil, fmt.Errorf("factory is nil for transport type: %s", transportType)
	}

	return factory.CreateWithContext(ctx, config)
}

// GetFactory returns the factory for a specific transport type.
func (r *DefaultTransportRegistry) GetFactory(transportType string) (TransportFactory, error) {
	if r == nil {
		return nil, fmt.Errorf("transport registry is nil")
	}
	if transportType == "" {
		return nil, fmt.Errorf("transport type cannot be empty")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.factories == nil {
		return nil, fmt.Errorf("factories map is nil")
	}

	factory, exists := r.factories[transportType]
	if !exists {
		return nil, fmt.Errorf("no factory registered for transport type: %s", transportType)
	}

	return factory, nil
}

// GetRegisteredTypes returns all registered transport types.
func (r *DefaultTransportRegistry) GetRegisteredTypes() []string {
	if r == nil {
		return []string{}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.factories == nil {
		return []string{}
	}

	types := make([]string, 0, len(r.factories))
	for transportType := range r.factories {
		types = append(types, transportType)
	}

	return types
}

// IsRegistered checks if a transport type is registered.
func (r *DefaultTransportRegistry) IsRegistered(transportType string) bool {
	if r == nil || transportType == "" {
		return false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.factories == nil {
		return false
	}

	_, exists := r.factories[transportType]
	return exists
}

// Clear removes all registered factories.
func (r *DefaultTransportRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factories = make(map[string]TransportFactory)
}

// TransportManagerConfig holds configuration for the transport manager cleanup mechanism
type TransportManagerConfig struct {
	// CleanupEnabled enables the periodic map cleanup mechanism
	CleanupEnabled bool
	// CleanupInterval specifies how often to run cleanup (default: 1 hour)
	CleanupInterval time.Duration
	// MaxMapSize is the threshold above which cleanup is triggered (default: 1000)
	MaxMapSize int
	// ActiveThreshold is the ratio below which cleanup occurs (default: 0.5)
	// If activeTransports/totalTransports < ActiveThreshold, cleanup runs
	ActiveThreshold float64
	// CleanupMetricsEnabled enables detailed cleanup metrics
	CleanupMetricsEnabled bool
}

// DefaultTransportManagerConfig returns default configuration for transport manager
func DefaultTransportManagerConfig() *TransportManagerConfig {
	return &TransportManagerConfig{
		CleanupEnabled:        true,
		CleanupInterval:       1 * time.Hour,
		MaxMapSize:           1000,
		ActiveThreshold:      0.5,
		CleanupMetricsEnabled: true,
	}
}

// TransportManager manages multiple transport instances and provides advanced features.
type DefaultTransportManager struct {
	mu                sync.RWMutex
	transports        map[string]Transport
	registry          TransportRegistry
	balancer          LoadBalancer
	middleware        MiddlewareChain
	eventBus          EventBus
	healthCheck       *HealthCheckManager
	metrics           *MetricsManager
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	logger            Logger
	config            *TransportManagerConfig
	cleanupTicker     *time.Ticker
	lastCleanupTime   time.Time
	mapCleanupMetrics *MapCleanupMetrics
}

// MapCleanupMetrics tracks transport map cleanup operation statistics
type MapCleanupMetrics struct {
	mu                  sync.RWMutex
	TotalCleanups       uint64
	TransportsRemoved   uint64
	TransportsRetained  uint64
	LastCleanupDuration time.Duration
	LastCleanupTime     time.Time
	CleanupErrors       uint64
}

// NewDefaultTransportManager creates a new default transport manager.
func NewDefaultTransportManager(registry TransportRegistry) *DefaultTransportManager {
	return NewDefaultTransportManagerWithConfig(registry, DefaultTransportManagerConfig())
}

// NewDefaultTransportManagerWithConfig creates a new default transport manager with custom configuration.
func NewDefaultTransportManagerWithConfig(registry TransportRegistry, config *TransportManagerConfig) *DefaultTransportManager {
	if registry == nil {
		return nil
	}
	
	if config == nil {
		config = DefaultTransportManagerConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	manager := &DefaultTransportManager{
		transports:        make(map[string]Transport),
		registry:          registry,
		middleware:        NewDefaultMiddlewareChain(),
		healthCheck:       NewHealthCheckManager(),
		metrics:           NewMetricsManager(),
		ctx:               ctx,
		cancel:            cancel,
		logger:            NewLogger(DefaultLoggerConfig()),
		config:            config,
		lastCleanupTime:   time.Now(),
		mapCleanupMetrics: &MapCleanupMetrics{},
	}
	
	// Start cleanup goroutine if enabled
	if config.CleanupEnabled {
		manager.startCleanupTicker()
	}
	
	return manager
}

// AddTransport adds a transport to the manager.
func (m *DefaultTransportManager) AddTransport(name string, transport Transport) error {
	if m == nil {
		return fmt.Errorf("transport manager is nil")
	}
	if name == "" {
		return fmt.Errorf("transport name cannot be empty")
	}

	if transport == nil {
		return fmt.Errorf("transport cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.transports == nil {
		m.transports = make(map[string]Transport)
	}

	if _, exists := m.transports[name]; exists {
		return fmt.Errorf("transport %s already exists", name)
	}

	m.transports[name] = transport
	
	// Check if cleanup is needed after adding transport
	m.checkAndTriggerCleanup()
	
	// Register with health checker
	if m.healthCheck != nil {
		m.healthCheck.AddTransport(name, transport)
	}

	// Register with metrics
	if m.metrics != nil {
		m.metrics.AddTransport(name, transport)
	}

	// Emit event
	if m.eventBus != nil {
		// Create a compatible event for the event bus
		// For now, we'll skip event publishing since it requires complex event creation
		// This can be implemented later with proper event construction
		// event := NewSimpleEvent("transport-added-"+name, string(EventTypeConnected), 
		//	map[string]interface{}{"transport": name})
		// m.eventBus.Publish(m.ctx, "transport.added", event)
	}

	return nil
}

// RemoveTransport removes a transport from the manager.
func (m *DefaultTransportManager) RemoveTransport(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	transport, exists := m.transports[name]
	if !exists {
		return fmt.Errorf("transport %s not found", name)
	}

	// Close the transport
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := transport.Close(ctx); err != nil {
		// Log error but don't fail the removal
		m.logger.Error("failed to close transport during removal",
			String("transport", name),
			Error(err))
	}

	delete(m.transports, name)

	// Remove from health checker
	if m.healthCheck != nil {
		m.healthCheck.RemoveTransport(name)
	}

	// Remove from metrics
	if m.metrics != nil {
		m.metrics.RemoveTransport(name)
	}

	// Emit event
	if m.eventBus != nil {
		// Skip event publishing for now - requires proper events.Event implementation
		// event := NewSimpleEvent("transport-removed-"+name, string(EventTypeDisconnected), 
		//	map[string]interface{}{"transport": name})
		// m.eventBus.Publish(m.ctx, "transport.removed", event)
	}

	return nil
}

// GetTransport retrieves a transport by name.
func (m *DefaultTransportManager) GetTransport(name string) (Transport, error) {
	if m == nil {
		return nil, fmt.Errorf("transport manager is nil")
	}
	if name == "" {
		return nil, fmt.Errorf("transport name cannot be empty")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.transports == nil {
		return nil, fmt.Errorf("transports map is nil")
	}

	transport, exists := m.transports[name]
	if !exists {
		return nil, fmt.Errorf("transport %s not found", name)
	}

	return transport, nil
}

// GetActiveTransports returns all active transports.
func (m *DefaultTransportManager) GetActiveTransports() map[string]Transport {
	if m == nil {
		return make(map[string]Transport)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.transports == nil {
		return make(map[string]Transport)
	}

	result := make(map[string]Transport, len(m.transports))
	for name, transport := range m.transports {
		if transport != nil && transport.IsConnected() {
			result[name] = transport
		}
	}

	return result
}

// SendEvent sends an event using the best available transport.
func (m *DefaultTransportManager) SendEvent(ctx context.Context, event TransportEvent) error {
	if m == nil {
		return fmt.Errorf("transport manager is nil")
	}
	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}
	activeTransports := m.GetActiveTransports()
	if len(activeTransports) == 0 {
		return fmt.Errorf("no active transports available")
	}

	// Use load balancer if available
	if m.balancer != nil {
		transportName, err := m.balancer.SelectTransport(activeTransports, event)
		if err != nil {
			return fmt.Errorf("failed to select transport: %w", err)
		}

		return m.SendEventToTransport(ctx, transportName, event)
	}

	// Use first available transport
	for name := range activeTransports {
		return m.SendEventToTransport(ctx, name, event)
	}

	return fmt.Errorf("no suitable transport found")
}

// SendEventToTransport sends an event to a specific transport.
func (m *DefaultTransportManager) SendEventToTransport(ctx context.Context, transportName string, event TransportEvent) error {
	transport, err := m.GetTransport(transportName)
	if err != nil {
		return err
	}

	if !transport.IsConnected() {
		return fmt.Errorf("transport %s is not connected", transportName)
	}

	// Process through middleware chain
	if m.middleware != nil {
		processedEvent, err := m.middleware.ProcessOutgoing(ctx, event)
		if err != nil {
			return fmt.Errorf("middleware processing failed: %w", err)
		}
		event = processedEvent
	}

	// Send the event
	if err := transport.Send(ctx, event); err != nil {
		// Update metrics
		if m.metrics != nil {
			m.metrics.RecordError(transportName, err)
		}

		// Emit error event
		if m.eventBus != nil {
			// Skip event publishing for now - requires proper events.Event implementation
			// errorEvent := NewSimpleEvent("transport-error-"+transportName, string(EventTypeError), 
			//	map[string]interface{}{"transport": transportName, "error": err.Error()})
			// m.eventBus.Publish(ctx, "transport.error", errorEvent)
		}

		return fmt.Errorf("failed to send event via transport %s: %w", transportName, err)
	}

	// Update metrics
	if m.metrics != nil {
		m.metrics.RecordEvent(transportName, event)
	}

	// Emit event
	if m.eventBus != nil {
		// Skip event publishing for now - requires proper events.Event implementation
		// sentEvent := NewSimpleEvent("transport-event-sent-"+transportName, string(EventTypeEventSent), 
		//	map[string]interface{}{"transport": transportName, "event": event})
		// m.eventBus.Publish(ctx, "transport.event_sent", sentEvent)
	}

	return nil
}

// ReceiveEvents returns a channel that receives events from all transports.
func (m *DefaultTransportManager) ReceiveEvents(ctx context.Context) (<-chan events.Event, error) {
	resultChan := make(chan events.Event, 100)
	
	m.mu.RLock()
	transportList := make([]Transport, 0, len(m.transports))
	transportNames := make([]string, 0, len(m.transports))
	
	for name, transport := range m.transports {
		transportList = append(transportList, transport)
		transportNames = append(transportNames, name)
	}
	m.mu.RUnlock()

	// Start goroutines to receive events from each transport
	for i, transport := range transportList {
		transportName := transportNames[i]
		
		m.wg.Add(1)
		go func(name string, t Transport) {
			defer m.wg.Done()
			
			eventChan, errorChan := t.Channels()

			for {
				select {
				case event, ok := <-eventChan:
					if !ok {
						return
					}

					// Process through middleware chain
					if m.middleware != nil {
						processedEvent, err := m.middleware.ProcessIncoming(ctx, event)
						if err != nil {
							m.logger.Error("middleware processing failed",
								String("transport", name),
								String("event_type", string(event.Type())),
								Error(err))
							continue
						}
						event = processedEvent
					}

					// Update metrics
					if m.metrics != nil {
						m.metrics.RecordEvent(name, event)
					}

					// Emit event
					if m.eventBus != nil {
						// Skip event publishing for now - requires proper events.Event implementation
						// receivedEvent := NewSimpleEvent("transport-event-received-"+name, string(EventTypeEventReceived), 
						//	map[string]interface{}{"transport": name, "event": event})
						// m.eventBus.Publish(ctx, "transport.event_received", receivedEvent)
					}

					// Send to result channel
					select {
					case resultChan <- event:
					case <-ctx.Done():
						return
					}

				case err := <-errorChan:
					// Handle errors from transport
					if err != nil {
						m.logger.Error("transport error received",
							String("transport", name),
							Error(err))
						// Forward the error to result error channel if available
						if m.eventBus != nil {
							// Emit transport error event
						}
						continue
					}

				case <-ctx.Done():
					return
				}
			}
		}(transportName, transport)
	}

	// Close result channel when all goroutines are done
	go func() {
		m.wg.Wait()
		close(resultChan)
	}()

	return resultChan, nil
}

// SetLoadBalancer sets the load balancing strategy.
func (m *DefaultTransportManager) SetLoadBalancer(balancer LoadBalancer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balancer = balancer
}

// SetMiddleware sets the middleware chain.
func (m *DefaultTransportManager) SetMiddleware(middleware MiddlewareChain) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.middleware = middleware
}

// SetEventBus sets the event bus.
func (m *DefaultTransportManager) SetEventBus(eventBus EventBus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventBus = eventBus
}

// startCleanupTicker starts the periodic cleanup goroutine
func (m *DefaultTransportManager) startCleanupTicker() {
	if m.config == nil || !m.config.CleanupEnabled {
		return
	}

	m.cleanupTicker = time.NewTicker(m.config.CleanupInterval)
	m.wg.Add(1)

	go func() {
		defer m.wg.Done()
		defer m.cleanupTicker.Stop()

		for {
			select {
			case <-m.cleanupTicker.C:
				m.runPeriodicCleanup()
			case <-m.ctx.Done():
				return
			}
		}
	}()

	m.logger.Info("Transport manager cleanup ticker started",
		String("interval", m.config.CleanupInterval.String()),
		Int("max_map_size", m.config.MaxMapSize),
		Float64("active_threshold", m.config.ActiveThreshold))
}

// checkAndTriggerCleanup checks if cleanup should be triggered and runs it if necessary
// This method is called with the manager lock already held
func (m *DefaultTransportManager) checkAndTriggerCleanup() {
	if m.config == nil || !m.config.CleanupEnabled {
		return
	}

	// Check if context is cancelled
	select {
	case <-m.ctx.Done():
		return
	default:
	}

	totalTransports := len(m.transports)
	if totalTransports <= m.config.MaxMapSize {
		return
	}

	// Count active transports
	activeCount := 0
	for _, transport := range m.transports {
		if transport.IsConnected() {
			activeCount++
		}
	}

	// Calculate active ratio
	activeRatio := float64(activeCount) / float64(totalTransports)

	// Trigger cleanup if we're above size threshold and below active threshold
	if activeRatio < m.config.ActiveThreshold {
		m.logger.Info("Triggering immediate cleanup due to size threshold",
			Int("total_transports", totalTransports),
			Int("active_transports", activeCount),
			Float64("active_ratio", activeRatio),
			Int("max_size", m.config.MaxMapSize),
			Float64("threshold", m.config.ActiveThreshold))

		// Run cleanup without additional locking since we already hold the lock
		m.cleanupInactiveTransportsLocked()
	}
}

// runPeriodicCleanup is called by the cleanup ticker
func (m *DefaultTransportManager) runPeriodicCleanup() {
	if m.config == nil || !m.config.CleanupEnabled {
		return
	}

	// Check if context is cancelled before proceeding
	select {
	case <-m.ctx.Done():
		return
	default:
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check context cancellation after acquiring lock
	select {
	case <-m.ctx.Done():
		return
	default:
	}

	totalTransports := len(m.transports)
	if totalTransports <= m.config.MaxMapSize {
		return
	}

	// Count active transports
	activeCount := 0
	for _, transport := range m.transports {
		if transport.IsConnected() {
			activeCount++
		}
	}

	// Calculate active ratio
	activeRatio := float64(activeCount) / float64(totalTransports)

	// Only run cleanup if we're below the active threshold
	if activeRatio < m.config.ActiveThreshold {
		m.logger.Info("Running periodic cleanup",
			Int("total_transports", totalTransports),
			Int("active_transports", activeCount),
			Float64("active_ratio", activeRatio))

		m.cleanupInactiveTransportsLocked()
	}
}

// cleanupInactiveTransportsLocked performs the actual cleanup operation
// This method assumes the manager mutex is already held
func (m *DefaultTransportManager) cleanupInactiveTransportsLocked() {
	if m.config == nil || !m.config.CleanupEnabled {
		return
	}

	startTime := time.Now()
	
	// Create new map to hold only active transports
	newTransports := make(map[string]Transport)
	removedCount := 0
	retainedCount := 0
	
	// Track transports that need to be cleaned up
	var toClose []Transport
	var closedNames []string

	// Iterate through existing transports
	for name, transport := range m.transports {
		if transport.IsConnected() {
			// Keep active transports
			newTransports[name] = transport
			retainedCount++
		} else {
			// Mark inactive transports for removal
			toClose = append(toClose, transport)
			closedNames = append(closedNames, name)
			removedCount++
		}
	}

	// Replace the map to reclaim memory
	oldLen := len(m.transports)
	m.transports = newTransports

	// Update cleanup metrics
	duration := time.Since(startTime)
	m.updateCleanupMetrics(removedCount, retainedCount, duration)

	m.logger.Info("Completed transport map cleanup",
		Int("old_map_size", oldLen),
		Int("new_map_size", len(m.transports)),
		Int("transports_removed", removedCount),
		Int("transports_retained", retainedCount),
		Duration("cleanup_duration", duration))

	// Close removed transports outside the lock to avoid blocking
	go m.closeRemovedTransports(toClose, closedNames)
}

// closeRemovedTransports closes transports that were removed during cleanup
func (m *DefaultTransportManager) closeRemovedTransports(transports []Transport, names []string) {
	for i, transport := range transports {
		name := names[i]
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		
		if err := transport.Close(closeCtx); err != nil {
			m.logger.Error("Failed to close removed transport during cleanup",
				String("transport", name),
				Error(err))
			
			// Update error metrics
			if m.config.CleanupMetricsEnabled {
				m.mapCleanupMetrics.mu.Lock()
				m.mapCleanupMetrics.CleanupErrors++
				m.mapCleanupMetrics.mu.Unlock()
			}
		} else {
			m.logger.Debug("Successfully closed removed transport",
				String("transport", name))
		}
		
		cancel()
		
		// Remove from health check and metrics managers
		if m.healthCheck != nil {
			m.healthCheck.RemoveTransport(name)
		}
		if m.metrics != nil {
			m.metrics.RemoveTransport(name)
		}
	}
}

// updateCleanupMetrics updates the cleanup operation metrics
func (m *DefaultTransportManager) updateCleanupMetrics(removed, retained int, duration time.Duration) {
	if !m.config.CleanupMetricsEnabled || m.mapCleanupMetrics == nil {
		return
	}

	m.mapCleanupMetrics.mu.Lock()
	defer m.mapCleanupMetrics.mu.Unlock()

	m.mapCleanupMetrics.TotalCleanups++
	m.mapCleanupMetrics.TransportsRemoved += uint64(removed)
	m.mapCleanupMetrics.TransportsRetained += uint64(retained)
	m.mapCleanupMetrics.LastCleanupDuration = duration
	m.mapCleanupMetrics.LastCleanupTime = time.Now()
	m.lastCleanupTime = time.Now()
}

// GetMapCleanupMetrics returns the current cleanup metrics
func (m *DefaultTransportManager) GetMapCleanupMetrics() MapCleanupMetrics {
	if m.mapCleanupMetrics == nil {
		return MapCleanupMetrics{}
	}

	m.mapCleanupMetrics.mu.RLock()
	defer m.mapCleanupMetrics.mu.RUnlock()

	// Return a copy to prevent external modification
	return *m.mapCleanupMetrics
}

// TriggerManualCleanup manually triggers a cleanup operation
func (m *DefaultTransportManager) TriggerManualCleanup() {
	if m.config == nil || !m.config.CleanupEnabled {
		m.logger.Warn("Manual cleanup requested but cleanup is disabled")
		return
	}

	// Check if context is cancelled before proceeding
	select {
	case <-m.ctx.Done():
		m.logger.Debug("Manual cleanup skipped - context cancelled")
		return
	default:
	}

	m.logger.Info("Manual cleanup triggered")
	m.runPeriodicCleanup()
}

// Close closes all managed transports.
func (m *DefaultTransportManager) Close() error {
	// First, acquire the lock to safely stop the cleanup ticker and signal shutdown
	m.mu.Lock()
	
	// Stop cleanup ticker first while holding the lock to prevent deadlock
	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
		m.cleanupTicker = nil
	}
	
	// Cancel context to signal all goroutines to stop
	if m.cancel != nil {
		m.cancel()
	}
	
	// Release the lock before waiting for goroutines to avoid deadlock
	m.mu.Unlock()
	
	// Wait for all goroutines (including cleanup goroutine) to finish
	// This must be done WITHOUT holding the lock since goroutines may need to acquire it
	m.wg.Wait()

	// Now safely acquire the lock for final cleanup
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error
	for name, transport := range m.transports {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := transport.Close(closeCtx); err != nil {
			errors = append(errors, fmt.Errorf("failed to close transport %s: %w", name, err))
		}
		cancel()
	}

	// Close managers
	if m.healthCheck != nil {
		if err := m.healthCheck.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close health checker: %w", err))
		}
	}

	if m.metrics != nil {
		if err := m.metrics.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close metrics manager: %w", err))
		}
	}

	if m.eventBus != nil {
		if err := m.eventBus.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close event bus: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("multiple errors occurred while closing transport manager: %v", errors)
	}

	return nil
}

// Stats returns aggregated statistics from all transports.
func (m *DefaultTransportManager) Stats() map[string]TransportStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]TransportStats, len(m.transports))
	for name, transport := range m.transports {
		stats[name] = transport.Stats()
	}

	return stats
}

// DefaultMiddlewareChain is the default implementation of MiddlewareChain.
type DefaultMiddlewareChain struct {
	mu          sync.RWMutex
	middlewares []Middleware
}

// NewDefaultMiddlewareChain creates a new default middleware chain.
func NewDefaultMiddlewareChain() *DefaultMiddlewareChain {
	return &DefaultMiddlewareChain{
		middlewares: make([]Middleware, 0),
	}
}

// Add adds middleware to the chain.
func (c *DefaultMiddlewareChain) Add(middleware Middleware) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.middlewares = append(c.middlewares, middleware)
}

// ProcessOutgoing processes an outgoing event through the middleware chain.
func (c *DefaultMiddlewareChain) ProcessOutgoing(ctx context.Context, event TransportEvent) (TransportEvent, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, middleware := range c.middlewares {
		var err error
		event, err = middleware.ProcessOutgoing(ctx, event)
		if err != nil {
			return nil, fmt.Errorf("middleware %s failed to process outgoing event: %w", middleware.Name(), err)
		}
	}

	return event, nil
}

// ProcessIncoming processes an incoming event through the middleware chain.
func (c *DefaultMiddlewareChain) ProcessIncoming(ctx context.Context, event events.Event) (events.Event, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Process in reverse order for incoming events
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		middleware := c.middlewares[i]
		var err error
		event, err = middleware.ProcessIncoming(ctx, event)
		if err != nil {
			return nil, fmt.Errorf("middleware %s failed to process incoming event: %w", middleware.Name(), err)
		}
	}

	return event, nil
}

// Clear removes all middleware from the chain.
func (c *DefaultMiddlewareChain) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.middlewares = make([]Middleware, 0)
}

// RoundRobinLoadBalancer is a simple round-robin load balancer.
type RoundRobinLoadBalancer struct {
	mu    sync.Mutex
	index int
}

// NewRoundRobinLoadBalancer creates a new round-robin load balancer.
func NewRoundRobinLoadBalancer() *RoundRobinLoadBalancer {
	return &RoundRobinLoadBalancer{}
}

// SelectTransport selects a transport using round-robin algorithm.
func (lb *RoundRobinLoadBalancer) SelectTransport(transports map[string]Transport, event TransportEvent) (string, error) {
	if len(transports) == 0 {
		return "", fmt.Errorf("no transports available")
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	// Get transport names as a slice
	names := make([]string, 0, len(transports))
	for name := range transports {
		names = append(names, name)
	}

	// Select transport using round-robin
	selected := names[lb.index%len(names)]
	lb.index++

	return selected, nil
}

// UpdateStats updates the load balancer with transport statistics.
func (lb *RoundRobinLoadBalancer) UpdateStats(transportName string, stats TransportStats) {
	// Round-robin doesn't use stats, so this is a no-op
}

// Name returns the load balancer name.
func (lb *RoundRobinLoadBalancer) Name() string {
	return "round_robin"
}

// WeightedLoadBalancer is a weighted load balancer based on transport performance.
type WeightedLoadBalancer struct {
	mu      sync.RWMutex
	weights map[string]int
	stats   map[string]TransportStats
}

// NewWeightedLoadBalancer creates a new weighted load balancer.
func NewWeightedLoadBalancer() *WeightedLoadBalancer {
	return &WeightedLoadBalancer{
		weights: make(map[string]int),
		stats:   make(map[string]TransportStats),
	}
}

// SelectTransport selects a transport using weighted algorithm.
func (lb *WeightedLoadBalancer) SelectTransport(transports map[string]Transport, event TransportEvent) (string, error) {
	if len(transports) == 0 {
		return "", fmt.Errorf("no transports available")
	}

	lb.mu.RLock()
	defer lb.mu.RUnlock()

	// Calculate weights based on performance
	bestTransport := ""
	bestWeight := -1

	for name := range transports {
		weight := lb.calculateWeight(name)
		if weight > bestWeight {
			bestWeight = weight
			bestTransport = name
		}
	}

	if bestTransport == "" {
		// Fallback to first available transport
		for name := range transports {
			return name, nil
		}
	}

	return bestTransport, nil
}

// calculateWeight calculates the weight for a transport based on its stats.
func (lb *WeightedLoadBalancer) calculateWeight(transportName string) int {
	stats, exists := lb.stats[transportName]
	if !exists {
		return 100 // Default weight
	}

	// Calculate weight based on performance metrics
	weight := 100
	
	// Reduce weight based on error rate
	if stats.EventsSent > 0 {
		errorRate := float64(stats.ErrorCount) / float64(stats.EventsSent)
		weight -= int(errorRate * 50)
	}

	// Reduce weight based on latency
	if stats.AverageLatency > 0 {
		latencyMs := stats.AverageLatency.Milliseconds()
		if latencyMs > 1000 {
			weight -= int(latencyMs / 100)
		}
	}

	if weight < 1 {
		weight = 1
	}

	return weight
}

// UpdateStats updates the load balancer with transport statistics.
func (lb *WeightedLoadBalancer) UpdateStats(transportName string, stats TransportStats) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.stats[transportName] = stats
}

// Name returns the load balancer name.
func (lb *WeightedLoadBalancer) Name() string {
	return "weighted"
}

// HealthCheckManager manages health checks for transports.
type HealthCheckManager struct {
	mu         sync.RWMutex
	transports map[string]Transport
	checkers   map[string]HealthChecker
	interval   time.Duration
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	logger     Logger
	onUnhealthy func(name string, err error)
}

// NewHealthCheckManager creates a new health check manager.
func NewHealthCheckManager() *HealthCheckManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &HealthCheckManager{
		transports: make(map[string]Transport),
		checkers:   make(map[string]HealthChecker),
		interval:   30 * time.Second,
		ctx:        ctx,
		cancel:     cancel,
		logger:     NewLogger(DefaultLoggerConfig()),
	}
}

// AddTransport adds a transport to health checking.
func (m *HealthCheckManager) AddTransport(name string, transport Transport) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.transports[name] = transport
	
	// Start health checking if transport implements HealthChecker
	if checker, ok := transport.(HealthChecker); ok {
		m.checkers[name] = checker
		m.startHealthCheck(name, checker)
	}
}

// RemoveTransport removes a transport from health checking.
func (m *HealthCheckManager) RemoveTransport(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.transports, name)
	delete(m.checkers, name)
}

// startHealthCheck starts health checking for a transport.
func (m *HealthCheckManager) startHealthCheck(name string, checker HealthChecker) {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := checker.CheckHealth(m.ctx); err != nil {
					m.logger.Error("health check failed",
						String("transport", name),
						Error(err))
					// Notify transport manager about unhealthy transport
					if m.onUnhealthy != nil {
						m.onUnhealthy(name, err)
					}
				}
			case <-m.ctx.Done():
				return
			}
		}
	}()
}

// Close closes the health check manager.
func (m *HealthCheckManager) Close() error {
	m.cancel()
	m.wg.Wait()
	return nil
}

// MetricsManager manages metrics collection for transports.
type MetricsManager struct {
	mu         sync.RWMutex
	transports map[string]Transport
	collectors map[string]MetricsCollector
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewMetricsManager creates a new metrics manager.
func NewMetricsManager() *MetricsManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &MetricsManager{
		transports: make(map[string]Transport),
		collectors: make(map[string]MetricsCollector),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// AddTransport adds a transport to metrics collection.
func (m *MetricsManager) AddTransport(name string, transport Transport) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.transports[name] = transport
	
	// Start metrics collection if transport implements MetricsCollector
	if collector, ok := transport.(MetricsCollector); ok {
		m.collectors[name] = collector
	}
}

// RemoveTransport removes a transport from metrics collection.
func (m *MetricsManager) RemoveTransport(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.transports, name)
	delete(m.collectors, name)
}

// RecordEvent records an event metric.
func (m *MetricsManager) RecordEvent(transportName string, event any) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if collector, exists := m.collectors[transportName]; exists {
		// Calculate event size
		eventSize := m.calculateEventSize(event)
		
		// Calculate latency based on event timestamp
		latency := m.calculateEventLatency(event)
		
		// Record the event with calculated metrics
		collector.RecordEvent("event", eventSize, latency)
	}
}

// calculateEventSize calculates the size of an event in bytes.
func (m *MetricsManager) calculateEventSize(event any) int64 {
	if event == nil {
		return 0
	}

	// Handle TransportEvent interface specifically
	if transportEvent, ok := event.(TransportEvent); ok {
		// Create a serializable representation of the transport event
		eventMap := map[string]interface{}{
			"id":        transportEvent.ID(),
			"type":      transportEvent.Type(),
			"timestamp": transportEvent.Timestamp(),
			"data":      transportEvent.Data(),
		}
		
		// Marshal to JSON to calculate size
		if jsonData, err := json.Marshal(eventMap); err == nil {
			return int64(len(jsonData))
		}
	}

	// Fallback: try to marshal the event directly
	if jsonData, err := json.Marshal(event); err == nil {
		return int64(len(jsonData))
	}

	// If JSON marshaling fails, estimate size using reflection
	return m.estimateEventSize(event)
}

// calculateEventLatency calculates the latency for an event based on its timestamp.
func (m *MetricsManager) calculateEventLatency(event any) time.Duration {
	if event == nil {
		return 0
	}

	// Handle TransportEvent interface
	if transportEvent, ok := event.(TransportEvent); ok {
		eventTimestamp := transportEvent.Timestamp()
		if !eventTimestamp.IsZero() {
			return time.Since(eventTimestamp)
		}
	}

	// Handle events.Event interface
	if coreEvent, ok := event.(events.Event); ok {
		eventTimestamp := coreEvent.Timestamp()
		if eventTimestamp != nil && *eventTimestamp > 0 {
			// Convert Unix milliseconds to time.Time
			timestamp := time.Unix(0, *eventTimestamp*int64(time.Millisecond))
			return time.Since(timestamp)
		}
	}

	// Try to extract timestamp using reflection
	v := reflect.ValueOf(event)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		// Look for common timestamp field names
		timestampFields := []string{"Timestamp", "CreatedAt", "EventTimestamp", "Time"}
		for _, fieldName := range timestampFields {
			if field := v.FieldByName(fieldName); field.IsValid() && field.Type() == reflect.TypeOf(time.Time{}) {
				if timestamp, ok := field.Interface().(time.Time); ok && !timestamp.IsZero() {
					return time.Since(timestamp)
				}
			}
		}
	}

	// If no timestamp found, return 0 (no latency calculated)
	return 0
}

// estimateEventSize estimates the size of an event using reflection when JSON marshaling fails.
func (m *MetricsManager) estimateEventSize(event any) int64 {
	if event == nil {
		return 0
	}

	v := reflect.ValueOf(event)
	return m.estimateValueSize(v)
}

// estimateValueSize recursively estimates the memory size of a value.
func (m *MetricsManager) estimateValueSize(v reflect.Value) int64 {
	if !v.IsValid() {
		return 0
	}

	var size int64

	switch v.Kind() {
	case reflect.String:
		size = int64(len(v.String()))
	case reflect.Slice, reflect.Array:
		size = int64(v.Len()) * 8 // Estimate 8 bytes per element
		for i := 0; i < v.Len(); i++ {
			size += m.estimateValueSize(v.Index(i))
		}
	case reflect.Map:
		size = int64(v.Len()) * 16 // Estimate 16 bytes per map entry
		for _, key := range v.MapKeys() {
			size += m.estimateValueSize(key)
			size += m.estimateValueSize(v.MapIndex(key))
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			size += m.estimateValueSize(v.Field(i))
		}
	case reflect.Ptr:
		if !v.IsNil() {
			size = 8 + m.estimateValueSize(v.Elem()) // 8 bytes for pointer + content
		}
	case reflect.Interface:
		if !v.IsNil() {
			size = 8 + m.estimateValueSize(v.Elem()) // 8 bytes for interface + content
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		size = 8
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		size = 8
	case reflect.Float32, reflect.Float64:
		size = 8
	case reflect.Bool:
		size = 1
	default:
		size = 8 // Default size for unknown types
	}

	return size
}

// RecordError records an error metric.
func (m *MetricsManager) RecordError(transportName string, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if collector, exists := m.collectors[transportName]; exists {
		collector.RecordError("transport_error", err)
	}
}

// Close closes the metrics manager.
func (m *MetricsManager) Close() error {
	m.cancel()
	m.wg.Wait()
	return nil
}

// Global registry instance
var globalRegistry = NewDefaultTransportRegistry()

// Register registers a transport factory globally.
func Register(transportType string, factory TransportFactory) error {
	return globalRegistry.Register(transportType, factory)
}

// Create creates a transport using the global registry.
func Create(config Config) (Transport, error) {
	return globalRegistry.Create(config)
}

// CreateWithContext creates a transport with context using the global registry.
func CreateWithContext(ctx context.Context, config Config) (Transport, error) {
	return globalRegistry.CreateWithContext(ctx, config)
}

// GetRegisteredTypes returns all registered transport types from the global registry.
func GetRegisteredTypes() []string {
	return globalRegistry.GetRegisteredTypes()
}

// IsRegistered checks if a transport type is registered in the global registry.
func IsRegistered(transportType string) bool {
	return globalRegistry.IsRegistered(transportType)
}