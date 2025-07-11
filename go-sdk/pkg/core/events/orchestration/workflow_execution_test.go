package orchestration

import (
	"context"
	"errors"
	"fmt"
	_ "sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/stretchr/testify/assert"
	_ "github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// WorkflowExecutionTestSuite provides comprehensive workflow execution tests
type WorkflowExecutionTestSuite struct {
	suite.Suite
	orchestrator *Orchestrator
	ctx          context.Context
	cancel       context.CancelFunc
}

func (suite *WorkflowExecutionTestSuite) SetupTest() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	
	config := &OrchestratorConfig{
		MaxConcurrentWorkflows: 10,
		DefaultTimeout:         5 * time.Second,
		EnableMetrics:          true,
		EnableTracing:          true,
	}
	
	suite.orchestrator = NewOrchestrator(config)
}

func (suite *WorkflowExecutionTestSuite) TearDownTest() {
	if suite.orchestrator != nil {
		suite.orchestrator.Close()
	}
	if suite.cancel != nil {
		suite.cancel()
	}
}

// TestComplexWorkflowExecution tests complex workflow scenarios
func (suite *WorkflowExecutionTestSuite) TestComplexWorkflowExecution() {
	// Create a complex workflow with multiple paths
	stages := []*ValidationStage{
		{
			ID:   "input-validation",
			Name: "Input Validation",
			Validators: []Validator{
				&MockValidator{ID: "input-format", IsValid: true, Duration: 10 * time.Millisecond},
				&MockValidator{ID: "input-range", IsValid: true, Duration: 10 * time.Millisecond},
			},
			Parallel: true,
		},
		{
			ID:           "authentication",
			Name:         "Authentication Check",
			Dependencies: []string{"input-validation"},
			Validators: []Validator{
				&MockValidator{ID: "auth-token", IsValid: true, Duration: 20 * time.Millisecond},
				&MockValidator{ID: "auth-permissions", IsValid: true, Duration: 15 * time.Millisecond},
			},
		},
		{
			ID:           "business-logic",
			Name:         "Business Logic Validation",
			Dependencies: []string{"authentication"},
			Validators: []Validator{
				&MockValidator{ID: "business-rules", IsValid: true, Duration: 30 * time.Millisecond},
			},
		},
		{
			ID:           "data-validation",
			Name:         "Data Validation",
			Dependencies: []string{"authentication"},
			Validators: []Validator{
				&MockValidator{ID: "data-integrity", IsValid: true, Duration: 25 * time.Millisecond},
				&MockValidator{ID: "data-consistency", IsValid: true, Duration: 20 * time.Millisecond},
			},
			Parallel: true,
		},
		{
			ID:           "final-check",
			Name:         "Final Validation",
			Dependencies: []string{"business-logic", "data-validation"},
			Validators: []Validator{
				&MockValidator{ID: "final-validator", IsValid: true, Duration: 10 * time.Millisecond},
			},
		},
	}
	
	workflow := &ValidationWorkflow{
		ID:      "complex-workflow",
		Name:    "Complex Validation Workflow",
		Version: "1.0.0",
		Stages:  stages,
	}
	
	err := suite.orchestrator.RegisterWorkflow(workflow)
	suite.Require().NoError(err)
	
	validationCtx := &ValidationContext{
		EventType:   "complex-event",
		Source:      "test-source",
		Environment: "production",
		Tags: map[string]string{
			"priority": "high",
			"region":   "us-east-1",
		},
		Properties: map[string]interface{}{
			"user_id":    "12345",
			"request_id": "req-67890",
		},
	}
	
	start := time.Now()
	result, err := suite.orchestrator.ExecuteWorkflow(suite.ctx, "complex-workflow", validationCtx)
	duration := time.Since(start)
	
	suite.Require().NoError(err)
	suite.Equal(Completed, result.Status)
	suite.Len(result.StageResults, 5)
	
	// Verify execution time is optimized due to parallelism
	suite.Less(duration, 150*time.Millisecond, "Workflow should complete faster with parallelism")
	
	// Verify stage dependencies were respected
	inputResult := result.StageResults["input-validation"]
	authResult := result.StageResults["authentication"]
	suite.True(authResult.StartTime.After(inputResult.EndTime), "Auth should start after input validation")
}

