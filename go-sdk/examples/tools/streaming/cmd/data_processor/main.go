package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/ag-ui/go-sdk/pkg/tools"
)

// DataProcessorExecutor implements real-time data processing with streaming output.
// This example demonstrates complex data transformation, statistical analysis,
// and real-time aggregation using the streaming interface.
type DataProcessorExecutor struct {
	maxDataPoints int
	maxDuration   time.Duration
}

// NewDataProcessorExecutor creates a new data processor executor.
func NewDataProcessorExecutor(maxDataPoints int, maxDuration time.Duration) *DataProcessorExecutor {
	return &DataProcessorExecutor{
		maxDataPoints: maxDataPoints,
		maxDuration:   maxDuration,
	}
}

// DataPoint represents a single data point for processing
type DataPoint struct {
	Timestamp time.Time   `json:"timestamp"`
	Value     float64     `json:"value"`
	Source    string      `json:"source"`
	Metadata  interface{} `json:"metadata,omitempty"`
}

// ProcessingStats holds statistical information about processed data
type ProcessingStats struct {
	Count      int     `json:"count"`
	Sum        float64 `json:"sum"`
	Mean       float64 `json:"mean"`
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
	StdDev     float64 `json:"std_dev"`
	Median     float64 `json:"median"`
	Variance   float64 `json:"variance"`
	Range      float64 `json:"range"`
	UpdatedAt  string  `json:"updated_at"`
}

// Execute implements the regular ToolExecutor interface (not used for streaming).
func (d *DataProcessorExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	return &tools.ToolExecutionResult{
		Success: false,
		Error:   "this tool only supports streaming execution, use ExecuteStream instead",
	}, nil
}

// ExecuteStream implements the StreamingToolExecutor interface for real-time data processing.
func (d *DataProcessorExecutor) ExecuteStream(ctx context.Context, params map[string]interface{}) (<-chan *tools.ToolStreamChunk, error) {
	// Extract and validate parameters
	processingType, ok := params["type"].(string)
	if !ok {
		return nil, fmt.Errorf("type parameter must be a string")
	}

	// Create output channel
	outputCh := make(chan *tools.ToolStreamChunk, 200)

	// Start processing based on type
	switch processingType {
	case "generate":
		go d.generateAndProcess(ctx, params, outputCh)
	case "analyze":
		go d.analyzeExistingData(ctx, params, outputCh)
	case "transform":
		go d.transformData(ctx, params, outputCh)
	case "aggregate":
		go d.aggregateData(ctx, params, outputCh)
	default:
		close(outputCh)
		return nil, fmt.Errorf("unsupported processing type: %s", processingType)
	}

	return outputCh, nil
}

