package errors

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	// StateClosed allows all requests through
	StateClosed CircuitBreakerState = iota
	// StateOpen blocks all requests
	StateOpen
	// StateHalfOpen allows limited requests to test if service recovered
	StateHalfOpen
)

// String returns the string representation of circuit breaker state
func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig contains configuration for a circuit breaker
type CircuitBreakerConfig struct {
	// MaxFailures is the number of failures that triggers the breaker to open
	MaxFailures uint64

	// ResetTimeout is how long to wait before attempting to reset from open to half-open
	ResetTimeout time.Duration

	// HalfOpenMaxCalls is the maximum number of calls allowed in half-open state
	HalfOpenMaxCalls uint64

	// SuccessThreshold is the number of consecutive successes required to close from half-open
	SuccessThreshold uint64

	// Timeout is the timeout for operations protected by the circuit breaker
	Timeout time.Duration

	// Name is a human-readable name for this circuit breaker
	Name string

	// ShouldTrip is a custom function to determine if the breaker should trip
	ShouldTrip func(counts Counts) bool
}

// DefaultCircuitBreakerConfig returns a default circuit breaker configuration
func DefaultCircuitBreakerConfig(name string) *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		MaxFailures:      5,
		ResetTimeout:     60 * time.Second,
		HalfOpenMaxCalls: 3,
		SuccessThreshold: 2,
		Timeout:          10 * time.Second,
		Name:             name,
		ShouldTrip:       nil, // Use default logic
	}
}

// Counts holds the statistics for a circuit breaker
type Counts struct {
	Requests             uint64
	TotalSuccesses       uint64
	TotalFailures        uint64
	ConsecutiveSuccesses uint64
	ConsecutiveFailures  uint64
}

// CircuitBreaker interface defines the contract for circuit breakers
type CircuitBreaker interface {
	// Execute runs the given function with circuit breaker protection
	Execute(ctx context.Context, operation func() error) error

	// Call runs the given function with circuit breaker protection and returns result
	Call(ctx context.Context, operation func() (interface{}, error)) (interface{}, error)

	// State returns the current state of the circuit breaker
	State() CircuitBreakerState

	// Counts returns the current statistics
	Counts() Counts

	// Name returns the name of the circuit breaker
	Name() string

	// Reset manually resets the circuit breaker to closed state
	Reset()

	// Trip manually trips the circuit breaker to open state
	Trip()
}

// circuitBreaker implements the CircuitBreaker interface
type circuitBreaker struct {
	config   *CircuitBreakerConfig
	state    CircuitBreakerState
	counts   Counts
	openTime time.Time
	mu       sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(config *CircuitBreakerConfig) CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig("default")
	}

	return &circuitBreaker{
		config: config,
		state:  StateClosed,
		counts: Counts{},
	}
}

// Execute runs the given function with circuit breaker protection
func (cb *circuitBreaker) Execute(ctx context.Context, operation func() error) error {
	_, err := cb.Call(ctx, func() (interface{}, error) {
		return nil, operation()
	})
	return err
}

// Call runs the given function with circuit breaker protection and returns result
func (cb *circuitBreaker) Call(ctx context.Context, operation func() (interface{}, error)) (interface{}, error) {
	// Check if we can proceed
	if err := cb.beforeCall(); err != nil {
		return nil, err
	}

	// Create a context with timeout if configured
	var opCtx context.Context
	var cancel context.CancelFunc
	if cb.config.Timeout > 0 {
		opCtx, cancel = context.WithTimeout(ctx, cb.config.Timeout)
	} else {
		opCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	// Execute the operation with panic recovery
	result, err := cb.executeWithRecovery(opCtx, operation)

	// Record the result
	cb.afterCall(err == nil)

	return result, err
}

// beforeCall checks if the circuit breaker allows the call
func (cb *circuitBreaker) beforeCall() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case StateClosed:
		// Allow the call
		return nil

	case StateOpen:
		// Check if we should transition to half-open
		if now.Sub(cb.openTime) >= cb.config.ResetTimeout {
			cb.state = StateHalfOpen
			cb.counts = Counts{} // Reset counts for half-open state
			return nil
		}
		// Still open, reject the call
		return cb.createCircuitOpenError()

	case StateHalfOpen:
		// Check if we've exceeded the half-open call limit
		if cb.counts.Requests >= cb.config.HalfOpenMaxCalls {
			return cb.createCircuitOpenError()
		}
		return nil

	default:
		return cb.createCircuitOpenError()
	}
}

