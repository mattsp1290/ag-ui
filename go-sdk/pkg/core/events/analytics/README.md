# Analytics Engine

The Analytics Engine provides advanced real-time analytics capabilities for AG-UI event streams, including pattern recognition, anomaly detection, predictive analytics, and performance trend analysis.

## Overview

The analytics package offers multiple levels of analytics capabilities:

1. **Simple Analytics Engine** - Basic pattern detection and anomaly detection
2. **Advanced Analytics Engine** (future) - Machine learning integration, complex pattern recognition, and predictive modeling

## Features

### Current Implementation (Simple Analytics)

- **Real-time Event Processing**: Analyze events as they occur
- **Pattern Detection**: Identify recurring patterns in event sequences
- **Anomaly Detection**: Detect unusual events based on frequency analysis
- **Event Buffering**: Maintain a rolling window of recent events for analysis
- **Metrics Collection**: Track processing performance and detection counts
- **Thread-Safe Operations**: Concurrent access to analytics data

### Future Implementation (Advanced Analytics)

- **Machine Learning Integration**: Advanced pattern recognition using ML models
- **Predictive Analytics**: Forecast future events based on historical patterns
- **Trend Analysis**: Analyze performance trends and seasonal patterns
- **Complex Anomaly Detection**: Multi-dimensional anomaly detection using statistical models
- **Real-time Stream Processing**: High-throughput event processing with multiple workers
- **Custom Rule Engine**: Configurable rules for custom analytics logic

## Quick Start

### Basic Usage

```go
package main

import (
    "fmt"
    "time"
    
    "github.com/ag-ui/go-sdk/pkg/core/events"
    "github.com/ag-ui/go-sdk/pkg/core/events/analytics"
)

func main() {
    // Create analytics engine with default configuration
    engine := analytics.NewSimpleAnalyticsEngine(nil)
    
    // Create a mock event for demonstration
    event := createMockEvent(events.EventTypeTextMessageStart)
    
    // Analyze the event
    result, err := engine.AnalyzeEvent(event)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Event Type: %s\n", result.EventType)
    fmt.Printf("Processing Time: %v\n", result.ProcessingTime)
    fmt.Printf("Patterns Found: %v\n", result.PatternsFound)
    fmt.Printf("Is Anomaly: %t\n", result.IsAnomaly)
    fmt.Printf("Anomaly Score: %.2f\n", result.AnomalyScore)
}
```

### Custom Configuration

```go
config := &analytics.SimpleAnalyticsConfig{
    BufferSize:      1000,              // Number of events to keep in buffer
    AnalysisWindow:  5 * time.Minute,   // Time window for pattern analysis
    MinPatternCount: 3,                 // Minimum occurrences to detect a pattern
}

engine := analytics.NewSimpleAnalyticsEngine(config)
```

## API Reference

### SimpleAnalyticsEngine

The main analytics engine for processing events.

#### Methods

##### `NewSimpleAnalyticsEngine(config *SimpleAnalyticsConfig) *SimpleAnalyticsEngine`

Creates a new analytics engine with the specified configuration. If config is nil, default values are used.

##### `AnalyzeEvent(event events.Event) (*SimpleAnalyticsResult, error)`

Analyzes a single event and returns analytics results including patterns found and anomaly detection.

##### `GetMetrics() *SimpleMetrics`

Returns current analytics metrics including events processed, patterns detected, and anomalies found.

##### `GetPatterns() map[string]*SimplePattern`

Returns all detected patterns with their statistics.

##### `GetRecentEvents(duration time.Duration) []events.Event`

Returns events from the buffer within the specified time duration.

##### `Reset()`

Resets all analytics state including metrics, patterns, and event buffer.

### Configuration

#### SimpleAnalyticsConfig

```go
type SimpleAnalyticsConfig struct {
    BufferSize      int           // Maximum number of events to store
    AnalysisWindow  time.Duration // Time window for pattern analysis
    MinPatternCount int           // Minimum occurrences to detect a pattern
}
```

**Default Values:**
- BufferSize: 1000 events
- AnalysisWindow: 5 minutes
- MinPatternCount: 3 occurrences

### Results

#### SimpleAnalyticsResult