// generateAndProcess generates synthetic data and processes it in real-time
func (d *DataProcessorExecutor) generateAndProcess(ctx context.Context, params map[string]interface{}, outputCh chan<- *tools.ToolStreamChunk) {
	defer close(outputCh)

	chunkIndex := 0

	// Extract generation parameters
	count := 100 // Default
	if countParam, exists := params["count"]; exists {
		if countFloat, ok := countParam.(float64); ok {
			count = int(countFloat)
		}
	}

	if count > d.maxDataPoints {
		d.sendError(outputCh, fmt.Sprintf("count %d exceeds maximum allowed %d", count, d.maxDataPoints), &chunkIndex)
		return
	}

	interval := 100 * time.Millisecond // Default
	if intervalParam, exists := params["interval"]; exists {
		if intervalFloat, ok := intervalParam.(float64); ok {
			interval = time.Duration(intervalFloat) * time.Millisecond
		}
	}

	pattern := "random" // Default
	if patternParam, exists := params["pattern"]; exists {
		if patternStr, ok := patternParam.(string); ok {
			pattern = patternStr
		}
	}

	// Send initial metadata
	d.sendChunk(outputCh, "metadata", map[string]interface{}{
		"processing_type": "generate",
		"count":          count,
		"interval_ms":    interval.Milliseconds(),
		"pattern":        pattern,
		"started_at":     time.Now().Format(time.RFC3339),
	}, &chunkIndex)

	// Initialize statistics tracking
	var dataPoints []float64
	stats := &ProcessingStats{
		Min: math.Inf(1),
		Max: math.Inf(-1),
	}

	// Generate and process data points
	generator := d.createGenerator(pattern)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			d.sendError(outputCh, "processing cancelled", &chunkIndex)
			return
		case <-ticker.C:
			// Generate data point
			value := generator(i, float64(i))
			dataPoint := DataPoint{
				Timestamp: time.Now(),
				Value:     value,
				Source:    "generator",
				Metadata: map[string]interface{}{
					"index":   i,
					"pattern": pattern,
				},
			}

			// Update statistics
			dataPoints = append(dataPoints, value)
			d.updateStats(stats, dataPoints)

			// Send data point
			d.sendChunk(outputCh, "data", map[string]interface{}{
				"point": dataPoint,
				"stats": stats,
				"progress": map[string]interface{}{
					"current": i + 1,
					"total":   count,
					"percent": float64(i+1) / float64(count) * 100,
				},
			}, &chunkIndex)

			// Send periodic statistics updates
			if (i+1)%10 == 0 {
				d.sendChunk(outputCh, "stats_update", map[string]interface{}{
					"current_stats": stats,
					"data_points":   len(dataPoints),
					"trend":         d.calculateTrend(dataPoints),
				}, &chunkIndex)
			}
		}
	}

	// Send final results
	d.sendChunk(outputCh, "complete", map[string]interface{}{
		"final_stats":    stats,
		"total_points":   len(dataPoints),
		"processing_time": time.Now().Format(time.RFC3339),
		"summary": map[string]interface{}{
			"pattern":      pattern,
			"data_quality": d.assessDataQuality(dataPoints),
			"outliers":     d.detectOutliers(dataPoints),
		},
	}, &chunkIndex)
}

// analyzeExistingData analyzes provided data points
func (d *DataProcessorExecutor) analyzeExistingData(ctx context.Context, params map[string]interface{}, outputCh chan<- *tools.ToolStreamChunk) {
	defer close(outputCh)

	chunkIndex := 0

	// Extract data parameter
	dataParam, exists := params["data"]
	if !exists {
		d.sendError(outputCh, "data parameter is required for analysis", &chunkIndex)
		return
	}

	// Parse data points
	var dataPoints []DataPoint
	dataBytes, err := json.Marshal(dataParam)
	if err != nil {
		d.sendError(outputCh, fmt.Sprintf("failed to marshal data: %v", err), &chunkIndex)
		return
	}

	if err := json.Unmarshal(dataBytes, &dataPoints); err != nil {
		d.sendError(outputCh, fmt.Sprintf("failed to parse data points: %v", err), &chunkIndex)
		return
	}

	if len(dataPoints) > d.maxDataPoints {
		d.sendError(outputCh, fmt.Sprintf("data size %d exceeds maximum allowed %d", len(dataPoints), d.maxDataPoints), &chunkIndex)
		return
	}

	// Send metadata
	d.sendChunk(outputCh, "metadata", map[string]interface{}{
		"processing_type": "analyze",
		"data_points":     len(dataPoints),
		"started_at":      time.Now().Format(time.RFC3339),
	}, &chunkIndex)

	// Analyze data in chunks
	chunkSize := 50
	values := make([]float64, len(dataPoints))
	for i, dp := range dataPoints {
		values[i] = dp.Value
	}

	for i := 0; i < len(dataPoints); i += chunkSize {
		select {
		case <-ctx.Done():
			d.sendError(outputCh, "analysis cancelled", &chunkIndex)
			return
		default:
		}

		end := i + chunkSize
		if end > len(dataPoints) {
			end = len(dataPoints)
		}

		chunkValues := values[i:end]
		chunkStats := &ProcessingStats{}
		d.updateStats(chunkStats, chunkValues)

		d.sendChunk(outputCh, "analysis_chunk", map[string]interface{}{
			"chunk_range": map[string]int{"start": i, "end": end},
			"chunk_stats": chunkStats,
			"progress": map[string]interface{}{
				"processed": end,
				"total":     len(dataPoints),
				"percent":   float64(end) / float64(len(dataPoints)) * 100,
			},
		}, &chunkIndex)

		// Add processing delay to simulate real work
		time.Sleep(50 * time.Millisecond)
	}

	// Final analysis
	finalStats := &ProcessingStats{}
	d.updateStats(finalStats, values)

	d.sendChunk(outputCh, "complete", map[string]interface{}{
		"final_analysis": finalStats,
		"insights": map[string]interface{}{
			"trend":           d.calculateTrend(values),
			"data_quality":    d.assessDataQuality(values),
			"outliers":        d.detectOutliers(values),
			"distribution":    d.analyzeDistribution(values),
			"correlation":     d.calculateAutoCorrelation(values),
		},
	}, &chunkIndex)
}

