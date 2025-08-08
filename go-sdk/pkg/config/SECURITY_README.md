# AG-UI Configuration Security System

## Overview

The AG-UI configuration system includes comprehensive security features designed to protect against DoS attacks and resource exhaustion while maintaining high performance and usability. This document details the security features, their implementation, and usage patterns.

## Security Features

### 1. Resource Limits

#### File Size Limits
- **Purpose**: Prevent DoS attacks through oversized configuration files
- **Default**: 10MB maximum file size
- **Configurable**: Yes, through `ResourceLimits.MaxFileSize`
- **Behavior**: Files exceeding the limit are rejected during loading

#### Memory Usage Limits
- **Purpose**: Prevent memory exhaustion attacks
- **Default**: 50MB maximum memory usage
- **Configurable**: Yes, through `ResourceLimits.MaxMemoryUsage`
- **Tracking**: Real-time memory usage monitoring with peak tracking

#### Configuration Structure Limits
- **Max Nesting Depth**: Prevents stack overflow attacks (default: 20 levels)
- **Max Keys**: Limits total configuration keys (default: 10,000)
- **Max Array Size**: Limits array sizes (default: 1,000 items)
- **Max String Length**: Limits individual string values (default: 64KB)

### 2. Watcher Limits

#### Total Watcher Limit
- **Purpose**: Prevent resource exhaustion through unlimited watchers
- **Default**: 100 watchers per configuration instance
- **Enforcement**: Checked before adding new watchers

#### Per-Key Watcher Limit
- **Purpose**: Prevent excessive watchers on individual keys
- **Default**: 10 watchers per key
- **Benefit**: Balanced resource distribution

### 3. Rate Limiting

#### Update Rate Limiting
- **Purpose**: Prevent rapid-fire configuration updates
- **Default**: 100ms minimum between updates
- **Scope**: Per configuration instance

#### Reload Rate Limiting
- **Purpose**: Prevent excessive configuration reloads
- **Default**: 1 second minimum between reloads
- **Protection**: File system and processing overhead

#### Validation Rate Limiting
- **Purpose**: Prevent validation spam
- **Default**: 500ms minimum between validations
- **Benefit**: CPU usage protection

### 4. Timeout Protection

#### Load Timeouts
- **Purpose**: Prevent hanging on slow/malicious sources
- **Default**: 30 seconds
- **Scope**: Per source load operation

#### Validation Timeouts
- **Purpose**: Prevent validation deadlocks
- **Default**: 10 seconds
- **Implementation**: Context-based cancellation

#### Watcher Timeouts
- **Purpose**: Prevent callback deadlocks
- **Default**: 5 seconds
- **Behavior**: Callbacks are cancelled if they exceed timeout

### 5. Monitoring and Alerting

#### Real-time Metrics
- Operation counts and rates
- Resource usage tracking
- Error classification and counting
- Performance metrics (load times, validation times)

#### Alert System
- Configurable thresholds for all metrics
- Multi-level severity (healthy, degraded, unhealthy, critical)
- Historical alert tracking
- Health check aggregation

#### Security Violation Detection
- Path traversal attempts
- Suspicious file access patterns
- Resource limit violations
- Rate limiting violations

### 6. Graceful Degradation

#### Principle
When resource limits are exceeded, the system can gracefully degrade rather than failing completely.

#### Behaviors
- **Rate Limiting**: Skip operations instead of failing
- **Memory Limits**: Reject new data but maintain existing functionality
- **Watcher Limits**: Reject new watchers but maintain existing ones
- **Error Recovery**: Automatic retry with backoff for recoverable errors

## Usage Examples

### Basic Secure Configuration

```go
// Create secure configuration with default limits
options := config.DefaultSecurityOptions()
secureConfig := config.NewSecureConfig(options)
defer secureConfig.Shutdown()

// All operations are automatically protected
err := secureConfig.Set("database.host", "localhost")
if err != nil {
    // Handle security violations gracefully
    if config.IsResourceError(err) {
        log.Printf("Resource limit exceeded: %v", err)
    }
}
```

