# AG-UI Go CLI with SSE Health Monitoring

A Go CLI client for AG-UI with built-in SSE (Server-Sent Events) health monitoring and metrics collection.

## Features

### SSE Health Monitoring
- **Connection lifecycle tracking**: Monitor connect/disconnect events with timestamps
- **Atomic counters**: Thread-safe metrics collection using `sync/atomic`
- **Real-time metrics**: Track bytes read, frames parsed, errors, and reconnection attempts
- **Rolling window rate calculation**: Accurate events/sec measurement with configurable window size

### Metrics Collection
- **Comprehensive metrics**: Connection duration, throughput, error rates, and more
- **Minimal overhead**: <1% CPU usage on idle streams
- **Concurrent-safe**: All operations are thread-safe for use in reconnection loops

### Pluggable Reporting
- **Logger Reporter**: Human-readable or JSON output via logrus
- **Callback Reporter**: Custom reporting hooks for tests or Prometheus adapters
- **Multi Reporter**: Send metrics to multiple destinations simultaneously
- **Configurable intervals**: Control sampling period via CLI flags or environment variables

## Installation

```bash
go get github.com/mattsp1290/ag-ui/go-cli
```

## Usage

### Basic Usage

```bash
# Connect to SSE endpoint with metrics
ag-ui-cli --sse-url https://api.example.com/events --metrics log --metrics-interval 5s

# JSON metrics output
ag-ui-cli --sse-url https://api.example.com/events --metrics json

# With custom headers
ag-ui-cli --sse-url https://api.example.com/events \
  --sse-headers "Authorization=Bearer token,X-API-Key=key123"

# Disable reconnection
ag-ui-cli --sse-url https://api.example.com/events --sse-reconnect=false
```

### CLI Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--sse-url` | SSE endpoint URL (required) | - |
| `--metrics` | Metrics mode: off\|log\|json | off |
| `--metrics-interval` | Metrics reporting interval | 5s |
| `--sse-timeout` | Connection timeout | 30s |
| `--sse-reconnect` | Enable automatic reconnection | true |
| `--sse-max-reconnect` | Maximum reconnection attempts (0=unlimited) | 0 |
| `--sse-initial-backoff` | Initial reconnection backoff | 1s |
| `--sse-max-backoff` | Maximum reconnection backoff | 30s |
| `--sse-headers` | Custom headers (key1=value1,key2=value2) | - |
| `--sse-buffer-size` | Event buffer size | 100 |
| `--log-level` | Log level: debug\|info\|warn\|error | info |
| `--log-format` | Log format: text\|json | text |

### Environment Variables

All flags can be configured via environment variables:

```bash
export AG_UI_SSE_URL=https://api.example.com/events
export AG_UI_METRICS=log
export AG_UI_METRICS_INTERVAL=10s
export AG_UI_LOG_LEVEL=debug
export AG_UI_LOG_FORMAT=json
```

## Metrics Output

### Human-Readable Format
```
INFO[2025-01-11T10:30:45Z] SSE Metrics  connection_id=abc123 connected=true duration=1m30s events_per_sec=12.50 events_total=1125 bytes_read=45.6 KB errors=0 reconnects=1
```

### JSON Format
```json
{
  "timestamp": 1736595045,
  "type": "sse_metrics",
  "metrics": {
    "connectionId": "abc123",
    "isConnected": true,
    "connectionDuration": 90000000000,
    "bytesRead": 46694,
    "framesRead": 1125,
    "totalEvents": 1125,
    "eventsPerSecond": 12.5,
    "parseErrors": 0,
    "reconnectAttempts": 1,
    "errorCount": 0,
    "uptimeSeconds": 90
  }
}
```

### Connection Summary
On disconnection, a comprehensive summary is emitted:

```
INFO[2025-01-11T10:35:00Z] SSE Connection Summary  
  connection_id=abc123 
  duration=5m0s 
  total_events=3600 
  total_bytes=144.0 KB 
  avg_rate=12.00 events/sec 
  error_rate=0.10% 
  bytes_per_event=40 
  reconnects=2 
  parse_errors=4
```

## Programmatic Usage

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/mattsp1290/ag-ui/go-cli/pkg/sse"
    "github.com/sirupsen/logrus"
)

func main() {
    // Configure the client
    config := sse.ClientConfig{
        URL: "https://api.example.com/events",
        Headers: map[string]string{
            "Authorization": "Bearer token",
        },
        
        // Enable metrics
        EnableMetrics:   true,
        MetricsInterval: 5 * time.Second,
        
        // Set up reporter
        MetricsReporter: sse.NewLoggerReporter(logrus.New(), "json"),
        
        // Reconnection settings
        EnableReconnect:   true,
        InitialBackoff:    time.Second,
        MaxBackoff:        30 * time.Second,
        BackoffMultiplier: 2.0,
        
        // Callbacks
        OnConnect: func(connID string) {
            log.Printf("Connected: %s", connID)
        },
        OnError: func(err error) {
            log.Printf("Error: %v", err)
        },
    }
    
    // Create client
    client, err := sse.NewClient(config)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    // Connect
    ctx := context.Background()
    if err := client.Connect(ctx); err != nil {
        log.Fatal(err)
    }
    
    // Process events
    for event := range client.Events() {
        log.Printf("Event: %s - %s", event.Type, event.Data)
        
        // Get current metrics
        metrics := client.GetMetrics()
        log.Printf("Events/sec: %.2f", metrics.EventsPerSecond)
    }
}
```

## Custom Reporters

Implement the `MetricsReporter` interface for custom reporting:

```go
type PrometheusReporter struct {
    // Prometheus metrics
}

func (r *PrometheusReporter) Report(ctx context.Context, metrics sse.Metrics) error {
    // Update Prometheus metrics
    return nil
}

func (r *PrometheusReporter) ReportSummary(ctx context.Context, metrics sse.Metrics) error {
    // Final summary
    return nil
}

func (r *PrometheusReporter) Start(ctx context.Context, health *sse.SSEHealth, interval time.Duration) error {
    // Start periodic reporting
    return nil
}

func (r *PrometheusReporter) Stop() error {
    // Cleanup
    return nil
}
```

## Testing

Run the test suite:

```bash
cd go-cli
go test ./pkg/sse/... -v

# With coverage
go test ./pkg/sse/... -cover

# Benchmarks
go test ./pkg/sse/... -bench=. -benchmem
```

## Performance

- **Memory efficient**: Ring buffer for rate calculation with automatic cleanup
- **Low overhead**: <1% CPU on idle streams, atomic operations for lock-free counters
- **Concurrent safe**: All metrics operations are thread-safe
- **Configurable buffering**: Adjustable event buffer size to handle bursts

## Security

- **Header redaction**: Sensitive headers (authorization, api-key, etc.) are automatically redacted in logs
- **No PII leakage**: Metrics contain only statistical data, no event content
- **Configurable verbosity**: Control what gets logged via log levels

## License

See the main AG-UI repository for license information.