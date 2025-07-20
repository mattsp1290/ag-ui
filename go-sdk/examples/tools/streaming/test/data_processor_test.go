package test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ag-ui/go-sdk/pkg/tools"
)

// MockStreamingDataProcessor provides a testable version of the streaming data processor
type MockStreamingDataProcessor struct {
	mu              sync.RWMutex
	isStreaming     bool
	streamingCancel context.CancelFunc
	lastParams      map[string]interface{}
}

func NewMockStreamingDataProcessor() *MockStreamingDataProcessor {
	return &MockStreamingDataProcessor{
		lastParams: make(map[string]interface{}),
	}
}

// StreamingExecute implements the StreamingToolExecutor interface
func (s *MockStreamingDataProcessor) StreamingExecute(ctx context.Context, params map[string]interface{}, outputChan chan<- interface{}) error {
	s.mu.Lock()
	s.isStreaming = true
	s.lastParams = params
	
	// Create cancelable context for streaming
	streamCtx, cancel := context.WithCancel(ctx)
	s.streamingCancel = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isStreaming = false
		s.streamingCancel = nil
		s.mu.Unlock()
		close(outputChan)
	}()

	// Parse parameters with validation
	dataType := getStringParam(params, "data_type", "numeric")
	processingType := getStringParam(params, "processing_type", "realtime")
	batchSize := getIntParam(params, "batch_size", 10)
	interval := time.Duration(getIntParam(params, "interval_ms", 100)) * time.Millisecond
	duration := time.Duration(getIntParam(params, "duration_seconds", 10)) * time.Second
	enableStats := getBoolParam(params, "enable_statistics", true)

	// Validate and enforce minimum values
	if batchSize <= 0 {
		batchSize = 1 // Minimum batch size
	}
	if interval <= 0 {
		interval = 10 * time.Millisecond // Minimum interval
	}
	if duration <= 0 {
		duration = 1 * time.Second // Minimum duration
	}

	// Initialize statistics if enabled
	var stats *StreamingStats
	if enableStats {
		stats = NewStreamingStats()
	}

	// Start streaming data processing
	return s.streamData(streamCtx, outputChan, dataType, processingType, batchSize, interval, duration, stats)
}

// Execute implements the regular ToolExecutor interface for non-streaming mode
func (s *MockStreamingDataProcessor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	// For non-streaming mode, collect all data and return at once
	outputChan := make(chan interface{}, 1000)
	
	// Run streaming in background
	go func() {
		s.StreamingExecute(ctx, params, outputChan)
	}()

	// Collect all outputs
	var outputs []interface{}
	for output := range outputChan {
		outputs = append(outputs, output)
	}

	return &tools.ToolExecutionResult{
		Success:   true,
		Data:      outputs,
		Timestamp: time.Now(),
		Duration:  time.Second, // Estimated
		Metadata: map[string]interface{}{
			"mode":         "batch",
			"output_count": len(outputs),
			"parameters":   params,
		},
	}, nil
}