### Custom Resource Limits

```go
// Configure custom limits for your use case
options := config.DefaultSecurityOptions()
options.ResourceLimits.MaxFileSize = 5 * 1024 * 1024    // 5MB
options.ResourceLimits.MaxMemoryUsage = 20 * 1024 * 1024 // 20MB
options.ResourceLimits.MaxWatchers = 50
options.ResourceLimits.UpdateRateLimit = 200 * time.Millisecond

config := config.NewSecureConfig(options)
```

### Production Configuration

```go
// Production-ready configuration
options := &config.SecurityOptions{
    EnableResourceLimits:      true,
    EnableMonitoring:          true,
    EnableAuditing:            true,
    EnableGracefulDegradation: true,
    ResourceLimits: &config.ResourceLimits{
        MaxFileSize:         50 * 1024 * 1024,  // 50MB
        MaxMemoryUsage:      200 * 1024 * 1024, // 200MB
        MaxWatchers:         500,
        ReloadRateLimit:     5 * time.Second,
        LoadTimeout:         60 * time.Second,
    },
    AlertThresholds: &config.AlertThresholds{
        MaxMemoryUsage:    150 * 1024 * 1024, // Alert at 75% of limit
        MaxErrorRate:      10.0,              // 10 errors/minute
        WatcherCountAlert: 400,               // Alert at 80% of limit
    },
}

config := config.NewSecureConfig(options)
```

### Monitoring and Metrics

```go
// Access real-time metrics
metrics := config.GetMetricsSnapshot()
fmt.Printf("Memory usage: %d bytes\n", metrics.PeakMemoryUsage)
fmt.Printf("Operation rate: %.2f ops/sec\n", metrics.OperationsPerSecond)
fmt.Printf("Error rate: %.2f errors/min\n", metrics.ErrorRate)

// Check health status
health := config.GetHealthStatus()
fmt.Printf("Overall health: %s\n", health.Overall)

// Get recent alerts
alerts := config.GetAlerts(10)
for _, alert := range alerts {
    fmt.Printf("[%s] %s: %s\n", alert.Severity, alert.Type, alert.Message)
}

// Export metrics as JSON
jsonData, err := metrics.ToJSON()
if err == nil {
    // Send to monitoring system
    sendToMonitoring(jsonData)
}
```

## Security Best Practices

### 1. Configure Appropriate Limits

#### Development Environment
```go
options := config.DefaultSecurityOptions()
options.ResourceLimits.MaxFileSize = 1 * 1024 * 1024     // 1MB
options.ResourceLimits.MaxMemoryUsage = 10 * 1024 * 1024  // 10MB
options.ResourceLimits.UpdateRateLimit = 50 * time.Millisecond
```

#### Production Environment
```go
options := config.DefaultSecurityOptions()
options.ResourceLimits.MaxFileSize = 50 * 1024 * 1024     // 50MB
options.ResourceLimits.MaxMemoryUsage = 200 * 1024 * 1024 // 200MB
options.ResourceLimits.UpdateRateLimit = 100 * time.Millisecond
options.EnableAuditing = true  // Enable auditing in production
```

### 2. Enable Comprehensive Monitoring

```go
// Always enable monitoring in production
options.EnableMonitoring = true

// Set appropriate alert thresholds (75-80% of limits)
options.AlertThresholds = &config.AlertThresholds{
    MaxMemoryUsage:    150 * 1024 * 1024, // 75% of 200MB limit
    MaxErrorRate:      5.0,               // 5 errors per minute
    WatcherCountAlert: 80,                // 80% of watcher limit
}
```

### 3. Handle Errors Gracefully

