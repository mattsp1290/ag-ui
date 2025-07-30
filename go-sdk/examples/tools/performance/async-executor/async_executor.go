package asyncexecutor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/tools"
)

// AsyncExecutorTool demonstrates asynchronous task execution with performance monitoring
type AsyncExecutorTool struct {
	maxConcurrency int
	taskQueue      chan Task
	results        map[string]*TaskResult
	resultsMutex   sync.RWMutex
	workers        int
}

// Task represents an asynchronous task
type Task struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Params   map[string]interface{} `json:"params"`
	Priority int                    `json:"priority"`
}

// TaskResult represents the result of an asynchronous task
type TaskResult struct {
	TaskID     string        `json:"task_id"`
	Success    bool          `json:"success"`
	Result     interface{}   `json:"result,omitempty"`
	Error      string        `json:"error,omitempty"`
	StartTime  time.Time     `json:"start_time"`
	EndTime    time.Time     `json:"end_time"`
	Duration   time.Duration `json:"duration"`
	WorkerID   int           `json:"worker_id"`
}

// NewAsyncExecutorTool creates a new async executor tool
func NewAsyncExecutorTool() *AsyncExecutorTool {
	return &AsyncExecutorTool{
		maxConcurrency: 10,
		taskQueue:      make(chan Task, 100),
		results:        make(map[string]*TaskResult),
		workers:        0,
	}
}

// Execute handles async executor operations
func (a *AsyncExecutorTool) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	operation, ok := params["operation"].(string)
	if !ok {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "operation parameter is required",
		}, nil
	}

	switch operation {
	case "submit":
		return a.submitTask(ctx, params)
	case "status":
		return a.getTaskStatus(ctx, params)
	case "results":
		return a.getAllResults(ctx)
	case "worker_stats":
		return a.getWorkerStats(ctx)
	default:
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("unsupported operation: %s", operation),
		}, nil
	}
}

// submitTask submits a new task for async execution
func (a *AsyncExecutorTool) submitTask(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	taskID, ok := params["task_id"].(string)
	if !ok {
		taskID = fmt.Sprintf("task_%d", time.Now().UnixNano())
	}

	taskType, ok := params["task_type"].(string)
	if !ok {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "task_type parameter is required",
		}, nil
	}

	taskParams, _ := params["task_params"].(map[string]interface{})
	if taskParams == nil {
		taskParams = make(map[string]interface{})
	}

	priority, _ := params["priority"].(float64)

	task := Task{
		ID:       taskID,
		Type:     taskType,
		Params:   taskParams,
		Priority: int(priority),
	}

	// Start workers if needed
	if a.workers == 0 {
		a.startWorkers()
	}

	// Submit task
	select {
	case a.taskQueue <- task:
		return &tools.ToolExecutionResult{
			Success: true,
			Data: map[string]interface{}{
				"task_id":        taskID,
				"status":         "submitted",
				"queue_length":   len(a.taskQueue),
				"active_workers": a.workers,
			},
		}, nil
	case <-ctx.Done():
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "task submission cancelled",
		}, nil
	default:
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "task queue full",
		}, nil
	}
}

// getTaskStatus gets the status of a specific task
func (a *AsyncExecutorTool) getTaskStatus(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	taskID, ok := params["task_id"].(string)
	if !ok {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "task_id parameter is required",
		}, nil
	}

	a.resultsMutex.RLock()
	result, exists := a.results[taskID]
	a.resultsMutex.RUnlock()

	if !exists {
		return &tools.ToolExecutionResult{
			Success: true,
			Data: map[string]interface{}{
				"task_id": taskID,
				"status":  "not_found",
			},
		}, nil
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"task_id":     taskID,
			"status":      "completed",
			"success":     result.Success,
			"duration_ms": result.Duration.Milliseconds(),
			"worker_id":   result.WorkerID,
			"result":      result.Result,
			"error":       result.Error,
		},
	}, nil
}

