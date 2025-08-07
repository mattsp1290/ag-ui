package calculator_test

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Import the calculator tool (assuming it's in the main package for now)
// In a real implementation, you would import from the actual package

// MockCalculatorExecutor provides a testable version of the calculator
type MockCalculatorExecutor struct{}

func (c *MockCalculatorExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	operation, ok := params["operation"].(string)
	if !ok {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "operation parameter is required",
		}, nil
	}

	operand1, ok := params["operand1"].(float64)
	if !ok {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "operand1 must be a number",
		}, nil
	}

	// operand2 is not required for sqrt operation
	var operand2 float64
	if operation != "sqrt" {
		operand2, ok = params["operand2"].(float64)
		if !ok {
			return &tools.ToolExecutionResult{
				Success: false,
				Error:   "operand2 must be a number",
			}, nil
		}
	}

	var result float64

	switch operation {
	case "add":
		result = operand1 + operand2
	case "subtract":
		result = operand1 - operand2
	case "multiply":
		result = operand1 * operand2
	case "divide":
		if operand2 == 0 {
			return &tools.ToolExecutionResult{
				Success: false,
				Error:   "division by zero",
			}, nil
		}
		result = operand1 / operand2
	case "power":
		result = math.Pow(operand1, operand2)
	case "modulo":
		if operand2 == 0 {
			return &tools.ToolExecutionResult{
				Success: false,
				Error:   "modulo by zero",
			}, nil
		}
		result = math.Mod(operand1, operand2)
	case "sqrt":
		if operand1 < 0 {
			return &tools.ToolExecutionResult{
				Success: false,
				Error:   "square root of negative number",
			}, nil
		}
		result = math.Sqrt(operand1)
	default:
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "unsupported operation: " + operation,
		}, nil
	}

	return &tools.ToolExecutionResult{
		Success:   true,
		Data:      result,
		Timestamp: time.Now(),
		Duration:  time.Microsecond * 100, // Simulated processing time
		Metadata: func() map[string]interface{} {
			metadata := map[string]interface{}{
				"operation": operation,
				"operand1":  operand1,
			}
			if operation != "sqrt" {
				metadata["operand2"] = operand2
			}
			return metadata
		}(),
	}, nil
}

func createCalculatorTool() *tools.Tool {
	return &tools.Tool{
		ID:          "calculator",
		Name:        "Calculator",
		Description: "A simple calculator tool for basic mathematical operations",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"operation": {
					Type:        "string",
					Description: "Mathematical operation to perform",
					Enum:        []interface{}{"add", "subtract", "multiply", "divide", "power", "modulo", "sqrt"},
				},
				"operand1": {
					Type:        "number",
					Description: "First operand",
				},
				"operand2": {
					Type:        "number",
					Description: "Second operand (not needed for sqrt)",
				},
			},
			Required: []string{"operation", "operand1"},
		},
		Executor: &MockCalculatorExecutor{},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  false,
			Async:      false,
			Cancelable: false,
			Cacheable:  true,
			Timeout:    5 * time.Second,
		},
		Metadata: &tools.ToolMetadata{
			Author:  "Math Team",
			License: "MIT",
			Tags:    []string{"math", "calculator", "arithmetic"},
			Examples: []tools.ToolExample{
				{
					Name:        "Addition",
					Description: "Add two numbers",
					Input: map[string]interface{}{
						"operation": "add",
						"operand1":  5.0,
						"operand2":  3.0,
					},
					Output: 8.0,
				},
			},
		},
	}
}

