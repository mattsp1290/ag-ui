package events

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestParallelValidationConfig tests the parallel validation configuration
func TestParallelValidationConfig(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		config := DefaultParallelValidationConfig()
		
		if config.MaxGoroutines != runtime.NumCPU() {
			t.Errorf("Expected MaxGoroutines to be %d, got %d", runtime.NumCPU(), config.MaxGoroutines)
		}
		
		if !config.EnableParallelExecution {
			t.Error("Expected EnableParallelExecution to be true")
		}
		
		if config.MinRulesForParallel != ParallelRuleThreshold {
			t.Errorf("Expected MinRulesForParallel to be %d, got %d", ParallelRuleThreshold, config.MinRulesForParallel)
		}
		
		if config.ValidationTimeout != ValidationTimeoutDefault {
			t.Errorf("Expected ValidationTimeout to be %v, got %v", ValidationTimeoutDefault, config.ValidationTimeout)
		}
		
		if !config.EnableDependencyAnalysis {
			t.Error("Expected EnableDependencyAnalysis to be true")
		}
	})
	
	t.Run("custom config", func(t *testing.T) {
		config := &ParallelValidationConfig{
			MaxGoroutines:           8,
			EnableParallelExecution: false,
			MinRulesForParallel:     5,
			ValidationTimeout:       10 * time.Second,
			BufferSize:              20,
			EnableDependencyAnalysis: false,
			StopOnFirstError:        true,
		}
		
		if config.MaxGoroutines != 8 {
			t.Errorf("Expected MaxGoroutines to be 8, got %d", config.MaxGoroutines)
		}
		
		if config.EnableParallelExecution {
			t.Error("Expected EnableParallelExecution to be false")
		}
		
		if config.StopOnFirstError != true {
			t.Error("Expected StopOnFirstError to be true")
		}
	})
}

// TestRuleDependencyAnalyzer tests the rule dependency analysis functionality
func TestRuleDependencyAnalyzer(t *testing.T) {
	analyzer := NewRuleDependencyAnalyzer()
	
	t.Run("known rule dependencies", func(t *testing.T) {
		// Test independent rules
		dep, exists := analyzer.GetRuleDependency("MESSAGE_CONTENT")
		if !exists {
			t.Error("Expected MESSAGE_CONTENT rule to exist")
		}
		if !dep.CanRunInParallel {
			t.Error("Expected MESSAGE_CONTENT to be able to run in parallel")
		}
		if dep.DependencyType != DependencyNone {
			t.Errorf("Expected DependencyNone, got %v", dep.DependencyType)
		}
		
		// Test dependent rules
		dep, exists = analyzer.GetRuleDependency("RUN_LIFECYCLE")
		if !exists {
			t.Error("Expected RUN_LIFECYCLE rule to exist")
		}
		if dep.CanRunInParallel {
			t.Error("Expected RUN_LIFECYCLE to not be able to run in parallel")
		}
		if dep.DependencyType != DependencyStateWrite {
			t.Errorf("Expected DependencyStateWrite, got %v", dep.DependencyType)
		}
	})
	
	t.Run("add custom rule dependency", func(t *testing.T) {
		customDep := &RuleDependency{
			RuleID:           "CUSTOM_RULE",
			DependencyType:   DependencyNone,
			RequiredRules:    []string{},
			ConflictingRules: []string{},
			CanRunInParallel: true,
		}
		
		analyzer.AddRuleDependency(customDep)
		
		dep, exists := analyzer.GetRuleDependency("CUSTOM_RULE")
		if !exists {
			t.Error("Expected CUSTOM_RULE to exist after adding")
		}
		if !dep.CanRunInParallel {
			t.Error("Expected CUSTOM_RULE to be able to run in parallel")
		}
	})
	
	t.Run("analyze rule groups", func(t *testing.T) {
		// Create mock rules
		rules := []ValidationRule{
			&MockValidationRule{id: "MESSAGE_CONTENT", enabled: true},
			&MockValidationRule{id: "TOOL_CALL_CONTENT", enabled: true},
			&MockValidationRule{id: "RUN_LIFECYCLE", enabled: true},
			&MockValidationRule{id: "EVENT_ORDERING", enabled: true},
			&MockValidationRule{id: "UNKNOWN_RULE", enabled: true},
		}
		
		independent, dependent := analyzer.AnalyzeRules(rules)
		
		if len(independent) != 2 {
			t.Errorf("Expected 2 independent rules, got %d", len(independent))
		}
		
		if len(dependent) != 3 {
			t.Errorf("Expected 3 dependent rules, got %d", len(dependent))
		}
		
		// Check that independent rules are correct
		independentIDs := make(map[string]bool)
		for _, rule := range independent {
			independentIDs[rule.ID()] = true
		}
		
		if !independentIDs["MESSAGE_CONTENT"] {
			t.Error("Expected MESSAGE_CONTENT to be in independent rules")
		}
		if !independentIDs["TOOL_CALL_CONTENT"] {
			t.Error("Expected TOOL_CALL_CONTENT to be in independent rules")
		}
	})
}

