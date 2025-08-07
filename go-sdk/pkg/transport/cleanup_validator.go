package transport

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// CleanupValidator validates that cleanup operations complete successfully
type CleanupValidator struct {
	mu                sync.RWMutex
	validationRules   []CleanupValidationRule
	completedCleanups map[string]*CleanupValidationResult
	config            CleanupValidationConfig

	// Validation state
	validationErrors []CleanupValidationError
	memoryLeaks      []MemoryLeak
	goroutineLeaks   []GoroutineLeak
	resourceLeaks    []ResourceLeak
}

// CleanupValidationConfig configures cleanup validation
type CleanupValidationConfig struct {
	// EnableMemoryValidation enables memory leak detection
	EnableMemoryValidation bool

	// EnableGoroutineValidation enables goroutine leak detection
	EnableGoroutineValidation bool

	// EnableResourceValidation enables resource leak detection
	EnableResourceValidation bool

	// MemoryThreshold is the memory increase threshold to consider a leak
	MemoryThreshold int64

	// GoroutineThreshold is the goroutine increase threshold to consider a leak
	GoroutineThreshold int

	// ValidationTimeout is the timeout for validation operations
	ValidationTimeout time.Duration

	// Logger for validation events
	Logger Logger
}

// DefaultCleanupValidationConfig returns default validation configuration
func DefaultCleanupValidationConfig() CleanupValidationConfig {
	return CleanupValidationConfig{
		EnableMemoryValidation:    true,
		EnableGoroutineValidation: true,
		EnableResourceValidation:  true,
		MemoryThreshold:           1024 * 1024, // 1MB
		GoroutineThreshold:        5,
		ValidationTimeout:         30 * time.Second,
		Logger:                    nil,
	}
}

// CleanupValidationRule defines a rule for validating cleanup
type CleanupValidationRule interface {
	Name() string
	Description() string
	Validate(ctx context.Context, result *CleanupValidationResult) error
}

// CleanupValidationResult contains the result of a cleanup validation
type CleanupValidationResult struct {
	ComponentID string
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	Success     bool
	Errors      []error

	// Memory statistics
	MemoryBefore runtime.MemStats
	MemoryAfter  runtime.MemStats
	MemoryDelta  int64

	// Goroutine statistics
	GoroutinesBefore int
	GoroutinesAfter  int
	GoroutineDelta   int

	// Resource tracking
	ResourcesTracked int
	ResourcesCleaned int
	ResourcesLeaked  int
	LeakedResources  []string

	// Cleanup phases
	PhaseResults map[CleanupPhase]PhaseResult

	// Validation results
	ValidationErrors []CleanupValidationError
	RulePassed       map[string]bool
	RuleErrors       map[string]error
}

// PhaseResult contains the result of a cleanup phase
type PhaseResult struct {
	Phase            CleanupPhase
	StartTime        time.Time
	EndTime          time.Time
	Duration         time.Duration
	Success          bool
	ResourcesCleaned int
	Errors           []error
}

// CleanupValidationError represents a validation error
type CleanupValidationError struct {
	Rule      string
	Message   string
	Severity  ErrorSeverity
	Component string
	Timestamp time.Time
	Details   map[string]interface{}
}

// ErrorSeverity represents the severity of a validation error
type ErrorSeverity int

const (
	SeverityInfoError ErrorSeverity = iota
	SeverityWarningError
	SeverityErrorError
	SeverityCritical
)

// MemoryLeak represents a detected memory leak
type MemoryLeak struct {
	Component   string
	Delta       int64
	Threshold   int64
	AllocBefore uint64
	AllocAfter  uint64
	DetectedAt  time.Time
	StackTrace  string
}

// GoroutineLeak represents a detected goroutine leak
type GoroutineLeak struct {
	Component      string
	Delta          int
	Threshold      int
	CountBefore    int
	CountAfter     int
	DetectedAt     time.Time
	StackTrace     string
	LeakedRoutines []string
}

// ResourceLeak represents a detected resource leak
type ResourceLeak struct {
	Component    string
	ResourceType ResourceType
	ResourceID   string
	LeakedAt     time.Time
	StackTrace   string
}

// NewCleanupValidator creates a new cleanup validator
func NewCleanupValidator(config CleanupValidationConfig) *CleanupValidator {
	cv := &CleanupValidator{
		validationRules:   make([]CleanupValidationRule, 0),
		completedCleanups: make(map[string]*CleanupValidationResult),
		config:            config,
		validationErrors:  make([]CleanupValidationError, 0),
		memoryLeaks:       make([]MemoryLeak, 0),
		goroutineLeaks:    make([]GoroutineLeak, 0),
		resourceLeaks:     make([]ResourceLeak, 0),
	}

	// Add default validation rules
	cv.addDefaultRules()

	return cv
}

