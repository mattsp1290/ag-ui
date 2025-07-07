# State Manager Monitoring and Observability

This document describes the comprehensive monitoring and observability system for the State Manager package.

## Overview

The monitoring system provides:

- **Prometheus Metrics Integration**: Comprehensive metrics collection with Prometheus
- **Structured Logging**: Configurable structured logging with different levels
- **Health Checks**: Automated health monitoring for all system components
- **Alert Management**: Configurable alerting with multiple notification channels
- **Resource Monitoring**: System resource usage tracking
- **Audit Integration**: Deep integration with the existing audit system
- **Configuration Management**: Flexible configuration from files, environment, or code

## Components

### 1. MonitoringSystem (`monitoring.go`)

The core monitoring system that coordinates all observability features.

```go
// Create a monitoring system
config := DefaultMonitoringConfig()
monitoringSystem, err := NewMonitoringSystem(config)
```

### 2. Health Checks (`health_checks.go`)

Automated health checks for system components:

- **StateManagerHealthCheck**: Validates core state manager functionality
- **MemoryHealthCheck**: Monitors memory usage and GC performance
- **StoreHealthCheck**: Verifies state store availability
- **EventHandlerHealthCheck**: Checks event processing health
- **RateLimiterHealthCheck**: Monitors rate limiting systems
- **AuditHealthCheck**: Validates audit log integrity
- **CustomHealthCheck**: Support for custom health checks

### 3. Alert Notifiers (`alert_notifiers.go`)

Multiple notification channels for alerts:

- **LogAlertNotifier**: Logs alerts to structured logger
- **WebhookAlertNotifier**: Sends alerts to HTTP webhooks
- **SlackAlertNotifier**: Integration with Slack channels
- **EmailAlertNotifier**: Email notifications (placeholder)
- **PagerDutyAlertNotifier**: PagerDuty integration
- **FileAlertNotifier**: Write alerts to files
- **CompositeAlertNotifier**: Send to multiple notifiers
- **ConditionalAlertNotifier**: Conditional alert routing
- **ThrottledAlertNotifier**: Prevent alert spam

### 4. Configuration (`monitoring_config.go`)

Flexible configuration system:

```go
// Builder pattern
config := NewMonitoringConfigBuilder().
    WithPrometheus("myapp", "state_manager").
    WithMetrics(true, 30*time.Second).
    WithLogging(zapcore.InfoLevel, "json", os.Stdout).
    WithTracing("myapp", "jaeger", 0.1).
    WithHealthChecks(true, 30*time.Second, 5*time.Second).
    Build()

// From environment variables
loader := NewMonitoringConfigLoader()
config := loader.LoadFromEnv()

// From JSON file
config, err := loader.LoadFromFile("monitoring.json")
```

### 5. Integration Helper (`monitoring_integration.go`)

Easy integration with existing StateManager:

```go
// Basic monitoring
integration, err := SetupBasicMonitoring(stateManager)

// Production monitoring with Slack
integration, err := SetupProductionMonitoring(stateManager, slackWebhookURL)

// Development monitoring
integration, err := SetupDevelopmentMonitoring(stateManager)
```

## Metrics

### Prometheus Metrics

The system automatically collects these metrics:

#### State Operations
- `state_operations_total`: Total number of state operations by type
- `state_operation_duration_seconds`: Duration of state operations
- `state_operation_errors_total`: Number of operation errors by type

#### Memory and Performance
- `memory_usage_bytes`: Current memory usage
- `memory_allocations_total`: Total memory allocations
- `gc_pause_duration_seconds`: Garbage collection pause times
- `object_pool_hit_rate`: Object pool efficiency percentage
- `goroutines_count`: Current number of goroutines

#### Event Processing
- `events_processed_total`: Events processed by type
- `event_processing_latency_seconds`: Event processing latency
- `event_queue_depth`: Current event queue depth

#### Storage Backend
- `storage_operations_total`: Storage operations by type
- `storage_latency_seconds`: Storage operation latency
- `storage_errors_total`: Storage errors by type

#### Connection Pool
- `connection_pool_size`: Total connections in pool
- `connection_pool_active`: Active connections
- `connection_pool_waiting`: Waiting connections
- `connection_pool_errors_total`: Connection errors

