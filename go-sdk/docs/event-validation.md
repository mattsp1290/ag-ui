# Event Validation System Guide

Comprehensive guide to the AG-UI Go SDK event validation system, covering advanced validation features, custom rules, and performance optimization.

## Table of Contents

- [Overview](#overview)
- [Core Concepts](#core-concepts)
- [Basic Validation](#basic-validation)
- [Advanced Validation Features](#advanced-validation-features)
- [Custom Validation Rules](#custom-validation-rules)
- [Performance Optimization](#performance-optimization)
- [Monitoring and Metrics](#monitoring-and-metrics)
- [Best Practices](#best-practices)
- [Examples](#examples)

## Overview

The AG-UI event validation system provides comprehensive validation capabilities for ensuring data integrity, protocol compliance, and business rule enforcement across all event processing workflows.

### Key Features

- **Multi-level Validation**: Structural, semantic, and business logic validation
- **Parallel Processing**: High-performance concurrent validation
- **Custom Rules Engine**: Extensible validation rule system
- **Protocol Compliance**: AG-UI protocol sequence validation
- **Performance Monitoring**: Built-in metrics and observability
- **Error Context**: Detailed error reporting with context preservation
- **Caching**: Intelligent validation result caching

## Core Concepts

### Event Types and Structure

The AG-UI protocol defines specific event types with structured validation:

```go
// Core event types
const (
    EventTypeTextMessageStart   EventType = "TEXT_MESSAGE_START"
    EventTypeTextMessageContent EventType = "TEXT_MESSAGE_CONTENT"
    EventTypeTextMessageEnd     EventType = "TEXT_MESSAGE_END"
    EventTypeToolCallStart      EventType = "TOOL_CALL_START"
    EventTypeToolCallArgs       EventType = "TOOL_CALL_ARGS"
    EventTypeToolCallEnd        EventType = "TOOL_CALL_END"
    EventTypeStateSnapshot      EventType = "STATE_SNAPSHOT"
    EventTypeStateDelta         EventType = "STATE_DELTA"
    EventTypeRunStarted         EventType = "RUN_STARTED"
    EventTypeRunFinished        EventType = "RUN_FINISHED"
    EventTypeRunError           EventType = "RUN_ERROR"
)
```

### Validation Levels

The validation system operates at multiple levels:

1. **Structural Validation**: JSON schema and type validation
2. **Protocol Validation**: AG-UI protocol compliance
3. **Sequence Validation**: Event ordering and lifecycle validation
4. **Business Logic Validation**: Custom domain-specific rules
5. **Security Validation**: Input sanitization and security checks

## Basic Validation

### Simple Event Validation

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

func main() {
    // Create a basic event
    event := events.NewBaseEvent(events.EventTypeTextMessageStart)
    
    // Validate the event
    if err := event.Validate(); err != nil {
        fmt.Printf("Validation failed: %v\n", err)
        return
    }
    
    fmt.Println("Event is valid!")
}
```

### Validating Event Sequences

```go
func validateEventSequence() error {
    // Create a sequence of events
    events := []events.Event{
        createRunStartEvent("run-123"),
        createMessageStartEvent("msg-456", "run-123"),
        createMessageContentEvent("msg-456", "Hello, world!"),
        createMessageEndEvent("msg-456"),
        createRunFinishEvent("run-123"),
    }
    
    // Validate the entire sequence
    if err := events.ValidateSequence(events); err != nil {
        return fmt.Errorf("sequence validation failed: %w", err)
    }
    
    return nil
}

func createRunStartEvent(runID string) events.Event {
    event := &events.RunStartedEvent{
        BaseEvent: *events.NewBaseEvent(events.EventTypeRunStarted),
        RunID:     runID,
        AgentName: "my-agent",
        StartTime: time.Now(),
    }
    return event
}

func createMessageStartEvent(messageID, runID string) events.Event {
    event := &events.TextMessageStartEvent{
        BaseEvent: *events.NewBaseEvent(events.EventTypeTextMessageStart),
        MessageID: messageID,
        RunID:     runID,
        Role:      "assistant",
    }
    return event
}
```

## Advanced Validation Features

### Parallel Validation

For high-throughput scenarios, use parallel validation to process multiple events concurrently:

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

func main() {
    // Configure parallel validator
    config := &events.ParallelValidatorConfig{
        MaxWorkers:       10,
        BufferSize:       100,
        TimeoutPerEvent:  5 * time.Second,
        EnableMetrics:    true,
        ErrorThreshold:   0.1, // 10% error rate threshold
    }
    
    validator := events.NewParallelValidator(config)
    defer validator.Shutdown()
    
    // Create batch of events to validate
    eventBatch := createEventBatch(1000)
    
    // Validate in parallel
    ctx := context.Background()
    results := validator.ValidateParallel(ctx, eventBatch)
    
    // Process results
    for result := range results {
        if result.Error != nil {
            fmt.Printf("Event %s validation failed: %v\n", 
                result.EventID, result.Error)
        } else {
            fmt.Printf("Event %s validated in %v\n", 
                result.EventID, result.Duration)
        }
    }
    
    // Get validation metrics
    metrics := validator.GetMetrics()
    fmt.Printf("Validation metrics: %+v\n", metrics)
}

func createEventBatch(count int) []*events.ValidationRequest {
    batch := make([]*events.ValidationRequest, count)
    
    for i := 0; i < count; i++ {
        event := events.NewBaseEvent(events.EventTypeTextMessageContent)
        batch[i] = &events.ValidationRequest{
            EventID: fmt.Sprintf("event-%d", i),
            Event:   event,
            Context: map[string]interface{}{
                "batch_id":     "batch-123",
                "source_ip":    "192.168.1.100",
                "user_agent":   "AG-UI-Client/1.0",
            },
        }
    }
    
    return batch
}
```

### Validation Result Processing

```go
type ValidationResult struct {
    EventID   string                 `json:"event_id"`
    Success   bool                   `json:"success"`
    Error     error                  `json:"error,omitempty"`
    Warnings  []string               `json:"warnings,omitempty"`
    Duration  time.Duration          `json:"duration"`
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
    Timestamp time.Time              `json:"timestamp"`
}

func processValidationResults(results <-chan *ValidationResult) {
    var (
        totalEvents   int
        successCount  int
        errorCount    int
        warningCount  int
        totalDuration time.Duration
    )
    
    for result := range results {
        totalEvents++
        totalDuration += result.Duration
        
        if result.Success {
            successCount++
        } else {
            errorCount++
            logValidationError(result)
        }
        
        if len(result.Warnings) > 0 {
            warningCount++
            logValidationWarnings(result)
        }
    }
    
    // Calculate statistics
    successRate := float64(successCount) / float64(totalEvents) * 100
    avgDuration := totalDuration / time.Duration(totalEvents)
    
    fmt.Printf("Validation Summary:\n")
    fmt.Printf("  Total Events: %d\n", totalEvents)
    fmt.Printf("  Success Rate: %.2f%%\n", successRate)
    fmt.Printf("  Error Count: %d\n", errorCount)
    fmt.Printf("  Warning Count: %d\n", warningCount)
    fmt.Printf("  Average Duration: %v\n", avgDuration)
}
```

### Validation Caching

Improve performance by caching validation results:

```go
// Configure validation cache
cacheConfig := &events.ValidationCacheConfig{
    MaxSize:        10000,
    TTL:            5 * time.Minute,
    CleanupInterval: 1 * time.Minute,
    HashFunction:   events.SHA256Hash,
    Metrics:        true,
}

cache := events.NewValidationCache(cacheConfig)

// Use cache with validator
validator := events.NewValidator(&events.ValidatorConfig{
    EnableCache: true,
    Cache:       cache,
})

// Validation will automatically use cache
result := validator.Validate(ctx, event)
```

## Custom Validation Rules

### Creating Custom Rules

```go
// Define a custom validation rule
type BusinessLogicRule struct {
    name        string
    description string
    priority    int
}

func (r *BusinessLogicRule) Name() string {
    return r.name
}

func (r *BusinessLogicRule) Description() string {
    return r.description
}

func (r *BusinessLogicRule) Priority() int {
    return r.priority
}

func (r *BusinessLogicRule) Validate(ctx context.Context, event events.Event) error {
    // Extract user context
    userID, ok := events.GetUserIDFromContext(ctx)
    if !ok {
        return fmt.Errorf("user context required")
    }
    
    // Validate based on event type
    switch e := event.(type) {
    case *events.TextMessageContentEvent:
        return r.validateMessageContent(userID, e)
    case *events.ToolCallStartEvent:
        return r.validateToolCall(userID, e)
    default:
        // No specific validation for this event type
        return nil
    }
}

func (r *BusinessLogicRule) validateMessageContent(userID string, event *events.TextMessageContentEvent) error {
    // Check content length
    if len(event.Content) > 10000 {
        return &events.ValidationError{
            Code:    "CONTENT_TOO_LONG",
            Message: "Message content exceeds maximum length",
            Field:   "content",
            Value:   fmt.Sprintf("%d characters", len(event.Content)),
        }
    }
    
    // Check for prohibited content
    if containsProhibitedContent(event.Content) {
        return &events.ValidationError{
            Code:    "PROHIBITED_CONTENT",
            Message: "Message contains prohibited content",
            Field:   "content",
        }
    }
    
    // Check user permissions
    if !hasMessagePermission(userID) {
        return &events.ValidationError{
            Code:    "INSUFFICIENT_PERMISSIONS",
            Message: "User does not have permission to send messages",
            Field:   "user_id",
            Value:   userID,
        }
    }
    
    return nil
}

func (r *BusinessLogicRule) validateToolCall(userID string, event *events.ToolCallStartEvent) error {
    // Check if user can call this tool
    if !canCallTool(userID, event.ToolName) {
        return &events.ValidationError{
            Code:    "TOOL_ACCESS_DENIED",
            Message: fmt.Sprintf("User cannot call tool '%s'", event.ToolName),
            Field:   "tool_name",
            Value:   event.ToolName,
        }
    }
    
    // Validate tool arguments
    if err := validateToolArguments(event.ToolName, event.Arguments); err != nil {
        return fmt.Errorf("tool argument validation failed: %w", err)
    }
    
    return nil
}

// Helper functions
func containsProhibitedContent(content string) bool {
    prohibitedWords := []string{"spam", "malware", "phishing"}
    contentLower := strings.ToLower(content)
    
    for _, word := range prohibitedWords {
        if strings.Contains(contentLower, word) {
            return true
        }
    }
    return false
}

func hasMessagePermission(userID string) bool {
    // Check user permissions (implement based on your auth system)
    return getUserPermissions(userID).HasPermission("send_messages")
}

func canCallTool(userID, toolName string) bool {
    // Check tool access permissions
    return getUserPermissions(userID).HasPermission("call_tool:" + toolName)
}
```

### Registering Custom Rules

```go
func setupCustomValidation() (*events.Validator, error) {
    // Create validator
    validator := events.NewValidator(&events.ValidatorConfig{
        EnableBuiltinRules: true,
        EnableCustomRules:  true,
        StrictMode:        false,
    })
    
    // Register custom rules
    rules := []events.ValidationRule{
        &BusinessLogicRule{
            name:        "business-logic",
            description: "Validates business logic constraints",
            priority:    100,
        },
        &SecurityRule{
            name:        "security-checks",
            description: "Performs security validation",
            priority:    200, // Higher priority = executed first
        },
        &ComplianceRule{
            name:        "compliance",
            description: "Ensures regulatory compliance",
            priority:    150,
        },
    }
    
    for _, rule := range rules {
        if err := validator.RegisterRule(rule); err != nil {
            return nil, fmt.Errorf("failed to register rule %s: %w", 
                rule.Name(), err)
        }
    }
    
    return validator, nil
}
```

### Conditional Rules

```go
// Conditional rule that applies only in specific contexts
type ConditionalRule struct {
    name      string
    condition func(context.Context, events.Event) bool
    validator func(context.Context, events.Event) error
}

func (r *ConditionalRule) Name() string {
    return r.name
}

func (r *ConditionalRule) Validate(ctx context.Context, event events.Event) error {
    // Check if rule applies
    if !r.condition(ctx, event) {
        return nil // Rule doesn't apply, skip validation
    }
    
    // Apply validation
    return r.validator(ctx, event)
}

// Example: Production-only validation
func createProductionOnlyRule() events.ValidationRule {
    return &ConditionalRule{
        name: "production-only",
        condition: func(ctx context.Context, event events.Event) bool {
            env := os.Getenv("ENVIRONMENT")
            return env == "production"
        },
        validator: func(ctx context.Context, event events.Event) error {
            // Strict production validation
            return validateForProduction(event)
        },
    }
}

// Example: Time-based validation
func createTimeBasedRule() events.ValidationRule {
    return &ConditionalRule{
        name: "business-hours-only",
        condition: func(ctx context.Context, event events.Event) bool {
            now := time.Now()
            hour := now.Hour()
            weekday := now.Weekday()
            
            // Apply only during business hours (9-17) on weekdays
            return weekday >= time.Monday && weekday <= time.Friday && 
                   hour >= 9 && hour < 17
        },
        validator: func(ctx context.Context, event events.Event) error {
            // Business hours specific validation
            return validateBusinessHours(event)
        },
    }
}
```

## Performance Optimization

### Batch Validation

```go
// Optimize validation by processing events in batches
func validateEventBatch(validator *events.Validator, eventBatch []events.Event) error {
    batchSize := 100
    errorChan := make(chan error, len(eventBatch))
    
    // Process in parallel batches
    for i := 0; i < len(eventBatch); i += batchSize {
        end := i + batchSize
        if end > len(eventBatch) {
            end = len(eventBatch)
        }
        
        batch := eventBatch[i:end]
        
        go func(batch []events.Event) {
            for _, event := range batch {
                if err := validator.Validate(context.Background(), event); err != nil {
                    errorChan <- err
                    return
                }
            }
            errorChan <- nil
        }(batch)
    }
    
    // Collect results
    var errors []error
    for i := 0; i < (len(eventBatch)+batchSize-1)/batchSize; i++ {
        if err := <-errorChan; err != nil {
            errors = append(errors, err)
        }
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("batch validation failed: %v", errors)
    }
    
    return nil
}
```

### Validation Profiles

```go
// Define validation profiles for different scenarios
type ValidationProfile struct {
    Name        string
    Rules       []string
    StrictMode  bool
    CacheEnabled bool
    Timeout     time.Duration
}

var ValidationProfiles = map[string]ValidationProfile{
    "development": {
        Name:        "development",
        Rules:       []string{"basic", "protocol"},
        StrictMode:  false,
        CacheEnabled: false,
        Timeout:     30 * time.Second,
    },
    "testing": {
        Name:        "testing",
        Rules:       []string{"basic", "protocol", "business-logic"},
        StrictMode:  true,
        CacheEnabled: true,
        Timeout:     10 * time.Second,
    },
    "production": {
        Name:        "production",
        Rules:       []string{"basic", "protocol", "business-logic", "security"},
        StrictMode:  true,
        CacheEnabled: true,
        Timeout:     5 * time.Second,
    },
}

func createValidatorForProfile(profileName string) (*events.Validator, error) {
    profile, exists := ValidationProfiles[profileName]
    if !exists {
        return nil, fmt.Errorf("unknown validation profile: %s", profileName)
    }
    
    config := &events.ValidatorConfig{
        StrictMode:    profile.StrictMode,
        EnableCache:   profile.CacheEnabled,
        Timeout:       profile.Timeout,
        EnabledRules:  profile.Rules,
    }
    
    return events.NewValidator(config), nil
}
```

### Memory Optimization

```go
// Use object pooling to reduce garbage collection pressure
var eventPool = sync.Pool{
    New: func() interface{} {
        return &events.ValidationRequest{}
    },
}

func validateWithPooling(validator *events.Validator, event events.Event) error {
    // Get request object from pool
    req := eventPool.Get().(*events.ValidationRequest)
    defer eventPool.Put(req)
    
    // Reset and populate request
    req.Reset()
    req.Event = event
    req.EventID = generateEventID()
    req.Timestamp = time.Now()
    
    // Validate
    return validator.ValidateRequest(context.Background(), req)
}
```

## Monitoring and Metrics

### Built-in Metrics

```go
// Access validation metrics
metrics := validator.GetMetrics()

fmt.Printf("Validation Metrics:\n")
fmt.Printf("  Total Validations: %d\n", metrics.TotalValidations)
fmt.Printf("  Success Rate: %.2f%%\n", metrics.SuccessRate)
fmt.Printf("  Average Latency: %v\n", metrics.AverageLatency)
fmt.Printf("  P95 Latency: %v\n", metrics.P95Latency)
fmt.Printf("  P99 Latency: %v\n", metrics.P99Latency)
fmt.Printf("  Error Rate: %.2f%%\n", metrics.ErrorRate)
fmt.Printf("  Cache Hit Rate: %.2f%%\n", metrics.CacheHitRate)
```

### Custom Metrics Collection

```go
// Implement custom metrics collector
type CustomMetricsCollector struct {
    validationCounter   *prometheus.CounterVec
    validationDuration *prometheus.HistogramVec
    ruleExecutionTime  *prometheus.HistogramVec
}

func NewCustomMetricsCollector() *CustomMetricsCollector {
    return &CustomMetricsCollector{
        validationCounter: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "event_validations_total",
                Help: "Total number of event validations",
            },
            []string{"event_type", "status", "rule"},
        ),
        validationDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "event_validation_duration_seconds",
                Help:    "Duration of event validation",
                Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
            },
            []string{"event_type", "rule"},
        ),
        ruleExecutionTime: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "validation_rule_execution_seconds",
                Help:    "Duration of individual validation rule execution",
                Buckets: prometheus.ExponentialBuckets(0.0001, 2, 8),
            },
            []string{"rule_name"},
        ),
    }
}

func (c *CustomMetricsCollector) RecordValidation(
    eventType string, 
    status string, 
    rule string, 
    duration time.Duration,
) {
    c.validationCounter.WithLabelValues(eventType, status, rule).Inc()
    c.validationDuration.WithLabelValues(eventType, rule).Observe(duration.Seconds())
}

func (c *CustomMetricsCollector) RecordRuleExecution(
    ruleName string, 
    duration time.Duration,
) {
    c.ruleExecutionTime.WithLabelValues(ruleName).Observe(duration.Seconds())
}
```

### Alert Configuration

```go
// Configure alerts for validation system
type ValidationAlerts struct {
    ErrorRateThreshold    float64       `json:"error_rate_threshold"`
    LatencyThreshold      time.Duration `json:"latency_threshold"`
    ThroughputThreshold   int           `json:"throughput_threshold"`
    CheckInterval         time.Duration `json:"check_interval"`
    AlertWebhook          string        `json:"alert_webhook"`
}

func setupValidationAlerts(metrics *events.ValidationMetrics) {
    alerts := &ValidationAlerts{
        ErrorRateThreshold:  5.0,  // 5% error rate
        LatencyThreshold:    100 * time.Millisecond,
        ThroughputThreshold: 1000, // events per second
        CheckInterval:       30 * time.Second,
        AlertWebhook:        "https://alerts.example.com/webhook",
    }
    
    ticker := time.NewTicker(alerts.CheckInterval)
    defer ticker.Stop()
    
    for range ticker.C {
        checkValidationHealth(metrics, alerts)
    }
}

func checkValidationHealth(metrics *events.ValidationMetrics, alerts *ValidationAlerts) {
    current := metrics.GetCurrent()
    
    // Check error rate
    if current.ErrorRate > alerts.ErrorRateThreshold {
        sendAlert("HIGH_ERROR_RATE", map[string]interface{}{
            "current_rate": current.ErrorRate,
            "threshold":    alerts.ErrorRateThreshold,
        }, alerts.AlertWebhook)
    }
    
    // Check latency
    if current.P95Latency > alerts.LatencyThreshold {
        sendAlert("HIGH_LATENCY", map[string]interface{}{
            "current_latency": current.P95Latency,
            "threshold":       alerts.LatencyThreshold,
        }, alerts.AlertWebhook)
    }
    
    // Check throughput
    if current.Throughput < float64(alerts.ThroughputThreshold) {
        sendAlert("LOW_THROUGHPUT", map[string]interface{}{
            "current_throughput": current.Throughput,
            "threshold":          alerts.ThroughputThreshold,
        }, alerts.AlertWebhook)
    }
}
```

## Best Practices

### 1. Layered Validation Strategy

```go
// Implement validation in layers
func validateEvent(ctx context.Context, event events.Event) error {
    // Layer 1: Basic structural validation
    if err := validateStructure(event); err != nil {
        return fmt.Errorf("structural validation failed: %w", err)
    }
    
    // Layer 2: Protocol compliance
    if err := validateProtocol(event); err != nil {
        return fmt.Errorf("protocol validation failed: %w", err)
    }
    
    // Layer 3: Business logic
    if err := validateBusinessLogic(ctx, event); err != nil {
        return fmt.Errorf("business validation failed: %w", err)
    }
    
    // Layer 4: Security checks
    if err := validateSecurity(ctx, event); err != nil {
        return fmt.Errorf("security validation failed: %w", err)
    }
    
    return nil
}
```

### 2. Error Context Preservation

```go
// Always preserve error context
func validateWithContext(ctx context.Context, event events.Event) error {
    // Add validation context
    validationCtx := events.WithValidationContext(ctx, &events.ValidationContext{
        EventID:     event.GetID(),
        EventType:   event.Type(),
        UserID:      events.GetUserIDFromContext(ctx),
        RequestID:   events.GetRequestIDFromContext(ctx),
        Timestamp:   time.Now(),
    })
    
    if err := validator.Validate(validationCtx, event); err != nil {
        // Wrap error with context
        return events.WrapValidationError(err, validationCtx)
    }
    
    return nil
}
```

### 3. Performance Monitoring

```go
// Monitor validation performance
func monitoredValidation(validator *events.Validator, event events.Event) error {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        metrics.RecordValidationDuration(event.Type().String(), duration)
    }()
    
    ctx := context.Background()
    return validator.Validate(ctx, event)
}
```

### 4. Graceful Degradation

```go
// Implement graceful degradation for validation failures
func validateWithFallback(validator *events.Validator, event events.Event) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    err := validator.Validate(ctx, event)
    
    if err != nil {
        // Check if it's a timeout or critical error
        if errors.Is(err, context.DeadlineExceeded) {
            // Log timeout and allow event to proceed with warning
            log.Warn("Validation timeout, proceeding with limited validation",
                "event_type", event.Type(),
                "event_id", event.GetID(),
            )
            
            // Perform basic validation only
            return validateBasicStructure(event)
        }
        
        // For other errors, fail the validation
        return err
    }
    
    return nil
}
```

## Examples

### Complete Validation Setup

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/core/events"
)

func main() {
    // Setup comprehensive validation system
    validator, err := setupValidationSystem()
    if err != nil {
        log.Fatal(err)
    }
    defer validator.Shutdown()
    
    // Example event stream processing
    if err := processEventStream(validator); err != nil {
        log.Fatal(err)
    }
}

func setupValidationSystem() (*events.Validator, error) {
    // Create validator with production configuration
    config := &events.ValidatorConfig{
        EnableBuiltinRules: true,
        EnableCustomRules:  true,
        StrictMode:        true,
        EnableCache:       true,
        EnableMetrics:     true,
        MaxWorkers:        10,
        Timeout:           5 * time.Second,
        CacheConfig: &events.ValidationCacheConfig{
            MaxSize: 10000,
            TTL:     5 * time.Minute,
        },
    }
    
    validator := events.NewValidator(config)
    
    // Register custom business rules
    customRules := []events.ValidationRule{
        &BusinessLogicRule{
            name:        "business-logic",
            description: "Validates business constraints",
            priority:    100,
        },
        &SecurityRule{
            name:        "security-checks",
            description: "Security validation",
            priority:    200,
        },
    }
    
    for _, rule := range customRules {
        if err := validator.RegisterRule(rule); err != nil {
            return nil, fmt.Errorf("failed to register rule: %w", err)
        }
    }
    
    // Setup monitoring
    go monitorValidationHealth(validator)
    
    return validator, nil
}

func processEventStream(validator *events.Validator) error {
    // Simulate event stream processing
    events := generateTestEvents(1000)
    
    for _, event := range events {
        if err := validateAndProcess(validator, event); err != nil {
            log.Printf("Event processing failed: %v", err)
            continue
        }
    }
    
    return nil
}

func validateAndProcess(validator *events.Validator, event events.Event) error {
    ctx := context.Background()
    
    // Add request context
    ctx = events.WithRequestID(ctx, generateRequestID())
    ctx = events.WithUserID(ctx, extractUserID(event))
    
    // Validate event
    start := time.Now()
    err := validator.Validate(ctx, event)
    duration := time.Since(start)
    
    // Record metrics
    status := "success"
    if err != nil {
        status = "failure"
    }
    
    recordValidationMetrics(event.Type().String(), status, duration)
    
    if err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    
    // Process validated event
    return processValidatedEvent(ctx, event)
}

func monitorValidationHealth(validator *events.Validator) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        metrics := validator.GetMetrics()
        
        log.Printf("Validation Health Check:")
        log.Printf("  Success Rate: %.2f%%", metrics.SuccessRate)
        log.Printf("  Average Latency: %v", metrics.AverageLatency)
        log.Printf("  P99 Latency: %v", metrics.P99Latency)
        log.Printf("  Cache Hit Rate: %.2f%%", metrics.CacheHitRate)
        
        // Alert on degraded performance
        if metrics.SuccessRate < 95.0 {
            sendHealthAlert("Low success rate", metrics)
        }
        
        if metrics.P99Latency > 100*time.Millisecond {
            sendHealthAlert("High latency", metrics)
        }
    }
}

// Helper functions (implement based on your needs)
func generateTestEvents(count int) []events.Event {
    // Generate test events
    return []events.Event{}
}

func extractUserID(event events.Event) string {
    // Extract user ID from event
    return "user-123"
}

func generateRequestID() string {
    // Generate unique request ID
    return fmt.Sprintf("req-%d", time.Now().UnixNano())
}

func recordValidationMetrics(eventType, status string, duration time.Duration) {
    // Record metrics to your monitoring system
}

func processValidatedEvent(ctx context.Context, event events.Event) error {
    // Process the validated event
    return nil
}

func sendHealthAlert(message string, metrics *events.ValidationMetrics) {
    // Send alert to monitoring system
}
```

This comprehensive event validation guide provides all the tools and patterns needed to implement robust, performant, and maintainable event validation in the AG-UI Go SDK.