```go
type SimpleAnalyticsResult struct {
    EventType        events.EventType  // Type of the analyzed event
    Timestamp        time.Time         // Analysis timestamp
    PatternsFound    []string          // Names of detected patterns
    IsAnomaly        bool              // Whether event is anomalous
    AnomalyScore     float64           // Anomaly score (0.0-1.0)
    ProcessingTime   time.Duration     // Time taken to process
}
```

#### SimpleMetrics

```go
type SimpleMetrics struct {
    EventsProcessed   int64     // Total events processed
    PatternsDetected  int64     // Total patterns detected
    AnomaliesDetected int64     // Total anomalies detected
    LastUpdate        time.Time // Last metrics update time
}
```

#### SimplePattern

```go
type SimplePattern struct {
    ID          string           // Unique pattern identifier
    Name        string           // Human-readable pattern name
    EventType   events.EventType // Event type this pattern applies to
    Count       int              // Current count in analysis window
    LastSeen    time.Time        // Last time pattern was detected
    Window      time.Duration    // Analysis window for this pattern
}
```

## Algorithm Details

### Pattern Detection

The current implementation uses a simple frequency-based approach:

1. **Event Counting**: Count occurrences of each event type within the analysis window
2. **Threshold Checking**: Compare counts against the minimum pattern threshold
3. **Pattern Creation**: Create or update pattern records for qualifying event types
4. **Pattern Reporting**: Report patterns that exceed the threshold

### Anomaly Detection

The anomaly detection algorithm analyzes event frequency:

1. **Frequency Calculation**: Calculate the ratio of specific event type to total events
2. **Rarity Analysis**: Events with frequency < 5% are considered rare anomalies
3. **Frequency Analysis**: Events with frequency > 80% are considered frequent anomalies
4. **Scoring**: Assign anomaly scores based on deviation from normal frequency ranges

**Anomaly Score Calculation:**
- Rare events (< 5%): `score = 1.0 - (frequency * 20)`
- Frequent events (> 80%): `score = frequency`
- Normal events: `score = 0.0`

Events with scores > 0.7 are flagged as anomalies.

## Performance Considerations

### Memory Usage

- Event buffer size directly impacts memory usage
- Default buffer of 1000 events uses approximately 100KB-1MB depending on event size
- Patterns and metrics have minimal memory overhead

### Processing Performance

- Analysis time is O(n) where n is the buffer size
- Typical processing time: < 1ms per event
- Thread-safe operations may introduce slight overhead in high-concurrency scenarios

### Recommended Configurations

#### Low Volume (< 100 events/minute)
```go
config := &SimpleAnalyticsConfig{
    BufferSize:      500,
    AnalysisWindow:  10 * time.Minute,
    MinPatternCount: 3,
}
```

#### Medium Volume (100-1000 events/minute)
```go
config := &SimpleAnalyticsConfig{
    BufferSize:      1000,           // Default
    AnalysisWindow:  5 * time.Minute, // Default
    MinPatternCount: 5,
}
```

#### High Volume (> 1000 events/minute)
```go
config := &SimpleAnalyticsConfig{
    BufferSize:      2000,
    AnalysisWindow:  2 * time.Minute,
    MinPatternCount: 10,
}
```

## Testing

The analytics package includes comprehensive tests covering:

- Engine creation and configuration
- Event analysis and pattern detection
- Anomaly detection algorithms
- Buffer management and overflow handling
- Metrics collection and reporting
- State reset functionality
- Performance benchmarks

### Running Tests

```bash
# Run all analytics tests
go test ./pkg/core/events/analytics/ -v

# Run specific test
go test ./pkg/core/events/analytics/ -v -run TestSimpleAnalyticsEngine_Creation

# Run benchmarks
go test ./pkg/core/events/analytics/ -bench=.
```

## Future Enhancements

### Advanced Analytics Engine

The future advanced analytics engine will include:

1. **Machine Learning Models**
   - Neural networks for pattern recognition
   - Clustering algorithms for anomaly detection
   - Time series forecasting models

2. **Stream Processing**
   - Multi-threaded event processing
   - Real-time analytics with sub-millisecond latency
   - Backpressure handling and load balancing

3. **Advanced Pattern Recognition**
   - Sequential pattern mining
   - Temporal pattern analysis
   - Semantic content analysis
   - Custom pattern definitions

4. **Predictive Analytics**
   - Event forecasting
   - Trend prediction
   - Performance modeling
   - Capacity planning insights

