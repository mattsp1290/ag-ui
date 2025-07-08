package events

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

// Example demonstrates how to use parallel validation for improved CPU utilization
func ExampleParallelValidation() {
	// Create a new event validator
	validator := NewEventValidator(DefaultValidationConfig())

	// Configure parallel validation for optimal performance
	parallelConfig := &ParallelValidationConfig{
		MaxGoroutines:            runtime.NumCPU(), // Use all available CPU cores
		EnableParallelExecution:  true,             // Enable parallel processing
		MinRulesForParallel:      3,                // Parallel processing threshold
		ValidationTimeout:        30 * time.Second, // Timeout for validation
		BufferSize:               10,               // Worker pool buffer size
		EnableDependencyAnalysis: true,             // Automatic dependency analysis
		StopOnFirstError:         false,            // Continue validation on errors
	}

	// Apply the parallel configuration
	validator.SetParallelConfig(parallelConfig)

	// Create a sample event
	event := &RunStartedEvent{
		BaseEvent: &BaseEvent{
			EventType:   EventTypeRunStarted,
			TimestampMs: func() *int64 { t := time.Now().UnixMilli(); return &t }(),
		},
		RunID:    "example-run-123",
		ThreadID: "example-thread-456",
	}

	// Validate using parallel execution
	ctx := context.Background()
	result := validator.ValidateEventParallel(ctx, event)

	// Check results
	if result.IsValid {
		fmt.Printf("✅ Validation successful!\n")
		fmt.Printf("📊 Performance metrics:\n")
		fmt.Printf("   - Rules executed in parallel: %d\n", result.RulesExecutedInParallel)
		fmt.Printf("   - Rules executed sequentially: %d\n", result.RulesExecutedSequentially)
		fmt.Printf("   - Goroutines used: %d\n", result.GoroutinesUsed)
		fmt.Printf("   - Total validation time: %v\n", result.Duration)

		if result.ParallelExecutionTime > 0 && result.SequentialExecutionTime > 0 {
			speedup := float64(result.SequentialExecutionTime) / float64(result.ParallelExecutionTime)
			fmt.Printf("   - Parallel execution speedup: %.2fx\n", speedup)
		}
	} else {
		fmt.Printf("❌ Validation failed with %d errors:\n", len(result.Errors))
		for _, err := range result.Errors {
			fmt.Printf("   - [%s] %s: %s\n", err.Severity, err.RuleID, err.Message)
		}
	}

	// Get overall parallel validation metrics
	metrics := validator.GetParallelMetrics()
	if metrics.TotalValidations > 0 {
		fmt.Printf("\n📈 Overall parallel validation metrics:\n")
		fmt.Printf("   - Total validations: %d\n", metrics.TotalValidations)
		fmt.Printf("   - Parallel validations: %d\n", metrics.ParallelValidations)
		fmt.Printf("   - Average speedup: %.2fx\n", metrics.AverageSpeedup)
	}
}

// Example of configuring parallel validation for different scenarios
func ExampleParallelValidationScenarios() {
	// High-throughput scenario (prioritize speed)
	highThroughputConfig := &ParallelValidationConfig{
		MaxGoroutines:            runtime.NumCPU() * 2, // More goroutines for high concurrency
		EnableParallelExecution:  true,
		MinRulesForParallel:      2,               // Lower threshold for parallel execution
		ValidationTimeout:        5 * time.Second, // Shorter timeout for faster response
		BufferSize:               20,              // Larger buffer for better throughput
		EnableDependencyAnalysis: true,
		StopOnFirstError:         true, // Fail fast for performance
	}

	// Memory-conscious scenario (prioritize low memory usage)
	memoryOptimizedConfig := &ParallelValidationConfig{
		MaxGoroutines:            2, // Fewer goroutines to save memory
		EnableParallelExecution:  true,
		MinRulesForParallel:      5,                // Higher threshold to avoid overhead
		ValidationTimeout:        60 * time.Second, // Longer timeout for thorough validation
		BufferSize:               5,                // Smaller buffer to save memory
		EnableDependencyAnalysis: true,
		StopOnFirstError:         false, // Complete validation for accuracy
	}

	// Development scenario (prioritize debugging)
	developmentConfig := &ParallelValidationConfig{
		MaxGoroutines:            1,                 // Sequential execution for easier debugging
		EnableParallelExecution:  false,             // Disable parallel execution
		MinRulesForParallel:      100,               // Very high threshold (effectively disabled)
		ValidationTimeout:        120 * time.Second, // Long timeout for debugging
		BufferSize:               1,
		EnableDependencyAnalysis: false, // Disable for simpler debugging
		StopOnFirstError:         false, // Get all validation errors
	}

	fmt.Printf("Example configurations:\n")
	fmt.Printf("📈 High-throughput: %d goroutines, %d min rules\n",
		highThroughputConfig.MaxGoroutines, highThroughputConfig.MinRulesForParallel)
	fmt.Printf("🧠 Memory-optimized: %d goroutines, %d buffer size\n",
		memoryOptimizedConfig.MaxGoroutines, memoryOptimizedConfig.BufferSize)
	fmt.Printf("🐛 Development: parallel=%v, dependency analysis=%v\n",
		developmentConfig.EnableParallelExecution, developmentConfig.EnableDependencyAnalysis)
}

// Example of understanding rule dependencies for parallel execution
func ExampleRuleDependencyAnalysis() {
	analyzer := NewRuleDependencyAnalyzer()

	// Example rules with different dependency characteristics
	rules := []ValidationRule{
		NewMessageContentRule(),  // Independent
		NewToolCallContentRule(), // Independent
		NewRunLifecycleRule(),    // State-dependent
		NewEventOrderingRule(),   // Sequence-dependent
		NewIDConsistencyRule(),   // ID tracking dependent
	}

	// Analyze which rules can run in parallel
	independentRules, dependentRules := analyzer.AnalyzeRules(rules)

	fmt.Printf("🔍 Rule dependency analysis:\n")
	fmt.Printf("   ✅ Independent rules (can run in parallel): %d\n", len(independentRules))
	for _, rule := range independentRules {
		fmt.Printf("      - %s\n", rule.ID())
	}

	fmt.Printf("   🔄 Dependent rules (require sequential execution): %d\n", len(dependentRules))
	for _, rule := range dependentRules {
		fmt.Printf("      - %s\n", rule.ID())
	}

	// Add a custom rule with specific dependencies
	customRule := &RuleDependency{
		RuleID:           "CUSTOM_BUSINESS_RULE",
		DependencyType:   DependencyNone,
		RequiredRules:    []string{},
		ConflictingRules: []string{},
		CanRunInParallel: true,
	}
	analyzer.AddRuleDependency(customRule)

	fmt.Printf("\n🔧 Added custom rule: %s (can run in parallel: %v)\n",
		customRule.RuleID, customRule.CanRunInParallel)
}
