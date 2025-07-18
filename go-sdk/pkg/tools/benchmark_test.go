package tools

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// BenchmarkExecutionEngine provides comprehensive benchmarking for the ExecutionEngine
func BenchmarkExecutionEngine(b *testing.B) {
	b.Run("SingleExecution", benchmarkSingleExecution)
	b.Run("ConcurrentExecution", benchmarkConcurrentExecution)
	b.Run("HighConcurrency", benchmarkHighConcurrency)
	b.Run("MemoryUsage", benchmarkMemoryUsage)
	b.Run("Throughput", benchmarkThroughput)
	b.Run("Latency", benchmarkLatency)
	b.Run("ScalabilityByToolCount", benchmarkScalabilityByToolCount)
	b.Run("StressTest", benchmarkStressTest)
}

// BenchmarkRegistry provides comprehensive benchmarking for the Registry
func BenchmarkRegistry(b *testing.B) {
	b.Run("Registration", benchmarkRegistration)
	b.Run("Lookup", benchmarkLookup)
	b.Run("LookupByName", benchmarkLookupByName)
	b.Run("ListAll", benchmarkListAll)
	b.Run("ListFiltered", benchmarkListFiltered)
	b.Run("ConcurrentAccess", benchmarkConcurrentAccess)
	b.Run("ScalabilityByRegistrySize", benchmarkScalabilityByRegistrySize)
	b.Run("DependencyResolution", benchmarkDependencyResolution)
}

// BenchmarkTool provides comprehensive benchmarking for Tool operations
func BenchmarkTool(b *testing.B) {
	b.Run("Validation", benchmarkToolValidation)
	b.Run("Cloning", benchmarkToolCloning)
	b.Run("Serialization", benchmarkToolSerialization)
	b.Run("SchemaValidation", benchmarkSchemaValidation)
}

// Single execution benchmarks
func benchmarkSingleExecution(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry)
	
	tool := createBenchmarkTool("bench-tool", 10*time.Millisecond)
	if err := registry.Register(tool); err != nil {
		b.Fatal(err)
	}
	
	params := map[string]interface{}{
		"input": "benchmark test",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := engine.Execute(context.Background(), tool.ID, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Concurrent execution benchmarks
func benchmarkConcurrentExecution(b *testing.B) {
	concurrencyLevels := []int{1, 2, 4, 8, 16, 32, 64, 128}
	
	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency%d", concurrency), func(b *testing.B) {
			registry := NewRegistry()
			engine := NewExecutionEngine(registry, WithMaxConcurrent(concurrency))
			
			tool := createBenchmarkTool("bench-tool", 1*time.Millisecond)
			if err := registry.Register(tool); err != nil {
				b.Fatal(err)
			}
			
			params := map[string]interface{}{
				"input": "benchmark test",
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			b.SetParallelism(concurrency)
			
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_, err := engine.Execute(context.Background(), tool.ID, params)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// High concurrency stress test
func benchmarkHighConcurrency(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(1000))
	
	// Create multiple tools
	tools := make([]*Tool, 100)
	for i := 0; i < 100; i++ {
		tools[i] = createBenchmarkTool(fmt.Sprintf("bench-tool-%d", i), time.Duration(rand.Intn(10))*time.Millisecond)
		if err := registry.Register(tools[i]); err != nil {
			b.Fatal(err)
		}
	}
	
	params := map[string]interface{}{
		"input": "benchmark test",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	var wg sync.WaitGroup
	workers := 100
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			wg.Add(workers)
			for i := 0; i < workers; i++ {
				go func() {
					defer wg.Done()
					tool := tools[rand.Intn(len(tools))]
					_, err := engine.Execute(context.Background(), tool.ID, params)
					if err != nil {
						b.Error(err)
					}
				}()
			}
			wg.Wait()
		}
	})
}

// Memory usage benchmark
func benchmarkMemoryUsage(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry)
	
	tool := createBenchmarkTool("bench-tool", 1*time.Millisecond)
	if err := registry.Register(tool); err != nil {
		b.Fatal(err)
	}
	
	params := map[string]interface{}{
		"input": "benchmark test",
	}
	
	// Force GC before starting
	runtime.GC()
	runtime.GC()
	
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := engine.Execute(context.Background(), tool.ID, params)
		if err != nil {
			b.Fatal(err)
		}
	}
	
	b.StopTimer()
	
	runtime.GC()
	runtime.GC()
	
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	
	b.ReportMetric(float64(after.Alloc-before.Alloc)/float64(b.N), "bytes/op")
	b.ReportMetric(float64(after.Mallocs-before.Mallocs)/float64(b.N), "allocs/op")
}