// transformData applies transformations to streaming data
func (d *DataProcessorExecutor) transformData(ctx context.Context, params map[string]interface{}, outputCh chan<- *tools.ToolStreamChunk) {
	defer close(outputCh)

	chunkIndex := 0

	// Extract transformation parameters
	transformation, ok := params["transformation"].(string)
	if !ok {
		d.sendError(outputCh, "transformation parameter must be a string", &chunkIndex)
		return
	}

	count := 50 // Default
	if countParam, exists := params["count"]; exists {
		if countFloat, ok := countParam.(float64); ok {
			count = int(countFloat)
		}
	}

	// Send metadata
	d.sendChunk(outputCh, "metadata", map[string]interface{}{
		"processing_type": "transform",
		"transformation":  transformation,
		"count":           count,
		"started_at":      time.Now().Format(time.RFC3339),
	}, &chunkIndex)

	// Generate and transform data
	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			d.sendError(outputCh, "transformation cancelled", &chunkIndex)
			return
		default:
		}

		// Generate base value
		baseValue := rand.Float64() * 100

		// Apply transformation
		transformedValue, err := d.applyTransformation(baseValue, transformation)
		if err != nil {
			d.sendError(outputCh, fmt.Sprintf("transformation error: %v", err), &chunkIndex)
			return
		}

		d.sendChunk(outputCh, "transformed_data", map[string]interface{}{
			"original":     baseValue,
			"transformed":  transformedValue,
			"transformation": transformation,
			"index":        i,
			"timestamp":    time.Now().Format(time.RFC3339),
		}, &chunkIndex)

		time.Sleep(100 * time.Millisecond)
	}

	d.sendChunk(outputCh, "complete", map[string]interface{}{
		"transformation_complete": true,
		"total_processed":         count,
	}, &chunkIndex)
}

// aggregateData performs real-time aggregation on incoming data
func (d *DataProcessorExecutor) aggregateData(ctx context.Context, params map[string]interface{}, outputCh chan<- *tools.ToolStreamChunk) {
	defer close(outputCh)

	chunkIndex := 0

	// Extract aggregation parameters
	windowSize := 10 // Default
	if windowParam, exists := params["window_size"]; exists {
		if windowFloat, ok := windowParam.(float64); ok {
			windowSize = int(windowFloat)
		}
	}

	aggregateType := "mean" // Default
	if aggParam, exists := params["aggregate_type"]; exists {
		if aggStr, ok := aggParam.(string); ok {
			aggregateType = aggStr
		}
	}

	count := 100 // Default
	if countParam, exists := params["count"]; exists {
		if countFloat, ok := countParam.(float64); ok {
			count = int(countFloat)
		}
	}

	// Send metadata
	d.sendChunk(outputCh, "metadata", map[string]interface{}{
		"processing_type": "aggregate",
		"window_size":     windowSize,
		"aggregate_type":  aggregateType,
		"count":           count,
		"started_at":      time.Now().Format(time.RFC3339),
	}, &chunkIndex)

	// Initialize sliding window
	window := make([]float64, 0, windowSize)

	// Generate and aggregate data
	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			d.sendError(outputCh, "aggregation cancelled", &chunkIndex)
			return
		default:
		}

		// Generate new value
		value := rand.Float64()*50 + math.Sin(float64(i)*0.1)*25 + 50

		// Add to window
		window = append(window, value)
		if len(window) > windowSize {
			window = window[1:] // Remove oldest value
		}

		// Calculate aggregate
		aggregate := d.calculateAggregate(window, aggregateType)

		d.sendChunk(outputCh, "aggregated_data", map[string]interface{}{
			"current_value": value,
			"aggregate":     aggregate,
			"window_size":   len(window),
			"window_data":   window,
			"index":         i,
			"timestamp":     time.Now().Format(time.RFC3339),
		}, &chunkIndex)

		time.Sleep(100 * time.Millisecond)
	}

	d.sendChunk(outputCh, "complete", map[string]interface{}{
		"aggregation_complete": true,
		"final_window":         window,
		"final_aggregate":      d.calculateAggregate(window, aggregateType),
	}, &chunkIndex)
}

