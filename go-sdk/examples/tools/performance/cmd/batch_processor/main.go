//go:build examples
// +build examples

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

// BatchProcessorTool demonstrates batch processing optimization techniques.
// This example shows how to efficiently process large datasets using various batching strategies,
// memory management, and performance optimization patterns.
type BatchProcessorTool struct{}

// BatchProcessorParams defines the parameters for batch processing
type BatchProcessorParams struct {
	DataSize       int    `json:"data_size" validate:"min=100,max=1000000"`
	BatchSize      int    `json:"batch_size" validate:"min=1,max=10000"`
	WorkerCount    int    `json:"worker_count" validate:"min=1,max=100"`
	ProcessingType string `json:"processing_type" validate:"oneof=sequential parallel pipeline adaptive"`
	MemoryLimit    int64  `json:"memory_limit_mb"`
	BufferSize     int    `json:"buffer_size" validate:"min=10,max=100000"`
	CompressionType string `json:"compression_type" validate:"oneof=none gzip lz4 snappy"`
	EnableMetrics  bool   `json:"enable_metrics"`
	OptimizeFor    string `json:"optimize_for" validate:"oneof=throughput latency memory cpu"`
	EnablePrefetch bool   `json:"enable_prefetch"`
	ChunkingStrategy string `json:"chunking_strategy" validate:"oneof=fixed dynamic adaptive"`
}

// BatchProcessor manages efficient batch processing with various optimization strategies
type BatchProcessor struct {
	params          *BatchProcessorParams
	dataSource      DataSource
	processors      []DataProcessor
	outputSink      OutputSink
	memoryManager   *MemoryManager
	metrics         *BatchMetrics
	prefetcher      *DataPrefetcher
	compressor      Compressor
	scheduler       *BatchScheduler
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	mu              sync.RWMutex
	state           ProcessingState
}

// Data structures for batch processing
type DataItem struct {
	ID        string      `json:"id"`
	Data      interface{} `json:"data"`
	Size      int64       `json:"size"`
	Priority  int         `json:"priority"`
	Timestamp time.Time   `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

type Batch struct {
	ID        string     `json:"id"`
	Items     []DataItem `json:"items"`
	Size      int        `json:"size"`
	TotalSize int64      `json:"total_size"`
	CreatedAt time.Time  `json:"created_at"`
	Priority  int        `json:"priority"`
	Metadata  map[string]interface{} `json:"metadata"`
}

type ProcessingResult struct {
	BatchID       string                 `json:"batch_id"`
	ItemResults   []ItemResult           `json:"item_results"`
	Success       bool                   `json:"success"`
	Error         string                 `json:"error,omitempty"`
	ProcessingTime time.Duration         `json:"processing_time"`
	MemoryUsed    int64                  `json:"memory_used"`
	WorkerID      int                    `json:"worker_id"`
	Metadata      map[string]interface{} `json:"metadata"`
}

type ItemResult struct {
	ItemID        string                 `json:"item_id"`
	Success       bool                   `json:"success"`
	Result        interface{}            `json:"result,omitempty"`
	Error         string                 `json:"error,omitempty"`
	ProcessingTime time.Duration         `json:"processing_time"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// Interfaces for flexible architecture
type DataSource interface {
	GetBatch(size int) (*Batch, error)
	HasMore() bool
	TotalSize() int
	Reset() error
	GetMetrics() map[string]interface{}
}

type DataProcessor interface {
	Process(ctx context.Context, batch *Batch) (*ProcessingResult, error)
	GetCapabilities() ProcessorCapabilities
	GetMetrics() map[string]interface{}
	Cleanup() error
}

type OutputSink interface {
	Write(result *ProcessingResult) error
	Flush() error
	GetMetrics() map[string]interface{}
	Close() error
}

type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
	GetCompressionRatio() float64
}

// Processing capabilities and configurations
type ProcessorCapabilities struct {
	MaxBatchSize     int           `json:"max_batch_size"`
	OptimalBatchSize int           `json:"optimal_batch_size"`
	MemoryPerItem    int64         `json:"memory_per_item"`
	ProcessingTime   time.Duration `json:"estimated_processing_time"`
	SupportsParallel bool          `json:"supports_parallel"`
	CPUIntensive     bool          `json:"cpu_intensive"`
	IOIntensive      bool          `json:"io_intensive"`
}

type ProcessingState int

const (
	StateIdle ProcessingState = iota
	StateInitializing
	StateProcessing
	StatePaused
	StateCompleted
	StateError
)

// Memory management for efficient batch processing
type MemoryManager struct {
	maxMemory     int64
	currentMemory int64
	pools         map[string]*ObjectPool
	gc            *GCManager
	mu            sync.RWMutex
	metrics       *MemoryMetrics
}

type ObjectPool struct {
	pool     sync.Pool
	size     int64
	maxItems int
	created  int64
	reused   int64
}

type GCManager struct {
	threshold     int64
	interval      time.Duration
	lastGC        time.Time
	forcedGCCount int64
}