func (s *MockStreamingDataProcessor) streamData(
	ctx context.Context,
	outputChan chan<- interface{},
	dataType, processingType string,
	batchSize int,
	interval, duration time.Duration,
	stats *StreamingStats,
) error {
	startTime := time.Now()
	endTime := startTime.Add(duration)
	batchNumber := 0

	// Ensure at least one iteration happens
	atLeastOnce := true
	
	for atLeastOnce || time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		batchNumber++
		
		// Generate batch of data
		batch := s.generateDataBatch(dataType, batchSize, batchNumber)
		
		// Process batch based on processing type
		processed := s.processBatch(batch, processingType, stats)
		
		// Create output message
		output := StreamOutput{
			Timestamp:   time.Now(),
			BatchNumber: batchNumber,
			DataType:    dataType,
			ProcessingType: processingType,
			Data:        processed,
			Statistics:  nil, // Will be added if stats enabled
		}

		// Add statistics if enabled
		if stats != nil {
			stats.RecordBatch(len(batch), processed)
			output.Statistics = stats.GetSnapshot()
		}

		// Send output
		select {
		case outputChan <- output:
		case <-ctx.Done():
			return ctx.Err()
		}

		// Clear the at least once flag after first iteration
		atLeastOnce = false
		
		// Check if we should continue
		if !time.Now().Before(endTime) {
			break
		}

		// Wait for next interval
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Send final summary if stats enabled (or if no batches were sent)
	if stats != nil || batchNumber == 0 {
		summary := StreamingSummary{
			Timestamp:     time.Now(),
			TotalBatches:  batchNumber,
			TotalItems:    0,
			ProcessingTime: time.Since(startTime),
			FinalStats:    nil,
		}
		
		if stats != nil {
			summary.TotalItems = stats.TotalItems
			summary.FinalStats = stats.GetSnapshot()
		}

		select {
		case outputChan <- summary:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func (s *MockStreamingDataProcessor) generateDataBatch(dataType string, batchSize, batchNumber int) []interface{} {
	batch := make([]interface{}, batchSize)
	
	for i := 0; i < batchSize; i++ {
		switch dataType {
		case "numeric":
			batch[i] = DataPoint{
				ID:        fmt.Sprintf("batch_%d_item_%d", batchNumber, i),
				Value:     rand.Float64() * 100,
				Timestamp: time.Now(),
				Metadata:  map[string]interface{}{"batch": batchNumber, "index": i},
			}
		case "text":
			batch[i] = TextData{
				ID:      fmt.Sprintf("text_%d_%d", batchNumber, i),
				Text:    generateRandomText(),
				Length:  0, // Will be calculated
				Timestamp: time.Now(),
			}
		case "sensor":
			batch[i] = SensorReading{
				SensorID:    fmt.Sprintf("sensor_%d", (batchNumber+i)%5),
				Value:       rand.Float64()*50 + 20, // Temperature between 20-70
				Unit:        "celsius",
				Timestamp:   time.Now(),
				Quality:     rand.Float64(),
			}
		case "events":
			batch[i] = EventData{
				EventID:   fmt.Sprintf("event_%d_%d", batchNumber, i),
				EventType: []string{"login", "logout", "error", "warning", "info"}[rand.Intn(5)],
				Severity:  rand.Intn(5) + 1,
				Message:   fmt.Sprintf("Event message for batch %d item %d", batchNumber, i),
				Timestamp: time.Now(),
			}
		default:
			batch[i] = map[string]interface{}{
				"id":        fmt.Sprintf("item_%d_%d", batchNumber, i),
				"value":     rand.Intn(1000),
				"timestamp": time.Now(),
			}
		}
	}
	
	return batch
}

func (s *MockStreamingDataProcessor) processBatch(batch []interface{}, processingType string, stats *StreamingStats) interface{} {
	switch processingType {
	case "realtime":
		return s.processRealtime(batch, stats)
	case "aggregation":
		return s.processAggregation(batch, stats)
	case "filtering":
		return s.processFiltering(batch, stats)
	case "transformation":
		return s.processTransformation(batch, stats)
	case "analytics":
		return s.processAnalytics(batch, stats)
	default:
		return batch
	}
}

func (s *MockStreamingDataProcessor) processRealtime(batch []interface{}, stats *StreamingStats) interface{} {
	processed := make([]interface{}, len(batch))
	for i, item := range batch {
		switch data := item.(type) {
		case DataPoint:
			processed[i] = ProcessedData{
				OriginalID: data.ID,
				Value:      data.Value * 1.1, // Apply some transformation
				ProcessedAt: time.Now(),
				Processing: "realtime_multiplication",
			}
		case SensorReading:
			processed[i] = ProcessedSensor{
				SensorID:     data.SensorID,
				OriginalValue: data.Value,
				ProcessedValue: (data.Value * 9 / 5) + 32, // Celsius to Fahrenheit
				Unit:         "fahrenheit",
				ProcessedAt:  time.Now(),
			}
		default:
			processed[i] = map[string]interface{}{
				"original": item,
				"processed_at": time.Now(),
				"processing": "realtime_passthrough",
			}
		}
	}
	return processed
}

func (s *MockStreamingDataProcessor) processAggregation(batch []interface{}, stats *StreamingStats) interface{} {
	result := AggregationResult{
		Count:       len(batch),
		ProcessedAt: time.Now(),
		Type:        "aggregation",
	}

	var sum, min, max float64
	var values []float64
	
	for i, item := range batch {
		var value float64
		switch data := item.(type) {
		case DataPoint:
			value = data.Value
		case SensorReading:
			value = data.Value
		default:
			value = float64(i) // Fallback
		}
		
		values = append(values, value)
		sum += value
		
		if i == 0 || value < min {
			min = value
		}
		if i == 0 || value > max {
			max = value
		}
	}

	if len(values) > 0 {
		result.Sum = sum
		result.Average = sum / float64(len(values))
		result.Min = min
		result.Max = max
		result.Range = max - min
	}

	return result
}

func (s *MockStreamingDataProcessor) processFiltering(batch []interface{}, stats *StreamingStats) interface{} {
	var filtered []interface{}
	
	for _, item := range batch {
		include := false
		switch data := item.(type) {
		case DataPoint:
			include = data.Value > 50 // Filter values above 50
		case SensorReading:
			include = data.Quality > 0.8 // Filter high quality readings
		case EventData:
			include = data.Severity >= 3 // Filter important events
		default:
			include = true
		}
		
		if include {
			filtered = append(filtered, FilteredItem{
				Item:        item,
				FilteredAt:  time.Now(),
				FilterType:  "threshold",
			})
		}
	}

	return FilterResult{
		OriginalCount: len(batch),
		FilteredCount: len(filtered),
		FilteredItems: filtered,
		ProcessedAt:   time.Now(),
	}
}

func (s *MockStreamingDataProcessor) processTransformation(batch []interface{}, stats *StreamingStats) interface{} {
	transformed := make([]TransformedItem, len(batch))
	
	for i, item := range batch {
		transformed[i] = TransformedItem{
			ID:             fmt.Sprintf("transformed_%d", i),
			OriginalItem:   item,
			TransformedAt:  time.Now(),
			Transformations: []string{"normalize", "standardize"},
		}
		
		// Apply transformations based on data type
		switch data := item.(type) {
		case DataPoint:
			transformed[i].TransformedValue = (data.Value - 50) / 25 // Normalize around 0
		case SensorReading:
			transformed[i].TransformedValue = data.Value / 100 // Scale to 0-1
		default:
			transformed[i].TransformedValue = 0
		}
	}

	return TransformationResult{
		Items:       transformed,
		ProcessedAt: time.Now(),
		Method:      "standard_normalization",
	}
}

func (s *MockStreamingDataProcessor) processAnalytics(batch []interface{}, stats *StreamingStats) interface{} {
	analytics := AnalyticsResult{
		ProcessedAt: time.Now(),
		BatchSize:   len(batch),
		Metrics:     make(map[string]float64),
		Insights:    []string{},
	}

	// Calculate various analytics
	var values []float64
	typeCount := make(map[string]int)
	
	for _, item := range batch {
		itemType := fmt.Sprintf("%T", item)
		typeCount[itemType]++
		
		switch data := item.(type) {
		case DataPoint:
			values = append(values, data.Value)
		case SensorReading:
			values = append(values, data.Value)
		}
	}

	// Calculate statistics
	if len(values) > 0 {
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		mean := sum / float64(len(values))
		
		variance := 0.0
		for _, v := range values {
			variance += (v - mean) * (v - mean)
		}
		variance /= float64(len(values))
		
		analytics.Metrics["mean"] = mean
		analytics.Metrics["variance"] = variance
		analytics.Metrics["std_dev"] = variance // Simplified
		
		// Generate insights
		if mean > 75 {
			analytics.Insights = append(analytics.Insights, "High average values detected")
		}
		if variance > 100 {
			analytics.Insights = append(analytics.Insights, "High variance in data")
		}
	}

	analytics.TypeDistribution = typeCount
	return analytics
}

// Data structures for streaming

type DataPoint struct {
	ID        string                 `json:"id"`
	Value     float64                `json:"value"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

type TextData struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Length    int       `json:"length"`
	Timestamp time.Time `json:"timestamp"`
}

type SensorReading struct {
	SensorID  string    `json:"sensor_id"`
	Value     float64   `json:"value"`
	Unit      string    `json:"unit"`
	Timestamp time.Time `json:"timestamp"`
	Quality   float64   `json:"quality"`
}

type EventData struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Severity  int       `json:"severity"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type StreamOutput struct {
	Timestamp      time.Time    `json:"timestamp"`
	BatchNumber    int          `json:"batch_number"`
	DataType       string       `json:"data_type"`
	ProcessingType string       `json:"processing_type"`
	Data           interface{}  `json:"data"`
	Statistics     *StatsSnapshot `json:"statistics,omitempty"`
}

type StreamingSummary struct {
	Timestamp      time.Time      `json:"timestamp"`
	TotalBatches   int           `json:"total_batches"`
	TotalItems     int64         `json:"total_items"`
	ProcessingTime time.Duration `json:"processing_time"`
	FinalStats     *StatsSnapshot `json:"final_stats"`
}

type ProcessedData struct {
	OriginalID  string    `json:"original_id"`
	Value       float64   `json:"value"`
	ProcessedAt time.Time `json:"processed_at"`
	Processing  string    `json:"processing"`
}

type ProcessedSensor struct {
	SensorID       string    `json:"sensor_id"`
	OriginalValue  float64   `json:"original_value"`
	ProcessedValue float64   `json:"processed_value"`
	Unit           string    `json:"unit"`
	ProcessedAt    time.Time `json:"processed_at"`
}

type AggregationResult struct {
	Count       int       `json:"count"`
	Sum         float64   `json:"sum"`
	Average     float64   `json:"average"`
	Min         float64   `json:"min"`
	Max         float64   `json:"max"`
	Range       float64   `json:"range"`
	ProcessedAt time.Time `json:"processed_at"`
	Type        string    `json:"type"`
}

type FilteredItem struct {
	Item       interface{} `json:"item"`
	FilteredAt time.Time   `json:"filtered_at"`
	FilterType string      `json:"filter_type"`
}

type FilterResult struct {
	OriginalCount int            `json:"original_count"`
	FilteredCount int            `json:"filtered_count"`
	FilteredItems []interface{}  `json:"filtered_items"`
	ProcessedAt   time.Time      `json:"processed_at"`
}

type TransformedItem struct {
	ID               string      `json:"id"`
	OriginalItem     interface{} `json:"original_item"`
	TransformedValue float64     `json:"transformed_value"`
	TransformedAt    time.Time   `json:"transformed_at"`
	Transformations  []string    `json:"transformations"`
}

type TransformationResult struct {
	Items       []TransformedItem `json:"items"`
	ProcessedAt time.Time         `json:"processed_at"`
	Method      string            `json:"method"`
}

type AnalyticsResult struct {
	ProcessedAt      time.Time          `json:"processed_at"`
	BatchSize        int                `json:"batch_size"`
	Metrics          map[string]float64 `json:"metrics"`
	TypeDistribution map[string]int     `json:"type_distribution"`
	Insights         []string           `json:"insights"`
}

// Statistics tracking
type StreamingStats struct {
	mu             sync.RWMutex
	StartTime      time.Time `json:"start_time"`
	TotalBatches   int64     `json:"total_batches"`
	TotalItems     int64     `json:"total_items"`
	ProcessingTime time.Duration `json:"processing_time"`
	LastBatchTime  time.Time `json:"last_batch_time"`
	ItemsPerSecond float64   `json:"items_per_second"`
	BytesProcessed int64     `json:"bytes_processed"`
}

type StatsSnapshot struct {
	Timestamp      time.Time     `json:"timestamp"`
	TotalBatches   int64         `json:"total_batches"`
	TotalItems     int64         `json:"total_items"`
	ProcessingTime time.Duration `json:"processing_time"`
	ItemsPerSecond float64       `json:"items_per_second"`
	BytesProcessed int64         `json:"bytes_processed"`
	Uptime         time.Duration `json:"uptime"`
}

func NewStreamingStats() *StreamingStats {
	return &StreamingStats{
		StartTime:     time.Now(),
		LastBatchTime: time.Now(),
	}
}

func (s *StreamingStats) RecordBatch(itemCount int, data interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.TotalBatches++
	s.TotalItems += int64(itemCount)
	s.LastBatchTime = time.Now()
	
	// Calculate throughput
	elapsed := time.Since(s.StartTime)
	if elapsed > 0 {
		s.ItemsPerSecond = float64(s.TotalItems) / elapsed.Seconds()
	}
	
	// Estimate bytes processed
	if jsonData, err := json.Marshal(data); err == nil {
		s.BytesProcessed += int64(len(jsonData))
	}
}

func (s *StreamingStats) GetSnapshot() *StatsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return &StatsSnapshot{
		Timestamp:      time.Now(),
		TotalBatches:   s.TotalBatches,
		TotalItems:     s.TotalItems,
		ProcessingTime: s.ProcessingTime,
		ItemsPerSecond: s.ItemsPerSecond,
		BytesProcessed: s.BytesProcessed,
		Uptime:         time.Since(s.StartTime),
	}
}

// Helper functions
func getStringParam(params map[string]interface{}, key, defaultValue string) string {
	if val, ok := params[key].(string); ok {
		return val
	}
	return defaultValue
}

func getIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if val, ok := params[key].(int); ok {
		return val
	}
	if val, ok := params[key].(float64); ok {
		return int(val)
	}
	return defaultValue
}

func getBoolParam(params map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := params[key].(bool); ok {
		return val
	}
	return defaultValue
}

func generateRandomText() string {
	words := []string{"stream", "data", "processing", "realtime", "analytics", "batch", "transform", "filter", "aggregate"}
	length := rand.Intn(5) + 3
	text := make([]string, length)
	for i := 0; i < length; i++ {
		text[i] = words[rand.Intn(len(words))]
	}
	return strings.Join(text, " ")
}

// Tool creation
func createStreamingDataProcessorTool() *tools.Tool {
	return &tools.Tool{
		ID:          "streaming-data-processor",
		Name:        "StreamingDataProcessor",
		Description: "Real-time data processing with streaming output and analytics",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"data_type": {
					Type:        "string",
					Description: "Type of data to process",
					Enum:        []interface{}{"numeric", "text", "sensor", "events"},
					Default:     "numeric",
				},
				"processing_type": {
					Type:        "string",
					Description: "Type of processing to apply",
					Enum:        []interface{}{"realtime", "aggregation", "filtering", "transformation", "analytics"},
					Default:     "realtime",
				},
				"batch_size": {
					Type:        "integer",
					Description: "Number of items per batch",
					Default:     10,
					Minimum:     func() *float64 { v := 1.0; return &v }(),
					Maximum:     func() *float64 { v := 1000.0; return &v }(),
				},
				"interval_ms": {
					Type:        "integer",
					Description: "Interval between batches in milliseconds",
					Default:     100,
					Minimum:     func() *float64 { v := 10.0; return &v }(),
					Maximum:     func() *float64 { v := 10000.0; return &v }(),
				},
				"duration_seconds": {
					Type:        "integer",
					Description: "How long to stream data",
					Default:     10,
					Minimum:     func() *float64 { v := 1.0; return &v }(),
					Maximum:     func() *float64 { v := 300.0; return &v }(),
				},
				"enable_statistics": {
					Type:        "boolean",
					Description: "Enable statistics collection",
					Default:     true,
				},
			},
			Required: []string{},
		},
		Executor: NewMockStreamingDataProcessor(),
		Capabilities: &tools.ToolCapabilities{
			Streaming:  true,
			Async:      true,
			Cancelable: true,
			Cacheable:  false,
			Timeout:    10 * time.Minute,
		},
		Metadata: &tools.ToolMetadata{
			Author:   "Streaming Team",
			License:  "MIT",
			Tags:     []string{"streaming", "data", "processing", "realtime"},
			Examples: []tools.ToolExample{
				{
					Name:        "Numeric Data Stream",
					Description: "Process numeric data in real-time",
					Input: map[string]interface{}{
						"data_type":        "numeric",
						"processing_type":  "realtime",
						"batch_size":       5,
						"interval_ms":      500,
						"duration_seconds": 5,
					},
				},
				{
					Name:        "Sensor Aggregation",
					Description: "Aggregate sensor readings",
					Input: map[string]interface{}{
						"data_type":        "sensor",
						"processing_type":  "aggregation",
						"batch_size":       20,
						"interval_ms":      1000,
					},
				},
			},
		},
	}
}

