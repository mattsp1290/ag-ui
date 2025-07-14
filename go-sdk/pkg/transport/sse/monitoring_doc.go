/*
Package sse provides comprehensive monitoring and observability for HTTP Server-Sent Events transport.

# Monitoring System Overview

The SSE monitoring system provides production-ready observability with:
  - Real-time connection metrics and statistics
  - Event throughput monitoring and performance tracking
  - Error rate tracking with configurable alerting
  - Performance benchmarking capabilities
  - Health check endpoints for service monitoring
  - Prometheus metrics integration
  - Structured logging with configurable levels
  - Distributed tracing support (OpenTelemetry)
  - Custom metrics and alerts for SSE-specific events

# Architecture

The monitoring system consists of several key components:

1. **MonitoringSystem**: The main coordinator that manages all monitoring components
2. **ConnectionTracker**: Tracks active connections and connection lifecycle events
3. **EventTracker**: Monitors event flow, processing times, and error rates
4. **PerformanceTracker**: Tracks latency, throughput, and performance benchmarks
5. **ResourceMonitor**: Monitors system resources (CPU, memory, goroutines)
6. **AlertManager**: Manages alerts based on configurable thresholds
7. **HealthChecks**: Provides health check endpoints for service monitoring

# Metrics

The system exposes comprehensive Prometheus metrics:

## Connection Metrics
  - sse_transport_connections_total: Total number of connections by status
  - sse_transport_connections_active: Current active connections
  - sse_transport_connection_duration_seconds: Connection duration histogram
  - sse_transport_connection_errors_total: Connection errors by type
  - sse_transport_reconnection_attempts_total: Reconnection attempts

## Event Metrics
  - sse_transport_events_received_total: Events received by type
  - sse_transport_events_sent_total: Events sent by type
  - sse_transport_events_processed_total: Events processed by type and status
  - sse_transport_event_processing_latency_seconds: Processing latency
  - sse_transport_event_size_bytes: Event size distribution
  - sse_transport_events_dropped_total: Dropped events by reason

## Throughput Metrics
  - sse_transport_bytes_received_total: Total bytes received
  - sse_transport_bytes_sent_total: Total bytes sent
  - sse_transport_messages_per_second: Current message throughput
  - sse_transport_bytes_per_second: Current bytes throughput

## Performance Metrics
  - sse_transport_request_latency_seconds: HTTP request latency
  - sse_transport_stream_latency_seconds: SSE stream latency
  - sse_transport_parse_latency_seconds: Event parsing latency
  - sse_transport_serialization_latency_seconds: Serialization latency

## Resource Metrics
  - sse_transport_memory_usage_bytes: Current memory usage
  - sse_transport_goroutine_count: Number of goroutines
  - sse_transport_cpu_usage_percent: CPU usage percentage
  - sse_transport_buffer_utilization_percent: Buffer utilization

## Health Metrics
  - sse_transport_health_check_status: Health check status (1=healthy, 0=unhealthy)
  - sse_transport_health_check_duration_seconds: Health check duration
  - sse_transport_health_check_failures_total: Health check failures

# Configuration

The monitoring system is highly configurable:

	config := sse.MonitoringConfig{
		Enabled: true,
		Metrics: sse.MetricsConfig{
			Enabled:  true,
			Interval: 30 * time.Second,
			Prometheus: sse.PrometheusConfig{
				Enabled:   true,
				Namespace: "myapp",
				Subsystem: "sse",
			},
		},
		Logging: sse.LoggingConfig{
			Enabled:    true,
			Level:      zapcore.InfoLevel,
			Format:     "json",
			Structured: true,
		},
		HealthChecks: sse.HealthChecksConfig{
			Enabled:  true,
			Interval: 30 * time.Second,
			Timeout:  5 * time.Second,
		},
		Alerting: sse.AlertingConfig{
			Enabled: true,
			Thresholds: sse.AlertThresholds{
				ErrorRate:       5.0,  // 5% error rate
				Latency:         1000, // 1000ms
				ConnectionCount: 1000, // 1000 connections
			},
		},
	}

# Usage Examples

## Basic Setup

	// Create monitoring system
	monitoring, err := sse.NewMonitoringSystem(config)
	if err != nil {
		log.Fatal(err)
	}
	defer monitoring.Shutdown(context.Background())

	// Track connections
	monitoring.RecordConnectionEstablished(connID, remoteAddr, userAgent)
	monitoring.RecordConnectionClosed(connID, reason)

	// Track events
	monitoring.RecordEventReceived(connID, eventType, size)
	monitoring.RecordEventProcessed(eventType, duration, err)

## Health Checks

	// Register custom health check
	monitoring.RegisterHealthCheck(&MyHealthCheck{})

	// Get health status
	status := monitoring.GetHealthStatus()

## Performance Benchmarking

	// Start benchmark
	benchmark := monitoring.StartBenchmark("operation-name")

	// Perform operations...

	// Complete benchmark
	monitoring.CompleteBenchmark(benchmark)

## Prometheus Integration

	// Expose metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

## Custom Alerts

	// Alerts are automatically triggered based on thresholds
	// Add custom notifiers for alert delivery
	monitoring.AddAlertNotifier(&SlackNotifier{})

# Best Practices

1. **Enable monitoring in production**: Always enable monitoring for production deployments
2. **Configure appropriate thresholds**: Set alert thresholds based on your SLAs
3. **Use structured logging**: Enable structured logging for better log analysis
4. **Monitor resource usage**: Keep track of memory and CPU usage
5. **Set up dashboards**: Create Grafana dashboards using the exposed metrics
6. **Implement health checks**: Add custom health checks for critical dependencies
7. **Use tracing for debugging**: Enable tracing for complex request flows
8. **Benchmark regularly**: Run performance benchmarks to track regressions

# Integration with Observability Stack

The monitoring system integrates well with common observability tools:

- **Prometheus**: Scrape metrics from /metrics endpoint
- **Grafana**: Visualize metrics with pre-built dashboards
- **Jaeger/Zipkin**: Distributed tracing for request flows
- **ELK Stack**: Structured logs for analysis
- **AlertManager**: Advanced alert routing and grouping
- **PagerDuty/Slack**: Alert notifications

# Performance Considerations

The monitoring system is designed for minimal overhead:
- Metrics collection uses atomic operations
- Sampling is configurable for high-traffic scenarios
- Background tasks run in separate goroutines
- Resource monitoring has minimal impact
- Configurable buffer sizes prevent memory issues
*/
package sse
