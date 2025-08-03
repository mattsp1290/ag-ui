# Validation Orchestration Engine

A comprehensive validation orchestration engine for the AG-UI Go SDK that provides DAG-based workflow execution and context-aware validation routing.

## Features

### Core Components

1. **Orchestrator** (`orchestrator.go`)
   - Validation workflow definition and execution
   - Conditional validation based on context
   - Validation pipeline orchestration
   - Custom validation stages and hooks
   - Validation result aggregation and reporting

2. **Workflow Engine** (`workflow_engine.go`)
   - DAG-based workflow execution
   - Automatic dependency resolution
   - Parallel and sequential execution
   - Critical path analysis
   - Execution planning and optimization

3. **Pipeline Executor** (`pipeline_executor.go`)
   - Stage execution management
   - Retry logic with configurable policies
   - Timeout handling
   - Parallel and sequential validator execution
   - Performance metrics collection

4. **Stage Manager** (`stage_manager.go`)
   - Validation stage coordination
   - Stage templates and groups
   - Resource management (circuit breakers, rate limiting)
   - Health checking
   - Priority-based scheduling

5. **Validation Framework** (`validation.go`)
   - Pluggable validator interface
   - Context-aware validation
   - Result aggregation
   - Error handling and reporting

## Key Features

### DAG-Based Workflow Execution
- Automatic dependency resolution
- Parallel execution where possible
- Topological sorting for optimal execution order
- Critical path identification

### Context-Aware Validation Routing
- Conditional stage execution based on context
- Property, tag, and metadata-based conditions
- Dynamic workflow adaptation

### Advanced Scheduling
- Priority-based stage scheduling
- Resource-aware execution
- Circuit breaker pattern for fault tolerance
- Rate limiting for resource protection

### Comprehensive Monitoring
- Execution metrics and timing
- Success/failure tracking
- Performance analysis
- Stage-level and overall metrics

## Usage Example

```go
package main

import (
    "context"
    "time"

    "github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events/orchestration"
)

func main() {
    // Create orchestrator
    config := &orchestration.OrchestratorConfig{
        MaxConcurrentWorkflows: 10,
        DefaultTimeout:         5 * time.Minute,
        EnableMetrics:          true,
    }
    orchestrator := orchestration.NewOrchestrator(config)
    defer orchestrator.Close()

    // Define validation stages
    stage1 := &orchestration.ValidationStage{
        ID:   "input-validation",
        Name: "Input Validation",
        Validators: []orchestration.Validator{
            orchestration.NewSimpleValidator(
                "schema-validator", 
                "schema", 
                "Validates input schema", 
                true, 
                "Schema validation passed",
            ),
        },
        Timeout: 30 * time.Second,
    }

    stage2 := &orchestration.ValidationStage{
        ID:           "business-rules",
        Name:         "Business Rules Validation",
        Dependencies: []string{"input-validation"},
        Validators: []orchestration.Validator{
            orchestration.NewSimpleValidator(
                "business-validator", 
                "business", 
                "Validates business rules", 
                true, 
                "Business rules validation passed",
            ),
        },
        Conditions: []orchestration.StageCondition{
            {
                Type:     orchestration.TagCondition,
                Field:    "validate_business",
                Operator: orchestration.Equals,
                Value:    "true",
            },
        },
    }

    // Create workflow
    workflow := &orchestration.ValidationWorkflow{
        ID:      "user-input-validation",
        Name:    "User Input Validation Workflow",
        Version: "1.0.0",
        Stages:  []*orchestration.ValidationStage{stage1, stage2},
    }

    // Register workflow
    err := orchestrator.RegisterWorkflow(workflow)
    if err != nil {
        panic(err)
    }

    // Execute workflow
    validationCtx := &orchestration.ValidationContext{
        EventType:   "user-input",
        Source:      "web-form",
        Environment: "production",
        Tags: map[string]string{
            "validate_business": "true",
        },
        Properties: map[string]interface{}{
            "user_id": "12345",
            "data":    map[string]interface{}{"name": "John Doe"},
        },
    }

    ctx := context.Background()
    result, err := orchestrator.ExecuteWorkflow(ctx, "user-input-validation", validationCtx)
    if err != nil {
        panic(err)
    }

    // Check results
    if result.Status == orchestration.Completed {
        fmt.Printf("Workflow completed successfully in %v\n", result.Duration)
        fmt.Printf("Success rate: %.2f%%\n", result.Summary.SuccessRate*100)
    } else {
        fmt.Printf("Workflow failed: %v\n", result.Errors)
    }
}
```

## Stage Configuration

### Circuit Breaker
```go
stage.Config = &orchestration.StageConfig{
    CircuitBreaker: &orchestration.CircuitBreaker{
        Enabled:          true,
        FailureThreshold: 5,
        RecoveryTimeout:  30 * time.Second,
    },
}
```

### Rate Limiting
```go
stage.Config = &orchestration.StageConfig{
    RateLimiter: &orchestration.RateLimiter{
        Enabled:           true,
        RequestsPerSecond: 10.0,
        BurstSize:         20,
    },
}
```

### Resource Limits
```go
stage.Config = &orchestration.StageConfig{
    ResourceLimits: &orchestration.ResourceLimits{
        MaxMemory:     100 * 1024 * 1024, // 100MB
        MaxDuration:   2 * time.Minute,
        MaxGoroutines: 50,
    },
}
```

## Metrics and Monitoring

The orchestration engine provides comprehensive metrics:

- **Workflow Metrics**: Success rate, execution time, error counts
- **Stage Metrics**: Individual stage performance, retry counts, timeouts
- **Resource Metrics**: Memory usage, goroutine counts, rate limiting

Access metrics through:
```go
// Stage-specific metrics
metrics := orchestrator.pipelineExecutor.GetStageMetrics("stage-id")

// Overall metrics
overallMetrics := orchestrator.pipelineExecutor.GetOverallMetrics()
```

## Testing

The package includes comprehensive tests covering:
- Workflow registration and validation
- DAG construction and execution
- Parallel and sequential execution
- Error handling and retries
- Circuit breaker and rate limiting
- Metrics collection
- Concurrent execution

Run tests with:
```bash
go test ./pkg/core/events/orchestration/... -v
```

## Architecture

The orchestration engine follows a modular architecture:

1. **Orchestrator**: High-level coordinator
2. **Workflow Engine**: DAG execution and planning
3. **Pipeline Executor**: Stage execution and lifecycle
4. **Stage Manager**: Resource management and scheduling
5. **Validation Framework**: Pluggable validation interface

This design enables:
- High performance through parallel execution
- Fault tolerance through circuit breakers and retries
- Scalability through resource management
- Flexibility through pluggable validators
- Observability through comprehensive metrics