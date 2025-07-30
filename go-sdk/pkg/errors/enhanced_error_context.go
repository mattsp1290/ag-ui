package errors

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
)

// CorrelationID represents a unique identifier for tracking requests across services
type CorrelationID string

// OperationMetadata contains metadata about an operation
type OperationMetadata struct {
	// OperationType describes the type of operation (cache, validation, consensus, etc.)
	OperationType string
	
	// NodeID identifies the node performing the operation
	NodeID string
	
	// Component identifies the component performing the operation
	Component string
	
	// RetryCount tracks how many times this operation has been retried
	RetryCount int
	
	// ActionableGuidance provides suggestions for resolving errors
	ActionableGuidance []string
	
	// RelatedResources lists resources involved in the operation
	RelatedResources []string
	
	// PerformanceMetrics contains operation performance data
	PerformanceMetrics *PerformanceMetrics
}

// PerformanceMetrics contains performance-related data
type PerformanceMetrics struct {
	StartTime    time.Time
	Duration     time.Duration
	MemoryUsage  uint64
	CPUUsage     float64
	NetworkCalls int
	CacheHits    int
	CacheMisses  int
}

// EnhancedErrorContext extends ErrorContext with distributed system context
type EnhancedErrorContext struct {
	*ErrorContext
	
	// CorrelationID for tracking requests across services
	CorrelationID CorrelationID
	
	// Operation metadata
	OperationMetadata *OperationMetadata
	
	// Circuit breaker state information
	CircuitBreakerStates map[string]CircuitBreakerState
	
	// Distributed system context
	ClusterState *ClusterState
}

// ClusterState contains information about the distributed system state
type ClusterState struct {
	// ActiveNodes lists currently active nodes
	ActiveNodes []string
	
	// FailedNodes lists nodes that have failed
	FailedNodes []string
	
	// PartitionedNodes lists nodes that are network partitioned
	PartitionedNodes []string
	
	// ClusterHealth indicates overall cluster health (0.0 to 1.0)
	ClusterHealth float64
	
	// ReplicationFactor indicates the current replication factor
	ReplicationFactor int
}

// correlationIDCounter for generating unique correlation IDs
var correlationIDCounter uint64

// NewEnhancedErrorContext creates a new enhanced error context
func NewEnhancedErrorContext(operationName string) *EnhancedErrorContext {
	return &EnhancedErrorContext{
		ErrorContext:         NewErrorContext(operationName),
		CorrelationID:        generateCorrelationID(),
		OperationMetadata:    &OperationMetadata{},
		CircuitBreakerStates: make(map[string]CircuitBreakerState),
	}
}

// WithCorrelationID sets the correlation ID
func (eec *EnhancedErrorContext) WithCorrelationID(correlationID CorrelationID) *EnhancedErrorContext {
	eec.CorrelationID = correlationID
	return eec
}

// WithOperationType sets the operation type
func (eec *EnhancedErrorContext) WithOperationType(operationType string) *EnhancedErrorContext {
	if eec.OperationMetadata == nil {
		eec.OperationMetadata = &OperationMetadata{}
	}
	eec.OperationMetadata.OperationType = operationType
	return eec
}

// WithNodeID sets the node ID
func (eec *EnhancedErrorContext) WithNodeID(nodeID string) *EnhancedErrorContext {
	if eec.OperationMetadata == nil {
		eec.OperationMetadata = &OperationMetadata{}
	}
	eec.OperationMetadata.NodeID = nodeID
	return eec
}

// WithComponent sets the component name
func (eec *EnhancedErrorContext) WithComponent(component string) *EnhancedErrorContext {
	if eec.OperationMetadata == nil {
		eec.OperationMetadata = &OperationMetadata{}
	}
	eec.OperationMetadata.Component = component
	return eec
}

