package events_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/core/events/cache"
	"github.com/ag-ui/go-sdk/pkg/core/events/orchestration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// SystemIntegrationTestSuite tests the complete integrated system
type SystemIntegrationTestSuite struct {
	suite.Suite
	ctx              context.Context
	cancel           context.CancelFunc
	orchestrator     *orchestration.Orchestrator
	cacheValidator   *cache.CacheValidator
	eventValidator   *events.Validator
}

func (suite *SystemIntegrationTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	
	// Setup event validator
	validationConfig := events.DefaultValidationConfig()
	validationConfig.Strict = true
	suite.eventValidator = events.NewValidator(validationConfig)
	
	// Setup cache validator
	cacheConfig := cache.DefaultCacheValidatorConfig()
	cacheConfig.L1Size = 1000
	cacheConfig.L1TTL = 5 * time.Minute
	cacheConfig.Validator = suite.eventValidator
	cacheConfig.MetricsEnabled = true
	
	var err error
	suite.cacheValidator, err = cache.NewCacheValidator(cacheConfig)
	suite.Require().NoError(err)
	
	// Setup orchestrator
	orchestratorConfig := &orchestration.OrchestratorConfig{
		MaxConcurrentWorkflows: 20,
		DefaultTimeout:         10 * time.Second,
		EnableMetrics:          true,
		EnableTracing:          true,
	}
	suite.orchestrator = orchestration.NewOrchestrator(orchestratorConfig)
}

func (suite *SystemIntegrationTestSuite) TearDownSuite() {
	if suite.orchestrator != nil {
		suite.orchestrator.Close()
	}
	if suite.cacheValidator != nil {
		suite.cacheValidator.Shutdown(suite.ctx)
	}
	if suite.cancel != nil {
		suite.cancel()
	}
}

// TestEndToEndEventProcessing tests complete event processing pipeline
func (suite *SystemIntegrationTestSuite) TestEndToEndEventProcessing() {
	// Create a comprehensive validation workflow
	workflow := &orchestration.ValidationWorkflow{
		ID:      "event-processing-workflow",
		Name:    "Event Processing Pipeline",
		Version: "1.0.0",
		Stages: []*orchestration.ValidationStage{
			{
				ID:   "event-validation",
				Name: "Event Validation",
				Validators: []orchestration.Validator{
					&EventValidatorAdapter{
						validator: suite.eventValidator,
						cache:     suite.cacheValidator,
					},
				},
			},
			{
				ID:           "business-rules",
				Name:         "Business Rules Validation",
				Dependencies: []string{"event-validation"},
				Validators: []orchestration.Validator{
					&BusinessRulesValidator{},
				},
			},
			{
				ID:           "persistence",
				Name:         "Event Persistence",
				Dependencies: []string{"business-rules"},
				Validators: []orchestration.Validator{
					&PersistenceValidator{},
				},
			},
		},
	}
	
	err := suite.orchestrator.RegisterWorkflow(workflow)
	suite.Require().NoError(err)
	
	// Test various event types
	testEvents := []events.Event{
		events.NewRunStartedEvent("thread-1", "run-1"),
		events.NewToolCallStartEvent("tool-1", "ToolName"),
		events.NewToolCallEndEvent("tool-1"),
		events.NewRunFinishedEvent("thread-1", "run-1"),
	}
	
	for _, event := range testEvents {
		validationCtx := &orchestration.ValidationContext{
			EventType: string(event.Type()),
			Source:    "integration-test",
			Properties: map[string]interface{}{
				"event":     event,
				"thread_id": event.ThreadID(),
				"run_id":    event.RunID(),
			},
		}
		
		result, err := suite.orchestrator.ExecuteWorkflow(suite.ctx, workflow.ID, validationCtx)
		suite.NoError(err)
		suite.Equal(orchestration.Completed, result.Status)
		suite.True(result.IsValid)
	}
	
	// Verify cache effectiveness
	cacheStats := suite.cacheValidator.GetStats()
	suite.Greater(cacheStats.TotalHits, uint64(0), "Should have cache hits")
}