// TestParallelWorkerPool tests the worker pool functionality
func TestParallelWorkerPool(t *testing.T) {
	t.Run("basic worker pool operations", func(t *testing.T) {
		workerCount := 2
		bufferSize := 5
		
		pool := NewParallelWorkerPool(workerCount, bufferSize)
		pool.Start()
		defer pool.Stop()
		
		// Submit jobs
		event := &RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
			RunID:     "test-run",
			ThreadID:  "test-thread",
		}
		
		rule := &MockValidationRule{id: "TEST_RULE", enabled: true}
		context := &ValidationContext{}
		
		job := ParallelValidationJob{
			Rule:    rule,
			Event:   event,
			Context: context,
			JobID:   1,
		}
		
		err := pool.SubmitJob(job)
		if err != nil {
			t.Fatalf("Failed to submit job: %v", err)
		}
		
		// Get result
		result, err := pool.GetResult()
		if err != nil {
			t.Fatalf("Failed to get result: %v", err)
		}
		
		if result.JobID != 1 {
			t.Errorf("Expected JobID 1, got %d", result.JobID)
		}
		
		if result.Result == nil {
			t.Error("Expected non-nil result")
		}
	})
	
	t.Run("worker pool capacity", func(t *testing.T) {
		workerCount := 2
		bufferSize := 5
		
		pool := NewParallelWorkerPool(workerCount, bufferSize)
		pool.Start()
		defer pool.Stop()
		
		event := &RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
			RunID:     "test-run",
			ThreadID:  "test-thread",
		}
		
		rule := &MockValidationRule{id: "FAST_RULE", enabled: true, delay: 1 * time.Millisecond}
		context := &ValidationContext{}
		
		// Submit and process jobs one by one
		job := ParallelValidationJob{Rule: rule, Event: event, Context: context, JobID: 1}
		err := pool.SubmitJob(job)
		if err != nil {
			t.Fatalf("Failed to submit job: %v", err)
		}
		
		// Wait a bit for processing
		time.Sleep(10 * time.Millisecond)
		
		// Try to get result
		result, err := pool.GetResult()
		if err != nil {
			t.Fatalf("Failed to get result: %v", err)
		}
		
		if result.JobID != 1 {
			t.Errorf("Expected JobID 1, got %d", result.JobID)
		}
	})
}