#### Rate Limiting
- `rate_limit_requests_total`: Rate limit requests by status
- `rate_limit_rejects_total`: Rate limit rejections
- `rate_limit_utilization`: Rate limit utilization percentage

#### Health Checks
- `health_check_status`: Health check status (1=healthy, 0=unhealthy)
- `health_check_duration_seconds`: Health check execution time

#### Audit System
- `audit_logs_written_total`: Audit logs written by action
- `audit_log_errors_total`: Audit logging errors
- `audit_verification_time_seconds`: Audit verification time

## Usage Examples

### Basic Setup

```go
// Create state manager
opts := DefaultManagerOptions()
stateManager, err := NewStateManager(opts)
if err != nil {
    log.Fatal(err)
}
defer stateManager.Close()

// Set up monitoring
integration, err := SetupBasicMonitoring(stateManager)
if err != nil {
    log.Fatal(err)
}
defer integration.Shutdown(context.Background())

// Record operations
start := time.Now()
// ... perform operation ...
integration.RecordCustomMetric("my_operation", time.Since(start), err)
```

### Production Setup

```go
// Production configuration
config := NewMonitoringConfigBuilder().
    WithPrometheus("myapp", "state_manager").
    WithMetrics(true, 15*time.Second).
    WithLogging(zapcore.InfoLevel, "json", os.Stdout).
    WithHealthChecks(true, 30*time.Second, 5*time.Second).
    WithAlertThresholds(AlertThresholds{
        ErrorRate:             1.0,
        P95LatencyMs:          50,
        MemoryUsagePercent:    70,
    }).
    Build()

// Add alert notifiers
slackNotifier := NewSlackAlertNotifier(webhookURL, "#alerts", "StateManager")
logNotifier := NewLogAlertNotifier(logger)
throttledSlack := NewThrottledAlertNotifier(slackNotifier, 5*time.Minute)

config.AlertNotifiers = []AlertNotifier{logNotifier, throttledSlack}

// Create integration
integration, err := NewMonitoringIntegration(stateManager, config)

// Start metrics server
go integration.StartMetricsServer() // Serves on :8080
```

### Custom Health Checks

```go
// Add custom health check
integration.RegisterCustomHealthCheck("database", func(ctx context.Context) error {
    // Check database connectivity
    return db.PingContext(ctx)
})

integration.RegisterCustomHealthCheck("external_api", func(ctx context.Context) error {
    // Check external API
    resp, err := http.Get("https://api.example.com/health")
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return fmt.Errorf("API unhealthy: %d", resp.StatusCode)
    }
    return nil
})
```

### HTTP Middleware

```go
// Add monitoring middleware to HTTP handlers
handler := integration.MonitoringMiddleware(http.HandlerFunc(myHandler))
http.Handle("/api/", handler)
```

## Configuration

### Environment Variables

```bash
# Prometheus
export MONITORING_PROMETHEUS_ENABLED=true
export MONITORING_PROMETHEUS_NAMESPACE=myapp
export MONITORING_PROMETHEUS_SUBSYSTEM=state_manager

# Metrics
export MONITORING_METRICS_ENABLED=true
export MONITORING_METRICS_INTERVAL=30s

# Logging
export MONITORING_LOG_LEVEL=info
export MONITORING_LOG_FORMAT=json
export MONITORING_LOG_SAMPLING=true

# Health checks
export MONITORING_HEALTH_CHECKS_ENABLED=true
export MONITORING_HEALTH_CHECK_INTERVAL=30s
export MONITORING_HEALTH_CHECK_TIMEOUT=5s

# Alert thresholds
export MONITORING_ALERT_ERROR_RATE=5.0
export MONITORING_ALERT_P95_LATENCY_MS=100
export MONITORING_ALERT_MEMORY_USAGE_PERCENT=80

# Resource monitoring
export MONITORING_RESOURCE_MONITORING_ENABLED=true
export MONITORING_RESOURCE_SAMPLE_INTERVAL=10s

# Audit integration
export MONITORING_AUDIT_INTEGRATION=true
export MONITORING_AUDIT_SEVERITY_LEVEL=info
```

### JSON Configuration File

