package validation

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/ag-ui/go-sdk/pkg/core/events"
	"github.com/ag-ui/go-sdk/pkg/encoding"
)

// BenchmarkSuite provides performance benchmarking for encoding/validation
type BenchmarkSuite struct {
	encoder   encoding.Encoder
	decoder   encoding.Decoder
	validator FormatValidator
	config    BenchmarkConfig
	results   []BenchmarkResult
	mu        sync.RWMutex
}

// BenchmarkConfig defines benchmarking configuration
type BenchmarkConfig struct {
	// Test parameters
	WarmupIterations int           // Number of warmup iterations
	TestIterations   int           // Number of test iterations
	Duration         time.Duration // Maximum test duration
	ConcurrencyLevel int           // Number of concurrent goroutines

	// Memory tracking
	TrackMemory     bool // Enable memory usage tracking
	GCBetweenTests  bool // Force GC between tests
	MemoryBaseline  bool // Establish memory baseline

	// Throughput testing
	EnableThroughputTest bool          // Enable throughput testing
	ThroughputDuration   time.Duration // Duration for throughput test
	BatchSizes           []int         // Batch sizes to test

	// Regression testing
	EnableRegressionTest bool           // Enable regression testing
	BaselineResults      []BenchmarkResult // Baseline results for comparison
	RegressionThreshold  float64        // Acceptable regression percentage

	// Profiling
	EnableCPUProfiling    bool // Enable CPU profiling
	EnableMemoryProfiling bool // Enable memory profiling
	EnableBlockProfiling  bool // Enable block profiling
}

// DefaultBenchmarkConfig returns default benchmark configuration
func DefaultBenchmarkConfig() BenchmarkConfig {
	return BenchmarkConfig{
		WarmupIterations:     100,
		TestIterations:       1000,
		Duration:             30 * time.Second,
		ConcurrencyLevel:     4,
		TrackMemory:          true,
		GCBetweenTests:       true,
		MemoryBaseline:       true,
		EnableThroughputTest: true,
		ThroughputDuration:   10 * time.Second,
		BatchSizes:           []int{1, 10, 100, 1000},
		EnableRegressionTest: false,
		RegressionThreshold:  10.0, // 10% regression threshold
		EnableCPUProfiling:   false,
		EnableMemoryProfiling: false,
		EnableBlockProfiling: false,
	}
}

// BenchmarkResult contains the results of a benchmark test
type BenchmarkResult struct {
	TestName     string        `json:"test_name"`
	Operation    string        `json:"operation"`
	Iterations   int           `json:"iterations"`
	Duration     time.Duration `json:"duration"`
	Throughput   float64       `json:"throughput"`   // ops/sec
	Latency      time.Duration `json:"latency"`      // average per operation
	MinLatency   time.Duration `json:"min_latency"`
	MaxLatency   time.Duration `json:"max_latency"`
	P50Latency   time.Duration `json:"p50_latency"`
	P95Latency   time.Duration `json:"p95_latency"`
	P99Latency   time.Duration `json:"p99_latency"`
	MemoryUsed   int64         `json:"memory_used"`
	MemoryAllocs int64         `json:"memory_allocs"`
	GCPauses     int64         `json:"gc_pauses"`
	ConcurrentOps int          `json:"concurrent_ops"`
	BatchSize    int           `json:"batch_size"`
	ErrorRate    float64       `json:"error_rate"`
}

// NewBenchmarkSuite creates a new benchmark suite
func NewBenchmarkSuite(encoder encoding.Encoder, decoder encoding.Decoder, validator FormatValidator, config BenchmarkConfig) *BenchmarkSuite {
	return &BenchmarkSuite{
		encoder:   encoder,
		decoder:   decoder,
		validator: validator,
		config:    config,
		results:   make([]BenchmarkResult, 0),
	}
}

