# Distributed Event Validation System

This package implements a distributed validation system for AG-UI events, providing fault-tolerant validation across multiple nodes with consensus-based decision making.

## Features

### Core Components

1. **DistributedValidator** (`distributed_validator.go`)
   - Main orchestrator for distributed validation
   - Manages node registration and health monitoring
   - Coordinates validation across multiple nodes
   - Handles partition tolerance and failover

2. **ConsensusManager** (`consensus.go`)
   - Implements multiple consensus algorithms:
     - Majority voting (default)
     - Unanimous agreement
     - Raft consensus
     - PBFT (Byzantine Fault Tolerance)
   - Manages distributed locks
   - Handles leader election (Raft)

3. **StateSynchronizer** (`state_sync.go`)
   - Synchronizes validation state across nodes
   - Supports multiple sync protocols:
     - Gossip protocol
     - Merkle trees
     - CRDTs (Conflict-free Replicated Data Types)
     - Snapshot-based sync
   - Handles conflict resolution

4. **PartitionHandler** (`partition_handler.go`)
   - Detects network partitions
   - Implements recovery strategies
   - Maintains node health monitoring
   - Supports automatic partition recovery

5. **LoadBalancer** (`load_balancer.go`)
   - Distributes validation load across nodes
   - Supports multiple algorithms:
     - Round-robin
     - Least connections
     - Weighted round-robin
     - Consistent hashing
     - Least response time
   - Circuit breaker functionality

## Usage

### Basic Setup

```go
// Create local validator
localValidator := events.NewEventValidator(nil)

// Configure distributed validator
config := DefaultDistributedValidatorConfig("node-1")
config.ConsensusConfig.Algorithm = ConsensusMajority
config.PartitionHandler.AllowLocalValidation = true

// Create distributed validator
dv, err := NewDistributedValidator(config, localValidator)
if err != nil {
    log.Fatal(err)
}

// Start the validator
ctx := context.Background()
err = dv.Start(ctx)
if err != nil {
    log.Fatal(err)
}
defer dv.Stop()
```

### Node Registration

```go
// Register additional validation nodes
node2 := &NodeInfo{
    ID:              "node-2",
    Address:         "node2:8080",
    State:           NodeStateActive,
    LastHeartbeat:   time.Now(),
    ValidationCount: 0,
    ErrorRate:       0.0,
    ResponseTimeMs:  50,
    Load:            0.3,
}

err := dv.RegisterNode(node2)
if err != nil {
    log.Printf("Failed to register node: %v", err)
}
```

### Event Validation

```go
// Validate a single event
ctx := context.Background()
result := dv.ValidateEvent(ctx, event)

if result.IsValid {
    log.Println("Event validation successful")
} else {
    log.Printf("Event validation failed with %d errors", len(result.Errors))
    for _, err := range result.Errors {
        log.Printf("Error: %s - %s", err.RuleID, err.Message)
    }
}
```

### Sequence Validation

```go
// Validate a sequence of events
events := []events.Event{
    &RunStartedEvent{RunID: "run-1", ThreadID: "thread-1"},
    &TextMessageStartEvent{MessageID: "msg-1"},
    &TextMessageEndEvent{MessageID: "msg-1"},
    &RunFinishedEvent{RunID: "run-1"},
}

result := dv.ValidateSequence(ctx, events)
```

## Configuration

### Consensus Configuration

```go
config.ConsensusConfig = &ConsensusConfig{
    Algorithm:         ConsensusMajority,
    MinNodes:          3,
    QuorumSize:        2,
    RequireUnanimous:  false,
    ElectionTimeout:   5 * time.Second,
    HeartbeatInterval: 1 * time.Second,
}
```

### Partition Handling

```go
config.PartitionHandler = &PartitionHandlerConfig{
    DetectionMethod:      PartitionDetectionHybrid,
    RecoveryStrategy:     PartitionRecoveryMerge,
    HeartbeatTimeout:     10 * time.Second,
    AllowLocalValidation: true,
    AutoRecovery:         true,
}
```

### Load Balancing

```go
config.LoadBalancer = &LoadBalancerConfig{
    Algorithm:               LoadBalancingLeastResponseTime,
    MaxLoadPerNode:          0.8,
    EnableCircuitBreaker:    true,
    CircuitBreakerThreshold: 0.5,
    CircuitBreakerTimeout:   30 * time.Second,
}
```