// getAllResults gets all task results
func (a *AsyncExecutorTool) getAllResults(ctx context.Context) (*tools.ToolExecutionResult, error) {
	a.resultsMutex.RLock()
	resultsCopy := make(map[string]*TaskResult)
	for k, v := range a.results {
		resultsCopy[k] = v
	}
	a.resultsMutex.RUnlock()

	summary := map[string]interface{}{
		"total_tasks":     len(resultsCopy),
		"successful":      0,
		"failed":          0,
		"average_duration": 0.0,
	}

	var totalDuration time.Duration
	for _, result := range resultsCopy {
		totalDuration += result.Duration
		if result.Success {
			summary["successful"] = summary["successful"].(int) + 1
		} else {
			summary["failed"] = summary["failed"].(int) + 1
		}
	}

	if len(resultsCopy) > 0 {
		summary["average_duration"] = float64(totalDuration.Milliseconds()) / float64(len(resultsCopy))
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"summary": summary,
			"results": resultsCopy,
		},
	}, nil
}

// getWorkerStats gets worker statistics
func (a *AsyncExecutorTool) getWorkerStats(ctx context.Context) (*tools.ToolExecutionResult, error) {
	return &tools.ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"active_workers":   a.workers,
			"max_concurrency":  a.maxConcurrency,
			"queue_length":     len(a.taskQueue),
			"queue_capacity":   cap(a.taskQueue),
			"total_results":    len(a.results),
		},
	}, nil
}

// startWorkers starts the worker goroutines
func (a *AsyncExecutorTool) startWorkers() {
	for i := 0; i < a.maxConcurrency; i++ {
		go a.worker(i + 1)
		a.workers++
	}
}

// worker processes tasks from the queue
func (a *AsyncExecutorTool) worker(workerID int) {
	for task := range a.taskQueue {
		result := &TaskResult{
			TaskID:    task.ID,
			StartTime: time.Now(),
			WorkerID:  workerID,
		}

		// Simulate task execution
		switch task.Type {
		case "compute":
			result.Result, result.Error = a.executeComputeTask(task.Params)
		case "io":
			result.Result, result.Error = a.executeIOTask(task.Params)
		case "network":
			result.Result, result.Error = a.executeNetworkTask(task.Params)
		default:
			result.Error = fmt.Sprintf("unknown task type: %s", task.Type)
		}

		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		result.Success = result.Error == ""

		a.resultsMutex.Lock()
		a.results[task.ID] = result
		a.resultsMutex.Unlock()
	}
}

// executeComputeTask simulates a compute-intensive task
func (a *AsyncExecutorTool) executeComputeTask(params map[string]interface{}) (interface{}, string) {
	iterations, _ := params["iterations"].(float64)
	if iterations == 0 {
		iterations = 1000000
	}

	// Simulate computation
	var sum int64
	for i := 0; i < int(iterations); i++ {
		sum += int64(i)
	}

	return map[string]interface{}{
		"sum":        sum,
		"iterations": int(iterations),
	}, ""
}

// executeIOTask simulates an I/O intensive task
func (a *AsyncExecutorTool) executeIOTask(params map[string]interface{}) (interface{}, string) {
	delay, _ := params["delay_ms"].(float64)
	if delay == 0 {
		delay = 100
	}

	// Simulate I/O delay
	time.Sleep(time.Duration(delay) * time.Millisecond)

	return map[string]interface{}{
		"operation": "file_read",
		"delay_ms":  int(delay),
		"status":    "completed",
	}, ""
}

// executeNetworkTask simulates a network task
func (a *AsyncExecutorTool) executeNetworkTask(params map[string]interface{}) (interface{}, string) {
	url, _ := params["url"].(string)
	if url == "" {
		url = "https://example.com"
	}

	// Simulate network delay
	time.Sleep(200 * time.Millisecond)

	return map[string]interface{}{
		"url":            url,
		"response_code":  200,
		"response_time":  "200ms",
		"content_length": 1234,
	}, ""
}

// CreateAsyncExecutorTool creates and configures the async executor tool
func CreateAsyncExecutorTool() *tools.Tool {
	return &tools.Tool{
		ID:          "async_executor",
		Name:        "Asynchronous Task Executor",
		Description: "Demonstrates asynchronous task execution with performance monitoring",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"operation": {
					Type:        "string",
					Description: "Operation to perform",
					Enum:        []interface{}{"submit", "status", "results", "worker_stats"},
				},
				"task_id": {
					Type:        "string",
					Description: "Task identifier",
				},
				"task_type": {
					Type:        "string",
					Description: "Type of task to execute",
					Enum:        []interface{}{"compute", "io", "network"},
				},
				"task_params": {
					Type:        "object",
					Description: "Task parameters",
				},
				"priority": {
					Type:    "number",
					Minimum: &[]float64{0}[0],
					Maximum: &[]float64{10}[0],
				},
			},
			Required: []string{"operation"},
		},
		Metadata: &tools.ToolMetadata{
			Author:  "AG-UI SDK Examples",
			License: "MIT",
			Tags:    []string{"async", "performance", "concurrency"},
		},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  false,
			Async:      true,
			Cancelable: true,
			Timeout:    60 * time.Second,
		},
		Executor: NewAsyncExecutorTool(),
	}
}