// Tests

// TestStreamingDataProcessor_BasicStreaming tests basic streaming functionality
func TestStreamingDataProcessor_BasicStreaming(t *testing.T) {
	tool := createStreamingDataProcessorTool()
	processor := tool.Executor.(*MockStreamingDataProcessor)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	params := map[string]interface{}{
		"data_type":        "numeric",
		"processing_type":  "realtime",
		"batch_size":       5,
		"interval_ms":      100,
		"duration_seconds": 2,
		"enable_statistics": true,
	}

	outputChan := make(chan interface{}, 100)
	
	// Start streaming
	go func() {
		err := processor.StreamingExecute(ctx, params, outputChan)
		assert.NoError(t, err)
	}()

	// Collect outputs
	var outputs []interface{}
	timeout := time.After(3 * time.Second)
	
	for {
		select {
		case output, ok := <-outputChan:
			if !ok {
				// Channel closed, streaming finished
				goto done
			}
			outputs = append(outputs, output)
		case <-timeout:
			t.Fatal("Test timed out")
		}
	}
	
done:
	// Verify we received outputs
	assert.NotEmpty(t, outputs)
	
	// Check output types
	hasStreamOutput := false
	hasSummary := false
	
	for _, output := range outputs {
		switch output.(type) {
		case StreamOutput:
			hasStreamOutput = true
		case StreamingSummary:
			hasSummary = true
		}
	}
	
	assert.True(t, hasStreamOutput, "Should have stream outputs")
	assert.True(t, hasSummary, "Should have summary")
}

