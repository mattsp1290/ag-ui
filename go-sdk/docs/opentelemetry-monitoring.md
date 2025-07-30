# OpenTelemetry Monitoring Setup Guide

Complete guide for setting up OpenTelemetry monitoring with the AG-UI Go SDK, including tracing, metrics, and logs integration with popular observability backends.

## Table of Contents

- [Overview](#overview)
- [OpenTelemetry Configuration](#opentelemetry-configuration)
- [Tracing Setup](#tracing-setup)
- [Metrics Collection](#metrics-collection)
- [Logging Integration](#logging-integration)
- [Backend Integration](#backend-integration)
- [Custom Instrumentation](#custom-instrumentation)
- [Performance Optimization](#performance-optimization)
- [Troubleshooting](#troubleshooting)

## Overview

OpenTelemetry provides vendor-neutral observability for AG-UI applications with comprehensive telemetry data collection across traces, metrics, and logs.

### Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   AG-UI App     │    │  OpenTelemetry  │    │   Observability │
│                 │    │    Collector    │    │     Backend     │
│  ┌───────────┐  │    │                 │    │                 │
│  │  Traces   │──┼────┼→ Trace Pipeline ├────┼→ Jaeger/Zipkin  │
│  │  Metrics  │──┼────┼→ Metrics Pipeline├───┼→ Prometheus     │
│  │  Logs     │──┼────┼→ Log Pipeline   │    │  Grafana        │
│  └───────────┘  │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Key Benefits

- **Unified Observability**: Single standard for traces, metrics, and logs
- **Vendor Neutral**: Works with multiple backend systems
- **Auto-instrumentation**: Automatic instrumentation for common libraries
- **Custom Instrumentation**: Rich APIs for application-specific telemetry
- **Performance**: High-performance, low-overhead data collection

## OpenTelemetry Configuration

### Basic Configuration

```go
// monitoring/otel.go
package monitoring

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
    "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/log/global"
    "go.opentelemetry.io/otel/metric"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/sdk/log"
    sdkmetric "go.opentelemetry.io/otel/sdk/metric"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
    "go.opentelemetry.io/otel/trace"
)

type OtelConfig struct {
    ServiceName        string            `json:"service_name"`
    ServiceVersion     string            `json:"service_version"`
    Environment        string            `json:"environment"`
    OTLPEndpoint      string            `json:"otlp_endpoint"`
    OTLPHeaders       map[string]string `json:"otlp_headers"`
    TracingSampleRate float64           `json:"tracing_sample_rate"`
    EnableTracing     bool              `json:"enable_tracing"`
    EnableMetrics     bool              `json:"enable_metrics"`
    EnableLogging     bool              `json:"enable_logging"`
    ResourceAttributes map[string]string `json:"resource_attributes"`
}

type OtelProvider struct {
    config         *OtelConfig
    tracerProvider *sdktrace.TracerProvider
    meterProvider  metric.MeterProvider
    loggerProvider *log.LoggerProvider
    tracer         trace.Tracer
    meter          metric.Meter
    resource       *resource.Resource
}

func NewOtelProvider(config *OtelConfig) (*OtelProvider, error) {
    ctx := context.Background()
    
    // Create resource
    res, err := createResource(config)
    if err != nil {
        return nil, fmt.Errorf("failed to create resource: %w", err)
    }
    
    provider := &OtelProvider{
        config:   config,
        resource: res,
    }
    
    // Initialize tracing
    if config.EnableTracing {
        if err := provider.initTracing(ctx); err != nil {
            return nil, fmt.Errorf("failed to initialize tracing: %w", err)
        }
    }
    
    // Initialize metrics
    if config.EnableMetrics {
        if err := provider.initMetrics(ctx); err != nil {
            return nil, fmt.Errorf("failed to initialize metrics: %w", err)
        }
    }
    
    // Initialize logging
    if config.EnableLogging {
        if err := provider.initLogging(ctx); err != nil {
            return nil, fmt.Errorf("failed to initialize logging: %w", err)
        }
    }
    
    // Set global providers
    if provider.tracerProvider != nil {
        otel.SetTracerProvider(provider.tracerProvider)
        provider.tracer = provider.tracerProvider.Tracer(config.ServiceName)
    }
    
    if provider.meterProvider != nil {
        otel.SetMeterProvider(provider.meterProvider)
        provider.meter = provider.meterProvider.Meter(config.ServiceName)
    }
    
    if provider.loggerProvider != nil {
        global.SetLoggerProvider(provider.loggerProvider)
    }
    
    // Set global propagator
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ))
    
    return provider, nil
}

func createResource(config *OtelConfig) (*resource.Resource, error) {
    attrs := []attribute.KeyValue{
        semconv.ServiceNameKey.String(config.ServiceName),
        semconv.ServiceVersionKey.String(config.ServiceVersion),
        semconv.DeploymentEnvironmentKey.String(config.Environment),
        semconv.ServiceInstanceIDKey.String(generateInstanceID()),
    }
    
    // Add custom resource attributes
    for key, value := range config.ResourceAttributes {
        attrs = append(attrs, attribute.String(key, value))
    }
    
    return resource.New(context.Background(),
        resource.WithAttributes(attrs...),
        resource.WithFromEnv(),
        resource.WithProcess(),
        resource.WithOS(),
        resource.WithContainer(),
        resource.WithHost(),
    )
}

func (p *OtelProvider) initTracing(ctx context.Context) error {
    // Create OTLP trace exporter
    traceExporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(p.config.OTLPEndpoint),
        otlptracegrpc.WithHeaders(p.config.OTLPHeaders),
        otlptracegrpc.WithInsecure(), // Use WithTLSCredentials() in production
    )
    if err != nil {
        return fmt.Errorf("failed to create trace exporter: %w", err)
    }
    
    // Create trace provider
    p.tracerProvider = sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(traceExporter,
            sdktrace.WithBatchTimeout(5*time.Second),
            sdktrace.WithMaxExportBatchSize(512),
            sdktrace.WithMaxQueueSize(2048),
        ),
        sdktrace.WithResource(p.resource),
        sdktrace.WithSampler(sdktrace.TraceIDRatioBased(p.config.TracingSampleRate)),
    )
    
    return nil
}

func (p *OtelProvider) initMetrics(ctx context.Context) error {
    // Create OTLP metric exporter
    metricExporter, err := otlpmetricgrpc.New(ctx,
        otlpmetricgrpc.WithEndpoint(p.config.OTLPEndpoint),
        otlpmetricgrpc.WithHeaders(p.config.OTLPHeaders),
        otlpmetricgrpc.WithInsecure(), // Use WithTLSCredentials() in production
    )
    if err != nil {
        return fmt.Errorf("failed to create metric exporter: %w", err)
    }
    
    // Create meter provider
    p.meterProvider = sdkmetric.NewMeterProvider(
        sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter,
            sdkmetric.WithInterval(30*time.Second),
        )),
        sdkmetric.WithResource(p.resource),
    )
    
    return nil
}

func (p *OtelProvider) initLogging(ctx context.Context) error {
    // Create OTLP log exporter
    logExporter, err := otlploggrpc.New(ctx,
        otlploggrpc.WithEndpoint(p.config.OTLPEndpoint),
        otlploggrpc.WithHeaders(p.config.OTLPHeaders),
        otlploggrpc.WithInsecure(), // Use WithTLSCredentials() in production
    )
    if err != nil {
        return fmt.Errorf("failed to create log exporter: %w", err)
    }
    
    // Create logger provider
    p.loggerProvider = log.NewLoggerProvider(
        log.WithProcessor(log.NewBatchProcessor(logExporter,
            log.WithBatchTimeout(5*time.Second),
            log.WithMaxQueueSize(2048),
        )),
        log.WithResource(p.resource),
    )
    
    return nil
}

func (p *OtelProvider) GetTracer() trace.Tracer {
    return p.tracer
}

func (p *OtelProvider) GetMeter() metric.Meter {
    return p.meter
}

func (p *OtelProvider) Shutdown(ctx context.Context) error {
    var errs []error
    
    if p.tracerProvider != nil {
        if err := p.tracerProvider.Shutdown(ctx); err != nil {
            errs = append(errs, err)
        }
    }
    
    if p.meterProvider != nil {
        if mp, ok := p.meterProvider.(*sdkmetric.MeterProvider); ok {
            if err := mp.Shutdown(ctx); err != nil {
                errs = append(errs, err)
            }
        }
    }
    
    if p.loggerProvider != nil {
        if err := p.loggerProvider.Shutdown(ctx); err != nil {
            errs = append(errs, err)
        }
    }
    
    if len(errs) > 0 {
        return fmt.Errorf("shutdown errors: %v", errs)
    }
    
    return nil
}

func generateInstanceID() string {
    hostname, _ := os.Hostname()
    return fmt.Sprintf("%s-%d", hostname, os.Getpid())
}
```

### Environment-based Configuration

```go
// config/otel.go
func LoadOtelConfig() (*OtelConfig, error) {
    return &OtelConfig{
        ServiceName:        getEnvOrDefault("OTEL_SERVICE_NAME", "ag-ui-server"),
        ServiceVersion:     getEnvOrDefault("OTEL_SERVICE_VERSION", "1.0.0"),
        Environment:        getEnvOrDefault("OTEL_ENVIRONMENT", "production"),
        OTLPEndpoint:      getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317"),
        TracingSampleRate: getFloatEnv("OTEL_TRACES_SAMPLER_ARG", 0.1),
        EnableTracing:     getBoolEnv("OTEL_TRACES_ENABLED", true),
        EnableMetrics:     getBoolEnv("OTEL_METRICS_ENABLED", true),
        EnableLogging:     getBoolEnv("OTEL_LOGS_ENABLED", true),
        OTLPHeaders:       parseHeadersEnv("OTEL_EXPORTER_OTLP_HEADERS"),
        ResourceAttributes: parseAttributesEnv("OTEL_RESOURCE_ATTRIBUTES"),
    }, nil
}

func parseHeadersEnv(envVar string) map[string]string {
    headers := make(map[string]string)
    headerStr := os.Getenv(envVar)
    if headerStr == "" {
        return headers
    }
    
    pairs := strings.Split(headerStr, ",")
    for _, pair := range pairs {
        kv := strings.SplitN(pair, "=", 2)
        if len(kv) == 2 {
            headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
        }
    }
    
    return headers
}

func parseAttributesEnv(envVar string) map[string]string {
    attrs := make(map[string]string)
    attrStr := os.Getenv(envVar)
    if attrStr == "" {
        return attrs
    }
    
    pairs := strings.Split(attrStr, ",")
    for _, pair := range pairs {
        kv := strings.SplitN(pair, "=", 2)
        if len(kv) == 2 {
            attrs[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
        }
    }
    
    return attrs
}
```

## Tracing Setup

### Automatic Instrumentation

```go
// instrumentation/auto.go
package instrumentation

import (
    "net/http"
    
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
    "go.opentelemetry.io/contrib/instrumentation/database/sql/otelsql"
    "go.opentelemetry.io/contrib/instrumentation/github.com/redis/go-redis/v9/otelredis"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/semconv/v1.21.0/httpconv"
    "go.opentelemetry.io/otel/trace"
)

// HTTP instrumentation
func InstrumentHTTPHandler(handler http.Handler, service string) http.Handler {
    return otelhttp.NewHandler(handler, service,
        otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
        otelhttp.WithSpanOptions(trace.WithAttributes(
            attribute.String("service.name", service),
        )),
        otelhttp.WithFilter(func(r *http.Request) bool {
            // Skip health check endpoints
            return r.URL.Path != "/health" && r.URL.Path != "/ready"
        }),
    )
}

// HTTP client instrumentation
func InstrumentHTTPClient(client *http.Client, name string) *http.Client {
    client.Transport = otelhttp.NewTransport(client.Transport,
        otelhttp.WithClientTrace(func(ctx context.Context) *httptrace.ClientTrace {
            return otelhttptrace.NewClientTrace(ctx)
        }),
    )
    return client
}

// Database instrumentation
func InstrumentDatabase(driverName, dataSourceName string) error {
    return otelsql.Register(driverName,
        otelsql.WithAttributes(
            semconv.DBSystemPostgreSQL,
        ),
        otelsql.WithSpanOptions(
            otelsql.WithQuery(true),
            otelsql.WithQueryParams(false), // Don't log sensitive parameters
        ),
    )
}

// Redis instrumentation
func InstrumentRedis(rdb *redis.Client) {
    if err := otelredis.InstrumentTracing(rdb); err != nil {
        log.Printf("Failed to instrument Redis tracing: %v", err)
    }
    
    if err := otelredis.InstrumentMetrics(rdb); err != nil {
        log.Printf("Failed to instrument Redis metrics: %v", err)
    }
}
```

### Custom Tracing

```go
// tracing/custom.go
package tracing

import (
    "context"
    "fmt"
    
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/trace"
)

type EventTracer struct {
    tracer trace.Tracer
}

func NewEventTracer() *EventTracer {
    return &EventTracer{
        tracer: otel.Tracer("ag-ui/events"),
    }
}

func (t *EventTracer) TraceEventValidation(ctx context.Context, eventType string, eventID string) (context.Context, trace.Span) {
    return t.tracer.Start(ctx, "event.validation",
        trace.WithAttributes(
            attribute.String("event.type", eventType),
            attribute.String("event.id", eventID),
            attribute.String("operation", "validation"),
        ),
        trace.WithSpanKind(trace.SpanKindInternal),
    )
}

func (t *EventTracer) TraceEventProcessing(ctx context.Context, eventType string, agentName string) (context.Context, trace.Span) {
    return t.tracer.Start(ctx, "event.processing",
        trace.WithAttributes(
            attribute.String("event.type", eventType),
            attribute.String("agent.name", agentName),
            attribute.String("operation", "processing"),
        ),
        trace.WithSpanKind(trace.SpanKindServer),
    )
}

func (t *EventTracer) TraceStateUpdate(ctx context.Context, path string, operation string) (context.Context, trace.Span) {
    return t.tracer.Start(ctx, "state.update",
        trace.WithAttributes(
            attribute.String("state.path", path),
            attribute.String("state.operation", operation),
        ),
        trace.WithSpanKind(trace.SpanKindInternal),
    )
}

func (t *EventTracer) TraceToolExecution(ctx context.Context, toolName string, toolID string) (context.Context, trace.Span) {
    return t.tracer.Start(ctx, "tool.execution",
        trace.WithAttributes(
            attribute.String("tool.name", toolName),
            attribute.String("tool.id", toolID),
            attribute.String("operation", "execution"),
        ),
        trace.WithSpanKind(trace.SpanKindInternal),
    )
}

// Helper functions for common tracing patterns
func RecordError(span trace.Span, err error) {
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }
}

func RecordSuccess(span trace.Span, message string) {
    span.SetStatus(codes.Ok, message)
}

func AddEventAttribute(span trace.Span, event interface{}) {
    switch e := event.(type) {
    case *events.TextMessageContentEvent:
        span.SetAttributes(
            attribute.String("message.content_length", fmt.Sprintf("%d", len(e.Content))),
            attribute.String("message.role", e.Role),
        )
    case *events.ToolCallStartEvent:
        span.SetAttributes(
            attribute.String("tool.name", e.ToolName),
            attribute.Int("tool.args_count", len(e.Arguments)),
        )
    case *events.StateSnapshotEvent:
        span.SetAttributes(
            attribute.String("state.version", e.Version),
            attribute.Int("state.size", len(e.State)),
        )
    }
}

// Context helpers
func GetTraceID(ctx context.Context) string {
    span := trace.SpanFromContext(ctx)
    if span != nil {
        return span.SpanContext().TraceID().String()
    }
    return ""
}

func GetSpanID(ctx context.Context) string {
    span := trace.SpanFromContext(ctx)
    if span != nil {
        return span.SpanContext().SpanID().String()
    }
    return ""
}
```

### Distributed Tracing

```go
// tracing/distributed.go
package tracing

import (
    "context"
    "net/http"
    
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/trace"
)

// HTTP middleware for trace propagation
func TracingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract trace context from headers
        ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
        
        // Start new span
        tracer := otel.Tracer("ag-ui/http")
        ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path,
            trace.WithAttributes(
                httpconv.HTTPMethodKey.String(r.Method),
                httpconv.HTTPURLKey.String(r.URL.String()),
                httpconv.HTTPSchemeKey.String(r.URL.Scheme),
                httpconv.HTTPHostKey.String(r.Host),
                httpconv.HTTPUserAgentKey.String(r.UserAgent()),
            ),
            trace.WithSpanKind(trace.SpanKindServer),
        )
        defer span.End()
        
        // Inject trace context into response headers
        otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(w.Header()))
        
        // Add trace ID to response header for debugging
        w.Header().Set("X-Trace-ID", span.SpanContext().TraceID().String())
        
        // Call next handler with traced context
        next.ServeHTTP(w, r.WithContext(ctx))
        
        // Record response status
        span.SetAttributes(
            httpconv.HTTPStatusCodeKey.Int(getStatusCode(w)),
        )
        
        if statusCode := getStatusCode(w); statusCode >= 400 {
            span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
        }
    })
}

// Outgoing HTTP request tracing
func TraceHTTPRequest(ctx context.Context, req *http.Request) {
    // Inject trace context into outgoing request
    otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
}

// WebSocket tracing
func TraceWebSocketConnection(ctx context.Context, connectionID string) (context.Context, trace.Span) {
    tracer := otel.Tracer("ag-ui/websocket")
    return tracer.Start(ctx, "websocket.connection",
        trace.WithAttributes(
            attribute.String("connection.id", connectionID),
            attribute.String("connection.protocol", "websocket"),
        ),
        trace.WithSpanKind(trace.SpanKindServer),
    )
}

func TraceWebSocketMessage(ctx context.Context, messageType string, messageSize int) (context.Context, trace.Span) {
    tracer := otel.Tracer("ag-ui/websocket")
    return tracer.Start(ctx, "websocket.message",
        trace.WithAttributes(
            attribute.String("message.type", messageType),
            attribute.Int("message.size", messageSize),
        ),
        trace.WithSpanKind(trace.SpanKindInternal),
    )
}
```

## Metrics Collection

### Standard Metrics

```go
// metrics/standard.go
package metrics

import (
    "context"
    "time"
    
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

type StandardMetrics struct {
    // HTTP metrics
    httpRequestsTotal     metric.Int64Counter
    httpRequestDuration   metric.Float64Histogram
    httpRequestSize       metric.Int64Histogram
    httpResponseSize      metric.Int64Histogram
    
    // Event metrics
    eventsProcessedTotal  metric.Int64Counter
    eventProcessingTime   metric.Float64Histogram
    eventValidationTime   metric.Float64Histogram
    eventsQueueLength     metric.Int64Gauge
    
    // System metrics
    goroutineCount        metric.Int64Gauge
    memoryUsage           metric.Int64Gauge
    cpuUsage              metric.Float64Gauge
    
    // Business metrics
    activeConnections     metric.Int64Gauge
    activeUsers           metric.Int64Gauge
    messageRate           metric.Float64Counter
}

func NewStandardMetrics() (*StandardMetrics, error) {
    meter := otel.Meter("ag-ui/standard")
    
    httpRequestsTotal, err := meter.Int64Counter(
        "http_requests_total",
        metric.WithDescription("Total number of HTTP requests"),
        metric.WithUnit("1"),
    )
    if err != nil {
        return nil, err
    }
    
    httpRequestDuration, err := meter.Float64Histogram(
        "http_request_duration",
        metric.WithDescription("HTTP request duration"),
        metric.WithUnit("s"),
        metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
    )
    if err != nil {
        return nil, err
    }
    
    eventsProcessedTotal, err := meter.Int64Counter(
        "events_processed_total",
        metric.WithDescription("Total number of events processed"),
        metric.WithUnit("1"),
    )
    if err != nil {
        return nil, err
    }
    
    eventProcessingTime, err := meter.Float64Histogram(
        "event_processing_time",
        metric.WithDescription("Event processing time"),
        metric.WithUnit("s"),
        metric.WithExplicitBucketBoundaries(0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1),
    )
    if err != nil {
        return nil, err
    }
    
    activeConnections, err := meter.Int64Gauge(
        "active_connections",
        metric.WithDescription("Number of active connections"),
        metric.WithUnit("1"),
    )
    if err != nil {
        return nil, err
    }
    
    return &StandardMetrics{
        httpRequestsTotal:     httpRequestsTotal,
        httpRequestDuration:   httpRequestDuration,
        eventsProcessedTotal:  eventsProcessedTotal,
        eventProcessingTime:   eventProcessingTime,
        activeConnections:     activeConnections,
    }, nil
}

func (m *StandardMetrics) RecordHTTPRequest(ctx context.Context, method, endpoint, status string, duration time.Duration, requestSize, responseSize int64) {
    attrs := []attribute.KeyValue{
        attribute.String("method", method),
        attribute.String("endpoint", endpoint),
        attribute.String("status", status),
    }
    
    m.httpRequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
    m.httpRequestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
    
    if requestSize > 0 {
        m.httpRequestSize.Record(ctx, requestSize, metric.WithAttributes(attrs...))
    }
    
    if responseSize > 0 {
        m.httpResponseSize.Record(ctx, responseSize, metric.WithAttributes(attrs...))
    }
}

func (m *StandardMetrics) RecordEventProcessing(ctx context.Context, eventType, agent, status string, duration time.Duration) {
    attrs := []attribute.KeyValue{
        attribute.String("event_type", eventType),
        attribute.String("agent", agent),
        attribute.String("status", status),
    }
    
    m.eventsProcessedTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
    m.eventProcessingTime.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

func (m *StandardMetrics) UpdateActiveConnections(ctx context.Context, count int64) {
    m.activeConnections.Record(ctx, count)
}
```

### Custom Business Metrics

```go
// metrics/business.go
package metrics

import (
    "context"
    "time"
    
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

type BusinessMetrics struct {
    // Agent metrics
    agentActivations      metric.Int64Counter
    agentResponseTime     metric.Float64Histogram
    agentErrors          metric.Int64Counter
    
    // Tool metrics
    toolExecutions       metric.Int64Counter
    toolExecutionTime    metric.Float64Histogram
    toolErrors          metric.Int64Counter
    
    // State metrics
    stateChanges        metric.Int64Counter
    stateSize           metric.Int64Gauge
    transactionDuration metric.Float64Histogram
    
    // Message metrics
    messagesSent        metric.Int64Counter
    messagesReceived    metric.Int64Counter
    messageSize         metric.Int64Histogram
    
    // User engagement metrics
    sessionDuration     metric.Float64Histogram
    userActions         metric.Int64Counter
    conversionRate      metric.Float64Gauge
}

func NewBusinessMetrics() (*BusinessMetrics, error) {
    meter := otel.Meter("ag-ui/business")
    
    agentActivations, err := meter.Int64Counter(
        "agent_activations_total",
        metric.WithDescription("Total number of agent activations"),
        metric.WithUnit("1"),
    )
    if err != nil {
        return nil, err
    }
    
    agentResponseTime, err := meter.Float64Histogram(
        "agent_response_time",
        metric.WithDescription("Agent response time"),
        metric.WithUnit("s"),
        metric.WithExplicitBucketBoundaries(0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60),
    )
    if err != nil {
        return nil, err
    }
    
    toolExecutions, err := meter.Int64Counter(
        "tool_executions_total",
        metric.WithDescription("Total number of tool executions"),
        metric.WithUnit("1"),
    )
    if err != nil {
        return nil, err
    }
    
    stateChanges, err := meter.Int64Counter(
        "state_changes_total",
        metric.WithDescription("Total number of state changes"),
        metric.WithUnit("1"),
    )
    if err != nil {
        return nil, err
    }
    
    messagesSent, err := meter.Int64Counter(
        "messages_sent_total",
        metric.WithDescription("Total number of messages sent"),
        metric.WithUnit("1"),
    )
    if err != nil {
        return nil, err
    }
    
    return &BusinessMetrics{
        agentActivations:  agentActivations,
        agentResponseTime: agentResponseTime,
        toolExecutions:   toolExecutions,
        stateChanges:     stateChanges,
        messagesSent:     messagesSent,
    }, nil
}

func (m *BusinessMetrics) RecordAgentActivation(ctx context.Context, agentName string, responseTime time.Duration, success bool) {
    attrs := []attribute.KeyValue{
        attribute.String("agent", agentName),
        attribute.Bool("success", success),
    }
    
    m.agentActivations.Add(ctx, 1, metric.WithAttributes(attrs...))
    m.agentResponseTime.Record(ctx, responseTime.Seconds(), metric.WithAttributes(attrs...))
    
    if !success {
        m.agentErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
    }
}

func (m *BusinessMetrics) RecordToolExecution(ctx context.Context, toolName string, executionTime time.Duration, success bool) {
    attrs := []attribute.KeyValue{
        attribute.String("tool", toolName),
        attribute.Bool("success", success),
    }
    
    m.toolExecutions.Add(ctx, 1, metric.WithAttributes(attrs...))
    m.toolExecutionTime.Record(ctx, executionTime.Seconds(), metric.WithAttributes(attrs...))
    
    if !success {
        m.toolErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
    }
}

func (m *BusinessMetrics) RecordStateChange(ctx context.Context, operation string, path string, size int64) {
    attrs := []attribute.KeyValue{
        attribute.String("operation", operation),
        attribute.String("path", path),
    }
    
    m.stateChanges.Add(ctx, 1, metric.WithAttributes(attrs...))
    m.stateSize.Record(ctx, size)
}

func (m *BusinessMetrics) RecordMessage(ctx context.Context, direction string, messageType string, size int64) {
    attrs := []attribute.KeyValue{
        attribute.String("direction", direction),
        attribute.String("type", messageType),
    }
    
    if direction == "sent" {
        m.messagesSent.Add(ctx, 1, metric.WithAttributes(attrs...))
    } else {
        m.messagesReceived.Add(ctx, 1, metric.WithAttributes(attrs...))
    }
    
    m.messageSize.Record(ctx, size, metric.WithAttributes(attrs...))
}
```

## Backend Integration

### Jaeger Integration

```yaml
# jaeger-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jaeger
  namespace: observability
spec:
  replicas: 1
  selector:
    matchLabels:
      app: jaeger
  template:
    metadata:
      labels:
        app: jaeger
    spec:
      containers:
      - name: jaeger
        image: jaegertracing/all-in-one:1.50
        env:
        - name: COLLECTOR_OTLP_ENABLED
          value: "true"
        - name: SPAN_STORAGE_TYPE
          value: "elasticsearch"
        - name: ES_SERVER_URLS
          value: "http://elasticsearch:9200"
        ports:
        - containerPort: 16686
          name: ui
        - containerPort: 14250
          name: grpc
        - containerPort: 4317
          name: otlp-grpc
        - containerPort: 4318
          name: otlp-http
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "500m"
---
apiVersion: v1
kind: Service
metadata:
  name: jaeger
  namespace: observability
spec:
  selector:
    app: jaeger
  ports:
  - name: ui
    port: 16686
    targetPort: 16686
  - name: grpc
    port: 14250
    targetPort: 14250
  - name: otlp-grpc
    port: 4317
    targetPort: 4317
  - name: otlp-http
    port: 4318
    targetPort: 4318
```

### Prometheus Integration

```yaml
# prometheus-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
  namespace: observability
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
      evaluation_interval: 15s
    
    rule_files:
      - "/etc/prometheus/rules/*.yml"
    
    scrape_configs:
    - job_name: 'ag-ui-server'
      static_configs:
      - targets: ['ag-ui-server.ag-ui-production:9090']
      scrape_interval: 15s
      metrics_path: /metrics
    
    - job_name: 'otel-collector'
      static_configs:
      - targets: ['otel-collector:8889']
      scrape_interval: 30s
    
    alerting:
      alertmanagers:
      - static_configs:
        - targets:
          - alertmanager:9093
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus
  namespace: observability
spec:
  replicas: 1
  selector:
    matchLabels:
      app: prometheus
  template:
    metadata:
      labels:
        app: prometheus
    spec:
      containers:
      - name: prometheus
        image: prom/prometheus:v2.45.0
        args:
        - '--config.file=/etc/prometheus/prometheus.yml'
        - '--storage.tsdb.path=/prometheus/'
        - '--web.console.libraries=/etc/prometheus/console_libraries'
        - '--web.console.templates=/etc/prometheus/consoles'
        - '--storage.tsdb.retention.time=30d'
        - '--web.enable-lifecycle'
        - '--web.enable-admin-api'
        ports:
        - containerPort: 9090
        volumeMounts:
        - name: prometheus-config
          mountPath: /etc/prometheus
        - name: prometheus-storage
          mountPath: /prometheus
        resources:
          requests:
            memory: "1Gi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "1000m"
      volumes:
      - name: prometheus-config
        configMap:
          name: prometheus-config
      - name: prometheus-storage
        persistentVolumeClaim:
          claimName: prometheus-storage
```

### OpenTelemetry Collector

```yaml
# otel-collector-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-collector-config
  namespace: observability
data:
  config.yaml: |
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318
      
      prometheus:
        config:
          scrape_configs:
          - job_name: 'otel-collector'
            scrape_interval: 10s
            static_configs:
            - targets: ['0.0.0.0:8888']
    
    processors:
      batch:
        timeout: 1s
        send_batch_size: 1024
        send_batch_max_size: 2048
      
      memory_limiter:
        limit_mib: 512
        spike_limit_mib: 128
        check_interval: 5s
      
      resource:
        attributes:
        - key: environment
          value: production
          action: insert
        - key: cluster
          value: ag-ui-cluster
          action: insert
      
      attributes:
        actions:
        - key: http.user_agent
          action: delete
        - key: http.request.header.authorization
          action: delete
    
    exporters:
      jaeger:
        endpoint: jaeger:14250
        tls:
          insecure: true
      
      prometheus:
        endpoint: "0.0.0.0:8889"
      
      loki:
        endpoint: http://loki:3100/loki/api/v1/push
        tenant_id: ag-ui
      
      elasticsearch:
        endpoints: [http://elasticsearch:9200]
        logs_index: ag-ui-logs
        traces_index: ag-ui-traces
    
    service:
      pipelines:
        traces:
          receivers: [otlp]
          processors: [memory_limiter, batch, resource, attributes]
          exporters: [jaeger, elasticsearch]
        
        metrics:
          receivers: [otlp, prometheus]
          processors: [memory_limiter, batch, resource]
          exporters: [prometheus]
        
        logs:
          receivers: [otlp]
          processors: [memory_limiter, batch, resource]
          exporters: [loki, elasticsearch]
      
      extensions: [health_check, pprof, zpages]
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otel-collector
  namespace: observability
spec:
  replicas: 2
  selector:
    matchLabels:
      app: otel-collector
  template:
    metadata:
      labels:
        app: otel-collector
    spec:
      containers:
      - name: otel-collector
        image: otel/opentelemetry-collector-contrib:0.85.0
        command:
        - "/otelcol-contrib"
        - "--config=/etc/otelcol-contrib/config.yaml"
        ports:
        - containerPort: 4317
          name: otlp-grpc
        - containerPort: 4318
          name: otlp-http
        - containerPort: 8888
          name: metrics
        - containerPort: 8889
          name: prometheus
        volumeMounts:
        - name: config
          mountPath: /etc/otelcol-contrib
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
      volumes:
      - name: config
        configMap:
          name: otel-collector-config
```

### Grafana Dashboards

```json
{
  "dashboard": {
    "id": null,
    "title": "AG-UI Application Metrics",
    "tags": ["ag-ui", "opentelemetry"],
    "timezone": "browser",
    "panels": [
      {
        "id": 1,
        "title": "Request Rate",
        "type": "stat",
        "targets": [
          {
            "expr": "sum(rate(http_requests_total[5m]))",
            "legendFormat": "Requests/sec"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "unit": "reqps"
          }
        }
      },
      {
        "id": 2,
        "title": "Response Time",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.50, sum(rate(http_request_duration_bucket[5m])) by (le))",
            "legendFormat": "50th percentile"
          },
          {
            "expr": "histogram_quantile(0.95, sum(rate(http_request_duration_bucket[5m])) by (le))",
            "legendFormat": "95th percentile"
          },
          {
            "expr": "histogram_quantile(0.99, sum(rate(http_request_duration_bucket[5m])) by (le))",
            "legendFormat": "99th percentile"
          }
        ]
      },
      {
        "id": 3,
        "title": "Event Processing",
        "type": "graph",
        "targets": [
          {
            "expr": "sum(rate(events_processed_total[5m])) by (event_type)",
            "legendFormat": "{{event_type}}"
          }
        ]
      },
      {
        "id": 4,
        "title": "Error Rate",
        "type": "stat",
        "targets": [
          {
            "expr": "sum(rate(http_requests_total{status=~\"5..\"}[5m])) / sum(rate(http_requests_total[5m])) * 100",
            "legendFormat": "Error Rate %"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "unit": "percent",
            "max": 100,
            "thresholds": {
              "steps": [
                {"color": "green", "value": null},
                {"color": "yellow", "value": 1},
                {"color": "red", "value": 5}
              ]
            }
          }
        }
      }
    ],
    "time": {
      "from": "now-1h",
      "to": "now"
    },
    "refresh": "30s"
  }
}
```

## Custom Instrumentation

### Event Processing Instrumentation

```go
// instrumentation/events.go
package instrumentation

import (
    "context"
    "time"
    
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
    
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

type InstrumentedEventProcessor struct {
    processor events.EventProcessor
    tracer    trace.Tracer
    metrics   *StandardMetrics
}

func NewInstrumentedEventProcessor(processor events.EventProcessor, metrics *StandardMetrics) *InstrumentedEventProcessor {
    return &InstrumentedEventProcessor{
        processor: processor,
        tracer:    otel.Tracer("ag-ui/events"),
        metrics:   metrics,
    }
}

func (p *InstrumentedEventProcessor) ProcessEvent(ctx context.Context, event events.Event) error {
    // Start tracing
    ctx, span := p.tracer.Start(ctx, "event.process",
        trace.WithAttributes(
            attribute.String("event.type", string(event.Type())),
            attribute.String("event.id", event.GetID()),
        ),
    )
    defer span.End()
    
    start := time.Now()
    err := p.processor.ProcessEvent(ctx, event)
    duration := time.Since(start)
    
    // Record metrics
    status := "success"
    if err != nil {
        status = "error"
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }
    
    p.metrics.RecordEventProcessing(ctx, string(event.Type()), "default", status, duration)
    
    // Add additional span attributes
    span.SetAttributes(
        attribute.String("processing.status", status),
        attribute.Float64("processing.duration_ms", float64(duration.Nanoseconds())/1e6),
    )
    
    return err
}
```

### State Management Instrumentation

```go
// instrumentation/state.go
package instrumentation

import (
    "context"
    "time"
    
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
    
    "github.com/ag-ui/go-sdk/pkg/state"
)

type InstrumentedStateStore struct {
    store   *state.StateStore
    tracer  trace.Tracer
    metrics *BusinessMetrics
}

func NewInstrumentedStateStore(store *state.StateStore, metrics *BusinessMetrics) *InstrumentedStateStore {
    return &InstrumentedStateStore{
        store:   store,
        tracer:  otel.Tracer("ag-ui/state"),
        metrics: metrics,
    }
}

func (s *InstrumentedStateStore) Set(ctx context.Context, path string, value interface{}) error {
    ctx, span := s.tracer.Start(ctx, "state.set",
        trace.WithAttributes(
            attribute.String("state.path", path),
            attribute.String("state.operation", "set"),
        ),
    )
    defer span.End()
    
    start := time.Now()
    err := s.store.Set(path, value)
    duration := time.Since(start)
    
    // Calculate state size
    stateSize := int64(len(fmt.Sprintf("%v", value)))
    
    // Record metrics
    s.metrics.RecordStateChange(ctx, "set", path, stateSize)
    
    // Add span attributes
    span.SetAttributes(
        attribute.Int64("state.size_bytes", stateSize),
        attribute.Float64("operation.duration_ms", float64(duration.Nanoseconds())/1e6),
    )
    
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }
    
    return err
}

func (s *InstrumentedStateStore) Get(ctx context.Context, path string) (interface{}, error) {
    ctx, span := s.tracer.Start(ctx, "state.get",
        trace.WithAttributes(
            attribute.String("state.path", path),
            attribute.String("state.operation", "get"),
        ),
    )
    defer span.End()
    
    start := time.Now()
    value, err := s.store.Get(path)
    duration := time.Since(start)
    
    // Add span attributes
    span.SetAttributes(
        attribute.Float64("operation.duration_ms", float64(duration.Nanoseconds())/1e6),
        attribute.Bool("cache.hit", err == nil),
    )
    
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }
    
    return value, err
}

func (s *InstrumentedStateStore) BeginTransaction(ctx context.Context) *InstrumentedTransaction {
    ctx, span := s.tracer.Start(ctx, "state.transaction",
        trace.WithAttributes(
            attribute.String("transaction.type", "state"),
        ),
    )
    
    tx := s.store.Begin()
    return &InstrumentedTransaction{
        transaction: tx,
        span:        span,
        metrics:     s.metrics,
        startTime:   time.Now(),
    }
}

type InstrumentedTransaction struct {
    transaction *state.StateTransaction
    span        trace.Span
    metrics     *BusinessMetrics
    startTime   time.Time
}

func (t *InstrumentedTransaction) Commit(ctx context.Context) error {
    defer t.span.End()
    
    err := t.transaction.Commit()
    duration := time.Since(t.startTime)
    
    status := "success"
    if err != nil {
        status = "error"
        t.span.RecordError(err)
        t.span.SetStatus(codes.Error, err.Error())
    }
    
    t.span.SetAttributes(
        attribute.String("transaction.status", status),
        attribute.Float64("transaction.duration_ms", float64(duration.Nanoseconds())/1e6),
    )
    
    t.metrics.transactionDuration.Record(ctx, duration.Seconds())
    
    return err
}

func (t *InstrumentedTransaction) Rollback(ctx context.Context) error {
    defer t.span.End()
    
    err := t.transaction.Rollback()
    
    t.span.SetAttributes(
        attribute.String("transaction.status", "rollback"),
    )
    
    if err != nil {
        t.span.RecordError(err)
    }
    
    return err
}
```

## Performance Optimization

### Sampling Strategies

```go
// sampling/strategies.go
package sampling

import (
    "context"
    
    "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/trace"
)

// Adaptive sampler that adjusts based on system load
type AdaptiveSampler struct {
    baseSampler    trace.Sampler
    loadThreshold  float64
    currentLoad    func() float64
    highLoadRate   float64
    normalLoadRate float64
}

func NewAdaptiveSampler(normalRate, highLoadRate, loadThreshold float64) *AdaptiveSampler {
    return &AdaptiveSampler{
        baseSampler:    trace.TraceIDRatioBased(normalRate),
        loadThreshold:  loadThreshold,
        currentLoad:    getCurrentSystemLoad,
        highLoadRate:   highLoadRate,
        normalLoadRate: normalRate,
    }
}

func (s *AdaptiveSampler) ShouldSample(parameters trace.SamplingParameters) trace.SamplingResult {
    load := s.currentLoad()
    
    var sampler trace.Sampler
    if load > s.loadThreshold {
        sampler = trace.TraceIDRatioBased(s.highLoadRate)
    } else {
        sampler = trace.TraceIDRatioBased(s.normalLoadRate)
    }
    
    return sampler.ShouldSample(parameters)
}

func (s *AdaptiveSampler) Description() string {
    return "AdaptiveSampler"
}

// Priority-based sampler for critical operations
type PrioritySampler struct {
    defaultSampler trace.Sampler
    priorities     map[string]float64
}

func NewPrioritySampler(defaultRate float64) *PrioritySampler {
    return &PrioritySampler{
        defaultSampler: trace.TraceIDRatioBased(defaultRate),
        priorities: map[string]float64{
            "event.validation": 1.0,    // Always sample validation
            "event.processing": 0.5,    // Sample 50% of processing
            "state.update":     0.3,    // Sample 30% of state updates
            "http.request":     0.1,    // Sample 10% of HTTP requests
        },
    }
}

func (s *PrioritySampler) ShouldSample(parameters trace.SamplingParameters) trace.SamplingResult {
    spanName := parameters.Name
    
    if rate, exists := s.priorities[spanName]; exists {
        sampler := trace.TraceIDRatioBased(rate)
        return sampler.ShouldSample(parameters)
    }
    
    return s.defaultSampler.ShouldSample(parameters)
}

func (s *PrioritySampler) Description() string {
    return "PrioritySampler"
}

func getCurrentSystemLoad() float64 {
    // Implement system load calculation
    // This could monitor CPU, memory, or other metrics
    return 0.5 // Placeholder
}
```

### Batch Processing

```go
// batching/config.go
package batching

import (
    "time"
    
    "go.opentelemetry.io/otel/sdk/trace"
)

func OptimizedBatchConfig() trace.BatchSpanProcessorOption {
    return trace.WithBatchTimeout(2 * time.Second)
}

func ProductionBatchConfig() []trace.BatchSpanProcessorOption {
    return []trace.BatchSpanProcessorOption{
        trace.WithMaxExportBatchSize(512),
        trace.WithBatchTimeout(5 * time.Second),
        trace.WithMaxQueueSize(2048),
        trace.WithExportTimeout(30 * time.Second),
    }
}

func HighThroughputBatchConfig() []trace.BatchSpanProcessorOption {
    return []trace.BatchSpanProcessorOption{
        trace.WithMaxExportBatchSize(1024),
        trace.WithBatchTimeout(1 * time.Second),
        trace.WithMaxQueueSize(4096),
        trace.WithExportTimeout(10 * time.Second),
    }
}
```

## Troubleshooting

### Common Issues and Solutions

#### Issue: High Memory Usage

```go
// monitoring/memory.go
package monitoring

import (
    "context"
    "runtime"
    "time"
    
    "go.opentelemetry.io/otel/metric"
)

type MemoryMonitor struct {
    memUsage    metric.Int64Gauge
    gcDuration  metric.Float64Histogram
    goroutines  metric.Int64Gauge
}

func NewMemoryMonitor(meter metric.Meter) (*MemoryMonitor, error) {
    memUsage, err := meter.Int64Gauge(
        "memory_usage_bytes",
        metric.WithDescription("Current memory usage in bytes"),
    )
    if err != nil {
        return nil, err
    }
    
    gcDuration, err := meter.Float64Histogram(
        "gc_duration_seconds",
        metric.WithDescription("Garbage collection duration"),
    )
    if err != nil {
        return nil, err
    }
    
    goroutines, err := meter.Int64Gauge(
        "goroutines",
        metric.WithDescription("Number of goroutines"),
    )
    if err != nil {
        return nil, err
    }
    
    return &MemoryMonitor{
        memUsage:   memUsage,
        gcDuration: gcDuration,
        goroutines: goroutines,
    }, nil
}

func (m *MemoryMonitor) StartMonitoring(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            m.collectMetrics(ctx)
        }
    }
}

func (m *MemoryMonitor) collectMetrics(ctx context.Context) {
    var stats runtime.MemStats
    runtime.ReadMemStats(&stats)
    
    m.memUsage.Record(ctx, int64(stats.Alloc))
    m.goroutines.Record(ctx, int64(runtime.NumGoroutine()))
    
    // Force GC and measure duration
    start := time.Now()
    runtime.GC()
    gcDuration := time.Since(start)
    
    m.gcDuration.Record(ctx, gcDuration.Seconds())
}
```

#### Issue: Trace Export Failures

```go
// troubleshooting/traces.go
package troubleshooting

import (
    "context"
    "log"
    "time"
    
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/trace"
)

type ResilientTraceExporter struct {
    primary   trace.SpanExporter
    fallback  trace.SpanExporter
    retries   int
    timeout   time.Duration
}

func NewResilientTraceExporter(primaryEndpoint, fallbackEndpoint string) (*ResilientTraceExporter, error) {
    primary, err := otlptrace.New(context.Background(),
        otlptracegrpc.NewClient(
            otlptracegrpc.WithEndpoint(primaryEndpoint),
            otlptracegrpc.WithInsecure(),
            otlptracegrpc.WithTimeout(10*time.Second),
        ),
    )
    if err != nil {
        return nil, err
    }
    
    fallback, err := otlptrace.New(context.Background(),
        otlptracegrpc.NewClient(
            otlptracegrpc.WithEndpoint(fallbackEndpoint),
            otlptracegrpc.WithInsecure(),
            otlptracegrpc.WithTimeout(10*time.Second),
        ),
    )
    if err != nil {
        return nil, err
    }
    
    return &ResilientTraceExporter{
        primary:  primary,
        fallback: fallback,
        retries:  3,
        timeout:  30 * time.Second,
    }, nil
}

func (e *ResilientTraceExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
    // Try primary exporter first
    err := e.exportWithRetries(ctx, e.primary, spans)
    if err == nil {
        return nil
    }
    
    log.Printf("Primary exporter failed: %v, trying fallback", err)
    
    // Try fallback exporter
    return e.exportWithRetries(ctx, e.fallback, spans)
}

func (e *ResilientTraceExporter) exportWithRetries(ctx context.Context, exporter trace.SpanExporter, spans []trace.ReadOnlySpan) error {
    var lastErr error
    
    for i := 0; i < e.retries; i++ {
        exportCtx, cancel := context.WithTimeout(ctx, e.timeout)
        err := exporter.ExportSpans(exportCtx, spans)
        cancel()
        
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        // Exponential backoff
        if i < e.retries-1 {
            backoff := time.Duration(1<<uint(i)) * time.Second
            time.Sleep(backoff)
        }
    }
    
    return lastErr
}

func (e *ResilientTraceExporter) Shutdown(ctx context.Context) error {
    err1 := e.primary.Shutdown(ctx)
    err2 := e.fallback.Shutdown(ctx)
    
    if err1 != nil {
        return err1
    }
    return err2
}
```

### Debug Configuration

```go
// debug/config.go
package debug

import (
    "log"
    "os"
    
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
    "go.opentelemetry.io/otel/sdk/trace"
)

func EnableDebugMode() error {
    // Create stdout exporter for debugging
    exporter, err := stdouttrace.New(
        stdouttrace.WithPrettyPrint(),
        stdouttrace.WithWriter(os.Stdout),
    )
    if err != nil {
        return err
    }
    
    // Create trace provider with debug exporter
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithSampler(trace.AlwaysSample()), // Sample all traces in debug mode
    )
    
    otel.SetTracerProvider(tp)
    
    // Enable debug logging
    otel.SetLogger(log.Default())
    
    return nil
}
```

This comprehensive OpenTelemetry monitoring setup guide provides everything needed to implement enterprise-grade observability for AG-UI applications with proper tracing, metrics, and logging integration.