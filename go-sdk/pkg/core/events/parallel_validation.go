package events

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// ParallelValidationConfig configures parallel validation behavior
type ParallelValidationConfig struct {
	// MaxGoroutines sets the maximum number of goroutines for parallel rule execution
	// If 0 or negative, defaults to runtime.NumCPU()
	MaxGoroutines int

	// EnableParallelExecution enables or disables parallel validation
	// When false, rules execute sequentially for compatibility
	EnableParallelExecution bool

	// MinRulesForParallel sets the minimum number of rules required to enable parallel execution
	// Below this threshold, sequential execution is used to avoid goroutine overhead
	MinRulesForParallel int

	// ValidationTimeout sets the maximum time allowed for validation
	// Individual rule timeouts are derived from this value
	ValidationTimeout time.Duration

	// BufferSize sets the buffer size for result channels
	// Larger buffers can improve throughput but use more memory
	BufferSize int

	// EnableDependencyAnalysis enables automatic rule dependency analysis
	// When true, the system analyzes rule dependencies to optimize parallel execution
	EnableDependencyAnalysis bool

	// StopOnFirstError determines whether to stop validation on the first error
	// When true, parallel execution stops immediately when any rule fails
	StopOnFirstError bool
}

// DefaultParallelValidationConfig returns the default configuration for parallel validation
func DefaultParallelValidationConfig() *ParallelValidationConfig {
	return &ParallelValidationConfig{
		MaxGoroutines:            runtime.NumCPU(),
		EnableParallelExecution:  true,
		MinRulesForParallel:      ParallelRuleThreshold,
		ValidationTimeout:        ValidationTimeoutDefault,
		BufferSize:               10,
		EnableDependencyAnalysis: true,
		StopOnFirstError:        false,
	}
}

// RuleDependencyType represents the type of dependency a rule has
type RuleDependencyType int

const (
	// DependencyNone means the rule has no dependencies and can run independently
	DependencyNone RuleDependencyType = iota
	
	// DependencyStateRead means the rule reads from validation state
	DependencyStateRead
	
	// DependencyStateWrite means the rule modifies validation state
	DependencyStateWrite
	
	// DependencySequence means the rule depends on event sequence/ordering
	DependencySequence
	
	// DependencyIDTracking means the rule depends on ID uniqueness tracking
	DependencyIDTracking
)

func (d RuleDependencyType) String() string {
	switch d {
	case DependencyNone:
		return "NONE"
	case DependencyStateRead:
		return "STATE_READ"
	case DependencyStateWrite:
		return "STATE_WRITE"
	case DependencySequence:
		return "SEQUENCE"
	case DependencyIDTracking:
		return "ID_TRACKING"
	default:
		return "UNKNOWN"
	}
}

// RuleDependency describes a rule's dependencies for parallel execution analysis
type RuleDependency struct {
	RuleID           string
	DependencyType   RuleDependencyType
	RequiredRules    []string // Rules that must execute before this rule
	ConflictingRules []string // Rules that cannot execute concurrently with this rule
	CanRunInParallel bool     // Whether this rule can run in parallel with other rules
}

// RuleDependencyAnalyzer analyzes rule dependencies to determine parallel execution strategy
type RuleDependencyAnalyzer struct {
	dependencies map[string]*RuleDependency
	mutex        sync.RWMutex
}

// NewRuleDependencyAnalyzer creates a new rule dependency analyzer
func NewRuleDependencyAnalyzer() *RuleDependencyAnalyzer {
	analyzer := &RuleDependencyAnalyzer{
		dependencies: make(map[string]*RuleDependency),
	}
	
	// Initialize with known rule dependencies
	analyzer.initializeKnownDependencies()
	
	return analyzer
}