type MemoryMetrics struct {
	AllocatedBytes   int64   `json:"allocated_bytes"`
	PooledBytes      int64   `json:"pooled_bytes"`
	GCCount          int64   `json:"gc_count"`
	GCTotalTime      time.Duration `json:"gc_total_time"`
	MemoryEfficiency float64 `json:"memory_efficiency"`
	FragmentationLevel float64 `json:"fragmentation_level"`
}

// Data prefetching for performance optimization
type DataPrefetcher struct {
	enabled       bool
	bufferSize    int
	prefetchQueue chan *Batch
	dataSource    DataSource
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	metrics       *PrefetchMetrics
}

type PrefetchMetrics struct {
	BatchesPrefetched int64         `json:"batches_prefetched"`
	CacheHitRate      float64       `json:"cache_hit_rate"`
	PrefetchTime      time.Duration `json:"total_prefetch_time"`
	BufferUtilization float64       `json:"buffer_utilization"`
}

// Batch scheduling and load balancing
type BatchScheduler struct {
	strategy       SchedulingStrategy
	workQueue      chan *Batch
	priorityQueues map[int]chan *Batch
	loadBalancer   *LoadBalancer
	metrics        *SchedulerMetrics
}

type SchedulingStrategy int

const (
	StrategyRoundRobin SchedulingStrategy = iota
	StrategyPriority
	StrategyLoadBased
	StrategyAdaptive
)

type LoadBalancer struct {
	workers    []WorkerInfo
	selector   WorkerSelector
	metrics    *LoadBalancerMetrics
}

type WorkerInfo struct {
	ID           int
	CurrentLoad  int64
	Capacity     int64
	LastUsed     time.Time
	Performance  WorkerPerformance
}

type WorkerPerformance struct {
	AverageProcessingTime time.Duration
	SuccessRate          float64
	ThroughputRPS        float64
	MemoryEfficiency     float64
}

type WorkerSelector interface {
	SelectWorker(workers []WorkerInfo, batch *Batch) int
}

// Metrics collection
type BatchMetrics struct {
	mu                   sync.RWMutex
	startTime           time.Time
	endTime             *time.Time
	totalBatches        int64
	processedBatches    int64
	failedBatches       int64
	totalItems          int64
	processedItems      int64
	failedItems         int64
	totalProcessingTime time.Duration
	averageBatchTime    time.Duration
	throughputBPS       float64
	throughputIPS       float64
	memoryPeakUsage     int64
	cpuUtilization      float64
	ioWaitTime          time.Duration
	queueSizes          map[string]int
	workerUtilization   map[int]float64
	errorsByType        map[string]int64
	performanceProfile  PerformanceProfile
}

type PerformanceProfile struct {
	BottleneckType    string        `json:"bottleneck_type"`
	OptimalBatchSize  int           `json:"optimal_batch_size"`
	OptimalWorkerCount int          `json:"optimal_worker_count"`
	RecommendedTuning map[string]interface{} `json:"recommended_tuning"`
}

type SchedulerMetrics struct {
	TotalScheduled   int64   `json:"total_scheduled"`
	AverageQueueTime time.Duration `json:"average_queue_time"`
	LoadBalance      float64 `json:"load_balance_efficiency"`
}

type LoadBalancerMetrics struct {
	WorkerDistribution map[int]int64 `json:"worker_distribution"`
	ImbalanceRatio     float64       `json:"imbalance_ratio"`
}

// Implementations