// Throughput benchmark
func benchmarkThroughput(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(100))
	
	tool := createBenchmarkTool("bench-tool", 1*time.Millisecond)
	if err := registry.Register(tool); err != nil {
		b.Fatal(err)
	}
	
	params := map[string]interface{}{
		"input": "benchmark test",
	}
	
	duration := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	var ops int64
	var wg sync.WaitGroup
	
	b.ResetTimer()
	
	// Start multiple workers
	workers := 50
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					_, err := engine.Execute(ctx, tool.ID, params)
					if err == nil {
						atomic.AddInt64(&ops, 1)
					}
				}
			}
		}()
	}
	
	wg.Wait()
	
	b.StopTimer()
	
	throughput := float64(ops) / duration.Seconds()
	b.ReportMetric(throughput, "ops/sec")
}

// Latency benchmark
func benchmarkLatency(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry)
	
	tool := createBenchmarkTool("bench-tool", 5*time.Millisecond)
	if err := registry.Register(tool); err != nil {
		b.Fatal(err)
	}
	
	params := map[string]interface{}{
		"input": "benchmark test",
	}
	
	latencies := make([]time.Duration, b.N)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		start := time.Now()
		_, err := engine.Execute(context.Background(), tool.ID, params)
		if err != nil {
			b.Fatal(err)
		}
		latencies[i] = time.Since(start)
	}
	
	b.StopTimer()
	
	// Calculate percentiles
	sortDurations(latencies)
	
	p50 := latencies[len(latencies)*50/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[len(latencies)*99/100]
	
	b.ReportMetric(float64(p50.Nanoseconds())/1e6, "p50_ms")
	b.ReportMetric(float64(p95.Nanoseconds())/1e6, "p95_ms")
	b.ReportMetric(float64(p99.Nanoseconds())/1e6, "p99_ms")
}