// TestCalculatorTool_BasicOperations tests basic arithmetic operations
func TestCalculatorTool_BasicOperations(t *testing.T) {
	tool := createCalculatorTool()
	ctx := context.Background()

	testCases := []struct {
		name      string
		params    map[string]interface{}
		expected  float64
		shouldErr bool
	}{
		{
			name: "Addition",
			params: map[string]interface{}{
				"operation": "add",
				"operand1":  5.0,
				"operand2":  3.0,
			},
			expected: 8.0,
		},
		{
			name: "Subtraction",
			params: map[string]interface{}{
				"operation": "subtract",
				"operand1":  10.0,
				"operand2":  4.0,
			},
			expected: 6.0,
		},
		{
			name: "Multiplication",
			params: map[string]interface{}{
				"operation": "multiply",
				"operand1":  3.0,
				"operand2":  4.0,
			},
			expected: 12.0,
		},
		{
			name: "Division",
			params: map[string]interface{}{
				"operation": "divide",
				"operand1":  15.0,
				"operand2":  3.0,
			},
			expected: 5.0,
		},
		{
			name: "Power",
			params: map[string]interface{}{
				"operation": "power",
				"operand1":  2.0,
				"operand2":  3.0,
			},
			expected: 8.0,
		},
		{
			name: "Modulo",
			params: map[string]interface{}{
				"operation": "modulo",
				"operand1":  17.0,
				"operand2":  5.0,
			},
			expected: 2.0,
		},
		{
			name: "Square Root",
			params: map[string]interface{}{
				"operation": "sqrt",
				"operand1":  25.0,
				"operand2":  0.0, // Not used for sqrt
			},
			expected: 5.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Executor.Execute(ctx, tc.params)

			if tc.shouldErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.Success)

			// Check the result
			resultValue, ok := result.Data.(float64)
			require.True(t, ok, "Result should be a float64")
			assert.InDelta(t, tc.expected, resultValue, 0.0001, "Result should match expected value")

			// Check metadata
			assert.NotNil(t, result.Metadata)
			assert.Equal(t, tc.params["operation"], result.Metadata["operation"])
			assert.Equal(t, tc.params["operand1"], result.Metadata["operand1"])
		})
	}
}

// TestCalculatorTool_ErrorHandling tests error conditions
func TestCalculatorTool_ErrorHandling(t *testing.T) {
	tool := createCalculatorTool()
	ctx := context.Background()

	testCases := []struct {
		name          string
		params        map[string]interface{}
		expectedError string
	}{
		{
			name: "Division by zero",
			params: map[string]interface{}{
				"operation": "divide",
				"operand1":  10.0,
				"operand2":  0.0,
			},
			expectedError: "division by zero",
		},
		{
			name: "Modulo by zero",
			params: map[string]interface{}{
				"operation": "modulo",
				"operand1":  10.0,
				"operand2":  0.0,
			},
			expectedError: "modulo by zero",
		},
		{
			name: "Square root of negative",
			params: map[string]interface{}{
				"operation": "sqrt",
				"operand1":  -4.0,
			},
			expectedError: "square root of negative number",
		},
		{
			name: "Invalid operation",
			params: map[string]interface{}{
				"operation": "invalid",
				"operand1":  5.0,
				"operand2":  3.0,
			},
			expectedError: "unsupported operation: invalid",
		},
		{
			name: "Missing operation",
			params: map[string]interface{}{
				"operand1": 5.0,
				"operand2": 3.0,
			},
			expectedError: "operation parameter is required",
		},
		{
			name: "Missing operand1",
			params: map[string]interface{}{
				"operation": "add",
				"operand2":  3.0,
			},
			expectedError: "operand1 must be a number",
		},
		{
			name: "Invalid operand1 type",
			params: map[string]interface{}{
				"operation": "add",
				"operand1":  "not a number",
				"operand2":  3.0,
			},
			expectedError: "operand1 must be a number",
		},
		{
			name: "Invalid operand2 type",
			params: map[string]interface{}{
				"operation": "add",
				"operand1":  5.0,
				"operand2":  "not a number",
			},
			expectedError: "operand2 must be a number",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Executor.Execute(ctx, tc.params)

			require.NoError(t, err) // No execution error
			require.NotNil(t, result)
			assert.False(t, result.Success)
			assert.Contains(t, result.Error, tc.expectedError)
		})
	}
}

// TestCalculatorTool_EdgeCases tests edge cases and boundary conditions
func TestCalculatorTool_EdgeCases(t *testing.T) {
	tool := createCalculatorTool()
	ctx := context.Background()

	testCases := []struct {
		name     string
		params   map[string]interface{}
		expected float64
	}{
		{
			name: "Large numbers addition",
			params: map[string]interface{}{
				"operation": "add",
				"operand1":  1e15,
				"operand2":  1e15,
			},
			expected: 2e15,
		},
		{
			name: "Small numbers addition",
			params: map[string]interface{}{
				"operation": "add",
				"operand1":  1e-15,
				"operand2":  1e-15,
			},
			expected: 2e-15,
		},
		{
			name: "Negative numbers",
			params: map[string]interface{}{
				"operation": "multiply",
				"operand1":  -5.0,
				"operand2":  -3.0,
			},
			expected: 15.0,
		},
		{
			name: "Zero operations",
			params: map[string]interface{}{
				"operation": "multiply",
				"operand1":  0.0,
				"operand2":  1000.0,
			},
			expected: 0.0,
		},
		{
			name: "Fractional power",
			params: map[string]interface{}{
				"operation": "power",
				"operand1":  8.0,
				"operand2":  1.0 / 3.0,
			},
			expected: 2.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Executor.Execute(ctx, tc.params)

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.Success)

			resultValue, ok := result.Data.(float64)
			require.True(t, ok)
			assert.InDelta(t, tc.expected, resultValue, 1e-10)
		})
	}
}