// TestStreamingDataProcessor_DataTypes tests different data types
func TestStreamingDataProcessor_DataTypes(t *testing.T) {
	tool := createStreamingDataProcessorTool()
	processor := tool.Executor.(*MockStreamingDataProcessor)

	dataTypes := []string{"numeric", "text", "sensor", "events"}
	
	for _, dataType := range dataTypes {
		t.Run("DataType_"+dataType, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			params := map[string]interface{}{
				"data_type":        dataType,
				"processing_type":  "realtime",
				"batch_size":       3,
				"interval_ms":      200,
				"duration_seconds": 1,
			}

			outputChan := make(chan interface{}, 50)
			
			err := processor.StreamingExecute(ctx, params, outputChan)
			assert.NoError(t, err)

			// Verify we got outputs
			var outputs []interface{}
			for output := range outputChan {
				outputs = append(outputs, output)
			}
			
			assert.NotEmpty(t, outputs)
			
			// Check first stream output
			if len(outputs) > 0 {
				if streamOutput, ok := outputs[0].(StreamOutput); ok {
					assert.Equal(t, dataType, streamOutput.DataType)
					assert.NotNil(t, streamOutput.Data)
				}
			}
		})
	}
}

// TestStreamingDataProcessor_ProcessingTypes tests different processing types
func TestStreamingDataProcessor_ProcessingTypes(t *testing.T) {
	tool := createStreamingDataProcessorTool()
	processor := tool.Executor.(*MockStreamingDataProcessor)

	processingTypes := []string{"realtime", "aggregation", "filtering", "transformation", "analytics"}
	
	for _, processingType := range processingTypes {
		t.Run("ProcessingType_"+processingType, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			params := map[string]interface{}{
				"data_type":        "numeric",
				"processing_type":  processingType,
				"batch_size":       5,
				"interval_ms":      200,
				"duration_seconds": 1,
			}

			outputChan := make(chan interface{}, 50)
			
			err := processor.StreamingExecute(ctx, params, outputChan)
			assert.NoError(t, err)

			// Verify we got outputs
			var outputs []interface{}
			for output := range outputChan {
				outputs = append(outputs, output)
			}
			
			assert.NotEmpty(t, outputs)
			
			// Check processing type in output
			if len(outputs) > 0 {
				if streamOutput, ok := outputs[0].(StreamOutput); ok {
					assert.Equal(t, processingType, streamOutput.ProcessingType)
				}
			}
		})
	}
}