// Scalability by tool count benchmark
func benchmarkScalabilityByToolCount(b *testing.B) {
	toolCounts := []int{1, 10, 100, 1000, 10000}
	
	for _, toolCount := range toolCounts {
		b.Run(fmt.Sprintf("Tools%d", toolCount), func(b *testing.B) {
			registry := NewRegistry()
			engine := NewExecutionEngine(registry)
			
			// Create tools
			tools := make([]*Tool, toolCount)
			for i := 0; i < toolCount; i++ {
				tools[i] = createBenchmarkTool(fmt.Sprintf("bench-tool-%d", i), 1*time.Millisecond)
				if err := registry.Register(tools[i]); err != nil {
					b.Fatal(err)
				}
			}
			
			params := map[string]interface{}{
				"input": "benchmark test",
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				// Execute random tool
				tool := tools[rand.Intn(len(tools))]
				_, err := engine.Execute(context.Background(), tool.ID, params)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Stress test benchmark
func benchmarkStressTest(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(500))
	
	// Create many tools
	tools := make([]*Tool, 1000)
	for i := 0; i < 1000; i++ {
		tools[i] = createBenchmarkTool(fmt.Sprintf("bench-tool-%d", i), time.Duration(rand.Intn(5))*time.Millisecond)
		if err := registry.Register(tools[i]); err != nil {
			b.Fatal(err)
		}
	}
	
	params := map[string]interface{}{
		"input": "benchmark test",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// High concurrency stress test
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tool := tools[rand.Intn(len(tools))]
			_, err := engine.Execute(context.Background(), tool.ID, params)
			if err != nil {
				b.Error(err)
			}
		}
	})
}

// Registry benchmarks
func benchmarkRegistration(b *testing.B) {
	registry := NewRegistry()
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		tool := createBenchmarkTool(fmt.Sprintf("bench-tool-%d", i), 1*time.Millisecond)
		b.StartTimer()
		
		if err := registry.Register(tool); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkLookup(b *testing.B) {
	registry := NewRegistry()
	
	// Pre-register tools
	tools := make([]*Tool, 10000)
	for i := 0; i < 10000; i++ {
		tools[i] = createBenchmarkTool(fmt.Sprintf("bench-tool-%d", i), 1*time.Millisecond)
		if err := registry.Register(tools[i]); err != nil {
			b.Fatal(err)
		}
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		toolID := fmt.Sprintf("bench-tool-%d", rand.Intn(10000))
		_, err := registry.Get(toolID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkLookupByName(b *testing.B) {
	registry := NewRegistry()
	
	// Pre-register tools
	tools := make([]*Tool, 10000)
	for i := 0; i < 10000; i++ {
		tools[i] = createBenchmarkTool(fmt.Sprintf("bench-tool-%d", i), 1*time.Millisecond)
		if err := registry.Register(tools[i]); err != nil {
			b.Fatal(err)
		}
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		toolName := fmt.Sprintf("Benchmark Tool %d", rand.Intn(10000))
		_, err := registry.GetByName(toolName)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkListAll(b *testing.B) {
	registry := NewRegistry()
	
	// Pre-register tools
	tools := make([]*Tool, 1000)
	for i := 0; i < 1000; i++ {
		tools[i] = createBenchmarkTool(fmt.Sprintf("bench-tool-%d", i), 1*time.Millisecond)
		if err := registry.Register(tools[i]); err != nil {
			b.Fatal(err)
		}
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := registry.ListAll()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkListFiltered(b *testing.B) {
	registry := NewRegistry()
	
	// Pre-register tools with tags
	tools := make([]*Tool, 1000)
	for i := 0; i < 1000; i++ {
		tool := createBenchmarkTool(fmt.Sprintf("bench-tool-%d", i), 1*time.Millisecond)
		tool.Metadata = &ToolMetadata{
			Tags: []string{fmt.Sprintf("tag-%d", i%10), "benchmark"},
		}
		tools[i] = tool
		if err := registry.Register(tool); err != nil {
			b.Fatal(err)
		}
	}
	
	filter := &ToolFilter{
		Tags: []string{"benchmark"},
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := registry.List(filter)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkConcurrentAccess(b *testing.B) {
	registry := NewRegistry()
	
	// Pre-register tools
	tools := make([]*Tool, 1000)
	for i := 0; i < 1000; i++ {
		tools[i] = createBenchmarkTool(fmt.Sprintf("bench-tool-%d", i), 1*time.Millisecond)
		if err := registry.Register(tools[i]); err != nil {
			b.Fatal(err)
		}
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Mix of operations
			switch rand.Intn(3) {
			case 0:
				// Get
				toolID := fmt.Sprintf("bench-tool-%d", rand.Intn(1000))
				registry.Get(toolID)
			case 1:
				// List
				registry.ListAll()
			case 2:
				// Register new tool
				newTool := createBenchmarkTool(fmt.Sprintf("new-bench-tool-%d", rand.Intn(10000)), 1*time.Millisecond)
				registry.Register(newTool)
			}
		}
	})
}

func benchmarkScalabilityByRegistrySize(b *testing.B) {
	registrySizes := []int{10, 100, 1000, 10000, 100000}
	
	for _, size := range registrySizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			registry := NewRegistry()
			
			// Pre-register tools
			tools := make([]*Tool, size)
			for i := 0; i < size; i++ {
				tools[i] = createBenchmarkTool(fmt.Sprintf("bench-tool-%d", i), 1*time.Millisecond)
				if err := registry.Register(tools[i]); err != nil {
					b.Fatal(err)
				}
			}
			
			b.ResetTimer()
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				toolID := fmt.Sprintf("bench-tool-%d", rand.Intn(size))
				_, err := registry.Get(toolID)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func benchmarkDependencyResolution(b *testing.B) {
	registry := NewRegistry()
	
	// Create tools with dependencies
	tools := make([]*Tool, 100)
	for i := 0; i < 100; i++ {
		tool := createBenchmarkTool(fmt.Sprintf("bench-tool-%d", i), 1*time.Millisecond)
		
		// Add dependencies
		if i > 0 {
			var deps []string
			for j := 0; j < i && j < 5; j++ {
				deps = append(deps, fmt.Sprintf("bench-tool-%d", j))
			}
			tool.Metadata = &ToolMetadata{
				Dependencies: deps,
			}
		}
		
		tools[i] = tool
		if err := registry.Register(tool); err != nil {
			b.Fatal(err)
		}
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		toolID := fmt.Sprintf("bench-tool-%d", rand.Intn(100))
		_, err := registry.GetDependencies(toolID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Tool operation benchmarks
func benchmarkToolValidation(b *testing.B) {
	tool := createBenchmarkTool("bench-tool", 1*time.Millisecond)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		err := tool.Validate()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkToolCloning(b *testing.B) {
	tool := createComplexBenchmarkTool("bench-tool", 1*time.Millisecond)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_ = tool.Clone()
	}
}

func benchmarkToolSerialization(b *testing.B) {
	tool := createComplexBenchmarkTool("bench-tool", 1*time.Millisecond)
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := tool.MarshalJSON()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkSchemaValidation(b *testing.B) {
	tool := createComplexBenchmarkTool("bench-tool", 1*time.Millisecond)
	validator := NewSchemaValidator(tool.Schema)
	
	params := map[string]interface{}{
		"input":   "test string",
		"number":  42,
		"boolean": true,
		"array":   []interface{}{"item1", "item2"},
		"object": map[string]interface{}{
			"nested": "value",
		},
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		err := validator.Validate(params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark helper functions
func createBenchmarkTool(id string, processingTime time.Duration) *Tool {
	return &Tool{
		ID:          id,
		Name:        fmt.Sprintf("Benchmark Tool %s", id),
		Description: "A tool for benchmark testing",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"input": {
					Type:        "string",
					Description: "Input string",
				},
			},
			Required: []string{"input"},
		},
		Executor: &BenchmarkExecutor{
			processingTime: processingTime,
		},
		Capabilities: &ToolCapabilities{
			Cacheable:  true,
			Cancelable: true,
			Retryable:  true,
			Timeout:    5 * time.Second,  // Reduced from 30s to 5s
		},
	}
}

func createComplexBenchmarkTool(id string, processingTime time.Duration) *Tool {
	return &Tool{
		ID:          id,
		Name:        fmt.Sprintf("Complex Benchmark Tool %s", id),
		Description: "A complex tool for benchmark testing with extensive schema",
		Version:     "2.1.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"input": {
					Type:        "string",
					Description: "Input string",
					MinLength:   intPtr(1),
					MaxLength:   intPtr(1000),
				},
				"number": {
					Type:        "integer",
					Description: "A number parameter",
					Minimum:     float64Ptr(0),
					Maximum:     float64Ptr(100),
				},
				"boolean": {
					Type:        "boolean",
					Description: "A boolean parameter",
				},
				"array": {
					Type:        "array",
					Description: "An array parameter",
					Items: &Property{
						Type: "string",
					},
					MinItems: intPtr(1),
					MaxItems: intPtr(10),
				},
				"object": {
					Type:        "object",
					Description: "A nested object parameter",
					Properties: map[string]*Property{
						"nested": {
							Type:        "string",
							Description: "Nested string",
						},
					},
					Required: []string{"nested"},
				},
			},
			Required: []string{"input", "number"},
		},
		Executor: &BenchmarkExecutor{
			processingTime: processingTime,
		},
		Capabilities: &ToolCapabilities{
			Streaming:  true,
			Async:      true,
			Cancelable: true,
			Retryable:  true,
			Cacheable:  true,
			RateLimit:  100,
			Timeout:    60 * time.Second,
		},
		Metadata: &ToolMetadata{
			Author:        "Benchmark Team",
			License:       "MIT",
			Documentation: "https://example.com/docs",
			Tags:          []string{"benchmark", "testing", "performance"},
			Dependencies:  []string{"base-tool", "utility-tool"},
			Examples: []ToolExample{
				{
					Name:        "Basic usage",
					Description: "Basic usage example",
					Input: map[string]interface{}{
						"input":   "hello world",
						"number":  42,
						"boolean": true,
						"array":   []interface{}{"item1", "item2"},
						"object": map[string]interface{}{
							"nested": "value",
						},
					},
					Output: map[string]interface{}{
						"result": "processed: hello world",
					},
				},
			},
			Custom: map[string]interface{}{
				"priority":    10,
				"category":    "benchmark",
				"performance": "high",
			},
		},
	}
}

// BenchmarkExecutor implements ToolExecutor for benchmarking
type BenchmarkExecutor struct {
	processingTime time.Duration
}

func (e *BenchmarkExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	// Simulate processing time
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(e.processingTime):
	}
	
	// Simulate some work
	result := make(map[string]interface{})
	result["processed"] = params["input"]
	result["timestamp"] = time.Now()
	result["processing_time"] = e.processingTime
	
	return &ToolExecutionResult{
		Success:   true,
		Data:      result,
		Timestamp: time.Now(),
	}, nil
}

// Utility functions
func intPtr(i int) *int {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}

func sortDurations(durations []time.Duration) {
	for i := 0; i < len(durations); i++ {
		for j := i + 1; j < len(durations); j++ {
			if durations[i] > durations[j] {
				durations[i], durations[j] = durations[j], durations[i]
			}
		}
	}
}

// Load pattern benchmarks
func BenchmarkLoadPatterns(b *testing.B) {
	b.Run("ConstantLoad", benchmarkConstantLoad)
	b.Run("RampLoad", benchmarkRampLoad)
	b.Run("SpikeLoad", benchmarkSpikeLoad)
	b.Run("WaveLoad", benchmarkWaveLoad)
	b.Run("BurstLoad", benchmarkBurstLoad)
	b.Run("ChaosLoad", benchmarkChaosLoad)
}

func benchmarkConstantLoad(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(100))
	
	tool := createBenchmarkTool("bench-tool", 5*time.Millisecond)
	if err := registry.Register(tool); err != nil {
		b.Fatal(err)
	}
	
	params := map[string]interface{}{
		"input": "constant load test",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Constant load with fixed number of workers
	workers := 50
	duration := 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	var ops int64
	var wg sync.WaitGroup
	
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					_, err := engine.Execute(ctx, tool.ID, params)
					if err == nil {
						atomic.AddInt64(&ops, 1)
					}
				}
			}
		}()
	}
	
	wg.Wait()
	
	b.StopTimer()
	b.ReportMetric(float64(ops)/duration.Seconds(), "ops/sec")
}

func benchmarkRampLoad(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(200))
	
	tool := createBenchmarkTool("bench-tool", 5*time.Millisecond)
	if err := registry.Register(tool); err != nil {
		b.Fatal(err)
	}
	
	params := map[string]interface{}{
		"input": "ramp load test",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Ramp load - gradually increase workers
	duration := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	var ops int64
	var wg sync.WaitGroup
	
	// Start workers gradually
	maxWorkers := 100
	rampDuration := 5 * time.Second
	
	for i := 0; i < maxWorkers; i++ {
		// Delay worker start based on ramp schedule
		delay := time.Duration(i) * rampDuration / time.Duration(maxWorkers)
		
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			// Wait for ramp delay
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			
			// Execute operations
			for {
				select {
				case <-ctx.Done():
					return
				default:
					_, err := engine.Execute(ctx, tool.ID, params)
					if err == nil {
						atomic.AddInt64(&ops, 1)
					}
				}
			}
		}()
	}
	
	wg.Wait()
	
	b.StopTimer()
	b.ReportMetric(float64(ops)/duration.Seconds(), "ops/sec")
}

func benchmarkSpikeLoad(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(500))
	
	tool := createBenchmarkTool("bench-tool", 2*time.Millisecond)
	if err := registry.Register(tool); err != nil {
		b.Fatal(err)
	}
	
	params := map[string]interface{}{
		"input": "spike load test",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Spike load - periodic high intensity bursts
	duration := 20 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	var ops int64
	var wg sync.WaitGroup
	
	// Background load
	baselineWorkers := 10
	for i := 0; i < baselineWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					_, err := engine.Execute(ctx, tool.ID, params)
					if err == nil {
						atomic.AddInt64(&ops, 1)
					}
					time.Sleep(100 * time.Millisecond)
				}
			}
		}()
	}
	
	// Spike generator
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Generate spike
				spikeWorkers := 200
				var spikeWg sync.WaitGroup
				
				for i := 0; i < spikeWorkers; i++ {
					spikeWg.Add(1)
					go func() {
						defer spikeWg.Done()
						_, err := engine.Execute(ctx, tool.ID, params)
						if err == nil {
							atomic.AddInt64(&ops, 1)
						}
					}()
				}
				spikeWg.Wait()
			}
		}
	}()
	
	wg.Wait()
	
	b.StopTimer()
	b.ReportMetric(float64(ops)/duration.Seconds(), "ops/sec")
}