// Helper methods for data processing

func (d *DataProcessorExecutor) createGenerator(pattern string) func(int, float64) float64 {
	switch pattern {
	case "sine":
		return func(i int, x float64) float64 {
			return 50 + 30*math.Sin(x*0.1)
		}
	case "cosine":
		return func(i int, x float64) float64 {
			return 50 + 30*math.Cos(x*0.1)
		}
	case "linear":
		return func(i int, x float64) float64 {
			return x * 2
		}
	case "exponential":
		return func(i int, x float64) float64 {
			return math.Pow(1.1, x/10)
		}
	case "random_walk":
		last := 50.0
		return func(i int, x float64) float64 {
			change := (rand.Float64() - 0.5) * 10
			last += change
			return last
		}
	default: // "random"
		return func(i int, x float64) float64 {
			return rand.Float64() * 100
		}
	}
}

func (d *DataProcessorExecutor) updateStats(stats *ProcessingStats, values []float64) {
	if len(values) == 0 {
		return
	}

	stats.Count = len(values)
	stats.Sum = 0
	stats.Min = math.Inf(1)
	stats.Max = math.Inf(-1)

	// Calculate sum, min, max
	for _, v := range values {
		stats.Sum += v
		if v < stats.Min {
			stats.Min = v
		}
		if v > stats.Max {
			stats.Max = v
		}
	}

	stats.Mean = stats.Sum / float64(stats.Count)
	stats.Range = stats.Max - stats.Min

	// Calculate variance and standard deviation
	sumSquaredDiff := 0.0
	for _, v := range values {
		diff := v - stats.Mean
		sumSquaredDiff += diff * diff
	}
	stats.Variance = sumSquaredDiff / float64(stats.Count)
	stats.StdDev = math.Sqrt(stats.Variance)

	// Calculate median
	sortedValues := make([]float64, len(values))
	copy(sortedValues, values)
	sort.Float64s(sortedValues)
	
	if len(sortedValues)%2 == 0 {
		mid := len(sortedValues) / 2
		stats.Median = (sortedValues[mid-1] + sortedValues[mid]) / 2
	} else {
		stats.Median = sortedValues[len(sortedValues)/2]
	}

	stats.UpdatedAt = time.Now().Format(time.RFC3339)
}

func (d *DataProcessorExecutor) calculateTrend(values []float64) string {
	if len(values) < 2 {
		return "insufficient_data"
	}

	// Simple linear regression to determine trend
	n := float64(len(values))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0

	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)

	if slope > 0.1 {
		return "increasing"
	} else if slope < -0.1 {
		return "decreasing"
	} else {
		return "stable"
	}
}

func (d *DataProcessorExecutor) assessDataQuality(values []float64) map[string]interface{} {
	if len(values) == 0 {
		return map[string]interface{}{"quality": "no_data"}
	}

	// Calculate coefficient of variation
	stats := &ProcessingStats{}
	d.updateStats(stats, values)
	
	cv := stats.StdDev / stats.Mean * 100
	
	quality := "good"
	if cv > 50 {
		quality = "high_variance"
	} else if cv > 25 {
		quality = "moderate_variance"
	}

	return map[string]interface{}{
		"quality":                quality,
		"coefficient_variation":  cv,
		"data_completeness":      100.0, // Assuming no missing values
		"outlier_percentage":     float64(len(d.detectOutliers(values))) / float64(len(values)) * 100,
	}
}

func (d *DataProcessorExecutor) detectOutliers(values []float64) []int {
	if len(values) < 4 {
		return []int{}
	}

	stats := &ProcessingStats{}
	d.updateStats(stats, values)

	// Use z-score method: outliers are values with |z-score| > 2
	threshold := 2.0
	var outliers []int

	for i, value := range values {
		zScore := math.Abs((value - stats.Mean) / stats.StdDev)
		if zScore > threshold {
			outliers = append(outliers, i)
		}
	}

	return outliers
}

