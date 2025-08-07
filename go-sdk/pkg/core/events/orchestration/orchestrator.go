package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ValidationContext provides context for validation execution
type ValidationContext struct {
	EventType   string
	Source      string
	Environment string
	Tags        map[string]string
	Properties  map[string]interface{}
	Metadata    map[string]interface{}
	Timeout     time.Duration
	RetryPolicy *RetryPolicy
}

// RetryPolicy defines retry behavior for failed validations
type RetryPolicy struct {
	MaxRetries      int
	BackoffStrategy BackoffStrategy
	RetryableErrors []string
}

// BackoffStrategy defines retry backoff behavior
type BackoffStrategy int

const (
	LinearBackoff BackoffStrategy = iota
	ExponentialBackoff
	FixedBackoff
)

// ValidationStage represents a single validation stage in the pipeline
type ValidationStage struct {
	ID           string
	Name         string
	Validators   []Validator
	Dependencies []string
	Conditions   []StageCondition
	Timeout      time.Duration
	Parallel     bool
	Optional     bool
	OnFailure    FailureAction
	Config       *StageConfig
}

// StageCondition defines when a stage should execute
type StageCondition struct {
	Type     ConditionType
	Field    string
	Operator ConditionOperator
	Value    interface{}
	Negated  bool
}

// ConditionType defines the type of condition
type ConditionType int

const (
	PropertyCondition ConditionType = iota
	TagCondition
	MetadataCondition
	ResultCondition
)

// ConditionOperator defines condition operators
type ConditionOperator int

const (
	Equals ConditionOperator = iota
	NotEquals
	Contains
	StartsWith
	EndsWith
	GreaterThan
	LessThan
	Exists
)

// FailureAction defines what to do when a stage fails
type FailureAction int

const (
	StopPipeline FailureAction = iota
	ContinueOnFailure
	RetryStage
	SkipToStage
)

// ValidationWorkflow defines a complete validation workflow
type ValidationWorkflow struct {
	ID          string
	Name        string
	Version     string
	Description string
	Stages      []*ValidationStage
	Hooks       *WorkflowHooks
	Config      *WorkflowConfig
}

// WorkflowHooks define lifecycle hooks
type WorkflowHooks struct {
	PreValidation  []Hook
	PostValidation []Hook
	OnSuccess      []Hook
	OnFailure      []Hook
	OnTimeout      []Hook
}

// Hook represents a callback function
type Hook func(ctx context.Context, result *ValidationResult) error

// WorkflowConfig provides workflow configuration
type WorkflowConfig struct {
	MaxConcurrency int
	DefaultTimeout time.Duration
	EnableMetrics  bool
	EnableTracing  bool
}

// ValidationResult represents the result of validation execution
type ValidationResult struct {
	WorkflowID   string
	ExecutionID  string
	Context      *ValidationContext
	StartTime    time.Time
	EndTime      time.Time
	Duration     time.Duration
	Status       ValidationStatus
	StageResults map[string]*StageResult
	Errors       []error
	Warnings     []string
	Metrics      *ExecutionMetrics
	Summary      *ResultSummary
	IsValid      bool // Add the missing field
}

// StageResult represents the result of a single stage
type StageResult struct {
	StageID    string
	Status     ValidationStatus
	StartTime  time.Time
	EndTime    time.Time
	Duration   time.Duration
	Results    []*OrchestrationValidationResult
	Errors     []error
	Skipped    bool
	SkipReason string
	RetryCount int
}

// ValidationStatus defines validation execution status
type ValidationStatus int

const (
	Pending ValidationStatus = iota
	Running
	Completed
	Failed
	Skipped
	Timeout
	Cancelled
)

// ExecutionMetrics provides execution metrics
type ExecutionMetrics struct {
	TotalStages      int
	CompletedStages  int
	FailedStages     int
	SkippedStages    int
	ParallelStages   int
	AverageStageTime time.Duration
	ValidationCount  int
	ErrorCount       int
	WarningCount     int
}

// ResultSummary provides a high-level summary
type ResultSummary struct {
	TotalValidations   int
	PassedValidations  int
	FailedValidations  int
	SkippedValidations int
	SuccessRate        float64
	Performance        string
}

// Orchestrator manages validation workflow execution
type Orchestrator struct {
	workflowEngine   *WorkflowEngine
	pipelineExecutor *PipelineExecutor
	stageManager     *StageManager
	workflows        map[string]*ValidationWorkflow
	executionHistory map[string]*ValidationResult
	config           *OrchestratorConfig
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
}

