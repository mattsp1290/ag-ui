package transport

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	
	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// ManagerConfig represents simplified transport configuration
type ManagerConfig struct {
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
	config              *ManagerConfig
	activeTransport     Transport
	fallbackQueue       []string
	middleware          []Middleware
	eventChan           chan events.Event
	errorChan           chan error
	stopChan            chan struct{}
	ctx                 context.Context    // Manager lifecycle context
	cancel              context.CancelFunc // Cancel function for lifecycle context
	running             int32 // Use atomic int32 for thread-safe access
	receiverActive      int32 // Use atomic int32 to track active receiveEvents goroutine
	startStopMu         sync.Mutex // Serialize Start/Stop operations
	metrics             *ManagerMetrics
	logger              Logger
	backpressureHandler *BackpressureHandler
	validator           Validator
	receiveWg           sync.WaitGroup // Track receiveEvents goroutines
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
func NewManager(cfg *ManagerConfig) *Manager {
	if cfg == nil {
		cfg = &ManagerConfig{
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
	
	// Validate and sanitize configuration
	if cfg.BufferSize < 0 {
		cfg.BufferSize = 1024 // Default to reasonable buffer size
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 1 // Minimum buffer size
	}
	
	// Validate backpressure configuration
	if cfg.Backpressure.BufferSize <= 0 {
		cfg.Backpressure.BufferSize = cfg.BufferSize
	}
	if cfg.Backpressure.Strategy == "" {
		cfg.Backpressure.Strategy = BackpressureNone
	}
	if cfg.Backpressure.HighWaterMark > 1.0 {
		cfg.Backpressure.HighWaterMark = 0.8
	}
	if cfg.Backpressure.LowWaterMark < 0 {
		cfg.Backpressure.LowWaterMark = 0.2
	}
	if cfg.Backpressure.BlockTimeout < 0 {
		cfg.Backpressure.BlockTimeout = 5 * time.Second
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	manager := &Manager{
		config:        cfg,
		middleware:    []Middleware{},
		eventChan:     make(chan events.Event, cfg.BufferSize),
		errorChan:     make(chan error, cfg.BufferSize),
		stopChan:      make(chan struct{}),
		ctx:           ctx,
		cancel:        cancel,
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
	}

	return manager
}

// NewManagerWithLogger creates a new transport manager with a custom logger
func NewManagerWithLogger(cfg *ManagerConfig, logger Logger) *Manager {
	manager := NewManager(cfg)
	if logger != nil {
		manager.logger = logger
	}
	return manager
}

// Start starts the transport manager
func (m *Manager) Start(ctx context.Context) error {
	// Serialize Start/Stop operations to prevent WaitGroup reuse issues
	m.startStopMu.Lock()
	defer m.startStopMu.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("Starting transport manager", 
		String("operation", "start"))

	if !atomic.CompareAndSwapInt32(&m.running, 0, 1) {
		m.logger.Debug("Manager already running", 
			String("operation", "start"))
		return fmt.Errorf("transport manager already running")
	}
	
	// Cancel any existing context and wait for goroutines to finish
	if m.cancel != nil {
		m.cancel()
	}
	
	// Wait for any existing receiveEvents goroutines to finish
	m.mu.Unlock() // Release lock while waiting
	m.receiveWg.Wait()
	m.mu.Lock() // Re-acquire lock
	
	// Create fresh context and stop channel for this start cycle
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.stopChan = make(chan struct{})
	
	// Start receiveEvents goroutine if we have an active transport but no receiver is running
	if m.activeTransport != nil && atomic.CompareAndSwapInt32(&m.receiverActive, 0, 1) {
		m.logger.Debug("Starting receiveEvents goroutine for existing transport", 
			String("operation", "start"))
		
		m.receiveWg.Add(1)
		go m.receiveEvents(m.activeTransport)
	}
	
	m.logger.Info("Transport manager started successfully", 
		String("operation", "start"))
	
	return nil
}

// Stop stops the transport manager
func (m *Manager) Stop(ctx context.Context) error {
	// Serialize Start/Stop operations to prevent WaitGroup reuse issues
	m.startStopMu.Lock()
	defer m.startStopMu.Unlock()

	m.logger.Info("Stopping transport manager", 
		String("operation", "stop"))

	if !atomic.CompareAndSwapInt32(&m.running, 1, 0) {
		m.logger.Debug("Manager already stopped", 
			String("operation", "stop"))
		return nil
	}

	// Signal stop first to unblock receiveEvents goroutine
	// Hold lock briefly to safely close stopChan and cancel context
	m.mu.Lock()
	select {
	case <-m.stopChan:
		// Already closed
	default:
		close(m.stopChan)
	}
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Unlock()

	// Wait for receiveEvents goroutine to finish before acquiring lock for cleanup
	// Use a timeout to prevent hanging
	receiveWgDone := make(chan struct{})
	go func() {
		m.receiveWg.Wait()
		close(receiveWgDone)
	}()
	
	// Use context timeout if available, otherwise use a reasonable default
	waitTimeout := 5 * time.Second
	if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
		if timeLeft := time.Until(deadline); timeLeft < waitTimeout && timeLeft > 0 {
			waitTimeout = timeLeft
		}
	}
	
	select {
	case <-receiveWgDone:
		m.logger.Debug("receiveEvents goroutines finished successfully", 
			String("operation", "stop"))
	case <-time.After(waitTimeout):
		m.logger.Warn("Timeout waiting for receiveEvents goroutines to finish, proceeding with cleanup", 
			String("operation", "stop"))
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Drain event channels with timeout - use shorter timeout and non-blocking approach
	drainCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	
	m.logger.Debug("Draining event channels", 
		String("operation", "stop"),
		Duration("timeout", 1*time.Second))

	// Use a simpler non-blocking drain approach to prevent deadlocks
	drained := m.drainChannelsNonBlocking(drainCtx)
	if drained {
		m.logger.Debug("Event channels drained successfully", 
			String("operation", "stop"))
	} else {
		m.logger.Warn("Event channel draining timed out, proceeding with cleanup", 
			String("operation", "stop"))
	}

	// Close active transport
	if m.activeTransport != nil {
		if err := m.activeTransport.Close(ctx); err != nil {
			// Check if this is a timeout error - if so, log but don't return error
			if ctx.Err() == context.DeadlineExceeded {
				m.logger.Warn("Transport close timed out, but continuing cleanup", 
					String("operation", "stop"),
					Err(err))
			} else {
				m.logger.Error("Failed to close active transport", 
					String("operation", "stop"),
					Err(err))
				return fmt.Errorf("failed to close active transport: %w", err)
			}
		} else {
			m.logger.Debug("Active transport closed successfully", 
				String("operation", "stop"))
		}
	}

	// Stop backpressure handler
	if m.backpressureHandler != nil {
		m.backpressureHandler.Stop()
		m.logger.Debug("Backpressure handler stopped", 
			String("operation", "stop"))
	}

	m.logger.Info("Transport manager stopped successfully", 
		String("operation", "stop"))
	
	return nil
}

// drainChannelsNonBlocking drains channels without risking deadlock
func (m *Manager) drainChannelsNonBlocking(ctx context.Context) bool {
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
			// Non-blocking continue
		case <-m.errorChan:
			errorCount++
			// Non-blocking continue
		default:
			// No more items to drain, we're done
			break drainLoop
		}
	}
	
	if eventCount > 0 || errorCount > 0 {
		m.logger.Debug("Channel drain completed",
			String("operation", "stop"),
			Int("events_drained", eventCount),
			Int("errors_drained", errorCount))
	}
	
	return ctx.Err() == nil
}

// Send sends an event through the active transport
func (m *Manager) Send(ctx context.Context, event TransportEvent) error {
	m.mu.RLock()
	transport := m.activeTransport
	validationEnabled := m.config.Validation != nil && m.config.Validation.Enabled
	validator := m.validator
	m.mu.RUnlock()

	// Validate outgoing event if validation is enabled (do this before transport check)
	if validationEnabled && validator != nil {
		if err := validator.ValidateOutgoing(ctx, event); err != nil {
			m.logger.Error("Event validation failed", 
				String("operation", "send"),
				String("event_id", event.ID()),
				String("event_type", event.Type()),
				Err(err))
			
			// Map validation errors to transport errors
			if IsValidationError(err) {
				errMsg := err.Error()
				if strings.Contains(errMsg, "event type") && strings.Contains(errMsg, "not in allowed types") {
					return ErrInvalidEventType
				}
				if strings.Contains(errMsg, "missing required fields") {
					return ErrMissingRequiredFields
				}
				if strings.Contains(errMsg, "message size") && strings.Contains(errMsg, "exceeds") {
					return ErrInvalidMessageSize
				}
				if strings.Contains(errMsg, "data format") {
					return ErrInvalidDataFormat
				}
				if strings.Contains(errMsg, "field") && strings.Contains(errMsg, "validation failed") {
					return ErrFieldValidationFailed
				}
				if strings.Contains(errMsg, "pattern") {
					return ErrPatternValidationFailed
				}
				// Default validation error
				return ErrValidationFailed
			}
			
			return err
		}
	}

	if transport == nil {
		m.logger.Error("Cannot send event: no active transport", 
			String("operation", "send"),
			String("event_id", event.ID()),
			String("event_type", event.Type()))
		return ErrNotConnected
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
			Err(err))
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
func (m *Manager) Receive() <-chan events.Event {
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

// Channels returns both event and error channels together
func (m *Manager) Channels() (<-chan events.Event, <-chan error) {
	if m.backpressureHandler != nil {
		return m.backpressureHandler.Channels()
	}
	return m.eventChan, m.errorChan
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
	
	// Only start receiveEvents goroutine if:
	// 1. We have a valid transport
	// 2. Manager is running
	// 3. No receiveEvents goroutine is currently active
	if transport != nil && atomic.LoadInt32(&m.running) == 1 {
		// Use atomic CAS to ensure only one receiveEvents goroutine is started
		if atomic.CompareAndSwapInt32(&m.receiverActive, 0, 1) {
			m.logger.Debug("Starting new receiveEvents goroutine", 
				String("operation", "set_transport"))
			
			m.receiveWg.Add(1)
			go m.receiveEvents(transport)
		} else {
			m.logger.Debug("receiveEvents goroutine already active, skipping start", 
				String("operation", "set_transport"))
		}
	} else if transport != nil {
		m.logger.Debug("Manager not running, will start receiveEvents on Start()", 
			String("operation", "set_transport"))
	}
}

// receiveEvents receives events from a transport
func (m *Manager) receiveEvents(transport Transport) {
	defer m.receiveWg.Done()
	defer atomic.StoreInt32(&m.receiverActive, 0) // Reset receiver active flag
	
	m.logger.Debug("Starting event receiver for transport", 
		String("operation", "receive_events"))
	
	defer m.logger.Debug("Event receiver stopped for transport", 
		String("operation", "receive_events"))

	eventCh, errorCh := transport.Channels()
	for {
		select {
		case event := <-eventCh:
			m.logger.Debug("Received event from transport", 
				String("operation", "receive_events"),
				String("event_type", string(event.Type())))
			
			// Validate incoming event if validation is enabled
			m.mu.RLock()
			validationEnabled := m.config.Validation != nil && m.config.Validation.Enabled
			validator := m.validator
			m.mu.RUnlock()
			
			// Validate incoming event if validation is enabled
			if validationEnabled && validator != nil {
				// First, use the event's built-in validation
				if err := event.Validate(); err != nil {
					m.logger.Warn("Event validation failed with built-in validator", 
						String("operation", "receive_events"),
						String("event_type", string(event.Type())),
						Err(err))
					// Continue processing - log error but don't block pipeline
					// In production, you might want to increment validation error metrics here
				} else {
					// Additionally, use the events package validator for comprehensive validation
					ctx := context.Background()
					if err := events.ValidateEventWithContext(ctx, event); err != nil {
						m.logger.Warn("Event validation failed with events package validator", 
							String("operation", "receive_events"),
							String("event_type", string(event.Type())),
							Err(err))
						// Continue processing - log error but don't block pipeline
						// In production, you might want to increment validation error metrics here
					} else {
						m.logger.Debug("Event validation passed", 
							String("operation", "receive_events"),
							String("event_type", string(event.Type())))
					}
				}
			}
			
			// Use backpressure handler to send event
			if err := m.backpressureHandler.SendEvent(event); err != nil {
				m.logger.Warn("Failed to send event due to backpressure", 
					String("operation", "receive_events"),
					String("event_type", string(event.Type())),
					Err(err))
			} else {
				m.logger.Debug("Event forwarded to event channel", 
					String("operation", "receive_events"),
					String("event_type", string(event.Type())))
			}
		case err := <-errorCh:
			m.logger.Error("Received error from transport", 
				String("operation", "receive_events"),
				Err(err))
			
			// Use backpressure handler to send error
			if sendErr := m.backpressureHandler.SendError(err); sendErr != nil {
				m.logger.Warn("Failed to send error due to backpressure", 
					String("operation", "receive_events"),
					Err(err),
					Any("send_error", sendErr))
			} else {
				m.logger.Debug("Error forwarded to error channel", 
					String("operation", "receive_events"))
			}
		case <-m.stopChan:
			m.logger.Debug("Stop signal received", 
				String("operation", "receive_events"))
			return
		case <-m.ctx.Done():
			m.logger.Debug("Manager context cancelled", 
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
	var validator Validator
	
	if config != nil {
		// Create validator outside the lock to minimize critical section
		validator = NewValidator(config)
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if config == nil {
		m.validator = nil
		m.config.Validation = nil
		m.logger.Debug("Validation configuration cleared", 
			String("operation", "set_validation_config"),
			Bool("enabled", false))
		return
	}
	
	// Update fields atomically to ensure consistency
	m.config.Validation = config
	m.validator = validator
	
	m.logger.Debug("Validation configuration updated", 
		String("operation", "set_validation_config"),
		Bool("enabled", config.Enabled))
}

// GetValidationConfig returns the current validation configuration
func (m *Manager) GetValidationConfig() *ValidationConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.config.Validation == nil {
		return nil
	}
	
	// Return a copy to prevent external modification
	configCopy := *m.config.Validation
	return &configCopy
}

// SetValidationEnabled enables or disables validation
func (m *Manager) SetValidationEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Update the config's enabled flag if config exists
	if m.config.Validation != nil {
		// Create a copy of the config to avoid modifying the original
		configCopy := *m.config.Validation
		configCopy.Enabled = enabled
		m.config.Validation = &configCopy
	}
	
	m.logger.Debug("Validation enabled/disabled", 
		String("operation", "set_validation_enabled"),
		Bool("enabled", enabled))
}

// IsValidationEnabled returns whether validation is enabled
func (m *Manager) IsValidationEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Validation != nil && m.config.Validation.Enabled
}