```json
{
  "prometheus": {
    "enabled": true,
    "namespace": "myapp",
    "subsystem": "state_manager"
  },
  "metrics": {
    "enabled": true,
    "interval": "30s"
  },
  "logging": {
    "level": "info",
    "format": "json",
    "structured": true,
    "sampling": true
  },
  "health_checks": {
    "enabled": true,
    "interval": "30s",
    "timeout": "5s"
  },
  "alert_thresholds": {
    "error_rate": 5.0,
    "error_rate_window": "5m",
    "p95_latency_ms": 100,
    "p99_latency_ms": 500,
    "memory_usage_percent": 80,
    "gc_pause_ms": 50,
    "connection_pool_util": 85,
    "queue_depth": 1000
  },
  "resource_monitoring": {
    "enabled": true,
    "sample_interval": "10s"
  },
  "audit": {
    "integration": true,
    "severity_level": "info"
  }
}
```

## Endpoints

When metrics server is enabled, the following endpoints are available:

- `GET /metrics` - Prometheus metrics
- `GET /health` - Basic health check (returns 200 if healthy)
- `GET /health/detailed` - Detailed health status (JSON)
- `GET /metrics/summary` - Metrics summary (JSON)

## Alert Thresholds

Default alert thresholds:

| Metric | Threshold | Description |
|--------|-----------|-------------|
| Error Rate | 5% | Percentage of operations that fail |
| P95 Latency | 100ms | 95th percentile operation latency |
| P99 Latency | 500ms | 99th percentile operation latency |
| Memory Usage | 80% | Memory usage percentage |
| GC Pause | 50ms | Garbage collection pause time |
| Connection Pool | 85% | Connection pool utilization |
| Queue Depth | 1000 | Event queue depth |
| Rate Limit Rejects | 100 | Rate limit rejections |

## Best Practices

### Production Deployment

1. **Use appropriate sampling rates** for tracing (1-10%)
2. **Set realistic alert thresholds** based on your SLA
3. **Use throttled notifiers** to prevent alert spam
4. **Monitor the monitoring system** itself
5. **Implement proper log rotation** for file outputs
6. **Use structured logging** in production
7. **Enable audit integration** for compliance

### Development

1. **Use debug logging** for troubleshooting
2. **Enable 100% trace sampling** for debugging
3. **Set lenient alert thresholds** to reduce noise
4. **Use console log format** for readability

### Performance

1. **Monitor resource usage** of the monitoring system
2. **Use appropriate metrics intervals** (don't over-sample)
3. **Consider metric cardinality** impact
4. **Use object pooling** where available
5. **Enable compression** for network transmission

## Integration with Existing Audit System

The monitoring system integrates deeply with the existing audit system:

- **Correlation IDs**: Links monitoring events with audit logs
- **Severity Levels**: Maps monitoring alerts to audit severity
- **Context Propagation**: Preserves audit context in monitoring
- **Verification**: Monitors audit log integrity
- **Performance**: Tracks audit system performance

## Troubleshooting

### Common Issues

1. **High Memory Usage**
   - Check object pool efficiency
   - Monitor GC pause times
   - Verify metric cardinality

2. **Missing Metrics**
   - Verify Prometheus configuration
   - Check metrics endpoint availability
   - Validate metric registration

3. **Failed Health Checks**
   - Check component dependencies
   - Verify network connectivity
   - Review health check timeouts

4. **Alert Spam**
   - Use throttled notifiers
   - Adjust alert thresholds
   - Implement conditional routing

### Debug Mode

Enable debug logging to troubleshoot issues:

```go
config := NewMonitoringConfigBuilder().
    WithLogging(zapcore.DebugLevel, "console", os.Stdout).
    Build()
```

## Future Enhancements

- **OpenTelemetry Integration**: Full distributed tracing support
- **Custom Metrics**: User-defined metrics collection
- **Dashboard Templates**: Pre-built Grafana dashboards
- **SLI/SLO Monitoring**: Service level monitoring
- **Anomaly Detection**: ML-based anomaly detection
- **Cost Monitoring**: Resource cost tracking

## Contributing

When adding new monitoring features:

1. Follow the existing patterns for metrics naming
2. Add comprehensive tests for new components
3. Update documentation and examples
4. Consider backward compatibility
5. Implement proper error handling
6. Add configuration options where appropriate