// RunAllBenchmarks runs all benchmark tests
func (b *BenchmarkSuite) RunAllBenchmarks(ctx context.Context) error {
	fmt.Println("Starting benchmark suite...")

	// Establish memory baseline if configured
	if b.config.MemoryBaseline {
		b.establishMemoryBaseline()
	}

	// Run encoding benchmarks
	if err := b.runEncodingBenchmarks(ctx); err != nil {
		return fmt.Errorf("encoding benchmarks failed: %w", err)
	}

	// Run decoding benchmarks
	if err := b.runDecodingBenchmarks(ctx); err != nil {
		return fmt.Errorf("decoding benchmarks failed: %w", err)
	}

	// Run validation benchmarks
	if err := b.runValidationBenchmarks(ctx); err != nil {
		return fmt.Errorf("validation benchmarks failed: %w", err)
	}

	// Run round-trip benchmarks
	if err := b.runRoundTripBenchmarks(ctx); err != nil {
		return fmt.Errorf("round-trip benchmarks failed: %w", err)
	}

	// Run throughput tests if enabled
	if b.config.EnableThroughputTest {
		if err := b.runThroughputTests(ctx); err != nil {
			return fmt.Errorf("throughput tests failed: %w", err)
		}
	}

	// Run regression tests if enabled
	if b.config.EnableRegressionTest {
		if err := b.runRegressionTests(); err != nil {
			return fmt.Errorf("regression tests failed: %w", err)
		}
	}

	fmt.Println("Benchmark suite completed successfully")
	return nil
}

// runEncodingBenchmarks runs encoding performance benchmarks
func (b *BenchmarkSuite) runEncodingBenchmarks(ctx context.Context) error {
	fmt.Println("Running encoding benchmarks...")

	testEvents := b.generateTestEvents()

	for _, event := range testEvents {
		eventName := string(event.Type())
		
		// Single event encoding
		result, err := b.benchmarkSingleEncoding(ctx, eventName, event)
		if err != nil {
			return fmt.Errorf("single encoding benchmark failed for %s: %w", eventName, err)
		}
		b.addResult(result)

		// Batch encoding
		for _, batchSize := range b.config.BatchSizes {
			batch := make([]events.Event, batchSize)
			for i := range batch {
				batch[i] = event
			}
			
			result, err := b.benchmarkBatchEncoding(ctx, eventName, batch)
			if err != nil {
				return fmt.Errorf("batch encoding benchmark failed for %s (batch size %d): %w", eventName, batchSize, err)
			}
			b.addResult(result)
		}
	}

	return nil
}

// runDecodingBenchmarks runs decoding performance benchmarks
func (b *BenchmarkSuite) runDecodingBenchmarks(ctx context.Context) error {
	fmt.Println("Running decoding benchmarks...")

	testEvents := b.generateTestEvents()

	for _, event := range testEvents {
		eventName := string(event.Type())
		
		// Encode the event first
		encoded, err := b.encoder.Encode(event)
		if err != nil {
			return fmt.Errorf("failed to encode event for decoding benchmark: %w", err)
		}

		// Single event decoding
		result, err := b.benchmarkSingleDecoding(ctx, eventName, encoded)
		if err != nil {
			return fmt.Errorf("single decoding benchmark failed for %s: %w", eventName, err)
		}
		b.addResult(result)

		// Batch decoding
		for _, batchSize := range b.config.BatchSizes {
			batch := make([]events.Event, batchSize)
			for i := range batch {
				batch[i] = event
			}
			
			encodedBatch, err := b.encoder.EncodeMultiple(batch)
			if err != nil {
				return fmt.Errorf("failed to encode batch for decoding benchmark: %w", err)
			}

			result, err := b.benchmarkBatchDecoding(ctx, eventName, encodedBatch, batchSize)
			if err != nil {
				return fmt.Errorf("batch decoding benchmark failed for %s (batch size %d): %w", eventName, batchSize, err)
			}
			b.addResult(result)
		}
	}

	return nil
}

// runValidationBenchmarks runs validation performance benchmarks
func (b *BenchmarkSuite) runValidationBenchmarks(ctx context.Context) error {
	if b.validator == nil {
		return nil // Skip if no validator
	}

	fmt.Println("Running validation benchmarks...")

	testEvents := b.generateTestEvents()

	for _, event := range testEvents {
		eventName := string(event.Type())
		
		// Event validation
		result, err := b.benchmarkEventValidation(ctx, eventName, event)
		if err != nil {
			return fmt.Errorf("event validation benchmark failed for %s: %w", eventName, err)
		}
		b.addResult(result)

		// Format validation
		encoded, err := b.encoder.Encode(event)
		if err != nil {
			return fmt.Errorf("failed to encode event for format validation benchmark: %w", err)
		}

		result, err = b.benchmarkFormatValidation(ctx, eventName, encoded)
		if err != nil {
			return fmt.Errorf("format validation benchmark failed for %s: %w", eventName, err)
		}
		b.addResult(result)
	}

	return nil
}