5. **Integration Features**
   - Export to monitoring systems (Prometheus, Grafana)
   - Real-time alerting and notifications
   - Dashboard and visualization support
   - Historical data analysis and reporting

### Migration Path

When the advanced analytics engine becomes available:

1. The simple analytics engine will remain for lightweight use cases
2. A migration utility will help transition configurations
3. Backward compatibility will be maintained for existing integrations
4. Performance improvements will benefit both implementations

## Troubleshooting

### Common Issues and Solutions

#### Low Pattern Detection Accuracy

**Problem**: Analytics engine not detecting expected patterns
```
Warning: pattern detection confidence below 50%
Info: only 2 patterns detected in last hour with 10K events
```

**Diagnostic Commands:**
```bash
# Test pattern detection
go test -v -run TestPatternDetection ./pkg/core/events/analytics/

# Run analytics benchmarks
go test -bench=BenchmarkAnalytics ./pkg/core/events/analytics/
```

**Diagnostic Steps:**
1. Check analytics configuration:
   ```go
   config := engine.GetConfig()
   log.Printf("Buffer size: %d", config.BufferSize)
   log.Printf("Analysis window: %v", config.AnalysisWindow)
   log.Printf("Min pattern count: %d", config.MinPatternCount)
   ```

2. Analyze event distribution:
   ```go
   metrics := engine.GetMetrics()
   log.Printf("Events processed: %d", metrics.EventsProcessed)
   log.Printf("Patterns detected: %d", metrics.PatternsDetected)
   
   patterns := engine.GetPatterns()
   for patternID, pattern := range patterns {
       log.Printf("Pattern %s: count=%d, frequency=%.2f", 
           patternID, pattern.Count, float64(pattern.Count)/float64(metrics.EventsProcessed))
   }
   ```

3. Check event variety:
   ```go
   recentEvents := engine.GetRecentEvents(config.AnalysisWindow)
   eventTypeCount := make(map[events.EventType]int)
   
   for _, event := range recentEvents {
       eventTypeCount[event.GetEventType()]++
   }
   
   log.Printf("Event type distribution:")
   for eventType, count := range eventTypeCount {
       log.Printf("  %s: %d events", eventType, count)
   }
   ```

**Solutions:**
- Adjust minimum pattern count: `config.MinPatternCount = 2` for smaller datasets
- Extend analysis window: `config.AnalysisWindow = 10 * time.Minute`
- Increase buffer size: `config.BufferSize = 2000` for longer history
- Verify event types have sufficient variety
- Check for clock synchronization issues affecting timestamps

#### High Memory Usage

**Problem**: Analytics engine consuming excessive memory
```
Error: analytics buffer using 500MB+ memory
Warning: GC pressure increased due to analytics
```

**Diagnostic Commands:**
```bash
# Profile memory usage
go test -memprofile=analytics.prof -bench=BenchmarkMemory ./pkg/core/events/analytics/
go tool pprof analytics.prof

# Monitor memory over time
watch -n 5 'ps aux | grep analytics'
```

**Diagnostic Steps:**
1. Check buffer size and usage:
   ```go
   import "runtime"
   
   var m runtime.MemStats
   runtime.ReadMemStats(&m)
   log.Printf("Total memory: %d MB", m.Alloc/1024/1024)
   
   metrics := engine.GetMetrics()
   bufferEvents := len(engine.GetRecentEvents(24 * time.Hour))
   log.Printf("Buffer contains %d events", bufferEvents)
   
   // Estimate memory per event
   if bufferEvents > 0 {
       avgEventSize := m.Alloc / uint64(bufferEvents)
       log.Printf("Average event size: %d bytes", avgEventSize)
   }
   ```

2. Analyze memory growth:
   ```go
   func monitorMemoryGrowth(engine *SimpleAnalyticsEngine) {
       var m1, m2 runtime.MemStats
       
       runtime.ReadMemStats(&m1)
       
       // Process events for a period
       time.Sleep(5 * time.Minute)
       
       runtime.ReadMemStats(&m2)
       growth := m2.Alloc - m1.Alloc
       
       log.Printf("Memory growth in 5 minutes: %d MB", growth/1024/1024)
       
       if growth > 50*1024*1024 { // 50MB
           log.Printf("WARNING: High memory growth detected")
       }
   }
   ```

