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

## Contributing

To contribute to the analytics package:

1. Follow the existing code style and patterns
2. Add comprehensive tests for new features
3. Update documentation for API changes
4. Consider performance implications of changes
5. Ensure thread safety for concurrent operations

## License

This analytics package is part of the AG-UI Go SDK and follows the same licensing terms.