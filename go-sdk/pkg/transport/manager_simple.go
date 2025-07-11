package transport

import (
	"context"
	"sync"
	"time"
)

// SimpleManager provides basic transport management without import cycles
type SimpleManager struct {
	mu                  sync.RWMutex
	activeTransport     Transport
	eventChan           chan Event
	errorChan           chan error
	stopChan            chan struct{}
	transportReady      chan struct{}
	running             bool
	backpressureHandler *BackpressureHandler
	backpressureConfig  BackpressureConfig
	validator           Validator
	validationConfig    *ValidationConfig
	validationEnabled   bool
}

// NewSimpleManager creates a new simple transport manager
func NewSimpleManager() *SimpleManager {
	return NewSimpleManagerWithBackpressure(BackpressureConfig{
		Strategy:      BackpressureNone,
		BufferSize:    100,
		HighWaterMark: 0.8,
		LowWaterMark:  0.2,
		BlockTimeout:  5 * time.Second,
		EnableMetrics: true,
	})
}

// NewSimpleManagerWithValidation creates a new simple transport manager with validation
func NewSimpleManagerWithValidation(backpressureConfig BackpressureConfig, validationConfig *ValidationConfig) *SimpleManager {
	manager := NewSimpleManagerWithBackpressure(backpressureConfig)
	manager.SetValidationConfig(validationConfig)
	return manager
}

// NewSimpleManagerWithBackpressure creates a new simple transport manager with backpressure configuration
func NewSimpleManagerWithBackpressure(backpressureConfig BackpressureConfig) *SimpleManager {
	manager := &SimpleManager{
		stopChan:           make(chan struct{}),
		transportReady:     make(chan struct{}, 1),
		backpressureConfig: backpressureConfig,
	}
	
	// Initialize backpressure handler
	manager.backpressureHandler = NewBackpressureHandler(backpressureConfig)
	manager.eventChan = make(chan Event, backpressureConfig.BufferSize)
	manager.errorChan = make(chan error, backpressureConfig.BufferSize)
	
	return manager
}

// SetTransport sets the active transport
func (m *SimpleManager) SetTransport(transport Transport) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.activeTransport != nil {
		// Use a default timeout context for closing the old transport
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.activeTransport.Close(ctx)
	}
	
	m.activeTransport = transport
}

// Start starts the manager
func (m *SimpleManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.running {
		return ErrAlreadyConnected
	}
	
	if m.activeTransport != nil {
		if err := m.activeTransport.Connect(ctx); err != nil {
			return err
		}
		
		// Start receiving events
		go m.receiveEvents()
	}
	
	m.running = true
	return nil
}

// Stop stops the manager
func (m *SimpleManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.running {
		return nil
	}
	
	// Set running to false immediately to prevent new operations
	m.running = false
	
	// Only close the channel if it's not already closed
	select {
	case <-m.stopChan:
		// Channel is already closed
	default:
		close(m.stopChan)
	}
	
	if m.activeTransport != nil {
		if err := m.activeTransport.Close(ctx); err != nil {
			// Even on error, we keep running=false to ensure shutdown
			return err
		}
	}
	
	// Stop backpressure handler
	if m.backpressureHandler != nil {
		m.backpressureHandler.Stop()
	}
	
	return nil
}

// Send sends an event
func (m *SimpleManager) Send(ctx context.Context, event TransportEvent) error {
	m.mu.RLock()
	transport := m.activeTransport
	validationEnabled := m.validationEnabled
	validator := m.validator
	m.mu.RUnlock()
	
	if transport == nil {
		return ErrNotConnected
	}
	
	// Validate outgoing event if validation is enabled
	if validationEnabled && validator != nil {
		if err := validator.ValidateOutgoing(ctx, event); err != nil {
			return err
		}
	}
	
	return transport.Send(ctx, event)
}

// Receive returns the event channel
func (m *SimpleManager) Receive() <-chan Event {
	if m.backpressureHandler != nil {
		return m.backpressureHandler.EventChan()
	}
	return m.eventChan
}

// Errors returns the error channel
func (m *SimpleManager) Errors() <-chan error {
	if m.backpressureHandler != nil {
		return m.backpressureHandler.ErrorChan()
	}
	return m.errorChan
}

// GetBackpressureMetrics returns the current backpressure metrics
func (m *SimpleManager) GetBackpressureMetrics() BackpressureMetrics {
	if m.backpressureHandler != nil {
		return m.backpressureHandler.GetMetrics()
	}
	return BackpressureMetrics{}
}

// SetValidationConfig sets the validation configuration
func (m *SimpleManager) SetValidationConfig(config *ValidationConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if config == nil {
		m.validationEnabled = false
		m.validator = nil
		m.validationConfig = nil
		return
	}
	
	m.validationConfig = config
	m.validator = NewValidator(config)
	m.validationEnabled = config.Enabled
}

// GetValidationConfig returns the current validation configuration
func (m *SimpleManager) GetValidationConfig() *ValidationConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.validationConfig
}

// SetValidationEnabled enables or disables validation
func (m *SimpleManager) SetValidationEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validationEnabled = enabled
}

// IsValidationEnabled returns whether validation is enabled
func (m *SimpleManager) IsValidationEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.validationEnabled
}

// receiveEvents receives events from the active transport
func (m *SimpleManager) receiveEvents() {
	for {
		select {
		case <-m.stopChan:
			return
		default:
			if m.activeTransport != nil {
				select {
				case event := <-m.activeTransport.Receive():
					// Validate incoming event if validation is enabled
					m.mu.RLock()
					validationEnabled := m.validationEnabled
					validator := m.validator
					m.mu.RUnlock()
					
					if validationEnabled && validator != nil {
						ctx := context.Background()
						if err := validator.ValidateIncoming(ctx, event.Event); err != nil {
							// Add validation error to event metadata
							event.Metadata.Headers["validation_error"] = err.Error()
							event.Metadata.Headers["validation_failed"] = "true"
						} else {
							event.Metadata.Headers["validation_passed"] = "true"
						}
					}
					
					// Use backpressure handler to send event
					if m.backpressureHandler != nil {
						m.backpressureHandler.SendEvent(event)
					} else {
						select {
						case m.eventChan <- event:
						case <-m.stopChan:
							return
						}
					}
				case err := <-m.activeTransport.Errors():
					// Use backpressure handler to send error
					if m.backpressureHandler != nil {
						m.backpressureHandler.SendError(err)
					} else {
						select {
						case m.errorChan <- err:
						case <-m.stopChan:
							return
						}
					}
				case <-m.stopChan:
					return
				}
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}