// TestParallelValidator tests the parallel validator functionality
func TestParallelValidator(t *testing.T) {
	t.Run("basic parallel validation", func(t *testing.T) {
		config := DefaultParallelValidationConfig()
		config.MaxGoroutines = 2
		config.MinRulesForParallel = 2
		
		validator := NewParallelValidator(config)
		
		event := &RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
			RunID:     "test-run",
			ThreadID:  "test-thread",
		}
		
		rules := []ValidationRule{
			&MockValidationRule{id: "MESSAGE_CONTENT", enabled: true, delay: 10 * time.Millisecond},
			&MockValidationRule{id: "TOOL_CALL_CONTENT", enabled: true, delay: 10 * time.Millisecond},
			&MockValidationRule{id: "RUN_LIFECYCLE", enabled: true, delay: 10 * time.Millisecond},
		}
		
		validationContext := &ValidationContext{
			State: NewValidationState(),
		}
		
		ctx := context.Background()
		result := validator.ValidateEventParallel(ctx, event, rules, validationContext)
		
		if !result.IsValid {
			t.Error("Expected validation to pass")
		}
		
		if result.RulesExecutedInParallel != 2 {
			t.Errorf("Expected 2 rules executed in parallel, got %d", result.RulesExecutedInParallel)
		}
		
		if result.RulesExecutedSequentially != 1 {
			t.Errorf("Expected 1 rule executed sequentially, got %d", result.RulesExecutedSequentially)
		}
		
		if result.GoroutinesUsed < 1 {
			t.Errorf("Expected at least 1 goroutine used, got %d", result.GoroutinesUsed)
		}
	})
	
	t.Run("sequential fallback", func(t *testing.T) {
		config := DefaultParallelValidationConfig()
		config.EnableParallelExecution = false
		
		validator := NewParallelValidator(config)
		
		event := &RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
			RunID:     "test-run",
			ThreadID:  "test-thread",
		}
		
		rules := []ValidationRule{
			&MockValidationRule{id: "MESSAGE_CONTENT", enabled: true},
			&MockValidationRule{id: "TOOL_CALL_CONTENT", enabled: true},
		}
		
		validationContext := &ValidationContext{
			State: NewValidationState(),
		}
		
		ctx := context.Background()
		result := validator.ValidateEventParallel(ctx, event, rules, validationContext)
		
		if !result.IsValid {
			t.Error("Expected validation to pass")
		}
		
		if result.RulesExecutedInParallel != 0 {
			t.Errorf("Expected 0 rules executed in parallel, got %d", result.RulesExecutedInParallel)
		}
		
		if result.RulesExecutedSequentially != 2 {
			t.Errorf("Expected 2 rules executed sequentially, got %d", result.RulesExecutedSequentially)
		}
	})
	
	t.Run("insufficient rules for parallel", func(t *testing.T) {
		config := DefaultParallelValidationConfig()
		config.MinRulesForParallel = 5
		
		validator := NewParallelValidator(config)
		
		event := &RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
			RunID:     "test-run",
			ThreadID:  "test-thread",
		}
		
		rules := []ValidationRule{
			&MockValidationRule{id: "MESSAGE_CONTENT", enabled: true},
			&MockValidationRule{id: "TOOL_CALL_CONTENT", enabled: true},
		}
		
		validationContext := &ValidationContext{
			State: NewValidationState(),
		}
		
		ctx := context.Background()
		result := validator.ValidateEventParallel(ctx, event, rules, validationContext)
		
		if result.RulesExecutedInParallel != 0 {
			t.Errorf("Expected 0 rules executed in parallel due to threshold, got %d", result.RulesExecutedInParallel)
		}
	})
	
	t.Run("stop on first error", func(t *testing.T) {
		config := DefaultParallelValidationConfig()
		config.StopOnFirstError = true
		config.MaxGoroutines = 2
		config.MinRulesForParallel = 2
		
		validator := NewParallelValidator(config)
		
		event := &RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
			RunID:     "test-run",
			ThreadID:  "test-thread",
		}
		
		rules := []ValidationRule{
			&MockValidationRule{id: "MESSAGE_CONTENT", enabled: true, shouldError: true},
			&MockValidationRule{id: "TOOL_CALL_CONTENT", enabled: true, delay: 50 * time.Millisecond},
		}
		
		validationContext := &ValidationContext{
			State: NewValidationState(),
		}
		
		ctx := context.Background()
		result := validator.ValidateEventParallel(ctx, event, rules, validationContext)
		
		if result.IsValid {
			t.Error("Expected validation to fail")
		}
		
		if len(result.Errors) == 0 {
			t.Error("Expected at least one error")
		}
	})
	
	t.Run("context cancellation", func(t *testing.T) {
		config := DefaultParallelValidationConfig()
		config.MaxGoroutines = 2
		config.MinRulesForParallel = 2
		
		validator := NewParallelValidator(config)
		
		event := &RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
			RunID:     "test-run",
			ThreadID:  "test-thread",
		}
		
		rules := []ValidationRule{
			&MockValidationRule{id: "MESSAGE_CONTENT", enabled: true, delay: 1 * time.Second},
			&MockValidationRule{id: "TOOL_CALL_CONTENT", enabled: true, delay: 1 * time.Second},
		}
		
		validationContext := &ValidationContext{
			State: NewValidationState(),
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		result := validator.ValidateEventParallel(ctx, event, rules, validationContext)
		
		if result.IsValid {
			t.Error("Expected validation to fail due to context cancellation")
		}
		
		foundCancellationError := false
		for _, err := range result.Errors {
			if err.RuleID == "CONTEXT_CANCELLED" {
				foundCancellationError = true
				break
			}
		}
		
		if !foundCancellationError {
			t.Error("Expected to find context cancellation error")
		}
	})
}

