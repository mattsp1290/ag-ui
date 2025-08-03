# HTTP Connection Pool

A high-performance, feature-rich HTTP connection pool manager for Go applications that provides:

## Features

### 1. HTTP Connection Pooling
- **Configurable size limits**: Set maximum connections per server and total connections
- **Connection reuse**: Efficiently reuses existing connections to reduce overhead
- **Idle connection management**: Automatic cleanup of idle connections based on configurable timeouts
- **Memory-efficient design**: Uses object pooling and efficient data structures

### 2. Connection Health Monitoring and Testing
- **Automatic health checks**: Periodically tests server availability with configurable intervals
- **Failure thresholds**: Configurable healthy/unhealthy thresholds for robust failure detection
- **Health status tracking**: Monitors server response times and failure counts
- **Graceful degradation**: Automatically removes unhealthy servers from rotation

### 3. Load Balancing Across Multiple Target Servers
- **Round Robin**: Evenly distributes requests across available servers
- **Least Connections**: Routes to server with fewest active connections
- **Weighted Round Robin**: Distributes based on server weights
- **Random**: Random server selection for load distribution
- **IP Hash**: Consistent routing based on client identifier

### 4. Connection Lifecycle Management
- **Automatic creation**: Creates connections on-demand when needed
- **Connection validation**: Validates connections before reuse
- **Graceful cleanup**: Properly closes and cleans up connections
- **Resource tracking**: Monitors connection usage and lifecycle

### 5. Performance Optimization and Connection Reuse
- **Intelligent pooling**: Keeps warm connections ready for immediate use
- **Connection validation**: Ensures connections are healthy before reuse
- **Configurable timeouts**: Fine-tune connection and request timeouts
- **Efficient semaphore management**: Controls global connection limits

### 6. Comprehensive Metrics Collection
- **Connection metrics**: Total, active, idle, created, destroyed, reused connections
- **Request metrics**: Total, successful, failed requests with response times
- **Health metrics**: Healthy/unhealthy server counts and health check statistics
- **Performance metrics**: Pool utilization, wait times, and throughput
- **Error tracking**: Connection errors, timeouts, and failure analysis
- **Resource monitoring**: Memory usage and resource consumption

### 7. Graceful Shutdown and Resource Cleanup
- **Context-aware shutdown**: Respects context timeouts for graceful termination
- **Connection draining**: Safely closes all active connections
- **Background worker cleanup**: Properly stops all background goroutines
- **Resource release**: Ensures all resources are properly released

### 8. Thread-Safe Operations with Proper Synchronization
- **Concurrent access**: Thread-safe access to all pool operations
- **Lock-free metrics**: Uses atomic operations for performance-critical metrics
- **Deadlock prevention**: Careful lock ordering and timeout handling
- **Race condition prevention**: Proper synchronization for all shared state

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/http"
)

func main() {
    // Create connection pool with default configuration
    pool, err := httppool.NewHTTPConnectionPool(nil)
    if err != nil {
        log.Fatal(err)
    }
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        pool.Shutdown(ctx)
    }()

    // Add backend servers
    pool.AddServer("https://api1.example.com", 1)
    pool.AddServer("https://api2.example.com", 1)

    // Get a connection
    req := &httppool.ConnectionRequest{
        Context: context.Background(),
    }

    resp, err := pool.GetConnection(req)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Connected to: %s\n", resp.Server.URL.String())
    
    // Use the connection (resp.Connection contains HTTP client and transport)
    // ... make HTTP requests ...

    // Release connection back to pool
    pool.ReleaseConnection(resp.Connection)

    // Get metrics
    metrics := pool.GetMetrics()
    fmt.Printf("Pool utilization: %.2f%%\n", metrics.PoolUtilization*100)
}
```

## Advanced Configuration

```go
config := &httppool.HTTPPoolConfig{
    // Connection limits
    MaxConnectionsPerServer: 100,
    MaxTotalConnections:     1000,
    MaxIdleConnections:      50,
    MaxIdleTime:             10 * time.Minute,

    // Timeouts
    ConnectTimeout:    15 * time.Second,
    RequestTimeout:    60 * time.Second,
    KeepAliveTimeout:  30 * time.Second,
    IdleConnTimeout:   90 * time.Second,

    // Health checking
    HealthCheckInterval: 30 * time.Second,
    HealthCheckTimeout:  5 * time.Second,
    HealthCheckPath:     "/health",
    UnhealthyThreshold:  3,
    HealthyThreshold:    2,

    // Load balancing
    LoadBalanceStrategy: httppool.LeastConn,

    // TLS configuration
    TLSConfig: &tls.Config{
        MinVersion: tls.VersionTLS12,
    },
}