// CreateBatchProcessorTool creates and registers the batch processor tool
func CreateBatchProcessorTool() *tools.Tool {
	return &tools.Tool{
		ID:          "batch-processor",
		Name:        "BatchProcessor",
		Description: "High-performance batch processing with optimization strategies",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"data_size": {
					Type:        "integer",
					Description: "Total number of data items to process",
					Default:     10000,
					Minimum:     func() *float64 { v := 100.0; return &v }(),
					Maximum:     func() *float64 { v := 1000000.0; return &v }(),
				},
				"batch_size": {
					Type:        "integer",
					Description: "Number of items per batch",
					Default:     100,
					Minimum:     func() *float64 { v := 1.0; return &v }(),
					Maximum:     func() *float64 { v := 10000.0; return &v }(),
				},
				"worker_count": {
					Type:        "integer",
					Description: "Number of worker goroutines",
					Default:     runtime.NumCPU(),
					Minimum:     func() *float64 { v := 1.0; return &v }(),
					Maximum:     func() *float64 { v := 100.0; return &v }(),
				},
				"processing_type": {
					Type:        "string",
					Description: "Type of processing strategy",
					Enum:        []interface{}{"sequential", "parallel", "pipeline", "adaptive"},
					Default:     "parallel",
				},
				"memory_limit_mb": {
					Type:        "integer",
					Description: "Memory limit in MB (0 = no limit)",
					Default:     512,
				},
				"buffer_size": {
					Type:        "integer",
					Description: "Buffer size for queues",
					Default:     1000,
					Minimum:     func() *float64 { v := 10.0; return &v }(),
					Maximum:     func() *float64 { v := 100000.0; return &v }(),
				},
				"compression_type": {
					Type:        "string",
					Description: "Compression algorithm for data",
					Enum:        []interface{}{"none", "gzip", "lz4", "snappy"},
					Default:     "none",
				},
				"enable_metrics": {
					Type:        "boolean",
					Description: "Enable detailed metrics collection",
					Default:     true,
				},
				"optimize_for": {
					Type:        "string",
					Description: "Optimization target",
					Enum:        []interface{}{"throughput", "latency", "memory", "cpu"},
					Default:     "throughput",
				},
				"enable_prefetch": {
					Type:        "boolean",
					Description: "Enable data prefetching",
					Default:     true,
				},
				"chunking_strategy": {
					Type:        "string",
					Description: "Data chunking strategy",
					Enum:        []interface{}{"fixed", "dynamic", "adaptive"},
					Default:     "adaptive",
				},
			},
			Required: []string{"data_size"},
		},
		Executor: &BatchProcessorTool{},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  false,
			Async:      true,
			Cancelable: true,
			Cacheable:  false,
			Timeout:    30 * time.Minute,
		},
		Metadata: &tools.ToolMetadata{
			Author:   "Performance Team",
			License:  "MIT",
			Tags:     []string{"performance", "batch", "processing", "optimization"},
			Examples: []tools.ToolExample{
				{
					Name:        "High Throughput Processing",
					Description: "Process large dataset optimized for throughput",
					Input: map[string]interface{}{
						"data_size":       100000,
						"batch_size":      500,
						"worker_count":    16,
						"processing_type": "parallel",
						"optimize_for":    "throughput",
						"enable_prefetch": true,
					},
				},
				{
					Name:        "Low Latency Processing",
					Description: "Process data optimized for low latency",
					Input: map[string]interface{}{
						"data_size":       10000,
						"batch_size":      50,
						"worker_count":    8,
						"processing_type": "pipeline",
						"optimize_for":    "latency",
						"memory_limit_mb": 256,
					},
				},
			},
		},
	}
}

// Execute runs the batch processing with optimization
func (t *BatchProcessorTool) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	// Parse parameters
	p, err := parseBatchParams(params)
	if err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Create batch processor
	processor := NewBatchProcessor(ctx, p)
	defer processor.Close()

	// Initialize components
	if err := processor.Initialize(); err != nil {
		return &tools.ToolExecutionResult{
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}, nil
	}

	// Run processing
	results, err := processor.Process()
	if err != nil {
		return &tools.ToolExecutionResult{
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}, nil
	}

	// Collect metrics and analysis
	metrics := processor.GetMetrics()
	analysis := processor.AnalyzePerformance()
	recommendations := generateBatchRecommendations(metrics, analysis, p)

	response := map[string]interface{}{
		"processing_results": results,
		"performance_metrics": metrics,
		"analysis":           analysis,
		"recommendations":    recommendations,
		"configuration":      p,
		"optimization_report": processor.GetOptimizationReport(),
	}

	return &tools.ToolExecutionResult{
		Success:   true,
		Data:      response,
		Timestamp: time.Now(),
		Duration:  metrics.totalProcessingTime,
		Metadata: map[string]interface{}{
			"batches_processed":  metrics.processedBatches,
			"items_processed":    metrics.processedItems,
			"throughput_bps":     metrics.throughputBPS,
			"throughput_ips":     metrics.throughputIPS,
			"memory_peak":        metrics.memoryPeakUsage,
			"success_rate":       float64(metrics.processedItems) / float64(metrics.totalItems) * 100,
		},
	}, nil
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(ctx context.Context, params *BatchProcessorParams) *BatchProcessor {
	procCtx, cancel := context.WithCancel(ctx)
	
	return &BatchProcessor{
		params:        params,
		ctx:           procCtx,
		cancel:        cancel,
		state:         StateIdle,
		memoryManager: NewMemoryManager(params.MemoryLimit * 1024 * 1024),
		metrics:       NewBatchMetrics(),
	}
}

// Initialize sets up the batch processor components
func (bp *BatchProcessor) Initialize() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	
	bp.state = StateInitializing
	
	// Initialize data source
	bp.dataSource = NewSyntheticDataSource(bp.params.DataSize, bp.params.ChunkingStrategy)
	
	// Initialize processors
	bp.processors = make([]DataProcessor, bp.params.WorkerCount)
	for i := 0; i < bp.params.WorkerCount; i++ {
		bp.processors[i] = NewOptimizedProcessor(i, bp.params.OptimizeFor)
	}
	
	// Initialize output sink
	bp.outputSink = NewInMemorySink(bp.params.BufferSize)
	
	// Initialize compressor if needed
	if bp.params.CompressionType != "none" {
		bp.compressor = NewCompressor(bp.params.CompressionType)
	}
	
	// Initialize prefetcher if enabled
	if bp.params.EnablePrefetch {
		bp.prefetcher = NewDataPrefetcher(bp.ctx, bp.dataSource, bp.params.BufferSize)
		bp.prefetcher.Start()
	}
	
	// Initialize scheduler
	bp.scheduler = NewBatchScheduler(bp.params.ProcessingType, bp.params.WorkerCount)
	
	bp.state = StateIdle
	return nil
}

