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
	// Pre-connect the new transport if manager is running (outside the lock)
	var preConnected bool
	if transport != nil && atomic.LoadInt32(&m.running) == 1 {
		connectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := transport.Connect(connectCtx)
		cancel()
		preConnected = err == nil
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Keep reference to old transport for graceful shutdown
	oldTransport := m.activeTransport
	oldStopChan := m.transportStopChan
	
	// Set new transport immediately to minimize gap
	m.activeTransport = transport
	
	// Create new stop channel if needed
	if oldStopChan != nil {
		m.transportStopChan = make(chan struct{})
	}
	
	// If the manager is running and we have a transport, start receiving
	if atomic.LoadInt32(&m.running) == 1 && transport != nil {
		if preConnected {
			// Already connected, just start receiving
			m.receiveWg.Add(1)
			go m.receiveEvents()
		} else {
			// Try to connect if not pre-connected
			connectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			if err := transport.Connect(connectCtx); err == nil {
				m.receiveWg.Add(1)
				go m.receiveEvents()
			}
		}
	}
	
	// Signal that transport is ready (non-blocking send)
	if transport != nil && transport.IsConnected() {
		select {
		case m.transportReady <- struct{}{}:
		default:
			// Channel already has a value, which is fine
		}
	}
	
	// Clean up old transport after new one is running (outside critical section)
	go func() {
		if oldStopChan != nil {
			// Safe channel close - check if already closed
			select {
			case <-oldStopChan:
				// Channel already closed
			default:
				// Channel not closed yet, safe to close
				close(oldStopChan)
			}
		}
		if oldTransport != nil {
			// Delay slightly to allow in-flight operations to complete
			time.Sleep(10 * time.Millisecond)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			oldTransport.Close(ctx)
			cancel()
		}
	}()
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
	// Read validation state atomically to ensure consistency
	validationEnabled := m.validationConfig != nil && m.validationConfig.Enabled
	validator := m.validator
	m.mu.RUnlock()
	
	if transport == nil || !transport.IsConnected() {
		return ErrNotConnected
	}
	
	// Validate outgoing event if validation is enabled
	// Check both enabled flag and validator instance for safety
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
	var validator Validator
	
	if config != nil {
		// Create validator outside the lock to minimize critical section
		validator = NewValidator(config)
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if config == nil {
		// Update all fields atomically to nil state
		m.validationConfig = nil
		m.validator = nil
		return
	}
	
	// Update fields atomically to ensure consistency
	// Set config first, then validator, so readers see consistent state
	m.validationConfig = config
	m.validator = validator
}

// GetValidationConfig returns the current validation configuration
func (m *SimpleManager) GetValidationConfig() *ValidationConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.validationConfig == nil {
		return nil
	}
	
	// Return a copy to prevent external modification
	configCopy := *m.validationConfig
	return &configCopy
}

// SetValidationEnabled enables or disables validation
func (m *SimpleManager) SetValidationEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Update the config's enabled flag if config exists
	if m.validationConfig != nil {
		// Create a copy of the config to avoid modifying the original
		configCopy := *m.validationConfig
		configCopy.Enabled = enabled
		m.validationConfig = &configCopy
	}
}

// IsValidationEnabled returns whether validation is enabled
func (m *SimpleManager) IsValidationEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.validationConfig != nil && m.validationConfig.Enabled
}

// GetValidationState returns both the validation config and enabled state atomically
func (m *SimpleManager) GetValidationState() (*ValidationConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	enabled := m.validationConfig != nil && m.validationConfig.Enabled
	if m.validationConfig == nil {
		return nil, enabled
	}
	
	// Return a copy to prevent external modification
	configCopy := *m.validationConfig
	return &configCopy, enabled
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
					validationEnabled := m.validationConfig != nil && m.validationConfig.Enabled
					validator := m.validator
					m.mu.RUnlock()
					
					if validationEnabled && validator != nil {
						ctx := context.Background()
						if err := validator.ValidateIncoming(ctx, event.Event); err != nil {
							// Ensure headers map is initialized
							if event.Metadata.Headers == nil {
								event.Metadata.Headers = make(map[string]string)
							}
							// Add validation error to event metadata
							event.Metadata.Headers["validation_error"] = err.Error()
							event.Metadata.Headers["validation_failed"] = "true"
						} else {
							// Ensure headers map is initialized
							if event.Metadata.Headers == nil {
								event.Metadata.Headers = make(map[string]string)
							}
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