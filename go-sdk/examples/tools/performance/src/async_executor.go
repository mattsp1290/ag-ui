package main

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ag-ui/go-sdk/pkg/tools"
)

// AsyncExecutorTool demonstrates asynchronous execution patterns and performance optimization.
// This example shows how to implement async tools, worker pools, task queues, and resource monitoring.
type AsyncExecutorTool struct{}

// AsyncExecutorParams defines the parameters for the async executor tool
type AsyncExecutorParams struct {
	WorkType     string `json:"work_type" validate:"required,oneof=cpu io mixed"`
	WorkerCount  int    `json:"worker_count" validate:"min=1,max=100"`
	TaskCount    int    `json:"task_count" validate:"min=1,max=10000"`
	TaskDuration int    `json:"task_duration_ms" validate:"min=1,max=5000"`
	BatchSize    int    `json:"batch_size" validate:"min=1,max=1000"`
	UsePool      bool   `json:"use_pool"`
	Monitor      bool   `json:"monitor_resources"`
}

// AsyncExecutor implements async execution patterns with various optimization strategies
type AsyncExecutor struct {
	workerPool    *WorkerPool
	taskQueue     chan Task
	results       chan TaskResult
	resourceMon   *ResourceMonitor
	stats         *ExecutionStats
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	maxWorkers    int
	running       int64
}

// Task represents a unit of work to be executed
type Task struct {
	ID          string
	Type        string
	Duration    time.Duration
	Data        interface{}
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Priority    int
}