func benchmarkWaveLoad(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(150))
	
	tool := createBenchmarkTool("bench-tool", 3*time.Millisecond)
	if err := registry.Register(tool); err != nil {
		b.Fatal(err)
	}
	
	params := map[string]interface{}{
		"input": "wave load test",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Wave load - sinusoidal pattern
	duration := 5 * time.Second  // Reduced from 30s to 5s
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	var ops int64
	var wg sync.WaitGroup
	
	// Wave generator
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		
		startTime := time.Now()
		var activeWorkers int
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				elapsed := time.Since(startTime).Seconds()
				
				// Sine wave with 20-second period
				waveValue := math.Sin(elapsed / 10 * math.Pi)
				targetWorkers := int(50 + 50*waveValue) // 0-100 workers
				
				// Adjust worker count
				if targetWorkers > activeWorkers {
					for i := activeWorkers; i < targetWorkers; i++ {
						wg.Add(1)
						go func() {
							defer wg.Done()
							_, err := engine.Execute(ctx, tool.ID, params)
							if err == nil {
								atomic.AddInt64(&ops, 1)
							}
						}()
					}
					activeWorkers = targetWorkers
				}
			}
		}
	}()
	
	wg.Wait()
	
	b.StopTimer()
	b.ReportMetric(float64(ops)/duration.Seconds(), "ops/sec")
}

