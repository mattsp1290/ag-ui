package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PipelineExecutor manages the execution of validation pipelines
type PipelineExecutor struct {
	orchestrator   *Orchestrator
	stageManager   *StageManager
	retryManager   *RetryManager
	timeoutManager *TimeoutManager
	metrics        *PipelineMetrics
	mu             sync.RWMutex
}

// RetryManager handles retry logic for failed stages
type RetryManager struct {
	policies map[string]*RetryPolicy
	mu       sync.RWMutex
}

// TimeoutManager handles stage timeouts
type TimeoutManager struct {
	defaultTimeout time.Duration
	stageTimeouts  map[string]time.Duration
	mu             sync.RWMutex
}

// PipelineMetrics tracks pipeline execution metrics
type PipelineMetrics struct {
	TotalExecutions    int64
	SuccessfulRuns     int64
	FailedRuns         int64
	AverageExecutionTime time.Duration
	StageMetrics       map[string]*StageMetrics
	mu                 sync.RWMutex
}

// StageMetrics tracks metrics for individual stages
type StageMetrics struct {
	ExecutionCount   int64
	SuccessCount     int64
	FailureCount     int64
	AverageTime      time.Duration
	MinTime          time.Duration
	MaxTime          time.Duration
	RetryCount       int64
	TimeoutCount     int64
}

// PipelineContext provides context for pipeline execution
type PipelineContext struct {
	ExecutionID     string
	WorkflowID      string
	StageID         string
	ValidationCtx   *ValidationContext
	ParentContext   context.Context
	Timeout         time.Duration
	RetryPolicy     *RetryPolicy
	Metadata        map[string]interface{}
	SharedData      map[string]interface{}
	mu              sync.RWMutex
}

// StageExecutionResult represents the result of stage execution
type StageExecutionResult struct {
	StageID        string
	Status         ValidationStatus
	StartTime      time.Time
	EndTime        time.Time
	Duration       time.Duration
	ValidationResults []*OrchestrationValidationResult
	Errors         []error
	Warnings       []string
	RetryCount     int
	TimeoutOccurred bool
	Skipped        bool
	SkipReason     string
	Metadata       map[string]interface{}
}

// NewPipelineExecutor creates a new pipeline executor
func NewPipelineExecutor(orchestrator *Orchestrator) *PipelineExecutor {
	return &PipelineExecutor{
		orchestrator:   orchestrator,
		retryManager:   NewRetryManager(),
		timeoutManager: NewTimeoutManager(5 * time.Minute),
		metrics:        NewPipelineMetrics(),
	}
}

// ExecuteStage executes a single validation stage
func (pe *PipelineExecutor) ExecuteStage(ctx context.Context, stage *ValidationStage, validationCtx *ValidationContext) (*StageResult, error) {
	startTime := time.Now()

	// Create pipeline context
	pipelineCtx := &PipelineContext{
		ExecutionID:   fmt.Sprintf("%s-%d", stage.ID, time.Now().UnixNano()),
		StageID:       stage.ID,
		ValidationCtx: validationCtx,
		ParentContext: ctx,
		Timeout:       pe.getStageTimeout(stage),
		RetryPolicy:   pe.getRetryPolicy(stage, validationCtx),
		Metadata:      make(map[string]interface{}),
		SharedData:    make(map[string]interface{}),
	}

	// Create stage result
	result := &StageResult{
		StageID:   stage.ID,
		Status:    Running,
		StartTime: startTime,
		Results:   make([]*OrchestrationValidationResult, 0),
		Errors:    make([]error, 0),
	}

	// Execute with retry logic
	err := pe.executeWithRetry(pipelineCtx, stage, result)

	// Update final result
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		result.Status = Failed
		result.Errors = append(result.Errors, err)
	} else if result.Status == Running {
		result.Status = Completed
	}

	// Update metrics
	pe.updateMetrics(stage.ID, result)

	return result, err
}