// Process executes the batch processing
func (bp *BatchProcessor) Process() (*ProcessingResults, error) {
	bp.mu.Lock()
	bp.state = StateProcessing
	bp.metrics.startTime = time.Now()
	bp.mu.Unlock()
	
	defer func() {
		bp.mu.Lock()
		endTime := time.Now()
		bp.metrics.endTime = &endTime
		bp.metrics.totalProcessingTime = endTime.Sub(bp.metrics.startTime)
		bp.state = StateCompleted
		bp.mu.Unlock()
	}()
	
	// Process based on strategy
	switch bp.params.ProcessingType {
	case "sequential":
		return bp.processSequential()
	case "parallel":
		return bp.processParallel()
	case "pipeline":
		return bp.processPipeline()
	case "adaptive":
		return bp.processAdaptive()
	default:
		return nil, fmt.Errorf("unsupported processing type: %s", bp.params.ProcessingType)
	}
}

// processParallel implements parallel batch processing
func (bp *BatchProcessor) processParallel() (*ProcessingResults, error) {
	results := &ProcessingResults{
		Results: make([]*ProcessingResult, 0),
		Summary: &ProcessingSummary{},
	}
	
	var resultsLock sync.Mutex
	var errorCount int64
	
	// Worker pool
	workChan := make(chan *Batch, bp.params.BufferSize)
	resultChan := make(chan *ProcessingResult, bp.params.BufferSize)
	
	// Start workers
	for i := 0; i < bp.params.WorkerCount; i++ {
		bp.wg.Add(1)
		go func(workerID int) {
			defer bp.wg.Done()
			
			for batch := range workChan {
				select {
				case <-bp.ctx.Done():
					return
				default:
				}
				
				// Process batch
				result, err := bp.processors[workerID].Process(bp.ctx, batch)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					result = &ProcessingResult{
						BatchID: batch.ID,
						Success: false,
						Error:   err.Error(),
						WorkerID: workerID,
					}
				}
				
				// Send result
				select {
				case resultChan <- result:
				case <-bp.ctx.Done():
					return
				}
			}
		}(i)
	}
	
	// Result collector
	go func() {
		for result := range resultChan {
			resultsLock.Lock()
			results.Results = append(results.Results, result)
			bp.updateMetrics(result)
			resultsLock.Unlock()
			
			// Write to output sink
			if err := bp.outputSink.Write(result); err != nil {
				// Log error but continue processing
			}
		}
	}()
	
	// Feed batches to workers
	batchCount := 0
	for bp.dataSource.HasMore() {
		batch, err := bp.getBatch()
		if err != nil {
			break
		}
		
		if batch == nil {
			break
		}
		
		select {
		case workChan <- batch:
			batchCount++
			atomic.AddInt64(&bp.metrics.totalBatches, 1)
		case <-bp.ctx.Done():
			close(workChan)
			bp.wg.Wait()
			close(resultChan)
			return results, bp.ctx.Err()
		}
	}
	
	// Close channels and wait for completion
	close(workChan)
	bp.wg.Wait()
	close(resultChan)
	
	// Finalize results
	results.Summary = bp.calculateSummary(results.Results)
	
	return results, nil
}

// getBatch retrieves the next batch, using prefetcher if available
func (bp *BatchProcessor) getBatch() (*Batch, error) {
	if bp.prefetcher != nil && bp.prefetcher.enabled {
		return bp.prefetcher.GetBatch()
	}
	return bp.dataSource.GetBatch(bp.params.BatchSize)
}

// updateMetrics updates processing metrics
func (bp *BatchProcessor) updateMetrics(result *ProcessingResult) {
	bp.metrics.mu.Lock()
	defer bp.metrics.mu.Unlock()
	
	if result.Success {
		bp.metrics.processedBatches++
		bp.metrics.processedItems += int64(len(result.ItemResults))
	} else {
		bp.metrics.failedBatches++
		bp.metrics.errorsByType[result.Error]++
	}
	
	bp.metrics.totalProcessingTime += result.ProcessingTime
	
	if result.MemoryUsed > bp.metrics.memoryPeakUsage {
		bp.metrics.memoryPeakUsage = result.MemoryUsed
	}
	
	// Update worker utilization
	if bp.metrics.workerUtilization == nil {
		bp.metrics.workerUtilization = make(map[int]float64)
	}
	bp.metrics.workerUtilization[result.WorkerID] = calculateWorkerUtilization(result)
}

// Additional processing strategies (simplified implementations)