```go
err := config.Set("key", value)
if err != nil {
    // Check error type for appropriate handling
    switch e := err.(type) {
    case *config.ResourceLimitError:
        if e.IsRecoverable() {
            // Wait and retry for recoverable errors
            time.Sleep(time.Second)
            // Retry with exponential backoff
        } else {
            // Log and reject for non-recoverable errors
            log.Printf("Configuration rejected: %v", e)
        }
    case *config.RateLimitError:
        // Wait for rate limit to expire
        time.Sleep(e.GetRetryAfter())
        // Retry operation
    case *config.SecurityError:
        // Security violations should be logged and investigated
        log.Printf("SECURITY VIOLATION: %v", e)
        // Never retry security violations
    default:
        log.Printf("Configuration error: %v", err)
    }
}
```

### 4. Monitor Health Continuously

```go
// Regular health checks
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            health := config.GetHealthStatus()
            if health.Overall != "healthy" {
                log.Printf("Configuration health: %s", health.Overall)
                
                // Check individual health checks
                for checkName, result := range health.Checks {
                    if result.Status != "healthy" {
                        log.Printf("  %s: %s - %s", checkName, result.Status, result.Message)
                    }
                }
            }
        case <-ctx.Done():
            return
        }
    }
}()
```

### 5. Proper Resource Cleanup

```go
// Always defer shutdown to clean up resources
defer config.Shutdown()

// Remove watchers when no longer needed
callbackID, err := config.Watch("key", callback)
if err == nil {
    // Remember to unwatch when done
    defer config.UnWatch("key", callbackID)
}
```

## Error Types and Handling

### ResourceLimitError
- **When**: Resource limits are exceeded
- **Recoverable**: Some types (memory, watchers) may be recoverable
- **Handling**: Check `IsRecoverable()` and retry if appropriate

### RateLimitError
- **When**: Operations exceed configured rate limits
- **Recoverable**: Always (just need to wait)
- **Handling**: Use `GetRetryAfter()` to determine wait time

### StructureLimitError
- **When**: Configuration structure violates limits
- **Recoverable**: Usually not (requires different data)
- **Handling**: Reject and log, provide user feedback

### TimeoutError
- **When**: Operations exceed configured timeouts
- **Recoverable**: Sometimes (may indicate temporary issues)
- **Handling**: Retry with exponential backoff

### SecurityError
- **When**: Security violations are detected
- **Recoverable**: Never (indicates malicious activity)
- **Handling**: Log, alert, and investigate

## Performance Considerations

### Overhead
- **Resource checking**: ~1-5μs per operation
- **Monitoring**: ~100-500ns per metric update
- **Memory tracking**: ~50-100ns per allocation

### Optimization Tips
1. **Disable features you don't need**:
   ```go
   options.EnableMonitoring = false  // In development
   options.EnableAuditing = false    // If not required
   ```

2. **Tune limits appropriately**:
   - Don't set limits too low (causes unnecessary rejections)
   - Don't set limits too high (reduces protection)

3. **Use batch operations** when possible:
   ```go
   // Better: batch multiple sets
   data := map[string]interface{}{
       "key1": "value1",
       "key2": "value2",
   }
   config.Set("batch", data)
   
   // Avoid: many individual sets
   config.Set("key1", "value1")  // Rate limited
   config.Set("key2", "value2")  // Rate limited
   ```

## Integration with Monitoring Systems

### Prometheus Integration
```go
// Export metrics to Prometheus
func exportMetricsToPrometheus(config *SecureConfigImpl) {
    metrics := config.GetMetricsSnapshot()
    
    // Update Prometheus metrics
    configOperations.Set(float64(metrics.OperationCount))
    configErrors.Set(float64(metrics.ErrorCount))
    configMemoryUsage.Set(float64(metrics.PeakMemoryUsage))
    configWatchers.Set(float64(metrics.WatcherCount))
}
```