// OrchestratorConfig provides orchestrator configuration
type OrchestratorConfig struct {
	MaxConcurrentWorkflows int
	DefaultTimeout         time.Duration
	HistoryRetention       time.Duration
	EnableMetrics          bool
	EnableTracing          bool
	MetricsInterval        time.Duration
}

// NewOrchestrator creates a new validation orchestrator
func NewOrchestrator(config *OrchestratorConfig) *Orchestrator {
	if config == nil {
		config = &OrchestratorConfig{
			MaxConcurrentWorkflows: 10,
			DefaultTimeout:         5 * time.Minute,
			HistoryRetention:       24 * time.Hour,
			EnableMetrics:          true,
			EnableTracing:          false,
			MetricsInterval:        1 * time.Minute,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	orchestrator := &Orchestrator{
		workflows:        make(map[string]*ValidationWorkflow),
		executionHistory: make(map[string]*ValidationResult),
		config:           config,
		ctx:              ctx,
		cancel:           cancel,
	}

	orchestrator.workflowEngine = NewWorkflowEngine(orchestrator)
	orchestrator.pipelineExecutor = NewPipelineExecutor(orchestrator)
	orchestrator.stageManager = NewStageManager(orchestrator)

	return orchestrator
}

// RegisterWorkflow registers a validation workflow
func (o *Orchestrator) RegisterWorkflow(workflow *ValidationWorkflow) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if workflow.ID == "" {
		return fmt.Errorf("workflow ID cannot be empty")
	}

	// Validate workflow structure
	if err := o.validateWorkflow(workflow); err != nil {
		return fmt.Errorf("invalid workflow: %w", err)
	}

	o.workflows[workflow.ID] = workflow
	return nil
}

// GetWorkflow retrieves a registered workflow
func (o *Orchestrator) GetWorkflow(workflowID string) (*ValidationWorkflow, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	workflow, exists := o.workflows[workflowID]
	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	return workflow, nil
}

// ExecuteWorkflow executes a validation workflow
func (o *Orchestrator) ExecuteWorkflow(ctx context.Context, workflowID string, validationCtx *ValidationContext) (*ValidationResult, error) {
	workflow, err := o.GetWorkflow(workflowID)
	if err != nil {
		return nil, err
	}

	// Create execution context
	execCtx, cancel := context.WithTimeout(ctx, o.getTimeout(validationCtx, workflow))
	defer cancel()

	// Generate execution ID
	executionID := fmt.Sprintf("%s-%d", workflowID, time.Now().UnixNano())

	// Initialize result
	result := &ValidationResult{
		WorkflowID:   workflowID,
		ExecutionID:  executionID,
		Context:      validationCtx,
		StartTime:    time.Now(),
		Status:       Running,
		StageResults: make(map[string]*StageResult),
		Metrics:      &ExecutionMetrics{},
		Summary:      &ResultSummary{},
	}

	// Store execution
	o.mu.Lock()
	o.executionHistory[executionID] = result
	o.mu.Unlock()

	// Execute pre-validation hooks
	if workflow.Hooks != nil && len(workflow.Hooks.PreValidation) > 0 {
		for _, hook := range workflow.Hooks.PreValidation {
			if err := hook(execCtx, result); err != nil {
				result.Status = Failed
				result.Errors = append(result.Errors, fmt.Errorf("pre-validation hook failed: %w", err))
				return result, err
			}
		}
	}

	// Execute workflow
	err = o.workflowEngine.Execute(execCtx, workflow, validationCtx, result)

	// Update final result
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		result.Status = Failed
		result.Errors = append(result.Errors, err)
	} else if result.Status == Running {
		result.Status = Completed
	}

	// Calculate summary
	o.calculateSummary(result)

	// Execute post-validation hooks
	if workflow.Hooks != nil {
		hooks := workflow.Hooks.PostValidation
		if result.Status == Completed && len(workflow.Hooks.OnSuccess) > 0 {
			hooks = append(hooks, workflow.Hooks.OnSuccess...)
		} else if result.Status == Failed && len(workflow.Hooks.OnFailure) > 0 {
			hooks = append(hooks, workflow.Hooks.OnFailure...)
		}

		for _, hook := range hooks {
			if hookErr := hook(execCtx, result); hookErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("post-validation hook failed: %v", hookErr))
			}
		}
	}

	return result, err
}

// GetExecutionResult retrieves execution result by ID
func (o *Orchestrator) GetExecutionResult(executionID string) (*ValidationResult, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result, exists := o.executionHistory[executionID]
	if !exists {
		return nil, fmt.Errorf("execution result not found: %s", executionID)
	}

	return result, nil
}