// runRoundTripBenchmarks runs round-trip performance benchmarks
func (b *BenchmarkSuite) runRoundTripBenchmarks(ctx context.Context) error {
	fmt.Println("Running round-trip benchmarks...")

	testEvents := b.generateTestEvents()
	roundTripValidator := NewRoundTripValidator(b.encoder, b.decoder)

	for _, event := range testEvents {
		eventName := string(event.Type())
		
		result, err := b.benchmarkRoundTrip(ctx, eventName, event, roundTripValidator)
		if err != nil {
			return fmt.Errorf("round-trip benchmark failed for %s: %w", eventName, err)
		}
		b.addResult(result)
	}

	return nil
}

// runThroughputTests runs throughput performance tests
func (b *BenchmarkSuite) runThroughputTests(ctx context.Context) error {
	fmt.Println("Running throughput tests...")

	testEvents := b.generateTestEvents()

	for _, event := range testEvents {
		eventName := string(event.Type())
		
		// Encoding throughput
		result, err := b.benchmarkEncodingThroughput(ctx, eventName, event)
		if err != nil {
			return fmt.Errorf("encoding throughput test failed for %s: %w", eventName, err)
		}
		b.addResult(result)

		// Decoding throughput
		encoded, err := b.encoder.Encode(event)
		if err != nil {
			return fmt.Errorf("failed to encode event for decoding throughput test: %w", err)
		}

		result, err = b.benchmarkDecodingThroughput(ctx, eventName, encoded)
		if err != nil {
			return fmt.Errorf("decoding throughput test failed for %s: %w", eventName, err)
		}
		b.addResult(result)
	}

	return nil
}

// Benchmark implementation methods

func (b *BenchmarkSuite) benchmarkSingleEncoding(ctx context.Context, name string, event events.Event) (BenchmarkResult, error) {
	return b.benchmarkOperation(ctx, fmt.Sprintf("%s_encode_single", name), func() error {
		_, err := b.encoder.Encode(event)
		return err
	}, 1)
}

func (b *BenchmarkSuite) benchmarkBatchEncoding(ctx context.Context, name string, events []events.Event) (BenchmarkResult, error) {
	return b.benchmarkOperation(ctx, fmt.Sprintf("%s_encode_batch_%d", name, len(events)), func() error {
		_, err := b.encoder.EncodeMultiple(events)
		return err
	}, len(events))
}

func (b *BenchmarkSuite) benchmarkSingleDecoding(ctx context.Context, name string, data []byte) (BenchmarkResult, error) {
	return b.benchmarkOperation(ctx, fmt.Sprintf("%s_decode_single", name), func() error {
		_, err := b.decoder.Decode(data)
		return err
	}, 1)
}

func (b *BenchmarkSuite) benchmarkBatchDecoding(ctx context.Context, name string, data []byte, batchSize int) (BenchmarkResult, error) {
	return b.benchmarkOperation(ctx, fmt.Sprintf("%s_decode_batch_%d", name, batchSize), func() error {
		_, err := b.decoder.DecodeMultiple(data)
		return err
	}, batchSize)
}

func (b *BenchmarkSuite) benchmarkEventValidation(ctx context.Context, name string, event events.Event) (BenchmarkResult, error) {
	return b.benchmarkOperation(ctx, fmt.Sprintf("%s_validate_event", name), func() error {
		return b.validator.ValidateEvent(event)
	}, 1)
}

func (b *BenchmarkSuite) benchmarkFormatValidation(ctx context.Context, name string, data []byte) (BenchmarkResult, error) {
	return b.benchmarkOperation(ctx, fmt.Sprintf("%s_validate_format", name), func() error {
		return b.validator.ValidateFormat(data)
	}, 1)
}

func (b *BenchmarkSuite) benchmarkRoundTrip(ctx context.Context, name string, event events.Event, validator *RoundTripValidator) (BenchmarkResult, error) {
	return b.benchmarkOperation(ctx, fmt.Sprintf("%s_round_trip", name), func() error {
		return validator.ValidateRoundTrip(ctx, event)
	}, 1)
}