func benchmarkBurstLoad(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(300))
	
	tool := createBenchmarkTool("bench-tool", 1*time.Millisecond)
	if err := registry.Register(tool); err != nil {
		b.Fatal(err)
	}
	
	params := map[string]interface{}{
		"input": "burst load test",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Burst load - rapid fire bursts
	duration := 15 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	var ops int64
	var wg sync.WaitGroup
	
	// Burst generator
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Generate burst
				burstSize := 100
				burstCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
				
				var burstWg sync.WaitGroup
				for i := 0; i < burstSize; i++ {
					burstWg.Add(1)
					go func() {
						defer burstWg.Done()
						_, err := engine.Execute(burstCtx, tool.ID, params)
						if err == nil {
							atomic.AddInt64(&ops, 1)
						}
					}()
				}
				burstWg.Wait()
				cancel()
			}
		}
	}()
	
	wg.Wait()
	
	b.StopTimer()
	b.ReportMetric(float64(ops)/duration.Seconds(), "ops/sec")
}

func benchmarkChaosLoad(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(400))
	
	tool := createBenchmarkTool("bench-tool", 2*time.Millisecond)
	if err := registry.Register(tool); err != nil {
		b.Fatal(err)
	}
	
	params := map[string]interface{}{
		"input": "chaos load test",
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	// Chaos load - random intensity changes
	duration := 20 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	var ops int64
	var wg sync.WaitGroup
	
	// Chaos generator
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Random intensity
				intensity := rand.Intn(200) + 10 // 10-210 workers
				
				var chaosWg sync.WaitGroup
				for i := 0; i < intensity; i++ {
					chaosWg.Add(1)
					go func() {
						defer chaosWg.Done()
						_, err := engine.Execute(ctx, tool.ID, params)
						if err == nil {
							atomic.AddInt64(&ops, 1)
						}
					}()
				}
				chaosWg.Wait()
			}
		}
	}()
	
	wg.Wait()
	
	b.StopTimer()
	b.ReportMetric(float64(ops)/duration.Seconds(), "ops/sec")
}