// TestEventValidatorParallel tests the integration with EventValidator
func TestEventValidatorParallel(t *testing.T) {
	t.Run("basic parallel validation integration", func(t *testing.T) {
		validator := NewEventValidator(DefaultValidationConfig())
		
		// Configure parallel validation
		parallelConfig := DefaultParallelValidationConfig()
		parallelConfig.MaxGoroutines = 2
		parallelConfig.MinRulesForParallel = 3
		validator.SetParallelConfig(parallelConfig)
		
		event := &RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
			RunID:     "test-run",
			ThreadID:  "test-thread",
		}
		
		ctx := context.Background()
		result := validator.ValidateEventParallel(ctx, event)
		
		if !result.IsValid {
			t.Errorf("Expected validation to pass, got errors: %v", result.Errors)
		}
		
		// Check that we have meaningful execution counts
		totalRules := result.RulesExecutedInParallel + result.RulesExecutedSequentially
		if totalRules == 0 {
			t.Error("Expected some rules to be executed")
		}
	})
	
	t.Run("parallel config management", func(t *testing.T) {
		validator := NewEventValidator(DefaultValidationConfig())
		
		// Test getting default config
		config := validator.GetParallelConfig()
		if config.MaxGoroutines != runtime.NumCPU() {
			t.Errorf("Expected default MaxGoroutines to be %d, got %d", runtime.NumCPU(), config.MaxGoroutines)
		}
		
		// Test setting new config
		newConfig := &ParallelValidationConfig{
			MaxGoroutines:           8,
			EnableParallelExecution: false,
			MinRulesForParallel:     5,
		}
		validator.SetParallelConfig(newConfig)
		
		config = validator.GetParallelConfig()
		if config.MaxGoroutines != 8 {
			t.Errorf("Expected MaxGoroutines to be 8, got %d", config.MaxGoroutines)
		}
		
		if config.EnableParallelExecution {
			t.Error("Expected EnableParallelExecution to be false")
		}
		
		// Test enable/disable
		validator.EnableParallelValidation(true)
		if !validator.IsParallelValidationEnabled() {
			t.Error("Expected parallel validation to be enabled")
		}
		
		validator.EnableParallelValidation(false)
		if validator.IsParallelValidationEnabled() {
			t.Error("Expected parallel validation to be disabled")
		}
		
		// Test max goroutines setting
		validator.SetMaxGoroutines(16)
		if validator.GetMaxGoroutines() != 16 {
			t.Errorf("Expected MaxGoroutines to be 16, got %d", validator.GetMaxGoroutines())
		}
	})
	
	t.Run("parallel metrics", func(t *testing.T) {
		validator := NewEventValidator(DefaultValidationConfig())
		
		// Configure for parallel execution
		parallelConfig := DefaultParallelValidationConfig()
		parallelConfig.MinRulesForParallel = 1 // Force parallel execution
		validator.SetParallelConfig(parallelConfig)
		
		event := &RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
			RunID:     "test-run",
			ThreadID:  "test-thread",
		}
		
		ctx := context.Background()
		_ = validator.ValidateEventParallel(ctx, event)
		
		metrics := validator.GetParallelMetrics()
		if metrics.TotalValidations == 0 {
			t.Error("Expected at least one validation to be recorded")
		}
	})
	
	t.Run("nil event handling", func(t *testing.T) {
		validator := NewEventValidator(DefaultValidationConfig())
		
		ctx := context.Background()
		result := validator.ValidateEventParallel(ctx, nil)
		
		if result.IsValid {
			t.Error("Expected validation to fail for nil event")
		}
		
		if len(result.Errors) == 0 {
			t.Error("Expected error for nil event")
		}
		
		if result.Errors[0].RuleID != "NULL_EVENT" {
			t.Errorf("Expected NULL_EVENT error, got %s", result.Errors[0].RuleID)
		}
	})
}

