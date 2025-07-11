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
err = dv.Start()
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

## Future Enhancements

- Network transport layer implementation
- Dynamic node discovery
- Advanced monitoring and alerting
- Performance optimizations
- Cross-datacenter support