## Fault Tolerance

### Network Partitions

The system automatically detects network partitions using:
- Heartbeat timeouts
- Quorum loss detection
- Gossip protocol failures

Recovery strategies include:
- Wait for partition to heal
- Merge diverged states
- Reset to known good state
- Manual intervention

### Node Failures

- Automatic failure detection
- Circuit breaker protection
- Load redistribution
- Health monitoring

### Byzantine Fault Tolerance

When using PBFT consensus:
- Tolerates up to f Byzantine failures with 3f+1 nodes
- Requires 2f+1 matching decisions for consensus
- Protects against malicious or corrupted nodes

## Monitoring and Metrics

```go
// Get distributed validation metrics
metrics := dv.GetMetrics()
fmt.Printf("Validation count: %d\n", metrics.GetValidationCount())
fmt.Printf("Error rate: %.2f\n", metrics.GetErrorRate())
fmt.Printf("Average response time: %.2f ms\n", metrics.GetAverageResponseTime())

// Get partition information
if dv.IsPartitioned() {
    partition := dv.GetCurrentPartition()
    fmt.Printf("Partition detected: %s\n", partition.Type)
    fmt.Printf("Affected nodes: %v\n", partition.AffectedNodes)
}

// Get load balancer metrics
lbMetrics := dv.GetLoadBalancer().GetMetrics()
```

## Testing

The package includes comprehensive tests covering:
- Node lifecycle management
- Consensus algorithms
- Partition detection and recovery
- Load balancing strategies
- Concurrent validation
- Circuit breaker functionality

Run tests with:
```bash
go test ./pkg/core/events/distributed/... -v
```

## Performance

Benchmarks are included for:
- Distributed validation throughput
- Consensus algorithm performance
- Load balancing efficiency

Run benchmarks with:
```bash
go test ./pkg/core/events/distributed/... -bench=.
```

## Integration

The distributed validator integrates seamlessly with the existing EventValidator:
- Maintains the same validation interface
- Supports all existing validation rules
- Provides enhanced fault tolerance
- Scales horizontally across multiple nodes

## Troubleshooting

### Common Issues and Solutions

#### Node Discovery and Registration Problems

**Problem**: Nodes unable to find or register with each other
```
Error: failed to register with node-2: connection refused
Warning: no active nodes found in cluster
Error: node discovery timeout after 30s
```

**Diagnostic Commands:**
```bash
# Check network connectivity
ping node-2.example.com
telnet node-2.example.com 8080

# Check cluster status
go run ./cmd/cluster-status/ --node-id=node-1

# Monitor node discovery
go test -v -run TestNodeDiscovery ./pkg/core/events/distributed/
```

**Diagnostic Steps:**
1. Check node configuration:
   ```go
   config := dv.GetConfig()
   log.Printf("Node ID: %s", config.NodeID)
   log.Printf("Bind address: %s", config.BindAddress)
   log.Printf("Discovery method: %s", config.DiscoveryMethod)
   ```

2. Verify network connectivity:
   ```go
   nodes := dv.GetNodes()
   for _, node := range nodes {
       log.Printf("Node %s: address=%s, state=%s, last_heartbeat=%v",
           node.ID, node.Address, node.State, node.LastHeartbeat)
   }
   ```

3. Test transport layer:
   ```go
   transport := dv.GetTransport()
   err := transport.Ping("node-2", 5*time.Second)
   if err != nil {
       log.Printf("Transport ping failed: %v", err)
   }
   ```

**Solutions:**
- Configure proper bind addresses: `config.BindAddress = "0.0.0.0:8080"`
- Use public IP addresses for multi-host deployments
- Configure firewall rules to allow cluster communication
- Implement service discovery (Consul, etcd, Kubernetes services)
- Add retry logic with exponential backoff
- Use health check endpoints for node monitoring

#### Consensus Failures

**Problem**: Distributed validation failing due to consensus issues
```
Error: consensus timeout: only 1 of 3 nodes responded
Error: consensus failed: conflicting validation results
Warning: split-brain scenario detected
```