pool, err := httppool.NewHTTPConnectionPool(config)
```

## Load Balancing Strategies

### Round Robin
```go
config.LoadBalanceStrategy = httppool.RoundRobin
```
Distributes requests evenly across all healthy servers.

### Least Connections
```go
config.LoadBalanceStrategy = httppool.LeastConn
```
Routes requests to the server with the fewest active connections.

### Weighted Round Robin
```go
config.LoadBalanceStrategy = httppool.WeightedRound
pool.AddServer("https://primary.example.com", 3)   // Higher weight
pool.AddServer("https://secondary.example.com", 1) // Lower weight
```
Distributes requests based on server weights.

### IP Hash
```go
config.LoadBalanceStrategy = httppool.IPHash
req.ClientID = "user-123" // Consistent routing for this client
```
Provides consistent routing based on client identifier.

## Health Monitoring

The connection pool automatically monitors server health:

```go
// Get server health statistics
stats := pool.GetServerStats()
for _, stat := range stats {
    fmt.Printf("Server: %s\n", stat.URL.String())
    fmt.Printf("  Healthy: %t\n", stat.IsHealthy)
    fmt.Printf("  Response Time: %v\n", stat.ResponseTime)
    fmt.Printf("  Failure Count: %d\n", stat.FailureCount)
    fmt.Printf("  Total Requests: %d\n", stat.TotalRequests)
}
```

## Metrics and Monitoring

Access comprehensive metrics for monitoring and tuning:

```go
metrics := pool.GetMetrics()

// Connection metrics
fmt.Printf("Total Connections: %d\n", metrics.TotalConnections)
fmt.Printf("Active Connections: %d\n", metrics.ActiveConnections)
fmt.Printf("Pool Utilization: %.2f%%\n", metrics.PoolUtilization*100)

// Performance metrics
fmt.Printf("Average Response Time: %v\n", metrics.AverageResponseTime)
fmt.Printf("Average Wait Time: %v\n", metrics.AverageWaitTime)

// Health metrics
fmt.Printf("Healthy Servers: %d\n", metrics.HealthyServers)
fmt.Printf("Unhealthy Servers: %d\n", metrics.UnhealthyServers)

// Request metrics
fmt.Printf("Total Requests: %d\n", metrics.TotalRequests)
fmt.Printf("Successful Requests: %d\n", metrics.SuccessfulRequests)
fmt.Printf("Failed Requests: %d\n", metrics.FailedRequests)
```

## Integration Examples

### With HTTP Client
```go
func makeHTTPRequest(pool *httppool.HTTPConnectionPool, path string) error {
    req := &httppool.ConnectionRequest{
        Context: context.Background(),
    }

    resp, err := pool.GetConnection(req)
    if err != nil {
        return err
    }
    defer pool.ReleaseConnection(resp.Connection)

    // Use the pooled connection's HTTP client
    httpReq, err := http.NewRequest("GET", resp.Server.URL.String()+path, nil)
    if err != nil {
        return err
    }

    httpResp, err := resp.Connection.client.Do(httpReq)
    if err != nil {
        return err
    }
    defer httpResp.Body.Close()

    // Process response...
    return nil
}
```

### With Context and Timeouts
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

req := &httppool.ConnectionRequest{
    Context:  ctx,
    ClientID: "user-123", // For IP hash load balancing
    Priority: 1,          // Request priority
}

resp, err := pool.GetConnection(req)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        // Handle timeout
    }
    return err
}
```

## Best Practices

1. **Pool Sizing**: Set `MaxConnectionsPerServer` based on your backend capacity
2. **Health Checks**: Configure appropriate intervals and thresholds for your environment
3. **Timeouts**: Set reasonable timeouts based on your application requirements
4. **Monitoring**: Regularly check metrics to optimize pool configuration
5. **Graceful Shutdown**: Always call `Shutdown()` with appropriate timeout
6. **Error Handling**: Implement proper error handling for connection failures
7. **Load Balancing**: Choose the appropriate strategy for your use case

## Configuration Reference

| Parameter | Default | Description |
|-----------|---------|-------------|
| `MaxConnectionsPerServer` | 100 | Maximum connections per backend server |
| `MaxTotalConnections` | 1000 | Maximum total connections across all servers |
| `MaxIdleConnections` | 50 | Maximum idle connections to keep in pool |
| `MaxIdleTime` | 5m | Maximum time connections can remain idle |
| `ConnectTimeout` | 10s | Timeout for establishing new connections |
| `RequestTimeout` | 30s | Timeout for individual requests |
| `KeepAliveTimeout` | 30s | TCP keep-alive timeout |
| `IdleConnTimeout` | 90s | Timeout for idle connections |
| `HealthCheckInterval` | 30s | Interval between health checks |
| `HealthCheckTimeout` | 5s | Timeout for health check requests |
| `HealthCheckPath` | `/health` | Path for health check requests |
| `UnhealthyThreshold` | 3 | Failures before marking server unhealthy |
| `HealthyThreshold` | 2 | Successes before marking server healthy |
| `LoadBalanceStrategy` | `RoundRobin` | Load balancing algorithm |
| `CleanupInterval` | 1m | Interval for connection cleanup |
| `MetricsInterval` | 10s | Interval for metrics updates |

## Performance Characteristics

- **Memory Efficient**: Uses object pooling and atomic operations
- **High Throughput**: Optimized for concurrent access
- **Low Latency**: Connection reuse minimizes connection overhead
- **Scalable**: Handles thousands of concurrent connections
- **Reliable**: Comprehensive error handling and recovery

The connection pool is designed to be production-ready with comprehensive monitoring, robust error handling, and efficient resource management.