// ListWorkflows returns all registered workflows
func (o *Orchestrator) ListWorkflows() []*ValidationWorkflow {
	o.mu.RLock()
	defer o.mu.RUnlock()

	workflows := make([]*ValidationWorkflow, 0, len(o.workflows))
	for _, workflow := range o.workflows {
		workflows = append(workflows, workflow)
	}

	return workflows
}

// validateWorkflow validates workflow structure and dependencies
func (o *Orchestrator) validateWorkflow(workflow *ValidationWorkflow) error {
	if len(workflow.Stages) == 0 {
		return fmt.Errorf("workflow must have at least one stage")
	}

	stageIDs := make(map[string]bool)
	for _, stage := range workflow.Stages {
		if stage.ID == "" {
			return fmt.Errorf("stage ID cannot be empty")
		}

		if stageIDs[stage.ID] {
			return fmt.Errorf("duplicate stage ID: %s", stage.ID)
		}
		stageIDs[stage.ID] = true

		// Validate dependencies
		for _, dep := range stage.Dependencies {
			if !stageIDs[dep] {
				// Check if dependency will be defined later
				found := false
				for _, futureStage := range workflow.Stages {
					if futureStage.ID == dep {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("stage %s has undefined dependency: %s", stage.ID, dep)
				}
			}
		}
	}

	// Check for circular dependencies
	if err := o.checkCircularDependencies(workflow.Stages); err != nil {
		return err
	}

	return nil
}

// checkCircularDependencies checks for circular dependencies in stages
func (o *Orchestrator) checkCircularDependencies(stages []*ValidationStage) error {
	// Build dependency graph
	graph := make(map[string][]string)
	for _, stage := range stages {
		graph[stage.ID] = stage.Dependencies
	}

	// DFS to detect cycles
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var hasCycle func(string) bool
	hasCycle = func(node string) bool {
		visited[node] = true
		recStack[node] = true

		for _, neighbor := range graph[node] {
			if !visited[neighbor] && hasCycle(neighbor) {
				return true
			} else if recStack[neighbor] {
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for _, stage := range stages {
		if !visited[stage.ID] && hasCycle(stage.ID) {
			return fmt.Errorf("circular dependency detected involving stage: %s", stage.ID)
		}
	}

	return nil
}

// getTimeout determines the appropriate timeout for execution
func (o *Orchestrator) getTimeout(validationCtx *ValidationContext, workflow *ValidationWorkflow) time.Duration {
	if validationCtx != nil && validationCtx.Timeout > 0 {
		return validationCtx.Timeout
	}

	if workflow.Config != nil && workflow.Config.DefaultTimeout > 0 {
		return workflow.Config.DefaultTimeout
	}

	return o.config.DefaultTimeout
}

// calculateSummary calculates execution summary
func (o *Orchestrator) calculateSummary(result *ValidationResult) {
	metrics := result.Metrics
	summary := result.Summary

	// Calculate stage metrics
	for _, stageResult := range result.StageResults {
		metrics.TotalStages++

		switch stageResult.Status {
		case Completed:
			metrics.CompletedStages++
		case Failed:
			metrics.FailedStages++
		case Skipped:
			metrics.SkippedStages++
		}

		// Count validations and results
		for _, valResult := range stageResult.Results {
			metrics.ValidationCount++
			summary.TotalValidations++

			if valResult.IsValid {
				summary.PassedValidations++
			} else {
				summary.FailedValidations++
				metrics.ErrorCount++
			}

			if valResult.Warnings != nil {
				metrics.WarningCount += len(valResult.Warnings)
			}
		}
	}

	// Calculate success rate
	if summary.TotalValidations > 0 {
		summary.SuccessRate = float64(summary.PassedValidations) / float64(summary.TotalValidations)
	}

	// Calculate average stage time
	if metrics.TotalStages > 0 {
		var totalTime time.Duration
		for _, stageResult := range result.StageResults {
			totalTime += stageResult.Duration
		}
		metrics.AverageStageTime = totalTime / time.Duration(metrics.TotalStages)
	}

	// Determine performance rating
	switch {
	case summary.SuccessRate >= 0.95:
		summary.Performance = "Excellent"
	case summary.SuccessRate >= 0.80:
		summary.Performance = "Good"
	case summary.SuccessRate >= 0.60:
		summary.Performance = "Fair"
	default:
		summary.Performance = "Poor"
	}
}

// Close shuts down the orchestrator
func (o *Orchestrator) Close() error {
	o.cancel()
	return nil
}