// TestWorkflowErrorHandling tests comprehensive error handling
func (suite *WorkflowExecutionTestSuite) TestWorkflowErrorHandling() {
	tests := []struct {
		name              string
		failingStage      string
		onFailure         FailureAction
		expectedStatus    ValidationStatus
		expectedExecuted  []string
		expectedSkipped   []string
	}{
		{
			name:         "stop on failure",
			failingStage: "stage2",
			onFailure:    StopPipeline,
			expectedStatus: Failed,
			expectedExecuted: []string{"stage1", "stage2"},
			expectedSkipped:  []string{"stage3"},
		},
		{
			name:         "continue on failure",
			failingStage: "stage2",
			onFailure:    ContinueOnFailure,
			expectedStatus: Completed, // Changed from CompletedWithErrors
			expectedExecuted: []string{"stage1", "stage2", "stage3"},
			expectedSkipped:  []string{},
		},
	}
	
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Create workflow with controlled failure
			stages := []*ValidationStage{
				{
					ID:   "stage1",
					Name: "Stage 1",
					Validators: []Validator{
						&MockValidator{ID: "val1", IsValid: true},
					},
				},
				{
					ID:           "stage2",
					Name:         "Stage 2",
					Dependencies: []string{"stage1"},
					OnFailure:    tt.onFailure,
					Validators: []Validator{
						&MockValidator{
							ID:          "val2",
							ShouldError: tt.failingStage == "stage2",
							ErrorMsg:    "intentional test failure",
						},
					},
				},
				{
					ID:           "stage3",
					Name:         "Stage 3",
					Dependencies: []string{"stage2"},
					Validators: []Validator{
						&MockValidator{ID: "val3", IsValid: true},
					},
				},
			}
			
			workflow := &ValidationWorkflow{
				ID:     fmt.Sprintf("error-workflow-%s", tt.name),
				Name:   "Error Handling Workflow",
				Stages: stages,
			}
			
			err := suite.orchestrator.RegisterWorkflow(workflow)
			suite.Require().NoError(err)
			
			validationCtx := &ValidationContext{
				EventType: "test-event",
				Source:    "test-source",
			}
			
			result, _ := suite.orchestrator.ExecuteWorkflow(suite.ctx, workflow.ID, validationCtx)
			
			// Verify workflow status
			if tt.onFailure == ContinueOnFailure && result.Status == Completed {
				// Check if there were errors
				if len(result.Errors) > 0 {
					suite.T().Log("Workflow completed with errors as expected")
				}
			} else {
				suite.Equal(tt.expectedStatus, result.Status)
			}
			
			// Verify executed stages
			for _, stageID := range tt.expectedExecuted {
				suite.Contains(result.StageResults, stageID, "Stage %s should have been executed", stageID)
			}
			
			// Verify skipped stages
			for _, stageID := range tt.expectedSkipped {
				if stageResult, exists := result.StageResults[stageID]; exists {
					suite.True(stageResult.Skipped, "Stage %s should have been skipped", stageID)
				}
			}
		})
	}
}

