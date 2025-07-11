package transport

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Config represents simplified transport configuration
type Config struct {
	Primary       string
	Fallback      []string
	BufferSize    int
	LogLevel      string
	EnableMetrics bool
	Backpressure  BackpressureConfig
	Validation    *ValidationConfig
}

// Manager orchestrates transport operations including selection, failover, and load balancing
type Manager struct {
	mu                  sync.RWMutex
	config              *Config
	activeTransport     Transport
	fallbackQueue       []string
	middleware          []Middleware
	eventChan           chan Event
	errorChan           chan error
	stopChan            chan struct{}
	running             bool
	metrics             *ManagerMetrics
	logger              Logger
	backpressureHandler *BackpressureHandler
	validator           Validator
	validationEnabled   bool
}

// ManagerMetrics contains metrics for the transport manager
type ManagerMetrics struct {
	mu                     sync.RWMutex
	TransportSwitches      uint64
	TotalConnections       uint64
	ActiveConnections      uint64
	FailedConnections      uint64
	TotalMessagesSent      uint64
	TotalMessagesReceived  uint64
	TotalBytesSent         uint64
	TotalBytesReceived     uint64
	AverageLatency         time.Duration
	LastTransportSwitch    time.Time
	TransportHealthScores  map[string]float64
}

// NewManager creates a new transport manager
func NewManager(cfg *Config) *Manager {
	if cfg == nil {
		cfg = &Config{
			Primary:     "websocket",
			Fallback:    []string{"sse", "http"},
			BufferSize:  1024,
			LogLevel:    "info",
			EnableMetrics: true,
			Backpressure: BackpressureConfig{
				Strategy:      BackpressureNone,
				BufferSize:    1024,
				HighWaterMark: 0.8,
				LowWaterMark:  0.2,
				BlockTimeout:  5 * time.Second,
				EnableMetrics: true,
			},
			Validation: DefaultValidationConfig(),
		}
	}
	
	manager := &Manager{
		config:        cfg,
		middleware:    []Middleware{},
		eventChan:     make(chan Event, cfg.BufferSize),
		errorChan:     make(chan error, cfg.BufferSize),
		stopChan:      make(chan struct{}),
		metrics:       &ManagerMetrics{
			TransportHealthScores: make(map[string]float64),
		},
		logger:        NewNoopLogger(),
	}
	
	// Initialize backpressure handler
	manager.backpressureHandler = NewBackpressureHandler(cfg.Backpressure)

	// Initialize fallback queue
	manager.fallbackQueue = make([]string, len(cfg.Fallback))
	copy(manager.fallbackQueue, cfg.Fallback)

	// Initialize validation
	if cfg.Validation != nil {
		manager.validator = NewValidator(cfg.Validation)
		manager.validationEnabled = cfg.Validation.Enabled
	}

	return manager
}

// NewManagerWithLogger creates a new transport manager with a custom logger
func NewManagerWithLogger(cfg *Config, logger Logger) *Manager {
	manager := NewManager(cfg)
	if logger != nil {
		manager.logger = logger
	}
	return manager
}

// Start starts the transport manager
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("Starting transport manager", 
		String("operation", "start"))

	if m.running {
		m.logger.Debug("Manager already running", 
			String("operation", "start"))
		return fmt.Errorf("transport manager already running")
	}

	// Start the manager
	m.running = true
	
	m.logger.Info("Transport manager started successfully", 
		String("operation", "start"))
	
	return nil
}