func (bp *BatchProcessor) processSequential() (*ProcessingResults, error) {
	// Simplified sequential processing
	results := &ProcessingResults{Results: make([]*ProcessingResult, 0)}
	
	for bp.dataSource.HasMore() {
		batch, err := bp.getBatch()
		if err != nil || batch == nil {
			break
		}
		
		result, err := bp.processors[0].Process(bp.ctx, batch)
		if err != nil {
			result = &ProcessingResult{
				BatchID: batch.ID,
				Success: false,
				Error:   err.Error(),
			}
		}
		
		results.Results = append(results.Results, result)
		bp.updateMetrics(result)
		
		if err := bp.outputSink.Write(result); err != nil {
			// Handle error
		}
	}
	
	results.Summary = bp.calculateSummary(results.Results)
	return results, nil
}

func (bp *BatchProcessor) processPipeline() (*ProcessingResults, error) {
	// Simplified pipeline processing
	return bp.processParallel() // For now, use parallel as base
}

func (bp *BatchProcessor) processAdaptive() (*ProcessingResults, error) {
	// Adaptive processing that switches strategies based on performance
	return bp.processParallel() // For now, use parallel as base
}

// Helper functions and supporting implementations

func parseBatchParams(params map[string]interface{}) (*BatchProcessorParams, error) {
	p := &BatchProcessorParams{
		DataSize:         10000,
		BatchSize:        100,
		WorkerCount:      runtime.NumCPU(),
		ProcessingType:   "parallel",
		MemoryLimit:      512,
		BufferSize:       1000,
		CompressionType:  "none",
		EnableMetrics:    true,
		OptimizeFor:      "throughput",
		EnablePrefetch:   true,
		ChunkingStrategy: "adaptive",
	}
	
	// Parse parameters (simplified)
	if v, ok := params["data_size"].(int); ok {
		p.DataSize = v
	}
	if v, ok := params["batch_size"].(int); ok {
		p.BatchSize = v
	}
	if v, ok := params["worker_count"].(int); ok {
		p.WorkerCount = v
	}
	if v, ok := params["processing_type"].(string); ok {
		p.ProcessingType = v
	}
	// ... parse other parameters
	
	return p, nil
}

func NewMemoryManager(maxMemory int64) *MemoryManager {
	return &MemoryManager{
		maxMemory: maxMemory,
		pools:     make(map[string]*ObjectPool),
		gc: &GCManager{
			threshold: maxMemory / 2,
			interval:  time.Minute,
		},
		metrics: &MemoryMetrics{},
	}
}

func NewBatchMetrics() *BatchMetrics {
	return &BatchMetrics{
		queueSizes:         make(map[string]int),
		workerUtilization:  make(map[int]float64),
		errorsByType:       make(map[string]int64),
	}
}

// Simplified implementations for supporting types

type SyntheticDataSource struct {
	totalSize int
	currentIndex int
	chunkingStrategy string
}

func NewSyntheticDataSource(size int, strategy string) *SyntheticDataSource {
	return &SyntheticDataSource{
		totalSize:        size,
		chunkingStrategy: strategy,
	}
}

func (s *SyntheticDataSource) GetBatch(size int) (*Batch, error) {
	if !s.HasMore() {
		return nil, nil
	}
	
	remainingItems := s.totalSize - s.currentIndex
	batchSize := size
	if remainingItems < batchSize {
		batchSize = remainingItems
	}
	
	items := make([]DataItem, batchSize)
	for i := 0; i < batchSize; i++ {
		items[i] = DataItem{
			ID:        fmt.Sprintf("item-%d", s.currentIndex+i),
			Data:      generateSyntheticData(),
			Size:      rand.Int63n(1024) + 100,
			Timestamp: time.Now(),
		}
	}
	
	batch := &Batch{
		ID:        fmt.Sprintf("batch-%d", s.currentIndex/size),
		Items:     items,
		Size:      batchSize,
		CreatedAt: time.Now(),
	}
	
	s.currentIndex += batchSize
	return batch, nil
}

func (s *SyntheticDataSource) HasMore() bool {
	return s.currentIndex < s.totalSize
}

func (s *SyntheticDataSource) TotalSize() int {
	return s.totalSize
}

func (s *SyntheticDataSource) Reset() error {
	s.currentIndex = 0
	return nil
}

func (s *SyntheticDataSource) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"total_size":     s.totalSize,
		"current_index":  s.currentIndex,
		"progress":       float64(s.currentIndex) / float64(s.totalSize) * 100,
	}
}

type OptimizedProcessor struct {
	id          int
	optimizeFor string
	metrics     map[string]interface{}
}

func NewOptimizedProcessor(id int, optimizeFor string) *OptimizedProcessor {
	return &OptimizedProcessor{
		id:          id,
		optimizeFor: optimizeFor,
		metrics:     make(map[string]interface{}),
	}
}