// WithRetryCount sets the retry count
func (eec *EnhancedErrorContext) WithRetryCount(retryCount int) *EnhancedErrorContext {
	if eec.OperationMetadata == nil {
		eec.OperationMetadata = &OperationMetadata{}
	}
	eec.OperationMetadata.RetryCount = retryCount
	return eec
}

// AddActionableGuidance adds actionable guidance for error resolution
func (eec *EnhancedErrorContext) AddActionableGuidance(guidance string) *EnhancedErrorContext {
	if eec.OperationMetadata == nil {
		eec.OperationMetadata = &OperationMetadata{}
	}
	eec.OperationMetadata.ActionableGuidance = append(eec.OperationMetadata.ActionableGuidance, guidance)
	return eec
}

// AddRelatedResource adds a related resource
func (eec *EnhancedErrorContext) AddRelatedResource(resource string) *EnhancedErrorContext {
	if eec.OperationMetadata == nil {
		eec.OperationMetadata = &OperationMetadata{}
	}
	eec.OperationMetadata.RelatedResources = append(eec.OperationMetadata.RelatedResources, resource)
	return eec
}

// WithPerformanceMetrics sets performance metrics
func (eec *EnhancedErrorContext) WithPerformanceMetrics(metrics *PerformanceMetrics) *EnhancedErrorContext {
	if eec.OperationMetadata == nil {
		eec.OperationMetadata = &OperationMetadata{}
	}
	eec.OperationMetadata.PerformanceMetrics = metrics
	return eec
}

// SetCircuitBreakerState sets the state of a circuit breaker
func (eec *EnhancedErrorContext) SetCircuitBreakerState(name string, state CircuitBreakerState) *EnhancedErrorContext {
	eec.CircuitBreakerStates[name] = state
	return eec
}

// WithClusterState sets the cluster state
func (eec *EnhancedErrorContext) WithClusterState(clusterState *ClusterState) *EnhancedErrorContext {
	eec.ClusterState = clusterState
	return eec
}

// RecordEnhancedError creates an enhanced error with all available context
func (eec *EnhancedErrorContext) RecordEnhancedError(err error, errorCode string) error {
	if err == nil {
		return nil
	}
	
	// Create enhanced error with all context
	enhanced := &BaseError{
		Code:      errorCode,
		Message:   err.Error(),
		Severity:  SeverityError,
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
		Cause:     err,
	}
	
	// Add correlation and operation context
	enhanced.Details["correlation_id"] = string(eec.CorrelationID)
	enhanced.Details["context_id"] = eec.ErrorContext.ID
	enhanced.Details["operation"] = eec.ErrorContext.OperationName
	
	// Add operation metadata
	if eec.OperationMetadata != nil {
		if eec.OperationMetadata.OperationType != "" {
			enhanced.Details["operation_type"] = eec.OperationMetadata.OperationType
		}
		if eec.OperationMetadata.NodeID != "" {
			enhanced.Details["node_id"] = eec.OperationMetadata.NodeID
		}
		if eec.OperationMetadata.Component != "" {
			enhanced.Details["component"] = eec.OperationMetadata.Component
		}
		if eec.OperationMetadata.RetryCount > 0 {
			enhanced.Details["retry_count"] = eec.OperationMetadata.RetryCount
		}
		if len(eec.OperationMetadata.ActionableGuidance) > 0 {
			enhanced.Details["actionable_guidance"] = eec.OperationMetadata.ActionableGuidance
		}
		if len(eec.OperationMetadata.RelatedResources) > 0 {
			enhanced.Details["related_resources"] = eec.OperationMetadata.RelatedResources
		}
		if eec.OperationMetadata.PerformanceMetrics != nil {
			enhanced.Details["performance_metrics"] = eec.OperationMetadata.PerformanceMetrics
		}
	}
	
	// Add circuit breaker states
	if len(eec.CircuitBreakerStates) > 0 {
		enhanced.Details["circuit_breaker_states"] = eec.CircuitBreakerStates
	}
	
	// Add cluster state
	if eec.ClusterState != nil {
		enhanced.Details["cluster_state"] = eec.ClusterState
	}
	
	// Add source location
	if pc, file, line, ok := runtime.Caller(1); ok {
		fn := runtime.FuncForPC(pc)
		var fnName string
		if fn != nil {
			fnName = fn.Name()
		}
		enhanced.Details["source"] = map[string]interface{}{
			"file":     file,
			"line":     line,
			"function": fnName,
		}
	}
	
	// Record in error context
	eec.ErrorContext.RecordError(enhanced)
	
	return enhanced
}

