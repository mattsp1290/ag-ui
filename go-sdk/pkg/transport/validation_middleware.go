package transport

import (
	"context"
	"fmt"
	"sync"
	"time"
	
	"github.com/ag-ui/go-sdk/pkg/core/events"
)

// ValidationMiddleware implements middleware for transport validation
type ValidationMiddleware struct {
	validator     Validator
	config        *ValidationConfig
	metrics       *ValidationMetrics
	logger        Logger
	enabled       bool
	mu            sync.RWMutex
}

// ValidationMetrics tracks validation performance metrics
type ValidationMetrics struct {
	mu                      sync.RWMutex
	TotalValidations        uint64
	SuccessfulValidations   uint64
	FailedValidations       uint64
	ValidationErrors        uint64
	IncomingValidations     uint64
	OutgoingValidations     uint64
	AverageValidationTime   time.Duration
	MaxValidationTime       time.Duration
	ValidationTimeTotal     time.Duration
	ValidationsByType       map[string]uint64
	ValidationsByRule       map[string]uint64
	LastValidationTime      time.Time
	LastValidationError     error
	LastValidationErrorTime time.Time
}

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware(config ...*ValidationConfig) Middleware {
	var cfg *ValidationConfig
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	} else {
		cfg = DefaultValidationConfig()
	}
	
	return &ValidationMiddleware{
		validator: NewValidator(cfg),
		config:    cfg,
		metrics: &ValidationMetrics{
			ValidationsByType: make(map[string]uint64),
			ValidationsByRule: make(map[string]uint64),
		},
		logger:  NewNoopLogger(),
		enabled: cfg.Enabled,
	}
}

// NewValidationMiddlewareWithLogger creates a new validation middleware with a logger
func NewValidationMiddlewareWithLogger(config *ValidationConfig, logger Logger) *ValidationMiddleware {
	vm := NewValidationMiddleware(config).(*ValidationMiddleware)
	if logger != nil {
		vm.logger = logger
	}
	return vm
}

// ProcessOutgoing processes outgoing events before they are sent
func (m *ValidationMiddleware) ProcessOutgoing(ctx context.Context, event TransportEvent) (TransportEvent, error) {
	if err := m.validateEvent(ctx, event, "outgoing"); err != nil {
		return nil, err
	}
	return event, nil
}

// ProcessIncoming processes incoming events before they are delivered
func (m *ValidationMiddleware) ProcessIncoming(ctx context.Context, event events.Event) (events.Event, error) {
	// Convert to TransportEvent for validation
	transportEvent := &SimpleTransportEvent{
		EventID:        fmt.Sprintf("event-%d", time.Now().UnixNano()),
		EventType:      string(event.Type()),
		EventTimestamp: time.Now(),
		EventData:      make(map[string]interface{}),
	}
	
	if err := m.validateEvent(ctx, transportEvent, "incoming"); err != nil {
		return nil, err
	}
	return event, nil
}

// Name returns the middleware name
func (m *ValidationMiddleware) Name() string {
	return "ValidationMiddleware"
}

// Wrap implements the Middleware interface
func (m *ValidationMiddleware) Wrap(transport Transport) Transport {
	return &validatedTransport{
		Transport:  transport,
		middleware: m,
	}
}

// SetEnabled enables or disables validation
func (m *ValidationMiddleware) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
}

// IsEnabled returns whether validation is enabled
func (m *ValidationMiddleware) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// GetMetrics returns validation metrics
func (m *ValidationMiddleware) GetMetrics() ValidationMetrics {
	m.metrics.mu.RLock()
	defer m.metrics.mu.RUnlock()
	
	// Deep copy metrics
	metrics := *m.metrics
	metrics.ValidationsByType = make(map[string]uint64)
	metrics.ValidationsByRule = make(map[string]uint64)
	
	for k, v := range m.metrics.ValidationsByType {
		metrics.ValidationsByType[k] = v
	}
	
	for k, v := range m.metrics.ValidationsByRule {
		metrics.ValidationsByRule[k] = v
	}
	
	return metrics
}

// ResetMetrics resets all validation metrics
func (m *ValidationMiddleware) ResetMetrics() {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	
	m.metrics.TotalValidations = 0
	m.metrics.SuccessfulValidations = 0
	m.metrics.FailedValidations = 0
	m.metrics.ValidationErrors = 0
	m.metrics.IncomingValidations = 0
	m.metrics.OutgoingValidations = 0
	m.metrics.AverageValidationTime = 0
	m.metrics.MaxValidationTime = 0
	m.metrics.ValidationTimeTotal = 0
	m.metrics.ValidationsByType = make(map[string]uint64)
	m.metrics.ValidationsByRule = make(map[string]uint64)
	m.metrics.LastValidationTime = time.Time{}
	m.metrics.LastValidationError = nil
	m.metrics.LastValidationErrorTime = time.Time{}
}