// Resource utilization benchmarks
func BenchmarkResourceUtilization(b *testing.B) {
	b.Run("MemoryIntensive", benchmarkMemoryIntensive)
	b.Run("CPUIntensive", benchmarkCPUIntensive)
	b.Run("GoroutineIntensive", benchmarkGoroutineIntensive)
	b.Run("AllocationIntensive", benchmarkAllocationIntensive)
}

func benchmarkMemoryIntensive(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(50))
	
	tool := createMemoryIntensiveTool("memory-tool")
	if err := registry.Register(tool); err != nil {
		b.Fatal(err)
	}
	
	params := map[string]interface{}{
		"size": 1024 * 1024, // 1MB
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := engine.Execute(context.Background(), tool.ID, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkCPUIntensive(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(runtime.NumCPU()))
	
	tool := createCPUIntensiveTool("cpu-tool")
	if err := registry.Register(tool); err != nil {
		b.Fatal(err)
	}
	
	params := map[string]interface{}{
		"iterations": 1000000,
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := engine.Execute(context.Background(), tool.ID, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkGoroutineIntensive(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(1000))
	
	tool := createGoroutineIntensiveTool("goroutine-tool")
	if err := registry.Register(tool); err != nil {
		b.Fatal(err)
	}
	
	params := map[string]interface{}{
		"goroutines": 100,
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := engine.Execute(context.Background(), tool.ID, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkAllocationIntensive(b *testing.B) {
	registry := NewRegistry()
	engine := NewExecutionEngine(registry, WithMaxConcurrent(50))
	
	tool := createAllocationIntensiveTool("allocation-tool")
	if err := registry.Register(tool); err != nil {
		b.Fatal(err)
	}
	
	params := map[string]interface{}{
		"allocations": 10000,
	}
	
	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		_, err := engine.Execute(context.Background(), tool.ID, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Resource-intensive tool executors
func createMemoryIntensiveTool(id string) *Tool {
	return &Tool{
		ID:          id,
		Name:        "Memory Intensive Tool",
		Description: "A tool that uses significant memory",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"size": {
					Type:        "integer",
					Description: "Memory size in bytes",
				},
			},
			Required: []string{"size"},
		},
		Executor: &MemoryIntensiveExecutor{},
	}
}

func createCPUIntensiveTool(id string) *Tool {
	return &Tool{
		ID:          id,
		Name:        "CPU Intensive Tool",
		Description: "A tool that uses significant CPU",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"iterations": {
					Type:        "integer",
					Description: "Number of iterations",
				},
			},
			Required: []string{"iterations"},
		},
		Executor: &CPUIntensiveExecutor{},
	}
}

func createGoroutineIntensiveTool(id string) *Tool {
	return &Tool{
		ID:          id,
		Name:        "Goroutine Intensive Tool",
		Description: "A tool that creates many goroutines",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"goroutines": {
					Type:        "integer",
					Description: "Number of goroutines",
				},
			},
			Required: []string{"goroutines"},
		},
		Executor: &GoroutineIntensiveExecutor{},
	}
}

func createAllocationIntensiveTool(id string) *Tool {
	return &Tool{
		ID:          id,
		Name:        "Allocation Intensive Tool",
		Description: "A tool that performs many allocations",
		Version:     "1.0.0",
		Schema: &ToolSchema{
			Type: "object",
			Properties: map[string]*Property{
				"allocations": {
					Type:        "integer",
					Description: "Number of allocations",
				},
			},
			Required: []string{"allocations"},
		},
		Executor: &AllocationIntensiveExecutor{},
	}
}

// Resource-intensive executors
type MemoryIntensiveExecutor struct{}

func (e *MemoryIntensiveExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	size := int(params["size"].(float64))
	
	// Allocate and use memory
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte(i % 256)
	}
	
	// Simulate processing
	checksum := 0
	for _, b := range data {
		checksum += int(b)
	}
	
	return &ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"checksum": checksum,
			"size":     size,
		},
		Timestamp: time.Now(),
	}, nil
}