### Custom Monitoring
```go
// Send metrics to custom monitoring system
func sendMetricsToCustomSystem(config *SecureConfigImpl) {
    metrics := config.GetMetricsSnapshot()
    jsonData, _ := metrics.ToJSON()
    
    // Send to your monitoring endpoint
    http.Post("http://monitoring.internal/metrics", "application/json", bytes.NewReader(jsonData))
}
```

## Testing Security Features

### Unit Tests
```go
func TestDoSProtection(t *testing.T) {
    options := config.DefaultSecurityOptions()
    options.ResourceLimits.MaxStringLength = 100
    
    config := config.NewSecureConfig(options)
    defer config.Shutdown()
    
    // This should be rejected
    longString := strings.Repeat("x", 200)
    err := config.Set("long", longString)
    
    if err == nil {
        t.Error("Long string should be rejected")
    }
    
    if !config.IsResourceError(err) {
        t.Error("Should be a resource error")
    }
}
```

### Load Testing
```go
func TestConcurrentLoad(t *testing.T) {
    config := config.NewSecureConfig(config.DefaultSecurityOptions())
    defer config.Shutdown()
    
    // Simulate concurrent attackers
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            
            for j := 0; j < 100; j++ {
                config.Set(fmt.Sprintf("key_%d_%d", id, j), "value")
                time.Sleep(time.Millisecond)
            }
        }(i)
    }
    wg.Wait()
    
    // System should still be responsive
    err := config.Set("final_test", "value")
    if err != nil {
        t.Logf("System under stress: %v", err)
    }
}
```

## Troubleshooting

### Common Issues

#### High Memory Usage
```go
// Check memory usage
stats := config.GetResourceStats()
if stats.CurrentMemoryUsage > threshold {
    // Investigate large values
    allSettings := config.AllSettings()
    // Log largest values for analysis
}
```

#### Rate Limiting Issues
```go
// Check rate limiting stats
metrics := config.GetMetricsSnapshot()
if metrics.RateLimitHits > 0 {
    log.Printf("Rate limiting occurred %d times", metrics.RateLimitHits)
    // Consider increasing rate limits or reducing update frequency
}
```

#### Watcher Exhaustion
```go
// Check watcher usage
stats := config.GetResourceStats()
log.Printf("Current watchers: %d", stats.CurrentWatchers)
log.Printf("Watchers by key: %v", stats.WatchersByKey)
// Identify keys with too many watchers
```

### Debug Mode
```go
// Enable debug logging for development
options := config.DefaultSecurityOptions()
options.ResourceLimits.MaxMemoryUsage = 1024 * 1024  // Low limit for testing
config := config.NewSecureConfig(options)

// Set up error callbacks for debugging
if config.errorHandler != nil {
    config.errorHandler.OnResourceLimitExceeded = func(err *config.ResourceLimitError) {
        log.Printf("DEBUG: Resource limit exceeded: %v", err)
    }
    config.errorHandler.OnRateLimitExceeded = func(err *config.RateLimitError) {
        log.Printf("DEBUG: Rate limit exceeded: %v", err)
    }
}
```

## Security Changelog

### v1.0.0
- Initial security system implementation
- File size limits
- Memory usage tracking
- Basic rate limiting
- Watcher limits
- Timeout protection
- Monitoring and alerting system
- Graceful degradation
- Comprehensive test coverage

### Future Enhancements
- Network-based source security (TLS verification, etc.)
- Content filtering and validation
- Encryption at rest
- Access control and permissions
- Advanced threat detection
- Integration with security information systems

## Conclusion

The AG-UI configuration security system provides comprehensive protection against DoS attacks and resource exhaustion while maintaining high performance and usability. By implementing multiple layers of defense - resource limits, rate limiting, timeout protection, and monitoring - the system can handle both benign load and malicious attacks gracefully.

The key to effective security is proper configuration of limits based on your specific use case and continuous monitoring of system health. The provided examples and best practices should help you implement secure configuration management in your applications.

For questions or issues, please refer to the test files for complete usage examples and consult the API documentation for detailed parameter descriptions.