// UpdateConfig updates the validation configuration
func (m *ValidationMiddleware) UpdateConfig(config *ValidationConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.config = config
	m.validator = NewValidator(config)
	m.enabled = config.Enabled
}

// validateEvent validates an event with proper context timeout handling and updates metrics
func (m *ValidationMiddleware) validateEvent(ctx context.Context, event TransportEvent, direction string) error {
	if !m.IsEnabled() {
		return nil
	}
	
	// Create a timeout context for validation if none exists
	validationCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		m.updateMetrics(event, direction, duration, nil)
	}()
	
	// Check if context was cancelled before validation
	select {
	case <-validationCtx.Done():
		return fmt.Errorf("validation cancelled: %w", validationCtx.Err())
	default:
	}
	
	var err error
	switch direction {
	case "incoming":
		err = m.validator.ValidateIncoming(validationCtx, event)
	case "outgoing":
		err = m.validator.ValidateOutgoing(validationCtx, event)
	default:
		err = m.validator.Validate(validationCtx, event)
	}
	
	if err != nil {
		m.updateMetrics(event, direction, time.Since(start), err)
		m.logger.Warn("Event validation failed", 
			String("direction", direction),
			String("event_id", event.ID()),
			String("event_type", event.Type()),
			Error(err))
		return err
	}
	
	m.logger.Debug("Event validation successful", 
		String("direction", direction),
		String("event_id", event.ID()),
		String("event_type", event.Type()))
	
	return nil
}

// updateMetrics updates validation metrics
func (m *ValidationMiddleware) updateMetrics(event TransportEvent, direction string, duration time.Duration, err error) {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	
	m.metrics.TotalValidations++
	m.metrics.LastValidationTime = time.Now()
	
	if direction == "incoming" {
		m.metrics.IncomingValidations++
	} else if direction == "outgoing" {
		m.metrics.OutgoingValidations++
	}
	
	eventType := event.Type()
	m.metrics.ValidationsByType[eventType]++
	
	if err != nil {
		m.metrics.FailedValidations++
		m.metrics.ValidationErrors++
		m.metrics.LastValidationError = err
		m.metrics.LastValidationErrorTime = time.Now()
		
		// Track validation rule errors
		if ve, ok := err.(*ValidationError); ok {
			for _, e := range ve.Errors() {
				if te, ok := e.(*TransportError); ok {
					m.metrics.ValidationsByRule[te.Op]++
				}
			}
		}
	} else {
		m.metrics.SuccessfulValidations++
	}
	
	// Update timing metrics
	m.metrics.ValidationTimeTotal += duration
	m.metrics.AverageValidationTime = m.metrics.ValidationTimeTotal / time.Duration(m.metrics.TotalValidations)
	
	if duration > m.metrics.MaxValidationTime {
		m.metrics.MaxValidationTime = duration
	}
}

// validatedTransport wraps a transport with validation
type validatedTransport struct {
	Transport
	middleware *ValidationMiddleware
}

// Send validates outgoing events before sending
func (t *validatedTransport) Send(ctx context.Context, event TransportEvent) error {
	if err := t.middleware.validateEvent(ctx, event, "outgoing"); err != nil {
		return err
	}
	
	return t.Transport.Send(ctx, event)
}

// Channels returns channels that validate incoming events and errors
func (t *validatedTransport) Channels() (<-chan events.Event, <-chan error) {
	originalEventChan, originalErrorChan := t.Transport.Channels()
	validatedEventChan := make(chan events.Event, 100) // Buffer for validation processing
	validatedErrorChan := make(chan error, 100) // Buffer for validation processing
	
	go func() {
		defer close(validatedEventChan)
		defer close(validatedErrorChan)
		
		for {
			select {
			case event, ok := <-originalEventChan:
				if !ok {
					return
				}
				// Validate event using events.Event interface
				if err := event.Validate(); err != nil {
					t.middleware.logger.Warn("Event validation failed with built-in validator", 
						String("event_type", string(event.Type())),
						Err(err))
					// Continue processing - middleware should not block pipeline
					// Invalid events are still passed through but logged
				} else {
					// Additionally, use the events package validator for comprehensive validation
					ctx := context.Background()
					if err := events.ValidateEventWithContext(ctx, event); err != nil {
						t.middleware.logger.Warn("Event validation failed with events package validator", 
							String("event_type", string(event.Type())),
							Err(err))
						// Continue processing - log error but don't block pipeline
					} else {
						t.middleware.logger.Debug("Event validation passed", 
							String("event_type", string(event.Type())))
					}
				}
				
				// Send the event directly without validation
				validatedEventChan <- event
			case err, ok := <-originalErrorChan:
				if !ok {
					return
				}
				// Forward errors without modification
				validatedErrorChan <- err
			}
		}
	}()
	
	return validatedEventChan, validatedErrorChan
}