// initializeKnownDependencies sets up dependency information for built-in rules
func (a *RuleDependencyAnalyzer) initializeKnownDependencies() {
	// Independent rules that can run in parallel
	independentRules := []string{
		"MESSAGE_CONTENT",
		"TOOL_CALL_CONTENT", 
		"CONTENT_VALIDATION",
		"TIMESTAMP_VALIDATION",
		"ID_FORMAT",
		"CUSTOM_EVENT",
	}
	
	for _, ruleID := range independentRules {
		a.dependencies[ruleID] = &RuleDependency{
			RuleID:           ruleID,
			DependencyType:   DependencyNone,
			RequiredRules:    []string{},
			ConflictingRules: []string{},
			CanRunInParallel: true,
		}
	}
	
	// State-dependent rules that require sequential execution
	stateDependentRules := map[string]RuleDependencyType{
		"RUN_LIFECYCLE":        DependencyStateWrite,
		"EVENT_ORDERING":       DependencySequence,
		"EVENT_SEQUENCE":       DependencyStateWrite,
		"MESSAGE_LIFECYCLE":    DependencyStateWrite,
		"TOOL_CALL_LIFECYCLE":  DependencyStateWrite,
		"MESSAGE_NESTING":      DependencyStateRead,
		"TOOL_CALL_NESTING":    DependencyStateRead,
		"ID_CONSISTENCY":       DependencyIDTracking,
		"ID_UNIQUENESS":        DependencyIDTracking,
		"STATE_VALIDATION":     DependencyStateRead,
		"STATE_CONSISTENCY":    DependencyStateRead,
	}
	
	for ruleID, depType := range stateDependentRules {
		conflictingRules := []string{}
		
		// Rules that write to state conflict with each other
		if depType == DependencyStateWrite {
			for otherRuleID, otherDepType := range stateDependentRules {
				if otherRuleID != ruleID && (otherDepType == DependencyStateWrite || otherDepType == DependencyStateRead) {
					conflictingRules = append(conflictingRules, otherRuleID)
				}
			}
		}
		
		a.dependencies[ruleID] = &RuleDependency{
			RuleID:           ruleID,
			DependencyType:   depType,
			RequiredRules:    []string{},
			ConflictingRules: conflictingRules,
			CanRunInParallel: false,
		}
	}
}

// AnalyzeRules analyzes a set of rules and returns parallel execution groups
func (a *RuleDependencyAnalyzer) AnalyzeRules(rules []ValidationRule) (independentRules []ValidationRule, dependentRules []ValidationRule) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	
	independent := make([]ValidationRule, 0)
	dependent := make([]ValidationRule, 0)
	
	for _, rule := range rules {
		if !rule.IsEnabled() {
			continue
		}
		
		ruleID := rule.ID()
		if dep, exists := a.dependencies[ruleID]; exists {
			if dep.CanRunInParallel {
				independent = append(independent, rule)
			} else {
				dependent = append(dependent, rule)
			}
		} else {
			// Unknown rule - assume it's dependent for safety
			dependent = append(dependent, rule)
		}
	}
	
	return independent, dependent
}

// AddRuleDependency adds or updates dependency information for a rule
func (a *RuleDependencyAnalyzer) AddRuleDependency(dependency *RuleDependency) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.dependencies[dependency.RuleID] = dependency
}

// GetRuleDependency retrieves dependency information for a rule
func (a *RuleDependencyAnalyzer) GetRuleDependency(ruleID string) (*RuleDependency, bool) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	dep, exists := a.dependencies[ruleID]
	return dep, exists
}

// ParallelValidationResult aggregates results from parallel rule execution
type ParallelValidationResult struct {
	*ValidationResult
	ParallelExecutionTime time.Duration
	SequentialExecutionTime time.Duration
	RulesExecutedInParallel int
	RulesExecutedSequentially int
	GoroutinesUsed int
}

// ParallelValidator executes validation rules in parallel when possible
type ParallelValidator struct {
	config            *ParallelValidationConfig
	dependencyAnalyzer *RuleDependencyAnalyzer
	workerPool        *ParallelWorkerPool
	metrics           *ParallelValidationMetrics
	mutex             sync.RWMutex
}

// ParallelValidationMetrics tracks metrics for parallel validation
type ParallelValidationMetrics struct {
	TotalValidations       int64
	ParallelValidations    int64
	SequentialValidations  int64
	AverageParallelTime    time.Duration
	AverageSequentialTime  time.Duration
	AverageSpeedup         float64
	GoroutinesUsedTotal    int64
	RulesExecutedParallel  int64
	RulesExecutedSequential int64
	
	mutex sync.RWMutex
}

