package basic

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ag-ui/go-sdk/pkg/tools"
)

// CalculatorExecutor implements basic arithmetic operations.
// This example demonstrates the simplest form of tool creation with
// parameter validation and error handling.
type CalculatorExecutor struct{}

// Execute performs the arithmetic operation based on the provided parameters.
func (c *CalculatorExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	// Extract parameters with type checking
	operation, ok := params["operation"].(string)
	if !ok {
		return nil, fmt.Errorf("operation parameter must be a string")
	}

	// Extract numbers - handle different numeric types
	var a, b float64
	var err error

	if aVal, exists := params["a"]; exists {
		a, err = extractNumber(aVal)
		if err != nil {
			return nil, fmt.Errorf("parameter 'a': %w", err)
		}
	} else {
		return nil, fmt.Errorf("parameter 'a' is required")
	}

	if bVal, exists := params["b"]; exists {
		b, err = extractNumber(bVal)
		if err != nil {
			return nil, fmt.Errorf("parameter 'b': %w", err)
		}
	} else {
		return nil, fmt.Errorf("parameter 'b' is required")
	}

	// Perform the calculation
	var result float64
	switch operation {
	case "add":
		result = a + b
	case "subtract":
		result = a - b
	case "multiply":
		result = a * b
	case "divide":
		if b == 0 {
			return &tools.ToolExecutionResult{
				Success: false,
				Error:   "division by zero",
				Data:    nil,
			}, nil
		}
		result = a / b
	case "power":
		// Simple power implementation for integer exponents
		if b != float64(int(b)) {
			return nil, fmt.Errorf("power operation only supports integer exponents")
		}
		result = 1
		for i := 0; i < int(b); i++ {
			result *= a
		}
	default:
		return nil, fmt.Errorf("unsupported operation: %s", operation)
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data: map[string]interface{}{
			"result":    result,
			"operation": operation,
			"operands":  []float64{a, b},
		},
		Metadata: map[string]interface{}{
			"execution_time": time.Now().Format(time.RFC3339),
			"precision":      "float64",
		},
	}, nil
}

// extractNumber converts various numeric types to float64
func extractNumber(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("expected number, got %T", value)
	}
}

// CreateCalculatorTool creates and configures the calculator tool.
func CreateCalculatorTool() *tools.Tool {
	return &tools.Tool{
		ID:          "basic_calculator",
		Name:        "Basic Calculator",
		Description: "Performs basic arithmetic operations (add, subtract, multiply, divide, power)",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"operation": {
					Type:        "string",
					Description: "The arithmetic operation to perform",
					Enum: []interface{}{
						"add", "subtract", "multiply", "divide", "power",
					},
				},
				"a": {
					Type:        "number",
					Description: "First operand",
				},
				"b": {
					Type:        "number",
					Description: "Second operand",
				},
			},
			Required: []string{"operation", "a", "b"},
		},
		Metadata: &tools.ToolMetadata{
			Author:        "AG-UI SDK Examples",
			License:       "MIT",
			Documentation: "https://github.com/mattsp1290/ag-ui/blob/main/go-sdk/examples/tools/basic/README.md",
			Tags:          []string{"math", "arithmetic", "basic", "calculator"},
			Examples: []tools.ToolExample{
				{
					Name:        "Addition",
					Description: "Add two numbers",
					Input: map[string]interface{}{
						"operation": "add",
						"a":         5,
						"b":         3,
					},
					Output: map[string]interface{}{
						"result":    8,
						"operation": "add",
						"operands":  []float64{5, 3},
					},
				},
				{
					Name:        "Division",
					Description: "Divide two numbers",
					Input: map[string]interface{}{
						"operation": "divide",
						"a":         10,
						"b":         2,
					},
					Output: map[string]interface{}{
						"result":    5,
						"operation": "divide",
						"operands":  []float64{10, 2},
					},
				},
			},
		},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  false,
			Async:      false,
			Cancelable: true,
			Retryable:  true,
			Cacheable:  true,
			Timeout:    10 * time.Second,
		},
		Executor: &CalculatorExecutor{},
	}
}

// RunCalculatorExample demonstrates the calculator tool functionality
func RunCalculatorExample() {
	// Create registry and register the calculator tool
	registry := tools.NewRegistry()
	calculatorTool := CreateCalculatorTool()

	if err := registry.Register(calculatorTool); err != nil {
		log.Fatalf("Failed to register calculator tool: %v", err)
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

	// Example usage
	ctx := context.Background()

	examples := []map[string]interface{}{
		{"operation": "add", "a": 15, "b": 25},
		{"operation": "multiply", "a": 7, "b": 8},
		{"operation": "divide", "a": 100, "b": 4},
		{"operation": "power", "a": 2, "b": 8},
	}

	fmt.Println("=== Basic Calculator Tool Example ===")
	fmt.Println("Demonstrates: Basic tool creation, parameter validation, and error handling")
	fmt.Println()

	for i, params := range examples {
		fmt.Printf("Example %d: %v\n", i+1, params)
		
		result, err := engine.Execute(ctx, "basic_calculator", params)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		} else if !result.Success {
			fmt.Printf("  Failed: %s\n", result.Error)
		} else {
			fmt.Printf("  Result: %v\n", result.Data)
			fmt.Printf("  Duration: %v\n", result.Duration)
		}
		fmt.Println()
	}

	// Demonstrate error handling
	fmt.Println("=== Error Handling Examples ===")
	
	errorExamples := []map[string]interface{}{
		{"operation": "divide", "a": 10, "b": 0}, // Division by zero
		{"operation": "invalid", "a": 1, "b": 2}, // Invalid operation
		{"operation": "add", "a": 1},              // Missing parameter
	}

	for i, params := range errorExamples {
		fmt.Printf("Error Example %d: %v\n", i+1, params)
		
		result, err := engine.Execute(ctx, "basic_calculator", params)
		if err != nil {
			fmt.Printf("  Validation Error: %v\n", err)
		} else if !result.Success {
			fmt.Printf("  Execution Error: %s\n", result.Error)
		} else {
			fmt.Printf("  Unexpected Success: %v\n", result.Data)
		}
		fmt.Println()
	}
}