// addDefaultRules adds default validation rules
func (cv *CleanupValidator) addDefaultRules() {
	if cv.config.EnableMemoryValidation {
		cv.AddRule(&MemoryLeakRule{threshold: cv.config.MemoryThreshold})
	}

	if cv.config.EnableGoroutineValidation {
		cv.AddRule(&GoroutineLeakRule{threshold: cv.config.GoroutineThreshold})
	}

	if cv.config.EnableResourceValidation {
		cv.AddRule(&ResourceLeakRule{})
	}

	cv.AddRule(&CleanupTimeoutRule{timeout: cv.config.ValidationTimeout})
	cv.AddRule(&CleanupCompletionRule{})
}

// AddRule adds a validation rule
func (cv *CleanupValidator) AddRule(rule CleanupValidationRule) {
	cv.mu.Lock()
	defer cv.mu.Unlock()

	cv.validationRules = append(cv.validationRules, rule)

	if cv.config.Logger != nil {
		cv.config.Logger.Debug("Validation rule added",
			String("name", rule.Name()),
			String("description", rule.Description()))
	}
}

// ValidateCleanup validates a cleanup operation
func (cv *CleanupValidator) ValidateCleanup(ctx context.Context, componentID string, tracker *CleanupTracker) (*CleanupValidationResult, error) {
	// Create validation context with timeout
	validationCtx, cancel := context.WithTimeout(ctx, cv.config.ValidationTimeout)
	defer cancel()

	// Collect pre-cleanup metrics
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)
	goroutinesBefore := runtime.NumGoroutine()

	// Create validation result
	result := &CleanupValidationResult{
		ComponentID:      componentID,
		StartTime:        time.Now(),
		MemoryBefore:     memBefore,
		GoroutinesBefore: goroutinesBefore,
		PhaseResults:     make(map[CleanupPhase]PhaseResult),
		RulePassed:       make(map[string]bool),
		RuleErrors:       make(map[string]error),
	}

	// Wait for cleanup to complete
	if err := tracker.Wait(); err != nil {
		result.Success = false
		result.Errors = append(result.Errors, err)
	} else {
		result.Success = true
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Collect post-cleanup metrics
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)
	goroutinesAfter := runtime.NumGoroutine()

	result.MemoryAfter = memAfter
	result.GoroutinesAfter = goroutinesAfter
	result.MemoryDelta = int64(memAfter.Alloc) - int64(memBefore.Alloc)
	result.GoroutineDelta = goroutinesAfter - goroutinesBefore

	// Get cleanup statistics
	stats := tracker.GetStats()
	result.ResourcesTracked = int(stats.TotalTracked)
	result.ResourcesCleaned = int(stats.TotalCleaned)
	result.ResourcesLeaked = len(stats.ResourcesLeaked)

	for _, leaked := range stats.ResourcesLeaked {
		result.LeakedResources = append(result.LeakedResources, leaked.ID)
	}

	// Run validation rules
	cv.runValidationRules(validationCtx, result)

	// Store result
	cv.mu.Lock()
	cv.completedCleanups[componentID] = result
	cv.mu.Unlock()

	if cv.config.Logger != nil {
		cv.config.Logger.Info("Cleanup validation completed",
			String("component", componentID),
			Bool("success", result.Success),
			Duration("duration", result.Duration),
			Int64("memory_delta", result.MemoryDelta),
			Int("goroutine_delta", result.GoroutineDelta),
			Any("resources_leaked", result.ResourcesLeaked),
			Int("validation_errors", len(result.ValidationErrors)))
	}

	return result, nil
}

// runValidationRules runs all validation rules against the result
func (cv *CleanupValidator) runValidationRules(ctx context.Context, result *CleanupValidationResult) {
	cv.mu.RLock()
	rules := make([]CleanupValidationRule, len(cv.validationRules))
	copy(rules, cv.validationRules)
	cv.mu.RUnlock()

	for _, rule := range rules {
		select {
		case <-ctx.Done():
			result.RuleErrors[rule.Name()] = fmt.Errorf("validation cancelled: %w", ctx.Err())
			return
		default:
		}

		if err := rule.Validate(ctx, result); err != nil {
			result.RulePassed[rule.Name()] = false
			result.RuleErrors[rule.Name()] = err

			// Add to validation errors
			validationErr := CleanupValidationError{
				Rule:      rule.Name(),
				Message:   err.Error(),
				Severity:  SeverityErrorError,
				Component: result.ComponentID,
				Timestamp: time.Now(),
			}
			result.ValidationErrors = append(result.ValidationErrors, validationErr)

			if cv.config.Logger != nil {
				cv.config.Logger.Error("Validation rule failed",
					String("rule", rule.Name()),
					String("component", result.ComponentID),
					Err(err))
			}
		} else {
			result.RulePassed[rule.Name()] = true
		}
	}
}