**Solutions:**
- Reduce buffer size: `config.BufferSize = 500`
- Shorten analysis window: `config.AnalysisWindow = 2 * time.Minute`
- Implement event sampling for high-volume scenarios
- Use more efficient data structures for large event buffers
- Implement periodic buffer cleanup
- Consider using external storage for historical data

#### Poor Anomaly Detection Performance

**Problem**: Anomaly detection missing obvious anomalies or generating false positives
```
Warning: anomaly detection sensitivity too low
Error: false positive rate at 25%
```

**Diagnostic Steps:**
1. Analyze anomaly detection parameters:
   ```go
   result, _ := engine.AnalyzeEvent(event)
   log.Printf("Event frequency: %.4f", getEventFrequency(event, engine))
   log.Printf("Anomaly score: %.2f", result.AnomalyScore)
   log.Printf("Is anomaly: %t", result.IsAnomaly)
   
   // Check frequency distribution
   recentEvents := engine.GetRecentEvents(config.AnalysisWindow)
   freqMap := make(map[events.EventType]int)
   for _, e := range recentEvents {
       freqMap[e.GetEventType()]++
   }
   
   total := len(recentEvents)
   for eventType, count := range freqMap {
       frequency := float64(count) / float64(total)
       log.Printf("Event type %s: frequency=%.2f%%", eventType, frequency*100)
   }
   ```

2. Test anomaly detection with known anomalies:
   ```go
   // Create rare event (should be anomaly)
   rareEvent := createRareEvent()
   result, _ := engine.AnalyzeEvent(rareEvent)
   log.Printf("Rare event detected as anomaly: %t (score: %.2f)", 
       result.IsAnomaly, result.AnomalyScore)
   
   // Create common event (should not be anomaly)
   commonEvent := createCommonEvent()
   result, _ = engine.AnalyzeEvent(commonEvent)
   log.Printf("Common event detected as anomaly: %t (score: %.2f)", 
       result.IsAnomaly, result.AnomalyScore)
   ```

**Solutions:**
- Tune anomaly thresholds based on data characteristics
- Implement adaptive thresholds that adjust over time
- Use statistical methods (z-score, IQR) for more sophisticated detection
- Implement learning period to establish baseline behavior
- Add whitelist for known non-anomalous rare events
- Consider time-based patterns (e.g., daily/weekly cycles)

#### Performance Degradation with High Event Volume

**Problem**: Analytics engine slowing down with increased event load
```
Warning: event processing time increased to 50ms
Error: analytics processing lag: 5000 events behind
```

**Performance Benchmarking:**
```bash
# Benchmark high-volume processing
go test -bench=BenchmarkHighVolume -benchtime=10s ./pkg/core/events/analytics/

# Profile CPU usage
go test -cpuprofile=analytics_cpu.prof -bench=. ./pkg/core/events/analytics/
go tool pprof analytics_cpu.prof
```

**Diagnostic Steps:**
1. Measure processing latency:
   ```go
   func measureProcessingLatency(engine *SimpleAnalyticsEngine) {
       events := make([]events.Event, 1000)
       for i := range events {
           events[i] = createTestEvent()
       }
       
       start := time.Now()
       for _, event := range events {
           _, err := engine.AnalyzeEvent(event)
           if err != nil {
               log.Printf("Analysis error: %v", err)
           }
       }
       duration := time.Since(start)
       
       avgLatency := duration / time.Duration(len(events))
       throughput := float64(len(events)) / duration.Seconds()
       
       log.Printf("Average latency: %v", avgLatency)
       log.Printf("Throughput: %.0f events/second", throughput)
   }
   ```

2. Identify bottlenecks:
   ```go
   // Profile individual operations
   start := time.Now()
   patterns := engine.GetPatterns()
   patternTime := time.Since(start)
   
   start = time.Now()
   recentEvents := engine.GetRecentEvents(5 * time.Minute)
   bufferTime := time.Since(start)
   
   start = time.Now()
   result, _ := engine.AnalyzeEvent(event)
   analysisTime := time.Since(start)
   
   log.Printf("Performance breakdown:")
   log.Printf("  Pattern retrieval: %v", patternTime)
   log.Printf("  Buffer access: %v", bufferTime)
   log.Printf("  Event analysis: %v", analysisTime)
   ```