**Diagnostic Commands:**
```bash
# Test consensus algorithm
go test -v -run TestConsensus ./pkg/core/events/distributed/

# Check consensus metrics
go run ./cmd/consensus-metrics/ --algorithm=majority

# Monitor consensus latency
go test -bench=BenchmarkConsensus ./pkg/core/events/distributed/
```

**Diagnostic Steps:**
1. Check consensus configuration:
   ```go
   consensus := dv.GetConsensusManager()
   config := consensus.GetConfig()
   log.Printf("Algorithm: %s", config.Algorithm)
   log.Printf("Min nodes: %d", config.MinNodes)
   log.Printf("Quorum size: %d", config.QuorumSize)
   log.Printf("Timeout: %v", config.Timeout)
   ```

2. Monitor consensus operations:
   ```go
   metrics := consensus.GetMetrics()
   log.Printf("Consensus operations: %d", metrics.TotalOperations)
   log.Printf("Success rate: %.2f%%", metrics.SuccessRate)
   log.Printf("Average latency: %v", metrics.AvgLatency)
   log.Printf("Current leader: %s", metrics.CurrentLeader)
   ```

3. Check node health:
   ```go
   activeNodes := dv.GetActiveNodes()
   log.Printf("Active nodes: %d/%d", len(activeNodes), dv.GetTotalNodes())
   
   for _, node := range activeNodes {
       health := dv.GetNodeHealth(node.ID)
       log.Printf("Node %s: health=%.2f, response_time=%v",
           node.ID, health.Score, health.ResponseTime)
   }
   ```

**Solutions:**
- Ensure minimum node count: at least 3 nodes for majority consensus
- Use odd number of nodes to avoid ties
- Adjust consensus timeouts: `config.ConsensusTimeout = 10 * time.Second`
- Implement proper leader election for Raft consensus
- Use PBFT for Byzantine fault tolerance scenarios
- Monitor and handle network partitions gracefully

#### Network Partition Recovery

**Problem**: Cluster split due to network partitions
```
Error: network partition detected: lost contact with 2 nodes
Warning: operating in minority partition
Error: partition recovery failed: state synchronization error
```

**Diagnostic Steps:**
1. Detect partition state:
   ```go
   partitionHandler := dv.GetPartitionHandler()
   if partitionHandler.IsPartitioned() {
       partition := partitionHandler.GetCurrentPartition()
       log.Printf("Partition type: %s", partition.Type)
       log.Printf("Affected nodes: %v", partition.AffectedNodes)
       log.Printf("Recovery strategy: %s", partition.RecoveryStrategy)
   }
   ```

2. Check partition recovery progress:
   ```go
   recovery := partitionHandler.GetRecoveryStatus()
   log.Printf("Recovery in progress: %t", recovery.InProgress)
   log.Printf("Recovery strategy: %s", recovery.Strategy)
   log.Printf("Progress: %.2f%%", recovery.Progress)
   ```

**Solutions:**
- Configure partition detection: `config.PartitionDetection.HeartbeatTimeout = 30 * time.Second`
- Implement auto-recovery: `config.PartitionRecovery.AutoRecovery = true`
- Use appropriate recovery strategy:
  - `PartitionRecoveryWait`: Wait for partition to heal
  - `PartitionRecoveryMerge`: Merge states when reconnected
  - `PartitionRecoveryReset`: Reset to known good state
- Enable local validation during partitions: `config.AllowLocalValidation = true`
- Implement conflict resolution for diverged states

#### Load Balancing Issues

**Problem**: Uneven load distribution across nodes
```
Warning: node-1 load at 95%, node-2 load at 15%
Error: circuit breaker opened for node-3: too many failures
Warning: load balancer fell back to single node
```

**Diagnostic Commands:**
```bash
# Monitor load distribution
go test -bench=BenchmarkLoadBalancing ./pkg/core/events/distributed/

# Check circuit breaker status
go run ./cmd/circuit-breaker-status/
```