// TestHighLoadSystemPerformance tests system under high load
func (suite *SystemIntegrationTestSuite) TestHighLoadSystemPerformance() {
	// Create performance testing workflow
	workflow := &orchestration.ValidationWorkflow{
		ID:   "performance-workflow",
		Name: "Performance Test Workflow",
		Stages: []*orchestration.ValidationStage{
			{
				ID:       "parallel-validation",
				Name:     "Parallel Validation",
				Parallel: true,
				Validators: []orchestration.Validator{
					&EventValidatorAdapter{
						validator: suite.eventValidator,
						cache:     suite.cacheValidator,
					},
					&PerformanceValidator{duration: 5 * time.Millisecond},
				},
			},
		},
	}
	
	err := suite.orchestrator.RegisterWorkflow(workflow)
	suite.Require().NoError(err)
	
	// Generate load
	numWorkers := 50
	numEventsPerWorker := 100
	totalEvents := numWorkers * numEventsPerWorker
	
	startTime := time.Now()
	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex
	
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for i := 0; i < numEventsPerWorker; i++ {
				event := events.NewRunStartedEvent(
					fmt.Sprintf("thread-%d", workerID),
					fmt.Sprintf("run-%d", i),
				)
				
				validationCtx := &orchestration.ValidationContext{
					EventType: string(event.Type()),
					Source:    "load-test",
					Properties: map[string]interface{}{
						"event":     event,
						"worker_id": workerID,
					},
				}
				
				result, err := suite.orchestrator.ExecuteWorkflow(suite.ctx, workflow.ID, validationCtx)
				if err == nil && result.Status == orchestration.Completed {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}
		}(w)
	}
	
	wg.Wait()
	duration := time.Since(startTime)
	
	// Calculate metrics
	successRate := float64(successCount) / float64(totalEvents) * 100
	eventsPerSecond := float64(totalEvents) / duration.Seconds()
	
	suite.T().Logf("High Load Test Results:")
	suite.T().Logf("  Total Events: %d", totalEvents)
	suite.T().Logf("  Successful: %d (%.2f%%)", successCount, successRate)
	suite.T().Logf("  Duration: %v", duration)
	suite.T().Logf("  Events/sec: %.2f", eventsPerSecond)
	
	// Assertions
	suite.Greater(successRate, 95.0, "Should have high success rate")
	suite.Greater(eventsPerSecond, 500.0, "Should handle high throughput")
	
	// Check cache performance
	cacheStats := suite.cacheValidator.GetStats()
	cacheHitRate := float64(cacheStats.TotalHits) / float64(cacheStats.TotalHits+cacheStats.TotalMisses) * 100
	suite.Greater(cacheHitRate, 50.0, "Should have good cache hit rate")
	suite.T().Logf("  Cache Hit Rate: %.2f%%", cacheHitRate)
}

// TestEventSequenceValidation tests complex event sequences
func (suite *SystemIntegrationTestSuite) TestEventSequenceValidation() {
	// Create sequence validation workflow
	workflow := &orchestration.ValidationWorkflow{
		ID:   "sequence-workflow",
		Name: "Sequence Validation Workflow",
		Stages: []*orchestration.ValidationStage{
			{
				ID:   "sequence-validation",
				Name: "Sequence Validation",
				Validators: []orchestration.Validator{
					&SequenceValidator{
						eventValidator: suite.eventValidator,
						cacheValidator: suite.cacheValidator,
					},
				},
			},
		},
	}
	
	err := suite.orchestrator.RegisterWorkflow(workflow)
	suite.Require().NoError(err)
	
	// Test valid sequence
	validSequence := []events.Event{
		events.NewRunStartedEvent("thread-1", "run-1"),
		events.NewToolCallStartEvent("tool-1", "ToolName"),
		events.NewToolCallEndEvent("tool-1"),
		events.NewRunFinishedEvent("thread-1", "run-1"),
	}
	
	validationCtx := &orchestration.ValidationContext{
		EventType: "sequence",
		Source:    "sequence-test",
		Properties: map[string]interface{}{
			"events": validSequence,
		},
	}
	
	result, err := suite.orchestrator.ExecuteWorkflow(suite.ctx, workflow.ID, validationCtx)
	suite.NoError(err)
	suite.Equal(orchestration.Completed, result.Status)
	suite.True(result.IsValid)
	
	// Test invalid sequence (missing run start)
	invalidSequence := []events.Event{
		events.NewToolCallStartEvent("tool-1", "ToolName"),
		events.NewToolCallEndEvent("tool-1"),
		events.NewRunFinishedEvent("thread-1", "run-1"),
	}
	
	validationCtx = &orchestration.ValidationContext{
		EventType: "sequence",
		Source:    "sequence-test",
		Properties: map[string]interface{}{
			"events": invalidSequence,
		},
	}
	
	result, err = suite.orchestrator.ExecuteWorkflow(suite.ctx, workflow.ID, validationCtx)
	suite.Error(err)
	suite.Equal(orchestration.Failed, result.Status)
}

func TestSystemIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(SystemIntegrationTestSuite))
}

// Validator adapters and helpers