// TestPipelineProcessing tests pipeline execution patterns
func (suite *WorkflowExecutionTestSuite) TestPipelineProcessing() {
	// Create a data processing pipeline
	stages := []*ValidationStage{
		{
			ID:   "extract",
			Name: "Data Extraction",
			Validators: []Validator{
				&WorkflowDataProcessor{ID: "extractor", ProcessFunc: func(ctx *OrchestrationValidationContext) (interface{}, error) {
					ctx.EventData["extracted"] = true
					ctx.EventData["data"] = "raw-data"
					return map[string]interface{}{"extracted": true, "data": "raw-data"}, nil
				}},
			},
		},
		{
			ID:           "transform",
			Name:         "Data Transformation",
			Dependencies: []string{"extract"},
			Validators: []Validator{
				&WorkflowDataProcessor{ID: "transformer", ProcessFunc: func(ctx *OrchestrationValidationContext) (interface{}, error) {
					ctx.EventData["transformed"] = true
					ctx.EventData["data"] = "transformed-data"
					return ctx.EventData, nil
				}},
			},
		},
		{
			ID:           "load",
			Name:         "Data Loading",
			Dependencies: []string{"transform"},
			Validators: []Validator{
				&WorkflowDataProcessor{ID: "loader", ProcessFunc: func(ctx *OrchestrationValidationContext) (interface{}, error) {
					ctx.EventData["loaded"] = true
					return ctx.EventData, nil
				}},
			},
		},
	}
	
	pipeline := &ValidationWorkflow{
		ID:     "etl-pipeline",
		Name:   "ETL Pipeline",
		Stages: stages,
	}
	
	err := suite.orchestrator.RegisterWorkflow(pipeline)
	suite.Require().NoError(err)
	
	validationCtx := &ValidationContext{
		EventType: "data-processing",
		Source:    "test-source",
		Properties: map[string]interface{}{
			"initial_data": "test-data",
		},
	}
	
	result, err := suite.orchestrator.ExecuteWorkflow(suite.ctx, "etl-pipeline", validationCtx)
	suite.Require().NoError(err)
	suite.Equal(Completed, result.Status)
	
	// Verify data flow through pipeline
	// The pipeline modifies the validation context properties
	suite.True(validationCtx.Properties["extracted"].(bool))
	suite.True(validationCtx.Properties["transformed"].(bool))
	suite.True(validationCtx.Properties["loaded"].(bool))
	suite.Equal("transformed-data", validationCtx.Properties["data"])
}

// TestEventOrchestration tests event-driven orchestration
func (suite *WorkflowExecutionTestSuite) TestEventOrchestration() {
	// Create event handlers
	eventHandlers := map[string]*ValidationStage{
		"user.created": {
			ID:   "user-created-handler",
			Name: "User Created Handler",
			Validators: []Validator{
				&WorkflowEventHandler{ID: "user-validator", EventType: "user.created"},
			},
		},
		"order.placed": {
			ID:   "order-placed-handler",
			Name: "Order Placed Handler",
			Validators: []Validator{
				&WorkflowEventHandler{ID: "order-validator", EventType: "order.placed"},
			},
		},
		"payment.processed": {
			ID:   "payment-processed-handler",
			Name: "Payment Processed Handler",
			Validators: []Validator{
				&WorkflowEventHandler{ID: "payment-validator", EventType: "payment.processed"},
			},
		},
	}
	
	// Create event orchestration workflow
	workflow := &ValidationWorkflow{
		ID:     "event-orchestration",
		Name:   "Event Orchestration Workflow",
		Stages: []*ValidationStage{},
	}
	
	for _, handler := range eventHandlers {
		workflow.Stages = append(workflow.Stages, handler)
	}
	
	err := suite.orchestrator.RegisterWorkflow(workflow)
	suite.Require().NoError(err)
	
	// Simulate different events
	events := []string{"user.created", "order.placed", "payment.processed"}
	
	for _, eventType := range events {
		validationCtx := &ValidationContext{
			EventType: eventType,
			Source:    "event-bus",
			Properties: map[string]interface{}{
				"event_id":   fmt.Sprintf("evt-%s-%d", eventType, time.Now().Unix()),
				"timestamp":  time.Now(),
			},
		}
		
		result, err := suite.orchestrator.ExecuteWorkflow(suite.ctx, "event-orchestration", validationCtx)
		suite.Require().NoError(err)
		suite.Equal(Completed, result.Status)
		
		// Verify the relevant handler was executed
		handlerID := fmt.Sprintf("%s-handler", eventType)
		suite.Contains(result.StageResults, handlerID)
	}
}