// executeWithRetry executes a stage with retry logic
func (pe *PipelineExecutor) executeWithRetry(pipelineCtx *PipelineContext, stage *ValidationStage, result *StageResult) error {
	var lastError error
	maxRetries := 0

	if pipelineCtx.RetryPolicy != nil {
		maxRetries = pipelineCtx.RetryPolicy.MaxRetries
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Apply backoff strategy
			backoffDuration := pe.calculateBackoff(pipelineCtx.RetryPolicy, attempt)
			time.Sleep(backoffDuration)
			result.RetryCount = attempt
		}

		// Create execution context with timeout
		execCtx, cancel := context.WithTimeout(pipelineCtx.ParentContext, pipelineCtx.Timeout)

		// Execute stage
		err := pe.executeSingleAttempt(execCtx, pipelineCtx, stage, result)
		cancel()

		if err == nil {
			return nil // Success
		}

		lastError = err

		// Check if error is retryable
		if !pe.isRetryableError(err, pipelineCtx.RetryPolicy) {
			break
		}

		// Check if we've exceeded max retries
		if attempt >= maxRetries {
			break
		}
	}

	return lastError
}

// executeSingleAttempt executes a single attempt of stage execution
func (pe *PipelineExecutor) executeSingleAttempt(ctx context.Context, pipelineCtx *PipelineContext, stage *ValidationStage, result *StageResult) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Execute validators in parallel or sequential based on stage configuration
	if stage.Parallel && len(stage.Validators) > 1 {
		return pe.executeValidatorsParallel(ctx, pipelineCtx, stage, result)
	}

	return pe.executeValidatorsSequential(ctx, pipelineCtx, stage, result)
}

// executeValidatorsParallel executes validators in parallel
func (pe *PipelineExecutor) executeValidatorsParallel(ctx context.Context, pipelineCtx *PipelineContext, stage *ValidationStage, result *StageResult) error {
	var wg sync.WaitGroup
	var resultMutex sync.Mutex
	
	// Track if any validation errors occurred
	hasErrors := false

	// Execute validators concurrently
	for _, validator := range stage.Validators {
		wg.Add(1)
		go func(v Validator) {
			defer wg.Done()

			valResult, err := pe.executeValidator(ctx, v, pipelineCtx)
			
			// Safely append results and errors with mutex protection
			resultMutex.Lock()
			defer resultMutex.Unlock()
			
			if err != nil {
				result.Errors = append(result.Errors, err)
				hasErrors = true
				return
			}

			result.Results = append(result.Results, valResult)
			if !valResult.IsValid {
				hasErrors = true
			}
		}(validator)
	}

	// Wait for completion
	wg.Wait()

	if hasErrors && stage.OnFailure == StopPipeline {
		return fmt.Errorf("validation failed in stage: %s", stage.ID)
	}

	return nil
}

// executeValidatorsSequential executes validators sequentially
func (pe *PipelineExecutor) executeValidatorsSequential(ctx context.Context, pipelineCtx *PipelineContext, stage *ValidationStage, result *StageResult) error {
	for _, validator := range stage.Validators {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		valResult, err := pe.executeValidator(ctx, validator, pipelineCtx)
		if err != nil {
			result.Errors = append(result.Errors, err)
			if stage.OnFailure == StopPipeline {
				return err
			}
			continue
		}

		result.Results = append(result.Results, valResult)

		// Stop on validation failure if configured
		if !valResult.IsValid && stage.OnFailure == StopPipeline {
			return fmt.Errorf("validation failed in stage: %s", stage.ID)
		}
	}

	return nil
}

