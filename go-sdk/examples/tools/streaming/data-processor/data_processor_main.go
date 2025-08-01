package dataprocessor

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ag-ui/go-sdk/pkg/tools"
)

func RunDataProcessorExample() error {
	// Create registry and register the data processor tool
	registry := tools.NewRegistry()
	dataProcessorTool := CreateDataProcessorTool()

	if err := registry.Register(dataProcessorTool); err != nil {
		return fmt.Errorf("failed to register data processor tool: %w", err)
	}

	// Create execution engine
	engine := tools.NewExecutionEngine(registry)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := engine.Shutdown(ctx); err != nil {
			log.Printf("Engine shutdown error: %v", err)
		}
	}()

	ctx := context.Background()

	fmt.Println("=== Real-time Data Processor Tool Example ===")
	fmt.Println("Demonstrates: Advanced streaming, statistical analysis, and real-time processing")
	fmt.Println()

	// Example 1: Generate sine wave data
	fmt.Println("1. Generating sine wave data with real-time statistics...")
	streamCh, err := engine.ExecuteStream(ctx, "data_processor", map[string]interface{}{
		"type":     "generate",
		"count":    20,
		"pattern":  "sine",
		"interval": 50,
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		consumeDataStream(streamCh, 30)
	}
	fmt.Println()

	// Example 2: Real-time aggregation
	fmt.Println("2. Performing real-time aggregation...")
	streamCh, err = engine.ExecuteStream(ctx, "data_processor", map[string]interface{}{
		"type":           "aggregate",
		"count":          15,
		"window_size":    5,
		"aggregate_type": "mean",
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		consumeDataStream(streamCh, 25)
	}
	fmt.Println()

	// Example 3: Data transformation
	fmt.Println("3. Applying square root transformation...")
	streamCh, err = engine.ExecuteStream(ctx, "data_processor", map[string]interface{}{
		"type":           "transform",
		"count":          10,
		"transformation": "sqrt",
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		consumeDataStream(streamCh, 20)
	}
	fmt.Println()

	// Example 4: Analyze provided data
	fmt.Println("4. Analyzing provided data points...")
	testData := []map[string]interface{}{
		{"value": 10.5, "timestamp": "2024-01-01T10:00:00Z"},
		{"value": 12.3, "timestamp": "2024-01-01T10:01:00Z"},
		{"value": 11.8, "timestamp": "2024-01-01T10:02:00Z"},
		{"value": 15.2, "timestamp": "2024-01-01T10:03:00Z"},
		{"value": 13.7, "timestamp": "2024-01-01T10:04:00Z"},
		{"value": 14.1, "timestamp": "2024-01-01T10:05:00Z"},
		{"value": 16.9, "timestamp": "2024-01-01T10:06:00Z"},
		{"value": 18.3, "timestamp": "2024-01-01T10:07:00Z"},
	}

	streamCh, err = engine.ExecuteStream(ctx, "data_processor", map[string]interface{}{
		"type": "analyze",
		"data": testData,
	})
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		consumeDataStream(streamCh, 15)
	}
	fmt.Println()
	
	return nil
}

// consumeDataStream consumes chunks from a data processing stream
func consumeDataStream(streamCh <-chan *tools.ToolStreamChunk, maxChunks int) {
	count := 0
	for chunk := range streamCh {
		if count >= maxChunks {
			break
		}

		switch chunk.Type {
		case "metadata":
			data := chunk.Data.(map[string]interface{})
			fmt.Printf("  [%d] Starting %s processing\n", chunk.Index, data["processing_type"])
		case "data":
			data := chunk.Data.(map[string]interface{})
			if point, exists := data["point"]; exists {
				pointData := point.(map[string]interface{})
				fmt.Printf("  [%d] Generated: %.2f\n", chunk.Index, pointData["value"])
			}
			if stats, exists := data["stats"]; exists {
				statsData := stats.(map[string]interface{})
				fmt.Printf("       Stats: Mean=%.2f, StdDev=%.2f\n", 
					statsData["mean"], statsData["std_dev"])
			}
		case "stats_update":
			data := chunk.Data.(map[string]interface{})
			fmt.Printf("  [%d] Statistics update: %d points processed\n", 
				chunk.Index, int(data["data_points"].(float64)))
		case "analysis_chunk":
			data := chunk.Data.(map[string]interface{})
			progress := data["progress"].(map[string]interface{})
			fmt.Printf("  [%d] Analyzed chunk: %.1f%% complete\n", 
				chunk.Index, progress["percent"])
		case "transformed_data":
			data := chunk.Data.(map[string]interface{})
			fmt.Printf("  [%d] Transformed: %.2f → %.2f\n", 
				chunk.Index, data["original"], data["transformed"])
		case "aggregated_data":
			data := chunk.Data.(map[string]interface{})
			fmt.Printf("  [%d] Current: %.2f, Aggregate: %.2f\n", 
				chunk.Index, data["current_value"], data["aggregate"])
		case "error":
			data := chunk.Data.(map[string]interface{})
			fmt.Printf("  [%d] Error: %v\n", chunk.Index, data["error"])
		case "complete":
			data := chunk.Data.(map[string]interface{})
			fmt.Printf("  [%d] Processing completed\n", chunk.Index)
			if insights, exists := data["insights"]; exists {
				insightsData := insights.(map[string]interface{})
				fmt.Printf("       Trend: %v\n", insightsData["trend"])
			}
		}

		count++
	}
}

// CreateDataProcessorTool creates a streaming data processor tool
func CreateDataProcessorTool() *tools.Tool {
	return &tools.Tool{
		ID:          "data_processor", 
		Name:        "Data Processor",
		Description: "A streaming data processor for real-time analytics and transformations",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"type": {
					Type:        "string",
					Description: "Type of data processing",
					Enum:        []interface{}{"generate", "aggregate", "transform", "analyze"},
				},
				"count": {
					Type:        "number",
					Description: "Number of data points to process",
					Default:     10,
				},
				"pattern": {
					Type:        "string", 
					Description: "Data generation pattern",
					Enum:        []interface{}{"sine", "random", "linear"},
					Default:     "random",
				},
				"window_size": {
					Type:        "number",
					Description: "Window size for aggregation",
					Default:     5,
				},
				"aggregate_type": {
					Type:        "string",
					Description: "Type of aggregation",
					Enum:        []interface{}{"mean", "sum", "max", "min"},
					Default:     "mean",
				},
				"transformation": {
					Type:        "string",
					Description: "Transformation to apply",
					Enum:        []interface{}{"sqrt", "square", "log", "abs"},
					Default:     "sqrt",
				},
				"data": {
					Type:        "array",
					Description: "Data points to analyze",
				},
				"interval": {
					Type:        "number",
					Description: "Interval between data points in ms",
					Default:     100,
				},
			},
			Required: []string{"type"},
		},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  true,
			Async:      true,
			Cancelable: true,
			Timeout:    30 * time.Second,
		},
		Metadata: &tools.ToolMetadata{
			Author:   "Streaming Team",
			License:  "MIT",
			Tags:     []string{"streaming", "data", "analytics", "real-time"},
		},
		// Note: In a real implementation, you would provide a proper StreamExecutor
		// For now, this is a placeholder to allow the example to compile
		Executor: &MockDataProcessorExecutor{},
	}
}

// MockDataProcessorExecutor is a placeholder for the actual streaming executor
type MockDataProcessorExecutor struct{}

func (e *MockDataProcessorExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	return &tools.ToolExecutionResult{
		Success: true,
		Data:    "Data processor executed successfully",
	}, nil
}