// TestStreamingDataProcessor_Statistics tests statistics collection
func TestStreamingDataProcessor_Statistics(t *testing.T) {
	tool := createStreamingDataProcessorTool()
	processor := tool.Executor.(*MockStreamingDataProcessor)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	params := map[string]interface{}{
		"data_type":         "numeric",
		"processing_type":   "realtime",
		"batch_size":        5,
		"interval_ms":       100,
		"duration_seconds":  2,
		"enable_statistics": true,
	}

	outputChan := make(chan interface{}, 100)
	
	err := processor.StreamingExecute(ctx, params, outputChan)
	assert.NoError(t, err)

	// Collect outputs and check for statistics
	var outputs []interface{}
	for output := range outputChan {
		outputs = append(outputs, output)
	}
	
	assert.NotEmpty(t, outputs)
	
	// Check that stream outputs have statistics
	hasStats := false
	for _, output := range outputs {
		if streamOutput, ok := output.(StreamOutput); ok {
			if streamOutput.Statistics != nil {
				hasStats = true
				assert.Greater(t, streamOutput.Statistics.TotalItems, int64(0))
				assert.Greater(t, streamOutput.Statistics.TotalBatches, int64(0))
				break
			}
		}
	}
	
	assert.True(t, hasStats, "Should have statistics in outputs")
	
	// Check for summary with final stats
	hasFinalStats := false
	for _, output := range outputs {
		if summary, ok := output.(StreamingSummary); ok {
			assert.NotNil(t, summary.FinalStats)
			assert.Greater(t, summary.TotalBatches, 0)
			assert.Greater(t, summary.TotalItems, int64(0))
			hasFinalStats = true
			break
		}
	}
	
	assert.True(t, hasFinalStats, "Should have final statistics")
}