**Diagnostic Steps:**
1. Check load balancer metrics:
   ```go
   lb := dv.GetLoadBalancer()
   metrics := lb.GetMetrics()
   log.Printf("Algorithm: %s", metrics.Algorithm)
   log.Printf("Total requests: %d", metrics.TotalRequests)
   log.Printf("Failed requests: %d", metrics.FailedRequests)
   
   for nodeID, nodeMetrics := range metrics.NodeMetrics {
       log.Printf("Node %s: load=%.2f, response_time=%v, error_rate=%.2f",
           nodeID, nodeMetrics.Load, nodeMetrics.AvgResponseTime, nodeMetrics.ErrorRate)
   }
   ```

2. Check circuit breaker status:
   ```go
   for _, node := range dv.GetNodes() {
       breaker := lb.GetCircuitBreaker(node.ID)
       if breaker != nil {
           state := breaker.GetState()
           log.Printf("Node %s circuit breaker: state=%s, failures=%d",
               node.ID, state.State, state.ConsecutiveFailures)
       }
   }
   ```

**Solutions:**
- Use appropriate load balancing algorithm:
  - `LoadBalancingRoundRobin`: Equal distribution
  - `LoadBalancingLeastConnections`: Balance by active connections
  - `LoadBalancingLeastResponseTime`: Route to fastest nodes
  - `LoadBalancingWeightedRoundRobin`: Weighted distribution
- Configure circuit breaker thresholds: `config.CircuitBreakerThreshold = 0.5`
- Implement health-based routing
- Add node capacity limits: `config.MaxLoadPerNode = 0.8`
- Monitor and adjust weights dynamically

#### State Synchronization Problems

**Problem**: Nodes have inconsistent validation state
```
Error: state synchronization failed: merkle tree mismatch
Warning: node-2 state is 30s behind leader
Error: CRDT merge conflict detected
```

**Diagnostic Steps:**
1. Check state synchronization status:
   ```go
   stateSyncer := dv.GetStateSynchronizer()
   status := stateSyncer.GetSyncStatus()
   log.Printf("Sync protocol: %s", status.Protocol)
   log.Printf("Last sync: %v", status.LastSync)
   log.Printf("Sync errors: %d", status.ErrorCount)
   
   for nodeID, nodeStatus := range status.NodeStatus {
       log.Printf("Node %s: state_version=%d, lag=%v",
           nodeID, nodeStatus.StateVersion, nodeStatus.Lag)
   }
   ```

2. Verify state consistency:
   ```go
   localState := dv.GetLocalState()
   for _, node := range dv.GetNodes() {
       remoteState, err := dv.GetRemoteState(node.ID)
       if err != nil {
           log.Printf("Failed to get state from %s: %v", node.ID, err)
           continue
       }
       
       if !stateSyncer.CompareStates(localState, remoteState) {
           log.Printf("State mismatch with node %s", node.ID)
       }
   }
   ```

**Solutions:**
- Choose appropriate sync protocol:
  - `SyncProtocolGossip`: Good for large clusters
  - `SyncProtocolMerkle`: Efficient for detecting differences
  - `SyncProtocolCRDT`: Conflict-free for concurrent updates
  - `SyncProtocolSnapshot`: Simple full-state synchronization
- Adjust sync frequency: `config.SyncInterval = 30 * time.Second`
- Implement conflict resolution strategies
- Use vector clocks for ordering events
- Enable state compression for large states

### Performance Issues

#### High Validation Latency

**Problem**: Distributed validation taking too long
```
Warning: validation latency 2.5s exceeds threshold
Error: validation timeout after 10s
```

**Diagnostic Commands:**
```bash
# Profile validation performance
go test -cpuprofile=distributed.prof -bench=BenchmarkValidation ./pkg/core/events/distributed/
go tool pprof distributed.prof

# Measure network latency
ping -c 10 node-2.example.com
```

**Performance Analysis:**
```go
func analyzeValidationLatency(dv *DistributedValidator) {
    event := createTestEvent()
    
    // Measure consensus latency
    start := time.Now()
    result := dv.ValidateEvent(context.Background(), event)
    totalLatency := time.Since(start)
    
    // Break down latency components
    metrics := dv.GetMetrics()
    log.Printf("Performance breakdown:")
    log.Printf("  Total latency: %v", totalLatency)
    log.Printf("  Network latency: %v", metrics.AvgNetworkLatency)
    log.Printf("  Consensus latency: %v", metrics.AvgConsensusLatency)
    log.Printf("  Local validation: %v", metrics.AvgLocalValidationLatency)
    
    // Identify bottlenecks
    if metrics.AvgNetworkLatency > 100*time.Millisecond {
        log.Printf("WARNING: High network latency detected")
    }
    if metrics.AvgConsensusLatency > 500*time.Millisecond {
        log.Printf("WARNING: Slow consensus algorithm")
    }
}
```

