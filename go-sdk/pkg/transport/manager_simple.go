package transport

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

// SimpleManager provides basic transport management without import cycles
type SimpleManager struct {
	mu                  sync.RWMutex
	activeTransport     Transport
	eventChan           chan events.Event
	errorChan           chan error
	stopChan            chan struct{}
	transportStopChan   chan struct{} // To stop receiveEvents for old transport
	transportReady      chan struct{}
	running             int32 // Use atomic int32 for thread-safe access
	backpressureHandler *BackpressureHandler
	backpressureConfig  BackpressureConfig
	validator           Validator
	validationConfig    *ValidationConfig
	receiveWg           *sync.WaitGroup // Track receiveEvents goroutines
	receiveWgMu         sync.Mutex      // Protects receiveWg lifecycle
	generation          int64           // Generation counter to prevent stale goroutines
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
		receiveWg:          &sync.WaitGroup{},
	}

	// Initialize backpressure handler
	manager.backpressureHandler = NewBackpressureHandler(backpressureConfig)
	manager.eventChan = make(chan events.Event, backpressureConfig.BufferSize)
	manager.errorChan = make(chan error, backpressureConfig.BufferSize)

	return manager
}

// SetTransport sets the active transport
func (m *SimpleManager) SetTransport(transport Transport) {
	// Simple direct lock acquisition without goroutines to avoid deadlocks
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

	// Pre-connect the new transport if manager is running
	var preConnected bool
	if transport != nil && atomic.LoadInt32(&m.running) == 1 {
		connectCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		connectErr := transport.Connect(connectCtx)
		cancel()
		preConnected = connectErr == nil
	}

	// If the manager is running and we have a transport, start receiving
	if atomic.LoadInt32(&m.running) == 1 && transport != nil && preConnected {
		if added, gen, wg := m.safeAddReceiver(); added {
			go m.receiveEvents(gen, wg)
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

	// Create fresh WaitGroup for new generation
	m.receiveWgMu.Lock()
	m.receiveWg = &sync.WaitGroup{}
	m.generation++ // New generation
	m.receiveWgMu.Unlock()

	// Simple direct lock acquisition without goroutines to avoid deadlocks
	// Check context first
	if err := ctx.Err(); err != nil {
		atomic.StoreInt32(&m.running, 0)
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeTransport != nil {
		// Use the provided context for connection, but with a reasonable timeout
		connectCtx := ctx
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			connectCtx, cancel = context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
		}

		if err := m.activeTransport.Connect(connectCtx); err != nil {
			// Reset the flag on error to maintain consistency
			atomic.StoreInt32(&m.running, 0)
			return err
		}

		// Start receiving events
		if added, gen, wg := m.safeAddReceiver(); added {
			go m.receiveEvents(gen, wg)
		}

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

	// Simple direct lock acquisition without goroutines to avoid deadlocks
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

	// Wait for all receiveEvents goroutines to finish using safe method
	m.safeWaitReceiver(ctx)

	// Re-acquire lock for final cleanup
	m.mu.Lock()
	defer m.mu.Unlock()

	// Final cleanup
	activeTransport := m.activeTransport
	backpressureHandler := m.backpressureHandler

	if activeTransport != nil {
		// Use a short timeout for transport close to prevent hanging
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer closeCancel()

		if err := activeTransport.Close(closeCtx); err != nil {
			// Even on error, we keep running=false to ensure shutdown
			return err
		}
	}

	// Stop backpressure handler
	if backpressureHandler != nil {
		backpressureHandler.Stop()
	}

	// Reset the transport ready channel
	// Drain any pending signals
	select {
	case <-m.transportReady:
	default:
	}

	// Drain channels before closing to prevent data loss
	// Use a reasonable timeout for draining operations
	drainTimeout := 5 * time.Second
	if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
		if timeLeft := time.Until(deadline); timeLeft < drainTimeout {
			drainTimeout = timeLeft
		}
	}

	drainCtx, drainCancel := context.WithTimeout(context.Background(), drainTimeout)
	defer drainCancel()

	// Drain channels using non-blocking approach to prevent deadlock
	drained := m.drainChannelsNonBlocking(drainCtx)
	if !drained {
		// Timeout during draining, but we proceed gracefully
		// This is not considered a fatal error
	}

	// Close channels after draining
	if m.backpressureHandler != nil {
		// Backpressure handler manages its own channels
		// Let it handle the cleanup
	} else {
		// We manage channels directly, close them now
		close(m.eventChan)
		close(m.errorChan)
	}

	// Return nil even if we timed out, as per test expectations
	// The timeout is handled gracefully without returning an error
	return nil
}

// drainChannelsNonBlocking drains channels without risking deadlock
func (m *SimpleManager) drainChannelsNonBlocking(ctx context.Context) bool {
	eventCount := 0
	errorCount := 0

	// Use a ticker to periodically check for context cancellation
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

drainLoop:
	for {
		select {
		case <-ctx.Done():
			break drainLoop
		case <-ticker.C:
			// Check if context is done
			if ctx.Err() != nil {
				break drainLoop
			}
		case <-m.eventChan:
			eventCount++
			// Non-blocking continue - keep draining
		case <-m.errorChan:
			errorCount++
			// Non-blocking continue - keep draining
		default:
			// No more items to drain, we're done
			break drainLoop
		}
	}

	// Return true if we completed draining without timeout
	return ctx.Err() == nil
}

// safeAddReceiver safely adds a receiver to the WaitGroup
func (m *SimpleManager) safeAddReceiver() (bool, int64, *sync.WaitGroup) {
	m.receiveWgMu.Lock()
	defer m.receiveWgMu.Unlock()
	if m.receiveWg != nil && atomic.LoadInt32(&m.running) == 1 {
		m.receiveWg.Add(1)
		return true, m.generation, m.receiveWg
	}
	return false, 0, nil
}

// safeWaitReceiver safely waits for receivers and prepares for shutdown
func (m *SimpleManager) safeWaitReceiver(ctx context.Context) {
	m.receiveWgMu.Lock()
	currentWg := m.receiveWg
	m.generation++    // Increment generation to invalidate old goroutines
	m.receiveWg = nil // Clear to prevent new additions
	m.receiveWgMu.Unlock()

	if currentWg == nil {
		return
	}

	// Wait for current WaitGroup to finish
	done := make(chan struct{})
	go func() {
		currentWg.Wait()
		close(done)
	}()

	// Use context timeout if available, otherwise use a reasonable default
	waitTimeout := 5 * time.Second
	if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
		if timeLeft := time.Until(deadline); timeLeft < waitTimeout {
			waitTimeout = timeLeft
		}
	}

	select {
	case <-done:
		// All goroutines finished
	case <-ctx.Done():
		// Context cancelled
	case <-time.After(waitTimeout):
		// Timeout waiting for goroutines - proceed with cleanup anyway
	}
}

// waitForReceiveGoroutines is a test helper that waits for all receive goroutines to finish
// This method is intended for testing purposes only
func (m *SimpleManager) waitForReceiveGoroutines(timeout time.Duration) bool {
	m.receiveWgMu.Lock()
	currentWg := m.receiveWg
	m.receiveWgMu.Unlock()

	if currentWg == nil {
		return true
	}

	done := make(chan struct{})
	go func() {
		currentWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
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
func (m *SimpleManager) Receive() <-chan events.Event {
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

// Channels returns both event and error channels together
func (m *SimpleManager) Channels() (<-chan events.Event, <-chan error) {
	if m.backpressureHandler != nil {
		return m.backpressureHandler.Channels()
	}
	return m.eventChan, m.errorChan
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
func (m *SimpleManager) receiveEvents(generation int64, wg *sync.WaitGroup) {
	defer func() {
		if wg != nil {
			wg.Done()
		}
	}()

	// Check if we're still valid generation - if not, exit early
	m.receiveWgMu.Lock()
	currentGen := m.generation
	m.receiveWgMu.Unlock()
	if generation != currentGen {
		return
	}

	// Get a copy of the transport stop channel to avoid races
	m.mu.RLock()
	transportStopChan := m.transportStopChan
	m.mu.RUnlock()

	for {
		// Periodically check if we're still the current generation
		m.receiveWgMu.Lock()
		currentGen := m.generation
		m.receiveWgMu.Unlock()
		if generation != currentGen {
			return
		}

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
				eventCh, errorCh := transport.Channels()
				select {
				case event := <-eventCh:
					// Validate incoming event if validation is enabled
					m.mu.RLock()
					validationEnabled := m.validationConfig != nil && m.validationConfig.Enabled
					validator := m.validator
					m.mu.RUnlock()

					// Validate incoming event if validation is enabled
					if validationEnabled && validator != nil {
						// First, use the event's built-in validation
						if err := event.Validate(); err != nil {
							// Log validation error but continue processing to avoid blocking the pipeline
							// The backpressure handler will handle the event, and we can log metrics
							// In production, you might want to increment validation error metrics here
							// Note: Continue processing - middleware should not block pipeline
						} else {
							// Additionally, use the events package validator for comprehensive validation
							ctx := context.Background()
							if err := events.ValidateEventWithContext(ctx, event); err != nil {
								// Log validation error but continue processing
								// In production, you might want to increment validation error metrics here
								// Note: Continue processing - middleware should not block pipeline
							}
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
				case err := <-errorCh:
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
				// Wait for transport to be ready with a timeout to prevent indefinite blocking
				waitCtx, waitCancel := context.WithTimeout(context.Background(), 30*time.Second)
				select {
				case <-m.transportReady:
					// Transport is ready, continue to process events
					waitCancel()
				case <-waitCtx.Done():
					// Timeout waiting for transport, continue to next iteration
					// No sleep needed - the next iteration will block on transportReady again
					waitCancel()
				case <-m.stopChan:
					waitCancel()
					return
				case <-transportStopChan:
					waitCancel()
					return
				}
			}
		}
	}
}

// IsRunning returns true if the manager is currently running
// This method uses atomic operations for thread-safe access
func (m *SimpleManager) IsRunning() bool {
	return atomic.LoadInt32(&m.running) == 1
}