func (p *OptimizedProcessor) Process(ctx context.Context, batch *Batch) (*ProcessingResult, error) {
	start := time.Now()
	
	itemResults := make([]ItemResult, len(batch.Items))
	
	// Simulate processing based on optimization target
	for i, item := range batch.Items {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		// Simulate different processing patterns
		var processingTime time.Duration
		switch p.optimizeFor {
		case "throughput":
			processingTime = time.Microsecond * 100 // Fast processing
		case "latency":
			processingTime = time.Microsecond * 50  // Very fast
		case "memory":
			processingTime = time.Microsecond * 200 // More CPU, less memory
		case "cpu":
			processingTime = time.Microsecond * 150 // Balanced
		default:
			processingTime = time.Microsecond * 100
		}
		
		time.Sleep(processingTime)
		
		itemResults[i] = ItemResult{
			ItemID:         item.ID,
			Success:        true,
			Result:         fmt.Sprintf("processed-%s", item.ID),
			ProcessingTime: processingTime,
		}
	}
	
	totalTime := time.Since(start)
	
	// Get memory usage
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	return &ProcessingResult{
		BatchID:        batch.ID,
		ItemResults:    itemResults,
		Success:        true,
		ProcessingTime: totalTime,
		MemoryUsed:     int64(memStats.Alloc),
		WorkerID:       p.id,
	}, nil
}

func (p *OptimizedProcessor) GetCapabilities() ProcessorCapabilities {
	return ProcessorCapabilities{
		MaxBatchSize:     1000,
		OptimalBatchSize: 100,
		MemoryPerItem:    1024,
		ProcessingTime:   time.Microsecond * 100,
		SupportsParallel: true,
		CPUIntensive:     p.optimizeFor == "cpu",
		IOIntensive:      p.optimizeFor == "latency",
	}
}

func (p *OptimizedProcessor) GetMetrics() map[string]interface{} {
	return p.metrics
}

func (p *OptimizedProcessor) Cleanup() error {
	return nil
}

type InMemorySink struct {
	buffer   []*ProcessingResult
	capacity int
	mu       sync.Mutex
	metrics  map[string]interface{}
}

func NewInMemorySink(capacity int) *InMemorySink {
	return &InMemorySink{
		buffer:   make([]*ProcessingResult, 0, capacity),
		capacity: capacity,
		metrics:  make(map[string]interface{}),
	}
}

func (s *InMemorySink) Write(result *ProcessingResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if len(s.buffer) >= s.capacity {
		// Remove oldest result
		s.buffer = s.buffer[1:]
	}
	
	s.buffer = append(s.buffer, result)
	return nil
}

func (s *InMemorySink) Flush() error {
	return nil
}

func (s *InMemorySink) GetMetrics() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	return map[string]interface{}{
		"buffer_size": len(s.buffer),
		"capacity":    s.capacity,
		"utilization": float64(len(s.buffer)) / float64(s.capacity) * 100,
	}
}

func (s *InMemorySink) Close() error {
	s.buffer = nil
	return nil
}

// Additional supporting types and methods...

type ProcessingResults struct {
	Results []*ProcessingResult `json:"results"`
	Summary *ProcessingSummary  `json:"summary"`
}

type ProcessingSummary struct {
	TotalBatches    int           `json:"total_batches"`
	SuccessfulBatches int         `json:"successful_batches"`
	FailedBatches   int           `json:"failed_batches"`
	TotalItems      int           `json:"total_items"`
	ProcessingTime  time.Duration `json:"processing_time"`
	ThroughputBPS   float64       `json:"throughput_bps"`
	ThroughputIPS   float64       `json:"throughput_ips"`
	SuccessRate     float64       `json:"success_rate"`
}

func generateSyntheticData() map[string]interface{} {
	return map[string]interface{}{
		"value":     rand.Intn(1000),
		"text":      fmt.Sprintf("data-%d", rand.Intn(10000)),
		"timestamp": time.Now(),
		"metadata":  map[string]string{"type": "synthetic"},
	}
}

func calculateWorkerUtilization(result *ProcessingResult) float64 {
	// Simplified calculation
	return float64(len(result.ItemResults)) / 100.0 * 100
}

func (bp *BatchProcessor) calculateSummary(results []*ProcessingResult) *ProcessingSummary {
	successful := 0
	failed := 0
	totalItems := 0
	var totalTime time.Duration
	
	for _, result := range results {
		if result.Success {
			successful++
		} else {
			failed++
		}
		totalItems += len(result.ItemResults)
		totalTime += result.ProcessingTime
	}
	
	summary := &ProcessingSummary{
		TotalBatches:      len(results),
		SuccessfulBatches: successful,
		FailedBatches:     failed,
		TotalItems:        totalItems,
		ProcessingTime:    totalTime,
	}
	
	if totalTime > 0 {
		summary.ThroughputBPS = float64(successful) / totalTime.Seconds()
		summary.ThroughputIPS = float64(totalItems) / totalTime.Seconds()
	}
	
	if len(results) > 0 {
		summary.SuccessRate = float64(successful) / float64(len(results)) * 100
	}
	
	return summary
}