**Solutions:**
- Optimize network configuration (increase bandwidth, reduce latency)
- Use faster consensus algorithms for small clusters
- Implement validation result caching
- Use regional node deployment to reduce geographical latency
- Configure appropriate timeouts based on network characteristics

#### Memory Leaks in Distributed Operations

**Problem**: Memory usage growing over time
```
Warning: memory usage increased 200MB in last hour
Error: out of memory: cannot allocate goroutine stack
```

**Memory Profiling:**
```bash
# Profile memory usage
go test -memprofile=distributed_mem.prof -bench=BenchmarkDistributedMemory ./pkg/core/events/distributed/
go tool pprof distributed_mem.prof

# Monitor goroutine leaks
go test -race -count=10 ./pkg/core/events/distributed/
```

**Diagnostic Code:**
```go
func monitorMemoryUsage(dv *DistributedValidator) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        
        log.Printf("Memory usage:")
        log.Printf("  Allocated: %d MB", m.Alloc/1024/1024)
        log.Printf("  Total allocated: %d MB", m.TotalAlloc/1024/1024)
        log.Printf("  Goroutines: %d", runtime.NumGoroutine())
        
        // Check for excessive memory growth
        if m.Alloc > 500*1024*1024 { // 500MB
            log.Printf("WARNING: High memory usage detected")
            
            // Trigger garbage collection
            runtime.GC()
            
            // Check for goroutine leaks
            if runtime.NumGoroutine() > 1000 {
                log.Printf("WARNING: Potential goroutine leak")
            }
        }
    }
}
```

**Solutions:**
- Implement proper context cancellation for all operations
- Use connection pooling instead of creating new connections
- Clean up resources in defer statements
- Implement bounded queues for async operations
- Monitor and limit goroutine creation

### Debugging and Monitoring

#### Enable Debug Logging

```go
config := &DistributedValidatorConfig{
    DebugMode: true,
    LogLevel:  "DEBUG",
    Monitoring: &MonitoringConfig{
        Enabled:           true,
        MetricsInterval:   30 * time.Second,
        HealthCheckURL:    "/health",
        PrometheusEnabled: true,
    },
}
```

#### Comprehensive Health Check

```go
func performHealthCheck(dv *DistributedValidator) {
    health := dv.GetHealth()
    
    log.Printf("Distributed Validator Health Report:")
    log.Printf("  Overall health: %t", health.Healthy)
    log.Printf("  Active nodes: %d/%d", health.ActiveNodes, health.TotalNodes)
    log.Printf("  Consensus health: %t", health.ConsensusHealthy)
    log.Printf("  Network health: %t", health.NetworkHealthy)
    
    // Check individual components
    if !health.ConsensusHealthy {
        consensus := dv.GetConsensusManager()
        consensusHealth := consensus.GetHealth()
        log.Printf("  Consensus issues: %v", consensusHealth.Issues)
    }
    
    if !health.NetworkHealthy {
        transport := dv.GetTransport()
        transportHealth := transport.GetHealth()
        log.Printf("  Network issues: %v", transportHealth.Issues)
    }
    
    // Check node-specific health
    for _, node := range dv.GetNodes() {
        nodeHealth := dv.GetNodeHealth(node.ID)
        if !nodeHealth.Healthy {
            log.Printf("  Node %s issues: %v", node.ID, nodeHealth.Issues)
        }
    }
}
```

#### Performance Metrics Collection