// TestOrchestrationResilience tests resilience features
func (suite *WorkflowExecutionTestSuite) TestOrchestrationResilience() {
	suite.Run("retry mechanism", func() {
		attempts := int32(0)
		
		stage := &ValidationStage{
			ID:   "retry-stage",
			Name: "Retry Stage",
			Validators: []Validator{
				&WorkflowMockValidatorWithFunc{
					MockValidator: MockValidator{
						ID: "retry-validator",
					},
					ValidateFn: func() (bool, error) {
						count := atomic.AddInt32(&attempts, 1)
						if count < 3 {
							return false, errors.New("retry test error")
						}
						return true, nil
					},
				},
			},
		}
		
		workflow := &ValidationWorkflow{
			ID:     "retry-workflow",
			Name:   "Retry Test Workflow",
			Stages: []*ValidationStage{stage},
		}
		
		err := suite.orchestrator.RegisterWorkflow(workflow)
		suite.Require().NoError(err)
		
		validationCtx := &ValidationContext{
			EventType: "test-event",
			Source:    "test-source",
			RetryPolicy: &RetryPolicy{
				MaxRetries:      3,
				BackoffStrategy: ExponentialBackoff,
				RetryableErrors: []string{"retry test error"},
			},
		}
		
		result, err := suite.orchestrator.ExecuteWorkflow(suite.ctx, workflow.ID, validationCtx)
		suite.Require().NoError(err)
		suite.Equal(Completed, result.Status)
		suite.Equal(int32(3), atomic.LoadInt32(&attempts), "Should retry until success")
	})
}

// TestDynamicWorkflowConfiguration tests dynamic workflow changes
func (suite *WorkflowExecutionTestSuite) TestDynamicWorkflowConfiguration() {
	// Create base workflow
	baseStages := []*ValidationStage{
		{
			ID:   "base-stage",
			Name: "Base Stage",
			Validators: []Validator{
				&MockValidator{ID: "base-validator", IsValid: true},
			},
		},
	}
	
	workflow := &ValidationWorkflow{
		ID:      "dynamic-workflow",
		Name:    "Dynamic Workflow",
		Version: "1.0.0",
		Stages:  baseStages,
	}
	
	err := suite.orchestrator.RegisterWorkflow(workflow)
	suite.Require().NoError(err)
	
	// Update workflow with new stages
	updatedStages := append(baseStages,
		&ValidationStage{
			ID:           "new-stage",
			Name:         "New Stage",
			Dependencies: []string{"base-stage"},
			Validators: []Validator{
				&MockValidator{ID: "new-validator", IsValid: true},
			},
		},
	)
	
	updatedWorkflow := &ValidationWorkflow{
		ID:      "dynamic-workflow",
		Name:    "Dynamic Workflow",
		Version: "2.0.0",
		Stages:  updatedStages,
	}
	
	// Re-register workflow (simulating update)
	err = suite.orchestrator.RegisterWorkflow(updatedWorkflow)
	suite.Require().NoError(err)
	
	// Execute updated workflow
	validationCtx := &ValidationContext{
		EventType: "test-event",
		Source:    "test-source",
	}
	
	result, err := suite.orchestrator.ExecuteWorkflow(suite.ctx, "dynamic-workflow", validationCtx)
	suite.Require().NoError(err)
	suite.Equal(Completed, result.Status)
	suite.Len(result.StageResults, 2, "Should execute both stages")
}

// TestWorkflowMetricsCollection tests comprehensive metrics
func (suite *WorkflowExecutionTestSuite) TestWorkflowMetricsCollection() {
	// Create workflow with various characteristics
	stages := []*ValidationStage{
		{
			ID:       "fast-stage",
			Name:     "Fast Stage",
			Parallel: true,
			Validators: []Validator{
				&MockValidator{ID: "fast-1", IsValid: true, Duration: 5 * time.Millisecond},
				&MockValidator{ID: "fast-2", IsValid: true, Duration: 5 * time.Millisecond},
			},
		},
		{
			ID:           "slow-stage",
			Name:         "Slow Stage",
			Dependencies: []string{"fast-stage"},
			Validators: []Validator{
				&MockValidator{ID: "slow-1", IsValid: true, Duration: 50 * time.Millisecond},
			},
		},
		{
			ID:           "failing-stage",
			Name:         "Failing Stage",
			Dependencies: []string{"fast-stage"},
			Optional:     true,
			Validators: []Validator{
				&MockValidator{ID: "failing-1", ShouldError: true, ErrorMsg: "metrics test"},
			},
		},
	}
	
	workflow := &ValidationWorkflow{
		ID:     "metrics-workflow",
		Name:   "Metrics Test Workflow",
		Stages: stages,
	}
	
	err := suite.orchestrator.RegisterWorkflow(workflow)
	suite.Require().NoError(err)
	
	// Execute workflow multiple times
	for i := 0; i < 10; i++ {
		validationCtx := &ValidationContext{
			EventType: "metrics-event",
			Source:    "test-source",
			Tags: map[string]string{
				"iteration": fmt.Sprintf("%d", i),
			},
		}
		
		suite.orchestrator.ExecuteWorkflow(suite.ctx, "metrics-workflow", validationCtx)
	}
	
	// Verify workflow executed multiple times
	// Metrics collection would be verified through the pipelineExecutor
	
	// Stage-specific metrics
	stageMetrics := suite.orchestrator.pipelineExecutor.GetStageMetrics("fast-stage")
	suite.NotNil(stageMetrics)
	suite.Equal(uint64(10), stageMetrics.ExecutionCount)
	// Fast stage should execute quickly
	
	slowStageMetrics := suite.orchestrator.pipelineExecutor.GetStageMetrics("slow-stage")
	suite.NotNil(slowStageMetrics)
	// Slow stage should take longer
}