// ValidationTransport provides a transport wrapper focused on validation
type ValidationTransport struct {
	Transport
	validator Validator
	config    *ValidationConfig
	metrics   *ValidationMetrics
	logger    Logger
}

// NewValidationTransport creates a new validation transport wrapper
func NewValidationTransport(transport Transport, config *ValidationConfig) *ValidationTransport {
	if config == nil {
		config = DefaultValidationConfig()
	}
	
	return &ValidationTransport{
		Transport: transport,
		validator: NewValidator(config),
		config:    config,
		metrics: &ValidationMetrics{
			ValidationsByType: make(map[string]uint64),
			ValidationsByRule: make(map[string]uint64),
		},
		logger: NewNoopLogger(),
	}
}

// NewValidationTransportWithLogger creates a new validation transport with logger
func NewValidationTransportWithLogger(transport Transport, config *ValidationConfig, logger Logger) *ValidationTransport {
	vt := NewValidationTransport(transport, config)
	if logger != nil {
		vt.logger = logger
	}
	return vt
}

// Send validates and sends an event
func (vt *ValidationTransport) Send(ctx context.Context, event TransportEvent) error {
	if vt.config.Enabled && !vt.config.SkipValidationOnOutgoing {
		if err := vt.validator.ValidateOutgoing(ctx, event); err != nil {
			vt.logger.Error("Outgoing event validation failed", 
				String("event_id", event.ID()),
				String("event_type", event.Type()),
				Error(err))
			return err
		}
	}
	
	return vt.Transport.Send(ctx, event)
}

// Channels returns validated events and errors
func (vt *ValidationTransport) Channels() (<-chan events.Event, <-chan error) {
	originalEventChan, originalErrorChan := vt.Transport.Channels()
	validatedEventChan := make(chan events.Event, 100)
	validatedErrorChan := make(chan error, 100)
	
	go func() {
		defer close(validatedEventChan)
		defer close(validatedErrorChan)
		
		for {
			select {
			case event, ok := <-originalEventChan:
				if !ok {
					return
				}
				if vt.config.Enabled && !vt.config.SkipValidationOnIncoming {
					// Validate event using events.Event interface
					if err := event.Validate(); err != nil {
						vt.logger.Warn("Event validation failed with built-in validator", 
							String("event_type", string(event.Type())),
							Err(err))
						// Continue processing - middleware should not block pipeline
						// Invalid events are still passed through but logged
					} else {
						// Additionally, use the events package validator for comprehensive validation
						ctx := context.Background()
						if err := events.ValidateEventWithContext(ctx, event); err != nil {
							vt.logger.Warn("Event validation failed with events package validator", 
								String("event_type", string(event.Type())),
								Err(err))
							// Continue processing - log error but don't block pipeline
						} else {
							vt.logger.Debug("Event validation passed", 
								String("event_type", string(event.Type())))
						}
					}
				}
				
				validatedEventChan <- event
			case err, ok := <-originalErrorChan:
				if !ok {
					return
				}
				validatedErrorChan <- err
			}
		}
	}()
	
	return validatedEventChan, validatedErrorChan
}

// GetValidationMetrics returns validation metrics
func (vt *ValidationTransport) GetValidationMetrics() ValidationMetrics {
	vt.metrics.mu.RLock()
	defer vt.metrics.mu.RUnlock()
	
	// Deep copy metrics
	metrics := *vt.metrics
	metrics.ValidationsByType = make(map[string]uint64)
	metrics.ValidationsByRule = make(map[string]uint64)
	
	for k, v := range vt.metrics.ValidationsByType {
		metrics.ValidationsByType[k] = v
	}
	
	for k, v := range vt.metrics.ValidationsByRule {
		metrics.ValidationsByRule[k] = v
	}
	
	return metrics
}

// UpdateValidationConfig updates the validation configuration
func (vt *ValidationTransport) UpdateValidationConfig(config *ValidationConfig) {
	vt.config = config
	vt.validator = NewValidator(config)
}