// NewParallelValidationMetrics creates new parallel validation metrics
func NewParallelValidationMetrics() *ParallelValidationMetrics {
	return &ParallelValidationMetrics{}
}

// RecordParallelValidation records metrics for a parallel validation
func (m *ParallelValidationMetrics) RecordParallelValidation(duration time.Duration, goroutinesUsed int, rulesExecuted int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.TotalValidations++
	m.ParallelValidations++
	m.GoroutinesUsedTotal += int64(goroutinesUsed)
	m.RulesExecutedParallel += int64(rulesExecuted)
	
	// Update average parallel time
	if m.ParallelValidations == 1 {
		m.AverageParallelTime = duration
	} else {
		m.AverageParallelTime = time.Duration(
			(int64(m.AverageParallelTime)*(m.ParallelValidations-1) + int64(duration)) / m.ParallelValidations,
		)
	}
	
	// Update speedup calculation
	m.updateSpeedup()
}

// RecordSequentialValidation records metrics for a sequential validation
func (m *ParallelValidationMetrics) RecordSequentialValidation(duration time.Duration, rulesExecuted int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.TotalValidations++
	m.SequentialValidations++
	m.RulesExecutedSequential += int64(rulesExecuted)
	
	// Update average sequential time
	if m.SequentialValidations == 1 {
		m.AverageSequentialTime = duration
	} else {
		m.AverageSequentialTime = time.Duration(
			(int64(m.AverageSequentialTime)*(m.SequentialValidations-1) + int64(duration)) / m.SequentialValidations,
		)
	}
	
	// Update speedup calculation
	m.updateSpeedup()
}

// updateSpeedup calculates the average speedup from parallel execution
func (m *ParallelValidationMetrics) updateSpeedup() {
	if m.AverageParallelTime > 0 && m.AverageSequentialTime > 0 {
		m.AverageSpeedup = float64(m.AverageSequentialTime) / float64(m.AverageParallelTime)
	}
}

// GetMetrics returns a copy of the current metrics
func (m *ParallelValidationMetrics) GetMetrics() *ParallelValidationMetrics {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	return &ParallelValidationMetrics{
		TotalValidations:        m.TotalValidations,
		ParallelValidations:     m.ParallelValidations,
		SequentialValidations:   m.SequentialValidations,
		AverageParallelTime:     m.AverageParallelTime,
		AverageSequentialTime:   m.AverageSequentialTime,
		AverageSpeedup:          m.AverageSpeedup,
		GoroutinesUsedTotal:     m.GoroutinesUsedTotal,
		RulesExecutedParallel:   m.RulesExecutedParallel,
		RulesExecutedSequential: m.RulesExecutedSequential,
	}
}