```go
func collectPerformanceMetrics(dv *DistributedValidator) {
    metrics := dv.GetMetrics()
    
    // Validation metrics
    log.Printf("Validation Metrics:")
    log.Printf("  Total validations: %d", metrics.TotalValidations)
    log.Printf("  Success rate: %.2f%%", metrics.SuccessRate)
    log.Printf("  Average latency: %v", metrics.AvgLatency)
    log.Printf("  Throughput: %.0f validations/second", metrics.Throughput)
    
    // Network metrics
    log.Printf("Network Metrics:")
    log.Printf("  Messages sent: %d", metrics.MessagesSent)
    log.Printf("  Messages received: %d", metrics.MessagesReceived)
    log.Printf("  Network errors: %d", metrics.NetworkErrors)
    log.Printf("  Average network latency: %v", metrics.AvgNetworkLatency)
    
    // Consensus metrics
    log.Printf("Consensus Metrics:")
    log.Printf("  Consensus operations: %d", metrics.ConsensusOperations)
    log.Printf("  Consensus failures: %d", metrics.ConsensusFailures)
    log.Printf("  Average consensus time: %v", metrics.AvgConsensusTime)
    
    // Resource metrics
    log.Printf("Resource Metrics:")
    log.Printf("  CPU usage: %.2f%%", metrics.CPUUsage)
    log.Printf("  Memory usage: %d MB", metrics.MemoryUsage/1024/1024)
    log.Printf("  Goroutines: %d", metrics.GoroutineCount)
}
```

#### Network Diagnostics

```go
func diagnoseNetworkIssues(dv *DistributedValidator) {
    transport := dv.GetTransport()
    
    // Test connectivity to all nodes
    for _, node := range dv.GetNodes() {
        start := time.Now()
        err := transport.Ping(node.ID, 5*time.Second)
        latency := time.Since(start)
        
        if err != nil {
            log.Printf("Node %s: UNREACHABLE (%v)", node.ID, err)
        } else {
            log.Printf("Node %s: reachable (latency: %v)", node.ID, latency)
        }
    }
    
    // Check transport statistics
    stats := transport.GetStats()
    log.Printf("Transport Statistics:")
    log.Printf("  Connections: active=%d, total=%d", stats.ActiveConnections, stats.TotalConnections)
    log.Printf("  Bytes sent: %d", stats.BytesSent)
    log.Printf("  Bytes received: %d", stats.BytesReceived)
    log.Printf("  Connection errors: %d", stats.ConnectionErrors)
}
```

### Configuration Recommendations

#### Small Cluster (3-5 nodes)

```go
config := &DistributedValidatorConfig{
    ConsensusConfig: &ConsensusConfig{
        Algorithm:         ConsensusMajority,
        MinNodes:          3,
        QuorumSize:        2,
        Timeout:           5 * time.Second,
    },
    LoadBalancer: &LoadBalancerConfig{
        Algorithm:               LoadBalancingRoundRobin,
        EnableCircuitBreaker:    true,
        CircuitBreakerThreshold: 0.5,
    },
    PartitionHandler: &PartitionHandlerConfig{
        AllowLocalValidation: true,
        RecoveryStrategy:     PartitionRecoveryMerge,
    },
}
```

#### Large Cluster (10+ nodes)

```go
config := &DistributedValidatorConfig{
    ConsensusConfig: &ConsensusConfig{
        Algorithm:         ConsensusRaft,
        MinNodes:          5,
        QuorumSize:        3,
        Timeout:           10 * time.Second,
        HeartbeatInterval: 1 * time.Second,
    },
    LoadBalancer: &LoadBalancerConfig{
        Algorithm:               LoadBalancingLeastResponseTime,
        EnableCircuitBreaker:    true,
        CircuitBreakerThreshold: 0.3,
    },
    StateSynchronizer: &StateSyncConfig{
        Protocol:     SyncProtocolGossip,
        SyncInterval: 30 * time.Second,
    },
}
```

#### High-Security Environment (Byzantine Fault Tolerance)

```go
config := &DistributedValidatorConfig{
    ConsensusConfig: &ConsensusConfig{
        Algorithm:         ConsensusPBFT,
        MinNodes:          4, // 3f+1 for f=1 Byzantine failures
        QuorumSize:        3, // 2f+1
        Timeout:           15 * time.Second,
    },
    Security: &SecurityConfig{
        EnableEncryption:    true,
        EnableAuthentication: true,
        SignMessages:        true,
    },
}
```

## Future Enhancements

- Network transport layer implementation
- Dynamic node discovery
- Advanced monitoring and alerting
- Performance optimizations
- Cross-datacenter support