// RunAsyncExecutorExample runs the async executor example
func RunAsyncExecutorExample() error {
	// Create registry and register the async executor tool
	registry := tools.NewRegistry()
	asyncTool := CreateAsyncExecutorTool()

	if err := registry.Register(asyncTool); err != nil {
		return fmt.Errorf("failed to register async executor tool: %w", err)
	}

	// Create execution engine
	engine := tools.NewExecutionEngine(registry,
		tools.WithDefaultTimeout(60*time.Second),
	)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := engine.Shutdown(ctx); err != nil {
			log.Printf("Engine shutdown error: %v", err)
		}
	}()

	ctx := context.Background()

	fmt.Println("=== Asynchronous Task Executor Example ===")
	fmt.Println("Demonstrates: Concurrent task execution, work queues, and performance monitoring")
	fmt.Println()

	// Example 1: Submit compute tasks
	fmt.Println("1. Submitting compute tasks...")
	for i := 0; i < 5; i++ {
		result, err := engine.Execute(ctx, "async_executor", map[string]interface{}{
			"operation":  "submit",
			"task_type":  "compute",
			"task_id":    fmt.Sprintf("compute_%d", i+1),
			"priority":   float64(i + 1),
			"task_params": map[string]interface{}{
				"iterations": float64(500000 * (i + 1)),
			},
		})
		if err != nil {
			fmt.Printf("Error submitting compute task %d: %v\n", i+1, err)
		} else if result.Success {
			data := result.Data.(map[string]interface{})
			fmt.Printf("  Submitted task %s (queue: %v)\n", data["task_id"], data["queue_length"])
		}
	}

	// Example 2: Submit I/O tasks
	fmt.Println("\n2. Submitting I/O tasks...")
	for i := 0; i < 3; i++ {
		result, err := engine.Execute(ctx, "async_executor", map[string]interface{}{
			"operation":  "submit",
			"task_type":  "io",
			"task_id":    fmt.Sprintf("io_%d", i+1),
			"task_params": map[string]interface{}{
				"delay_ms": float64(100 + i*50),
			},
		})
		if err != nil {
			fmt.Printf("Error submitting I/O task %d: %v\n", i+1, err)
		} else if result.Success {
			data := result.Data.(map[string]interface{})
			fmt.Printf("  Submitted task %s\n", data["task_id"])
		}
	}

	// Wait for tasks to complete
	fmt.Println("\n3. Waiting for tasks to complete...")
	time.Sleep(3 * time.Second)

	// Example 3: Check worker stats
	fmt.Println("\n4. Checking worker statistics...")
	result, err := engine.Execute(ctx, "async_executor", map[string]interface{}{
		"operation": "worker_stats",
	})
	if err != nil {
		fmt.Printf("Error getting worker stats: %v\n", err)
	} else if result.Success {
		data := result.Data.(map[string]interface{})
		fmt.Printf("  Active workers: %v\n", data["active_workers"])
		fmt.Printf("  Queue length: %v\n", data["queue_length"])
		fmt.Printf("  Total results: %v\n", data["total_results"])
	}

	// Example 4: Get all results
	fmt.Println("\n5. Getting all task results...")
	result, err = engine.Execute(ctx, "async_executor", map[string]interface{}{
		"operation": "results",
	})
	if err != nil {
		fmt.Printf("Error getting results: %v\n", err)
	} else if result.Success {
		data := result.Data.(map[string]interface{})
		summary := data["summary"].(map[string]interface{})
		fmt.Printf("  Total tasks: %v\n", summary["total_tasks"])
		fmt.Printf("  Successful: %v\n", summary["successful"])
		fmt.Printf("  Failed: %v\n", summary["failed"])
		fmt.Printf("  Average duration: %.2fms\n", summary["average_duration"])
	}

	return nil
}