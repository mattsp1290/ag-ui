package events

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"
)

// BenchmarkSequentialValidation benchmarks traditional sequential rule execution
func BenchmarkSequentialValidation(b *testing.B) {
	validator := NewEventValidator(DefaultValidationConfig())

	// Disable parallel validation to force sequential execution
	validator.EnableParallelValidation(false)

	event := &RunStartedEvent{
		BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
		RunID:     "benchmark-run",
		ThreadID:  "benchmark-thread",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset validator state for each iteration
		validator.Reset()

		result := validator.ValidateEvent(ctx, event)
		if !result.IsValid {
			b.Fatalf("Validation failed: %v", result.Errors)
		}
	}
}

// BenchmarkParallelValidationExecution benchmarks parallel rule execution
func BenchmarkParallelValidationExecution(b *testing.B) {
	validator := NewEventValidator(DefaultValidationConfig())

	// Configure for optimal parallel execution
	parallelConfig := DefaultParallelValidationConfig()
	parallelConfig.MaxGoroutines = runtime.NumCPU()
	parallelConfig.MinRulesForParallel = 1 // Force parallel execution
	parallelConfig.EnableParallelExecution = true
	validator.SetParallelConfig(parallelConfig)

	event := &RunStartedEvent{
		BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
		RunID:     "benchmark-run",
		ThreadID:  "benchmark-thread",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset validator state for each iteration
		validator.Reset()

		result := validator.ValidateEventParallel(ctx, event)
		if !result.IsValid {
			b.Fatalf("Validation failed: %v", result.Errors)
		}
	}
}

// BenchmarkParallelValidationWithDifferentRuleCounts benchmarks parallel validation with varying rule counts
func BenchmarkParallelValidationWithDifferentRuleCounts(b *testing.B) {
	ruleCounts := []int{1, 3, 5, 10, 20}

	for _, ruleCount := range ruleCounts {
		b.Run(fmt.Sprintf("Rules%d", ruleCount), func(b *testing.B) {
			validator := NewEventValidator(DefaultValidationConfig())

			// Add additional mock rules to reach desired count
			for i := len(validator.GetRules()); i < ruleCount; i++ {
				validator.AddRule(&MockValidationRule{
					id:      fmt.Sprintf("BENCHMARK_RULE_%d", i),
					enabled: true,
					delay:   1 * time.Millisecond, // Small delay to simulate real work
				})
			}

			// Configure for parallel execution
			parallelConfig := DefaultParallelValidationConfig()
			parallelConfig.MaxGoroutines = runtime.NumCPU()
			parallelConfig.MinRulesForParallel = 1
			validator.SetParallelConfig(parallelConfig)

			event := &RunStartedEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
				RunID:     "benchmark-run",
				ThreadID:  "benchmark-thread",
			}

			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Reset validator state for each iteration
				validator.Reset()

				result := validator.ValidateEventParallel(ctx, event)
				if !result.IsValid {
					b.Fatalf("Validation failed: %v", result.Errors)
				}
			}
		})
	}
}

// BenchmarkParallelValidationWithDifferentGoroutineCounts benchmarks parallel validation with varying goroutine counts
func BenchmarkParallelValidationWithDifferentGoroutineCounts(b *testing.B) {
	goroutineCounts := []int{1, 2, 4, 8, 16}

	for _, goroutineCount := range goroutineCounts {
		b.Run(fmt.Sprintf("Goroutines%d", goroutineCount), func(b *testing.B) {
			validator := NewEventValidator(DefaultValidationConfig())

			// Add some mock rules to ensure we have work to parallelize
			for i := 0; i < 10; i++ {
				validator.AddRule(&MockValidationRule{
					id:      fmt.Sprintf("BENCHMARK_RULE_%d", i),
					enabled: true,
					delay:   1 * time.Millisecond,
				})
			}

			// Configure parallel execution with specific goroutine count
			parallelConfig := DefaultParallelValidationConfig()
			parallelConfig.MaxGoroutines = goroutineCount
			parallelConfig.MinRulesForParallel = 1
			validator.SetParallelConfig(parallelConfig)

			event := &RunStartedEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
				RunID:     "benchmark-run",
				ThreadID:  "benchmark-thread",
			}

			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Reset validator state for each iteration
				validator.Reset()

				result := validator.ValidateEventParallel(ctx, event)
				if !result.IsValid {
					b.Fatalf("Validation failed: %v", result.Errors)
				}
			}
		})
	}
}