// TestStreamingDataProcessor_Cancellation tests streaming cancellation
func TestStreamingDataProcessor_Cancellation(t *testing.T) {
	tool := createStreamingDataProcessorTool()
	processor := tool.Executor.(*MockStreamingDataProcessor)
	ctx, cancel := context.WithCancel(context.Background())

	params := map[string]interface{}{
		"data_type":        "numeric",
		"processing_type":  "realtime",
		"batch_size":       5,
		"interval_ms":      100,
		"duration_seconds": 60, // Long duration
	}

	outputChan := make(chan interface{}, 100)
	errChan := make(chan error, 1)
	
	// Start streaming
	go func() {
		err := processor.StreamingExecute(ctx, params, outputChan)
		errChan <- err
	}()

	// Let it run for a bit
	time.Sleep(300 * time.Millisecond)
	
	// Cancel the context
	cancel()

	// Wait for completion
	select {
	case err := <-errChan:
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Streaming didn't stop after cancellation")
	}

	// Verify channel is closed
	select {
	case _, ok := <-outputChan:
		if ok {
			// Drain remaining outputs
			for range outputChan {
			}
		}
	default:
	}
}

// TestStreamingDataProcessor_NonStreamingMode tests regular execution mode
func TestStreamingDataProcessor_NonStreamingMode(t *testing.T) {
	tool := createStreamingDataProcessorTool()
	processor := tool.Executor.(*MockStreamingDataProcessor)
	ctx := context.Background()

	params := map[string]interface{}{
		"data_type":        "numeric",
		"processing_type":  "aggregation",
		"batch_size":       3,
		"interval_ms":      100,
		"duration_seconds": 1,
	}

	result, err := processor.Execute(ctx, params)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	
	// Check result structure
	outputs, ok := result.Data.([]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, outputs)
	
	// Check metadata
	assert.NotNil(t, result.Metadata)
	assert.Equal(t, "batch", result.Metadata["mode"])
	assert.Equal(t, len(outputs), result.Metadata["output_count"])
}