// afterCall records the result and updates the circuit breaker state
func (cb *circuitBreaker) afterCall(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.counts.Requests++

	if success {
		cb.counts.TotalSuccesses++
		cb.counts.ConsecutiveSuccesses++
		cb.counts.ConsecutiveFailures = 0

		// Check if we should close from half-open
		if cb.state == StateHalfOpen && cb.counts.ConsecutiveSuccesses >= cb.config.SuccessThreshold {
			cb.state = StateClosed
			cb.counts = Counts{} // Reset counts for closed state
		}
	} else {
		cb.counts.TotalFailures++
		cb.counts.ConsecutiveFailures++
		cb.counts.ConsecutiveSuccesses = 0

		// Check if we should trip the breaker
		if cb.shouldTrip() {
			cb.state = StateOpen
			cb.openTime = time.Now()
		}
	}
}

// shouldTrip determines if the circuit breaker should trip
func (cb *circuitBreaker) shouldTrip() bool {
	// Use custom function if provided
	if cb.config.ShouldTrip != nil {
		return cb.config.ShouldTrip(cb.counts)
	}

	// Default logic: trip if consecutive failures exceed threshold
	return cb.counts.ConsecutiveFailures >= cb.config.MaxFailures
}

// executeWithRecovery executes the operation with panic recovery
func (cb *circuitBreaker) executeWithRecovery(ctx context.Context, operation func() (interface{}, error)) (result interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			// Create enhanced panic error with circuit breaker context
			panicErr := &BaseError{
				Code:      "CIRCUIT_BREAKER_PANIC",
				Message:   fmt.Sprintf("Operation panicked: %v", r),
				Severity:  SeverityError,
				Timestamp: time.Now(),
				Details:   make(map[string]interface{}),
			}
			panicErr.WithDetail("circuit_breaker", cb.config.Name).
				WithDetail("panic_value", r).
				WithDetail("state", cb.state.String())

			result = nil
			err = panicErr
		}
	}()

	// Create a channel to receive the result
	type opResult struct {
		result interface{}
		err    error
	}

	resultChan := make(chan opResult, 1)

	// Execute operation in goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- opResult{nil, fmt.Errorf("operation panicked: %v", r)}
			}
		}()

		res, err := operation()
		select {
		case resultChan <- opResult{res, err}:
		case <-ctx.Done():
			// Context cancelled while operation was running
		}
	}()

	// Wait for result or timeout/cancellation
	select {
	case res := <-resultChan:
		return res.result, res.err
	case <-ctx.Done():
		return nil, cb.createTimeoutError(ctx.Err())
	}
}

// State returns the current state of the circuit breaker
func (cb *circuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Counts returns the current statistics
func (cb *circuitBreaker) Counts() Counts {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.counts
}

// Name returns the name of the circuit breaker
func (cb *circuitBreaker) Name() string {
	return cb.config.Name
}

// Reset manually resets the circuit breaker to closed state
func (cb *circuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateClosed
	cb.counts = Counts{}
}

// Trip manually trips the circuit breaker to open state
func (cb *circuitBreaker) Trip() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = StateOpen
	cb.openTime = time.Now()
}

// createCircuitOpenError creates an error when the circuit is open
func (cb *circuitBreaker) createCircuitOpenError() error {
	return newBaseError("CIRCUIT_BREAKER_OPEN", "Circuit breaker is open").
		WithDetail("circuit_breaker", cb.config.Name).
		WithDetail("state", cb.state.String()).
		WithDetail("consecutive_failures", cb.counts.ConsecutiveFailures).
		WithDetail("max_failures", cb.config.MaxFailures).
		WithDetail("suggestion", "Wait for circuit breaker to reset or check underlying service health").
		WithRetry(cb.config.ResetTimeout)
}