func (bp *BatchProcessor) GetMetrics() *BatchMetrics {
	bp.metrics.mu.RLock()
	defer bp.metrics.mu.RUnlock()
	
	// Calculate derived metrics
	if bp.metrics.processedBatches > 0 {
		bp.metrics.averageBatchTime = bp.metrics.totalProcessingTime / time.Duration(bp.metrics.processedBatches)
	}
	
	if bp.metrics.totalProcessingTime > 0 {
		bp.metrics.throughputBPS = float64(bp.metrics.processedBatches) / bp.metrics.totalProcessingTime.Seconds()
		bp.metrics.throughputIPS = float64(bp.metrics.processedItems) / bp.metrics.totalProcessingTime.Seconds()
	}
	
	return bp.metrics
}

func (bp *BatchProcessor) AnalyzePerformance() map[string]interface{} {
	metrics := bp.GetMetrics()
	
	analysis := map[string]interface{}{
		"efficiency_score":     calculateEfficiencyScore(metrics),
		"bottleneck_analysis":  identifyBottlenecks(metrics),
		"resource_utilization": calculateResourceUtilization(metrics),
		"scaling_potential":    assessScalingPotential(metrics, bp.params),
	}
	
	return analysis
}

func (bp *BatchProcessor) GetOptimizationReport() map[string]interface{} {
	return map[string]interface{}{
		"current_performance": bp.GetMetrics(),
		"optimization_opportunities": identifyOptimizations(bp.GetMetrics(), bp.params),
		"recommended_settings": recommendOptimalSettings(bp.GetMetrics(), bp.params),
	}
}

// Additional helper functions...

func calculateEfficiencyScore(metrics *BatchMetrics) float64 {
	score := 100.0
	
	// Penalize for failures
	if metrics.totalBatches > 0 {
		failureRate := float64(metrics.failedBatches) / float64(metrics.totalBatches)
		score -= failureRate * 50
	}
	
	// Penalize for low throughput
	if metrics.throughputBPS < 10 {
		score -= 25
	}
	
	return max(0, score)
}

func identifyBottlenecks(metrics *BatchMetrics) map[string]string {
	bottlenecks := make(map[string]string)
	
	if metrics.cpuUtilization > 90 {
		bottlenecks["cpu"] = "High CPU utilization detected"
	}
	
	if metrics.memoryPeakUsage > 1024*1024*1024 { // 1GB
		bottlenecks["memory"] = "High memory usage detected"
	}
	
	if metrics.ioWaitTime > metrics.totalProcessingTime/10 {
		bottlenecks["io"] = "High I/O wait time detected"
	}
	
	return bottlenecks
}

func calculateResourceUtilization(metrics *BatchMetrics) map[string]float64 {
	return map[string]float64{
		"cpu":    metrics.cpuUtilization,
		"memory": float64(metrics.memoryPeakUsage) / (1024 * 1024 * 1024), // GB
		"workers": calculateAverageWorkerUtilization(metrics.workerUtilization),
	}
}

func calculateAverageWorkerUtilization(utilization map[int]float64) float64 {
	if len(utilization) == 0 {
		return 0
	}
	
	total := 0.0
	for _, util := range utilization {
		total += util
	}
	
	return total / float64(len(utilization))
}

func assessScalingPotential(metrics *BatchMetrics, params *BatchProcessorParams) map[string]interface{} {
	return map[string]interface{}{
		"can_scale_workers":     metrics.cpuUtilization < 70,
		"can_increase_batch_size": metrics.memoryPeakUsage < 512*1024*1024,
		"bottleneck_resource":   identifyPrimaryBottleneck(metrics),
	}
}

func identifyPrimaryBottleneck(metrics *BatchMetrics) string {
	if metrics.cpuUtilization > 85 {
		return "cpu"
	}
	if metrics.memoryPeakUsage > 1024*1024*1024 {
		return "memory"
	}
	if metrics.ioWaitTime > metrics.totalProcessingTime/5 {
		return "io"
	}
	return "none"
}

func identifyOptimizations(metrics *BatchMetrics, params *BatchProcessorParams) []string {
	optimizations := []string{}
	
	if metrics.throughputBPS < 50 {
		optimizations = append(optimizations, "Consider increasing batch size for better throughput")
	}
	
	if calculateAverageWorkerUtilization(metrics.workerUtilization) < 70 {
		optimizations = append(optimizations, "Worker utilization is low - consider reducing worker count")
	}
	
	if metrics.memoryPeakUsage > 1024*1024*1024 {
		optimizations = append(optimizations, "High memory usage - consider enabling compression or reducing batch size")
	}
	
	return optimizations
}

func recommendOptimalSettings(metrics *BatchMetrics, params *BatchProcessorParams) map[string]interface{} {
	recommendations := map[string]interface{}{
		"batch_size":    params.BatchSize, // Keep current as baseline
		"worker_count":  params.WorkerCount,
	}
	
	// Adjust recommendations based on metrics
	if metrics.throughputBPS < 10 {
		recommendations["batch_size"] = params.BatchSize * 2
	}
	
	if calculateAverageWorkerUtilization(metrics.workerUtilization) > 90 {
		recommendations["worker_count"] = params.WorkerCount + 2
	}
	
	return recommendations
}