// ParallelWorkerPool manages a pool of worker goroutines for parallel rule execution
type ParallelWorkerPool struct {
	workerCount int
	jobCh       chan ParallelValidationJob
	resultCh    chan ParallelValidationJobResult
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// ParallelValidationJob represents a validation task for a worker
type ParallelValidationJob struct {
	Rule    ValidationRule
	Event   Event
	Context *ValidationContext
	JobID   int
}

// ParallelValidationJobResult represents the result of a validation job
type ParallelValidationJobResult struct {
	JobID    int
	Result   *ValidationResult
	Duration time.Duration
	Error    error
}

// NewParallelWorkerPool creates a new worker pool with the specified number of workers
func NewParallelWorkerPool(workerCount int, bufferSize int) *ParallelWorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ParallelWorkerPool{
		workerCount: workerCount,
		jobCh:       make(chan ParallelValidationJob, bufferSize),
		resultCh:    make(chan ParallelValidationJobResult, bufferSize),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start starts the worker pool
func (wp *ParallelWorkerPool) Start() {
	for i := 0; i < wp.workerCount; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Stop stops the worker pool and waits for all workers to finish
func (wp *ParallelWorkerPool) Stop() {
	wp.cancel()
	close(wp.jobCh)
	wp.wg.Wait()
	close(wp.resultCh)
}

// worker is the worker goroutine function
func (wp *ParallelWorkerPool) worker(workerID int) {
	defer wp.wg.Done()
	
	for {
		select {
		case <-wp.ctx.Done():
			return
		case job, ok := <-wp.jobCh:
			if !ok {
				return
			}
			
			start := time.Now()
			result := job.Rule.Validate(job.Event, job.Context)
			duration := time.Since(start)
			
			wp.resultCh <- ParallelValidationJobResult{
				JobID:    job.JobID,
				Result:   result,
				Duration: duration,
				Error:    nil,
			}
		}
	}
}

// SubmitJob submits a validation job to the worker pool
func (wp *ParallelWorkerPool) SubmitJob(job ParallelValidationJob) error {
	select {
	case wp.jobCh <- job:
		return nil
	case <-wp.ctx.Done():
		return fmt.Errorf("worker pool is shutting down")
	default:
		return fmt.Errorf("worker pool job queue is full")
	}
}

// GetResult retrieves a validation result from the worker pool
func (wp *ParallelWorkerPool) GetResult() (ParallelValidationJobResult, error) {
	select {
	case result, ok := <-wp.resultCh:
		if !ok {
			return ParallelValidationJobResult{}, fmt.Errorf("result channel is closed")
		}
		return result, nil
	case <-wp.ctx.Done():
		return ParallelValidationJobResult{}, fmt.Errorf("worker pool is shutting down")
	}
}

// NewParallelValidator creates a new parallel validator
func NewParallelValidator(config *ParallelValidationConfig) *ParallelValidator {
	if config == nil {
		config = DefaultParallelValidationConfig()
	}
	
	return &ParallelValidator{
		config:             config,
		dependencyAnalyzer: NewRuleDependencyAnalyzer(),
		metrics:            NewParallelValidationMetrics(),
	}
}

// ValidateEventParallel validates an event using parallel rule execution when possible
func (pv *ParallelValidator) ValidateEventParallel(ctx context.Context, event Event, rules []ValidationRule, validationContext *ValidationContext) *ParallelValidationResult {
	start := time.Now()
	
	result := &ParallelValidationResult{
		ValidationResult: &ValidationResult{
			IsValid:    true,
			Errors:     make([]*ValidationError, 0),
			Warnings:   make([]*ValidationError, 0),
			Information: make([]*ValidationError, 0),
			EventCount: 1,
			Timestamp:  time.Now(),
		},
		GoroutinesUsed: 1, // Default to 1 for sequential execution
	}

	if event == nil {
		result.AddError(&ValidationError{
			RuleID:    "NULL_EVENT",
			Message:   "Event cannot be nil",
			Severity:  ValidationSeverityError,
			Timestamp: time.Now(),
		})
		return result
	}

	// Check if parallel execution is enabled and we have enough rules
	enabledRules := make([]ValidationRule, 0, len(rules))
	for _, rule := range rules {
		if rule.IsEnabled() {
			enabledRules = append(enabledRules, rule)
		}
	}

	if !pv.config.EnableParallelExecution || len(enabledRules) < pv.config.MinRulesForParallel {
		// Execute sequentially
		result.RulesExecutedSequentially = len(enabledRules)
		seqStart := time.Now()
		
		for _, rule := range enabledRules {
			ruleResult := rule.Validate(event, validationContext)
			if ruleResult != nil {
				// Merge results
				for _, err := range ruleResult.Errors {
					result.AddError(err)
				}
				for _, warning := range ruleResult.Warnings {
					result.AddWarning(warning)
				}
				for _, info := range ruleResult.Information {
					result.AddInfo(info)
				}
			}
		}
		
		result.SequentialExecutionTime = time.Since(seqStart)
		pv.metrics.RecordSequentialValidation(result.SequentialExecutionTime, len(enabledRules))
		result.Duration = time.Since(start)
		return result
	}

	// Analyze rule dependencies for parallel execution
	independentRules, dependentRules := pv.dependencyAnalyzer.AnalyzeRules(enabledRules)

	// Execute dependent rules sequentially first
	if len(dependentRules) > 0 {
		seqStart := time.Now()
		for _, rule := range dependentRules {
			ruleResult := rule.Validate(event, validationContext)
			if ruleResult != nil {
				// Merge results
				for _, err := range ruleResult.Errors {
					result.AddError(err)
					if pv.config.StopOnFirstError {
						result.SequentialExecutionTime = time.Since(seqStart)
						result.RulesExecutedSequentially = len(dependentRules)
						result.Duration = time.Since(start)
						return result
					}
				}
				for _, warning := range ruleResult.Warnings {
					result.AddWarning(warning)
				}
				for _, info := range ruleResult.Information {
					result.AddInfo(info)
				}
			}
		}
		result.SequentialExecutionTime = time.Since(seqStart)
		result.RulesExecutedSequentially = len(dependentRules)
	}

	// Execute independent rules in parallel if we have any
	if len(independentRules) > 0 {
		parallelStart := time.Now()
		
		// Create worker pool for parallel execution
		workerCount := pv.config.MaxGoroutines
		if workerCount <= 0 {
			workerCount = runtime.NumCPU()
		}
		if workerCount > len(independentRules) {
			workerCount = len(independentRules)
		}
		
		workerPool := NewParallelWorkerPool(workerCount, pv.config.BufferSize)
		workerPool.Start()
		
		// Submit jobs
		for i, rule := range independentRules {
			job := ParallelValidationJob{
				Rule:    rule,
				Event:   event,
				Context: validationContext,
				JobID:   i,
			}
			
			if err := workerPool.SubmitJob(job); err != nil {
				// Fallback to sequential execution for this rule
				ruleResult := rule.Validate(event, validationContext)
				if ruleResult != nil {
					for _, err := range ruleResult.Errors {
						result.AddError(err)
					}
					for _, warning := range ruleResult.Warnings {
						result.AddWarning(warning)
					}
					for _, info := range ruleResult.Information {
						result.AddInfo(info)
					}
				}
			}
		}
		
		// Collect results
		resultsCollected := 0
		for resultsCollected < len(independentRules) {
			select {
			case <-ctx.Done():
				workerPool.Stop()
				result.AddError(&ValidationError{
					RuleID:    "CONTEXT_CANCELLED",
					Message:   "Parallel validation cancelled by context",
					Severity:  ValidationSeverityError,
					Timestamp: time.Now(),
				})
				result.Duration = time.Since(start)
				return result
				
			default:
				jobResult, err := workerPool.GetResult()
				if err != nil {
					continue
				}
				
				if jobResult.Result != nil {
					// Merge results
					for _, err := range jobResult.Result.Errors {
						result.AddError(err)
						if pv.config.StopOnFirstError {
							workerPool.Stop()
							result.ParallelExecutionTime = time.Since(parallelStart)
							result.RulesExecutedInParallel = resultsCollected + 1
							result.GoroutinesUsed = workerCount
							result.Duration = time.Since(start)
							return result
						}
					}
					for _, warning := range jobResult.Result.Warnings {
						result.AddWarning(warning)
					}
					for _, info := range jobResult.Result.Information {
						result.AddInfo(info)
					}
				}
				
				resultsCollected++
			}
		}
		
		workerPool.Stop()
		result.ParallelExecutionTime = time.Since(parallelStart)
		result.RulesExecutedInParallel = len(independentRules)
		result.GoroutinesUsed = workerCount
		
		// Record metrics
		pv.metrics.RecordParallelValidation(result.ParallelExecutionTime, workerCount, len(independentRules))
	}

	result.Duration = time.Since(start)
	return result
}

// GetMetrics returns the parallel validation metrics
func (pv *ParallelValidator) GetMetrics() *ParallelValidationMetrics {
	return pv.metrics.GetMetrics()
}

// GetConfig returns the parallel validation configuration
func (pv *ParallelValidator) GetConfig() *ParallelValidationConfig {
	pv.mutex.RLock()
	defer pv.mutex.RUnlock()
	
	// Return a copy to prevent external modification
	configCopy := *pv.config
	return &configCopy
}

// UpdateConfig updates the parallel validation configuration
func (pv *ParallelValidator) UpdateConfig(config *ParallelValidationConfig) {
	pv.mutex.Lock()
	defer pv.mutex.Unlock()
	
	if config != nil {
		pv.config = config
	}
}