// TestCalculatorTool_Performance tests performance characteristics
func TestCalculatorTool_Performance(t *testing.T) {
	tool := createCalculatorTool()
	ctx := context.Background()

	params := map[string]interface{}{
		"operation": "multiply",
		"operand1":  123.456,
		"operand2":  789.012,
	}

	// Test single operation performance
	start := time.Now()
	result, err := tool.Executor.Execute(ctx, params)
	duration := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)

	// Should complete quickly
	assert.Less(t, duration, time.Millisecond, "Operation should complete within 1ms")

	// Test multiple operations performance
	numOperations := 1000
	start = time.Now()

	for i := 0; i < numOperations; i++ {
		result, err := tool.Executor.Execute(ctx, params)
		require.NoError(t, err)
		require.True(t, result.Success)
	}

	totalDuration := time.Since(start)
	avgDuration := totalDuration / time.Duration(numOperations)

	assert.Less(t, avgDuration, time.Millisecond, "Average operation should complete within 1ms")

	t.Logf("Performed %d operations in %v (avg: %v per operation)",
		numOperations, totalDuration, avgDuration)
}

// TestCalculatorTool_Concurrency tests concurrent execution
func TestCalculatorTool_Concurrency(t *testing.T) {
	tool := createCalculatorTool()
	ctx := context.Background()

	const numGoroutines = 10
	const operationsPerGoroutine = 100

	results := make(chan *tools.ToolExecutionResult, numGoroutines*operationsPerGoroutine)
	errors := make(chan error, numGoroutines*operationsPerGoroutine)

	// Start multiple goroutines performing calculations
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < operationsPerGoroutine; j++ {
				params := map[string]interface{}{
					"operation": "add",
					"operand1":  float64(goroutineID),
					"operand2":  float64(j),
				}

				result, err := tool.Executor.Execute(ctx, params)
				if err != nil {
					errors <- err
					return
				}
				results <- result
			}
		}(i)
	}

	// Collect results
	var successCount int
	var errorCount int

	timeout := time.After(5 * time.Second)
	for i := 0; i < numGoroutines*operationsPerGoroutine; i++ {
		select {
		case result := <-results:
			if result.Success {
				successCount++
			} else {
				errorCount++
			}
		case err := <-errors:
			t.Errorf("Unexpected error: %v", err)
			errorCount++
		case <-timeout:
			t.Fatal("Test timed out")
		}
	}

	assert.Equal(t, numGoroutines*operationsPerGoroutine, successCount)
	assert.Equal(t, 0, errorCount)
}