func (d *DataProcessorExecutor) analyzeDistribution(values []float64) map[string]interface{} {
	if len(values) == 0 {
		return map[string]interface{}{"distribution": "no_data"}
	}

	// Simple distribution analysis
	stats := &ProcessingStats{}
	d.updateStats(stats, values)

	// Calculate skewness (simplified)
	sumCubedDiff := 0.0
	for _, v := range values {
		diff := v - stats.Mean
		sumCubedDiff += diff * diff * diff
	}
	skewness := sumCubedDiff / (float64(len(values)) * math.Pow(stats.StdDev, 3))

	distribution := "normal"
	if math.Abs(skewness) > 1 {
		if skewness > 0 {
			distribution = "right_skewed"
		} else {
			distribution = "left_skewed"
		}
	} else if math.Abs(skewness) > 0.5 {
		distribution = "moderately_skewed"
	}

	return map[string]interface{}{
		"distribution": distribution,
		"skewness":     skewness,
		"kurtosis":     "not_calculated", // Would require more complex calculation
	}
}

func (d *DataProcessorExecutor) calculateAutoCorrelation(values []float64) float64 {
	if len(values) < 2 {
		return 0.0
	}

	// Simple lag-1 autocorrelation
	n := len(values) - 1
	meanX := 0.0
	meanY := 0.0

	for i := 0; i < n; i++ {
		meanX += values[i]
		meanY += values[i+1]
	}
	meanX /= float64(n)
	meanY /= float64(n)

	numerator := 0.0
	denomX := 0.0
	denomY := 0.0

	for i := 0; i < n; i++ {
		dx := values[i] - meanX
		dy := values[i+1] - meanY
		numerator += dx * dy
		denomX += dx * dx
		denomY += dy * dy
	}

	if denomX == 0 || denomY == 0 {
		return 0.0
	}

	return numerator / math.Sqrt(denomX*denomY)
}

func (d *DataProcessorExecutor) applyTransformation(value float64, transformation string) (float64, error) {
	switch transformation {
	case "log":
		if value <= 0 {
			return 0, fmt.Errorf("log transformation requires positive values")
		}
		return math.Log(value), nil
	case "sqrt":
		if value < 0 {
			return 0, fmt.Errorf("sqrt transformation requires non-negative values")
		}
		return math.Sqrt(value), nil
	case "square":
		return value * value, nil
	case "normalize":
		return (value - 50) / 25, nil // Normalize around mean=50, std=25
	case "reciprocal":
		if value == 0 {
			return 0, fmt.Errorf("reciprocal transformation cannot handle zero values")
		}
		return 1.0 / value, nil
	default:
		return value, nil
	}
}

func (d *DataProcessorExecutor) calculateAggregate(values []float64, aggregateType string) float64 {
	if len(values) == 0 {
		return 0.0
	}

	switch aggregateType {
	case "sum":
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum
	case "mean":
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values))
	case "min":
		min := values[0]
		for _, v := range values[1:] {
			if v < min {
				min = v
			}
		}
		return min
	case "max":
		max := values[0]
		for _, v := range values[1:] {
			if v > max {
				max = v
			}
		}
		return max
	default:
		return values[len(values)-1] // Return last value
	}
}

func (d *DataProcessorExecutor) sendChunk(outputCh chan<- *tools.ToolStreamChunk, chunkType string, data interface{}, chunkIndex *int) {
	chunk := &tools.ToolStreamChunk{
		Type:      chunkType,
		Data:      data,
		Index:     *chunkIndex,
		Timestamp: time.Now(),
	}

	// Implement proper backpressure handling
	timeout := time.NewTimer(time.Second * 5)
	defer timeout.Stop()
	
	select {
	case outputCh <- chunk:
		*chunkIndex++
	case <-timeout.C:
		// Create error chunk for dropped data
		errorChunk := &tools.ToolStreamChunk{
			Type:      "error",
			Data:      map[string]interface{}{
				"error": fmt.Sprintf("Data chunk %d dropped due to backpressure", *chunkIndex),
				"original_chunk_index": *chunkIndex,
				"dropped_data_type": chunkType,
			},
			Index:     *chunkIndex,
			Timestamp: time.Now(),
		}
		
		// Try to send error notification (with shorter timeout)
		select {
		case outputCh <- errorChunk:
			*chunkIndex++
		case <-time.After(time.Second):
			// If even error notification can't be sent, log to stdout
			fmt.Printf("Critical: failed to send chunk %d and error notification due to severe backpressure\n", *chunkIndex)
		}
	}
}