// createTimeoutError creates an error when operation times out
func (cb *circuitBreaker) createTimeoutError(cause error) error {
	return newBaseError("CIRCUIT_BREAKER_TIMEOUT", "Operation timed out").
		WithDetail("circuit_breaker", cb.config.Name).
		WithDetail("timeout", cb.config.Timeout.String()).
		WithDetail("suggestion", "Check network connectivity or increase timeout").
		WithCause(cause)
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	breakers map[string]CircuitBreaker
	mu       sync.RWMutex
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]CircuitBreaker),
	}
}

// GetOrCreate gets an existing circuit breaker or creates a new one
func (cbm *CircuitBreakerManager) GetOrCreate(name string, config *CircuitBreakerConfig) CircuitBreaker {
	cbm.mu.RLock()
	if cb, exists := cbm.breakers[name]; exists {
		cbm.mu.RUnlock()
		return cb
	}
	cbm.mu.RUnlock()

	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	// Double-check in case another goroutine created it
	if cb, exists := cbm.breakers[name]; exists {
		return cb
	}

	if config == nil {
		config = DefaultCircuitBreakerConfig(name)
	} else if config.Name == "" {
		config.Name = name
	}

	cb := NewCircuitBreaker(config)
	cbm.breakers[name] = cb
	return cb
}

// Get retrieves a circuit breaker by name
func (cbm *CircuitBreakerManager) Get(name string) (CircuitBreaker, bool) {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	cb, exists := cbm.breakers[name]
	return cb, exists
}

// Remove removes a circuit breaker
func (cbm *CircuitBreakerManager) Remove(name string) bool {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	if _, exists := cbm.breakers[name]; exists {
		delete(cbm.breakers, name)
		return true
	}
	return false
}

// List returns all circuit breaker names
func (cbm *CircuitBreakerManager) List() []string {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	names := make([]string, 0, len(cbm.breakers))
	for name := range cbm.breakers {
		names = append(names, name)
	}
	return names
}

// GetStats returns statistics for all circuit breakers
func (cbm *CircuitBreakerManager) GetStats() map[string]CircuitBreakerStats {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	stats := make(map[string]CircuitBreakerStats)
	for name, cb := range cbm.breakers {
		counts := cb.Counts()
		stats[name] = CircuitBreakerStats{
			Name:        cb.Name(),
			State:       cb.State(),
			Counts:      counts,
			SuccessRate: calculateSuccessRate(counts),
			FailureRate: calculateFailureRate(counts),
		}
	}
	return stats
}

// CircuitBreakerStats contains statistics for a circuit breaker
type CircuitBreakerStats struct {
	Name        string
	State       CircuitBreakerState
	Counts      Counts
	SuccessRate float64
	FailureRate float64
}

// calculateSuccessRate calculates the success rate
func calculateSuccessRate(counts Counts) float64 {
	if counts.Requests == 0 {
		return 0.0
	}
	return float64(counts.TotalSuccesses) / float64(counts.Requests)
}

// calculateFailureRate calculates the failure rate
func calculateFailureRate(counts Counts) float64 {
	if counts.Requests == 0 {
		return 0.0
	}
	return float64(counts.TotalFailures) / float64(counts.Requests)
}

// Global circuit breaker manager instance
var globalCircuitBreakerManager = NewCircuitBreakerManager()

// GetCircuitBreaker gets or creates a circuit breaker with the given name and config
func GetCircuitBreaker(name string, config *CircuitBreakerConfig) CircuitBreaker {
	return globalCircuitBreakerManager.GetOrCreate(name, config)
}

// GetCircuitBreakerStats returns statistics for all circuit breakers
func GetCircuitBreakerStats() map[string]CircuitBreakerStats {
	return globalCircuitBreakerManager.GetStats()
}

// Helper function to create a BaseError with circuit breaker context
func newBaseError(code, message string) *BaseError {
	return &BaseError{
		Code:      code,
		Message:   message,
		Severity:  SeverityError,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}
}
