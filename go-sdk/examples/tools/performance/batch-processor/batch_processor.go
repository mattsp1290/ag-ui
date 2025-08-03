package batchprocessor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
)

// BatchProcessorTool demonstrates batch processing with performance optimization
type BatchProcessorTool struct {
	batches     map[string]*Batch
	batchMutex  sync.RWMutex
	processors  map[string]*Processor
	processMutex sync.RWMutex
}

// Batch represents a batch of items to process
type Batch struct {
	ID          string        `json:"id"`
	Items       []interface{} `json:"items"`
	Status      string        `json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	Results     []interface{} `json:"results"`
	Errors      []string      `json:"errors"`
	ProcessorID string        `json:"processor_id"`
}

// Processor represents a batch processor
type Processor struct {
	ID           string        `json:"id"`
	Type         string        `json:"type"`
	BatchSize    int           `json:"batch_size"`
	Concurrency  int           `json:"concurrency"`
	ProcessingFn func(interface{}) (interface{}, error)
}

// NewBatchProcessorTool creates a new batch processor tool
func NewBatchProcessorTool() *BatchProcessorTool {
	tool := &BatchProcessorTool{
		batches:    make(map[string]*Batch),
		processors: make(map[string]*Processor),
	}

	// Register default processors
	tool.registerDefaultProcessors()
	return tool
}

// Execute handles batch processor operations
func (b *BatchProcessorTool) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	operation, ok := params["operation"].(string)
	if !ok {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "operation parameter is required",
		}, nil
	}

	switch operation {
	case "create_batch":
		return b.createBatch(ctx, params)
	case "process_batch":
		return b.processBatch(ctx, params)
	case "get_batch_status":
		return b.getBatchStatus(ctx, params)
	case "list_batches":
		return b.listBatches(ctx)
	case "get_processors":
		return b.getProcessors(ctx)
	default:
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("unsupported operation: %s", operation),
		}, nil
	}
}

// createBatch creates a new batch
func (b *BatchProcessorTool) createBatch(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	batchID, ok := params["batch_id"].(string)
	if !ok {
		batchID = fmt.Sprintf("batch_%d", time.Now().UnixNano())
	}

	items, ok := params["items"].([]interface{})
	if !ok {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "items parameter is required and must be an array",
		}, nil
	}

	batch := &Batch{
		ID:        batchID,
		Items:     items,
		Status:    "created",
		CreatedAt: time.Now(),
		Results:   make([]interface{}, 0),
		Errors:    make([]string, 0),
	}

	b.batchMutex.Lock()
	b.batches[batchID] = batch
	b.batchMutex.Unlock()

	return &tools.ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"batch_id":   batchID,
			"item_count": len(items),
			"status":     "created",
		},
	}, nil
}

// processBatch processes a batch using a specified processor
func (b *BatchProcessorTool) processBatch(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	batchID, ok := params["batch_id"].(string)
	if !ok {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "batch_id parameter is required",
		}, nil
	}

	processorID, ok := params["processor_id"].(string)
	if !ok {
		processorID = "default"
	}

	b.batchMutex.Lock()
	batch, exists := b.batches[batchID]
	if !exists {
		b.batchMutex.Unlock()
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("batch %s not found", batchID),
		}, nil
	}

	if batch.Status != "created" {
		b.batchMutex.Unlock()
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("batch %s is already %s", batchID, batch.Status),
		}, nil
	}

	batch.Status = "processing"
	startTime := time.Now()
	batch.StartedAt = &startTime
	batch.ProcessorID = processorID
	b.batchMutex.Unlock()

	b.processMutex.RLock()
	processor, exists := b.processors[processorID]
	b.processMutex.RUnlock()

	if !exists {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("processor %s not found", processorID),
		}, nil
	}

	// Process items in parallel
	results, errors := b.processItems(ctx, batch.Items, processor)

	// Update batch with results
	b.batchMutex.Lock()
	batch.Results = results
	batch.Errors = errors
	batch.Status = "completed"
	completedTime := time.Now()
	batch.CompletedAt = &completedTime
	b.batchMutex.Unlock()

	duration := completedTime.Sub(*batch.StartedAt)

	return &tools.ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"batch_id":        batchID,
			"status":          "completed",
			"items_processed": len(batch.Items),
			"successful":      len(results),
			"errors":          len(errors),
			"duration_ms":     duration.Milliseconds(),
			"items_per_sec":   float64(len(batch.Items)) / duration.Seconds(),
		},
	}, nil
}

// processItems processes items using the specified processor
func (b *BatchProcessorTool) processItems(ctx context.Context, items []interface{}, processor *Processor) ([]interface{}, []string) {
	results := make([]interface{}, 0, len(items))
	errors := make([]string, 0)
	
	// Create worker pool
	itemChan := make(chan interface{}, len(items))
	resultChan := make(chan interface{}, len(items))
	errorChan := make(chan string, len(items))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < processor.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range itemChan {
				select {
				case <-ctx.Done():
					errorChan <- "processing cancelled"
					return
				default:
					result, err := processor.ProcessingFn(item)
					if err != nil {
						errorChan <- err.Error()
					} else {
						resultChan <- result
					}
				}
			}
		}()
	}

	// Send items to workers
	go func() {
		defer close(itemChan)
		for _, item := range items {
			select {
			case itemChan <- item:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// Gather results and errors
	for result := range resultChan {
		results = append(results, result)
	}

	for err := range errorChan {
		errors = append(errors, err)
	}

	return results, errors
}

// getBatchStatus gets the status of a specific batch
func (b *BatchProcessorTool) getBatchStatus(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	batchID, ok := params["batch_id"].(string)
	if !ok {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "batch_id parameter is required",
		}, nil
	}

	b.batchMutex.RLock()
	batch, exists := b.batches[batchID]
	b.batchMutex.RUnlock()

	if !exists {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("batch %s not found", batchID),
		}, nil
	}

	data := map[string]interface{}{
		"batch_id":   batch.ID,
		"status":     batch.Status,
		"item_count": len(batch.Items),
		"created_at": batch.CreatedAt,
	}

	if batch.StartedAt != nil {
		data["started_at"] = *batch.StartedAt
	}

	if batch.CompletedAt != nil {
		data["completed_at"] = *batch.CompletedAt
		data["duration_ms"] = batch.CompletedAt.Sub(*batch.StartedAt).Milliseconds()
		data["results_count"] = len(batch.Results)
		data["errors_count"] = len(batch.Errors)
	}

	if batch.ProcessorID != "" {
		data["processor_id"] = batch.ProcessorID
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    data,
	}, nil
}

// listBatches lists all batches
func (b *BatchProcessorTool) listBatches(ctx context.Context) (*tools.ToolExecutionResult, error) {
	b.batchMutex.RLock()
	batches := make([]map[string]interface{}, 0, len(b.batches))
	
	for _, batch := range b.batches {
		batchInfo := map[string]interface{}{
			"batch_id":   batch.ID,
			"status":     batch.Status,
			"item_count": len(batch.Items),
			"created_at": batch.CreatedAt,
		}
		
		if batch.CompletedAt != nil {
			batchInfo["duration_ms"] = batch.CompletedAt.Sub(*batch.StartedAt).Milliseconds()
		}
		
		batches = append(batches, batchInfo)
	}
	b.batchMutex.RUnlock()

	return &tools.ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"batches": batches,
			"count":   len(batches),
		},
	}, nil
}

// getProcessors lists available processors
func (b *BatchProcessorTool) getProcessors(ctx context.Context) (*tools.ToolExecutionResult, error) {
	b.processMutex.RLock()
	processors := make([]map[string]interface{}, 0, len(b.processors))
	
	for _, processor := range b.processors {
		processors = append(processors, map[string]interface{}{
			"id":           processor.ID,
			"type":         processor.Type,
			"batch_size":   processor.BatchSize,
			"concurrency":  processor.Concurrency,
		})
	}
	b.processMutex.RUnlock()

	return &tools.ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"processors": processors,
			"count":      len(processors),
		},
	}, nil
}

// registerDefaultProcessors registers default processors
func (b *BatchProcessorTool) registerDefaultProcessors() {
	// Default text processor
	b.processors["default"] = &Processor{
		ID:          "default",
		Type:        "text",
		BatchSize:   100,
		Concurrency: 4,
		ProcessingFn: func(item interface{}) (interface{}, error) {
			// Simple text processing simulation
			time.Sleep(10 * time.Millisecond)
			return map[string]interface{}{
				"processed": fmt.Sprintf("processed: %v", item),
				"length":    len(fmt.Sprintf("%v", item)),
			}, nil
		},
	}

	// Math processor
	b.processors["math"] = &Processor{
		ID:          "math",
		Type:        "computation",
		BatchSize:   50,
		Concurrency: 8,
		ProcessingFn: func(item interface{}) (interface{}, error) {
			if num, ok := item.(float64); ok {
				// Simulate computation
				time.Sleep(5 * time.Millisecond)
				return map[string]interface{}{
					"input":  num,
					"square": num * num,
					"sqrt":   fmt.Sprintf("%.2f", num*0.5), // Simplified
				}, nil
			}
			return nil, fmt.Errorf("item must be a number")
		},
	}

	// I/O simulator processor
	b.processors["io"] = &Processor{
		ID:          "io",
		Type:        "io_simulation",
		BatchSize:   20,
		Concurrency: 2,
		ProcessingFn: func(item interface{}) (interface{}, error) {
			// Simulate I/O operation
			time.Sleep(50 * time.Millisecond)
			return map[string]interface{}{
				"item":      item,
				"processed": true,
				"timestamp": time.Now().Unix(),
			}, nil
		},
	}
}

// CreateBatchProcessorTool creates and configures the batch processor tool
func CreateBatchProcessorTool() *tools.Tool {
	return &tools.Tool{
		ID:          "batch_processor",
		Name:        "Batch Processing Tool",
		Description: "Demonstrates batch processing with parallel execution and performance monitoring",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"operation": {
					Type:        "string",
					Description: "Operation to perform",
					Enum:        []interface{}{"create_batch", "process_batch", "get_batch_status", "list_batches", "get_processors"},
				},
				"batch_id": {
					Type:        "string",
					Description: "Batch identifier",
				},
				"items": {
					Type:        "array",
					Description: "Items to process in the batch",
				},
				"processor_id": {
					Type:        "string",
					Description: "Processor to use for batch processing",
					Enum:        []interface{}{"default", "math", "io"},
				},
			},
			Required: []string{"operation"},
		},
		Metadata: &tools.ToolMetadata{
			Author:  "AG-UI SDK Examples",
			License: "MIT",
			Tags:    []string{"batch", "performance", "parallel"},
		},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  false,
			Async:      true,
			Cancelable: true,
			Timeout:    120 * time.Second,
		},
		Executor: NewBatchProcessorTool(),
	}
}

// RunBatchProcessorExample runs the batch processor example
func RunBatchProcessorExample() error {
	// Create registry and register the batch processor tool
	registry := tools.NewRegistry()
	batchTool := CreateBatchProcessorTool()

	if err := registry.Register(batchTool); err != nil {
		return fmt.Errorf("failed to register batch processor tool: %w", err)
	}

	// Create execution engine
	engine := tools.NewExecutionEngine(registry,
		tools.WithDefaultTimeout(120*time.Second),
	)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := engine.Shutdown(ctx); err != nil {
			log.Printf("Engine shutdown error: %v", err)
		}
	}()

	ctx := context.Background()

	fmt.Println("=== Batch Processing Tool Example ===")
	fmt.Println("Demonstrates: Batch processing, parallel execution, and performance monitoring")
	fmt.Println()

	// Example 1: Get available processors
	fmt.Println("1. Getting available processors...")
	result, err := engine.Execute(ctx, "batch_processor", map[string]interface{}{
		"operation": "get_processors",
	})
	if err != nil {
		fmt.Printf("Error getting processors: %v\n", err)
	} else if result.Success {
		data := result.Data.(map[string]interface{})
		processors := data["processors"].([]interface{})
		fmt.Printf("  Available processors: %d\n", len(processors))
		for _, proc := range processors {
			procMap := proc.(map[string]interface{})
			fmt.Printf("    - %s (%s): concurrency=%v\n", 
				procMap["id"], procMap["type"], procMap["concurrency"])
		}
	}

	// Example 2: Create text processing batch
	fmt.Println("\n2. Creating text processing batch...")
	textItems := []interface{}{
		"Hello, world!",
		"Batch processing is efficient",
		"Performance monitoring helps optimization",
		"Concurrent execution improves throughput",
		"Error handling is important",
	}

	result, err = engine.Execute(ctx, "batch_processor", map[string]interface{}{
		"operation": "create_batch",
		"batch_id":  "text_batch",
		"items":     textItems,
	})
	if err != nil {
		fmt.Printf("Error creating batch: %v\n", err)
	} else if result.Success {
		data := result.Data.(map[string]interface{})
		fmt.Printf("  Created batch: %s with %v items\n", data["batch_id"], data["item_count"])
	}

	// Example 3: Process the text batch
	fmt.Println("\n3. Processing text batch...")
	result, err = engine.Execute(ctx, "batch_processor", map[string]interface{}{
		"operation":    "process_batch",
		"batch_id":     "text_batch",
		"processor_id": "default",
	})
	if err != nil {
		fmt.Printf("Error processing batch: %v\n", err)
	} else if result.Success {
		data := result.Data.(map[string]interface{})
		fmt.Printf("  Processed %v items in %vms\n", data["items_processed"], data["duration_ms"])
		fmt.Printf("  Successful: %v, Errors: %v\n", data["successful"], data["errors"])
		fmt.Printf("  Throughput: %.2f items/sec\n", data["items_per_sec"])
	}

	// Example 4: Create and process math batch
	fmt.Println("\n4. Creating and processing math batch...")
	mathItems := []interface{}{1.0, 4.0, 9.0, 16.0, 25.0, 36.0, 49.0, 64.0, 81.0, 100.0}

	result, err = engine.Execute(ctx, "batch_processor", map[string]interface{}{
		"operation": "create_batch",
		"batch_id":  "math_batch",
		"items":     mathItems,
	})
	if err != nil {
		fmt.Printf("Error creating math batch: %v\n", err)
	} else {
		// Process immediately
		result, err = engine.Execute(ctx, "batch_processor", map[string]interface{}{
			"operation":    "process_batch",
			"batch_id":     "math_batch",
			"processor_id": "math",
		})
		if err != nil {
			fmt.Printf("Error processing math batch: %v\n", err)
		} else if result.Success {
			data := result.Data.(map[string]interface{})
			fmt.Printf("  Math batch processed: %v items in %vms\n", 
				data["items_processed"], data["duration_ms"])
		}
	}

	// Example 5: List all batches
	fmt.Println("\n5. Listing all batches...")
	result, err = engine.Execute(ctx, "batch_processor", map[string]interface{}{
		"operation": "list_batches",
	})
	if err != nil {
		fmt.Printf("Error listing batches: %v\n", err)
	} else if result.Success {
		data := result.Data.(map[string]interface{})
		batches := data["batches"].([]interface{})
		fmt.Printf("  Total batches: %d\n", len(batches))
		for _, batch := range batches {
			batchMap := batch.(map[string]interface{})
			fmt.Printf("    - %s: %s (%v items)\n", 
				batchMap["batch_id"], batchMap["status"], batchMap["item_count"])
			if duration, exists := batchMap["duration_ms"]; exists {
				fmt.Printf("      Duration: %vms\n", duration)
			}
		}
	}

	return nil
}