func (d *DataProcessorExecutor) sendError(outputCh chan<- *tools.ToolStreamChunk, errorMsg string, chunkIndex *int) {
	d.sendChunk(outputCh, "error", map[string]interface{}{
		"error": errorMsg,
		"fatal": true,
	}, chunkIndex)
}

// CreateDataProcessorTool creates and configures the data processor tool.
func CreateDataProcessorTool() *tools.Tool {
	maxDataPoints := 10000
	maxDuration := 5 * time.Minute

	return &tools.Tool{
		ID:          "data_processor",
		Name:        "Real-time Data Processor",
		Description: "Processes data in real-time with streaming output, supporting generation, analysis, transformation, and aggregation",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"type": {
					Type:        "string",
					Description: "Type of data processing to perform",
					Enum: []interface{}{
						"generate", "analyze", "transform", "aggregate",
					},
				},
				"count": {
					Type:        "number",
					Description: "Number of data points to process",
					Minimum:     &[]float64{1}[0],
					Maximum:     &[]float64{10000}[0],
					Default:     100,
				},
				"interval": {
					Type:        "number",
					Description: "Interval between data points in milliseconds",
					Minimum:     &[]float64{10}[0],
					Maximum:     &[]float64{5000}[0],
					Default:     100,
				},
				"pattern": {
					Type:        "string",
					Description: "Data generation pattern (for generate type)",
					Enum: []interface{}{
						"random", "sine", "cosine", "linear", "exponential", "random_walk",
					},
					Default: "random",
				},
				"transformation": {
					Type:        "string",
					Description: "Data transformation to apply (for transform type)",
					Enum: []interface{}{
						"log", "sqrt", "square", "normalize", "reciprocal",
					},
					Default: "normalize",
				},
				"window_size": {
					Type:        "number",
					Description: "Window size for aggregation (for aggregate type)",
					Minimum:     &[]float64{2}[0],
					Maximum:     &[]float64{100}[0],
					Default:     10,
				},
				"aggregate_type": {
					Type:        "string",
					Description: "Type of aggregation to perform (for aggregate type)",
					Enum: []interface{}{
						"mean", "sum", "min", "max",
					},
					Default: "mean",
				},
				"data": {
					Type:        "array",
					Description: "Existing data to analyze (for analyze type)",
					Items: &tools.Property{
						Type: "object",
						Properties: map[string]*tools.Property{
							"timestamp": {Type: "string"},
							"value":     {Type: "number"},
							"source":    {Type: "string"},
						},
						Required: []string{"value"},
					},
				},
			},
			Required: []string{"type"},
		},
		Metadata: &tools.ToolMetadata{
			Author:        "AG-UI SDK Examples",
			License:       "MIT",
			Documentation: "https://github.com/mattsp1290/ag-ui/blob/main/go-sdk/examples/tools/streaming/README.md",
			Tags:          []string{"streaming", "data-processing", "analytics", "real-time"},
			Examples: []tools.ToolExample{
				{
					Name:        "Generate Sine Wave Data",
					Description: "Generate and analyze a sine wave pattern",
					Input: map[string]interface{}{
						"type":     "generate",
						"count":    50,
						"pattern":  "sine",
						"interval": 200,
					},
				},
				{
					Name:        "Real-time Aggregation",
					Description: "Perform rolling aggregation on generated data",
					Input: map[string]interface{}{
						"type":           "aggregate",
						"count":          100,
						"window_size":    5,
						"aggregate_type": "mean",
					},
				},
				{
					Name:        "Data Transformation",
					Description: "Apply logarithmic transformation to data",
					Input: map[string]interface{}{
						"type":           "transform",
						"count":          30,
						"transformation": "log",
					},
				},
			},
		},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  true,
			Async:      false,
			Cancelable: true,
			Retryable:  false,
			Cacheable:  false,
			Timeout:    5 * time.Minute,
		},
		Executor: NewDataProcessorExecutor(maxDataPoints, maxDuration),
	}
}

