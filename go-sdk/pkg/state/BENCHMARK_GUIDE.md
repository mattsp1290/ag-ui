# State Management Performance Benchmarks Guide

## Overview

This guide describes the comprehensive performance benchmarks and regression tests for the AG-UI Go SDK state management system. These benchmarks are designed to measure performance across all critical operations and detect performance regressions.

## Benchmark Categories

### 1. State Update Benchmarks (`BenchmarkStateUpdate`)

Tests single update performance with different data sizes:
- **Small**: 100 items
- **Medium**: 1,000 items  
- **Large**: 10,000 items

**Usage:**
```bash
go test -bench=BenchmarkStateUpdate -benchmem ./pkg/state
```

**Expected Performance:**
- Small: < 50µs per operation
- Medium: < 500µs per operation
- Large: < 5ms per operation

### 2. Batch Update Benchmarks (`BenchmarkBatchUpdate`)

Tests batch update performance using JSON Patch operations:
- Batch sizes: 10, 100, 1000 operations

**Usage:**
```bash
go test -bench=BenchmarkBatchUpdate -benchmem ./pkg/state
```

### 3. State Read Benchmarks (`BenchmarkStateRead`)

Tests read performance under different scenarios:
- Full state reads (different sizes)
- Partial path reads
- Deep path navigation

**Usage:**
```bash
go test -bench=BenchmarkStateRead -benchmem ./pkg/state
```

### 4. Concurrency Benchmarks

#### Read Concurrency (`BenchmarkConcurrentReads`)
Tests parallel read performance with different concurrency levels:
- 10, 100, 1000 concurrent readers

**Usage:**
```bash
go test -bench=BenchmarkConcurrentReads -benchmem ./pkg/state
```

#### Write Concurrency (`BenchmarkConcurrentWrites`) 
Tests parallel write performance:
- 10, 50, 100 concurrent writers

**Usage:**
```bash
go test -bench=BenchmarkConcurrentWrites -benchmem ./pkg/state
```

### 5. Delta Computation Benchmarks (`BenchmarkDeltaComputationPerf`)

Tests delta algorithm performance with different change ratios:
- Small/Medium/Large datasets
- 10%, 50%, 90% change ratios

**Usage:**
```bash
go test -bench=BenchmarkDeltaComputationPerf -benchmem ./pkg/state
```

**Expected Performance:**
- Small 10%: < 1ms per operation
- Large 10%: < 100ms per operation

### 6. JSON Patch Benchmarks (`BenchmarkJSONPatch`)

Tests patch application performance:
- 10, 100, 1000 patch operations

**Usage:**
```bash
go test -bench=BenchmarkJSONPatch -benchmem ./pkg/state
```

### 7. Subscription Benchmarks (`BenchmarkSubscriptions`)

Tests event delivery performance:
- Small: 10 subscribers
- Medium: 100 subscribers  
- Large: 1000 subscribers

**Usage:**
```bash
go test -bench=BenchmarkSubscriptions -benchmem ./pkg/state
```

### 8. Memory Usage Benchmarks (`BenchmarkMemoryUsage`)

Tracks memory allocation patterns:
- Single updates
- Batch updates (10 operations)
- Mixed read/write workloads

**Usage:**
```bash
go test -bench=BenchmarkMemoryUsage -benchmem ./pkg/state
```

**Key Metrics:**
- `allocs/op`: Memory allocations per operation
- `bytes/op`: Bytes allocated per operation

### 9. Worst Case Benchmarks (`BenchmarkWorstCase`)

Tests performance under challenging scenarios:
- **DeepNesting**: 10 levels deep, 5 branches each
- **LargeArrayModification**: 10,000 element array updates
- **RapidStateChurn**: Frequent complete state replacements

**Usage:**
```bash
go test -bench=BenchmarkWorstCase -benchmem ./pkg/state
```

### 10. Copy-on-Write Efficiency (`BenchmarkCOWEfficiency`)

Tests COW performance under different workloads:
- **ReadHeavyWorkload**: 95% reads, 5% writes
- **WriteHeavyWorkload**: 20% reads, 80% writes

**Usage:**
```bash
go test -bench=BenchmarkCOWEfficiency -benchmem ./pkg/state
```

### 11. Transaction Performance (`BenchmarkTransactionPerformance`)

Tests transaction system performance:
- Simple transactions (single operation)
- Batch transactions (10 operations)

**Usage:**
```bash
go test -bench=BenchmarkTransactionPerformance -benchmem ./pkg/state
```