// GetValidationResult returns the validation result for a component
func (cv *CleanupValidator) GetValidationResult(componentID string) (*CleanupValidationResult, bool) {
	cv.mu.RLock()
	defer cv.mu.RUnlock()

	result, exists := cv.completedCleanups[componentID]
	return result, exists
}

// GetAllValidationResults returns all validation results
func (cv *CleanupValidator) GetAllValidationResults() map[string]*CleanupValidationResult {
	cv.mu.RLock()
	defer cv.mu.RUnlock()

	results := make(map[string]*CleanupValidationResult)
	for k, v := range cv.completedCleanups {
		results[k] = v
	}

	return results
}

// GetValidationErrors returns all validation errors
func (cv *CleanupValidator) GetValidationErrors() []CleanupValidationError {
	cv.mu.RLock()
	defer cv.mu.RUnlock()

	return cv.validationErrors
}

// Default validation rules

// MemoryLeakRule validates memory usage
type MemoryLeakRule struct {
	threshold int64
}

func (r *MemoryLeakRule) Name() string {
	return "memory_leak"
}

func (r *MemoryLeakRule) Description() string {
	return "Validates that memory usage doesn't increase beyond threshold"
}

func (r *MemoryLeakRule) Validate(ctx context.Context, result *CleanupValidationResult) error {
	if result.MemoryDelta > r.threshold {
		return fmt.Errorf("memory increased by %d bytes, threshold %d bytes", result.MemoryDelta, r.threshold)
	}
	return nil
}

// GoroutineLeakRule validates goroutine count
type GoroutineLeakRule struct {
	threshold int
}

func (r *GoroutineLeakRule) Name() string {
	return "goroutine_leak"
}

func (r *GoroutineLeakRule) Description() string {
	return "Validates that goroutine count doesn't increase beyond threshold"
}

func (r *GoroutineLeakRule) Validate(ctx context.Context, result *CleanupValidationResult) error {
	if result.GoroutineDelta > r.threshold {
		return fmt.Errorf("goroutine count increased by %d, threshold %d", result.GoroutineDelta, r.threshold)
	}
	return nil
}

// ResourceLeakRule validates resource cleanup
type ResourceLeakRule struct{}

func (r *ResourceLeakRule) Name() string {
	return "resource_leak"
}

func (r *ResourceLeakRule) Description() string {
	return "Validates that all tracked resources are properly cleaned up"
}

func (r *ResourceLeakRule) Validate(ctx context.Context, result *CleanupValidationResult) error {
	if result.ResourcesLeaked > 0 {
		return fmt.Errorf("leaked resources: %v", result.LeakedResources)
	}
	return nil
}

// CleanupTimeoutRule validates cleanup timing
type CleanupTimeoutRule struct {
	timeout time.Duration
}

func (r *CleanupTimeoutRule) Name() string {
	return "cleanup_timeout"
}

func (r *CleanupTimeoutRule) Description() string {
	return "Validates that cleanup completes within timeout"
}

func (r *CleanupTimeoutRule) Validate(ctx context.Context, result *CleanupValidationResult) error {
	if result.Duration > r.timeout {
		return fmt.Errorf("cleanup took %v, timeout %v", result.Duration, r.timeout)
	}
	return nil
}

// CleanupCompletionRule validates cleanup completion
type CleanupCompletionRule struct{}

func (r *CleanupCompletionRule) Name() string {
	return "cleanup_completion"
}

func (r *CleanupCompletionRule) Description() string {
	return "Validates that cleanup completed successfully"
}

func (r *CleanupCompletionRule) Validate(ctx context.Context, result *CleanupValidationResult) error {
	if !result.Success {
		return fmt.Errorf("cleanup failed with errors: %v", result.Errors)
	}
	return nil
}

// String returns string representation of error severity
func (s ErrorSeverity) String() string {
	switch s {
	case SeverityInfoError:
		return "info"
	case SeverityWarningError:
		return "warning"
	case SeverityErrorError:
		return "error"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// String returns string representation of validation error
func (ve CleanupValidationError) String() string {
	return fmt.Sprintf("[%s] %s: %s (%s)", ve.Severity, ve.Rule, ve.Message, ve.Component)
}

// String returns string representation of memory leak
func (ml MemoryLeak) String() string {
	return fmt.Sprintf("Memory leak in %s: %d bytes (threshold: %d)", ml.Component, ml.Delta, ml.Threshold)
}

// String returns string representation of goroutine leak
func (gl GoroutineLeak) String() string {
	return fmt.Sprintf("Goroutine leak in %s: %d goroutines (threshold: %d)", gl.Component, gl.Delta, gl.Threshold)
}

// String returns string representation of resource leak
func (rl ResourceLeak) String() string {
	return fmt.Sprintf("Resource leak in %s: %s (%s)", rl.Component, rl.ResourceID, rl.ResourceType)
}