// EventValidatorAdapter adapts the event validator for orchestration
type EventValidatorAdapter struct {
	validator *events.Validator
	cache     *cache.CacheValidator
}

func (eva *EventValidatorAdapter) Validate(ctx *orchestration.OrchestrationValidationContext) (*orchestration.OrchestrationValidationResult, error) {
	event, ok := ctx.Properties["event"].(events.Event)
	if !ok {
		return nil, fmt.Errorf("missing or invalid event in context")
	}
	
	// Try cache first
	err := eva.cache.ValidateEvent(context.Background(), event)
	
	return &orchestration.OrchestrationValidationResult{
		IsValid:   err == nil,
		Message:   "Event validation completed",
		Validator: "event-validator",
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"event_type": event.Type(),
			"cached":     err == nil,
		},
	}, err
}

func (eva *EventValidatorAdapter) GetID() string {
	return "event-validator-adapter"
}

func (eva *EventValidatorAdapter) GetType() string {
	return "event-validation"
}

func (eva *EventValidatorAdapter) GetDescription() string {
	return "Event validator with caching"
}

// BusinessRulesValidator validates business rules
type BusinessRulesValidator struct{}

func (brv *BusinessRulesValidator) Validate(ctx *orchestration.OrchestrationValidationContext) (*orchestration.OrchestrationValidationResult, error) {
	// Simulate business rule validation
	event, ok := ctx.Properties["event"].(events.Event)
	if !ok {
		return nil, fmt.Errorf("missing event in context")
	}
	
	// Example business rule: runs must have valid thread IDs
	if event.ThreadID() == "" {
		return nil, fmt.Errorf("invalid thread ID")
	}
	
	return &orchestration.OrchestrationValidationResult{
		IsValid:   true,
		Message:   "Business rules passed",
		Validator: "business-rules",
		Timestamp: time.Now(),
	}, nil
}

func (brv *BusinessRulesValidator) GetID() string { return "business-rules-validator" }
func (brv *BusinessRulesValidator) GetType() string { return "business-rules" }
func (brv *BusinessRulesValidator) GetDescription() string { return "Business rules validator" }

// PersistenceValidator simulates event persistence
type PersistenceValidator struct{}

func (pv *PersistenceValidator) Validate(ctx *orchestration.OrchestrationValidationContext) (*orchestration.OrchestrationValidationResult, error) {
	// Simulate persistence
	time.Sleep(2 * time.Millisecond)
	
	return &orchestration.OrchestrationValidationResult{
		IsValid:   true,
		Message:   "Event persisted",
		Validator: "persistence",
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"persisted_at": time.Now(),
		},
	}, nil
}

func (pv *PersistenceValidator) GetID() string { return "persistence-validator" }
func (pv *PersistenceValidator) GetType() string { return "persistence" }
func (pv *PersistenceValidator) GetDescription() string { return "Event persistence validator" }

// PerformanceValidator for performance testing
type PerformanceValidator struct {
	duration time.Duration
}

func (pv *PerformanceValidator) Validate(ctx *orchestration.OrchestrationValidationContext) (*orchestration.OrchestrationValidationResult, error) {
	time.Sleep(pv.duration)
	
	return &orchestration.OrchestrationValidationResult{
		IsValid:   true,
		Message:   "Performance test",
		Validator: "performance",
		Timestamp: time.Now(),
	}, nil
}

func (pv *PerformanceValidator) GetID() string { return "performance-validator" }
func (pv *PerformanceValidator) GetType() string { return "performance" }
func (pv *PerformanceValidator) GetDescription() string { return "Performance test validator" }

// SequenceValidator validates event sequences
type SequenceValidator struct {
	eventValidator *events.Validator
	cacheValidator *cache.CacheValidator
}

func (sv *SequenceValidator) Validate(ctx *orchestration.OrchestrationValidationContext) (*orchestration.OrchestrationValidationResult, error) {
	events, ok := ctx.Properties["events"].([]events.Event)
	if !ok {
		return nil, fmt.Errorf("missing or invalid events in context")
	}
	
	// Validate sequence
	err := sv.cacheValidator.ValidateSequence(context.Background(), events)
	
	return &orchestration.OrchestrationValidationResult{
		IsValid:   err == nil,
		Message:   "Sequence validation completed",
		Validator: "sequence-validator",
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"event_count": len(events),
		},
	}, err
}

func (sv *SequenceValidator) GetID() string { return "sequence-validator" }
func (sv *SequenceValidator) GetType() string { return "sequence-validation" }
func (sv *SequenceValidator) GetDescription() string { return "Event sequence validator" }