**Solutions:**
- Implement asynchronous processing for non-critical analytics
- Use sampling for high-volume streams: analyze every Nth event
- Optimize data structures (use circular buffers, hash maps)
- Implement batch processing for multiple events
- Consider moving to external analytics systems (e.g., Elasticsearch, InfluxDB)
- Use worker pools for concurrent processing

### Concurrency and Thread Safety Issues

**Problem**: Race conditions or data corruption in concurrent scenarios
```
Error: concurrent map writes detected
panic: slice bounds out of range
```

**Diagnostic Commands:**
```bash
# Test for race conditions
go test -race ./pkg/core/events/analytics/

# Concurrent stress testing
go test -race -count=100 ./pkg/core/events/analytics/
```

**Testing Concurrent Access:**
```go
func testConcurrentAccess(engine *SimpleAnalyticsEngine) {
    var wg sync.WaitGroup
    numGoroutines := 100
    eventsPerGoroutine := 100
    
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func(routineID int) {
            defer wg.Done()
            
            for j := 0; j < eventsPerGoroutine; j++ {
                event := createTestEvent()
                _, err := engine.AnalyzeEvent(event)
                if err != nil {
                    log.Printf("Goroutine %d error: %v", routineID, err)
                }
                
                // Occasionally read patterns
                if j%10 == 0 {
                    patterns := engine.GetPatterns()
                    _ = patterns // Use patterns to avoid optimization
                }
            }
        }(i)
    }
    
    wg.Wait()
    log.Printf("Concurrent test completed successfully")
}
```

**Solutions:**
- Ensure proper mutex usage around shared data structures
- Use atomic operations for counters and simple values
- Implement read-write mutexes for read-heavy operations
- Use channels for communication between goroutines
- Consider lock-free data structures for high-performance scenarios

### Configuration and Integration Issues

#### Analytics Not Starting Properly

**Problem**: Analytics engine failing to initialize
```
Error: failed to create analytics engine: invalid configuration
panic: nil pointer dereference in analytics initialization
```

**Diagnostic Steps:**
1. Validate configuration:
   ```go
   config := &analytics.SimpleAnalyticsConfig{
       BufferSize:      1000,
       AnalysisWindow:  5 * time.Minute,
       MinPatternCount: 3,
   }
   
   // Validate configuration
   if config.BufferSize <= 0 {
       log.Printf("ERROR: Invalid buffer size: %d", config.BufferSize)
   }
   if config.AnalysisWindow <= 0 {
       log.Printf("ERROR: Invalid analysis window: %v", config.AnalysisWindow)
   }
   if config.MinPatternCount <= 0 {
       log.Printf("ERROR: Invalid min pattern count: %d", config.MinPatternCount)
   }
   ```

2. Test engine creation:
   ```go
   engine := analytics.NewSimpleAnalyticsEngine(config)
   if engine == nil {
       log.Printf("ERROR: Failed to create analytics engine")
       return
   }
   
   // Test basic functionality
   testEvent := createTestEvent()
   result, err := engine.AnalyzeEvent(testEvent)
   if err != nil {
       log.Printf("ERROR: Basic analysis failed: %v", err)
   } else {
       log.Printf("SUCCESS: Analytics engine working, result: %+v", result)
   }
   ```

**Solutions:**
- Use `analytics.DefaultSimpleAnalyticsConfig()` for safe defaults
- Validate all configuration parameters before engine creation
- Implement configuration validation in the constructor
- Add proper error handling and logging during initialization

### Performance Monitoring and Debugging

#### Real-time Performance Monitoring

```go
func monitorAnalyticsPerformance(engine *SimpleAnalyticsEngine) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    var lastMetrics *analytics.SimpleMetrics
    
    for range ticker.C {
        currentMetrics := engine.GetMetrics()
        
        log.Printf("Analytics Performance Report:")
        log.Printf("  Events processed: %d", currentMetrics.EventsProcessed)
        log.Printf("  Patterns detected: %d", currentMetrics.PatternsDetected)
        log.Printf("  Anomalies detected: %d", currentMetrics.AnomaliesDetected)
        
        if lastMetrics != nil {
            eventsPerSecond := float64(currentMetrics.EventsProcessed - lastMetrics.EventsProcessed) / 30.0
            log.Printf("  Processing rate: %.1f events/second", eventsPerSecond)
        }
        
        // Memory usage
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        log.Printf("  Memory usage: %d MB", m.Alloc/1024/1024)
        
        lastMetrics = currentMetrics
    }
}
```