type CPUIntensiveExecutor struct{}

func (e *CPUIntensiveExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	iterations := int(params["iterations"].(float64))
	
	// CPU-intensive computation
	result := 0
	for i := 0; i < iterations; i++ {
		result += i * i
		result = result % 1000000
	}
	
	return &ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"result":     result,
			"iterations": iterations,
		},
		Timestamp: time.Now(),
	}, nil
}

type GoroutineIntensiveExecutor struct{}

func (e *GoroutineIntensiveExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	goroutines := int(params["goroutines"].(float64))
	
	var wg sync.WaitGroup
	results := make([]int, goroutines)
	
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			
			// Simulate work
			sum := 0
			for j := 0; j < 1000; j++ {
				sum += j
			}
			results[idx] = sum
		}(i)
	}
	
	wg.Wait()
	
	total := 0
	for _, r := range results {
		total += r
	}
	
	return &ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"total":      total,
			"goroutines": goroutines,
		},
		Timestamp: time.Now(),
	}, nil
}

type AllocationIntensiveExecutor struct{}

func (e *AllocationIntensiveExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolExecutionResult, error) {
	allocations := int(params["allocations"].(float64))
	
	// Perform many allocations
	var objects []interface{}
	for i := 0; i < allocations; i++ {
		obj := map[string]interface{}{
			"id":    i,
			"data":  make([]byte, 1024),
			"items": make([]int, 100),
		}
		objects = append(objects, obj)
	}
	
	// Force some GC work
	runtime.GC()
	
	return &ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"allocated": len(objects),
			"count":     allocations,
		},
		Timestamp: time.Now(),
	}, nil
}