// BenchmarkParallelWorkerPool benchmarks the worker pool performance
func BenchmarkParallelWorkerPool(b *testing.B) {
	workerCounts := []int{1, 2, 4, 8}

	for _, workerCount := range workerCounts {
		b.Run(fmt.Sprintf("Workers%d", workerCount), func(b *testing.B) {
			pool := NewParallelWorkerPool(workerCount, 10)
			pool.Start()
			defer pool.Stop()

			event := &RunStartedEvent{
				BaseEvent: &BaseEvent{EventType: EventTypeRunStarted},
				RunID:     "benchmark-run",
				ThreadID:  "benchmark-thread",
			}

			rule := &MockValidationRule{
				id:      "BENCHMARK_RULE",
				enabled: true,
				delay:   100 * time.Microsecond,
			}

			context := &ValidationContext{State: NewValidationState()}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				job := ParallelValidationJob{
					Rule:    rule,
					Event:   event,
					Context: context,
					JobID:   i,
				}

				err := pool.SubmitJob(job)
				if err != nil {
					b.Fatalf("Failed to submit job: %v", err)
				}

				_, err = pool.GetResult()
				if err != nil {
					b.Fatalf("Failed to get result: %v", err)
				}
			}
		})
	}
}

// BenchmarkRuleDependencyAnalysis benchmarks the rule dependency analysis
func BenchmarkRuleDependencyAnalysis(b *testing.B) {
	analyzer := NewRuleDependencyAnalyzer()

	// Create a large set of rules
	rules := make([]ValidationRule, 100)
	for i := 0; i < 100; i++ {
		rules[i] = &MockValidationRule{
			id:      fmt.Sprintf("RULE_%d", i),
			enabled: true,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = analyzer.AnalyzeRules(rules)
	}
}

// BenchmarkComplexEventValidation benchmarks validation of complex events with nested structures
func BenchmarkComplexEventValidation(b *testing.B) {
	validator := NewEventValidator(DefaultValidationConfig())

	// Configure for parallel execution
	parallelConfig := DefaultParallelValidationConfig()
	parallelConfig.MaxGoroutines = runtime.NumCPU()
	parallelConfig.MinRulesForParallel = 1
	validator.SetParallelConfig(parallelConfig)

	// Create a complex event sequence
	events := []Event{
		&RunStartedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
			RunID:     "complex-run",
			ThreadID:  "complex-thread",
		},
		&TextMessageStartEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageStart, TimestampMs: timePtr(time.Now().UnixMilli())},
			MessageID: "complex-msg",
			Role:      parallelStringPtr("user"),
		},
		&TextMessageContentEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageContent, TimestampMs: timePtr(time.Now().UnixMilli())},
			MessageID: "complex-msg",
			Delta:     "This is a complex message with lots of content that needs validation",
		},
		&ToolCallStartEvent{
			BaseEvent:    &BaseEvent{EventType: EventTypeToolCallStart, TimestampMs: timePtr(time.Now().UnixMilli())},
			ToolCallID:   "complex-tool",
			ToolCallName: "complex_tool",
		},
		&ToolCallArgsEvent{
			BaseEvent:  &BaseEvent{EventType: EventTypeToolCallArgs, TimestampMs: timePtr(time.Now().UnixMilli())},
			ToolCallID: "complex-tool",
			Delta:      `{"param1": "value1", "param2": "value2"}`,
		},
		&ToolCallEndEvent{
			BaseEvent:  &BaseEvent{EventType: EventTypeToolCallEnd, TimestampMs: timePtr(time.Now().UnixMilli())},
			ToolCallID: "complex-tool",
		},
		&TextMessageEndEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeTextMessageEnd, TimestampMs: timePtr(time.Now().UnixMilli())},
			MessageID: "complex-msg",
		},
		&RunFinishedEvent{
			BaseEvent: &BaseEvent{EventType: EventTypeRunFinished, TimestampMs: timePtr(time.Now().UnixMilli())},
			RunID:     "complex-run",
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, event := range events {
			result := validator.ValidateEventParallel(ctx, event)
			if !result.IsValid {
				b.Fatalf("Validation failed for event %T: %v", event, result.Errors)
			}
		}

		// Reset validator state for next iteration
		validator.Reset()
	}
}

// BenchmarkMemoryUsage benchmarks memory usage during parallel validation
func BenchmarkMemoryUsage(b *testing.B) {
	validator := NewEventValidator(DefaultValidationConfig())

	// Configure for parallel execution
	parallelConfig := DefaultParallelValidationConfig()
	parallelConfig.MaxGoroutines = runtime.NumCPU()
	parallelConfig.MinRulesForParallel = 1
	validator.SetParallelConfig(parallelConfig)

	event := &RunStartedEvent{
		BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
		RunID:     "memory-test-run",
		ThreadID:  "memory-test-thread",
	}

	ctx := context.Background()

	// Force garbage collection before benchmark
	runtime.GC()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := validator.ValidateEventParallel(ctx, event)
		if !result.IsValid {
			b.Fatalf("Validation failed: %v", result.Errors)
		}
	}
}