// TestParallelValidationMetrics tests the metrics functionality
func TestParallelValidationMetrics(t *testing.T) {
	t.Run("metric recording", func(t *testing.T) {
		metrics := NewParallelValidationMetrics()
		
		// Record some parallel validations
		metrics.RecordParallelValidation(100*time.Millisecond, 4, 5)
		metrics.RecordParallelValidation(200*time.Millisecond, 4, 5)
		
		// Record some sequential validations
		metrics.RecordSequentialValidation(300*time.Millisecond, 5)
		metrics.RecordSequentialValidation(400*time.Millisecond, 5)
		
		if metrics.TotalValidations != 4 {
			t.Errorf("Expected 4 total validations, got %d", metrics.TotalValidations)
		}
		
		if metrics.ParallelValidations != 2 {
			t.Errorf("Expected 2 parallel validations, got %d", metrics.ParallelValidations)
		}
		
		if metrics.SequentialValidations != 2 {
			t.Errorf("Expected 2 sequential validations, got %d", metrics.SequentialValidations)
		}
		
		if metrics.AverageSpeedup <= 0 {
			t.Errorf("Expected positive speedup, got %f", metrics.AverageSpeedup)
		}
		
		// Test metrics copy
		metricsCopy := metrics.GetMetrics()
		if metricsCopy.TotalValidations != metrics.TotalValidations {
			t.Error("Metrics copy should match original")
		}
	})
}

// TestConcurrentParallelValidation tests concurrent usage of parallel validation
func TestConcurrentParallelValidation(t *testing.T) {
	validator := NewEventValidator(DefaultValidationConfig())
	
	// Configure for parallel execution
	parallelConfig := DefaultParallelValidationConfig()
	parallelConfig.MaxGoroutines = 2
	parallelConfig.MinRulesForParallel = 1
	validator.SetParallelConfig(parallelConfig)
	
	var wg sync.WaitGroup
	errors := make(chan error, 10)
	
	// Run multiple parallel validations concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			event := &RunStartedEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
				RunID:     fmt.Sprintf("test-run-%d", id),
				ThreadID:  fmt.Sprintf("test-thread-%d", id),
			}
			
			ctx := context.Background()
			result := validator.ValidateEventParallel(ctx, event)
			
			if !result.IsValid {
				errors <- fmt.Errorf("validation %d failed: %v", id, result.Errors)
			}
		}(i)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	for err := range errors {
		t.Error(err)
	}
}

// MockValidationRule is a mock implementation of ValidationRule for testing
type MockValidationRule struct {
	id          string
	description string
	enabled     bool
	severity    ValidationSeverity
	delay       time.Duration
	shouldError bool
}

func (m *MockValidationRule) ID() string {
	return m.id
}

func (m *MockValidationRule) Description() string {
	if m.description == "" {
		return fmt.Sprintf("Mock rule %s", m.id)
	}
	return m.description
}

func (m *MockValidationRule) Validate(event Event, context *ValidationContext) *ValidationResult {
	// Simulate processing time
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	
	result := &ValidationResult{
		IsValid:   true,
		Errors:    make([]*ValidationError, 0),
		Warnings:  make([]*ValidationError, 0),
		Timestamp: time.Now(),
	}
	
	if m.shouldError {
		result.AddError(&ValidationError{
			RuleID:    m.id,
			Message:   fmt.Sprintf("Mock error from %s", m.id),
			Severity:  m.severity,
			Timestamp: time.Now(),
		})
	}
	
	return result
}

func (m *MockValidationRule) IsEnabled() bool {
	return m.enabled
}

func (m *MockValidationRule) SetEnabled(enabled bool) {
	m.enabled = enabled
}

func (m *MockValidationRule) GetSeverity() ValidationSeverity {
	return m.severity
}

func (m *MockValidationRule) SetSeverity(severity ValidationSeverity) {
	m.severity = severity
}

// Helper function for creating time pointers (renamed to avoid conflicts)
func parallelTimePtr(t int64) *int64 {
	return &t
}