// GenerateErrorReport generates a comprehensive error report
func (eec *EnhancedErrorContext) GenerateErrorReport() *ErrorReport {
	errors := eec.ErrorContext.GetErrors()
	
	report := &ErrorReport{
		CorrelationID:    string(eec.CorrelationID),
		ContextID:        eec.ErrorContext.ID,
		Operation:        eec.ErrorContext.OperationName,
		Duration:         eec.ErrorContext.Duration(),
		ErrorCount:       len(errors),
		Errors:           make([]ErrorSummary, 0, len(errors)),
		Timestamp:        time.Now(),
		OperationMetadata: eec.OperationMetadata,
		ClusterState:     eec.ClusterState,
	}
	
	// Summarize errors
	for _, err := range errors {
		summary := ErrorSummary{
			Message:   err.Error(),
			Timestamp: time.Now(),
		}
		
		if baseErr, ok := err.(*BaseError); ok {
			summary.Code = baseErr.Code
			summary.Severity = baseErr.Severity
			summary.Details = baseErr.Details
		}
		
		report.Errors = append(report.Errors, summary)
	}
	
	// Generate recommendations
	report.Recommendations = eec.generateRecommendations()
	
	return report
}

// ErrorReport contains a comprehensive error report
type ErrorReport struct {
	CorrelationID     string             `json:"correlation_id"`
	ContextID         string             `json:"context_id"`
	Operation         string             `json:"operation"`
	Duration          time.Duration      `json:"duration"`
	ErrorCount        int                `json:"error_count"`
	Errors            []ErrorSummary     `json:"errors"`
	Timestamp         time.Time          `json:"timestamp"`
	OperationMetadata *OperationMetadata `json:"operation_metadata,omitempty"`
	ClusterState      *ClusterState      `json:"cluster_state,omitempty"`
	Recommendations   []string           `json:"recommendations"`
}