// TestStreamingDataProcessor_ParameterValidation tests parameter validation
func TestStreamingDataProcessor_ParameterValidation(t *testing.T) {
	tool := createStreamingDataProcessorTool()
	processor := tool.Executor.(*MockStreamingDataProcessor)

	testCases := []struct {
		name   string
		params map[string]interface{}
	}{
		{
			name: "Default parameters",
			params: map[string]interface{}{
				"duration_seconds": 1, // Override default to fit within test timeout
			},
		},
		{
			name: "All parameters specified",
			params: map[string]interface{}{
				"data_type":         "sensor",
				"processing_type":   "analytics",
				"batch_size":        5,
				"interval_ms":       100,
				"duration_seconds":  1,
				"enable_statistics": false,
			},
		},
		{
			name: "Edge case values",
			params: map[string]interface{}{
				"batch_size":       1,
				"interval_ms":      10,
				"duration_seconds": 1,
			},
		},
		{
			name: "Zero duration edge case",
			params: map[string]interface{}{
				"batch_size":       1,
				"interval_ms":      10,
				"duration_seconds": 0, // Should be normalized to 1 second minimum
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh context for each test case
			timeout := 3 * time.Second // Give extra time for processing
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			
			outputChan := make(chan interface{}, 100)
			
			// Run streaming in a goroutine
			errChan := make(chan error, 1)
			go func() {
				err := processor.StreamingExecute(ctx, tc.params, outputChan)
				errChan <- err
			}()

			// Collect outputs with timeout
			var outputCount int
			done := make(chan bool)
			
			go func() {
				for range outputChan {
					outputCount++
				}
				done <- true
			}()
			
			// Wait for completion or timeout
			select {
			case <-done:
				// Normal completion
			case <-time.After(timeout):
				t.Fatal("Test timed out waiting for outputs")
			}
			
			// Check for errors
			select {
			case err := <-errChan:
				assert.NoError(t, err)
			default:
				// No error yet, which is fine
			}
			
			// Should get at least one output (even if just summary)
			assert.Greater(t, outputCount, 0, "Expected at least one output but got none")
		})
	}
}

// TestStreamingDataProcessor_Performance tests performance characteristics
func TestStreamingDataProcessor_Performance(t *testing.T) {
	tool := createStreamingDataProcessorTool()
	processor := tool.Executor.(*MockStreamingDataProcessor)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	params := map[string]interface{}{
		"data_type":         "numeric",
		"processing_type":   "realtime",
		"batch_size":        50,
		"interval_ms":       10, // High frequency
		"duration_seconds":  2,
		"enable_statistics": true,
	}

	outputChan := make(chan interface{}, 1000)
	start := time.Now()
	
	err := processor.StreamingExecute(ctx, params, outputChan)
	assert.NoError(t, err)

	// Collect outputs and measure performance
	var outputs []interface{}
	for output := range outputChan {
		outputs = append(outputs, output)
	}
	
	duration := time.Since(start)
	
	// Performance assertions
	assert.NotEmpty(t, outputs)
	assert.Less(t, duration, 3*time.Second) // Should complete reasonably quickly
	
	// Check throughput from final statistics
	for _, output := range outputs {
		if summary, ok := output.(StreamingSummary); ok {
			if summary.FinalStats != nil {
				assert.Greater(t, summary.FinalStats.ItemsPerSecond, 0.0)
				t.Logf("Processed %.2f items/second", summary.FinalStats.ItemsPerSecond)
			}
			break
		}
	}
}

// TestStreamingDataProcessor_ConcurrentStreaming tests concurrent streaming
func TestStreamingDataProcessor_ConcurrentStreaming(t *testing.T) {
	_ = createStreamingDataProcessorTool() // tool not used in this test
	
	const numStreams = 3
	var wg sync.WaitGroup
	results := make(chan error, numStreams)

	// Start multiple concurrent streams
	for i := 0; i < numStreams; i++ {
		wg.Add(1)
		go func(streamID int) {
			defer wg.Done()
			
			processor := NewMockStreamingDataProcessor() // New instance for each stream
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			params := map[string]interface{}{
				"data_type":        []string{"numeric", "sensor", "events"}[streamID%3],
				"processing_type":  "realtime",
				"batch_size":       5,
				"interval_ms":      200,
				"duration_seconds": 1,
			}

			outputChan := make(chan interface{}, 100)
			
			err := processor.StreamingExecute(ctx, params, outputChan)
			
			// Drain the channel
			for range outputChan {
			}
			
			results <- err
		}(i)
	}

	wg.Wait()
	close(results)

	// Check all streams completed successfully
	var errors []error
	for err := range results {
		if err != nil {
			errors = append(errors, err)
		}
	}

	assert.Empty(t, errors, "All concurrent streams should complete successfully")
}