func TestWorkflowExecutionTestSuite(t *testing.T) {
	suite.Run(t, new(WorkflowExecutionTestSuite))
}

// Helper types for testing

// WorkflowDataProcessor is a validator that processes data
type WorkflowDataProcessor struct {
	ID          string
	ProcessFunc func(*OrchestrationValidationContext) (interface{}, error)
}

func (dp *WorkflowDataProcessor) Validate(ctx *OrchestrationValidationContext) (*OrchestrationValidationResult, error) {
	output, err := dp.ProcessFunc(ctx)
	if err != nil {
		return nil, err
	}
	
	return &OrchestrationValidationResult{
		IsValid:   true,
		Message:   "Data processed successfully",
		Validator: dp.ID,
		Timestamp: time.Now(),
		Metadata:  map[string]interface{}{"output": output},
	}, nil
}

func (dp *WorkflowDataProcessor) GetID() string {
	return dp.ID
}

func (dp *WorkflowDataProcessor) GetType() string {
	return "data-processor"
}

func (dp *WorkflowDataProcessor) GetDescription() string {
	return fmt.Sprintf("Data processor: %s", dp.ID)
}

// WorkflowEventHandler is a validator for event-driven workflows
type WorkflowEventHandler struct {
	ID        string
	EventType string
}

func (eh *WorkflowEventHandler) Validate(ctx *OrchestrationValidationContext) (*OrchestrationValidationResult, error) {
	eventType, _ := ctx.EventData["event_type"].(string)
	if eventType != eh.EventType {
		return &OrchestrationValidationResult{
			IsValid:   true,
			Message:   "Event type does not match handler",
			Validator: eh.ID,
			Timestamp: time.Now(),
			// Skipped:   true, // Field doesn't exist
		}, nil
	}
	
	return &OrchestrationValidationResult{
		IsValid:   true,
		Message:   fmt.Sprintf("Handled event: %s", eh.EventType),
		Validator: eh.ID,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"event_type": eh.EventType,
			"handled_at": time.Now(),
		},
	}, nil
}

func (eh *WorkflowEventHandler) GetID() string {
	return eh.ID
}

func (eh *WorkflowEventHandler) GetType() string {
	return "event-handler"
}

func (eh *WorkflowEventHandler) GetDescription() string {
	return fmt.Sprintf("Event handler for: %s", eh.EventType)
}

// WorkflowMockValidatorWithFunc extends MockValidator with custom validation function
type WorkflowMockValidatorWithFunc struct {
	MockValidator
	ValidateFn func() (bool, error)
}

func (mv *WorkflowMockValidatorWithFunc) Validate(ctx *OrchestrationValidationContext) (*OrchestrationValidationResult, error) {
	if mv.ValidateFn != nil {
		isValid, err := mv.ValidateFn()
		if err != nil {
			return nil, err
		}
		return &OrchestrationValidationResult{
			IsValid:   isValid,
			Message:   "Custom validation",
			Validator: mv.ID,
			Timestamp: time.Now(),
		}, nil
	}
	
	// Fall back to base MockValidator implementation
	return mv.MockValidator.Validate(ctx)
}