// TaskResult represents the result of task execution
type TaskResult struct {
	TaskID      string                 `json:"task_id"`
	Success     bool                   `json:"success"`
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Duration    time.Duration          `json:"duration"`
	WorkerID    int                    `json:"worker_id"`
	MemoryUsed  int64                  `json:"memory_used"`
	CPUTime     time.Duration          `json:"cpu_time"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// WorkerPool manages a pool of workers for concurrent task execution
type WorkerPool struct {
	workers     []*Worker
	taskQueue   chan Task
	resultQueue chan TaskResult
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	size        int
	stats       *PoolStats
}

// Worker represents a single worker in the pool
type Worker struct {
	ID          int
	taskQueue   chan Task
	resultQueue chan TaskResult
	ctx         context.Context
	stats       *WorkerStats
	executing   int64
}

// WorkerStats tracks worker performance metrics
type WorkerStats struct {
	TasksProcessed int64         `json:"tasks_processed"`
	TotalDuration  time.Duration `json:"total_duration"`
	AverageDuration time.Duration `json:"average_duration"`
	ErrorCount     int64         `json:"error_count"`
	IdleTime       time.Duration `json:"idle_time"`
	LastTaskTime   time.Time     `json:"last_task_time"`
}

// PoolStats tracks overall pool performance
type PoolStats struct {
	TotalTasks     int64           `json:"total_tasks"`
	CompletedTasks int64           `json:"completed_tasks"`
	FailedTasks    int64           `json:"failed_tasks"`
	ActiveWorkers  int64           `json:"active_workers"`
	QueueSize      int64           `json:"queue_size"`
	WorkerStats    []*WorkerStats  `json:"worker_stats"`
	ThroughputRPS  float64         `json:"throughput_rps"`
	AverageLatency time.Duration   `json:"average_latency"`
	StartTime      time.Time       `json:"start_time"`
}

// ResourceMonitor tracks system resource usage during execution
type ResourceMonitor struct {
	mu               sync.RWMutex
	monitoring       bool
	interval         time.Duration
	samples          []ResourceSample
	maxSamples       int
	cpuPercent       float64
	memoryBytes      int64
	goroutineCount   int
	gcPauseTotal     time.Duration
	gcPauseCount     int64
	startMemStats    runtime.MemStats
	currentMemStats  runtime.MemStats
}

// ResourceSample represents a point-in-time resource measurement
type ResourceSample struct {
	Timestamp      time.Time     `json:"timestamp"`
	CPUPercent     float64       `json:"cpu_percent"`
	MemoryBytes    int64         `json:"memory_bytes"`
	GoroutineCount int           `json:"goroutine_count"`
	GCPauseNS      uint64        `json:"gc_pause_ns"`
	HeapAllocBytes uint64        `json:"heap_alloc_bytes"`
	HeapSysBytes   uint64        `json:"heap_sys_bytes"`
	StackInUse     uint64        `json:"stack_in_use"`
}

// ExecutionStats tracks overall execution statistics
type ExecutionStats struct {
	StartTime         time.Time     `json:"start_time"`
	EndTime           *time.Time    `json:"end_time,omitempty"`
	TotalDuration     time.Duration `json:"total_duration"`
	TasksSubmitted    int64         `json:"tasks_submitted"`
	TasksCompleted    int64         `json:"tasks_completed"`
	TasksFailed       int64         `json:"tasks_failed"`
	ThroughputRPS     float64       `json:"throughput_rps"`
	AverageLatency    time.Duration `json:"average_latency"`
	P95Latency        time.Duration `json:"p95_latency"`
	P99Latency        time.Duration `json:"p99_latency"`
	ErrorRate         float64       `json:"error_rate"`
	PeakMemoryUsage   int64         `json:"peak_memory_usage"`
	TotalCPUTime      time.Duration `json:"total_cpu_time"`
	ConcurrencyLevel  int           `json:"concurrency_level"`
	QueueUtilization  float64       `json:"queue_utilization"`
}

// CreateAsyncExecutorTool creates and registers the async executor tool
func CreateAsyncExecutorTool() *tools.Tool {
	return &tools.Tool{
		ID:          "async-executor",
		Name:        "AsyncExecutor",
		Description: "Demonstrates asynchronous execution patterns and performance optimization techniques",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"work_type": {
					Type:        "string",
					Description: "Type of work to simulate (cpu, io, mixed)",
					Enum:        []interface{}{"cpu", "io", "mixed"},
					Default:     "mixed",
				},
				"worker_count": {
					Type:        "integer",
					Description: "Number of workers in the pool",
					Default:     runtime.NumCPU(),
					Minimum:     func() *float64 { v := 1.0; return &v }(),
					Maximum:     func() *float64 { v := 100.0; return &v }(),
				},
				"task_count": {
					Type:        "integer",
					Description: "Total number of tasks to execute",
					Default:     1000,
					Minimum:     func() *float64 { v := 1.0; return &v }(),
					Maximum:     func() *float64 { v := 10000.0; return &v }(),
				},
				"task_duration_ms": {
					Type:        "integer",
					Description: "Average duration per task in milliseconds",
					Default:     100,
					Minimum:     func() *float64 { v := 1.0; return &v }(),
					Maximum:     func() *float64 { v := 5000.0; return &v }(),
				},
				"batch_size": {
					Type:        "integer",
					Description: "Number of tasks to process in each batch",
					Default:     50,
					Minimum:     func() *float64 { v := 1.0; return &v }(),
					Maximum:     func() *float64 { v := 1000.0; return &v }(),
				},
				"use_pool": {
					Type:        "boolean",
					Description: "Whether to use worker pool (vs simple goroutines)",
					Default:     true,
				},
				"monitor_resources": {
					Type:        "boolean",
					Description: "Whether to monitor system resources during execution",
					Default:     true,
				},
			},
			Required: []string{"work_type"},
		},
		Executor: &AsyncExecutorTool{},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  false,
			Async:      true,
			Cancelable: true,
			Cacheable:  false,
			Timeout:    5 * time.Minute,
		},
		Metadata: &tools.ToolMetadata{
			Author:   "Performance Team",
			License:  "MIT",
			Tags:     []string{"performance", "async", "concurrency", "monitoring"},
			Examples: []tools.ToolExample{
				{
					Name:        "CPU Intensive Tasks",
					Description: "Execute CPU-bound tasks with worker pool",
					Input: map[string]interface{}{
						"work_type":        "cpu",
						"worker_count":     8,
						"task_count":       500,
						"task_duration_ms": 50,
						"use_pool":         true,
						"monitor_resources": true,
					},
				},
				{
					Name:        "I/O Simulation",
					Description: "Simulate I/O-bound operations",
					Input: map[string]interface{}{
						"work_type":        "io",
						"worker_count":     20,
						"task_count":       1000,
						"task_duration_ms": 200,
						"batch_size":       100,
					},
				},
			},
		},
	}
}

// Execute runs the async executor with the specified parameters
func (t *AsyncExecutorTool) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	// Parse and validate parameters
	p, err := parseAsyncParams(params)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Create execution context with cancellation
	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Initialize executor
	executor := NewAsyncExecutor(execCtx, p.WorkerCount, p.Monitor)
	defer executor.Close()

	// Start resource monitoring if enabled
	if p.Monitor {
		executor.StartResourceMonitoring(time.Millisecond * 100)
		defer executor.StopResourceMonitoring()
	}

	// Execute tasks based on configuration
	var results *ExecutionStats
	if p.UsePool {
		results, err = executor.ExecuteWithPool(p)
	} else {
		results, err = executor.ExecuteWithGoroutines(p)
	}

	if err != nil {
		return &tools.ToolExecutionResult{
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}, nil
	}

	// Collect final metrics
	response := map[string]interface{}{
		"execution_stats":   results,
		"worker_pool_stats": executor.GetPoolStats(),
		"resource_usage":    executor.GetResourceStats(),
		"recommendations":   generateRecommendations(results, p),
		"configuration":     p,
	}

	return &tools.ToolExecutionResult{
		Success:   true,
		Data:      response,
		Timestamp: time.Now(),
		Duration:  results.TotalDuration,
		Metadata: map[string]interface{}{
			"tasks_per_second": results.ThroughputRPS,
			"peak_memory":      results.PeakMemoryUsage,
			"error_rate":       results.ErrorRate,
			"concurrency":      results.ConcurrencyLevel,
		},
	}, nil
}

// NewAsyncExecutor creates a new async executor
func NewAsyncExecutor(ctx context.Context, maxWorkers int, monitor bool) *AsyncExecutor {
	execCtx, cancel := context.WithCancel(ctx)
	
	executor := &AsyncExecutor{
		taskQueue:   make(chan Task, maxWorkers*2), // Buffered queue
		results:     make(chan TaskResult, maxWorkers*2),
		stats:       &ExecutionStats{StartTime: time.Now()},
		maxWorkers:  maxWorkers,
		ctx:         execCtx,
		cancel:      cancel,
	}

	if monitor {
		executor.resourceMon = NewResourceMonitor()
	}

	return executor
}

// ExecuteWithPool executes tasks using a worker pool
func (e *AsyncExecutor) ExecuteWithPool(params *AsyncExecutorParams) (*ExecutionStats, error) {
	// Create worker pool
	pool := NewWorkerPool(e.ctx, params.WorkerCount, e.taskQueue, e.results)
	e.workerPool = pool
	pool.Start()
	defer pool.Stop()

	// Submit tasks
	go e.submitTasks(params)

	// Collect results
	return e.collectResults(params.TaskCount)
}

// ExecuteWithGoroutines executes tasks using simple goroutines
func (e *AsyncExecutor) ExecuteWithGoroutines(params *AsyncExecutorParams) (*ExecutionStats, error) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, params.WorkerCount) // Limit concurrency

	// Generate and execute tasks
	for i := 0; i < params.TaskCount; i++ {
		task := e.createTask(i, params)
		
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			
			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-e.ctx.Done():
				return
			}
			
			// Execute task
			result := e.executeTask(t, 0) // Worker ID 0 for goroutines
			
			select {
			case e.results <- result:
			case <-e.ctx.Done():
				return
			}
		}(task)
	}

	// Close results channel when all tasks complete
	go func() {
		wg.Wait()
		close(e.results)
	}()

	// Collect results
	return e.collectResults(params.TaskCount)
}

// submitTasks submits tasks to the worker pool
func (e *AsyncExecutor) submitTasks(params *AsyncExecutorParams) {
	defer close(e.taskQueue)
	
	for i := 0; i < params.TaskCount; i++ {
		task := e.createTask(i, params)
		
		select {
		case e.taskQueue <- task:
			atomic.AddInt64(&e.stats.TasksSubmitted, 1)
		case <-e.ctx.Done():
			return
		}
	}
}

// createTask creates a new task based on parameters
func (e *AsyncExecutor) createTask(id int, params *AsyncExecutorParams) Task {
	// Add some randomness to task duration
	variance := float64(params.TaskDuration) * 0.2 // 20% variance
	duration := time.Duration(float64(params.TaskDuration) + (rand.Float64()-0.5)*variance*2) * time.Millisecond
	
	return Task{
		ID:        fmt.Sprintf("task-%d", id),
		Type:      params.WorkType,
		Duration:  duration,
		Data:      map[string]interface{}{"index": id, "batch": id / params.BatchSize},
		CreatedAt: time.Now(),
		Priority:  rand.Intn(3), // Random priority 0-2
	}
}

// executeTask executes a single task
func (e *AsyncExecutor) executeTask(task Task, workerID int) TaskResult {
	startTime := time.Now()
	task.StartedAt = &startTime
	
	var result interface{}
	var err error
	
	// Simulate different types of work
	switch task.Type {
	case "cpu":
		result = e.simulateCPUWork(task.Duration)
	case "io":
		result = e.simulateIOWork(task.Duration)
	case "mixed":
		if rand.Float32() < 0.5 {
			result = e.simulateCPUWork(task.Duration / 2)
		} else {
			result = e.simulateIOWork(task.Duration / 2)
		}
	default:
		err = fmt.Errorf("unknown work type: %s", task.Type)
	}
	
	endTime := time.Now()
	task.CompletedAt = &endTime
	duration := endTime.Sub(startTime)
	
	// Get memory usage
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	return TaskResult{
		TaskID:     task.ID,
		Success:    err == nil,
		Result:     result,
		Error:      formatError(err),
		Duration:   duration,
		WorkerID:   workerID,
		MemoryUsed: int64(memStats.Alloc),
		CPUTime:    duration, // Approximation
		Metadata: map[string]interface{}{
			"task_type":    task.Type,
			"priority":     task.Priority,
			"queue_time":   startTime.Sub(task.CreatedAt),
			"worker_id":    workerID,
		},
	}
}

// simulateCPUWork simulates CPU-intensive work
func (e *AsyncExecutor) simulateCPUWork(duration time.Duration) map[string]interface{} {
	start := time.Now()
	iterations := 0
	
	// CPU-bound loop
	for time.Since(start) < duration {
		// Some CPU work - calculating prime numbers
		n := rand.Intn(1000) + 100
		isPrime := true
		for i := 2; i*i <= n; i++ {
			if n%i == 0 {
				isPrime = false
				break
			}
		}
		if isPrime {
			iterations++
		}
	}
	
	return map[string]interface{}{
		"type":       "cpu",
		"iterations": iterations,
		"duration":   time.Since(start),
		"result":     fmt.Sprintf("processed %d iterations", iterations),
	}
}

// simulateIOWork simulates I/O-bound work
func (e *AsyncExecutor) simulateIOWork(duration time.Duration) map[string]interface{} {
	start := time.Now()
	
	// Simulate I/O wait
	select {
	case <-time.After(duration):
	case <-e.ctx.Done():
		return map[string]interface{}{
			"type":   "io",
			"status": "cancelled",
		}
	}
	
	return map[string]interface{}{
		"type":     "io",
		"duration": time.Since(start),
		"result":   "I/O operation completed",
		"bytes":    rand.Intn(1024) + 512,
	}
}

// collectResults collects and analyzes task results
func (e *AsyncExecutor) collectResults(expectedCount int) (*ExecutionStats, error) {
	results := make([]TaskResult, 0, expectedCount)
	completed := 0
	failed := 0
	var totalDuration time.Duration
	var latencies []time.Duration
	var peakMemory int64
	
	for result := range e.results {
		results = append(results, result)
		
		if result.Success {
			completed++
		} else {
			failed++
		}
		
		totalDuration += result.Duration
		latencies = append(latencies, result.Duration)
		
		if result.MemoryUsed > peakMemory {
			peakMemory = result.MemoryUsed
		}
		
		if len(results) >= expectedCount {
			break
		}
	}
	
	endTime := time.Now()
	totalTime := endTime.Sub(e.stats.StartTime)
	
	// Calculate statistics
	stats := &ExecutionStats{
		StartTime:        e.stats.StartTime,
		EndTime:          &endTime,
		TotalDuration:    totalTime,
		TasksSubmitted:   int64(expectedCount),
		TasksCompleted:   int64(completed),
		TasksFailed:      int64(failed),
		PeakMemoryUsage:  peakMemory,
		ConcurrencyLevel: e.maxWorkers,
	}
	
	if completed > 0 {
		stats.ThroughputRPS = float64(completed) / totalTime.Seconds()
		stats.AverageLatency = totalDuration / time.Duration(completed)
		stats.ErrorRate = float64(failed) / float64(completed+failed) * 100
		
		// Calculate percentiles
		if len(latencies) > 0 {
			stats.P95Latency = calculatePercentile(latencies, 0.95)
			stats.P99Latency = calculatePercentile(latencies, 0.99)
		}
	}
	
	return stats, nil
}

// Helper functions

func parseAsyncParams(params map[string]interface{}) (*AsyncExecutorParams, error) {
	p := &AsyncExecutorParams{
		WorkType:     "mixed",
		WorkerCount:  runtime.NumCPU(),
		TaskCount:    1000,
		TaskDuration: 100,
		BatchSize:    50,
		UsePool:      true,
		Monitor:      true,
	}
	
	if v, ok := params["work_type"].(string); ok {
		p.WorkType = v
	}
	if v, ok := params["worker_count"].(int); ok {
		p.WorkerCount = v
	}
	if v, ok := params["task_count"].(int); ok {
		p.TaskCount = v
	}
	if v, ok := params["task_duration_ms"].(int); ok {
		p.TaskDuration = v
	}
	if v, ok := params["batch_size"].(int); ok {
		p.BatchSize = v
	}
	if v, ok := params["use_pool"].(bool); ok {
		p.UsePool = v
	}
	if v, ok := params["monitor_resources"].(bool); ok {
		p.Monitor = v
	}
	
	return p, nil
}

func formatError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func calculatePercentile(latencies []time.Duration, percentile float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	
	// Sort latencies
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	
	// Simple bubble sort for small datasets
	for i := 0; i < len(sorted); i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}
	
	index := int(float64(len(sorted)) * percentile)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	
	return sorted[index]
}

func generateRecommendations(stats *ExecutionStats, params *AsyncExecutorParams) []string {
	var recommendations []string
	
	// Throughput recommendations
	if stats.ThroughputRPS < 100 {
		recommendations = append(recommendations, "Consider increasing worker count for better throughput")
	}
	
	// Latency recommendations
	if stats.AverageLatency > time.Millisecond*500 {
		recommendations = append(recommendations, "High average latency detected - consider optimizing task implementation")
	}
	
	// Error rate recommendations
	if stats.ErrorRate > 5.0 {
		recommendations = append(recommendations, "High error rate detected - review task implementation and error handling")
	}
	
	// Memory recommendations
	if stats.PeakMemoryUsage > 100*1024*1024 { // 100MB
		recommendations = append(recommendations, "High memory usage detected - consider implementing memory pooling")
	}
	
	// Concurrency recommendations
	cpuCount := runtime.NumCPU()
	if params.WorkType == "cpu" && params.WorkerCount > cpuCount {
		recommendations = append(recommendations, fmt.Sprintf("For CPU-bound tasks, consider reducing worker count to %d (CPU cores)", cpuCount))
	} else if params.WorkType == "io" && params.WorkerCount < cpuCount*2 {
		recommendations = append(recommendations, fmt.Sprintf("For I/O-bound tasks, consider increasing worker count to %d", cpuCount*2))
	}
	
	return recommendations
}

// Additional implementation for WorkerPool and ResourceMonitor...

func NewWorkerPool(ctx context.Context, size int, taskQueue chan Task, resultQueue chan TaskResult) *WorkerPool {
	poolCtx, cancel := context.WithCancel(ctx)
	
	pool := &WorkerPool{
		workers:     make([]*Worker, size),
		taskQueue:   taskQueue,
		resultQueue: resultQueue,
		ctx:         poolCtx,
		cancel:      cancel,
		size:        size,
		stats:       &PoolStats{StartTime: time.Now()},
	}
	
	// Create workers
	for i := 0; i < size; i++ {
		pool.workers[i] = &Worker{
			ID:          i + 1,
			taskQueue:   taskQueue,
			resultQueue: resultQueue,
			ctx:         poolCtx,
			stats:       &WorkerStats{},
		}
	}
	
	return pool
}

func (p *WorkerPool) Start() {
	for _, worker := range p.workers {
		p.wg.Add(1)
		go worker.Run(&p.wg)
	}
}

func (p *WorkerPool) Stop() {
	p.cancel()
	p.wg.Wait()
}

func (w *Worker) Run(wg *sync.WaitGroup) {
	defer wg.Done()
	
	for {
		select {
		case task, ok := <-w.taskQueue:
			if !ok {
				return // Channel closed
			}
			
			atomic.StoreInt64(&w.executing, 1)
			w.stats.TasksProcessed++
			
			// Execute task (simplified)
			start := time.Now()
			
			// Simulate work based on task type
			switch task.Type {
			case "cpu":
				// CPU work simulation
				for i := 0; i < 1000; i++ {
					_ = i * i
				}
			case "io":
				// I/O work simulation
				time.Sleep(task.Duration)
			}
			
			duration := time.Since(start)
			w.stats.TotalDuration += duration
			w.stats.AverageDuration = w.stats.TotalDuration / time.Duration(w.stats.TasksProcessed)
			w.stats.LastTaskTime = time.Now()
			
			result := TaskResult{
				TaskID:   task.ID,
				Success:  true,
				Duration: duration,
				WorkerID: w.ID,
			}
			
			select {
			case w.resultQueue <- result:
			case <-w.ctx.Done():
				return
			}
			
			atomic.StoreInt64(&w.executing, 0)
			
		case <-w.ctx.Done():
			return
		}
	}
}

func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{
		interval:   time.Millisecond * 100,
		maxSamples: 1000,
		samples:    make([]ResourceSample, 0, 1000),
	}
}

func (r *ResourceMonitor) Start() {
	r.mu.Lock()
	r.monitoring = true
	runtime.ReadMemStats(&r.startMemStats)
	r.mu.Unlock()
	
	go r.monitorLoop()
}

func (r *ResourceMonitor) Stop() {
	r.mu.Lock()
	r.monitoring = false
	r.mu.Unlock()
}

func (r *ResourceMonitor) monitorLoop() {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	
	for range ticker.C {
		r.mu.RLock()
		if !r.monitoring {
			r.mu.RUnlock()
			return
		}
		r.mu.RUnlock()
		
		r.takeSample()
	}
}

func (r *ResourceMonitor) takeSample() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	sample := ResourceSample{
		Timestamp:      time.Now(),
		MemoryBytes:    int64(memStats.Alloc),
		GoroutineCount: runtime.NumGoroutine(),
		HeapAllocBytes: memStats.HeapAlloc,
		HeapSysBytes:   memStats.HeapSys,
		StackInUse:     memStats.StackInuse,
	}
	
	r.mu.Lock()
	if len(r.samples) >= r.maxSamples {
		// Remove oldest sample
		r.samples = r.samples[1:]
	}
	r.samples = append(r.samples, sample)
	r.currentMemStats = memStats
	r.mu.Unlock()
}

func (e *AsyncExecutor) StartResourceMonitoring(interval time.Duration) {
	if e.resourceMon != nil {
		e.resourceMon.interval = interval
		e.resourceMon.Start()
	}
}

func (e *AsyncExecutor) StopResourceMonitoring() {
	if e.resourceMon != nil {
		e.resourceMon.Stop()
	}
}

func (e *AsyncExecutor) GetPoolStats() *PoolStats {
	if e.workerPool == nil {
		return nil
	}
	return e.workerPool.stats
}

func (e *AsyncExecutor) GetResourceStats() map[string]interface{} {
	if e.resourceMon == nil {
		return nil
	}
	
	e.resourceMon.mu.RLock()
	defer e.resourceMon.mu.RUnlock()
	
	return map[string]interface{}{
		"samples_collected": len(e.resourceMon.samples),
		"current_memory":    e.resourceMon.currentMemStats.Alloc,
		"peak_memory":       e.resourceMon.memoryBytes,
		"goroutine_count":   runtime.NumGoroutine(),
		"gc_pause_total":    e.resourceMon.currentMemStats.PauseTotalNs,
	}
}

func (e *AsyncExecutor) Close() {
	e.cancel()
	if e.resourceMon != nil {
		e.resourceMon.Stop()
	}
}

func main() {
	tool := CreateAsyncExecutorTool()
	
	// Example usage
	params := map[string]interface{}{
		"work_type":         "mixed",
		"worker_count":      8,
		"task_count":        1000,
		"task_duration_ms":  100,
		"use_pool":          true,
		"monitor_resources": true,
	}
	
	ctx := context.Background()
	executor := &AsyncExecutorTool{}
	
	result, err := executor.Execute(ctx, params)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	fmt.Printf("Async Execution Demo Results:\n")
	fmt.Printf("Success: %t\n", result.Success)
	fmt.Printf("Duration: %v\n", result.Duration)
	
	if data, ok := result.Data.(map[string]interface{}); ok {
		if stats, ok := data["execution_stats"].(*ExecutionStats); ok {
			fmt.Printf("Throughput: %.2f tasks/sec\n", stats.ThroughputRPS)
			fmt.Printf("Average Latency: %v\n", stats.AverageLatency)
			fmt.Printf("Error Rate: %.2f%%\n", stats.ErrorRate)
			fmt.Printf("Peak Memory: %d bytes\n", stats.PeakMemoryUsage)
		}
	}
}