// benchmarkOperation is the core benchmarking function
func (b *BenchmarkSuite) benchmarkOperation(ctx context.Context, testName string, operation func() error, batchSize int) (BenchmarkResult, error) {
	result := BenchmarkResult{
		TestName:  testName,
		BatchSize: batchSize,
	}

	// Warmup
	for i := 0; i < b.config.WarmupIterations; i++ {
		if err := operation(); err != nil {
			return result, fmt.Errorf("warmup failed: %w", err)
		}
	}

	// Force GC if configured
	if b.config.GCBetweenTests {
		runtime.GC()
	}

	// Memory baseline
	var memBefore, memAfter runtime.MemStats
	if b.config.TrackMemory {
		runtime.ReadMemStats(&memBefore)
	}

	// Benchmark test
	latencies := make([]time.Duration, b.config.TestIterations)
	errorCount := 0
	start := time.Now()

	for i := 0; i < b.config.TestIterations; i++ {
		opStart := time.Now()
		if err := operation(); err != nil {
			errorCount++
		}
		latencies[i] = time.Since(opStart)

		// Check context cancellation
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		// Check duration limit
		if time.Since(start) > b.config.Duration {
			latencies = latencies[:i+1]
			result.Iterations = i + 1
			break
		}
	}

	duration := time.Since(start)
	if result.Iterations == 0 {
		result.Iterations = b.config.TestIterations
	}

	// Memory usage
	if b.config.TrackMemory {
		runtime.ReadMemStats(&memAfter)
		result.MemoryUsed = int64(memAfter.TotalAlloc - memBefore.TotalAlloc)
		result.MemoryAllocs = int64(memAfter.Mallocs - memBefore.Mallocs)
		result.GCPauses = int64(memAfter.NumGC - memBefore.NumGC)
	}

	// Calculate metrics
	result.Duration = duration
	result.Throughput = float64(result.Iterations*batchSize) / duration.Seconds()
	result.ErrorRate = float64(errorCount) / float64(result.Iterations) * 100

	// Calculate latency statistics
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	if len(latencies) > 0 {
		result.MinLatency = latencies[0]
		result.MaxLatency = latencies[len(latencies)-1]
		result.P50Latency = latencies[len(latencies)*50/100]
		result.P95Latency = latencies[len(latencies)*95/100]
		result.P99Latency = latencies[len(latencies)*99/100]

		var total time.Duration
		for _, lat := range latencies {
			total += lat
		}
		result.Latency = total / time.Duration(len(latencies))
	}

	return result, nil
}

// Throughput benchmarks

func (b *BenchmarkSuite) benchmarkEncodingThroughput(ctx context.Context, name string, event events.Event) (BenchmarkResult, error) {
	result := BenchmarkResult{
		TestName:  fmt.Sprintf("%s_encoding_throughput", name),
		Operation: "encoding_throughput",
		BatchSize: 1,
	}

	start := time.Now()
	iterations := 0
	errorCount := 0

	for time.Since(start) < b.config.ThroughputDuration {
		if _, err := b.encoder.Encode(event); err != nil {
			errorCount++
		}
		iterations++

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
	}

	duration := time.Since(start)
	result.Duration = duration
	result.Iterations = iterations
	result.Throughput = float64(iterations) / duration.Seconds()
	result.ErrorRate = float64(errorCount) / float64(iterations) * 100

	return result, nil
}

func (b *BenchmarkSuite) benchmarkDecodingThroughput(ctx context.Context, name string, data []byte) (BenchmarkResult, error) {
	result := BenchmarkResult{
		TestName:  fmt.Sprintf("%s_decoding_throughput", name),
		Operation: "decoding_throughput",
		BatchSize: 1,
	}

	start := time.Now()
	iterations := 0
	errorCount := 0

	for time.Since(start) < b.config.ThroughputDuration {
		if _, err := b.decoder.Decode(data); err != nil {
			errorCount++
		}
		iterations++

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
	}

	duration := time.Since(start)
	result.Duration = duration
	result.Iterations = iterations
	result.Throughput = float64(iterations) / duration.Seconds()
	result.ErrorRate = float64(errorCount) / float64(iterations) * 100

	return result, nil
}

// Regression testing