// TestCalculatorTool_Context tests context handling
func TestCalculatorTool_Context(t *testing.T) {
	tool := createCalculatorTool()

	t.Run("Normal execution with context", func(t *testing.T) {
		ctx := context.Background()
		params := map[string]interface{}{
			"operation": "add",
			"operand1":  5.0,
			"operand2":  3.0,
		}

		result, err := tool.Executor.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("Execution with timeout context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Microsecond)
		defer cancel()

		params := map[string]interface{}{
			"operation": "add",
			"operand1":  5.0,
			"operand2":  3.0,
		}

		// For this simple calculator, it should still complete
		// In a real implementation with longer operations, this might timeout
		result, err := tool.Executor.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("Execution with cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		params := map[string]interface{}{
			"operation": "add",
			"operand1":  5.0,
			"operand2":  3.0,
		}

		// For this simple calculator, it should still complete
		// In a real implementation, you might want to check for context cancellation
		result, err := tool.Executor.Execute(ctx, params)
		require.NoError(t, err)
		assert.True(t, result.Success)
	})
}

// TestCalculatorTool_Schema tests schema validation (if implemented)
func TestCalculatorTool_Schema(t *testing.T) {
	t.Parallel()
	tool := createCalculatorTool()

	// Test schema structure
	assert.NotNil(t, tool.Schema)
	assert.Equal(t, "object", tool.Schema.Type)
	assert.Contains(t, tool.Schema.Properties, "operation")
	assert.Contains(t, tool.Schema.Properties, "operand1")
	assert.Contains(t, tool.Schema.Properties, "operand2")
	assert.Contains(t, tool.Schema.Required, "operation")
	assert.Contains(t, tool.Schema.Required, "operand1")

	// Test operation enum values
	operationProp := tool.Schema.Properties["operation"]
	assert.NotNil(t, operationProp.Enum)
	assert.Contains(t, operationProp.Enum, "add")
	assert.Contains(t, operationProp.Enum, "subtract")
	assert.Contains(t, operationProp.Enum, "multiply")
	assert.Contains(t, operationProp.Enum, "divide")
}

// TestCalculatorTool_Metadata tests tool metadata
func TestCalculatorTool_Metadata(t *testing.T) {
	t.Parallel()
	tool := createCalculatorTool()

	assert.Equal(t, "calculator", tool.ID)
	assert.Equal(t, "Calculator", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Equal(t, "1.0.0", tool.Version)

	assert.NotNil(t, tool.Metadata)
	assert.Equal(t, "Math Team", tool.Metadata.Author)
	assert.Equal(t, "MIT", tool.Metadata.License)
	assert.Contains(t, tool.Metadata.Tags, "math")
	assert.Contains(t, tool.Metadata.Tags, "calculator")

	// Test examples
	assert.NotEmpty(t, tool.Metadata.Examples)
	example := tool.Metadata.Examples[0]
	assert.Equal(t, "Addition", example.Name)
	assert.NotEmpty(t, example.Description)
	assert.NotNil(t, example.Input)
	assert.NotNil(t, example.Output)
}

// TestCalculatorTool_Capabilities tests tool capabilities
func TestCalculatorTool_Capabilities(t *testing.T) {
	t.Parallel()
	tool := createCalculatorTool()

	assert.NotNil(t, tool.Capabilities)
	assert.False(t, tool.Capabilities.Streaming)
	assert.False(t, tool.Capabilities.Async)
	assert.False(t, tool.Capabilities.Cancelable)
	assert.True(t, tool.Capabilities.Cacheable)
	assert.Equal(t, 5*time.Second, tool.Capabilities.Timeout)
}

// BenchmarkCalculatorTool_Operations benchmarks different operations
func BenchmarkCalculatorTool_Operations(b *testing.B) {
	tool := createCalculatorTool()
	ctx := context.Background()

	operations := []struct {
		name   string
		params map[string]interface{}
	}{
		{
			name: "Addition",
			params: map[string]interface{}{
				"operation": "add",
				"operand1":  123.456,
				"operand2":  789.012,
			},
		},
		{
			name: "Multiplication",
			params: map[string]interface{}{
				"operation": "multiply",
				"operand1":  123.456,
				"operand2":  789.012,
			},
		},
		{
			name: "Division",
			params: map[string]interface{}{
				"operation": "divide",
				"operand1":  123.456,
				"operand2":  789.012,
			},
		},
		{
			name: "Power",
			params: map[string]interface{}{
				"operation": "power",
				"operand1":  123.456,
				"operand2":  2.0,
			},
		},
	}

	for _, op := range operations {
		b.Run(op.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := tool.Executor.Execute(ctx, op.params)
				if err != nil || !result.Success {
					b.Fatalf("Operation failed: %v", err)
				}
			}
		})
	}
}

// Example test showing how to use the calculator tool
func Example_calculatorBasicUsage() {
	tool := createCalculatorTool()
	ctx := context.Background()

	// Perform addition
	params := map[string]interface{}{
		"operation": "add",
		"operand1":  5.0,
		"operand2":  3.0,
	}

	result, err := tool.Executor.Execute(ctx, params)
	if err != nil {
		panic(err)
	}

	if result.Success {
		fmt.Printf("Result: %.0f\n", result.Data.(float64))
	} else {
		fmt.Println("Error:", result.Error)
	}

	// Output: Result: 8
}