// ErrorSummary contains a summary of an error
type ErrorSummary struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Severity  Severity               `json:"severity"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// generateRecommendations generates recommendations based on the error context
func (eec *EnhancedErrorContext) generateRecommendations() []string {
	var recommendations []string
	
	// Add actionable guidance from operation metadata
	if eec.OperationMetadata != nil && len(eec.OperationMetadata.ActionableGuidance) > 0 {
		recommendations = append(recommendations, eec.OperationMetadata.ActionableGuidance...)
	}
	
	// Add circuit breaker recommendations
	for name, state := range eec.CircuitBreakerStates {
		switch state {
		case StateOpen:
			recommendations = append(recommendations, 
				fmt.Sprintf("Circuit breaker '%s' is open - check underlying service health", name))
		case StateHalfOpen:
			recommendations = append(recommendations, 
				fmt.Sprintf("Circuit breaker '%s' is half-open - reducing load may help recovery", name))
		}
	}
	
	// Add cluster state recommendations
	if eec.ClusterState != nil {
		if eec.ClusterState.ClusterHealth < 0.5 {
			recommendations = append(recommendations, "Cluster health is low - investigate failed nodes")
		}
		if len(eec.ClusterState.PartitionedNodes) > 0 {
			recommendations = append(recommendations, "Network partitions detected - check network connectivity")
		}
		if len(eec.ClusterState.FailedNodes) > 0 {
			recommendations = append(recommendations, "Failed nodes detected - restart or replace failed nodes")
		}
	}
	
	// Add retry recommendations
	if eec.OperationMetadata != nil && eec.OperationMetadata.RetryCount > 0 {
		recommendations = append(recommendations, 
			fmt.Sprintf("Operation has been retried %d times - consider exponential backoff", 
				eec.OperationMetadata.RetryCount))
	}
	
	return recommendations
}

// generateCorrelationID generates a unique correlation ID
func generateCorrelationID() CorrelationID {
	counter := atomic.AddUint64(&correlationIDCounter, 1)
	return CorrelationID(fmt.Sprintf("corr_%d_%d", time.Now().UnixNano(), counter))
}

// Enhanced error context key for context.Context
type enhancedErrorContextKey struct{}

// ToContext adds this enhanced error context to a context.Context
func (eec *EnhancedErrorContext) ToContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, enhancedErrorContextKey{}, eec)
}

// FromEnhancedContext retrieves an enhanced error context from a context.Context
func FromEnhancedContext(ctx context.Context) (*EnhancedErrorContext, bool) {
	eec, ok := ctx.Value(enhancedErrorContextKey{}).(*EnhancedErrorContext)
	return eec, ok
}

// GetOrCreateEnhancedContext gets an existing enhanced error context or creates a new one
func GetOrCreateEnhancedContext(ctx context.Context, operationName string) (*EnhancedErrorContext, context.Context) {
	if eec, ok := FromEnhancedContext(ctx); ok {
		return eec, ctx
	}
	
	eec := NewEnhancedErrorContext(operationName)
	newCtx := eec.ToContext(ctx)
	return eec, newCtx
}

// RecordEnhancedErrorInContext records an enhanced error in the context
func RecordEnhancedErrorInContext(ctx context.Context, err error, errorCode string) error {
	if eec, ok := FromEnhancedContext(ctx); ok {
		return eec.RecordEnhancedError(err, errorCode)
	}
	
	// Fallback to regular error recording
	RecordErrorInContext(ctx, err)
	return err
}

// EnhancedContextBuilder provides a fluent interface for building enhanced error contexts
type EnhancedContextBuilder struct {
	eec *EnhancedErrorContext
}

// NewEnhancedContextBuilder creates a new enhanced context builder
func NewEnhancedContextBuilder(operationName string) *EnhancedContextBuilder {
	return &EnhancedContextBuilder{
		eec: NewEnhancedErrorContext(operationName),
	}
}

// WithCorrelationID sets the correlation ID
func (b *EnhancedContextBuilder) WithCorrelationID(correlationID CorrelationID) *EnhancedContextBuilder {
	b.eec.WithCorrelationID(correlationID)
	return b
}

// WithOperationType sets the operation type
func (b *EnhancedContextBuilder) WithOperationType(operationType string) *EnhancedContextBuilder {
	b.eec.WithOperationType(operationType)
	return b
}

// WithNodeID sets the node ID
func (b *EnhancedContextBuilder) WithNodeID(nodeID string) *EnhancedContextBuilder {
	b.eec.WithNodeID(nodeID)
	return b
}

// WithComponent sets the component name
func (b *EnhancedContextBuilder) WithComponent(component string) *EnhancedContextBuilder {
	b.eec.WithComponent(component)
	return b
}

// AddActionableGuidance adds actionable guidance
func (b *EnhancedContextBuilder) AddActionableGuidance(guidance string) *EnhancedContextBuilder {
	b.eec.AddActionableGuidance(guidance)
	return b
}

// Build returns the built enhanced error context
func (b *EnhancedContextBuilder) Build() *EnhancedErrorContext {
	return b.eec
}

// BuildContext returns a context.Context with the enhanced error context
func (b *EnhancedContextBuilder) BuildContext(ctx context.Context) context.Context {
	return b.eec.ToContext(ctx)
}