// Stop stops the transport manager
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("Stopping transport manager", 
		String("operation", "stop"))

	if !m.running {
		m.logger.Debug("Manager already stopped", 
			String("operation", "stop"))
		return nil
	}

	// Signal stop
	close(m.stopChan)

	// Drain event channels with timeout
	drainCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	
	m.logger.Debug("Draining event channels", 
		String("operation", "stop"),
		Duration("timeout", 2*time.Second))

	// Create a wait group to track draining completion
	var wg sync.WaitGroup
	
	// Drain eventChan
	wg.Add(1)
	go func() {
		defer wg.Done()
		eventCount := 0
		for {
			select {
			case <-m.eventChan:
				eventCount++
				// Discard event but continue draining
			case <-drainCtx.Done():
				if eventCount > 0 {
					m.logger.Debug("Drained events from event channel", 
						String("operation", "stop"),
						Int("events_drained", eventCount))
				}
				return
			}
		}
	}()
	
	// Drain errorChan
	wg.Add(1)
	go func() {
		defer wg.Done()
		errorCount := 0
		for {
			select {
			case <-m.errorChan:
				errorCount++
				// Discard error but continue draining
			case <-drainCtx.Done():
				if errorCount > 0 {
					m.logger.Debug("Drained errors from error channel", 
						String("operation", "stop"),
						Int("errors_drained", errorCount))
				}
				return
			}
		}
	}()
	
	// Wait for draining to complete or timeout
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()
	
	select {
	case <-doneChan:
		m.logger.Debug("Event channels drained successfully", 
			String("operation", "stop"))
	case <-drainCtx.Done():
		m.logger.Warn("Event channel draining timed out", 
			String("operation", "stop"))
	}

	// Close active transport
	if m.activeTransport != nil {
		if err := m.activeTransport.Close(ctx); err != nil {
			m.logger.Error("Failed to close active transport", 
				String("operation", "stop"),
				Error(err))
			return fmt.Errorf("failed to close active transport: %w", err)
		}
		
		m.logger.Debug("Active transport closed successfully", 
			String("operation", "stop"))
	}

	// Stop backpressure handler
	if m.backpressureHandler != nil {
		m.backpressureHandler.Stop()
		m.logger.Debug("Backpressure handler stopped", 
			String("operation", "stop"))
	}

	m.running = false
	m.logger.Info("Transport manager stopped successfully", 
		String("operation", "stop"))
	
	return nil
}

// Send sends an event through the active transport
func (m *Manager) Send(ctx context.Context, event TransportEvent) error {
	m.mu.RLock()
	transport := m.activeTransport
	validationEnabled := m.validationEnabled
	validator := m.validator
	m.mu.RUnlock()

	if transport == nil {
		m.logger.Error("Cannot send event: no active transport", 
			String("operation", "send"),
			String("event_id", event.ID()),
			String("event_type", event.Type()))
		return ErrNotConnected
	}

	// Validate outgoing event if validation is enabled
	if validationEnabled && validator != nil {
		if err := validator.ValidateOutgoing(ctx, event); err != nil {
			m.logger.Error("Event validation failed", 
				String("operation", "send"),
				String("event_id", event.ID()),
				String("event_type", event.Type()),
				Error(err))
			return err
		}
	}

	m.logger.Debug("Sending event through active transport", 
		String("operation", "send"),
		String("event_id", event.ID()),
		String("event_type", event.Type()))

	// Apply middleware
	finalTransport := transport
	for i := len(m.middleware) - 1; i >= 0; i-- {
		finalTransport = m.middleware[i].Wrap(finalTransport)
	}

	// Send event
	err := finalTransport.Send(ctx, event)
	if err != nil {
		m.logger.Error("Failed to send event", 
			String("operation", "send"),
			String("event_id", event.ID()),
			Error(err))
		return err
	}

	m.logger.Debug("Event sent successfully", 
		String("operation", "send"),
		String("event_id", event.ID()))

	// Update metrics
	m.updateSendMetrics()

	return nil
}

// Receive returns the event channel for receiving events
func (m *Manager) Receive() <-chan Event {
	if m.backpressureHandler != nil {
		return m.backpressureHandler.EventChan()
	}
	return m.eventChan
}

// Errors returns the error channel
func (m *Manager) Errors() <-chan error {
	if m.backpressureHandler != nil {
		return m.backpressureHandler.ErrorChan()
	}
	return m.errorChan
}

// GetActiveTransport returns the currently active transport
func (m *Manager) GetActiveTransport() Transport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeTransport
}

// GetBackpressureMetrics returns the current backpressure metrics
func (m *Manager) GetBackpressureMetrics() BackpressureMetrics {
	if m.backpressureHandler != nil {
		return m.backpressureHandler.GetMetrics()
	}
	return BackpressureMetrics{}
}

// GetMetrics returns the manager metrics
func (m *Manager) GetMetrics() ManagerMetrics {
	m.metrics.mu.RLock()
	defer m.metrics.mu.RUnlock()
	
	// Deep copy metrics
	metrics := *m.metrics
	metrics.TransportHealthScores = make(map[string]float64)
	for k, v := range m.metrics.TransportHealthScores {
		metrics.TransportHealthScores[k] = v
	}
	
	return metrics
}