#### Pattern Analysis and Debugging

```go
func analyzePatternDetection(engine *SimpleAnalyticsEngine) {
    patterns := engine.GetPatterns()
    
    log.Printf("Pattern Analysis Report:")
    log.Printf("  Total patterns: %d", len(patterns))
    
    if len(patterns) == 0 {
        log.Printf("  WARNING: No patterns detected")
        
        // Analyze why no patterns were found
        recentEvents := engine.GetRecentEvents(engine.GetConfig().AnalysisWindow)
        log.Printf("  Recent events count: %d", len(recentEvents))
        
        if len(recentEvents) < engine.GetConfig().MinPatternCount {
            log.Printf("  Insufficient events for pattern detection")
        }
        
        return
    }
    
    // Analyze pattern quality
    for patternID, pattern := range patterns {
        timeSinceLastSeen := time.Since(pattern.LastSeen)
        log.Printf("  Pattern %s:", patternID)
        log.Printf("    Count: %d", pattern.Count)
        log.Printf("    Last seen: %v ago", timeSinceLastSeen)
        log.Printf("    Event type: %s", pattern.EventType)
        
        if timeSinceLastSeen > pattern.Window {
            log.Printf("    WARNING: Pattern may be stale")
        }
    }
}
```

#### Event Processing Latency Analysis

```go
func analyzeProcessingLatency(engine *SimpleAnalyticsEngine) {
    const numSamples = 1000
    latencies := make([]time.Duration, numSamples)
    
    for i := 0; i < numSamples; i++ {
        event := createTestEvent()
        
        start := time.Now()
        _, err := engine.AnalyzeEvent(event)
        latency := time.Since(start)
        
        if err != nil {
            log.Printf("Error in sample %d: %v", i, err)
            continue
        }
        
        latencies[i] = latency
    }
    
    // Calculate statistics
    sort.Slice(latencies, func(i, j int) bool {
        return latencies[i] < latencies[j]
    })
    
    p50 := latencies[numSamples/2]
    p95 := latencies[int(float64(numSamples)*0.95)]
    p99 := latencies[int(float64(numSamples)*0.99)]
    
    var total time.Duration
    for _, latency := range latencies {
        total += latency
    }
    avg := total / time.Duration(numSamples)
    
    log.Printf("Processing Latency Analysis:")
    log.Printf("  Average: %v", avg)
    log.Printf("  P50: %v", p50)
    log.Printf("  P95: %v", p95)
    log.Printf("  P99: %v", p99)
    log.Printf("  Max: %v", latencies[numSamples-1])
    
    if avg > 10*time.Millisecond {
        log.Printf("  WARNING: High average latency")
    }
    if p99 > 100*time.Millisecond {
        log.Printf("  WARNING: High tail latency")
    }
}
```

### Advanced Debugging Techniques

#### Event Stream Analysis

```go
func analyzeEventStream(engine *SimpleAnalyticsEngine, duration time.Duration) {
    start := time.Now()
    eventCounts := make(map[events.EventType]int)
    var totalEvents int
    
    ticker := time.NewTicker(duration)
    defer ticker.Stop()
    
    <-ticker.C
    
    recentEvents := engine.GetRecentEvents(duration)
    for _, event := range recentEvents {
        eventCounts[event.GetEventType()]++
        totalEvents++
    }
    
    log.Printf("Event Stream Analysis (last %v):", duration)
    log.Printf("  Total events: %d", totalEvents)
    log.Printf("  Event types: %d", len(eventCounts))
    log.Printf("  Average rate: %.1f events/minute", 
        float64(totalEvents)/duration.Minutes())
    
    log.Printf("  Event distribution:")
    for eventType, count := range eventCounts {
        percentage := float64(count) / float64(totalEvents) * 100
        log.Printf("    %s: %d (%.1f%%)", eventType, count, percentage)
    }
}
```

## Contributing

To contribute to the analytics package:

1. Follow the existing code style and patterns
2. Add comprehensive tests for new features
3. Update documentation for API changes
4. Consider performance implications of changes
5. Ensure thread safety for concurrent operations

## License

This analytics package is part of the AG-UI Go SDK and follows the same licensing terms.