// TestStreamingDataProcessor_Schema tests schema validation
func TestStreamingDataProcessor_Schema(t *testing.T) {
	tool := createStreamingDataProcessorTool()

	// Test schema structure
	assert.NotNil(t, tool.Schema)
	assert.Equal(t, "object", tool.Schema.Type)
	assert.Contains(t, tool.Schema.Properties, "data_type")
	assert.Contains(t, tool.Schema.Properties, "processing_type")
	assert.Contains(t, tool.Schema.Properties, "batch_size")

	// Test enum values
	dataTypeProp := tool.Schema.Properties["data_type"]
	assert.NotNil(t, dataTypeProp.Enum)
	assert.Contains(t, dataTypeProp.Enum, "numeric")
	assert.Contains(t, dataTypeProp.Enum, "sensor")

	processingTypeProp := tool.Schema.Properties["processing_type"]
	assert.NotNil(t, processingTypeProp.Enum)
	assert.Contains(t, processingTypeProp.Enum, "realtime")
	assert.Contains(t, processingTypeProp.Enum, "aggregation")
}

// TestStreamingDataProcessor_Capabilities tests tool capabilities
func TestStreamingDataProcessor_Capabilities(t *testing.T) {
	tool := createStreamingDataProcessorTool()

	assert.NotNil(t, tool.Capabilities)
	assert.True(t, tool.Capabilities.Streaming)
	assert.True(t, tool.Capabilities.Async)
	assert.True(t, tool.Capabilities.Cancelable)
	assert.False(t, tool.Capabilities.Cacheable)
	assert.Equal(t, 10*time.Minute, tool.Capabilities.Timeout)
}

// BenchmarkStreamingDataProcessor benchmarks streaming performance
func BenchmarkStreamingDataProcessor(b *testing.B) {
	_ = createStreamingDataProcessorTool() // tool not used in this benchmark
	ctx := context.Background()

	b.Run("HighFrequencyStreaming", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			processor := NewMockStreamingDataProcessor()
			params := map[string]interface{}{
				"data_type":         "numeric",
				"processing_type":   "realtime",
				"batch_size":        10,
				"interval_ms":       1,
				"duration_seconds":  1,
				"enable_statistics": false,
			}

			outputChan := make(chan interface{}, 1000)
			
			err := processor.StreamingExecute(ctx, params, outputChan)
			if err != nil {
				b.Fatal(err)
			}

			// Drain channel
			for range outputChan {
			}
		}
	})

	b.Run("LargeBatchProcessing", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			processor := NewMockStreamingDataProcessor()
			params := map[string]interface{}{
				"data_type":         "numeric",
				"processing_type":   "aggregation",
				"batch_size":        1000,
				"interval_ms":       100,
				"duration_seconds":  1,
				"enable_statistics": false,
			}

			outputChan := make(chan interface{}, 100)
			
			err := processor.StreamingExecute(ctx, params, outputChan)
			if err != nil {
				b.Fatal(err)
			}

			// Drain channel
			for range outputChan {
			}
		}
	})
}

// Example tests - this is just an example function, not attached to a type
func Example_streamingDataProcessor_BasicUsage() {
	tool := createStreamingDataProcessorTool()
	processor := tool.Executor.(*MockStreamingDataProcessor)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	params := map[string]interface{}{
		"data_type":        "numeric",
		"processing_type":  "realtime",
		"batch_size":       3,
		"interval_ms":      500,
		"duration_seconds": 1,
	}

	outputChan := make(chan interface{}, 50)
	
	// Start streaming in background
	go func() {
		processor.StreamingExecute(ctx, params, outputChan)
	}()

	// Process outputs
	for output := range outputChan {
		switch data := output.(type) {
		case StreamOutput:
			fmt.Printf("Batch %d: %d items processed\n", data.BatchNumber, len(data.Data.([]interface{})))
		case StreamingSummary:
			fmt.Printf("Summary: %d total batches, %d total items\n", data.TotalBatches, data.TotalItems)
		}
	}

	// Output: 
	// Batch 1: 3 items processed
	// Batch 2: 3 items processed
	// Summary: 2 total batches, 6 total items
}