// SetTransport sets the active transport
func (m *Manager) SetTransport(transport Transport) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.logger.Debug("Setting active transport", 
		String("operation", "set_transport"))
	
	if m.activeTransport != nil {
		// Use a default timeout context for closing the old transport
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.activeTransport.Close(ctx)
		m.logger.Debug("Previous transport closed", 
			String("operation", "set_transport"))
	}
	
	m.activeTransport = transport
	
	if transport != nil {
		m.logger.Debug("New transport set successfully", 
			String("operation", "set_transport"))
		
		// Start receiving events from the new transport
		go m.receiveEvents(transport)
	}
}

// receiveEvents receives events from a transport
func (m *Manager) receiveEvents(transport Transport) {
	m.logger.Debug("Starting event receiver for transport", 
		String("operation", "receive_events"))
	
	defer m.logger.Debug("Event receiver stopped for transport", 
		String("operation", "receive_events"))

	for {
		select {
		case event := <-transport.Receive():
			m.logger.Debug("Received event from transport", 
				String("operation", "receive_events"),
				String("event_id", event.Event.ID()),
				String("event_type", event.Event.Type()))
			
			// Validate incoming event if validation is enabled
			m.mu.RLock()
			validationEnabled := m.validationEnabled
			validator := m.validator
			m.mu.RUnlock()
			
			if validationEnabled && validator != nil {
				ctx := context.Background()
				if err := validator.ValidateIncoming(ctx, event.Event); err != nil {
					m.logger.Warn("Incoming event validation failed", 
						String("operation", "receive_events"),
						String("event_id", event.Event.ID()),
						String("event_type", event.Event.Type()),
						Error(err))
					
					// Add validation error to event metadata
					event.Metadata.Headers["validation_error"] = err.Error()
					event.Metadata.Headers["validation_failed"] = "true"
				} else {
					event.Metadata.Headers["validation_passed"] = "true"
				}
			}
			
			// Use backpressure handler to send event
			if err := m.backpressureHandler.SendEvent(event); err != nil {
				m.logger.Warn("Failed to send event due to backpressure", 
					String("operation", "receive_events"),
					String("event_id", event.Event.ID()),
					Error(err))
			} else {
				m.logger.Debug("Event forwarded to event channel", 
					String("operation", "receive_events"),
					String("event_id", event.Event.ID()))
			}
		case err := <-transport.Errors():
			m.logger.Error("Received error from transport", 
				String("operation", "receive_events"),
				Error(err))
			
			// Use backpressure handler to send error
			if sendErr := m.backpressureHandler.SendError(err); sendErr != nil {
				m.logger.Warn("Failed to send error due to backpressure", 
					String("operation", "receive_events"),
					Error(err),
					Any("send_error", sendErr))
			} else {
				m.logger.Debug("Error forwarded to error channel", 
					String("operation", "receive_events"))
			}
		case <-m.stopChan:
			m.logger.Debug("Stop signal received", 
				String("operation", "receive_events"))
			return
		}
	}
}

// AddMiddleware adds middleware to the transport stack
func (m *Manager) AddMiddleware(middleware ...Middleware) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.middleware = append(m.middleware, middleware...)
}

// updateSendMetrics updates send-related metrics
func (m *Manager) updateSendMetrics() {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()

	m.metrics.TotalMessagesSent++
}

// SetValidationConfig sets the validation configuration
func (m *Manager) SetValidationConfig(config *ValidationConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if config == nil {
		m.validationEnabled = false
		m.validator = nil
		m.config.Validation = nil
		return
	}
	
	m.config.Validation = config
	m.validator = NewValidator(config)
	m.validationEnabled = config.Enabled
	
	m.logger.Debug("Validation configuration updated", 
		String("operation", "set_validation_config"),
		Bool("enabled", config.Enabled))
}

// GetValidationConfig returns the current validation configuration
func (m *Manager) GetValidationConfig() *ValidationConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Validation
}

// SetValidationEnabled enables or disables validation
func (m *Manager) SetValidationEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.validationEnabled = enabled
	
	m.logger.Debug("Validation enabled/disabled", 
		String("operation", "set_validation_enabled"),
		Bool("enabled", enabled))
}

// IsValidationEnabled returns whether validation is enabled
func (m *Manager) IsValidationEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.validationEnabled
}