func (b *BenchmarkSuite) runRegressionTests() error {
	if len(b.config.BaselineResults) == 0 {
		return fmt.Errorf("no baseline results provided for regression testing")
	}

	fmt.Println("Running regression tests...")

	// Create a map of baseline results by test name
	baselineMap := make(map[string]BenchmarkResult)
	for _, baseline := range b.config.BaselineResults {
		baselineMap[baseline.TestName] = baseline
	}

	// Compare current results with baseline
	var regressions []string
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, result := range b.results {
		baseline, ok := baselineMap[result.TestName]
		if !ok {
			continue // New test, no baseline to compare
		}

		// Check throughput regression
		if baseline.Throughput > 0 {
			regressionPercent := (baseline.Throughput - result.Throughput) / baseline.Throughput * 100
			if regressionPercent > b.config.RegressionThreshold {
				regressions = append(regressions, fmt.Sprintf("Test %s: throughput regression of %.2f%% (%.2f -> %.2f ops/sec)",
					result.TestName, regressionPercent, baseline.Throughput, result.Throughput))
			}
		}

		// Check latency regression
		if baseline.Latency > 0 {
			regressionPercent := float64(result.Latency-baseline.Latency) / float64(baseline.Latency) * 100
			if regressionPercent > b.config.RegressionThreshold {
				regressions = append(regressions, fmt.Sprintf("Test %s: latency regression of %.2f%% (%v -> %v)",
					result.TestName, regressionPercent, baseline.Latency, result.Latency))
			}
		}
	}

	if len(regressions) > 0 {
		return fmt.Errorf("performance regressions detected:\n%s", fmt.Sprintf("- %s", regressions))
	}

	fmt.Println("No performance regressions detected")
	return nil
}

// Utility methods

func (b *BenchmarkSuite) addResult(result BenchmarkResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.results = append(b.results, result)
}

func (b *BenchmarkSuite) GetResults() []BenchmarkResult {
	b.mu.RLock()
	defer b.mu.RUnlock()
	
	results := make([]BenchmarkResult, len(b.results))
	copy(results, b.results)
	return results
}

func (b *BenchmarkSuite) establishMemoryBaseline() {
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	fmt.Printf("Memory baseline established: %d bytes allocated\n", stats.TotalAlloc)
}

func (b *BenchmarkSuite) generateTestEvents() []events.Event {
	return []events.Event{
		&events.RunStartedEvent{
			BaseEvent: &events.BaseEvent{
				EventType: events.EventTypeRunStarted,
				TimestampMs: int64Ptr(time.Now().Unix()),
			},
			RunID:    "run-benchmark-123",
			ThreadID: "thread-benchmark-456",
		},
		&events.TextMessageContentEvent{
			BaseEvent: &events.BaseEvent{
				EventType: events.EventTypeTextMessageContent,
				TimestampMs: int64Ptr(time.Now().Unix()),
			},
			MessageID: "msg-benchmark-789",
			Delta:     "This is a benchmark test message with some content to encode and decode.",
		},
		&events.ToolCallStartEvent{
			BaseEvent: &events.BaseEvent{
				EventType: events.EventTypeToolCallStart,
				TimestampMs: int64Ptr(time.Now().Unix()),
			},
			ToolCallID:   "tool-benchmark-abc",
			ToolCallName: "benchmark_calculator",
		},
		&events.StateSnapshotEvent{
			BaseEvent: &events.BaseEvent{
				EventType: events.EventTypeStateSnapshot,
				TimestampMs: int64Ptr(time.Now().Unix()),
			},
			Snapshot: map[string]interface{}{
				"key1": "value1",
				"key2": 42,
				"key3": []interface{}{"a", "b", "c"},
				"key4": map[string]interface{}{
					"nested": "value",
					"count":  100,
				},
			},
		},
	}
}

// Memory profiler for detailed memory analysis
type MemoryProfiler struct {
	enabled bool
	samples []MemorySample
	mu      sync.Mutex
}

type MemorySample struct {
	Timestamp time.Time
	Alloc     uint64
	TotalAlloc uint64
	Sys       uint64
	Mallocs   uint64
	Frees     uint64
	GCCycles  uint32
}

func NewMemoryProfiler(enabled bool) *MemoryProfiler {
	return &MemoryProfiler{
		enabled: enabled,
		samples: make([]MemorySample, 0),
	}
}

func (p *MemoryProfiler) Sample() {
	if !p.enabled {
		return
	}

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.samples = append(p.samples, MemorySample{
		Timestamp:  time.Now(),
		Alloc:      stats.Alloc,
		TotalAlloc: stats.TotalAlloc,
		Sys:        stats.Sys,
		Mallocs:    stats.Mallocs,
		Frees:      stats.Frees,
		GCCycles:   stats.NumGC,
	})
}

func (p *MemoryProfiler) GetSamples() []MemorySample {
	p.mu.Lock()
	defer p.mu.Unlock()

	samples := make([]MemorySample, len(p.samples))
	copy(samples, p.samples)
	return samples
}

func (p *MemoryProfiler) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.samples = p.samples[:0]
}