// BenchmarkParallelValidationLatency measures latency characteristics
func BenchmarkParallelValidationLatency(b *testing.B) {
	validator := NewEventValidator(DefaultValidationConfig())

	// Add some rules with varying processing times to simulate real-world scenarios
	for i := 0; i < 5; i++ {
		validator.AddRule(&MockValidationRule{
			id:      fmt.Sprintf("LATENCY_RULE_%d", i),
			enabled: true,
			delay:   time.Duration(i+1) * time.Millisecond,
		})
	}

	// Configure for parallel execution
	parallelConfig := DefaultParallelValidationConfig()
	parallelConfig.MaxGoroutines = runtime.NumCPU()
	parallelConfig.MinRulesForParallel = 1
	validator.SetParallelConfig(parallelConfig)

	event := &RunStartedEvent{
		BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
		RunID:     "latency-test-run",
		ThreadID:  "latency-test-thread",
	}

	ctx := context.Background()

	latencies := make([]time.Duration, b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		result := validator.ValidateEventParallel(ctx, event)
		latencies[i] = time.Since(start)

		if !result.IsValid {
			b.Fatalf("Validation failed: %v", result.Errors)
		}
	}

	// Calculate and report statistics
	if b.N > 0 {
		var total time.Duration
		min := latencies[0]
		max := latencies[0]

		for _, latency := range latencies {
			total += latency
			if latency < min {
				min = latency
			}
			if latency > max {
				max = latency
			}
		}

		avg := total / time.Duration(b.N)
		b.Logf("Latency stats - Min: %v, Max: %v, Avg: %v", min, max, avg)
	}
}

// BenchmarkSpeedupMeasurement measures actual speedup achieved by parallel validation
func BenchmarkSpeedupMeasurement(b *testing.B) {
	// This benchmark measures both sequential and parallel execution to calculate speedup
	validator := NewEventValidator(DefaultValidationConfig())

	// Add several rules with processing delays to simulate real validation work
	for i := 0; i < 8; i++ {
		validator.AddRule(&MockValidationRule{
			id:      fmt.Sprintf("SPEEDUP_RULE_%d", i),
			enabled: true,
			delay:   2 * time.Millisecond, // Simulated processing time
		})
	}

	event := &RunStartedEvent{
		BaseEvent: &BaseEvent{EventType: EventTypeRunStarted, TimestampMs: timePtr(time.Now().UnixMilli())},
		RunID:     "speedup-test-run",
		ThreadID:  "speedup-test-thread",
	}

	ctx := context.Background()

	b.Run("Sequential", func(b *testing.B) {
		// Disable parallel validation
		validator.EnableParallelValidation(false)

		var totalDuration time.Duration
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			start := time.Now()
			result := validator.ValidateEvent(ctx, event)
			totalDuration += time.Since(start)

			if !result.IsValid {
				b.Fatalf("Sequential validation failed: %v", result.Errors)
			}
		}

		if b.N > 0 {
			avgSeq := totalDuration / time.Duration(b.N)
			b.Logf("Sequential average: %v", avgSeq)
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		// Enable parallel validation
		parallelConfig := DefaultParallelValidationConfig()
		parallelConfig.MaxGoroutines = runtime.NumCPU()
		parallelConfig.MinRulesForParallel = 1
		validator.SetParallelConfig(parallelConfig)
		validator.EnableParallelValidation(true)

		var totalDuration time.Duration
		var totalSpeedup float64
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			start := time.Now()
			result := validator.ValidateEventParallel(ctx, event)
			totalDuration += time.Since(start)

			if !result.IsValid {
				b.Fatalf("Parallel validation failed: %v", result.Errors)
			}

			// Calculate speedup for this iteration
			if result.SequentialExecutionTime > 0 && result.ParallelExecutionTime > 0 {
				speedup := float64(result.SequentialExecutionTime) / float64(result.ParallelExecutionTime)
				totalSpeedup += speedup
			}
		}

		if b.N > 0 {
			avgPar := totalDuration / time.Duration(b.N)
			avgSpeedup := totalSpeedup / float64(b.N)
			b.Logf("Parallel average: %v, Average speedup: %.2fx", avgPar, avgSpeedup)
		}
	})
}

// Helper function for string pointers (renamed to avoid conflicts)
func parallelStringPtr(s string) *string {
	return &s
}
