# Performance Debugging and Diagnostic Guide

This guide provides comprehensive techniques and tools for debugging performance issues and conducting diagnostics in the AG-UI Go SDK enhanced event validation system.

## Table of Contents

1. [Performance Monitoring Overview](#performance-monitoring-overview)
2. [Diagnostic Commands and Tools](#diagnostic-commands-and-tools)
3. [CPU Performance Analysis](#cpu-performance-analysis)
4. [Memory Analysis and Leak Detection](#memory-analysis-and-leak-detection)
5. [Concurrency and Race Condition Detection](#concurrency-and-race-condition-detection)
6. [Network and Distributed Performance](#network-and-distributed-performance)
7. [Cache Performance Optimization](#cache-performance-optimization)
8. [Benchmarking and Load Testing](#benchmarking-and-load-testing)
9. [Profiling Techniques](#profiling-techniques)
10. [Real-time Monitoring](#real-time-monitoring)

## Performance Monitoring Overview

### Key Performance Metrics

The AG-UI Go SDK tracks several critical performance metrics:

| Metric | Description | Target Range | Critical Threshold |
|--------|-------------|--------------|-------------------|
| Validation Latency | Time to validate single event | < 100μs | > 1ms |
| Throughput | Events validated per second | > 50K/sec | < 1K/sec |
| Memory Usage | Total memory consumption | < 500MB | > 1GB |
| CPU Usage | CPU utilization percentage | < 70% | > 90% |
| Cache Hit Rate | Percentage of cache hits | > 80% | < 50% |
| Goroutines | Number of active goroutines | < 1000 | > 5000 |
| GC Pause | Garbage collection pause time | < 10ms | > 100ms |

### Performance Categories

**Excellent Performance:**
- Validation latency: < 50μs
- Throughput: > 100K events/second
- Memory usage: < 100MB
- Cache hit rate: > 95%

**Good Performance:**
- Validation latency: 50-100μs
- Throughput: 50-100K events/second
- Memory usage: 100-300MB
- Cache hit rate: 85-95%

**Poor Performance:**
- Validation latency: > 1ms
- Throughput: < 10K events/second
- Memory usage: > 500MB
- Cache hit rate: < 80%

## Diagnostic Commands and Tools

### Basic Diagnostic Commands

```bash
# Run comprehensive performance benchmarks
go test -bench=. -benchmem ./pkg/core/events/...

# Test with race detection
go test -race ./pkg/core/events/...

# Memory profiling
go test -memprofile=mem.prof -bench=. ./pkg/core/events/
go tool pprof mem.prof

# CPU profiling
go test -cpuprofile=cpu.prof -bench=. ./pkg/core/events/
go tool pprof cpu.prof

# Block profiling (for goroutine blocking)
go test -blockprofile=block.prof -bench=. ./pkg/core/events/
go tool pprof block.prof

# Mutex profiling (for lock contention)
go test -mutexprofile=mutex.prof -bench=. ./pkg/core/events/
go tool pprof mutex.prof

# Trace analysis
go test -trace=trace.out -bench=. ./pkg/core/events/
go tool trace trace.out

# Test coverage with performance impact
go test -cover -coverprofile=coverage.out ./pkg/core/events/
go tool cover -html=coverage.out -o coverage.html
```

### Advanced Diagnostic Scripts

```bash
#!/bin/bash
# performance_analysis.sh - Comprehensive performance analysis script

echo "Starting AG-UI Go SDK Performance Analysis..."

# Create output directory
mkdir -p performance_reports/$(date +%Y%m%d_%H%M%S)
cd performance_reports/$(date +%Y%m%d_%H%M%S)

# Run CPU profiling
echo "Running CPU profiling..."
go test -cpuprofile=cpu.prof -bench=BenchmarkValidation -benchtime=30s ../../pkg/core/events/
go tool pprof -pdf ../../pkg/core/events/ cpu.prof > cpu_profile.pdf

# Run memory profiling
echo "Running memory profiling..."
go test -memprofile=mem.prof -bench=BenchmarkValidation -benchtime=30s ../../pkg/core/events/
go tool pprof -pdf ../../pkg/core/events/ mem.prof > memory_profile.pdf

# Run goroutine analysis
echo "Running goroutine analysis..."
go test -bench=BenchmarkConcurrentValidation -benchtime=10s ../../pkg/core/events/ > goroutine_analysis.txt

# Run trace analysis
echo "Running trace analysis..."
go test -trace=trace.out -bench=BenchmarkValidation -benchtime=10s ../../pkg/core/events/

# Generate performance report
echo "Generating performance report..."
cat > performance_report.md << EOF
# Performance Analysis Report - $(date)

## CPU Profile
See cpu_profile.pdf for detailed CPU usage analysis.

## Memory Profile  
See memory_profile.pdf for detailed memory usage analysis.

## Goroutine Analysis
See goroutine_analysis.txt for concurrent validation performance.

## Trace Analysis
Use: go tool trace trace.out

## Recommendations
- Review CPU hotspots in cpu_profile.pdf
- Check for memory leaks in memory_profile.pdf
- Verify goroutine scaling in goroutine_analysis.txt
EOF

echo "Performance analysis complete. Reports saved in performance_reports/"
```

## CPU Performance Analysis

### CPU Profiling with pprof

```go
// cpu_profiling.go - CPU profiling utilities
package main

import (
    _ "net/http/pprof"
    "context"
    "log"
    "net/http"
    "os"
    "runtime/pprof"
    "time"
    
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

func enableCPUProfiling() {
    // Enable HTTP pprof endpoint
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
    
    // File-based CPU profiling
    f, err := os.Create("cpu.prof")
    if err != nil {
        log.Fatal(err)
    }
    defer f.Close()
    
    if err := pprof.StartCPUProfile(f); err != nil {
        log.Fatal(err)
    }
    defer pprof.StopCPUProfile()
    
    // Run performance test
    runValidationTest()
}

func runValidationTest() {
    validator := events.NewEventValidator(events.DefaultValidationConfig())
    
    // CPU-intensive validation test
    for i := 0; i < 100000; i++ {
        event := &events.RunStartedEvent{
            BaseEvent: &events.BaseEvent{
                EventType:   events.EventTypeRunStarted,
                TimestampMs: timePtr(time.Now().UnixMilli()),
            },
            RunIDValue:    fmt.Sprintf("run-%d", i),
            ThreadIDValue: "thread-1",
        }
        
        result := validator.ValidateEvent(context.Background(), event)
        if !result.IsValid {
            log.Printf("Validation failed for event %d", i)
        }
    }
}
```

### CPU Hotspot Analysis

```go
// cpu_analysis.go - CPU performance analysis tools
package main

import (
    "context"
    "fmt"
    "runtime"
    "time"
    
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

func analyzeCPUHotspots() {
    validator := events.NewEventValidator(events.DefaultValidationConfig())
    
    // Measure CPU time for different operations
    measurements := map[string]time.Duration{
        "event_creation":    measureEventCreation(),
        "basic_validation":  measureBasicValidation(validator),
        "rule_evaluation":   measureRuleEvaluation(validator),
        "result_processing": measureResultProcessing(validator),
    }
    
    fmt.Println("CPU Performance Breakdown:")
    total := time.Duration(0)
    for operation, duration := range measurements {
        total += duration
        fmt.Printf("  %s: %v\n", operation, duration)
    }
    
    fmt.Printf("Total: %v\n", total)
    
    // Calculate percentages
    fmt.Println("\nPercentage Breakdown:")
    for operation, duration := range measurements {
        percentage := float64(duration) / float64(total) * 100
        fmt.Printf("  %s: %.1f%%\n", operation, percentage)
        
        if percentage > 40 {
            fmt.Printf("    WARNING: %s is a CPU hotspot!\n", operation)
        }
    }
}

func measureEventCreation() time.Duration {
    start := time.Now()
    for i := 0; i < 10000; i++ {
        _ = &events.RunStartedEvent{
            BaseEvent: &events.BaseEvent{
                EventType:   events.EventTypeRunStarted,
                TimestampMs: timePtr(time.Now().UnixMilli()),
            },
            RunIDValue:    fmt.Sprintf("run-%d", i),
            ThreadIDValue: "thread-1",
        }
    }
    return time.Since(start)
}

func measureBasicValidation(validator *events.EventValidator) time.Duration {
    event := createTestEvent()
    
    start := time.Now()
    for i := 0; i < 10000; i++ {
        validator.ValidateEvent(context.Background(), event)
    }
    return time.Since(start)
}

func measureRuleEvaluation(validator *events.EventValidator) time.Duration {
    // Add multiple rules to measure rule evaluation overhead
    validator.AddValidationRule(&events.TimingConstraintRule{})
    validator.AddValidationRule(&events.ContentValidationRule{})
    
    event := createTestEvent()
    
    start := time.Now()
    for i := 0; i < 10000; i++ {
        validator.ValidateEvent(context.Background(), event)
    }
    return time.Since(start)
}

func measureResultProcessing(validator *events.EventValidator) time.Duration {
    event := createTestEvent()
    
    start := time.Now()
    for i := 0; i < 10000; i++ {
        result := validator.ValidateEvent(context.Background(), event)
        // Process result (simulate actual usage)
        _ = result.IsValid
        _ = len(result.Errors)
        _ = result.ProcessingTime
    }
    return time.Since(start)
}
```

### CPU Usage Monitoring

```go
// cpu_monitor.go - Real-time CPU monitoring
package main

import (
    "fmt"
    "runtime"
    "time"
)

type CPUMonitor struct {
    interval time.Duration
    samples  []CPUSample
}

type CPUSample struct {
    Timestamp time.Time
    NumCPU    int
    Goroutines int
    GCCycles  uint32
}

func NewCPUMonitor(interval time.Duration) *CPUMonitor {
    return &CPUMonitor{
        interval: interval,
        samples:  make([]CPUSample, 0),
    }
}

func (m *CPUMonitor) Start() {
    ticker := time.NewTicker(m.interval)
    defer ticker.Stop()
    
    var lastGC uint32
    
    for range ticker.C {
        var stats runtime.MemStats
        runtime.ReadMemStats(&stats)
        
        sample := CPUSample{
            Timestamp:  time.Now(),
            NumCPU:     runtime.NumCPU(),
            Goroutines: runtime.NumGoroutine(),
            GCCycles:   uint32(stats.NumGC),
        }
        
        m.samples = append(m.samples, sample)
        
        // Log if significant changes
        if len(m.samples) > 1 {
            prev := m.samples[len(m.samples)-2]
            
            if sample.Goroutines > prev.Goroutines*2 {
                fmt.Printf("WARNING: Goroutine count doubled: %d -> %d\n", 
                    prev.Goroutines, sample.Goroutines)
            }
            
            if sample.GCCycles > lastGC {
                gcFreq := float64(sample.GCCycles-lastGC) / m.interval.Seconds()
                if gcFreq > 1.0 { // More than 1 GC per second
                    fmt.Printf("WARNING: High GC frequency: %.1f cycles/second\n", gcFreq)
                }
            }
            lastGC = sample.GCCycles
        }
        
        // Keep only last 1000 samples
        if len(m.samples) > 1000 {
            m.samples = m.samples[len(m.samples)-1000:]
        }
    }
}

func (m *CPUMonitor) GetReport() string {
    if len(m.samples) == 0 {
        return "No CPU samples collected"
    }
    
    latest := m.samples[len(m.samples)-1]
    
    return fmt.Sprintf(`CPU Monitoring Report:
  Timestamp: %v
  CPUs: %d
  Goroutines: %d
  GC Cycles: %d
  Sample Count: %d
  Monitoring Duration: %v`,
        latest.Timestamp,
        latest.NumCPU,
        latest.Goroutines,
        latest.GCCycles,
        len(m.samples),
        latest.Timestamp.Sub(m.samples[0].Timestamp))
}
```

## Memory Analysis and Leak Detection

### Memory Profiling Tools

```go
// memory_analysis.go - Memory analysis and leak detection
package main

import (
    "context"
    "fmt"
    "runtime"
    "time"
    
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

type MemoryAnalyzer struct {
    baseline runtime.MemStats
    samples  []MemorySample
}

type MemorySample struct {
    Timestamp    time.Time
    Alloc        uint64
    TotalAlloc   uint64
    Sys          uint64
    Mallocs      uint64
    Frees        uint64
    HeapAlloc    uint64
    HeapSys      uint64
    GoroutineCount int
}

func NewMemoryAnalyzer() *MemoryAnalyzer {
    analyzer := &MemoryAnalyzer{
        samples: make([]MemorySample, 0),
    }
    
    // Capture baseline
    runtime.GC()
    runtime.ReadMemStats(&analyzer.baseline)
    
    return analyzer
}

func (ma *MemoryAnalyzer) TakeSample() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    sample := MemorySample{
        Timestamp:      time.Now(),
        Alloc:          m.Alloc,
        TotalAlloc:     m.TotalAlloc,
        Sys:            m.Sys,
        Mallocs:        m.Mallocs,
        Frees:          m.Frees,
        HeapAlloc:      m.HeapAlloc,
        HeapSys:        m.HeapSys,
        GoroutineCount: runtime.NumGoroutine(),
    }
    
    ma.samples = append(ma.samples, sample)
}

func (ma *MemoryAnalyzer) DetectLeaks() []string {
    if len(ma.samples) < 2 {
        return []string{"Insufficient samples for leak detection"}
    }
    
    var issues []string
    latest := ma.samples[len(ma.samples)-1]
    first := ma.samples[0]
    
    // Check for growing allocations
    allocGrowth := latest.Alloc - first.Alloc
    if allocGrowth > 100*1024*1024 { // 100MB growth
        issues = append(issues, fmt.Sprintf(
            "Memory allocation grew by %d MB", 
            allocGrowth/1024/1024))
    }
    
    // Check for goroutine leaks
    goroutineGrowth := latest.GoroutineCount - first.GoroutineCount
    if goroutineGrowth > 100 {
        issues = append(issues, fmt.Sprintf(
            "Goroutine count grew by %d", 
            goroutineGrowth))
    }
    
    // Check allocation/free ratio
    if latest.Mallocs > 0 && latest.Frees > 0 {
        ratio := float64(latest.Mallocs) / float64(latest.Frees)
        if ratio > 1.1 { // More than 10% unfreed allocations
            issues = append(issues, fmt.Sprintf(
                "Poor malloc/free ratio: %.2f", ratio))
        }
    }
    
    return issues
}

func (ma *MemoryAnalyzer) GetReport() string {
    if len(ma.samples) == 0 {
        return "No memory samples collected"
    }
    
    latest := ma.samples[len(ma.samples)-1]
    
    report := fmt.Sprintf(`Memory Analysis Report:
Current Memory Usage:
  Allocated: %d MB
  System: %d MB
  Heap Allocated: %d MB
  Heap System: %d MB
  Goroutines: %d

Memory Growth Since Baseline:
  Allocated: +%d MB
  System: +%d MB
  Total Allocations: +%d
  Total Frees: +%d

Potential Issues:`,
        latest.Alloc/1024/1024,
        latest.Sys/1024/1024,
        latest.HeapAlloc/1024/1024,
        latest.HeapSys/1024/1024,
        latest.GoroutineCount,
        (latest.Alloc-ma.baseline.Alloc)/1024/1024,
        (latest.Sys-ma.baseline.Sys)/1024/1024,
        latest.Mallocs-ma.baseline.Mallocs,
        latest.Frees-ma.baseline.Frees)
    
    issues := ma.DetectLeaks()
    for _, issue := range issues {
        report += fmt.Sprintf("\n  - %s", issue)
    }
    
    if len(issues) == 0 {
        report += "\n  None detected"
    }
    
    return report
}

// Memory leak detection test
func testMemoryLeaks() {
    analyzer := NewMemoryAnalyzer()
    validator := events.NewEventValidator(events.DefaultValidationConfig())
    
    fmt.Println("Starting memory leak detection test...")
    
    // Initial sample
    analyzer.TakeSample()
    
    // Simulate workload
    for round := 0; round < 10; round++ {
        fmt.Printf("Round %d...\n", round+1)
        
        // Process many events
        for i := 0; i < 10000; i++ {
            event := &events.RunStartedEvent{
                BaseEvent: &events.BaseEvent{
                    EventType:   events.EventTypeRunStarted,
                    TimestampMs: timePtr(time.Now().UnixMilli()),
                },
                RunIDValue:    fmt.Sprintf("run-%d-%d", round, i),
                ThreadIDValue: fmt.Sprintf("thread-%d", round),
            }
            
            result := validator.ValidateEvent(context.Background(), event)
            _ = result // Use result to prevent optimization
        }
        
        // Force garbage collection and take sample
        runtime.GC()
        time.Sleep(100 * time.Millisecond) // Let GC complete
        analyzer.TakeSample()
    }
    
    fmt.Println(analyzer.GetReport())
}
```

### Garbage Collection Analysis

```go
// gc_analysis.go - Garbage collection performance analysis
package main

import (
    "fmt"
    "runtime"
    "runtime/debug"
    "time"
)

type GCAnalyzer struct {
    startStats runtime.MemStats
    samples    []GCSample
}

type GCSample struct {
    Timestamp     time.Time
    NumGC         uint32
    PauseTotal    time.Duration
    LastPause     time.Duration
    GCCPUFraction float64
}

func NewGCAnalyzer() *GCAnalyzer {
    analyzer := &GCAnalyzer{
        samples: make([]GCSample, 0),
    }
    
    runtime.ReadMemStats(&analyzer.startStats)
    return analyzer
}

func (gca *GCAnalyzer) TakeSample() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    sample := GCSample{
        Timestamp:     time.Now(),
        NumGC:         m.NumGC,
        PauseTotal:    time.Duration(m.PauseTotalNs),
        GCCPUFraction: m.GCCPUFraction,
    }
    
    if m.NumGC > 0 {
        sample.LastPause = time.Duration(m.PauseNs[(m.NumGC+255)%256])
    }
    
    gca.samples = append(gca.samples, sample)
}

func (gca *GCAnalyzer) GetReport() string {
    if len(gca.samples) < 2 {
        return "Insufficient GC samples"
    }
    
    latest := gca.samples[len(gca.samples)-1]
    first := gca.samples[0]
    
    gcCount := latest.NumGC - first.NumGC
    totalTime := latest.Timestamp.Sub(first.Timestamp)
    gcFrequency := float64(gcCount) / totalTime.Seconds()
    
    report := fmt.Sprintf(`Garbage Collection Analysis:
Total GC Cycles: %d
GC Frequency: %.2f cycles/second
Last GC Pause: %v
Total GC Time: %v
GC CPU Fraction: %.4f

Performance Assessment:`,
        gcCount,
        gcFrequency,
        latest.LastPause,
        latest.PauseTotal-gca.startStats.PauseTotalNs,
        latest.GCCPUFraction)
    
    // Performance assessment
    if latest.LastPause > 100*time.Millisecond {
        report += "\n  WARNING: High GC pause time (>100ms)"
    }
    
    if gcFrequency > 2.0 {
        report += "\n  WARNING: High GC frequency (>2 cycles/second)"
    }
    
    if latest.GCCPUFraction > 0.1 {
        report += "\n  WARNING: High GC CPU usage (>10%)"
    }
    
    if latest.LastPause <= 10*time.Millisecond && 
       gcFrequency <= 1.0 && 
       latest.GCCPUFraction <= 0.05 {
        report += "\n  EXCELLENT: GC performance is optimal"
    }
    
    return report
}

// GC tuning recommendations
func getGCTuningRecommendations() string {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    recommendations := `GC Tuning Recommendations:

1. GOGC Environment Variable:
   - Current default: GOGC=100
   - For lower latency: GOGC=200 (less frequent GC)
   - For lower memory: GOGC=50 (more frequent GC)

2. Memory Ballast Pattern:
   ballast := make([]byte, 100<<20) // 100MB ballast
   runtime.KeepAlive(ballast)

3. Manual GC Control:
   debug.SetGCPercent(-1) // Disable automatic GC
   // Run GC manually at optimal times

4. Memory Pool Pattern:
   var pool = sync.Pool{
       New: func() interface{} {
           return make([]byte, 1024)
       },
   }

Current Memory Stats:`
    
    recommendations += fmt.Sprintf(`
   Heap Size: %d MB
   Next GC: %d MB
   Last GC: %v ago
   GC CPU: %.2f%%`,
        m.HeapSys/1024/1024,
        m.NextGC/1024/1024,
        time.Since(time.Unix(0, int64(m.LastGC))),
        m.GCCPUFraction*100)
    
    return recommendations
}
```

## Concurrency and Race Condition Detection

### Race Detection Tools

```bash
#!/bin/bash
# race_detection.sh - Comprehensive race condition detection

echo "Starting race condition detection..."

# Test with race detector enabled
go test -race -v ./pkg/core/events/... 2>&1 | tee race_detection.log

# Check for specific race patterns
echo "Checking for common race patterns..."

# Look for concurrent map access
grep -n "concurrent map" race_detection.log || echo "No concurrent map access detected"

# Look for data races
grep -n "WARNING: DATA RACE" race_detection.log || echo "No data races detected"

# Look for goroutine leaks
go test -race -count=10 ./pkg/core/events/... 2>&1 | grep -i "goroutine" | tail -10

echo "Race detection complete. Check race_detection.log for details."
```

### Concurrency Testing Framework

```go
// concurrency_test.go - Comprehensive concurrency testing
package main

import (
    "context"
    "fmt"
    "runtime"
    "sync"
    "sync/atomic"
    "time"
    
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

type ConcurrencyTester struct {
    validator     *events.EventValidator
    goroutineCount int64
    errorCount    int64
    successCount  int64
}

func NewConcurrencyTester() *ConcurrencyTester {
    return &ConcurrencyTester{
        validator: events.NewEventValidator(events.DefaultValidationConfig()),
    }
}

func (ct *ConcurrencyTester) TestConcurrentValidation(numGoroutines, eventsPerGoroutine int) {
    fmt.Printf("Testing concurrent validation: %d goroutines, %d events each\n", 
        numGoroutines, eventsPerGoroutine)
    
    var wg sync.WaitGroup
    startTime := time.Now()
    
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            atomic.AddInt64(&ct.goroutineCount, 1)
            
            defer func() {
                atomic.AddInt64(&ct.goroutineCount, -1)
                if r := recover(); r != nil {
                    atomic.AddInt64(&ct.errorCount, 1)
                    fmt.Printf("Worker %d panicked: %v\n", workerID, r)
                }
            }()
            
            for j := 0; j < eventsPerGoroutine; j++ {
                event := &events.RunStartedEvent{
                    BaseEvent: &events.BaseEvent{
                        EventType:   events.EventTypeRunStarted,
                        TimestampMs: timePtr(time.Now().UnixMilli()),
                    },
                    RunIDValue:    fmt.Sprintf("run-%d-%d", workerID, j),
                    ThreadIDValue: fmt.Sprintf("thread-%d", workerID),
                }
                
                result := ct.validator.ValidateEvent(context.Background(), event)
                if result.IsValid {
                    atomic.AddInt64(&ct.successCount, 1)
                } else {
                    atomic.AddInt64(&ct.errorCount, 1)
                }
                
                // Add some variability
                if j%100 == 0 {
                    runtime.Gosched()
                }
            }
        }(i)
    }
    
    wg.Wait()
    duration := time.Since(startTime)
    
    totalEvents := int64(numGoroutines * eventsPerGoroutine)
    throughput := float64(totalEvents) / duration.Seconds()
    
    fmt.Printf("Concurrency test results:\n")
    fmt.Printf("  Duration: %v\n", duration)
    fmt.Printf("  Total events: %d\n", totalEvents)
    fmt.Printf("  Successful: %d\n", atomic.LoadInt64(&ct.successCount))
    fmt.Printf("  Errors: %d\n", atomic.LoadInt64(&ct.errorCount))
    fmt.Printf("  Throughput: %.0f events/second\n", throughput)
    fmt.Printf("  Active goroutines: %d\n", atomic.LoadInt64(&ct.goroutineCount))
    
    if atomic.LoadInt64(&ct.errorCount) > 0 {
        fmt.Printf("  WARNING: %d errors detected in concurrent test\n", 
            atomic.LoadInt64(&ct.errorCount))
    }
}

func (ct *ConcurrencyTester) TestGoroutineLeaks() {
    fmt.Println("Testing for goroutine leaks...")
    
    initialGoroutines := runtime.NumGoroutine()
    
    // Run concurrent operations
    ct.TestConcurrentValidation(50, 100)
    
    // Wait for cleanup
    time.Sleep(1 * time.Second)
    runtime.GC()
    time.Sleep(1 * time.Second)
    
    finalGoroutines := runtime.NumGoroutine()
    leakedGoroutines := finalGoroutines - initialGoroutines
    
    fmt.Printf("Goroutine leak test results:\n")
    fmt.Printf("  Initial goroutines: %d\n", initialGoroutines)
    fmt.Printf("  Final goroutines: %d\n", finalGoroutines)
    fmt.Printf("  Leaked goroutines: %d\n", leakedGoroutines)
    
    if leakedGoroutines > 10 { // Allow for some variation
        fmt.Printf("  WARNING: Potential goroutine leak detected!\n")
    } else {
        fmt.Printf("  PASS: No significant goroutine leaks detected\n")
    }
}

func (ct *ConcurrencyTester) StressTest(duration time.Duration) {
    fmt.Printf("Running stress test for %v...\n", duration)
    
    ctx, cancel := context.WithTimeout(context.Background(), duration)
    defer cancel()
    
    var wg sync.WaitGroup
    numWorkers := runtime.NumCPU() * 2
    
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            eventCounter := 0
            for {
                select {
                case <-ctx.Done():
                    return
                default:
                    event := &events.RunStartedEvent{
                        BaseEvent: &events.BaseEvent{
                            EventType:   events.EventTypeRunStarted,
                            TimestampMs: timePtr(time.Now().UnixMilli()),
                        },
                        RunIDValue:    fmt.Sprintf("stress-%d-%d", workerID, eventCounter),
                        ThreadIDValue: fmt.Sprintf("thread-%d", workerID),
                    }
                    
                    result := ct.validator.ValidateEvent(context.Background(), event)
                    if result.IsValid {
                        atomic.AddInt64(&ct.successCount, 1)
                    } else {
                        atomic.AddInt64(&ct.errorCount, 1)
                    }
                    
                    eventCounter++
                }
            }
        }(i)
    }
    
    wg.Wait()
    
    totalEvents := atomic.LoadInt64(&ct.successCount) + atomic.LoadInt64(&ct.errorCount)
    throughput := float64(totalEvents) / duration.Seconds()
    
    fmt.Printf("Stress test results:\n")
    fmt.Printf("  Duration: %v\n", duration)
    fmt.Printf("  Workers: %d\n", numWorkers)
    fmt.Printf("  Total events: %d\n", totalEvents)
    fmt.Printf("  Throughput: %.0f events/second\n", throughput)
    fmt.Printf("  Error rate: %.2f%%\n", 
        float64(atomic.LoadInt64(&ct.errorCount))/float64(totalEvents)*100)
}
```

## Network and Distributed Performance

### Network Latency Measurement

```go
// network_diagnostics.go - Network performance diagnostics
package main

import (
    "context"
    "fmt"
    "net"
    "sync"
    "time"
)

type NetworkDiagnostics struct {
    targets []string
    results map[string][]LatencyMeasurement
    mu      sync.RWMutex
}

type LatencyMeasurement struct {
    Timestamp time.Time
    Latency   time.Duration
    Success   bool
    Error     error
}

func NewNetworkDiagnostics(targets []string) *NetworkDiagnostics {
    return &NetworkDiagnostics{
        targets: targets,
        results: make(map[string][]LatencyMeasurement),
    }
}

func (nd *NetworkDiagnostics) MeasureLatency(target string, samples int) []LatencyMeasurement {
    measurements := make([]LatencyMeasurement, samples)
    
    for i := 0; i < samples; i++ {
        start := time.Now()
        
        conn, err := net.DialTimeout("tcp", target, 5*time.Second)
        latency := time.Since(start)
        
        measurement := LatencyMeasurement{
            Timestamp: start,
            Latency:   latency,
            Success:   err == nil,
            Error:     err,
        }
        
        if conn != nil {
            conn.Close()
        }
        
        measurements[i] = measurement
        
        // Wait between measurements
        time.Sleep(100 * time.Millisecond)
    }
    
    nd.mu.Lock()
    nd.results[target] = measurements
    nd.mu.Unlock()
    
    return measurements
}

func (nd *NetworkDiagnostics) RunDiagnostics(samples int) {
    fmt.Println("Running network diagnostics...")
    
    var wg sync.WaitGroup
    
    for _, target := range nd.targets {
        wg.Add(1)
        go func(t string) {
            defer wg.Done()
            nd.MeasureLatency(t, samples)
        }(target)
    }
    
    wg.Wait()
    nd.PrintReport()
}

func (nd *NetworkDiagnostics) PrintReport() {
    nd.mu.RLock()
    defer nd.mu.RUnlock()
    
    fmt.Println("\nNetwork Diagnostics Report:")
    fmt.Println("============================")
    
    for target, measurements := range nd.results {
        fmt.Printf("\nTarget: %s\n", target)
        
        var totalLatency time.Duration
        var successCount int
        var minLatency, maxLatency time.Duration
        
        minLatency = time.Hour // Initialize to high value
        
        for _, m := range measurements {
            if m.Success {
                successCount++
                totalLatency += m.Latency
                
                if minLatency > m.Latency {
                    minLatency = m.Latency
                }
                if maxLatency < m.Latency {
                    maxLatency = m.Latency
                }
            }
        }
        
        if successCount > 0 {
            avgLatency := totalLatency / time.Duration(successCount)
            successRate := float64(successCount) / float64(len(measurements)) * 100
            
            fmt.Printf("  Success Rate: %.1f%%\n", successRate)
            fmt.Printf("  Average Latency: %v\n", avgLatency)
            fmt.Printf("  Min Latency: %v\n", minLatency)
            fmt.Printf("  Max Latency: %v\n", maxLatency)
            
            // Performance assessment
            if avgLatency > 100*time.Millisecond {
                fmt.Printf("  WARNING: High average latency\n")
            }
            if successRate < 95 {
                fmt.Printf("  WARNING: Low success rate\n")
            }
            if maxLatency > 1*time.Second {
                fmt.Printf("  WARNING: High maximum latency\n")
            }
        } else {
            fmt.Printf("  ERROR: No successful connections\n")
        }
    }
}

// Distributed system diagnostics
func diagnoseDitributedSystem() {
    nodes := []string{
        "node1.example.com:8080",
        "node2.example.com:8080", 
        "node3.example.com:8080",
    }
    
    diagnostics := NewNetworkDiagnostics(nodes)
    diagnostics.RunDiagnostics(50) // 50 samples per node
    
    // Test with different connection patterns
    fmt.Println("\nTesting mesh connectivity...")
    testMeshConnectivity(nodes)
}

func testMeshConnectivity(nodes []string) {
    for i, source := range nodes {
        for j, target := range nodes {
            if i != j {
                fmt.Printf("Testing %s -> %s\n", source, target)
                
                start := time.Now()
                conn, err := net.DialTimeout("tcp", target, 2*time.Second)
                latency := time.Since(start)
                
                if err != nil {
                    fmt.Printf("  FAILED: %v\n", err)
                } else {
                    fmt.Printf("  SUCCESS: %v\n", latency)
                    conn.Close()
                }
            }
        }
    }
}
```

## Cache Performance Optimization

### Cache Performance Analysis

```go
// cache_diagnostics.go - Cache performance analysis
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/ag-ui/go-sdk/pkg/core/events/cache"
)

type CachePerformanceAnalyzer struct {
    validator *cache.CacheValidator
    metrics   []CacheMetricSample
}

type CacheMetricSample struct {
    Timestamp time.Time
    L1Hits    int64
    L1Misses  int64
    L2Hits    int64
    L2Misses  int64
    HitRate   float64
    Latency   time.Duration
}

func NewCachePerformanceAnalyzer() *CachePerformanceAnalyzer {
    config := cache.DefaultCacheValidatorConfig()
    config.L1Size = 10000
    config.L1TTL = 5 * time.Minute
    config.MetricsEnabled = true
    
    validator, _ := cache.NewCacheValidator(config)
    
    return &CachePerformanceAnalyzer{
        validator: validator,
        metrics:   make([]CacheMetricSample, 0),
    }
}

func (cpa *CachePerformanceAnalyzer) RunPerformanceTest(numEvents int) {
    fmt.Printf("Running cache performance test with %d events...\n", numEvents)
    
    // Warm up cache
    for i := 0; i < 1000; i++ {
        event := createTestEvent()
        cpa.validator.ValidateEvent(context.Background(), event)
    }
    
    // Measure performance with different access patterns
    cpa.testSequentialAccess(numEvents / 4)
    cpa.testRandomAccess(numEvents / 4)
    cpa.testRepeatedAccess(numEvents / 4)
    cpa.testMixedAccess(numEvents / 4)
    
    cpa.analyzeResults()
}

func (cpa *CachePerformanceAnalyzer) testSequentialAccess(numEvents int) {
    fmt.Println("Testing sequential access pattern...")
    
    start := time.Now()
    for i := 0; i < numEvents; i++ {
        event := &events.RunStartedEvent{
            BaseEvent: &events.BaseEvent{
                EventType:   events.EventTypeRunStarted,
                TimestampMs: timePtr(time.Now().UnixMilli()),
            },
            RunIDValue:    fmt.Sprintf("seq-run-%d", i),
            ThreadIDValue: "seq-thread",
        }
        
        cpa.validator.ValidateEvent(context.Background(), event)
    }
    
    cpa.recordMetrics("sequential", time.Since(start))
}

func (cpa *CachePerformanceAnalyzer) testRandomAccess(numEvents int) {
    fmt.Println("Testing random access pattern...")
    
    // Create pool of events
    events := make([]*events.RunStartedEvent, 100)
    for i := range events {
        events[i] = &events.RunStartedEvent{
            BaseEvent: &events.BaseEvent{
                EventType:   events.EventTypeRunStarted,
                TimestampMs: timePtr(time.Now().UnixMilli()),
            },
            RunIDValue:    fmt.Sprintf("rand-run-%d", i),
            ThreadIDValue: "rand-thread",
        }
    }
    
    start := time.Now()
    for i := 0; i < numEvents; i++ {
        event := events[i%len(events)] // Random-ish access
        cpa.validator.ValidateEvent(context.Background(), event)
    }
    
    cpa.recordMetrics("random", time.Since(start))
}

func (cpa *CachePerformanceAnalyzer) testRepeatedAccess(numEvents int) {
    fmt.Println("Testing repeated access pattern...")
    
    event := &events.RunStartedEvent{
        BaseEvent: &events.BaseEvent{
            EventType:   events.EventTypeRunStarted,
            TimestampMs: timePtr(time.Now().UnixMilli()),
        },
        RunIDValue:    "repeated-run",
        ThreadIDValue: "repeated-thread",
    }
    
    start := time.Now()
    for i := 0; i < numEvents; i++ {
        cpa.validator.ValidateEvent(context.Background(), event)
    }
    
    cpa.recordMetrics("repeated", time.Since(start))
}

func (cpa *CachePerformanceAnalyzer) testMixedAccess(numEvents int) {
    fmt.Println("Testing mixed access pattern...")
    
    start := time.Now()
    for i := 0; i < numEvents; i++ {
        var event *events.RunStartedEvent
        
        switch i % 4 {
        case 0: // New event
            event = &events.RunStartedEvent{
                BaseEvent: &events.BaseEvent{
                    EventType:   events.EventTypeRunStarted,
                    TimestampMs: timePtr(time.Now().UnixMilli()),
                },
                RunIDValue:    fmt.Sprintf("mixed-new-%d", i),
                ThreadIDValue: "mixed-thread",
            }
        case 1: // Repeated event
            event = &events.RunStartedEvent{
                BaseEvent: &events.BaseEvent{
                    EventType:   events.EventTypeRunStarted,
                    TimestampMs: timePtr(time.Now().UnixMilli()),
                },
                RunIDValue:    "mixed-repeated",
                ThreadIDValue: "mixed-thread",
            }
        case 2: // Recent event
            event = &events.RunStartedEvent{
                BaseEvent: &events.BaseEvent{
                    EventType:   events.EventTypeRunStarted,
                    TimestampMs: timePtr(time.Now().UnixMilli()),
                },
                RunIDValue:    fmt.Sprintf("mixed-recent-%d", (i/10)*10), // Group by 10s
                ThreadIDValue: "mixed-thread",
            }
        case 3: // Old event (likely cache miss)
            event = &events.RunStartedEvent{
                BaseEvent: &events.BaseEvent{
                    EventType:   events.EventTypeRunStarted,
                    TimestampMs: timePtr(time.Now().UnixMilli()),
                },
                RunIDValue:    fmt.Sprintf("mixed-old-%d", i/1000), // Older pattern
                ThreadIDValue: "mixed-thread",
            }
        }
        
        cpa.validator.ValidateEvent(context.Background(), event)
    }
    
    cpa.recordMetrics("mixed", time.Since(start))
}

func (cpa *CachePerformanceAnalyzer) recordMetrics(pattern string, duration time.Duration) {
    stats := cpa.validator.GetStats()
    
    sample := CacheMetricSample{
        Timestamp: time.Now(),
        L1Hits:    stats.L1Hits,
        L1Misses:  stats.L1Misses,
        L2Hits:    stats.L2Hits,
        L2Misses:  stats.L2Misses,
        HitRate:   float64(stats.L1Hits+stats.L2Hits) / float64(stats.L1Hits+stats.L1Misses+stats.L2Hits+stats.L2Misses) * 100,
        Latency:   duration,
    }
    
    cpa.metrics = append(cpa.metrics, sample)
    
    fmt.Printf("  %s pattern results:\n", pattern)
    fmt.Printf("    Hit rate: %.1f%%\n", sample.HitRate)
    fmt.Printf("    L1 hits: %d, L1 misses: %d\n", sample.L1Hits, sample.L1Misses)
    fmt.Printf("    L2 hits: %d, L2 misses: %d\n", sample.L2Hits, sample.L2Misses)
    fmt.Printf("    Duration: %v\n", duration)
}

func (cpa *CachePerformanceAnalyzer) analyzeResults() {
    fmt.Println("\nCache Performance Analysis:")
    fmt.Println("===========================")
    
    if len(cpa.metrics) == 0 {
        fmt.Println("No metrics collected")
        return
    }
    
    // Calculate overall statistics
    var totalHits, totalMisses int64
    var totalDuration time.Duration
    
    for _, metric := range cpa.metrics {
        totalHits += metric.L1Hits + metric.L2Hits
        totalMisses += metric.L1Misses + metric.L2Misses
        totalDuration += metric.Latency
    }
    
    overallHitRate := float64(totalHits) / float64(totalHits+totalMisses) * 100
    avgDuration := totalDuration / time.Duration(len(cpa.metrics))
    
    fmt.Printf("Overall Performance:\n")
    fmt.Printf("  Hit Rate: %.1f%%\n", overallHitRate)
    fmt.Printf("  Total Operations: %d\n", totalHits+totalMisses)
    fmt.Printf("  Average Test Duration: %v\n", avgDuration)
    
    // Performance recommendations
    fmt.Println("\nRecommendations:")
    if overallHitRate < 80 {
        fmt.Println("  - Consider increasing cache size")
        fmt.Println("  - Extend TTL for stable validation results")
        fmt.Println("  - Enable prefetch engine")
    }
    
    if overallHitRate > 95 {
        fmt.Println("  - Excellent cache performance!")
        fmt.Println("  - Consider reducing cache size to save memory")
    }
    
    // L2 cache analysis
    l2Usage := false
    for _, metric := range cpa.metrics {
        if metric.L2Hits > 0 || metric.L2Misses > 0 {
            l2Usage = true
            break
        }
    }
    
    if !l2Usage {
        fmt.Println("  - L2 cache not being used, consider enabling for better hit rates")
    }
}
```

## Benchmarking and Load Testing

### Comprehensive Benchmarking Suite

```go
// benchmark_suite.go - Comprehensive benchmarking framework
package main

import (
    "context"
    "fmt"
    "runtime"
    "sync"
    "sync/atomic"
    "testing"
    "time"
    
    "github.com/ag-ui/go-sdk/pkg/core/events"
)

type BenchmarkSuite struct {
    validator *events.EventValidator
    results   []BenchmarkResult
}

type BenchmarkResult struct {
    Name          string
    Duration      time.Duration
    Operations    int64
    Throughput    float64
    MemoryUsage   uint64
    Allocations   uint64
    GoroutineCount int
}

func NewBenchmarkSuite() *BenchmarkSuite {
    return &BenchmarkSuite{
        validator: events.NewEventValidator(events.DefaultValidationConfig()),
        results:   make([]BenchmarkResult, 0),
    }
}

func (bs *BenchmarkSuite) RunAllBenchmarks() {
    fmt.Println("Starting comprehensive benchmark suite...")
    
    // Single-threaded benchmarks
    bs.benchmarkSingleValidation()
    bs.benchmarkSequentialValidation()
    bs.benchmarkDifferentEventTypes()
    
    // Multi-threaded benchmarks
    bs.benchmarkConcurrentValidation()
    bs.benchmarkLoadTest()
    
    // Memory benchmarks
    bs.benchmarkMemoryUsage()
    
    // Print summary
    bs.printSummary()
}

func (bs *BenchmarkSuite) benchmarkSingleValidation() {
    fmt.Println("Benchmarking single event validation...")
    
    event := createTestEvent()
    const iterations = 100000
    
    // Warm up
    for i := 0; i < 1000; i++ {
        bs.validator.ValidateEvent(context.Background(), event)
    }
    
    // Measure
    var m1, m2 runtime.MemStats
    runtime.ReadMemStats(&m1)
    
    start := time.Now()
    for i := 0; i < iterations; i++ {
        result := bs.validator.ValidateEvent(context.Background(), event)
        _ = result // Prevent optimization
    }
    duration := time.Since(start)
    
    runtime.ReadMemStats(&m2)
    
    result := BenchmarkResult{
        Name:          "Single Validation",
        Duration:      duration,
        Operations:    iterations,
        Throughput:    float64(iterations) / duration.Seconds(),
        MemoryUsage:   m2.Alloc - m1.Alloc,
        Allocations:   m2.Mallocs - m1.Mallocs,
        GoroutineCount: runtime.NumGoroutine(),
    }
    
    bs.results = append(bs.results, result)
    bs.printResult(result)
}

func (bs *BenchmarkSuite) benchmarkSequentialValidation() {
    fmt.Println("Benchmarking sequential validation...")
    
    const iterations = 10000
    events := make([]events.Event, iterations)
    
    for i := range events {
        events[i] = &events.RunStartedEvent{
            BaseEvent: &events.BaseEvent{
                EventType:   events.EventTypeRunStarted,
                TimestampMs: timePtr(time.Now().UnixMilli()),
            },
            RunIDValue:    fmt.Sprintf("bench-run-%d", i),
            ThreadIDValue: "bench-thread",
        }
    }
    
    var m1, m2 runtime.MemStats
    runtime.ReadMemStats(&m1)
    
    start := time.Now()
    for _, event := range events {
        result := bs.validator.ValidateEvent(context.Background(), event)
        _ = result
    }
    duration := time.Since(start)
    
    runtime.ReadMemStats(&m2)
    
    result := BenchmarkResult{
        Name:          "Sequential Validation",
        Duration:      duration,
        Operations:    iterations,
        Throughput:    float64(iterations) / duration.Seconds(),
        MemoryUsage:   m2.Alloc - m1.Alloc,
        Allocations:   m2.Mallocs - m1.Mallocs,
        GoroutineCount: runtime.NumGoroutine(),
    }
    
    bs.results = append(bs.results, result)
    bs.printResult(result)
}

func (bs *BenchmarkSuite) benchmarkConcurrentValidation() {
    fmt.Println("Benchmarking concurrent validation...")
    
    const numGoroutines = 100
    const eventsPerGoroutine = 1000
    const totalOperations = numGoroutines * eventsPerGoroutine
    
    var wg sync.WaitGroup
    var operations int64
    
    var m1, m2 runtime.MemStats
    runtime.ReadMemStats(&m1)
    
    start := time.Now()
    
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            for j := 0; j < eventsPerGoroutine; j++ {
                event := &events.RunStartedEvent{
                    BaseEvent: &events.BaseEvent{
                        EventType:   events.EventTypeRunStarted,
                        TimestampMs: timePtr(time.Now().UnixMilli()),
                    },
                    RunIDValue:    fmt.Sprintf("concurrent-run-%d-%d", workerID, j),
                    ThreadIDValue: fmt.Sprintf("concurrent-thread-%d", workerID),
                }
                
                result := bs.validator.ValidateEvent(context.Background(), event)
                _ = result
                atomic.AddInt64(&operations, 1)
            }
        }(i)
    }
    
    wg.Wait()
    duration := time.Since(start)
    
    runtime.ReadMemStats(&m2)
    
    result := BenchmarkResult{
        Name:          "Concurrent Validation",
        Duration:      duration,
        Operations:    atomic.LoadInt64(&operations),
        Throughput:    float64(atomic.LoadInt64(&operations)) / duration.Seconds(),
        MemoryUsage:   m2.Alloc - m1.Alloc,
        Allocations:   m2.Mallocs - m1.Mallocs,
        GoroutineCount: runtime.NumGoroutine(),
    }
    
    bs.results = append(bs.results, result)
    bs.printResult(result)
}

func (bs *BenchmarkSuite) benchmarkLoadTest() {
    fmt.Println("Running load test...")
    
    duration := 30 * time.Second
    ctx, cancel := context.WithTimeout(context.Background(), duration)
    defer cancel()
    
    numWorkers := runtime.NumCPU() * 2
    var operations int64
    var wg sync.WaitGroup
    
    var m1, m2 runtime.MemStats
    runtime.ReadMemStats(&m1)
    
    start := time.Now()
    
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            eventCounter := 0
            for {
                select {
                case <-ctx.Done():
                    return
                default:
                    event := &events.RunStartedEvent{
                        BaseEvent: &events.BaseEvent{
                            EventType:   events.EventTypeRunStarted,
                            TimestampMs: timePtr(time.Now().UnixMilli()),
                        },
                        RunIDValue:    fmt.Sprintf("load-run-%d-%d", workerID, eventCounter),
                        ThreadIDValue: fmt.Sprintf("load-thread-%d", workerID),
                    }
                    
                    result := bs.validator.ValidateEvent(context.Background(), event)
                    _ = result
                    atomic.AddInt64(&operations, 1)
                    eventCounter++
                }
            }
        }(i)
    }
    
    wg.Wait()
    actualDuration := time.Since(start)
    
    runtime.ReadMemStats(&m2)
    
    result := BenchmarkResult{
        Name:          "Load Test",
        Duration:      actualDuration,
        Operations:    atomic.LoadInt64(&operations),
        Throughput:    float64(atomic.LoadInt64(&operations)) / actualDuration.Seconds(),
        MemoryUsage:   m2.Alloc - m1.Alloc,
        Allocations:   m2.Mallocs - m1.Mallocs,
        GoroutineCount: runtime.NumGoroutine(),
    }
    
    bs.results = append(bs.results, result)
    bs.printResult(result)
}

func (bs *BenchmarkSuite) printResult(result BenchmarkResult) {
    fmt.Printf("  %s Results:\n", result.Name)
    fmt.Printf("    Duration: %v\n", result.Duration)
    fmt.Printf("    Operations: %d\n", result.Operations)
    fmt.Printf("    Throughput: %.0f ops/sec\n", result.Throughput)
    fmt.Printf("    Memory Used: %d bytes\n", result.MemoryUsage)
    fmt.Printf("    Allocations: %d\n", result.Allocations)
    fmt.Printf("    Goroutines: %d\n", result.GoroutineCount)
    fmt.Println()
}

func (bs *BenchmarkSuite) printSummary() {
    fmt.Println("Benchmark Summary:")
    fmt.Println("==================")
    
    for _, result := range bs.results {
        fmt.Printf("%-20s: %8.0f ops/sec, %6dms, %4d MB\n",
            result.Name,
            result.Throughput,
            result.Duration.Milliseconds(),
            result.MemoryUsage/1024/1024)
    }
    
    // Find best and worst performance
    if len(bs.results) > 0 {
        bestThroughput := bs.results[0]
        worstThroughput := bs.results[0]
        
        for _, result := range bs.results {
            if result.Throughput > bestThroughput.Throughput {
                bestThroughput = result
            }
            if result.Throughput < worstThroughput.Throughput {
                worstThroughput = result
            }
        }
        
        fmt.Printf("\nBest Performance: %s (%.0f ops/sec)\n", 
            bestThroughput.Name, bestThroughput.Throughput)
        fmt.Printf("Worst Performance: %s (%.0f ops/sec)\n", 
            worstThroughput.Name, worstThroughput.Throughput)
    }
}
```

This comprehensive performance debugging guide provides tools and techniques for analyzing every aspect of the AG-UI Go SDK's performance, from CPU and memory usage to concurrency and distributed operations. The diagnostic commands and code examples enable developers to identify bottlenecks, detect leaks, and optimize their validation systems for production workloads.

<function_calls>
<invoke name="TodoWrite">
<parameter name="todos">[{"id": "1", "content": "Create comprehensive testing guide with unit, integration, and performance testing patterns", "status": "completed", "priority": "high"}, {"id": "2", "content": "Add troubleshooting sections to auth README", "status": "completed", "priority": "high"}, {"id": "3", "content": "Add troubleshooting sections to cache README", "status": "completed", "priority": "high"}, {"id": "4", "content": "Add troubleshooting sections to distributed README", "status": "completed", "priority": "high"}, {"id": "5", "content": "Add troubleshooting sections to analytics README", "status": "completed", "priority": "high"}, {"id": "6", "content": "Create testing patterns documentation with mock and stub usage", "status": "completed", "priority": "medium"}, {"id": "7", "content": "Document performance debugging techniques and diagnostic commands", "status": "completed", "priority": "medium"}]