### 12. Regression Detection (`BenchmarkRegressionDetection`)

Establishes baseline performance metrics for regression detection:
- **Baseline_Update**: Standard update operations
- **Baseline_Read**: Standard read operations  
- **Baseline_Delta**: Delta computation baseline

**Usage:**
```bash
go test -bench=BenchmarkRegressionDetection -benchmem -benchtime=10s ./pkg/state
```

## Running All Benchmarks

### Complete Suite
```bash
go test -run=^$ -bench=Benchmark -benchmem -benchtime=1s ./pkg/state
```

### With CPU Profiling
```bash
go test -bench=BenchmarkStateUpdate -benchmem -cpuprofile=cpu.prof ./pkg/state
```

### With Memory Profiling
```bash
go test -bench=BenchmarkMemoryUsage -benchmem -memprofile=mem.prof ./pkg/state
```

### Regression Testing
```bash
# Run baseline benchmarks
go test -bench=BenchmarkRegressionDetection -benchmem -benchtime=10s ./pkg/state > baseline.txt

# After changes, compare
go test -bench=BenchmarkRegressionDetection -benchmem -benchtime=10s ./pkg/state > current.txt
benchcmp baseline.txt current.txt
```

## Performance Targets

### Update Operations
- Single update: < 50µs
- Batch update (10): < 500µs  
- Batch update (100): < 5ms

### Read Operations
- Single read: < 10µs
- Full state read (1K items): < 1ms
- Concurrent reads: < 1µs per operation

### Memory Efficiency
- Single update: < 10 allocs/op
- Read operations: < 5 allocs/op
- COW efficiency: Minimal allocations for reads

### Delta Computation
- Small changes (10%): < 1ms
- Medium changes (50%): < 10ms
- Large changes (90%): < 100ms

### Subscription Performance  
- Event delivery: < 1ms per subscriber
- 1000 subscribers: < 1s total delivery time

## Interpreting Results

### Key Metrics
- **ns/op**: Nanoseconds per operation (lower is better)
- **B/op**: Bytes allocated per operation (lower is better)  
- **allocs/op**: Number of allocations per operation (lower is better)

### Performance Analysis
1. **Latency**: Look at ns/op for operation speed
2. **Memory Efficiency**: Monitor B/op and allocs/op trends
3. **Scalability**: Compare performance across different data sizes
4. **Concurrency**: Ensure parallel performance doesn't degrade significantly

### Regression Detection
- **>20% increase** in ns/op: Performance regression
- **>50% increase** in B/op: Memory regression  
- **>100% increase** in allocs/op: Allocation regression

## Troubleshooting

### Common Issues
1. **High Memory Usage**: Check for memory leaks in COW implementation
2. **Poor Concurrency**: Investigate lock contention in parallel benchmarks
3. **Slow Delta Computation**: Analyze algorithm efficiency for large datasets
4. **Subscription Bottlenecks**: Monitor event delivery performance

### Debugging Performance
```bash
# Generate CPU profile
go test -bench=BenchmarkWorstCase -benchmem -cpuprofile=cpu.prof ./pkg/state
go tool pprof cpu.prof

# Generate memory profile  
go test -bench=BenchmarkMemoryUsage -benchmem -memprofile=mem.prof ./pkg/state
go tool pprof mem.prof

# Trace execution
go test -bench=BenchmarkConcurrentWrites -trace=trace.out ./pkg/state
go tool trace trace.out
```

## Benchmark Configuration

### Environment Setup
- Run on consistent hardware (Apple M4 Max recommended)
- Close other applications to minimize interference
- Use `-benchtime=10s` for stable results
- Run multiple times and average results

### CI/CD Integration
```yaml
# Example GitHub Actions workflow
- name: Run Performance Benchmarks  
  run: |
    go test -bench=BenchmarkRegressionDetection -benchmem -benchtime=5s ./pkg/state > bench.txt
    # Compare with baseline and fail if regression > 20%
```

## Best Practices

### When to Run Benchmarks
- Before major releases
- After performance-related changes
- During optimization work
- As part of CI/CD pipeline

### Benchmark Maintenance
- Update baselines after intentional performance changes
- Add new benchmarks for new features
- Remove obsolete benchmarks
- Keep benchmark data sizes realistic

### Performance Optimization
1. Use benchmark results to identify bottlenecks
2. Focus on hot paths identified by profiling
3. Optimize memory allocations first
4. Consider algorithmic improvements for large datasets
5. Test concurrency assumptions under load