package transport

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// SimpleManager provides basic transport management without import cycles
type SimpleManager struct {
	mu                  sync.RWMutex
	activeTransport     Transport
	eventChan           chan Event
	errorChan           chan error
	stopChan            chan struct{}
	transportStopChan   chan struct{} // To stop receiveEvents for old transport
	transportReady      chan struct{}
	running             int32 // Use atomic int32 for thread-safe access
	backpressureHandler *BackpressureHandler
	backpressureConfig  BackpressureConfig
	validator           Validator
	validationConfig    *ValidationConfig
	validationEnabled   bool
	receiveWg           sync.WaitGroup // Track receiveEvents goroutines
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
		transportStopChan:  make(chan struct{}),
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
	
	// Stop the old receiveEvents goroutine if there is one
	if m.transportStopChan != nil {
		close(m.transportStopChan)
		// Create a new stop channel for the new transport
		m.transportStopChan = make(chan struct{})
	}
	
	if m.activeTransport != nil {
		// Use a default timeout context for closing the old transport
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.activeTransport.Close(ctx)
	}
	
	m.activeTransport = transport
	
	// If the manager is running and we have a transport, start receiving
	if atomic.LoadInt32(&m.running) == 1 && transport != nil {
		m.receiveWg.Add(1)
		go m.receiveEvents()
	}
	
	// Signal that transport is ready (non-blocking send)
	if transport != nil {
		select {
		case m.transportReady <- struct{}{}:
		default:
			// Channel already has a value, which is fine
		}
	}
}

// Start starts the manager
func (m *SimpleManager) Start(ctx context.Context) error {
	// Use atomic CAS to ensure only one goroutine can start
	if !atomic.CompareAndSwapInt32(&m.running, 0, 1) {
		return ErrAlreadyConnected
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.activeTransport != nil {
		if err := m.activeTransport.Connect(ctx); err != nil {
			// Reset the flag on error to maintain consistency
			atomic.StoreInt32(&m.running, 0)
			return err
		}
		
		// Start receiving events
		m.receiveWg.Add(1)
		go m.receiveEvents()
		
		// Signal that transport is ready (non-blocking send)
		select {
		case m.transportReady <- struct{}{}:
		default:
			// Channel already has a value, which is fine
		}
	}
	
	return nil
}

// Stop stops the manager
func (m *SimpleManager) Stop(ctx context.Context) error {
	// Use atomic CAS to ensure only one goroutine can stop
	if !atomic.CompareAndSwapInt32(&m.running, 1, 0) {
		return nil
	}
	
	m.mu.Lock()
	
	// Close the stop channel to signal all goroutines to stop
	select {
	case <-m.stopChan:
		// Channel is already closed
	default:
		close(m.stopChan)
	}
	
	// Close transport stop channel if set
	if m.transportStopChan != nil {
		select {
		case <-m.transportStopChan:
			// Channel is already closed
		default:
			close(m.transportStopChan)
		}
	}
	
	// Unlock before waiting for goroutines
	m.mu.Unlock()
	
	// Wait for all receiveEvents goroutines to finish
	done := make(chan struct{})
	go func() {
		m.receiveWg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// All goroutines finished
	case <-ctx.Done():
		// Context cancelled
	case <-time.After(5 * time.Second):
		// Timeout waiting for goroutines
	}
	
	// Lock again for final cleanup
	m.mu.Lock()
	defer m.mu.Unlock()
	
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
	
	
	// Reset the transport ready channel
	// Drain any pending signals
	select {
	case <-m.transportReady:
	default:
	}
	
	// Drain channels before closing to prevent data loss
	if m.backpressureHandler == nil {
		// Only drain if we're managing channels directly
		// Use a reasonable timeout for draining
		drainTimeout := 100 * time.Millisecond
		
		drained := make(chan struct{})
		go func() {
			defer close(drained)
			
			// Drain events
			go func() {
				for range m.eventChan {
					// Drain events
				}
			}()
			
			// Drain errors
			go func() {
				for range m.errorChan {
					// Drain errors
				}
			}()
			
			// Give draining goroutines a moment to start
			time.Sleep(10 * time.Millisecond)
			
			// Close channels
			close(m.eventChan)
			close(m.errorChan)
		}()
		
		// Wait for draining with timeout
		select {
		case <-drained:
			// Successfully drained
		case <-time.After(drainTimeout):
			// Timeout during draining, but we don't fail
		}
	}
	
	// Return nil even if we timed out, as per test expectations
	// The timeout is handled gracefully without returning an error
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
	defer m.receiveWg.Done()
	
	// Get a copy of the transport stop channel to avoid races
	m.mu.RLock()
	transportStopChan := m.transportStopChan
	m.mu.RUnlock()
	
	for {
		select {
		case <-m.stopChan:
			return
		case <-transportStopChan:
			return
		default:
			// Get transport reference under lock
			m.mu.RLock()
			transport := m.activeTransport
			m.mu.RUnlock()
			
			if transport != nil {
				select {
				case event := <-transport.Receive():
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
						// Ignore error from SendEvent as backpressure handler
						// manages drops internally and updates metrics
						_ = m.backpressureHandler.SendEvent(event)
					} else {
						select {
						case m.eventChan <- event:
						case <-m.stopChan:
							return
						case <-transportStopChan:
							return
						}
					}
				case err := <-transport.Errors():
					// Use backpressure handler to send error
					if m.backpressureHandler != nil {
						m.backpressureHandler.SendError(err)
					} else {
						select {
						case m.errorChan <- err:
						case <-m.stopChan:
							return
						case <-transportStopChan:
							return
						}
					}
				case <-m.stopChan:
					return
				case <-transportStopChan:
					return
				}
			} else {
				// Wait for transport to be ready
				select {
				case <-m.transportReady:
					// Transport is ready, continue to process events
				case <-m.stopChan:
					return
				case <-transportStopChan:
					return
				}
			}
		}
	}
}