// executeValidator executes a single validator
func (pe *PipelineExecutor) executeValidator(ctx context.Context, validator Validator, pipelineCtx *PipelineContext) (*OrchestrationValidationResult, error) {
	// Create validation context with thread-safe access to shared maps
	eventData := make(map[string]interface{})
	
	// Use pipeline context mutex to safely access shared properties
	pipelineCtx.mu.RLock()
	if pipelineCtx.ValidationCtx.Properties != nil {
		for k, v := range pipelineCtx.ValidationCtx.Properties {
			eventData[k] = v
		}
	}
	// Ensure event_type is available in EventData
	if pipelineCtx.ValidationCtx.EventType != "" {
		eventData["event_type"] = pipelineCtx.ValidationCtx.EventType
	}
	pipelineCtx.mu.RUnlock()
	
	valCtx := &OrchestrationValidationContext{
		EventData:   eventData,
		Properties:  pipelineCtx.ValidationCtx.Properties,
		Metadata:    pipelineCtx.ValidationCtx.Metadata,
		Source:      pipelineCtx.ValidationCtx.Source,
		Environment: pipelineCtx.ValidationCtx.Environment,
		Tags:        pipelineCtx.ValidationCtx.Tags,
	}

	// Execute validator with timeout
	done := make(chan struct{})
	var result *OrchestrationValidationResult
	var err error

	go func() {
		defer close(done)
		result, err = validator.Validate(valCtx)
	}()

	select {
	case <-done:
		return result, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// getStageTimeout determines the timeout for a stage
func (pe *PipelineExecutor) getStageTimeout(stage *ValidationStage) time.Duration {
	if stage.Timeout > 0 {
		return stage.Timeout
	}

	pe.timeoutManager.mu.RLock()
	defer pe.timeoutManager.mu.RUnlock()

	if timeout, exists := pe.timeoutManager.stageTimeouts[stage.ID]; exists {
		return timeout
	}

	return pe.timeoutManager.defaultTimeout
}

// getRetryPolicy determines the retry policy for a stage
func (pe *PipelineExecutor) getRetryPolicy(stage *ValidationStage, validationCtx *ValidationContext) *RetryPolicy {
	// Use context retry policy if available
	if validationCtx.RetryPolicy != nil {
		return validationCtx.RetryPolicy
	}

	pe.retryManager.mu.RLock()
	defer pe.retryManager.mu.RUnlock()

	// Use stage-specific retry policy
	if policy, exists := pe.retryManager.policies[stage.ID]; exists {
		return policy
	}

	// Default retry policy
	return &RetryPolicy{
		MaxRetries:      3,
		BackoffStrategy: ExponentialBackoff,
		RetryableErrors: []string{"timeout", "network", "temporary"},
	}
}

// calculateBackoff calculates backoff duration based on strategy
func (pe *PipelineExecutor) calculateBackoff(policy *RetryPolicy, attempt int) time.Duration {
	baseDelay := time.Second

	switch policy.BackoffStrategy {
	case LinearBackoff:
		return time.Duration(attempt) * baseDelay
	case ExponentialBackoff:
		return time.Duration(1<<uint(attempt)) * baseDelay
	case FixedBackoff:
		return baseDelay
	default:
		return baseDelay
	}
}

// isRetryableError checks if an error is retryable based on policy
func (pe *PipelineExecutor) isRetryableError(err error, policy *RetryPolicy) bool {
	if policy == nil || len(policy.RetryableErrors) == 0 {
		return false
	}

	errStr := err.Error()
	for _, retryable := range policy.RetryableErrors {
		if contains(errStr, retryable) {
			return true
		}
	}

	return false
}

// updateMetrics updates pipeline execution metrics
func (pe *PipelineExecutor) updateMetrics(stageID string, result *StageResult) {
	pe.metrics.mu.Lock()
	defer pe.metrics.mu.Unlock()

	// Update overall metrics
	pe.metrics.TotalExecutions++
	if result.Status == Completed {
		pe.metrics.SuccessfulRuns++
	} else {
		pe.metrics.FailedRuns++
	}

	// Update stage metrics
	if pe.metrics.StageMetrics == nil {
		pe.metrics.StageMetrics = make(map[string]*StageMetrics)
	}

	stageMetrics, exists := pe.metrics.StageMetrics[stageID]
	if !exists {
		stageMetrics = &StageMetrics{
			MinTime: result.Duration,
			MaxTime: result.Duration,
		}
		pe.metrics.StageMetrics[stageID] = stageMetrics
	}

	stageMetrics.ExecutionCount++
	if result.Status == Completed {
		stageMetrics.SuccessCount++
	} else {
		stageMetrics.FailureCount++
	}

	// Update timing metrics
	if result.Duration < stageMetrics.MinTime {
		stageMetrics.MinTime = result.Duration
	}
	if result.Duration > stageMetrics.MaxTime {
		stageMetrics.MaxTime = result.Duration
	}

	// Calculate average time
	stageMetrics.AverageTime = time.Duration(
		(int64(stageMetrics.AverageTime)*stageMetrics.ExecutionCount + int64(result.Duration)) /
		(stageMetrics.ExecutionCount + 1))

	stageMetrics.RetryCount += int64(result.RetryCount)
}

// GetStageMetrics returns metrics for a specific stage
func (pe *PipelineExecutor) GetStageMetrics(stageID string) *StageMetrics {
	pe.metrics.mu.RLock()
	defer pe.metrics.mu.RUnlock()

	if metrics, exists := pe.metrics.StageMetrics[stageID]; exists {
		// Return a copy to avoid race conditions
		return &StageMetrics{
			ExecutionCount: metrics.ExecutionCount,
			SuccessCount:   metrics.SuccessCount,
			FailureCount:   metrics.FailureCount,
			AverageTime:    metrics.AverageTime,
			MinTime:        metrics.MinTime,
			MaxTime:        metrics.MaxTime,
			RetryCount:     metrics.RetryCount,
			TimeoutCount:   metrics.TimeoutCount,
		}
	}

	return nil
}

// GetOverallMetrics returns overall pipeline metrics
func (pe *PipelineExecutor) GetOverallMetrics() *PipelineMetrics {
	pe.metrics.mu.RLock()
	defer pe.metrics.mu.RUnlock()

	// Calculate average execution time
	var avgTime time.Duration
	if pe.metrics.TotalExecutions > 0 {
		totalTime := time.Duration(0)
		for _, stageMetrics := range pe.metrics.StageMetrics {
			totalTime += time.Duration(stageMetrics.ExecutionCount) * stageMetrics.AverageTime
		}
		avgTime = totalTime / time.Duration(pe.metrics.TotalExecutions)
	}

	return &PipelineMetrics{
		TotalExecutions:      pe.metrics.TotalExecutions,
		SuccessfulRuns:       pe.metrics.SuccessfulRuns,
		FailedRuns:           pe.metrics.FailedRuns,
		AverageExecutionTime: avgTime,
	}
}

// SetStageTimeout sets a custom timeout for a specific stage
func (pe *PipelineExecutor) SetStageTimeout(stageID string, timeout time.Duration) {
	pe.timeoutManager.mu.Lock()
	defer pe.timeoutManager.mu.Unlock()

	if pe.timeoutManager.stageTimeouts == nil {
		pe.timeoutManager.stageTimeouts = make(map[string]time.Duration)
	}

	pe.timeoutManager.stageTimeouts[stageID] = timeout
}

// SetRetryPolicy sets a custom retry policy for a specific stage
func (pe *PipelineExecutor) SetRetryPolicy(stageID string, policy *RetryPolicy) {
	pe.retryManager.mu.Lock()
	defer pe.retryManager.mu.Unlock()

	if pe.retryManager.policies == nil {
		pe.retryManager.policies = make(map[string]*RetryPolicy)
	}

	pe.retryManager.policies[stageID] = policy
}

// NewRetryManager creates a new retry manager
func NewRetryManager() *RetryManager {
	return &RetryManager{
		policies: make(map[string]*RetryPolicy),
	}
}

// NewTimeoutManager creates a new timeout manager
func NewTimeoutManager(defaultTimeout time.Duration) *TimeoutManager {
	return &TimeoutManager{
		defaultTimeout: defaultTimeout,
		stageTimeouts:  make(map[string]time.Duration),
	}
}

// NewPipelineMetrics creates a new pipeline metrics instance
func NewPipelineMetrics() *PipelineMetrics {
	return &PipelineMetrics{
		StageMetrics: make(map[string]*StageMetrics),
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		 (len(s) > len(substr) && s[1:len(substr)+1] == substr))))
}