func generateBatchRecommendations(metrics *BatchMetrics, analysis map[string]interface{}, params *BatchProcessorParams) []string {
	recommendations := []string{}
	
	if score, ok := analysis["efficiency_score"].(float64); ok && score < 70 {
		recommendations = append(recommendations, "Overall efficiency is low - review processing logic and resource allocation")
	}
	
	if bottlenecks, ok := analysis["bottleneck_analysis"].(map[string]string); ok && len(bottlenecks) > 0 {
		for resource, description := range bottlenecks {
			recommendations = append(recommendations, fmt.Sprintf("%s bottleneck: %s", resource, description))
		}
	}
	
	if metrics.failedBatches > 0 {
		recommendations = append(recommendations, "Some batches failed - implement retry mechanism and better error handling")
	}
	
	return recommendations
}

// Additional supporting implementations...

func NewDataPrefetcher(ctx context.Context, source DataSource, bufferSize int) *DataPrefetcher {
	prefetchCtx, cancel := context.WithCancel(ctx)
	
	return &DataPrefetcher{
		enabled:       true,
		bufferSize:    bufferSize,
		prefetchQueue: make(chan *Batch, bufferSize),
		dataSource:    source,
		ctx:           prefetchCtx,
		cancel:        cancel,
		metrics:       &PrefetchMetrics{},
	}
}

func (pf *DataPrefetcher) Start() {
	go pf.prefetchLoop()
}

func (pf *DataPrefetcher) prefetchLoop() {
	// Simplified prefetch implementation
	for pf.dataSource.HasMore() {
		select {
		case <-pf.ctx.Done():
			return
		default:
		}
		
		batch, err := pf.dataSource.GetBatch(100) // Fixed size for prefetch
		if err != nil || batch == nil {
			continue
		}
		
		select {
		case pf.prefetchQueue <- batch:
			pf.metrics.BatchesPrefetched++
		case <-pf.ctx.Done():
			return
		}
	}
	
	close(pf.prefetchQueue)
}

func (pf *DataPrefetcher) GetBatch() (*Batch, error) {
	select {
	case batch, ok := <-pf.prefetchQueue:
		if !ok {
			return nil, nil // Channel closed
		}
		return batch, nil
	case <-pf.ctx.Done():
		return nil, pf.ctx.Err()
	}
}

func NewBatchScheduler(strategy string, workerCount int) *BatchScheduler {
	return &BatchScheduler{
		workQueue: make(chan *Batch, workerCount*2),
		metrics:   &SchedulerMetrics{},
	}
}

func NewCompressor(compressionType string) Compressor {
	// Return a mock compressor for this example
	return &MockCompressor{compressionType: compressionType}
}

type MockCompressor struct {
	compressionType string
}

func (c *MockCompressor) Compress(data []byte) ([]byte, error) {
	// Mock compression - just return the data
	return data, nil
}

func (c *MockCompressor) Decompress(data []byte) ([]byte, error) {
	// Mock decompression - just return the data
	return data, nil
}

func (c *MockCompressor) GetCompressionRatio() float64 {
	// Mock compression ratio
	switch c.compressionType {
	case "gzip":
		return 0.6
	case "lz4":
		return 0.8
	case "snappy":
		return 0.7
	default:
		return 1.0
	}
}

func (bp *BatchProcessor) Close() {
	bp.cancel()
	
	if bp.prefetcher != nil {
		bp.prefetcher.cancel()
	}
	
	for _, processor := range bp.processors {
		processor.Cleanup()
	}
	
	if bp.outputSink != nil {
		bp.outputSink.Close()
	}
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func main() {
<<<<<<<< HEAD:go-sdk/examples/tools/performance/batch-processor/main.go
========
	_ = CreateBatchProcessorTool()
	
>>>>>>>> main:go-sdk/examples/tools/performance/cmd/batch_processor/main.go
	// Example usage
	params := map[string]interface{}{
		"data_size":         10000,
		"batch_size":        100,
		"worker_count":      8,
		"processing_type":   "parallel",
		"optimize_for":      "throughput",
		"enable_prefetch":   true,
		"memory_limit_mb":   512,
	}
	
	ctx := context.Background()
	processor := &BatchProcessorTool{}
	
	result, err := processor.Execute(ctx, params)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	fmt.Printf("Batch Processing Results:\n")
	fmt.Printf("Success: %t\n", result.Success)
	fmt.Printf("Duration: %v\n", result.Duration)
	
	if data, ok := result.Data.(map[string]interface{}); ok {
		if metrics, ok := data["performance_metrics"].(*BatchMetrics); ok {
			fmt.Printf("Batches Processed: %d\n", metrics.processedBatches)
			fmt.Printf("Items Processed: %d\n", metrics.processedItems)
			fmt.Printf("Throughput: %.2f batches/sec\n", metrics.throughputBPS)
			fmt.Printf("Item Throughput: %.2f items/sec\n", metrics.throughputIPS)
		}
		
		if recs, ok := data["recommendations"].([]string); ok {
			fmt.Printf("Recommendations:\n")
			for _, rec := range recs {
				fmt.Printf("  - %s\n", rec)
			}
		}
	}
}